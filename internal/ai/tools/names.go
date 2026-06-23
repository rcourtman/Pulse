package tools

import (
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

// IsKnownToolName reports whether name is one of the canonical Pulse tool
// names. Used by chat-content sanitisers to gate stripping on a closed
// allowlist (e.g. JSON tool-call shapes leaked by weak local models that
// route the call into content instead of through the tool_calls channel).
func IsKnownToolName(name string) bool {
	if name == "" {
		return false
	}
	return AssistantProviderToolNameCatalog().Has(name)
}

// IsKnownToolNamePrefix reports whether prefix can still become a canonical
// Pulse tool name. Streaming chat sanitizers use this to hold a tiny trailing
// token fragment until the next chunk proves whether it is prose or a leaked
// tool call.
func IsKnownToolNamePrefix(prefix string) bool {
	if prefix == "" {
		return false
	}
	return AssistantProviderToolNameCatalog().HasPrefix(prefix)
}

// AssistantProviderToolNameCatalog returns the registry-backed provider-tool
// name catalog for the native Assistant surface, including native provider
// tools such as pulse_question.
func AssistantProviderToolNameCatalog() agentcapabilities.ProviderToolNameCatalog {
	initKnownToolNames()
	return knownToolNameCatalog
}

var (
	knownToolNamesOnce   sync.Once
	knownToolNameCatalog agentcapabilities.ProviderToolNameCatalog
)

func initKnownToolNames() {
	knownToolNamesOnce.Do(func() {
		e := NewPulseToolExecutor(ExecutorConfig{})
		knownToolNameCatalog = agentcapabilities.NewAssistantProviderToolNameCatalog(e.registry.allNames())
	})
}
