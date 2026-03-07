package pulsecli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type CommandSpec struct {
	Use             string
	Short           string
	Long            string
	Version         string
	VersionTemplate string
	VersionPrinter  func(io.Writer)
}

type RuntimeSpec struct {
	Run func(context.Context) error
}

type CommandDeps struct {
	Config    *ConfigDeps
	Bootstrap *BootstrapDeps
	Mock      *MockDeps
}

func NewRootCommand(command CommandSpec, runtime RuntimeSpec, deps CommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:          command.Use,
		Short:        command.Short,
		Long:         command.Long,
		SilenceUsage: true,
		Version:      command.Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtime.Run == nil {
				return nil
			}
			return runtime.Run(cmd.Context())
		},
	}

	if command.VersionTemplate != "" {
		cmd.SetVersionTemplate(command.VersionTemplate)
	}

	cmd.AddCommand(newVersionCmd(command))
	cmd.AddCommand(newConfigCmd(deps.Config))
	cmd.AddCommand(newBootstrapTokenCmd(deps.Bootstrap))
	cmd.AddCommand(newMockCmd(deps.Mock))

	return cmd
}

func ResetFlags(config *ConfigDeps) {
	if config == nil {
		return
	}
	if config.ExportFile != nil {
		*config.ExportFile = ""
	}
	if config.ImportFile != nil {
		*config.ImportFile = ""
	}
	if config.Passphrase != nil {
		*config.Passphrase = ""
	}
	if config.ForceImport != nil {
		*config.ForceImport = false
	}
}

func newVersionCmd(command CommandSpec) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			if command.VersionPrinter != nil {
				command.VersionPrinter(cmd.OutOrStdout())
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", command.Version)
		},
	}
}
