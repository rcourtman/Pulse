package api

import (
	"bytes"
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

// ========================================
// HandleLicenseStatus tests
// ========================================

func TestHandleLicenseStatus_MethodNotAllowed(t *testing.T) {
	handler := NewLicenseHandlers(t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/license/status", nil)
	rec := httptest.NewRecorder()

	handler.HandleLicenseStatus(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleLicenseStatus_NoLicense(t *testing.T) {
	handler := NewLicenseHandlers(t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/api/license/status", nil)
	rec := httptest.NewRecorder()

	handler.HandleLicenseStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp license.LicenseStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// LicenseStatus uses Valid field and Tier field (no State field)
	// For no license, Valid should be false and Tier should be TierFree
	if resp.Valid {
		t.Fatalf("expected Valid=false for no license")
	}
	if resp.Tier != license.TierFree {
		t.Fatalf("expected tier %q, got %q", license.TierFree, resp.Tier)
	}
}

func TestHandleLicenseStatus_WithActiveLicense(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	handler := NewLicenseHandlers(t.TempDir())
	licenseKey, err := license.GenerateLicenseForTesting("test@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}
	if _, err := handler.Service().Activate(licenseKey); err != nil {
		t.Fatalf("failed to activate test license: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/license/status", nil)
	rec := httptest.NewRecorder()

	handler.HandleLicenseStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp license.LicenseStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// LicenseStatus uses Valid field and Tier field
	// For active license, Valid should be true and Tier should be TierPro
	if !resp.Valid {
		t.Fatalf("expected Valid=true for active license")
	}
	if resp.Email != "test@example.com" {
		t.Fatalf("expected email %q, got %q", "test@example.com", resp.Email)
	}
	if resp.Tier != license.TierPro {
		t.Fatalf("expected tier %q, got %q", license.TierPro, resp.Tier)
	}
}

// ========================================
// HandleActivateLicense tests
// ========================================

func TestHandleActivateLicense_MethodNotAllowed(t *testing.T) {
	handler := NewLicenseHandlers(t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/api/license/activate", nil)
	rec := httptest.NewRecorder()

	handler.HandleActivateLicense(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleActivateLicense_EmptyKey(t *testing.T) {
	handler := NewLicenseHandlers(t.TempDir())

	body := []byte(`{"license_key":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/license/activate", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleActivateLicense(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ActivateLicenseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Success {
		t.Fatalf("expected Success=false for empty key")
	}
	if resp.Message != "License key is required" {
		t.Fatalf("expected message %q, got %q", "License key is required", resp.Message)
	}
}

func TestHandleActivateLicense_InvalidKey(t *testing.T) {
	handler := NewLicenseHandlers(t.TempDir())

	body := []byte(`{"license_key":"invalid-license-key"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/license/activate", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleActivateLicense(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ActivateLicenseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Success {
		t.Fatalf("expected Success=false for invalid key")
	}
}

func TestHandleActivateLicense_InvalidBody(t *testing.T) {
	handler := NewLicenseHandlers(t.TempDir())

	body := []byte(`{invalid json}`)
	req := httptest.NewRequest(http.MethodPost, "/api/license/activate", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleActivateLicense(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ActivateLicenseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Success {
		t.Fatalf("expected Success=false for invalid body")
	}
	if resp.Message != "Invalid request body" {
		t.Fatalf("expected message %q, got %q", "Invalid request body", resp.Message)
	}
}

func TestHandleActivateLicense_ValidKey(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	handler := NewLicenseHandlers(t.TempDir())
	licenseKey, err := license.GenerateLicenseForTesting("pro@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"license_key": licenseKey})
	req := httptest.NewRequest(http.MethodPost, "/api/license/activate", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleActivateLicense(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp ActivateLicenseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected Success=true, got message: %s", resp.Message)
	}
	if resp.Status == nil {
		t.Fatalf("expected Status to be non-nil")
	}
	if resp.Status.Email != "pro@example.com" {
		t.Fatalf("expected email %q, got %q", "pro@example.com", resp.Status.Email)
	}
}

// ========================================
// HandleClearLicense tests
// ========================================

func TestHandleClearLicense_MethodNotAllowed(t *testing.T) {
	handler := NewLicenseHandlers(t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/api/license/clear", nil)
	rec := httptest.NewRecorder()

	handler.HandleClearLicense(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleClearLicense_NoLicense(t *testing.T) {
	handler := NewLicenseHandlers(t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/license/clear", nil)
	rec := httptest.NewRecorder()

	handler.HandleClearLicense(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if success, ok := resp["success"].(bool); !ok || !success {
		t.Fatalf("expected success=true")
	}
}

func TestHandleClearLicense_WithActiveLicense(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	handler := NewLicenseHandlers(t.TempDir())
	licenseKey, err := license.GenerateLicenseForTesting("test@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}
	if _, err := handler.Service().Activate(licenseKey); err != nil {
		t.Fatalf("failed to activate test license: %v", err)
	}

	// Verify license is active
	if !handler.Service().IsValid() {
		t.Fatalf("expected license to be valid before clearing")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/license/clear", nil)
	rec := httptest.NewRecorder()

	handler.HandleClearLicense(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Verify license is cleared
	if handler.Service().IsValid() {
		t.Fatalf("expected license to be invalid after clearing")
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if success, ok := resp["success"].(bool); !ok || !success {
		t.Fatalf("expected success=true")
	}
}

// ========================================
// RequireLicenseFeature middleware tests
// ========================================

func TestRequireLicenseFeature_NoLicense(t *testing.T) {
	handler := NewLicenseHandlers(t.TempDir())

	handlerCalled := false
	wrappedHandler := RequireLicenseFeature(handler.Service(), license.FeatureAIPatrol, func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected status %d, got %d", http.StatusPaymentRequired, rec.Code)
	}
	if handlerCalled {
		t.Fatalf("expected handler not to be called when license is missing")
	}
}

func TestRequireLicenseFeature_WithLicense(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	handler := NewLicenseHandlers(t.TempDir())
	licenseKey, err := license.GenerateLicenseForTesting("test@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}
	if _, err := handler.Service().Activate(licenseKey); err != nil {
		t.Fatalf("failed to activate test license: %v", err)
	}

	handlerCalled := false
	wrappedHandler := RequireLicenseFeature(handler.Service(), license.FeatureAIPatrol, func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !handlerCalled {
		t.Fatalf("expected handler to be called when license is valid")
	}
}

// ========================================
// LicenseGatedEmptyResponse middleware tests
// ========================================

func TestLicenseGatedEmptyResponse_NoLicense(t *testing.T) {
	handler := NewLicenseHandlers(t.TempDir())

	handlerCalled := false
	wrappedHandler := LicenseGatedEmptyResponse(handler.Service(), license.FeatureAIPatrol, func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"real"}`))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if handlerCalled {
		t.Fatalf("expected handler not to be called when license is missing")
	}
	// LicenseGatedEmptyResponse returns empty array, not empty object
	if rec.Body.String() != "[]" {
		t.Fatalf("expected empty array [], got %s", rec.Body.String())
	}
}

func TestLicenseGatedEmptyResponse_WithLicense(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	handler := NewLicenseHandlers(t.TempDir())
	licenseKey, err := license.GenerateLicenseForTesting("test@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}
	if _, err := handler.Service().Activate(licenseKey); err != nil {
		t.Fatalf("failed to activate test license: %v", err)
	}

	handlerCalled := false
	wrappedHandler := LicenseGatedEmptyResponse(handler.Service(), license.FeatureAIPatrol, func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"real"}`))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !handlerCalled {
		t.Fatalf("expected handler to be called when license is valid")
	}
	if rec.Body.String() != `{"data":"real"}` {
		t.Fatalf("expected real data, got %s", rec.Body.String())
	}
}
