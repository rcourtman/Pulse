package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rs/zerolog/log"
)

// ExecutionProvenance tracks where a command actually executed.
// This makes it observable whether a command ran on the intended target
// or fell back to a different execution context.
type ExecutionProvenance struct {
	// What the model requested
	RequestedTargetHost string `json:"requested_target_host"`

	// What we resolved it to
	ResolvedKind string `json:"resolved_kind"` // "host", "lxc", "vm", "docker"
	ResolvedNode string `json:"resolved_node"` // Proxmox node name (if applicable)
	ResolvedUID  string `json:"resolved_uid"`  // VMID or container ID

	// How we executed it
	AgentHost string `json:"agent_host"` // Hostname of the agent that executed
	Transport string `json:"transport"`  // "direct", "pct_exec", "qm_guest_exec"
}

// registerFileTools registers the file editing tool
func (e *PulseToolExecutor) registerFileTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_file_edit",
			Description: `Read and edit files on remote hosts, LXC containers, VMs, and Docker containers safely.

Actions:
- read: Read the contents of a file
- append: Append content to the end of a file
- write: Write/overwrite a file with new content (creates if doesn't exist)

This tool handles escaping automatically - just provide the content as-is.
Use this instead of shell commands for editing config files (YAML, JSON, etc.)

Routing: target_host can be a Proxmox host (delly), an LXC name (homepage-docker), or a VM name. Commands are automatically routed through the appropriate agent.

Docker container support: Use docker_container to access files INSIDE a Docker container. The target_host specifies where Docker is running.

Examples:
- Read from LXC: action="read", path="/opt/app/config.yaml", target_host="homepage-docker"
- Write to host: action="write", path="/tmp/test.txt", content="hello", target_host="delly"
- Read from Docker: action="read", path="/config/settings.json", target_host="tower", docker_container="jellyfin"
- Write to Docker: action="write", path="/tmp/test.txt", content="hello", target_host="tower", docker_container="nginx"`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"action": {
						Type:        "string",
						Description: "File action: read, append, or write",
						Enum:        []string{"read", "append", "write"},
					},
					"path": {
						Type:        "string",
						Description: "Absolute path to the file",
					},
					"content": {
						Type:        "string",
						Description: "Content to write or append (for append/write actions)",
					},
					"target_host": {
						Type:        "string",
						Description: "Hostname where the file exists (or where Docker is running)",
					},
					"docker_container": {
						Type:        "string",
						Description: "Docker container name (for files inside containers)",
					},
				},
				Required: []string{"action", "path", "target_host"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeFileEdit(ctx, args)
		},
		RequireControl: true,
	})
}

// executeFileEdit handles file read/write operations
func (e *PulseToolExecutor) executeFileEdit(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	action, _ := args["action"].(string)
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	targetHost, _ := args["target_host"].(string)
	dockerContainer, _ := args["docker_container"].(string)

	if path == "" {
		return NewErrorResult(fmt.Errorf("path is required")), nil
	}
	if targetHost == "" {
		return NewErrorResult(fmt.Errorf("target_host is required")), nil
	}

	// Validate path - must be absolute
	if !strings.HasPrefix(path, "/") {
		return NewErrorResult(fmt.Errorf("path must be absolute (start with /)")), nil
	}

	// Validate docker_container if provided (simple alphanumeric + _ + -)
	if dockerContainer != "" {
		for _, c := range dockerContainer {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.') {
				return NewErrorResult(fmt.Errorf("invalid character '%c' in docker_container name", c)), nil
			}
		}
	}

	// Check control level
	if e.controlLevel == ControlLevelReadOnly && action != "read" {
		return NewTextResult("File editing is not available in read-only mode."), nil
	}

	switch action {
	case "read":
		return e.executeFileRead(ctx, path, targetHost, dockerContainer)
	case "append":
		if content == "" {
			return NewErrorResult(fmt.Errorf("content is required for append action")), nil
		}
		return e.executeFileAppend(ctx, path, content, targetHost, dockerContainer, args)
	case "write":
		if content == "" {
			return NewErrorResult(fmt.Errorf("content is required for write action")), nil
		}
		return e.executeFileWrite(ctx, path, content, targetHost, dockerContainer, args)
	default:
		return NewErrorResult(fmt.Errorf("unknown action: %s. Use: read, append, write", action)), nil
	}
}

// executeFileRead reads a file's contents
func (e *PulseToolExecutor) executeFileRead(ctx context.Context, path, targetHost, dockerContainer string) (CallToolResult, error) {
	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	// Validate routing context - block if targeting a Proxmox host when child resources exist
	// This prevents accidentally reading files from the host when user meant to read from an LXC/VM
	routingResult := e.validateRoutingContext(targetHost)
	if routingResult.IsBlocked() {
		return NewToolResponseResult(routingResult.RoutingError.ToToolResponse()), nil
	}

	// Use full routing resolution - includes provenance for debugging
	routing := e.resolveTargetForCommandFull(targetHost)
	if routing.AgentID == "" {
		if routing.TargetType == "container" || routing.TargetType == "vm" {
			return NewTextResult(fmt.Sprintf("'%s' is a %s but no agent is available on its Proxmox host. Install Pulse Unified Agent on the Proxmox node.", targetHost, routing.TargetType)), nil
		}
		return NewTextResult(fmt.Sprintf("No agent found for host '%s'. Check that the hostname is correct and an agent is connected.", targetHost)), nil
	}

	var command string
	if dockerContainer != "" {
		// File is inside Docker container
		command = fmt.Sprintf("docker exec %s cat %s", shellEscape(dockerContainer), shellEscape(path))
	} else {
		// File is on host filesystem (existing behavior)
		command = fmt.Sprintf("cat %s", shellEscape(path))
	}

	result, err := e.agentServer.ExecuteCommand(ctx, routing.AgentID, agentexec.ExecuteCommandPayload{
		Command:    command,
		TargetType: routing.TargetType,
		TargetID:   routing.TargetID,
	})
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to read file: %w", err)), nil
	}

	if result.ExitCode != 0 {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = result.Stdout
		}
		if dockerContainer != "" {
			return NewTextResult(fmt.Sprintf("Failed to read file from container '%s' (exit code %d): %s", dockerContainer, result.ExitCode, errMsg)), nil
		}
		return NewTextResult(fmt.Sprintf("Failed to read file (exit code %d): %s", result.ExitCode, errMsg)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"path":    path,
		"content": result.Stdout,
		"host":    targetHost,
		"size":    len(result.Stdout),
	}
	if dockerContainer != "" {
		response["docker_container"] = dockerContainer
	}
	// Include execution provenance for observability
	response["execution"] = buildExecutionProvenance(targetHost, routing)
	return NewJSONResult(response), nil
}

// executeFileAppend appends content to a file
func (e *PulseToolExecutor) executeFileAppend(ctx context.Context, path, content, targetHost, dockerContainer string, args map[string]interface{}) (CallToolResult, error) {
	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	// Validate routing context - block if targeting a Proxmox host when child resources exist
	// This prevents accidentally writing files to the host when user meant to write to an LXC/VM
	routingResult := e.validateRoutingContext(targetHost)
	if routingResult.IsBlocked() {
		return NewToolResponseResult(routingResult.RoutingError.ToToolResponse()), nil
	}

	// Validate resource is in resolved context (write operation)
	// With PULSE_STRICT_RESOLUTION=true, this blocks execution on undiscovered resources
	validation := e.validateResolvedResource(targetHost, "append", true)
	if validation.IsBlocked() {
		// Hard validation failure - return consistent error envelope
		return NewToolResponseResult(validation.StrictError.ToToolResponse()), nil
	}
	// Soft validation warnings are logged inside validateResolvedResource

	// Use full routing resolution - includes provenance for debugging
	routing := e.resolveTargetForCommandFull(targetHost)
	if routing.AgentID == "" {
		if routing.TargetType == "container" || routing.TargetType == "vm" {
			return NewTextResult(fmt.Sprintf("'%s' is a %s but no agent is available on its Proxmox host. Install Pulse Unified Agent on the Proxmox node.", targetHost, routing.TargetType)), nil
		}
		return NewTextResult(fmt.Sprintf("No agent found for host '%s'. Check that the hostname is correct and an agent is connected.", targetHost)), nil
	}

	// INVARIANT: If the target resolves to a child resource (LXC/VM), writes MUST execute
	// inside that context via pct_exec/qm_guest_exec. No silent node fallback.
	if err := e.validateWriteExecutionContext(targetHost, routing); err != nil {
		return NewToolResponseResult(err.ToToolResponse()), nil
	}

	// Check if pre-approved
	preApproved := isPreApproved(args)

	// Skip approval checks if pre-approved or in autonomous mode
	if !preApproved && !e.isAutonomous && e.controlLevel == ControlLevelControlled {
		target := targetHost
		if dockerContainer != "" {
			target = fmt.Sprintf("%s (container: %s)", targetHost, dockerContainer)
		}
		approvalID := createApprovalRecord(
			fmt.Sprintf("Append to file: %s", path),
			"file",
			path,
			target,
			fmt.Sprintf("Append %d bytes to %s", len(content), path),
		)
		return NewTextResult(formatFileApprovalNeeded(path, target, "append", len(content), approvalID)), nil
	}

	// Use base64 encoding to safely transfer content
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	var command string
	if dockerContainer != "" {
		// Append inside Docker container - docker exec needs its own sh -c
		command = fmt.Sprintf("docker exec %s sh -c 'echo %s | base64 -d >> %s'",
			shellEscape(dockerContainer), encoded, shellEscape(path))
	} else {
		// For host/LXC/VM targets - agent handles sh -c wrapping for LXC/VM
		command = fmt.Sprintf("echo '%s' | base64 -d >> %s", encoded, shellEscape(path))
	}

	result, err := e.agentServer.ExecuteCommand(ctx, routing.AgentID, agentexec.ExecuteCommandPayload{
		Command:    command,
		TargetType: routing.TargetType,
		TargetID:   routing.TargetID,
	})
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to append to file: %w", err)), nil
	}

	if result.ExitCode != 0 {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = result.Stdout
		}
		if dockerContainer != "" {
			return NewTextResult(fmt.Sprintf("Failed to append to file in container '%s' (exit code %d): %s", dockerContainer, result.ExitCode, errMsg)), nil
		}
		return NewTextResult(fmt.Sprintf("Failed to append to file (exit code %d): %s", result.ExitCode, errMsg)), nil
	}

	response := map[string]interface{}{
		"success":       true,
		"action":        "append",
		"path":          path,
		"host":          targetHost,
		"bytes_written": len(content),
	}
	if dockerContainer != "" {
		response["docker_container"] = dockerContainer
	}
	// Include execution provenance for observability
	response["execution"] = buildExecutionProvenance(targetHost, routing)
	return NewJSONResult(response), nil
}

// executeFileWrite writes content to a file (overwrites)
func (e *PulseToolExecutor) executeFileWrite(ctx context.Context, path, content, targetHost, dockerContainer string, args map[string]interface{}) (CallToolResult, error) {
	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	// Validate routing context - block if targeting a Proxmox host when child resources exist
	// This prevents accidentally writing files to the host when user meant to write to an LXC/VM
	routingResult := e.validateRoutingContext(targetHost)
	if routingResult.IsBlocked() {
		return NewToolResponseResult(routingResult.RoutingError.ToToolResponse()), nil
	}

	// Validate resource is in resolved context (write operation)
	// With PULSE_STRICT_RESOLUTION=true, this blocks execution on undiscovered resources
	validation := e.validateResolvedResource(targetHost, "write", true)
	if validation.IsBlocked() {
		// Hard validation failure - return consistent error envelope
		return NewToolResponseResult(validation.StrictError.ToToolResponse()), nil
	}
	// Soft validation warnings are logged inside validateResolvedResource

	// Use full routing resolution - includes provenance for debugging
	routing := e.resolveTargetForCommandFull(targetHost)
	if routing.AgentID == "" {
		if routing.TargetType == "container" || routing.TargetType == "vm" {
			return NewTextResult(fmt.Sprintf("'%s' is a %s but no agent is available on its Proxmox host. Install Pulse Unified Agent on the Proxmox node.", targetHost, routing.TargetType)), nil
		}
		return NewTextResult(fmt.Sprintf("No agent found for host '%s'. Check that the hostname is correct and an agent is connected.", targetHost)), nil
	}

	// INVARIANT: If the target resolves to a child resource (LXC/VM), writes MUST execute
	// inside that context via pct_exec/qm_guest_exec. No silent node fallback.
	if err := e.validateWriteExecutionContext(targetHost, routing); err != nil {
		return NewToolResponseResult(err.ToToolResponse()), nil
	}

	// Check if pre-approved
	preApproved := isPreApproved(args)

	// Skip approval checks if pre-approved or in autonomous mode
	if !preApproved && !e.isAutonomous && e.controlLevel == ControlLevelControlled {
		target := targetHost
		if dockerContainer != "" {
			target = fmt.Sprintf("%s (container: %s)", targetHost, dockerContainer)
		}
		approvalID := createApprovalRecord(
			fmt.Sprintf("Write file: %s", path),
			"file",
			path,
			target,
			fmt.Sprintf("Write %d bytes to %s", len(content), path),
		)
		return NewTextResult(formatFileApprovalNeeded(path, target, "write", len(content), approvalID)), nil
	}

	// Use base64 encoding to safely transfer content
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	var command string
	if dockerContainer != "" {
		// Write inside Docker container - docker exec needs its own sh -c
		command = fmt.Sprintf("docker exec %s sh -c 'echo %s | base64 -d > %s'",
			shellEscape(dockerContainer), encoded, shellEscape(path))
	} else {
		// For host/LXC/VM targets - agent handles sh -c wrapping for LXC/VM
		command = fmt.Sprintf("echo '%s' | base64 -d > %s", encoded, shellEscape(path))
	}

	result, err := e.agentServer.ExecuteCommand(ctx, routing.AgentID, agentexec.ExecuteCommandPayload{
		Command:    command,
		TargetType: routing.TargetType,
		TargetID:   routing.TargetID,
	})
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to write file: %w", err)), nil
	}

	if result.ExitCode != 0 {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = result.Stdout
		}
		if dockerContainer != "" {
			return NewTextResult(fmt.Sprintf("Failed to write file in container '%s' (exit code %d): %s", dockerContainer, result.ExitCode, errMsg)), nil
		}
		return NewTextResult(fmt.Sprintf("Failed to write file (exit code %d): %s", result.ExitCode, errMsg)), nil
	}

	response := map[string]interface{}{
		"success":       true,
		"action":        "write",
		"path":          path,
		"host":          targetHost,
		"bytes_written": len(content),
	}
	if dockerContainer != "" {
		response["docker_container"] = dockerContainer
	}
	// Include execution provenance for observability
	response["execution"] = buildExecutionProvenance(targetHost, routing)
	return NewJSONResult(response), nil
}

// ErrExecutionContextUnavailable is returned when a write operation targets a child resource
// (LXC/VM) but the execution cannot be properly routed into that resource context.
// This prevents silent fallback to node-level execution, which would write files on the
// Proxmox host instead of inside the LXC/VM.
type ErrExecutionContextUnavailable struct {
	TargetHost   string // What the model requested
	ResolvedKind string // What the state says it is (lxc, vm)
	ResolvedNode string // Which Proxmox node it's on
	Transport    string // What transport we got (should be pct_exec but might be "direct")
	Message      string
}

func (e *ErrExecutionContextUnavailable) Error() string {
	return e.Message
}

func (e *ErrExecutionContextUnavailable) ToToolResponse() ToolResponse {
	return NewToolBlockedError("EXECUTION_CONTEXT_UNAVAILABLE", e.Message, map[string]interface{}{
		"target_host":      e.TargetHost,
		"resolved_kind":    e.ResolvedKind,
		"resolved_node":    e.ResolvedNode,
		"transport":        e.Transport,
		"auto_recoverable": false,
		"recovery_hint":    "Cannot write files to this target. The execution context (LXC/VM) is not reachable via pct exec/qm guest exec. Verify the agent is installed on the Proxmox node and the target is running.",
	})
}

// validateWriteExecutionContext ensures write operations execute inside the correct context.
//
// INVARIANT: If state.ResolveResource says the target is an LXC/VM, writes MUST use
// pct_exec/qm_guest_exec to run inside that container. A "direct" transport on a child
// resource means we'd write to the Proxmox host's filesystem instead — which is always wrong.
//
// This catches the scenario where:
// 1. target_host="homepage-docker" (an LXC)
// 2. An agent on the node matches "homepage-docker" as a direct hostname
// 3. Command runs on the node without pct exec → writes to node filesystem
func (e *PulseToolExecutor) validateWriteExecutionContext(targetHost string, routing CommandRoutingResult) *ErrExecutionContextUnavailable {
	if e.stateProvider == nil {
		return nil // Can't validate without state
	}

	state := e.stateProvider.GetState()
	loc := state.ResolveResource(targetHost)
	if !loc.Found {
		return nil // Unknown resource, nothing to validate
	}

	// Only validate for child resources (LXC/VM)
	isChildResource := loc.ResourceType == "lxc" || loc.ResourceType == "vm"
	if !isChildResource {
		return nil
	}

	// For child resources, the routing MUST use pct_exec or qm_guest_exec
	// If it resolved as "direct" (host type), that means we'd execute on the node, not inside the LXC/VM
	if routing.Transport == "direct" && routing.TargetType == "host" {
		log.Warn().
			Str("target_host", targetHost).
			Str("resolved_kind", loc.ResourceType).
			Str("resolved_node", loc.Node).
			Str("agent_hostname", routing.AgentHostname).
			Str("transport", routing.Transport).
			Msg("[FileWrite] BLOCKED: Write would execute on node, not inside child resource. " +
				"Agent matched target hostname directly, but state says target is LXC/VM.")

		return &ErrExecutionContextUnavailable{
			TargetHost:   targetHost,
			ResolvedKind: loc.ResourceType,
			ResolvedNode: loc.Node,
			Transport:    routing.Transport,
			Message: fmt.Sprintf(
				"'%s' is a %s on node '%s', but the write would execute on the Proxmox node instead of inside the %s. "+
					"This happens when an agent matches the hostname directly instead of routing via pct exec. "+
					"The file would be written to the node's filesystem, not the %s's filesystem.",
				targetHost, loc.ResourceType, loc.Node, loc.ResourceType, loc.ResourceType),
		}
	}

	// Also validate: if resolved as LXC but no agent found for the node
	if routing.AgentID == "" {
		return &ErrExecutionContextUnavailable{
			TargetHost:   targetHost,
			ResolvedKind: loc.ResourceType,
			ResolvedNode: loc.Node,
			Transport:    "none",
			Message: fmt.Sprintf(
				"'%s' is a %s on node '%s', but no agent is available on that Proxmox node. "+
					"Install the Pulse Unified Agent on '%s' to enable file operations inside the %s.",
				targetHost, loc.ResourceType, loc.Node, loc.Node, loc.ResourceType),
		}
	}

	return nil
}

// buildExecutionProvenance creates provenance metadata for tool responses.
// This makes it observable WHERE a command actually executed.
func buildExecutionProvenance(targetHost string, routing CommandRoutingResult) map[string]interface{} {
	return map[string]interface{}{
		"requested_target_host": targetHost,
		"resolved_kind":         routing.ResolvedKind,
		"resolved_node":         routing.ResolvedNode,
		"agent_host":            routing.AgentHostname,
		"transport":             routing.Transport,
		"target_type":           routing.TargetType,
		"target_id":             routing.TargetID,
	}
}

// findAgentByHostname finds an agent ID by hostname
func (e *PulseToolExecutor) findAgentByHostname(hostname string) string {
	if e.agentServer == nil {
		return ""
	}

	agents := e.agentServer.GetConnectedAgents()
	hostnameLower := strings.ToLower(hostname)

	for _, agent := range agents {
		// Match by hostname (case-insensitive) or by agentID (case-sensitive)
		if strings.ToLower(agent.Hostname) == hostnameLower || agent.AgentID == hostname {
			return agent.AgentID
		}
	}
	return ""
}

// shellEscape escapes a string for safe use in shell commands
func shellEscape(s string) string {
	// Use single quotes and escape any existing single quotes
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// formatFileApprovalNeeded formats an approval-required response for file operations
func formatFileApprovalNeeded(path, host, action string, size int, approvalID string) string {
	return fmt.Sprintf(`APPROVAL_REQUIRED: {"type":"approval_required","approval_id":"%s","action":"file_%s","path":"%s","host":"%s","size":%d,"message":"File %s operation requires approval"}`,
		approvalID, action, path, host, size, action)
}
