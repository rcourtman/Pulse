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

	return m.dockerMetadataStore.UpdateAll(func(metadata map[string]*config.DockerMetadata) bool {
		oldMeta := metadata[oldKey]
		if oldMeta == nil {
			return false
		}
		if oldMeta.CustomURL == "" && oldMeta.Description == "" && len(oldMeta.Tags) == 0 && len(oldMeta.Notes) == 0 {
			return false
		}

		newMeta := metadata[newKey]
		var merged config.DockerMetadata
		if newMeta != nil {
			merged = *newMeta
		}

		// Merge missing fields from old -> new, so a concurrent or already
		// persisted value under the recreated runtime ID always wins.
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

		if newMeta != nil &&
			merged.CustomURL == newMeta.CustomURL &&
			merged.Description == newMeta.Description &&
			slices.Equal(merged.Tags, newMeta.Tags) &&
			slices.Equal(merged.Notes, newMeta.Notes) {
			return false
		}

		merged.ID = newKey
		metadata[newKey] = &merged
		return true
	})
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
	return store.UpdateAll(func(metadata map[string]*config.DockerMetadata) bool {
		if metadata[targetKey] != nil {
			return false
		}
		source := metadata[sourceKey]
		if source == nil {
			return false
		}
		clone := &config.DockerMetadata{
			ID:          targetKey,
			CustomURL:   source.CustomURL,
			Description: source.Description,
		}
		if len(source.Tags) > 0 {
			clone.Tags = append([]string(nil), source.Tags...)
		}
		if len(source.Notes) > 0 {
			clone.Notes = append([]string(nil), source.Notes...)
		}
		metadata[targetKey] = clone
		return true
	})
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
	return store.UpdateAll(func(metadata map[string]*config.GuestMetadata) bool {
		if metadata[targetKey] != nil {
			return false
		}
		source := metadata[sourceKey]
		if source == nil {
			return false
		}
		clone := &config.GuestMetadata{
			ID:            targetKey,
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
		metadata[targetKey] = clone
		return true
	})
}

type dockerStableMetadataMove struct {
	sourceKey  string
	targetKey  string
	targetName string
}

func dockerContainerStableMetadataMoves(
	hostID string,
	previousContainers []models.DockerContainer,
	currentContainers []models.DockerContainer,
	keyForName func(string, string) string,
) []dockerStableMetadataMove {
	previousByID := make(map[string]models.DockerContainer, len(previousContainers))
	ambiguousIDs := make(map[string]struct{})
	previousNameCounts := make(map[string]int, len(previousContainers))
	for _, container := range previousContainers {
		containerID := strings.TrimSpace(container.ID)
		if name := normalizeDockerContainerMetadataIdentity(container.Name); name != "" {
			previousNameCounts[keyForName(hostID, name)]++
		}
		if containerID == "" {
			continue
		}
		if _, exists := previousByID[containerID]; exists {
			ambiguousIDs[containerID] = struct{}{}
			delete(previousByID, containerID)
			continue
		}
		if _, ambiguous := ambiguousIDs[containerID]; ambiguous {
			continue
		}
		previousByID[containerID] = container
	}

	currentNameCounts := make(map[string]int, len(currentContainers))
	for _, container := range currentContainers {
		if name := normalizeDockerContainerMetadataIdentity(container.Name); name != "" {
			currentNameCounts[keyForName(hostID, name)]++
		}
	}

	sourceCounts := make(map[string]int)
	targetCounts := make(map[string]int)
	candidates := make([]dockerStableMetadataMove, 0)
	for _, current := range currentContainers {
		containerID := strings.TrimSpace(current.ID)
		if containerID == "" {
			continue
		}
		previous, ok := previousByID[containerID]
		if !ok {
			continue
		}
		previousName := normalizeDockerContainerMetadataIdentity(previous.Name)
		currentName := normalizeDockerContainerMetadataIdentity(current.Name)
		if previousName == "" || currentName == "" || previousName == currentName {
			continue
		}
		sourceKey := keyForName(hostID, previousName)
		targetKey := keyForName(hostID, currentName)
		if sourceKey == "" || targetKey == "" || sourceKey == targetKey {
			continue
		}
		candidates = append(candidates, dockerStableMetadataMove{
			sourceKey:  sourceKey,
			targetKey:  targetKey,
			targetName: currentName,
		})
		sourceCounts[sourceKey]++
		targetCounts[targetKey]++
	}

	moves := make([]dockerStableMetadataMove, 0, len(candidates))
	for _, move := range candidates {
		if sourceCounts[move.sourceKey] == 1 &&
			targetCounts[move.targetKey] == 1 &&
			previousNameCounts[move.sourceKey] == 1 &&
			currentNameCounts[move.targetKey] == 1 {
			moves = append(moves, move)
		}
	}
	return moves
}

func applyStableMetadataMoves[T any](
	moves []dockerStableMetadataMove,
	updateAll func(func(map[string]*T) bool) error,
	cloneForTarget func(*T, dockerStableMetadataMove) *T,
) error {
	if len(moves) == 0 {
		return nil
	}
	return updateAll(func(metadata map[string]*T) bool {
		sourceSnapshots := make(map[string]*T, len(moves))
		sourceKeys := make(map[string]struct{}, len(moves))
		targetKeys := make(map[string]struct{}, len(moves))
		for _, move := range moves {
			sourceKeys[move.sourceKey] = struct{}{}
			targetKeys[move.targetKey] = struct{}{}
			if _, loaded := sourceSnapshots[move.sourceKey]; !loaded {
				sourceSnapshots[move.sourceKey] = metadata[move.sourceKey]
			}
		}

		changed := false
		for _, move := range moves {
			source := sourceSnapshots[move.sourceKey]
			if source == nil {
				continue
			}
			target := metadata[move.targetKey]
			_, targetIsMovingSource := sourceKeys[move.targetKey]
			if target != nil && !targetIsMovingSource {
				// The destination already has intentional name-scoped metadata.
				// Preserve it rather than attaching a renamed container's metadata
				// to a name that may represent a different logical resource.
				continue
			}
			metadata[move.targetKey] = cloneForTarget(source, move)
			changed = true
		}

		for _, move := range moves {
			if sourceSnapshots[move.sourceKey] == nil {
				continue
			}
			if _, sourceIsAnotherTarget := targetKeys[move.sourceKey]; sourceIsAnotherTarget {
				continue
			}
			delete(metadata, move.sourceKey)
			changed = true
		}
		return changed
	})
}

func (m *Monitor) migrateDockerContainerMetadataForRenamedContainers(
	hostID string,
	previousContainers []models.DockerContainer,
	currentContainers []models.DockerContainer,
) {
	if m == nil {
		return
	}
	hostID = strings.TrimSpace(hostID)
	if hostID == "" || len(previousContainers) == 0 || len(currentContainers) == 0 {
		return
	}

	if m.guestMetadataStore != nil {
		moves := dockerContainerStableMetadataMoves(
			hostID,
			previousContainers,
			currentContainers,
			dockerAppContainerMetadataKey,
		)
		err := applyStableMetadataMoves(
			moves,
			m.guestMetadataStore.UpdateAll,
			func(source *config.GuestMetadata, move dockerStableMetadataMove) *config.GuestMetadata {
				clone := &config.GuestMetadata{
					CustomURL:     source.CustomURL,
					Description:   source.Description,
					LastKnownName: move.targetName,
					LastKnownType: "app-container",
				}
				if len(source.Tags) > 0 {
					clone.Tags = append([]string(nil), source.Tags...)
				}
				if len(source.Notes) > 0 {
					clone.Notes = append([]string(nil), source.Notes...)
				}
				return clone
			},
		)
		if err != nil {
			log.Warn().
				Err(err).
				Str("dockerHostID", hostID).
				Int("renameCount", len(moves)).
				Msg("Failed to move guest metadata after Docker container rename")
		}
	}

	if m.dockerMetadataStore != nil {
		moves := dockerContainerStableMetadataMoves(
			hostID,
			previousContainers,
			currentContainers,
			dockerContainerNameMetadataKey,
		)
		err := applyStableMetadataMoves(
			moves,
			m.dockerMetadataStore.UpdateAll,
			func(source *config.DockerMetadata, _ dockerStableMetadataMove) *config.DockerMetadata {
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
				return clone
			},
		)
		if err != nil {
			log.Warn().
				Err(err).
				Str("dockerHostID", hostID).
				Int("renameCount", len(moves)).
				Msg("Failed to move Docker metadata after container rename")
		}
	}
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
					if err := m.guestMetadataStore.UpdateAll(func(metadata map[string]*config.GuestMetadata) bool {
						if metadata[stableKey] != nil {
							return false
						}
						clone := &config.GuestMetadata{
							ID:            stableKey,
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
						metadata[stableKey] = clone
						return true
					}); err != nil {
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
