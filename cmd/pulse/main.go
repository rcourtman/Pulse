package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli"
	"github.com/rcourtman/pulse-go-rewrite/pkg/server"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Version information (set at build time with -ldflags)
var (
	Version     = "dev"
	BuildTime   = "unknown"
	GitCommit   = "unknown"
	metricsPort = 9091
)

var (
	exportFile        string
	importFile        string
	passphrase        string
	forceImport       bool
	osExit            = os.Exit
	readPassword      = term.ReadPassword
	mockEnvDefaultDir = "/opt/pulse"
	mockEnvStat       = os.Stat
)

var cliRuntime = &pulsecli.Runtime{
	Config: pulsecli.ConfigRuntime{
		ExportFile:  &exportFile,
		ImportFile:  &importFile,
		Passphrase:  &passphrase,
		ForceImport: &forceImport,
		ReadPassword: func(fd int) ([]byte, error) {
			return readPassword(fd)
		},
	},
	Bootstrap: pulsecli.BootstrapRuntime{
		Exit: func(code int) {
			osExit(code)
		},
	},
	Mock: pulsecli.MockRuntime{
		Exit: func(code int) {
			osExit(code)
		},
		DefaultEnvDir: func() string {
			return mockEnvDefaultDir
		},
		Stat: func(path string) (os.FileInfo, error) {
			return mockEnvStat(path)
		},
	},
}

var rootCmd = newRootCmd()

func runServer(ctx context.Context) error {
	server.MetricsPort = metricsPort
	return server.Run(ctx, Version)
}

func newRootCmd() *cobra.Command {
	return pulsecli.NewRootCommand(pulsecli.Options{
		Use:             "pulse",
		Short:           "Pulse - Proxmox VE and PBS monitoring system",
		Long:            `Pulse is a real-time monitoring system for Proxmox Virtual Environment (PVE) and Proxmox Backup Server (PBS)`,
		Version:         Version,
		VersionTemplate: "Pulse {{.Version}}\n",
		RunE:            runServer,
		VersionPrinter:  printVersion,
		Runtime:         cliRuntime,
	})
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
	if err := rootCmd.Execute(); err != nil {
		osExit(1)
	}
}

// Force rebuild 1769525035
