package hostagent

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

func TestPersistAgentID_WritesFile(t *testing.T) {
	mc := &mockCollector{}
	var writtenPath string
	var writtenData []byte

	mc.mkdirAllFn = func(path string, perm os.FileMode) error { return nil }
	mc.writeFileFn = func(filename string, data []byte, perm os.FileMode) error {
		writtenPath = filename
		writtenData = data
		if perm != 0600 {
			t.Fatalf("expected perm 0600, got %o", perm)
		}
		return nil
	}

	a := &Agent{
		logger:    zerolog.Nop(),
		stateDir:  "/tmp/test-state",
		collector: mc,
	}

	a.persistAgentID("abc-123")

	expected := filepath.Join("/tmp/test-state", "agent-id")
	if writtenPath != expected {
		t.Fatalf("wrote to %q, want %q", writtenPath, expected)
	}
	if string(writtenData) != "abc-123" {
		t.Fatalf("wrote %q, want %q", string(writtenData), "abc-123")
	}
}

func TestPersistAgentID_EmptyAgentID(t *testing.T) {
	mc := &mockCollector{}

	a := &Agent{
		logger:    zerolog.Nop(),
		stateDir:  "/tmp/test-state",
		collector: mc,
	}

	// persistAgentID is only called when agentID != "" (guard is in sendReport).
	// Calling directly with empty string is safe — mkdirAll + write of empty content.
	a.persistAgentID("")
}

func TestPersistAgentID_EmptyStateDir(t *testing.T) {
	mc := &mockCollector{}
	mkdirCalled := false
	mc.mkdirAllFn = func(path string, perm os.FileMode) error {
		mkdirCalled = true
		return nil
	}

	a := &Agent{
		logger:    zerolog.Nop(),
		stateDir:  "",
		collector: mc,
	}

	a.persistAgentID("abc-123")

	if mkdirCalled {
		t.Fatal("expected no MkdirAll call when stateDir is empty")
	}
}

func TestPersistAgentID_MkdirFails(t *testing.T) {
	mc := &mockCollector{}
	writeCalled := false

	mc.mkdirAllFn = func(path string, perm os.FileMode) error {
		return fmt.Errorf("permission denied")
	}
	mc.writeFileFn = func(filename string, data []byte, perm os.FileMode) error {
		writeCalled = true
		return nil
	}

	a := &Agent{
		logger:    zerolog.Nop(),
		stateDir:  "/tmp/test-state",
		collector: mc,
	}

	// Should not panic, just debug-log and return
	a.persistAgentID("abc-123")

	if writeCalled {
		t.Fatal("expected WriteFile not called when MkdirAll fails")
	}
}

func TestPersistAgentID_WriteFileFails(t *testing.T) {
	mc := &mockCollector{}

	mc.mkdirAllFn = func(path string, perm os.FileMode) error { return nil }
	mc.writeFileFn = func(filename string, data []byte, perm os.FileMode) error {
		return fmt.Errorf("disk full")
	}

	a := &Agent{
		logger:    zerolog.Nop(),
		stateDir:  "/tmp/test-state",
		collector: mc,
	}

	// Should not panic, just debug-log
	a.persistAgentID("abc-123")
}

func TestPersistAgentID_MkdirAllPermissions(t *testing.T) {
	mc := &mockCollector{}
	var gotPerm os.FileMode

	mc.mkdirAllFn = func(path string, perm os.FileMode) error {
		gotPerm = perm
		return nil
	}
	mc.writeFileFn = func(filename string, data []byte, perm os.FileMode) error { return nil }

	a := &Agent{
		logger:    zerolog.Nop(),
		stateDir:  "/tmp/test-state",
		collector: mc,
	}

	a.persistAgentID("abc-123")

	if gotPerm != 0700 {
		t.Fatalf("MkdirAll perm = %o, want %o", gotPerm, 0700)
	}
}
