package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license/conversion"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/metering"
)

func TestConversionHandleRecordEventValidPOST(t *testing.T) {
	handlers := NewConversionHandlers(nil)

	body := []byte(fmt.Sprintf(`{
		"type":"paywall_viewed",
		"capability":"long_term_metrics",
		"surface":"history_chart",
		"tenant_mode":"single",
		"timestamp":%d,
		"idempotency_key":"paywall_viewed:history_chart:long_term_metrics:1"
	}`, time.Now().UnixMilli()))

	req := httptest.NewRequest(http.MethodPost, "/api/conversion/events", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handlers.HandleRecordEvent(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	var resp map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	if !resp["accepted"] {
		t.Fatalf("accepted = %v, want true", resp["accepted"])
	}
}

func TestConversionHandleRecordEventInvalidBody(t *testing.T) {
	handlers := NewConversionHandlers(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/conversion/events", bytes.NewReader([]byte("{")))
	rec := httptest.NewRecorder()

	handlers.HandleRecordEvent(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	if resp["error"] != "validation_error" {
		t.Fatalf("error = %q, want validation_error", resp["error"])
	}
}

func TestConversionHandleRecordEventMissingRequiredFields(t *testing.T) {
	handlers := NewConversionHandlers(nil)

	body := []byte(fmt.Sprintf(`{
		"type":"paywall_viewed",
		"surface":"history_chart",
		"timestamp":%d,
		"idempotency_key":"paywall_viewed:history_chart::1"
	}`, time.Now().UnixMilli()))

	req := httptest.NewRequest(http.MethodPost, "/api/conversion/events", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handlers.HandleRecordEvent(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	if resp["error"] != "validation_error" {
		t.Fatalf("error = %q, want validation_error", resp["error"])
	}
}

func TestConversionHandleRecordEventNonPOST(t *testing.T) {
	handlers := NewConversionHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/conversion/events", nil)
	rec := httptest.NewRecorder()

	handlers.HandleRecordEvent(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestConversionHandleGetStats(t *testing.T) {
	agg := metering.NewWindowedAggregator()
	recorder := conversion.NewRecorder(agg)
	handlers := NewConversionHandlers(recorder)

	events := []conversion.ConversionEvent{
		{
			Type:           conversion.EventPaywallViewed,
			Capability:     "long_term_metrics",
			Surface:        "history_chart",
			Timestamp:      time.Now().UnixMilli(),
			IdempotencyKey: "paywall_viewed:history_chart:long_term_metrics:1",
		},
		{
			Type:           conversion.EventPaywallViewed,
			Capability:     "long_term_metrics",
			Surface:        "history_chart",
			Timestamp:      time.Now().UnixMilli(),
			IdempotencyKey: "paywall_viewed:history_chart:long_term_metrics:2",
		},
		{
			Type:           conversion.EventTrialStarted,
			Surface:        "license_panel",
			Timestamp:      time.Now().UnixMilli(),
			IdempotencyKey: "trial_started:license_panel::1",
		},
	}
	for _, event := range events {
		if err := recorder.Record(event); err != nil {
			t.Fatalf("record failed: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/conversion/stats", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		WindowStart int64 `json:"window_start"`
		WindowEnd   int64 `json:"window_end"`
		Buckets     []struct {
			Type       string `json:"type"`
			Key        string `json:"key"`
			Count      int64  `json:"count"`
			TotalValue int64  `json:"total_value"`
		} `json:"buckets"`
		TotalEvents int64 `json:"total_events"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}

	if resp.WindowStart <= 0 {
		t.Fatalf("window_start = %d, want > 0", resp.WindowStart)
	}
	if resp.WindowEnd < resp.WindowStart {
		t.Fatalf("window_end = %d, want >= window_start %d", resp.WindowEnd, resp.WindowStart)
	}
	if len(resp.Buckets) != 2 {
		t.Fatalf("len(buckets) = %d, want 2", len(resp.Buckets))
	}
	if resp.TotalEvents != 3 {
		t.Fatalf("total_events = %d, want 3", resp.TotalEvents)
	}

	byKey := make(map[string]struct {
		Type       string
		Count      int64
		TotalValue int64
	}, len(resp.Buckets))
	for _, bucket := range resp.Buckets {
		byKey[bucket.Key] = struct {
			Type       string
			Count      int64
			TotalValue int64
		}{
			Type:       bucket.Type,
			Count:      bucket.Count,
			TotalValue: bucket.TotalValue,
		}
	}

	paywallBucket, ok := byKey["history_chart:long_term_metrics"]
	if !ok {
		t.Fatal("missing history_chart:long_term_metrics bucket")
	}
	if paywallBucket.Type != conversion.EventPaywallViewed {
		t.Fatalf("paywall bucket type = %q, want %q", paywallBucket.Type, conversion.EventPaywallViewed)
	}
	if paywallBucket.Count != 2 {
		t.Fatalf("paywall bucket count = %d, want 2", paywallBucket.Count)
	}
	if paywallBucket.TotalValue != 2 {
		t.Fatalf("paywall bucket total_value = %d, want 2", paywallBucket.TotalValue)
	}

	trialBucket, ok := byKey["license_panel:"]
	if !ok {
		t.Fatal("missing license_panel: bucket")
	}
	if trialBucket.Type != conversion.EventTrialStarted {
		t.Fatalf("trial bucket type = %q, want %q", trialBucket.Type, conversion.EventTrialStarted)
	}
	if trialBucket.Count != 1 {
		t.Fatalf("trial bucket count = %d, want 1", trialBucket.Count)
	}
	if trialBucket.TotalValue != 1 {
		t.Fatalf("trial bucket total_value = %d, want 1", trialBucket.TotalValue)
	}
}

func TestConversionHandleGetStatsNonGET(t *testing.T) {
	handlers := NewConversionHandlers(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/conversion/stats", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetStats(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
