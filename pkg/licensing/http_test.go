package licensing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteLicenseRequired(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteLicenseRequired(rec, FeatureAIAutoFix, "missing feature", func(feature string) string {
		if feature == FeatureAIAutoFix {
			return "https://example.com/upgrade?feature=ai_autofix"
		}
		return ""
	})

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusPaymentRequired)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload["error"] != "license_required" {
		t.Fatalf("error=%v want license_required", payload["error"])
	}
	if payload["feature"] != FeatureAIAutoFix {
		t.Fatalf("feature=%v want %s", payload["feature"], FeatureAIAutoFix)
	}
	if payload["upgrade_url"] != "https://example.com/upgrade?feature=ai_autofix" {
		t.Fatalf("upgrade_url=%v", payload["upgrade_url"])
	}
}

func TestWriteLicenseRequired_NoResolver(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteLicenseRequired(rec, FeatureRBAC, "missing feature", nil)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusPaymentRequired)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload["upgrade_url"] != "" {
		t.Fatalf("upgrade_url=%v want empty string", payload["upgrade_url"])
	}
}

func TestUpgradeURLForFeature(t *testing.T) {
	if got := UpgradeURLForFeature(FeatureAIAutoFix); got == DefaultUpgradeURL {
		t.Fatalf("expected feature-specific URL for %s", FeatureAIAutoFix)
	}

	if got := UpgradeURLForFeature("unknown_feature"); got != DefaultUpgradeURL {
		t.Fatalf("unexpected URL for unknown feature: %q", got)
	}
}
