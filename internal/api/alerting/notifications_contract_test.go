package alerting

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	tmock "github.com/stretchr/testify/mock"
)

// TestContract_WebhookSigningSecretMaskedAndPreserved pins the webhook
// management payload contract for delivery signing secrets.
func TestContract_WebhookSigningSecretMaskedAndPreserved(t *testing.T) {
	mockMonitor := new(MockNotificationMonitor)
	mockManager := new(MockNotificationManager)
	mockPersistence := new(MockNotificationConfigPersistence)
	mockMonitor.On("GetNotificationManager").Return(mockManager)
	mockMonitor.On("GetConfigPersistence").Return(mockPersistence)
	h := NewNotificationHandlers(nil, mockMonitor)

	stored := notifications.WebhookConfig{
		ID:            "wh-signed",
		Name:          "Signed Webhook",
		URL:           "https://psa.example.com/inbound/pulse",
		Enabled:       true,
		Service:       "generic",
		SigningSecret: "stored-secret",
	}

	mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{stored}).Once()
	rec := httptest.NewRecorder()
	h.GetWebhooks(rec, httptest.NewRequest(http.MethodGet, "/api/notifications/webhooks", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GetWebhooks responded %d", rec.Code)
	}
	var listed []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode webhook list: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected one webhook in list, got %d", len(listed))
	}
	if got := listed[0]["signingSecret"]; got != "***REDACTED***" {
		t.Fatalf("list signingSecret = %v, want masked placeholder", got)
	}

	mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{stored}).Once()
	mockManager.On("ValidateWebhookURL", stored.URL).Return(nil).Once()
	mockManager.On("UpdateWebhook", "wh-signed", tmock.MatchedBy(func(w notifications.WebhookConfig) bool {
		return w.SigningSecret == "stored-secret"
	})).Return(nil).Once()
	mockPersistence.On("SaveWebhooks", tmock.Anything).Return(nil).Once()

	update := stored
	update.SigningSecret = "***REDACTED***"
	body, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("marshal update: %v", err)
	}
	rec = httptest.NewRecorder()
	h.UpdateWebhook(rec, httptest.NewRequest(http.MethodPut, "/api/notifications/webhooks/wh-signed", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("UpdateWebhook responded %d: %s", rec.Code, rec.Body.String())
	}
	mockManager.AssertExpectations(t)
}
