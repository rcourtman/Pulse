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
	parentResources := make(map[string]unifiedresources.Resource, len(resources))
	childrenByParent := make(map[string][]unifiedresources.Resource)
	for _, resource := range resources {
		parentResources[resource.ID] = resource
		if resource.ParentID != nil && strings.TrimSpace(*resource.ParentID) != "" {
			parentID := strings.TrimSpace(*resource.ParentID)
			childrenByParent[parentID] = append(childrenByParent[parentID], resource)
		}
	}

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
				if shouldSuppressUnifiedIncidentAlert(resource, incident, parentResources, childrenByParent) {
					continue
				}
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

func shouldSuppressUnifiedIncidentAlert(resource unifiedresources.Resource, incident unifiedresources.ResourceIncident, resourcesByID map[string]unifiedresources.Resource, childrenByParent map[string][]unifiedresources.Resource) bool {
	key := unifiedIncidentKey(incident)
	if key == "" {
		return false
	}

	switch resource.Type {
	case unifiedresources.ResourceTypeAgent:
		for _, child := range childrenByParent[resource.ID] {
			if child.Type != unifiedresources.ResourceTypeStorage {
				continue
			}
			if resourceHasIncidentKey(child, key) {
				return true
			}
		}
	case unifiedresources.ResourceTypePhysicalDisk:
		if resource.ParentID == nil || strings.TrimSpace(*resource.ParentID) == "" {
			return false
		}
		parent, ok := resourcesByID[strings.TrimSpace(*resource.ParentID)]
		if !ok || parent.Type != unifiedresources.ResourceTypeStorage {
			return false
		}
		return resourceHasIncidentKey(parent, key)
	case unifiedresources.ResourceTypeStorage:
		if resource.ParentID == nil || strings.TrimSpace(*resource.ParentID) == "" {
			return false
		}
		parent, ok := resourcesByID[strings.TrimSpace(*resource.ParentID)]
		if !ok || parent.Type != unifiedresources.ResourceTypePBS {
			return false
		}
		return resourceHasIncidentKey(parent, key)
	}

	return false
}

func resourceHasIncidentKey(resource unifiedresources.Resource, key string) bool {
	for _, incident := range resource.Incidents {
		if unifiedIncidentKey(incident) == key {
			return true
		}
	}
	return false
}

func unifiedIncidentKey(incident unifiedresources.ResourceIncident) string {
	key := strings.TrimSpace(incident.Provider) + "|" + strings.TrimSpace(incident.NativeID) + "|" + strings.TrimSpace(incident.Code)
	if key == "||" {
		return ""
	}
	return key
}

func resourceSupportsUnifiedIncidentAlerts(resource unifiedresources.Resource) bool {
	switch resource.Type {
	case unifiedresources.ResourceTypeStorage, unifiedresources.ResourceTypePhysicalDisk:
		return true
	case unifiedresources.ResourceTypeAgent:
		return len(resource.Incidents) > 0
	case unifiedresources.ResourceTypePBS:
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
	case resource.PBS != nil:
		return "PBS"
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
	case unifiedresources.ResourceTypePBS:
		return "backup-posture-incident"
	case unifiedresources.ResourceTypeStorage:
		if unifiedresources.IsBackupStorageResource(resource.Storage) {
			return "backup-storage-incident"
		}
		if resource.Storage != nil && resource.Storage.IsZFS {
			return "zfs-pool-state"
		}
		return "storage-incident"
	default:
		return "storage-incident"
	}
}

func unifiedIncidentMessage(resource unifiedresources.Resource, incident unifiedresources.ResourceIncident) string {
	if resource.Type == unifiedresources.ResourceTypePBS {
		return unifiedIncidentPBSMessage(resource, incident)
	}

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

func unifiedIncidentPBSMessage(resource unifiedresources.Resource, incident unifiedresources.ResourceIncident) string {
	base := unifiedIncidentPBSBaseMessage(resource, incident)
	summary := unifiedIncidentPBSDatastoreSummary(resource, incident.Code)
	if summary == "" {
		return base
	}
	if base == "" {
		return summary
	}
	return base + ". " + summary
}

func unifiedIncidentPBSBaseMessage(resource unifiedresources.Resource, incident unifiedresources.ResourceIncident) string {
	name := unifiedIncidentResourceName(resource)
	switch strings.TrimSpace(incident.Code) {
	case "capacity_runway_low":
		return "Backup server " + name + " has datastore capacity risk"
	case "pbs_datastore_error":
		return "Backup server " + name + " has datastore errors"
	case "pbs_datastore_state":
		return "Backup server " + name + " has degraded datastore availability"
	default:
		base := strings.TrimSpace(incident.Summary)
		if base != "" {
			return base
		}
		return "Backup server " + name + " has degraded storage posture"
	}
}

func unifiedIncidentPBSDatastoreSummary(resource unifiedresources.Resource, incidentCode string) string {
	if resource.PBS != nil && strings.TrimSpace(resource.PBS.AffectedDatastoreSummary) != "" {
		return strings.TrimSpace(resource.PBS.AffectedDatastoreSummary)
	}
	if resource.PBS == nil || len(resource.PBS.Datastores) == 0 {
		return ""
	}

	names := affectedPBSDatastoreNames(resource.PBS.Datastores, incidentCode)
	if len(names) == 0 {
		return ""
	}

	label := "backup datastore"
	if len(names) != 1 {
		label = "backup datastores"
	}
	return "Affects " + intLabel(len(names)) + " " + label + ": " + strings.Join(names, ", ")
}

func affectedPBSDatastoreNames(datastores []unifiedresources.PBSDatastoreMeta, incidentCode string) []string {
	names := make([]string, 0, len(datastores))
	for _, datastore := range datastores {
		if !pbsDatastoreMatchesIncident(datastore, incidentCode) {
			continue
		}
		name := strings.TrimSpace(datastore.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}

func pbsDatastoreMatchesIncident(datastore unifiedresources.PBSDatastoreMeta, incidentCode string) bool {
	switch strings.TrimSpace(incidentCode) {
	case "capacity_runway_low":
		usage := datastore.UsagePercent
		if usage <= 0 && datastore.Total > 0 {
			usage = (float64(datastore.Used) / float64(datastore.Total)) * 100
		}
		return usage >= 90
	case "pbs_datastore_error":
		return strings.TrimSpace(datastore.Error) != ""
	case "pbs_datastore_state":
		switch strings.ToUpper(strings.TrimSpace(datastore.Status)) {
		case "OFFLINE", "UNAVAILABLE", "ERROR", "FAILED", "DEGRADED", "WARN", "WARNING", "READ_ONLY":
			return true
		default:
			return false
		}
	default:
		return strings.TrimSpace(datastore.Error) != "" || pbsDatastoreMatchesIncident(datastore, "pbs_datastore_state") || pbsDatastoreMatchesIncident(datastore, "capacity_runway_low")
	}
}

func unifiedIncidentConsumerSummary(storage *unifiedresources.StorageMeta) string {
	if storage == nil || storage.ConsumerCount <= 0 {
		return ""
	}
	if summary := strings.TrimSpace(storage.ConsumerImpactSummary); summary != "" {
		return summary
	}

	return unifiedresources.StorageConsumerImpactSummary(storage)
}

func unifiedIncidentDependentConsumerSummary(storage *unifiedresources.StorageMeta) string {
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

func unifiedIncidentBackupConsumerSummary(storage *unifiedresources.StorageMeta) string {
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

	workloadLabel := "protected workload"
	if storage.ConsumerCount != 1 {
		workloadLabel = "protected workloads"
	}

	if len(names) == 0 {
		return "Puts backups for " + intLabel(storage.ConsumerCount) + " " + workloadLabel + " at risk"
	}

	if remaining := storage.ConsumerCount - len(names); remaining > 0 {
		return "Puts backups for " + intLabel(storage.ConsumerCount) + " " + workloadLabel + " at risk: " + strings.Join(names, ", ") + ", and " + intLabel(remaining) + " more"
	}

	return "Puts backups for " + intLabel(storage.ConsumerCount) + " " + workloadLabel + " at risk: " + strings.Join(names, ", ")
}

func unifiedIncidentMetadata(resource unifiedresources.Resource, incident unifiedresources.ResourceIncident, alertType string) map[string]interface{} {
	metadata := map[string]interface{}{
		"metric":           alertType,
		"resourceType":     string(resource.Type),
		"incidentProvider": incident.Provider,
		"incidentCode":     incident.Code,
		"incidentCategory": unifiedresources.IncidentCategoryForResource(&resource, incident),
		"incidentLabel":    unifiedresources.IncidentLabelForResource(&resource, incident, unifiedresources.IncidentCategoryForResource(&resource, incident)),
		"incidentNativeID": incident.NativeID,
		"incidentSource":   incident.Source,
	}
	if urgency, action := unifiedresources.IncidentActionForResource(&resource, incident, unifiedresources.IncidentCategoryForResource(&resource, incident)); urgency != "" {
		metadata["incidentUrgency"] = urgency
		if action != "" {
			metadata["incidentAction"] = action
		}
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
		if resource.Storage.Risk != nil {
			riskCodes, protectionReduced, rebuildInProgress, protectionSummary, rebuildSummary := storageRiskAlertSemantics(resource.Storage.Risk)
			if len(riskCodes) > 0 {
				metadata["storageRiskCodes"] = riskCodes
			}
			if protectionReduced {
				metadata["protectionReduced"] = true
			}
			if rebuildInProgress {
				metadata["rebuildInProgress"] = true
			}
			if protectionSummary != "" {
				metadata["protectionSummary"] = protectionSummary
			}
			if rebuildSummary != "" {
				metadata["rebuildSummary"] = rebuildSummary
			}
		}
		if resource.Storage.ConsumerCount > 0 {
			metadata["consumerCount"] = resource.Storage.ConsumerCount
			metadata["consumerImpactSummary"] = unifiedIncidentConsumerSummary(resource.Storage)
		}
		if unifiedresources.IsBackupStorageResource(resource.Storage) {
			metadata["backupTarget"] = true
			if resource.Storage.ConsumerCount > 0 {
				metadata["protectedWorkloadCount"] = resource.Storage.ConsumerCount
				metadata["protectedWorkloadSummary"] = unifiedIncidentBackupConsumerSummary(resource.Storage)
			}
		}
		if len(resource.Storage.ConsumerTypes) > 0 {
			metadata["consumerTypes"] = append([]string(nil), resource.Storage.ConsumerTypes...)
			if unifiedresources.IsBackupStorageResource(resource.Storage) {
				metadata["protectedWorkloadTypes"] = append([]string(nil), resource.Storage.ConsumerTypes...)
			}
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
				if unifiedresources.IsBackupStorageResource(resource.Storage) {
					metadata["protectedWorkloadNames"] = append([]string(nil), names...)
				}
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

	if resource.PBS != nil {
		metadata["storagePlatform"] = "pbs"
		metadata["backupServer"] = true
		metadata["datastoreCount"] = resource.PBS.DatastoreCount
		if hostname := strings.TrimSpace(resource.PBS.Hostname); hostname != "" {
			metadata["hostname"] = hostname
		}
		if resource.PBS.StorageRisk != nil {
			riskCodes, protectionReduced, rebuildInProgress, protectionSummary, rebuildSummary := storageRiskAlertSemantics(resource.PBS.StorageRisk)
			if len(riskCodes) > 0 {
				metadata["storageRiskCodes"] = riskCodes
			}
			if protectionReduced {
				metadata["protectionReduced"] = true
			}
			if rebuildInProgress {
				metadata["rebuildInProgress"] = true
			}
			if protectionSummary != "" {
				metadata["protectionSummary"] = protectionSummary
			}
			if rebuildSummary != "" {
				metadata["rebuildSummary"] = rebuildSummary
			}
		}
		if names := affectedPBSDatastoreNames(resource.PBS.Datastores, incident.Code); len(names) > 0 {
			metadata["affectedDatastoreCount"] = len(names)
			metadata["affectedDatastores"] = append([]string(nil), names...)
			metadata["backupServerPostureSummary"] = "Affects " + intLabel(len(names)) + " backup datastore"
			if len(names) != 1 {
				metadata["backupServerPostureSummary"] = "Affects " + intLabel(len(names)) + " backup datastores"
			}
		}
		if resource.PBS.ProtectedWorkloadCount > 0 {
			metadata["consumerCount"] = resource.PBS.ProtectedWorkloadCount
			metadata["protectedWorkloadCount"] = resource.PBS.ProtectedWorkloadCount
			metadata["consumerImpactSummary"] = unifiedIncidentPBSProtectedWorkloadSummary(resource.PBS)
			metadata["protectedWorkloadSummary"] = unifiedIncidentPBSProtectedWorkloadSummary(resource.PBS)
		}
		if len(resource.PBS.ProtectedWorkloadTypes) > 0 {
			metadata["consumerTypes"] = append([]string(nil), resource.PBS.ProtectedWorkloadTypes...)
			metadata["protectedWorkloadTypes"] = append([]string(nil), resource.PBS.ProtectedWorkloadTypes...)
		}
		if len(resource.PBS.ProtectedWorkloadNames) > 0 {
			metadata["topConsumerNames"] = append([]string(nil), resource.PBS.ProtectedWorkloadNames...)
			metadata["protectedWorkloadNames"] = append([]string(nil), resource.PBS.ProtectedWorkloadNames...)
		}
	}

	return metadata
}

func intLabel(value int) string {
	return strconv.Itoa(value)
}

func unifiedIncidentPBSProtectedWorkloadSummary(pbs *unifiedresources.PBSData) string {
	if pbs == nil || pbs.ProtectedWorkloadCount <= 0 {
		return ""
	}
	if strings.TrimSpace(pbs.ProtectedWorkloadSummary) != "" {
		return strings.TrimSpace(pbs.ProtectedWorkloadSummary)
	}

	workloadLabel := "protected workload"
	if pbs.ProtectedWorkloadCount != 1 {
		workloadLabel = "protected workloads"
	}
	if len(pbs.ProtectedWorkloadNames) == 0 {
		return "Puts backups for " + intLabel(pbs.ProtectedWorkloadCount) + " " + workloadLabel + " at risk"
	}
	names := append([]string(nil), pbs.ProtectedWorkloadNames...)
	if len(names) > 3 {
		names = names[:3]
	}
	if remaining := pbs.ProtectedWorkloadCount - len(names); remaining > 0 {
		return "Puts backups for " + intLabel(pbs.ProtectedWorkloadCount) + " " + workloadLabel + " at risk: " + strings.Join(names, ", ") + ", and " + intLabel(remaining) + " more"
	}
	return "Puts backups for " + intLabel(pbs.ProtectedWorkloadCount) + " " + workloadLabel + " at risk: " + strings.Join(names, ", ")
}

func storageRiskAlertSemantics(risk *unifiedresources.StorageRisk) ([]string, bool, bool, string, string) {
	return unifiedresources.StorageRiskSemantics(risk)
}
