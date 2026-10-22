package notifications

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

const apprisePipeFixtureEnv = "PULSE_APPRISE_PIPE_FIXTURE"

// Only our test executable is launched. A synthetic child inherits one or both
// streams, then waits for an owned release file (or its independent lifetime
// cap). No provider, shell, network, process group or external workload is used.
func TestApprisePipeFixture(t *testing.T) {
	mode := os.Getenv(apprisePipeFixtureEnv)
	if mode == "" {
		return
	}
	dir := os.Getenv("PULSE_APPRISE_PIPE_DIRECTORY")
	if mode == "child" {
		if err := os.WriteFile(filepath.Join(dir, "ready"), nil, 0600); err != nil {
			os.Exit(81)
		}
		until := time.Now().Add(12 * time.Second)
		for time.Now().Before(until) {
			if _, err := os.Stat(filepath.Join(dir, "release")); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err := os.WriteFile(filepath.Join(dir, "done"), nil, 0600); err != nil {
			os.Exit(82)
		}
		os.Exit(0)
	}
	separator := slices.Index(os.Args, "--")
	if separator < 0 {
		os.Exit(83)
	}
	args := os.Args[separator+1:]
	if len(args) != 5 || args[0] != "-t" || args[2] != "-b" || args[4] != appriseFixtureArgs[4] {
		os.Exit(84)
	}
	executable, err := os.Executable()
	if err != nil {
		os.Exit(85)
	}
	child := exec.Command(executable, "-test.run=^TestApprisePipeFixture$")
	child.Env = append(os.Environ(), apprisePipeFixtureEnv+"=child")
	streams := os.Getenv("PULSE_APPRISE_PIPE_STREAMS")
	if streams == "stdout" || streams == "both" {
		child.Stdout = os.Stdout
	}
	if streams == "stderr" || streams == "both" {
		child.Stderr = os.Stderr
	}
	if err := child.Start(); err != nil {
		os.Exit(86)
	}
	until := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, "ready")); err == nil {
			break
		}
		if time.Now().After(until) {
			os.Exit(87)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := io.WriteString(os.Stdout, "ready\n"); err != nil {
		os.Exit(88)
	}
	if mode == "deadline" {
		time.Sleep(time.Minute) // The delivery context must kill this parent.
	}
	if mode == "failure" {
		os.Exit(7)
	}
	if mode == "drained" {
		if err := os.WriteFile(filepath.Join(dir, "release"), nil, 0600); err != nil {
			os.Exit(89)
		}
		if err := child.Wait(); err != nil {
			os.Exit(90)
		}
	}
	os.Exit(0)
}

func prepareApprisePipeFixture(t *testing.T, mode, streams string) []string {
	t.Helper()
	installAppriseExecFixture(t)
	dir := t.TempDir()
	t.Setenv(apprisePipeFixtureEnv, mode)
	t.Setenv("PULSE_APPRISE_PIPE_STREAMS", streams)
	t.Setenv("PULSE_APPRISE_PIPE_DIRECTORY", dir)
	// Avoid the race detector's post-main sleep in the re-executed fixture.
	t.Setenv("GORACE", os.Getenv("GORACE")+" atexit_sleep_ms=0")
	t.Cleanup(func() {
		if err := os.WriteFile(filepath.Join(dir, "release"), nil, 0600); err != nil {
			t.Error(err)
		}
		until := time.Now().Add(5 * time.Second)
		for time.Now().Before(until) {
			if _, err := os.Stat(filepath.Join(dir, "done")); err == nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Error("synthetic inherited-pipe child did not acknowledge release")
	})
	return append([]string{"-test.run=^TestApprisePipeFixture$", "--"}, appriseFixtureArgs...)
}

func TestDefaultAppriseExecBoundsInheritedPipes(t *testing.T) {
	for _, tc := range []struct {
		name, mode, streams string
	}{
		{"successful-parent-stdout", "success", "stdout"},
		{"successful-parent-stderr", "success", "stderr"},
		{"failed-parent-both", "failure", "both"},
		{"deadline-parent-both", "deadline", "both"},
		{"normally-drained-child", "drained", "both"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := prepareApprisePipeFixture(t, tc.mode, tc.streams)
			timeout := 20 * time.Second
			if tc.mode == "deadline" {
				timeout = 2 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			started := time.Now()
			outputBytes, err := defaultAppriseExec(ctx, args)
			elapsed := time.Since(started)
			if outputBytes != int64(len("ready\n")) {
				t.Errorf("output bytes = %d; want only parent's readiness marker", outputBytes)
			}
			if elapsed > 5*time.Second {
				t.Errorf("inherited pipes held delivery for %v; want bounded completion", elapsed)
			}
			switch tc.mode {
			case "success":
				if !errors.Is(err, exec.ErrWaitDelay) || ctx.Err() != nil {
					t.Errorf("successful parent with unclosed pipes = %v, context %v; want ErrWaitDelay", err, ctx.Err())
				}
			case "failure":
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 || len(exitErr.Stderr) != 0 {
					t.Errorf("failed parent = %v; want output-free exit status 7", err)
				}
			case "deadline":
				if err == nil || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
					t.Errorf("deadline parent = %v, context %v; want expired delivery", err, ctx.Err())
				}
			case "drained":
				if err != nil {
					t.Errorf("normally closed inherited streams failed: %v", err)
				}
			}
			t.Logf("mode=%s streams=%s elapsed=%v outputBytes=%d error=%v", tc.mode, tc.streams, elapsed, outputBytes, err)
		})
	}
}
