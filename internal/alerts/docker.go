package alerts

import (
	"fmt"
	"strings"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

type dockerRestartRecord struct {
	count       int
	lastCount   int
	times       []time.Time // Track restart times for loop detection
	lastChecked time.Time
}

// dockerInstanceName returns the logical instance name used for Docker alerts.
func dockerInstanceName(host models.DockerHost) string {
	name := strings.TrimSpace(host.DisplayName)
	if name == "" {
		name = strings.TrimSpace(host.Hostname)
	}
	if name == "" {
		return "Docker"
	}
	return fmt.Sprintf("Docker:%s", name)
}

// dockerContainerDisplayName normalizes the container name for alert readability.
func dockerContainerDisplayName(container models.DockerContainer) string {
	name := strings.TrimSpace(container.Name)
	if strings.HasPrefix(name, "/") {
		name = strings.TrimLeft(name, "/")
	}
	if name == "" {
		containerID := strings.TrimSpace(container.ID)
		if len(containerID) > 12 {
			containerID = containerID[:12]
		}
		return containerID
	}
	return name
}

// dockerResourceID builds a stable identifier for Docker container alerts.
func dockerResourceID(hostID, containerID string) string {
	hostID = strings.TrimSpace(hostID)
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		if hostID == "" {
			return "docker:unknown"
		}
		return fmt.Sprintf("docker:%s", hostID)
	}
	if hostID == "" {
		return fmt.Sprintf("docker:container/%s", containerID)
	}
	return fmt.Sprintf("docker:%s/%s", hostID, containerID)
}

func normalizeDockerUpdateTrackingPart(part string) string {
	return strings.ToLower(strings.TrimSpace(part))
}

// dockerUpdateTrackingHostKey builds a stable host identity for Docker update timing.
func dockerUpdateTrackingHostKey(host models.DockerHost) string {
	switch {
	case normalizeDockerUpdateTrackingPart(host.AgentID) != "":
		return "agent:" + normalizeDockerUpdateTrackingPart(host.AgentID)
	case normalizeDockerUpdateTrackingPart(host.TokenID) != "":
		return "token:" + normalizeDockerUpdateTrackingPart(host.TokenID)
	case normalizeDockerUpdateTrackingPart(host.MachineID) != "":
		return "machine:" + normalizeDockerUpdateTrackingPart(host.MachineID)
	case normalizeDockerUpdateTrackingPart(host.Hostname) != "":
		return "hostname:" + normalizeDockerUpdateTrackingPart(host.Hostname)
	case normalizeDockerUpdateTrackingPart(host.ID) != "":
		return "id:" + normalizeDockerUpdateTrackingPart(host.ID)
	case normalizeDockerUpdateTrackingPart(host.DisplayName) != "":
		return "name:" + normalizeDockerUpdateTrackingPart(host.DisplayName)
	default:
		return "unknown-host"
	}
}

func dockerUpdateTrackingContainerKey(container models.DockerContainer) string {
	if containerID := normalizeDockerUpdateTrackingPart(container.ID); containerID != "" {
		return "id:" + containerID
	}

	name := normalizeDockerUpdateTrackingPart(container.Name)
	name = strings.TrimPrefix(name, "/")
	if name != "" {
		return "name:" + name
	}

	if image := normalizeDockerUpdateTrackingPart(container.Image); image != "" {
		return "image:" + image
	}

	return "unknown-container"
}

func dockerUpdateTrackingKey(host models.DockerHost, container models.DockerContainer) string {
	return fmt.Sprintf("docker-update:%s/%s", dockerUpdateTrackingHostKey(host), dockerUpdateTrackingContainerKey(container))
}

func dockerUpdateTrackingHostPrefix(host models.DockerHost) string {
	return fmt.Sprintf("docker-update:%s/", dockerUpdateTrackingHostKey(host))
}

// dockerServiceDisplayName normalizes the service name for alert readability.
func dockerServiceDisplayName(service models.DockerService) string {
	name := strings.TrimSpace(service.Name)
	if name != "" {
		return name
	}
	serviceID := strings.TrimSpace(service.ID)
	if len(serviceID) > 12 {
		serviceID = serviceID[:12]
	}
	if serviceID == "" {
		return "service"
	}
	return serviceID
}

func dockerServiceResourceID(hostID, serviceID, serviceName string) string {
	hostID = strings.TrimSpace(hostID)
	normalizedServiceID := strings.TrimSpace(serviceID)
	if normalizedServiceID == "" {
		name := strings.TrimSpace(serviceName)
		if name == "" {
			name = "service"
		}
		builder := strings.Builder{}
		for _, r := range strings.ToLower(name) {
			switch {
			case r >= 'a' && r <= 'z':
				builder.WriteRune(r)
			case r >= '0' && r <= '9':
				builder.WriteRune(r)
			case r == '-', r == '_':
				builder.WriteRune(r)
			case r == ' ' || r == '/' || r == '\\' || r == ':' || r == '.':
				builder.WriteRune('-')
			}
		}
		normalizedServiceID = strings.Trim(builder.String(), "-_")
		if normalizedServiceID == "" {
			normalizedServiceID = "service"
		}
		if len(normalizedServiceID) > 32 {
			normalizedServiceID = normalizedServiceID[:32]
		}
	}
	if hostID == "" {
		return fmt.Sprintf("docker-service:%s", normalizedServiceID)
	}
	return fmt.Sprintf("docker:%s/service/%s", hostID, normalizedServiceID)
}

func matchesDockerIgnoredPrefix(name, id string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return false
	}

	name = strings.ToLower(strings.TrimSpace(name))
	id = strings.ToLower(strings.TrimSpace(id))

	for _, raw := range prefixes {
		prefix := strings.ToLower(strings.TrimSpace(raw))
		if prefix == "" {
			continue
		}
		if name != "" && strings.HasPrefix(name, prefix) {
			return true
		}
		if id != "" && strings.HasPrefix(id, prefix) {
			return true
		}
	}

	return false
}

// CheckDockerHost evaluates Docker host telemetry and container metrics for alerts.
func (m *Manager) CheckDockerHost(host models.DockerHost) {
	if host.ID == "" {
		return
	}

	// Fresh telemetry marks the host as online and clears any offline alert.
	m.HandleDockerHostOnline(host)

	m.mu.RLock()
	alertsEnabled := m.config.Enabled
	disableAllDockerHosts := m.config.DisableAllDockerHosts
	ignoredPrefixes := append([]string(nil), m.config.DockerIgnoredContainerPrefixes...)
	m.mu.RUnlock()
	if !alertsEnabled {
		return
	}
	if disableAllDockerHosts {
		return
	}

	seen := make(map[string]struct{}, len(host.Containers)+len(host.Services))
	seenUpdateTracking := make(map[string]struct{}, len(host.Containers))
	for _, container := range host.Containers {
		containerName := dockerContainerDisplayName(container)
		resourceID := dockerResourceID(host.ID, container.ID)
		updateTrackingKey := dockerUpdateTrackingKey(host, container)

		if matchesDockerIgnoredPrefix(containerName, container.ID, ignoredPrefixes) {
			log.Debug().
				Str("container", containerName).
				Str("host", host.DisplayName).
				Msg("Skipping Docker container alert evaluation due to ignored prefix")
			m.clearDockerContainerStateAlert(resourceID)
			m.clearDockerContainerHealthAlert(resourceID)
			m.clearDockerContainerMetricAlerts(resourceID)
			m.clearAlert(fmt.Sprintf("docker-container-restart-loop-%s", resourceID))
			m.clearAlert(fmt.Sprintf("docker-container-oom-%s", resourceID))
			m.clearAlert(fmt.Sprintf("docker-container-memory-limit-%s", resourceID))
			m.mu.Lock()
			delete(m.dockerRestartTracking, resourceID)
			delete(m.dockerLastExitCode, resourceID)
			m.mu.Unlock()
			m.clearDockerContainerUpdateTracking(resourceID, updateTrackingKey)
			continue
		}

		seen[resourceID] = struct{}{}
		seenUpdateTracking[updateTrackingKey] = struct{}{}
		m.evaluateDockerContainer(host, container, resourceID)
	}

	for _, service := range host.Services {
		resourceID := dockerServiceResourceID(host.ID, service.ID, service.Name)
		seen[resourceID] = struct{}{}
		m.evaluateDockerService(host, service, resourceID)
	}

	m.cleanupDockerContainerAlertsWithTracking(host, seen, seenUpdateTracking)
}

func (m *Manager) evaluateDockerContainer(host models.DockerHost, container models.DockerContainer, resourceID string) {
	m.mu.RLock()
	disableAllContainers := m.config.DisableAllDockerContainers
	m.mu.RUnlock()
	if disableAllContainers {
		return
	}

	containerName := dockerContainerDisplayName(container)
	nodeName := strings.TrimSpace(host.Hostname)
	instanceName := dockerInstanceName(host)
	resourceType := "app-container"

	m.mu.RLock()
	overrideConfig, hasOverride := m.config.Overrides[resourceID]
	m.mu.RUnlock()
	if hasOverride && overrideConfig.Disabled {
		// Alerts disabled via override; clear any existing alerts and skip evaluation.
		m.clearDockerContainerStateAlert(resourceID)
		m.clearDockerContainerHealthAlert(resourceID)
		m.clearDockerContainerMetricAlerts(resourceID)
		m.clearAlert(fmt.Sprintf("docker-container-update-%s", resourceID))
		m.clearAlert(buildCanonicalStateID(resourceID, resourceID+"-image-update"))
		m.clearDockerContainerUpdateTracking(resourceID, dockerUpdateTrackingKey(host, container))
		return
	}

	state := strings.ToLower(strings.TrimSpace(container.State))
	if state == "" {
		state = strings.ToLower(strings.TrimSpace(container.Status))
	}

	if state != "running" {
		m.checkDockerContainerState(host, container, resourceID, containerName, instanceName, nodeName)
		m.clearDockerContainerMetricAlerts(resourceID, "cpu", "memory", "disk")
	} else {
		m.clearDockerContainerStateAlert(resourceID)

		// Use Docker-specific defaults for containers
		thresholds := ThresholdConfig{
			CPU:    &m.config.DockerDefaults.CPU,
			Memory: &m.config.DockerDefaults.Memory,
			Disk:   &m.config.DockerDefaults.Disk,
		}
		if hasOverride {
			thresholds = m.applyThresholdOverride(thresholds, overrideConfig)
		}

		if thresholds.CPU != nil {
			cpuCapacityPercent := models.DockerContainerCPUCapacityPercent(container, host.CPUs)
			cpuMetadata := map[string]interface{}{
				"resourceType":       resourceType,
				"hostId":             host.ID,
				"hostName":           host.DisplayName,
				"hostHostname":       host.Hostname,
				"containerId":        container.ID,
				"containerName":      containerName,
				"image":              container.Image,
				"state":              container.State,
				"status":             container.Status,
				"restartCount":       container.RestartCount,
				"metric":             "cpu",
				"cpuPercent":         cpuCapacityPercent,
				"cpuCapacityPercent": cpuCapacityPercent,
				"cpuRawPercent":      container.CPUPercent,
				"cpuCapacityCores":   host.CPUs,
			}
			spec, err := buildCanonicalMetricSpec(resourceID, containerName, unifiedresources.ResourceTypeAppContainer, "cpu", thresholds.CPU)
			if err != nil {
				log.Warn().
					Err(err).
					Str("resourceID", resourceID).
					Str("container", containerName).
					Msg("Skipping invalid canonical docker container CPU metric spec")
			} else {
				m.checkMetricWithCanonicalSpec(spec, containerName, nodeName, instanceName, resourceType, cpuCapacityPercent, thresholds.CPU, &metricOptions{Metadata: cpuMetadata})
			}
		}

		if thresholds.Memory != nil {
			memMetadata := map[string]interface{}{
				"resourceType":     resourceType,
				"hostId":           host.ID,
				"hostName":         host.DisplayName,
				"hostHostname":     host.Hostname,
				"containerId":      container.ID,
				"containerName":    containerName,
				"image":            container.Image,
				"state":            container.State,
				"status":           container.Status,
				"restartCount":     container.RestartCount,
				"metric":           "memory",
				"memoryPercent":    container.MemoryPercent,
				"memoryUsageBytes": container.MemoryUsage,
			}
			if container.MemoryLimit > 0 {
				memMetadata["memoryLimitBytes"] = container.MemoryLimit
			}
			spec, err := buildCanonicalMetricSpec(resourceID, containerName, unifiedresources.ResourceTypeAppContainer, "memory", thresholds.Memory)
			if err != nil {
				log.Warn().
					Err(err).
					Str("resourceID", resourceID).
					Str("container", containerName).
					Msg("Skipping invalid canonical docker container memory metric spec")
			} else {
				m.checkMetricWithCanonicalSpec(spec, containerName, nodeName, instanceName, resourceType, container.MemoryPercent, thresholds.Memory, &metricOptions{Metadata: memMetadata})
			}
		}

		if thresholds.Disk != nil {
			totalBytes := container.RootFilesystemBytes
			usedBytes := container.WritableLayerBytes
			if totalBytes > 0 && usedBytes >= 0 {
				diskPercent := (float64(usedBytes) / float64(totalBytes)) * 100
				diskMetadata := map[string]interface{}{
					"resourceType":        resourceType,
					"hostId":              host.ID,
					"hostName":            host.DisplayName,
					"hostHostname":        host.Hostname,
					"containerId":         container.ID,
					"containerName":       containerName,
					"image":               container.Image,
					"state":               container.State,
					"status":              container.Status,
					"restartCount":        container.RestartCount,
					"metric":              "disk",
					"diskPercent":         diskPercent,
					"writableLayerBytes":  usedBytes,
					"rootFilesystemBytes": totalBytes,
					"mountCount":          len(container.Mounts),
				}
				if container.BlockIO != nil {
					diskMetadata["blockIoReadBytes"] = container.BlockIO.ReadBytes
					diskMetadata["blockIoWriteBytes"] = container.BlockIO.WriteBytes
				}
				spec, err := buildCanonicalMetricSpec(resourceID, containerName, unifiedresources.ResourceTypeAppContainer, "disk", thresholds.Disk)
				if err != nil {
					log.Warn().
						Err(err).
						Str("resourceID", resourceID).
						Str("container", containerName).
						Msg("Skipping invalid canonical docker container disk metric spec")
				} else {
					m.checkMetricWithCanonicalSpec(spec, containerName, nodeName, instanceName, resourceType, diskPercent, thresholds.Disk, &metricOptions{Metadata: diskMetadata})
				}
			} else {
				m.clearDockerContainerMetricAlerts(resourceID, "disk")
			}
		}
	}

	m.checkDockerContainerHealth(host, container, resourceID, containerName, instanceName, nodeName)

	// Docker-specific checks
	m.checkDockerContainerRestartLoop(host, container, resourceID, containerName, instanceName, nodeName)
	m.checkDockerContainerOOMKill(host, container, resourceID, containerName, instanceName, nodeName)
	m.checkDockerContainerMemoryLimit(host, container, resourceID, containerName, instanceName, nodeName)
	m.checkDockerContainerImageUpdate(host, container, resourceID, containerName, instanceName, nodeName)
}

func (m *Manager) evaluateDockerService(host models.DockerHost, service models.DockerService, resourceID string) {
	m.mu.RLock()
	disableAllServices := m.config.DisableAllDockerServices
	warnPct := m.config.DockerDefaults.ServiceWarnGapPct
	critPct := m.config.DockerDefaults.ServiceCritGapPct
	overrideConfig, hasOverride := m.config.Overrides[resourceID]
	m.mu.RUnlock()

	if disableAllServices {
		m.clearDockerServiceAlert(resourceID)
		return
	}
	if hasOverride && overrideConfig.Disabled {
		m.clearDockerServiceAlert(resourceID)
		return
	}

	desired := service.DesiredTasks
	running := service.RunningTasks
	if desired <= 0 {
		m.clearDockerServiceAlert(resourceID)
		return
	}

	missing := desired - running
	if missing < 0 {
		missing = 0
	}

	percentMissing := 0.0
	if desired > 0 {
		percentMissing = (float64(missing) / float64(desired)) * 100.0
	}

	thresholdValue := 0.0
	serviceName := dockerServiceDisplayName(service)
	instanceName := dockerInstanceName(host)
	nodeName := strings.TrimSpace(host.Hostname)

	metadata := map[string]interface{}{
		"resourceType":   "docker-service",
		"hostId":         host.ID,
		"hostName":       host.DisplayName,
		"hostHostname":   host.Hostname,
		"serviceId":      service.ID,
		"serviceName":    service.Name,
		"stack":          service.Stack,
		"mode":           service.Mode,
		"desiredTasks":   service.DesiredTasks,
		"runningTasks":   service.RunningTasks,
		"completedTasks": service.CompletedTasks,
		"missingTasks":   missing,
		"percentMissing": percentMissing,
	}
	alertID := fmt.Sprintf("docker-service-health-%s", resourceID)

	if critPct > 0 && percentMissing >= float64(critPct) {
		thresholdValue = float64(critPct)
	} else if warnPct > 0 && percentMissing >= float64(warnPct) {
		thresholdValue = float64(warnPct)
	}

	updateState := ""
	updateMessage := ""
	updateSeverity := AlertLevel("")
	if service.UpdateStatus != nil {
		updateState = strings.ToLower(strings.TrimSpace(service.UpdateStatus.State))
		updateMessage = strings.TrimSpace(service.UpdateStatus.Message)
		switch updateState {
		case "paused", "rollback_started", "rollback_paused":
			updateSeverity = AlertLevelWarning
		case "rollback_failed":
			updateSeverity = AlertLevelCritical
		}
		if service.UpdateStatus.CompletedAt != nil && !service.UpdateStatus.CompletedAt.IsZero() {
			metadata["updateCompletedAt"] = service.UpdateStatus.CompletedAt.UTC()
		}
		if updateState != "" {
			metadata["updateState"] = service.UpdateStatus.State
		}
		if updateMessage != "" {
			metadata["updateMessage"] = updateMessage
		}
	}

	if thresholdValue == 0 && updateSeverity != "" {
		spec, err := buildCanonicalDiscreteStateSpec(resourceID, serviceName, unifiedresources.ResourceTypeDockerService, updateSeverity, 1, false, "update-state",
			[]string{"paused", "rollback_started", "rollback_paused", "rollback_failed"})
		if err != nil {
			log.Warn().
				Err(err).
				Str("service", serviceName).
				Str("resourceID", resourceID).
				Msg("Skipping invalid canonical docker service update-state spec")
			return
		}

		message := fmt.Sprintf("Docker service '%s' update state: %s", serviceName, service.UpdateStatus.State)
		if updateMessage != "" {
			message = fmt.Sprintf("%s (%s)", message, updateMessage)
		}

		_, _ = m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
			Spec:                         spec,
			Evidence:                     alertspecs.AlertEvidence{ObservedAt: time.Now(), DiscreteState: &alertspecs.DiscreteStateEvidence{StateKey: "update-state", Observed: updateState}},
			AlertID:                      alertID,
			AlertType:                    "docker-service-health",
			ResourceID:                   resourceID,
			ResourceName:                 serviceName,
			Node:                         nodeName,
			Instance:                     instanceName,
			Message:                      message,
			Value:                        percentMissing,
			Threshold:                    0,
			Metadata:                     metadata,
			AddToRecent:                  true,
			AddToHistory:                 true,
			RateLimit:                    true,
			NotifyOnSeverityChange:       true,
			AddToHistoryOnSeverityChange: true,
			DispatchAsync:                true,
		})
		return
	}

	if thresholdValue == 0 {
		m.clearDockerServiceAlert(resourceID)
		return
	}

	spec, err := buildCanonicalServiceGapSpec(resourceID, serviceName, unifiedresources.ResourceTypeDockerService, serviceName, float64(warnPct), float64(critPct), false)
	if err != nil {
		log.Warn().
			Err(err).
			Str("service", serviceName).
			Str("resourceID", resourceID).
			Msg("Skipping invalid canonical docker service gap spec")
		m.clearDockerServiceAlert(resourceID)
		return
	}

	message := fmt.Sprintf("Docker service '%s' is running %d of %d desired tasks", serviceName, service.RunningTasks, service.DesiredTasks)
	_, _ = m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt: time.Now(),
			ServiceGap: &alertspecs.ServiceGapEvidence{
				Service: serviceName,
				Desired: desired,
				Running: running,
			},
		},
		AlertID:                      alertID,
		AlertType:                    "docker-service-health",
		ResourceID:                   resourceID,
		ResourceName:                 serviceName,
		Node:                         nodeName,
		Instance:                     instanceName,
		Message:                      message,
		Value:                        percentMissing,
		Threshold:                    thresholdValue,
		Metadata:                     metadata,
		AddToRecent:                  true,
		AddToHistory:                 true,
		RateLimit:                    true,
		NotifyOnSeverityChange:       true,
		AddToHistoryOnSeverityChange: true,
		DispatchAsync:                true,
	})
}

func (m *Manager) clearDockerServiceAlert(resourceID string) {
	m.clearAlert(canonicalServiceGapStateID(resourceID))
	m.clearAlert(canonicalDiscreteStateStateID(resourceID, "update-state"))
}

// HandleDockerHostOnline clears offline tracking and alerts for a Docker host.
func (m *Manager) HandleDockerHostOnline(host models.DockerHost) {
	if host.ID == "" {
		return
	}

	alertID := canonicalConnectivityStateID(fmt.Sprintf("docker:%s", strings.TrimSpace(host.ID)))

	m.mu.Lock()
	delete(m.dockerOfflineCount, host.ID)
	exists := m.hasActiveAlertNoLock(alertID)
	m.mu.Unlock()

	if exists {
		m.clearAlert(alertID)
	}
}

// HandleDockerHostRemoved clears all alerts and tracking when a Docker host is deleted.
func (m *Manager) HandleDockerHostRemoved(host models.DockerHost) {
	if host.ID == "" {
		return
	}

	// Reuse the online handler to clear offline alerts and tracking.
	m.HandleDockerHostOnline(host)
	// Drop any container alerts and host-scoped tracking entries.
	m.clearDockerHostContainerAlerts(host)
}

// HandleDockerHostOffline raises an alert when a Docker host stops reporting.
func (m *Manager) HandleDockerHostOffline(host models.DockerHost) {
	if host.ID == "" {
		return
	}

	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return
	}
	disableDockerHostsOffline := m.config.DisableAllDockerHostsOffline
	m.mu.RUnlock()

	resourceID := fmt.Sprintf("docker:%s", strings.TrimSpace(host.ID))
	alertID := canonicalConnectivityStateID(resourceID)
	instanceName := dockerInstanceName(host)
	nodeName := strings.TrimSpace(host.Hostname)

	if disableDockerHostsOffline {
		m.mu.Lock()
		delete(m.dockerOfflineCount, host.ID)
		m.mu.Unlock()
		m.clearAlert(alertID)
		return
	}

	var disableConnectivity bool
	m.mu.RLock()
	if override, exists := m.config.Overrides[host.ID]; exists {
		disableConnectivity = override.DisableConnectivity
	}
	m.mu.RUnlock()

	if disableConnectivity {
		m.clearAlert(alertID)
		m.mu.Lock()
		delete(m.dockerOfflineCount, host.ID)
		m.mu.Unlock()
		return
	}

	spec, err := buildCanonicalConnectivitySpec(resourceID, host.DisplayName, unifiedresources.ResourceType("docker-host"), AlertLevelCritical, 3, false)
	if err != nil {
		log.Warn().
			Err(err).
			Str("dockerHost", host.DisplayName).
			Str("hostID", host.ID).
			Msg("Skipping invalid canonical docker host connectivity spec")
		return
	}

	result, ok := m.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec:         spec,
		Evidence:     alertspecs.AlertEvidence{ObservedAt: time.Now(), Connectivity: &alertspecs.ConnectivityEvidence{Signal: "status", Connected: false}},
		Tracking:     m.dockerOfflineCount,
		TrackingKey:  host.ID,
		AlertID:      alertID,
		AlertType:    "docker-host-offline",
		ResourceID:   resourceID,
		ResourceName: host.DisplayName,
		Node:         nodeName,
		Instance:     instanceName,
		Message:      fmt.Sprintf("Docker host '%s' is offline", host.DisplayName),
		Metadata: map[string]interface{}{
			"resourceType": "docker-host",
			"hostId":       host.ID,
			"hostname":     host.Hostname,
			"agentId":      host.AgentID,
			"displayName":  host.DisplayName,
		},
		AddToRecent:   true,
		AddToHistory:  true,
		RateLimit:     true,
		DispatchAsync: false,
	})
	if !ok || result.Transition == nil || result.Transition.Kind != alertspecs.EvaluationTransitionActivated {
		return
	}

	m.mu.RLock()
	alert, _ := m.getActiveAlertNoLock(alertID)
	m.mu.RUnlock()
	if alert != nil {
		if callbacks := m.getAlertForAICallbacks(); len(callbacks) > 0 {
			alertCopy := cloneAlertForOutput(alert)
			go func(a *Alert, fns []func(*Alert)) {
				defer func() {
					if r := recover(); r != nil {
						log.Error().Interface("panic", r).Str("alertID", a.ID).Msg("panic in AI alert callback")
					}
				}()
				for _, callback := range fns {
					callback(a)
				}
			}(alertCopy, callbacks)
		}
	}

	m.clearDockerHostContainerAlerts(host)
}

func (m *Manager) checkDockerContainerState(host models.DockerHost, container models.DockerContainer, resourceID, containerName, instanceName, nodeName string) {
	alertID := fmt.Sprintf("docker-container-state-%s", resourceID)
	stateKey := resourceID

	m.mu.RLock()
	override, hasOverride := m.config.Overrides[resourceID]
	defaultDisable := m.config.DockerDefaults.StateDisableConnectivity
	defaultSeverity := NormalizePoweredOffSeverity(m.config.DockerDefaults.StatePoweredOffSeverity)
	m.mu.RUnlock()

	disableConnectivity := defaultDisable
	severity := defaultSeverity
	if hasOverride {
		if defaultDisable && !override.DisableConnectivity {
			disableConnectivity = false
		} else if override.DisableConnectivity {
			disableConnectivity = true
		}

		if override.PoweredOffSeverity != "" {
			severity = NormalizePoweredOffSeverity(override.PoweredOffSeverity)
		}
	}

	if disableConnectivity {
		m.clearDockerContainerStateAlert(resourceID)
		return
	}

	observedState := strings.ToLower(strings.TrimSpace(container.State))
	if observedState == "" {
		observedState = "unknown"
	}

	spec, err := buildCanonicalDiscreteStateSpec(resourceID, containerName, unifiedresources.ResourceTypeAppContainer, severity, 2, false, "runtime-state",
		[]string{"created", "restarting", "removing", "paused", "exited", "dead", "unknown"})
	if err != nil {
		log.Warn().
			Err(err).
			Str("resourceID", resourceID).
			Str("container", containerName).
			Msg("Skipping invalid canonical docker container state spec")
		return
	}

	_, _ = m.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt: time.Now(),
			DiscreteState: &alertspecs.DiscreteStateEvidence{
				StateKey: "runtime-state",
				Observed: observedState,
			},
		},
		Tracking:     m.dockerStateConfirm,
		TrackingKey:  stateKey,
		AlertID:      alertID,
		AlertType:    "docker-container-state",
		ResourceID:   resourceID,
		ResourceName: containerName,
		Node:         nodeName,
		Instance:     instanceName,
		Message:      fmt.Sprintf("Docker container '%s' is %s", containerName, strings.TrimSpace(container.Status)),
		Metadata: map[string]interface{}{
			"resourceType":  "app-container",
			"hostId":        host.ID,
			"hostName":      host.DisplayName,
			"hostHostname":  host.Hostname,
			"containerId":   container.ID,
			"containerName": containerName,
			"image":         container.Image,
			"state":         container.State,
			"status":        container.Status,
		},
		AddToRecent:   true,
		AddToHistory:  true,
		DispatchAsync: true,
	})
}

func (m *Manager) clearDockerContainerStateAlert(resourceID string) {
	m.mu.Lock()
	delete(m.dockerStateConfirm, resourceID)
	m.mu.Unlock()
	m.clearAlert(canonicalDiscreteStateStateID(resourceID, "runtime-state"))
}

func dockerContainerAlertMetadata(host models.DockerHost, container models.DockerContainer, containerName string) map[string]interface{} {
	return map[string]interface{}{
		"resourceType":  "app-container",
		"hostId":        host.ID,
		"hostName":      host.DisplayName,
		"hostHostname":  host.Hostname,
		"containerId":   container.ID,
		"containerName": containerName,
		"image":         container.Image,
		"state":         container.State,
		"status":        container.Status,
	}
}

func (m *Manager) checkDockerContainerHealth(host models.DockerHost, container models.DockerContainer, resourceID, containerName, instanceName, nodeName string) {
	health := strings.ToLower(strings.TrimSpace(container.Health))
	if health == "" || health == "none" || health == "healthy" || health == "starting" {
		m.clearDockerContainerHealthAlert(resourceID)
		return
	}

	alertID := fmt.Sprintf("docker-container-health-%s", resourceID)
	spec, err := buildCanonicalHealthAssessmentSpec(resourceID+"-health", resourceID, containerName, unifiedresources.ResourceTypeAppContainer, "docker-container-health", nil, false)
	if err != nil {
		log.Warn().
			Err(err).
			Str("resourceID", resourceID).
			Str("container", containerName).
			Msg("Skipping invalid canonical docker container health spec")
		return
	}

	severity := alertspecs.AlertSeverityWarning
	if health == "unhealthy" {
		severity = alertspecs.AlertSeverityCritical
	}

	metadata := dockerContainerAlertMetadata(host, container, containerName)
	metadata["health"] = container.Health

	_, _ = m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt: time.Now(),
			HealthAssessment: &alertspecs.HealthAssessmentEvidence{
				Signal:   "docker-container-health",
				Severity: severity,
				Codes:    []string{health},
			},
		},
		AlertID:                      alertID,
		AlertType:                    "docker-container-health",
		ResourceID:                   resourceID,
		ResourceName:                 containerName,
		Node:                         nodeName,
		Instance:                     instanceName,
		Message:                      fmt.Sprintf("Docker container '%s' health is %s", containerName, container.Health),
		Metadata:                     metadata,
		AddToRecent:                  true,
		AddToHistory:                 true,
		DispatchAsync:                false,
		NotifyOnSeverityChange:       true,
		AddToHistoryOnSeverityChange: true,
	})

	log.Warn().
		Str("container", containerName).
		Str("host", host.DisplayName).
		Str("health", container.Health).
		Msg("Docker container health alert raised")
}

func (m *Manager) clearDockerContainerHealthAlert(resourceID string) {
	m.clearAlert(buildCanonicalStateID(resourceID, resourceID+"-health"))
}

// checkDockerContainerRestartLoop detects containers stuck in a restart loop
func (m *Manager) checkDockerContainerRestartLoop(host models.DockerHost, container models.DockerContainer, resourceID, containerName, instanceName, nodeName string) {
	alertID := fmt.Sprintf("docker-container-restart-loop-%s", resourceID)
	now := time.Now()

	// Get config values with defaults
	restartThreshold := m.config.DockerDefaults.RestartCount
	if restartThreshold == 0 {
		restartThreshold = 3 // Default: 3 restarts
	}
	timeWindow := m.config.DockerDefaults.RestartWindow
	if timeWindow == 0 {
		timeWindow = 300 // Default: 5 minutes (300 seconds)
	}

	m.mu.Lock()

	record, exists := m.dockerRestartTracking[resourceID]
	if !exists {
		record = &dockerRestartRecord{
			count:       container.RestartCount,
			lastCount:   container.RestartCount,
			times:       []time.Time{},
			lastChecked: now,
		}
		m.dockerRestartTracking[resourceID] = record
		m.mu.Unlock()
		return
	}

	// If restart count increased, track it
	if container.RestartCount > record.lastCount {
		newRestarts := container.RestartCount - record.lastCount
		for i := 0; i < newRestarts; i++ {
			record.times = append(record.times, now)
		}
		record.lastCount = container.RestartCount
	}

	// Clean up old restart times outside the window
	cutoff := now.Add(-time.Duration(timeWindow) * time.Second)
	var recentRestarts []time.Time
	for _, t := range record.times {
		if t.After(cutoff) {
			recentRestarts = append(recentRestarts, t)
		}
	}
	record.times = recentRestarts
	record.lastChecked = now

	recentCount := len(record.times)
	m.mu.Unlock()

	// Check if we have a restart loop
	if recentCount > restartThreshold {
		spec, err := buildCanonicalSeverityThresholdSpec(resourceID+"-restart-loop", resourceID, containerName, unifiedresources.ResourceTypeAppContainer, "restart-count-window", 0, float64(restartThreshold+1), false)
		if err != nil {
			log.Warn().
				Err(err).
				Str("resourceID", resourceID).
				Str("container", containerName).
				Msg("Skipping invalid canonical docker container restart loop spec")
			return
		}

		metadata := dockerContainerAlertMetadata(host, container, containerName)
		metadata["restartCount"] = container.RestartCount
		metadata["recentRestarts"] = recentCount

		_, _ = m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
			Spec: spec,
			Evidence: alertspecs.AlertEvidence{
				ObservedAt: now,
				SeverityThreshold: &alertspecs.SeverityThresholdEvidence{
					Metric:    "restart-count-window",
					Direction: alertspecs.ThresholdDirectionAbove,
					Observed:  float64(recentCount),
				},
			},
			AlertID:      alertID,
			AlertType:    "docker-container-restart-loop",
			ResourceID:   resourceID,
			ResourceName: containerName,
			Node:         nodeName,
			Instance:     instanceName,
			Message:      fmt.Sprintf("Docker container '%s' has restarted %d times in the last %d minutes (restart loop detected)", containerName, recentCount, timeWindow/60),
			Metadata:     metadata,
			AddToRecent:  true,
			AddToHistory: true,
		})

		log.Warn().
			Str("container", containerName).
			Str("host", host.DisplayName).
			Int("restarts", recentCount).
			Msg("Docker container restart loop detected")
	} else {
		// Clear alert if restart loop has stopped
		m.clearAlert(buildCanonicalStateID(resourceID, resourceID+"-restart-loop"))
	}
}

// checkDockerContainerOOMKill detects when a container was killed due to out of memory
func (m *Manager) checkDockerContainerOOMKill(host models.DockerHost, container models.DockerContainer, resourceID, containerName, instanceName, nodeName string) {
	alertID := fmt.Sprintf("docker-container-oom-%s", resourceID)

	// Exit code 137 means the container was killed by SIGKILL, often due to OOM
	// Only alert if the container exited (not running) with exit code 137
	state := strings.ToLower(strings.TrimSpace(container.State))
	if (state == "exited" || state == "dead") && container.ExitCode == 137 {
		m.mu.Lock()
		m.dockerLastExitCode[resourceID] = 137
		m.mu.Unlock()

		spec, err := buildCanonicalHealthAssessmentSpec(resourceID+"-oom-kill", resourceID, containerName, unifiedresources.ResourceTypeAppContainer, "docker-container-exit", []string{"oom-kill"}, false)
		if err != nil {
			log.Warn().
				Err(err).
				Str("resourceID", resourceID).
				Str("container", containerName).
				Msg("Skipping invalid canonical docker container OOM spec")
			return
		}

		metadata := dockerContainerAlertMetadata(host, container, containerName)
		metadata["exitCode"] = container.ExitCode
		metadata["memoryUsageBytes"] = container.MemoryUsage
		metadata["memoryLimitBytes"] = container.MemoryLimit

		_, _ = m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
			Spec: spec,
			Evidence: alertspecs.AlertEvidence{
				ObservedAt: time.Now(),
				HealthAssessment: &alertspecs.HealthAssessmentEvidence{
					Signal:   "docker-container-exit",
					Severity: alertspecs.AlertSeverityCritical,
					Codes:    []string{"oom-kill"},
				},
			},
			AlertID:      alertID,
			AlertType:    "docker-container-oom-kill",
			ResourceID:   resourceID,
			ResourceName: containerName,
			Node:         nodeName,
			Instance:     instanceName,
			Message:      fmt.Sprintf("Docker container '%s' was killed due to out of memory (OOM)", containerName),
			Metadata:     metadata,
			AddToRecent:  true,
			AddToHistory: true,
		})

		log.Error().
			Str("container", containerName).
			Str("host", host.DisplayName).
			Int64("memoryUsage", container.MemoryUsage).
			Int64("memoryLimit", container.MemoryLimit).
			Msg("Docker container OOM killed")
	} else {
		// Update last exit code if it changed
		if container.ExitCode != 0 {
			m.mu.Lock()
			m.dockerLastExitCode[resourceID] = container.ExitCode
			m.mu.Unlock()
		}
		// Clear OOM alert if container is running or exited with different code
		m.clearAlert(buildCanonicalStateID(resourceID, resourceID+"-oom-kill"))
	}
}

// checkDockerContainerMemoryLimit alerts when container approaches its memory limit
func (m *Manager) checkDockerContainerMemoryLimit(host models.DockerHost, container models.DockerContainer, resourceID, containerName, instanceName, nodeName string) {
	// Only check if container is running and has a memory limit
	state := strings.ToLower(strings.TrimSpace(container.State))
	if state != "running" || container.MemoryLimit <= 0 {
		return
	}

	alertID := fmt.Sprintf("docker-container-memory-limit-%s", resourceID)

	// Get config values with defaults
	warnThreshold := float64(m.config.DockerDefaults.MemoryWarnPct)
	if warnThreshold == 0 {
		warnThreshold = 90.0 // Default: 90%
	}
	criticalThreshold := float64(m.config.DockerDefaults.MemoryCriticalPct)
	if criticalThreshold == 0 {
		criticalThreshold = 95.0 // Default: 95%
	}

	// Calculate percentage of limit used
	limitPercent := (float64(container.MemoryUsage) / float64(container.MemoryLimit)) * 100

	clearThreshold := warnThreshold - 5
	recovery := clearThreshold
	spec, err := buildCanonicalSeverityThresholdSpecWithRecovery(resourceID+"-memory-limit", resourceID, containerName, unifiedresources.ResourceTypeAppContainer, "memory-limit-percent", warnThreshold, criticalThreshold, &recovery, false)
	if err != nil {
		log.Warn().
			Err(err).
			Str("resourceID", resourceID).
			Str("container", containerName).
			Msg("Skipping invalid canonical docker container memory limit spec")
		return
	}

	metadata := dockerContainerAlertMetadata(host, container, containerName)
	metadata["memoryUsageBytes"] = container.MemoryUsage
	metadata["memoryLimitBytes"] = container.MemoryLimit
	metadata["limitPercent"] = limitPercent

	_, _ = m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt: time.Now(),
			SeverityThreshold: &alertspecs.SeverityThresholdEvidence{
				Metric:    "memory-limit-percent",
				Direction: alertspecs.ThresholdDirectionAbove,
				Observed:  limitPercent,
			},
		},
		AlertID:      alertID,
		AlertType:    "docker-container-memory-limit",
		ResourceID:   resourceID,
		ResourceName: containerName,
		Node:         nodeName,
		Instance:     instanceName,
		Message:      fmt.Sprintf("Docker container '%s' is using %.1f%% of its memory limit (%d MB / %d MB)", containerName, limitPercent, container.MemoryUsage/(1024*1024), container.MemoryLimit/(1024*1024)),
		Metadata:     metadata,
		AddToRecent:  true,
		AddToHistory: true,
	})

	if limitPercent >= warnThreshold {
		log.Warn().
			Str("container", containerName).
			Str("host", host.DisplayName).
			Float64("limitPercent", limitPercent).
			Msg("Docker container approaching memory limit")
	}
}

func (m *Manager) clearDockerContainerMetricAlerts(resourceID string, metrics ...string) {
	if len(metrics) == 0 {
		metrics = []string{"cpu", "memory", "disk"}
	}
	for _, metric := range metrics {
		m.clearAlert(canonicalMetricStateID(resourceID, metric))
	}
}

func (m *Manager) clearDockerContainerUpdateTracking(resourceID, trackingKey string) {
	m.mu.Lock()
	delete(m.dockerUpdateFirstSeen, resourceID)
	if trackingKey != "" {
		delete(m.dockerUpdateFirstSeenByIdentity, trackingKey)
	}
	m.mu.Unlock()
}

func dockerUpdateTrackingKeyFromAlert(alert *Alert) string {
	if alert == nil || alert.Metadata == nil {
		return ""
	}

	metadataString := func(key string) string {
		value, ok := alert.Metadata[key]
		if !ok || value == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(value))
	}

	host := models.DockerHost{
		ID:          metadataString("hostId"),
		DisplayName: metadataString("hostName"),
		Hostname:    metadataString("hostHostname"),
	}
	container := models.DockerContainer{
		ID:    metadataString("containerId"),
		Name:  metadataString("containerName"),
		Image: metadataString("image"),
	}

	if host.ID == "" && host.DisplayName == "" && host.Hostname == "" &&
		container.ID == "" && container.Name == "" && container.Image == "" {
		return ""
	}

	return dockerUpdateTrackingKey(host, container)
}

func (m *Manager) clearDockerContainerUpdateStateLocked(alert *Alert) {
	if alert == nil {
		return
	}

	if alert.ResourceID != "" {
		delete(m.dockerUpdateFirstSeen, alert.ResourceID)
	}
	if trackingKey := dockerUpdateTrackingKeyFromAlert(alert); trackingKey != "" {
		delete(m.dockerUpdateFirstSeenByIdentity, trackingKey)
	}
}

func (m *Manager) clearDockerContainerUpdateAlertsLocked() {
	toClear := make([]string, 0)
	for storageKey, alert := range m.activeAlerts {
		if alert == nil {
			continue
		}
		alertID := effectiveAlertID(alert, storageKey)
		if alert.Type != "docker-container-update" && !strings.HasPrefix(alertID, "docker-container-update-") {
			continue
		}
		m.clearDockerContainerUpdateStateLocked(alert)
		toClear = append(toClear, alertID)
	}
	for _, alertID := range toClear {
		m.clearAlertNoLock(alertID)
	}
}

func (m *Manager) shouldResolveDockerContainerUpdateAlertLocked(alert *Alert) bool {
	if alert == nil {
		return false
	}

	if m.config.DisableAllDockerContainers || m.config.DockerDefaults.UpdateAlertDelayHours < 0 {
		m.clearDockerContainerUpdateStateLocked(alert)
		return true
	}

	if override, exists := m.config.Overrides[alert.ResourceID]; exists && override.Disabled {
		m.clearDockerContainerUpdateStateLocked(alert)
		return true
	}

	containerName := strings.TrimSpace(alert.ResourceName)
	containerID := ""
	if alert.Metadata != nil {
		if value, ok := alert.Metadata["containerName"].(string); ok && containerName == "" {
			containerName = value
		}
		if value, ok := alert.Metadata["containerId"].(string); ok {
			containerID = value
		}
	}
	if matchesDockerIgnoredPrefix(containerName, containerID, m.config.DockerIgnoredContainerPrefixes) {
		m.clearDockerContainerUpdateStateLocked(alert)
		return true
	}

	return false
}

func (m *Manager) touchDockerContainerUpdateAlert(alertID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if alert, exists := m.getActiveAlertNoLock(alertID); exists && alert != nil {
		alert.LastSeen = time.Now()
	}
}

// checkDockerContainerImageUpdate checks if an image update has been pending for too long
func (m *Manager) checkDockerContainerImageUpdate(host models.DockerHost, container models.DockerContainer, resourceID, containerName, instanceName, nodeName string) {
	alertID := fmt.Sprintf("docker-container-update-%s", resourceID)
	canonicalAlertID := buildCanonicalStateID(resourceID, resourceID+"-image-update")
	updateTrackingKey := dockerUpdateTrackingKey(host, container)

	// Check if update detection is enabled
	m.mu.RLock()
	delayHours := m.config.DockerDefaults.UpdateAlertDelayHours
	m.mu.RUnlock()

	// Negative value means disabled
	if delayHours < 0 {
		m.clearAlert(canonicalAlertID)
		m.clearDockerContainerUpdateTracking(resourceID, updateTrackingKey)
		return
	}

	// Check if this container has an update status reported
	if container.UpdateStatus == nil {
		// Missing update status means the condition is unknown, not resolved.
		// Preserve any active alert and first-seen tracking until we see an affirmative clear.
		m.touchDockerContainerUpdateAlert(canonicalAlertID)
		return
	}

	// Check for errors in update detection (don't alert on errors)
	if container.UpdateStatus.Error != "" {
		// A failed update check cannot confirm the pending update has been resolved.
		m.touchDockerContainerUpdateAlert(canonicalAlertID)
		return
	}

	// Check if an update is available
	if !container.UpdateStatus.UpdateAvailable {
		// No update available - clear tracking and alert
		m.clearAlert(canonicalAlertID)
		m.clearDockerContainerUpdateTracking(resourceID, updateTrackingKey)
		return
	}

	// Update is available - track when we first saw it
	m.mu.Lock()
	firstSeen, exists := m.dockerUpdateFirstSeenByIdentity[updateTrackingKey]
	if !exists {
		firstSeen, exists = m.dockerUpdateFirstSeen[resourceID]
	}
	if !exists {
		firstSeen = time.Now()
	}
	m.dockerUpdateFirstSeen[resourceID] = firstSeen
	m.dockerUpdateFirstSeenByIdentity[updateTrackingKey] = firstSeen
	m.mu.Unlock()

	// Check if we've exceeded the delay threshold
	pendingDuration := time.Since(firstSeen)
	threshold := time.Duration(delayHours) * time.Hour
	if pendingDuration < threshold {
		// Not yet time to alert
		log.Debug().
			Str("container", containerName).
			Str("host", host.DisplayName).
			Str("image", container.Image).
			Dur("pending", pendingDuration).
			Dur("threshold", threshold).
			Msg("Container update pending but below alert threshold")
		return
	}

	// Create or update the alert
	pendingHours := int(pendingDuration.Hours())
	spec, err := buildCanonicalSeverityThresholdSpec(resourceID+"-image-update", resourceID, containerName, unifiedresources.ResourceTypeAppContainer, "image-update-hours", float64(delayHours), 0, false)
	if err != nil {
		log.Warn().
			Err(err).
			Str("resourceID", resourceID).
			Str("container", containerName).
			Msg("Skipping invalid canonical docker container update spec")
		return
	}

	metadata := dockerContainerAlertMetadata(host, container, containerName)
	metadata["currentDigest"] = container.UpdateStatus.CurrentDigest
	metadata["latestDigest"] = container.UpdateStatus.LatestDigest
	metadata["lastChecked"] = container.UpdateStatus.LastChecked
	metadata["firstSeen"] = firstSeen
	metadata["pendingHours"] = pendingHours
	metadata["thresholdHours"] = delayHours

	_, _ = m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt: time.Now(),
			SeverityThreshold: &alertspecs.SeverityThresholdEvidence{
				Metric:    "image-update-hours",
				Direction: alertspecs.ThresholdDirectionAbove,
				Observed:  pendingDuration.Hours(),
			},
		},
		AlertID:           alertID,
		AlertType:         "docker-container-update",
		ResourceID:        resourceID,
		ResourceName:      containerName,
		Node:              nodeName,
		Instance:          instanceName,
		Message:           fmt.Sprintf("Docker container '%s' has an image update available for %d hours", containerName, pendingHours),
		StartTimeOverride: firstSeen,
		Metadata:          metadata,
		AddToRecent:       true,
		AddToHistory:      true,
	})

	log.Warn().
		Str("container", containerName).
		Str("host", host.DisplayName).
		Str("image", container.Image).
		Int("pendingHours", pendingHours).
		Msg("Docker container has pending image update")
}

func (m *Manager) cleanupDockerContainerAlerts(host models.DockerHost, seen map[string]struct{}) {
	m.cleanupDockerContainerAlertsWithTracking(host, seen, nil)
}

func (m *Manager) cleanupDockerContainerAlertsWithTracking(host models.DockerHost, seen map[string]struct{}, seenUpdateTracking map[string]struct{}) {
	prefix := fmt.Sprintf("docker:%s/", strings.TrimSpace(host.ID))
	updateTrackingPrefix := dockerUpdateTrackingHostPrefix(host)

	m.mu.Lock()
	toClear := make([]string, 0)
	for storageKey, alert := range m.activeAlerts {
		alertID := effectiveAlertID(alert, storageKey)
		if !strings.HasPrefix(alert.ResourceID, prefix) {
			continue
		}
		if _, exists := seen[alert.ResourceID]; exists {
			continue
		}
		toClear = append(toClear, alertID)
	}
	for resourceID := range m.dockerStateConfirm {
		if strings.HasPrefix(resourceID, prefix) {
			if _, exists := seen[resourceID]; !exists {
				delete(m.dockerStateConfirm, resourceID)
			}
		}
	}
	// Cleanup update tracking for removed containers
	for resourceID := range m.dockerUpdateFirstSeen {
		if strings.HasPrefix(resourceID, prefix) {
			if _, exists := seen[resourceID]; !exists {
				delete(m.dockerUpdateFirstSeen, resourceID)
			}
		}
	}
	if seenUpdateTracking != nil {
		for trackingKey := range m.dockerUpdateFirstSeenByIdentity {
			if !strings.HasPrefix(trackingKey, updateTrackingPrefix) {
				continue
			}
			if _, exists := seenUpdateTracking[trackingKey]; !exists {
				delete(m.dockerUpdateFirstSeenByIdentity, trackingKey)
			}
		}
	}
	m.mu.Unlock()

	for _, alertID := range toClear {
		m.clearAlert(alertID)
	}
}

func (m *Manager) clearDockerHostContainerAlerts(host models.DockerHost) {
	prefix := fmt.Sprintf("docker:%s/", strings.TrimSpace(host.ID))
	updateTrackingPrefix := dockerUpdateTrackingHostPrefix(host)

	m.mu.Lock()
	toClear := make([]string, 0)
	for storageKey, alert := range m.activeAlerts {
		alertID := effectiveAlertID(alert, storageKey)
		if strings.HasPrefix(alert.ResourceID, prefix) {
			toClear = append(toClear, alertID)
		}
	}
	for resourceID := range m.dockerStateConfirm {
		if strings.HasPrefix(resourceID, prefix) {
			delete(m.dockerStateConfirm, resourceID)
		}
	}
	for resourceID := range m.dockerRestartTracking {
		if strings.HasPrefix(resourceID, prefix) {
			delete(m.dockerRestartTracking, resourceID)
		}
	}
	for resourceID := range m.dockerLastExitCode {
		if strings.HasPrefix(resourceID, prefix) {
			delete(m.dockerLastExitCode, resourceID)
		}
	}
	for resourceID := range m.dockerUpdateFirstSeen {
		if strings.HasPrefix(resourceID, prefix) {
			delete(m.dockerUpdateFirstSeen, resourceID)
		}
	}
	for trackingKey := range m.dockerUpdateFirstSeenByIdentity {
		if strings.HasPrefix(trackingKey, updateTrackingPrefix) {
			delete(m.dockerUpdateFirstSeenByIdentity, trackingKey)
		}
	}
	m.mu.Unlock()

	for _, alertID := range toClear {
		m.clearAlert(alertID)
	}
}
