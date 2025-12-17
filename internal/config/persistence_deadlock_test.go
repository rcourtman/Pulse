package config

import (
	"testing"
	"time"
)

func TestSaveNodesConfig_EmptyDoesNotDeadlockWhenExistingNodesPresent(t *testing.T) {
	cp := NewConfigPersistence(t.TempDir())

	if err := cp.SaveNodesConfig([]PVEInstance{{Name: "pve1", Host: "https://example.invalid:8006", User: "root@pam"}}, nil, nil); err != nil {
		t.Fatalf("SaveNodesConfig seed: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cp.SaveNodesConfig(nil, nil, nil)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected error when saving empty over existing nodes")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("SaveNodesConfig appears to have deadlocked")
	}
}
