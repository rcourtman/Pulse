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

type cliEnv struct {
	exportFile        string
	importFile        string
	passphrase        string
	forceImport       bool
	exit              func(int)
	readPassword      func(int) ([]byte, error)
	mockEnvDefaultDir string
	mockEnvStat       func(string) (os.FileInfo, error)
}

func newCLIEnv() *cliEnv {
	return &cliEnv{
		exit:              os.Exit,
		readPassword:      term.ReadPassword,
		mockEnvDefaultDir: "/opt/pulse",
		mockEnvStat:       os.Stat,
	}
}

func (env *cliEnv) configDeps() *pulsecli.ConfigDeps {
	return pulsecli.NewConfigDeps(
		&env.exportFile,
		&env.importFile,
		&env.passphrase,
		&env.forceImport,
		env.readPassword,
	)
}

func (env *cliEnv) bootstrapDeps() *pulsecli.BootstrapDeps {
	return pulsecli.NewBootstrapDeps(env.exit)
}

func (env *cliEnv) mockDeps() *pulsecli.MockDeps {
	return pulsecli.NewMockDeps(
		env.exit,
		func() string {
			return env.mockEnvDefaultDir
		},
		env.mockEnvStat,
	)
}

func runServer(ctx context.Context) error {
	server.MetricsPort = metricsPort
	return server.Run(ctx, Version)
}

func newRootCmd(env *cliEnv) *cobra.Command {
	if env == nil {
		env = newCLIEnv()
	}
	return pulsecli.NewRootCommand(pulsecli.Options{
		Use:             "pulse",
		Short:           "Pulse - Proxmox VE and PBS monitoring system",
		Long:            `Pulse is a real-time monitoring system for Proxmox Virtual Environment (PVE) and Proxmox Backup Server (PBS)`,
		Version:         Version,
		VersionTemplate: "Pulse {{.Version}}\n",
		RunE:            runServer,
		VersionPrinter:  printVersion,
		Config:          env.configDeps(),
		Bootstrap:       env.bootstrapDeps(),
		Mock:            env.mockDeps(),
	})
}

func executeCLI(env *cliEnv, args []string) error {
	cmd := newRootCmd(env)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func runMain(env *cliEnv, args []string) {
	if env == nil {
		env = newCLIEnv()
	}
	if err := executeCLI(env, args); err != nil {
		env.exit(1)
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
	runMain(newCLIEnv(), os.Args[1:])
}

// Force rebuild 1769525035
