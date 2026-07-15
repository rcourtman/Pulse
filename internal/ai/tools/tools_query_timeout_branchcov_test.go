package tools

import (
	"strings"
	"testing"
)

// TestIsTimeoutDurationToken_BranchCoverage exercises every arm of
// isTimeoutDurationToken: the empty-token guard, the digit/first-dot loop
// arms, the suffix-unit check (all four units plus non-matching suffixes and
// the seenDigit=false case), and the loop-completion arm hit by no-unit digit
// and dot-without-unit tokens.
func TestIsTimeoutDurationToken_BranchCoverage(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{name: "empty token rejected", token: "", want: false},
		{name: "no-unit digit token accepted", token: "30", want: true},
		{name: "single digit no unit accepted", token: "5", want: true},
		{name: "digit plus s unit", token: "5s", want: true},
		{name: "digit plus m unit", token: "5m", want: true},
		{name: "digit plus h unit", token: "5h", want: true},
		{name: "digit plus d unit", token: "5d", want: true},
		{name: "dot plus unit duration accepted", token: "1.5s", want: true},
		{name: "dot no unit accepted as bare number", token: "1.5", want: true},
		{name: "second dot falls through to suffix mismatch", token: "1.5.5", want: false},
		{name: "bare dot no digit rejected", token: ".", want: false},
		{name: "digit plus non-unit letter rejected", token: "5x", want: false},
		{name: "digit plus multi-char suffix rejected", token: "5ss", want: false},
		{name: "unit suffix without leading digit rejected", token: "s", want: false},
		{name: "non-digit non-unit prefix rejected", token: "abc", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isTimeoutDurationToken(tc.token)
			if got != tc.want {
				t.Fatalf("isTimeoutDurationToken(%q) = %v, want %v", tc.token, got, tc.want)
			}
		})
	}
}

// TestTimeoutInnerCommand_BranchCoverage walks every flag-parsing arm of
// timeoutInnerCommand: the too-short guard, the "--" separator, each
// standalone flag (--preserve-status, --foreground, --verbose, -v), the
// value-taking flags -s/--signal/-k/--kill-after, the "=" attached forms, the
// combined short forms (-sX/-kX), the unknown-flag rejection, the missing
// duration arm, the missing command arm, and the success/join arm.
func TestTimeoutInnerCommand_BranchCoverage(t *testing.T) {
	tests := []struct {
		name    string
		fields  []string
		wantCmd string
		wantOk  bool
	}{
		{name: "too short single arg", fields: []string{"timeout"}, wantOk: false},
		{name: "too short two args", fields: []string{"timeout", "5s"}, wantOk: false},
		{name: "bare duration then command", fields: []string{"timeout", "5s", "ls"}, wantCmd: "ls", wantOk: true},
		{name: "join multiple args after duration", fields: []string{"timeout", "5s", "ls", "-la"}, wantCmd: "ls -la", wantOk: true},
		{name: "double dash separator", fields: []string{"timeout", "--", "5s", "ls"}, wantCmd: "ls", wantOk: true},
		{name: "preserve status flag", fields: []string{"timeout", "--preserve-status", "5s", "ls"}, wantCmd: "ls", wantOk: true},
		{name: "foreground flag", fields: []string{"timeout", "--foreground", "5s", "ls"}, wantCmd: "ls", wantOk: true},
		{name: "verbose flag", fields: []string{"timeout", "--verbose", "5s", "ls"}, wantCmd: "ls", wantOk: true},
		{name: "short verbose flag", fields: []string{"timeout", "-v", "5s", "ls"}, wantCmd: "ls", wantOk: true},
		{name: "short signal flag with separate value", fields: []string{"timeout", "-s", "TERM", "5s", "ls"}, wantCmd: "ls", wantOk: true},
		{name: "long signal flag with separate value", fields: []string{"timeout", "--signal", "TERM", "5s", "ls"}, wantCmd: "ls", wantOk: true},
		{name: "short kill flag with separate value", fields: []string{"timeout", "-k", "2s", "5s", "ls"}, wantCmd: "ls", wantOk: true},
		{name: "long kill flag with separate value", fields: []string{"timeout", "--kill-after", "2s", "5s", "ls"}, wantCmd: "ls", wantOk: true},
		{name: "attached long signal form", fields: []string{"timeout", "--signal=TERM", "5s", "ls"}, wantCmd: "ls", wantOk: true},
		{name: "attached long kill form", fields: []string{"timeout", "--kill-after=2s", "5s", "ls"}, wantCmd: "ls", wantOk: true},
		{name: "combined short signal form", fields: []string{"timeout", "-sTERM", "5s", "ls"}, wantCmd: "ls", wantOk: true},
		{name: "combined short kill form", fields: []string{"timeout", "-k5s", "5s", "ls"}, wantCmd: "ls", wantOk: true},
		{name: "unknown short flag rejected", fields: []string{"timeout", "-x", "5s"}, wantOk: false},
		{name: "separator then non-duration token", fields: []string{"timeout", "--", "notdur", "ls"}, wantOk: false},
		{name: "bare non-duration token", fields: []string{"timeout", "notdur", "ls"}, wantOk: false},
		{name: "duration present but no command after separator", fields: []string{"timeout", "--", "5s"}, wantOk: false},
		{name: "standalone flags consume everything then no duration", fields: []string{"timeout", "--preserve-status", "--verbose"}, wantOk: false},
		{name: "value flag consumes everything then no duration", fields: []string{"timeout", "-s", "TERM"}, wantOk: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotCmd, gotOk := timeoutInnerCommand(tc.fields)
			if gotOk != tc.wantOk {
				t.Fatalf("timeoutInnerCommand(%v) ok = %v, want %v", tc.fields, gotOk, tc.wantOk)
			}
			if gotOk && gotCmd != tc.wantCmd {
				t.Fatalf("timeoutInnerCommand(%v) cmd = %q, want %q", tc.fields, gotCmd, tc.wantCmd)
			}
			if !gotOk && gotCmd != "" {
				t.Fatalf("timeoutInnerCommand(%v) returned non-empty cmd %q on failure", tc.fields, gotCmd)
			}
		})
	}
}

// TestClassifyTimeoutWrapper_BranchCoverage drives the four return arms of
// classifyTimeoutWrapper: not-a-timeout (handled=false), malformed timeout
// wrapper, inner read-only-certain, inner read-only-conditional, and inner
// write/unknown.
func TestClassifyTimeoutWrapper_BranchCoverage(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		wantHandled    bool
		wantIntent     ExecutionIntent
		reasonContains string
	}{
		{name: "empty command not handled", command: "", wantHandled: false},
		{name: "non-timeout first token not handled", command: "ls -la", wantHandled: false},
		{name: "case-insensitive timeout prefix", command: "TIMEOUT 5s ls", wantHandled: true, wantIntent: IntentReadOnlyCertain, reasonContains: "timeout-bounded"},
		{name: "path-qualified timeout binary", command: "/usr/bin/timeout 5s ls", wantHandled: true, wantIntent: IntentReadOnlyCertain, reasonContains: "timeout-bounded"},
		{name: "quoted timeout token", command: `"timeout" 5s ls`, wantHandled: true, wantIntent: IntentReadOnlyCertain, reasonContains: "timeout-bounded"},
		{name: "bare timeout missing duration and command", command: "timeout", wantHandled: true, wantIntent: IntentWriteOrUnknown, reasonContains: "timeout wrapper must include a duration and command"},
		{name: "timeout duration but no command", command: "timeout 5s", wantHandled: true, wantIntent: IntentWriteOrUnknown, reasonContains: "timeout wrapper must include a duration and command"},
		{name: "timeout wrapping write command stays write", command: "timeout 5s rm -rf /tmp/branch", wantHandled: true, wantIntent: IntentWriteOrUnknown, reasonContains: "timeout-wrapped command not read-only"},
		{name: "timeout wrapping conditional read-only sql", command: `timeout 5s sqlite3 ":memory:" "SELECT 1"`, wantHandled: true, wantIntent: IntentReadOnlyConditional, reasonContains: "timeout-bounded"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, handled := classifyTimeoutWrapper(tc.command)
			if handled != tc.wantHandled {
				t.Fatalf("classifyTimeoutWrapper(%q) handled = %v, want %v", tc.command, handled, tc.wantHandled)
			}
			if !tc.wantHandled {
				// Unhandled invocations return the zero-value IntentResult.
				if result.Intent != 0 || result.Reason != "" || result.NonInteractiveBlock != nil {
					t.Fatalf("classifyTimeoutWrapper(%q) unhandled returned non-zero result %+v", tc.command, result)
				}
				return
			}
			if result.Intent != tc.wantIntent {
				t.Fatalf("classifyTimeoutWrapper(%q) intent = %v, want %v", tc.command, result.Intent, tc.wantIntent)
			}
			if tc.reasonContains != "" && !strings.Contains(result.Reason, tc.reasonContains) {
				t.Fatalf("classifyTimeoutWrapper(%q) reason = %q, want substring %q", tc.command, result.Reason, tc.reasonContains)
			}
		})
	}
}
