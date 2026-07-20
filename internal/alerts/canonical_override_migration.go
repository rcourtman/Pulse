package alerts

import "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"

// MigrateCanonicalOverrideKeys re-homes overrides written under a retired
// canonical resource ID onto the resource's current canonical ID. Only
// provider-declared superseded IDs participate: display aliases, hostnames,
// metric targets, and other lookup conveniences are never persistence keys.
//
// A current-key override wins over an old-key override, and an old ID is left
// untouched when it is still live or maps to more than one current resource.
// The function mutates config only when it can make an unambiguous migration.
func MigrateCanonicalOverrideKeys(config *AlertConfig, resources []unifiedresources.Resource) bool {
	if config == nil || len(config.Overrides) == 0 || len(resources) == 0 {
		return false
	}

	liveIDs := make(map[string]struct{}, len(resources))
	successors := make(map[string]string)
	ambiguous := make(map[string]struct{})

	for _, resource := range resources {
		currentID := unifiedresources.CanonicalResourceID(resource.ID)
		if currentID == "" {
			continue
		}
		liveIDs[currentID] = struct{}{}

		for _, supersededID := range resource.SupersededCanonicalIDs {
			oldID := unifiedresources.CanonicalResourceID(supersededID)
			if oldID == "" || oldID == currentID {
				continue
			}
			if existing, ok := successors[oldID]; ok && existing != currentID {
				delete(successors, oldID)
				ambiguous[oldID] = struct{}{}
				continue
			}
			if _, conflict := ambiguous[oldID]; conflict {
				continue
			}
			successors[oldID] = currentID
		}
	}

	changed := false
	overrides := make(map[string]ThresholdConfig, len(config.Overrides))
	for resourceID, override := range config.Overrides {
		overrides[resourceID] = override
	}
	for oldID, newID := range successors {
		if _, stillLive := liveIDs[oldID]; stillLive {
			continue
		}
		override, exists := overrides[oldID]
		if !exists {
			continue
		}
		if _, currentExists := overrides[newID]; !currentExists {
			overrides[newID] = override
		}
		delete(overrides, oldID)
		changed = true
	}
	if changed {
		config.Overrides = overrides
	}
	return changed
}
