package ai

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Issue #2350: cross the real HTTP decoder, execution loop, cost ledger,
// on-disk persistence and budget gate in one test. No response or usage event
// is injected after the provider boundary. Dollar expectations are independent
// of the calculator: Sonnet 5 input/output/write/read = $2/$10/$2.50/$0.20 per
// million tokens (the existing five-minute cache-write rate, not one hour).
func TestAnthropicUsageHTTPToPersistedBudget(t *testing.T) {
	for _, tc := range []struct {
		name           string
		input, output  int
		creation, read int
		usd            float64
		blocked        bool
	}{
		{"mixed", 100, 20, 1000, 5000, 0.0039, true},
		{"cache_write_only", 0, 0, 1000, 0, 0.0025, true},
		{"cache_read_only", 0, 0, 0, 5000, 0.001, true},
		{"uncached", 100, 20, 0, 0, 0.0004, false},
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
					r.Method != http.MethodPost || request.Model != "claude-sonnet-5" || request.Stream {
					t.Errorf("unexpected Anthropic request: %+v, %v", request, err)
					http.Error(w, "invalid fixture request", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(map[string]any{
					"id": "msg_fixture", "type": "message", "role": "assistant",
					"model": "claude-sonnet-5", "stop_reason": "end_turn",
					"content": []map[string]string{{"type": "text", "text": "Fixture response."}},
					"usage": map[string]int{
						"input_tokens": tc.input, "output_tokens": tc.output,
						"cache_creation_input_tokens": tc.creation, "cache_read_input_tokens": tc.read,
					},
				}); err != nil {
					t.Errorf("write fixture: %v", err)
				}
			}))
			defer server.Close()

			cfg := &config.AIConfig{
				Enabled: true, ChatModel: "anthropic:claude-sonnet-5", CostBudgetUSD30d: 0.0008,
			}
			persistence := config.NewConfigPersistence(t.TempDir())
			client := providers.NewAnthropicClientWithBaseURL("fixture-key", "claude-sonnet-5", server.URL, 2*time.Second)
			svc := NewService(nil, nil)
			svc.cfg, svc.provider = cfg, client
			// Leave the configured key empty so model construction falls back
			// to this real, loopback-bound client, never a public provider.
			if err := svc.costStore.SetPersistence(NewCostPersistenceAdapter(persistence)); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(svc.Stop)
			req := ExecuteRequest{Prompt: "Return the fixture response.", UseCase: "chat"}
			resp, err := svc.Execute(context.Background(), req)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if resp.InputTokens != tc.input || resp.OutputTokens != tc.output ||
				resp.CacheCreationInputTokens != tc.creation || resp.CacheReadInputTokens != tc.read {
				t.Fatalf("execution lost or merged usage: %+v", resp)
			}
			if err := svc.costStore.Flush(); err != nil {
				t.Fatal(err)
			}

			// Reopen through a new persistence adapter and service, rather than
			// reading back the original store's in-memory events.
			restarted := NewService(nil, nil)
			restarted.cfg, restarted.provider = cfg, client
			t.Cleanup(restarted.Stop)
			if err := restarted.costStore.SetPersistence(NewCostPersistenceAdapter(config.NewConfigPersistence(persistence.DataDir()))); err != nil {
				t.Fatal(err)
			}
			events := restarted.ListCostEvents(30)
			if len(events) != 1 || events[0].Provider != "anthropic" || events[0].UseCase != "chat" ||
				events[0].InputTokens != tc.input || events[0].OutputTokens != tc.output ||
				events[0].CacheCreationInputTokens != tc.creation || events[0].CacheReadInputTokens != tc.read {
				t.Fatalf("persisted ledger lost attribution or usage: %+v", events)
			}
			totals := restarted.GetCostSummary(30).Totals
			if !totals.PricingKnown || math.Abs(totals.EstimatedUSD-tc.usd) > 1e-12 ||
				totals.InputTokens != int64(tc.input) || totals.OutputTokens != int64(tc.output) ||
				totals.CacheCreationInputTokens != int64(tc.creation) || totals.CacheReadInputTokens != int64(tc.read) ||
				totals.TotalTokens != int64(tc.input+tc.output) {
				t.Fatalf("persisted totals/rates changed: %+v, want USD %.7f", totals, tc.usd)
			}

			_, err = restarted.Execute(context.Background(), req)
			if tc.blocked {
				if !errors.Is(err, ErrCostBudgetExceeded) || calls.Load() != 1 || len(restarted.ListCostEvents(30)) != 1 {
					t.Fatalf("persisted cache spend did not stop the next HTTP call: err=%v calls=%d", err, calls.Load())
				}
			} else if err != nil || calls.Load() != 2 || len(restarted.ListCostEvents(30)) != 2 {
				t.Fatalf("uncached control was wrongly blocked: err=%v calls=%d", err, calls.Load())
			}
			t.Logf("HTTP -> Execute -> persisted ledger: input=%d output=%d creation=%d read=%d Total=%d USD=%.7f; next request blocked=%t HTTP calls=%d",
				tc.input, tc.output, tc.creation, tc.read, totals.TotalTokens, totals.EstimatedUSD, tc.blocked, calls.Load())
		})
	}
}
