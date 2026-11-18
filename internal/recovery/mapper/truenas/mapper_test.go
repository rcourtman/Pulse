package truenasmapper

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/idgen"
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

	// Undated success cannot establish a completed copy.
	if result[0].Outcome != recovery.OutcomeUnknown {
		t.Errorf("Outcome = %v, want %v (nil LastRun)", result[0].Outcome, recovery.OutcomeUnknown)
	}
}

func TestFromTrueNASSnapshot_ReplicationTimeAndStateEvidence(t *testing.T) {
	zero := time.Time{}
	native := time.Date(2026, 10, 1, 12, 30, 0, 123000000, time.FixedZone("native", 3600))
	for _, date := range []struct {
		name string
		time *time.Time
	}{
		{"missing", nil},
		{"zero", &zero},
		{"native", &native},
	} {
		for _, state := range []struct {
			name    string
			state   string
			errText string
			want    recovery.Outcome
		}{
			{"success", "SUCCESS", "", recovery.OutcomeSuccess},
			{"finished", "FINISHED", "", recovery.OutcomeSuccess},
			{"failed", "FAILED", "", recovery.OutcomeFailed},
			{"aborted", "ABORTED", "", recovery.OutcomeFailed},
			{"warning", "WARNING", "", recovery.OutcomeWarning},
			{"partial", "PARTIAL", "", recovery.OutcomeWarning},
			{"running", "RUNNING", "", recovery.OutcomeRunning},
			{"active", "ACTIVE", "", recovery.OutcomeRunning},
			{"unknown", "UNRECOGNISED", "", recovery.OutcomeUnknown},
			{"absent", "", "", recovery.OutcomeUnknown},
			{"error overrides success", "SUCCESS", "fixture target unavailable", recovery.OutcomeFailed},
			{"error without state", "", "fixture target unavailable", recovery.OutcomeFailed},
		} {
			t.Run(date.name+"/"+state.name, func(t *testing.T) {
				snapshot := &truenas.FixtureSnapshot{ReplicationTasks: []truenas.ReplicationTask{{
					ID: "7", Name: "offsite", LastRun: date.time, LastState: state.state, LastError: state.errText,
				}}}
				point := FromTrueNASSnapshot("nas-a", snapshot)[0]
				want := state.want
				if date.time == nil || date.time.IsZero() {
					if want == recovery.OutcomeSuccess {
						want = recovery.OutcomeUnknown
					}
					if point.StartedAt != nil || point.CompletedAt != nil {
						t.Errorf("unavailable date gained timestamps: %+v", point)
					}
				} else if point.StartedAt == nil || point.CompletedAt == nil ||
					!point.StartedAt.Equal(native) || !point.CompletedAt.Equal(native) {
					t.Errorf("native date changed: %+v", point)
				}
				if point.Outcome != want {
					t.Errorf("Outcome = %q, want %q", point.Outcome, want)
				}
				if point.Details["lastState"] != state.state || point.Details["lastError"] != state.errText {
					t.Errorf("provider evidence changed: %+v", point.Details)
				}
				wantID := idgen.StableID("truenas", "nas-a", "replication-task", "7", "offsite", idgen.TimeKey(date.time, date.time))
				if point.ID != wantID {
					t.Errorf("task identity changed: %s, want %s", point.ID, wantID)
				}
			})
		}
	}
}

func TestFromTrueNASSnapshot_UndatedReplicationIdentity(t *testing.T) {
	zero := time.Time{}
	snapshot := &truenas.FixtureSnapshot{ReplicationTasks: []truenas.ReplicationTask{{
		ID: "7", Name: "offsite", SourceDatasets: []string{"tank/apps"}, TargetDataset: "vault/apps",
	}}}
	wantID := idgen.StableID("truenas", "nas-a", "replication-task", "7", "offsite", "")
	for _, date := range []*time.Time{nil, &zero} {
		for i, state := range []string{"FAILED", "WARNING", "RUNNING", "SUCCESS"} {
			snapshot.CollectedAt = time.Date(2026, 10, 1, 12+i, 0, 0, 0, time.UTC)
			snapshot.ReplicationTasks[0].LastRun = date
			snapshot.ReplicationTasks[0].LastState = state
			point := FromTrueNASSnapshot("nas-a", snapshot)[0]
			if point.ID != wantID || point.StartedAt != nil || point.CompletedAt != nil {
				t.Fatalf("collection/state changed undated identity/time: %+v", point)
			}
			if point.SubjectRef.ID != "tank/apps" || point.RepositoryRef.ID != "vault/apps" {
				t.Fatalf("lost source/target: %+v", point)
			}
			if other := FromTrueNASSnapshot("nas-b", snapshot)[0]; other.ID == point.ID {
				t.Fatal("connections share a task ID")
			}
			if snapshot.ReplicationTasks[0].LastRun != date || !zero.IsZero() {
				t.Fatal("mapping mutated the provider's last-run time")
			}
		}
	}
}

func TestFromTrueNASSnapshot_Truncation(t *testing.T) {
	older := time.Unix(1710000000, 0)
	tied := older.Add(time.Hour)
	newest := tied.Add(time.Hour)
	snapshot := &truenas.FixtureSnapshot{
		System:           truenas.SystemInfo{Hostname: "truenas-main"},
		ZFSSnapshots:     make([]truenas.ZFSSnapshot, 2240),
		ReplicationTasks: make([]truenas.ReplicationTask, 220),
	}
	for i := range snapshot.ZFSSnapshots {
		createdAt := &older
		switch i % 4 {
		case 0:
			createdAt = &tied
		case 1:
			createdAt = nil
		}
		// Names deliberately don't put equal-time rows in input order.
		name := fmt.Sprintf("snap-%04d", (i*17)%len(snapshot.ZFSSnapshots))
		snapshot.ZFSSnapshots[i] = truenas.ZFSSnapshot{
			Dataset:   "pool/data",
			Name:      name,
			FullName:  "pool/data@" + name,
			CreatedAt: createdAt,
		}
	}
	snapshot.ZFSSnapshots[len(snapshot.ZFSSnapshots)-1].CreatedAt = &newest
	for i := range snapshot.ReplicationTasks {
		snapshot.ReplicationTasks[i].Name = fmt.Sprintf("rep-%03d", len(snapshot.ReplicationTasks)-i)
	}
	inventory := slices.Clone(snapshot.ZFSSnapshots)
	result := FromTrueNASSnapshot("conn-1", snapshot)

	if got := len(result); got != 700 {
		t.Fatalf("expected 500 snapshot and 200 replication points, got %d total", got)
	}
	if cap(result) > 700 {
		t.Errorf("point buffer capacity = %d, want at most the ingest limits", cap(result))
	}
	if !slices.Equal(snapshot.ZFSSnapshots, inventory) {
		t.Fatal("mapper changed the provider's inventory")
	}
	for i, point := range result[:500] {
		row := inventory[len(inventory)-1]
		if i > 0 {
			row = inventory[(i-1)*4]
		}
		if got := point.Details["fullName"]; point.Kind != recovery.KindSnapshot || got != row.FullName {
			t.Fatalf("snapshot at rank %d = %v, want %s", i, got, row.FullName)
		}
	}
	for i, point := range result[500:] {
		want := fmt.Sprintf("rep-%03d", i+1)
		if got := point.Details["taskName"]; point.Kind != recovery.KindBackup || got != want {
			t.Fatalf("replication task at rank %d = %v, want %s", i, got, want)
		}
	}
}

func TestFromTrueNASSnapshot_UndatedOrder(t *testing.T) {
	known := time.Unix(1710000000, 0)
	var zero time.Time
	snapshot := &truenas.FixtureSnapshot{ZFSSnapshots: []truenas.ZFSSnapshot{
		{FullName: "tank/data@a", UsedBytes: ptrInt64(1)},
		{FullName: "tank/data@z"},
		{FullName: "tank/data@a", UsedBytes: ptrInt64(2)},
		{FullName: "tank/data@zero", CreatedAt: &zero},
		{FullName: "tank/data@known", CreatedAt: &known},
	}}
	result := FromTrueNASSnapshot("conn-1", snapshot)
	want := []string{"tank/data@known", "tank/data@zero", "tank/data@z", "tank/data@a", "tank/data@a"}
	if len(result) != len(want) {
		t.Fatalf("mapped %d points, want %d", len(result), len(want))
	}
	for i, point := range result {
		if got := point.Details["fullName"]; got != want[i] {
			t.Errorf("snapshot at rank %d = %v, want %s", i, got, want[i])
		}
		if i > 0 && (point.CompletedAt != nil || point.Outcome != recovery.OutcomeUnknown) {
			t.Errorf("snapshot at rank %d acquired time or success: %+v", i, point)
		}
	}
	if result[3].SizeBytes == nil || *result[3].SizeBytes != 1 ||
		result[4].SizeBytes == nil || *result[4].SizeBytes != 2 {
		t.Error("equal undated rows lost their input order")
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
		{"finished uppercase", "FINISHED", "", recovery.OutcomeSuccess},
		{"finished normalized", "  Finished  ", "  ", recovery.OutcomeSuccess},
		{"finished error takes precedence", "FINISHED", "replication failed", recovery.OutcomeFailed},
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

// FINISHED is emitted by TrueNAS zettarepl for ReplicationTaskSuccess (#1892).
func TestFromTrueNASSnapshot_FinishedReplication(t *testing.T) {
	lastRun := time.Date(2026, 9, 4, 5, 45, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		lastRun *time.Time
		errText string
		want    recovery.Outcome
	}{
		{"completed", &lastRun, "", recovery.OutcomeSuccess},
		{"error retained", &lastRun, "transfer failed", recovery.OutcomeFailed},
		{"missing run remains unknown", nil, "", recovery.OutcomeUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			points := FromTrueNASSnapshot("conn", &truenas.FixtureSnapshot{
				ReplicationTasks: []truenas.ReplicationTask{{
					ID: "1", LastState: "FINISHED", LastRun: tc.lastRun, LastError: tc.errText,
				}},
			})
			if len(points) != 1 {
				t.Fatalf("got %d points", len(points))
			}
			if points[0].Outcome != tc.want {
				t.Errorf("outcome = %v, want %v", points[0].Outcome, tc.want)
			}
			if points[0].Details["lastState"] != "FINISHED" {
				t.Error("provider state lost")
			}
		})
	}
}

func TestFromTrueNASSnapshot_MissingCreationTime(t *testing.T) {
	collected := time.Date(2026, 10, 25, 20, 0, 0, 0, time.UTC)
	zero := time.Time{}
	for _, tc := range []struct {
		name    string
		created *time.Time
	}{
		{"absent", nil},
		{"zero", &zero},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := &truenas.FixtureSnapshot{ZFSSnapshots: []truenas.ZFSSnapshot{{
				ID: "tank/apps@daily", FullName: "tank/apps@daily", CreatedAt: tc.created,
			}}}
			var id string
			for _, at := range []time.Time{collected, collected.Add(time.Hour), {}} {
				snapshot.CollectedAt = at
				points := FromTrueNASSnapshot("nas-a", snapshot)
				if len(points) != 1 {
					t.Fatalf("got %d points, want 1", len(points))
				}
				p := points[0]
				if p.StartedAt != nil || p.CompletedAt != nil || p.Outcome != recovery.OutcomeUnknown {
					t.Errorf("undated snapshot gained creation/success evidence: %+v", p)
				}
				if p.ID == "" {
					t.Fatal("missing artifact identity")
				}
				if id != "" && id != p.ID {
					t.Errorf("collection changed ID: %s -> %s", id, p.ID)
				}
				id = p.ID
				if p.SubjectRef.ID != "tank/apps" || p.Details["snapshot"] != "daily" {
					t.Errorf("lost snapshot identity: %+v", p)
				}
				other := FromTrueNASSnapshot("nas-b", snapshot)
				if other[0].ID == id {
					t.Error("connections share an artifact ID")
				}
			}
		})
	}
}

func TestFromTrueNASSnapshot_PreservesNativeCreationTimeAndIdentity(t *testing.T) {
	created := time.Date(2026, 10, 1, 12, 30, 0, 123000000, time.FixedZone("native", 3600))
	snapshot := &truenas.FixtureSnapshot{ZFSSnapshots: []truenas.ZFSSnapshot{{
		FullName: "tank/apps@daily", CreatedAt: &created,
	}}}
	wantID := idgen.StableID("truenas", "nas-a", "zfs-snapshot", "tank/apps@daily", created.UTC().Format(time.RFC3339Nano))
	for _, at := range []time.Time{created.Add(time.Hour), created.Add(24 * time.Hour), {}} {
		snapshot.CollectedAt = at
		p := FromTrueNASSnapshot("nas-a", snapshot)[0]
		if p.ID != wantID || p.Outcome != recovery.OutcomeSuccess || p.StartedAt == nil || p.CompletedAt == nil ||
			!p.StartedAt.Equal(created) || !p.CompletedAt.Equal(created) {
			t.Fatalf("native creation/identity changed: %+v, want %s at %s", p, wantID, created)
		}
	}
}
