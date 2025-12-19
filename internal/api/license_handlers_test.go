package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
)

type licenseFeaturesResponse struct {
	LicenseStatus string          `json:"license_status"`
	Features      map[string]bool `json:"features"`
	UpgradeURL    string          `json:"upgrade_url"`
}

func TestHandleLicenseFeatures_MethodNotAllowed(t *testing.T) {
	handler := NewLicenseHandlers(t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/license/features", nil)
	rec := httptest.NewRecorder()

	handler.HandleLicenseFeatures(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleLicenseFeatures_NoLicense(t *testing.T) {
	handler := NewLicenseHandlers(t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/api/license/features", nil)
	rec := httptest.NewRecorder()

	handler.HandleLicenseFeatures(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp licenseFeaturesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.LicenseStatus != string(license.LicenseStateNone) {
		t.Fatalf("expected license_status %q, got %q", license.LicenseStateNone, resp.LicenseStatus)
	}
	if resp.UpgradeURL == "" {
		t.Fatalf("expected upgrade_url to be set")
	}

	expectedFeatures := []string{
		license.FeatureAIPatrol,
		license.FeatureAIAlerts,
		license.FeatureAIAutoFix,
		license.FeatureKubernetesAI,
	}
	for _, feature := range expectedFeatures {
		if value, ok := resp.Features[feature]; !ok {
			t.Fatalf("expected feature %q in response", feature)
		} else if value {
			t.Fatalf("expected feature %q to be false without a license", feature)
		}
	}
}

func TestHandleLicenseFeatures_WithActiveLicense(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	handler := NewLicenseHandlers(t.TempDir())
	licenseKey, err := license.GenerateLicenseForTesting("test@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}
	if _, err := handler.Service().Activate(licenseKey); err != nil {
		t.Fatalf("failed to activate test license: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/license/features", nil)
	rec := httptest.NewRecorder()

	handler.HandleLicenseFeatures(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp licenseFeaturesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.LicenseStatus != string(license.LicenseStateActive) {
		t.Fatalf("expected license_status %q, got %q", license.LicenseStateActive, resp.LicenseStatus)
	}

	expectedFeatures := []string{
		license.FeatureAIPatrol,
		license.FeatureAIAlerts,
		license.FeatureAIAutoFix,
		license.FeatureKubernetesAI,
	}
	for _, feature := range expectedFeatures {
		if value, ok := resp.Features[feature]; !ok {
			t.Fatalf("expected feature %q in response", feature)
		} else if !value {
			t.Fatalf("expected feature %q to be true with active license", feature)
		}
	}
}
