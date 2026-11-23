package alerting

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/stretchr/testify/require"
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

	var dlq []struct {
		ID    string                              `json:"id"`
		Links []operationaltrust.NotificationLink `json:"links"`
	}
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

func TestNotificationQueueHandlers_GetDLQSummaryPreservesStoredRecord(t *testing.T) {
	for _, status := range []int{http.StatusLocked, http.StatusTooEarly} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			assertDLQSummaryPreservesStoredRecord(t, status)
		})
	}
}

func assertDLQSummaryPreservesStoredRecord(t *testing.T, status int) {
	t.Helper()
	handler, queue := newNotificationQueueHandlers(t)
	// An unattempted row without a completion time must not displace the
	// completed row when the list is limited to one.
	enqueueDLQNotification(t, queue, "older-row")
	item := notifications.QueuedNotification{
		ID:          "delivery-fixture",
		Type:        "webhook",
		Method:      "fixture-method",
		Status:      notifications.QueueStatusDLQ,
		Attempts:    2,
		MaxAttempts: 5,
		CreatedAt:   time.Now().UTC().Truncate(time.Second).Add(-time.Minute),
		Config:      json.RawMessage(`{ "fixtureOption": { "value": "stored-fixture" } }`),
		Alerts: []*alerts.Alert{{
			ID:         "alert-fixture",
			Type:       "memory",
			Level:      alerts.AlertLevelWarning,
			ResourceID: "resource-fixture",
			Message:    "stored alert detail",
			Metadata:   map[string]interface{}{"detail": "stored fixture metadata"},
		}},
	}
	require.NoError(t, queue.Enqueue(&item))
	// Store the prose in the queue and the structured verdict in the audit, as
	// a sender does. The queue summary must not turn these 4xx codes into a
	// rejection when the recorded attempt says they were a server error.
	failure := notifications.FailfWithClass(notifications.ClassFromHTTPStatus(status), "HTTP %d: stored fixture diagnostic", status)
	require.NoError(t, queue.MoveToDLQ(item.ID, failure.Error()))
	require.NoError(t, queue.RecordAuditError(&item, false, failure))

	rec := httptest.NewRecorder()
	handler.GetDLQ(rec, httptest.NewRequest(http.MethodGet, "/api/notifications/dlq?limit=1", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	stored, err := queue.GetDLQ(1)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	saved := stored[0]
	require.Equal(t, item.ID, saved.ID)
	require.True(t, bytes.Equal(item.Config, saved.Config), "provider snapshot changed on read")
	require.NotNil(t, saved.LastError)
	require.Equal(t, failure.Error(), *saved.LastError)
	require.NotNil(t, saved.LastAttempt)
	require.NotNil(t, saved.CompletedAt)
	require.Len(t, saved.Alerts, 1)
	require.Equal(t, item.Alerts[0].Message, saved.Alerts[0].Message)
	require.Equal(t, item.Alerts[0].Metadata, saved.Alerts[0].Metadata)

	// Assert the entire wire shape, so an accidentally serialized storage
	// field (including any full alert payload) fails the response contract.
	want, err := json.Marshal([]map[string]interface{}{{
		"id":     item.ID,
		"type":   "webhook",
		"method": "fixture-method",
		"status": "dlq",
		"alerts": []map[string]string{{
			"id": "alert-fixture", "type": "memory", "level": "warning", "resourceId": "resource-fixture",
		}},
		"attempts":    2,
		"maxAttempts": 5,
		"createdAt":   saved.CreatedAt,
		"lastAttempt": saved.LastAttempt,
		"completedAt": saved.CompletedAt,
	}})
	require.NoError(t, err)
	require.JSONEq(t, string(want), rec.Body.String())
	require.True(t, saved.CreatedAt.Equal(item.CreatedAt))

	logRec := httptest.NewRecorder()
	NewNotificationHandlers(nil, NewNotificationMonitorWrapper(handler.monitor)).GetDeliveryLog(
		logRec, httptest.NewRequest(http.MethodGet, "/api/notifications/delivery-log?limit=1", nil),
	)
	require.Equal(t, http.StatusOK, logRec.Code)
	var deliveryLog struct {
		Entries []notifications.DeliveryLogEntry `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(logRec.Body.Bytes(), &deliveryLog))
	require.Len(t, deliveryLog.Entries, 1)
	attempt := deliveryLog.Entries[0]
	require.Equal(t, saved.ID, attempt.NotificationID)
	require.Equal(t, saved.Attempts, attempt.Attempts)
	require.Equal(t, notifications.DeliveryOutcomeDeadLetter, attempt.Outcome)
	require.Equal(t, string(notifications.NotificationFailureServerError), attempt.FailureClass)
}

func TestNotificationQueueHandlers_GetDLQEmpty(t *testing.T) {
	handler, _ := newNotificationQueueHandlers(t)
	rec := httptest.NewRecorder()
	handler.GetDLQ(rec, httptest.NewRequest(http.MethodGet, "/api/notifications/dlq", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, "null", rec.Body.String())
}

func TestNotificationQueueHandlers_RetryAndDelete(t *testing.T) {
	handler, queue := newNotificationQueueHandlers(t)
	queue.SetProcessor(nil) // Inspect the operator transitions without sending.
	enqueueDLQNotification(t, queue, "notif-2")

	rec := httptest.NewRecorder()
	handler.GetDLQ(rec, httptest.NewRequest(http.MethodGet, "/api/notifications/dlq", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var entries []struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &entries))
	require.Len(t, entries, 1)
	require.Equal(t, "notif-2", entries[0].ID)
	body, err := json.Marshal(entries[0])
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/dlq/retry", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.RetryDLQItem(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"success":true,"message":"Notification scheduled for retry","id":"notif-2"}`, rec.Body.String())
	stats, err := queue.GetQueueStats()
	require.NoError(t, err)
	require.Equal(t, 1, stats["pending"])
	require.Zero(t, stats["dlq"])

	req = httptest.NewRequest(http.MethodPost, "/api/notifications/dlq/delete", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.DeleteDLQItem(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"success":true,"message":"Notification deleted from DLQ","id":"notif-2"}`, rec.Body.String())
	stats, err = queue.GetQueueStats()
	require.NoError(t, err)
	require.Zero(t, stats["pending"])
	require.Equal(t, 1, stats["cancelled"])
}

func TestNotificationQueueHandlers_BulkTerminalFailureRecovery(t *testing.T) {
	handler, queue := newNotificationQueueHandlers(t)
	enqueueDLQNotification(t, queue, "notif-bulk")
	if err := queue.UpdateStatus("notif-bulk", notifications.QueueStatusDLQ, "destination unavailable"); err != nil {
		t.Fatalf("mark notification DLQ: %v", err)
	}
	assertNotificationDeliveryAlertActive(t, handler.monitor)

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/terminal-failures/retry", nil)
	rec := httptest.NewRecorder()
	handler.RetryTerminalFailures(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("RetryTerminalFailures status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Affected int `json:"affected"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode retry response: %v", err)
	}
	if response.Affected != 1 {
		t.Fatalf("retry affected = %d, want 1", response.Affected)
	}
	for _, active := range handler.monitor.GetAlertManager().GetActiveAlerts() {
		if active.Type == alerts.NotificationDeliveryAlertType {
			t.Fatalf("notification delivery alert remained active after retry: %+v", active)
		}
	}

	// Use a distinct terminal delivery for dismissal. The retry above is now
	// pending and the real background worker is entitled to deliver it at any
	// time; forcing that in-flight row back to DLQ makes this handler test race
	// the queue processor instead of testing the dismissal contract.
	enqueueDLQNotification(t, queue, "notif-dismiss")
	if err := queue.UpdateStatus("notif-dismiss", notifications.QueueStatusDLQ, "still unavailable"); err != nil {
		t.Fatalf("mark dismiss notification DLQ: %v", err)
	}
	assertNotificationDeliveryAlertActive(t, handler.monitor)
	req = httptest.NewRequest(http.MethodPost, "/api/notifications/terminal-failures/dismiss", nil)
	rec = httptest.NewRecorder()
	handler.DismissTerminalFailures(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DismissTerminalFailures status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode dismiss response: %v", err)
	}
	if response.Affected != 1 {
		t.Fatalf("dismiss affected = %d, want 1", response.Affected)
	}
	for _, active := range handler.monitor.GetAlertManager().GetActiveAlerts() {
		if active.Type == alerts.NotificationDeliveryAlertType {
			t.Fatalf("notification delivery alert remained active after dismiss: %+v", active)
		}
	}
}

func assertNotificationDeliveryAlertActive(t *testing.T, monitor *monitoring.Monitor) {
	t.Helper()
	for _, active := range monitor.GetAlertManager().GetActiveAlerts() {
		if active.Type == alerts.NotificationDeliveryAlertType {
			return
		}
	}
	t.Fatal("expected notification delivery alert to be active")
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
