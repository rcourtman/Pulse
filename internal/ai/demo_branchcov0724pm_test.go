package ai

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/mockruntime"
)

// This file raises branch coverage on ai.IsDemoRuntimeIntended (demo.go:32):
//
//	func IsDemoRuntimeIntended() bool {
//	    return IsDemoMode() || mockmode.IsRequestedFromEnv()
//	}
//
// It reports operator *intent* to serve demo fixtures, not the live enablement
// gate, so boot-time lifecycle (whether to start the patrol loop) can survive the
// release demo ordering where fixtures enable only after the license sync runs.
//
// IsDemoMode() reads the in-process mockruntime toggle; IsRequestedFromEnv()
// reads PULSE_MOCK_MODE. The table below covers the || short-circuit's three
// meaningful outcomes: both false, only the env arm true, and the demo-mode arm
// true (which short-circuits before the env is consulted).

func TestBranchcov0724pmIsDemoRuntimeIntended(t *testing.T) {
	// IsDemoMode() ultimately reads a package-level mockruntime flag. Save and
	// restore it so this test cannot perturb the ai package's serial test run.
	originalMock := mockruntime.IsEnabled()
	t.Cleanup(func() { mockruntime.SetEnabled(originalMock) })

	cases := []struct {
		name     string
		mockOn   bool
		envValue string // value applied to PULSE_MOCK_MODE ("" models an absent var)
		want     bool
	}{
		{
			name:     "false when mock disabled and env does not request",
			mockOn:   false,
			envValue: "false",
			want:     false,
		},
		{
			name:     "true via env request when mock disabled",
			mockOn:   false,
			envValue: "true",
			want:     true,
		},
		{
			name:     "env request is case-insensitive",
			mockOn:   false,
			envValue: "TRUE",
			want:     true,
		},
		{
			name:     "env request tolerates surrounding whitespace",
			mockOn:   false,
			envValue: " true ",
			want:     true,
		},
		{
			name:     "env value other than true does not request",
			mockOn:   false,
			envValue: "yes",
			want:     false,
		},
		{
			name:     "true via demo mode when env false (short-circuits env arm)",
			mockOn:   true,
			envValue: "false",
			want:     true,
		},
		{
			name:     "true when both demo mode and env request",
			mockOn:   true,
			envValue: "true",
			want:     true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// t.Setenv restores the previous value at the end of the subtest.
			t.Setenv("PULSE_MOCK_MODE", tc.envValue)
			mockruntime.SetEnabled(tc.mockOn)

			got := IsDemoRuntimeIntended()
			if got != tc.want {
				t.Errorf("IsDemoRuntimeIntended() = %v, want %v (mockOn=%v PULSE_MOCK_MODE=%q)",
					got, tc.want, tc.mockOn, tc.envValue)
			}
		})
	}
}
