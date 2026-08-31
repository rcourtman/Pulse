package dockeragent

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

const helperInventoryOperationDeadline = 30 * time.Second

// ContainerInventory is the collector-side view of the helper's closed,
// read-only container inventory operation. It intentionally exposes no daemon
// socket, URL, HTTP method, query, container selector, or mutation primitive.
type ContainerInventory interface {
	Inventory(context.Context) (agenthelper.ContainerInventoryResult, error)
}

type privilegeHelperContainerInventory struct {
	client *agenthelper.Client
}

// NewPrivilegeHelperContainerInventory creates a local-only client for the
// fixed helper socket selected by the installer. An empty path disables the
// helper inventory bridge.
func NewPrivilegeHelperContainerInventory(socketPath string) (ContainerInventory, error) {
	if strings.TrimSpace(socketPath) == "" {
		return nil, nil
	}
	client, err := agenthelper.NewClient(agenthelper.ClientConfig{
		SocketPath:  socketPath,
		MaxDeadline: helperInventoryOperationDeadline,
	})
	if err != nil {
		return nil, err
	}
	return &privilegeHelperContainerInventory{client: client}, nil
}

func (c *privilegeHelperContainerInventory) Inventory(ctx context.Context) (agenthelper.ContainerInventoryResult, error) {
	var response agenthelper.ContainerInventoryResult
	_, err := c.client.Call(
		ctx,
		agenthelper.OperationContainerInventory,
		agenthelper.OperationVersion1,
		helperInventoryOperationDeadline,
		struct{}{},
		&response,
	)
	if err != nil {
		return agenthelper.ContainerInventoryResult{}, err
	}
	return response, nil
}

func selectHelperRuntime(result agenthelper.ContainerInventoryResult, preference RuntimeKind) (agenthelper.ContainerRuntimeSnapshot, error) {
	available := make(map[RuntimeKind]agenthelper.ContainerRuntimeSnapshot, len(result.Runtimes))
	for _, snapshot := range result.Runtimes {
		runtime := RuntimeKind(strings.ToLower(strings.TrimSpace(snapshot.Runtime)))
		if runtime != RuntimeDocker && runtime != RuntimePodman {
			continue
		}
		if snapshot.Available {
			available[runtime] = snapshot
		}
	}

	if preference == RuntimeDocker || preference == RuntimePodman {
		if snapshot, ok := available[preference]; ok {
			return snapshot, nil
		}
		return agenthelper.ContainerRuntimeSnapshot{}, fmt.Errorf("typed helper reports %s runtime unavailable", preference)
	}
	if snapshot, ok := available[RuntimeDocker]; ok {
		return snapshot, nil
	}
	if snapshot, ok := available[RuntimePodman]; ok {
		return snapshot, nil
	}
	return agenthelper.ContainerRuntimeSnapshot{}, errors.New("typed helper reports no available container runtime")
}

func (a *Agent) buildHelperInventoryReport(ctx context.Context) (agentsdocker.Report, error) {
	ctx, cancel := context.WithTimeout(ctx, helperInventoryOperationDeadline)
	defer cancel()

	result, err := a.helperInventory.Inventory(ctx)
	if err != nil {
		a.recordHelperInventoryStatus(err)
		return agentsdocker.Report{}, fmt.Errorf("collect typed helper container inventory: %w", err)
	}
	snapshot, err := selectHelperRuntime(result, a.runtimePref)
	if err != nil {
		a.recordHelperInventoryStatus(err)
		return agentsdocker.Report{}, err
	}
	a.recordHelperInventoryStatus(nil)
	runtimeKind := RuntimeKind(strings.ToLower(strings.TrimSpace(snapshot.Runtime)))
	if runtimeKind != a.runtime {
		a.logger.Info().
			Str("runtime_previous", string(a.runtime)).
			Str("runtime_current", string(runtimeKind)).
			Msg("Typed helper container inventory runtime changed")
		a.runtime = runtimeKind
	}

	metricsCtx, metricsCancel := context.WithTimeout(ctx, 10*time.Second)
	metrics, err := hostmetricsCollectWithDiskFilters(metricsCtx, a.cfg.DiskExclude, a.cfg.DiskInclude)
	metricsCancel()
	if err != nil {
		return agentsdocker.Report{}, fmt.Errorf("collect host metrics: %w", err)
	}

	agentID := a.helperAgentID()
	containers := make([]agentsdocker.Container, 0, len(snapshot.Containers))
	for _, summary := range snapshot.Containers {
		state := strings.ToLower(strings.TrimSpace(summary.State))
		if len(a.allowedStates) > 0 {
			if _, ok := a.allowedStates[state]; !ok {
				continue
			}
		}
		if isBackupContainer(summary.Names) {
			continue
		}
		name := ""
		for _, candidate := range summary.Names {
			candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "/")
			if candidate != "" {
				name = candidate
				break
			}
		}
		if name == "" {
			name = shortContainerID(summary.ID)
		}
		container := agentsdocker.Container{
			ID:     strings.TrimSpace(summary.ID),
			Name:   name,
			Image:  strings.TrimSpace(summary.Image),
			State:  state,
			Status: strings.TrimSpace(summary.Status),
		}
		if summary.Created > 0 {
			container.CreatedAt = time.Unix(summary.Created, 0).UTC()
		}
		containers = append(containers, container)
	}

	intervalSeconds := int(a.cfg.Interval / time.Second)
	if intervalSeconds <= 0 {
		intervalSeconds = 30
	}
	inventoryComplete := true
	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID: agentID, Version: a.agentVersion, Type: a.cfg.AgentType,
			IntervalSeconds: intervalSeconds, Modules: a.helperOperationModuleStatuses(),
		},
		InventoryComplete: &inventoryComplete,
		Host: agentsdocker.HostInfo{
			Hostname:         a.hostName,
			Name:             a.hostName,
			MachineID:        a.machineID,
			Runtime:          string(runtimeKind),
			CollectionMode:   agentsdocker.CollectionModeTypedHelperSummary,
			OS:               runtime.GOOS,
			Architecture:     runtime.GOARCH,
			TotalCPU:         metrics.CPUCount,
			UptimeSeconds:    readSystemUptime(),
			CPUUsagePercent:  safeFloat(metrics.CPUUsagePercent),
			LoadAverage:      append([]float64(nil), metrics.LoadAverage...),
			Memory:           metrics.Memory,
			TotalMemoryBytes: metrics.Memory.TotalBytes,
			Disks:            append([]agentsdocker.Disk(nil), metrics.Disks...),
			Network:          append([]agentsdocker.NetworkInterface(nil), metrics.Network...),
		},
		Containers: containers,
		Timestamp:  time.Now().UTC(),
		SequenceID: a.nextReportSequenceID(),
	}
	return report, nil
}

func (a *Agent) buildHelperInventoryStatusReport() agentsdocker.Report {
	inventoryComplete := false
	intervalSeconds := int(a.cfg.Interval / time.Second)
	if intervalSeconds <= 0 {
		intervalSeconds = 30
	}
	return agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID: a.helperAgentID(), Version: a.agentVersion, Type: a.cfg.AgentType,
			IntervalSeconds: intervalSeconds, Modules: a.helperOperationModuleStatuses(),
		},
		Host: agentsdocker.HostInfo{
			Hostname: a.hostName, Name: a.hostName, MachineID: a.machineID,
			Runtime: string(a.runtime), CollectionMode: agentsdocker.CollectionModeTypedHelperSummary,
			OS: runtime.GOOS, Architecture: runtime.GOARCH,
		},
		InventoryComplete: &inventoryComplete,
		Timestamp:         time.Now().UTC(),
		SequenceID:        a.nextReportSequenceID(),
	}
}

func (a *Agent) helperAgentID() string {
	if agentID := strings.TrimSpace(a.cfg.AgentID); agentID != "" {
		return agentID
	}
	if machineID := strings.TrimSpace(a.machineID); machineID != "" {
		return machineID
	}
	return strings.TrimSpace(a.hostName)
}

func (a *Agent) helperOperationModuleStatuses() []agentshost.ModuleStatus {
	if a == nil || a.cfg.HelperOperationStatus == nil {
		return nil
	}
	return []agentshost.ModuleStatus{a.cfg.HelperOperationStatus.ModuleStatus()}
}

func (a *Agent) recordHelperInventoryStatus(err error) {
	if a == nil || a.cfg.HelperOperationStatus == nil {
		return
	}
	a.cfg.HelperOperationStatus.Record(agenthelper.OperationContainerInventory, err)
}

func shortContainerID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
