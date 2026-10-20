package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Both ordinary HTTP adapters -> polling -> collected state/recovery/alerts.
// The unread slow-node handler requires a healthy peer's content request to
// arrive before it can finish: a serial node scan cannot satisfy this control.
// No real provider, QGA, backup or recovery operation is involved.
func TestSnapshotSizeWorkersKeepHTTPPeersIndependent(t *testing.T) {
	for _, adapter := range []string{"single", "cluster"} {
		t.Run(adapter, func(t *testing.T) {
			const gib = int64(1 << 30)
			m, recoveryManager := recoveryIngestTestMonitor(t)
			m.state = models.NewState()
			m.alertManager = alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
			t.Cleanup(m.alertManager.Stop)
			cfg := m.alertManager.GetConfig()
			cfg.Enabled = true
			cfg.SnapshotDefaults = alerts.SnapshotAlertConfig{Enabled: true, WarningSizeGiB: 50, CriticalSizeGiB: 80}
			m.alertManager.UpdateConfig(cfg)
			m.state.UpdateGuestsForInstance("site", []models.VM{{ID: "site:a-slow:100", VMID: 100, Instance: "site", Node: "a-slow", Name: "database", Status: "running"}}, []models.Container{{ID: "site:b-peer:200", VMID: 200, Instance: "site", Node: "b-peer", Name: "worker", Status: "running"}})
			created := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
			type phaseInput struct {
				index    int
				peerRead chan struct{}
			}
			var phase atomic.Pointer[phaseInput]
			var unexpected, blockedPeerControls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					unexpected.Add(1)
					http.Error(w, "read-only fixture", http.StatusMethodNotAllowed)
					return
				}
				p := phase.Load()
				switch r.URL.Path {
				case "/api2/json/nodes":
					fmt.Fprint(w, `{"data":[{"node":"a-slow","status":"online"},{"node":"b-peer","status":"online"}]}`)
				case "/api2/json/nodes/a-slow/qemu/100/snapshot", "/api2/json/nodes/b-peer/lxc/200/snapshot":
					if p.index == 4 && strings.Contains(r.URL.Path, "/a-slow/") {
						fmt.Fprint(w, `{"data":[]}`)
					} else {
						fmt.Fprintf(w, `{"data":[{"name":"weekly","snaptime":%d}]}`, created.Unix())
					}
				case "/api2/json/nodes/a-slow/storage":
					fmt.Fprint(w, `{"data":[{"storage":"fast","content":"images","active":1,"enabled":1},{"storage":"remote","content":"images","active":1,"enabled":1}]}`)
				case "/api2/json/nodes/b-peer/storage":
					fmt.Fprint(w, `{"data":[{"storage":"fast","content":"rootdir","active":1,"enabled":1}]}`)
				case "/api2/json/nodes/a-slow/storage/fast/content":
					size := 15 * gib
					if p.index >= 3 {
						size = 10 * gib
					}
					fmt.Fprintf(w, `{"data":[{"volid":"fast:vm-100-disk-0@weekly","content":"images","vmid":100,"size":%d}]}`, size)
				case "/api2/json/nodes/a-slow/storage/remote/content":
					if p.index == 1 {
						select {
						case <-p.peerRead:
							blockedPeerControls.Add(1)
						case <-time.After(5 * time.Second):
							t.Error("healthy peer could not read while slow node occupied its sizing worker")
						}
						// An observed denial must keep the whole node unknown,
						// not publish its fast disk's 15 GiB partial sum.
						http.Error(w, "fixture unread storage", http.StatusForbidden)
						return
					}
					size := 45 * gib
					if p.index == 2 {
						size = 0
					} else if p.index >= 3 {
						size = 10 * gib
					}
					fmt.Fprintf(w, `{"data":[{"volid":"remote:vm-100-disk-1@weekly","content":"images","vmid":100,"size":%d}]}`, size)
				case "/api2/json/nodes/b-peer/storage/fast/content":
					if p.index == 1 {
						close(p.peerRead)
					}
					size := int64(p.index+1) * 12 * gib
					if p.index == 4 {
						size = 24 * gib
					}
					fmt.Fprintf(w, `{"data":[{"volid":"fast:subvol-200-disk-0@weekly","content":"rootdir","vmid":200,"size":%d}]}`, size)
				default:
					unexpected.Add(1)
					t.Errorf("unexpected provider operation: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			phase.Store(&phaseInput{})
			clientConfig := proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture-only", Timeout: 10 * time.Second}
			var client PVEClientInterface
			if adapter == "cluster" {
				client = proxmox.NewClusterClient("site", clientConfig, []string{server.URL}, nil)
			} else {
				var err error
				client, err = proxmox.NewClient(clientConfig)
				if err != nil {
					t.Fatal(err)
				}
			}
			wait := func() {
				t.Helper()
				deadline := time.Now().Add(10 * time.Second)
				for {
					m.recoveryIngestMu.Lock()
					pending := m.recoveryIngestRunning
					m.recoveryIngestMu.Unlock()
					if !pending {
						return
					}
					if time.Now().After(deadline) {
						t.Fatal("recovery ingestion did not settle")
					}
					time.Sleep(time.Millisecond)
				}
			}
			t.Cleanup(wait)
			frames := make(map[string]any)
			var originalID string
			var originalObservedAt any
			for p, name := range []string{"initial", "unread", "missing-size", "recovered", "empty"} {
				t.Run(name, func(t *testing.T) {
					phase.Store(&phaseInput{index: p, peerRead: make(chan struct{})})
					m.pollGuestSnapshots(context.Background(), "site", client)
					wait()
					state := m.GetState()
					wantSizes := map[string]int64{"a-slow": 0, "b-peer": int64(p+1) * 12 * gib}
					if p == 0 {
						wantSizes["a-slow"] = 60 * gib
					}
					if p == 3 {
						wantSizes["a-slow"] = 20 * gib
					}
					if p == 4 {
						delete(wantSizes, "a-slow")
						wantSizes["b-peer"] = 24 * gib
					}
					if len(state.PVEBackups.GuestSnapshots) != len(wantSizes) {
						t.Fatalf("snapshot inventory changed: %+v", state.PVEBackups)
					}
					for _, snap := range state.PVEBackups.GuestSnapshots {
						want, ok := wantSizes[snap.Node]
						if !ok || snap.SizeBytes != want {
							t.Fatalf("partial, stale or mixed-node size: %+v, wanted %v", snap, wantSizes)
						}
					}
					active := m.alertManager.GetActiveAlerts()
					if p < 3 {
						if len(active) != 1 || active[0].Value != 60 {
							t.Fatalf("unknown total replaced or cleared last breach: %+v", active)
						}
						if p == 0 {
							originalID, originalObservedAt = active[0].ID, active[0].Metadata["snapshotSizeObservedAt"]
							if err := m.alertManager.AcknowledgeAlert(originalID, "fixture operator"); err != nil {
								t.Fatal(err)
							}
						} else if active[0].ID != originalID || !active[0].Acknowledged || active[0].Metadata["snapshotSizeHeld"] != true || !reflect.DeepEqual(active[0].Metadata["snapshotSizeObservedAt"], originalObservedAt) || !strings.Contains(active[0].Message, "last known") {
							t.Fatalf("uncertainty renewed size evidence or lost held identity/acknowledgement: %+v", active[0])
						}
					} else if len(active) != 0 {
						t.Fatalf("complete smaller total did not resolve: %+v", active)
					}
					store, err := recoveryManager.StoreForOrg("default")
					if err != nil {
						t.Fatal(err)
					}
					points, _, err := store.ListPoints(context.Background(), recovery.ListPointsOptions{})
					if err != nil || len(points) != len(wantSizes) {
						t.Fatalf("recovery scope mismatch: %+v / %v", points, err)
					}
					for _, point := range points {
						node, _ := point.Details["node"].(string)
						want, ok := wantSizes[node]
						if !ok || (want == 0 && point.SizeBytes != nil) || (want > 0 && (point.SizeBytes == nil || *point.SizeBytes != want)) {
							t.Fatalf("stored partial or mixed total: %+v", point)
						}
					}
					frames[name] = map[string]any{"pve": state.PVEBackups, "active": m.alertManager.GetActiveAlerts(), "points": points}
				})
			}
			if unexpected.Load() != 0 || blockedPeerControls.Load() != 1 {
				t.Fatalf("HTTP census: unexpected=%d concurrent-peer-controls=%d", unexpected.Load(), blockedPeerControls.Load())
			}
			payload, err := json.Marshal(frames)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("SNAPSHOT_SIZE_WORKER_FRAMES %s %s", adapter, payload)
		})
	}
}
