package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// registerControlTools registers control tools (conditional on control level)
func (e *PulseToolExecutor) registerControlTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_run_command",
			Description: `Execute a shell command on infrastructure via a connected agent.

This tool has built-in user approval - just call it directly when requested.
Prefer query tools first. If multiple agents exist and target is unclear, ask which host.

Routing: target_host can be a Proxmox host (delly), an LXC name (homepage-docker), or a VM name.
Commands targeting LXCs/VMs are automatically routed through the Proxmox host agent.`,
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
						Description: "Hostname to run on (e.g. 'delly'). Ask the user to choose if multiple agents are connected or if the target is unclear.",
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
			Description: `Start, stop, restart, or delete Proxmox VMs and LXC containers.

This tool has built-in user approval - just call it directly when requested.
Use pulse_search_resources to find the guest first if needed.
For Docker containers, use pulse_control_docker instead.
Delete requires the guest to be stopped first.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"guest_id": {
						Type:        "string",
						Description: "VMID number (e.g. '101') or exact name of the VM/LXC container",
					},
					"action": {
						Type:        "string",
						Description: "start, stop (immediate), shutdown (graceful), restart, or delete (permanent removal - guest must be stopped first)",
						Enum:        []string{"start", "stop", "shutdown", "restart", "delete"},
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

Prefer: Use pulse_search_resources or pulse_list_infrastructure to confirm the container and host before control actions.

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

	// Validate resource is in resolved context
	// Uses command risk classification: read-only commands bypass strict mode
	// With PULSE_STRICT_RESOLUTION=true, write commands are blocked on undiscovered resources
	if targetHost != "" {
		validation := e.validateResolvedResourceForExec(targetHost, command, true)
		if validation.IsBlocked() {
			// Hard validation failure - return consistent error envelope
			return NewToolResponseResult(validation.StrictError.ToToolResponse()), nil
		}
		if validation.ErrorMsg != "" {
			// Soft validation - log warning but allow operation
			log.Warn().
				Str("target", targetHost).
				Str("command", command).
				Str("validation_error", validation.ErrorMsg).
				Msg("[Control] Target resource not in resolved context - may indicate model hallucination")
		}

		// Validate routing context - block if targeting a Proxmox host when child resources exist
		// This prevents accidentally executing commands on the host when user meant to target an LXC/VM
		routingResult := e.validateRoutingContext(targetHost)
		if routingResult.IsBlocked() {
			return NewToolResponseResult(routingResult.RoutingError.ToToolResponse()), nil
		}
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

	if targetHost == "" && e.agentServer != nil {
		agents := e.agentServer.GetConnectedAgents()
		if len(agents) > 1 {
			return NewTextResult(formatTargetHostRequired(agents)), nil
		}
	}

	// Skip approval checks if pre-approved or in autonomous mode
	if !preApproved && !e.isAutonomous && e.controlLevel == ControlLevelControlled {
		targetType := "container"
		if runOnHost {
			targetType = "host"
		}
		approvalID := createApprovalRecord(command, targetType, e.targetID, targetHost, "Control level requires approval")
		return NewTextResult(formatApprovalNeeded(command, "Control level requires approval", approvalID)), nil
	}
	if e.isAutonomous {
		log.Debug().
			Str("command", command).
			Bool("read_only", safety.IsReadOnlyCommand(command)).
			Msg("Auto-approving command for autonomous investigation")
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

	// Resolve target to the correct agent and routing info (with full provenance)
	// If targetHost is an LXC/VM name, this routes to the Proxmox host agent
	// with the correct TargetType and TargetID for pct exec / qm guest exec
	routing := e.resolveTargetForCommandFull(targetHost)
	if routing.AgentID == "" {
		if targetHost != "" {
			if routing.TargetType == "container" || routing.TargetType == "vm" {
				return NewErrorResult(fmt.Errorf("'%s' is a %s but no agent is available on its Proxmox host. Install Pulse Unified Agent on the Proxmox node.", targetHost, routing.TargetType)), nil
			}
			return NewErrorResult(fmt.Errorf("no agent available for target '%s'. Specify a valid hostname with a connected agent.", targetHost)), nil
		}
		return NewErrorResult(fmt.Errorf("no agent available for target")), nil
	}

	log.Debug().
		Str("target_host", targetHost).
		Str("agent_id", routing.AgentID).
		Str("agent_host", routing.AgentHostname).
		Str("resolved_kind", routing.ResolvedKind).
		Str("resolved_node", routing.ResolvedNode).
		Str("transport", routing.Transport).
		Str("target_type", routing.TargetType).
		Str("target_id", routing.TargetID).
		Msg("[pulse_control] Routing command execution")

	result, err := e.agentServer.ExecuteCommand(ctx, routing.AgentID, agentexec.ExecuteCommandPayload{
		Command:    command,
		TargetType: routing.TargetType,
		TargetID:   routing.TargetID,
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

	// Success - always show output explicitly to prevent LLM hallucination
	// When output is empty, we must be explicit about it so the LLM doesn't fabricate results
	if output == "" {
		return NewTextResult("Command completed successfully (exit code 0).\n\nOutput:\n(no output)"), nil
	}
	return NewTextResult(fmt.Sprintf("Command completed successfully (exit code 0).\n\nOutput:\n%s", output)), nil
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

	validActions := map[string]bool{"start": true, "stop": true, "shutdown": true, "restart": true, "delete": true}
	if !validActions[action] {
		return NewErrorResult(fmt.Errorf("invalid action: %s. Use start, stop, shutdown, restart, or delete", action)), nil
	}

	// Validate resource is in resolved context
	// With PULSE_STRICT_RESOLUTION=true, this blocks execution on undiscovered resources
	validation := e.validateResolvedResource(guestID, action, true)
	if validation.IsBlocked() {
		// Hard validation failure - return consistent error envelope
		return NewToolResponseResult(validation.StrictError.ToToolResponse()), nil
	}
	if validation.ErrorMsg != "" {
		// Soft validation - log warning but allow operation
		log.Warn().
			Str("guest_id", guestID).
			Str("action", action).
			Str("validation_error", validation.ErrorMsg).
			Msg("[ControlGuest] Guest not in resolved context - may indicate model hallucination")
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
			return NewTextResult(fmt.Sprintf("Guest %s (VMID %d) is protected and cannot be controlled by Pulse Assistant.", guest.Name, guest.VMID)), nil
		}
	}

	// Build the command
	cmdTool := "pct"
	if guest.Type == "vm" {
		cmdTool = "qm"
	}

	// For delete action, verify guest is stopped first
	if action == "delete" && guest.Status != "stopped" {
		return NewTextResult(fmt.Sprintf("Cannot delete %s (VMID %d) - it is currently %s. Stop it first, then try deleting again.", guest.Name, guest.VMID, guest.Status)), nil
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
	case "delete":
		// Delete uses 'destroy' subcommand with --purge to also remove associated storage
		command = fmt.Sprintf("%s destroy %d --purge", cmdTool, guest.VMID)
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

	// Validate resource is in resolved context
	// With PULSE_STRICT_RESOLUTION=true, this blocks execution on undiscovered resources
	validation := e.validateResolvedResource(containerName, action, true)
	if validation.IsBlocked() {
		// Hard validation failure - return consistent error envelope
		return NewToolResponseResult(validation.StrictError.ToToolResponse()), nil
	}
	if validation.ErrorMsg != "" {
		// Soft validation - log warning but allow operation
		log.Warn().
			Str("container", containerName).
			Str("action", action).
			Str("host", hostName).
			Str("validation_error", validation.ErrorMsg).
			Msg("[ControlDocker] Container not in resolved context - may indicate model hallucination")
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

	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	// Resolve the Docker host to the correct agent and routing info (with full provenance)
	routing := e.resolveDockerHostRoutingFull(dockerHost)
	if routing.AgentID == "" {
		if routing.TargetType == "container" || routing.TargetType == "vm" {
			return NewTextResult(fmt.Sprintf("Docker host '%s' is a %s but no agent is available on its Proxmox host. Install Pulse Unified Agent on the Proxmox node.", dockerHost.Hostname, routing.TargetType)), nil
		}
		return NewTextResult(fmt.Sprintf("No agent available on Docker host '%s'. Install Pulse Unified Agent on the host to enable control.", dockerHost.Hostname)), nil
	}

	log.Debug().
		Str("docker_host", dockerHost.Hostname).
		Str("agent_id", routing.AgentID).
		Str("agent_host", routing.AgentHostname).
		Str("resolved_kind", routing.ResolvedKind).
		Str("resolved_node", routing.ResolvedNode).
		Str("transport", routing.Transport).
		Str("target_type", routing.TargetType).
		Str("target_id", routing.TargetID).
		Msg("[pulse_control docker] Routing docker command execution")

	result, err := e.agentServer.ExecuteCommand(ctx, routing.AgentID, agentexec.ExecuteCommandPayload{
		Command:    command,
		TargetType: routing.TargetType,
		TargetID:   routing.TargetID,
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

// CommandRoutingResult contains full routing information for command execution.
// This provides the provenance needed to verify where commands actually run.
type CommandRoutingResult struct {
	// Routing info for agent
	AgentID    string // The agent that will execute the command
	TargetType string // "host", "container", or "vm"
	TargetID   string // VMID for LXC/VM, empty for host

	// Provenance info
	AgentHostname string // Hostname of the agent
	ResolvedKind  string // What kind of resource we resolved to: "node", "lxc", "vm", "docker", "host"
	ResolvedNode  string // Proxmox node name (if applicable)
	Transport     string // How command will be executed: "direct", "pct_exec", "qm_guest_exec"
}

// resolveTargetForCommandFull resolves a target_host to full routing info including provenance.
// Use this for write operations where you need to verify execution context.
//
// CRITICAL ORDERING: Topology resolution (state.ResolveResource) happens FIRST.
// Agent hostname matching is a FALLBACK only when the state doesn't know the resource.
// This prevents the "hostname collision" bug where an agent with hostname matching an LXC name
// causes commands to execute on the node instead of inside the LXC via pct exec.
func (e *PulseToolExecutor) resolveTargetForCommandFull(targetHost string) CommandRoutingResult {
	result := CommandRoutingResult{
		TargetType: "host",
		Transport:  "direct",
	}

	if e.agentServer == nil {
		return result
	}

	agents := e.agentServer.GetConnectedAgents()
	if len(agents) == 0 {
		return result
	}

	if targetHost == "" {
		// No target_host specified - require exactly one agent or fail
		if len(agents) > 1 {
			return result
		}
		result.AgentID = agents[0].AgentID
		result.AgentHostname = agents[0].Hostname
		result.ResolvedKind = "host"
		return result
	}

	// STEP 1: Consult topology (state) FIRST — this is authoritative.
	// If the state knows about this resource, use topology-based routing.
	// This prevents hostname collisions from masquerading as host targets.
	if e.stateProvider != nil {
		state := e.stateProvider.GetState()
		loc := state.ResolveResource(targetHost)

		if loc.Found {
			// Route based on resource type
			switch loc.ResourceType {
			case "node":
				// Direct Proxmox node
				nodeAgentID := e.findAgentForNode(loc.Node)
				result.AgentID = nodeAgentID
				result.ResolvedKind = "node"
				result.ResolvedNode = loc.Node
				for _, agent := range agents {
					if agent.AgentID == nodeAgentID {
						result.AgentHostname = agent.Hostname
						break
					}
				}
				return result

			case "lxc":
				// LXC container - route through Proxmox node agent via pct exec
				nodeAgentID := e.findAgentForNode(loc.Node)
				result.ResolvedKind = "lxc"
				result.ResolvedNode = loc.Node
				result.TargetType = "container"
				result.TargetID = fmt.Sprintf("%d", loc.VMID)
				result.Transport = "pct_exec"
				if nodeAgentID != "" {
					result.AgentID = nodeAgentID
					for _, agent := range agents {
						if agent.AgentID == nodeAgentID {
							result.AgentHostname = agent.Hostname
							break
						}
					}
				}
				return result

			case "vm":
				// VM - route through Proxmox node agent via qm guest exec
				nodeAgentID := e.findAgentForNode(loc.Node)
				result.ResolvedKind = "vm"
				result.ResolvedNode = loc.Node
				result.TargetType = "vm"
				result.TargetID = fmt.Sprintf("%d", loc.VMID)
				result.Transport = "qm_guest_exec"
				if nodeAgentID != "" {
					result.AgentID = nodeAgentID
					for _, agent := range agents {
						if agent.AgentID == nodeAgentID {
							result.AgentHostname = agent.Hostname
							break
						}
					}
				}
				return result

			case "docker", "dockerhost":
				// Docker container or Docker host
				result.ResolvedKind = loc.ResourceType
				result.ResolvedNode = loc.Node

				if loc.DockerHostType == "lxc" {
					nodeAgentID := e.findAgentForNode(loc.Node)
					result.TargetType = "container"
					result.TargetID = fmt.Sprintf("%d", loc.DockerHostVMID)
					result.Transport = "pct_exec"
					if nodeAgentID != "" {
						result.AgentID = nodeAgentID
						for _, agent := range agents {
							if agent.AgentID == nodeAgentID {
								result.AgentHostname = agent.Hostname
								break
							}
						}
					}
					return result
				}
				if loc.DockerHostType == "vm" {
					nodeAgentID := e.findAgentForNode(loc.Node)
					result.TargetType = "vm"
					result.TargetID = fmt.Sprintf("%d", loc.DockerHostVMID)
					result.Transport = "qm_guest_exec"
					if nodeAgentID != "" {
						result.AgentID = nodeAgentID
						for _, agent := range agents {
							if agent.AgentID == nodeAgentID {
								result.AgentHostname = agent.Hostname
								break
							}
						}
					}
					return result
				}
				// Standalone Docker host - find agent directly
				for _, agent := range agents {
					if agent.Hostname == loc.TargetHost || agent.AgentID == loc.TargetHost {
						result.AgentID = agent.AgentID
						result.AgentHostname = agent.Hostname
						return result
					}
				}
			}
		}
	}

	// STEP 2: FALLBACK — agent hostname match.
	// Only used when the state doesn't know about this resource at all.
	// This handles standalone hosts without Proxmox topology.
	for _, agent := range agents {
		if agent.Hostname == targetHost || agent.AgentID == targetHost {
			result.AgentID = agent.AgentID
			result.AgentHostname = agent.Hostname
			result.ResolvedKind = "host"
			return result
		}
	}

	return result
}

// resolveTargetForCommand resolves a target_host to the correct agent and routing info.
// Uses the authoritative ResolveResource function from models.StateSnapshot.
// Returns: agentID, targetType ("host", "container", or "vm"), targetID (vmid for LXC/VM)
//
// CRITICAL ORDERING: Same as resolveTargetForCommandFull — topology first, agent fallback second.
func (e *PulseToolExecutor) resolveTargetForCommand(targetHost string) (agentID string, targetType string, targetID string) {
	// Delegate to the full resolver and extract the triple
	r := e.resolveTargetForCommandFull(targetHost)
	return r.AgentID, r.TargetType, r.TargetID
}

func (e *PulseToolExecutor) findAgentForCommand(runOnHost bool, targetHost string) string {
	agentID, _, _ := e.resolveTargetForCommand(targetHost)
	return agentID
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
	type dockerMatch struct {
		host *models.DockerHost
		idx  int
	}
	matches := []dockerMatch{}

	for i := range state.DockerHosts {
		host := &state.DockerHosts[i]
		if hostName != "" && host.Hostname != hostName && host.DisplayName != hostName {
			continue
		}

		for ci := range host.Containers {
			container := host.Containers[ci]
			if container.Name == containerName ||
				container.ID == containerName ||
				strings.HasPrefix(container.ID, containerName) {
				matches = append(matches, dockerMatch{host: host, idx: ci})
			}
		}
	}

	if hostName != "" {
		if len(matches) == 0 {
			return nil, nil, fmt.Errorf("container '%s' not found on host '%s'", containerName, hostName)
		}
		match := matches[0]
		return &match.host.Containers[match.idx], match.host, nil
	}

	if len(matches) == 0 {
		return nil, nil, fmt.Errorf("container '%s' not found on any Docker host", containerName)
	}
	if len(matches) > 1 {
		hostNames := make([]string, 0, len(matches))
		seen := make(map[string]bool)
		for _, match := range matches {
			name := strings.TrimSpace(match.host.DisplayName)
			if name == "" {
				name = strings.TrimSpace(match.host.Hostname)
			}
			if name == "" {
				name = strings.TrimSpace(match.host.ID)
			}
			if name == "" || seen[name] {
				continue
			}
			hostNames = append(hostNames, name)
			seen[name] = true
		}
		if len(hostNames) == 0 {
			return nil, nil, fmt.Errorf("container '%s' exists on multiple Docker hosts; specify host", containerName)
		}
		return nil, nil, fmt.Errorf("container '%s' exists on multiple Docker hosts: %s. Specify host.", containerName, strings.Join(hostNames, ", "))
	}

	match := matches[0]
	return &match.host.Containers[match.idx], match.host, nil
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

// resolveDockerHostRoutingFull resolves a Docker host to the correct agent and routing info
// with full provenance metadata. If the Docker host is actually an LXC or VM, it routes
// through the Proxmox host agent with the correct TargetType and TargetID so commands
// are executed inside the guest.
func (e *PulseToolExecutor) resolveDockerHostRoutingFull(dockerHost *models.DockerHost) CommandRoutingResult {
	result := CommandRoutingResult{
		TargetType: "host",
		Transport:  "direct",
	}

	if e.agentServer == nil {
		return result
	}

	// STEP 1: Check topology — is the Docker host actually an LXC or VM?
	if e.stateProvider != nil {
		state := e.stateProvider.GetState()

		// Check LXCs
		for _, ct := range state.Containers {
			if ct.Name == dockerHost.Hostname {
				result.ResolvedKind = "lxc"
				result.ResolvedNode = ct.Node
				result.TargetType = "container"
				result.TargetID = fmt.Sprintf("%d", ct.VMID)
				result.Transport = "pct_exec"
				nodeAgentID := e.findAgentForNode(ct.Node)
				if nodeAgentID != "" {
					result.AgentID = nodeAgentID
					result.AgentHostname = ct.Node
					log.Debug().
						Str("docker_host", dockerHost.Hostname).
						Str("node", ct.Node).
						Int("vmid", ct.VMID).
						Str("agent", nodeAgentID).
						Str("transport", result.Transport).
						Msg("Resolved Docker host as LXC, routing through Proxmox agent")
				}
				return result
			}
		}

		// Check VMs
		for _, vm := range state.VMs {
			if vm.Name == dockerHost.Hostname {
				result.ResolvedKind = "vm"
				result.ResolvedNode = vm.Node
				result.TargetType = "vm"
				result.TargetID = fmt.Sprintf("%d", vm.VMID)
				result.Transport = "qm_guest_exec"
				nodeAgentID := e.findAgentForNode(vm.Node)
				if nodeAgentID != "" {
					result.AgentID = nodeAgentID
					result.AgentHostname = vm.Node
					log.Debug().
						Str("docker_host", dockerHost.Hostname).
						Str("node", vm.Node).
						Int("vmid", vm.VMID).
						Str("agent", nodeAgentID).
						Str("transport", result.Transport).
						Msg("Resolved Docker host as VM, routing through Proxmox agent")
				}
				return result
			}
		}
	}

	// STEP 2: Docker host is not an LXC/VM — use direct agent routing
	agentID := e.findAgentForDockerHost(dockerHost)
	result.AgentID = agentID
	result.ResolvedKind = "dockerhost"
	if agentID != "" {
		// Try to get agent hostname
		agents := e.agentServer.GetConnectedAgents()
		for _, a := range agents {
			if a.AgentID == agentID {
				result.AgentHostname = a.Hostname
				break
			}
		}
	}
	return result
}

// resolveDockerHostRouting delegates to resolveDockerHostRoutingFull for backwards compatibility.
func (e *PulseToolExecutor) resolveDockerHostRouting(dockerHost *models.DockerHost) (agentID string, targetType string, targetID string) {
	r := e.resolveDockerHostRoutingFull(dockerHost)
	return r.AgentID, r.TargetType, r.TargetID
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

func formatTargetHostRequired(agents []agentexec.ConnectedAgent) string {
	hostnames := make([]string, 0, len(agents))
	for _, agent := range agents {
		name := strings.TrimSpace(agent.Hostname)
		if name == "" {
			name = strings.TrimSpace(agent.AgentID)
		}
		if name != "" {
			hostnames = append(hostnames, name)
		}
	}
	if len(hostnames) == 0 {
		return "Multiple agents are connected. Please specify target_host."
	}
	maxItems := 6
	list := hostnames
	if len(hostnames) > maxItems {
		list = hostnames[:maxItems]
	}
	message := fmt.Sprintf("Multiple agents are connected. Please specify target_host. Available: %s", strings.Join(list, ", "))
	if len(hostnames) > maxItems {
		message = fmt.Sprintf("%s (+%d more)", message, len(hostnames)-maxItems)
	}
	return message
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
