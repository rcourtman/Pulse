package api

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
)

type staticLicenseProvider struct {
	service *license.Service
}

func (p *staticLicenseProvider) Service(context.Context) *license.Service {
	return p.service
}

func setMaxNodesLicenseForTests(t *testing.T, maxNodes int) {
	t.Helper()

	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	service := license.NewService()
	licenseKey, err := license.GenerateLicenseForTesting("limits@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}

	lic, err := service.Activate(licenseKey)
	if err != nil {
		t.Fatalf("failed to activate test license: %v", err)
	}
	lic.Claims.MaxNodes = maxNodes

	SetLicenseServiceProvider(&staticLicenseProvider{service: service})
	t.Cleanup(func() {
		SetLicenseServiceProvider(nil)
	})
}
