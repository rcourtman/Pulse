package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
					"resource_type": {
						Type:        "string",
						Description: "Optional: filter by resource type (vm, container, node)",
					},
					"limit": {
						Type:        "integer",
						Description: "Max results when returning summary (default 100)",
					},
					"offset": {
						Type:        "integer",
						Description: "Skip N results when returning summary",
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
					"resource_type": {
						Type:        "string",
						Description: "Optional: filter by resource type (vm, container, node)",
					},
					"limit": {
						Type:        "integer",
						Description: "Max results when returning baselines (default 100)",
					},
					"offset": {
						Type:        "integer",
						Description: "Skip N results when returning baselines",
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
						Description: "Filter: critical, warning, or info. Omit for all.",
						Enum:        []string{"critical", "warning", "info"},
					},
					"resource_type": {
						Type:        "string",
						Description: "Optional: filter by resource type (vm, container, node, docker)",
					},
					"resource_id": {
						Type:        "string",
						Description: "Optional: filter by resource ID",
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

	// ========== Resolved Alerts Tool ==========

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_list_resolved_alerts",
			Description: `List recently resolved alerts (alerts that were active but have since cleared).

Returns: JSON with alerts array containing alert details including type, level, resource info, message, start time, and when it was resolved.

Use when: User asks about alerts that cleared, what issues resolved themselves, or wants to see recent alert history.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"type": {
						Type:        "string",
						Description: "Optional: filter by alert type",
					},
					"level": {
						Type:        "string",
						Description: "Optional: filter by level (critical, warning)",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results (default: 50)",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeListResolvedAlerts(ctx, args)
		},
	})
}

func (e *PulseToolExecutor) executeGetMetrics(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	period, _ := args["period"].(string)
	resourceID, _ := args["resource_id"].(string)
	resourceType, _ := args["resource_type"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	if resourceType != "" {
		validTypes := map[string]bool{"vm": true, "container": true, "node": true}
		if !validTypes[resourceType] {
			return NewErrorResult(fmt.Errorf("invalid resource_type: %s. Use vm, container, or node", resourceType)), nil
		}
	}

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
		return NewJSONResult(response), nil
	}

	summary, err := e.metricsHistory.GetAllMetricsSummary(duration)
	if err != nil {
		return NewErrorResult(err), nil
	}

	keys := make([]string, 0, len(summary))
	for id, metric := range summary {
		if resourceType != "" && strings.ToLower(metric.ResourceType) != resourceType {
			continue
		}
		keys = append(keys, id)
	}
	sort.Strings(keys)

	filtered := make(map[string]ResourceMetricsSummary)
	total := 0
	for _, id := range keys {
		if total < offset {
			total++
			continue
		}
		if len(filtered) >= limit {
			total++
			continue
		}
		filtered[id] = summary[id]
		total++
	}

	if filtered == nil {
		filtered = map[string]ResourceMetricsSummary{}
	}

	response.Summary = filtered
	if offset > 0 || total > limit {
		response.Pagination = &PaginationInfo{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		}
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeGetBaselines(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceID, _ := args["resource_id"].(string)
	resourceType, _ := args["resource_type"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	if resourceType != "" {
		validTypes := map[string]bool{"vm": true, "container": true, "node": true}
		if !validTypes[resourceType] {
			return NewErrorResult(fmt.Errorf("invalid resource_type: %s. Use vm, container, or node", resourceType)), nil
		}
	}

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
		return NewJSONResult(response), nil
	}

	baselines := e.baselineProvider.GetAllBaselines()
	keys := make([]string, 0, len(baselines))
	var typeIndex map[string]string
	if resourceType != "" {
		if e.stateProvider == nil {
			return NewErrorResult(fmt.Errorf("state provider not available")), nil
		}
		state := e.stateProvider.GetState()
		typeIndex = make(map[string]string)
		for _, vm := range state.VMs {
			typeIndex[fmt.Sprintf("%d", vm.VMID)] = "vm"
		}
		for _, ct := range state.Containers {
			typeIndex[fmt.Sprintf("%d", ct.VMID)] = "container"
		}
		for _, node := range state.Nodes {
			typeIndex[node.ID] = "node"
		}
	}

	for id := range baselines {
		if resourceType != "" {
			if t, ok := typeIndex[id]; !ok || t != resourceType {
				continue
			}
		}
		keys = append(keys, id)
	}
	sort.Strings(keys)

	filtered := make(map[string]map[string]*MetricBaseline)
	total := 0
	for _, id := range keys {
		if total < offset {
			total++
			continue
		}
		if len(filtered) >= limit {
			total++
			continue
		}
		filtered[id] = baselines[id]
		total++
	}

	if filtered == nil {
		filtered = map[string]map[string]*MetricBaseline{}
	}

	response.Baselines = filtered
	if offset > 0 || total > limit {
		response.Pagination = &PaginationInfo{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		}
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
		validTypes := map[string]bool{"vm": true, "container": true, "node": true, "docker": true}
		if !validTypes[resourceType] {
			return NewErrorResult(fmt.Errorf("invalid resource_type: %s. Use vm, container, node, or docker", resourceType)), nil
		}
	}

	if e.findingsProvider == nil {
		return NewTextResult("Patrol findings not available. AI Patrol may not be running."), nil
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
		case "lxc", "lxc container", "lxc-container", "lxc_container":
			return "container"
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

// ========== Resolved Alerts Tool Implementation ==========

func (e *PulseToolExecutor) executeListResolvedAlerts(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	typeFilter, _ := args["type"].(string)
	levelFilter, _ := args["level"].(string)
	limit := intArg(args, "limit", 50)

	state := e.stateProvider.GetState()

	if len(state.RecentlyResolved) == 0 {
		return NewTextResult("No recently resolved alerts."), nil
	}

	var alerts []ResolvedAlertSummary

	for _, alert := range state.RecentlyResolved {
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
		Total:  len(state.RecentlyResolved),
	}

	return NewJSONResult(response), nil
}
