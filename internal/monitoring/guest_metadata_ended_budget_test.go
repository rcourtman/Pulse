package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Cancellation before a callback starts is not an OS-command outcome. Once a
// callback starts, cancellation cannot establish whether its command ran.
func TestGuestAgentEndedBudgetAdmission(t *testing.T) {
	for _, budget := range []string{"cancelled", "expired"} {
		for _, retries := range []int{0, 2} {
			t.Run(fmt.Sprintf("%s/retries-%d", budget, retries), func(t *testing.T) {
				ctx, cancel := endedMetadataBudget(budget)
				defer cancel()
				calls := 0
				_, err := (&Monitor{}).retryGuestAgentCall(ctx, time.Second, retries, func(context.Context) (interface{}, error) {
					calls++
					return "unexpected", nil
				})
				if calls != 0 || !errors.Is(err, ctx.Err()) || errors.Is(err, proxmox.ErrGuestAgentDeferred) {
					t.Errorf("ended budget invoked a callback or invented a shared pause: calls=%d error=%v", calls, err)
				}
			})
		}
	}
}

func endedMetadataBudget(budget string) (context.Context, context.CancelFunc) {
	if budget == "expired" {
		return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx, cancel
}

type endedBudgetMetadataClient struct {
	PVEClientInterface
	network, os, version  int
	afterNetwork, afterOS func()
}

func (c *endedBudgetMetadataClient) GetVMNetworkInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.VMNetworkInterface, error) {
	c.network++
	data, err := c.PVEClientInterface.GetVMNetworkInterfaces(ctx, node, vmid)
	if c.afterNetwork != nil {
		c.afterNetwork()
	}
	return data, err
}

func (c *endedBudgetMetadataClient) GetVMAgentInfo(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	c.os++
	data, err := c.PVEClientInterface.GetVMAgentInfo(ctx, node, vmid)
	if c.afterOS != nil {
		c.afterOS()
	}
	return data, err
}

func (c *endedBudgetMetadataClient) GetVMAgentVersion(ctx context.Context, node string, vmid int) (string, error) {
	c.version++
	return c.PVEClientInterface.GetVMAgentVersion(ctx, node, vmid)
}

type metadataBudgetFixture struct {
	commands, osCommands atomic.Int32
	recovered, rejectOS  atomic.Bool
}

func (f *metadataBudgetFixture) client(t *testing.T, kind string) PVEClientInterface {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
			t.Error("read-only fixture authentication changed")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/agent/") {
			f.commands.Add(1)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/nodes"):
			fmt.Fprint(w, `{"data":[]}`)
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, `{"data":{}}`)
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			used, available := 3*bootstrapMiB, 5*bootstrapMiB
			if f.recovered.Load() {
				used, available = 4*bootstrapMiB, 4*bootstrapMiB
			}
			fmt.Fprintf(w, `{"data":{"status":"running","agent":1,"maxmem":8388608,"mem":%d,"meminfo":{"total":8388608,"available":%d}}}`, used, available)
		case strings.HasSuffix(r.URL.Path, "/get-fsinfo"):
			fmt.Fprint(w, `{"data":{"result":[{"name":"C:","mountpoint":"C:\\","type":"ntfs","total-bytes":1572864000,"used-bytes":419430400},{"name":"D:","mountpoint":"D:\\","type":"ntfs","total-bytes":524288000,"used-bytes":104857600}]}}`)
		case strings.HasSuffix(r.URL.Path, "/network-get-interfaces"):
			fmt.Fprint(w, `{"data":{"result":[{"name":"Ethernet","hardware-address":"02:00:00:00:00:01","ip-addresses":[{"ip-address":"192.0.2.20","prefix":24,"ip-address-type":"ipv4"}]}]}}`)
		case strings.HasSuffix(r.URL.Path, "/get-osinfo"):
			f.osCommands.Add(1)
			if f.rejectOS.Load() {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, "completed OS command rejection")
			} else {
				fmt.Fprint(w, `{"data":{"result":{"name":"Windows","version":"2025"}}}`)
			}
		case strings.HasSuffix(r.URL.Path, "/info"):
			fmt.Fprint(w, `{"data":{"result":{"version":"2.0"}}}`)
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return windowsBootstrapClient(t, server.URL, kind)
}

func metadataBudgetMonitor(t *testing.T, seed string) (*Monitor, string, guestMetadataCacheEntry) {
	key := guestMetadataCacheKey("budget", "node", 105)
	entry := guestMetadataCacheEntry{}
	if seed != "cold" {
		entry = metadataObservationFixture(time.Now().Add(-2 * guestMetadataCacheTTL))
		entry.osInfoFailureCount = guestAgentOSInfoFailureThreshold - 1
		if seed == "suppressed" {
			entry.osInfoFailureCount = guestAgentOSInfoFailureThreshold
			entry.osInfoSkip = true
		}
	}
	t.Helper()
	m := &Monitor{config: &config.Config{}, state: models.NewState(), rateTracker: NewRateTracker(),
		guestMetadataCache:   map[string]guestMetadataCacheEntry{key: entry},
		guestMetadataLimiter: make(map[string]time.Time), guestAgentRetries: 2,
		guestAgentNetworkTimeout: time.Second, guestAgentOSInfoTimeout: time.Second, guestAgentVersionTimeout: time.Second}
	m.alertManager = alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(m.alertManager.Stop)
	return m, key, entry
}

func TestGuestMetadataEndedBudgetPreservesSafetyState(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, seed := range []string{"cold", "partial-failure", "suppressed"} {
			for _, budget := range []string{"cancelled", "expired"} {
				t.Run(kind+"/"+seed+"/"+budget, func(t *testing.T) {
					f := &metadataBudgetFixture{}
					client := &endedBudgetMetadataClient{PVEClientInterface: f.client(t, kind)}
					m, key, original := metadataBudgetMonitor(t, seed)
					status := &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}
					for round := 0; round < guestAgentOSInfoFailureThreshold; round++ {
						// Advance only test-owned scheduling, never a command uncertainty
						// fence or the OS suppression being tested.
						m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
						ctx, cancel := endedMetadataBudget(budget)
						ips, ifaces, name, version, agent, paused := m.fetchGuestAgentMetadata(ctx, client, "budget", "node", "guest", 105, status, false)
						cancel()
						if paused || !reflect.DeepEqual(m.guestMetadataCache[key], original) || !reflect.DeepEqual(ips, original.ipAddresses) || !reflect.DeepEqual(ifaces, original.networkInterfaces) || name != original.osName || version != original.osVersion || agent != original.agentVersion {
							t.Errorf("ended budget changed safety/observation state: %+v; want %+v; paused=%t", m.guestMetadataCache[key], original, paused)
						}
						if !m.guestMetadataLimiter[key].After(time.Now()) || m.hasRecentGuestMetadataEvidence("budget", "node", 105, time.Now()) {
							t.Error("ended budget bypassed backoff or renewed observation age")
						}
						assertMetadataReservationReleased(t, m)
					}
					if client.network+client.os+client.version != 0 || f.commands.Load() != 0 {
						t.Errorf("ended budget entered a callback/command: %+v, commands=%d", client, f.commands.Load())
					}
					m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
					_, _, name, version, _, paused := m.fetchGuestAgentMetadata(context.Background(), client, "budget", "node", "guest", 105, status, false)
					wantOS := int32(1)
					if seed == "suppressed" {
						wantOS = 0
					}
					if paused || f.osCommands.Load() != wantOS || (seed != "suppressed" && (name != "Windows" || version != "2025" || m.guestMetadataCache[key].osInfoSkip)) {
						t.Error("ordinary recovery lost OS admission or weakened genuine suppression")
					}
				})
			}
		}
	}
}

func TestGuestMetadataBudgetEndsAfterNetworkRecovery(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, seed := range []string{"cold", "partial-failure", "suppressed"} {
			for _, collector := range []string{"cluster", "node"} {
				t.Run(kind+"/"+seed+"/"+collector, func(t *testing.T) {
					f := &metadataBudgetFixture{}
					client := &endedBudgetMetadataClient{PVEClientInterface: f.client(t, kind)}
					m, key, original := metadataBudgetMonitor(t, seed)
					status := &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}
					for round := 0; round < guestAgentOSInfoFailureThreshold; round++ {
						entry := m.guestMetadataCache[key]
						entry.fetchedAt = time.Now().Add(-2 * guestMetadataCacheTTL)
						m.guestMetadataCache[key] = entry
						m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
						ctx, cancel := context.WithCancel(context.Background())
						client.afterNetwork = cancel // Both config checks and the body already completed.
						start := time.Now()
						_, _, _, _, _, paused := m.fetchGuestAgentMetadata(ctx, client, "budget", "node", "guest", 105, status, false)
						cancel()
						entry = m.guestMetadataCache[key]
						if paused || entry.osInfoFailureCount != original.osInfoFailureCount || entry.osInfoSkip != original.osInfoSkip || entry.osName != original.osName || !entry.osInfoObservedAt.Equal(original.osInfoObservedAt) {
							t.Errorf("unstarted OS command changed its safety/identity: %+v", entry)
						}
						if entry.fetchedAt.Before(start) || !m.hasRecentGuestMetadataEvidence("budget", "node", 105, time.Now()) || len(entry.networkInterfaces) != 1 || entry.networkInterfaces[0].Name != "Ethernet" {
							t.Error("completed network observation was discarded")
						}
						assertMetadataReservationReleased(t, m)
					}
					client.afterNetwork = nil
					if client.network != guestAgentOSInfoFailureThreshold || client.os+client.version != 0 || f.commands.Load() != guestAgentOSInfoFailureThreshold {
						t.Errorf("budget admitted a later callback/command: network=%d os=%d version=%d commands=%d", client.network, client.os, client.version, f.commands.Load())
					}
					poll := func(previous *models.VM) models.VM {
						cpu := .1
						if f.recovered.Load() {
							cpu = .2
						}
						id := makeGuestID("budget", "node", 105)
						if collector == "node" {
							prior := make(map[string]models.VM)
							if previous != nil {
								prior[id] = *previous
							}
							vms, _ := m.pollNodeVMsWithClusterResourceBuilder(context.Background(), "budget", "node", []proxmox.VM{{VMID: 105, Name: "budget-guest", Status: "running", MaxMem: 8 * bootstrapMiB, MaxDisk: 2000 * bootstrapMiB, CPU: cpu}}, client, prior, nil)
							if len(vms) != 1 {
								t.Fatal("node poll lost guest")
							}
							return vms[0]
						}
						vm, _, _, _, _, ok := m.buildVMFromClusterResource(context.Background(), "budget", proxmox.ClusterResource{VMID: 105, Node: "node", Type: "qemu", Name: "budget-guest", Status: "running", MaxMem: 8 * bootstrapMiB, MaxDisk: 2000 * bootstrapMiB, CPU: cpu}, client, id, nil, previous)
						if !ok {
							t.Fatal("cluster poll lost guest")
						}
						return vm
					}
					before := poll(nil)
					f.recovered.Store(true)
					entry := m.guestMetadataCache[key]
					entry.fetchedAt = time.Now().Add(-2 * guestMetadataCacheTTL)
					m.guestMetadataCache[key] = entry
					m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
					after := poll(&before)
					for _, vm := range []models.VM{before, after} {
						if len(vm.Disks) != 2 || vm.Disk.Used != int64(500*bootstrapMiB) || vm.DiskStatusReason != "" || vm.GuestAgentStatus != "available" || len(vm.IPAddresses) != 1 || vm.IPAddresses[0] != "192.0.2.20" {
							t.Errorf("independent ordinary readings were lost: %+v", vm)
						}
					}
					if before.CPU != .1 || after.CPU != .2 || before.Memory.Used != int64(3*bootstrapMiB) || after.Memory.Used != int64(4*bootstrapMiB) {
						t.Error("independent CPU/memory stopped advancing")
					}
					if seed == "suppressed" {
						if f.osCommands.Load() != 0 || after.OSName != original.osName || !m.guestMetadataCache[key].osInfoSkip {
							t.Error("genuine suppression was weakened")
						}
					} else if f.osCommands.Load() != 1 || after.OSName != "Windows" || after.OSVersion != "2025" || m.guestMetadataCache[key].osInfoFailureCount != 0 || !m.guestMetadataCache[key].windowsGuest {
						t.Error("ended budgets permanently disabled ordinary OS recovery")
					}
					if os.Getenv("PULSE_TEST_ENDED_BUDGET_PROJECTION") == "1" && kind == "client" && seed == "partial-failure" && collector == "cluster" {
						data, err := json.Marshal(map[string]models.VM{"before": before, "after": after})
						if err != nil {
							t.Fatal(err)
						}
						t.Logf("ENDED_BUDGET_PROJECTION %s", data)
					}
				})
			}
		}
	}
}

func TestGuestMetadataStartedOSOutcomeSurvivesBudgetEnd(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, rejected := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/rejected-%t", kind, rejected), func(t *testing.T) {
				f := &metadataBudgetFixture{}
				f.rejectOS.Store(rejected)
				client := &endedBudgetMetadataClient{PVEClientInterface: f.client(t, kind)}
				m, key, original := metadataBudgetMonitor(t, "partial-failure")
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				client.afterOS = cancel
				_, _, _, _, _, paused := m.fetchGuestAgentMetadata(ctx, client, "budget", "node", "guest", 105, &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}, false)
				entry := m.guestMetadataCache[key]
				if paused || client.network != 1 || client.os != 1 || client.version != 0 || f.commands.Load() != 2 {
					t.Error("budget end admitted a later callback or lost a completed outcome")
				}
				if rejected {
					if !entry.osInfoSkip || entry.osInfoFailureCount != guestAgentOSInfoFailureThreshold || entry.osName != original.osName {
						t.Error("completed rejection was reclassified as an unstarted OS command")
					}
				} else if entry.osInfoSkip || entry.osInfoFailureCount != 0 || entry.osName != "Windows" || !entry.windowsGuest || entry.osInfoObservedAt.IsZero() {
					t.Error("completed OS observation was discarded")
				}
			})
		}
	}
}
