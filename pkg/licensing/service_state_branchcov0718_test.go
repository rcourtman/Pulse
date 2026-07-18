package licensing

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overflowMockSource is an EntitlementSource whose OverflowGrantedAt can be
// configured. The sibling mockSource (evaluator_test.go) hard-wires
// OverflowGrantedAt to nil, so a dedicated mock is required to exercise the
// "value present" branch of (*Evaluator).OverflowGrantedAt.
type overflowMockSource struct {
	overflow *int64
	subState SubscriptionState
}

func (m overflowMockSource) Capabilities() []string               { return nil }
func (m overflowMockSource) Limits() map[string]int64             { return nil }
func (m overflowMockSource) MetersEnabled() []string              { return nil }
func (m overflowMockSource) PlanVersion() string                  { return "" }
func (m overflowMockSource) SubscriptionState() SubscriptionState { return m.subState }
func (m overflowMockSource) TrialStartedAt() *int64               { return nil }
func (m overflowMockSource) TrialEndsAt() *int64                  { return nil }
func (m overflowMockSource) OverflowGrantedAt() *int64 {
	if m.overflow == nil {
		return nil
	}
	c := *m.overflow
	return &c
}

// TestEvaluatorOverflowGrantedAt0718 covers every branch of
// (*Evaluator).OverflowGrantedAt: nil receiver, non-nil receiver with a nil
// source, source returning nil, and source returning a value (which must be
// handed back as a defensive copy).
func TestEvaluatorOverflowGrantedAt0718(t *testing.T) {
	ts := int64(1700000000)

	t.Run("nil_receiver_returns_nil", func(t *testing.T) {
		var e *Evaluator
		assert.Nil(t, e.OverflowGrantedAt())
	})

	t.Run("nil_source_returns_nil", func(t *testing.T) {
		// White-box construction: receiver non-nil, source nil -> second
		// short-circuit arm.
		e := &Evaluator{source: nil}
		assert.Nil(t, e.OverflowGrantedAt())
	})

	t.Run("source_returns_nil_propagates_nil", func(t *testing.T) {
		e := NewEvaluator(overflowMockSource{overflow: nil})
		assert.Nil(t, e.OverflowGrantedAt())
	})

	t.Run("source_returns_value_returns_distinct_copy", func(t *testing.T) {
		e := NewEvaluator(overflowMockSource{overflow: &ts})
		got := e.OverflowGrantedAt()
		require.NotNil(t, got)
		assert.Equal(t, ts, *got)
		// cloneInt64Ptr must produce a fresh pointer, not alias the input.
		assert.NotSame(t, &ts, got)
	})
}

// TestDatabaseSourceOverflowGrantedAt0718 covers both arms of
// (*DatabaseSource).OverflowGrantedAt. The active-subscription store path is
// used so no lease-token validation runs and the stored timestamp survives
// normalization; the nil and populated cases exercise the two cloneInt64Ptr
// outcomes.
func TestDatabaseSourceOverflowGrantedAt0718(t *testing.T) {
	ts := int64(1700000000)

	t.Run("nil_when_state_has_no_timestamp", func(t *testing.T) {
		store := &mockBillingStore{
			state: &BillingState{
				PlanVersion:       "pro",
				SubscriptionState: SubStateActive,
			},
		}
		source := NewDatabaseSource(store, "org-1", time.Hour)
		assert.Nil(t, source.OverflowGrantedAt())
	})

	t.Run("returns_value_when_state_has_timestamp", func(t *testing.T) {
		store := &mockBillingStore{
			state: &BillingState{
				PlanVersion:       "pro",
				SubscriptionState: SubStateActive,
				OverflowGrantedAt: &ts,
			},
		}
		source := NewDatabaseSource(store, "org-1", time.Hour)
		got := source.OverflowGrantedAt()
		require.NotNil(t, got)
		assert.Equal(t, ts, *got)
		// Must be a defensive copy, not the internal pointer.
		assert.NotSame(t, &ts, got)
	})

	t.Run("returns_distinct_pointers_across_calls", func(t *testing.T) {
		store := &mockBillingStore{
			state: &BillingState{
				PlanVersion:       "pro",
				SubscriptionState: SubStateActive,
				OverflowGrantedAt: &ts,
			},
		}
		source := NewDatabaseSource(store, "org-1", time.Hour)
		first := source.OverflowGrantedAt()
		second := source.OverflowGrantedAt()
		require.NotNil(t, first)
		require.NotNil(t, second)
		assert.Equal(t, ts, *first)
		assert.Equal(t, ts, *second)
		// Each call must yield its own copy (no shared aliasing).
		assert.NotSame(t, first, second)
	})
}

// TestServiceGetLicenseState0718 covers every return path of
// (*Service).GetLicenseState: the no-license/no-evaluator None path, every
// evaluator-driven arm (active/trial/grace/expired via suspended), and every
// JWT-driven arm (active/trial/grace/expired-past-grace/suspended-claim).
func TestServiceGetLicenseState0718(t *testing.T) {
	// Defensive: ensure no env-var bypass influences the derivation.
	t.Setenv("PULSE_DEV", "")
	t.Setenv("PULSE_MOCK_MODE", "")

	t.Run("no_license_no_evaluator_returns_none", func(t *testing.T) {
		s := NewService()
		state, lic := s.GetLicenseState()
		assert.Equal(t, LicenseStateNone, state)
		assert.Nil(t, lic)
	})

	// Hosted / evaluator-driven arm (s.license == nil && s.evaluator != nil).
	t.Run("evaluator_active_returns_active_nil_license", func(t *testing.T) {
		s := NewService()
		s.SetEvaluator(NewEvaluator(mockSource{subState: SubStateActive}))
		state, lic := s.GetLicenseState()
		assert.Equal(t, LicenseStateActive, state)
		assert.Nil(t, lic)
	})

	t.Run("evaluator_trial_returns_active_nil_license", func(t *testing.T) {
		s := NewService()
		s.SetEvaluator(NewEvaluator(mockSource{subState: SubStateTrial}))
		state, lic := s.GetLicenseState()
		assert.Equal(t, LicenseStateActive, state)
		assert.Nil(t, lic)
	})

	t.Run("evaluator_grace_returns_grace_nil_license", func(t *testing.T) {
		s := NewService()
		s.SetEvaluator(NewEvaluator(mockSource{subState: SubStateGrace}))
		state, lic := s.GetLicenseState()
		assert.Equal(t, LicenseStateGracePeriod, state)
		assert.Nil(t, lic)
	})

	t.Run("evaluator_expired_returns_expired_nil_license", func(t *testing.T) {
		s := NewService()
		s.SetEvaluator(NewEvaluator(mockSource{subState: SubStateExpired}))
		state, lic := s.GetLicenseState()
		assert.Equal(t, LicenseStateExpired, state)
		assert.Nil(t, lic)
	})

	t.Run("evaluator_suspended_falls_through_default_to_expired", func(t *testing.T) {
		s := NewService()
		s.SetEvaluator(NewEvaluator(mockSource{subState: SubStateSuspended}))
		state, lic := s.GetLicenseState()
		assert.Equal(t, LicenseStateExpired, state)
		assert.Nil(t, lic)
	})

	// JWT-driven arm (s.license != nil) via currentJWTSubscriptionStateLocked.
	t.Run("jwt_active_no_substate_returns_active_with_clone", func(t *testing.T) {
		s := NewService()
		original := &License{Claims: Claims{
			Tier:      TierPro,
			ExpiresAt: time.Now().Add(30 * 24 * time.Hour).Unix(),
		}}
		s.SetCurrentForTesting(original)
		state, lic := s.GetLicenseState()
		assert.Equal(t, LicenseStateActive, state)
		require.NotNil(t, lic)
		assert.Equal(t, TierPro, lic.Claims.Tier)
		// Returned license must be a clone, not the internal pointer.
		assert.NotSame(t, original, lic)
		assert.Same(t, original, s.CurrentUnsafeForTesting(), "internal pointer must be unchanged")
	})

	t.Run("jwt_trial_claim_returns_active", func(t *testing.T) {
		s := NewService()
		s.SetCurrentForTesting(&License{Claims: Claims{
			Tier:      TierPro,
			ExpiresAt: time.Now().Add(30 * 24 * time.Hour).Unix(),
			SubState:  SubStateTrial,
		}})
		state, lic := s.GetLicenseState()
		assert.Equal(t, LicenseStateActive, state)
		require.NotNil(t, lic)
	})

	t.Run("jwt_expired_within_grace_returns_grace", func(t *testing.T) {
		s := NewService()
		s.SetCurrentForTesting(&License{Claims: Claims{
			Tier:      TierPro,
			ExpiresAt: time.Now().Add(-24 * time.Hour).Unix(), // expired yesterday, within 7-day grace
		}})
		state, lic := s.GetLicenseState()
		assert.Equal(t, LicenseStateGracePeriod, state)
		require.NotNil(t, lic)
	})

	t.Run("jwt_expired_past_grace_returns_expired", func(t *testing.T) {
		s := NewService()
		s.SetCurrentForTesting(&License{Claims: Claims{
			Tier:      TierPro,
			ExpiresAt: time.Now().Add(-365 * 24 * time.Hour).Unix(), // long past grace
		}})
		state, lic := s.GetLicenseState()
		assert.Equal(t, LicenseStateExpired, state)
		require.NotNil(t, lic)
	})

	t.Run("jwt_suspended_claim_returns_expired", func(t *testing.T) {
		s := NewService()
		s.SetCurrentForTesting(&License{Claims: Claims{
			Tier:      TierPro,
			ExpiresAt: time.Now().Add(30 * 24 * time.Hour).Unix(),
			SubState:  SubStateSuspended,
		}})
		state, lic := s.GetLicenseState()
		assert.Equal(t, LicenseStateExpired, state)
		require.NotNil(t, lic)
	})

	t.Run("jwt_canceled_claim_returns_expired", func(t *testing.T) {
		s := NewService()
		s.SetCurrentForTesting(&License{Claims: Claims{
			Tier:      TierPro,
			ExpiresAt: time.Now().Add(30 * 24 * time.Hour).Unix(),
			SubState:  SubStateCanceled,
		}})
		state, lic := s.GetLicenseState()
		assert.Equal(t, LicenseStateExpired, state)
		require.NotNil(t, lic)
	})
}

// TestServiceGetLicenseStateString0718 covers both arms of the hasFeatures
// computation in (*Service).GetLicenseStateString: true for active/grace,
// false for none/expired.
func TestServiceGetLicenseStateString0718(t *testing.T) {
	t.Setenv("PULSE_DEV", "")
	t.Setenv("PULSE_MOCK_MODE", "")

	tests := []struct {
		name      string
		setup     func(*Service)
		wantStr   string
		wantAvail bool
	}{
		{
			name:      "none_returns_false",
			setup:     func(s *Service) {},
			wantStr:   string(LicenseStateNone),
			wantAvail: false,
		},
		{
			name: "active_returns_true",
			setup: func(s *Service) {
				s.SetEvaluator(NewEvaluator(mockSource{subState: SubStateActive}))
			},
			wantStr:   string(LicenseStateActive),
			wantAvail: true,
		},
		{
			name: "grace_returns_true",
			setup: func(s *Service) {
				s.SetEvaluator(NewEvaluator(mockSource{subState: SubStateGrace}))
			},
			wantStr:   string(LicenseStateGracePeriod),
			wantAvail: true,
		},
		{
			name: "expired_returns_false",
			setup: func(s *Service) {
				s.SetEvaluator(NewEvaluator(mockSource{subState: SubStateExpired}))
			},
			wantStr:   string(LicenseStateExpired),
			wantAvail: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			s := NewService()
			tt.setup(s)
			gotStr, gotAvail := s.GetLicenseStateString()
			assert.Equal(t, tt.wantStr, gotStr)
			assert.Equal(t, tt.wantAvail, gotAvail)
		})
	}
}

// TestServiceRequireFeature0718 covers both arms of (*Service).RequireFeature:
// nil when the feature is genuinely entitled via an active license, and a
// wrapped ErrFeatureNotIncluded when it is not. RBAC is a Pro-only feature
// (not free), so it deterministically distinguishes the two arms.
func TestServiceRequireFeature0718(t *testing.T) {
	// Force real entitlement evaluation: no dev/demo env bypass.
	t.Setenv("PULSE_DEV", "")
	t.Setenv("PULSE_MOCK_MODE", "")
	t.Setenv("PULSE_LICENSE_DEV_MODE", "")

	// Precondition: RBAC is a Pro capability, absent from the free tier, so
	// the only path to a nil error is an active license claim.
	require.False(t, TierHasFeature(TierFree, FeatureRBAC),
		"test fixture requires RBAC to be a non-free feature")

	t.Run("licensed_feature_returns_nil", func(t *testing.T) {
		s := NewService()
		s.SetCurrentForTesting(&License{Claims: Claims{
			Tier:         TierPro,
			ExpiresAt:    0, // lifetime -> IsExpired=false -> SubStateActive
			Capabilities: []string{FeatureRBAC},
		}})
		assert.NoError(t, s.RequireFeature(FeatureRBAC))
	})

	t.Run("unlicensed_feature_returns_ErrFeatureNotIncluded", func(t *testing.T) {
		s := NewService() // no license, no evaluator -> free-tier fallback

		err := s.RequireFeature(FeatureRBAC)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrFeatureNotIncluded),
			"want errors.Is ErrFeatureNotIncluded, got %v", err)
		// Message format is "<display> requires Pulse <minTier> or above";
		// RBAC's min tier is Pro, so the suffix is a stable behavioral signal.
		assert.Contains(t, err.Error(), "Pro or above",
			"error message should name the required tier")
	})

	t.Run("free_tier_feature_returns_nil_without_license", func(t *testing.T) {
		s := NewService() // no license, no evaluator
		// FeatureAIPatrol is in the free tier, so it is granted even without
		// any license, exercising the no-error arm via the free fallback.
		require.True(t, TierHasFeature(TierFree, FeatureAIPatrol))
		assert.NoError(t, s.RequireFeature(FeatureAIPatrol))
	})
}
