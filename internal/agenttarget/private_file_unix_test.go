//go:build unix

package agenttarget

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestObserverPrivateFileRejectsSymlinkAfterPathCheck(t *testing.T) {
	dir := t.TempDir()
	path, target := filepath.Join(dir, "observer.token"), filepath.Join(dir, "replacement.token")
	writePrivateFile(t, path, "original-synthetic-token")
	writePrivateFile(t, target, "replacement-synthetic-token")
	called := false
	data, err := readPrivateRegularFileWithOpener(path, "observer token", maxTokenBytes, func(path string) (*os.File, error) {
		called = true
		if err := os.Rename(path, path+".saved"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Base(target), path); err != nil {
			t.Fatal(err)
		}
		return openPrivateFile(path)
	})
	if !called || err == nil || data != nil {
		t.Fatal("symlink introduced after the path check became authority")
	}
	for file, expected := range map[string]string{path + ".saved": "original-synthetic-token", target: "replacement-synthetic-token"} {
		got, err := os.ReadFile(file)
		if err != nil || string(got) != expected {
			t.Fatal("refusal changed original or linked credential contents")
		}
	}
}

func TestObserverPrivateFileUsesValidatedReplacementHandle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observer.token")
	writePrivateFile(t, path, "old-synthetic-token")
	writePrivateFile(t, path+".new", "new-synthetic-token")
	data, err := readPrivateRegularFileWithOpener(path, "observer token", maxTokenBytes, func(path string) (*os.File, error) {
		if err := os.Rename(path+".new", path); err != nil {
			t.Fatal(err)
		}
		return openPrivateFile(path)
	})
	if err != nil || string(data) != "new-synthetic-token" {
		t.Fatal("safe atomic regular-file replacement was not validated and read")
	}
}

func TestObserverPrivateFileRejectsFIFOAfterPathCheckWithoutWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observer.token")
	writePrivateFile(t, path, "original-synthetic-token")
	// Bound the test as well as the reader. A regression to a blocking open
	// must not leave the test process with a stuck goroutine or require a writer.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestObserverPrivateFileFIFOChild$")
	cmd.Env = append(os.Environ(), "PULSE_TEST_OBSERVER_FIFO="+path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("swapped FIFO refusal did not complete: context=%v exit=%v output=%s", ctx.Err(), err, output)
	}
	got, err := os.ReadFile(path + ".saved")
	if err != nil || string(got) != "original-synthetic-token" {
		t.Fatal("swapped FIFO refusal changed saved original credential")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatal("reader replaced the refused FIFO")
	}
}

func TestObserverPrivateFileFIFOChild(t *testing.T) {
	path := os.Getenv("PULSE_TEST_OBSERVER_FIFO")
	if path == "" {
		t.Skip("bounded parent fixture starts this subprocess with its synthetic path")
	}
	called := false
	data, err := readPrivateRegularFileWithOpener(path, "observer token", maxTokenBytes, func(path string) (*os.File, error) {
		called = true
		if err := os.Rename(path, path+".saved"); err != nil {
			t.Fatal(err)
		}
		if err := unix.Mkfifo(path, 0600); err != nil {
			t.Fatal(err)
		}
		return openPrivateFile(path)
	})
	if !called || err == nil || data != nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatal("non-regular descriptor was read as an observer credential")
	}
}
