package main

import (
	"fmt"
	"io"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/internal/auth"
)

var (
	hashPassword           = auth.HashPassword
	osArgs                 = os.Args
	osExit                 = os.Exit
	stdout       io.Writer = os.Stdout
)

func run(args []string, out io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(out, "Usage: hashpw <password>")
		return 1
	}

	password := args[1]
	hash, err := hashPassword(password)
	if err != nil {
		fmt.Fprintf(out, "Error: %v\n", err)
		return 1
	}

	fmt.Fprintln(out, hash)
	return 0
}

func main() {
	osExit(run(osArgs, stdout))
}
