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
	Runtime         *Runtime
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
	cmd.AddCommand(newConfigCmd(opts.Runtime))
	cmd.AddCommand(newBootstrapTokenCmd(opts.Runtime))
	cmd.AddCommand(newMockCmd(opts.Runtime))

	return cmd
}

func ResetFlags(runtime *Runtime) {
	if runtime == nil {
		return
	}
	if runtime.Config.ExportFile != nil {
		*runtime.Config.ExportFile = ""
	}
	if runtime.Config.ImportFile != nil {
		*runtime.Config.ImportFile = ""
	}
	if runtime.Config.Passphrase != nil {
		*runtime.Config.Passphrase = ""
	}
	if runtime.Config.ForceImport != nil {
		*runtime.Config.ForceImport = false
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
