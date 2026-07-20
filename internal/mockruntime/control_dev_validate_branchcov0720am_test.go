//go:build !release

package mockruntime

import (
	"os"
	"testing"
)

// unsetEnvVarForTest removes the named env var for the duration of t and
// restores whatever value (if any) was present beforehand. Used for the
// "env unset" arm of startupEnabledFromEnv: t.Setenv only supports assigning a
// value, so we drop down to os.Unsetenv with an explicit cleanup. The helper
// deliberately does not call t.Parallel — env mutation is inherently serial.
func unsetEnvVarForTest(t *testing.T, name string) {
	t.Helper()
	prevValue, hadPrev := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("failed to unsetenv %q before test: %v", name, err)
	}
	t.Cleanup(func() {
		if hadPrev {
			if err := os.Setenv(name, prevValue); err != nil {
				t.Errorf("failed to restore %q=%q after test: %v", name, prevValue, err)
			}
		} else if err := os.Unsetenv(name); err != nil {
			t.Errorf("failed to clear %q after test: %v", name, err)
		}
	})
}

// TestBranchcov0720am_StartupEnabledFromEnv_Branches drives every distinct
// classification arm of startupEnabledFromEnv by varying PULSE_MOCK_MODE:
//
//   - the env-unset arm (no var present at all),
//   - the empty / whitespace-only arms (TrimSpace yields ""),
//   - the matching arm — canonical "true" plus the case-insensitive and
//     whitespace-padded variants that EqualFold + TrimSpace must accept,
//   - the non-matching arm — plausible-but-wrong values ("false", "1", "yes",
//     "on", "enabled", "truely") that must classify as disabled.
//
// Each assertion is behavioural: it pins the classification outcome for a
// representative input, not a literal internal string. startupEnabledFromEnv
// reads os.Getenv fresh on every call (it is not cached — see
// TestBranchcov0720am_StartupEnabledFromEnv_ReadsEnvFreshOnEachCall), so
// t.Setenv is sufficient to steer each branch without restarting the process.
func TestBranchcov0720am_StartupEnabledFromEnv_Branches(t *testing.T) {
	cases := []struct {
		name   string
		envVal string // assigned to PULSE_MOCK_MODE when unset is false
		unset  bool   // when true, PULSE_MOCK_MODE is removed entirely
		want   bool
	}{
		{name: "env unset returns false", unset: true, want: false},
		{name: "empty string returns false", envVal: "", want: false},
		{name: "whitespace-only returns false", envVal: "   ", want: false},
		{name: "literal true returns true", envVal: "true", want: true},
		{name: "uppercase TRUE returns true via EqualFold", envVal: "TRUE", want: true},
		{name: "mixed-case True returns true via EqualFold", envVal: "True", want: true},
		{name: "odd-case tRuE returns true via EqualFold", envVal: "tRuE", want: true},
		{name: "leading and trailing spaces trim to true", envVal: "   true   ", want: true},
		{name: "embedded tab and newline trim to true", envVal: "\ttrue\n", want: true},
		{name: "literal false returns false", envVal: "false", want: false},
		{name: "uppercase FALSE returns false", envVal: "FALSE", want: false},
		{name: "numeric one is not recognized (only literal true matches)", envVal: "1", want: false},
		{name: "yes is not recognized", envVal: "yes", want: false},
		{name: "on is not recognized", envVal: "on", want: false},
		{name: "enabled is not recognized", envVal: "enabled", want: false},
		{name: "true-ish prefix is not a substring match", envVal: "truely", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.unset {
				unsetEnvVarForTest(t, "PULSE_MOCK_MODE")
			} else {
				t.Setenv("PULSE_MOCK_MODE", tc.envVal)
			}
			got := startupEnabledFromEnv()
			if got != tc.want {
				t.Fatalf("startupEnabledFromEnv() = %v, want %v (PULSE_MOCK_MODE=%q)",
					got, tc.want, tc.envVal)
			}
		})
	}
}

// TestBranchcov0720am_StartupEnabledFromEnv_ReadsEnvFreshOnEachCall pins the
// behavioural guarantee that startupEnabledFromEnv is a pure read of the
// current process environment rather than a once-cached init value: flipping
// PULSE_MOCK_MODE between values within a single test process must flip the
// next call's result. This is what makes the per-case env manipulation in the
// table above meaningful, and it is the only contract worth pinning beyond the
// per-input classification (a stale-cache regression would silently break the
// init() seed in runtime.go:18 without failing the table test).
func TestBranchcov0720am_StartupEnabledFromEnv_ReadsEnvFreshOnEachCall(t *testing.T) {
	t.Setenv("PULSE_MOCK_MODE", "true")
	if got := startupEnabledFromEnv(); !got {
		t.Fatalf("first call with PULSE_MOCK_MODE=true: got false, want true")
	}
	t.Setenv("PULSE_MOCK_MODE", "false")
	if got := startupEnabledFromEnv(); got {
		t.Fatalf("second call with PULSE_MOCK_MODE=false: got true, want false (env not read fresh)")
	}
	t.Setenv("PULSE_MOCK_MODE", "true")
	if got := startupEnabledFromEnv(); !got {
		t.Fatalf("third call with PULSE_MOCK_MODE=true again: got false, want true (env not read fresh)")
	}
}
