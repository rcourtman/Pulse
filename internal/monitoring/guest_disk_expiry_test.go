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

// These assertions also compile over the parent: retained identity and the
// latest poll time cannot establish an unknown original filesystem read time.
func testGuestDiskLegacyObservationAge(t *testing.T) {
	now := time.Date(2026, 10, 9, 14, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, previousReason string
		lastSeen             time.Time
		wantRetained         bool
	}{
		{"direct recent legacy observation", "", now.Add(-time.Minute), true},
		{"direct legacy at existing age boundary", "", now.Add(-recentGuestAgentEvidenceMaxAge), true},
		{"direct legacy expired", "", now.Add(-recentGuestAgentEvidenceMaxAge - time.Second), false},
		{"direct legacy missing time", "", time.Time{}, false},
		{"direct legacy future time", "", now.Add(time.Second), false},
		{"retained legacy missing original age", "prev-agent-error", now.Add(-time.Second), false},
		{"backup-retained legacy missing original age", "prev-vm-locked", now.Add(-time.Second), false},
		{"cooldown-retained legacy missing original age", "prev-agent-cooldown", now.Add(-time.Second), false},
		{"unavailable legacy with numeric carrier", "agent-error", now.Add(-time.Second), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prev := &models.VM{Type: "qemu", Status: "running", LastSeen: tc.lastSeen, AgentVersion: "retained-version", DiskStatusReason: tc.previousReason,
				Disk:  models.Disk{Total: 1000, Used: 400, Free: 600, Usage: 40},
				Disks: []models.Disk{{Total: 1000, Used: 400, Free: 600, Usage: 40, Mountpoint: "/"}}}
			total, used, free, usage, disks, reason := stabilizeGuestLowTrustDisk(prev, "running", 2000, 0, 2000, -1, nil, "agent-error", false, now)
			if tc.wantRetained {
				if total != 1000 || used != 400 || free != 600 || usage != 40 || len(disks) != 1 || reason != "prev-agent-error" {
					t.Errorf("recent direct observation lost: %d/%d/%d/%v/%d/%s", total, used, free, usage, len(disks), reason)
				}
			} else if total != 2000 || used != 0 || free != 2000 || usage != -1 || len(disks) != 0 || reason != "agent-error" {
				t.Errorf("unknown/expired original age became retained usage: %d/%d/%d/%v/%d/%s", total, used, free, usage, len(disks), reason)
			}
		})
	}
}

func testGuestDiskOriginalObservationAge(t *testing.T) {
	now := time.Date(2026, 10, 9, 14, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, source string
		at           time.Time
		wantRetained bool
	}{
		{"disk-only recent original", "guest-agent", now.Add(-time.Minute), true},
		{"disk-only inclusive original boundary", "guest-agent", now.Add(-recentGuestAgentEvidenceMaxAge), true},
		{"expired original despite fresh poll and identity", "guest-agent", now.Add(-recentGuestAgentEvidenceMaxAge - time.Nanosecond), false},
		{"future original", "guest-agent", now.Add(time.Second), false},
		{"missing original", "guest-agent", time.Time{}, false},
		{"unknown source", "unknown", now.Add(-time.Minute), false},
		{"missing source", "", now.Add(-time.Minute), false},
		{"linked agent is not QGA evidence", "agent", now.Add(-time.Minute), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prev := &models.VM{Type: "qemu", Status: "running", LastSeen: now, DiskStatusReason: "prev-agent-error",
				DiskObservation: models.GuestDiskObservation{Source: tc.source, ObservedAt: tc.at},
				Disk:            models.Disk{Total: 1000, Used: 400, Free: 600, Usage: 40},
				Disks:           []models.Disk{{Total: 1000, Used: 400, Free: 600, Usage: 40, Mountpoint: "/"}}}
			if !tc.wantRetained {
				prev.AgentVersion, prev.OSName = "retained-version", "retained-os"
			}
			_, used, _, usage, disks, reason := stabilizeGuestLowTrustDisk(prev, "running", 2000, 0, 2000, -1, nil, "vm-locked", false, now)
			if tc.wantRetained {
				if used != 400 || usage != 40 || len(disks) != 1 || reason != "prev-vm-locked" {
					t.Errorf("original disk-only evidence lost: %d/%v/%d/%s", used, usage, len(disks), reason)
				}
				disks[0].Used = 999
				if prev.Disks[0].Used != 400 {
					t.Fatal("retention aliased the previous disk inventory")
				}
			} else if used != 0 || usage != -1 || len(disks) != 0 || reason != "vm-locked" {
				t.Errorf("expired/missing/wrong-source evidence became usage: %d/%v/%d/%s", used, usage, len(disks), reason)
			}
			if prev.DiskObservation.ObservedAt != tc.at {
				t.Fatal("selector renewed the source time")
			}
		})
	}
}

func testGuestDiskRepeatedPollsAndCanonicalOrigin(t *testing.T) {
	original := time.Date(2026, 10, 9, 14, 0, 0, 0, time.UTC)
	prev := models.VM{ID: "disk-expiry:node:105", Instance: "disk-expiry", Node: "node", VMID: 105, Type: "qemu", Status: "running", LastSeen: original,
		DiskObservation: models.GuestDiskObservation{Source: "guest-agent", ObservedAt: original},
		Disk:            models.Disk{Total: 1000, Used: 400, Free: 600, Usage: 40}, Disks: []models.Disk{{Total: 1000, Used: 400, Free: 600, Usage: 40, Mountpoint: "/"}}}
	registry := unifiedresources.NewRegistry(nil)
	for minute := 1; minute <= 12; minute++ {
		t.Run(fmt.Sprintf("minute-%02d", minute), func(t *testing.T) {
			now := original.Add(time.Duration(minute) * time.Minute)
			total, used, free, usage, disks, reason := stabilizeGuestLowTrustDisk(&prev, "running", 2000, 0, 2000, -1, nil, "agent-error", false, now)
			vm := prev
			vm.LastSeen = now
			vm.Disk = models.Disk{Total: int64(total), Used: int64(used), Free: int64(free), Usage: usage}
			vm.Disks, vm.DiskStatusReason = disks, reason
			vm.DiskObservation = models.GuestDiskObservation{Source: "guest-agent"}
			if strings.HasPrefix(reason, "prev-") {
				vm.DiskObservation.ObservedAt = guestDiskObservationTime(&prev)
			}
			wantTime := time.Time{}
			if minute <= 10 {
				wantTime = original
				if used != 400 || usage != 40 || reason != "prev-agent-error" {
					t.Errorf("disk-only evidence lost before expiry: %d/%v/%s", used, usage, reason)
				}
			} else if used != 0 || usage != -1 || reason != "agent-error" {
				t.Errorf("minute %d renewed expired usage: %d/%v/%s", minute, used, usage, reason)
			}
			registry.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{vm}})
			view := registry.VMs()[0]
			prev = previousVMFromView(view)
			if prev.DiskObservation.Source != "guest-agent" || !prev.DiskObservation.ObservedAt.Equal(wantTime) || prev.DiskStatusReason != reason || !prev.LastSeen.Equal(now) {
				t.Fatal("canonical replacement renewed, lost or resurrected the disk's original source time")
			}
		})
	}
}

// Exercise ordinary HTTP reads and both guest collectors through the canonical
// previous-state boundary. Synthetic prior times avoid waiting ten minutes;
// no native backup/freeze, command failure injection or admission bypass occurs.
func testGuestDiskOrdinaryDeferralExpiryAndRecovery(t *testing.T) {
	const mib = uint64(1024 * 1024)
	for _, path := range []string{"cluster", "node"} {
		for _, reason := range []string{"vm-locked", "lock-unverified"} {
			t.Run(path+"/"+reason, func(t *testing.T) {
				var phase, commands atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch {
					case strings.HasSuffix(r.URL.Path, "/config"):
						if phase.Load() == 1 {
							if reason == "vm-locked" {
								fmt.Fprint(w, `{"data":{"lock":"backup"}}`)
							} else {
								fmt.Fprint(w, `{"data":null}`)
							}
						} else {
							fmt.Fprint(w, `{"data":{}}`)
						}
					case strings.HasSuffix(r.URL.Path, "/status/current"):
						fmt.Fprintf(w, `{"data":{"status":"running","agent":1,"maxmem":%d,"mem":%d,"meminfo":{"total":%d,"available":%d}}}`, 8*mib, 2*mib, 8*mib, 6*mib)
					case strings.Contains(r.URL.Path, "/agent/"):
						commands.Add(1)
						if strings.HasSuffix(r.URL.Path, "/get-fsinfo") {
							used := 980 * mib
							if phase.Load() == 2 {
								used = 0 // A measured zero is a successful recovery.
							}
							fmt.Fprintf(w, `{"data":{"result":[{"mountpoint":"C:\\","type":"ntfs","total-bytes":%d,"used-bytes":%d}]}}`, 1000*mib, used)
						} else {
							fmt.Fprint(w, `{"data":{"result":[]}}`)
						}
					default:
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
				if err != nil {
					t.Fatal(err)
				}
				m := guestHistoryObservationMonitor(t)
				m.alertManager = alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
				t.Cleanup(m.alertManager.Stop)
				alertConfig := m.alertManager.GetConfig()
				alertConfig.Enabled = true
				alertConfig.GuestDefaults.Disk = &alerts.HysteresisThreshold{Trigger: 90, Clear: 85}
				alertConfig.MetricTimeThresholds = map[string]map[string]int{"guest": {"disk": 0}}
				m.alertManager.UpdateConfig(alertConfig)
				m.config, m.state, m.rateTracker = &config.Config{}, models.NewState(), NewRateTracker()
				m.guestMetadataLimiter = make(map[string]time.Time)
				registry := unifiedresources.NewRegistry(nil)
				m.resourceStore = unifiedresources.NewMonitorAdapter(registry)
				res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "windows", VMID: 105, Status: "running", MaxMem: 8 * mib, Mem: 2 * mib, MaxDisk: 1000 * mib, CPU: .2}
				id := makeGuestID("disk-expiry", "node", 105)
				publish := func(vm models.VM) {
					// An accepted poll owns publication. Read hydration may reuse a
					// recent generation and is not an ingestion boundary.
					m.state.UpdateVMs([]models.VM{vm})
					m.updateResourceStore(m.currentStateWithScope())
				}
				build := func(ctx context.Context) models.VM {
					t.Helper()
					previous := m.previousGuestContextForInstance("disk-expiry").vmsByID[id]
					var vm models.VM
					if path == "node" {
						vms, _ := m.pollNodeVMsWithClusterResourceBuilder(ctx, "disk-expiry", "node", []proxmox.VM{{VMID: 105, Name: res.Name, Status: res.Status, MaxMem: res.MaxMem, Mem: res.Mem, MaxDisk: res.MaxDisk, CPU: res.CPU}}, client, map[string]models.VM{id: previous}, nil)
						if len(vms) != 1 {
							t.Fatal("node collector lost the guest")
						}
						vm = vms[0]
					} else {
						var ok bool
						vm, _, _, _, _, ok = m.buildVMFromClusterResource(ctx, "disk-expiry", res, client, id, nil, &previous)
						if !ok {
							t.Fatal("cluster builder lost the guest")
						}
						m.checkGuestAlertsForVM("disk-expiry", vm)
					}
					publish(vm)
					m.recordGuestMetrics([]models.VM{vm}, nil, time.Now().Add(-time.Second))
					view := m.currentModeReadState().VMs()[0]
					if view.DiskStatusReason() != vm.DiskStatusReason || view.DiskObservation() != vm.DiskObservation {
						t.Fatal("source disk observation changed at canonical read boundary")
					}
					front := m.buildBroadcastFrontendStateFromSnapshot(models.StateSnapshot{VMs: []models.VM{vm}}, m.mockModeFence.begin())
					wire, err := json.Marshal(front.Resources)
					if err != nil || strings.Contains(string(wire), "DiskObservation") || strings.Contains(string(wire), "diskObservation") {
						t.Fatal("internal carry-forward evidence leaked into public JSON")
					}
					var served []struct{ Proxmox unifiedresources.ProxmoxData }
					if err := json.Unmarshal(wire, &served); err != nil || len(served) != 1 || served[0].Proxmox.DiskStatusReason != vm.DiskStatusReason {
						t.Fatalf("served source reason differs from poll %q: %s / %v", vm.DiskStatusReason, wire, err)
					}
					return vm
				}
				initial := build(context.Background())
				if initial.Disk.Used != int64(980*mib) || initial.DiskStatusReason != "" || initial.DiskObservation.Source != "guest-agent" || initial.DiskObservation.ObservedAt.IsZero() || initial.DiskObservation.ObservedAt.After(initial.LastSeen) {
					t.Fatal("filesystem read did not establish its own original time")
				}
				originalHistory := guestHistoryStoredPoints(t, m, "vm", id, "disk")
				if len(originalHistory) != 1 || originalHistory[0].Value != 98 || len(m.alertManager.GetActiveAlerts()) != 1 {
					t.Fatal("current filesystem evidence failed to establish History and an alert")
				}
				before := commands.Load()
				phase.Store(1)
				for poll := 0; poll < 3; poll++ {
					retained := build(context.Background())
					if retained.Disk.Used != initial.Disk.Used || retained.DiskStatusReason != "prev-"+reason || retained.DiskObservation != initial.DiskObservation || len(retained.Disks) != 1 {
						t.Fatal("disk-only last-known value or original source time lost during deferral")
					}
				}
				// Shift only the accepted source's observation time, not LastSeen.
				// This models an extended pause without an actual ten-minute delay.
				aged := m.previousGuestContextForInstance("disk-expiry").vmsByID[id]
				aged.DiskObservation.ObservedAt = time.Now().Add(-recentGuestAgentEvidenceMaxAge - time.Second)
				publish(aged)
				expired := build(context.Background())
				if expired.Disk.Usage != -1 || expired.DiskStatusReason != reason || !expired.DiskObservation.ObservedAt.IsZero() || len(expired.Disks) != 0 || expired.GuestAgentStatus != "deferred" {
					t.Fatal("extended pause manufactured retained usage or hid the admission deferral")
				}
				// Exhausted detail budget may retain text inventory, but must not
				// restore the expired numeric summary or a successful read time.
				canceled, cancel := context.WithCancel(context.Background())
				cancel()
				budgeted := build(canceled)
				if strings.HasPrefix(budgeted.DiskStatusReason, "prev-") || !budgeted.DiskObservation.ObservedAt.IsZero() {
					t.Fatal("canceled enrichment revived expired disk evidence")
				}
				if commands.Load() != before || !reflect.DeepEqual(guestHistoryStoredPoints(t, m, "vm", id, "disk"), originalHistory) || len(m.metricsHistory.GetGuestMetrics(id, "disk", time.Hour)) != 1 || len(m.metricsHistory.GetGuestMetrics(id, "cpu", time.Hour)) != 6 || len(m.alertManager.GetActiveAlerts()) != 1 {
					t.Fatal("deferral dispatched a guest command, renewed disk History or suppressed independent CPU History")
				}
				phase.Store(2)
				// Persisted History keys have second precision; make recovery
				// distinct so zero cannot overwrite the original trusted point.
				time.Sleep(1100 * time.Millisecond)
				resumed := build(context.Background())
				if resumed.Disk.Usage != 0 || resumed.Disk.Used != 0 || resumed.DiskStatusReason != "" || resumed.DiskObservation.ObservedAt.IsZero() || !resumed.DiskObservation.ObservedAt.After(initial.DiskObservation.ObservedAt) || commands.Load() <= before || len(m.alertManager.GetActiveAlerts()) != 0 {
					t.Fatal("new ordinary filesystem read failed to recover with measured zero")
				}
				points := guestHistoryStoredPoints(t, m, "vm", id, "disk")
				if len(points) != 2 || points[0] != originalHistory[0] || points[1].Value != 0 {
					t.Fatal("ordinary recovery lost original History or failed to append measured zero")
				}
			})
		}
	}
}
