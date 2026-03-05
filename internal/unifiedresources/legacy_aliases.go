package unifiedresources

import "strings"

// IsUnsupportedLegacyResourceTypeAlias reports whether the provided resource type
// token is a removed legacy alias in strict v6 runtime paths.
func IsUnsupportedLegacyResourceTypeAlias(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "host")
}

// IsUnsupportedLegacyResourceIDAlias reports whether the provided resource ID
// uses a removed legacy host-prefixed identifier format.
func IsUnsupportedLegacyResourceIDAlias(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "host:")
}
