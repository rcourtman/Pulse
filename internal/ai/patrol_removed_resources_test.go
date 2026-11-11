package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// A finding on a container that has been removed is closed between runs. A
// run closes it when it starts, and with the default schedule the next run can
// be six hours away. Until then the finding was shown as a current problem on
// a container that no longer existed, also while a new container had its name.
func TestFindingOnARemovedContainerIsClosedWithoutARun(t *testing.T) {
	rig := newReplacedResourceRig(t)
	var resolvedInUnifiedStore []string
	rig.ps.SetUnifiedFindingResolver(func(findingID string) { resolvedInUnifiedStore = append(resolvedInUnifiedStore, findingID) })

	oldContainer := strings.Repeat("a1", 32)
	newContainer := strings.Repeat("b2", 32)
	exited := func(id string) models.DockerContainer {
		return models.DockerContainer{ID: id, Name: replacedTestContainerName, State: "exited", Status: "Exited (139) 2 minutes ago", ExitCode: 139}
	}
	running := func(id string) models.DockerContainer {
		return models.DockerContainer{ID: id, Name: replacedTestContainerName, State: "running", Status: "Up 1 minute"}
	}
	other := models.DockerContainer{ID: strings.Repeat("c3", 32), Name: "nextcloud", State: "running", Status: "Up 3 days"}

	// A real alert run files a finding on the crashed container.
	oldID := rig.setContainers(exited(oldContainer), other)[replacedTestContainerName]
	rig.report = rig.crashReport(oldID)
	rig.alertRun(oldContainer)
	open := rig.activeFindings()
	if len(open) != 1 || open[0].ResourceID != oldID {
		t.Fatalf("setup: want one open finding on the crashed container, got %#v", open)
	}
	finding := open[0]
	runs := rig.ps.runHistoryStore.Count()
	rig.report = func(tools.PatrolFindingCreator) error {
		t.Error("the sweep asked the model")
		return nil
	}

	start := time.Now()
	missingSince := make(map[string]time.Time)
	sweep := func(after time.Duration) int {
		return rig.ps.sweepFindingsForRemovedResources(start.Add(after), missingSince)
	}
	stillOpen := func(when string) {
		t.Helper()
		if stored := rig.ps.findings.Get(finding.ID); stored == nil || !stored.IsActive() {
			t.Fatalf("%s: the finding was closed: %#v", when, stored)
		}
	}

	// While the container is there, nothing happens however long it takes.
	if closed := sweep(0); closed != 0 {
		t.Fatalf("the container is in the inventory and the sweep closed %d findings", closed)
	}
	if closed := sweep(time.Hour); closed != 0 {
		t.Fatalf("the container is in the inventory and the sweep closed %d findings", closed)
	}
	stillOpen("container present")

	// The container is recreated: same name, new ID. One look is not enough.
	newID := rig.setContainers(running(newContainer), other)[replacedTestContainerName]
	if newID == oldID {
		t.Fatal("setup: the recreated container kept its inventory ID")
	}
	base := 2 * time.Hour
	sweep(base)
	sweep(base + time.Minute)
	sweep(base + removedResourceGrace - time.Second)
	stillOpen("inside the grace period")

	// It comes back (the inventory was only refilling). The count starts over.
	rig.setContainers(exited(oldContainer), other)
	sweep(base + removedResourceGrace)
	stillOpen("resource back in the inventory")
	rig.setContainers(running(newContainer), other)
	sweep(base + removedResourceGrace + time.Minute)
	sweep(base + 2*removedResourceGrace)
	stillOpen("missing again, grace started over")

	// An inventory with nothing in it, as after a restart before anything
	// has reported, says nothing about one container.
	empty := unifiedresources.NewRegistry(nil)
	rig.provider.state = models.StateSnapshot{}
	rig.ps.SetReadState(empty)
	rig.ps.SetUnifiedResourceProvider(unifiedresources.NewUnifiedAIAdapter(empty))
	sweep(base + 3*removedResourceGrace)
	sweep(base + 5*removedResourceGrace)
	stillOpen("empty inventory")
	if len(missingSince) != 0 {
		t.Errorf("an empty inventory counted towards the grace period: %v", missingSince)
	}

	// Missing on every look for the whole grace period: closed, as a run
	// would close it, with no run and no model call.
	rig.setContainers(running(newContainer), other)
	base = 10 * time.Hour
	sweep(base)
	sweep(base + time.Minute)
	sweep(base + 2*time.Minute)
	if closed := sweep(base + removedResourceGrace); closed != 1 {
		t.Fatalf("the container has been gone for the grace period and the sweep closed %d findings, want 1", closed)
	}
	stored := rig.ps.findings.Get(finding.ID)
	if stored == nil || stored.IsActive() || stored.ResolveReason != removedResourceResolveReason || !stored.AutoResolved {
		t.Fatalf("the finding on the removed container is not closed as removed: %#v", stored)
	}
	if len(resolvedInUnifiedStore) != 1 || resolvedInUnifiedStore[0] != finding.ID {
		t.Errorf("the unified store was told to resolve %v, want %s once", resolvedInUnifiedStore, finding.ID)
	}
	if got := rig.ps.runHistoryStore.Count(); got != runs {
		t.Errorf("the sweep left %d run records, want none", got-runs)
	}
	if len(missingSince) != 0 {
		t.Errorf("the sweep still remembers %v after closing", missingSince)
	}
	if closed := sweep(base + 2*removedResourceGrace); closed != 0 {
		t.Errorf("a second sweep closed %d more findings", closed)
	}

	// A finding on the new container is its own and stays open.
	rig.report = rig.crashReport(newID)
	rig.setContainers(exited(newContainer), other)
	rig.alertRun(newContainer)
	rig.report = nil
	base = 20 * time.Hour
	sweep(base)
	sweep(base + 2*removedResourceGrace)
	open = rig.activeFindings()
	if len(open) != 1 || open[0].ResourceID != newID {
		t.Fatalf("want one open finding on the new container, got %#v", open)
	}
}

// A finding whose resource ID is not an inventory ID is tied to its resource
// by name, as in a run. It stays open while something has that name.
func TestFindingKeyedByNameIsNotClosedWhileTheNameIsInUse(t *testing.T) {
	rig := newReplacedResourceRig(t)
	rig.setContainers(models.DockerContainer{ID: strings.Repeat("a1", 32), Name: replacedTestContainerName, State: "running", Status: "Up 1 minute"})
	rig.ps.findings.Add(&Finding{
		ID: "by-name", Key: "disk-growth", Severity: FindingSeverityWarning, Category: FindingCategoryCapacity,
		ResourceID: replacedTestContainerName, ResourceName: replacedTestContainerName, ResourceType: "app-container",
		Title: "Volume is growing", Description: "The volume grows by 2 GB a day.",
	})
	start := time.Now()
	missingSince := make(map[string]time.Time)
	rig.ps.sweepFindingsForRemovedResources(start, missingSince)
	if closed := rig.ps.sweepFindingsForRemovedResources(start.Add(2*removedResourceGrace), missingSince); closed != 0 {
		t.Fatalf("the name is in use and the sweep closed %d findings", closed)
	}

	// Nothing has the name any more.
	rig.setContainers(models.DockerContainer{ID: strings.Repeat("c3", 32), Name: "nextcloud", State: "running", Status: "Up 3 days"})
	rig.ps.sweepFindingsForRemovedResources(start.Add(time.Hour), missingSince)
	if closed := rig.ps.sweepFindingsForRemovedResources(start.Add(time.Hour+removedResourceGrace), missingSince); closed != 1 {
		t.Fatalf("nothing has the name and the sweep closed %d findings, want 1", closed)
	}
}
