package dockeragent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	systemtypes "github.com/moby/moby/api/types/system"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

// The deliberately conflicting compatibility percentages below are synthetic,
// not captured Podman responses. Walk the actual collector, compressed HTTPS
// report, authenticated ingestion, memory/store/HTTP History and alert manager.
// This reproduces a source mechanism, not the reporter's installed cause.
func TestPodmanCPUCollectorHistoryAndAlerts(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{}, nil
	})
	for _, cpus := range []int{8, 4} {
		t.Run(fmt.Sprintf("%d-cpu", cpus), func(t *testing.T) {
			const reportToken = "podman-cpu-report-fixture.12345678"
			const readToken = "podman-cpu-read-fixture.12345678"
			reporter, err := config.NewAPITokenRecord(reportToken, "CPU report fixture", []string{config.ScopeDockerReport})
			if err != nil {
				t.Fatal(err)
			}
			reader, err := config.NewAPITokenRecord(readToken, "CPU History fixture", []string{config.ScopeMonitoringRead})
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			cfg := &config.Config{DataPath: dir, ConfigPath: dir, APITokens: []config.APITokenRecord{*reporter, *reader}}
			monitor, err := monitoring.New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(monitor.Stop)
			if monitor.GetMetricsStore() == nil {
				t.Fatal("persistent History store unavailable")
			}
			router := api.NewRouter(cfg, monitor, nil, nil, nil, "test")
			t.Cleanup(router.ShutdownResourceStores)
			t.Cleanup(router.ShutdownRBAC)
			t.Cleanup(router.ShutdownBackgroundWorkers)
			server := httptest.NewTLSServer(router.Handler())
			t.Cleanup(server.Close)

			alertConfig := monitor.GetAlertManager().GetConfig()
			alertConfig.Enabled = true
			alertConfig.ActivationState = alerts.ActivationActive
			alertConfig.Schedule.QuietHours.Enabled = false
			alertConfig.DockerDefaults.CPU = alerts.HysteresisThreshold{Trigger: 80, Clear: 70}
			alertConfig.MetricTimeThresholds = map[string]map[string]int{"all": {"cpu": 0}}
			// Exercise the supported explicit instantaneous policy. The default
			// five-minute window cannot have coverage in this short source fixture.
			alertConfig.MetricEvaluationWindows = map[string]map[string]int{"all": {"cpu": 0}}
			monitor.GetAlertManager().UpdateConfig(alertConfig)

			epoch := time.Now().UTC().Add(-time.Minute)
			type sample struct {
				total, halfTotal   uint64
				native, halfNative float64
				want, halfWant     float64
				start              int
			}
			// The first three disagreements can produce the reported 74.3/88.0/
			// 90.7 capacity values on 8 CPUs with the old positive-value override.
			samples := []sample{
				{total: 1_000_000_000, halfTotal: 500_000_000},
				{total: 1_999_900_000, halfTotal: 1_000_400_000, native: 594.4, halfNative: 725.6, want: 99.99, halfWant: 50.04},
				{total: 2_999_700_000, halfTotal: 1_500_400_000, native: 704, halfNative: 92.8, want: 99.98, halfWant: 50},
				{total: 2_999_700_000, halfTotal: 1_500_400_000, native: 800, halfNative: 148.8},                                                         // Idle, not a positive-value fallback.
				{total: 10_000_000, halfTotal: 5_000_000, native: 800, halfNative: 800},                                                                  // Counter reset.
				{total: 10_000_000 + uint64(cpus)*900_000_000, halfTotal: 505_000_000, native: 1, halfNative: 1, want: float64(cpus) * 90, halfWant: 50}, // Real sustained high usage still alerts.
				{total: 20_000_000_000, halfTotal: 20_000_000_000, native: 800, halfNative: 800, start: 1},                                               // Restart with a larger lifetime counter.
			}
			var current sample
			var index int
			fullID, halfID := "podman-full-core-123456", "podman-half-core-123456"
			agent := &Agent{
				cfg:     Config{AgentID: fmt.Sprintf("podman-cpu-host-%d", cpus), IncludeContainers: true, Interval: time.Second},
				runtime: RuntimePodman, logger: zerolog.Nop(), agentVersion: "test", reportStreamID: "cpu-roundtrip-stream",
				targets:     []TargetConfig{{URL: server.URL, Token: reportToken}},
				httpClients: map[bool]*http.Client{false: server.Client()},
				docker: &fakeDockerClient{
					infoFunc: func(context.Context) (systemtypes.Info, error) {
						return systemtypes.Info{Name: "cpu-host", NCPU: cpus, ServerVersion: "5.4.2"}, nil
					},
					containerListFunc: func(context.Context, dockerContainerListOptions) ([]containertypes.Summary, error) {
						return []containertypes.Summary{
							{ID: fullID, Names: []string{"/full-core"}, State: "running", Status: "Up"},
							{ID: halfID, Names: []string{"/half-core"}, State: "running", Status: "Up"},
						}, nil
					},
					containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
						inspect := baseInspect()
						inspect.State = &containertypes.State{Running: true, StartedAt: epoch.Add(time.Duration(current.start-60) * time.Second).Format(time.RFC3339Nano)}
						return inspect, nil, nil
					},
					containerStatsOneShotFn: func(_ context.Context, id string) (dockerStatsResponseReader, error) {
						total, percent := current.total, current.native
						if id == halfID {
							total, percent = current.halfTotal, current.halfNative
						}
						payload := fmt.Sprintf(`{"read":%q,"cpu_stats":{"cpu_usage":{"total_usage":%d},"system_cpu_usage":%d,"online_cpus":%d,"cpu":%g},"memory_stats":{"usage":1000000,"limit":4000000}}`,
							epoch.Add(time.Duration(index)*time.Second).Format(time.RFC3339Nano), total, uint64(index+1)*1_000_000_000, cpus, percent)
						return dockerStatsResponseReader{Body: io.NopCloser(strings.NewReader(payload))}, nil
					},
				},
			}
			requestHistory := func(token, id, metric string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/history?resourceType=app-container&resourceId="+id+"&metric="+metric+"&range=5m&maxPoints=240", nil)
				if token != "" {
					req.Header.Set("X-API-Token", token)
				}
				rec := httptest.NewRecorder()
				router.Handler().ServeHTTP(rec, req)
				return rec
			}
			for _, test := range []struct {
				token  string
				status int
			}{{"", http.StatusUnauthorized}, {reportToken, http.StatusForbidden}} {
				if rec := requestHistory(test.token, fullID, "cpu"); rec.Code != test.status {
					t.Fatalf("History auth/scope status %d, want %d", rec.Code, test.status)
				}
			}

			var lastReport agentsdocker.Report
			for i, s := range samples {
				if i > 0 {
					// The persistent raw tier has whole-second timestamps. Give each
					// real ingestion a distinct bucket rather than inserting fake rows.
					time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)) + 10*time.Millisecond)
				}
				index, current = i, s
				report, err := agent.buildReport(context.Background())
				if err != nil || len(report.Containers) != 2 {
					t.Fatalf("build sample %d: %v, containers %d", i, err, len(report.Containers))
				}
				before := time.Now()
				if err := agent.sendReport(context.Background(), report); err != nil {
					t.Fatalf("send sample %d: %v", i, err)
				}
				after := time.Now()
				lastReport = report
				host, ok := monitor.GetDockerHost(agent.cfg.AgentID)
				if !ok || host.CPUs != cpus || host.Runtime != "podman" || len(host.Containers) != 2 {
					t.Fatalf("ingested identity/capacity changed: %+v", host)
				}
				monitor.GetMetricsStore().Flush()
				for _, c := range host.Containers {
					want := s.want
					if c.ID == halfID {
						want = s.halfWant
					}
					capacity := want / float64(cpus)
					if math.Abs(c.CPUPercent-want) > 1e-8 || math.Abs(c.CPUCapacityPercent-capacity) > 1e-8 || c.MemoryUsage != 1_000_000 || c.MemoryPercent != 25 {
						t.Errorf("sample %d container %s: raw %g capacity %g memory %g, want raw %g capacity %g memory 25", i, c.ID, c.CPUPercent, c.CPUCapacityPercent, c.MemoryPercent, want, capacity)
					}
					memory := monitor.GetMetricsHistory().GetGuestMetrics("docker:"+c.ID, "cpu", 5*time.Minute)
					if len(memory) != i+1 || math.Abs(memory[len(memory)-1].Value-capacity) > 1e-8 || memory[len(memory)-1].Timestamp.Before(before) || memory[len(memory)-1].Timestamp.After(after) {
						t.Errorf("sample %d memory History for %s = %+v, want capacity %g at receipt time", i, c.ID, memory, capacity)
					}
					rec := requestHistory(readToken, c.ID, "cpu")
					var history struct {
						ResourceType string `json:"resourceType"`
						ResourceID   string `json:"resourceId"`
						Metric       string `json:"metric"`
						Source       string `json:"source"`
						Points       []struct {
							Timestamp int64   `json:"timestamp"`
							Value     float64 `json:"value"`
						} `json:"points"`
					}
					if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &history) != nil || history.ResourceType != "app-container" || history.ResourceID != c.ID || history.Metric != "cpu" || history.Source != "store" || len(history.Points) != i+1 {
						t.Fatalf("sample %d stored HTTP History identity/coverage: %d %s", i, rec.Code, rec.Body.String())
					}
					point := history.Points[len(history.Points)-1]
					if math.Abs(point.Value-capacity) > 1e-8 || point.Timestamp < before.Truncate(time.Second).UnixMilli() || point.Timestamp > after.Truncate(time.Second).UnixMilli() {
						t.Errorf("sample %d stored HTTP History %s: %+v, want capacity %g at receipt time", i, c.ID, point, capacity)
					}
					// The existing drawer asks for all metrics, not just CPU. Require
					// that response to preserve the same stored identity/value/time.
					allRec := requestHistory(readToken, c.ID, "")
					var all struct {
						ResourceType string `json:"resourceType"`
						ResourceID   string `json:"resourceId"`
						Metrics      map[string][]struct {
							Timestamp int64   `json:"timestamp"`
							Value     float64 `json:"value"`
							Min       float64 `json:"min"`
							Max       float64 `json:"max"`
						} `json:"metrics"`
					}
					if allRec.Code != http.StatusOK || json.Unmarshal(allRec.Body.Bytes(), &all) != nil || all.ResourceType != "app-container" || all.ResourceID != c.ID || len(all.Metrics["cpu"]) != i+1 || len(all.Metrics["memory"]) != i+1 {
						t.Fatalf("sample %d all-metric drawer identity/coverage: %d %s", i, allRec.Code, allRec.Body.String())
					}
					cpuPoint := all.Metrics["cpu"][i]
					memoryPoint := all.Metrics["memory"][i]
					if cpuPoint.Timestamp != point.Timestamp || math.Abs(cpuPoint.Value-capacity) > 1e-8 || math.Abs(cpuPoint.Min-capacity) > 1e-8 || math.Abs(cpuPoint.Max-capacity) > 1e-8 || memoryPoint.Value != 25 {
						t.Errorf("sample %d all-metric drawer changed CPU/memory: cpu %+v, memory %+v", i, cpuPoint, memoryPoint)
					}
				}
				active := monitor.GetAlertManager().GetActiveAlerts()
				if i == 5 {
					if len(active) != 1 || active[0].Value != 90 || active[0].Metadata["containerId"] != fullID || active[0].Metadata["cpuRawPercent"] != float64(cpus)*90 || active[0].Metadata["cpuCapacityCores"] != cpus {
						t.Errorf("real high usage did not reach the correct alert identity/units: %+v", active)
					}
				} else if len(active) != 0 {
					t.Errorf("sample %d conflicting one-shot percentage manufactured alerts: %+v", i, active)
				}
				t.Logf("sample %d: checked final compressed report, two raw/capacity identities, memory/store HTTP History and %d active alerts on %d CPUs", i, len(active), cpus)
			}
			// A duplicated final report must not refresh or add History samples.
			if err := agent.sendReport(context.Background(), lastReport); err != nil {
				t.Fatal(err)
			}
			if n := len(monitor.GetMetricsHistory().GetGuestMetrics("docker:"+fullID, "cpu", 5*time.Minute)); n != len(samples) {
				t.Fatalf("duplicate report added History: %d points", n)
			}
			// The History credential must not admit report writes.
			req := httptest.NewRequest(http.MethodPost, "/api/agents/docker/report", strings.NewReader(`{}`))
			req.Header.Set("X-API-Token", readToken)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("read token admitted report write: %d", rec.Code)
			}
		})
	}
}
