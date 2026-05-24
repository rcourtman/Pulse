package dockeragent

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	swarmtypes "github.com/moby/moby/api/types/swarm"
	systemtypes "github.com/moby/moby/api/types/system"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

const (
	swarmScopeAuto    = "auto"
	swarmScopeNode    = "node"
	swarmScopeCluster = "cluster"
)

func normalizeSwarmScope(value string) (string, error) {
	scope := strings.ToLower(strings.TrimSpace(value))
	if scope == "" {
		return swarmScopeNode, nil
	}

	switch scope {
	case swarmScopeNode, swarmScopeCluster, swarmScopeAuto:
		return scope, nil
	default:
		return "", fmt.Errorf("invalid swarm scope %q: must be node, cluster, or auto", value)
	}
}

func (a *Agent) resolvedSwarmScope(info systemtypes.Info) string {
	switch a.cfg.SwarmScope {
	case swarmScopeAuto:
		if info.Swarm.ControlAvailable {
			return swarmScopeCluster
		}
		return swarmScopeNode
	case swarmScopeCluster, swarmScopeNode:
		return a.cfg.SwarmScope
	default:
		return swarmScopeNode
	}
}

func hasReportableSwarmInfo(info systemtypes.Info) bool {
	swarm := info.Swarm
	state := strings.ToLower(strings.TrimSpace(string(swarm.LocalNodeState)))
	if state == string(swarmtypes.LocalNodeStateInactive) {
		return swarm.ControlAvailable ||
			strings.TrimSpace(swarm.Error) != "" ||
			(swarm.Cluster != nil &&
				(strings.TrimSpace(swarm.Cluster.ID) != "" ||
					strings.TrimSpace(swarm.Cluster.Spec.Annotations.Name) != ""))
	}

	return swarm.NodeID != "" ||
		state != "" ||
		swarm.ControlAvailable ||
		strings.TrimSpace(swarm.Error) != "" ||
		(swarm.Cluster != nil &&
			(strings.TrimSpace(swarm.Cluster.ID) != "" ||
				strings.TrimSpace(swarm.Cluster.Spec.Annotations.Name) != ""))
}

func (a *Agent) collectSwarmData(ctx context.Context, info systemtypes.Info, containers []agentsdocker.Container) ([]agentsdocker.Service, []agentsdocker.Task, []agentsdocker.Node, []agentsdocker.Secret, []agentsdocker.Config, *agentsdocker.SwarmInfo) {
	if !a.supportsSwarm {
		return nil, nil, nil, nil, nil, nil
	}

	if !hasReportableSwarmInfo(info) {
		return nil, nil, nil, nil, nil, nil
	}

	scope := a.resolvedSwarmScope(info)
	effectiveScope := scope

	nodeRole := "worker"
	if info.Swarm.ControlAvailable {
		nodeRole = "manager"
	}

	swarmInfo := &agentsdocker.SwarmInfo{
		NodeID:           info.Swarm.NodeID,
		NodeRole:         nodeRole,
		LocalState:       string(info.Swarm.LocalNodeState),
		ControlAvailable: info.Swarm.ControlAvailable,
		Error:            strings.TrimSpace(info.Swarm.Error),
		Scope:            scope,
	}

	if info.Swarm.Cluster != nil {
		swarmInfo.ClusterID = info.Swarm.Cluster.ID
		swarmInfo.ClusterName = info.Swarm.Cluster.Spec.Annotations.Name
	}

	includeServices := a.cfg.IncludeServices
	includeTasks := a.cfg.IncludeTasks

	if info.Swarm.LocalNodeState != swarmtypes.LocalNodeStateActive {
		return nil, nil, nil, nil, nil, swarmInfo
	}

	var services []agentsdocker.Service
	var tasks []agentsdocker.Task
	var nodes []agentsdocker.Node
	var secrets []agentsdocker.Secret
	var configs []agentsdocker.Config

	if info.Swarm.ControlAvailable {
		managerNodes, err := a.collectSwarmNodes(ctx)
		if err != nil {
			a.logger.Warn().Err(err).Msg("failed to collect swarm nodes from manager")
		} else {
			nodes = managerNodes
		}
		managerSecrets, err := a.collectSwarmSecrets(ctx)
		if err != nil {
			a.logger.Warn().Err(err).Msg("failed to collect swarm secrets from manager")
		} else {
			secrets = managerSecrets
		}
		managerConfigs, err := a.collectSwarmConfigs(ctx)
		if err != nil {
			a.logger.Warn().Err(err).Msg("failed to collect swarm configs from manager")
		} else {
			configs = managerConfigs
		}
	}
	if len(nodes) == 0 {
		nodes = deriveLocalSwarmNode(info, firstNonEmptyString(a.hostName, info.Name))
	}

	containerIndex := buildContainerIndex(containers)

	if info.Swarm.ControlAvailable && (includeServices || includeTasks) {
		managerServices, managerTasks, err := a.collectSwarmDataFromManager(ctx, info, scope, containerIndex, includeServices, includeTasks)
		if err != nil {
			a.logger.Warn().Err(err).Msg("failed to collect swarm data from manager; falling back to local inference")
		} else {
			if includeServices {
				services = managerServices
			}
			if includeTasks {
				tasks = managerTasks
			}
		}
	}

	if includeTasks && len(tasks) == 0 {
		tasks = deriveSwarmTasksFromContainers(containers, info)
		if len(tasks) > 0 {
			effectiveScope = swarmScopeNode
		}
	}

	if includeServices && len(services) == 0 {
		services = deriveSwarmServicesFromData(tasks, containers)
		if len(services) > 0 {
			effectiveScope = swarmScopeNode
		}
	}

	if includeTasks && len(tasks) > 0 {
		sort.Slice(tasks, func(i, j int) bool {
			if tasks[i].ServiceName == tasks[j].ServiceName {
				if tasks[i].Slot == tasks[j].Slot {
					return tasks[i].ID < tasks[j].ID
				}
				return tasks[i].Slot < tasks[j].Slot
			}
			return tasks[i].ServiceName < tasks[j].ServiceName
		})
	}

	if includeServices && len(services) > 0 {
		sort.Slice(services, func(i, j int) bool {
			if services[i].Name == services[j].Name {
				return services[i].ID < services[j].ID
			}
			return services[i].Name < services[j].Name
		})
	}

	if len(nodes) > 0 {
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].Hostname == nodes[j].Hostname {
				return nodes[i].ID < nodes[j].ID
			}
			return nodes[i].Hostname < nodes[j].Hostname
		})
	}
	if len(secrets) > 0 {
		sort.Slice(secrets, func(i, j int) bool {
			if secrets[i].Name == secrets[j].Name {
				return secrets[i].ID < secrets[j].ID
			}
			return secrets[i].Name < secrets[j].Name
		})
	}
	if len(configs) > 0 {
		sort.Slice(configs, func(i, j int) bool {
			if configs[i].Name == configs[j].Name {
				return configs[i].ID < configs[j].ID
			}
			return configs[i].Name < configs[j].Name
		})
	}

	swarmInfo.Scope = effectiveScope

	if !includeServices {
		services = nil
	}
	if !includeTasks {
		tasks = nil
	}

	return services, tasks, nodes, secrets, configs, swarmInfo
}

func (a *Agent) collectSwarmNodes(ctx context.Context) ([]agentsdocker.Node, error) {
	if a == nil || a.docker == nil {
		return nil, nil
	}

	nodeList, err := dockerCallWithRetry(ctx, dockerSwarmListCallTimeout, func(callCtx context.Context) ([]swarmtypes.Node, error) {
		return a.docker.NodeList(callCtx, dockerNodeListOptions{})
	})
	if err != nil {
		return nil, annotateDockerConnectionError(err)
	}

	nodes := make([]agentsdocker.Node, 0, len(nodeList))
	for i := range nodeList {
		node := mapSwarmNode(&nodeList[i])
		if strings.TrimSpace(node.ID) == "" && strings.TrimSpace(node.Hostname) == "" {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (a *Agent) collectSwarmSecrets(ctx context.Context) ([]agentsdocker.Secret, error) {
	if a == nil || a.docker == nil {
		return nil, nil
	}

	secretList, err := dockerCallWithRetry(ctx, dockerSwarmListCallTimeout, func(callCtx context.Context) ([]swarmtypes.Secret, error) {
		return a.docker.SecretList(callCtx, dockerSecretListOptions{})
	})
	if err != nil {
		return nil, annotateDockerConnectionError(err)
	}

	secrets := make([]agentsdocker.Secret, 0, len(secretList))
	for i := range secretList {
		secret := mapSwarmSecret(&secretList[i])
		if strings.TrimSpace(secret.ID) == "" && strings.TrimSpace(secret.Name) == "" {
			continue
		}
		secrets = append(secrets, secret)
	}
	return secrets, nil
}

func (a *Agent) collectSwarmConfigs(ctx context.Context) ([]agentsdocker.Config, error) {
	if a == nil || a.docker == nil {
		return nil, nil
	}

	configList, err := dockerCallWithRetry(ctx, dockerSwarmListCallTimeout, func(callCtx context.Context) ([]swarmtypes.Config, error) {
		return a.docker.ConfigList(callCtx, dockerConfigListOptions{})
	})
	if err != nil {
		return nil, annotateDockerConnectionError(err)
	}

	configs := make([]agentsdocker.Config, 0, len(configList))
	for i := range configList {
		config := mapSwarmConfig(&configList[i])
		if strings.TrimSpace(config.ID) == "" && strings.TrimSpace(config.Name) == "" {
			continue
		}
		configs = append(configs, config)
	}
	return configs, nil
}

func (a *Agent) collectSwarmDataFromManager(ctx context.Context, info systemtypes.Info, scope string, containers map[string]agentsdocker.Container, includeServices, includeTasks bool) ([]agentsdocker.Service, []agentsdocker.Task, error) {
	serviceList, err := dockerCallWithRetry(ctx, dockerSwarmListCallTimeout, func(callCtx context.Context) ([]swarmtypes.Service, error) {
		return a.docker.ServiceList(callCtx, dockerServiceListOptions{Status: true})
	})
	if err != nil {
		return nil, nil, annotateDockerConnectionError(err)
	}

	servicePointers := make(map[string]*swarmtypes.Service, len(serviceList))
	for i := range serviceList {
		servicePointers[serviceList[i].ID] = &serviceList[i]
	}

	var services []agentsdocker.Service
	if includeServices {
		services = make([]agentsdocker.Service, 0, len(serviceList))
		for i := range serviceList {
			services = append(services, mapSwarmService(&serviceList[i]))
		}
	}

	var tasks []agentsdocker.Task
	if includeTasks {
		taskFilters := newDockerFilters()
		taskFilters.Add("desired-state", string(swarmtypes.TaskStateRunning))
		if scope != swarmScopeCluster && info.Swarm.NodeID != "" {
			taskFilters.Add("node", info.Swarm.NodeID)
		}

		taskList, err := dockerCallWithRetry(ctx, dockerSwarmListCallTimeout, func(callCtx context.Context) ([]swarmtypes.Task, error) {
			return a.docker.TaskList(callCtx, dockerTaskListOptions{Filters: taskFilters})
		})
		if err != nil {
			return services, nil, annotateDockerConnectionError(err)
		}

		tasks = make([]agentsdocker.Task, 0, len(taskList))
		for i := range taskList {
			if !isRuntimeSwarmTask(&taskList[i]) {
				continue
			}
			var svc *swarmtypes.Service
			if ptr, ok := servicePointers[taskList[i].ServiceID]; ok {
				svc = ptr
			}
			tasks = append(tasks, mapSwarmTask(&taskList[i], svc, containers))
		}

		if scope == swarmScopeNode && includeServices && len(services) > 0 {
			used := make(map[string]struct{}, len(tasks))
			for _, task := range tasks {
				// Only count running tasks - ignore shutdown/historical tasks
				if task.ServiceID != "" && strings.ToLower(task.DesiredState) == "running" {
					used[task.ServiceID] = struct{}{}
				}
			}

			filtered := services[:0]
			for _, svc := range services {
				if _, ok := used[svc.ID]; ok || len(used) == 0 {
					filtered = append(filtered, svc)
				}
			}
			services = filtered
		}
	}

	return services, tasks, nil
}

func isRuntimeSwarmTask(task *swarmtypes.Task) bool {
	if task == nil {
		return false
	}
	if task.DesiredState == swarmtypes.TaskStateRunning {
		return true
	}

	// Defensive fallback in case the daemon returns an empty desired state for an
	// otherwise active task. Terminal tasks should never be retained in runtime state.
	switch task.Status.State {
	case swarmtypes.TaskStateNew,
		swarmtypes.TaskStateAllocated,
		swarmtypes.TaskStatePending,
		swarmtypes.TaskStateAssigned,
		swarmtypes.TaskStateAccepted,
		swarmtypes.TaskStatePreparing,
		swarmtypes.TaskStateReady,
		swarmtypes.TaskStateStarting,
		swarmtypes.TaskStateRunning:
		return task.DesiredState == ""
	default:
		return false
	}
}

func mapSwarmService(svc *swarmtypes.Service) agentsdocker.Service {
	service := agentsdocker.Service{
		ID:   svc.ID,
		Name: svc.Spec.Annotations.Name,
		Mode: serviceMode(svc.Spec.Mode),
	}

	if svc.Spec.TaskTemplate.ContainerSpec != nil {
		service.Image = svc.Spec.TaskTemplate.ContainerSpec.Image
	}

	if svc.Spec.Annotations.Labels != nil {
		service.Labels = copyStringMap(svc.Spec.Annotations.Labels)
		if stack, ok := svc.Spec.Annotations.Labels["com.docker.stack.namespace"]; ok {
			service.Stack = stack
		}
	}

	if svc.ServiceStatus != nil {
		service.DesiredTasks = int(svc.ServiceStatus.DesiredTasks)
		service.RunningTasks = int(svc.ServiceStatus.RunningTasks)
		service.CompletedTasks = int(svc.ServiceStatus.CompletedTasks)
	}

	if svc.UpdateStatus != nil {
		update := &agentsdocker.ServiceUpdate{
			State:   string(svc.UpdateStatus.State),
			Message: svc.UpdateStatus.Message,
		}
		if svc.UpdateStatus.CompletedAt != nil {
			completed := *svc.UpdateStatus.CompletedAt
			if !completed.IsZero() {
				update.CompletedAt = &completed
			}
		}
		service.UpdateStatus = update
	}

	if len(svc.Endpoint.Ports) > 0 {
		service.EndpointPorts = make([]agentsdocker.ServicePort, len(svc.Endpoint.Ports))
		for i, port := range svc.Endpoint.Ports {
			service.EndpointPorts[i] = agentsdocker.ServicePort{
				Name:          port.Name,
				Protocol:      string(port.Protocol),
				TargetPort:    port.TargetPort,
				PublishedPort: port.PublishedPort,
				PublishMode:   string(port.PublishMode),
			}
		}
	}

	if !svc.Meta.CreatedAt.IsZero() {
		created := svc.Meta.CreatedAt
		service.CreatedAt = &created
	}
	if !svc.Meta.UpdatedAt.IsZero() {
		updated := svc.Meta.UpdatedAt
		service.UpdatedAt = &updated
	}

	return service
}

func mapSwarmTask(task *swarmtypes.Task, svc *swarmtypes.Service, containers map[string]agentsdocker.Container) agentsdocker.Task {
	result := agentsdocker.Task{
		ID:           task.ID,
		ServiceID:    task.ServiceID,
		Slot:         task.Slot,
		NodeID:       task.NodeID,
		DesiredState: string(task.DesiredState),
		CurrentState: string(task.Status.State),
		Error:        task.Status.Err,
		Message:      task.Status.Message,
		CreatedAt:    task.Meta.CreatedAt,
	}

	if svc != nil {
		result.ServiceName = svc.Spec.Annotations.Name
	}

	if !task.Meta.UpdatedAt.IsZero() {
		updated := task.Meta.UpdatedAt
		result.UpdatedAt = &updated
	}

	if ts := task.Status.Timestamp; !ts.IsZero() {
		if task.Status.State == swarmtypes.TaskStateRunning {
			started := ts
			result.StartedAt = &started
		} else if isTaskCompletedState(string(task.Status.State)) {
			completed := ts
			result.CompletedAt = &completed
		}
	}

	if cs := task.Status.ContainerStatus; cs != nil {
		if cs.ContainerID != "" {
			result.ContainerID = cs.ContainerID
			if container, ok := lookupContainer(containers, cs.ContainerID); ok {
				result.ContainerID = container.ID
				result.ContainerName = container.Name
				if container.StartedAt != nil && !container.StartedAt.IsZero() && result.StartedAt == nil {
					started := *container.StartedAt
					result.StartedAt = &started
				}
				if container.FinishedAt != nil && !container.FinishedAt.IsZero() && result.CompletedAt == nil {
					finished := *container.FinishedAt
					result.CompletedAt = &finished
				}
			}
		}
	}

	return result
}

func mapSwarmNode(node *swarmtypes.Node) agentsdocker.Node {
	result := agentsdocker.Node{
		ID:            strings.TrimSpace(node.ID),
		Hostname:      strings.TrimSpace(node.Description.Hostname),
		Role:          strings.TrimSpace(string(node.Spec.Role)),
		Availability:  strings.TrimSpace(string(node.Spec.Availability)),
		State:         strings.TrimSpace(string(node.Status.State)),
		Message:       strings.TrimSpace(node.Status.Message),
		Address:       strings.TrimSpace(node.Status.Addr),
		EngineVersion: strings.TrimSpace(node.Description.Engine.EngineVersion),
		OS:            strings.TrimSpace(node.Description.Platform.OS),
		Architecture:  strings.TrimSpace(node.Description.Platform.Architecture),
		NanoCPUs:      node.Description.Resources.NanoCPUs,
		MemoryBytes:   node.Description.Resources.MemoryBytes,
		Labels:        copyStringMap(node.Spec.Annotations.Labels),
		EngineLabels:  copyStringMap(node.Description.Engine.Labels),
		CreatedAt:     node.Meta.CreatedAt,
	}

	if !node.Meta.UpdatedAt.IsZero() {
		updated := node.Meta.UpdatedAt
		result.UpdatedAt = &updated
	}

	if node.ManagerStatus != nil {
		result.ManagerReachability = strings.TrimSpace(string(node.ManagerStatus.Reachability))
		result.ManagerAddress = strings.TrimSpace(node.ManagerStatus.Addr)
		result.Leader = node.ManagerStatus.Leader
	}

	return result
}

func mapSwarmSecret(secret *swarmtypes.Secret) agentsdocker.Secret {
	if secret == nil {
		return agentsdocker.Secret{}
	}

	result := agentsdocker.Secret{
		ID:        strings.TrimSpace(secret.ID),
		Name:      strings.TrimSpace(secret.Spec.Annotations.Name),
		Labels:    copyStringMap(secret.Spec.Annotations.Labels),
		CreatedAt: secret.Meta.CreatedAt,
	}
	if secret.Spec.Driver != nil {
		result.DriverName = strings.TrimSpace(secret.Spec.Driver.Name)
	}
	if secret.Spec.Templating != nil {
		result.TemplatingDriver = strings.TrimSpace(secret.Spec.Templating.Name)
	}
	if !secret.Meta.UpdatedAt.IsZero() {
		updated := secret.Meta.UpdatedAt
		result.UpdatedAt = &updated
	}
	return result
}

func mapSwarmConfig(config *swarmtypes.Config) agentsdocker.Config {
	if config == nil {
		return agentsdocker.Config{}
	}

	result := agentsdocker.Config{
		ID:        strings.TrimSpace(config.ID),
		Name:      strings.TrimSpace(config.Spec.Annotations.Name),
		Labels:    copyStringMap(config.Spec.Annotations.Labels),
		CreatedAt: config.Meta.CreatedAt,
	}
	if config.Spec.Templating != nil {
		result.TemplatingDriver = strings.TrimSpace(config.Spec.Templating.Name)
	}
	if !config.Meta.UpdatedAt.IsZero() {
		updated := config.Meta.UpdatedAt
		result.UpdatedAt = &updated
	}
	return result
}

func deriveLocalSwarmNode(info systemtypes.Info, hostname string) []agentsdocker.Node {
	nodeID := strings.TrimSpace(info.Swarm.NodeID)
	if nodeID == "" {
		return nil
	}

	role := "worker"
	if info.Swarm.ControlAvailable {
		role = "manager"
	}

	node := agentsdocker.Node{
		ID:            nodeID,
		Hostname:      strings.TrimSpace(hostname),
		Role:          role,
		State:         string(info.Swarm.LocalNodeState),
		Message:       strings.TrimSpace(info.Swarm.Error),
		EngineVersion: strings.TrimSpace(info.ServerVersion),
		OS:            strings.TrimSpace(info.OSType),
		Architecture:  strings.TrimSpace(info.Architecture),
		NanoCPUs:      int64(info.NCPU) * 1_000_000_000,
		MemoryBytes:   info.MemTotal,
	}
	return []agentsdocker.Node{node}
}

func serviceMode(mode swarmtypes.ServiceMode) string {
	switch {
	case mode.Global != nil:
		return "global"
	case mode.ReplicatedJob != nil:
		return "replicated-job"
	case mode.GlobalJob != nil:
		return "global-job"
	case mode.Replicated != nil:
		return "replicated"
	default:
		return ""
	}
}

func buildContainerIndex(containers []agentsdocker.Container) map[string]agentsdocker.Container {
	if len(containers) == 0 {
		return nil
	}

	index := make(map[string]agentsdocker.Container, len(containers)*2)
	for _, c := range containers {
		index[c.ID] = c
		if len(c.ID) >= 12 {
			index[c.ID[:12]] = c
		}
	}
	return index
}

func lookupContainer(index map[string]agentsdocker.Container, id string) (agentsdocker.Container, bool) {
	if len(index) == 0 {
		return agentsdocker.Container{}, false
	}

	if container, ok := index[id]; ok {
		return container, true
	}
	if len(id) > 12 {
		if container, ok := index[id[:12]]; ok {
			return container, true
		}
	}
	return agentsdocker.Container{}, false
}

func deriveSwarmTasksFromContainers(containers []agentsdocker.Container, info systemtypes.Info) []agentsdocker.Task {
	if len(containers) == 0 {
		return nil
	}

	tasks := make([]agentsdocker.Task, 0, len(containers))

	for _, container := range containers {
		if len(container.Labels) == 0 {
			continue
		}

		serviceID := container.Labels["com.docker.swarm.service.id"]
		serviceName := container.Labels["com.docker.swarm.service.name"]
		if serviceID == "" && serviceName == "" {
			continue
		}

		taskID := container.Labels["com.docker.swarm.task.id"]
		if taskID == "" {
			taskID = container.ID
		}

		task := agentsdocker.Task{
			ID:            taskID,
			ServiceID:     serviceID,
			ServiceName:   serviceName,
			ContainerID:   container.ID,
			ContainerName: container.Name,
			NodeID:        container.Labels["com.docker.swarm.node.id"],
			NodeName:      container.Labels["com.docker.swarm.node.name"],
			DesiredState:  container.Labels["com.docker.swarm.task.desired-state"],
			CurrentState:  strings.ToLower(strings.TrimSpace(container.State)),
			CreatedAt:     container.CreatedAt,
		}

		if task.NodeID == "" {
			task.NodeID = info.Swarm.NodeID
		}

		if slotStr := container.Labels["com.docker.swarm.task.slot"]; slotStr != "" {
			if slot, err := strconv.Atoi(slotStr); err == nil {
				task.Slot = slot
			}
		}

		if msg := container.Labels["com.docker.swarm.task.message"]; msg != "" {
			task.Message = msg
		}
		if errMsg := container.Labels["com.docker.swarm.task.error"]; errMsg != "" {
			task.Error = errMsg
		}

		if container.StartedAt != nil && !container.StartedAt.IsZero() {
			started := *container.StartedAt
			task.StartedAt = &started
		}
		if container.FinishedAt != nil && !container.FinishedAt.IsZero() {
			finished := *container.FinishedAt
			task.CompletedAt = &finished
		}

		tasks = append(tasks, task)
	}

	return tasks
}

func deriveSwarmServicesFromData(tasks []agentsdocker.Task, containers []agentsdocker.Container) []agentsdocker.Service {
	if len(tasks) == 0 {
		return nil
	}

	type aggregate struct {
		service   agentsdocker.Service
		total     int
		running   int
		completed int
	}

	aggregates := make(map[string]*aggregate)
	for _, task := range tasks {
		key := task.ServiceID
		if key == "" {
			key = task.ServiceName
		}
		if key == "" {
			continue
		}

		agg, ok := aggregates[key]
		if !ok {
			serviceID := task.ServiceID
			if serviceID == "" {
				serviceID = key
			}
			agg = &aggregate{
				service: agentsdocker.Service{
					ID:   serviceID,
					Name: task.ServiceName,
				},
			}
			aggregates[key] = agg
		}

		if task.ServiceName != "" {
			agg.service.Name = task.ServiceName
		}

		agg.total++
		if strings.EqualFold(task.CurrentState, "running") {
			agg.running++
		}
		if isTaskCompletedState(task.CurrentState) {
			agg.completed++
		}
	}

	if len(aggregates) == 0 {
		return nil
	}

	for _, container := range containers {
		if len(container.Labels) == 0 {
			continue
		}
		key := container.Labels["com.docker.swarm.service.id"]
		if key == "" {
			key = container.Labels["com.docker.swarm.service.name"]
		}
		if key == "" {
			continue
		}
		agg, ok := aggregates[key]
		if !ok {
			continue
		}
		if agg.service.Image == "" {
			agg.service.Image = container.Image
		}
		if stack := container.Labels["com.docker.stack.namespace"]; stack != "" {
			if agg.service.Stack == "" {
				agg.service.Stack = stack
			}
			if agg.service.Labels == nil {
				agg.service.Labels = map[string]string{}
			}
			agg.service.Labels["com.docker.stack.namespace"] = stack
		}
	}

	services := make([]agentsdocker.Service, 0, len(aggregates))
	for _, agg := range aggregates {
		agg.service.DesiredTasks = agg.total
		agg.service.RunningTasks = agg.running
		agg.service.CompletedTasks = agg.completed
		if len(agg.service.Labels) == 0 {
			agg.service.Labels = nil
		}
		services = append(services, agg.service)
	}

	return services
}

func copyStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]string, len(source))
	for k, v := range source {
		result[k] = v
	}
	return result
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func isTaskCompletedState(state string) bool {
	switch strings.ToLower(state) {
	case "completed", "complete", "shutdown", "failed", "rejected":
		return true
	default:
		return false
	}
}
