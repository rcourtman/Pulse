//go:build linux

package api

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Only used after a regression: supply EOF to an actual blocked reader and
// join it, rather than abandoning a goroutine or waiting for the suite timeout.
func releaseStartupFIFO(t *testing.T, fifo string, done <-chan startupAuthInventory) startupAuthInventory {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		writer, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err == nil {
			_, writeErr := writer.Write([]byte("[]"))
			closeErr := writer.Close()
			if writeErr != nil || closeErr != nil {
				t.Fatalf("release regressed FIFO reader: write=%v close=%v", writeErr, closeErr)
			}
			select {
			case s := <-done:
				return s
			case <-time.After(5 * time.Second):
				t.Fatal("regressed inventory reader did not join after EOF")
			}
		}
		if !errors.Is(err, syscall.ENXIO) {
			t.Fatalf("open regressed FIFO writer: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("regressed inventory reader did not open its FIFO")
	return startupAuthInventory{}
}

func TestAuthInventoryStartupRefusesFIFO(t *testing.T) {
	for _, kind := range []string{"sessions", "csrf_tokens", "recovery_tokens"} {
		for _, linked := range []bool{false, true} {
			shape := "direct"
			if linked {
				shape = "linked"
			}
			t.Run(kind+"/"+shape, func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, kind+".json")
				fifo := path
				if linked {
					fifo = filepath.Join(dir, "unread-pipe")
				}
				if err := syscall.Mkfifo(fifo, 0600); err != nil {
					t.Fatal(err)
				}
				if linked {
					if err := os.Symlink(filepath.Base(fifo), path); err != nil {
						t.Fatal(err)
					}
				}
				entryBefore, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				fifoBefore, err := os.Lstat(fifo)
				if err != nil {
					t.Fatal(err)
				}
				done := make(chan startupAuthInventory, 1)
				go func() { done <- startAuthInventory(kind, dir) }()
				var s startupAuthInventory
				select {
				case s = <-done:
				case <-time.After(2 * time.Second):
					s = releaseStartupFIFO(t, fifo, done)
					s.shutdown()
					t.Fatal("startup read waited for a FIFO writer")
				}
				t.Cleanup(s.shutdown)
				s.requireUnread(t)
				s.exerciseHeldWrites(t)
				s.shutdown()
				entryAfter, err := os.Lstat(path)
				if err != nil || !os.SameFile(entryBefore, entryAfter) || entryBefore.Mode() != entryAfter.Mode() {
					t.Fatal("startup/save/shutdown changed the unread canonical entry")
				}
				fifoAfter, err := os.Lstat(fifo)
				if err != nil || !os.SameFile(fifoBefore, fifoAfter) || fifoAfter.Mode()&os.ModeNamedPipe == 0 {
					t.Fatal("startup/save/shutdown changed the unread pipe")
				}
				entries, err := os.ReadDir(dir)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if entry.Name() != filepath.Base(path) && entry.Name() != filepath.Base(fifo) && entry.Name() != ".encryption.key" {
						t.Fatalf("refused inventory left an unexpected file: %s", entry.Name())
					}
				}
			})
		}
	}
}
