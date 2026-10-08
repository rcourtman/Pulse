package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
)

func TestPollPMGInstancePopulatesState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/access/ticket":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"ticket":"ticket123","CSRFPreventionToken":"csrf123"}}`)

		case "/api2/json/version":
			if !strings.Contains(r.Header.Get("Cookie"), "PMGAuthCookie=ticket123") {
				t.Fatalf("expected auth cookie, got %s", r.Header.Get("Cookie"))
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"version":"8.3.1","release":"1"}}`)

		case "/api2/json/config/cluster/status":
			if r.URL.Query().Get("list_single_node") != "1" {
				t.Fatalf("expected list_single_node query, got %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"cid":1,"name":"mail-gateway","type":"master","ip":"10.0.0.1"}]}`)

		case "/api2/json/statistics/mail":
			// PMG API does not accept timeframe parameter
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"count":100,"count_in":60,"count_out":40,"spamcount_in":5,"spamcount_out":2,"viruscount_in":1,"viruscount_out":0,"bounces_in":3,"bounces_out":1,"bytes_in":12345,"bytes_out":54321,"glcount":7,"junk_in":4,"avptime":0.5,"rbl_rejects":2,"pregreet_rejects":1}}`)

		case "/api2/json/statistics/mailcount":
			if r.URL.Query().Get("timespan") != "86400" {
				t.Fatalf("expected timespan=86400 (24 hours in seconds), got %s", r.URL.RawQuery)
			}
			now := time.Now().Unix()
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"data":[{"index":0,"time":%d,"count":100,"count_in":60,"count_out":40,"spamcount_in":5,"spamcount_out":2,"viruscount_in":1,"viruscount_out":0,"bounces_in":3,"bounces_out":1,"rbl_rejects":2,"pregreet_rejects":1,"glcount":7}]}`, now)

		case "/api2/json/statistics/spamscores":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"level":"low","count":10,"ratio":0.1}]}`)

		case "/api2/json/quarantine/spamstatus":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"count":5,"avgbytes":0,"mbytes":0}}`)

		case "/api2/json/quarantine/virusstatus":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"count":2,"avgbytes":0,"mbytes":0}}`)

		case "/api2/json/nodes/mail-gateway/postfix/queue":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"active":5,"deferred":2,"hold":0,"maildrop":0}}`)

		case "/api2/json/nodes/mail-gateway/backup":
			w.Header().Set("Content-Type", "application/json")
			timestamp := time.Now().Add(-8 * 24 * time.Hour).Unix()
			fmt.Fprintf(w, `{"data":[{"filename":"pmg-backup_2024-01-01.tgz","size":123456,"timestamp":%d}]}`, timestamp)

		case "/api2/json/config/domains":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"domain":"example.com","comment":"Primary relay"}]}`)

		case "/api2/json/statistics/domains":
			if r.URL.Query().Get("starttime") == "" || r.URL.Query().Get("endtime") == "" {
				t.Fatalf("expected starttime/endtime query, got %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"domain":"example.com","count":120,"spamcount":8,"viruscount":1,"bytes":1234567}]}`)

		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := pmg.NewClient(pmg.ClientConfig{
		Host:      server.URL,
		User:      "api@pmg",
		Password:  "secret",
		VerifySSL: false,
	})
	if err != nil {
		t.Fatalf("unexpected client error: %v", err)
	}

	cfg := &config.Config{
		PMGInstances: []config.PMGInstance{
			{
				Name:               "primary",
				Host:               server.URL,
				User:               "api@pmg",
				Password:           "secret",
				MonitorMailStats:   true,
				MonitorQueues:      true,
				MonitorQuarantine:  true,
				MonitorDomainStats: true,
			},
		},
	}

	mon, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}

	mon.pollPMGInstance(context.Background(), "primary", client)

	snapshot := mon.state.GetSnapshot()

	if len(snapshot.PMGInstances) != 1 {
		t.Fatalf("expected 1 PMG instance in state, got %d", len(snapshot.PMGInstances))
	}

	instance := snapshot.PMGInstances[0]

	if instance.Status != "online" {
		t.Fatalf("expected PMG status online, got %s", instance.Status)
	}

	if instance.ConnectionHealth != "healthy" {
		t.Fatalf("expected connection health healthy, got %s", instance.ConnectionHealth)
	}

	if instance.MailStats == nil || instance.MailStats.CountTotal != 100 {
		t.Fatalf("expected mail stats totals, got %+v", instance.MailStats)
	}

	if len(instance.MailCount) != 1 {
		t.Fatalf("expected 1 mail count point, got %d", len(instance.MailCount))
	}

	if len(instance.SpamDistribution) != 1 {
		t.Fatalf("expected 1 spam distribution bucket, got %d", len(instance.SpamDistribution))
	}

	if instance.Quarantine == nil || instance.Quarantine.Spam != 5 || instance.Quarantine.Virus != 2 {
		t.Fatalf("expected quarantine counts, got %+v", instance.Quarantine)
	}

	if health := snapshot.ConnectionHealth["pmg-primary"]; !health {
		t.Fatalf("expected connection health tracked as healthy, got %v", health)
	}

	if failures := mon.authFailures["pmg-primary"]; failures != 0 {
		t.Fatalf("expected no auth failures tracked, got %d", failures)
	}

	if len(snapshot.PMGBackups) != 1 {
		t.Fatalf("expected 1 PMG backup in state, got %d", len(snapshot.PMGBackups))
	}

	pmgBackup := snapshot.PMGBackups[0]
	if pmgBackup.Node != "mail-gateway" {
		t.Fatalf("expected PMG backup node mail-gateway, got %s", pmgBackup.Node)
	}

	if len(instance.RelayDomains) != 1 {
		t.Fatalf("expected 1 relay domain, got %d", len(instance.RelayDomains))
	}
	if instance.RelayDomains[0].Domain != "example.com" {
		t.Fatalf("expected relay domain example.com, got %q", instance.RelayDomains[0].Domain)
	}

	if len(instance.DomainStats) != 1 {
		t.Fatalf("expected 1 domain stat row, got %d", len(instance.DomainStats))
	}
	if instance.DomainStats[0].Domain != "example.com" || instance.DomainStats[0].SpamCount != 8 {
		t.Fatalf("unexpected domain stats: %+v", instance.DomainStats[0])
	}
}

func TestPollPMGInstanceRecordsAuthFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/version":
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, "unauthorized")

		case "/api2/json/access/ticket":
			t.Fatalf("token client should not request auth ticket")

		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := pmg.NewClient(pmg.ClientConfig{
		Host:       server.URL,
		User:       "apitest@pmg",
		TokenName:  "apitoken",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating token client: %v", err)
	}

	cfg := &config.Config{
		PMGInstances: []config.PMGInstance{
			{
				Name:       "failing",
				Host:       server.URL,
				User:       "apitest@pmg",
				TokenName:  "apitoken",
				TokenValue: "secret",
			},
		},
	}

	mon, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}

	mon.pollPMGInstance(context.Background(), "failing", client)

	snapshot := mon.state.GetSnapshot()

	if health := snapshot.ConnectionHealth["pmg-failing"]; health {
		t.Fatalf("expected unhealthy connection, got %v", health)
	}

	if len(snapshot.PMGInstances) != 1 {
		t.Fatalf("expected failed PMG instance in state, got %d", len(snapshot.PMGInstances))
	}

	instance := snapshot.PMGInstances[0]
	if instance.Status != "offline" {
		t.Fatalf("expected offline status, got %s", instance.Status)
	}

	if instance.ConnectionHealth != "unhealthy" {
		t.Fatalf("expected unhealthy connection status, got %s", instance.ConnectionHealth)
	}

	if failures := mon.authFailures["pmg-failing"]; failures != 1 {
		t.Fatalf("expected one auth failure tracked, got %d", failures)
	}

	// Regression: pollStatusMap must record the failure. A defer-arg bug
	// previously captured pollErr at register-time (always nil), so failed
	// polls were recorded as success and the connections aggregator reported
	// broken instances as healthy.
	status := mon.pollStatusMap["pmg::failing"]
	if status == nil {
		t.Fatal("expected pollStatusMap entry for pmg::failing, got nil")
	}
	if !status.LastSuccess.IsZero() {
		t.Errorf("expected LastSuccess to remain zero on auth failure, got %v", status.LastSuccess)
	}
	if status.ConsecutiveFailures == 0 {
		t.Error("expected ConsecutiveFailures > 0 after auth failure, got 0")
	}
}

// Uses the real poll consumer with a typed, read-only provider boundary. No
// listener, mail, credentials, external services or native appliance is used.
type pmgScopeFixture struct {
	calls     map[string]int
	fail      string
	onVersion func()
}

func (f *pmgScopeFixture) read(key, data string, out any) error {
	f.calls[key]++
	if f.fail == key {
		return errors.New("fixture collection refused")
	}
	return json.Unmarshal([]byte(data), out)
}
func (f *pmgScopeFixture) GetVersion(context.Context) (*pmg.VersionInfo, error) {
	var v pmg.VersionInfo
	err := f.read("version", `{"version":"8.3"}`, &v)
	if f.onVersion != nil {
		f.onVersion()
	}
	return &v, err
}
func (f *pmgScopeFixture) GetClusterStatus(_ context.Context, single bool) ([]pmg.ClusterStatusEntry, error) {
	if !single {
		return nil, errors.New("single-node query omitted")
	}
	var v []pmg.ClusterStatusEntry
	err := f.read("cluster", `[{"name":"one","type":"master"},{"name":"two","type":"slave"}]`, &v)
	return v, err
}
func (f *pmgScopeFixture) GetQueueStatus(_ context.Context, node string) (*pmg.QueueStatusEntry, error) {
	var v pmg.QueueStatusEntry
	err := f.read("queue/"+node, `{"active":20,"deferred":10,"hold":5,"incoming":2,"oldest_age":7200}`, &v)
	return &v, err
}
func (f *pmgScopeFixture) ListBackups(_ context.Context, node string) ([]pmg.BackupEntry, error) {
	var v []pmg.BackupEntry
	err := f.read("backup/"+node, `[{"filename":"fixture.tgz","timestamp":1700000000,"size":123}]`, &v)
	return v, err
}
func (f *pmgScopeFixture) GetMailStatistics(_ context.Context, frame string) (*pmg.MailStatistics, error) {
	if frame != "" {
		return nil, errors.New("unexpected timeframe")
	}
	var v pmg.MailStatistics
	err := f.read("mail", `{"count":123,"avptime":0.5}`, &v)
	return &v, err
}
func (f *pmgScopeFixture) GetMailCount(_ context.Context, span int) ([]pmg.MailCountEntry, error) {
	if span != 86400 {
		return nil, errors.New("wrong collection window")
	}
	var v []pmg.MailCountEntry
	err := f.read("count", `[{"time":1700000000,"count":123}]`, &v)
	return v, err
}
func (f *pmgScopeFixture) GetSpamScores(context.Context) ([]pmg.SpamScore, error) {
	var v []pmg.SpamScore
	err := f.read("scores", `[{"level":"low","count":4}]`, &v)
	return v, err
}
func (f *pmgScopeFixture) GetQuarantineStatus(_ context.Context, category string) (*pmg.QuarantineStatus, error) {
	var v pmg.QuarantineStatus
	err := f.read("quarantine/"+category, `{"count":200}`, &v)
	return &v, err
}
func (f *pmgScopeFixture) ListRelayDomains(context.Context) ([]pmg.RelayDomainEntry, error) {
	var v []pmg.RelayDomainEntry
	err := f.read("domains", `[{"domain":"example.invalid"}]`, &v)
	return v, err
}
func (f *pmgScopeFixture) GetDomainStatistics(_ context.Context, start, end int64) ([]pmg.DomainStatisticsEntry, error) {
	if end-start != 86400 {
		return nil, errors.New("wrong domain window")
	}
	var v []pmg.DomainStatisticsEntry
	err := f.read("domain-stats", `[{"domain":"example.invalid","count":123}]`, &v)
	return v, err
}
func pmgScopeMonitor(cfg config.PMGInstance) *Monitor {
	cfg.Name = "gateway"
	return &Monitor{
		config: &config.Config{PMGInstances: []config.PMGInstance{cfg}},
		state:  models.NewState(), authFailures: map[string]int{}, lastAuthAttempt: map[string]time.Time{},
		pollStatusMap: map[string]*pollStatus{}, circuitBreakers: map[string]*circuitBreaker{},
	}
}
func allPMGScope() config.PMGInstance {
	return config.PMGInstance{MonitoringConfigured: true, MonitorMailStats: true, MonitorQueues: true, MonitorQuarantine: true, MonitorDomainStats: true}
}
func TestPMGCollectionScope(t *testing.T) {
	for mask := 0; mask < 16; mask++ {
		t.Run(fmt.Sprintf("scope-%04b", mask), func(t *testing.T) {
			cfg := config.PMGInstance{MonitoringConfigured: true, MonitorMailStats: mask&1 != 0, MonitorQueues: mask&2 != 0, MonitorQuarantine: mask&4 != 0, MonitorDomainStats: mask&8 != 0}
			m := pmgScopeMonitor(cfg)
			f := &pmgScopeFixture{calls: map[string]int{}}
			m.pollPMGInstance(context.Background(), "gateway", f)
			want := map[string]int{"version": 1, "cluster": 1, "backup/one": 1, "backup/two": 1}
			if cfg.MonitorMailStats {
				want["mail"] = 1
				want["count"] = 1
				want["scores"] = 1
			}
			if cfg.MonitorQueues {
				want["queue/one"] = 1
				want["queue/two"] = 1
			}
			if cfg.MonitorQuarantine {
				want["quarantine/spam"] = 1
				want["quarantine/virus"] = 1
			}
			if cfg.MonitorDomainStats {
				want["domains"] = 1
				want["domain-stats"] = 1
			}
			if !reflect.DeepEqual(f.calls, want) {
				t.Fatalf("requests=%v want=%v", f.calls, want)
			}
			v := m.state.GetSnapshot().PMGInstances[0]
			if (v.MailStats != nil) != cfg.MonitorMailStats || (len(v.MailCount) > 0) != cfg.MonitorMailStats || (len(v.SpamDistribution) > 0) != cfg.MonitorMailStats {
				t.Fatalf("incorrect mail data: %+v", v)
			}
			if v.MailStats != nil && (v.MailStats.CountTotal != 123 || v.MailStats.AverageProcessTimeMs != 500) {
				t.Fatal("selected statistics lost values/units")
			}
			if (v.Quarantine != nil) != cfg.MonitorQuarantine {
				t.Fatalf("incorrect quarantine: %+v", v.Quarantine)
			}
			for _, n := range v.Nodes {
				if (n.QueueStatus != nil) != cfg.MonitorQueues {
					t.Fatalf("incorrect queue: %+v", n)
				}
				if n.QueueStatus != nil && n.QueueStatus.Total != 37 {
					t.Fatal("wrong queue total")
				}
			}
			if (len(v.DomainStats) > 0) != cfg.MonitorDomainStats || (len(v.RelayDomains) > 0) != cfg.MonitorDomainStats {
				t.Fatal("incorrect domain collection")
			}
			if v.Status != "online" || v.ConnectionHealth != "healthy" || m.pollStatusMap["pmg::gateway"].ConsecutiveFailures != 0 {
				t.Fatal("selected collection not recorded as successful")
			}
		})
	}
}
func TestPMGCollectionLegacyDefault(t *testing.T) {
	m := pmgScopeMonitor(config.PMGInstance{})
	f := &pmgScopeFixture{calls: map[string]int{}}
	m.pollPMGInstance(context.Background(), "gateway", f)
	want := map[string]int{"version": 1, "cluster": 1, "backup/one": 1, "backup/two": 1, "mail": 1, "count": 1, "scores": 1}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("legacy requests=%v want=%v", f.calls, want)
	}
}
func TestPMGCollectionEditsPauseAndResume(t *testing.T) {
	m := pmgScopeMonitor(allPMGScope())
	f := &pmgScopeFixture{calls: map[string]int{}}
	m.pollPMGInstance(context.Background(), "gateway", f)
	config.Mu.Lock()
	m.config.PMGInstances[0] = config.PMGInstance{Name: "gateway", MonitoringConfigured: true}
	config.Mu.Unlock()
	f.calls = map[string]int{}
	m.pollPMGInstance(context.Background(), "gateway", f)
	if len(f.calls) != 4 {
		t.Fatalf("all-off still read datasets: %v", f.calls)
	}
	v := m.state.GetSnapshot().PMGInstances[0]
	if v.MailStats != nil || v.Quarantine != nil || v.Nodes[0].QueueStatus != nil || len(v.DomainStats) != 0 {
		t.Fatal("old dataset exposed as current after disable")
	}
	config.Mu.Lock()
	m.config.PMGInstances[0].Disabled = true
	config.Mu.Unlock()
	f.calls = map[string]int{}
	m.pollPMGInstance(context.Background(), "gateway", f)
	if len(f.calls) != 0 {
		t.Fatalf("paused requests=%v", f.calls)
	}
	if !reflect.DeepEqual(m.state.GetSnapshot().PMGInstances[0], v) {
		t.Fatal("pause overwrote last readings")
	}
	config.Mu.Lock()
	m.config.PMGInstances[0].Disabled = false
	m.config.PMGInstances[0].MonitorQueues = true
	config.Mu.Unlock()
	m.pollPMGInstance(context.Background(), "gateway", f)
	if f.calls["queue/one"] != 1 || f.calls["mail"] != 0 {
		t.Fatalf("resume scope=%v", f.calls)
	}
}
func TestPMGCollectionInFlightSnapshot(t *testing.T) {
	m := pmgScopeMonitor(allPMGScope())
	f := &pmgScopeFixture{calls: map[string]int{}}
	reached := make(chan struct{})
	resume := make(chan struct{})
	done := make(chan struct{})
	f.onVersion = func() { close(reached); <-resume }
	go func() { defer close(done); m.pollPMGInstance(context.Background(), "gateway", f) }()
	<-reached
	config.Mu.Lock()
	m.config.PMGInstances[0] = config.PMGInstance{Name: "gateway", MonitoringConfigured: true, Disabled: true}
	config.Mu.Unlock()
	close(resume)
	<-done
	if f.calls["mail"] != 1 || f.calls["queue/one"] != 1 || f.calls["domain-stats"] != 1 || f.calls["quarantine/spam"] != 1 {
		t.Fatal("in-flight snapshot changed during poll")
	}
	f.onVersion = nil
	f.calls = map[string]int{}
	m.pollPMGInstance(context.Background(), "gateway", f)
	if len(f.calls) != 0 {
		t.Fatalf("next paused poll requests=%v", f.calls)
	}
}
func TestPMGCollectionFailureEvidence(t *testing.T) {
	for _, op := range []string{"version", "cluster", "backup/one", "backup/two", "mail", "count", "scores", "queue/one", "queue/two", "quarantine/spam", "quarantine/virus", "domains", "domain-stats"} {
		t.Run(op, func(t *testing.T) {
			m := pmgScopeMonitor(allPMGScope())
			f := &pmgScopeFixture{calls: map[string]int{}}
			m.pollPMGInstance(context.Background(), "gateway", f)
			before := m.pollStatusMap["pmg::gateway"].LastSuccess
			f.fail = op
			m.pollPMGInstance(context.Background(), "gateway", f)
			status := m.pollStatusMap["pmg::gateway"]
			if status.ConsecutiveFailures != 1 || status.LastErrorMessage == "" || !status.LastSuccess.Equal(before) {
				t.Fatalf("failed read recorded as complete: %+v", status)
			}
			v := m.state.GetSnapshot().PMGInstances[0]
			if op == "version" {
				if v.Status != "offline" || v.ConnectionHealth != "unhealthy" {
					t.Fatal("failed connection not offline")
				}
				return
			}
			if v.Status != "online" || v.ConnectionHealth != "healthy" || !m.state.GetSnapshot().ConnectionHealth["pmg-gateway"] {
				t.Fatal("downstream failure mislabeled as version connection failure")
			}
			if (op == "quarantine/spam" || op == "quarantine/virus") && v.Quarantine != nil {
				t.Fatal("partial quarantine reported as zero/complete")
			}
			if op == "mail" && v.MailStats != nil {
				t.Fatal("failed stats published")
			}
			if op == "queue/one" && v.Nodes[0].QueueStatus != nil {
				t.Fatal("failed queue published")
			}
			f.fail = ""
			m.pollPMGInstance(context.Background(), "gateway", f)
			if status.ConsecutiveFailures != 0 || status.LastErrorMessage != "" {
				t.Fatal("next complete poll did not recover")
			}
		})
	}
}
func TestPMGCollectionCancelledAndUnknown(t *testing.T) {
	m := pmgScopeMonitor(allPMGScope())
	f := &pmgScopeFixture{calls: map[string]int{}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.pollPMGInstance(ctx, "gateway", f)
	m.pollPMGInstance(context.Background(), "unknown", f)
	if len(f.calls) != 0 {
		t.Fatalf("cancelled/unknown requests=%v", f.calls)
	}
}
