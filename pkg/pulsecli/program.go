package pulsecli

import (
	"context"
	"os"

	"github.com/spf13/cobra"
)

type Program struct {
	Command     CommandSpec
	Runtime     RuntimeSpec
	Deps        CommandDeps
	Exit        func(int)
	HandleError func(error)
}

func (program *Program) exitFunc() func(int) {
	if program != nil && program.Exit != nil {
		return program.Exit
	}
	return os.Exit
}

func (program *Program) RootCommand() *cobra.Command {
	if program == nil {
		return NewRootCommand(CommandSpec{}, RuntimeSpec{}, CommandDeps{})
	}
	return NewRootCommand(program.Command, program.Runtime, program.Deps)
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
	if err := program.Execute(ctx, args); err != nil {
		if program != nil && program.HandleError != nil {
			program.HandleError(err)
		}
		program.exitFunc()(1)
	}
}
