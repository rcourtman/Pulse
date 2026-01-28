package tools

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// routingMismatchLogLimiter provides rate limiting for routing mismatch debug logs.
// This prevents log spam while still providing visibility into routing issues.
var routingMismatchLogLimiter = struct {
	mu       sync.Mutex
	lastLog  time.Time
	interval time.Duration
}{
	interval: 10 * time.Second, // Log at most once per 10 seconds
}

// ErrStrictResolution is returned when a write operation is attempted on an
// undiscovered resource while PULSE_STRICT_RESOLUTION is enabled.
// Use errors.As() to check for this error type.
type ErrStrictResolution struct {
	ResourceID string // The resource identifier that wasn't found
	Action     string // The action that was attempted
	Message    string // Human-readable message
}

func (e *ErrStrictResolution) Error() string {
	return e.Message
}

// Code returns the error code for structured responses
func (e *ErrStrictResolution) Code() string {
	return ErrCodeStrictResolution
}

// ToToolResponse returns a consistent ToolResponse for blocked operations.
// This enables the agentic loop to detect and auto-recover (discover then retry).
func (e *ErrStrictResolution) ToToolResponse() ToolResponse {
	return NewToolBlockedError(
		ErrCodeStrictResolution,
		e.Message,
		map[string]interface{}{
			"resource_id":      e.ResourceID,
			"action":           e.Action,
			"recovery_hint":    "Use pulse_query action=search to discover the resource first",
			"auto_recoverable": true, // Signal to agentic loop that auto-discovery can help
		},
	)
}

// ToStructuredError returns a structured error payload for tool responses
// Deprecated: Use ToToolResponse() for consistent envelope
func (e *ErrStrictResolution) ToStructuredError() map[string]interface{} {
	return map[string]interface{}{
		"error_code":  e.Code(),
		"message":     e.Message,
		"resource_id": e.ResourceID,
		"action":      e.Action,
	}
}

// ErrRoutingMismatch is returned when a tool targets a parent host (e.g., Proxmox node)
// but the session has discovered more specific child resources (LXC/VM) on that host.
// This prevents accidentally operating on the host filesystem when the user intended
// to target a container.
type ErrRoutingMismatch struct {
	TargetHost            string   // The host that was targeted
	MoreSpecificResources []string // Child resource names that exist on this host
	MoreSpecificIDs       []string // Canonical resource IDs (kind:host:id) for future ID-based targeting
	ChildKinds            []string // Resource kinds of children (for telemetry: "lxc", "vm", etc.)
	Message               string   // Human-readable message
}

func (e *ErrRoutingMismatch) Error() string {
	return e.Message
}

// Code returns the error code for structured responses
func (e *ErrRoutingMismatch) Code() string {
	return "ROUTING_MISMATCH"
}

// ToToolResponse returns a consistent ToolResponse for routing mismatches.
func (e *ErrRoutingMismatch) ToToolResponse() ToolResponse {
	details := map[string]interface{}{
		"target_host":             e.TargetHost,
		"more_specific_resources": e.MoreSpecificResources,
		"auto_recoverable":        true,
	}

	// Include canonical IDs and prefer ID-based targeting in recovery hint
	if len(e.MoreSpecificIDs) > 0 {
		details["more_specific_resource_ids"] = e.MoreSpecificIDs
		details["target_resource_id"] = e.MoreSpecificIDs[0] // Primary suggestion
		// Prefer ID-based targeting, with legacy target_host as fallback
		details["recovery_hint"] = fmt.Sprintf(
			"Retry with target_resource_id='%s' (preferred) or target_host='%s' (legacy)",
			e.MoreSpecificIDs[0], e.MoreSpecificResources[0])
	} else {
		// Fallback if no IDs available
		details["recovery_hint"] = fmt.Sprintf(
			"Use target_host='%s' to target the specific resource, not the parent Proxmox host",
			e.MoreSpecificResources[0])
	}

	return NewToolBlockedError(
		"ROUTING_MISMATCH",
		e.Message,
		details,
	)
}

// isStrictResolutionEnabled returns true if hard validation is enabled for write operations.
// Set PULSE_STRICT_RESOLUTION=true to block write operations on undiscovered resources.
func isStrictResolutionEnabled() bool {
	val := os.Getenv("PULSE_STRICT_RESOLUTION")
	return val == "true" || val == "1" || val == "yes"
}

// isWriteAction returns true if the action is a write/mutating operation.
// Note: "exec" is treated as write because it can execute arbitrary commands.
// For finer control, use classifyCommandRisk() to distinguish read-only exec commands.
func isWriteAction(action string) bool {
	writeActions := map[string]bool{
		"start":    true,
		"stop":     true,
		"restart":  true,
		"delete":   true,
		"shutdown": true,
		"exec":     true,
		"write":    true,
		"append":   true,
	}
	return writeActions[action]
}

// CommandRisk represents the risk level of a shell command
type CommandRisk int

const (
	CommandRiskReadOnly    CommandRisk = 0 // Safe read-only commands
	CommandRiskLowWrite    CommandRisk = 1 // Low-risk writes (touch, mkdir temp)
	CommandRiskMediumWrite CommandRisk = 2 // Medium-risk writes (config changes)
	CommandRiskHighWrite   CommandRisk = 3 // High-risk writes (rm, systemctl, package managers)
)

// ExecutionIntent represents whether a command can be proven non-mutating.
// This is the primary abstraction for pulse_read gating decisions.
//
// Invariant: pulse_read may execute commands that are provably non-mutating
// either by construction (known read-only commands) or by bounded inspection
// (self-contained input + no shell composition + no write patterns). Any command
// that depends on external input, shell composition, or ambiguous semantics is
// treated as write-capable and blocked from pulse_read.
type ExecutionIntent int

const (
	// IntentReadOnlyCertain - command is non-mutating by construction.
	// Examples: ls, cat, grep, docker logs, ffprobe, kubectl get
	// These cannot mutate regardless of arguments.
	IntentReadOnlyCertain ExecutionIntent = iota

	// IntentReadOnlyConditional - command appears read-only by bounded inspection.
	// The command is self-contained (no shell composition) and content inspection
	// found no write patterns. Examples: sqlite3 "SELECT ...", psql -c "SELECT ..."
	// Guardrails: no redirects, no pipes, no subshells, no chaining, inline input only.
	IntentReadOnlyConditional

	// IntentWriteOrUnknown - command may mutate or cannot be proven safe.
	// Either it matches known write patterns, has shell composition that prevents
	// analysis, or is unknown and we fail closed.
	IntentWriteOrUnknown
)

// IntentResult contains the execution intent classification and the reason for it.
type IntentResult struct {
	Intent              ExecutionIntent
	Reason              string                     // Human-readable reason for classification
	NonInteractiveBlock *NonInteractiveBlockResult // Non-nil if blocked by NonInteractiveOnly guardrail
}

// ContentInspector examines command content to determine if it's read-only.
// Different inspectors handle different tool families (SQL, Redis, kubectl, etc.)
type ContentInspector interface {
	// Applies returns true if this inspector handles the given command
	Applies(cmdLower string) bool
	// IsReadOnly returns (true, "") if content is read-only, or (false, reason) if not
	IsReadOnly(cmdLower string) (bool, string)
}

// sqlContentInspector handles SQL CLI tools (sqlite3, mysql, psql, etc.)
type sqlContentInspector struct{}

func (s *sqlContentInspector) Applies(cmdLower string) bool {
	sqlCLIs := []string{"sqlite3 ", "mysql ", "mariadb ", "psql ", "mycli ", "pgcli ", "litecli "}
	for _, cli := range sqlCLIs {
		if strings.Contains(cmdLower, cli) || strings.HasPrefix(cmdLower, strings.TrimSuffix(cli, " ")) {
			return true
		}
	}
	return false
}

func (s *sqlContentInspector) IsReadOnly(cmdLower string) (bool, string) {
	// SQL statements that mutate data or schema.
	// Conservative: includes DDL, DML writes, transaction control, and admin commands.
	sqlWriteKeywords := []string{
		// DML writes
		"insert ", "update ", "delete ", "replace ",
		// DDL
		"create ", "drop ", "alter ", "truncate ",
		"merge ", "upsert ",
		// Transaction control (expands attack surface)
		"begin", "commit", "rollback", "savepoint", "release ",
		// Database management
		"attach ", "detach ",
		"vacuum", "reindex",
		"grant ", "revoke ",
		"pragma ",
	}
	for _, kw := range sqlWriteKeywords {
		if strings.Contains(cmdLower, kw) {
			return false, fmt.Sprintf("SQL contains write/control keyword: %s", strings.TrimSpace(kw))
		}
	}

	// Conservative: if we can't find inline SQL content, assume external input
	hasInlineSQL := strings.Contains(cmdLower, `"`) ||
		strings.Contains(cmdLower, `'`) ||
		strings.Contains(cmdLower, " .") || // dot commands like .tables, .schema
		strings.Contains(cmdLower, " -e ") || // mysql -e
		strings.Contains(cmdLower, " -c ") // psql -c
	if !hasInlineSQL {
		return false, "no inline SQL found; input may be external (piped/interactive)"
	}

	return true, ""
}

// registeredInspectors is the list of content inspectors to try.
// Add new inspectors here for redis-cli, kubectl, etc.
var registeredInspectors = []ContentInspector{
	&sqlContentInspector{},
	// Future: &redisContentInspector{},
	// Future: &kubectlContentInspector{},
}

// ClassifyExecutionIntent determines whether a command can be proven non-mutating.
// This is the main entry point for pulse_read gating decisions.
func ClassifyExecutionIntent(command string) IntentResult {
	cmdLower := strings.ToLower(command)

	// === PHASE 1: Mutation-capability guards ===
	// These make ANY command potentially dangerous regardless of the binary.
	// Includes: sudo, redirects, tee, subshells, pipes, shell chaining
	if reason := checkMutationCapabilityGuards(command, cmdLower); reason != "" {
		return IntentResult{Intent: IntentWriteOrUnknown, Reason: reason}
	}

	// === PHASE 1.5: NonInteractiveOnly guardrails ===
	// MUST be checked before Phase 3 (read-only by construction) because even
	// read-only commands like `tail -f` and `journalctl -f` can hang indefinitely.
	// pulse_read requires commands that terminate deterministically.
	if niBlock := checkNonInteractiveGuardrails(command, cmdLower); niBlock != nil {
		return IntentResult{
			Intent:              IntentWriteOrUnknown,
			Reason:              niBlock.FormatMessage(),
			NonInteractiveBlock: niBlock,
		}
	}

	// === PHASE 2: Known write patterns ===
	// Check BEFORE read-only patterns to catch write variants like "sed -i"
	// before generic patterns like "sed " match.
	if reason := matchesWritePatterns(cmdLower); reason != "" {
		return IntentResult{Intent: IntentWriteOrUnknown, Reason: reason}
	}

	// === PHASE 3: Known read-only by construction ===
	// Commands that cannot mutate regardless of arguments.
	// Only reached if Phase 2 didn't match any write patterns.
	if isReadOnlyByConstruction(cmdLower) {
		return IntentResult{Intent: IntentReadOnlyCertain, Reason: "known read-only command"}
	}

	// === PHASE 4: Self-contained read candidate check ===
	// Additional guardrails before content inspection.
	if reason := checkSelfContainedGuardrails(command, cmdLower); reason != "" {
		return IntentResult{Intent: IntentWriteOrUnknown, Reason: reason}
	}

	// === PHASE 5: Content inspection via registered inspectors ===
	for _, inspector := range registeredInspectors {
		if inspector.Applies(cmdLower) {
			if isReadOnly, reason := inspector.IsReadOnly(cmdLower); isReadOnly {
				return IntentResult{Intent: IntentReadOnlyConditional, Reason: "content inspection: read-only"}
			} else {
				return IntentResult{Intent: IntentWriteOrUnknown, Reason: "content inspection: " + reason}
			}
		}
	}

	// === PHASE 6: Conservative fallback ===
	// Unknown command with no inspector match → treat as write
	return IntentResult{Intent: IntentWriteOrUnknown, Reason: "unknown command; no inspector matched"}
}

// checkMutationCapabilityGuards checks for shell patterns that enable mutation
// regardless of the underlying command. Returns reason if any guard fails.
//
// Also includes NonInteractiveOnly guardrails - pulse_read runs non-interactively,
// so commands requiring TTY or indefinite streaming are blocked.
func checkMutationCapabilityGuards(command, cmdLower string) string {
	// sudo escalates any command
	if strings.Contains(cmdLower, "sudo ") || strings.HasPrefix(cmdLower, "sudo") {
		return "sudo escalates command privileges"
	}

	// Output redirection can overwrite files
	if hasStdoutRedirect(command) {
		return "output redirection can overwrite files"
	}
	if strings.Contains(cmdLower, " tee ") || strings.Contains(cmdLower, "|tee ") {
		return "tee can write to files"
	}

	// Subshell/command substitution can execute arbitrary commands
	if strings.Contains(command, "$(") || strings.Contains(command, "`") {
		return "command substitution can execute arbitrary commands"
	}

	// Input redirection means we can't inspect the content.
	// This catches: < (redirect), << (heredoc), <<< (here-string)
	// Examples blocked:
	//   sqlite3 db < script.sql
	//   psql <<EOF ... EOF
	//   sqlite3 db <<< "SELECT ..."
	if strings.Contains(command, "<") {
		return "input redirection prevents content inspection"
	}

	// Pipes: only block when piping to a dual-use tool that interprets input
	// (like SQL CLIs). Piping to read-only filters (grep, head, etc.) is safe.
	if strings.Contains(command, "|") && !strings.Contains(command, "||") {
		if pipedToDualUseTool(cmdLower) {
			return "piped input to dual-use tool prevents content inspection"
		}
	}

	// Shell chaining outside quotes (;, &&, ||)
	if hasShellChainingOutsideQuotes(command) {
		return "shell chaining detected outside quotes"
	}

	return ""
}

// NonInteractiveBlockResult contains structured information about a NonInteractiveOnly block.
type NonInteractiveBlockResult struct {
	Category        string // telemetry category: tty_flag, pager, unbounded_stream, interactive_repl
	Message         string // human-readable reason
	SuggestedCmd    string // drop-in rewrite suggestion (empty if none available)
	AutoRecoverable bool   // true if the suggested rewrite is safe for auto-recovery
}

// checkNonInteractiveGuardrails enforces the exit-boundedness invariant:
// pulse_read must only execute commands that terminate deterministically.
//
// Categories (for telemetry labels):
//   - tty_flag: interactive TTY flags (-it, --tty)
//   - pager: pager/editor tools (less, vim, nano)
//   - unbounded_stream: follow mode without bounds (-f without -n/--since/timeout)
//   - interactive_repl: commands that open REPL/shell without non-interactive flags
//
// Returns nil if command passes all guardrails.
func checkNonInteractiveGuardrails(command, cmdLower string) *NonInteractiveBlockResult {
	// [tty_flag] Interactive TTY flags allocate a terminal
	if hasInteractiveTTYFlags(cmdLower) {
		return &NonInteractiveBlockResult{
			Category:        "tty_flag",
			Message:         "[tty_flag] interactive/TTY flags require terminal; use non-interactive form",
			SuggestedCmd:    suggestNonInteractiveTTY(command),
			AutoRecoverable: true,
		}
	}

	// [pager] Pager and editor tools require terminal interaction
	if isPagerOrEditorTool(cmdLower) {
		return &NonInteractiveBlockResult{
			Category:        "pager",
			Message:         "[pager] pager/editor tools require terminal; use cat, head, or tail instead",
			SuggestedCmd:    suggestPagerReplacement(command, cmdLower),
			AutoRecoverable: true,
		}
	}

	// [unbounded_stream] Live monitoring tools never terminate
	if isLiveMonitoringTool(cmdLower) {
		return &NonInteractiveBlockResult{
			Category:        "unbounded_stream",
			Message:         "[unbounded_stream] live monitoring tools run indefinitely; use bounded alternatives",
			SuggestedCmd:    suggestLiveMonitoringReplacement(command, cmdLower),
			AutoRecoverable: true,
		}
	}

	// [unbounded_stream] Follow mode without explicit bound
	if isUnboundedStreaming(cmdLower) {
		return &NonInteractiveBlockResult{
			Category:        "unbounded_stream",
			Message:         "[unbounded_stream] follow mode without bound; add --tail/--since or wrap with timeout",
			SuggestedCmd:    suggestBoundedStreaming(command, cmdLower),
			AutoRecoverable: true,
		}
	}

	// [interactive_repl] Commands that open REPL/interactive session
	if isInteractiveREPL(cmdLower) {
		return &NonInteractiveBlockResult{
			Category:        "interactive_repl",
			Message:         "[interactive_repl] command opens interactive session; add -c/--execute flag or inline command",
			SuggestedCmd:    suggestNonInteractiveREPL(command, cmdLower),
			AutoRecoverable: false, // REPL rewrites need human judgment (what query to run?)
		}
	}

	return nil
}

// FormatNonInteractiveBlock formats a block result for tool response.
func (r *NonInteractiveBlockResult) FormatMessage() string {
	if r.SuggestedCmd != "" {
		return fmt.Sprintf("%s\n\nSuggested rewrite:\n  %s", r.Message, r.SuggestedCmd)
	}
	return r.Message
}

// suggestNonInteractiveTTY suggests removing -it flags from docker/kubectl commands.
func suggestNonInteractiveTTY(command string) string {
	// Remove -it, -i -t, --interactive, --tty flags
	result := command
	replacements := []struct{ old, new string }{
		{" -it ", " "},
		{" -i -t ", " "},
		{" -ti ", " "},
		{" --interactive --tty ", " "},
		{" --tty --interactive ", " "},
		{" --interactive ", " "},
		{" --tty ", " "},
	}
	for _, r := range replacements {
		result = strings.ReplaceAll(result, r.old, r.new)
	}
	// Clean up double spaces
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}
	if result != command {
		return strings.TrimSpace(result)
	}
	return ""
}

// suggestPagerReplacement suggests cat/head/tail instead of pagers.
func suggestPagerReplacement(command, cmdLower string) string {
	parts := strings.Fields(command)
	if len(parts) < 2 {
		return ""
	}
	// Extract the file argument (everything after the pager command)
	fileArgs := strings.Join(parts[1:], " ")
	pager := strings.ToLower(parts[0])

	switch pager {
	case "less", "more":
		return fmt.Sprintf("cat %s | head -200", fileArgs)
	case "vim", "vi", "nano", "emacs", "pico", "ed":
		return fmt.Sprintf("cat %s", fileArgs)
	}
	return ""
}

// suggestLiveMonitoringReplacement suggests bounded alternatives for live tools.
func suggestLiveMonitoringReplacement(command, cmdLower string) string {
	parts := strings.Fields(cmdLower)
	if len(parts) == 0 {
		return ""
	}
	tool := parts[0]

	switch tool {
	case "top", "htop", "atop":
		return "ps aux --sort=-%cpu | head -20"
	case "iotop":
		return "iotop -b -n 1" // batch mode, 1 iteration
	case "iftop", "nload":
		return "ss -s" // socket statistics summary
	case "watch":
		// Extract the watched command and suggest running it once
		if len(parts) > 1 {
			// Skip watch and any flags like -n 1
			cmdStart := 1
			for i := 1; i < len(parts); i++ {
				if strings.HasPrefix(parts[i], "-") {
					cmdStart = i + 1
					if parts[i] == "-n" && i+1 < len(parts) {
						cmdStart = i + 2
					}
				} else {
					break
				}
			}
			if cmdStart < len(parts) {
				watchedCmd := strings.Join(strings.Fields(command)[cmdStart:], " ")
				return strings.Trim(watchedCmd, "'\"")
			}
		}
	}
	return ""
}

// suggestBoundedStreaming adds --tail/--since bounds to streaming commands.
func suggestBoundedStreaming(command, cmdLower string) string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}
	tool := strings.ToLower(parts[0])

	switch {
	case tool == "tail":
		// tail -f /var/log/app.log → tail -n 200 --follow=name /var/log/app.log (or just remove -f)
		// Simplest: add -n 200 and keep -f for "recent + follow with bound"
		// Or suggest removing -f entirely
		result := strings.ReplaceAll(command, " -f", " -n 200")
		result = strings.ReplaceAll(result, " --follow", " -n 200")
		return result

	case tool == "journalctl":
		// journalctl -f → journalctl -n 200 --since "10 min ago"
		result := command
		if strings.Contains(cmdLower, " -f") {
			result = strings.ReplaceAll(result, " -f", ` -n 200 --since "10 min ago"`)
		}
		if strings.Contains(cmdLower, " --follow") {
			result = strings.ReplaceAll(result, " --follow", ` -n 200 --since "10 min ago"`)
		}
		return result

	case strings.HasPrefix(tool, "docker") && strings.Contains(cmdLower, "logs"):
		// docker logs -f container → docker logs --tail=200 container
		result := strings.ReplaceAll(command, " -f ", " --tail=200 ")
		result = strings.ReplaceAll(result, " -f", " --tail=200")
		result = strings.ReplaceAll(result, " --follow ", " --tail=200 ")
		result = strings.ReplaceAll(result, " --follow", " --tail=200")
		return result

	case strings.HasPrefix(tool, "kubectl") && strings.Contains(cmdLower, "logs"):
		// kubectl logs -f pod → kubectl logs --tail=200 --since=10m pod
		result := strings.ReplaceAll(command, " -f ", " --tail=200 --since=10m ")
		result = strings.ReplaceAll(result, " -f", " --tail=200 --since=10m")
		result = strings.ReplaceAll(result, " --follow ", " --tail=200 --since=10m ")
		result = strings.ReplaceAll(result, " --follow", " --tail=200 --since=10m")
		return result

	case tool == "dmesg":
		// dmesg -w → dmesg | tail -200
		result := strings.ReplaceAll(command, " -w", "")
		result = strings.ReplaceAll(result, " --follow", "")
		return result + " | tail -200"
	}
	return ""
}

// suggestNonInteractiveREPL suggests non-interactive form for REPL commands.
// Returns empty string for cases needing human judgment (what query to run?).
func suggestNonInteractiveREPL(command, cmdLower string) string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}
	tool := strings.ToLower(parts[0])

	// For SQL CLIs and REPLs, we can't suggest a specific query
	// but we can show the pattern
	switch tool {
	case "mysql", "mariadb":
		return fmt.Sprintf("%s -e \"SELECT ...\"", command)
	case "psql":
		return fmt.Sprintf("%s -c \"SELECT ...\"", command)
	case "sqlite3":
		return fmt.Sprintf("%s \"SELECT ...\"", command)
	case "redis-cli":
		return fmt.Sprintf("%s PING", command)
	case "python", "python3", "python2":
		return fmt.Sprintf("%s -c \"...\"", parts[0])
	case "node", "nodejs":
		return fmt.Sprintf("%s -e \"...\"", parts[0])
	case "ssh":
		// ssh host → ssh host "command"
		return fmt.Sprintf("%s \"ls -la\"", command)
	}
	return ""
}

// hasInteractiveTTYFlags detects flags that request interactive/TTY mode.
func hasInteractiveTTYFlags(cmdLower string) bool {
	// Only check for docker/kubectl commands
	isDockerKubectl := strings.HasPrefix(cmdLower, "docker ") ||
		strings.HasPrefix(cmdLower, "kubectl ")
	if !isDockerKubectl {
		return false
	}

	// Docker/kubectl -it or -i -t combinations (common shorthand)
	if strings.Contains(cmdLower, " -it ") || strings.Contains(cmdLower, " -it\t") ||
		strings.HasSuffix(cmdLower, " -it") ||
		strings.Contains(cmdLower, " -ti ") || strings.Contains(cmdLower, " -ti\t") ||
		strings.HasSuffix(cmdLower, " -ti") {
		return true
	}

	// Explicit long flags
	if strings.Contains(cmdLower, " --tty") || strings.Contains(cmdLower, " --interactive") {
		return true
	}

	// Check for standalone -t and -i flags that aren't part of other patterns
	// Avoid matching: 2>&1 (stderr redirect), -t tablename, etc.
	// Look for " -t " or " -i " as standalone flags followed by non-alphanumeric
	parts := strings.Fields(cmdLower)
	for i, part := range parts {
		if part == "-t" || part == "-i" {
			// Found standalone -t or -i flag
			// Check if this is in the context of exec/run subcommands
			for j := 0; j < i; j++ {
				if parts[j] == "exec" || parts[j] == "run" {
					return true
				}
			}
		}
	}

	return false
}

// isPagerOrEditorTool detects pager and editor tools that require terminal interaction.
func isPagerOrEditorTool(cmdLower string) bool {
	// Extract first word
	firstWord := cmdLower
	if spaceIdx := strings.Index(cmdLower, " "); spaceIdx > 0 {
		firstWord = cmdLower[:spaceIdx]
	}

	pagerEditorTools := []string{"less", "more", "vim", "vi", "nano", "emacs", "pico", "ed"}
	for _, tool := range pagerEditorTools {
		if firstWord == tool {
			return true
		}
	}
	return false
}

// isLiveMonitoringTool detects tools that run indefinitely showing live data.
func isLiveMonitoringTool(cmdLower string) bool {
	firstWord := cmdLower
	if spaceIdx := strings.Index(cmdLower, " "); spaceIdx > 0 {
		firstWord = cmdLower[:spaceIdx]
	}

	// These tools run until interrupted
	liveTools := []string{"top", "htop", "atop", "iotop", "iftop", "nload", "watch"}
	for _, tool := range liveTools {
		if firstWord == tool {
			return true
		}
	}
	return false
}

// isUnboundedStreaming detects follow-mode commands without an exit bound.
// Exit-bounded = terminates deterministically (line count, time window, or timeout wrapper).
//
// Allowed (exit-bounded):
//   - journalctl -n 100, tail -n 50, tail -100 -f, kubectl logs --tail=100
//   - journalctl --since "10 min ago", kubectl logs --since=10m
//   - timeout 5s tail -f
//
// Blocked (runs indefinitely):
//   - journalctl -f, tail -f, kubectl logs -f, dmesg -w
func isUnboundedStreaming(cmdLower string) bool {
	// Only certain commands support follow mode - don't flag -f on other commands
	// (e.g., "hostname -f" uses -f for "full", not "follow")
	streamingCommands := []string{"tail ", "journalctl ", "docker logs ", "kubectl logs ", "dmesg "}
	isStreamingCmd := false
	for _, prefix := range streamingCommands {
		if strings.HasPrefix(cmdLower, prefix) {
			isStreamingCmd = true
			break
		}
	}
	if !isStreamingCmd {
		return false
	}

	// Check for follow flags
	hasFollowFlag := strings.Contains(cmdLower, " -f") ||
		strings.Contains(cmdLower, " --follow") ||
		strings.Contains(cmdLower, " -w") // dmesg uses -w/--follow

	if !hasFollowFlag {
		return false
	}

	// If wrapped in timeout, it's exit-bounded
	if strings.HasPrefix(cmdLower, "timeout ") {
		return false
	}

	// Check for explicit bounds that make it exit-bounded:
	// - Line count: -n, --lines, --tail
	// - Time window: --since, --until (journalctl/kubectl logs)
	hasBound := strings.Contains(cmdLower, " -n ") ||
		strings.Contains(cmdLower, " -n=") ||
		strings.Contains(cmdLower, " --lines") ||
		strings.Contains(cmdLower, " --tail=") ||
		strings.Contains(cmdLower, " --tail ") ||
		strings.Contains(cmdLower, " --since") || // journalctl --since "10 min ago", kubectl logs --since=10m
		strings.Contains(cmdLower, " --until") || // journalctl --until "2024-01-01"
		hasTailShorthandBound(cmdLower) // tail -100 shorthand

	// Follow flag without bounds = runs indefinitely
	return !hasBound
}

// hasTailShorthandBound checks for tail's -N shorthand (e.g., tail -100 -f)
func hasTailShorthandBound(cmdLower string) bool {
	if !strings.HasPrefix(cmdLower, "tail ") {
		return false
	}
	// Look for -NUMBER pattern (tail's shorthand for -n NUMBER)
	// Match patterns like: tail -100, tail -50 -f
	parts := strings.Fields(cmdLower)
	for _, part := range parts {
		if len(part) >= 2 && part[0] == '-' {
			// Check if rest is digits
			allDigits := true
			for _, c := range part[1:] {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits && len(part) > 1 {
				return true
			}
		}
	}
	return false
}

// isInteractiveREPL detects commands that open an interactive REPL/shell
// unless given explicit non-interactive flags (-c, --execute, inline command).
//
// Blocked (opens REPL):
//   - ssh host (no command)
//   - mysql, psql, sqlite3 db (no -c/-e/inline SQL)
//   - redis-cli (no command args)
//   - python, node, irb (no script/command)
//   - openssl s_client
//
// Allowed (non-interactive):
//   - ssh host "command"
//   - mysql -e "SELECT 1"
//   - sqlite3 db "SELECT 1"
//   - python -c "print(1)"
//   - python script.py
func isInteractiveREPL(cmdLower string) bool {
	firstWord := cmdLower
	if spaceIdx := strings.Index(cmdLower, " "); spaceIdx > 0 {
		firstWord = cmdLower[:spaceIdx]
	}

	// SSH: interactive unless a command is provided after host
	// ssh host -> interactive
	// ssh host "ls -la" -> non-interactive
	// ssh -t host -> interactive (explicit TTY)
	if firstWord == "ssh" {
		// If has -t flag, it's explicitly requesting TTY
		if strings.Contains(cmdLower, " -t ") || strings.Contains(cmdLower, " -t") {
			return true
		}
		// Count non-flag arguments after ssh
		// ssh [options] host [command]
		parts := strings.Fields(cmdLower)
		nonFlagArgs := 0
		skipNext := false
		for i, part := range parts[1:] { // skip "ssh"
			if skipNext {
				skipNext = false
				continue
			}
			// Skip flags that take arguments
			if part == "-i" || part == "-l" || part == "-p" || part == "-o" || part == "-F" {
				skipNext = true
				continue
			}
			// Skip other flags
			if strings.HasPrefix(part, "-") {
				continue
			}
			nonFlagArgs++
			// If we have more than just the host, there's a command
			if nonFlagArgs > 1 || (nonFlagArgs == 1 && i < len(parts)-2) {
				return false // has command, not interactive
			}
		}
		// Only host, no command = interactive
		return nonFlagArgs <= 1
	}

	// SQL CLIs: handled by sqlContentInspector, but catch bare invocations
	// mysql, psql without -c/-e, sqlite3 without inline SQL
	if firstWord == "mysql" || firstWord == "mariadb" {
		// Non-interactive if has -e or --execute
		if strings.Contains(cmdLower, " -e ") || strings.Contains(cmdLower, " -e\"") ||
			strings.Contains(cmdLower, " --execute") {
			return false
		}
		// Non-interactive if has piped input (handled elsewhere, but check)
		if strings.Contains(cmdLower, " < ") || strings.Contains(cmdLower, " <<") {
			return false
		}
		return true // bare mysql = interactive
	}

	if firstWord == "psql" {
		// Non-interactive if has -c or --command
		if strings.Contains(cmdLower, " -c ") || strings.Contains(cmdLower, " -c\"") ||
			strings.Contains(cmdLower, " --command") {
			return false
		}
		if strings.Contains(cmdLower, " < ") || strings.Contains(cmdLower, " <<") {
			return false
		}
		return true
	}

	// redis-cli: interactive without command arguments
	if firstWord == "redis-cli" {
		// Check for command after connection flags
		parts := strings.Fields(cmdLower)
		hasCommand := false
		skipNext := false
		for _, part := range parts[1:] {
			if skipNext {
				skipNext = false
				continue
			}
			// Connection flags that take arguments
			if part == "-h" || part == "-p" || part == "-a" || part == "-n" || part == "--user" {
				skipNext = true
				continue
			}
			if strings.HasPrefix(part, "-") {
				continue
			}
			// Non-flag argument = Redis command
			hasCommand = true
			break
		}
		return !hasCommand
	}

	// Scripting REPLs: python, node, irb without script/command
	if firstWord == "python" || firstWord == "python3" || firstWord == "python2" {
		// Non-interactive if has -c or script file
		if strings.Contains(cmdLower, " -c ") || strings.Contains(cmdLower, " -c\"") {
			return false
		}
		// Check for script file (non-flag argument)
		parts := strings.Fields(cmdLower)
		for _, part := range parts[1:] {
			if !strings.HasPrefix(part, "-") && !strings.HasPrefix(part, "\"") {
				return false // has script file
			}
		}
		return true // bare python = REPL
	}

	if firstWord == "node" || firstWord == "nodejs" {
		// Non-interactive if has -e or script file
		if strings.Contains(cmdLower, " -e ") || strings.Contains(cmdLower, " -e\"") ||
			strings.Contains(cmdLower, " --eval") {
			return false
		}
		parts := strings.Fields(cmdLower)
		for _, part := range parts[1:] {
			if !strings.HasPrefix(part, "-") {
				return false // has script file
			}
		}
		return true
	}

	if firstWord == "irb" || firstWord == "pry" {
		// Ruby REPLs - almost always interactive
		// Non-interactive only with -e
		if strings.Contains(cmdLower, " -e ") {
			return false
		}
		return true
	}

	// openssl s_client is always interactive (waits for input)
	if strings.HasPrefix(cmdLower, "openssl s_client") || strings.HasPrefix(cmdLower, "openssl s_server") {
		return true
	}

	return false
}

// hasStdoutRedirect checks for dangerous output redirects while allowing safe stderr redirects.
func hasStdoutRedirect(command string) bool {
	if !strings.Contains(command, ">") {
		return false
	}
	// Remove safe stderr redirects before checking
	cmd := strings.ReplaceAll(command, "2>/dev/null", "")
	cmd = strings.ReplaceAll(cmd, "2>&1", "")
	return strings.Contains(cmd, ">")
}

// pipedToDualUseTool checks if a piped command sends input to a dual-use tool
// that could interpret piped input dangerously (like SQL CLIs).
// Piping to read-only filters (grep, head, tail, etc.) is safe.
func pipedToDualUseTool(cmdLower string) bool {
	// Find the last pipe (not ||)
	pipeIdx := -1
	for i := 0; i < len(cmdLower)-1; i++ {
		if cmdLower[i] == '|' && cmdLower[i+1] != '|' {
			pipeIdx = i
		}
	}
	if pipeIdx == -1 {
		return false
	}

	// Get the command after the last pipe
	afterPipe := strings.TrimSpace(cmdLower[pipeIdx+1:])

	// Dual-use tools that interpret piped input dangerously
	dualUseTools := []string{
		"sqlite3", "mysql", "mariadb", "psql", "mycli", "pgcli", "litecli",
		"redis-cli", "mongo", "mongosh",
		"sh ", "sh\t", "bash ", "bash\t", "zsh ", "zsh\t",
		"python", "perl", "ruby", "node",
		"xargs",
	}
	for _, tool := range dualUseTools {
		if strings.HasPrefix(afterPipe, tool) {
			return true
		}
	}

	return false
}

// checkSelfContainedGuardrails verifies the command is a single execution unit.
// Returns reason if any guardrail fails.
// Note: Most guardrails have been moved to checkMutationCapabilityGuards (Phase 1)
// to ensure they run before read-only-by-construction checks.
func checkSelfContainedGuardrails(command, cmdLower string) string {
	// Most checks are now in Phase 1 (checkMutationCapabilityGuards)
	// This phase is kept for potential future guardrails that should run
	// after write pattern matching.
	return ""
}

// isReadOnlyByConstruction returns true for commands that cannot mutate by design.
// Only matches patterns at the START of the command to avoid false positives
// (e.g., "date " inside "UPDATE" SQL statements).
func isReadOnlyByConstruction(cmdLower string) bool {
	// Note: Pager tools (less, more) and live monitors (top, htop) are excluded here
	// because they're blocked by NonInteractiveOnly guardrails in Phase 1.
	readOnlyCommands := []string{
		"cat", "head", "tail",
		"ls", "ll", "dir",
		"ps", "free", "df", "du",
		"grep", "awk", "sed", "find", "locate", "which", "whereis",
		"journalctl", "dmesg",
		"uname", "hostname", "whoami", "id", "groups",
		"date", "uptime", "env", "printenv", "locale",
		"netstat", "ss", "ifconfig", "route",
		"ping", "traceroute", "tracepath", "nslookup", "dig", "host",
		"file", "stat", "wc", "sort", "uniq", "cut", "tr",
		"lsof", "fuser",
		"getent", "nproc", "lscpu", "lsmem", "lsblk", "blkid",
		"zcat", "zgrep", "bzcat", "xzcat",
		"md5sum", "sha256sum", "sha1sum",
		"test",
		// Media inspection tools
		"ffprobe", "mediainfo", "exiftool",
	}

	// Multi-word patterns that must appear at the start
	multiWordPatterns := []string{
		"curl -s", "curl --silent", "curl -I", "curl --head",
		"wget -q", "wget --spider",
		"docker ps", "docker logs", "docker inspect", "docker stats", "docker images", "docker info",
		"systemctl status", "systemctl is-active", "systemctl is-enabled", "systemctl list", "systemctl show",
		"ip addr", "ip route", "ip link",
		// Kubectl read-only commands
		"kubectl get", "kubectl describe", "kubectl logs", "kubectl top", "kubectl cluster-info",
		"kubectl api-resources", "kubectl api-versions", "kubectl version", "kubectl config view",
		// Timeout wrapper (makes any command bounded)
		"timeout ",
	}

	// Extract first word of command
	firstWord := cmdLower
	if spaceIdx := strings.Index(cmdLower, " "); spaceIdx > 0 {
		firstWord = cmdLower[:spaceIdx]
	}

	// Check single-word commands
	for _, cmd := range readOnlyCommands {
		if firstWord == cmd {
			return true
		}
	}

	// Check multi-word patterns at start
	for _, pattern := range multiWordPatterns {
		if strings.HasPrefix(cmdLower, pattern) {
			return true
		}
	}

	// Special case: [ (test shorthand)
	if strings.HasPrefix(cmdLower, "[ ") {
		return true
	}

	return false
}

// matchesWritePatterns checks for known write-capable command patterns.
// Returns reason if a write pattern matches.
func matchesWritePatterns(cmdLower string) string {
	// High-risk patterns
	highRiskPatterns := map[string]string{
		"rm ": "file deletion", "rm\t": "file deletion", "rmdir": "directory deletion",
		"shutdown": "system shutdown", "reboot": "system reboot", "poweroff": "system poweroff", "halt": "system halt",
		"systemctl restart": "service restart", "systemctl stop": "service stop", "systemctl start": "service start",
		"systemctl enable": "service enable", "systemctl disable": "service disable",
		"service ": "service control", "init ": "init control",
		"apt ": "package management", "apt-get ": "package management", "yum ": "package management",
		"dnf ": "package management", "pacman ": "package management", "apk ": "package management", "brew ": "package management",
		"pip install": "package install", "pip uninstall": "package uninstall",
		"npm install": "package install", "npm uninstall": "package uninstall", "cargo install": "package install",
		"docker rm": "container removal", "docker stop": "container stop", "docker kill": "container kill",
		"docker restart": "container restart", "docker exec": "container exec",
		"kill ": "process termination", "killall ": "process termination", "pkill ": "process termination",
		"dd ": "disk write", "mkfs": "filesystem creation", "fdisk": "disk partition", "parted": "disk partition", "mkswap": "swap creation",
		"iptables": "firewall modification", "firewall-cmd": "firewall modification", "ufw ": "firewall modification",
		"truncate": "file truncation",
		"chmod ":   "permission change", "chown ": "ownership change", "chgrp ": "group change",
		"useradd": "user creation", "userdel": "user deletion", "usermod": "user modification",
		"passwd": "password change", "chpasswd": "password change",
		"crontab -e": "cron edit", "crontab -r": "cron removal", "crontab -": "cron modification",
		"visudo": "sudoers edit", "vipw": "passwd edit",
		"mount ": "filesystem mount", "umount ": "filesystem unmount",
		"modprobe": "kernel module", "rmmod": "kernel module removal", "insmod": "kernel module insertion",
		"sysctl -w": "kernel parameter change",
	}
	for pattern, reason := range highRiskPatterns {
		if strings.Contains(cmdLower, pattern) {
			return reason
		}
	}

	// Medium-risk patterns
	mediumRiskPatterns := map[string]string{
		"mv ": "file move", "cp ": "file copy",
		"sed -i": "in-place edit", "awk -i": "in-place edit",
		"touch ": "file creation", "mkdir ": "directory creation",
		"echo ": "output (may redirect)", "printf ": "output (may redirect)",
		"wget -O": "file download", "wget --output": "file download",
		"tar -x": "archive extraction", "tar x": "archive extraction", "unzip ": "archive extraction", "gunzip ": "archive extraction",
		"ln ": "link creation", "link ": "link creation",
	}
	for pattern, reason := range mediumRiskPatterns {
		if strings.Contains(cmdLower, pattern) {
			return reason
		}
	}

	// Curl with mutation verbs
	if strings.Contains(cmdLower, "curl") {
		if strings.Contains(cmdLower, "-d ") || strings.Contains(cmdLower, "--data") ||
			strings.Contains(cmdLower, "--upload") ||
			strings.Contains(cmdLower, "-X POST") || strings.Contains(cmdLower, "-X PUT") ||
			strings.Contains(cmdLower, "-X DELETE") || strings.Contains(cmdLower, "-X PATCH") {
			return "HTTP mutation request"
		}
	}

	return ""
}

// hasShellChainingOutsideQuotes checks if a command contains shell chaining operators
// (;, &&, ||) outside of quoted strings. This allows SQL statements like "SELECT 1;"
// while still catching shell command chaining like "ls; rm -rf /".
//
// Handles escaped quotes (\' and \") by skipping the escaped character.
// Fails closed: if quote state becomes ambiguous (unclosed quotes), returns true.
func hasShellChainingOutsideQuotes(cmd string) bool {
	inSingleQuote := false
	inDoubleQuote := false

	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]

		// Handle escape sequences: skip the next character
		// This prevents \" or \' from toggling quote state
		if ch == '\\' && i+1 < len(cmd) {
			i++ // Skip the escaped character
			continue
		}

		// Track quote state
		switch ch {
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			}
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			}
		case ';':
			if !inSingleQuote && !inDoubleQuote {
				return true
			}
		case '&':
			// Check for && (need to look at next char)
			if !inSingleQuote && !inDoubleQuote && i+1 < len(cmd) && cmd[i+1] == '&' {
				return true
			}
		case '|':
			// Check for || (need to look at next char)
			// Note: single | is a pipe, which is allowed for read operations
			if !inSingleQuote && !inDoubleQuote && i+1 < len(cmd) && cmd[i+1] == '|' {
				return true
			}
		}
	}

	// Fail closed: if quotes are unclosed, treat as potentially dangerous
	// (ambiguous state means we can't be sure chaining operators are inside quotes)
	if inSingleQuote || inDoubleQuote {
		return true
	}

	return false
}

// classifyCommandRisk provides backward-compatible risk classification.
// It delegates to ClassifyExecutionIntent and maps the result to CommandRisk,
// preserving the High/Medium write distinction for existing callers.
//
// Deprecated: Use ClassifyExecutionIntent for new code.
func classifyCommandRisk(command string) CommandRisk {
	result := ClassifyExecutionIntent(command)
	switch result.Intent {
	case IntentReadOnlyCertain, IntentReadOnlyConditional:
		return CommandRiskReadOnly
	default:
		// For backward compatibility, distinguish HighWrite from MediumWrite
		// using the same pattern checks from matchesWritePatterns
		return classifyWriteRiskLevel(command, result.Reason)
	}
}

// classifyWriteRiskLevel determines whether a write command is high or medium risk.
// Used by classifyCommandRisk for backward compatibility.
func classifyWriteRiskLevel(command, reason string) CommandRisk {
	cmdLower := strings.ToLower(command)

	// High-risk: destructive system operations
	highRiskPatterns := []string{
		// Shell mutation capabilities (these dominate everything)
		"> ", ">>", "| tee ",
		// Destructive file operations
		"rm ", "rm\t", "rmdir",
		// System control
		"shutdown", "reboot", "poweroff", "halt",
		// Service control (except status)
		"systemctl restart", "systemctl stop", "systemctl start",
		"systemctl enable", "systemctl disable",
		"service ", "init ",
		// Package managers
		"apt ", "apt-get ", "yum ", "dnf ", "pacman ", "apk ", "brew ",
		"pip install", "pip uninstall", "npm install", "npm uninstall", "cargo install",
		// Container destruction
		"docker rm", "docker stop", "docker kill", "docker restart",
		// Process termination
		"kill ", "killall ", "pkill ",
		// Disk operations
		"dd ", "mkfs", "fdisk", "parted", "mkswap",
		// Firewall
		"iptables", "firewall-cmd", "ufw ",
		// File truncation
		"truncate",
		// Permissions/ownership
		"chmod ", "chown ", "chgrp ",
		// User management
		"useradd", "userdel", "usermod", "passwd", "chpasswd",
		// Cron/sudoers
		"crontab -", "visudo", "vipw",
		// Mounts and kernel
		"mount ", "umount ", "modprobe", "rmmod", "insmod", "sysctl -w",
		// sudo escalation
		"sudo",
	}

	for _, pattern := range highRiskPatterns {
		if strings.Contains(cmdLower, pattern) {
			return CommandRiskHighWrite
		}
	}

	// Everything else is medium-risk
	return CommandRiskMediumWrite
}

// GetReadOnlyViolationHint returns a hint for why a command was blocked from pulse_read.
// Uses the IntentResult reason plus context-aware suggestions.
func GetReadOnlyViolationHint(command string, result IntentResult) string {
	baseHint := result.Reason
	cmdLower := strings.ToLower(command)

	// Phase 1 guardrail hints (structural issues that must be removed)
	if isPhase1GuardrailFailure(result.Reason) {
		return getPhase1Hint(result.Reason, baseHint)
	}

	// Content inspection hints (SQL CLIs, etc.)
	isSQLCLI := strings.Contains(cmdLower, "sqlite3") ||
		strings.Contains(cmdLower, "mysql") ||
		strings.Contains(cmdLower, "mariadb") ||
		strings.Contains(cmdLower, "psql")

	if isSQLCLI {
		return getSQLHint(result.Reason, baseHint)
	}

	// Unknown command fallback hint
	if strings.Contains(result.Reason, "unknown") || strings.Contains(result.Reason, "no inspector") {
		return baseHint + ". Try a self-contained form: no pipes, no redirects, single statement. If this is a read-only operation, consider using a known read-only command instead."
	}

	return baseHint
}

// isPhase1GuardrailFailure returns true if the reason indicates a Phase 1 structural issue.
func isPhase1GuardrailFailure(reason string) bool {
	guardrailKeywords := []string{
		"sudo", "redirect", "tee", "substitution", "chaining", "piped input",
		// NonInteractiveOnly guardrails
		"TTY", "terminal", "pager", "editor", "indefinitely", "unbounded", "streaming",
	}
	for _, kw := range guardrailKeywords {
		if strings.Contains(reason, kw) {
			return true
		}
	}
	return false
}

// getPhase1Hint returns actionable hints for Phase 1 guardrail failures.
func getPhase1Hint(reason, baseHint string) string {
	switch {
	case strings.Contains(reason, "sudo"):
		return baseHint + ". Remove sudo to use pulse_read; use pulse_control for privileged operations."
	case strings.Contains(reason, "redirect"):
		return baseHint + ". Remove redirects (>, >>, <, <<, <<<) to use pulse_read."
	case strings.Contains(reason, "tee"):
		return baseHint + ". Remove tee to use pulse_read; tee writes to files."
	case strings.Contains(reason, "substitution"):
		return baseHint + ". Remove $() or backticks to use pulse_read."
	case strings.Contains(reason, "chaining"):
		return baseHint + ". Run commands separately instead of chaining with ; && ||."
	case strings.Contains(reason, "piped input"):
		return baseHint + ". For dual-use tools, include content directly instead of piping. Example: sqlite3 db.db \"SELECT ...\" instead of cat file | sqlite3 db.db"
	// NonInteractiveOnly hints
	case strings.Contains(reason, "TTY") || strings.Contains(reason, "terminal"):
		return baseHint + ". Remove -it/--tty/--interactive flags. Use non-interactive form: docker exec container cmd (not docker exec -it)."
	case strings.Contains(reason, "pager") || strings.Contains(reason, "editor"):
		return baseHint + ". Use cat, head -n, or tail -n instead of interactive tools."
	case strings.Contains(reason, "indefinitely"):
		return baseHint + ". Use bounded alternatives: ps aux (not top), journalctl -n 100 (not watch)."
	case strings.Contains(reason, "unbounded") || strings.Contains(reason, "streaming"):
		return baseHint + ". Add line limit: journalctl -n 100 -f or tail -n 50 -f, or wrap with timeout."
	default:
		return baseHint + ". Remove redirects, chaining, sudo, or subshells to use pulse_read."
	}
}

// getSQLHint returns actionable hints for SQL CLI content inspection failures.
func getSQLHint(reason, baseHint string) string {
	switch {
	case strings.Contains(reason, "external") || strings.Contains(reason, "no inline"):
		return baseHint + ". Include SQL directly in quotes: sqlite3 db.db \"SELECT ...\""
	case strings.Contains(reason, "write") || strings.Contains(reason, "control"):
		return baseHint + ". Use only SELECT statements. Avoid: INSERT, UPDATE, DELETE, DROP, CREATE, PRAGMA, BEGIN, COMMIT, ROLLBACK."
	default:
		return baseHint + ". For read-only queries, use self-contained SELECT statements without transaction control."
	}
}

const (
	defaultMaxTopologyNodes                   = 5
	defaultMaxTopologyVMsPerNode              = 5
	defaultMaxTopologyContainersPerNode       = 5
	defaultMaxTopologyDockerHosts             = 3
	defaultMaxTopologyDockerContainersPerHost = 5
	defaultMaxListDockerContainersPerHost     = 10
)

// buildResourceID creates a canonical resource ID.
// Prefers kind:provider_uid when UID is available, falls back to kind:name.
func buildResourceID(kind, name, providerUID string) string {
	if providerUID != "" {
		return kind + ":" + providerUID
	}
	return kind + ":" + name
}

// buildDisplayPath creates a human-readable location path.
// e.g., "docker:jellyfin @ lxc:media-server @ node:delly"
func buildDisplayPath(locationChain []string) string {
	if len(locationChain) == 0 {
		return ""
	}
	// Reverse for display (innermost first)
	reversed := make([]string, len(locationChain))
	for i, loc := range locationChain {
		reversed[len(locationChain)-1-i] = loc
	}
	return strings.Join(reversed, " @ ")
}

// registerResolvedResource adds a discovered resource to the resolved context if available.
// This is called by query tools when they find resources, enabling action tools to validate
// that commands are targeting legitimate, discovered resources.
//
// NOTE: This does NOT mark the resource as "recently accessed" for routing validation.
// Use registerResolvedResourceWithExplicitAccess() for single-resource operations where
// user intent is clear.
func (e *PulseToolExecutor) registerResolvedResource(reg ResourceRegistration) {
	if e.resolvedContext == nil {
		return
	}
	e.resolvedContext.AddResolvedResource(reg)
}

// registerResolvedResourceWithExplicitAccess adds a resource AND marks it as recently accessed.
// Use this for single-resource operations (pulse_query get, explicit select) where user
// intent to target this specific resource is clear.
//
// DO NOT use this for bulk operations (list, search) that return many resources,
// as it would poison routing validation and cause false ROUTING_MISMATCH blocks.
func (e *PulseToolExecutor) registerResolvedResourceWithExplicitAccess(reg ResourceRegistration) {
	if e.resolvedContext == nil {
		return
	}
	e.resolvedContext.AddResolvedResource(reg)

	// Build the resource ID to mark explicit access (must match AddResolvedResource format)
	// Format: {kind}:{host}:{provider_uid} for scoped resources
	//         {kind}:{provider_uid} for global resources
	var resourceID string
	if reg.ProviderUID != "" {
		if reg.HostUID != "" || reg.HostName != "" {
			hostScope := reg.HostUID
			if hostScope == "" {
				hostScope = reg.HostName
			}
			resourceID = reg.Kind + ":" + hostScope + ":" + reg.ProviderUID
		} else {
			resourceID = reg.Kind + ":" + reg.ProviderUID
		}
	} else {
		if reg.HostUID != "" || reg.HostName != "" {
			hostScope := reg.HostUID
			if hostScope == "" {
				hostScope = reg.HostName
			}
			resourceID = reg.Kind + ":" + hostScope + ":" + reg.Name
		} else {
			resourceID = reg.Kind + ":" + reg.Name
		}
	}
	e.resolvedContext.MarkExplicitAccess(resourceID)
}

// ValidationResult holds the result of resource validation.
// Check StrictError first using errors.As() for typed error handling.
type ValidationResult struct {
	Resource    ResolvedResourceInfo
	ErrorMsg    string               // Human-readable error (backwards compat)
	StrictError *ErrStrictResolution // Typed error for strict mode violations
}

// IsBlocked returns true if the validation blocked the operation
func (v *ValidationResult) IsBlocked() bool {
	return v.StrictError != nil
}

// validateResolvedResource checks if a resource has been previously discovered via query/discovery tools.
// Returns a ValidationResult containing:
//   - Resource: the resolved resource info if found
//   - ErrorMsg: human-readable error message (empty if valid)
//   - StrictError: typed error for strict mode violations (nil if not blocked)
//
// Setting skipIfNoContext=true makes validation optional (for backwards compatibility).
//
// When PULSE_STRICT_RESOLUTION=true is set, write operations (start, stop, restart, delete, exec)
// will be blocked if the resource wasn't discovered first. This prevents the AI from operating
// on fabricated or hallucinated resource IDs.
func (e *PulseToolExecutor) validateResolvedResource(resourceName, action string, skipIfNoContext bool) ValidationResult {
	// Determine if this requires hard validation (strict mode + write action)
	strictMode := isStrictResolutionEnabled()
	isWrite := isWriteAction(action)
	requireHardValidation := strictMode && isWrite

	if e.resolvedContext == nil {
		if requireHardValidation {
			// Record telemetry for strict resolution block
			if e.telemetryCallback != nil {
				e.telemetryCallback.RecordStrictResolutionBlock("validateResolvedResource", action)
			}
			err := &ErrStrictResolution{
				ResourceID: resourceName,
				Action:     action,
				Message:    fmt.Sprintf("Resource '%s' has not been discovered. Use pulse_query to find resources before performing '%s' action.", resourceName, action),
			}
			return ValidationResult{
				ErrorMsg:    err.Message,
				StrictError: err,
			}
		}
		if skipIfNoContext {
			return ValidationResult{}
		}
		return ValidationResult{
			ErrorMsg: fmt.Sprintf("Resource '%s' has not been discovered. Use pulse_query to find resources first.", resourceName),
		}
	}

	// First, try to find by alias (most common case - user refers to resources by name)
	res, found := e.resolvedContext.GetResolvedResourceByAlias(resourceName)
	if found {
		// Check if action is allowed
		allowedActions := res.GetAllowedActions()
		if len(allowedActions) > 0 {
			actionAllowed := false
			for _, allowed := range allowedActions {
				if allowed == action || allowed == "*" {
					actionAllowed = true
					break
				}
			}
			if !actionAllowed {
				return ValidationResult{
					Resource: res,
					ErrorMsg: fmt.Sprintf("Action '%s' is not permitted for resource '%s'. Allowed actions: %v", action, resourceName, allowedActions),
				}
			}
		}
		return ValidationResult{Resource: res}
	}

	// Try direct ID lookup (for when caller passes canonical ID)
	res, found = e.resolvedContext.GetResolvedResourceByID(resourceName)
	if found {
		// Same action validation
		allowedActions := res.GetAllowedActions()
		if len(allowedActions) > 0 {
			actionAllowed := false
			for _, allowed := range allowedActions {
				if allowed == action || allowed == "*" {
					actionAllowed = true
					break
				}
			}
			if !actionAllowed {
				return ValidationResult{
					Resource: res,
					ErrorMsg: fmt.Sprintf("Action '%s' is not permitted for resource '%s'. Allowed actions: %v", action, resourceName, allowedActions),
				}
			}
		}
		return ValidationResult{Resource: res}
	}

	// Resource not found
	if requireHardValidation {
		// Record telemetry for strict resolution block
		if e.telemetryCallback != nil {
			e.telemetryCallback.RecordStrictResolutionBlock("validateResolvedResource", action)
		}
		err := &ErrStrictResolution{
			ResourceID: resourceName,
			Action:     action,
			Message:    fmt.Sprintf("Resource '%s' has not been discovered in this session. Use pulse_query action=search to find it before performing '%s' action.", resourceName, action),
		}
		return ValidationResult{
			ErrorMsg:    err.Message,
			StrictError: err,
		}
	}

	// Allow operation if skipIfNoContext (backwards compat for soft validation)
	if skipIfNoContext {
		return ValidationResult{}
	}

	return ValidationResult{
		ErrorMsg: fmt.Sprintf("Resource '%s' has not been discovered in this session. Use pulse_query action=search to find it first.", resourceName),
	}
}

// validateResolvedResourceForExec validates a resource for command execution.
// It uses command risk classification to determine if strict validation applies.
//
// Behavior in strict mode (PULSE_STRICT_RESOLUTION=true):
//   - Read-only commands are allowed IF the session has ANY resolved context
//     (prevents arbitrary host guessing while allowing diagnostic commands)
//   - Write commands require the specific resource to be discovered first
//
// Behavior in normal mode:
//   - All commands are allowed with soft validation (warning logs)
func (e *PulseToolExecutor) validateResolvedResourceForExec(resourceName, command string, skipIfNoContext bool) ValidationResult {
	// Classify the command risk
	risk := classifyCommandRisk(command)

	// For read-only commands in strict mode, allow if session has ANY resolved context
	// This prevents arbitrary host guessing while still allowing diagnostic commands
	// on hosts that have been discovered (even if not the specific resource)
	if risk == CommandRiskReadOnly && isStrictResolutionEnabled() {
		// Check if there's any resolved context at all
		if e.resolvedContext != nil {
			// Try to find the resource - if found, great
			result := e.validateResolvedResource(resourceName, "query", true)
			if result.Resource != nil {
				return result
			}

			// Resource not found, but we have some context - check if ANY host is discovered
			// This is a scoped bypass: read-only commands allowed only if session is "active"
			// (i.e., user has already done some discovery)
			if e.hasAnyResolvedHost() {
				// Allow read-only command with warning
				return ValidationResult{
					ErrorMsg: fmt.Sprintf("Resource '%s' not explicitly discovered, but allowing read-only command due to existing session context", resourceName),
				}
			}
		}
		// No context at all - require discovery even for read-only in strict mode
		// Record telemetry for strict resolution block
		if e.telemetryCallback != nil {
			e.telemetryCallback.RecordStrictResolutionBlock("validateResolvedResourceForExec", "exec (read-only)")
		}
		return ValidationResult{
			ErrorMsg: "No resources discovered in this session. Use pulse_query to discover resources first.",
			StrictError: &ErrStrictResolution{
				ResourceID: resourceName,
				Action:     "exec (read-only)",
				Message:    fmt.Sprintf("Resource '%s' cannot be accessed. No resources have been discovered in this session. Use pulse_query action=search to discover available resources.", resourceName),
			},
		}
	}

	// For read-only commands in non-strict mode, use soft validation
	if risk == CommandRiskReadOnly {
		return e.validateResolvedResource(resourceName, "query", skipIfNoContext)
	}

	// For write commands, use "exec" action which triggers strict validation
	return e.validateResolvedResource(resourceName, "exec", skipIfNoContext)
}

// hasAnyResolvedHost checks if there's at least one discovered resource in the session.
// This is used to scope read-only exec bypass - if the user has discovered ANY resource,
// we allow read-only commands to other resources (with warnings).
func (e *PulseToolExecutor) hasAnyResolvedHost() bool {
	if e.resolvedContext == nil {
		return false
	}
	return e.resolvedContext.HasAnyResources()
}

// RoutingValidationResult holds the result of routing context validation.
type RoutingValidationResult struct {
	RoutingError *ErrRoutingMismatch // Non-nil if routing mismatch detected
}

// IsBlocked returns true if routing validation blocked the operation
func (r *RoutingValidationResult) IsBlocked() bool {
	return r.RoutingError != nil
}

// validateRoutingContext checks if a target_host should be a more specific resource.
//
// This validation prevents the model from accidentally operating on a parent Proxmox host
// when the user clearly intends to target a child resource (LXC/VM) on that host.
//
// IMPORTANT: This check is intentionally scoped to RECENTLY ACCESSED resources to avoid
// false positives. The logic is:
//
//   - If target_host resolves directly to a resource in ResolvedContext → OK
//   - If target_host is a Proxmox node AND the user RECENTLY referenced child resources
//     on that node (within RecentAccessWindow) → block with ROUTING_MISMATCH
//
// This prevents blocking legitimate host-level operations like "apt update on @delly"
// while still catching the "user said @homepage-docker but model targets delly" scenario.
//
// The key insight: if the user explicitly mentioned a child resource in this turn/exchange,
// they probably intend to target that child, not the parent host.
func (e *PulseToolExecutor) validateRoutingContext(targetHost string) RoutingValidationResult {
	// Skip if no state provider or resolved context
	if e.stateProvider == nil || e.resolvedContext == nil {
		return RoutingValidationResult{}
	}

	// First, check if targetHost resolves directly to a resource in ResolvedContext
	// If so, no routing mismatch - user is targeting the right thing
	if res, found := e.resolvedContext.GetResolvedResourceByAlias(targetHost); found {
		// Target matches a resolved resource directly - no mismatch
		_ = res
		return RoutingValidationResult{}
	}

	// Check if targetHost is a Proxmox node (host)
	state := e.stateProvider.GetState()
	loc := state.ResolveResource(targetHost)

	// Only check for mismatch if targetHost is a Proxmox node (host type)
	if !loc.Found || loc.ResourceType != "node" {
		return RoutingValidationResult{}
	}

	// targetHost is a Proxmox node. Check if ResolvedContext has RECENTLY ACCESSED
	// child resources on this node (within the recent access window).
	// This is the key refinement: we only block if the user recently referenced
	// a child resource, implying they intended to target that child.
	recentChildren := e.findRecentlyReferencedChildrenOnNode(loc.Node)
	if len(recentChildren) == 0 {
		return RoutingValidationResult{}
	}

	// Extract names, IDs, and kinds for the error response
	var childNames []string
	var childIDs []string
	var childKinds []string
	for _, child := range recentChildren {
		childNames = append(childNames, child.Name)
		childIDs = append(childIDs, child.ResourceID)
		childKinds = append(childKinds, child.Kind)
	}

	// Record telemetry for routing mismatch block
	// Use the first child kind for the label (we use small enums to avoid cardinality issues)
	if e.telemetryCallback != nil && len(childKinds) > 0 {
		e.telemetryCallback.RecordRoutingMismatchBlock("routing_validation", "node", childKinds[0])
	}

	// Rate-limited debug logging for support/debugging
	// Logs: target_kind, child_kind, suggested_resource_id (no user paths to avoid cardinality)
	logRoutingMismatchDebug(targetHost, childKinds, childIDs)

	// Found recently referenced child resources! Block with ROUTING_MISMATCH
	return RoutingValidationResult{
		RoutingError: &ErrRoutingMismatch{
			TargetHost:            targetHost,
			MoreSpecificResources: childNames,
			MoreSpecificIDs:       childIDs,
			ChildKinds:            childKinds,
			Message: fmt.Sprintf(
				"target_host '%s' is a Proxmox node, but you recently referenced more specific resources on it: %v. "+
					"Did you mean to target one of these instead? File operations on a Proxmox host do NOT affect files inside LXC/VM guests.",
				targetHost, childNames),
		},
	}
}

// recentChildInfo holds both the name and canonical ID of a recently referenced child resource.
type recentChildInfo struct {
	Name       string // Human-readable name
	ResourceID string // Canonical ID (kind:host:id)
	Kind       string // Resource kind (lxc, vm, docker_container) for telemetry
}

// findRecentlyReferencedChildrenOnNode returns the names and IDs of LXC/VM resources on a
// specific Proxmox node that were RECENTLY ACCESSED (within RecentAccessWindow).
//
// This is used by validateRoutingContext to detect when the user referenced a child resource
// in the current turn/exchange, indicating they probably intended to target that child.
func (e *PulseToolExecutor) findRecentlyReferencedChildrenOnNode(nodeName string) []recentChildInfo {
	if e.resolvedContext == nil || e.stateProvider == nil {
		return nil
	}

	var children []recentChildInfo
	state := e.stateProvider.GetState()

	// Check LXC containers on this node
	for _, ct := range state.Containers {
		if ct.Node != nodeName {
			continue
		}
		// Check if this LXC is in the resolved context AND was recently accessed
		if res, found := e.resolvedContext.GetResolvedResourceByAlias(ct.Name); found {
			if res.GetKind() == "lxc" {
				// Check if this resource was recently accessed
				resourceID := res.GetResourceID()
				if e.resolvedContext.WasRecentlyAccessed(resourceID, RecentAccessWindow) {
					children = append(children, recentChildInfo{
						Name:       ct.Name,
						ResourceID: resourceID,
						Kind:       "lxc",
					})
				}
			}
		}
	}

	// Check VMs on this node
	for _, vm := range state.VMs {
		if vm.Node != nodeName {
			continue
		}
		// Check if this VM is in the resolved context AND was recently accessed
		if res, found := e.resolvedContext.GetResolvedResourceByAlias(vm.Name); found {
			if res.GetKind() == "vm" {
				// Check if this resource was recently accessed
				resourceID := res.GetResourceID()
				if e.resolvedContext.WasRecentlyAccessed(resourceID, RecentAccessWindow) {
					children = append(children, recentChildInfo{
						Name:       vm.Name,
						ResourceID: resourceID,
						Kind:       "vm",
					})
				}
			}
		}
	}

	return children
}

// logRoutingMismatchDebug logs routing mismatch details for debugging and support.
// Rate-limited to avoid log spam (at most once per 10 seconds).
// Only logs safe, low-cardinality fields: target_kind, child_kind, suggested_resource_id.
func logRoutingMismatchDebug(targetHost string, childKinds, childIDs []string) {
	routingMismatchLogLimiter.mu.Lock()
	defer routingMismatchLogLimiter.mu.Unlock()

	if time.Since(routingMismatchLogLimiter.lastLog) < routingMismatchLogLimiter.interval {
		return // Rate limited
	}
	routingMismatchLogLimiter.lastLog = time.Now()

	// Get first child kind and ID for logging (safe, low cardinality)
	childKind := "unknown"
	suggestedID := "none"
	if len(childKinds) > 0 {
		childKind = childKinds[0]
	}
	if len(childIDs) > 0 {
		suggestedID = childIDs[0]
	}

	log.Debug().
		Str("event", "routing_mismatch_block").
		Str("target_kind", "node").
		Str("child_kind", childKind).
		Str("suggested_resource_id", suggestedID).
		Int("affected_children", len(childIDs)).
		Msg("[RoutingValidation] Blocked operation targeting parent node when child recently referenced")
}

// registerQueryTools registers the consolidated pulse_query tool
func (e *PulseToolExecutor) registerQueryTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_query",
			Description: `Query and search infrastructure resources. Start here to find resources by name.

Actions:
- search: Find resources by name, type, or status. Use this first when looking for a specific service/container/VM by name.
- get: Get detailed info about a specific resource (CPU, memory, status, host)
- config: Get VM/LXC configuration (disk, network, resources)
- topology: Get hierarchical infrastructure view
- list: List all infrastructure (lightweight overview)
- health: Check connection health for instances

When investigating applications (e.g., "check Jellyfin logs"):
1. Use action="search" with query="jellyfin" to find where it runs
2. Use pulse_discovery to get deep context (log paths, config locations)
3. Use pulse_control type="command" to run investigative commands

Examples:
- Find a service: action="search", query="jellyfin"
- Get resource details: action="get", resource_type="docker", resource_id="nginx"
- List running VMs: action="list", type="vms", status="running"`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"action": {
						Type:        "string",
						Description: "Query action to perform",
						Enum:        []string{"search", "get", "config", "topology", "list", "health"},
					},
					"query": {
						Type:        "string",
						Description: "Search query (for action: search)",
					},
					"resource_type": {
						Type:        "string",
						Description: "Resource type: 'vm', 'container', 'docker', 'node' (for action: get, config, search)",
						Enum:        []string{"vm", "container", "docker", "node"},
					},
					"resource_id": {
						Type:        "string",
						Description: "Resource identifier (VMID or name) (for action: get, config)",
					},
					"type": {
						Type:        "string",
						Description: "Filter by type (for action: list): 'nodes', 'vms', 'containers', 'docker'",
						Enum:        []string{"nodes", "vms", "containers", "docker"},
					},
					"status": {
						Type:        "string",
						Description: "Filter by status (for action: search, list)",
					},
					"include": {
						Type:        "string",
						Description: "Include filter for topology: 'all', 'proxmox', 'docker' (for action: topology)",
						Enum:        []string{"all", "proxmox", "docker"},
					},
					"summary_only": {
						Type:        "boolean",
						Description: "Return only summary counts for topology (for action: topology)",
					},
					"max_nodes": {
						Type:        "integer",
						Description: "Max Proxmox nodes to include (for action: topology)",
					},
					"max_vms_per_node": {
						Type:        "integer",
						Description: "Max VMs per node (for action: topology)",
					},
					"max_containers_per_node": {
						Type:        "integer",
						Description: "Max containers per node (for action: topology)",
					},
					"max_docker_hosts": {
						Type:        "integer",
						Description: "Max Docker hosts (for action: topology)",
					},
					"max_docker_containers_per_host": {
						Type:        "integer",
						Description: "Max Docker containers per host (for action: topology, list)",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results (default: 100)",
					},
					"offset": {
						Type:        "integer",
						Description: "Number of results to skip",
					},
				},
				Required: []string{"action"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeQuery(ctx, args)
		},
	})
}

// executeQuery routes to the appropriate query handler based on action
func (e *PulseToolExecutor) executeQuery(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	action, _ := args["action"].(string)
	switch action {
	case "search":
		return e.executeSearchResources(ctx, args)
	case "get":
		return e.executeGetResource(ctx, args)
	case "config":
		return e.executeGetGuestConfig(ctx, args)
	case "topology":
		return e.executeGetTopology(ctx, args)
	case "list":
		return e.executeListInfrastructure(ctx, args)
	case "health":
		return e.executeGetConnectionHealth(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("unknown action: %s. Use: search, get, config, topology, list, health", action)), nil
	}
}

func (e *PulseToolExecutor) executeListInfrastructure(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewErrorResult(fmt.Errorf("state provider not available")), nil
	}

	filterType, _ := args["type"].(string)
	filterStatus, _ := args["status"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)
	maxDockerContainersPerHost := intArg(args, "max_docker_containers_per_host", 0)
	if _, ok := args["max_docker_containers_per_host"]; !ok {
		maxDockerContainersPerHost = defaultMaxListDockerContainersPerHost
	}
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	state := e.stateProvider.GetState()

	// Build a set of connected agent hostnames for quick lookup
	connectedAgentHostnames := make(map[string]bool)
	if e.agentServer != nil {
		for _, agent := range e.agentServer.GetConnectedAgents() {
			connectedAgentHostnames[agent.Hostname] = true
		}
	}

	response := InfrastructureResponse{
		Total: TotalCounts{
			Nodes:       len(state.Nodes),
			VMs:         len(state.VMs),
			Containers:  len(state.Containers),
			DockerHosts: len(state.DockerHosts),
		},
	}

	totalMatches := 0

	// Nodes
	if filterType == "" || filterType == "nodes" {
		count := 0
		for _, node := range state.Nodes {
			if filterStatus != "" && filterStatus != "all" && node.Status != filterStatus {
				continue
			}
			if count < offset {
				count++
				continue
			}
			if len(response.Nodes) >= limit {
				count++
				continue
			}
			response.Nodes = append(response.Nodes, NodeSummary{
				Name:           node.Name,
				Status:         node.Status,
				ID:             node.ID,
				AgentConnected: connectedAgentHostnames[node.Name],
			})
			count++
		}
		if filterType == "nodes" {
			totalMatches = count
		}
	}

	// VMs
	if filterType == "" || filterType == "vms" {
		count := 0
		for _, vm := range state.VMs {
			if filterStatus != "" && filterStatus != "all" && vm.Status != filterStatus {
				continue
			}
			if count < offset {
				count++
				continue
			}
			if len(response.VMs) >= limit {
				count++
				continue
			}
			response.VMs = append(response.VMs, VMSummary{
				VMID:   vm.VMID,
				Name:   vm.Name,
				Status: vm.Status,
				Node:   vm.Node,
				CPU:    vm.CPU * 100,
				Memory: vm.Memory.Usage * 100,
			})
			count++
		}
		if filterType == "vms" {
			totalMatches = count
		}
	}

	// Containers (LXC)
	if filterType == "" || filterType == "containers" {
		count := 0
		for _, ct := range state.Containers {
			if filterStatus != "" && filterStatus != "all" && ct.Status != filterStatus {
				continue
			}
			if count < offset {
				count++
				continue
			}
			if len(response.Containers) >= limit {
				count++
				continue
			}
			response.Containers = append(response.Containers, ContainerSummary{
				VMID:   ct.VMID,
				Name:   ct.Name,
				Status: ct.Status,
				Node:   ct.Node,
				CPU:    ct.CPU * 100,
				Memory: ct.Memory.Usage * 100,
			})
			count++
		}
		if filterType == "containers" {
			totalMatches = count
		}
	}

	// Docker hosts
	if filterType == "" || filterType == "docker" {
		count := 0
		for _, host := range state.DockerHosts {
			if count < offset {
				count++
				continue
			}
			if len(response.DockerHosts) >= limit {
				count++
				continue
			}
			dockerHost := DockerHostSummary{
				ID:             host.ID,
				Hostname:       host.Hostname,
				DisplayName:    host.DisplayName,
				ContainerCount: len(host.Containers),
				AgentConnected: connectedAgentHostnames[host.Hostname],
			}
			for _, c := range host.Containers {
				if filterStatus != "" && filterStatus != "all" && c.State != filterStatus {
					continue
				}
				if maxDockerContainersPerHost > 0 && len(dockerHost.Containers) >= maxDockerContainersPerHost {
					continue
				}
				dockerHost.Containers = append(dockerHost.Containers, DockerContainerSummary{
					ID:     c.ID,
					Name:   c.Name,
					State:  c.State,
					Image:  c.Image,
					Health: c.Health,
				})
			}
			if filterStatus != "" && filterStatus != "all" && len(dockerHost.Containers) == 0 {
				continue
			}
			response.DockerHosts = append(response.DockerHosts, dockerHost)
			count++
		}
		if filterType == "docker" {
			totalMatches = count
		}
	}

	if filterType != "" && (offset > 0 || totalMatches > limit) {
		response.Pagination = &PaginationInfo{
			Total:  totalMatches,
			Limit:  limit,
			Offset: offset,
		}
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeGetTopology(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewErrorResult(fmt.Errorf("state provider not available")), nil
	}

	include, _ := args["include"].(string)
	if include == "" {
		include = "all"
	}
	switch include {
	case "all", "proxmox", "docker":
	default:
		return NewErrorResult(fmt.Errorf("invalid include: %s. Use all, proxmox, or docker", include)), nil
	}

	summaryOnly, summaryProvided := args["summary_only"].(bool)
	maxNodes := intArg(args, "max_nodes", 0)
	maxVMsPerNode := intArg(args, "max_vms_per_node", 0)
	maxContainersPerNode := intArg(args, "max_containers_per_node", 0)
	maxDockerHosts := intArg(args, "max_docker_hosts", 0)
	maxDockerContainersPerHost := intArg(args, "max_docker_containers_per_host", 0)
	_, maxNodesProvided := args["max_nodes"]
	_, maxVMsProvided := args["max_vms_per_node"]
	_, maxContainersProvided := args["max_containers_per_node"]
	_, maxDockerHostsProvided := args["max_docker_hosts"]
	_, maxDockerContainersProvided := args["max_docker_containers_per_host"]

	if !summaryProvided {
		summaryOnly = false
	}
	if !summaryOnly {
		if !maxNodesProvided {
			maxNodes = defaultMaxTopologyNodes
		}
		if !maxVMsProvided {
			maxVMsPerNode = defaultMaxTopologyVMsPerNode
		}
		if !maxContainersProvided {
			maxContainersPerNode = defaultMaxTopologyContainersPerNode
		}
		if !maxDockerHostsProvided {
			maxDockerHosts = defaultMaxTopologyDockerHosts
		}
		if !maxDockerContainersProvided {
			maxDockerContainersPerHost = defaultMaxTopologyDockerContainersPerHost
		}
	}

	state := e.stateProvider.GetState()

	// Build a set of connected agent hostnames for quick lookup
	connectedAgentHostnames := make(map[string]bool)
	if e.agentServer != nil {
		for _, agent := range e.agentServer.GetConnectedAgents() {
			connectedAgentHostnames[agent.Hostname] = true
		}
	}

	// Check if control is enabled
	controlEnabled := e.controlLevel != ControlLevelReadOnly && e.controlLevel != ""

	includeProxmox := include == "all" || include == "proxmox"
	includeDocker := include == "all" || include == "docker"

	// Summary counters
	summary := TopologySummary{
		TotalNodes:         len(state.Nodes),
		TotalVMs:           len(state.VMs),
		TotalLXCContainers: len(state.Containers),
		TotalDockerHosts:   len(state.DockerHosts),
	}

	for _, node := range state.Nodes {
		if connectedAgentHostnames[node.Name] {
			summary.NodesWithAgents++
		}
	}
	for _, host := range state.DockerHosts {
		if connectedAgentHostnames[host.Hostname] {
			summary.DockerHostsWithAgents++
		}
	}

	// Build Proxmox topology - group VMs and containers by node
	nodeMap := make(map[string]*ProxmoxNodeTopology)
	if includeProxmox && !summaryOnly {
		for _, node := range state.Nodes {
			if maxNodes > 0 && len(nodeMap) >= maxNodes {
				break
			}
			hasAgent := connectedAgentHostnames[node.Name]
			nodeMap[node.Name] = &ProxmoxNodeTopology{
				Name:           node.Name,
				ID:             node.ID,
				Status:         node.Status,
				AgentConnected: hasAgent,
				CanExecute:     hasAgent && controlEnabled,
				VMs:            []TopologyVM{},
				Containers:     []TopologyLXC{},
			}
		}
	}

	ensureNode := func(name, id, status string) *ProxmoxNodeTopology {
		if !includeProxmox || summaryOnly {
			return nil
		}
		if node, exists := nodeMap[name]; exists {
			return node
		}
		if maxNodes > 0 && len(nodeMap) >= maxNodes {
			return nil
		}
		hasAgent := connectedAgentHostnames[name]
		nodeMap[name] = &ProxmoxNodeTopology{
			Name:           name,
			ID:             id,
			Status:         status,
			AgentConnected: hasAgent,
			CanExecute:     hasAgent && controlEnabled,
			VMs:            []TopologyVM{},
			Containers:     []TopologyLXC{},
		}
		return nodeMap[name]
	}

	// Add VMs to their nodes
	for _, vm := range state.VMs {
		if vm.Status == "running" {
			summary.RunningVMs++
		}

		nodeTopology := ensureNode(vm.Node, "", "unknown")
		if nodeTopology == nil {
			continue
		}

		nodeTopology.VMCount++
		if maxVMsPerNode <= 0 || len(nodeTopology.VMs) < maxVMsPerNode {
			nodeTopology.VMs = append(nodeTopology.VMs, TopologyVM{
				VMID:   vm.VMID,
				Name:   vm.Name,
				Status: vm.Status,
				CPU:    vm.CPU * 100,
				Memory: vm.Memory.Usage * 100,
				OS:     vm.OSName,
				Tags:   vm.Tags,
			})
		}
	}

	// Add containers to their nodes
	for _, ct := range state.Containers {
		if ct.Status == "running" {
			summary.RunningLXC++
		}

		nodeTopology := ensureNode(ct.Node, "", "unknown")
		if nodeTopology == nil {
			continue
		}

		nodeTopology.ContainerCount++
		if maxContainersPerNode <= 0 || len(nodeTopology.Containers) < maxContainersPerNode {
			nodeTopology.Containers = append(nodeTopology.Containers, TopologyLXC{
				VMID:      ct.VMID,
				Name:      ct.Name,
				Status:    ct.Status,
				CPU:       ct.CPU * 100,
				Memory:    ct.Memory.Usage * 100,
				OS:        ct.OSName,
				Tags:      ct.Tags,
				HasDocker: ct.HasDocker,
			})
		}
	}

	// Convert node map to slice
	proxmoxNodes := []ProxmoxNodeTopology{}
	if includeProxmox && !summaryOnly {
		for _, node := range nodeMap {
			proxmoxNodes = append(proxmoxNodes, *node)
		}
	}

	// Build Docker topology
	dockerHosts := []DockerHostTopology{}
	for _, host := range state.DockerHosts {
		hasAgent := connectedAgentHostnames[host.Hostname]
		runningCount := 0
		var containers []DockerContainerSummary

		for _, c := range host.Containers {
			if c.State == "running" {
				runningCount++
				summary.RunningDocker++
			}
			summary.TotalDockerContainers++

			if includeDocker && !summaryOnly {
				if maxDockerContainersPerHost <= 0 || len(containers) < maxDockerContainersPerHost {
					containers = append(containers, DockerContainerSummary{
						ID:     c.ID,
						Name:   c.Name,
						State:  c.State,
						Image:  c.Image,
						Health: c.Health,
					})
				}
			}
		}

		if includeDocker && !summaryOnly {
			if maxDockerHosts > 0 && len(dockerHosts) >= maxDockerHosts {
				continue
			}

			dockerHosts = append(dockerHosts, DockerHostTopology{
				Hostname:       host.Hostname,
				DisplayName:    host.DisplayName,
				AgentConnected: hasAgent,
				CanExecute:     hasAgent && controlEnabled,
				Containers:     containers,
				ContainerCount: len(host.Containers),
				RunningCount:   runningCount,
			})
		}
	}

	response := TopologyResponse{
		Proxmox: ProxmoxTopology{Nodes: proxmoxNodes},
		Docker:  DockerTopology{Hosts: dockerHosts},
		Summary: summary,
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeGetResource(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceType, _ := args["resource_type"].(string)
	resourceID, _ := args["resource_id"].(string)

	if resourceType == "" {
		return NewErrorResult(fmt.Errorf("resource_type is required")), nil
	}
	if resourceID == "" {
		return NewErrorResult(fmt.Errorf("resource_id is required")), nil
	}

	if e.stateProvider == nil {
		return NewTextResult("State information not available."), nil
	}

	state := e.stateProvider.GetState()

	switch resourceType {
	case "vm":
		for _, vm := range state.VMs {
			if fmt.Sprintf("%d", vm.VMID) == resourceID || vm.Name == resourceID || vm.ID == resourceID {
				response := ResourceResponse{
					Type:   "vm",
					ID:     vm.ID,
					Name:   vm.Name,
					Status: vm.Status,
					Node:   vm.Node,
					CPU: ResourceCPU{
						Percent: vm.CPU * 100,
						Cores:   vm.CPUs,
					},
					Memory: ResourceMemory{
						Percent: vm.Memory.Usage * 100,
						UsedGB:  float64(vm.Memory.Used) / (1024 * 1024 * 1024),
						TotalGB: float64(vm.Memory.Total) / (1024 * 1024 * 1024),
					},
					OS:   vm.OSName,
					Tags: vm.Tags,
				}
				if !vm.LastBackup.IsZero() {
					response.LastBackup = &vm.LastBackup
				}
				for _, nic := range vm.NetworkInterfaces {
					response.Networks = append(response.Networks, NetworkInfo{
						Name:      nic.Name,
						Addresses: nic.Addresses,
					})
				}
				// Register in resolved context WITH explicit access (single-resource get operation)
				e.registerResolvedResourceWithExplicitAccess(ResourceRegistration{
					Kind:          "vm",
					ProviderUID:   fmt.Sprintf("%d", vm.VMID), // VMID is the stable provider ID
					Name:          vm.Name,
					Aliases:       []string{vm.Name, fmt.Sprintf("%d", vm.VMID), vm.ID},
					HostUID:       vm.Node,
					HostName:      vm.Node,
					VMID:          vm.VMID,
					Node:          vm.Node,
					LocationChain: []string{"node:" + vm.Node, "vm:" + vm.Name},
					Executors: []ExecutorRegistration{{
						ExecutorID: vm.Node,
						Adapter:    "qm",
						Actions:    []string{"query", "get", "logs", "console"},
						Priority:   10,
					}},
				})
				return NewJSONResult(response), nil
			}
		}
		return NewJSONResult(map[string]interface{}{
			"error":       "not_found",
			"resource_id": resourceID,
			"type":        "vm",
		}), nil

	case "container":
		for _, ct := range state.Containers {
			if fmt.Sprintf("%d", ct.VMID) == resourceID || ct.Name == resourceID || ct.ID == resourceID {
				response := ResourceResponse{
					Type:   "container",
					ID:     ct.ID,
					Name:   ct.Name,
					Status: ct.Status,
					Node:   ct.Node,
					CPU: ResourceCPU{
						Percent: ct.CPU * 100,
						Cores:   ct.CPUs,
					},
					Memory: ResourceMemory{
						Percent: ct.Memory.Usage * 100,
						UsedGB:  float64(ct.Memory.Used) / (1024 * 1024 * 1024),
						TotalGB: float64(ct.Memory.Total) / (1024 * 1024 * 1024),
					},
					OS:   ct.OSName,
					Tags: ct.Tags,
				}
				if !ct.LastBackup.IsZero() {
					response.LastBackup = &ct.LastBackup
				}
				for _, nic := range ct.NetworkInterfaces {
					response.Networks = append(response.Networks, NetworkInfo{
						Name:      nic.Name,
						Addresses: nic.Addresses,
					})
				}
				// Register in resolved context WITH explicit access (single-resource get operation)
				e.registerResolvedResourceWithExplicitAccess(ResourceRegistration{
					Kind:          "lxc",
					ProviderUID:   fmt.Sprintf("%d", ct.VMID), // VMID is the stable provider ID
					Name:          ct.Name,
					Aliases:       []string{ct.Name, fmt.Sprintf("%d", ct.VMID), ct.ID},
					HostUID:       ct.Node,
					HostName:      ct.Node,
					VMID:          ct.VMID,
					Node:          ct.Node,
					LocationChain: []string{"node:" + ct.Node, "lxc:" + ct.Name},
					Executors: []ExecutorRegistration{{
						ExecutorID: ct.Node,
						Adapter:    "pct",
						Actions:    []string{"query", "get", "logs", "console", "exec"},
						Priority:   10,
					}},
				})
				return NewJSONResult(response), nil
			}
		}
		return NewJSONResult(map[string]interface{}{
			"error":       "not_found",
			"resource_id": resourceID,
			"type":        "container",
		}), nil

	case "docker":
		for _, host := range state.DockerHosts {
			for _, c := range host.Containers {
				if c.ID == resourceID || c.Name == resourceID || strings.HasPrefix(c.ID, resourceID) {
					response := ResourceResponse{
						Type:   "docker",
						ID:     c.ID,
						Name:   c.Name,
						Status: c.State,
						Host:   host.Hostname,
						Image:  c.Image,
						Health: c.Health,
						CPU: ResourceCPU{
							Percent: c.CPUPercent,
						},
						Memory: ResourceMemory{
							Percent: c.MemoryPercent,
							UsedGB:  float64(c.MemoryUsage) / (1024 * 1024 * 1024),
							TotalGB: float64(c.MemoryLimit) / (1024 * 1024 * 1024),
						},
						RestartCount: c.RestartCount,
						Labels:       c.Labels,
					}

					if c.UpdateStatus != nil && c.UpdateStatus.UpdateAvailable {
						response.UpdateAvailable = true
					}

					for _, p := range c.Ports {
						response.Ports = append(response.Ports, PortInfo{
							Private:  p.PrivatePort,
							Public:   p.PublicPort,
							Protocol: p.Protocol,
							IP:       p.IP,
						})
					}

					for _, n := range c.Networks {
						response.Networks = append(response.Networks, NetworkInfo{
							Name:      n.Name,
							Addresses: []string{n.IPv4},
						})
					}

					for _, m := range c.Mounts {
						response.Mounts = append(response.Mounts, MountInfo{
							Source:      m.Source,
							Destination: m.Destination,
							ReadWrite:   m.RW,
						})
					}

					// Register in resolved context WITH explicit access (single-resource get operation)
					aliases := []string{c.Name, c.ID}
					if len(c.ID) > 12 {
						aliases = append(aliases, c.ID[:12]) // Add short ID for longer IDs
					}
					e.registerResolvedResourceWithExplicitAccess(ResourceRegistration{
						Kind:          "docker_container",
						ProviderUID:   c.ID, // Docker container ID is the stable provider ID
						Name:          c.Name,
						Aliases:       aliases,
						HostUID:       host.ID,
						HostName:      host.Hostname,
						LocationChain: []string{"host:" + host.Hostname, "docker:" + c.Name},
						Executors: []ExecutorRegistration{{
							ExecutorID: host.Hostname,
							Adapter:    "docker",
							Actions:    []string{"query", "get", "logs", "exec", "restart", "stop", "start"},
							Priority:   10,
						}},
					})
					return NewJSONResult(response), nil
				}
			}
		}
		return NewJSONResult(map[string]interface{}{
			"error":       "not_found",
			"resource_id": resourceID,
			"type":        "docker",
		}), nil

	default:
		return NewErrorResult(fmt.Errorf("invalid resource_type: %s. Use 'vm', 'container', or 'docker'", resourceType)), nil
	}
}

func (e *PulseToolExecutor) executeGetGuestConfig(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceType, _ := args["resource_type"].(string)
	resourceID, _ := args["resource_id"].(string)

	if resourceType == "" {
		return NewErrorResult(fmt.Errorf("resource_type is required")), nil
	}
	if resourceID == "" {
		return NewErrorResult(fmt.Errorf("resource_id is required")), nil
	}
	if e.stateProvider == nil {
		return NewTextResult("State information not available."), nil
	}
	if e.guestConfigProvider == nil {
		return NewTextResult("Guest configuration not available."), nil
	}

	state := e.stateProvider.GetState()
	guestType, vmid, name, node, instance, err := resolveGuestFromState(state, resourceType, resourceID)
	if err != nil {
		return NewErrorResult(err), nil
	}

	rawConfig, err := e.guestConfigProvider.GetGuestConfig(guestType, instance, node, vmid)
	if err != nil {
		return NewErrorResult(err), nil
	}

	response := GuestConfigResponse{
		GuestType: guestType,
		VMID:      vmid,
		Name:      name,
		Node:      node,
		Instance:  instance,
	}

	switch guestType {
	case "container", "lxc":
		hostname, osType, onboot, rootfs, mounts := parseContainerConfig(rawConfig)
		response.Hostname = hostname
		response.OSType = osType
		response.Onboot = onboot
		response.RootFS = rootfs
		response.Mounts = mounts
	case "vm":
		osType, onboot, disks := parseVMConfig(rawConfig)
		response.OSType = osType
		response.Onboot = onboot
		response.Disks = disks
	default:
		return NewErrorResult(fmt.Errorf("unsupported guest type: %s", guestType)), nil
	}

	return NewJSONResult(response), nil
}

func resolveGuestFromState(state models.StateSnapshot, resourceType, resourceID string) (guestType string, vmid int, name, node, instance string, err error) {
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	resourceID = strings.TrimSpace(resourceID)
	if resourceType == "" || resourceID == "" {
		return "", 0, "", "", "", fmt.Errorf("resource_type and resource_id are required")
	}

	switch resourceType {
	case "container", "lxc":
		for _, ct := range state.Containers {
			if fmt.Sprintf("%d", ct.VMID) == resourceID || ct.Name == resourceID || ct.ID == resourceID {
				return "container", ct.VMID, ct.Name, ct.Node, ct.Instance, nil
			}
		}
		return "", 0, "", "", "", fmt.Errorf("container not found: %s", resourceID)
	case "vm":
		for _, vm := range state.VMs {
			if fmt.Sprintf("%d", vm.VMID) == resourceID || vm.Name == resourceID || vm.ID == resourceID {
				return "vm", vm.VMID, vm.Name, vm.Node, vm.Instance, nil
			}
		}
		return "", 0, "", "", "", fmt.Errorf("vm not found: %s", resourceID)
	default:
		return "", 0, "", "", "", fmt.Errorf("invalid resource_type: %s", resourceType)
	}
}

func parseContainerConfig(config map[string]interface{}) (hostname, osType string, onboot *bool, rootfs string, mounts []GuestMountConfig) {
	if len(config) == 0 {
		return "", "", nil, "", nil
	}

	for key, value := range config {
		lowerKey := strings.ToLower(strings.TrimSpace(key))
		switch lowerKey {
		case "hostname":
			hostname = strings.TrimSpace(fmt.Sprint(value))
		case "ostype":
			osType = strings.TrimSpace(fmt.Sprint(value))
		case "onboot":
			onboot = parseOnbootValue(value)
		}
		if lowerKey != "rootfs" && !strings.HasPrefix(lowerKey, "mp") {
			continue
		}

		raw := strings.TrimSpace(fmt.Sprint(value))
		if raw == "" {
			continue
		}

		source, mountpoint := parseMountValue(raw)
		if lowerKey == "rootfs" {
			rootfs = source
			if mountpoint == "" {
				mountpoint = "/"
			}
		}

		mounts = append(mounts, GuestMountConfig{
			Key:        lowerKey,
			Source:     source,
			Mountpoint: mountpoint,
		})
	}

	if len(mounts) > 1 {
		sort.Slice(mounts, func(i, j int) bool {
			return mounts[i].Key < mounts[j].Key
		})
	}

	return hostname, osType, onboot, rootfs, mounts
}

func parseVMConfig(config map[string]interface{}) (osType string, onboot *bool, disks []GuestDiskConfig) {
	if len(config) == 0 {
		return "", nil, nil
	}

	for key, value := range config {
		lowerKey := strings.ToLower(strings.TrimSpace(key))
		switch lowerKey {
		case "ostype":
			osType = strings.TrimSpace(fmt.Sprint(value))
		case "onboot":
			onboot = parseOnbootValue(value)
		}
		if !isVMConfigDiskKey(lowerKey) {
			continue
		}
		raw := strings.TrimSpace(fmt.Sprint(value))
		if raw == "" {
			continue
		}
		disks = append(disks, GuestDiskConfig{
			Key:   lowerKey,
			Value: raw,
		})
	}

	if len(disks) > 1 {
		sort.Slice(disks, func(i, j int) bool {
			return disks[i].Key < disks[j].Key
		})
	}

	return osType, onboot, disks
}

func isVMConfigDiskKey(key string) bool {
	if strings.HasPrefix(key, "scsi") ||
		strings.HasPrefix(key, "virtio") ||
		strings.HasPrefix(key, "sata") ||
		strings.HasPrefix(key, "ide") ||
		strings.HasPrefix(key, "unused") ||
		strings.HasPrefix(key, "efidisk") ||
		strings.HasPrefix(key, "tpmstate") {
		return true
	}
	return false
}

func parseMountValue(raw string) (source, mountpoint string) {
	parts := strings.Split(raw, ",")
	if len(parts) > 0 {
		source = strings.TrimSpace(parts[0])
	}
	for _, part := range parts[1:] {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(kv[0]))
		v := strings.TrimSpace(kv[1])
		if k == "mp" || k == "mountpoint" {
			mountpoint = v
		}
	}
	return source, mountpoint
}

func parseOnbootValue(value interface{}) *bool {
	raw := strings.TrimSpace(fmt.Sprint(value))
	if raw == "" {
		return nil
	}
	if raw == "1" || strings.EqualFold(raw, "yes") || strings.EqualFold(raw, "true") {
		val := true
		return &val
	}
	if raw == "0" || strings.EqualFold(raw, "no") || strings.EqualFold(raw, "false") {
		val := false
		return &val
	}
	return nil
}

func (e *PulseToolExecutor) executeSearchResources(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	rawQuery, _ := args["query"].(string)
	query := strings.TrimSpace(rawQuery)
	if query == "" {
		return NewErrorResult(fmt.Errorf("query is required")), nil
	}

	typeFilter, _ := args["type"].(string)
	// Map resource_type to type for search
	if typeFilter == "" {
		typeFilter, _ = args["resource_type"].(string)
	}
	statusFilter, _ := args["status"].(string)
	limit := intArg(args, "limit", 20)
	offset := intArg(args, "offset", 0)
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	allowedTypes := map[string]bool{
		"":            true,
		"node":        true,
		"vm":          true,
		"container":   true,
		"docker":      true,
		"docker_host": true,
	}
	if !allowedTypes[typeFilter] {
		return NewErrorResult(fmt.Errorf("invalid type: %s. Use node, vm, container, docker, or docker_host", typeFilter)), nil
	}

	// normalizeForSearch replaces common separators with spaces for fuzzy matching
	normalizeForSearch := func(s string) string {
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, "-", " ")
		s = strings.ReplaceAll(s, "_", " ")
		s = strings.ReplaceAll(s, ".", " ")
		return s
	}

	matchesQuery := func(query string, candidates ...string) bool {
		queryNorm := normalizeForSearch(query)
		queryWords := strings.Fields(queryNorm)

		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}
			candidateNorm := normalizeForSearch(candidate)

			// Direct substring match (normalized)
			if strings.Contains(candidateNorm, queryNorm) {
				return true
			}

			// All query words must be present in candidate
			if len(queryWords) > 0 {
				allMatch := true
				for _, word := range queryWords {
					if !strings.Contains(candidateNorm, word) {
						allMatch = false
						break
					}
				}
				if allMatch {
					return true
				}
			}
		}
		return false
	}

	// Helper to collect IP addresses from guest network interfaces
	collectGuestIPs := func(interfaces []models.GuestNetworkInterface) []string {
		var ips []string
		for _, iface := range interfaces {
			ips = append(ips, iface.Addresses...)
		}
		return ips
	}

	// Helper to collect IP addresses from Docker container networks
	collectDockerIPs := func(networks []models.DockerContainerNetworkLink) []string {
		var ips []string
		for _, net := range networks {
			if net.IPv4 != "" {
				ips = append(ips, net.IPv4)
			}
			if net.IPv6 != "" {
				ips = append(ips, net.IPv6)
			}
		}
		return ips
	}

	queryLower := strings.ToLower(query)
	state := e.stateProvider.GetState()

	// Build a set of connected agent hostnames for quick lookup
	connectedAgentHostnames := make(map[string]bool)
	if e.agentServer != nil {
		for _, agent := range e.agentServer.GetConnectedAgents() {
			connectedAgentHostnames[agent.Hostname] = true
		}
	}

	matches := make([]ResourceMatch, 0, limit)
	total := 0

	addMatch := func(match ResourceMatch) {
		if total < offset {
			total++
			return
		}
		if len(matches) >= limit {
			total++
			return
		}
		matches = append(matches, match)
		total++
	}

	if typeFilter == "" || typeFilter == "node" {
		for _, node := range state.Nodes {
			if statusFilter != "" && !strings.EqualFold(node.Status, statusFilter) {
				continue
			}
			if !matchesQuery(queryLower, node.Name, node.ID) {
				continue
			}
			addMatch(ResourceMatch{
				Type:           "node",
				ID:             node.ID,
				Name:           node.Name,
				Status:         node.Status,
				AgentConnected: connectedAgentHostnames[node.Name],
			})
		}
	}

	if typeFilter == "" || typeFilter == "vm" {
		for _, vm := range state.VMs {
			if statusFilter != "" && !strings.EqualFold(vm.Status, statusFilter) {
				continue
			}
			// Build searchable candidates: name, ID, VMID, IPs, tags
			candidates := []string{vm.Name, vm.ID, fmt.Sprintf("%d", vm.VMID)}
			candidates = append(candidates, vm.IPAddresses...)
			candidates = append(candidates, vm.Tags...)
			candidates = append(candidates, collectGuestIPs(vm.NetworkInterfaces)...)

			if !matchesQuery(queryLower, candidates...) {
				continue
			}
			addMatch(ResourceMatch{
				Type:           "vm",
				ID:             vm.ID,
				Name:           vm.Name,
				Status:         vm.Status,
				Node:           vm.Node,
				NodeHasAgent:   connectedAgentHostnames[vm.Node],
				VMID:           vm.VMID,
				AgentConnected: connectedAgentHostnames[vm.Name],
			})
		}
	}

	if typeFilter == "" || typeFilter == "container" {
		for _, ct := range state.Containers {
			if statusFilter != "" && !strings.EqualFold(ct.Status, statusFilter) {
				continue
			}
			// Build searchable candidates: name, ID, VMID, IPs, tags
			candidates := []string{ct.Name, ct.ID, fmt.Sprintf("%d", ct.VMID)}
			candidates = append(candidates, ct.IPAddresses...)
			candidates = append(candidates, ct.Tags...)
			candidates = append(candidates, collectGuestIPs(ct.NetworkInterfaces)...)

			if !matchesQuery(queryLower, candidates...) {
				continue
			}
			addMatch(ResourceMatch{
				Type:           "container",
				ID:             ct.ID,
				Name:           ct.Name,
				Status:         ct.Status,
				Node:           ct.Node,
				NodeHasAgent:   connectedAgentHostnames[ct.Node],
				VMID:           ct.VMID,
				AgentConnected: connectedAgentHostnames[ct.Name],
			})
		}
	}

	if typeFilter == "" || typeFilter == "docker_host" {
		for _, host := range state.DockerHosts {
			if statusFilter != "" && !strings.EqualFold(host.Status, statusFilter) {
				continue
			}
			if !matchesQuery(queryLower, host.ID, host.Hostname, host.DisplayName, host.CustomDisplayName) {
				continue
			}
			displayName := host.DisplayName
			if host.CustomDisplayName != "" {
				displayName = host.CustomDisplayName
			}
			if displayName == "" {
				displayName = host.Hostname
			}
			addMatch(ResourceMatch{
				Type:   "docker_host",
				ID:     host.ID,
				Name:   displayName,
				Status: host.Status,
				Host:   host.Hostname,
			})
		}
	}

	if typeFilter == "" || typeFilter == "docker" {
		for _, host := range state.DockerHosts {
			for _, c := range host.Containers {
				if statusFilter != "" && !strings.EqualFold(c.State, statusFilter) {
					continue
				}
				// Build searchable candidates: name, ID, image, IPs
				candidates := []string{c.Name, c.ID, c.Image}
				candidates = append(candidates, collectDockerIPs(c.Networks)...)

				if !matchesQuery(queryLower, candidates...) {
					continue
				}
				addMatch(ResourceMatch{
					Type:   "docker",
					ID:     c.ID,
					Name:   c.Name,
					Status: c.State,
					Host:   host.Hostname,
					Image:  c.Image,
				})
			}
		}
	}

	response := ResourceSearchResponse{
		Query:   query,
		Matches: matches,
		Total:   total,
	}

	if offset > 0 || total > limit {
		response.Pagination = &PaginationInfo{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		}
	}

	// Register all found resources in the resolved context
	// This enables action tools to validate that commands target legitimate resources
	for _, match := range matches {
		var reg ResourceRegistration

		switch match.Type {
		case "node":
			reg = ResourceRegistration{
				Kind:          "node",
				ProviderUID:   match.ID, // Node ID is the provider UID
				Name:          match.Name,
				Aliases:       []string{match.Name, match.ID},
				HostName:      match.Name,
				LocationChain: []string{"node:" + match.Name},
				Executors: []ExecutorRegistration{{
					ExecutorID: match.Name,
					Adapter:    "direct",
					Actions:    []string{"query", "get", "exec"},
					Priority:   10,
				}},
			}
		case "vm":
			reg = ResourceRegistration{
				Kind:          "vm",
				ProviderUID:   fmt.Sprintf("%d", match.VMID),
				Name:          match.Name,
				Aliases:       []string{match.Name, fmt.Sprintf("%d", match.VMID), match.ID},
				HostUID:       match.Node,
				HostName:      match.Node,
				VMID:          match.VMID,
				Node:          match.Node,
				LocationChain: []string{"node:" + match.Node, "vm:" + match.Name},
				Executors: []ExecutorRegistration{{
					ExecutorID: match.Node,
					Adapter:    "qm",
					Actions:    []string{"query", "get", "logs", "console"},
					Priority:   10,
				}},
			}
		case "container":
			reg = ResourceRegistration{
				Kind:          "lxc",
				ProviderUID:   fmt.Sprintf("%d", match.VMID),
				Name:          match.Name,
				Aliases:       []string{match.Name, fmt.Sprintf("%d", match.VMID), match.ID},
				HostUID:       match.Node,
				HostName:      match.Node,
				VMID:          match.VMID,
				Node:          match.Node,
				LocationChain: []string{"node:" + match.Node, "lxc:" + match.Name},
				Executors: []ExecutorRegistration{{
					ExecutorID: match.Node,
					Adapter:    "pct",
					Actions:    []string{"query", "get", "logs", "console", "exec"},
					Priority:   10,
				}},
			}
		case "docker_host":
			reg = ResourceRegistration{
				Kind:          "docker_host",
				ProviderUID:   match.ID,
				Name:          match.Name,
				Aliases:       []string{match.Name, match.ID, match.Host},
				HostUID:       match.Host,
				HostName:      match.Host,
				LocationChain: []string{"host:" + match.Host},
				Executors: []ExecutorRegistration{{
					ExecutorID: match.Host,
					Adapter:    "direct",
					Actions:    []string{"query", "get"},
					Priority:   10,
				}},
			}
		case "docker":
			reg = ResourceRegistration{
				Kind:          "docker_container",
				ProviderUID:   match.ID, // Docker container ID
				Name:          match.Name,
				Aliases:       []string{match.Name, match.ID},
				HostUID:       match.Host,
				HostName:      match.Host,
				LocationChain: []string{"host:" + match.Host, "docker:" + match.Name},
				Executors: []ExecutorRegistration{{
					ExecutorID: match.Host,
					Adapter:    "docker",
					Actions:    []string{"query", "get", "logs", "exec", "restart", "stop", "start"},
					Priority:   10,
				}},
			}
		default:
			continue // Skip unknown types
		}

		e.registerResolvedResource(reg)
	}

	return NewJSONResult(response), nil
}

// Helper to get int args with default
func intArg(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		case int64:
			return int(val)
		}
	}
	return defaultVal
}
