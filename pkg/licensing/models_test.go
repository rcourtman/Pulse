package licensing

import (
	"encoding/json"
	"testing"
	"time"
)

func TestClaims_EffectiveCapabilities(t *testing.T) {
	tests := []struct {
		name     string
		claims   Claims
		expected []string
	}{
		{
			name: "explicit_capabilities_returns_them",
			claims: Claims{
				Capabilities: []string{"feature_a", "feature_b"},
				Tier:         TierPro,
			},
			expected: []string{"feature_a", "feature_b"},
		},
		{
			name: "nil_capabilities_derives_from_tier",
			claims: Claims{
				Capabilities: nil,
				Tier:         TierPro,
				Features:     []string{"extra_feature"},
			},
			expected: DeriveCapabilitiesFromTier(TierPro, []string{"extra_feature"}),
		},
		{
			name: "empty_capabilities_derives_from_tier",
			claims: Claims{
				Capabilities: []string{},
				Tier:         TierProAnnual,
			},
			expected: DeriveCapabilitiesFromTier(TierProAnnual, nil),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := tt.claims.EffectiveCapabilities()
			if len(got) != len(tt.expected) {
				t.Fatalf("EffectiveCapabilities() returned %d capabilities, want %d", len(got), len(tt.expected))
			}
			for i, cap := range got {
				if i >= len(tt.expected) {
					break
				}
				if cap != tt.expected[i] {
					t.Fatalf("EffectiveCapabilities()[%d] = %q, want %q", i, cap, tt.expected[i])
				}
			}
		})
	}
}

func TestClaims_EffectiveLimits(t *testing.T) {
	tests := []struct {
		name     string
		claims   Claims
		expected map[string]int64
	}{
		{
			name: "explicit_limits_returns_them",
			claims: Claims{
				Limits:    map[string]int64{"max_agents": 100, "max_guests": 500},
				MaxAgents: 50,
				MaxGuests: 200,
			},
			expected: map[string]int64{"max_agents": 100, "max_guests": 500},
		},
		{
			name: "nil_limits_derives_from_legacy_fields",
			claims: Claims{
				Limits:    nil,
				MaxAgents: 25,
				MaxGuests: 100,
			},
			expected: map[string]int64{"max_agents": 25, "max_guests": 100},
		},
		{
			name: "zero_max_agents_ignored",
			claims: Claims{
				Limits:    nil,
				MaxAgents: 0,
				MaxGuests: 100,
			},
			expected: map[string]int64{"max_guests": 100},
		},
		{
			name:     "no_limits_returns_empty",
			claims:   Claims{},
			expected: map[string]int64{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := tt.claims.EffectiveLimits()
			if len(got) != len(tt.expected) {
				t.Fatalf("EffectiveLimits() returned %d limits, want %d", len(got), len(tt.expected))
			}
			for key, val := range tt.expected {
				if got[key] != val {
					t.Fatalf("EffectiveLimits()[%q] = %d, want %d", key, got[key], val)
				}
			}
		})
	}
}

func TestClaims_EntitlementMetersEnabled(t *testing.T) {
	tests := []struct {
		name     string
		claims   *Claims
		expected []string
	}{
		{
			name:     "nil_claims_returns_nil",
			claims:   nil,
			expected: nil,
		},
		{
			name: "returns_meters_enabled",
			claims: &Claims{
				MetersEnabled: []string{"meter_a", "meter_b"},
			},
			expected: []string{"meter_a", "meter_b"},
		},
		{
			name: "empty_meters_returns_empty",
			claims: &Claims{
				MetersEnabled: []string{},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := tt.claims.EntitlementMetersEnabled()
			if len(got) != len(tt.expected) {
				t.Fatalf("EntitlementMetersEnabled() returned %d, want %d", len(got), len(tt.expected))
			}
		})
	}
}

func TestClaims_EntitlementPlanVersion(t *testing.T) {
	tests := []struct {
		name     string
		claims   *Claims
		expected string
	}{
		{
			name:     "nil_claims_returns_empty",
			claims:   nil,
			expected: "",
		},
		{
			name: "returns_plan_version",
			claims: &Claims{
				PlanVersion: "v2",
			},
			expected: "v2",
		},
		{
			name: "empty_plan_version_returns_empty",
			claims: &Claims{
				PlanVersion: "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := tt.claims.EntitlementPlanVersion()
			if got != tt.expected {
				t.Fatalf("EntitlementPlanVersion() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestClaims_EntitlementSubscriptionState(t *testing.T) {
	tests := []struct {
		name     string
		claims   *Claims
		expected SubscriptionState
	}{
		{
			name:     "nil_claims_returns_active",
			claims:   nil,
			expected: SubStateActive,
		},
		{
			name: "returns_subscription_state",
			claims: &Claims{
				SubState: SubStateGrace,
			},
			expected: SubStateGrace,
		},
		{
			name: "empty_substate_returns_active",
			claims: &Claims{
				SubState: "",
			},
			expected: SubStateActive,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := tt.claims.EntitlementSubscriptionState()
			if got != tt.expected {
				t.Fatalf("EntitlementSubscriptionState() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestLicense_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		license  License
		expected bool
	}{
		{
			name: "lifetime_license_not_expired",
			license: License{
				Claims: Claims{
					ExpiresAt: 0,
				},
			},
			expected: false,
		},
		{
			name: "future_expiry_not_expired",
			license: License{
				Claims: Claims{
					ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
				},
			},
			expected: false,
		},
		{
			name: "past_expiry_is_expired",
			license: License{
				Claims: Claims{
					ExpiresAt: time.Now().Add(-24 * time.Hour).Unix(),
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := tt.license.IsExpired()
			if got != tt.expected {
				t.Fatalf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLicense_IsLifetime(t *testing.T) {
	tests := []struct {
		name     string
		license  License
		expected bool
	}{
		{
			name: "zero_expires_at_is_lifetime",
			license: License{
				Claims: Claims{
					ExpiresAt: 0,
				},
			},
			expected: true,
		},
		{
			name: "lifetime_tier_is_lifetime",
			license: License{
				Claims: Claims{
					ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
					Tier:      TierLifetime,
				},
			},
			expected: true,
		},
		{
			name: "non_lifetime_not_lifetime",
			license: License{
				Claims: Claims{
					ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
					Tier:      TierPro,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := tt.license.IsLifetime()
			if got != tt.expected {
				t.Fatalf("IsLifetime() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLicense_DaysRemaining(t *testing.T) {
	tests := []struct {
		name     string
		license  License
		expected int
		cmp      func(got, want int) bool
	}{
		{
			name: "lifetime_returns_minus_one",
			license: License{
				Claims: Claims{
					ExpiresAt: 0,
				},
			},
			expected: -1,
			cmp:      func(got, want int) bool { return got == want },
		},
		{
			name: "future_returns_positive_days",
			license: License{
				Claims: Claims{
					ExpiresAt: time.Now().Add(10 * 24 * time.Hour).Unix(),
				},
			},
			expected: 10,
			cmp:      func(got, want int) bool { return got >= 9 && got <= 10 },
		},
		{
			name: "past_returns_zero",
			license: License{
				Claims: Claims{
					ExpiresAt: time.Now().Add(-10 * time.Hour).Unix(),
				},
			},
			expected: 0,
			cmp:      func(got, want int) bool { return got == want },
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := tt.license.DaysRemaining()
			if !tt.cmp(got, tt.expected) {
				t.Fatalf("DaysRemaining() = %d, want approximately %d", got, tt.expected)
			}
		})
	}
}

func TestLicense_ExpiresAt(t *testing.T) {
	tests := []struct {
		name      string
		license   License
		expectNil bool
	}{
		{
			name: "lifetime_returns_nil",
			license: License{
				Claims: Claims{
					ExpiresAt: 0,
				},
			},
			expectNil: true,
		},
		{
			name: "non_lifetime_returns_time",
			license: License{
				Claims: Claims{
					ExpiresAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC).Unix(),
				},
			},
			expectNil: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := tt.license.ExpiresAt()
			if tt.expectNil && got != nil {
				t.Fatalf("ExpiresAt() = %v, want nil", got)
			}
			if !tt.expectNil && got == nil {
				t.Fatalf("ExpiresAt() = nil, want non-nil")
			}
		})
	}
}

func TestLicense_HasFeature(t *testing.T) {
	license := License{
		Claims: Claims{
			Capabilities: []string{"feature_a", "feature_b"},
			Tier:         TierPro,
		},
	}

	tests := []struct {
		name     string
		feature  string
		expected bool
	}{
		{
			name:     "has_feature",
			feature:  "feature_a",
			expected: true,
		},
		{
			name:     "missing_feature",
			feature:  "feature_c",
			expected: false,
		},
		{
			name:     "empty_feature",
			feature:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := license.HasFeature(tt.feature)
			if got != tt.expected {
				t.Fatalf("HasFeature(%q) = %v, want %v", tt.feature, got, tt.expected)
			}
		})
	}
}

func TestLicense_AllFeatures(t *testing.T) {
	license := License{
		Claims: Claims{
			Capabilities: []string{"zebra", "alpha", "middle"},
			Tier:         TierPro,
		},
	}

	got := license.AllFeatures()
	if len(got) != 3 {
		t.Fatalf("AllFeatures() returned %d features, want 3", len(got))
	}

	if got[0] != "alpha" || got[1] != "middle" || got[2] != "zebra" {
		t.Fatalf("AllFeatures() not sorted: %v", got)
	}
}

func TestClaims_UnmarshalJSON_Migration(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantMax int
	}{
		{
			name:    "new_key_only",
			json:    `{"lid":"x","email":"a@b","tier":"pro","iat":1,"max_agents":10}`,
			wantMax: 10,
		},
		{
			name:    "legacy_key_only",
			json:    `{"lid":"x","email":"a@b","tier":"pro","iat":1,"max_nodes":5}`,
			wantMax: 5,
		},
		{
			name:    "both_keys_prefer_new",
			json:    `{"lid":"x","email":"a@b","tier":"pro","iat":1,"max_agents":15,"max_nodes":5}`,
			wantMax: 15,
		},
		{
			name:    "new_key_zero_ignores_legacy",
			json:    `{"lid":"x","email":"a@b","tier":"pro","iat":1,"max_agents":0,"max_nodes":5}`,
			wantMax: 0,
		},
		{
			name:    "neither_key",
			json:    `{"lid":"x","email":"a@b","tier":"pro","iat":1}`,
			wantMax: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Claims
			if err := json.Unmarshal([]byte(tt.json), &c); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if c.MaxAgents != tt.wantMax {
				t.Fatalf("MaxAgents = %d, want %d", c.MaxAgents, tt.wantMax)
			}
		})
	}
}
