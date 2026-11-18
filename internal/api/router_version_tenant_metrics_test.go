package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/stretchr/testify/require"
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

	if got := router.getTenantMonitor(ctx); got != nil {
		t.Fatalf("expected nil monitor on tenant resolution error")
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

func TestHandleVersion_IncludesReleaseAgentUpdateTarget(t *testing.T) {
	oldBuildVersion := updates.BuildVersion
	updates.BuildVersion = "6.0.0-rc.6"
	t.Cleanup(func() {
		updates.BuildVersion = oldBuildVersion
	})

	router := &Router{updateManager: updates.NewManager(&config.Config{})}
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rec := httptest.NewRecorder()

	router.handleVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload VersionResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.AgentUpdateTargetVersion != "6.0.0-rc.6" {
		t.Fatalf("AgentUpdateTargetVersion = %q, want 6.0.0-rc.6", payload.AgentUpdateTargetVersion)
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

func TestParseMetricsHistoryDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{input: "", want: 24 * time.Hour},
		{input: "  ", want: 24 * time.Hour},
		{input: "30m", want: 30 * time.Minute},
		{input: "24h", want: 24 * time.Hour},
		{input: "7d", want: 7 * 24 * time.Hour},
		{input: "14d", want: 14 * 24 * time.Hour},
		{input: "15d", want: 15 * 24 * time.Hour},
		{input: "336h", want: 14 * 24 * time.Hour},
		{input: " 365D ", want: 365 * 24 * time.Hour},
		{input: "8760h", want: 365 * 24 * time.Hour},
		{input: "8759h59m59.999999999s", want: 365*24*time.Hour - time.Nanosecond},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseMetricsHistoryDuration(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
	for _, input := range []string{"0d", "-1d", "0h", "-1h", "nonsense", "366d", "8760h1ns",
		"213504d", "213505d", "9223372036854775808d", "9223372036854775808ns"} {
		t.Run(input, func(t *testing.T) {
			duration, err := parseMetricsHistoryDuration(input)
			require.ErrorIs(t, err, errInvalidMetricsHistoryRange)
			require.Zero(t, duration)
		})
	}
}

func TestContractMetricsHistoryRangeAdmission(t *testing.T) {
	// With no monitor these requests would fail with 500 if they reached
	// runtime lookup. Reject bad ranges without consulting inventory or storage.
	router := &Router{}
	const path = "/api/metrics-store/history?resourceType=vm&resourceId=vm-1"
	for _, raw := range []string{"366d", "8760h1ns", "213504d", "213505d", "0d", "bad", "%FF", "%ZZ"} {
		t.Run(raw, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path+"&range="+raw, nil)
			rec := httptest.NewRecorder()
			router.handleMetricsHistory(rec, req)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.NotContains(t, rec.Body.String(), raw)
		})
	}

	const token = "synthetic-history-window.12345678"
	cfg := newTestConfigWithTokens(t, newTokenRecord(t, token, []string{config.ScopeMonitoringRead}, nil))
	monitor, _, _ := newTestMonitor(t)
	storeConfig := metrics.DefaultConfig(t.TempDir())
	storeConfig.RetentionDaily = 400 * 24 * time.Hour
	store, err := metrics.NewStore(storeConfig)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	now := time.Now()
	for _, age := range []int{364, 366} {
		store.WriteBatchSync([]metrics.WriteMetric{{
			ResourceType: "vm", ResourceID: "vm-1", MetricType: "cpu", Value: float64(age),
			Timestamp: now.Add(-time.Duration(age) * 24 * time.Hour), Tier: metrics.TierDaily,
		}})
	}
	retained, err := store.Query("vm", "vm-1", "cpu", now.Add(-367*24*time.Hour), now, 0)
	require.NoError(t, err)
	require.Len(t, retained, 2, "the API window must not depend on trimming retained data")
	setUnexportedField(t, monitor, "metricsStore", store)
	registered := newTestRouter(t, cfg, monitor, nil, nil, nil, "test")
	for _, rangeStr := range []string{"365d", "8760h"} {
		t.Run(rangeStr, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path+"&range="+url.QueryEscape(rangeStr)+"&metric=cpu", nil)
			req.Header.Set("X-API-Token", token)
			rec := httptest.NewRecorder()
			registered.Handler().ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var response struct {
				Start, End int64
				Source     string
				Points     []struct{ Value float64 }
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
			require.EqualValues(t, (365 * 24 * time.Hour).Milliseconds(), response.End-response.Start)
			require.Equal(t, "store", response.Source)
			require.Len(t, response.Points, 1)
			require.EqualValues(t, 364, response.Points[0].Value)
		})
	}
}

func setMetricsHistoryLicenseTier(t *testing.T, handlers *LicenseHandlers, tier pkglicensing.Tier) {
	t.Helper()
	svc := handlers.Service(context.Background())
	svc.SetCurrentForTesting(&pkglicensing.License{
		Claims: pkglicensing.Claims{
			LicenseID: "metrics-history-tier-test",
			Email:     "metrics-history@example.test",
			Tier:      tier,
			IssuedAt:  time.Now().Add(-time.Hour).Unix(),
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		},
		ValidatedAt: time.Now(),
	})
}

// Metric history is free on every plan: no tier, including Community with no
// licence at all, is refused a supported range. Storage retention determines
// what observations are available within it.
func TestHandleMetricsHistory_EveryPlanServesEveryRange(t *testing.T) {
	tests := []struct {
		name   string
		tier   pkglicensing.Tier
		range_ string
		want   int
	}{
		{name: "community serves seven days", range_: "7d", want: http.StatusOK},
		{name: "community serves fourteen days", range_: "14d", want: http.StatusOK},
		{name: "community serves thirty days", range_: "30d", want: http.StatusOK},
		{name: "community serves ninety days", range_: "90d", want: http.StatusOK},
		{name: "community serves an equivalent hour range", range_: "2160h", want: http.StatusOK},
		{name: "relay serves ninety days", tier: pkglicensing.TierRelay, range_: "90d", want: http.StatusOK},
		{name: "pro serves past ninety days", tier: pkglicensing.TierPro, range_: "91d", want: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor, _, _ := newTestMonitor(t)
			store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
			if err != nil {
				t.Fatalf("metrics.NewStore error: %v", err)
			}
			defer store.Close()
			setUnexportedField(t, monitor, "metricsStore", store)

			mtp := config.NewMultiTenantPersistence(t.TempDir())
			if _, err := mtp.GetPersistence("default"); err != nil {
				t.Fatalf("failed to init persistence: %v", err)
			}
			handlers := NewLicenseHandlers(mtp, false)
			if tt.tier != "" {
				setMetricsHistoryLicenseTier(t, handlers, tt.tier)
			}

			router := &Router{
				monitor:         monitor,
				licenseHandlers: handlers,
			}

			req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/history?resourceType=vm&resourceId=vm-1&metric=cpu&range="+tt.range_, nil)
			rec := httptest.NewRecorder()

			router.handleMetricsHistory(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.want, rec.Body.String())
			}
		})
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

func TestHandleMetricsHistory_AgentRequestReadsAgentWrites(t *testing.T) {
	monitor, _, _ := newTestMonitor(t)
	store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("metrics.NewStore error: %v", err)
	}
	defer store.Close()

	store.WriteBatchSync([]metrics.WriteMetric{{
		ResourceType: "agent",
		ResourceID:   "agent-1",
		MetricType:   "cpu",
		Value:        37.5,
		Timestamp:    time.Now(),
		Tier:         metrics.TierRaw,
	}})

	setUnexportedField(t, monitor, "metricsStore", store)
	router := &Router{monitor: monitor}

	req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/history?resourceType=agent&resourceId=agent-1&metric=cpu&range=1h", nil)
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
	if payload["resourceType"] != "agent" {
		t.Fatalf("expected canonical resourceType agent, got %#v", payload["resourceType"])
	}
	pointsRaw, ok := payload["points"].([]interface{})
	if !ok || len(pointsRaw) == 0 {
		t.Fatalf("expected non-empty points, got %#v", payload["points"])
	}
}

func TestHandleMetricsHistory_AgentRequestFallsBackToNodeStoreKey(t *testing.T) {
	monitor, _, _ := newTestMonitor(t)
	store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("metrics.NewStore error: %v", err)
	}
	defer store.Close()

	store.WriteBatchSync([]metrics.WriteMetric{{
		ResourceType: "node",
		ResourceID:   "pve-node-1",
		MetricType:   "cpu",
		Value:        51.2,
		Timestamp:    time.Now(),
		Tier:         metrics.TierRaw,
	}})

	setUnexportedField(t, monitor, "metricsStore", store)
	router := &Router{monitor: monitor}

	req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/history?resourceType=agent&resourceId=pve-node-1&metric=cpu&range=1h", nil)
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
	if payload["resourceType"] != "agent" {
		t.Fatalf("expected canonical resourceType agent, got %#v", payload["resourceType"])
	}
	pointsRaw, ok := payload["points"].([]interface{})
	if !ok || len(pointsRaw) == 0 {
		t.Fatalf("expected non-empty points, got %#v", payload["points"])
	}
}

func TestHandleMetricsHistory_HostAliasReadsAgentStoreKeyAndReturnsAgentType(t *testing.T) {
	monitor, _, _ := newTestMonitor(t)
	store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("metrics.NewStore error: %v", err)
	}
	defer store.Close()

	store.WriteBatchSync([]metrics.WriteMetric{{
		ResourceType: "agent",
		ResourceID:   "agent-1",
		MetricType:   "cpu",
		Value:        44.2,
		Timestamp:    time.Now(),
		Tier:         metrics.TierRaw,
	}})

	setUnexportedField(t, monitor, "metricsStore", store)
	router := &Router{monitor: monitor}

	req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/history?resourceType=host&resourceId=agent-1&metric=cpu&range=1h", nil)
	rec := httptest.NewRecorder()

	router.handleMetricsHistory(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if body := rec.Body.String(); !strings.Contains(body, `unsupported resourceType "host"`) {
		t.Fatalf("unexpected response body: %s", body)
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
