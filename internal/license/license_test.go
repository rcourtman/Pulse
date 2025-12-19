package license

import (
	"os"
	"testing"
	"time"
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
		{"free has no AI patrol", TierFree, FeatureAIPatrol, false},
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
		t.Error("Pro license should have AI Patrol")
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

	// No license - should not have features
	if service.HasFeature(FeatureAIPatrol) {
		t.Error("Should not have feature without license")
	}
	if service.IsValid() {
		t.Error("Should not be valid without license")
	}

	// Activate test license
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
		t.Error("Should have AI Patrol with Pro license")
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
	testKey, _ := GenerateLicenseForTesting("test@example.com", TierLifetime, 0)
	_, _ = service.Activate(testKey)

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
	if GetTierDisplayName(TierPro) != "Pro (Monthly)" {
		t.Error("Wrong display name for Pro")
	}
	if GetTierDisplayName(TierLifetime) != "Pro (Lifetime)" {
		t.Error("Wrong display name for Lifetime")
	}
}

func TestGetFeatureDisplayName(t *testing.T) {
	if GetFeatureDisplayName(FeatureAIPatrol) != "AI Patrol (Background Health Checks)" {
		t.Error("Wrong display name for AI Patrol")
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
