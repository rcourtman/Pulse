package hostagent

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

func TestObserverAuthGuidanceAndSameCredentialRecovery(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			const token = "synthetic-observer-credential-not-for-logs"
			const responseSecret = "synthetic-observer-body-not-for-logs"
			var responseStatus atomic.Int32
			responseStatus.Store(int32(status))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != agentReportEndpoint || r.Header.Get("X-API-Token") != token || r.Header.Get("Authorization") != "Bearer "+token {
					t.Error("ordinary observer report changed its endpoint or credential")
				}
				code := int(responseStatus.Load())
				w.WriteHeader(code)
				if code == http.StatusOK {
					_, _ = io.WriteString(w, `{"success":true,"config":{"commandsEnabled":true}}`)
				} else {
					_, _ = io.WriteString(w, responseSecret)
				}
			}))
			defer server.Close()
			var logs bytes.Buffer
			primaryAcknowledgements := 0
			a := &Agent{
				cfg: Config{APIToken: "synthetic-primary-credential", CommandAuthorityProfile: CommandAuthorityMonitoringOnly,
					OnPrimaryReportAccepted: func() { primaryAcknowledgements++ }},
				logger: zerolog.New(&logs), reportBuffer: utils.New[agentshost.Report](8),
			}
			observer := &observerReporter{
				target:     ObserverTarget{Name: "synthetic-destination", PulseURL: server.URL, APIToken: token},
				httpClient: server.Client(), reportBuffer: utils.New[agentshost.Report](8),
			}
			report := agentshost.Report{Agent: agentshost.AgentInfo{ID: "auth-diagnostic-agent"}}
			for range 2 {
				a.deliverObserverReport(context.Background(), observer, report)
			}
			assertSafeReportAuthLog(t, logs.String(), server.URL, status, true, token, responseSecret)
			if got := strings.Count(logs.String(), "Pulse observer rejected host reporting"); got != 1 || !observer.reportBuffer.IsEmpty() {
				t.Fatal("observer rejection did not stay bounded and throttled")
			}
			responseStatus.Store(http.StatusOK)
			a.deliverObserverReport(context.Background(), observer, report)
			if !observer.lastAuthFailureLog.IsZero() || !observer.reportBuffer.IsEmpty() || primaryAcknowledgements != 0 || a.cfg.EnableCommands || a.cfg.CommandAuthorityProfile != CommandAuthorityMonitoringOnly || a.cfg.APIToken != "synthetic-primary-credential" || observer.target.APIToken != token {
				t.Fatal("observer recovery changed credentials, primary readiness or command authority")
			}
			responseStatus.Store(int32(status))
			a.deliverObserverReport(context.Background(), observer, report)
			if got := strings.Count(logs.String(), "Pulse observer rejected host reporting"); got != 2 {
				t.Fatal("ordinary recovery did not reset the observer diagnostic throttle")
			}
		})
	}
}

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
