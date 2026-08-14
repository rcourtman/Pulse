package alerts

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// MigrateDockerContainerOverrideKeys re-homes container overrides onto the
// stable host+name key and prunes entries whose container no longer exists.
// Container IDs change on every recreate, so an image update orphaned the
// user's per-container alert toggle and left a dead entry in alerts.json
// forever (#1601). Three key shapes exist in the wild:
//
//	docker:{host}/{name}                    stable (target, written by the UI now)
//	docker:{host}/{container ID}            legacy backend key (v5-era writes)
//	docker:{host}/app-container-{16 hex}    unified hash IDs the v6 UI wrote
//
// Live legacy and hash keys are moved onto the stable key; orphaned ID-shaped
// keys are pruned. Only hosts with at least one docker-sourced app-container
// in the snapshot are touched (an empty set can be a transient collection
// failure), and name-keyed overrides for absent containers are kept so a
// later recreate under the same name still honours them. Like
// MigrateCanonicalOverrideKeys, the config is mutated copy-on-write: callers
// hand in a GetConfig() snapshot whose Overrides map is shared with the live
// manager config.
func MigrateDockerContainerOverrideKeys(config *AlertConfig, resources []unifiedresources.Resource) bool {
	if config == nil || len(config.Overrides) == 0 || len(resources) == 0 {
		return false
	}

	changed := false
	overrides := make(map[string]ThresholdConfig, len(config.Overrides))
	for resourceID, override := range config.Overrides {
		overrides[resourceID] = override
	}

	rehome := func(fromKey, toKey string) {
		override, exists := overrides[fromKey]
		if !exists {
			return
		}
		if toKey != "" && toKey != fromKey {
			if _, taken := overrides[toKey]; !taken {
				overrides[toKey] = override
			}
		}
		delete(overrides, fromKey)
		changed = true
	}

	liveKeysByHost := make(map[string]map[string]struct{})
	for _, resource := range resources {
		if unifiedresources.CanonicalResourceType(resource.Type) != unifiedresources.ResourceTypeAppContainer {
			continue
		}
		if resource.Docker == nil {
			continue
		}
		hostID := strings.TrimSpace(resource.Docker.HostSourceID)
		if hostID == "" {
			continue
		}

		live := liveKeysByHost[hostID]
		if live == nil {
			live = make(map[string]struct{})
			liveKeysByHost[hostID] = live
		}

		stableKey := dockerContainerOverrideKey(hostID, resource.Name)
		if stableKey != "" {
			live[stableKey] = struct{}{}
		}

		legacyKey := ""
		if containerID := strings.TrimSpace(resource.Docker.ContainerID); containerID != "" {
			legacyKey = DockerResourceID(hostID, containerID)
			live[legacyKey] = struct{}{}
		}
		if legacyKey != "" && legacyKey != stableKey {
			rehome(legacyKey, stableKey)
		}

		// The v6 thresholds UI keyed container overrides by the unified hash
		// ID, which the evaluator never read. Re-home them while the hash is
		// still resolvable to a live container.
		if resourceID := strings.TrimSpace(resource.ID); resourceID != "" {
			if hashKey := "docker:" + hostID + "/" + resourceID; hashKey != stableKey {
				rehome(hashKey, stableKey)
			}
		}
	}

	for hostID, live := range liveKeysByHost {
		if len(live) == 0 {
			continue
		}
		prefix := "docker:" + hostID + "/"
		for key := range overrides {
			if !strings.HasPrefix(key, prefix) {
				continue
			}
			rest := key[len(prefix):]
			// "service/<name>" keys belong to Swarm services.
			if strings.Contains(rest, "/") {
				continue
			}
			if _, isLive := live[key]; isLive {
				continue
			}
			// A suffix that is neither a container ID nor a unified hash ID
			// is a name-keyed override for a currently absent container; keep
			// it so a recreate under the same name still honours it.
			if !looksLikeDockerContainerID(rest) && !looksLikeUnifiedAppContainerID(rest) {
				continue
			}
			delete(overrides, key)
			changed = true
		}
	}

	if changed {
		config.Overrides = overrides
	}
	return changed
}

// looksLikeDockerContainerID reports whether the value has the shape of a
// Docker container ID (12-char short or 64-char full hex).
func looksLikeDockerContainerID(value string) bool {
	if len(value) != 12 && len(value) != 64 {
		return false
	}
	return isHexString(value)
}

// looksLikeUnifiedAppContainerID reports whether the value has the shape of a
// unified app-container resource ID ("app-container-" + 16 hex chars).
func looksLikeUnifiedAppContainerID(value string) bool {
	const prefix = "app-container-"
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	suffix := value[len(prefix):]
	return len(suffix) == 16 && isHexString(suffix)
}

func isHexString(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}
