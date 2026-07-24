package storagehealth

import (
	"testing"
)

// TestBranchcov0724pmZfsScanActive covers the default return-false arm
// (topology.go:241-242) that the existing suite never reaches — all current
// indirect callers pass "SCANNING", hitting only the return-true arm.
// Also verifies case- and whitespace-insensitivity for every canonical
// active state.
func TestBranchcov0724pmZfsScanActive(t *testing.T) {
	activeCases := []struct {
		name  string
		state string
	}{
		{name: "scanning", state: "SCANNING"},
		{name: "running", state: "RUNNING"},
		{name: "in-progress", state: "IN_PROGRESS"},
		{name: "inprogress-no-underscore", state: "INPROGRESS"},
		{name: "lowercase-scanning", state: "scanning"},
		{name: "mixedcase-Running", state: "Running"},
		{name: "whitespace-padded", state: "  SCANNING  "},
	}
	for _, tc := range activeCases {
		t.Run("active-"+tc.name, func(t *testing.T) {
			if !zfsScanActive(tc.state) {
				t.Fatalf("zfsScanActive(%q) = false, want true", tc.state)
			}
		})
	}

	inactiveCases := []struct {
		name  string
		state string
	}{
		{name: "finished", state: "FINISHED"},
		{name: "done", state: "DONE"},
		{name: "empty", state: ""},
		{name: "canceled", state: "CANCELED"},
		{name: "paused", state: "PAUSED"},
		{name: "whitespace-only", state: "   "},
		{name: "lowercase-finished", state: "finished"},
	}
	for _, tc := range inactiveCases {
		t.Run("inactive-"+tc.name, func(t *testing.T) {
			if zfsScanActive(tc.state) {
				t.Fatalf("zfsScanActive(%q) = true, want false", tc.state)
			}
		})
	}
}

// TestBranchcov0724pmFirstNonEmpty covers the all-empty return arm
// (topology.go:252) that the existing suite never reaches, plus the
// trim-and-skip behaviour and the no-argument edge case.
func TestBranchcov0724pmFirstNonEmpty(t *testing.T) {
	t.Run("returns-first-non-empty", func(t *testing.T) {
		if got := firstNonEmpty("", "hello", "world"); got != "hello" {
			t.Fatalf("firstNonEmpty = %q, want %q", got, "hello")
		}
	})

	t.Run("returns-trimmed-value", func(t *testing.T) {
		if got := firstNonEmpty("  padded  "); got != "padded" {
			t.Fatalf("firstNonEmpty = %q, want %q", got, "padded")
		}
	})

	t.Run("skips-whitespace-only-values", func(t *testing.T) {
		if got := firstNonEmpty("   ", "\t", "\n", "found"); got != "found" {
			t.Fatalf("firstNonEmpty = %q, want %q", got, "found")
		}
	})

	t.Run("all-empty-returns-empty-string", func(t *testing.T) {
		if got := firstNonEmpty("", "   ", "\t"); got != "" {
			t.Fatalf("firstNonEmpty = %q, want empty string", got)
		}
	})

	t.Run("no-args-returns-empty-string", func(t *testing.T) {
		if got := firstNonEmpty(); got != "" {
			t.Fatalf("firstNonEmpty() = %q, want empty string", got)
		}
	})

	t.Run("single-empty-arg-returns-empty-string", func(t *testing.T) {
		if got := firstNonEmpty(""); got != "" {
			t.Fatalf("firstNonEmpty(\"\") = %q, want empty string", got)
		}
	})
}
