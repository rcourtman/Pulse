package unifiedresources

import (
	"strings"
	"testing"
)

// TestBranchcov0720am_TopLevelSystemIdentityMatchBasis exercises every
// conditional arm of topLevelSystemIdentityMatchBasis: the two recognized
// identity-match signals ("dmi_uuid", "hostname+mac") and the fallback for
// any other (including empty) signal.
//
// The function is a pure signal classifier that returns a human-readable
// basis string. Rather than pinning exact literals (which would be a brittle
// change-detector), these subtests assert the *classifier* behavior:
//
//   - every recognized signal produces a distinct, non-empty classification
//     (i.e. the function discriminates between the known signals);
//   - any unrecognized signal collapses to the single shared fallback;
//   - the fallback is distinct from every recognized-signal output (so the
//     default arm is genuinely a third branch, not a silent alias);
//   - the function is deterministic for identical input.
func TestBranchcov0720am_TopLevelSystemIdentityMatchBasis(t *testing.T) {
	recognized := []string{"dmi_uuid", "hostname+mac"}
	unknown := []string{
		"",
		"   ",
		"machine-id",
		"agent-id",
		"canonical-primary-id",
		"resource-id",
		"ip",
		"something-the-classifier-does-not-know",
	}

	t.Run("recognized signals each yield a distinct non-empty classification", func(t *testing.T) {
		seen := make(map[string]string, len(recognized))
		for _, signal := range recognized {
			got := topLevelSystemIdentityMatchBasis(signal)
			if strings.TrimSpace(got) == "" {
				t.Fatalf("signal %q returned empty basis", signal)
			}
			if existing, ok := seen[got]; ok {
				t.Fatalf(
					"signals %q and %q collapsed to the same basis %q; "+
						"recognized signals must remain distinguishable",
					existing, signal, got,
				)
			}
			seen[got] = signal
		}
		if len(seen) != len(recognized) {
			t.Fatalf("expected %d distinct classifications, got %d", len(recognized), len(seen))
		}
	})

	t.Run("unknown signals all collapse to the single fallback classification", func(t *testing.T) {
		fallback := topLevelSystemIdentityMatchBasis("__definitely-not-a-known-signal__")
		if strings.TrimSpace(fallback) == "" {
			t.Fatalf("fallback basis is empty; classifier must always classify")
		}
		for _, signal := range unknown {
			got := topLevelSystemIdentityMatchBasis(signal)
			if got != fallback {
				t.Fatalf(
					"unknown signal %q returned %q, expected the shared fallback %q",
					signal, got, fallback,
				)
			}
		}
	})

	t.Run("fallback arm is a real third branch distinct from recognized arms", func(t *testing.T) {
		fallback := topLevelSystemIdentityMatchBasis("nope")
		for _, signal := range recognized {
			if got := topLevelSystemIdentityMatchBasis(signal); got == fallback {
				t.Fatalf(
					"recognized signal %q produced the same basis as the fallback %q; "+
						"default arm is not exercising a distinct branch",
					signal, fallback,
				)
			}
		}
	})

	t.Run("classification is deterministic for identical input", func(t *testing.T) {
		for _, signal := range append(append([]string{}, recognized...), unknown...) {
			first := topLevelSystemIdentityMatchBasis(signal)
			second := topLevelSystemIdentityMatchBasis(signal)
			if first != second {
				t.Fatalf(
					"signal %q classified non-deterministically: %q then %q",
					signal, first, second,
				)
			}
		}
	})

	t.Run("result never carries leading/trailing whitespace", func(t *testing.T) {
		for _, signal := range append(append([]string{}, recognized...), "__unknown__") {
			if got := topLevelSystemIdentityMatchBasis(signal); got != strings.TrimSpace(got) {
				t.Fatalf("signal %q returned padded basis %q", signal, got)
			}
		}
	})
}
