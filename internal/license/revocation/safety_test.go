package revocation

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
)

type staticSource struct {
	capabilities []string
	limits       map[string]int64
	meters       []string
}

func (s staticSource) Capabilities() []string {
	return s.capabilities
}

func (s staticSource) Limits() map[string]int64 {
	return s.limits
}

func (s staticSource) MetersEnabled() []string {
	return s.meters
}

func (s staticSource) PlanVersion() string {
	return "test"
}

func (s staticSource) SubscriptionState() license.SubscriptionState {
	return license.SubStateActive
}

type panicSource struct {
	panicCapabilities bool
	panicLimits       bool
}

func (p panicSource) Capabilities() []string {
	if p.panicCapabilities {
		panic("capabilities boom")
	}
	return nil
}

func (p panicSource) Limits() map[string]int64 {
	if p.panicLimits {
		panic("limits boom")
	}
	return nil
}

func (p panicSource) MetersEnabled() []string {
	return nil
}

func (p panicSource) PlanVersion() string {
	return "test"
}

func (p panicSource) SubscriptionState() license.SubscriptionState {
	return license.SubStateActive
}

func TestSafeEvaluatorHasCapability_Normal(t *testing.T) {
	inner := entitlements.NewEvaluator(staticSource{
		capabilities: []string{"relay"},
	})
	safe := NewSafeEvaluator(inner)

	if !safe.HasCapability("relay") {
		t.Fatal("expected relay capability to be true")
	}

	if safe.HasCapability("nonexistent") {
		t.Fatal("expected nonexistent capability to be false")
	}
}

func TestSafeEvaluatorHasCapability_Panic(t *testing.T) {
	inner := entitlements.NewEvaluator(panicSource{
		panicCapabilities: true,
	})
	safe := NewSafeEvaluator(inner)

	if !safe.HasCapability("relay") {
		t.Fatal("expected fail-open true after panic")
	}
}

func TestSafeEvaluatorCheckLimit_Panic(t *testing.T) {
	inner := entitlements.NewEvaluator(panicSource{
		panicLimits: true,
	})
	safe := NewSafeEvaluator(inner)

	if got := safe.CheckLimit("max_nodes", 999); got != license.LimitAllowed {
		t.Fatalf("expected %q, got %q", license.LimitAllowed, got)
	}
}

func TestEnrollmentRateLimitDefaults(t *testing.T) {
	if DefaultEnrollmentRateLimit.MaxPerIPPerHour != 100 {
		t.Fatalf("expected MaxPerIPPerHour=100, got %d", DefaultEnrollmentRateLimit.MaxPerIPPerHour)
	}
	if DefaultEnrollmentRateLimit.MaxPerOrgPerHour != 100 {
		t.Fatalf("expected MaxPerOrgPerHour=100, got %d", DefaultEnrollmentRateLimit.MaxPerOrgPerHour)
	}
	if DefaultEnrollmentRateLimit.MaxGlobal != 10000 {
		t.Fatalf("expected MaxGlobal=10000, got %d", DefaultEnrollmentRateLimit.MaxGlobal)
	}
}
