package notifications

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestGetDLQLogsStructuredContextOnScanFailure(t *testing.T) {
	nq, err := NewNotificationQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewNotificationQueue failed: %v", err)
	}
	t.Cleanup(func() {
		_ = nq.Stop()
	})

	_, err = nq.db.Exec(
		`INSERT INTO notification_queue (id, type, method, status, alerts, config, attempts, max_attempts, created_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"bad-dlq-json",
		"webhook",
		"slack",
		QueueStatusDLQ,
		"{invalid-json",
		"{}",
		1,
		3,
		time.Now().Unix(),
		time.Now().Unix(),
	)
	if err != nil {
		t.Fatalf("failed to insert malformed dlq row: %v", err)
	}

	logOutput := captureNotificationQueueLogs(t)
	notifications, err := nq.GetDLQ(5)
	if err != nil {
		t.Fatalf("GetDLQ returned error: %v", err)
	}
	if len(notifications) != 0 {
		t.Fatalf("expected malformed row to be skipped, got %d notifications", len(notifications))
	}

	for _, expected := range []string{
		`"component":"notification_queue"`,
		`"action":"scan_dlq_row"`,
		`"queueStatus":"dlq"`,
		`"batchLimit":5`,
		`"dbPath":"` + nq.dbPath + `"`,
		`"message":"Failed to scan DLQ notification"`,
		`"error":"failed to unmarshal alerts`,
	} {
		if !strings.Contains(logOutput.String(), expected) {
			t.Fatalf("expected log output to include %s, got %q", expected, logOutput.String())
		}
	}
}

func TestCancelByAlertIDsLogsStructuredContextOnAlertUnmarshalFailure(t *testing.T) {
	nq, err := NewNotificationQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewNotificationQueue failed: %v", err)
	}
	t.Cleanup(func() {
		_ = nq.Stop()
	})

	_, err = nq.db.Exec(
		`INSERT INTO notification_queue (id, type, method, status, alerts, config, attempts, max_attempts, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"bad-cancel-json",
		"email",
		"smtp",
		QueueStatusSending,
		`{"broken":`,
		"{}",
		1,
		3,
		time.Now().Unix(),
	)
	if err != nil {
		t.Fatalf("failed to insert malformed sending row: %v", err)
	}

	logOutput := captureNotificationQueueLogs(t)
	if err := nq.CancelByAlertIDs([]string{"alert-1"}); err != nil {
		t.Fatalf("CancelByAlertIDs returned error: %v", err)
	}

	for _, expected := range []string{
		`"component":"notification_queue"`,
		`"action":"cancel_unmarshal_alerts"`,
		`"notifID":"bad-cancel-json"`,
		`"message":"Failed to unmarshal alerts for cancellation check"`,
		`"error":"unexpected end of JSON input"`,
	} {
		if !strings.Contains(logOutput.String(), expected) {
			t.Fatalf("expected log output to include %s, got %q", expected, logOutput.String())
		}
	}
}

func captureNotificationQueueLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	origLogger := log.Logger
	log.Logger = zerolog.New(&buf).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	t.Cleanup(func() {
		log.Logger = origLogger
	})

	return &buf
}
