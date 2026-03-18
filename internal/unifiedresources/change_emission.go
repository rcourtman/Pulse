package unifiedresources

import (
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
)

func recordRegistryChanges(store ResourceStore, before, after []Resource, observedAt time.Time, occurredAt *time.Time, sourceType ChangeSourceType, sourceAdapterHint ChangeSourceAdapter) {
	if store == nil {
		return
	}

	beforeByID := make(map[string]Resource, len(before))
	for _, resource := range before {
		if resource.ID == "" {
			continue
		}
		beforeByID[resource.ID] = resource
	}

	afterByID := make(map[string]Resource, len(after))
	for _, resource := range after {
		if resource.ID == "" {
			continue
		}
		afterByID[resource.ID] = resource
	}

	seen := make(map[string]struct{}, len(beforeByID)+len(afterByID))
	for id := range beforeByID {
		seen[id] = struct{}{}
	}
	for id := range afterByID {
		seen[id] = struct{}{}
	}

	for id := range seen {
		beforeResource, beforeOK := beforeByID[id]
		afterResource, afterOK := afterByID[id]
		change := buildResourceChange(beforeResource, beforeOK, afterResource, afterOK, observedAt, occurredAt, sourceType, sourceAdapterHint)
		if change == nil {
			continue
		}
		if err := store.RecordChange(*change); err != nil {
			log.Printf("unifiedresources: failed to record change for %s: %v", change.ResourceID, err)
		}
	}
}

func buildResourceChange(before Resource, beforeOK bool, after Resource, afterOK bool, observedAt time.Time, occurredAt *time.Time, sourceType ChangeSourceType, sourceAdapterHint ChangeSourceAdapter) *ResourceChange {
	if !beforeOK && !afterOK {
		return nil
	}

	change := &ResourceChange{
		ID:            uuid.NewString(),
		ObservedAt:    observedAt,
		OccurredAt:    cloneTimePtr(occurredAt),
		Confidence:    ConfidenceHigh,
		SourceType:    sourceType,
		SourceAdapter: sourceAdapterHint,
	}

	if afterOK {
		change.ResourceID = after.ID
	} else {
		change.ResourceID = before.ID
	}
	if change.ResourceID == "" {
		return nil
	}

	if !beforeOK {
		change.Kind = ChangeStateTransition
		change.From = "absent"
		change.To = resourceStatusString(after.Status)
		change.Reason = "resource discovered"
		change.Metadata = map[string]any{"changeType": "resource_created"}
		if change.SourceAdapter == "" {
			change.SourceAdapter = resourceChangeSourceAdapter(&after)
		}
		return change
	}

	if !afterOK {
		change.Kind = ChangeStateTransition
		change.From = resourceStatusString(before.Status)
		change.To = "absent"
		change.Reason = "resource removed"
		change.Metadata = map[string]any{"changeType": "resource_removed"}
		if change.SourceAdapter == "" {
			change.SourceAdapter = resourceChangeSourceAdapter(&before)
		}
		return change
	}

	changedFields := resourceChangedFields(before, after)
	if len(changedFields) == 0 {
		return nil
	}

	change.Metadata = map[string]any{"changedFields": changedFields}

	switch {
	case before.Status != after.Status || dockerCommandChanged(before, after) || dockerUpdateStatusChanged(before, after) || proxmoxLifecycleChanged(before, after):
		change.Kind = ChangeStateTransition
		change.From = resourceStateSummary(before)
		change.To = resourceStateSummary(after)
		change.Reason = "resource state changed"
	case !equalStringPtr(before.ParentID, after.ParentID) || !reflect.DeepEqual(before.Relationships, after.Relationships):
		change.Kind = ChangeRelationship
		change.From = resourceRelationSummary(before)
		change.To = resourceRelationSummary(after)
		change.Reason = "resource relationship changed"
		change.RelatedResources = relatedResourceIDs(change.ResourceID, before, after)
	case !reflect.DeepEqual(before.Capabilities, after.Capabilities):
		change.Kind = ChangeCapability
		change.From = fmt.Sprintf("%d", len(before.Capabilities))
		change.To = fmt.Sprintf("%d", len(after.Capabilities))
		change.Reason = "resource capabilities changed"
	default:
		change.Kind = ChangeConfigUpdate
		change.From = resourceConfigSummary(before)
		change.To = resourceConfigSummary(after)
		change.Reason = "resource configuration changed"
	}

	if change.SourceAdapter == "" {
		change.SourceAdapter = resourceChangeSourceAdapter(&after)
	}

	return change
}

func resourceChangedFields(before, after Resource) []string {
	var changed []string

	if before.Type != after.Type {
		changed = append(changed, "type")
	}
	if before.Technology != after.Technology {
		changed = append(changed, "technology")
	}
	if before.Name != after.Name {
		changed = append(changed, "name")
	}
	if before.Status != after.Status {
		changed = append(changed, "status")
	}
	if !equalStringPtr(before.ParentID, after.ParentID) {
		changed = append(changed, "parentId")
	}
	if !reflect.DeepEqual(before.Relationships, after.Relationships) {
		changed = append(changed, "relationships")
	}
	if !reflect.DeepEqual(before.Capabilities, after.Capabilities) {
		changed = append(changed, "capabilities")
	}
	if !sameStringSet(before.Tags, after.Tags) {
		changed = append(changed, "tags")
	}
	if before.CustomURL != after.CustomURL {
		changed = append(changed, "customUrl")
	}
	if !reflect.DeepEqual(before.Identity, after.Identity) {
		changed = append(changed, "identity")
	}
	if dockerCommandChanged(before, after) {
		changed = append(changed, "docker.command")
	}
	if dockerUpdateStatusChanged(before, after) {
		changed = append(changed, "docker.updateStatus")
	}
	if proxmoxLifecycleChanged(before, after) {
		changed = append(changed, "proxmox.lifecycle")
	}

	return changed
}

func dockerCommandChanged(before, after Resource) bool {
	var beforeCommand, afterCommand any
	if before.Docker != nil {
		beforeCommand = before.Docker.Command
	}
	if after.Docker != nil {
		afterCommand = after.Docker.Command
	}
	return !reflect.DeepEqual(beforeCommand, afterCommand)
}

func dockerUpdateStatusChanged(before, after Resource) bool {
	var beforeStatus, afterStatus any
	if before.Docker != nil {
		beforeStatus = before.Docker.UpdateStatus
	}
	if after.Docker != nil {
		afterStatus = after.Docker.UpdateStatus
	}
	return !reflect.DeepEqual(beforeStatus, afterStatus)
}

func proxmoxLifecycleChanged(before, after Resource) bool {
	var beforeProxmox, afterProxmox *struct {
		ClusterName    string
		LinkedAgentID  string
		Lock           string
		PendingUpdates int
	}
	if before.Proxmox != nil {
		beforeProxmox = &struct {
			ClusterName    string
			LinkedAgentID  string
			Lock           string
			PendingUpdates int
		}{
			ClusterName:    before.Proxmox.ClusterName,
			LinkedAgentID:  before.Proxmox.LinkedAgentID,
			Lock:           before.Proxmox.Lock,
			PendingUpdates: before.Proxmox.PendingUpdates,
		}
	}
	if after.Proxmox != nil {
		afterProxmox = &struct {
			ClusterName    string
			LinkedAgentID  string
			Lock           string
			PendingUpdates int
		}{
			ClusterName:    after.Proxmox.ClusterName,
			LinkedAgentID:  after.Proxmox.LinkedAgentID,
			Lock:           after.Proxmox.Lock,
			PendingUpdates: after.Proxmox.PendingUpdates,
		}
	}
	return !reflect.DeepEqual(beforeProxmox, afterProxmox)
}

func resourceStateSummary(resource Resource) string {
	status := resourceStatusString(resource.Status)
	if status == "" {
		status = "unknown"
	}
	return status
}

func resourceRelationSummary(resource Resource) string {
	parentID := ""
	if resource.ParentID != nil {
		parentID = strings.TrimSpace(*resource.ParentID)
	}
	if parentID == "" {
		parentID = "root"
	}
	return parentID
}

func resourceConfigSummary(resource Resource) string {
	return fmt.Sprintf("%s|%s|%s|%s", resource.Type, resource.Technology, resource.Name, resource.CustomURL)
}

func resourceStatusString(status ResourceStatus) string {
	return strings.TrimSpace(string(status))
}

func relatedResourceIDs(primaryID string, before, after Resource) []string {
	primaryID = CanonicalResourceID(primaryID)
	seen := make(map[string]struct{}, 2)
	var out []string
	appendID := func(id string) {
		id = CanonicalResourceID(id)
		if id == "" || id == primaryID {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if before.ParentID != nil {
		appendID(*before.ParentID)
	}
	if after.ParentID != nil {
		appendID(*after.ParentID)
	}
	for _, relationship := range before.Relationships {
		appendID(relationship.SourceID)
		appendID(relationship.TargetID)
	}
	for _, relationship := range after.Relationships {
		appendID(relationship.SourceID)
		appendID(relationship.TargetID)
	}
	return out
}

func resourceChangeSourceAdapter(resource *Resource) ChangeSourceAdapter {
	if resource == nil {
		return ""
	}
	switch {
	case resource.Docker != nil || hasDataSource(resource.Sources, SourceDocker):
		return AdapterDocker
	case resource.Proxmox != nil || hasDataSource(resource.Sources, SourceProxmox):
		return AdapterProxmox
	case resource.TrueNAS != nil || hasDataSource(resource.Sources, SourceTrueNAS):
		return AdapterTrueNAS
	case resource.Agent != nil || hasDataSource(resource.Sources, SourceAgent):
		return AdapterOpsAgent
	default:
		return ""
	}
}

func changeSourceAdapterForDataSource(source DataSource) ChangeSourceAdapter {
	switch source {
	case SourceDocker:
		return AdapterDocker
	case SourceProxmox:
		return AdapterProxmox
	case SourceTrueNAS:
		return AdapterTrueNAS
	case SourceAgent:
		return AdapterOpsAgent
	default:
		return ""
	}
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}

	counts := make(map[string]int, len(a))
	for _, v := range a {
		counts[v]++
	}
	for _, v := range b {
		counts[v]--
		if counts[v] < 0 {
			return false
		}
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}

func equalStringPtr(a, b *string) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return strings.TrimSpace(*a) == strings.TrimSpace(*b)
	}
}
