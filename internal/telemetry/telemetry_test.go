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

	"github.com/google/uuid"
)

func TestGetOrCreateInstallID_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)
	id := getOrCreateInstallIDAt(dir, now)
	if id == "" {
		t.Fatal("expected non-empty install ID")
	}

	// Verify file was persisted.
	data, err := os.ReadFile(filepath.Join(dir, installIDFile))
	if err != nil {
		t.Fatal(err)
	}
	record := decodeInstallIDRecord(t, data)
	if record.InstallID != id {
		t.Fatalf("persisted install ID = %q, want %q", record.InstallID, id)
	}
	if !record.IssuedAt.Equal(now) {
		t.Fatalf("persisted issued_at = %v, want %v", record.IssuedAt, now)
	}
}

func TestGetOrCreateInstallID_ReusesExisting(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)

	// Create first ID.
	id1 := getOrCreateInstallIDAt(dir, now)

	// Second call should return the same ID.
	id2 := getOrCreateInstallIDAt(dir, now.Add(7*24*time.Hour))
	if id1 != id2 {
		t.Fatalf("expected same ID across calls, got %q and %q", id1, id2)
	}
}

func TestGetOrCreateInstallID_RegeneratesInvalid(t *testing.T) {
	dir := t.TempDir()
	// Write garbage.
	os.WriteFile(filepath.Join(dir, installIDFile), []byte("not-a-uuid\n"), 0600)

	id := getOrCreateInstallIDAt(dir, time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC))
	if id == "" || id == "not-a-uuid" {
		t.Fatalf("expected new valid UUID, got %q", id)
	}
}

func TestGetOrCreateInstallID_RotatesExpiredRecord(t *testing.T) {
	dir := t.TempDir()
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	first := getOrCreateInstallIDAt(dir, start)
	second := getOrCreateInstallIDAt(dir, start.Add(installIDRotationWindow+time.Hour))
	if first == second {
		t.Fatalf("expected install ID rotation after %v, got same value %q", installIDRotationWindow, first)
	}
}

func TestGetOrCreateInstallID_RotatesLegacyPlaintextID(t *testing.T) {
	dir := t.TempDir()
	legacyID := uuid.New().String()
	if err := os.WriteFile(filepath.Join(dir, installIDFile), []byte(legacyID+"\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	now := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)
	rotated := getOrCreateInstallIDAt(dir, now)
	if rotated == legacyID {
		t.Fatalf("expected legacy plaintext install ID to rotate, got same value %q", rotated)
	}

	record := decodeInstallIDRecordFile(t, filepath.Join(dir, installIDFile))
	if record.InstallID != rotated {
		t.Fatalf("persisted install ID = %q, want %q", record.InstallID, rotated)
	}
}

func TestResetInstallID_RewritesRecordImmediately(t *testing.T) {
	dir := t.TempDir()
	start := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	original := getOrCreateInstallIDAt(dir, start)

	resetAt := start.Add(12 * time.Hour)
	rotated, err := resetInstallIDAt(dir, resetAt)
	if err != nil {
		t.Fatalf("resetInstallIDAt: %v", err)
	}
	if rotated == "" {
		t.Fatal("expected non-empty rotated install ID")
	}
	if rotated == original {
		t.Fatalf("expected reset to rotate install ID, got same value %q", rotated)
	}

	record := decodeInstallIDRecordFile(t, filepath.Join(dir, installIDFile))
	if record.InstallID != rotated {
		t.Fatalf("persisted install ID = %q, want %q", record.InstallID, rotated)
	}
	if !record.IssuedAt.Equal(resetAt) {
		t.Fatalf("persisted issued_at = %v, want %v", record.IssuedAt, resetAt)
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
			ActiveAlerts: 2,
			PaidLicense:  true,
			HasAPITokens: true,
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
	if !ping.PaidLicense {
		t.Fatal("PaidLicense should be true")
	}
	if !ping.HasAPITokens {
		t.Fatal("HasAPITokens should be true")
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

func TestBuildPreview_UsesCurrentHeartbeatPayload(t *testing.T) {
	dir := t.TempDir()

	preview, err := BuildPreview(Config{
		Version:  "6.0.0",
		DataDir:  dir,
		IsDocker: true,
		GetSnapshot: func() Snapshot {
			return Snapshot{
				PVENodes:     3,
				VMs:          10,
				ActiveAlerts: 2,
				AIEnabled:    true,
			}
		},
	})
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}

	if preview.Event != "heartbeat" {
		t.Fatalf("preview event = %q, want heartbeat", preview.Event)
	}
	if preview.Platform != "docker" {
		t.Fatalf("preview platform = %q, want docker", preview.Platform)
	}
	if preview.PVENodes != 3 || preview.VMs != 10 || preview.ActiveAlerts != 2 {
		t.Fatalf("preview snapshot = %#v", preview)
	}
	if preview.InstallID == "" {
		t.Fatal("expected preview install ID")
	}

	record := decodeInstallIDRecordFile(t, filepath.Join(dir, installIDFile))
	if record.InstallID != preview.InstallID {
		t.Fatalf("persisted install ID = %q, want %q", record.InstallID, preview.InstallID)
	}
}

func TestSend_Success(t *testing.T) {
	var received atomic.Int32
	var lastPing Ping

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &lastPing)
		received.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	// Override the endpoint for testing.
	origEndpoint := pingEndpoint
	pingEndpoint = ts.URL
	defer func() { pingEndpoint = origEndpoint }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ping := Ping{
		InstallID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Version:   "6.0.0",
		Event:     "startup",
		Platform:  "docker",
		OS:        "linux",
		Arch:      "amd64",
	}
	send(ctx, ping)

	if received.Load() != 1 {
		t.Fatalf("expected 1 request to reach server, got %d", received.Load())
	}
	if lastPing.InstallID != ping.InstallID {
		t.Errorf("install_id = %q, want %q", lastPing.InstallID, ping.InstallID)
	}
	if lastPing.Event != "startup" {
		t.Errorf("event = %q, want %q", lastPing.Event, "startup")
	}
	if lastPing.Version != "6.0.0" {
		t.Errorf("version = %q, want %q", lastPing.Version, "6.0.0")
	}
}

func TestSend_UsesReducedCommercialSignals(t *testing.T) {
	var rawBody []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	origEndpoint := pingEndpoint
	pingEndpoint = ts.URL
	defer func() { pingEndpoint = origEndpoint }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	send(ctx, Ping{
		InstallID:    "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Version:      "6.0.0",
		Event:        "heartbeat",
		Platform:     "binary",
		OS:           "linux",
		Arch:         "amd64",
		PaidLicense:  true,
		HasAPITokens: true,
	})

	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := payload["license_tier"]; ok {
		t.Fatal("legacy license_tier field should not be sent")
	}
	if _, ok := payload["api_tokens"]; ok {
		t.Fatal("legacy api_tokens field should not be sent")
	}
	if got, ok := payload["paid_license"].(bool); !ok || !got {
		t.Fatalf("paid_license = %#v, want true", payload["paid_license"])
	}
	if got, ok := payload["has_api_tokens"].(bool); !ok || !got {
		t.Fatalf("has_api_tokens = %#v, want true", payload["has_api_tokens"])
	}
}

func TestJitteredHeartbeat_WithinBounds(t *testing.T) {
	min := heartbeatInterval - maxHeartbeatJitter
	max := heartbeatInterval + maxHeartbeatJitter

	for i := 0; i < 1000; i++ {
		d := jitteredHeartbeat()
		if d < min || d > max {
			t.Fatalf("jitteredHeartbeat() = %v, want [%v, %v]", d, min, max)
		}
	}
}

func TestJitteredHeartbeat_NotConstant(t *testing.T) {
	seen := make(map[time.Duration]bool)
	for i := 0; i < 100; i++ {
		seen[jitteredHeartbeat()] = true
	}
	if len(seen) < 2 {
		t.Fatal("jitteredHeartbeat() returned the same value 100 times — jitter is not working")
	}
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

func decodeInstallIDRecordFile(t *testing.T, path string) installIDRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return decodeInstallIDRecord(t, data)
}

func decodeInstallIDRecord(t *testing.T, data []byte) installIDRecord {
	t.Helper()
	var record installIDRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("Unmarshal install ID record: %v", err)
	}
	return record
}
