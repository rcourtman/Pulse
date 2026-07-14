package licensing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// These tests add coverage for previously-uncovered functions in
// activation_types.go. White-box (same package) so unexported helpers can be
// exercised directly.

func TestCoverageParseExpiresAt(t *testing.T) {
	t.Run("empty_string_returns_zero", func(t *testing.T) {
		g := GrantEnvelope{ExpiresAt: ""}
		assert.Equal(t, int64(0), g.ParseExpiresAt())
	})

	t.Run("zero_value_envelope_returns_zero", func(t *testing.T) {
		var g GrantEnvelope
		assert.Equal(t, int64(0), g.ParseExpiresAt())
	})

	t.Run("valid_rfc3339_returns_unix_ts", func(t *testing.T) {
		ts := "2025-01-15T10:30:00Z"
		g := GrantEnvelope{ExpiresAt: ts}
		expected, err := time.Parse(time.RFC3339, ts)
		assert.NoError(t, err)
		assert.Equal(t, expected.Unix(), g.ParseExpiresAt())
	})

	t.Run("valid_rfc3339_with_offset_returns_unix_ts", func(t *testing.T) {
		ts := "2025-06-30T23:59:59-07:00"
		g := GrantEnvelope{ExpiresAt: ts}
		expected, err := time.Parse(time.RFC3339, ts)
		assert.NoError(t, err)
		assert.Equal(t, expected.Unix(), g.ParseExpiresAt())
	})

	t.Run("invalid_format_returns_zero", func(t *testing.T) {
		g := GrantEnvelope{ExpiresAt: "not-a-date"}
		assert.Equal(t, int64(0), g.ParseExpiresAt())
	})

	t.Run("garbage_numeric_returns_zero", func(t *testing.T) {
		// A bare unix epoch number is not RFC3339 and must not parse.
		g := GrantEnvelope{ExpiresAt: "0"}
		assert.Equal(t, int64(0), g.ParseExpiresAt())
	})

	t.Run("value_receiver_does_not_panic", func(t *testing.T) {
		g := GrantEnvelope{ExpiresAt: "2025-01-15T10:30:00Z"}
		assert.NotPanics(t, func() {
			_ = g.ParseExpiresAt()
		})
	})
}

func TestCoverageGrantClaimsUseUncappedCoreMonitoring(t *testing.T) {
	t.Run("nil_claims_false", func(t *testing.T) {
		assert.False(t, grantClaimsUseUncappedCoreMonitoring(nil))
	})

	t.Run("uncapped_tier_pro_true", func(t *testing.T) {
		assert.True(t, grantClaimsUseUncappedCoreMonitoring(&GrantClaims{Tier: string(TierPro)}))
	})

	t.Run("uncapped_tier_free_true", func(t *testing.T) {
		assert.True(t, grantClaimsUseUncappedCoreMonitoring(&GrantClaims{Tier: string(TierFree)}))
	})

	t.Run("uncapped_tier_relay_true", func(t *testing.T) {
		assert.True(t, grantClaimsUseUncappedCoreMonitoring(&GrantClaims{Tier: string(TierRelay)}))
	})

	t.Run("uncapped_tier_lifetime_true", func(t *testing.T) {
		assert.True(t, grantClaimsUseUncappedCoreMonitoring(&GrantClaims{Tier: string(TierLifetime)}))
	})

	t.Run("uncapped_tier_business_true", func(t *testing.T) {
		assert.True(t, grantClaimsUseUncappedCoreMonitoring(&GrantClaims{Tier: string(TierBusiness)}))
	})

	t.Run("capped_tier_cloud_false", func(t *testing.T) {
		assert.False(t, grantClaimsUseUncappedCoreMonitoring(&GrantClaims{Tier: string(TierCloud)}))
	})

	t.Run("capped_tier_msp_false", func(t *testing.T) {
		assert.False(t, grantClaimsUseUncappedCoreMonitoring(&GrantClaims{Tier: string(TierMSP)}))
	})

	t.Run("capped_tier_enterprise_false", func(t *testing.T) {
		assert.False(t, grantClaimsUseUncappedCoreMonitoring(&GrantClaims{Tier: string(TierEnterprise)}))
	})

	t.Run("empty_tier_false", func(t *testing.T) {
		assert.False(t, grantClaimsUseUncappedCoreMonitoring(&GrantClaims{Tier: ""}))
	})

	t.Run("unknown_tier_false", func(t *testing.T) {
		assert.False(t, grantClaimsUseUncappedCoreMonitoring(&GrantClaims{Tier: "does-not-exist"}))
	})

	t.Run("case_insensitive_uppercase_true", func(t *testing.T) {
		// IsSelfHostedCoreMonitoringUncappedTier lowercases the input.
		assert.True(t, grantClaimsUseUncappedCoreMonitoring(&GrantClaims{Tier: "PRO"}))
	})

	t.Run("case_insensitive_mixed_with_whitespace_true", func(t *testing.T) {
		assert.True(t, grantClaimsUseUncappedCoreMonitoring(&GrantClaims{Tier: "  Pro_Plus  "}))
	})
}
