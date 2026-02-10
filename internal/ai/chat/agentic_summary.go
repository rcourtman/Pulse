package chat

import (
	"encoding/json"
	"fmt"
	"strings"
)

// summarizeForNegativeMarker creates a concise summary of a tool result for
// use in negative markers. Tries to extract meaningful context from JSON
// responses rather than blindly truncating.
func summarizeForNegativeMarker(resultText string) string {
	if len(resultText) == 0 {
		return "empty response"
	}

	// Try to parse as JSON and extract key indicators
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(resultText), &obj); err == nil {
		var indicators []string

		// Check for common empty-result patterns
		for _, arrayKey := range []string{"points", "pools", "disks", "alerts", "findings", "jobs", "tasks", "snapshots", "resources", "containers", "vms", "nodes", "updates"} {
			if arr, ok := obj[arrayKey]; ok {
				if slice, ok := arr.([]interface{}); ok {
					indicators = append(indicators, fmt.Sprintf("%s: %d items", arrayKey, len(slice)))
				}
			}
		}

		// Check for total field
		if total, ok := obj["total"]; ok {
			indicators = append(indicators, fmt.Sprintf("total=%v", total))
		}

		// Check for period/resource_id context
		if rid, ok := obj["resource_id"]; ok {
			indicators = append(indicators, fmt.Sprintf("resource=%v", rid))
		}
		if period, ok := obj["period"]; ok {
			indicators = append(indicators, fmt.Sprintf("period=%v", period))
		}

		// Check for error field
		if errVal, ok := obj["error"]; ok {
			indicators = append(indicators, fmt.Sprintf("error=%v", errVal))
		}

		if len(indicators) > 0 {
			result := strings.Join(indicators, ", ")
			if len(result) > 200 {
				result = result[:200]
			}
			return result
		}
	}

	// Try JSON array
	var arr []interface{}
	if err := json.Unmarshal([]byte(resultText), &arr); err == nil {
		return fmt.Sprintf("array with %d items", len(arr))
	}

	// Fall back to truncated text
	summary := resultText
	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}
	return summary
}
