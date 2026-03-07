package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli"
	"github.com/rcourtman/pulse-go-rewrite/pkg/server"
)

// Version information (set at build time with -ldflags)
var (
	Version     = "dev"
	BuildTime   = "unknown"
	GitCommit   = "unknown"
	metricsPort = 9091
)

func runServer(ctx context.Context) error {
	server.MetricsPort = metricsPort
	return server.Run(ctx, Version)
}

func newProgram(env *pulsecli.Env) *pulsecli.Program {
	if env == nil {
		env = pulsecli.NewEnv()
	}
	return &pulsecli.Program{
		Command: pulsecli.CommandSpec{
			Use:             "pulse",
			Short:           "Pulse - Proxmox VE and PBS monitoring system",
			Long:            `Pulse is a real-time monitoring system for Proxmox Virtual Environment (PVE) and Proxmox Backup Server (PBS)`,
			Version:         Version,
			VersionTemplate: "Pulse {{.Version}}\n",
			VersionPrinter:  printVersion,
		},
		Runtime: pulsecli.RuntimeSpec{
			Run: runServer,
		},
		Deps: env.CommandDeps(),
		Exit: env.Exit,
	}
}

func printVersion(w io.Writer) {
	fmt.Fprintf(w, "Pulse %s\n", Version)
	if BuildTime != "unknown" {
		fmt.Fprintf(w, "Built: %s\n", BuildTime)
	}
	if GitCommit != "unknown" {
		fmt.Fprintf(w, "Commit: %s\n", GitCommit)
	}
}

func main() {
	newProgram(pulsecli.NewEnv()).Run(context.Background(), os.Args[1:])
}

// Force rebuild 1769525035
