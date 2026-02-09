package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestTrueNASPollerPollsConfiguredConnections(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	mock := newTrueNASMockServer(t, "nas-one")
	t.Cleanup(mock.Close)

	persistence := config.NewConfigPersistence(t.TempDir())
	connection := trueNASInstanceForServer(t, "conn-1", mock.URL(), true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	registry := unifiedresources.NewRegistry(nil)
	poller := NewTrueNASPoller(registry, persistence, 50*time.Millisecond)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return hasTrueNASHost(registry, "nas-one")
	}, "expected TrueNAS resources to be ingested")
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

	persistence := config.NewConfigPersistence(t.TempDir())
	connection := trueNASInstanceForServer(t, "metrics-conn", server.URL, true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	registry := unifiedresources.NewRegistry(nil)
	poller := NewTrueNASPoller(registry, persistence, 50*time.Millisecond)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 5*time.Second, func() bool {
		return hasTrueNASHost(registry, "metrics-host")
	}, "expected TrueNAS resources to appear after initial failures")

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

func TestTrueNASPollerHandlesConnectionAddRemove(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	first := newTrueNASMockServer(t, "nas-one")
	second := newTrueNASMockServer(t, "nas-two")
	t.Cleanup(first.Close)
	t.Cleanup(second.Close)

	persistence := config.NewConfigPersistence(t.TempDir())
	connOne := trueNASInstanceForServer(t, "conn-1", first.URL(), true)
	connTwo := trueNASInstanceForServer(t, "conn-2", second.URL(), true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connOne}); err != nil {
		t.Fatalf("SaveTrueNASConfig() initial error = %v", err)
	}

	registry := unifiedresources.NewRegistry(nil)
	poller := NewTrueNASPoller(registry, persistence, 50*time.Millisecond)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return pollerProviderCount(poller) == 1 && hasTrueNASHost(registry, "nas-one")
	}, "expected first connection provider and records")

	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connOne, connTwo}); err != nil {
		t.Fatalf("SaveTrueNASConfig() add error = %v", err)
	}

	waitForCondition(t, 2*time.Second, func() bool {
		return pollerProviderCount(poller) == 2 && hasTrueNASHost(registry, "nas-two")
	}, "expected second connection to be discovered and polled")

	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connTwo}); err != nil {
		t.Fatalf("SaveTrueNASConfig() remove error = %v", err)
	}

	waitForCondition(t, 2*time.Second, func() bool {
		return pollerProviderCount(poller) == 1 && !pollerHasProvider(poller, "conn-1")
	}, "expected removed connection provider to be pruned")
}

func TestTrueNASPollerSkipsDisabledConnections(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	enabled := newTrueNASMockServer(t, "nas-enabled")
	disabled := newTrueNASMockServer(t, "nas-disabled")
	t.Cleanup(enabled.Close)
	t.Cleanup(disabled.Close)

	persistence := config.NewConfigPersistence(t.TempDir())
	enabledConn := trueNASInstanceForServer(t, "conn-enabled", enabled.URL(), true)
	disabledConn := trueNASInstanceForServer(t, "conn-disabled", disabled.URL(), false)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{enabledConn, disabledConn}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	registry := unifiedresources.NewRegistry(nil)
	poller := NewTrueNASPoller(registry, persistence, 50*time.Millisecond)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return pollerProviderCount(poller) == 1 && hasTrueNASHost(registry, "nas-enabled")
	}, "expected only enabled connection provider and resources")

	time.Sleep(150 * time.Millisecond)
	if disabled.RequestCount() != 0 {
		t.Fatalf("expected disabled connection to be skipped, got %d requests", disabled.RequestCount())
	}
	if hasTrueNASHost(registry, "nas-disabled") {
		t.Fatal("expected disabled connection host to be absent from registry")
	}
}

func TestTrueNASPollerStopsCleanly(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	persistence := config.NewConfigPersistence(t.TempDir())
	registry := unifiedresources.NewRegistry(nil)
	poller := NewTrueNASPoller(registry, persistence, 50*time.Millisecond)
	poller.Start(context.Background())
	poller.Stop()

	select {
	case <-poller.stopped:
	case <-time.After(time.Second):
		t.Fatal("expected poller stopped channel to close")
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

func hasTrueNASHost(registry *unifiedresources.ResourceRegistry, hostname string) bool {
	if registry == nil {
		return false
	}

	resources := registry.List()
	for _, resource := range resources {
		if resource.Type != unifiedresources.ResourceTypeHost || resource.Name != hostname {
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
	return len(poller.providers)
}

func pollerHasProvider(poller *TrueNASPoller, id string) bool {
	if poller == nil {
		return false
	}
	poller.mu.Lock()
	defer poller.mu.Unlock()
	_, ok := poller.providers[id]
	return ok
}
