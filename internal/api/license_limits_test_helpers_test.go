package api

import (
	"context"
	"testing"
	"time"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

type staticLicenseProvider struct {
	service *pkglicensing.Service
}

func (p *staticLicenseProvider) Service(context.Context) *pkglicensing.Service {
	return p.service
}

func setMaxNodesLicenseForTests(t *testing.T, maxNodes int) {
	t.Helper()

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
	lic.Claims.MaxNodes = maxNodes

	SetLicenseServiceProvider(&staticLicenseProvider{service: service})
	t.Cleanup(func() {
		SetLicenseServiceProvider(nil)
	})
}
