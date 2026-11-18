package ai

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// The triage tables named resources without IDs, so a model filing a finding
// guessed one: "ct/gitea", "system-container/gitea", "node/pmxlab". The flag
// table and the inventory tables now carry each resource's canonical ID, and
// that ID is the one the report tool files under.
func TestPatrolTriageTablesCarryTheIDAFindingIsFiledUnder(t *testing.T) {
	now := time.Now().UTC()
	stale := now.Add(-54 * time.Hour)
	snapshot := models.StateSnapshot{
		Nodes: []models.Node{{ID: "pve1-pmxlab", Name: "pmxlab", Instance: "pve1", Status: "online", Type: "node", LastSeen: now}},
		Containers: []models.Container{
			{ID: "pve1-pmxlab-111", VMID: 111, Name: "nextcloud", Node: "pmxlab", Instance: "pve1", Status: "running", Type: "lxc", LastSeen: now, LastBackup: stale},
			{ID: "pve1-pmxlab-112", VMID: 112, Name: "gitea", Node: "pmxlab", Instance: "pve1", Status: "running", Type: "lxc", LastSeen: now, LastBackup: stale},
		},
		Storage: []models.Storage{{ID: "pve1-pmxlab-local", Name: "local", Node: "pmxlab", Instance: "pve1", Type: "dir", Status: "available",
			Content: "backup", Enabled: true, Active: true, Total: 1000, Used: 500, LastSeen: now}},
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	state := newPatrolRuntimeStateWithProviders(snapshot, registry, unifiedresources.NewUnifiedAIAdapter(registry))

	var giteaID, nodeID string
	for _, r := range registry.List() {
		switch {
		case r.Type == unifiedresources.ResourceTypeSystemContainer && r.Name == "gitea":
			giteaID = r.ID
		case r.Type == unifiedresources.ResourceTypeAgent && r.Name == "pmxlab":
			nodeID = r.ID
		}
	}
	if giteaID == "" || nodeID == "" {
		t.Fatalf("registry lacks gitea or pmxlab: gitea=%q node=%q", giteaID, nodeID)
	}

	ps := NewPatrolService(nil, nil)
	triage := ps.runDeterministicTriageState(context.Background(), state, nil, nil)
	flags := formatTriageFlagsSection(triage)
	row := regexp.MustCompile(`(?m)^\| gitea \| (\S+) \| system-container \| [^|]*Backup`).FindStringSubmatch(flags)
	if row == nil {
		t.Fatalf("no gitea backup flag row with an ID:\n%s", flags)
	}
	if row[1] != giteaID {
		t.Fatalf("flag row ID = %q, want gitea's canonical ID %q", row[1], giteaID)
	}
	if summary := formatTriageFlagsSummary(triage); !strings.Contains(summary, "gitea ["+giteaID+"]") {
		t.Fatalf("the short form of the flags drops the ID:\n%s", summary)
	}

	cfg := DefaultPatrolConfig()
	inventory := ps.seedResourceInventoryState(state, nil, cfg, now, false, nil)
	for _, want := range []string{"| gitea | " + giteaID + " |", "| pmxlab | " + nodeID + " |"} {
		if !strings.Contains(inventory, want) {
			t.Fatalf("inventory lacks %q:\n%s", want, inventory)
		}
	}

	// The ID in the table is the one a finding is filed under, as given.
	adapter := newPatrolFindingCreatorAdapterState(ps, state)
	id, _, err := adapter.CreateFinding(tools.PatrolFindingInput{
		ResourceID: row[1], ResourceName: "gitea", ResourceType: "system-container",
		Key: "backup-stale", Severity: "warning", Category: "backup", Title: "gitea not backed up for 54h",
		Description: "d", Evidence: "e", Recommendation: "r",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := ps.findings.Get(id); got == nil || got.ResourceID != giteaID {
		t.Fatalf("finding filed under %+v, want %s", got, giteaID)
	}

	// A flag whose reference names no resource shows no ID rather than a guess.
	unknown := formatTriageFlagsSection(&TriageResult{Flags: []TriageFlag{{ResourceID: "pve9", ResourceName: "pve9", ResourceType: "node",
		Category: "connectivity", Severity: "critical", Reason: "Instance disconnected"}}})
	if !strings.Contains(unknown, "| pve9 | — | node |") {
		t.Fatalf("an unresolved flag must show no ID:\n%s", unknown)
	}
}
