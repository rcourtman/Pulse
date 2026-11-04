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
// as the docker hostname.
//
// loc.AgentID arrives holding the agent that reports this Docker resource, since
// a hostname cannot say which machine it means. An agent record found on the way
// up replaces it. A backing guest replaces it with the agent on its node and
// keeps it as GuestAgentID.
//
// A guest backs this host only when something beyond a matching name says so: an
// operator's link folded them together, Pulse found the host by probing that
// guest, or the two share an address with no link saying otherwise. Anything else
// can be another machine with the same name, so the host stays standalone.
func resolveDockerHostParent(rs ReadState, hostname, sourceID string, rewriteTarget bool, loc *models.ResourceLocation) {
	var guest, reporter *Resource
	if resolver, ok := rs.(ReferenceReadState); ok && sourceID != "" {
		resource, _, found := resolver.GetByReference(sourceID)
		if found && resource != nil && resource.Docker != nil && resource.Type == ResourceTypeAgent {
			reporter = resource
		}
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
	probedGuest, probed := guestProbedForDockerHost(rs, reporter)
	if probed {
		guest = probedGuest
	}
	// A reference that is only its hostname can find a guest by name too.
	inferred := guest == nil || sourceID == hostname
	if guest == nil && !probed {
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
	if inferred && (!dockerHostSharesGuestAddress(reporter, guest) || agentLinkedElsewhere(rs, reporter, guest)) {
		loc.DockerHostType = "standalone"
		return
	}
	owned := resourceContext(rs, guest).Location
	// A name can place this Docker host in a guest, but cannot make an agent in
	// that guest its own.
	loc.GuestAgentID, loc.DockerHostInferred = loc.AgentID, inferred
	if !inferred && owned.GuestAgentID != "" {
		loc.GuestAgentID = owned.GuestAgentID
	}
	loc.DockerHostType, loc.DockerHostVMID, loc.Node = owned.ResourceType, owned.VMID, owned.Node
	loc.NodeResourceID, loc.AgentID = owned.NodeResourceID, owned.AgentID
	if rewriteTarget {
		loc.TargetHost = owned.TargetHost
	}
}

// guestProbedForDockerHost finds the guest Pulse probed through its node to discover
// a Docker host, so no name is consulted. The agent ID names that guest. It comes
// from the same report as the host's inventory, while the host's own ID can name
// another guest: a probe report matches an existing row by hostname. The second
// result says the ID is one of these, including when its guest has gone.
func guestProbedForDockerHost(rs ReadState, host *Resource) (*Resource, bool) {
	if host == nil || host.Docker == nil {
		return nil, false
	}
	guestSourceID, probed := strings.CutPrefix(strings.TrimSpace(host.Docker.AgentID), proxmoxLXCDockerHostSourcePrefix)
	if !probed {
		return nil, false
	}
	// References follow a guest across a migration and through a merge of two
	// connections. Its VMID can pass to a VM once it has gone.
	if resolver, ok := rs.(ReferenceReadState); ok {
		if guest, _, found := resolver.GetByReference(guestSourceID); found && guest != nil && guest.Type == ResourceTypeSystemContainer {
			return guest, true
		}
	}
	return nil, true
}

// dockerHostSharesGuestAddress reports whether a Docker host's machine lists an
// address its same-named guest also has. Sites can reuse a private range, so this
// corroborates the name and does not stand alone, and addresses every machine has
// to itself do not count. A guest's known addresses can be stale, so a miss only
// means nothing shows they are the same machine.
func dockerHostSharesGuestAddress(host, guest *Resource) bool {
	if host == nil || guest == nil {
		return false
	}
	// A guest's flat address list has no interface names to filter, so it counts only
	// when the guest has no interface list.
	guestNetwork := []NetworkInterface{{Addresses: guest.Identity.IPAddresses}}
	if guest.Proxmox != nil && len(guest.Proxmox.NetworkInterfaces) > 0 {
		guestNetwork = guest.Proxmox.NetworkInterfaces
	}
	addresses := networkLinkAddresses(guestNetwork)
	sharesAny := func(network []NetworkInterface) bool {
		for address := range networkLinkAddresses(network) {
			if _, shared := addresses[address]; shared {
				return true
			}
		}
		return false
	}
	if host.Docker != nil && sharesAny(host.Docker.NetworkInterfaces) {
		return true
	}
	return host.Agent != nil && sharesAny(host.Agent.NetworkInterfaces)
}

// agentLinkedElsewhere reports whether a Docker host's agent record is linked to a
// node or to another guest. A link to this guest adds nothing, since it can come from
// the name, but one elsewhere contradicts placing the host here.
func agentLinkedElsewhere(rs ReadState, host, guest *Resource) bool {
	resolver, ok := rs.(ReferenceReadState)
	if !ok || host == nil || host.Agent == nil {
		return false
	}
	if strings.TrimSpace(host.Agent.LinkedNodeID) != "" {
		return true
	}
	for _, linked := range []string{host.Agent.LinkedVMID, host.Agent.LinkedContainerID} {
		if linked = strings.TrimSpace(linked); linked == "" {
			continue
		}
		// References follow a guest across a migration and through a merge of two
		// connections, and an unreadable link cannot show it means this guest.
		if resolved, _, found := resolver.GetByReference(linked); !found || resolved == nil || resolved.ID != guest.ID {
			return true
		}
	}
	return false
}

func networkLinkAddresses(network []NetworkInterface) map[string]struct{} {
	addresses := make(map[string]struct{})
	for _, nic := range network {
		if models.IsHostLocalLinkInterface(nic.Name) {
			continue
		}
		for _, address := range nic.Addresses {
			if normalized := models.NodeLinkAddress(address); normalized != "" {
				addresses[normalized] = struct{}{}
			}
		}
	}
	return addresses
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
	if resource.Docker != nil {
		loc.AgentID = resource.Docker.AgentID
	}
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
