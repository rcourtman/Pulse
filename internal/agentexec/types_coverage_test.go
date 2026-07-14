package agentexec

import "testing"

func TestCoverageNormalizeDeployMaxParallel(t *testing.T) {
	for _, tc := range []struct {
		name         string
		value        int
		defaultValue int
		want         int
	}{
		{"zero value uses default", 0, 5, 5},
		{"negative value uses default", -3, 4, 4},
		{"normal passthrough below cap", 3, 5, 3},
		{"one passthrough", 1, 5, 1},
		{"at cap passthrough", MaxDeployParallel, 5, MaxDeployParallel},
		{"over cap clamped to max", 15, 5, MaxDeployParallel},
		{"far over cap clamped to max", 1000, 1, MaxDeployParallel},
		{"default over cap clamped to max", 0, 25, MaxDeployParallel},
		{"default zero stays zero", 0, 0, 0},
		{"positive value ignores zero default", 7, 0, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeDeployMaxParallel(tc.value, tc.defaultValue)
			if got != tc.want {
				t.Fatalf("NormalizeDeployMaxParallel(%d, %d) = %d, want %d",
					tc.value, tc.defaultValue, got, tc.want)
			}
		})
	}
}
