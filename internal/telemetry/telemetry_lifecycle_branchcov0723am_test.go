package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

// decodeLifecycleRaw reads and JSON-decodes the lifecycle state file WITHOUT
// applying the validActivationStage normalization that the production
// readLifecycleRecord performs. It is used to assert the exact on-disk state
// independently of the function under test.
func decodeLifecycleRaw(t *testing.T, dir string) lifecycleRecord {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, lifecycleStateFile))
	if err != nil {
		t.Fatalf("read lifecycle state: %v", err)
	}
	var rec lifecycleRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("unmarshal lifecycle state: %v", err)
	}
	return rec
}

// lifecycleIsZero reports whether r is the zero lifecycleRecord value.
func lifecycleIsZero(r lifecycleRecord) bool {
	return r.FirstObservedAt.IsZero() &&
		r.FirstMonitoredResourceAt == nil &&
		r.HighestObservedActivation == ""
}

// unwritableDataDir returns a dataDir whose parent component is a regular
// file, so that any os.MkdirAll / write attempt on it fails deterministically.
// The blocking file lives under t.TempDir() and is cleaned up automatically.
func unwritableDataDir(t *testing.T) string {
	t.Helper()
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("create blocker file: %v", err)
	}
	return filepath.Join(blocker, "child")
}

// TestBranchcov0723Am_ReadLifecycleRecord covers every return path of
// readLifecycleRecord: a missing directory, an existing directory with no
// file, a present-but-malformed file, a present file whose activation stage is
// invalid (it must be blanked while the other fields are kept), and a fully
// valid record whose every field round-trips.
func TestBranchcov0723Am_ReadLifecycleRecord(t *testing.T) {
	t.Run("missing directory returns zero record", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "does-not-exist")
		if got := readLifecycleRecord(dir); !lifecycleIsZero(got) {
			t.Fatalf("readLifecycleRecord(missing dir) = %+v, want zero value", got)
		}
	})

	t.Run("existing directory with file absent returns zero record", func(t *testing.T) {
		dir := t.TempDir()
		if got := readLifecycleRecord(dir); !lifecycleIsZero(got) {
			t.Fatalf("readLifecycleRecord(no file) = %+v, want zero value", got)
		}
	})

	t.Run("malformed JSON returns zero record", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, lifecycleStateFile), []byte("{not json"), 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		if got := readLifecycleRecord(dir); !lifecycleIsZero(got) {
			t.Fatalf("readLifecycleRecord(malformed) = %+v, want zero value", got)
		}
	})

	t.Run("invalid activation stage is blanked but other fields kept", func(t *testing.T) {
		dir := t.TempDir()
		firstObs := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
		raw := fmt.Sprintf(`{"first_observed_at":%q,"highest_observed_activation":"totally_bogus"}`, firstObs.Format(time.RFC3339Nano))
		if err := os.WriteFile(filepath.Join(dir, lifecycleStateFile), []byte(raw), 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		got := readLifecycleRecord(dir)
		if !got.FirstObservedAt.Equal(firstObs) {
			t.Fatalf("FirstObservedAt = %v, want %v (must be preserved)", got.FirstObservedAt, firstObs)
		}
		if got.HighestObservedActivation != "" {
			t.Fatalf("HighestObservedActivation = %q, want empty after blanking", got.HighestObservedActivation)
		}
		if got.FirstMonitoredResourceAt != nil {
			t.Fatalf("FirstMonitoredResourceAt = %v, want nil", got.FirstMonitoredResourceAt)
		}
	})

	t.Run("valid record round-trips every field", func(t *testing.T) {
		dir := t.TempDir()
		firstObs := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
		firstMon := time.Date(2026, 1, 16, 8, 0, 0, 0, time.UTC)
		seed := lifecycleRecord{
			FirstObservedAt:           firstObs,
			FirstMonitoredResourceAt:  &firstMon,
			HighestObservedActivation: "monitoring",
		}
		encoded, err := json.Marshal(seed)
		if err != nil {
			t.Fatalf("marshal seed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, lifecycleStateFile), encoded, 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}

		got := readLifecycleRecord(dir)
		if !got.FirstObservedAt.Equal(firstObs) {
			t.Fatalf("FirstObservedAt = %v, want %v", got.FirstObservedAt, firstObs)
		}
		if got.HighestObservedActivation != "monitoring" {
			t.Fatalf("HighestObservedActivation = %q, want monitoring", got.HighestObservedActivation)
		}
		if got.FirstMonitoredResourceAt == nil {
			t.Fatal("FirstMonitoredResourceAt = nil, want non-nil pointer")
		}
		if !got.FirstMonitoredResourceAt.Equal(firstMon) {
			t.Fatalf("FirstMonitoredResourceAt = %v, want %v", *got.FirstMonitoredResourceAt, firstMon)
		}
	})
}

// TestBranchcov0723Am_WriteLifecycleRecord covers the happy path (writing into
// a fresh directory, which must be created) with a full round-trip read-back,
// and the os.MkdirAll failure path when the dataDir's parent is a regular file.
func TestBranchcov0723Am_WriteLifecycleRecord(t *testing.T) {
	t.Run("happy path round-trips every field and creates the directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nested")
		firstObs := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
		firstMon := time.Date(2026, 2, 4, 5, 6, 7, 0, time.UTC)
		seed := lifecycleRecord{
			FirstObservedAt:           firstObs,
			FirstMonitoredResourceAt:  &firstMon,
			HighestObservedActivation: "outcome_observed",
		}
		if err := writeLifecycleRecord(dir, seed); err != nil {
			t.Fatalf("writeLifecycleRecord: %v", err)
		}

		got := decodeLifecycleRaw(t, dir)
		if !got.FirstObservedAt.Equal(firstObs) {
			t.Fatalf("FirstObservedAt = %v, want %v", got.FirstObservedAt, firstObs)
		}
		if got.HighestObservedActivation != "outcome_observed" {
			t.Fatalf("HighestObservedActivation = %q, want outcome_observed", got.HighestObservedActivation)
		}
		if got.FirstMonitoredResourceAt == nil || !got.FirstMonitoredResourceAt.Equal(firstMon) {
			t.Fatalf("FirstMonitoredResourceAt = %v, want %v", got.FirstMonitoredResourceAt, firstMon)
		}
		if _, err := os.Stat(filepath.Join(dir, lifecycleStateFile)); err != nil {
			t.Fatalf("lifecycle file not created under fresh nested dir: %v", err)
		}
	})

	t.Run("dataDir whose parent is a regular file returns non-nil error", func(t *testing.T) {
		dir := unwritableDataDir(t)
		if err := writeLifecycleRecord(dir, lifecycleRecord{}); err == nil {
			t.Fatal("writeLifecycleRecord with unwritable dataDir = nil error, want non-nil")
		}
	})

	t.Run("read only data dir fails at temp file creation", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "read-only")
		if err := os.MkdirAll(sub, 0700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.Chmod(sub, 0500); err != nil {
			t.Fatalf("chmod read-only: %v", err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(sub, 0700); err != nil {
				t.Errorf("restore dir mode: %v", err)
			}
		})
		// MkdirAll on the existing directory is a no-op even when read-only,
		// so the failure surfaces at os.CreateTemp inside writeLifecycleRecord.
		err := writeLifecycleRecord(sub, lifecycleRecord{HighestObservedActivation: "started"})
		if err == nil {
			t.Skip("temp file creation succeeded despite read-only dir (e.g. running as root); CreateTemp error arm not exercisable here")
		}
		// No final lifecycle file should exist.
		if _, err := os.Stat(filepath.Join(sub, lifecycleStateFile)); err == nil {
			t.Fatal("lifecycle file must not exist after a failed write")
		}
	})
}

// TestBranchcov0723Am_ApplyLifecycle covers the nil-ping no-op, the first-ever
// ping with and without monitored resources (which selects the
// present-at-first-observation vs not_observed time-to-monitoring arms), a
// subsequent ping that preserves FirstObservedAt while recording the first
// monitored resource later, a subsequent lower-stage ping that preserves the
// highest observed activation, and the outcome-observed signal path.
func TestBranchcov0723Am_ApplyLifecycle(t *testing.T) {
	t.Run("nil ping is a no-op and writes nothing", func(t *testing.T) {
		dir := t.TempDir()
		applyLifecycle(nil, dir, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		if _, err := os.Stat(filepath.Join(dir, lifecycleStateFile)); !os.IsNotExist(err) {
			t.Fatalf("nil ping must not create a lifecycle file; stat err = %v", err)
		}
	})

	t.Run("first ever ping without monitoring records first observed as not observed", func(t *testing.T) {
		dir := t.TempDir()
		now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
		ping := &Ping{AuthConfigured: true}
		applyLifecycle(ping, dir, now)

		if ping.ActivationStage != "secured" {
			t.Fatalf("ActivationStage = %q, want secured", ping.ActivationStage)
		}
		if ping.MonitoringActive {
			t.Fatal("MonitoringActive = true, want false (no monitored resources)")
		}
		if ping.OutcomeObserved30d {
			t.Fatal("OutcomeObserved30d = true, want false")
		}
		if ping.KnownInstallAgeBucket != "under_1d" {
			t.Fatalf("KnownInstallAgeBucket = %q, want under_1d", ping.KnownInstallAgeBucket)
		}
		if ping.EstateSizeBucket != "empty" {
			t.Fatalf("EstateSizeBucket = %q, want empty", ping.EstateSizeBucket)
		}
		if ping.TimeToFirstMonitoredResourceBucket != "not_observed" {
			t.Fatalf("TimeToFirstMonitoredResourceBucket = %q, want not_observed", ping.TimeToFirstMonitoredResourceBucket)
		}

		got := decodeLifecycleRaw(t, dir)
		if !got.FirstObservedAt.Equal(now) {
			t.Fatalf("persisted FirstObservedAt = %v, want %v", got.FirstObservedAt, now)
		}
		if got.HighestObservedActivation != "secured" {
			t.Fatalf("persisted HighestObservedActivation = %q, want secured", got.HighestObservedActivation)
		}
		if got.FirstMonitoredResourceAt != nil {
			t.Fatalf("persisted FirstMonitoredResourceAt = %v, want nil", got.FirstMonitoredResourceAt)
		}
	})

	t.Run("first ever ping with monitoring marks present at first observation", func(t *testing.T) {
		dir := t.TempDir()
		now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
		ping := &Ping{PVENodes: 3}
		applyLifecycle(ping, dir, now)

		if ping.ActivationStage != "monitoring" {
			t.Fatalf("ActivationStage = %q, want monitoring", ping.ActivationStage)
		}
		if !ping.MonitoringActive {
			t.Fatal("MonitoringActive = false, want true")
		}
		if ping.EstateSizeBucket != "1_10" {
			t.Fatalf("EstateSizeBucket = %q, want 1_10", ping.EstateSizeBucket)
		}
		if ping.TimeToFirstMonitoredResourceBucket != "present_at_first_observation" {
			t.Fatalf("TimeToFirstMonitoredResourceBucket = %q, want present_at_first_observation", ping.TimeToFirstMonitoredResourceBucket)
		}

		got := decodeLifecycleRaw(t, dir)
		if !got.FirstObservedAt.Equal(now) {
			t.Fatalf("persisted FirstObservedAt = %v, want %v", got.FirstObservedAt, now)
		}
		if got.FirstMonitoredResourceAt == nil || !got.FirstMonitoredResourceAt.Equal(now) {
			t.Fatalf("persisted FirstMonitoredResourceAt = %v, want %v", got.FirstMonitoredResourceAt, now)
		}
		if got.HighestObservedActivation != "monitoring" {
			t.Fatalf("persisted HighestObservedActivation = %q, want monitoring", got.HighestObservedActivation)
		}
	})

	t.Run("subsequent ping preserves first observed and records first monitored later", func(t *testing.T) {
		dir := t.TempDir()
		start := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
		seed := lifecycleRecord{
			FirstObservedAt:           start,
			HighestObservedActivation: "secured",
		}
		encoded, err := json.Marshal(seed)
		if err != nil {
			t.Fatalf("marshal seed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, lifecycleStateFile), encoded, 0600); err != nil {
			t.Fatalf("write seed: %v", err)
		}

		later := start.Add(2 * time.Hour)
		ping := &Ping{AuthConfigured: true, PVENodes: 1}
		applyLifecycle(ping, dir, later)

		if ping.ActivationStage != "monitoring" {
			t.Fatalf("ActivationStage = %q, want monitoring (upgraded)", ping.ActivationStage)
		}
		if !ping.MonitoringActive {
			t.Fatal("MonitoringActive = false, want true")
		}
		if ping.KnownInstallAgeBucket != "under_1d" {
			t.Fatalf("KnownInstallAgeBucket = %q, want under_1d", ping.KnownInstallAgeBucket)
		}
		if ping.TimeToFirstMonitoredResourceBucket != "1_6h" {
			t.Fatalf("TimeToFirstMonitoredResourceBucket = %q, want 1_6h", ping.TimeToFirstMonitoredResourceBucket)
		}

		got := decodeLifecycleRaw(t, dir)
		if !got.FirstObservedAt.Equal(start) {
			t.Fatalf("persisted FirstObservedAt = %v, want preserved %v", got.FirstObservedAt, start)
		}
		if got.HighestObservedActivation != "monitoring" {
			t.Fatalf("persisted HighestObservedActivation = %q, want monitoring", got.HighestObservedActivation)
		}
		if got.FirstMonitoredResourceAt == nil || !got.FirstMonitoredResourceAt.Equal(later) {
			t.Fatalf("persisted FirstMonitoredResourceAt = %v, want %v", got.FirstMonitoredResourceAt, later)
		}
	})

	t.Run("subsequent lower stage ping preserves highest observed activation and first monitored", func(t *testing.T) {
		dir := t.TempDir()
		start := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
		firstMon := start.Add(30 * time.Minute)
		seed := lifecycleRecord{
			FirstObservedAt:           start,
			FirstMonitoredResourceAt:  &firstMon,
			HighestObservedActivation: "monitoring",
		}
		encoded, err := json.Marshal(seed)
		if err != nil {
			t.Fatalf("marshal seed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, lifecycleStateFile), encoded, 0600); err != nil {
			t.Fatalf("write seed: %v", err)
		}

		later := start.Add(48 * time.Hour)
		ping := &Ping{AuthConfigured: true}
		applyLifecycle(ping, dir, later)

		if ping.ActivationStage != "monitoring" {
			t.Fatalf("ActivationStage = %q, want preserved monitoring", ping.ActivationStage)
		}
		if ping.MonitoringActive {
			t.Fatal("MonitoringActive = true, want false")
		}
		if ping.KnownInstallAgeBucket != "1_7d" {
			t.Fatalf("KnownInstallAgeBucket = %q, want 1_7d", ping.KnownInstallAgeBucket)
		}
		if ping.TimeToFirstMonitoredResourceBucket != "15m_1h" {
			t.Fatalf("TimeToFirstMonitoredResourceBucket = %q, want 15m_1h", ping.TimeToFirstMonitoredResourceBucket)
		}

		got := decodeLifecycleRaw(t, dir)
		if !got.FirstObservedAt.Equal(start) {
			t.Fatalf("persisted FirstObservedAt = %v, want %v", got.FirstObservedAt, start)
		}
		if got.HighestObservedActivation != "monitoring" {
			t.Fatalf("persisted HighestObservedActivation = %q, want monitoring", got.HighestObservedActivation)
		}
		if got.FirstMonitoredResourceAt == nil || !got.FirstMonitoredResourceAt.Equal(firstMon) {
			t.Fatalf("persisted FirstMonitoredResourceAt = %v, want preserved %v", got.FirstMonitoredResourceAt, firstMon)
		}
	})

	t.Run("outcome signals set OutcomeObserved30d and upgrade stage to outcome observed", func(t *testing.T) {
		dir := t.TempDir()
		now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
		ping := &Ping{PVENodes: 1, AlertsFired30d: 2}
		applyLifecycle(ping, dir, now)

		if !ping.OutcomeObserved30d {
			t.Fatal("OutcomeObserved30d = false, want true")
		}
		if !ping.MonitoringActive {
			t.Fatal("MonitoringActive = false, want true")
		}
		if ping.ActivationStage != "outcome_observed" {
			t.Fatalf("ActivationStage = %q, want outcome_observed", ping.ActivationStage)
		}

		got := decodeLifecycleRaw(t, dir)
		if got.HighestObservedActivation != "outcome_observed" {
			t.Fatalf("persisted HighestObservedActivation = %q, want outcome_observed", got.HighestObservedActivation)
		}
	})

	t.Run("corrupted record with first monitored before first observed clamps elapsed to zero", func(t *testing.T) {
		dir := t.TempDir()
		firstObs := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
		beforeFirstObs := firstObs.Add(-30 * time.Minute)
		seed := lifecycleRecord{
			FirstObservedAt:           firstObs,
			FirstMonitoredResourceAt:  &beforeFirstObs,
			HighestObservedActivation: "monitoring",
		}
		encoded, err := json.Marshal(seed)
		if err != nil {
			t.Fatalf("marshal seed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, lifecycleStateFile), encoded, 0600); err != nil {
			t.Fatalf("write seed: %v", err)
		}

		// No monitored resources -> MonitoringActive stays false and the stored
		// (corrupted) FirstMonitoredResourceAt is preserved, exercising the
		// negative-elapsed clamp.
		ping := &Ping{AuthConfigured: true}
		applyLifecycle(ping, dir, firstObs.Add(time.Hour))

		if ping.TimeToFirstMonitoredResourceBucket != "under_15m" {
			t.Fatalf("TimeToFirstMonitoredResourceBucket = %q, want under_15m (negative elapsed clamped to 0)", ping.TimeToFirstMonitoredResourceBucket)
		}
		if ping.ActivationStage != "monitoring" {
			t.Fatalf("ActivationStage = %q, want preserved monitoring", ping.ActivationStage)
		}
	})

	t.Run("unwritable data dir still populates ping and tolerates write failure", func(t *testing.T) {
		dir := unwritableDataDir(t)
		now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
		ping := &Ping{AuthConfigured: true}
		// Must not panic; the in-memory record is still applied to the ping even
		// though persistence fails.
		applyLifecycle(ping, dir, now)

		if ping.ActivationStage != "secured" {
			t.Fatalf("ActivationStage = %q, want secured", ping.ActivationStage)
		}
		if ping.KnownInstallAgeBucket != "under_1d" {
			t.Fatalf("KnownInstallAgeBucket = %q, want under_1d", ping.KnownInstallAgeBucket)
		}
		if ping.TimeToFirstMonitoredResourceBucket != "not_observed" {
			t.Fatalf("TimeToFirstMonitoredResourceBucket = %q, want not_observed", ping.TimeToFirstMonitoredResourceBucket)
		}
	})
}

// TestBranchcov0723Am_ParseInstallIDRecord covers empty/whitespace input,
// garbage, a legacy plaintext UUID, valid JSON with an invalid UUID, valid JSON
// with a valid UUID but a zero IssuedAt, malformed JSON, a fully valid record,
// and the whitespace-trimming of the install_id field.
func TestBranchcov0723Am_ParseInstallIDRecord(t *testing.T) {
	id := uuid.New().String()
	issuedAt := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		input  []byte
		wantOK bool
	}{
		{"empty input returns false", []byte(""), false},
		{"whitespace only input returns false", []byte("   \n\t "), false},
		{"garbage non json returns false", []byte("not-a-uuid"), false},
		{"legacy plaintext uuid returns false", []byte(id + "\n"), false},
		{"valid json but invalid uuid returns false", []byte(`{"install_id":"not-a-uuid","issued_at":"2026-03-28T12:00:00Z"}`), false},
		{"valid json valid uuid but zero issued at returns false", []byte(`{"install_id":"` + id + `"}`), false},
		{"malformed json returns false", []byte(`{"install_id":}`), false},
		{"valid record with valid uuid and issued at returns true", []byte(fmt.Sprintf(`{"install_id":%q,"issued_at":%q}`, id, issuedAt.Format(time.RFC3339Nano))), true},
		{"whitespace around install id is trimmed and record still valid", []byte(fmt.Sprintf(`{"install_id":%q,"issued_at":%q}`, "  "+id+"  ", issuedAt.Format(time.RFC3339Nano))), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, ok := parseInstallIDRecord(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("parseInstallIDRecord(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if tt.wantOK {
				if record.InstallID != id {
					t.Fatalf("record.InstallID = %q, want %q", record.InstallID, id)
				}
				if !record.IssuedAt.Equal(issuedAt) {
					t.Fatalf("record.IssuedAt = %v, want %v", record.IssuedAt, issuedAt)
				}
			}
		})
	}
}

// TestBranchcov0723Am_ShouldKeepInstallIDRecord covers each keep/discard arm:
// an invalid UUID, a zero IssuedAt, a future IssuedAt, an expired record at the
// exact rotation-window boundary (strict <), and a recent valid record.
func TestBranchcov0723Am_ShouldKeepInstallIDRecord(t *testing.T) {
	id := uuid.New().String()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		record installIDRecord
		now    time.Time
		want   bool
	}{
		{"invalid uuid discarded", installIDRecord{InstallID: "nope", IssuedAt: now.Add(-time.Hour)}, now, false},
		{"zero issued at discarded", installIDRecord{InstallID: id}, now, false},
		{"future issued at discarded", installIDRecord{InstallID: id, IssuedAt: now.Add(time.Hour)}, now, false},
		{"expired at exact rotation window boundary discarded", installIDRecord{InstallID: id, IssuedAt: now.Add(-installIDRotationWindow)}, now, false},
		{"recent valid record kept", installIDRecord{InstallID: id, IssuedAt: now.Add(-time.Hour)}, now, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldKeepInstallIDRecord(tt.record, tt.now); got != tt.want {
				t.Fatalf("shouldKeepInstallIDRecord(%+v, now) = %v, want %v", tt.record, got, tt.want)
			}
		})
	}
}

// TestBranchcov0723Am_WriteInstallIDRecordAt covers the happy path (persisting
// a record that round-trips exactly) and the os.MkdirAll failure path when the
// dataDir's parent is a regular file.
func TestBranchcov0723Am_WriteInstallIDRecordAt(t *testing.T) {
	t.Run("happy path persists record exactly", func(t *testing.T) {
		dir := t.TempDir()
		now := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
		record := installIDRecord{InstallID: uuid.New().String(), IssuedAt: now}
		if err := writeInstallIDRecordAt(dir, record); err != nil {
			t.Fatalf("writeInstallIDRecordAt: %v", err)
		}
		got := decodeInstallIDRecordFile(t, filepath.Join(dir, installIDFile))
		if got.InstallID != record.InstallID {
			t.Fatalf("persisted InstallID = %q, want %q", got.InstallID, record.InstallID)
		}
		if !got.IssuedAt.Equal(now) {
			t.Fatalf("persisted IssuedAt = %v, want %v", got.IssuedAt, now)
		}
	})

	t.Run("dataDir whose parent is a regular file returns non-nil error", func(t *testing.T) {
		dir := unwritableDataDir(t)
		err := writeInstallIDRecordAt(dir, installIDRecord{InstallID: uuid.New().String(), IssuedAt: time.Now()})
		if err == nil {
			t.Fatal("writeInstallIDRecordAt with unwritable dataDir = nil error, want non-nil")
		}
	})
}

// TestBranchcov0723Am_ResetInstallIDAt covers the happy path (a new non-empty
// ID persisted with the given IssuedAt) and the write-failure path (empty ID
// and non-nil error).
func TestBranchcov0723Am_ResetInstallIDAt(t *testing.T) {
	t.Run("happy path writes new id with given issued at", func(t *testing.T) {
		dir := t.TempDir()
		now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
		id, err := resetInstallIDAt(dir, now)
		if err != nil {
			t.Fatalf("resetInstallIDAt: %v", err)
		}
		if id == "" {
			t.Fatal("expected non-empty install id")
		}
		got := decodeInstallIDRecordFile(t, filepath.Join(dir, installIDFile))
		if got.InstallID != id {
			t.Fatalf("persisted InstallID = %q, want %q", got.InstallID, id)
		}
		if !got.IssuedAt.Equal(now) {
			t.Fatalf("persisted IssuedAt = %v, want %v", got.IssuedAt, now)
		}
	})

	t.Run("write error returns empty id and non-nil error", func(t *testing.T) {
		dir := unwritableDataDir(t)
		id, err := resetInstallIDAt(dir, time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC))
		if err == nil {
			t.Fatal("resetInstallIDAt with unwritable dataDir = nil error, want non-nil")
		}
		if id != "" {
			t.Fatalf("resetInstallIDAt returned id %q, want empty on write failure", id)
		}
	})
}

// TestBranchcov0723Am_GetOrCreateInstallIDAt covers the previously-uncovered
// write-failure arm: when persistence fails the caller still receives a fresh,
// valid, non-empty ID for the session, even though nothing lands on disk.
// (The create-when-absent, reuse-when-present, and rotation arms are already
// exercised by sibling tests in telemetry_test.go.)
func TestBranchcov0723Am_GetOrCreateInstallIDAt(t *testing.T) {
	t.Run("write failure still returns a generated id for the session", func(t *testing.T) {
		dir := unwritableDataDir(t)
		now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
		id := getOrCreateInstallIDAt(dir, now)
		if id == "" {
			t.Fatal("getOrCreateInstallIDAt returned empty id; a fresh id should be generated even when persistence fails")
		}
		if _, err := uuid.Parse(id); err != nil {
			t.Fatalf("generated id %q is not a valid uuid: %v", id, err)
		}
		if _, err := os.Stat(filepath.Join(dir, installIDFile)); err == nil {
			t.Fatal("install id file should not be accessible after a write failure")
		}
	})
}

// TestBranchcov0723Am_InstallIDWrappers covers the thin non-"At" wrappers
// ResetInstallID and getOrCreateInstallID. Both take an explicit dataDir, so
// they are pointed at t.TempDir() and never touch a real config path. They use
// time.Now() internally, so the assertions are on non-emptiness, persistence,
// reuse, and rotation (ID change) rather than exact timestamps.
func TestBranchcov0723Am_InstallIDWrappers(t *testing.T) {
	t.Run("getOrCreateInstallID creates then reuses across calls", func(t *testing.T) {
		dir := t.TempDir()
		first := getOrCreateInstallID(dir)
		if first == "" {
			t.Fatal("getOrCreateInstallID returned empty id on first call")
		}
		if _, err := uuid.Parse(first); err != nil {
			t.Fatalf("first id %q is not a valid uuid: %v", first, err)
		}
		got := decodeInstallIDRecordFile(t, filepath.Join(dir, installIDFile))
		if got.InstallID != first {
			t.Fatalf("persisted id = %q, want %q", got.InstallID, first)
		}
		if second := getOrCreateInstallID(dir); second != first {
			t.Fatalf("second getOrCreateInstallID = %q, want same %q", second, first)
		}
	})

	t.Run("ResetInstallID rotates and the new id persists and is reused", func(t *testing.T) {
		dir := t.TempDir()
		original := getOrCreateInstallID(dir)
		if original == "" {
			t.Fatal("getOrCreateInstallID returned empty id")
		}
		rotated, err := ResetInstallID(dir)
		if err != nil {
			t.Fatalf("ResetInstallID: %v", err)
		}
		if rotated == "" {
			t.Fatal("ResetInstallID returned empty id")
		}
		if rotated == original {
			t.Fatalf("ResetInstallID returned the same id %q, want a new rotated one", rotated)
		}
		if next := getOrCreateInstallID(dir); next != rotated {
			t.Fatalf("getOrCreateInstallID after reset = %q, want persisted %q", next, rotated)
		}
	})
}
