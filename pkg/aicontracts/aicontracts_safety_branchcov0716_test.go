package aicontracts

import "testing"

// TestBranchCovIsDestructiveCommand exercises IsDestructiveCommand directly.
// IsDestructiveCommand delegates to IsBlockedCommand, whose branches are:
// the empty-input early return, the per-pattern substring match (hit), and
// the fall-through no-match return. It also depends on normalizeCommandForCheck
// (quote/escape stripping + whitespace collapsing) and case folding, all of
// which are covered here with inputs the sibling ClassifyAutomationRisk test
// never reaches directly.
func TestBranchCovIsDestructiveCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{
			name:    "empty input short circuits to false",
			command: "",
			want:    false,
		},
		{
			name:    "direct blocked pattern match returns true",
			command: "rm -rf /tmp/pulse-data",
			want:    true,
		},
		{
			name:    "benign command falls through all patterns to false",
			command: "uptime",
			want:    false,
		},
		{
			name:    "uppercase blocked pattern still matches case insensitively",
			command: "RM -RF /x",
			want:    true,
		},
		{
			name:    "single quoted command word is normalized before matching",
			command: "'rm' -rf /x",
			want:    true,
		},
		{
			name:    "double quoted token is normalized before matching",
			command: `"shutdown" now`,
			want:    true,
		},
		{
			name:    "backslash escaped command is normalized before matching",
			command: `\rm -rf /x`,
			want:    true,
		},
		{
			name:    "backtick wrapped command is normalized before matching",
			command: "`mkfs`",
			want:    true,
		},
		{
			name:    "tab between flag tokens collapses to single space then matches",
			command: "rm\t-rf /x",
			want:    true,
		},
		{
			name:    "runs of spaces collapse to single space then match",
			command: "rm     -rf /x",
			want:    true,
		},
		{
			name:    "sql drop table matches case insensitively inside quoted arg",
			command: "psql -c 'drop table users'",
			want:    true,
		},
		{
			name:    "substring overmatch flags benign word containing blocked token",
			command: "echo information",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDestructiveCommand(tt.command); got != tt.want {
				t.Fatalf("IsDestructiveCommand(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

// TestBranchCovIsReadOnlyPart exercises isReadOnlyPart directly. Its branches
// are the per-pattern prefix match (hit) and the fall-through no-match return.
// It folds case and trims leading/trailing whitespace before matching.
func TestBranchCovIsReadOnlyPart(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{
			name: "empty input matches no pattern",
			cmd:  "",
			want: false,
		},
		{
			name: "whitespace only trims to empty and matches nothing",
			cmd:  "   ",
			want: false,
		},
		{
			name: "prefixed pattern with trailing space variant matches",
			cmd:  "ls -la",
			want: true,
		},
		{
			name: "bare single token pattern matches",
			cmd:  "ls",
			want: true,
		},
		{
			name: "case insensitive prefix match",
			cmd:  "LS -LA",
			want: true,
		},
		{
			name: "leading whitespace is trimmed before prefix match",
			cmd:  "   cat /etc/hosts",
			want: true,
		},
		{
			name: "bare env token pattern matches",
			cmd:  "env",
			want: true,
		},
		{
			name: "non read only command falls through to false",
			cmd:  "rm -rf /tmp",
			want: false,
		},
		{
			name: "command sharing no read only prefix is false",
			cmd:  "vim /etc/hosts",
			want: false,
		},
		{
			name: "single char w pattern overmatches commands starting with w",
			cmd:  "wget -O- http://example/payload",
			want: true,
		},
		{
			name: "env pattern overmatches command prefixed with env",
			cmd:  "environment-frobber --delete-everything",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isReadOnlyPart(tt.cmd); got != tt.want {
				t.Fatalf("isReadOnlyPart(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

// TestBranchCovIsSafePipeCommand exercises isSafePipeCommand directly. Its
// branches are: per-pattern prefix match (hit, return true), the special case
// where a matched command is sed and contains -i (return false), and the
// fall-through no-match return. Case folding and whitespace trimming precede
// matching.
func TestBranchCovIsSafePipeCommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{
			name: "empty input matches no pattern",
			cmd:  "",
			want: false,
		},
		{
			name: "whitespace only trims to empty and matches nothing",
			cmd:  "   ",
			want: false,
		},
		{
			name: "grep prefix matches",
			cmd:  "grep foo",
			want: true,
		},
		{
			name: "fgrep with i flag is not treated like sed in place",
			cmd:  "fgrep -i foo",
			want: true,
		},
		{
			name: "sed without in place flag is safe",
			cmd:  "sed 's/a/b/'",
			want: true,
		},
		{
			name: "sed bare token with no flag is safe",
			cmd:  "sed",
			want: true,
		},
		{
			name: "sed with short in place flag is rejected",
			cmd:  "sed -i.bak file",
			want: false,
		},
		{
			name: "sed with long in place flag rejected via i substring",
			cmd:  "sed --in-place file",
			want: false,
		},
		{
			name: "uppercase sed with in place flag still rejected after fold",
			cmd:  "SED -I file",
			want: false,
		},
		{
			name: "sort with flags matches",
			cmd:  "sort -u",
			want: true,
		},
		{
			name: "wc token matches",
			cmd:  "wc -l",
			want: true,
		},
		{
			name: "tee to dev null multi word pattern matches",
			cmd:  "tee /dev/null",
			want: true,
		},
		{
			name: "tee to real file is not in safe pattern set",
			cmd:  "tee /tmp/log",
			want: false,
		},
		{
			name: "xargs echo multi word pattern matches",
			cmd:  "xargs echo",
			want: true,
		},
		{
			name: "xargs with destructive payload is not the safe variant",
			cmd:  "xargs rm",
			want: false,
		},
		{
			name: "read only command not in pipe safe set is rejected",
			cmd:  "cat foo",
			want: false,
		},
		{
			name: "non pipe safe command falls through to false",
			cmd:  "curl http://example",
			want: false,
		},
		{
			name: "case insensitive prefix match",
			cmd:  "GREP foo",
			want: true,
		},
		{
			name: "leading whitespace trimmed before prefix match",
			cmd:  "   head -5",
			want: true,
		},
		{
			name: "awk in place edit is classified safe despite being destructive",
			cmd:  "awk -i inplace file",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSafePipeCommand(tt.cmd); got != tt.want {
				t.Fatalf("isSafePipeCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}
