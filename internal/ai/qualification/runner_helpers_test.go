package qualification

import (
	"testing"
	"time"
)

// This file is a white-box table-test suite for the small pure helpers in
// runner.go and lab.go: phaseDuration, dockerTargetLabel, and commandSummary.
// Every helper and shared var introduced here is prefixed with `qualrunner` to
// avoid collisions with sibling test files in package qualification.

// qualrunnerPhase is a small constructor keeping table rows terse while letting
// each row spell out its own duration explicitly.
func qualrunnerPhase(name string, duration time.Duration) PhaseTiming {
	return PhaseTiming{Name: name, Duration: duration}
}

func TestPhaseDurationLinearScan(t *testing.T) {
	cases := []struct {
		name   string
		phases []PhaseTiming
		lookup string
		want   time.Duration
	}{
		{
			name:   "nil slice returns zero",
			phases: nil,
			lookup: "anything",
			want:   0,
		},
		{
			name:   "empty slice returns zero",
			phases: []PhaseTiming{},
			lookup: "anything",
			want:   0,
		},
		{
			name:   "name absent returns zero",
			phases: []PhaseTiming{qualrunnerPhase("preflight", 1*time.Second)},
			lookup: "does-not-exist",
			want:   0,
		},
		{
			name:   "first and only element matches",
			phases: []PhaseTiming{qualrunnerPhase("preflight", 250*time.Millisecond)},
			lookup: "preflight",
			want:   250 * time.Millisecond,
		},
		{
			name: "match found later in slice exercises linear scan",
			phases: []PhaseTiming{
				qualrunnerPhase("preflight", 1*time.Second),
				qualrunnerPhase("provision_and_baseline", 2*time.Second),
				qualrunnerPhase("real_model_patrol", 3*time.Second),
			},
			lookup: "real_model_patrol",
			want:   3 * time.Second,
		},
		{
			name: "first occurrence wins when name duplicated",
			phases: []PhaseTiming{
				qualrunnerPhase("revert_and_verify", 5*time.Second),
				qualrunnerPhase("revert_and_verify", 999*time.Second),
			},
			lookup: "revert_and_verify",
			want:   5 * time.Second,
		},
		{
			name:   "matching phase with explicit zero duration returns zero",
			phases: []PhaseTiming{qualrunnerPhase("preflight", 0)},
			lookup: "preflight",
			want:   0,
		},
		{
			name:   "negative duration is returned verbatim not clamped",
			phases: []PhaseTiming{qualrunnerPhase("preflight", -7*time.Second)},
			lookup: "preflight",
			want:   -7 * time.Second,
		},
		{
			name:   "empty lookup name never matches a populated phase",
			phases: []PhaseTiming{qualrunnerPhase("preflight", 1*time.Second)},
			lookup: "",
			want:   0,
		},
		{
			name:   "empty name phase can be matched by empty lookup",
			phases: []PhaseTiming{qualrunnerPhase("", 42*time.Millisecond)},
			lookup: "",
			want:   42 * time.Millisecond,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := phaseDuration(tc.phases, tc.lookup)
			if got != tc.want {
				t.Fatalf("phaseDuration(%v, %q) = %v, want %v", tc.phases, tc.lookup, got, tc.want)
			}
		})
	}
}

func TestDockerTargetLabelBranches(t *testing.T) {
	cases := []struct {
		name   string
		target DockerTarget
		want   string
	}{
		{
			name:   "ssh host wins when set",
			target: DockerTarget{SSHHost: "lab.example.com", Context: "ignored"},
			want:   "ssh:lab.example.com",
		},
		{
			name:   "ssh host only",
			target: DockerTarget{SSHHost: "user@host"},
			want:   "ssh:user@host",
		},
		{
			name:   "context used when ssh host empty",
			target: DockerTarget{Context: "colima"},
			want:   "context:colima",
		},
		{
			name:   "empty target yields bare context prefix",
			target: DockerTarget{},
			want:   "context:",
		},
		{
			name:   "context empty but ssh host set still uses ssh",
			target: DockerTarget{SSHHost: "bridge"},
			want:   "ssh:bridge",
		},
		{
			name:   "allow shared host flag does not affect label",
			target: DockerTarget{Context: "default", AllowSharedHost: true},
			want:   "context:default",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := dockerTargetLabel(tc.target)
			if got != tc.want {
				t.Fatalf("dockerTargetLabel(%+v) = %q, want %q", tc.target, got, tc.want)
			}
		})
	}
}

func TestCommandSummaryTruncation(t *testing.T) {
	cases := []struct {
		name string
		in   string
		args []string
		want string
	}{
		{
			name: "nil args yields name only",
			in:   "docker",
			args: nil,
			want: "docker",
		},
		{
			name: "empty args yields name only",
			in:   "docker",
			args: []string{},
			want: "docker",
		},
		{
			name: "empty name joined with args",
			in:   "",
			args: []string{"ps"},
			want: " ps",
		},
		{
			name: "single arg joined",
			in:   "git",
			args: []string{"status"},
			want: "git status",
		},
		{
			name: "exactly six args not truncated",
			in:   "docker",
			args: []string{"run", "-d", "--name", "svc", "--network", "net"},
			want: "docker run -d --name svc --network net",
		},
		{
			name: "seven args truncated to first six",
			in:   "docker",
			args: []string{"run", "-d", "--name", "svc", "--network", "net", "image"},
			want: "docker run -d --name svc --network net",
		},
		{
			name: "many args truncated to first six preserving order",
			in:   "ssh",
			args: []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=15", "host", "docker", "ps", "-aq", "--no-trunc"},
			want: "ssh -o BatchMode=yes -o ConnectTimeout=15 host docker",
		},
		{
			name: "five args under boundary passed through",
			in:   "docker",
			args: []string{"network", "create", "--label", "k=v"},
			want: "docker network create --label k=v",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := commandSummary(tc.in, tc.args)
			if got != tc.want {
				t.Fatalf("commandSummary(%q, %v) = %q, want %q", tc.in, tc.args, got, tc.want)
			}
		})
	}
}
