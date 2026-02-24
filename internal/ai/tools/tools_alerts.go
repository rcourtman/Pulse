package tools

import (
	"context"
	"fmt"
	"strings"
)

// registerAlertsTools registers the pulse_alerts tool
func (e *PulseToolExecutor) registerAlertsTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_alerts",
			Description: `Manage alerts and AI patrol findings.

Actions:
- list: List active threshold alerts (CPU > 80%, disk full, etc.)
- findings: List AI patrol findings (detected issues)
- resolved: List recently resolved alerts
- resolve: Mark a finding as resolved
- dismiss: Dismiss a finding as not an issue

Examples:
- List critical alerts: action="list", severity="critical"
- List all findings: action="findings"
- List resolved: action="resolved"
- Resolve finding: action="resolve", finding_id="abc123", resolution_note="Fixed by restarting service"
- Dismiss finding: action="dismiss", finding_id="abc123", reason="expected_behavior", note="This is normal during maintenance"`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"action": {
						Type:        "string",
						Description: "Alert action to perform",
						Enum:        []string{"list", "findings", "resolved", "resolve", "dismiss"},
					},
					"severity": {
						Type:        "string",
						Description: "Filter by severity: critical, warning, info (for list, findings)",
						Enum:        []string{"critical", "warning", "info"},
					},
					"resource_type": {
						Type:        "string",
						Description: "Filter by resource type: vm, container, node, docker (for findings)",
					},
					"resource_id": {
						Type:        "string",
						Description: "Filter by resource ID (for findings)",
					},
					"finding_id": {
						Type:        "string",
						Description: "Finding ID (for resolve, dismiss)",
					},
					"resolution_note": {
						Type:        "string",
						Description: "Resolution note (for resolve action)",
					},
					"note": {
						Type:        "string",
						Description: "Explanation note (for dismiss action)",
					},
					"reason": {
						Type:        "string",
						Description: "Dismissal reason: not_an_issue, expected_behavior, will_fix_later",
						Enum:        []string{"not_an_issue", "expected_behavior", "will_fix_later"},
					},
					"include_dismissed": {
						Type:        "boolean",
						Description: "Include previously dismissed findings (for findings)",
					},
					"type": {
						Type:        "string",
						Description: "Filter resolved alerts by type",
					},
					"level": {
						Type:        "string",
						Description: "Filter resolved alerts by level: critical, warning",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results (default: 100)",
					},
					"offset": {
						Type:        "integer",
						Description: "Number of results to skip",
					},
				},
				Required: []string{"action"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeAlerts(ctx, args)
		},
	})
}

// executeAlerts routes to the appropriate alerts handler based on action
func (e *PulseToolExecutor) executeAlerts(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	action, _ := args["action"].(string)
	switch action {
	case "list":
		return e.executeListAlerts(ctx, args)
	case "findings":
		return e.executeListFindings(ctx, args)
	case "resolved":
		return e.executeListResolvedAlerts(ctx, args)
	case "resolve":
		return e.executeResolveFinding(ctx, args)
	case "dismiss":
		return e.executeDismissFinding(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("unknown action: %s. Use: list, findings, resolved, resolve, dismiss", action)), nil
	}
}

func (e *PulseToolExecutor) executeListAlerts(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.alertProvider == nil {
		return NewTextResult("Alert data not available."), nil
	}

	severityFilter, _ := args["severity"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)

	allAlerts := e.alertProvider.GetActiveAlerts()

	var filtered []ActiveAlert
	for i, a := range allAlerts {
		if i < offset {
			continue
		}
		if len(filtered) >= limit {
			break
		}
		if severityFilter != "" && a.Severity != severityFilter {
			continue
		}
		filtered = append(filtered, a)
	}

	if filtered == nil {
		filtered = []ActiveAlert{}
	}

	response := AlertsResponse{
		Alerts: filtered,
		Count:  len(filtered),
	}

	if offset > 0 || len(allAlerts) > limit {
		response.Pagination = &PaginationInfo{
			Total:  len(allAlerts),
			Limit:  limit,
			Offset: offset,
		}
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeListFindings(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	includeDismissed, _ := args["include_dismissed"].(bool)
	severityFilter, _ := args["severity"].(string)
	resourceType, _ := args["resource_type"].(string)
	resourceID, _ := args["resource_id"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	resourceID = strings.TrimSpace(resourceID)
	if resourceType != "" {
		validTypes := map[string]bool{"vm": true, "system-container": true, "container": true, "node": true, "docker": true}
		if !validTypes[resourceType] {
			return NewErrorResult(fmt.Errorf("invalid resource_type: %s. Use vm, system-container, node, or docker", resourceType)), nil
		}
	}

	if e.findingsProvider == nil {
		return NewTextResult("Patrol findings not available. Pulse Patrol may not be running."), nil
	}

	allActive := e.findingsProvider.GetActiveFindings()
	var allDismissed []Finding
	if includeDismissed {
		allDismissed = e.findingsProvider.GetDismissedFindings()
	}

	normalizeType := func(value string) string {
		normalized := strings.ToLower(strings.TrimSpace(value))
		switch normalized {
		case "docker container", "docker-container", "docker_container":
			return "docker"
		case "system-container", "lxc", "lxc container", "lxc-container", "lxc_container", "container":
			return "system-container"
		default:
			return normalized
		}
	}

	matches := func(f Finding) bool {
		if severityFilter != "" && f.Severity != severityFilter {
			return false
		}
		if resourceID != "" && f.ResourceID != resourceID {
			return false
		}
		if resourceType != "" && normalizeType(f.ResourceType) != resourceType {
			return false
		}
		return true
	}

	// Filter active
	var active []Finding
	totalActive := 0
	for _, f := range allActive {
		if !matches(f) {
			continue
		}
		if totalActive < offset {
			totalActive++
			continue
		}
		if len(active) >= limit {
			totalActive++
			continue
		}
		active = append(active, f)
		totalActive++
	}

	// Filter dismissed
	var dismissed []Finding
	totalDismissed := 0
	if includeDismissed {
		for _, f := range allDismissed {
			if !matches(f) {
				continue
			}
			if totalDismissed < offset {
				totalDismissed++
				continue
			}
			if len(dismissed) >= limit {
				totalDismissed++
				continue
			}
			dismissed = append(dismissed, f)
			totalDismissed++
		}
	}

	if active == nil {
		active = []Finding{}
	}
	if dismissed == nil {
		dismissed = []Finding{}
	}

	response := FindingsResponse{
		Active:    active,
		Dismissed: dismissed,
		Counts: FindingCounts{
			Active:    totalActive,
			Dismissed: totalDismissed,
		},
	}

	total := totalActive + totalDismissed
	if offset > 0 || total > limit {
		response.Pagination = &PaginationInfo{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		}
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeResolveFinding(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	findingID, _ := args["finding_id"].(string)
	resolutionNote, _ := args["resolution_note"].(string)

	if findingID == "" {
		return NewErrorResult(fmt.Errorf("finding_id is required")), nil
	}
	if resolutionNote == "" {
		return NewErrorResult(fmt.Errorf("resolution_note is required")), nil
	}

	if e.findingsManager == nil {
		return NewTextResult("Findings manager not available."), nil
	}

	if err := e.findingsManager.ResolveFinding(findingID, resolutionNote); err != nil {
		return NewErrorResult(err), nil
	}

	return NewJSONResult(map[string]interface{}{
		"success":         true,
		"finding_id":      findingID,
		"action":          "resolved",
		"resolution_note": resolutionNote,
		"verification":    map[string]interface{}{"ok": true, "method": "store"},
	}), nil
}

func (e *PulseToolExecutor) executeDismissFinding(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	findingID, _ := args["finding_id"].(string)
	reason, _ := args["reason"].(string)
	note, _ := args["note"].(string)

	if findingID == "" {
		return NewErrorResult(fmt.Errorf("finding_id is required")), nil
	}
	if reason == "" {
		return NewErrorResult(fmt.Errorf("reason is required")), nil
	}
	if note == "" {
		return NewErrorResult(fmt.Errorf("note is required")), nil
	}

	if e.findingsManager == nil {
		return NewTextResult("Findings manager not available."), nil
	}

	if err := e.findingsManager.DismissFinding(findingID, reason, note); err != nil {
		return NewErrorResult(err), nil
	}

	return NewJSONResult(map[string]interface{}{
		"success":      true,
		"finding_id":   findingID,
		"action":       "dismissed",
		"reason":       reason,
		"note":         note,
		"verification": map[string]interface{}{"ok": true, "method": "store"},
	}), nil
}

// ========== Resolved Alerts Tool Implementation ==========

func (e *PulseToolExecutor) executeListResolvedAlerts(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	typeFilter, _ := args["type"].(string)
	levelFilter, _ := args["level"].(string)
	limit := intArg(args, "limit", 50)

	if e.alertProvider == nil {
		return NewTextResult("Alert provider not available."), nil
	}

	recentlyResolved := e.alertProvider.GetRecentlyResolved(24 * 60) // past 24 hours

	if len(recentlyResolved) == 0 {
		return NewTextResult("No recently resolved alerts."), nil
	}

	var alerts []ResolvedAlertSummary

	for _, alert := range recentlyResolved {
		// Apply filters
		if typeFilter != "" && !strings.EqualFold(alert.Type, typeFilter) {
			continue
		}
		if levelFilter != "" && !strings.EqualFold(alert.Level, levelFilter) {
			continue
		}

		if len(alerts) >= limit {
			break
		}

		alerts = append(alerts, ResolvedAlertSummary{
			ID:           alert.ID,
			Type:         alert.Type,
			Level:        alert.Level,
			ResourceID:   alert.ResourceID,
			ResourceName: alert.ResourceName,
			Node:         alert.Node,
			Instance:     alert.Instance,
			Message:      alert.Message,
			Value:        alert.Value,
			Threshold:    alert.Threshold,
			StartTime:    alert.StartTime,
			ResolvedTime: alert.ResolvedTime,
		})
	}

	if alerts == nil {
		alerts = []ResolvedAlertSummary{}
	}

	response := ResolvedAlertsResponse{
		Alerts: alerts,
		Total:  len(recentlyResolved),
	}

	return NewJSONResult(response), nil
}
