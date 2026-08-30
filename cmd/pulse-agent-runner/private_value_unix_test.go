//go:build !windows

package main

import (
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestReadPrivateValueRejectsFIFOWithoutBlocking(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runner.token")
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, err := readPrivateValue(path, "runner token"); err == nil {
		t.Fatal("FIFO was accepted as a private runner value")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("FIFO rejection blocked for %s", elapsed)
	}
}
