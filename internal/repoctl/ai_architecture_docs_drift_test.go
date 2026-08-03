package repoctl

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The AI architecture documents describe enumerations and tuned constants that
// live in code. Prose cannot notice when a state, a signal, or an error code is
// added next to it, so these tests derive the truth from the source and fail
// when a document no longer covers it.
//
// Each check is deliberately source-derived rather than a literal-string
// assertion. A test that only asserts "the document contains RESOLVING" keeps
// passing when a fifth state is added, which is exactly the drift being
// guarded against.

const (
	patrolArchitectureDoc    = "docs/PATROL_ARCHITECTURE.md"
	assistantSafetyDoc       = "docs/ASSISTANT_SAFETY.md"
	assistantArchitectureDoc = "docs/ASSISTANT_ARCHITECTURE.md"
)

// constStringValues returns every string literal assigned in the source that
// matches the supplied pattern, which is expected to capture the value.
func constStringValues(t *testing.T, source, pattern string) []string {
	t.Helper()

	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(source, -1)
	if len(matches) == 0 {
		t.Fatalf("pattern %q matched nothing; the source shape changed and this guard is no longer reading it", pattern)
	}

	seen := map[string]struct{}{}
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		value := match[1]
		if _, dup := seen[value]; dup {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

// singleConstValue returns the one value captured by pattern, failing when the
// constant it reads has been renamed or removed.
func singleConstValue(t *testing.T, source, pattern, name string) string {
	t.Helper()

	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(source)
	if match == nil {
		t.Fatalf("could not read %s from source; it was renamed or removed and this guard no longer covers it", name)
	}
	return match[1]
}

func assertDocumentsValues(t *testing.T, docRel, doc string, values []string, subject string) {
	t.Helper()

	var missing []string
	for _, value := range values {
		if !strings.Contains(doc, value) {
			missing = append(missing, value)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%s does not document %s %v declared in code; the document has drifted from the source", docRel, subject, missing)
	}
}

func TestPatrolArchitectureDocMatchesSignalDefinitions(t *testing.T) {
	source := readRepoFile(t, "internal/ai/patrol_signals.go")
	doc := readRepoFile(t, patrolArchitectureDoc)

	// Every SignalType declared in code must appear in the document's table.
	signals := constStringValues(t, source, `Signal\w+\s+SignalType\s*=\s*"([a-z_]+)"`)
	assertDocumentsValues(t, patrolArchitectureDoc, doc, signals, "signal type")

	// And the document must not invent signal types that no longer exist. Only
	// rows of the signal table are considered, so unrelated backticked prose
	// elsewhere in the document cannot trip this.
	documented := docTableCodeValues(doc, "| Signal | Meaning |")
	for _, value := range documented {
		if !containsString(signals, value) {
			t.Errorf("%s documents signal type %q which is not declared in internal/ai/patrol_signals.go", patrolArchitectureDoc, value)
		}
	}

	// Tuned defaults quoted in the threshold table.
	for _, tc := range []struct {
		name    string
		pattern string
		want    string
	}{
		{"signalStorageWarningPercent", `signalStorageWarningPercent\s*=\s*([0-9.]+)`, "75"},
		{"signalStorageCriticalPercent", `signalStorageCriticalPercent\s*=\s*([0-9.]+)`, "95"},
		{"signalHighCPUPercent", `signalHighCPUPercent\s*=\s*([0-9.]+)`, "70"},
		{"signalHighMemoryPercent", `signalHighMemoryPercent\s*=\s*([0-9.]+)`, "80"},
		{"signalBackupStaleThreshold", `signalBackupStaleThreshold\s*=\s*(\d+)\s*\*\s*time\.Hour`, "48"},
	} {
		got := strings.TrimSuffix(singleConstValue(t, source, tc.pattern, tc.name), ".0")
		if got != tc.want {
			t.Errorf("%s is now %s in code but this guard expected %s; update the guard and the threshold table in %s together", tc.name, got, tc.want, patrolArchitectureDoc)
		}
		if !strings.Contains(doc, got) {
			t.Errorf("%s does not document the %s value %s", patrolArchitectureDoc, tc.name, got)
		}
	}
}

func TestPatrolArchitectureDocMatchesInvestigationLimits(t *testing.T) {
	source := readRepoFile(t, "internal/ai/findings.go")
	doc := readRepoFile(t, patrolArchitectureDoc)

	attempts := singleConstValue(t, source, `maxInvestigationAttempts\s*=\s*(\d+)`, "maxInvestigationAttempts")
	if attempts != "3" {
		t.Errorf("maxInvestigationAttempts is now %s; %s still says three attempts", attempts, patrolArchitectureDoc)
	}
	if !strings.Contains(doc, "three times") {
		t.Errorf("%s no longer states the investigation attempt limit", patrolArchitectureDoc)
	}

	cooldown := singleConstValue(t, source, `investigationCooldown\s*=\s*(\d+)\s*\*\s*time\.Hour`, "investigationCooldown")
	if cooldown != "1" {
		t.Errorf("investigationCooldown is now %s hours; %s still says one hour", cooldown, patrolArchitectureDoc)
	}
	if !strings.Contains(doc, "one hour") {
		t.Errorf("%s no longer states the investigation cooldown", patrolArchitectureDoc)
	}
}

func TestAssistantSafetyDocMatchesSessionStateMachine(t *testing.T) {
	source := readRepoFile(t, "internal/ai/chat/fsm.go")
	doc := readRepoFile(t, assistantSafetyDoc)

	states := constStringValues(t, source, `State\w+\s+SessionState\s*=\s*"([A-Z_]+)"`)
	assertDocumentsValues(t, assistantSafetyDoc, doc, states, "session state")

	documentedStates := docTableCodeValues(doc, "| State | What it means |")
	for _, value := range documentedStates {
		if !containsString(states, value) {
			t.Errorf("%s documents session state %q which is not declared in internal/ai/chat/fsm.go", assistantSafetyDoc, value)
		}
	}

	// Tool kinds drive the transitions, so a new kind changes the machine's
	// behaviour and must reach the document. They are an int iota, so the wire
	// names come from the String method rather than from the const block.
	kindSource := readRepoFile(t, "internal/agentcapabilities/tool_call.go")
	kinds := constStringValues(t, kindSource, `case ToolCallKind\w+:\s*\n\s*return "([a-z_]+)"`)
	assertDocumentsValues(t, assistantSafetyDoc, doc, kinds, "tool kind")

	ttl := singleConstValue(t, source, `RecoveryTTL\s*=\s*(\d+)\s*\*\s*time\.Minute`, "RecoveryTTL")
	if ttl != "10" {
		t.Errorf("RecoveryTTL is now %s minutes; %s still says ten minutes", ttl, assistantSafetyDoc)
	}
	if !strings.Contains(doc, "ten minutes") {
		t.Errorf("%s no longer states the pending-recovery expiry", assistantSafetyDoc)
	}
}

func TestAssistantArchitectureDocMatchesAgentErrorCodes(t *testing.T) {
	// The document names a representative sample rather than all 45 declared
	// codes, so the useful direction is document to code. Every code the
	// document names must still exist, which catches a rename or a removal.
	// Requiring the reverse would force a 45 row table that churns constantly.
	declared := map[string]struct{}{}
	for _, value := range constStringValues(t, readRepoFile(t, "internal/agentcapabilities/errors.go"), `AgentErrCode\w+\s*=\s*"([a-z_]+)"`) {
		declared[value] = struct{}{}
	}
	for _, value := range constStringValues(t, readRepoFile(t, "internal/agentcapabilities/tool_response.go"), `ErrCode\w+\s*=\s*"([A-Za-z_]+)"`) {
		declared[value] = struct{}{}
	}

	// Tool names are legitimate identifiers to mention alongside codes, and a
	// rename of either should fail the document, so both are accepted.
	for _, value := range constStringValues(t, readRepoFile(t, "internal/agentcapabilities/invocation.go"), `(\w+ToolName)\s*:`) {
		declared[value] = struct{}{}
	}
	for _, value := range constStringValues(t, readRepoFile(t, "internal/agentcapabilities/tool_names.go"), `\w+ToolName\s*=\s*"([a-z_]+)"`) {
		declared[value] = struct{}{}
	}

	doc := readRepoFile(t, assistantArchitectureDoc)
	for _, named := range docNamedErrorCodes(sectionOf(doc, "## Structured errors")) {
		if _, ok := declared[named]; !ok {
			t.Errorf("%s names error code %q which is not declared in internal/agentcapabilities; it was renamed or removed", assistantArchitectureDoc, named)
		}
	}
}

// sectionOf returns the body of a markdown section, so identifier checks apply
// only where the document is actually making claims about them.
func sectionOf(doc, heading string) string {
	start := strings.Index(doc, heading)
	if start < 0 {
		return ""
	}
	rest := doc[start+len(heading):]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		return rest[:end]
	}
	return rest
}

// docNamedErrorCodes returns backticked identifiers from the document that look
// like error codes, meaning lower snake case or upper snake case tokens.
func docNamedErrorCodes(doc string) []string {
	re := regexp.MustCompile("`([a-z]+_[a-z_]+|[A-Z]+_[A-Z_]+)`")
	seen := map[string]struct{}{}
	var values []string
	for _, match := range re.FindAllStringSubmatch(doc, -1) {
		value := match[1]
		if _, dup := seen[value]; dup {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func TestAssistantArchitectureDocMatchesLoopBounds(t *testing.T) {
	source := readRepoFile(t, "internal/ai/chat/agentic.go")
	doc := readRepoFile(t, assistantArchitectureDoc)

	blocks := singleConstValue(t, source, `maxLookGateBlocks\s*=\s*(\d+)`, "maxLookGateBlocks")
	if blocks != "2" {
		t.Errorf("maxLookGateBlocks is now %s; %s still says the gate allows two blocks", blocks, assistantArchitectureDoc)
	}
	if !strings.Contains(doc, "two blocks") {
		t.Errorf("%s no longer states the look-before-asking gate bound", assistantArchitectureDoc)
	}

	// The concurrency cap is described in the parallel-execution comment rather
	// than a named constant, so the comment itself is the contract.
	if !strings.Contains(source, "concurrency capped at four") {
		t.Errorf("the parallel execution cap in internal/ai/chat/agentic.go changed; %s still says four", assistantArchitectureDoc)
	}
	if !strings.Contains(doc, "capped at four") {
		t.Errorf("%s no longer states the tool execution concurrency cap", assistantArchitectureDoc)
	}
}

// docTableCodeValues returns the backticked values in the first column of the
// markdown table introduced by header, so a document's own claims can be
// compared against the source.
func docTableCodeValues(doc, header string) []string {
	index := strings.Index(doc, header)
	if index < 0 {
		return nil
	}

	cell := regexp.MustCompile("^\\|\\s*`([^`]+)`")
	var values []string
	for _, line := range strings.Split(doc[index:], "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			break
		}
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		if match := cell.FindStringSubmatch(trimmed); match != nil {
			values = append(values, match[1])
		}
	}
	return values
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
