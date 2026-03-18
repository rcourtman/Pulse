package ai

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	maxKubernetesNodeIssues       = 15
	maxKubernetesPodIssues        = 25
	maxKubernetesPodRestartLeads  = 10
	maxKubernetesDeploymentIssues = 15
	maxKubernetesMessageLength    = 160
)

var (
	ErrKubernetesStateUnavailable = errors.New("kubernetes state unavailable")
	ErrKubernetesClusterNotFound  = errors.New("kubernetes cluster not found")
)

// AnalyzeKubernetesCluster runs AI analysis for a specific Kubernetes cluster.
func (s *Service) AnalyzeKubernetesCluster(ctx context.Context, clusterID string) (*ExecuteResponse, error) {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return nil, fmt.Errorf("cluster_id is required")
	}

	s.mu.RLock()
	rs := s.readState
	s.mu.RUnlock()
	if rs == nil {
		return nil, ErrKubernetesStateUnavailable
	}

	cluster := findK8sCluster(rs.K8sClusters(), clusterID)
	if cluster == nil {
		return nil, ErrKubernetesClusterNotFound
	}

	clusterName := cluster.Name()
	prompt := fmt.Sprintf(
		"Analyze the Kubernetes cluster %q. Summarize health, highlight critical issues, and suggest the next actions. Be concise and specific to the telemetry.",
		clusterName,
	)

	clusterSourceID := cluster.ClusterID()
	systemPrompt := s.buildSystemPrompt(ExecuteRequest{
		Prompt:     prompt,
		TargetType: "k8s-cluster",
		TargetID:   clusterSourceID,
	})
	systemPrompt += "\n\n## Kubernetes Cluster Telemetry\n"
	systemPrompt += buildK8sClusterContext(cluster, rs)
	systemPrompt += "\n\nUse the telemetry above only. Do not request kubectl output."

	return s.Execute(ctx, ExecuteRequest{
		Prompt:       prompt,
		TargetType:   "k8s-cluster",
		TargetID:     clusterSourceID,
		SystemPrompt: systemPrompt,
		UseCase:      "chat",
	})
}

func findK8sCluster(clusters []*unifiedresources.K8sClusterView, clusterID string) *unifiedresources.K8sClusterView {
	for _, cluster := range clusters {
		if cluster == nil {
			continue
		}
		if cluster.ClusterID() == clusterID {
			return cluster
		}
	}
	return nil
}

func buildK8sClusterContext(cluster *unifiedresources.K8sClusterView, rs unifiedresources.ReadState) string {
	var b strings.Builder

	clusterName := cluster.Name()
	b.WriteString("### Cluster Summary\n")
	b.WriteString(fmt.Sprintf("- Name: %s\n", clusterName))
	b.WriteString(fmt.Sprintf("- ID: %s\n", cluster.ClusterID()))
	if status := cluster.SourceStatus(); status != "" {
		b.WriteString(fmt.Sprintf("- Status: %s\n", status))
	}
	if v := cluster.Version(); v != "" {
		b.WriteString(fmt.Sprintf("- Version: %s\n", v))
	}
	if s := cluster.Server(); s != "" {
		b.WriteString(fmt.Sprintf("- API server: %s\n", s))
	}
	if ctx := cluster.Context(); ctx != "" {
		b.WriteString(fmt.Sprintf("- Context: %s\n", ctx))
	}
	if av := cluster.AgentVersion(); av != "" {
		b.WriteString(fmt.Sprintf("- Agent version: %s\n", av))
	}
	if is := cluster.IntervalSeconds(); is > 0 {
		b.WriteString(fmt.Sprintf("- Telemetry interval: %ds\n", is))
	}
	if ls := cluster.LastSeen(); !ls.IsZero() {
		age := formatKubernetesAge(time.Since(ls))
		b.WriteString(fmt.Sprintf("- Last seen: %s (%s ago)\n", ls.Format(time.RFC3339), age))
	}
	if cluster.PendingUninstall() {
		b.WriteString("- Pending uninstall: true\n")
	}

	// Filter nodes, pods, and deployments belonging to this cluster by ParentID.
	clusterResID := cluster.ID()
	var clusterNodes []*unifiedresources.K8sNodeView
	for _, node := range rs.K8sNodes() {
		if node != nil && node.ParentID() == clusterResID {
			clusterNodes = append(clusterNodes, node)
		}
	}
	var clusterPods []*unifiedresources.PodView
	for _, pod := range rs.Pods() {
		if pod != nil && pod.ParentID() == clusterResID {
			clusterPods = append(clusterPods, pod)
		}
	}
	var clusterDeploys []*unifiedresources.K8sDeploymentView
	for _, deploy := range rs.K8sDeployments() {
		if deploy != nil && deploy.ParentID() == clusterResID {
			clusterDeploys = append(clusterDeploys, deploy)
		}
	}

	nodeSummary, nodeIssues := summarizeK8sNodes(clusterNodes)
	podSummary, podIssues, restartLeaders := summarizeK8sPods(clusterPods)
	deploymentSummary, deploymentIssues := summarizeK8sDeployments(clusterDeploys)

	b.WriteString("\n### Workload Summary\n")
	b.WriteString(nodeSummary)
	b.WriteString(podSummary)
	b.WriteString(deploymentSummary)

	if len(nodeIssues) > 0 {
		b.WriteString("\n### Unhealthy Nodes\n")
		for _, issue := range nodeIssues {
			b.WriteString("- ")
			b.WriteString(issue)
			b.WriteString("\n")
		}
	}

	if len(podIssues) > 0 {
		b.WriteString("\n### Unhealthy Pods\n")
		for _, issue := range podIssues {
			b.WriteString("- ")
			b.WriteString(issue)
			b.WriteString("\n")
		}
	}

	if len(restartLeaders) > 0 {
		b.WriteString("\n### Pods With Restarts\n")
		for _, entry := range restartLeaders {
			b.WriteString("- ")
			b.WriteString(entry)
			b.WriteString("\n")
		}
	}

	if len(deploymentIssues) > 0 {
		b.WriteString("\n### Deployments Not Fully Available\n")
		for _, issue := range deploymentIssues {
			b.WriteString("- ")
			b.WriteString(issue)
			b.WriteString("\n")
		}
	}

	return strings.TrimSpace(b.String())
}

func summarizeK8sNodes(nodes []*unifiedresources.K8sNodeView) (string, []string) {
	total := len(nodes)
	ready := 0
	notReady := 0
	unschedulable := 0
	var issues []string

	for _, node := range nodes {
		if node.Ready() {
			ready++
		} else {
			notReady++
		}
		if node.Unschedulable() {
			unschedulable++
		}

		if !node.Ready() || node.Unschedulable() {
			issue := fmt.Sprintf("%s (ready=%t, unschedulable=%t)", node.Name(), node.Ready(), node.Unschedulable())
			issues = append(issues, issue)
		}
	}

	if len(issues) > maxKubernetesNodeIssues {
		issues = append(issues[:maxKubernetesNodeIssues], fmt.Sprintf("... and %d more", len(issues)-maxKubernetesNodeIssues))
	}

	summary := fmt.Sprintf("- Nodes: %d total, %d ready, %d not ready, %d unschedulable\n", total, ready, notReady, unschedulable)
	return summary, issues
}

func summarizeK8sPods(pods []*unifiedresources.PodView) (string, []string, []string) {
	total := len(pods)
	phaseCounts := make(map[string]int)
	var unhealthy []podIssue
	var restarts []podIssue

	for _, pod := range pods {
		phase := strings.ToLower(strings.TrimSpace(pod.PodPhase()))
		if phase == "" {
			phase = "unknown"
		}
		phaseCounts[phase]++

		if phase == "succeeded" {
			continue
		}

		if !isK8sPodHealthy(pod) {
			unhealthy = append(unhealthy, podIssue{
				name:      pod.Name(),
				namespace: pod.Namespace(),
				reason:    k8sPodReason(pod),
				restarts:  pod.Restarts(),
			})
		}

		if pod.Restarts() > 0 {
			restarts = append(restarts, podIssue{
				name:      pod.Name(),
				namespace: pod.Namespace(),
				reason:    k8sPodReason(pod),
				restarts:  pod.Restarts(),
			})
		}
	}

	issueLines := formatPodIssues(unhealthy, maxKubernetesPodIssues)
	restartLines := formatPodRestarts(restarts, maxKubernetesPodRestartLeads)
	summary := fmt.Sprintf(
		"- Pods: %d total, %d running, %d pending, %d failed, %d succeeded, %d unknown\n",
		total,
		phaseCounts["running"],
		phaseCounts["pending"],
		phaseCounts["failed"],
		phaseCounts["succeeded"],
		phaseCounts["unknown"],
	)
	return summary, issueLines, restartLines
}

func summarizeK8sDeployments(deployments []*unifiedresources.K8sDeploymentView) (string, []string) {
	total := len(deployments)
	healthy := 0
	var issues []string

	for _, deployment := range deployments {
		if isK8sDeploymentHealthy(deployment) {
			healthy++
			continue
		}
		issues = append(issues, fmt.Sprintf(
			"%s/%s desired=%d ready=%d updated=%d available=%d",
			deployment.Namespace(),
			deployment.Name(),
			deployment.DesiredReplicas(),
			deployment.ReadyReplicas(),
			deployment.UpdatedReplicas(),
			deployment.AvailableReplicas(),
		))
	}

	if len(issues) > maxKubernetesDeploymentIssues {
		issues = append(issues[:maxKubernetesDeploymentIssues], fmt.Sprintf("... and %d more", len(issues)-maxKubernetesDeploymentIssues))
	}

	summary := fmt.Sprintf("- Deployments: %d total, %d healthy, %d unhealthy\n", total, healthy, total-healthy)
	return summary, issues
}

type podIssue struct {
	name      string
	namespace string
	reason    string
	restarts  int
}

func formatPodIssues(issues []podIssue, limit int) []string {
	lines := make([]string, 0, min(limit, len(issues)))
	for _, issue := range issues {
		if len(lines) >= limit {
			break
		}
		lines = append(lines, formatPodIssueLine(issue))
	}
	if len(issues) > limit {
		lines = append(lines, fmt.Sprintf("... and %d more", len(issues)-limit))
	}
	return lines
}

func formatPodRestarts(issues []podIssue, limit int) []string {
	if len(issues) == 0 {
		return nil
	}
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].restarts == issues[j].restarts {
			return issues[i].name < issues[j].name
		}
		return issues[i].restarts > issues[j].restarts
	})
	if len(issues) > limit {
		issues = issues[:limit]
	}

	lines := make([]string, 0, len(issues))
	for _, issue := range issues {
		line := fmt.Sprintf("%s/%s restarts=%d", issue.namespace, issue.name, issue.restarts)
		if issue.reason != "" {
			line += " " + issue.reason
		}
		lines = append(lines, line)
	}
	return lines
}

func formatPodIssueLine(issue podIssue) string {
	base := fmt.Sprintf("%s/%s", issue.namespace, issue.name)
	if issue.reason == "" {
		return base
	}
	return fmt.Sprintf("%s %s", base, issue.reason)
}

func isK8sPodHealthy(pod *unifiedresources.PodView) bool {
	phase := strings.ToLower(strings.TrimSpace(pod.PodPhase()))
	if phase == "" {
		return false
	}
	if phase != "running" {
		return false
	}

	containers := pod.PodContainers()
	if len(containers) == 0 {
		return true
	}

	for _, container := range containers {
		if !container.Ready {
			return false
		}
		state := strings.ToLower(strings.TrimSpace(container.State))
		if state != "" && state != "running" {
			return false
		}
	}
	return true
}

func isK8sDeploymentHealthy(deployment *unifiedresources.K8sDeploymentView) bool {
	desired := deployment.DesiredReplicas()
	if desired <= 0 {
		return true
	}
	if deployment.AvailableReplicas() < desired {
		return false
	}
	if deployment.ReadyReplicas() < desired {
		return false
	}
	if deployment.UpdatedReplicas() < desired {
		return false
	}
	return true
}

func k8sPodReason(pod *unifiedresources.PodView) string {
	var parts []string
	phase := strings.TrimSpace(pod.PodPhase())
	if phase != "" {
		parts = append(parts, fmt.Sprintf("phase=%s", phase))
	}
	if reason := pod.PodReason(); reason != "" {
		parts = append(parts, fmt.Sprintf("reason=%s", reason))
	}
	if message := strings.TrimSpace(pod.PodMessage()); message != "" {
		parts = append(parts, fmt.Sprintf("message=%s", truncateKubernetesMessage(message)))
	}

	containerIssues := []string{}
	for _, container := range pod.PodContainers() {
		if container.Ready && strings.ToLower(strings.TrimSpace(container.State)) == "running" {
			continue
		}
		detail := container.Name
		if container.State != "" {
			detail += fmt.Sprintf(" state=%s", container.State)
		}
		if container.Reason != "" {
			detail += fmt.Sprintf(" reason=%s", container.Reason)
		}
		containerIssues = append(containerIssues, detail)
		if len(containerIssues) >= 3 {
			break
		}
	}
	if len(containerIssues) > 0 {
		parts = append(parts, fmt.Sprintf("containers=%s", strings.Join(containerIssues, "; ")))
	}
	if pod.Restarts() > 0 {
		parts = append(parts, fmt.Sprintf("restarts=%d", pod.Restarts()))
	}

	return strings.TrimSpace(strings.Join(parts, ", "))
}

func truncateKubernetesMessage(message string) string {
	if len(message) <= maxKubernetesMessageLength {
		return message
	}
	return message[:maxKubernetesMessageLength] + "..."
}

func formatKubernetesAge(duration time.Duration) string {
	if duration < time.Minute {
		seconds := int(duration.Seconds())
		if seconds < 0 {
			seconds = 0
		}
		return fmt.Sprintf("%ds", seconds)
	}
	if duration < time.Hour {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%dm", minutes)
	}
	if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%dh", hours)
	}
	days := int(duration.Hours() / 24)
	return fmt.Sprintf("%dd", days)
}
