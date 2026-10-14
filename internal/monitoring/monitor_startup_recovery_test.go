package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Advance the real retry timers with Go's virtual clock, not shorter delays or
// a changed cutoff. A provider returning after six minutes must become eligible
// for ordinary polling on the same running monitor, with unchanged TLS inputs.
func TestConnectionRetrySurvivesLongStartupOutage(t *testing.T) {
	original := newProxmoxClientFunc
	defer func() { newProxmoxClientFunc = original }()
	synctest.Test(t, func(t *testing.T) {
		instance := config.PVEInstance{Name: "late-pve", Host: "https://pve.local:8006", User: "fixture@pam", Password: "fixture-password", VerifySSL: true, Fingerprint: "fixture-saved-pin"}
		m := &Monitor{config: &config.Config{PVEInstances: []config.PVEInstance{instance}, ConnectionTimeout: 3 * time.Second}, pveClients: make(map[string]PVEClientInterface), pbsClients: make(map[string]*pbs.Client), state: models.NewState()}
		started := time.Now()
		var attempts []time.Duration
		newProxmoxClientFunc = func(cfg proxmox.ClientConfig) (PVEClientInterface, error) {
			want := config.CreateProxmoxConfig(&instance)
			want.Timeout = 3 * time.Second
			if !reflect.DeepEqual(cfg, want) {
				t.Fatal("retry changed configured authority, credentials, TLS pin or timeout")
			}
			elapsed := time.Since(started)
			attempts = append(attempts, elapsed)
			if elapsed < 6*time.Minute {
				return nil, fmt.Errorf("fixture provider still starting")
			}
			return &mockPVEClientExtra{}, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		m.retryFailedConnections(ctx)
		client, exists := m.getPVEClient(instance.Name)
		if !exists || client == nil || len(newPVEPollProvider().ListInstances(m)) != 1 {
			t.Fatalf("six-minute startup outage left the connection permanently outside polling; attempts=%v", attempts)
		}
		want := []time.Duration{5 * time.Second, 15 * time.Second, 35 * time.Second, 75 * time.Second, 135 * time.Second, 195 * time.Second, 255 * time.Second, 315 * time.Second, 375 * time.Second}
		if !reflect.DeepEqual(attempts, want) {
			t.Fatalf("retry cadence=%v, want capped schedule=%v", attempts, want)
		}
		t.Logf("late startup recovered after %s, attempts=%v; ordinary provider lists the retained client", attempts[len(attempts)-1], attempts)
	})
}

func TestConnectionRetryLongOutageStopsOnCancellation(t *testing.T) {
	original := newProxmoxClientFunc
	defer func() { newProxmoxClientFunc = original }()
	synctest.Test(t, func(t *testing.T) {
		m := &Monitor{config: &config.Config{PVEInstances: []config.PVEInstance{{Name: "still-unavailable", Host: "https://pve.local:8006"}}}, pveClients: make(map[string]PVEClientInterface), pbsClients: make(map[string]*pbs.Client), state: models.NewState()}
		calls := 0
		newProxmoxClientFunc = func(proxmox.ClientConfig) (PVEClientInterface, error) {
			calls++
			return nil, fmt.Errorf("fixture provider still unavailable")
		}
		started := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute+10*time.Second)
		defer cancel()
		m.retryFailedConnections(ctx)
		if elapsed := time.Since(started); elapsed != 6*time.Minute+10*time.Second || calls != 8 || len(m.pveClients) != 0 {
			t.Fatalf("shutdown must interrupt the capped wait without another attempt: elapsed=%s calls=%d clients=%d", elapsed, calls, len(m.pveClients))
		}
	})
}

func TestConnectionRetryDoesNotResumePausedConnections(t *testing.T) {
	original := newProxmoxClientFunc
	defer func() { newProxmoxClientFunc = original }()
	synctest.Test(t, func(t *testing.T) {
		m := &Monitor{config: &config.Config{PVEInstances: []config.PVEInstance{{Name: "paused-pve", Disabled: true}}, PBSInstances: []config.PBSInstance{{Name: "paused-pbs", Disabled: true, Host: "://invalid"}}}, pveClients: make(map[string]PVEClientInterface), pbsClients: make(map[string]*pbs.Client), state: models.NewState()}
		newProxmoxClientFunc = func(proxmox.ClientConfig) (PVEClientInterface, error) {
			t.Fatal("retry attempted a paused connection")
			return nil, nil
		}
		m.retryFailedConnections(context.Background())
		if len(m.pveClients) != 0 || len(m.pbsClients) != 0 || len(m.state.GetSnapshot().ConnectionHealth) != 0 {
			t.Fatal("paused connections were recreated or published as healthy")
		}
	})
}

// The clock test above exercises the production six-minute/capped schedule.
// This independent, closed TLS fixture exercises PBS's actual constructor,
// scheduler admission, first poll and broadcast producer. Only the waits are
// shortened here; no credentials, trust, daemon, backup or guest operation runs.
func TestPBSStartupRecoveryKeepsTrustAndFirstPollBoundary(t *testing.T) {
	var logins atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/access/ticket" {
			if logins.Add(1) <= 2 {
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprint(w, `{"message":"fixture starting"}`)
				return
			}
			fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
			return
		}
		if r.Method != http.MethodGet || !strings.Contains(r.Header.Get("Cookie"), "fixture-ticket") {
			t.Errorf("ordinary PBS read lost authentication or used a mutation: %s %s", r.Method, r.URL.Path)
			http.Error(w, "fixture refuses unauthenticated reads", http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/api2/json/version":
			fmt.Fprint(w, `{"data":{"version":"3.4.2"}}`)
		case "/api2/json/nodes/localhost/status":
			fmt.Fprint(w, `{"data":{"cpu":0.15,"memory":{"used":512,"total":1024},"uptime":120}}`)
		case "/api2/json/admin/datastore":
			fmt.Fprint(w, `{"data":[{"store":"backups","total":1000,"used":250,"avail":750}]}`)
		case "/api2/json/admin/datastore/backups/status":
			fmt.Fprint(w, `{"data":{"status":"available","total":1000,"used":250,"avail":750}}`)
		default:
			fmt.Fprint(w, `{"data":[]}`)
		}
	}))
	defer server.Close()
	pin := sha256.Sum256(server.Certificate().Raw)
	instance := config.PBSInstance{Name: "startup-pbs", Host: server.URL, User: "fixture@pbs", Password: "fixture-password", VerifySSL: true, Fingerprint: hex.EncodeToString(pin[:]), MonitorDatastores: true}
	m := newPBSHealthAuthorityMonitor([]config.PBSInstance{instance})
	project := func(phase string) {
		snapshot, scope := m.currentStateWithScope()
		frontend := m.buildBroadcastFrontendStateFromSnapshot(snapshot, scope)
		if phase == "recovered" {
			found := false
			for _, resource := range frontend.Resources {
				if resource.Type == "pbs" {
					found = true
					if resource.CPU == nil || resource.CPU.Current != 15 || resource.Memory == nil || resource.Memory.Current != 50 {
						t.Fatalf("broadcast lost the canonical host percentages: CPU=%+v memory=%+v", resource.CPU, resource.Memory)
					}
				}
			}
			if !found {
				t.Fatal("broadcast omitted the recovered PBS server")
			}
		}
		wire, err := json.Marshal(struct {
			Phase string               `json:"phase"`
			State models.StateFrontend `json:"state"`
		}{phase, frontend})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("PBS_STARTUP_PROJECTION_JSON=%s", wire)
	}
	m.initPBSClients(m.config)
	assertPBSConnectionProjection(t, m, instance.Name, false, "offline")
	if len(newPBSPollProvider().ListInstances(m)) != 0 {
		t.Fatal("failed constructor entered polling")
	}
	project("unavailable")
	originalDelays := connRetryDelays
	connRetryDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	defer func() { connRetryDelays = originalDelays }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m.retryFailedConnections(ctx)
	client, exists := m.getPBSClient(instance.Name)
	if !exists || client == nil || logins.Load() != 3 || len(newPBSPollProvider().ListInstances(m)) != 1 {
		t.Fatalf("ordinary PBS startup recovery missing or duplicated: exists=%t logins=%d", exists, logins.Load())
	}
	assertPBSConnectionProjection(t, m, instance.Name, false, "offline")
	project("awaiting-poll")
	task, err := newPBSPollProvider().BuildPollTask(m, instance.Name)
	if err != nil {
		t.Fatal(err)
	}
	newRealExecutor(m).Execute(ctx, task)
	assertPBSConnectionProjection(t, m, instance.Name, true, "online")
	published := pbsInstanceByName(t, m.state.GetSnapshot(), instance.Name)
	// Legacy PBS state keeps the provider CPU ratio; the canonical broadcast
	// converts it to percent (asserted by project), not by rewriting the poll.
	if published.CPU != 0.15 || published.Memory != 50 || len(published.Datastores) != 1 || published.Datastores[0].Usage != 25 || logins.Load() != 3 || !reflect.DeepEqual(instance, m.config.PBSInstances[0]) {
		t.Fatalf("recovery lost native readings/identity/trust or replayed authentication: %+v logins=%d", published, logins.Load())
	}
	project("recovered")

	// A different saved pin is still a refusal, not recovery authority. The
	// same real retry path must not make a password-bearing HTTP request.
	wrong := instance
	wrong.Name = "wrong-pin-pbs"
	wrong.Fingerprint = strings.Repeat("0", 64)
	untrusted := newPBSHealthAuthorityMonitor([]config.PBSInstance{wrong})
	untrusted.initPBSClients(untrusted.config)
	refusedCtx, refusedCancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer refusedCancel()
	untrusted.retryFailedConnections(refusedCtx)
	if len(untrusted.pbsClients) != 0 || logins.Load() != 3 || untrusted.config.PBSInstances[0].Fingerprint != wrong.Fingerprint {
		t.Fatal("retry bypassed or replaced a saved TLS pin")
	}
	assertPBSConnectionProjection(t, untrusted, wrong.Name, false, "offline")
}
