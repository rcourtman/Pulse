package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

// registerPatrolTools registers the three patrol-specific tools.
// These tools are only functional during a patrol run when patrolFindingCreator is set.
func (e *PulseToolExecutor) registerPatrolTools() {
	// patrol_report_finding — LLM calls this to create a finding
	e.registry.registerBuiltin(RegisteredTool{
		Definition: Tool{
			Name: agentcapabilities.PatrolReportFindingToolName,
			Description: `Report an infrastructure finding discovered during patrol investigation.

Call this tool to create a structured finding after you have gathered sufficient evidence via investigation tools.
The finding will be validated against current metrics and deduplicated automatically.

Returns: {"ok": true, "finding_id": "...", "is_new": true/false} on success.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"key": {
						Type:        "string",
						Description: "Stable issue key for deduplication. Use the canonical key when one fits — cpu-high, memory-high, disk-high, backup-stale, backup-failed, smart-failure, guest-unreachable (these have deterministic verifiers that ground resolution) — otherwise a stable kebab-case key like backup-never, storage-high-usage, node-offline, restart-loop, pbs-job-failed.",
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
						Description: "Resource ID (e.g. node/pve1, vm/100, ct/101)",
					},
					"resource_name": {
						Type:        "string",
						Description: "Human-readable resource name",
					},
					"resource_type": {
						Type:        "string",
						Description: "Canonical v6 resource type",
						Enum:        []string{"node", "vm", "system-container", "app-container", "storage", "physical_disk", "agent", "k8s-cluster", "pbs"},
					},
					"title": {
						Type:        "string",
						Description: "Brief finding title (1 sentence)",
					},
					"description": {
						Type:        "string",
						Description: "Detailed description of the issue found",
					},
					"impact": {
						Type: "string",
						Description: "Operator-facing consequence if ignored: what happens to which workloads, jobs, or data if the operator does nothing. " +
							"Base this on the data you gathered, not on the severity level. Most infrastructure issues have knowable consequences — " +
							"provide this whenever the data supports it. Omit only if the consequence is truly unknowable from available information.",
					},
					"recommendation": {
						Type:        "string",
						Description: "Specific, actionable next step the operator should take. Include verification steps where possible.",
					},
					"evidence": {
						Type: "string",
						Description: "Specific data points, metric values, command outputs, or tool results that led to this finding. " +
							"This is the trust anchor: the operator uses it to verify your conclusion independently. " +
							"Always include the key evidence — if you gathered enough data to create this finding, you have evidence to report.",
					},
				},
				Required: []string{"key", "severity", "category", "resource_id", "resource_name", "resource_type", "title", "description"},
			},
		},
		Handler: handlePatrolReportFinding,
		Governance: ToolGovernance{
			ActionMode:      ToolActionWrite,
			ApprovalPolicy:  ToolApprovalScopeOnly,
			ApprovalSummary: "patrol-only; records a governed finding after evidence collection",
			Summary:         "Creates a structured patrol finding during autonomous investigation.",
		},
	})

	// patrol_resolve_finding — LLM calls this to resolve an active finding
	e.registry.registerBuiltin(RegisteredTool{
		Definition: Tool{
			Name: agentcapabilities.PatrolResolveFindingToolName,
			Description: `Resolve an active patrol finding after verifying the issue is no longer present.

Call this after investigating an existing finding and confirming the issue has been resolved.
Returns: {"ok": true, "resolved": true} on success.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					agentcapabilities.FindingIDArgumentName: {
						Type:        "string",
						Description: "The ID of the finding to resolve",
					},
					agentcapabilities.ReasonArgumentName: {
						Type:        "string",
						Description: "Brief explanation of why this finding is being resolved (e.g. 'CPU has returned to 35%, well below threshold')",
					},
				},
				Required: []string{agentcapabilities.FindingIDArgumentName, agentcapabilities.ReasonArgumentName},
			},
		},
		Handler: handlePatrolResolveFinding,
		Governance: ToolGovernance{
			ActionMode:      ToolActionWrite,
			ApprovalPolicy:  ToolApprovalScopeOnly,
			ApprovalSummary: "patrol-only; resolves a finding after verification",
			Summary:         "Marks a patrol finding resolved after current evidence supports closure.",
		},
	})

	// patrol_get_findings — LLM calls this to check existing active findings
	e.registry.registerBuiltin(RegisteredTool{
		Definition: Tool{
			Name: agentcapabilities.PatrolGetFindingsToolName,
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
		Governance: ToolGovernance{
			ActionMode:      ToolActionRead,
			ApprovalPolicy:  ToolApprovalScopeOnly,
			ApprovalSummary: "no approval required",
			Summary:         "Reads active patrol findings for deduplication and investigation context.",
		},
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
	resourceType = canonicalPatrolResourceType(resourceType)
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
	if isUnsupportedLegacyResourceTypeToken(resourceType) {
		return NewErrorResult(fmt.Errorf("unsupported resource_type %q", resourceType)), nil
	}
	validResourceTypes := map[string]bool{
		"node":             true,
		"vm":               true,
		"system-container": true,
		"app-container":    true,
		"storage":          true,
		"physical_disk":    true,
		"agent":            true,
		"k8s-cluster":      true,
		"pbs":              true,
	}
	if !validResourceTypes[resourceType] {
		return NewErrorResult(fmt.Errorf("unsupported resource_type %q: use node, vm, system-container, app-container, storage, physical_disk, agent, k8s-cluster, or pbs", resourceType)), nil
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
	impact, _ := args["impact"].(string)
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
		Impact:         impact,
		Recommendation: recommendation,
		Evidence:       evidence,
	}

	findingID, isNew, err := creator.CreateFinding(input)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to create finding: %w", err)), nil
	}

	result := map[string]interface{}{
		"ok":                                    true,
		agentcapabilities.FindingIDArgumentName: findingID,
		"is_new":                                isNew,
	}
	return NewJSONResult(result), nil
}

func handlePatrolResolveFinding(_ context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
	creator := e.GetPatrolFindingCreator()
	if creator == nil {
		return NewTextResult("patrol_resolve_finding is only available during a patrol run."), nil
	}
	if checker, ok := creator.(PatrolFindingsChecker); ok && !checker.HasCheckedFindings() {
		return NewErrorResult(fmt.Errorf("call patrol_get_findings before resolving a finding")), nil
	}

	findingID, _ := args[agentcapabilities.FindingIDArgumentName].(string)
	reason, _ := args[agentcapabilities.ReasonArgumentName].(string)

	if findingID == "" {
		return NewErrorResult(fmt.Errorf("missing required field: %s", agentcapabilities.FindingIDArgumentName)), nil
	}
	if reason == "" {
		return NewErrorResult(fmt.Errorf("missing required field: %s", agentcapabilities.ReasonArgumentName)), nil
	}

	if err := creator.ResolveFinding(findingID, reason); err != nil {
		return NewErrorResult(fmt.Errorf("failed to resolve finding: %w", err)), nil
	}

	result := map[string]interface{}{
		"ok":       true,
		"resolved": true,
	}
	return NewJSONResult(result), nil
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
	return NewJSONResult(result), nil
}

func canonicalPatrolResourceType(resourceType string) string {
	return strings.ToLower(strings.TrimSpace(resourceType))
}
