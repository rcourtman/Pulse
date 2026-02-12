package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPITokenRecord_CanAccessOrg(t *testing.T) {
	tests := []struct {
		name     string
		token    APITokenRecord
		orgID    string
		expected bool
	}{
		// Legacy / Wildcard
		{
			name:     "Legacy Token (No OrgID) - Access Random Org",
			token:    APITokenRecord{OrgID: ""},
			orgID:    "any-org",
			expected: true,
		},
		{
			name:     "Legacy Token - Access Default",
			token:    APITokenRecord{OrgID: ""},
			orgID:    "default",
			expected: true,
		},

		// Single Org Binding
		{
			name:     "Bound Token - Correct Org",
			token:    APITokenRecord{OrgID: "org-1"},
			orgID:    "org-1",
			expected: true,
		},
		{
			name:     "Bound Token - Wrong Org",
			token:    APITokenRecord{OrgID: "org-1"},
			orgID:    "org-2",
			expected: false,
		},
		{
			name:     "Bound Token - Access Default (Denied)",
			token:    APITokenRecord{OrgID: "org-1"},
			orgID:    "default",
			expected: false,
		},

		// Multi-Org Binding
		{
			name:     "Multi-Bound Token - Match First",
			token:    APITokenRecord{OrgIDs: []string{"org-1", "org-2"}},
			orgID:    "org-1",
			expected: true,
		},
		{
			name:     "Multi-Bound Token - Match Second",
			token:    APITokenRecord{OrgIDs: []string{"org-1", "org-2"}},
			orgID:    "org-2",
			expected: true,
		},
		{
			name:     "Multi-Bound Token - No Match",
			token:    APITokenRecord{OrgIDs: []string{"org-1", "org-2"}},
			orgID:    "org-3",
			expected: false,
		},
		{
			name:     "Multi-Bound Token - Explicit Default",
			token:    APITokenRecord{OrgIDs: []string{"org-1", "default"}},
			orgID:    "default",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.token.CanAccessOrg(tt.orgID))
		})
	}
}

func TestAPITokenRecord_GetBoundOrgs(t *testing.T) {
	t.Run("Legacy Token", func(t *testing.T) {
		token := APITokenRecord{}
		assert.Nil(t, token.GetBoundOrgs())
	})

	t.Run("Single Org Token", func(t *testing.T) {
		token := APITokenRecord{OrgID: "org-1"}
		assert.Equal(t, []string{"org-1"}, token.GetBoundOrgs())
	})

	t.Run("Multi Org Token", func(t *testing.T) {
		token := APITokenRecord{OrgIDs: []string{"org-1", "org-2"}}
		assert.Equal(t, []string{"org-1", "org-2"}, token.GetBoundOrgs())
	})
}
