package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestNormalizeCommandStatus(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantError bool
	}{
		// Acknowledged status variants
		{name: "empty string maps to acknowledged", input: "", want: monitoring.DockerCommandStatusAcknowledged},
		{name: "ack maps to acknowledged", input: "ack", want: monitoring.DockerCommandStatusAcknowledged},
		{name: "acknowledged maps to acknowledged", input: "acknowledged", want: monitoring.DockerCommandStatusAcknowledged},
		{name: "ACK uppercase maps to acknowledged", input: "ACK", want: monitoring.DockerCommandStatusAcknowledged},
		{name: "Acknowledged mixed case maps to acknowledged", input: "Acknowledged", want: monitoring.DockerCommandStatusAcknowledged},

		// Completed status variants
		{name: "success maps to completed", input: "success", want: monitoring.DockerCommandStatusCompleted},
		{name: "completed maps to completed", input: "completed", want: monitoring.DockerCommandStatusCompleted},
		{name: "complete maps to completed", input: "complete", want: monitoring.DockerCommandStatusCompleted},
		{name: "SUCCESS uppercase maps to completed", input: "SUCCESS", want: monitoring.DockerCommandStatusCompleted},
		{name: "Completed mixed case maps to completed", input: "Completed", want: monitoring.DockerCommandStatusCompleted},

		// Failed status variants
		{name: "fail maps to failed", input: "fail", want: monitoring.DockerCommandStatusFailed},
		{name: "failed maps to failed", input: "failed", want: monitoring.DockerCommandStatusFailed},
		{name: "error maps to failed", input: "error", want: monitoring.DockerCommandStatusFailed},
		{name: "FAILED uppercase maps to failed", input: "FAILED", want: monitoring.DockerCommandStatusFailed},
		{name: "Error mixed case maps to failed", input: "Error", want: monitoring.DockerCommandStatusFailed},

		// Whitespace handling
		{name: "whitespace around ack is trimmed", input: "  ack  ", want: monitoring.DockerCommandStatusAcknowledged},
		{name: "tab and newline around success is trimmed", input: "\tsuccess\n", want: monitoring.DockerCommandStatusCompleted},
		{name: "spaces only maps to acknowledged", input: "   ", want: monitoring.DockerCommandStatusAcknowledged},

		// Invalid inputs
		{name: "unknown status returns error", input: "unknown", wantError: true},
		{name: "pending returns error", input: "pending", wantError: true},
		{name: "queued returns error", input: "queued", wantError: true},
		{name: "random string returns error", input: "foobar", wantError: true},
		{name: "partial match returns error", input: "acked", wantError: true},
		{name: "partial match fail returns error", input: "failure", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeCommandStatus(tt.input)
			if (err != nil) != tt.wantError {
				t.Fatalf("normalizeCommandStatus(%q) error = %v, wantError %v", tt.input, err, tt.wantError)
			}
			if !tt.wantError && got != tt.want {
				t.Fatalf("normalizeCommandStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
