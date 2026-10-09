package api

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Reproduce the alert button's exact request using a disk alert emitted by
// CheckHost. Keep Patrol busy so successful identity admission cannot call a
// model or start a background run. A valid identity must reach the busy guard.
func TestSupportHostDiskInvestigationContract(t *testing.T) {
	for _, linked := range []bool{false, true} {
		for _, unified := range []bool{false, true} {
			name := "snapshot"
			if unified {
				name = "unified"
			}
			if linked {
				name += "-linked-vm"
			} else {
				name += "-standalone"
			}
			t.Run(name, func(t *testing.T) {
				host := models.Host{
					ID: "repro-host-source-01", Hostname: "docker-host-ext-01",
					DisplayName: "docker-host-ext-01", Status: "online",
					MachineID: "repro-machine-01", LastSeen: time.Now(),
					Disks: []models.Disk{{Mountpoint: "/docker-data", Device: "/dev/sdb1", Usage: 85, Total: 1000, Used: 850, Free: 150}},
				}
				snapshot := models.StateSnapshot{Hosts: []models.Host{host, {
					ID: "unrelated-host", Hostname: "unrelated", MachineID: "unrelated-machine", Status: "online",
				}}}
				if linked {
					host.LinkedVMID = "vm-101"
					snapshot.Hosts[0] = host
					snapshot.VMs = []models.VM{{ID: "vm-101", Name: host.Hostname, VMID: 101, Node: "pve-1", Instance: "pve", Status: "running"}}
				}
				manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
				t.Cleanup(manager.Stop)
				// The production evaluator is unchanged; only temporal delay and
				// unrelated default policies are disabled for this immediate proof.
				setUnexportedField(t, manager, "config", alerts.AlertConfig{
					Enabled: true, ActivationState: alerts.ActivationActive,
					AgentDefaults: alerts.ThresholdConfig{Disk: &alerts.HysteresisThreshold{Trigger: 85, Clear: 80}},
					Overrides:     map[string]alerts.ThresholdConfig{}, TimeThresholds: map[string]int{},
				})
				manager.CheckHost(host)
				active := manager.GetActiveAlerts()
				if len(active) != 1 {
					t.Fatalf("disk fixture did not produce exactly one alert: %+v", active)
				}
				alert := active[0]
				if alert.ResourceID != "agent:repro-host-source-01/disk:docker-data" || alert.Type != "disk" {
					t.Fatalf("unexpected emitted disk identity: %+v", alert)
				}
				t.Logf("emitted resource_id=%q resource_name=%q hostId=%v mountpoint=%v", alert.ResourceID, alert.ResourceName, alert.Metadata["hostId"], alert.Metadata["mountpoint"])
				snapshot.ActiveAlerts = []models.Alert{hostDiskPatrolStateAlert(alert)}
				handler, patrol, _, _ := setupAIHandlerWithPatrol(t)
				seedReadyAnthropicPatrolRuntime(t, handler)
				handler.defaultAIService.SetStateProvider(&scopedPatrolStateProvider{state: snapshot})
				parentID := host.ID
				if unified {
					store := unifiedresources.NewMemoryStore()
					registry := unifiedresources.NewRegistry(store)
					registry.IngestSnapshot(snapshot)
					if linked {
						_, hostCanonicalID, hostFound := registry.GetByReference(host.ID)
						_, vmCanonicalID, vmFound := registry.GetByReference("vm-101")
						if !hostFound || !vmFound {
							t.Fatal("manual-link fixture is missing a source")
						}
						if err := store.AddLink(unifiedresources.ResourceLink{ResourceA: hostCanonicalID, ResourceB: vmCanonicalID, PrimaryID: vmCanonicalID}); err != nil {
							t.Fatal(err)
						}
						registry = unifiedresources.NewRegistry(store)
						registry.IngestSnapshot(snapshot)
						_, foldedOwner, found := registry.GetByReference(host.ID)
						if !found || foldedOwner != vmCanonicalID {
							t.Fatalf("fixture host did not merge into VM: %q want %q", foldedOwner, vmCanonicalID)
						}
						t.Logf("manual-link host=%q folded into VM=%q", hostCanonicalID, foldedOwner)
					}
					handler.defaultAIService.SetReadState(registry)
					patrol.SetUnifiedResourceProvider(unifiedresources.NewUnifiedAIAdapter(registry))
					resource, canonicalID, ok := registry.GetByReference(host.ID)
					if !ok {
						t.Fatalf("host source identity missing from canonical registry")
					}
					parentID = canonicalID
					t.Logf("canonical owner id=%q type=%q aliases=%+v sources=%+v", canonicalID, resource.Type, resource.Canonical, registry.SourceTargets(canonicalID))
					if _, _, ok := registry.GetByReference(alert.ResourceID); ok {
						t.Log("disk alert unexpectedly has a canonical registry resource")
					}
				}
				_, parentResolution := patrol.ResolvePatrolScope(ai.PatrolScope{ResourceIDs: []string{parentID}})
				if len(parentResolution.UnmatchedResourceIDs) != 0 || len(parentResolution.AmbiguousResourceIDs) != 0 || len(parentResolution.EffectiveResourceIDs) == 0 {
					t.Fatalf("parent control did not resolve: %+v", parentResolution)
				}
				t.Logf("parent control resolution=%+v", parentResolution)
				_, diskResolution := patrol.ResolvePatrolScope(ai.PatrolScope{ResourceIDs: []string{alert.ResourceID}, AlertIdentifier: alert.ID})
				t.Logf("disk resolution=%+v", diskResolution)
				setUnexportedField(t, patrol, "runInProgress", true)
				setUnexportedField(t, patrol, "currentRunID", "repro-busy-sentinel")
				request := func(resourceID string) *httptest.ResponseRecorder {
					body, err := json.Marshal(map[string]any{"resource_ids": []string{resourceID}, "alert_identifier": alert.ID, "alert_type": alert.Type})
					if err != nil {
						t.Fatal(err)
					}
					rec := httptest.NewRecorder()
					handler.HandleForcePatrol(rec, newLoopbackRequest(http.MethodPost, "/api/ai/patrol/run", bytes.NewReader(body)))
					return rec
				}
				control := request(parentID)
				if control.Code != http.StatusConflict || !strings.Contains(control.Body.String(), "patrol_already_running") {
					t.Fatalf("known parent did not reach downstream busy guard: %d %s", control.Code, control.Body.String())
				}
				rec := request(alert.ResourceID)
				t.Logf("disk HTTP status=%d body=%s", rec.Code, rec.Body.String())
				if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "patrol_already_running") {
					t.Fatalf("disk alert must resolve before busy admission, got %d %s", rec.Code, rec.Body.String())
				}
			})
		}
	}
}

// Opt-in loopback backend for scripts/check-host-disk-patrol.mjs. The browser
// exercises the real admission handler against a production-emitted alert.
// Inference is absent, so acceptance is not a claim of completed diagnosis.
func TestPatrolHostDiskBrowserBackend(t *testing.T) {
	address := os.Getenv("PULSE_HOST_DISK_BROWSER_ADDRESS")
	if address == "" {
		t.Skip("opt-in browser admission fixture")
	}
	hostIP, _, err := net.SplitHostPort(address)
	if err != nil || hostIP != "127.0.0.1" {
		t.Fatal("browser fixture requires a loopback address")
	}
	host := models.Host{ID: "repro-host-source-01", Hostname: "docker-host-ext-01", MachineID: "repro-machine-01", Status: "online", LastSeen: time.Now(), Disks: []models.Disk{{Mountpoint: "/docker-data", Device: "/dev/sdb1", Usage: 85, Total: 1000, Used: 850, Free: 150}}}
	manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)
	setUnexportedField(t, manager, "config", alerts.AlertConfig{Enabled: true, ActivationState: alerts.ActivationActive, AgentDefaults: alerts.ThresholdConfig{Disk: &alerts.HysteresisThreshold{Trigger: 85, Clear: 80}}, Overrides: map[string]alerts.ThresholdConfig{}, TimeThresholds: map[string]int{}})
	manager.CheckHost(host)
	active := manager.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("fixture alert missing: %+v", active)
	}
	stop := make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/proof/alert", func(w http.ResponseWriter, r *http.Request) { _ = json.NewEncoder(w).Encode(active[0]) })
	mux.HandleFunc("/proof/stop", func(w http.ResponseWriter, r *http.Request) { stop <- struct{}{} })
	mux.HandleFunc("/api/ai/patrol/run", func(w http.ResponseWriter, r *http.Request) {
		handler, patrol, _, _ := setupAIHandlerWithPatrol(t)
		seedReadyAnthropicPatrolRuntime(t, handler)
		handler.defaultAIService.SetChatService(nil)
		snapshot := models.StateSnapshot{Hosts: []models.Host{host}, ActiveAlerts: []models.Alert{hostDiskPatrolStateAlert(active[0])}}
		if r.URL.Query().Get("case") == "unresolved" {
			snapshot.Hosts = nil
		}
		handler.defaultAIService.SetStateProvider(&scopedPatrolStateProvider{state: snapshot})
		if r.URL.Query().Get("case") == "busy" {
			setUnexportedField(t, patrol, "runInProgress", true)
			setUnexportedField(t, patrol, "currentRunID", "browser-busy")
		}
		handler.HandleForcePatrol(w, r)
	})
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(mux)
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	t.Logf("browser admission fixture ready at %s", server.URL)
	select {
	case <-stop:
	case <-time.After(4 * time.Minute):
		t.Fatal("browser fixture was not stopped")
	}
}

func hostDiskPatrolStateAlert(alert alerts.Alert) models.Alert {
	return models.Alert{ID: alert.ID, Type: alert.Type, Level: string(alert.Level), ResourceID: alert.ResourceID, ResourceName: alert.ResourceName, Message: alert.Message, Value: alert.Value, Threshold: alert.Threshold, StartTime: alert.StartTime, Metadata: alert.Metadata}
}
