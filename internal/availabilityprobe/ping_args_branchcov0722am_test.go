package availabilityprobe

import (
	"reflect"
	"runtime"
	"strconv"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Branch-coverage test set for pingArgs, moved verbatim (aside from the
// package clause) from
// internal/monitoring/availability_poller_purehelpers_branchcov0722am_test.go
// when the probe execution core was extracted into this package. It is still
// selected via `-run "^TestBranchcov0722"` and covers:
//
//   - pingArgs: the timeoutMillis <= 0 fallback to the config default, the
//     runtime.GOOS switch (windows / darwin+BSD / default), and the default
//     arm's ceiling arithmetic ((ms+999)/1000) plus the clamp-to-1 guard.
//
// Conventions match the sibling monitoring tests: stdlib `testing` only,
// table-driven subtests, t.Fatalf assertions, no testify.

func TestBranchcov0722PingArgs(t *testing.T) {
	const host = "probe.example.test"

	t.Run("positive timeout reaches the platform arm verbatim", func(t *testing.T) {
		got := pingArgs(host, 5000)
		switch runtime.GOOS {
		case "windows":
			want := []string{"-n", "1", "-w", "5000", host}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("pingArgs(%q, 5000) on windows = %v, want %v", host, got, want)
			}
		case "darwin", "freebsd", "openbsd", "netbsd":
			want := []string{"-n", "-c", "1", "-W", "5000", host}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("pingArgs(%q, 5000) on %s = %v, want %v", host, runtime.GOOS, got, want)
			}
		default:
			// Default arm: (5000 + 999) / 1000 = 5 whole seconds.
			want := []string{"-n", "-c", "1", "-W", "5", host}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("pingArgs(%q, 5000) on %s = %v, want %v", host, runtime.GOOS, got, want)
			}
		}
	})

	t.Run("zero or negative timeout falls back to config default", func(t *testing.T) {
		defaultMillis := config.DefaultAvailabilityTimeoutMillis
		for _, ms := range []int{0, -1, -99999} {
			got := pingArgs(host, ms)
			switch runtime.GOOS {
			case "windows":
				want := []string{"-n", "1", "-w", strconv.Itoa(defaultMillis), host}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("pingArgs(%q, %d) on windows = %v, want %v", host, ms, got, want)
				}
			case "darwin", "freebsd", "openbsd", "netbsd":
				want := []string{"-n", "-c", "1", "-W", strconv.Itoa(defaultMillis), host}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("pingArgs(%q, %d) on %s = %v, want %v", host, ms, runtime.GOOS, got, want)
				}
			default:
				secs := (defaultMillis + 999) / 1000
				want := []string{"-n", "-c", "1", "-W", strconv.Itoa(secs), host}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("pingArgs(%q, %d) on %s = %v, want %v", host, ms, runtime.GOOS, got, want)
				}
			}
		}
	})

}

// TestBranchcov0722PingArgsDefaultArmCeilingArithmetic targets the
// non-Windows / non-BSD ("default") arm of the runtime.GOOS switch, which is
// the only arm that performs the (ms+999)/1000 ceiling division and the
// clamp-to-1 guard. pingArgs reads runtime.GOOS directly, so this arm can only
// be reached on platforms such as linux; on windows/darwin/BSD the test skips
// honestly rather than asserting behaviour it cannot reach.
func TestBranchcov0722PingArgsDefaultArmCeilingArithmetic(t *testing.T) {
	switch runtime.GOOS {
	case "windows", "darwin", "freebsd", "openbsd", "netbsd":
		t.Skipf("ceiling arithmetic lives only in the default (linux-like) pingArgs arm; skipping on %s", runtime.GOOS)
	}

	const host = "probe.example.test"
	cases := []struct {
		name        string
		timeoutMs   int
		wantSeconds int
	}{
		// Ceiling division: any sub-second value rounds up to 1 second.
		{"one millisecond rounds up to one second", 1, 1},
		{"just under a second rounds up to one second", 999, 1},
		// Exact second boundary stays put.
		{"exact one thousand millis is one second", 1000, 1},
		// Fractional seconds round up to the next whole second.
		{"one thousand one millis rounds up to two seconds", 1001, 2},
		{"fifteen hundred millis rounds up to two seconds", 1500, 2},
		{"five thousand nine hundred ninety nine rounds up to six seconds", 5999, 6},
		// NOTE: the `if timeoutSeconds <= 0 { timeoutSeconds = 1 }` clamp is
		// unreachable here. The non-positive fallback guarantees timeoutMillis
		// is at least DefaultAvailabilityTimeoutMillis (2000) before this arm,
		// and any positive value yields (ms+999)/1000 >= 1. No table case can
		// exercise the clamp; this is documented in the report, not "fixed".
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pingArgs(host, tc.timeoutMs)
			// Default arm shape: ["-n", "-c", "1", "-W", <seconds>, host].
			want := []string{"-n", "-c", "1", "-W", strconv.Itoa(tc.wantSeconds), host}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("pingArgs(%q, %d) = %v, want %v", host, tc.timeoutMs, got, want)
			}
		})
	}
}

// TestBranchcov0722PingArgsCloneIndependence asserts that the returned slice
// is freshly allocated on every call: mutating one result must not corrupt a
// previously returned or subsequently returned slice, and vice versa. (pingArgs
// takes only immutable inputs — a string and an int — so the relevant
// invariant is that distinct calls do not alias backing arrays.)
func TestBranchcov0722PingArgsCloneIndependence(t *testing.T) {
	first := pingArgs("alpha", 1000)
	second := pingArgs("beta", 2000)

	snapshot := append([]string(nil), first...)
	for i := range second {
		second[i] = "MUTATED"
	}
	if !reflect.DeepEqual(first, snapshot) {
		t.Fatalf("mutating a pingArgs result corrupted a distinct result: first = %v, want %v", first, snapshot)
	}

	third := pingArgs("gamma", 3000)
	snapshotThird := append([]string(nil), third...)
	for i := range first {
		first[i] = "MUTATED"
	}
	if !reflect.DeepEqual(third, snapshotThird) {
		t.Fatalf("mutating an earlier pingArgs result corrupted a later result: third = %v, want %v", third, snapshotThird)
	}
}
