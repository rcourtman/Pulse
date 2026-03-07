package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
)

func createTestHandler(t *testing.T) *LicenseHandlers {
	handler, _ := createTestHandlerWithDir(t)
	return handler
}

func createTestHandlerWithDir(t *testing.T) (*LicenseHandlers, string) {
	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	// Ensure default persistence exists
	_, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("Failed to initialize default persistence: %v", err)
	}
	return NewLicenseHandlers(mtp), tempDir
}

func makeLicenseKeyForClaims(t *testing.T, claims license.Claims) string {
	t.Helper()

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("failed to marshal test claims: %v", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return header + "." + payload + ".fake-sig"
}

func TestLicenseHandlers_FallbackToLegacyPersistence(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	handler := NewLicenseHandlers(nil)
	handler.SetLegacyPersistence(persistence)

	svc, p, err := handler.getTenantComponents(context.Background())
	if err != nil {
		t.Fatalf("expected legacy persistence fallback, got error: %v", err)
	}
	if svc == nil || p == nil {
		t.Fatalf("expected service and persistence from legacy fallback")
	}
}

type licenseFeaturesResponse struct {
	LicenseStatus string          `json:"license_status"`
	Features      map[string]bool `json:"features"`
	UpgradeURL    string          `json:"upgrade_url"`
}

func TestHandleLicenseFeatures_MethodNotAllowed(t *testing.T) {
	handler := createTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/license/features", nil)
	rec := httptest.NewRecorder()

	handler.HandleLicenseFeatures(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleLicenseFeatures_NoLicense(t *testing.T) {
	handler := createTestHandler(t)

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

	// Patrol is in free tier, so it should be true even without a license
	freeTierFeatures := []string{
		license.FeatureAIPatrol,
	}
	for _, feature := range freeTierFeatures {
		if value, ok := resp.Features[feature]; !ok {
			t.Fatalf("expected feature %q in response", feature)
		} else if !value {
			t.Fatalf("expected feature %q to be true in free tier", feature)
		}
	}

	// These Pro features should be false without a license
	proOnlyFeatures := []string{
		license.FeatureAIAlerts,
		license.FeatureAIAutoFix,
		license.FeatureKubernetesAI,
	}
	for _, feature := range proOnlyFeatures {
		if value, ok := resp.Features[feature]; !ok {
			t.Fatalf("expected feature %q in response", feature)
		} else if value {
			t.Fatalf("expected feature %q to be false without a license", feature)
		}
	}
}

func TestHandleLicenseFeatures_WithActiveLicense(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	handler := createTestHandler(t)
	licenseKey, err := license.GenerateLicenseForTesting("test@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}
	if _, err := handler.Service(context.Background()).Activate(licenseKey); err != nil {
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

func TestHandleLicenseFeatures_CorruptPersistedLicense(t *testing.T) {
	handler, tempDir := createTestHandlerWithDir(t)

	licensePath := filepath.Join(tempDir, license.LicenseFileName)
	if err := os.WriteFile(licensePath, []byte("%%%not-base64%%%"), 0600); err != nil {
		t.Fatalf("failed to write corrupt persisted license: %v", err)
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

	if resp.LicenseStatus != string(license.LicenseStateCorrupt) {
		t.Fatalf("expected license_status %q, got %q", license.LicenseStateCorrupt, resp.LicenseStatus)
	}
	if resp.Features[license.FeatureAIPatrol] != true {
		t.Fatalf("expected free-tier feature %q to remain enabled", license.FeatureAIPatrol)
	}
	if resp.Features[license.FeatureAIAutoFix] {
		t.Fatalf("expected Pro-only feature %q to be disabled", license.FeatureAIAutoFix)
	}
}

// ========================================
// HandleLicenseStatus tests
// ========================================

func TestHandleLicenseStatus_MethodNotAllowed(t *testing.T) {
	handler := createTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/license/status", nil)
	rec := httptest.NewRecorder()

	handler.HandleLicenseStatus(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleLicenseStatus_NoLicense(t *testing.T) {
	handler := createTestHandler(t)

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
	if resp.State != string(license.LicenseStateNone) {
		t.Fatalf("expected state %q, got %q", license.LicenseStateNone, resp.State)
	}
	if resp.Tier != license.TierFree {
		t.Fatalf("expected tier %q, got %q", license.TierFree, resp.Tier)
	}
}

func TestHandleLicenseStatus_WithActiveLicense(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	handler := createTestHandler(t)
	licenseKey, err := license.GenerateLicenseForTesting("test@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}
	if _, err := handler.Service(context.Background()).Activate(licenseKey); err != nil {
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
	if resp.State != string(license.LicenseStateActive) {
		t.Fatalf("expected state %q, got %q", license.LicenseStateActive, resp.State)
	}
	if resp.Email != "test@example.com" {
		t.Fatalf("expected email %q, got %q", "test@example.com", resp.Email)
	}
	if resp.Tier != license.TierPro {
		t.Fatalf("expected tier %q, got %q", license.TierPro, resp.Tier)
	}
}

func TestHandleLicenseStatus_CorruptPersistedLicense(t *testing.T) {
	handler, tempDir := createTestHandlerWithDir(t)

	licensePath := filepath.Join(tempDir, license.LicenseFileName)
	if err := os.WriteFile(licensePath, []byte("%%%not-base64%%%"), 0600); err != nil {
		t.Fatalf("failed to write corrupt persisted license: %v", err)
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

	if resp.Valid {
		t.Fatalf("expected Valid=false for corrupt persisted license")
	}
	if resp.State != string(license.LicenseStateCorrupt) {
		t.Fatalf("expected state %q, got %q", license.LicenseStateCorrupt, resp.State)
	}
	if resp.LoadError == "" {
		t.Fatalf("expected load_error to be set for corrupt persisted license")
	}
	if resp.Tier != license.TierFree {
		t.Fatalf("expected tier %q, got %q", license.TierFree, resp.Tier)
	}
}

func TestHandleLicenseStatus_ExpiredPersistedLicense(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	handler := createTestHandler(t)
	persistence, err := handler.getPersistenceForOrg("default")
	if err != nil {
		t.Fatalf("failed to get persistence: %v", err)
	}

	expiredKey := makeLicenseKeyForClaims(t, license.Claims{
		LicenseID: "test-expired-persisted",
		Email:     "expired@example.com",
		Tier:      license.TierPro,
		IssuedAt:  time.Now().Add(-40 * 24 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(-10 * 24 * time.Hour).Unix(),
	})
	if err := persistence.SaveWithGracePeriod(expiredKey, nil); err != nil {
		t.Fatalf("failed to persist expired license: %v", err)
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

	if resp.Valid {
		t.Fatalf("expected Valid=false for expired persisted license")
	}
	if resp.State != string(license.LicenseStateExpired) {
		t.Fatalf("expected state %q, got %q", license.LicenseStateExpired, resp.State)
	}
	if resp.Email != "expired@example.com" {
		t.Fatalf("expected email %q, got %q", "expired@example.com", resp.Email)
	}
	if resp.Tier != license.TierPro {
		t.Fatalf("expected tier %q, got %q", license.TierPro, resp.Tier)
	}
	if resp.ExpiresAt == nil {
		t.Fatalf("expected expires_at to be reported")
	}
	if resp.LoadError != "" {
		t.Fatalf("expected load_error to be empty, got %q", resp.LoadError)
	}
}

// ========================================
// HandleActivateLicense tests
// ========================================

func TestHandleActivateLicense_MethodNotAllowed(t *testing.T) {
	handler := createTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/license/activate", nil)
	rec := httptest.NewRecorder()

	handler.HandleActivateLicense(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleActivateLicense_EmptyKey(t *testing.T) {
	handler := createTestHandler(t)

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
	handler := createTestHandler(t)

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
	handler := createTestHandler(t)

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

	handler := createTestHandler(t)
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

func TestHandleActivateLicense_PersistenceUnavailableClearsRuntimeState(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	handler := NewLicenseHandlers(nil)
	licenseKey, err := license.GenerateLicenseForTesting("pro@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"license_key": licenseKey})
	req := httptest.NewRequest(http.MethodPost, "/api/license/activate", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleActivateLicense(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}

	var resp ActivateLicenseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Success {
		t.Fatalf("expected Success=false when persistence fails")
	}
	if resp.Message != "License could not be persisted" {
		t.Fatalf("expected message %q, got %q", "License could not be persisted", resp.Message)
	}
	if handler.Service(context.Background()).Current() != nil {
		t.Fatalf("expected runtime license to be cleared after persistence failure")
	}
}

// ========================================
// HandleClearLicense tests
// ========================================

func TestHandleClearLicense_MethodNotAllowed(t *testing.T) {
	handler := createTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/license/clear", nil)
	rec := httptest.NewRecorder()

	handler.HandleClearLicense(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleClearLicense_NoLicense(t *testing.T) {
	handler := createTestHandler(t)

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

	handler := createTestHandler(t)
	licenseKey, err := license.GenerateLicenseForTesting("test@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}
	if _, err := handler.Service(context.Background()).Activate(licenseKey); err != nil {
		t.Fatalf("failed to activate test license: %v", err)
	}

	// Verify license is active
	if !handler.Service(context.Background()).IsValid() {
		t.Fatalf("expected license to be valid before clearing")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/license/clear", nil)
	rec := httptest.NewRecorder()

	handler.HandleClearLicense(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Verify license is cleared
	if handler.Service(context.Background()).IsValid() {
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
	handler := createTestHandler(t)

	handlerCalled := false
	// Use a Pro-only feature (ai_autofix) to test that middleware blocks without license
	wrappedHandler := RequireLicenseFeature(handler, license.FeatureAIAutoFix, func(w http.ResponseWriter, r *http.Request) {
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

	handler := createTestHandler(t)
	licenseKey, err := license.GenerateLicenseForTesting("test@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}
	if _, err := handler.Service(context.Background()).Activate(licenseKey); err != nil {
		t.Fatalf("failed to activate test license: %v", err)
	}

	handlerCalled := false
	// Use a Pro-only feature (ai_autofix) to test that middleware passes with license
	wrappedHandler := RequireLicenseFeature(handler, license.FeatureAIAutoFix, func(w http.ResponseWriter, r *http.Request) {
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
	handler := createTestHandler(t)

	handlerCalled := false
	// Use a Pro-only feature (ai_autofix) to test gating without license
	wrappedHandler := LicenseGatedEmptyResponse(handler, license.FeatureAIAutoFix, func(w http.ResponseWriter, r *http.Request) {
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

	handler := createTestHandler(t)
	licenseKey, err := license.GenerateLicenseForTesting("test@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}
	if _, err := handler.Service(context.Background()).Activate(licenseKey); err != nil {
		t.Fatalf("failed to activate test license: %v", err)
	}

	handlerCalled := false
	// Use a Pro-only feature (ai_autofix) to test gating with license
	wrappedHandler := LicenseGatedEmptyResponse(handler, license.FeatureAIAutoFix, func(w http.ResponseWriter, r *http.Request) {
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
