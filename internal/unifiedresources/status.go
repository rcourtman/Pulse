package unifiedresources

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func statusFromString(status string) ResourceStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online", "running", "ok":
		return StatusOnline
	case "offline", "down", "stopped":
		return StatusOffline
	case "warning", "degraded":
		return StatusWarning
	default:
		return StatusUnknown
	}
}

func statusFromGuest(status string) ResourceStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running", "online":
		return StatusOnline
	case "stopped", "offline", "paused":
		return StatusOffline
	case "warning", "degraded":
		return StatusWarning
	default:
		return StatusUnknown
	}
}

func statusFromStorage(storage models.Storage) ResourceStatus {
	switch strings.ToLower(strings.TrimSpace(storage.Status)) {
	case "online", "running", "available", "active", "ok":
		return StatusOnline
	case "warning", "degraded":
		return StatusWarning
	case "offline", "down", "unavailable", "error":
		return StatusOffline
	}
	if !storage.Active {
		return StatusOffline
	}
	if !storage.Enabled {
		return StatusWarning
	}
	return StatusOnline
}

func statusFromPhysicalDisk(health string) ResourceStatus {
	switch strings.ToUpper(strings.TrimSpace(health)) {
	case "PASSED", "OK":
		return StatusOnline
	case "FAILED":
		return StatusOffline
	default:
		return StatusUnknown
	}
}

func statusFromCephHealth(health string) ResourceStatus {
	switch strings.ToUpper(strings.TrimSpace(health)) {
	case "HEALTH_OK":
		return StatusOnline
	case "HEALTH_WARN":
		return StatusWarning
	case "HEALTH_ERR":
		return StatusOffline
	default:
		return StatusUnknown
	}
}

func statusFromDockerState(state string) ResourceStatus {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "running":
		return StatusOnline
	case "created", "exited", "dead", "paused":
		return StatusOffline
	case "restarting":
		return StatusWarning
	default:
		return StatusUnknown
	}
}

func statusFromPBSInstance(instance models.PBSInstance) ResourceStatus {
	primary := strings.ToLower(strings.TrimSpace(instance.Status))
	health := strings.ToLower(strings.TrimSpace(instance.ConnectionHealth))

	switch {
	case primary == "online" || primary == "running" || primary == "ok" || primary == "healthy":
		return StatusOnline
	case primary == "warning" || primary == "degraded":
		return StatusWarning
	case primary == "offline" || primary == "down" || primary == "stopped" || primary == "error":
		return StatusOffline
	}

	switch health {
	case "online", "running", "ok", "healthy", "connected":
		return StatusOnline
	case "warning", "degraded":
		return StatusWarning
	case "offline", "down", "stopped", "error", "disconnected":
		return StatusOffline
	default:
		return StatusUnknown
	}
}

func statusFromPMGInstance(instance models.PMGInstance) ResourceStatus {
	primary := strings.ToLower(strings.TrimSpace(instance.Status))
	health := strings.ToLower(strings.TrimSpace(instance.ConnectionHealth))

	switch {
	case primary == "online" || primary == "running" || primary == "ok" || primary == "healthy":
		return StatusOnline
	case primary == "warning" || primary == "degraded":
		return StatusWarning
	case primary == "offline" || primary == "down" || primary == "stopped" || primary == "error":
		return StatusOffline
	}

	switch health {
	case "online", "running", "ok", "healthy", "connected":
		return StatusOnline
	case "warning", "degraded":
		return StatusWarning
	case "offline", "down", "stopped", "error", "disconnected":
		return StatusOffline
	default:
		return StatusUnknown
	}
}

func statusFromKubernetesCluster(cluster models.KubernetesCluster) ResourceStatus {
	switch strings.ToLower(strings.TrimSpace(cluster.Status)) {
	case "online", "running", "ready", "healthy":
		return StatusOnline
	case "warning", "degraded":
		return StatusWarning
	case "offline", "disconnected", "error":
		return StatusOffline
	}

	if len(cluster.Nodes) == 0 {
		return StatusUnknown
	}
	ready := 0
	for _, node := range cluster.Nodes {
		if node.Ready {
			ready++
		}
	}
	switch {
	case ready == len(cluster.Nodes):
		return StatusOnline
	case ready == 0:
		return StatusOffline
	default:
		return StatusWarning
	}
}

func statusFromKubernetesNode(node models.KubernetesNode) ResourceStatus {
	if !node.Ready {
		return StatusOffline
	}
	if node.Unschedulable {
		return StatusWarning
	}
	return StatusOnline
}

func statusFromKubernetesPod(pod models.KubernetesPod) ResourceStatus {
	phase := strings.ToLower(strings.TrimSpace(pod.Phase))
	switch phase {
	case "failed", "succeeded":
		return StatusOffline
	case "running":
		if len(pod.Containers) == 0 {
			return StatusOnline
		}
		for _, container := range pod.Containers {
			if !container.Ready {
				return StatusWarning
			}
			state := strings.ToLower(strings.TrimSpace(container.State))
			if state != "" && state != "running" {
				return StatusWarning
			}
		}
		return StatusOnline
	case "pending", "unknown":
		return StatusWarning
	default:
		return StatusUnknown
	}
}

func statusFromKubernetesDeployment(deployment models.KubernetesDeployment) ResourceStatus {
	desired := deployment.DesiredReplicas
	available := deployment.AvailableReplicas
	if desired <= 0 {
		if available > 0 {
			return StatusOnline
		}
		return StatusUnknown
	}
	if available >= desired {
		return StatusOnline
	}
	if available == 0 {
		return StatusOffline
	}
	return StatusWarning
}
