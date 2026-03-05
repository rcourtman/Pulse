package ai

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// isUnsupportedLegacyAIResourceTypeToken reports legacy/v5 type tokens that are
// explicitly rejected by strict v6 AI resource-type normalization paths.
func isUnsupportedLegacyAIResourceTypeToken(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
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
