package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// newOperatorStateHandlers spins up a ResourceHandlers wired against a
// per-test temp data dir so the SQLite store is ephemeral, mirroring the
// existing resources_test.go fixtures.
func newOperatorStateHandlers(t *testing.T) *ResourceHandlers {
	t.Helper()
	cfg := &config.Config{DataPath: t.TempDir()}
	return NewResourceHandlers(cfg)
}

func TestHandleResourceOperatorState_GetReturns404WhenUnset(t *testing.T) {
	h := newOperatorStateHandlers(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources/vm:101/operator-state", nil)

	h.HandleResourceOperatorState(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on unset state; got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body must be JSON; got %q", rec.Body.String())
	}
	// The handler must surface a stable error code so the frontend can
	// branch on it without string-matching the human message.
	if body["error"] != "operator_state_not_set" {
		t.Errorf("expected error=operator_state_not_set; got %v", body)
	}
}

func TestHandleResourceOperatorState_PutPersistsAndGetReturns200(t *testing.T) {
	h := newOperatorStateHandlers(t)

	start := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 9, 14, 0, 0, 0, time.UTC)
	payload := map[string]any{
		// canonical_id in body is intentionally a different value to
		// confirm the URL path wins over body. Anything in the body must
		// not be able to retarget the write at a different resource.
		"canonicalId":          "vm:999",
		"intentionallyOffline": true,
		"neverAutoRemediate":   true,
		"maintenanceStartAt":   start.Format(time.RFC3339),
		"maintenanceEndAt":     end.Format(time.RFC3339),
		"maintenanceReason":    "Q3 storage upgrade",
		"criticality":          "high",
		"note":                 "do not auto-fix",
	}
	body, _ := json.Marshal(payload)

	putRec := httptest.NewRecorder()
	putReq := httptest.NewRequest(http.MethodPut, "/api/resources/vm:101/operator-state", bytes.NewReader(body))
	h.HandleResourceOperatorState(putRec, putReq)

	if putRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on PUT; got %d body=%s", putRec.Code, putRec.Body.String())
	}
	var persisted resourceOperatorStateAPI
	if err := json.Unmarshal(putRec.Body.Bytes(), &persisted); err != nil {
		t.Fatalf("PUT response must be parseable as resourceOperatorStateAPI; got %q", putRec.Body.String())
	}
	// URL canonical_id wins over body — even though body said vm:999.
	if persisted.CanonicalID != "vm:101" {
		t.Errorf("URL canonical_id must override body; got %q", persisted.CanonicalID)
	}
	if !persisted.IntentionallyOffline || !persisted.NeverAutoRemediate {
		t.Errorf("boolean flags must round-trip on PUT response: %+v", persisted)
	}
	if persisted.Criticality != "high" {
		t.Errorf("criticality must round-trip; got %q", persisted.Criticality)
	}
	// Server-populated attribution: SetAt is populated even when the
	// body omits it. Ignoring client-supplied values keeps the audit
	// trail honest.
	if persisted.SetAt.IsZero() {
		t.Error("server must populate SetAt; got zero")
	}

	// Read-back via GET.
	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/resources/vm:101/operator-state", nil)
	h.HandleResourceOperatorState(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET after PUT; got %d body=%s", getRec.Code, getRec.Body.String())
	}
	var got resourceOperatorStateAPI
	if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("GET response must be parseable; got %q", getRec.Body.String())
	}
	if got.MaintenanceReason != "Q3 storage upgrade" {
		t.Errorf("maintenance reason must round-trip via GET; got %q", got.MaintenanceReason)
	}
	if got.MaintenanceStartAt == nil || !got.MaintenanceStartAt.Equal(start) {
		t.Errorf("maintenance start must round-trip; got %v", got.MaintenanceStartAt)
	}
}

func TestHandleResourceOperatorState_PutRejectsInvalidWith400(t *testing.T) {
	h := newOperatorStateHandlers(t)

	// Unknown criticality value — must be rejected with 400 +
	// operator_state_invalid code, NOT silently coerced or persisted.
	body, _ := json.Marshal(map[string]any{
		"criticality": "very-high",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/resources/vm:101/operator-state", bytes.NewReader(body))
	h.HandleResourceOperatorState(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid state; got %d body=%s", rec.Code, rec.Body.String())
	}
	var errBody map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("body must be JSON error; got %q", rec.Body.String())
	}
	if errBody["error"] != "operator_state_invalid" {
		t.Errorf("expected error=operator_state_invalid; got %v", errBody)
	}

	// And the rejection must not have persisted: GET still returns 404.
	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/resources/vm:101/operator-state", nil)
	h.HandleResourceOperatorState(getRec, getReq)
	if getRec.Code != http.StatusNotFound {
		t.Errorf("rejected PUT must not persist; GET should still 404; got %d", getRec.Code)
	}
}

func TestHandleResourceOperatorState_DeleteIsIdempotent(t *testing.T) {
	h := newOperatorStateHandlers(t)

	// DELETE on a fresh store with no entry: 204 (idempotent).
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/resources/vm:101/operator-state", nil)
	h.HandleResourceOperatorState(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on idempotent DELETE; got %d body=%s", rec.Code, rec.Body.String())
	}

	// Set, then DELETE, then GET should 404.
	setRec := httptest.NewRecorder()
	setReq := httptest.NewRequest(http.MethodPut, "/api/resources/vm:101/operator-state",
		bytes.NewReader([]byte(`{"intentionallyOffline":true}`)))
	h.HandleResourceOperatorState(setRec, setReq)
	if setRec.Code != http.StatusOK {
		t.Fatalf("setup PUT failed: %d %s", setRec.Code, setRec.Body.String())
	}

	delRec := httptest.NewRecorder()
	delReq := httptest.NewRequest(http.MethodDelete, "/api/resources/vm:101/operator-state", nil)
	h.HandleResourceOperatorState(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on DELETE after PUT; got %d", delRec.Code)
	}

	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/resources/vm:101/operator-state", nil)
	h.HandleResourceOperatorState(getRec, getReq)
	if getRec.Code != http.StatusNotFound {
		t.Errorf("expected 404 after DELETE; got %d", getRec.Code)
	}
}

func TestHandleResourceOperatorState_RejectsEmptyResourceID(t *testing.T) {
	h := newOperatorStateHandlers(t)
	rec := httptest.NewRecorder()
	// Path with no resource ID between /resources/ and /operator-state.
	req := httptest.NewRequest(http.MethodGet, "/api/resources//operator-state", nil)
	h.HandleResourceOperatorState(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty resource id; got %d", rec.Code)
	}
}

func TestHandleResourceOperatorState_RejectsUnsupportedMethod(t *testing.T) {
	h := newOperatorStateHandlers(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/resources/vm:101/operator-state", nil)
	h.HandleResourceOperatorState(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 on POST; got %d", rec.Code)
	}
}

func TestExtractOperatorStateResourceID(t *testing.T) {
	cases := []struct {
		name string
		path string
		want string
	}{
		{"canonical resource id", "/api/resources/vm:101/operator-state", "vm:101"},
		{"trailing slash before suffix", "/api/resources/vm:101/operator-state/", "vm:101"},
		{"empty id", "/api/resources//operator-state", ""},
		{"colon-bearing id", "/api/resources/instance:node:200/operator-state", "instance:node:200"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractOperatorStateResourceID(tc.path); got != tc.want {
				t.Errorf("extractOperatorStateResourceID(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}
