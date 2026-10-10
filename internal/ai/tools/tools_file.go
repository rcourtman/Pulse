package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
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

// executeFileRead reads a file's contents
func (e *PulseToolExecutor) executeFileRead(ctx context.Context, path, targetHost, dockerContainer string) (CallToolResult, error) {

	if blocked, reason := safety.IsSensitivePath(path); blocked {
		return NewToolResponseResult(NewToolBlockedError(
			"SENSITIVE_PATH",
			fmt.Sprintf("Refusing to read sensitive path '%s' (%s).", path, reason),
			map[string]interface{}{
				"path":            path,
				"reason":          reason,
				"policy_boundary": "Credential files cannot be read through AI tools.",
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
		return unavailableCommandConnection(targetHost, routing), nil
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
			return NewErrorResult(fmt.Errorf("Failed to read file from container '%s' (exit code %d): %s", dockerContainer, result.ExitCode, errMsg)), nil
		}
		return NewErrorResult(fmt.Errorf("Failed to read file (exit code %d): %s", result.ExitCode, errMsg)), nil
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
	setContainerResponseFields(response, dockerContainer)
	// Include execution provenance for observability
	response["execution"] = buildExecutionProvenance(targetHost, routing)
	return NewJSONResult(response), nil
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
