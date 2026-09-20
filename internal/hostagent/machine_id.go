package hostagent

import "strings"

// normalizeMachineGUID returns the canonical form of a Windows MachineGuid.
// Windows may report the registry value either bare ("xxxxxxxx-....") or
// wrapped in braces ("{xxxxxxxx-....}"); both must resolve to the same stable
// agent identity, so the optional braces are stripped and the result is
// trimmed and lower-cased.
func normalizeMachineGUID(raw string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(raw), "{}"))
}
