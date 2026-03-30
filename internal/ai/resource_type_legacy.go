package ai

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// canonicalizeAICompatibilityResourceType collapses compatibility-only runtime
// tokens onto the canonical v6 AI resource type contract.
func canonicalizeAICompatibilityResourceType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "truenas":
		return "agent"
	case "physical-disk":
		return "physical_disk"
	default:
		return normalized
	}
}

// isUnsupportedLegacyAIResourceTypeToken reports legacy/v5 type tokens that are
// explicitly rejected by strict v6 AI resource-type normalization paths.
func isUnsupportedLegacyAIResourceTypeToken(value string) bool {
	normalized := canonicalizeAICompatibilityResourceType(value)
	if normalized == "" {
		return false
	}
	if unifiedresources.IsUnsupportedLegacyResourceTypeAlias(normalized) {
		return true
	}
	switch normalized {
	case "guest", "qemu", "container", "lxc", "docker", "docker-container", "k8s", "kubernetes", "kubernetes-cluster", "docker_service", "dockerhost":
		return true
	default:
		return false
	}
}
