package qualification

import (
	"errors"
	"testing"
)

// TestBranchCovNormalizeJSONText drives every behavioral edge of the
// normalizeJSONText helper. The helper is
// strings.Join(strings.Fields(strings.TrimSpace(value)), " "), so genuine
// branch coverage here means exercising empty input, whitespace-only input
// (covering each Unicode whitespace class that strings.Fields recognises:
// ASCII space/tab/newline/CR/VT/FF as well as non-ASCII space U+00A0),
// single-token input that is already normal, and inputs whose internal
// whitespace runs mix several separators that must all collapse to a single
// ASCII space.
func TestBranchCovNormalizeJSONText(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty string", input: "", want: ""},
		{name: "single ascii space", input: " ", want: ""},
		{name: "ascii whitespace only collapses to empty", input: " \t\n\r\v\f", want: ""},
		{name: "leading and trailing whitespace trimmed", input: "  hello  ", want: "hello"},
		{name: "already normal single token", input: "hello", want: "hello"},
		{name: "already normal two tokens", input: "hello world", want: "hello world"},
		{name: "multiple spaces collapsed", input: "hello   world", want: "hello world"},
		{name: "mixed tab newline cr become single spaces", input: "a\tb\n c\r d", want: "a b c d"},
		{name: "newline separated lines flattened", input: "line1\nline2", want: "line1 line2"},
		{name: "non-ascii non-whitespace content preserved", input: "emoji 😀 text", want: "emoji 😀 text"},
		{name: "nbsp treated as field separator", input: "a\xc2\xa0b", want: "a b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeJSONText(tc.input); got != tc.want {
				t.Errorf("normalizeJSONText(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestBranchCovPatrolAutonomyEffective covers both arms of the conditional in
// PatrolAutonomy.Effective: the EffectiveAutonomyLevel branch when that field
// has non-empty content after trimming (including that the result is itself
// trimmed), and the fallback arm when EffectiveAutonomyLevel is empty or
// whitespace-only (covering the AutonomyLevel trim on that path), plus the
// degenerate case where both fields are empty/whitespace.
func TestBranchCovPatrolAutonomyEffective(t *testing.T) {
	cases := []struct {
		name string
		a    PatrolAutonomy
		want string
	}{
		{
			name: "effective level wins over autonomy level",
			a:    PatrolAutonomy{EffectiveAutonomyLevel: "auto", AutonomyLevel: "semi"},
			want: "auto",
		},
		{
			name: "effective level is trimmed before return",
			a:    PatrolAutonomy{EffectiveAutonomyLevel: "  auto  ", AutonomyLevel: "semi"},
			want: "auto",
		},
		{
			name: "empty effective falls back to autonomy level",
			a:    PatrolAutonomy{EffectiveAutonomyLevel: "", AutonomyLevel: "semi"},
			want: "semi",
		},
		{
			name: "whitespace-only effective falls back to autonomy level",
			a:    PatrolAutonomy{EffectiveAutonomyLevel: "   ", AutonomyLevel: "semi"},
			want: "semi",
		},
		{
			name: "fallback autonomy level is trimmed",
			a:    PatrolAutonomy{EffectiveAutonomyLevel: "", AutonomyLevel: "  semi  "},
			want: "semi",
		},
		{
			name: "both fields whitespace only yields empty",
			a:    PatrolAutonomy{EffectiveAutonomyLevel: "  ", AutonomyLevel: "\t"},
			want: "",
		},
		{name: "zero value yields empty", a: PatrolAutonomy{}, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Effective(); got != tc.want {
				t.Errorf("Effective() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBranchCovHTTPError asserts the real formatting behaviour of
// (*HTTPError).Error() across the meaningful input space: the zero value, a
// typical populated error, an empty body, a negative status code, and —
// crucially — the case where Body itself contains fmt verbs. Because Body is
// passed as an argument to fmt.Sprintf (not interpolated into the format
// string), such verbs must appear verbatim and never be re-expanded; this
// guards against a future refactor that turns the format into user-controlled
// text.
//
// Each case also exercises *HTTPError through the `error` interface, locking
// in that the method set still satisfies `error` (callers in client.go return
// &HTTPError{} as `error`, so a receiver change would silently break them).
func TestBranchCovHTTPError(t *testing.T) {
	cases := []struct {
		name string
		err  *HTTPError
		want string
	}{
		{name: "zero value", err: &HTTPError{}, want: "Pulse API  returned 0: "},
		{
			name: "typical 404",
			err:  &HTTPError{StatusCode: 404, Path: "/api/v1/foo", Body: "not found"},
			want: "Pulse API /api/v1/foo returned 404: not found",
		},
		{
			name: "empty body rendered with trailing colon space",
			err:  &HTTPError{StatusCode: 500, Path: "/api/v1/bar", Body: ""},
			want: "Pulse API /api/v1/bar returned 500: ",
		},
		{
			name: "negative status code rendered verbatim",
			err:  &HTTPError{StatusCode: -1, Path: "/x", Body: "weird"},
			want: "Pulse API /x returned -1: weird",
		},
		{
			name: "body containing fmt verbs is not re-expanded",
			err:  &HTTPError{StatusCode: 502, Path: "/api/v1/baz", Body: "boom %s %d %%"},
			want: "Pulse API /api/v1/baz returned 502: boom %s %d %%",
		},
		{
			name: "percent literal in path preserved",
			err:  &HTTPError{StatusCode: 400, Path: "/api/%2Fenc", Body: "bad"},
			want: "Pulse API /api/%2Fenc returned 400: bad",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var asErr error = tc.err
			if got := asErr.Error(); got != tc.want {
				t.Errorf("(*HTTPError).Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBranchCovHTTPErrorErrorsAsRoundTrip verifies the concrete type survives a
// round trip through the `error` interface via errors.As, mirroring how
// callers of client.go surface HTTP failures (and how they recover them with
// errors.As/Is). This locks in that *HTTPError — not HTTPError — is the
// addressable error type, so the errors.As target *HTTPError matches.
func TestBranchCovHTTPErrorErrorsAsRoundTrip(t *testing.T) {
	original := &HTTPError{StatusCode: 418, Path: "/teapot", Body: "i am a teapot"}
	var asErr error = original

	var target *HTTPError
	if !errors.As(asErr, &target) {
		t.Fatalf("errors.As((*HTTPError)(nil) target) = false, want true")
	}
	if target == nil {
		t.Fatal("errors.As assigned nil target")
	}
	if target.StatusCode != original.StatusCode ||
		target.Path != original.Path ||
		target.Body != original.Body {
		t.Errorf("errors.As round trip lost data: got %+v, want %+v", target, original)
	}

	// An unrelated error type must NOT be misidentified as *HTTPError.
	unrelated := errors.New("plain sentinel")
	var badTarget *HTTPError
	if errors.As(unrelated, &badTarget) {
		t.Errorf("errors.As matched *HTTPError on unrelated error %v", unrelated)
	}
}
