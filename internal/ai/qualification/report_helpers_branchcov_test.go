package qualification

import (
	"strings"
	"testing"
)

// TestSanitizeArtifactTextRedaction exercises the secretPatterns loop in
// sanitizeArtifactText: the NumSubexp()>0 branch (bearer / api-key / password
// / secret) uses the "${1}[REDACTED]" template, and the NumSubexp()==0 else
// branch (PEM private-key blocks) uses the whole-match "[REDACTED PRIVATE KEY]"
// replacement. Non-secret text must pass through verbatim.
func TestSanitizeArtifactTextRedaction(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "bearer token redacted keeping label",
			input: "Authorization: Bearer abc123-xyz",
			want:  "Authorization: Bearer [REDACTED]",
		},
		{
			name:  "bearer header is case insensitive",
			input: "authorization: bearer SecretToken",
			want:  "authorization: bearer [REDACTED]",
		},
		{
			name:  "api_key with equals assignment",
			input: "api_key=hunter2",
			want:  "api_key=[REDACTED]",
		},
		{
			name:  "api-key with dash and colon separator",
			input: "api-key: my-token",
			want:  "api-key: [REDACTED]",
		},
		{
			name:  "apitoken variant redacted",
			input: "apitoken=abcdef",
			want:  "apitoken=[REDACTED]",
		},
		{
			name:  "password with colon space separator",
			input: "password: hunter2",
			want:  "password: [REDACTED]",
		},
		{
			name:  "secret equals assignment",
			input: "secret=s3cr3t",
			want:  "secret=[REDACTED]",
		},
		{
			name:  "plain private key block uses whole match redaction",
			input: "-----BEGIN PRIVATE KEY-----\nMIIBOAIB\n-----END PRIVATE KEY-----",
			want:  "[REDACTED PRIVATE KEY]",
		},
		{
			name:  "rsa private key block redacted via prefix class",
			input: "-----BEGIN RSA PRIVATE KEY-----\nMIIBOAIB\n-----END RSA PRIVATE KEY-----",
			want:  "[REDACTED PRIVATE KEY]",
		},
		{
			name:  "non secret text is unchanged",
			input: "the quick brown fox jumps",
			want:  "the quick brown fox jumps",
		},
		{
			name:  "multiple secret kinds redacted together in one payload",
			input: "Authorization: Bearer tok123\napi_key=keyval\npassword=pw",
			want:  "Authorization: Bearer [REDACTED]\napi_key=[REDACTED]\npassword=[REDACTED]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeArtifactText(tc.input); got != tc.want {
				t.Fatalf("sanitizeArtifactText(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestSanitizeArtifactTextTruncatesAbove512KB covers the >512KB truncation
// branch of sanitizeArtifactText. The boundary value (exactly 512*1024 bytes)
// must NOT be truncated, while anything larger is cut to the limit and gets the
// "\n[TRUNCATED]" marker appended. Truncation runs before redaction, so a
// secret that falls inside the retained prefix must still be redacted.
func TestSanitizeArtifactTextTruncatesAbove512KB(t *testing.T) {
	const maxText = 512 * 1024 // 524288, mirrors the source constant
	marker := "\n[TRUNCATED]"

	t.Run("exactly at limit is not truncated", func(t *testing.T) {
		value := strings.Repeat("a", maxText)
		got := sanitizeArtifactText(value)
		if len(got) != maxText {
			t.Fatalf("len = %d, want %d (no truncation at exact boundary)", len(got), maxText)
		}
		if strings.Contains(got, "[TRUNCATED]") {
			t.Fatal("value at exact limit must not gain the truncation marker")
		}
	})

	t.Run("over limit is cut to limit plus marker and drops the tail", func(t *testing.T) {
		// "HEAD" + maxText bytes + "TAIL" is strictly longer than maxText.
		value := "HEAD" + strings.Repeat("x", maxText) + "TAIL"
		got := sanitizeArtifactText(value)
		if wantLen := maxText + len(marker); len(got) != wantLen {
			t.Fatalf("len = %d, want %d", len(got), wantLen)
		}
		if !strings.HasPrefix(got, "HEAD") {
			t.Fatalf("retained prefix lost head bytes: %q", got[:8])
		}
		if !strings.HasSuffix(got, marker) {
			t.Fatalf("missing truncation marker; suffix = %q", got[len(got)-len(marker):])
		}
		if strings.Contains(got, "TAIL") {
			t.Fatal("bytes past the limit must be dropped, not retained")
		}
	})

	t.Run("secret inside retained prefix is redacted after truncation", func(t *testing.T) {
		head := "Authorization: Bearer leak-token\n" + strings.Repeat("z", 32)
		value := head + strings.Repeat("y", maxText) // guarantees truncation; head is retained
		got := sanitizeArtifactText(value)
		if !strings.HasSuffix(got, marker) {
			t.Fatalf("expected truncation marker; suffix = %q", got[len(got)-len(marker):])
		}
		if !strings.Contains(got, "Authorization: Bearer [REDACTED]") {
			t.Fatalf("secret within retained prefix was not redacted: %q", got[:80])
		}
		if strings.Contains(got, "leak-token") {
			t.Fatal("bearer secret value must be redacted even after truncation")
		}
	})
}

// TestAllObservationsPassedBranches covers every branch of allObservationsPassed:
// the empty/nil guard (returns false), the early return on the first failing
// observation, and the all-passed tail return. This is intentionally distinct
// from observationsPassedOrEmpty (covered elsewhere), which treats an empty
// slice as passing.
func TestAllObservationsPassedBranches(t *testing.T) {
	cases := []struct {
		name         string
		observations []PredicateObservation
		want         bool
	}{
		{"nil slice returns false", nil, false},
		{"empty slice returns false", []PredicateObservation{}, false},
		{"single passing observation returns true", []PredicateObservation{{Passed: true}}, true},
		{"single failing observation returns false", []PredicateObservation{{Passed: false}}, false},
		{"all passing returns true", []PredicateObservation{{Passed: true}, {Passed: true}, {Passed: true}}, true},
		{"first failing short circuits to false", []PredicateObservation{{Passed: false}, {Passed: true}}, false},
		{"failing in the middle returns false", []PredicateObservation{{Passed: true}, {Passed: false}, {Passed: true}}, false},
		{"last failing returns false", []PredicateObservation{{Passed: true}, {Passed: true}, {Passed: false}}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := allObservationsPassed(tc.observations); got != tc.want {
				t.Fatalf("allObservationsPassed(%d obs) = %v, want %v", len(tc.observations), got, tc.want)
			}
		})
	}
}
