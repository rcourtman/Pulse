package pulsecli

import (
	"context"
	"errors"
	"testing"
)

func TestProgramRootCommandUsesEnvForConfigFlags(t *testing.T) {
	env := NewEnv()
	process := NewProcess()
	program := &Program{
		Command: CommandSpec{
			Use: "pulse",
		},
		Deps: env.CommandDeps(process),
		Exit: process.Exit,
	}

	cmd := program.RootCommand()
	configExportCmd, _, err := cmd.Find([]string{"config", "export"})
	if err != nil {
		t.Fatalf("find config export: %v", err)
	}
	if err := configExportCmd.ParseFlags([]string{"-o", "pulse.enc", "-p", "secret"}); err != nil {
		t.Fatalf("parse export flags: %v", err)
	}

	if env.ExportFile != "pulse.enc" {
		t.Fatalf("ExportFile = %q, want %q", env.ExportFile, "pulse.enc")
	}
	if env.Passphrase != "secret" {
		t.Fatalf("Passphrase = %q, want %q", env.Passphrase, "secret")
	}

	configImportCmd, _, err := cmd.Find([]string{"config", "import"})
	if err != nil {
		t.Fatalf("find config import: %v", err)
	}
	if err := configImportCmd.ParseFlags([]string{"-i", "pulse.enc", "-p", "secret", "-f"}); err != nil {
		t.Fatalf("parse import flags: %v", err)
	}

	if env.ImportFile != "pulse.enc" {
		t.Fatalf("ImportFile = %q, want %q", env.ImportFile, "pulse.enc")
	}
	if !env.ForceImport {
		t.Fatal("ForceImport = false, want true")
	}
}

func TestProgramRunReportsErrorAndExits(t *testing.T) {
	process := NewProcess()
	exitCode := 0
	process.Exit = func(code int) {
		exitCode = code
	}

	var handled error
	program := &Program{
		Command: CommandSpec{
			Use: "pulse",
		},
		Runtime: RuntimeSpec{
			Run: func(context.Context) error {
				return errors.New("boom")
			},
		},
		Exit: process.Exit,
		HandleError: func(err error) {
			handled = err
		},
	}

	program.Run(context.Background(), nil)

	if handled == nil || handled.Error() != "boom" {
		t.Fatalf("handled error = %v, want boom", handled)
	}
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
}
