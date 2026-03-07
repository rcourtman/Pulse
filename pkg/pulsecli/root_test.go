package pulsecli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestNewRootCommandVersionSkipsRunE(t *testing.T) {
	runCalled := false
	cmd := NewRootCommand(Options{
		Use:            "pulse",
		Short:          "Pulse",
		Long:           "Pulse",
		Version:        "1.2.3",
		RunE:           func(context.Context) error { runCalled = true; return nil },
		VersionPrinter: func(w io.Writer) { _, _ = w.Write([]byte("Pulse 1.2.3\n")) },
	})
	out := bytes.NewBuffer(nil)
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"version"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext(version): %v", err)
	}
	if runCalled {
		t.Fatal("RunE should not be called for version")
	}
	if got := out.String(); !strings.Contains(got, "Pulse 1.2.3") {
		t.Fatalf("version output = %q", got)
	}
}

func TestNewRootCommandConfigInfoSkipsRunE(t *testing.T) {
	runCalled := false
	cmd := NewRootCommand(Options{
		Use:   "pulse",
		Short: "Pulse",
		Long:  "Pulse",
		RunE:  func(context.Context) error { runCalled = true; return nil },
		State: &State{},
	})
	out := bytes.NewBuffer(nil)
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"config", "info"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext(config info): %v", err)
	}
	if runCalled {
		t.Fatal("RunE should not be called for config info")
	}
	if got := out.String(); !strings.Contains(got, "Pulse Configuration Information") {
		t.Fatalf("config info output = %q", got)
	}
}
