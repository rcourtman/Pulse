package pulsecli

import (
	"context"

	"github.com/spf13/cobra"
)

type Program struct {
	Root        Options
	Env         *Env
	HandleError func(error)
}

func (program *Program) commandEnv() *Env {
	if program == nil || program.Env == nil {
		return NewEnv()
	}
	return program.Env
}

func (program *Program) rootOptions() Options {
	if program == nil {
		return Options{}
	}

	opts := program.Root
	env := program.commandEnv()
	if opts.Config == nil {
		opts.Config = env.ConfigDeps()
	}
	if opts.Bootstrap == nil {
		opts.Bootstrap = env.BootstrapDeps()
	}
	if opts.Mock == nil {
		opts.Mock = env.MockDeps()
	}

	return opts
}

func (program *Program) RootCommand() *cobra.Command {
	return NewRootCommand(program.rootOptions())
}

func (program *Program) Execute(ctx context.Context, args []string) error {
	cmd := program.RootCommand()
	cmd.SetArgs(args)
	if ctx != nil {
		return cmd.ExecuteContext(ctx)
	}
	return cmd.Execute()
}

func (program *Program) Run(ctx context.Context, args []string) {
	env := program.commandEnv()
	if err := program.Execute(ctx, args); err != nil {
		if program != nil && program.HandleError != nil {
			program.HandleError(err)
		}
		env.Exit(1)
	}
}
