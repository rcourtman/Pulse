package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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
	definition := reporting.DescribePerformanceReport()
	if disp := rr.Header().Get("Content-Disposition"); !strings.Contains(disp, definition.SingleFilenamePrefix+"-node-1") {
		t.Fatalf("expected content-disposition to contain canonical filename prefix, got %q", disp)
	}
	if body := rr.Body.String(); body != "report" {
		t.Fatalf("expected report body, got %q", body)
	}

	if engine.lastReq.ResourceType != "node" || engine.lastReq.ResourceID != "node-1" {
		t.Fatalf("unexpected request: %+v", engine.lastReq)
	}
}

func TestReportingHandlers_GenerateReport_TrimsOptionalFields(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reporting?format=pdf&resourceType=node&resourceId=node-1&metricType=+cpu+&title=+Node+report+",
		nil,
	)
	rr := httptest.NewRecorder()

	handler.HandleGenerateReport(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}
	if engine.lastReq.MetricType != "cpu" {
		t.Fatalf("expected trimmed metric type, got %q", engine.lastReq.MetricType)
	}
	if engine.lastReq.Title != "Node report" {
		t.Fatalf("expected trimmed title, got %q", engine.lastReq.Title)
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

func TestNormalizeReportResourceType_RejectsLegacyAliases(t *testing.T) {
	tests := []string{"host", "container"}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			got, err := normalizeReportResourceType(input)
			if err == nil {
				t.Fatalf("expected error for legacy alias %q, got canonical type %q", input, got)
			}
			if !strings.Contains(err.Error(), `unsupported resourceType "`+input+`"`) {
				t.Fatalf("unexpected error for %q: %v", input, err)
			}
		})
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

func TestReportingHandlers_ExportVMInventory_MethodNotAllowed(t *testing.T) {
	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/reports/inventory/vms/export", nil)
	rr := httptest.NewRecorder()

	handler.HandleExportVMInventory(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestReportingHandlers_GetReportingCatalog_MethodNotAllowed(t *testing.T) {
	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/reports/catalog", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetReportingCatalog(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestReportingHandlers_GetReportingCatalog_ReturnsCanonicalDefinition(t *testing.T) {
	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/reports/catalog", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetReportingCatalog(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("expected JSON content-type, got %q", got)
	}

	var payload reporting.ReportingCatalog
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode catalog response: %v", err)
	}
	if payload.ID != "advanced_reporting" {
		t.Fatalf("expected advanced_reporting id, got %q", payload.ID)
	}
	if payload.PerformanceReport.ID != "performance_reports" {
		t.Fatalf("expected performance report definition, got %#v", payload.PerformanceReport)
	}
	if payload.VMInventoryExport.ID != "vm_inventory" {
		t.Fatalf("expected vm inventory definition, got %#v", payload.VMInventoryExport)
	}
}

func TestReportingHandlers_ExportVMInventory_InvalidFormat(t *testing.T) {
	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/reports/inventory/vms/export?format=pdf", nil)
	rr := httptest.NewRecorder()

	handler.HandleExportVMInventory(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode invalid-format response: %v", err)
	}
	if payload.Error != reporting.DescribeVMInventoryExport().InvalidFormatError() {
		t.Fatalf("expected canonical inventory invalid-format error, got %q", payload.Error)
	}
}

func TestReportingHandlers_GenerateMultiReport_UsesCatalogLimit(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "text/csv"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	definition := reporting.DescribePerformanceReport()
	resources := make([]string, 0, definition.MultiResourceMax+1)
	for i := 0; i < definition.MultiResourceMax+1; i++ {
		resources = append(resources, fmt.Sprintf(`{"resourceType":"vm","resourceId":"vm-%d"}`, i))
	}
	body := `{"resources":[` + strings.Join(resources, ",") + `],"format":"csv"}`
	req := httptest.NewRequest(http.MethodPost, "/api/reporting/generate-multi", strings.NewReader(body))
	rr := httptest.NewRecorder()

	handler.HandleGenerateMultiReport(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), fmt.Sprintf("Maximum %d resources allowed", definition.MultiResourceMax)) {
		t.Fatalf("expected canonical multi-resource max in error, got %s", rr.Body.String())
	}
}

func TestReportingHandlers_GenerateMultiReport_TrimsOptionalFieldsAndUsesCanonicalFilename(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "text/csv"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	body := `{"resources":[{"resourceType":"vm","resourceId":"vm-1"}],"format":"csv","metricType":" cpu ","title":" Fleet report "}`
	req := httptest.NewRequest(http.MethodPost, "/api/reporting/generate-multi", strings.NewReader(body))
	rr := httptest.NewRecorder()

	handler.HandleGenerateMultiReport(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}
	definition := reporting.DescribePerformanceReport()
	if disp := rr.Header().Get("Content-Disposition"); !strings.Contains(disp, definition.MultiFilenamePrefix+"-") {
		t.Fatalf("expected canonical multi-report filename prefix, got %q", disp)
	}
	if engine.lastMulti.MetricType != "cpu" {
		t.Fatalf("expected trimmed metric type, got %q", engine.lastMulti.MetricType)
	}
	if engine.lastMulti.Title != "Fleet report" {
		t.Fatalf("expected trimmed title, got %q", engine.lastMulti.Title)
	}
	if len(engine.lastMulti.Resources) != 1 {
		t.Fatalf("expected one resource request, got %+v", engine.lastMulti.Resources)
	}
	if engine.lastMulti.Resources[0].MetricType != "cpu" || engine.lastMulti.Resources[0].Title != "Fleet report" {
		t.Fatalf("expected canonical optional fields to propagate to per-resource requests, got %+v", engine.lastMulti.Resources[0])
	}
}

func TestReportingHandlers_GenerateReport_UsesCatalogDefaultRangeDuration(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	end := time.Date(2026, 3, 25, 15, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reporting?format=pdf&resourceType=node&resourceId=node-1&end="+url.QueryEscape(end.Format(time.RFC3339)),
		nil,
	)
	rr := httptest.NewRecorder()

	handler.HandleGenerateReport(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}
	if got := engine.lastReq.Start; !got.Equal(end.Add(-reporting.DescribePerformanceReport().DefaultRangeDuration())) {
		t.Fatalf("expected canonical default start, got %s", got)
	}
	if !engine.lastReq.End.Equal(end) {
		t.Fatalf("expected canonical end time, got %s", engine.lastReq.End)
	}
}

func TestReportingHandlers_ExportVMInventory_EmptySnapshotStillReturnsCSVHeader(t *testing.T) {
	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/reports/inventory/vms/export", nil)
	rr := httptest.NewRecorder()

	handler.HandleExportVMInventory(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}
	if contentType := rr.Header().Get("Content-Type"); !strings.Contains(contentType, "text/csv") {
		t.Fatalf("expected CSV content type, got %q", contentType)
	}
	if !strings.Contains(rr.Body.String(), "Resource ID,Instance,Node,Pool,VMID,VM Name") {
		t.Fatalf("expected CSV header row, got %q", rr.Body.String())
	}
}

func TestBuildVMInventoryRows_UsesCanonicalFieldsAndDiskFallback(t *testing.T) {
	total := int64(16 * 1024)
	resources := []unifiedresources.Resource{
		{
			ID:     "vm-101",
			Type:   unifiedresources.ResourceTypeVM,
			Name:   "app-vm",
			Status: unifiedresources.StatusWarning,
			Metrics: &unifiedresources.ResourceMetrics{
				Memory: &unifiedresources.MetricValue{Total: &total},
			},
			Proxmox: &unifiedresources.ProxmoxData{
				Instance:         "lab",
				NodeName:         "pve-a",
				Pool:             "prod",
				VMID:             101,
				CPUs:             4,
				DiskStatusReason: "guest agent offline",
				Disks: []unifiedresources.DiskInfo{
					{Device: "scsi0", Total: 100 * 1024, Used: 40 * 1024},
					{Device: "scsi1", Total: 50 * 1024, Used: 10 * 1024},
				},
			},
		},
		{
			ID:   "node-1",
			Type: unifiedresources.ResourceTypeAgent,
			Name: "node-a",
		},
	}

	rows := buildVMInventoryRows(resources)
	if len(rows) != 1 {
		t.Fatalf("expected one VM row, got %d", len(rows))
	}
	row := rows[0]
	if row.ResourceID != "vm-101" || row.Name != "app-vm" || row.Instance != "lab" || row.Node != "pve-a" {
		t.Fatalf("unexpected inventory row identity: %+v", row)
	}
	if row.Pool != "prod" {
		t.Fatalf("expected pool to come from canonical model, got %+v", row)
	}
	if row.CPUCores != 4 || row.MemoryAllocatedBytes != total {
		t.Fatalf("expected CPU and memory totals from canonical model, got %+v", row)
	}
	if row.DiskAllocatedBytes != 150*1024 || row.DiskUsedBytes != 50*1024 {
		t.Fatalf("expected disk totals to fall back to per-disk sums, got %+v", row)
	}
	if row.DiskStatusReason != "guest agent offline" {
		t.Fatalf("expected disk status reason to be preserved, got %+v", row)
	}
}
