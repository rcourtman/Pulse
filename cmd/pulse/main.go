package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli"
	"github.com/rcourtman/pulse-go-rewrite/pkg/server"
	"github.com/spf13/cobra"
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

func newRootCmd(env *pulsecli.Env) *cobra.Command {
	if env == nil {
		env = pulsecli.NewEnv()
	}
	return pulsecli.NewRootCommand(pulsecli.Options{
		Use:             "pulse",
		Short:           "Pulse - Proxmox VE and PBS monitoring system",
		Long:            `Pulse is a real-time monitoring system for Proxmox Virtual Environment (PVE) and Proxmox Backup Server (PBS)`,
		Version:         Version,
		VersionTemplate: "Pulse {{.Version}}\n",
		RunE:            runServer,
		VersionPrinter:  printVersion,
		Config:          env.ConfigDeps(),
		Bootstrap:       env.BootstrapDeps(),
		Mock:            env.MockDeps(),
	})
}

func executeCLI(env *pulsecli.Env, args []string) error {
	cmd := newRootCmd(env)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func runMain(env *pulsecli.Env, args []string) {
	if env == nil {
		env = pulsecli.NewEnv()
	}
	if err := executeCLI(env, args); err != nil {
		env.Exit(1)
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
	runMain(pulsecli.NewEnv(), os.Args[1:])
}

// Force rebuild 1769525035
