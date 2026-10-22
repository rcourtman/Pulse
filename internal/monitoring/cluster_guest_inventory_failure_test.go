package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Test-owned HTTP listeners really become unreachable, then return at the same
// addresses. No guest, backup, QGA command or uncertainty fence is exercised.
func TestClusterGuestInventoryOutageRetainsAndRecovers(t *testing.T) {
	const gib = int64(1024 * 1024 * 1024)
	var deleted atomic.Bool
	var guestCommands atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/agent/"):
			guestCommands.Add(1)
			t.Errorf("inventory recovery sent guest work: %s", r.URL.Path)
			http.NotFound(w, r)
		case r.URL.Path == "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"member","status":"online"}]}`)
		case r.URL.Path == "/api2/json/cluster/resources":
			// Exercise the ordinary node fallback, not a parallel custom path.
			http.Error(w, "unsupported fixture endpoint", http.StatusNotImplemented)
		case r.URL.Path == "/api2/json/nodes/member/qemu":
			if deleted.Load() {
				fmt.Fprint(w, `{"data":[]}`)
			} else {
				fmt.Fprintf(w, `{"data":[{"vmid":105,"name":"recovered-vm","status":"running","cpu":0.3,"maxmem":%d,"mem":%d,"maxdisk":%d,"uptime":3600}]}`, 8*gib, 3*gib, 10*gib)
			}
		case r.URL.Path == "/api2/json/nodes/member/lxc":
			if deleted.Load() {
				fmt.Fprint(w, `{"data":[]}`)
			} else {
				fmt.Fprintf(w, `{"data":[{"vmid":206,"name":"recovered-container","status":"running","cpu":0.4,"maxmem":%d,"mem":%d,"maxdisk":%d,"disk":%d,"uptime":3600}]}`, 8*gib, 4*gib, 10*gib, 5*gib)
			}
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			fmt.Fprintf(w, `{"data":{"status":"running","agent":0,"cpu":0.3,"maxmem":%d,"mem":%d,"maxdisk":%d,"disk":%d,"uptime":3600,"meminfo":{"total":%d,"available":%d}}}`, 8*gib, 3*gib, 10*gib, 5*gib, 8*gib, 5*gib)
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, `{"data":{"agent":0}}`)
		case strings.HasSuffix(r.URL.Path, "/interfaces"), strings.HasSuffix(r.URL.Path, "/rrddata"):
			fmt.Fprint(w, `{"data":[]}`)
		default:
			http.NotFound(w, r)
		}
	})
	start := func(address string) (string, func()) {
		t.Helper()
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Fatal(err)
		}
		server := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second}
		done := make(chan error, 1)
		go func() { done <- server.Serve(listener) }()
		var once sync.Once
		stop := func() {
			once.Do(func() {
				if err := server.Close(); err != nil {
					t.Error(err)
				}
				if err := <-done; err != http.ErrServerClosed {
					t.Errorf("fixture server stopped unexpectedly: %v", err)
				}
			})
		}
		t.Cleanup(stop)
		return "http://" + listener.Addr().String(), stop
	}
	first, stopFirst := start("127.0.0.1:0")
	second, stopSecond := start("127.0.0.1:0")
	client := proxmox.NewClusterClient("inventory-fixture", proxmox.ClientConfig{Host: first, TokenName: "fixture@pve!read", TokenValue: "fixture", Timeout: 2 * time.Second}, []string{first, second}, nil)
	nodes, err := client.GetNodes(context.Background())
	if err != nil || len(nodes) != 1 {
		t.Fatalf("fixture node discovery failed: %v", err)
	}
	m := newTestPVEMonitor("inventory-fixture")
	t.Cleanup(m.alertManager.Stop)
	t.Cleanup(m.notificationMgr.Stop)
	m.resourceStore = unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	m.replacePVEClient("inventory-fixture", client)
	at := time.Now().Add(-2 * time.Minute).Truncate(time.Second)
	memory := models.Memory{Total: 8 * gib, Used: 2 * gib, Free: 6 * gib, Usage: 25, Observation: models.MemoryObservation{State: "current", Source: "proxmox-status", ObservedAt: at}}
	vm := models.VM{ID: "inventory-fixture:member:105", VMID: 105, Type: "qemu", Instance: "inventory-fixture", Node: "member", Name: "accepted-vm", Status: "running", CPU: 0.2, Memory: memory, Disk: models.Disk{Total: 10 * gib, Used: 4 * gib, Usage: 40}, LastSeen: at, Tags: []string{"accepted"}}
	ct := models.Container{ID: "inventory-fixture:member:206", VMID: 206, Type: "lxc", Instance: "inventory-fixture", Node: "member", Name: "accepted-container", Status: "running", CPU: 0.25, Memory: memory, Disk: models.Disk{Total: 10 * gib, Used: 3 * gib, Usage: 30}, LastSeen: at, Tags: []string{"accepted"}}
	other := models.VM{ID: "other:member:105", VMID: 105, Type: "qemu", Instance: "other", Node: "member", Name: "independent-vm", Status: "running", Memory: memory, LastSeen: at}
	// State reads normalize empty collections. Compare the same complete tuple
	// representation, rather than nil fixture slices against normalized clones.
	vm, ct, other = vm.NormalizeCollections(), ct.NormalizeCollections(), other.NormalizeCollections()
	m.state.UpdateGuestsForInstance("inventory-fixture", []models.VM{vm}, []models.Container{ct})
	m.state.UpdateGuestsForInstance("other", []models.VM{other}, nil)
	projections := make(map[string]any)
	project := func(label string) {
		t.Helper()
		m.updateResourceStore(m.currentStateWithScope())
		front := m.buildBroadcastFrontendStateFromSnapshot(m.GetState(), m.mockModeFence.begin())
		projections[label] = front.Resources
	}
	project("initial")
	stopFirst()
	stopSecond()
	cfg := &config.PVEInstance{MonitorVMs: true, MonitorContainers: true}
	poll := func() {
		t.Helper()
		if err := m.pollGuestsWithFallback(context.Background(), "inventory-fixture", cfg, client, nodes, map[string]string{"member": "online"}); err != nil {
			t.Fatal(err)
		}
	}
	poll()
	gotVMs, gotCTs := m.state.GuestObservationsForNodes("inventory-fixture", map[string]struct{}{"member": {}})
	if !reflect.DeepEqual(gotVMs, []models.VM{vm}) || !reflect.DeepEqual(gotCTs, []models.Container{ct}) {
		t.Fatalf("unavailable cluster replaced accepted tuples: vms=%+v containers=%+v", gotVMs, gotCTs)
	}
	for _, id := range []string{vm.ID, ct.ID} {
		if points := m.metricsHistory.GetGuestMetrics(id, "cpu", time.Hour); len(points) != 0 {
			t.Errorf("unavailable enumeration invented History for %s: %+v", id, points)
		}
	}
	project("retained")
	// Restore only the local fixture listeners, at their original addresses.
	start(strings.TrimPrefix(first, "http://"))
	start(strings.TrimPrefix(second, "http://"))
	// Respect the real endpoint recovery interval; do not alter health maps,
	// shorten a safety fence, replace the client, restart Pulse or poll in a loop.
	time.Sleep(11 * time.Second)
	poll()
	snapshot := m.GetState()
	var recovered *models.VM
	for i := range snapshot.VMs {
		if snapshot.VMs[i].ID == vm.ID {
			recovered = &snapshot.VMs[i]
		}
	}
	if recovered == nil || recovered.Name != "recovered-vm" || !recovered.LastSeen.After(at) || len(snapshot.Containers) != 1 || snapshot.Containers[0].Name != "recovered-container" || !snapshot.Containers[0].LastSeen.After(at) {
		t.Fatalf("ordinary restored enumeration did not advance the same guests: %+v", snapshot)
	}
	project("resumed")
	deleted.Store(true)
	poll()
	snapshot = m.GetState()
	if len(snapshot.VMs) != 1 || !reflect.DeepEqual(snapshot.VMs[0], other) || len(snapshot.Containers) != 0 || guestCommands.Load() != 0 {
		t.Fatal("confirmed deletion retained removed guests, changed independent scope or sent guest work")
	}
	project("deleted")
	wire, err := json.Marshal(projections)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("CLUSTER_INVENTORY_PROJECTIONS %s", wire)
}
