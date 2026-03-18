package unifiedresources

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// ResolvedResourceContext combines canonical routing coordinates with the
// matching unified resource record so downstream consumers can apply canonical
// policy metadata without rescanning unrelated resource state.
type ResolvedResourceContext struct {
	Location models.ResourceLocation
	Resource *Resource
}

// ResolveResourceContext performs canonical name-based resolution and returns
// both the resolved location and the matching unified resource record.
func ResolveResourceContext(rs ReadState, name string) ResolvedResourceContext {
	loc := ResolveResource(rs, name)
	if !loc.Found {
		return ResolvedResourceContext{Location: loc}
	}

	return ResolvedResourceContext{
		Location: loc,
		Resource: lookupResolvedResource(rs, loc),
	}
}

func lookupResolvedResource(rs ReadState, loc models.ResourceLocation) *Resource {
	if rs == nil || !loc.Found {
		return nil
	}

	name := strings.TrimSpace(loc.Name)
	switch loc.ResourceType {
	case "node":
		for _, node := range rs.Nodes() {
			if node == nil {
				continue
			}
			if node.NodeName() == name {
				return cloneResourcePtr(node.r)
			}
		}

	case "vm":
		for _, vm := range rs.VMs() {
			if vm == nil {
				continue
			}
			if vm.Name() == name {
				return cloneResourcePtr(vm.r)
			}
		}

	case "system-container":
		for _, container := range rs.Containers() {
			if container == nil {
				continue
			}
			if container.Name() == name {
				return cloneResourcePtr(container.r)
			}
		}

	case "docker-host":
		for _, dockerHost := range rs.DockerHosts() {
			if dockerHost == nil {
				continue
			}
			if dockerHost.Hostname() == name || dockerHost.HostSourceID() == name {
				return cloneResourcePtr(dockerHost.r)
			}
		}

	case "app-container":
		for _, container := range rs.DockerContainers() {
			if container == nil {
				continue
			}
			if container.Name() == name {
				return cloneResourcePtr(container.r)
			}
		}

	case "agent":
		for _, host := range rs.Hosts() {
			if host == nil {
				continue
			}
			if host.Hostname() == name || host.AgentID() == name {
				return cloneResourcePtr(host.r)
			}
		}

	case "k8s-cluster":
		for _, cluster := range rs.K8sClusters() {
			if cluster == nil {
				continue
			}
			sourceName := strings.TrimSpace(cluster.SourceName())
			clusterName := strings.TrimSpace(cluster.Name())
			clusterID := strings.TrimSpace(cluster.ClusterID())
			if sourceName == name || clusterName == name || clusterID == name {
				return cloneResourcePtr(cluster.r)
			}
		}

	case "k8s-pod":
		for _, pod := range rs.Pods() {
			if pod == nil {
				continue
			}
			if pod.Name() == name {
				return cloneResourcePtr(pod.r)
			}
		}

	case "k8s-deployment":
		for _, deployment := range rs.K8sDeployments() {
			if deployment == nil {
				continue
			}
			if deployment.Name() == name {
				return cloneResourcePtr(deployment.r)
			}
		}
	}

	return nil
}
