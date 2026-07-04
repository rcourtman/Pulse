//go:build release

package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

type demoFixturesEntitlementSource struct {
	capabilities      []string
	subscriptionState pkglicensing.SubscriptionState
}

func (s demoFixturesEntitlementSource) Capabilities() []string {
	return append([]string(nil), s.capabilities...)
}

func (s demoFixturesEntitlementSource) Limits() map[string]int64 { return nil }
func (s demoFixturesEntitlementSource) MetersEnabled() []string  { return nil }
func (s demoFixturesEntitlementSource) PlanVersion() string      { return "" }

func (s demoFixturesEntitlementSource) SubscriptionState() pkglicensing.SubscriptionState {
	if s.subscriptionState == "" {
		return pkglicensing.SubStateActive
	}
	return s.subscriptionState
}

func (s demoFixturesEntitlementSource) TrialStartedAt() *int64    { return nil }
func (s demoFixturesEntitlementSource) TrialEndsAt() *int64       { return nil }
func (s demoFixturesEntitlementSource) OverflowGrantedAt() *int64 { return nil }

func buildDemoFixturesMonitor(t *testing.T) *monitoring.Monitor {
	t.Helper()

	cfg := &config.Config{
		ConfigPath:       t.TempDir(),
		DataPath:         t.TempDir(),
		DiscoveryEnabled: false,
		AllowedOrigins:   "*",
		EnvOverrides:     make(map[string]bool),
	}

	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	return monitor
}

func TestSyncReleaseDemoFixtureRuntime_AuthorizedDemoEnablesMockFixtures(t *testing.T) {
	previousEnabled := mock.IsMockEnabled()
	t.Cleanup(func() {
		_ = mock.SetEnabled(false)
		mock.SetReleaseFixturesAuthorized(false)
		_ = mock.SetEnabled(previousEnabled)
	})
	_ = mock.SetEnabled(false)
	mock.SetReleaseFixturesAuthorized(false)
	t.Setenv("PULSE_MOCK_MODE", "true")

	monitor := buildDemoFixturesMonitor(t)
	defer monitor.GetAlertManager().Stop()

	handler := NewLicenseHandlers(nil, false, &config.Config{DemoMode: true})
	handler.SetMonitors(monitor, nil)

	service := newLicenseService()
	service.SetEvaluator(pkglicensing.NewEvaluator(demoFixturesEntitlementSource{
		capabilities:      []string{featureDemoFixturesValue},
		subscriptionState: pkglicensing.SubStateActive,
	}))

	handler.syncReleaseDemoFixtureRuntime("default", service)

	if !mock.IsMockEnabled() {
		t.Fatal("expected demo fixture sync to enable mock mode for entitled demo runtime")
	}
}

func TestSyncReleaseDemoFixtureRuntime_UnauthorizedDisablesMockFixtures(t *testing.T) {
	previousEnabled := mock.IsMockEnabled()
	mock.SetReleaseFixturesAuthorized(true)
	if err := mock.SetEnabled(true); err != nil {
		t.Fatalf("seed mock mode: %v", err)
	}
	t.Cleanup(func() {
		_ = mock.SetEnabled(false)
		mock.SetReleaseFixturesAuthorized(false)
		_ = mock.SetEnabled(previousEnabled)
	})
	t.Setenv("PULSE_MOCK_MODE", "true")

	monitor := buildDemoFixturesMonitor(t)
	defer monitor.GetAlertManager().Stop()

	handler := NewLicenseHandlers(nil, false, &config.Config{DemoMode: true})
	handler.SetMonitors(monitor, nil)

	service := newLicenseService()
	service.SetEvaluator(pkglicensing.NewEvaluator(demoFixturesEntitlementSource{
		capabilities:      []string{featureAIAutoFixValue},
		subscriptionState: pkglicensing.SubStateActive,
	}))

	handler.syncReleaseDemoFixtureRuntime("default", service)

	if mock.IsMockEnabled() {
		t.Fatal("expected demo fixture sync to disable mock mode when entitlement is absent")
	}
}

func TestSyncReleaseDemoFixtureRuntime_IgnoresNonDefaultOrg(t *testing.T) {
	previousEnabled := mock.IsMockEnabled()
	t.Cleanup(func() {
		_ = mock.SetEnabled(false)
		mock.SetReleaseFixturesAuthorized(false)
		_ = mock.SetEnabled(previousEnabled)
	})
	_ = mock.SetEnabled(false)
	mock.SetReleaseFixturesAuthorized(false)
	t.Setenv("PULSE_MOCK_MODE", "true")

	handler := NewLicenseHandlers(nil, false, &config.Config{DemoMode: true})
	service := newLicenseService()
	service.SetEvaluator(pkglicensing.NewEvaluator(demoFixturesEntitlementSource{
		capabilities:      []string{featureDemoFixturesValue},
		subscriptionState: pkglicensing.SubStateActive,
	}))

	handler.syncReleaseDemoFixtureRuntime("tenant-a", service)

	if mock.IsMockEnabled() {
		t.Fatal("expected non-default org sync to leave process-wide mock mode untouched")
	}
}

func TestSyncReleaseDemoFixtureRuntime_FileBillingStateSeedEnablesMockFixtures(t *testing.T) {
	previousEnabled := mock.IsMockEnabled()
	t.Cleanup(func() {
		_ = mock.SetEnabled(false)
		mock.SetReleaseFixturesAuthorized(false)
		_ = mock.SetEnabled(previousEnabled)
	})
	_ = mock.SetEnabled(false)
	mock.SetReleaseFixturesAuthorized(false)
	t.Setenv("PULSE_MOCK_MODE", "true")

	baseDir := t.TempDir()
	billingState := []byte(`{"capabilities":["demo_fixtures"],"limits":{},"meters_enabled":[],"plan_version":"community","subscription_state":"active"}`)
	if err := os.WriteFile(filepath.Join(baseDir, "billing.json"), billingState, 0o600); err != nil {
		t.Fatalf("write demo billing state: %v", err)
	}

	handler := NewLicenseHandlers(
		config.NewMultiTenantPersistence(baseDir),
		false,
		&config.Config{DemoMode: true},
	)

	service, _, err := handler.getTenantComponents(context.Background())
	if err != nil {
		t.Fatalf("getTenantComponents: %v", err)
	}
	if !service.HasFeature(featureDemoFixturesValue) {
		t.Fatal("expected seeded demo billing state to authorize demo fixtures")
	}
	if !mock.IsMockEnabled() {
		t.Fatal("expected demo fixture sync to enable mock mode from seeded billing state")
	}
}
