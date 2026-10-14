package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Use saved configuration, the production endpoint builder and real TLS
// clients. Carry failed trust into the same member fields served to Settings.
// A subsequent normal config reload has independently reconciled fixture
// trust; this does not edit a live client's trust, probe guests or prove native
// deployment/recovery, and says nothing about any reported incident's cause.
func TestClusterConflictingPinsEndpointProjection(t *testing.T) {
	for _, mode := range []string{"token", "password-json", "password-form"} {
		t.Run(mode, func(t *testing.T) {
			var badRequests atomic.Int32
			endpoint := func(bad bool) *httptest.Server {
				s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if bad {
						badRequests.Add(1)
					}
					switch r.URL.Path {
					case "/api2/json/access/ticket":
						if mode == "password-form" && r.Header.Get("Content-Type") == "application/json" {
							w.WriteHeader(http.StatusUnsupportedMediaType)
							fmt.Fprint(w, `{"message":"fixture requires form encoding"}`)
							return
						}
						fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
					case "/api2/json/nodes":
						if mode == "token" {
							if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
								t.Error("lost token fixture auth")
							}
						} else if r.Header.Get("Cookie") != "PVEAuthCookie=fixture-ticket" {
							t.Error("lost fixture session")
						}
						fmt.Fprint(w, `{"data":[{"node":"primary","status":"online","cpu":0.125},{"node":"member","status":"online","cpu":0.25},{"node":"member-alias","status":"online","cpu":0.25}]}`)
					default:
						t.Errorf("left ordinary login/nodes path: %s", r.URL.Path)
						http.NotFound(w, r)
					}
				}))
				t.Cleanup(s.Close)
				return s
			}
			primary, member := endpoint(false), endpoint(true)
			pin := func(s *httptest.Server) string {
				sum := sha256.Sum256(s.Certificate().Raw)
				return hex.EncodeToString(sum[:])
			}
			inst := config.PVEInstance{Name: "pin-conflict-cluster", Host: primary.URL, IsCluster: true, ClusterName: "Fixture cluster", VerifySSL: true, Fingerprint: pin(primary), ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "primary", NodeIdentity: "primary-identity", Host: primary.URL, Fingerprint: pin(primary)},
				{NodeName: "member", NodeIdentity: "member-identity", Host: member.URL, Fingerprint: pin(member)},
				{NodeName: "member-alias", NodeIdentity: "alias-identity", Host: member.URL + "/./", Fingerprint: strings.Repeat("0", 64)},
			}}
			m := &Monitor{config: &config.Config{PVEInstances: []config.PVEInstance{inst}}}
			construct := func() *proxmox.ClusterClient {
				endpoints, pins := m.buildClusterEndpointsForInit(inst)
				if len(endpoints) != 3 || len(pins) != 3 {
					t.Fatalf("saved alias evidence was collapsed: endpoints=%d pins=%d", len(endpoints), len(pins))
				}
				cfg := proxmox.ClientConfig{Host: inst.Host, Fingerprint: inst.Fingerprint, VerifySSL: true, Timeout: 2 * time.Second}
				if mode == "token" {
					cfg.TokenName = "fixture@pve!pulse"
					cfg.TokenValue = "fixture"
				} else {
					cfg.User = "fixture@pam"
					cfg.Password = "fixture-password"
				}
				return proxmox.NewClusterClient(inst.Name, cfg, endpoints, pins)
			}
			project := func(cc *proxmox.ClusterClient, phase string, conflict bool) {
				nodes, err := cc.GetNodes(context.Background())
				if err != nil || len(nodes) != 3 {
					t.Fatalf("trusted primary lost ordinary monitoring: %v", err)
				}
				model := make([]models.Node, len(nodes))
				for i, node := range nodes {
					model[i] = models.Node{Name: node.Node, Status: node.Status}
				}
				m.updateClusterEndpointStatus(inst.Name, &inst, cc, model)
				for i, ep := range inst.ClusterEndpoints {
					if !ep.Online || ep.NodeIdentity != []string{"primary-identity", "member-identity", "alias-identity"}[i] {
						t.Fatal("trust refusal changed native online state or identity")
					}
					if ep.PulseReachable == nil || *ep.PulseReachable != (i == 0 || !conflict) || ep.LastPulseCheck == nil {
						t.Fatalf("missing/false Pulse reachability observation: %+v", ep)
					}
					if i > 0 && conflict && !strings.Contains(ep.PulseError, "TLS trust conflict") {
						t.Fatalf("member lost actionable trust diagnostic: %+v", ep)
					}
					if !conflict && ep.PulseError != "" {
						t.Fatalf("agreed trust kept a stale failure: %+v", ep)
					}
				}
				wire, err := json.Marshal(struct {
					Case, Phase string
					Endpoints   []config.ClusterEndpoint
				}{mode, phase, inst.ClusterEndpoints})
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("PIN_CONFLICT_ENDPOINT_PROJECTION_JSON=%s", wire)
			}
			refused := construct()
			project(refused, "conflicting-saved-trust", true)
			if badRequests.Load() != 0 || inst.ClusterEndpoints[2].Fingerprint != strings.Repeat("0", 64) {
				t.Fatal("conflict reached credentials or rewrote saved trust")
			}
			// Normal source configuration reload, not mutation of the owned map or a
			// forced/favourable repeat on the refused source. Native state stays online.
			inst.ClusterEndpoints[2].Fingerprint = pin(member)
			accepted := construct()
			project(accepted, "reconciled-config-reload", false)
			if badRequests.Load() == 0 {
				t.Fatal("agreed saved trust never reached accepted ordinary use")
			}
		})
	}
}
