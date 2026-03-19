package utils

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{name: "just now", d: 30 * time.Second, want: "just now"},
		{name: "minutes", d: 2 * time.Minute, want: "2 minutes"},
		{name: "hours", d: 3 * time.Hour, want: "3 hours"},
		{name: "days", d: 48 * time.Hour, want: "2 days"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := FormatDuration(tc.d); got != tc.want {
				t.Fatalf("FormatDuration(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}
