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
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Walk the actual efficient enumeration -> fallback -> accepted publication ->
// canonical projection. Malformed successes cannot remove guests, refresh their
// last observation, clear a breach or invalidate template backup eligibility.
// All endpoints are synthetic; no backup, guest command or live send occurs.
func TestGuestInventoryFailurePreservesOwnedReadingsAndRecovers(t *testing.T) {
	for _, failure := range []string{"missing-data", "trailing-value", "truncated-prefix", "duplicate-id"} {
		t.Run(failure, func(t *testing.T) {
			t.Setenv("PULSE_DATA_DIR", t.TempDir())
			var phase, commands, clusterReads, listReads atomic.Int32
			corrupt := func(w http.ResponseWriter, prefix string) {
				switch failure {
				case "missing-data":
					fmt.Fprint(w, `{}`)
				case "trailing-value":
					fmt.Fprint(w, prefix+` {"errors":{"message":"fixture-private-detail"}}`)
				case "truncated-prefix":
					w.Header().Set("Content-Length", fmt.Sprint(len(prefix)+100))
					fmt.Fprint(w, prefix)
				case "duplicate-id":
					var payload struct {
						Data []json.RawMessage `json:"data"`
					}
					if err := json.Unmarshal([]byte(prefix), &payload); err != nil {
						t.Error(err)
						return
					}
					payload.Data = append(payload.Data, payload.Data[0])
					_ = json.NewEncoder(w).Encode(payload)
				}
			}
			const vm = `{"vmid":105,"type":"qemu","node":"member","name":"kept-vm","status":"running","lock":"backup","cpu":0.95,"maxmem":1073741824,"maxdisk":2147483648}`
			const extra = `{"vmid":106,"type":"qemu","node":"member","name":"later-deleted-vm","status":"running","lock":"backup","cpu":0.1,"maxmem":1073741824}`
			const ct = `{"vmid":205,"type":"lxc","node":"member","name":"kept-container","status":"running","lock":"backup","cpu":0.15,"mem":104857600,"maxmem":1073741824,"disk":268435456,"maxdisk":2147483648}`
			const template = `{"vmid":999,"type":"qemu","node":"member","template":1}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				p := phase.Load()
				switch {
				case strings.HasSuffix(r.URL.Path, "/cluster/resources"):
					clusterReads.Add(1)
					if p == 1 || p == 2 {
						corrupt(w, `{"data":[`+vm+`]}`)
						return
					}
					if p == 4 {
						http.NotFound(w, r)
						return
					}
					rows := vm + `,` + ct
					if p == 0 {
						rows += `,` + extra + `,` + template
					}
					if p == 3 {
						rows += `,{"vmid":108,"type":"qemu","node":"member","name":"new-vm","status":"running","lock":"backup","cpu":0,"maxmem":1073741824}`
					}
					fmt.Fprint(w, `{"data":[`+rows+`]}`)
				case strings.HasSuffix(r.URL.Path, "/member/qemu"):
					listReads.Add(1)
					if p == 1 {
						corrupt(w, `{"data":[`+vm+`]}`)
						return
					}
					if p == 4 {
						fmt.Fprint(w, `{"data":[]}`)
						return
					}
					fmt.Fprint(w, `{"data":[`+vm+`]}`)
				case strings.HasSuffix(r.URL.Path, "/member/lxc"):
					listReads.Add(1)
					if p == 1 {
						corrupt(w, `{"data":[`+ct+`]}`)
						return
					}
					if p == 4 {
						fmt.Fprint(w, `{"data":[]}`)
						return
					}
					// Keep the existing older-PVE string identifier compatibility on fallback.
					fmt.Fprint(w, `{"data":[`+strings.Replace(ct, `"vmid":205`, `"vmid":"205"`, 1)+`]}`)
				case strings.Contains(r.URL.Path, "/agent/"):
					commands.Add(1)
					t.Error("inventory proof dispatched an in-guest command")
					http.Error(w, "unexpected command", 500)
				case strings.HasSuffix(r.URL.Path, "/qemu/105/status/current"), strings.HasSuffix(r.URL.Path, "/qemu/106/status/current"), strings.HasSuffix(r.URL.Path, "/qemu/108/status/current"):
					fmt.Fprint(w, `{"data":{"status":"running","lock":"backup","maxmem":1073741824,"mem":104857600}}`)
				case strings.HasSuffix(r.URL.Path, "/lxc/205/status/current"):
					fmt.Fprint(w, `{"data":{"vmid":205,"status":"running","lock":"backup","maxmem":1073741824,"mem":104857600,"maxdisk":2147483648,"disk":268435456}}`)
				case strings.HasSuffix(r.URL.Path, "/config"):
					fmt.Fprint(w, `{"data":{"lock":"backup"}}`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 10 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			m := newTestPVEMonitor("inventory")
			t.Cleanup(m.alertManager.Stop)
			t.Cleanup(m.notificationMgr.Stop)
			m.replacePVEClient("inventory", client)
			alertCfg := m.alertManager.GetConfig()
			alertCfg.Enabled = true
			alertCfg.GuestDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}
			alertCfg.MetricTimeThresholds = map[string]map[string]int{"guest": {"cpu": 0}}
			m.alertManager.UpdateConfig(alertCfg)
			run := func() models.StateSnapshot {
				t.Helper()
				if err := m.pollGuestsWithFallback(context.Background(), "inventory", &config.PVEInstance{MonitorVMs: true, MonitorContainers: true}, client, []proxmox.Node{{Node: "member", Status: "online"}}, map[string]string{"member": "online"}); err != nil {
					t.Fatal(err)
				}
				return m.state.GetSnapshot()
			}
			// A second registered connection has its own observation and must survive
			// both failed enumerations and actual removals on this one.
			peer := models.VM{ID: "peer:other:305", Instance: "peer", Node: "other", VMID: 305, Name: "independent-peer", Type: "qemu", Status: "running", LastSeen: time.Now()}
			m.state.UpdateVMsForInstance("peer", []models.VM{peer})
			original := run()
			if len(original.VMs) != 3 || len(original.Containers) != 1 {
				t.Fatalf("initial complete inventory not established: %+v", original)
			}
			templateKey := pveBackupTemplateSubjectKey("inventory", "qemu", "member", 999)
			if _, ok := m.pveBackupTemplateSubjects["inventory"][templateKey]; !ok {
				t.Fatal("initial template not established")
			}
			originalHistory := m.metricsHistory.GetGuestMetrics("inventory:member:105", "cpu", time.Hour)
			originalAlerts := m.alertManager.GetActiveAlerts()
			if len(originalHistory) != 1 || len(originalAlerts) != 1 {
				t.Fatalf("initial history/breach not established: %d/%d", len(originalHistory), len(originalAlerts))
			}
			emit := func(label string, snapshot models.StateSnapshot) {
				registry := unifiedresources.NewRegistry(nil)
				registry.IngestSnapshot(snapshot)
				projection, err := json.Marshal(map[string]any{"case": failure, "phase": label, "snapshot": snapshot, "resources": registry.List()})
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("GUEST_INVENTORY_PROJECTION %s", projection)
			}
			emit("initial", original)
			phase.Store(1)
			failed := run()
			if !reflect.DeepEqual(failed.VMs, original.VMs) || !reflect.DeepEqual(failed.Containers, original.Containers) {
				t.Error("incomplete inventory changed retained guests or their observation time")
			}
			if !reflect.DeepEqual(m.metricsHistory.GetGuestMetrics("inventory:member:105", "cpu", time.Hour), originalHistory) {
				t.Error("failed inventory fabricated a new History sample")
			}
			if !reflect.DeepEqual(m.alertManager.GetActiveAlerts(), originalAlerts) {
				t.Error("failed inventory changed an existing breach")
			}
			if _, ok := m.pveBackupTemplateSubjects["inventory"][templateKey]; !ok {
				t.Error("failed inventory erased backup-template eligibility")
			}
			emit("both-inventories-refused", failed)
			phase.Store(2)
			fallback := run()
			if len(fallback.VMs) != 2 || len(fallback.Containers) != 1 || fallback.VMs[0].VMID == 106 {
				t.Error("complete fallback failed to publish a real deletion")
			}
			if _, ok := m.pveBackupTemplateSubjects["inventory"][templateKey]; ok {
				t.Error("complete fallback retained a deleted template")
			}
			if len(m.metricsHistory.GetGuestMetrics("inventory:member:105", "cpu", time.Hour)) != 2 {
				t.Error("complete fallback failed to resume History")
			}
			emit("complete-node-fallback", fallback)
			before := listReads.Load()
			phase.Store(3)
			recovered := run()
			if len(recovered.VMs) != 3 || len(recovered.Containers) != 1 || listReads.Load() != before {
				t.Error("complete cluster recovery did not return to the efficient route")
			}
			emit("complete-cluster-recovery", recovered)
			phase.Store(4)
			empty := run()
			if len(empty.VMs) != 1 || empty.VMs[0].ID != peer.ID || len(empty.Containers) != 0 {
				t.Error("explicit empty node inventories failed to remove guests within their scope")
			}
			if commands.Load() != 0 || clusterReads.Load() != 5 || listReads.Load() != 6 || len(m.pveGuestPolls) != 0 {
				t.Errorf("unexpected command/retry/owner leak: %d/%d/%d/%d", commands.Load(), clusterReads.Load(), listReads.Load(), len(m.pveGuestPolls))
			}
			emit("complete-scoped-empty", empty)
		})
	}
}
