package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

type pbsHealthTestMode int32

const (
	pbsHealthTestSuccess pbsHealthTestMode = iota
	pbsHealthTestAuthFailure
	pbsHealthTestTimeout
	pbsHealthTestPartialData
)

type pbsHealthTestServer struct {
	server        *httptest.Server
	mode          atomic.Int32
	requiredToken atomic.Value
}

func newPBSHealthTestServer(t *testing.T) *pbsHealthTestServer {
	t.Helper()

	fixture := &pbsHealthTestServer{}
	fixture.mode.Store(int32(pbsHealthTestSuccess))
	fixture.requiredToken.Store("")
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requiredToken := fixture.requiredToken.Load().(string); requiredToken != "" &&
			!strings.Contains(r.Header.Get("Authorization"), requiredToken) {
			http.Error(w, "authentication failed: 401 Unauthorized", http.StatusUnauthorized)
			return
		}
		mode := pbsHealthTestMode(fixture.mode.Load())
		switch mode {
		case pbsHealthTestAuthFailure:
			http.Error(w, "authentication failed: 401 Unauthorized", http.StatusUnauthorized)
			return
		case pbsHealthTestTimeout:
			<-r.Context().Done()
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/version":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"version": "3.4.2"},
			})
		case "/api2/json/nodes/localhost/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"cpu":    0.15,
					"memory": map[string]any{"used": 512, "total": 1024},
					"uptime": 120,
				},
			})
		case "/api2/json/admin/datastore":
			if mode == pbsHealthTestPartialData {
				http.Error(w, "datastore inventory temporarily unavailable", http.StatusServiceUnavailable)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"store": "backups",
					"total": 1000,
					"used":  250,
					"avail": 750,
				}},
			})
		case "/api2/json/admin/datastore/backups/namespace":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		}
	}))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (s *pbsHealthTestServer) setMode(mode pbsHealthTestMode) {
	s.mode.Store(int32(mode))
}

func (s *pbsHealthTestServer) requireToken(token string) {
	s.requiredToken.Store(token)
}

func newPBSHealthAuthorityMonitor(instances []config.PBSInstance) *Monitor {
	return &Monitor{
		config:           &config.Config{PBSInstances: instances},
		state:            models.NewState(),
		pbsClients:       make(map[string]*pbs.Client),
		authFailures:     make(map[string]int),
		lastAuthAttempt:  make(map[string]time.Time),
		pollStatusMap:    make(map[string]*pollStatus),
		circuitBreakers:  make(map[string]*circuitBreaker),
		failureCounts:    make(map[string]int),
		lastOutcome:      make(map[string]taskOutcome),
		stalenessTracker: NewStalenessTracker(nil),
	}
}

func newPBSHealthTestClient(t *testing.T, host string) *pbs.Client {
	return newPBSHealthTestClientWithToken(t, host, "secret")
}

func newPBSHealthTestClientWithToken(t *testing.T, host, token string) *pbs.Client {
	t.Helper()
	client, err := pbs.NewClient(pbs.ClientConfig{
		Host:       host,
		TokenName:  "root@pam!pulse",
		TokenValue: token,
		Timeout:    75 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new PBS client: %v", err)
	}
	return client
}

func pbsInstanceByName(t *testing.T, snapshot models.StateSnapshot, name string) models.PBSInstance {
	t.Helper()
	for _, instance := range snapshot.PBSInstances {
		if instance.Name == name {
			return instance
		}
	}
	t.Fatalf("PBS instance %q missing from snapshot: %+v", name, snapshot.PBSInstances)
	return models.PBSInstance{}
}

func assertPBSConnectionProjection(
	t *testing.T,
	monitor *Monitor,
	name string,
	wantConnected bool,
	wantStatus string,
) {
	t.Helper()
	snapshot := monitor.state.GetSnapshot()
	instance := pbsInstanceByName(t, snapshot, name)
	if got := snapshot.ConnectionHealth["pbs-"+name]; got != wantConnected {
		t.Fatalf("%s connection-health map = %v, want %v", name, got, wantConnected)
	}
	if got := instance.Status; got != wantStatus {
		t.Fatalf("%s dashboard status = %q, want %q", name, got, wantStatus)
	}
	wantResourceHealth := "healthy"
	if !wantConnected {
		wantResourceHealth = "error"
	}
	if got := instance.ConnectionHealth; got != wantResourceHealth {
		t.Fatalf("%s dashboard connection health = %q, want %q", name, got, wantResourceHealth)
	}
}

func TestPollPBSInstancesKeepsAllHealthProjectionsOnOneOutcome(t *testing.T) {
	first := newPBSHealthTestServer(t)
	second := newPBSHealthTestServer(t)
	instances := []config.PBSInstance{
		{Name: "pbs-primary", Host: first.server.URL, MonitorDatastores: true},
		{Name: "pbs-secondary", Host: second.server.URL, MonitorDatastores: true},
	}
	monitor := newPBSHealthAuthorityMonitor(instances)
	clients := map[string]*pbs.Client{
		"pbs-primary":   newPBSHealthTestClient(t, first.server.URL),
		"pbs-secondary": newPBSHealthTestClient(t, second.server.URL),
	}

	for name, client := range clients {
		monitor.pollPBSInstance(context.Background(), name, client)
		assertPBSConnectionProjection(t, monitor, name, true, "online")
	}
	primarySuccess := monitor.pollStatusMap["pbs::pbs-primary"].LastSuccess
	secondarySuccess := monitor.pollStatusMap["pbs::pbs-secondary"].LastSuccess
	if primarySuccess.IsZero() || secondarySuccess.IsZero() {
		t.Fatalf("initial successes were not recorded: primary=%s secondary=%s", primarySuccess, secondarySuccess)
	}

	first.setMode(pbsHealthTestAuthFailure)
	second.setMode(pbsHealthTestTimeout)
	monitor.pollPBSInstance(context.Background(), "pbs-primary", clients["pbs-primary"])
	monitor.pollPBSInstance(context.Background(), "pbs-secondary", clients["pbs-secondary"])

	for name, previousSuccess := range map[string]time.Time{
		"pbs-primary":   primarySuccess,
		"pbs-secondary": secondarySuccess,
	} {
		status := monitor.pollStatusMap["pbs::"+name]
		if status == nil || status.LastErrorMessage == "" || status.ConsecutiveFailures != 1 {
			t.Fatalf("%s failure ledger = %+v, want one current failure", name, status)
		}
		if !status.LastSuccess.Equal(previousSuccess) {
			t.Fatalf("%s last success changed on failure: got %s want %s", name, status.LastSuccess, previousSuccess)
		}
		assertPBSConnectionProjection(t, monitor, name, false, "offline")
	}

	first.setMode(pbsHealthTestSuccess)
	first.requireToken("rotated-secret")
	monitor.pollPBSInstance(context.Background(), "pbs-primary", clients["pbs-primary"])
	assertPBSConnectionProjection(t, monitor, "pbs-primary", false, "offline")

	clients["pbs-primary"] = newPBSHealthTestClientWithToken(t, first.server.URL, "rotated-secret")
	monitor.pollPBSInstance(context.Background(), "pbs-primary", clients["pbs-primary"])
	assertPBSConnectionProjection(t, monitor, "pbs-primary", true, "online")

	replacement := newPBSHealthTestServer(t)
	monitor.config.PBSInstances[1].Host = replacement.server.URL
	clients["pbs-secondary"] = newPBSHealthTestClient(t, replacement.server.URL)
	time.Sleep(time.Millisecond)
	monitor.pollPBSInstance(context.Background(), "pbs-secondary", clients["pbs-secondary"])
	if got := pbsInstanceByName(t, monitor.state.GetSnapshot(), "pbs-secondary").Host; got != replacement.server.URL {
		t.Fatalf("reconfigured PBS host = %q, want %q", got, replacement.server.URL)
	}

	for name := range clients {
		status := monitor.pollStatusMap["pbs::"+name]
		if status == nil || !status.LastErrorAt.IsZero() || status.LastErrorMessage != "" || status.ConsecutiveFailures != 0 {
			t.Fatalf("%s recovery ledger = %+v, want cleared current failure", name, status)
		}
		assertPBSConnectionProjection(t, monitor, name, true, "online")
	}

	restarted := newPBSHealthAuthorityMonitor(monitor.config.PBSInstances)
	for name, client := range clients {
		restarted.pollPBSInstance(context.Background(), name, client)
		assertPBSConnectionProjection(t, restarted, name, true, "online")
	}
}

func TestPollPBSInstanceKeepsPartialDataSeparateFromConnectivity(t *testing.T) {
	fixture := newPBSHealthTestServer(t)
	fixture.setMode(pbsHealthTestPartialData)
	instance := config.PBSInstance{
		Name:              "pbs-partial",
		Host:              fixture.server.URL,
		MonitorDatastores: true,
	}
	monitor := newPBSHealthAuthorityMonitor([]config.PBSInstance{instance})

	monitor.pollPBSInstance(context.Background(), instance.Name, newPBSHealthTestClient(t, instance.Host))

	assertPBSConnectionProjection(t, monitor, instance.Name, true, "online")
	status := monitor.pollStatusMap["pbs::"+instance.Name]
	if status == nil || status.LastSuccess.IsZero() || status.LastErrorMessage != "" {
		t.Fatalf("partial collection must retain successful connectivity, got %+v", status)
	}
	published := pbsInstanceByName(t, monitor.state.GetSnapshot(), instance.Name)
	if len(published.Datastores) != 0 {
		t.Fatalf("partial datastore inventory = %+v, want no fabricated rows", published.Datastores)
	}
}

func TestInitPBSClientsDoesNotTreatClientConstructionAsConnectivity(t *testing.T) {
	instances := []config.PBSInstance{
		{
			Name:       "pbs-awaiting-poll",
			Host:       "https://backup.local:8007",
			TokenName:  "root@pam!pulse",
			TokenValue: "secret",
		},
		{
			Name:       "pbs-invalid-url",
			Host:       "://not-a-url",
			TokenName:  "root@pam!pulse",
			TokenValue: "secret",
		},
	}
	monitor := newPBSHealthAuthorityMonitor(instances)

	monitor.initPBSClients(monitor.config)

	snapshot := monitor.state.GetSnapshot()
	if _, ok := snapshot.ConnectionHealth["pbs-pbs-awaiting-poll"]; ok {
		t.Fatal("successful client construction fabricated a connected health result")
	}
	if _, ok := monitor.pollStatusMap["pbs::pbs-awaiting-poll"]; ok {
		t.Fatal("successful client construction fabricated a completed poll")
	}

	status := monitor.pollStatusMap["pbs::pbs-invalid-url"]
	if status == nil || status.LastErrorMessage == "" || status.ConsecutiveFailures != 1 {
		t.Fatalf("invalid URL initialization ledger = %+v, want one current failure", status)
	}
	assertPBSConnectionProjection(t, monitor, "pbs-invalid-url", false, "offline")
}
