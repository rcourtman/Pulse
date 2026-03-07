package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func createTestHandler(t *testing.T) *LicenseHandlers {
	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	// Ensure default persistence exists
	_, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("Failed to initialize default persistence: %v", err)
	}
	return NewLicenseHandlers(mtp, false)
}

type licenseFeaturesResponseDTO struct {
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

	var resp licenseFeaturesResponseDTO
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

	var resp licenseFeaturesResponseDTO
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
	if resp.Email != "test@example.com" {
		t.Fatalf("expected email %q, got %q", "test@example.com", resp.Email)
	}
	if resp.Tier != license.TierPro {
		t.Fatalf("expected tier %q, got %q", license.TierPro, resp.Tier)
	}
}

func TestHandleLicenseStatus_ExpiredBillingBackedTrialFallsBackToFreeDisplay(t *testing.T) {
	handler := createTestHandler(t)

	now := time.Now()
	startedAt := now.Add(-15 * 24 * time.Hour).Unix()
	endedAt := now.Add(-2 * time.Hour).Unix()
	store := config.NewFileBillingStore(handler.mtPersistence.BaseDataDir())
	if err := store.SaveBillingState("default", &pkglicensing.BillingState{
		Capabilities:      append([]string(nil), license.TierFeatures[license.TierPro]...),
		Limits:            map[string]int64{"max_agents": 15},
		MetersEnabled:     []string{"agents"},
		PlanVersion:       "trial",
		SubscriptionState: entitlements.SubStateTrial,
		TrialStartedAt:    &startedAt,
		TrialEndsAt:       &endedAt,
	}); err != nil {
		t.Fatalf("SaveBillingState: %v", err)
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
		t.Fatalf("expected Valid=false for expired billing-backed trial")
	}
	if resp.Tier != license.TierFree {
		t.Fatalf("expected tier %q, got %q", license.TierFree, resp.Tier)
	}
	if resp.MaxAgents != license.TierAgentLimits[license.TierFree] {
		t.Fatalf("expected max_agents %d, got %d", license.TierAgentLimits[license.TierFree], resp.MaxAgents)
	}
	if resp.MaxGuests != 0 {
		t.Fatalf("expected max_guests 0, got %d", resp.MaxGuests)
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

func TestHandleActivateLicense_ExchangesLegacyJWTInStrictV6(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	grantJWT, grantPublicKey, err := pkglicensing.GenerateGrantJWTForTesting(pkglicensing.GrantClaims{
		LicenseID: "lic_exchanged",
		Tier:      "pro",
		State:     "active",
		Features:  []string{"relay"},
		MaxAgents: 10,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
		Email:     "legacy-jwt@example.com",
	})
	if err != nil {
		t.Fatalf("generate grant jwt: %v", err)
	}
	license.SetPublicKey(grantPublicKey)
	t.Cleanup(func() { license.SetPublicKey(nil) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/licenses/exchange" {
			t.Fatalf("path = %q, want /v1/licenses/exchange", r.URL.Path)
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"license": map[string]any{
				"license_id": "lic_exchanged",
				"state":      "active",
				"tier":       "pro",
				"features":   []string{"relay"},
				"max_agents": 10,
			},
			"installation": map[string]any{
				"installation_id":    "inst_exchanged",
				"installation_token": "pit_live_exchanged",
				"status":             "active",
			},
			"grant": map[string]any{
				"jwt":        grantJWT,
				"jti":        "grant_exchanged",
				"expires_at": time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()
	t.Setenv("PULSE_LICENSE_SERVER_URL", server.URL)

	handler := createTestHandler(t)
	licenseKey, err := license.GenerateLicenseForTesting("legacy-jwt@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"license_key": licenseKey})
	req := httptest.NewRequest(http.MethodPost, "/api/license/activate", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleActivateLicense(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp ActivateLicenseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected Success=true for legacy JWT exchange, got message %q", resp.Message)
	}
	if resp.Message != "Pulse v5 license migrated and activated successfully" {
		t.Fatalf("message=%q, want %q", resp.Message, "Pulse v5 license migrated and activated successfully")
	}
	if svc := handler.Service(context.Background()); svc == nil || !svc.IsActivated() {
		t.Fatalf("expected service activation state after exchange")
	}
	cp, err := handler.mtPersistence.GetPersistence("default")
	if err != nil {
		t.Fatalf("get default persistence: %v", err)
	}
	persistence, err := pkglicensing.NewPersistence(cp.GetConfigDir())
	if err != nil {
		t.Fatalf("new license persistence: %v", err)
	}
	legacyLeft, err := persistence.Load()
	if err != nil {
		t.Fatalf("load preserved legacy key: %v", err)
	}
	if legacyLeft != licenseKey {
		t.Fatalf("expected legacy key to be preserved for downgrade, got %q", legacyLeft)
	}
}

func TestHandleActivateLicense_ClearsCommercialMigrationStateOnNativeActivation(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	grantJWT, grantPublicKey, err := pkglicensing.GenerateGrantJWTForTesting(pkglicensing.GrantClaims{
		LicenseID: "lic_v6_native",
		Tier:      "pro",
		State:     "active",
		Features:  []string{"relay"},
		MaxAgents: 10,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
		Email:     "native-v6@example.com",
	})
	if err != nil {
		t.Fatalf("generate grant jwt: %v", err)
	}
	license.SetPublicKey(grantPublicKey)
	t.Cleanup(func() { license.SetPublicKey(nil) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/activate" {
			t.Fatalf("path = %q, want /v1/activate", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"license": map[string]any{
				"license_id": "lic_v6_native",
				"state":      "active",
				"tier":       "pro",
				"features":   []string{"relay"},
				"max_agents": 10,
			},
			"installation": map[string]any{
				"installation_id":    "inst_v6_native",
				"installation_token": "pit_live_native",
				"status":             "active",
			},
			"grant": map[string]any{
				"jwt":        grantJWT,
				"jti":        "grant_v6_native",
				"expires_at": time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()
	t.Setenv("PULSE_LICENSE_SERVER_URL", server.URL)

	handler := createTestHandler(t)
	store := config.NewFileBillingStore(handler.mtPersistence.BaseDataDir())
	if err := store.SaveBillingState("default", &pkglicensing.BillingState{
		Capabilities:      []string{},
		Limits:            map[string]int64{},
		MetersEnabled:     []string{},
		PlanVersion:       string(entitlements.SubStateExpired),
		SubscriptionState: entitlements.SubStateExpired,
		CommercialMigration: &pkglicensing.CommercialMigrationStatus{
			Source:            pkglicensing.CommercialMigrationSourceV5License,
			State:             pkglicensing.CommercialMigrationStatePending,
			Reason:            pkglicensing.CommercialMigrationReasonExchangeUnavailable,
			RecommendedAction: pkglicensing.CommercialMigrationActionRetryActivation,
		},
	}); err != nil {
		t.Fatalf("SaveBillingState: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"license_key": "ppk_live_native_activation"})
	req := httptest.NewRequest(http.MethodPost, "/api/license/activate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleActivateLicense(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	loaded, err := store.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected billing state to remain")
	}
	if loaded.CommercialMigration != nil {
		t.Fatalf("commercial_migration=%+v, want nil", loaded.CommercialMigration)
	}
}

func TestHandleActivateLicense_ActivationKeyClearsStaleLegacyPersistence(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	grantJWT, grantPublicKey, err := pkglicensing.GenerateGrantJWTForTesting(pkglicensing.GrantClaims{
		LicenseID: "lic_v6_native",
		Tier:      "pro",
		State:     "active",
		Features:  []string{"relay"},
		MaxAgents: 10,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
		Email:     "native-v6@example.com",
	})
	if err != nil {
		t.Fatalf("generate grant jwt: %v", err)
	}
	license.SetPublicKey(grantPublicKey)
	t.Cleanup(func() { license.SetPublicKey(nil) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/activate" {
			t.Fatalf("path = %q, want /v1/activate", r.URL.Path)
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"license": map[string]any{
				"license_id": "lic_v6_native",
				"state":      "active",
				"tier":       "pro",
				"features":   []string{"relay"},
				"max_agents": 10,
			},
			"installation": map[string]any{
				"installation_id":    "inst_v6_native",
				"installation_token": "pit_live_native",
				"status":             "active",
			},
			"grant": map[string]any{
				"jwt":        grantJWT,
				"jti":        "grant_v6_native",
				"expires_at": time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()
	t.Setenv("PULSE_LICENSE_SERVER_URL", server.URL)

	handler := createTestHandler(t)
	_ = handler.Service(context.Background())
	cp, err := handler.mtPersistence.GetPersistence("default")
	if err != nil {
		t.Fatalf("get default persistence: %v", err)
	}
	persistence, err := pkglicensing.NewPersistence(cp.GetConfigDir())
	if err != nil {
		t.Fatalf("new license persistence: %v", err)
	}

	legacyKey, err := license.GenerateLicenseForTesting("legacy-stale@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("generate stale legacy license: %v", err)
	}
	if err := persistence.Save(legacyKey); err != nil {
		t.Fatalf("save stale legacy license: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"license_key": "ppk_live_native_activation"})
	req := httptest.NewRequest(http.MethodPost, "/api/license/activate", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleActivateLicense(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if legacyLeft, err := persistence.Load(); err != nil {
		t.Fatalf("load legacy persistence after native activation: %v", err)
	} else if legacyLeft != "" {
		t.Fatalf("expected native v6 activation to clear stale legacy persistence, got %q", legacyLeft)
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

// ========================================
// userFriendlyActivationError tests
// ========================================

func TestUserFriendlyActivationError_NoGoErrorChainSyntax(t *testing.T) {
	// These are real errors that service.Activate() can produce.
	// None of the user-facing messages should contain Go error chain syntax,
	// JWT terminology, or internal system terms.
	cases := []struct {
		name  string
		err   error
		check func(t *testing.T, msg string)
	}{
		{
			name: "malformed license (header encoding)",
			err:  fmt.Errorf("validate license: %w: invalid header encoding", license.ErrMalformedLicense),
			check: func(t *testing.T, msg string) {
				if strings.Contains(msg, "header encoding") {
					t.Errorf("message leaks internal detail: %q", msg)
				}
				if strings.Contains(msg, "validate license:") {
					t.Errorf("message contains Go error chain syntax: %q", msg)
				}
			},
		},
		{
			name: "malformed license (empty jwt segment)",
			err:  fmt.Errorf("validate license: %w: empty jwt segment", license.ErrMalformedLicense),
			check: func(t *testing.T, msg string) {
				if strings.Contains(msg, "jwt") {
					t.Errorf("message leaks JWT terminology: %q", msg)
				}
			},
		},
		{
			name: "signature invalid",
			err:  fmt.Errorf("validate license: %w", license.ErrSignatureInvalid),
			check: func(t *testing.T, msg string) {
				if strings.Contains(msg, "signature") {
					t.Errorf("message leaks signature terminology: %q", msg)
				}
			},
		},
		{
			name: "expired license",
			err:  fmt.Errorf("validate license: %w: expired on 2025-01-01 (grace period ended 2025-01-08)", license.ErrExpiredLicense),
			check: func(t *testing.T, msg string) {
				if strings.Contains(msg, "grace period") {
					t.Errorf("message leaks grace period detail: %q", msg)
				}
			},
		},
		{
			name: "invalid license",
			err:  license.ErrInvalidLicense,
			check: func(t *testing.T, msg string) {
				if msg == license.ErrInvalidLicense.Error() {
					t.Errorf("message is raw sentinel error: %q", msg)
				}
			},
		},
		{
			name: "license server client not configured",
			err:  fmt.Errorf("activation unavailable: license server client not configured"),
			check: func(t *testing.T, msg string) {
				if strings.Contains(msg, "license server client") {
					t.Errorf("message leaks internal detail: %q", msg)
				}
			},
		},
		{
			name: "generate instance fingerprint",
			err:  fmt.Errorf("generate instance fingerprint: some system error"),
			check: func(t *testing.T, msg string) {
				if strings.Contains(msg, "fingerprint") {
					t.Errorf("message leaks fingerprint detail: %q", msg)
				}
			},
		},
		{
			name: "unsupported activation format",
			err:  fmt.Errorf("license key is not a supported v6 activation key or migratable v5 license"),
			check: func(t *testing.T, msg string) {
				if strings.Contains(msg, "JWT") {
					t.Errorf("message leaks JWT terminology: %q", msg)
				}
				if !strings.Contains(strings.ToLower(msg), "v6 activation key") {
					t.Errorf("message should mention v6 activation keys: %q", msg)
				}
				if !strings.Contains(strings.ToLower(msg), "v5 license") {
					t.Errorf("message should mention v5 migration support: %q", msg)
				}
				if !strings.Contains(strings.ToLower(msg), "exchange it automatically") {
					t.Errorf("message should mention automatic exchange: %q", msg)
				}
			},
		},
	}

	// Forbidden patterns that must never appear in any user-facing error.
	forbidden := []string{"jwt segment", "header encoding", "signature encoding",
		"fingerprint", "license server client", "validate license:", "parse grant"}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := userFriendlyActivationError(tc.err)
			tc.check(t, msg)

			// No message should contain nested colon-delimited chains (X: Y: Z).
			colonParts := strings.Split(msg, ": ")
			if len(colonParts) > 2 {
				t.Errorf("message contains Go error chain syntax (nested colons): %q", msg)
			}

			for _, pattern := range forbidden {
				if strings.Contains(strings.ToLower(msg), pattern) {
					t.Errorf("message contains forbidden pattern %q: %q", pattern, msg)
				}
			}
		})
	}
}

func TestUserFriendlyActivationError_ServerError(t *testing.T) {
	retryableErr := fmt.Errorf("activation failed: %w", &licenseServerErrorModel{
		StatusCode: 503,
		Code:       "service_unavailable",
		Message:    "Service temporarily unavailable",
		Retryable:  true,
	})
	msg := userFriendlyActivationError(retryableErr)
	if !strings.Contains(msg, "try again") {
		t.Errorf("retryable server error should suggest retry: %q", msg)
	}

	nonRetryableErr := fmt.Errorf("activation failed: %w", &licenseServerErrorModel{
		StatusCode: 422,
		Code:       "key_already_used",
		Message:    "This activation key has already been used on another instance",
		Retryable:  false,
	})
	msg = userFriendlyActivationError(nonRetryableErr)
	if msg != "This activation key has already been used on another instance" {
		t.Errorf("non-retryable server error should pass through server message, got: %q", msg)
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

func TestHandleClearLicense_ClearsActiveTrialButPreservesTrialUsedMarker(t *testing.T) {
	handler := createTestHandler(t)

	now := time.Now()
	startedAt := now.Add(-2 * time.Hour).Unix()
	endsAt := now.Add(12 * time.Hour).Unix()
	store := config.NewFileBillingStore(handler.mtPersistence.BaseDataDir())
	if err := store.SaveBillingState("default", &pkglicensing.BillingState{
		Capabilities:      append([]string(nil), license.TierFeatures[license.TierPro]...),
		Limits:            map[string]int64{"max_agents": 15},
		MetersEnabled:     []string{"agents"},
		PlanVersion:       "trial",
		SubscriptionState: entitlements.SubStateTrial,
		TrialStartedAt:    &startedAt,
		TrialEndsAt:       &endsAt,
	}); err != nil {
		t.Fatalf("SaveBillingState: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/license/clear", nil)
	rec := httptest.NewRecorder()
	handler.HandleClearLicense(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	cleared, err := store.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if cleared == nil {
		t.Fatal("expected billing state to remain so trial reuse stays blocked")
	}
	if cleared.SubscriptionState != pkglicensing.SubStateExpired {
		t.Fatalf("subscription_state=%q, want %q", cleared.SubscriptionState, pkglicensing.SubStateExpired)
	}
	if cleared.TrialStartedAt == nil || *cleared.TrialStartedAt != startedAt {
		t.Fatalf("trial_started_at=%v, want %d", cleared.TrialStartedAt, startedAt)
	}
	if cleared.TrialEndsAt != nil {
		t.Fatalf("trial_ends_at=%v, want nil", cleared.TrialEndsAt)
	}
	if len(cleared.Capabilities) != 0 {
		t.Fatalf("capabilities=%v, want empty", cleared.Capabilities)
	}

	entReq := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil)
	entRec := httptest.NewRecorder()
	handler.HandleEntitlements(entRec, entReq)
	if entRec.Code != http.StatusOK {
		t.Fatalf("entitlements status=%d, want %d: %s", entRec.Code, http.StatusOK, entRec.Body.String())
	}

	var payload EntitlementPayload
	if err := json.Unmarshal(entRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode entitlements payload: %v", err)
	}
	if payload.SubscriptionState != string(license.SubStateExpired) {
		t.Fatalf("payload.subscription_state=%q, want %q", payload.SubscriptionState, license.SubStateExpired)
	}
	if payload.TrialEligible {
		t.Fatalf("payload.trial_eligible=%v, want false", payload.TrialEligible)
	}
	if payload.TrialEligibilityReason != "already_used" {
		t.Fatalf("payload.trial_eligibility_reason=%q, want %q", payload.TrialEligibilityReason, "already_used")
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
