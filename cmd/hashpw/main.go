package main

import (
	"fmt"
	"io"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/internal/auth"
)

func run(args []string, out io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(out, "Usage: hashpw <password>")
		return 1
	}

	password := args[1]
	hash, err := auth.HashPassword(password)
	if err != nil {
		fmt.Fprintf(out, "Error: %v\n", err)
		return 1
	}

	fmt.Fprintln(out, hash)
	return 0
}

func main() {
	os.Exit(run(os.Args, os.Stdout))
}
