package main

import (
	"encoding/json"
	"fmt"
	"github.com/allegro/bigcache/v3"
	"github.com/tidwall/redcon"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

var addr = ":6380"

var config = bigcache.Config{
	// number of shards (must be a power of 2)
	Shards: 1024,

	// time after which entry can be evicted
	LifeWindow: 10 * time.Minute,

	// Interval between removing expired entries (clean up).
	// If set to <= 0 then no action is performed.
	// Setting to < 1 second is counterproductive — bigcache has a one second resolution.
	CleanWindow: 5 * time.Minute,

	// rps * lifeWindow, used only in initial memory allocation
	MaxEntriesInWindow: 1000 * 10 * 60,

	// max entry size in bytes, used only in initial memory allocation
	MaxEntrySize: 500,

	// prints information about additional memory allocation
	Verbose: true,

	// cache will not allocate more memory than this limit, value in MB
	// if value is reached then the oldest entries can be overridden for the new ones
	// 0 value means no size limit
	HardMaxCacheSize: 8192,

	// callback fired when the oldest entry is removed because of its expiration time or no space left
	// for the new entry, or because delete was called. A bitmask representing the reason will be returned.
	// Default value is nil which means no callback and it prevents from unwrapping the oldest entry.
	OnRemove: nil,

	// OnRemoveWithReason is a callback fired when the oldest entry is removed because of its expiration time or no space left
	// for the new entry, or because delete was called. A constant representing the reason will be passed through.
	// Default value is nil which means no callback and it prevents from unwrapping the oldest entry.
	// Ignored if OnRemove is specified.
	OnRemoveWithReason: nil,
}


func dnsCache() *bigcache.BigCache {
	cache, initErr := bigcache.NewBigCache(config)
	if initErr != nil {
		log.Fatal(initErr)
	}
	return cache
}

var cache = dnsCache()

type DNS struct {
	DNS string
	TTL uint64
}
type IP struct {
	DNS map[string]uint64
}

func AddDNS(ip string, dns string, ttl uint64) {
	result, err := cache.Get(ip)
	if err != nil {
		dnsResolve := &IP{DNS: make(map[string]uint64)}
		dnsResolve.DNS[dns] = ttl
		b, _ := json.Marshal(dnsResolve)
		cache.Set(ip, b)
	} else {
		var dnsResolve IP
		if err := json.Unmarshal(result, &dnsResolve); err != nil {
			panic(err)
		}
		dnsResolve.DNS[dns] = ttl
		b, _ := json.Marshal(dnsResolve)
		cache.Set(ip, b)
	}
}

func GetDNS(ip string) (string, bool) {
	result, err := cache.Get(ip)
	if err != nil {
		return "", false
	}else{
		var dnsResolve IP
		if err := json.Unmarshal(result, &dnsResolve); err != nil {
			return "", false
		}
		stringArray := []string{}
		for k := range dnsResolve.DNS {
			stringArray = append(stringArray, k)
		}
		return strings.Join(stringArray,","), true
	}
	return "", false
}

func headers(w http.ResponseWriter, req *http.Request) {
	AddDNS("8.8.8.8", "k8.google.com", 10)
	AddDNS("8.8.8.8", "www.google.com", 10)
	
	fmt.Fprintln(w, "Stats:", cache.Stats())
	fmt.Fprintln(w, "Len:", cache.Len())
	fmt.Fprintln(w, "Capacity:", cache.Capacity())

	result, _ := cache.Get("8.8.8.8")
	fmt.Fprintln(w, "get:", string(result))
	result2, result2b := GetDNS("8.8.8.8")
	fmt.Fprintln(w, "get:", string(result2), result2b)
	result3, result3b := GetDNS("8.8.8.1")
	fmt.Fprintln(w, "get:", string(result3), result3b)
}


func main() {

	http.HandleFunc("/", headers)

	http.ListenAndServe(":8090", nil)

	cache.Set("my-unique-key", []byte("value"))

	if entry, err := cache.Get("my-unique-key"); err == nil {
		fmt.Println(string(entry))
	}

	var mu sync.RWMutex
	var items = make(map[string][]byte)
	var ps redcon.PubSub
	go log.Printf("started server at %s", addr)
	err := redcon.ListenAndServe(addr,
		func(conn redcon.Conn, cmd redcon.Command) {
			switch strings.ToLower(string(cmd.Args[0])) {
			default:
				conn.WriteError("ERR unknown command '" + string(cmd.Args[0]) + "'")
			case "ping":
				conn.WriteString("PONG")
			case "quit":
				conn.WriteString("OK")
				conn.Close()
			case "set":
				if len(cmd.Args) != 3 {
					conn.WriteError("ERR wrong number of arguments for '" + string(cmd.Args[0]) + "' command")
					return
				}
				mu.Lock()
				items[string(cmd.Args[1])] = cmd.Args[2]
				mu.Unlock()
				conn.WriteString("OK")
			case "get":
				if len(cmd.Args) != 2 {
					conn.WriteError("ERR wrong number of arguments for '" + string(cmd.Args[0]) + "' command")
					return
				}
				mu.RLock()
				val, ok := items[string(cmd.Args[1])]
				mu.RUnlock()
				if !ok {
					conn.WriteNull()
				} else {
					conn.WriteBulk(val)
				}
			case "del":
				if len(cmd.Args) != 2 {
					conn.WriteError("ERR wrong number of arguments for '" + string(cmd.Args[0]) + "' command")
					return
				}
				mu.Lock()
				_, ok := items[string(cmd.Args[1])]
				delete(items, string(cmd.Args[1]))
				mu.Unlock()
				if !ok {
					conn.WriteInt(0)
				} else {
					conn.WriteInt(1)
				}
			case "publish":
				if len(cmd.Args) != 3 {
					conn.WriteError("ERR wrong number of arguments for '" + string(cmd.Args[0]) + "' command")
					return
				}
				conn.WriteInt(ps.Publish(string(cmd.Args[1]), string(cmd.Args[2])))
			case "subscribe", "psubscribe":
				if len(cmd.Args) < 2 {
					conn.WriteError("ERR wrong number of arguments for '" + string(cmd.Args[0]) + "' command")
					return
				}
				command := strings.ToLower(string(cmd.Args[0]))
				for i := 1; i < len(cmd.Args); i++ {
					if command == "psubscribe" {
						ps.Psubscribe(conn, string(cmd.Args[i]))
					} else {
						ps.Subscribe(conn, string(cmd.Args[i]))
					}
				}
			}
		},
		func(conn redcon.Conn) bool {
			// Use this function to accept or deny the connection.
			// log.Printf("accept: %s", conn.RemoteAddr())
			return true
		},
		func(conn redcon.Conn, err error) {
			// This is called when the connection has been closed
			// log.Printf("closed: %s, err: %v", conn.RemoteAddr(), err)
		},
	)
	if err != nil {
		log.Fatal(err)
	}

}
