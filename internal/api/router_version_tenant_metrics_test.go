package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func TestGetTenantMonitor_Default(t *testing.T) {
	defaultMonitor, _, _ := newTestMonitor(t)
	router := &Router{monitor: defaultMonitor}

	if got := router.getTenantMonitor(context.Background()); got != defaultMonitor {
		t.Fatalf("expected default monitor to be returned")
	}
}

func TestGetTenantMonitor_WithTenant(t *testing.T) {
	defaultMonitor, _, _ := newTestMonitor(t)
	tenantMonitor, _, _ := newTestMonitor(t)

	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{
		"tenant-1": tenantMonitor,
	})

	router := &Router{monitor: defaultMonitor, mtMonitor: mtm}
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")

	if got := router.getTenantMonitor(ctx); got != tenantMonitor {
		t.Fatalf("expected tenant monitor to be returned")
	}
}

func TestGetTenantMonitor_FallbackOnError(t *testing.T) {
	defaultMonitor, _, _ := newTestMonitor(t)
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	defer mtm.Stop()

	router := &Router{monitor: defaultMonitor, mtMonitor: mtm}
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "../bad")

	if got := router.getTenantMonitor(ctx); got != defaultMonitor {
		t.Fatalf("expected fallback to default monitor")
	}
}

func TestHandleVersion_MethodNotAllowed(t *testing.T) {
	router := &Router{updateManager: updates.NewManager(&config.Config{})}
	req := httptest.NewRequest(http.MethodPost, "/api/version", nil)
	rec := httptest.NewRecorder()

	router.handleVersion(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleVersion_Success(t *testing.T) {
	router := &Router{updateManager: updates.NewManager(&config.Config{})}
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rec := httptest.NewRecorder()

	router.handleVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["version"] == "" {
		t.Fatalf("expected version in response, got %#v", payload)
	}
}

func TestHandleMetricsHistory_MethodNotAllowed(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodPost, "/api/metrics/history", nil)
	rec := httptest.NewRecorder()

	router.handleMetricsHistory(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleMetricsHistory_MissingParams(t *testing.T) {
	monitor, _, _ := newTestMonitor(t)
	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/metrics/history", nil)
	rec := httptest.NewRecorder()

	router.handleMetricsHistory(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleMetricsHistory_LicenseRequired(t *testing.T) {
	monitor, _, _ := newTestMonitor(t)
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("failed to init persistence: %v", err)
	}

	router := &Router{
		monitor:         monitor,
		licenseHandlers: NewLicenseHandlers(mtp),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/history?resourceType=vm&resourceId=vm-1&range=30d", nil)
	rec := httptest.NewRecorder()

	router.handleMetricsHistory(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusPaymentRequired)
	}
}

func TestHandleMetricsHistory_UsesStore(t *testing.T) {
	monitor, _, _ := newTestMonitor(t)
	store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("metrics.NewStore error: %v", err)
	}
	defer store.Close()

	store.WriteBatchSync([]metrics.WriteMetric{{
		ResourceType: "vm",
		ResourceID:   "vm-1",
		MetricType:   "cpu",
		Value:        42.0,
		Timestamp:    time.Now(),
		Tier:         metrics.TierRaw,
	}})

	setUnexportedField(t, monitor, "metricsStore", store)
	router := &Router{monitor: monitor}

	req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/history?resourceType=vm&resourceId=vm-1&metric=cpu&range=1h", nil)
	rec := httptest.NewRecorder()

	router.handleMetricsHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["source"] != "store" {
		t.Fatalf("expected source store, got %#v", payload["source"])
	}
}

func TestHandleMetricsHistory_TenantScopedStoreIsolation(t *testing.T) {
	defaultMonitor, _, _ := newTestMonitor(t)
	tenantMonitor, _, _ := newTestMonitor(t)

	defaultStore, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("default metrics.NewStore error: %v", err)
	}
	defer defaultStore.Close()

	tenantStore, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("tenant metrics.NewStore error: %v", err)
	}
	defer tenantStore.Close()

	now := time.Now()
	defaultStore.WriteBatchSync([]metrics.WriteMetric{{
		ResourceType: "vm",
		ResourceID:   "vm-1",
		MetricType:   "cpu",
		Value:        11.0,
		Timestamp:    now,
		Tier:         metrics.TierRaw,
	}})
	tenantStore.WriteBatchSync([]metrics.WriteMetric{{
		ResourceType: "vm",
		ResourceID:   "vm-1",
		MetricType:   "cpu",
		Value:        84.0,
		Timestamp:    now,
		Tier:         metrics.TierRaw,
	}})

	setUnexportedField(t, defaultMonitor, "metricsStore", defaultStore)
	setUnexportedField(t, tenantMonitor, "metricsStore", tenantStore)

	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{
		"org-a": tenantMonitor,
	})

	router := &Router{
		monitor:   defaultMonitor,
		mtMonitor: mtm,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/history?resourceType=vm&resourceId=vm-1&metric=cpu&range=1h", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-a"))
	rec := httptest.NewRecorder()

	router.handleMetricsHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["source"] != "store" {
		t.Fatalf("expected source store, got %#v", payload["source"])
	}
	if payload["metric"] != "cpu" {
		t.Fatalf("expected metric cpu, got %#v", payload["metric"])
	}

	pointsRaw, ok := payload["points"]
	if !ok {
		t.Fatalf("expected points in response, got %#v", payload)
	}
	points, ok := pointsRaw.([]interface{})
	if !ok || len(points) == 0 {
		t.Fatalf("expected non-empty points series, got %#v", pointsRaw)
	}
	firstPoint, ok := points[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected point object, got %#v", points[0])
	}
	value, ok := firstPoint["value"].(float64)
	if !ok {
		t.Fatalf("expected numeric value in point, got %#v", firstPoint["value"])
	}
	if value != 84.0 {
		t.Fatalf("expected tenant-specific value 84.0, got %v", value)
	}
}

func TestHandleMetricsHistory_UsesStoreAllMetrics(t *testing.T) {
	monitor, _, _ := newTestMonitor(t)
	store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("metrics.NewStore error: %v", err)
	}
	defer store.Close()

	now := time.Now()
	store.WriteBatchSync([]metrics.WriteMetric{
		{
			ResourceType: "vm",
			ResourceID:   "vm-1",
			MetricType:   "cpu",
			Value:        50.0,
			Timestamp:    now,
			Tier:         metrics.TierRaw,
		},
		{
			ResourceType: "vm",
			ResourceID:   "vm-1",
			MetricType:   "memory",
			Value:        70.0,
			Timestamp:    now,
			Tier:         metrics.TierRaw,
		},
	})

	setUnexportedField(t, monitor, "metricsStore", store)
	router := &Router{monitor: monitor}

	req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/history?resourceType=vm&resourceId=vm-1&range=1h", nil)
	rec := httptest.NewRecorder()

	router.handleMetricsHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["source"] != "store" {
		t.Fatalf("expected source store, got %#v", payload["source"])
	}
	metricsMap, ok := payload["metrics"].(map[string]interface{})
	if !ok || metricsMap["cpu"] == nil {
		t.Fatalf("expected cpu metrics in response, got %#v", payload["metrics"])
	}
}

func TestHandleCharts_MethodNotAllowed(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodPost, "/api/charts", nil)
	rec := httptest.NewRecorder()

	router.handleCharts(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
