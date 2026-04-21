package unifiedresources

import "testing"

func TestHostnamesEquivalent(t *testing.T) {
	tests := []struct {
		name     string
		left     string
		right    string
		expected bool
	}{
		{
			name:     "exact short hostname",
			left:     "qnap",
			right:    "qnap",
			expected: true,
		},
		{
			name:     "short and fqdn match",
			left:     "qnap",
			right:    "qnap.local",
			expected: true,
		},
		{
			name:     "fqdn and short match",
			left:     "qnap.local",
			right:    "qnap",
			expected: true,
		},
		{
			name:     "exact fqdn match",
			left:     "qnap.local",
			right:    "QNAP.local.",
			expected: true,
		},
		{
			name:     "different fqdn suffixes stay distinct",
			left:     "prox97.a.local",
			right:    "prox97.b.local",
			expected: false,
		},
		{
			name:     "different hosts do not match",
			left:     "qnap",
			right:    "other",
			expected: false,
		},
		{
			name:     "ip address does not count as hostname",
			left:     "10.0.0.5",
			right:    "qnap.local",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HostnamesEquivalent(tt.left, tt.right); got != tt.expected {
				t.Fatalf("HostnamesEquivalent(%q, %q) = %v, want %v", tt.left, tt.right, got, tt.expected)
			}
		})
	}
}
