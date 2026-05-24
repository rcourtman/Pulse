package monitoring

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	removedKubernetesClustersTTL = 24 * time.Hour
)

func normalizeKubernetesClusterIdentifier(report agentsk8s.Report) string {
	if v := strings.TrimSpace(report.Cluster.ID); v != "" {
		return v
	}
	if v := strings.TrimSpace(report.Agent.ID); v != "" {
		return v
	}

	stableKey := strings.TrimSpace(report.Cluster.Server) + "|" + strings.TrimSpace(report.Cluster.Context) + "|" + strings.TrimSpace(report.Cluster.Name)
	stableKey = strings.TrimSpace(stableKey)
	if stableKey == "||" || stableKey == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(stableKey))
	return hex.EncodeToString(sum[:])
}

// ApplyKubernetesReport ingests a Kubernetes agent report into state.
func (m *Monitor) ApplyKubernetesReport(report agentsk8s.Report, tokenRecord *config.APITokenRecord) (models.KubernetesCluster, error) {
	identifier := normalizeKubernetesClusterIdentifier(report)
	if strings.TrimSpace(identifier) == "" {
		return models.KubernetesCluster{}, fmt.Errorf("kubernetes report missing cluster identifier")
	}

	// Check if this cluster was deliberately removed - reject report to prevent resurrection
	m.mu.RLock()
	removedAt, wasRemoved := m.removedKubernetesClusters[identifier]
	m.mu.RUnlock()
	if wasRemoved {
		log.Info().
			Str("k8sClusterID", identifier).
			Time("removedAt", removedAt).
			Msg("Rejecting report from deliberately removed Kubernetes cluster")
		return models.KubernetesCluster{}, fmt.Errorf("kubernetes cluster %q had monitoring stopped at %v and cannot report again. Use Allow reconnect in Settings -> Infrastructure or rerun the installer with a kubernetes:manage token to clear this block", identifier, removedAt.Format(time.RFC3339))
	}

	// Enforce token uniqueness: each token can only be bound to one cluster agent
	if tokenRecord != nil && tokenRecord.ID != "" {
		tokenID := strings.TrimSpace(tokenRecord.ID)
		agentID := strings.TrimSpace(report.Agent.ID)
		if agentID == "" {
			agentID = identifier
		}

		m.mu.Lock()
		if boundAgentID, exists := m.kubernetesTokenBindings[tokenID]; exists {
			if boundAgentID != agentID {
				m.mu.Unlock()
				tokenHint := tokenHintFromRecord(tokenRecord)
				if tokenHint != "" {
					tokenHint = " (" + tokenHint + ")"
				}
				log.Warn().
					Str("tokenID", tokenID).
					Str("tokenHint", tokenHint).
					Str("reportingAgentID", agentID).
					Str("boundAgentID", boundAgentID).
					Msg("Rejecting Kubernetes report: token already bound to different agent")
				return models.KubernetesCluster{}, fmt.Errorf("API token%s is already in use by agent %q. Each Kubernetes agent must use a unique API token. Generate a new token for this agent", tokenHint, boundAgentID)
			}
		} else {
			m.kubernetesTokenBindings[tokenID] = agentID
			log.Debug().
				Str("tokenID", tokenID).
				Str("agentID", agentID).
				Str("clusterID", identifier).
				Msg("Bound Kubernetes agent token to agent identity")
		}
		m.mu.Unlock()
	}

	timestamp := report.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	agentID := strings.TrimSpace(report.Agent.ID)
	if agentID == "" {
		agentID = identifier
	}

	name := strings.TrimSpace(report.Cluster.Name)
	displayName := name
	if displayName == "" {
		displayName = identifier
	}

	nodes := make([]models.KubernetesNode, 0, len(report.Nodes))
	for _, n := range report.Nodes {
		var usageCPUMilli int64
		var usageMemoryBytes int64
		if n.Usage != nil {
			usageCPUMilli = n.Usage.CPUMilliCores
			usageMemoryBytes = n.Usage.MemoryBytes
		}
		roles := append([]string(nil), n.Roles...)
		nodes = append(nodes, models.KubernetesNode{
			UID:                     strings.TrimSpace(n.UID),
			Name:                    strings.TrimSpace(n.Name),
			Ready:                   n.Ready,
			Unschedulable:           n.Unschedulable,
			KubeletVersion:          strings.TrimSpace(n.KubeletVersion),
			ContainerRuntimeVersion: strings.TrimSpace(n.ContainerRuntimeVersion),
			OSImage:                 strings.TrimSpace(n.OSImage),
			KernelVersion:           strings.TrimSpace(n.KernelVersion),
			Architecture:            strings.TrimSpace(n.Architecture),
			CapacityCPU:             n.Capacity.CPUCores,
			CapacityMemoryBytes:     n.Capacity.MemoryBytes,
			CapacityPods:            n.Capacity.Pods,
			AllocCPU:                n.Allocatable.CPUCores,
			AllocMemoryBytes:        n.Allocatable.MemoryBytes,
			AllocPods:               n.Allocatable.Pods,
			UsageCPUMilliCores:      usageCPUMilli,
			UsageMemoryBytes:        usageMemoryBytes,
			UsageCPUPercent:         kubernetesCPUUsagePercent(usageCPUMilli, n.Allocatable.CPUCores, n.Capacity.CPUCores),
			UsageMemoryPercent:      kubernetesMemoryUsagePercent(usageMemoryBytes, n.Allocatable.MemoryBytes, n.Capacity.MemoryBytes),
			Roles:                   roles,
		})
	}

	nodeByName := make(map[string]models.KubernetesNode, len(nodes))
	for _, node := range nodes {
		nodeName := strings.TrimSpace(node.Name)
		if nodeName != "" {
			nodeByName[nodeName] = node
		}
	}

	pods := make([]models.KubernetesPod, 0, len(report.Pods))
	for _, p := range report.Pods {
		var usageCPUMilli int
		var usageMemoryBytes int64
		var networkRxBytes int64
		var networkTxBytes int64
		var ephemeralStorageUsedBytes int64
		var ephemeralStorageCapacityBytes int64
		if p.Usage != nil {
			usageCPUMilli = int(p.Usage.CPUMilliCores)
			usageMemoryBytes = p.Usage.MemoryBytes
			networkRxBytes = p.Usage.NetworkRxBytes
			networkTxBytes = p.Usage.NetworkTxBytes
			ephemeralStorageUsedBytes = p.Usage.EphemeralStorageUsedBytes
			ephemeralStorageCapacityBytes = p.Usage.EphemeralStorageCapacityBytes
		}
		labels := make(map[string]string, len(p.Labels))
		for k, v := range p.Labels {
			labels[k] = v
		}
		containers := make([]models.KubernetesPodContainer, 0, len(p.Containers))
		for _, c := range p.Containers {
			containers = append(containers, models.KubernetesPodContainer{
				Name:         strings.TrimSpace(c.Name),
				Image:        strings.TrimSpace(c.Image),
				Ready:        c.Ready,
				RestartCount: c.RestartCount,
				State:        strings.TrimSpace(c.State),
				Reason:       strings.TrimSpace(c.Reason),
				Message:      strings.TrimSpace(c.Message),
			})
		}
		pods = append(pods, models.KubernetesPod{
			UID:                           strings.TrimSpace(p.UID),
			Name:                          strings.TrimSpace(p.Name),
			Namespace:                     strings.TrimSpace(p.Namespace),
			NodeName:                      strings.TrimSpace(p.NodeName),
			Phase:                         strings.TrimSpace(p.Phase),
			Reason:                        strings.TrimSpace(p.Reason),
			Message:                       strings.TrimSpace(p.Message),
			QoSClass:                      strings.TrimSpace(p.QoSClass),
			CreatedAt:                     p.CreatedAt,
			StartTime:                     p.StartTime,
			Restarts:                      p.Restarts,
			UsageCPUMilliCores:            usageCPUMilli,
			UsageMemoryBytes:              usageMemoryBytes,
			UsageCPUPercent:               kubernetesCPUUsagePercent(int64(usageCPUMilli), nodeAllocCPU(nodeByName, p.NodeName), nodeCapacityCPU(nodeByName, p.NodeName)),
			UsageMemoryPercent:            kubernetesMemoryUsagePercent(usageMemoryBytes, nodeAllocMemory(nodeByName, p.NodeName), nodeCapacityMemory(nodeByName, p.NodeName)),
			NetworkRxBytes:                networkRxBytes,
			NetworkTxBytes:                networkTxBytes,
			EphemeralStorageUsedBytes:     ephemeralStorageUsedBytes,
			EphemeralStorageCapacityBytes: ephemeralStorageCapacityBytes,
			DiskUsagePercent:              kubernetesDiskUsagePercent(ephemeralStorageUsedBytes, ephemeralStorageCapacityBytes),
			Labels:                        labels,
			OwnerKind:                     strings.TrimSpace(p.OwnerKind),
			OwnerName:                     strings.TrimSpace(p.OwnerName),
			Containers:                    containers,
		})
	}

	clusterKey := models.KubernetesCluster{
		ID:          identifier,
		Name:        name,
		DisplayName: displayName,
	}
	if m.rateTracker != nil {
		for i := range pods {
			metricID := kubernetesPodMetricID(clusterKey, pods[i])
			if metricID == "" {
				continue
			}
			currentMetrics := IOMetrics{
				NetworkIn:  pods[i].NetworkRxBytes,
				NetworkOut: pods[i].NetworkTxBytes,
				Timestamp:  timestamp,
			}
			_, _, netInRate, netOutRate := m.rateTracker.CalculateRates(metricID, currentMetrics)
			if netInRate > 0 {
				pods[i].NetInRate = netInRate
			}
			if netOutRate > 0 {
				pods[i].NetOutRate = netOutRate
			}
		}
	}

	deployments := make([]models.KubernetesDeployment, 0, len(report.Deployments))
	for _, d := range report.Deployments {
		labels := make(map[string]string, len(d.Labels))
		for k, v := range d.Labels {
			labels[k] = v
		}
		deployments = append(deployments, models.KubernetesDeployment{
			UID:               strings.TrimSpace(d.UID),
			Name:              strings.TrimSpace(d.Name),
			Namespace:         strings.TrimSpace(d.Namespace),
			DesiredReplicas:   d.DesiredReplicas,
			UpdatedReplicas:   d.UpdatedReplicas,
			ReadyReplicas:     d.ReadyReplicas,
			AvailableReplicas: d.AvailableReplicas,
			Labels:            labels,
		})
	}

	namespaces := convertKubernetesNamespaces(report.Namespaces)
	statefulSets := convertKubernetesStatefulSets(report.StatefulSets)
	daemonSets := convertKubernetesDaemonSets(report.DaemonSets)
	services := convertKubernetesServices(report.Services)
	jobs := convertKubernetesJobs(report.Jobs)
	cronJobs := convertKubernetesCronJobs(report.CronJobs)
	ingresses := convertKubernetesIngresses(report.Ingresses)
	persistentVolumes := convertKubernetesPersistentVolumes(report.PersistentVolumes)
	persistentVolumeClaims := convertKubernetesPersistentVolumeClaims(report.PersistentVolumeClaims)
	events := convertKubernetesEvents(report.Events)

	agentVersion := normalizeAgentVersion(report.Agent.Version)

	cluster := models.KubernetesCluster{
		ID:                     identifier,
		AgentID:                agentID,
		Name:                   name,
		DisplayName:            displayName,
		Server:                 strings.TrimSpace(report.Cluster.Server),
		Context:                strings.TrimSpace(report.Cluster.Context),
		Version:                strings.TrimSpace(report.Cluster.Version),
		Status:                 "online",
		LastSeen:               timestamp,
		IntervalSeconds:        report.Agent.IntervalSeconds,
		AgentVersion:           agentVersion,
		Nodes:                  nodes,
		Namespaces:             namespaces,
		Pods:                   pods,
		Deployments:            deployments,
		StatefulSets:           statefulSets,
		DaemonSets:             daemonSets,
		Services:               services,
		Jobs:                   jobs,
		CronJobs:               cronJobs,
		Ingresses:              ingresses,
		PersistentVolumes:      persistentVolumes,
		PersistentVolumeClaims: persistentVolumeClaims,
		Events:                 events,
	}

	if tokenRecord != nil {
		cluster.TokenID = strings.TrimSpace(tokenRecord.ID)
		cluster.TokenName = strings.TrimSpace(tokenRecord.Name)
		cluster.TokenHint = tokenHintFromRecord(tokenRecord)
		cluster.TokenLastUsedAt = tokenRecord.LastUsedAt
	}

	m.state.UpsertKubernetesCluster(cluster)
	m.state.SetConnectionHealth(kubernetesConnectionPrefix+identifier, true)
	m.recordKubernetesPodMetrics(cluster, timestamp)

	return cluster, nil
}

func convertKubernetesNamespaces(namespaces []agentsk8s.Namespace) []models.KubernetesNamespace {
	if len(namespaces) == 0 {
		return nil
	}
	out := make([]models.KubernetesNamespace, 0, len(namespaces))
	for _, namespace := range namespaces {
		out = append(out, models.KubernetesNamespace{
			UID:       strings.TrimSpace(namespace.UID),
			Name:      strings.TrimSpace(namespace.Name),
			Phase:     strings.TrimSpace(namespace.Phase),
			CreatedAt: namespace.CreatedAt,
			Labels:    cloneStringMap(namespace.Labels),
		}.NormalizeCollections())
	}
	return out
}

func convertKubernetesStatefulSets(statefulSets []agentsk8s.StatefulSet) []models.KubernetesStatefulSet {
	if len(statefulSets) == 0 {
		return nil
	}
	out := make([]models.KubernetesStatefulSet, 0, len(statefulSets))
	for _, statefulSet := range statefulSets {
		out = append(out, models.KubernetesStatefulSet{
			UID:               strings.TrimSpace(statefulSet.UID),
			Name:              strings.TrimSpace(statefulSet.Name),
			Namespace:         strings.TrimSpace(statefulSet.Namespace),
			DesiredReplicas:   statefulSet.DesiredReplicas,
			ReadyReplicas:     statefulSet.ReadyReplicas,
			CurrentReplicas:   statefulSet.CurrentReplicas,
			UpdatedReplicas:   statefulSet.UpdatedReplicas,
			AvailableReplicas: statefulSet.AvailableReplicas,
			ServiceName:       strings.TrimSpace(statefulSet.ServiceName),
			Labels:            cloneStringMap(statefulSet.Labels),
		}.NormalizeCollections())
	}
	return out
}

func convertKubernetesDaemonSets(daemonSets []agentsk8s.DaemonSet) []models.KubernetesDaemonSet {
	if len(daemonSets) == 0 {
		return nil
	}
	out := make([]models.KubernetesDaemonSet, 0, len(daemonSets))
	for _, daemonSet := range daemonSets {
		out = append(out, models.KubernetesDaemonSet{
			UID:                    strings.TrimSpace(daemonSet.UID),
			Name:                   strings.TrimSpace(daemonSet.Name),
			Namespace:              strings.TrimSpace(daemonSet.Namespace),
			DesiredNumberScheduled: daemonSet.DesiredNumberScheduled,
			CurrentNumberScheduled: daemonSet.CurrentNumberScheduled,
			NumberReady:            daemonSet.NumberReady,
			UpdatedNumberScheduled: daemonSet.UpdatedNumberScheduled,
			NumberAvailable:        daemonSet.NumberAvailable,
			NumberUnavailable:      daemonSet.NumberUnavailable,
			NumberMisscheduled:     daemonSet.NumberMisscheduled,
			Labels:                 cloneStringMap(daemonSet.Labels),
		}.NormalizeCollections())
	}
	return out
}

func convertKubernetesServices(services []agentsk8s.Service) []models.KubernetesService {
	if len(services) == 0 {
		return nil
	}
	out := make([]models.KubernetesService, 0, len(services))
	for _, service := range services {
		ports := make([]models.KubernetesServicePort, 0, len(service.Ports))
		for _, port := range service.Ports {
			ports = append(ports, models.KubernetesServicePort{
				Name:       strings.TrimSpace(port.Name),
				Protocol:   strings.TrimSpace(port.Protocol),
				Port:       port.Port,
				TargetPort: strings.TrimSpace(port.TargetPort),
				NodePort:   port.NodePort,
			})
		}
		out = append(out, models.KubernetesService{
			UID:         strings.TrimSpace(service.UID),
			Name:        strings.TrimSpace(service.Name),
			Namespace:   strings.TrimSpace(service.Namespace),
			ServiceType: strings.TrimSpace(service.Type),
			ClusterIP:   strings.TrimSpace(service.ClusterIP),
			ExternalIPs: append([]string(nil), service.ExternalIPs...),
			Ports:       ports,
			Selector:    cloneStringMap(service.Selector),
			CreatedAt:   service.CreatedAt,
			Labels:      cloneStringMap(service.Labels),
		}.NormalizeCollections())
	}
	return out
}

func convertKubernetesJobs(jobs []agentsk8s.Job) []models.KubernetesJob {
	if len(jobs) == 0 {
		return nil
	}
	out := make([]models.KubernetesJob, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, models.KubernetesJob{
			UID:                strings.TrimSpace(job.UID),
			Name:               strings.TrimSpace(job.Name),
			Namespace:          strings.TrimSpace(job.Namespace),
			DesiredCompletions: job.DesiredCompletions,
			Succeeded:          job.Succeeded,
			Failed:             job.Failed,
			Active:             job.Active,
			StartTime:          cloneReportTimePtr(job.StartTime),
			CompletionTime:     cloneReportTimePtr(job.CompletionTime),
			Labels:             cloneStringMap(job.Labels),
		}.NormalizeCollections())
	}
	return out
}

func convertKubernetesCronJobs(cronJobs []agentsk8s.CronJob) []models.KubernetesCronJob {
	if len(cronJobs) == 0 {
		return nil
	}
	out := make([]models.KubernetesCronJob, 0, len(cronJobs))
	for _, cronJob := range cronJobs {
		out = append(out, models.KubernetesCronJob{
			UID:                strings.TrimSpace(cronJob.UID),
			Name:               strings.TrimSpace(cronJob.Name),
			Namespace:          strings.TrimSpace(cronJob.Namespace),
			Schedule:           strings.TrimSpace(cronJob.Schedule),
			Suspend:            cronJob.Suspend,
			Active:             cronJob.Active,
			LastScheduleTime:   cloneReportTimePtr(cronJob.LastScheduleTime),
			LastSuccessfulTime: cloneReportTimePtr(cronJob.LastSuccessfulTime),
			Labels:             cloneStringMap(cronJob.Labels),
		}.NormalizeCollections())
	}
	return out
}

func convertKubernetesIngresses(ingresses []agentsk8s.Ingress) []models.KubernetesIngress {
	if len(ingresses) == 0 {
		return nil
	}
	out := make([]models.KubernetesIngress, 0, len(ingresses))
	for _, ingress := range ingresses {
		out = append(out, models.KubernetesIngress{
			UID:       strings.TrimSpace(ingress.UID),
			Name:      strings.TrimSpace(ingress.Name),
			Namespace: strings.TrimSpace(ingress.Namespace),
			ClassName: strings.TrimSpace(ingress.ClassName),
			Hosts:     append([]string(nil), ingress.Hosts...),
			Addresses: append([]string(nil), ingress.Addresses...),
			CreatedAt: ingress.CreatedAt,
			Labels:    cloneStringMap(ingress.Labels),
		}.NormalizeCollections())
	}
	return out
}

func convertKubernetesPersistentVolumes(volumes []agentsk8s.PersistentVolume) []models.KubernetesPersistentVolume {
	if len(volumes) == 0 {
		return nil
	}
	out := make([]models.KubernetesPersistentVolume, 0, len(volumes))
	for _, volume := range volumes {
		out = append(out, models.KubernetesPersistentVolume{
			UID:            strings.TrimSpace(volume.UID),
			Name:           strings.TrimSpace(volume.Name),
			Phase:          strings.TrimSpace(volume.Phase),
			StorageClass:   strings.TrimSpace(volume.StorageClass),
			CapacityBytes:  volume.CapacityBytes,
			AccessModes:    append([]string(nil), volume.AccessModes...),
			ReclaimPolicy:  strings.TrimSpace(volume.ReclaimPolicy),
			ClaimNamespace: strings.TrimSpace(volume.ClaimNamespace),
			ClaimName:      strings.TrimSpace(volume.ClaimName),
			CreatedAt:      volume.CreatedAt,
			Labels:         cloneStringMap(volume.Labels),
		}.NormalizeCollections())
	}
	return out
}

func convertKubernetesPersistentVolumeClaims(claims []agentsk8s.PersistentVolumeClaim) []models.KubernetesPersistentVolumeClaim {
	if len(claims) == 0 {
		return nil
	}
	out := make([]models.KubernetesPersistentVolumeClaim, 0, len(claims))
	for _, claim := range claims {
		out = append(out, models.KubernetesPersistentVolumeClaim{
			UID:            strings.TrimSpace(claim.UID),
			Name:           strings.TrimSpace(claim.Name),
			Namespace:      strings.TrimSpace(claim.Namespace),
			Phase:          strings.TrimSpace(claim.Phase),
			StorageClass:   strings.TrimSpace(claim.StorageClass),
			RequestedBytes: claim.RequestedBytes,
			CapacityBytes:  claim.CapacityBytes,
			AccessModes:    append([]string(nil), claim.AccessModes...),
			VolumeName:     strings.TrimSpace(claim.VolumeName),
			CreatedAt:      claim.CreatedAt,
			Labels:         cloneStringMap(claim.Labels),
		}.NormalizeCollections())
	}
	return out
}

func convertKubernetesEvents(events []agentsk8s.Event) []models.KubernetesEvent {
	if len(events) == 0 {
		return nil
	}
	out := make([]models.KubernetesEvent, 0, len(events))
	for _, event := range events {
		out = append(out, models.KubernetesEvent{
			UID:          strings.TrimSpace(event.UID),
			Name:         strings.TrimSpace(event.Name),
			Namespace:    strings.TrimSpace(event.Namespace),
			EventType:    strings.TrimSpace(event.Type),
			Reason:       strings.TrimSpace(event.Reason),
			Message:      strings.TrimSpace(event.Message),
			InvolvedKind: strings.TrimSpace(event.InvolvedKind),
			InvolvedName: strings.TrimSpace(event.InvolvedName),
			Count:        event.Count,
			FirstSeen:    cloneReportTimePtr(event.FirstSeen),
			LastSeen:     cloneReportTimePtr(event.LastSeen),
			EventTime:    cloneReportTimePtr(event.EventTime),
		})
	}
	return out
}

func cloneReportTimePtr(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}
	dest := *src
	return &dest
}

func kubernetesCPUUsagePercent(usageMilli, allocCPU, capacityCPU int64) float64 {
	denom := allocCPU
	if denom <= 0 {
		denom = capacityCPU
	}
	if usageMilli <= 0 || denom <= 0 {
		return 0
	}
	percent := (float64(usageMilli) / float64(denom*1000)) * 100
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func kubernetesMemoryUsagePercent(usageBytes, allocMemoryBytes, capacityMemoryBytes int64) float64 {
	denom := allocMemoryBytes
	if denom <= 0 {
		denom = capacityMemoryBytes
	}
	if usageBytes <= 0 || denom <= 0 {
		return 0
	}
	percent := (float64(usageBytes) / float64(denom)) * 100
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func kubernetesDiskUsagePercent(usedBytes, capacityBytes int64) float64 {
	if usedBytes <= 0 || capacityBytes <= 0 {
		return 0
	}
	percent := (float64(usedBytes) / float64(capacityBytes)) * 100
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func nodeAllocCPU(nodes map[string]models.KubernetesNode, nodeName string) int64 {
	if node, ok := nodes[strings.TrimSpace(nodeName)]; ok {
		return node.AllocCPU
	}
	return 0
}

func nodeCapacityCPU(nodes map[string]models.KubernetesNode, nodeName string) int64 {
	if node, ok := nodes[strings.TrimSpace(nodeName)]; ok {
		return node.CapacityCPU
	}
	return 0
}

func nodeAllocMemory(nodes map[string]models.KubernetesNode, nodeName string) int64 {
	if node, ok := nodes[strings.TrimSpace(nodeName)]; ok {
		return node.AllocMemoryBytes
	}
	return 0
}

func nodeCapacityMemory(nodes map[string]models.KubernetesNode, nodeName string) int64 {
	if node, ok := nodes[strings.TrimSpace(nodeName)]; ok {
		return node.CapacityMemoryBytes
	}
	return 0
}

func (m *Monitor) recordKubernetesPodMetrics(cluster models.KubernetesCluster, timestamp time.Time) {
	if m == nil {
		return
	}
	if m.metricsHistory == nil && m.metricsStore == nil {
		return
	}

	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	if shouldSkipNativeMockStateMetricWrites() {
		return
	}

	for _, pod := range cluster.Pods {
		metricID := kubernetesPodMetricID(cluster, pod)
		if metricID == "" {
			continue
		}

		if pod.UsageCPUPercent > 0 {
			if m.metricsHistory != nil {
				m.metricsHistory.AddGuestMetric(metricID, "cpu", pod.UsageCPUPercent, timestamp)
			}
			if m.metricsStore != nil {
				m.metricsStore.Write("k8s", metricID, "cpu", pod.UsageCPUPercent, timestamp)
			}
		}
		if pod.UsageMemoryPercent > 0 {
			if m.metricsHistory != nil {
				m.metricsHistory.AddGuestMetric(metricID, "memory", pod.UsageMemoryPercent, timestamp)
			}
			if m.metricsStore != nil {
				m.metricsStore.Write("k8s", metricID, "memory", pod.UsageMemoryPercent, timestamp)
			}
		}
		if pod.DiskUsagePercent > 0 {
			if m.metricsHistory != nil {
				m.metricsHistory.AddGuestMetric(metricID, "disk", pod.DiskUsagePercent, timestamp)
			}
			if m.metricsStore != nil {
				m.metricsStore.Write("k8s", metricID, "disk", pod.DiskUsagePercent, timestamp)
			}
		}
		if pod.NetInRate > 0 {
			if m.metricsHistory != nil {
				m.metricsHistory.AddGuestMetric(metricID, "netin", pod.NetInRate, timestamp)
			}
			if m.metricsStore != nil {
				m.metricsStore.Write("k8s", metricID, "netin", pod.NetInRate, timestamp)
			}
		}
		if pod.NetOutRate > 0 {
			if m.metricsHistory != nil {
				m.metricsHistory.AddGuestMetric(metricID, "netout", pod.NetOutRate, timestamp)
			}
			if m.metricsStore != nil {
				m.metricsStore.Write("k8s", metricID, "netout", pod.NetOutRate, timestamp)
			}
		}
	}
}

// RemoveKubernetesCluster removes a kubernetes cluster from the shared state and clears related data.
func (m *Monitor) RemoveKubernetesCluster(clusterID string) (models.KubernetesCluster, error) {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return models.KubernetesCluster{}, fmt.Errorf("kubernetes cluster id is required")
	}

	cluster, removed := m.state.RemoveKubernetesCluster(clusterID)
	if !removed {
		if logging.IsLevelEnabled(zerolog.DebugLevel) {
			log.Debug().Str("k8sClusterID", clusterID).Msg("kubernetes cluster not present in state during removal; proceeding")
		}
		cluster = models.KubernetesCluster{
			ID:          clusterID,
			Name:        clusterID,
			DisplayName: clusterID,
		}
	}

	// Revoke the API token associated with this Kubernetes cluster
	if cluster.TokenID != "" {
		tokenRemoved := m.config.RemoveAPIToken(cluster.TokenID)
		if tokenRemoved != nil {
			m.config.SortAPITokens()

			if m.persistence != nil {
				if err := m.persistence.SaveAPITokens(m.config.APITokens); err != nil {
					log.Warn().Err(err).Str("tokenID", cluster.TokenID).Msg("failed to persist API token revocation after Kubernetes cluster removal")
				} else {
					log.Info().Str("tokenID", cluster.TokenID).Str("tokenName", cluster.TokenName).Msg("API token revoked for removed Kubernetes cluster")
				}
			}
		}
	}

	removedAt := time.Now()

	m.mu.Lock()
	m.removedKubernetesClusters[clusterID] = removedAt
	if cluster.TokenID != "" {
		delete(m.kubernetesTokenBindings, cluster.TokenID)
		log.Debug().
			Str("tokenID", cluster.TokenID).
			Str("k8sClusterID", clusterID).
			Msg("Unbound Kubernetes agent token from removed cluster")
	}
	m.mu.Unlock()

	m.state.AddRemovedKubernetesCluster(models.RemovedKubernetesCluster{
		ID:          clusterID,
		Name:        cluster.Name,
		DisplayName: cluster.DisplayName,
		RemovedAt:   removedAt,
	})

	m.state.RemoveConnectionHealth(kubernetesConnectionPrefix + clusterID)

	log.Info().
		Str("k8sClusterID", clusterID).
		Bool("removed", removed).
		Msg("Kubernetes cluster removed")

	return cluster, nil
}

// AllowKubernetesClusterReenroll clears the removal block for a kubernetes cluster.
func (m *Monitor) AllowKubernetesClusterReenroll(clusterID string) error {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return fmt.Errorf("kubernetes cluster id is required")
	}

	m.mu.Lock()
	_, exists := m.removedKubernetesClusters[clusterID]
	if !exists {
		m.mu.Unlock()
		return nil
	}
	delete(m.removedKubernetesClusters, clusterID)
	m.mu.Unlock()

	m.state.RemoveRemovedKubernetesCluster(clusterID)
	return nil
}

// UnhideKubernetesCluster clears the hidden flag.
func (m *Monitor) UnhideKubernetesCluster(clusterID string) (models.KubernetesCluster, error) {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return models.KubernetesCluster{}, fmt.Errorf("kubernetes cluster id is required")
	}

	cluster, ok := m.state.SetKubernetesClusterHidden(clusterID, false)
	if !ok {
		return models.KubernetesCluster{}, fmt.Errorf("kubernetes cluster %q not found", clusterID)
	}
	return cluster, nil
}

// MarkKubernetesClusterPendingUninstall sets the pending uninstall flag on a cluster.
func (m *Monitor) MarkKubernetesClusterPendingUninstall(clusterID string) (models.KubernetesCluster, error) {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return models.KubernetesCluster{}, fmt.Errorf("kubernetes cluster id is required")
	}

	cluster, ok := m.state.SetKubernetesClusterPendingUninstall(clusterID, true)
	if !ok {
		return models.KubernetesCluster{}, fmt.Errorf("kubernetes cluster %q not found", clusterID)
	}
	return cluster, nil
}

// SetKubernetesClusterCustomDisplayName updates the custom display name for a cluster.
func (m *Monitor) SetKubernetesClusterCustomDisplayName(clusterID, customName string) (models.KubernetesCluster, error) {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return models.KubernetesCluster{}, fmt.Errorf("kubernetes cluster id is required")
	}

	cluster, ok := m.state.SetKubernetesClusterCustomDisplayName(clusterID, customName)
	if !ok {
		return models.KubernetesCluster{}, fmt.Errorf("kubernetes cluster %q not found", clusterID)
	}
	return cluster, nil
}

func (m *Monitor) evaluateKubernetesAgents(now time.Time) {
	clusters := m.state.GetKubernetesClusters()
	for _, cluster := range clusters {
		interval := cluster.IntervalSeconds
		if interval <= 0 {
			interval = int(kubernetesMinimumHealthWindow / time.Second)
		}

		window := time.Duration(interval) * time.Second * kubernetesOfflineGraceMultiplier
		if window < kubernetesMinimumHealthWindow {
			window = kubernetesMinimumHealthWindow
		} else if window > kubernetesMaximumHealthWindow {
			window = kubernetesMaximumHealthWindow
		}

		healthy := !cluster.LastSeen.IsZero() && now.Sub(cluster.LastSeen) <= window
		key := kubernetesConnectionPrefix + cluster.ID
		m.state.SetConnectionHealth(key, healthy)
		if healthy {
			m.state.SetKubernetesClusterStatus(cluster.ID, "online")
		} else {
			m.state.SetKubernetesClusterStatus(cluster.ID, "offline")
		}
	}
}

func (m *Monitor) cleanupRemovedKubernetesClusters(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for clusterID, removedAt := range m.removedKubernetesClusters {
		if now.Sub(removedAt) > removedKubernetesClustersTTL {
			delete(m.removedKubernetesClusters, clusterID)
			m.state.RemoveRemovedKubernetesCluster(clusterID)
			if logging.IsLevelEnabled(zerolog.DebugLevel) {
				log.Debug().
					Str("k8sClusterID", clusterID).
					Time("removedAt", removedAt).
					Msg("Cleaned up stale removed Kubernetes cluster block")
			}
		}
	}
}
