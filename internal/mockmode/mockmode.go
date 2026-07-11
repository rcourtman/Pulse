package mockmode

import (
	"os"
	"strings"
)

// IsRequestedFromEnv reports whether the operator asked for mock fixtures via
// PULSE_MOCK_MODE, regardless of whether the current build has authorized and
// enabled them yet. Never use this as an enablement gate: release builds must
// keep failing closed until the demo_fixtures entitlement authorizes fixtures.
// It exists for boot-time lifecycle decisions that must survive the release
// demo ordering, where fixtures enable only after the license sync runs.
func IsRequestedFromEnv() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("PULSE_MOCK_MODE")), "true")
}
