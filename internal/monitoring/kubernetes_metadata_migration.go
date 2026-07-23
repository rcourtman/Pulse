package monitoring

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

func kubernetesWorkloadMetadataKey(resource unifiedresources.Resource) string {
	if resource.Kubernetes == nil {
		return ""
	}

	clusterID := strings.TrimSpace(resource.Kubernetes.ClusterID)
	namespace := strings.TrimSpace(resource.Kubernetes.Namespace)
	name := strings.TrimSpace(resource.Name)
	if clusterID == "" || namespace == "" || name == "" {
		return ""
	}

	var kind string
	switch unifiedresources.CanonicalResourceType(resource.Type) {
	case unifiedresources.ResourceTypePod:
		kind = "pod"
	case unifiedresources.ResourceTypeK8sDeployment:
		kind = "deployment"
	case unifiedresources.ResourceTypeK8sService:
		kind = "service"
	default:
		return ""
	}

	return fmt.Sprintf("k8s-workload:%s:%s:%s:%s", clusterID, kind, namespace, name)
}

func kubernetesWorkloadLegacyMetadataKeys(resource unifiedresources.Resource) []string {
	candidates := []string{strings.TrimSpace(resource.ID)}
	if resource.Kubernetes != nil &&
		unifiedresources.CanonicalResourceType(resource.Type) == unifiedresources.ResourceTypePod {
		clusterID := strings.TrimSpace(resource.Kubernetes.ClusterID)
		podUID := strings.TrimSpace(resource.Kubernetes.PodUID)
		if clusterID != "" && podUID != "" {
			candidates = append(candidates, fmt.Sprintf("k8s:%s:pod:%s", clusterID, podUID))
		}
	}

	out := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func cloneKubernetesGuestMetadata(
	source *config.GuestMetadata,
	resource unifiedresources.Resource,
) *config.GuestMetadata {
	clone := &config.GuestMetadata{
		CustomURL:     source.CustomURL,
		Description:   source.Description,
		LastKnownName: strings.TrimSpace(resource.Name),
		LastKnownType: string(unifiedresources.CanonicalResourceType(resource.Type)),
	}
	if len(source.Tags) > 0 {
		clone.Tags = append([]string(nil), source.Tags...)
	}
	if len(source.Notes) > 0 {
		clone.Notes = append([]string(nil), source.Notes...)
	}
	return clone
}

func (m *Monitor) kubernetesWorkloadCustomURL(resource unifiedresources.Resource) (string, bool) {
	if m == nil || m.guestMetadataStore == nil {
		return "", false
	}

	stableKey := kubernetesWorkloadMetadataKey(resource)
	if stableKey == "" {
		return "", false
	}
	var (
		customURL      string
		found          bool
		migratedLegacy string
	)
	err := m.guestMetadataStore.UpdateAll(func(metadata map[string]*config.GuestMetadata) bool {
		if meta := metadata[stableKey]; meta != nil {
			customURL = strings.TrimSpace(meta.CustomURL)
			found = true
			return false
		}
		for _, legacyKey := range kubernetesWorkloadLegacyMetadataKeys(resource) {
			meta := metadata[legacyKey]
			if meta == nil {
				continue
			}
			customURL = strings.TrimSpace(meta.CustomURL)
			found = true
			migratedLegacy = legacyKey
			clone := cloneKubernetesGuestMetadata(meta, resource)
			clone.ID = stableKey
			metadata[stableKey] = clone
			return true
		}
		return false
	})
	if err != nil {
		log.Warn().
			Err(err).
			Str("resourceID", resource.ID).
			Str("legacyMetadataKey", migratedLegacy).
			Str("stableMetadataKey", stableKey).
			Msg("Failed to migrate Kubernetes workload metadata to stable logical identity")
	}
	return customURL, found
}
