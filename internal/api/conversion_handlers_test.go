package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
