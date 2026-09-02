package api

import (
	"fmt"
	"html"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// renderPatrolDigestEmail turns the digest into the weekly "what Patrol did for
// you" email. It mirrors the Patrol page's This week card line for line and
// uses the same plain vocabulary: no evidence classes, verdicts, or model names.
// The mode sentence is included because, unlike the page, the email has no
// header that already states it.
func renderPatrolDigestEmail(digest ai.PatrolDigest, publicURL string) (subject, htmlBody, textBody string) {
	publicURL = strings.TrimRight(strings.TrimSpace(publicURL), "/")
	lines := patrolDigestEmailLines(digest)
	subject = patrolDigestEmailSubject(digest)

	var text strings.Builder
	text.WriteString("What Patrol did for you. " + patrolDigestWindowLabel(digest) + ".\n\n")
	for _, line := range lines {
		text.WriteString(line.label + ": " + line.value + "\n")
		for _, detail := range line.details {
			text.WriteString("  " + detail + "\n")
		}
		text.WriteString("\n")
	}
	text.WriteString(patrolDigestModeSentence(digest.Mode) + "\n")
	if publicURL != "" {
		text.WriteString("\nOpen Patrol: " + publicURL + "/patrol\n")
		if digest.Actions.Pending > 0 {
			text.WriteString("Review waiting fixes: " + publicURL + "/actions\n")
		}
	}
	textBody = text.String()

	var b strings.Builder
	b.WriteString(`<div style="font-family:-apple-system,Segoe UI,Helvetica,Arial,sans-serif;max-width:560px;color:#111827">`)
	b.WriteString(`<h2 style="margin:0 0 4px">This week</h2>`)
	b.WriteString(`<p style="margin:0 0 16px;color:#6b7280">What Patrol did for you. ` + html.EscapeString(patrolDigestWindowLabel(digest)) + `.</p>`)
	for _, line := range lines {
		b.WriteString(`<p style="margin:0 0 12px"><strong>` + html.EscapeString(line.label) + `:</strong> ` + html.EscapeString(line.value))
		for _, detail := range line.details {
			b.WriteString(`<br><span style="color:#6b7280">` + html.EscapeString(detail) + `</span>`)
		}
		b.WriteString(`</p>`)
	}
	b.WriteString(`<p style="margin:16px 0 0;color:#6b7280">` + html.EscapeString(patrolDigestModeSentence(digest.Mode)) + `</p>`)
	if publicURL != "" {
		safe := html.EscapeString(publicURL)
		b.WriteString(`<p style="margin:16px 0 0"><a href="` + safe + `/patrol">Open Patrol</a>`)
		if digest.Actions.Pending > 0 {
			b.WriteString(` &middot; <a href="` + safe + `/actions">Review waiting fixes</a>`)
		}
		b.WriteString(`</p>`)
	}
	b.WriteString(`</div>`)
	htmlBody = b.String()
	return subject, htmlBody, textBody
}

type patrolDigestEmailLine struct {
	label   string
	value   string
	details []string
}

func patrolDigestEmailSubject(digest ai.PatrolDigest) string {
	if digest.Runs.Total == 0 {
		return fmt.Sprintf("Pulse: Patrol has not run in the last %d days", digest.Window.Days)
	}
	return fmt.Sprintf("Pulse: what Patrol did this week (%s, %s)",
		pluralCount(digest.Runs.Total, "run", "runs"),
		pluralCount(digest.Findings.New, "new issue", "new issues"))
}

func patrolDigestWindowLabel(digest ai.PatrolDigest) string {
	if !digest.Window.HistoryComplete && digest.Window.HistorySince != nil {
		return "Since " + digest.Window.HistorySince.UTC().Format("2 Jan") + " (older runs are no longer kept)"
	}
	return fmt.Sprintf("Last %d days", digest.Window.Days)
}

func patrolDigestEmailLines(digest ai.PatrolDigest) []patrolDigestEmailLine {
	runs := patrolDigestEmailLine{label: "Patrol runs", value: fmt.Sprintf("%d", digest.Runs.Total)}
	runs.details = append(runs.details, fmt.Sprintf("%s across %s.", pluralCount(digest.Runs.Checks, "check", "checks"), pluralCount(digest.Runs.ResourcesCovered, "resource", "resources")))
	if digest.Alerts.Reviewed > 0 {
		runs.details = append(runs.details, pluralCount(digest.Alerts.Reviewed, "alert", "alerts")+" looked into.")
	}
	if digest.Runs.Failed > 0 {
		runs.details = append(runs.details, pluralCount(digest.Runs.Failed, "run", "runs")+" failed.")
	}
	if digest.Runs.LastRunAt != nil {
		runs.details = append(runs.details, "Last run "+digest.Runs.LastRunAt.UTC().Format("2 Jan 15:04 UTC")+".")
	}

	open := digest.Findings.OpenBySeverity
	openTotal := open.Critical + open.Warning + open.Watch + open.Info
	newLine := patrolDigestEmailLine{label: "New issues", value: fmt.Sprintf("%d", digest.Findings.New)}
	switch {
	case digest.Findings.New == 0:
		newLine.details = []string{"Nothing new was raised."}
	case openTotal == 0:
		newLine.details = []string{"All of them have since cleared."}
	default:
		parts := []string{}
		if open.Critical > 0 {
			parts = append(parts, fmt.Sprintf("%d critical", open.Critical))
		}
		if open.Warning > 0 {
			parts = append(parts, fmt.Sprintf("%d warning", open.Warning))
		}
		detail := fmt.Sprintf("%d still open", openTotal)
		if len(parts) > 0 {
			detail += " (" + strings.Join(parts, ", ") + ")"
		}
		newLine.details = []string{detail + "."}
	}

	resolved := patrolDigestEmailLine{label: "Issues resolved", value: fmt.Sprintf("%d", digest.Findings.Resolved)}
	if digest.Findings.AutoResolved > 0 {
		resolved.details = append(resolved.details, fmt.Sprintf("%d cleared by Patrol on its own.", digest.Findings.AutoResolved))
	}
	if digest.Findings.Dismissed > 0 {
		resolved.details = append(resolved.details, fmt.Sprintf("%d dismissed by you.", digest.Findings.Dismissed))
	}
	if digest.Findings.Suppressed > 0 {
		resolved.details = append(resolved.details, fmt.Sprintf("%d muted for good.", digest.Findings.Suppressed))
	}
	if len(resolved.details) == 0 {
		if digest.Findings.Resolved > 0 {
			resolved.details = []string{"Resolved by you."}
		} else {
			resolved.details = []string{"No issues were resolved this period."}
		}
	}

	investigated := patrolDigestEmailLine{label: "Investigated", value: fmt.Sprintf("%d", digest.Investigations.Total)}
	outcomeCopy := map[string]string{
		"needs_attention": "need you", "fix_failed": "fix failed", "fix_verification_failed": "fix not confirmed",
		"cannot_fix": "could not fix", "timed_out": "timed out", "fix_queued": "fix waiting for approval",
		"fix_executed": "fix run", "fix_verification_unknown": "fix run, result unknown", "fix_verified": "fixed and verified",
		"resolved": "resolved", "fix_rejected": "fix declined",
	}
	outcomeDetails := []string{}
	for _, key := range ai.PatrolDigestOutcomeOrder(digest.Investigations.ByOutcome) {
		label, ok := outcomeCopy[key]
		if !ok {
			continue
		}
		outcomeDetails = append(outcomeDetails, fmt.Sprintf("%d %s", digest.Investigations.ByOutcome[key], label))
		if len(outcomeDetails) == 2 {
			break
		}
	}
	switch {
	case len(outcomeDetails) > 0:
		investigated.details = []string{strings.Join(outcomeDetails, ", ") + "."}
	case digest.Mode == config.PatrolAutonomyMonitor:
		investigated.details = []string{"Patrol is watch only, so it reports issues without investigating them."}
	default:
		investigated.details = []string{"No issues needed a closer look."}
	}

	fixes := patrolDigestEmailLine{label: "Fixes run", value: fmt.Sprintf("%d", digest.Actions.Executed)}
	if digest.Actions.Executed > 0 {
		fixes.details = append(fixes.details, fmt.Sprintf("%d of %d verified afterwards.", digest.Actions.Verified, digest.Actions.Executed))
	}
	if digest.Actions.Failed > 0 {
		fixes.details = append(fixes.details, pluralCount(digest.Actions.Failed, "action", "actions")+" failed.")
	}
	if digest.Actions.Rejected > 0 {
		fixes.details = append(fixes.details, fmt.Sprintf("%d declined by you.", digest.Actions.Rejected))
	}
	if digest.Actions.Pending > 0 {
		fixes.details = append(fixes.details, pluralCount(digest.Actions.Pending, "fix", "fixes")+" waiting for your approval.")
	}
	if len(fixes.details) == 0 {
		if digest.Mode == config.PatrolAutonomyMonitor {
			fixes.details = []string{"Patrol is watch only, so no fixes were proposed."}
		} else {
			fixes.details = []string{"No fixes were needed."}
		}
	}

	spend := patrolDigestEmailLine{label: "Estimated spend", value: fmt.Sprintf("$%.2f", digest.Spend.EstimatedUSD)}
	spend.details = append(spend.details, pluralCount(digest.Spend.Calls, "model call", "model calls")+".")
	if digest.Spend.Calls > 0 && !digest.Spend.PricingKnown {
		spend.details = append(spend.details, "Some calls used a model with no known price.")
	}

	return []patrolDigestEmailLine{runs, newLine, resolved, investigated, fixes, spend}
}

func patrolDigestModeSentence(mode string) string {
	switch mode {
	case config.PatrolAutonomyApproval:
		return "Ask first. Patrol investigates and prepares fixes, but every change waits for your approval."
	case config.PatrolAutonomyAssisted:
		return "Safe auto-fix. Patrol can run low- or medium-risk fixes allowed by policy. Higher-risk work still asks first."
	case config.PatrolAutonomyFull:
		return "Autopilot. Patrol can act automatically within policy and still asks when approval is required."
	default:
		return "Watch only. Patrol checks infrastructure and reports issues only. It does not start fixes."
	}
}

func pluralCount(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}
