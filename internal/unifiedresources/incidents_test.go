package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

// --- mergeResourceIncidents ---

func TestMergeResourceIncidents_BothEmpty(t *testing.T) {
	merged := mergeResourceIncidents(nil, nil)
	if len(merged) != 0 {
		t.Errorf("expected 0 incidents, got %d", len(merged))
	}
}

func TestMergeResourceIncidents_ExistingEmpty(t *testing.T) {
	incoming := []ResourceIncident{
		{Provider: "p1", Code: "c1", Summary: "s1"},
	}
	merged := mergeResourceIncidents(nil, incoming)
	if len(merged) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(merged))
	}
	if merged[0].Code != "c1" {
		t.Errorf("expected code c1, got %s", merged[0].Code)
	}
}

func TestMergeResourceIncidents_IncomingEmpty(t *testing.T) {
	existing := []ResourceIncident{
		{Provider: "p1", Code: "c1", Summary: "s1"},
	}
	merged := mergeResourceIncidents(existing, nil)
	if len(merged) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(merged))
	}
}

func TestMergeResourceIncidents_Dedup(t *testing.T) {
	existing := []ResourceIncident{
		{Provider: "p1", NativeID: "n1", Code: "c1", Summary: "same"},
	}
	incoming := []ResourceIncident{
		{Provider: "p1", NativeID: "n1", Code: "c1", Summary: "same"}, // duplicate
		{Provider: "p1", NativeID: "n1", Code: "c2", Summary: "diff"}, // different code
	}
	merged := mergeResourceIncidents(existing, incoming)
	if len(merged) != 2 {
		t.Errorf("expected 2 incidents (1 deduped), got %d", len(merged))
	}
}

func TestMergeResourceIncidents_ClonesSafety(t *testing.T) {
	existing := []ResourceIncident{
		{Provider: "p1", Code: "c1", Summary: "original"},
	}
	merged := mergeResourceIncidents(existing, nil)
	merged[0].Summary = "modified"
	if existing[0].Summary != "original" {
		t.Error("merging should clone, not share slices")
	}
}

// --- incidentSeverityRank ---

func TestIncidentSeverityRank_Ordering(t *testing.T) {
	levels := []storagehealth.RiskLevel{
		storagehealth.RiskHealthy,
		storagehealth.RiskMonitor,
		storagehealth.RiskWarning,
		storagehealth.RiskCritical,
	}
	for i := 1; i < len(levels); i++ {
		if incidentSeverityRank(levels[i]) <= incidentSeverityRank(levels[i-1]) {
			t.Errorf("%s should rank higher than %s", levels[i], levels[i-1])
		}
	}
}

func TestIncidentSeverityRank_Unknown(t *testing.T) {
	if incidentSeverityRank("unknown") != 0 {
		t.Error("unknown level should have rank 0")
	}
	if incidentSeverityRank("") != 0 {
		t.Error("empty level should have rank 0")
	}
}

// --- highestIncidentSeverity ---

func TestHighestIncidentSeverity_EmptySlice(t *testing.T) {
	got := highestIncidentSeverity(nil)
	if got != storagehealth.RiskHealthy {
		t.Errorf("expected healthy for empty, got %s", got)
	}
}

func TestHighestIncidentSeverity_PicksHighest(t *testing.T) {
	incidents := []ResourceIncident{
		{Severity: storagehealth.RiskMonitor},
		{Severity: storagehealth.RiskCritical},
		{Severity: storagehealth.RiskWarning},
	}
	got := highestIncidentSeverity(incidents)
	if got != storagehealth.RiskCritical {
		t.Errorf("expected critical, got %s", got)
	}
}

// --- IncidentsStatus ---

func TestIncidentsStatus_NoIncidents(t *testing.T) {
	if got := IncidentsStatus(StatusOnline, nil); got != StatusOnline {
		t.Errorf("expected online, got %s", got)
	}
}

func TestIncidentsStatus_WarningIncidentOnOnlineResource(t *testing.T) {
	incidents := []ResourceIncident{
		{Severity: storagehealth.RiskWarning, Summary: "something wrong"},
	}
	got := IncidentsStatus(StatusOnline, incidents)
	if got != StatusWarning {
		t.Errorf("expected warning when online resource has warning incident, got %s", got)
	}
}

func TestIncidentsStatus_CriticalIncidentOnOnlineResource(t *testing.T) {
	incidents := []ResourceIncident{
		{Severity: storagehealth.RiskCritical, Summary: "critical issue"},
	}
	got := IncidentsStatus(StatusOnline, incidents)
	if got != StatusWarning {
		t.Errorf("expected warning when online resource has critical incident, got %s", got)
	}
}

func TestIncidentsStatus_WarningDoesNotDowngradeOffline(t *testing.T) {
	incidents := []ResourceIncident{
		{Severity: storagehealth.RiskWarning, Summary: "issue"},
	}
	got := IncidentsStatus(StatusOffline, incidents)
	if got != StatusOffline {
		t.Errorf("warning should not change offline status, got %s", got)
	}
}

func TestIncidentsStatus_MonitorDoesNotChangeOnline(t *testing.T) {
	incidents := []ResourceIncident{
		{Severity: storagehealth.RiskMonitor, Summary: "minor"},
	}
	got := IncidentsStatus(StatusOnline, incidents)
	if got != StatusOnline {
		t.Errorf("monitor severity should not change online to warning, got %s", got)
	}
}

// --- incidentsFromAssessment ---

func TestIncidentsFromAssessment_EmptyReasons(t *testing.T) {
	got := incidentsFromAssessment("prov", "src", "prefix",
		storagehealth.Assessment{}, time.Now())
	if got != nil {
		t.Errorf("expected nil for empty assessment, got %v", got)
	}
}

func TestIncidentsFromAssessment_CreatesIncidents(t *testing.T) {
	now := time.Now()
	assessment := storagehealth.Assessment{
		Reasons: []storagehealth.Reason{
			{Code: "raid_degraded", Severity: storagehealth.RiskCritical, Summary: "RAID degraded"},
			{Code: "disk_hot", Severity: storagehealth.RiskWarning, Summary: "Disk temp high"},
		},
	}
	incidents := incidentsFromAssessment("proxmox", "pve", "node1", assessment, now)
	if len(incidents) != 2 {
		t.Fatalf("expected 2 incidents, got %d", len(incidents))
	}

	// Verify first incident.
	inc := incidents[0]
	if inc.Provider != "proxmox" {
		t.Errorf("expected provider 'proxmox', got %q", inc.Provider)
	}
	if inc.NativeID != "node1:raid_degraded" {
		t.Errorf("expected nativeID 'node1:raid_degraded', got %q", inc.NativeID)
	}
	if inc.Code != "raid_degraded" {
		t.Errorf("expected code 'raid_degraded', got %q", inc.Code)
	}
	if inc.Severity != storagehealth.RiskCritical {
		t.Errorf("expected critical, got %s", inc.Severity)
	}
}

func TestIncidentsFromAssessment_SkipsEmptyCodeOrSummary(t *testing.T) {
	assessment := storagehealth.Assessment{
		Reasons: []storagehealth.Reason{
			{Code: "", Severity: storagehealth.RiskWarning, Summary: "No code"},
			{Code: "valid", Severity: storagehealth.RiskWarning, Summary: ""},
			{Code: "ok", Severity: storagehealth.RiskWarning, Summary: "Valid"},
		},
	}
	incidents := incidentsFromAssessment("p", "s", "", assessment, time.Now())
	if len(incidents) != 1 {
		t.Errorf("expected 1 incident (skipping empty code/summary), got %d", len(incidents))
	}
	if incidents[0].Code != "ok" {
		t.Errorf("expected code 'ok', got %q", incidents[0].Code)
	}
}

func TestIncidentsFromAssessment_EmptyPrefix(t *testing.T) {
	assessment := storagehealth.Assessment{
		Reasons: []storagehealth.Reason{
			{Code: "test_code", Severity: storagehealth.RiskMonitor, Summary: "Test"},
		},
	}
	incidents := incidentsFromAssessment("p", "s", "", assessment, time.Now())
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	// With empty prefix, nativeID should just be the code.
	if incidents[0].NativeID != "test_code" {
		t.Errorf("expected nativeID 'test_code', got %q", incidents[0].NativeID)
	}
}

// --- refreshResourceIncidentRollup ---

func TestRefreshResourceIncidentRollup_NilResource(t *testing.T) {
	// Should not panic on nil.
	refreshResourceIncidentRollup(nil)
}

func TestRefreshResourceIncidentRollup_NoIncidents(t *testing.T) {
	resource := &Resource{
		Incidents:        nil,
		IncidentCode:     "leftover",
		IncidentSeverity: storagehealth.RiskWarning,
		IncidentSummary:  "old",
	}
	refreshResourceIncidentRollup(resource)
	if resource.IncidentCount != 0 {
		t.Errorf("expected count 0, got %d", resource.IncidentCount)
	}
	if resource.IncidentCode != "" {
		t.Errorf("expected empty code, got %q", resource.IncidentCode)
	}
	if resource.IncidentSeverity != "" {
		t.Errorf("expected empty severity, got %q", resource.IncidentSeverity)
	}
	if resource.IncidentSummary != "" {
		t.Errorf("expected empty summary, got %q", resource.IncidentSummary)
	}
}

func TestRefreshResourceIncidentRollup_SingleIncident(t *testing.T) {
	resource := &Resource{
		Incidents: []ResourceIncident{
			{Code: "raid_degraded", Severity: storagehealth.RiskCritical, Summary: "Array failed"},
		},
	}
	refreshResourceIncidentRollup(resource)
	if resource.IncidentCount != 1 {
		t.Errorf("expected count 1, got %d", resource.IncidentCount)
	}
	if resource.IncidentCode != "raid_degraded" {
		t.Errorf("expected code 'raid_degraded', got %q", resource.IncidentCode)
	}
	if resource.IncidentSeverity != storagehealth.RiskCritical {
		t.Errorf("expected critical, got %s", resource.IncidentSeverity)
	}
}

func TestRefreshResourceIncidentRollup_PicksHighestSeverity(t *testing.T) {
	resource := &Resource{
		Incidents: []ResourceIncident{
			{Code: "minor", Severity: storagehealth.RiskMonitor, Summary: "Minor issue"},
			{Code: "critical", Severity: storagehealth.RiskCritical, Summary: "Critical issue"},
			{Code: "warning", Severity: storagehealth.RiskWarning, Summary: "Warning issue"},
		},
	}
	refreshResourceIncidentRollup(resource)
	if resource.IncidentSeverity != storagehealth.RiskCritical {
		t.Errorf("expected critical severity, got %s", resource.IncidentSeverity)
	}
	if resource.IncidentCode != "critical" {
		t.Errorf("expected code 'critical', got %q", resource.IncidentCode)
	}
}
