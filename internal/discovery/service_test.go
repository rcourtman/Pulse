package discovery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkgdiscovery "github.com/rcourtman/pulse-go-rewrite/pkg/discovery"
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

func TestPerformScanRecordsHistoryAndMetrics(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
