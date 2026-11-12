//go:build unix

package agentstate

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestReadFileRejectsFIFOWithoutWaitingForWriter(t *testing.T) {
	dir := t.TempDir()
	pipe := filepath.Join(dir, "saved-state")
	if err := syscall.Mkfifo(pipe, 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "projected-token")
	if err := os.Symlink(filepath.Base(pipe), alias); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(pipe)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{"path", "opened-handle"} {
		for _, path := range []string{pipe, alias} {
			t.Run(entry+"/"+filepath.Base(path), func(t *testing.T) {
				// Bound the regression test itself. A broken reader must not leave
				// an uninterruptible test goroutine waiting for a pipe writer.
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestReadFIFOChild$")
				cmd.Env = append(os.Environ(), "PULSE_TEST_STATE_FIFO="+path, "PULSE_TEST_STATE_FIFO_ENTRY="+entry)
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("FIFO refusal did not finish: context=%v exit=%v output=%s", ctx.Err(), err, output)
				}
			})
		}
	}
	after, err := os.Lstat(pipe)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("refusal replaced or modified the saved pipe")
	}
}

func TestReadFIFOChild(t *testing.T) {
	path := os.Getenv("PULSE_TEST_STATE_FIFO")
	if path == "" {
		return // Executed only by the bounded parent fixture.
	}
	var data []byte
	var err error
	if os.Getenv("PULSE_TEST_STATE_FIFO_ENTRY") == "opened-handle" {
		// Models the opened object after a regular path was swapped to a
		// FIFO. Exercise the production nonblocking open and descriptor check.
		f, openErr := openStateFile(path)
		if openErr != nil {
			t.Fatal(openErr)
		}
		defer f.Close()
		data, err = readOpenedFile(f)
	} else {
		data, err = ReadFile(path)
	}
	if !errors.Is(err, ErrNotRegular) || data != nil {
		t.Fatalf("FIFO became saved authority: %v", err)
	}
}
