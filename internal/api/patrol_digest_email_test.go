package api

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestRenderPatrolDigestEmailMirrorsTheCardInPlainLanguage(t *testing.T) {
	last := time.Date(2026, 9, 1, 11, 0, 0, 0, time.UTC)
	digest := ai.PatrolDigest{
		Window: ai.PatrolDigestWindow{Days: 7, HistoryComplete: true},
		Mode:   config.PatrolAutonomyApproval,
		Runs:   ai.PatrolDigestRuns{Total: 38, Checks: 1520, ResourcesCovered: 40, Failed: 1, LastRunAt: &last},
		Findings: ai.PatrolDigestFindings{
			New: 12, OpenBySeverity: ai.PatrolDigestSeverityCounts{Critical: 1, Warning: 3}, Resolved: 9, AutoResolved: 7, Dismissed: 2,
		},
		Investigations: ai.PatrolDigestInvestigations{Total: 4, ByOutcome: map[string]int{"fix_verified": 2, "needs_attention": 1, "resolved": 1}},
		Actions:        ai.PatrolDigestActions{Executed: 2, Verified: 1, Pending: 1},
		Alerts:         ai.PatrolDigestAlerts{Reviewed: 5},
		Spend:          ai.PatrolDigestSpend{EstimatedUSD: 1.2345, PricingKnown: true, Calls: 40},
	}

	subject, htmlBody, textBody := renderPatrolDigestEmail(digest, "https://pulse.example.com/")

	if subject != "Pulse: what Patrol did this week (38 runs, 12 new issues)" {
		t.Fatalf("subject = %q", subject)
	}
	for _, want := range []string{
		"Patrol runs: 38", "1520 checks across 40 resources.", "5 alerts looked into.", "1 run failed.",
		"New issues: 12", "4 still open (1 critical, 3 warning).",
		"Issues resolved: 9", "7 cleared by Patrol on its own.", "2 dismissed by you.",
		"Investigated: 4", "1 need you, 2 fixed and verified.",
		"Fixes run: 2", "1 of 2 verified afterwards.", "1 fix waiting for your approval.",
		"Estimated spend: $1.23", "40 model calls.",
		"Ask first. Patrol investigates and prepares fixes, but every change waits for your approval.",
		"Open Patrol: https://pulse.example.com/patrol", "Review waiting fixes: https://pulse.example.com/actions",
	} {
		if !strings.Contains(textBody, want) {
			t.Fatalf("text body missing %q:\n%s", want, textBody)
		}
	}
	for _, want := range []string{`href="https://pulse.example.com/patrol"`, `href="https://pulse.example.com/actions"`, "<strong>New issues:</strong> 12"} {
		if !strings.Contains(htmlBody, want) {
			t.Fatalf("html body missing %q:\n%s", want, htmlBody)
		}
	}
	for _, forbidden := range []string{"evidence class", "verdict", "agent_attested", "fix_verified"} {
		if strings.Contains(strings.ToLower(textBody), forbidden) || strings.Contains(strings.ToLower(htmlBody), forbidden) {
			t.Fatalf("email leaks forensic vocabulary %q", forbidden)
		}
	}
}

func TestRenderPatrolDigestEmailHandlesQuietWatchOnlyWeeks(t *testing.T) {
	since := time.Date(2026, 8, 29, 8, 0, 0, 0, time.UTC)
	digest := ai.PatrolDigest{
		Window:         ai.PatrolDigestWindow{Days: 7, HistoryComplete: false, HistorySince: &since},
		Mode:           config.PatrolAutonomyMonitor,
		Runs:           ai.PatrolDigestRuns{Total: 3, Checks: 30, ResourcesCovered: 10},
		Investigations: ai.PatrolDigestInvestigations{ByOutcome: map[string]int{}},
		Spend:          ai.PatrolDigestSpend{PricingKnown: false, Calls: 3, EstimatedUSD: 0.5},
	}
	subject, htmlBody, textBody := renderPatrolDigestEmail(digest, "<script>")
	if subject != "Pulse: what Patrol did this week (3 runs, 0 new issues)" {
		t.Fatalf("subject = %q", subject)
	}
	for _, want := range []string{
		"Since 29 Aug (older runs are no longer kept)", "Nothing new was raised.", "No issues were resolved this period.",
		"Patrol is watch only, so it reports issues without investigating them.", "Patrol is watch only, so no fixes were proposed.",
		"Some calls used a model with no known price.", "Watch only. Patrol checks infrastructure and reports issues only.",
	} {
		if !strings.Contains(textBody, want) {
			t.Fatalf("text body missing %q:\n%s", want, textBody)
		}
	}
	if strings.Contains(htmlBody, "<script>") {
		t.Fatal("public URL must be HTML-escaped")
	}

	empty := ai.PatrolDigest{Window: ai.PatrolDigestWindow{Days: 7, HistoryComplete: true}, Investigations: ai.PatrolDigestInvestigations{ByOutcome: map[string]int{}}, Spend: ai.PatrolDigestSpend{PricingKnown: true}}
	subject, _, _ = renderPatrolDigestEmail(empty, "")
	if subject != "Pulse: Patrol has not run in the last 7 days" {
		t.Fatalf("empty subject = %q", subject)
	}
}
