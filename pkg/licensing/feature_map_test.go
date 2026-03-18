package licensing

import "testing"

type mockFeatureChecker struct {
	enabled map[string]bool
}

func (m mockFeatureChecker) RequireFeature(feature string) error {
	if m.enabled[feature] {
		return nil
	}
	return errFeatureMissing(feature)
}

func (m mockFeatureChecker) HasFeature(feature string) bool {
	return m.enabled[feature]
}

type errFeatureMissing string

func (e errFeatureMissing) Error() string {
	return "feature missing: " + string(e)
}

func TestBuildFeatureMap_DefaultKeys(t *testing.T) {
	checker := mockFeatureChecker{
		enabled: map[string]bool{
			FeatureAIAutoFix: true,
			FeatureRBAC:      true,
		},
	}

	got := BuildFeatureMap(checker, nil)
	if len(got) != len(DefaultFeatureKeys) {
		t.Fatalf("len(got)=%d want %d", len(got), len(DefaultFeatureKeys))
	}
	if !got[FeatureAIAutoFix] {
		t.Fatalf("expected %s=true", FeatureAIAutoFix)
	}
	if !got[FeatureRBAC] {
		t.Fatalf("expected %s=true", FeatureRBAC)
	}
	if got[FeatureAuditLogging] {
		t.Fatalf("expected %s=false", FeatureAuditLogging)
	}
}

func TestBuildFeatureMap_CustomKeys(t *testing.T) {
	checker := mockFeatureChecker{
		enabled: map[string]bool{FeatureRelay: true},
	}

	got := BuildFeatureMap(checker, []string{FeatureRelay, FeatureSSO})
	if len(got) != 2 {
		t.Fatalf("len(got)=%d want 2", len(got))
	}
	if !got[FeatureRelay] {
		t.Fatalf("expected %s=true", FeatureRelay)
	}
	if got[FeatureSSO] {
		t.Fatalf("expected %s=false", FeatureSSO)
	}
}

func TestBuildFeatureMap_NilChecker(t *testing.T) {
	got := BuildFeatureMap(nil, nil)
	if len(got) != 0 {
		t.Fatalf("len(got)=%d want 0", len(got))
	}
}
