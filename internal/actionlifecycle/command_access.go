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

// A Proxmox guest action runs one of two ways. The action runner on the
// guest's node runs all four: start, stop, shutdown and reboot. A Pulse agent
// with commands enabled runs start and nothing else, after an admin approves
// it. So a refused guest action has one of these reasons, and each has a
// different thing for the owner to do.
const (
	// ReasonCodeActionRunnerUnavailable refuses stop, shutdown or reboot on a
	// node with no connected action runner.
	ReasonCodeActionRunnerUnavailable = "action_runner_unavailable"
	// ReasonCodeGuestStartAgentNotConnected refuses a start on a node where no
	// Pulse agent holds a command session and none reports commands off.
	ReasonCodeGuestStartAgentNotConnected = "guest_start_agent_not_connected"
	// ReasonCodeGuestStartAgentUnconfirmed refuses a start on a node that has
	// a Pulse agent tied to it which Pulse cannot show runs on that node.
	ReasonCodeGuestStartAgentUnconfirmed = "guest_start_agent_unconfirmed"
	// ReasonCodeGuestStartAgentUpdateRequired refuses a start on a node whose
	// agent predates guest start.
	ReasonCodeGuestStartAgentUpdateRequired = "guest_start_agent_update_required"
	// ReasonCodeGuestStartUnavailable refuses a start on a node whose agent
	// knows guest start and reports that it cannot offer it there.
	ReasonCodeGuestStartUnavailable = "guest_start_unavailable"
)

// IsProxmoxGuestActionUnavailableReasonCode reports whether a refusal says
// Pulse cannot run this guest action on this node as the node is set up. A
// commands-off node is not one of these: it has its own code and its own step.
func IsProxmoxGuestActionUnavailableReasonCode(code string) bool {
	switch strings.TrimSpace(code) {
	case ReasonCodeActionRunnerUnavailable, ReasonCodeGuestStartAgentNotConnected, ReasonCodeGuestStartAgentUnconfirmed,
		ReasonCodeGuestStartAgentUpdateRequired, ReasonCodeGuestStartUnavailable:
		return true
	default:
		return false
	}
}

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

// ProxmoxGuestRunnerOnlyReason is what a refused stop, shutdown or reboot
// shows on a node with no action runner. It says the install option covers
// starting only, because a person who ticked it reasonably expects more. It
// gives no step that connects a runner: today that is a migration of the
// node's agent that turns command access off and needs HTTPS to Pulse.
func ProxmoxGuestRunnerOnlyReason(nodeName string) string {
	return fmt.Sprintf("Pulse cannot stop or restart guests on %s: that needs the Pulse action runner on that node. %q covers starting a stopped guest only. Use Proxmox for this.", proxmoxGuestNodeName(nodeName), commandAccessInstallOption)
}

// ProxmoxGuestStartAgentNotConnectedReason is what a refused start shows when
// no Pulse agent on the node holds a command session.
func ProxmoxGuestStartAgentNotConnectedReason(nodeName string) string {
	return fmt.Sprintf("Pulse cannot start guests on %s: no Pulse agent that accepts commands is connected on that node. Use Proxmox for this.", proxmoxGuestNodeName(nodeName))
}

// ProxmoxGuestStartAgentUnconfirmedReason is what a refused start shows when a
// Pulse agent is tied to the node and Pulse cannot show it runs there. Pulse
// ties an agent to a node by name when nothing contradicts it. That is enough
// to show the two together and not enough to start a guest: another machine
// can hold the name, and the same VMID there is a different guest.
func ProxmoxGuestStartAgentUnconfirmedReason(nodeName string) string {
	return fmt.Sprintf("Pulse cannot start guests on %s: it cannot confirm which Pulse agent runs on that node. It needs an address that the agent reports and only that node has. Use Proxmox for this.", proxmoxGuestNodeName(nodeName))
}

// ProxmoxGuestStartAgentUpdateReason is what a refused start shows when the
// node's agent accepts commands and is too old to start a guest.
func ProxmoxGuestStartAgentUpdateReason(nodeName string) string {
	nodeName = proxmoxGuestNodeName(nodeName)
	return fmt.Sprintf("Pulse cannot start guests on %s yet: the Pulse agent on that node is too old. Update the Pulse agent on %s, then try again.", nodeName, nodeName)
}

// ProxmoxGuestStartNotSystemdReason is what a refused start shows when the
// node's agent does not run as its own systemd service. Pulse runs a start in
// a unit tied to that service, so there is nothing to tie it to.
func ProxmoxGuestStartNotSystemdReason(nodeName string) string {
	return fmt.Sprintf("Pulse cannot start guests on %s: the Pulse agent on that node does not run as the pulse-agent systemd service, and Pulse starts a guest only through one that does. Use Proxmox for this.", proxmoxGuestNodeName(nodeName))
}

// ProxmoxGuestStartContainmentReason is what a refused start shows when the
// node's agent tried to set up the contained way it runs a start and failed.
func ProxmoxGuestStartContainmentReason(nodeName string) string {
	return fmt.Sprintf("Pulse cannot start guests on %s: the Pulse agent on that node could not set up the contained way it runs a start. Its log says why. Use Proxmox for this.", proxmoxGuestNodeName(nodeName))
}

func proxmoxGuestNodeName(nodeName string) string {
	if nodeName = strings.TrimSpace(nodeName); nodeName != "" {
		return nodeName
	}
	return "this node"
}

func commandAccessHostName(hostName string) string {
	if hostName = strings.TrimSpace(hostName); hostName != "" {
		return hostName
	}
	return "this host"
}
