package monitoring

import "testing"

// benchLayoutProbe exists only to move the functions linked after it by one
// 32-byte alignment slot. It tests whether BenchmarkHistoryReadFilter is
// sensitive to code placement on the CI runner. Not for merge.
//
//go:noinline
func benchLayoutProbe() int { return 1 }

func TestBenchLayoutProbe(t *testing.T) {
	if got := benchLayoutProbe(); got != 1 {
		t.Fatalf("probe = %d", got)
	}
}
