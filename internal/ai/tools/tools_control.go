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
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// registerControlTools registers the pulse_control tool
func (e *PulseToolExecutor) registerControlTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_control",
			Description: `WRITE operations: control VMs/LXCs (start/stop/restart/delete) or execute state-modifying commands. For read-only operations use pulse_read. For Docker use pulse_docker.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"type": {
						Type:        "string",
						Description: "Control type: guest or command",
						Enum:        []string{"guest", "command"},
					},
					"guest_id": {
						Type:        "string",
						Description: "For guest: VMID or name",
					},
					"action": {
						Type:        "string",
						Description: "For guest: start, stop, shutdown, restart, delete",
						Enum:        []string{"start", "stop", "shutdown", "restart", "delete"},
					},
					"command": {
						Type:        "string",
						Description: "For command type: the shell command to execute",
					},
					"target_host": {
						Type:        "string",
						Description: "For command type: hostname to run command on",
					},
					"run_on_host": {
						Type:        "boolean",
						Description: "For command type: run on host (default true)",
					},
					"force": {
						Type:        "boolean",
						Description: "For guest stop: force stop without graceful shutdown",
					},
				},
				Required: []string{"type"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeControl(ctx, args)
		},
		RequireControl: true,
	})
}

// executeControl routes to the appropriate control handler based on type
func (e *PulseToolExecutor) executeControl(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	controlType, _ := args["type"].(string)
	switch controlType {
	case "guest":
		return e.executeControlGuest(ctx, args)
	case "command":
		return e.executeRunCommand(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("unknown type: %s. Use: guest, command", controlType)), nil
	}
}

func (e *PulseToolExecutor) executeRunCommand(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	command, _ := args["command"].(string)
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
			return NewErrorResult(fmt.Errorf("no agent available for target '%s'. %s", targetHost, formatAvailableAgentHosts(e.agentServer.GetConnectedAgents()))), nil
		}
		return NewErrorResult(fmt.Errorf("no agent available for target")), nil
	}

	approvalTargetType, approvalTargetID, approvalTargetName := approvalTargetForCommand(targetHost, routing)

	// Check if this is a pre-approved execution with command hash validation.
	// This validates the approval matches this exact command+target and marks it as consumed.
	preApproved := consumeApprovalWithValidation(args, command, approvalTargetType, approvalTargetID)

	// Skip approval checks if pre-approved or in autonomous mode.
	if !preApproved && !e.isAutonomous && e.controlLevel == ControlLevelControlled {
		approvalID := createApprovalRecord(command, approvalTargetType, approvalTargetID, approvalTargetName, "Control level requires approval")
		return NewTextResult(formatApprovalNeeded(command, "Control level requires approval", approvalID)), nil
	}
	if e.isAutonomous {
		log.Debug().
			Str("command", command).
			Bool("read_only", safety.IsReadOnlyCommand(command)).
			Msg("Auto-approving command for autonomous investigation")
	}
	if !preApproved && decision == agentexec.PolicyRequireApproval && !e.isAutonomous {
		approvalID := createApprovalRecord(command, approvalTargetType, approvalTargetID, approvalTargetName, "Security policy requires approval")
		return NewTextResult(formatApprovalNeeded(command, "Security policy requires approval", approvalID)), nil
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
	if redacted, n := safety.RedactSensitiveText(output); n > 0 {
		output = redacted + fmt.Sprintf("\n\n[redacted %d sensitive value(s)]", n)
	}

	success := result.ExitCode == 0
	response := map[string]interface{}{
		"success":      success,
		"type":         "command",
		"command":      command,
		"target_host":  targetHost,
		"exit_code":    result.ExitCode,
		"output":       output,
		"execution":    buildExecutionProvenance(targetHost, routing),
		"verification": map[string]interface{}{"ok": success, "method": "exit_code", "exit_code": result.ExitCode},
	}
	return NewJSONResultWithIsError(response, !success), nil
}

// approvalTargetForCommand derives stable approval binding fields from resolved routing.
// This ensures replay protection hashes match the actual execution target.
func approvalTargetForCommand(targetHost string, routing CommandRoutingResult) (targetType, targetID, targetName string) {
	targetType = routing.TargetType
	targetID = strings.TrimSpace(routing.TargetID)

	if targetType == "" {
		targetType = "host"
	}

	if targetType == "host" {
		// Host executions do not carry routing.TargetID in ExecuteCommand payload.
		// Use agent ID to bind approvals to a specific connected host.
		targetID = strings.TrimSpace(routing.AgentID)
		if targetID == "" {
			targetID = strings.TrimSpace(targetHost)
		}
	} else if targetID == "" {
		// Defensive fallback for unexpected missing IDs.
		targetID = strings.TrimSpace(targetHost)
	}

	targetName = strings.TrimSpace(targetHost)
	if targetName == "" {
		targetName = strings.TrimSpace(routing.AgentHostname)
	}
	if targetName == "" {
		targetName = strings.TrimSpace(routing.AgentID)
	}

	return targetType, targetID, targetName
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
		return NewErrorResult(fmt.Errorf("Could not find guest '%s': %v", guestID, err)), nil
	}

	// Check if guest is protected
	vmidStr := fmt.Sprintf("%d", guest.VMID)
	for _, protected := range e.protectedGuests {
		if protected == vmidStr || protected == guest.Name {
			return NewErrorResult(fmt.Errorf("Guest %s (VMID %d) is protected and cannot be controlled by Pulse Assistant.", guest.Name, guest.VMID)), nil
		}
	}

	// Build the command
	cmdTool := "pct"
	if guest.Type == "vm" {
		cmdTool = "qm"
	}

	// For delete action, verify guest is stopped first
	if action == "delete" && guest.Status != "stopped" {
		return NewErrorResult(fmt.Errorf("Cannot delete %s (VMID %d) - it is currently %s. Stop it first, then try deleting again.", guest.Name, guest.VMID, guest.Status)), nil
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

	approvalTargetID := fmt.Sprintf("%s:%d", guest.Node, guest.VMID)

	// Check if this is a pre-approved execution (agentic loop re-executing after user approval).
	// Use consumeApprovalWithValidation to enforce command-bound, single-use approvals.
	preApproved := consumeApprovalWithValidation(args, command, guest.Type, approvalTargetID)

	// Check security policy (skip if pre-approved)
	if !preApproved && e.policy != nil {
		decision := e.policy.Evaluate(command)
		if decision == agentexec.PolicyBlock {
			return NewTextResult(formatPolicyBlocked(command, "This command is blocked by security policy")), nil
		}
		if decision == agentexec.PolicyRequireApproval && !e.isAutonomous {
			// Use guest.Node (the Proxmox host) as targetName so approval execution can find the correct agent
			approvalID := createApprovalRecord(command, guest.Type, approvalTargetID, guest.Node, fmt.Sprintf("%s guest %s", action, guest.Name))
			return NewTextResult(formatControlApprovalNeeded(guest.Name, guest.VMID, action, command, approvalID)), nil
		}
	}

	// Check control level - this must be outside policy check since policy may be nil (skip if pre-approved)
	if !preApproved && e.controlLevel == ControlLevelControlled {
		// Use guest.Node (the Proxmox host) as targetName so approval execution can find the correct agent
		approvalID := createApprovalRecord(command, guest.Type, approvalTargetID, guest.Node, fmt.Sprintf("%s guest %s", action, guest.Name))
		return NewTextResult(formatControlApprovalNeeded(guest.Name, guest.VMID, action, command, approvalID)), nil
	}

	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	agentID := e.findAgentForNode(guest.Node)
	if agentID == "" {
		return NewErrorResult(fmt.Errorf("No agent available on node '%s'. Install Pulse Unified Agent on the Proxmox host to enable control.", guest.Node)), nil
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

	// Detect idempotent success: the guest is already in the desired state.
	// Proxmox returns exit code 255 with "not running" for stop/shutdown on a stopped guest,
	// or "already running" for start on a running guest. These aren't failures — the desired
	// state is already achieved.
	outputLower := strings.ToLower(output)
	alreadyDone := false
	switch action {
	case "stop", "shutdown":
		alreadyDone = strings.Contains(outputLower, "not running")
	case "start":
		alreadyDone = strings.Contains(outputLower, "already running")
	}
	if alreadyDone {
		result.ExitCode = 0
		output = fmt.Sprintf("%s\n(idempotent: desired state already set)", output)
	}

	if redacted, n := safety.RedactSensitiveText(output); n > 0 {
		output = redacted + fmt.Sprintf("\n\n[redacted %d sensitive value(s)]", n)
	}

	verify := e.verifyGuestAction(ctx, agentID, cmdTool, guest.VMID, action)
	verify["ok"] = result.ExitCode == 0
	success := result.ExitCode == 0
	response := map[string]interface{}{
		"success":      success,
		"type":         "guest",
		"guest":        guest.Name,
		"guest_id":     fmt.Sprintf("%d", guest.VMID),
		"guest_type":   guest.Type,
		"node":         guest.Node,
		"action":       action,
		"command":      command,
		"exit_code":    result.ExitCode,
		"output":       output,
		"verification": verify,
	}
	return NewJSONResultWithIsError(response, !success), nil
}

func (e *PulseToolExecutor) verifyGuestAction(ctx context.Context, agentID, cmdTool string, vmID int, action string) map[string]interface{} {
	expect := ""
	switch action {
	case "start", "restart":
		expect = "running"
	case "stop", "shutdown":
		expect = "stopped"
	case "delete":
		expect = "deleted"
	}

	statusCmd := fmt.Sprintf("%s status %d", cmdTool, vmID)
	res, err := e.agentServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		Command:    statusCmd,
		TargetType: "host",
	})
	if err != nil {
		return map[string]interface{}{"confirmed": false, "method": "status", "command": statusCmd, "note": err.Error()}
	}
	out := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
	outLower := strings.ToLower(out)

	// Delete verification: status should fail with does-not-exist semantics.
	if action == "delete" {
		confirmed := res.ExitCode != 0 && (strings.Contains(outLower, "does not exist") || strings.Contains(outLower, "no such") || strings.Contains(outLower, "not found"))
		return map[string]interface{}{"confirmed": confirmed, "method": "status", "command": statusCmd, "expected": expect, "raw": out}
	}

	observed := ""
	if strings.Contains(outLower, "status: running") {
		observed = "running"
	} else if strings.Contains(outLower, "status: stopped") {
		observed = "stopped"
	}
	confirmed := res.ExitCode == 0 && observed != "" && observed == expect
	return map[string]interface{}{"confirmed": confirmed, "method": "status", "command": statusCmd, "expected": expect, "observed": observed, "raw": out}
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

func (e *PulseToolExecutor) readStateForControl() (unifiedresources.ReadState, error) {
	if rs := e.getReadState(); rs != nil {
		return rs, nil
	}
	if e.stateProvider == nil {
		return nil, fmt.Errorf("read state not available")
	}

	// Compatibility bridge for tests and any remaining legacy wiring:
	// build a typed ReadState view from the current StateSnapshot.
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestSnapshot(e.stateProvider.GetState())
	return rr, nil
}

func (e *PulseToolExecutor) resolveGuest(guestID string) (*GuestInfo, error) {
	rs, err := e.readStateForControl()
	if err != nil {
		return nil, err
	}

	vmID, convErr := strconv.Atoi(guestID)
	for _, vm := range rs.VMs() {
		if (convErr == nil && vm.VMID() == vmID) || vm.Name() == guestID || vm.ID() == guestID {
			return &GuestInfo{
				VMID:     vm.VMID(),
				Name:     vm.Name(),
				Node:     vm.Node(),
				Type:     "vm",
				Status:   string(vm.Status()),
				Instance: vm.Instance(),
			}, nil
		}
	}

	for _, ct := range rs.Containers() {
		if (convErr == nil && ct.VMID() == vmID) || ct.Name() == guestID || ct.ID() == guestID {
			return &GuestInfo{
				VMID:     ct.VMID(),
				Name:     ct.Name(),
				Node:     ct.Node(),
				Type:     "lxc",
				Status:   string(ct.Status()),
				Instance: ct.Instance(),
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

	rs, err := e.readStateForControl()
	if err != nil {
		return ""
	}

	// Map linked node IDs to node names for quick lookup.
	nodeNamesByID := make(map[string]string)
	for _, node := range rs.Nodes() {
		name := node.Name()
		if name == "" {
			name = node.NodeName()
		}
		if node.ID() != "" {
			nodeNamesByID[node.ID()] = name
		}
	}

	for _, host := range rs.Hosts() {
		linked := host.LinkedNodeID()
		if linked == "" {
			continue
		}
		if nodeNamesByID[linked] != nodeName {
			continue
		}
		for _, agent := range agents {
			if agent.Hostname == host.Hostname() || agent.AgentID == host.ID() {
				return agent.AgentID
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
	if rs, err := e.readStateForControl(); err == nil {
		// Check LXCs
		for _, ct := range rs.Containers() {
			if ct.Name() == dockerHost.Hostname {
				result.ResolvedKind = "lxc"
				result.ResolvedNode = ct.Node()
				result.TargetType = "container"
				result.TargetID = fmt.Sprintf("%d", ct.VMID())
				result.Transport = "pct_exec"
				nodeAgentID := e.findAgentForNode(ct.Node())
				if nodeAgentID != "" {
					result.AgentID = nodeAgentID
					result.AgentHostname = ct.Node()
					log.Debug().
						Str("docker_host", dockerHost.Hostname).
						Str("node", ct.Node()).
						Int("vmid", ct.VMID()).
						Str("agent", nodeAgentID).
						Str("transport", result.Transport).
						Msg("Resolved Docker host as LXC, routing through Proxmox agent")
				}
				return result
			}
		}

		// Check VMs
		for _, vm := range rs.VMs() {
			if vm.Name() == dockerHost.Hostname {
				result.ResolvedKind = "vm"
				result.ResolvedNode = vm.Node()
				result.TargetType = "vm"
				result.TargetID = fmt.Sprintf("%d", vm.VMID())
				result.Transport = "qm_guest_exec"
				nodeAgentID := e.findAgentForNode(vm.Node())
				if nodeAgentID != "" {
					result.AgentID = nodeAgentID
					result.AgentHostname = vm.Node()
					log.Debug().
						Str("docker_host", dockerHost.Hostname).
						Str("node", vm.Node()).
						Int("vmid", vm.VMID()).
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

// createApprovalRecord creates an approval record in the store and returns the approval ID.
// Returns empty string if store is not available (approvals will still work, just without persistence).
func createApprovalRecord(command, targetType, targetID, targetName, context string) string {
	store := approval.GetStore()
	if store == nil {
		log.Debug().Msg("approval store not available, approval will not be persisted")
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
		log.Warn().Err(err).Msg("failed to create approval record")
		return ""
	}

	log.Debug().Str("approval_id", req.ID).Str("command", command).Msg("created approval record")
	return req.ID
}

// isPreApproved checks if the args contain a valid, approved approval_id.
// This is used when the agentic loop re-executes a tool after user approval.
// DEPRECATED: Use consumeApprovalWithValidation instead for replay protection.
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
		log.Debug().Str("approval_id", approvalID).Msg("pre-approval check: approval not found")
		return false
	}

	if req.Status == approval.StatusApproved {
		log.Debug().Str("approval_id", approvalID).Msg("pre-approval check: approved, skipping approval flow")
		return true
	}

	log.Debug().Str("approval_id", approvalID).Str("status", string(req.Status)).Msg("pre-approval check: not approved")
	return false
}

// consumeApprovalWithValidation validates and consumes an approval for a specific command.
// It verifies the command hash matches the approval and marks it as consumed (single-use).
// Returns true if the approval is valid and was consumed, false otherwise.
func consumeApprovalWithValidation(args map[string]interface{}, command, targetType, targetID string) bool {
	approvalID, ok := args["_approval_id"].(string)
	if !ok || approvalID == "" {
		return false
	}

	store := approval.GetStore()
	if store == nil {
		return false
	}

	_, err := store.ConsumeApproval(approvalID, command, targetType, targetID)
	if err != nil {
		log.Warn().Err(err).Str("approval_id", approvalID).Msg("failed to consume approval")
		return false
	}

	return true
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

func collectAgentHostnames(agents []agentexec.ConnectedAgent, max int) (all []string, truncated []string) {
	all = make([]string, 0, len(agents))
	for _, agent := range agents {
		name := strings.TrimSpace(agent.Hostname)
		if name == "" {
			name = strings.TrimSpace(agent.AgentID)
		}
		if name != "" {
			all = append(all, name)
		}
	}

	truncated = all
	if max >= 0 && len(all) > max {
		truncated = all[:max]
	}

	return all, truncated
}

func formatTargetHostRequired(agents []agentexec.ConnectedAgent) string {
	const maxItems = 6
	hostnames, list := collectAgentHostnames(agents, maxItems)
	if len(hostnames) == 0 {
		return "Multiple agents are connected. Please specify target_host."
	}
	message := fmt.Sprintf("Multiple agents are connected. Please specify target_host. Available: %s", strings.Join(list, ", "))
	if len(hostnames) > maxItems {
		message = fmt.Sprintf("%s (+%d more)", message, len(hostnames)-maxItems)
	}
	return message
}

// formatAvailableAgentHosts returns a hint listing connected agent hostnames.
func formatAvailableAgentHosts(agents []agentexec.ConnectedAgent) string {
	const maxItems = 6
	hostnames, list := collectAgentHostnames(agents, maxItems)
	if len(hostnames) == 0 {
		return "No agents are currently connected."
	}
	msg := fmt.Sprintf("Available targets: %s", strings.Join(list, ", "))
	if len(hostnames) > maxItems {
		msg = fmt.Sprintf("%s (+%d more)", msg, len(hostnames)-maxItems)
	}
	return msg
}

func formatControlApprovalNeeded(name string, vmID int, action, command, approvalID string) string {
	payload := map[string]interface{}{
		"type":           "approval_required",
		"approval_id":    approvalID,
		"guest_name":     name,
		"guest_vmid":     vmID,
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
