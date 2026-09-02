package ai

import (
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestFindingsStoreAdd_ReRaiseRefreshesFailureCause(t *testing.T) {
	store := NewFindingsStore()
	now := time.Now()

	first := newPatrolRuntimeFailureFinding(patrolRuntimeFailureFromError(errors.New("failed to connect: connection refused")), now)
	if !store.Add(first) {
		t.Fatal("expected first runtime finding to be added")
	}
	if got := store.Get(first.ID); got == nil || got.FailureCause != string(PatrolFailureCauseProviderConnection) {
		t.Fatalf("expected provider_connection cause on first raise, got %+v", got)
	}

	budget := newPatrolRuntimeFailureFinding(patrolRuntimeFailureFromError(&CostBudgetExceededError{SpentUSD: 21, BudgetUSD: 20, Days: 30}), now.Add(time.Minute))
	if store.Add(budget) {
		t.Fatal("expected the re-raised runtime finding to merge into the existing record")
	}
	got := store.Get(first.ID)
	if got == nil {
		t.Fatal("runtime finding missing after re-raise")
	}
	if got.FailureCause != string(PatrolFailureCauseBudgetExhausted) {
		t.Fatalf("re-raise must carry the new cause, got %q", got.FailureCause)
	}
	if got.Title != budget.Title {
		t.Fatalf("re-raise should refresh the title, got %q", got.Title)
	}
}

func TestResolvePatrolRuntimeFailureFinding_PreflightSuccessKeepsBudgetPause(t *testing.T) {
	p := &PatrolService{findings: NewFindingsStore()}
	budget := newPatrolRuntimeFailureFinding(patrolRuntimeFailureFromError(&CostBudgetExceededError{SpentUSD: 21, BudgetUSD: 20, Days: 30}), time.Now())
	if !p.findings.Add(budget) {
		t.Fatal("expected budget finding to be added")
	}
	if p.resolvePatrolRuntimeFailureFinding(patrolRuntimeResolveReasonPreflight) {
		t.Fatal("a provider preflight must not clear a budget pause")
	}
	if got := p.findings.Get(budget.ID); got == nil || got.IsResolved() {
		t.Fatal("budget finding should stay open after preflight success")
	}
	if !p.resolvePatrolRuntimeFailureFinding("full_patrol_success") {
		t.Fatal("a run that got past the budget check resolves the finding")
	}

	provider := &PatrolService{findings: NewFindingsStore()}
	fault := newPatrolRuntimeFailureFinding(patrolRuntimeFailureFromError(errors.New("failed to connect: connection refused")), time.Now())
	provider.findings.Add(fault)
	if !provider.resolvePatrolRuntimeFailureFinding(patrolRuntimeResolveReasonPreflight) {
		t.Fatal("a provider fault is still cleared by a successful preflight")
	}
}

func TestFindingsPersistence_RoundTripsFailureCause(t *testing.T) {
	adapter := NewFindingsPersistenceAdapter(config.NewConfigPersistence(t.TempDir()))
	budget := newPatrolRuntimeFailureFinding(patrolRuntimeFailureFromError(&CostBudgetExceededError{SpentUSD: 21, BudgetUSD: 20, Days: 30}), time.Now())
	if err := adapter.SaveFindings(map[string]*Finding{budget.ID: budget}); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := adapter.LoadFindings()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got := loaded[budget.ID]
	if got == nil || got.FailureCause != string(PatrolFailureCauseBudgetExhausted) {
		t.Fatalf("failure cause must survive a restart, got %+v", got)
	}
}
