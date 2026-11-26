package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
)

func newPVEStartupTestMonitor(instances []config.PVEInstance) *Monitor {
	return &Monitor{
		config: &config.Config{
			PVEInstances:      instances,
			ConnectionTimeout: 2 * time.Second,
		},
		state:            models.NewState(),
		pveClients:       make(map[string]PVEClientInterface),
		pbsClients:       make(map[string]*pbs.Client),
		pmgClients:       make(map[string]*pmg.Client),
		authFailures:     make(map[string]int),
		lastAuthAttempt:  make(map[string]time.Time),
		pollStatusMap:    make(map[string]*pollStatus),
		circuitBreakers:  make(map[string]*circuitBreaker),
		failureCounts:    make(map[string]int),
		lastOutcome:      make(map[string]taskOutcome),
		stalenessTracker: NewStalenessTracker(nil),
		nodeLastOnline:   make(map[string]time.Time),
	}
}

func TestInitPVEClientsDoesNotTreatClientConstructionAsConnectivity(t *testing.T) {
	instances := []config.PVEInstance{
		{
			Name:       "pve-awaiting-poll",
			Host:       "https://pve.local:8006",
			TokenName:  "root@pam!pulse",
			TokenValue: "secret",
		},
		{
			Name:        "pve-cluster-awaiting-poll",
			Host:        "https://pve-c1.local:8006",
			TokenName:   "root@pam!pulse",
			TokenValue:  "secret",
			IsCluster:   true,
			ClusterName: "lab-cluster",
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "pve-c1", Host: "https://pve-c1.local:8006"},
			},
		},
		{
			Name:       "pve-invalid-url",
			Host:       "://not-a-url",
			TokenName:  "root@pam!pulse",
			TokenValue: "secret",
		},
		{
			Name:       "pve-paused",
			Host:       "://not-a-url",
			TokenName:  "root@pam!pulse",
			TokenValue: "secret",
			Disabled:   true,
		},
	}
	monitor := newPVEStartupTestMonitor(instances)

	monitor.initPVEClients(monitor.config)

	snapshot := monitor.state.GetSnapshot()
	for _, name := range []string{"pve-awaiting-poll", "pve-cluster-awaiting-poll"} {
		if _, ok := snapshot.ConnectionHealth[name]; ok {
			t.Fatalf("%s: successful client construction fabricated a connected health result", name)
		}
		if _, ok := monitor.pollStatusMap["pve::"+name]; ok {
			t.Fatalf("%s: successful client construction fabricated a completed poll", name)
		}
		if connected := monitor.GetConnectionStatuses()["pve-"+name]; connected {
			t.Fatalf("%s: GetConnectionStatuses() reported connected before first poll", name)
		}
	}

	status := monitor.pollStatusMap["pve::pve-invalid-url"]
	if status == nil || status.LastErrorMessage == "" || status.ConsecutiveFailures != 1 {
		t.Fatalf("invalid URL initialization ledger = %+v, want one current failure", status)
	}
	if healthy, ok := snapshot.ConnectionHealth["pve-invalid-url"]; !ok || healthy {
		t.Fatalf("invalid URL ConnectionHealth = (%v, %v), want (false, true)", healthy, ok)
	}
	if snap, ok := monitor.stalenessTracker.snapshot(InstanceTypePVE, "pve-invalid-url"); !ok || snap.LastError.IsZero() {
		t.Fatalf("invalid URL staleness error not recorded: %+v ok=%v", snap, ok)
	}

	if _, exists := monitor.pveClients["pve-paused"]; exists {
		t.Fatal("paused PVE instance constructed a client")
	}
	if _, exists := monitor.pollStatusMap["pve::pve-paused"]; exists {
		t.Fatal("paused PVE instance recorded a poll status")
	}
	if _, exists := snapshot.ConnectionHealth["pve-paused"]; exists {
		t.Fatal("paused PVE instance published connection health")
	}
}

func TestPVEStartupRecoveryKeepsTrustAndFirstPollBoundary(t *testing.T) {
	var logins atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/access/ticket" {
			switch logins.Add(1) {
			case 1:
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprint(w, `{"message":"fixture starting"}`)
				return
			case 2:
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"message":"authentication failure"}`)
				return
			}
			fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
			return
		}
		if r.Method != http.MethodGet || !strings.Contains(r.Header.Get("Cookie"), "PVEAuthCookie=fixture-ticket") {
			t.Errorf("ordinary PVE read lost authentication or used a mutation: %s %s", r.Method, r.URL.Path)
			http.Error(w, "fixture refuses unauthenticated reads", http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"pve-node-1","status":"online","cpu":0.12,"maxcpu":4,"mem":2147483648,"maxmem":8589934592,"disk":10737418240,"maxdisk":107374182400,"uptime":3600}]}`)
		case "/api2/json/nodes/pve-node-1/status":
			fmt.Fprint(w, `{"data":{"cpu":0.12,"memory":{"used":2147483648,"total":8589934592,"free":6442450944},"rootfs":{"used":10737418240,"total":107374182400,"avail":96636764160},"uptime":3600,"pveversion":"pve-manager/8.3.0","kversion":"Linux 6.8.12"}}`)
		case "/api2/json/cluster/resources":
			fmt.Fprint(w, `{"data":[]}`)
		case "/api2/json/cluster/status":
			fmt.Fprint(w, `{"data":[{"type":"node","name":"pve-node-1","online":1}]}`)
		default:
			fmt.Fprint(w, `{"data":[]}`)
		}
	}))
	defer server.Close()

	pin := sha256.Sum256(server.Certificate().Raw)
	instance := config.PVEInstance{
		Name:        "startup-pve",
		Host:        server.URL,
		User:        "fixture@pam",
		Password:    "fixture-password",
		VerifySSL:   true,
		Fingerprint: hex.EncodeToString(pin[:]),
	}
	m, err := New(&config.Config{
		DataPath:          t.TempDir(),
		ConnectionTimeout: 2 * time.Second,
		PVEInstances:      []config.PVEInstance{instance},
	})
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(m.Stop)

	// Phase 1: startup client creation failed with 503 during New() -> recorded in pollStatusMap, stalenessTracker, and ConnectionHealth=false.
	if len(newPVEPollProvider().ListInstances(m)) != 0 {
		t.Fatal("failed PVE constructor entered polling")
	}
	status := m.pollStatusMap["pve::startup-pve"]
	if status == nil || status.ConsecutiveFailures != 1 || status.LastErrorMessage == "" {
		t.Fatalf("startup failure ledger = %+v, want one current failure", status)
	}
	if healthy, ok := m.state.GetSnapshot().ConnectionHealth["startup-pve"]; !ok || healthy {
		t.Fatalf("startup failure ConnectionHealth = (%v, %v), want (false, true)", healthy, ok)
	}

	// Phase 2: retryFailedConnections hits 401 on attempt 2 (updating pollStatusMap) then succeeds on attempt 3,
	// without fabricating ConnectionHealth=true prior to the first poll.
	originalDelays := connRetryDelays
	connRetryDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	defer func() { connRetryDelays = originalDelays }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m.retryFailedConnections(ctx)

	client, exists := m.getPVEClient(instance.Name)
	if !exists || client == nil || len(newPVEPollProvider().ListInstances(m)) != 1 {
		t.Fatalf("ordinary PVE startup recovery missing or duplicated: exists=%t logins=%d", exists, logins.Load())
	}
	if status := m.pollStatusMap["pve::startup-pve"]; status == nil || status.ConsecutiveFailures != 2 || !strings.Contains(strings.ToLower(status.LastErrorMessage), "401") {
		t.Fatalf("reconnect 401 failure not retained before first poll: %+v", status)
	}
	if healthy := m.state.GetSnapshot().ConnectionHealth["startup-pve"]; healthy {
		t.Fatal("PVE client recreation fabricated ConnectionHealth=true before first poll")
	}
	if connected := m.GetConnectionStatuses()["pve-startup-pve"]; connected {
		t.Fatal("GetConnectionStatuses() reported PVE connected before first poll")
	}

	// Phase 3: first scheduled poll succeeds -> healthy and clears startup error.
	task, err := newPVEPollProvider().BuildPollTask(m, instance.Name)
	if err != nil {
		t.Fatal(err)
	}
	newRealExecutor(m).Execute(ctx, task)

	if healthy := m.state.GetSnapshot().ConnectionHealth["startup-pve"]; !healthy {
		t.Fatal("first successful PVE poll did not mark ConnectionHealth true")
	}
	if connected := m.GetConnectionStatuses()["pve-startup-pve"]; !connected {
		t.Fatal("GetConnectionStatuses() remained disconnected after first successful PVE poll")
	}
	if status := m.pollStatusMap["pve::startup-pve"]; status == nil || status.ConsecutiveFailures != 0 || status.LastErrorMessage != "" || status.LastSuccess.IsZero() {
		t.Fatalf("first successful PVE poll did not clear startup error: %+v", status)
	}
	if !reflect.DeepEqual(instance, m.config.PVEInstances[0]) {
		t.Fatal("PVE startup recovery mutated saved configuration")
	}

	// A different saved TLS pin remains refused without sending credentials and records the failure.
	loginsBeforeWrongPin := logins.Load()
	wrong := instance
	wrong.Name = "wrong-pin-pve"
	wrong.Fingerprint = strings.Repeat("0", 64)
	untrusted := newPVEStartupTestMonitor([]config.PVEInstance{wrong})
	untrusted.initPVEClients(untrusted.config)
	if len(untrusted.pveClients) != 0 || logins.Load() != loginsBeforeWrongPin {
		t.Fatalf("wrong TLS pin created client or sent request: clients=%d logins=%d want=%d", len(untrusted.pveClients), logins.Load(), loginsBeforeWrongPin)
	}
	if status := untrusted.pollStatusMap["pve::wrong-pin-pve"]; status == nil || status.ConsecutiveFailures != 1 || status.LastErrorMessage == "" {
		t.Fatalf("wrong TLS pin startup failure not recorded in pollStatusMap: %+v", status)
	}
	if healthy, ok := untrusted.state.GetSnapshot().ConnectionHealth["wrong-pin-pve"]; !ok || healthy {
		t.Fatalf("wrong TLS pin ConnectionHealth = (%v, %v), want (false, true)", healthy, ok)
	}
}
