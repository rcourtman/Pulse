package alerting

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUpdateEmailConfigBrandingPresenceAndNormalization(t *testing.T) {
	mockMonitor := new(MockNotificationMonitor)
	mockManager := new(MockNotificationManager)
	mockPersistence := new(MockNotificationConfigPersistence)

	mockMonitor.On("GetNotificationManager").Return(mockManager)
	mockMonitor.On("GetConfigPersistence").Return(mockPersistence)

	h := NewNotificationHandlers(nil, mockMonitor)

	t.Run("normalizes_and_saves_branding_fields", func(t *testing.T) {
		mockManager.On("GetEmailConfig").Return(notifications.EmailConfig{}).Once()
		expected := func(cfg notifications.EmailConfig) bool {
			return cfg.SubjectPrefix == "[Acme Ops]" &&
				cfg.BrandName == "Acme Infrastructure" &&
				cfg.FooterNote == "Escalation desk"
		}
		mockPersistence.On("SaveEmailConfig", mock.MatchedBy(expected)).Return(nil).Once()
		mockManager.On("SetEmailConfig", mock.MatchedBy(expected)).Return().Once()

		req := httptest.NewRequest(
			http.MethodPut,
			"/api/notifications/email",
			bytes.NewBufferString(`{"enabled":true,"subjectPrefix":"  [Acme Ops]\r\n ","brandName":"  Acme\tInfrastructure ","footerNote":"Escalation\ndesk"}`),
		)
		w := httptest.NewRecorder()
		h.UpdateEmailConfig(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockManager.AssertExpectations(t)
		mockPersistence.AssertExpectations(t)
	})

	t.Run("preserves_omitted_branding_fields", func(t *testing.T) {
		existing := notifications.EmailConfig{
			SubjectPrefix: "[Acme Ops]",
			BrandName:     "Acme Infrastructure",
			FooterNote:    "Escalation desk",
		}
		mockManager.On("GetEmailConfig").Return(existing).Once()
		expected := func(cfg notifications.EmailConfig) bool {
			return cfg.SubjectPrefix == "[Acme Ops]" &&
				cfg.BrandName == "Acme Infrastructure" &&
				cfg.FooterNote == "Escalation desk"
		}
		mockPersistence.On("SaveEmailConfig", mock.MatchedBy(expected)).Return(nil).Once()
		mockManager.On("SetEmailConfig", mock.MatchedBy(expected)).Return().Once()

		req := httptest.NewRequest(
			http.MethodPut,
			"/api/notifications/email",
			bytes.NewBufferString(`{"enabled":true}`),
		)
		w := httptest.NewRecorder()
		h.UpdateEmailConfig(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockManager.AssertExpectations(t)
		mockPersistence.AssertExpectations(t)
	})

	t.Run("clears_branding_fields_when_empty_strings_sent", func(t *testing.T) {
		existing := notifications.EmailConfig{
			SubjectPrefix: "[Acme Ops]",
			BrandName:     "Acme Infrastructure",
			FooterNote:    "Escalation desk",
		}
		mockManager.On("GetEmailConfig").Return(existing).Once()
		expected := func(cfg notifications.EmailConfig) bool {
			return cfg.SubjectPrefix == "" &&
				cfg.BrandName == "" &&
				cfg.FooterNote == ""
		}
		mockPersistence.On("SaveEmailConfig", mock.MatchedBy(expected)).Return(nil).Once()
		mockManager.On("SetEmailConfig", mock.MatchedBy(expected)).Return().Once()

		req := httptest.NewRequest(
			http.MethodPut,
			"/api/notifications/email",
			bytes.NewBufferString(`{"enabled":true,"subjectPrefix":"","brandName":"","footerNote":""}`),
		)
		w := httptest.NewRecorder()
		h.UpdateEmailConfig(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockManager.AssertExpectations(t)
		mockPersistence.AssertExpectations(t)
	})
}
