package mcp

import (
	"context"
	"fmt"
	"time"
)

// registerPatrolTools registers patrol context tools (metrics, baselines, patterns, alerts, findings)
func (e *PulseToolExecutor) registerPatrolTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_get_metrics",
			Description: "Get historical metrics (CPU, memory, disk) for resources over 24 hours or 7 days. Use this to understand trends and detect anomalies.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"period": {
						Type:        "string",
						Description: "Time period: '24h' for last 24 hours, '7d' for last 7 days",
						Enum:        []string{"24h", "7d"},
					},
					"resource_id": {
						Type:        "string",
						Description: "Optional: specific resource ID. If omitted, returns summary for all resources.",
					},
				},
				Required: []string{"period"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetMetrics(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_get_baselines",
			Description: "Get learned baselines for resources. Baselines represent 'normal' behavior and help detect anomalies.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_id": {
						Type:        "string",
						Description: "Optional: specific resource ID. If omitted, returns all baselines.",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetBaselines(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_get_patterns",
			Description: "Get detected operational patterns and predictions. Includes recurring spikes, growth trends, and predicted issues.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]PropertySchema{},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetPatterns(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_list_alerts",
			Description: `List active threshold alerts (CPU > 80%, disk full, etc).

Returns: JSON array of alerts with resource, type, severity, value, threshold.

Use when: User asks about alerts, warnings, or "what's wrong" with infrastructure.

Do NOT use for: Checking if something is running (use pulse_get_topology).`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"severity": {
						Type:        "string",
						Description: "Filter: 'critical', 'warning', or 'info'. Omit for all.",
						Enum:        []string{"critical", "warning", "info"},
					},
					"limit": {
						Type:        "integer",
						Description: "Max results (default 100)",
					},
					"offset": {
						Type:        "integer",
						Description: "Skip N results for pagination",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeListAlerts(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_list_findings",
			Description: `List AI patrol findings - issues detected by automated analysis.

Returns: JSON with active findings (current issues) and counts.

Use when: User asks about findings, issues, or "what did the AI find".

Do NOT use for: Checking if something is running (use pulse_get_topology), or threshold alerts (use pulse_list_alerts).`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"include_dismissed": {
						Type:        "boolean",
						Description: "Include previously dismissed findings",
					},
					"severity": {
						Type:        "string",
						Description: "Filter: 'critical', 'warning', or 'info'. Omit for all.",
						Enum:        []string{"critical", "warning", "info"},
					},
					"limit": {
						Type:        "integer",
						Description: "Max results (default 100)",
					},
					"offset": {
						Type:        "integer",
						Description: "Skip N results for pagination",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeListFindings(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_resolve_finding",
			Description: "Mark an AI patrol finding as resolved after fixing the issue.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"finding_id": {
						Type:        "string",
						Description: "The finding ID to resolve",
					},
					"resolution_note": {
						Type:        "string",
						Description: "Brief description of how the issue was resolved",
					},
				},
				Required: []string{"finding_id", "resolution_note"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeResolveFinding(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_dismiss_finding",
			Description: "Dismiss an AI patrol finding as not an issue or expected behavior.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"finding_id": {
						Type:        "string",
						Description: "The finding ID to dismiss",
					},
					"reason": {
						Type:        "string",
						Description: "Why the finding is being dismissed",
						Enum:        []string{"not_an_issue", "expected_behavior", "will_fix_later"},
					},
					"note": {
						Type:        "string",
						Description: "Explanation of why this is being dismissed",
					},
				},
				Required: []string{"finding_id", "reason", "note"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeDismissFinding(ctx, args)
		},
	})
}

func (e *PulseToolExecutor) executeGetMetrics(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	period, _ := args["period"].(string)
	resourceID, _ := args["resource_id"].(string)

	if e.metricsHistory == nil {
		return NewTextResult("Metrics history not available. The system may still be collecting data."), nil
	}

	var duration time.Duration
	switch period {
	case "24h":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	default:
		duration = 24 * time.Hour
		period = "24h"
	}

	response := MetricsResponse{
		Period: period,
	}

	if resourceID != "" {
		response.ResourceID = resourceID
		metrics, err := e.metricsHistory.GetResourceMetrics(resourceID, duration)
		if err != nil {
			return NewErrorResult(err), nil
		}
		response.Points = metrics
	} else {
		summary, err := e.metricsHistory.GetAllMetricsSummary(duration)
		if err != nil {
			return NewErrorResult(err), nil
		}
		response.Summary = summary
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeGetBaselines(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceID, _ := args["resource_id"].(string)

	if e.baselineProvider == nil {
		return NewTextResult("Baseline data not available. The system needs time to learn normal behavior patterns."), nil
	}

	response := BaselinesResponse{
		Baselines: make(map[string]map[string]*MetricBaseline),
	}

	if resourceID != "" {
		response.ResourceID = resourceID
		cpuBaseline := e.baselineProvider.GetBaseline(resourceID, "cpu")
		memBaseline := e.baselineProvider.GetBaseline(resourceID, "memory")

		if cpuBaseline != nil || memBaseline != nil {
			response.Baselines[resourceID] = make(map[string]*MetricBaseline)
			if cpuBaseline != nil {
				response.Baselines[resourceID]["cpu"] = cpuBaseline
			}
			if memBaseline != nil {
				response.Baselines[resourceID]["memory"] = memBaseline
			}
		}
	} else {
		response.Baselines = e.baselineProvider.GetAllBaselines()
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeGetPatterns(_ context.Context, _ map[string]interface{}) (CallToolResult, error) {
	if e.patternProvider == nil {
		return NewTextResult("Pattern detection not available. The system needs more historical data."), nil
	}

	response := PatternsResponse{
		Patterns:    e.patternProvider.GetPatterns(),
		Predictions: e.patternProvider.GetPredictions(),
	}

	// Ensure non-nil slices for clean JSON
	if response.Patterns == nil {
		response.Patterns = []Pattern{}
	}
	if response.Predictions == nil {
		response.Predictions = []Prediction{}
	}

	return NewJSONResult(response), nil
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
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)

	if e.findingsProvider == nil {
		return NewTextResult("Patrol findings not available. AI Patrol may not be running."), nil
	}

	allActive := e.findingsProvider.GetActiveFindings()
	var allDismissed []Finding
	if includeDismissed {
		allDismissed = e.findingsProvider.GetDismissedFindings()
	}

	// Filter active
	var active []Finding
	for i, f := range allActive {
		if i < offset {
			continue
		}
		if len(active) >= limit {
			break
		}
		if severityFilter != "" && f.Severity != severityFilter {
			continue
		}
		active = append(active, f)
	}

	// Filter dismissed
	var dismissed []Finding
	if includeDismissed {
		for i, f := range allDismissed {
			if i < offset {
				continue
			}
			if len(dismissed) >= limit {
				break
			}
			if severityFilter != "" && f.Severity != severityFilter {
				continue
			}
			dismissed = append(dismissed, f)
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
			Active:    len(allActive),
			Dismissed: len(allDismissed),
		},
	}

	if offset > 0 || len(allActive) > limit || len(allDismissed) > limit {
		response.Pagination = &PaginationInfo{
			Total:  len(allActive) + len(allDismissed),
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
		"success":    true,
		"finding_id": findingID,
		"action":     "dismissed",
		"reason":     reason,
		"note":       note,
	}), nil
}
