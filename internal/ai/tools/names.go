package tools

import "sync"

// KnownToolNames returns the canonical list of registered Pulse tool names,
// keyed off the registry built by registerTools(). The list is built lazily
// on first call via a throwaway executor so adding a new tool to
// registerTools() automatically extends the allowlist — no separate
// hand-maintained list to drift out of sync.
func KnownToolNames() []string {
	initKnownToolNames()
	out := make([]string, len(knownToolNamesList))
	copy(out, knownToolNamesList)
	return out
}

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
