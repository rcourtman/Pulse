package vmware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const vmwareSignalEnrichmentConcurrency = 8

type viJSONReference struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type viJSONLocalizedMessage struct {
	LocalizedMessage string `json:"localizedMessage"`
	Message          string `json:"message"`
	Key              string `json:"key"`
}

type viJSONAlarmState struct {
	Alarm         viJSONReference `json:"alarm"`
	OverallStatus string          `json:"overallStatus"`
	Acknowledged  bool            `json:"acknowledged"`
	Time          *time.Time      `json:"time"`
}

type viJSONTaskInfo struct {
	Name          *viJSONLocalizedMessage `json:"name"`
	DescriptionID string                  `json:"descriptionId"`
	State         string                  `json:"state"`
	StartTime     *time.Time              `json:"startTime"`
	CompleteTime  *time.Time              `json:"completeTime"`
	Error         *viJSONLocalizedMessage `json:"error"`
}

type viJSONSnapshotInfo struct {
	RootSnapshotList []viJSONSnapshotTree `json:"rootSnapshotList"`
}

type viJSONSnapshotTree struct {
	ChildSnapshotList []viJSONSnapshotTree `json:"childSnapshotList"`
}

type viJSONAlarmInfo struct {
	Name string `json:"name"`
}

type alarmNameCache struct {
	mu    sync.Mutex
	names map[string]string
}

type entitySignals struct {
	OverallStatus string
	Alarms        []InventoryAlarm
	RecentTasks   []InventoryTask
}

func (c *Client) validateSignalFloor(
	ctx context.Context,
	release string,
	sessionID string,
	perfManagerMoID string,
	snapshot *InventorySnapshot,
	perfCounters perfCounterCatalog,
) error {
	if snapshot == nil {
		return nil
	}

	cache := &alarmNameCache{names: make(map[string]string)}
	if len(snapshot.Hosts) > 0 {
		host := snapshot.Hosts[0]
		if _, err := c.collectManagedEntitySignals(ctx, release, sessionID, "HostSystem", host.Host, cache, false); err != nil {
			return err
		}
		hostMetrics, err := c.collectHostPerformanceMetrics(ctx, release, sessionID, perfManagerMoID, perfCounters, host)
		if err != nil {
			return err
		}
		if hostMetrics == nil || hostMetrics.CPUPercent == nil || hostMetrics.MemoryPercent == nil {
			return &ConnectionError{Category: "endpoint", Message: "VMware performance metrics are unavailable for host inventory"}
		}
	}
	if len(snapshot.VMs) > 0 {
		vm := snapshot.VMs[0]
		if _, err := c.collectManagedEntitySignals(ctx, release, sessionID, "VirtualMachine", vm.VM, cache, false); err != nil {
			return err
		}
		if _, err := c.collectVMSnapshotCount(ctx, release, sessionID, vm.VM); err != nil && !isVIJSONNotFound(err) {
			return err
		}
		vmMetrics, err := c.collectVMPerformanceMetrics(ctx, release, sessionID, perfManagerMoID, perfCounters, vm)
		if err != nil {
			return err
		}
		if vmMetrics == nil || vmMetrics.CPUPercent == nil || vmMetrics.MemoryPercent == nil {
			return &ConnectionError{Category: "endpoint", Message: "VMware performance metrics are unavailable for vm inventory"}
		}
	}
	if len(snapshot.Datastores) > 0 {
		datastore := snapshot.Datastores[0]
		if _, err := c.collectManagedEntitySignals(ctx, release, sessionID, "Datastore", datastore.Datastore, cache, false); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) enrichInventorySnapshot(
	ctx context.Context,
	release string,
	sessionID string,
	perfManagerMoID string,
	perfCounters perfCounterCatalog,
	snapshot *InventorySnapshot,
) error {
	if snapshot == nil {
		return nil
	}

	cache := &alarmNameCache{names: make(map[string]string)}
	sem := make(chan struct{}, vmwareSignalEnrichmentConcurrency)
	var wg sync.WaitGroup
	var firstErr error
	var firstErrMu sync.Mutex

	run := func(fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := fn(); err != nil {
				firstErrMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				firstErrMu.Unlock()
			}
		}()
	}

	for i := range snapshot.Hosts {
		i := i
		run(func() error {
			signals, err := c.collectManagedEntitySignals(ctx, release, sessionID, "HostSystem", snapshot.Hosts[i].Host, cache, true)
			if err != nil {
				return err
			}
			metrics, err := c.collectHostPerformanceMetrics(ctx, release, sessionID, perfManagerMoID, perfCounters, snapshot.Hosts[i])
			if err != nil {
				return err
			}
			snapshot.Hosts[i].OverallStatus = signals.OverallStatus
			snapshot.Hosts[i].TriggeredAlarms = signals.Alarms
			snapshot.Hosts[i].RecentTasks = signals.RecentTasks
			snapshot.Hosts[i].Metrics = metrics
			return nil
		})
	}

	for i := range snapshot.VMs {
		i := i
		run(func() error {
			signals, err := c.collectManagedEntitySignals(ctx, release, sessionID, "VirtualMachine", snapshot.VMs[i].VM, cache, true)
			if err != nil {
				return err
			}
			snapshot.VMs[i].OverallStatus = signals.OverallStatus
			snapshot.VMs[i].TriggeredAlarms = signals.Alarms
			snapshot.VMs[i].RecentTasks = signals.RecentTasks
			snapshot.VMs[i].SnapshotCount, err = c.collectVMSnapshotCount(ctx, release, sessionID, snapshot.VMs[i].VM)
			if err != nil && !isVIJSONNotFound(err) {
				return err
			}
			metrics, err := c.collectVMPerformanceMetrics(ctx, release, sessionID, perfManagerMoID, perfCounters, snapshot.VMs[i])
			if err != nil {
				return err
			}
			snapshot.VMs[i].Metrics = metrics
			return nil
		})
	}

	for i := range snapshot.Datastores {
		i := i
		run(func() error {
			signals, err := c.collectManagedEntitySignals(ctx, release, sessionID, "Datastore", snapshot.Datastores[i].Datastore, cache, true)
			if err != nil {
				return err
			}
			snapshot.Datastores[i].OverallStatus = signals.OverallStatus
			snapshot.Datastores[i].TriggeredAlarms = signals.Alarms
			snapshot.Datastores[i].RecentTasks = signals.RecentTasks
			return nil
		})
	}

	wg.Wait()

	firstErrMu.Lock()
	defer firstErrMu.Unlock()
	return firstErr
}

func (c *Client) collectManagedEntitySignals(ctx context.Context, release, sessionID, managedType, managedObjectID string, cache *alarmNameCache, allowNotFound bool) (entitySignals, error) {
	managedObjectID = strings.TrimSpace(managedObjectID)
	if managedObjectID == "" {
		return entitySignals{}, nil
	}

	signals := entitySignals{}

	overallStatus, err := c.collectOverallStatus(ctx, release, sessionID, managedType, managedObjectID)
	if err != nil {
		if !allowNotFound || !isVIJSONNotFound(err) {
			return entitySignals{}, err
		}
	} else {
		signals.OverallStatus = overallStatus
	}

	alarms, err := c.collectTriggeredAlarms(ctx, release, sessionID, managedType, managedObjectID, cache)
	if err != nil {
		if !allowNotFound || !isVIJSONNotFound(err) {
			return entitySignals{}, err
		}
	} else {
		signals.Alarms = alarms
	}

	tasks, err := c.collectRecentTasks(ctx, release, sessionID, managedType, managedObjectID)
	if err != nil {
		if !allowNotFound || !isVIJSONNotFound(err) {
			return entitySignals{}, err
		}
	} else {
		signals.RecentTasks = tasks
	}

	return signals, nil
}

func (c *Client) collectOverallStatus(ctx context.Context, release, sessionID, managedType, managedObjectID string) (string, error) {
	var status string
	path := fmt.Sprintf("/sdk/vim25/%s/%s/%s/overallStatus", release, managedType, managedObjectID)
	if err := c.getVIJSONJSON(ctx, sessionID, path, managedType+" overall status", &status); err != nil {
		return "", err
	}
	return strings.TrimSpace(status), nil
}

func (c *Client) collectTriggeredAlarms(ctx context.Context, release, sessionID, managedType, managedObjectID string, cache *alarmNameCache) ([]InventoryAlarm, error) {
	var states []viJSONAlarmState
	path := fmt.Sprintf("/sdk/vim25/%s/%s/%s/triggeredAlarmState", release, managedType, managedObjectID)
	if err := c.getVIJSONJSON(ctx, sessionID, path, managedType+" triggered alarms", &states); err != nil {
		return nil, err
	}
	alarms := make([]InventoryAlarm, 0, len(states))
	for _, state := range states {
		alarmID := strings.TrimSpace(state.Alarm.Value)
		alarmName := alarmID
		if alarmID != "" {
			if resolvedName, err := c.resolveAlarmName(ctx, release, sessionID, alarmID, cache); err == nil && resolvedName != "" {
				alarmName = resolvedName
			} else if err != nil && !isVIJSONNotFound(err) {
				return nil, err
			}
		}
		var triggeredAt time.Time
		if state.Time != nil {
			triggeredAt = state.Time.UTC()
		}
		alarms = append(alarms, InventoryAlarm{
			Alarm:         alarmID,
			Name:          strings.TrimSpace(alarmName),
			OverallStatus: strings.TrimSpace(state.OverallStatus),
			Acknowledged:  state.Acknowledged,
			TriggeredAt:   triggeredAt,
		})
	}
	sort.SliceStable(alarms, func(i, j int) bool {
		left := inventoryAlarmSortTime(alarms[i])
		right := inventoryAlarmSortTime(alarms[j])
		if !left.Equal(right) {
			return left.After(right)
		}
		return vmwareSortKey(alarms[i].Alarm, alarms[i].Name) < vmwareSortKey(alarms[j].Alarm, alarms[j].Name)
	})
	return alarms, nil
}

func (c *Client) collectRecentTasks(ctx context.Context, release, sessionID, managedType, managedObjectID string) ([]InventoryTask, error) {
	var refs []viJSONReference
	path := fmt.Sprintf("/sdk/vim25/%s/%s/%s/recentTask", release, managedType, managedObjectID)
	if err := c.getVIJSONJSON(ctx, sessionID, path, managedType+" recent tasks", &refs); err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, nil
	}

	limit := len(refs)
	if limit > 3 {
		limit = 3
	}
	tasks := make([]InventoryTask, 0, limit)
	for _, ref := range refs[:limit] {
		taskID := strings.TrimSpace(ref.Value)
		if taskID == "" {
			continue
		}
		info, err := c.collectTaskInfo(ctx, release, sessionID, taskID)
		if err != nil {
			if isVIJSONNotFound(err) {
				continue
			}
			return nil, err
		}
		tasks = append(tasks, info)
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		left := inventoryTaskSortTime(tasks[i])
		right := inventoryTaskSortTime(tasks[j])
		if !left.Equal(right) {
			return left.After(right)
		}
		return vmwareSortKey(tasks[i].Task, tasks[i].Name) < vmwareSortKey(tasks[j].Task, tasks[j].Name)
	})
	return tasks, nil
}

func (c *Client) collectTaskInfo(ctx context.Context, release, sessionID, taskID string) (InventoryTask, error) {
	var payload viJSONTaskInfo
	path := fmt.Sprintf("/sdk/vim25/%s/Task/%s/info", release, taskID)
	if err := c.getVIJSONJSON(ctx, sessionID, path, "vmware task info", &payload); err != nil {
		return InventoryTask{}, err
	}

	task := InventoryTask{
		Task:          taskID,
		Name:          strings.TrimSpace(localizedMessageText(payload.Name)),
		State:         strings.TrimSpace(payload.State),
		DescriptionID: strings.TrimSpace(payload.DescriptionID),
	}
	if task.Name == "" {
		task.Name = task.DescriptionID
	}
	if payload.StartTime != nil {
		task.StartedAt = payload.StartTime.UTC()
	}
	if payload.CompleteTime != nil {
		task.CompletedAt = payload.CompleteTime.UTC()
	}
	task.ErrorMessage = strings.TrimSpace(localizedMessageText(payload.Error))
	return task, nil
}

func (c *Client) collectVMSnapshotCount(ctx context.Context, release, sessionID, managedObjectID string) (int, error) {
	var payload viJSONSnapshotInfo
	path := fmt.Sprintf("/sdk/vim25/%s/VirtualMachine/%s/snapshot", release, managedObjectID)
	if err := c.getVIJSONJSON(ctx, sessionID, path, "vmware vm snapshot tree", &payload); err != nil {
		return 0, err
	}
	return countSnapshotTrees(payload.RootSnapshotList), nil
}

func (c *Client) resolveAlarmName(ctx context.Context, release, sessionID, alarmID string, cache *alarmNameCache) (string, error) {
	alarmID = strings.TrimSpace(alarmID)
	if alarmID == "" {
		return "", nil
	}
	if cache != nil {
		cache.mu.Lock()
		if name, ok := cache.names[alarmID]; ok {
			cache.mu.Unlock()
			return name, nil
		}
		cache.mu.Unlock()
	}

	var payload viJSONAlarmInfo
	path := fmt.Sprintf("/sdk/vim25/%s/Alarm/%s/info", release, alarmID)
	if err := c.getVIJSONJSON(ctx, sessionID, path, "vmware alarm info", &payload); err != nil {
		return "", err
	}
	name := strings.TrimSpace(payload.Name)
	if cache != nil {
		cache.mu.Lock()
		cache.names[alarmID] = name
		cache.mu.Unlock()
	}
	return name, nil
}

func (c *Client) getVIJSONJSON(ctx context.Context, sessionID, path, label string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL.String()+path, nil)
	if err != nil {
		return fmt.Errorf("build %s request: %w", label, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("vmware-api-session-id", sessionID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return classifyTransportError(label, err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, inventoryResponseLimitByte))
	if readErr != nil {
		return fmt.Errorf("read %s response: %w", label, readErr)
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return &ConnectionError{Category: "auth", Message: fmt.Sprintf("VMware authentication failed while reading %s", label)}
	case http.StatusForbidden:
		return &ConnectionError{Category: "permission", Message: fmt.Sprintf("VMware permissions are insufficient for %s", label)}
	case http.StatusNotFound:
		return &ConnectionError{Category: "not_found", Message: fmt.Sprintf("VMware %s endpoint is unavailable", label)}
	default:
		return &ConnectionError{Category: "endpoint", Message: fmt.Sprintf("VMware %s request failed with HTTP %d", label, resp.StatusCode)}
	}
	if err := json.Unmarshal(body, target); err != nil {
		return &ConnectionError{Category: "endpoint", Message: fmt.Sprintf("VMware %s response was not valid JSON", label)}
	}
	return nil
}

func isVIJSONNotFound(err error) bool {
	connectionErr, ok := err.(*ConnectionError)
	return ok && connectionErr.Category == "not_found"
}

func localizedMessageText(message *viJSONLocalizedMessage) string {
	if message == nil {
		return ""
	}
	return firstNonEmptyTrimmed(message.LocalizedMessage, message.Message, message.Key)
}

func inventoryAlarmSortTime(alarm InventoryAlarm) time.Time {
	if alarm.TriggeredAt.IsZero() {
		return time.Time{}
	}
	return alarm.TriggeredAt.UTC()
}

func inventoryTaskSortTime(task InventoryTask) time.Time {
	if !task.StartedAt.IsZero() {
		return task.StartedAt.UTC()
	}
	if !task.CompletedAt.IsZero() {
		return task.CompletedAt.UTC()
	}
	return time.Time{}
}

func countSnapshotTrees(roots []viJSONSnapshotTree) int {
	count := 0
	for _, root := range roots {
		count++
		count += countSnapshotTrees(root.ChildSnapshotList)
	}
	return count
}
