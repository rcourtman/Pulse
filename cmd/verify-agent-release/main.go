// verify-agent-release checks copied agent artifacts before release packaging.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentidentity"
)

func verify(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("verify-agent-release", flag.ContinueOnError)
	flags.SetOutput(stderr)
	version := flags.String("version", "", "exact release version")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if _, err := agentidentity.Version(*version); err != nil {
		return err
	}
	pairs := flags.Args()
	if len(pairs) == 0 || len(pairs)%2 != 0 {
		return fmt.Errorf("expected target and executable path pairs")
	}
	for index := 0; index < len(pairs); index += 2 {
		target, path := pairs[index], pairs[index+1]
		if target == "" {
			return fmt.Errorf("empty executable target")
		}
		if err := agentidentity.Validate(path, *version, target); err != nil {
			return fmt.Errorf("%s (%s): %w", path, target, err)
		}
	}
	return nil
}

func main() {
	if err := verify(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
