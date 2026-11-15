package auth

import (
	"maps"
	"slices"
	"strconv"
	"strings"
)

// savedPermissions preserves complete rules, not authorization decisions.
// Only identical rules are deduplicated; conditions and the literal effect
// must never be discarded just because another rule has the same target.
func savedPermissions(roles []Role) []Permission {
	type identity struct {
		action, resource, effect, conditions string
	}
	seen := make(map[identity]bool)
	permissions := make([]Permission, 0)
	for _, role := range roles {
		for _, permission := range role.Permissions {
			var conditions strings.Builder
			for _, key := range slices.Sorted(maps.Keys(permission.Conditions)) {
				// Quoting is lossless even for non-UTF-8 Go strings, and keeps
				// delimiters inside a key/value from colliding with other rules.
				conditions.WriteString(strconv.Quote(key))
				conditions.WriteByte(':')
				conditions.WriteString(strconv.Quote(permission.Conditions[key]))
				conditions.WriteByte(';')
			}
			key := identity{permission.Action, permission.Resource, permission.Effect, conditions.String()}
			if seen[key] {
				continue
			}
			seen[key] = true
			permission.Conditions = maps.Clone(permission.Conditions)
			permissions = append(permissions, permission)
		}
	}
	return permissions
}
