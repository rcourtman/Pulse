package unifiedresources

import "testing"

func TestExportSensitivityFloor(t *testing.T) {
	if got := ExportSensitivityFloor(nil); got != SensitivityPublic {
		t.Fatalf("ExportSensitivityFloor(nil) = %q, want %q", got, SensitivityPublic)
	}

	counts := map[ResourceSensitivity]int{
		ResourceSensitivityPublic:     2,
		ResourceSensitivityInternal:   1,
		ResourceSensitivitySensitive:  3,
		ResourceSensitivityRestricted: 4,
	}
	if got := ExportSensitivityFloor(counts); got != SensitivityRestricted {
		t.Fatalf("ExportSensitivityFloor(restricted) = %q, want %q", got, SensitivityRestricted)
	}

	delete(counts, ResourceSensitivityRestricted)
	if got := ExportSensitivityFloor(counts); got != SensitivitySensitive {
		t.Fatalf("ExportSensitivityFloor(sensitive) = %q, want %q", got, SensitivitySensitive)
	}

	delete(counts, ResourceSensitivitySensitive)
	if got := ExportSensitivityFloor(counts); got != SensitivityInternal {
		t.Fatalf("ExportSensitivityFloor(internal) = %q, want %q", got, SensitivityInternal)
	}

	delete(counts, ResourceSensitivityInternal)
	if got := ExportSensitivityFloor(counts); got != SensitivityPublic {
		t.Fatalf("ExportSensitivityFloor(public) = %q, want %q", got, SensitivityPublic)
	}
}

func TestExportDecisionForContext(t *testing.T) {
	if got, reason := ExportDecisionForContext(SensitivityPublic, 0, 0); got != ExportAllowed || reason != "public unified resource context" {
		t.Fatalf("public decision = (%q, %q), want (%q, %q)", got, reason, ExportAllowed, "public unified resource context")
	}
	if got, reason := ExportDecisionForContext(SensitivityInternal, 0, 0); got != ExportRequiresConsent || reason != "internal unified resource context requires export consent" {
		t.Fatalf("internal decision = (%q, %q), want (%q, %q)", got, reason, ExportRequiresConsent, "internal unified resource context requires export consent")
	}
	if got, reason := ExportDecisionForContext(SensitivitySensitive, 0, 0); got != ExportRedacted || reason != "governed unified resource context exported in redacted form" {
		t.Fatalf("sensitive decision = (%q, %q), want (%q, %q)", got, reason, ExportRedacted, "governed unified resource context exported in redacted form")
	}
	if got, reason := ExportDecisionForContext(SensitivityPublic, 1, 0); got != ExportRedacted || reason != "governed unified resource context exported in redacted form" {
		t.Fatalf("local-only decision = (%q, %q), want (%q, %q)", got, reason, ExportRedacted, "governed unified resource context exported in redacted form")
	}
	if got, reason := ExportDecisionForContext(SensitivityPublic, 0, 1); got != ExportRedacted || reason != "governed unified resource context exported in redacted form" {
		t.Fatalf("redaction decision = (%q, %q), want (%q, %q)", got, reason, ExportRedacted, "governed unified resource context exported in redacted form")
	}
}
