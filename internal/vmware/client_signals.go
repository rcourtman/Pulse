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

type viJSONEventFilterSpec struct {
	Entity   *viJSONEventFilterEntity `json:"entity,omitempty"`
	MaxCount int                      `json:"maxCount,omitempty"`
}

type viJSONEventFilterEntity struct {
	Entity    viJSONReference `json:"entity"`
	Recursion string          `json:"recursion,omitempty"`
}

type viJSONEvent struct {
	Key                  int64      `json:"key"`
	ChainID              int64      `json:"chainId"`
	UserName             string     `json:"userName"`
	CreatedTime          *time.Time `json:"createdTime"`
	FullFormattedMessage string     `json:"fullFormattedMessage"`
	EventTypeID          string     `json:"eventTypeId"`
	TypeName             string     `json:"_typeName"`
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
	RecentEvents  []InventoryEvent
}

func (c *Client) validateSignalFloor(
	ctx context.Context,
	release string,
	sessionID string,
	perfManagerMoID string,
	eventManagerMoID string,
	snapshot *InventorySnapshot,
	perfCounters perfCounterCatalog,
) error {
	if snapshot == nil {
		return nil
	}

	cache := &alarmNameCache{names: make(map[string]string)}
	if len(snapshot.Hosts) > 0 {
		host := snapshot.Hosts[0]
		if _, err := c.collectManagedEntitySignals(ctx, release, sessionID, "HostSystem", host.Host, perfManagerMoID, eventManagerMoID, cache, false); err != nil {
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
		if _, err := c.collectManagedEntitySignals(ctx, release, sessionID, "VirtualMachine", vm.VM, perfManagerMoID, eventManagerMoID, cache, false); err != nil {
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
		if _, err := c.collectManagedEntitySignals(ctx, release, sessionID, "Datastore", datastore.Datastore, perfManagerMoID, eventManagerMoID, cache, false); err != nil {
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
	eventManagerMoID string,
	perfCounters perfCounterCatalog,
	snapshot *InventorySnapshot,
) ([]InventoryEnrichmentIssue, error) {
	if snapshot == nil {
		return nil, nil
	}

	cache := &alarmNameCache{names: make(map[string]string)}
	sem := make(chan struct{}, vmwareSignalEnrichmentConcurrency)
	var wg sync.WaitGroup
	var firstErr error
	var firstErrMu sync.Mutex
	var issues []InventoryEnrichmentIssue
	var issuesMu sync.Mutex

	recordIssue := func(issue *InventoryEnrichmentIssue) {
		if issue == nil {
			return
		}
		issuesMu.Lock()
		issues = append(issues, *issue)
		issuesMu.Unlock()
	}

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
			signals, err := c.collectManagedEntitySignals(ctx, release, sessionID, "HostSystem", snapshot.Hosts[i].Host, perfManagerMoID, eventManagerMoID, cache, true)
			if issue, ok := classifyInventoryEnrichmentIssue("signals", "host", snapshot.Hosts[i].Host, err); ok {
				recordIssue(issue)
			} else if err != nil {
				return err
			}
			metrics, err := c.collectHostPerformanceMetrics(ctx, release, sessionID, perfManagerMoID, perfCounters, snapshot.Hosts[i])
			if issue, ok := classifyInventoryEnrichmentIssue("signals", "host", snapshot.Hosts[i].Host, err); ok {
				recordIssue(issue)
			} else if err != nil {
				return err
			}
			snapshot.Hosts[i].OverallStatus = signals.OverallStatus
			snapshot.Hosts[i].TriggeredAlarms = signals.Alarms
			snapshot.Hosts[i].RecentTasks = signals.RecentTasks
			snapshot.Hosts[i].RecentEvents = signals.RecentEvents
			snapshot.Hosts[i].Metrics = metrics
			return nil
		})
	}

	for i := range snapshot.VMs {
		i := i
		run(func() error {
			signals, err := c.collectManagedEntitySignals(ctx, release, sessionID, "VirtualMachine", snapshot.VMs[i].VM, perfManagerMoID, eventManagerMoID, cache, true)
			if issue, ok := classifyInventoryEnrichmentIssue("signals", "vm", snapshot.VMs[i].VM, err); ok {
				recordIssue(issue)
			} else if err != nil {
				return err
			}
			snapshot.VMs[i].OverallStatus = signals.OverallStatus
			snapshot.VMs[i].TriggeredAlarms = signals.Alarms
			snapshot.VMs[i].RecentTasks = signals.RecentTasks
			snapshot.VMs[i].RecentEvents = signals.RecentEvents
			snapshot.VMs[i].SnapshotCount, err = c.collectVMSnapshotCount(ctx, release, sessionID, snapshot.VMs[i].VM)
			if issue, ok := classifyInventoryEnrichmentIssue("signals", "vm", snapshot.VMs[i].VM, err); ok {
				recordIssue(issue)
			} else if err != nil && !isVIJSONNotFound(err) {
				return err
			}
			metrics, err := c.collectVMPerformanceMetrics(ctx, release, sessionID, perfManagerMoID, perfCounters, snapshot.VMs[i])
			if issue, ok := classifyInventoryEnrichmentIssue("signals", "vm", snapshot.VMs[i].VM, err); ok {
				recordIssue(issue)
			} else if err != nil {
				return err
			}
			snapshot.VMs[i].Metrics = metrics
			return nil
		})
	}

	for i := range snapshot.Datastores {
		i := i
		run(func() error {
			signals, err := c.collectManagedEntitySignals(ctx, release, sessionID, "Datastore", snapshot.Datastores[i].Datastore, perfManagerMoID, eventManagerMoID, cache, true)
			if issue, ok := classifyInventoryEnrichmentIssue("signals", "storage", snapshot.Datastores[i].Datastore, err); ok {
				recordIssue(issue)
			} else if err != nil {
				return err
			}
			snapshot.Datastores[i].OverallStatus = signals.OverallStatus
			snapshot.Datastores[i].TriggeredAlarms = signals.Alarms
			snapshot.Datastores[i].RecentTasks = signals.RecentTasks
			snapshot.Datastores[i].RecentEvents = signals.RecentEvents
			return nil
		})
	}

	wg.Wait()

	firstErrMu.Lock()
	err := firstErr
	firstErrMu.Unlock()
	if err != nil {
		return nil, err
	}
	sort.Slice(issues, func(i, j int) bool {
		return inventoryEnrichmentIssueSortKey(issues[i]) < inventoryEnrichmentIssueSortKey(issues[j])
	})
	return issues, nil
}

func (c *Client) collectManagedEntitySignals(ctx context.Context, release, sessionID, managedType, managedObjectID, perfManagerMoID, eventManagerMoID string, cache *alarmNameCache, allowNotFound bool) (entitySignals, error) {
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

	events, err := c.collectRecentEvents(ctx, release, sessionID, eventManagerMoID, managedType, managedObjectID)
	if err != nil {
		if !allowNotFound || !isVIJSONNotFound(err) {
			return entitySignals{}, err
		}
	} else {
		signals.RecentEvents = events
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

func (c *Client) collectRecentEvents(ctx context.Context, release, sessionID, eventManagerMoID, managedType, managedObjectID string) ([]InventoryEvent, error) {
	eventManagerMoID = strings.TrimSpace(eventManagerMoID)
	if eventManagerMoID == "" {
		return nil, &ConnectionError{Category: "endpoint", Message: "VMware VI JSON API service-instance response did not include an event manager reference"}
	}

	var payload []viJSONEvent
	path := fmt.Sprintf("/sdk/vim25/%s/EventManager/%s/QueryEvents", release, eventManagerMoID)
	body := map[string]any{
		"filter": viJSONEventFilterSpec{
			Entity: &viJSONEventFilterEntity{
				Entity:    viJSONReference{Type: strings.TrimSpace(managedType), Value: strings.TrimSpace(managedObjectID)},
				Recursion: "self",
			},
			MaxCount: 3,
		},
	}
	if err := c.postVIJSONJSON(ctx, sessionID, path, managedType+" recent events", body, &payload); err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return nil, nil
	}

	events := make([]InventoryEvent, 0, len(payload))
	for _, event := range payload {
		eventID := ""
		if event.Key > 0 {
			eventID = fmt.Sprintf("event-%d", event.Key)
		}
		var createdAt time.Time
		if event.CreatedTime != nil {
			createdAt = event.CreatedTime.UTC()
		}
		events = append(events, InventoryEvent{
			Event:     eventID,
			Type:      firstNonEmptyTrimmed(event.EventTypeID, event.TypeName),
			Message:   strings.TrimSpace(event.FullFormattedMessage),
			User:      strings.TrimSpace(event.UserName),
			CreatedAt: createdAt,
		})
	}
	sort.SliceStable(events, func(i, j int) bool {
		left := inventoryEventSortTime(events[i])
		right := inventoryEventSortTime(events[j])
		if !left.Equal(right) {
			return left.After(right)
		}
		return vmwareSortKey(events[i].Event, firstNonEmptyTrimmed(events[i].Type, events[i].Message)) <
			vmwareSortKey(events[j].Event, firstNonEmptyTrimmed(events[j].Type, events[j].Message))
	})
	return events, nil
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
	default:
		return classifyReadStatusCode(label, resp.StatusCode)
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

func inventoryEventSortTime(event InventoryEvent) time.Time {
	if event.CreatedAt.IsZero() {
		return time.Time{}
	}
	return event.CreatedAt.UTC()
}

func countSnapshotTrees(roots []viJSONSnapshotTree) int {
	count := 0
	for _, root := range roots {
		count++
		count += countSnapshotTrees(root.ChildSnapshotList)
	}
	return count
}
