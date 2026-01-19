package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// registerControlTools registers control tools (conditional on control level)
func (e *PulseToolExecutor) registerControlTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_run_command",
			Description: `Execute a shell command on infrastructure via a connected agent.

Returns: Command output (stdout/stderr) and exit code. Exit code 0 = success.

Use when: User explicitly asks to run a command, or you need to investigate something inside a system.

Do NOT use for: Checking if something is running (use pulse_get_topology), or starting/stopping VMs/containers (use pulse_control_guest or pulse_control_docker).

Important: Commands run on the HOST, not inside VMs/containers. To run inside an LXC, use: pct exec <vmid> -- <command>`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"command": {
						Type:        "string",
						Description: "Shell command to execute (runs via sh -c)",
					},
					"run_on_host": {
						Type:        "boolean",
						Description: "If true, run on host. If false/omitted, runs on host anyway (use pct exec for LXC internals)",
					},
					"target_host": {
						Type:        "string",
						Description: "Hostname to run on (e.g. 'delly'). Required if multiple agents connected.",
					},
				},
				Required: []string{"command"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeRunCommand(ctx, args)
		},
		RequireControl: true,
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_control_guest",
			Description: `Start, stop, or restart Proxmox VMs and LXC containers.

Returns: Success message with VM/container name, or error if failed.

Use when: User asks to start, stop, restart, or shutdown a VM or LXC container.

Do NOT use for: Docker containers (use pulse_control_docker), or checking status (use pulse_get_topology).

Note: These are LXC containers managed by Proxmox, NOT Docker containers. Uses 'pct' commands internally.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"guest_id": {
						Type:        "string",
						Description: "VMID number (e.g. '101') or exact name of the VM/LXC container",
					},
					"action": {
						Type:        "string",
						Description: "start, stop (immediate), shutdown (graceful), or restart",
						Enum:        []string{"start", "stop", "shutdown", "restart"},
					},
					"force": {
						Type:        "boolean",
						Description: "Force stop without graceful shutdown (rarely needed)",
					},
				},
				Required: []string{"guest_id", "action"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeControlGuest(ctx, args)
		},
		RequireControl: true,
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_control_docker",
			Description: `Start, stop, or restart Docker containers.

Returns: Success message with container name and host, or error if failed.

Use when: User asks to start, stop, or restart a Docker container.

Do NOT use for: Proxmox VMs/LXC containers (use pulse_control_guest), or checking status (use pulse_get_topology).

Note: These are Docker containers, not Proxmox LXC containers. Uses 'docker' commands internally.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"container": {
						Type:        "string",
						Description: "Container name (e.g. 'nginx') or container ID",
					},
					"host": {
						Type:        "string",
						Description: "Docker host name (e.g. 'Tower'). Required if multiple Docker hosts exist.",
					},
					"action": {
						Type:        "string",
						Description: "start, stop, or restart",
						Enum:        []string{"start", "stop", "restart"},
					},
				},
				Required: []string{"container", "action"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeControlDocker(ctx, args)
		},
		RequireControl: true,
	})
}

func (e *PulseToolExecutor) executeRunCommand(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	command, _ := args["command"].(string)
	runOnHost, _ := args["run_on_host"].(bool)
	targetHost, _ := args["target_host"].(string)

	if command == "" {
		return NewErrorResult(fmt.Errorf("command is required")), nil
	}

	// Note: Control level read_only check is now centralized in registry.Execute()

	// Check if this is a pre-approved execution (agentic loop re-executing after user approval)
	preApproved := isPreApproved(args)

	// Check security policy (skip block check - blocks cannot be pre-approved)
	decision := agentexec.PolicyAllow
	if e.policy != nil {
		decision = e.policy.Evaluate(command)
		if decision == agentexec.PolicyBlock {
			return NewTextResult(formatPolicyBlocked(command, "This command is blocked by security policy")), nil
		}
	}

	if e.controlLevel == ControlLevelSuggest {
		return NewTextResult(formatCommandSuggestion(command, runOnHost, targetHost)), nil
	}

	// Skip approval checks if pre-approved
	if !preApproved && e.controlLevel == ControlLevelControlled {
		targetType := "container"
		if runOnHost {
			targetType = "host"
		}
		approvalID := createApprovalRecord(command, targetType, e.targetID, targetHost, "Control level requires approval")
		return NewTextResult(formatApprovalNeeded(command, "Control level requires approval", approvalID)), nil
	}
	if !preApproved && decision == agentexec.PolicyRequireApproval && !e.isAutonomous {
		targetType := "container"
		if runOnHost {
			targetType = "host"
		}
		approvalID := createApprovalRecord(command, targetType, e.targetID, targetHost, "Security policy requires approval")
		return NewTextResult(formatApprovalNeeded(command, "Security policy requires approval", approvalID)), nil
	}

	// Execute via agent server
	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	agentID := e.findAgentForCommand(runOnHost, targetHost)
	if agentID == "" {
		return NewErrorResult(fmt.Errorf("no agent available for target")), nil
	}

	targetType := "container"
	if runOnHost {
		targetType = "host"
	}

	result, err := e.agentServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		Command:    command,
		TargetType: targetType,
		TargetID:   e.targetID,
	})
	if err != nil {
		return NewErrorResult(err), nil
	}

	output := result.Stdout
	if result.Stderr != "" {
		output += "\n" + result.Stderr
	}
	if result.ExitCode != 0 {
		return NewTextResult(fmt.Sprintf("Command failed (exit code %d):\n%s", result.ExitCode, output)), nil
	}

	// Success - include guidance to prevent unnecessary verification
	if output == "" {
		return NewTextResult("✓ Command completed successfully (exit code 0). No verification needed."), nil
	}
	return NewTextResult(fmt.Sprintf("✓ Command completed successfully (exit code 0). No verification needed.\n%s", output)), nil
}

func (e *PulseToolExecutor) executeControlGuest(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	guestID, _ := args["guest_id"].(string)
	action, _ := args["action"].(string)
	force, _ := args["force"].(bool)

	if guestID == "" {
		return NewErrorResult(fmt.Errorf("guest_id is required")), nil
	}
	if action == "" {
		return NewErrorResult(fmt.Errorf("action is required")), nil
	}

	validActions := map[string]bool{"start": true, "stop": true, "shutdown": true, "restart": true}
	if !validActions[action] {
		return NewErrorResult(fmt.Errorf("invalid action: %s. Use start, stop, shutdown, or restart", action)), nil
	}

	// Note: Control level read_only check is now centralized in registry.Execute()

	guest, err := e.resolveGuest(guestID)
	if err != nil {
		return NewTextResult(fmt.Sprintf("Could not find guest '%s': %v", guestID, err)), nil
	}

	// Check if guest is protected
	vmidStr := fmt.Sprintf("%d", guest.VMID)
	for _, protected := range e.protectedGuests {
		if protected == vmidStr || protected == guest.Name {
			return NewTextResult(fmt.Sprintf("Guest %s (VMID %d) is protected and cannot be controlled by AI.", guest.Name, guest.VMID)), nil
		}
	}

	// Build the command
	cmdTool := "pct"
	if guest.Type == "vm" {
		cmdTool = "qm"
	}

	var command string
	switch action {
	case "start":
		command = fmt.Sprintf("%s start %d", cmdTool, guest.VMID)
	case "stop":
		command = fmt.Sprintf("%s stop %d", cmdTool, guest.VMID)
	case "shutdown":
		command = fmt.Sprintf("%s shutdown %d", cmdTool, guest.VMID)
	case "restart":
		command = fmt.Sprintf("%s reboot %d", cmdTool, guest.VMID)
	}

	if force && action == "stop" {
		command = fmt.Sprintf("%s stop %d --skiplock", cmdTool, guest.VMID)
	}

	// Check if this is a pre-approved execution (agentic loop re-executing after user approval)
	preApproved := isPreApproved(args)

	// Check security policy (skip if pre-approved)
	if !preApproved && e.policy != nil {
		decision := e.policy.Evaluate(command)
		if decision == agentexec.PolicyBlock {
			return NewTextResult(formatPolicyBlocked(command, "This command is blocked by security policy")), nil
		}
		if decision == agentexec.PolicyRequireApproval && !e.isAutonomous {
			// Use guest.Node (the Proxmox host) as targetName so approval execution can find the correct agent
			approvalID := createApprovalRecord(command, guest.Type, fmt.Sprintf("%d", guest.VMID), guest.Node, fmt.Sprintf("%s guest %s", action, guest.Name))
			return NewTextResult(formatControlApprovalNeeded(guest.Name, guest.VMID, action, command, approvalID)), nil
		}
	}

	// Check control level - this must be outside policy check since policy may be nil (skip if pre-approved)
	if !preApproved && e.controlLevel == ControlLevelControlled {
		// Use guest.Node (the Proxmox host) as targetName so approval execution can find the correct agent
		approvalID := createApprovalRecord(command, guest.Type, fmt.Sprintf("%d", guest.VMID), guest.Node, fmt.Sprintf("%s guest %s", action, guest.Name))
		return NewTextResult(formatControlApprovalNeeded(guest.Name, guest.VMID, action, command, approvalID)), nil
	}

	if e.controlLevel == ControlLevelSuggest {
		return NewTextResult(formatControlSuggestion(guest.Name, guest.VMID, action, command, guest.Node)), nil
	}

	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	agentID := e.findAgentForNode(guest.Node)
	if agentID == "" {
		return NewTextResult(fmt.Sprintf("No agent available on node '%s'. Install Pulse Unified Agent on the Proxmox host to enable control.", guest.Node)), nil
	}

	result, err := e.agentServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		Command:    command,
		TargetType: "host",
		TargetID:   "",
	})
	if err != nil {
		return NewErrorResult(err), nil
	}

	output := result.Stdout
	if result.Stderr != "" {
		output += "\n" + result.Stderr
	}

	if result.ExitCode == 0 {
		return NewTextResult(fmt.Sprintf("✓ Successfully executed '%s' on %s (VMID %d). Action complete - no verification needed (state updates in ~10s).\n%s", action, guest.Name, guest.VMID, output)), nil
	}

	return NewTextResult(fmt.Sprintf("Command failed (exit code %d):\n%s", result.ExitCode, output)), nil
}

func (e *PulseToolExecutor) executeControlDocker(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	containerName, _ := args["container"].(string)
	hostName, _ := args["host"].(string)
	action, _ := args["action"].(string)

	if containerName == "" {
		return NewErrorResult(fmt.Errorf("container name is required")), nil
	}
	if action == "" {
		return NewErrorResult(fmt.Errorf("action is required")), nil
	}

	validActions := map[string]bool{"start": true, "stop": true, "restart": true}
	if !validActions[action] {
		return NewErrorResult(fmt.Errorf("invalid action: %s. Use start, stop, or restart", action)), nil
	}

	// Note: Control level read_only check is now centralized in registry.Execute()

	// Check if this is a pre-approved execution (agentic loop re-executing after user approval)
	preApproved := isPreApproved(args)

	container, dockerHost, err := e.resolveDockerContainer(containerName, hostName)
	if err != nil {
		return NewTextResult(fmt.Sprintf("Could not find Docker container '%s': %v", containerName, err)), nil
	}

	command := fmt.Sprintf("docker %s %s", action, container.Name)

	// Get the agent hostname for approval records (may differ from docker host display name)
	agentHostname := e.getAgentHostnameForDockerHost(dockerHost)

	// Skip approval checks if pre-approved
	if !preApproved && e.policy != nil {
		decision := e.policy.Evaluate(command)
		if decision == agentexec.PolicyBlock {
			return NewTextResult(formatPolicyBlocked(command, "This command is blocked by security policy")), nil
		}
		if decision == agentexec.PolicyRequireApproval && !e.isAutonomous {
			approvalID := createApprovalRecord(command, "docker", container.Name, agentHostname, fmt.Sprintf("%s Docker container %s", action, container.Name))
			return NewTextResult(formatDockerApprovalNeeded(container.Name, dockerHost.Hostname, action, command, approvalID)), nil
		}
	}

	// Check control level - this must be outside policy check since policy may be nil (skip if pre-approved)
	if !preApproved && e.controlLevel == ControlLevelControlled {
		approvalID := createApprovalRecord(command, "docker", container.Name, agentHostname, fmt.Sprintf("%s Docker container %s", action, container.Name))
		return NewTextResult(formatDockerApprovalNeeded(container.Name, dockerHost.Hostname, action, command, approvalID)), nil
	}

	if e.controlLevel == ControlLevelSuggest {
		return NewTextResult(formatDockerSuggestion(container.Name, dockerHost.Hostname, action, command)), nil
	}

	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	agentID := e.findAgentForDockerHost(dockerHost)
	if agentID == "" {
		return NewTextResult(fmt.Sprintf("No agent available on Docker host '%s'. Install Pulse Unified Agent on the host to enable control.", dockerHost.Hostname)), nil
	}

	result, err := e.agentServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		Command:    command,
		TargetType: "host",
		TargetID:   "",
	})
	if err != nil {
		return NewErrorResult(err), nil
	}

	output := result.Stdout
	if result.Stderr != "" {
		output += "\n" + result.Stderr
	}

	if result.ExitCode == 0 {
		return NewTextResult(fmt.Sprintf("✓ Successfully executed 'docker %s' on container '%s' (host: %s). Action complete - no verification needed (state updates in ~10s).\n%s", action, container.Name, dockerHost.Hostname, output)), nil
	}

	return NewTextResult(fmt.Sprintf("Command failed (exit code %d):\n%s", result.ExitCode, output)), nil
}

// Helper methods for control tools

func (e *PulseToolExecutor) findAgentForCommand(runOnHost bool, targetHost string) string {
	if e.agentServer == nil {
		return ""
	}

	agents := e.agentServer.GetConnectedAgents()
	if len(agents) == 0 {
		return ""
	}

	if targetHost != "" {
		for _, agent := range agents {
			if agent.Hostname == targetHost || agent.AgentID == targetHost {
				return agent.AgentID
			}
		}
	}

	return agents[0].AgentID
}

func (e *PulseToolExecutor) resolveGuest(guestID string) (*GuestInfo, error) {
	if e.stateProvider == nil {
		return nil, fmt.Errorf("state provider not available")
	}

	state := e.stateProvider.GetState()
	vmid, err := strconv.Atoi(guestID)

	for _, vm := range state.VMs {
		if (err == nil && vm.VMID == vmid) || vm.Name == guestID || vm.ID == guestID {
			return &GuestInfo{
				VMID:     vm.VMID,
				Name:     vm.Name,
				Node:     vm.Node,
				Type:     "vm",
				Status:   vm.Status,
				Instance: vm.Instance,
			}, nil
		}
	}

	for _, ct := range state.Containers {
		if (err == nil && ct.VMID == vmid) || ct.Name == guestID || ct.ID == guestID {
			return &GuestInfo{
				VMID:     ct.VMID,
				Name:     ct.Name,
				Node:     ct.Node,
				Type:     "lxc",
				Status:   ct.Status,
				Instance: ct.Instance,
			}, nil
		}
	}

	return nil, fmt.Errorf("no VM or container found with ID or name '%s'", guestID)
}

func (e *PulseToolExecutor) resolveDockerContainer(containerName, hostName string) (*models.DockerContainer, *models.DockerHost, error) {
	if e.stateProvider == nil {
		return nil, nil, fmt.Errorf("state provider not available")
	}

	state := e.stateProvider.GetState()

	for _, host := range state.DockerHosts {
		if hostName != "" && host.Hostname != hostName && host.DisplayName != hostName {
			continue
		}

		for i, container := range host.Containers {
			if container.Name == containerName ||
				container.ID == containerName ||
				strings.HasPrefix(container.ID, containerName) {
				return &host.Containers[i], &host, nil
			}
		}
	}

	if hostName != "" {
		return nil, nil, fmt.Errorf("container '%s' not found on host '%s'", containerName, hostName)
	}
	return nil, nil, fmt.Errorf("container '%s' not found on any Docker host", containerName)
}

func (e *PulseToolExecutor) findAgentForNode(nodeName string) string {
	if e.agentServer == nil {
		return ""
	}

	agents := e.agentServer.GetConnectedAgents()
	for _, agent := range agents {
		if agent.Hostname == nodeName {
			return agent.AgentID
		}
	}

	if e.stateProvider != nil {
		state := e.stateProvider.GetState()
		for _, host := range state.Hosts {
			if host.LinkedNodeID != "" {
				for _, node := range state.Nodes {
					if node.ID == host.LinkedNodeID && node.Name == nodeName {
						for _, agent := range agents {
							if agent.Hostname == host.Hostname || agent.AgentID == host.ID {
								return agent.AgentID
							}
						}
					}
				}
			}
		}
	}

	return ""
}

func (e *PulseToolExecutor) findAgentForDockerHost(dockerHost *models.DockerHost) string {
	if e.agentServer == nil {
		return ""
	}

	agents := e.agentServer.GetConnectedAgents()

	// First try to match by AgentID (most reliable)
	if dockerHost.AgentID != "" {
		for _, agent := range agents {
			if agent.AgentID == dockerHost.AgentID {
				return agent.AgentID
			}
		}
	}

	// Fall back to hostname match
	for _, agent := range agents {
		if agent.Hostname == dockerHost.Hostname {
			return agent.AgentID
		}
	}

	return ""
}

// getAgentHostnameForDockerHost finds the agent hostname for a Docker host (for approval records)
func (e *PulseToolExecutor) getAgentHostnameForDockerHost(dockerHost *models.DockerHost) string {
	if e.agentServer == nil {
		return dockerHost.Hostname // fallback
	}

	agents := e.agentServer.GetConnectedAgents()

	// Try to match by AgentID first
	if dockerHost.AgentID != "" {
		for _, agent := range agents {
			if agent.AgentID == dockerHost.AgentID {
				return agent.Hostname
			}
		}
	}

	// Fall back to the docker host's hostname
	return dockerHost.Hostname
}

// createApprovalRecord creates an approval record in the store and returns the approval ID.
// Returns empty string if store is not available (approvals will still work, just without persistence).
func createApprovalRecord(command, targetType, targetID, targetName, context string) string {
	store := approval.GetStore()
	if store == nil {
		log.Debug().Msg("Approval store not available, approval will not be persisted")
		return ""
	}

	req := &approval.ApprovalRequest{
		Command:    command,
		TargetType: targetType,
		TargetID:   targetID,
		TargetName: targetName,
		Context:    context,
	}

	if err := store.CreateApproval(req); err != nil {
		log.Warn().Err(err).Msg("Failed to create approval record")
		return ""
	}

	log.Debug().Str("approval_id", req.ID).Str("command", command).Msg("Created approval record")
	return req.ID
}

// isPreApproved checks if the args contain a valid, approved approval_id.
// This is used when the agentic loop re-executes a tool after user approval.
func isPreApproved(args map[string]interface{}) bool {
	approvalID, ok := args["_approval_id"].(string)
	if !ok || approvalID == "" {
		return false
	}

	store := approval.GetStore()
	if store == nil {
		return false
	}

	req, found := store.GetApproval(approvalID)
	if !found {
		log.Debug().Str("approval_id", approvalID).Msg("Pre-approval check: approval not found")
		return false
	}

	if req.Status == approval.StatusApproved {
		log.Debug().Str("approval_id", approvalID).Msg("Pre-approval check: approved, skipping approval flow")
		return true
	}

	log.Debug().Str("approval_id", approvalID).Str("status", string(req.Status)).Msg("Pre-approval check: not approved")
	return false
}

// Formatting helpers for control tools

func formatApprovalNeeded(command, reason, approvalID string) string {
	payload := map[string]interface{}{
		"type":           "approval_required",
		"approval_id":    approvalID,
		"command":        command,
		"reason":         reason,
		"how_to_approve": "Click the approval button in the chat to execute this command.",
		"do_not_retry":   true,
	}
	b, _ := json.Marshal(payload)
	return "APPROVAL_REQUIRED: " + string(b)
}

func formatPolicyBlocked(command, reason string) string {
	payload := map[string]interface{}{
		"type":         "policy_blocked",
		"command":      command,
		"reason":       reason,
		"do_not_retry": true,
	}
	b, _ := json.Marshal(payload)
	return "POLICY_BLOCKED: " + string(b)
}

func formatCommandSuggestion(command string, runOnHost bool, targetHost string) string {
	target := "current target"
	if runOnHost {
		target = "host"
	}
	if strings.TrimSpace(targetHost) != "" {
		target = fmt.Sprintf("host %s", targetHost)
	}
	return fmt.Sprintf("Suggested command for %s:\n%s", target, command)
}

func formatControlApprovalNeeded(name string, vmid int, action, command, approvalID string) string {
	payload := map[string]interface{}{
		"type":           "approval_required",
		"approval_id":    approvalID,
		"guest_name":     name,
		"guest_vmid":     vmid,
		"action":         action,
		"command":        command,
		"how_to_approve": "Click the approval button in the chat to execute this action.",
		"do_not_retry":   true,
	}
	b, _ := json.Marshal(payload)
	return "APPROVAL_REQUIRED: " + string(b)
}

func formatDockerApprovalNeeded(name, host, action, command, approvalID string) string {
	payload := map[string]interface{}{
		"type":           "approval_required",
		"approval_id":    approvalID,
		"container_name": name,
		"docker_host":    host,
		"action":         action,
		"command":        command,
		"how_to_approve": "Click the approval button in the chat to execute this action.",
		"do_not_retry":   true,
	}
	b, _ := json.Marshal(payload)
	return "APPROVAL_REQUIRED: " + string(b)
}

func formatControlSuggestion(name string, vmid int, action, command, node string) string {
	return fmt.Sprintf(`To %s %s (VMID %d), run this command on node %s:

%s

Copy and paste this command to execute it manually.`, action, name, vmid, node, command)
}

func formatDockerSuggestion(name, host, action, command string) string {
	return fmt.Sprintf(`To %s container '%s' on host %s, run:

%s

Copy and paste this command to execute it manually.`, action, name, host, command)
}
