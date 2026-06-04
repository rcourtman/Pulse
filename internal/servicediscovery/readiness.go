package servicediscovery

import (
	"strings"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const discoveryReadinessSource = "service-discovery"

// DiscoveryResourceTypeForTarget maps unified resource discovery targets onto
// the persisted service-discovery resource vocabulary.
func DiscoveryResourceTypeForTarget(target *unified.DiscoveryTarget) (ResourceType, bool) {
	if target == nil {
		return "", false
	}
	switch normalized := strings.ToLower(strings.TrimSpace(target.ResourceType)); normalized {
	case string(ResourceTypeAgent):
		return ResourceTypeAgent, true
	case string(ResourceTypeVM):
		return ResourceTypeVM, true
	case string(ResourceTypeSystemContainer):
		return ResourceTypeSystemContainer, true
	case "app-container", string(ResourceTypeDocker):
		return ResourceTypeDocker, true
	case "pod", string(ResourceTypeK8s):
		return ResourceTypeK8s, true
	default:
		if strings.HasPrefix(normalized, "k8s-") {
			return ResourceTypeK8s, true
		}
		return "", false
	}
}

// DiscoveryIDForTarget returns the persisted discovery id that corresponds to
// a unified discovery target when that target can be represented by service
// discovery.
func DiscoveryIDForTarget(target *unified.DiscoveryTarget) (string, bool) {
	resourceType, ok := DiscoveryResourceTypeForTarget(target)
	if !ok || target == nil {
		return "", false
	}
	targetID := strings.TrimSpace(target.AgentID)
	resourceID := strings.TrimSpace(target.ResourceID)
	if targetID == "" || resourceID == "" {
		return "", false
	}
	return MakeResourceID(resourceType, targetID, resourceID), true
}

// DiscoveryReadinessForResource projects the discovery store into the stable
// unified resource readiness contract.
func (s *Service) DiscoveryReadinessForResource(resource unified.Resource, now time.Time) unified.ResourceDiscoveryReadiness {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	target := resource.DiscoveryTarget
	if target == nil {
		return DiscoveryReadinessForTarget(nil, nil, nil, defaultDiscoveryMaxAge, now)
	}
	if s == nil || s.store == nil {
		return DiscoveryReadinessUnavailableForTarget(target, now, "Discovery service is not configured.")
	}

	resourceType, ok := DiscoveryResourceTypeForTarget(target)
	if !ok {
		return DiscoveryReadinessForTarget(target, nil, nil, s.GetMaxDiscoveryAge(), now)
	}

	discoveryID, _ := DiscoveryIDForTarget(target)
	progress := s.GetProgress(discoveryID)
	discovery, err := s.GetDiscoveryByResource(resourceType, target.AgentID, target.ResourceID)
	if err != nil {
		return DiscoveryReadinessReadFailureForTarget(target, now)
	}
	return DiscoveryReadinessForTarget(target, discovery, progress, s.GetMaxDiscoveryAge(), now)
}

// DiscoveryReadinessUnavailableForTarget creates a readiness projection for a
// supported target when Pulse cannot currently read service-discovery state.
func DiscoveryReadinessUnavailableForTarget(target *unified.DiscoveryTarget, now time.Time, reason string) unified.ResourceDiscoveryReadiness {
	readiness := discoveryReadinessBase(target, now)
	readiness.State = unified.ResourceDiscoveryReadinessUnavailable
	readiness.Reason = strings.TrimSpace(reason)
	if readiness.Reason == "" {
		readiness.Reason = "Discovery status is not available."
	}
	return readiness
}

// DiscoveryReadinessReadFailureForTarget creates a readiness projection for a
// supported target when the discovery store read failed.
func DiscoveryReadinessReadFailureForTarget(target *unified.DiscoveryTarget, now time.Time) unified.ResourceDiscoveryReadiness {
	readiness := discoveryReadinessBase(target, now)
	readiness.State = unified.ResourceDiscoveryReadinessFailed
	readiness.Reason = "Discovery status could not be read."
	return readiness
}

// DiscoveryReadinessForTarget summarizes discovery, scan progress, and the max
// age policy without exposing raw discovery output.
func DiscoveryReadinessForTarget(target *unified.DiscoveryTarget, discovery *ResourceDiscovery, progress *DiscoveryProgress, maxAge time.Duration, now time.Time) unified.ResourceDiscoveryReadiness {
	readiness := discoveryReadinessBase(target, now)
	if target == nil {
		readiness.State = unified.ResourceDiscoveryReadinessUnsupported
		readiness.Reason = "No discovery target is available for this resource."
		return readiness
	}
	if _, ok := DiscoveryResourceTypeForTarget(target); !ok {
		readiness.State = unified.ResourceDiscoveryReadinessUnsupported
		readiness.Reason = "Service discovery does not support this resource type."
		return readiness
	}

	if progress != nil {
		observedAt := timePtrIfSet(progress.StartedAt)
		readiness.ObservedAt = observedAt
		if strings.TrimSpace(progress.Error) != "" {
			readiness.State = unified.ResourceDiscoveryReadinessFailed
			readiness.Reason = "Last discovery run failed."
			return readiness
		}
		if progress.Status == DiscoveryStatusRunning {
			readiness.State = unified.ResourceDiscoveryReadinessRunning
			readiness.Reason = "Discovery is currently running."
			return readiness
		}
	}

	if discovery == nil {
		readiness.State = unified.ResourceDiscoveryReadinessMissing
		readiness.Reason = "Discovery has not run for this resource."
		return readiness
	}

	readiness.DiscoveryID = strings.TrimSpace(discovery.ID)
	if readiness.DiscoveryID == "" {
		if discoveryID, ok := DiscoveryIDForTarget(target); ok {
			readiness.DiscoveryID = discoveryID
		}
	}
	readiness.ServiceName = strings.TrimSpace(discovery.ServiceName)
	readiness.ServiceCategory = strings.TrimSpace(string(discovery.Category))
	readiness.Confidence = discovery.Confidence
	readiness.FactCount = discoveryContextFactCount(discovery)

	observed := discovery.UpdatedAt
	if observed.IsZero() {
		observed = discovery.DiscoveredAt
	}
	if !observed.IsZero() {
		observed = observed.UTC()
		readiness.ObservedAt = &observed
		age := now.Sub(observed)
		if age < 0 {
			age = 0
		}
		readiness.AgeSeconds = int64(age.Seconds())
	}

	if maxAge <= 0 {
		maxAge = defaultDiscoveryMaxAge
	}
	readiness.StaleAfterSeconds = int64(maxAge.Seconds())
	if readiness.ObservedAt == nil {
		readiness.State = unified.ResourceDiscoveryReadinessStale
		readiness.Reason = "Discovery exists but has no observation timestamp."
		return readiness
	}
	if time.Duration(readiness.AgeSeconds)*time.Second > maxAge {
		readiness.State = unified.ResourceDiscoveryReadinessStale
		readiness.Reason = "Discovery data is older than the configured freshness window."
		return readiness
	}
	readiness.State = unified.ResourceDiscoveryReadinessFresh
	readiness.Reason = "Discovery data is within the configured freshness window."
	return readiness
}

func discoveryReadinessBase(target *unified.DiscoveryTarget, now time.Time) unified.ResourceDiscoveryReadiness {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	readiness := unified.ResourceDiscoveryReadiness{
		Source:      discoveryReadinessSource,
		GeneratedAt: now.UTC(),
	}
	if target == nil {
		return readiness
	}
	readiness.ResourceType = strings.TrimSpace(target.ResourceType)
	readiness.TargetID = strings.TrimSpace(target.AgentID)
	readiness.ResourceID = strings.TrimSpace(target.ResourceID)
	if discoveryID, ok := DiscoveryIDForTarget(target); ok {
		readiness.DiscoveryID = discoveryID
	}
	return readiness
}

func discoveryContextFactCount(discovery *ResourceDiscovery) int {
	if discovery == nil {
		return 0
	}
	return len(discovery.Facts) +
		len(discovery.ConfigPaths) +
		len(discovery.DataPaths) +
		len(discovery.LogPaths) +
		len(discovery.Ports) +
		len(discovery.DockerMounts)
}

func timePtrIfSet(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	value = value.UTC()
	return &value
}
