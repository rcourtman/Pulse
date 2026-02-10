package chat

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
)

// requiresToolUse determines if the user's message requires live data or an action.
// Returns true for messages that need infrastructure access (check status, restart, etc.)
// Returns false for conceptual questions (What is TCP?, How does Docker work?)
func requiresToolUse(messages []providers.Message) bool {
	// Find the last user message
	var lastUserContent string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].ToolResult == nil {
			lastUserContent = strings.ToLower(messages[i].Content)
			break
		}
	}

	if lastUserContent == "" {
		return false
	}

	// First, check for explicit conceptual question patterns
	// These should NOT require tools even if they mention infrastructure terms
	conceptualPatterns := []string{
		"what is ", "what's the difference", "what are the",
		"explain ", "how does ", "how do i ", "how to ",
		"why do ", "why does ", "why is it ",
		"tell me about ", "describe ",
		"can you explain", "help me understand",
		"difference between", "best way to", "best practice",
		"is it hard", "is it difficult", "is it easy",
		"should i ", "would you recommend", "what do you think",
	}

	for _, pattern := range conceptualPatterns {
		if strings.Contains(lastUserContent, pattern) {
			// Exception: questions about MY specific infrastructure state are action queries
			// e.g., "what is the status of my server" or "what is my CPU usage"
			hasMyInfra := strings.Contains(lastUserContent, "my ") ||
				strings.Contains(lastUserContent, "on my") ||
				strings.Contains(lastUserContent, "@")
			hasStateQuery := strings.Contains(lastUserContent, "status") ||
				strings.Contains(lastUserContent, "doing") ||
				strings.Contains(lastUserContent, "running") ||
				strings.Contains(lastUserContent, "using") ||
				strings.Contains(lastUserContent, "usage")

			if hasMyInfra && hasStateQuery {
				return true // Explicit state query about user's infrastructure
			}

			// Exception: explicit resource references should trigger tools even in "tell me about" queries.
			resourceNouns := []string{
				"container", "vm", "lxc", "node", "pod", "deployment", "service", "host", "cluster",
			}
			hasResourceNoun := false
			for _, noun := range resourceNouns {
				if strings.Contains(lastUserContent, noun) {
					hasResourceNoun = true
					break
				}
			}
			explicitIndicator := strings.Contains(lastUserContent, "@") ||
				strings.Contains(lastUserContent, "\"") ||
				strings.Contains(lastUserContent, "-") ||
				strings.Contains(lastUserContent, "_") ||
				strings.Contains(lastUserContent, "/")

			if hasResourceNoun && explicitIndicator {
				return true // Treat as action: specific resource is referenced
			}

			return false
		}
	}

	// Pattern 1: @mentions indicate infrastructure references
	if strings.Contains(lastUserContent, "@") {
		return true
	}

	// Pattern 2: Action verbs that require live data
	// These are more specific to avoid matching conceptual discussions
	actionPatterns := []string{
		// Direct action commands
		"restart ", "start ", "stop ", "reboot ", "shutdown ",
		"kill ", "terminate ",
		// Status checks (specific phrasing)
		"check ", "check the", "status of", "is it running", "is it up", "is it down",
		"is running", "is stopped", "is down",
		// "is X running?" pattern
		" running?", " up?", " down?", " stopped?",
		// Live data queries
		"show me the", "list my", "list the", "list all",
		"what's the cpu", "what's the memory", "what's the disk",
		"cpu usage", "memory usage", "disk usage", "storage usage",
		"how much memory", "how much cpu", "how much disk",
		// Logs and debugging
		"show logs", "show the logs", "check logs", "view logs",
		"why is my", "why did my", "troubleshoot my", "debug my", "diagnose my",
		// Discovery of MY resources
		"where is my", "which of my", "find my",
		// Questions about "my" specific infrastructure
		"my server", "my container", "my vm", "my host", "my infrastructure",
		"my node", "my cluster", "my proxmox", "my docker",
		// Inventory-style queries
		"what nodes do i have", "what proxmox nodes",
		"what containers do i have", "what vms do i have",
		"what is running on", "what's running on",
	}

	for _, pattern := range actionPatterns {
		if strings.Contains(lastUserContent, pattern) {
			return true
		}
	}

	// Logs or journal queries should always hit tools.
	if strings.Contains(lastUserContent, "logs") ||
		strings.Contains(lastUserContent, " log") ||
		strings.Contains(lastUserContent, "journal") ||
		strings.Contains(lastUserContent, "journald") {
		return true
	}

	// Default: assume conceptual question, don't force tools
	return false
}

// getPreferredTool returns a tool name if the user explicitly requested one.
// Only returns tools that are available for this request.
func getPreferredTool(messages []providers.Message, tools []providers.Tool) string {
	var lastUserContent string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].ToolResult == nil {
			lastUserContent = strings.ToLower(messages[i].Content)
			break
		}
	}
	if lastUserContent == "" {
		return ""
	}

	toolSet := make(map[string]bool, len(tools))
	for _, tool := range tools {
		if tool.Name != "" {
			toolSet[tool.Name] = true
		}
	}

	// Explicit tool mentions
	explicitTools := []string{
		"pulse_read",
		"pulse_control",
		"pulse_query",
		"pulse_discovery",
		"pulse_docker",
		"pulse_kubernetes",
		"pulse_metrics",
		"pulse_storage",
	}
	for _, tool := range explicitTools {
		if strings.Contains(lastUserContent, tool) && toolSet[tool] {
			return tool
		}
	}

	// Natural language aliases
	if (strings.Contains(lastUserContent, "read-only tool") || strings.Contains(lastUserContent, "read only tool")) && toolSet["pulse_read"] {
		return "pulse_read"
	}
	if strings.Contains(lastUserContent, "control tool") && toolSet["pulse_control"] {
		return "pulse_control"
	}
	if strings.Contains(lastUserContent, "query tool") && toolSet["pulse_query"] {
		return "pulse_query"
	}

	// Context carryover: if we injected an explicit target and logs are requested, force pulse_read.
	if strings.Contains(lastUserContent, "explicit target") &&
		(strings.Contains(lastUserContent, "log") || strings.Contains(lastUserContent, "journal")) &&
		toolSet["pulse_read"] {
		return "pulse_read"
	}

	return ""
}

// isSingleToolRequest detects user instructions to use exactly one tool call.
func isSingleToolRequest(messages []providers.Message) bool {
	var lastUserContent string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].ToolResult == nil {
			lastUserContent = strings.ToLower(messages[i].Content)
			break
		}
	}
	if lastUserContent == "" {
		return false
	}

	patterns := []string{
		"only that tool once",
		"only this tool once",
		"call only that tool once",
		"call only this tool once",
		"call only that tool",
		"call only this tool",
		"call only one tool",
		"only one tool",
		"single tool",
		"use only that tool",
		"use only this tool",
		"do not call any other tools",
		"don't call any other tools",
		"no other tools",
	}

	for _, pattern := range patterns {
		if strings.Contains(lastUserContent, pattern) {
			return true
		}
	}

	return false
}

// hasWriteIntent checks if the user's message contains explicit write/control intent.
// Returns true if the user is asking for an action (stop, start, restart, run command, etc.).
// Returns false if the intent is read-only (status check, logs, monitoring).
// This is used to structurally block control tools on read-only requests.
func hasWriteIntent(messages []providers.Message) bool {
	var lastUserContent string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].ToolResult == nil {
			lastUserContent = strings.ToLower(messages[i].Content)
			break
		}
	}
	if lastUserContent == "" {
		return false
	}

	// Explicit write/control action verbs
	writePatterns := []string{
		"stop ", "start ", "restart ", "reboot ", "shutdown ", "shut down",
		"kill ", "terminate ",
		"turn off", "turn on", "bring up", "bring down", "bring back",
		"run command", "run the command", "execute ",
		"using the control tool", "use pulse_control",
		"using pulse_control",
		// File editing
		"edit ", "modify ", "change ", "update ", "write ",
		"use pulse_file_edit",
	}

	for _, pattern := range writePatterns {
		if strings.Contains(lastUserContent, pattern) {
			return true
		}
	}

	return false
}

// isWriteTool returns true if the tool name is a write/control tool that modifies infrastructure.
func isWriteTool(name string) bool {
	switch name {
	case "pulse_control", "pulse_docker", "pulse_file_edit":
		return true
	default:
		return false
	}
}
