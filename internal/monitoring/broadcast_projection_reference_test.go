package monitoring

import (
	"encoding/json"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"sort"
	"strings"
)

// Retain the pre-repair projection as a content oracle, including its second
// coalesce and input-first sorting. This is never production or persistence code.
func convertResourcesForBroadcastReference(
	allResources []unifiedresources.Resource,
	metricsTargetResolvers ...MetricsTargetResourceStore,
) ([]models.ResourceFrontend, broadcastResourceCatalogs) {
	if len(allResources) == 0 {
		return []models.ResourceFrontend{}, broadcastResourceCatalogs{}
	}
	allResources = attachBroadcastMetricsTargets(
		allResources,
		firstBroadcastMetricsTargetResolver(metricsTargetResolvers),
	)
	allResources = unifiedresources.CoalescePresentationHostResources(allResources)
	type broadcastResource struct {
		input      models.ResourceConvertInput
		sortKey    string
		resourceID string
	}

	converted := make([]broadcastResource, 0, len(allResources))
	catalogs := broadcastResourceCatalogs{
		capabilities:    make(map[string]json.RawMessage),
		policies:        make(map[string]json.RawMessage),
		aiSafeSummaries: make(map[string]string),
	}
	for _, r := range allResources {
		input := monitorResourceToConvertInput(r)
		if len(input.Capabilities) > 0 {
			id := capabilityCatalogID(input.Capabilities)
			catalogs.capabilities[id] = input.Capabilities
			input.CapabilitiesRef = id
			input.Capabilities = nil
		}
		// Non-default policies and AI-safe summaries dedupe the same way:
		// estates carry a handful of distinct postures and templated summary
		// strings, so refs replace per-resource inline duplication.
		if len(input.Policy) > 0 {
			id := capabilityCatalogID(input.Policy)
			catalogs.policies[id] = input.Policy
			input.PolicyRef = id
			input.Policy = nil
		}
		if input.AISafeSummary != "" {
			id := capabilityCatalogID(json.RawMessage(input.AISafeSummary))
			catalogs.aiSafeSummaries[id] = input.AISafeSummary
			input.AISafeSummaryRef = id
			input.AISafeSummary = ""
		}
		sortKey := strings.ToLower(input.DisplayName)
		if sortKey == "" {
			sortKey = strings.ToLower(input.Name)
		}
		converted = append(converted, broadcastResource{
			input:      input,
			sortKey:    sortKey,
			resourceID: input.ID,
		})
	}

	sort.Slice(converted, func(i, j int) bool {
		if converted[i].sortKey == converted[j].sortKey {
			return converted[i].resourceID < converted[j].resourceID
		}
		return converted[i].sortKey < converted[j].sortKey
	})

	result := make([]models.ResourceFrontend, len(converted))
	for i, resource := range converted {
		result[i] = models.ConvertResourceToFrontend(resource.input)
	}
	if len(catalogs.capabilities) == 0 {
		catalogs.capabilities = nil
	}
	if len(catalogs.policies) == 0 {
		catalogs.policies = nil
	}
	if len(catalogs.aiSafeSummaries) == 0 {
		catalogs.aiSafeSummaries = nil
	}
	return result, catalogs
}
