package alerts

import (
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const unifiedIncidentAlertPrefix = "unified-incident-"

// SyncUnifiedResourceIncidents mirrors canonical unified-resource incidents into
// the live alert manager so provider-native storage incidents participate in the
// standard alert pipeline.
func (m *Manager) SyncUnifiedResourceIncidents(resources []unifiedresources.Resource) {
	if m == nil {
		return
	}

	desired := make(map[string]*Alert)

	m.mu.RLock()
	enabled := m.config.Enabled
	disableAllStorage := m.config.DisableAllStorage
	overrides := m.config.Overrides
	m.mu.RUnlock()

	if enabled {
		now := time.Now()
		for _, resource := range resources {
			if !resourceSupportsUnifiedIncidentAlerts(resource) {
				continue
			}
			if disableAllStorage {
				continue
			}
			if override, exists := overrides[resource.ID]; exists && override.Disabled {
				continue
			}
			for _, incident := range resource.Incidents {
				level, ok := alertLevelFromIncidentSeverity(incident.Severity)
				if !ok {
					continue
				}
				alert := unifiedIncidentAlert(resource, incident, level, now)
				desired[alert.ID] = alert
			}
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for alertID := range m.activeAlerts {
		if !strings.HasPrefix(alertID, unifiedIncidentAlertPrefix) {
			continue
		}
		if _, keep := desired[alertID]; keep {
			continue
		}
		m.clearAlertNoLock(alertID)
	}

	for alertID, alert := range desired {
		if existing, exists := m.activeAlerts[alertID]; exists && existing != nil {
			existing.LastSeen = alert.LastSeen
			existing.Level = alert.Level
			existing.ResourceID = alert.ResourceID
			existing.ResourceName = alert.ResourceName
			existing.Node = alert.Node
			existing.NodeDisplayName = m.resolveNodeDisplayName(alert.Node)
			existing.Instance = alert.Instance
			existing.Message = alert.Message
			existing.Metadata = alert.Metadata
			continue
		}

		m.preserveAlertState(alertID, alert)
		m.activeAlerts[alertID] = alert
		m.recentAlerts[alertID] = alert
		m.historyManager.AddAlert(*alert)
		m.dispatchAlert(alert, false)
	}
}

func resourceSupportsUnifiedIncidentAlerts(resource unifiedresources.Resource) bool {
	switch resource.Type {
	case unifiedresources.ResourceTypeStorage, unifiedresources.ResourceTypePhysicalDisk:
		return true
	case unifiedresources.ResourceTypeAgent:
		return len(resource.Incidents) > 0
	default:
		return false
	}
}

func alertLevelFromIncidentSeverity(level storagehealth.RiskLevel) (AlertLevel, bool) {
	switch level {
	case storagehealth.RiskCritical:
		return AlertLevelCritical, true
	case storagehealth.RiskWarning:
		return AlertLevelWarning, true
	default:
		return "", false
	}
}

func unifiedIncidentAlert(resource unifiedresources.Resource, incident unifiedresources.ResourceIncident, level AlertLevel, now time.Time) *Alert {
	resourceName := unifiedIncidentResourceName(resource)
	nodeName := unifiedIncidentNode(resource)
	instanceName := unifiedIncidentInstance(resource)
	alertType := unifiedIncidentAlertType(resource, incident)
	alertID := unifiedIncidentAlertID(resource, incident)
	message := unifiedIncidentMessage(resource, incident)
	startTime := incident.StartedAt
	if startTime.IsZero() {
		startTime = now
	}

	return &Alert{
		ID:           alertID,
		Type:         alertType,
		Level:        level,
		ResourceID:   resource.ID,
		ResourceName: resourceName,
		Node:         nodeName,
		Instance:     instanceName,
		Message:      message,
		Value:        0,
		Threshold:    0,
		StartTime:    startTime,
		LastSeen:     now,
		Metadata:     unifiedIncidentMetadata(resource, incident, alertType),
	}
}

func unifiedIncidentAlertID(resource unifiedresources.Resource, incident unifiedresources.ResourceIncident) string {
	keyParts := []string{resource.ID, incident.Provider, incident.NativeID, incident.Code}
	return unifiedIncidentAlertPrefix + sanitizeAlertKey(strings.Join(keyParts, "-"))
}

func unifiedIncidentResourceName(resource unifiedresources.Resource) string {
	if resource.Canonical != nil && strings.TrimSpace(resource.Canonical.DisplayName) != "" {
		return strings.TrimSpace(resource.Canonical.DisplayName)
	}
	if strings.TrimSpace(resource.Name) != "" {
		return strings.TrimSpace(resource.Name)
	}
	return string(resource.Type)
}

func unifiedIncidentNode(resource unifiedresources.Resource) string {
	if strings.TrimSpace(resource.ParentName) != "" {
		return strings.TrimSpace(resource.ParentName)
	}
	if resource.Canonical != nil && strings.TrimSpace(resource.Canonical.Hostname) != "" {
		return strings.TrimSpace(resource.Canonical.Hostname)
	}
	return unifiedIncidentResourceName(resource)
}

func unifiedIncidentInstance(resource unifiedresources.Resource) string {
	if resource.Proxmox != nil && strings.TrimSpace(resource.Proxmox.Instance) != "" {
		return strings.TrimSpace(resource.Proxmox.Instance)
	}
	switch {
	case resource.TrueNAS != nil:
		return "TrueNAS"
	case resource.Storage != nil && strings.TrimSpace(resource.Storage.Platform) != "":
		platform := strings.TrimSpace(resource.Storage.Platform)
		if platform == "" {
			return ""
		}
		lower := strings.ToLower(platform)
		return strings.ToUpper(lower[:1]) + lower[1:]
	default:
		return ""
	}
}

func unifiedIncidentAlertType(resource unifiedresources.Resource, incident unifiedresources.ResourceIncident) string {
	code := strings.ToLower(strings.TrimSpace(incident.Code))
	switch resource.Type {
	case unifiedresources.ResourceTypePhysicalDisk:
		if strings.Contains(code, "wear") {
			return "disk-wearout"
		}
		return "disk-health"
	case unifiedresources.ResourceTypeStorage:
		if resource.Storage != nil && resource.Storage.IsZFS {
			return "zfs-pool-state"
		}
		return "storage-incident"
	default:
		return "storage-incident"
	}
}

func unifiedIncidentMessage(resource unifiedresources.Resource, incident unifiedresources.ResourceIncident) string {
	base := strings.TrimSpace(incident.Summary)
	if base == "" {
		base = strings.TrimSpace(incident.Code)
	}
	if resource.Storage == nil || resource.Storage.ConsumerCount <= 0 {
		return base
	}

	summary := unifiedIncidentConsumerSummary(resource.Storage)
	if summary == "" {
		return base
	}
	if base == "" {
		return summary
	}
	return base + ". " + summary
}

func unifiedIncidentConsumerSummary(storage *unifiedresources.StorageMeta) string {
	if storage == nil || storage.ConsumerCount <= 0 {
		return ""
	}

	names := make([]string, 0, len(storage.TopConsumers))
	for _, consumer := range storage.TopConsumers {
		name := strings.TrimSpace(consumer.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
		if len(names) == 3 {
			break
		}
	}

	resourceLabel := "dependent resource"
	if storage.ConsumerCount != 1 {
		resourceLabel = "dependent resources"
	}

	if len(names) == 0 {
		return "Affects " + intLabel(storage.ConsumerCount) + " " + resourceLabel
	}

	if remaining := storage.ConsumerCount - len(names); remaining > 0 {
		return "Affects " + intLabel(storage.ConsumerCount) + " " + resourceLabel + ": " + strings.Join(names, ", ") + ", and " + intLabel(remaining) + " more"
	}

	return "Affects " + intLabel(storage.ConsumerCount) + " " + resourceLabel + ": " + strings.Join(names, ", ")
}

func unifiedIncidentMetadata(resource unifiedresources.Resource, incident unifiedresources.ResourceIncident, alertType string) map[string]interface{} {
	metadata := map[string]interface{}{
		"metric":           alertType,
		"resourceType":     string(resource.Type),
		"incidentProvider": incident.Provider,
		"incidentCode":     incident.Code,
		"incidentNativeID": incident.NativeID,
		"incidentSource":   incident.Source,
	}

	if !incident.StartedAt.IsZero() {
		metadata["incidentStartedAt"] = incident.StartedAt.UTC().Format(time.RFC3339)
	}

	if len(resource.Sources) > 0 {
		sources := make([]string, 0, len(resource.Sources))
		for _, source := range resource.Sources {
			sources = append(sources, string(source))
		}
		metadata["resourceSources"] = sources
	}

	if resource.Storage != nil {
		if platform := strings.TrimSpace(resource.Storage.Platform); platform != "" {
			metadata["storagePlatform"] = platform
		}
		if topology := strings.TrimSpace(resource.Storage.Topology); topology != "" {
			metadata["storageTopology"] = topology
		}
		if protection := strings.TrimSpace(resource.Storage.Protection); protection != "" {
			metadata["storageProtection"] = protection
		}
		if resource.Storage.ConsumerCount > 0 {
			metadata["consumerCount"] = resource.Storage.ConsumerCount
			metadata["consumerImpactSummary"] = unifiedIncidentConsumerSummary(resource.Storage)
		}
		if len(resource.Storage.ConsumerTypes) > 0 {
			metadata["consumerTypes"] = append([]string(nil), resource.Storage.ConsumerTypes...)
		}
		if len(resource.Storage.TopConsumers) > 0 {
			names := make([]string, 0, len(resource.Storage.TopConsumers))
			topConsumers := make([]map[string]interface{}, 0, len(resource.Storage.TopConsumers))
			for _, consumer := range resource.Storage.TopConsumers {
				if name := strings.TrimSpace(consumer.Name); name != "" {
					names = append(names, name)
				}
				topConsumers = append(topConsumers, map[string]interface{}{
					"resourceId":   consumer.ResourceID,
					"resourceType": string(consumer.ResourceType),
					"name":         consumer.Name,
					"diskCount":    consumer.DiskCount,
				})
			}
			if len(names) > 0 {
				metadata["topConsumerNames"] = names
			}
			metadata["topConsumers"] = topConsumers
		}
	}

	if resource.PhysicalDisk != nil {
		if devPath := strings.TrimSpace(resource.PhysicalDisk.DevPath); devPath != "" {
			metadata["device"] = devPath
		}
		if model := strings.TrimSpace(resource.PhysicalDisk.Model); model != "" {
			metadata["model"] = model
		}
		if serial := strings.TrimSpace(resource.PhysicalDisk.Serial); serial != "" {
			metadata["serial"] = serial
		}
		if wwn := strings.TrimSpace(resource.PhysicalDisk.WWN); wwn != "" {
			metadata["wwn"] = wwn
		}
	}

	if resource.TrueNAS != nil {
		metadata["storagePlatform"] = "truenas"
		if hostname := strings.TrimSpace(resource.TrueNAS.Hostname); hostname != "" {
			metadata["hostname"] = hostname
		}
	}

	return metadata
}

func intLabel(value int) string {
	return strconv.Itoa(value)
}
