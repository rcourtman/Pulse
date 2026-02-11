package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
	"github.com/rs/zerolog/log"
)

// registerReadTools registers the read-only pulse_read tool
// This tool is ALWAYS classified as ToolKindRead and will never trigger VERIFYING state
func (e *PulseToolExecutor) registerReadTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_read",
			Description: `Execute read-only operations on infrastructure (exec, file, find, tail, logs). Rejects write commands. target_host routes to Proxmox host, LXC, or VM by name.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"action": {
						Type:        "string",
						Description: "Read action: exec, file, find, tail, logs",
						Enum:        []string{"exec", "file", "find", "tail", "logs"},
					},
					"target_host": {
						Type:        "string",
						Description: "Hostname to read from (Proxmox host, LXC name, or VM name)",
					},
					"command": {
						Type:        "string",
						Description: "For exec: the read-only shell command to run",
					},
					"path": {
						Type:        "string",
						Description: "For file/find/tail: the file path or glob pattern",
					},
					"pattern": {
						Type:        "string",
						Description: "For find: glob pattern to search for",
					},
					"lines": {
						Type:        "integer",
						Description: "For tail: number of lines (default 100)",
					},
					"source": {
						Type:        "string",
						Description: "For logs: 'docker' or 'journal'",
						Enum:        []string{"docker", "journal"},
					},
					"container": {
						Type:        "string",
						Description: "For logs with source=docker: container name",
					},
					"unit": {
						Type:        "string",
						Description: "For logs with source=journal: systemd unit name",
					},
					"since": {
						Type:        "string",
						Description: "For logs: time filter (e.g., '1h', '30m', '2024-01-01')",
					},
					"grep": {
						Type:        "string",
						Description: "For logs/tail: filter output by pattern",
					},
					"docker_container": {
						Type:        "string",
						Description: "Read from inside a Docker container (target_host is where Docker runs)",
					},
				},
				Required: []string{"action", "target_host"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeRead(ctx, args)
		},
		// Note: RequireControl is NOT set - this is a read-only tool
		// It's available at all control levels including read_only
	})
}

// executeRead routes to the appropriate read handler based on action
func (e *PulseToolExecutor) executeRead(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	action, _ := args["action"].(string)
	switch action {
	case "exec":
		return e.executeReadExec(ctx, args)
	case "file":
		return e.executeReadFile(ctx, args)
	case "find":
		return e.executeReadFind(ctx, args)
	case "tail":
		return e.executeReadTail(ctx, args)
	case "logs":
		return e.executeReadLogs(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("unknown action: %s. Use: exec, file, find, tail, logs", action)), nil
	}
}

// executeReadExec executes a read-only command
// This STRUCTURALLY ENFORCES read-only by rejecting non-read commands at the tool layer
func (e *PulseToolExecutor) executeReadExec(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	command, _ := args["command"].(string)
	targetHost, _ := args["target_host"].(string)
	dockerContainer, _ := args["docker_container"].(string)

	if command == "" {
		return NewErrorResult(fmt.Errorf("command is required for exec action")), nil
	}
	if targetHost == "" {
		return NewErrorResult(fmt.Errorf("target_host is required")), nil
	}

	// High-confidence secret exfiltration blocks.
	if blocked, reason := safety.CommandTouchesSensitivePath(command); blocked {
		return NewToolResponseResult(NewToolBlockedError(
			"SENSITIVE_COMMAND",
			fmt.Sprintf("Refusing to run command that touches sensitive paths (%s).", reason),
			map[string]interface{}{
				"reason":        reason,
				"recovery_hint": "Avoid reading credential files or process env via AI tools. Scope the request to non-sensitive logs/status output instead.",
			},
		)), nil
	}

	// STRUCTURAL ENFORCEMENT: Reject non-read-only commands at the tool layer
	// This is enforced HERE, not in the model's prompt
	// Uses ExecutionIntent: ReadOnlyCertain and ReadOnlyConditional are allowed;
	// WriteOrUnknown is rejected.
	intentResult := ClassifyExecutionIntent(command)
	if intentResult.Intent == IntentReadOnlyConditional && strings.Contains(intentResult.Reason, "model-trusted") {
		log.Info().
			Str("command", truncateCommand(command, 200)).
			Str("reason", intentResult.Reason).
			Str("target_host", targetHost).
			Msg("pulse_read: allowing model-trusted command (no blocklist match)")
	}
	if intentResult.Intent == IntentWriteOrUnknown {
		hint := GetReadOnlyViolationHint(command, intentResult)
		alternative := "Use pulse_control type=command for write operations"

		details := map[string]interface{}{
			"command":     truncateCommand(command, 100),
			"reason":      intentResult.Reason,
			"hint":        hint,
			"alternative": alternative,
		}

		// If this is a NonInteractiveOnly block with a suggested rewrite,
		// include auto-recovery information
		if niBlock := intentResult.NonInteractiveBlock; niBlock != nil {
			details["auto_recoverable"] = niBlock.AutoRecoverable
			details["category"] = niBlock.Category
			if niBlock.SuggestedCmd != "" {
				details["suggested_rewrite"] = niBlock.SuggestedCmd
				details["recovery_hint"] = fmt.Sprintf("Retry with: %s", niBlock.SuggestedCmd)
			}
		}

		return NewToolResponseResult(NewToolBlockedError(
			"READ_ONLY_VIOLATION",
			fmt.Sprintf("Command '%s' is not read-only. Use pulse_control for write operations.", truncateCommand(command, 50)),
			details,
		)), nil
	}

	// Validate routing context - block if targeting a Proxmox host when child resources exist
	// This prevents accidentally reading from the host when user meant to read from an LXC/VM
	routingResult := e.validateRoutingContext(targetHost)
	if routingResult.IsBlocked() {
		return NewToolResponseResult(routingResult.RoutingError.ToToolResponse()), nil
	}

	// Validate resource is in resolved context
	// For read-only exec, we allow if ANY resource has been discovered in the session
	validation := e.validateResolvedResourceForExec(targetHost, command, true)
	if validation.IsBlocked() {
		return NewToolResponseResult(validation.StrictError.ToToolResponse()), nil
	}

	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	// Resolve target to the correct agent and routing info (with full provenance)
	routing := e.resolveTargetForCommandFull(targetHost)
	if routing.AgentID == "" {
		if routing.TargetType == "container" || routing.TargetType == "vm" {
			return NewErrorResult(fmt.Errorf("'%s' is a %s but no agent is available on its Proxmox host", targetHost, routing.TargetType)), nil
		}
		return NewErrorResult(fmt.Errorf("no agent available for target '%s'. %s", targetHost, formatAvailableAgentHosts(e.agentServer.GetConnectedAgents()))), nil
	}

	// Build command (with optional Docker wrapper)
	execCommand := command
	if dockerContainer != "" {
		// Validate container name
		if !isValidContainerName(dockerContainer) {
			return NewErrorResult(fmt.Errorf("invalid docker_container name")), nil
		}
		execCommand = fmt.Sprintf("docker exec %s sh -c %s", shellEscape(dockerContainer), shellEscape(command))
	}

	log.Debug().
		Str("command", truncateCommand(command, 100)).
		Str("target", targetHost).
		Str("agent", routing.AgentID).
		Str("agent_host", routing.AgentHostname).
		Str("target_type", routing.TargetType).
		Str("target_id", routing.TargetID).
		Str("transport", routing.Transport).
		Str("resolved_kind", routing.ResolvedKind).
		Msg("[pulse_read] Executing read-only command")

	result, err := e.agentServer.ExecuteCommand(ctx, routing.AgentID, agentexec.ExecuteCommandPayload{
		Command:    execCommand,
		TargetType: routing.TargetType,
		TargetID:   routing.TargetID,
	})
	if err != nil {
		return NewErrorResult(fmt.Errorf("command execution failed: %w", err)), nil
	}

	output := result.Stdout
	if result.Stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += result.Stderr
	}

	if redacted, n := safety.RedactSensitiveText(output); n > 0 {
		output = redacted + fmt.Sprintf("\n\n[redacted %d sensitive value(s)]", n)
	}

	if result.ExitCode != 0 {
		return NewTextResult(fmt.Sprintf("Command exited with code %d:\n%s", result.ExitCode, output)), nil
	}

	if output == "" {
		return NewTextResult("Command completed successfully (no output)"), nil
	}
	return NewTextResult(output), nil
}

// executeReadFile reads a file's contents
func (e *PulseToolExecutor) executeReadFile(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	path, _ := args["path"].(string)
	targetHost, _ := args["target_host"].(string)
	dockerContainer, _ := args["docker_container"].(string)

	if path == "" {
		return NewErrorResult(fmt.Errorf("path is required for file action")), nil
	}
	if targetHost == "" {
		return NewErrorResult(fmt.Errorf("target_host is required")), nil
	}

	// Validate path is absolute
	if !strings.HasPrefix(path, "/") {
		return NewErrorResult(fmt.Errorf("path must be absolute (start with /)")), nil
	}

	// Note: routing validation is done inside executeFileRead
	// Use the existing file read implementation
	return e.executeFileRead(ctx, path, targetHost, dockerContainer)
}

// executeReadFind finds files by pattern
func (e *PulseToolExecutor) executeReadFind(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	pattern, _ := args["pattern"].(string)
	path, _ := args["path"].(string)
	targetHost, _ := args["target_host"].(string)

	if pattern == "" && path == "" {
		return NewErrorResult(fmt.Errorf("pattern or path is required for find action")), nil
	}
	if targetHost == "" {
		return NewErrorResult(fmt.Errorf("target_host is required")), nil
	}

	// Use pattern if provided, otherwise use path as the pattern
	searchPattern := pattern
	if searchPattern == "" {
		searchPattern = path
	}

	// Extract directory and filename pattern
	dir := "/"
	filePattern := searchPattern
	if lastSlash := strings.LastIndex(searchPattern, "/"); lastSlash > 0 {
		dir = searchPattern[:lastSlash]
		filePattern = searchPattern[lastSlash+1:]
	}

	// Build a safe find command
	// Use -maxdepth to prevent runaway searches
	command := fmt.Sprintf("find %s -maxdepth 3 -name %s -type f 2>/dev/null | head -50",
		shellEscape(dir), shellEscape(filePattern))

	// Execute via read exec
	return e.executeReadExec(ctx, map[string]interface{}{
		"action":      "exec",
		"command":     command,
		"target_host": targetHost,
	})
}

// executeReadTail tails a file
func (e *PulseToolExecutor) executeReadTail(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	path, _ := args["path"].(string)
	targetHost, _ := args["target_host"].(string)
	lines := intArg(args, "lines", 100)
	grepPattern, _ := args["grep"].(string)
	dockerContainer, _ := args["docker_container"].(string)

	if path == "" {
		return NewErrorResult(fmt.Errorf("path is required for tail action")), nil
	}
	if targetHost == "" {
		return NewErrorResult(fmt.Errorf("target_host is required")), nil
	}

	// Validate path is absolute
	if !strings.HasPrefix(path, "/") {
		return NewErrorResult(fmt.Errorf("path must be absolute (start with /)")), nil
	}

	// Cap lines to prevent memory issues
	if lines > 1000 {
		lines = 1000
	}
	if lines < 1 {
		lines = 100
	}

	// Build command
	command := fmt.Sprintf("tail -n %d %s", lines, shellEscape(path))
	if grepPattern != "" {
		command += fmt.Sprintf(" | grep -i %s", shellEscape(grepPattern))
	}

	return e.executeReadExec(ctx, map[string]interface{}{
		"action":           "exec",
		"command":          command,
		"target_host":      targetHost,
		"docker_container": dockerContainer,
	})
}

// executeReadLogs reads logs from docker or journalctl
func (e *PulseToolExecutor) executeReadLogs(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	source, _ := args["source"].(string)
	source = strings.ToLower(strings.TrimSpace(source))
	targetHost, _ := args["target_host"].(string)
	container, _ := args["container"].(string)
	unit, _ := args["unit"].(string)
	since, _ := args["since"].(string)
	grepPattern, _ := args["grep"].(string)
	lines := intArg(args, "lines", 100)

	if targetHost == "" {
		return NewErrorResult(fmt.Errorf("target_host is required")), nil
	}

	// Cap lines
	if lines > 1000 {
		lines = 1000
	}
	if lines < 1 {
		lines = 100
	}

	var command string

	// If source is omitted, infer from provided identifiers:
	// - container -> docker logs
	// - otherwise -> journal logs
	if source == "" {
		if container != "" {
			source = "docker"
		} else {
			source = "journal"
		}
	}

	switch source {
	case "docker":
		if container == "" {
			// Graceful fallback: list active docker containers/status when no specific
			// container was provided. This keeps read-only workflows moving instead of
			// trapping the model in repeated argument errors.
			command = "docker ps --format '{{.Names}}\t{{.Status}}' | head -20"
			if grepPattern != "" {
				command += fmt.Sprintf(" | grep -i %s", shellEscape(grepPattern))
			}
			return e.executeReadExec(ctx, map[string]interface{}{
				"action":      "exec",
				"command":     command,
				"target_host": targetHost,
			})
		}
		if !isValidContainerName(container) {
			return NewErrorResult(fmt.Errorf("invalid container name")), nil
		}
		command = fmt.Sprintf("docker logs --tail %d %s", lines, shellEscape(container))
		if since != "" {
			command = fmt.Sprintf("docker logs --since %s --tail %d %s", shellEscape(since), lines, shellEscape(container))
		}

	case "journal":
		if unit == "" {
			command = fmt.Sprintf("journalctl -n %d --no-pager", lines)
			if since != "" {
				command = fmt.Sprintf("journalctl --since %s -n %d --no-pager", shellEscape(since), lines)
			}
			break
		}
		command = fmt.Sprintf("journalctl -u %s -n %d --no-pager", shellEscape(unit), lines)
		if since != "" {
			command = fmt.Sprintf("journalctl -u %s --since %s -n %d --no-pager", shellEscape(unit), shellEscape(since), lines)
		}

	default:
		// Unknown source - prefer a safe fallback over hard failure to avoid
		// repeated tool loops caused by minor argument mistakes.
		log.Warn().Str("source", source).Msg("pulse_read logs: unknown source, using journal fallback")
		command = fmt.Sprintf("journalctl -n %d --no-pager", lines)
	}

	// Add grep filter if provided
	if grepPattern != "" {
		command += fmt.Sprintf(" 2>&1 | grep -i %s", shellEscape(grepPattern))
	}

	return e.executeReadExec(ctx, map[string]interface{}{
		"action":      "exec",
		"command":     command,
		"target_host": targetHost,
	})
}

// truncateCommand truncates a command for display/logging
func truncateCommand(cmd string, maxLen int) string {
	if len(cmd) <= maxLen {
		return cmd
	}
	return cmd[:maxLen] + "..."
}

// isValidContainerName validates a container name (alphanumeric, _, -, .)
func isValidContainerName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.') {
			return false
		}
	}
	return true
}
