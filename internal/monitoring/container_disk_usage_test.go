package monitoring

import (
	"math"
	"testing"
)

func TestClampToInt64(t *testing.T) {
	tests := []struct {
		name     string
		value    uint64
		expected int64
	}{
		{
			name:     "zero",
			value:    0,
			expected: 0,
		},
		{
			name:     "small value",
			value:    100,
			expected: 100,
		},
		{
			name:     "max int64",
			value:    uint64(math.MaxInt64),
			expected: math.MaxInt64,
		},
		{
			name:     "one below max int64",
			value:    uint64(math.MaxInt64) - 1,
			expected: math.MaxInt64 - 1,
		},
		{
			name:     "one above max int64",
			value:    uint64(math.MaxInt64) + 1,
			expected: math.MaxInt64,
		},
		{
			name:     "max uint64",
			value:    math.MaxUint64,
			expected: math.MaxInt64,
		},
		{
			name:     "typical disk size 1TB",
			value:    1099511627776, // 1 TiB
			expected: 1099511627776,
		},
		{
			name:     "typical disk size 100TB",
			value:    109951162777600, // 100 TiB
			expected: 109951162777600,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := clampToInt64(tc.value)
			if result != tc.expected {
				t.Errorf("clampToInt64(%d) = %d, want %d", tc.value, result, tc.expected)
			}
		})
	}
}
