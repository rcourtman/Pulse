package tools

import (
	"context"
	"fmt"
)

// registerAlertsTools registers the consolidated pulse_alerts tool
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
// All handler functions are implemented in tools_patrol.go
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
