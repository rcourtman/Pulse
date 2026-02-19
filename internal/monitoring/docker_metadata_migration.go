package monitoring

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// CopyDockerContainerMetadata copies persisted container metadata from an old container runtime ID to a new one.
//
// Docker container updates typically recreate the container, producing a new runtime ID. Persisted metadata
// (custom URL, description, tags, notes) is keyed by resource ID and would otherwise be "lost" for the new
// container.
//
// This is intentionally a copy (not a move) so rollback-to-backup scenarios can still find metadata under
// the original container ID.
func (m *Monitor) CopyDockerContainerMetadata(hostID, oldContainerID, newContainerID string) error {
	if m == nil || m.dockerMetadataStore == nil {
		return nil
	}

	hostID = strings.TrimSpace(hostID)
	oldContainerID = strings.TrimSpace(oldContainerID)
	newContainerID = strings.TrimSpace(newContainerID)
	if hostID == "" || oldContainerID == "" || newContainerID == "" || oldContainerID == newContainerID {
		return nil
	}

	oldKey := fmt.Sprintf("%s:container:%s", hostID, oldContainerID)
	newKey := fmt.Sprintf("%s:container:%s", hostID, newContainerID)

	oldMeta := m.dockerMetadataStore.Get(oldKey)
	if oldMeta == nil {
		return nil
	}
	if oldMeta.CustomURL == "" && oldMeta.Description == "" && len(oldMeta.Tags) == 0 && len(oldMeta.Notes) == 0 {
		return nil
	}

	newMeta := m.dockerMetadataStore.Get(newKey)
	var merged config.DockerMetadata
	if newMeta != nil {
		merged = *newMeta
	}

	// Merge missing fields from old -> new, so we don't clobber any metadata already present under the new ID.
	if merged.CustomURL == "" {
		merged.CustomURL = oldMeta.CustomURL
	}
	if merged.Description == "" {
		merged.Description = oldMeta.Description
	}
	if len(merged.Tags) == 0 && len(oldMeta.Tags) > 0 {
		merged.Tags = append([]string(nil), oldMeta.Tags...)
	}
	if len(merged.Notes) == 0 && len(oldMeta.Notes) > 0 {
		merged.Notes = append([]string(nil), oldMeta.Notes...)
	}

	// Avoid an unnecessary disk write if nothing changed.
	if newMeta != nil &&
		merged.CustomURL == newMeta.CustomURL &&
		merged.Description == newMeta.Description &&
		slices.Equal(merged.Tags, newMeta.Tags) &&
		slices.Equal(merged.Notes, newMeta.Notes) {
		return nil
	}

	return m.dockerMetadataStore.Set(newKey, &merged)
}
