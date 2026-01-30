package eval

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
)

// PatrolQualityReport summarizes signal coverage quality for a patrol run.
type PatrolQualityReport struct {
	SignalCoverage   float64              `json:"signal_coverage"`
	SignalsTotal     int                  `json:"signals_total"`
	SignalsMatched   int                  `json:"signals_matched"`
	SignalsUnmatched int                  `json:"signals_unmatched"`
	CoverageKnown    bool                 `json:"coverage_known"`
	ToolCallsSeen    int                  `json:"tool_calls_seen"`
	Signals          []PatrolSignalResult `json:"signals,omitempty"`
	Notes            []string             `json:"notes,omitempty"`
}

// PatrolSignalResult captures a single signal match outcome.
type PatrolSignalResult struct {
	SignalType        string `json:"signal_type"`
	ResourceID        string `json:"resource_id"`
	ResourceName      string `json:"resource_name"`
	ResourceType      string `json:"resource_type"`
	Category          string `json:"category"`
	SuggestedSeverity string `json:"suggested_severity"`
	Summary           string `json:"summary"`
	Matched           bool   `json:"matched"`
	MatchedFindingID  string `json:"matched_finding_id,omitempty"`
	MatchedFindingKey string `json:"matched_finding_key,omitempty"`
	MatchedFinding    string `json:"matched_finding_title,omitempty"`
	Reason            string `json:"reason,omitempty"`
}

// EvaluatePatrolQuality computes signal coverage based on deterministic tool signals.
// This is best-effort: if tool calls aren't captured, coverage is unknown.
func EvaluatePatrolQuality(result *PatrolRunResult) *PatrolQualityReport {
	if result == nil {
		return nil
	}

	report := &PatrolQualityReport{
		ToolCallsSeen: len(result.ToolCalls),
	}

	if len(result.ToolCalls) == 0 {
		report.CoverageKnown = false
		report.Notes = append(report.Notes, "no tool calls captured; signal coverage unknown")
		return report
	}

	toolCalls := make([]ai.ToolCallRecord, 0, len(result.ToolCalls))
	for _, tc := range result.ToolCalls {
		toolCalls = append(toolCalls, ai.ToolCallRecord{
			ID:       tc.ID,
			ToolName: tc.Name,
			Input:    tc.Input,
			Output:   tc.Output,
			Success:  tc.Success,
		})
	}

	signals := ai.DetectSignals(toolCalls, ai.DefaultSignalThresholds())
	report.SignalsTotal = len(signals)
	report.CoverageKnown = true

	for _, signal := range signals {
		match, reason, f := matchSignalToFinding(signal, result.Findings)
		if match {
			report.SignalsMatched++
		} else {
			report.SignalsUnmatched++
		}
		entry := PatrolSignalResult{
			SignalType:        string(signal.SignalType),
			ResourceID:        signal.ResourceID,
			ResourceName:      signal.ResourceName,
			ResourceType:      signal.ResourceType,
			Category:          signal.Category,
			SuggestedSeverity: signal.SuggestedSeverity,
			Summary:           signal.Summary,
			Matched:           match,
			Reason:            reason,
		}
		if f != nil {
			entry.MatchedFindingID = f.ID
			entry.MatchedFindingKey = f.Key
			entry.MatchedFinding = f.Title
		}
		report.Signals = append(report.Signals, entry)
	}

	if report.SignalsTotal > 0 {
		report.SignalCoverage = float64(report.SignalsMatched) / float64(report.SignalsTotal)
	}

	return report
}

func matchSignalToFinding(signal ai.DetectedSignal, findings []PatrolFinding) (bool, string, *PatrolFinding) {
	if len(findings) == 0 {
		return false, "no findings returned", nil
	}

	signalSeverity := severityRank(signal.SuggestedSeverity)
	for i := range findings {
		f := &findings[i]
		if !resourceMatches(signal, f) {
			continue
		}
		if signal.Category != "" && !strings.EqualFold(f.Category, signal.Category) {
			continue
		}
		if signalSeverity > 0 && severityRank(f.Severity) < signalSeverity {
			continue
		}
		return true, "", f
	}

	return false, fmt.Sprintf("no matching finding for resource/category/severity"), nil
}

func resourceMatches(signal ai.DetectedSignal, finding *PatrolFinding) bool {
	if finding == nil {
		return false
	}

	if signal.ResourceID != "" && (strings.EqualFold(signal.ResourceID, finding.ResourceID) ||
		strings.EqualFold(signal.ResourceID, finding.ResourceName)) {
		return true
	}
	if signal.ResourceName != "" && (strings.EqualFold(signal.ResourceName, finding.ResourceID) ||
		strings.EqualFold(signal.ResourceName, finding.ResourceName)) {
		return true
	}

	return false
}

func severityRank(sev string) int {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "critical":
		return 3
	case "warning":
		return 2
	case "watch":
		return 1
	case "info":
		return 0
	default:
		return 0
	}
}
