package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	testifymock "github.com/stretchr/testify/mock"
)

// Mock implementations for testing
type MockAlertManager struct {
	testifymock.Mock
}

func (m *MockAlertManager) GetConfig() alerts.AlertConfig {
	args := m.Called()
	return args.Get(0).(alerts.AlertConfig)
}

func (m *MockAlertManager) UpdateConfig(cfg alerts.AlertConfig) {
	m.Called(cfg)
}

func (m *MockAlertManager) GetActiveAlerts() []alerts.Alert {
	args := m.Called()
	return args.Get(0).([]alerts.Alert)
}

func (m *MockAlertManager) NotifyExistingAlert(id string) {
	m.Called(id)
}

func (m *MockAlertManager) ClearAlertHistory() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockAlertManager) UnacknowledgeAlert(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAlertManager) AcknowledgeAlert(id, user string) error {
	args := m.Called(id, user)
	return args.Error(0)
}

func (m *MockAlertManager) ClearAlert(id string) bool {
	args := m.Called(id)
	return args.Bool(0)
}

func (m *MockAlertManager) GetAlertHistory(limit int) []alerts.Alert {
	args := m.Called(limit)
	return args.Get(0).([]alerts.Alert)
}

func (m *MockAlertManager) GetAlertHistorySince(since time.Time, limit int) []alerts.Alert {
	args := m.Called(since, limit)
	return args.Get(0).([]alerts.Alert)
}

type MockAlertMonitor struct {
	testifymock.Mock
}

func (m *MockAlertMonitor) GetAlertManager() AlertManager {
	args := m.Called()
	return args.Get(0).(AlertManager)
}

func (m *MockAlertMonitor) GetConfigPersistence() ConfigPersistence {
	args := m.Called()
	return args.Get(0).(ConfigPersistence)
}

func (m *MockAlertMonitor) GetIncidentStore() *memory.IncidentStore {
	args := m.Called()
	if store := args.Get(0); store != nil {
		return store.(*memory.IncidentStore)
	}
	return nil
}

func (m *MockAlertMonitor) GetNotificationManager() *notifications.NotificationManager {
	args := m.Called()
	return args.Get(0).(*notifications.NotificationManager)
}

func (m *MockAlertMonitor) SyncAlertState() {
	m.Called()
}

func (m *MockAlertMonitor) GetState() models.StateSnapshot {
	args := m.Called()
	return args.Get(0).(models.StateSnapshot)
}

type MockConfigPersistence struct {
	testifymock.Mock
}

func (m *MockConfigPersistence) SaveAlertConfig(cfg alerts.AlertConfig) error {
	args := m.Called(cfg)
	return args.Error(0)
}

// Tests
func TestGetAlertConfig(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)

	h := NewAlertHandlers(nil, mockMonitor, nil)

	cfg := alerts.AlertConfig{Enabled: true}
	mockManager.On("GetConfig").Return(cfg)

	req := httptest.NewRequest("GET", "/api/alerts/config", nil)
	w := httptest.NewRecorder()

	h.GetAlertConfig(w, req)

	assert.Equal(t, 200, w.Code)
	var resp alerts.AlertConfig
	_ = json.NewDecoder(w.Body).Decode(&resp)
	assert.True(t, resp.Enabled)
}

func TestUpdateAlertConfig(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockPersist := new(MockConfigPersistence)

	mockMonitor.On("GetAlertManager").Return(mockManager)
	mockMonitor.On("GetConfigPersistence").Return(mockPersist)
	mockMonitor.On("GetNotificationManager").Return(&notifications.NotificationManager{})

	h := NewAlertHandlers(nil, mockMonitor, nil)

	cfg := alerts.AlertConfig{Enabled: true}
	mockManager.On("UpdateConfig", testifymock.Anything).Return()
	mockManager.On("GetConfig").Return(cfg)
	mockPersist.On("SaveAlertConfig", testifymock.Anything).Return(nil)

	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest("POST", "/api/alerts/config", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.UpdateAlertConfig(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestGetActiveAlerts(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)

	h := NewAlertHandlers(nil, mockMonitor, nil)

	mockManager.On("GetActiveAlerts").Return([]alerts.Alert{{ID: "a1"}})

	req := httptest.NewRequest("GET", "/api/alerts/active", nil)
	w := httptest.NewRecorder()

	h.GetActiveAlerts(w, req)

	assert.Equal(t, 200, w.Code)
	var resp []alerts.Alert
	_ = json.NewDecoder(w.Body).Decode(&resp)
	assert.Len(t, resp, 1)
	assert.Equal(t, "a1", resp[0].ID)
}

func TestAcknowledgeAlert(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)
	mockMonitor.On("SyncAlertState").Return()

	h := NewAlertHandlers(nil, mockMonitor, nil)

	mockManager.On("AcknowledgeAlert", "a1", testifymock.Anything).Return(nil)

	req := httptest.NewRequest("POST", "/api/alerts/a1/acknowledge", nil)
	req.SetPathValue("id", "a1")
	w := httptest.NewRecorder()

	h.AcknowledgeAlert(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestClearAlert(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)
	mockMonitor.On("SyncAlertState").Return()

	h := NewAlertHandlers(nil, mockMonitor, nil)

	mockManager.On("ClearAlert", "a1").Return(true)

	req := httptest.NewRequest("POST", "/api/alerts/a1/clear", nil)
	req.SetPathValue("id", "a1")
	w := httptest.NewRecorder()

	h.ClearAlert(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestValidateAlertID(t *testing.T) {
	testCases := []struct {
		name  string
		id    string
		valid bool
	}{
		{name: "basic", id: "guest-powered-off-pve-101", valid: true},
		{name: "with spaces", id: "cluster one-node-101-cpu", valid: true},
		{name: "with slash and colon", id: "pve1:qemu/101-cpu", valid: true},
		{name: "empty", id: "", valid: false},
		{name: "too long", id: string(make([]byte, 501)), valid: false},
		{name: "control char", id: "bad\nvalue", valid: false},
		{name: "path traversal", id: "../etc/passwd", valid: false},
		{name: "path traversal middle", id: "pve/../secret", valid: false},
	}

	for _, tc := range testCases {
		if got := validateAlertID(tc.id); got != tc.valid {
			t.Errorf("validateAlertID(%s) = %v, want %v", tc.name, got, tc.valid)
		}
	}
}
func TestAlertHandlers_SetMonitor(t *testing.T) {
	mockMonitor1 := new(MockAlertMonitor)
	mockMonitor2 := new(MockAlertMonitor)
	h := NewAlertHandlers(nil, mockMonitor1, nil)
	assert.Equal(t, mockMonitor1, h.legacyMonitor)
	h.SetMonitor(mockMonitor2)
	assert.Equal(t, mockMonitor2, h.legacyMonitor)
}

func TestGetAlertHistory(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)
	h := NewAlertHandlers(nil, mockMonitor, nil)

	mockManager.On("GetAlertHistory", testifymock.Anything).Return([]alerts.Alert{{ID: "h1"}})

	req := httptest.NewRequest("GET", "/api/alerts/history?limit=10", nil)
	w := httptest.NewRecorder()
	h.GetAlertHistory(w, req)

	assert.Equal(t, 200, w.Code)
	var resp []alerts.Alert
	_ = json.NewDecoder(w.Body).Decode(&resp)
	assert.Len(t, resp, 1)
}

func TestUnacknowledgeAlert(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)
	mockMonitor.On("SyncAlertState").Return()
	h := NewAlertHandlers(nil, mockMonitor, nil)

	mockManager.On("UnacknowledgeAlert", "a1").Return(nil)

	req := httptest.NewRequest("POST", "/api/alerts/a1/unacknowledge", nil)
	req.SetPathValue("id", "a1")
	w := httptest.NewRecorder()
	h.UnacknowledgeAlert(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestClearAlertHistory(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)
	h := NewAlertHandlers(nil, mockMonitor, nil)

	mockManager.On("ClearAlertHistory").Return(nil).Once()

	req := httptest.NewRequest("POST", "/api/alerts/history/clear", nil)
	w := httptest.NewRecorder()
	h.ClearAlertHistory(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestAcknowledgeAlertURL_Success(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)
	mockMonitor.On("SyncAlertState").Return()
	h := NewAlertHandlers(nil, mockMonitor, nil)

	mockManager.On("AcknowledgeAlert", "a/b", testifymock.Anything).Return(nil).Once()

	req := httptest.NewRequest("POST", "/api/alerts/a%2Fb/acknowledge", nil)
	w := httptest.NewRecorder()
	h.AcknowledgeAlert(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestUnacknowledgeAlertURL_Success(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)
	mockMonitor.On("SyncAlertState").Return()
	h := NewAlertHandlers(nil, mockMonitor, nil)

	mockManager.On("UnacknowledgeAlert", "a/b").Return(nil).Once()

	req := httptest.NewRequest("POST", "/api/alerts/a%2Fb/unacknowledge", nil)
	w := httptest.NewRecorder()
	h.UnacknowledgeAlert(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestClearAlertURL_Success(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)
	mockMonitor.On("SyncAlertState").Return()
	h := NewAlertHandlers(nil, mockMonitor, nil)

	mockManager.On("ClearAlert", "a/b").Return(true).Once()

	req := httptest.NewRequest("POST", "/api/alerts/a%2Fb/clear", nil)
	w := httptest.NewRecorder()
	h.ClearAlert(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestSaveAlertIncidentNote(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	mockMonitor.On("GetIncidentStore").Return(mockStore)
	h := NewAlertHandlers(nil, mockMonitor, nil)

	// Create an incident first so RecordNote has something to attach to
	alert := &alerts.Alert{ID: "a1", Type: "test"}
	mockStore.RecordAlertFired(alert)

	body := `{"alert_id": "a1", "note": "test note", "user": "admin"}`
	req := httptest.NewRequest("POST", "/api/alerts/note", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.SaveAlertIncidentNote(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestBulkAcknowledgeAlerts(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)
	mockMonitor.On("SyncAlertState").Return()
	h := NewAlertHandlers(nil, mockMonitor, nil)

	mockManager.On("AcknowledgeAlert", "a1", testifymock.Anything).Return(nil)
	mockManager.On("AcknowledgeAlert", "a2", testifymock.Anything).Return(fmt.Errorf("error"))

	body := `{"alertIds": ["a1", "a2"], "user": "admin"}`
	req := httptest.NewRequest("POST", "/api/alerts/bulk/acknowledge", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.BulkAcknowledgeAlerts(w, req)

	assert.Equal(t, 200, w.Code)
	var resp struct {
		Results []map[string]interface{} `json:"results"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	assert.Len(t, resp.Results, 2)
}

func TestHandleAlerts(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)
	mockMonitor.On("GetConfigPersistence").Return(new(MockConfigPersistence))
	mockMonitor.On("GetNotificationManager").Return(&notifications.NotificationManager{})
	mockMonitor.On("SyncAlertState").Return()
	h := NewAlertHandlers(nil, mockMonitor, nil)

	type route struct {
		method string
		path   string
		setup  func()
	}

	routes := []route{
		{"GET", "/api/alerts/active", func() { mockManager.On("GetActiveAlerts").Return([]alerts.Alert{}).Once() }},
		{"GET", "/api/alerts/history", func() {
			mockManager.On("GetAlertHistory", mock.MatchedBy(func(int) bool { return true })).Return([]alerts.Alert{}).Once()
		}},
		{"GET", "/api/alerts/incidents?alert_id=a1", func() {
			mockMonitor.On("GetIncidentStore").Return(memory.NewIncidentStore(memory.IncidentStoreConfig{})).Once()
		}},
		{"POST", "/api/alerts/incidents/note", func() {
			store := memory.NewIncidentStore(memory.IncidentStoreConfig{})
			store.RecordAlertFired(&alerts.Alert{ID: "a1", Type: "test"})
			mockMonitor.On("GetIncidentStore").Return(store).Once()
		}},
		{"DELETE", "/api/alerts/history", func() { mockManager.On("ClearAlertHistory").Return(nil).Once() }},
		{"POST", "/api/alerts/bulk/acknowledge", func() {
			mockManager.On("AcknowledgeAlert", mock.Anything, mock.Anything).Return(nil)
			mockMonitor.On("SyncAlertState").Return()
		}},
		{"POST", "/api/alerts/bulk/clear", func() {
			mockManager.On("ClearAlert", mock.Anything).Return(true)
			mockMonitor.On("SyncAlertState").Return()
		}},
		{"POST", "/api/alerts/acknowledge", func() {
			mockManager.On("AcknowledgeAlert", "a1", testifymock.Anything).Return(nil).Once()
			mockMonitor.On("SyncAlertState").Return()
		}},
		{"POST", "/api/alerts/unacknowledge", func() {
			mockManager.On("UnacknowledgeAlert", "a1").Return(nil).Once()
			mockMonitor.On("SyncAlertState").Return()
		}},
		{"POST", "/api/alerts/clear", func() {
			mockManager.On("ClearAlert", "a1").Return(true).Once()
			mockMonitor.On("SyncAlertState").Return()
		}},
		{"POST", "/api/alerts/a1/acknowledge", func() {
			mockManager.On("AcknowledgeAlert", "a1", testifymock.Anything).Return(nil).Once()
			mockMonitor.On("SyncAlertState").Return()
		}},
		{"POST", "/api/alerts/a1/unacknowledge", func() {
			mockManager.On("UnacknowledgeAlert", "a1").Return(nil).Once()
			mockMonitor.On("SyncAlertState").Return()
		}},
		{"POST", "/api/alerts/a1/clear", func() {
			mockManager.On("ClearAlert", "a1").Return(true).Once()
			mockMonitor.On("SyncAlertState").Return()
		}},
	}

	for _, rt := range routes {
		t.Run(rt.method+"_"+rt.path, func(t *testing.T) {
			rt.setup()
			var body []byte
			if rt.method == "POST" || rt.method == "PUT" || rt.method == "DELETE" {
				if strings.Contains(rt.path, "bulk") {
					body = []byte(`{"alertIds": ["a1"]}`)
				} else if strings.Contains(rt.path, "note") {
					body = []byte(`{"alert_id": "a1", "note": "test"}`)
				} else {
					body = []byte(`{"id": "a1", "user": "admin"}`)
				}
			}
			req := httptest.NewRequest(rt.method, rt.path, bytes.NewReader(body))
			w := httptest.NewRecorder()
			h.HandleAlerts(w, req)
			assert.Equal(t, 200, w.Code)
		})
	}

	// Test NotFound
	req := httptest.NewRequest("GET", "/api/alerts/unknown", nil)
	w := httptest.NewRecorder()
	h.HandleAlerts(w, req)
	assert.Equal(t, 404, w.Code)
}

func TestBulkClearAlerts(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)
	mockMonitor.On("SyncAlertState").Return()
	h := NewAlertHandlers(nil, mockMonitor, nil)

	mockManager.On("ClearAlert", "a1").Return(true)
	mockManager.On("ClearAlert", "a2").Return(false)

	body := `{"alertIds": ["a1", "a2"]}`
	req := httptest.NewRequest("POST", "/api/alerts/bulk/clear", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.BulkClearAlerts(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestAcknowledgeAlertByBody_Success(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)
	mockMonitor.On("SyncAlertState").Return()
	h := NewAlertHandlers(nil, mockMonitor, nil)

	mockManager.On("AcknowledgeAlert", "a1", testifymock.Anything).Return(nil)

	body := `{"id": "a1", "user": "admin"}`
	req := httptest.NewRequest("POST", "/api/alerts/acknowledge", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.AcknowledgeAlertByBody(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestUnacknowledgeAlertByBody_Success(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)
	mockMonitor.On("SyncAlertState").Return()
	h := NewAlertHandlers(nil, mockMonitor, nil)

	mockManager.On("UnacknowledgeAlert", "a1").Return(nil)

	body := `{"id": "a1"}`
	req := httptest.NewRequest("POST", "/api/alerts/unacknowledge", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.UnacknowledgeAlertByBody(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestClearAlertByBody_Success(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)
	mockMonitor.On("SyncAlertState").Return()
	h := NewAlertHandlers(nil, mockMonitor, nil)

	mockManager.On("ClearAlert", "a1").Return(true)

	body := `{"id": "a1"}`
	req := httptest.NewRequest("POST", "/api/alerts/clear", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ClearAlertByBody(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestAlertHandlers_ErrorCases(t *testing.T) {
	mockMonitor := new(MockAlertMonitor)
	mockManager := new(MockAlertManager)
	mockMonitor.On("GetAlertManager").Return(mockManager)
	h := NewAlertHandlers(nil, mockMonitor, nil)

	t.Run("AcknowledgeAlertByBody_InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/acknowledge", strings.NewReader(`{invalid`))
		w := httptest.NewRecorder()
		h.AcknowledgeAlertByBody(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("AcknowledgeAlertByBody_MissingID", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/acknowledge", strings.NewReader(`{"id": ""}`))
		w := httptest.NewRecorder()
		h.AcknowledgeAlertByBody(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("AcknowledgeAlertByBody_InvalidID", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/acknowledge", strings.NewReader(`{"id": "bad\x01"}`))
		w := httptest.NewRecorder()
		h.AcknowledgeAlertByBody(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("AcknowledgeAlertByBody_ManagerError", func(t *testing.T) {
		mockManager.On("AcknowledgeAlert", "a1", testifymock.Anything).Return(fmt.Errorf("error")).Once()
		req := httptest.NewRequest("POST", "/api/alerts/acknowledge", strings.NewReader(`{"id": "a1", "user": "admin"}`))
		w := httptest.NewRecorder()
		h.AcknowledgeAlertByBody(w, req)
		assert.Equal(t, 404, w.Code)
	})

	t.Run("UnacknowledgeAlertByBody_MissingID", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/unacknowledge", strings.NewReader(`{"id": ""}`))
		w := httptest.NewRecorder()
		h.UnacknowledgeAlertByBody(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("UnacknowledgeAlertByBody_InvalidID", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/unacknowledge", strings.NewReader(`{"id": "bad\x01"}`))
		w := httptest.NewRecorder()
		h.UnacknowledgeAlertByBody(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("ClearAlertByBody_MissingID", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/clear", strings.NewReader(`{"id": ""}`))
		w := httptest.NewRecorder()
		h.ClearAlertByBody(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("ClearAlertByBody_InvalidID", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/clear", strings.NewReader(`{"id": "bad\x01"}`))
		w := httptest.NewRecorder()
		h.ClearAlertByBody(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("ClearAlertByBody_NotFound", func(t *testing.T) {
		mockManager.On("ClearAlert", "unknown").Return(false).Once()
		req := httptest.NewRequest("POST", "/api/alerts/clear", strings.NewReader(`{"id": "unknown"}`))
		w := httptest.NewRecorder()
		h.ClearAlertByBody(w, req)
		assert.Equal(t, 404, w.Code)
	})

	t.Run("BulkAcknowledgeAlerts_InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/bulk/acknowledge", strings.NewReader(`{invalid`))
		w := httptest.NewRecorder()
		h.BulkAcknowledgeAlerts(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("BulkAcknowledgeAlerts_NoIDs", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/bulk/acknowledge", strings.NewReader(`{"alertIds": []}`))
		w := httptest.NewRecorder()
		h.BulkAcknowledgeAlerts(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("UnacknowledgeAlertByBody_InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/unacknowledge", strings.NewReader(`{invalid`))
		w := httptest.NewRecorder()
		h.UnacknowledgeAlertByBody(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("ClearAlertByBody_InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/clear", strings.NewReader(`{invalid`))
		w := httptest.NewRecorder()
		h.ClearAlertByBody(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("SaveAlertIncidentNote_NoStore", func(t *testing.T) {
		mockMonitor2 := new(MockAlertMonitor)
		mockMonitor2.On("GetIncidentStore").Return(nil)
		h2 := NewAlertHandlers(nil, mockMonitor2, nil)
		req := httptest.NewRequest("POST", "/api/alerts/note", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h2.SaveAlertIncidentNote(w, req)
		assert.Equal(t, 503, w.Code)
	})

	t.Run("SaveAlertIncidentNote_InvalidBody", func(t *testing.T) {
		mockStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
		mockMonitor.On("GetIncidentStore").Return(mockStore)
		req := httptest.NewRequest("POST", "/api/alerts/note", strings.NewReader(`{invalid`))
		w := httptest.NewRecorder()
		h.SaveAlertIncidentNote(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("SaveAlertIncidentNote_MissingIDs", func(t *testing.T) {
		mockStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
		mockMonitor.On("GetIncidentStore").Return(mockStore)
		req := httptest.NewRequest("POST", "/api/alerts/note", strings.NewReader(`{"note": "test"}`))
		w := httptest.NewRecorder()
		h.SaveAlertIncidentNote(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("SaveAlertIncidentNote_InvalidAlertID", func(t *testing.T) {
		mockStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
		mockMonitor.On("GetIncidentStore").Return(mockStore)
		req := httptest.NewRequest("POST", "/api/alerts/note", strings.NewReader(`{"alert_id": "bad\x01", "note": "test"}`))
		w := httptest.NewRecorder()
		h.SaveAlertIncidentNote(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("SaveAlertIncidentNote_MissingNote", func(t *testing.T) {
		mockStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
		mockMonitor.On("GetIncidentStore").Return(mockStore)
		req := httptest.NewRequest("POST", "/api/alerts/note", strings.NewReader(`{"alert_id": "a1", "note": ""}`))
		w := httptest.NewRecorder()
		h.SaveAlertIncidentNote(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("SaveAlertIncidentNote_NotFound", func(t *testing.T) {
		mockStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
		mockMonitor.On("GetIncidentStore").Return(mockStore)
		// alert_id non-existent in store
		req := httptest.NewRequest("POST", "/api/alerts/note", strings.NewReader(`{"alert_id": "none", "note": "test"}`))
		w := httptest.NewRecorder()
		h.SaveAlertIncidentNote(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("ClearAlertHistory_Error", func(t *testing.T) {
		mockManager.On("ClearAlertHistory").Return(errors.New("failed")).Once()
		req := httptest.NewRequest("POST", "/api/alerts/history/clear", nil)
		w := httptest.NewRecorder()
		h.ClearAlertHistory(w, req)
		assert.Equal(t, 500, w.Code)
	})

	t.Run("AcknowledgeAlert_InvalidURL", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/a/notack", nil)
		w := httptest.NewRecorder()
		h.AcknowledgeAlert(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("AcknowledgeAlert_InvalidID", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/bad%01/acknowledge", nil)
		w := httptest.NewRecorder()
		h.AcknowledgeAlert(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("AcknowledgeAlert_Error", func(t *testing.T) {
		mockManager.On("AcknowledgeAlert", "a1", testifymock.Anything).Return(errors.New("not found")).Once()
		req := httptest.NewRequest("POST", "/api/alerts/a1/acknowledge", nil)
		w := httptest.NewRecorder()
		h.AcknowledgeAlert(w, req)
		assert.Equal(t, 404, w.Code)
	})

	t.Run("UnacknowledgeAlert_InvalidURL", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/a/notunack", nil)
		w := httptest.NewRecorder()
		h.UnacknowledgeAlert(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("UnacknowledgeAlert_InvalidID", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/bad%01/unacknowledge", nil)
		w := httptest.NewRecorder()
		h.UnacknowledgeAlert(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("UnacknowledgeAlert_Error", func(t *testing.T) {
		mockManager.On("UnacknowledgeAlert", "a1").Return(errors.New("not found")).Once()
		req := httptest.NewRequest("POST", "/api/alerts/a1/unacknowledge", nil)
		w := httptest.NewRecorder()
		h.UnacknowledgeAlert(w, req)
		assert.Equal(t, 404, w.Code)
	})

	t.Run("ClearAlert_InvalidURL", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/a/notclear", nil)
		w := httptest.NewRecorder()
		h.ClearAlert(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("ClearAlert_InvalidID", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/alerts/bad%01/clear", nil)
		w := httptest.NewRecorder()
		h.ClearAlert(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("ClearAlert_NotFound", func(t *testing.T) {
		mockManager.On("ClearAlert", "none").Return(false).Once()
		req := httptest.NewRequest("POST", "/api/alerts/none/clear", nil)
		w := httptest.NewRecorder()
		h.ClearAlert(w, req)
		assert.Equal(t, 404, w.Code)
	})
}
