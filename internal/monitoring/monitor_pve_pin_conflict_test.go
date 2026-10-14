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
		for _, layout := range []string{"url-aliases", "exact-duplicates"} {
			t.Run(layout+"/"+mode, func(t *testing.T) {
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
				if layout == "exact-duplicates" {
					inst.ClusterEndpoints[2].Host = member.URL
				}
				m := &Monitor{config: &config.Config{PVEInstances: []config.PVEInstance{inst}}}
				construct := func(conflict bool) *proxmox.ClusterClient {
					endpoints, pins, conflicts := m.buildClusterEndpointsForInit(inst)
					wantEndpoints, wantConflicts := 3, 0
					if layout == "exact-duplicates" {
						wantEndpoints = 2
						if conflict {
							wantConflicts = 1
						}
					}
					if len(endpoints) != wantEndpoints || len(pins) != wantEndpoints || len(conflicts) != wantConflicts {
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
					return proxmox.NewClusterClient(inst.Name, cfg, endpoints, pins, conflicts...)
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
					}{layout + "/" + mode, phase, inst.ClusterEndpoints})
					if err != nil {
						t.Fatal(err)
					}
					t.Logf("PIN_CONFLICT_ENDPOINT_PROJECTION_JSON=%s", wire)
				}
				refused := construct(true)
				project(refused, "conflicting-saved-trust", true)
				if badRequests.Load() != 0 || inst.ClusterEndpoints[2].Fingerprint != strings.Repeat("0", 64) {
					t.Fatal("conflict reached credentials or rewrote saved trust")
				}
				// Normal source configuration reload, not mutation of the owned map or a
				// forced/favourable repeat on the refused source. Native state stays online.
				inst.ClusterEndpoints[2].Fingerprint = pin(member)
				accepted := construct(false)
				project(accepted, "reconciled-config-reload", false)
				if badRequests.Load() == 0 {
					t.Fatal("agreed saved trust never reached accepted ordinary use")
				}
			})
		}
	}
}

// Exercise the owning monitor routes, not just equivalent calls to their
// builder. Discovery is allowed to report a collapsed/changed member list,
// but must not make saved ambiguity disappear or interrupt trusted siblings.
func TestClusterPinConflictsSurviveMonitorLifecycle(t *testing.T) {
	originalDelays := connRetryDelays
	connRetryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { connRetryDelays = originalDelays })
	for _, mode := range []string{"token", "password-json", "password-form"} {
		for _, layout := range []string{"url-aliases", "exact-duplicates"} {
			for _, route := range []string{"init", "reconnect", "refresh"} {
				t.Run(layout+"/"+mode+"/"+route, func(t *testing.T) {
					var badRequests, safeReads atomic.Int32
					endpoint := func(bad bool) *httptest.Server {
						s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							if bad {
								badRequests.Add(1)
							}
							switch r.URL.Path {
							case "/api2/json/access/ticket":
								if mode == "password-form" && r.Header.Get("Content-Type") == "application/json" {
									w.WriteHeader(http.StatusUnsupportedMediaType)
									return
								}
								fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
							case "/api2/json/nodes":
								if !bad {
									safeReads.Add(1)
								}
								fmt.Fprint(w, `{"data":[{"node":"primary","status":"online"},{"node":"member","status":"online"}]}`)
							default:
								t.Errorf("unexpected fixture request: %s", r.URL.Path)
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
					alias := member.URL + "/./"
					if layout == "exact-duplicates" {
						alias = member.URL
					}
					inst := config.PVEInstance{Name: "lifecycle-cluster", Host: primary.URL, IsCluster: true, ClusterName: "Fixture cluster", VerifySSL: true, Fingerprint: pin(primary), ClusterEndpoints: []config.ClusterEndpoint{
						{NodeName: "primary", NodeIdentity: "primary-id", Host: primary.URL, Fingerprint: pin(primary)},
						{NodeName: "member", NodeIdentity: "member-id", Host: member.URL, Fingerprint: pin(member)},
						{NodeName: "member-alias", NodeIdentity: "alias-id", Host: alias, Fingerprint: strings.Repeat("0", 64)},
					}}
					if mode == "token" {
						inst.TokenName, inst.TokenValue = "fixture@pve!pulse", "fixture"
					} else {
						inst.User, inst.Password = "fixture@pam", "fixture-password"
					}
					m := &Monitor{config: &config.Config{PVEInstances: []config.PVEInstance{inst}, ConnectionTimeout: 2 * time.Second}, pveClients: make(map[string]PVEClientInterface)}
					switch route {
					case "init":
						m.initPVEClients(m.config)
					case "reconnect":
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						m.retryFailedConnections(ctx)
						if err := ctx.Err(); err != nil {
							t.Fatalf("missing-client retry did not finish: %v", err)
						}
					case "refresh":
						m.rebuildPVEClusterClient(inst.Name)
						original := m.pveClients[inst.Name]
						// Discovery both drops the contradictory row and changes a
						// sibling's address. Neither is permission to reconcile trust.
						discovered := append([]config.ClusterEndpoint(nil), inst.ClusterEndpoints[:2]...)
						discovered[0].Host += "/"
						m.refreshClusterEndpoints(inst.Name, &m.config.PVEInstances[0], inst.ClusterName, discovered)
						if m.pveClients[inst.Name] != original || !reflect.DeepEqual(m.config.PVEInstances[0], inst) {
							t.Fatal("discovery erased conflicting saved trust or replaced active sibling clients")
						}
					}
					cc, ok := m.pveClients[inst.Name].(*proxmox.ClusterClient)
					if !ok {
						t.Fatal("owning monitor route did not retain its cluster client")
					}
					if _, err := cc.GetNodes(context.Background()); err != nil || safeReads.Load() == 0 || badRequests.Load() != 0 {
						t.Fatalf("monitor route lost sibling use or dispatched conflicting trust: %v safe=%d bad=%d", err, safeReads.Load(), badRequests.Load())
					}
					if health := cc.GetHealthStatusWithErrors()[member.URL]; health.Healthy || !strings.Contains(health.LastError, "TLS trust conflict") {
						t.Fatalf("monitor route lost member's trust refusal: %+v", health)
					}
					if !reflect.DeepEqual(m.config.PVEInstances[0], inst) {
						t.Fatal("monitor route changed saved configuration")
					}
					if route == "refresh" {
						// A normal independently reconciled configuration removes
						// the hold; automatic address refresh is not disabled forever.
						m.config.PVEInstances[0].ClusterEndpoints[2].Fingerprint = pin(member)
						discovered := append([]config.ClusterEndpoint(nil), m.config.PVEInstances[0].ClusterEndpoints...)
						discovered[0].Host += "/"
						m.refreshClusterEndpoints(inst.Name, &m.config.PVEInstances[0], inst.ClusterName, discovered)
						reloaded := m.pveClients[inst.Name]
						if reloaded == cc {
							t.Fatal("reconciled configuration did not resume address refresh")
						}
						if _, err := reloaded.GetNodes(context.Background()); err != nil || badRequests.Load() == 0 {
							t.Fatalf("reconciled refresh did not resume ordinary member use: %v", err)
						}
					}
				})
			}
		}
	}
}
