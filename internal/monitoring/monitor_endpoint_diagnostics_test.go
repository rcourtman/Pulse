package monitoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func endpointDiagnosticMonitor() *Monitor {
	return &Monitor{
		state:         models.NewState(),
		pbsClients:    make(map[string]*pbs.Client),
		pmgClients:    make(map[string]*pmg.Client),
		pollStatusMap: make(map[string]*pollStatus),
	}
}

// No workers, database, listener or appliance: token construction performs no
// HTTP request. The config factories validate before removing anything, so
// userinfo, a query or a fragment is refused like malformed syntax rather
// than stripped into a usable credential destination. Every case here is a
// refusal; the saved configuration never changes and no input is logged.
// Non-parallel because these tests temporarily replace the global logger.
func TestMonitorClientInitEndpointDiagnostics(t *testing.T) {
	for index, host := range []string{
		"http://synthetic-user:synthetic-secret@gateway.invalid",
		"https://synthetic-user:synthetic-secret@gateway.invalid",
		"synthetic-user:synthetic-secret@gateway.invalid",
		"http://gateway.invalid?token=synthetic-secret",
		"https://gateway.invalid#synthetic-secret",
		"gateway.invalid?token=synthetic-secret",
		"http://synthetic-user:synthetic-secret@gateway.invalid:bad-port",
		"https://gateway.invalid/%invalid?token=synthetic-secret",
	} {
		t.Run(fmt.Sprintf("case-%d", index), func(t *testing.T) {
			cfg := &config.Config{
				PVEInstances: []config.PVEInstance{{Name: "pve-fixture", Host: host, TokenName: "fixture@pam!pulse", TokenValue: "synthetic-token"}},
				PBSInstances: []config.PBSInstance{{Name: "pbs-fixture", Host: host, TokenName: "fixture@pbs!pulse", TokenValue: "synthetic-token"}},
				PMGInstances: []config.PMGInstance{{Name: "pmg-fixture", Host: host, TokenName: "fixture@pmg!pulse", TokenValue: "synthetic-token"}},
			}
			before, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			m := endpointDiagnosticMonitor()
			var captured bytes.Buffer
			prior := log.Logger
			log.Logger = zerolog.New(&captured).Level(zerolog.DebugLevel)
			defer func() { log.Logger = prior }()
			m.initPVEClients(cfg)
			m.initPBSClients(cfg)
			m.initPMGClients(cfg)
			after, err := json.Marshal(cfg)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("display filtering changed saved configuration")
			}
			const wantRefusal = true
			const wantClients = 0
			if len(m.pveClients)+len(m.pbsClients)+len(m.pmgClients) != wantClients {
				t.Fatal("display filtering changed the existing configuration/constructor admission")
			}
			for _, nodeType := range []InstanceType{InstanceTypePVE, InstanceTypePBS, InstanceTypePMG} {
				name := string(nodeType) + "-fixture"
				key := m.connectionHealthStateKey(nodeType, name)
				healthy, found := m.state.GetConnectionHealth()[key]
				if nodeType == InstanceTypePBS && !wantRefusal {
					if found {
						t.Fatal("PBS client construction must still await its first poll")
					}
				} else if !found || healthy == wantRefusal {
					t.Fatal("display filtering changed existing connection health")
				}
			}
			for _, marker := range []string{"synthetic-secret", "synthetic-user", "synthetic-token"} {
				if strings.Contains(captured.String(), marker) {
					t.Error("client-init diagnostic disclosed endpoint or authentication input")
				}
			}
			decoder := json.NewDecoder(&captured)
			configurationEvents := 0
			refusalEvents := 0
			for decoder.More() {
				var event map[string]interface{}
				if err := decoder.Decode(&event); err != nil {
					t.Fatal(err)
				}
				message, _ := event["message"].(string)
				if strings.HasPrefix(message, "Configuring ") {
					configurationEvents++
					if event["host"] != "[invalid endpoint]" {
						t.Error("saved host did not use a safe display-only label")
					}
				}
				if strings.HasPrefix(message, "Failed to create ") {
					refusalEvents++
					if event["host"] != "[invalid endpoint]" {
						t.Error("refused host did not use a safe display-only label")
					}
				}
			}
			wantRefusalEvents := 0
			if wantRefusal {
				wantRefusalEvents = 3
			}
			if configurationEvents != 3 || refusalEvents != wantRefusalEvents {
				t.Error("lost existing configuring/refusal diagnostics for a provider")
			}
			t.Logf("configuration_events=%d refusal_events=%d registered_clients=%d", configurationEvents, refusalEvents, wantClients)
		})
	}
}

func TestMonitorClientInitAcceptedEndpointDiagnostics(t *testing.T) {
	const host = "https://gateway.invalid:8443/tenant"
	cfg := &config.Config{
		PVEInstances: []config.PVEInstance{{Name: "pve-fixture", Host: host, TokenName: "fixture@pam!pulse", TokenValue: "synthetic-token", VerifySSL: true}},
		PBSInstances: []config.PBSInstance{{Name: "pbs-fixture", Host: host, TokenName: "fixture@pbs!pulse", TokenValue: "synthetic-token", VerifySSL: true}},
		PMGInstances: []config.PMGInstance{{Name: "pmg-fixture", Host: host, TokenName: "fixture@pmg!pulse", TokenValue: "synthetic-token", VerifySSL: true}},
	}
	m := endpointDiagnosticMonitor()
	var captured bytes.Buffer
	prior := log.Logger
	log.Logger = zerolog.New(&captured).Level(zerolog.DebugLevel)
	defer func() { log.Logger = prior }()
	m.initPVEClients(cfg)
	m.initPBSClients(cfg)
	m.initPMGClients(cfg)
	if len(m.pveClients) != 1 || len(m.pbsClients) != 1 || len(m.pmgClients) != 1 {
		t.Fatal("accepted token clients were not retained")
	}
	// PBS construction still awaits its first poll, rather than inventing an
	// online reading. Other providers retain their existing construction state.
	wantHealth := map[string]bool{
		m.connectionHealthStateKey(InstanceTypePVE, "pve-fixture"): true,
		m.connectionHealthStateKey(InstanceTypePMG, "pmg-fixture"): true,
	}
	if !reflect.DeepEqual(m.state.GetConnectionHealth(), wantHealth) {
		t.Fatal("diagnostic filtering changed accepted-client health publication")
	}
	if strings.Contains(captured.String(), "synthetic-token") {
		t.Fatal("accepted-client diagnostic disclosed authentication input")
	}
	decoder := json.NewDecoder(&captured)
	hosts := 0
	for decoder.More() {
		var event map[string]interface{}
		if err := decoder.Decode(&event); err != nil {
			t.Fatal(err)
		}
		if value, found := event["host"]; found {
			hosts++
			if value != host {
				t.Fatal("accepted host lost its useful endpoint diagnostic")
			}
		}
	}
	if hosts != 3 {
		t.Fatal("accepted-provider configuring diagnostics are missing")
	}
}
