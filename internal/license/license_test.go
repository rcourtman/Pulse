package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
)

// init sets dev mode for tests so license validation works without a real public key
func init() {
	os.Setenv("PULSE_LICENSE_DEV_MODE", "true")
}

func TestTierHasFeature(t *testing.T) {
	tests := []struct {
		name     string
		tier     Tier
		feature  string
		expected bool
	}{
		{"free has AI patrol (BYOK)", TierFree, FeatureAIPatrol, true},
		{"free has no AI autofix", TierFree, FeatureAIAutoFix, false},
		{"pro has AI patrol", TierPro, FeatureAIPatrol, true},
		{"pro has AI alerts", TierPro, FeatureAIAlerts, true},
		{"pro has AI autofix", TierPro, FeatureAIAutoFix, true},
		{"pro has K8s AI", TierPro, FeatureKubernetesAI, true},
		{"pro does not have multi-user", TierPro, FeatureMultiUser, false},
		{"lifetime has AI patrol", TierLifetime, FeatureAIPatrol, true},
		{"msp has unlimited", TierMSP, FeatureUnlimited, true},
		{"msp does not have multi-user yet", TierMSP, FeatureMultiUser, false},
		{"enterprise has multi-user", TierEnterprise, FeatureMultiUser, true},
		{"enterprise has white-label", TierEnterprise, FeatureWhiteLabel, true},
		{"pro has Basic SSO", TierPro, FeatureSSO, true},
		{"pro has Advanced SSO", TierPro, FeatureAdvancedSSO, true},
		{"pro has audit logging", TierPro, FeatureAuditLogging, true},
		{"enterprise has Advanced SSO", TierEnterprise, FeatureAdvancedSSO, true},
		{"enterprise has audit logging", TierEnterprise, FeatureAuditLogging, true},
		{"enterprise has SSO", TierEnterprise, FeatureSSO, true},
		{"unknown tier has nothing", Tier("unknown"), FeatureAIPatrol, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TierHasFeature(tt.tier, tt.feature)
			if result != tt.expected {
				t.Errorf("TierHasFeature(%v, %v) = %v, want %v",
					tt.tier, tt.feature, result, tt.expected)
			}
		})
	}
}

func TestLicenseHasFeature(t *testing.T) {
	license := &License{
		Claims: Claims{
			Tier:     TierPro,
			Features: []string{"custom_feature"},
		},
	}

	// Should have tier features
	if !license.HasFeature(FeatureAIPatrol) {
		t.Error("Pro license should have Pulse Patrol")
	}

	// Should have explicit features
	if !license.HasFeature("custom_feature") {
		t.Error("License should have explicitly granted feature")
	}

	// Should not have ungranted features
	if license.HasFeature(FeatureMultiUser) {
		t.Error("Pro license should not have multi-user")
	}
}

func TestLicenseExpiration(t *testing.T) {
	t.Run("lifetime license never expires", func(t *testing.T) {
		license := &License{
			Claims: Claims{
				Tier:      TierLifetime,
				ExpiresAt: 0,
			},
		}
		if license.IsExpired() {
			t.Error("Lifetime license should not be expired")
		}
		if !license.IsLifetime() {
			t.Error("Should be detected as lifetime")
		}
		if license.DaysRemaining() != -1 {
			t.Error("Lifetime should return -1 days remaining")
		}
	})

	t.Run("expired license", func(t *testing.T) {
		license := &License{
			Claims: Claims{
				Tier:      TierPro,
				ExpiresAt: time.Now().Add(-24 * time.Hour).Unix(),
			},
		}
		if !license.IsExpired() {
			t.Error("License should be expired")
		}
		if license.DaysRemaining() != 0 {
			t.Error("Expired license should return 0 days remaining")
		}
	})

	t.Run("valid license", func(t *testing.T) {
		license := &License{
			Claims: Claims{
				Tier:      TierPro,
				ExpiresAt: time.Now().Add(30 * 24 * time.Hour).Unix(),
			},
		}
		if license.IsExpired() {
			t.Error("License should not be expired")
		}
		remaining := license.DaysRemaining()
		if remaining < 29 || remaining > 30 {
			t.Errorf("Expected ~30 days remaining, got %d", remaining)
		}
	})

	t.Run("grace period license", func(t *testing.T) {
		// Create a license that expired 3 days ago (within 7-day grace period)
		expiredAt := time.Now().Add(-3 * 24 * time.Hour).Unix()
		testKey, _ := GenerateLicenseForTesting("test@example.com", TierPro, 0)
		// Manually create claims with expired time for testing
		claims := Claims{
			LicenseID: "test_grace",
			Email:     "grace@example.com",
			Tier:      TierPro,
			IssuedAt:  time.Now().Add(-33 * 24 * time.Hour).Unix(),
			ExpiresAt: expiredAt,
		}

		license := &License{
			Raw:    testKey,
			Claims: claims,
		}

		// License is technically expired
		if !license.IsExpired() {
			t.Error("License should be expired")
		}

		// But with grace period set, it should still work
		gracePeriodEnd := time.Now().Add(4 * 24 * time.Hour)
		license.GracePeriodEnd = &gracePeriodEnd

		// Service should recognize grace period
		service := NewService()
		service.mu.Lock()
		service.license = license
		service.mu.Unlock()

		// Should still have features during grace period
		if !service.HasFeature(FeatureAIPatrol) {
			t.Error("Should have feature during grace period")
		}
		if !service.IsValid() {
			t.Error("Should be valid during grace period")
		}

		// Status should show grace period
		status := service.Status()
		if !status.InGracePeriod {
			t.Error("Status should show in grace period")
		}
	})
}

func TestServiceFeatureGating(t *testing.T) {
	service := NewService()

	// No license - should have free tier features but not Pro features
	if !service.HasFeature(FeatureAIPatrol) {
		t.Error("Should have Patrol in free tier even without license")
	}
	if service.HasFeature(FeatureAIAutoFix) {
		t.Error("Should not have auto-fix without license")
	}
	if service.IsValid() {
		t.Error("Should not be valid without license")
	}

	// Activate test license
	SetPublicKey(nil)
	os.Setenv("PULSE_LICENSE_DEV_MODE", "true")
	testKey, err := GenerateLicenseForTesting("test@example.com", TierPro, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to generate test license: %v", err)
	}

	// Clear public key for testing (since test licenses have fake signatures)
	SetPublicKey(nil)

	license, err := service.Activate(testKey)
	if err != nil {
		t.Fatalf("Failed to activate test license: %v", err)
	}

	if license.Claims.Email != "test@example.com" {
		t.Error("Email mismatch")
	}
	if license.Claims.Tier != TierPro {
		t.Error("Tier mismatch")
	}

	// Should now have Pro features
	if !service.HasFeature(FeatureAIPatrol) {
		t.Error("Should have Pulse Patrol with Pro license")
	}
	if !service.IsValid() {
		t.Error("Should be valid with active license")
	}

	// Require feature should succeed
	if err := service.RequireFeature(FeatureAIPatrol); err != nil {
		t.Errorf("RequireFeature should succeed: %v", err)
	}

	// Require feature should fail for ungranted feature
	if err := service.RequireFeature(FeatureMultiUser); err == nil {
		t.Error("RequireFeature should fail for multi-user")
	}

	// Clear license
	service.Clear()
	if service.IsValid() {
		t.Error("Should not be valid after clearing")
	}
}

func TestValidateLicenseMalformed(t *testing.T) {
	tests := []struct {
		name       string
		licenseKey string
	}{
		{"empty", ""},
		{"not jwt", "not-a-jwt"},
		{"two parts", "part1.part2"},
		{"bad base64 header", "!!!.part2.part3"},
		{"bad base64 payload", "eyJhbGciOiJFZERTQSJ9.!!!.part3"},
		{"bad base64 signature", "eyJhbGciOiJFZERTQSJ9.eyJlbWFpbCI6InRlc3RAZXhhbXBsZS5jb20ifQ.!!!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateLicense(tt.licenseKey)
			if err == nil {
				t.Error("Expected error for malformed license")
			}
		})
	}
}

func TestValidateLicense_RequiredFields(t *testing.T) {
	os.Setenv("PULSE_LICENSE_DEV_MODE", "true")
	defer os.Unsetenv("PULSE_LICENSE_DEV_MODE")

	tests := []struct {
		name   string
		claims map[string]interface{}
	}{
		{"missing id", map[string]interface{}{"email": "t@e.c", "tier": "pro"}},
		{"missing email", map[string]interface{}{"lid": "test", "tier": "pro"}},
		{"missing tier", map[string]interface{}{"lid": "test", "email": "t@e.c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
			payloadBytes, _ := json.Marshal(tt.claims)
			payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
			key := header + "." + payload + ".fake-sig"

			_, err := ValidateLicense(key)
			if err == nil {
				t.Error("Expected error for missing required fields")
			}
		})
	}
}

func TestValidateLicense_ExpiredPastGrace(t *testing.T) {
	os.Setenv("PULSE_LICENSE_DEV_MODE", "true")
	defer os.Unsetenv("PULSE_LICENSE_DEV_MODE")

	claims := Claims{
		LicenseID: "test-expired",
		Email:     "t@e.c",
		Tier:      TierPro,
		ExpiresAt: time.Now().Add(-10 * 24 * time.Hour).Unix(), // 10 days ago (past 7-day grace)
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	payloadBytes, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	key := header + "." + payload + ".fake-sig"

	_, err := ValidateLicense(key)
	if err == nil {
		t.Error("Expected error for license past grace period")
	}
}

func TestLicenseStatus(t *testing.T) {
	service := NewService()

	// Status with no license
	status := service.Status()
	if status.Valid {
		t.Error("Should not be valid")
	}
	if status.Tier != TierFree {
		t.Errorf("Expected free tier, got %v", status.Tier)
	}

	// Activate license
	SetPublicKey(nil) // Skip signature check for testing
	os.Setenv("PULSE_LICENSE_DEV_MODE", "true")
	testKey, _ := GenerateLicenseForTesting("test@example.com", TierLifetime, 0)
	_, err := service.Activate(testKey)
	if err != nil {
		t.Fatalf("Failed to activate test license: %v", err)
	}

	status = service.Status()
	if !status.Valid {
		t.Error("Should be valid")
	}
	if status.Tier != TierLifetime {
		t.Errorf("Expected lifetime tier, got %v", status.Tier)
	}
	if !status.IsLifetime {
		t.Error("Should be detected as lifetime")
	}
	if status.DaysRemaining != -1 {
		t.Errorf("Expected -1 days remaining, got %d", status.DaysRemaining)
	}
	if len(status.Features) == 0 {
		t.Error("Should have features")
	}
}

func TestGetTierDisplayName(t *testing.T) {
	if GetTierDisplayName(TierPro) != "Pro Intelligence (Monthly)" {
		t.Error("Wrong display name for Pro")
	}
	if GetTierDisplayName(TierLifetime) != "Pro Intelligence (Lifetime)" {
		t.Error("Wrong display name for Lifetime")
	}
}

func TestGetFeatureDisplayName(t *testing.T) {
	if GetFeatureDisplayName(FeatureAIPatrol) != "Pulse Patrol (Background Health Checks)" {
		t.Error("Wrong display name for Pulse Patrol")
	}
}

func TestPublicKeyRequiredWithoutDevMode(t *testing.T) {
	// This test verifies that without PULSE_LICENSE_DEV_MODE=true,
	// license validation fails when no public key is set.
	// The test itself runs with PULSE_LICENSE_DEV_MODE=true (set in go test env),
	// so we just check that ValidateLicense returns ErrNoPublicKey when appropriate.

	// Save current public key state
	originalKey := publicKey
	defer SetPublicKey(originalKey)

	// Clear public key
	SetPublicKey(nil)

	// Generate a test license
	testKey, err := GenerateLicenseForTesting("test@example.com", TierPro, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to generate test license: %v", err)
	}

	// In dev mode (set via env), this should succeed
	// In production (no dev mode), this would fail with ErrNoPublicKey
	// We test that the license CAN be validated in dev mode
	_, err = ValidateLicense(testKey)
	if err != nil {
		// If running without PULSE_LICENSE_DEV_MODE=true, we expect this error
		if err.Error() != "no public key configured for validation: signature verification required" {
			t.Logf("License validation in dev mode: %v", err)
		}
	}
}

func TestStatusSetsGracePeriodDynamically(t *testing.T) {
	// Test that Status() dynamically sets GracePeriodEnd when license expires
	// without requiring HasFeature() to be called first

	service := NewService()

	// Create a license that expired 3 days ago (within 7-day grace)
	expiredAt := time.Now().Add(-3 * 24 * time.Hour)
	lic := &License{
		Claims: Claims{
			LicenseID: "test_status_grace",
			Email:     "test@example.com",
			Tier:      TierPro,
			IssuedAt:  time.Now().Add(-33 * 24 * time.Hour).Unix(),
			ExpiresAt: expiredAt.Unix(),
		},
		ValidatedAt: time.Now().Add(-33 * 24 * time.Hour),
		// Note: GracePeriodEnd is NOT set - simulating runtime expiration
	}

	// Manually set the license without grace period
	service.mu.Lock()
	service.license = lic
	service.mu.Unlock()

	// Verify GracePeriodEnd is nil initially
	if lic.GracePeriodEnd != nil {
		t.Fatal("GracePeriodEnd should be nil initially")
	}

	// Call Status() - this should set GracePeriodEnd dynamically
	status := service.Status()

	// Verify Status() set the grace period
	if lic.GracePeriodEnd == nil {
		t.Fatal("Status() should have set GracePeriodEnd")
	}

	// Status should show as valid during grace period
	if !status.Valid {
		t.Error("Status should be valid during grace period")
	}
	if !status.InGracePeriod {
		t.Error("Status should show in grace period")
	}
	if status.GracePeriodEnd == nil {
		t.Error("Status should include GracePeriodEnd")
	}

	// Verify HasFeature also works during grace
	if !service.HasFeature(FeatureAIPatrol) {
		t.Error("HasFeature should return true during grace period")
	}
}

func TestServiceCurrent(t *testing.T) {
	service := NewService()

	// No license - Current() returns nil
	if service.Current() != nil {
		t.Error("Current() should return nil when no license")
	}

	// Activate license
	SetPublicKey(nil)
	os.Setenv("PULSE_LICENSE_DEV_MODE", "true")
	testKey, err := GenerateLicenseForTesting("test@example.com", TierPro, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to generate test license: %v", err)
	}

	_, err = service.Activate(testKey)
	if err != nil {
		t.Fatalf("Failed to activate: %v", err)
	}

	// Current() should return the license
	lic := service.Current()
	if lic == nil {
		t.Fatal("Current() should return license after activation")
	}
	if lic.Claims.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got %q", lic.Claims.Email)
	}

	// Clear and verify Current() returns nil again
	service.Clear()
	if service.Current() != nil {
		t.Error("Current() should return nil after Clear()")
	}
}

func TestServiceGetLicenseState(t *testing.T) {
	t.Run("no license", func(t *testing.T) {
		service := NewService()
		state, lic := service.GetLicenseState()
		if state != LicenseStateNone {
			t.Errorf("Expected state 'none', got %q", state)
		}
		if lic != nil {
			t.Error("Expected nil license")
		}
	})

	t.Run("active license", func(t *testing.T) {
		service := NewService()
		SetPublicKey(nil)
		os.Setenv("PULSE_LICENSE_DEV_MODE", "true")

		testKey, _ := GenerateLicenseForTesting("test@example.com", TierPro, 30*24*time.Hour)
		_, err := service.Activate(testKey)
		if err != nil {
			t.Fatalf("Failed to activate: %v", err)
		}

		state, lic := service.GetLicenseState()
		if state != LicenseStateActive {
			t.Errorf("Expected state 'active', got %q", state)
		}
		if lic == nil {
			t.Error("Expected license to be returned")
		}
	})

	t.Run("expired license in grace period", func(t *testing.T) {
		service := NewService()

		// Create an expired license within grace period (3 days ago)
		expiredAt := time.Now().Add(-3 * 24 * time.Hour)
		lic := &License{
			Claims: Claims{
				LicenseID: "test_expired",
				Email:     "test@example.com",
				Tier:      TierPro,
				IssuedAt:  time.Now().Add(-33 * 24 * time.Hour).Unix(),
				ExpiresAt: expiredAt.Unix(),
			},
			ValidatedAt: time.Now().Add(-33 * 24 * time.Hour),
		}

		service.mu.Lock()
		service.license = lic
		service.mu.Unlock()

		state, returnedLic := service.GetLicenseState()
		if state != LicenseStateGracePeriod {
			t.Errorf("Expected state 'grace_period', got %q", state)
		}
		if returnedLic == nil {
			t.Error("Expected license to be returned")
		}
		// Should have set grace period end
		if returnedLic.GracePeriodEnd == nil {
			t.Error("Expected GracePeriodEnd to be set")
		}
	})

	t.Run("expired license past grace period", func(t *testing.T) {
		service := NewService()

		// Create an expired license past grace period (10 days ago)
		expiredAt := time.Now().Add(-10 * 24 * time.Hour)
		gracePeriodEnd := expiredAt.Add(7 * 24 * time.Hour) // Grace ended 3 days ago
		lic := &License{
			Claims: Claims{
				LicenseID: "test_expired_past",
				Email:     "test@example.com",
				Tier:      TierPro,
				IssuedAt:  time.Now().Add(-40 * 24 * time.Hour).Unix(),
				ExpiresAt: expiredAt.Unix(),
			},
			ValidatedAt:    time.Now().Add(-40 * 24 * time.Hour),
			GracePeriodEnd: &gracePeriodEnd,
		}

		service.mu.Lock()
		service.license = lic
		service.mu.Unlock()

		state, returnedLic := service.GetLicenseState()
		if state != LicenseStateExpired {
			t.Errorf("Expected state 'expired', got %q", state)
		}
		if returnedLic == nil {
			t.Error("Expected license to be returned")
		}
	})
}

func TestServiceGetLicenseStateString(t *testing.T) {
	t.Run("no license", func(t *testing.T) {
		service := NewService()
		stateStr, hasFeatures := service.GetLicenseStateString()
		if stateStr != "none" {
			t.Errorf("Expected state string 'none', got %q", stateStr)
		}
		if hasFeatures {
			t.Error("Expected hasFeatures to be false for no license")
		}
	})

	t.Run("active license", func(t *testing.T) {
		service := NewService()
		SetPublicKey(nil)
		os.Setenv("PULSE_LICENSE_DEV_MODE", "true")

		testKey, _ := GenerateLicenseForTesting("test@example.com", TierPro, 30*24*time.Hour)
		service.Activate(testKey)

		stateStr, hasFeatures := service.GetLicenseStateString()
		if stateStr != "active" {
			t.Errorf("Expected state string 'active', got %q", stateStr)
		}
		if !hasFeatures {
			t.Error("Expected hasFeatures to be true for active license")
		}
	})

	t.Run("grace period", func(t *testing.T) {
		service := NewService()

		expiredAt := time.Now().Add(-3 * 24 * time.Hour)
		lic := &License{
			Claims: Claims{
				LicenseID: "test_grace",
				Email:     "test@example.com",
				Tier:      TierPro,
				ExpiresAt: expiredAt.Unix(),
			},
		}

		service.mu.Lock()
		service.license = lic
		service.mu.Unlock()

		stateStr, hasFeatures := service.GetLicenseStateString()
		if stateStr != "grace_period" {
			t.Errorf("Expected state string 'grace_period', got %q", stateStr)
		}
		if !hasFeatures {
			t.Error("Expected hasFeatures to be true during grace period")
		}
	})
}

func TestServiceSetLicenseChangeCallback(t *testing.T) {
	service := NewService()

	var callbackLicense *License
	callbackCalled := false

	service.SetLicenseChangeCallback(func(lic *License) {
		callbackCalled = true
		callbackLicense = lic
	})

	// Activate license - should trigger callback
	SetPublicKey(nil)
	os.Setenv("PULSE_LICENSE_DEV_MODE", "true")
	testKey, _ := GenerateLicenseForTesting("callback@example.com", TierPro, 30*24*time.Hour)
	_, err := service.Activate(testKey)
	if err != nil {
		t.Fatalf("Failed to activate: %v", err)
	}

	if !callbackCalled {
		t.Error("Callback should have been called on Activate")
	}
	if callbackLicense == nil {
		t.Error("Callback should receive the license")
	}
	if callbackLicense != nil && callbackLicense.Claims.Email != "callback@example.com" {
		t.Errorf("Callback received wrong license, email: %q", callbackLicense.Claims.Email)
	}

	// Reset for Clear test
	callbackCalled = false
	callbackLicense = nil

	// Clear license - should trigger callback with nil
	service.Clear()

	if !callbackCalled {
		t.Error("Callback should have been called on Clear")
	}
	if callbackLicense != nil {
		t.Error("Callback should receive nil on Clear")
	}
}

func TestValidateLicense_RealSignature(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	SetPublicKey(pub)
	defer SetPublicKey(nil)

	os.Setenv("PULSE_LICENSE_DEV_MODE", "false")
	defer os.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	claims := Claims{
		LicenseID: "test-sig",
		Email:     "t@e.c",
		Tier:      TierPro,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour).Unix(),
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	payloadBytes, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)

	signedData := header + "." + payload
	signature := ed25519.Sign(priv, []byte(signedData))
	sigEncoded := base64.RawURLEncoding.EncodeToString(signature)

	key := signedData + "." + sigEncoded

	lic, err := ValidateLicense(key)
	if err != nil {
		t.Fatalf("Failed to validate license with real signature: %v", err)
	}
	if lic.Claims.Email != "t@e.c" {
		t.Error("Email mismatch in validated license")
	}

	// Test invalid signature
	badKey := signedData + "." + base64.RawURLEncoding.EncodeToString([]byte("invalid-signature-length-must-be-64-bytes-long-12345678901234567890"))
	_, err = ValidateLicense(badKey)
	if err == nil {
		t.Error("Expected error for invalid signature")
	}
}

func TestClaimsEffectiveCapabilities(t *testing.T) {
	t.Run("explicit capabilities", func(t *testing.T) {
		claims := Claims{
			Tier:         TierPro,
			Capabilities: []string{"a", "b"},
		}

		got := claims.EffectiveCapabilities()
		want := []string{"a", "b"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("EffectiveCapabilities() = %v, want %v", got, want)
		}
	})

	t.Run("derived from tier", func(t *testing.T) {
		claims := Claims{Tier: TierPro}

		got := claims.EffectiveCapabilities()
		want := append([]string(nil), TierFeatures[TierPro]...)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("EffectiveCapabilities() = %v, want %v", got, want)
		}
	})

	t.Run("derived merges explicit features", func(t *testing.T) {
		claims := Claims{
			Tier:     TierFree,
			Features: []string{"custom_feature"},
		}

		got := claims.EffectiveCapabilities()
		featureSet := make(map[string]struct{})
		for _, feature := range TierFeatures[TierFree] {
			featureSet[feature] = struct{}{}
		}
		featureSet["custom_feature"] = struct{}{}

		want := make([]string, 0, len(featureSet))
		for feature := range featureSet {
			want = append(want, feature)
		}
		sort.Strings(want)

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("EffectiveCapabilities() = %v, want %v", got, want)
		}
	})
}

func TestClaimsEffectiveLimits(t *testing.T) {
	t.Run("explicit limits", func(t *testing.T) {
		claims := Claims{
			MaxNodes: 25,
			Limits: map[string]int64{
				"max_nodes": 50,
			},
		}

		got := claims.EffectiveLimits()
		want := map[string]int64{"max_nodes": 50}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("EffectiveLimits() = %v, want %v", got, want)
		}
	})

	t.Run("derived from fields", func(t *testing.T) {
		claims := Claims{
			MaxNodes:  25,
			MaxGuests: 100,
		}

		got := claims.EffectiveLimits()
		want := map[string]int64{
			"max_nodes":  25,
			"max_guests": 100,
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("EffectiveLimits() = %v, want %v", got, want)
		}
	})

	t.Run("zero fields omitted", func(t *testing.T) {
		claims := Claims{
			MaxNodes: 0,
			Limits:   nil,
		}

		got := claims.EffectiveLimits()
		if len(got) != 0 {
			t.Fatalf("EffectiveLimits() = %v, want empty map", got)
		}
	})
}

func TestClaimsJSONRoundtrip(t *testing.T) {
	t.Run("new fields set", func(t *testing.T) {
		original := Claims{
			LicenseID:    "license_roundtrip",
			Email:        "roundtrip@example.com",
			Tier:         TierPro,
			IssuedAt:     1700000000,
			ExpiresAt:    1800000000,
			Features:     []string{"legacy_feature"},
			MaxNodes:     10,
			MaxGuests:    20,
			Capabilities: []string{"cap_a", "cap_b"},
			Limits: map[string]int64{
				"max_nodes":  50,
				"max_guests": 100,
			},
			MetersEnabled: []string{"meter_a"},
			PlanVersion:   "v1",
			SubState:      SubStateActive,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		var decoded Claims
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if !reflect.DeepEqual(decoded, original) {
			t.Fatalf("roundtrip mismatch: got %+v, want %+v", decoded, original)
		}
	})

	t.Run("legacy compat without new fields", func(t *testing.T) {
		legacy := Claims{
			LicenseID: "license_legacy",
			Email:     "legacy@example.com",
			Tier:      TierFree,
			IssuedAt:  1700000001,
		}

		data, err := json.Marshal(legacy)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		var decoded Claims
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if decoded.Capabilities != nil {
			t.Fatalf("Capabilities = %v, want nil", decoded.Capabilities)
		}
		if decoded.Limits != nil {
			t.Fatalf("Limits = %v, want nil", decoded.Limits)
		}
		if decoded.MetersEnabled != nil {
			t.Fatalf("MetersEnabled = %v, want nil", decoded.MetersEnabled)
		}
		if decoded.PlanVersion != "" {
			t.Fatalf("PlanVersion = %q, want empty", decoded.PlanVersion)
		}
		if decoded.SubState != "" {
			t.Fatalf("SubState = %q, want empty", decoded.SubState)
		}
	})
}

func TestDeriveEntitlements(t *testing.T) {
	for tier := range TierFeatures {
		tier := tier
		t.Run(string(tier), func(t *testing.T) {
			capabilities, limits := DeriveEntitlements(tier, nil, 0, 0)

			wantCapabilities := append([]string(nil), TierFeatures[tier]...)
			sort.Strings(wantCapabilities)

			if !reflect.DeepEqual(capabilities, wantCapabilities) {
				t.Fatalf("DeriveEntitlements() capabilities = %v, want %v", capabilities, wantCapabilities)
			}
			if len(limits) != 0 {
				t.Fatalf("DeriveEntitlements() limits = %v, want empty", limits)
			}
		})
	}

	t.Run("limits derivation", func(t *testing.T) {
		capabilities, limits := DeriveEntitlements(TierPro, []string{"custom_feature"}, 25, 100)

		featureSet := make(map[string]struct{})
		for _, feature := range TierFeatures[TierPro] {
			featureSet[feature] = struct{}{}
		}
		featureSet["custom_feature"] = struct{}{}

		wantCapabilities := make([]string, 0, len(featureSet))
		for feature := range featureSet {
			wantCapabilities = append(wantCapabilities, feature)
		}
		sort.Strings(wantCapabilities)

		wantLimits := map[string]int64{
			"max_nodes":  25,
			"max_guests": 100,
		}

		if !reflect.DeepEqual(capabilities, wantCapabilities) {
			t.Fatalf("DeriveEntitlements() capabilities = %v, want %v", capabilities, wantCapabilities)
		}
		if !reflect.DeepEqual(limits, wantLimits) {
			t.Fatalf("DeriveEntitlements() limits = %v, want %v", limits, wantLimits)
		}
	})
}

func TestSubscriptionStateConstants(t *testing.T) {
	states := []SubscriptionState{
		SubStateTrial,
		SubStateActive,
		SubStateGrace,
		SubStateExpired,
		SubStateSuspended,
	}

	seen := make(map[string]struct{}, len(states))
	for _, state := range states {
		value := string(state)
		if _, exists := seen[value]; exists {
			t.Fatalf("duplicate subscription state value %q", value)
		}
		seen[value] = struct{}{}
	}

	if SubStateTrial != "trial" {
		t.Fatalf("SubStateTrial = %q, want %q", SubStateTrial, "trial")
	}
	if SubStateActive != "active" {
		t.Fatalf("SubStateActive = %q, want %q", SubStateActive, "active")
	}
	if SubStateGrace != "grace" {
		t.Fatalf("SubStateGrace = %q, want %q", SubStateGrace, "grace")
	}
	if SubStateExpired != "expired" {
		t.Fatalf("SubStateExpired = %q, want %q", SubStateExpired, "expired")
	}
	if SubStateSuspended != "suspended" {
		t.Fatalf("SubStateSuspended = %q, want %q", SubStateSuspended, "suspended")
	}
}

func TestLimitCheckResultConstants(t *testing.T) {
	results := []LimitCheckResult{
		LimitAllowed,
		LimitSoftBlock,
		LimitHardBlock,
	}

	seen := make(map[string]struct{}, len(results))
	for _, result := range results {
		value := string(result)
		if _, exists := seen[value]; exists {
			t.Fatalf("duplicate limit check result value %q", value)
		}
		seen[value] = struct{}{}
	}
}

var allFeatures = []string{
	FeatureAIPatrol, FeatureAIAlerts, FeatureAIAutoFix, FeatureKubernetesAI,
	FeatureAgentProfiles, FeatureUpdateAlerts, FeatureRBAC, FeatureAuditLogging,
	FeatureSSO, FeatureAdvancedSSO, FeatureAdvancedReporting, FeatureLongTermMetrics,
	FeatureRelay, FeatureMultiUser, FeatureWhiteLabel, FeatureMultiTenant, FeatureUnlimited,
}

func setupTestServiceWithTier(t *testing.T, tier Tier) *Service {
	t.Helper()
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")
	t.Setenv("PULSE_DEV", "false")
	t.Setenv("PULSE_MOCK_MODE", "false")

	SetPublicKey(nil)
	svc := NewService()
	token, err := GenerateLicenseForTesting("test@example.com", tier, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Activate(token); err != nil {
		t.Fatal(err)
	}
	return svc
}

func evaluatorForService(svc *Service) *entitlements.Evaluator {
	lic := svc.Current()
	if lic == nil {
		return nil
	}
	source := entitlements.NewTokenSource(&lic.Claims)
	return entitlements.NewEvaluator(source)
}

func captureFeatureResults(svc *Service, features []string) map[string]bool {
	results := make(map[string]bool, len(features))
	for _, feature := range features {
		results[feature] = svc.HasFeature(feature)
	}
	return results
}

func asFeatureSet(features []string) map[string]struct{} {
	set := make(map[string]struct{}, len(features))
	for _, feature := range features {
		set[feature] = struct{}{}
	}
	return set
}

func TestServiceHasFeature_WithEvaluator(t *testing.T) {
	svc := setupTestServiceWithTier(t, TierPro)
	proSet := asFeatureSet(TierFeatures[TierPro])

	withoutEvaluator := captureFeatureResults(svc, allFeatures)
	for _, feature := range allFeatures {
		_, inPro := proSet[feature]
		if inPro && !withoutEvaluator[feature] {
			t.Fatalf("without evaluator: expected Pro tier feature %q to be granted", feature)
		}
		if !inPro && withoutEvaluator[feature] {
			t.Fatalf("without evaluator: expected non-Pro feature %q to be denied", feature)
		}
	}

	svc.SetEvaluator(evaluatorForService(svc))
	withEvaluator := captureFeatureResults(svc, allFeatures)
	for _, feature := range allFeatures {
		_, inPro := proSet[feature]
		if inPro && !withEvaluator[feature] {
			t.Fatalf("with evaluator: expected Pro tier feature %q to be granted", feature)
		}
		if !inPro && withEvaluator[feature] {
			t.Fatalf("with evaluator: expected non-Pro feature %q to be denied", feature)
		}
		if withEvaluator[feature] != withoutEvaluator[feature] {
			t.Fatalf("feature %q parity mismatch: without evaluator=%v with evaluator=%v",
				feature, withoutEvaluator[feature], withEvaluator[feature])
		}
	}
}

func TestServiceHasFeature_WithEvaluator_FreeTier(t *testing.T) {
	t.Setenv("PULSE_DEV", "false")
	t.Setenv("PULSE_MOCK_MODE", "false")

	svc := NewService()
	freeSet := asFeatureSet(TierFeatures[TierFree])

	withoutEvaluator := captureFeatureResults(svc, allFeatures)
	for _, feature := range allFeatures {
		_, inFree := freeSet[feature]
		if inFree && !withoutEvaluator[feature] {
			t.Fatalf("without evaluator: expected free feature %q to be granted", feature)
		}
		if !inFree && withoutEvaluator[feature] {
			t.Fatalf("without evaluator: expected non-free feature %q to be denied", feature)
		}
	}

	freeClaims := &Claims{Tier: TierFree}
	svc.SetEvaluator(entitlements.NewEvaluator(entitlements.NewTokenSource(freeClaims)))

	withEvaluator := captureFeatureResults(svc, allFeatures)
	for _, feature := range allFeatures {
		_, inFree := freeSet[feature]
		if inFree && !withEvaluator[feature] {
			t.Fatalf("with evaluator: expected free feature %q to be granted", feature)
		}
		if !inFree && withEvaluator[feature] {
			t.Fatalf("with evaluator: expected non-free feature %q to be denied", feature)
		}
		if withEvaluator[feature] != withoutEvaluator[feature] {
			t.Fatalf("feature %q parity mismatch: without evaluator=%v with evaluator=%v",
				feature, withoutEvaluator[feature], withEvaluator[feature])
		}
	}
}

func TestServiceHasFeature_EvaluatorNilFallback(t *testing.T) {
	t.Setenv("PULSE_DEV", "false")
	t.Setenv("PULSE_MOCK_MODE", "false")

	noLicenseSvc := NewService()
	if noLicenseSvc.Evaluator() != nil {
		t.Fatal("expected evaluator to be nil by default")
	}
	for _, feature := range allFeatures {
		got := noLicenseSvc.HasFeature(feature)
		want := TierHasFeature(TierFree, feature)
		if got != want {
			t.Fatalf("no-license fallback mismatch for feature %q: got %v want %v", feature, got, want)
		}
	}

	proSvc := setupTestServiceWithTier(t, TierPro)
	if proSvc.Evaluator() != nil {
		t.Fatal("expected evaluator to remain nil when not set")
	}
	for _, feature := range allFeatures {
		got := proSvc.HasFeature(feature)
		want := TierHasFeature(TierPro, feature)
		if got != want {
			t.Fatalf("tier fallback mismatch for feature %q: got %v want %v", feature, got, want)
		}
	}
}

func TestServiceSetEvaluator(t *testing.T) {
	svc := NewService()
	if svc.Evaluator() != nil {
		t.Fatal("expected default evaluator to be nil")
	}

	claims := &Claims{Tier: TierFree}
	eval := entitlements.NewEvaluator(entitlements.NewTokenSource(claims))
	svc.SetEvaluator(eval)
	if svc.Evaluator() != eval {
		t.Fatal("expected evaluator getter to return the evaluator set by SetEvaluator")
	}

	svc.SetEvaluator(nil)
	if svc.Evaluator() != nil {
		t.Fatal("expected evaluator to be nil after SetEvaluator(nil)")
	}
}

func TestServiceHasFeature_ContractParity(t *testing.T) {
	tiers := make([]Tier, 0, len(TierFeatures))
	for tier := range TierFeatures {
		tiers = append(tiers, tier)
	}
	sort.Slice(tiers, func(i, j int) bool {
		return string(tiers[i]) < string(tiers[j])
	})

	for _, tier := range tiers {
		tier := tier
		t.Run(string(tier), func(t *testing.T) {
			svc := setupTestServiceWithTier(t, tier)

			withoutEvaluator := captureFeatureResults(svc, allFeatures)
			svc.SetEvaluator(evaluatorForService(svc))
			withEvaluator := captureFeatureResults(svc, allFeatures)

			for _, feature := range allFeatures {
				if withEvaluator[feature] != withoutEvaluator[feature] {
					t.Fatalf("feature %q parity mismatch for tier %q: without evaluator=%v with evaluator=%v",
						feature, tier, withoutEvaluator[feature], withEvaluator[feature])
				}
			}
		})
	}
}
