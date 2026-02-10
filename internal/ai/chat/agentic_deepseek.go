package chat

import "strings"

// cleanDeepSeekArtifacts removes DeepSeek's internal tool call format leakage.
// When DeepSeek doesn't properly use the function calling API, it may output
// its internal markup like <｜DSML｜function_calls>, <｜DSML｜invoke>, etc.
// These patterns can appear with Unicode pipe (｜) or ASCII pipe (|).
// This is applied to chat responses to prevent the artifacts from being shown to users.
func cleanDeepSeekArtifacts(content string) string {
	if content == "" {
		return content
	}

	// DeepSeek internal function call format markers
	markers := []string{
		"<｜DSML｜",  // Unicode pipe variant (opening)
		"</｜DSML｜", // Unicode pipe variant (closing)
		"<|DSML|",  // ASCII pipe variant (opening)
		"</|DSML|", // ASCII pipe variant (closing)
		"<｜/DSML｜", // Alternative Unicode closing
		"<|/DSML|", // Alternative ASCII closing
	}

	for _, marker := range markers {
		if idx := strings.Index(content, marker); idx >= 0 {
			// DeepSeek function call blocks typically appear at the end of responses
			// Remove everything from the marker to the end
			content = strings.TrimSpace(content[:idx])
		}
	}

	return content
}

// containsDeepSeekMarker checks if the content contains any DeepSeek internal function call markers.
// This is used during streaming to detect when we should stop forwarding content.
func containsDeepSeekMarker(content string) bool {
	markers := []string{
		"<｜DSML｜", // Unicode pipe variant
		"<|DSML|", // ASCII pipe variant
	}
	for _, marker := range markers {
		if strings.Contains(content, marker) {
			return true
		}
	}
	return false
}
