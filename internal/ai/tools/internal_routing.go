package tools

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func (e *PulseToolExecutor) resolveResourceLocation(name string) models.ResourceLocation {
	rs, err := e.readStateForControl()
	if err != nil {
		return models.ResourceLocation{Found: false, Name: name}
	}

	for _, node := range rs.Nodes() {
		nodeName := node.Name()
		if nodeName == "" {
			nodeName = node.NodeName()
		}
		if nodeName == name || node.ID() == name {
			return models.ResourceLocation{
				Found:        true,
				Name:         name,
				ResourceType: "node",
				Node:         nodeName,
				TargetHost:   nodeName,
			}
		}
	}

	for _, vm := range rs.VMs() {
		if vm.Name() == name || vm.ID() == name {
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

	for _, lxc := range rs.Containers() {
		if lxc.Name() == name || lxc.ID() == name {
			return models.ResourceLocation{
				Found:        true,
				Name:         name,
				ResourceType: "lxc",
				VMID:         lxc.VMID(),
				Node:         lxc.Node(),
				TargetHost:   lxc.Name(),
			}
		}
	}

	for _, dh := range rs.DockerHosts() {
		dhName := dh.Hostname()
		if dhName == name || dh.ID() == name {
			loc := models.ResourceLocation{
				Found:          true,
				Name:           dhName,
				ResourceType:   "dockerhost",
				DockerHostName: dhName,
				TargetHost:     dhName,
			}

			// Resolve Docker host parent if lxc
			for _, lxc := range rs.Containers() {
				if lxc.Name() == dhName || lxc.ID() == dh.ID() {
					loc.DockerHostType = "lxc"
					loc.DockerHostVMID = lxc.VMID()
					loc.Node = lxc.Node()
					break
				}
			}
			if loc.DockerHostType == "" {
				for _, vm := range rs.VMs() {
					if vm.Name() == dhName || vm.ID() == dh.ID() {
						loc.DockerHostType = "vm"
						loc.DockerHostVMID = vm.VMID()
						loc.Node = vm.Node()
						break
					}
				}
			}
			if loc.DockerHostType == "" {
				loc.DockerHostType = "standalone"
			}
			return loc
		}
	}

	for _, dc := range rs.DockerContainers() {
		if dc.Name() == name || dc.ID() == name {
			dhID := dc.ParentID()
			var dhName string
			for _, dh := range rs.DockerHosts() {
				if dh.ID() == dhID {
					dhName = dh.Hostname()
					if dhName == "" {
						dhName = dh.Name()
					}
					break
				}
			}
			loc := models.ResourceLocation{
				Found:          true,
				Name:           name,
				ResourceType:   "docker",
				DockerHostName: dhName,
				TargetHost:     dhName,
			}

			// Resolve docker host parent
			for _, lxc := range rs.Containers() {
				if lxc.Name() == dhName || lxc.ID() == dhID || lxc.ID() == dhName {
					loc.DockerHostType = "lxc"
					loc.DockerHostVMID = lxc.VMID()
					loc.Node = lxc.Node()
					loc.TargetHost = lxc.Name()
					break
				}
			}
			if loc.DockerHostType == "" {
				for _, vm := range rs.VMs() {
					if vm.Name() == dhName || vm.ID() == dhID || vm.ID() == dhName {
						loc.DockerHostType = "vm"
						loc.DockerHostVMID = vm.VMID()
						loc.Node = vm.Node()
						loc.TargetHost = vm.Name()
						break
					}
				}
			}
			if loc.DockerHostType == "" {
				loc.DockerHostType = "standalone"
			}
			return loc
		}
	}

	for _, host := range rs.Hosts() {
		if host.Hostname() == name || host.ID() == name {
			return models.ResourceLocation{
				Found:        true,
				Name:         host.Hostname(),
				ResourceType: "host",
				HostID:       host.ID(),
				Platform:     host.Platform(),
				TargetHost:   host.Hostname(),
			}
		}
	}

	for _, cluster := range rs.K8sClusters() {
		if cluster.Name() == name || cluster.ID() == name {
			return models.ResourceLocation{
				Found:          true,
				Name:           cluster.Name(),
				ResourceType:   "k8s_cluster",
				K8sClusterName: cluster.Name(),
				K8sAgentID:     cluster.AgentID(),
				TargetHost:     cluster.Name(),
				AgentID:        cluster.AgentID(),
			}
		}
	}

	for _, pod := range rs.Pods() {
		if pod.Name() == name || pod.ID() == name {
			clusterName := pod.ClusterName()

			agentID := ""
			for _, c := range rs.K8sClusters() {
				if c.Name() == clusterName {
					agentID = c.AgentID()
					break
				}
			}

			return models.ResourceLocation{
				Found:          true,
				Name:           pod.Name(),
				ResourceType:   "k8s_pod",
				K8sClusterName: clusterName,
				K8sNamespace:   pod.Namespace(),
				K8sAgentID:     agentID,
				TargetHost:     clusterName,
				AgentID:        agentID,
			}
		}
	}

	for _, deploy := range rs.K8sDeployments() {
		if deploy.Name() == name || deploy.ID() == name {
			clusterName := deploy.ClusterName()

			agentID := ""
			for _, c := range rs.K8sClusters() {
				if c.Name() == clusterName {
					agentID = c.AgentID()
					break
				}
			}

			return models.ResourceLocation{
				Found:          true,
				Name:           deploy.Name(),
				ResourceType:   "k8s_deployment",
				K8sClusterName: clusterName,
				K8sNamespace:   deploy.Namespace(),
				K8sAgentID:     agentID,
				TargetHost:     clusterName,
				AgentID:        agentID,
			}
		}
	}

	return models.ResourceLocation{Found: false, Name: name}
}
