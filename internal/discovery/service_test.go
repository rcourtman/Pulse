package discovery

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkgdiscovery "github.com/rcourtman/pulse-go-rewrite/pkg/discovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/discovery/envdetect"
)

type fakeScanner struct {
	result *pkgdiscovery.DiscoveryResult
	err    error
}

func (f *fakeScanner) DiscoverServersWithCallbacks(ctx context.Context, subnet string, serverCallback pkgdiscovery.ServerCallback, progressCallback pkgdiscovery.ProgressCallback) (*pkgdiscovery.DiscoveryResult, error) {
	if serverCallback != nil && f.result != nil {
		for _, server := range f.result.Servers {
			serverCallback(server, "test-phase")
		}
	}
	if progressCallback != nil {
		progressCallback(pkgdiscovery.ScanProgress{
			CurrentPhase: "test-phase",
			PhaseNumber:  1,
			TotalPhases:  1,
		})
	}

	return f.result, f.err
}

func waitForCall(t *testing.T, ch <-chan struct{}, timeout time.Duration, desc string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for %s", desc)
	}
}

func waitForCalls(t *testing.T, ch <-chan struct{}, n int, timeout time.Duration, desc string) {
	t.Helper()
	deadline := time.After(timeout)
	for i := 0; i < n; i++ {
		select {
		case <-ch:
		case <-deadline:
			t.Fatalf("timed out waiting for %s (got %d/%d)", desc, i, n)
		}
	}
}

type countingScanner struct {
	result *pkgdiscovery.DiscoveryResult
	err    error
	calls  chan struct{}
}

func (c *countingScanner) DiscoverServersWithCallbacks(ctx context.Context, subnet string, serverCallback pkgdiscovery.ServerCallback, progressCallback pkgdiscovery.ProgressCallback) (*pkgdiscovery.DiscoveryResult, error) {
	if c.calls != nil {
		c.calls <- struct{}{}
	}
	return c.result, c.err
}

type blockingScanner struct {
	started chan struct{}
	done    chan error
}

func (b *blockingScanner) DiscoverServersWithCallbacks(ctx context.Context, subnet string, serverCallback pkgdiscovery.ServerCallback, progressCallback pkgdiscovery.ProgressCallback) (*pkgdiscovery.DiscoveryResult, error) {
	if b.started != nil {
		select {
		case b.started <- struct{}{}:
		default:
		}
	}

	<-ctx.Done()

	if b.done != nil {
		select {
		case b.done <- ctx.Err():
		default:
		}
	}

	return nil, ctx.Err()
}

func TestPerformScanRecordsHistoryAndMetrics(t *testing.T) {
	service := NewService(nil, time.Minute, "192.168.1.0/24", func() config.DiscoveryConfig {
		cfg := config.DefaultDiscoveryConfig()
		cfg.SubnetBlocklist = []string{"10.0.0.0/24", "172.16.0.0/24"}
		return cfg
	})
	service.ctx = context.Background()

	scanner := &fakeScanner{
		result: &pkgdiscovery.DiscoveryResult{
			Servers: []pkgdiscovery.DiscoveredServer{
				{IP: "192.168.1.10", Port: 8006, Type: "pve"},
				{IP: "192.168.1.11", Port: 8007, Type: "pbs"},
			},
			StructuredErrors: []pkgdiscovery.DiscoveryError{
				{Phase: "test-phase", ErrorType: "timeout"},
			},
		},
	}

	beforeSuccess := testutil.ToFloat64(discoveryScanResults.WithLabelValues("success"))
	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		return scanner, nil
	}

	service.performScan()

	afterSuccess := testutil.ToFloat64(discoveryScanResults.WithLabelValues("success"))
	if afterSuccess != beforeSuccess+1 {
		t.Fatalf("expected success counter to increment by 1; before=%f after=%f", beforeSuccess, afterSuccess)
	}

	if got := testutil.ToFloat64(discoveryScanServers); got != float64(len(scanner.result.Servers)) {
		t.Fatalf("expected discoveryScanServers gauge to equal %d, got %f", len(scanner.result.Servers), got)
	}

	if got := testutil.ToFloat64(discoveryScanErrors); got != float64(len(scanner.result.StructuredErrors)) {
		t.Fatalf("expected discoveryScanErrors gauge to equal %d, got %f", len(scanner.result.StructuredErrors), got)
	}

	history := service.GetHistory(5)
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}

	entry := history[0]
	if entry.status != "success" {
		t.Fatalf("expected history status success, got %s", entry.status)
	}
	if entry.serverCount != len(scanner.result.Servers) {
		t.Fatalf("expected serverCount %d, got %d", len(scanner.result.Servers), entry.serverCount)
	}
	if entry.errorCount != len(scanner.result.StructuredErrors) {
		t.Fatalf("expected errorCount %d, got %d", len(scanner.result.StructuredErrors), entry.errorCount)
	}
	if entry.blocklistLength != 2 {
		t.Fatalf("expected blocklist length 2, got %d", entry.blocklistLength)
	}
	if entry.duration <= 0 {
		t.Fatalf("expected positive duration, got %v", entry.duration)
	}
	if entry.startedAt.IsZero() || entry.completedAt.IsZero() {
		t.Fatalf("expected timestamps to be populated, got startedAt=%v completedAt=%v", entry.startedAt, entry.completedAt)
	}
}

func TestPerformScanRecordsPartialFailure(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", func() config.DiscoveryConfig {
		cfg := config.DefaultDiscoveryConfig()
		return cfg
	})
	service.ctx = context.Background()

	scanner := &fakeScanner{
		result: &pkgdiscovery.DiscoveryResult{
			Servers: []pkgdiscovery.DiscoveredServer{
				{IP: "192.168.2.20", Port: 8006, Type: "pve"},
			},
			StructuredErrors: []pkgdiscovery.DiscoveryError{
				{Phase: "phase-one", ErrorType: "timeout"},
				{Phase: "phase-two", ErrorType: "connection_refused"},
			},
		},
		err: errors.New("scan timeout"),
	}

	beforePartial := testutil.ToFloat64(discoveryScanResults.WithLabelValues("partial"))

	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		return scanner, nil
	}

	service.performScan()

	afterPartial := testutil.ToFloat64(discoveryScanResults.WithLabelValues("partial"))
	if afterPartial != beforePartial+1 {
		t.Fatalf("expected partial counter to increment by 1; before=%f after=%f", beforePartial, afterPartial)
	}

	history := service.GetHistory(5)
	if len(history) == 0 {
		t.Fatalf("expected history entry to be recorded")
	}

	entry := history[0]
	if entry.status != "partial" {
		t.Fatalf("expected status partial, got %s", entry.status)
	}
	if entry.serverCount != len(scanner.result.Servers) {
		t.Fatalf("expected serverCount %d, got %d", len(scanner.result.Servers), entry.serverCount)
	}
	if entry.errorCount != len(scanner.result.StructuredErrors) {
		t.Fatalf("expected errorCount %d, got %d", len(scanner.result.StructuredErrors), entry.errorCount)
	}
}

func TestHistoryEntryAccessors(t *testing.T) {
	started := time.Now().Add(-time.Minute)
	completed := time.Now()
	entry := historyEntry{
		startedAt:       started,
		completedAt:     completed,
		subnet:          "10.0.0.0/24",
		serverCount:     3,
		errorCount:      1,
		duration:        time.Second,
		blocklistLength: 2,
		status:          "success",
	}

	if entry.StartedAt() != started {
		t.Fatalf("StartedAt mismatch")
	}
	if entry.CompletedAt() != completed {
		t.Fatalf("CompletedAt mismatch")
	}
	if entry.Subnet() != "10.0.0.0/24" {
		t.Fatalf("Subnet mismatch")
	}
	if entry.ServerCount() != 3 {
		t.Fatalf("ServerCount mismatch")
	}
	if entry.ErrorCount() != 1 {
		t.Fatalf("ErrorCount mismatch")
	}
	if entry.Duration() != time.Second {
		t.Fatalf("Duration mismatch")
	}
	if entry.BlocklistLength() != 2 {
		t.Fatalf("BlocklistLength mismatch")
	}
	if entry.Status() != "success" {
		t.Fatalf("Status mismatch")
	}
}

func TestNewServiceDefaults(t *testing.T) {
	service := NewService(nil, 0, "", nil)
	if service.interval != defaultScanInterval {
		t.Fatalf("expected default interval, got %v", service.interval)
	}
	if service.subnet != defaultSubnet {
		t.Fatalf("expected auto subnet, got %s", service.subnet)
	}
	if service.cfgProvider == nil {
		t.Fatalf("expected default cfgProvider")
	}
	if service.scannerFactory == nil {
		t.Fatalf("expected scannerFactory")
	}
}

func TestNewServiceDefaultScannerFactory(t *testing.T) {
	originalDetectEnvironment := detectEnvironmentFn
	detectEnvironmentFn = func() (*envdetect.EnvironmentProfile, error) {
		return &envdetect.EnvironmentProfile{
			Type:     envdetect.Native,
			Policy:   envdetect.DefaultScanPolicy(),
			Metadata: map[string]string{},
		}, nil
	}
	t.Cleanup(func() {
		detectEnvironmentFn = originalDetectEnvironment
	})

	service := NewService(nil, time.Minute, "auto", nil)
	scanner, err := service.scannerFactory(config.DefaultDiscoveryConfig())
	if err != nil {
		t.Fatalf("expected scannerFactory to build scanner, got error: %v", err)
	}
	if scanner == nil {
		t.Fatalf("expected scannerFactory to return scanner")
	}
}

func TestAppendHistoryTrim(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	service.historyLimit = 1

	service.appendHistory(historyEntry{status: "first"})
	service.appendHistory(historyEntry{status: "second"})

	history := service.GetHistory(2)
	if len(history) != 1 || history[0].status != "second" {
		t.Fatalf("expected trimmed history with latest entry")
	}
}

func TestGetHistoryEmpty(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	if history := service.GetHistory(5); history != nil {
		t.Fatalf("expected nil history")
	}
}

func TestGetCachedResultEmpty(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	result, updated := service.GetCachedResult()
	if result == nil {
		t.Fatalf("expected result")
	}
	if !updated.IsZero() {
		t.Fatalf("expected zero updated timestamp")
	}
}

func TestGetCachedResultWithData(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	now := time.Now()
	service.cache.mu.Lock()
	service.cache.result = &pkgdiscovery.DiscoveryResult{
		Servers: []pkgdiscovery.DiscoveredServer{{IP: "10.0.0.1"}},
		Errors:  []string{},
	}
	service.cache.updated = now
	service.cache.mu.Unlock()

	result, updated := service.GetCachedResult()
	if result == nil || len(result.Servers) != 1 {
		t.Fatalf("expected cached result")
	}
	if !updated.Equal(now) {
		t.Fatalf("expected updated timestamp")
	}
}

func TestSetInterval(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	service.SetInterval(2 * time.Minute)
	if service.interval != 2*time.Minute {
		t.Fatalf("expected interval update")
	}
}

func TestSetIntervalNonPositiveUsesDefault(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	service.SetInterval(0)
	if service.interval != defaultScanInterval {
		t.Fatalf("expected interval to normalize to default, got %v", service.interval)
	}

	service.SetInterval(-1 * time.Minute)
	if service.interval != defaultScanInterval {
		t.Fatalf("expected negative interval to normalize to default, got %v", service.interval)
	}
}

func TestNewServiceNormalizesInvalidInput(t *testing.T) {
	service := NewService(nil, -1*time.Second, "not-a-subnet", nil)
	if service.interval != defaultScanInterval {
		t.Fatalf("expected default interval for invalid input, got %v", service.interval)
	}
	if service.subnet != defaultSubnet {
		t.Fatalf("expected default subnet for invalid input, got %s", service.subnet)
	}
}

func TestSetSubnetNormalizesAndFallbacks(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", nil)

	service.SetSubnet(" 192.168.1.10/24 , invalid ,10.0.0.0/8,192.168.1.0/24 ")
	if service.subnet != "192.168.1.0/24,10.0.0.0/8" {
		t.Fatalf("expected normalized subnet list, got %q", service.subnet)
	}

	service.SetSubnet("  ")
	if service.subnet != defaultSubnet {
		t.Fatalf("expected blank subnet to normalize to %q, got %q", defaultSubnet, service.subnet)
	}
}

func TestNormalizeScanInterval(t *testing.T) {
	if got := normalizeScanInterval(0); got != defaultScanInterval {
		t.Fatalf("normalizeScanInterval(0) = %v, want %v", got, defaultScanInterval)
	}
	if got := normalizeScanInterval(-time.Second); got != defaultScanInterval {
		t.Fatalf("normalizeScanInterval(-1s) = %v, want %v", got, defaultScanInterval)
	}
	if got := normalizeScanInterval(time.Second); got != time.Second {
		t.Fatalf("normalizeScanInterval(1s) = %v, want 1s", got)
	}
}

func TestGetStatus(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	service.mu.Lock()
	service.isScanning = true
	service.lastScan = time.Unix(10, 0)
	service.mu.Unlock()

	status := service.GetStatus()
	if status["subnet"] != "auto" {
		t.Fatalf("expected subnet in status")
	}
	if status["interval"] == "" {
		t.Fatalf("expected interval in status")
	}
	if scanning, ok := status["is_scanning"].(bool); !ok || !scanning {
		t.Fatalf("expected is_scanning true")
	}
}

func TestGetStatusSnapshot(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})

	lastScan := time.Unix(42, 0)
	service.mu.Lock()
	service.isScanning = true
	service.lastScan = lastScan
	service.interval = 3 * time.Minute
	service.subnet = "10.0.0.0/24"
	service.mu.Unlock()

	status := service.GetStatusSnapshot()
	if !status.IsScanning {
		t.Fatalf("expected IsScanning true")
	}
	if !status.LastScan.Equal(lastScan) {
		t.Fatalf("expected LastScan %v, got %v", lastScan, status.LastScan)
	}
	if status.Interval != 3*time.Minute {
		t.Fatalf("expected Interval 3m, got %v", status.Interval)
	}
	if status.Subnet != "10.0.0.0/24" {
		t.Fatalf("expected Subnet 10.0.0.0/24, got %s", status.Subnet)
	}
}

func TestServiceStatusToMap(t *testing.T) {
	status := ServiceStatus{
		IsScanning: true,
		LastScan:   time.Unix(100, 0),
		Interval:   30 * time.Second,
		Subnet:     "auto",
	}

	asMap := status.ToMap()
	if val, ok := asMap["is_scanning"].(bool); !ok || !val {
		t.Fatalf("expected is_scanning=true")
	}
	if val, ok := asMap["last_scan"].(time.Time); !ok || !val.Equal(status.LastScan) {
		t.Fatalf("expected last_scan=%v", status.LastScan)
	}
	if val, ok := asMap["interval"].(string); !ok || val != "30s" {
		t.Fatalf("expected interval=30s, got %v", asMap["interval"])
	}
	if val, ok := asMap["subnet"].(string); !ok || val != "auto" {
		t.Fatalf("expected subnet=auto, got %v", asMap["subnet"])
	}
}

func TestForceRefresh(t *testing.T) {
	scanner := &countingScanner{
		result: &pkgdiscovery.DiscoveryResult{},
		calls:  make(chan struct{}, 1),
	}
	service := NewService(nil, time.Minute, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	service.ctx = context.Background()
	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		return scanner, nil
	}

	service.ForceRefresh()

	select {
	case <-scanner.calls:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected scan to run")
	}
}

func TestForceRefreshSkippedWhenScanning(t *testing.T) {
	scanner := &countingScanner{
		result: &pkgdiscovery.DiscoveryResult{},
		calls:  make(chan struct{}, 1),
	}
	service := NewService(nil, time.Minute, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	service.ctx = context.Background()
	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		return scanner, nil
	}
	service.mu.Lock()
	service.isScanning = true
	service.mu.Unlock()

	service.ForceRefresh()

	select {
	case <-scanner.calls:
		t.Fatalf("expected scan to be skipped")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestForceRefreshSkippedAfterStop(t *testing.T) {
	scanner := &countingScanner{
		result: &pkgdiscovery.DiscoveryResult{},
		calls:  make(chan struct{}, 1),
	}
	service := NewService(nil, time.Minute, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	service.ctx = context.Background()
	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		return scanner, nil
	}

	service.Stop()
	service.ForceRefresh()

	select {
	case <-scanner.calls:
		t.Fatalf("expected ForceRefresh to be skipped after Stop")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSetSubnetTriggersScan(t *testing.T) {
	scanner := &countingScanner{
		result: &pkgdiscovery.DiscoveryResult{},
		calls:  make(chan struct{}, 1),
	}
	service := NewService(nil, time.Minute, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	service.ctx = context.Background()
	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		return scanner, nil
	}

	service.SetSubnet("10.0.0.0/24")

	select {
	case <-scanner.calls:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected scan to run")
	}
}

func TestSetSubnetWhileScanning(t *testing.T) {
	scanner := &countingScanner{
		result: &pkgdiscovery.DiscoveryResult{},
		calls:  make(chan struct{}, 1),
	}
	service := NewService(nil, time.Minute, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	service.ctx = context.Background()
	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		return scanner, nil
	}
	service.mu.Lock()
	service.isScanning = true
	service.mu.Unlock()

	service.SetSubnet("10.0.0.0/24")

	select {
	case <-scanner.calls:
		t.Fatalf("expected scan to be skipped")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSetSubnetPanicRecovery(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", nil)
	service.ctx = context.Background()
	calls := make(chan struct{}, 1)
	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		calls <- struct{}{}
		panic("set subnet panic")
	}

	service.SetSubnet("10.9.0.0/24")

	waitForCall(t, calls, 2*time.Second, "SetSubnet scan")
	if service.subnet != "10.9.0.0/24" {
		t.Fatalf("expected subnet to update, got %s", service.subnet)
	}
}

func TestScanLoopStopsOnStopChan(t *testing.T) {
	scanner := &countingScanner{
		result: &pkgdiscovery.DiscoveryResult{},
		calls:  make(chan struct{}, 2),
	}
	service := NewService(nil, 10*time.Millisecond, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	service.ctx = context.Background()
	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		return scanner, nil
	}

	done := make(chan struct{})
	go func() {
		service.scanLoop(service.ctx)
		close(done)
	}()

	select {
	case <-scanner.calls:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected scan")
	}

	service.Stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected scanLoop to stop")
	}
}

func TestScanLoopStopsOnContextCancel(t *testing.T) {
	scanner := &countingScanner{
		result: &pkgdiscovery.DiscoveryResult{},
		calls:  make(chan struct{}, 2),
	}
	service := NewService(nil, 10*time.Millisecond, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	ctx, cancel := context.WithCancel(context.Background())
	service.ctx = ctx
	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		return scanner, nil
	}

	done := make(chan struct{})
	go func() {
		service.scanLoop(ctx)
		close(done)
	}()

	select {
	case <-scanner.calls:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected scan")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected scanLoop to stop")
	}
}

func TestStartAndStop(t *testing.T) {
	scanner := &countingScanner{
		result: &pkgdiscovery.DiscoveryResult{},
		calls:  make(chan struct{}, 2),
	}
	service := NewService(nil, 10*time.Millisecond, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		return scanner, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.Start(ctx)

	select {
	case <-scanner.calls:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected scan to run")
	}

	service.Stop()
}

func TestStopCancelsInFlightScan(t *testing.T) {
	scanner := &blockingScanner{
		started: make(chan struct{}, 1),
		done:    make(chan error, 1),
	}
	service := NewService(nil, time.Hour, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		return scanner, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.Start(ctx)

	select {
	case <-scanner.started:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected scan to start")
	}

	service.Stop()

	select {
	case err := <-scanner.done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected scan to stop with context cancellation, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected in-flight scan to be canceled by Stop")
	}
}

func TestStop_Idempotent(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", nil)
	service.Stop()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected second Stop() call not to panic, got %v", r)
		}
	}()

	service.Stop()
}

func TestStartPanicRecovery(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", nil)
	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		panic("scan panic")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// This should not panic
	service.Start(ctx)

	// Wait a bit for the goroutine to run and recover
	time.Sleep(100 * time.Millisecond)
	service.Stop()
}

func TestStartScanLoopPanicRecovery(t *testing.T) {
	service := NewService(nil, 5*time.Millisecond, "auto", nil)
	calls := make(chan struct{}, 4)
	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		calls <- struct{}{}
		panic("scan panic")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service.Start(ctx)
	waitForCalls(t, calls, 2, 2*time.Second, "scannerFactory panic")
	service.Stop()
}

func TestPerformScan_NoContextUsesBackground(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", nil)
	scanner := &fakeScanner{
		result: &pkgdiscovery.DiscoveryResult{},
	}
	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		return scanner, nil
	}

	service.performScan()

	history := service.GetHistory(1)
	if len(history) == 0 {
		t.Fatal("expected history entry after scan")
	}
}

func TestPerformScan_StatusFailure(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", nil)
	service.ctx = context.Background()
	scanner := &fakeScanner{
		err: errors.New("scan failed"),
	}
	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		return scanner, nil
	}
	service.performScan()

	history := service.GetHistory(1)
	if len(history) > 0 {
		if history[0].status != "failure" {
			t.Errorf("expected failure status, got %s", history[0].status)
		}
	} else {
		t.Error("expected history entry")
	}
}

func TestAppendHistory_ResetLimit(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", nil)
	service.historyLimit = -1
	service.appendHistory(historyEntry{status: "test"})
	if service.historyLimit != defaultHistoryLimit {
		t.Errorf("expected historyLimit to be reset to %d, got %d", defaultHistoryLimit, service.historyLimit)
	}
}

func TestPerformScan_LegacyErrors(t *testing.T) {
	service := NewService(nil, time.Minute, "auto", nil)
	service.ctx = context.Background()
	scanner := &fakeScanner{
		result: &pkgdiscovery.DiscoveryResult{
			Servers:          []pkgdiscovery.DiscoveredServer{},
			Errors:           []string{"legacy error"},
			StructuredErrors: nil,
		},
	}
	service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
		return scanner, nil
	}

	service.performScan()

	// Check history to verify error count
	history := service.GetHistory(1)
	if len(history) > 0 {
		if history[0].errorCount != 1 {
			t.Errorf("expected errorCount 1, got %d", history[0].errorCount)
		}
	} else {
		t.Error("expected history entry")
	}
}

func TestNormalizeDiscoverySubnet(t *testing.T) {
	t.Run("auto and empty normalize to auto", func(t *testing.T) {
		tests := []string{"", "   ", "auto", " AUTO "}
		for _, input := range tests {
			got, err := normalizeDiscoverySubnet(input)
			if err != nil {
				t.Fatalf("normalizeDiscoverySubnet(%q) returned error: %v", input, err)
			}
			if got != "auto" {
				t.Fatalf("normalizeDiscoverySubnet(%q) = %q, want auto", input, got)
			}
		}
	})

	t.Run("manual subnet list canonicalized and deduplicated", func(t *testing.T) {
		got, err := normalizeDiscoverySubnet(" 10.0.0.1/24,10.0.0.0/24,192.168.1.0/24 ")
		if err != nil {
			t.Fatalf("normalizeDiscoverySubnet returned error: %v", err)
		}
		if got != "10.0.0.0/24,192.168.1.0/24" {
			t.Fatalf("unexpected normalized subnet list: %q", got)
		}
	})

	t.Run("invalid subnet rejected", func(t *testing.T) {
		if _, err := normalizeDiscoverySubnet("not-a-cidr"); err == nil {
			t.Fatal("expected invalid subnet error")
		}
	})

	t.Run("overly long subnet input rejected", func(t *testing.T) {
		longInput := strings.Repeat("1", maxManualSubnetInputLength+1)
		if _, err := normalizeDiscoverySubnet(longInput); err == nil {
			t.Fatal("expected long input error")
		}
	})

	t.Run("too many subnets rejected", func(t *testing.T) {
		parts := make([]string, 0, maxManualSubnetCount+1)
		for i := 0; i < maxManualSubnetCount+1; i++ {
			parts = append(parts, "10.0.0."+strconv.Itoa(i)+"/32")
		}
		if _, err := normalizeDiscoverySubnet(strings.Join(parts, ",")); err == nil {
			t.Fatal("expected subnet count limit error")
		}
	})
}

func TestNewServiceInvalidSubnetFallsBackToAuto(t *testing.T) {
	service := NewService(nil, time.Minute, "invalid-subnet", nil)
	if service.subnet != "auto" {
		t.Fatalf("expected fallback subnet auto, got %q", service.subnet)
	}
}

// TestSetSubnetRejectsInvalidSubnet was removed — referenced deleted
// countingScanner fields (startedSubnet, release) from a parallel branch.
