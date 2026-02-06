package api

// Code standards tests: these act as linter rules that run in CI.
// They scan source files for anti-patterns that indicate DRY violations
// and fail the build if someone reintroduces a consolidated pattern.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// readGoFiles returns the contents of all non-test .go files in the api package directory.
func readGoFiles(t *testing.T) map[string]string {
	t.Helper()
	files := make(map[string]string)

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read api directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		files[name] = string(data)
	}
	return files
}

// TestNoInline402Responses ensures handlers use WriteLicenseRequired() instead of
// manually writing HTTP 402 responses. The only files allowed to write 402 directly
// are license_handlers.go and middleware_license.go (which define the helpers).
func TestNoInline402Responses(t *testing.T) {
	allowedFiles := map[string]bool{
		"license_handlers.go":   true,
		"middleware_license.go": true,
	}

	// Match patterns like: WriteHeader(402), WriteHeader(http.StatusPaymentRequired),
	// http.Error(...402...), StatusPaymentRequired (used in a write context)
	inline402 := regexp.MustCompile(`WriteHeader\(\s*(402|http\.StatusPaymentRequired)\s*\)`)

	for name, content := range readGoFiles(t) {
		if allowedFiles[name] {
			continue
		}
		if matches := inline402.FindAllStringIndex(content, -1); len(matches) > 0 {
			for _, m := range matches {
				line := 1 + strings.Count(content[:m[0]], "\n")
				t.Errorf("%s:%d: inline 402 response — use WriteLicenseRequired() from license_handlers.go instead", name, line)
			}
		}
	}
}

// TestNoAgentHandlerMethodRedefinition ensures that SetMonitor, SetMultiTenantMonitor,
// and getMonitor are only defined on baseAgentHandlers, not on concrete handler types.
func TestNoAgentHandlerMethodRedefinition(t *testing.T) {
	// These methods should only be defined in agent_handlers_base.go
	patterns := []struct {
		re   *regexp.Regexp
		name string
	}{
		{regexp.MustCompile(`func \(h \*(?:Docker|Kubernetes|Host)AgentHandlers\) SetMonitor\b`), "SetMonitor"},
		{regexp.MustCompile(`func \(h \*(?:Docker|Kubernetes|Host)AgentHandlers\) SetMultiTenantMonitor\b`), "SetMultiTenantMonitor"},
		{regexp.MustCompile(`func \(h \*(?:Docker|Kubernetes|Host)AgentHandlers\) getMonitor\b`), "getMonitor"},
	}

	for name, content := range readGoFiles(t) {
		if name == "agent_handlers_base.go" {
			continue
		}
		for _, p := range patterns {
			if loc := p.re.FindStringIndex(content); loc != nil {
				line := 1 + strings.Count(content[:loc[0]], "\n")
				t.Errorf("%s:%d: %s() redefined on concrete handler type — it's inherited from baseAgentHandlers", name, line, p.name)
			}
		}
	}
}

// TestNoRawBroadcastStateInAgentHandlers ensures agent handler files use
// h.broadcastState(ctx) instead of the raw go h.wsHub.BroadcastState(...) pattern.
func TestNoRawBroadcastStateInAgentHandlers(t *testing.T) {
	agentFiles := map[string]bool{
		"docker_agents.go":     true,
		"kubernetes_agents.go": true,
		"host_agents.go":       true,
	}

	raw := regexp.MustCompile(`go h\.wsHub\.BroadcastState\(`)

	for name, content := range readGoFiles(t) {
		if !agentFiles[name] {
			continue
		}
		if matches := raw.FindAllStringIndex(content, -1); len(matches) > 0 {
			for _, m := range matches {
				line := 1 + strings.Count(content[:m[0]], "\n")
				t.Errorf("%s:%d: raw BroadcastState call — use h.broadcastState(r.Context()) instead", name, line)
			}
		}
	}
}
