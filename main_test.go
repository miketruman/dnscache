package main

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"testing"
)

func TestIntMinTableDriven(t *testing.T) {
	fmt.Println("test")

	AddDNS("8.8.8.8", "2.google.com", 123)
	AddDNS("8.8.8.8", "1.google.com", 123)

	result, ok := GetDNS("8.8.8.8")
	fmt.Println("result:", string(result))
	if ok != true {
		t.Fatalf("Panic! nothing found")
	}
	if string(result) != "1.google.com,2.google.com" {
		t.Fatalf("Panic! match failed. Result: %v", string(result))
	}
}

var ctx = context.Background()

func TestRedisServer(t *testing.T) {
	go redisServer()

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	fmt.Println(rdb)

	err := rdb.Set(ctx, "8.8.8.4", "1.google.com;22", 0).Err()
	if err != nil {
		panic(err)
	}

	val, err := rdb.Get(ctx, "8.8.8.4").Result()
	if err != nil {
		panic(err)
	}
	if string(val) != "1.google.com" {
		t.Fatalf("Panic! match failed. Result: %v", string(val))
	}
	// Test not found
	val, err = rdb.Get(ctx, "8.8.8.0").Result()
	if err == nil {
		panic(err)
	}
}
