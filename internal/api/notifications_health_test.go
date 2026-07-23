package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
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
			Healthy                      bool     `json:"healthy"`
			Status                       string   `json:"status"`
			AttentionRequired            int      `json:"attention_required"`
			ReasonCodes                  []string `json:"reason_codes"`
			CompletedRetentionDays       int      `json:"completed_retention_days"`
			DeadLetterRetentionDays      int      `json:"dead_letter_retention_days"`
			CountsAreRetentionBounded    bool     `json:"counts_are_retention_bounded"`
			RetryAttemptsAffectHealth    bool     `json:"retry_attempts_affect_health"`
			TerminalFailuresAffectHealth bool     `json:"terminal_failures_affect_health"`
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
}

func TestGetNotificationHealthFailsClosedWhenQueueStatsAreUnavailable(t *testing.T) {
	mockMonitor := new(MockNotificationMonitor)
	mockManager := new(MockNotificationManager)
	mockPersistence := new(MockNotificationConfigPersistence)
	mockMonitor.On("GetNotificationManager").Return(mockManager)
	mockMonitor.On("GetConfigPersistence").Return(mockPersistence)
	mockManager.On("GetQueueStats").Return(nil, errors.New("database path /secret unavailable")).Once()
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

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
