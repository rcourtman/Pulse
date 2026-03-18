package truenasmapper

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
)

func TestFromTrueNASSnapshot_NilSnapshot(t *testing.T) {
	result := FromTrueNASSnapshot("conn-1", nil)
	if result != nil {
		t.Errorf("FromTrueNASSnapshot(nil) = %v, want nil", result)
	}
}

func TestFromTrueNASSnapshot_EmptySnapshot(t *testing.T) {
	snapshot := &truenas.FixtureSnapshot{
		System: truenas.SystemInfo{Hostname: "truenas-1"},
	}
	result := FromTrueNASSnapshot("conn-1", snapshot)
	if len(result) != 0 {
		t.Errorf("FromTrueNASSnapshot(empty) = %d points, want 0", len(result))
	}
}

func TestFromTrueNASSnapshot_ZFSSnapshot(t *testing.T) {
	createdAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	snapshot := &truenas.FixtureSnapshot{
		System: truenas.SystemInfo{Hostname: "truenas-main"},
		ZFSSnapshots: []truenas.ZFSSnapshot{
			{
				ID:        "snap-1",
				Dataset:   "pool/data",
				Name:      "daily-001",
				FullName:  "pool/data@daily-001",
				CreatedAt: &createdAt,
				UsedBytes: ptrInt64(1024),
			},
		},
	}

	result := FromTrueNASSnapshot("conn-1", snapshot)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.Provider != recovery.ProviderTrueNAS {
		t.Errorf("Provider = %v, want %v", p.Provider, recovery.ProviderTrueNAS)
	}
	if p.Kind != recovery.KindSnapshot {
		t.Errorf("Kind = %v, want %v", p.Kind, recovery.KindSnapshot)
	}
	if p.Outcome != recovery.OutcomeSuccess {
		t.Errorf("Outcome = %v, want %v", p.Outcome, recovery.OutcomeSuccess)
	}
	if p.SubjectRef == nil {
		t.Error("expected SubjectRef to be set")
	} else {
		if p.SubjectRef.Type != "truenas-dataset" {
			t.Errorf("SubjectRef.Type = %q, want 'truenas-dataset'", p.SubjectRef.Type)
		}
		if p.SubjectRef.Name != "pool/data" {
			t.Errorf("SubjectRef.Name = %q, want 'pool/data'", p.SubjectRef.Name)
		}
	}
}

func TestFromTrueNASSnapshot_ReplicationTask(t *testing.T) {
	lastRun := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	snapshot := &truenas.FixtureSnapshot{
		System: truenas.SystemInfo{Hostname: "truenas-main"},
		ReplicationTasks: []truenas.ReplicationTask{
			{
				ID:             "rep-1",
				Name:           "backup-task-1",
				SourceDatasets: []string{"pool/source"},
				TargetDataset:  "pool/target",
				Direction:      "push",
				LastRun:        &lastRun,
				LastState:      "SUCCESS",
				LastError:      "",
			},
		},
	}

	result := FromTrueNASSnapshot("conn-1", snapshot)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.Provider != recovery.ProviderTrueNAS {
		t.Errorf("Provider = %v, want %v", p.Provider, recovery.ProviderTrueNAS)
	}
	if p.Kind != recovery.KindBackup {
		t.Errorf("Kind = %v, want %v", p.Kind, recovery.KindBackup)
	}
	if p.Mode != recovery.ModeRemote {
		t.Errorf("Mode = %v, want %v", p.Mode, recovery.ModeRemote)
	}
	if p.Outcome != recovery.OutcomeSuccess {
		t.Errorf("Outcome = %v, want %v", p.Outcome, recovery.OutcomeSuccess)
	}
}

func TestFromTrueNASSnapshot_ReplicationTaskNilLastRun(t *testing.T) {
	snapshot := &truenas.FixtureSnapshot{
		System: truenas.SystemInfo{Hostname: "truenas-main"},
		ReplicationTasks: []truenas.ReplicationTask{
			{
				ID:        "rep-1",
				Name:      "backup-task-1",
				LastRun:   nil,
				LastState: "SUCCESS",
				LastError: "",
			},
		},
	}

	result := FromTrueNASSnapshot("conn-1", snapshot)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	// When LastRun is nil, outcome should be Unknown
	if result[0].Outcome != recovery.OutcomeUnknown {
		t.Errorf("Outcome = %v, want %v (nil LastRun)", result[0].Outcome, recovery.OutcomeUnknown)
	}
}

func TestFromTrueNASSnapshot_Truncation(t *testing.T) {
	snapshot := &truenas.FixtureSnapshot{
		System: truenas.SystemInfo{Hostname: "truenas-main"},
	}

	// Create 501 ZFS snapshots
	snapshot.ZFSSnapshots = make([]truenas.ZFSSnapshot, 501)
	for i := range snapshot.ZFSSnapshots {
		snapshot.ZFSSnapshots[i] = truenas.ZFSSnapshot{
			ID:       "snap",
			Dataset:  "pool/data",
			Name:     "snap",
			FullName: "pool/data@snap",
		}
	}

	result := FromTrueNASSnapshot("conn-1", snapshot)

	if len(result) != 500 {
		t.Errorf("expected 500 points (maxTrueNASSnapshotsPerIngest), got %d", len(result))
	}
}

func TestFromTrueNASSnapshot_IDStability(t *testing.T) {
	createdAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	snapshot := &truenas.FixtureSnapshot{
		System: truenas.SystemInfo{Hostname: "truenas-main"},
		ZFSSnapshots: []truenas.ZFSSnapshot{
			{
				ID:        "snap-1",
				Dataset:   "pool/data",
				Name:      "daily-001",
				FullName:  "pool/data@daily-001",
				CreatedAt: &createdAt,
			},
		},
	}

	result1 := FromTrueNASSnapshot("conn-1", snapshot)
	result2 := FromTrueNASSnapshot("conn-1", snapshot)

	if len(result1) != 1 || len(result2) != 1 {
		t.Fatal("expected 1 point each")
	}

	if result1[0].ID != result2[0].ID {
		t.Errorf("IDs not stable: %q != %q", result1[0].ID, result2[0].ID)
	}
}

func TestOutcomeFromTrueNASReplication(t *testing.T) {
	tests := []struct {
		name     string
		state    string
		errText  string
		expected recovery.Outcome
	}{
		{"success lowercase", "success", "", recovery.OutcomeSuccess},
		{"success uppercase", "SUCCESS", "", recovery.OutcomeSuccess},
		{"success mixed case", "Success", "", recovery.OutcomeSuccess},
		{"succeeded", "succeeded", "", recovery.OutcomeSuccess},
		{"ok", "ok", "", recovery.OutcomeSuccess},
		{"complete", "complete", "", recovery.OutcomeSuccess},
		{"completed", "completed", "", recovery.OutcomeSuccess},
		{"warning", "warning", "", recovery.OutcomeWarning},
		{"partial", "partial", "", recovery.OutcomeWarning},
		{"partiallyfailed", "partiallyfailed", "", recovery.OutcomeWarning},
		{"partially_failed", "partially_failed", "", recovery.OutcomeWarning},
		{"failed", "failed", "", recovery.OutcomeFailed},
		{"error state", "error", "", recovery.OutcomeFailed},
		{"aborted", "aborted", "", recovery.OutcomeFailed},
		{"running", "running", "", recovery.OutcomeRunning},
		{"inprogress", "inprogress", "", recovery.OutcomeRunning},
		{"in_progress", "in_progress", "", recovery.OutcomeRunning},
		{"active", "active", "", recovery.OutcomeRunning},
		{"unknown", "unknown_state", "", recovery.OutcomeUnknown},
		{"empty", "", "", recovery.OutcomeUnknown},
		{"error text takes precedence", "success", "some error", recovery.OutcomeFailed},
		{"error text with warning state", "warning", "disk full", recovery.OutcomeFailed},
		{"whitespace trimmed", "  success  ", "", recovery.OutcomeSuccess},
		{"case insensitive", "WARNING", "", recovery.OutcomeWarning},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := outcomeFromTrueNASReplication(tc.state, tc.errText)
			if result != tc.expected {
				t.Errorf("outcomeFromTrueNASReplication(%q, %q) = %v, want %v", tc.state, tc.errText, result, tc.expected)
			}
		})
	}
}

func TestSplitSnapshotName(t *testing.T) {
	tests := []struct {
		name         string
		full         string
		wantDataset  string
		wantSnapshot string
	}{
		{"normal", "pool/data@snap-name", "pool/data", "snap-name"},
		{"no at", "pool/data", "", ""},
		{"empty", "", "", ""},
		{"multiple at signs", "pool/data@snap@extra", "pool/data", "snap@extra"},
		{"whitespace", "  pool/data  @  snap-name  ", "pool/data", "snap-name"},
		{"just snapshot", "@snap", "", "snap"},
		{"trailing at", "pool/data@", "pool/data", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dataset, snapshot := splitSnapshotName(tc.full)
			if dataset != tc.wantDataset {
				t.Errorf("splitSnapshotName(%q) dataset = %q, want %q", tc.full, dataset, tc.wantDataset)
			}
			if snapshot != tc.wantSnapshot {
				t.Errorf("splitSnapshotName(%q) snapshot = %q, want %q", tc.full, snapshot, tc.wantSnapshot)
			}
		})
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}
