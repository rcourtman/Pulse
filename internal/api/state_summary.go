package api

import (
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

const stateSummaryVersion = 1

type StateSummaryResponse struct {
	Version          int                          `json:"version"`
	LastUpdate       int64                        `json:"lastUpdate"`
	Coverage         StateSummaryCoverage         `json:"coverage"`
	Health           StateSummaryHealth           `json:"health"`
	Alerts           StateSummaryAlerts           `json:"alerts"`
	ConnectionHealth StateSummaryConnectionHealth `json:"connectionHealth"`
}

type StateSummaryCoverage struct {
	Proxmox    StateSummaryProxmoxCoverage    `json:"proxmox"`
	Docker     StateSummaryDockerCoverage     `json:"docker"`
	Kubernetes StateSummaryKubernetesCoverage `json:"kubernetes"`
	HostAgents StateSummaryHostAgentCoverage  `json:"hostAgents"`
	Backup     StateSummaryBackupCoverage     `json:"backup"`
}

type StateSummaryProxmoxCoverage struct {
	Nodes         int `json:"nodes"`
	VMs           int `json:"vms"`
	Containers    int `json:"containers"`
	Storage       int `json:"storage"`
	CephClusters  int `json:"cephClusters"`
	PhysicalDisks int `json:"physicalDisks"`
}

type StateSummaryDockerCoverage struct {
	Hosts      int `json:"hosts"`
	Containers int `json:"containers"`
	Services   int `json:"services"`
	Tasks      int `json:"tasks"`
}

type StateSummaryKubernetesCoverage struct {
	Clusters    int `json:"clusters"`
	Nodes       int `json:"nodes"`
	Pods        int `json:"pods"`
	Deployments int `json:"deployments"`
}

type StateSummaryHostAgentCoverage struct {
	Hosts      int `json:"hosts"`
	Sensors    int `json:"sensors"`
	RAIDArrays int `json:"raidArrays"`
	SMARTDisks int `json:"smartDisks"`
}

type StateSummaryBackupCoverage struct {
	PBSInstances      int `json:"pbsInstances"`
	PBSBackups        int `json:"pbsBackups"`
	PMGInstances      int `json:"pmgInstances"`
	PMGBackups        int `json:"pmgBackups"`
	PVEBackupTasks    int `json:"pveBackupTasks"`
	PVEStorageBackups int `json:"pveStorageBackups"`
	PVEGuestSnapshots int `json:"pveGuestSnapshots"`
	ReplicationJobs   int `json:"replicationJobs"`
}

type StateSummaryHealth struct {
	Total    int `json:"total"`
	Up       int `json:"up"`
	Degraded int `json:"degraded"`
	Down     int `json:"down"`
	Unknown  int `json:"unknown"`
}

type StateSummaryAlerts struct {
	Active           int `json:"active"`
	Critical         int `json:"critical"`
	Warning          int `json:"warning"`
	Info             int `json:"info"`
	Acknowledged     int `json:"acknowledged"`
	RecentlyResolved int `json:"recentlyResolved"`
}

type StateSummaryConnectionHealth struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Unknown   int `json:"unknown"`
}

func (r *Router) handleStateSummary(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed",
			"Only GET method is allowed", nil)
		return
	}

	monitor := r.getTenantMonitor(req.Context())
	if monitor == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "no_monitor",
			"Monitor not available", nil)
		return
	}

	summary := buildStateSummary(monitor.GetState())
	if err := utils.WriteJSONResponse(w, summary); err != nil {
		log.Error().Err(err).Msg("Failed to encode state summary response")
		writeErrorResponse(w, http.StatusInternalServerError, "encoding_error",
			"Failed to encode state summary", nil)
	}
}

func buildStateSummary(state models.StateSnapshot) StateSummaryResponse {
	summary := StateSummaryResponse{
		Version:    stateSummaryVersion,
		LastUpdate: state.LastUpdate.Unix(),
	}

	summary.Coverage.Proxmox = StateSummaryProxmoxCoverage{
		Nodes:         len(state.Nodes),
		VMs:           len(state.VMs),
		Containers:    len(state.Containers),
		Storage:       len(state.Storage),
		CephClusters:  len(state.CephClusters),
		PhysicalDisks: len(state.PhysicalDisks),
	}
	summary.Coverage.Kubernetes.Clusters = len(state.KubernetesClusters)
	summary.Coverage.HostAgents.Hosts = len(state.Hosts)
	summary.Coverage.Backup = StateSummaryBackupCoverage{
		PBSInstances:      len(state.PBSInstances),
		PBSBackups:        len(state.PBSBackups),
		PMGInstances:      len(state.PMGInstances),
		PMGBackups:        len(state.PMGBackups),
		PVEBackupTasks:    len(state.PVEBackups.BackupTasks),
		PVEStorageBackups: len(state.PVEBackups.StorageBackups),
		PVEGuestSnapshots: len(state.PVEBackups.GuestSnapshots),
		ReplicationJobs:   len(state.ReplicationJobs),
	}

	for _, node := range state.Nodes {
		addStateSummaryStatus(&summary.Health, node.Status)
	}
	for _, vm := range state.VMs {
		addStateSummaryStatus(&summary.Health, vm.Status)
	}
	for _, container := range state.Containers {
		addStateSummaryStatus(&summary.Health, container.Status)
	}
	for _, storage := range state.Storage {
		addStateSummaryStatus(&summary.Health, storageHealthStatus(storage))
	}
	for _, ceph := range state.CephClusters {
		addStateSummaryStatus(&summary.Health, ceph.Health)
	}
	for _, disk := range state.PhysicalDisks {
		addStateSummaryStatus(&summary.Health, disk.Health)
	}
	for _, pbs := range state.PBSInstances {
		addStateSummaryStatus(&summary.Health, firstNonEmpty(pbs.ConnectionHealth, pbs.Status))
		for _, datastore := range pbs.Datastores {
			addStateSummaryStatus(&summary.Health, datastore.Status)
		}
	}
	for _, pmg := range state.PMGInstances {
		addStateSummaryStatus(&summary.Health, firstNonEmpty(pmg.ConnectionHealth, pmg.Status))
	}

	for _, host := range state.DockerHosts {
		summary.Coverage.Docker.Hosts++
		summary.Coverage.Docker.Containers += len(host.Containers)
		summary.Coverage.Docker.Services += len(host.Services)
		summary.Coverage.Docker.Tasks += len(host.Tasks)

		addStateSummaryStatus(&summary.Health, host.Status)
		for _, container := range host.Containers {
			addStateSummaryStatus(&summary.Health, firstNonEmpty(container.Health, container.State, container.Status))
		}
		for _, service := range host.Services {
			addStateSummaryServiceHealth(&summary.Health, service.RunningTasks, service.DesiredTasks)
		}
	}

	for _, cluster := range state.KubernetesClusters {
		summary.Coverage.Kubernetes.Nodes += len(cluster.Nodes)
		summary.Coverage.Kubernetes.Pods += len(cluster.Pods)
		summary.Coverage.Kubernetes.Deployments += len(cluster.Deployments)

		addStateSummaryStatus(&summary.Health, cluster.Status)
		for _, node := range cluster.Nodes {
			addStateSummaryBool(&summary.Health, node.Ready)
		}
		for _, pod := range cluster.Pods {
			addStateSummaryStatus(&summary.Health, pod.Phase)
		}
		for _, deployment := range cluster.Deployments {
			addStateSummaryServiceHealth(&summary.Health, int(deployment.AvailableReplicas), int(deployment.DesiredReplicas))
		}
	}

	for _, host := range state.Hosts {
		addStateSummaryStatus(&summary.Health, host.Status)
		summary.Coverage.HostAgents.Sensors += len(host.Sensors.TemperatureCelsius) + len(host.Sensors.FanRPM) + len(host.Sensors.Additional)
		summary.Coverage.HostAgents.SMARTDisks += len(host.Sensors.SMART)
		summary.Coverage.HostAgents.RAIDArrays += len(host.RAID)
		for _, smart := range host.Sensors.SMART {
			addStateSummaryStatus(&summary.Health, smart.Health)
		}
		for _, raid := range host.RAID {
			addStateSummaryStatus(&summary.Health, raid.State)
		}
	}

	for _, alert := range state.ActiveAlerts {
		summary.Alerts.Active++
		if alert.Acknowledged {
			summary.Alerts.Acknowledged++
		}
		switch strings.ToLower(strings.TrimSpace(alert.Level)) {
		case "critical", "error", "fatal":
			summary.Alerts.Critical++
		case "warning", "warn":
			summary.Alerts.Warning++
		default:
			summary.Alerts.Info++
		}
	}
	summary.Alerts.RecentlyResolved = len(state.RecentlyResolved)

	for _, healthy := range state.ConnectionHealth {
		summary.ConnectionHealth.Total++
		if healthy {
			summary.ConnectionHealth.Healthy++
		} else {
			summary.ConnectionHealth.Unhealthy++
		}
	}

	return summary
}

func storageHealthStatus(storage models.Storage) string {
	if storage.ZFSPool != nil {
		return firstNonEmpty(storage.ZFSPool.State, storage.ZFSPool.Status, storage.Status)
	}
	if !storage.Enabled || !storage.Active {
		if stateSummaryHealthBucket(storage.Status) == "down" {
			return storage.Status
		}
		return "degraded"
	}
	return storage.Status
}

func addStateSummaryBool(health *StateSummaryHealth, healthy bool) {
	if healthy {
		addStateSummaryBucket(health, "up")
		return
	}
	addStateSummaryBucket(health, "down")
}

func addStateSummaryServiceHealth(health *StateSummaryHealth, running, desired int) {
	switch {
	case desired <= 0:
		addStateSummaryBucket(health, "up")
	case running >= desired:
		addStateSummaryBucket(health, "up")
	case running > 0:
		addStateSummaryBucket(health, "degraded")
	default:
		addStateSummaryBucket(health, "down")
	}
}

func addStateSummaryStatus(health *StateSummaryHealth, status string) {
	addStateSummaryBucket(health, stateSummaryHealthBucket(status))
}

func addStateSummaryBucket(health *StateSummaryHealth, bucket string) {
	if health == nil {
		return
	}
	health.Total++
	switch bucket {
	case "up":
		health.Up++
	case "degraded":
		health.Degraded++
	case "down":
		health.Down++
	default:
		health.Unknown++
	}
}

func stateSummaryHealthBucket(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "" {
		return "unknown"
	}
	normalized = strings.ReplaceAll(normalized, "_", "-")
	normalized = strings.ReplaceAll(normalized, " ", "-")

	switch normalized {
	case "active", "available", "bound", "connected", "healthy", "health-ok", "ok", "online", "passed", "ready", "running", "succeeded", "up":
		return "up"
	case "degraded", "health-warn", "maintenance", "not-ready", "pending", "restarting", "starting", "stopping", "unhealthy", "warn", "warning":
		return "degraded"
	case "critical", "crashloopbackoff", "disconnected", "down", "error", "failed", "failing", "faulted", "health-err", "offline", "removed", "stopped", "unavail", "unavailable":
		return "down"
	case "unknown":
		return "unknown"
	default:
		return "unknown"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
