package unifiedresources

import "strings"

// CanonicalizeLegacyResourceTypeAlias returns the canonical v6 type for a known
// removed legacy alias token.
func CanonicalizeLegacyResourceTypeAlias(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "host":
		return "agent", true
	case "system_container":
		return "system-container", true
	case "docker_container", "app_container":
		return "app-container", true
	case "docker_host":
		return "docker-host", true
	case "kubernetes_cluster", "k8s_cluster":
		return "k8s-cluster", true
	default:
		return "", false
	}
}

// IsUnsupportedLegacyResourceTypeAlias reports whether the provided resource type
// token is a removed legacy alias in strict v6 runtime paths.
func IsUnsupportedLegacyResourceTypeAlias(value string) bool {
	_, ok := CanonicalizeLegacyResourceTypeAlias(value)
	return ok
}

// IsUnsupportedLegacyResourceIDAlias reports whether the provided resource ID
// uses a removed legacy host-prefixed identifier format.
func IsUnsupportedLegacyResourceIDAlias(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "host:")
}
