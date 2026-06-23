package agentcapabilities

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// NormalizeRequiredScopes returns the unique non-empty scopes ordered by the
// canonical auth vocabulary and then by input order for future scopes that have
// not yet landed in auth.AllKnownScopes.
func NormalizeRequiredScopes(scopes []string) []string {
	seen := make(map[string]bool)
	for _, raw := range scopes {
		scope := strings.TrimSpace(raw)
		if scope == "" {
			continue
		}
		seen[scope] = true
	}
	if len(seen) == 0 {
		return nil
	}

	ordered := make([]string, 0, len(seen))
	for _, scope := range auth.AllKnownScopes {
		if !seen[scope] {
			continue
		}
		ordered = append(ordered, scope)
		delete(seen, scope)
	}
	for _, raw := range scopes {
		scope := strings.TrimSpace(raw)
		if !seen[scope] {
			continue
		}
		ordered = append(ordered, scope)
		delete(seen, scope)
	}
	return ordered
}

// RequiredCapabilityScopes returns the unique non-empty scopes declared by a
// capability set, ordered by the canonical auth vocabulary and then by manifest
// order for any future scope that has not yet landed in auth.AllKnownScopes.
func RequiredCapabilityScopes(capabilities []Capability) []string {
	scopes := make([]string, 0, len(capabilities))
	for _, cap := range capabilities {
		scopes = append(scopes, cap.Scope)
	}
	return NormalizeRequiredScopes(scopes)
}

// RequiredScopeList formats a manifest-owned required-scope set for operator
// guidance and adapter startup errors.
func RequiredScopeList(scopes []string) string {
	return strings.Join(NormalizeRequiredScopes(scopes), ", ")
}

// RequiredCapabilityScopeList formats RequiredCapabilityScopes for operator
// guidance and adapter startup errors.
func RequiredCapabilityScopeList(capabilities []Capability) string {
	return RequiredScopeList(RequiredCapabilityScopes(capabilities))
}

// ManifestRequiredScopeList formats the manifest-owned requiredScopes summary.
// Older manifests did not expose requiredScopes, so callers get a compatibility
// fallback from the capability rows while current manifests stay field-owned.
func ManifestRequiredScopeList(manifest *Manifest) string {
	if manifest == nil {
		return ""
	}
	if scopeList := RequiredScopeList(manifest.RequiredScopes); scopeList != "" {
		return scopeList
	}
	return RequiredCapabilityScopeList(manifest.Capabilities)
}
