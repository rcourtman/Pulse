package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const metricAuthorityToken = "metric-authority-primary.12345678"
const metricAuthorityControl = "metric-authority-control.12345678"
const metricAuthorityID = "physical-disk:fixture-node:sda"

// Pause the real resource lookup after route admission. This is a normal
// provider boundary, not an auth stub, database lock or production test hook.
type metricAuthorityProvider struct {
	staticPhysicalDiskResourceStore
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	calls   atomic.Int32
	armed   atomic.Bool
}

func (p *metricAuthorityProvider) GetAll() []unified.Resource {
	if p.armed.Load() && p.calls.Add(1) <= 2 {
		p.entered <- struct{}{}
		<-p.release
	}
	return p.staticPhysicalDiskResourceStore.GetAll()
}
func (p *metricAuthorityProvider) ReadSnapshot() models.StateSnapshot { return models.StateSnapshot{} }
func (p *metricAuthorityProvider) UnifiedResourceSnapshot() ([]unified.Resource, time.Time) {
	return p.GetAll(), time.Now()
}
func (p *metricAuthorityProvider) unblock() { p.once.Do(func() { close(p.release) }) }

type metricAuthorityFixture struct {
	router   *Router
	server   *httptest.Server
	provider *metricAuthorityProvider
	path     string
}

func newMetricAuthorityFixture(t *testing.T, entry string, scopes []string) *metricAuthorityFixture {
	t.Helper()
	setMockModeForTest(t, false)
	t.Setenv("PULSE_AUTH_BYPASS", "false")
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, metricAuthorityToken, scopes, nil),
		newTokenRecord(t, metricAuthorityControl, []string{config.ScopeWildcard}, nil),
	)
	cfg.AuthUser, cfg.AuthPass = "operator", "fixture-nonempty-hash"
	cfg.ProxyAuthSecret, cfg.ProxyAuthUserHeader = "metric-authority-proxy-secret", "X-Remote-User"
	InitSessionStore(cfg.DataPath)
	p := &metricAuthorityProvider{
		staticPhysicalDiskResourceStore: staticPhysicalDiskResourceStore{resources: []unified.Resource{{
			ID: metricAuthorityID, Type: unified.ResourceTypePhysicalDisk, Name: "synthetic-disk",
			PhysicalDisk:  &unified.PhysicalDiskMeta{Serial: "metric-authority-serial", Temperature: 37},
			MetricsTarget: &unified.MetricsTarget{ResourceType: "disk", ResourceID: metricAuthorityID},
		}}},
		entered: make(chan struct{}, 2), release: make(chan struct{}),
	}
	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", models.NewState())
	setUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))
	monitor.SetResourceStore(p)
	router := NewRouter(cfg, nil, nil, nil, nil, "test")
	router.monitor = monitor
	router.resourceHandlers.SetStateProvider(p)
	// SetResourceStore publishes an initial view synchronously. Only later
	// request-owned lookups belong to the withdrawal experiment.
	p.armed.Store(true)
	t.Cleanup(router.ShutdownResourceStores)
	path := "/api/metrics-store/history"
	q := url.Values{"resourceType": {"disk"}, "resourceId": {metricAuthorityID}, "metric": {"smart_temp"}, "range": {"1h"}}
	if entry == "agent" {
		path = agentcapabilities.MetricHistoryCapabilityPath
		q.Del("resourceType")
	}
	f := &metricAuthorityFixture{router: router, provider: p, path: path + "?" + q.Encode()}
	f.server = httptest.NewServer(router.Handler())
	t.Cleanup(f.server.Close)
	t.Cleanup(p.unblock)
	return f
}

func (f *metricAuthorityFixture) request(t *testing.T, raw, transport string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, f.server.URL+f.path, nil)
	if err != nil {
		t.Fatal(err)
	}
	switch transport {
	case "header":
		req.Header.Set("X-API-Token", raw)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+raw)
	case "session":
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
	case "proxy":
		req.Header.Set("X-Proxy-Secret", f.router.config.ProxyAuthSecret)
		req.Header.Set("X-Remote-User", "operator")
	default:
		t.Fatalf("unknown metric fixture transport %q", transport)
	}
	return req
}
func (f *metricAuthorityFixture) begin(t *testing.T, raw, transport string) <-chan historyAuthorityResult {
	t.Helper()
	req := f.request(t, raw, transport)
	result := make(chan historyAuthorityResult, 1)
	go func() { response, err := f.server.Client().Do(req); result <- historyAuthorityResult{response, err} }()
	select {
	case <-f.provider.entered:
	case got := <-result:
		if got.response != nil {
			got.response.Body.Close()
		}
		t.Fatalf("metric history returned before its inventory gate: %v", got.err)
	case <-time.After(5 * time.Second):
		t.Fatal("metric history did not reach its inventory gate")
	}
	return result
}

type metricAuthorityWire struct {
	Status int         `json:"status"`
	Header http.Header `json:"header"`
	Body   string      `json:"body"`
}

func readMetricAuthorityResult(t *testing.T, result <-chan historyAuthorityResult, status int) metricAuthorityWire {
	t.Helper()
	select {
	case got := <-result:
		if got.err != nil {
			t.Fatal(got.err)
		}
		defer got.response.Body.Close()
		body, err := io.ReadAll(got.response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if got.response.StatusCode != status {
			t.Errorf("metric history status = %d, want %d: %s", got.response.StatusCode, status, body)
		}
		if status != http.StatusOK {
			if string(body) != "Authentication required\n" || strings.Contains(got.response.Header.Get("Content-Type"), "json") || strings.Contains(string(body), metricAuthorityID) {
				t.Error("withdrawn history disclosed data or implied successful empty JSON")
			}
			return metricAuthorityWire{got.response.StatusCode, got.response.Header, string(body)}
		}
		var history struct {
			metricsHistoryResponse
			Metrics map[string][]struct {
				Timestamp int64   `json:"timestamp"`
				Value     float64 `json:"value"`
			} `json:"metrics"`
		}
		if json.Unmarshal(body, &history) != nil {
			t.Fatal("invalid history JSON")
		}
		if history.Metric == "" {
			history.Points = history.Metrics["smart_temp"]
		}
		if history.ResourceId != metricAuthorityID || history.ResourceType != "disk" || history.Source != "live" || len(history.Points) != 1 || history.Points[0].Value != 37 {
			t.Errorf("authorised history lost its real temperature and existing envelope: %s", body)
		}
		if got.response.Header.Get("Content-Type") != "application/json" {
			t.Error("history media changed")
		}
		return metricAuthorityWire{got.response.StatusCode, got.response.Header, string(body)}
	case <-time.After(5 * time.Second):
		t.Fatal("metric history did not finish after inventory release")
	}
	return metricAuthorityWire{}
}

// Revocation uses the registered persisted DELETE, not only a fixture edit.
func (f *metricAuthorityFixture) revokeThroughHTTP(t *testing.T) {
	t.Helper()
	id := f.router.config.APITokens[0].ID
	req, err := http.NewRequest(http.MethodDelete, f.server.URL+"/api/security/tokens/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-API-Token", metricAuthorityControl)
	response, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != http.StatusNoContent {
		t.Fatalf("token deletion = %d %s %v", response.StatusCode, body, err)
	}
	persisted, err := f.router.persistence.LoadAPITokens()
	if err != nil || len(persisted) != 1 || persisted[0].ID == id {
		t.Fatalf("metric token revocation not persisted: %v", err)
	}
}

func TestMetricHistoryWithdrawnSessionCannotDisclose(t *testing.T) {
	for _, entry := range []string{"chart", "agent"} {
		for _, change := range []string{"logout", "expired", "replaced", "principal", "store"} {
			t.Run(entry+"/"+change, func(t *testing.T) {
				f := newMetricAuthorityFixture(t, entry, []string{config.ScopeMonitoringRead})
				const raw = "metric-authority-session"
				GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", f.router.config.AuthUser)
				t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
				control := f.begin(t, metricAuthorityControl, "header")
				primary := f.begin(t, raw, "session")
				switch change {
				case "logout":
					logoutReadSession(t, f.server.URL, raw)
				case "store":
					InitSessionStore(t.TempDir())
				default:
					mutateReadSession(raw, change)
				}
				f.provider.unblock()
				readMetricAuthorityResult(t, primary, http.StatusUnauthorized)
				readMetricAuthorityResult(t, control, http.StatusOK)
			})
		}
	}
}

func TestMetricHistoryAllSeriesCannotDiscloseAfterLogout(t *testing.T) {
	for _, entry := range []string{"chart", "agent"} {
		t.Run(entry, func(t *testing.T) {
			f := newMetricAuthorityFixture(t, entry, []string{config.ScopeMonitoringRead})
			f.path = strings.ReplaceAll(f.path, "metric=smart_temp&", "")
			const raw = "metric-authority-all-series-session"
			GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", f.router.config.AuthUser)
			t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
			control := f.begin(t, metricAuthorityControl, "header")
			primary := f.begin(t, raw, "session")
			logoutReadSession(t, f.server.URL, raw)
			f.provider.unblock()
			readMetricAuthorityResult(t, primary, http.StatusUnauthorized)
			readMetricAuthorityResult(t, control, http.StatusOK)
		})
	}
}

func TestMetricHistoryKeepsInitialAdmissionGates(t *testing.T) {
	for _, entry := range []string{"chart", "agent"} {
		t.Run(entry, func(t *testing.T) {
			f := newMetricAuthorityFixture(t, entry, []string{config.ScopeSettingsRead})
			for _, token := range []string{"", metricAuthorityToken} {
				req := f.request(t, token, "header")
				response, err := f.server.Client().Do(req)
				if err != nil {
					t.Fatal(err)
				}
				response.Body.Close()
				want := http.StatusForbidden
				if token == "" {
					want = http.StatusUnauthorized
				}
				if response.StatusCode != want || f.provider.calls.Load() != 0 {
					t.Fatalf("initial history gate = %d, want %d; inventory calls = %d", response.StatusCode, want, f.provider.calls.Load())
				}
			}
			req := f.request(t, metricAuthorityControl, "header")
			req.Method = http.MethodPost
			response, err := f.server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusMethodNotAllowed || f.provider.calls.Load() != 0 {
				t.Fatal("history method gate changed")
			}
		})
	}
}
