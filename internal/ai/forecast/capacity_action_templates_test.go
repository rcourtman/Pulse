package forecast

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestBuildActionPlanForFinding_ReturnsNilForUnknownPair(t *testing.T) {
	plan := BuildActionPlanForFinding(CapacityFindingInput{
		FindingID:    "f-1",
		ResourceType: "wireguard-tunnel",
		Metric:       "handshakes_per_minute",
	})
	if plan != nil {
		t.Fatalf("expected nil plan for unknown (resourceType, metric); got %+v", plan)
	}
}

func TestBuildActionPlanForFinding_RequiresApprovalInvariantHoldsForEveryTemplate(t *testing.T) {
	for _, key := range CapacityActionTemplateKeys() {
		t.Run(key.ResourceType+"/"+key.Metric, func(t *testing.T) {
			plan := BuildActionPlanForFinding(CapacityFindingInput{
				FindingID:      "f-" + key.ResourceType,
				ResourceID:     "res/" + key.ResourceType,
				ResourceName:   "test-" + key.ResourceType,
				ResourceType:   key.ResourceType,
				Metric:         key.Metric,
				CurrentValue:   88.0,
				PredictedValue: 95.0,
				ThresholdValue: 90.0,
			})
			if plan == nil {
				t.Fatal("expected plan, got nil")
			}
			if !plan.RequiresApproval {
				t.Errorf("RequiresApproval = false; templates MUST set RequiresApproval=true")
			}
			if plan.Allowed {
				t.Errorf("Allowed = true; lane currently ships preflight-only proposals (no write capability wired). " +
					"If a capability lands later, update this assertion alongside the template.")
			}
			if plan.ApprovalPolicy != unifiedresources.ApprovalAdmin {
				t.Errorf("ApprovalPolicy = %q; want %q", plan.ApprovalPolicy, unifiedresources.ApprovalAdmin)
			}
			if plan.Message == "" {
				t.Error("Message must be non-empty operator-facing description")
			}
			if plan.ActionID == "" {
				t.Error("ActionID must be set")
			}
			if !strings.HasPrefix(plan.ActionID, "capacity-forecast-") {
				t.Errorf("ActionID = %q; want capacity-forecast- prefix so audit can identify forecast-driven actions", plan.ActionID)
			}
			if plan.Preflight == nil {
				t.Fatal("Preflight is required so operators can see the proposed change before approving")
			}
			if plan.Preflight.IntendedChange == "" {
				t.Error("Preflight.IntendedChange must describe the proposed remediation")
			}
			if plan.Preflight.DryRunAvailable {
				t.Error("Preflight.DryRunAvailable = true; no provider dry-run exists yet for these templates")
			}
			if len(plan.Preflight.SafetyChecks) == 0 {
				t.Error("Preflight.SafetyChecks must be non-empty so the audit trail records the gate")
			}
			if len(plan.Preflight.VerificationSteps) == 0 {
				t.Error("Preflight.VerificationSteps must be non-empty so post-action verification is documented")
			}
			if plan.PlannedAt.IsZero() {
				t.Error("PlannedAt must be set")
			}
			if !plan.ExpiresAt.After(plan.PlannedAt) {
				t.Errorf("ExpiresAt (%v) must be after PlannedAt (%v)", plan.ExpiresAt, plan.PlannedAt)
			}
		})
	}
}

func TestBuildActionPlanForFinding_PBSDatastoreTemplateMessageMentionsPruneAndGC(t *testing.T) {
	plan := BuildActionPlanForFinding(CapacityFindingInput{
		FindingID:      "f-pbs-1",
		ResourceID:     "pbs-1/datastore/main",
		ResourceName:   "main",
		ResourceType:   "pbs-datastore",
		Metric:         "usage_percent",
		CurrentValue:   91.4,
		PredictedValue: 96.0,
		ThresholdValue: 90.0,
	})
	if plan == nil {
		t.Fatal("expected plan, got nil")
	}
	msg := strings.ToLower(plan.Message)
	if !strings.Contains(msg, "prune") {
		t.Errorf("PBS template message missing prune verb: %q", plan.Message)
	}
	if !strings.Contains(msg, "garbage-collect") && !strings.Contains(msg, "garbage collect") && !strings.Contains(msg, "gc") {
		t.Errorf("PBS template message missing garbage-collect verb: %q", plan.Message)
	}
	if !strings.Contains(plan.Message, "91.4") {
		t.Errorf("PBS template message missing current value 91.4: %q", plan.Message)
	}
	if !strings.Contains(plan.Message, "96.0") {
		t.Errorf("PBS template message missing projected value 96.0: %q", plan.Message)
	}
}

func TestBuildActionPlanForFinding_StoragePoolTemplateMessageMentionsSnapshotPrune(t *testing.T) {
	plan := BuildActionPlanForFinding(CapacityFindingInput{
		FindingID:      "f-zfs-1",
		ResourceID:     "node-a/storage/tank",
		ResourceName:   "tank",
		ResourceType:   "storage",
		Metric:         "usage_percent",
		CurrentValue:   87.2,
		PredictedValue: 93.5,
		ThresholdValue: 90.0,
	})
	if plan == nil {
		t.Fatal("expected plan, got nil")
	}
	if !strings.Contains(strings.ToLower(plan.Message), "snapshot") {
		t.Errorf("storage template message missing snapshot reference: %q", plan.Message)
	}
}

func TestBuildActionPlanForFinding_VMDiskTemplateMessageMentionsExpand(t *testing.T) {
	plan := BuildActionPlanForFinding(CapacityFindingInput{
		FindingID:      "f-vm-1",
		ResourceID:     "node-a/qemu/101",
		ResourceName:   "appserver",
		ResourceType:   "qemu",
		Metric:         "disk_usage_percent",
		CurrentValue:   89.1,
		PredictedValue: 95.0,
		ThresholdValue: 90.0,
	})
	if plan == nil {
		t.Fatal("expected plan, got nil")
	}
	if !strings.Contains(strings.ToLower(plan.Message), "expand") {
		t.Errorf("vm disk template message missing expand verb: %q", plan.Message)
	}
}

func TestBuildActionPlanForFinding_NormalizesCaseAndWhitespace(t *testing.T) {
	plan := BuildActionPlanForFinding(CapacityFindingInput{
		FindingID:    "f-norm",
		ResourceType: "  STORAGE  ",
		Metric:       "  Usage_Percent  ",
		CurrentValue: 80,
	})
	if plan == nil {
		t.Fatal("expected plan after case/whitespace normalization, got nil")
	}
}

func TestBuildActionPlanForFinding_RespectsExplicitNow(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	plan := BuildActionPlanForFinding(CapacityFindingInput{
		FindingID:    "f-now",
		ResourceType: "storage",
		Metric:       "usage_percent",
		CurrentValue: 80,
		Now:          now,
	})
	if plan == nil {
		t.Fatal("expected plan, got nil")
	}
	if !plan.PlannedAt.Equal(now) {
		t.Errorf("PlannedAt = %v; want %v (caller's Now must be honored for deterministic tests)", plan.PlannedAt, now)
	}
	wantExpiry := now.Add(proposalTTL)
	if !plan.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("ExpiresAt = %v; want %v", plan.ExpiresAt, wantExpiry)
	}
}

func TestHasCapacityActionTemplate(t *testing.T) {
	cases := []struct {
		name         string
		resourceType string
		metric       string
		want         bool
	}{
		{"pbs-datastore-usage", "pbs-datastore", "usage_percent", true},
		{"storage-usage", "storage", "usage_percent", true},
		{"qemu-disk", "qemu", "disk_usage_percent", true},
		{"vm-disk-alias", "vm", "disk_usage_percent", true},
		{"lxc-disk", "lxc", "disk_usage_percent", true},
		{"system-container-disk-alias", "system-container", "disk_usage_percent", true},
		{"unknown-pair", "node", "uptime_seconds", false},
		{"capacity-cpu-not-registered", "qemu", "cpu_percent", false},
		{"empty-strings", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := HasCapacityActionTemplate(tc.resourceType, tc.metric)
			if got != tc.want {
				t.Errorf("HasCapacityActionTemplate(%q, %q) = %v; want %v", tc.resourceType, tc.metric, got, tc.want)
			}
		})
	}
}

func TestCapacityActionPlanSourceIsStable(t *testing.T) {
	// The frontend FindingsPanel.tsx looks for this exact string to
	// distinguish the capacity-forecast approval card from the generic
	// remediation plan card. If you change this constant, update the
	// frontend at the same time.
	if CapacityActionPlanSource != "capacity_forecast" {
		t.Fatalf("CapacityActionPlanSource = %q; frontend depends on the literal %q", CapacityActionPlanSource, "capacity_forecast")
	}
}
