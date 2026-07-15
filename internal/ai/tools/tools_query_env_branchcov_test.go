package tools

import "testing"

// This file provides white-box table tests that raise branch coverage for the
// pure env-classification helpers in tools_query.go:
//   - classifyEnvCommand    (top-level routing: not-env / write / read-only)
//   - envCommandHasUtility  (per-token switch: -- terminator, continue flags,
//                            -u/--chdir value skips, --split-string, VAR=value,
//                            wrapped-command default)
//
// Both helpers are pure (no I/O, no executor/provider state) and return exact
// values, so every sub-case asserts the concrete return values rather than
// "no panic".

func TestClassifyEnvCommand_BranchCoverage(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		wantOK     bool
		wantIntent ExecutionIntent // asserted only when wantOK is true
		wantReason string          // asserted only when wantOK is true; when !wantOK the reason must be empty
	}{
		// --- not-handled guard: empty command (no tokens). ---
		{
			name:    "empty command is not classified",
			command: "",
			wantOK:  false,
		},
		// --- not-handled guard: leading token is not "env". ---
		{
			name:    "non-env leading token is not classified",
			command: "ls -la",
			wantOK:  false,
		},
		// --- not-handled guard: token that merely starts with env-ish text. ---
		{
			name:    "environment assignment token is not the env command",
			command: "ENVIRONMENT=1",
			wantOK:  false,
		},

		// --- handled + read-only: bare env (no utility tokens). ---
		{
			name:       "bare env prints environment and is read-only certain",
			command:    "env",
			wantOK:     true,
			wantIntent: IntentReadOnlyCertain,
			wantReason: "bare env command prints environment only",
		},
		// classifyEnvCommand lowercases the whole command first.
		{
			name:       "uppercase ENV is treated as bare env via lowercasing",
			command:    "ENV",
			wantOK:     true,
			wantIntent: IntentReadOnlyCertain,
			wantReason: "bare env command prints environment only",
		},
		// Continue-only flags (--null/-i/etc.) leave no utility, still read-only.
		{
			name:       "env with only continue flags stays read-only",
			command:    "env -i --null --help",
			wantOK:     true,
			wantIntent: IntentReadOnlyCertain,
			wantReason: "bare env command prints environment only",
		},
		// VAR=value assignments without a trailing command stay read-only.
		{
			name:       "env with only VAR=value assignment is read-only",
			command:    "env FOO=bar",
			wantOK:     true,
			wantIntent: IntentReadOnlyCertain,
			wantReason: "bare env command prints environment only",
		},
		// "--" terminator with nothing after it is read-only.
		{
			name:       "env terminator alone is read-only",
			command:    "env --",
			wantOK:     true,
			wantIntent: IntentReadOnlyCertain,
			wantReason: "bare env command prints environment only",
		},
		// -u with a value but no trailing command is read-only.
		{
			name:       "env unset with value and no wrapped command is read-only",
			command:    "env -u FOO",
			wantOK:     true,
			wantIntent: IntentReadOnlyCertain,
			wantReason: "bare env command prints environment only",
		},

		// --- handled + write: utility detected via wrapped command / flags. ---
		{
			name:       "env terminator followed by wrapped command is write",
			command:    "env -- ls",
			wantOK:     true,
			wantIntent: IntentWriteOrUnknown,
			wantReason: "env can execute wrapped commands and is not read-only by construction",
		},
		{
			name:       "env split-string flag is write-capable",
			command:    "env -s",
			wantOK:     true,
			wantIntent: IntentWriteOrUnknown,
			wantReason: "env can execute wrapped commands and is not read-only by construction",
		},
		{
			name:       "env VAR=value then wrapped command is write",
			command:    "env FOO=bar ls",
			wantOK:     true,
			wantIntent: IntentWriteOrUnknown,
			wantReason: "env can execute wrapped commands and is not read-only by construction",
		},
		{
			name:       "env unset with value then wrapped command is write",
			command:    "env -u FOO ls",
			wantOK:     true,
			wantIntent: IntentWriteOrUnknown,
			wantReason: "env can execute wrapped commands and is not read-only by construction",
		},
		{
			name:       "env chdir=value then wrapped command is write",
			command:    "env --chdir=/tmp ls",
			wantOK:     true,
			wantIntent: IntentWriteOrUnknown,
			wantReason: "env can execute wrapped commands and is not read-only by construction",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := classifyEnvCommand(tc.command)
			if ok != tc.wantOK {
				t.Fatalf("classifyEnvCommand(%q) handled = %v, want %v", tc.command, ok, tc.wantOK)
			}
			if !tc.wantOK {
				if got.Reason != "" {
					t.Fatalf("classifyEnvCommand(%q) not-handled Reason = %q, want empty", tc.command, got.Reason)
				}
				return
			}
			if got.Intent != tc.wantIntent {
				t.Fatalf("classifyEnvCommand(%q) Intent = %v, want %v", tc.command, got.Intent, tc.wantIntent)
			}
			if got.Reason != tc.wantReason {
				t.Fatalf("classifyEnvCommand(%q) Reason = %q, want %q", tc.command, got.Reason, tc.wantReason)
			}
		})
	}
}

func TestEnvCommandHasUtility_BranchCoverage(t *testing.T) {
	tests := []struct {
		name   string
		fields []string
		want   bool
	}{
		// Loop never executes: only the leading token present.
		{name: "single token never enters loop", fields: []string{"env"}, want: false},

		// "--" terminator arm.
		{name: "terminator with wrapped command after", fields: []string{"env", "--", "ls"}, want: true},
		{name: "terminator as the final token", fields: []string{"env", "--"}, want: false},
		{name: "terminator after a continue flag is still last", fields: []string{"env", "-i", "--"}, want: false},

		// Continue-only flags (arm 2), each exercised in isolation -> loop exits -> false.
		{name: "continue flag -0", fields: []string{"env", "-0"}, want: false},
		{name: "continue flag -i", fields: []string{"env", "-i"}, want: false},
		{name: "continue flag -", fields: []string{"env", "-"}, want: false},
		{name: "continue flag --ignore-environment", fields: []string{"env", "--ignore-environment"}, want: false},
		{name: "continue flag --null", fields: []string{"env", "--null"}, want: false},
		{name: "continue flag --help", fields: []string{"env", "--help"}, want: false},
		{name: "continue flag --version", fields: []string{"env", "--version"}, want: false},
		// Continue flag followed by a bare token: the bare token hits default -> true.
		{name: "continue flag then wrapped command", fields: []string{"env", "-0", "ls"}, want: true},

		// Value-skipping flags (arm 3): -u / --unset / --chdir each consume the next arg.
		{name: "unset -u skips value then nothing follows", fields: []string{"env", "-u", "FOO"}, want: false},
		{name: "unset --unset skips value then nothing follows", fields: []string{"env", "--unset", "FOO"}, want: false},
		{name: "--chdir skips value then nothing follows", fields: []string{"env", "--chdir", "/tmp"}, want: false},
		{name: "-u skips value then wrapped command follows", fields: []string{"env", "-u", "FOO", "ls"}, want: true},
		{name: "trailing -u with no value", fields: []string{"env", "-u"}, want: false},
		{name: "chdir then unset then wrapped command", fields: []string{"env", "--unset", "FOO", "--chdir", "/tmp", "ls"}, want: true},

		// =-form prefixes (arm 4): --unset= / --chdir=.
		{name: "--unset= prefix form", fields: []string{"env", "--unset=FOO"}, want: false},
		{name: "--chdir= prefix form", fields: []string{"env", "--chdir=/tmp"}, want: false},
		{name: "--chdir= prefix then wrapped command", fields: []string{"env", "--chdir=/tmp", "ls"}, want: true},

		// Split-string arm (arm 5) -> immediately true.
		{name: "split-string short -s", fields: []string{"env", "-s"}, want: true},
		{name: "split-string long --split-string", fields: []string{"env", "--split-string"}, want: true},
		{name: "split-string prefix --split-string=", fields: []string{"env", "--split-string=|"}, want: true},

		// VAR=value arm (arm 6): contains '=' and does not start with '='.
		{name: "VAR=value assignment alone", fields: []string{"env", "FOO=bar"}, want: false},
		{name: "VAR=value then wrapped command", fields: []string{"env", "FOO=bar", "ls"}, want: true},
		{name: "empty VAR= assignment alone", fields: []string{"env", "FOO="}, want: false},

		// Default arm (arm 7): bare wrapped command token -> true.
		{name: "bare wrapped command default arm", fields: []string{"env", "ls"}, want: true},
		{name: "unknown token falls to default", fields: []string{"env", "frobnicate"}, want: true},
		// Token starting with '=' is NOT treated as VAR=value; it falls through to default.
		{name: "leading equals token falls to default", fields: []string{"env", "=FOO"}, want: true},

		// Quote-trimming path: surrounding " or ' are stripped before matching.
		{name: "quoted continue flag trims to -i", fields: []string{"env", `"-i"`}, want: false},
		{name: "single-quoted VAR=value trims to assignment", fields: []string{"env", "'FOO=bar'"}, want: false},
		{name: "quoted split-string trims to -s and is write", fields: []string{"env", `'-s'`}, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := envCommandHasUtility(tc.fields)
			if got != tc.want {
				t.Fatalf("envCommandHasUtility(%v) = %v, want %v", tc.fields, got, tc.want)
			}
		})
	}
}
