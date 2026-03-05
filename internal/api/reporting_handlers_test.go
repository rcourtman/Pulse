package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

type stubReportingEngine struct {
	data        []byte
	contentType string
	err         error
	lastReq     reporting.MetricReportRequest
	lastMulti   reporting.MultiReportRequest
}

func (s *stubReportingEngine) Generate(req reporting.MetricReportRequest) ([]byte, string, error) {
	s.lastReq = req
	if s.err != nil {
		return nil, "", s.err
	}
	return s.data, s.contentType, nil
}

func (s *stubReportingEngine) GenerateMulti(req reporting.MultiReportRequest) ([]byte, string, error) {
	s.lastMulti = req
	if s.err != nil {
		return nil, "", s.err
	}
	return s.data, s.contentType, nil
}

func TestReportingHandlers_MethodNotAllowed(t *testing.T) {
	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/reporting", nil)
	rr := httptest.NewRecorder()

	handler.HandleGenerateReport(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestReportingHandlers_EngineUnavailable(t *testing.T) {
	original := reporting.GetEngine()
	reporting.SetEngine(nil)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/reporting?resourceType=node&resourceId=1", nil)
	rr := httptest.NewRecorder()

	handler.HandleGenerateReport(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["code"] != "engine_unavailable" {
		t.Fatalf("expected engine_unavailable, got %#v", resp["code"])
	}
}

func TestReportingHandlers_InvalidFormatAndParams(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("ok"), contentType: "text/plain"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/reporting?format=txt&resourceType=node&resourceId=1", nil)
	rr := httptest.NewRecorder()
	handler.HandleGenerateReport(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/reporting?format=pdf", nil)
	rr = httptest.NewRecorder()
	handler.HandleGenerateReport(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestReportingHandlers_GenerateReport(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)

	start := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	end := time.Now().UTC().Format(time.RFC3339)
	query := url.Values{
		"format":       []string{"pdf"},
		"resourceType": []string{"node"},
		"resourceId":   []string{"node-1"},
		"metricType":   []string{"cpu"},
		"start":        []string{start},
		"end":          []string{end},
		"title":        []string{"Node report"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/reporting?"+query.Encode(), nil)
	rr := httptest.NewRecorder()
	handler.HandleGenerateReport(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("expected content-type application/pdf, got %q", ct)
	}
	if disp := rr.Header().Get("Content-Disposition"); !strings.Contains(disp, "report-node-1") {
		t.Fatalf("expected content-disposition to contain sanitized filename, got %q", disp)
	}
	if body := rr.Body.String(); body != "report" {
		t.Fatalf("expected report body, got %q", body)
	}

	if engine.lastReq.ResourceType != "node" || engine.lastReq.ResourceID != "node-1" {
		t.Fatalf("unexpected request: %+v", engine.lastReq)
	}
}

func TestReportingHandlers_GenerateReport_RejectsLegacyResourceTypeAlias(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/reporting?format=pdf&resourceType=container&resourceId=ct-200", nil)
	rr := httptest.NewRecorder()

	handler.HandleGenerateReport(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
	if engine.lastReq.ResourceType != "" {
		t.Fatalf("expected engine not to be called for legacy alias, got %+v", engine.lastReq)
	}
}

func TestReportingHandlers_GenerateReport_RejectsUnsupportedResourceType(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/reporting?format=pdf&resourceType=host&resourceId=h-1", nil)
	rr := httptest.NewRecorder()

	handler.HandleGenerateReport(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
	if engine.lastReq.ResourceType != "" {
		t.Fatalf("expected engine not to be called for unsupported type, got %+v", engine.lastReq)
	}
}

func TestReportingHandlers_GenerateReport_AcceptsCanonicalAppContainerType(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reporting?format=pdf&resourceType=app-container&resourceId=docker-1",
		nil,
	)
	rr := httptest.NewRecorder()

	handler.HandleGenerateReport(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}
	if engine.lastReq.ResourceType != "app-container" {
		t.Fatalf("expected canonical resource type app-container, got %q", engine.lastReq.ResourceType)
	}
}

func TestSanitizeFilename(t *testing.T) {
	raw := "\"bad/../name\\\r\n"
	got := sanitizeFilename(raw)
	if strings.ContainsAny(got, "\"\\/\r\n") {
		t.Fatalf("sanitizeFilename did not remove unsafe characters: %q", got)
	}
}
