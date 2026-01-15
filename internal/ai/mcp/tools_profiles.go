package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// registerProfileTools registers agent profile tools
func (e *PulseToolExecutor) registerProfileTools() {
	// Read-only profile tool (always available)
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_get_agent_scope",
			Description: "Get the current unified agent scope (profile assignment and settings).",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"agent_id": {
						Type:        "string",
						Description: "Unified agent ID (preferred if known)",
					},
					"hostname": {
						Type:        "string",
						Description: "Hostname or display name to resolve the agent ID",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetAgentScope(ctx, args)
		},
		RequireControl: false,
	})

	// Unified write tool for profile management
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_set_agent_scope",
			Description: "Update a unified agent's scope via profile settings or assign an existing profile. Use this to enable/disable modules like Docker, Kubernetes, or Proxmox.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"agent_id": {
						Type:        "string",
						Description: "Unified agent ID (preferred if known)",
					},
					"hostname": {
						Type:        "string",
						Description: "Hostname or display name to resolve the agent ID",
					},
					"profile_id": {
						Type:        "string",
						Description: "Assign an existing profile ID (optional; omit to use settings)",
					},
					"settings": {
						Type:        "object",
						Description: "Profile settings (e.g., enable_host, enable_docker, enable_kubernetes, enable_proxmox, proxmox_type, docker_runtime, disable_auto_update, disable_docker_update_checks, kube_include_all_pods, kube_include_all_deployments, log_level, interval, report_ip, disable_ceph)",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeSetAgentScope(ctx, args)
		},
		RequireControl: true,
	})
}

func (e *PulseToolExecutor) executeGetAgentScope(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	agentID, _ := args["agent_id"].(string)
	hostname, _ := args["hostname"].(string)
	agentID = strings.TrimSpace(agentID)
	hostname = strings.TrimSpace(hostname)

	if agentID == "" && hostname == "" {
		return NewErrorResult(fmt.Errorf("agent_id or hostname is required")), nil
	}

	agentLabel := agentID
	if agentID == "" {
		if e.stateProvider == nil {
			return NewErrorResult(fmt.Errorf("state provider not available to resolve hostname")), nil
		}
		resolvedID, resolvedLabel := resolveAgentFromHostname(e.stateProvider.GetState(), hostname)
		if resolvedID == "" {
			return NewJSONResult(map[string]interface{}{
				"error":    "not_found",
				"hostname": hostname,
				"message":  fmt.Sprintf("No agent found for hostname '%s'", hostname),
			}), nil
		}
		agentID = resolvedID
		agentLabel = resolvedLabel
	} else if e.stateProvider != nil {
		if resolvedLabel := resolveAgentLabel(e.stateProvider.GetState(), agentID); resolvedLabel != "" {
			agentLabel = resolvedLabel
		}
	}

	response := AgentScopeResponse{
		AgentID:    agentID,
		AgentLabel: agentLabel,
	}

	// Get profile info
	if e.agentProfileManager != nil {
		scope, err := e.agentProfileManager.GetAgentScope(ctx, agentID)
		if err != nil {
			return NewJSONResult(map[string]interface{}{
				"error":       "failed_to_load",
				"agent_id":    agentID,
				"agent_label": agentLabel,
				"message":     fmt.Sprintf("Failed to load agent scope: %v", err),
			}), nil
		}
		if scope != nil {
			response.ProfileID = scope.ProfileID
			response.ProfileName = scope.ProfileName
			response.ProfileVersion = scope.ProfileVersion
			response.Settings = scope.Settings
		}
	}

	// Get observed modules
	if e.stateProvider != nil {
		observed, commandsEnabled := detectAgentModules(e.stateProvider.GetState(), agentID)
		response.ObservedModules = observed
		response.CommandsEnabled = commandsEnabled
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeSetAgentScope(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.agentProfileManager == nil {
		return NewTextResult("Agent scope management is not available."), nil
	}

	// Note: Control level check is now centralized in registry.Execute()

	agentID, _ := args["agent_id"].(string)
	hostname, _ := args["hostname"].(string)
	profileID, _ := args["profile_id"].(string)

	agentID = strings.TrimSpace(agentID)
	hostname = strings.TrimSpace(hostname)
	profileID = strings.TrimSpace(profileID)

	settings := map[string]interface{}{}
	if rawSettings, ok := args["settings"].(map[string]interface{}); ok {
		for key, value := range rawSettings {
			if value != nil {
				settings[key] = value
			}
		}
	}

	if agentID == "" && hostname == "" {
		return NewErrorResult(fmt.Errorf("agent_id or hostname is required")), nil
	}

	agentLabel := agentID
	if agentID == "" {
		if e.stateProvider == nil {
			return NewErrorResult(fmt.Errorf("state provider not available to resolve hostname")), nil
		}
		resolvedID, resolvedLabel := resolveAgentFromHostname(e.stateProvider.GetState(), hostname)
		if resolvedID == "" {
			return NewJSONResult(map[string]interface{}{
				"error":    "not_found",
				"hostname": hostname,
				"message":  fmt.Sprintf("No agent found for hostname '%s'", hostname),
			}), nil
		}
		agentID = resolvedID
		agentLabel = resolvedLabel
	} else if e.stateProvider != nil {
		if resolvedLabel := resolveAgentLabel(e.stateProvider.GetState(), agentID); resolvedLabel != "" {
			agentLabel = resolvedLabel
		}
	}

	if profileID != "" && len(settings) > 0 {
		return NewErrorResult(fmt.Errorf("use either profile_id or settings, not both")), nil
	}

	if e.controlLevel == ControlLevelSuggest {
		if profileID != "" {
			return NewJSONResult(map[string]interface{}{
				"type":        "suggestion",
				"action":      "assign_profile",
				"agent_id":    agentID,
				"agent_label": agentLabel,
				"profile_id":  profileID,
				"message":     fmt.Sprintf("Suggestion: assign profile %s to agent %s.", profileID, agentLabel),
			}), nil
		}
		if len(settings) == 0 {
			return NewErrorResult(fmt.Errorf("settings are required when profile_id is not provided")), nil
		}
		return NewJSONResult(map[string]interface{}{
			"type":        "suggestion",
			"action":      "apply_settings",
			"agent_id":    agentID,
			"agent_label": agentLabel,
			"settings":    settings,
			"message":     fmt.Sprintf("Suggestion: apply agent scope to %s with settings: %s", agentLabel, formatSettingsSummary(settings)),
		}), nil
	}

	if profileID != "" {
		profileName, err := e.agentProfileManager.AssignProfile(ctx, agentID, profileID)
		if err != nil {
			return NewErrorResult(err), nil
		}
		return NewJSONResult(map[string]interface{}{
			"success":      true,
			"action":       "assigned",
			"agent_id":     agentID,
			"agent_label":  agentLabel,
			"profile_id":   profileID,
			"profile_name": profileName,
			"message":      fmt.Sprintf("Assigned profile '%s' to agent %s. Restart the agent to apply changes.", profileName, agentLabel),
		}), nil
	}

	if len(settings) == 0 {
		return NewErrorResult(fmt.Errorf("settings are required when profile_id is not provided")), nil
	}

	newProfileID, profileName, created, err := e.agentProfileManager.ApplyAgentScope(ctx, agentID, agentLabel, settings)
	if err != nil {
		return NewErrorResult(err), nil
	}

	action := "updated"
	if created {
		action = "created"
	}

	return NewJSONResult(map[string]interface{}{
		"success":      true,
		"action":       action,
		"agent_id":     agentID,
		"agent_label":  agentLabel,
		"profile_id":   newProfileID,
		"profile_name": profileName,
		"settings":     settings,
		"message":      fmt.Sprintf("%s profile '%s' and assigned to agent %s. Restart the agent to apply changes.", strings.Title(action), profileName, agentLabel),
	}), nil
}

// Helper functions for profile tools

func resolveAgentFromHostname(state models.StateSnapshot, hostname string) (string, string) {
	needle := strings.TrimSpace(hostname)
	if needle == "" {
		return "", ""
	}
	for _, host := range state.Hosts {
		if strings.EqualFold(host.Hostname, needle) || strings.EqualFold(host.DisplayName, needle) || strings.EqualFold(host.ID, needle) {
			label := firstNonEmpty(host.DisplayName, host.Hostname, host.ID)
			return host.ID, label
		}
	}
	for _, host := range state.DockerHosts {
		if strings.EqualFold(host.Hostname, needle) || strings.EqualFold(host.DisplayName, needle) || strings.EqualFold(host.CustomDisplayName, needle) || strings.EqualFold(host.ID, needle) {
			label := firstNonEmpty(host.CustomDisplayName, host.DisplayName, host.Hostname, host.ID)
			agentID := strings.TrimSpace(host.AgentID)
			if agentID == "" {
				agentID = host.ID
			}
			return agentID, label
		}
	}
	return "", ""
}

func resolveAgentLabel(state models.StateSnapshot, agentID string) string {
	needle := strings.TrimSpace(agentID)
	if needle == "" {
		return ""
	}
	for _, host := range state.Hosts {
		if strings.EqualFold(host.ID, needle) {
			return firstNonEmpty(host.DisplayName, host.Hostname, host.ID)
		}
	}
	for _, host := range state.DockerHosts {
		if strings.EqualFold(host.AgentID, needle) || strings.EqualFold(host.ID, needle) {
			return firstNonEmpty(host.CustomDisplayName, host.DisplayName, host.Hostname, host.ID)
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func formatSettingsSummary(settings map[string]interface{}) string {
	if len(settings) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, settings[key]))
	}
	return strings.Join(parts, ", ")
}

func detectAgentModules(state models.StateSnapshot, agentID string) ([]string, *bool) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, nil
	}

	var modules []string
	var commandsEnabled *bool

	for _, host := range state.Hosts {
		if strings.EqualFold(host.ID, agentID) {
			modules = append(modules, "host")
			val := host.CommandsEnabled
			commandsEnabled = &val
			if host.LinkedNodeID != "" {
				modules = append(modules, "proxmox")
			}
			break
		}
	}

	for _, dockerHost := range state.DockerHosts {
		if strings.EqualFold(dockerHost.AgentID, agentID) || strings.EqualFold(dockerHost.ID, agentID) {
			modules = append(modules, "docker")
			break
		}
	}

	for _, cluster := range state.KubernetesClusters {
		if strings.EqualFold(cluster.AgentID, agentID) {
			modules = append(modules, "kubernetes")
			break
		}
	}

	if len(modules) == 0 {
		return nil, commandsEnabled
	}

	sort.Strings(modules)
	return modules, commandsEnabled
}
