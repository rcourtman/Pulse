package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const dockerContainerLifecycleHandler = "docker.container.lifecycle"

type dockerActionAgentCommander interface {
	ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error)
}

type dockerContainerActionExecutor struct {
	resources *ResourceHandlers
	agents    dockerActionAgentCommander
}

func newDockerContainerActionExecutor(resources *ResourceHandlers, agents dockerActionAgentCommander) ActionExecutor {
	if resources == nil || agents == nil {
		return nil
	}
	return dockerContainerActionExecutor{resources: resources, agents: agents}
}

func (e dockerContainerActionExecutor) ExecuteAction(ctx context.Context, record unified.ActionAuditRecord) (*unified.ExecutionResult, error) {
	record, err := unified.NormalizeActionAuditRecord(record)
	if err != nil {
		return nil, err
	}
	operation := strings.TrimSpace(record.Request.CapabilityName)
	if !isDockerContainerLifecycleOperation(operation) {
		return nil, fmt.Errorf("unsupported docker container lifecycle capability %q", operation)
	}

	resource, err := e.currentDockerContainerResource(ctx, record.Request.ResourceID, operation)
	if err != nil {
		return nil, err
	}
	runtime, err := dockerContainerRuntime(resource)
	if err != nil {
		return nil, err
	}
	containerRef := dockerContainerRef(resource)
	if containerRef == "" {
		return nil, fmt.Errorf("docker container resource %q has no executable container id", record.Request.ResourceID)
	}
	agentID := strings.TrimSpace(resource.Docker.AgentID)
	if agentID == "" {
		return nil, fmt.Errorf("docker container resource %q is not backed by a reporting Pulse agent", record.Request.ResourceID)
	}

	command := fmt.Sprintf("%s %s %s", runtime, operation, shellQuote(containerRef))
	result, err := e.agents.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		RequestID:  record.ID,
		Command:    command,
		ApprovalID: record.ID,
		TargetType: "agent",
		Timeout:    120,
	})
	if err != nil {
		return nil, err
	}

	output := redactActionOutput(commandOutput(result))
	execution := &unified.ExecutionResult{
		Success: result.ExitCode == 0,
		Output:  output,
	}
	if result.ExitCode != 0 {
		execution.ErrorMessage = strings.TrimSpace(firstNonEmpty(result.Error, output, fmt.Sprintf("container %s exited with status %d", operation, result.ExitCode)))
		return execution, nil
	}

	verification := e.verifyContainerState(ctx, agentID, record.ID, runtime, containerRef, operation)
	execution.Verification = verification
	if verification != nil && verification.Ran && !verification.Success {
		execution.Success = false
		execution.ErrorMessage = "container lifecycle action completed but verification did not confirm the expected state"
	}
	return execution, nil
}

func (e dockerContainerActionExecutor) currentDockerContainerResource(ctx context.Context, resourceID, operation string) (unified.Resource, error) {
	if e.resources == nil {
		return unified.Resource{}, fmt.Errorf("resource handler unavailable")
	}
	registry, err := e.resources.buildRegistry(GetOrgID(ctx))
	if err != nil {
		return unified.Resource{}, err
	}
	resource, ok := registry.Get(resourceID)
	if !ok || resource == nil {
		return unified.Resource{}, fmt.Errorf("resource %q is no longer present", resourceID)
	}
	if resource.Type != unified.ResourceTypeAppContainer || resource.Docker == nil {
		return unified.Resource{}, fmt.Errorf("resource %q is not a Docker or Podman container", resourceID)
	}
	if status := resource.SourceStatus[unified.SourceDocker]; strings.EqualFold(strings.TrimSpace(status.Status), "stale") || strings.EqualFold(strings.TrimSpace(status.Status), "offline") {
		return unified.Resource{}, fmt.Errorf("docker inventory for resource %q is %s", resourceID, status.Status)
	}
	if resource.Docker.Security != nil && resource.Docker.Security.MutatingCommandsBlocked {
		reason := strings.TrimSpace(resource.Docker.Security.MutatingCommandsBlockedReason)
		if reason == "" {
			reason = "Docker daemon-mutating commands are disabled for this host"
		}
		return unified.Resource{}, fmt.Errorf("docker container lifecycle action is blocked by host policy: %s", reason)
	}
	if capability, ok := findDockerLifecycleCapability(resource.Capabilities, operation); !ok {
		return unified.Resource{}, fmt.Errorf("resource %q does not currently advertise %s capability", resourceID, operation)
	} else if capability.InternalHandler != dockerContainerLifecycleHandler {
		return unified.Resource{}, fmt.Errorf("resource %q advertises %s through unsupported handler %q", resourceID, operation, capability.InternalHandler)
	}
	return *resource, nil
}

func findDockerLifecycleCapability(capabilities []unified.ResourceCapability, operation string) (unified.ResourceCapability, bool) {
	operation = strings.TrimSpace(operation)
	for _, capability := range capabilities {
		if strings.TrimSpace(capability.Name) == operation {
			return capability, true
		}
	}
	return unified.ResourceCapability{}, false
}

func dockerContainerRuntime(resource unified.Resource) (string, error) {
	if resource.Docker == nil {
		return "", fmt.Errorf("docker resource metadata missing")
	}
	switch strings.ToLower(strings.TrimSpace(firstNonEmpty(resource.Docker.Runtime, resource.Technology))) {
	case "docker":
		return "docker", nil
	case "podman":
		return "podman", nil
	default:
		return "", fmt.Errorf("unsupported container runtime %q", firstNonEmpty(resource.Docker.Runtime, resource.Technology))
	}
}

func dockerContainerRef(resource unified.Resource) string {
	if resource.Docker == nil {
		return ""
	}
	if ref := strings.TrimSpace(resource.Docker.ContainerID); ref != "" {
		return ref
	}
	return strings.TrimSpace(resource.Name)
}

func (e dockerContainerActionExecutor) verifyContainerState(ctx context.Context, agentID, actionID, runtime, containerRef, operation string) *unified.ActionVerificationResult {
	expectRunning := operation == "start" || operation == "restart"
	command := fmt.Sprintf("%s inspect -f '{{.State.Status}} {{.State.Running}}' %s", runtime, shellQuote(containerRef))

	var lastOutput string
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(500 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return &unified.ActionVerificationResult{Ran: false}
			case <-timer.C:
			}
		}

		result, err := e.agents.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
			RequestID:  actionID + "-verify",
			Command:    command,
			ApprovalID: actionID,
			TargetType: "agent",
			Timeout:    30,
		})
		if err != nil {
			return &unified.ActionVerificationResult{Ran: false}
		}
		lastOutput = redactActionOutput(commandOutput(result))
		status, running := parseDockerInspectState(lastOutput)
		if result.ExitCode == 0 && status != "" && running == expectRunning {
			return &unified.ActionVerificationResult{
				Ran:     true,
				Command: command,
				Output:  lastOutput,
				Success: true,
				RanAt:   time.Now().UTC(),
			}
		}
	}

	return &unified.ActionVerificationResult{
		Ran:     true,
		Command: command,
		Output:  lastOutput,
		Success: false,
		RanAt:   time.Now().UTC(),
		Note:    fmt.Sprintf("expected running=%t", expectRunning),
	}
}

func parseDockerInspectState(output string) (string, bool) {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(output)))
	if len(fields) == 0 {
		return "", false
	}
	running := false
	if len(fields) > 1 {
		running = fields[1] == "true"
	}
	return fields[0], running
}

func isDockerContainerLifecycleOperation(operation string) bool {
	switch strings.TrimSpace(operation) {
	case "start", "stop", "restart":
		return true
	default:
		return false
	}
}

func commandOutput(result *agentexec.CommandResultPayload) string {
	if result == nil {
		return ""
	}
	parts := []string{}
	if stdout := strings.TrimSpace(result.Stdout); stdout != "" {
		parts = append(parts, stdout)
	}
	if stderr := strings.TrimSpace(result.Stderr); stderr != "" {
		parts = append(parts, stderr)
	}
	if errText := strings.TrimSpace(result.Error); errText != "" {
		parts = append(parts, errText)
	}
	return strings.Join(parts, "\n")
}

func redactActionOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	redacted, n := safety.RedactSensitiveText(output)
	if n > 0 {
		return strings.TrimSpace(redacted) + fmt.Sprintf("\n\n[redacted %d sensitive value(s)]", n)
	}
	return output
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
