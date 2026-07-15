package qualification

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// This file is a white-box table-test suite for the pure manifest predicate
// helpers in manifest.go: validatePredicates and positiveDuration. Every
// helper introduced here is prefixed qualmp so it cannot collide with
// identifiers defined by sibling test files in package qualification.

// qualmpAliases builds the alias set from the supplied resource aliases. It
// mirrors how Manifest.Validate constructs the set before calling
// validatePredicates, but keeps the table rows compact and literal.
func qualmpAliases(names ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))
	for _, n := range names {
		out[n] = struct{}{}
	}
	return out
}

// qualmpJoinErrs flattens a validatePredicates result into a single
// newline-delimited string so table rows can assert on substrings the same
// way the rest of this package checks joined errors.
func qualmpJoinErrs(errs []error) string {
	var b strings.Builder
	for _, e := range errs {
		b.WriteString(e.Error())
		b.WriteByte('\n')
	}
	return b.String()
}

func TestValidatePredicates(t *testing.T) {
	cases := []struct {
		name       string
		label      string
		predicates []Predicate
		aliases    map[string]struct{}
		wantCount  int
		wantSubs   []string
	}{
		// for-range not entered: nil slice yields no errors.
		{
			name:       "nil predicate slice returns no errors",
			label:      "baseline",
			predicates: nil,
			aliases:    qualmpAliases("target"),
			wantCount:  0,
		},
		// for-range not entered: empty slice yields no errors.
		{
			name:       "empty predicate slice returns no errors",
			label:      "baseline",
			predicates: []Predicate{},
			aliases:    qualmpAliases("target"),
			wantCount:  0,
		},
		// Happy path: known target, valid probe, valid operator, valid value,
		// no timeout. Exercises every ok/default arm with the ok branch.
		{
			name:  "fully valid predicate with no timeout returns no errors",
			label: "baseline",
			predicates: []Predicate{{
				Probe: "docker.running", Target: "target", Operator: "eq",
				Value: json.RawMessage("true"),
			}},
			aliases:   qualmpAliases("target"),
			wantCount: 0,
		},
		// Happy path with a non-empty, parseable, positive timeout: exercises
		// the predicate.Timeout != "" branch and the positiveDuration
		// success arm (err == nil) inside validatePredicates.
		{
			name:  "valid predicate with positive timeout returns no errors",
			label: "teardown",
			predicates: []Predicate{{
				Probe: "docker.running", Target: "target", Operator: "eq",
				Value: json.RawMessage("false"), Timeout: "5s",
			}},
			aliases:   qualmpAliases("target"),
			wantCount: 0,
		},
		// Accepted JSON value shapes: confirm len>0 && json.Valid for numbers,
		// null, arrays and objects so the invalid-value arm is not taken.
		{
			name:  "numeric value is accepted",
			label: "baseline",
			predicates: []Predicate{{
				Probe: "docker.restart_count", Target: "target", Operator: "eq",
				Value: json.RawMessage("3"),
			}},
			aliases:   qualmpAliases("target"),
			wantCount: 0,
		},
		{
			name:  "array value accepted for in operator",
			label: "baseline",
			predicates: []Predicate{{
				Probe: "docker.status", Target: "target", Operator: "in",
				Value: json.RawMessage(`["running","healthy"]`),
			}},
			aliases:   qualmpAliases("target"),
			wantCount: 0,
		},
		// Unknown target arm: aliases lookup misses.
		{
			name:  "target not in aliases reports unknown resource",
			label: "baseline",
			predicates: []Predicate{{
				Probe: "docker.running", Target: "ghost", Operator: "eq",
				Value: json.RawMessage("true"),
			}},
			aliases:   qualmpAliases("target"),
			wantCount: 1,
			wantSubs:  []string{`baseline[0] targets unknown resource "ghost"`},
		},
		// Unsupported probe default arm.
		{
			name:  "unsupported probe is reported",
			label: "baseline",
			predicates: []Predicate{{
				Probe: "tcp.port_open", Target: "target", Operator: "eq",
				Value: json.RawMessage("true"),
			}},
			aliases:   qualmpAliases("target"),
			wantCount: 1,
			wantSubs:  []string{`baseline[0] has unsupported probe "tcp.port_open"`},
		},
		// Unsupported operator default arm.
		{
			name:  "unsupported operator is reported",
			label: "baseline",
			predicates: []Predicate{{
				Probe: "docker.running", Target: "target", Operator: "matches",
				Value: json.RawMessage("true"),
			}},
			aliases:   qualmpAliases("target"),
			wantCount: 1,
			wantSubs:  []string{`baseline[0] has unsupported operator "matches"`},
		},
		// Empty value arm: len(predicate.Value) == 0.
		{
			name:  "empty value is reported as invalid",
			label: "baseline",
			predicates: []Predicate{{
				Probe: "docker.running", Target: "target", Operator: "eq",
				Value: json.RawMessage(""),
			}},
			aliases:   qualmpAliases("target"),
			wantCount: 1,
			wantSubs:  []string{"baseline[0] has invalid value"},
		},
		// Nil value arm: a nil json.RawMessage has len 0.
		{
			name:  "nil value is reported as invalid",
			label: "baseline",
			predicates: []Predicate{{
				Probe: "docker.running", Target: "target", Operator: "eq",
				Value: nil,
			}},
			aliases:   qualmpAliases("target"),
			wantCount: 1,
			wantSubs:  []string{"baseline[0] has invalid value"},
		},
		// Invalid JSON arm: non-empty bytes that fail json.Valid.
		{
			name:  "malformed json value is reported as invalid",
			label: "baseline",
			predicates: []Predicate{{
				Probe: "docker.running", Target: "target", Operator: "eq",
				Value: json.RawMessage("{bad"),
			}},
			aliases:   qualmpAliases("target"),
			wantCount: 1,
			wantSubs:  []string{"baseline[0] has invalid value"},
		},
		// Timeout arm with unparseable duration: positiveDuration parse error.
		{
			name:  "unparseable timeout is wrapped",
			label: "teardown",
			predicates: []Predicate{{
				Probe: "docker.running", Target: "target", Operator: "eq",
				Value: json.RawMessage("true"), Timeout: "not-a-duration",
			}},
			aliases:   qualmpAliases("target"),
			wantCount: 1,
			wantSubs:  []string{"teardown[0] timeout:"},
		},
		// Timeout arm with non-positive (zero) duration.
		{
			name:  "zero timeout is rejected as non-positive",
			label: "teardown",
			predicates: []Predicate{{
				Probe: "docker.running", Target: "target", Operator: "eq",
				Value: json.RawMessage("true"), Timeout: "0s",
			}},
			aliases:   qualmpAliases("target"),
			wantCount: 1,
			wantSubs:  []string{"teardown[0] timeout:", "duration must be positive"},
		},
		// Timeout arm with negative duration.
		{
			name:  "negative timeout is rejected as non-positive",
			label: "teardown",
			predicates: []Predicate{{
				Probe: "docker.running", Target: "target", Operator: "eq",
				Value: json.RawMessage("true"), Timeout: "-3s",
			}},
			aliases:   qualmpAliases("target"),
			wantCount: 1,
			wantSubs:  []string{"teardown[0] timeout:", "duration must be positive"},
		},
		// Index propagation: a bad predicate at position 1 reports [1], and a
		// valid predicate at position 0 contributes nothing.
		{
			name:  "error reports the offending slice index",
			label: "teardown",
			predicates: []Predicate{
				{Probe: "docker.running", Target: "target", Operator: "eq", Value: json.RawMessage("true")},
				{Probe: "docker.running", Target: "missing", Operator: "eq", Value: json.RawMessage("true")},
			},
			aliases:   qualmpAliases("target"),
			wantCount: 1,
			wantSubs:  []string{`teardown[1] targets unknown resource "missing"`},
		},
		// Accumulation: every guard in one predicate produces five independent
		// errors for a single element, confirming append-only behaviour.
		{
			name:  "all guards fail on one predicate yielding five errors",
			label: "fault f1 oracle",
			predicates: []Predicate{{
				Probe: "bogus.probe", Target: "ghost", Operator: "matches",
				Value: json.RawMessage("{bad"), Timeout: "nope",
			}},
			aliases:   qualmpAliases("target"),
			wantCount: 5,
			wantSubs: []string{
				`fault f1 oracle[0] targets unknown resource "ghost"`,
				`fault f1 oracle[0] has unsupported probe "bogus.probe"`,
				`fault f1 oracle[0] has unsupported operator "matches"`,
				"fault f1 oracle[0] has invalid value",
				"fault f1 oracle[0] timeout:",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := validatePredicates(tc.label, tc.predicates, tc.aliases)
			if len(errs) != tc.wantCount {
				t.Fatalf("validatePredicates returned %d errors, want %d: %v", len(errs), tc.wantCount, qualmpJoinErrs(errs))
			}
			if len(tc.wantSubs) == 0 {
				return
			}
			joined := qualmpJoinErrs(errs)
			for _, sub := range tc.wantSubs {
				if !strings.Contains(joined, sub) {
					t.Errorf("validatePredicates error text = %q, want substring %q", joined, sub)
				}
			}
		})
	}
}

func TestPositiveDuration(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		wantErr bool
		wantDur time.Duration
		errSub  string
	}{
		// Success arm: positive duration parses and is returned verbatim.
		{"seconds parse", "5s", false, 5 * time.Second, ""},
		{"compound duration parses", "1m30s", false, 90 * time.Second, ""},
		{"milliseconds parse", "250ms", false, 250 * time.Millisecond, ""},
		// Parse error arm: empty string cannot be parsed.
		{"empty string parse error", "", true, 0, ""},
		// Parse error arm: missing unit.
		{"bare number missing unit parse error", "5", true, 0, "missing unit"},
		// Parse error arm: unparseable text.
		{"garbage parse error", "not-a-duration", true, 0, "invalid duration"},
		// Non-positive arm: zero duration parses but is rejected.
		{"zero duration rejected", "0s", true, 0, "duration must be positive"},
		{"zero without unit rejected", "0", true, 0, "duration must be positive"},
		// Non-positive arm: negative duration parses but is rejected.
		{"negative duration rejected", "-3s", true, 0, "duration must be positive"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := positiveDuration(tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("positiveDuration(%q) err = nil, want non-nil", tc.value)
				}
				if tc.errSub != "" && !strings.Contains(err.Error(), tc.errSub) {
					t.Fatalf("positiveDuration(%q) err = %q, want substring %q", tc.value, err.Error(), tc.errSub)
				}
				if got != 0 {
					t.Fatalf("positiveDuration(%q) = %v, want 0 on error path", tc.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("positiveDuration(%q) err = %v, want nil", tc.value, err)
			}
			if got != tc.wantDur {
				t.Fatalf("positiveDuration(%q) = %v, want %v", tc.value, got, tc.wantDur)
			}
		})
	}
}
