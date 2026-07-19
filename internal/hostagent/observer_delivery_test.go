package hostagent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

func TestObserverDeliveryIsIsolatedAndReportOnly(t *testing.T) {
	primaryRequests := atomic.Int32{}
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		primaryRequests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"config":{"commandsEnabled":false}}`))
	}))
	defer primary.Close()

	observerRequests := atomic.Int32{}
	observerHealthy := atomic.Bool{}
	observer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		observerRequests.Add(1)
		if !observerHealthy.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"config":{"commandsEnabled":true}}`))
	}))
	defer observer.Close()

	logger := zerolog.Nop()
	a := &Agent{
		cfg:                 Config{APIToken: "primary-token"},
		logger:              logger,
		httpClient:          primary.Client(),
		trimmedPulseURL:     primary.URL,
		reportBuffer:        utils.New[agentshost.Report](10),
		remoteConfigChanged: make(chan struct{}, 1),
		observerReporters: []*observerReporter{{
			target:     ObserverTarget{Name: "dev", ID: "dev12345", PulseURL: observer.URL, APIToken: "observer-token"},
			httpClient: observer.Client(), reportBuffer: utils.New[agentshost.Report](10),
		}},
	}
	report := agentshost.Report{Host: agentshost.HostInfo{Hostname: "node"}}

	if err := a.deliverPrimaryReport(context.Background(), report); err != nil {
		t.Fatalf("primary delivery: %v", err)
	}
	a.deliverObserverReport(context.Background(), a.observerReporters[0], report)
	if got := a.reportBuffer.Len(); got != 0 {
		t.Fatalf("primary buffer depth = %d, want 0", got)
	}
	if got := a.observerReporters[0].reportBuffer.Len(); got != 1 {
		t.Fatalf("observer buffer depth = %d, want 1", got)
	}

	observerHealthy.Store(true)
	a.deliverObserverReport(context.Background(), a.observerReporters[0], report)
	if got := a.observerReporters[0].reportBuffer.Len(); got != 0 {
		t.Fatalf("observer buffer depth after recovery = %d, want 0", got)
	}
	if a.cfg.EnableCommands {
		t.Fatal("observer response changed authoritative command configuration")
	}
	if primaryRequests.Load() != 1 || observerRequests.Load() != 3 {
		t.Fatalf("requests primary=%d observer=%d, want 1 and 3", primaryRequests.Load(), observerRequests.Load())
	}
}

func TestObserverReporterPlaintextPolicyIsDestinationScoped(t *testing.T) {
	target := ObserverTarget{
		Name:     "observer",
		ID:       "observer-id",
		PulseURL: "http://203.0.113.10:7655",
		APIToken: "observer-token",
	}
	if _, err := newObserverReporters([]ObserverTarget{target}); err == nil {
		t.Fatal("expected observer plaintext URL rejection without destination opt-in")
	}
	target.AllowPlaintextHTTP = true
	reporters, err := newObserverReporters([]ObserverTarget{target})
	if err != nil {
		t.Fatalf("explicit observer plaintext opt-in: %v", err)
	}
	if len(reporters) != 1 || !reporters[0].target.AllowPlaintextHTTP {
		t.Fatalf("observer plaintext policy was not preserved: %+v", reporters)
	}
}
