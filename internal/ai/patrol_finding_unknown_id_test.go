package ai

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// A fresh Patrol run filed one backup finding for three guests as resource_id
// "ct/gitea", resource_name "gitea (and nextcloud, pihole)". The triage tables
// name resources without IDs, so the ID was a guess. The refusal said the
// resource was outside the patrol scope, the model tried
// "system-container/gitea" and gave up, and the real finding was lost. An ID
// that names nothing now gets the in-scope resources the report mentions, with
// their IDs, and a retry with one of them is filed.
func TestPatrolFindingUnknownIDNamesTheResourcesItMeant(t *testing.T) {
	state := newPatrolRuntimeState(models.StateSnapshot{
		Nodes: []models.Node{{ID: "pve1-pmxlab", Name: "pmxlab", Status: "online"}},
		Containers: []models.Container{
			{ID: "pve1-pmxlab-110", VMID: 110, Name: "pihole", Node: "pmxlab", Instance: "pve1", Status: "running", Type: "lxc"},
			{ID: "pve1-pmxlab-111", VMID: 111, Name: "nextcloud", Node: "pmxlab", Instance: "pve1", Status: "running", Type: "lxc"},
			{ID: "pve1-pmxlab-112", VMID: 112, Name: "gitea", Node: "pmxlab", Instance: "pve1", Status: "running", Type: "lxc"},
		},
	})
	adapter := newPatrolFindingCreatorAdapterState(NewPatrolService(nil, nil), state)
	report := tools.PatrolFindingInput{
		ResourceID: "ct/gitea", ResourceName: "gitea (and nextcloud, pihole)", ResourceType: "system-container",
		Key: "backup-stale", Severity: "warning", Category: "backup", Title: "Guests not backed up since 14 Nov",
		Description: "Last backup 54h ago", Evidence: "triage: last backup 54h ago", Recommendation: "Run the backup job",
	}

	_, _, err := adapter.CreateFinding(report)
	if err == nil {
		t.Fatal("a guessed ID must not be filed")
	}
	message := err.Error()
	for _, want := range []string{`"ct/gitea" is not a Pulse resource ID`, "gitea is ", "nextcloud is ", "pihole is ", "one finding per resource"} {
		if !strings.Contains(message, want) {
			t.Fatalf("refusal %q lacks %q", message, want)
		}
	}
	if strings.Contains(message, "outside the requested patrol finding scope") {
		t.Fatalf("an ID that names nothing is not out of scope: %q", message)
	}
	if strings.Contains(message, "pmxlab is ") {
		t.Fatalf("the node is not mentioned in the report: %q", message)
	}

	match := regexp.MustCompile(`gitea is (\S+) \(`).FindStringSubmatch(message)
	if match == nil {
		t.Fatalf("no ID for gitea in %q", message)
	}
	// A scoped run names only the resources in its scope.
	scoped := newPatrolFindingCreatorAdapterState(NewPatrolService(nil, nil), state)
	scoped.setFindingScope([]string{match[1]})
	_, _, err = scoped.CreateFinding(report)
	if err == nil || !strings.Contains(err.Error(), "gitea is "+match[1]) || strings.Contains(err.Error(), "nextcloud is ") {
		t.Fatalf("scoped refusal = %v, want only gitea", err)
	}

	report.ResourceID, report.ResourceName = match[1], "gitea"
	if id, _, err := adapter.CreateFinding(report); err != nil || id == "" {
		t.Fatalf("retry with the named ID = (%q, %v), want a finding", id, err)
	}
}

// Patrol also filed "gitea" as resource_id, with resource_name "gitea,
// nextcloud, pihole (pmxlab)". The name admitted it, and the finding was
// stored under "gitea", so a later report of the same guest by its ID would
// be a second finding. A resource's exact name as resource_id is filed under
// that resource's ID; an ambiguous name is left as it was.
func TestPatrolFindingNameAsIDIsFiledUnderTheResourcesID(t *testing.T) {
	state := newPatrolRuntimeState(models.StateSnapshot{
		Containers: []models.Container{
			{ID: "pve1-pmxlab-112", VMID: 112, Name: "gitea", Node: "pmxlab", Instance: "pve1", Status: "running", Type: "lxc"},
			{ID: "pve1-pmxlab-113", VMID: 113, Name: "twin", Node: "pmxlab", Instance: "pve1", Status: "running", Type: "lxc"},
			{ID: "pve1-pmxlab-114", VMID: 114, Name: "twin", Node: "pmxlab", Instance: "pve1", Status: "running", Type: "lxc"},
		},
	})
	ps := NewPatrolService(nil, nil)
	adapter := newPatrolFindingCreatorAdapterState(ps, state)
	report := func(id, name string) tools.PatrolFindingInput {
		return tools.PatrolFindingInput{
			ResourceID: id, ResourceName: name, ResourceType: "system-container",
			Key: "backup-stale-" + id, Severity: "warning", Category: "backup", Title: "Not backed up for 54h",
			Description: "Last backup 54h ago", Evidence: "triage: last backup 54h ago", Recommendation: "Run the backup job",
		}
	}

	id, _, err := adapter.CreateFinding(report("gitea", "gitea, nextcloud, pihole (pmxlab)"))
	if err != nil {
		t.Fatal(err)
	}
	if got := ps.findings.Get(id); got == nil || got.ResourceID != "pve1-pmxlab-112" {
		t.Fatalf("finding filed under %+v, want pve1-pmxlab-112", got)
	}

	// Findings already filed under a name keep that lineage.
	legacyStore := NewPatrolService(nil, nil)
	legacyStore.findings.Add(&Finding{ID: "legacy-gitea", Key: "old", Severity: FindingSeverityWarning, Category: FindingCategoryBackup,
		ResourceID: "gitea", ResourceName: "gitea", ResourceType: "system-container", Title: "old", Description: "d", Evidence: "e"})
	if !legacyStore.findings.Snooze("legacy-gitea", time.Hour) || len(legacyStore.findings.GetByResource("gitea")) != 0 {
		t.Fatal("the legacy finding should be snoozed, so no longer active")
	}
	legacy := newPatrolFindingCreatorAdapterState(legacyStore, state)
	if got, ok := legacy.canonicalizeNameAsFindingID(report("gitea", "gitea")); ok {
		t.Fatalf("a name with a snoozed finding must keep it: %+v", got)
	}
	if _, ok := adapter.canonicalizeNameAsFindingID(report("gitea", "gitea")); !ok {
		t.Fatal("the same report without stored findings is filed under the ID")
	}

	id, _, err = adapter.CreateFinding(report("twin", "twin"))
	if err != nil {
		t.Fatal(err)
	}
	if got := ps.findings.Get(id); got == nil || got.ResourceID != "twin" {
		t.Fatalf("an ambiguous name must not pick one resource: %+v", got)
	}
}

// Patrol often files a host finding as "node/pmxlab". That ID names nothing,
// but the name admitted it, and only registry-shaped IDs were repaired, so it
// was stored under "node/pmxlab" apart from findings filed under the node's
// ID. An unknown ID with a unique exact name is filed under that name's ID.
func TestPatrolFindingGuessedNodeIDIsFiledUnderTheNodesID(t *testing.T) {
	state := newPatrolRuntimeState(models.StateSnapshot{
		Nodes: []models.Node{{ID: "pve1-pmxlab", Name: "pmxlab", Instance: "pve1", Status: "online"}},
	})
	ps := NewPatrolService(nil, nil)
	adapter := newPatrolFindingCreatorAdapterState(ps, state)
	id, _, err := adapter.CreateFinding(tools.PatrolFindingInput{
		ResourceID: "node/pmxlab", ResourceName: "pmxlab", ResourceType: "node",
		Key: "backup-schedule", Severity: "warning", Category: "backup", Title: "Backup schedule slower than 48h",
		Description: "d", Evidence: "e", Recommendation: "r",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := ps.findings.Get(id); got == nil || got.ResourceID != "pve1-pmxlab" {
		t.Fatalf("finding filed under %+v, want pve1-pmxlab", got)
	}
}
