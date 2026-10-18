package reporting

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-pdf/fpdf"
)

const movedAlertSummary = "Alert moved to pve1 (Host Agent). This is not a recovery: check the agent for the current reading."

// movedNodeAlert is a node memory alert that closed because the linked Pulse
// agent took the metric over, as GetRecentlyResolved reports it.
func movedNodeAlert(level string) AlertInfo {
	resolved := time.Now().Add(-2 * time.Minute)
	return AlertInfo{
		Type:         "memory",
		Level:        level,
		Message:      "Node memory at 95%",
		StartTime:    time.Now().Add(-time.Hour),
		ResolvedTime: &resolved,
		ResourceID:   "pve1-node",
		Resolution: &AlertResolution{
			Reason:              AlertResolutionMovedToAgent,
			SuccessorResourceID: "agent:host-1",
			SuccessorName:       "pve1 (Host Agent)",
			Summary:             movedAlertSummary,
		},
	}
}

// A node alert that moved to its Pulse agent within the recently-resolved
// window must not turn the report HEALTHY: nothing saw the condition clear.
func TestHeuristicNarrator_MovedAlertIsNotARecovery(t *testing.T) {
	out, err := HeuristicNarrator{}.Narrate(context.Background(), NarrativeInput{
		MetricStats: map[string]MetricStats{"memory": {Avg: 60, Max: 96}},
		Alerts:      []AlertInfo{movedNodeAlert("warning")},
	})
	if err != nil {
		t.Fatalf("Narrate: %v", err)
	}
	if out.HealthStatus != "WARNING" {
		t.Fatalf("HealthStatus = %q, want WARNING", out.HealthStatus)
	}
	if want := "Alert moved to pve1 (Host Agent) - check the agent for the current reading"; out.HealthMessage != want {
		t.Fatalf("HealthMessage = %q, want %q", out.HealthMessage, want)
	}

	var movedBullet *NarrativeBullet
	for i, bullet := range out.Observations {
		if strings.Contains(bullet.Text, "triggered and resolved") {
			t.Fatalf("moved alert counted as triggered and resolved: %+v", out.Observations)
		}
		if bullet.Text == movedAlertSummary {
			movedBullet = &out.Observations[i]
		}
	}
	if movedBullet == nil || movedBullet.Severity != NarrativeSeverityInfo {
		t.Fatalf("expected an observation recording the handover, got %+v", out.Observations)
	}

	recs := strings.Join(out.Recommendations, "\n")
	if !strings.Contains(recs, "Check the current reading on pve1 (Host Agent)") {
		t.Fatalf("expected a recommendation to check the agent, got %q", recs)
	}
	if strings.Contains(recs, "No immediate action required") {
		t.Fatalf("a moved alert is not a reason to stand down, got %q", recs)
	}

	critical := assessAlertHealth([]AlertInfo{movedNodeAlert("critical")})
	if critical.Status != "CRITICAL" || critical.Critical != 1 {
		t.Fatalf("a moved critical alert keeps its level, got %+v", critical)
	}

	// An ordinary recovery is still a recovery.
	recovered := movedNodeAlert("warning")
	recovered.Resolution = nil
	plain, _ := HeuristicNarrator{}.Narrate(context.Background(), NarrativeInput{
		MetricStats: map[string]MetricStats{"memory": {Avg: 60, Max: 70}},
		Alerts:      []AlertInfo{recovered},
	})
	if plain.HealthStatus != "HEALTHY" || plain.HealthMessage != "All systems operating normally" {
		t.Fatalf("recovered alert should leave the report healthy, got %q / %q", plain.HealthStatus, plain.HealthMessage)
	}
}

// When the report also lists the alert the successor raised for the same
// metric, that alert carries the condition: the moved alert must not count
// it a second time.
func TestUnresolvedAlerts_SuccessorAlertCarriesTheCondition(t *testing.T) {
	agentMemory := AlertInfo{Type: "memory", Level: "warning", Message: "Agent memory at 94%", ResourceID: "agent:host-1"}
	health := assessAlertHealth([]AlertInfo{movedNodeAlert("warning"), agentMemory})
	if health.Warning != 1 || len(health.Unresolved) != 1 || health.Unresolved[0] != agentMemory {
		t.Fatalf("expected only the agent's own alert to count, got %+v", health)
	}
	if health.Message != "1 warning detected - review recommended" {
		t.Fatalf("HealthMessage = %q", health.Message)
	}
	out, _ := HeuristicNarrator{}.Narrate(context.Background(), NarrativeInput{
		MetricStats: map[string]MetricStats{"memory": {Avg: 60, Max: 96}},
		Alerts:      []AlertInfo{movedNodeAlert("warning"), agentMemory},
	})
	if strings.Contains(strings.Join(out.Recommendations, "\n"), "Check the current reading") {
		t.Fatalf("covered handover should not ask to check the agent: %+v", out.Recommendations)
	}

	// The agent's filesystem alerts sit under the agent's resource ID.
	movedDisk := movedNodeAlert("warning")
	movedDisk.Type = "disk"
	agentRoot := AlertInfo{Type: "disk", Level: "warning", ResourceID: "agent:host-1/disk:root"}
	if got := unresolvedAlerts([]AlertInfo{movedDisk, agentRoot}); len(got) != 1 || got[0] != agentRoot {
		t.Fatalf("expected the agent filesystem alert to cover the moved disk alert, got %+v", got)
	}

	// A different metric, or an agent whose ID merely shares a prefix, does
	// not carry the moved alert's condition.
	agentCPU := AlertInfo{Type: "cpu", Level: "warning", ResourceID: "agent:host-1"}
	otherAgent := AlertInfo{Type: "memory", Level: "warning", ResourceID: "agent:host-10"}
	if got := unresolvedAlerts([]AlertInfo{movedNodeAlert("warning"), agentCPU, otherAgent}); len(got) != 3 {
		t.Fatalf("expected the moved alert to stay unresolved beside unrelated alerts, got %+v", got)
	}
}

func TestExecutiveSummary_MovedAlertIsNotHealthy(t *testing.T) {
	data := &ReportData{
		Title:        "pve1",
		ResourceType: "node",
		ResourceID:   "pve1-node",
		Start:        time.Now().Add(-time.Hour),
		End:          time.Now(),
		GeneratedAt:  time.Now(),
		Summary: MetricSummary{ByMetric: map[string]MetricStats{
			"cpu":    {Avg: 5, Max: 12, Count: 60},
			"memory": {Avg: 60, Max: 96, Count: 60},
		}},
		TotalPoints: 120,
		Alerts:      []AlertInfo{movedNodeAlert("warning")},
	}
	// fpdf escapes parentheses inside PDF string literals.
	text := strings.NewReplacer(`\(`, "(", `\)`, ")").Replace(renderExecutiveSummaryText(t, data))
	for _, want := range []string{
		"WARNING",
		"Alert moved to pve1 (Host Agent) - check the agent for the current reading",
		"Node memory at 95% (moved to pve1 (Host Agent))",
		"Moved",
		"Moved: " + movedAlertSummary,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %q in executive summary, got:\n%s", want, text)
		}
	}
	if strings.Contains(text, "HEALTHY") || strings.Contains(text, "All systems operating normally") {
		t.Errorf("a handover must not render the healthy card, got:\n%s", text)
	}

	// The health card does not wrap, so the handover message must fit it.
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetFont("Arial", "", 11)
	pageWidth, _ := pdf.GetPageSize()
	if width := pdf.GetStringWidth(assessAlertHealth(data.Alerts).Message); width > pageWidth-40 {
		t.Fatalf("handover health message is %.1fmm wide, card is %.1fmm", width, pageWidth-40)
	}
}

// A successor's reading only counts from the handover on: an agent recovery
// from before the node alert moved says nothing about the condition since.
func TestUnresolvedAlerts_SuccessorReadingMustFollowTheHandover(t *testing.T) {
	moved := movedNodeAlert("warning")
	before := moved.ResolvedTime.Add(-time.Minute)
	after := moved.ResolvedTime.Add(time.Minute)
	agentRecoveredBefore := AlertInfo{Type: "memory", Level: "warning", ResourceID: "agent:host-1", ResolvedTime: &before}
	agentRecoveredAfter := AlertInfo{Type: "memory", Level: "warning", ResourceID: "agent:host-1", ResolvedTime: &after}

	stale := assessAlertHealth([]AlertInfo{agentRecoveredBefore, moved})
	if stale.Status != "WARNING" || len(stale.Unresolved) != 1 || stale.Unresolved[0] != moved {
		t.Fatalf("an earlier agent recovery must not cover a later handover, got %+v", stale)
	}
	if moved.SuccessorAlertListed([]AlertInfo{agentRecoveredBefore, moved}) {
		t.Fatal("SuccessorAlertListed must ignore a recovery from before the handover")
	}

	cleared := assessAlertHealth([]AlertInfo{agentRecoveredAfter, moved})
	if cleared.Status != "HEALTHY" || len(cleared.Unresolved) != 0 {
		t.Fatalf("an agent recovery after the handover shows the condition cleared, got %+v", cleared)
	}
	if !moved.SuccessorAlertListed([]AlertInfo{agentRecoveredAfter, moved}) {
		t.Fatal("SuccessorAlertListed must accept a recovery after the handover")
	}
}

func TestBuildFleetNarrativeInput_MovedAlertStaysActive(t *testing.T) {
	now := time.Now()
	movedElsewhere := movedNodeAlert("critical")
	movedElsewhere.Resolution = &AlertResolution{
		Reason:              AlertResolutionMovedToAgent,
		SuccessorResourceID: "agent:host-3",
		SuccessorName:       "pve3 (Host Agent)",
		Summary:             "Alert moved to pve3 (Host Agent). This is not a recovery: check the agent for the current reading.",
	}
	multi := &MultiReportData{
		Title: "Fleet",
		Start: now.Add(-time.Hour),
		End:   now,
		Resources: []*ReportData{
			{
				ResourceID:   "pve1-node",
				ResourceType: "node",
				Resource:     &ResourceInfo{Name: "pve1", Status: "online"},
				Alerts:       []AlertInfo{movedNodeAlert("warning")},
			},
			{
				// The agent pve1's alert moved to is its own fleet entry.
				ResourceID:   "agent:host-1",
				ResourceType: "agent",
				Resource:     &ResourceInfo{Name: "pve1 (Host Agent)", Status: "online"},
				Alerts:       []AlertInfo{{Type: "memory", Level: "warning", ResourceID: "agent:host-1"}},
			},
			{
				ResourceID:   "pve3-node",
				ResourceType: "node",
				Resource:     &ResourceInfo{Name: "pve3", Status: "online"},
				Alerts:       []AlertInfo{movedElsewhere},
			},
		},
	}
	in := buildFleetNarrativeInput(multi)
	if in.Aggregate.TotalResolvedAlerts != 0 {
		t.Fatalf("moved alerts are not resolved, got %d", in.Aggregate.TotalResolvedAlerts)
	}
	// pve1's handover is carried by the agent's own alert in another fleet
	// entry; pve3's has no successor reading, so it stays live at its level.
	if in.Aggregate.TotalActiveAlerts != 2 || in.Aggregate.TotalCriticalAlerts != 1 {
		t.Fatalf("expected the agent alert and pve3's moved alert active, got active=%d critical=%d",
			in.Aggregate.TotalActiveAlerts, in.Aggregate.TotalCriticalAlerts)
	}
	if got := in.Resources[0]; got.ActiveAlerts != 0 || len(got.MovedTo) != 0 {
		t.Fatalf("pve1's covered handover must not count again, got %+v", got)
	}
	if got := in.Resources[2]; got.ActiveAlerts != 1 || len(got.MovedTo) != 1 || got.MovedTo[0] != "pve3 (Host Agent)" {
		t.Fatalf("pve3 should keep its moved alert and name the agent, got %+v", got)
	}

	out, _ := HeuristicFleetNarrator{}.NarrateFleet(context.Background(), in)
	if out.HealthStatus != "CRITICAL" {
		t.Fatalf("fleet HealthStatus = %q, want CRITICAL", out.HealthStatus)
	}
	recs := strings.Join(out.Recommendations, "\n")
	if !strings.Contains(recs, "Check the current reading on pve3 (Host Agent)") {
		t.Fatalf("expected a recommendation naming pve3's agent, got %q", recs)
	}
	if strings.Contains(recs, "pve1 (Host Agent)") || strings.Contains(recs, "No fleet-wide action required") {
		t.Fatalf("unexpected recommendation, got %q", recs)
	}

	// A covered handover leaves only the agent's open warning: still no
	// all-clear in the patterns or the recommendations.
	multi.Resources = multi.Resources[:2]
	covered, _ := HeuristicFleetNarrator{}.NarrateFleet(context.Background(), buildFleetNarrativeInput(multi))
	if covered.HealthStatus != "WARNING" {
		t.Fatalf("fleet HealthStatus = %q, want WARNING", covered.HealthStatus)
	}
	for _, pattern := range covered.Patterns {
		if strings.Contains(pattern.Text, "nominal thresholds") {
			t.Fatalf("an open alert rules out the all-clear pattern, got %+v", covered.Patterns)
		}
	}
	if recs := strings.Join(covered.Recommendations, "\n"); strings.Contains(recs, "No fleet-wide action required") ||
		!strings.Contains(recs, "Review the active alerts") {
		t.Fatalf("an open alert needs a review recommendation, got %q", recs)
	}
}
