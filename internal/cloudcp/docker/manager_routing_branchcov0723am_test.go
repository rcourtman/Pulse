package docker

import (
	"errors"
	"fmt"
	"testing"

	"github.com/containerd/errdefs"
)

// customNotFoundErr implements errdefs' private notFound interface (a method
// named NotFound()) without embedding errdefs.ErrNotFound, so it exercises the
// isInterface arm of errdefs.IsNotFound rather than the errors.Is arm.
type customNotFoundErr struct{ msg string }

func (c customNotFoundErr) Error() string { return "missing: " + c.msg }
func (c customNotFoundErr) NotFound()     {}

func TestBranchcov0723AmIsNotFound(t *testing.T) {
	t.Parallel()

	notFoundWrapped := fmt.Errorf("inspect image pulse:test: %w", errdefs.ErrNotFound)

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error is not not-found", err: nil, want: false},
		{name: "plain errors.New is not not-found", err: errors.New("boom"), want: false},
		{name: "verbatim errdefs.ErrNotFound matches", err: errdefs.ErrNotFound, want: true},
		{name: "wrapped errdefs.ErrNotFound via fmt.Errorf %w matches", err: notFoundWrapped, want: true},
		{name: "unrelated typed errdefs error does not match", err: errdefs.ErrAlreadyExists, want: false},
		{name: "similarly-worded string error must not match", err: errors.New("not found: image pulse:test"), want: false},
		{name: "custom error implementing NotFound() matches via interface arm", err: customNotFoundErr{msg: "image pulse:test"}, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsNotFound(tc.err); got != tc.want {
				t.Fatalf("IsNotFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestBranchcov0723AmParseTraefikHostRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rule string
		want string
	}{
		{name: "empty rule yields empty host", rule: "", want: ""},
		{name: "whitespace-only rule yields empty host", rule: "   \t\n  ", want: ""},
		{name: "rule without Host clause yields empty host", rule: "PathPrefix(`/api`)", want: ""},
		{name: "well-formed backtick Host returns exact hostname", rule: "Host(`example.com`)", want: "example.com"},
		{name: "surrounding whitespace is trimmed before matching", rule: "   Host(`example.com`)   ", want: "example.com"},
		{name: "single-quote variant is rejected (source matches backticks only)", rule: "Host('example.com')", want: ""},
		{name: "double-quote variant is rejected (source matches backticks only)", rule: "Host(\"example.com\")", want: ""},
		// NOTE: the source checks prefix "Host(`" and suffix "`)" independently
		// rather than verifying the rule is a single Host clause. Combined
		// rules that happen to start with Host(` and end with `) therefore
		// pass both checks and return the malformed text in between. The
		// following two cases assert that real (buggy) behaviour rather than
		// the intuitive empty string; see REPORT for the suspected source bug.
		{name: "Host combined with PathPrefix via && is NOT rejected (lenient parser)", rule: "Host(`example.com`) && PathPrefix(`/api`)", want: "example.com`) && PathPrefix(`/api"},
		{name: "Host combined with another Host via || is NOT rejected (lenient parser)", rule: "Host(`a.com`) || Host(`b.com`)", want: "a.com`) || Host(`b.com"},
		{name: "PathPrefix first then Host is rejected (prefix mismatch)", rule: "PathPrefix(`/api`) && Host(`example.com`)", want: ""},
		{name: "trailing content after closing backtick-paren is rejected", rule: "Host(`example.com`)foo", want: ""},
		{name: "unterminated Host clause yields empty host", rule: "Host(`example.com", want: ""},
		{name: "missing opening backtick yields empty host", rule: "Host(example.com`)", want: ""},
		{name: "Host with empty hostname between backticks yields empty host", rule: "Host(``)", want: ""},
		{name: "host containing a dot inside backticks is preserved verbatim", rule: "Host(`sub.example.com`)", want: "sub.example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := parseTraefikHostRule(tc.rule); got != tc.want {
				t.Fatalf("parseTraefikHostRule(%q) = %q, want %q", tc.rule, got, tc.want)
			}
		})
	}
}

func TestBranchcov0723AmRouteHostFromLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// wantSet, when non-nil, is used when the source's behaviour is
		// intentionally non-deterministic (Go map iteration order is random).
		// The subtest passes if the result is any one of wantSet's entries.
		wantSet map[string]bool
		labels  map[string]string
		want    string
	}{
		{name: "nil map returns empty host", labels: nil, want: ""},
		{name: "empty map returns empty host", labels: map[string]string{}, want: ""},
		{
			name:   "matching key with well-formed Host rule returns the host",
			labels: map[string]string{"traefik.http.routers.pulse-t-acme.rule": "Host(`t-acme.cloud.example.com`)"},
			want:   "t-acme.cloud.example.com",
		},
		{name: "matching key with empty rule returns empty host", labels: map[string]string{"traefik.http.routers.pulse.rule": ""}, want: ""},
		{name: "matching key with non-Host rule returns empty host", labels: map[string]string{"traefik.http.routers.pulse.rule": "PathPrefix(`/api`)"}, want: ""},
		{name: "matching key with unterminated Host returns empty host", labels: map[string]string{"traefik.http.routers.pulse.rule": "Host(`example.com"}, want: ""},
		{
			// Inherits parseTraefikHostRule's lenient behaviour: a combined
			// matcher rule starting with Host(` and ending with `) parses to a
			// non-empty (but malformed) value, so routeHostFromLabels returns
			// that value rather than the empty string. See REPORT for the
			// suspected source bug in parseTraefikHostRule.
			name: "router-rule key with combined matcher rule returns the parser's malformed output",
			labels: map[string]string{
				"traefik.http.routers.pulse.rule": "Host(`example.com`) && PathPrefix(`/api`)",
			},
			want: "example.com`) && PathPrefix(`/api",
		},
		{
			name: "non-router prefix label is filtered out even when its value would parse",
			labels: map[string]string{
				"traefik.http.services.pulse.rule": "Host(`example.com`)",
			},
			want: "",
		},
		{
			name: "non-rule suffix label is filtered out even when its value would parse",
			labels: map[string]string{
				"traefik.http.routers.pulse.middlewares": "Host(`example.com`)",
			},
			want: "",
		},
		{
			name: "unrelated label is ignored and returns empty host",
			labels: map[string]string{
				"pulse.tenant.id": "t-acme",
				"com.example.foo": "bar",
			},
			want: "",
		},
		{
			name: "single valid Host rule among several malformed matching labels is returned",
			labels: map[string]string{
				"traefik.http.routers.alpha.rule": "",
				"traefik.http.routers.beta.rule":  "PathPrefix(`/api`)",
				"traefik.http.routers.gamma.rule": "Host(`gamma.example.com`)",
				"traefik.http.routers.delta.rule": "Host(`delta.example`)foo",
			},
			want: "gamma.example.com",
		},
		{
			name: "multiple matching labels that all yield empty hosts return empty",
			labels: map[string]string{
				"traefik.http.routers.alpha.rule": "",
				"traefik.http.routers.beta.rule":  "PathPrefix(`/api`)",
			},
			want: "",
		},
		{
			// The source iterates a Go map (random order) and returns the first
			// label that parses to a non-empty host; with two equally-valid
			// Host rules the result is non-deterministic. Assert the result is
			// one of the valid candidates rather than a specific winner.
			name: "two equally-valid matching labels: result is one of the valid hosts",
			labels: map[string]string{
				"traefik.http.routers.alpha.rule": "Host(`alpha.example.com`)",
				"traefik.http.routers.beta.rule":  "Host(`beta.example.com`)",
			},
			wantSet: map[string]bool{"alpha.example.com": true, "beta.example.com": true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := routeHostFromLabels(tc.labels)
			if tc.wantSet != nil {
				if !tc.wantSet[got] {
					t.Fatalf("routeHostFromLabels(%v) = %q, want one of %v", tc.labels, got, tc.wantSet)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("routeHostFromLabels(%v) = %q, want %q", tc.labels, got, tc.want)
			}
		})
	}
}
