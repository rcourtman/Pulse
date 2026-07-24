package tools

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

// This file raises branch coverage on the unexported helper
// findCanonicalAppContainerResourceByReferences (tools_query.go:2865):
//
//	func findCanonicalAppContainerResourceByReferences(provider UnifiedResourceProvider, references ...string) (unifiedresources.Resource, bool) {
//	    for _, ref := range references {
//	        if resource, _, ok := findCanonicalAppContainerResource(provider, ref); ok {
//	            return resource, true
//	        }
//	    }
//	    return unifiedresources.Resource{}, false
//	}
//
// It walks the supplied references in order and returns the first app-container
// resource any of them resolves to. The cases below cover every arm: no
// references at all, a nil provider, no match, a first-reference match, a
// later-reference match, empty-string references that must be skipped, and
// ambiguous/duplicate references where the first matching reference must win.
//
// matchCanonicalAppContainerResource (the per-resource matcher the inner helper
// calls) accepts four reference shapes, so the table also drives each of them:
// canonical ID (resource.ID), Docker provider ID (Docker.ContainerID), display
// name (resource.Name), and a prefix of the provider ID.

// branchcov0724pmFakeProvider is a minimal in-package UnifiedResourceProvider
// whose GetByType returns a fixed, type-keyed table. Only the app-container
// bucket is populated; every other type yields nothing, matching how the real
// registry isolates app containers.
type branchcov0724pmFakeProvider struct {
	byType map[unifiedresources.ResourceType][]unifiedresources.Resource
}

func (f *branchcov0724pmFakeProvider) GetByType(t unifiedresources.ResourceType) []unifiedresources.Resource {
	if f == nil || f.byType == nil {
		return nil
	}
	return f.byType[t]
}

// branchcov0724pmAppContainer builds an app-container resource whose three
// referenceable identifiers are distinct, so each matcher arm can be targeted
// independently.
func branchcov0724pmAppContainer(id, name, dockerContainerID string) unifiedresources.Resource {
	return unifiedresources.Resource{
		ID:   id,
		Type: unifiedresources.ResourceTypeAppContainer,
		Name: name,
		Docker: &unifiedresources.DockerData{
			ContainerID: dockerContainerID,
		},
	}
}

func TestBranchcov0724pmFindCanonicalAppContainerResourceByReferences(t *testing.T) {
	alpha := branchcov0724pmAppContainer("app/alpha", "alpha-app", "cid-alpha")
	beta := branchcov0724pmAppContainer("app/beta", "beta-app", "cid-beta")

	provider := &branchcov0724pmFakeProvider{
		byType: map[unifiedresources.ResourceType][]unifiedresources.Resource{
			unifiedresources.ResourceTypeAppContainer: {alpha, beta},
		},
	}

	cases := []struct {
		name       string
		provider   UnifiedResourceProvider
		references []string
		wantOK     bool
		wantID     string // expected resource.ID when wantOK is true
	}{
		{
			name:       "no references returns not found",
			provider:   provider,
			references: nil,
			wantOK:     false,
		},
		{
			name:       "nil provider returns not found",
			provider:   nil,
			references: []string{"app/alpha"},
			wantOK:     false,
		},
		{
			name:       "single non-matching reference returns not found",
			provider:   provider,
			references: []string{"does-not-exist"},
			wantOK:     false,
		},
		{
			name:       "all-empty references return not found",
			provider:   provider,
			references: []string{"", "  ", ""},
			wantOK:     false,
		},
		{
			name:       "first reference matches by canonical id",
			provider:   provider,
			references: []string{"app/alpha"},
			wantOK:     true,
			wantID:     "app/alpha",
		},
		{
			name:       "match by docker provider id",
			provider:   provider,
			references: []string{"cid-beta"},
			wantOK:     true,
			wantID:     "app/beta",
		},
		{
			name:       "match by display name",
			provider:   provider,
			references: []string{"alpha-app"},
			wantOK:     true,
			wantID:     "app/alpha",
		},
		{
			name:       "match by prefix of provider id",
			provider:   provider,
			references: []string{"cid-al"},
			wantOK:     true,
			wantID:     "app/alpha",
		},
		{
			name:       "reference is matched case-insensitively",
			provider:   provider,
			references: []string{"APP/ALPHA"},
			wantOK:     true,
			wantID:     "app/alpha",
		},
		{
			name:       "later reference matches after an earlier miss",
			provider:   provider,
			references: []string{"nope", "app/beta"},
			wantOK:     true,
			wantID:     "app/beta",
		},
		{
			name:       "empty reference is skipped and the next one matches",
			provider:   provider,
			references: []string{"", "app/alpha"},
			wantOK:     true,
			wantID:     "app/alpha",
		},
		{
			name:       "ambiguous references resolve to the first matching reference",
			provider:   provider,
			references: []string{"app/beta", "app/alpha"},
			wantOK:     true,
			wantID:     "app/beta",
		},
		{
			name:       "duplicate matching reference resolves once to the first hit",
			provider:   provider,
			references: []string{"app/alpha", "app/alpha"},
			wantOK:     true,
			wantID:     "app/alpha",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := findCanonicalAppContainerResourceByReferences(tc.provider, tc.references...)
			require.Equal(t, tc.wantOK, ok, "ok verdict")

			if tc.wantOK {
				require.Equal(t, tc.wantID, got.ID, "matched resource ID")
			} else {
				// No match must return the zero-value resource.
				require.Equal(t, "", got.ID, "expected zero-value resource on miss")
			}
		})
	}
}

// TestBranchcov0724pmFindCanonicalAppContainerResourceByReferencesEmptyProvider
// covers the empty-but-non-nil provider: GetByType returns no app containers,
// so every reference misses and the function returns not found. This exercises
// the inner loop completing without a single GetByType match.
func TestBranchcov0724pmFindCanonicalAppContainerResourceByReferencesEmptyProvider(t *testing.T) {
	empty := &branchcov0724pmFakeProvider{
		byType: map[unifiedresources.ResourceType][]unifiedresources.Resource{},
	}

	got, ok := findCanonicalAppContainerResourceByReferences(empty, "app/alpha", "cid-beta", "alpha-app")
	require.False(t, ok, "expected not found with an empty provider")
	require.Equal(t, "", got.ID, "expected zero-value resource")
}
