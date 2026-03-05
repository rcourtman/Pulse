package ai

import "testing"

func TestIsUnsupportedLegacyAIResourceTypeToken(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "empty allowed", in: "", want: false},
		{name: "whitespace allowed", in: "   ", want: false},
		{name: "legacy host rejected", in: "host", want: true},
		{name: "mixed case host rejected", in: " HoSt ", want: true},
		{name: "legacy guest rejected", in: "guest", want: true},
		{name: "legacy container rejected", in: "container", want: true},
		{name: "legacy docker rejected", in: "docker", want: true},
		{name: "legacy k8s rejected", in: "k8s", want: true},
		{name: "agent allowed", in: "agent", want: false},
		{name: "system container allowed", in: "system-container", want: false},
		{name: "app container allowed", in: "app-container", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUnsupportedLegacyAIResourceTypeToken(tt.in); got != tt.want {
				t.Fatalf("isUnsupportedLegacyAIResourceTypeToken(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
