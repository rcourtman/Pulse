package alerts

import (
	"strings"
	"time"

	alertconfig "github.com/rcourtman/pulse-go-rewrite/internal/alerts/config"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rs/zerolog/log"
)

// UpdateConfig updates the alert configuration.
func (m *Manager) UpdateConfig(config AlertConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clients and rollback-era config writers may not know about the additive
	// identity schema marker. Never lower a version already held by this
	// process; an older binary can still omit it on disk and the migration will
	// safely rerun after a later upgrade.
	if config.IdentitySchemaVersion < m.config.IdentitySchemaVersion {
		config.IdentitySchemaVersion = m.config.IdentitySchemaVersion
	}

	// Preserve activation state/time when clients update the config without including it.
	// This avoids unintentionally resetting alerts to pending review when saving thresholds.
	if config.ActivationState == "" && m.config.ActivationState != "" {
		config.ActivationState = m.config.ActivationState
		if config.ActivationTime == nil && m.config.ActivationTime != nil {
			config.ActivationTime = m.config.ActivationTime
		}
	}

	// Normalize all config sections
	alertconfig.NormalizeAlertConfigAliases(&config)
	alertconfig.NormalizeStorageDefaults(&config)
	alertconfig.NormalizeDockerDefaults(&config)
	alertconfig.NormalizePMGDefaults(&config)
	alertconfig.NormalizePBSDefaults(&config)
	alertconfig.NormalizeSnapshotDefaults(&config)
	alertconfig.NormalizeBackupDefaults(&config)
	alertconfig.NormalizeRecoveryOverrides(&config)
	alertconfig.NormalizeNodeDefaults(&config)
	alertconfig.NormalizeAgentDefaults(&config)
	alertconfig.NormalizeKubernetesDefaults(&config)
	alertconfig.NormalizeTrueNASDefaults(&config)
	alertconfig.NormalizeVMwareDefaults(&config)
	alertconfig.NormalizeGeneralSettings(&config)
	alertconfig.NormalizeTimeThresholds(&config)
	config.MetricEvaluationWindows = alertconfig.NormalizeMetricEvaluationWindows(config.MetricEvaluationWindows)

	config.GuestDefaults.PoweredOffSeverity = alertconfig.NormalizePoweredOffSeverity(config.GuestDefaults.PoweredOffSeverity)
	config.NodeDefaults.PoweredOffSeverity = alertconfig.NormalizePoweredOffSeverity(config.NodeDefaults.PoweredOffSeverity)
	config.DockerIgnoredContainerPrefixes = alertconfig.NormalizeDockerIgnoredPrefixes(config.DockerIgnoredContainerPrefixes)
	config.Schedule.InitialNotify = alertconfig.NormalizeNotificationDeliveryTarget(config.Schedule.InitialNotify)
	alertconfig.NormalizeEscalationConfig(&config.Schedule.Escalation)

	// Migration logic for activation state (backward compatibility)
	m.migrateActivationState(&config)

	// Validate hysteresis thresholds to prevent stuck alerts
	alertconfig.ValidateHysteresisThresholds(&config)

	// Validate timezone if quiet hours are enabled
	alertconfig.ValidateQuietHoursTimezone(&config)

	m.config = config
	normalizeOverrides(m.config.Overrides)

	// Update cached quiet hours location
	if m.config.Schedule.QuietHours.Enabled && m.config.Schedule.QuietHours.Timezone != "" {
		loc, err := time.LoadLocation(m.config.Schedule.QuietHours.Timezone)
		if err == nil {
			m.quietHoursLoc = loc
		} else {
			m.quietHoursLoc = time.Local
		}
	} else {
		m.quietHoursLoc = time.Local
	}

	if !m.config.SnapshotDefaults.Enabled {
		m.clearSnapshotAlertsForInstanceLocked("")
	}
	if !m.config.BackupDefaults.Enabled {
		m.clearBackupAlertsLocked()
	}

	m.applyGlobalOfflineSettingsLocked()

	log.Info().
		Bool("enabled", config.Enabled).
		Interface("guestDefaults", config.GuestDefaults).
		Msg("Alert configuration updated")

	// Re-evaluate active alerts against new thresholds
	m.reevaluateActiveAlertsLocked()
}

// migrateActivationState handles backward compatibility for activation state.
func (m *Manager) migrateActivationState(config *AlertConfig) {
	if config.ActivationState == "" {
		// Determine if this is an existing installation or new.
		// Existing installations have active alerts already.
		isExistingInstall := len(m.activeAlerts) > 0 || len(config.Overrides) > 0
		if isExistingInstall {
			// Existing install: auto-activate to preserve behavior.
			config.ActivationState = ActivationActive
			now := time.Now()
			config.ActivationTime = &now
			log.Info().Msg("migrating existing installation to active alert state")
		} else {
			// New install: start in pending review.
			config.ActivationState = ActivationPending
			log.Info().Msg("new installation: alerts pending activation")
		}
	}
}

// normalizeOverrides normalizes all threshold overrides.
func normalizeOverrides(overrides map[string]ThresholdConfig) {
	normalized := make(map[string]ThresholdConfig, len(overrides))
	priorityByKey := make(map[string]int, len(overrides))
	for id, override := range overrides {
		// An unset powered-off severity means "follow the global default".
		// Normalizing it here would stamp every override with an explicit
		// "warning" and silently downgrade guests whose global default is
		// critical, so only normalize values the user actually set.
		if override.PoweredOffSeverity != "" {
			override.PoweredOffSeverity = NormalizePoweredOffSeverity(override.PoweredOffSeverity)
		}
		if override.Usage != nil {
			override.Usage = ensureHysteresisThreshold(override.Usage)
		}
		normalizedKey := id
		if ident, ok := parseCanonicalGuestKey(id); ok {
			if stableKey := clusteredGuestOverrideKey(ident); stableKey != "" {
				normalizedKey = stableKey
			}
		}
		priority := 0
		if normalizedKey == id {
			priority = 1
		}
		if existingPriority, exists := priorityByKey[normalizedKey]; exists && existingPriority > priority {
			continue
		}
		priorityByKey[normalizedKey] = priority
		normalized[normalizedKey] = override
	}
	for key := range overrides {
		delete(overrides, key)
	}
	for key, override := range normalized {
		overrides[key] = override
	}
}

// applyGlobalOfflineSettingsLocked clears tracking and active alerts for globally disabled offline detectors.
// Caller must hold m.mu.
func (m *Manager) applyGlobalOfflineSettingsLocked() {
	_, nodesOfflineDisabled := m.alertPolicyTypeSwitchesNoLock("node")
	_, pbsOfflineDisabled := m.alertPolicyTypeSwitchesNoLock("pbs")
	_, guestsOfflineDisabled := m.alertPolicyTypeSwitchesNoLock("vm")
	_, dockerHostsOfflineDisabled := m.alertPolicyTypeSwitchesNoLock("docker-host")
	containersDisabled, _ := m.alertPolicyTypeSwitchesNoLock("app-container")
	servicesDisabled, _ := m.alertPolicyTypeSwitchesNoLock("docker-service")
	kubernetesDisabled, _ := m.alertPolicyTypeSwitchesNoLock("k8s-node")
	truenasDisabled, _ := m.alertPolicyTypeSwitchesNoLock("truenas-system")
	vmwareDisabled, _ := m.alertPolicyTypeSwitchesNoLock("vmware-host")

	if nodesOfflineDisabled {
		var nodeAlerts []string
		for storageKey, alert := range m.activeAlerts {
			if alert == nil {
				continue
			}
			resourceType, _ := alert.Metadata["resourceType"].(string)
			isCanonicalNodeConnectivity := alert.CanonicalKind == string(alertspecs.AlertSpecKindConnectivity) && resourceType == "node"
			isLegacyNodeConnectivity := alert.Type == "connectivity" && (strings.HasPrefix(alert.ID, "node-offline-") || strings.HasPrefix(storageKey, "node-offline-"))
			if isCanonicalNodeConnectivity || isLegacyNodeConnectivity {
				nodeAlerts = append(nodeAlerts, storageKey)
			}
		}
		for _, alertID := range nodeAlerts {
			m.clearAlertNoLock(alertID)
		}
	}

	if pbsOfflineDisabled {
		var pbsAlerts []string
		for storageKey, alert := range m.activeAlerts {
			if alert != nil && alert.CanonicalKind == string(alertspecs.AlertSpecKindConnectivity) {
				if resourceType, _ := alert.Metadata["resourceType"].(string); resourceType == "pbs" {
					pbsAlerts = append(pbsAlerts, storageKey)
				}
			}
		}
		for _, alertID := range pbsAlerts {
			m.clearAlertNoLock(alertID)
		}
	}

	if guestsOfflineDisabled {
		var guestAlerts []string
		for storageKey, alert := range m.activeAlerts {
			if alert != nil && alert.CanonicalKind == string(alertspecs.AlertSpecKindPoweredState) {
				guestAlerts = append(guestAlerts, storageKey)
			}
		}
		for _, alertID := range guestAlerts {
			m.clearAlertNoLock(alertID)
		}
	}

	if dockerHostsOfflineDisabled {
		var hostAlerts []string
		for storageKey, alert := range m.activeAlerts {
			if alert != nil && alert.CanonicalKind == string(alertspecs.AlertSpecKindConnectivity) {
				if resourceType, _ := alert.Metadata["resourceType"].(string); resourceType == "docker-host" {
					hostAlerts = append(hostAlerts, storageKey)
				}
			}
		}
		for _, alertID := range hostAlerts {
			m.clearAlertNoLock(alertID)
		}
	}

	if containersDisabled {
		var containerAlerts []string
		for storageKey, alert := range m.activeAlerts {
			id := effectiveAlertID(alert, storageKey)
			if strings.HasPrefix(id, "docker-container-") || isDockerContainerAlert(alert) {
				containerAlerts = append(containerAlerts, id)
			}
		}
		for _, alertID := range containerAlerts {
			m.clearAlertNoLock(alertID)
		}
		m.dockerRestartTracking = make(map[string]*dockerRestartRecord)
		m.dockerUpdateFirstSeen = make(map[string]time.Time)
		m.dockerUpdateFirstSeenByIdentity = make(map[string]time.Time)
	}
	if m.config.DockerDefaults.UpdateAlertDelayHours < 0 && !containersDisabled {
		m.clearDockerContainerUpdateAlertsLocked()
		m.dockerUpdateFirstSeen = make(map[string]time.Time)
		m.dockerUpdateFirstSeenByIdentity = make(map[string]time.Time)
	}
	if servicesDisabled {
		var serviceAlerts []string
		for storageKey, alert := range m.activeAlerts {
			id := effectiveAlertID(alert, storageKey)
			if strings.HasPrefix(id, "docker-service-") || isDockerServiceAlert(alert) {
				serviceAlerts = append(serviceAlerts, id)
			}
		}
		for _, alertID := range serviceAlerts {
			m.clearAlertNoLock(alertID)
		}
	}

	if kubernetesDisabled || truenasDisabled || vmwareDisabled {
		var platformAlerts []string
		for storageKey, alert := range m.activeAlerts {
			primaryType := alertPrimaryResourceType(alert)
			if primaryType == "" {
				continue
			}
			switch {
			case kubernetesDisabled && isUnifiedKubernetesAlertType(primaryType):
				platformAlerts = append(platformAlerts, effectiveAlertID(alert, storageKey))
			case truenasDisabled && isUnifiedTrueNASAlertType(primaryType):
				platformAlerts = append(platformAlerts, effectiveAlertID(alert, storageKey))
			case vmwareDisabled && isUnifiedVMwareAlertType(primaryType):
				platformAlerts = append(platformAlerts, effectiveAlertID(alert, storageKey))
			}
		}
		for _, alertID := range platformAlerts {
			m.clearAlertNoLock(alertID)
		}
	}
}

// dockerAlertResourcePath returns the portion of the alert's resource ID after
// the "docker:" scheme, matching the IDs built by DockerResourceID and
// DockerServiceResourceID. Canonical alerts are stored under
// "<resourceID>::<specID>" state IDs, so the legacy "docker-container-" /
// "docker-service-" alert-ID prefixes never match them.
func dockerAlertResourcePath(alert *Alert) (string, bool) {
	if alert == nil {
		return "", false
	}
	resourceID := strings.TrimSpace(alert.ResourceID)
	if !strings.HasPrefix(resourceID, "docker:") {
		return "", false
	}
	return strings.TrimPrefix(resourceID, "docker:"), true
}

func isDockerContainerAlert(alert *Alert) bool {
	path, ok := dockerAlertResourcePath(alert)
	if !ok {
		return false
	}
	// "hostID/containerID" has a path separator; host-level IDs ("hostID") do
	// not, and services use "hostID/service/serviceID".
	return strings.Contains(path, "/") && !strings.Contains(path, "/service/")
}

func isDockerServiceAlert(alert *Alert) bool {
	if alert != nil && strings.HasPrefix(strings.TrimSpace(alert.ResourceID), "docker-service:") {
		return true
	}
	path, ok := dockerAlertResourcePath(alert)
	if !ok {
		return false
	}
	return strings.Contains(path, "/service/")
}

func alertPrimaryResourceType(alert *Alert) string {
	if alert == nil || alert.Metadata == nil {
		return ""
	}
	metaType, ok := alert.Metadata["resourceType"].(string)
	if !ok {
		return ""
	}
	keys := CanonicalResourceTypeKeys(metaType)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

func alertResourceTypeKeysContain(keys []string, target string) bool {
	for _, key := range keys {
		if key == target {
			return true
		}
	}
	return false
}

// reevaluateActiveAlertsLocked re-evaluates all active alerts against the current configuration.
// This should only be called with m.mu already locked.
func (m *Manager) reevaluateActiveAlertsLocked() {
	if len(m.activeAlerts) == 0 {
		return
	}

	// Track alerts that should be resolved.
	alertsToResolve := make([]string, 0)

	for storageKey, alert := range m.activeAlerts {
		alertID := effectiveAlertID(alert, storageKey)
		backfillCanonicalIdentity(alert)
		if alert.Type == "docker-container-update" || strings.HasPrefix(alertID, "docker-container-update-") {
			if m.shouldResolveDockerContainerUpdateAlertLocked(alert) {
				alertsToResolve = append(alertsToResolve, alertID)
			}
			continue
		}
		resourceID := alert.ResourceID
		metricType := alert.Type
		if resourceID == "" || metricType == "" {
			parts := strings.Split(alertID, "-")
			if len(parts) < 2 {
				continue
			}
			metricType = parts[len(parts)-1]
			resourceID = strings.Join(parts[:len(parts)-1], "-")
		}

		var threshold *HysteresisThreshold

		resourceTypeMeta := ""
		if alert.Metadata != nil {
			if metaType, ok := alert.Metadata["resourceType"].(string); ok {
				resourceTypeMeta = alertconfig.CanonicalAlertResourceType(metaType)
			}
		}
		resourceTypeKeys := CanonicalResourceTypeKeys(resourceTypeMeta)
		primaryResourceType := ""
		if len(resourceTypeKeys) > 0 {
			primaryResourceType = resourceTypeKeys[0]
		}

		if connectionType, ok := connectionTypeFromAlert(alert); ok {
			policyResourceID := metadataStringValue(alert.Metadata, "policyResourceID")
			if m.connectionDegradedPolicyDisabledNoLock(resourceID, policyResourceID, connectionType) {
				alertsToResolve = append(alertsToResolve, alertID)
			}
			continue
		}

		if alert.Type == "queue-depth" || alert.Type == "queue-deferred" || alert.Type == "queue-hold" || alert.Type == "message-age" {
			if allDisabled, _ := m.alertPolicyTypeSwitchesNoLock("pmg"); allDisabled {
				alertsToResolve = append(alertsToResolve, alertID)
				continue
			}
		}

		handledModernPlatformType := false
		if isUnifiedModernPlatformAlertType(primaryResourceType) {
			handledModernPlatformType = true
			if m.unifiedPlatformAlertsDisabledNoLock(primaryResourceType) {
				alertsToResolve = append(alertsToResolve, alertID)
				continue
			}
			thresholds := m.resolveResourceThresholds(primaryResourceType, resourceID)
			if thresholds.Disabled {
				alertsToResolve = append(alertsToResolve, alertID)
				continue
			}
			threshold = getThresholdForMetric(thresholds, metricType)
		}

		isAgentResource := alertResourceTypeKeysContain(resourceTypeKeys, "agent")
		if !handledModernPlatformType && isAgentResource {
			if allDisabled, _ := m.alertPolicyTypeSwitchesNoLock("agent"); allDisabled {
				alertsToResolve = append(alertsToResolve, alertID)
				continue
			}
			thresholds := m.resolveHostAlertThresholdsNoLock(alert, resourceID)
			if thresholds.Disabled {
				alertsToResolve = append(alertsToResolve, alertID)
				continue
			}
			threshold = getThresholdForMetric(thresholds, metricType)
		}

		if alert.Type == "docker-host-offline" ||
			alert.Type == "docker-container-health" ||
			alert.Type == "docker-container-state" ||
			alert.Type == "docker-container-restart-loop" ||
			alert.Type == "docker-container-oom-kill" ||
			alert.Type == "docker-container-memory-limit" {
			continue
		}

		if resourceTypeMeta == "docker-host" {
			if allDisabled, _ := m.alertPolicyTypeSwitchesNoLock("docker-host"); allDisabled {
				alertsToResolve = append(alertsToResolve, alertID)
				continue
			}
			continue
		}
		if resourceTypeMeta == "app-container" {
			if allDisabled, _ := m.alertPolicyTypeSwitchesNoLock("app-container"); allDisabled {
				alertsToResolve = append(alertsToResolve, alertID)
				continue
			}
			containerName := strings.TrimSpace(alert.ResourceName)
			containerID := ""
			hostID := ""
			if alert.Metadata != nil {
				if val, ok := alert.Metadata["containerId"].(string); ok {
					containerID = strings.ToLower(strings.TrimSpace(val))
				}
				if val, ok := alert.Metadata["containerName"].(string); ok && containerName == "" {
					containerName = strings.TrimSpace(val)
				}
				if val, ok := alert.Metadata["hostId"].(string); ok {
					hostID = val
				}
			}
			if matchesDockerIgnoredPrefix(strings.ToLower(containerName), containerID, m.config.DockerIgnoredContainerPrefixes) {
				alertsToResolve = append(alertsToResolve, alertID)
				continue
			}
			thresholds := ThresholdConfig{
				CPU:    cloneThreshold(&m.config.DockerDefaults.CPU),
				Memory: cloneThreshold(&m.config.DockerDefaults.Memory),
				Disk:   cloneThreshold(&m.config.DockerDefaults.Disk),
			}
			if override, exists := m.lookupDockerContainerOverrideNoLock(hostID, containerName, resourceID); exists {
				if override.Disabled {
					alertsToResolve = append(alertsToResolve, alertID)
					continue
				}
				thresholds = m.applyThresholdOverride(thresholds, override)
			}
			threshold = getThresholdForMetric(thresholds, metricType)
		}

		isNodeResource := primaryResourceType == "" || primaryResourceType == "node"
		isStorageResource := alertResourceTypeKeysContain(resourceTypeKeys, "storage")
		if threshold == nil && !handledModernPlatformType && isNodeResource && !strings.Contains(resourceID, ":") && (alert.Instance == "Node" || alert.Instance == alert.Node) {
			if allDisabled, _ := m.alertPolicyTypeSwitchesNoLock("node"); allDisabled {
				alertsToResolve = append(alertsToResolve, alertID)
				continue
			}
			thresholds := m.resolveResourceThresholds("node", resourceID)
			if thresholds.Disabled {
				alertsToResolve = append(alertsToResolve, alertID)
				continue
			}
			threshold = getThresholdForMetric(thresholds, metricType)
		} else if threshold == nil && !handledModernPlatformType && (isStorageResource || alert.Instance == "Storage" || strings.Contains(alert.ResourceID, ":storage/")) {
			if allDisabled, _ := m.alertPolicyTypeSwitchesNoLock("storage"); allDisabled {
				alertsToResolve = append(alertsToResolve, alertID)
				continue
			}
			thresholds := m.resolveResourceThresholds("storage", resourceID)
			if thresholds.Disabled {
				alertsToResolve = append(alertsToResolve, alertID)
				continue
			}
			threshold = getThresholdForMetric(thresholds, metricType)
		} else if threshold == nil && !handledModernPlatformType && (resourceTypeMeta == "pbs" || alert.Instance == "PBS") {
			if allDisabled, _ := m.alertPolicyTypeSwitchesNoLock("pbs"); allDisabled {
				alertsToResolve = append(alertsToResolve, alertID)
				continue
			}
			thresholds := m.resolveResourceThresholds("pbs", resourceID)
			if thresholds.Disabled {
				alertsToResolve = append(alertsToResolve, alertID)
				continue
			}
			threshold = getThresholdForMetric(thresholds, metricType)
		}

		if threshold == nil && !handledModernPlatformType {
			if allDisabled, _ := m.alertPolicyTypeSwitchesNoLock("vm"); allDisabled {
				alertsToResolve = append(alertsToResolve, alertID)
				continue
			}

			guestThresholds := m.getGuestThresholds(guestSnapshotFromAlert(alert, resourceID), resourceID)
			if guestThresholds.Disabled {
				alertsToResolve = append(alertsToResolve, alertID)
				continue
			}

			switch alert.Type {
			case "snapshot-age":
				if !snapshotAlertStillTriggered(alert, m.resolvedSnapshotAlertConfigNoLock(guestThresholds)) {
					alertsToResolve = append(alertsToResolve, alertID)
				}
				continue
			case "backup-age":
				if !backupAlertStillTriggered(alert, m.resolvedBackupAlertConfigNoLock(guestThresholds)) {
					alertsToResolve = append(alertsToResolve, alertID)
				}
				continue
			case "powered-off":
				if guestThresholds.DisableConnectivity {
					alertsToResolve = append(alertsToResolve, alertID)
					continue
				}
				alert.Level = NormalizePoweredOffSeverity(guestThresholds.PoweredOffSeverity)
				continue
			}

			threshold = getThresholdForMetric(guestThresholds, metricType)
		}

		// Provider-owned incidents and other non-metric alerts are not threshold
		// observations. An unrelated config save must leave their lifecycle and
		// acknowledgement state intact; only their evaluator (or an explicit
		// resource-disable policy handled above) can resolve them.
		if !isMetricThresholdAlertType(metricType) {
			continue
		}

		if threshold == nil || threshold.Trigger <= 0 {
			alertsToResolve = append(alertsToResolve, alertID)
			continue
		}

		clearThreshold := threshold.Clear
		if clearThreshold <= 0 {
			clearThreshold = threshold.Trigger
		}

		if alert.Value <= clearThreshold {
			alertsToResolve = append(alertsToResolve, alertID)
			log.Info().
				Str("alertID", alertID).
				Float64("value", alert.Value).
				Float64("oldThreshold", alert.Threshold).
				Float64("newClearThreshold", clearThreshold).
				Msg("Resolving alert due to threshold change")
		} else if alert.Value < threshold.Trigger {
			alertsToResolve = append(alertsToResolve, alertID)
			log.Info().
				Str("alertID", alertID).
				Float64("value", alert.Value).
				Float64("newTrigger", threshold.Trigger).
				Float64("newClear", clearThreshold).
				Msg("Resolving alert - value now below trigger threshold after config change")
		}
	}

	for _, alertID := range alertsToResolve {
		if alert, exists := m.getActiveAlertNoLock(alertID); exists {
			resolvedAlert := m.newResolvedAlert(alert, time.Now(), nil)

			m.mirrorForgetAlertNoLock(alert)
			m.removeActiveAlertNoLock(alertID)
			m.addRecentlyResolvedWithPrimaryLock(resolvedAlert)

			log.Info().
				Str("alertID", alertID).
				Msg("Alert auto-resolved after configuration change")

			m.safeCallResolvedAlertCallback(resolvedAlert.Alert, alertID, true)
		}
	}

	if len(alertsToResolve) > 0 {
		m.saveActiveAlertsAsync("config update")
	}
}

// GetConfig returns the current alert configuration.
func (m *Manager) GetConfig() AlertConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

func cloneThreshold(threshold *HysteresisThreshold) *HysteresisThreshold {
	if threshold == nil {
		return nil
	}
	clone := *threshold
	return &clone
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneSnapshotConfig(cfg *SnapshotAlertConfig) *SnapshotAlertConfig {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	return &clone
}

func cloneBackupConfig(cfg *BackupAlertConfig) *BackupAlertConfig {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	if cfg.AlertOrphaned != nil {
		value := *cfg.AlertOrphaned
		clone.AlertOrphaned = &value
	}
	if len(cfg.IgnoreVMIDs) > 0 {
		clone.IgnoreVMIDs = append([]string(nil), cfg.IgnoreVMIDs...)
	}
	return &clone
}

func cloneThresholdConfig(cfg ThresholdConfig) ThresholdConfig {
	clone := cfg
	clone.CPU = cloneThreshold(cfg.CPU)
	clone.Memory = cloneThreshold(cfg.Memory)
	clone.Disk = cloneThreshold(cfg.Disk)
	clone.DiskRead = cloneThreshold(cfg.DiskRead)
	clone.DiskWrite = cloneThreshold(cfg.DiskWrite)
	clone.NetworkIn = cloneThreshold(cfg.NetworkIn)
	clone.NetworkOut = cloneThreshold(cfg.NetworkOut)
	clone.Temperature = cloneThreshold(cfg.Temperature)
	clone.DiskTemperature = cloneThreshold(cfg.DiskTemperature)
	clone.SMARTHealthFailure = cloneInt(cfg.SMARTHealthFailure)
	clone.SMARTReallocated = cloneInt64(cfg.SMARTReallocated)
	clone.SMARTPending = cloneInt64(cfg.SMARTPending)
	clone.SMARTUncorrectable = cloneInt64(cfg.SMARTUncorrectable)
	clone.SMARTMediaErrors = cloneInt64(cfg.SMARTMediaErrors)
	clone.SMARTCRCErrorDelta = cloneInt64(cfg.SMARTCRCErrorDelta)
	clone.SMARTLifeWarning = cloneInt(cfg.SMARTLifeWarning)
	clone.SMARTLifeCritical = cloneInt(cfg.SMARTLifeCritical)
	clone.SMARTSpareWarning = cloneInt(cfg.SMARTSpareWarning)
	clone.SMARTSpareCritical = cloneInt(cfg.SMARTSpareCritical)
	clone.Usage = cloneThreshold(cfg.Usage)
	clone.Backup = cloneBackupConfig(cfg.Backup)
	clone.Snapshot = cloneSnapshotConfig(cfg.Snapshot)
	clone.Note = cloneStringPtr(cfg.Note)
	return clone
}

func (m *Manager) applyThresholdOverride(base ThresholdConfig, override ThresholdConfig) ThresholdConfig {
	result := base

	if override.Disabled {
		result.Disabled = true
	}
	if override.DisableConnectivity {
		result.DisableConnectivity = true
	} else if override.PoweredOffSeverity != "" {
		// An override carrying an explicit powered-off severity is that row's
		// offline control set to warning/critical, so it re-enables
		// connectivity alerts over a disabled default. A per-row "off" is
		// stored as DisableConnectivity=true and takes the branch above.
		result.DisableConnectivity = false
	}
	if override.PoweredOffSeverity != "" {
		result.PoweredOffSeverity = NormalizePoweredOffSeverity(override.PoweredOffSeverity)
	}

	if override.CPU != nil {
		result.CPU = ensureHysteresisThreshold(cloneThreshold(override.CPU))
	}
	if override.Memory != nil {
		result.Memory = ensureHysteresisThreshold(cloneThreshold(override.Memory))
	}
	if override.Disk != nil {
		result.Disk = ensureHysteresisThreshold(cloneThreshold(override.Disk))
	}
	if override.DiskRead != nil {
		result.DiskRead = ensureHysteresisThreshold(cloneThreshold(override.DiskRead))
	}
	if override.DiskWrite != nil {
		result.DiskWrite = ensureHysteresisThreshold(cloneThreshold(override.DiskWrite))
	}
	if override.NetworkIn != nil {
		result.NetworkIn = ensureHysteresisThreshold(cloneThreshold(override.NetworkIn))
	}
	if override.NetworkOut != nil {
		result.NetworkOut = ensureHysteresisThreshold(cloneThreshold(override.NetworkOut))
	}
	if override.Temperature != nil {
		result.Temperature = ensureHysteresisThreshold(cloneThreshold(override.Temperature))
	}
	if override.DiskTemperature != nil {
		result.DiskTemperature = ensureHysteresisThreshold(cloneThreshold(override.DiskTemperature))
	}
	if override.SMARTHealthFailure != nil {
		result.SMARTHealthFailure = cloneInt(override.SMARTHealthFailure)
	}
	if override.SMARTReallocated != nil {
		result.SMARTReallocated = cloneInt64(override.SMARTReallocated)
	}
	if override.SMARTPending != nil {
		result.SMARTPending = cloneInt64(override.SMARTPending)
	}
	if override.SMARTUncorrectable != nil {
		result.SMARTUncorrectable = cloneInt64(override.SMARTUncorrectable)
	}
	if override.SMARTMediaErrors != nil {
		result.SMARTMediaErrors = cloneInt64(override.SMARTMediaErrors)
	}
	if override.SMARTCRCErrorDelta != nil {
		result.SMARTCRCErrorDelta = cloneInt64(override.SMARTCRCErrorDelta)
	}
	if override.SMARTLifeWarning != nil {
		result.SMARTLifeWarning = cloneInt(override.SMARTLifeWarning)
	}
	if override.SMARTLifeCritical != nil {
		result.SMARTLifeCritical = cloneInt(override.SMARTLifeCritical)
	}
	if override.SMARTSpareWarning != nil {
		result.SMARTSpareWarning = cloneInt(override.SMARTSpareWarning)
	}
	if override.SMARTSpareCritical != nil {
		result.SMARTSpareCritical = cloneInt(override.SMARTSpareCritical)
	}
	if override.Usage != nil {
		result.Usage = ensureHysteresisThreshold(cloneThreshold(override.Usage))
	}
	if override.Backup != nil {
		result.Backup = cloneBackupConfig(override.Backup)
	}
	if override.Snapshot != nil {
		result.Snapshot = cloneSnapshotConfig(override.Snapshot)
	}

	if override.Note != nil {
		note := strings.TrimSpace(*override.Note)
		if note == "" {
			result.Note = nil
		} else {
			noteCopy := note
			result.Note = &noteCopy
		}
	}

	return result
}

// ensureHysteresisThreshold ensures a threshold has hysteresis configured.
func ensureHysteresisThreshold(threshold *HysteresisThreshold) *HysteresisThreshold {
	if threshold == nil {
		return nil
	}
	if threshold.Clear <= 0 {
		threshold.Clear = threshold.Trigger - 5.0 // Default 5% margin
	}
	return threshold
}
