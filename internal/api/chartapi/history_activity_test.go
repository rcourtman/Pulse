package chartapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/testutil"
)

type historyReadResolver struct {
	monitor *monitoring.Monitor
	reads   int
}

func (r *historyReadResolver) MonitorForContext(context.Context) *monitoring.Monitor {
	return r.monitor
}
func (r *historyReadResolver) RecordSuccessfulExtendedHistoryRead() { r.reads++ }

type failedHistoryWriter struct{ short bool }

func (failedHistoryWriter) Header() http.Header { return http.Header{} }
func (failedHistoryWriter) WriteHeader(int)     {}
func (w failedHistoryWriter) Write(p []byte) (int, error) {
	if w.short {
		return len(p) - 1, nil
	}
	return 0, errors.New("closed connection")
}

func TestExtendedHistoryReadRequiresActualOldMetric(t *testing.T) {
	cutoff := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC).UnixMilli()
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{"old metric", fmt.Sprintf(`{"points":[{"timestamp":%d,"value":0}]}`, cutoff-1), true},
		{"exact boundary", fmt.Sprintf(`{"points":[{"timestamp":%d,"value":1}]}`, cutoff), false},
		{"recent only", fmt.Sprintf(`{"points":[{"timestamp":%d,"value":1}]}`, cutoff+1), false},
		{"empty long window", `{"range":"90d","start":1,"stats":{"oldestDataTimestamp":1},"points":[]}`, false},
		{"metadata is not a metric", `{"stats":{"timestamp":1,"value":2},"points":[]}`, false},
		{"zero timestamp", `{"points":[{"timestamp":0,"value":2}]}`, false},
		{"invalid value", `{"points":[{"timestamp":1,"value":null}]}`, false},
		{"nested named resource", `{"data":{"value":{"timestamp":[{"timestamp":1,"value":2}]}}}`, true},
		{"invalid JSON", `{"points":[{"timestamp":1,"value":2}]}invalid`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := containsExtendedHistoryPayload([]byte(tc.body), cutoff); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtendedHistoryReadIgnoresUnsuccessfulOrSyntheticResponses(t *testing.T) {
	testutil.SetMockMode(t, false)
	body := []byte(fmt.Sprintf(`{"points":[{"timestamp":%d,"value":1}]}`, time.Now().Add(-8*24*time.Hour).UnixMilli()))
	for _, tc := range []struct {
		name, method        string
		duration            time.Duration
		cancel, short, fail bool
		want                int
	}{
		{"successful GET", http.MethodGet, 30 * 24 * time.Hour, false, false, false, 1},
		{"HEAD", http.MethodHead, 30 * 24 * time.Hour, false, false, false, 0},
		{"seven day request", http.MethodGet, 7 * 24 * time.Hour, false, false, false, 0},
		{"cancelled", http.MethodGet, 30 * 24 * time.Hour, true, false, false, 0},
		{"short write", http.MethodGet, 30 * 24 * time.Hour, false, true, false, 0},
		{"failed write", http.MethodGet, 30 * 24 * time.Hour, false, false, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &historyReadResolver{}
			service := NewService(resolver)
			req := httptest.NewRequest(tc.method, "/history", nil)
			if tc.cancel {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			var writer http.ResponseWriter = httptest.NewRecorder()
			if tc.short || tc.fail {
				writer = failedHistoryWriter{short: tc.short}
			}
			_ = service.WriteHistoryPayload(writer, req, body, tc.duration)
			if resolver.reads != tc.want {
				t.Fatalf("reads = %d, want %d", resolver.reads, tc.want)
			}
		})
	}
	resolver := &historyReadResolver{}
	service := NewService(resolver)
	req := httptest.NewRequest(http.MethodGet, "/history", nil)
	if err := service.WriteHistoryJSON(httptest.NewRecorder(), req, map[string]any{"value": math.NaN()}, 30*24*time.Hour); err == nil {
		t.Fatal("invalid JSON must fail")
	}
}

func TestExtendedHistoryReadRejectsMockPayload(t *testing.T) {
	testutil.SetMockMode(t, true)
	resolver := &historyReadResolver{}
	service := NewService(resolver)
	req := httptest.NewRequest(http.MethodGet, "/history", nil)
	body := []byte(`{"points":[{"timestamp":1,"value":2}]}`)
	_ = service.WriteHistoryPayload(httptest.NewRecorder(), req, body, 30*24*time.Hour)
	if resolver.reads != 0 {
		t.Fatalf("failed encoding/mock reads counted: %d", resolver.reads)
	}
}

func TestExtendedHistoryReadCountsCachedChartResponses(t *testing.T) {
	testutil.SetMockMode(t, false)
	monitor, state, _ := newTestMonitor(t)
	syncTestResourceStore(t, monitor, state)
	resolver := &historyReadResolver{monitor: monitor}
	service := NewService(resolver)
	body, _ := json.Marshal(map[string]any{"data": map[string]any{"resource": map[string]any{"cpu": []MetricPoint{{Timestamp: time.Now().Add(-8 * 24 * time.Hour).UnixMilli(), Value: 1}}}}})
	now := time.Now()
	service.chartPayloads.put(workloadChartsCachePrefix+"default|30d||", body, now.Add(time.Minute), now)
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		service.HandleWorkloadCharts(rec, httptest.NewRequest(http.MethodGet, "/api/charts/workloads?range=30d", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
	}
	if resolver.reads != 2 {
		t.Fatalf("successful cached responses = %d, want 2", resolver.reads)
	}
}
