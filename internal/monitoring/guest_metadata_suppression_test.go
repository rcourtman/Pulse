package monitoring

import (
	"context"
	"errors"
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
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type metadataSuppressionClient struct {
	mockPVEClient
	networkErr, osErr, versionErr       error
	os                                  map[string]interface{}
	networkCalls, osCalls, versionCalls int
}

func (c *metadataSuppressionClient) GetVMNetworkInterfaces(context.Context, string, int) ([]proxmox.VMNetworkInterface, error) {
	c.networkCalls++
	return []proxmox.VMNetworkInterface{{Name: "eth1", HardwareAddr: "02:00:00:00:00:02", IPAddresses: []proxmox.VMIPAddress{{Address: "192.0.2.11"}}}}, c.networkErr
}
func (c *metadataSuppressionClient) GetVMAgentInfo(context.Context, string, int) (map[string]interface{}, error) {
	c.osCalls++
	return c.os, c.osErr
}
func (c *metadataSuppressionClient) GetVMAgentVersion(context.Context, string, int) (string, error) {
	c.versionCalls++
	return "2.0", c.versionErr
}

func TestGuestMetadataOSInfoStateSurvivesVersionDeferral(t *testing.T) {
	for _, mode := range []string{"unsupported", "repeated-failure", "supported-reset"} {
		for _, seeded := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cached-%t", mode, seeded), func(t *testing.T) {
				const instance = "suppression"
				key := guestMetadataCacheKey(instance, "node", 105)
				m := &Monitor{guestMetadataLimiter: make(map[string]time.Time), guestMetadataRetryBackoff: 45 * time.Second}
				original := guestMetadataCacheEntry{}
				if seeded {
					original = metadataObservationFixture(time.Now().Add(-2 * guestMetadataCacheTTL))
					m.guestMetadataCache = map[string]guestMetadataCacheEntry{key: original}
				}
				client := &metadataSuppressionClient{versionErr: proxmox.ErrGuestAgentDeferred}
				rounds, wantFailures, wantSkip := 1, guestAgentOSInfoFailureThreshold, true
				switch mode {
				case "unsupported":
					client.osErr = errors.New("Failed to open file '/etc/os-release': No such file or directory")
				case "repeated-failure":
					rounds = guestAgentOSInfoFailureThreshold
					client.osErr = errors.New("OS info unavailable")
				case "supported-reset":
					original.osInfoFailureCount = guestAgentOSInfoFailureThreshold - 1
					m.guestMetadataCache = map[string]guestMetadataCacheEntry{key: original}
					client.os = map[string]interface{}{"name": "NewOS", "version": "2"}
					wantFailures, wantSkip = 0, false
				}
				status := &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}
				for round := 1; round <= rounds; round++ {
					m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
					start := time.Now()
					ips, ifaces, name, version, agent, deferred := m.fetchGuestAgentMetadata(context.Background(), client, instance, "node", "guest", 105, status, false)
					end := time.Now()
					if !deferred || !reflect.DeepEqual(ips, original.ipAddresses) || !reflect.DeepEqual(ifaces, original.networkInterfaces) || name != original.osName || version != original.osVersion || agent != original.agentVersion {
						t.Fatal("version deferral did not retain only last-known identity and the shared pause")
					}
					entry := m.guestMetadataCache[key]
					expected := original
					expected.osInfoFailureCount, expected.osInfoSkip = wantFailures, wantSkip
					if mode == "repeated-failure" {
						expected.osInfoFailureCount, expected.osInfoSkip = round, round >= guestAgentOSInfoFailureThreshold
					}
					if !reflect.DeepEqual(entry, expected) {
						t.Errorf("completed OS-info state lost or partial identity published: failures=%d skip=%t, want failures=%d skip=%t", entry.osInfoFailureCount, entry.osInfoSkip, expected.osInfoFailureCount, expected.osInfoSkip)
					}
					if m.hasRecentGuestMetadataEvidence(instance, "node", 105, end) {
						t.Error("version deferral renewed useful metadata evidence")
					}
					if next := m.guestMetadataLimiter[key]; next.Before(start.Add(45*time.Second)) || next.After(end.Add(45*time.Second)) {
						t.Error("version deferral changed retry backoff")
					}
					if len(ips) > 0 {
						ips[0], ifaces[0].Addresses[0] = "192.0.2.99", "192.0.2.99"
						if !reflect.DeepEqual(m.guestMetadataCache[key].networkInterfaces, original.networkInterfaces) {
							t.Fatal("returned identity aliases cache")
						}
					}
					before := client.networkCalls + client.osCalls + client.versionCalls
					m.fetchGuestAgentMetadata(context.Background(), client, instance, "node", "guest", 105, status, false)
					if after := client.networkCalls + client.osCalls + client.versionCalls; after != before {
						t.Error("shared retry backoff queued another command")
					}
				}
				client.versionErr = nil
				m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
				_, _, _, _, agent, deferred := m.fetchGuestAgentMetadata(context.Background(), client, instance, "node", "guest", 105, status, false)
				if deferred || agent != "2.0" || !m.hasRecentGuestMetadataEvidence(instance, "node", 105, time.Now()) {
					t.Error("ordinary healthy metadata did not resume")
				}
				wantOSCalls := rounds
				if !wantSkip {
					wantOSCalls++
				}
				if client.osCalls != wantOSCalls || client.networkCalls != rounds+1 || client.versionCalls != rounds+1 {
					t.Errorf("resumed command counts network/os/version=%d/%d/%d, want %d/%d/%d", client.networkCalls, client.osCalls, client.versionCalls, rounds+1, wantOSCalls, rounds+1)
				}
			})
		}
	}
}

func TestGuestMetadataUncompletedOSInfoDoesNotCreateSuppression(t *testing.T) {
	for _, phase := range []string{"network", "os"} {
		t.Run(phase, func(t *testing.T) {
			key := guestMetadataCacheKey("suppression", "node", 105)
			original := metadataObservationFixture(time.Now().Add(-2 * guestMetadataCacheTTL))
			original.osInfoFailureCount = guestAgentOSInfoFailureThreshold - 1
			m := &Monitor{guestMetadataCache: map[string]guestMetadataCacheEntry{key: original}, guestMetadataLimiter: make(map[string]time.Time)}
			client := &metadataSuppressionClient{}
			if phase == "network" {
				client.networkErr = proxmox.ErrGuestAgentDeferred
			} else {
				client.osErr = proxmox.ErrGuestAgentDeferred
			}
			_, _, _, _, _, deferred := m.fetchGuestAgentMetadata(context.Background(), client, "suppression", "node", "guest", 105, &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}, false)
			if !deferred || !reflect.DeepEqual(m.guestMetadataCache[key], original) || client.versionCalls != 0 {
				t.Fatal("uncompleted command changed OS-info suppression or queued version")
			}
			wantOSCalls := 0
			if phase == "os" {
				wantOSCalls = 1
			}
			if client.networkCalls != 1 || client.osCalls != wantOSCalls {
				t.Fatal("deferred command was replayed")
			}
		})
	}
}

// Actual HTTP decoding, both VM poll entry points and ordinary resumption.
// The fixture only changes its config reply; it never freezes a guest, runs a
// backup, shortens a QGA uncertainty fence or uses a native PVE installation.
func TestGuestMetadataOSInfoSuppressionHTTPPollLifecycle(t *testing.T) {
	for _, route := range []string{"cluster", "node"} {
		for _, seeded := range []bool{false, true} {
			for _, mode := range []string{"unsupported", "repeated-failure"} {
				t.Run(fmt.Sprintf("%s/cached-%t/%s", route, seeded, mode), func(t *testing.T) {
					const mib = uint64(1024 * 1024)
					var locked atomic.Bool
					var osCalls, versionCalls, networkCalls, fsCalls atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						switch {
						case strings.HasSuffix(r.URL.Path, "/config"):
							if locked.Load() {
								fmt.Fprint(w, `{"data":{"lock":"backup"}}`)
							} else {
								fmt.Fprint(w, `{"data":{}}`)
							}
						case strings.HasSuffix(r.URL.Path, "/status/current"):
							fmt.Fprint(w, `{"data":{"status":"running","agent":1,"maxmem":8388608,"mem":8388608,"meminfo":{"total":8388608,"available":5242880}}}`)
						case strings.HasSuffix(r.URL.Path, "/get-fsinfo"):
							fsCalls.Add(1)
							fmt.Fprintf(w, `{"data":{"result":[{"mountpoint":"C:\\","type":"ntfs","total-bytes":%d,"used-bytes":%d}]}}`, 1000*mib, 300*mib)
						case strings.HasSuffix(r.URL.Path, "/network-get-interfaces"):
							networkCalls.Add(1)
							fmt.Fprint(w, `{"data":{"result":[{"name":"eth1","hardware-address":"02:00:00:00:00:02","ip-addresses":[{"ip-address":"192.0.2.11","ip-address-type":"ipv4","prefix":24}]}]}}`)
						case strings.HasSuffix(r.URL.Path, "/get-osinfo"):
							osCalls.Add(1)
							locked.Store(true)
							if mode == "unsupported" {
								w.WriteHeader(http.StatusInternalServerError)
								fmt.Fprint(w, `{"data":null,"message":"VM 105 qmp command 'guest-get-osinfo' failed - guest agent command failed: Failed to open file '/etc/os-release': No such file or directory"}`)
							} else {
								w.WriteHeader(http.StatusForbidden)
								fmt.Fprint(w, `{"data":null,"message":"permission denied"}`)
							}
						case strings.HasSuffix(r.URL.Path, "/info"):
							versionCalls.Add(1)
							fmt.Fprint(w, `{"data":{"result":{"version":"2.0"}}}`)
						default:
							t.Errorf("unexpected request: %s", r.URL.Path)
							http.NotFound(w, r)
						}
					}))
					defer server.Close()
					client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
					if err != nil {
						t.Fatal(err)
					}
					m := &Monitor{config: &config.Config{}, rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour), guestMetadataLimiter: make(map[string]time.Time)}
					manager := alerts.NewManagerWithDataDir(t.TempDir())
					defer manager.Stop()
					m.alertManager = manager
					key := guestMetadataCacheKey("suppression", "node", 105)
					original := guestMetadataCacheEntry{}
					if seeded {
						original = metadataObservationFixture(time.Now().Add(-2 * guestMetadataCacheTTL))
						m.guestMetadataCache = map[string]guestMetadataCacheEntry{key: original}
					}
					resource := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "guest", VMID: 105, Status: "running", MaxMem: 8 * mib, Mem: 8 * mib, MaxDisk: 1000 * mib}
					build := func() models.VM {
						if route == "cluster" {
							vm, _, _, _, _, ok := m.buildVMFromClusterResource(context.Background(), "suppression", resource, client, makeGuestID("suppression", "node", 105), nil, nil)
							if !ok {
								t.Fatal("VM disappeared")
							}
							return vm
						}
						vms, _ := m.pollNodeVMsWithClusterResourceBuilder(context.Background(), "suppression", "node", []proxmox.VM{{VMID: 105, Name: "guest", Status: "running", MaxMem: 8 * mib, Mem: 8 * mib, MaxDisk: 1000 * mib}}, client, nil, nil)
						if len(vms) != 1 {
							t.Fatal("node VM disappeared")
						}
						return vms[0]
					}
					rounds := 1
					if mode == "repeated-failure" {
						rounds = guestAgentOSInfoFailureThreshold
					}
					for round := 1; round <= rounds; round++ {
						locked.Store(false)
						m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
						vm := build()
						if vm.GuestAgentStatus != "deferred" || vm.AgentVersion != original.agentVersion || !reflect.DeepEqual(vm.NetworkInterfaces, original.networkInterfaces) {
							t.Fatal("version lock did not retain shared pause and last-known identity")
						}
						entry := m.guestMetadataCache[key]
						expected := original
						expected.osInfoFailureCount, expected.osInfoSkip = guestAgentOSInfoFailureThreshold, true
						if mode == "repeated-failure" {
							expected.osInfoFailureCount, expected.osInfoSkip = round, round >= rounds
						}
						if !reflect.DeepEqual(entry, expected) {
							t.Errorf("OS-info outcome lost or partial readings renewed at round %d: count=%d skip=%t", round, entry.osInfoFailureCount, entry.osInfoSkip)
						}
						if versionCalls.Load() != 0 || networkCalls.Load() != int32(round) || osCalls.Load() != int32(round) || fsCalls.Load() != int32(round) {
							t.Fatal("failed poll sent wrong command counts or replayed a command")
						}
					}
					locked.Store(false)
					m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
					resumed := build()
					if resumed.GuestAgentStatus != "available" || resumed.AgentVersion != "2.0" || resumed.Disk.Used != int64(300*mib) || len(resumed.Disks) != 1 || len(resumed.NetworkInterfaces) != 1 || resumed.NetworkInterfaces[0].Name != "eth1" {
						t.Errorf("normal poll did not resume useful disk/network/version without OS-info: status=%s version=%s disks=%d", resumed.GuestAgentStatus, resumed.AgentVersion, len(resumed.Disks))
					}
					if osCalls.Load() != int32(rounds) || versionCalls.Load() != 1 || networkCalls.Load() != int32(rounds+1) || fsCalls.Load() != int32(rounds+1) {
						t.Errorf("resumed wire counts fs/network/os/version=%d/%d/%d/%d", fsCalls.Load(), networkCalls.Load(), osCalls.Load(), versionCalls.Load())
					}
					if !m.guestMetadataCache[key].osInfoSkip || !m.hasRecentGuestMetadataEvidence("suppression", "node", 105, time.Now()) {
						t.Error("resumed metadata lost suppression or useful current evidence")
					}
					m.recordGuestMetrics([]models.VM{resumed}, nil, time.Now().Add(-time.Second))
					if got := len(m.metricsHistory.GetGuestMetrics(resumed.ID, "disk", time.Hour)); got != 1 {
						t.Errorf("fresh disk History points=%d, want one", got)
					}
				})
			}
		}
	}
}
