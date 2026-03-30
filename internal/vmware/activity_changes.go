package vmware

import (
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// ActivityChanges returns canonical shared-timeline activity derived from the
// cached VMware snapshot.
func (p *Provider) ActivityChanges() []unifiedresources.ResourceChange {
	if p == nil || !IsFeatureEnabled() {
		return nil
	}

	snapshot := p.Snapshot()
	if snapshot == nil {
		return nil
	}

	changes := make([]unifiedresources.ResourceChange, 0)
	for _, host := range snapshot.Hosts {
		resourceID := vmwareSourceID(snapshot.ConnectionID, "host", host.Host)
		changes = append(changes, entityActivityChanges(resourceID, snapshot.ConnectionID, "host", host.Host, host.RecentTasks, host.RecentEvents)...)
	}
	for _, vm := range snapshot.VMs {
		resourceID := vmwareSourceID(snapshot.ConnectionID, "vm", vm.VM)
		changes = append(changes, entityActivityChanges(resourceID, snapshot.ConnectionID, "vm", vm.VM, vm.RecentTasks, vm.RecentEvents)...)
	}
	for _, datastore := range snapshot.Datastores {
		resourceID := vmwareSourceID(snapshot.ConnectionID, "datastore", datastore.Datastore)
		changes = append(changes, entityActivityChanges(resourceID, snapshot.ConnectionID, "datastore", datastore.Datastore, datastore.RecentTasks, datastore.RecentEvents)...)
	}

	sort.SliceStable(changes, func(i, j int) bool {
		if !changes[i].ObservedAt.Equal(changes[j].ObservedAt) {
			return changes[i].ObservedAt.After(changes[j].ObservedAt)
		}
		return changes[i].ID > changes[j].ID
	})
	return changes
}

func entityActivityChanges(resourceID, connectionID, entityType, managedObjectID string, tasks []InventoryTask, events []InventoryEvent) []unifiedresources.ResourceChange {
	out := make([]unifiedresources.ResourceChange, 0, len(tasks)+len(events))
	for _, task := range tasks {
		change := unifiedresources.BuildPlatformActivityChange(resourceID, unifiedresources.PlatformActivityChange{
			SourceAdapter: unifiedresources.AdapterVMware,
			ActivityType:  "vmware_task",
			NativeID:      strings.TrimSpace(task.Task),
			Title:         strings.TrimSpace(task.Name),
			State:         strings.TrimSpace(task.State),
			Message:       strings.TrimSpace(task.ErrorMessage),
			OccurredAt:    firstNonZeroTime(task.CompletedAt, task.StartedAt),
			Metadata: map[string]any{
				"vmwareConnectionId":    strings.TrimSpace(connectionID),
				"vmwareEntityType":      strings.TrimSpace(entityType),
				"vmwareManagedObjectId": strings.TrimSpace(managedObjectID),
				"vmwareTask":            strings.TrimSpace(task.Task),
				"vmwareTaskName":        strings.TrimSpace(task.Name),
				"vmwareTaskState":       strings.TrimSpace(task.State),
				"vmwareTaskDescription": strings.TrimSpace(task.DescriptionID),
				"vmwareTaskError":       strings.TrimSpace(task.ErrorMessage),
			},
		})
		if change != nil {
			out = append(out, *change)
		}
	}
	for _, event := range events {
		change := unifiedresources.BuildPlatformActivityChange(resourceID, unifiedresources.PlatformActivityChange{
			SourceAdapter: unifiedresources.AdapterVMware,
			ActivityType:  "vmware_event",
			NativeID:      strings.TrimSpace(event.Event),
			Title:         firstNonEmptyTrimmed(event.Type),
			Message:       strings.TrimSpace(event.Message),
			Actor:         strings.TrimSpace(event.User),
			OccurredAt:    event.CreatedAt,
			Metadata: map[string]any{
				"vmwareConnectionId":    strings.TrimSpace(connectionID),
				"vmwareEntityType":      strings.TrimSpace(entityType),
				"vmwareManagedObjectId": strings.TrimSpace(managedObjectID),
				"vmwareEvent":           strings.TrimSpace(event.Event),
				"vmwareEventType":       strings.TrimSpace(event.Type),
				"vmwareEventMessage":    strings.TrimSpace(event.Message),
				"vmwareEventUser":       strings.TrimSpace(event.User),
			},
		})
		if change != nil {
			out = append(out, *change)
		}
	}
	return out
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value.UTC()
		}
	}
	return time.Time{}
}
