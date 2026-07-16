package monitoring

import (
	"testing"
	"time"
)

// This file adds branch-coverage tests for the small pure helper functions in
// agent_fleet_doctor.go:
//   - identitySplitReason
//   - sameAgentIdentity
//   - hasType
//   - nonEmptyStrings
//   - roundDuration
//
// It targets genuinely-uncovered branches (both sides of each conditional,
// short-circuit ordering, empty/nil/zero and boundary inputs) and asserts real
// behaviour using only the package's own (same-package, unexported) functions.
// Tests are prefixed with BranchCov so `-run BranchCov` selects them.

// identitySplitReason always emits the two base evidence lines ("Peer type:"
// and "Peer ID:") and conditionally appends agent/token lines only when those
// values are non-empty. The returned Code/Severity/Message are invariant.
func TestBranchCovIdentitySplitReason(t *testing.T) {
	t.Parallel()

	const wantCode = "agent_identity_split"
	const wantSeverity = AgentFleetStatusWarning
	const wantMessage = "Host and workload telemetry appear to belong to the same machine but are reporting as separate agent identities."

	cases := []struct {
		name               string
		peerType           string
		peerID             string
		peerAgentID        string
		peerTokenID        string
		wantEvidenceLength int
	}{
		// Both optional branches skipped: only the two base evidence lines.
		{name: "both ids empty -> base evidence only", peerType: "Docker", peerID: "docker-1", peerAgentID: "", peerTokenID: "", wantEvidenceLength: 2},

		// Only the agent-id branch taken.
		{name: "only agent id present", peerType: "Host", peerID: "host-1", peerAgentID: "agent-9", peerTokenID: "", wantEvidenceLength: 3},

		// Only the token-id branch taken (independent of the agent-id branch).
		{name: "only token id present", peerType: "Docker", peerID: "docker-2", peerAgentID: "", peerTokenID: "tok-3", wantEvidenceLength: 3},

		// Both optional branches taken.
		{name: "both ids present", peerType: "Host", peerID: "host-2", peerAgentID: "agent-1", peerTokenID: "tok-1", wantEvidenceLength: 4},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reason := identitySplitReason(tc.peerType, tc.peerID, tc.peerAgentID, tc.peerTokenID)

			if reason.Code != wantCode {
				t.Fatalf("Code = %q, want %q", reason.Code, wantCode)
			}
			if reason.Severity != wantSeverity {
				t.Fatalf("Severity = %q, want %q", reason.Severity, wantSeverity)
			}
			if reason.Message != wantMessage {
				t.Fatalf("Message = %q, want %q", reason.Message, wantMessage)
			}

			// The two base evidence lines are always present and carry the
			// supplied peerType/peerID verbatim.
			if len(reason.Evidence) != tc.wantEvidenceLength {
				t.Fatalf("len(Evidence) = %d, want %d (%#v)", len(reason.Evidence), tc.wantEvidenceLength, reason.Evidence)
			}
			if got := reason.Evidence[0]; got != "Peer type: "+tc.peerType {
				t.Fatalf("Evidence[0] = %q, want %q", got, "Peer type: "+tc.peerType)
			}
			if got := reason.Evidence[1]; got != "Peer ID: "+tc.peerID {
				t.Fatalf("Evidence[1] = %q, want %q", got, "Peer ID: "+tc.peerID)
			}

			// When the optional IDs are supplied they must be appended, in
			// order, after the base lines.
			offset := 2
			if tc.peerAgentID != "" {
				if got := reason.Evidence[offset]; got != "Peer agent ID: "+tc.peerAgentID {
					t.Fatalf("agent evidence = %q, want %q", got, "Peer agent ID: "+tc.peerAgentID)
				}
				offset++
			}
			if tc.peerTokenID != "" {
				if got := reason.Evidence[offset]; got != "Peer token ID: "+tc.peerTokenID {
					t.Fatalf("token evidence = %q, want %q", got, "Peer token ID: "+tc.peerTokenID)
				}
			}
		})
	}
}

// TestBranchCovIdentitySplitReasonEmptyBaseLines documents real behaviour: the
// base peerType/peerID evidence lines are emitted unconditionally even when
// those values are empty, producing "Peer type: " / "Peer ID: " entries.
func TestBranchCovIdentitySplitReasonEmptyBaseLines(t *testing.T) {
	t.Parallel()

	reason := identitySplitReason("", "", "", "")
	if len(reason.Evidence) != 2 {
		t.Fatalf("len(Evidence) = %d, want 2 (%#v)", len(reason.Evidence), reason.Evidence)
	}
	if reason.Evidence[0] != "Peer type: " || reason.Evidence[1] != "Peer ID: " {
		t.Fatalf("empty base evidence = %#v, want [\"Peer type: \" \"Peer ID: \"]", reason.Evidence)
	}
}

// sameAgentIdentity short-circuits through three independent identity signals
// (agentID, tokenID, hostname). These cases pin down each branch, the
// short-circuit priority, the empty-guard fall-throughs, and the
// case-insensitive hostname comparison.
func TestBranchCovSameAgentIdentity(t *testing.T) {
	t.Parallel()

	mkSubject := func(agentID, tokenID, hostname string) agentFleetSubject {
		return agentFleetSubject{agentID: agentID, tokenID: tokenID, hostname: hostname}
	}

	cases := []struct {
		name             string
		subject          agentFleetSubject
		id               string
		agentID          string
		hostname         string
		tokenID          string
		want             bool
		descriptionOfHit string
	}{
		// agentID branch: match via the agentID argument.
		{name: "agent id matches agentID arg", subject: mkSubject("a1", "", ""), id: "x", agentID: "a1", hostname: "", tokenID: "", want: true},

		// agentID branch: match via the id argument (the second operand of the
		// short-circuit OR), agentID arg deliberately differs.
		{name: "agent id matches id arg", subject: mkSubject("a1", "", ""), id: "a1", agentID: "other", hostname: "", tokenID: "", want: true},

		// agentID guard: subject.agentID empty -> falls through to token/hostname.
		{name: "subject agentID empty falls through to false", subject: mkSubject("", "", ""), id: "x", agentID: "y", hostname: "", tokenID: "", want: false},

		// agentID guard: subject.agentID set but matches neither arg -> falls
		// through to token, which then matches.
		{name: "agentID no match falls to token match", subject: mkSubject("a1", "t1", ""), id: "x", agentID: "y", hostname: "", tokenID: "t1", want: true},

		// tokenID branch: match when subject & peer both non-empty and equal.
		{name: "token id match", subject: mkSubject("", "t1", ""), id: "", agentID: "", hostname: "", tokenID: "t1", want: true},

		// tokenID guard: subject.tokenID empty -> skip (hostname empty -> false).
		{name: "subject token empty skip -> false", subject: mkSubject("", "", ""), id: "", agentID: "", hostname: "h", tokenID: "t1", want: false},

		// tokenID guard: peer tokenID empty -> skip even when subject token set,
		// then fall through to a hostname match.
		{name: "peer token empty skip -> falls to hostname match", subject: mkSubject("", "t1", "h"), id: "", agentID: "", hostname: "h", tokenID: "", want: true},

		// tokenID guard: peer tokenID empty -> skip and subject has no hostname
		// to fall through to, so overall false.
		{name: "peer token empty skip -> no hostname -> false", subject: mkSubject("", "t1", ""), id: "", agentID: "", hostname: "h", tokenID: "", want: false},

		// tokenID comparison: both non-empty but unequal -> skip.
		{name: "tokens differ skip -> false", subject: mkSubject("", "t1", ""), id: "", agentID: "", hostname: "", tokenID: "t2", want: false},

		// hostname branch: case-insensitive match.
		{name: "hostname case-insensitive match", subject: mkSubject("", "", "Host-A"), id: "", agentID: "", hostname: "host-a", tokenID: "", want: true},

		// hostname guard: subject.hostname empty -> skip -> false.
		{name: "subject hostname empty -> false", subject: mkSubject("", "", ""), id: "", agentID: "", hostname: "host-a", tokenID: "", want: false},

		// hostname comparison: peer hostname empty while subject set ->
		// EqualFold returns false -> overall false.
		{name: "peer hostname empty subject set -> false", subject: mkSubject("", "", "host-a"), id: "", agentID: "", hostname: "", tokenID: "", want: false},

		// All signals absent -> false.
		{name: "all empty -> false", subject: mkSubject("", "", ""), id: "", agentID: "", hostname: "", tokenID: "", want: false},

		// Short-circuit priority: agentID match wins even when token and
		// hostname would disagree.
		{name: "agentID match priority ignores token/hostname mismatch", subject: mkSubject("a1", "t1", "real-host"), id: "x", agentID: "a1", hostname: "different", tokenID: "tX", want: true},

		// Short-circuit priority: token match wins even when hostname would
		// disagree (agentID empty so token branch is reached).
		{name: "token match priority ignores hostname mismatch", subject: mkSubject("", "t1", "real-host"), id: "", agentID: "", hostname: "different", tokenID: "t1", want: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := sameAgentIdentity(tc.subject, tc.id, tc.agentID, tc.hostname, tc.tokenID)
			if got != tc.want {
				t.Fatalf("sameAgentIdentity(%+v, id=%q, agentID=%q, hostname=%q, tokenID=%q) = %v, want %v",
					tc.subject, tc.id, tc.agentID, tc.hostname, tc.tokenID, got, tc.want)
			}
		})
	}
}

// hasType is a thin map-membership check. Cover present, absent, nil-map and
// empty-string-key edges (a nil map lookup is well-defined in Go and returns
// ok=false).
func TestBranchCovHasType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		types map[string]struct{}
		kind  string
		want  bool
	}{
		{name: "present key -> true", types: map[string]struct{}{"docker": {}}, kind: "docker", want: true},
		{name: "absent key -> false", types: map[string]struct{}{"host": {}}, kind: "docker", want: false},
		{name: "nil map -> false", types: nil, kind: "docker", want: false},
		{name: "empty map -> false", types: map[string]struct{}{}, kind: "docker", want: false},
		{name: "empty key present -> true", types: map[string]struct{}{"": {}}, kind: "", want: true},
		{name: "empty key absent -> false", types: map[string]struct{}{"docker": {}}, kind: "", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := hasType(tc.types, tc.kind); got != tc.want {
				t.Fatalf("hasType(%#v, %q) = %v, want %v", tc.types, tc.kind, got, tc.want)
			}
		})
	}
}

// nonEmptyStrings trims whitespace from each arg and drops empties. It always
// returns a non-nil slice (it is initialised with make), including for a
// zero-arg / all-empty invocation.
func TestBranchCovNonEmptyStrings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  []string
		want   []string
		wantOK bool // always true: result must be non-nil
	}{
		{name: "no args -> non-nil empty", input: nil, want: []string{}, wantOK: true},
		{name: "all empty/whitespace -> non-nil empty", input: []string{"", " ", "\t"}, want: []string{}, wantOK: true},
		{name: "mix keeps non-empty and trims", input: []string{"a", "", " b ", "\t", "c"}, want: []string{"a", "b", "c"}, wantOK: true},
		{name: "all non-empty", input: []string{"x", "y"}, want: []string{"x", "y"}, wantOK: true},
		{name: "whitespace-only excluded", input: []string{"   "}, want: []string{}, wantOK: true},
		{name: "leading/trailing whitespace trimmed", input: []string{"  hello  "}, want: []string{"hello"}, wantOK: true},
		{name: "internal whitespace preserved", input: []string{"keep me"}, want: []string{"keep me"}, wantOK: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := nonEmptyStrings(tc.input...)
			if got == nil {
				t.Fatalf("nonEmptyStrings(%v) returned nil; want non-nil slice", tc.input)
			}
			if !stringSlicesEqual(got, tc.want) {
				t.Fatalf("nonEmptyStrings(%v) = %#v, want %#v", tc.input, got, tc.want)
			}
		})
	}
}

// roundDuration takes one of two formatting paths split at exactly one second.
// Durations strictly below one second are stringified verbatim; durations of at
// least one second are rounded to whole seconds first (Go rounds ties away from
// zero). Negative durations are always less than time.Second and therefore take
// the verbatim branch.
func TestBranchCovRoundDuration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		// Sub-second branch (duration < time.Second): verbatim .String().
		{name: "zero", duration: 0, want: "0s"},
		{name: "one millisecond", duration: time.Millisecond, want: "1ms"},
		{name: "five hundred ms", duration: 500 * time.Millisecond, want: "500ms"},
		{name: "just under one second", duration: 999 * time.Millisecond, want: "999ms"},

		// Boundary: exactly one second is NOT < time.Second, so it takes the
		// rounding branch and stays "1s".
		{name: "exactly one second via round branch", duration: time.Second, want: "1s"},

		// Rounding branch (>= 1s): whole-second .String() after rounding.
		{name: "rounds down 1.4s", duration: 1400 * time.Millisecond, want: "1s"},
		{name: "rounds half away from zero 1.5s -> 2s", duration: 1500 * time.Millisecond, want: "2s"},
		{name: "rounds 2.6s -> 3s", duration: 2600 * time.Millisecond, want: "3s"},
		{name: "truncated sub-second over one second", duration: 2300 * time.Millisecond, want: "2s"},
		{name: "ninety seconds composes m+s", duration: 90 * time.Second, want: "1m30s"},
		{name: "compound h+m+s", duration: time.Hour + 2*time.Minute + 3*time.Second, want: "1h2m3s"},

		// Negative durations are always < time.Second (positive), so they take
		// the verbatim branch regardless of magnitude.
		{name: "negative sub-second verbatim", duration: -500 * time.Millisecond, want: "-500ms"},
		{name: "negative whole second still verbatim", duration: -2 * time.Second, want: "-2s"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := roundDuration(tc.duration); got != tc.want {
				t.Fatalf("roundDuration(%v) = %q, want %q", tc.duration, got, tc.want)
			}
		})
	}
}
