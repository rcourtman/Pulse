package main

import (
	"fmt"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/internal/auth"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: hashpw <password>")
		os.Exit(1)
	}

	password := os.Args[1]
	hash, err := auth.HashPassword(password)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(hash)
}
