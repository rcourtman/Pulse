package monitoring

import "testing"

func TestResolveTaskWorkerCountDefaultClamp(t *testing.T) {
	t.Setenv("POLL_TASK_WORKERS", "")

	cases := []struct {
		name string
		in   int
		want int
	}{
		{name: "zero clients floors to one worker", in: 0, want: 1},
		{name: "negative floors to one worker", in: -3, want: 1},
		{name: "small estates keep their count", in: 4, want: 4},
		{name: "large estates cap at ten", in: 40, want: 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveTaskWorkerCount(tc.in); got != tc.want {
				t.Fatalf("resolveTaskWorkerCount(%d) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveTaskWorkerCountEnvOverride(t *testing.T) {
	t.Setenv("POLL_TASK_WORKERS", "24")
	if got := resolveTaskWorkerCount(3); got != 24 {
		t.Fatalf("override ignored: got %d, want 24", got)
	}
}

func TestResolveTaskWorkerCountEnvOverrideCeiling(t *testing.T) {
	t.Setenv("POLL_TASK_WORKERS", "5000")
	if got := resolveTaskWorkerCount(3); got != 128 {
		t.Fatalf("override ceiling not applied: got %d, want 128", got)
	}
}

func TestResolveTaskWorkerCountInvalidOverrideFallsBack(t *testing.T) {
	t.Setenv("POLL_TASK_WORKERS", "not-a-number")
	if got := resolveTaskWorkerCount(40); got != 10 {
		t.Fatalf("invalid override should fall back to default clamp: got %d, want 10", got)
	}
}
