package licensing

import "time"

// LegacyAliases maps old feature/capability names to their canonical replacements.
// When HasCapability receives an aliased key, it checks for the canonical key.
// Keep this empty until a real wire-level rename ships; speculative aliases
// become part of the compatibility contract and are costly to remove.
var LegacyAliases = map[string]string{
	// Example aliases for an actual migration:
	// "old_feature_name": "new_feature_name",
}

// DeprecatedCapability tracks a capability that is being phased out.
type DeprecatedCapability struct {
	// ReplacementKey is the canonical key that replaces this capability.
	ReplacementKey string

	// SunsetAt is the date after which this capability key will stop working.
	SunsetAt time.Time
}

// DeprecatedCapabilities maps capability keys to their deprecation metadata.
// Evaluator logs warnings when deprecated keys are checked.
// Keep this empty until an actual deprecation window is defined in product and
// migration plans.
var DeprecatedCapabilities = map[string]DeprecatedCapability{
	// Example deprecations for an actual migration:
	// "old_capability": {ReplacementKey: "new_capability", SunsetAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
}

// ResolveAlias returns the canonical key for the given key.
// If the key has a legacy alias, returns the alias target.
// Otherwise returns the key unchanged.
func ResolveAlias(key string) string {
	if canonical, ok := LegacyAliases[key]; ok {
		return canonical
	}
	return key
}

// IsDeprecated checks if a capability key is deprecated and returns the metadata.
func IsDeprecated(key string) (DeprecatedCapability, bool) {
	dep, ok := DeprecatedCapabilities[key]
	return dep, ok
}
