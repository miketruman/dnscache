package main
import (
    "fmt"
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
