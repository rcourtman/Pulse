package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleExportAICostHistory_JSON(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	events := []config.AIUsageEventRecord{
		{
			Timestamp:    time.Now().UTC(),
			Provider:     "openai",
			RequestModel: "gpt-4o-mini",
			UseCase:      "chat",
			InputTokens:  42,
			OutputTokens: 17,
			TargetType:   "vm",
			TargetID:     "vm-1",
			FindingID:    "finding-1",
		},
	}
	if err := persistence.SaveAIUsageHistory(events); err != nil {
		t.Fatalf("SaveAIUsageHistory: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/cost/export?days=7&format=json", nil)
	rec := httptest.NewRecorder()

	handler.HandleExportAICostHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("expected json content type")
	}

	var resp struct {
		Days   int `json:"days"`
		Events []struct {
			Provider     string `json:"provider"`
			PricingKnown bool   `json:"pricing_known"`
		} `json:"events"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Days != 7 {
		t.Fatalf("days = %d, want 7", resp.Days)
	}
	if len(resp.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(resp.Events))
	}
	if resp.Events[0].Provider != "openai" {
		t.Fatalf("provider = %s, want openai", resp.Events[0].Provider)
	}
	if !resp.Events[0].PricingKnown {
		t.Fatalf("expected pricing known")
	}
}

func TestHandleExportAICostHistory_CSV(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	events := []config.AIUsageEventRecord{
		{
			Timestamp:    time.Now().UTC(),
			Provider:     "openai",
			RequestModel: "gpt-4o-mini",
			UseCase:      "chat",
			InputTokens:  5,
			OutputTokens: 3,
			TargetType:   "node",
			TargetID:     "node-1",
		},
	}
	if err := persistence.SaveAIUsageHistory(events); err != nil {
		t.Fatalf("SaveAIUsageHistory: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/cost/export?days=1&format=csv", nil)
	rec := httptest.NewRecorder()

	handler.HandleExportAICostHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/csv") {
		t.Fatalf("expected csv content type")
	}

	lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and data rows")
	}
	if !strings.HasPrefix(lines[0], "timestamp,provider,request_model") {
		t.Fatalf("unexpected header: %s", lines[0])
	}
}
