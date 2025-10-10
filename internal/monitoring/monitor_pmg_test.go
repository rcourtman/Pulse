package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
			if r.URL.Query().Get("timeframe") != "day" {
				t.Fatalf("expected timeframe=day, got %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"count":100,"count_in":60,"count_out":40,"spamcount_in":5,"spamcount_out":2,"viruscount_in":1,"viruscount_out":0,"bounces_in":3,"bounces_out":1,"bytes_in":12345,"bytes_out":54321,"glcount":7,"junk_in":4,"avptime":0.5,"rbl_rejects":2,"pregreet_rejects":1}}`)

		case "/api2/json/statistics/mailcount":
			if r.URL.Query().Get("timespan") != "24" {
				t.Fatalf("expected timespan=24, got %s", r.URL.RawQuery)
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

	mon := &Monitor{
		config: &config.Config{
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
		},
		state:            models.NewState(),
		authFailures:     make(map[string]int),
		lastAuthAttempt:  make(map[string]time.Time),
		pbsBackupPollers: make(map[string]bool),
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

	mon := &Monitor{
		config: &config.Config{
			PMGInstances: []config.PMGInstance{
				{
					Name:       "failing",
					Host:       server.URL,
					User:       "apitest@pmg",
					TokenName:  "apitoken",
					TokenValue: "secret",
				},
			},
		},
		state:            models.NewState(),
		authFailures:     make(map[string]int),
		lastAuthAttempt:  make(map[string]time.Time),
		pbsBackupPollers: make(map[string]bool),
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
}
