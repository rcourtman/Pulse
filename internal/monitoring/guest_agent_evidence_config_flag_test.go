package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Real qemu-server status/current reports `"agent": 1` whenever the guest
// agent is enabled in the VM config (PVE/API2/Qemu.pm vm_status), whether or
// not the in-guest service ever answers. That flag must not renew fallback
// eligibility: only an actual agent reply may. All HTTP endpoints are synthetic.
func testGuestAgentConfigFlagCannotRenewAdmission(t *testing.T) {
	for _, agentAnswers := range []bool{false, true} {
		name := "enabled-but-silent"
		if agentAnswers {
			name = "answered"
		}
		t.Run(name, func(t *testing.T) {
			var statusFails atomic.Bool
			var fallbackGuestCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "/config"):
					fmt.Fprint(w, `{"data":{"agent":"1"}}`)
				case strings.HasSuffix(r.URL.Path, "/status/current"):
					if statusFails.Load() {
						http.Error(w, "permission denied", http.StatusForbidden)
						return
					}
					fmt.Fprint(w, `{"data":{"status":"running","agent":1,"maxmem":8000,"meminfo":{"available":6000}}}`)
				case strings.Contains(r.URL.Path, "/agent/"):
					if statusFails.Load() {
						fallbackGuestCalls.Add(1)
					}
					if !agentAnswers {
						http.Error(w, "QEMU guest agent is not running", http.StatusInternalServerError)
						return
					}
					switch {
					case strings.HasSuffix(r.URL.Path, "get-fsinfo"):
						fmt.Fprint(w, `{"data":{"result":[{"mountpoint":"/","type":"ext4","total-bytes":1000,"used-bytes":250}]}}`)
					case strings.HasSuffix(r.URL.Path, "network-get-interfaces"):
						fmt.Fprint(w, `{"data":{"result":[{"name":"eth0","ip-addresses":[{"ip-address":"192.0.2.12","ip-address-type":"ipv4","prefix":24}]}]}}`)
					case strings.HasSuffix(r.URL.Path, "get-osinfo"):
						fmt.Fprint(w, `{"data":{"result":{"name":"Debian","version":"fixture"}}}`)
					case strings.HasSuffix(r.URL.Path, "info"):
						fmt.Fprint(w, `{"data":{"result":{"version":"2.0"}}}`)
					default:
						http.NotFound(w, r)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 5 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			m := &Monitor{config: &config.Config{}, state: models.NewState(), rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour), guestMetadataLimiter: make(map[string]time.Time)}
			m.alertManager = alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
			t.Cleanup(m.alertManager.Stop)
			m.guestMetadataRetryBackoff = time.Nanosecond
			id := makeGuestID("flag", "node", 107)
			prev := models.VM{ID: id, Instance: "flag", Node: "node", VMID: 107, Type: "qemu", Status: "running", LastSeen: time.Now()}
			res := proxmox.ClusterResource{Type: "qemu", Node: "node", VMID: 107, Name: "guest", Status: "running", MaxMem: 8000, Mem: 2000, MaxDisk: 1000, CPU: .2}
			registry := unifiedresources.NewRegistry(nil)
			build := func() models.VM {
				t.Helper()
				vms := m.collectClusterVMResources(context.Background(), "flag", []indexedClusterResource{{resource: res, guestID: id}}, client, map[string]models.VM{id: prev}, nil)
				if len(vms) != 1 {
					t.Fatal("guest disappeared")
				}
				registry.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{vms[0]}})
				prev = previousVMFromView(registry.VMs()[0])
				return vms[0]
			}

			for poll := 0; poll < 2; poll++ {
				build()
			}
			if got := hasRecentGuestAgentEvidence(&prev, time.Now()); got != agentAnswers {
				t.Fatalf("eligibility after status reads=%t, want %t (evidence %+v)", got, agentAnswers, prev.GuestAgentEvidence)
			}

			statusFails.Store(true)
			build()
			if agentAnswers && fallbackGuestCalls.Load() == 0 {
				t.Fatal("recent real agent reply did not keep fallback eligibility after status failed")
			}
			if !agentAnswers && fallbackGuestCalls.Load() != 0 {
				t.Fatalf("config-level agent flag admitted %d fallback guest commands after status failed", fallbackGuestCalls.Load())
			}
		})
	}
}
