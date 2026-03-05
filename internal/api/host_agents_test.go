package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestHandleLookupMethodNotAllowed(t *testing.T) {
	t.Parallel()

	handler := newHostAgentHandlerForTests(t)

	// Only GET is allowed
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := httptest.NewRequest(method, "/api/agents/agent/lookup?id=test", nil)
		rec := httptest.NewRecorder()

		handler.HandleLookup(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: expected status %d, got %d", method, http.StatusMethodNotAllowed, rec.Code)
		}
	}
}

func TestHandleLookupMissingParams(t *testing.T) {
	t.Parallel()

	handler := newHostAgentHandlerForTests(t)

	// Neither id nor hostname provided
	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent/lookup", nil)
	rec := httptest.NewRecorder()

	handler.HandleLookup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "missing_lookup_param" {
		t.Errorf("expected error code 'missing_lookup_param', got %q", resp.Code)
	}
}

func TestHandleLookupByIDSuccess(t *testing.T) {
	t.Parallel()

	hostID := "host-123"
	tokenID := "token-abc"
	lastSeen := time.Now().UTC()

	handler := newHostAgentHandlerForTests(t, models.Host{
		ID:          hostID,
		Hostname:    "host.local",
		DisplayName: "Host Local",
		Status:      "online",
		TokenID:     tokenID,
		LastSeen:    lastSeen,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent/lookup?id="+hostID, nil)
	attachAPITokenRecord(req, &config.APITokenRecord{ID: tokenID})

	rec := httptest.NewRecorder()
	handler.HandleLookup(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Success bool `json:"success"`
		Agent   struct {
			ID        string    `json:"id"`
			Hostname  string    `json:"hostname"`
			Status    string    `json:"status"`
			Connected bool      `json:"connected"`
			LastSeen  time.Time `json:"lastSeen"`
		} `json:"agent"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true")
	}
	if resp.Agent.ID != hostID {
		t.Fatalf("unexpected agent id %q", resp.Agent.ID)
	}
	if !resp.Agent.Connected {
		t.Fatalf("expected connected host")
	}
	if !resp.Agent.LastSeen.Equal(lastSeen) {
		t.Fatalf("expected lastSeen %v, got %v", lastSeen, resp.Agent.LastSeen)
	}
}

func TestHandleLookupForbiddenOnTokenMismatch(t *testing.T) {
	t.Parallel()

	hostID := "host-456"

	handler := newHostAgentHandlerForTests(t, models.Host{
		ID:       hostID,
		Hostname: "mismatch.local",
		Status:   "online",
		TokenID:  "token-correct",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent/lookup?id="+hostID, nil)
	attachAPITokenRecord(req, &config.APITokenRecord{ID: "token-wrong"})

	rec := httptest.NewRecorder()
	handler.HandleLookup(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestHandleLookupNotFound(t *testing.T) {
	t.Parallel()

	handler := newHostAgentHandlerForTests(t)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent/lookup?id=missing", nil)
	rec := httptest.NewRecorder()

	handler.HandleLookup(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestHandleLookupByHostname(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		hosts          []models.Host
		queryHostname  string
		expectedHostID string
		expectedStatus int
	}{
		{
			name: "exact hostname match",
			hosts: []models.Host{
				{ID: "host-1", Hostname: "server.example.com", DisplayName: "Server One"},
			},
			queryHostname:  "server.example.com",
			expectedHostID: "host-1",
			expectedStatus: http.StatusOK,
		},
		{
			name: "exact hostname case-insensitive",
			hosts: []models.Host{
				{ID: "host-1", Hostname: "server.example.com", DisplayName: "Server One"},
			},
			queryHostname:  "SERVER.EXAMPLE.COM",
			expectedHostID: "host-1",
			expectedStatus: http.StatusOK,
		},
		{
			name: "display name match",
			hosts: []models.Host{
				{ID: "host-1", Hostname: "srv1", DisplayName: "ProductionServer"},
			},
			queryHostname:  "ProductionServer",
			expectedHostID: "host-1",
			expectedStatus: http.StatusOK,
		},
		{
			name: "display name case-insensitive",
			hosts: []models.Host{
				{ID: "host-1", Hostname: "srv1", DisplayName: "ProductionServer"},
			},
			queryHostname:  "productionserver",
			expectedHostID: "host-1",
			expectedStatus: http.StatusOK,
		},
		{
			name: "short hostname match",
			hosts: []models.Host{
				{ID: "host-1", Hostname: "webserver.corp.example.com", DisplayName: "Web Server"},
			},
			queryHostname:  "webserver",
			expectedHostID: "host-1",
			expectedStatus: http.StatusOK,
		},
		{
			name: "short hostname case-insensitive",
			hosts: []models.Host{
				{ID: "host-1", Hostname: "webserver.corp.example.com", DisplayName: "Web Server"},
			},
			queryHostname:  "WEBSERVER",
			expectedHostID: "host-1",
			expectedStatus: http.StatusOK,
		},
		{
			name: "exact match preferred over short match",
			hosts: []models.Host{
				{ID: "host-1", Hostname: "web.example.com", DisplayName: "Web 1"},
				{ID: "host-2", Hostname: "web", DisplayName: "Web 2"},
			},
			queryHostname:  "web",
			expectedHostID: "host-2",
			expectedStatus: http.StatusOK,
		},
		{
			name: "first match wins in sorted order",
			hosts: []models.Host{
				// After sorting by Hostname: "aaa" < "zzz", so host-2 is checked first
				{ID: "host-1", Hostname: "zzz", DisplayName: "target"},
				{ID: "host-2", Hostname: "aaa", DisplayName: "target"},
			},
			queryHostname:  "target",
			expectedHostID: "host-2",
			expectedStatus: http.StatusOK,
		},
		{
			name: "display name matched before hostname of later host",
			hosts: []models.Host{
				{ID: "host-1", Hostname: "other", DisplayName: "target"},
				{ID: "host-2", Hostname: "target", DisplayName: "Other"},
			},
			queryHostname:  "target",
			expectedHostID: "host-1",
			expectedStatus: http.StatusOK,
		},
		{
			name: "short hostname with FQDN query",
			hosts: []models.Host{
				{ID: "host-1", Hostname: "db.prod.example.com", DisplayName: "Database"},
			},
			queryHostname:  "db.staging.example.com",
			expectedHostID: "host-1",
			expectedStatus: http.StatusOK,
		},
		{
			name: "no match returns not found",
			hosts: []models.Host{
				{ID: "host-1", Hostname: "server.example.com", DisplayName: "Server"},
			},
			queryHostname:  "unknown",
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "hostname without dots matches exactly",
			hosts: []models.Host{
				{ID: "host-1", Hostname: "localhost", DisplayName: "Local"},
			},
			queryHostname:  "localhost",
			expectedHostID: "host-1",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := newHostAgentHandlerForTests(t, tc.hosts...)
			req := httptest.NewRequest(http.MethodGet, "/api/agents/agent/lookup?hostname="+tc.queryHostname, nil)
			rec := httptest.NewRecorder()

			handler.HandleLookup(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d: %s", tc.expectedStatus, rec.Code, rec.Body.String())
			}

			if tc.expectedStatus != http.StatusOK {
				return
			}

			var resp struct {
				Success bool `json:"success"`
				Agent   struct {
					ID string `json:"id"`
				} `json:"agent"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if !resp.Success {
				t.Fatalf("expected success=true")
			}
			if resp.Agent.ID != tc.expectedHostID {
				t.Fatalf("expected agent id %q, got %q", tc.expectedHostID, resp.Agent.ID)
			}
		})
	}
}

func TestHandleConfigMissingConfigScope(t *testing.T) {
	t.Parallel()

	hostID := "host-789"

	handler := newHostAgentHandlerForTests(t, models.Host{
		ID:      hostID,
		TokenID: "token-expected",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent/"+hostID+"/config", nil)
	attachAPITokenRecord(req, &config.APITokenRecord{
		ID:     "token-other",
		Scopes: []string{config.ScopeAgentReport},
	})

	rec := httptest.NewRecorder()
	handler.HandleConfig(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestHandleConfigUsesTokenBinding(t *testing.T) {
	t.Parallel()

	handler := newHostAgentHandlerForTests(t, models.Host{
		ID:      "host-1",
		TokenID: "token-expected",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent/other-host/config", nil)
	attachAPITokenRecord(req, &config.APITokenRecord{
		ID:     "token-expected",
		Scopes: []string{config.ScopeAgentConfigRead},
	})

	rec := httptest.NewRecorder()
	handler.HandleConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		AgentID string `json:"agentId"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.AgentID != "host-1" {
		t.Fatalf("expected agent id %q, got %q", "host-1", resp.AgentID)
	}
}

func TestHandleConfigAllowsHostManageScope(t *testing.T) {
	t.Parallel()

	hostID := "host-910"

	handler := newHostAgentHandlerForTests(t, models.Host{
		ID:      hostID,
		TokenID: "token-expected",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent/"+hostID+"/config", nil)
	attachAPITokenRecord(req, &config.APITokenRecord{
		ID:     "token-other",
		Scopes: []string{config.ScopeAgentManage},
	})

	rec := httptest.NewRecorder()
	handler.HandleConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func newHostAgentHandlerForTests(t *testing.T, hosts ...models.Host) *HostAgentHandlers {
	t.Helper()

	monitor := &monitoring.Monitor{}
	state := models.NewState()
	for _, host := range hosts {
		state.UpsertHost(host)
	}

	setUnexportedField(t, monitor, "state", state)

	return &HostAgentHandlers{
		baseAgentHandlers: baseAgentHandlers{defaultMonitor: monitor},
	}
}

func setUnexportedField(t *testing.T, target interface{}, field string, value interface{}) {
	t.Helper()

	v := reflect.ValueOf(target).Elem()
	f := v.FieldByName(field)
	if !f.IsValid() {
		t.Fatalf("field %q not found", field)
	}

	ptr := unsafe.Pointer(f.UnsafeAddr())
	reflect.NewAt(f.Type(), ptr).Elem().Set(reflect.ValueOf(value))
}
