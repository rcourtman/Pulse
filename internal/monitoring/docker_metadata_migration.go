package monitoring

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
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

	oldKey := dockerContainerRuntimeMetadataKey(hostID, oldContainerID)
	newKey := dockerContainerRuntimeMetadataKey(hostID, newContainerID)

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

func dockerContainerRuntimeMetadataKey(hostID, containerID string) string {
	hostID = strings.TrimSpace(hostID)
	containerID = strings.TrimSpace(containerID)
	if hostID == "" || containerID == "" {
		return ""
	}
	return fmt.Sprintf("%s:container:%s", hostID, containerID)
}

func dockerContainerNameMetadataKey(hostID, containerName string) string {
	hostID = strings.TrimSpace(hostID)
	containerName = normalizeDockerContainerMetadataIdentity(containerName)
	if hostID == "" || containerName == "" {
		return ""
	}
	return fmt.Sprintf("%s:container-name:%s", hostID, containerName)
}

func dockerAppContainerMetadataKey(hostID, containerName string) string {
	hostID = strings.TrimSpace(hostID)
	containerName = normalizeDockerContainerMetadataIdentity(containerName)
	if hostID == "" || containerName == "" {
		return ""
	}
	return fmt.Sprintf("app-container:%s:name:%s", hostID, containerName)
}

func dockerAppContainerLegacyResourceID(hostID, containerID string) string {
	hostID = strings.TrimSpace(hostID)
	containerID = strings.TrimSpace(containerID)
	if hostID == "" || containerID == "" {
		return ""
	}
	return unifiedresources.SourceSpecificID(
		unifiedresources.ResourceTypeAppContainer,
		unifiedresources.SourceDocker,
		fmt.Sprintf("%s/container/%s", hostID, containerID),
	)
}

func dockerAppContainerLegacyMetadataKey(hostID, containerID string) string {
	hostID = strings.TrimSpace(hostID)
	containerID = strings.TrimSpace(containerID)
	if hostID == "" || containerID == "" {
		return ""
	}
	return fmt.Sprintf("app-container:%s:%s", hostID, containerID)
}

func dockerAppContainerGuestMetadataLegacyKeys(hostID, containerID string) []string {
	candidates := []string{
		dockerAppContainerLegacyResourceID(hostID, containerID),
		dockerAppContainerLegacyMetadataKey(hostID, containerID),
	}
	out := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func copyDockerMetadataAliasIfTargetMissing(store *config.DockerMetadataStore, sourceKey, targetKey string) error {
	if store == nil || strings.TrimSpace(sourceKey) == "" || strings.TrimSpace(targetKey) == "" {
		return nil
	}
	if store.Get(targetKey) != nil {
		return nil
	}
	source := store.Get(sourceKey)
	if source == nil {
		return nil
	}
	clone := &config.DockerMetadata{
		CustomURL:   source.CustomURL,
		Description: source.Description,
	}
	if len(source.Tags) > 0 {
		clone.Tags = append([]string(nil), source.Tags...)
	}
	if len(source.Notes) > 0 {
		clone.Notes = append([]string(nil), source.Notes...)
	}
	return store.Set(targetKey, clone)
}

func copyGuestMetadataAliasIfTargetMissing(
	store *config.GuestMetadataStore,
	sourceKey,
	targetKey,
	containerName string,
) error {
	if store == nil || strings.TrimSpace(sourceKey) == "" || strings.TrimSpace(targetKey) == "" {
		return nil
	}
	if store.Get(targetKey) != nil {
		return nil
	}
	source := store.Get(sourceKey)
	if source == nil {
		return nil
	}
	clone := &config.GuestMetadata{
		CustomURL:     source.CustomURL,
		Description:   source.Description,
		LastKnownName: normalizeDockerContainerMetadataIdentity(containerName),
		LastKnownType: "app-container",
	}
	if len(source.Tags) > 0 {
		clone.Tags = append([]string(nil), source.Tags...)
	}
	if len(source.Notes) > 0 {
		clone.Notes = append([]string(nil), source.Notes...)
	}
	return store.Set(targetKey, clone)
}

func (m *Monitor) migrateCurrentDockerContainerMetadataToStableIdentities(
	hostID string,
	containers []models.DockerContainer,
) {
	if m == nil || len(containers) == 0 {
		return
	}

	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return
	}

	for _, container := range containers {
		containerID := strings.TrimSpace(container.ID)
		containerName := normalizeDockerContainerMetadataIdentity(container.Name)
		if containerID == "" || containerName == "" {
			continue
		}

		if stableKey := dockerContainerNameMetadataKey(hostID, containerName); stableKey != "" {
			sourceKey := dockerContainerRuntimeMetadataKey(hostID, containerID)
			if err := copyDockerMetadataAliasIfTargetMissing(m.dockerMetadataStore, sourceKey, stableKey); err != nil {
				log.Warn().
					Err(err).
					Str("dockerHostID", hostID).
					Str("containerName", container.Name).
					Str("containerID", container.ID).
					Msg("Failed to migrate docker metadata to stable container name key")
			}
		}

		if stableKey := dockerAppContainerMetadataKey(hostID, containerName); stableKey != "" {
			for _, sourceKey := range dockerAppContainerGuestMetadataLegacyKeys(hostID, containerID) {
				if err := copyGuestMetadataAliasIfTargetMissing(m.guestMetadataStore, sourceKey, stableKey, containerName); err != nil {
					log.Warn().
						Err(err).
						Str("dockerHostID", hostID).
						Str("containerName", container.Name).
						Str("containerID", container.ID).
						Msg("Failed to migrate guest metadata to stable app-container name key")
				}
				if m.guestMetadataStore != nil && m.guestMetadataStore.Get(stableKey) != nil {
					break
				}
			}

			// URLs saved through the resource drawer historically landed in the
			// docker store under the runtime container key, which any stable
			// record outranks in the unified customUrl projection. Fold them
			// into the stable guest key so those saves surface in the tables
			// and survive container recreation. Runtime key first: it is where
			// writes landed, so it is fresher than its copy-if-missing snapshot
			// under the container-name key.
			if m.guestMetadataStore != nil && m.dockerMetadataStore != nil &&
				m.guestMetadataStore.Get(stableKey) == nil {
				source := m.dockerMetadataStore.Get(dockerContainerRuntimeMetadataKey(hostID, containerID))
				if source == nil {
					source = m.dockerMetadataStore.Get(dockerContainerNameMetadataKey(hostID, containerName))
				}
				if source != nil {
					clone := &config.GuestMetadata{
						CustomURL:     source.CustomURL,
						Description:   source.Description,
						LastKnownName: containerName,
						LastKnownType: "app-container",
					}
					if len(source.Tags) > 0 {
						clone.Tags = append([]string(nil), source.Tags...)
					}
					if len(source.Notes) > 0 {
						clone.Notes = append([]string(nil), source.Notes...)
					}
					if err := m.guestMetadataStore.Set(stableKey, clone); err != nil {
						log.Warn().
							Err(err).
							Str("dockerHostID", hostID).
							Str("containerName", container.Name).
							Str("containerID", container.ID).
							Msg("Failed to migrate docker metadata to stable app-container guest key")
					}
				}
			}
		}
	}
}

func (m *Monitor) migrateDockerContainerMetadataForRecreatedContainers(
	hostID string,
	previousContainers []models.DockerContainer,
	currentContainers []models.DockerContainer,
) {
	if m == nil || m.dockerMetadataStore == nil {
		return
	}

	hostID = strings.TrimSpace(hostID)
	if hostID == "" || len(previousContainers) == 0 || len(currentContainers) == 0 {
		return
	}

	previousByName := make(map[string]models.DockerContainer)
	ambiguous := make(map[string]struct{})
	for _, container := range previousContainers {
		name := normalizeDockerContainerMetadataIdentity(container.Name)
		if name == "" || strings.TrimSpace(container.ID) == "" {
			continue
		}
		if _, exists := previousByName[name]; exists {
			ambiguous[name] = struct{}{}
			delete(previousByName, name)
			continue
		}
		if _, dup := ambiguous[name]; dup {
			continue
		}
		previousByName[name] = container
	}

	for _, container := range currentContainers {
		name := normalizeDockerContainerMetadataIdentity(container.Name)
		if name == "" || strings.TrimSpace(container.ID) == "" {
			continue
		}
		if _, dup := ambiguous[name]; dup {
			continue
		}
		previousContainer, ok := previousByName[name]
		if !ok {
			continue
		}
		if strings.TrimSpace(previousContainer.ID) == strings.TrimSpace(container.ID) {
			continue
		}
		if err := m.CopyDockerContainerMetadata(hostID, previousContainer.ID, container.ID); err != nil {
			log.Warn().
				Err(err).
				Str("dockerHostID", hostID).
				Str("containerName", container.Name).
				Str("oldContainerID", previousContainer.ID).
				Str("newContainerID", container.ID).
				Msg("Failed to migrate docker container metadata after observed recreation")
		}
	}
}

func normalizeDockerContainerMetadataIdentity(name string) string {
	return strings.TrimLeft(strings.TrimSpace(name), "/")
}
