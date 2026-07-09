//go:build !windows

package agentupdate

import (
	"errors"
	"testing"
)

func TestRestartProcess(t *testing.T) {
	orig := execFn
	t.Cleanup(func() { execFn = orig })

	execFn = func(string, []string, []string) error { return nil }
	if err := restartProcess("/bin/true"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	execFn = func(string, []string, []string) error { return errors.New("boom") }
	if err := restartProcess("/bin/false"); err == nil {
		t.Fatal("expected error")
	}
}
