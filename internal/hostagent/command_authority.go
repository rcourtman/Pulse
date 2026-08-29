package hostagent

import (
	"fmt"
	"strings"
)

// CommandAuthorityProfile is the local, install-time ceiling for command
// execution. It is intentionally independent of the server's desired command
// state so remote configuration cannot widen a monitoring-only process.
type CommandAuthorityProfile string

const (
	// CommandAuthorityLegacy preserves update continuity for installations
	// created before an explicit local authority marker existed. It may follow
	// the server's desired command state and is not a least-privilege claim.
	CommandAuthorityLegacy CommandAuthorityProfile = "legacy"
	// CommandAuthorityMonitoringOnly permanently disables command execution for
	// the lifetime of the process, regardless of server configuration.
	CommandAuthorityMonitoringOnly CommandAuthorityProfile = "monitoring-only"
	// CommandAuthorityCommandCapable permits the server to disable and re-enable
	// command execution because the operator provisioned that authority locally.
	CommandAuthorityCommandCapable CommandAuthorityProfile = "command-capable"
)

// NormalizeCommandAuthorityProfile validates the local profile. An absent
// marker is deliberately legacy for upgrade compatibility; fresh installers
// always write an explicit profile.
func NormalizeCommandAuthorityProfile(raw string) (CommandAuthorityProfile, error) {
	switch CommandAuthorityProfile(strings.ToLower(strings.TrimSpace(raw))) {
	case "", CommandAuthorityLegacy:
		return CommandAuthorityLegacy, nil
	case CommandAuthorityMonitoringOnly:
		return CommandAuthorityMonitoringOnly, nil
	case CommandAuthorityCommandCapable:
		return CommandAuthorityCommandCapable, nil
	default:
		return "", fmt.Errorf("invalid command authority profile %q", raw)
	}
}

// ResolveCommandAuthority applies a desired command state within the local
// profile. accepted is false only when a monitoring-only profile rejects a
// request to enable commands.
func ResolveCommandAuthority(profile CommandAuthorityProfile, desired bool) (effective bool, accepted bool) {
	if profile == CommandAuthorityMonitoringOnly && desired {
		return false, false
	}
	return desired, true
}
