package agentcapabilities

// ControlLevel is the shared Pulse Intelligence permission level for governed
// tool/action surfaces. Assistant settings, Assistant tool availability, and
// external-agent adapters must use this vocabulary rather than local strings.
type ControlLevel string

const (
	// ControlLevelReadOnly allows read/query tools only.
	ControlLevelReadOnly ControlLevel = "read_only"
	// ControlLevelControlled allows control tools through approval-backed paths.
	ControlLevelControlled ControlLevel = "controlled"
	// ControlLevelAutonomous allows eligible control tools without per-command approval.
	ControlLevelAutonomous ControlLevel = "autonomous"
)

// NormalizeControlLevel fails closed to read_only for unset or unknown values.
func NormalizeControlLevel(level string) ControlLevel {
	switch ControlLevel(level) {
	case ControlLevelReadOnly, ControlLevelControlled, ControlLevelAutonomous:
		return ControlLevel(level)
	default:
		return ControlLevelReadOnly
	}
}

// IsValidControlLevel reports whether a value is in the stable shared
// vocabulary. Empty and legacy values are invalid and should be normalized by
// callers that need a runtime setting.
func IsValidControlLevel(level string) bool {
	switch ControlLevel(level) {
	case ControlLevelReadOnly, ControlLevelControlled, ControlLevelAutonomous:
		return true
	default:
		return false
	}
}

// ControlLevelAllowsControlTools reports whether the normalized level may expose
// tools that can change Pulse or target-side state. Unknown levels fail closed.
func ControlLevelAllowsControlTools(level ControlLevel) bool {
	switch NormalizeControlLevel(string(level)) {
	case ControlLevelControlled, ControlLevelAutonomous:
		return true
	default:
		return false
	}
}
