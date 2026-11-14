package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Keep requested guests/tags fixed while growing only details that snapshot
// discovery never consumes. The registry and its typed views are prepared
// before measurement. No recovery manager, database or guest command is used.
func snapshotGuestReadFixture(guests, detail int) *Monitor {
	var resources []unifiedresources.Resource
	for i := 0; i < guests+3; i++ {
		kind, containerType := unifiedresources.ResourceTypeVM, ""
		if i%2 == 1 {
			kind, containerType = unifiedresources.ResourceTypeSystemContainer, "lxc"
		}
		instance, node, vmid := "site", "node-a", 100+i
		if i%2 == 1 {
			node = "node-b"
		}
		if i == guests+2 {
			instance, node, vmid = "peer", "node-a", 100
		}
		tags := []string{"production"}
		if i < 2 {
			tags = append(tags, "maintenance")
		}
		r := unifiedresources.Resource{ID: fmt.Sprintf("%s-%d", instance, vmid), Type: kind,
			Name: fmt.Sprintf("guest-%d", i), Status: unifiedresources.StatusOnline, Tags: tags,
			Proxmox: &unifiedresources.ProxmoxData{Instance: instance, NodeName: node, VMID: vmid,
				ContainerType: containerType, Template: i >= guests && i < guests+2}}
		for j := 0; j < detail; j++ {
			r.Proxmox.Disks = append(r.Proxmox.Disks, unifiedresources.DiskInfo{Device: fmt.Sprintf("disk-%d", j), Mountpoint: fmt.Sprintf("/data/%d", j), Total: 1024, Used: 512})
			r.Identity.IPAddresses = append(r.Identity.IPAddresses, fmt.Sprintf("192.0.2.%d", j+1))
			r.Proxmox.NetworkInterfaces = append(r.Proxmox.NetworkInterfaces,
				unifiedresources.NetworkInterface{Name: fmt.Sprintf("eth%d", j), Addresses: []string{"192.0.2.1", "2001:db8::1"}})
		}
		resources = append(resources, r)
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestResources(resources)
	adapter := unifiedresources.NewMonitorAdapter(registry)
	adapter.VMs()
	adapter.Containers()
	return &Monitor{state: models.NewState(), resourceStore: adapter}
}

func TestSnapshotGuestReadKeepsTargetsNamesTagsAndEvidence(t *testing.T) {
	var sparseProjection []byte
	for _, detail := range []int{0, 64} {
		t.Run(strconv.Itoa(detail), func(t *testing.T) {
			m := snapshotGuestReadFixture(4, detail)
			am := alerts.NewManagerWithDataDir(t.TempDir())
			t.Cleanup(am.Stop)
			cfg := am.GetConfig()
			cfg.Enabled, cfg.ActivationState = true, alerts.ActivationActive
			cfg.SnapshotDefaults = alerts.SnapshotAlertConfig{Enabled: true, WarningDays: 7, CriticalDays: 14}
			cfg.GuestTagBlacklist = []string{"maintenance"}
			cfg.TimeThresholds = map[string]int{}
			am.UpdateConfig(cfg)
			m.alertManager = am
			peer := models.GuestSnapshot{ID: "peer-retained", Instance: "peer", Node: "node-a", VMID: 100, Name: "independent", Time: time.Unix(1690000000, 0)}
			m.state.UpdateGuestSnapshotsForInstance("peer", []models.GuestSnapshot{peer})
			var mu sync.Mutex
			calls := map[string]int{}
			client := &snapshotPoolClient{read: func(_ context.Context, kind string, vmid int) ([]proxmox.Snapshot, error) {
				mu.Lock()
				calls[fmt.Sprintf("%s/%d", kind, vmid)]++
				mu.Unlock()
				return []proxmox.Snapshot{{Name: "weekly", SnapTime: 1700000000, Description: "accepted listing", Parent: "base"}}, nil
			}}
			backupPollingCompleteRead(t, m, func(m *Monitor) { m.pollGuestSnapshots(context.Background(), "site", client) })
			wantCalls := map[string]int{"qemu/100": 1, "lxc/101": 1, "qemu/102": 1, "lxc/103": 1}
			if !reflect.DeepEqual(calls, wantCalls) {
				t.Fatalf("templates or peer changed eligible reads: %v", calls)
			}
			rows := m.state.GetPVEBackups().GuestSnapshots
			sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
			if len(rows) != 5 || !reflect.DeepEqual(rows[0], peer) {
				t.Fatalf("lost or changed another instance's evidence: %+v", rows)
			}
			for _, row := range rows[1:] {
				wantType, wantNode := "qemu", "node-a"
				if row.VMID%2 == 1 {
					wantType, wantNode = "lxc", "node-b"
				}
				want := models.GuestSnapshot{ID: fmt.Sprintf("site-%s-%d-weekly", wantNode, row.VMID), Instance: "site", Node: wantNode, VMID: row.VMID, Type: wantType,
					Name: "weekly", Time: time.Unix(1700000000, 0), Description: "accepted listing", Parent: "base", VMState: wantType == "qemu"}
				if !reflect.DeepEqual(row, want) {
					t.Fatalf("snapshot projection changed: got %+v, want %+v", row, want)
				}
			}
			names := map[string]string{}
			for _, alert := range am.GetActiveAlerts() {
				names[alert.ResourceID] = alert.ResourceName
			}
			if !reflect.DeepEqual(names, map[string]string{"site:node-a:102": "guest-2", "site:node-b:103": "guest-3"}) {
				t.Fatalf("snapshot alerts lost scoped names or ignored tags: %v", names)
			}
			projection, err := json.Marshal(struct {
				Rows  []models.GuestSnapshot
				Calls map[string]int
				Names map[string]string
			}{rows, calls, names})
			if err != nil {
				t.Fatal(err)
			}
			if detail == 0 {
				sparseProjection = projection
			} else if !reflect.DeepEqual(projection, sparseProjection) {
				t.Fatal("unused guest details changed snapshot/alert results")
			}
			if os.Getenv("PULSE_SNAPSHOT_GUEST_READ_EXPORT") == "1" {
				t.Logf("SNAPSHOT_GUEST_READ=%s", projection)
			}
		})
	}
}

func TestSnapshotGuestReadDoesNotCopyUnusedDetails(t *testing.T) {
	// Process-wide allocation counts must exclude other tests' background work.
	const childEnv = "PULSE_SNAPSHOT_GUEST_READ_CHILD"
	if os.Getenv(childEnv) != t.Name() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$", "-test.count=1", "-test.v", "-test.timeout=20s",
			"-test.cpu="+strconv.Itoa(runtime.GOMAXPROCS(0)))
		cmd.Env = append(os.Environ(), childEnv+"="+t.Name())
		output, err := cmd.CombinedOutput()
		t.Logf("isolated snapshot allocation check:\n%s", output)
		if err != nil || !strings.Contains(string(output), "--- PASS: "+t.Name()+" ") || !strings.Contains(string(output), "unused-detail copy positive control:") {
			t.Fatalf("isolated allocation guard/control did not pass: %v", err)
		}
		return
	}
	previous := log.Logger
	log.Logger = zerolog.Nop()
	defer func() { log.Logger = previous }()
	call := func(m *Monitor) { m.pollGuestSnapshots(context.Background(), "site", backupPollingUnreadClient{}) }
	sparse, detailed := snapshotGuestReadFixture(16, 0), snapshotGuestReadFixture(16, 64)
	base, grown, peak := backupPollingAllocationFloors(t, sparse, detailed, call)
	if grown > base+4 || peak > base+100 {
		t.Fatalf("unused guest details increased real poll allocations: sparse=%.0f detailed=%.0f peak=%.0f", base, grown, peak)
	}
	base, grown, _ = backupPollingAllocationFloors(t, sparse, detailed, func(m *Monitor) {
		call(m)
		for _, vm := range m.resourceStore.(unifiedresources.ReadState).VMs() {
			backupPollingReadResult = vmFromReadStateView(vm)
		}
	})
	t.Logf("unused-detail copy positive control: %.0f -> %.0f", base, grown)
	if grown <= base+500 {
		t.Fatal("fixture did not expose full guest conversions in the real callback")
	}
}

func BenchmarkSnapshotGuestReadCost(b *testing.B) {
	previous := log.Logger
	log.Logger = zerolog.Nop()
	b.Cleanup(func() { log.Logger = previous })
	for _, detail := range []int{0, 64} {
		b.Run(fmt.Sprintf("guests-16/detail-%d", detail), func(b *testing.B) {
			m := snapshotGuestReadFixture(16, detail)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				backupPollingCompleteRead(b, m, func(m *Monitor) { m.pollGuestSnapshots(context.Background(), "site", backupPollingUnreadClient{}) })
			}
		})
	}
}
