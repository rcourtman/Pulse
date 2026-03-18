package unifiedresources

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSanitizeOrgID_AllowsSafeChars(t *testing.T) {
	in := "Acme_Org-123"
	if got := sanitizeOrgID(in); got != in {
		t.Fatalf("sanitizeOrgID(%q) = %q, want %q", in, got, in)
	}
}

func TestSanitizeOrgID_StripsUnsafeCharsAndBoundsLength(t *testing.T) {
	in := "../../../../tenant?mode=memory&_pragma=trusted_schema(OFF)#frag"
	got := sanitizeOrgID(in)

	if got == "" {
		t.Fatal("expected non-empty sanitized org ID")
	}
	if len(got) > maxOrgIDLength {
		t.Fatalf("sanitizeOrgID length = %d, want <= %d", len(got), maxOrgIDLength)
	}
	if strings.ContainsAny(got, "/\\?&=#. \t\r\n") {
		t.Fatalf("sanitizeOrgID produced unsafe characters: %q", got)
	}
}

func TestSanitizeOrgID_AllUnsafeInputReturnsEmpty(t *testing.T) {
	if got := sanitizeOrgID("../??//..  "); got != "" {
		t.Fatalf("sanitizeOrgID returned %q, want empty string", got)
	}
}

func TestNewSQLiteResourceStore_DefaultOrgUsesSharedResourcesPath(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewSQLiteResourceStore(dataDir, "default")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore returned error: %v", err)
	}
	defer store.Close()

	wantPath := filepath.Join(dataDir, "resources", "unified_resources.db")
	if store.dbPath != wantPath {
		t.Fatalf("db path = %q, want %q", store.dbPath, wantPath)
	}
}

func TestNewSQLiteResourceStore_NonDefaultOrgUsesTenantScopedPath(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewSQLiteResourceStore(dataDir, "org-a")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore returned error: %v", err)
	}
	defer store.Close()

	wantPath := filepath.Join(dataDir, "orgs", "org-a", "resources", "unified_resources.db")
	if store.dbPath != wantPath {
		t.Fatalf("db path = %q, want %q", store.dbPath, wantPath)
	}
}

func TestNewSQLiteResourceStore_OrgDotAndUnderscoreDoNotCollide(t *testing.T) {
	dataDir := t.TempDir()
	dotStore, err := NewSQLiteResourceStore(dataDir, "org.a")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore(org.a) returned error: %v", err)
	}
	defer dotStore.Close()

	underscoreStore, err := NewSQLiteResourceStore(dataDir, "org_a")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore(org_a) returned error: %v", err)
	}
	defer underscoreStore.Close()

	if dotStore.dbPath == underscoreStore.dbPath {
		t.Fatalf("db path collision: org.a and org_a both mapped to %q", dotStore.dbPath)
	}
}

func TestNewSQLiteResourceStore_RejectsInvalidOrgID(t *testing.T) {
	dataDir := t.TempDir()
	if _, err := NewSQLiteResourceStore(dataDir, "../bad-org"); err == nil {
		t.Fatal("expected invalid org ID error, got nil")
	}
}

func TestNewSQLiteResourceStore_MigratesLegacyStore(t *testing.T) {
	dataDir := t.TempDir()
	orgID := "org.a"
	legacyPath := filepath.Join(dataDir, "resources", legacyResourceStoreFileName(orgID))
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) failed: %v", filepath.Dir(legacyPath), err)
	}
	seedLegacyLinksTable(t, legacyPath)

	store, err := NewSQLiteResourceStore(dataDir, orgID)
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore returned error: %v", err)
	}
	defer store.Close()

	links, err := store.GetLinks()
	if err != nil {
		t.Fatalf("GetLinks returned error: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("GetLinks length = %d, want 1", len(links))
	}
	if links[0].ResourceA != "legacy-a" || links[0].ResourceB != "legacy-b" {
		t.Fatalf("unexpected migrated link: %+v", links[0])
	}

	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy db should remain for compatibility, stat(%q) failed: %v", legacyPath, err)
	}
	if store.dbPath == legacyPath {
		t.Fatalf("expected migrated store path to differ from legacy path: %q", store.dbPath)
	}
}

func newTestStore(t *testing.T) *SQLiteResourceStore {
	t.Helper()
	dir := t.TempDir()
	store, err := NewSQLiteResourceStore(dir, "testorg")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestRecordChange_RoundTrip(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	occurredAt := now.Add(-30 * time.Second)

	change := ResourceChange{
		ID:               "chg-1",
		ResourceID:       "vm:100",
		ObservedAt:       now,
		OccurredAt:       &occurredAt,
		Kind:             ChangeStateTransition,
		From:             "offline",
		To:               "online",
		SourceType:       SourcePlatformEvent,
		SourceAdapter:    AdapterProxmox,
		Confidence:       ConfidenceHigh,
		Actor:            "agent:ops-helper",
		RelatedResources: []string{"node:1", "storage:2"},
		Reason:           "vm started",
		Metadata: map[string]any{
			"source": "snapshot",
			"retry":  1,
		},
	}

	if err := store.RecordChange(change); err != nil {
		t.Fatalf("RecordChange: %v", err)
	}

	results, err := store.GetRecentChanges("vm:100", now.Add(-time.Minute), 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 change, got %d", len(results))
	}
	got := results[0]
	if got.ID != change.ID {
		t.Errorf("ID: got %q, want %q", got.ID, change.ID)
	}
	if got.Kind != change.Kind {
		t.Errorf("Kind: got %q, want %q", got.Kind, change.Kind)
	}
	if got.From != change.From || got.To != change.To {
		t.Errorf("From/To: got %q/%q, want %q/%q", got.From, got.To, change.From, change.To)
	}
	if got.Confidence != change.Confidence {
		t.Errorf("Confidence: got %q, want %q", got.Confidence, change.Confidence)
	}
	if got.SourceType != change.SourceType {
		t.Errorf("SourceType: got %q, want %q", got.SourceType, change.SourceType)
	}
	if got.SourceAdapter != change.SourceAdapter {
		t.Errorf("SourceAdapter: got %q, want %q", got.SourceAdapter, change.SourceAdapter)
	}
	if got.Actor != change.Actor {
		t.Errorf("Actor: got %q, want %q", got.Actor, change.Actor)
	}
	if got.Reason != change.Reason {
		t.Errorf("Reason: got %q, want %q", got.Reason, change.Reason)
	}
	if len(got.RelatedResources) != len(change.RelatedResources) {
		t.Fatalf("RelatedResources length: got %d, want %d", len(got.RelatedResources), len(change.RelatedResources))
	}
	for i := range change.RelatedResources {
		if got.RelatedResources[i] != change.RelatedResources[i] {
			t.Fatalf("RelatedResources[%d]: got %q, want %q", i, got.RelatedResources[i], change.RelatedResources[i])
		}
	}
	if got.OccurredAt == nil || !got.OccurredAt.Equal(occurredAt) {
		t.Fatalf("OccurredAt: got %v, want %v", got.OccurredAt, occurredAt)
	}
	if got.Metadata["source"] != change.Metadata["source"] {
		t.Fatalf("Metadata source: got %v, want %v", got.Metadata["source"], change.Metadata["source"])
	}
	if fmt.Sprint(got.Metadata["retry"]) != "1" {
		t.Fatalf("Metadata retry: got %v, want 1", got.Metadata["retry"])
	}
}

func TestRecordChange_PreservesTimelineMetadata(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	occurredAt := now.Add(-5 * time.Minute)

	change := ResourceChange{
		ID:               "chg-rich-1",
		ResourceID:       "vm:200",
		ObservedAt:       now,
		OccurredAt:       &occurredAt,
		Kind:             ChangeRelationship,
		SourceType:       SourcePulseDiff,
		SourceAdapter:    AdapterDocker,
		Confidence:       ConfidenceMedium,
		Actor:            "pulse:differ",
		RelatedResources: []string{"node:20", "service:api"},
		Reason:           "dependency graph updated",
		Metadata: map[string]any{
			"edgeType": "runs_on",
			"active":   true,
		},
	}

	if err := store.RecordChange(change); err != nil {
		t.Fatalf("RecordChange: %v", err)
	}

	results, err := store.GetRecentChanges("vm:200", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 change, got %d", len(results))
	}

	got := results[0]
	if got.Kind != change.Kind || got.SourceType != change.SourceType || got.SourceAdapter != change.SourceAdapter {
		t.Fatalf("unexpected change headers: %+v", got)
	}
	if got.OccurredAt == nil || !got.OccurredAt.Equal(occurredAt) {
		t.Fatalf("OccurredAt: got %v, want %v", got.OccurredAt, occurredAt)
	}
	if len(got.RelatedResources) != 2 || got.RelatedResources[0] != "node:20" || got.RelatedResources[1] != "service:api" {
		t.Fatalf("RelatedResources round-trip failed: %+v", got.RelatedResources)
	}
	if got.Metadata["edgeType"] != "runs_on" {
		t.Fatalf("Metadata edgeType: got %v, want %v", got.Metadata["edgeType"], "runs_on")
	}
	if got.Metadata["active"] != true {
		t.Fatalf("Metadata active: got %v, want true", got.Metadata["active"])
	}
}

func TestCountRecentChanges_RespectsFilters(t *testing.T) {
	store := newTestStore(t)
	base := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	changes := []ResourceChange{
		{
			ID:         "chg-count-1",
			ResourceID: "vm:1",
			ObservedAt: base.Add(-30 * time.Minute),
			Kind:       ChangeStateTransition,
			SourceType: SourcePlatformEvent,
			Confidence: ConfidenceHigh,
		},
		{
			ID:         "chg-count-2",
			ResourceID: "vm:1",
			ObservedAt: base.Add(-20 * time.Minute),
			Kind:       ChangeAnomaly,
			SourceType: SourcePulseDiff,
			Confidence: ConfidenceMedium,
		},
		{
			ID:         "chg-count-3",
			ResourceID: "vm:1",
			ObservedAt: base.Add(-10 * time.Minute),
			Kind:       ChangeRelationship,
			SourceType: SourcePulseDiff,
			Confidence: ConfidenceLow,
		},
		{
			ID:         "chg-count-4",
			ResourceID: "vm:2",
			ObservedAt: base.Add(-5 * time.Minute),
			Kind:       ChangeCapability,
			SourceType: SourcePulseDiff,
			Confidence: ConfidenceLow,
		},
	}
	for _, change := range changes {
		if err := store.RecordChange(change); err != nil {
			t.Fatalf("RecordChange(%s): %v", change.ID, err)
		}
	}

	count, err := store.CountRecentChanges("vm:1", base.Add(-25*time.Minute))
	if err != nil {
		t.Fatalf("CountRecentChanges vm:1: %v", err)
	}
	if count != 2 {
		t.Fatalf("CountRecentChanges vm:1 = %d, want 2", count)
	}

	allCount, err := store.CountRecentChanges("", base.Add(-15*time.Minute))
	if err != nil {
		t.Fatalf("CountRecentChanges all: %v", err)
	}
	if allCount != 2 {
		t.Fatalf("CountRecentChanges all = %d, want 2", allCount)
	}
}

func TestActionAuditRecord_RoundTrip(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 3, 18, 13, 0, 0, 0, time.UTC)
	expires := now.Add(15 * time.Minute)
	approvedAt := now.Add(2 * time.Minute)
	result := &ExecutionResult{Success: true, Output: "completed"}

	record := ActionAuditRecord{
		ID:        "action-1",
		CreatedAt: now,
		UpdatedAt: now.Add(5 * time.Minute),
		State:     ActionStateCompleted,
		Request: ActionRequest{
			RequestID:      "req-1",
			ResourceID:     "vm:300",
			CapabilityName: "restart",
			Params: map[string]any{
				"force": true,
			},
			Reason:      "restart for maintenance",
			RequestedBy: "agent:oncall-helper",
		},
		Plan: ActionPlan{
			ActionID:             "action-1",
			RequestID:            "req-1",
			Allowed:              true,
			RequiresApproval:     true,
			ApprovalPolicy:       ApprovalAdmin,
			PredictedBlastRadius: []string{"node:1", "storage:1"},
			RollbackAvailable:    true,
			Message:              "allowed",
			PlannedAt:            now,
			ExpiresAt:            expires,
			ResourceVersion:      "rv-1",
			PolicyVersion:        "pv-1",
			GraphVersion:         "gv-1",
			PlanHash:             "plan-hash-1",
		},
		Approvals: []ActionApprovalRecord{
			{
				Actor:     "admin@example.com",
				Method:    MethodUI,
				Timestamp: approvedAt,
				Outcome:   OutcomeApproved,
				Reason:    "approved for maintenance window",
			},
		},
		Result: result,
	}

	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	results, err := store.GetActionAudits("vm:300", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 action audit, got %d", len(results))
	}

	got := results[0]
	if got.ID != record.ID || got.State != record.State {
		t.Fatalf("unexpected audit headers: %+v", got)
	}
	if got.Request.RequestID != record.Request.RequestID || got.Request.RequestedBy != record.Request.RequestedBy {
		t.Fatalf("request round-trip failed: %+v", got.Request)
	}
	if got.Plan.PolicyVersion != record.Plan.PolicyVersion || got.Plan.GraphVersion != record.Plan.GraphVersion {
		t.Fatalf("plan round-trip failed: %+v", got.Plan)
	}
	if len(got.Approvals) != 1 || got.Approvals[0].Actor != record.Approvals[0].Actor || got.Approvals[0].Outcome != record.Approvals[0].Outcome {
		t.Fatalf("approvals round-trip failed: %+v", got.Approvals)
	}
	if got.Result == nil || !got.Result.Success || got.Result.Output != result.Output {
		t.Fatalf("result round-trip failed: %+v", got.Result)
	}
}

func TestMemoryStore_RecordActionAudit_UpsertsByID(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 3, 18, 13, 30, 0, 0, time.UTC)

	first := ActionAuditRecord{
		ID:        "action-2",
		CreatedAt: now,
		UpdatedAt: now,
		State:     ActionStatePlanned,
		Request: ActionRequest{
			RequestID:      "req-2",
			ResourceID:     "vm:301",
			CapabilityName: "restart",
			RequestedBy:    "agent:test",
		},
	}
	second := first
	second.UpdatedAt = now.Add(2 * time.Minute)
	second.State = ActionStateCompleted
	second.Result = &ExecutionResult{Success: true, Output: "done"}

	if err := store.RecordActionAudit(first); err != nil {
		t.Fatalf("RecordActionAudit(first): %v", err)
	}
	if err := store.RecordActionAudit(second); err != nil {
		t.Fatalf("RecordActionAudit(second): %v", err)
	}

	results, err := store.GetActionAudits("vm:301", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 action audit after upsert, got %d", len(results))
	}
	if results[0].State != ActionStateCompleted {
		t.Fatalf("expected latest action state to win, got %q", results[0].State)
	}
	if results[0].Result == nil || results[0].Result.Output != "done" {
		t.Fatalf("expected latest action result to win, got %+v", results[0].Result)
	}
}

func TestSQLiteStore_GetActionAudits_AllWhenResourceIDBlank(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 3, 18, 13, 45, 0, 0, time.UTC)

	records := []ActionAuditRecord{
		{
			ID:        "action-3",
			CreatedAt: now.Add(-2 * time.Minute),
			UpdatedAt: now.Add(-2 * time.Minute),
			State:     ActionStatePlanned,
			Request: ActionRequest{
				RequestID:      "req-3",
				ResourceID:     "vm:400",
				CapabilityName: "restart",
				RequestedBy:    "agent:test",
			},
		},
		{
			ID:        "action-4",
			CreatedAt: now.Add(-time.Minute),
			UpdatedAt: now.Add(-time.Minute),
			State:     ActionStateCompleted,
			Request: ActionRequest{
				RequestID:      "req-4",
				ResourceID:     "vm:401",
				CapabilityName: "restart",
				RequestedBy:    "agent:test",
			},
		},
	}

	for _, record := range records {
		if err := store.RecordActionAudit(record); err != nil {
			t.Fatalf("RecordActionAudit(%s): %v", record.ID, err)
		}
	}

	results, err := store.GetActionAudits("", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 action audits without resource filter, got %d", len(results))
	}
	if results[0].ID != "action-4" || results[1].ID != "action-3" {
		t.Fatalf("unexpected action audit order: %+v", results)
	}
}

func TestSQLiteStore_GetRecentChanges_AllWhenResourceIDBlank(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 3, 18, 13, 50, 0, 0, time.UTC)

	changes := []ResourceChange{
		{
			ID:            "change-1",
			ResourceID:    "vm:500",
			ObservedAt:    now.Add(-2 * time.Minute),
			Kind:          ChangeRestart,
			SourceType:    SourcePlatformEvent,
			SourceAdapter: AdapterDocker,
			Confidence:    ConfidenceHigh,
			Reason:        "restart detected",
		},
		{
			ID:            "change-2",
			ResourceID:    "vm:501",
			ObservedAt:    now.Add(-time.Minute),
			Kind:          ChangeConfigUpdate,
			SourceType:    SourcePulseDiff,
			SourceAdapter: AdapterProxmox,
			Confidence:    ConfidenceMedium,
			Reason:        "configuration drift",
		},
	}

	for _, change := range changes {
		if err := store.RecordChange(change); err != nil {
			t.Fatalf("RecordChange(%s): %v", change.ID, err)
		}
	}

	results, err := store.GetRecentChanges("", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 recent changes without resource filter, got %d", len(results))
	}
	if results[0].ID != "change-2" || results[1].ID != "change-1" {
		t.Fatalf("unexpected recent change order: %+v", results)
	}
	if results[0].ResourceID != "vm:501" || results[1].ResourceID != "vm:500" {
		t.Fatalf("expected canonical resource IDs to round-trip, got %+v", results)
	}
}

func TestActionLifecycleEvent_RoundTrip(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 3, 18, 14, 0, 0, 0, time.UTC)

	events := []ActionLifecycleEvent{
		{
			ActionID:  "action-2",
			Timestamp: now,
			State:     ActionStatePlanned,
			Actor:     "system",
			Message:   "planned",
		},
		{
			ActionID:  "action-2",
			Timestamp: now.Add(1 * time.Minute),
			State:     ActionStateApproved,
			Actor:     "admin@example.com",
			Message:   "approved",
		},
	}

	for _, event := range events {
		if err := store.RecordActionLifecycleEvent(event); err != nil {
			t.Fatalf("RecordActionLifecycleEvent: %v", err)
		}
	}

	results, err := store.GetActionLifecycleEvents("action-2", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 lifecycle events, got %d", len(results))
	}
	if results[0].State != ActionStateApproved || results[1].State != ActionStatePlanned {
		t.Fatalf("unexpected lifecycle ordering: %+v", results)
	}
}

func TestExportAuditRecord_RoundTrip(t *testing.T) {
	store := newTestStore(t)
	now := time.Date(2026, 3, 18, 15, 0, 0, 0, time.UTC)

	record := ExportAuditRecord{
		ID:           "export-1",
		Timestamp:    now,
		Actor:        "agent:context-router",
		EnvelopeHash: "sha256:deadbeef",
		Decision:     ExportRedacted,
		Destination:  "local-llama",
		Redactions:   []string{"metadata.hostname", "identity.ipAddresses"},
	}

	if err := store.RecordExportAudit(record); err != nil {
		t.Fatalf("RecordExportAudit: %v", err)
	}

	results, err := store.GetExportAudits(now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetExportAudits: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 export audit, got %d", len(results))
	}

	got := results[0]
	if got.ID != record.ID || got.Decision != record.Decision || got.Destination != record.Destination {
		t.Fatalf("unexpected export audit round-trip: %+v", got)
	}
	if len(got.Redactions) != len(record.Redactions) || got.Redactions[0] != record.Redactions[0] || got.Redactions[1] != record.Redactions[1] {
		t.Fatalf("redactions round-trip failed: %+v", got.Redactions)
	}
}

func TestGetRecentChanges_RespectsTimeFilter(t *testing.T) {
	store := newTestStore(t)
	base := time.Now().UTC().Truncate(time.Second)

	old := ResourceChange{ID: "chg-old", ResourceID: "vm:1", ObservedAt: base.Add(-2 * time.Hour), Kind: ChangeStateTransition, SourceType: "proxmox", Confidence: ConfidenceHigh}
	recent := ResourceChange{ID: "chg-new", ResourceID: "vm:1", ObservedAt: base, Kind: ChangeStateTransition, SourceType: "proxmox", Confidence: ConfidenceHigh}

	for _, c := range []ResourceChange{old, recent} {
		if err := store.RecordChange(c); err != nil {
			t.Fatalf("RecordChange: %v", err)
		}
	}

	results, err := store.GetRecentChanges("vm:1", base.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (recent only), got %d", len(results))
	}
	if results[0].ID != "chg-new" {
		t.Errorf("expected chg-new, got %q", results[0].ID)
	}
}

func TestGetRecentChanges_RespectsLimit(t *testing.T) {
	store := newTestStore(t)
	base := time.Now().UTC().Truncate(time.Second)

	for i := 0; i < 5; i++ {
		c := ResourceChange{
			ID:         strings.Repeat("x", 3) + string(rune('0'+i)),
			ResourceID: "vm:2",
			ObservedAt: base.Add(time.Duration(i) * time.Second),
			Kind:       ChangeStateTransition,
			SourceType: "proxmox",
			Confidence: ConfidenceHigh,
		}
		if err := store.RecordChange(c); err != nil {
			t.Fatalf("RecordChange: %v", err)
		}
	}

	results, err := store.GetRecentChanges("vm:2", base.Add(-time.Minute), 3)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results (limit), got %d", len(results))
	}
}

func seedLegacyLinksTable(t *testing.T, legacyPath string) {
	t.Helper()

	db, err := sql.Open("sqlite", legacyPath)
	if err != nil {
		t.Fatalf("sql.Open(%q) failed: %v", legacyPath, err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS resource_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			resource_a TEXT NOT NULL,
			resource_b TEXT NOT NULL,
			primary_id TEXT NOT NULL,
			reason TEXT,
			created_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(resource_a, resource_b)
		);
	`); err != nil {
		t.Fatalf("failed to create legacy schema: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO resource_links (resource_a, resource_b, primary_id, reason, created_by, created_at)
		VALUES ('legacy-a', 'legacy-b', 'legacy-a', 'legacy migration', 'tester', CURRENT_TIMESTAMP);
	`); err != nil {
		t.Fatalf("failed to seed legacy link row: %v", err)
	}
}
