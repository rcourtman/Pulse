package tools

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// resourceMetricsTargetResolver is the optional capability the unified
// resource provider exposes for translating canonical resource IDs into
// metrics-store query targets. The production provider (the monitor's
// unified adapter) implements it; lightweight fixtures may not.
type resourceMetricsTargetResolver interface {
	MetricsTargetForResource(resourceID string) *unifiedresources.MetricsTarget
}

// resourceMetricsTarget resolves the metrics-store target for a unified
// resource, asking the provider's on-demand resolver first (registry targets
// are computed lazily; the struct field is only populated by fixtures).
func (e *PulseToolExecutor) resourceMetricsTarget(res unifiedresources.Resource) *unifiedresources.MetricsTarget {
	if resolver, ok := e.unifiedResourceProvider.(resourceMetricsTargetResolver); ok && resolver != nil {
		if target := resolver.MetricsTargetForResource(res.ID); target != nil {
			return target
		}
	}
	return res.MetricsTarget
}

// resolvePerformanceResource keeps canonical identity separate from metrics-store
// coordinates. Names and provider IDs are accepted only when unambiguous.
func (e *PulseToolExecutor) resolvePerformanceResource(reference, kind string) (unifiedresources.Resource, error) {
	reference = strings.TrimSpace(reference)
	var matches []unifiedresources.Resource
	for _, resourceType := range []unifiedresources.ResourceType{
		unifiedresources.ResourceTypeAgent, unifiedresources.ResourceTypeVM, unifiedresources.ResourceTypeSystemContainer,
	} {
		if kind != "" && canonicalQueryResourceType(kind) != string(resourceType) {
			continue
		}
		for _, resource := range e.unifiedResourceProvider.GetByType(resourceType) {
			if resource.ID == reference {
				return resource, nil
			}
			candidates := []string{resource.Name, resourceDisplayName(resource), canonicalGuestManagedID(resource)}
			candidates = append(candidates, resource.Identity.Hostnames...)
			if target := e.resourceMetricsTarget(resource); target != nil {
				candidates = append(candidates, target.ResourceID)
			}
			if matchesCanonicalResourceReference(resource, reference, candidates...) {
				matches = append(matches, resource)
			}
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return unifiedresources.Resource{}, fmt.Errorf("resource reference %q is ambiguous; use a canonical resource ID from pulse_query", reference)
	}
	return unifiedresources.Resource{}, fmt.Errorf("resource %q was not found for performance metrics; use pulse_query to find its canonical ID", reference)
}
