package chat

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

func Test_w0716_budget_IsPatrolInvestigationExecution(t *testing.T) {
	tests := []struct {
		name    string
		profile tools.ExecutionProfile
		want    bool
	}{
		{name: "investigation profile matches", profile: tools.ProfilePatrolInvestigation, want: true},
		{name: "interactive assistant is not investigation", profile: tools.ProfileInteractiveAssistant, want: false},
		{name: "patrol detection is not investigation", profile: tools.ProfilePatrolDetection, want: false},
		{name: "zero value falls through to interactive", profile: tools.ProfileInteractiveAssistant, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPatrolInvestigationExecution(tt.profile); got != tt.want {
				t.Fatalf("isPatrolInvestigationExecution(%v) = %v, want %v", tt.profile, got, tt.want)
			}
		})
	}
}

func Test_w0716_budget_IsInvestigationEvidenceTool(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "read-only query consumes evidence budget", in: agentcapabilities.PulseQueryToolName, want: true},
		{name: "arbitrary evidence tool consumes budget", in: "pulse_read", want: true},
		{name: "capability lookup is planning not evidence", in: agentcapabilities.PatrolActionCapabilitiesToolName, want: false},
		{name: "terminal proposal does not consume budget", in: agentcapabilities.PatrolProposeActionToolName, want: false},
		{name: "whitespace-padded proposal still not evidence", in: "  " + agentcapabilities.PatrolProposeActionToolName + "\t", want: false},
		{name: "empty name is not evidence", in: "", want: false},
		{name: "only-whitespace name is not evidence", in: "   ", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInvestigationEvidenceTool(tt.in); got != tt.want {
				t.Fatalf("isInvestigationEvidenceTool(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func Test_w0716_budget_InvestigationTerminalTools(t *testing.T) {
	proposal := providers.Tool{Name: agentcapabilities.PatrolProposeActionToolName}
	query := providers.Tool{Name: agentcapabilities.PulseQueryToolName}
	caps := providers.Tool{Name: agentcapabilities.PatrolActionCapabilitiesToolName}

	tests := []struct {
		name      string
		available []providers.Tool
		wantFound bool
		wantName  string
	}{
		{name: "empty input returns nil", available: nil, wantFound: false},
		{name: "no terminal tool available returns nil", available: []providers.Tool{query, caps}, wantFound: false},
		{name: "terminal tool at index zero", available: []providers.Tool{proposal, query}, wantFound: true, wantName: agentcapabilities.PatrolProposeActionToolName},
		{name: "terminal tool in middle", available: []providers.Tool{query, proposal, caps}, wantFound: true, wantName: agentcapabilities.PatrolProposeActionToolName},
		{name: "terminal tool at end", available: []providers.Tool{query, caps, proposal}, wantFound: true, wantName: agentcapabilities.PatrolProposeActionToolName},
		{name: "duplicate terminal tools returns only the first", available: []providers.Tool{proposal, proposal}, wantFound: true, wantName: agentcapabilities.PatrolProposeActionToolName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := investigationTerminalTools(tt.available)
			if !tt.wantFound {
				if got != nil {
					t.Fatalf("investigationTerminalTools() = %+v, want nil", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("investigationTerminalTools() len = %d, want 1 (only patrol_propose_action): %+v", len(got), got)
			}
			if got[0].Name != tt.wantName {
				t.Fatalf("investigationTerminalTools()[0].Name = %q, want %q", got[0].Name, tt.wantName)
			}
		})
	}
}
