package alerting

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/stretchr/testify/assert"
)

// An empty tenant manager has no persistence, so every organization lookup
// it is asked for fails. The default-org mocks carry no expectations: any
// call that falls back to them fails the test.

func tenantRequest(method, target, orgID string) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewReader([]byte(`{}`)))
	return req.WithContext(context.WithValue(req.Context(), apicontext.OrgIDContextKey, orgID))
}

func TestAlertHandlersGetMonitor_NonDefaultOrgFailsClosed(t *testing.T) {
	defaultMonitor := new(MockAlertMonitor)
	tenantCtx := context.WithValue(context.Background(), apicontext.OrgIDContextKey, "tenant-a")

	h := NewAlertHandlers(&monitoring.MultiTenantMonitor{}, defaultMonitor, nil)
	assert.Nil(t, h.getMonitor(tenantCtx))
	assert.Nil(t, h.MonitorForContext(tenantCtx))
	assert.Same(t, defaultMonitor, h.getMonitor(context.Background()))

	noManager := NewAlertHandlers(nil, defaultMonitor, nil)
	assert.Nil(t, noManager.getMonitor(tenantCtx))
	assert.Same(t, defaultMonitor, noManager.getMonitor(context.Background()))
}

func TestAlertHandlers_UnresolvableTenantReturns503(t *testing.T) {
	defaultMonitor := new(MockAlertMonitor)
	h := NewAlertHandlers(&monitoring.MultiTenantMonitor{}, defaultMonitor, nil)

	cases := []struct {
		name    string
		method  string
		target  string
		handler http.HandlerFunc
	}{
		{"get config", http.MethodGet, "/api/alerts/config", h.GetAlertConfig},
		{"update config", http.MethodPut, "/api/alerts/config", h.UpdateAlertConfig},
		{"active alerts", http.MethodGet, "/api/alerts/active", h.GetActiveAlerts},
		{"clear history", http.MethodDelete, "/api/alerts/history", h.ClearAlertHistory},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.handler(rec, tenantRequest(tc.method, tc.target, "tenant-a"))
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
		})
	}
	defaultMonitor.AssertExpectations(t)
}

func TestNotificationHandlersGetMonitor_NonDefaultOrgFailsClosed(t *testing.T) {
	defaultMonitor := new(MockNotificationMonitor)
	tenantCtx := context.WithValue(context.Background(), apicontext.OrgIDContextKey, "tenant-a")

	h := NewNotificationHandlers(&monitoring.MultiTenantMonitor{}, defaultMonitor)
	assert.Nil(t, h.getMonitor(tenantCtx))
	assert.Nil(t, h.MonitorForContext(tenantCtx))
	assert.Same(t, defaultMonitor, h.getMonitor(context.Background()))

	noManager := NewNotificationHandlers(nil, defaultMonitor)
	assert.Nil(t, noManager.getMonitor(tenantCtx))
	assert.Same(t, defaultMonitor, noManager.getMonitor(context.Background()))
}

func TestNotificationHandlers_UnresolvableTenantReturns503(t *testing.T) {
	defaultMonitor := new(MockNotificationMonitor)
	h := NewNotificationHandlers(&monitoring.MultiTenantMonitor{}, defaultMonitor)

	cases := []struct {
		name    string
		method  string
		target  string
		handler http.HandlerFunc
	}{
		{"get email", http.MethodGet, "/api/notifications/email", h.GetEmailConfig},
		{"update email", http.MethodPut, "/api/notifications/email", h.UpdateEmailConfig},
		{"get webhooks", http.MethodGet, "/api/notifications/webhooks", h.GetWebhooks},
		{"delete webhook", http.MethodDelete, "/api/notifications/webhooks/wh-1", h.DeleteWebhook},
		{"health", http.MethodGet, "/api/notifications/health", h.GetNotificationHealth},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.handler(rec, tenantRequest(tc.method, tc.target, "tenant-a"))
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
		})
	}
	defaultMonitor.AssertExpectations(t)
}
