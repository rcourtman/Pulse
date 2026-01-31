package eval

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// === Execution Assertions ===

// PatrolAssertNoError checks that no execution/network error occurred.
func PatrolAssertNoError() PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		if result.Error == nil {
			return AssertionResult{
				Name:    "no_error",
				Passed:  true,
				Message: "No execution error",
			}
		}
		return AssertionResult{
			Name:    "no_error",
			Passed:  false,
			Message: fmt.Sprintf("Execution error: %v", result.Error),
		}
	}
}

// PatrolAssertCompleted checks that the patrol run completed.
// Uses the Completed field (from status polling) or a "complete" SSE event.
func PatrolAssertCompleted() PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		if result.Completed {
			return AssertionResult{
				Name:    "completed",
				Passed:  true,
				Message: "Patrol completed (confirmed via status API)",
			}
		}
		for _, e := range result.RawEvents {
			if e.Type == "complete" {
				return AssertionResult{
					Name:    "completed",
					Passed:  true,
					Message: "Patrol completed (confirmed via SSE stream)",
				}
			}
		}
		return AssertionResult{
			Name:    "completed",
			Passed:  false,
			Message: "Patrol did not complete",
		}
	}
}

// PatrolAssertDurationUnder checks that the total run time is under the given max.
func PatrolAssertDurationUnder(max time.Duration) PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		if result.Duration <= max {
			return AssertionResult{
				Name:    fmt.Sprintf("duration_under:%v", max),
				Passed:  true,
				Message: fmt.Sprintf("Completed in %v (max: %v)", result.Duration.Round(time.Second), max),
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("duration_under:%v", max),
			Passed:  false,
			Message: fmt.Sprintf("Took %v which exceeds max of %v", result.Duration.Round(time.Second), max),
		}
	}
}

// === Tool Usage Assertions ===

// PatrolAssertToolUsed checks that a tool was called at least once.
func PatrolAssertToolUsed(toolName string) PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		for _, tc := range result.ToolCalls {
			if tc.Name == toolName {
				return AssertionResult{
					Name:    fmt.Sprintf("tool_used:%s", toolName),
					Passed:  true,
					Message: fmt.Sprintf("Tool '%s' was called", toolName),
				}
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("tool_used:%s", toolName),
			Passed:  false,
			Message: fmt.Sprintf("Tool '%s' was NOT called. Tools used: %v", toolName, patrolGetToolNames(result.ToolCalls)),
		}
	}
}

// PatrolAssertToolUsedAny checks that at least one of the given tools was called.
func PatrolAssertToolUsedAny(toolNames ...string) PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		for _, tc := range result.ToolCalls {
			for _, name := range toolNames {
				if tc.Name == name {
					return AssertionResult{
						Name:    "tool_used_any",
						Passed:  true,
						Message: fmt.Sprintf("Tool '%s' was called", name),
					}
				}
			}
		}
		return AssertionResult{
			Name:    "tool_used_any",
			Passed:  false,
			Message: fmt.Sprintf("None of %v were called. Tools used: %v", toolNames, patrolGetToolNames(result.ToolCalls)),
		}
	}
}

// PatrolAssertInvestigatedBeforeReporting checks that infrastructure tools were used
// before reporting findings. If no findings were reported, this passes.
func PatrolAssertInvestigatedBeforeReporting(toolNames ...string) PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		reported := false
		for _, tc := range result.ToolCalls {
			if tc.Name == "patrol_report_finding" {
				reported = true
				break
			}
		}
		if !reported {
			return AssertionResult{
				Name:    "investigated_before_reporting",
				Passed:  true,
				Message: "No findings reported",
			}
		}

		for _, tc := range result.ToolCalls {
			for _, name := range toolNames {
				if tc.Name == name {
					return AssertionResult{
						Name:    "investigated_before_reporting",
						Passed:  true,
						Message: fmt.Sprintf("Tool '%s' was called before reporting", name),
					}
				}
			}
		}
		return AssertionResult{
			Name:    "investigated_before_reporting",
			Passed:  false,
			Message: fmt.Sprintf("No investigation tools called before reporting. Tools used: %v", patrolGetToolNames(result.ToolCalls)),
		}
	}
}

// PatrolAssertMinToolCalls checks that the total tool call count is at least min.
func PatrolAssertMinToolCalls(min int) PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		if len(result.ToolCalls) >= min {
			return AssertionResult{
				Name:    fmt.Sprintf("min_tool_calls:%d", min),
				Passed:  true,
				Message: fmt.Sprintf("%d tool calls made (min: %d)", len(result.ToolCalls), min),
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("min_tool_calls:%d", min),
			Passed:  false,
			Message: fmt.Sprintf("Only %d tool calls made (expected at least %d)", len(result.ToolCalls), min),
		}
	}
}

// PatrolAssertNoToolErrors checks that all tool calls succeeded.
func PatrolAssertNoToolErrors() PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		var failures []string
		for _, tc := range result.ToolCalls {
			if !tc.Success {
				failures = append(failures, fmt.Sprintf("%s: %s", tc.Name, truncate(tc.Output, 100)))
			}
		}
		if len(failures) == 0 {
			return AssertionResult{
				Name:    "no_tool_errors",
				Passed:  true,
				Message: "All tool calls succeeded",
			}
		}
		return AssertionResult{
			Name:    "no_tool_errors",
			Passed:  false,
			Message: fmt.Sprintf("Tool failures: %v", failures),
		}
	}
}

// PatrolAssertToolSuccessRate checks that at least the given fraction of tool calls succeeded.
// For example, 0.8 means 80% of tool calls must succeed.
func PatrolAssertToolSuccessRate(minRate float64) PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		if len(result.ToolCalls) == 0 {
			return AssertionResult{
				Name:    fmt.Sprintf("tool_success_rate:%.0f%%", minRate*100),
				Passed:  true,
				Message: "No tool calls to evaluate",
			}
		}
		succeeded := 0
		for _, tc := range result.ToolCalls {
			if tc.Success {
				succeeded++
			}
		}
		rate := float64(succeeded) / float64(len(result.ToolCalls))
		if rate >= minRate {
			return AssertionResult{
				Name:    fmt.Sprintf("tool_success_rate:%.0f%%", minRate*100),
				Passed:  true,
				Message: fmt.Sprintf("%d/%d tool calls succeeded (%.0f%%, min: %.0f%%)", succeeded, len(result.ToolCalls), rate*100, minRate*100),
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("tool_success_rate:%.0f%%", minRate*100),
			Passed:  false,
			Message: fmt.Sprintf("%d/%d tool calls succeeded (%.0f%%, min: %.0f%%)", succeeded, len(result.ToolCalls), rate*100, minRate*100),
		}
	}
}

// PatrolAssertToolSequence checks that tools were called in the given order (non-contiguous).
func PatrolAssertToolSequence(sequence []string) PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		if len(sequence) == 0 {
			return AssertionResult{
				Name:    "tool_sequence",
				Passed:  true,
				Message: "No sequence required",
			}
		}

		seqIdx := 0
		for _, tc := range result.ToolCalls {
			if tc.Name == sequence[seqIdx] {
				seqIdx++
				if seqIdx == len(sequence) {
					return AssertionResult{
						Name:    "tool_sequence",
						Passed:  true,
						Message: fmt.Sprintf("Tool sequence matched: %v", sequence),
					}
				}
			}
		}

		return AssertionResult{
			Name:    "tool_sequence",
			Passed:  false,
			Message: fmt.Sprintf("Tool sequence not found. Expected: %v, got: %v", sequence, patrolGetToolNames(result.ToolCalls)),
		}
	}
}

// PatrolAssertToolInputContains checks that a tool's input contains a substring.
func PatrolAssertToolInputContains(toolName, substring string) PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		for _, tc := range result.ToolCalls {
			if tc.Name != toolName {
				continue
			}
			if strings.Contains(strings.ToLower(tc.Input), strings.ToLower(substring)) {
				return AssertionResult{
					Name:    fmt.Sprintf("tool_input:%s_contains:%s", toolName, truncate(substring, 20)),
					Passed:  true,
					Message: fmt.Sprintf("Tool '%s' input contains '%s'", toolName, substring),
				}
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("tool_input:%s_contains:%s", toolName, truncate(substring, 20)),
			Passed:  false,
			Message: fmt.Sprintf("No '%s' call input contains '%s'", toolName, substring),
		}
	}
}

// === Finding Assertions (from REST API results) ===

// PatrolAssertHasFindings checks that at least one finding exists.
func PatrolAssertHasFindings() PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		if len(result.Findings) > 0 {
			return AssertionResult{
				Name:    "has_findings",
				Passed:  true,
				Message: fmt.Sprintf("%d finding(s) present", len(result.Findings)),
			}
		}
		return AssertionResult{
			Name:    "has_findings",
			Passed:  false,
			Message: "No findings were produced",
		}
	}
}

// PatrolAssertFindingCount checks that the finding count is within [min, max].
func PatrolAssertFindingCount(minCount, maxCount int) PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		count := len(result.Findings)
		if count >= minCount && count <= maxCount {
			return AssertionResult{
				Name:    fmt.Sprintf("finding_count:%d-%d", minCount, maxCount),
				Passed:  true,
				Message: fmt.Sprintf("%d findings (range: %d-%d)", count, minCount, maxCount),
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("finding_count:%d-%d", minCount, maxCount),
			Passed:  false,
			Message: fmt.Sprintf("%d findings outside range %d-%d", count, minCount, maxCount),
		}
	}
}

// PatrolAssertAllFindingsValid checks that every finding has non-empty required fields.
func PatrolAssertAllFindingsValid() PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		if len(result.Findings) == 0 {
			return AssertionResult{
				Name:    "all_findings_valid",
				Passed:  true,
				Message: "No findings to validate",
			}
		}

		var issues []string
		for i, f := range result.Findings {
			if f.Key == "" {
				issues = append(issues, fmt.Sprintf("finding[%d]: missing key", i))
			}
			if f.Severity == "" {
				issues = append(issues, fmt.Sprintf("finding[%d]: missing severity", i))
			}
			if f.Title == "" {
				issues = append(issues, fmt.Sprintf("finding[%d]: missing title", i))
			}
			if f.Description == "" {
				issues = append(issues, fmt.Sprintf("finding[%d]: missing description", i))
			}
			if f.ResourceType == "" {
				issues = append(issues, fmt.Sprintf("finding[%d]: missing resource_type", i))
			}
		}

		if len(issues) == 0 {
			return AssertionResult{
				Name:    "all_findings_valid",
				Passed:  true,
				Message: fmt.Sprintf("All %d findings have required fields", len(result.Findings)),
			}
		}
		return AssertionResult{
			Name:    "all_findings_valid",
			Passed:  false,
			Message: fmt.Sprintf("Invalid findings: %s", strings.Join(issues, "; ")),
		}
	}
}

// PatrolAssertFindingSeveritiesValid checks that all findings use valid severity values.
func PatrolAssertFindingSeveritiesValid() PatrolAssertion {
	validSeverities := map[string]bool{
		"critical": true,
		"warning":  true,
		"watch":    true,
		"info":     true,
	}
	return func(result *PatrolRunResult) AssertionResult {
		var invalid []string
		for _, f := range result.Findings {
			if !validSeverities[strings.ToLower(f.Severity)] {
				invalid = append(invalid, fmt.Sprintf("%s (severity=%q)", f.Key, f.Severity))
			}
		}
		if len(invalid) == 0 {
			return AssertionResult{
				Name:    "finding_severities_valid",
				Passed:  true,
				Message: "All finding severities are valid",
			}
		}
		return AssertionResult{
			Name:    "finding_severities_valid",
			Passed:  false,
			Message: fmt.Sprintf("Invalid severities: %v", invalid),
		}
	}
}

// PatrolAssertFindingCategoriesValid checks that all findings use valid category values.
func PatrolAssertFindingCategoriesValid() PatrolAssertion {
	validCategories := map[string]bool{
		"performance": true,
		"capacity":    true,
		"reliability": true,
		"backup":      true,
		"security":    true,
		"general":     true,
	}
	return func(result *PatrolRunResult) AssertionResult {
		var invalid []string
		for _, f := range result.Findings {
			if f.Category != "" && !validCategories[strings.ToLower(f.Category)] {
				invalid = append(invalid, fmt.Sprintf("%s (category=%q)", f.Key, f.Category))
			}
		}
		if len(invalid) == 0 {
			return AssertionResult{
				Name:    "finding_categories_valid",
				Passed:  true,
				Message: "All finding categories are valid",
			}
		}
		return AssertionResult{
			Name:    "finding_categories_valid",
			Passed:  false,
			Message: fmt.Sprintf("Invalid categories: %v", invalid),
		}
	}
}

// PatrolAssertSignalCoverage checks that signal coverage meets a minimum rate.
// If tool calls aren't captured or no signals are detected, this passes with a note.
func PatrolAssertSignalCoverage(minRate float64) PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		if result == nil {
			return AssertionResult{
				Name:    "signal_coverage",
				Passed:  false,
				Message: "No result available",
			}
		}

		if result.Quality == nil {
			result.Quality = EvaluatePatrolQuality(result)
		}

		q := result.Quality
		if q == nil || !q.CoverageKnown {
			return AssertionResult{
				Name:    "signal_coverage",
				Passed:  true,
				Message: "Signal coverage not measured (no tool calls captured)",
			}
		}
		if q.SignalsTotal == 0 {
			return AssertionResult{
				Name:    "signal_coverage",
				Passed:  true,
				Message: "No deterministic signals detected",
			}
		}
		if q.SignalCoverage >= minRate {
			return AssertionResult{
				Name:   "signal_coverage",
				Passed: true,
				Message: fmt.Sprintf("Signal coverage %.0f%% (%d/%d), min %.0f%%",
					q.SignalCoverage*100, q.SignalsMatched, q.SignalsTotal, minRate*100),
			}
		}
		return AssertionResult{
			Name:   "signal_coverage",
			Passed: false,
			Message: fmt.Sprintf("Signal coverage %.0f%% (%d/%d) below min %.0f%%",
				q.SignalCoverage*100, q.SignalsMatched, q.SignalsTotal, minRate*100),
		}
	}
}

// PatrolAssertFindingWithKey checks that a finding with the given key exists.
func PatrolAssertFindingWithKey(key string) PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		for _, f := range result.Findings {
			if strings.EqualFold(f.Key, key) {
				return AssertionResult{
					Name:    fmt.Sprintf("finding_with_key:%s", key),
					Passed:  true,
					Message: fmt.Sprintf("Finding with key '%s' exists", key),
				}
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("finding_with_key:%s", key),
			Passed:  false,
			Message: fmt.Sprintf("No finding with key '%s'", key),
		}
	}
}

// PatrolAssertNoFindingWithKey checks that no finding with the given key exists.
func PatrolAssertNoFindingWithKey(key string) PatrolAssertion {
	return func(result *PatrolRunResult) AssertionResult {
		for _, f := range result.Findings {
			if strings.EqualFold(f.Key, key) {
				return AssertionResult{
					Name:    fmt.Sprintf("no_finding_with_key:%s", key),
					Passed:  false,
					Message: fmt.Sprintf("Finding with key '%s' exists but should not", key),
				}
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("no_finding_with_key:%s", key),
			Passed:  true,
			Message: fmt.Sprintf("No finding with key '%s' (as expected)", key),
		}
	}
}

// === Finding Tool Call Assertions ===

// PatrolAssertReportFindingFieldsPresent checks that every patrol_report_finding
// tool call includes all required fields in input.
func PatrolAssertReportFindingFieldsPresent() PatrolAssertion {
	requiredFields := []string{"key", "severity", "title", "description", "resource_type"}
	return func(result *PatrolRunResult) AssertionResult {
		reportCalls := 0
		var issues []string
		missingInputs := 0

		for _, tc := range result.ToolCalls {
			if tc.Name != "patrol_report_finding" {
				continue
			}
			reportCalls++

			input := strings.TrimSpace(tc.Input)
			if input == "" || input == "{}" {
				if !tc.Success {
					issues = append(issues, fmt.Sprintf("call %d missing input", reportCalls))
				} else {
					missingInputs++
				}
				continue
			}

			// Try to parse input as JSON to check fields
			var inputMap map[string]interface{}
			if err := json.Unmarshal([]byte(input), &inputMap); err != nil {
				// Fall back to substring check
				for _, field := range requiredFields {
					if !strings.Contains(input, field) {
						issues = append(issues, fmt.Sprintf("call %d missing '%s'", reportCalls, field))
					}
				}
				continue
			}

			for _, field := range requiredFields {
				val, ok := inputMap[field]
				if !ok {
					issues = append(issues, fmt.Sprintf("call %d missing '%s'", reportCalls, field))
				} else if str, isStr := val.(string); isStr && str == "" {
					issues = append(issues, fmt.Sprintf("call %d has empty '%s'", reportCalls, field))
				}
			}
		}

		if reportCalls == 0 {
			return AssertionResult{
				Name:    "report_finding_fields_present",
				Passed:  true,
				Message: "No patrol_report_finding calls to validate",
			}
		}

		if len(issues) == 0 {
			if missingInputs > 0 {
				return AssertionResult{
					Name:    "report_finding_fields_present",
					Passed:  true,
					Message: fmt.Sprintf("Required fields assumed present for %d call(s) with missing input", missingInputs),
				}
			}
			return AssertionResult{
				Name:    "report_finding_fields_present",
				Passed:  true,
				Message: fmt.Sprintf("All %d patrol_report_finding calls have required fields", reportCalls),
			}
		}
		return AssertionResult{
			Name:    "report_finding_fields_present",
			Passed:  false,
			Message: fmt.Sprintf("Missing fields: %s", strings.Join(issues, "; ")),
		}
	}
}

// === Helpers ===

func patrolGetToolNames(toolCalls []ToolCallEvent) []string {
	names := make([]string, len(toolCalls))
	for i, tc := range toolCalls {
		names[i] = tc.Name
	}
	return names
}
