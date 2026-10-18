package ai

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// A child reference selects its owner only through an exact collected alert.
// An arbitrary caller-authored child path cannot broaden Patrol scope.
func patrolScopeAlert(state patrolRuntimeState, scope PatrolScope) *models.Alert {
	identifier := strings.TrimSpace(scope.AlertIdentifier)
	if identifier == "" {
		return nil
	}
	var found *models.Alert
	if scope.Reason != TriggerReasonAlertCleared {
		for i := range state.ActiveAlerts {
			if state.ActiveAlerts[i].ID != identifier {
				continue
			}
			if found != nil {
				return nil
			}
			alert := state.ActiveAlerts[i]
			found = &alert
		}
		if found != nil {
			return found
		}
	}
	// Recurrences reuse an alert ID. An open occurrence takes precedence;
	// after clearing, use the latest collected resolution for that ID.
	var latest *models.ResolvedAlert
	for i := range state.RecentlyResolved {
		if state.RecentlyResolved[i].Alert.ID != identifier {
			continue
		}
		candidate := &state.RecentlyResolved[i]
		if latest == nil || candidate.ResolvedTime.After(latest.ResolvedTime) {
			latest = candidate
		}
	}
	if latest == nil {
		return nil
	}
	alert := latest.Alert
	return &alert
}

func patrolAlertReferenceResolver(state patrolRuntimeState) unifiedresources.AlertResourceReferenceResolver {
	if state.alertResourceResolver != nil {
		return state.alertResourceResolver
	}
	if resolver, ok := state.readState.(unifiedresources.AlertResourceReferenceResolver); ok {
		return resolver
	}
	if resolver, ok := state.unifiedResourceProvider.(unifiedresources.AlertResourceReferenceResolver); ok {
		return resolver
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(state.resourceSnapshot())
	if state.unifiedResourceProvider != nil {
		registry.IngestResources(state.unifiedResourceProvider.GetAll())
	}
	return registry
}

func patrolAlertOwnerIDs(resolver unifiedresources.AlertResourceReferenceResolver, binding unifiedresources.AlertResourceReference) []string {
	ids := []string{binding.ResourceID}
	for _, target := range binding.SourceTargets {
		// Unqualified source IDs can collide across collectors. Only project
		// IDs that still uniquely identify this owner into the snapshot matcher.
		candidate, _ := resolver.ResolveAlertResourceReference(target.SourceID)
		if candidate.ResourceID == binding.ResourceID {
			ids = append(ids, target.SourceID)
		}
	}
	return ids
}

// Keep exact child evidence when its current owner is selected. The subject's
// original identity stays on the alert; it is not an extra scoped resource.
func patrolAlertBelongsToScope(state patrolRuntimeState, resourceID string, included map[string]bool) bool {
	if included[resourceID] {
		return true
	}
	resolver := patrolAlertReferenceResolver(state)
	binding, _ := resolver.ResolveAlertResourceReference(resourceID)
	if binding.ResourceID == "" {
		return false
	}
	if included[binding.ResourceID] {
		return true
	}
	for _, target := range binding.SourceTargets {
		if !included[target.SourceID] {
			continue
		}
		candidate, _ := resolver.ResolveAlertResourceReference(target.SourceID)
		if candidate.ResourceID == binding.ResourceID {
			return true
		}
	}
	return false
}

func patrolAlertContext(alert *models.Alert) *PatrolAlertContext {
	if alert == nil {
		return nil
	}
	metadataString := func(key string) string { value, _ := alert.Metadata[key].(string); return value }
	return &PatrolAlertContext{AlertType: alert.Type, Level: alert.Level, Value: alert.Value, Threshold: alert.Threshold,
		Message: alert.Message, ResourceID: alert.ResourceID, ResourceName: alert.ResourceName,
		Mountpoint: metadataString("mountpoint"), Device: metadataString("device")}
}
