package tools

import (
	"strings"
	"sync"
)

// IsKnownToolName reports whether name is one of the canonical Pulse tool
// names. Used by chat-content sanitisers to gate stripping on a closed
// allowlist (e.g. JSON tool-call shapes leaked by weak local models that
// route the call into content instead of through the tool_calls channel).
func IsKnownToolName(name string) bool {
	if name == "" {
		return false
	}
	initKnownToolNames()
	_, ok := knownToolNamesSet[name]
	return ok
}

// IsKnownToolNamePrefix reports whether prefix can still become a canonical
// Pulse tool name. Streaming chat sanitizers use this to hold a tiny trailing
// token fragment until the next chunk proves whether it is prose or a leaked
// tool call.
func IsKnownToolNamePrefix(prefix string) bool {
	if prefix == "" {
		return false
	}
	initKnownToolNames()
	for _, name := range knownToolNamesList {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

var (
	knownToolNamesOnce sync.Once
	knownToolNamesList []string
	knownToolNamesSet  map[string]struct{}
)

func initKnownToolNames() {
	knownToolNamesOnce.Do(func() {
		e := NewPulseToolExecutor(ExecutorConfig{})
		names := e.registry.allNames()
		knownToolNamesList = names
		knownToolNamesSet = make(map[string]struct{}, len(names))
		for _, n := range names {
			knownToolNamesSet[n] = struct{}{}
		}
	})
}
