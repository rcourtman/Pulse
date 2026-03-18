package entitlements

import pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"

// LegacyAliases maps old feature/capability names to their canonical replacements.
// When HasCapability receives an aliased key, it checks for the canonical key.
var LegacyAliases = pkglicensing.LegacyAliases

// DeprecatedCapability tracks a capability that is being phased out.
type DeprecatedCapability = pkglicensing.DeprecatedCapability

// DeprecatedCapabilities maps capability keys to their deprecation metadata.
// Evaluator logs warnings when deprecated keys are checked.
var DeprecatedCapabilities = pkglicensing.DeprecatedCapabilities

// ResolveAlias returns the canonical key for the given key.
// If the key has a legacy alias, returns the alias target.
// Otherwise returns the key unchanged.
func ResolveAlias(key string) string {
	return pkglicensing.ResolveAlias(key)
}

// IsDeprecated checks if a capability key is deprecated and returns the metadata.
func IsDeprecated(key string) (DeprecatedCapability, bool) {
	return pkglicensing.IsDeprecated(key)
}
