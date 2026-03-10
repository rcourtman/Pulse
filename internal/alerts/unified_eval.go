package alerts

import (
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// Override Key Format by Resource Type
//
// The config.Overrides map is keyed by resource ID. Each resource type
// uses the following canonical key format:
//
//	VM / Container:    monitoring resource ID, e.g. "pve1:node1:100"
//	Node:              node.ID             e.g. "node/pve-1"
//	Agent:             host.ID             e.g. "host1" (without "agent:" prefix)
//	Agent Disk:        "agent:{hostID}/disk:{mountpoint}" e.g. "agent:host1/disk:root"
//	Storage:           storage.ID          e.g. "local-lvm"
//	PBS:               pbs.ID              e.g. "pbs-1"
//	PMG:               pmg.ID              e.g. "pmg-1"
//	Docker Container:  "docker:{hostID}/{containerID}"
//
// For guests, CheckUnifiedResource expects the same canonical resource ID the
// monitoring pipeline already uses. It does not translate IDs.
// UnifiedResourceMetric holds a single metric value for unified evaluation.
type UnifiedResourceMetric struct {
	Value   float64
	Percent float64
}

// UnifiedResourceInput is the data needed to evaluate alerts for a unified resource.
// This avoids importing unifiedresources (which would cause an import cycle).
type UnifiedResourceInput struct {
	ID         string
	Type       string // lowercase: "vm", "system-container", "app-container", "agent", "pbs", "storage", "pmg"
	Name       string
	Node       string
	Instance   string
	CPU        *UnifiedResourceMetric
	Memory     *UnifiedResourceMetric
	Disk       *UnifiedResourceMetric
	DiskRead   *UnifiedResourceMetric
	DiskWrite  *UnifiedResourceMetric
	NetworkIn  *UnifiedResourceMetric
	NetworkOut *UnifiedResourceMetric
}

type unifiedMetricCandidate struct {
	Spec      alertspecs.ResourceAlertSpec
	Value     float64
	Threshold *HysteresisThreshold
}

// unifiedAlertType maps a resource type key to the alert system's display type.
func unifiedAlertType(typeKey string) string {
	switch typeKey {
	case "vm":
		return "VM"
	case "system-container":
		return "Container"
	case "app-container":
		return "Docker"
	case "agent":
		return "Agent"
	case "node":
		return "Node"
	case "pbs":
		return "PBS"
	case "storage":
		return "Storage"
	case "pmg":
		return "PMG"
	default:
		return typeKey
	}
}

// isUnifiedGuestType returns true for resource types that support I/O metrics.
func isUnifiedGuestType(typeKey string) bool {
	switch typeKey {
	case "vm", "system-container", "app-container":
		return true
	default:
		return false
	}
}

// defaultThresholdsForResourceType returns the default ThresholdConfig for a resource type key.
func (m *Manager) defaultThresholdsForResourceType(typeKey string) ThresholdConfig {
	switch typeKey {
	case "vm", "system-container", "app-container":
		return cloneThresholdConfig(m.config.GuestDefaults)
	case "agent":
		return cloneThresholdConfig(m.config.AgentDefaults)
	case "node":
		return cloneThresholdConfig(m.config.NodeDefaults)
	case "pbs":
		return cloneThresholdConfig(m.config.PBSDefaults)
	case "storage":
		return ThresholdConfig{Usage: cloneThreshold(&m.config.StorageDefault)}
	default:
		return ThresholdConfig{}
	}
}

// resolveThresholdOverride applies an override for a resource ID onto an existing base config.
// Callers must hold m.mu when reading config through this helper.
func (m *Manager) resolveThresholdOverride(base ThresholdConfig, resourceID string) ThresholdConfig {
	if override, exists := m.config.Overrides[resourceID]; exists {
		return m.applyThresholdOverride(base, override)
	}
	return base
}

// resolveResourceThresholds builds the effective thresholds for a resource type and ID.
// Callers must hold m.mu when reading config through this helper.
func (m *Manager) resolveResourceThresholds(typeKey, resourceID string) ThresholdConfig {
	return m.resolveThresholdOverride(m.defaultThresholdsForResourceType(typeKey), resourceID)
}

// evaluateUnifiedMetrics runs the common metric dispatch path for unified resources.
func (m *Manager) evaluateUnifiedMetrics(input *UnifiedResourceInput, thresholds ThresholdConfig, opts *metricOptions) {
	if input == nil {
		return
	}

	for _, candidate := range buildUnifiedMetricCandidates(input, thresholds) {
		m.checkMetricWithCanonicalSpec(candidate.Spec, input.Name, input.Node, input.Instance, unifiedAlertType(input.Type), candidate.Value, candidate.Threshold, opts)
	}
}

func buildUnifiedMetricCandidates(input *UnifiedResourceInput, thresholds ThresholdConfig) []unifiedMetricCandidate {
	if input == nil {
		return nil
	}

	resourceType, ok := unifiedMetricResourceType(input.Type)
	if !ok {
		return nil
	}

	candidates := make([]unifiedMetricCandidate, 0, 8)
	appendCandidate := func(metricType string, metric *UnifiedResourceMetric, value float64, threshold *HysteresisThreshold) {
		if metric == nil {
			return
		}
		spec, err := buildCanonicalMetricSpec(input.ID, input.Name, resourceType, metricType, threshold)
		if err != nil {
			log.Warn().
				Err(err).
				Str("resourceID", input.ID).
				Str("resourceType", input.Type).
				Str("metricType", metricType).
				Msg("Skipping invalid canonical unified metric spec")
			return
		}
		candidates = append(candidates, unifiedMetricCandidate{
			Spec:      spec,
			Value:     value,
			Threshold: threshold,
		})
	}

	appendCandidate("cpu", input.CPU, input.CPUValue(), thresholds.CPU)
	appendCandidate("memory", input.Memory, input.MemoryValue(), thresholds.Memory)
	appendCandidate("disk", input.Disk, input.DiskValue(), thresholds.Disk)

	if isUnifiedGuestType(input.Type) {
		appendCandidate("diskRead", input.DiskRead, input.DiskReadValue(), thresholds.DiskRead)
		appendCandidate("diskWrite", input.DiskWrite, input.DiskWriteValue(), thresholds.DiskWrite)
		appendCandidate("networkIn", input.NetworkIn, input.NetworkInValue(), thresholds.NetworkIn)
		appendCandidate("networkOut", input.NetworkOut, input.NetworkOutValue(), thresholds.NetworkOut)
	}

	if input.Type == "storage" && input.Disk != nil {
		appendCandidate("usage", input.Disk, input.DiskValue(), thresholds.Usage)
	}

	return candidates
}

func buildCanonicalMetricSpec(resourceID, title string, resourceType unifiedresources.ResourceType, metricType string, threshold *HysteresisThreshold) (alertspecs.ResourceAlertSpec, error) {
	spec := alertspecs.ResourceAlertSpec{
		ID:           resourceID + "-" + metricType,
		ResourceID:   resourceID,
		ResourceType: resourceType,
		Kind:         alertspecs.AlertSpecKindMetricThreshold,
		Severity:     alertspecs.AlertSeverityWarning,
		Title:        title,
		Disabled:     threshold == nil || threshold.Trigger <= 0,
		MetricThreshold: &alertspecs.MetricThresholdSpec{
			Metric:    metricType,
			Direction: alertspecs.ThresholdDirectionAbove,
			Trigger:   0,
		},
	}

	if threshold != nil {
		spec.MetricThreshold.Trigger = threshold.Trigger
		if threshold.Clear > 0 && threshold.Clear < threshold.Trigger {
			recovery := threshold.Clear
			spec.MetricThreshold.Recovery = &recovery
		}
		critical := threshold.Trigger + 10
		spec.MetricThreshold.Critical = &critical
	}

	return spec, spec.Validate()
}

func (m *Manager) checkMetricWithCanonicalSpec(spec alertspecs.ResourceAlertSpec, resourceName, node, instance, resourceType string, value float64, threshold *HysteresisThreshold, opts *metricOptions) {
	if spec.MetricThreshold == nil {
		return
	}
	m.evaluateCanonicalMetricAlert(spec, resourceName, node, instance, resourceType, value, threshold, opts)
}

func unifiedMetricResourceType(typeKey string) (unifiedresources.ResourceType, bool) {
	switch typeKey {
	case "vm":
		return unifiedresources.ResourceTypeVM, true
	case "system-container":
		return unifiedresources.ResourceTypeSystemContainer, true
	case "app-container":
		return unifiedresources.ResourceTypeAppContainer, true
	case "agent":
		return unifiedresources.ResourceTypeAgent, true
	case "node":
		return unifiedresources.ResourceType("node"), true
	case "pbs":
		return unifiedresources.ResourceTypePBS, true
	case "storage":
		return unifiedresources.ResourceTypeStorage, true
	case "pmg":
		return unifiedresources.ResourceTypePMG, true
	default:
		return "", false
	}
}

func mergeMetricOptions(base *metricOptions, extra map[string]interface{}) *metricOptions {
	if len(extra) == 0 {
		return base
	}

	merged := &metricOptions{}
	if base != nil {
		*merged = *base
	}
	if len(extra) > 0 {
		if merged.Metadata == nil {
			merged.Metadata = make(map[string]interface{}, len(extra))
		} else {
			copied := make(map[string]interface{}, len(merged.Metadata)+len(extra))
			for k, v := range merged.Metadata {
				copied[k] = v
			}
			merged.Metadata = copied
		}
		for k, v := range extra {
			merged.Metadata[k] = v
		}
	}
	return merged
}

func (i *UnifiedResourceInput) CPUValue() float64 {
	if i == nil || i.CPU == nil {
		return 0
	}
	return i.CPU.Percent
}

func (i *UnifiedResourceInput) MemoryValue() float64 {
	if i == nil || i.Memory == nil {
		return 0
	}
	return i.Memory.Percent
}

func (i *UnifiedResourceInput) DiskValue() float64 {
	if i == nil || i.Disk == nil {
		return 0
	}
	return i.Disk.Percent
}

func (i *UnifiedResourceInput) DiskReadValue() float64 {
	if i == nil || i.DiskRead == nil {
		return 0
	}
	return i.DiskRead.Value
}

func (i *UnifiedResourceInput) DiskWriteValue() float64 {
	if i == nil || i.DiskWrite == nil {
		return 0
	}
	return i.DiskWrite.Value
}

func (i *UnifiedResourceInput) NetworkInValue() float64 {
	if i == nil || i.NetworkIn == nil {
		return 0
	}
	return i.NetworkIn.Value
}

func (i *UnifiedResourceInput) NetworkOutValue() float64 {
	if i == nil || i.NetworkOut == nil {
		return 0
	}
	return i.NetworkOut.Value
}

// CheckUnifiedResource evaluates threshold-based metric alerts for a unified resource.
// It resolves thresholds (defaults + overrides) and calls checkMetric for each
// available metric. Discrete event alerts (offline, RAID, backup age, etc.)
// are NOT evaluated here — they remain in the typed Check* methods.
func (m *Manager) CheckUnifiedResource(input *UnifiedResourceInput) {
	if input == nil {
		return
	}

	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return
	}

	thresholds := m.resolveResourceThresholds(input.Type, input.ID)
	m.mu.RUnlock()

	if thresholds.Disabled {
		return
	}

	log.Debug().
		Str("resourceID", input.ID).
		Str("resourceName", input.Name).
		Str("resourceType", unifiedAlertType(input.Type)).
		Msg("Evaluating unified resource metrics")

	m.evaluateUnifiedMetrics(input, thresholds, nil)
}
