package dockeragent

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	systemtypes "github.com/docker/docker/api/types/system"
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

func (a *Agent) collectSwarmData(ctx context.Context, info systemtypes.Info, containers []agentsdocker.Container) ([]agentsdocker.Service, []agentsdocker.Task, *agentsdocker.SwarmInfo) {
	if !a.supportsSwarm {
		return nil, nil, nil
	}

	if info.Swarm.NodeID == "" && string(info.Swarm.LocalNodeState) == "" && strings.TrimSpace(info.Swarm.Error) == "" {
		return nil, nil, nil
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
		return nil, nil, swarmInfo
	}

	var services []agentsdocker.Service
	var tasks []agentsdocker.Task

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

	swarmInfo.Scope = effectiveScope

	if !includeServices {
		services = nil
	}
	if !includeTasks {
		tasks = nil
	}

	return services, tasks, swarmInfo
}

func (a *Agent) collectSwarmDataFromManager(ctx context.Context, info systemtypes.Info, scope string, containers map[string]agentsdocker.Container, includeServices, includeTasks bool) ([]agentsdocker.Service, []agentsdocker.Task, error) {
	serviceList, err := dockerCallWithRetry(ctx, dockerSwarmListCallTimeout, func(callCtx context.Context) ([]swarmtypes.Service, error) {
		return a.docker.ServiceList(callCtx, swarmtypes.ServiceListOptions{Status: true})
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
		taskFilters := filters.NewArgs()
		if scope != swarmScopeCluster && info.Swarm.NodeID != "" {
			taskFilters.Add("node", info.Swarm.NodeID)
		}

		taskList, err := dockerCallWithRetry(ctx, dockerSwarmListCallTimeout, func(callCtx context.Context) ([]swarmtypes.Task, error) {
			return a.docker.TaskList(callCtx, swarmtypes.TaskListOptions{Filters: taskFilters})
		})
		if err != nil {
			return services, nil, annotateDockerConnectionError(err)
		}

		tasks = make([]agentsdocker.Task, 0, len(taskList))
		for i := range taskList {
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

func isTaskCompletedState(state string) bool {
	switch strings.ToLower(state) {
	case "completed", "complete", "shutdown", "failed", "rejected":
		return true
	default:
		return false
	}
}
