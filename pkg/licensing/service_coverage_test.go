package licensing

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests add coverage for previously-uncovered pure helpers and simple
// *Service accessors/mutators in service.go. They are white-box (same package)
// so unexported helpers can be exercised directly.

// -----------------------------------------------------------------------------
// Pure helpers (no *Service setup required)
// -----------------------------------------------------------------------------

func TestCoverageUnionFeatures(t *testing.T) {
	t.Run("both_empty", func(t *testing.T) {
		got := unionFeatures(nil, nil)
		assert.Empty(t, got)
	})

	t.Run("first_empty_second_non_empty", func(t *testing.T) {
		got := unionFeatures(nil, []string{"a"})
		assert.Equal(t, []string{"a"}, got)
	})

	t.Run("second_empty_first_non_empty", func(t *testing.T) {
		got := unionFeatures([]string{"a"}, nil)
		assert.Equal(t, []string{"a"}, got)
	})

	t.Run("overlap_dedup", func(t *testing.T) {
		got := unionFeatures([]string{"a", "b"}, []string{"b", "c"})
		assert.Equal(t, []string{"a", "b", "c"}, got)
	})

	t.Run("sorted_output", func(t *testing.T) {
		got := unionFeatures([]string{"c", "a"}, []string{"b"})
		assert.Equal(t, []string{"a", "b", "c"}, got)
		assert.True(t, sliceIsSorted(got), "output must be sorted")
	})

	t.Run("duplicates_within_same_slice", func(t *testing.T) {
		got := unionFeatures([]string{"a", "a"}, nil)
		assert.Equal(t, []string{"a"}, got)
	})

	t.Run("does_not_mutate_input_order_semantics", func(t *testing.T) {
		// Unsorted, overlapping inputs from both sides collapse to one sorted set.
		got := unionFeatures([]string{"z", "a", "m"}, []string{"m", "q"})
		assert.Equal(t, []string{"a", "m", "q", "z"}, got)
	})
}

func sliceIsSorted(s []string) bool {
	for i := 1; i < len(s); i++ {
		if s[i-1] > s[i] {
			return false
		}
	}
	return true
}

func TestCoverageSafeIntFromInt64(t *testing.T) {
	t.Run("normal_passthrough", func(t *testing.T) {
		assert.Equal(t, 100, safeIntFromInt64(100))
	})

	t.Run("zero", func(t *testing.T) {
		assert.Equal(t, 0, safeIntFromInt64(0))
	})

	t.Run("negative_clamps_to_zero", func(t *testing.T) {
		assert.Equal(t, 0, safeIntFromInt64(-1))
		assert.Equal(t, 0, safeIntFromInt64(-9999))
	})

	t.Run("large_negative", func(t *testing.T) {
		assert.Equal(t, 0, safeIntFromInt64(math.MinInt64))
	})

	t.Run("maxint64_clamps_to_maxint", func(t *testing.T) {
		maxInt := int(^uint(0) >> 1)
		assert.Equal(t, maxInt, safeIntFromInt64(math.MaxInt64))
	})

	t.Run("large_in_range_value_on_64bit", func(t *testing.T) {
		// 1 << 40 fits in a 64-bit int.
		assert.Equal(t, 1<<40, safeIntFromInt64(int64(1)<<40))
	})
}

func TestCoverageRemainingDaysCeil(t *testing.T) {
	const day = int64(86400)

	t.Run("expired_returns_zero", func(t *testing.T) {
		now := int64(1_000_000)
		assert.Equal(t, 0, remainingDaysCeil(now-day, now))
	})

	t.Run("negative_delta_returns_zero", func(t *testing.T) {
		now := int64(2_000_000)
		assert.Equal(t, 0, remainingDaysCeil(now-5, now))
	})

	t.Run("exact_boundary_delta_zero", func(t *testing.T) {
		now := int64(3_000_000)
		assert.Equal(t, 0, remainingDaysCeil(now, now))
	})

	t.Run("one_second_rounds_up_to_one_day", func(t *testing.T) {
		now := int64(4_000_000)
		assert.Equal(t, 1, remainingDaysCeil(now+1, now))
	})

	t.Run("partial_day_rounds_up", func(t *testing.T) {
		now := int64(5_000_000)
		// 1.5 days -> ceil -> 2
		assert.Equal(t, 2, remainingDaysCeil(now+(day+day/2), now))
	})

	t.Run("exactly_one_day", func(t *testing.T) {
		now := int64(6_000_000)
		assert.Equal(t, 1, remainingDaysCeil(now+day, now))
	})

	t.Run("exactly_two_days", func(t *testing.T) {
		now := int64(7_000_000)
		assert.Equal(t, 2, remainingDaysCeil(now+2*day, now))
	})
}

// -----------------------------------------------------------------------------
// *Service helpers / accessors
// -----------------------------------------------------------------------------

func TestCoverageEnsureGracePeriodEnd(t *testing.T) {
	t.Run("nil_license_noop", func(t *testing.T) {
		s := NewService()
		s.mu.Lock()
		s.ensureGracePeriodEnd()
		s.mu.Unlock()
		// Still no license, no panic, no grace period set.
		assert.Nil(t, s.CurrentUnsafeForTesting())
	})

	t.Run("nil_GracePeriodEnd_sets_from_ExpiresAt_plus_grace", func(t *testing.T) {
		s := NewService()
		expiresAt := time.Now().Add(24 * time.Hour).Unix()
		lic := &License{Claims: Claims{ExpiresAt: expiresAt}}
		s.SetCurrentForTesting(lic)

		s.mu.Lock()
		s.ensureGracePeriodEnd()
		s.mu.Unlock()

		got := s.CurrentUnsafeForTesting().GracePeriodEnd
		require.NotNil(t, got)
		expected := time.Unix(expiresAt, 0).Add(DefaultGracePeriod)
		assert.True(t, got.Equal(expected), "grace end = expiresAt + DefaultGracePeriod")
	})

	t.Run("already_set_noop", func(t *testing.T) {
		s := NewService()
		preExisting := time.Now().Add(48 * time.Hour)
		lic := &License{
			Claims:         Claims{ExpiresAt: time.Now().Add(24 * time.Hour).Unix()},
			GracePeriodEnd: &preExisting,
		}
		s.SetCurrentForTesting(lic)

		s.mu.Lock()
		s.ensureGracePeriodEnd()
		s.mu.Unlock()

		got := s.CurrentUnsafeForTesting().GracePeriodEnd
		require.NotNil(t, got)
		assert.True(t, got.Equal(preExisting), "pre-existing grace period must be preserved")
	})
}

func TestCoverageSetEvaluator(t *testing.T) {
	t.Run("set_nil", func(t *testing.T) {
		s := NewService()
		s.SetEvaluator(nil)
		assert.Nil(t, s.Evaluator())
	})

	t.Run("set_non_nil_then_get", func(t *testing.T) {
		s := NewService()
		eval := NewEvaluator(NewTokenSource(&Claims{Tier: TierPro}))
		s.SetEvaluator(eval)
		assert.Same(t, eval, s.Evaluator())
	})

	t.Run("overwrite", func(t *testing.T) {
		s := NewService()
		first := NewEvaluator(NewTokenSource(&Claims{Tier: TierPro}))
		second := NewEvaluator(NewTokenSource(&Claims{Tier: TierProPlus}))
		s.SetEvaluator(first)
		s.SetEvaluator(second)
		assert.Same(t, second, s.Evaluator())
	})
}

func TestCoverageSetStateMachine(t *testing.T) {
	t.Run("nil_is_noop_safe", func(t *testing.T) {
		s := NewService()
		// Must not panic and must leave subscription derivation claim-based.
		s.SetStateMachine(nil)
		assert.NotPanics(t, func() {
			_ = s.SubscriptionState()
		})
	})

	t.Run("non_nil_configures_state_machine_hook", func(t *testing.T) {
		// When the state-machine hook is configured and a license with a non-empty
		// SubState is present, SubscriptionState short-circuits to the claim's
		// SubState instead of deriving from expiration/grace logic. We observe
		// the toggle using an expired-but-claimed-active license.
		s := NewService()
		lic := &License{
			Claims: Claims{
				Tier:      TierPro,
				ExpiresAt: time.Now().Add(-365 * 24 * time.Hour).Unix(), // long expired
				SubState:  SubStateActive,
			},
		}
		s.SetCurrentForTesting(lic)

		// Without the hook: expiration derivation wins -> expired.
		assert.Equal(t, string(SubStateExpired), s.SubscriptionState())

		// With the hook: claim SubState is returned verbatim -> active.
		s.SetStateMachine("fake-state-machine")
		assert.Equal(t, string(SubStateActive), s.SubscriptionState())

		// Clearing the hook restores derived behavior -> expired.
		s.SetStateMachine(nil)
		assert.Equal(t, string(SubStateExpired), s.SubscriptionState())
	})
}

func TestCoverageCurrentUnsafeForTesting(t *testing.T) {
	t.Run("nil_when_no_license", func(t *testing.T) {
		s := NewService()
		assert.Nil(t, s.CurrentUnsafeForTesting())
	})

	t.Run("returns_internal_pointer", func(t *testing.T) {
		s := NewService()
		lic := &License{Claims: Claims{Tier: TierPro}}
		s.SetCurrentForTesting(lic)
		// Unsafe accessor returns the exact internal pointer (not a clone).
		assert.Same(t, lic, s.CurrentUnsafeForTesting())
	})
}

func TestCoverageIsValid(t *testing.T) {
	t.Run("nil_license_false", func(t *testing.T) {
		s := NewService()
		assert.False(t, s.IsValid())
	})

	t.Run("active_valid_true", func(t *testing.T) {
		s := NewService()
		s.SetCurrentForTesting(&License{
			Claims: Claims{
				Tier:      TierPro,
				ExpiresAt: time.Now().Add(30 * 24 * time.Hour).Unix(),
			},
		})
		assert.True(t, s.IsValid())
	})

	t.Run("lifetime_valid_true", func(t *testing.T) {
		s := NewService()
		s.SetCurrentForTesting(&License{
			Claims: Claims{Tier: TierLifetime}, // ExpiresAt == 0 -> never expires
		})
		assert.True(t, s.IsValid())
	})

	t.Run("expired_past_grace_false", func(t *testing.T) {
		s := NewService()
		s.SetCurrentForTesting(&License{
			Claims: Claims{
				Tier:      TierPro,
				ExpiresAt: time.Now().Add(-365 * 24 * time.Hour).Unix(),
			},
		})
		assert.False(t, s.IsValid())
	})

	t.Run("suspended_claim_false", func(t *testing.T) {
		s := NewService()
		s.SetCurrentForTesting(&License{
			Claims: Claims{
				Tier:      TierPro,
				ExpiresAt: time.Now().Add(30 * 24 * time.Hour).Unix(),
				SubState:  SubStateSuspended,
			},
		})
		assert.False(t, s.IsValid())
	})
}

func TestCoverageIsLicenseValidationDevMode(t *testing.T) {
	t.Run("unset_is_false", func(t *testing.T) {
		t.Setenv("PULSE_LICENSE_DEV_MODE", "")
		assert.False(t, IsLicenseValidationDevMode())
	})

	t.Run("explicit_false", func(t *testing.T) {
		t.Setenv("PULSE_LICENSE_DEV_MODE", "false")
		assert.False(t, IsLicenseValidationDevMode())
	})

	t.Run("explicit_true", func(t *testing.T) {
		t.Setenv("PULSE_LICENSE_DEV_MODE", "true")
		assert.True(t, IsLicenseValidationDevMode())
	})

	t.Run("true_with_surrounding_whitespace", func(t *testing.T) {
		t.Setenv("PULSE_LICENSE_DEV_MODE", "  true  ")
		assert.True(t, IsLicenseValidationDevMode())
	})

	t.Run("uppercase_true_is_case_insensitive_match", func(t *testing.T) {
		// Uses strings.EqualFold, so "TRUE" also enables dev mode.
		t.Setenv("PULSE_LICENSE_DEV_MODE", "TRUE")
		assert.True(t, IsLicenseValidationDevMode())
	})

	t.Run("non_true_word_false", func(t *testing.T) {
		t.Setenv("PULSE_LICENSE_DEV_MODE", "yes")
		assert.False(t, IsLicenseValidationDevMode())
	})
}
