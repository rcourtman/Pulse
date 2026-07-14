package tools

import "testing"

// This file provides white-box table tests for the pure string-routing hint
// helpers in tools_query.go:
//   - GetReadOnlyViolationHint (top-level routing)
//   - isPhase1GuardrailFailure (keyword classification)
//   - getPhase1Hint            (structural guardrail switch)
//   - getSQLHint               (SQL content-inspection switch)
//
// All four functions are pure (no I/O) and return exact strings/bools, so every
// case asserts the exact returned value rather than just "no panic".

func TestGetReadOnlyViolationHint(t *testing.T) {
	tests := []struct {
		name    string
		command string
		result  IntentResult
		want    string
	}{
		// Empty / boundary inputs.
		{
			name:    "empty command and empty reason returns empty base hint",
			command: "",
			result:  IntentResult{},
			want:    "",
		},
		{
			name:    "phase1 reason with empty command still routes to phase1 hint",
			command: "",
			result:  IntentResult{Intent: IntentWriteOrUnknown, Reason: "sudo escalates command privileges"},
			want:    "sudo escalates command privileges. Read-only execution does not accept sudo; privileged operations require the governed control path.",
		},
		{
			name:    "sql cli command with empty reason routes to sql default hint",
			command: "sqlite3 db",
			result:  IntentResult{},
			want:    ". For read-only queries, use self-contained SELECT statements without transaction control.",
		},

		// Phase 1 guardrail routing.
		{
			name:    "phase1 redirect reason routes to phase1 redirect hint",
			command: "echo hi > /tmp/x",
			result:  IntentResult{Intent: IntentWriteOrUnknown, Reason: "output redirection can overwrite files"},
			want:    "output redirection can overwrite files. Read-only execution does not accept redirects (>, >>, <, <<, <<<).",
		},

		// Phase 1 precedence over SQL CLI detection.
		{
			name:    "phase1 takes precedence over sql cli command match",
			command: "sudo sqlite3 db.db \"SELECT 1\"",
			result:  IntentResult{Intent: IntentWriteOrUnknown, Reason: "sudo escalates command privileges"},
			want:    "sudo escalates command privileges. Read-only execution does not accept sudo; privileged operations require the governed control path.",
		},

		// SQL CLI routing for each recognised CLI binary.
		{
			name:    "sqlite3 cli with write keyword routes to sql write hint",
			command: "sqlite3 db.db \"INSERT INTO t VALUES(1)\"",
			result:  IntentResult{Intent: IntentWriteOrUnknown, Reason: "content inspection: SQL contains write/control keyword: insert"},
			want:    "content inspection: SQL contains write/control keyword: insert. Use only SELECT statements. Avoid: INSERT, UPDATE, DELETE, DROP, CREATE, PRAGMA, BEGIN, COMMIT, ROLLBACK.",
		},
		{
			name:    "mysql cli with no-inline reason routes to sql external hint",
			command: "mysql db",
			result:  IntentResult{Intent: IntentWriteOrUnknown, Reason: "content inspection: no inline SQL found; input may be external (piped/interactive)"},
			want:    "content inspection: no inline SQL found; input may be external (piped/interactive). Include SQL directly in quotes: sqlite3 db.db \"SELECT ...\"",
		},
		{
			name:    "mariadb cli with neutral reason routes to sql default hint",
			command: "mariadb db -e \"SELECT 1\"",
			result:  IntentResult{Intent: IntentReadOnlyConditional, Reason: "content inspection: read-only"},
			want:    "content inspection: read-only. For read-only queries, use self-contained SELECT statements without transaction control.",
		},
		{
			name:    "psql cli with control keyword routes to sql write hint",
			command: "psql -c \"BEGIN\"",
			result:  IntentResult{Intent: IntentWriteOrUnknown, Reason: "content inspection: SQL contains write/control keyword: begin"},
			want:    "content inspection: SQL contains write/control keyword: begin. Use only SELECT statements. Avoid: INSERT, UPDATE, DELETE, DROP, CREATE, PRAGMA, BEGIN, COMMIT, ROLLBACK.",
		},

		// SQL CLI precedence over the unknown-command fallback.
		{
			name:    "sql cli command beats unknown reason fallback",
			command: "sqlite3 db",
			result:  IntentResult{Intent: IntentWriteOrUnknown, Reason: "unknown command"},
			want:    "unknown command. For read-only queries, use self-contained SELECT statements without transaction control.",
		},

		// Unknown-command fallback (non-SQL, non-phase1).
		{
			name:    "non-sql unknown reason routes to self-contained hint",
			command: "foobar",
			result:  IntentResult{Intent: IntentWriteOrUnknown, Reason: "unknown command is not on the read-only allowlist"},
			want:    "unknown command is not on the read-only allowlist. Try a self-contained form: no pipes, no redirects, single statement. If this is a read-only operation, consider using a known read-only command instead.",
		},
		{
			name:    "non-sql no-inspector reason routes to self-contained hint",
			command: "foobar",
			result:  IntentResult{Intent: IntentWriteOrUnknown, Reason: "no inspector matched this command"},
			want:    "no inspector matched this command. Try a self-contained form: no pipes, no redirects, single statement. If this is a read-only operation, consider using a known read-only command instead.",
		},

		// Plain fallback (no routing matches): base hint returned unchanged.
		{
			name:    "non-sql unmatched reason returns base hint unchanged",
			command: "foobar",
			result:  IntentResult{Intent: IntentWriteOrUnknown, Reason: "random reason"},
			want:    "random reason",
		},

		// Documents actual (surprising) behaviour for interactive_repl blocks.
		// The interactive_repl NonInteractiveBlock reason is not in the phase1
		// keyword list, so routing falls through to SQL/unknown/default paths.
		{
			name:    "bare mysql interactive_repl block misroutes to sql default hint",
			command: "mysql",
			result:  IntentResult{Intent: IntentWriteOrUnknown, Reason: "[interactive_repl] command opens interactive session; pulse_read requires bounded non-interactive commands"},
			want:    "[interactive_repl] command opens interactive session; pulse_read requires bounded non-interactive commands. For read-only queries, use self-contained SELECT statements without transaction control.",
		},
		{
			name:    "python interactive_repl block returns base hint unchanged",
			command: "python",
			result:  IntentResult{Intent: IntentWriteOrUnknown, Reason: "[interactive_repl] command opens interactive session; pulse_read requires bounded non-interactive commands"},
			want:    "[interactive_repl] command opens interactive session; pulse_read requires bounded non-interactive commands",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GetReadOnlyViolationHint(tc.command, tc.result)
			if got != tc.want {
				t.Fatalf("GetReadOnlyViolationHint(command=%q, reason=%q) =\n  %q\nwant\n  %q",
					tc.command, tc.result.Reason, got, tc.want)
			}
		})
	}
}

func TestIsPhase1GuardrailFailure(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		want   bool
	}{
		// Phase 1 structural keywords (case-sensitive substring match).
		{name: "sudo keyword", reason: "sudo escalates command privileges", want: true},
		{name: "redirect substring inside redirection", reason: "output redirection can overwrite files", want: true},
		{name: "redirect exact substring", reason: "has redirect here", want: true},
		{name: "tee keyword", reason: "tee can write to files", want: true},
		{name: "substitution keyword", reason: "command substitution can execute arbitrary commands", want: true},
		{name: "chaining keyword", reason: "shell chaining detected outside quotes", want: true},
		{name: "piped input keyword", reason: "piped input to dual-use tool prevents content inspection", want: true},

		// NonInteractiveOnly keywords.
		{name: "TTY uppercase keyword", reason: "[tty_flag] interactive/TTY flags require terminal", want: true},
		{name: "terminal keyword alone", reason: "requires terminal interaction", want: true},
		{name: "pager keyword", reason: "[pager] pager/editor tools require terminal", want: true},
		{name: "editor keyword alone", reason: "editor tools need a terminal", want: true},
		{name: "indefinitely keyword", reason: "live monitoring tools run indefinitely", want: true},
		{name: "unbounded keyword", reason: "[unbounded_stream] follow mode without bound", want: true},
		{name: "streaming keyword alone", reason: "unbounded streaming detected", want: true},

		// Negative cases (case-sensitive, spacing-sensitive, unmatched).
		{name: "empty reason returns false", reason: "", want: false},
		{name: "tty lowercase does not match (case sensitive)", reason: "tty flag", want: false},
		{name: "SUDO uppercase does not match (case sensitive)", reason: "SUDO escalation", want: false},
		{name: "pipedinput without space does not match", reason: "pipedinput detected", want: false},
		{name: "unrelated reason returns false", reason: "unknown command is not on the read-only allowlist", want: false},
		{name: "interactive_repl real reason not classified as phase1", reason: "[interactive_repl] command opens interactive session; pulse_read requires bounded non-interactive commands", want: false},
		{name: "content inspection read-only reason not classified as phase1", reason: "content inspection: read-only", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isPhase1GuardrailFailure(tc.reason)
			if got != tc.want {
				t.Fatalf("isPhase1GuardrailFailure(%q) = %v, want %v", tc.reason, got, tc.want)
			}
		})
	}
}

func TestGetPhase1Hint(t *testing.T) {
	const base = "REASON" // arbitrary non-empty base hint to verify concatenation
	tests := []struct {
		name     string
		reason   string
		baseHint string
		want     string
	}{
		// One case per switch arm (in source order).
		{name: "sudo arm", reason: "sudo escalates", baseHint: base, want: base + ". Read-only execution does not accept sudo; privileged operations require the governed control path."},
		{name: "redirect arm via redirection substring", reason: "output redirection detected", baseHint: base, want: base + ". Read-only execution does not accept redirects (>, >>, <, <<, <<<)."},
		{name: "tee arm", reason: "tee can write to files", baseHint: base, want: base + ". Read-only execution does not accept tee because tee writes to files."},
		{name: "substitution arm", reason: "command substitution detected", baseHint: base, want: base + ". Read-only execution does not accept $() or backticks."},
		{name: "chaining arm", reason: "shell chaining detected", baseHint: base, want: base + ". Run commands separately instead of chaining with ; && ||."},
		{name: "piped input arm", reason: "piped input to dual-use tool", baseHint: base, want: base + ". For dual-use tools, include content directly instead of piping. Example: sqlite3 db.db \"SELECT ...\" instead of cat file | sqlite3 db.db"},
		{name: "TTY arm", reason: "interactive/TTY flags", baseHint: base, want: base + ". Remove -it/--tty/--interactive flags. Use non-interactive form: docker exec container cmd (not docker exec -it)."},
		{name: "terminal arm without TTY", reason: "requires terminal interaction", baseHint: base, want: base + ". Remove -it/--tty/--interactive flags. Use non-interactive form: docker exec container cmd (not docker exec -it)."},
		{name: "pager arm", reason: "pager tools", baseHint: base, want: base + ". Use cat, head -n, or tail -n instead of interactive tools."},
		{name: "editor arm without pager", reason: "editor tools", baseHint: base, want: base + ". Use cat, head -n, or tail -n instead of interactive tools."},
		{name: "indefinitely arm", reason: "runs indefinitely", baseHint: base, want: base + ". Use bounded alternatives: ps aux (not top), journalctl -n 100 (not watch)."},
		{name: "unbounded arm", reason: "unbounded stream", baseHint: base, want: base + ". Add line limit: journalctl -n 100 -f or tail -n 50 -f, or wrap with timeout."},
		{name: "streaming arm without unbounded", reason: "streaming follow mode", baseHint: base, want: base + ". Add line limit: journalctl -n 100 -f or tail -n 50 -f, or wrap with timeout."},

		// default / fallthrough arm.
		{name: "default arm with unmatched reason", reason: "unknown phase1 failure", baseHint: base, want: base + ". Read-only execution does not accept redirects, chaining, sudo, or subshells."},
		{name: "default arm with empty reason", reason: "", baseHint: base, want: base + ". Read-only execution does not accept redirects, chaining, sudo, or subshells."},

		// Switch-order precedence (first matching case wins).
		{name: "precedence sudo before redirect", reason: "sudo and redirect", baseHint: base, want: base + ". Read-only execution does not accept sudo; privileged operations require the governed control path."},
		{name: "precedence redirect before tee", reason: "redirect and tee", baseHint: base, want: base + ". Read-only execution does not accept redirects (>, >>, <, <<, <<<)."},
		{name: "precedence piped input before TTY", reason: "piped input and TTY flags", baseHint: base, want: base + ". For dual-use tools, include content directly instead of piping. Example: sqlite3 db.db \"SELECT ...\" instead of cat file | sqlite3 db.db"},
		{name: "precedence indefinitely before unbounded", reason: "indefinitely and unbounded", baseHint: base, want: base + ". Use bounded alternatives: ps aux (not top), journalctl -n 100 (not watch)."},

		// Empty base hint (still gets the suffix).
		{name: "empty base hint sudo arm", reason: "sudo", baseHint: "", want: ". Read-only execution does not accept sudo; privileged operations require the governed control path."},
		{name: "empty base hint default arm", reason: "unmatched", baseHint: "", want: ". Read-only execution does not accept redirects, chaining, sudo, or subshells."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := getPhase1Hint(tc.reason, tc.baseHint)
			if got != tc.want {
				t.Fatalf("getPhase1Hint(%q, %q) =\n  %q\nwant\n  %q",
					tc.reason, tc.baseHint, got, tc.want)
			}
		})
	}
}

func TestGetSQLHint(t *testing.T) {
	const base = "REASON" // arbitrary non-empty base hint to verify concatenation
	tests := []struct {
		name     string
		reason   string
		baseHint string
		want     string
	}{
		// external / no-inline arm (first case).
		{name: "external arm", reason: "input may be external", baseHint: base, want: base + ". Include SQL directly in quotes: sqlite3 db.db \"SELECT ...\""},
		{name: "no inline arm", reason: "no inline SQL found", baseHint: base, want: base + ". Include SQL directly in quotes: sqlite3 db.db \"SELECT ...\""},

		// write / control arm (second case).
		{name: "write arm", reason: "contains write keyword", baseHint: base, want: base + ". Use only SELECT statements. Avoid: INSERT, UPDATE, DELETE, DROP, CREATE, PRAGMA, BEGIN, COMMIT, ROLLBACK."},
		{name: "control arm", reason: "transaction control keyword", baseHint: base, want: base + ". Use only SELECT statements. Avoid: INSERT, UPDATE, DELETE, DROP, CREATE, PRAGMA, BEGIN, COMMIT, ROLLBACK."},

		// default / fallthrough arm.
		{name: "default arm with unmatched reason", reason: "read-only content", baseHint: base, want: base + ". For read-only queries, use self-contained SELECT statements without transaction control."},
		{name: "default arm with empty reason", reason: "", baseHint: base, want: base + ". For read-only queries, use self-contained SELECT statements without transaction control."},

		// Switch-order precedence (external/no-inline before write/control).
		{name: "precedence external before write", reason: "external write detected", baseHint: base, want: base + ". Include SQL directly in quotes: sqlite3 db.db \"SELECT ...\""},
		{name: "precedence no inline before control", reason: "no inline control found", baseHint: base, want: base + ". Include SQL directly in quotes: sqlite3 db.db \"SELECT ...\""},

		// Empty base hint (still gets the suffix).
		{name: "empty base hint external arm", reason: "external", baseHint: "", want: ". Include SQL directly in quotes: sqlite3 db.db \"SELECT ...\""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := getSQLHint(tc.reason, tc.baseHint)
			if got != tc.want {
				t.Fatalf("getSQLHint(%q, %q) =\n  %q\nwant\n  %q",
					tc.reason, tc.baseHint, got, tc.want)
			}
		})
	}
}
