package ai

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestRunScopedPatrol_Disabled(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.config = PatrolConfig{Enabled: false}
	ps.runScopedPatrol(context.Background(), PatrolScope{ResourceIDs: []string{"node-1"}})
	// No panic and no state changes expected.
}

func TestRunScopedPatrol_RequeueWhenRunInProgress(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.config = PatrolConfig{Enabled: true}
	ps.runInProgress = true

	scope := PatrolScope{ResourceIDs: []string{"node-1"}, Reason: TriggerReasonManual}
	ps.runScopedPatrol(context.Background(), scope)

	if !ps.runInProgress {
		t.Fatalf("expected runInProgress to remain true when run is already in progress")
	}
}

func TestRunScopedPatrol_StuckRunClearsAndNoStateProvider(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.config = PatrolConfig{Enabled: true}
	ps.runInProgress = true
	ps.runStartedAt = time.Now().Add(-21 * time.Minute)

	ps.runScopedPatrol(context.Background(), PatrolScope{ResourceIDs: []string{"node-1"}, Reason: TriggerReasonManual})

	if ps.runInProgress {
		t.Fatalf("expected stuck run to be cleared")
	}
}

func TestRunScopedPatrol_StoresEffectiveScopeResourceIDs(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "mock:model"}
	svc.provider = &mockProvider{}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	svc.SetChatService(&mockChatService{
		executor: executor,
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			return &PatrolStreamResponse{Content: "no issues"}, nil
		},
	})

	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					ID:       "docker-host-1",
					Hostname: "docker-1",
					Containers: []models.DockerContainer{
						{ID: "container-1", Name: "web"},
						{ID: "container-2", Name: "db"},
					},
				},
			},
		},
	}

	ps := NewPatrolService(svc, stateProvider)
	ps.SetConfig(PatrolConfig{
		Enabled:       true,
		Interval:      10 * time.Minute,
		AnalyzeDocker: true,
	})

	ps.runScopedPatrol(context.Background(), PatrolScope{
		ResourceIDs:   []string{"web"},
		ResourceTypes: []string{"app-container"},
		Reason:        TriggerReasonManual,
		NoStream:      true,
	})

	runs := ps.runHistoryStore.GetRecent(1)
	if len(runs) != 1 {
		t.Fatalf("expected 1 scoped patrol run, got %d", len(runs))
	}

	run := runs[0]
	if len(run.ScopeResourceIDs) != 1 || run.ScopeResourceIDs[0] != "web" {
		t.Fatalf("expected requested scope IDs [web], got %v", run.ScopeResourceIDs)
	}
	if len(run.EffectiveScopeResourceIDs) == 0 {
		t.Fatalf("expected non-empty effective scope resource IDs")
	}
	if !containsScopeID(run.EffectiveScopeResourceIDs, "container-1") {
		t.Fatalf("expected effective scope IDs to include matched container, got %v", run.EffectiveScopeResourceIDs)
	}
	if !containsScopeID(run.EffectiveScopeResourceIDs, "docker-host-1") {
		t.Fatalf("expected effective scope IDs to include docker host, got %v", run.EffectiveScopeResourceIDs)
	}
	if containsScopeID(run.EffectiveScopeResourceIDs, "container-2") {
		t.Fatalf("expected effective scope IDs to exclude unrelated container, got %v", run.EffectiveScopeResourceIDs)
	}
}

func containsScopeID(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestPatrolLogResourceIDs_TruncatesLongLists(t *testing.T) {
	ids := make([]string, scopedPatrolLogIDLimit+2)
	for i := range ids {
		ids[i] = "id-" + string(rune('a'+i))
	}

	logIDs := patrolLogResourceIDs(ids)
	if len(logIDs) != scopedPatrolLogIDLimit+1 {
		t.Fatalf("expected %d logged IDs including truncation marker, got %d", scopedPatrolLogIDLimit+1, len(logIDs))
	}
	if logIDs[scopedPatrolLogIDLimit] != "... +2 more" {
		t.Fatalf("expected truncation marker, got %q", logIDs[scopedPatrolLogIDLimit])
	}
	if logIDs[0] != "id-a" || logIDs[scopedPatrolLogIDLimit-1] != "id-j" {
		t.Fatalf("expected first %d IDs to be preserved, got %v", scopedPatrolLogIDLimit, logIDs)
	}
}
