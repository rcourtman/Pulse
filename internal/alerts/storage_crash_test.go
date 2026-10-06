package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Unlike an orderly restart, process exit must not get a shutdown checkpoint
// or store Close to rescue a lifecycle transition. Keep the recovery mirror
// deliberately stale to prove SQLite, not the mirror, preserves the incident.
// This tests process interruption, not power loss or installed delivery.
func TestStorageLifecycleAcrossProcessExit(t *testing.T) {
	const helperEnv = "PULSE_TEST_STORAGE_CRASH_PHASE"
	storage := models.Storage{ID: "pbs-crash-backups", Name: "backups", Type: "pbs", Status: "online", Total: 1000, Used: 850, Free: 150, Usage: 85}
	id := canonicalMetricStateID(storage.ID, "usage")
	start := func(t *testing.T, dir string) *Manager {
		t.Helper()
		m := NewManagerWithDataDir(dir)
		m.EnableEventLog()
		if !m.activeStateAuthoritative.Load() {
			t.Fatal("SQLite authority not enabled")
		}
		m.UpdateConfig(AlertConfig{Enabled: true, ActivationState: ActivationActive,
			TimeThresholds: map[string]int{"storage": 0}, StorageDefault: HysteresisThreshold{Trigger: 80, Clear: 70}})
		return m
	}
	observe := func(m *Manager, s models.Storage) {
		for range 5 {
			m.CheckStorage(s)
		}
	}
	if phase := os.Getenv(helperEnv); phase != "" {
		// Use test output rather than application logging: a stuck child must
		// identify its last application boundary even when logging is disabled.
		stage := func(name string) { t.Logf("child phase=%s stage=%s", phase, name) }
		stage("validate")
		if phase != "fired" && phase != "resolved" {
			t.Fatal("invalid child phase")
		}
		stage("manager-start")
		m := start(t, os.Getenv("PULSE_TEST_STORAGE_CRASH_DIR"))
		stage("manager-ready")
		if phase == "resolved" {
			stage("seed-firing")
			observe(m, storage)
			testRequireActiveAlert(t, m, id)
		}
		stage("initial-checkpoint")
		if err := m.SaveActiveAlerts(); err != nil {
			t.Fatal(err)
		}
		// Freeze checkpoints, leaving either an empty or a firing JSON mirror.
		// Lifecycle commits must remain independent of periodic/async saves.
		stage("checkpoint-lock")
		m.saveMu.Lock()
		stage("checkpoint-locked")
		if phase == "resolved" {
			storage.Used, storage.Free, storage.Usage = 0, 1000, 0
		}
		stage("lifecycle-observation")
		observe(m, storage)
		stage("lifecycle-observed")
		if testHasActiveAlert(t, m, id) != (phase == "fired") {
			t.Fatal("child did not reach expected lifecycle state")
		}
		stage("process-exit")
		os.Exit(0) // Intentionally bypass all cleanups, Stop and store Close.
	}

	for _, phase := range []string{"fired", "resolved"} {
		t.Run(phase, func(t *testing.T) {
			dir := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestStorageLifecycleAcrossProcessExit$", "-test.v")
			cmd.Env = append(os.Environ(), helperEnv+"="+phase, "PULSE_TEST_STORAGE_CRASH_DIR="+dir)
			output, err := storageProcessExitChildResult(ctx, cmd, phase)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("child output:\n%s", output)
			mirror, err := os.ReadFile(filepath.Join(dir, "alerts", "active-alerts.json"))
			if err != nil {
				t.Fatal(err)
			}
			var stale []*Alert
			if err := json.Unmarshal(mirror, &stale); err != nil {
				t.Fatal(err)
			}
			wantStale := 0
			if phase == "resolved" {
				wantStale = 1
			}
			if len(stale) != wantStale {
				t.Fatalf("recovery mirror was not stale: %s", mirror)
			}
			m := start(t, dir)
			t.Cleanup(m.Stop)
			if testHasActiveAlert(t, m, id) != (phase == "fired") {
				t.Fatal("process exit lost firing incident or resurrected resolved incident")
			}
			events, err := m.AlertEvents(eventlog.Filter{Types: []string{eventlog.TypeFired, eventlog.TypeResolved}})
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if phase == "resolved" {
				want = 2
			}
			counts := make(map[string]int)
			for _, event := range events {
				counts[event.Type]++
			}
			if counts[eventlog.TypeFired] != 1 || counts[eventlog.TypeResolved] != want-1 {
				t.Fatalf("unexpected lifecycle history: %v", counts)
			}
			if len(events) != want {
				t.Fatalf("durable lifecycle events = %d, want %d", len(events), want)
			}
			missing := storage
			missing.Total, missing.Used, missing.Free, missing.Usage = 0, 0, 0, 0
			observe(m, missing)
			if testHasActiveAlert(t, m, id) != (phase == "fired") {
				t.Fatal("missing post-restart counters changed the incident state")
			}
		})
	}
}

// Keep the command failure and actual context result separate. In particular,
// "signal: killed" alone is not evidence that this harness's deadline fired.
func storageProcessExitChildResult(ctx context.Context, cmd *exec.Cmd, phase string) ([]byte, error) {
	started := time.Now()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("child phase=%s failed after %s (context=%v): %w\n%s",
			phase, time.Since(started), ctx.Err(), errors.Join(err, ctx.Err()), output)
	}
	return output, nil
}

// These child controls use no Manager or database. They prove timeout and
// nonzero-exit diagnostics without reenacting a persistence failure or making
// the real lifecycle test's deadline more permissive.
func TestStorageProcessExitChildDiagnostics(t *testing.T) {
	const helperEnv = "PULSE_TEST_STORAGE_CHILD_DIAGNOSTIC"
	if mode := os.Getenv(helperEnv); mode != "" {
		fmt.Fprintf(os.Stderr, "helper-stage=%s\n", mode)
		switch mode {
		case "timeout":
			<-time.After(time.Minute)
		case "exit":
			os.Exit(17)
		case "success":
			os.Exit(0)
		default:
			t.Fatal("invalid diagnostic mode")
		}
		return
	}
	for _, mode := range []string{"timeout", "exit", "success"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestStorageProcessExitChildDiagnostics$")
			cmd.Env = append(os.Environ(), helperEnv+"="+mode)
			output, err := storageProcessExitChildResult(ctx, cmd, mode)
			if !strings.Contains(string(output), "helper-stage="+mode) {
				t.Fatalf("missing executed-child stage: %q; error: %v", output, err)
			}
			switch mode {
			case "timeout":
				if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "child phase=timeout") || !strings.Contains(err.Error(), "helper-stage=timeout") {
					t.Fatalf("timeout lost context, phase or stage: %v", err)
				}
			case "exit":
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 17 || !strings.Contains(err.Error(), "context=<nil>") || !strings.Contains(err.Error(), "helper-stage=exit") {
					t.Fatalf("nonzero exit lost command result or invented a deadline: %v", err)
				}
			case "success":
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
