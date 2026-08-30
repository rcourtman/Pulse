package alerting

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/mock"
)

func TestGetNotificationHealthReportsRetainedTerminalFailures(t *testing.T) {
	mockMonitor := new(MockNotificationMonitor)
	mockManager := new(MockNotificationManager)
	mockPersistence := new(MockNotificationConfigPersistence)
	mockMonitor.On("GetNotificationManager").Return(mockManager)
	mockMonitor.On("GetConfigPersistence").Return(mockPersistence)
	mockManager.On("GetQueueStats").Return(map[string]int{
		"pending": 4,
		"sending": 1,
		"sent":    9,
		"failed":  2,
		"dlq":     3,
	}, nil).Once()
	mockManager.On("GetTelemetryStats", mock.Anything).Return(notifications.TelemetryStats{
		Failures: 5,
		FailureClasses: notifications.NotificationFailureClassCounts{
			Authentication: 3,
			Connectivity:   2,
		},
	}, nil).Once()
	mockManager.On("GetEmailConfig").Return(notifications.EmailConfig{}).Once()
	mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{}).Once()
	mockPersistence.On("IsEncryptionEnabled").Return(true).Once()

	rec := httptest.NewRecorder()
	NewNotificationHandlers(nil, mockMonitor).GetNotificationHealth(
		rec,
		httptest.NewRequest(http.MethodGet, "/api/notifications/health", nil),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var response struct {
		OverallHealthy bool `json:"overall_healthy"`
		Queue          struct {
			Healthy                      bool           `json:"healthy"`
			Status                       string         `json:"status"`
			AttentionRequired            int            `json:"attention_required"`
			ReasonCodes                  []string       `json:"reason_codes"`
			CompletedRetentionDays       int            `json:"completed_retention_days"`
			DeadLetterRetentionDays      int            `json:"dead_letter_retention_days"`
			CountsAreRetentionBounded    bool           `json:"counts_are_retention_bounded"`
			RetryAttemptsAffectHealth    bool           `json:"retry_attempts_affect_health"`
			TerminalFailuresAffectHealth bool           `json:"terminal_failures_affect_health"`
			FailureClasses7d             map[string]int `json:"failure_classes_7d"`
			FailureClassesAvailable      bool           `json:"failure_classes_available"`
			FailureClassWindowDays       int            `json:"failure_class_window_days"`
		} `json:"queue"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if response.OverallHealthy || response.Queue.Healthy || response.Queue.Status != "degraded" {
		t.Fatalf("health response = %#v, want degraded and unhealthy", response)
	}
	if response.Queue.AttentionRequired != 5 {
		t.Fatalf("attention_required = %d, want 5", response.Queue.AttentionRequired)
	}
	if len(response.Queue.ReasonCodes) != 2 ||
		response.Queue.ReasonCodes[0] != "retained_failed_deliveries" ||
		response.Queue.ReasonCodes[1] != "retained_dead_letter_deliveries" {
		t.Fatalf("reason_codes = %#v", response.Queue.ReasonCodes)
	}
	if response.Queue.CompletedRetentionDays != 7 ||
		response.Queue.DeadLetterRetentionDays != 30 ||
		!response.Queue.CountsAreRetentionBounded ||
		response.Queue.RetryAttemptsAffectHealth ||
		!response.Queue.TerminalFailuresAffectHealth {
		t.Fatalf("queue semantics = %#v", response.Queue)
	}
	if !response.Queue.FailureClassesAvailable ||
		response.Queue.FailureClassWindowDays != 7 ||
		response.Queue.FailureClasses7d["authentication"] != 3 ||
		response.Queue.FailureClasses7d["connectivity"] != 2 {
		t.Fatalf("failure classes = %#v", response.Queue)
	}
}

func TestGetNotificationHealthFailsClosedWhenQueueStatsAreUnavailable(t *testing.T) {
	mockMonitor := new(MockNotificationMonitor)
	mockManager := new(MockNotificationManager)
	mockPersistence := new(MockNotificationConfigPersistence)
	mockMonitor.On("GetNotificationManager").Return(mockManager)
	mockMonitor.On("GetConfigPersistence").Return(mockPersistence)
	mockManager.On("GetQueueStats").Return(nil, errors.New("database path /secret unavailable")).Once()
	mockManager.On("GetTelemetryStats", mock.Anything).Return(
		notifications.TelemetryStats{},
		errors.New("database unavailable"),
	).Once()
	mockManager.On("GetEmailConfig").Return(notifications.EmailConfig{}).Once()
	mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{}).Once()
	mockPersistence.On("IsEncryptionEnabled").Return(false).Once()

	rec := httptest.NewRecorder()
	NewNotificationHandlers(nil, mockMonitor).GetNotificationHealth(
		rec,
		httptest.NewRequest(http.MethodGet, "/api/notifications/health", nil),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var response struct {
		OverallHealthy bool `json:"overall_healthy"`
		Queue          struct {
			Healthy     bool     `json:"healthy"`
			Status      string   `json:"status"`
			ReasonCodes []string `json:"reason_codes"`
		} `json:"queue"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if response.OverallHealthy || response.Queue.Healthy || response.Queue.Status != "unavailable" {
		t.Fatalf("health response = %#v, want unavailable and unhealthy", response)
	}
	if len(response.Queue.ReasonCodes) != 1 ||
		response.Queue.ReasonCodes[0] != "queue_stats_unavailable" {
		t.Fatalf("reason_codes = %#v", response.Queue.ReasonCodes)
	}
	if body := rec.Body.String(); body == "" ||
		containsAny(body, "database path", "/secret") {
		t.Fatalf("health response exposed internal queue error: %s", body)
	}
}

func TestGetDeliveryLogReturnsEntriesWithRedactedErrors(t *testing.T) {
	mockMonitor := new(MockNotificationMonitor)
	mockManager := new(MockNotificationManager)
	mockMonitor.On("GetNotificationManager").Return(mockManager)
	mockManager.On("GetDeliveryLog", mock.MatchedBy(func(since time.Time) bool {
		age := time.Since(since)
		return age >= 29*24*time.Hour && age <= 31*24*time.Hour
	}), 25).Return([]notifications.DeliveryLogEntry{
		{
			NotificationID: "webhook-1",
			Type:           "webhook",
			DestinationID:  "wh-ops",
			Outcome:        notifications.DeliveryOutcomeFailed,
			AlertIDs:       []string{"vm-offline-101"},
			AlertCount:     1,
			Attempts:       3,
			ErrorMessage:   "post https://hooks.example.test/notify?token=supersecret returned 401",
			FailureClass:   "authentication",
		},
		{
			NotificationID: "email-1",
			Type:           "email",
			Outcome:        notifications.DeliveryOutcomeSent,
			AlertIDs:       []string{"disk-critical-1"},
			AlertCount:     1,
			Attempts:       1,
			Success:        true,
		},
	}, nil).Once()

	rec := httptest.NewRecorder()
	NewNotificationHandlers(nil, mockMonitor).GetDeliveryLog(
		rec,
		httptest.NewRequest(http.MethodGet, "/api/notifications/delivery-log?limit=25", nil),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var response struct {
		Entries []struct {
			NotificationID string   `json:"notificationId"`
			Outcome        string   `json:"outcome"`
			DestinationID  string   `json:"destinationId"`
			AlertIDs       []string `json:"alertIds"`
			ErrorMessage   string   `json:"errorMessage"`
			FailureClass   string   `json:"failureClass"`
		} `json:"entries"`
		WindowDays                 int  `json:"window_days"`
		CompletedRetentionDays     int  `json:"completed_retention_days"`
		DeadLetterRetentionDays    int  `json:"dead_letter_retention_days"`
		EntriesAreRetentionBounded bool `json:"entries_are_retention_bounded"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode delivery log response: %v", err)
	}
	if len(response.Entries) != 2 {
		t.Fatalf("entries = %#v, want 2", response.Entries)
	}
	if response.Entries[0].Outcome != "failed" ||
		response.Entries[0].DestinationID != "wh-ops" ||
		response.Entries[0].FailureClass != "authentication" ||
		len(response.Entries[0].AlertIDs) != 1 ||
		response.Entries[0].AlertIDs[0] != "vm-offline-101" {
		t.Fatalf("failed entry = %#v", response.Entries[0])
	}
	if strings.Contains(response.Entries[0].ErrorMessage, "supersecret") ||
		!strings.Contains(response.Entries[0].ErrorMessage, "token=REDACTED") {
		t.Fatalf("error message not redacted: %q", response.Entries[0].ErrorMessage)
	}
	if response.WindowDays != 30 ||
		response.CompletedRetentionDays != 7 ||
		response.DeadLetterRetentionDays != 30 ||
		!response.EntriesAreRetentionBounded {
		t.Fatalf("retention context = %#v", response)
	}
}

func TestGetDeliveryLogRejectsInvalidLimit(t *testing.T) {
	mockMonitor := new(MockNotificationMonitor)
	mockManager := new(MockNotificationManager)
	mockMonitor.On("GetNotificationManager").Return(mockManager)

	for _, limit := range []string{"abc", "-1", "0"} {
		rec := httptest.NewRecorder()
		NewNotificationHandlers(nil, mockMonitor).GetDeliveryLog(
			rec,
			httptest.NewRequest(http.MethodGet, "/api/notifications/delivery-log?limit="+limit, nil),
		)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("limit=%q status = %d, want %d", limit, rec.Code, http.StatusBadRequest)
		}
	}
	mockManager.AssertNotCalled(t, "GetDeliveryLog", mock.Anything, mock.Anything)
}

func TestGetDeliveryLogFailsClosedWhenQueueUnavailable(t *testing.T) {
	mockMonitor := new(MockNotificationMonitor)
	mockManager := new(MockNotificationManager)
	mockMonitor.On("GetNotificationManager").Return(mockManager)
	mockManager.On("GetDeliveryLog", mock.Anything, 0).Return(
		nil, errors.New("database path /secret unavailable"),
	).Once()

	rec := httptest.NewRecorder()
	NewNotificationHandlers(nil, mockMonitor).GetDeliveryLog(
		rec,
		httptest.NewRequest(http.MethodGet, "/api/notifications/delivery-log", nil),
	)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if body := rec.Body.String(); containsAny(body, "database path", "/secret") {
		t.Fatalf("delivery log response exposed internal queue error: %s", body)
	}
}

// A test send bypasses the activation gate real alerts honor, so its result
// must say when real delivery is paused instead of reporting bare success.
func TestTestNotificationReportsPausedDeliveryInResult(t *testing.T) {
	mockMonitor := new(MockNotificationMonitor)
	mockManager := new(MockNotificationManager)
	mockMonitor.On("GetNotificationManager").Return(mockManager)
	mockManager.On("SendTestNotification", "email").Return(nil).Once()
	mockManager.On("IsEnabled").Return(false).Once()

	rec := httptest.NewRecorder()
	NewNotificationHandlers(nil, mockMonitor).TestNotification(
		rec,
		httptest.NewRequest(
			http.MethodPost,
			"/api/notifications/test",
			strings.NewReader(`{"method":"email"}`),
		),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var response struct {
		Status         string `json:"status"`
		Message        string `json:"message"`
		DeliveryPaused bool   `json:"deliveryPaused"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode test notification response: %v", err)
	}
	if response.Status != "success" || !response.DeliveryPaused {
		t.Fatalf("response = %#v, want success with deliveryPaused", response)
	}
	if !strings.Contains(response.Message, "paused") {
		t.Fatalf("message %q does not warn about paused delivery", response.Message)
	}
}

func TestTestNotificationOmitsPausedFlagWhenDeliveryActive(t *testing.T) {
	mockMonitor := new(MockNotificationMonitor)
	mockManager := new(MockNotificationManager)
	mockMonitor.On("GetNotificationManager").Return(mockManager)
	mockManager.On("SendTestNotification", "email").Return(nil).Once()
	mockManager.On("IsEnabled").Return(true).Once()

	rec := httptest.NewRecorder()
	NewNotificationHandlers(nil, mockMonitor).TestNotification(
		rec,
		httptest.NewRequest(
			http.MethodPost,
			"/api/notifications/test",
			strings.NewReader(`{"method":"email"}`),
		),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if strings.Contains(rec.Body.String(), "deliveryPaused") {
		t.Fatalf("active delivery response carries paused flag: %s", rec.Body.String())
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
