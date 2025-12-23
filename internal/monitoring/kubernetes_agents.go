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
		return models.KubernetesCluster{}, fmt.Errorf("kubernetes cluster %q was removed at %v and cannot report again. Use Allow re-enroll in Settings -> Agents -> Removed Kubernetes Clusters or rerun the installer with a kubernetes:manage token to clear this block", identifier, removedAt.Format(time.RFC3339))
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
			Roles:                   roles,
		})
	}

	pods := make([]models.KubernetesPod, 0, len(report.Pods))
	for _, p := range report.Pods {
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
			UID:        strings.TrimSpace(p.UID),
			Name:       strings.TrimSpace(p.Name),
			Namespace:  strings.TrimSpace(p.Namespace),
			NodeName:   strings.TrimSpace(p.NodeName),
			Phase:      strings.TrimSpace(p.Phase),
			Reason:     strings.TrimSpace(p.Reason),
			Message:    strings.TrimSpace(p.Message),
			QoSClass:   strings.TrimSpace(p.QoSClass),
			CreatedAt:  p.CreatedAt,
			StartTime:  p.StartTime,
			Restarts:   p.Restarts,
			Labels:     labels,
			OwnerKind:  strings.TrimSpace(p.OwnerKind),
			OwnerName:  strings.TrimSpace(p.OwnerName),
			Containers: containers,
		})
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

	agentVersion := normalizeAgentVersion(report.Agent.Version)

	cluster := models.KubernetesCluster{
		ID:              identifier,
		AgentID:         agentID,
		Name:            name,
		DisplayName:     displayName,
		Server:          strings.TrimSpace(report.Cluster.Server),
		Context:         strings.TrimSpace(report.Cluster.Context),
		Version:         strings.TrimSpace(report.Cluster.Version),
		Status:          "online",
		LastSeen:        timestamp,
		IntervalSeconds: report.Agent.IntervalSeconds,
		AgentVersion:    agentVersion,
		Nodes:           nodes,
		Pods:            pods,
		Deployments:     deployments,
	}

	if tokenRecord != nil {
		cluster.TokenID = strings.TrimSpace(tokenRecord.ID)
		cluster.TokenName = strings.TrimSpace(tokenRecord.Name)
		cluster.TokenHint = tokenHintFromRecord(tokenRecord)
		cluster.TokenLastUsedAt = tokenRecord.LastUsedAt
	}

	m.state.UpsertKubernetesCluster(cluster)
	m.state.SetConnectionHealth(kubernetesConnectionPrefix+identifier, true)

	return cluster, nil
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
			log.Debug().Str("k8sClusterID", clusterID).Msg("Kubernetes cluster not present in state during removal; proceeding")
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
			m.config.APITokenEnabled = m.config.HasAPITokens()

			if m.persistence != nil {
				if err := m.persistence.SaveAPITokens(m.config.APITokens); err != nil {
					log.Warn().Err(err).Str("tokenID", cluster.TokenID).Msg("Failed to persist API token revocation after Kubernetes cluster removal")
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
