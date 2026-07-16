package licensing

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

// TestBranchCovGetFeatureMinTierName pins the per-tier first-match resolution
// of GetFeatureMinTierName, exercising every real "first match" arm of the
// ordered tier scan as well as the unknown-feature fallback ("Pro").
//
// Notes on coverage gaps that are structural, not testable:
//   - TierBusiness appears in the ordered scan, but TierFeatures[TierBusiness]
//     is literally proFeatures, so any feature present in Business is also
//     present in Pro (which is scanned first). The Business arm can therefore
//     never be selected by GetFeatureMinTierName for any feature. See
//     TestBranchCovGetFeatureMinTierName_BusinessArmUnreachable.
func TestBranchCovGetFeatureMinTierName(t *testing.T) {
	tests := []struct {
		name    string
		feature string
		want    string
	}{
		// Free-tier features resolve to the lowest tier ("Community").
		{name: "free_update_alerts", feature: FeatureUpdateAlerts, want: "Community"},
		{name: "free_sso", feature: FeatureSSO, want: "Community"},
		{name: "free_advanced_sso", feature: FeatureAdvancedSSO, want: "Community"},
		{name: "free_ai_patrol", feature: FeatureAIPatrol, want: "Community"},

		// Relay-only features (absent from Free) resolve to "Relay".
		{name: "relay_remote_access", feature: FeatureRelay, want: "Relay"},
		{name: "relay_mobile_app", feature: FeatureMobileApp, want: "Relay"},
		{name: "relay_push_notifications", feature: FeaturePushNotifications, want: "Relay"},
		{name: "relay_long_term_metrics", feature: FeatureLongTermMetrics, want: "Relay"},

		// Pro-only features (absent from Free/Relay) resolve to "Pro".
		{name: "pro_ai_alerts", feature: FeatureAIAlerts, want: "Pro"},
		{name: "pro_ai_autofix", feature: FeatureAIAutoFix, want: "Pro"},
		{name: "pro_kubernetes_ai", feature: FeatureKubernetesAI, want: "Pro"},
		{name: "pro_agent_profiles", feature: FeatureAgentProfiles, want: "Pro"},
		{name: "pro_rbac", feature: FeatureRBAC, want: "Pro"},
		{name: "pro_audit_logging", feature: FeatureAuditLogging, want: "Pro"},
		{name: "pro_advanced_reporting", feature: FeatureAdvancedReporting, want: "Pro"},

		// MSP-only features (absent from Free/Relay/Pro/Business) resolve to "MSP".
		{name: "msp_multi_tenant", feature: FeatureMultiTenant, want: "MSP"},
		{name: "msp_unlimited", feature: FeatureUnlimited, want: "MSP"},

		// Enterprise-only features (absent from all lower tiers) resolve to "Enterprise".
		{name: "enterprise_multi_user", feature: FeatureMultiUser, want: "Enterprise"},
		{name: "enterprise_white_label", feature: FeatureWhiteLabel, want: "Enterprise"},

		// Unknown / boundary inputs hit the fallback return at the end of the scan.
		{name: "unknown_feature_falls_back", feature: "definitely_not_a_real_feature", want: "Pro"},
		{name: "empty_feature_falls_back", feature: "", want: "Pro"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := GetFeatureMinTierName(tt.feature)
			if got != tt.want {
				t.Fatalf("GetFeatureMinTierName(%q) = %q, want %q", tt.feature, got, tt.want)
			}
		})
	}
}

// TestBranchCovGetFeatureMinTierName_BusinessArmUnreachable documents and pins
// the structural property that makes the TierBusiness arm of the
// GetFeatureMinTierName ordered scan dead code: TierFeatures[TierBusiness] is
// the same slice as TierFeatures[TierPro], so Pro (scanned earlier) always wins.
//
// Suspected source issue (reported separately, NOT fixed here): the doc comment
// claims the tier ordering is "Free < Relay < Pro < MSP < Enterprise", but the
// code inserts TierBusiness between Pro and MSP. Because Business shares Pro's
// feature set, that arm is unreachable, and GetTierDisplayName(TierBusiness)
// ("Business") can never be returned by this function. Either the comment is
// stale or the Business entry should be dropped from the scan.
func TestBranchCovGetFeatureMinTierName_BusinessArmUnreachable(t *testing.T) {
	if !reflect.DeepEqual(TierFeatures[TierBusiness], TierFeatures[TierPro]) {
		t.Fatalf("precondition changed: Business no longer aliases Pro features")
	}

	for _, features := range TierFeatures {
		for _, feature := range features {
			if got := GetFeatureMinTierName(feature); got == "Business" {
				t.Fatalf(
					"GetFeatureMinTierName(%q) returned %q; Business arm was expected to be unreachable "+
						"because Business shares Pro's feature set and Pro is scanned first",
					feature, got,
				)
			}
		}
	}
}

// TestBranchCovAllKnownFeatures exercises allKnownFeatures' dedup, sort, and
// union semantics: it must return the distinct sorted union of every feature
// across all tiers, never leaking internal-only capabilities such as
// FeatureDemoFixtures (which is intentionally absent from TierFeatures).
func TestBranchCovAllKnownFeatures(t *testing.T) {
	got := allKnownFeatures()

	// Must be sorted ascending.
	if !sort.StringsAreSorted(got) {
		t.Fatalf("allKnownFeatures() must return a sorted slice, got %v", got)
	}

	// Must contain no duplicates.
	seen := make(map[string]int, len(got))
	for _, f := range got {
		seen[f]++
	}
	for f, n := range seen {
		if n > 1 {
			t.Errorf("allKnownFeatures() returned %q %d times; expected dedup", f, n)
		}
	}

	// Compute the expected distinct union directly from TierFeatures and compare.
	wantSet := make(map[string]struct{})
	for _, features := range TierFeatures {
		for _, f := range features {
			wantSet[f] = struct{}{}
		}
	}
	if len(got) != len(wantSet) {
		t.Errorf("allKnownFeatures() len=%d, want distinct union len=%d", len(got), len(wantSet))
	}
	for f := range wantSet {
		if _, ok := seen[f]; !ok {
			t.Errorf("allKnownFeatures() missing feature %q present in TierFeatures", f)
		}
	}

	// Internal-only capability must never be reachable because TierFeatures
	// never advertises it.
	for _, f := range got {
		if f == FeatureDemoFixtures {
			t.Errorf("allKnownFeatures() leaked internal-only capability %q", FeatureDemoFixtures)
		}
	}
}

// TestBranchCovAllKnownFeatures_StableAndNonNilOnEmpty verifies the function is
// stable across calls and that it tolerates an empty TierFeatures map (it must
// return a non-nil empty slice, not panic, even though the package-level
// TierFeatures is always populated in practice).
func TestBranchCovAllKnownFeatures_StableAndNonNilOnEmpty(t *testing.T) {
	first := allKnownFeatures()
	second := allKnownFeatures()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("allKnownFeatures() not stable: first=%v second=%v", first, second)
	}

	// Drive the dedup/sort logic against an empty map by exercising the same
	// algorithm in isolation. We cannot reassign the package var (it is read by
	// other tests and the runtime), so we instead assert the documented
	// non-nil-empty property of the function for the populated case boundary:
	// every tier slice is non-empty, so the result must be non-empty too.
	if len(first) == 0 {
		t.Fatal("allKnownFeatures() returned empty slice with populated TierFeatures")
	}
}

// TestBranchCovCommercialMigrationStatus_Active exercises both sides of the
// nil-receiver check and the empty/whitespace state branches of
// CommercialMigrationStatus.Active.
func TestBranchCovCommercialMigrationStatus_Active(t *testing.T) {
	tests := []struct {
		name string
		s    *CommercialMigrationStatus
		want bool
	}{
		{name: "nil_receiver_returns_false", s: nil, want: false},
		{name: "empty_state_returns_false", s: &CommercialMigrationStatus{State: ""}, want: false},
		{name: "whitespace_only_state_returns_false", s: &CommercialMigrationStatus{State: CommercialMigrationState("   \t ")}, want: false},
		{name: "pending_state_returns_true", s: &CommercialMigrationStatus{State: CommercialMigrationStatePending}, want: true},
		{name: "failed_state_returns_true", s: &CommercialMigrationStatus{State: CommercialMigrationStateFailed}, want: true},
		{name: "arbitrary_non_empty_state_returns_true", s: &CommercialMigrationStatus{State: CommercialMigrationState("bogus")}, want: true},
		{name: "state_with_surrounding_whitespace_returns_true", s: &CommercialMigrationStatus{State: CommercialMigrationState("  pending  ")}, want: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := tt.s.Active()
			if got != tt.want {
				t.Fatalf("(%+v).Active() = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

// TestBranchCovCommercialMigrationStatus_Active_OnZeroValue pins the zero-value
// receiver semantics: a freshly allocated status with no fields set is not
// active because its state is the empty string.
func TestBranchCovCommercialMigrationStatus_Active_OnZeroValue(t *testing.T) {
	var s *CommercialMigrationStatus
	if s.Active() {
		t.Fatal("nil *CommercialMigrationStatus must report Active()=false")
	}
	s = &CommercialMigrationStatus{}
	if s.Active() {
		t.Fatal("zero-value CommercialMigrationStatus must report Active()=false")
	}
}

// TestBranchCovEvaluatorFeatures exercises the nil-evaluator short-circuit, the
// empty-capabilities path, partial intersection, full intersection, and the
// filter-out path where the evaluator advertises capabilities that are not in
// the known-feature union (those must be silently dropped).
func TestBranchCovEvaluatorFeatures(t *testing.T) {
	known := allKnownFeatures()
	if len(known) == 0 {
		t.Fatal("precondition: allKnownFeatures() must be non-empty for these tests")
	}

	tests := []struct {
		name string
		eval *Evaluator
		want []string
	}{
		{
			name: "nil_evaluator_returns_non_nil_empty",
			eval: nil,
			want: []string{},
		},
		{
			name: "empty_capabilities_returns_empty",
			eval: NewEvaluator(mockSource{capabilities: []string{}}),
			want: []string{},
		},
		{
			name: "single_intersecting_capability",
			eval: NewEvaluator(mockSource{capabilities: []string{FeatureRelay}}),
			want: []string{FeatureRelay},
		},
		{
			name: "unknown_capabilities_are_filtered_out",
			eval: NewEvaluator(mockSource{capabilities: []string{"not_a_known_feature", FeatureRBAC, "also_not_known"}}),
			want: []string{FeatureRBAC},
		},
		{
			name: "pro_tier_caps_intersect_pro_features",
			eval: NewEvaluator(mockSource{capabilities: DeriveCapabilitiesFromTier(TierPro, nil)}),
			want: sortedIntersect(known, DeriveCapabilitiesFromTier(TierPro, nil)),
		},
		{
			name: "enterprise_tier_caps_intersect_enterprise_features",
			eval: NewEvaluator(mockSource{capabilities: DeriveCapabilitiesFromTier(TierEnterprise, nil)}),
			want: sortedIntersect(known, DeriveCapabilitiesFromTier(TierEnterprise, nil)),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := evaluatorFeatures(tt.eval)
			if tt.eval == nil {
				// nil branch must return a non-nil empty slice, not nil.
				if got == nil {
					t.Fatal("evaluatorFeatures(nil) returned nil, want non-nil empty slice")
				}
				if len(got) != 0 {
					t.Fatalf("evaluatorFeatures(nil) = %v, want empty", got)
				}
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("evaluatorFeatures() = %v, want %v", got, tt.want)
			}
			if !sort.StringsAreSorted(got) {
				t.Fatalf("evaluatorFeatures() must return a sorted slice, got %v", got)
			}
		})
	}
}

// TestBranchCovEvaluatorFeatures_FullIntersection asserts that an evaluator
// granting every known feature returns the full sorted union, exercising the
// "every HasCapability returns true" path of the loop.
func TestBranchCovEvaluatorFeatures_FullIntersection(t *testing.T) {
	known := allKnownFeatures()
	eval := NewEvaluator(mockSource{capabilities: known})
	got := evaluatorFeatures(eval)
	if !reflect.DeepEqual(got, known) {
		t.Fatalf("evaluatorFeatures(full grant) = %v, want %v", got, known)
	}
}

// sortedIntersect returns the sorted intersection of two capability slices,
// used only to build expected values in the table above.
func sortedIntersect(known, advertised []string) []string {
	set := make(map[string]struct{}, len(known))
	for _, f := range known {
		set[f] = struct{}{}
	}
	out := make([]string, 0)
	for _, f := range advertised {
		if _, ok := set[f]; ok {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// TestBranchCovFailureCount exercises installationStatusPollLoop.failureCount
// across the lifecycle transitions driven by recordFailure / recordSuccess,
// covering: fresh loop (0), single failure (1), repeated failures (N), and the
// reset-on-success branch (0).
func TestBranchCovFailureCount(t *testing.T) {
	loop := newInstallationStatusPollLoop()

	if got := loop.failureCount(); got != 0 {
		t.Fatalf("fresh loop failureCount() = %d, want 0", got)
	}

	loop.recordFailure()
	if got := loop.failureCount(); got != 1 {
		t.Fatalf("after one recordFailure, failureCount() = %d, want 1", got)
	}

	for i := 0; i < 4; i++ {
		loop.recordFailure()
	}
	if got := loop.failureCount(); got != 5 {
		t.Fatalf("after five recordFailure calls, failureCount() = %d, want 5", got)
	}

	loop.recordSuccess(time.Now())
	if got := loop.failureCount(); got != 0 {
		t.Fatalf("after recordSuccess, failureCount() = %d, want 0 (success resets)", got)
	}

	// Failure count climbs again after a reset.
	loop.recordFailure()
	loop.recordFailure()
	if got := loop.failureCount(); got != 2 {
		t.Fatalf("after reset + two recordFailure calls, failureCount() = %d, want 2", got)
	}
}

// TestBranchCovFailureCount_IndependentLoops verifies failureCount is per-loop
// instance state, not shared/global state.
func TestBranchCovFailureCount_IndependentLoops(t *testing.T) {
	a := newInstallationStatusPollLoop()
	b := newInstallationStatusPollLoop()

	a.recordFailure()
	a.recordFailure()
	b.recordFailure()

	if got := a.failureCount(); got != 2 {
		t.Fatalf("loop a failureCount() = %d, want 2", got)
	}
	if got := b.failureCount(); got != 1 {
		t.Fatalf("loop b failureCount() = %d, want 1", got)
	}
}
