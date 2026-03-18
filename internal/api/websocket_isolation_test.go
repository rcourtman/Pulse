package api

import (
	"context"
	"testing"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/stretchr/testify/assert"
)

// WSMockLicenseService for checking feature flags in tests
type WSMockLicenseService struct {
	Features map[string]bool
}

func (m *WSMockLicenseService) HasFeature(feature string) bool {
	return m.Features[feature]
}

func (m *WSMockLicenseService) Service(ctx context.Context) *pkglicensing.Service {
	// Return empty service (no license) by default
	return pkglicensing.NewService()
}

// WebSocketMockLicenseProvider to return our mock service
type WebSocketMockLicenseProvider struct {
	service *WSMockLicenseService
}

func (p *WebSocketMockLicenseProvider) Service(ctx context.Context) *pkglicensing.Service {
	return pkglicensing.NewService()
}

func TestWebSocketIsolation_Permanent(t *testing.T) {
	// Reset global state
	defer SetMultiTenantEnabled(false)

	checker := NewMultiTenantChecker(false) // self-hosted mode for existing tests

	// Case 1: Default Org always allowed
	result := checker.CheckMultiTenant(context.Background(), "default")
	assert.True(t, result.Allowed)
	assert.True(t, result.FeatureEnabled)
	assert.True(t, result.Licensed)

	// Case 2: Multi-Tenant Disabled (Flag=False)
	SetMultiTenantEnabled(false)
	result = checker.CheckMultiTenant(context.Background(), "tenant-1")
	assert.False(t, result.Allowed)
	assert.False(t, result.FeatureEnabled, "Feature should be disabled")
	assert.Contains(t, result.Reason, "not enabled")

	// Case 3: Flag=True, License=False
	SetMultiTenantEnabled(true)

	// Default license provider returns no license, which is what we want to test (Block unlicensed)
	result = checker.CheckMultiTenant(context.Background(), "tenant-1")
	assert.False(t, result.Allowed)
	assert.True(t, result.FeatureEnabled)
	assert.False(t, result.Licensed, "Should be unlicensed by default")
	assert.Contains(t, result.Reason, "Enterprise license")
}
