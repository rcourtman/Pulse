package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Pause only the first descriptor write, after its first byte reaches a real
// exclusive regular file. Subsequent queued saves still use ordinary file.Write.
// Capture the actual written bytes before the production close/rename, not a
// fabricated successful snapshot. No elapsed-time assumption opens the barrier.
func pauseAuthSnapshotWrite(t *testing.T, install func(authSnapshotWriteFunc), action func()) func() []byte {
	t.Helper()
	entered := make(chan error, 1)
	release := make(chan struct{})
	done := make(chan struct{})
	var paused atomic.Bool
	var captured []byte
	install(func(file *os.File, data []byte) (int, error) {
		if !paused.CompareAndSwap(false, true) {
			return file.Write(data)
		}
		if len(data) == 0 {
			entered <- errors.New("snapshot fixture requires nonempty data")
			return 0, io.ErrUnexpectedEOF
		}
		n, err := file.Write(data[:1])
		if err == nil && n != 1 {
			err = io.ErrShortWrite
		}
		if err == nil {
			info, statErr := file.Stat()
			if statErr != nil {
				err = statErr
			} else if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
				err = errors.New("paused descriptor is not a private regular file")
			}
		}
		entered <- err
		if err != nil {
			return n, err
		}
		<-release
		rest, err := file.Write(data[1:])
		n += rest
		if err == nil {
			captured, err = os.ReadFile(file.Name())
		}
		if err == nil && !bytes.Equal(captured, data) {
			err = errors.New("real snapshot bytes differ from the selected snapshot")
		}
		return n, err
	})
	go func() { defer close(done); action() }()
	var once sync.Once
	resume := func() []byte {
		t.Helper()
		once.Do(func() {
			close(release)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("snapshot writer did not join after storage resumed")
			}
		})
		return captured
	}
	t.Cleanup(func() { resume() })
	select {
	case err := <-entered:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("save did not reach its owned descriptor write")
	}
	select {
	case <-done:
		t.Error("mutation returned before its snapshot write completed")
	default:
	}
	return resume
}

func checkNoOwnedAuthTemps(t *testing.T, dir, name string) {
	t.Helper()
	leftovers, err := filepath.Glob(filepath.Join(dir, "."+name+".*.tmp"))
	if err != nil || len(leftovers) != 0 {
		t.Fatal("failed publication left an owned temporary snapshot")
	}
}

// This exercises real helper descriptors for partial/failed writes and close
// errors. The original authority must survive, and the next normal save works.
func TestPrivateAuthSnapshotWriteFailure(t *testing.T) {
	for _, fault := range []string{"short-write", "write-error", "close-error", "rename-error"} {
		t.Run(fault, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "authority.json")
			const old, next = "old synthetic authority", "next synthetic authority"
			if err := os.WriteFile(path, []byte(old), 0600); err != nil {
				t.Fatal(err)
			}
			var write authSnapshotWriteFunc
			switch fault {
			case "short-write":
				write = func(f *os.File, data []byte) (int, error) { return f.Write(data[:1]) }
			case "write-error":
				write = func(f *os.File, data []byte) (int, error) { n, _ := f.Write(data[:1]); return n, os.ErrPermission }
			case "close-error":
				write = func(f *os.File, data []byte) (int, error) {
					n, err := f.Write(data)
					if err == nil {
						err = f.Close()
					}
					return n, err
				}
			case "rename-error":
				if err := os.Rename(path, path+".saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			err := writePrivateAuthSnapshot(path, []byte(next), write)
			if err == nil {
				t.Fatal("failed snapshot was acknowledged")
			}
			if fault == "short-write" && !errors.Is(err, io.ErrShortWrite) {
				t.Error("short write lost its failure identity")
			}
			if fault == "write-error" && !errors.Is(err, os.ErrPermission) {
				t.Error("write failure lost its identity")
			}
			if fault == "close-error" && !errors.Is(err, os.ErrClosed) {
				t.Error("close failure lost its identity")
			}
			if fault == "rename-error" {
				if info, err := os.Stat(path); err != nil || !info.IsDir() {
					t.Fatal("failed rename changed its blocking destination")
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(path+".saved", path); err != nil {
					t.Fatal(err)
				}
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != old {
				t.Fatal("failed write changed original authority")
			}
			checkNoOwnedAuthTemps(t, dir, "authority.json")
			if err := writePrivateAuthSnapshot(path, []byte(next), nil); err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(path)
			if err != nil || string(data) != next {
				t.Fatal("normal publication did not recover")
			}
			checkNoOwnedAuthTemps(t, dir, "authority.json")
		})
	}
}

func TestPrivateAuthSnapshotReportsCleanupFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX rename of an open descriptor")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "authority.json")
	const old = "old synthetic authority"
	if err := os.WriteFile(path, []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	var moved string
	cause := errors.New("synthetic write failure")
	err := writePrivateAuthSnapshot(path, []byte("next"), func(file *os.File, data []byte) (int, error) {
		moved = file.Name() + ".saved"
		if err := os.Rename(file.Name(), moved); err != nil {
			return 0, err
		}
		if err := os.Mkdir(file.Name(), 0700); err != nil {
			return 0, err
		}
		if err := os.WriteFile(filepath.Join(file.Name(), "blocker"), []byte("synthetic"), 0600); err != nil {
			return 0, err
		}
		return 0, cause
	})
	if !errors.Is(err, cause) || !bytes.Contains([]byte(fmt.Sprint(err)), []byte("clean up private snapshot")) {
		t.Fatal("cleanup failure hid itself or replaced the original failure")
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != old {
		t.Fatal("failed cleanup changed original authority")
	}
	if info, err := os.Stat(moved); err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("unpublished owned snapshot lost private permissions")
	}
}
