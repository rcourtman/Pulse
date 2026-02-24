package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// registerPatrolTools registers the three patrol-specific tools.
// These tools are only functional during a patrol run when patrolFindingCreator is set.
func (e *PulseToolExecutor) registerPatrolTools() {
	// patrol_report_finding — LLM calls this to create a finding
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "patrol_report_finding",
			Description: `Report an infrastructure finding discovered during patrol investigation.

Call this tool to create a structured finding after you have gathered sufficient evidence via investigation tools.
The finding will be validated against current metrics and deduplicated automatically.

Returns: {"ok": true, "finding_id": "...", "is_new": true/false} on success.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"key": {
						Type:        "string",
						Description: "Stable issue key for deduplication (e.g. high-cpu, high-memory, high-disk, backup-stale, backup-never, storage-high-usage, node-offline, restart-loop, pbs-job-failed)",
					},
					"severity": {
						Type:        "string",
						Description: "Finding severity level. Only report actionable issues — observations and minor notes belong in your analysis text, not as findings.",
						Enum:        []string{"critical", "warning"},
					},
					"category": {
						Type:        "string",
						Description: "Finding category",
						Enum:        []string{"performance", "capacity", "reliability", "backup", "security", "general"},
					},
					"resource_id": {
						Type:        "string",
						Description: "Resource ID (e.g. node/pve1, qemu/100, lxc/101)",
					},
					"resource_name": {
						Type:        "string",
						Description: "Human-readable resource name",
					},
					"resource_type": {
						Type:        "string",
						Description: "Resource type",
						Enum:        []string{"node", "vm", "system-container", "container", "docker_container", "storage", "host", "kubernetes_cluster", "pbs"},
					},
					"title": {
						Type:        "string",
						Description: "Brief finding title (1 sentence)",
					},
					"description": {
						Type:        "string",
						Description: "Detailed description of the issue found",
					},
					"recommendation": {
						Type:        "string",
						Description: "Specific actionable recommendation for the operator",
					},
					"evidence": {
						Type:        "string",
						Description: "Specific data/metrics/commands that support this finding",
					},
				},
				Required: []string{"key", "severity", "category", "resource_id", "resource_name", "resource_type", "title", "description"},
			},
		},
		Handler: handlePatrolReportFinding,
	})

	// patrol_resolve_finding — LLM calls this to resolve an active finding
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "patrol_resolve_finding",
			Description: `Resolve an active patrol finding after verifying the issue is no longer present.

Call this after investigating an existing finding and confirming the issue has been resolved.
Returns: {"ok": true, "resolved": true} on success.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"finding_id": {
						Type:        "string",
						Description: "The ID of the finding to resolve",
					},
					"reason": {
						Type:        "string",
						Description: "Brief explanation of why this finding is being resolved (e.g. 'CPU has returned to 35%, well below threshold')",
					},
				},
				Required: []string{"finding_id", "reason"},
			},
		},
		Handler: handlePatrolResolveFinding,
	})

	// patrol_get_findings — LLM calls this to check existing active findings
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "patrol_get_findings",
			Description: `Get currently active patrol findings. Use this to check what findings already exist before reporting new ones (avoids duplicates) and to identify findings that may need resolution.

Returns a list of active findings with their IDs, severity, resource, and title.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_id": {
						Type:        "string",
						Description: "Filter by resource ID (optional)",
					},
					"severity": {
						Type:        "string",
						Description: "Minimum severity to include (optional, default: info)",
						Enum:        []string{"info", "watch", "warning", "critical"},
					},
				},
			},
		},
		Handler: handlePatrolGetFindings,
	})
}

func handlePatrolReportFinding(_ context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
	creator := e.GetPatrolFindingCreator()
	if creator == nil {
		return NewTextResult("patrol_report_finding is only available during a patrol run."), nil
	}
	if checker, ok := creator.(PatrolFindingsChecker); ok && !checker.HasCheckedFindings() {
		return NewErrorResult(fmt.Errorf("call patrol_get_findings before reporting a finding")), nil
	}

	// Extract required fields
	key, _ := args["key"].(string)
	severity, _ := args["severity"].(string)
	category, _ := args["category"].(string)
	resourceID, _ := args["resource_id"].(string)
	resourceName, _ := args["resource_name"].(string)
	resourceType, _ := args["resource_type"].(string)
	title, _ := args["title"].(string)
	description, _ := args["description"].(string)

	// Validate required fields
	var missing []string
	if key == "" {
		missing = append(missing, "key")
	}
	if severity == "" {
		missing = append(missing, "severity")
	}
	if category == "" {
		missing = append(missing, "category")
	}
	if resourceID == "" {
		missing = append(missing, "resource_id")
	}
	if resourceName == "" {
		missing = append(missing, "resource_name")
	}
	if resourceType == "" {
		missing = append(missing, "resource_type")
	}
	if title == "" {
		missing = append(missing, "title")
	}
	if description == "" {
		missing = append(missing, "description")
	}
	if len(missing) > 0 {
		return NewErrorResult(fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))), nil
	}

	// Validate enums
	validSeverities := map[string]bool{"critical": true, "warning": true}
	if !validSeverities[severity] {
		return NewErrorResult(fmt.Errorf("invalid severity %q: must be critical or warning. For minor observations, include them in your analysis text instead of creating a finding", severity)), nil
	}
	validCategories := map[string]bool{"performance": true, "capacity": true, "reliability": true, "backup": true, "security": true, "general": true}
	if !validCategories[category] {
		return NewErrorResult(fmt.Errorf("invalid category %q: must be performance, capacity, reliability, backup, security, or general", category)), nil
	}

	// Extract optional fields
	recommendation, _ := args["recommendation"].(string)
	evidence, _ := args["evidence"].(string)

	input := PatrolFindingInput{
		Key:            key,
		Severity:       severity,
		Category:       category,
		ResourceID:     resourceID,
		ResourceName:   resourceName,
		ResourceType:   resourceType,
		Title:          title,
		Description:    description,
		Recommendation: recommendation,
		Evidence:       evidence,
	}

	findingID, isNew, err := creator.CreateFinding(input)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to create finding: %w", err)), nil
	}

	result := map[string]interface{}{
		"ok":         true,
		"finding_id": findingID,
		"is_new":     isNew,
	}
	b, _ := json.Marshal(result)
	return NewTextResult(string(b)), nil
}

func handlePatrolResolveFinding(_ context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
	creator := e.GetPatrolFindingCreator()
	if creator == nil {
		return NewTextResult("patrol_resolve_finding is only available during a patrol run."), nil
	}
	if checker, ok := creator.(PatrolFindingsChecker); ok && !checker.HasCheckedFindings() {
		return NewErrorResult(fmt.Errorf("call patrol_get_findings before resolving a finding")), nil
	}

	findingID, _ := args["finding_id"].(string)
	reason, _ := args["reason"].(string)

	if findingID == "" {
		return NewErrorResult(fmt.Errorf("missing required field: finding_id")), nil
	}
	if reason == "" {
		return NewErrorResult(fmt.Errorf("missing required field: reason")), nil
	}

	if err := creator.ResolveFinding(findingID, reason); err != nil {
		return NewErrorResult(fmt.Errorf("failed to resolve finding: %w", err)), nil
	}

	result := map[string]interface{}{
		"ok":       true,
		"resolved": true,
	}
	b, _ := json.Marshal(result)
	return NewTextResult(string(b)), nil
}

func handlePatrolGetFindings(_ context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
	creator := e.GetPatrolFindingCreator()
	if creator == nil {
		return NewTextResult("patrol_get_findings is only available during a patrol run."), nil
	}

	resourceID, _ := args["resource_id"].(string)
	minSeverity, _ := args["severity"].(string)

	findings := creator.GetActiveFindings(resourceID, minSeverity)

	result := map[string]interface{}{
		"ok":       true,
		"count":    len(findings),
		"findings": findings,
	}
	b, _ := json.Marshal(result)
	return NewTextResult(string(b)), nil
}
