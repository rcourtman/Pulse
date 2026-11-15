//go:build linux

package api

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestAuthSnapshotPublicationIgnoresLegacyFIFO(t *testing.T) {
	for _, kind := range []string{"sessions", "csrf"} {
		t.Run(kind, func(t *testing.T) {
			f := newAuthSnapshotFixture(t, kind)
			legacy := filepath.Join(f.dir, f.name+".tmp")
			if err := syscall.Mkfifo(legacy, 0600); err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{})
			go func() { defer close(done); f.revoke() }()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				// Release a regressed writer without leaving a blocked goroutine.
				// These small snapshots fit the FIFO buffer without a drainer.
				reader, err := os.OpenFile(legacy, os.O_RDWR|syscall.O_NONBLOCK, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer reader.Close()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("regressed FIFO writer did not join")
				}
				t.Fatal("save opened an unowned FIFO instead of an exclusive regular file")
			}
			if info, err := os.Lstat(legacy); err != nil || info.Mode()&os.ModeNamedPipe == 0 {
				t.Fatal("save changed or removed an unowned FIFO")
			}
			checkPrivateAuthSnapshot(t, f)
			f.checkLive(t)
			f.checkReload(t)
		})
	}
}
