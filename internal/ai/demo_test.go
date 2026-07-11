package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mockruntime"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestIsDemoMode(t *testing.T) {
	original := mockruntime.IsEnabled()
	t.Cleanup(func() { mockruntime.SetEnabled(original) })

	mockruntime.SetEnabled(true)
	if !IsDemoMode() {
		t.Fatal("expected demo mode true when runtime mock mode is enabled")
	}

	mockruntime.SetEnabled(false)
	if IsDemoMode() {
		t.Fatal("expected demo mode false when runtime mock mode is disabled")
	}
}

type demoSnapshotProvider struct {
	snapshot models.StateSnapshot
}

func (p demoSnapshotProvider) ReadSnapshot() models.StateSnapshot {
	return p.snapshot
}

func demoTestSnapshot(now time.Time) models.StateSnapshot {
	snapshot := models.EmptyStateSnapshot()
	snapshot.Nodes = []models.Node{
		{ID: "node/pve1", Name: "pve1", Status: "online", Uptime: 86400, CPU: 0.42},
	}
	snapshot.VMs = []models.VM{
		{ID: "vm/101", VMID: 101, Name: "media-server", Node: "pve1", Status: "running",
			Memory: models.Memory{Total: 16 << 30, Used: 15 << 30, Usage: 93.75}},
	}
	snapshot.Storage = []models.Storage{
		{ID: "pve1-local-zfs", Name: "local-zfs", Node: "pve1", Status: "available",
			Enabled: true, Active: true, Total: 750 << 30, Used: 700 << 30, Free: 50 << 30, Usage: 93.3},
	}
	snapshot.DockerHosts = []models.DockerHost{
		{ID: "docker/edge-01", Hostname: "edge-01", DisplayName: "Edge 01", Status: "offline",
			LastSeen: now.Add(-2 * time.Hour),
			Containers: []models.DockerContainer{
				{ID: "docker/edge-01/portal", Name: "portal", State: "exited"},
			}},
		{ID: "docker/core-01", Hostname: "core-01", DisplayName: "Core 01", Status: "online",
			LastSeen: now,
			Containers: []models.DockerContainer{
				{ID: "docker/core-01/uptime-kuma", Name: "uptime-kuma", State: "running",
					Health: "unhealthy", RestartCount: 4, Image: "louislam/uptime-kuma:1"},
			}},
	}
	snapshot.PBSInstances = []models.PBSInstance{
		{ID: "pbs/dr-vault", Name: "dr-vault", Status: "degraded", ConnectionHealth: "degraded"},
	}
	return snapshot
}

func TestPatrolService_RunDemoPatrolCycle_SynthesizesFromMockState(t *testing.T) {
	original := mockruntime.IsEnabled()
	t.Cleanup(func() { mockruntime.SetEnabled(original) })
	mockruntime.SetEnabled(true)

	now := time.Now()
	service := NewPatrolService(nil, demoSnapshotProvider{snapshot: demoTestSnapshot(now)})
	if service.findings == nil || service.runHistoryStore == nil {
		t.Fatal("expected findings and run history to be initialized")
	}

	service.runDemoPatrolCycle(TriggerReasonStartup)

	findings := service.findings.GetActive(FindingSeverityInfo)
	if len(findings) < 4 {
		t.Fatalf("expected at least 4 demo findings (offline host, storage, pbs, container), got %d", len(findings))
	}
	byID := map[string]*Finding{}
	for _, f := range findings {
		if !isDemoFindingID(f.ID) {
			t.Fatalf("expected only demo finding IDs, got %q", f.ID)
		}
		byID[f.ID] = f
	}
	offline := byID["demo-docker-host-offline-edge-01"]
	if offline == nil {
		t.Fatalf("expected offline docker host finding, got IDs %v", keysOfDemoFindings(byID))
	}
	if offline.Severity != FindingSeverityCritical {
		t.Fatalf("expected offline host finding to be critical, got %s", offline.Severity)
	}
	storage := byID["demo-storage-capacity-pve1-local-zfs"]
	if storage == nil {
		t.Fatalf("expected storage capacity finding, got IDs %v", keysOfDemoFindings(byID))
	}
	if storage.Severity != FindingSeverityCritical {
		t.Fatalf("expected 93%% full storage finding to be critical, got %s", storage.Severity)
	}
	if !strings.Contains(storage.Title, "93%") {
		t.Fatalf("expected storage title to quote the observed usage, got %q", storage.Title)
	}

	// Run history: backfill plus the current run.
	if service.runHistoryStore.Count() != 11 {
		t.Fatalf("expected 10 backfill entries plus the current run, got %d", service.runHistoryStore.Count())
	}
	runs := service.GetRunHistory(1)
	if len(runs) != 1 || runs[0].Source != PatrolRunSourceDemo {
		t.Fatalf("expected newest run to be the demo run, got %+v", runs)
	}
	if runs[0].Status != "critical" {
		t.Fatalf("expected run status critical with a critical finding active, got %q", runs[0].Status)
	}
	if runs[0].ResourcesChecked == 0 {
		t.Fatalf("expected run to report resources checked, got %+v", runs[0])
	}

	// Second cycle refreshes instead of duplicating.
	service.runDemoPatrolCycle(TriggerReasonScheduled)
	refreshed := service.findings.GetActive(FindingSeverityInfo)
	if len(refreshed) != len(findings) {
		t.Fatalf("expected second cycle to refresh findings, got %d then %d", len(findings), len(refreshed))
	}
	if service.runHistoryStore.Count() != 12 {
		t.Fatalf("expected exactly one additional run record on second cycle, got %d", service.runHistoryStore.Count())
	}
}

func keysOfDemoFindings(m map[string]*Finding) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestPatrolService_RunDemoPatrolCycle_ResolvesStaleDemoFindings(t *testing.T) {
	original := mockruntime.IsEnabled()
	t.Cleanup(func() { mockruntime.SetEnabled(original) })
	mockruntime.SetEnabled(true)

	now := time.Now()
	snapshot := demoTestSnapshot(now)
	service := NewPatrolService(nil, demoSnapshotProvider{snapshot: snapshot})
	service.runDemoPatrolCycle(TriggerReasonStartup)

	// Mock condition clears: the edge host comes back online.
	healed := demoTestSnapshot(now)
	healed.DockerHosts[0].Status = "online"
	service.SetStateProvider(demoSnapshotProvider{snapshot: healed})
	service.runDemoPatrolCycle(TriggerReasonScheduled)

	offline := service.findings.Get("demo-docker-host-offline-edge-01")
	if offline == nil {
		t.Fatal("expected offline host finding to still exist")
	}
	if !offline.IsResolved() {
		t.Fatal("expected offline host finding to auto-resolve once the mock condition cleared")
	}
}

func TestPatrolService_RunDemoPatrolCycle_ResolvesRuntimeFailureFinding(t *testing.T) {
	original := mockruntime.IsEnabled()
	t.Cleanup(func() { mockruntime.SetEnabled(original) })
	mockruntime.SetEnabled(true)

	now := time.Now()
	service := NewPatrolService(nil, demoSnapshotProvider{snapshot: demoTestSnapshot(now)})

	// A real run failed before mock mode enabled and left the meta-finding.
	failure := patrolRuntimeFailure{
		Cause:       PatrolFailureCauseProviderConnection,
		Title:       "Pulse Patrol: Provider analysis error",
		Description: "Pulse Patrol reached the configured provider, but the provider did not complete the Patrol analysis request.",
	}
	stale := newPatrolRuntimeFailureFinding(failure, now.Add(-time.Hour))
	service.findings.Add(stale)

	service.runDemoPatrolCycle(TriggerReasonScheduled)

	refreshed := service.findings.Get(stale.ID)
	if refreshed == nil {
		t.Fatal("expected runtime failure finding to still exist")
	}
	if !refreshed.IsResolved() {
		t.Fatal("expected demo cycle to auto-resolve the stale provider error finding")
	}
}

func TestPatrolService_RunDemoPatrolCycle_NoStore(t *testing.T) {
	service := &PatrolService{}
	service.runDemoPatrolCycle(TriggerReasonStartup)
}

func TestPatrolService_RunDemoPatrolCycle_EmptyStateSkipsRecording(t *testing.T) {
	original := mockruntime.IsEnabled()
	t.Cleanup(func() { mockruntime.SetEnabled(original) })
	mockruntime.SetEnabled(true)

	service := NewPatrolService(nil, demoSnapshotProvider{snapshot: models.EmptyStateSnapshot()})
	service.runDemoPatrolCycle(TriggerReasonStartup)

	if count := service.runHistoryStore.Count(); count != 0 {
		t.Fatalf("expected no run records for an empty mock state, got %d", count)
	}
}

func TestDemoFindingsExcludedFromPersistence(t *testing.T) {
	findings := map[string]*Finding{
		"demo-storage-capacity-pve1-local-zfs": {ID: "demo-storage-capacity-pve1-local-zfs"},
		"abc123def456":                         {ID: "abc123def456"},
	}
	records := findingsToRecords(findings)
	if _, ok := records["demo-storage-capacity-pve1-local-zfs"]; ok {
		t.Fatal("expected demo finding to be excluded from persistence records")
	}
	if _, ok := records["abc123def456"]; !ok {
		t.Fatal("expected real finding to be persisted")
	}
}

func TestPatrolRunHistoryFiltersDemoEvidenceOutsideDemoMode(t *testing.T) {
	original := mockruntime.IsEnabled()
	t.Cleanup(func() { mockruntime.SetEnabled(original) })
	mockruntime.SetEnabled(false)

	service := NewPatrolService(nil, nil)
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	service.runHistoryStore.Add(PatrolRunRecord{
		ID:               "live-run-1",
		StartedAt:        now.Add(-15 * time.Minute),
		CompletedAt:      now.Add(-14 * time.Minute),
		Type:             "patrol",
		ResourcesChecked: 12,
		FindingsSummary:  "No issues found",
		FindingIDs:       []string{},
		Status:           "healthy",
	})
	service.runHistoryStore.Add(PatrolRunRecord{
		ID:               "demo-run-legacy",
		StartedAt:        now.Add(-5 * time.Minute),
		CompletedAt:      now.Add(-4 * time.Minute),
		Type:             "patrol",
		ResourcesChecked: 47,
		ExistingFindings: 5,
		FindingsSummary:  "2 critical, 3 warnings",
		FindingIDs:       []string{"demo-storage-critical"},
		Status:           "issues_found",
	})

	runs := service.GetRunHistory(1)
	if len(runs) != 1 {
		t.Fatalf("expected one live run after filtering demo evidence, got %d", len(runs))
	}
	if runs[0].ID != "live-run-1" {
		t.Fatalf("expected live-run-1 after filtering demo evidence, got %q", runs[0].ID)
	}
	if _, ok := service.GetRunByID("demo-run-legacy"); ok {
		t.Fatal("expected legacy demo run lookup to be hidden outside demo mode")
	}
}

func TestPatrolRunHistoryKeepsDemoEvidenceInDemoMode(t *testing.T) {
	original := mockruntime.IsEnabled()
	t.Cleanup(func() { mockruntime.SetEnabled(original) })
	mockruntime.SetEnabled(true)

	service := NewPatrolService(nil, nil)
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	service.runHistoryStore.Add(PatrolRunRecord{
		ID:               "live-run-1",
		StartedAt:        now.Add(-15 * time.Minute),
		CompletedAt:      now.Add(-14 * time.Minute),
		Type:             "patrol",
		ResourcesChecked: 12,
		FindingsSummary:  "No issues found",
		FindingIDs:       []string{},
		Status:           "healthy",
	})
	service.runHistoryStore.Add(PatrolRunRecord{
		ID:               "demo-run-1",
		Source:           PatrolRunSourceDemo,
		StartedAt:        now.Add(-5 * time.Minute),
		CompletedAt:      now.Add(-4 * time.Minute),
		Type:             "patrol",
		ResourcesChecked: 47,
		ExistingFindings: 5,
		FindingsSummary:  "2 critical, 3 warnings",
		FindingIDs:       []string{"demo-storage-critical"},
		Status:           "issues_found",
	})

	runs := service.GetRunHistory(1)
	if len(runs) != 1 {
		t.Fatalf("expected one run, got %d", len(runs))
	}
	if runs[0].ID != "demo-run-1" || runs[0].Source != PatrolRunSourceDemo {
		t.Fatalf("expected current demo run in demo mode, got %+v", runs[0])
	}
	if _, ok := service.GetRunByID("demo-run-1"); !ok {
		t.Fatal("expected demo run lookup to be available in demo mode")
	}
}

func TestPatrolCoverageIgnoresPersistedDemoRunsOutsideDemoMode(t *testing.T) {
	original := mockruntime.IsEnabled()
	t.Cleanup(func() { mockruntime.SetEnabled(original) })
	mockruntime.SetEnabled(false)

	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	store := NewPatrolRunHistoryStore(10)
	store.Add(PatrolRunRecord{
		ID:               "demo-run-legacy",
		StartedAt:        now.Add(-5 * time.Minute),
		CompletedAt:      now.Add(-4 * time.Minute),
		Type:             "patrol",
		ResourcesChecked: 47,
		ErrorCount:       1,
		Status:           "error",
		FindingsSummary:  "2 critical, 3 warnings",
		FindingIDs:       []string{"demo-storage-critical"},
	})

	if factor, ok := summarizeRecentPatrolCoverage(store, now); ok {
		t.Fatalf("expected persisted demo run to be excluded from coverage scoring, got %+v", factor)
	}
}

func TestGenerateDemoAIResponse(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		expected string
	}{
		{"patrol", "Analyze the infrastructure for issues", "ZFS pool 'local-zfs' is 94% full"},
		{"disk", "disk is full", "Disk Usage Analysis"},
		{"memory", "memory pressure", "Memory Analysis"},
		{"backup", "pbs backup status", "Backup Status Review"},
		{"cpu", "cpu load is high", "CPU/Performance Analysis"},
		{"hello", "hello there", "Pulse Assistant"},
		{"default", "status report", "This Demo Shows"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := GenerateDemoAIResponse(tt.prompt)
			if resp == nil {
				t.Fatal("expected response")
			}
			if !strings.Contains(resp.Content, tt.expected) {
				t.Fatalf("expected response to contain %q, got %q", tt.expected, resp.Content)
			}
			if resp.Model == "" {
				t.Fatal("expected model to be set")
			}
		})
	}
}

func TestGenerateDemoAIStream(t *testing.T) {
	var content strings.Builder
	done := false

	resp, err := GenerateDemoAIStream("disk usage", func(event StreamEvent) {
		switch event.Type {
		case "content":
			chunk, ok := event.Data.(string)
			if !ok {
				t.Fatalf("expected string content chunk, got %T", event.Data)
			}
			content.WriteString(chunk)
		case "done":
			done = true
		}
	})
	if err != nil {
		t.Fatalf("GenerateDemoAIStream failed: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if !done {
		t.Fatal("expected done event")
	}
	if content.String() != resp.Content {
		t.Fatal("expected streamed content to match response content")
	}
}
