package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const appriseOutputBlockBytes = 32 * 1024
const appriseOutputPrivateMarker = "synthetic-apprise-private-output"

// The installed CLI is not needed: the real default executor starts an owned
// process that writes to both OS pipes. It never contacts a provider.
func TestAppriseOutputProcessHelper(t *testing.T) {
	if os.Getenv("PULSE_APPRISE_OUTPUT_CHILD") != "1" {
		return
	}
	if encoded := os.Getenv("PULSE_APPRISE_OUTPUT_ARGS"); encoded != "" {
		var expected []string
		if json.Unmarshal([]byte(encoded), &expected) != nil {
			os.Exit(41)
		}
		separator := slices.Index(os.Args, "--")
		if separator < 0 || !slices.Equal(os.Args[separator+1:], expected) {
			os.Exit(42)
		}
	}
	blocks, err := strconv.Atoi(os.Getenv("PULSE_APPRISE_OUTPUT_BLOCKS"))
	if err != nil {
		os.Exit(43)
	}
	block := bytes.Repeat([]byte(appriseOutputPrivateMarker), appriseOutputBlockBytes/len(appriseOutputPrivateMarker)+1)[:appriseOutputBlockBytes]
	var writers sync.WaitGroup
	for _, stream := range []*os.File{os.Stdout, os.Stderr} {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for range blocks {
				if n, err := stream.Write(block); err != nil || n != len(block) {
					os.Exit(44)
				}
			}
		}()
	}
	writers.Wait()
	if os.WriteFile(os.Getenv("PULSE_APPRISE_OUTPUT_READY"), []byte("output complete"), 0600) != nil {
		os.Exit(45)
	}
	if os.Getenv("PULSE_APPRISE_OUTPUT_HOLD") == "1" {
		for {
			time.Sleep(time.Hour)
		}
	}
	exit, err := strconv.Atoi(os.Getenv("PULSE_APPRISE_OUTPUT_EXIT"))
	if err != nil {
		os.Exit(46)
	}
	os.Exit(exit)
}

func appriseOutputProcessFixture(t *testing.T, blocks, exit int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the owned process launcher uses /bin/sh")
	}
	dir := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(dir, "ready")
	t.Setenv("PATH", dir)
	t.Setenv("PULSE_APPRISE_OUTPUT_CHILD", "1")
	t.Setenv("PULSE_APPRISE_OUTPUT_BINARY", binary)
	t.Setenv("PULSE_APPRISE_OUTPUT_BLOCKS", strconv.Itoa(blocks))
	t.Setenv("PULSE_APPRISE_OUTPUT_EXIT", strconv.Itoa(exit))
	t.Setenv("PULSE_APPRISE_OUTPUT_READY", ready)
	t.Setenv("PULSE_APPRISE_OUTPUT_HOLD", "")
	t.Setenv("PULSE_APPRISE_OUTPUT_ARGS", "")
	launcher := "#!/bin/sh\nexec \"$PULSE_APPRISE_OUTPUT_BINARY\" -test.run=^TestAppriseOutputProcessHelper$ -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "apprise"), []byte(launcher), 0700); err != nil {
		t.Fatal(err)
	}
	return ready
}

func TestDefaultAppriseExecCountsOutputWithoutRetainingContent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		blocks int
		exit   int
	}{
		{"silent-success", 0, 0},
		{"mixed-large-success", 256, 0},
		{"mixed-large-exit-seven", 256, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			appriseOutputProcessFixture(t, tc.blocks, tc.exit)
			args := []string{"-t", "synthetic title", "-b", "synthetic body", "json://synthetic/one value"}
			encoded, err := json.Marshal(args)
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("PULSE_APPRISE_OUTPUT_ARGS", string(encoded))
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			count, err := defaultAppriseExec(ctx, args)
			runtime.ReadMemStats(&after)
			want := int64(2 * tc.blocks * appriseOutputBlockBytes)
			if count != want {
				t.Fatalf("counted %d stdout/stderr bytes, want %d", count, want)
			}
			if tc.exit == 0 {
				if err != nil {
					t.Fatalf("output changed successful exit: %v", err)
				}
			} else {
				var native *exec.ExitError
				if !errors.As(err, &native) || native.ExitCode() != tc.exit {
					t.Fatalf("native exit identity changed: %v", err)
				}
				if len(native.Stderr) != 0 {
					t.Fatal("native exit retained private stderr")
				}
			}
			t.Logf("counted %d bytes without returning content; parent allocated %d bytes", count, after.TotalAlloc-before.TotalAlloc)
		})
	}
}

func TestDefaultAppriseExecPreservesStartFailures(t *testing.T) {
	t.Run("missing-executable", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("the executable fixture is Unix-specific")
		}
		t.Setenv("PATH", t.TempDir())
		count, err := defaultAppriseExec(context.Background(), nil)
		if count != 0 || !errors.Is(err, exec.ErrNotFound) {
			t.Fatalf("missing executable: count=%d, err=%v", count, err)
		}
	})
	t.Run("cancelled-before-start", func(t *testing.T) {
		appriseOutputProcessFixture(t, 0, 0)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		count, err := defaultAppriseExec(ctx, nil)
		if count != 0 || !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled start: count=%d, err=%v", count, err)
		}
	})
}

func TestDefaultAppriseExecPreservesCancellationAfterOutput(t *testing.T) {
	ready := appriseOutputProcessFixture(t, 1, 0)
	t.Setenv("PULSE_APPRISE_OUTPUT_HOLD", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	type result struct {
		count int64
		err   error
	}
	done := make(chan result, 1)
	go func() {
		count, err := defaultAppriseExec(ctx, nil)
		done <- result{count, err}
	}()
	joined := false
	t.Cleanup(func() {
		cancel()
		if !joined {
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Error("owned CLI process did not join during cleanup")
			}
		}
	})
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("owned CLI process did not finish its output")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	select {
	case got := <-done:
		joined = true
		var native *exec.ExitError
		if got.count != 2*appriseOutputBlockBytes || !errors.As(got.err, &native) || native.ProcessState.Success() {
			t.Fatalf("cancelled running process lost output count or native error: %d, %v", got.count, got.err)
		}
		if len(native.Stderr) != 0 {
			t.Fatal("cancelled process retained private stderr")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CLI did not stop after context cancellation")
	}
}

func TestAppriseCLIOutputCountsAcrossSenders(t *testing.T) {
	for _, kind := range []string{"firing", "resolved", "test"} {
		for _, exit := range []int{0, 7} {
			t.Run(kind+"/exit="+strconv.Itoa(exit), func(t *testing.T) {
				appriseOutputProcessFixture(t, 8, exit)
				captured := captureAppriseLogs(t)
				err := sendAppriseForConfidentiality(&NotificationManager{}, AppriseConfig{
					Enabled: true, Mode: AppriseModeCLI, TimeoutSeconds: 5,
					Targets: []string{"json://synthetic-one", "json://synthetic-two"},
				}, kind)
				if (err != nil) != (exit != 0) {
					t.Fatalf("sender verdict changed: %v", err)
				}
				if err != nil {
					var native *exec.ExitError
					if !errors.As(err, &native) || native.ExitCode() != exit || ClassifyNotificationFailureError(err) != NotificationFailureUnknown {
						t.Fatalf("sender lost native cause/class: %v", err)
					}
				}
				if strings.Contains(captured.String(), appriseOutputPrivateMarker) {
					t.Fatal("CLI content reached diagnostics")
				}
				found := false
				for _, line := range strings.Split(captured.String(), "\n") {
					var event map[string]any
					if json.Unmarshal([]byte(line), &event) == nil && event["outputBytes"] != nil {
						found = true
						if event["outputBytes"] != float64(16*appriseOutputBlockBytes) || event["targetCount"] != float64(2) {
							t.Fatalf("CLI diagnostic counts changed: %v", event)
						}
					}
				}
				if !found {
					t.Fatal("structured CLI output count was lost")
				}
			})
		}
	}
}

func TestAppriseOutputCounterDoesNotAllocate(t *testing.T) {
	var counter appriseOutputCounter
	block := make([]byte, appriseOutputBlockBytes)
	allocs := testing.AllocsPerRun(100, func() {
		if n, err := counter.Write(block); n != len(block) || err != nil {
			t.Fatalf("counter write: %d, %v", n, err)
		}
	})
	if allocs != 0 {
		t.Fatalf("counting output allocated %v times per write", allocs)
	}
}

func TestAppriseOutputCounterConcurrentWriters(t *testing.T) {
	var counter appriseOutputCounter
	var writers sync.WaitGroup
	block := make([]byte, 1024)
	for range 8 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for range 128 {
				_, _ = counter.Write(block)
			}
		}()
	}
	writers.Wait()
	if count := counter.bytes.Load(); count != 8*128*1024 {
		t.Fatalf("counted %d bytes, want %d", count, 8*128*1024)
	}
}
