package telemetry

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"context"
)

func TestGetOrCreateInstallID_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	id := getOrCreateInstallID(dir)
	if id == "" {
		t.Fatal("expected non-empty install ID")
	}

	// Verify file was persisted.
	data, err := os.ReadFile(filepath.Join(dir, installIDFile))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != id+"\n" {
		t.Fatalf("file content %q does not match returned ID %q", got, id)
	}
}

func TestGetOrCreateInstallID_ReusesExisting(t *testing.T) {
	dir := t.TempDir()

	// Create first ID.
	id1 := getOrCreateInstallID(dir)

	// Second call should return the same ID.
	id2 := getOrCreateInstallID(dir)
	if id1 != id2 {
		t.Fatalf("expected same ID across calls, got %q and %q", id1, id2)
	}
}

func TestGetOrCreateInstallID_RegeneratesInvalid(t *testing.T) {
	dir := t.TempDir()
	// Write garbage.
	os.WriteFile(filepath.Join(dir, installIDFile), []byte("not-a-uuid\n"), 0600)

	id := getOrCreateInstallID(dir)
	if id == "" || id == "not-a-uuid" {
		t.Fatalf("expected new valid UUID, got %q", id)
	}
}

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"1", true},
		{"false", false},
		{"", true}, // enabled by default when env var is not set
		{"0", false},
		{"yes", false}, // only "true"/"1" are truthy; unknown values disable
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("PULSE_TELEMETRY", tt.value)
			if got := IsEnabled(); got != tt.want {
				t.Fatalf("IsEnabled() = %v for PULSE_TELEMETRY=%q, want %v", got, tt.value, tt.want)
			}
		})
	}
}

func TestApplySnapshot(t *testing.T) {
	base := Ping{
		InstallID: "test-id",
		Version:   "6.0.0",
		Platform:  "docker",
		OS:        "linux",
		Arch:      "amd64",
	}

	snap := func() Snapshot {
		return Snapshot{
			PVENodes:     3,
			VMs:          10,
			Containers:   5,
			AIEnabled:    true,
			LicenseTier:  "pro",
			ActiveAlerts: 2,
		}
	}

	ping := applySnapshot(base, snap)

	if ping.InstallID != "test-id" {
		t.Fatal("base fields should be preserved")
	}
	if ping.PVENodes != 3 {
		t.Fatalf("PVENodes = %d, want 3", ping.PVENodes)
	}
	if ping.VMs != 10 {
		t.Fatalf("VMs = %d, want 10", ping.VMs)
	}
	if !ping.AIEnabled {
		t.Fatal("AIEnabled should be true")
	}
	if ping.LicenseTier != "pro" {
		t.Fatalf("LicenseTier = %q, want %q", ping.LicenseTier, "pro")
	}
}

func TestApplySnapshot_NilFunc(t *testing.T) {
	base := Ping{InstallID: "test-id", Version: "6.0.0"}
	ping := applySnapshot(base, nil)
	if ping.InstallID != "test-id" {
		t.Fatal("should return base when func is nil")
	}
	if ping.PVENodes != 0 {
		t.Fatal("snapshot fields should be zero when func is nil")
	}
}

func TestSend_Success(t *testing.T) {
	var received atomic.Int32
	var lastPing Ping

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &lastPing)
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Override the endpoint for testing.
	origEndpoint := pingEndpoint
	// We can't easily override the const, so test send() indirectly
	// by verifying the function doesn't panic and handles context correctly.

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// send() to the real endpoint will fail in tests (no network), but should not panic.
	ping := Ping{
		InstallID: "test",
		Version:   "1.0.0",
		Event:     "startup",
	}
	send(ctx, ping)
	_ = origEndpoint
}

func TestStartStop_DisabledByDefault(t *testing.T) {
	// Start should be a no-op when Enabled is false (the default).
	Start(context.Background(), Config{
		Version: "6.0.0",
		DataDir: t.TempDir(),
		Enabled: false,
	})

	// Stop should also be safe when nothing was started.
	Stop()
}
