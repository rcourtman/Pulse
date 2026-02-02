package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

func TestNotificationQueueHandlers_GetDLQ_MissingScope(t *testing.T) {
	handler := &NotificationQueueHandlers{}

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/dlq", nil)
	record := &config.APITokenRecord{Scopes: []string{config.ScopeMonitoringWrite}}
	attachAPITokenRecord(req, record)

	rec := httptest.NewRecorder()
	handler.GetDLQ(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestNotificationQueueHandlers_GetDLQ_QueueNil(t *testing.T) {
	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "notificationMgr", &notifications.NotificationManager{})
	handler := NewNotificationQueueHandlers(monitor)

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/dlq", nil)
	rec := httptest.NewRecorder()
	handler.GetDLQ(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestNotificationQueueHandlers_GetQueueStats_QueueNil(t *testing.T) {
	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "notificationMgr", &notifications.NotificationManager{})
	handler := NewNotificationQueueHandlers(monitor)

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/queue/stats", nil)
	rec := httptest.NewRecorder()
	handler.GetQueueStats(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestNotificationQueueHandlers_RetryDLQItem_Errors(t *testing.T) {
	handler := &NotificationQueueHandlers{}

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/dlq/retry", bytes.NewReader([]byte("{bad")))
	rec := httptest.NewRecorder()
	handler.RetryDLQItem(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/notifications/dlq/retry", bytes.NewReader([]byte(`{"id":""}`)))
	rec = httptest.NewRecorder()
	handler.RetryDLQItem(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestNotificationQueueHandlers_RetryDLQItem_QueueNil(t *testing.T) {
	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "notificationMgr", &notifications.NotificationManager{})
	handler := NewNotificationQueueHandlers(monitor)

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/dlq/retry", bytes.NewReader([]byte(`{"id":"missing"}`)))
	rec := httptest.NewRecorder()
	handler.RetryDLQItem(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestNotificationQueueHandlers_DeleteDLQItem_Errors(t *testing.T) {
	handler := &NotificationQueueHandlers{}

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/dlq/delete", bytes.NewReader([]byte("{bad")))
	rec := httptest.NewRecorder()
	handler.DeleteDLQItem(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/notifications/dlq/delete", bytes.NewReader([]byte(`{"id":""}`)))
	rec = httptest.NewRecorder()
	handler.DeleteDLQItem(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestNotificationQueueHandlers_DeleteDLQItem_QueueNil(t *testing.T) {
	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "notificationMgr", &notifications.NotificationManager{})
	handler := NewNotificationQueueHandlers(monitor)

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/dlq/delete", bytes.NewReader([]byte(`{"id":"missing"}`)))
	rec := httptest.NewRecorder()
	handler.DeleteDLQItem(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestNotificationQueueHandlers_GetDLQ_InvalidLimit(t *testing.T) {
	handler, _ := newNotificationQueueHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/dlq?limit=invalid", nil)
	rec := httptest.NewRecorder()
	handler.GetDLQ(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var dlq []notifications.QueuedNotification
	if err := json.Unmarshal(rec.Body.Bytes(), &dlq); err != nil {
		t.Fatalf("decode dlq: %v", err)
	}
}
