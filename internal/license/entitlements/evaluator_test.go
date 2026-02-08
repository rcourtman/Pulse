package entitlements_test

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	. "github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
)

type mockSource struct {
	capabilities      []string
	limits            map[string]int64
	metersEnabled     []string
	planVersion       string
	subscriptionState license.SubscriptionState
}

func (m mockSource) Capabilities() []string {
	return m.capabilities
}

func (m mockSource) Limits() map[string]int64 {
	return m.limits
}

func (m mockSource) MetersEnabled() []string {
	return m.metersEnabled
}

func (m mockSource) PlanVersion() string {
	return m.planVersion
}

func (m mockSource) SubscriptionState() license.SubscriptionState {
	return m.subscriptionState
}

func TestEvaluatorHasCapability(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		e := NewEvaluator(mockSource{capabilities: []string{"rbac", "relay"}})
		if !e.HasCapability("rbac") {
			t.Fatal("expected capability to be found")
		}
	})

	t.Run("not found", func(t *testing.T) {
		e := NewEvaluator(mockSource{capabilities: []string{"rbac", "relay"}})
		if e.HasCapability("nonexistent") {
			t.Fatal("expected capability to be absent")
		}
	})

	t.Run("empty capabilities", func(t *testing.T) {
		e := NewEvaluator(mockSource{capabilities: []string{}})
		if e.HasCapability("rbac") {
			t.Fatal("expected capability to be absent")
		}
	})
}

func TestEvaluatorHasCapabilityWithAlias(t *testing.T) {
	originalAlias, hadOriginalAlias := LegacyAliases["old_relay"]
	LegacyAliases["old_relay"] = "relay"
	t.Cleanup(func() {
		if hadOriginalAlias {
			LegacyAliases["old_relay"] = originalAlias
			return
		}
		delete(LegacyAliases, "old_relay")
	})

	e := NewEvaluator(mockSource{capabilities: []string{"relay"}})

	if !e.HasCapability("old_relay") {
		t.Fatal("expected aliased capability to be found")
	}

	if !e.HasCapability("relay") {
		t.Fatal("expected direct capability lookup to still work")
	}
}

func TestEvaluatorHasCapabilityDeprecated(t *testing.T) {
	originalDep, hadOriginalDep := DeprecatedCapabilities["relay"]
	DeprecatedCapabilities["relay"] = DeprecatedCapability{
		ReplacementKey: "remote_access",
		SunsetAt:       time.Now().AddDate(1, 0, 0),
	}
	t.Cleanup(func() {
		if hadOriginalDep {
			DeprecatedCapabilities["relay"] = originalDep
			return
		}
		delete(DeprecatedCapabilities, "relay")
	})

	e := NewEvaluator(mockSource{capabilities: []string{"relay"}})
	if !e.HasCapability("relay") {
		t.Fatal("expected deprecated capability to still be found")
	}
}

func TestResolveAlias(t *testing.T) {
	relayAlias, hadRelayAlias := LegacyAliases["relay"]
	if hadRelayAlias {
		delete(LegacyAliases, "relay")
	}

	oldRelayAlias, hadOldRelayAlias := LegacyAliases["old_relay"]
	LegacyAliases["old_relay"] = "relay"

	t.Cleanup(func() {
		if hadRelayAlias {
			LegacyAliases["relay"] = relayAlias
		}
		if hadOldRelayAlias {
			LegacyAliases["old_relay"] = oldRelayAlias
			return
		}
		delete(LegacyAliases, "old_relay")
	})

	if got := ResolveAlias("relay"); got != "relay" {
		t.Fatalf("expected passthrough alias resolution, got %q", got)
	}

	if got := ResolveAlias("old_relay"); got != "relay" {
		t.Fatalf("expected alias to resolve to relay, got %q", got)
	}
}

func TestIsDeprecated(t *testing.T) {
	originalDep, hadOriginalDep := DeprecatedCapabilities["relay"]
	if hadOriginalDep {
		delete(DeprecatedCapabilities, "relay")
	}

	expected := DeprecatedCapability{
		ReplacementKey: "remote_access",
		SunsetAt:       time.Now().AddDate(2, 0, 0),
	}

	t.Cleanup(func() {
		if hadOriginalDep {
			DeprecatedCapabilities["relay"] = originalDep
			return
		}
		delete(DeprecatedCapabilities, "relay")
	})

	if _, ok := IsDeprecated("relay"); ok {
		t.Fatal("expected relay to not be deprecated")
	}

	DeprecatedCapabilities["relay"] = expected

	got, ok := IsDeprecated("relay")
	if !ok {
		t.Fatal("expected relay to be deprecated")
	}
	if got.ReplacementKey != expected.ReplacementKey {
		t.Fatalf("expected replacement key %q, got %q", expected.ReplacementKey, got.ReplacementKey)
	}
	if !got.SunsetAt.Equal(expected.SunsetAt) {
		t.Fatalf("expected sunset %s, got %s", expected.SunsetAt, got.SunsetAt)
	}
}

func TestEvaluatorGetLimit(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		e := NewEvaluator(mockSource{limits: map[string]int64{"max_nodes": 50}})
		limit, ok := e.GetLimit("max_nodes")
		if !ok || limit != 50 {
			t.Fatalf("expected (50, true), got (%d, %t)", limit, ok)
		}
	})

	t.Run("not exists", func(t *testing.T) {
		e := NewEvaluator(mockSource{limits: map[string]int64{"max_nodes": 50}})
		limit, ok := e.GetLimit("max_guests")
		if ok || limit != 0 {
			t.Fatalf("expected (0, false), got (%d, %t)", limit, ok)
		}
	})

	t.Run("nil limits", func(t *testing.T) {
		e := NewEvaluator(mockSource{limits: map[string]int64{}})
		limit, ok := e.GetLimit("max_nodes")
		if ok || limit != 0 {
			t.Fatalf("expected (0, false), got (%d, %t)", limit, ok)
		}
	})
}

func TestEvaluatorCheckLimit(t *testing.T) {
	t.Run("well under limit", func(t *testing.T) {
		e := NewEvaluator(mockSource{limits: map[string]int64{"max_nodes": 100}})
		if got := e.CheckLimit("max_nodes", 50); got != license.LimitAllowed {
			t.Fatalf("expected %q, got %q", license.LimitAllowed, got)
		}
	})

	t.Run("at soft threshold", func(t *testing.T) {
		e := NewEvaluator(mockSource{limits: map[string]int64{"max_nodes": 100}})
		if got := e.CheckLimit("max_nodes", 90); got != license.LimitSoftBlock {
			t.Fatalf("expected %q, got %q", license.LimitSoftBlock, got)
		}
	})

	t.Run("above soft below hard", func(t *testing.T) {
		e := NewEvaluator(mockSource{limits: map[string]int64{"max_nodes": 100}})
		if got := e.CheckLimit("max_nodes", 95); got != license.LimitSoftBlock {
			t.Fatalf("expected %q, got %q", license.LimitSoftBlock, got)
		}
	})

	t.Run("at hard limit", func(t *testing.T) {
		e := NewEvaluator(mockSource{limits: map[string]int64{"max_nodes": 100}})
		if got := e.CheckLimit("max_nodes", 100); got != license.LimitHardBlock {
			t.Fatalf("expected %q, got %q", license.LimitHardBlock, got)
		}
	})

	t.Run("over hard limit", func(t *testing.T) {
		e := NewEvaluator(mockSource{limits: map[string]int64{"max_nodes": 100}})
		if got := e.CheckLimit("max_nodes", 110); got != license.LimitHardBlock {
			t.Fatalf("expected %q, got %q", license.LimitHardBlock, got)
		}
	})

	t.Run("no limit defined", func(t *testing.T) {
		e := NewEvaluator(mockSource{limits: map[string]int64{}})
		if got := e.CheckLimit("max_nodes", 1000); got != license.LimitAllowed {
			t.Fatalf("expected %q, got %q", license.LimitAllowed, got)
		}
	})

	t.Run("zero limit means unlimited", func(t *testing.T) {
		e := NewEvaluator(mockSource{limits: map[string]int64{"max_nodes": 0}})
		if got := e.CheckLimit("max_nodes", 10_000); got != license.LimitAllowed {
			t.Fatalf("expected %q, got %q", license.LimitAllowed, got)
		}
	})
}

func TestEvaluatorMeterEnabled(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		e := NewEvaluator(mockSource{metersEnabled: []string{"active_agents", "relay_bytes"}})
		if !e.MeterEnabled("active_agents") {
			t.Fatal("expected meter to be enabled")
		}
	})

	t.Run("not enabled", func(t *testing.T) {
		e := NewEvaluator(mockSource{metersEnabled: []string{"active_agents", "relay_bytes"}})
		if e.MeterEnabled("nonexistent") {
			t.Fatal("expected meter to be disabled")
		}
	})
}

func TestTokenSourceLegacyDerivation(t *testing.T) {
	t.Run("legacy claims", func(t *testing.T) {
		claims := &license.Claims{
			Tier:     license.TierPro,
			MaxNodes: 25,
		}
		source := NewTokenSource(claims)

		expectedCaps := append([]string(nil), license.TierFeatures[license.TierPro]...)
		sort.Strings(expectedCaps)
		if !reflect.DeepEqual(source.Capabilities(), expectedCaps) {
			t.Fatalf("expected capabilities %v, got %v", expectedCaps, source.Capabilities())
		}

		limits := source.Limits()
		if got, ok := limits["max_nodes"]; !ok || got != 25 {
			t.Fatalf("expected max_nodes limit 25, got (%d, %t)", got, ok)
		}
	})

	t.Run("explicit claims", func(t *testing.T) {
		claims := &license.Claims{
			Tier:         license.TierPro,
			Capabilities: []string{"custom"},
			Limits:       map[string]int64{"custom": 99},
		}
		source := NewTokenSource(claims)

		if !reflect.DeepEqual(source.Capabilities(), []string{"custom"}) {
			t.Fatalf("expected explicit capabilities, got %v", source.Capabilities())
		}

		if !reflect.DeepEqual(source.Limits(), map[string]int64{"custom": 99}) {
			t.Fatalf("expected explicit limits, got %v", source.Limits())
		}
	})

	t.Run("default subscription state", func(t *testing.T) {
		claims := &license.Claims{}
		source := NewTokenSource(claims)
		if got := source.SubscriptionState(); got != license.SubStateActive {
			t.Fatalf("expected %q, got %q", license.SubStateActive, got)
		}
	})
}

func TestTokenSourceMetersEnabled(t *testing.T) {
	t.Run("meters set", func(t *testing.T) {
		claims := &license.Claims{
			MetersEnabled: []string{"active_agents"},
		}
		source := NewTokenSource(claims)
		if !reflect.DeepEqual(source.MetersEnabled(), []string{"active_agents"}) {
			t.Fatalf("expected [active_agents], got %v", source.MetersEnabled())
		}
	})

	t.Run("meters nil", func(t *testing.T) {
		claims := &license.Claims{}
		source := NewTokenSource(claims)
		if len(source.MetersEnabled()) != 0 {
			t.Fatalf("expected empty meters, got %v", source.MetersEnabled())
		}
	})
}

func TestEvaluatorSubscriptionState(t *testing.T) {
	e := NewEvaluator(mockSource{subscriptionState: license.SubStateGrace})
	if got := e.SubscriptionState(); got != license.SubStateGrace {
		t.Fatalf("expected %q, got %q", license.SubStateGrace, got)
	}
}
