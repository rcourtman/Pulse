package conversion

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
)

func TestGenerateUpgradeReasons_EmptyCapabilitiesReturnsAll(t *testing.T) {
	reasons := GenerateUpgradeReasons(nil)
	if len(reasons) != len(UpgradeReasonMatrix) {
		t.Fatalf("expected %d reasons, got %d", len(UpgradeReasonMatrix), len(reasons))
	}
}

func TestGenerateUpgradeReasons_ProCapabilitiesReturnsEmpty(t *testing.T) {
	capabilities := append([]string(nil), license.TierFeatures[license.TierPro]...)
	reasons := GenerateUpgradeReasons(capabilities)
	if len(reasons) != 0 {
		t.Fatalf("expected 0 reasons for pro capabilities, got %d", len(reasons))
	}
}

func TestGenerateUpgradeReasons_SortedByPriority(t *testing.T) {
	reasons := GenerateUpgradeReasons(nil)
	for i := 1; i < len(reasons); i++ {
		if reasons[i-1].Priority > reasons[i].Priority {
			t.Fatalf(
				"expected reasons sorted by priority at index %d: %d > %d",
				i,
				reasons[i-1].Priority,
				reasons[i].Priority,
			)
		}
	}
}

func TestGenerateUpgradeReasons_EntriesHaveRequiredFields(t *testing.T) {
	reasons := GenerateUpgradeReasons(nil)
	for i, reason := range reasons {
		if reason.Feature == "" {
			t.Fatalf("entry %d has empty feature", i)
		}
		if reason.Reason == "" {
			t.Fatalf("entry %d has empty reason", i)
		}
		if reason.ActionURL == "" {
			t.Fatalf("entry %d has empty action_url", i)
		}
	}
}

func TestUpgradeReasonMatrix_CompletenessAgainstProMinusFree(t *testing.T) {
	expected := expectedProOnlyFeatureSet()
	seen := make(map[string]struct{}, len(UpgradeReasonMatrix))

	if len(UpgradeReasonMatrix) != len(expected) {
		t.Fatalf("expected %d matrix entries, got %d", len(expected), len(UpgradeReasonMatrix))
	}

	for _, entry := range UpgradeReasonMatrix {
		if _, exists := seen[entry.Feature]; exists {
			t.Fatalf("duplicate matrix feature entry: %s", entry.Feature)
		}
		seen[entry.Feature] = struct{}{}
		if _, ok := expected[entry.Feature]; !ok {
			t.Fatalf("unexpected matrix feature: %s", entry.Feature)
		}
	}

	for feature := range expected {
		if _, ok := seen[feature]; !ok {
			t.Fatalf("missing matrix feature: %s", feature)
		}
	}
}

func TestUpgradeURLForFeature(t *testing.T) {
	for _, entry := range UpgradeReasonMatrix {
		got := UpgradeURLForFeature(entry.Feature)
		if got != entry.ActionURL {
			t.Fatalf("expected action URL %q for feature %q, got %q", entry.ActionURL, entry.Feature, got)
		}
	}
}

func TestUpgradeURLForFeatureUnknownFallsBackToGenericPricing(t *testing.T) {
	const expected = "https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade"
	if got := UpgradeURLForFeature("nonexistent_feature"); got != expected {
		t.Fatalf("expected generic pricing URL %q, got %q", expected, got)
	}
}

func expectedProOnlyFeatureSet() map[string]struct{} {
	proFeatures := make(map[string]struct{}, len(license.TierFeatures[license.TierPro]))
	for _, feature := range license.TierFeatures[license.TierPro] {
		proFeatures[feature] = struct{}{}
	}
	for _, feature := range license.TierFeatures[license.TierFree] {
		delete(proFeatures, feature)
	}
	return proFeatures
}
