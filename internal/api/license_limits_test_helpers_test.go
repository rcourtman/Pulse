package api

import (
	"context"
	"sync"
	"testing"
	"time"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	licensetestsupport "github.com/rcourtman/pulse-go-rewrite/pkg/licensing/testsupport"
)

type staticLicenseProvider struct {
	service *pkglicensing.Service
}

var testLicenseProviderMu sync.Mutex

func (p *staticLicenseProvider) Service(context.Context) *pkglicensing.Service {
	return p.service
}

func setMaxMonitoredSystemsLicenseForTests(t *testing.T, maxMonitoredSystems int) {
	t.Helper()

	testLicenseProviderMu.Lock()

	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	service := pkglicensing.NewService()
	licenseKey, err := licensetestsupport.GenerateLicenseForTesting("limits@example.com", pkglicensing.TierEnterprise, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}

	lic, err := service.Activate(licenseKey)
	if err != nil {
		t.Fatalf("failed to activate test license: %v", err)
	}
	lic.Claims.MaxMonitoredSystems = maxMonitoredSystems

	SetLicenseServiceProvider(&staticLicenseProvider{service: service})
	t.Cleanup(func() {
		SetLicenseServiceProvider(nil)
		testLicenseProviderMu.Unlock()
	})
}

func setLicenseTierForHandlersForTests(t *testing.T, handlers *LicenseHandlers, orgID string, tier pkglicensing.Tier) {
	t.Helper()
	if handlers == nil {
		t.Fatal("license handlers are required")
	}
	ctx := context.Background()
	if orgID != "" {
		ctx = context.WithValue(ctx, OrgIDContextKey, orgID)
	}
	svc := handlers.Service(ctx)
	if svc == nil {
		t.Fatal("license service is required")
	}
	svc.SetCurrentForTesting(&pkglicensing.License{
		Claims: pkglicensing.Claims{
			LicenseID: "api-route-license-tier-test",
			Email:     "route-license@example.test",
			Tier:      tier,
			IssuedAt:  time.Now().Add(-time.Hour).Unix(),
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		},
		ValidatedAt: time.Now(),
	})
}
