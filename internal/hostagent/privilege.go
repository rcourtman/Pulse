package hostagent

import (
	"os"
	"os/user"
	"strings"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// collectPrivilegeStatus reports the privilege the agent actually runs with.
// The values are facts about this process, not configuration: effective uid,
// the service account name, and whether the scoped privilege-helper overrides
// a least-privilege install configures are in effect. On Windows Geteuid is
// always -1, so RunningAsRoot stays false and ServiceUser carries the account
// name; the server renders the profile descriptively rather than judging it.
func collectPrivilegeStatus(commandAuthority CommandAuthorityProfile) *agentshost.PrivilegeStatus {
	status := &agentshost.PrivilegeStatus{
		RunningAsRoot:    os.Geteuid() == 0,
		CommandAuthority: string(commandAuthority),
		SmartctlHelper:   strings.TrimSpace(os.Getenv("PULSE_SMARTCTL_PATH")) != "",
		PctHelper:        strings.TrimSpace(os.Getenv("PULSE_PCT_PATH")) != "",
	}
	if current, err := user.Current(); err == nil {
		status.ServiceUser = strings.TrimSpace(current.Username)
	}
	return status
}
