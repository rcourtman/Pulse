package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// registerKubernetesTools registers the pulse_kubernetes tool
func (e *PulseToolExecutor) registerKubernetesTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_kubernetes",
			Description: `Query and control Kubernetes clusters, nodes, pods, and deployments. Query: clusters, nodes, pods, deployments. Control: scale, restart, delete_pod, exec, logs.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"type": {
						Type:        "string",
						Description: "Operation type",
						Enum:        []string{"clusters", "nodes", "pods", "deployments", "scale", "restart", "delete_pod", "exec", "logs"},
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
	// Control operations
	case "scale":
		return e.executeKubernetesScale(ctx, args)
	case "restart":
		return e.executeKubernetesRestart(ctx, args)
	case "delete_pod":
		return e.executeKubernetesDeletePod(ctx, args)
	case "exec":
		return e.executeKubernetesExec(ctx, args)
	case "logs":
		return e.executeKubernetesLogs(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("unknown type: %s. Use: clusters, nodes, pods, deployments, scale, restart, delete_pod, exec, logs", resourceType)), nil
	}
}

func (e *PulseToolExecutor) executeGetKubernetesClusters(_ context.Context) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	state := e.stateProvider.GetState()

	if len(state.KubernetesClusters) == 0 {
		return NewTextResult("No Kubernetes clusters found. Kubernetes monitoring may not be configured."), nil
	}

	var clusters []KubernetesClusterSummary
	for _, c := range state.KubernetesClusters {
		readyNodes := 0
		for _, node := range c.Nodes {
			if node.Ready {
				readyNodes++
			}
		}

		displayName := c.DisplayName
		if c.CustomDisplayName != "" {
			displayName = c.CustomDisplayName
		}

		clusters = append(clusters, KubernetesClusterSummary{
			ID:              c.ID,
			Name:            c.Name,
			DisplayName:     displayName,
			Server:          c.Server,
			Version:         c.Version,
			Status:          c.Status,
			NodeCount:       len(c.Nodes),
			PodCount:        len(c.Pods),
			DeploymentCount: len(c.Deployments),
			ReadyNodes:      readyNodes,
		})
	}

	response := KubernetesClustersResponse{
		Clusters: clusters,
		Total:    len(clusters),
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeGetKubernetesNodes(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	clusterArg, _ := args["cluster"].(string)
	if clusterArg == "" {
		return NewErrorResult(fmt.Errorf("cluster is required")), nil
	}

	state := e.stateProvider.GetState()

	// Find the cluster (also match CustomDisplayName)
	var cluster *KubernetesClusterSummary
	for _, c := range state.KubernetesClusters {
		if c.ID == clusterArg || c.Name == clusterArg || c.DisplayName == clusterArg || c.CustomDisplayName == clusterArg {
			displayName := c.DisplayName
			if c.CustomDisplayName != "" {
				displayName = c.CustomDisplayName
			}
			cluster = &KubernetesClusterSummary{
				ID:          c.ID,
				Name:        c.Name,
				DisplayName: displayName,
			}

			var nodes []KubernetesNodeSummary
			for _, node := range c.Nodes {
				nodes = append(nodes, KubernetesNodeSummary{
					UID:                     node.UID,
					Name:                    node.Name,
					Ready:                   node.Ready,
					Unschedulable:           node.Unschedulable,
					Roles:                   node.Roles,
					KubeletVersion:          node.KubeletVersion,
					ContainerRuntimeVersion: node.ContainerRuntimeVersion,
					OSImage:                 node.OSImage,
					Architecture:            node.Architecture,
					CapacityCPU:             node.CapacityCPU,
					CapacityMemoryBytes:     node.CapacityMemoryBytes,
					CapacityPods:            node.CapacityPods,
					AllocatableCPU:          node.AllocCPU,
					AllocatableMemoryBytes:  node.AllocMemoryBytes,
					AllocatablePods:         node.AllocPods,
				})
			}

			response := KubernetesNodesResponse{
				Cluster: cluster.DisplayName,
				Nodes:   nodes,
				Total:   len(nodes),
			}
			if response.Nodes == nil {
				response.Nodes = []KubernetesNodeSummary{}
			}
			return NewJSONResult(response), nil
		}
	}

	return NewTextResult(fmt.Sprintf("Kubernetes cluster '%s' not found.", clusterArg)), nil
}

func (e *PulseToolExecutor) executeGetKubernetesPods(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	clusterArg, _ := args["cluster"].(string)
	if clusterArg == "" {
		return NewErrorResult(fmt.Errorf("cluster is required")), nil
	}

	namespaceFilter, _ := args["namespace"].(string)
	statusFilter, _ := args["status"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)

	state := e.stateProvider.GetState()

	// Find the cluster (also match CustomDisplayName)
	for _, c := range state.KubernetesClusters {
		if c.ID == clusterArg || c.Name == clusterArg || c.DisplayName == clusterArg || c.CustomDisplayName == clusterArg {
			displayName := c.DisplayName
			if c.CustomDisplayName != "" {
				displayName = c.CustomDisplayName
			}

			var pods []KubernetesPodSummary
			totalPods := 0
			filteredCount := 0

			for _, pod := range c.Pods {
				// Apply filters
				if namespaceFilter != "" && pod.Namespace != namespaceFilter {
					continue
				}
				if statusFilter != "" && !strings.EqualFold(pod.Phase, statusFilter) {
					continue
				}

				filteredCount++

				// Apply pagination
				if totalPods < offset {
					totalPods++
					continue
				}
				if len(pods) >= limit {
					totalPods++
					continue
				}

				var containers []KubernetesPodContainerSummary
				for _, container := range pod.Containers {
					containers = append(containers, KubernetesPodContainerSummary{
						Name:         container.Name,
						Ready:        container.Ready,
						State:        container.State,
						RestartCount: container.RestartCount,
						Reason:       container.Reason,
					})
				}

				pods = append(pods, KubernetesPodSummary{
					UID:        pod.UID,
					Name:       pod.Name,
					Namespace:  pod.Namespace,
					NodeName:   pod.NodeName,
					Phase:      pod.Phase,
					Reason:     pod.Reason,
					Restarts:   pod.Restarts,
					QoSClass:   pod.QoSClass,
					OwnerKind:  pod.OwnerKind,
					OwnerName:  pod.OwnerName,
					Containers: containers,
				})
				totalPods++
			}

			response := KubernetesPodsResponse{
				Cluster:  displayName,
				Pods:     pods,
				Total:    len(c.Pods),
				Filtered: filteredCount,
			}
			if response.Pods == nil {
				response.Pods = []KubernetesPodSummary{}
			}
			return NewJSONResult(response), nil
		}
	}

	return NewTextResult(fmt.Sprintf("Kubernetes cluster '%s' not found.", clusterArg)), nil
}

func (e *PulseToolExecutor) executeGetKubernetesDeployments(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	clusterArg, _ := args["cluster"].(string)
	if clusterArg == "" {
		return NewErrorResult(fmt.Errorf("cluster is required")), nil
	}

	namespaceFilter, _ := args["namespace"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)

	state := e.stateProvider.GetState()

	// Find the cluster (also match CustomDisplayName)
	for _, c := range state.KubernetesClusters {
		if c.ID == clusterArg || c.Name == clusterArg || c.DisplayName == clusterArg || c.CustomDisplayName == clusterArg {
			displayName := c.DisplayName
			if c.CustomDisplayName != "" {
				displayName = c.CustomDisplayName
			}

			var deployments []KubernetesDeploymentSummary
			filteredCount := 0
			count := 0

			for _, dep := range c.Deployments {
				// Apply namespace filter
				if namespaceFilter != "" && dep.Namespace != namespaceFilter {
					continue
				}

				filteredCount++

				// Apply pagination
				if count < offset {
					count++
					continue
				}
				if len(deployments) >= limit {
					count++
					continue
				}

				deployments = append(deployments, KubernetesDeploymentSummary{
					UID:               dep.UID,
					Name:              dep.Name,
					Namespace:         dep.Namespace,
					DesiredReplicas:   dep.DesiredReplicas,
					ReadyReplicas:     dep.ReadyReplicas,
					AvailableReplicas: dep.AvailableReplicas,
					UpdatedReplicas:   dep.UpdatedReplicas,
				})
				count++
			}

			response := KubernetesDeploymentsResponse{
				Cluster:     displayName,
				Deployments: deployments,
				Total:       len(c.Deployments),
				Filtered:    filteredCount,
			}
			if response.Deployments == nil {
				response.Deployments = []KubernetesDeploymentSummary{}
			}
			return NewJSONResult(response), nil
		}
	}

	return NewTextResult(fmt.Sprintf("Kubernetes cluster '%s' not found.", clusterArg)), nil
}

// ========== Kubernetes Control Operations ==========

// findAgentForKubernetesCluster finds the agent for a Kubernetes cluster
func (e *PulseToolExecutor) findAgentForKubernetesCluster(clusterArg string) (string, *models.KubernetesCluster, error) {
	rs, err := e.readStateForControl()
	if err != nil {
		return "", nil, fmt.Errorf("state not available: %w", err)
	}
	for _, c := range rs.K8sClusters() {
		if c.ID() == clusterArg || c.ClusterID() == clusterArg || c.Name() == clusterArg || c.ClusterName() == clusterArg || c.SourceName() == clusterArg {
			agentID := c.AgentID()
			if agentID == "" {
				return "", nil, fmt.Errorf("cluster '%s' has no agent configured - kubectl commands cannot be executed", clusterArg)
			}

			// Try to return the real models.KubernetesCluster when available (richer display name fields).
			if e.stateProvider != nil {
				state := e.stateProvider.GetState()
				for i := range state.KubernetesClusters {
					sc := &state.KubernetesClusters[i]
					if sc.ID == clusterArg || sc.Name == clusterArg || sc.DisplayName == clusterArg || sc.CustomDisplayName == clusterArg {
						return agentID, sc, nil
					}
					// Also match by agent ID + server/context when the arg was an ID-like value.
					if sc.AgentID == agentID && (sc.ID == c.ClusterID() || sc.Name == c.ClusterName()) {
						return agentID, sc, nil
					}
				}
			}

			// Fallback: synthesize a minimal cluster struct for approval labels.
			display := c.ClusterName()
			if display == "" {
				display = c.Name()
			}
			if display == "" {
				display = c.ID()
			}
			return agentID, &models.KubernetesCluster{
				ID:          c.ID(),
				AgentID:     agentID,
				Name:        c.ClusterName(),
				DisplayName: display,
				Server:      c.Server(),
				Context:     c.Context(),
				Version:     c.Version(),
				Status:      string(c.Status()),
			}, nil
		}
	}

	return "", nil, fmt.Errorf("kubernetes cluster '%s' not found", clusterArg)
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

// buildKubectlExecCommand builds a kubectl exec command safely.
// It runs the command via "sh -c" inside the pod and shell-escapes all
// user-controlled values to prevent host shell injection.
func buildKubectlExecCommand(namespace, pod, container, command string) string {
	base := fmt.Sprintf("kubectl -n %s exec %s", shellEscape(namespace), shellEscape(pod))
	if container != "" {
		base += fmt.Sprintf(" -c %s", shellEscape(container))
	}
	return base + fmt.Sprintf(" -- sh -c %s", shellEscape(command))
}

// executeKubernetesScale scales a deployment
func (e *PulseToolExecutor) executeKubernetesScale(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	clusterArg, _ := args["cluster"].(string)
	namespace, _ := args["namespace"].(string)
	deployment, _ := args["deployment"].(string)
	replicas := intArg(args, "replicas", -1)

	if clusterArg == "" {
		return NewErrorResult(fmt.Errorf("cluster is required")), nil
	}
	if deployment == "" {
		return NewErrorResult(fmt.Errorf("deployment is required")), nil
	}
	if replicas < 0 {
		return NewErrorResult(fmt.Errorf("replicas is required and must be >= 0")), nil
	}
	if namespace == "" {
		namespace = "default"
	}

	// Validate identifiers
	if err := validateKubernetesResourceID(namespace); err != nil {
		return NewErrorResult(fmt.Errorf("invalid namespace: %w", err)), nil
	}
	if err := validateKubernetesResourceID(deployment); err != nil {
		return NewErrorResult(fmt.Errorf("invalid deployment: %w", err)), nil
	}

	// Check control level
	if e.controlLevel == ControlLevelReadOnly {
		return NewTextResult("Kubernetes control operations are not available in read-only mode."), nil
	}

	agentID, cluster, err := e.findAgentForKubernetesCluster(clusterArg)
	if err != nil {
		return NewTextResult(err.Error()), nil
	}

	// Build command
	command := fmt.Sprintf("kubectl -n %s scale deployment %s --replicas=%d", namespace, deployment, replicas)
	clusterScope := cluster.ID
	if clusterScope == "" {
		clusterScope = clusterArg
	}
	approvalTargetID := fmt.Sprintf("%s:%s:deployment:%s", clusterScope, namespace, deployment)

	// Check if pre-approved (validated + single-use).
	preApproved := consumeApprovalWithValidation(args, command, "kubernetes", approvalTargetID)

	// Request approval if needed
	if !preApproved && !e.isAutonomous && e.controlLevel == ControlLevelControlled {
		displayName := cluster.DisplayName
		if cluster.CustomDisplayName != "" {
			displayName = cluster.CustomDisplayName
		}
		approvalID := createApprovalRecord(command, "kubernetes", approvalTargetID, displayName, fmt.Sprintf("Scale deployment %s to %d replicas", deployment, replicas))
		return NewTextResult(formatKubernetesApprovalNeeded("scale", deployment, namespace, displayName, command, approvalID)), nil
	}

	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	result, err := e.agentServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		Command:    command,
		TargetType: "host",
		TargetID:   "",
	})
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to execute kubectl: %w", err)), nil
	}

	output := result.Stdout
	if result.Stderr != "" {
		output += "\n" + result.Stderr
	}

	if result.ExitCode == 0 {
		return NewTextResult(fmt.Sprintf("✓ Successfully scaled deployment '%s' to %d replicas in namespace '%s'. Action complete - no verification needed.\n%s", deployment, replicas, namespace, output)), nil
	}

	return NewTextResult(fmt.Sprintf("kubectl command failed (exit code %d):\n%s", result.ExitCode, output)), nil
}

// executeKubernetesRestart restarts a deployment via rollout restart
func (e *PulseToolExecutor) executeKubernetesRestart(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	clusterArg, _ := args["cluster"].(string)
	namespace, _ := args["namespace"].(string)
	deployment, _ := args["deployment"].(string)

	if clusterArg == "" {
		return NewErrorResult(fmt.Errorf("cluster is required")), nil
	}
	if deployment == "" {
		return NewErrorResult(fmt.Errorf("deployment is required")), nil
	}
	if namespace == "" {
		namespace = "default"
	}

	// Validate identifiers
	if err := validateKubernetesResourceID(namespace); err != nil {
		return NewErrorResult(fmt.Errorf("invalid namespace: %w", err)), nil
	}
	if err := validateKubernetesResourceID(deployment); err != nil {
		return NewErrorResult(fmt.Errorf("invalid deployment: %w", err)), nil
	}

	// Check control level
	if e.controlLevel == ControlLevelReadOnly {
		return NewTextResult("Kubernetes control operations are not available in read-only mode."), nil
	}

	agentID, cluster, err := e.findAgentForKubernetesCluster(clusterArg)
	if err != nil {
		return NewTextResult(err.Error()), nil
	}

	// Build command
	command := fmt.Sprintf("kubectl -n %s rollout restart deployment/%s", namespace, deployment)
	clusterScope := cluster.ID
	if clusterScope == "" {
		clusterScope = clusterArg
	}
	approvalTargetID := fmt.Sprintf("%s:%s:deployment:%s", clusterScope, namespace, deployment)

	// Check if pre-approved (validated + single-use).
	preApproved := consumeApprovalWithValidation(args, command, "kubernetes", approvalTargetID)

	// Request approval if needed
	if !preApproved && !e.isAutonomous && e.controlLevel == ControlLevelControlled {
		displayName := cluster.DisplayName
		if cluster.CustomDisplayName != "" {
			displayName = cluster.CustomDisplayName
		}
		approvalID := createApprovalRecord(command, "kubernetes", approvalTargetID, displayName, fmt.Sprintf("Restart deployment %s", deployment))
		return NewTextResult(formatKubernetesApprovalNeeded("restart", deployment, namespace, displayName, command, approvalID)), nil
	}

	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	result, err := e.agentServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		Command:    command,
		TargetType: "host",
		TargetID:   "",
	})
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to execute kubectl: %w", err)), nil
	}

	output := result.Stdout
	if result.Stderr != "" {
		output += "\n" + result.Stderr
	}

	if result.ExitCode == 0 {
		return NewTextResult(fmt.Sprintf("✓ Successfully initiated rollout restart for deployment '%s' in namespace '%s'. Action complete - pods will restart gradually.\n%s", deployment, namespace, output)), nil
	}

	return NewTextResult(fmt.Sprintf("kubectl command failed (exit code %d):\n%s", result.ExitCode, output)), nil
}

// executeKubernetesDeletePod deletes a pod
func (e *PulseToolExecutor) executeKubernetesDeletePod(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	clusterArg, _ := args["cluster"].(string)
	namespace, _ := args["namespace"].(string)
	pod, _ := args["pod"].(string)

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

	// Check control level
	if e.controlLevel == ControlLevelReadOnly {
		return NewTextResult("Kubernetes control operations are not available in read-only mode."), nil
	}

	agentID, cluster, err := e.findAgentForKubernetesCluster(clusterArg)
	if err != nil {
		return NewTextResult(err.Error()), nil
	}

	// Build command
	command := fmt.Sprintf("kubectl -n %s delete pod %s", namespace, pod)
	clusterScope := cluster.ID
	if clusterScope == "" {
		clusterScope = clusterArg
	}
	approvalTargetID := fmt.Sprintf("%s:%s:pod:%s", clusterScope, namespace, pod)

	// Check if pre-approved (validated + single-use).
	preApproved := consumeApprovalWithValidation(args, command, "kubernetes", approvalTargetID)

	// Request approval if needed
	if !preApproved && !e.isAutonomous && e.controlLevel == ControlLevelControlled {
		displayName := cluster.DisplayName
		if cluster.CustomDisplayName != "" {
			displayName = cluster.CustomDisplayName
		}
		approvalID := createApprovalRecord(command, "kubernetes", approvalTargetID, displayName, fmt.Sprintf("Delete pod %s", pod))
		return NewTextResult(formatKubernetesApprovalNeeded("delete_pod", pod, namespace, displayName, command, approvalID)), nil
	}

	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	result, err := e.agentServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		Command:    command,
		TargetType: "host",
		TargetID:   "",
	})
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to execute kubectl: %w", err)), nil
	}

	output := result.Stdout
	if result.Stderr != "" {
		output += "\n" + result.Stderr
	}

	if result.ExitCode == 0 {
		return NewTextResult(fmt.Sprintf("✓ Successfully deleted pod '%s' in namespace '%s'. If managed by a controller, a new pod will be created.\n%s", pod, namespace, output)), nil
	}

	return NewTextResult(fmt.Sprintf("kubectl command failed (exit code %d):\n%s", result.ExitCode, output)), nil
}

// executeKubernetesExec executes a command inside a pod
func (e *PulseToolExecutor) executeKubernetesExec(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	clusterArg, _ := args["cluster"].(string)
	namespace, _ := args["namespace"].(string)
	pod, _ := args["pod"].(string)
	container, _ := args["container"].(string)
	command, _ := args["command"].(string)

	if clusterArg == "" {
		return NewErrorResult(fmt.Errorf("cluster is required")), nil
	}
	if pod == "" {
		return NewErrorResult(fmt.Errorf("pod is required")), nil
	}
	if command == "" {
		return NewErrorResult(fmt.Errorf("command is required")), nil
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

	// Check control level
	if e.controlLevel == ControlLevelReadOnly {
		return NewTextResult("Kubernetes control operations are not available in read-only mode."), nil
	}

	agentID, cluster, err := e.findAgentForKubernetesCluster(clusterArg)
	if err != nil {
		return NewTextResult(err.Error()), nil
	}

	// Build kubectl command safely to prevent shell metacharacter breakout on the host.
	kubectlCmd := buildKubectlExecCommand(namespace, pod, container, command)
	clusterScope := cluster.ID
	if clusterScope == "" {
		clusterScope = clusterArg
	}
	approvalTargetID := fmt.Sprintf("%s:%s:pod:%s", clusterScope, namespace, pod)

	// Check if pre-approved (validated + single-use).
	preApproved := consumeApprovalWithValidation(args, kubectlCmd, "kubernetes", approvalTargetID)

	// Request approval if needed
	if !preApproved && !e.isAutonomous && e.controlLevel == ControlLevelControlled {
		displayName := cluster.DisplayName
		if cluster.CustomDisplayName != "" {
			displayName = cluster.CustomDisplayName
		}
		approvalID := createApprovalRecord(kubectlCmd, "kubernetes", approvalTargetID, displayName, fmt.Sprintf("Execute command in pod %s", pod))
		return NewTextResult(formatKubernetesApprovalNeeded("exec", pod, namespace, displayName, kubectlCmd, approvalID)), nil
	}

	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	result, err := e.agentServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		Command:    kubectlCmd,
		TargetType: "host",
		TargetID:   "",
	})
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to execute kubectl: %w", err)), nil
	}

	output := result.Stdout
	if result.Stderr != "" {
		output += "\n" + result.Stderr
	}

	// Always show output explicitly to prevent LLM hallucination
	if result.ExitCode == 0 {
		if output == "" {
			return NewTextResult(fmt.Sprintf("Command executed in pod '%s' (exit code 0).\n\nOutput:\n(no output)", pod)), nil
		}
		return NewTextResult(fmt.Sprintf("Command executed in pod '%s' (exit code 0).\n\nOutput:\n%s", pod, output)), nil
	}

	if output == "" {
		return NewTextResult(fmt.Sprintf("Command in pod '%s' exited with code %d.\n\nOutput:\n(no output)", pod, result.ExitCode)), nil
	}
	return NewTextResult(fmt.Sprintf("Command in pod '%s' exited with code %d.\n\nOutput:\n%s", pod, result.ExitCode, output)), nil
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
		kubectlCmd = fmt.Sprintf("kubectl -n %s logs %s -c %s --tail=%d", namespace, pod, container, lines)
	} else {
		kubectlCmd = fmt.Sprintf("kubectl -n %s logs %s --tail=%d", namespace, pod, lines)
	}

	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	result, err := e.agentServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		Command:    kubectlCmd,
		TargetType: "host",
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

// formatKubernetesApprovalNeeded formats an approval-required response for Kubernetes operations
func formatKubernetesApprovalNeeded(action, resource, namespace, cluster, command, approvalID string) string {
	payload := map[string]interface{}{
		"type":           "approval_required",
		"approval_id":    approvalID,
		"action":         action,
		"resource":       resource,
		"namespace":      namespace,
		"cluster":        cluster,
		"command":        command,
		"how_to_approve": "Click the approval button in the chat to execute this action.",
		"do_not_retry":   true,
	}
	b, _ := json.Marshal(payload)
	return "APPROVAL_REQUIRED: " + string(b)
}
