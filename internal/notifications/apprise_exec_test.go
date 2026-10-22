package notifications

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

const appriseFixtureEnv = "PULSE_APPRISE_EXEC_FIXTURE"

var appriseFixtureArgs = []string{"-t", "synthetic title", "-b", "private-alert-body", "tgram://synthetic-secret/topic"}

// The test executable stands in for the fixed apprise binary. It exercises the
// real OS pipes and exit/cancellation path without installing a provider CLI,
// contacting a destination or starting any child/grandchild process.
func TestAppriseExecFixture(t *testing.T) {
	mode := os.Getenv(appriseFixtureEnv)
	if mode == "" {
		return
	}
	separator := slices.Index(os.Args, "--")
	if separator == -1 || !slices.Equal(os.Args[separator+1:], appriseFixtureArgs) {
		os.Exit(91)
	}
	switch mode {
	case "silent":
		os.Exit(0)
	case "wait":
		if _, err := io.WriteString(os.Stdout, "ready\n"); err != nil {
			os.Exit(92)
		}
		time.Sleep(time.Hour) // CommandContext must terminate this exact process.
		os.Exit(93)
	case "success", "failure":
		chunk := bytes.Repeat([]byte("synthetic-secret"), 4096) // 64 KiB
		for range 128 {
			for _, stream := range []*os.File{os.Stdout, os.Stderr} {
				if n, err := stream.Write(chunk); err != nil || n != len(chunk) {
					os.Exit(94)
				}
			}
		}
		if mode == "failure" {
			os.Exit(7)
		}
		os.Exit(0)
	default:
		os.Exit(95)
	}
}

func installAppriseExecFixture(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	name := "apprise"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, name)
	if err := os.Link(executable, target); err != nil {
		// Temp directories need not share a filesystem; Windows may also
		// refuse hard links. Copy only our own test executable in that case.
		input, err := os.Open(executable)
		if err != nil {
			t.Fatal(err)
		}
		defer input.Close()
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
		if err != nil {
			t.Fatal(err)
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
}

func TestDefaultAppriseExecDrainsWithoutCapture(t *testing.T) {
	installAppriseExecFixture(t)
	args := append([]string{"-test.run=^TestAppriseExecFixture$", "--"}, appriseFixtureArgs...)
	for _, mode := range []string{"silent", "success", "failure"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv(appriseFixtureEnv, mode)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			outputBytes, err := defaultAppriseExec(ctx, args)
			wantBytes := int64(16 * 1024 * 1024)
			if mode == "silent" {
				wantBytes = 0
			}
			if outputBytes != wantBytes {
				t.Fatalf("output bytes = %d; want %d from both streams", outputBytes, wantBytes)
			}
			if mode != "failure" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
				t.Fatalf("exit = %v; want typed exit status 7", err)
			}
			if len(exitErr.Stderr) != 0 || strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatal("provider output was retained in the process error")
			}
		})
	}
	t.Run("deadline", func(t *testing.T) {
		t.Setenv(appriseFixtureEnv, "wait")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		started := time.Now()
		outputBytes, err := defaultAppriseExec(ctx, args)
		if err == nil || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("process error = %v; context = %v; want expired delivery", err, ctx.Err())
		}
		if outputBytes != int64(len("ready\n")) || time.Since(started) > 10*time.Second {
			t.Fatalf("deadline did not retire a started process: bytes=%d elapsed=%v", outputBytes, time.Since(started))
		}
	})
	t.Run("already-cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		outputBytes, err := defaultAppriseExec(ctx, args)
		if outputBytes != 0 || !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled process = %d, %v", outputBytes, err)
		}
	})
	t.Run("missing-executable", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		outputBytes, err := defaultAppriseExec(context.Background(), args)
		if outputBytes != 0 || !errors.Is(err, exec.ErrNotFound) {
			t.Fatalf("missing executable = %d, %v", outputBytes, err)
		}
	})
}

func TestAppriseOutputCounter(t *testing.T) {
	var output appriseOutputCounter
	chunk := make([]byte, 64*1024)
	if n, err := output.Write(nil); n != 0 || err != nil {
		t.Fatalf("empty write = %d, %v", n, err)
	}
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(func() {
			for range 1024 {
				if n, err := output.Write(chunk); n != len(chunk) || err != nil {
					t.Errorf("write = %d, %v", n, err)
				}
			}
		})
	}
	workers.Wait()
	if got, want := output.bytes.Load(), int64(4*1024*len(chunk)); got != want {
		t.Fatalf("concurrent byte count = %d; want %d", got, want)
	}
	if got := testing.AllocsPerRun(100, func() { _, _ = output.Write(chunk) }); got != 0 {
		t.Fatalf("counting output allocates %g times per write", got)
	}
}
