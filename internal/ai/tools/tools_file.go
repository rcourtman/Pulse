package tools

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
	"github.com/rs/zerolog/log"
)

// ExecutionProvenance tracks where a command actually executed.
// This makes it observable whether a command ran on the intended target
// or fell back to a different execution context.
type ExecutionProvenance struct {
	// What the model requested
	RequestedTargetHost string `json:"requested_target_host"`

	// What we resolved it to
	ResolvedKind string `json:"resolved_kind"` // "host", "system-container", "vm", "docker"
	ResolvedNode string `json:"resolved_node"` // Hypervisor node name (if applicable)
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
			Description: `Read and edit files on remote hosts, containers, VMs, and Docker containers safely.

Actions:
- read: Read the contents of a file
- append: Append content to the end of a file
- write: Write/overwrite a file with new content (creates if doesn't exist)

This tool handles escaping automatically - just provide the content as-is.
Use this instead of shell commands for editing config files (YAML, JSON, etc.)

Routing: target_host can be a node (delly), a container name (homepage-docker), or a VM name. Commands are automatically routed through the appropriate agent.

Docker container support: Use docker_container to access files INSIDE a Docker container. The target_host specifies where Docker is running.

Examples:
- Read from container: action="read", path="/opt/app/config.yaml", target_host="homepage-docker"
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

	if blocked, reason := safety.IsSensitivePath(path); blocked {
		return NewToolResponseResult(NewToolBlockedError(
			"SENSITIVE_PATH",
			fmt.Sprintf("Refusing to read sensitive path '%s' (%s).", path, reason),
			map[string]interface{}{
				"path":          path,
				"reason":        reason,
				"recovery_hint": "Avoid reading credential files. If you need a value, provide it manually or scope the request to non-sensitive config/log files.",
			},
		)), nil
	}

	// Validate routing context - block if targeting a host node when child resources exist
	// This prevents accidentally reading files from the host when user meant to read from an container/VM
	routingResult := e.validateRoutingContext(targetHost)
	if routingResult.IsBlocked() {
		return NewToolResponseResult(routingResult.RoutingError.ToToolResponse()), nil
	}

	// Use full routing resolution - includes provenance for debugging
	routing := e.resolveTargetForCommandFull(targetHost)
	if routing.AgentID == "" {
		if routing.TargetType == "container" || routing.TargetType == "vm" {
			return NewTextResult(fmt.Sprintf("'%s' is a %s but no agent is available on its host node. Install Pulse Unified Agent on the node.", targetHost, routing.TargetType)), nil
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

	redacted, redactionCount := safety.RedactSensitiveText(result.Stdout)

	response := map[string]interface{}{
		"success":    true,
		"path":       path,
		"content":    redacted,
		"host":       targetHost,
		"size":       len(redacted),
		"redacted":   redactionCount > 0,
		"redactions": redactionCount,
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

	if blocked, reason := safety.IsSensitivePath(path); blocked {
		return NewToolResponseResult(NewToolBlockedError(
			"SENSITIVE_PATH",
			fmt.Sprintf("Refusing to write sensitive path '%s' (%s).", path, reason),
			map[string]interface{}{
				"path":          path,
				"reason":        reason,
				"recovery_hint": "Avoid modifying credential files via AI. Apply this change manually if needed.",
			},
		)), nil
	}

	// Validate routing context - block if targeting a host node when child resources exist
	// This prevents accidentally writing files to the host when user meant to write to an container/VM
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
			return NewTextResult(fmt.Sprintf("'%s' is a %s but no agent is available on its host node. Install Pulse Unified Agent on the node.", targetHost, routing.TargetType)), nil
		}
		return NewTextResult(fmt.Sprintf("No agent found for host '%s'. Check that the hostname is correct and an agent is connected.", targetHost)), nil
	}

	// INVARIANT: If the target resolves to a child resource (container/VM), writes MUST execute
	// inside that context via pct_exec/qm_guest_exec. No silent node fallback.
	if err := e.validateWriteExecutionContext(targetHost, routing); err != nil {
		return NewToolResponseResult(err.ToToolResponse()), nil
	}

	approvalCommand := fmt.Sprintf("Append to file: %s", path)
	approvalTargetID := fmt.Sprintf("host=%s|container=%s|path=%s", targetHost, dockerContainer, path)

	// Check if pre-approved (validated + single-use).
	preApproved := consumeApprovalWithValidation(args, e.orgID, approvalCommand, "file", approvalTargetID)

	// Skip approval checks if pre-approved or in autonomous mode
	if !preApproved && !e.isAutonomous && e.controlLevel == ControlLevelControlled {
		target := targetHost
		if dockerContainer != "" {
			target = fmt.Sprintf("%s (container: %s)", targetHost, dockerContainer)
		}
		approvalID := createApprovalRecordForOrg(
			e.orgID,
			approvalCommand,
			"file",
			approvalTargetID,
			target,
			fmt.Sprintf("Append %d bytes to %s", len(content), path),
		)
		return NewTextResult(formatFileApprovalNeeded(path, target, "append", len(content), approvalID)), nil
	}

	// Use base64 encoding to safely transfer content
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	var command string
	if dockerContainer != "" {
		// Append inside Docker container - escape the full inner command to avoid nested quote breakage.
		innerCommand := fmt.Sprintf("echo %s | base64 -d >> %s", shellEscape(encoded), shellEscape(path))
		command = fmt.Sprintf("docker exec %s sh -c %s", shellEscape(dockerContainer), shellEscape(innerCommand))
	} else {
		// For host/container/VM targets - agent handles sh -c wrapping for container/VM
		command = fmt.Sprintf("echo %s | base64 -d >> %s", shellEscape(encoded), shellEscape(path))
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
			return NewJSONResultWithIsError(map[string]interface{}{
				"success":          false,
				"action":           "append",
				"path":             path,
				"host":             targetHost,
				"docker_container": dockerContainer,
				"exit_code":        result.ExitCode,
				"error":            errMsg,
			}, true), nil
		}
		return NewJSONResultWithIsError(map[string]interface{}{
			"success":   false,
			"action":    "append",
			"path":      path,
			"host":      targetHost,
			"exit_code": result.ExitCode,
			"error":     errMsg,
		}, true), nil
	}

	verify := e.verifyFileTailHash(ctx, routing, path, dockerContainer, content)

	response := map[string]interface{}{
		"success":       true,
		"action":        "append",
		"path":          path,
		"host":          targetHost,
		"bytes_written": len(content),
		"verification":  verify,
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

	if blocked, reason := safety.IsSensitivePath(path); blocked {
		return NewToolResponseResult(NewToolBlockedError(
			"SENSITIVE_PATH",
			fmt.Sprintf("Refusing to write sensitive path '%s' (%s).", path, reason),
			map[string]interface{}{
				"path":          path,
				"reason":        reason,
				"recovery_hint": "Avoid modifying credential files via AI. Apply this change manually if needed.",
			},
		)), nil
	}

	// Validate routing context - block if targeting a host node when child resources exist
	// This prevents accidentally writing files to the host when user meant to write to an container/VM
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
			return NewTextResult(fmt.Sprintf("'%s' is a %s but no agent is available on its host node. Install Pulse Unified Agent on the node.", targetHost, routing.TargetType)), nil
		}
		return NewTextResult(fmt.Sprintf("No agent found for host '%s'. Check that the hostname is correct and an agent is connected.", targetHost)), nil
	}

	// INVARIANT: If the target resolves to a child resource (container/VM), writes MUST execute
	// inside that context via pct_exec/qm_guest_exec. No silent node fallback.
	if err := e.validateWriteExecutionContext(targetHost, routing); err != nil {
		return NewToolResponseResult(err.ToToolResponse()), nil
	}

	approvalCommand := fmt.Sprintf("Write file: %s", path)
	approvalTargetID := fmt.Sprintf("host=%s|container=%s|path=%s", targetHost, dockerContainer, path)

	// Check if pre-approved (validated + single-use).
	preApproved := consumeApprovalWithValidation(args, e.orgID, approvalCommand, "file", approvalTargetID)

	// Skip approval checks if pre-approved or in autonomous mode
	if !preApproved && !e.isAutonomous && e.controlLevel == ControlLevelControlled {
		target := targetHost
		if dockerContainer != "" {
			target = fmt.Sprintf("%s (container: %s)", targetHost, dockerContainer)
		}
		approvalID := createApprovalRecordForOrg(
			e.orgID,
			approvalCommand,
			"file",
			approvalTargetID,
			target,
			fmt.Sprintf("Write %d bytes to %s", len(content), path),
		)
		return NewTextResult(formatFileApprovalNeeded(path, target, "write", len(content), approvalID)), nil
	}

	// Use base64 encoding to safely transfer content
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	var command string
	if dockerContainer != "" {
		// Write inside Docker container - escape the full inner command to avoid nested quote breakage.
		innerCommand := fmt.Sprintf("echo %s | base64 -d > %s", shellEscape(encoded), shellEscape(path))
		command = fmt.Sprintf("docker exec %s sh -c %s", shellEscape(dockerContainer), shellEscape(innerCommand))
	} else {
		// For host/container/VM targets - agent handles sh -c wrapping for container/VM
		command = fmt.Sprintf("echo %s | base64 -d > %s", shellEscape(encoded), shellEscape(path))
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
			return NewJSONResultWithIsError(map[string]interface{}{
				"success":          false,
				"action":           "write",
				"path":             path,
				"host":             targetHost,
				"docker_container": dockerContainer,
				"exit_code":        result.ExitCode,
				"error":            errMsg,
			}, true), nil
		}
		return NewJSONResultWithIsError(map[string]interface{}{
			"success":   false,
			"action":    "write",
			"path":      path,
			"host":      targetHost,
			"exit_code": result.ExitCode,
			"error":     errMsg,
		}, true), nil
	}

	verify := e.verifyFileSHA256(ctx, routing, path, dockerContainer, content)

	response := map[string]interface{}{
		"success":       true,
		"action":        "write",
		"path":          path,
		"host":          targetHost,
		"bytes_written": len(content),
		"verification":  verify,
	}
	if dockerContainer != "" {
		response["docker_container"] = dockerContainer
	}
	// Include execution provenance for observability
	response["execution"] = buildExecutionProvenance(targetHost, routing)
	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) verifyFileSHA256(ctx context.Context, routing CommandRoutingResult, path, dockerContainer, content string) map[string]interface{} {
	expected := sha256.Sum256([]byte(content))
	expectedHex := hex.EncodeToString(expected[:])

	// Use a portable command chain; not all systems have sha256sum.
	hashCmd := fmt.Sprintf("sha256sum %s 2>/dev/null || shasum -a 256 %s 2>/dev/null || openssl dgst -sha256 %s 2>/dev/null", shellEscape(path), shellEscape(path), shellEscape(path))
	if dockerContainer != "" {
		hashCmd = fmt.Sprintf("docker exec %s sh -c %s", shellEscape(dockerContainer), shellEscape(hashCmd))
	}

	res, err := e.agentServer.ExecuteCommand(ctx, routing.AgentID, agentexec.ExecuteCommandPayload{
		Command:    hashCmd,
		TargetType: routing.TargetType,
		TargetID:   routing.TargetID,
	})
	if err != nil {
		return map[string]interface{}{
			"ok":     true,
			"method": "exit_code",
			"note":   fmt.Sprintf("sha256 verification unavailable: %v", err),
		}
	}
	combined := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
	actualHex := ""
	if fields := strings.Fields(combined); len(fields) > 0 {
		actualHex = fields[0]
	}
	if res.ExitCode != 0 || actualHex == "" {
		return map[string]interface{}{
			"ok":     true,
			"method": "exit_code",
			"note":   "sha256 verification unavailable on target (missing tools or non-zero exit)",
		}
	}
	ok := strings.EqualFold(actualHex, expectedHex)
	return map[string]interface{}{
		"ok":       ok,
		"method":   "sha256",
		"expected": expectedHex,
		"actual":   actualHex,
	}
}

func (e *PulseToolExecutor) verifyFileTailHash(ctx context.Context, routing CommandRoutingResult, path, dockerContainer, appended string) map[string]interface{} {
	// Verify that the file's trailing bytes match what we appended, without re-reading the full file.
	n := len(appended)
	if n <= 0 {
		return map[string]interface{}{"ok": true, "method": "tail_sha256", "note": "no appended content"}
	}
	expected := sha256.Sum256([]byte(appended))
	expectedHex := hex.EncodeToString(expected[:])

	tailCmd := fmt.Sprintf("tail -c %d %s 2>/dev/null | (sha256sum 2>/dev/null || shasum -a 256 2>/dev/null)", n, shellEscape(path))
	if dockerContainer != "" {
		tailCmd = fmt.Sprintf("docker exec %s sh -c %s", shellEscape(dockerContainer), shellEscape(tailCmd))
	}

	res, err := e.agentServer.ExecuteCommand(ctx, routing.AgentID, agentexec.ExecuteCommandPayload{
		Command:    tailCmd,
		TargetType: routing.TargetType,
		TargetID:   routing.TargetID,
	})
	if err != nil {
		return map[string]interface{}{"ok": true, "method": "exit_code", "note": fmt.Sprintf("tail sha256 verification unavailable: %v", err)}
	}
	combined := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
	actualHex := ""
	if fields := strings.Fields(combined); len(fields) > 0 {
		actualHex = fields[0]
	}
	if res.ExitCode != 0 || actualHex == "" {
		return map[string]interface{}{"ok": true, "method": "exit_code", "note": "tail sha256 verification unavailable on target (missing tools or non-zero exit)"}
	}
	ok := strings.EqualFold(actualHex, expectedHex)
	return map[string]interface{}{
		"ok":       ok,
		"method":   "tail_sha256",
		"expected": expectedHex,
		"actual":   actualHex,
	}
}

// ErrExecutionContextUnavailable is returned when a write operation targets a child resource
// (container/VM) but the execution cannot be properly routed into that resource context.
// This prevents silent fallback to node-level execution, which would write files on the
// host node instead of inside the container/VM.
type ErrExecutionContextUnavailable struct {
	TargetHost   string // What the model requested
	ResolvedKind string // What the state says it is (system-container, vm)
	ResolvedNode string // Which hypervisor node it's on
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
		"recovery_hint":    "Cannot write files to this target. The execution context (container/VM) is not reachable via pct exec/qm guest exec. Verify the agent is installed on the host node and the target is running.",
	})
}

// validateWriteExecutionContext ensures write operations execute inside the correct context.
//
// INVARIANT: If state.ResolveResource says the target is an container/VM, writes MUST use
// pct_exec/qm_guest_exec to run inside that container. A "direct" transport on a child
// resource means we'd write to the host node's filesystem instead — which is always wrong.
//
// This catches the scenario where:
// 1. target_host="homepage-docker" (a system container)
// 2. An agent on the node matches "homepage-docker" as a direct hostname
// 3. Command runs on the node without pct exec → writes to node filesystem
func (e *PulseToolExecutor) validateWriteExecutionContext(targetHost string, routing CommandRoutingResult) *ErrExecutionContextUnavailable {
	if !e.hasReadState() {
		return nil // Can't validate without state
	}

	loc := e.resolveResourceLocation(targetHost)
	if !loc.Found {
		return nil // Unknown resource, nothing to validate
	}

	// Only validate for child resources (system containers / VMs)
	isChildResource := loc.ResourceType == "system-container" || loc.ResourceType == "vm"
	if !isChildResource {
		return nil
	}

	// For child resources, the routing MUST use pct_exec or qm_guest_exec
	// If it resolved as "direct" (host type), that means we'd execute on the node, not inside the container/VM
	if routing.Transport == "direct" && routing.TargetType == "host" {
		log.Warn().
			Str("target_host", targetHost).
			Str("resolved_kind", loc.ResourceType).
			Str("resolved_node", loc.Node).
			Str("agent_hostname", routing.AgentHostname).
			Str("transport", routing.Transport).
			Msg("[FileWrite] BLOCKED: Write would execute on node, not inside child resource. " +
				"Agent matched target hostname directly, but state says target is container/VM.")

		return &ErrExecutionContextUnavailable{
			TargetHost:   targetHost,
			ResolvedKind: loc.ResourceType,
			ResolvedNode: loc.Node,
			Transport:    routing.Transport,
			Message: fmt.Sprintf(
				"'%s' is a %s on node '%s', but the write would execute on the host node instead of inside the %s. "+
					"This happens when an agent matches the hostname directly instead of routing via pct exec. "+
					"The file would be written to the node's filesystem, not the %s's filesystem.",
				targetHost, loc.ResourceType, loc.Node, loc.ResourceType, loc.ResourceType),
		}
	}

	// Also validate: if resolved as a guest but no agent found for the node
	if routing.AgentID == "" {
		return &ErrExecutionContextUnavailable{
			TargetHost:   targetHost,
			ResolvedKind: loc.ResourceType,
			ResolvedNode: loc.Node,
			Transport:    "none",
			Message: fmt.Sprintf(
				"'%s' is a %s on node '%s', but no agent is available on that node. "+
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

// shellEscape escapes a string for safe use in shell commands
func shellEscape(s string) string {
	// Use single quotes and escape any existing single quotes
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// formatFileApprovalNeeded formats an approval-required response for file operations
func formatFileApprovalNeeded(path, host, action string, size int, approvalID string) string {
	payload := map[string]interface{}{
		"type":        "approval_required",
		"approval_id": approvalID,
		"action":      "file_" + action,
		"path":        path,
		"host":        host,
		"size":        size,
		"message":     fmt.Sprintf("File %s operation requires approval", action),
	}
	b, _ := json.Marshal(payload)
	return "APPROVAL_REQUIRED: " + string(b)
}
