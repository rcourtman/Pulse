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
	return ResolveResourceContext(rs, name).Location
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
	var guest *Resource
	if resolver, ok := rs.(ReferenceReadState); ok && sourceID != "" {
		resource, _, found := resolver.GetByReference(sourceID)
		visited := map[string]bool{}
		for found && resource != nil && !visited[resource.ID] {
			visited[resource.ID] = true
			if resource.Type == ResourceTypeVM || resource.Type == ResourceTypeSystemContainer {
				guest = resource
				break
			}
			if resource.Agent != nil {
				loc.AgentID = resource.Agent.AgentID
			}
			if resource.ParentID == nil {
				break
			}
			resource, _, found = resolver.GetByReference(*resource.ParentID)
			if !found {
				loc.TargetHost = ""
				return
			}
		}
	}
	if guest == nil {
		add := func(resource *Resource) bool {
			if resource == nil || (resource.Name != hostname && resource.Name != sourceID) {
				return true
			}
			if resolver, ok := rs.(ReferenceReadState); ok {
				owned, _, found := resolver.GetByReference(resource.ID)
				if !found || owned == nil {
					return true
				}
				resource = owned
			}
			if guest != nil && guest.ID != resource.ID {
				return false
			}
			guest = resource
			return true
		}
		for _, view := range rs.Containers() {
			if view != nil && !add(view.r) {
				loc.TargetHost = ""
				return
			}
		}
		for _, view := range rs.VMs() {
			if view != nil && !add(view.r) {
				loc.TargetHost = ""
				return
			}
		}
	}
	if guest == nil {
		loc.DockerHostType = "standalone"
		return
	}
	owned := resourceContext(rs, guest).Location
	loc.DockerHostType, loc.DockerHostVMID, loc.Node = owned.ResourceType, owned.VMID, owned.Node
	loc.NodeResourceID, loc.AgentID = owned.NodeResourceID, owned.AgentID
	if rewriteTarget {
		loc.TargetHost = owned.TargetHost
	}
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
