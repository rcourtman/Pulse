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
		return mock.RequestCount() >= 5
	}, "expected at least one successful TrueNAS poll cycle")

	poller.Stop()
	if !hasTrueNASHost(registry, "nas-one") {
		t.Fatal("expected TrueNAS resources to be ingested")
	}
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
		return successCount.Load() > 0
	}, "expected successful requests after initial failures")

	poller.Stop()
	if !hasTrueNASHost(registry, "metrics-host") {
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
	if !hasTrueNASHost(registry, "nas-one") {
		t.Fatal("expected first host resources to remain in registry after pruning provider")
	}
	if !hasTrueNASHost(registry, "nas-two") {
		t.Fatal("expected second host resources to be ingested")
	}
}

func TestTrueNASPollerAPITimeout(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	var requestCount atomic.Int64
	var injectDelay atomic.Bool
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
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	persistence := config.NewConfigPersistence(t.TempDir())
	connection := trueNASInstanceForServer(t, "timeout-conn", server.URL, true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	registry := unifiedresources.NewRegistry(nil)
	poller := NewTrueNASPoller(registry, persistence, 50*time.Millisecond)
	injectTrueNASProviderTimeout(t, poller, connection, 75*time.Millisecond)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return requestCount.Load() >= 3
	}, "expected poller to continue retrying while API requests time out")

	injectDelay.Store(false)
	recoveryStart := requestCount.Load()

	waitForCondition(t, 3*time.Second, func() bool {
		return requestCount.Load() >= recoveryStart+5
	}, "expected at least one successful poll cycle after timeout clears")

	poller.Stop()
	if !hasTrueNASHost(registry, "timeout-host") {
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

	persistence := config.NewConfigPersistence(t.TempDir())
	connection := trueNASInstanceForServer(t, "auth-failure-conn", server.URL, true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	registry := unifiedresources.NewRegistry(nil)
	poller := NewTrueNASPoller(registry, persistence, 50*time.Millisecond)
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
	if hasTrueNASHost(registry, "auth-failure-host") {
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

	persistence := config.NewConfigPersistence(t.TempDir())
	connection := trueNASInstanceForServer(t, "stale-recovery-conn", server.URL, true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	registry := unifiedresources.NewRegistry(nil)
	poller := NewTrueNASPoller(registry, persistence, 50*time.Millisecond)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return initialSuccesses.Load() > 0
	}, "expected initial successful polls to ingest baseline resources")

	waitForCondition(t, 3*time.Second, func() bool {
		return pollAttempts.Load() >= initialSuccessPolls+failurePolls
	}, "expected poller to continue attempts throughout failure window")

	waitForCondition(t, 3*time.Second, func() bool {
		return recoverySuccesses.Load() > 0
	}, "expected poller to recover and ingest refreshed data after failures")

	poller.Stop()
	if !hasTrueNASHost(registry, "stale-after") {
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

	persistence := config.NewConfigPersistence(t.TempDir())
	connection := trueNASInstanceForServer(t, "connection-flap-conn", server.URL, true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("SaveTrueNASConfig() error = %v", err)
	}

	registry := unifiedresources.NewRegistry(nil)
	poller := NewTrueNASPoller(registry, persistence, 50*time.Millisecond)
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
		return afterRecoverySuccesses.Load() > 0
	}, "expected poller to recover ingestion after endpoint returns")

	poller.Stop()
	if !hasTrueNASHost(registry, "flap-after") {
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

	persistence := config.NewConfigPersistence(t.TempDir())
	connOne := trueNASInstanceForServer(t, "config-change-1", first.URL(), true)
	connTwo := trueNASInstanceForServer(t, "config-change-2", second.URL(), true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connOne}); err != nil {
		t.Fatalf("SaveTrueNASConfig() initial error = %v", err)
	}

	registry := unifiedresources.NewRegistry(nil)
	poller := NewTrueNASPoller(registry, persistence, 50*time.Millisecond)
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
	if !hasTrueNASHost(registry, "config-change-two") {
		t.Fatal("expected second connection resources to be ingested")
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
		return pollerProviderCount(poller) == 1 && enabled.RequestCount() >= 5
	}, "expected only enabled connection provider and resources")

	waitForCondition(t, 2*time.Second, func() bool {
		return enabled.RequestCount() >= 10
	}, "expected additional polling cycles for enabled connection")

	if disabled.RequestCount() != 0 {
		t.Fatalf("expected disabled connection to be skipped, got %d requests", disabled.RequestCount())
	}
	poller.Stop()
	if !hasTrueNASHost(registry, "nas-enabled") {
		t.Fatal("expected enabled connection host to be present in registry")
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
	poller.providers[instance.ID] = truenas.NewLiveProvider(&truenas.APIFetcher{Client: client})
}
