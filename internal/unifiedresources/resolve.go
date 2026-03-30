package unifiedresources

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// ResolveResource looks up a resource by name using the ReadState interface
// and returns its location in the hierarchy. This is the ReadState equivalent
// of models.StateSnapshot.ResolveResource and is the preferred path for
// consumer packages that have been migrated away from direct state access.
func ResolveResource(rs ReadState, name string) models.ResourceLocation {
	name = strings.TrimSpace(name)
	if rs == nil {
		return models.ResourceLocation{Found: false, Name: name}
	}
	if name == "" {
		return models.ResourceLocation{Found: false, Name: name}
	}

	// Check Proxmox nodes first.
	// Match on NodeName() (raw Proxmox name) for parity with state.Nodes[].Name.
	for _, node := range rs.Nodes() {
		if node == nil {
			continue
		}
		nodeName := node.NodeName()
		if nodeName == name {
			return models.ResourceLocation{
				Found:        true,
				Name:         name,
				ResourceType: "node",
				Node:         nodeName,
				TargetHost:   nodeName,
			}
		}
	}

	// Check VMs.
	for _, vm := range rs.VMs() {
		if vm == nil {
			continue
		}
		if vm.Name() == name {
			return models.ResourceLocation{
				Found:        true,
				Name:         name,
				ResourceType: "vm",
				VMID:         vm.VMID(),
				Node:         vm.Node(),
				TargetHost:   vm.Name(),
			}
		}
	}

	// Check system containers (LXC/Incus).
	for _, ct := range rs.Containers() {
		if ct == nil {
			continue
		}
		if ct.Name() == name {
			return models.ResourceLocation{
				Found:        true,
				Name:         name,
				ResourceType: "system-container",
				VMID:         ct.VMID(),
				Node:         ct.Node(),
				TargetHost:   ct.Name(),
			}
		}
	}

	// Check Docker hosts (system-containers/VMs/standalone hosts running Docker).
	// Match on Hostname() or HostSourceID() for parity with state.DockerHosts[].Hostname/ID.
	// Note: Docker HOST lookups do NOT rewrite TargetHost — it stays as hostname.
	for _, dh := range rs.DockerHosts() {
		if dh == nil {
			continue
		}
		if dh.Hostname() == name || dh.HostSourceID() == name {
			loc := models.ResourceLocation{
				Found:          true,
				Name:           dh.Hostname(),
				ResourceType:   "docker-host",
				DockerHostName: dh.Hostname(),
				TargetHost:     dh.Hostname(),
			}
			resolveDockerHostParent(rs, dh.Hostname(), dh.HostSourceID(), false, &loc)
			return loc
		}
	}

	// Check Docker containers — flat list, resolve parent host.
	// Docker CONTAINER lookups DO rewrite TargetHost to the backing guest name.
	for _, resource := range appContainerResourcesFromReadState(rs) {
		if !appContainerResourceMatches(resource, name) {
			continue
		}
		loc := models.ResourceLocation{
			Found:        true,
			Name:         name,
			ResourceType: "app-container",
		}
		populateAppContainerLocation(rs, resource, &loc)
		return loc
	}

	// Check storage pools, including VMware datastores and shared platform storage.
	for _, pool := range rs.StoragePools() {
		if pool == nil {
			continue
		}
		if !storagePoolMatches(pool, name) {
			continue
		}
		targetID := storagePoolTarget(pool)
		return models.ResourceLocation{
			Found:        true,
			Name:         pool.Name(),
			ResourceType: "storage",
			TargetID:     targetID,
			TargetHost:   targetID,
		}
	}

	// Check generic Hosts (Windows/Linux via Pulse Unified Agent).
	// Match on Hostname() or AgentID() for parity with state.Hosts[].Hostname/ID.
	// Use AgentID() as canonical target ID for host resources.
	for _, host := range rs.Hosts() {
		if host == nil {
			continue
		}
		agentID := host.AgentID()
		if host.Hostname() == name || agentID == name {
			return models.ResourceLocation{
				Found:        true,
				Name:         host.Hostname(),
				ResourceType: "agent",
				TargetID:     agentID,
				Platform:     host.Platform(),
				TargetHost:   host.Hostname(),
			}
		}
	}

	// Check Kubernetes clusters, pods, and deployments.
	// Build a map from cluster resource ID → cluster view for pod/deployment resolution.
	clusterByID := make(map[string]*K8sClusterView, len(rs.K8sClusters()))
	for _, cluster := range rs.K8sClusters() {
		if cluster == nil {
			continue
		}
		clusterByID[cluster.ID()] = cluster

		// Match by SourceName (original cluster.Name), ClusterID (original cluster.ID),
		// or unified Name (display name) for parity with state.KubernetesClusters[].Name/ID/DisplayName.
		sourceName := cluster.SourceName()
		clusterName := cluster.Name()
		if sourceName == name || cluster.ClusterID() == name || clusterName == name {
			// Return fields use SourceName() for Name/K8sClusterName/TargetHost
			// to match old resolver which used cluster.Name (raw source name).
			// Fall back to display name if source name is empty.
			returnName := sourceName
			if returnName == "" {
				returnName = clusterName
			}
			return models.ResourceLocation{
				Found:          true,
				Name:           returnName,
				ResourceType:   "k8s-cluster",
				K8sClusterName: returnName,
				K8sAgentID:     cluster.AgentID(),
				TargetHost:     returnName,
				AgentID:        cluster.AgentID(),
			}
		}
	}

	// Check pods.
	for _, pod := range rs.Pods() {
		if pod == nil {
			continue
		}
		if pod.Name() == name {
			loc := models.ResourceLocation{
				Found:        true,
				Name:         pod.Name(),
				ResourceType: "k8s-pod",
				K8sNamespace: pod.Namespace(),
			}
			if cluster := clusterByID[pod.ParentID()]; cluster != nil {
				cn := clusterSourceOrDisplayName(cluster)
				loc.K8sClusterName = cn
				loc.K8sAgentID = cluster.AgentID()
				loc.TargetHost = cn
				loc.AgentID = cluster.AgentID()
			}
			return loc
		}
	}

	// Check deployments.
	for _, deploy := range rs.K8sDeployments() {
		if deploy == nil {
			continue
		}
		if deploy.Name() == name {
			loc := models.ResourceLocation{
				Found:        true,
				Name:         deploy.Name(),
				ResourceType: "k8s-deployment",
				K8sNamespace: deploy.Namespace(),
			}
			if cluster := clusterByID[deploy.ParentID()]; cluster != nil {
				cn := clusterSourceOrDisplayName(cluster)
				loc.K8sClusterName = cn
				loc.K8sAgentID = cluster.AgentID()
				loc.TargetHost = cn
				loc.AgentID = cluster.AgentID()
			}
			return loc
		}
	}

	return models.ResourceLocation{Found: false, Name: name}
}

// clusterSourceOrDisplayName returns SourceName() (raw cluster.Name) when
// available, falling back to the unified display Name(). This matches the
// old StateSnapshot.ResolveResource which used cluster.Name for returned fields.
func clusterSourceOrDisplayName(c *K8sClusterView) string {
	if sn := c.SourceName(); sn != "" {
		return sn
	}
	return c.Name()
}

// resolveDockerHostParent checks whether a Docker host is backed by an LXC
// container, a VM, or is standalone, and fills the corresponding fields in loc.
// When rewriteTarget is true (docker container lookups), TargetHost is rewritten
// to the backing guest name. When false (docker host lookups), TargetHost stays
// as the docker hostname — matching StateSnapshot.ResolveResource parity.
func resolveDockerHostParent(rs ReadState, hostname, sourceID string, rewriteTarget bool, loc *models.ResourceLocation) {
	// Check if the Docker host is a system container.
	for _, ct := range rs.Containers() {
		if ct == nil {
			continue
		}
		if ct.Name() == hostname || ct.Name() == sourceID {
			loc.DockerHostType = "system-container"
			loc.DockerHostVMID = ct.VMID()
			loc.Node = ct.Node()
			if rewriteTarget {
				loc.TargetHost = ct.Name()
			}
			return
		}
	}
	// Check if the Docker host is a VM.
	for _, vm := range rs.VMs() {
		if vm == nil {
			continue
		}
		if vm.Name() == hostname || vm.Name() == sourceID {
			loc.DockerHostType = "vm"
			loc.DockerHostVMID = vm.VMID()
			loc.Node = vm.Node()
			if rewriteTarget {
				loc.TargetHost = vm.Name()
			}
			return
		}
	}
	loc.DockerHostType = "standalone"
}

type appContainerResourceGetter interface {
	GetByType(ResourceType) []Resource
}

type appContainerResourceLister interface {
	ListByType(ResourceType) []Resource
}

func appContainerResourcesFromReadState(rs ReadState) []Resource {
	if rs == nil {
		return nil
	}
	if getter, ok := any(rs).(appContainerResourceGetter); ok {
		return getter.GetByType(ResourceTypeAppContainer)
	}
	if lister, ok := any(rs).(appContainerResourceLister); ok {
		return lister.ListByType(ResourceTypeAppContainer)
	}

	containers := rs.DockerContainers()
	if len(containers) == 0 {
		return nil
	}
	resources := make([]Resource, 0, len(containers))
	for _, container := range containers {
		if container == nil || container.r == nil {
			continue
		}
		resources = append(resources, cloneResource(container.r))
	}
	return resources
}

func appContainerResourceMatches(resource Resource, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(resource.Name), name) || strings.EqualFold(strings.TrimSpace(resource.ID), name) {
		return true
	}
	if resource.Docker != nil && strings.EqualFold(strings.TrimSpace(resource.Docker.ContainerID), name) {
		return true
	}
	if resource.Canonical != nil {
		for _, alias := range resource.Canonical.Aliases {
			if strings.EqualFold(strings.TrimSpace(alias), name) {
				return true
			}
		}
	}
	return false
}

func storagePoolMatches(pool *StoragePoolView, name string) bool {
	name = strings.TrimSpace(name)
	if pool == nil || name == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(pool.Name()), name) ||
		strings.EqualFold(strings.TrimSpace(pool.ID()), name) ||
		strings.EqualFold(strings.TrimSpace(pool.SourceID()), name) {
		return true
	}
	if pool.r != nil && pool.r.Canonical != nil {
		for _, alias := range pool.r.Canonical.Aliases {
			if strings.EqualFold(strings.TrimSpace(alias), name) {
				return true
			}
		}
	}
	return false
}

func storagePoolTarget(pool *StoragePoolView) string {
	if pool == nil {
		return ""
	}
	vmwareConnectionName := ""
	vmwareRuntimeHost := ""
	parentName := ""
	if pool.r != nil {
		parentName = strings.TrimSpace(pool.r.ParentName)
		if pool.r.VMware != nil {
			vmwareConnectionName = strings.TrimSpace(pool.r.VMware.ConnectionName)
			vmwareRuntimeHost = strings.TrimSpace(pool.r.VMware.RuntimeHostName)
		}
	}
	return firstNonEmptyResolvedString(
		pool.Node(),
		pool.Instance(),
		parentName,
		vmwareConnectionName,
		vmwareRuntimeHost,
		pool.ParentName(),
		pool.Name(),
	)
}

func firstNonEmptyResolvedString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func appContainerResourceHost(resource Resource) string {
	if host := strings.TrimSpace(resource.ParentName); host != "" {
		return host
	}
	if resource.Docker != nil {
		if host := strings.TrimSpace(resource.Docker.Hostname); host != "" {
			return host
		}
	}
	if resource.TrueNAS != nil {
		if host := strings.TrimSpace(resource.TrueNAS.Hostname); host != "" {
			return host
		}
	}
	for _, host := range resource.Identity.Hostnames {
		host = strings.TrimSpace(host)
		if host != "" {
			return host
		}
	}
	return ""
}

func populateAppContainerLocation(rs ReadState, resource Resource, loc *models.ResourceLocation) {
	if loc == nil {
		return
	}
	hostName := appContainerResourceHost(resource)
	if hostName == "" {
		return
	}
	loc.TargetHost = hostName

	if !appContainerUsesDockerRouting(resource) {
		return
	}

	loc.DockerHostName = hostName
	hostSourceID := hostName
	if resource.ParentID != nil {
		if parentID := strings.TrimSpace(*resource.ParentID); parentID != "" {
			hostSourceID = parentID
		}
	}
	resolveDockerHostParent(rs, hostName, hostSourceID, true, loc)
}

func appContainerUsesDockerRouting(resource Resource) bool {
	if resourceHasTagCaseInsensitive(resource, "truenas") {
		return false
	}
	if resource.Docker != nil {
		return true
	}
	return resourceHasTagCaseInsensitive(resource, "docker")
}

func resourceHasTagCaseInsensitive(resource Resource, needle string) bool {
	for _, tag := range resource.Tags {
		if strings.EqualFold(strings.TrimSpace(tag), needle) {
			return true
		}
	}
	return false
}
