package ai

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// oneSlotOrchestrator behaves like the Pro orchestrator on a route that serves
// one investigation at a time: the slot check is advisory and the reservation
// inside InvestigateFinding is the real gate.
type oneSlotOrchestrator struct {
	mu       sync.Mutex
	running  string
	started  chan string
	releases map[string]chan struct{}
	// raceNext makes the next slot check answer "free" while the slot is
	// taken, the way two findings can both pass the advisory check.
	raceNext bool
}

func newOneSlotOrchestrator() *oneSlotOrchestrator {
	return &oneSlotOrchestrator{
		started:  make(chan string, 16),
		releases: make(map[string]chan struct{}),
	}
}

func (o *oneSlotOrchestrator) InvestigateFinding(_ context.Context, f *InvestigationFinding, _ string) error {
	o.mu.Lock()
	if o.running != "" {
		o.mu.Unlock()
		return errors.New("maximum concurrent investigations reached (1)")
	}
	o.running = f.ID
	release := make(chan struct{})
	o.releases[f.ID] = release
	o.mu.Unlock()

	o.started <- f.ID
	<-release

	o.mu.Lock()
	o.running = ""
	o.mu.Unlock()
	return nil
}

func (o *oneSlotOrchestrator) finish(t *testing.T, findingID string) {
	t.Helper()
	o.mu.Lock()
	release, ok := o.releases[findingID]
	delete(o.releases, findingID)
	o.mu.Unlock()
	if !ok {
		t.Fatalf("investigation of %s is not running", findingID)
	}
	close(release)
}

func (o *oneSlotOrchestrator) CanStartInvestigation() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.raceNext {
		o.raceNext = false
		return true
	}
	return o.running == ""
}

func (o *oneSlotOrchestrator) GetRunningCount() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.running == "" {
		return 0
	}
	return 1
}

func (o *oneSlotOrchestrator) GetInvestigationByFinding(string) *InvestigationSession { return nil }
func (o *oneSlotOrchestrator) GetFixedCount() int                                     { return 0 }
func (o *oneSlotOrchestrator) ReinvestigateFinding(context.Context, string, string) error {
	return nil
}
func (o *oneSlotOrchestrator) Shutdown(context.Context) error { return nil }

func (o *oneSlotOrchestrator) expectStart(t *testing.T, findingID string) {
	t.Helper()
	select {
	case got := <-o.started:
		if got != findingID {
			t.Fatalf("expected the investigation of %s to start, got %s", findingID, got)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("expected the investigation of %s to start, none did", findingID)
	}
}

func (o *oneSlotOrchestrator) expectNoStart(t *testing.T) {
	t.Helper()
	select {
	case got := <-o.started:
		t.Fatalf("expected no investigation to start, got %s", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func newInvestigationQueuePatrol(t *testing.T, orch InvestigationOrchestrator) *PatrolService {
	t.Helper()
	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	svc.cfg = &config.AIConfig{
		Enabled:             true,
		PatrolModel:         "mock:model",
		PatrolAutonomyLevel: "approval",
	}
	ps := NewPatrolService(svc, &patrolTestStateProvider{})
	ps.SetInvestigationOrchestrator(orch)
	return ps
}

func queueTestFinding(id string, severity FindingSeverity) *Finding {
	return &Finding{
		ID:           id,
		Key:          id,
		Severity:     severity,
		Category:     FindingCategoryReliability,
		ResourceID:   "resource-" + id,
		ResourceName: id,
		ResourceType: "app-container",
		Title:        "Problem on " + id,
		Source:       "ai-analysis",
	}
}

// A finding raised while the only investigation slot is busy used to be
// dropped until a later Patrol run reported it again. It must be investigated
// as soon as the slot frees, with no run in between.
func TestInvestigationQueue_FindingTurnedAwayIsInvestigatedWhenTheSlotFrees(t *testing.T) {
	orch := newOneSlotOrchestrator()
	ps := newInvestigationQueuePatrol(t, orch)

	ps.recordFinding(queueTestFinding("first", FindingSeverityWarning))
	orch.expectStart(t, "first")

	// Both arrive while "first" holds the slot.
	ps.recordFinding(queueTestFinding("later-warning", FindingSeverityWarning))
	ps.recordFinding(queueTestFinding("later-critical", FindingSeverityCritical))
	orch.expectNoStart(t)

	// The slot frees. The critical finding goes first.
	orch.finish(t, "first")
	orch.expectStart(t, "later-critical")
	orch.expectNoStart(t)

	orch.finish(t, "later-critical")
	orch.expectStart(t, "later-warning")

	orch.finish(t, "later-warning")
	orch.expectNoStart(t)
	ps.investigationWg.Wait()
}

// Waiting does not get around the investigation gate: a finding that is
// resolved while it waits is not investigated.
func TestInvestigationQueue_ResolvedWhileWaitingIsNotInvestigated(t *testing.T) {
	orch := newOneSlotOrchestrator()
	ps := newInvestigationQueuePatrol(t, orch)

	ps.recordFinding(queueTestFinding("first", FindingSeverityWarning))
	orch.expectStart(t, "first")
	ps.recordFinding(queueTestFinding("fixed-meanwhile", FindingSeverityCritical))
	orch.expectNoStart(t)

	if !ps.findings.Resolve("fixed-meanwhile", true) {
		t.Fatal("expected the waiting finding to resolve")
	}

	orch.finish(t, "first")
	orch.expectNoStart(t)
	ps.investigationWg.Wait()
}

// The slot check is advisory. When two findings pass it together the
// orchestrator admits one and refuses the other, which never ran. It waits for
// the slot like any finding turned away.
func TestInvestigationQueue_FindingThatLostTheSlotRaceIsRetried(t *testing.T) {
	orch := newOneSlotOrchestrator()
	ps := newInvestigationQueuePatrol(t, orch)

	ps.recordFinding(queueTestFinding("first", FindingSeverityWarning))
	orch.expectStart(t, "first")

	orch.mu.Lock()
	orch.raceNext = true
	orch.mu.Unlock()
	ps.recordFinding(queueTestFinding("lost-race", FindingSeverityWarning))
	orch.expectNoStart(t)

	orch.finish(t, "first")
	orch.expectStart(t, "lost-race")
	orch.finish(t, "lost-race")
	ps.investigationWg.Wait()
}

// The end of a Patrol run is the backstop when a slot frees without one of
// Patrol's own investigations ending.
func TestInvestigationQueue_RunEndStartsWaitingFindings(t *testing.T) {
	orch := newOneSlotOrchestrator()
	ps := newInvestigationQueuePatrol(t, orch)

	// Something outside Patrol's own dispatch holds the slot.
	orch.mu.Lock()
	orch.running = "manual-run"
	orch.mu.Unlock()
	ps.recordFinding(queueTestFinding("waiting", FindingSeverityWarning))
	orch.expectNoStart(t)

	orch.mu.Lock()
	orch.running = ""
	orch.mu.Unlock()
	ps.dispatchPatrolInvestigations(nil)
	orch.expectStart(t, "waiting")
	orch.finish(t, "waiting")
	ps.investigationWg.Wait()
}

func TestInvestigationQueue_WaitingListIsBounded(t *testing.T) {
	store := NewFindingsStore()
	now := time.Now()
	for i := 0; i < maxDeferredInvestigations+5; i++ {
		f := queueTestFinding("f"+string(rune('a'+i/26))+string(rune('a'+i%26)), FindingSeverityWarning)
		store.Add(f)
		queued := store.DeferInvestigation(f.ID, now)
		if want := i < maxDeferredInvestigations; queued != want {
			t.Fatalf("finding %d: queued=%v, want %v", i, queued, want)
		}
	}
	if store.DeferInvestigation("missing", now) {
		t.Fatal("a finding that does not exist must not wait")
	}
	taken := 0
	for store.TakeNextDeferredInvestigation() != nil {
		taken++
	}
	if taken != maxDeferredInvestigations {
		t.Fatalf("expected %d waiting findings, got %d", maxDeferredInvestigations, taken)
	}
}

func TestInvestigationQueue_NextIsCriticalThenLongestWaiting(t *testing.T) {
	store := NewFindingsStore()
	base := time.Now()
	for _, f := range []*Finding{
		queueTestFinding("warning-old", FindingSeverityWarning),
		queueTestFinding("warning-new", FindingSeverityWarning),
		queueTestFinding("critical-new", FindingSeverityCritical),
		queueTestFinding("critical-old", FindingSeverityCritical),
	} {
		store.Add(f)
	}
	store.DeferInvestigation("warning-old", base)
	store.DeferInvestigation("warning-new", base.Add(2*time.Minute))
	store.DeferInvestigation("critical-new", base.Add(3*time.Minute))
	store.DeferInvestigation("critical-old", base.Add(time.Minute))

	var order []string
	for f := store.TakeNextDeferredInvestigation(); f != nil; f = store.TakeNextDeferredInvestigation() {
		order = append(order, f.ID)
	}
	want := []string{"critical-old", "critical-new", "warning-old", "warning-new"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}
