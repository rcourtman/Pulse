package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

func newNotificationQueueHandlers(t *testing.T) (*NotificationQueueHandlers, *notifications.NotificationQueue) {
	t.Helper()

	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	cfg := &config.Config{DataPath: t.TempDir()}

	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })

	queue := monitor.GetNotificationManager().GetQueue()
	if queue == nil {
		t.Fatalf("expected notification queue to be initialized")
	}

	handler := NewNotificationQueueHandlers(monitor)
	return handler, queue
}

func enqueueDLQNotification(t *testing.T, queue *notifications.NotificationQueue, id string) {
	t.Helper()

	notification := &notifications.QueuedNotification{
		ID:            id,
		Type:          "webhook",
		DestinationID: "webhook:primary",
		Status:        notifications.QueueStatusDLQ,
		Alerts: []*alerts.Alert{{
			ID:   "alert-1",
			Type: "test",
			OperationalRecord: &operationaltrust.OperationalRecord{
				ID: "record-1",
			},
			LatestTransition: &operationaltrust.LifecycleTransition{
				ID:                  "transition-1",
				OperationalRecordID: "record-1",
				To:                  operationaltrust.OperationalOpen,
				CauseKey:            "alert-1",
			},
		}},
		Config: json.RawMessage(`{}`),
	}
	if err := queue.Enqueue(notification); err != nil {
		t.Fatalf("queue.Enqueue: %v", err)
	}
}

func TestNotificationQueueHandlers_GetDLQAndStats(t *testing.T) {
	handler, queue := newNotificationQueueHandlers(t)
	enqueueDLQNotification(t, queue, "notif-1")

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/dlq?limit=10", nil)
	rec := httptest.NewRecorder()
	handler.GetDLQ(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GetDLQ status = %d, want 200", rec.Code)
	}

	var dlq []notifications.QueuedNotification
	if err := json.Unmarshal(rec.Body.Bytes(), &dlq); err != nil {
		t.Fatalf("decode DLQ: %v", err)
	}
	if len(dlq) != 1 || dlq[0].ID != "notif-1" {
		t.Fatalf("DLQ = %+v, want notif-1", dlq)
	}
	if len(dlq[0].Links) != 1 ||
		dlq[0].Links[0].TransitionID != "transition-1" ||
		dlq[0].Links[0].DeliveryState != operationaltrust.NotificationDeadLetter {
		t.Fatalf("DLQ operational links = %+v", dlq[0].Links)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/notifications/queue/stats", nil)
	rec = httptest.NewRecorder()
	handler.GetQueueStats(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GetQueueStats status = %d, want 200", rec.Code)
	}
}

func TestNotificationQueueHandlers_RetryAndDelete(t *testing.T) {
	handler, queue := newNotificationQueueHandlers(t)
	enqueueDLQNotification(t, queue, "notif-2")

	retryBody := []byte(`{"id":"notif-2"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/notifications/dlq/retry", bytes.NewReader(retryBody))
	rec := httptest.NewRecorder()
	handler.RetryDLQItem(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("RetryDLQItem status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	deleteBody := []byte(`{"id":"notif-2"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/notifications/dlq/delete", bytes.NewReader(deleteBody))
	rec = httptest.NewRecorder()
	handler.DeleteDLQItem(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DeleteDLQItem status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestNotificationQueueHandlers_HandleNotificationQueue(t *testing.T) {
	handler, queue := newNotificationQueueHandlers(t)
	enqueueDLQNotification(t, queue, "notif-3")

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/dlq", nil)
	rec := httptest.NewRecorder()
	handler.HandleNotificationQueue(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("HandleNotificationQueue DLQ status = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/notifications/unknown", nil)
	rec = httptest.NewRecorder()
	handler.HandleNotificationQueue(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("HandleNotificationQueue status = %d, want 404", rec.Code)
	}
}
