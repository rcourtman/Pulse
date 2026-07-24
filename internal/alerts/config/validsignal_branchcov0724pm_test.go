package config

import "testing"

// This file is a purpose-built branch-coverage test set (selected via
// `-run "^TestBranchcov0724pm"`) for one exported pure helper in package
// config that previously had 0.0% coverage:
//
//   - ValidAlertIntentSignal(signal string) bool — intent.go:225
//
// ValidAlertIntentSignal normalises its input with strings.ToLower +
// strings.TrimSpace before delegating to the unexported validAlertIntentSignal.
// Every arm of the delegated function is exercised through the exported
// boundary: the three constant accepted signals (*, state.offline,
// incident.availability), the metric.<name> prefix path (valid and with empty
// suffix), the unsupported default arm, and the normalisation layer itself
// (whitespace trimming + case folding that makes uppercase / padded input
// resolve identically to its canonical lowercase form).
//
// Conventions match sibling in-package tests in this directory (see
// intent_branchcov0720pm_test.go): stdlib `testing` only, table-driven
// subtests, t.Fatalf assertions, no testify.

func TestBranchcov0724pmValidAlertIntentSignal(t *testing.T) {
	tests := []struct {
		name   string
		signal string
		want   bool
	}{
		// Arm: the three constant accepted signals.
		{"default star is valid", string(AlertIntentSignalDefault), true},
		{"offline is valid", string(AlertIntentSignalOffline), true},
		{"availability is valid", string(AlertIntentSignalAvailability), true},

		// Arm: metric.<name> prefix path with a non-empty suffix.
		{"metric with name is valid", "metric.cpu", true},
		{"metric with dotted name is valid", "metric.cpu.load", true},

		// Arm: metric. prefix with empty suffix is rejected.
		{"metric with empty suffix is invalid", "metric.", false},
		// Arm: bare "metric" without a dot does not match the prefix.
		{"metric without dot is invalid", "metric", false},

		// Arm: empty and unknown signals are rejected.
		{"empty string is invalid", "", false},
		{"unknown signal is invalid", "state.online", false},
		{"arbitrary string is invalid", "foo", false},

		// Normalisation layer: ValidAlertIntentSignal applies ToLower +
		// TrimSpace before delegating, so uppercase / padded input resolves
		// the same as its canonical lowercase form.
		{"uppercase offline normalised to valid", "STATE.OFFLINE", true},
		{"padded availability normalised to valid", "  Incident.Availability  ", true},
		{"padded star normalised to valid", "  *  ", true},
		{"uppercase metric name normalised to valid", "METRIC.CPU", true},
		// After normalisation, "Metric." becomes "metric." → empty suffix.
		{"uppercase metric empty suffix still invalid", "Metric.", false},
		// Whitespace-only collapses to "" which is invalid.
		{"whitespace-only collapses to empty and is invalid", "   ", false},
		// Case-folded unknown signal is still unknown.
		{"case-differently state online still invalid", "State.Online", false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidAlertIntentSignal(tc.signal); got != tc.want {
				t.Fatalf("ValidAlertIntentSignal(%q) = %v, want %v", tc.signal, got, tc.want)
			}
		})
	}
}
