package unifiedresources

import "strings"

func RefreshCanonicalIdentity(resource *Resource) {
	if resource == nil {
		return
	}

	displayName := firstTrimmed(resource.Name, canonicalHostname(*resource), resource.ID)
	hostname := canonicalHostname(*resource)
	platformID := canonicalPlatformID(*resource)
	primaryID := canonicalPrimaryID(*resource)
	aliases := canonicalAliases(*resource, primaryID, platformID, hostname)

	if displayName == "" && hostname == "" && platformID == "" && primaryID == "" && len(aliases) == 0 {
		resource.Canonical = nil
		return
	}

	resource.Canonical = &CanonicalIdentity{
		DisplayName: displayName,
		Hostname:    hostname,
		PlatformID:  platformID,
		PrimaryID:   primaryID,
		Aliases:     aliases,
	}
}

func canonicalPrimaryID(resource Resource) string {
	if nodeID := canonicalProxmoxNodePrimaryID(resource); nodeID != "" {
		return nodeID
	}
	if identity := formatTargetIdentity(resource.MetricsTarget); identity != "" {
		return identity
	}
	if identity := formatTargetIdentity(resource.DiscoveryTarget); identity != "" {
		return identity
	}
	if runtimeID := strings.TrimSpace(canonicalDockerRuntimeID(resource)); runtimeID != "" {
		return "docker-host:" + runtimeID
	}
	if clusterID := strings.TrimSpace(canonicalKubernetesClusterID(resource)); clusterID != "" {
		switch CanonicalResourceType(resource.Type) {
		case ResourceTypeK8sCluster, ResourceTypeK8sNode, ResourceTypePod, ResourceTypeK8sDeployment:
			return "k8s:" + clusterID
		}
	}
	if agentID := strings.TrimSpace(canonicalAgentID(resource)); agentID != "" {
		return "agent:" + agentID
	}
	if instanceID := strings.TrimSpace(canonicalPBSInstanceID(resource)); instanceID != "" {
		return "pbs:" + instanceID
	}
	if instanceID := strings.TrimSpace(canonicalPMGInstanceID(resource)); instanceID != "" {
		return "pmg:" + instanceID
	}
	return strings.TrimSpace(resource.ID)
}

func canonicalProxmoxNodePrimaryID(resource Resource) string {
	if CanonicalResourceType(resource.Type) != ResourceTypeAgent || resource.Proxmox == nil {
		return ""
	}
	sourceID := strings.TrimSpace(resource.Proxmox.SourceID)
	if sourceID == "" {
		return ""
	}
	return "node:" + sourceID
}

func canonicalAliases(resource Resource, primaryID, platformID, hostname string) []string {
	values := []string{
		primaryID,
		targetResourceID(resource.MetricsTarget),
		targetAgentID(resource.DiscoveryTarget),
		targetResourceID(resource.DiscoveryTarget),
		canonicalDockerRuntimeID(resource),
		canonicalKubernetesClusterID(resource),
		canonicalAgentID(resource),
		canonicalPBSInstanceID(resource),
		canonicalPMGInstanceID(resource),
		platformID,
		hostname,
		strings.TrimSpace(resource.Identity.MachineID),
		strings.TrimSpace(resource.ID),
	}

	return uniqueTrimmed(values...)
}

func canonicalPlatformID(resource Resource) string {
	return firstTrimmed(
		canonicalProxmoxNodeName(resource),
		canonicalAgentHostname(resource),
		canonicalDockerHostname(resource),
		canonicalPBSHostname(resource),
		canonicalPMGHostname(resource),
		canonicalTrueNASHostname(resource),
		canonicalKubernetesPlatformID(resource),
		resource.Name,
		resource.ID,
	)
}

func canonicalHostname(resource Resource) string {
	return firstTrimmed(
		firstIdentityHostname(resource.Identity),
		canonicalAgentHostname(resource),
		canonicalDockerHostname(resource),
		canonicalPBSHostname(resource),
		canonicalPMGHostname(resource),
		canonicalTrueNASHostname(resource),
	)
}

func canonicalProxmoxNodeName(resource Resource) string {
	if resource.Proxmox == nil {
		return ""
	}
	return strings.TrimSpace(resource.Proxmox.NodeName)
}

func canonicalAgentHostname(resource Resource) string {
	if resource.Agent == nil {
		return ""
	}
	return strings.TrimSpace(resource.Agent.Hostname)
}

func canonicalDockerHostname(resource Resource) string {
	if resource.Docker == nil {
		return ""
	}
	return strings.TrimSpace(resource.Docker.Hostname)
}

func canonicalPBSHostname(resource Resource) string {
	if resource.PBS == nil {
		return ""
	}
	return strings.TrimSpace(resource.PBS.Hostname)
}

func canonicalPMGHostname(resource Resource) string {
	if resource.PMG == nil {
		return ""
	}
	return strings.TrimSpace(resource.PMG.Hostname)
}

func canonicalTrueNASHostname(resource Resource) string {
	if resource.TrueNAS == nil {
		return ""
	}
	return strings.TrimSpace(resource.TrueNAS.Hostname)
}

func canonicalKubernetesPlatformID(resource Resource) string {
	if resource.Kubernetes == nil {
		return ""
	}
	return firstTrimmed(
		resource.Kubernetes.NodeName,
		resource.Kubernetes.ClusterName,
		resource.Kubernetes.SourceName,
		resource.Kubernetes.Context,
		resource.Kubernetes.ClusterID,
	)
}

func canonicalAgentID(resource Resource) string {
	if resource.Agent != nil {
		if id := strings.TrimSpace(resource.Agent.AgentID); id != "" {
			return id
		}
	}
	if resource.Kubernetes != nil {
		if id := strings.TrimSpace(resource.Kubernetes.AgentID); id != "" {
			return id
		}
	}
	if resource.DiscoveryTarget != nil {
		if id := strings.TrimSpace(resource.DiscoveryTarget.AgentID); id != "" {
			return id
		}
	}
	return ""
}

func canonicalDockerRuntimeID(resource Resource) string {
	if resource.Docker != nil {
		if id := strings.TrimSpace(resource.Docker.HostSourceID); id != "" {
			return id
		}
	}
	return ""
}

func canonicalKubernetesClusterID(resource Resource) string {
	if resource.Kubernetes == nil {
		return ""
	}
	return firstTrimmed(resource.Kubernetes.ClusterID)
}

func canonicalPBSInstanceID(resource Resource) string {
	if resource.PBS == nil {
		return ""
	}
	return strings.TrimSpace(resource.PBS.InstanceID)
}

func canonicalPMGInstanceID(resource Resource) string {
	if resource.PMG == nil {
		return ""
	}
	return strings.TrimSpace(resource.PMG.InstanceID)
}

func firstIdentityHostname(identity ResourceIdentity) string {
	for _, hostname := range identity.Hostnames {
		if trimmed := strings.TrimSpace(hostname); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func formatTargetIdentity(target interface {
	GetResourceType() string
	GetResourceID() string
}) string {
	if target == nil {
		return ""
	}
	resourceType := strings.TrimSpace(target.GetResourceType())
	resourceID := strings.TrimSpace(target.GetResourceID())
	if resourceType == "" || resourceID == "" {
		return ""
	}
	return resourceType + ":" + resourceID
}

func targetResourceID(target interface{ GetResourceID() string }) string {
	if target == nil {
		return ""
	}
	return strings.TrimSpace(target.GetResourceID())
}

func targetAgentID(target interface{ GetAgentID() string }) string {
	if target == nil {
		return ""
	}
	return strings.TrimSpace(target.GetAgentID())
}

func firstTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func uniqueTrimmed(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	aliases := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		aliases = append(aliases, trimmed)
	}
	return aliases
}

func (target *DiscoveryTarget) GetResourceType() string {
	if target == nil {
		return ""
	}
	return target.ResourceType
}

func (target *DiscoveryTarget) GetAgentID() string {
	if target == nil {
		return ""
	}
	return target.AgentID
}

func (target *DiscoveryTarget) GetResourceID() string {
	if target == nil {
		return ""
	}
	return target.ResourceID
}

func (target *MetricsTarget) GetResourceType() string {
	if target == nil {
		return ""
	}
	return target.ResourceType
}

func (target *MetricsTarget) GetResourceID() string {
	if target == nil {
		return ""
	}
	return target.ResourceID
}
