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

func TestAutoRemediationPolicySQLiteRoundTrip(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore: %v", err)
	}
	defer store.Close()

	state := ResourceOperatorState{
		CanonicalID: "docker:container:web",
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
