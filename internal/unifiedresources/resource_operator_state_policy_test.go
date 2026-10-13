package unifiedresources

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAutoRemediationPolicyRequiresExplicitCapabilityAndValidWindow(t *testing.T) {
	state := ResourceOperatorState{
		CanonicalID:           "vm:42",
		AutoRemediationPolicy: AutoRemediationPolicy{Enabled: true},
	}
	if err := ValidateResourceOperatorState(state); err == nil {
		t.Fatal("enabled policy without capabilities must fail")
	}
	state.AutoRemediationPolicy = AutoRemediationPolicy{
		Enabled:         true,
		CapabilityNames: []string{" restart ", "restart"},
		Window: &AutoRemediationWindow{
			Timezone: "Europe/London", StartMinute: 60, EndMinute: 180,
		},
	}
	normalized := NormalizeResourceOperatorState(state)
	if err := ValidateResourceOperatorState(normalized); err != nil {
		t.Fatalf("valid policy: %v", err)
	}
	if len(normalized.AutoRemediationPolicy.CapabilityNames) != 1 || normalized.AutoRemediationPolicy.CapabilityNames[0] != "restart" {
		t.Fatalf("normalized capabilities = %#v", normalized.AutoRemediationPolicy.CapabilityNames)
	}
}

func TestResourceOperatorStateAllowsAutoRemediationInsideDailyWindow(t *testing.T) {
	state := ResourceOperatorState{
		AutoRemediationPolicy: AutoRemediationPolicy{
			Enabled:         true,
			CapabilityNames: []string{"restart"},
			Window: &AutoRemediationWindow{
				Timezone: "UTC", StartMinute: 23 * 60, EndMinute: 2 * 60,
			},
		},
	}
	if !state.AllowsAutoRemediationAt("restart", time.Date(2026, 7, 10, 23, 30, 0, 0, time.UTC)) {
		t.Fatal("cross-midnight window should allow late segment")
	}
	if !state.AllowsAutoRemediationAt("restart", time.Date(2026, 7, 11, 1, 30, 0, 0, time.UTC)) {
		t.Fatal("cross-midnight window should allow early segment")
	}
	if state.AllowsAutoRemediationAt("restart", time.Date(2026, 7, 11, 3, 0, 0, 0, time.UTC)) {
		t.Fatal("outside window must deny")
	}
	state.NeverAutoRemediate = true
	if state.AllowsAutoRemediationAt("restart", time.Date(2026, 7, 10, 23, 30, 0, 0, time.UTC)) {
		t.Fatal("never-auto-remediate must override explicit scope")
	}
}

func TestResourceOperatorStateInheritsGlobalAutonomyWithoutOverrides(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	if !(ResourceOperatorState{}).AllowsAutoRemediationAt("restart", now) {
		t.Fatal("an empty resource policy must inherit the tenant Patrol mode")
	}
	if (ResourceOperatorState{NeverAutoRemediate: true}).AllowsAutoRemediationAt("restart", now) {
		t.Fatal("the explicit resource opt-out must override inherited autonomy")
	}
	if (ResourceOperatorState{LifecycleState: LifecycleStateRetired}).AllowsAutoRemediationAt("restart", now) {
		t.Fatal("retired resources must not inherit automatic remediation")
	}
	if (ResourceOperatorState{AutoRemediationPolicy: AutoRemediationPolicy{CapabilityNames: []string{"restart"}}}).AllowsAutoRemediationAt("restart", now) {
		t.Fatal("an explicitly disabled narrowing policy must deny its configured scope")
	}
}

func TestAutoRemediationPolicySQLiteRoundTrip(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore: %v", err)
	}
	defer store.Close()

	state := ResourceOperatorState{
		CanonicalID:    "docker:container:web",
		MonitoringMode: MonitoringModeMuted,
		LifecycleState: LifecycleStateRetired,
		AutoRemediationPolicy: AutoRemediationPolicy{
			Enabled:         true,
			CapabilityNames: []string{"restart"},
			Window: &AutoRemediationWindow{
				Timezone: "Europe/London", StartMinute: 120, EndMinute: 300,
			},
		},
		SetAt: time.Now().UTC(),
		SetBy: "operator@example.com",
	}
	if err := store.SetResourceOperatorState(state); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}
	got, found, err := store.GetResourceOperatorState(state.CanonicalID)
	if err != nil || !found {
		t.Fatalf("GetResourceOperatorState: found=%v err=%v", found, err)
	}
	if !got.AutoRemediationPolicy.Enabled || len(got.AutoRemediationPolicy.CapabilityNames) != 1 || got.AutoRemediationPolicy.Window == nil || got.AutoRemediationPolicy.Window.Timezone != "Europe/London" {
		t.Fatalf("round-trip policy = %#v", got.AutoRemediationPolicy)
	}
	if got.MonitoringMode != MonitoringModeMuted || got.LifecycleState != LifecycleStateRetired {
		t.Fatalf("round-trip monitoring policy = %+v", got)
	}
}

func TestSQLiteOperatorStateNullPolicyColumnNormalizesOnRead(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), defaultOrgID)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	saved := ResourceOperatorState{
		CanonicalID:          "docker:host-1/ct-nginx",
		IntentionallyOffline: true,
		SetAt:                time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC),
		SetBy:                "operator:richard",
	}
	if err := store.SetResourceOperatorState(saved); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, found, err := store.GetResourceOperatorState(saved.CanonicalID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !found {
		t.Fatal("expected entry after set")
	}
	if got.AutoRemediationPolicy.CapabilityNames == nil {
		t.Fatal("CapabilityNames must be normalized when the policy column is NULL")
	}

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), `"capabilityNames":null`) ||
		!strings.Contains(string(encoded), `"capabilityNames":[]`) {
		t.Fatalf("wire payload must contain an empty capabilityNames list, got: %s", encoded)
	}
}

func TestActiveMaintenanceCoverageAtInheritsOnlyDescendantScopedAncestors(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	window := func(endIn time.Duration, scope MaintenanceScope, reason string) ResourceOperatorState {
		start, end := now.Add(-time.Hour), now.Add(endIn)
		return ResourceOperatorState{MaintenanceStartAt: &start, MaintenanceEndAt: &end, MaintenanceScope: scope, MaintenanceReason: reason}
	}
	states := map[string]ResourceOperatorState{
		"node":    window(2*time.Hour, MaintenanceScopeResourceAndDescendants, "node patching"),
		"cluster": window(4*time.Hour, MaintenanceScopeResource, "cluster only"),
	}
	read := func(id string) (ResourceOperatorState, bool) {
		state, ok := states[id]
		return state, ok
	}

	// No own state: the descendant-scoped node window covers the guest; the
	// resource-only cluster window, though it ends later, does not.
	coverage, ok := ActiveMaintenanceCoverageAt(now, "guest", nil, []string{"node", "cluster"}, read)
	if !ok || !coverage.Inherited || coverage.SourceID != "node" || coverage.Reason != "node patching" || !coverage.Occurrence.EndAt.Equal(now.Add(2*time.Hour)) {
		t.Fatalf("coverage = %+v (ok=%v), want the node's inherited window", coverage, ok)
	}

	// An own window ending earlier yields to the later inherited one; ending
	// later, it wins.
	short := window(time.Hour, MaintenanceScopeResource, "guest short")
	if coverage, _ := ActiveMaintenanceCoverageAt(now, "guest", &short, []string{"node"}, read); coverage.SourceID != "node" {
		t.Fatalf("earlier own window won over a later inherited one: %+v", coverage)
	}
	long := window(3*time.Hour, MaintenanceScopeResource, "guest long")
	if coverage, _ := ActiveMaintenanceCoverageAt(now, "guest", &long, []string{"node"}, read); coverage.Inherited || coverage.SourceID != "guest" || coverage.Reason != "guest long" {
		t.Fatalf("later own window lost to the inherited one: %+v", coverage)
	}

	// Controls: no ancestors or no reader means own state only.
	if _, ok := ActiveMaintenanceCoverageAt(now, "guest", nil, nil, read); ok {
		t.Fatal("covered with no own window and no ancestors")
	}
	if _, ok := ActiveMaintenanceCoverageAt(now, "guest", nil, []string{"node"}, nil); ok {
		t.Fatal("covered with no ancestor reader")
	}
	expired := window(-time.Minute, MaintenanceScopeResourceAndDescendants, "")
	states["node"] = expired
	if _, ok := ActiveMaintenanceCoverageAt(now, "guest", nil, []string{"node"}, read); ok {
		t.Fatal("an ancestor window that already ended still covers the guest")
	}
}
