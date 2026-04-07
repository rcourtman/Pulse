package api

import (
	"context"
	"sync"
	"testing"
	"time"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
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
	licenseKey, err := pkglicensing.GenerateLicenseForTesting("limits@example.com", pkglicensing.TierPro, 24*time.Hour)
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
