package ai

import (
	"context"
	"fmt"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Heuristic flags are supplied evidence, not a quota of findings or model runs.
func TestPatrolDetectionKeepsOneConversationAcrossFlagCounts(t *testing.T) {
	var normalLimit int
	for _, count := range []int{0, 1, 15} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
			svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "mock:model"}
			calls := 0
			const conclusion = "The observed symptoms have no established common cause. No action was taken."
			svc.SetChatService(&mockChatService{
				executor: tools.NewPulseToolExecutor(tools.ExecutorConfig{}),
				executePatrolStreamFunc: func(_ context.Context, req PatrolExecuteRequest, _ ChatStreamCallback) (*PatrolStreamResponse, error) {
					calls++
					if req.SessionID != "patrol-main" {
						t.Fatalf("unexpected diagnostic session %q", req.SessionID)
					}
					if normalLimit == 0 {
						normalLimit = req.MaxTurns
					}
					if req.MaxTurns <= 0 || req.MaxTurns != normalLimit {
						t.Fatalf("flags changed execution limit: %d, want %d", req.MaxTurns, normalLimit)
					}
					return &PatrolStreamResponse{Content: conclusion, InputTokens: 7, OutputTokens: 3}, nil
				},
			})
			ps := NewPatrolService(svc, nil)
			state := models.StateSnapshot{DockerHosts: []models.DockerHost{{ID: "host", Hostname: "host", Status: "online"}}}
			for i := 0; i < count; i++ {
				state.DockerHosts[0].Containers = append(state.DockerHosts[0].Containers, models.DockerContainer{
					ID: fmt.Sprintf("container-%d", i), Name: fmt.Sprintf("service-%d", i), State: "running", Health: "unhealthy",
				})
			}
			snap := patrolRuntimeStateForTest(ps, state)
			triage := ps.runDeterministicTriageState(context.Background(), snap, nil, nil)
			if len(triage.Flags) != count {
				t.Fatalf("fixture has %d flags, want %d", len(triage.Flags), count)
			}
			result, err := ps.runAIAnalysisState(context.Background(), snap, nil, "continuity-run")
			if err != nil || result == nil {
				t.Fatalf("analysis failed: result=%+v error=%v", result, err)
			}
			if calls != 1 || result.Response != conclusion || len(result.Findings) != 0 {
				t.Fatalf("flag processing replaced the model's decision: calls=%d result=%+v", calls, result)
			}
			if result.InputTokens != 7 || result.OutputTokens != 3 {
				t.Fatalf("usage includes an unexpected auxiliary session: %+v", result)
			}
		})
	}
}
