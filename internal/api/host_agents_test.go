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

func TestHandleLookupByIDSuccess(t *testing.T) {
	t.Parallel()

	hostID := "host-123"
	tokenID := "token-abc"
	lastSeen := time.Now().UTC()

	handler := newHostAgentHandlerForTests(t, models.Host{
		ID:         hostID,
		Hostname:   "host.local",
		DisplayName:"Host Local",
		Status:     "online",
		TokenID:    tokenID,
		LastSeen:   lastSeen,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/agents/host/lookup?id="+hostID, nil)
	attachAPITokenRecord(req, &config.APITokenRecord{ID: tokenID})

	rec := httptest.NewRecorder()
	handler.HandleLookup(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Success bool `json:"success"`
		Host struct {
			ID        string    `json:"id"`
			Hostname  string    `json:"hostname"`
			Status    string    `json:"status"`
			Connected bool      `json:"connected"`
			LastSeen  time.Time `json:"lastSeen"`
		} `json:"host"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true")
	}
	if resp.Host.ID != hostID {
		t.Fatalf("unexpected host id %q", resp.Host.ID)
	}
	if !resp.Host.Connected {
		t.Fatalf("expected connected host")
	}
	if !resp.Host.LastSeen.Equal(lastSeen) {
		t.Fatalf("expected lastSeen %v, got %v", lastSeen, resp.Host.LastSeen)
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

	req := httptest.NewRequest(http.MethodGet, "/api/agents/host/lookup?id="+hostID, nil)
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

	req := httptest.NewRequest(http.MethodGet, "/api/agents/host/lookup?id=missing", nil)
	rec := httptest.NewRecorder()

	handler.HandleLookup(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
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
		monitor: monitor,
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
