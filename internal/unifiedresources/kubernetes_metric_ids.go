package unifiedresources

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func CanonicalKubernetesClusterSourceID(cluster models.KubernetesCluster) string {
	if v := strings.TrimSpace(cluster.ID); v != "" {
		return v
	}
	if v := strings.TrimSpace(cluster.AgentID); v != "" {
		return v
	}
	if v := strings.TrimSpace(cluster.Name); v != "" {
		return v
	}
	if v := strings.TrimSpace(cluster.DisplayName); v != "" {
		return v
	}
	return strings.TrimSpace(cluster.Context)
}

func CanonicalKubernetesNodeSourceID(clusterSourceID string, node models.KubernetesNode) string {
	clusterKey := strings.TrimSpace(clusterSourceID)
	nodeID := strings.TrimSpace(node.UID)
	if nodeID == "" {
		nodeID = strings.TrimSpace(node.Name)
	}
	if clusterKey == "" || nodeID == "" {
		return ""
	}
	return fmt.Sprintf("%s:node:%s", clusterKey, nodeID)
}

func CanonicalKubernetesPodSourceID(clusterSourceID string, pod models.KubernetesPod) string {
	clusterKey := strings.TrimSpace(clusterSourceID)
	podID := strings.TrimSpace(pod.UID)
	if podID == "" {
		podID = fmt.Sprintf("%s/%s", strings.TrimSpace(pod.Namespace), strings.TrimSpace(pod.Name))
	}
	if clusterKey == "" || podID == "" {
		return ""
	}
	return fmt.Sprintf("%s:pod:%s", clusterKey, podID)
}

func CanonicalKubernetesDeploymentSourceID(clusterSourceID string, deployment models.KubernetesDeployment) string {
	clusterKey := strings.TrimSpace(clusterSourceID)
	deploymentID := strings.TrimSpace(deployment.UID)
	if deploymentID == "" {
		deploymentID = fmt.Sprintf("%s/%s", strings.TrimSpace(deployment.Namespace), strings.TrimSpace(deployment.Name))
	}
	if clusterKey == "" || deploymentID == "" {
		return ""
	}
	return fmt.Sprintf("%s:deployment:%s", clusterKey, deploymentID)
}
