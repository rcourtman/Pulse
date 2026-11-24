package alerting

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/mock"
)

// A blank password, a blank API key and a masked webhook value all mean "keep
// the saved one". None of them may carry the saved value to an address it was
// not entered for: the caller cannot read it, but would receive it there.

func savedCredentialHandlers() (*NotificationHandlers, *MockNotificationManager, *MockNotificationConfigPersistence) {
	monitor := new(MockNotificationMonitor)
	manager := new(MockNotificationManager)
	persistence := new(MockNotificationConfigPersistence)
	monitor.On("GetNotificationManager").Return(manager)
	monitor.On("GetConfigPersistence").Return(persistence)
	return NewNotificationHandlers(nil, monitor), manager, persistence
}

func TestSavedSMTPPasswordStaysWithItsServer(t *testing.T) {
	saved := notifications.EmailConfig{Enabled: true, SMTPHost: "smtp.operator.example", SMTPPort: 587, Username: "pulse", Password: "saved-password", StartTLS: true}
	cases := []struct {
		name      string
		requested notifications.EmailConfig
		want      int
		password  string // the password the accepted config is saved or tested with
	}{
		{"same server", notifications.EmailConfig{Enabled: true, SMTPHost: "SMTP.operator.example ", SMTPPort: 587, StartTLS: true, From: "new@operator.example"}, http.StatusOK, saved.Password},
		{"same server, implicit TLS instead of STARTTLS", notifications.EmailConfig{Enabled: true, SMTPHost: "smtp.operator.example", SMTPPort: 587, TLS: true}, http.StatusOK, saved.Password},
		{"password entered again", notifications.EmailConfig{Enabled: true, SMTPHost: "smtp.other.example", SMTPPort: 2525, Password: "entered"}, http.StatusOK, "entered"},
		// net.Dial reads ":2525" as this machine, so no server is not the saved server.
		{"no server named", notifications.EmailConfig{Enabled: true, SMTPPort: 2525}, http.StatusOK, ""},
		{"blank server", notifications.EmailConfig{Enabled: true, SMTPHost: " ", SMTPPort: 2525}, http.StatusOK, ""},
		{"another host", notifications.EmailConfig{Enabled: true, SMTPHost: "smtp.other.example", SMTPPort: 587, StartTLS: true}, http.StatusBadRequest, ""},
		{"another port", notifications.EmailConfig{Enabled: true, SMTPHost: "smtp.operator.example", SMTPPort: 2525, StartTLS: true}, http.StatusBadRequest, ""},
		{"encryption turned off", notifications.EmailConfig{Enabled: true, SMTPHost: "smtp.operator.example", SMTPPort: 587}, http.StatusBadRequest, ""},
	}
	for _, tc := range cases {
		t.Run("update/"+tc.name, func(t *testing.T) {
			h, manager, persistence := savedCredentialHandlers()
			manager.On("GetEmailConfig").Return(saved).Maybe()
			if tc.want == http.StatusOK {
				carries := mock.MatchedBy(func(config notifications.EmailConfig) bool { return config.Password == tc.password })
				persistence.On("SaveEmailConfig", carries).Return(nil).Once()
				manager.On("SetEmailConfig", carries).Return(nil).Once()
			}
			body, _ := json.Marshal(tc.requested)
			w := httptest.NewRecorder()
			h.UpdateEmailConfig(w, httptest.NewRequest(http.MethodPut, "/api/notifications/email", bytes.NewReader(body)))
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if strings.Contains(w.Body.String(), saved.Password) {
				t.Fatal("response carries the saved password")
			}
			manager.AssertExpectations(t)
			persistence.AssertExpectations(t)
		})
		t.Run("test/"+tc.name, func(t *testing.T) {
			h, manager, _ := savedCredentialHandlers()
			manager.On("GetEmailConfig").Return(saved).Maybe()
			if tc.want == http.StatusOK {
				carries := mock.MatchedBy(func(config *notifications.EmailConfig) bool { return config.Password == tc.password })
				manager.On("SendTestNotificationWithConfig", "email", carries, mock.Anything).Return(nil).Once()
				manager.On("IsEnabled").Return(true).Once()
			}
			body, _ := json.Marshal(map[string]any{"method": "email", "config": tc.requested})
			w := httptest.NewRecorder()
			h.TestNotification(w, httptest.NewRequest(http.MethodPost, "/api/notifications/test", bytes.NewReader(body)))
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			// Nothing is sent on refusal: the mock has no send expectation.
			manager.AssertExpectations(t)
		})
	}
}

func TestSavedAppriseAPIKeyStaysWithItsServer(t *testing.T) {
	saved := notifications.AppriseConfig{Enabled: true, Mode: notifications.AppriseModeHTTP, ServerURL: "https://apprise.operator.example", ConfigKey: "ops", APIKey: "saved-key"}
	cases := []struct {
		name      string
		requested notifications.AppriseConfig
		want      int
		apiKey    string // the API key the accepted config is saved or tested with
	}{
		{"same server", notifications.AppriseConfig{Enabled: true, Mode: notifications.AppriseModeHTTP, ServerURL: "https://APPRISE.operator.example:443/", ConfigKey: "other"}, http.StatusOK, saved.APIKey},
		{"key entered again", notifications.AppriseConfig{Enabled: true, Mode: notifications.AppriseModeHTTP, ServerURL: "https://apprise.other.example", APIKey: "entered"}, http.StatusOK, "entered"},
		// Without a server URL the key is sent nowhere, and a later URL must match none.
		{"no server named", notifications.AppriseConfig{Enabled: true}, http.StatusOK, saved.APIKey},
		{"another host", notifications.AppriseConfig{Enabled: true, Mode: notifications.AppriseModeHTTP, ServerURL: "https://apprise.other.example"}, http.StatusBadRequest, ""},
		{"another port", notifications.AppriseConfig{Enabled: true, Mode: notifications.AppriseModeHTTP, ServerURL: "https://apprise.operator.example:8443"}, http.StatusBadRequest, ""},
		{"plain HTTP", notifications.AppriseConfig{Enabled: true, Mode: notifications.AppriseModeHTTP, ServerURL: "http://apprise.operator.example:443"}, http.StatusBadRequest, ""},
		{"certificate check turned off", notifications.AppriseConfig{Enabled: true, Mode: notifications.AppriseModeHTTP, ServerURL: "https://apprise.operator.example", SkipTLSVerify: true}, http.StatusBadRequest, ""},
		{"another host parked under CLI mode", notifications.AppriseConfig{Enabled: true, ServerURL: "https://apprise.other.example"}, http.StatusBadRequest, ""},
	}
	for _, tc := range cases {
		carries := mock.MatchedBy(func(config notifications.AppriseConfig) bool { return config.APIKey == tc.apiKey })
		t.Run("update/"+tc.name, func(t *testing.T) {
			h, manager, persistence := savedCredentialHandlers()
			manager.On("GetAppriseConfig").Return(saved).Maybe()
			if tc.want == http.StatusOK {
				persistence.On("SaveAppriseConfig", carries).Return(nil).Once()
				manager.On("SetAppriseConfig", carries).Return(nil).Once()
			}
			body, _ := json.Marshal(tc.requested)
			w := httptest.NewRecorder()
			h.UpdateAppriseConfig(w, httptest.NewRequest(http.MethodPut, "/api/notifications/apprise", bytes.NewReader(body)))
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if strings.Contains(w.Body.String(), saved.APIKey) {
				t.Fatal("response carries the saved API key")
			}
			manager.AssertExpectations(t)
			persistence.AssertExpectations(t)
		})
		t.Run("test/"+tc.name, func(t *testing.T) {
			h, manager, _ := savedCredentialHandlers()
			manager.On("GetAppriseConfig").Return(saved).Maybe()
			if tc.want == http.StatusOK {
				manager.On("SendTestAppriseWithConfig", carries).Return(nil).Once()
				manager.On("IsEnabled").Return(true).Once()
			}
			body, _ := json.Marshal(map[string]any{"method": "apprise", "config": tc.requested})
			w := httptest.NewRecorder()
			h.TestNotification(w, httptest.NewRequest(http.MethodPost, "/api/notifications/test", bytes.NewReader(body)))
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			manager.AssertExpectations(t)
		})
	}
}

func TestSavedWebhookValuesStayWithTheirAddress(t *testing.T) {
	saved := notifications.WebhookConfig{
		ID: "wh1", Name: "Ops", URL: "https://hooks.operator.example/alerts", Enabled: true, Service: "generic",
		Headers:       map[string]string{"Authorization": "saved-header"},
		CustomFields:  map[string]string{"token": "saved-field"},
		SigningSecret: "saved-signing",
	}
	masked := func(url string) notifications.WebhookConfig {
		return notifications.WebhookConfig{
			ID: "wh1", Name: "Ops", URL: url, Enabled: true, Service: "generic",
			Headers:       map[string]string{"Authorization": maskedWebhookSecret},
			CustomFields:  map[string]string{"token": maskedWebhookSecret},
			SigningSecret: maskedWebhookSecret,
		}
	}
	entered := masked("https://hooks.other.example/alerts")
	entered.Headers = map[string]string{"Authorization": "entered"}
	entered.CustomFields = map[string]string{"token": "entered"}
	cases := []struct {
		name      string
		requested notifications.WebhookConfig
		want      int
	}{
		{"same address", masked(saved.URL), http.StatusOK},
		{"another path on the same origin", masked("https://HOOKS.operator.example:443/other?x=1"), http.StatusOK},
		{"values entered again, signing secret kept", entered, http.StatusOK},
		{"another host", masked("https://hooks.other.example/alerts"), http.StatusBadRequest},
		{"another port", masked("https://hooks.operator.example:8443/alerts"), http.StatusBadRequest},
		{"plain HTTP", masked("http://hooks.operator.example/alerts"), http.StatusBadRequest},
		{"templated host", masked("https://{{.Node}}.operator.example/alerts"), http.StatusBadRequest},
		{"saved host as userinfo", masked("https://hooks.operator.example@hooks.other.example/alerts"), http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run("update/"+tc.name, func(t *testing.T) {
			h, manager, persistence := savedCredentialHandlers()
			manager.On("GetWebhooks").Return([]notifications.WebhookConfig{saved}).Maybe()
			if tc.want == http.StatusOK {
				manager.On("ValidateWebhookURL", mock.Anything).Return(nil).Once()
				persistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()
				manager.On("UpdateWebhook", "wh1", mock.MatchedBy(func(webhook notifications.WebhookConfig) bool {
					return webhook.SigningSecret == saved.SigningSecret
				})).Return(nil).Once()
			}
			body, _ := json.Marshal(tc.requested)
			w := httptest.NewRecorder()
			h.UpdateWebhook(w, httptest.NewRequest(http.MethodPut, "/api/notifications/webhooks/wh1", bytes.NewReader(body)))
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "saved-") {
				t.Fatal("response carries a saved value")
			}
			manager.AssertExpectations(t)
			persistence.AssertExpectations(t)
		})
		t.Run("test/"+tc.name, func(t *testing.T) {
			h, manager, _ := savedCredentialHandlers()
			manager.On("GetWebhooks").Return([]notifications.WebhookConfig{saved}).Maybe()
			if tc.want == http.StatusOK {
				manager.On("ValidateWebhookURL", mock.Anything).Return(nil).Maybe()
				manager.On("TestEnhancedWebhook", mock.Anything).Return(200, "OK", nil).Once()
				manager.On("IsEnabled").Return(true).Maybe()
			}
			body, _ := json.Marshal(tc.requested)
			w := httptest.NewRecorder()
			h.TestWebhook(w, httptest.NewRequest(http.MethodPost, "/api/notifications/webhooks/test", bytes.NewReader(body)))
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			manager.AssertExpectations(t)
		})
	}
}
