package tools

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// registerControlTools registers the pulse_control tool
func (e *PulseToolExecutor) registerControlTools() {
	e.registry.registerBuiltin(RegisteredTool{
		Definition: Tool{
			Name:        agentcapabilities.PulseControlToolName,
			Description: `Plan one typed action for a canonical resource that explicitly advertises the requested capability. The plan is persisted on Pulse's shared action lifecycle; this tool never executes commands or contacts infrastructure directly.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"type": {
						Type:        "string",
						Description: "Canonical control type. Only resource is supported.",
						Enum:        []string{"resource"},
					},
					"guest_id": {
						Type:        "string",
						Description: "For guest: VMID or name",
					},
					"resource_id": {
						Type:        "string",
						Description: "For resource: discovered resource name or canonical resource ID from pulse_query",
					},
					"action": {
						Type:        "string",
						Description: "Advertised resource capability name, such as start, stop, shutdown, reboot, or restart",
						Enum:        []string{"start", "stop", "shutdown", "reboot", "restart"},
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
		Governance: ToolGovernance{
			ActionMode:      ToolActionWrite,
			ApprovalPolicy:  ToolApprovalActionPlan,
			ApprovalSummary: "hidden in read-only mode; approval required in controlled mode",
			Summary:         "Plans shared Pulse resource actions; approval and execution stay on the canonical action lifecycle.",
		},
	})
}

// executeControl routes to the appropriate control handler based on type
func (e *PulseToolExecutor) executeControl(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	controlType, _ := args["type"].(string)
	switch controlType {
	case "resource":
		return e.executeControlResource(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("control type %q is retired and denied; use type=resource with an advertised capability", controlType)), nil
	}
}

func (e *PulseToolExecutor) executeControlResource(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceRef, _ := args["resource_id"].(string)
	resourceRef = strings.TrimSpace(resourceRef)
	action, _ := args["action"].(string)
	action = strings.TrimSpace(action)
	if resourceRef == "" {
		return NewErrorResult(fmt.Errorf("resource_id is required")), nil
	}
	if action == "" {
		return NewErrorResult(fmt.Errorf("action is required")), nil
	}

	validation := e.validateResolvedResource(resourceRef, action, true)
	if validation.IsBlocked() {
		return NewToolResponseResult(validation.StrictError.ToToolResponse()), nil
	}
	if validation.Resource == nil {
		if validation.ErrorMsg != "" {
			return NewErrorResult(errors.New(validation.ErrorMsg)), nil
		}
		return NewErrorResult(fmt.Errorf("resource '%s' has not been discovered in this session. Resource discovery is required first", resourceRef)), nil
	}
	if validation.ErrorMsg != "" {
		return NewErrorResult(errors.New(validation.ErrorMsg)), nil
	}

	if e.typedActionPlanner == nil {
		return NewErrorResult(fmt.Errorf("canonical action planning is unavailable")), nil
	}
	resourceID := unifiedresources.CanonicalResourceID(validation.Resource.GetResourceID())
	if resourceID == "" {
		return NewErrorResult(fmt.Errorf("resource %q has no canonical resource id", resourceRef)), nil
	}
	plan, err := e.typedActionPlanner.PlanTypedAction(ctx, e.orgID, unifiedresources.ActionRequest{
		RequestID:      uuid.NewString(),
		ResourceID:     resourceID,
		CapabilityName: action,
		Reason:         fmt.Sprintf("Assistant proposed %s for %s", action, resourceID),
		RequestedBy:    "pulse_assistant",
	})
	if err != nil {
		return NewErrorResult(err), nil
	}
	return NewJSONResult(map[string]interface{}{
		"planned":           true,
		"action_id":         plan.ActionID,
		"resource_id":       resourceID,
		"capability":        action,
		"requires_approval": plan.RequiresApproval,
		"approval_policy":   plan.ApprovalPolicy,
		"plan_hash":         plan.PlanHash,
		"expires_at":        plan.ExpiresAt,
		"message":           "Typed action planned. Approval and execution remain on the canonical action lifecycle.",
	}), nil
}

func (e *PulseToolExecutor) executeNativeAppContainerControl(ctx context.Context, resource ResolvedResourceInfo, action string, approvalID string) (CallToolResult, error) {
	if resource == nil {
		return NewErrorResult(fmt.Errorf("resolved resource is nil")), nil
	}
	if e.appContainerActionProvider == nil {
		return NewErrorResult(fmt.Errorf("native app-container control provider is not available")), nil
	}

	validActions := map[string]bool{"start": true, "stop": true, "restart": true}
	if !validActions[action] {
		return NewErrorResult(fmt.Errorf("invalid app-container action: %s. Use start, stop, or restart", action)), nil
	}
	if e.controlLevel == ControlLevelReadOnly {
		return NewTextResult("Resource control actions are not available in read-only mode."), nil
	}

	resourceName := resolvedResourceDisplayName(resource)
	resourceHost := strings.TrimSpace(resource.GetTargetHost())
	command := fmt.Sprintf("truenas app %s %s", action, resourceName)
	if resourceHost != "" {
		command = fmt.Sprintf("%s on %s", command, resourceHost)
	}
	approvalTargetType := "app-container"
	approvalTargetID := strings.TrimSpace(resource.GetResourceID())
	if approvalTargetID == "" {
		approvalTargetID = strings.TrimSpace(resource.GetProviderUID())
	}

	preApproved := consumeApprovalWithValidation(agentcapabilities.WithApprovalArgument(nil, approvalID), e.orgID, command, approvalTargetType, approvalTargetID)
	decision := agentexec.PolicyAllow
	if e.policy != nil {
		decision = e.policy.Evaluate(command)
		if decision == agentexec.PolicyBlock {
			return NewTextResult(formatPolicyBlocked(command, "This action is blocked by security policy")), nil
		}
	}
	requiresApproval := !e.isAutonomous && (e.controlLevel == ControlLevelControlled || decision == agentexec.PolicyRequireApproval)

	if !preApproved && decision == agentexec.PolicyRequireApproval && !e.isAutonomous {
		approvalID = e.createApprovalRecord(command, approvalTargetType, approvalTargetID, resourceName, fmt.Sprintf("%s app-container %s", action, resourceName))
		return NewTextResult(formatAppContainerApprovalNeeded(resourceName, resourceHost, action, command, approvalID)), nil
	}
	if !preApproved && e.controlLevel == ControlLevelControlled {
		approvalID = e.createApprovalRecord(command, approvalTargetType, approvalTargetID, resourceName, fmt.Sprintf("%s app-container %s", action, resourceName))
		return NewTextResult(formatAppContainerApprovalNeeded(resourceName, resourceHost, action, command, approvalID)), nil
	}

	var actionResult *AppContainerActionResult
	executionResult, err := e.executeNativeActionWithAudit(
		ctx,
		"pulse_control",
		strings.TrimSpace(resource.GetResourceID()),
		approvalID,
		requiresApproval,
		map[string]any{
			"action":      action,
			"kind":        resource.GetKind(),
			"providerUid": resource.GetProviderUID(),
			"host":        resourceHost,
			"platform":    "truenas",
			"approvalId":  approvalID,
			"requestedBy": "pulse_control",
		},
		"pulse_control",
		fmt.Sprintf("%s app-container %s", action, resourceName),
		func(ctx context.Context) (*unifiedresources.ExecutionResult, error) {
			result, err := e.appContainerActionProvider.ExecuteAction(ctx, AppContainerActionRequest{
				OrgID:       e.orgID,
				ResourceID:  strings.TrimSpace(resource.GetResourceID()),
				ProviderUID: strings.TrimSpace(resource.GetProviderUID()),
				Name:        resourceName,
				Host:        resourceHost,
				Platform:    "truenas",
				Action:      action,
			})
			if err != nil {
				return nil, err
			}
			actionResult = result
			return &unifiedresources.ExecutionResult{
				Success: true,
				Output:  strings.TrimSpace(result.Output),
			}, nil
		},
	)
	if err != nil {
		return NewErrorResult(err), nil
	}
	if actionResult == nil {
		actionResult = &AppContainerActionResult{
			ResourceID:  strings.TrimSpace(resource.GetResourceID()),
			ProviderUID: strings.TrimSpace(resource.GetProviderUID()),
			Name:        resourceName,
			Host:        resourceHost,
			Platform:    "truenas",
			Action:      action,
			Output:      strings.TrimSpace(executionResult.Output),
		}
	}
	if strings.TrimSpace(actionResult.Output) == "" {
		actionResult.Output = strings.TrimSpace(executionResult.Output)
	}

	response := map[string]interface{}{
		"success":       executionResult.Success,
		"type":          "resource",
		"resource_id":   actionResult.ResourceID,
		"resource":      actionResult.Name,
		"resource_type": resource.GetKind(),
		"provider_uid":  actionResult.ProviderUID,
		"platform":      actionResult.Platform,
		"host":          actionResult.Host,
		"action":        actionResult.Action,
		"status":        actionResult.Status,
		"output":        actionResult.Output,
		"verification": map[string]interface{}{
			"ok":              executionResult.Success,
			"method":          "provider_refresh",
			"observed_status": actionResult.Status,
		},
	}
	return NewJSONResultWithIsError(response, !executionResult.Success), nil
}

func resolvedResourceControlIdentity(resource ResolvedResourceInfo, fallback string) string {
	if resource == nil {
		return strings.TrimSpace(fallback)
	}
	if providerUID := strings.TrimSpace(resource.GetProviderUID()); providerUID != "" {
		return providerUID
	}
	for _, alias := range resource.GetAliases() {
		alias = strings.TrimSpace(alias)
		if alias != "" {
			return alias
		}
	}
	return strings.TrimSpace(fallback)
}

func resolvedResourceDisplayName(resource ResolvedResourceInfo) string {
	if resource == nil {
		return ""
	}
	aliases := resource.GetAliases()
	if len(aliases) > 0 {
		if name := strings.TrimSpace(aliases[0]); name != "" {
			return name
		}
	}
	if providerUID := strings.TrimSpace(resource.GetProviderUID()); providerUID != "" {
		return providerUID
	}
	return strings.TrimSpace(resource.GetResourceID())
}

func (e *PulseToolExecutor) executeRunCommand(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	command, _ := args["command"].(string)
	targetHost, _ := args["target_host"].(string)
	approvalID := agentcapabilities.ApprovalArgument(args)
	auditResourceID := strings.TrimSpace(targetHost)
	if e.isAutonomous {
		return NewErrorResult(fmt.Errorf("raw command execution is unavailable in autonomous model sessions; use a typed governed action proposal")), nil
	}

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
		if validation.Resource != nil && strings.TrimSpace(validation.Resource.GetResourceID()) != "" {
			auditResourceID = strings.TrimSpace(validation.Resource.GetResourceID())
		}

		// Validate routing context - block if targeting a host node when child resources exist
		// This prevents accidentally executing commands on the host when user meant to target a container/VM
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

	requiresApproval := !e.isAutonomous && (e.controlLevel == ControlLevelControlled || decision == agentexec.PolicyRequireApproval)

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
	// If targetHost is a container/VM name, this routes to the host node agent
	// with the correct TargetType and TargetID for pct exec / qm guest exec
	routing := e.resolveTargetForCommandFull(targetHost)
	if routing.AgentID == "" {
		if targetHost != "" {
			if routing.TargetType == "container" || routing.TargetType == "vm" {
				return NewErrorResult(fmt.Errorf("'%s' is a %s but no agent is available on its host node; install Pulse Unified Agent on the node", targetHost, routing.TargetType)), nil
			}
			return NewErrorResult(fmt.Errorf("no agent available for target '%s'. %s", targetHost, formatAvailableAgentHosts(e.agentServer.GetConnectedAgents()))), nil
		}
		return NewErrorResult(fmt.Errorf("no agent available for target")), nil
	}

	approvalTargetType, approvalTargetID, approvalTargetName := approvalTargetForCommand(targetHost, routing)

	// Check if this is a pre-approved execution with command hash validation.
	// Final single-use consumption is owned by agentexec immediately before
	// grant signing and WebSocket dispatch.
	preApproved := consumeApprovalWithValidation(args, e.orgID, command, approvalTargetType, approvalTargetID)

	if !preApproved && e.controlLevel == ControlLevelControlled {
		approvalID := e.createApprovalRecord(command, approvalTargetType, approvalTargetID, approvalTargetName, "Control level requires approval")
		return NewTextResult(formatApprovalNeeded(command, "Control level requires approval", approvalID)), nil
	}
	if !preApproved && decision == agentexec.PolicyRequireApproval {
		approvalID := e.createApprovalRecord(command, approvalTargetType, approvalTargetID, approvalTargetName, "Security policy requires approval")
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

	result, err := e.executeCommandWithAudit(
		ctx,
		"pulse_control",
		func() string {
			if auditResourceID != "" {
				return auditResourceID
			}
			if targetHost != "" {
				return targetHost
			}
			return routing.AgentHostname
		}(),
		approvalID,
		requiresApproval,
		routing.AgentID,
		agentexec.ExecuteCommandPayload{
			Command:    command,
			TargetType: routing.TargetType,
			TargetID:   routing.TargetID,
		},
		"pulse_control",
		fmt.Sprintf("run command %q on %s", command, func() string {
			if strings.TrimSpace(targetHost) != "" {
				return strings.TrimSpace(targetHost)
			}
			return strings.TrimSpace(routing.AgentHostname)
		}()),
	)
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
		targetType = "agent"
	}

	if targetType == "agent" {
		// Agent-level executions do not carry routing.TargetID in ExecuteCommand payload.
		// Use agent ID to bind approvals to a specific connected agent.
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
	approvalID := agentcapabilities.ApprovalArgument(args)

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
		if validation.Resource != nil {
			return NewErrorResult(errors.New(validation.ErrorMsg)), nil
		}
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
		return NewErrorResult(fmt.Errorf("could not find guest '%s': %v", guestID, err)), nil
	}

	// Check if guest is protected
	vmidStr := fmt.Sprintf("%d", guest.VMID)
	for _, protected := range e.protectedGuests {
		if protected == vmidStr || protected == guest.Name {
			return NewErrorResult(fmt.Errorf("guest %s (VMID %d) is protected and cannot be controlled by Pulse Assistant", guest.Name, guest.VMID)), nil
		}
	}

	// Build the command
	cmdTool := "pct"
	if guest.Type == "vm" {
		cmdTool = "qm"
	}

	// For delete action, verify guest is stopped first
	if action == "delete" && guest.Status != "stopped" {
		return NewErrorResult(fmt.Errorf("cannot delete %s (VMID %d) - it is currently %s; stop it first, then try deleting again", guest.Name, guest.VMID, guest.Status)), nil
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
	preApproved := consumeApprovalWithValidation(args, e.orgID, command, guest.Type, approvalTargetID)

	// Check security policy (skip if pre-approved)
	if !preApproved && e.policy != nil {
		decision := e.policy.Evaluate(command)
		if decision == agentexec.PolicyBlock {
			return NewTextResult(formatPolicyBlocked(command, "This command is blocked by security policy")), nil
		}
		if decision == agentexec.PolicyRequireApproval && !e.isAutonomous {
			// Use guest.Node (the Proxmox host) as targetName so approval execution can find the correct agent
			approvalID := e.createApprovalRecord(command, guest.Type, approvalTargetID, guest.Node, fmt.Sprintf("%s guest %s", action, guest.Name))
			return NewTextResult(formatControlApprovalNeeded(guest.Name, guest.VMID, action, command, approvalID)), nil
		}
	}

	// Check control level - this must be outside policy check since policy may be nil (skip if pre-approved)
	if !preApproved && e.controlLevel == ControlLevelControlled {
		// Use guest.Node (the Proxmox host) as targetName so approval execution can find the correct agent
		approvalID := e.createApprovalRecord(command, guest.Type, approvalTargetID, guest.Node, fmt.Sprintf("%s guest %s", action, guest.Name))
		return NewTextResult(formatControlApprovalNeeded(guest.Name, guest.VMID, action, command, approvalID)), nil
	}

	requiresApproval := !e.isAutonomous && e.controlLevel == ControlLevelControlled

	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	agentID := e.findAgentForNode(guest.Node)
	if agentID == "" {
		return NewErrorResult(fmt.Errorf("no agent available on node '%s'; install Pulse Unified Agent on the node to enable control", guest.Node)), nil
	}

	result, err := e.executeCommandWithAudit(
		ctx,
		"pulse_control",
		fmt.Sprintf("%s:%d", guest.Node, guest.VMID),
		approvalID,
		requiresApproval,
		agentID,
		agentexec.ExecuteCommandPayload{
			Command:    command,
			TargetType: "agent",
			TargetID:   "",
		},
		"pulse_control",
		fmt.Sprintf("%s guest %s", action, guest.Name),
	)
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
		TargetType: "agent",
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
	TargetType string // "agent", "container", or "vm"
	TargetID   string // VMID for LXC/VM, empty for agent

	// Provenance info
	AgentHostname string // Hostname of the agent
	ResolvedKind  string // Technology/transport kind: "node", "system-container", "vm", "app-container", "docker-host", "agent" (drives routing decisions)
	ResolvedNode  string // Hypervisor node name (if applicable)
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
		TargetType: "agent",
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
		result.ResolvedKind = "agent"
		return result
	}

	// STEP 1: Consult topology (state) FIRST — this is authoritative.
	// If the state knows about this resource, use topology-based routing.
	// This prevents hostname collisions from masquerading as host targets.
	loc := e.resolveResourceLocation(targetHost)

	if loc.Found {
		// Route based on resource type
		switch loc.ResourceType {
		case "agent":
			for _, agent := range agents {
				if unifiedresources.HostnamesEquivalent(agent.Hostname, loc.TargetHost) || agent.AgentID == loc.TargetID {
					result.AgentID = agent.AgentID
					result.AgentHostname = agent.Hostname
					result.ResolvedKind = "agent"
					return result
				}
			}

		case "node":
			// Direct hypervisor node
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

		case "system-container":
			// System container - route through node agent via pct exec
			nodeAgentID := e.findAgentForNode(loc.Node)
			result.ResolvedKind = "system-container"
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
			// VM - route through node agent via qm guest exec
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

		case "app-container", "docker-host":
			// Docker container or Docker host
			result.ResolvedKind = loc.ResourceType
			result.ResolvedNode = loc.Node

			if loc.DockerHostType == "system-container" {
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
				if unifiedresources.HostnamesEquivalent(agent.Hostname, loc.TargetHost) || agent.AgentID == loc.TargetHost {
					result.AgentID = agent.AgentID
					result.AgentHostname = agent.Hostname
					return result
				}
			}
		}
	}

	// STEP 2: FALLBACK — agent hostname match.
	// Only used when the state doesn't know about this resource at all.
	// This handles standalone hosts without Proxmox topology.
	for _, agent := range agents {
		if unifiedresources.HostnamesEquivalent(agent.Hostname, targetHost) || agent.AgentID == targetHost {
			result.AgentID = agent.AgentID
			result.AgentHostname = agent.Hostname
			result.ResolvedKind = "agent"
			return result
		}
	}

	return result
}

// resolveTargetForCommand resolves a target_host to the correct agent and routing info.
// Uses the authoritative resolveResourceLocation function.
// Returns: agentID, targetType ("agent", "container", or "vm"), targetID (vmid for LXC/VM)
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
	return nil, fmt.Errorf("read state not available")
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
				VMID:       vm.VMID(),
				Name:       vm.Name(),
				Node:       vm.Node(),
				Type:       "vm",
				Technology: "qemu",
				Status:     string(vm.Status()),
				Instance:   vm.Instance(),
			}, nil
		}
	}

	for _, ct := range rs.Containers() {
		if (convErr == nil && ct.VMID() == vmID) || ct.Name() == guestID || ct.ID() == guestID {
			return &GuestInfo{
				VMID:       ct.VMID(),
				Name:       ct.Name(),
				Node:       ct.Node(),
				Type:       "system-container",
				Technology: "lxc",
				Status:     string(ct.Status()),
				Instance:   ct.Instance(),
			}, nil
		}
	}

	return nil, fmt.Errorf("no VM or container found with ID or name '%s'", guestID)
}

func (e *PulseToolExecutor) resolveDockerContainer(containerName, hostName string) (*models.DockerContainer, *models.DockerHost, error) {
	rs, err := e.readStateForControl()
	if err != nil {
		return nil, nil, fmt.Errorf("read state not available: %w", err)
	}

	containersByHost := make(map[string][]*unifiedresources.DockerContainerView)
	for _, container := range rs.DockerContainers() {
		if container == nil {
			continue
		}
		parentID := strings.TrimSpace(container.ParentID())
		if parentID == "" {
			continue
		}
		containersByHost[parentID] = append(containersByHost[parentID], container)
	}

	type dockerMatch struct {
		host      models.DockerHost
		container models.DockerContainer
	}
	matches := []dockerMatch{}

	for _, hostView := range rs.DockerHosts() {
		if hostView == nil {
			continue
		}
		if hostName != "" && !matchesDockerHostFilter(hostView, hostName) {
			continue
		}

		hostModel := dockerHostModelFromView(hostView)
		for _, containerView := range containersByHost[hostView.ID()] {
			container := dockerContainerModelFromView(containerView)
			if container.Name == containerName ||
				container.ID == containerName ||
				strings.HasPrefix(container.ID, containerName) {
				matches = append(matches, dockerMatch{host: hostModel, container: container})
			}
		}
	}

	if hostName != "" {
		if len(matches) == 0 {
			return nil, nil, fmt.Errorf("container '%s' not found on host '%s'", containerName, hostName)
		}
		match := matches[0]
		return &match.container, &match.host, nil
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
		return nil, nil, fmt.Errorf("container '%s' exists on multiple Docker hosts: %s. Specify host", containerName, strings.Join(hostNames, ", "))
	}

	match := matches[0]
	return &match.container, &match.host, nil
}

func matchesDockerHostFilter(host *unifiedresources.DockerHostView, filter string) bool {
	if host == nil {
		return false
	}
	if host.Hostname() == filter || host.Name() == filter {
		return true
	}
	if host.ID() == filter || host.HostSourceID() == filter {
		return true
	}
	return false
}

func dockerHostModelFromView(host *unifiedresources.DockerHostView) models.DockerHost {
	if host == nil {
		return models.DockerHost{}
	}

	hostID := strings.TrimSpace(host.HostSourceID())
	if hostID == "" {
		hostID = strings.TrimSpace(host.ID())
	}

	displayName := strings.TrimSpace(host.Name())
	hostname := strings.TrimSpace(host.Hostname())
	if hostname == "" {
		hostname = displayName
	}

	return models.DockerHost{
		ID:          hostID,
		AgentID:     strings.TrimSpace(host.AgentID()),
		Hostname:    hostname,
		DisplayName: displayName,
		Runtime:     strings.TrimSpace(host.Runtime()),
		Status:      string(host.Status()),
		LastSeen:    host.LastSeen(),
		Command:     host.Command(),
		Security:    host.Security(),
	}
}

func dockerContainerModelFromView(container *unifiedresources.DockerContainerView) models.DockerContainer {
	if container == nil {
		return models.DockerContainer{}
	}

	containerID := strings.TrimSpace(container.ContainerID())
	if containerID == "" {
		containerID = strings.TrimSpace(container.ID())
	}

	state := strings.TrimSpace(container.ContainerState())
	if state == "" {
		state = string(container.Status())
	}

	ports := container.Ports()
	portModels := make([]models.DockerContainerPort, 0, len(ports))
	for _, p := range ports {
		portModels = append(portModels, models.DockerContainerPort{
			PrivatePort: p.PrivatePort,
			PublicPort:  p.PublicPort,
			Protocol:    p.Protocol,
			IP:          p.IP,
		})
	}

	networks := container.Networks()
	networkModels := make([]models.DockerContainerNetworkLink, 0, len(networks))
	for _, n := range networks {
		networkModels = append(networkModels, models.DockerContainerNetworkLink{
			Name: n.Name,
			IPv4: n.IPv4,
			IPv6: n.IPv6,
		})
	}

	mounts := container.Mounts()
	mountModels := make([]models.DockerContainerMount, 0, len(mounts))
	for _, m := range mounts {
		mountModels = append(mountModels, models.DockerContainerMount{
			Type:        m.Type,
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
			RW:          m.RW,
		})
	}

	var updateStatus *models.DockerContainerUpdateStatus
	if status := container.UpdateStatus(); status != nil {
		updateStatus = &models.DockerContainerUpdateStatus{
			UpdateAvailable: status.UpdateAvailable,
			CurrentDigest:   status.CurrentDigest,
			LatestDigest:    status.LatestDigest,
			Error:           status.Error,
		}
	}

	return models.DockerContainer{
		ID:            containerID,
		Name:          container.Name(),
		Image:         container.Image(),
		State:         state,
		Health:        container.Health(),
		CPUPercent:    container.CPUPercent(),
		MemoryUsage:   container.MemoryUsed(),
		MemoryLimit:   container.MemoryTotal(),
		MemoryPercent: container.MemoryPercent(),
		UptimeSeconds: container.UptimeSeconds(),
		RestartCount:  container.RestartCount(),
		ExitCode:      container.ExitCode(),
		Ports:         portModels,
		Labels:        container.Labels(),
		Networks:      networkModels,
		Mounts:        mountModels,
		UpdateStatus:  updateStatus,
	}
}

func (e *PulseToolExecutor) findAgentForNode(nodeName string) string {
	if e.agentServer == nil {
		return ""
	}

	agents := e.agentServer.GetConnectedAgents()
	for _, agent := range agents {
		if unifiedresources.HostnamesEquivalent(agent.Hostname, nodeName) {
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
		if !unifiedresources.HostnamesEquivalent(nodeNamesByID[linked], nodeName) {
			continue
		}
		for _, agent := range agents {
			if unifiedresources.HostnamesEquivalent(agent.Hostname, host.Hostname()) || agent.AgentID == host.ID() {
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
		if unifiedresources.HostnamesEquivalent(agent.Hostname, dockerHost.Hostname) {
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
// with full provenance metadata. If the Docker host is actually a system container or VM,
// it routes through the node agent with the correct TargetType and TargetID so commands
// are executed inside the guest.
func (e *PulseToolExecutor) resolveDockerHostRoutingFull(dockerHost *models.DockerHost) CommandRoutingResult {
	result := CommandRoutingResult{
		TargetType: "agent",
		Transport:  "direct",
	}

	if e.agentServer == nil {
		return result
	}

	// STEP 1: Check topology — is the Docker host actually a system container or VM?
	if rs, err := e.readStateForControl(); err == nil {
		// Check system containers
		for _, ct := range rs.Containers() {
			if ct.Name() == dockerHost.Hostname {
				result.ResolvedKind = "system-container"
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
	result.ResolvedKind = "docker-host"
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
	return createApprovalRecordForOrg("", command, targetType, targetID, targetName, context)
}

func createApprovalRecordForOrg(orgID, command, targetType, targetID, targetName, context string) string {
	return createApprovalRecordForOrgWithExecutor(nil, orgID, command, targetType, targetID, targetName, context)
}

func (e *PulseToolExecutor) createApprovalRecord(command, targetType, targetID, targetName, context string) string {
	orgID := ""
	if e != nil {
		orgID = e.orgID
	}
	return createApprovalRecordForOrgWithExecutor(e, orgID, command, targetType, targetID, targetName, context)
}

func createApprovalRecordForOrgWithExecutor(e *PulseToolExecutor, orgID, command, targetType, targetID, targetName, context string) string {
	store := approval.GetStore()
	if store == nil {
		log.Debug().Msg("approval store not available, approval will not be persisted")
		return ""
	}

	approvalID := uuid.NewString()
	now := time.Now().UTC()
	req := &approval.ApprovalRequest{
		ID:         approvalID,
		OrgID:      strings.TrimSpace(orgID),
		Command:    command,
		TargetType: targetType,
		TargetID:   targetID,
		TargetName: targetName,
		Context:    context,
	}
	AttachApprovalActionPlan(req, now)

	if err := store.CreateApproval(req); err != nil {
		log.Warn().Err(err).Msg("failed to create approval record")
		return ""
	}
	if req.Plan != nil {
		req.Plan.ExpiresAt = req.ExpiresAt.UTC()
		if req.Plan.Message == "" {
			req.Plan.Message = strings.TrimSpace(req.Context)
		}
	}

	log.Debug().Str("approval_id", req.ID).Str("command", command).Msg("created approval record")
	if e != nil {
		e.recordPendingApprovalAction(req)
	}
	return req.ID
}

func approvalBlastRadius(targetType, command string) []string {
	targetType = strings.ToLower(strings.TrimSpace(targetType))
	commandLower := strings.ToLower(strings.TrimSpace(command))
	switch {
	case strings.Contains(commandLower, "delete") || strings.HasPrefix(commandLower, "rm ") || strings.Contains(commandLower, " rm "):
		return []string{"destructive target change"}
	case strings.Contains(commandLower, "restart") || strings.Contains(commandLower, "reboot") || strings.Contains(commandLower, "shutdown"):
		return []string{"service interruption on target"}
	case targetType == "file":
		return []string{"file contents on target"}
	case targetType == "kubernetes":
		return []string{"kubernetes workload state"}
	case targetType == "docker":
		return []string{"container runtime state"}
	default:
		return []string{"target resource state"}
	}
}

func approvalRollbackAvailable(targetType, command string) bool {
	targetType = strings.ToLower(strings.TrimSpace(targetType))
	commandLower := strings.ToLower(strings.TrimSpace(command))
	if targetType == "file" || strings.Contains(commandLower, "delete") || strings.HasPrefix(commandLower, "rm ") || strings.Contains(commandLower, " rm ") {
		return false
	}
	return strings.Contains(commandLower, "restart") || strings.Contains(commandLower, "start") || strings.Contains(commandLower, "stop") || targetType == "docker"
}

func approvalContextConfidence(req *approval.ApprovalRequest) *approval.ContextConfidence {
	if req == nil {
		return nil
	}
	targetType := strings.TrimSpace(req.TargetType)
	targetID := strings.TrimSpace(req.TargetID)
	targetName := strings.TrimSpace(req.TargetName)
	evidence := make([]string, 0, 3)
	if targetType != "" {
		evidence = append(evidence, fmt.Sprintf("Target type resolved as %s.", targetType))
	}
	if targetID != "" {
		evidence = append(evidence, fmt.Sprintf("Target identifier bound to %s.", targetID))
	}
	if targetName != "" {
		evidence = append(evidence, fmt.Sprintf("Display target resolved as %s.", targetName))
	}

	level := approval.ContextConfidenceUnknown
	summary := "Pulse Assistant could not bind this action to a resolved target."
	switch {
	case targetType != "" && targetID != "":
		level = approval.ContextConfidenceVerified
		summary = "Target was resolved to a concrete resource before approval."
	case targetType != "" && targetName != "":
		level = approval.ContextConfidencePartial
		summary = "Target type and display name were resolved before approval."
	case targetType != "" || targetName != "":
		level = approval.ContextConfidenceInferred
		summary = "Target context was inferred from the requested action."
	}

	return &approval.ContextConfidence{
		Level:    level,
		Summary:  summary,
		Evidence: evidence,
	}
}

func approvalPreflight(req *approval.ApprovalRequest) *approval.ActionPreflight {
	if req == nil {
		return nil
	}
	target := approvalPreflightTarget(req)
	intendedChange := strings.TrimSpace(req.Context)
	if intendedChange == "" {
		intendedChange = strings.TrimSpace(req.Command)
	}
	dryRunAvailable := approvalDryRunAvailable(req.TargetType, req.Command)
	dryRunSummary := "No provider-supported dry run is available for this action; Pulse will hold execution until approval and validate the approval binding before dispatch."
	if dryRunAvailable {
		dryRunSummary = "Provider dry-run semantics are available for this action class before execution."
	}

	// Default safety / verification copy applies to every action. Per-command
	// class additions enrich it with concrete operational context the
	// operator needs before approving (what "restart this service" actually
	// touches, how Pulse will read back success).
	safetyChecks := []string{
		"Approval is scoped to the current organization.",
		"Command hash must match before execution.",
		"Approval can be consumed only once.",
		"Target type and identifier must match the planned action.",
	}
	verificationSteps := []string{
		"Persist unified action audit lifecycle.",
		"Dispatch only after approval is granted.",
		"Capture command result or execution error.",
		"Require Assistant read-after-write verification before final response.",
	}
	if extraSafety, extraVerify := approvalCommandClassPreflightAdditions(req.TargetType, req.Command); len(extraSafety) > 0 || len(extraVerify) > 0 {
		safetyChecks = append(safetyChecks, extraSafety...)
		verificationSteps = append(verificationSteps, extraVerify...)
	}

	return &approval.ActionPreflight{
		Target:            target,
		CurrentState:      fmt.Sprintf("Resolved approval target: %s.", target),
		IntendedChange:    intendedChange,
		DryRunAvailable:   dryRunAvailable,
		DryRunSummary:     dryRunSummary,
		SafetyChecks:      safetyChecks,
		VerificationSteps: verificationSteps,
		GeneratedAt:       time.Now().UTC(),
	}
}

// VerificationCommandForCommand derives the read-after-write check Pulse
// will run after a successful dispatch. The check is keyed on the same
// command class as approvalCommandClassPreflightAdditions, so the
// verification narrative the operator saw at approval time and the
// verification actually executed stay coherent. Returns "", false for
// classes without a derivable check; the broker must skip verification
// rather than fabricate one.
//
// Tool-layer-verified classes are intentionally excluded here:
//   - Container classes (container-restart, container-stop) are verified
//     by pulse_docker via per-container `docker inspect` at the tool
//     layer.
//   - Proxmox VM/CT classes (proxmox-vm-*, proxmox-ct-*) are verified by
//     pulse_control via `verifyGuestAction` (which dispatches `qm status`
//     or `pct status` already) at the tool layer.
//
// Adding a parallel broker-level dispatch for these would double-run the
// same check. The preflight copy authored by
// approvalCommandClassPreflightAdditions still names what the
// tool-layer verification will read so the operator-facing narrative
// stays accurate.
func VerificationCommandForCommand(targetType, command string) (string, bool) {
	class := classifyApprovalCommand(targetType, command)
	switch class {
	case "service-restart", "service-start", "service-reload", "service-stop":
		unit := extractServiceUnitName(command)
		if unit == "" {
			return "", false
		}
		return "systemctl is-active " + shellQuoteSingle(unit), true
	}
	return "", false
}

// extractServiceUnitName parses the unit name out of a systemctl/service
// command. Supports the common shapes:
//
//	"systemctl restart nginx"   -> "nginx"
//	"systemctl reload my.service" -> "my.service"
//	"service restart redis"     -> "redis"
//
// Returns empty string when the command does not parse cleanly so the
// caller can decline to verify rather than guess at the unit.
func extractServiceUnitName(command string) string {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) < 3 {
		return ""
	}
	first := strings.ToLower(fields[0])
	if first != "systemctl" && first != "service" {
		return ""
	}
	// Service action verbs (restart/start/stop/reload) live at index 1 for
	// "systemctl restart nginx" and at index 2 for "service restart nginx";
	// the canonical form we recognize puts the action at index 1 and the
	// unit at index 2.
	verb := strings.ToLower(fields[1])
	switch verb {
	case "restart", "start", "stop", "reload":
		return fields[2]
	}
	return ""
}

// shellQuoteSingle wraps the value in single quotes and escapes any
// embedded single quotes for shell-safe inclusion in the verification
// command. The verification path runs through the same agent dispatch as
// the original command, so the same shell-escape contract applies.
func shellQuoteSingle(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// classifyApprovalCommand maps a (targetType, command) pair to a stable
// class string used by approvalCommandClassPreflightAdditions. Returns
// empty string for commands that do not match a known class — those keep
// the default preflight content rather than getting fabricated extras.
func classifyApprovalCommand(targetType, command string) string {
	cmd := strings.ToLower(strings.TrimSpace(command))
	if cmd == "" {
		return ""
	}
	switch {
	case strings.Contains(cmd, "systemctl restart"), strings.Contains(cmd, "service restart"):
		return "service-restart"
	case strings.Contains(cmd, "systemctl stop"), strings.Contains(cmd, "service stop"):
		return "service-stop"
	case strings.Contains(cmd, "systemctl start"), strings.Contains(cmd, "service start"):
		return "service-start"
	case strings.Contains(cmd, "systemctl reload"):
		return "service-reload"
	case strings.Contains(cmd, "docker restart"), strings.Contains(cmd, "podman restart"):
		return "container-restart"
	case strings.Contains(cmd, "docker stop"), strings.Contains(cmd, "podman stop"):
		return "container-stop"
	case strings.HasPrefix(cmd, "kubectl rollout restart"):
		return "k8s-rollout-restart"
	// Proxmox VM lifecycle (qm) — Pulse's primary monitored platform. Each
	// verb has distinct operational semantics (graceful shutdown vs hard
	// stop vs reboot) so they map to distinct classes rather than being
	// folded into the generic service-* buckets.
	case strings.HasPrefix(cmd, "qm reboot ") || strings.HasPrefix(cmd, "qm restart "):
		return "proxmox-vm-reboot"
	case strings.HasPrefix(cmd, "qm stop "):
		return "proxmox-vm-stop"
	case strings.HasPrefix(cmd, "qm start "):
		return "proxmox-vm-start"
	case strings.HasPrefix(cmd, "qm shutdown "):
		return "proxmox-vm-shutdown"
	// Proxmox LXC container lifecycle (pct). Same per-verb split as qm
	// because pct shutdown initiates an in-guest shutdown via lxc-attach
	// while pct stop is an immediate halt.
	case strings.HasPrefix(cmd, "pct reboot ") || strings.HasPrefix(cmd, "pct restart "):
		return "proxmox-ct-reboot"
	case strings.HasPrefix(cmd, "pct stop "):
		return "proxmox-ct-stop"
	case strings.HasPrefix(cmd, "pct start "):
		return "proxmox-ct-start"
	case strings.HasPrefix(cmd, "pct shutdown "):
		return "proxmox-ct-shutdown"
	}
	return ""
}

// approvalCommandClassPreflightAdditions returns hand-authored safety and
// verification additions for known command classes. The text is concrete
// operational context the operator wants before approving — what the
// command actually touches, how Pulse will read back success — rather
// than generic boilerplate. Unknown classes return empty slices so the
// default preflight is not padded with fabricated assertions.
func approvalCommandClassPreflightAdditions(targetType, command string) (safetyChecks []string, verificationSteps []string) {
	switch classifyApprovalCommand(targetType, command) {
	case "service-restart":
		return []string{
				"Service will be briefly unavailable during the systemctl restart; no other unit dependencies are altered.",
				"Restart applies only to the named unit on the named host; no fleet-wide propagation.",
			}, []string{
				"Read back `systemctl is-active <unit>` after dispatch.",
				"Tail recent journal entries for the unit to confirm a clean restart and no crash loop.",
			}
	case "service-stop":
		return []string{
				"Service will remain stopped after dispatch; restart requires a follow-up approved action.",
				"Stopping the unit may impact dependent services; verify the dependency tree before approving.",
			}, []string{
				"Read back `systemctl is-active <unit>` and confirm `inactive`.",
				"Check journal for any auto-restart triggered by Restart=always policy.",
			}
	case "service-start":
		return []string{
				"Start applies only to the named unit; no other state changes.",
				"Unit will run with the configuration currently on disk; uncommitted config drift will take effect.",
			}, []string{
				"Read back `systemctl is-active <unit>` and confirm `active`.",
				"Tail journal for startup errors before the first request lands.",
			}
	case "service-reload":
		return []string{
				"Reload reads new configuration without dropping in-flight requests when supported by the unit.",
				"Units without reload semantics fall back to restart; check unit type before approving.",
			}, []string{
				"Read back `systemctl status <unit>` and confirm `Active: active (running)` post-reload.",
				"Tail recent journal entries for configuration parse errors.",
			}
	case "container-restart":
		return []string{
				"Container will be briefly unavailable during the restart; image and volume mounts are unchanged.",
				"Restart applies only to the named container; no compose-stack-wide propagation.",
			}, []string{
				"Read back container state via `docker inspect` (or podman) and confirm `Status: running`.",
				"Tail recent container logs for crash patterns or startup errors.",
			}
	case "container-stop":
		return []string{
				"Container will remain stopped after dispatch; image and volume mounts persist for next start.",
				"Stopping a container may impact dependent containers in the same compose stack.",
			}, []string{
				"Read back container state and confirm `Status: exited`.",
				"Check for any restart policy that may auto-restart the container.",
			}
	case "k8s-rollout-restart":
		return []string{
				"Pods are rolled in waves per the deployment strategy; service availability is preserved when readiness probes pass.",
				"PodDisruptionBudget and HPA-driven scaling continue to apply during the rollout.",
			}, []string{
				"Watch `kubectl rollout status` until the deployment converges.",
				"Verify pod readiness with `kubectl get pods -l <selector>` after rollout.",
			}
	case "proxmox-vm-reboot":
		return []string{
				"`qm reboot` performs an ACPI shutdown followed by a start; in-guest workloads see a clean OS reboot.",
				"VM RAM state is not preserved (no live migration); guests with non-persistent state will lose it.",
			}, []string{
				"Read back `qm status <vmid>` and confirm `status: running`.",
				"Verify the guest's monitoring agent reconnects (Pulse will detect and update the resource state).",
			}
	case "proxmox-vm-stop":
		return []string{
				"`qm stop` is an immediate hard stop, NOT a graceful shutdown — guest filesystems may be left dirty.",
				"Use `qm shutdown` instead if the workload needs to flush state cleanly; only approve `qm stop` when the guest is unresponsive.",
			}, []string{
				"Read back `qm status <vmid>` and confirm `status: stopped`.",
				"Verify no auto-start policy will immediately restart the VM.",
			}
	case "proxmox-vm-start":
		return []string{
				"VM will boot from disk with the configuration currently on the host; uncommitted config drift will take effect.",
				"Boot order, attached disks, and network bridges all apply as currently defined — verify before approving.",
			}, []string{
				"Read back `qm status <vmid>` and confirm `status: running`.",
				"Tail `journalctl -u qmeventd` for the start lifecycle event.",
			}
	case "proxmox-vm-shutdown":
		return []string{
				"`qm shutdown` issues an ACPI shutdown to the guest and waits for the VM to power off cleanly.",
				"Default timeout is 60s; an unresponsive guest will force a stop after the timeout, leaving filesystems in the same state as `qm stop`.",
			}, []string{
				"Read back `qm status <vmid>` and confirm `status: stopped`.",
				"Verify the shutdown was clean (exit code 0) rather than a forced stop after timeout.",
			}
	case "proxmox-ct-reboot":
		return []string{
				"`pct reboot` performs an in-guest reboot; container processes restart while the LXC instance stays managed by Proxmox.",
				"Mounted bind-mounts and shared volumes persist through the reboot; only in-memory state is lost.",
			}, []string{
				"Read back `pct status <ctid>` and confirm `status: running`.",
				"Verify the container's foreground service has come back up before treating the reboot as complete.",
			}
	case "proxmox-ct-stop":
		return []string{
				"`pct stop` is an immediate hard stop — in-guest processes do not get a chance to flush state cleanly.",
				"Use `pct shutdown` instead for graceful in-guest shutdown; reserve `pct stop` for unresponsive containers.",
			}, []string{
				"Read back `pct status <ctid>` and confirm `status: stopped`.",
				"Verify no on-boot auto-start policy will immediately restart the container.",
			}
	case "proxmox-ct-start":
		return []string{
				"Container will start with the configuration currently on the host; uncommitted config drift takes effect.",
				"Mounted volumes, network bridges, and resource limits all apply as currently defined.",
			}, []string{
				"Read back `pct status <ctid>` and confirm `status: running`.",
				"Verify the container's foreground service is reachable from the host network.",
			}
	case "proxmox-ct-shutdown":
		return []string{
				"`pct shutdown` issues a graceful in-guest shutdown via lxc-attach and waits for the container to halt.",
				"Default timeout falls back to a hard stop on unresponsive containers, leaving processes in the same state as `pct stop`.",
			}, []string{
				"Read back `pct status <ctid>` and confirm `status: stopped`.",
				"Verify the shutdown was clean rather than a forced stop after timeout.",
			}
	}
	return nil, nil
}

func approvalPreflightTarget(req *approval.ApprovalRequest) string {
	if req == nil {
		return "unknown target"
	}
	parts := make([]string, 0, 3)
	if targetType := strings.TrimSpace(req.TargetType); targetType != "" {
		parts = append(parts, targetType)
	}
	if targetName := strings.TrimSpace(req.TargetName); targetName != "" {
		parts = append(parts, targetName)
	}
	if targetID := strings.TrimSpace(req.TargetID); targetID != "" {
		parts = append(parts, targetID)
	}
	if len(parts) == 0 {
		return "Pulse runtime"
	}
	return strings.Join(parts, " / ")
}

func approvalDryRunAvailable(targetType, command string) bool {
	targetType = strings.ToLower(strings.TrimSpace(targetType))
	commandLower := strings.ToLower(strings.TrimSpace(command))
	return targetType == "kubernetes" && strings.Contains(commandLower, "--dry-run")
}

// isPreApproved checks if the args contain a valid, approved approval_id.
// This is used when the agentic loop re-executes a tool after user approval.
// DEPRECATED: Use consumeApprovalWithValidation instead for replay protection.
func isPreApproved(args map[string]interface{}) bool {
	approvalID := agentcapabilities.ApprovalArgument(args)
	if approvalID == "" {
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

// consumeApprovalWithValidation validates that an approved, org-bound record
// exists for this exact command and target. Final single-use consumption now
// happens inside agentexec immediately before grant minting and WebSocket
// dispatch, so no caller can turn a merely nonempty ID into authority.
func consumeApprovalWithValidation(args map[string]interface{}, orgID, command, targetType, targetID string) bool {
	approvalID := agentcapabilities.ApprovalArgument(args)
	if approvalID == "" {
		return false
	}

	store := approval.GetStore()
	if store == nil {
		return false
	}

	req, found := store.GetApproval(approvalID)
	if !found {
		log.Warn().Str("approval_id", approvalID).Msg("failed to find approval for pre-approved execution")
		return false
	}

	if !approval.BelongsToOrg(req, orgID) {
		log.Warn().
			Str("approval_id", approvalID).
			Str("request_org", approval.NormalizeOrgID(req.OrgID)).
			Str("requested_org", approval.NormalizeOrgID(orgID)).
			Msg("cross-org pre-approved execution rejected")
		return false
	}

	if req.Status != approval.StatusApproved || req.Consumed || time.Now().After(req.ExpiresAt) {
		log.Warn().Str("approval_id", approvalID).Str("status", string(req.Status)).Bool("consumed", req.Consumed).Time("expires_at", req.ExpiresAt).Msg("approval is not dispatchable")
		return false
	}
	actualHash := approval.ComputeCommandHash(command, targetType, targetID)
	approvedHash := func() string {
		if strings.TrimSpace(req.CommandHash) != "" {
			return req.CommandHash
		}
		return approval.ComputeCommandHash(req.Command, req.TargetType, req.TargetID)
	}()
	if actualHash != approvedHash {
		log.Warn().Str("approval_id", approvalID).Msg("approval command or target does not match dispatch")
		return false
	}
	return true
}

// Formatting helpers for control tools

func enrichApprovalRequiredPayload(payload map[string]interface{}, approvalID string) map[string]interface{} {
	if payload == nil {
		payload = map[string]interface{}{}
	}
	req := approvalRequestForID(approvalID)
	if req == nil {
		return payload
	}
	if _, ok := payload["risk"]; !ok && req.RiskLevel != "" {
		payload["risk"] = string(req.RiskLevel)
	}
	if _, ok := payload["description"]; !ok && strings.TrimSpace(req.Context) != "" {
		payload["description"] = strings.TrimSpace(req.Context)
	}
	if strings.TrimSpace(req.TargetType) != "" {
		payload["target_type"] = strings.TrimSpace(req.TargetType)
	}
	if strings.TrimSpace(req.TargetID) != "" {
		payload["target_id"] = strings.TrimSpace(req.TargetID)
	}
	if strings.TrimSpace(req.TargetName) != "" {
		payload["target_name"] = strings.TrimSpace(req.TargetName)
	}
	if req.Plan != nil {
		payload["audit_id"] = strings.TrimSpace(req.Plan.ActionID)
		payload["plan"] = map[string]interface{}{
			"action_id":          strings.TrimSpace(req.Plan.ActionID),
			"request_id":         strings.TrimSpace(req.Plan.RequestID),
			"summary":            strings.TrimSpace(req.Plan.Message),
			"requires_approval":  req.Plan.RequiresApproval,
			"approval_policy":    string(req.Plan.ApprovalPolicy),
			"blast_radius":       strings.TrimSpace(strings.Join(req.Plan.PredictedBlastRadius, ", ")),
			"rollback_available": req.Plan.RollbackAvailable,
			"plan_hash":          strings.TrimSpace(req.Plan.PlanHash),
			"expires_at":         req.Plan.ExpiresAt.UTC().Format(time.RFC3339),
		}
	}
	if req.ContextConfidence != nil {
		payload["context_confidence"] = map[string]interface{}{
			"level":    string(req.ContextConfidence.Level),
			"summary":  strings.TrimSpace(req.ContextConfidence.Summary),
			"evidence": append([]string(nil), req.ContextConfidence.Evidence...),
		}
	}
	if req.Preflight != nil {
		payload["preflight"] = map[string]interface{}{
			"target":             strings.TrimSpace(req.Preflight.Target),
			"current_state":      strings.TrimSpace(req.Preflight.CurrentState),
			"intended_change":    strings.TrimSpace(req.Preflight.IntendedChange),
			"dry_run_available":  req.Preflight.DryRunAvailable,
			"dry_run_summary":    strings.TrimSpace(req.Preflight.DryRunSummary),
			"safety_checks":      append([]string(nil), req.Preflight.SafetyChecks...),
			"verification_steps": append([]string(nil), req.Preflight.VerificationSteps...),
			"generated_at":       req.Preflight.GeneratedAt.UTC().Format(time.RFC3339),
		}
	}
	return payload
}

func formatApprovalNeeded(command, reason, approvalID string) string {
	payload := map[string]interface{}{
		"approval_id":    approvalID,
		"command":        command,
		"reason":         reason,
		"how_to_approve": "Click the approval button in the chat to execute this command.",
	}
	payload = enrichApprovalRequiredPayload(payload, approvalID)
	return agentcapabilities.FormatApprovalRequiredToolMarker(payload)
}

func formatPolicyBlocked(command, reason string) string {
	payload := map[string]interface{}{
		"command": command,
		"reason":  reason,
	}
	return agentcapabilities.FormatPolicyBlockedToolMarker(payload)
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
		"approval_id":    approvalID,
		"guest_name":     name,
		"guest_vmid":     vmID,
		"action":         action,
		"command":        command,
		"how_to_approve": "Click the approval button in the chat to execute this action.",
	}
	payload = enrichApprovalRequiredPayload(payload, approvalID)
	return agentcapabilities.FormatApprovalRequiredToolMarker(payload)
}

func formatDockerApprovalNeeded(name, host, action, command, approvalID string) string {
	payload := map[string]interface{}{
		"approval_id":    approvalID,
		"container_name": name,
		"docker_host":    host,
		"action":         action,
		"command":        command,
		"how_to_approve": "Click the approval button in the chat to execute this action.",
	}
	payload = enrichApprovalRequiredPayload(payload, approvalID)
	return agentcapabilities.FormatApprovalRequiredToolMarker(payload)
}

func formatAppContainerApprovalNeeded(name, host, action, command, approvalID string) string {
	payload := map[string]interface{}{
		"approval_id":    approvalID,
		"resource_name":  name,
		"resource_host":  host,
		"resource_type":  "app-container",
		"action":         action,
		"command":        command,
		"how_to_approve": "Click the approval button in the chat to execute this action.",
	}
	payload = enrichApprovalRequiredPayload(payload, approvalID)
	return agentcapabilities.FormatApprovalRequiredToolMarker(payload)
}
