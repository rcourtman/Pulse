package chat

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

// msgWithResult builds a providers.Message carrying a non-nil tool result so
// appendInvestigationBudgetMessage has something to mutate.
func msgWithResult(id, content string, isError bool) providers.Message {
	return providers.Message{
		Role:       "user",
		ToolResult: &providers.ToolResult{ToolUseID: id, Content: content, IsError: isError},
	}
}

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
		{name: "terminal proposal does not consume budget", in: agentcapabilities.PatrolProposeActionToolName, want: false},
		{name: "whitespace-padded proposal still not evidence", in: "  " + agentcapabilities.PatrolProposeActionToolName + "\t", want: false},
		{name: "empty name treated as evidence tool", in: "", want: true},
		{name: "only-whitespace name treated as evidence tool", in: "   ", want: true},
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

func Test_w0716_budget_InvestigationEvidenceCheckpoint(t *testing.T) {
	tests := []struct {
		name string
		max  int
		want int
	}{
		{name: "zero max clamps to floor", max: 0, want: 3},
		{name: "negative max clamps to floor", max: -5, want: 3},
		{name: "max 1 clamps to floor", max: 1, want: 3},
		{name: "max 4 half-rounded-down still clamps", max: 4, want: 3},
		{name: "max 5 hits floor via normal path", max: 5, want: 3},
		{name: "max 6 still at floor", max: 6, want: 3},
		{name: "max 7 exceeds floor", max: 7, want: 4},
		{name: "max 15 halves up", max: 15, want: 8},
		{name: "large budget halves", max: 100, want: 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := investigationEvidenceCheckpoint(tt.max); got != tt.want {
				t.Fatalf("investigationEvidenceCheckpoint(%d) = %d, want %d", tt.max, got, tt.want)
			}
		})
	}
}

func Test_w0716_budget_AppendInvestigationBudgetMessage(t *testing.T) {
	const guidance = "[budget note]"

	t.Run("empty messages returns false and mutates nothing", func(t *testing.T) {
		var msgs []providers.Message
		if appendInvestigationBudgetMessage(msgs, guidance, "phase") {
			t.Fatal("expected false on empty messages")
		}
	})

	t.Run("only nil tool results returns false", func(t *testing.T) {
		msgs := []providers.Message{
			{Role: "user", Content: "plain"},
			{Role: "assistant", Content: "prose"},
		}
		if appendInvestigationBudgetMessage(msgs, guidance, "phase") {
			t.Fatal("expected false when no message carries a tool result")
		}
		if msgs[0].Content != "plain" || msgs[1].Content != "prose" {
			t.Fatalf("non-result content was mutated: %+v", msgs)
		}
	})

	t.Run("only error tool results returns false", func(t *testing.T) {
		msgs := []providers.Message{msgWithResult("err-1", "boom", true)}
		if appendInvestigationBudgetMessage(msgs, guidance, "phase") {
			t.Fatal("expected false when the only result is an error")
		}
		if msgs[0].ToolResult.Content != "boom" {
			t.Fatalf("error result content was mutated: %q", msgs[0].ToolResult.Content)
		}
	})

	t.Run("appends guidance to the most recent non-error result only", func(t *testing.T) {
		msgs := []providers.Message{
			msgWithResult("ok-early", "early-ok", false),
			msgWithResult("err-mid", "mid-err", true),
			msgWithResult("ok-late", "late-ok", false),
		}
		if !appendInvestigationBudgetMessage(msgs, guidance, "phase") {
			t.Fatal("expected true when a non-error result exists")
		}
		wantEarly := "early-ok"
		wantLate := "late-ok" + "\n\n" + guidance
		if msgs[0].ToolResult.Content != wantEarly {
			t.Fatalf("earlier non-error result was mutated: got %q, want %q", msgs[0].ToolResult.Content, wantEarly)
		}
		if msgs[2].ToolResult.Content != wantLate {
			t.Fatalf("latest non-error result = %q, want %q", msgs[2].ToolResult.Content, wantLate)
		}
		if msgs[1].ToolResult.Content != "mid-err" {
			t.Fatalf("error result content was mutated: %q", msgs[1].ToolResult.Content)
		}
	})

	t.Run("skips trailing error to mutate earlier non-error result", func(t *testing.T) {
		msgs := []providers.Message{
			msgWithResult("ok-0", "first-ok", false),
			msgWithResult("err-1", "trailing-err", true),
		}
		if !appendInvestigationBudgetMessage(msgs, guidance, "phase") {
			t.Fatal("expected true when an earlier non-error result exists")
		}
		if want := "first-ok" + "\n\n" + guidance; msgs[0].ToolResult.Content != want {
			t.Fatalf("earlier result = %q, want %q", msgs[0].ToolResult.Content, want)
		}
		if msgs[1].ToolResult.Content != "trailing-err" {
			t.Fatalf("trailing error result was mutated: %q", msgs[1].ToolResult.Content)
		}
	})

	t.Run("mutates exactly one result when several qualify", func(t *testing.T) {
		msgs := []providers.Message{
			msgWithResult("ok-a", "a-ok", false),
			msgWithResult("ok-b", "b-ok", false),
		}
		if !appendInvestigationBudgetMessage(msgs, guidance, "phase") {
			t.Fatal("expected true")
		}
		if msgs[0].ToolResult.Content != "a-ok" {
			t.Fatalf("first qualifying result should be untouched: %q", msgs[0].ToolResult.Content)
		}
		if want := "b-ok" + "\n\n" + guidance; msgs[1].ToolResult.Content != want {
			t.Fatalf("only the last qualifying result should be mutated: got %q, want %q", msgs[1].ToolResult.Content, want)
		}
	})
}

func Test_w0716_budget_MaybeInjectInvestigationEvidenceCheckpoint(t *testing.T) {
	t.Run("injects checkpoint guidance with used and remaining counts", func(t *testing.T) {
		msgs := []providers.Message{msgWithResult("evidence-1", "raw-evidence", false)}
		if !maybeInjectInvestigationEvidenceCheckpoint(msgs, 4, 3) {
			t.Fatal("expected checkpoint injection to return true")
		}
		got := msgs[0].ToolResult.Content
		if !strings.Contains(got, "raw-evidence") {
			t.Fatalf("original content must be preserved, got %q", got)
		}
		for _, want := range []string{"4 evidence calls used", "3 remain", "checkpoint", "completion questions"} {
			if !strings.Contains(got, want) {
				t.Fatalf("checkpoint content missing %q: %q", want, got)
			}
		}
		if !strings.HasPrefix(got, "raw-evidence\n\n") {
			t.Fatalf("guidance must be appended after a blank line, got %q", got)
		}
	})

	t.Run("returns false when no injectable result exists", func(t *testing.T) {
		msgs := []providers.Message{
			{Role: "user", Content: "no result here"},
			msgWithResult("err", "boom", true),
		}
		if maybeInjectInvestigationEvidenceCheckpoint(msgs, 1, 0) {
			t.Fatal("expected false when no non-error tool result is present")
		}
		if msgs[1].ToolResult.Content != "boom" {
			t.Fatalf("error result must not be mutated: %q", msgs[1].ToolResult.Content)
		}
	})
}

func Test_w0716_budget_MaybeInjectInvestigationEvidenceBudgetWarning(t *testing.T) {
	t.Run("injects budget warning with used and remaining counts", func(t *testing.T) {
		msgs := []providers.Message{
			msgWithResult("evidence-early", "first", false),
			msgWithResult("evidence-late", "second", false),
		}
		if !maybeInjectInvestigationEvidenceBudgetWarning(msgs, 9, 1) {
			t.Fatal("expected warning injection to return true")
		}
		if msgs[0].ToolResult.Content != "first" {
			t.Fatalf("earlier result must be untouched, got %q", msgs[0].ToolResult.Content)
		}
		got := msgs[1].ToolResult.Content
		for _, want := range []string{"9 evidence calls used", "1 remain", "budget", "Stop exploratory investigation", "typed proposal"} {
			if !strings.Contains(got, want) {
				t.Fatalf("warning content missing %q: %q", want, got)
			}
		}
	})

	t.Run("returns false on empty messages", func(t *testing.T) {
		if maybeInjectInvestigationEvidenceBudgetWarning(nil, 5, 0) {
			t.Fatal("expected false on empty messages")
		}
	})

	t.Run("returns false when all results are errors", func(t *testing.T) {
		msgs := []providers.Message{msgWithResult("e1", "err1", true), msgWithResult("e2", "err2", true)}
		if maybeInjectInvestigationEvidenceBudgetWarning(msgs, 5, 0) {
			t.Fatal("expected false when every result is an error")
		}
	})
}
