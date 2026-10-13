package unifiedresources

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// ResolvedResourceContext keeps identity, routing and policy on one owned record.
type ResolvedResourceContext struct {
	Location models.ResourceLocation
	Resource *Resource
}

// ResolveResourceReferenceContext resolves an explicit selection without using
// its display label as a fallback. Unknown or ambiguous references fail closed.
func ResolveResourceReferenceContext(rs ReadState, reference string) ResolvedResourceContext {
	reference = strings.TrimSpace(reference)
	resolver, ok := rs.(ReferenceReadState)
	if !ok || reference == "" {
		return ResolvedResourceContext{}
	}
	resource, _, found := resolver.GetByReference(reference)
	if !found {
		resource = legacySelectedResource(rs, reference)
	}
	if resource == nil {
		return ResolvedResourceContext{}
	}
	return resourceContext(rs, resource)
}

// ResolveResourceContext retains the name-based read boundary for callers that
// have no explicit selection. A duplicated label cannot identify a resource.
func ResolveResourceContext(rs ReadState, name string) ResolvedResourceContext {
	name = strings.TrimSpace(name)
	if rs == nil || name == "" {
		return ResolvedResourceContext{Location: models.ResourceLocation{Name: name}}
	}
	if resolver, ok := rs.(ReferenceReadState); ok {
		resource, _, found := resolver.GetByReference(name)
		if found && resource != nil {
			return resourceContext(rs, resource)
		}
	}
	resource, ambiguous := resourceByName(rs, name)
	if resource == nil {
		return ResolvedResourceContext{Location: models.ResourceLocation{Name: name, Ambiguous: ambiguous}}
	}
	return resourceContext(rs, resource)
}

func resourceContext(rs ReadState, resource *Resource) ResolvedResourceContext {
	resource = cloneResourcePtr(resource)
	loc := models.ResourceLocation{Found: true, Name: resource.Name, ResourceType: string(resource.Type)}
	switch resource.Type {
	case ResourceTypeVM:
		view := NewVMView(resource)
		loc.VMID, loc.Node, loc.TargetHost = view.VMID(), view.Node(), view.Name()
		setParentNodeLocation(rs, resource, &loc)
	case ResourceTypeSystemContainer:
		view := NewContainerView(resource)
		loc.VMID, loc.Node, loc.TargetHost = view.VMID(), view.Node(), view.Name()
		setParentNodeLocation(rs, resource, &loc)
	case ResourceTypeAppContainer:
		populateAppContainerLocation(rs, *resource, &loc)
	case ResourceTypeStorage:
		view := NewStoragePoolView(resource)
		loc.TargetID = storagePoolTarget(&view)
		loc.TargetHost = loc.TargetID
	case ResourceTypeAgent:
		switch {
		case resource.Proxmox != nil && NewNodeView(resource).NodeName() != "":
			view := NewNodeView(resource)
			loc.ResourceType = "node"
			loc.Name, loc.Node, loc.TargetHost = view.NodeName(), view.NodeName(), view.NodeName()
			loc.NodeResourceID = resource.ID
			if resource.Agent != nil {
				loc.AgentID = resource.Agent.AgentID
			}
		case resource.Docker != nil && NewDockerHostView(resource).Hostname() != "":
			view := NewDockerHostView(resource)
			loc.ResourceType = "docker-host"
			loc.Name, loc.DockerHostName, loc.TargetHost = view.Hostname(), view.Hostname(), view.Hostname()
			resolveDockerHostParent(rs, view.Hostname(), view.HostSourceID(), false, &loc)
		default:
			view := NewHostView(resource)
			loc.Name, loc.TargetID, loc.Platform, loc.TargetHost = view.Hostname(), view.AgentID(), view.Platform(), view.Hostname()
			loc.AgentID = view.AgentID()
		}
	case ResourceTypeK8sCluster:
		view := NewK8sClusterView(resource)
		loc.Name = clusterSourceOrDisplayName(&view)
		loc.K8sClusterName, loc.TargetHost = loc.Name, loc.Name
		loc.K8sAgentID, loc.AgentID = view.AgentID(), view.AgentID()
	case ResourceTypePod, ResourceTypeK8sDeployment:
		if resource.Type == ResourceTypePod {
			view := NewPodView(resource)
			loc.ResourceType, loc.K8sNamespace = "k8s-pod", view.Namespace()
		} else {
			view := NewK8sDeploymentView(resource)
			loc.K8sNamespace = view.Namespace()
		}
		if resource.ParentID != nil {
			for _, cluster := range rs.K8sClusters() {
				if cluster != nil && cluster.ID() == *resource.ParentID {
					loc.K8sClusterName = clusterSourceOrDisplayName(cluster)
					loc.TargetHost = loc.K8sClusterName
					loc.K8sAgentID, loc.AgentID = cluster.AgentID(), cluster.AgentID()
					break
				}
			}
		}
	}
	return ResolvedResourceContext{Location: loc, Resource: resource}
}

func resourceByName(rs ReadState, name string) (*Resource, bool) {
	var result *Resource
	ambiguous := false
	add := func(resource *Resource, matches bool) {
		if !matches || resource == nil {
			return
		}
		if resolver, ok := rs.(ReferenceReadState); ok {
			owned, _, found := resolver.GetByReference(resource.ID)
			if !found || owned == nil {
				return
			}
			resource = owned
		}
		if result != nil && result.ID != resource.ID {
			ambiguous = true
		}
		result = resource
	}
	for _, view := range rs.Nodes() {
		if view != nil {
			add(view.r, view.NodeName() == name || view.ID() == name)
		}
	}
	for _, view := range rs.VMs() {
		if view != nil {
			add(view.r, view.Name() == name || view.ID() == name)
		}
	}
	for _, view := range rs.Containers() {
		if view != nil {
			add(view.r, view.Name() == name)
		}
	}
	for _, view := range rs.DockerHosts() {
		if view != nil {
			add(view.r, HostnamesEquivalent(view.Hostname(), name) || view.HostSourceID() == name || view.ID() == name)
		}
	}
	for _, resource := range appContainerResourcesFromReadState(rs) {
		add(cloneResourcePtr(&resource), appContainerResourceMatches(resource, name))
	}
	for _, view := range rs.StoragePools() {
		if view != nil {
			add(view.r, storagePoolMatches(view, name))
		}
	}
	for _, view := range rs.Hosts() {
		if view != nil {
			add(view.r, HostnamesEquivalent(view.Hostname(), name) || view.AgentID() == name || view.ID() == name)
		}
	}
	for _, view := range rs.K8sClusters() {
		if view != nil {
			add(view.r, view.SourceName() == name || view.Name() == name || view.ClusterID() == name)
		}
	}
	for _, view := range rs.Pods() {
		if view != nil {
			add(view.r, view.Name() == name)
		}
	}
	for _, view := range rs.K8sDeployments() {
		if view != nil {
			add(view.r, view.Name() == name)
		}
	}
	if ambiguous {
		return nil, true
	}
	return result, false
}

// legacySelectedResource bounds the pre-canonical mention wire formats to a
// unique current owner. The label and caller-supplied node never select a row.
func legacySelectedResource(rs ReadState, reference string) *Resource {
	var result *Resource
	ambiguous := false
	add := func(resource *Resource, native string) {
		if resource == nil || native != reference {
			return
		}
		if resolver, ok := rs.(ReferenceReadState); ok {
			owned, _, found := resolver.GetByReference(resource.ID)
			if !found || owned == nil {
				return
			}
			resource = owned
		}
		if result != nil && result.ID != resource.ID {
			ambiguous = true
		}
		result = resource
	}
	for _, view := range rs.VMs() {
		if view != nil && view.VMID() > 0 {
			add(view.r, fmt.Sprintf("vm:%s:%d", view.Node(), view.VMID()))
		}
	}
	for _, view := range rs.Containers() {
		if view != nil && view.VMID() > 0 {
			add(view.r, fmt.Sprintf("system-container:%s:%d", view.Node(), view.VMID()))
		}
	}
	for _, resource := range appContainerResourcesFromReadState(rs) {
		host := appContainerResourceHost(resource)
		uid := ""
		if resource.Docker != nil {
			uid = strings.TrimSpace(resource.Docker.ContainerID)
		}
		if uid == "" && resource.Canonical != nil {
			uid = strings.TrimSpace(resource.Canonical.PrimaryID)
		}
		if host != "" && uid != "" {
			add(&resource, "app-container:"+host+":"+uid)
		}
		if resource.Docker != nil && uid != "" && resource.Docker.HostSourceID != "" {
			add(&resource, "docker:"+resource.Docker.HostSourceID+":"+uid)
		}
	}
	for _, view := range rs.Hosts() {
		if view != nil && view.AgentID() != "" {
			add(view.r, "agent:"+view.AgentID())
		}
	}
	for _, view := range rs.DockerHosts() {
		if view != nil && view.HostSourceID() != "" {
			add(view.r, "agent:"+view.HostSourceID())
		}
	}
	for _, view := range rs.Nodes() {
		if view != nil && view.r.Proxmox != nil {
			for _, instance := range []string{view.r.Proxmox.Instance, view.r.Proxmox.SourceID, ""} {
				add(view.r, "node:"+instance+":"+view.NodeName())
			}
		}
	}
	if ambiguous {
		return nil
	}
	return cloneResourcePtr(result)
}

func setParentNodeLocation(rs ReadState, resource *Resource, loc *models.ResourceLocation) {
	if resource.Proxmox == nil {
		return
	}
	var node *Resource
	if resource.ParentID != nil {
		if resolver, ok := rs.(ReferenceReadState); ok {
			parent, _, found := resolver.GetByReference(*resource.ParentID)
			if found && parent != nil && parent.Type == ResourceTypeAgent && parent.Proxmox != nil {
				node = parent
			}
		}
	}
	if node == nil {
		for _, view := range rs.Nodes() {
			if view == nil || view.r.Proxmox == nil || view.NodeName() != resource.Proxmox.NodeName {
				continue
			}
			if resource.Proxmox.Instance != "" && view.r.Proxmox.Instance != resource.Proxmox.Instance {
				continue
			}
			if node != nil && node.ID != view.ID() {
				return
			}
			node = view.r
		}
	}
	if node != nil {
		loc.NodeResourceID = node.ID
		if node.Agent != nil {
			loc.AgentID = node.Agent.AgentID
		}
	}
}
