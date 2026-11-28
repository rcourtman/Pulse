package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/chartapi"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

type customWindowObserverResolver struct {
	count atomic.Int32
}

func (r *customWindowObserverResolver) RecordSuccessfulExtendedHistoryRead() {
	r.count.Add(1)
}

func (r *customWindowObserverResolver) MonitorForContext(_ context.Context) *monitoring.Monitor {
	return nil
}

func TestHandleMetricsHistory_CustomWindowValidation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	validStart := now.Add(-2 * time.Hour).UnixMilli()
	validEnd := now.Add(-1 * time.Hour).UnixMilli()
	futureEnd := now.Add(10 * time.Minute).UnixMilli()

	// Note: monitor is intentionally nil to prove validation runs before monitor lookup.
	router := &Router{}

	badQueries := []struct {
		name  string
		query string
	}{
		{"start_only", fmt.Sprintf("resourceType=vm&resourceId=vm-1&start=%d", validStart)},
		{"end_only", fmt.Sprintf("resourceType=vm&resourceId=vm-1&end=%d", validEnd)},
		{"combined_with_range", fmt.Sprintf("resourceType=vm&resourceId=vm-1&start=%d&end=%d&range=24h", validStart, validEnd)},
		{"combined_with_empty_range", fmt.Sprintf("resourceType=vm&resourceId=vm-1&start=%d&end=%d&range=", validStart, validEnd)},
		{"duplicate_start", fmt.Sprintf("resourceType=vm&resourceId=vm-1&start=%d&start=%d&end=%d", validStart, validStart, validEnd)},
		{"blank_start", fmt.Sprintf("resourceType=vm&resourceId=vm-1&start=&end=%d", validEnd)},
		{"non_numeric_start", fmt.Sprintf("resourceType=vm&resourceId=vm-1&start=abc&end=%d", validEnd)},
		{"negative_start", fmt.Sprintf("resourceType=vm&resourceId=vm-1&start=-1000&end=%d", validEnd)},
		{"end_before_start", fmt.Sprintf("resourceType=vm&resourceId=vm-1&start=%d&end=%d", validEnd, validStart)},
		{"end_equals_start", fmt.Sprintf("resourceType=vm&resourceId=vm-1&start=%d&end=%d", validStart, validStart)},
		{"shorter_than_1s", fmt.Sprintf("resourceType=vm&resourceId=vm-1&start=%d&end=%d", validStart, validStart+999)},
		{"longer_than_365d", fmt.Sprintf("resourceType=vm&resourceId=vm-1&start=%d&end=%d", now.Add(-366*24*time.Hour).UnixMilli(), now.UnixMilli())},
		{"future_end", fmt.Sprintf("resourceType=vm&resourceId=vm-1&start=%d&end=%d", validStart, futureEnd)},
	}

	for _, tc := range badQueries {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/history?"+tc.query, nil)
			rec := httptest.NewRecorder()
			router.handleMetricsHistory(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHandleMetricsHistory_CustomWindowStoreNoLiveAndExtendedTelemetry(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	cfg := metrics.DefaultConfig(t.TempDir())
	cfg.RetentionRaw = 500 * 24 * time.Hour
	store, err := metrics.NewStore(cfg)
	if err != nil {
		t.Fatalf("metrics.NewStore error: %v", err)
	}
	defer store.Close()
	setUnexportedField(t, monitor, "metricsStore", store)

	// Seed a live VM in state so we can verify custom windows never fabricate source="live".
	state.UpdateVMs([]models.VM{{
		ID:     "vm-custom",
		Name:   "vm-custom",
		Status: "running",
		CPU:    0.42,
	}})

	now := time.Now().UTC().Truncate(time.Second)
	oldTime := now.Add(-10 * 24 * time.Hour)
	ancientTime := now.Add(-400 * 24 * time.Hour)
	recentTime := now.Add(-30 * time.Minute)

	store.Write("vm", "vm-custom", "cpu", 25, oldTime.Add(10*time.Minute))
	store.Write("vm", "vm-custom", "cpu", 35, ancientTime.Add(10*time.Minute))
	store.Write("vm", "vm-custom", "cpu", 45, recentTime.Add(5*time.Minute))
	store.Flush()

	observer := &customWindowObserverResolver{}
	router := &Router{
		monitor:      monitor,
		chartService: chartapi.NewService(observer),
	}

	// 1. Empty custom window MUST return 200 with range="custom", exact start/end, empty metrics, and NOT source="live".
	emptyStart := now.Add(-12 * 24 * time.Hour).UnixMilli()
	emptyEnd := now.Add(-11 * 24 * time.Hour).UnixMilli()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/metrics-store/history?resourceType=vm&resourceId=vm-custom&start=%d&end=%d", emptyStart, emptyEnd), nil)
	rec := httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("empty status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var emptyBody struct {
		Range   string                      `json:"range"`
		Start   int64                       `json:"start"`
		End     int64                       `json:"end"`
		Source  string                      `json:"source"`
		Metrics map[string][]map[string]any `json:"metrics"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &emptyBody); err != nil {
		t.Fatalf("decode empty body: %v", err)
	}
	if emptyBody.Range != "custom" || emptyBody.Start != emptyStart || emptyBody.End != emptyEnd {
		t.Fatalf("unexpected window echo: %+v", emptyBody)
	}
	if emptyBody.Source == "live" {
		t.Fatalf("custom window must never emit source=live")
	}
	if len(emptyBody.Metrics) != 0 {
		t.Fatalf("expected empty metrics, got %+v", emptyBody.Metrics)
	}
	if got := observer.count.Load(); got != 0 {
		t.Fatalf("empty old custom read must not increment extended observer, got %d", got)
	}

	// 2. Short 2h custom window 10 days ago with real data MUST count exactly once.
	oldStart := oldTime.UnixMilli()
	oldEnd := oldTime.Add(2 * time.Hour).UnixMilli()
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/metrics-store/history?resourceType=vm&resourceId=vm-custom&start=%d&end=%d&maxPoints=240", oldStart, oldEnd), nil)
	rec = httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("old custom status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := observer.count.Load(); got != 1 {
		t.Fatalf("non-empty old custom read observer count = %d, want 1", got)
	}

	// 3. Narrow 2h window 400 days ago (older than 365d, but span <= 365d) is admitted when retained.
	ancStart := ancientTime.UnixMilli()
	ancEnd := ancientTime.Add(2 * time.Hour).UnixMilli()
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/metrics-store/history?resourceType=vm&resourceId=vm-custom&metric=cpu&start=%d&end=%d", ancStart, ancEnd), nil)
	rec = httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ancient narrow status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var ancBody struct {
		Range  string           `json:"range"`
		Start  int64            `json:"start"`
		End    int64            `json:"end"`
		Points []map[string]any `json:"points"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ancBody); err != nil {
		t.Fatal(err)
	}
	if ancBody.Range != "custom" || ancBody.Start != ancStart || ancBody.End != ancEnd || len(ancBody.Points) != 1 {
		t.Fatalf("unexpected ancient narrow reply: %+v", ancBody)
	}
	if got := observer.count.Load(); got != 2 {
		t.Fatalf("observer count = %d, want 2", got)
	}

	// 4. Recent 15m custom window inside 7 days must not increment extended observer.
	recStart := recentTime.UnixMilli()
	recEnd := recentTime.Add(15 * time.Minute).UnixMilli()
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/metrics-store/history?resourceType=vm&resourceId=vm-custom&start=%d&end=%d", recStart, recEnd), nil)
	rec = httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("recent status = %d, want 200", rec.Code)
	}
	if got := observer.count.Load(); got != 2 {
		t.Fatalf("recent custom read incremented observer: got %d, want 2", got)
	}
}

func TestHandleMetricsHistory_CustomWindowRingInclusiveAndMock(t *testing.T) {
	monitor, _, _ := newTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	winStart := now.Add(-10 * time.Minute)
	winEnd := now.Add(-5 * time.Minute)

	// Store nil -> falls back to in-memory ring; verify endpoints are inclusive and out-of-window points excluded.
	mh := monitoring.NewMetricsHistory(100, time.Hour)
	mh.AddGuestMetric("vm-ring", "cpu", 10, winStart.Add(-time.Second))
	mh.AddGuestMetric("vm-ring", "cpu", 20, winStart)
	mh.AddGuestMetric("vm-ring", "cpu", 30, winEnd)
	mh.AddGuestMetric("vm-ring", "cpu", 40, winEnd.Add(time.Second))
	setUnexportedField(t, monitor, "metricsHistory", mh)

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/metrics-store/history?resourceType=vm&resourceId=vm-ring&start=%d&end=%d", winStart.UnixMilli(), winEnd.UnixMilli()), nil)
	rec := httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ring status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var ringBody struct {
		Range   string `json:"range"`
		Source  string `json:"source"`
		Metrics map[string][]struct {
			Timestamp int64   `json:"timestamp"`
			Value     float64 `json:"value"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ringBody); err != nil {
		t.Fatal(err)
	}
	if ringBody.Range != "custom" || ringBody.Source != "memory" {
		t.Fatalf("unexpected ring metadata: %+v", ringBody)
	}
	cpuPts := ringBody.Metrics["cpu"]
	if len(cpuPts) != 2 || cpuPts[0].Timestamp != winStart.UnixMilli() || cpuPts[1].Timestamp != winEnd.UnixMilli() {
		t.Fatalf("ring inclusive boundaries failed: got %+v", cpuPts)
	}

	// Mock mode all-metrics custom window must stay strictly inside [start, end].
	prevMock := mock.IsMockEnabled()
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(prevMock) })

	mockStart := now.Add(-6 * time.Hour).UnixMilli()
	mockEnd := now.Add(-5 * time.Hour).UnixMilli()
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/metrics-store/history?resourceType=vm&resourceId=mock-vm-1&start=%d&end=%d&maxPoints=240", mockStart, mockEnd), nil)
	rec = httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("mock custom status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var mockBody struct {
		Range   string `json:"range"`
		Start   int64  `json:"start"`
		End     int64  `json:"end"`
		Source  string `json:"source"`
		Metrics map[string][]struct {
			Timestamp int64 `json:"timestamp"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &mockBody); err != nil {
		t.Fatal(err)
	}
	if mockBody.Range != "custom" || mockBody.Source != "mock_synthetic" || len(mockBody.Metrics["cpu"]) < 2 {
		t.Fatalf("unexpected mock custom response: %+v", mockBody)
	}
	for metric, pts := range mockBody.Metrics {
		for _, pt := range pts {
			if pt.Timestamp < mockStart || pt.Timestamp > mockEnd {
				t.Fatalf("mock metric %s timestamp %d outside [%d, %d]", metric, pt.Timestamp, mockStart, mockEnd)
			}
		}
	}
}

type stubNativeDiskTempProvider struct {
	series map[string][]monitoring.MetricPoint
}

func (p stubNativeDiskTempProvider) SupplementalRecords(_ *monitoring.Monitor, _ string) []unifiedresources.IngestRecord {
	return nil
}

func (p stubNativeDiskTempProvider) PhysicalDiskTemperatureHistory(_ *monitoring.Monitor, _ string, _ time.Duration) map[string][]monitoring.MetricPoint {
	return p.series
}

func TestHandleMetricsHistory_CustomWindowClipsBeforeSourceSelection(t *testing.T) {
	monitor, _, _ := newTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	winStart := now.Add(-20 * time.Minute)
	winEnd := now.Add(-10 * time.Minute)

	// In-memory ring has dense coverage inside [winStart, winEnd].
	mh := monitoring.NewMetricsHistory(100, time.Hour)
	mh.AddDiskMetric("disk-clip", "smart_temp", 38, winStart.Add(time.Minute))
	mh.AddDiskMetric("disk-clip", "smart_temp", 39, winStart.Add(5*time.Minute))
	mh.AddDiskMetric("disk-clip", "smart_temp", 40, winStart.Add(9*time.Minute))
	setUnexportedField(t, monitor, "metricsHistory", mh)

	// Native provider has a much wider span OVERALL (6 hours), but only ONE point inside [winStart, winEnd].
	// Clipping before coverage comparison must pick the clipped ring ("memory"), NOT native.
	nativeWideOutside := stubNativeDiskTempProvider{
		series: map[string][]monitoring.MetricPoint{
			"disk-clip": {
				{Timestamp: now.Add(-6 * time.Hour), Value: 30},
				{Timestamp: winStart.Add(2 * time.Minute), Value: 77},
				{Timestamp: now.Add(-1 * time.Minute), Value: 31},
			},
		},
	}
	setUnexportedField(t, monitor, "supplementalProviders", map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider{
		unifiedresources.SourceTrueNAS: nativeWideOutside,
	})

	pts, origin := monitor.GetDiskMetricsForWindowWithSource("disk-clip", "smart_temp", winStart, winEnd)
	if origin != "memory" || len(pts) != 3 {
		t.Fatalf("expected clipped ring (memory, 3 pts) to beat native with wider out-of-window span; got origin=%q pts=%+v", origin, pts)
	}

	// Now reverse: native has dense points inside [winStart, winEnd] while ring has only 1 point.
	mhSparse := monitoring.NewMetricsHistory(100, time.Hour)
	mhSparse.AddDiskMetric("disk-clip", "smart_temp", 38, winStart.Add(5*time.Minute))
	setUnexportedField(t, monitor, "metricsHistory", mhSparse)

	nativeDenseInside := stubNativeDiskTempProvider{
		series: map[string][]monitoring.MetricPoint{
			"disk-clip": {
				{Timestamp: now.Add(-6 * time.Hour), Value: 30},
				{Timestamp: winStart.Add(time.Minute), Value: 41},
				{Timestamp: winStart.Add(5 * time.Minute), Value: 42},
				{Timestamp: winStart.Add(9 * time.Minute), Value: 43},
				{Timestamp: now.Add(-1 * time.Minute), Value: 31},
			},
		},
	}
	setUnexportedField(t, monitor, "supplementalProviders", map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider{
		unifiedresources.SourceTrueNAS: nativeDenseInside,
	})
	pts, origin = monitor.GetDiskMetricsForWindowWithSource("disk-clip", "smart_temp", winStart, winEnd)
	if origin != "native" || len(pts) != 3 {
		t.Fatalf("expected clipped native (native, 3 pts) to beat sparse ring; got origin=%q pts=%+v", origin, pts)
	}
	for _, pt := range pts {
		if pt.Timestamp.Before(winStart) || pt.Timestamp.After(winEnd) {
			t.Fatalf("native point %v leaked outside [%v, %v]", pt.Timestamp, winStart, winEnd)
		}
	}

	// Competing native providers: one straddles the window (both points outside),
	// another has two points inside. Clipping each provider before comparing must
	// select the in-window provider instead of letting the straddler win and clip to empty.
	setUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(100, time.Hour))
	straddlingProvider := stubNativeDiskTempProvider{
		series: map[string][]monitoring.MetricPoint{
			"disk-compete": {
				{Timestamp: winStart.Add(-2 * time.Hour), Value: 20},
				{Timestamp: winEnd.Add(2 * time.Hour), Value: 21},
			},
		},
	}
	inWindowProvider := stubNativeDiskTempProvider{
		series: map[string][]monitoring.MetricPoint{
			"disk-compete": {
				{Timestamp: winStart.Add(2 * time.Minute), Value: 44},
				{Timestamp: winStart.Add(8 * time.Minute), Value: 46},
			},
		},
	}
	setUnexportedField(t, monitor, "supplementalProviders", map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider{
		unifiedresources.SourceProxmox: straddlingProvider,
		unifiedresources.SourceTrueNAS: inWindowProvider,
	})
	pts, origin = monitor.GetDiskMetricsForWindowWithSource("disk-compete", "smart_temp", winStart, winEnd)
	if origin != "native" || len(pts) != 2 || pts[0].Value != 44 {
		t.Fatalf("competing native providers failed after clipping: origin=%q pts=%+v", origin, pts)
	}

	// Single-metric disk request preserves source="native", matching all-metrics metricSources.
	router := &Router{monitor: monitor}
	reqSingle := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/metrics-store/history?resourceType=disk&resourceId=disk-compete&metric=smart_temp&start=%d&end=%d", winStart.UnixMilli(), winEnd.UnixMilli()), nil)
	recSingle := httptest.NewRecorder()
	router.handleMetricsHistory(recSingle, reqSingle)
	if recSingle.Code != http.StatusOK {
		t.Fatalf("single disk status = %d, want 200; body=%s", recSingle.Code, recSingle.Body.String())
	}
	var singleDisk struct {
		Source string `json:"source"`
	}
	if err := json.Unmarshal(recSingle.Body.Bytes(), &singleDisk); err != nil {
		t.Fatal(err)
	}
	if singleDisk.Source != "native" {
		t.Fatalf("single disk source = %q, want native", singleDisk.Source)
	}
}

type toggleMockOnWriteRecorder struct {
	*httptest.ResponseRecorder
}

func (r *toggleMockOnWriteRecorder) Write(p []byte) (int, error) {
	mock.SetEnabled(false)
	return r.ResponseRecorder.Write(p)
}

func TestHandleMetricsHistory_CustomWindowParityBudgetUnknownMockAndSyntheticObserver(t *testing.T) {
	monitor, _, _ := newTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	winStart := now.Add(-20 * time.Minute)
	winEnd := now.Add(-10 * time.Minute)

	// 1. Agent single-metric vs all-metrics parity when guest ring only has
	// out-of-window samples while node ring has an in-window CPU reading.
	mh := monitoring.NewMetricsHistory(2000, 4*time.Hour)
	mh.AddGuestMetric("agent:agent-1", "cpu", 11, winStart.Add(-2*time.Minute))
	mh.AddNodeMetric("agent-1", "cpu", 88, winStart.Add(5*time.Minute))
	setUnexportedField(t, monitor, "metricsHistory", mh)

	router := &Router{monitor: monitor}
	singleReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/metrics-store/history?resourceType=agent&resourceId=agent-1&metric=cpu&start=%d&end=%d", winStart.UnixMilli(), winEnd.UnixMilli()), nil)
	singleRec := httptest.NewRecorder()
	router.handleMetricsHistory(singleRec, singleReq)
	if singleRec.Code != http.StatusOK {
		t.Fatalf("agent single status = %d, want 200; body=%s", singleRec.Code, singleRec.Body.String())
	}
	allReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/metrics-store/history?resourceType=agent&resourceId=agent-1&start=%d&end=%d", winStart.UnixMilli(), winEnd.UnixMilli()), nil)
	allRec := httptest.NewRecorder()
	router.handleMetricsHistory(allRec, allReq)
	if allRec.Code != http.StatusOK {
		t.Fatalf("agent all status = %d, want 200 (parity with single); body=%s", allRec.Code, allRec.Body.String())
	}
	var allBody struct {
		Metrics map[string][]struct {
			Value float64 `json:"value"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(allRec.Body.Bytes(), &allBody); err != nil {
		t.Fatal(err)
	}
	if len(allBody.Metrics["cpu"]) != 1 || allBody.Metrics["cpu"][0].Value != 88 {
		t.Fatalf("agent all missing node fallback cpu=88: %+v", allBody.Metrics)
	}

	// 2. Custom window 2h + 1s with maxPoints=240 honors the 240-point budget on dense ring data.
	longStart := now.Add(-2*time.Hour - time.Minute)
	longEnd := longStart.Add(2*time.Hour + time.Second)
	for offset := time.Duration(0); offset <= 2*time.Hour+time.Second; offset += 10 * time.Second {
		val := 10.0
		if offset == 30*time.Minute {
			val = 99.0
		}
		mh.AddGuestMetric("vm-budget", "cpu", val, longStart.Add(offset))
	}
	budgetReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/metrics-store/history?resourceType=vm&resourceId=vm-budget&metric=cpu&start=%d&end=%d&maxPoints=240", longStart.UnixMilli(), longEnd.UnixMilli()), nil)
	budgetRec := httptest.NewRecorder()
	router.handleMetricsHistory(budgetRec, budgetReq)
	if budgetRec.Code != http.StatusOK {
		t.Fatalf("budget status = %d, want 200; body=%s", budgetRec.Code, budgetRec.Body.String())
	}
	var budgetBody struct {
		Points []struct {
			Timestamp int64   `json:"timestamp"`
			Max       float64 `json:"max"`
		} `json:"points"`
	}
	if err := json.Unmarshal(budgetRec.Body.Bytes(), &budgetBody); err != nil {
		t.Fatal(err)
	}
	if len(budgetBody.Points) > 240 {
		t.Fatalf("2h+1s custom window ignored maxPoints=240 budget: got %d points", len(budgetBody.Points))
	}
	sawPeak := false
	for _, pt := range budgetBody.Points {
		if pt.Max == 99.0 {
			sawPeak = true
		}
		if pt.Timestamp < longStart.UnixMilli() || pt.Timestamp > longEnd.UnixMilli() {
			t.Fatalf("bucketed point %d outside window", pt.Timestamp)
		}
	}
	if !sawPeak {
		t.Fatalf("bucketing lost peak max=99.0")
	}

	// 2b. Exact 2h ring series (10s) and 240s store series (1s) with inclusive endpoints must stay <= 240.
	exact2hStart := now.Add(-2 * time.Hour)
	exact2hEnd := now
	for offset := time.Duration(0); offset <= 2*time.Hour; offset += 10 * time.Second {
		mh.AddGuestMetric("vm-budget-2h", "cpu", 25, exact2hStart.Add(offset))
	}
	req2h := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/metrics-store/history?resourceType=vm&resourceId=vm-budget-2h&metric=cpu&start=%d&end=%d&maxPoints=240", exact2hStart.UnixMilli(), exact2hEnd.UnixMilli()), nil)
	rec2h := httptest.NewRecorder()
	router.handleMetricsHistory(rec2h, req2h)
	if rec2h.Code != http.StatusOK {
		t.Fatalf("2h budget status = %d", rec2h.Code)
	}
	var body2h struct {
		Points []struct {
			Timestamp int64 `json:"timestamp"`
		} `json:"points"`
	}
	if err := json.Unmarshal(rec2h.Body.Bytes(), &body2h); err != nil {
		t.Fatal(err)
	}
	if len(body2h.Points) > 240 {
		t.Fatalf("exact 2h ring returned %d points, want <= 240", len(body2h.Points))
	}

	// 2c. Store partial-agent parity (CPU in store + memory in node ring) and 240s store 1s series <= 240.
	storeCfg := metrics.DefaultConfig(t.TempDir())
	storeCfg.RetentionRaw = 24 * time.Hour
	store, err := metrics.NewStore(storeCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	setUnexportedField(t, monitor, "metricsStore", store)
	store.Write("agent", "agent-partial", "cpu", 25, winStart.Add(2*time.Minute))
	s240Start := now.Add(-240 * time.Second)
	for sec := 0; sec <= 240; sec++ {
		store.Write("vm", "vm-store-240", "cpu", float64(sec%100), s240Start.Add(time.Duration(sec)*time.Second))
	}
	store.Flush()
	mh.AddNodeMetric("agent-partial", "memory", 88, winStart.Add(3*time.Minute))
	mh.AddGuestMetric("agent:agent-partial", "netin", 75, winStart.Add(4*time.Minute))
	mh.AddGuestMetric("agent:agent-partial", "netout", 45, winStart.Add(4*time.Minute))

	partialAllReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/metrics-store/history?resourceType=agent&resourceId=agent-partial&start=%d&end=%d", winStart.UnixMilli(), winEnd.UnixMilli()), nil)
	partialAllRec := httptest.NewRecorder()
	router.handleMetricsHistory(partialAllRec, partialAllReq)
	if partialAllRec.Code != http.StatusOK {
		t.Fatalf("partial agent all status = %d, body=%s", partialAllRec.Code, partialAllRec.Body.String())
	}
	var partialAllBody struct {
		Metrics map[string][]struct {
			Value float64 `json:"value"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(partialAllRec.Body.Bytes(), &partialAllBody); err != nil {
		t.Fatal(err)
	}
	if len(partialAllBody.Metrics["cpu"]) != 1 ||
		len(partialAllBody.Metrics["memory"]) != 1 || partialAllBody.Metrics["memory"][0].Value != 88 ||
		len(partialAllBody.Metrics["netin"]) != 1 || partialAllBody.Metrics["netin"][0].Value != 75 ||
		len(partialAllBody.Metrics["netout"]) != 1 || partialAllBody.Metrics["netout"][0].Value != 45 {
		t.Fatalf("partial agent all missing cpu/memory/netin/netout parity: %+v", partialAllBody.Metrics)
	}

	// 2d. 2h SQLite downsampling must preserve exact sample-weighted average (0, 10, 20, 90 -> 30, NOT 50).
	// Because 7200/30 + 1 = 241 > 240, a 2h window with maxPoints=240 uses 31s SQLite buckets.
	// Pick a 31s bucket strictly inside [wStart, wEnd] that straddles a 30s boundary so a buggy
	// 30s-then-31s unweighted re-aggregation would average [0,10,20]->10 and [90]->90 into 50.
	wEnd := now.Truncate(time.Minute)
	wStart := wEnd.Add(-2 * time.Hour)
	bucketStart := ((wStart.Unix()+3600)/31 + 1) * 31
	for bucketStart%30 != 15 && bucketStart+30 < wEnd.Unix() {
		bucketStart += 31
	}
	if bucketStart%30 != 15 || bucketStart+30 >= wEnd.Unix() {
		t.Fatalf("failed to find interior 31s bucket straddling 30s boundary in [%d, %d]", wStart.Unix(), wEnd.Unix())
	}
	boundary30 := bucketStart + 15
	store.Write("vm", "vm-weighted", "cpu", 50, wStart.Add(time.Minute))
	store.Write("vm", "vm-weighted", "cpu", 0, time.Unix(boundary30-3, 0))
	store.Write("vm", "vm-weighted", "cpu", 10, time.Unix(boundary30-2, 0))
	store.Write("vm", "vm-weighted", "cpu", 20, time.Unix(boundary30-1, 0))
	store.Write("vm", "vm-weighted", "cpu", 90, time.Unix(boundary30, 0))
	store.Flush()

	wReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/metrics-store/history?resourceType=vm&resourceId=vm-weighted&metric=cpu&start=%d&end=%d&maxPoints=240", wStart.UnixMilli(), wEnd.UnixMilli()), nil)
	wRec := httptest.NewRecorder()
	router.handleMetricsHistory(wRec, wReq)
	if wRec.Code != http.StatusOK {
		t.Fatalf("weighted store status = %d, body=%s", wRec.Code, wRec.Body.String())
	}
	var wBody struct {
		Points []struct {
			Value float64 `json:"value"`
			Min   float64 `json:"min"`
			Max   float64 `json:"max"`
		} `json:"points"`
	}
	if err := json.Unmarshal(wRec.Body.Bytes(), &wBody); err != nil {
		t.Fatal(err)
	}
	if len(wBody.Points) < 2 {
		t.Fatalf("expected >= 2 weighted points, got %+v", wBody.Points)
	}
	lastPt := wBody.Points[len(wBody.Points)-1]
	if lastPt.Value != 30 || lastPt.Min != 0 || lastPt.Max != 90 {
		t.Fatalf("weighted SQLite bucket corrupted average: got %+v, want avg=30 min=0 max=90", lastPt)
	}

	req240 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/metrics-store/history?resourceType=vm&resourceId=vm-store-240&metric=cpu&start=%d&end=%d&maxPoints=240", s240Start.UnixMilli(), now.UnixMilli()), nil)
	rec240 := httptest.NewRecorder()
	router.handleMetricsHistory(rec240, req240)
	if rec240.Code != http.StatusOK {
		t.Fatalf("240s store status = %d", rec240.Code)
	}
	var body240 struct {
		Points []struct {
			Timestamp int64 `json:"timestamp"`
		} `json:"points"`
	}
	if err := json.Unmarshal(rec240.Body.Bytes(), &body240); err != nil {
		t.Fatal(err)
	}
	if len(body240.Points) > 240 {
		t.Fatalf("240s store series returned %d points, want <= 240", len(body240.Points))
	}

	// 3. Unknown disk metric in mock mode must not synthesize fake data.
	setUnexportedField(t, monitor, "metricsStore", (*metrics.Store)(nil))
	prevMock := mock.IsMockEnabled()
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(prevMock) })

	unknownReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/metrics-store/history?resourceType=disk&resourceId=disk-1&metric=unknown_metric&start=%d&end=%d", winStart.UnixMilli(), winEnd.UnixMilli()), nil)
	unknownRec := httptest.NewRecorder()
	router.handleMetricsHistory(unknownRec, unknownReq)
	if unknownRec.Code == http.StatusOK {
		t.Fatalf("unknown mock disk metric unexpectedly returned 200 with body=%s", unknownRec.Body.String())
	}

	// 4. Extended-history observer: synthetic payload never counts even when mock mode is OFF before write;
	// genuine store payload with multiple old points counts exactly once.
	observer := &customWindowObserverResolver{}
	svc := chartapi.NewService(observer)
	oldTs := now.Add(-10 * 24 * time.Hour)
	syntheticPayload := map[string]any{
		"resourceType": "vm",
		"resourceId":   "mock-vm-1",
		"range":        "custom",
		"start":        oldTs.UnixMilli(),
		"end":          oldTs.Add(time.Hour).UnixMilli(),
		"source":       "mock_synthetic",
		"metrics": map[string]any{
			"cpu": []map[string]any{
				{"timestamp": oldTs.Add(5 * time.Minute).UnixMilli(), "value": 10, "min": 10, "max": 10},
				{"timestamp": oldTs.Add(10 * time.Minute).UnixMilli(), "value": 20, "min": 20, "max": 20},
			},
		},
	}
	mock.SetEnabled(false)
	obsReq := httptest.NewRequest(http.MethodGet, "/api/metrics-store/history", nil)
	if err := svc.WriteHistoryJSONWithEarliest(httptest.NewRecorder(), obsReq, syntheticPayload, time.Hour, oldTs); err != nil {
		t.Fatal(err)
	}
	if got := observer.count.Load(); got != 0 {
		t.Fatalf("synthetic payload with mock mode off incremented extended observer: got %d, want 0", got)
	}
	realPayload := map[string]any{
		"resourceType": "vm",
		"resourceId":   "real-vm-1",
		"range":        "custom",
		"start":        oldTs.UnixMilli(),
		"end":          oldTs.Add(time.Hour).UnixMilli(),
		"source":       "store",
		"metrics": map[string]any{
			"cpu": []map[string]any{
				{"timestamp": oldTs.Add(5 * time.Minute).UnixMilli(), "value": 10, "min": 10, "max": 10},
				{"timestamp": oldTs.Add(10 * time.Minute).UnixMilli(), "value": 20, "min": 20, "max": 20},
			},
		},
	}
	if err := svc.WriteHistoryJSONWithEarliest(httptest.NewRecorder(), obsReq, realPayload, time.Hour, oldTs); err != nil {
		t.Fatal(err)
	}
	if got := observer.count.Load(); got != 1 {
		t.Fatalf("real store payload with 2 old points observer count = %d, want 1", got)
	}
}
