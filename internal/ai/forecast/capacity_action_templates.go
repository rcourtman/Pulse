// capacity_action_templates.go: deterministic remediation proposals for
// capacity findings.
//
// When a capacity finding crosses (or is projected to cross) a threshold,
// patrol_findings.go projects the finding into a CapacityFindingInput and
// asks this registry for a deterministic ActionPlan proposal. The proposal
// is attached to the finding's RemediationPlan and surfaced in
// FindingsPanel.tsx as an explicit approve/reject card.
//
// Every template here MUST set RequiresApproval=true. When no Pulse write
// capability is wired for the proposed remediation, the template sets
// Allowed=false and ships preflight-only intent so the operator can
// approve and act manually until a capability is added. We intentionally
// do NOT invent capabilities in pulse_control / agentexec; making a
// proposal that references an unimplemented capability would be a lie
// at the audit layer.
package forecast

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// CapacityActionPlanSource identifies forecast-driven proposals on the wire.
// FindingsPanel uses this to render the capacity-forecast approval card
// variant rather than the generic remediation plan card.
const CapacityActionPlanSource = "capacity_forecast"

// proposalTTL controls how long a proposed plan remains presentable before
// it is considered stale. Operator approval still flows through the action
// engine (which has its own freshness check), so this is just the
// presentation-side staleness for findings that go un-actioned.
const proposalTTL = 30 * time.Minute

// CapacityFindingInput is the minimum context the registry needs to build a
// deterministic capacity-remediation proposal.
//
// patrol_findings.go projects a *Finding plus any available forecast
// snapshot into this struct so the forecast package can stay free of an
// import on the broader internal/ai package.
type CapacityFindingInput struct {
	FindingID       string
	ResourceID      string
	ResourceName    string
	ResourceType    string
	Node            string
	Metric          string
	CurrentValue    float64
	PredictedValue  float64
	ThresholdValue  float64
	TimeToThreshold *time.Duration
	Now             time.Time
}

type templateKey struct {
	ResourceType string
	Metric       string
}

type capacityActionTemplate func(input CapacityFindingInput) *unifiedresources.ActionPlan

// capacityActionTemplates registers the deterministic templates.
//
// Lookups are normalized via lower-case + trim on both fields. The wire-in
// in patrol_findings.go funnels VM/CT and PBS findings through the
// canonical aliases below so callers don't have to remember the
// resource_type variants used across signal detectors and unified
// resources.
var capacityActionTemplates = map[templateKey]capacityActionTemplate{
	{ResourceType: "pbs-datastore", Metric: "usage_percent"}:         pbsDatastorePruneAndGCTemplate,
	{ResourceType: "pbs", Metric: "usage_percent"}:                   pbsDatastorePruneAndGCTemplate,
	{ResourceType: "storage", Metric: "usage_percent"}:               zfsPoolSnapshotPruneTemplate,
	{ResourceType: "qemu", Metric: "disk_usage_percent"}:             vmDiskExpandTemplate,
	{ResourceType: "vm", Metric: "disk_usage_percent"}:               vmDiskExpandTemplate,
	{ResourceType: "lxc", Metric: "disk_usage_percent"}:              vmDiskExpandTemplate,
	{ResourceType: "system-container", Metric: "disk_usage_percent"}: vmDiskExpandTemplate,
}

// BuildActionPlanForFinding returns a deterministic ActionPlan proposal for
// the given (resourceType, metric) pair, or nil if no template is
// registered.
//
// The returned plan is guaranteed to have RequiresApproval=true. Templates
// can choose Allowed=false to indicate "no write capability wired yet —
// preflight-only proposal."
func BuildActionPlanForFinding(input CapacityFindingInput) *unifiedresources.ActionPlan {
	key := templateKey{
		ResourceType: strings.ToLower(strings.TrimSpace(input.ResourceType)),
		Metric:       strings.ToLower(strings.TrimSpace(input.Metric)),
	}
	fn, ok := capacityActionTemplates[key]
	if !ok {
		return nil
	}
	plan := fn(input)
	if plan == nil {
		return nil
	}
	plan.RequiresApproval = true
	return plan
}

// HasCapacityActionTemplate reports whether a template exists for the
// (resourceType, metric) pair without constructing a plan.
func HasCapacityActionTemplate(resourceType, metric string) bool {
	_, ok := capacityActionTemplates[templateKey{
		ResourceType: strings.ToLower(strings.TrimSpace(resourceType)),
		Metric:       strings.ToLower(strings.TrimSpace(metric)),
	}]
	return ok
}

// CapacityActionTemplateKey is a registered (resourceType, metric) pair.
type CapacityActionTemplateKey struct {
	ResourceType string
	Metric       string
}

// CapacityActionTemplateKeys returns the registered (resourceType, metric)
// pairs. Useful for tests and for surfacing the catalog in observability
// tooling. Order is not stable across calls.
func CapacityActionTemplateKeys() []CapacityActionTemplateKey {
	out := make([]CapacityActionTemplateKey, 0, len(capacityActionTemplates))
	for k := range capacityActionTemplates {
		out = append(out, CapacityActionTemplateKey{ResourceType: k.ResourceType, Metric: k.Metric})
	}
	return out
}

func resolveNow(in CapacityFindingInput) time.Time {
	if !in.Now.IsZero() {
		return in.Now.UTC()
	}
	return time.Now().UTC()
}

func displayName(in CapacityFindingInput) string {
	name := strings.TrimSpace(in.ResourceName)
	if name == "" {
		name = strings.TrimSpace(in.ResourceID)
	}
	if name == "" {
		name = "(unknown)"
	}
	return name
}

func currentStateString(in CapacityFindingInput) string {
	parts := []string{fmt.Sprintf("%s=%.1f%%", in.Metric, in.CurrentValue)}
	if in.PredictedValue > 0 && in.PredictedValue != in.CurrentValue {
		parts = append(parts, fmt.Sprintf("projected=%.1f%%", in.PredictedValue))
	}
	if in.ThresholdValue > 0 {
		parts = append(parts, fmt.Sprintf("threshold=%.1f%%", in.ThresholdValue))
	}
	if in.TimeToThreshold != nil {
		parts = append(parts, fmt.Sprintf("ttb=%s", roundDuration(*in.TimeToThreshold)))
	}
	return strings.Join(parts, ", ")
}

func roundDuration(d time.Duration) time.Duration {
	if d >= 24*time.Hour {
		return d.Round(time.Hour)
	}
	if d >= time.Hour {
		return d.Round(time.Minute)
	}
	return d.Round(time.Second)
}

// --- Templates ---

func pbsDatastorePruneAndGCTemplate(in CapacityFindingInput) *unifiedresources.ActionPlan {
	now := resolveNow(in)
	name := displayName(in)

	msg := fmt.Sprintf(
		"PBS datastore %q is at %.1f%% usage (projected %.1f%%). Propose: prune snapshots against the configured retention policy, then run garbage-collect to reclaim chunk-store space. No PBS prune/GC capability is wired into Pulse yet, so this proposal is preflight-only — approve to record intent and run the remediation manually until the capability lands.",
		name, in.CurrentValue, in.PredictedValue,
	)

	return &unifiedresources.ActionPlan{
		ActionID:         "capacity-forecast-" + uuid.NewString(),
		Allowed:          false,
		RequiresApproval: true,
		ApprovalPolicy:   unifiedresources.ApprovalAdmin,
		Message:          msg,
		PlannedAt:        now,
		ExpiresAt:        now.Add(proposalTTL),
		Preflight: &unifiedresources.ActionPreflight{
			Target:          fmt.Sprintf("pbs-datastore/%s", name),
			CurrentState:    currentStateString(in),
			IntendedChange:  "Prune backups against retention policy, then run garbage-collect.",
			DryRunAvailable: false,
			DryRunSummary:   "No PBS prune/GC capability is wired into Pulse yet; this proposal records intent and surfaces the operator-facing remediation.",
			SafetyChecks: []string{
				"Operator must explicitly approve before any execution path is wired.",
				"This proposal ships with Allowed=false; the action broker will refuse execution.",
				"Verify the configured retention policy matches your recovery objectives before approving.",
			},
			VerificationSteps: []string{
				"After manual prune+GC, re-check the datastore usage on the next Patrol pass.",
				"Confirm chunk-store free space increased and that recent backups remain restorable.",
			},
			GeneratedAt: now,
		},
	}
}

func zfsPoolSnapshotPruneTemplate(in CapacityFindingInput) *unifiedresources.ActionPlan {
	now := resolveNow(in)
	name := displayName(in)

	msg := fmt.Sprintf(
		"Storage pool %q is at %.1f%% usage (projected %.1f%%). Propose: prune oldest auto-snapshots and surface the largest reclaimable datasets. No snapshot-prune capability is wired into Pulse yet, so this proposal is preflight-only — approve to record intent and run the remediation manually until the capability lands.",
		name, in.CurrentValue, in.PredictedValue,
	)

	return &unifiedresources.ActionPlan{
		ActionID:         "capacity-forecast-" + uuid.NewString(),
		Allowed:          false,
		RequiresApproval: true,
		ApprovalPolicy:   unifiedresources.ApprovalAdmin,
		Message:          msg,
		PlannedAt:        now,
		ExpiresAt:        now.Add(proposalTTL),
		Preflight: &unifiedresources.ActionPreflight{
			Target:          fmt.Sprintf("storage/%s", name),
			CurrentState:    currentStateString(in),
			IntendedChange:  "Prune oldest auto-snapshots, then list largest reclaimable datasets for review.",
			DryRunAvailable: false,
			DryRunSummary:   "No snapshot-prune capability is wired into Pulse yet; this proposal records intent and surfaces the operator-facing remediation.",
			SafetyChecks: []string{
				"Operator must explicitly approve before any execution path is wired.",
				"This proposal ships with Allowed=false; the action broker will refuse execution.",
				"Confirm snapshot retention is sufficient before approving — this proposal targets oldest auto-snapshots only.",
			},
			VerificationSteps: []string{
				"After manual snapshot prune, re-check pool usage on the next Patrol pass.",
				"Confirm that critical snapshots required for rollback or replication are still present.",
			},
			GeneratedAt: now,
		},
	}
}

func vmDiskExpandTemplate(in CapacityFindingInput) *unifiedresources.ActionPlan {
	now := resolveNow(in)
	name := displayName(in)

	rt := strings.ToUpper(strings.TrimSpace(in.ResourceType))
	if rt == "" {
		rt = "GUEST"
	}

	msg := fmt.Sprintf(
		"%s %q is at %.1f%% disk usage (projected %.1f%%). Propose: expand the guest disk by the next sensible increment, or compact the qcow2 image if the underlying allocation has grown beyond the in-guest footprint. No guest-disk write capability is wired into Pulse yet, so this proposal is preflight-only — approve to record intent and run the remediation manually until the capability lands.",
		rt, name, in.CurrentValue, in.PredictedValue,
	)

	return &unifiedresources.ActionPlan{
		ActionID:         "capacity-forecast-" + uuid.NewString(),
		Allowed:          false,
		RequiresApproval: true,
		ApprovalPolicy:   unifiedresources.ApprovalAdmin,
		Message:          msg,
		PlannedAt:        now,
		ExpiresAt:        now.Add(proposalTTL),
		Preflight: &unifiedresources.ActionPreflight{
			Target:          fmt.Sprintf("%s/%s", strings.ToLower(strings.TrimSpace(in.ResourceType)), name),
			CurrentState:    currentStateString(in),
			IntendedChange:  "Expand guest disk by the next sensible increment, or compact the qcow2 image.",
			DryRunAvailable: false,
			DryRunSummary:   "No guest-disk capability is wired into Pulse yet; this proposal records intent and surfaces the operator-facing remediation.",
			SafetyChecks: []string{
				"Operator must explicitly approve before any execution path is wired.",
				"This proposal ships with Allowed=false; the action broker will refuse execution.",
				"Take a snapshot before expanding or compacting — disk operations risk filesystem damage if interrupted.",
			},
			VerificationSteps: []string{
				"After manual expand/compact, re-check disk usage on the next Patrol pass.",
				"Inside the guest, confirm the partition and filesystem were resized to use the new capacity.",
			},
			GeneratedAt: now,
		},
	}
}
