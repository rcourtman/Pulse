package licensing

import (
	"strings"
	"testing"
	"time"
)

func TestServiceActivate_RejectsLegacyJWTOutsideDevMode(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	svc := NewService()
	licenseKey, err := GenerateLicenseForTesting("strict-v6@example.com", TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateLicenseForTesting: %v", err)
	}

	_, err = svc.Activate(licenseKey)
	if err == nil {
		t.Fatal("expected legacy JWT activation rejection in strict v6 mode")
	}
	if !strings.Contains(err.Error(), "/api/license/exchange") {
		t.Fatalf("expected exchange guidance error, got %q", err.Error())
	}
}

func TestServiceActivate_AllowsLegacyJWTInDevMode(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	svc := NewService()
	licenseKey, err := GenerateLicenseForTesting("dev-jwt@example.com", TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateLicenseForTesting: %v", err)
	}

	lic, err := svc.Activate(licenseKey)
	if err != nil {
		t.Fatalf("Activate in dev mode: %v", err)
	}
	if lic == nil {
		t.Fatal("expected non-nil license in dev mode")
	}
	if svc.IsActivated() {
		t.Fatal("expected dev JWT activation to remain non-activation-key mode")
	}
}
