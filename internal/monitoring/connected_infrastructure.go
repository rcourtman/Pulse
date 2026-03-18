package monitoring

import (
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type connectedInfrastructureGroup struct {
	id                string
	name              string
	displayName       string
	hostname          string
	status            string
	healthStatus      string
	lastSeen          int64
	removedAt         int64
	version           string
	isOutdatedBinary  bool
	linkedNodeID      string
	commandsEnabled   bool
	scopeAgentID      string
	upgradePlatform   string
	uninstallAgentID  string
	uninstallHostname string
	surfaces          map[string]models.ConnectedInfrastructureSurfaceFrontend
}

func buildConnectedInfrastructure(
	resources []unifiedresources.Resource,
	snapshot models.StateSnapshot,
) []models.ConnectedInfrastructureItemFrontend {
	groups := make(map[string]*connectedInfrastructureGroup)

	for _, resource := range resources {
		appendConnectedInfrastructureSurfaces(groups, resource)
	}

	applyConnectedInfrastructureIgnoreState(groups, snapshot)

	for _, removed := range snapshot.RemovedHostAgents {
		item := removedConnectedInfrastructureItem("agent", removed.ID, removed.Hostname, removed.DisplayName, removed.RemovedAt.UnixMilli())
		item.Surfaces = []models.ConnectedInfrastructureSurfaceFrontend{{
			ID:        "agent:" + removed.ID,
			Kind:      "agent",
			Label:     "Host telemetry",
			Detail:    "Pulse is blocking host telemetry from this machine.",
			ControlID: removed.ID,
			Action:    "allow-reconnect",
			IDLabel:   "Agent ID",
			IDValue:   removed.ID,
		}}
		groups[item.ID] = connectedInfrastructureGroupFromFrontend(item)
	}

	for _, removed := range snapshot.RemovedDockerHosts {
		item := removedConnectedInfrastructureItem("docker", removed.ID, removed.Hostname, removed.DisplayName, removed.RemovedAt.UnixMilli())
		item.Surfaces = []models.ConnectedInfrastructureSurfaceFrontend{{
			ID:        "docker:" + removed.ID,
			Kind:      "docker",
			Label:     "Docker runtime data",
			Detail:    "Pulse is blocking Docker runtime reports from this machine.",
			ControlID: removed.ID,
			Action:    "allow-reconnect",
			IDLabel:   "Docker runtime ID",
			IDValue:   removed.ID,
		}}
		groups[item.ID] = connectedInfrastructureGroupFromFrontend(item)
	}

	for _, removed := range snapshot.RemovedKubernetesClusters {
		name := strings.TrimSpace(removed.Name)
		if name == "" {
			name = strings.TrimSpace(removed.DisplayName)
		}
		if name == "" {
			name = removed.ID
		}
		item := models.ConnectedInfrastructureItemFrontend{
			ID:              "ignored:kubernetes:" + removed.ID,
			Name:            name,
			DisplayName:     strings.TrimSpace(removed.DisplayName),
			Status:          "ignored",
			RemovedAt:       removed.RemovedAt.UnixMilli(),
			UpgradePlatform: "linux",
			Surfaces: []models.ConnectedInfrastructureSurfaceFrontend{{
				ID:        "kubernetes:" + removed.ID,
				Kind:      "kubernetes",
				Label:     "Kubernetes cluster data",
				Detail:    "Pulse is blocking Kubernetes telemetry for this cluster.",
				ControlID: removed.ID,
				Action:    "allow-reconnect",
				IDLabel:   "Cluster ID",
				IDValue:   removed.ID,
			}},
		}
		groups[item.ID] = connectedInfrastructureGroupFromFrontend(item)
	}

	items := make([]models.ConnectedInfrastructureItemFrontend, 0, len(groups))
	for _, group := range groups {
		surfaces := make([]models.ConnectedInfrastructureSurfaceFrontend, 0, len(group.surfaces))
		for _, surface := range group.surfaces {
			surfaces = append(surfaces, surface)
		}
		sort.Slice(surfaces, func(i, j int) bool {
			if surfaces[i].Kind == surfaces[j].Kind {
				return surfaces[i].ID < surfaces[j].ID
			}
			return connectedInfrastructureSurfaceOrder(surfaces[i].Kind) < connectedInfrastructureSurfaceOrder(surfaces[j].Kind)
		})

		items = append(items, models.ConnectedInfrastructureItemFrontend{
			ID:                group.id,
			Name:              group.name,
			DisplayName:       group.displayName,
			Hostname:          group.hostname,
			Status:            group.status,
			HealthStatus:      group.healthStatus,
			LastSeen:          group.lastSeen,
			RemovedAt:         group.removedAt,
			Version:           group.version,
			IsOutdatedBinary:  group.isOutdatedBinary,
			LinkedNodeID:      group.linkedNodeID,
			CommandsEnabled:   group.commandsEnabled,
			ScopeAgentID:      group.scopeAgentID,
			UpgradePlatform:   group.upgradePlatform,
			UninstallAgentID:  group.uninstallAgentID,
			UninstallHostname: group.uninstallHostname,
			Surfaces:          surfaces,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Status != items[j].Status {
			return items[i].Status < items[j].Status
		}
		left := strings.ToLower(strings.TrimSpace(items[i].Name))
		right := strings.ToLower(strings.TrimSpace(items[j].Name))
		if left == right {
			return items[i].ID < items[j].ID
		}
		return left < right
	})

	return items
}

func applyConnectedInfrastructureIgnoreState(
	groups map[string]*connectedInfrastructureGroup,
	snapshot models.StateSnapshot,
) {
	ignoredHostIDs := make(map[string]struct{}, len(snapshot.RemovedHostAgents))
	for _, removed := range snapshot.RemovedHostAgents {
		if controlID := strings.TrimSpace(removed.ID); controlID != "" {
			ignoredHostIDs[controlID] = struct{}{}
		}
	}

	ignoredDockerIDs := make(map[string]struct{}, len(snapshot.RemovedDockerHosts))
	for _, removed := range snapshot.RemovedDockerHosts {
		if controlID := strings.TrimSpace(removed.ID); controlID != "" {
			ignoredDockerIDs[controlID] = struct{}{}
		}
	}

	ignoredKubernetesIDs := make(map[string]struct{}, len(snapshot.RemovedKubernetesClusters))
	for _, removed := range snapshot.RemovedKubernetesClusters {
		if controlID := strings.TrimSpace(removed.ID); controlID != "" {
			ignoredKubernetesIDs[controlID] = struct{}{}
		}
	}

	for _, group := range groups {
		if group.status != "active" {
			continue
		}

		if agentSurface, ok := group.surfaces["agent"]; ok {
			if _, ignored := ignoredHostIDs[strings.TrimSpace(agentSurface.ControlID)]; ignored {
				delete(group.surfaces, "agent")
				delete(group.surfaces, "proxmox")
				delete(group.surfaces, "pbs")
				delete(group.surfaces, "pmg")
			}
		}

		if dockerSurface, ok := group.surfaces["docker"]; ok {
			if _, ignored := ignoredDockerIDs[strings.TrimSpace(dockerSurface.ControlID)]; ignored {
				delete(group.surfaces, "docker")
			}
		}

		if kubernetesSurface, ok := group.surfaces["kubernetes"]; ok {
			if _, ignored := ignoredKubernetesIDs[strings.TrimSpace(kubernetesSurface.ControlID)]; ignored {
				delete(group.surfaces, "kubernetes")
			}
		}
	}
}

func appendConnectedInfrastructureSurfaces(
	groups map[string]*connectedInfrastructureGroup,
	resource unifiedresources.Resource,
) {
	if surface, ok := connectedInfrastructureAgentSurface(resource); ok {
		addConnectedInfrastructureSurface(groups, resource, surface)
	}
	if surface, ok := connectedInfrastructureDockerSurface(resource); ok {
		addConnectedInfrastructureSurface(groups, resource, surface)
	}
	if surface, ok := connectedInfrastructureKubernetesSurface(resource); ok {
		addConnectedInfrastructureSurface(groups, resource, surface)
	}
	if surface, ok := connectedInfrastructureProxmoxSurface(resource); ok {
		addConnectedInfrastructureSurface(groups, resource, surface)
	}
	if surface, ok := connectedInfrastructurePBSSurface(resource); ok {
		addConnectedInfrastructureSurface(groups, resource, surface)
	}
	if surface, ok := connectedInfrastructurePMGSurface(resource); ok {
		addConnectedInfrastructureSurface(groups, resource, surface)
	}
}

func addConnectedInfrastructureSurface(
	groups map[string]*connectedInfrastructureGroup,
	resource unifiedresources.Resource,
	surface models.ConnectedInfrastructureSurfaceFrontend,
) {
	key := connectedInfrastructureGroupKey(resource, surface.Kind)
	group, exists := groups[key]
	if !exists {
		group = &connectedInfrastructureGroup{
			id:                key,
			name:              connectedInfrastructureName(resource),
			displayName:       connectedInfrastructureDisplayName(resource),
			hostname:          connectedInfrastructureHostname(resource),
			status:            "active",
			healthStatus:      strings.TrimSpace(string(resource.Status)),
			lastSeen:          resource.LastSeen.UnixMilli(),
			version:           connectedInfrastructureVersion(resource),
			isOutdatedBinary:  connectedInfrastructureIsOutdated(resource),
			linkedNodeID:      connectedInfrastructureLinkedNodeID(resource),
			commandsEnabled:   connectedInfrastructureCommandsEnabled(resource),
			scopeAgentID:      connectedInfrastructureScopeAgentID(resource),
			upgradePlatform:   connectedInfrastructureUpgradePlatform(resource),
			uninstallAgentID:  connectedInfrastructureUninstallAgentID(resource),
			uninstallHostname: connectedInfrastructureHostname(resource),
			surfaces:          make(map[string]models.ConnectedInfrastructureSurfaceFrontend),
		}
		groups[key] = group
	}

	if resourceLastSeen := resource.LastSeen.UnixMilli(); resourceLastSeen > group.lastSeen {
		group.lastSeen = resourceLastSeen
	}
	if group.version == "" {
		group.version = connectedInfrastructureVersion(resource)
	}
	if !group.isOutdatedBinary {
		group.isOutdatedBinary = connectedInfrastructureIsOutdated(resource)
	}
	if group.linkedNodeID == "" {
		group.linkedNodeID = connectedInfrastructureLinkedNodeID(resource)
	}
	if !group.commandsEnabled {
		group.commandsEnabled = connectedInfrastructureCommandsEnabled(resource)
	}
	if group.scopeAgentID == "" {
		group.scopeAgentID = connectedInfrastructureScopeAgentID(resource)
	}
	if group.upgradePlatform == "" {
		group.upgradePlatform = connectedInfrastructureUpgradePlatform(resource)
	}
	if group.uninstallAgentID == "" {
		group.uninstallAgentID = connectedInfrastructureUninstallAgentID(resource)
	}
	if group.uninstallHostname == "" {
		group.uninstallHostname = connectedInfrastructureHostname(resource)
	}
	if group.hostname == "" {
		group.hostname = connectedInfrastructureHostname(resource)
	}
	if group.displayName == "" {
		group.displayName = connectedInfrastructureDisplayName(resource)
	}
	if group.name == "" {
		group.name = connectedInfrastructureName(resource)
	}

	group.surfaces[surface.Kind] = surface
}

func connectedInfrastructureGroupFromFrontend(
	item models.ConnectedInfrastructureItemFrontend,
) *connectedInfrastructureGroup {
	surfaces := make(map[string]models.ConnectedInfrastructureSurfaceFrontend, len(item.Surfaces))
	for _, surface := range item.Surfaces {
		surfaces[surface.Kind] = surface
	}
	return &connectedInfrastructureGroup{
		id:                item.ID,
		name:              item.Name,
		displayName:       item.DisplayName,
		hostname:          item.Hostname,
		status:            item.Status,
		healthStatus:      item.HealthStatus,
		lastSeen:          item.LastSeen,
		removedAt:         item.RemovedAt,
		version:           item.Version,
		isOutdatedBinary:  item.IsOutdatedBinary,
		linkedNodeID:      item.LinkedNodeID,
		commandsEnabled:   item.CommandsEnabled,
		scopeAgentID:      item.ScopeAgentID,
		upgradePlatform:   item.UpgradePlatform,
		uninstallAgentID:  item.UninstallAgentID,
		uninstallHostname: item.UninstallHostname,
		surfaces:          surfaces,
	}
}

func removedConnectedInfrastructureItem(
	kind string,
	controlID string,
	hostname string,
	displayName string,
	removedAt int64,
) models.ConnectedInfrastructureItemFrontend {
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = strings.TrimSpace(hostname)
	}
	if name == "" {
		name = controlID
	}
	return models.ConnectedInfrastructureItemFrontend{
		ID:                "ignored:" + kind + ":" + controlID,
		Name:              name,
		DisplayName:       strings.TrimSpace(displayName),
		Hostname:          strings.TrimSpace(hostname),
		Status:            "ignored",
		RemovedAt:         removedAt,
		UpgradePlatform:   "linux",
		UninstallHostname: strings.TrimSpace(hostname),
	}
}

func connectedInfrastructureGroupKey(resource unifiedresources.Resource, surfaceKind string) string {
	if surfaceKind == "kubernetes" {
		if resource.Kubernetes != nil && strings.TrimSpace(resource.Kubernetes.ClusterID) != "" {
			return "kubernetes:" + strings.TrimSpace(resource.Kubernetes.ClusterID)
		}
		return "kubernetes:" + resource.ID
	}
	if machineID := connectedInfrastructureMachineID(resource); machineID != "" {
		return "machine:" + machineID
	}
	if hostname := connectedInfrastructureHostname(resource); hostname != "" {
		return "host:" + strings.ToLower(hostname)
	}
	if resource.Canonical != nil && strings.TrimSpace(resource.Canonical.PrimaryID) != "" {
		return "primary:" + strings.TrimSpace(resource.Canonical.PrimaryID)
	}
	return "resource:" + resource.ID
}

func connectedInfrastructureMachineID(resource unifiedresources.Resource) string {
	switch {
	case resource.Agent != nil && strings.TrimSpace(resource.Agent.MachineID) != "":
		return strings.TrimSpace(resource.Agent.MachineID)
	case resource.Docker != nil && strings.TrimSpace(resource.Docker.MachineID) != "":
		return strings.TrimSpace(resource.Docker.MachineID)
	case strings.TrimSpace(resource.Identity.MachineID) != "":
		return strings.TrimSpace(resource.Identity.MachineID)
	default:
		return ""
	}
}

func connectedInfrastructureHostname(resource unifiedresources.Resource) string {
	for _, candidate := range []string{
		connectedInfrastructureAgentHostname(resource),
		connectedInfrastructureDockerHostname(resource),
		connectedInfrastructurePBSHostname(resource),
		connectedInfrastructurePMGHostname(resource),
		connectedInfrastructureCanonicalHostname(resource),
		strings.TrimSpace(resource.Name),
	} {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func connectedInfrastructureName(resource unifiedresources.Resource) string {
	for _, candidate := range []string{
		connectedInfrastructureCanonicalDisplayName(resource),
		strings.TrimSpace(resource.Name),
		connectedInfrastructureHostname(resource),
		resource.ID,
	} {
		if candidate != "" {
			return candidate
		}
	}
	return resource.ID
}

func connectedInfrastructureDisplayName(resource unifiedresources.Resource) string {
	if resource.Canonical != nil {
		return strings.TrimSpace(resource.Canonical.DisplayName)
	}
	return ""
}

func connectedInfrastructureVersion(resource unifiedresources.Resource) string {
	switch {
	case resource.Agent != nil && strings.TrimSpace(resource.Agent.AgentVersion) != "":
		return strings.TrimSpace(resource.Agent.AgentVersion)
	case resource.Docker != nil && strings.TrimSpace(resource.Docker.AgentVersion) != "":
		return strings.TrimSpace(resource.Docker.AgentVersion)
	case resource.Docker != nil && strings.TrimSpace(resource.Docker.DockerVersion) != "":
		return strings.TrimSpace(resource.Docker.DockerVersion)
	case resource.Kubernetes != nil && strings.TrimSpace(resource.Kubernetes.AgentVersion) != "":
		return strings.TrimSpace(resource.Kubernetes.AgentVersion)
	case resource.Kubernetes != nil && strings.TrimSpace(resource.Kubernetes.Version) != "":
		return strings.TrimSpace(resource.Kubernetes.Version)
	case resource.PBS != nil && strings.TrimSpace(resource.PBS.Version) != "":
		return strings.TrimSpace(resource.PBS.Version)
	case resource.PMG != nil && strings.TrimSpace(resource.PMG.Version) != "":
		return strings.TrimSpace(resource.PMG.Version)
	case resource.Proxmox != nil && strings.TrimSpace(resource.Proxmox.PVEVersion) != "":
		return strings.TrimSpace(resource.Proxmox.PVEVersion)
	default:
		return ""
	}
}

func connectedInfrastructureIsOutdated(resource unifiedresources.Resource) bool {
	return (resource.Agent != nil && resource.Agent.IsLegacy) || (resource.Docker != nil && resource.Docker.IsLegacy)
}

func connectedInfrastructureLinkedNodeID(resource unifiedresources.Resource) string {
	if resource.Agent != nil {
		return strings.TrimSpace(resource.Agent.LinkedNodeID)
	}
	return ""
}

func connectedInfrastructureCommandsEnabled(resource unifiedresources.Resource) bool {
	return resource.Agent != nil && resource.Agent.CommandsEnabled
}

func connectedInfrastructureScopeAgentID(resource unifiedresources.Resource) string {
	switch {
	case resource.Agent != nil && strings.TrimSpace(resource.Agent.AgentID) != "":
		return strings.TrimSpace(resource.Agent.AgentID)
	case resource.Kubernetes != nil && strings.TrimSpace(resource.Kubernetes.AgentID) != "":
		return strings.TrimSpace(resource.Kubernetes.AgentID)
	case resource.Docker != nil && strings.TrimSpace(resource.Docker.AgentID) != "":
		return strings.TrimSpace(resource.Docker.AgentID)
	default:
		return ""
	}
}

func connectedInfrastructureUninstallAgentID(resource unifiedresources.Resource) string {
	if resource.Agent != nil && strings.TrimSpace(resource.Agent.AgentID) != "" {
		return strings.TrimSpace(resource.Agent.AgentID)
	}
	return ""
}

func connectedInfrastructureUpgradePlatform(resource unifiedresources.Resource) string {
	if resource.Agent != nil {
		switch strings.ToLower(strings.TrimSpace(resource.Agent.Platform)) {
		case "windows":
			return "windows"
		case "darwin", "macos", "mac":
			return "macos"
		case "freebsd":
			return "freebsd"
		}
	}
	return "linux"
}

func connectedInfrastructureAgentSurface(
	resource unifiedresources.Resource,
) (models.ConnectedInfrastructureSurfaceFrontend, bool) {
	if resource.Agent == nil || strings.TrimSpace(resource.Agent.AgentID) == "" {
		return models.ConnectedInfrastructureSurfaceFrontend{}, false
	}
	agentID := strings.TrimSpace(resource.Agent.AgentID)
	return models.ConnectedInfrastructureSurfaceFrontend{
		ID:        "agent:" + agentID,
		Kind:      "agent",
		Label:     "Host telemetry",
		Detail:    "System health, inventory, and Pulse command connectivity.",
		ControlID: agentID,
		Action:    "stop-monitoring",
		IDLabel:   "Agent ID",
		IDValue:   agentID,
	}, true
}

func connectedInfrastructureDockerSurface(
	resource unifiedresources.Resource,
) (models.ConnectedInfrastructureSurfaceFrontend, bool) {
	if resource.Docker == nil {
		return models.ConnectedInfrastructureSurfaceFrontend{}, false
	}
	hostSourceID := strings.TrimSpace(resource.Docker.HostSourceID)
	if hostSourceID == "" {
		hostSourceID = strings.TrimSpace(resource.Docker.MachineID)
	}
	if hostSourceID == "" {
		hostSourceID = strings.TrimSpace(resource.Docker.AgentID)
	}
	if hostSourceID == "" {
		hostSourceID = strings.TrimSpace(resource.Identity.MachineID)
	}
	if hostSourceID == "" {
		hostSourceID = resource.ID
	}
	return models.ConnectedInfrastructureSurfaceFrontend{
		ID:        "docker:" + hostSourceID,
		Kind:      "docker",
		Label:     "Docker runtime data",
		Detail:    "Container runtime coverage reported from this machine.",
		ControlID: hostSourceID,
		Action:    "stop-monitoring",
		IDLabel:   "Docker runtime ID",
		IDValue:   hostSourceID,
	}, true
}

func connectedInfrastructureKubernetesSurface(
	resource unifiedresources.Resource,
) (models.ConnectedInfrastructureSurfaceFrontend, bool) {
	if resource.Kubernetes == nil || strings.TrimSpace(resource.Kubernetes.ClusterID) == "" {
		return models.ConnectedInfrastructureSurfaceFrontend{}, false
	}
	clusterID := strings.TrimSpace(resource.Kubernetes.ClusterID)
	return models.ConnectedInfrastructureSurfaceFrontend{
		ID:        "kubernetes:" + clusterID,
		Kind:      "kubernetes",
		Label:     "Kubernetes cluster data",
		Detail:    "Cluster inventory and Kubernetes telemetry reported through Pulse.",
		ControlID: clusterID,
		Action:    "stop-monitoring",
		IDLabel:   "Cluster ID",
		IDValue:   clusterID,
	}, true
}

func connectedInfrastructureProxmoxSurface(
	resource unifiedresources.Resource,
) (models.ConnectedInfrastructureSurfaceFrontend, bool) {
	if resource.Proxmox == nil {
		return models.ConnectedInfrastructureSurfaceFrontend{}, false
	}
	sourceID := strings.TrimSpace(resource.Proxmox.SourceID)
	if sourceID == "" {
		sourceID = resource.ID
	}
	return models.ConnectedInfrastructureSurfaceFrontend{
		ID:      "proxmox:" + sourceID,
		Kind:    "proxmox",
		Label:   "Proxmox data",
		Detail:  "Proxmox node telemetry linked to this machine.",
		IDLabel: "Node ID",
		IDValue: sourceID,
	}, true
}

func connectedInfrastructurePBSSurface(
	resource unifiedresources.Resource,
) (models.ConnectedInfrastructureSurfaceFrontend, bool) {
	if resource.PBS == nil {
		return models.ConnectedInfrastructureSurfaceFrontend{}, false
	}
	instanceID := strings.TrimSpace(resource.PBS.InstanceID)
	if instanceID == "" {
		instanceID = resource.ID
	}
	return models.ConnectedInfrastructureSurfaceFrontend{
		ID:      "pbs:" + instanceID,
		Kind:    "pbs",
		Label:   "PBS data",
		Detail:  "Proxmox Backup Server inventory and backup telemetry.",
		IDLabel: "PBS ID",
		IDValue: instanceID,
	}, true
}

func connectedInfrastructurePMGSurface(
	resource unifiedresources.Resource,
) (models.ConnectedInfrastructureSurfaceFrontend, bool) {
	if resource.PMG == nil {
		return models.ConnectedInfrastructureSurfaceFrontend{}, false
	}
	instanceID := strings.TrimSpace(resource.PMG.InstanceID)
	if instanceID == "" {
		instanceID = resource.ID
	}
	return models.ConnectedInfrastructureSurfaceFrontend{
		ID:      "pmg:" + instanceID,
		Kind:    "pmg",
		Label:   "PMG data",
		Detail:  "Proxmox Mail Gateway telemetry.",
		IDLabel: "PMG ID",
		IDValue: instanceID,
	}, true
}

func connectedInfrastructureSurfaceOrder(kind string) int {
	switch kind {
	case "agent":
		return 0
	case "docker":
		return 1
	case "kubernetes":
		return 2
	case "proxmox":
		return 3
	case "pbs":
		return 4
	case "pmg":
		return 5
	default:
		return 99
	}
}

func connectedInfrastructureAgentHostname(resource unifiedresources.Resource) string {
	if resource.Agent != nil {
		return strings.TrimSpace(resource.Agent.Hostname)
	}
	return ""
}

func connectedInfrastructureDockerHostname(resource unifiedresources.Resource) string {
	if resource.Docker != nil {
		if hostname := strings.TrimSpace(resource.Docker.Hostname); hostname != "" {
			return hostname
		}
		if displayName := strings.TrimSpace(resource.Docker.DisplayName); displayName != "" {
			return displayName
		}
	}
	return ""
}

func connectedInfrastructurePBSHostname(resource unifiedresources.Resource) string {
	if resource.PBS != nil {
		return strings.TrimSpace(resource.PBS.Hostname)
	}
	return ""
}

func connectedInfrastructurePMGHostname(resource unifiedresources.Resource) string {
	if resource.PMG != nil {
		return strings.TrimSpace(resource.PMG.Hostname)
	}
	return ""
}

func connectedInfrastructureCanonicalHostname(resource unifiedresources.Resource) string {
	if resource.Canonical != nil {
		return strings.TrimSpace(resource.Canonical.Hostname)
	}
	return ""
}

func connectedInfrastructureCanonicalDisplayName(resource unifiedresources.Resource) string {
	if resource.Canonical != nil {
		return strings.TrimSpace(resource.Canonical.DisplayName)
	}
	return ""
}
