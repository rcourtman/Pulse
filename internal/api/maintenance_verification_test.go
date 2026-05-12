package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/maintenancesentinel"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func newMaintenanceVerificationFixture(t *testing.T) (*MaintenanceVerificationHandlers, unified.ResourceStore) {
	t.Helper()
	rh := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	store, err := rh.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	providers := maintenancesentinel.Providers{
		Stores: func(orgID string) (unified.ResourceStore, error) {
			return rh.getStore(orgID)
		},
		Now: func() time.Time { return time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC) },
	}
	sentinel, err := maintenancesentinel.New(maintenancesentinel.Config{OrgID: "default"}, providers)
	if err != nil {
		t.Fatalf("new sentinel: %v", err)
	}
	return NewMaintenanceVerificationHandlers(rh, sentinel), store
}

func withDefaultOrg(req *http.Request) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "default"))
}

func seedMaintenanceWindow(t *testing.T, store unified.ResourceStore, canonicalID string, start, end time.Time) {
	t.Helper()
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID:        canonicalID,
		MaintenanceStartAt: &start,
		MaintenanceEndAt:   &end,
		SetAt:              start,
		SetBy:              "operator",
	}); err != nil {
		t.Fatalf("seed operator state: %v", err)
	}
}

func TestMaintenanceVerification_HandleListForResource_Empty(t *testing.T) {
	h, _ := newMaintenanceVerificationFixture(t)
	rec := httptest.NewRecorder()
	req := withDefaultOrg(httptest.NewRequest(http.MethodGet, "/api/resources/vm:101/maintenance-verifications", nil))
	h.HandleListForResource(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data []maintenanceVerificationReportAPI `json:"data"`
		Meta struct {
			ResourceID string `json:"resourceId"`
			Limit      int    `json:"limit"`
			Total      int    `json:"total"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if resp.Meta.ResourceID != "vm:101" || resp.Meta.Total != 0 || resp.Meta.Limit != 25 {
		t.Fatalf("meta = %+v", resp.Meta)
	}
}

func TestMaintenanceVerification_RerunWritesReport(t *testing.T) {
	h, store := newMaintenanceVerificationFixture(t)
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	seedMaintenanceWindow(t, store, "vm:101", now.Add(-time.Hour), now.Add(-15*time.Minute))

	rec := httptest.NewRecorder()
	req := withDefaultOrg(httptest.NewRequest(http.MethodPost, "/api/resources/vm:101/maintenance-verifications/rerun", nil))
	h.HandleRerun(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rerun status = %d body=%s", rec.Code, rec.Body.String())
	}
	var report maintenanceVerificationReportAPI
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if report.ResourceID != "vm:101" {
		t.Fatalf("resource id = %q", report.ResourceID)
	}
	if report.Status == "" {
		t.Fatalf("status must be set")
	}

	// Confirm the report landed in the store.
	reports, err := store.ListLoopReportsForResource(unified.LoopReportTypeMaintenanceVerification, "vm:101", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report after rerun, got %d", len(reports))
	}
}

func TestMaintenanceVerification_RerunWithoutWindowReturns400(t *testing.T) {
	h, _ := newMaintenanceVerificationFixture(t)
	rec := httptest.NewRecorder()
	req := withDefaultOrg(httptest.NewRequest(http.MethodPost, "/api/resources/vm:101/maintenance-verifications/rerun", nil))
	h.HandleRerun(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing window, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body must be JSON: %v", err)
	}
	if body["error"] != "maintenance_window_missing" {
		t.Fatalf("error code = %q want maintenance_window_missing", body["error"])
	}
}

func TestMaintenanceVerification_ReviewMarksReportReviewed(t *testing.T) {
	h, store := newMaintenanceVerificationFixture(t)
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	seedMaintenanceWindow(t, store, "vm:101", now.Add(-time.Hour), now.Add(-15*time.Minute))

	rerunRec := httptest.NewRecorder()
	rerunReq := withDefaultOrg(httptest.NewRequest(http.MethodPost, "/api/resources/vm:101/maintenance-verifications/rerun", nil))
	h.HandleRerun(rerunRec, rerunReq)
	if rerunRec.Code != http.StatusOK {
		t.Fatalf("rerun status = %d body=%s", rerunRec.Code, rerunRec.Body.String())
	}
	var seeded maintenanceVerificationReportAPI
	if err := json.Unmarshal(rerunRec.Body.Bytes(), &seeded); err != nil {
		t.Fatalf("parse rerun: %v", err)
	}

	rec := httptest.NewRecorder()
	req := withDefaultOrg(httptest.NewRequest(
		http.MethodPost,
		"/api/maintenance-verifications/"+seeded.ID+"/review",
		nil,
	))
	h.HandleReview(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("review status = %d body=%s", rec.Code, rec.Body.String())
	}
	var reviewed maintenanceVerificationReportAPI
	if err := json.Unmarshal(rec.Body.Bytes(), &reviewed); err != nil {
		t.Fatalf("parse review response: %v", err)
	}
	if reviewed.UserOutcome != string(unified.LoopReportUserOutcomeReviewed) {
		t.Fatalf("userOutcome = %q want reviewed", reviewed.UserOutcome)
	}
	if reviewed.Status != seeded.Status {
		t.Fatalf("status mutated: was %q, now %q", seeded.Status, reviewed.Status)
	}
}

func TestMaintenanceVerification_ReviewMissingReportReturns404(t *testing.T) {
	h, _ := newMaintenanceVerificationFixture(t)
	rec := httptest.NewRecorder()
	req := withDefaultOrg(httptest.NewRequest(http.MethodPost, "/api/maintenance-verifications/no-such-id/review", nil))
	h.HandleReview(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on missing report, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMaintenanceVerification_RerunReturns503WhenSentinelNil(t *testing.T) {
	rh := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h := NewMaintenanceVerificationHandlers(rh, nil)

	rec := httptest.NewRecorder()
	req := withDefaultOrg(httptest.NewRequest(http.MethodPost, "/api/resources/vm:101/maintenance-verifications/rerun", nil))
	h.HandleRerun(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when sentinel disabled, got %d body=%s", rec.Code, rec.Body.String())
	}
}
