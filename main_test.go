package main
import (
    "fmt"
    "testing"
)

func TestIntMinTableDriven(t *testing.T) {
	fmt.Println("test")

	AddDNS("8.8.8.8", "google.com", 123)
}
