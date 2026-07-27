package monitoring

import (
	"testing"
)

// This file is a purpose-built branch-coverage test set (selected via
// `-run "^TestBranchcov0722"`) for two pure helpers in
// availability_poller.go that previously had 0.0% coverage:
//
//   - availabilityConnectionKey(targetID string) string
//   - (availabilityPollProvider).ConnectionHealthKey(_ *Monitor, instanceName string) string
//
// The third helper originally covered here, pingArgs(host string, timeoutMillis
// int) []string, moved with the probe execution core to
// internal/availabilityprobe; its cases live in
// internal/availabilityprobe/ping_args_branchcov0722am_test.go.
//
// Every arm of each function is exercised directly here:
//
//   - availabilityConnectionKey: empty + whitespace-only -> "", plain id and
//     whitespace-surrounded id -> "availability-"+trimmed id, and the
//     already-prefixed input which (because there is no idempotency guard)
//     gets the prefix appended a second time.
//   - ConnectionHealthKey: pure delegation to availabilityConnectionKey with a
//     nil *Monitor (the receiver ignores the monitor entirely).
//
// Conventions match sibling in-package tests in this directory (see
// monitoring_infra_keys_branchcov0716_test.go and the connected-infrastructure
// branch-cov set): stdlib `testing` only, table-driven subtests, t.Fatalf
// assertions, no testify. Each case is value-in -> value-out.

func TestBranchcov0722AvailabilityConnectionKey(t *testing.T) {
	cases := []struct {
		name     string
		targetID string
		want     string
	}{
		// Arm: trimmed value is empty -> "".
		{"empty input returns empty", "", ""},
		{"whitespace-only input returns empty", "   \t\n", ""},

		// Arm: non-empty trimmed value -> "availability-" + trimmed id.
		{"plain id is prefixed", "router-1", "availability-router-1"},
		{"leading and trailing whitespace trimmed before prefixing", "  router-1  ", "availability-router-1"},
		{"internal whitespace preserved", "zone a / rack 2", "availability-zone a / rack 2"},
		{"unicode id preserved", "café-设备", "availability-café-设备"},

		// Behavioural note (NOT a bug fix): the function performs no
		// idempotency check, so an id that already carries the prefix gets it
		// appended again. This asserts the real, observed behaviour.
		{"already-prefixed id is prefixed again", "availability-foo", "availability-availability-foo"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := availabilityConnectionKey(tc.targetID); got != tc.want {
				t.Fatalf("availabilityConnectionKey(%q) = %q, want %q", tc.targetID, got, tc.want)
			}
		})
	}
}

func TestBranchcov0722ConnectionHealthKey(t *testing.T) {
	provider := availabilityPollProvider{}

	cases := []struct {
		name         string
		instanceName string
		want         string
	}{
		// Arm: empty / whitespace-only instance name -> "" (via the underlying
		// availabilityConnectionKey trim+empty check). The *Monitor argument is
		// ignored, so a nil monitor exercises the function with no receiver
		// state dependency.
		{"empty instance name returns empty with nil monitor", "", ""},
		{"whitespace-only instance name returns empty with nil monitor", "   ", ""},

		// Arm: non-empty name -> "availability-" + trimmed name.
		{"plain name delegated and prefixed", "sensor-7", "availability-sensor-7"},
		{"whitespace trimmed before delegation", "  sensor-7  ", "availability-sensor-7"},
		{"already-prefixed name doubles through delegation", "availability-x", "availability-availability-x"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The receiver is a value type and the *Monitor parameter is
			// discarded, so passing nil is both safe and the cleanest way to
			// prove the method never touches monitor state.
			if got := provider.ConnectionHealthKey(nil, tc.instanceName); got != tc.want {
				t.Fatalf("ConnectionHealthKey(nil, %q) = %q, want %q", tc.instanceName, got, tc.want)
			}
		})
	}
}
