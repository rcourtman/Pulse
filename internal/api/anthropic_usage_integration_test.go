package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Issue #2350's streaming path must survive the actual Anthropic SSE decoder,
// AgenticLoop, API adapter, Patrol cost recorder and disk reload, not just an
// injected DoneEvent or a mock ChatServiceProvider result.
func TestAnthropicUsageSSEToPersistedBudget(t *testing.T) {
	for _, tc := range []struct {
		name              string
		input, output     int
		creation, read    int
		repeatPromptUsage bool
		usd               float64
		blocked           bool
	}{
		{"mixed_repeated_cumulative", 100, 20, 1000, 5000, true, 0.0039, true},
		{"mixed_start_only", 100, 20, 1000, 5000, false, 0.0039, true},
		{"cache_write_only", 0, 0, 1000, 0, false, 0.0025, true},
		{"cache_read_only", 0, 0, 0, 5000, false, 0.001, true},
		{"uncached", 100, 20, 0, 0, false, 0.0004, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				var request struct {
					Model  string `json:"model"`
					Stream bool   `json:"stream"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil ||
					r.Method != http.MethodPost || request.Model != "claude-sonnet-5" || !request.Stream {
					t.Errorf("unexpected Anthropic stream request: %+v, %v", request, err)
					http.Error(w, "invalid fixture request", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				emit := func(event any) {
					data, err := json.Marshal(event)
					if err != nil {
						t.Errorf("encode fixture: %v", err)
						return
					}
					if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
						t.Errorf("write fixture: %v", err)
					}
					w.(http.Flusher).Flush()
				}
				emit(map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]int{
					"input_tokens": tc.input, "output_tokens": 0,
					"cache_creation_input_tokens": tc.creation, "cache_read_input_tokens": tc.read,
				}}})
				emit(map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]string{"type": "text"}})
				emit(map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]string{
					"type": "text_delta", "text": "Fixture response.",
				}})
				emit(map[string]any{"type": "content_block_stop", "index": 0})
				usage := map[string]int{"output_tokens": tc.output}
				if tc.repeatPromptUsage {
					usage["input_tokens"] = tc.input
					usage["cache_creation_input_tokens"] = tc.creation
					usage["cache_read_input_tokens"] = tc.read
				}
				emit(map[string]any{"type": "message_delta", "delta": map[string]string{"stop_reason": "end_turn"}, "usage": usage})
				emit(map[string]string{"type": "message_stop"})
			}))
			defer server.Close()

			persistence := config.NewConfigPersistence(t.TempDir())
			cfg := &config.AIConfig{
				Enabled: true, AnthropicAPIKey: "fixture-key", ChatModel: "anthropic:claude-sonnet-5",
				Model: "anthropic:claude-sonnet-5", PatrolModel: "anthropic:claude-sonnet-5", CostBudgetUSD30d: 0.0008,
			}
			if err := persistence.SaveAIConfig(*cfg); err != nil {
				t.Fatal(err)
			}
			owner := ai.NewService(persistence, nil)
			t.Cleanup(owner.Stop)
			if err := owner.LoadConfig(); err != nil {
				t.Fatal(err)
			}
			svc := chat.NewService(chat.Config{AIConfig: cfg, DataDir: t.TempDir(), CostStore: owner.CostStore()})
			if err := svc.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = svc.Stop(context.Background()) })
			owner.SetChatService(&chatServiceAdapter{svc: svc})
			// Change only the HTTP endpoint through the existing Patrol factory
			// seam. Decoder, loop, adapter and ledger are production code.
			svc.SetPatrolProviderFactory(func(model string) (providers.StreamingProvider, error) {
				if model != cfg.PatrolModel {
					return nil, fmt.Errorf("unexpected route %q", model)
				}
				return providers.NewAnthropicClientWithBaseURL("fixture-key", "claude-sonnet-5", server.URL, 2*time.Second), nil
			})
			svc.SetBudgetChecker(func() error { return owner.CheckBudget("chat") })
			owner.SetStateProvider(&scopedPatrolStateProvider{state: models.StateSnapshot{
				Nodes: []models.Node{{ID: "fixture-node", Name: "fixture", Status: "online"}},
			}})
			patrol := owner.GetPatrolService()
			patrol.SetConfig(ai.PatrolConfig{Enabled: true, AnalyzeNodes: true})
			patrol.TriggerScopedPatrol(context.Background(), ai.PatrolScope{
				ResourceIDs: []string{"fixture-node"}, Reason: ai.TriggerReasonManual,
				Depth: ai.PatrolDepthQuick, NoStream: true,
			})
			runs := patrol.GetRunHistory(1)
			if len(runs) != 1 || runs[0].ErrorCount != 0 || runs[0].InputTokens != tc.input || runs[0].OutputTokens != tc.output ||
				runs[0].CacheCreationInputTokens != tc.creation || runs[0].CacheReadInputTokens != tc.read || calls.Load() != 1 {
				t.Fatalf("stream/adapter/Patrol accounting changed: runs=%+v HTTP calls=%d", runs, calls.Load())
			}
			if err := owner.CostStore().Flush(); err != nil {
				t.Fatal(err)
			}
			owner.Stop()
			restarted := ai.NewService(config.NewConfigPersistence(persistence.DataDir()), nil)
			t.Cleanup(restarted.Stop)
			if err := restarted.LoadConfig(); err != nil {
				t.Fatal(err)
			}
			events := restarted.ListCostEvents(30)
			if len(events) != 1 || events[0].Provider != "anthropic" || events[0].UseCase != "patrol" ||
				events[0].InputTokens != tc.input || events[0].OutputTokens != tc.output ||
				events[0].CacheCreationInputTokens != tc.creation || events[0].CacheReadInputTokens != tc.read {
				t.Fatalf("persisted stream ledger lost attribution or usage: %+v", events)
			}
			totals := restarted.GetCostSummary(30).Totals
			if !totals.PricingKnown || math.Abs(totals.EstimatedUSD-tc.usd) > 1e-12 ||
				totals.TotalTokens != int64(tc.input+tc.output) {
				t.Fatalf("persisted stream totals/rates changed: %+v, want USD %.7f", totals, tc.usd)
			}
			err := restarted.CheckBudget("patrol")
			if tc.blocked && !errors.Is(err, ai.ErrCostBudgetExceeded) || !tc.blocked && err != nil {
				t.Fatalf("reloaded stream spend gave wrong budget decision: %v", err)
			}
			t.Logf("SSE -> AgenticLoop -> API adapter -> Patrol -> persisted ledger: input=%d output=%d creation=%d read=%d Total=%d USD=%.7f; reloaded budget blocked=%t HTTP calls=%d",
				tc.input, tc.output, tc.creation, tc.read, totals.TotalTokens, totals.EstimatedUSD, tc.blocked, calls.Load())
		})
	}
}
