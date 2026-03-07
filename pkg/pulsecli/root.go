package pulsecli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type Options struct {
	Use             string
	Short           string
	Long            string
	Version         string
	VersionTemplate string
	RunE            func(context.Context) error
	VersionPrinter  func(io.Writer)
	State           *State
}

func NewRootCommand(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:          opts.Use,
		Short:        opts.Short,
		Long:         opts.Long,
		SilenceUsage: true,
		Version:      opts.Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.RunE == nil {
				return nil
			}
			return opts.RunE(cmd.Context())
		},
	}

	if opts.VersionTemplate != "" {
		cmd.SetVersionTemplate(opts.VersionTemplate)
	}

	cmd.AddCommand(newVersionCmd(opts))
	cmd.AddCommand(newConfigCmd(opts.State))
	cmd.AddCommand(newBootstrapTokenCmd(opts.State))
	cmd.AddCommand(newMockCmd(opts.State))

	return cmd
}

func ResetFlags(state *State) {
	if state == nil {
		return
	}
	if state.ExportFile != nil {
		*state.ExportFile = ""
	}
	if state.ImportFile != nil {
		*state.ImportFile = ""
	}
	if state.Passphrase != nil {
		*state.Passphrase = ""
	}
	if state.ForceImport != nil {
		*state.ForceImport = false
	}
}

func newVersionCmd(opts Options) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			if opts.VersionPrinter != nil {
				opts.VersionPrinter(cmd.OutOrStdout())
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", opts.Version)
		},
	}
}
