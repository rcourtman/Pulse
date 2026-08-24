package configapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func nodeTestTallyCounts(t *testing.T, dataPath string) (attempts int, failures int) {
	t.Helper()
	tally, err := config.NewConfigPersistence(dataPath).LoadNodeTestTally()
	if err != nil {
		t.Fatalf("LoadNodeTestTally: %v", err)
	}
	since := time.Now().UTC().AddDate(0, 0, -30)
	return tally.AttemptsSince(since), tally.FailuresSince(since)
}

func postNodeConnectionTest(t *testing.T, h *ConfigHandlers, body map[string]string) {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest("POST", "/api/config/nodes/test-connection", bytes.NewBuffer(encoded))
	h.HandleTestConnection(httptest.NewRecorder(), req)
}

// A request that never carried a target and credentials was not an attempt to
// reach a node, so counting it would inflate the failure share this counter
// exists to measure.
func TestNodeTestTallyIgnoresRequestsWithoutATarget(t *testing.T) {
	dataPath := t.TempDir()
	h := newTestConfigHandlers(t, &config.Config{DataPath: dataPath})

	for _, body := range []map[string]string{
		{"type": "pve"},
		{"type": "pve", "host": "10.0.0.1"},
		{"type": "unknown", "host": "10.0.0.1", "user": "root@pam", "password": "x"},
	} {
		postNodeConnectionTest(t, h, body)
	}

	attempts, failures := nodeTestTallyCounts(t, dataPath)
	if attempts != 0 || failures != 0 {
		t.Fatalf("targetless requests tallied: attempts=%d failures=%d, want 0 and 0", attempts, failures)
	}
}

// The other side of that boundary: someone who typed an unusable host and got
// an error did attempt to reach a node, and that attempt failed. Excluding it
// would hide exactly the population this counter exists to find.
func TestNodeTestTallyCountsUnusableHostAsFailedAttempt(t *testing.T) {
	dataPath := t.TempDir()
	h := newTestConfigHandlers(t, &config.Config{DataPath: dataPath})

	postNodeConnectionTest(t, h, map[string]string{
		"type":     "pve",
		"host":     "://invalid-url",
		"user":     "root@pam",
		"password": "password",
	})

	attempts, failures := nodeTestTallyCounts(t, dataPath)
	if attempts != 1 || failures != 1 {
		t.Fatalf("attempts=%d failures=%d, want 1 and 1", attempts, failures)
	}
}

// An unreachable target is the case the counter exists for. proxmox.NewClient
// authenticates eagerly, so this fails during client construction rather than
// on a later call, and it must still be counted.
func TestNodeTestTallyCountsUnreachableTargetAsFailure(t *testing.T) {
	dataPath := t.TempDir()
	h := newTestConfigHandlers(t, &config.Config{DataPath: dataPath})

	postNodeConnectionTest(t, h, map[string]string{
		"type":     "pve",
		"host":     "127.0.0.1:1",
		"user":     "root@pam",
		"password": "password",
	})

	attempts, failures := nodeTestTallyCounts(t, dataPath)
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if failures != 1 {
		t.Fatalf("failures = %d, want 1", failures)
	}
}

// The success paths write a body without setting a status explicitly, so the
// recorder must treat an unset status as success rather than failure. Driving a
// real successful test would need a live Proxmox endpoint, so the mapping is
// asserted directly.
func TestNodeTestOutcomeRecorderTreatsImplicitStatusAsSuccess(t *testing.T) {
	rec := newNodeTestOutcomeRecorder(httptest.NewRecorder())
	if rec.failed() {
		t.Fatal("implicit 200 reported as failure")
	}
	if _, err := rec.Write([]byte(`{"status":"success"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if rec.failed() {
		t.Fatal("body write without WriteHeader reported as failure")
	}

	rec.WriteHeader(http.StatusBadRequest)
	if !rec.failed() {
		t.Fatal("400 not reported as failure")
	}
}
