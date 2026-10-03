package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func TestTrueNASPollerPollsConfiguredConnections(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	mock := newTrueNASMockServer(t, "nas-one")
	t.Cleanup(mock.Close)

	mtp, persistence := newTestTenantPersistence(t)
	connection := trueNASInstanceForServer(t, "conn-1", mock.URL(), true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return mock.RequestCount() >= 5 && hasTrueNASHostForOrg(poller, "default", "nas-one")
	}, "expected configured TrueNAS connection to poll and ingest host resources")

	poller.Stop()
	if !hasTrueNASHostForOrg(poller, "default", "nas-one") {
		t.Fatal("expected TrueNAS resources to be ingested")
	}
}

func TestTrueNASPollerFeatureFlagGate(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(false)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	mock := newTrueNASMockServer(t, "nas-feature-flag-off")
	t.Cleanup(mock.Close)

	mtp, persistence := newTestTenantPersistence(t)
	connection := trueNASInstanceForServer(t, "feature-flag-off-conn", mock.URL(), true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	initialStopped := poller.stopped

	poller.Start(context.Background())

	if poller.cancel != nil {
		t.Fatal("expected Start() to be a no-op with feature flag disabled")
	}
	if poller.stopped != initialStopped {
		t.Fatal("expected stopped channel to remain unchanged when Start() is gated")
	}
	select {
	case <-poller.stopped:
	default:
		t.Fatal("expected stopped channel to remain pre-closed when Start() is gated")
	}

	noPollDeadline := time.Now().Add(200 * time.Millisecond)
	waitForCondition(t, 500*time.Millisecond, func() bool {
		return time.Now().After(noPollDeadline) && mock.RequestCount() == 0
	}, "expected no TrueNAS polling requests when feature flag is disabled")

	poller.Stop()
}

func TestTrueNASPollerStaysMonitoringOnly(t *testing.T) {
	source, err := os.ReadFile("truenas_poller.go")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	for _, retired := range []string{
		"max_monitored_systems",
		"plan_limit",
		"would_exceed_limit",
		"grandfather",
		"admission",
		"billing",
		"capacity",
		"limit",
	} {
		if strings.Contains(string(source), retired) {
			t.Fatalf("TrueNAS poller must stay inventory-only and must not reference retired monitor-count cap token %q", retired)
		}
	}
}

func TestTrueNASPollerEnableDisableCycle(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	mock := newTrueNASMockServer(t, "nas-enable-disable")
	t.Cleanup(mock.Close)

	mtp, persistence := newTestTenantPersistence(t)
	connection := trueNASInstanceForServer(t, "enable-disable-conn", mock.URL(), true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return pollerProviderCount(poller) == 1 &&
			pollerHasProvider(poller, connection.ID) &&
			mock.RequestCount() >= 5 &&
			hasTrueNASHostForOrg(poller, "default", "nas-enable-disable")
	}, "expected enabled poller to start and poll configured TrueNAS connection")

	poller.Stop()
	if !hasTrueNASHostForOrg(poller, "default", "nas-enable-disable") {
		t.Fatal("expected enabled poller to ingest TrueNAS resources")
	}

	requestCountAfterStop := mock.RequestCount()
	recordCountAfterStop := len(poller.GetCurrentRecordsForOrg("default"))

	truenas.SetFeatureEnabled(false)
	poller.Start(context.Background())

	if poller.cancel != nil {
		t.Fatal("expected Start() to remain a no-op after disable without restarting process")
	}

	noPollDeadline := time.Now().Add(200 * time.Millisecond)
	waitForCondition(t, 500*time.Millisecond, func() bool {
		return time.Now().After(noPollDeadline) && mock.RequestCount() == requestCountAfterStop
	}, "expected no additional polling requests after disable and restart attempt")

	if got := len(poller.GetCurrentRecordsForOrg("default")); got != recordCountAfterStop {
		t.Fatalf("expected no new records after disable restart attempt, got before=%d after=%d", recordCountAfterStop, got)
	}
}

func TestTrueNASPollerKillSwitchAllConnectionsRemoved(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	mock := newTrueNASMockServer(t, "nas-kill-switch")
	t.Cleanup(mock.Close)

	mtp, persistence := newTestTenantPersistence(t)
	connection := trueNASInstanceForServer(t, "kill-switch-conn", mock.URL(), true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return pollerProviderCount(poller) == 1 && pollerHasProvider(poller, connection.ID) && mock.RequestCount() >= 5
	}, "expected initial TrueNAS connection to be active and polling")

	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{}); err != nil {
		t.Fatalf("SaveTrueNASConfig() clear error = %v", err)
	}

	waitForCondition(t, 2*time.Second, func() bool {
		return pollerProviderCount(poller) == 0
	}, "expected all TrueNAS providers to be drained after removing all connections")

	if pollerHasProvider(poller, connection.ID) {
		t.Fatalf("expected provider %q to be removed after kill-switch config update", connection.ID)
	}

	requestCountAfterDrain := mock.RequestCount()
	noPollDeadline := time.Now().Add(200 * time.Millisecond)
	waitForCondition(t, 500*time.Millisecond, func() bool {
		return time.Now().After(noPollDeadline) && mock.RequestCount() == requestCountAfterDrain
	}, "expected no further polling after all TrueNAS connections are removed")

	poller.Stop()
}

func TestTrueNASPollerRecordsMetrics(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	var requestCount atomic.Int64
	var errorCount atomic.Int64
	var successCount atomic.Int64
	var remainingFailures atomic.Int64
	remainingFailures.Store(3)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")

		if remainingFailures.Load() > 0 {
			remainingFailures.Add(-1)
			errorCount.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"simulated failure"}`))
			return
		}

		successCount.Add(1)
		switch r.URL.Path {
		case "/api/v2.0/system/info":
			_, _ = w.Write([]byte(`{"hostname":"metrics-host","version":"TrueNAS-SCALE-24.10.2","buildtime":"24.10.2.1","uptime_seconds":86400,"system_serial":"SER-001"}`))
		case "/api/v2.0/pool":
			_, _ = w.Write([]byte(`[{"id":1,"name":"tank","status":"ONLINE","size":1000,"allocated":400,"free":600}]`))
		case "/api/v2.0/pool/dataset":
			_, _ = w.Write([]byte(`[]`))
		case "/api/v2.0/disk":
			_, _ = w.Write([]byte(`[]`))
		case "/api/v2.0/alert/list":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	mtp, persistence := newTestTenantPersistence(t)
	connection := trueNASInstanceForServer(t, "metrics-conn", server.URL, true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 5*time.Second, func() bool {
		return successCount.Load() > 0 && hasTrueNASHostForOrg(poller, "default", "metrics-host")
	}, "expected TrueNAS resources to appear after initial failures")

	poller.Stop()
	if !hasTrueNASHostForOrg(poller, "default", "metrics-host") {
		t.Fatal("expected TrueNAS resources to appear after initial failures")
	}

	if errorCount.Load() == 0 {
		t.Fatal("expected at least one failed request to exercise metrics error path")
	}
	if successCount.Load() == 0 {
		t.Fatal("expected successful requests to exercise metrics success path")
	}
	if requestCount.Load() < errorCount.Load()+successCount.Load() {
		t.Fatalf("unexpected request accounting: total=%d errors=%d successes=%d", requestCount.Load(), errorCount.Load(), successCount.Load())
	}
}

func TestTrueNASPollerConnectionSummariesExposeObservedCounts(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	mock := newTrueNASMockServer(t, "summary-host")
	t.Cleanup(mock.Close)

	mtp, persistence := newTestTenantPersistence(t)
	connection := trueNASInstanceForServer(t, "summary-conn", mock.URL(), true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		summaries := poller.ConnectionSummaries("default", []config.TrueNASInstance{connection})
		summary, ok := summaries[connection.ID]
		return ok && summary.Poll != nil && summary.Poll.LastSuccessAt != nil && summary.Observed != nil
	}, "expected connection summary to include successful poll and observed counts")

	summary := poller.ConnectionSummaries("default", []config.TrueNASInstance{connection})[connection.ID]
	if summary.Poll == nil || summary.Poll.IntervalSeconds != 60 {
		t.Fatalf("expected poll interval summary 60 seconds, got %+v", summary.Poll)
	}
	if summary.Observed == nil {
		t.Fatal("expected observed summary to be present")
	}
	if summary.Observed.Host != "summary-host" || summary.Observed.ResourceID != "summary-host" {
		t.Fatalf("unexpected observed host identity: %+v", summary.Observed)
	}
	if summary.Observed.Systems != 1 || summary.Observed.StoragePools != 1 || summary.Observed.Datasets != 1 || summary.Observed.Disks != 1 {
		t.Fatalf("unexpected observed counts: %+v", summary.Observed)
	}
	if summary.Transport == nil ||
		summary.Transport.Mode != truenas.TransportLegacyREST ||
		!summary.Transport.Connected {
		t.Fatalf("unexpected connection-local transport summary: %+v", summary.Transport)
	}
}

func TestTrueNASObservedSummaryIncludesNativeRuntimeAndSharingFacets(t *testing.T) {
	collectedAt := time.Date(2026, time.March, 30, 12, 0, 0, 0, time.UTC)
	summary := buildTrueNASObservedSummary(&truenas.FixtureSnapshot{
		CollectedAt: collectedAt,
		System:      truenas.SystemInfo{Hostname: "summary-native"},
		Pools:       []truenas.Pool{{Name: "tank"}},
		Datasets:    []truenas.Dataset{{Name: "tank/apps"}},
		Apps:        []truenas.App{{ID: "nextcloud"}, {ID: "adguard"}},
		VMs:         []truenas.VirtualMachine{{ID: "42"}, {ID: "43"}},
		Shares:      []truenas.NetworkShare{{ID: "smb-1"}, {ID: "nfs-1"}, {ID: "smb-2"}},
		Disks:       []truenas.Disk{{Name: "sda"}},
		ZFSSnapshots: []truenas.ZFSSnapshot{
			{ID: "tank/apps@auto-1"},
		},
		ReplicationTasks: []truenas.ReplicationTask{
			{ID: "replicate-tank-apps"},
		},
	})

	if summary == nil {
		t.Fatal("expected observed summary")
	}
	if summary.Systems != 1 ||
		summary.StoragePools != 1 ||
		summary.Datasets != 1 ||
		summary.Apps != 2 ||
		summary.VMs != 2 ||
		summary.Shares != 3 ||
		summary.Disks != 1 ||
		summary.RecoveryArtifacts != 2 {
		t.Fatalf("unexpected native observed counts: %+v", summary)
	}
	if summary.CollectedAt == nil || !summary.CollectedAt.Equal(collectedAt) {
		t.Fatalf("expected collectedAt %s, got %+v", collectedAt, summary.CollectedAt)
	}
}

func TestTrueNASPollerConnectionSummariesCaptureFailures(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	t.Cleanup(server.Close)

	mtp, persistence := newTestTenantPersistence(t)
	connection := trueNASInstanceForServer(t, "summary-fail", server.URL, true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		summaries := poller.ConnectionSummaries("default", []config.TrueNASInstance{connection})
		summary, ok := summaries[connection.ID]
		return ok && summary.Poll != nil && summary.Poll.LastError != nil && summary.Poll.ConsecutiveFailures > 0
	}, "expected connection summary to capture poll failure state")

	summary := poller.ConnectionSummaries("default", []config.TrueNASInstance{connection})[connection.ID]
	if summary.Poll == nil || summary.Poll.LastError == nil {
		t.Fatalf("expected poll failure summary, got %+v", summary.Poll)
	}
	if summary.Poll.LastError.Category != "auth" {
		t.Fatalf("expected auth error category, got %+v", summary.Poll.LastError)
	}
}

func TestTrueNASPollerRuntimeRecoveryUpdatesSummariesAndObservedCounts(t *testing.T) {
	poller := NewTrueNASPoller(nil, time.Minute, nil)
	connection := config.TrueNASInstance{
		ID:               "manual-test-conn",
		Host:             "manual-test.local",
		APIKey:           "secret",
		UseHTTPS:         true,
		Enabled:          true,
		PollIntervalSecs: 120,
	}
	connection.ApplyDefaults()

	snapshot := &truenas.FixtureSnapshot{
		System: truenas.SystemInfo{
			Hostname: "manual-test",
		},
		Pools: []truenas.Pool{{Name: "tank"}},
	}
	firstSuccess := time.Date(2026, time.March, 30, 10, 0, 0, 0, time.UTC)
	failureAt := firstSuccess.Add(2 * time.Minute)
	recoveryAt := failureAt.Add(2 * time.Minute)

	poller.mu.Lock()
	poller.recordConnectionSuccessLocked("default", connection.ID, connection, firstSuccess, firstSuccess, snapshot)
	poller.recordConnectionFailureLocked("default", connection.ID, connection, errors.New("manual auth failed"), failureAt)
	poller.mu.Unlock()

	snapshot.CollectedAt = recoveryAt
	poller.mu.Lock()
	poller.recordConnectionSuccessLocked("default", connection.ID, connection, recoveryAt, recoveryAt, snapshot)
	poller.mu.Unlock()

	summary := poller.ConnectionSummaries("default", []config.TrueNASInstance{connection})[connection.ID]
	if summary.Poll == nil || summary.Poll.LastSuccessAt == nil {
		t.Fatalf("expected runtime recovery to update poll summary, got %+v", summary.Poll)
	}
	if summary.Poll.LastError != nil || summary.Poll.ConsecutiveFailures != 0 || !summary.Poll.LastSuccessAt.Equal(recoveryAt) {
		t.Fatalf("expected runtime recovery to clear previous error, got %+v", summary.Poll)
	}
	if summary.Observed == nil || summary.Observed.Host != "manual-test" || summary.Observed.StoragePools != 1 ||
		summary.Observed.CollectedAt == nil || !summary.Observed.CollectedAt.Equal(recoveryAt) {
		t.Fatalf("expected observed summary to advance after runtime recovery, got %+v", summary.Observed)
	}
}

func TestTrueNASPollerHonorsConfiguredPollInterval(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	mock := newTrueNASMockServer(t, "interval-host")
	t.Cleanup(mock.Close)

	mtp, persistence := newTestTenantPersistence(t)
	connection := trueNASInstanceForServer(t, "interval-conn", mock.URL(), true)
	connection.PollIntervalSecs = 1
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 0, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	var firstPollSuccessAt time.Time
	waitForCondition(t, 2*time.Second, func() bool {
		summaries := poller.ConnectionSummaries("default", []config.TrueNASInstance{connection})
		summary, ok := summaries[connection.ID]
		if !ok || summary.Poll == nil || summary.Poll.LastSuccessAt == nil {
			return false
		}
		firstPollSuccessAt = *summary.Poll.LastSuccessAt
		return true
	}, "expected initial immediate poll for configured TrueNAS connection")

	time.Sleep(400 * time.Millisecond)
	if summary := poller.ConnectionSummaries("default", []config.TrueNASInstance{connection})[connection.ID]; summary.Poll == nil || summary.Poll.LastSuccessAt == nil || !summary.Poll.LastSuccessAt.Equal(firstPollSuccessAt) {
		t.Fatalf("expected configured 1s poll interval to avoid an early repoll, got first=%s current=%v", firstPollSuccessAt.Format(time.RFC3339Nano), summary.Poll.LastSuccessAt)
	}

	waitForCondition(t, 2*time.Second, func() bool {
		summary := poller.ConnectionSummaries("default", []config.TrueNASInstance{connection})[connection.ID]
		return summary.Poll != nil && summary.Poll.LastSuccessAt != nil && summary.Poll.LastSuccessAt.After(firstPollSuccessAt)
	}, "expected configured 1s poll interval to trigger the next poll without waiting for the 60s default")
}

func TestTrueNASPollerPhysicalDiskTemperatureHistoryUsesTenantScopedProvider(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	fixtures := truenas.DefaultFixtures()
	now := time.Date(2026, 3, 29, 20, 0, 0, 0, time.UTC)
	fetcher := &controllableTrueNASHistoryFetcher{
		snapshot: &fixtures,
		history: map[string][]truenas.TimeSeriesPoint{
			"sda": {
				{Timestamp: now.Add(-2 * time.Hour), Value: 30},
				{Timestamp: now.Add(-1 * time.Hour), Value: 32},
				{Timestamp: now, Value: 34},
			},
		},
	}
	provider := truenas.NewLiveProvider(fetcher)
	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	poller := NewTrueNASPoller(nil, time.Minute, nil)
	poller.providersByOrg["default"] = map[string]*truenas.Provider{
		"conn-1": provider,
	}

	history := poller.PhysicalDiskTemperatureHistory(nil, "default", 4*time.Hour)
	points, ok := history["ZL0A1234"]
	if !ok {
		t.Fatalf("expected canonical metric resource id ZL0A1234, got %#v", history)
	}
	if len(points) != 3 || points[len(points)-1].Value != 34 {
		t.Fatalf("unexpected tenant-scoped disk history: %+v", points)
	}
}

func TestTrueNASPollerGuestMetricHistoryUsesTenantScopedProvider(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	fixtures := truenas.DefaultFixtures()
	now := time.Date(2026, 3, 29, 20, 0, 0, 0, time.UTC)
	fetcher := &controllableTrueNASHistoryFetcher{
		snapshot: &fixtures,
		systemHistory: &truenas.SystemMetricHistory{
			CPUPercent: []truenas.TimeSeriesPoint{
				{Timestamp: now.Add(-2 * time.Hour), Value: 20},
				{Timestamp: now, Value: 34},
			},
			MemoryPercent: []truenas.TimeSeriesPoint{
				{Timestamp: now.Add(-2 * time.Hour), Value: 45},
				{Timestamp: now, Value: 62},
			},
		},
	}
	provider := truenas.NewLiveProvider(fetcher)
	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	poller := NewTrueNASPoller(nil, time.Minute, nil)
	poller.providersByOrg["default"] = map[string]*truenas.Provider{
		"conn-1": provider,
	}

	history := poller.GuestMetricHistory(nil, "default", "agent", 4*time.Hour)
	metricMap, ok := history["truenas-main"]
	if !ok {
		t.Fatalf("expected canonical agent metric id truenas-main, got %#v", history)
	}
	if len(metricMap["cpu"]) != 2 || metricMap["cpu"][1].Value != 34 {
		t.Fatalf("unexpected cpu history: %+v", metricMap["cpu"])
	}
	if len(metricMap["memory"]) != 2 || metricMap["memory"][1].Value != 62 {
		t.Fatalf("unexpected memory history: %+v", metricMap["memory"])
	}
}

type controllableTrueNASHistoryFetcher struct {
	snapshot      *truenas.FixtureSnapshot
	history       map[string][]truenas.TimeSeriesPoint
	systemHistory *truenas.SystemMetricHistory
}

func (s *controllableTrueNASHistoryFetcher) Fetch(context.Context) (*truenas.FixtureSnapshot, error) {
	if s == nil || s.snapshot == nil {
		return nil, nil
	}
	copied := *s.snapshot
	copied.Disks = append([]truenas.Disk(nil), s.snapshot.Disks...)
	copied.Pools = append([]truenas.Pool(nil), s.snapshot.Pools...)
	copied.Datasets = append([]truenas.Dataset(nil), s.snapshot.Datasets...)
	copied.Alerts = append([]truenas.Alert(nil), s.snapshot.Alerts...)
	copied.Apps = append([]truenas.App(nil), s.snapshot.Apps...)
	copied.ZFSSnapshots = append([]truenas.ZFSSnapshot(nil), s.snapshot.ZFSSnapshots...)
	copied.ReplicationTasks = append([]truenas.ReplicationTask(nil), s.snapshot.ReplicationTasks...)
	return &copied, nil
}

func (s *controllableTrueNASHistoryFetcher) DiskTemperatureHistory(_ context.Context, identifiers []string, _ time.Duration) (map[string][]truenas.TimeSeriesPoint, error) {
	result := make(map[string][]truenas.TimeSeriesPoint)
	for _, identifier := range identifiers {
		points, ok := s.history[identifier]
		if !ok || len(points) == 0 {
			continue
		}
		copied := make([]truenas.TimeSeriesPoint, len(points))
		copy(copied, points)
		result[identifier] = copied
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func (s *controllableTrueNASHistoryFetcher) SystemMetricHistory(context.Context, time.Duration) (*truenas.SystemMetricHistory, error) {
	if s == nil || s.systemHistory == nil {
		return nil, nil
	}
	copied := *s.systemHistory
	copied.CPUPercent = append([]truenas.TimeSeriesPoint(nil), s.systemHistory.CPUPercent...)
	copied.MemoryPercent = append([]truenas.TimeSeriesPoint(nil), s.systemHistory.MemoryPercent...)
	copied.MemoryUsedBytes = append([]truenas.TimeSeriesPoint(nil), s.systemHistory.MemoryUsedBytes...)
	copied.MemoryAvailableBytes = append([]truenas.TimeSeriesPoint(nil), s.systemHistory.MemoryAvailableBytes...)
	copied.MemoryTotalBytes = append([]truenas.TimeSeriesPoint(nil), s.systemHistory.MemoryTotalBytes...)
	copied.NetInRate = append([]truenas.TimeSeriesPoint(nil), s.systemHistory.NetInRate...)
	copied.NetOutRate = append([]truenas.TimeSeriesPoint(nil), s.systemHistory.NetOutRate...)
	copied.DiskReadRate = append([]truenas.TimeSeriesPoint(nil), s.systemHistory.DiskReadRate...)
	copied.DiskWriteRate = append([]truenas.TimeSeriesPoint(nil), s.systemHistory.DiskWriteRate...)
	return &copied, nil
}

type pollerControlFetcher struct {
	snapshot   *truenas.FixtureSnapshot
	startCalls []string
	stopCalls  []string
	logReads   []pollerLogReadCall
}

type pollerLogReadCall struct {
	appName     string
	containerID string
	tailLines   int
}

func (f *pollerControlFetcher) Fetch(context.Context) (*truenas.FixtureSnapshot, error) {
	if f == nil {
		return nil, nil
	}
	return copyTrueNASSnapshot(f.snapshot), nil
}

func (f *pollerControlFetcher) StartApp(_ context.Context, appID string) error {
	f.startCalls = append(f.startCalls, appID)
	for i := range f.snapshot.Apps {
		if f.snapshot.Apps[i].ID == appID {
			f.snapshot.Apps[i].State = "RUNNING"
			if len(f.snapshot.Apps[i].Containers) > 0 {
				f.snapshot.Apps[i].Containers[0].State = "running"
			}
		}
	}
	return nil
}

func (f *pollerControlFetcher) StopApp(_ context.Context, appID string) error {
	f.stopCalls = append(f.stopCalls, appID)
	for i := range f.snapshot.Apps {
		if f.snapshot.Apps[i].ID == appID {
			f.snapshot.Apps[i].State = "STOPPED"
			if len(f.snapshot.Apps[i].Containers) > 0 {
				f.snapshot.Apps[i].Containers[0].State = "stopped"
			}
		}
	}
	return nil
}

func (f *pollerControlFetcher) ReadAppLogs(_ context.Context, appName, containerID string, tailLines int) ([]truenas.AppLogLine, error) {
	f.logReads = append(f.logReads, pollerLogReadCall{
		appName:     appName,
		containerID: containerID,
		tailLines:   tailLines,
	})
	return []truenas.AppLogLine{
		{Timestamp: "2026-03-29T18:00:00Z", Data: "ready"},
	}, nil
}

func TestTrueNASPollerControlAppRefreshesCachedRecords(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	fixtures := truenas.DefaultFixtures()
	for i := range fixtures.Apps {
		if fixtures.Apps[i].ID == "nextcloud" {
			fixtures.Apps[i].State = "STOPPED"
			if len(fixtures.Apps[i].Containers) > 0 {
				fixtures.Apps[i].Containers[0].State = "stopped"
			}
		}
	}

	fetcher := &pollerControlFetcher{snapshot: &fixtures}
	provider := truenas.NewLiveProvider(fetcher)
	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	poller := NewTrueNASPoller(nil, 0, nil)
	poller.providersByOrg["default"] = map[string]*truenas.Provider{"conn-1": provider}
	poller.cachedRecordsByOrg["default"] = map[string][]unifiedresources.IngestRecord{"conn-1": provider.Records()}

	app, err := poller.ControlApp(context.Background(), "default", "truenas-main", "nextcloud", "start")
	if err != nil {
		t.Fatalf("ControlApp() error = %v", err)
	}
	if app == nil || app.State != "RUNNING" {
		t.Fatalf("expected RUNNING app after control, got %+v", app)
	}
	if len(fetcher.startCalls) != 1 || fetcher.startCalls[0] != "nextcloud" {
		t.Fatalf("expected start call for nextcloud, got %+v", fetcher.startCalls)
	}
	if got := len(poller.cachedRecordsByOrg["default"]["conn-1"]); got == 0 {
		t.Fatal("expected refreshed cached records after app control")
	}
}

func TestExpectedTrueNASReplicationReadOnlyTargetsAreConnectionScoped(t *testing.T) {
	connections := []trueNASReplicationConnection{
		{
			connectionID:   "sender",
			configuredHost: "https://sender.example.test",
			snapshot: &truenas.FixtureSnapshot{
				System: truenas.SystemInfo{Hostname: "sender"},
				ReplicationTasks: []truenas.ReplicationTask{{
					ID:             "7",
					Direction:      "PUSH",
					Transport:      "SSH",
					ReadOnlyMode:   "SET",
					TargetHost:     "backup.example.test",
					SourceDatasets: []string{"tank/apps", "tank/vms"},
					TargetDataset:  "backup/replicas",
				}},
			},
		},
		{
			connectionID:   "receiver",
			configuredHost: "https://backup.example.test:443",
			snapshot: &truenas.FixtureSnapshot{
				System: truenas.SystemInfo{Hostname: "backup"},
				Datasets: []truenas.Dataset{
					{Name: "backup/replicas", Mounted: true, ReadOnly: true},
					{Name: "backup/replicas/apps", Mounted: true, ReadOnly: true},
				},
			},
		},
		{
			connectionID:   "same-dataset-name-on-another-system",
			configuredHost: "https://unrelated.example.test",
			snapshot: &truenas.FixtureSnapshot{
				System: truenas.SystemInfo{Hostname: "unrelated"},
				Datasets: []truenas.Dataset{
					{Name: "backup/replicas", Mounted: true, ReadOnly: true},
				},
			},
		},
	}

	targets := expectedTrueNASReplicationReadOnlyTargets(connections)
	if got := targets["receiver"]; len(got) != 1 || got[0] != "backup/replicas" {
		t.Fatalf("expected receiver target root, got %#v", targets)
	}
	if got := targets["same-dataset-name-on-another-system"]; len(got) != 0 {
		t.Fatalf("dataset names must not correlate across unmatched connections, got %#v", targets)
	}
}

func TestExpectedTrueNASReplicationReadOnlyTargetsFailClosed(t *testing.T) {
	connections := []trueNASReplicationConnection{
		{
			connectionID:   "sender",
			configuredHost: "https://sender.example.test",
			snapshot: &truenas.FixtureSnapshot{
				ReplicationTasks: []truenas.ReplicationTask{
					{ID: "set-without-host", Direction: "PUSH", Transport: "SSH", ReadOnlyMode: "SET", TargetDataset: "backup/a"},
					{ID: "set-ambiguous-host", Direction: "PUSH", Transport: "SSH", ReadOnlyMode: "SET", TargetHost: "backup.example.test", TargetDataset: "backup/a"},
					{ID: "ignored", Direction: "PUSH", Transport: "SSH", ReadOnlyMode: "IGNORE", TargetHost: "backup.example.test", TargetDataset: "backup/b"},
				},
			},
		},
		{connectionID: "receiver-a", configuredHost: "https://backup.example.test", snapshot: &truenas.FixtureSnapshot{}},
		{connectionID: "receiver-b", configuredHost: "https://backup.example.test:443", snapshot: &truenas.FixtureSnapshot{}},
	}

	if targets := expectedTrueNASReplicationReadOnlyTargets(connections); len(targets) != 0 {
		t.Fatalf("ambiguous, hostless, and IGNORE tasks must fail closed, got %#v", targets)
	}
}

func TestExpectedTrueNASReplicationReadOnlyTargetsSupportLocalAndPullTasks(t *testing.T) {
	connections := []trueNASReplicationConnection{{
		connectionID:   "nas",
		configuredHost: "https://nas.example.test",
		snapshot: &truenas.FixtureSnapshot{
			ReplicationTasks: []truenas.ReplicationTask{
				{ID: "pull", Direction: "PULL", Transport: "SSH", ReadOnlyMode: "REQUIRE", TargetDataset: "tank/pull-replica"},
				{ID: "local", Direction: "PUSH", Transport: "LOCAL", ReadOnlyMode: "SET", TargetDataset: "tank/local-replica"},
			},
		},
	}}

	targets := expectedTrueNASReplicationReadOnlyTargets(connections)
	got := targets["nas"]
	if len(got) != 2 || got[0] != "tank/local-replica" || got[1] != "tank/pull-replica" {
		t.Fatalf("expected sorted local targets, got %#v", targets)
	}
}

func TestApplyTrueNASReplicationReadOnlyTargetsPreservesFaultStates(t *testing.T) {
	snapshot := &truenas.FixtureSnapshot{
		Datasets: []truenas.Dataset{
			{Name: "backup/replicas", Mounted: true, ReadOnly: true},
			{Name: "backup/replicas/apps", Mounted: true, ReadOnly: true},
			{Name: "backup/replicas/locked", Mounted: false, Locked: true, ReadOnly: true},
			{Name: "backup/unrelated", Mounted: true, ReadOnly: true},
		},
	}

	classified := applyTrueNASReplicationReadOnlyTargets(snapshot, []string{"backup/replicas"})
	if classified.Datasets[0].ReadOnlyReason != truenas.DatasetReadOnlyReplicationTarget ||
		classified.Datasets[1].ReadOnlyReason != truenas.DatasetReadOnlyReplicationTarget {
		t.Fatalf("expected root and descendants to carry replication posture: %+v", classified.Datasets)
	}
	if classified.Datasets[2].ReadOnlyReason != truenas.DatasetReadOnlyReplicationTarget || !classified.Datasets[2].Locked {
		t.Fatalf("expected lock state to survive classification: %+v", classified.Datasets[2])
	}
	if classified.Datasets[3].ReadOnlyReason != truenas.DatasetReadOnlyUnspecified {
		t.Fatalf("unexpected unrelated dataset classification: %+v", classified.Datasets[3])
	}
}

func TestTrueNASPollerRebuildProjectsReplicationReadonlyOnlyOnMatchedReceiver(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	newProvider := func(connectionID string, snapshot truenas.FixtureSnapshot) *truenas.Provider {
		t.Helper()
		provider := truenas.NewLiveProviderForConnection(&truenas.FixtureFetcher{Snapshot: snapshot}, connectionID)
		if err := provider.Refresh(context.Background()); err != nil {
			t.Fatalf("Refresh(%s) error = %v", connectionID, err)
		}
		return provider
	}

	sender := newProvider("sender", truenas.FixtureSnapshot{
		System: truenas.SystemInfo{Hostname: "sender", Healthy: true},
		ReplicationTasks: []truenas.ReplicationTask{{
			ID:            "7",
			Direction:     "PUSH",
			Transport:     "SSH",
			ReadOnlyMode:  "SET",
			TargetHost:    "backup.example.test",
			TargetDataset: "backup/replicas",
		}},
	})
	receiver := newProvider("receiver", truenas.FixtureSnapshot{
		System:   truenas.SystemInfo{Hostname: "backup", Healthy: true},
		Pools:    []truenas.Pool{{Name: "backup", Status: "ONLINE"}},
		Datasets: []truenas.Dataset{{Name: "backup/replicas", Pool: "backup", Mounted: true, ReadOnly: true}},
	})
	unrelated := newProvider("unrelated", truenas.FixtureSnapshot{
		System:   truenas.SystemInfo{Hostname: "unrelated", Healthy: true},
		Pools:    []truenas.Pool{{Name: "backup", Status: "ONLINE"}},
		Datasets: []truenas.Dataset{{Name: "backup/replicas", Pool: "backup", Mounted: true, ReadOnly: true}},
	})

	poller := NewTrueNASPoller(nil, time.Minute, nil)
	poller.providersByOrg["default"] = map[string]*truenas.Provider{
		"sender":    sender,
		"receiver":  receiver,
		"unrelated": unrelated,
	}
	poller.configsByOrg["default"] = map[string]config.TrueNASInstance{
		"sender":    {ID: "sender", Host: "sender.example.test"},
		"receiver":  {ID: "receiver", Host: "backup.example.test"},
		"unrelated": {ID: "unrelated", Host: "unrelated.example.test"},
	}

	poller.rebuildCachedRecordsForOrg("default")

	assertDataset := func(connectionID string, wantStatus unifiedresources.ResourceStatus, wantTag string) {
		t.Helper()
		for _, record := range poller.cachedRecordsByOrg["default"][connectionID] {
			if record.Resource.Name != "backup/replicas" {
				continue
			}
			if record.Resource.Status != wantStatus {
				t.Fatalf("%s dataset status = %q, want %q", connectionID, record.Resource.Status, wantStatus)
			}
			for _, tag := range record.Resource.Tags {
				if tag == wantTag {
					return
				}
			}
			t.Fatalf("%s dataset tags = %v, want %q", connectionID, record.Resource.Tags, wantTag)
		}
		t.Fatalf("%s dataset record not found", connectionID)
	}

	assertDataset("receiver", unifiedresources.StatusOnline, "state:replication-readonly")
	assertDataset("unrelated", unifiedresources.StatusWarning, "state:readonly")
}

func TestTrueNASPollerReadAppLogsUsesTenantScopedProvider(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	fixtures := truenas.DefaultFixtures()
	fetcher := &pollerControlFetcher{snapshot: &fixtures}
	provider := truenas.NewLiveProvider(fetcher)
	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	poller := NewTrueNASPoller(nil, 0, nil)
	poller.providersByOrg["default"] = map[string]*truenas.Provider{"conn-1": provider}
	poller.cachedRecordsByOrg["default"] = map[string][]unifiedresources.IngestRecord{"conn-1": provider.Records()}

	result, err := poller.ReadAppLogs(context.Background(), "default", "truenas-main", "nextcloud", "", 20)
	if err != nil {
		t.Fatalf("ReadAppLogs() error = %v", err)
	}
	if result == nil || result.App.Name != "Nextcloud" {
		t.Fatalf("expected Nextcloud log result, got %+v", result)
	}
	if result.Container.ID != "nextcloud-web-1" {
		t.Fatalf("expected canonical primary container, got %+v", result.Container)
	}
	if len(fetcher.logReads) != 1 {
		t.Fatalf("expected one log read, got %+v", fetcher.logReads)
	}
	if call := fetcher.logReads[0]; call.appName != "nextcloud" || call.containerID != "nextcloud-web-1" || call.tailLines != 20 {
		t.Fatalf("unexpected log read call: %+v", call)
	}
}

func TestTrueNASPollerGetAppConfigUsesTenantScopedProvider(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	fixtures := truenas.DefaultFixtures()
	fetcher := &pollerControlFetcher{snapshot: &fixtures}
	provider := truenas.NewLiveProvider(fetcher)
	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	poller := NewTrueNASPoller(nil, 0, nil)
	poller.providersByOrg["default"] = map[string]*truenas.Provider{"conn-1": provider}
	poller.cachedRecordsByOrg["default"] = map[string][]unifiedresources.IngestRecord{"conn-1": provider.Records()}

	result, err := poller.GetAppConfig(context.Background(), "default", "truenas-main", "nextcloud")
	if err != nil {
		t.Fatalf("GetAppConfig() error = %v", err)
	}
	if result == nil || result.App.Name != "Nextcloud" {
		t.Fatalf("expected Nextcloud config result, got %+v", result)
	}
	if result.Host != "truenas-main" {
		t.Fatalf("expected config host truenas-main, got %+v", result)
	}
	if len(result.App.Containers) != 2 {
		t.Fatalf("expected canonical app runtime shape, got %+v", result.App.Containers)
	}
}

func TestTrueNASPollerHandlesConnectionAddRemove(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	first := newTrueNASMockServer(t, "nas-one")
	second := newTrueNASMockServer(t, "nas-two")
	t.Cleanup(first.Close)
	t.Cleanup(second.Close)

	mtp, persistence := newTestTenantPersistence(t)
	connOne := trueNASInstanceForServer(t, "conn-1", first.URL(), true)
	connTwo := trueNASInstanceForServer(t, "conn-2", second.URL(), true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connOne}); err != nil {
		t.Fatalf("SaveTrueNASConfig() initial error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return pollerProviderCount(poller) == 1 && first.RequestCount() >= 5
	}, "expected first connection provider and successful poll cycle")

	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connOne, connTwo}); err != nil {
		t.Fatalf("SaveTrueNASConfig() add error = %v", err)
	}

	waitForCondition(t, 2*time.Second, func() bool {
		return pollerProviderCount(poller) == 2 && second.RequestCount() >= 5
	}, "expected second connection to be discovered and polled")

	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connTwo}); err != nil {
		t.Fatalf("SaveTrueNASConfig() remove error = %v", err)
	}

	waitForCondition(t, 2*time.Second, func() bool {
		return pollerProviderCount(poller) == 1 && !pollerHasProvider(poller, "conn-1")
	}, "expected removed connection provider to be pruned")

	poller.Stop()
	if hasTrueNASHostForOrg(poller, "default", "nas-one") {
		t.Fatal("expected first host resources to be removed after pruning provider")
	}
	if !hasTrueNASHostForOrg(poller, "default", "nas-two") {
		t.Fatal("expected second host resources to be ingested")
	}
}

func TestTrueNASPollerKeepsPoolHealthConnectionLocalAcrossMatchingAppliances(t *testing.T) {
	observedAt := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	fixture := func(state string) truenas.FixtureSnapshot {
		return truenas.FixtureSnapshot{
			CollectedAt: observedAt,
			System:      truenas.SystemInfo{Hostname: "truenas", Healthy: true},
			Pools: []truenas.Pool{{
				ID:     "7",
				GUID:   "shared-restored-guid",
				Name:   "tank",
				Status: state,
			}},
		}
	}
	provider := func(connectionID, state string) *truenas.Provider {
		p := truenas.NewLiveProviderForConnection(
			&truenas.FixtureFetcher{Snapshot: fixture(state)},
			connectionID,
		)
		if err := p.Refresh(context.Background()); err != nil {
			t.Fatalf("refresh %s: %v", connectionID, err)
		}
		return p
	}

	poller := NewTrueNASPoller(nil, 0, nil)
	poller.providersByOrg["default"] = map[string]*truenas.Provider{
		"conn-primary": provider("conn-primary", "DEGRADED"),
		"conn-dr":      provider("conn-dr", "ONLINE"),
	}
	poller.configsByOrg["default"] = map[string]config.TrueNASInstance{
		"conn-primary": {ID: "conn-primary", Host: "192.0.2.10", Enabled: true},
		"conn-dr":      {ID: "conn-dr", Host: "192.0.2.11", Enabled: true},
	}
	poller.rebuildCachedRecordsForOrg("default")

	records := poller.GetCurrentRecordsForOrg("default")
	poolSources := make(map[string]string)
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestRecords(unifiedresources.SourceTrueNAS, records)
	for _, record := range records {
		if record.Resource.Type != unifiedresources.ResourceTypeStorage ||
			record.Resource.Storage == nil ||
			record.Resource.Storage.Topology != "pool" {
			continue
		}
		health := record.Resource.Storage.PoolHealth
		if health == nil {
			t.Fatalf("pool health missing from %s: %+v", record.SourceID, record.Resource.Storage)
		}
		poolSources[record.SourceID] = health.CanonicalState
	}
	if len(poolSources) != 2 {
		t.Fatalf("connection-local pool records = %+v", poolSources)
	}
	if poolSources["system:conn-primary/pool:tank"] != "DEGRADED" ||
		poolSources["system:conn-dr/pool:tank"] != "ONLINE" {
		t.Fatalf("connection-local pool health = %+v", poolSources)
	}

	storageResources := 0
	ids := make(map[string]struct{})
	for _, resource := range registry.List() {
		if resource.Type != unifiedresources.ResourceTypeStorage || resource.Name != "tank" {
			continue
		}
		storageResources++
		ids[resource.ID] = struct{}{}
	}
	if storageResources != 2 || len(ids) != 2 {
		t.Fatalf("matching appliances merged pool identity: resources=%d ids=%+v", storageResources, ids)
	}
}

func copyTrueNASSnapshot(snapshot *truenas.FixtureSnapshot) *truenas.FixtureSnapshot {
	if snapshot == nil {
		return nil
	}
	cloned := *snapshot
	cloned.Pools = make([]truenas.Pool, len(snapshot.Pools))
	for i := range snapshot.Pools {
		cloned.Pools[i] = snapshot.Pools[i]
		if snapshot.Pools[i].Scan != nil {
			scan := *snapshot.Pools[i].Scan
			cloned.Pools[i].Scan = &scan
		}
		cloned.Pools[i].VDevs = append([]truenas.PoolVDev(nil), snapshot.Pools[i].VDevs...)
		cloned.Pools[i].DiskMembers = append([]truenas.PoolDiskMember(nil), snapshot.Pools[i].DiskMembers...)
	}
	cloned.Datasets = append([]truenas.Dataset(nil), snapshot.Datasets...)
	cloned.Disks = append([]truenas.Disk(nil), snapshot.Disks...)
	cloned.Alerts = append([]truenas.Alert(nil), snapshot.Alerts...)
	cloned.ZFSSnapshots = append([]truenas.ZFSSnapshot(nil), snapshot.ZFSSnapshots...)
	cloned.ReplicationTasks = append([]truenas.ReplicationTask(nil), snapshot.ReplicationTasks...)
	if snapshot.System.TemperatureCelsius != nil {
		cloned.System.TemperatureCelsius = make(map[string]float64, len(snapshot.System.TemperatureCelsius))
		for key, value := range snapshot.System.TemperatureCelsius {
			cloned.System.TemperatureCelsius[key] = value
		}
	}
	if len(snapshot.Apps) > 0 {
		cloned.Apps = make([]truenas.App, len(snapshot.Apps))
		for i, app := range snapshot.Apps {
			appCopy := app
			appCopy.UsedHostIPs = append([]string(nil), app.UsedHostIPs...)
			appCopy.UsedPorts = append([]truenas.AppPort(nil), app.UsedPorts...)
			appCopy.Volumes = append([]truenas.AppVolume(nil), app.Volumes...)
			appCopy.Images = append([]string(nil), app.Images...)
			appCopy.Networks = append([]truenas.AppNetwork(nil), app.Networks...)
			appCopy.Containers = append([]truenas.AppContainer(nil), app.Containers...)
			if app.Stats != nil {
				statsCopy := *app.Stats
				statsCopy.Interfaces = append([]truenas.AppInterfaceStats(nil), app.Stats.Interfaces...)
				appCopy.Stats = &statsCopy
			}
			cloned.Apps[i] = appCopy
		}
	}
	return &cloned
}

func TestTrueNASPollerAPITimeout(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	var requestCount atomic.Int64
	var injectDelay atomic.Bool
	var recoverySuccesses atomic.Int64
	injectDelay.Store(true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/v2.0/system/info" && injectDelay.Load() {
			time.Sleep(200 * time.Millisecond)
		}

		switch r.URL.Path {
		case "/api/v2.0/system/info":
			_, _ = w.Write([]byte(`{"hostname":"timeout-host","version":"TrueNAS-SCALE-24.10.2","buildtime":"24.10.2.1","uptime_seconds":86400,"system_serial":"SER-timeout-host"}`))
		case "/api/v2.0/pool":
			_, _ = w.Write([]byte(`[{"id":1,"name":"timeout-pool","status":"ONLINE","size":1000,"allocated":400,"free":600}]`))
		case "/api/v2.0/pool/dataset":
			_, _ = w.Write([]byte(`[]`))
		case "/api/v2.0/disk":
			_, _ = w.Write([]byte(`[]`))
		case "/api/v2.0/alert/list":
			if !injectDelay.Load() {
				recoverySuccesses.Add(1)
			}
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	mtp, persistence := newTestTenantPersistence(t)
	connection := trueNASInstanceForServer(t, "timeout-conn", server.URL, true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	injectTrueNASProviderTimeout(t, poller, connection, 75*time.Millisecond)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return requestCount.Load() >= 3
	}, "expected poller to continue retrying while API requests time out")

	injectDelay.Store(false)

	waitForCondition(t, 3*time.Second, func() bool {
		return recoverySuccesses.Load() > 0 && hasTrueNASHostForOrg(poller, "default", "timeout-host")
	}, "expected at least one successful poll cycle to ingest TrueNAS resources after timeout clears")

	poller.Stop()
	if !hasTrueNASHostForOrg(poller, "default", "timeout-host") {
		t.Fatal("expected poller to recover and ingest TrueNAS resources after timeout clears")
	}
}

func TestTrueNASPollerAuthFailure(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	var requestCount atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	t.Cleanup(server.Close)

	mtp, persistence := newTestTenantPersistence(t)
	connection := trueNASInstanceForServer(t, "auth-failure-conn", server.URL, true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return requestCount.Load() >= 2
	}, "expected at least two poll attempts with auth failures")

	before := requestCount.Load()
	waitForCondition(t, 2*time.Second, func() bool {
		return requestCount.Load() > before
	}, "expected poller to keep attempting after repeated auth failures")

	select {
	case <-poller.stopped:
		t.Fatal("expected poller to keep running after auth failures")
	default:
	}

	poller.Stop()
	if hasTrueNASHostForOrg(poller, "default", "auth-failure-host") {
		t.Fatal("expected no resources to be ingested when every poll fails auth")
	}
}

func TestTrueNASPollerStaleDataRecovery(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	const (
		initialSuccessPolls = int64(2)
		failurePolls        = int64(3)
	)

	var pollAttempts atomic.Int64
	var initialSuccesses atomic.Int64
	var recoverySuccesses atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		attempt := pollAttempts.Load()

		if r.URL.Path == "/api/v2.0/system/info" {
			attempt = pollAttempts.Add(1)
			switch {
			case attempt <= initialSuccessPolls:
				_, _ = w.Write([]byte(`{"hostname":"stale-before","version":"TrueNAS-SCALE-24.10.2","buildtime":"24.10.2.1","uptime_seconds":86400,"system_serial":"SER-stale-before"}`))
			case attempt <= initialSuccessPolls+failurePolls:
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"simulated outage"}`))
			default:
				_, _ = w.Write([]byte(`{"hostname":"stale-after","version":"TrueNAS-SCALE-24.10.2","buildtime":"24.10.2.1","uptime_seconds":86500,"system_serial":"SER-stale-after"}`))
			}
			return
		}

		if attempt <= initialSuccessPolls {
			switch r.URL.Path {
			case "/api/v2.0/pool":
				_, _ = w.Write([]byte(`[{"id":1,"name":"before-pool","status":"ONLINE","size":1000,"allocated":400,"free":600}]`))
			case "/api/v2.0/pool/dataset":
				_, _ = w.Write([]byte(`[]`))
			case "/api/v2.0/disk":
				_, _ = w.Write([]byte(`[]`))
			case "/api/v2.0/alert/list":
				initialSuccesses.Add(1)
				_, _ = w.Write([]byte(`[]`))
			default:
				http.NotFound(w, r)
			}
			return
		}

		switch r.URL.Path {
		case "/api/v2.0/pool":
			_, _ = w.Write([]byte(`[{"id":1,"name":"after-pool","status":"ONLINE","size":1000,"allocated":500,"free":500}]`))
		case "/api/v2.0/pool/dataset":
			_, _ = w.Write([]byte(`[]`))
		case "/api/v2.0/disk":
			_, _ = w.Write([]byte(`[]`))
		case "/api/v2.0/alert/list":
			recoverySuccesses.Add(1)
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	mtp, persistence := newTestTenantPersistence(t)
	connection := trueNASInstanceForServer(t, "stale-recovery-conn", server.URL, true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return initialSuccesses.Load() > 0
	}, "expected initial successful polls to ingest baseline resources")

	waitForCondition(t, 3*time.Second, func() bool {
		return pollAttempts.Load() >= initialSuccessPolls+failurePolls
	}, "expected poller to continue attempts throughout failure window")

	waitForCondition(t, 3*time.Second, func() bool {
		return recoverySuccesses.Load() > 0 && hasTrueNASHostForOrg(poller, "default", "stale-after")
	}, "expected poller to recover and ingest refreshed data after failures")

	poller.Stop()
	if !hasTrueNASHostForOrg(poller, "default", "stale-after") {
		t.Fatal("expected recovered TrueNAS host data to be ingested")
	}
}

func TestTrueNASPollerConnectionFlap(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	var requestCount atomic.Int64
	var isDown atomic.Bool
	var recovered atomic.Bool
	var beforeDownSuccesses atomic.Int64
	var afterRecoverySuccesses atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")

		if isDown.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"temporarily unavailable"}`))
			return
		}

		hostname := "flap-before"
		if recovered.Load() {
			hostname = "flap-after"
		}

		switch r.URL.Path {
		case "/api/v2.0/system/info":
			_, _ = w.Write([]byte(`{"hostname":"` + hostname + `","version":"TrueNAS-SCALE-24.10.2","buildtime":"24.10.2.1","uptime_seconds":86400,"system_serial":"SER-` + hostname + `"}`))
		case "/api/v2.0/pool":
			_, _ = w.Write([]byte(`[{"id":1,"name":"flap-pool","status":"ONLINE","size":1000,"allocated":400,"free":600}]`))
		case "/api/v2.0/pool/dataset":
			_, _ = w.Write([]byte(`[]`))
		case "/api/v2.0/disk":
			_, _ = w.Write([]byte(`[]`))
		case "/api/v2.0/alert/list":
			if recovered.Load() {
				afterRecoverySuccesses.Add(1)
			} else {
				beforeDownSuccesses.Add(1)
			}
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	mtp, persistence := newTestTenantPersistence(t)
	connection := trueNASInstanceForServer(t, "connection-flap-conn", server.URL, true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return beforeDownSuccesses.Load() > 0
	}, "expected initial TrueNAS ingest before simulated outage")

	isDown.Store(true)
	startedDownAt := requestCount.Load()
	waitForCondition(t, 2*time.Second, func() bool {
		return requestCount.Load() >= startedDownAt+3
	}, "expected poller to continue making requests while endpoint is down")

	recovered.Store(true)
	isDown.Store(false)

	waitForCondition(t, 3*time.Second, func() bool {
		return afterRecoverySuccesses.Load() > 0 && hasTrueNASHostForOrg(poller, "default", "flap-after")
	}, "expected poller to recover ingestion after endpoint returns")

	poller.Stop()
	if !hasTrueNASHostForOrg(poller, "default", "flap-after") {
		t.Fatal("expected recovered endpoint data to be ingested")
	}
}

func TestTrueNASPollerConcurrentConfigChange(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	first := newTrueNASMockServer(t, "config-change-one")
	second := newTrueNASMockServer(t, "config-change-two")
	t.Cleanup(first.Close)
	t.Cleanup(second.Close)

	mtp, persistence := newTestTenantPersistence(t)
	connOne := trueNASInstanceForServer(t, "config-change-1", first.URL(), true)
	connTwo := trueNASInstanceForServer(t, "config-change-2", second.URL(), true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connOne}); err != nil {
		t.Fatalf("SaveTrueNASConfig() initial error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return pollerProviderCount(poller) == 1 && pollerHasProvider(poller, connOne.ID) && first.RequestCount() >= 5
	}, "expected first connection to be active before config updates")

	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connOne, connTwo}); err != nil {
		t.Fatalf("SaveTrueNASConfig() add error = %v", err)
	}

	waitForCondition(t, 2*time.Second, func() bool {
		return pollerProviderCount(poller) == 2 && pollerHasProvider(poller, connOne.ID) && pollerHasProvider(poller, connTwo.ID) && second.RequestCount() >= 5
	}, "expected second connection to appear while poller is running")

	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connTwo}); err != nil {
		t.Fatalf("SaveTrueNASConfig() remove error = %v", err)
	}

	waitForCondition(t, 2*time.Second, func() bool {
		return pollerProviderCount(poller) == 1 && !pollerHasProvider(poller, connOne.ID) && pollerHasProvider(poller, connTwo.ID)
	}, "expected provider map to converge after removing first connection")

	poller.Stop()
	if !hasTrueNASHostForOrg(poller, "default", "config-change-two") {
		t.Fatal("expected second connection resources to be ingested")
	}
}

func TestTrueNASPollerRebindsProviderWhenConnectionConfigChanges(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	first := newTrueNASMockServer(t, "config-rebind-one")
	second := newTrueNASMockServer(t, "config-rebind-two")
	t.Cleanup(first.Close)
	t.Cleanup(second.Close)

	mtp, persistence := newTestTenantPersistence(t)
	connection := trueNASInstanceForServer(t, "config-rebind", first.URL(), true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() initial error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return pollerProviderCount(poller) == 1 && first.RequestCount() >= 5
	}, "expected initial TrueNAS connection to poll before config change")

	updated := trueNASInstanceForServer(t, connection.ID, second.URL(), true)
	updated.Name = connection.Name
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{updated}); err != nil {
		t.Fatalf("SaveTrueNASConfig() updated error = %v", err)
	}

	waitForCondition(t, 2*time.Second, func() bool {
		return pollerProviderCount(poller) == 1 && second.RequestCount() >= 5
	}, "expected provider to rebind to updated TrueNAS endpoint")

	firstCountAfterRebind := first.RequestCount()
	noPollDeadline := time.Now().Add(200 * time.Millisecond)
	waitForCondition(t, 500*time.Millisecond, func() bool {
		return time.Now().After(noPollDeadline) && first.RequestCount() == firstCountAfterRebind
	}, "expected replaced TrueNAS endpoint to stop receiving poll requests")

	poller.Stop()
	if hasTrueNASHostForOrg(poller, "default", "config-rebind-one") {
		t.Fatal("expected old TrueNAS host to be replaced after config rebind")
	}
	if !hasTrueNASHostForOrg(poller, "default", "config-rebind-two") {
		t.Fatal("expected updated TrueNAS host to be ingested after config rebind")
	}
}

func TestTrueNASPollerSkipsDisabledConnections(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	enabled := newTrueNASMockServer(t, "nas-enabled")
	disabled := newTrueNASMockServer(t, "nas-disabled")
	t.Cleanup(enabled.Close)
	t.Cleanup(disabled.Close)

	mtp, persistence := newTestTenantPersistence(t)
	enabledConn := trueNASInstanceForServer(t, "conn-enabled", enabled.URL(), true)
	disabledConn := trueNASInstanceForServer(t, "conn-disabled", disabled.URL(), false)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{enabledConn, disabledConn}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return pollerProviderCount(poller) == 1 &&
			enabled.RequestCount() >= 5 &&
			hasTrueNASHostForOrg(poller, "default", "nas-enabled")
	}, "expected only enabled connection provider and resources")

	waitForCondition(t, 2*time.Second, func() bool {
		return enabled.RequestCount() >= 10 &&
			hasTrueNASHostForOrg(poller, "default", "nas-enabled")
	}, "expected additional polling cycles for enabled connection")

	if disabled.RequestCount() != 0 {
		t.Fatalf("expected disabled connection to be skipped, got %d requests", disabled.RequestCount())
	}
	poller.Stop()
	if !hasTrueNASHostForOrg(poller, "default", "nas-enabled") {
		t.Fatal("expected enabled connection host to be present in cached records")
	}
	if hasTrueNASHostForOrg(poller, "default", "nas-disabled") {
		t.Fatal("expected disabled connection host to be absent from cached records")
	}
}

func TestTrueNASPollerCachesRecordsPerOrganization(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	defaultOrgMock := newTrueNASMockServer(t, "default-nas")
	tenantMock := newTrueNASMockServer(t, "tenant-nas")
	t.Cleanup(defaultOrgMock.Close)
	t.Cleanup(tenantMock.Close)

	mtp, defaultPersistence := newTestTenantPersistence(t)
	defaultConn := trueNASInstanceForServer(t, "default-conn", defaultOrgMock.URL(), true)
	if err := defaultPersistence.SaveTrueNASConfig([]config.TrueNASInstance{defaultConn}); err != nil {
		t.Fatalf("SaveTrueNASConfig(default) error = %v", err)
	}

	tenantPersistence, err := mtp.GetPersistence("org-a")
	if err != nil {
		t.Fatalf("GetPersistence(org-a) error = %v", err)
	}
	tenantConn := trueNASInstanceForServer(t, "tenant-conn", tenantMock.URL(), true)
	if err := tenantPersistence.SaveTrueNASConfig([]config.TrueNASInstance{tenantConn}); err != nil {
		t.Fatalf("SaveTrueNASConfig(org-a) error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return defaultOrgMock.RequestCount() >= 5 &&
			tenantMock.RequestCount() >= 5 &&
			hasTrueNASHostForOrg(poller, "default", "default-nas") &&
			hasTrueNASHostForOrg(poller, "org-a", "tenant-nas")
	}, "expected polling for both default and org-a TrueNAS connections and cached records for each org")

	poller.Stop()
	if !hasTrueNASHostForOrg(poller, "default", "default-nas") {
		t.Fatal("expected default org records to include default host")
	}
	if hasTrueNASHostForOrg(poller, "default", "tenant-nas") {
		t.Fatal("expected default org records to exclude tenant host")
	}
	if !hasTrueNASHostForOrg(poller, "org-a", "tenant-nas") {
		t.Fatal("expected tenant records to include tenant host")
	}
}

func TestTrueNASPollerStopsCleanly(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	mtp, _ := newTestTenantPersistence(t)
	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	poller.Stop()

	select {
	case <-poller.stopped:
	case <-time.After(time.Second):
		t.Fatal("expected poller stopped channel to close")
	}
}

func TestTrueNASPollerSnapshotOwnedSources(t *testing.T) {
	poller := NewTrueNASPoller(nil, time.Second, nil)

	defaultSources := poller.SnapshotOwnedSources()
	if len(defaultSources) != 1 || defaultSources[0] != unifiedresources.SourceTrueNAS {
		t.Fatalf("default owned sources = %#v, want [%q]", defaultSources, unifiedresources.SourceTrueNAS)
	}

	orgSources := poller.SnapshotOwnedSourcesForOrg("org-a")
	if len(orgSources) != 1 || orgSources[0] != unifiedresources.SourceTrueNAS {
		t.Fatalf("org owned sources = %#v, want [%q]", orgSources, unifiedresources.SourceTrueNAS)
	}
}

func TestTrueNASPollerSupplementalInventoryReadyAtUsesPersistedActiveConnections(t *testing.T) {
	mtp, persistence := newTestTenantPersistence(t)

	connection := config.NewTrueNASInstance()
	connection.ID = "conn-ready"
	connection.Host = "nas-ready.lab.local"
	connection.APIKey = "api-key"
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, time.Second, nil)

	if readyAt, settled := poller.SupplementalInventoryReadyAt(nil, "default"); settled || !readyAt.IsZero() {
		t.Fatalf("SupplementalInventoryReadyAt() before any attempt = (%v, %t), want (zero, false)", readyAt, settled)
	}

	attemptedAt := time.Now().UTC()
	poller.mu.Lock()
	poller.recordConnectionSuccessLocked("default", connection.ID, connection, attemptedAt, attemptedAt, &truenas.FixtureSnapshot{CollectedAt: attemptedAt})
	poller.mu.Unlock()

	readyAt, settled := poller.SupplementalInventoryReadyAt(nil, "default")
	if !settled {
		t.Fatal("expected readiness to settle after the first recorded attempt")
	}
	if !readyAt.Equal(attemptedAt) {
		t.Fatalf("SupplementalInventoryReadyAt() = %v, want %v", readyAt, attemptedAt)
	}
}

func TestTrueNASPollerSyncConnectionsLogsStructuredContextWhenPersistenceNil(t *testing.T) {
	logOutput := captureTrueNASPollerLogs(t)

	poller := NewTrueNASPoller(nil, time.Second, nil)
	poller.syncConnections()

	for _, expected := range []string{
		`"level":"warn"`,
		`"component":"truenas_poller"`,
		`"action":"sync_connections"`,
		`"message":"TrueNAS poller cannot sync connections because multi-tenant persistence is nil"`,
	} {
		if !strings.Contains(logOutput.String(), expected) {
			t.Fatalf("expected log output to include %s, got %q", expected, logOutput.String())
		}
	}
}

func TestTrueNASPollerPollAllLogsStructuredContextOnRefreshFailure(t *testing.T) {
	logOutput := captureTrueNASPollerLogs(t)

	poller := NewTrueNASPoller(nil, time.Second, nil)
	poller.mu.Lock()
	if poller.configsByOrg == nil {
		poller.configsByOrg = make(map[string]map[string]config.TrueNASInstance)
	}
	if poller.configsByOrg["default"] == nil {
		poller.configsByOrg["default"] = make(map[string]config.TrueNASInstance)
	}
	connection := config.NewTrueNASInstance()
	connection.ID = "conn-refresh-fail"
	connection.Host = "nas-refresh-fail.lab.local"
	connection.APIKey = "api-key"
	poller.configsByOrg["default"][connection.ID] = connection
	if poller.providersByOrg == nil {
		poller.providersByOrg = make(map[string]map[string]*truenas.Provider)
	}
	if poller.providersByOrg["default"] == nil {
		poller.providersByOrg["default"] = make(map[string]*truenas.Provider)
	}
	poller.providersByOrg["default"][connection.ID] = truenas.NewLiveProvider(failingTrueNASFetcher{err: fmt.Errorf("refresh exploded")})
	poller.mu.Unlock()

	poller.pollAll(context.Background())

	for _, expected := range []string{
		`"level":"warn"`,
		`"component":"truenas_poller"`,
		`"action":"refresh_connection"`,
		`"connection_id":"conn-refresh-fail"`,
		`"error":"refresh truenas snapshot: refresh exploded"`,
		`"message":"TrueNAS poller refresh failed"`,
	} {
		if !strings.Contains(logOutput.String(), expected) {
			t.Fatalf("expected log output to include %s, got %q", expected, logOutput.String())
		}
	}
}

func TestClassifyTrueNASError(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		expectedType  string
		expectedRetry bool
	}{
		{
			name:         "nil error returns nil",
			err:          nil,
			expectedType: "",
		},
		{
			name:          "APIError 401 classifies as auth",
			err:           &truenas.APIError{StatusCode: 401, Method: "GET", Path: "/system/info", Body: "Unauthorized"},
			expectedType:  "auth",
			expectedRetry: false,
		},
		{
			name:          "APIError 403 classifies as auth",
			err:           &truenas.APIError{StatusCode: 403, Method: "GET", Path: "/pool", Body: "Forbidden"},
			expectedType:  "auth",
			expectedRetry: false,
		},
		{
			name:          "APIError 500 classifies as api",
			err:           &truenas.APIError{StatusCode: 500, Method: "GET", Path: "/pool", Body: "Internal Server Error"},
			expectedType:  "api",
			expectedRetry: true,
		},
		{
			name:          "APIError 408 classifies as timeout",
			err:           &truenas.APIError{StatusCode: 408, Method: "GET", Path: "/system/info", Body: "Request Timeout"},
			expectedType:  "timeout",
			expectedRetry: true,
		},
		{
			name:          "APIError 504 classifies as timeout",
			err:           &truenas.APIError{StatusCode: 504, Method: "GET", Path: "/pool", Body: "Gateway Timeout"},
			expectedType:  "timeout",
			expectedRetry: true,
		},
		{
			name:          "wrapped APIError 401 classifies as auth",
			err:           fmt.Errorf("fetch truenas system info: %w", &truenas.APIError{StatusCode: 401, Method: "GET", Path: "/system/info", Body: "Unauthorized"}),
			expectedType:  "auth",
			expectedRetry: false,
		},
		{
			name:          "websocket handshake 403 classifies as auth",
			err:           &truenas.RPCHandshakeError{StatusCode: 403, Err: fmt.Errorf("forbidden")},
			expectedType:  "auth",
			expectedRetry: false,
		},
		{
			name: "JSON-RPC login response classifies as auth",
			err: &truenas.RPCAuthError{
				Mechanism:    "api-key-plain",
				ResponseType: "AUTH_ERR",
			},
			expectedType:  "auth",
			expectedRetry: false,
		},
		{
			name: "JSON-RPC permission error classifies as auth",
			err: &truenas.RPCError{
				Code:    -32001,
				Method:  "pool.query",
				Message: "Method call error",
				Reason:  "Not authorized",
				Errname: "EACCES",
			},
			expectedType:  "auth",
			expectedRetry: false,
		},
		{
			name:          "context.DeadlineExceeded classifies as timeout",
			err:           context.DeadlineExceeded,
			expectedType:  "timeout",
			expectedRetry: true,
		},
		{
			name:          "wrapped context.DeadlineExceeded classifies as timeout",
			err:           fmt.Errorf("fetch truenas system info: %w", context.DeadlineExceeded),
			expectedType:  "timeout",
			expectedRetry: true,
		},
		{
			name:          "url.Error with timeout classifies as timeout",
			err:           &url.Error{Op: "Get", URL: "https://truenas.local/api/v2.0/system/info", Err: context.DeadlineExceeded},
			expectedType:  "timeout",
			expectedRetry: true,
		},
		{
			name: "WebSocket socket deadline classifies as timeout",
			err: &truenas.RPCTransportError{Method: "system.info", Phase: "read", Err: &net.OpError{
				Op: "read", Net: "tcp", Err: os.ErrDeadlineExceeded,
			}},
			expectedType:  "timeout",
			expectedRetry: true,
		},
		{
			name:          "net.OpError classifies as connection",
			err:           &net.OpError{Op: "dial", Net: "tcp", Addr: nil, Err: fmt.Errorf("connection refused")},
			expectedType:  "connection",
			expectedRetry: true,
		},
		{
			name:          "wrapped net.OpError classifies as connection",
			err:           fmt.Errorf("truenas request GET /system/info failed: %w", &net.OpError{Op: "dial", Net: "tcp", Addr: nil, Err: fmt.Errorf("connection refused")}),
			expectedType:  "connection",
			expectedRetry: true,
		},
		{
			name:          "plain error classifies as api fallback",
			err:           fmt.Errorf("some unknown error"),
			expectedType:  "api",
			expectedRetry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyTrueNASError(tt.err, "test-conn")

			if tt.err == nil {
				if result != nil {
					t.Fatalf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil MonitorError")
			}

			if string(result.Type) != tt.expectedType {
				t.Errorf("expected type %q, got %q", tt.expectedType, result.Type)
			}
			if result.Retryable != tt.expectedRetry {
				t.Errorf("expected retryable=%v, got %v", tt.expectedRetry, result.Retryable)
			}
			if result.Instance != "test-conn" {
				t.Errorf("expected instance %q, got %q", "test-conn", result.Instance)
			}
			if result.Op != "truenas_poll" {
				t.Errorf("expected op %q, got %q", "truenas_poll", result.Op)
			}
		})
	}
}

func TestTrueNASPollerUnresponsiveRPCDoesNotFreezeOtherConnections(t *testing.T) {
	var stalled atomic.Bool
	release := make(chan struct{})
	newServer := func(hostname string, mayStall bool) *httptest.Server {
		upgrader := websocket.Upgrader{}
		return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/current" {
				t.Error("modern polling fell back to REST")
				http.NotFound(w, r)
				return
			}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			for {
				var request struct {
					ID     int64             `json:"id"`
					Method string            `json:"method"`
					Params []json.RawMessage `json:"params"`
				}
				if err := conn.ReadJSON(&request); err != nil {
					return
				}
				var result any = []any{}
				switch request.Method {
				case "auth.login_ex":
					result = map[string]any{"response_type": "SUCCESS"}
				case "system.info":
					if mayStall && stalled.Load() {
						<-release
					}
					result = map[string]any{"hostname": hostname, "version": "TrueNAS-SCALE-25.10.7", "system_serial": hostname}
				case "core.subscribe":
					result = "fixture-realtime"
				case "core.unsubscribe":
					result = nil
				}
				if err := conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}); err != nil {
					return
				}
				if request.Method == "core.subscribe" {
					var event string
					if len(request.Params) != 1 || json.Unmarshal(request.Params[0], &event) != nil {
						t.Error("subscription did not supply one event")
						return
					}
					var fields any = map[string]any{"cpu": map[string]any{"usage": 12}}
					if strings.HasPrefix(event, "app.stats:") {
						fields = []any{}
					}
					if err := conn.WriteJSON(map[string]any{
						"jsonrpc": "2.0", "method": "collection_update",
						"params": map[string]any{"collection": event, "fields": fields},
					}); err != nil {
						return
					}
				}
			}
		}))
	}
	brokenServer := newServer("timeout-nas", true)
	healthyServer := newServer("healthy-nas", false)
	t.Cleanup(brokenServer.Close)
	t.Cleanup(healthyServer.Close)
	poller := NewTrueNASPoller(nil, 0, nil)
	instances := []config.TrueNASInstance{
		{ID: "timeout-connection", Host: brokenServer.URL, Enabled: true},
		{ID: "healthy-connection", Host: healthyServer.URL, Enabled: true},
	}
	poller.providersByOrg["default"] = make(map[string]*truenas.Provider)
	poller.configsByOrg["default"] = make(map[string]config.TrueNASInstance)
	for i, instance := range instances {
		server := []*httptest.Server{brokenServer, healthyServer}[i]
		client, err := truenas.NewClient(truenas.ClientConfig{
			Host: server.URL, APIKey: "fixture-key", Username: "fixture-user", Timeout: 200 * time.Millisecond,
			InsecureSkipVerify: true, Fingerprint: fmt.Sprintf("%x", sha256.Sum256(server.Certificate().Raw)),
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(client.Close)
		poller.providersByOrg["default"][instance.ID] = truenas.NewLiveProviderForConnection(&truenas.APIFetcher{Client: client}, instance.ID)
		poller.configsByOrg["default"][instance.ID] = instance
	}
	t.Cleanup(func() { close(release) })
	due := func() {
		for _, instance := range instances {
			poller.ensureConnectionRuntimeStatusLocked("default", instance.ID).nextPollAt = time.Now().Add(-time.Second)
		}
	}
	poller.pollAll(context.Background())
	before := poller.ConnectionSummaries("default", instances)
	for _, instance := range instances {
		if before[instance.ID].Poll.LastSuccessAt == nil {
			t.Fatal("initial successful poll was not established")
		}
	}
	stalled.Store(true)
	due()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { poller.pollAll(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("one unresponsive RPC froze the shared poll cycle")
		cancel() // Safely terminate the pre-repair adverse control.
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("poll cycle did not stop after cancellation")
		}
	}
	after := poller.ConnectionSummaries("default", instances)
	broken, healthy := after[instances[0].ID], after[instances[1].ID]
	if broken.Poll.LastError == nil || broken.Poll.LastError.Category != "timeout" || broken.Poll.ConsecutiveFailures != 1 || !broken.Poll.LastSuccessAt.Equal(*before[instances[0].ID].Poll.LastSuccessAt) {
		t.Fatalf("failed connection lost truthful timeout or previous success: %+v", broken)
	}
	if healthy.Poll.LastError != nil || !healthy.Poll.LastSuccessAt.After(*before[instances[1].ID].Poll.LastSuccessAt) {
		t.Fatalf("healthy connection did not continue polling: %+v", healthy)
	}
	if !hasTrueNASHostForOrg(poller, "default", "timeout-nas") || !hasTrueNASHostForOrg(poller, "default", "healthy-nas") {
		t.Fatal("timeout discarded a cached host identity")
	}
	status := poller.statusByOrg["default"][instances[0].ID]
	if !status.nextPollAt.Equal(status.lastAttemptAt.Add(defaultTrueNASPollInterval)) {
		t.Fatal("timeout changed completion-based failure backoff")
	}
	stalled.Store(false)
	due()
	poller.pollAll(context.Background())
	recovered := poller.ConnectionSummaries("default", instances)[instances[0].ID]
	if recovered.Poll.LastError != nil || recovered.Poll.ConsecutiveFailures != 0 || !recovered.Poll.LastSuccessAt.After(*broken.Poll.LastSuccessAt) || recovered.Transport == nil || !recovered.Transport.Connected || recovered.Transport.Mode != truenas.TransportJSONRPC {
		t.Fatalf("next poll did not recover the original connection over RPC: %+v", recovered)
	}
}

// #2396's nonempty Apps path: an accepted subscription is then rejected by
// middlewared. Keep the inventory, advance the poll ledger, observe native
// alert disappearance and recover stats on the next ordinary poll. This is a
// protocol/poller control, not a native appliance or incident-delivery proof.
func TestTrueNASPollerSubscriptionRejectionKeepsPollingAndRecovers(t *testing.T) {
	var reject atomic.Bool
	var sessions, statsSubscriptions atomic.Int32
	upgrader := websocket.Upgrader{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/current" {
			t.Error("modern polling fell back to REST")
			http.NotFound(w, r)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		sessions.Add(1)
		for {
			var request struct {
				ID     int64             `json:"id"`
				Method string            `json:"method"`
				Params []json.RawMessage `json:"params"`
			}
			if conn.ReadJSON(&request) != nil {
				return
			}
			var result any = []any{}
			var collection string
			switch request.Method {
			case "auth.login_ex":
				result = map[string]any{"response_type": "SUCCESS"}
			case "system.info":
				result = map[string]any{"hostname": "reject-nas", "version": "TrueNAS-SCALE-25.04.2.6", "system_serial": "REJECT-FIXTURE"}
			case "pool.query":
				result = []map[string]any{{"id": 1, "name": "tank", "status": "ONLINE", "size": 1000, "allocated": 400}}
			case "app.query":
				result = []map[string]any{{"id": "fixture-app", "name": "fixture-app", "state": "RUNNING"}}
			case "alert.list":
				if !reject.Load() {
					result = []map[string]any{{"id": "fixture-alert", "level": "WARNING", "formatted": "fixture warning", "source": "fixture"}}
				}
			case "core.subscribe":
				if len(request.Params) != 1 || json.Unmarshal(request.Params[0], &collection) != nil {
					t.Error("invalid subscription")
					return
				}
				result = "fixture-sub"
			case "core.unsubscribe":
				result = nil
			}
			if conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}) != nil {
				return
			}
			if collection == "" {
				continue
			}
			var params any
			method := "collection_update"
			if strings.HasPrefix(collection, "app.stats:") {
				statsSubscriptions.Add(1)
				if reject.Load() {
					method = "notify_unsubscribed"
					params = map[string]any{"collection": collection, "error": map[string]any{"error": 14, "errname": "EFAULT", "reason": "[EFAULT] Apps are not available", "trace": nil, "extra": nil}}
				} else {
					params = map[string]any{"collection": collection, "fields": []any{map[string]any{"app_name": "fixture-app", "cpu_usage": 12}}}
				}
			} else {
				params = map[string]any{"collection": collection, "fields": map[string]any{"cpu": map[string]any{"usage": 10}}}
			}
			if conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "method": method, "params": params}) != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)
	client, err := truenas.NewClient(truenas.ClientConfig{
		Host: server.URL, APIKey: "fixture-key", Username: "fixture-user", Timeout: 2 * time.Second,
		InsecureSkipVerify: true, Fingerprint: fmt.Sprintf("%x", sha256.Sum256(server.Certificate().Raw)),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	instance := config.TrueNASInstance{ID: "reject-connection", Host: server.URL, Enabled: true}
	provider := truenas.NewLiveProviderForConnection(&truenas.APIFetcher{Client: client}, instance.ID)
	poller := NewTrueNASPoller(nil, 0, nil)
	poller.providersByOrg["default"] = map[string]*truenas.Provider{instance.ID: provider}
	poller.configsByOrg["default"] = map[string]config.TrueNASInstance{instance.ID: instance}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var lastSuccess time.Time
	for poll := 0; poll < 3; poll++ {
		reject.Store(poll == 1)
		poller.ensureConnectionRuntimeStatusLocked("default", instance.ID).nextPollAt = time.Now().Add(-time.Second)
		started := time.Now()
		poller.pollAll(ctx)
		if elapsed := time.Since(started); elapsed >= time.Second {
			t.Fatalf("rejection stalled poll %d for %s", poll, elapsed)
		}
		summary := poller.ConnectionSummaries("default", []config.TrueNASInstance{instance})[instance.ID]
		if summary.Poll.LastError != nil || summary.Poll.LastSuccessAt == nil || !summary.Poll.LastSuccessAt.After(lastSuccess) || summary.Poll.LastAttemptAt == nil {
			t.Fatalf("optional stats rejection prevented poll progress: %+v", summary)
		}
		lastSuccess = *summary.Poll.LastSuccessAt
		snapshot := provider.Snapshot()
		if snapshot == nil || snapshot.System.Hostname != "reject-nas" || len(snapshot.Pools) != 1 || len(snapshot.Apps) != 1 || snapshot.System.CPUPercent != 10 {
			t.Fatalf("poll %d lost usable inventory/telemetry: %+v", poll, snapshot)
		}
		if poll == 1 {
			if snapshot.Apps[0].Stats != nil || len(snapshot.Alerts) != 0 {
				t.Fatal("rejected stats fabricated data or left the former native alert")
			}
		} else if snapshot.Apps[0].Stats == nil || snapshot.Apps[0].Stats.CPUPercent != 12 || len(snapshot.Alerts) != 1 {
			t.Fatal("healthy stats/native alerts did not return")
		}
		if !hasTrueNASHostForOrg(poller, "default", "reject-nas") {
			t.Fatal("poll lost the appliance identity")
		}
	}
	if statsSubscriptions.Load() != 3 || sessions.Load() != 2 {
		t.Fatalf("rejection replayed or churned later polls: subscriptions=%d sessions=%d", statsSubscriptions.Load(), sessions.Load())
	}
}

type trueNASMockServer struct {
	server   *httptest.Server
	requests atomic.Int64
}

func newTrueNASMockServer(t *testing.T, hostname string) *trueNASMockServer {
	t.Helper()

	mock := &trueNASMockServer{}
	poolName := "pool-" + hostname

	mock.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mock.requests.Add(1)
		writer.Header().Set("Content-Type", "application/json")

		switch request.URL.Path {
		case "/api/v2.0/system/info":
			_, _ = writer.Write([]byte(`{"hostname":"` + hostname + `","version":"TrueNAS-SCALE-24.10.2","buildtime":"24.10.2.1","uptime_seconds":86400,"system_serial":"SER-` + hostname + `"}`))
		case "/api/v2.0/pool":
			_, _ = writer.Write([]byte(`[{"id":1,"name":"` + poolName + `","status":"ONLINE","size":1000,"allocated":400,"free":600}]`))
		case "/api/v2.0/pool/dataset":
			_, _ = writer.Write([]byte(`[{"id":"` + poolName + `/apps","name":"` + poolName + `/apps","pool":"` + poolName + `","used":{"rawvalue":"12345","parsed":12345},"available":{"rawvalue":"555","parsed":555},"mountpoint":"/mnt/` + poolName + `/apps","readonly":{"rawvalue":"off","parsed":false},"mounted":true}]`))
		case "/api/v2.0/disk":
			_, _ = writer.Write([]byte(`[{"identifier":"{disk-1}","name":"sda","serial":"SER-A","size":1000000,"model":"Seagate","type":"HDD","pool":"` + poolName + `","bus":"SATA","rotationrate":7200,"status":"ONLINE"}]`))
		case "/api/v2.0/alert/list":
			_, _ = writer.Write([]byte(`[{"id":"a1","level":"WARNING","formatted":"Disk temp high","source":"DiskService","dismissed":false,"datetime":{"$date":1707400000000}}]`))
		default:
			http.NotFound(writer, request)
		}
	}))

	return mock
}

func (m *trueNASMockServer) URL() string {
	return m.server.URL
}

func (m *trueNASMockServer) Close() {
	if m != nil && m.server != nil {
		m.server.Close()
	}
}

func (m *trueNASMockServer) RequestCount() int64 {
	if m == nil {
		return 0
	}
	return m.requests.Load()
}

func trueNASInstanceForServer(t *testing.T, id string, rawURL string, enabled bool) config.TrueNASInstance {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", rawURL, err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatalf("parse port from %q error = %v", rawURL, err)
	}

	return config.TrueNASInstance{
		ID:       id,
		Name:     "connection-" + id,
		Host:     parsed.Hostname(),
		Port:     port,
		APIKey:   "test-api-key",
		UseHTTPS: strings.EqualFold(parsed.Scheme, "https"),
		Enabled:  enabled,
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, failureMessage string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(failureMessage)
}

func newTestTenantPersistence(t *testing.T) (*config.MultiTenantPersistence, *config.ConfigPersistence) {
	t.Helper()

	mtp := config.NewMultiTenantPersistence(t.TempDir())
	persistence, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("GetPersistence(default) error = %v", err)
	}
	return mtp, persistence
}

func hasTrueNASHostForOrg(poller *TrueNASPoller, orgID, hostname string) bool {
	if poller == nil {
		return false
	}

	records := poller.GetCurrentRecordsForOrg(orgID)
	if len(records) == 0 {
		return false
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestRecords(unifiedresources.SourceTrueNAS, records)
	return hasTrueNASHost(registry, hostname)
}

func hasTrueNASHost(registry *unifiedresources.ResourceRegistry, hostname string) bool {
	if registry == nil {
		return false
	}

	resources := registry.List()
	for _, resource := range resources {
		if resource.Type != unifiedresources.ResourceTypeAgent || resource.Name != hostname {
			continue
		}
		if resourceHasSource(resource, unifiedresources.SourceTrueNAS) {
			return true
		}
	}
	return false
}

func resourceHasSource(resource unifiedresources.Resource, source unifiedresources.DataSource) bool {
	for _, candidate := range resource.Sources {
		if candidate == source {
			return true
		}
	}
	return false
}

func pollerProviderCount(poller *TrueNASPoller) int {
	if poller == nil {
		return 0
	}
	poller.mu.Lock()
	defer poller.mu.Unlock()
	total := 0
	for _, providers := range poller.providersByOrg {
		total += len(providers)
	}
	return total
}

func pollerHasProvider(poller *TrueNASPoller, id string) bool {
	if poller == nil {
		return false
	}
	poller.mu.Lock()
	defer poller.mu.Unlock()
	providers := poller.providersByOrg["default"]
	if providers == nil {
		return false
	}
	_, ok := providers[id]
	return ok
}

func injectTrueNASProviderTimeout(t *testing.T, poller *TrueNASPoller, instance config.TrueNASInstance, timeout time.Duration) {
	t.Helper()

	client, err := truenas.NewClient(truenas.ClientConfig{
		Host:               instance.Host,
		Port:               instance.Port,
		APIKey:             instance.APIKey,
		Username:           instance.Username,
		Password:           instance.Password,
		UseHTTPS:           instance.UseHTTPS,
		InsecureSkipVerify: instance.InsecureSkipVerify,
		Fingerprint:        instance.Fingerprint,
		Timeout:            timeout,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	poller.mu.Lock()
	defer poller.mu.Unlock()
	if poller.providersByOrg == nil {
		poller.providersByOrg = make(map[string]map[string]*truenas.Provider)
	}
	if poller.providersByOrg["default"] == nil {
		poller.providersByOrg["default"] = make(map[string]*truenas.Provider)
	}
	poller.providersByOrg["default"][instance.ID] = truenas.NewLiveProvider(&truenas.APIFetcher{Client: client})
}

func captureTrueNASPollerLogs(t *testing.T) *monitoringLogCapture {
	t.Helper()
	return newMonitoringLogCapture(t)
}

type failingTrueNASFetcher struct {
	err error
}

func (f failingTrueNASFetcher) Fetch(context.Context) (*truenas.FixtureSnapshot, error) {
	return nil, f.err
}

func TestTrueNASPollerDoubleStartKeepsSingleLoop(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	mock := newTrueNASMockServer(t, "nas-double-start")
	t.Cleanup(mock.Close)

	mtp, persistence := newTestTenantPersistence(t)
	connection := trueNASInstanceForServer(t, "double-start-conn", mock.URL(), true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	poller.mu.Lock()
	firstStopped := poller.stopped
	poller.mu.Unlock()
	if firstStopped == nil {
		t.Fatal("expected Start() to install a poller loop")
	}

	// A second Start while the first loop is live must be a no-op: the
	// lifecycle slots keep pointing at the original run.
	poller.Start(context.Background())
	poller.mu.Lock()
	secondStopped := poller.stopped
	poller.mu.Unlock()
	if secondStopped != firstStopped {
		t.Fatal("expected double Start() to keep the original poller loop")
	}

	// Stop must clear the cancel slot so a later Start can run a fresh loop.
	poller.Stop()
	poller.mu.Lock()
	cancelAfterStop := poller.cancel
	poller.mu.Unlock()
	if cancelAfterStop != nil {
		t.Fatal("expected Stop() to clear the lifecycle cancel slot")
	}

	poller.Start(context.Background())
	poller.mu.Lock()
	thirdStopped := poller.stopped
	poller.mu.Unlock()
	if thirdStopped == firstStopped {
		t.Fatal("expected restart to install a fresh poller loop")
	}
}

func TestTrueNASPollerKeysSystemsByConnection(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	// Both appliances report the same hostname (#1573, #1575); the configured
	// connections must keep them distinct instead of collapsing them into one
	// flapping resource.
	first := newTrueNASMockServer(t, "truenas")
	t.Cleanup(first.Close)
	second := newTrueNASMockServer(t, "truenas")
	t.Cleanup(second.Close)

	mtp, persistence := newTestTenantPersistence(t)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{
		trueNASInstanceForServer(t, "conn-first", first.URL(), true),
		trueNASInstanceForServer(t, "conn-second", second.URL(), true),
	}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	poller := NewTrueNASPoller(mtp, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	systemSourceIDs := func() map[string]struct{} {
		ids := make(map[string]struct{})
		for _, record := range poller.GetCurrentRecordsForOrg("default") {
			if record.Resource.Type == unifiedresources.ResourceTypeAgent {
				ids[record.SourceID] = struct{}{}
			}
		}
		return ids
	}
	waitForCondition(t, 2*time.Second, func() bool {
		ids := systemSourceIDs()
		_, firstOK := ids["system:conn-first"]
		_, secondOK := ids["system:conn-second"]
		return firstOK && secondOK && len(ids) == 2
	}, "expected one connection-scoped system source ID per configured connection")
}

func TestTrueNASSuccessfulPollCadenceIncludesBoundedIdleGap(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		interval, duration, next time.Duration
	}{
		{"fast", time.Minute, 2 * time.Second, time.Minute},
		{"nearly_due", time.Minute, 58 * time.Second, 63 * time.Second},
		{"slow", time.Minute, 90 * time.Second, 95 * time.Second},
		{"genuinely_stale", time.Minute, 122 * time.Second, 127 * time.Second},
		{"short_interval", time.Second, 2 * time.Second, 3 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			poller := NewTrueNASPoller(nil, 0, nil)
			instance := config.TrueNASInstance{ID: "cadence", PollIntervalSecs: int(tc.interval / time.Second)}
			start := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
			end := start.Add(tc.duration)
			poller.recordConnectionSuccessLocked("default", instance.ID, instance, start, end, nil)
			status := poller.ensureConnectionRuntimeStatusLocked("default", instance.ID)
			want := start.Add(tc.next)
			if !status.nextPollAt.Equal(want) {
				t.Errorf("next poll = %v, want %v", status.nextPollAt, want)
			}
			if !status.lastSuccessAt.Equal(end) || !status.lastAttemptAt.Equal(end) {
				t.Error("scheduling altered observed completion timestamps")
			}
			if poller.connectionPollDueLocked("default", instance.ID, instance, want.Add(-time.Nanosecond)) {
				t.Error("poll due before bounded idle gap elapsed")
			}
			if !poller.connectionPollDueLocked("default", instance.ID, instance, want) {
				t.Error("poll not due at scheduled time")
			}
			// Failed refreshes retain the full interval after completion; this change
			// must not accelerate retries against an unavailable appliance.
			poller.recordConnectionFailureLocked("default", instance.ID, instance, errors.New("offline"), end)
			if !status.nextPollAt.Equal(end.Add(tc.interval)) {
				t.Error("failure backoff changed")
			}
		})
	}
}

// HTTP/client -> poller -> registry -> row/metric writer -> chart readback.
// Synthetic response shapes prove omission/zero handling, not CORE acceptance.
func TestTrueNASPartialReportingPipeline(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })
	cycles := []struct {
		body          string
		status        int
		want          map[string]float64
		errorCategory string
		native        bool
	}{
		{`[{"name":"memory","legend":["free"],"data":[[1789000060,8]]}]`, 200, map[string]float64{"memory": 50}, "", false},
		{`[{"name":"cpu","legend":["usage"],"data":[[1789000060,0]]}]`, 200, map[string]float64{"cpu": 0}, "", false},
		{`provider-private-text`, 401, nil, "authentication", false},
		{`[{"name":"cpu","legend":["usage"],"data":[[1789000060,0]]},{"name":"memory","legend":["free"],"data":[[1789000060,0]]},{"name":"interface","legend":["received"],"data":[[1789000060,0]]},{"name":"disk","legend":["write"],"data":[[1789000060,0]]}]`, 200, map[string]float64{"cpu": 0, "memory": 100, "netin": 0, "diskwrite": 0}, "", false},
		{`[
 {"name":"cpu","legend":["interrupt","system","user","nice","idle"],"data":[[1,2,3,4,90]]},
 {"name":"memory","legend":["memory-active_value","memory-inactive_value","memory-wired_value","memory-laundry_value","memory-free_value"],"data":[[1,1,10,0,4]]},
 {"name":"arcsize","legend":["arcsize_value"],"data":[[2]]},
 {"name":"cputemp","legend":["cputemp0","cputemp1"],"data":[[41,42]]},
 {"name":"interface","identifier":"nic-a","legend":["rx","tx","overlap"],"data":[[8,16,8]]},
 {"name":"disk","identifier":"disk-a","legend":["disk_octets_read","disk_octets_write"],"data":[[0,32]]}
 ]`, 200, map[string]float64{"cpu": 10, "memory": 62.5, "netin": 8, "netout": 16, "diskread": 0, "diskwrite": 32, "temperature": 42}, "", true},
	}
	var current atomic.Int64
	var nativeEnd atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2.0/system/info":
			_, _ = w.Write([]byte(`{"hostname":"synthetic-core","version":"TrueNAS-13.0-U6.1","cores":8,"physmem":16}`))
		case "/api/v2.0/pool":
			_, _ = w.Write([]byte(`[{"id":1,"name":"tank","status":"ONLINE","size":100,"allocated":50,"free":50}]`))
		case "/api/v2.0/pool/dataset", "/api/v2.0/disk", "/api/v2.0/alert/list":
			_, _ = w.Write([]byte(`[]`))
		case "/api/v2.0/reporting/graphs":
			if cycles[current.Load()].native {
				_, _ = w.Write([]byte(`[{"name":"disk","identifiers":["disk-a"]},{"name":"interface","identifiers":["nic-a"]}]`))
			} else {
				http.NotFound(w, r)
			}
		case "/api/v2.0/reporting/get_data":
			cycle := cycles[current.Load()]
			w.WriteHeader(cycle.status)
			if !cycle.native {
				_, _ = w.Write([]byte(cycle.body))
				break
			}
			var query struct {
				Query  map[string]any   `json:"query"`
				Graphs []map[string]any `json:"graphs"`
			}
			if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
				t.Error(err)
				return
			}
			var rows []map[string]any
			if err := json.Unmarshal([]byte(cycle.body), &rows); err != nil {
				t.Error(err)
				return
			}
			end := int64(query.Query["end"].(float64))
			nativeEnd.Store(end)
			for _, row := range rows {
				row["start"] = end
				row["end"] = end
				row["step"] = 10
			}
			for _, graph := range query.Graphs {
				if name := graph["name"]; (name == "interface" || name == "disk") && graph["identifier"] == nil {
					t.Error("native device requested without identifier")
					return
				}
			}
			if err := json.NewEncoder(w).Encode(rows); err != nil {
				t.Error(err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	fingerprint := fmt.Sprintf("%x", sha256.Sum256(server.Certificate().Raw))
	// Trust only this fixture certificate via VerifyConnection, rather than
	// requiring httptest's self-signed certificate to have a public CA chain.
	// Fingerprint pinning remains enforced (including the existing mismatch
	// controls in internal/truenas); no production TLS setting is changed.
	client, err := truenas.NewClient(truenas.ClientConfig{Host: server.URL, APIKey: "synthetic", Fingerprint: fingerprint, InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	provider := truenas.NewLiveProviderForConnection(&truenas.APIFetcher{Client: client}, "synthetic-connection")
	instance := config.TrueNASInstance{ID: "synthetic-connection", Host: server.URL, Enabled: true}
	poller := NewTrueNASPoller(nil, 0, nil)
	poller.providersByOrg["default"] = map[string]*truenas.Provider{instance.ID: provider}
	poller.configsByOrg["default"] = map[string]config.TrueNASInstance{instance.ID: instance}
	resourceStore := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(unifiedresources.NewMemoryStore()))
	persistent, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = persistent.Close() }()
	monitor := &Monitor{metricsHistory: NewMetricsHistory(1024, 24*time.Hour), metricsStore: persistent}
	counts := make(map[string]int)
	observedAt := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Second)
	for index, cycle := range cycles {
		current.Store(int64(index))
		// Model the next due poll, not an absent deadline: a zero deadline
		// is deliberately reconstructed from the previous attempt's cadence.
		poller.ensureConnectionRuntimeStatusLocked("default", instance.ID).nextPollAt = time.Now().Add(-time.Second)
		poller.pollAll(context.Background())
		summary := poller.ConnectionSummaries("default", []config.TrueNASInstance{instance})[instance.ID]
		if summary.Poll == nil || summary.Poll.LastSuccessAt == nil || summary.Poll.LastError != nil || summary.Poll.ConsecutiveFailures != 0 || summary.Observed == nil || summary.Observed.StoragePools != 1 {
			t.Fatalf("cycle %d lost successful inventory health: %+v", index, summary)
		}
		availability := summary.Observed.Telemetry
		if availability == nil || availability.ErrorCategory != cycle.errorCategory {
			t.Fatalf("cycle %d lost telemetry result: %+v", index, availability)
		}
		for key, present := range map[string]bool{"cpu": availability.CPU, "memory": availability.Memory, "netin": availability.NetIn, "netout": availability.NetOut, "diskread": availability.DiskRead, "diskwrite": availability.DiskWrite} {
			_, want := cycle.want[key]
			if present != want {
				t.Errorf("cycle %d diagnostics %s presence=%v, want %v", index, key, present, want)
			}
		}
		serialized, err := json.Marshal(summary)
		if err != nil || strings.Contains(string(serialized), "provider-private-text") || strings.Contains(string(serialized), "synthetic\"") {
			t.Fatalf("cycle %d secret-bearing diagnostic: %s %v", index, serialized, err)
		}
		availability.ErrorCategory = "mutated"
		if poller.ConnectionSummaries("default", []config.TrueNASInstance{instance})[instance.ID].Observed.Telemetry.ErrorCategory != cycle.errorCategory {
			t.Fatal("connection diagnostics share mutable provider state")
		}
		records := poller.GetCurrentRecordsForOrg("default")
		// Space the modeled poll observations apart without a wall-clock sleep;
		// the writer/store deliberately coalesce identical second timestamps.
		for i := range records {
			records[i].Resource.LastSeen = observedAt.Add(time.Duration(index) * time.Minute)
			records[i].Resource.UpdatedAt = records[i].Resource.LastSeen
		}
		resourceStore.PopulateSnapshotAndSupplemental(models.StateSnapshot{}, map[unifiedresources.DataSource][]unifiedresources.IngestRecord{unifiedresources.SourceTrueNAS: records})
		var host *unifiedresources.Resource
		for _, resource := range resourceStore.GetAll() {
			if resource.Type == unifiedresources.ResourceTypeAgent {
				host = &resource
			}
		}
		if host == nil || host.Agent == nil || host.Agent.CPUCount != 8 || host.Agent.Memory == nil || host.Agent.Memory.Total != 16 {
			t.Fatalf("cycle %d lost host/capacity: %+v", index, host)
		}
		if host.Agent.Memory.UsageUnavailable != !availability.Memory {
			t.Fatalf("cycle %d stale/fabricated memory metadata: %+v", index, host.Agent.Memory)
		}
		if cycle.native && (host.Temperature == nil || *host.Temperature != 42) {
			t.Fatalf("native collapsed/expanded temperature missing: %+v", host)
		}
		var writes []metrics.WriteMetric
		monitor.syncUnifiedAgentMetrics(resourceStore, &writes)
		seen := make(map[string]bool)
		for _, write := range writes {
			want, present := cycle.want[write.MetricType]
			if write.MetricType == "disk" {
				want, present = 50, true // independently collected pool capacity
			}
			if !present || write.Value != want || write.ResourceID != instance.ID || write.ResourceType != "agent" || seen[write.MetricType] {
				t.Errorf("cycle %d wrong/duplicate/missing-source write: %+v", index, write)
			}
			seen[write.MetricType] = true
			counts[write.MetricType]++
		}
		if len(writes) != len(cycle.want)+1 {
			t.Fatalf("cycle %d writes=%+v, want only observed metrics plus pool usage", index, writes)
		}
		persistent.WriteBatchSync(writes)
		inMemory := monitor.GetGuestMetrics("agent:"+instance.ID, time.Hour)
		chart := monitor.GetGuestMetricsForChart("agent:"+instance.ID, "agent", instance.ID, time.Hour)
		for _, key := range []string{"cpu", "memory", "disk", "netin", "netout", "diskread", "diskwrite", "temperature"} {
			if len(inMemory[key]) != counts[key] || len(chart[key]) != counts[key] {
				t.Errorf("cycle %d %s chart contains fabricated/lost points: memory=%d chart=%d want=%d", index, key, len(inMemory[key]), len(chart[key]), counts[key])
			}
		}
		id, native, err := provider.SystemMetricHistory(context.Background(), time.Hour)
		if cycle.status != 200 {
			if err == nil {
				t.Fatal("failed native History read was disguised as success")
			}
			continue
		}
		wantNative := len(cycle.want)
		if err != nil || id != instance.ID || len(native) != wantNative {
			t.Fatalf("cycle %d native History=%+v id=%q err=%v", index, native, id, err)
		}
		for key, want := range cycle.want {
			points := native[key]
			if len(points) != 1 || points[0].Value != want || points[0].Timestamp.Unix() != func() int64 {
				if cycle.native {
					return nativeEnd.Load()
				}
				return 1789000060
			}() {
				t.Errorf("cycle %d %s native data/timestamp altered: %+v", index, key, points)
			}
		}
		if cycle.native {
			points := native["temperature"]
			if len(points) != 1 || points[0].Value != 42 || points[0].Timestamp.Unix() != nativeEnd.Load() {
				t.Fatalf("native temperature History missing: %+v", points)
			}
			shared := poller.GuestMetricHistory(nil, "default", "agent", time.Hour)[instance.ID]
			if len(shared) != 7 || len(shared["temperature"]) != 1 {
				t.Fatalf("shared History fallback dropped native panels: %+v", shared)
			}
		}
	}
	// Once local CPU History covers the window, the chart's fast path must
	// still retain Thermals without relying on another appliance request.
	now := time.Now()
	for i := 0; i <= 60; i++ {
		monitor.metricsHistory.AddGuestMetric("agent:"+instance.ID, "cpu", 10, now.Add(time.Duration(i-60)*time.Minute))
	}
	chart := monitor.GetGuestMetricsForChart("agent:"+instance.ID, "agent", instance.ID, time.Hour)
	if points := chart["temperature"]; len(points) != 1 || points[0].Value != 42 {
		t.Fatalf("sufficiently covered local History lost Thermals: %+v", points)
	}
}
