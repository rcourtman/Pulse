// Command pulse-cli is the lightweight cross-platform workstation CLI for Pulse.
// It ships with zero server runtime and zero embedded frontend, providing
// `pulse auth`, `pulse connect`, `pulse doctor`, built-in `pulse mcp`, and
// manifest-backed fleet/read/action commands.
//
// When invoked via a symlink or binary name starting with `pulse-mcp`, it
// dispatches directly to `mcp` for seamless backwards compatibility.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli"
)

// Version information (set at build time with -ldflags).
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func printVersion(w io.Writer) {
	fmt.Fprintf(w, "Pulse CLI %s\n", Version)
	if BuildTime != "unknown" {
		fmt.Fprintf(w, "Built: %s\n", BuildTime)
	}
	if GitCommit != "unknown" {
		fmt.Fprintf(w, "Commit: %s\n", GitCommit)
	}
}

func main() {
	cmd := pulsecli.NewClientRootCommand(pulsecli.CommandSpec{
		Use:             "pulse",
		Short:           "Pulse CLI — connect external agents and inspect your Pulse fleet",
		Version:         Version,
		VersionTemplate: "Pulse CLI {{.Version}}\n",
		VersionPrinter:  printVersion,
	}, pulsecli.ClientDeps{})

	cmd.SetArgs(pulsecli.NormalizeClientArgs(os.Args[0], os.Args[1:]))
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}
