package ai

import (
	"regexp"
	"strings"
	"testing"

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
