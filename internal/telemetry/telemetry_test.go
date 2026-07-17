package telemetry

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"context"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/testutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
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
	if err := os.WriteFile(filepath.Join(dir, installIDFile), []byte("not-a-uuid\n"), 0600); err != nil {
		t.Fatalf("write install id file: %v", err)
	}

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

func TestClassifyPulseIntelligenceProActivationProof(t *testing.T) {
	tests := []struct {
		name  string
		input PulseIntelligenceProActivationProofInput
		want  PulseIntelligenceProActivationProof
	}{
		{
			name: "not started",
			want: PulseIntelligenceProActivationProof{
				ValueProofState: PulseIntelligenceProActivationValueProofNotStarted,
			},
		},
		{
			name: "starter only stays in progress",
			input: PulseIntelligenceProActivationProofInput{
				ProActivationStarterCount: 1,
			},
			want: PulseIntelligenceProActivationProof{
				ValueProofState: PulseIntelligenceProActivationValueProofInProgress,
			},
		},
		{
			name: "approved verified outcome resolves without external agent readiness",
			input: PulseIntelligenceProActivationProofInput{
				ProActivationStarterCount:    1,
				PatrolIssueEvidenceCount:     1,
				ContextualCollaborationCount: 1,
				ApprovedDecisionCount:        1,
				VerifiedOutcomeCount:         1,
			},
			want: PulseIntelligenceProActivationProof{
				Completed:       true,
				Resolved:        true,
				ValueProofState: PulseIntelligenceProActivationValueProofVerified,
			},
		},
		{
			name: "rejected governed decision records proof state before full completion",
			input: PulseIntelligenceProActivationProofInput{
				ProActivationStarterCount: 1,
				RejectedDecisionCount:     1,
			},
			want: PulseIntelligenceProActivationProof{
				ValueProofState: PulseIntelligenceProActivationValueProofGovernedDecisionRecorded,
			},
		},
		{
			name: "rejected governed decision completes without resolving",
			input: PulseIntelligenceProActivationProofInput{
				ProActivationStarterCount:    1,
				PatrolIssueEvidenceCount:     1,
				ContextualCollaborationCount: 1,
				RejectedDecisionCount:        1,
			},
			want: PulseIntelligenceProActivationProof{
				Completed:       true,
				ValueProofState: PulseIntelligenceProActivationValueProofGovernedDecisionRecorded,
			},
		},
		{
			name: "approved verified outcome resolves",
			input: PulseIntelligenceProActivationProofInput{
				ProActivationStarterCount:    1,
				PatrolIssueEvidenceCount:     1,
				ContextualCollaborationCount: 1,
				ApprovedDecisionCount:        1,
				VerifiedOutcomeCount:         1,
				ExternalAgentReady:           true,
			},
			want: PulseIntelligenceProActivationProof{
				Completed:       true,
				Resolved:        true,
				ValueProofState: PulseIntelligenceProActivationValueProofVerified,
			},
		},
		{
			name: "approved decision without verified outcome stays in progress",
			input: PulseIntelligenceProActivationProofInput{
				ProActivationStarterCount:    1,
				PatrolIssueEvidenceCount:     1,
				ContextualCollaborationCount: 1,
				ApprovedDecisionCount:        1,
				ExternalAgentReady:           true,
			},
			want: PulseIntelligenceProActivationProof{
				ValueProofState: PulseIntelligenceProActivationValueProofInProgress,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyPulseIntelligenceProActivationProof(tt.input); got != tt.want {
				t.Fatalf("ClassifyPulseIntelligenceProActivationProof() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestClassifyPulseIntelligencePatrolControlProof(t *testing.T) {
	tests := []struct {
		name  string
		input PulseIntelligencePatrolControlProofInput
		want  PulseIntelligencePatrolControlProof
	}{
		{
			name: "not started",
			want: PulseIntelligencePatrolControlProof{
				ValueProofState: PulseIntelligenceProActivationValueProofNotStarted,
			},
		},
		{
			name: "starter only stays in progress",
			input: PulseIntelligencePatrolControlProofInput{
				PatrolControlStarterCount: 1,
			},
			want: PulseIntelligencePatrolControlProof{
				ValueProofState: PulseIntelligenceProActivationValueProofInProgress,
			},
		},
		{
			name: "rejected governed decision completes without resolving",
			input: PulseIntelligencePatrolControlProofInput{
				PatrolControlStarterCount:    1,
				PatrolIssueEvidenceCount:     1,
				ContextualCollaborationCount: 1,
				RejectedDecisionCount:        1,
			},
			want: PulseIntelligencePatrolControlProof{
				Completed:       true,
				ValueProofState: PulseIntelligenceProActivationValueProofGovernedDecisionRecorded,
			},
		},
		{
			name: "approved verified outcome resolves",
			input: PulseIntelligencePatrolControlProofInput{
				PatrolControlStarterCount:    1,
				PatrolIssueEvidenceCount:     1,
				ContextualCollaborationCount: 1,
				ApprovedDecisionCount:        1,
				VerifiedOutcomeCount:         1,
				ExternalAgentReady:           true,
			},
			want: PulseIntelligencePatrolControlProof{
				Completed:       true,
				Resolved:        true,
				ValueProofState: PulseIntelligenceProActivationValueProofVerified,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyPulseIntelligencePatrolControlProof(tt.input); got != tt.want {
				t.Fatalf("ClassifyPulseIntelligencePatrolControlProof() = %#v, want %#v", got, tt.want)
			}
		})
	}

	legacy := ClassifyPulseIntelligencePatrolAutonomyProof(PulseIntelligencePatrolAutonomyProofInput{
		PatrolAutonomyStarterCount:   1,
		PatrolIssueEvidenceCount:     1,
		ContextualCollaborationCount: 1,
		ApprovedDecisionCount:        1,
		VerifiedOutcomeCount:         1,
	})
	if legacy != (PulseIntelligencePatrolAutonomyProof{
		Completed:       true,
		Resolved:        true,
		ValueProofState: PulseIntelligenceProActivationValueProofVerified,
	}) {
		t.Fatalf("legacy Patrol autonomy classifier alias drifted: %#v", legacy)
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
			PVENodes:                        3,
			VMs:                             10,
			Containers:                      5,
			AgentHosts:                      2,
			DockerContainers:                12,
			KubernetesPods:                  18,
			StoragePools:                    4,
			PhysicalDisks:                   9,
			TrueNASSystems:                  1,
			TrueNASApps:                     3,
			VMwareHosts:                     2,
			AvailabilityTargets:             6,
			AIEnabled:                       true,
			PatrolEnabled:                   true,
			DiscoveryEnabled:                true,
			NotificationsEnabled:            true,
			AIActionsEnabled:                true,
			ActiveAlerts:                    2,
			PaidLicense:                     true,
			HasAPITokens:                    true,
			UpdateAttempts30d:               4,
			UpdateSuccesses30d:              2,
			UpdateFailures30d:               1,
			UpdateLastFailureCategory:       "download",
			PulseIntelligenceLoopConfigured: true,
			PulseIntelligenceLoopActive30d:  true,
			PulseIntelligenceCompleteOperationsLoop30d:                     true,
			PulseIntelligenceApprovedExecutionLoop30d:                      true,
			PulseIntelligenceResolvedOperationsLoop30d:                     true,
			PulseIntelligencePatrolControlCompletedOperationsLoop30d:       true,
			PulseIntelligencePatrolControlResolvedOperationsLoop30d:        true,
			PulseIntelligencePatrolControlPaidCompletedOperationsLoop30d:   true,
			PulseIntelligencePatrolControlPaidResolvedOperationsLoop30d:    true,
			PulseIntelligenceProActivationCompletedOperationsLoop30d:       true,
			PulseIntelligenceProActivationResolvedOperationsLoop30d:        true,
			PulseIntelligenceProActivationPaidCompletedOperationsLoop30d:   true,
			PulseIntelligenceProActivationPaidResolvedOperationsLoop30d:    true,
			PulseIntelligenceGovernedActionActive30d:                       true,
			PulseIntelligenceAssistantOperationsLoop30d:                    true,
			PulseIntelligenceAssistantApprovedExecutionLoop30d:             true,
			PulseIntelligenceAssistantApprovedActionSuccessLoop30d:         true,
			PulseIntelligenceAssistantResolvedOperationsLoop30d:            true,
			PulseIntelligenceExternalAgentOperationsLoop30d:                true,
			PulseIntelligenceExternalAgentApprovedExecutionLoop30d:         true,
			PulseIntelligenceExternalAgentApprovedActionSuccessLoop30d:     true,
			PulseIntelligenceExternalAgentResolvedOperationsLoop30d:        true,
			PulseIntelligenceMCPAdapterOperationsLoop30d:                   true,
			PulseIntelligenceMCPAdapterApprovedExecutionLoop30d:            true,
			PulseIntelligenceMCPAdapterApprovedActionSuccessLoop30d:        true,
			PulseIntelligenceMCPAdapterResolvedOperationsLoop30d:           true,
			PulseIntelligenceOperationsLoopStarterRequests30d:              5,
			PulseIntelligenceAssistantOperationsLoopStarterRequests30d:     2,
			PulseIntelligencePatrolOperationsLoopStarterRequests30d:        1,
			PulseIntelligencePatrolControlOperationsLoopStarterRequests30d: 3,
			PulseIntelligenceProActivationOperationsLoopStarterRequests30d: 1,
			PulseIntelligenceMCPOperationsLoopStarterRequests30d:           1,
			PulseIntelligenceAssistantAICalls30d:                           7,
			PulseIntelligenceAssistantContextAICalls30d:                    4,
			PulseIntelligenceAssistantToolCalls30d:                         9,
			PulseIntelligencePatrolAICalls30d:                              3,
			PulseIntelligencePatrolRuns30d:                                 4,
			PulseIntelligencePatrolNewFindings30d:                          5,
			PulseIntelligencePatrolInvestigations30d:                       6,
			PulseIntelligencePatrolResolvedFindings30d:                     3,
			PulseIntelligencePatrolAutofixes30d:                            2,
			PulseIntelligenceExternalAgentEnabled:                          true,
			PulseIntelligenceExternalAgentUsed30d:                          true,
			PulseIntelligenceMCPAdapterUsed30d:                             true,
			PulseIntelligenceExternalAgentContextRequests30d:               8,
			PulseIntelligenceExternalAgentEventStreamRequests30d:           2,
			PulseIntelligenceExternalAgentProvisioningRequests30d:          1,
			PulseIntelligenceExternalAgentOperatorStateRequests30d:         3,
			PulseIntelligenceExternalAgentFindingRequests30d:               5,
			PulseIntelligenceExternalAgentActionRequests30d:                4,
			PulseIntelligenceActionPlans30d:                                6,
			PulseIntelligenceApprovalRequests30d:                           2,
			PulseIntelligenceRejectedActionDecisions30d:                    1,
			PulseIntelligenceApprovedActionDecisions30d:                    1,
			PulseIntelligenceApprovedActionAttempts30d:                     1,
			PulseIntelligenceApprovedActionSuccesses30d:                    1,
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
	if ping.AgentHosts != 2 || ping.DockerContainers != 12 || ping.KubernetesPods != 18 {
		t.Fatalf("expanded workload counts not applied: %#v", ping)
	}
	if ping.StoragePools != 4 || ping.PhysicalDisks != 9 {
		t.Fatalf("expanded storage counts not applied: %#v", ping)
	}
	if ping.TrueNASSystems != 1 || ping.TrueNASApps != 3 || ping.VMwareHosts != 2 || ping.AvailabilityTargets != 6 {
		t.Fatalf("expanded platform counts not applied: %#v", ping)
	}
	if !ping.AIEnabled {
		t.Fatal("AIEnabled should be true")
	}
	if !ping.PatrolEnabled || !ping.DiscoveryEnabled || !ping.NotificationsEnabled || !ping.AIActionsEnabled {
		t.Fatalf("expanded feature flags not applied: %#v", ping)
	}
	if !ping.PaidLicense {
		t.Fatal("PaidLicense should be true")
	}
	if !ping.HasAPITokens {
		t.Fatal("HasAPITokens should be true")
	}
	if ping.UpdateAttempts30d != 4 || ping.UpdateSuccesses30d != 2 || ping.UpdateFailures30d != 1 ||
		ping.UpdateLastFailureCategory != "download" {
		t.Fatalf("update telemetry counters not applied: %#v", ping)
	}
	if !ping.PulseIntelligenceLoopConfigured || !ping.PulseIntelligenceLoopActive30d ||
		!ping.PulseIntelligenceCompleteOperationsLoop30d ||
		!ping.PulseIntelligenceApprovedExecutionLoop30d ||
		!ping.PulseIntelligenceResolvedOperationsLoop30d ||
		!ping.PulseIntelligencePatrolControlCompletedOperationsLoop30d ||
		!ping.PulseIntelligencePatrolControlResolvedOperationsLoop30d ||
		!ping.PulseIntelligencePatrolControlPaidCompletedOperationsLoop30d ||
		!ping.PulseIntelligencePatrolControlPaidResolvedOperationsLoop30d ||
		!ping.PulseIntelligenceProActivationCompletedOperationsLoop30d ||
		!ping.PulseIntelligenceProActivationResolvedOperationsLoop30d ||
		!ping.PulseIntelligenceProActivationPaidCompletedOperationsLoop30d ||
		!ping.PulseIntelligenceProActivationPaidResolvedOperationsLoop30d ||
		!ping.PulseIntelligenceGovernedActionActive30d ||
		!ping.PulseIntelligenceAssistantOperationsLoop30d ||
		!ping.PulseIntelligenceAssistantApprovedExecutionLoop30d ||
		!ping.PulseIntelligenceAssistantApprovedActionSuccessLoop30d ||
		!ping.PulseIntelligenceAssistantResolvedOperationsLoop30d ||
		!ping.PulseIntelligenceExternalAgentOperationsLoop30d ||
		!ping.PulseIntelligenceExternalAgentApprovedExecutionLoop30d ||
		!ping.PulseIntelligenceExternalAgentApprovedActionSuccessLoop30d ||
		!ping.PulseIntelligenceExternalAgentResolvedOperationsLoop30d ||
		!ping.PulseIntelligenceMCPAdapterOperationsLoop30d ||
		!ping.PulseIntelligenceMCPAdapterApprovedExecutionLoop30d ||
		!ping.PulseIntelligenceMCPAdapterApprovedActionSuccessLoop30d ||
		!ping.PulseIntelligenceMCPAdapterResolvedOperationsLoop30d {
		t.Fatalf("Pulse Intelligence adoption state not applied: %#v", ping)
	}
	if ping.PulseIntelligenceAssistantAICalls30d != 7 ||
		ping.PulseIntelligenceOperationsLoopStarterRequests30d != 5 ||
		ping.PulseIntelligenceAssistantOperationsLoopStarterRequests30d != 2 ||
		ping.PulseIntelligencePatrolOperationsLoopStarterRequests30d != 1 ||
		ping.PulseIntelligencePatrolControlOperationsLoopStarterRequests30d != 3 ||
		ping.PulseIntelligenceProActivationOperationsLoopStarterRequests30d != 1 ||
		ping.PulseIntelligenceMCPOperationsLoopStarterRequests30d != 1 ||
		ping.PulseIntelligenceAssistantContextAICalls30d != 4 ||
		ping.PulseIntelligenceAssistantToolCalls30d != 9 ||
		ping.PulseIntelligencePatrolAICalls30d != 3 ||
		ping.PulseIntelligencePatrolRuns30d != 4 ||
		ping.PulseIntelligencePatrolNewFindings30d != 5 ||
		ping.PulseIntelligencePatrolInvestigations30d != 6 ||
		ping.PulseIntelligencePatrolResolvedFindings30d != 3 ||
		ping.PulseIntelligencePatrolAutofixes30d != 2 ||
		ping.PulseIntelligenceActionPlans30d != 6 ||
		ping.PulseIntelligenceApprovalRequests30d != 2 ||
		ping.PulseIntelligenceRejectedActionDecisions30d != 1 ||
		ping.PulseIntelligenceApprovedActionDecisions30d != 1 ||
		ping.PulseIntelligenceApprovedActionAttempts30d != 1 ||
		ping.PulseIntelligenceApprovedActionSuccesses30d != 1 {
		t.Fatalf("Pulse Intelligence counters not applied: %#v", ping)
	}
	if !ping.PulseIntelligenceExternalAgentEnabled || !ping.PulseIntelligenceExternalAgentUsed30d ||
		!ping.PulseIntelligenceMCPAdapterUsed30d {
		t.Fatalf("Pulse Intelligence external-agent booleans not applied: %#v", ping)
	}
	if ping.PulseIntelligenceExternalAgentContextRequests30d != 8 ||
		ping.PulseIntelligenceExternalAgentEventStreamRequests30d != 2 ||
		ping.PulseIntelligenceExternalAgentProvisioningRequests30d != 1 ||
		ping.PulseIntelligenceExternalAgentOperatorStateRequests30d != 3 ||
		ping.PulseIntelligenceExternalAgentFindingRequests30d != 5 ||
		ping.PulseIntelligenceExternalAgentActionRequests30d != 4 {
		t.Fatalf("Pulse Intelligence external-agent class counters not applied: %#v", ping)
	}
}

func TestApplyUpdateTelemetrySnapshotSummarizesHistory(t *testing.T) {
	history, err := updates.NewUpdateHistory(t.TempDir())
	if err != nil {
		t.Fatalf("NewUpdateHistory: %v", err)
	}
	now := time.Date(2026, 6, 28, 14, 0, 0, 0, time.UTC)
	ctx := context.Background()
	entries := []updates.UpdateHistoryEntry{
		{
			EventID:   "old-success",
			Timestamp: now.Add(-(installIDRotationWindow + time.Hour)),
			Action:    "update",
			Status:    updates.StatusSuccess,
		},
		{
			EventID:   "recent-success",
			Timestamp: now.Add(-2 * time.Hour),
			Action:    "update",
			Status:    updates.StatusSuccess,
		},
		{
			EventID:   "recent-failed-download",
			Timestamp: now.Add(-time.Hour),
			Action:    "update",
			Status:    updates.StatusFailed,
			Error: &updates.UpdateError{
				Message: "failed to download update: upstream timed out",
				Details: "raw details stay local",
			},
		},
		{
			EventID:   "recent-check",
			Timestamp: now.Add(-30 * time.Minute),
			Action:    "check",
			Status:    updates.StatusSuccess,
		},
	}
	for _, entry := range entries {
		if _, err := history.CreateEntry(ctx, entry); err != nil {
			t.Fatalf("CreateEntry(%s): %v", entry.EventID, err)
		}
	}

	var snap Snapshot
	ApplyUpdateTelemetrySnapshot(&snap, history, now)

	if snap.UpdateAttempts30d != 2 {
		t.Fatalf("UpdateAttempts30d = %d, want 2", snap.UpdateAttempts30d)
	}
	if snap.UpdateSuccesses30d != 1 {
		t.Fatalf("UpdateSuccesses30d = %d, want 1", snap.UpdateSuccesses30d)
	}
	if snap.UpdateFailures30d != 1 {
		t.Fatalf("UpdateFailures30d = %d, want 1", snap.UpdateFailures30d)
	}
	if snap.UpdateLastFailureCategory != "download" {
		t.Fatalf("UpdateLastFailureCategory = %q, want download", snap.UpdateLastFailureCategory)
	}
}

func TestApplyUpdateTelemetrySnapshotDoesNotExposeRawFailureText(t *testing.T) {
	history, err := updates.NewUpdateHistory(t.TempDir())
	if err != nil {
		t.Fatalf("NewUpdateHistory: %v", err)
	}
	now := time.Date(2026, 6, 28, 14, 0, 0, 0, time.UTC)
	if _, err := history.CreateEntry(context.Background(), updates.UpdateHistoryEntry{
		EventID:   "recent-sensitive-failure",
		Timestamp: now.Add(-time.Minute),
		Action:    "update",
		Status:    updates.StatusFailed,
		Error: &updates.UpdateError{
			Code:    "UPSTREAM_503",
			Message: "failed for https://updates.example.test/private/build?token=secret",
			Details: "/home/alice/pulse/update.log contained local command output",
		},
	}); err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}

	var snap Snapshot
	ApplyUpdateTelemetrySnapshot(&snap, history, now)

	if snap.UpdateLastFailureCategory != "unknown" {
		t.Fatalf("UpdateLastFailureCategory = %q, want unknown", snap.UpdateLastFailureCategory)
	}
	for _, disallowed := range []string{"https://", "updates.example", "/home/alice", "token=", "command output"} {
		if strings.Contains(snap.UpdateLastFailureCategory, disallowed) {
			t.Fatalf("UpdateLastFailureCategory leaked raw failure text %q in %q", disallowed, snap.UpdateLastFailureCategory)
		}
	}
}

func TestPulseIntelligenceTelemetryFieldsAreDisclosed(t *testing.T) {
	pingType := reflect.TypeOf(Ping{})
	fieldLabels := make([]string, 0)
	for i := 0; i < pingType.NumField(); i++ {
		jsonName := strings.Split(pingType.Field(i).Tag.Get("json"), ",")[0]
		if !strings.HasPrefix(jsonName, "pulse_intelligence_") {
			continue
		}
		fieldLabels = append(fieldLabels, normalizedTelemetryDisclosureLabel(jsonName))
	}
	if len(fieldLabels) == 0 {
		t.Fatal("expected Pulse Intelligence telemetry fields on Ping")
	}

	for _, relativePath := range []string{
		filepath.Join("..", "..", "docs", "PRIVACY.md"),
		filepath.Join("..", "..", "frontend-modern", "public", "docs", "PRIVACY.md"),
	} {
		raw, err := os.ReadFile(relativePath)
		if err != nil {
			t.Fatalf("read %s: %v", relativePath, err)
		}
		doc := normalizedTelemetryDisclosureTableText(string(raw))
		for _, label := range fieldLabels {
			if !strings.Contains(doc, label) {
				t.Errorf("%s must disclose Pulse Intelligence telemetry field %q", relativePath, label)
			}
		}
	}
}

func TestTelemetryPrivacyDocsDisclosePseudonymousIdentityAndIPHandling(t *testing.T) {
	for _, relativePath := range []string{
		filepath.Join("..", "..", "docs", "PRIVACY.md"),
		filepath.Join("..", "..", "frontend-modern", "public", "docs", "PRIVACY.md"),
	} {
		raw, err := os.ReadFile(relativePath)
		if err != nil {
			t.Fatalf("read %s: %v", relativePath, err)
		}
		content := string(raw)
		normalized := normalizedTelemetryDisclosureText(content)

		for _, required := range []string{
			"outbound usage telemetry",
			"enabled by default",
			"rotating pseudonymous install ID",
			"PULSE_TELEMETRY=false",
			"The license server uses request IP addresses transiently for abuse/rate limiting",
		} {
			if !strings.Contains(content, required) {
				t.Errorf("%s must disclose %q", relativePath, required)
			}
		}
		if !strings.Contains(normalized, "does not store ip addresses in telemetry rows") {
			t.Errorf("%s must disclose that telemetry rows do not store IP addresses", relativePath)
		}
		if strings.Contains(normalized, "anonymous telemetry") {
			t.Errorf("%s must not describe outbound usage telemetry as anonymous", relativePath)
		}
	}
}

func normalizedTelemetryDisclosureTableText(value string) string {
	tableLines := make([]string, 0)
	for _, line := range strings.Split(value, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			tableLines = append(tableLines, line)
		}
	}
	return normalizedTelemetryDisclosureText(strings.Join(tableLines, "\n"))
}

func normalizedTelemetryDisclosureLabel(jsonName string) string {
	label := strings.TrimPrefix(jsonName, "pulse_intelligence_")
	return normalizedTelemetryDisclosureText("Pulse Intelligence " + strings.ReplaceAll(label, "_", " "))
}

func normalizedTelemetryDisclosureText(value string) string {
	value = strings.ToLower(value)
	replacer := strings.NewReplacer(
		"`", " ",
		"|", " ",
		"_", " ",
		"-", " ",
		"/", " ",
		",", " ",
		".", " ",
		":", " ",
		";", " ",
		"*", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
	)
	value = replacer.Replace(value)
	return strings.Join(strings.Fields(value), " ")
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
		Version:  "v6.0.0-rc.1-45-gABCDEF",
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
	if preview.Version != "6.0.0-rc.1+git.45.gabcdef" {
		t.Fatalf("preview version = %q, want normalized development version", preview.Version)
	}
	if preview.VersionRaw != "v6.0.0-rc.1-45-gABCDEF" {
		t.Fatalf("preview raw version = %q, want original version string", preview.VersionRaw)
	}
	if preview.VersionChannel != "dev" {
		t.Fatalf("preview version channel = %q, want dev", preview.VersionChannel)
	}
	if preview.VersionBuild != "git.45.gabcdef" {
		t.Fatalf("preview version build = %q, want git.45.gabcdef", preview.VersionBuild)
	}
	if !preview.VersionDevelopment {
		t.Fatal("expected preview to mark development build")
	}
	if preview.VersionPublished {
		t.Fatal("development preview must not be marked as published release")
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
		_ = json.Unmarshal(body, &lastPing)
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
		InstallID:          "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Version:            "6.0.0-rc.1",
		VersionRaw:         "v6.0.0-rc.1",
		VersionChannel:     "rc",
		VersionDevelopment: false,
		VersionPublished:   true,
		Event:              "startup",
		Platform:           "docker",
		OS:                 "linux",
		Arch:               "amd64",
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
	if lastPing.Version != "6.0.0-rc.1" {
		t.Errorf("version = %q, want %q", lastPing.Version, "6.0.0-rc.1")
	}
	if lastPing.VersionRaw != "v6.0.0-rc.1" {
		t.Errorf("version_raw = %q, want %q", lastPing.VersionRaw, "v6.0.0-rc.1")
	}
	if lastPing.VersionChannel != "rc" {
		t.Errorf("version_channel = %q, want rc", lastPing.VersionChannel)
	}
	if !lastPing.VersionPublished {
		t.Error("expected version_is_published_release to be true")
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

func TestSendEvent_SuppressedWhileMockModeEnabled(t *testing.T) {
	var received atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	origEndpoint := pingEndpoint
	pingEndpoint = ts.URL
	defer func() { pingEndpoint = origEndpoint }()

	testutil.SetMockMode(t, true)

	sendEvent(context.Background(), Ping{InstallID: uuid.New().String()}, nil, "startup")
	sendEvent(context.Background(), Ping{InstallID: uuid.New().String()}, nil, "heartbeat")

	if got := received.Load(); got != 0 {
		t.Fatalf("expected no telemetry pings while mock mode is enabled, got %d", got)
	}
}

func TestSendEvent_SendsWhenMockModeDisabled(t *testing.T) {
	var received atomic.Int32
	var lastPing Ping
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &lastPing)
		received.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	origEndpoint := pingEndpoint
	pingEndpoint = ts.URL
	defer func() { pingEndpoint = origEndpoint }()

	testutil.SetMockMode(t, false)

	sendEvent(context.Background(), Ping{InstallID: uuid.New().String()}, nil, "heartbeat")

	if got := received.Load(); got != 1 {
		t.Fatalf("expected 1 telemetry ping with mock mode disabled, got %d", got)
	}
	if lastPing.Event != "heartbeat" {
		t.Errorf("event = %q, want %q", lastPing.Event, "heartbeat")
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
