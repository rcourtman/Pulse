package actionlifecycle

import (
	"fmt"
	"strings"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// A host's Pulse agent can fail to take a command for two unrelated reasons,
// and each has a different fix. Commands were never enabled on it: nothing is
// broken, and the owner turns them on with the install command. Or commands
// are enabled and the command connection is down: something needs repair.
// Executors, the AI read tools and the Patrol catalog report these two codes
// so no surface has to guess which one a refusal means.
const (
	ReasonCodeHostCommandsDisabled     = "host_commands_disabled"
	ReasonCodeCommandAgentDisconnected = "command_agent_disconnected"
)

// commandAccessInstallOption is the installer checkbox that grants command
// access. The label lives in the frontend (getUnifiedAgentCommandAccessOptionLabel).
const commandAccessInstallOption = "Allow Pulse to run commands on this host"

// HostCommandsDisabled reports whether a host's own Pulse agent is reporting
// and says command execution is off. An agent that has stopped reporting is
// not judged: its last report cannot say why a command would fail now. Nor is
// a host only an integration reports (TrueNAS, vSphere): those fill the agent
// facet so machine surfaces render, and no install command applies to them.
func HostCommandsDisabled(host *unified.Resource) bool {
	if host == nil || host.Agent == nil || host.Agent.Stale || host.Agent.CommandsEnabled {
		return false
	}
	for _, source := range host.Sources {
		if source == unified.SourceAgent {
			return true
		}
	}
	return false
}

// HostCommandsDisabledReason is the one-sentence fact for a host whose agent
// was installed without command access.
func HostCommandsDisabledReason(hostName string) string {
	return fmt.Sprintf("Commands are not enabled on the Pulse agent on %s.", commandAccessHostName(hostName))
}

// HostCommandsDisabledStep is the one step that turns commands on. Pulse
// cannot do it from the server: a monitoring-only agent sets its command
// ceiling locally at install time and refuses a remote request to raise it.
func HostCommandsDisabledStep(hostName string) string {
	return fmt.Sprintf("To allow them, run the Pulse install command on %s again with %q ticked.", commandAccessHostName(hostName), commandAccessInstallOption)
}

// HostCommandsDisabledRefusal is the reason an action or read refusal shows:
// the fact, then the step.
func HostCommandsDisabledRefusal(hostName string) string {
	return HostCommandsDisabledReason(hostName) + " " + HostCommandsDisabledStep(hostName)
}

func commandAccessHostName(hostName string) string {
	if hostName = strings.TrimSpace(hostName); hostName != "" {
		return hostName
	}
	return "this host"
}
