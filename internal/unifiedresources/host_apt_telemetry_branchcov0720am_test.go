package unifiedresources

import (
	"strings"
	"testing"
)

// Branch-coverage tests for ValidHostAPTDigest in host_apt_telemetry.go.
//
// The validator normalizes with TrimSpace, then guards on length AND a
// "sha256:" prefix (combined in a single short-circuiting OR), then accepts
// only if the trailing 64 bytes are lowercase-hex-decodable AND the whole
// token is already lowercase. The table below drives every distinct arm:
//
//   - empty / whitespace-only input (length arm via TrimSpace normalization)
//   - leading+trailing whitespace around an otherwise valid token (TrimSpace)
//   - correct length but wrong prefix (prefix arm of the OR, length passes)
//   - correct prefix but length too short / too long (length arm of the OR)
//   - both length and prefix wrong (OR side that proves the second operand
//     can also drive the false return when the first is true)
//   - correct shape but uppercase hex letters (ToLower arm, decode succeeds)
//   - correct shape but mixed-case hex (ToLower arm still rejects)
//   - correct shape but non-hex characters in body (hex.DecodeString err arm)
//   - canonical happy path: lowercase-hex 64-char digest, valid prefix/length

// hexLower builds a lowercase hex body of the requested length using a
// deterministic repeating pattern so the test does not depend on importing
// any source-side constant.
func hexLower(n int) string {
	const src = "0123456789abcdef"
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(src[i%len(src)])
	}
	return b.String()
}

// hexUpper mirrors hexLower but uses A-F so the body decodes successfully yet
// fails the lowercase identity check.
func hexUpper(n int) string {
	const src = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(src[i%len(src)])
	}
	return b.String()
}

func TestBranchcov0720am_ValidHostAPTDigest(t *testing.T) {
	// Reference lowercase 64-char hex body.
	lower64 := hexLower(64)
	upper64 := hexUpper(64)
	// Non-hex body of correct length: 'z' is not a hex digit.
	nonHex64 := strings.Repeat("z", 64)

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		// --- TrimSpace normalization arm (line 12) ---
		{
			name:  "empty string fails length arm after trim",
			value: "",
			want:  false,
		},
		{
			name:  "whitespace-only collapses to empty and fails length arm",
			value: "   \t\n",
			want:  false,
		},
		{
			name:  "leading and trailing whitespace around valid token is trimmed and accepted",
			value: "  sha256:" + lower64 + "  \t",
			want:  true,
		},

		// --- Length arm of the OR guard (line 13, left operand) ---
		{
			name:  "correct prefix but body too short fails length arm",
			value: "sha256:" + hexLower(63),
			want:  false,
		},
		{
			name:  "correct prefix but body too long fails length arm",
			value: "sha256:" + hexLower(65),
			want:  false,
		},

		// --- Prefix arm of the OR guard (line 13, right operand) ---
		{
			name:  "correct length but wrong prefix fails prefix arm",
			value: "sha512:" + lower64,
			want:  false,
		},
		{
			name:  "correct length but missing prefix fails prefix arm",
			value: lower64,
			want:  false,
		},

		// --- Both operands of the OR true (line 13) ---
		{
			name:  "wrong prefix and wrong length fails composite guard",
			value: "md5:" + hexLower(16),
			want:  false,
		},

		// --- hex.DecodeString error arm (line 16-17) ---
		{
			name:  "correct shape but non-hex characters in body fails decode arm",
			value: "sha256:" + nonHex64,
			want:  false,
		},

		// --- ToLower identity check arm (line 17, right operand) ---
		{
			name:  "correct shape but uppercase hex body fails lowercase identity arm",
			value: "sha256:" + upper64,
			want:  false,
		},
		{
			name:  "correct shape but mixed-case hex body fails lowercase identity arm",
			value: "sha256:" + hexLower(32) + hexUpper(32),
			want:  false,
		},

		// --- Happy path (line 17 returns true) ---
		{
			name:  "canonical lowercase sha256 digest is accepted",
			value: "sha256:" + lower64,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidHostAPTDigest(tt.value)
			if got != tt.want {
				t.Fatalf("ValidHostAPTDigest(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// TestBranchcov0720am_ValidHostAPTDigest_Pure asserts behavioral invariants
// of the validator that hold regardless of the exact internal expression:
// the result must be deterministic for repeated calls, and TrimSpace
// equivalence must hold for any token the caller passes.
func TestBranchcov0720am_ValidHostAPTDigest_Invariants(t *testing.T) {
	t.Run("deterministic across repeated calls for valid input", func(t *testing.T) {
		v := "sha256:" + hexLower(64)
		first := ValidHostAPTDigest(v)
		for i := 0; i < 5; i++ {
			if got := ValidHostAPTDigest(v); got != first {
				t.Fatalf("non-deterministic result on call %d: got %v, want %v", i, got, first)
			}
		}
		if !first {
			t.Fatalf("expected canonical digest to validate, got %v", first)
		}
	})

	t.Run("trimspace equivalent token yields same classification", func(t *testing.T) {
		bare := "sha256:" + hexLower(64)
		padded := "\t " + bare + " \n"
		if ValidHostAPTDigest(bare) != ValidHostAPTDigest(padded) {
			t.Fatalf("expected identical classification for bare and padded forms; bare=%v padded=%v",
				ValidHostAPTDigest(bare), ValidHostAPTDigest(padded))
		}
	})

	t.Run("uppercase variant is never accepted when lowercase is", func(t *testing.T) {
		lower := "sha256:" + hexLower(64)
		upper := "sha256:" + hexUpper(64)
		if !ValidHostAPTDigest(lower) {
			t.Fatalf("precondition: lowercase digest must validate")
		}
		if ValidHostAPTDigest(upper) {
			t.Fatalf("uppercase variant must be rejected by lowercase identity check")
		}
	})
}
