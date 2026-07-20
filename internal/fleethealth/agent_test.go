package fleethealth

import (
	"testing"
	"time"
)

func TestDeriveAgentLiveness(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		lastSeen time.Time
		interval int
		want     AgentLiveness
	}{
		{name: "never reported", want: AgentLivenessPending},
		{name: "recent", lastSeen: now.Add(-30 * time.Second), want: AgentLivenessActive},
		{name: "at five minute floor", lastSeen: now.Add(-5 * time.Minute), interval: 30, want: AgentLivenessActive},
		{name: "past five minute floor", lastSeen: now.Add(-5*time.Minute - time.Second), interval: 30, want: AgentLivenessStale},
		{name: "at five intervals", lastSeen: now.Add(-10 * time.Minute), interval: 120, want: AgentLivenessActive},
		{name: "past five intervals", lastSeen: now.Add(-10*time.Minute - time.Second), interval: 120, want: AgentLivenessStale},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DeriveAgentLiveness(test.lastSeen, now, test.interval); got != test.want {
				t.Fatalf("DeriveAgentLiveness() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAgentStaleThresholdUsesFloorAndReportedInterval(t *testing.T) {
	if got := AgentStaleThreshold(30); got != 5*time.Minute {
		t.Fatalf("AgentStaleThreshold(30) = %s, want 5m", got)
	}
	if got := AgentStaleThreshold(120); got != 10*time.Minute {
		t.Fatalf("AgentStaleThreshold(120) = %s, want 10m", got)
	}
}

func TestDeriveAgentVersionDrift(t *testing.T) {
	tests := []struct {
		name    string
		current string
		target  string
		want    AgentVersionDrift
	}{
		{name: "current", current: "6.2.0", target: "6.2.0", want: AgentVersionCurrent},
		{name: "ahead", current: "6.3.0", target: "6.2.0", want: AgentVersionCurrent},
		{name: "behind", current: "6.1.0", target: "6.2.0", want: AgentVersionBehind},
		{name: "missing", current: "", target: "6.2.0", want: AgentVersionUnknown},
		{name: "invalid current", current: "latest", target: "6.2.0", want: AgentVersionUnknown},
		{name: "invalid target", current: "6.2.0", target: "dev", want: AgentVersionUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DeriveAgentVersionDrift(test.current, test.target); got != test.want {
				t.Fatalf("DeriveAgentVersionDrift() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAgentConnectionIDRejectsEmptyIdentity(t *testing.T) {
	if got := AgentConnectionID("  "); got != "" {
		t.Fatalf("AgentConnectionID(empty) = %q, want empty", got)
	}
	if got := AgentConnectionID(" agent-1 "); got != "agent:agent-1" {
		t.Fatalf("AgentConnectionID(agent-1) = %q", got)
	}
}
