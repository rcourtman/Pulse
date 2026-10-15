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

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// The efficient route owns removal as well as continuity. Use real node and
// cluster HTTP reads and the ordinary publication owner, with no guest commands.
type efficientInventoryFixture struct {
	m         *Monitor
	client    *proxmox.Client
	cfg       *config.PVEInstance
	nodes     atomic.Value
	resources atomic.Value
	reads     atomic.Int32
	forbidden atomic.Int32
	vm        models.VM
	ct        models.Container
	other     models.VM
}

func newEfficientInventoryFixture(t *testing.T) *efficientInventoryFixture {
	t.Helper()
	const gib = int64(1 << 30)
	f := &efficientInventoryFixture{m: newTestPVEMonitor("efficient")}
	f.nodes.Store(`{"data":[{"node":"member","status":"online"}]}`)
	f.resources.Store(`{"data":[]}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api2/json/nodes":
			fmt.Fprint(w, f.nodes.Load().(string))
		case r.URL.Path == "/api2/json/cluster/resources" && r.URL.Query().Get("type") == "vm":
			f.reads.Add(1)
			fmt.Fprint(w, f.resources.Load().(string))
		case strings.Contains(r.URL.Path, "/agent/"), strings.HasSuffix(r.URL.Path, "/qemu"), strings.HasSuffix(r.URL.Path, "/lxc"):
			f.forbidden.Add(1)
			t.Errorf("efficient inventory dispatched a guest command or fallback list: %s", r.URL.Path)
			http.NotFound(w, r)
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			fmt.Fprintf(w, `{"data":{"status":"running","agent":0,"cpu":0.3,"maxmem":%d,"mem":%d,"maxdisk":%d,"disk":%d,"uptime":3600,"meminfo":{"total":%d,"available":%d}}}`, 8*gib, 3*gib, 10*gib, 5*gib, 8*gib, 5*gib)
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, `{"data":{"agent":0}}`)
		case strings.HasSuffix(r.URL.Path, "/interfaces"), strings.HasSuffix(r.URL.Path, "/rrddata"):
			fmt.Fprint(w, `{"data":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	var err error
	f.client, err = proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!read", TokenValue: "fixture", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	f.cfg = &config.PVEInstance{Name: "efficient", Host: server.URL, MonitorVMs: true, MonitorContainers: true}
	t.Cleanup(f.m.alertManager.Stop)
	t.Cleanup(f.m.notificationMgr.Stop)
	f.m.resourceStore = unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	f.m.replacePVEClient("efficient", f.client)
	at := time.Now().Add(-2 * time.Minute).Truncate(time.Second)
	memory := models.Memory{Total: 8 * gib, Used: 2 * gib, Free: 6 * gib, Usage: 25, Observation: models.MemoryObservation{State: "current", Source: "proxmox-status", ObservedAt: at}}
	f.vm = (models.VM{ID: "efficient:member:105", VMID: 105, Type: "qemu", Instance: "efficient", Node: "member", Name: "accepted-vm", Status: "running", CPU: 0.2, Memory: memory, Disk: models.Disk{Total: 10 * gib, Used: 4 * gib, Usage: 40}, LastSeen: at, Tags: []string{"accepted"}}).NormalizeCollections()
	f.ct = (models.Container{ID: "efficient:member:206", VMID: 206, Type: "lxc", Instance: "efficient", Node: "member", Name: "accepted-container", Status: "running", CPU: 0.25, Memory: memory, Disk: models.Disk{Total: 10 * gib, Used: 3 * gib, Usage: 30}, LastSeen: at, Tags: []string{"accepted"}}).NormalizeCollections()
	f.other = (models.VM{ID: "other:member:105", VMID: 105, Type: "qemu", Instance: "other", Node: "member", Name: "independent-vm", Status: "running", Memory: memory, LastSeen: at}).NormalizeCollections()
	f.m.state.UpdateGuestsForInstance("efficient", []models.VM{f.vm}, []models.Container{f.ct})
	f.m.state.UpdateGuestsForInstance("other", []models.VM{f.other}, nil)
	// Seed the same canonical membership the production poller's previous-state
	// read uses; an allocated but empty registry is not the accepted generation.
	f.m.updateResourceStore(f.m.currentStateWithScope())
	return f
}

func (f *efficientInventoryFixture) poll(t *testing.T, carried map[string]string) {
	t.Helper()
	nodes, err := f.client.GetNodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	effective := make(map[string]string)
	for node, status := range carried {
		effective[node] = status
	}
	for _, node := range nodes {
		_, effective[node.Node] = f.m.determineNodeIDAndStatus("efficient", f.cfg, node)
	}
	before := f.reads.Load()
	if err := f.m.pollGuestsWithFallback(context.Background(), "efficient", f.cfg, f.client, nodes, effective); err != nil {
		t.Fatal(err)
	}
	if f.reads.Load() != before+1 || f.forbidden.Load() != 0 {
		t.Fatal("ordinary efficient poll did not remain one inventory read without guest commands/fallback")
	}
}

func (f *efficientInventoryFixture) assertRetained(t *testing.T) {
	t.Helper()
	vms, cts := f.m.state.GuestObservationsForNodes("efficient", map[string]struct{}{"member": {}})
	if !reflect.DeepEqual(vms, []models.VM{f.vm}) || !reflect.DeepEqual(cts, []models.Container{f.ct}) {
		t.Fatalf("grace gap changed complete source tuples/times: vms=%+v containers=%+v", vms, cts)
	}
}

func (f *efficientInventoryFixture) assertNoFreshHistory(t *testing.T) {
	t.Helper()
	for _, id := range []string{f.vm.ID, f.ct.ID} {
		if points := f.m.metricsHistory.GetGuestMetrics(id, "cpu", time.Hour); len(points) != 0 {
			t.Errorf("unobserved/deleted guest gained new History: %s %+v", id, points)
		}
	}
}

func TestEfficientGuestInventoryEmptyAndGrace(t *testing.T) {
	for _, name := range []string{"online-empty", "offline-grace", "omitted-grace", "expired-offline", "never-online", "unknown-node-data", "online-template"} {
		t.Run(name, func(t *testing.T) {
			f := newEfficientInventoryFixture(t)
			// Seed the real node grace policy from an online observation. No
			// production grace/cooldown interval is changed for these controls.
			nodeID, _ := f.m.determineNodeIDAndStatus("efficient", f.cfg, proxmox.Node{Node: "member", Status: "online"})
			var carried map[string]string
			keep := false
			switch name {
			case "offline-grace":
				f.nodes.Store(`{"data":[{"node":"member","status":"offline"}]}`)
				keep = true
			case "omitted-grace":
				f.nodes.Store(`{"data":[]}`)
				carried = map[string]string{}
				for _, node := range f.m.preserveOrExpireNodes([]models.Node{{ID: nodeID, Name: "member", Instance: "efficient", Status: "online", LastSeen: time.Now()}}) {
					carried[node.Name] = node.Status
				}
				keep = true
			case "expired-offline", "never-online":
				f.nodes.Store(`{"data":[{"node":"member","status":"offline"}]}`)
				if name == "expired-offline" {
					f.m.nodeLastOnline[nodeID] = time.Now().Add(-2 * f.m.pveNodeOfflineGracePeriod())
				} else {
					delete(f.m.nodeLastOnline, nodeID)
				}
			case "unknown-node-data":
				f.nodes.Store(`{"data":[]}`)
				keep = true
			case "online-template":
				f.resources.Store(`{"data":[{"vmid":105,"type":"qemu","node":"member","template":1},{"vmid":206,"type":"lxc","node":"member","template":1}]}`)
			}
			f.poll(t, carried)
			if keep {
				f.assertRetained(t)
			} else {
				state := f.m.GetState()
				if len(state.VMs) != 1 || !reflect.DeepEqual(state.VMs[0], f.other) || len(state.Containers) != 0 {
					t.Fatal("accepted empty/template inventory retained removed guests or changed independent scope")
				}
			}
			f.assertNoFreshHistory(t)
		})
	}
}

func TestEfficientGuestInventoryRecoveryAndDeletion(t *testing.T) {
	f := newEfficientInventoryFixture(t)
	projections := make(map[string]any)
	project := func(phase string) {
		t.Helper()
		f.m.updateResourceStore(f.m.currentStateWithScope())
		projections[phase] = f.m.buildBroadcastFrontendStateFromSnapshot(f.m.GetState(), f.m.mockModeFence.begin()).Resources
	}
	project("initial")
	f.m.determineNodeIDAndStatus("efficient", f.cfg, proxmox.Node{Node: "member", Status: "online"})
	f.nodes.Store(`{"data":[{"node":"member","status":"offline"}]}`)
	f.poll(t, nil)
	f.assertRetained(t)
	f.assertNoFreshHistory(t)
	project("retained")

	f.nodes.Store(`{"data":[{"node":"member","status":"online"}]}`)
	f.resources.Store(`{"data":[{"vmid":105,"type":"qemu","node":"member","name":"recovered-vm","status":"running","cpu":0.3,"maxmem":8589934592,"mem":3221225472,"maxdisk":10737418240,"uptime":3600},{"vmid":206,"type":"lxc","node":"member","name":"recovered-container","status":"running","cpu":0.4,"maxmem":8589934592,"mem":4294967296,"maxdisk":10737418240,"disk":5368709120,"uptime":3600}]}`)
	f.poll(t, nil)
	vms, cts := f.m.state.GuestObservationsForNodes("efficient", map[string]struct{}{"member": {}})
	if len(vms) != 1 || len(cts) != 1 || vms[0].ID != f.vm.ID || cts[0].ID != f.ct.ID || vms[0].Name != "recovered-vm" || cts[0].Name != "recovered-container" || !vms[0].LastSeen.After(f.vm.LastSeen) || !cts[0].LastSeen.After(f.ct.LastSeen) {
		t.Fatal("ordinary recovery did not advance the same accepted identities")
	}
	for _, id := range []string{f.vm.ID, f.ct.ID} {
		if points := f.m.metricsHistory.GetGuestMetrics(id, "cpu", time.Hour); len(points) != 1 {
			t.Errorf("recovery did not record exactly one current CPU observation: %s %+v", id, points)
		}
	}
	project("resumed")
	f.resources.Store(`{"data":[]}`)
	f.poll(t, nil)
	state := f.m.GetState()
	if len(state.VMs) != 1 || !reflect.DeepEqual(state.VMs[0], f.other) || len(state.Containers) != 0 {
		t.Fatal("confirmed empty online inventory did not remove only its guests")
	}
	project("deleted")
	wire, err := json.Marshal(projections)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("EFFICIENT_INVENTORY_PROJECTIONS %s", wire)
}

func TestEfficientGuestInventoryMoveDoesNotResurrectOldCoordinates(t *testing.T) {
	for _, template := range []bool{false, true} {
		t.Run(fmt.Sprint("template=", template), func(t *testing.T) {
			f := newEfficientInventoryFixture(t)
			f.m.determineNodeIDAndStatus("efficient", f.cfg, proxmox.Node{Node: "member", Status: "online"})
			f.nodes.Store(`{"data":[{"node":"member","status":"offline"},{"node":"destination","status":"online"}]}`)
			flag := 0
			if template {
				flag = 1
			}
			f.resources.Store(fmt.Sprintf(`{"data":[{"vmid":105,"type":"qemu","node":"destination","name":"moved-vm","status":"running","template":%d},{"vmid":206,"type":"lxc","node":"destination","name":"moved-container","status":"running","template":%d}]}`, flag, flag))
			f.poll(t, nil)
			oldVMs, oldCTs := f.m.state.GuestObservationsForNodes("efficient", map[string]struct{}{"member": {}})
			if len(oldVMs) != 0 || len(oldCTs) != 0 {
				t.Fatal("grace retention resurrected an observed cluster-wide ID at obsolete coordinates")
			}
			newVMs, newCTs := f.m.state.GuestObservationsForNodes("efficient", map[string]struct{}{"destination": {}})
			want := 1
			if template {
				want = 0
			}
			if len(newVMs) != want || len(newCTs) != want {
				t.Fatal("migration/template control changed authoritative membership")
			}
		})
	}
}
