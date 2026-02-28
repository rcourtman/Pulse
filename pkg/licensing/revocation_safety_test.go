package licensing

import (
	"testing"
)

// panicSource is an EntitlementSource that panics on every method.
type panicSource struct{}

func (panicSource) Capabilities() []string               { panic("capabilities boom") }
func (panicSource) Limits() map[string]int64             { panic("limits boom") }
func (panicSource) MetersEnabled() []string              { panic("meters boom") }
func (panicSource) PlanVersion() string                  { panic("plan boom") }
func (panicSource) SubscriptionState() SubscriptionState { panic("sub boom") }
func (panicSource) TrialStartedAt() *int64               { panic("trial start boom") }
func (panicSource) TrialEndsAt() *int64                  { panic("trial end boom") }
func (panicSource) OverflowGrantedAt() *int64            { panic("overflow boom") }

func TestSafeEvaluator_HasCapability_Delegates(t *testing.T) {
	inner := NewEvaluator(mockSource{
		capabilities: []string{"feature_a", "feature_b"},
	})
	safe := NewSafeEvaluator(inner)

	if !safe.HasCapability("feature_a") {
		t.Error("expected HasCapability(feature_a) = true")
	}
	if safe.HasCapability("feature_c") {
		t.Error("expected HasCapability(feature_c) = false")
	}
}

func TestSafeEvaluator_HasCapability_PanicRecovery(t *testing.T) {
	inner := NewEvaluator(panicSource{})
	safe := NewSafeEvaluator(inner)

	// Must not panic — should recover and return true (fail-open).
	result := safe.HasCapability("anything")
	if !result {
		t.Error("HasCapability panic recovery should fail open (return true)")
	}
}

func TestSafeEvaluator_GetLimit_Delegates(t *testing.T) {
	inner := NewEvaluator(mockSource{
		limits: map[string]int64{"max_agents": 50},
	})
	safe := NewSafeEvaluator(inner)

	limit, ok := safe.GetLimit("max_agents")
	if !ok {
		t.Error("expected ok=true for max_agents")
	}
	if limit != 50 {
		t.Errorf("limit = %d, want 50", limit)
	}

	limit, ok = safe.GetLimit("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent key")
	}
	if limit != 0 {
		t.Errorf("limit = %d, want 0 for nonexistent", limit)
	}
}

func TestSafeEvaluator_GetLimit_PanicRecovery(t *testing.T) {
	inner := NewEvaluator(panicSource{})
	safe := NewSafeEvaluator(inner)

	// Must not panic — should recover and return (0, false) (fail-open: no limit).
	limit, ok := safe.GetLimit("anything")
	if ok {
		t.Error("GetLimit panic recovery should return ok=false")
	}
	if limit != 0 {
		t.Errorf("GetLimit panic recovery should return limit=0, got %d", limit)
	}
}

func TestSafeEvaluator_CheckLimit_Delegates(t *testing.T) {
	inner := NewEvaluator(mockSource{
		limits: map[string]int64{"max_agents": 10},
	})
	safe := NewSafeEvaluator(inner)

	// Well under limit.
	if result := safe.CheckLimit("max_agents", 5); result != LimitAllowed {
		t.Errorf("CheckLimit(5) = %q, want %q", result, LimitAllowed)
	}

	// At 90% threshold (soft block).
	if result := safe.CheckLimit("max_agents", 9); result != LimitSoftBlock {
		t.Errorf("CheckLimit(9) = %q, want %q", result, LimitSoftBlock)
	}

	// At limit (hard block).
	if result := safe.CheckLimit("max_agents", 10); result != LimitHardBlock {
		t.Errorf("CheckLimit(10) = %q, want %q", result, LimitHardBlock)
	}

	// No limit defined.
	if result := safe.CheckLimit("nonexistent", 100); result != LimitAllowed {
		t.Errorf("CheckLimit(nonexistent) = %q, want %q", result, LimitAllowed)
	}
}

func TestSafeEvaluator_CheckLimit_PanicRecovery(t *testing.T) {
	inner := NewEvaluator(panicSource{})
	safe := NewSafeEvaluator(inner)

	// Must not panic — should recover and return LimitAllowed (fail-open).
	result := safe.CheckLimit("anything", 999)
	if result != LimitAllowed {
		t.Errorf("CheckLimit panic recovery should return LimitAllowed, got %q", result)
	}
}

func TestSafeEvaluator_MeterEnabled_Delegates(t *testing.T) {
	inner := NewEvaluator(mockSource{
		metersEnabled: []string{"meter_a"},
	})
	safe := NewSafeEvaluator(inner)

	if !safe.MeterEnabled("meter_a") {
		t.Error("expected MeterEnabled(meter_a) = true")
	}
	if safe.MeterEnabled("meter_b") {
		t.Error("expected MeterEnabled(meter_b) = false")
	}
}

func TestSafeEvaluator_MeterEnabled_PanicRecovery(t *testing.T) {
	inner := NewEvaluator(panicSource{})
	safe := NewSafeEvaluator(inner)

	// Must not panic — should recover and return false (meters disabled on panic).
	result := safe.MeterEnabled("anything")
	if result {
		t.Error("MeterEnabled panic recovery should return false")
	}
}

func TestSafeEvaluator_NilInner(t *testing.T) {
	// SafeEvaluator wrapping a nil evaluator should not panic.
	safe := NewSafeEvaluator(nil)

	// These should not panic — nil evaluator returns defaults.
	if safe.HasCapability("any") {
		t.Error("nil inner: HasCapability should return false")
	}
	if limit, ok := safe.GetLimit("any"); ok || limit != 0 {
		t.Errorf("nil inner: GetLimit = (%d, %v), want (0, false)", limit, ok)
	}
	if result := safe.CheckLimit("any", 5); result != LimitAllowed {
		t.Errorf("nil inner: CheckLimit = %q, want %q", result, LimitAllowed)
	}
	if safe.MeterEnabled("any") {
		t.Error("nil inner: MeterEnabled should return false")
	}
}
