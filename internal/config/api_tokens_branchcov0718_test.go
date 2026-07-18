package config

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestActiveAPITokenHashes_BranchCov0718 exercises every branch of
// (*Config).ActiveAPITokenHashes: empty config, single token, multiple tokens
// preserving order, records with empty hashes being filtered, all-empty result,
// and expired tokens still being returned (the method filters only on Hash != "").
func TestActiveAPITokenHashes_BranchCov0718(t *testing.T) {
	pastExpiry := time.Now().UTC().Add(-time.Hour)

	tests := []struct {
		name   string
		config *Config
		want   []string
	}{
		{
			name:   "nil token slice returns non-nil empty result",
			config: &Config{},
			want:   []string{},
		},
		{
			name: "single token hash returned",
			config: &Config{
				APITokens: []APITokenRecord{{ID: "t1", Hash: "hash-1"}},
			},
			want: []string{"hash-1"},
		},
		{
			name: "multiple tokens preserve insertion order",
			config: &Config{
				APITokens: []APITokenRecord{
					{ID: "t1", Hash: "hash-1"},
					{ID: "t2", Hash: "hash-2"},
					{ID: "t3", Hash: "hash-3"},
				},
			},
			want: []string{"hash-1", "hash-2", "hash-3"},
		},
		{
			name: "empty-hash records are filtered out while non-empty are kept in order",
			config: &Config{
				APITokens: []APITokenRecord{
					{ID: "t1", Hash: ""},
					{ID: "t2", Hash: "hash-2"},
					{ID: "t3", Hash: ""},
					{ID: "t4", Hash: "hash-4"},
				},
			},
			want: []string{"hash-2", "hash-4"},
		},
		{
			name: "all empty hashes yields empty non-nil result",
			config: &Config{
				APITokens: []APITokenRecord{
					{ID: "t1", Hash: ""},
					{ID: "t2", Hash: ""},
				},
			},
			want: []string{},
		},
		{
			name: "expired tokens are still returned (method does not filter on expiry)",
			config: &Config{
				APITokens: []APITokenRecord{
					{ID: "t1", Hash: "hash-1", ExpiresAt: &pastExpiry},
					{ID: "t2", Hash: "hash-2"},
				},
			},
			want: []string{"hash-1", "hash-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ActiveAPITokenHashes()
			assert.NotNil(t, got, "result should be non-nil even when empty (make-allocated)")
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestHasAPITokenHash_BranchCov0718 covers (*Config).HasAPITokenHash across
// present, absent, empty-query, and the empty-query-matches-empty-hash-record
// edge case (a direct == comparison with no empty-string guard).
func TestHasAPITokenHash_BranchCov0718(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		hash   string
		want   bool
	}{
		{
			name:   "empty config reports absent",
			config: &Config{},
			hash:   "hash-1",
			want:   false,
		},
		{
			name: "matching hash at first position",
			config: &Config{
				APITokens: []APITokenRecord{{Hash: "hash-1"}, {Hash: "hash-2"}},
			},
			hash: "hash-1",
			want: true,
		},
		{
			name: "matching hash at non-first position",
			config: &Config{
				APITokens: []APITokenRecord{{Hash: "hash-1"}, {Hash: "hash-2"}},
			},
			hash: "hash-2",
			want: true,
		},
		{
			name: "non-matching hash reports absent",
			config: &Config{
				APITokens: []APITokenRecord{{Hash: "hash-1"}, {Hash: "hash-2"}},
			},
			hash: "not-present",
			want: false,
		},
		{
			name: "empty query hash against populated config reports absent",
			config: &Config{
				APITokens: []APITokenRecord{{Hash: "hash-1"}, {Hash: "hash-2"}},
			},
			hash: "",
			want: false,
		},
		{
			name: "empty query hash matches a stored empty-hash record",
			config: &Config{
				APITokens: []APITokenRecord{{Hash: ""}, {Hash: "hash-2"}},
			},
			hash: "",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.config.HasAPITokenHash(tt.hash))
		})
	}
}

// TestNewAvailabilityTarget_BranchCov0718 asserts every default field set by
// NewAvailabilityTarget, validates the generated ID is a parseable UUID, and
// verifies successive calls produce distinct IDs.
func TestNewAvailabilityTarget_BranchCov0718(t *testing.T) {
	t.Run("returns canonical defaults", func(t *testing.T) {
		target := NewAvailabilityTarget()

		assert.NotEmpty(t, target.ID, "ID should be generated")
		parsed, err := uuid.Parse(target.ID)
		assert.NoError(t, err, "ID should be a parseable UUID")
		assert.Equal(t, target.ID, parsed.String(), "ID should be in canonical UUID form")

		assert.Equal(t, AvailabilityTargetService, target.TargetKind)
		assert.Equal(t, AvailabilityProbeICMP, target.Protocol)
		assert.True(t, target.Enabled, "Enabled should default to true")
		assert.Equal(t, DefaultAvailabilityPollIntervalSecs, target.PollIntervalSecs)
		assert.Equal(t, DefaultAvailabilityTimeoutMillis, target.TimeoutMillis)
		assert.Equal(t, DefaultAvailabilityFailureThreshold, target.FailureThreshold)

		// Fields left at their zero values by the constructor.
		assert.Empty(t, target.Name)
		assert.Empty(t, target.Address)
		assert.Equal(t, 0, target.Port)
		assert.Empty(t, target.Path)
		assert.Empty(t, target.LinkedResourceID)
	})

	t.Run("each call generates a distinct UUID", func(t *testing.T) {
		a := NewAvailabilityTarget()
		b := NewAvailabilityTarget()
		assert.NotEqual(t, a.ID, b.ID, "successive calls should produce distinct IDs")
		assert.NotEmpty(t, a.ID)
		assert.NotEmpty(t, b.ID)
	})
}
