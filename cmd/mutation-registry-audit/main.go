package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/internal/mutationregistry"
)

func main() {
	check := flag.Bool("check", false, "validate the closed mutation registry")
	format := flag.String("format", "json", "output format (json)")
	flag.Parse()
	if *check {
		if err := mutationregistry.Validate(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if *format != "json" {
		fmt.Fprintf(os.Stderr, "unsupported format %q\n", *format)
		os.Exit(2)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(mutationregistry.Entries()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
