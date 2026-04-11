//go:build !release

package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

type devDemoFixturesEntitlementSource struct {
	capabilities      []string
	subscriptionState pkglicensing.SubscriptionState
}

func (s devDemoFixturesEntitlementSource) Capabilities() []string {
	return append([]string(nil), s.capabilities...)
}

func (s devDemoFixturesEntitlementSource) Limits() map[string]int64 { return nil }
func (s devDemoFixturesEntitlementSource) MetersEnabled() []string  { return nil }
func (s devDemoFixturesEntitlementSource) PlanVersion() string      { return "" }

func (s devDemoFixturesEntitlementSource) SubscriptionState() pkglicensing.SubscriptionState {
	if s.subscriptionState == "" {
		return pkglicensing.SubStateActive
	}
	return s.subscriptionState
}

func (s devDemoFixturesEntitlementSource) TrialStartedAt() *int64    { return nil }
func (s devDemoFixturesEntitlementSource) TrialEndsAt() *int64       { return nil }
func (s devDemoFixturesEntitlementSource) OverflowGrantedAt() *int64 { return nil }

func TestSyncReleaseDemoFixtureRuntime_DoesNotPoliceMockFixturesInDevBuilds(t *testing.T) {
	setMockModeForTest(t, true)

	handler := NewLicenseHandlers(nil, false, &config.Config{DemoMode: false})
	service := newLicenseService()
	service.SetEvaluator(pkglicensing.NewEvaluator(devDemoFixturesEntitlementSource{
		capabilities:      []string{featureAIAutoFixValue},
		subscriptionState: pkglicensing.SubStateActive,
	}))

	handler.syncReleaseDemoFixtureRuntime("default", service)

	if !mock.IsMockEnabled() {
		t.Fatal("expected dev build demo-fixture sync to leave explicit mock mode enabled")
	}
}
