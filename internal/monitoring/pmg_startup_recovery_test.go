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
	"testing/synctest"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
)

func newPMGStartupTestMonitor(instances []config.PMGInstance) *Monitor {
	return &Monitor{
		config:           &config.Config{PMGInstances: instances},
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
	}
}

func assertPMGStartupProjection(t *testing.T, m *Monitor, name string, wantHealthy bool, wantStatus string) {
	t.Helper()
	snap := m.state.GetSnapshot()
	if got := snap.ConnectionHealth["pmg-"+name]; got != wantHealthy {
		t.Fatalf("ConnectionHealth[pmg-%s] = %v, want %v", name, got, wantHealthy)
	}
	var found *models.PMGInstance
	for i := range snap.PMGInstances {
		if snap.PMGInstances[i].Name == name {
			found = &snap.PMGInstances[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("PMG instance %q missing from state snapshot", name)
	}
	if found.Status != wantStatus {
		t.Fatalf("PMG instance %q status = %q, want %q", name, found.Status, wantStatus)
	}
}

func TestPMGConnectionRetrySurvivesLongStartupOutage(t *testing.T) {
	original := newPMGClientFunc
	defer func() { newPMGClientFunc = original }()

	synctest.Test(t, func(t *testing.T) {
		instance := config.PMGInstance{
			Name:        "late-pmg",
			Host:        "https://pmg.local:8006",
			User:        "fixture@pmg",
			Password:    "fixture-password",
			VerifySSL:   true,
			Fingerprint: "fixture-saved-pin",
		}
		m := newPMGStartupTestMonitor([]config.PMGInstance{instance})
		started := time.Now()
		var attempts []time.Duration

		newPMGClientFunc = func(_ context.Context, cfg pmg.ClientConfig) (*pmg.Client, error) {
			want := config.CreatePMGConfig(&instance)
			if want.Timeout <= 0 {
				want.Timeout = 45 * time.Second
			}
			if !reflect.DeepEqual(cfg, want) {
				t.Fatal("PMG retry changed configured authority, credentials, TLS pin or timeout")
			}
			elapsed := time.Since(started)
			attempts = append(attempts, elapsed)
			if elapsed < 6*time.Minute {
				return nil, fmt.Errorf("fixture PMG still starting")
			}
			return &pmg.Client{}, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		m.retryFailedConnections(ctx)

		client, exists := m.getPMGClient(instance.Name)
		if !exists || client == nil || len(newPMGPollProvider().ListInstances(m)) != 1 {
			t.Fatalf("six-minute startup outage left PMG outside ordinary polling; attempts=%v", attempts)
		}
		want := []time.Duration{
			5 * time.Second,
			15 * time.Second,
			35 * time.Second,
			75 * time.Second,
			135 * time.Second,
			195 * time.Second,
			255 * time.Second,
			315 * time.Second,
			375 * time.Second,
		}
		if !reflect.DeepEqual(attempts, want) {
			t.Fatalf("PMG retry cadence=%v, want capped schedule=%v", attempts, want)
		}
		// Recreation alone must not claim healthy before the first poll runs.
		assertPMGStartupProjection(t, m, instance.Name, false, "offline")
	})
}

func TestPMGConnectionRetrySkipsPausedConnections(t *testing.T) {
	original := newPMGClientFunc
	defer func() { newPMGClientFunc = original }()

	synctest.Test(t, func(t *testing.T) {
		m := newPMGStartupTestMonitor([]config.PMGInstance{
			{Name: "paused-pmg", Host: "://invalid", Disabled: true},
		})
		newPMGClientFunc = func(context.Context, pmg.ClientConfig) (*pmg.Client, error) {
			t.Fatal("retry attempted a paused PMG connection")
			return nil, nil
		}
		m.initPMGClients(m.config)
		m.retryFailedConnections(context.Background())

		if len(m.pmgClients) != 0 || len(m.state.GetSnapshot().PMGInstances) != 0 || len(m.state.GetSnapshot().ConnectionHealth) != 0 {
			t.Fatal("paused PMG connection was recreated or published to state")
		}
	})
}

func TestPMGStartupRecoveryKeepsTrustAndFirstPollBoundary(t *testing.T) {
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
		if r.Method != http.MethodGet || !strings.Contains(r.Header.Get("Cookie"), "PMGAuthCookie=fixture-ticket") {
			t.Errorf("ordinary PMG read lost authentication or used a mutation: %s %s", r.Method, r.URL.Path)
			http.Error(w, "fixture refuses unauthenticated reads", http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/api2/json/version":
			fmt.Fprint(w, `{"data":{"version":"8.3.1","release":"1"}}`)
		case "/api2/json/config/cluster/status":
			fmt.Fprint(w, `{"data":[{"cid":1,"name":"mail-gw-01","type":"master","ip":"10.0.0.10"}]}`)
		case "/api2/json/statistics/mail":
			fmt.Fprint(w, `{"data":{"count":100,"count_in":60,"count_out":40,"spamcount_in":5,"spamcount_out":1,"viruscount_in":1,"viruscount_out":0,"bounces_in":2,"bounces_out":0,"bytes_in":12345,"bytes_out":54321,"glcount":4,"junk_in":3,"avptime":0.4,"rbl_rejects":2,"pregreet_rejects":1}}`)
		case "/api2/json/statistics/mailcount":
			fmt.Fprintf(w, `{"data":[{"index":0,"time":%d,"count":100,"count_in":60,"count_out":40,"spamcount_in":5,"spamcount_out":1,"viruscount_in":1,"viruscount_out":0}]}`, time.Now().Unix())
		case "/api2/json/statistics/spamscores":
			fmt.Fprint(w, `{"data":[{"level":"low","count":10,"ratio":0.1}]}`)
		case "/api2/json/quarantine/spamstatus":
			fmt.Fprint(w, `{"data":{"count":5,"avgbytes":0,"mbytes":0}}`)
		case "/api2/json/quarantine/virusstatus":
			fmt.Fprint(w, `{"data":{"count":2,"avgbytes":0,"mbytes":0}}`)
		case "/api2/json/nodes/mail-gw-01/postfix/qshape":
			q := r.URL.Query().Get("queue")
			val := "0"
			if q == "active" {
				val = "5"
			} else if q == "deferred" {
				val = "2"
			}
			fmt.Fprintf(w, `{"data":[{"domain":"TOTAL","total":"%s","5m":"0","10m":"0","20m":"0","40m":"0","80m":"0","160m":"0","320m":"0","640m":"0","1280m":"0","1280m+":"0"}]}`, val)
		case "/api2/json/nodes/mail-gw-01/backup":
			fmt.Fprint(w, `{"data":[]}`)
		case "/api2/json/config/domains":
			fmt.Fprint(w, `{"data":[{"domain":"example.com","comment":"Primary relay"}]}`)
		case "/api2/json/statistics/domains":
			fmt.Fprint(w, `{"data":[{"domain":"example.com","count":100,"spamcount":5,"viruscount":1,"bytes":12345}]}`)
		default:
			fmt.Fprint(w, `{"data":[]}`)
		}
	}))
	defer server.Close()

	pin := sha256.Sum256(server.Certificate().Raw)
	instance := config.PMGInstance{
		Name:               "startup-pmg",
		Host:               server.URL,
		User:               "fixture@pmg",
		Password:           "fixture-password",
		VerifySSL:          true,
		Fingerprint:        hex.EncodeToString(pin[:]),
		MonitorMailStats:   true,
		MonitorQueues:      true,
		MonitorQuarantine:  true,
		MonitorDomainStats: true,
	}
	m := newPMGStartupTestMonitor([]config.PMGInstance{instance})
	project := func(phase string) {
		snapshot, scope := m.currentStateWithScope()
		frontend := m.buildBroadcastFrontendStateFromSnapshot(snapshot, scope)
		wire, err := json.Marshal(struct {
			Phase string               `json:"phase"`
			State models.StateFrontend `json:"state"`
		}{phase, frontend})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("PMG_STARTUP_PROJECTION_JSON=%s", wire)
	}

	// Phase 1: startup client creation fails -> truthfully offline, recorded in scheduler health, not in poll provider.
	m.initPMGClients(m.config)
	assertPMGStartupProjection(t, m, instance.Name, false, "offline")
	if len(newPMGPollProvider().ListInstances(m)) != 0 {
		t.Fatal("failed PMG constructor entered polling")
	}
	if status := m.pollStatusMap["pmg::startup-pmg"]; status == nil || status.ConsecutiveFailures != 1 {
		t.Fatalf("expected startup failure recorded in pollStatusMap, got %+v", status)
	}
	project("unavailable")

	// Phase 2: retryFailedConnections recovers client once PMG starts answering, staying offline until first poll.
	originalDelays := connRetryDelays
	connRetryDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	defer func() { connRetryDelays = originalDelays }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m.retryFailedConnections(ctx)

	client, exists := m.getPMGClient(instance.Name)
	if !exists || client == nil || logins.Load() != 3 || len(newPMGPollProvider().ListInstances(m)) != 1 {
		t.Fatalf("ordinary PMG startup recovery missing or duplicated: exists=%t logins=%d", exists, logins.Load())
	}
	assertPMGStartupProjection(t, m, instance.Name, false, "offline")
	project("awaitingPoll")

	// Phase 3: first scheduled poll succeeds -> online with native version, mail stats, quarantine, and queues.
	task, err := newPMGPollProvider().BuildPollTask(m, instance.Name)
	if err != nil {
		t.Fatal(err)
	}
	newRealExecutor(m).Execute(ctx, task)
	assertPMGStartupProjection(t, m, instance.Name, true, "online")

	snap := m.state.GetSnapshot()
	if len(snap.PMGInstances) != 1 {
		t.Fatalf("expected 1 PMG instance in state, got %d", len(snap.PMGInstances))
	}
	published := snap.PMGInstances[0]
	if published.Version != "8.3.1" || published.ConnectionHealth != "healthy" ||
		published.MailStats == nil || published.MailStats.CountIn != 60 ||
		published.Quarantine == nil || published.Quarantine.Spam != 5 || published.Quarantine.Virus != 2 ||
		len(published.Nodes) != 1 || published.Nodes[0].QueueStatus == nil ||
		published.Nodes[0].QueueStatus.Active != 5 || published.Nodes[0].QueueStatus.Deferred != 2 ||
		logins.Load() != 3 || !reflect.DeepEqual(instance, m.config.PMGInstances[0]) {
		t.Fatalf("recovery lost PMG readings/identity/trust or replayed login: %+v logins=%d", published, logins.Load())
	}
	if status := m.pollStatusMap["pmg::startup-pmg"]; status == nil || status.ConsecutiveFailures != 0 || status.LastSuccess.IsZero() {
		t.Fatalf("expected scheduler pollStatus cleared after first successful poll, got %+v", status)
	}
	project("recovered")

	// A different saved TLS pin remains a refusal and sends no password-bearing HTTP request.
	wrong := instance
	wrong.Name = "wrong-pin-pmg"
	wrong.Fingerprint = strings.Repeat("0", 64)
	untrusted := newPMGStartupTestMonitor([]config.PMGInstance{wrong})
	untrusted.initPMGClients(untrusted.config)
	refusedCtx, refusedCancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer refusedCancel()
	untrusted.retryFailedConnections(refusedCtx)
	if len(untrusted.pmgClients) != 0 || logins.Load() != 3 || untrusted.config.PMGInstances[0].Fingerprint != wrong.Fingerprint {
		t.Fatal("PMG retry bypassed or replaced a saved TLS pin")
	}
	assertPMGStartupProjection(t, untrusted, wrong.Name, false, "offline")
}

func TestPMGStartupCancellationHonoursContext(t *testing.T) {
	var logins atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	requestEnded := make(chan struct{})
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
		fmt.Fprint(w, `{"data":{"version":"8.3.1"}}`)
	}))
	defer server.Close()
	defer releaseOnce.Do(func() { close(release) })

	pin := sha256.Sum256(server.Certificate().Raw)
	instance := config.PMGInstance{
		Name:        "cancellable-pmg",
		Host:        server.URL,
		User:        "fixture@pmg",
		Password:    "fixture-password",
		VerifySSL:   true,
		Fingerprint: hex.EncodeToString(pin[:]),
	}
	monitor := newPMGStartupTestMonitor([]config.PMGInstance{instance})
	monitor.initPMGClients(monitor.config)
	assertPMGStartupProjection(t, monitor, instance.Name, false, "offline")

	originalDelays := connRetryDelays
	connRetryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	defer func() { connRetryDelays = originalDelays }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		monitor.retryFailedConnections(ctx)
		close(done)
	}()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		releaseOnce.Do(func() { close(release) })
		cancel()
		<-done
		t.Fatal("fixture did not reach PMG reconnect login request")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		releaseOnce.Do(func() { close(release) })
		<-done
		t.Fatal("cancelled PMG retry did not return promptly")
	}
	select {
	case <-requestEnded:
	case <-time.After(time.Second):
		t.Fatal("cancelled PMG retry did not cancel its HTTP request")
	}

	if logins.Load() != 2 || len(monitor.pmgClients) != 0 || len(newPMGPollProvider().ListInstances(monitor)) != 0 {
		t.Fatalf("cancelled PMG retry published a client: logins=%d clients=%d", logins.Load(), len(monitor.pmgClients))
	}
	assertPMGStartupProjection(t, monitor, instance.Name, false, "offline")

	// A later retry with an active context succeeds without affecting saved config.
	releaseOnce.Do(func() { close(release) })
	laterCtx, laterCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer laterCancel()
	monitor.retryFailedConnections(laterCtx)
	if logins.Load() != 3 || len(newPMGPollProvider().ListInstances(monitor)) != 1 {
		t.Fatalf("later PMG retry did not recover client: logins=%d", logins.Load())
	}
	assertPMGStartupProjection(t, monitor, instance.Name, false, "offline")
}
