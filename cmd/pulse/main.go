package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

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

func resolveMetricsPortFromEnv(stderr io.Writer, fallback int) int {
	for _, envName := range []string{"PULSE_METRICS_PORT", "METRICS_PORT"} {
		raw := strings.TrimSpace(os.Getenv(envName))
		if raw == "" {
			continue
		}

		port, err := strconv.Atoi(raw)
		if err != nil || port < 0 || port > 65535 {
			if stderr != nil {
				fmt.Fprintf(stderr, "Ignoring invalid %s value %q; using metrics port %d\n", envName, raw, fallback)
			}
			return fallback
		}
		return port
	}

	return fallback
}

func runServer(ctx context.Context) error {
	server.MetricsPort = resolveMetricsPortFromEnv(os.Stderr, metricsPort)
	return server.Run(ctx, Version)
}

func newProgram(env *pulsecli.Env, process pulsecli.ProcessIO, mockFS pulsecli.MockFS) *pulsecli.Program {
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
		Deps: env.CommandDeps(process, mockFS),
		Exit: process.Exit,
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
	newProgram(pulsecli.NewEnv(), pulsecli.NewProcessIO(), pulsecli.NewMockFS()).Run(context.Background(), os.Args[1:])
}

// Force rebuild 1769525035
