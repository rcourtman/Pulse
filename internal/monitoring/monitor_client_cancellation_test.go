package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

func testStartupClientConstructionHonoursCancellation(t *testing.T) {
	for _, provider := range []string{"pve", "pbs", "cluster"} {
		t.Run(provider, func(t *testing.T) {
			entered, release, requestEnded := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var once, endedOnce, releaseOnce sync.Once
			var requests atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				requests.Add(1)
				if !strings.HasSuffix(r.URL.Path, "/api2/json/access/ticket") {
					t.Errorf("ended startup construction advanced to another request: %s", r.URL.Path)
				}
				once.Do(func() { close(entered) })
				select {
				case <-r.Context().Done():
					endedOnce.Do(func() { close(requestEnded) })
					return
				case <-release:
				}
				fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
			}))
			defer server.Close()
			defer releaseOnce.Do(func() { close(release) })
			pin := sha256.Sum256(server.Certificate().Raw)
			cfg := &config.Config{ConnectionTimeout: 8 * time.Second}
			if provider == "pbs" {
				cfg.PBSInstances = []config.PBSInstance{{Name: "cancelled-pbs", Host: server.URL, User: "fixture@pbs", Password: "fixture-password", VerifySSL: true, Fingerprint: hex.EncodeToString(pin[:])}}
			} else {
				instance := config.PVEInstance{Name: "cancelled-pve", Host: server.URL, User: "fixture@pam", Password: "fixture-password", VerifySSL: true, Fingerprint: hex.EncodeToString(pin[:])}
				if provider == "cluster" {
					// Two distinct base paths on the same trusted local TLS origin
					// exercise synchronous initial health/session checks too.
					instance.IsCluster = true
					instance.ClusterEndpoints = []config.ClusterEndpoint{{Host: server.URL, Fingerprint: instance.Fingerprint}}
					// Canonical authority deduplication would otherwise skip the check.
					instance.ClusterEndpoints[0].Host += "/member"
				}
				cfg.PVEInstances = []config.PVEInstance{instance}
			}
			monitor := &Monitor{config: cfg, state: models.NewState(), pveClients: make(map[string]PVEClientInterface), pbsClients: make(map[string]*pbs.Client)}
			originalDelays := connRetryDelays
			connRetryDelays = []time.Duration{time.Millisecond}
			defer func() { connRetryDelays = originalDelays }()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() { monitor.retryFailedConnections(ctx); close(done) }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				releaseOnce.Do(func() { close(release) })
				cancel()
				<-done
				t.Fatal("startup constructor did not reach local password login")
			}
			cancel()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Error("retry shutdown waited for detached constructor authentication")
				releaseOnce.Do(func() { close(release) })
				<-done
			}
			// Both cluster constructor requests may have started before cancellation.
			if provider != "cluster" {
				select {
				case <-requestEnded:
				case <-time.After(time.Second):
					t.Error("cancelled constructor left the password request active")
				}
			}
			if len(monitor.pveClients) != 0 || len(monitor.pbsClients) != 0 || len(monitor.state.GetSnapshot().ConnectionHealth) != 0 {
				t.Error("cancelled startup construction published a polling client or health outcome")
			}
			if provider != "cluster" && requests.Load() != 1 {
				t.Errorf("shutdown sent more than the admitted fixture login: %d", requests.Load())
			}
		})
	}
}

// Real pinned PBS construction, ordinary retry admission, first poll and the
// production broadcast feed the consuming browser proof. This is not an
// installed/reporter recovery or a reason to restart monitoring on a workload.
func testPBSStartupCancellationKeepsFirstPollBoundary(t *testing.T) {
	var logins, reads atomic.Int32
	entered, release, requestEnded := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/access/ticket" {
			switch logins.Add(1) {
			case 1:
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprint(w, `{"message":"fixture starting"}`)
				return
			case 2:
				close(entered)
				select {
				case <-r.Context().Done():
					close(requestEnded)
					return
				case <-release:
				}
			}
			fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
			return
		}
		reads.Add(1)
		if r.Method != http.MethodGet || !strings.Contains(r.Header.Get("Cookie"), "fixture-ticket") {
			t.Errorf("non-ordinary or unauthenticated PBS fixture read: %s %s", r.Method, r.URL.Path)
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
	defer releaseOnce.Do(func() { close(release) })
	pin := sha256.Sum256(server.Certificate().Raw)
	instance := config.PBSInstance{Name: "cancellable-pbs", Host: server.URL, User: "fixture@pbs", Password: "fixture-password", VerifySSL: true, Fingerprint: hex.EncodeToString(pin[:]), MonitorDatastores: true}
	monitor := newPBSHealthAuthorityMonitor([]config.PBSInstance{instance})
	project := func(phase string) {
		snapshot, scope := monitor.currentStateWithScope()
		frontend := monitor.buildBroadcastFrontendStateFromSnapshot(snapshot, scope)
		wire, err := json.Marshal(struct {
			Phase string               `json:"phase"`
			State models.StateFrontend `json:"state"`
		}{phase, frontend})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("PBS_CANCELLATION_PROJECTION_JSON=%s", wire)
	}
	monitor.initPBSClients(monitor.config)
	assertPBSConnectionProjection(t, monitor, instance.Name, false, "offline")
	project("unavailable")
	originalDelays := connRetryDelays
	connRetryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	defer func() { connRetryDelays = originalDelays }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { monitor.retryFailedConnections(ctx); close(done) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		releaseOnce.Do(func() { close(release) })
		cancel()
		<-done
		t.Fatal("fixture did not reach the reconnect password request")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("ended retry caller still owns a detached password request")
		releaseOnce.Do(func() { close(release) })
		<-done
	}
	select {
	case <-requestEnded:
	case <-time.After(time.Second):
		t.Error("ended retry caller did not cancel its HTTP request")
	}
	if logins.Load() != 2 || reads.Load() != 0 || len(monitor.pbsClients) != 0 || len(newPBSPollProvider().ListInstances(monitor)) != 0 {
		t.Fatalf("cancelled construction published a session or advanced polling: logins=%d reads=%d clients=%d", logins.Load(), reads.Load(), len(monitor.pbsClients))
	}
	assertPBSConnectionProjection(t, monitor, instance.Name, false, "offline")
	project("cancelled")

	releaseOnce.Do(func() { close(release) })
	laterCtx, laterCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer laterCancel()
	monitor.retryFailedConnections(laterCtx)
	if logins.Load() != 3 || len(newPBSPollProvider().ListInstances(monitor)) != 1 {
		t.Fatal("later admitted retry did not retain its accepted session exactly once")
	}
	assertPBSConnectionProjection(t, monitor, instance.Name, false, "offline")
	project("awaiting-poll")
	task, err := newPBSPollProvider().BuildPollTask(monitor, instance.Name)
	if err != nil {
		t.Fatal(err)
	}
	newRealExecutor(monitor).Execute(laterCtx, task)
	assertPBSConnectionProjection(t, monitor, instance.Name, true, "online")
	actual := pbsInstanceByName(t, monitor.state.GetSnapshot(), instance.Name)
	if actual.CPU != 0.15 || actual.Memory != 50 || len(actual.Datastores) != 1 || actual.Datastores[0].Usage != 25 || logins.Load() != 3 || !reflect.DeepEqual(instance, monitor.config.PBSInstances[0]) {
		t.Fatalf("ordinary first poll lost readings, identity or saved trust: %+v logins=%d", actual, logins.Load())
	}
	project("recovered")
}
