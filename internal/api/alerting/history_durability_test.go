package alerting

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type durabilityTestManager struct {
	*MockAlertManager
	status   eventlog.DurabilityStatus
	called   int
	revision uint64
	reason   string
	err      error
}

func (m *durabilityTestManager) EventHistoryDurability() eventlog.DurabilityStatus { return m.status }
func (m *durabilityTestManager) AcknowledgeEventHistoryGaps(revision uint64, reason string) error {
	m.called++
	m.revision = revision
	m.reason = reason
	return m.err
}

func durabilityTestHandler(manager *durabilityTestManager) *AlertHandlers {
	monitor := new(MockAlertMonitor)
	monitor.On("GetAlertManager").Return(manager)
	return NewAlertHandlers(nil, monitor, nil)
}

func TestHistoryDurabilityReportsGapsAndHidesStoragePaths(t *testing.T) {
	manager := &durabilityTestManager{MockAlertManager: new(MockAlertManager), status: eventlog.DurabilityStatus{LostEvents: 4, UnknownGap: true, GapRevision: 7, Error: "/private/tenant-a/events.db unavailable"}}
	h := durabilityTestHandler(manager)
	rec := httptest.NewRecorder()
	h.HandleAlerts(rec, httptest.NewRequest(http.MethodGet, "/api/alerts/history/durability", nil))
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "/private/") {
		t.Fatalf("durability response: %d %s", rec.Code, rec.Body)
	}
	var response struct {
		Healthy    bool                      `json:"healthy"`
		Durability eventlog.DurabilityStatus `json:"durability"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Healthy || response.Durability.LostEvents != 4 || !response.Durability.UnknownGap || response.Durability.GapRevision != 7 {
		t.Fatalf("gap concealed: %+v", response)
	}
	if manager.called != 0 {
		t.Fatal("read acknowledged gap")
	}
}

func TestHistoryDurabilityAcknowledgementValidatesExactRevisionAndReason(t *testing.T) {
	for _, body := range []string{`{}`, `{"revision":2}`, `{"revision":2,"reason":" "}`, `{"revision":2,"reason":"ok","unknown":true}`, `{"revision":2,"reason":"ok"}{}`, `{"revision":-1,"reason":"ok"}`, fmt.Sprintf(`{"revision":2,"reason":%q}`, strings.Repeat("x", 1025))} {
		manager := &durabilityTestManager{MockAlertManager: new(MockAlertManager)}
		rec := httptest.NewRecorder()
		durabilityTestHandler(manager).HandleAlerts(rec, httptest.NewRequest(http.MethodPost, "/api/alerts/history/durability/acknowledgement", strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest || manager.called != 0 {
			t.Fatalf("invalid acknowledgement admitted: %q code=%d calls=%d", body, rec.Code, manager.called)
		}
	}
	manager := &durabilityTestManager{MockAlertManager: new(MockAlertManager)}
	h := durabilityTestHandler(manager)
	rec := httptest.NewRecorder()
	h.HandleAlerts(rec, httptest.NewRequest(http.MethodPost, "/api/alerts/history/durability/acknowledgement", strings.NewReader(`{"revision":7,"reason":" accepted missing facts "}`)))
	if rec.Code != http.StatusOK || manager.called != 1 || manager.revision != 7 || manager.reason != "accepted missing facts" {
		t.Fatalf("acknowledgement: %d %s %+v", rec.Code, rec.Body, manager)
	}
	manager.err = fmt.Errorf("new gap or retained events")
	rec = httptest.NewRecorder()
	h.HandleAlerts(rec, httptest.NewRequest(http.MethodPost, "/api/alerts/history/durability/acknowledgement", strings.NewReader(`{"revision":7,"reason":"accepted missing facts"}`)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("changed/unsettled gap was acknowledged: %d", rec.Code)
	}
}

func TestHistoryDurabilityEndpointsKeepTenantAndScopeBoundaries(t *testing.T) {
	for _, tc := range []struct{ method, path, wrongScope string }{
		{http.MethodGet, "/api/alerts/history/durability", config.ScopeMonitoringWrite},
		{http.MethodPost, "/api/alerts/history/durability/acknowledgement", config.ScopeMonitoringRead},
	} {
		h := NewAlertHandlers(&monitoring.MultiTenantMonitor{}, new(MockAlertMonitor), nil)
		req := tenantRequest(tc.method, tc.path, "missing-tenant")
		rec := httptest.NewRecorder()
		h.HandleAlerts(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("tenant fallback: %s %d", tc.path, rec.Code)
		}
		req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"revision":1,"reason":"ok"}`))
		req = req.WithContext(internalauth.WithAPIToken(req.Context(), &config.APITokenRecord{Scopes: []string{tc.wrongScope}}))
		rec = httptest.NewRecorder()
		h.HandleAlerts(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("missing scope admitted: %s %d", tc.path, rec.Code)
		}
	}
}

func TestHistoryDurabilityGapDoesNotDisableRetainedEventReads(t *testing.T) {
	manager := &durabilityTestManager{MockAlertManager: new(MockAlertManager), status: eventlog.DurabilityStatus{UnknownGap: true, GapRevision: 3}}
	manager.On("AlertEvents", eventlog.Filter{}).Return([]eventlog.Event{{ID: 1, Type: eventlog.TypeFired, AlertID: "retained"}}, nil)
	rec := httptest.NewRecorder()
	durabilityTestHandler(manager).HandleAlerts(rec, httptest.NewRequest(http.MethodGet, "/api/alerts/events", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("X-Pulse-History-Complete") != "false" || rec.Header().Get("X-Pulse-History-Gap-Revision") != "3" {
		t.Fatalf("retained read: %d %v %s", rec.Code, rec.Header(), rec.Body)
	}
	var events []eventlog.Event
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil || len(events) != 1 {
		t.Fatalf("retained array changed: %s %v", rec.Body, err)
	}
}
