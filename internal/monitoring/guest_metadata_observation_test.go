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

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type metadataObservationClient struct {
	mockPVEClient
	err     error
	network []proxmox.VMNetworkInterface
	os      map[string]interface{}
	version string
	calls   int
}

func (c *metadataObservationClient) GetVMNetworkInterfaces(context.Context, string, int) ([]proxmox.VMNetworkInterface, error) {
	c.calls++
	return c.network, c.err
}

func (c *metadataObservationClient) GetVMAgentInfo(context.Context, string, int) (map[string]interface{}, error) {
	c.calls++
	return c.os, c.err
}

func (c *metadataObservationClient) GetVMAgentVersion(context.Context, string, int) (string, error) {
	c.calls++
	return c.version, c.err
}

func metadataObservationFixture(at time.Time) guestMetadataCacheEntry {
	return guestMetadataCacheEntry{
		ipAddresses:       []string{"192.0.2.10"},
		networkInterfaces: []models.GuestNetworkInterface{{Name: "eth0", MAC: "02:00:00:00:00:01", Addresses: []string{"192.0.2.10"}}},
		osName:            "Linux",
		osVersion:         "fixture",
		agentVersion:      "1.0",
		fetchedAt:         at,
	}
}

func TestGuestMetadataFailedRefreshPreservesObservation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		client metadataObservationClient
	}{
		{"ordinary-error", metadataObservationClient{err: errors.New("metadata unavailable")}},
		{"empty-success", metadataObservationClient{}},
		{"filtered-network", metadataObservationClient{network: []proxmox.VMNetworkInterface{{Name: "lo", IPAddresses: []proxmox.VMIPAddress{{Address: "127.0.0.1"}}}}}},
		{"unusable-os", metadataObservationClient{os: map[string]interface{}{"result": map[string]interface{}{"unrelated": "fixture"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := guestMetadataCacheKey("metadata", "node", 105)
			original := metadataObservationFixture(time.Now().Add(-2 * guestMetadataCacheTTL))
			m := &Monitor{guestMetadataCache: map[string]guestMetadataCacheEntry{key: original}, guestMetadataLimiter: make(map[string]time.Time), guestMetadataRetryBackoff: 45 * time.Second}
			status := &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}
			client := tc.client
			for attempt := 0; attempt < 2; attempt++ {
				m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
				start := time.Now()
				ips, ifaces, name, version, agent := m.fetchGuestAgentMetadata(context.Background(), &client, "metadata", "node", "guest", 105, status, false)
				end := time.Now()
				if !reflect.DeepEqual(ips, original.ipAddresses) || !reflect.DeepEqual(ifaces, original.networkInterfaces) || name != original.osName || version != original.osVersion || agent != original.agentVersion {
					t.Error("unsuccessful refresh lost last-known identity")
				}
				entry := m.guestMetadataCache[key]
				if !entry.fetchedAt.Equal(original.fetchedAt) || m.hasRecentGuestMetadataEvidence("metadata", "node", 105, end) {
					t.Error("unsuccessful refresh renewed old guest-agent evidence")
				}
				if next := m.guestMetadataLimiter[key]; next.Before(start.Add(45*time.Second)) || next.After(end.Add(45*time.Second)) {
					t.Error("unsuccessful refresh did not retain the configured retry backoff")
				}
				// Caller-owned slices cannot change the retained cache.
				ips[0] = "192.0.2.99"
				ifaces[0].Addresses[0] = "192.0.2.99"
				if !reflect.DeepEqual(m.guestMetadataCache[key].networkInterfaces, original.networkInterfaces) || !reflect.DeepEqual(m.guestMetadataCache[key].ipAddresses, original.ipAddresses) {
					t.Error("retained identity aliases returned values")
				}
				calls := client.calls
				m.fetchGuestAgentMetadata(context.Background(), &client, "metadata", "node", "guest", 105, status, false)
				if client.calls != calls {
					t.Error("backoff queued another metadata command")
				}
			}
			if client.calls != 6 {
				t.Errorf("metadata calls = %d, want exactly two three-command attempts", client.calls)
			}
		})
	}
}

func TestGuestMetadataUsefulRefreshRenewsOnlyObservedEvidence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		client metadataObservationClient
	}{
		{"network", metadataObservationClient{network: []proxmox.VMNetworkInterface{{Name: "eth1", IPAddresses: []proxmox.VMIPAddress{{Address: "192.0.2.11"}}}}}},
		{"os", metadataObservationClient{os: map[string]interface{}{"name": "NewOS", "version": "2"}}},
		{"agent-version", metadataObservationClient{version: "2.0"}},
		{"unchanged-but-observed", metadataObservationClient{version: "1.0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := guestMetadataCacheKey("metadata", "node", 105)
			original := metadataObservationFixture(time.Now().Add(-2 * guestMetadataCacheTTL))
			m := &Monitor{guestMetadataCache: map[string]guestMetadataCacheEntry{key: original}, guestMetadataLimiter: make(map[string]time.Time)}
			client := tc.client
			start := time.Now()
			m.fetchGuestAgentMetadata(context.Background(), &client, "metadata", "node", "guest", 105, &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}, false)
			end := time.Now()
			entry := m.guestMetadataCache[key]
			if entry.fetchedAt.Before(start) || entry.fetchedAt.After(end) || !m.hasRecentGuestMetadataEvidence("metadata", "node", 105, end) {
				t.Error("accepted useful metadata did not establish current evidence")
			}
			if tc.client.version != "" && entry.agentVersion != tc.client.version {
				t.Error("current agent version was lost")
			}
			if tc.client.os != nil && (entry.osName != "NewOS" || entry.osVersion != "2") {
				t.Error("current OS identity was lost")
			}
			if len(tc.client.network) > 0 && (len(entry.networkInterfaces) != 1 || entry.networkInterfaces[0].Name != "eth1") {
				t.Error("current network metadata was lost")
			}
			if len(tc.client.network) == 0 && !reflect.DeepEqual(entry.networkInterfaces, original.networkInterfaces) {
				t.Error("an independent useful field cleared retained network identity")
			}
		})
	}
}

// The real HTTP/client/metadata/builder path must not turn a complete failed or
// empty metadata refresh into recent guest-agent evidence. This is not native
// QGA execution, backup overlap or workload recovery.
func TestGuestMetadataObservationHTTPPollLifecycle(t *testing.T) {
	for _, mode := range []string{"refused", "empty", "unusable"} {
		t.Run(mode, func(t *testing.T) {
			const mib = uint64(1024 * 1024)
			var phase, calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				p := phase.Load()
				switch {
				case strings.HasSuffix(r.URL.Path, "/config"):
					fmt.Fprint(w, `{"data":{}}`)
				case strings.HasSuffix(r.URL.Path, "/status/current"):
					agent := `1`
					if p == 2 {
						agent = `{"enabled":1,"available":0}`
					}
					fmt.Fprintf(w, `{"data":{"status":"running","agent":%s,"maxmem":%d,"mem":%d,"meminfo":{"available":%d},"cpu":0.2}}`, agent, 8*mib, 3*mib, 5*mib)
				case strings.Contains(r.URL.Path, "/agent/"):
					calls.Add(1)
					if p == 1 {
						if mode == "refused" {
							http.Error(w, "permission denied", http.StatusForbidden)
							return
						}
						switch {
						case strings.HasSuffix(r.URL.Path, "network-get-interfaces"):
							fmt.Fprint(w, `{"data":{"result":[]}}`)
						case strings.HasSuffix(r.URL.Path, "get-osinfo") && mode == "unusable":
							fmt.Fprint(w, `{"data":{"unrelated":"fixture"}}`)
						case strings.HasSuffix(r.URL.Path, "get-osinfo"):
							fmt.Fprint(w, `{"data":{}}`)
						default:
							fmt.Fprint(w, `{"data":{"result":{}}}`)
						}
						return
					}
					ip, version := "192.0.2.10", "1.0"
					if p == 3 {
						ip, version = "192.0.2.11", "2.0"
					}
					switch {
					case strings.HasSuffix(r.URL.Path, "network-get-interfaces"):
						fmt.Fprintf(w, `{"data":{"result":[{"name":"eth0","ip-addresses":[{"ip-address":%q,"ip-address-type":"ipv4","prefix":24}]}]}}`, ip)
					case strings.HasSuffix(r.URL.Path, "get-osinfo"):
						fmt.Fprint(w, `{"data":{"name":"Linux","version":"fixture"}}`)
					case strings.HasSuffix(r.URL.Path, "info"):
						fmt.Fprintf(w, `{"data":{"result":{"version":%q}}}`, version)
					case strings.HasSuffix(r.URL.Path, "get-fsinfo"):
						fmt.Fprintf(w, `{"data":{"result":[{"mountpoint":"/","type":"ext4","total-bytes":%d,"used-bytes":%d}]}}`, 1000*mib, 300*mib)
					default:
						http.NotFound(w, r)
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
			m := &Monitor{config: &config.Config{}, rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(16, time.Hour), guestMetadataLimiter: make(map[string]time.Time)}
			res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "guest", VMID: 105, Status: "running", MaxMem: 8 * mib, Mem: 3 * mib, MaxDisk: 1000 * mib, CPU: 0.2}
			build := func() models.VM {
				vm, _, _, _, _, ok := m.buildVMFromClusterResource(context.Background(), "metadata", res, client, "metadata:node:105", nil, nil)
				if !ok {
					t.Fatal("guest disappeared")
				}
				return vm
			}
			first := build()
			if first.GuestAgentStatus != "available" || first.AgentVersion != "1.0" || calls.Load() != 4 {
				t.Fatalf("healthy fixture absent: status=%s version=%s calls=%d", first.GuestAgentStatus, first.AgentVersion, calls.Load())
			}
			key := guestMetadataCacheKey("metadata", "node", 105)
			original := m.guestMetadataCache[key]
			original.fetchedAt = time.Now().Add(-2 * guestMetadataCacheTTL)
			m.guestMetadataCache[key] = original
			m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
			phase.Store(1)
			ips, ifaces, name, version, agent := m.fetchGuestAgentMetadata(context.Background(), client, "metadata", "node", "guest", 105, &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}, false)
			if !reflect.DeepEqual(ips, first.IPAddresses) || !reflect.DeepEqual(ifaces, first.NetworkInterfaces) || name != first.OSName || version != first.OSVersion || agent != first.AgentVersion {
				t.Error("failed HTTP refresh lost last-known metadata")
			}
			if !m.guestMetadataCache[key].fetchedAt.Equal(original.fetchedAt) || m.hasRecentGuestMetadataEvidence("metadata", "node", 105, time.Now()) {
				t.Error("failed HTTP refresh became new evidence")
			}
			phase.Store(2)
			unavailable := build()
			if unavailable.GuestAgentStatus != "not-running" || unavailable.GuestAgentExpected {
				t.Errorf("retained identity changed guest-agent runtime classification: %s/%t", unavailable.GuestAgentStatus, unavailable.GuestAgentExpected)
			}
			if unavailable.CPU != first.CPU || unavailable.Memory.Used != first.Memory.Used || unavailable.ID != first.ID || calls.Load() != 7 {
				t.Error("independent status/identity was lost or unavailable agent was queried")
			}
			phase.Store(3)
			recovered := build()
			if recovered.GuestAgentStatus != "available" || recovered.AgentVersion != "2.0" || len(recovered.IPAddresses) != 1 || recovered.IPAddresses[0] != "192.0.2.11" || !m.hasRecentGuestMetadataEvidence("metadata", "node", 105, time.Now()) || calls.Load() != 11 {
				t.Error("healthy poll did not recover current metadata and evidence")
			}
		})
	}
}
