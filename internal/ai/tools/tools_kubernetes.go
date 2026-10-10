package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type kubernetesClusterTarget struct {
	ID          string
	AgentID     string
	Name        string
	DisplayName string
	Server      string
	Context     string
	Version     string
	Status      string
}

// registerKubernetesTools registers the pulse_kubernetes tool
func (e *PulseToolExecutor) registerKubernetesTools() {
	e.registry.registerBuiltin(RegisteredTool{
		Definition: Tool{
			Name:        agentcapabilities.PulseKubernetesToolName,
			Description: `Read Kubernetes clusters, nodes, pods, deployments, and bounded logs. Workload mutations remain unavailable until typed capability, durable delivery, and compensation contracts are complete.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"type": {
						Type:        "string",
						Description: "Operation type",
						Enum:        []string{"clusters", "nodes", "pods", "deployments", "logs"},
					},
					"cluster": {
						Type:        "string",
						Description: "Cluster name or ID",
					},
					"namespace": {
						Type:        "string",
						Description: "Kubernetes namespace (default: 'default')",
					},
					"deployment": {
						Type:        "string",
						Description: "Deployment name (for scale, restart)",
					},
					"pod": {
						Type:        "string",
						Description: "Pod name (for delete_pod, exec, logs)",
					},
					"container": {
						Type:        "string",
						Description: "Container name (for exec, logs - uses first container if omitted)",
					},
					"command": {
						Type:        "string",
						Description: "Command to execute (for exec)",
					},
					"replicas": {
						Type:        "integer",
						Description: "Desired replica count (for scale)",
					},
					"lines": {
						Type:        "integer",
						Description: "Number of log lines to return (for logs, default: 100)",
					},
					"status": {
						Type:        "string",
						Description: "Filter by pod phase: Running, Pending, Failed, Succeeded (for pods)",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results (default: 100)",
					},
					"offset": {
						Type:        "integer",
						Description: "Number of results to skip",
					},
				},
				Required: []string{"type"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeKubernetes(ctx, args)
		},
		Governance: ToolGovernance{
			ActionMode:     ToolActionRead,
			ApprovalPolicy: ToolApprovalScopeOnly,
			Summary:        "Reads Kubernetes topology and bounded logs; workload mutations are denied.",
		},
	})
}

// executeKubernetes routes to the appropriate kubernetes handler based on type
func (e *PulseToolExecutor) executeKubernetes(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceType, _ := args["type"].(string)
	switch resourceType {
	case "clusters":
		return e.executeGetKubernetesClusters(ctx)
	case "nodes":
		return e.executeGetKubernetesNodes(ctx, args)
	case "pods":
		return e.executeGetKubernetesPods(ctx, args)
	case "deployments":
		return e.executeGetKubernetesDeployments(ctx, args)
	case "logs":
		return e.executeKubernetesLogs(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("Kubernetes operation %q is unavailable; only clusters, nodes, pods, deployments, and logs are supported", resourceType)), nil
	}
}

func (e *PulseToolExecutor) kubernetesReadState() (unifiedresources.ReadState, error) {
	if rs := e.getCanonicalReadState(); rs != nil {
		return rs, nil
	}
	return nil, fmt.Errorf("read state not available")
}

func matchKubernetesCluster(cluster *unifiedresources.K8sClusterView, clusterArg string) bool {
	if cluster == nil {
		return false
	}
	return cluster.ID() == clusterArg ||
		cluster.ClusterID() == clusterArg ||
		cluster.Name() == clusterArg ||
		cluster.ClusterName() == clusterArg ||
		cluster.SourceName() == clusterArg
}

func newKubernetesClusterTarget(cluster *unifiedresources.K8sClusterView) *kubernetesClusterTarget {
	if cluster == nil {
		return nil
	}

	name := strings.TrimSpace(cluster.SourceName())
	if name == "" {
		name = strings.TrimSpace(cluster.Name())
	}
	displayName := strings.TrimSpace(cluster.ClusterName())
	if displayName == "" {
		displayName = name
	}
	if displayName == "" {
		displayName = strings.TrimSpace(cluster.ID())
	}
	clusterID := strings.TrimSpace(cluster.ClusterID())
	if clusterID == "" {
		clusterID = strings.TrimSpace(cluster.ID())
	}

	return &kubernetesClusterTarget{
		ID:          clusterID,
		AgentID:     cluster.AgentID(),
		Name:        name,
		DisplayName: displayName,
		Server:      cluster.Server(),
		Context:     cluster.Context(),
		Version:     cluster.Version(),
		Status:      string(cluster.Status()),
	}
}

func findKubernetesCluster(rs unifiedresources.ReadState, clusterArg string) *unifiedresources.K8sClusterView {
	for _, cluster := range rs.K8sClusters() {
		if matchKubernetesCluster(cluster, clusterArg) {
			return cluster
		}
	}
	return nil
}

func (e *PulseToolExecutor) executeGetKubernetesClusters(_ context.Context) (CallToolResult, error) {
	rs, err := e.kubernetesReadState()
	if err != nil {
		return NewErrorResult(err), nil
	}

	clusters := rs.K8sClusters()
	if len(clusters) == 0 {
		return NewTextResult("No Kubernetes clusters found. Kubernetes monitoring may not be configured."), nil
	}

	var summaries []KubernetesClusterSummary
	for _, c := range clusters {
		var clusterNodes []unifiedresources.K8sNodeView
		for _, node := range rs.K8sNodes() {
			if node.ParentID() == c.ID() {
				clusterNodes = append(clusterNodes, *node)
			}
		}

		readyNodes := 0
		for _, node := range clusterNodes {
			if node.Ready() {
				readyNodes++
			}
		}

		podCount := 0
		for _, pod := range rs.Pods() {
			if pod.ParentID() == c.ID() {
				podCount++
			}
		}

		depCount := 0
		for _, dep := range rs.K8sDeployments() {
			if dep.ParentID() == c.ID() {
				depCount++
			}
		}

		summaries = append(summaries, KubernetesClusterSummary{
			ID:              c.ID(),
			Name:            c.SourceName(),
			DisplayName:     c.ClusterName(),
			Server:          c.Server(),
			Version:         c.Version(),
			Status:          string(c.Status()),
			NodeCount:       len(clusterNodes),
			PodCount:        podCount,
			DeploymentCount: depCount,
			ReadyNodes:      readyNodes,
		})
	}

	response := EmptyKubernetesClustersResponse()
	response.Clusters = summaries
	response.Total = len(summaries)

	return NewJSONResult(response.NormalizeCollections()), nil
}

func (e *PulseToolExecutor) executeGetKubernetesNodes(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	clusterArg, _ := args["cluster"].(string)
	if clusterArg == "" {
		return NewErrorResult(fmt.Errorf("cluster is required")), nil
	}

	rs, err := e.kubernetesReadState()
	if err != nil {
		return NewErrorResult(err), nil
	}

	clusterView := findKubernetesCluster(rs, clusterArg)
	if clusterView == nil {
		return NewTextResult(fmt.Sprintf("Kubernetes cluster '%s' not found.", clusterArg)), nil
	}
	cluster := newKubernetesClusterTarget(clusterView)

	var nodes []KubernetesNodeSummary
	for _, node := range rs.K8sNodes() {
		if node.ParentID() != clusterView.ID() {
			continue
		}
		nodes = append(nodes, KubernetesNodeSummary{
			UID:                     node.NodeUID(),
			Name:                    node.NodeName(),
			Ready:                   node.Ready(),
			Unschedulable:           node.Unschedulable(),
			Roles:                   node.Roles(),
			KubeletVersion:          node.KubeletVersion(),
			ContainerRuntimeVersion: node.ContainerRuntimeVersion(),
			OSImage:                 node.OSImage(),
			Architecture:            node.Architecture(),
			CapacityCPU:             node.CapacityCPU(),
			CapacityMemoryBytes:     node.CapacityMemoryBytes(),
			CapacityPods:            node.CapacityPods(),
			AllocatableCPU:          node.AllocCPU(),
			AllocatableMemoryBytes:  node.AllocMemoryBytes(),
			AllocatablePods:         node.AllocPods(),
		})
	}

	response := EmptyKubernetesNodesResponse()
	response.Cluster = cluster.DisplayName
	response.Nodes = nodes
	response.Total = len(nodes)
	return NewJSONResult(response.NormalizeCollections()), nil
}

func (e *PulseToolExecutor) executeGetKubernetesPods(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	clusterArg, _ := args["cluster"].(string)
	if clusterArg == "" {
		return NewErrorResult(fmt.Errorf("cluster is required")), nil
	}

	namespaceFilter, _ := args["namespace"].(string)
	statusFilter, _ := args["status"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)

	rs, err := e.kubernetesReadState()
	if err != nil {
		return NewErrorResult(err), nil
	}

	clusterView := findKubernetesCluster(rs, clusterArg)
	if clusterView == nil {
		return NewTextResult(fmt.Sprintf("Kubernetes cluster '%s' not found.", clusterArg)), nil
	}
	cluster := newKubernetesClusterTarget(clusterView)

	var pods []KubernetesPodSummary
	totalClusterPods := 0
	filteredCount := 0

	for _, pod := range rs.Pods() {
		if pod.ParentID() != clusterView.ID() {
			continue
		}
		totalClusterPods++

		if namespaceFilter != "" && pod.Namespace() != namespaceFilter {
			continue
		}
		if statusFilter != "" && !strings.EqualFold(pod.PodPhase(), statusFilter) {
			continue
		}

		filteredCount++
	}

	currentFilteredIndex := 0
	addedCount := 0

	for _, pod := range rs.Pods() {
		if pod.ParentID() != clusterView.ID() {
			continue
		}

		if namespaceFilter != "" && pod.Namespace() != namespaceFilter {
			continue
		}
		if statusFilter != "" && !strings.EqualFold(pod.PodPhase(), statusFilter) {
			continue
		}

		if currentFilteredIndex < offset {
			currentFilteredIndex++
			continue
		}
		if addedCount >= limit {
			break
		}

		currentFilteredIndex++
		addedCount++

		var containers []KubernetesPodContainerSummary

		pods = append(pods, KubernetesPodSummary{
			UID:        pod.PodUID(),
			Name:       pod.Name(),
			Namespace:  pod.Namespace(),
			NodeName:   "",
			Phase:      pod.PodPhase(),
			Reason:     "",
			Restarts:   pod.Restarts(),
			QoSClass:   "",
			OwnerKind:  pod.OwnerKind(),
			OwnerName:  pod.OwnerName(),
			Containers: containers,
		})
	}

	response := EmptyKubernetesPodsResponse()
	response.Cluster = cluster.DisplayName
	response.Pods = pods
	response.Total = totalClusterPods
	response.Filtered = filteredCount
	return NewJSONResult(response.NormalizeCollections()), nil
}

func (e *PulseToolExecutor) executeGetKubernetesDeployments(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	clusterArg, _ := args["cluster"].(string)
	if clusterArg == "" {
		return NewErrorResult(fmt.Errorf("cluster is required")), nil
	}

	namespaceFilter, _ := args["namespace"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)

	rs, err := e.kubernetesReadState()
	if err != nil {
		return NewErrorResult(err), nil
	}

	clusterView := findKubernetesCluster(rs, clusterArg)
	if clusterView == nil {
		return NewTextResult(fmt.Sprintf("Kubernetes cluster '%s' not found.", clusterArg)), nil
	}
	cluster := newKubernetesClusterTarget(clusterView)

	var deployments []KubernetesDeploymentSummary

	totalClusterDeployments := 0
	for _, dep := range rs.K8sDeployments() {
		if dep.ParentID() == clusterView.ID() {
			totalClusterDeployments++
		}
	}

	filteredCount := 0
	currentFilteredIndex := 0

	for _, dep := range rs.K8sDeployments() {
		if dep.ParentID() != clusterView.ID() {
			continue
		}
		if namespaceFilter != "" && dep.Namespace() != namespaceFilter {
			continue
		}

		filteredCount++

		if currentFilteredIndex < offset {
			currentFilteredIndex++
			continue
		}
		if len(deployments) >= limit {
			break
		}

		currentFilteredIndex++

		deployments = append(deployments, KubernetesDeploymentSummary{
			UID:               dep.DeploymentUID(),
			Name:              dep.Name(),
			Namespace:         dep.Namespace(),
			DesiredReplicas:   dep.DesiredReplicas(),
			UpdatedReplicas:   dep.UpdatedReplicas(),
			ReadyReplicas:     dep.ReadyReplicas(),
			AvailableReplicas: dep.AvailableReplicas(),
		})
	}

	response := EmptyKubernetesDeploymentsResponse()
	response.Cluster = cluster.DisplayName
	response.Deployments = deployments
	response.Total = totalClusterDeployments
	response.Filtered = filteredCount
	return NewJSONResult(response.NormalizeCollections()), nil
}

// ========== Kubernetes Control Operations ==========

// findAgentForKubernetesCluster finds the agent for a Kubernetes cluster.
func (e *PulseToolExecutor) findAgentForKubernetesCluster(clusterArg string) (string, *kubernetesClusterTarget, error) {
	rs, err := e.kubernetesReadState()
	if err != nil {
		return "", nil, fmt.Errorf("state not available: %w", err)
	}
	cluster := newKubernetesClusterTarget(findKubernetesCluster(rs, clusterArg))
	if cluster == nil {
		return "", nil, fmt.Errorf("kubernetes cluster '%s' not found", clusterArg)
	}
	if cluster.AgentID == "" {
		return "", nil, fmt.Errorf("cluster '%s' has no agent configured - kubectl commands cannot be executed", clusterArg)
	}
	return cluster.AgentID, cluster, nil
}

// validateKubernetesResourceID validates a Kubernetes resource identifier (namespace, pod, deployment, container names)
func validateKubernetesResourceID(value string) error {
	if value == "" {
		return fmt.Errorf("value cannot be empty")
	}
	// Kubernetes resource names must be valid DNS subdomains: lowercase, alphanumeric, '-' and '.'
	// Max 253 characters
	if len(value) > 253 {
		return fmt.Errorf("value too long (max 253 characters)")
	}
	for _, c := range value {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '.') {
			return fmt.Errorf("invalid character '%c' in resource name", c)
		}
	}
	return nil
}

// executeKubernetesLogs retrieves pod logs
func (e *PulseToolExecutor) executeKubernetesLogs(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	clusterArg, _ := args["cluster"].(string)
	namespace, _ := args["namespace"].(string)
	pod, _ := args["pod"].(string)
	container, _ := args["container"].(string)
	lines := intArg(args, "lines", 100)

	if clusterArg == "" {
		return NewErrorResult(fmt.Errorf("cluster is required")), nil
	}
	if pod == "" {
		return NewErrorResult(fmt.Errorf("pod is required")), nil
	}
	if namespace == "" {
		namespace = "default"
	}

	// Validate identifiers
	if err := validateKubernetesResourceID(namespace); err != nil {
		return NewErrorResult(fmt.Errorf("invalid namespace: %w", err)), nil
	}
	if err := validateKubernetesResourceID(pod); err != nil {
		return NewErrorResult(fmt.Errorf("invalid pod: %w", err)), nil
	}
	if container != "" {
		if err := validateKubernetesResourceID(container); err != nil {
			return NewErrorResult(fmt.Errorf("invalid container: %w", err)), nil
		}
	}

	// Logs is a read operation, but still requires a connected agent
	agentID, _, err := e.findAgentForKubernetesCluster(clusterArg)
	if err != nil {
		return NewTextResult(err.Error()), nil
	}

	// Build kubectl command - logs is read-only so no approval needed
	var kubectlCmd string
	if container != "" {
		kubectlCmd = fmt.Sprintf("kubectl -n %s logs %s -c %s --tail=%d", shellEscape(namespace), shellEscape(pod), shellEscape(container), lines)
	} else {
		kubectlCmd = fmt.Sprintf("kubectl -n %s logs %s --tail=%d", shellEscape(namespace), shellEscape(pod), lines)
	}

	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	result, err := e.agentServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		Command:    kubectlCmd,
		TargetType: "agent",
		TargetID:   "",
	})
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to execute kubectl: %w", err)), nil
	}

	output := result.Stdout
	if result.Stderr != "" && result.ExitCode != 0 {
		output += "\n" + result.Stderr
	}

	if result.ExitCode == 0 {
		if output == "" {
			return NewTextResult(fmt.Sprintf("No logs found for pod '%s' in namespace '%s'", pod, namespace)), nil
		}
		return NewTextResult(fmt.Sprintf("Logs from pod '%s' (last %d lines):\n%s", pod, lines, output)), nil
	}

	return NewTextResult(fmt.Sprintf("kubectl logs failed (exit code %d):\n%s", result.ExitCode, output)), nil
}
