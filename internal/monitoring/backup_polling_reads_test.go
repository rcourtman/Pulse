package monitoring

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// These fakes only read synthetic provider inventory; they perform no guest
// commands, backups, database operations or writes to an external service.
type backupPollingUnreadClient struct{ PVEClientInterface }

var errBackupPollingUnread = errors.New("synthetic inventory read unavailable")

func (backupPollingUnreadClient) GetVMSnapshots(context.Context, string, int) ([]proxmox.Snapshot, error) {
	return nil, errBackupPollingUnread
}

func (backupPollingUnreadClient) GetStorage(context.Context, string) ([]proxmox.Storage, error) {
	return nil, nil
}

func backupPollingReadFixture(unrelated int) *Monitor {
	at := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	state := models.NewState()
	state.PVEBackups = models.PVEBackups{
		BackupTasks: []models.BackupTask{{ID: "running", Instance: "site", Node: "node", VMID: 100, Status: "running", StartTime: at}},
		StorageBackups: []models.StorageBackup{
			{ID: "archive", Instance: "site", Storage: "local", StorageNode: "node", Node: "node", VMID: 100, Time: at, Protected: true},
			{ID: "peer-archive", Instance: "peer", Storage: "local", StorageNode: "node", Node: "node", VMID: 100, Time: at},
		},
		GuestSnapshots: []models.GuestSnapshot{
			{ID: "site-node-100-old", Instance: "site", Node: "node", VMID: 100, Type: "qemu", Name: "old", Time: at, Description: "accepted", SizeBytes: 4096},
			{ID: "peer-node-100-old", Instance: "peer", Node: "node", VMID: 100, Type: "qemu", Name: "peer-old", Time: at},
		},
	}
	state.PBSBackups = []models.PBSBackup{{ID: "pbs", Instance: "pbs", Datastore: "archive", Namespace: "team", BackupType: "vm", VMID: "100", BackupTime: at,
		Protected: true, InProgress: true, WriteActivityObserved: true, WriteActive: true, CarriedOver: true,
		Files: []string{"index.json.blob"}, VerificationRaw: map[string]interface{}{"state": "ok"}, Owner: "fixture@pbs"}}
	// The separately cached aggregate must never supply producer evidence.
	state.Backups = models.Backups{PVE: models.PVEBackups{StorageBackups: []models.StorageBackup{{ID: "stale-aggregate"}}}, PBS: []models.PBSBackup{{ID: "stale-aggregate"}}}
	state.VMs = []models.VM{{ID: "site-node-100", Instance: "site", Node: "node", VMID: 100}}
	cluster := models.KubernetesCluster{ID: "unrelated"}
	for i := 0; i < unrelated; i++ {
		id := fmt.Sprint(i)
		state.VMs = append(state.VMs, models.VM{ID: "unrelated-" + id, Instance: "unrelated", VMID: i + 1000, Disks: []models.Disk{{Mountpoint: "/"}}})
		cluster.ConfigMaps = append(cluster.ConfigMaps, models.KubernetesConfigMap{UID: id, Labels: map[string]string{"fixture": "only"}, DataKeys: []string{"key"}})
	}
	if unrelated > 0 {
		state.KubernetesClusters = []models.KubernetesCluster{cluster}
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestResources([]unifiedresources.Resource{{ID: "canonical-site-100", Type: unifiedresources.ResourceTypeVM, Name: "guest", Status: unifiedresources.StatusOnline,
		Proxmox: &unifiedresources.ProxmoxData{Instance: "site", NodeName: "node", VMID: 100}}})
	return &Monitor{state: state, resourceStore: unifiedresources.NewMonitorAdapter(registry)}
}

var backupPollingReadResult any

func pollUnreadPVEStorage(m *Monitor) {
	m.pollStorageBackupsWithNodes(context.Background(), "site", backupPollingUnreadClient{}, []proxmox.Node{{Node: "node"}}, map[string]string{"node": "offline"})
}

func backupPollingReadCases() []struct {
	name string
	call func(*Monitor)
} {
	return []struct {
		name string
		call func(*Monitor)
	}{
		{"pbs-cache", func(m *Monitor) { backupPollingReadResult = m.buildPBSBackupCache("pbs") }},
		{"storage-unread", pollUnreadPVEStorage},
		{"snapshots-unread", func(m *Monitor) { m.pollGuestSnapshots(context.Background(), "site", backupPollingUnreadClient{}) }},
	}
}

func pveBackupCarryForwardFixture(peerRows int) *Monitor {
	m := backupPollingReadFixture(0)
	pve := &m.state.PVEBackups
	for i := range peerRows {
		id := strconv.Itoa(i)
		pve.BackupTasks = append(pve.BackupTasks, models.BackupTask{ID: "peer-task-" + id, Instance: "peer", Node: "node", VMID: 100})
		pve.StorageBackups = append(pve.StorageBackups, models.StorageBackup{ID: "peer-file-" + id, Instance: "peer", StorageNode: "node", VMID: 100})
		pve.GuestSnapshots = append(pve.GuestSnapshots, models.GuestSnapshot{ID: "peer-snapshot-" + id, Instance: "peer", Node: "node", VMID: 100})
	}
	return m
}

func TestBackupPollingUnreadReadsKeepPrimaryBackupEvidence(t *testing.T) {
	m := backupPollingReadFixture(10)
	before := m.state.GetPVEBackups()
	byID := func(rows []models.GuestSnapshot) map[string]models.GuestSnapshot {
		result := make(map[string]models.GuestSnapshot, len(rows))
		for _, row := range rows {
			result[row.ID] = row
		}
		return result
	}
	for _, c := range backupPollingReadCases()[1:] {
		c.call(m)
		after := m.state.GetPVEBackups()
		// Publication preserves per-instance rows but moves the replaced
		// instance after its peers. Compare evidence, not that incidental order.
		if !reflect.DeepEqual(after.BackupTasks, before.BackupTasks) ||
			!reflect.DeepEqual(after.StorageBackups, before.StorageBackups) ||
			!reflect.DeepEqual(byID(after.GuestSnapshots), byID(before.GuestSnapshots)) {
			t.Fatalf("%s changed primary evidence or another instance: got %+v, want %+v", c.name, after, before)
		}
	}
}

func TestBackupPollingPBSCacheKeepsIdentityAndDetachedEvidence(t *testing.T) {
	m := backupPollingReadFixture(10)
	original := m.state.PBSBackups[0]
	for _, change := range []struct{ instance, store, namespace, kind string }{
		{"peer", "archive", "team", "vm"},
		{"pbs", "other", "team", "vm"},
		{"pbs", "archive", "other", "vm"},
		{"pbs", "archive", "team", "ct"},
	} {
		row := original
		row.ID = fmt.Sprint(change)
		row.Instance, row.Datastore, row.Namespace, row.BackupType = change.instance, change.store, change.namespace, change.kind
		m.state.PBSBackups = append(m.state.PBSBackups, row)
	}
	before := m.state.GetSnapshot().PBSBackups
	cache := m.buildPBSBackupCache("pbs")
	if len(cache) != 4 {
		t.Fatalf("instance/namespace/type/datastore grouping changed: %+v", cache)
	}
	for key, group := range cache {
		if len(group.snapshots) != 1 || !group.latest.Equal(original.BackupTime) {
			t.Fatalf("unexpected group %v: %+v", key, group)
		}
		row := group.snapshots[0]
		if row.Instance != "pbs" || row.Datastore != key.datastore || row.Namespace != key.namespace || row.BackupType != key.backupType || row.VMID != key.backupID ||
			!row.Protected || !row.InProgress || !row.WriteActivityObserved || !row.WriteActive || !row.CarriedOver || row.Owner != original.Owner {
			t.Fatalf("group %v lost source identity or evidence: %+v", key, row)
		}
		row.Files[0] = "reader-only"
		row.VerificationRaw.(map[string]interface{})["state"] = "reader-only"
		group.snapshots[0] = models.PBSBackup{}
	}
	if after := m.state.GetSnapshot().PBSBackups; !reflect.DeepEqual(after, before) {
		t.Fatal("cache caller mutated primary backup evidence")
	}
}

func TestBackupPollingReadsDoNotCopyUnrelatedInventory(t *testing.T) {
	// AllocsPerRun counts process-wide allocations. The real snapshot
	// callback yields to worker goroutines, so other monitoring tests' work
	// must not be attributed to the size of this fixture. Keep this binary's
	// race instrumentation, and verify that the child actually ran the guard.
	const childEnv = "PULSE_BACKUP_POLLING_ALLOCATION_CHILD"
	if os.Getenv(childEnv) != t.Name() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0],
			"-test.run=^"+regexp.QuoteMeta(t.Name())+"$",
			"-test.count=1", "-test.v", "-test.timeout=20s",
			"-test.cpu="+strconv.Itoa(runtime.GOMAXPROCS(0)))
		cmd.Env = append(os.Environ(), childEnv+"="+t.Name())
		output, err := cmd.CombinedOutput()
		t.Logf("isolated backup allocation check:\n%s", output)
		if err != nil {
			t.Fatalf("isolated backup allocation check: %v", err)
		}
		if !strings.Contains(string(output), "--- PASS: "+t.Name()+" ") ||
			!strings.Contains(string(output), "full snapshot positive control:") {
			t.Fatal("isolated backup allocation check did not complete the guard and positive control")
		}
		return
	}
	// Formatting is not fleet-copy work. In race builds, sync.Pool deliberately
	// drops pooled log events/buffers at random. Disable only this isolated
	// child's logger; the real callbacks and their state/ingest work still run.
	previous := log.Logger
	log.Logger = zerolog.Nop()
	defer func() { log.Logger = previous }()

	// A copy of the state costs about 1.3 MB and 4000 objects per call with
	// the fixture's 1000 unrelated VMs and config maps. Over 200 race runs
	// under load the fixtures differed by at most 288 bytes and 4 objects.
	const everyCallBytes, everyCallObjects, someCallBytes = 4 << 10, 16, 16 << 10
	t.Logf("allocation child GOMAXPROCS=%d", runtime.GOMAXPROCS(0))
	for _, c := range backupPollingReadCases() {
		t.Run(c.name, func(t *testing.T) {
			small, large := backupPollingReadFixture(0), backupPollingReadFixture(1000)
			costs := backupPollingCallCosts(t, small, large, c.call)
			t.Logf("allocations per real callback: %s", costs)
			if costs.grewEveryCall(everyCallBytes, everyCallObjects) {
				t.Fatalf("backup callback copied unrelated fleet inventory: %s", costs)
			}
			// Do not let a clean minimum and median hide a copy on some calls.
			if costs.grewSomeCall(someCallBytes) {
				t.Fatalf("backup callback intermittently copied unrelated fleet inventory: %s", costs)
			}
		})
		t.Run(c.name+"-full-copy-control", func(t *testing.T) {
			small, large := backupPollingReadFixture(0), backupPollingReadFixture(1000)
			costs := backupPollingCallCosts(t, small, large, func(m *Monitor) {
				c.call(m)
				backupPollingReadResult = m.state.GetSnapshot()
			})
			t.Logf("real callback full-copy control: %s", costs)
			if !costs.grewEveryCall(everyCallBytes, everyCallObjects) || costs.large.objects[0] <= costs.small.objects[0]+1000 {
				t.Fatal("allocation guard did not detect a full-fleet copy in the real callback")
			}
		})
		t.Run(c.name+"-intermittent-copy-control", func(t *testing.T) {
			small, large := backupPollingReadFixture(0), backupPollingReadFixture(1000)
			calls := map[*Monitor]int{small: 0, large: 0}
			costs := backupPollingCallCosts(t, small, large, func(m *Monitor) {
				c.call(m)
				// Copy on one measured call per fixture: the minimum and
				// median stay clean, so only the peak bound can expose it.
				if calls[m]++; calls[m] == backupPollingCostWarmup+5 {
					backupPollingReadResult = m.state.GetSnapshot()
				}
			})
			t.Logf("real callback intermittent-copy control: %s", costs)
			if costs.grewEveryCall(everyCallBytes, everyCallObjects) || !costs.grewSomeCall(someCallBytes) {
				t.Fatal("intermittent-copy control must keep a clean minimum and median and fail the peak bound")
			}
		})
	}
	t.Run("storage-unread-peer-backups", func(t *testing.T) {
		small, large := pveBackupCarryForwardFixture(0), pveBackupCarryForwardFixture(2000)
		// An all-offline enumeration does not rewrite the live inventory.
		// Its cost must not grow with a different connection's backup rows.
		costs := backupPollingCallCosts(t, small, large, pollUnreadPVEStorage)
		t.Logf("allocation per storage read with 2000 peer rows per collection: %s", costs)
		if costs.grewEveryCall(everyCallBytes, everyCallObjects) || costs.grewSomeCall(someCallBytes) {
			t.Fatalf("storage poll copied a peer's backup inventory: %s", costs)
		}
	})
	t.Run("task-carry-forward-unread-node", func(t *testing.T) {
		for _, fixture := range []struct {
			name, instance, node string
		}{
			{name: "peer-connection", instance: "peer", node: "node"},
			{name: "healthy-local-node", instance: "site", node: "healthy"},
		} {
			t.Run(fixture.name, func(t *testing.T) {
				small, large := backupPollingReadFixture(0), backupPollingReadFixture(0)
				prior := large.state.PVEBackups.BackupTasks[0]
				for i := range 2000 {
					row := prior
					row.ID, row.Instance, row.Node = strconv.Itoa(i), fixture.instance, fixture.node
					large.state.PVEBackups.BackupTasks = append(large.state.PVEBackups.BackupTasks, row)
				}
				unread := []proxmox.BackupTaskHistoryNode{{Node: prior.Node, Reason: "collection_failed"}}
				costs := backupPollingCallCosts(t, small, large, func(m *Monitor) {
					backupPollingReadResult = m.retainUnreadBackupTasks("site", unread, nil, nil)
				})
				t.Logf("allocation per carry-forward with 2000 unrelated task rows: %s", costs)
				if costs.grewEveryCall(everyCallBytes, everyCallObjects) || costs.grewSomeCall(someCallBytes) {
					t.Fatalf("unread-node carry-forward copied unrelated task rows: %s", costs)
				}
			})
		}
	})
	t.Run("pbs-cache-peer-backups", func(t *testing.T) {
		small, large := backupPollingReadFixture(0), backupPollingReadFixture(0)
		peer := large.state.PBSBackups[0]
		peer.Instance = "peer"
		for i := range 2000 {
			peer.ID = strconv.Itoa(i)
			large.state.PBSBackups = append(large.state.PBSBackups, peer)
		}
		costs := backupPollingCallCosts(t, small, large, func(m *Monitor) {
			backupPollingReadResult = m.buildPBSBackupCache("pbs")
		})
		t.Logf("allocation per PBS cache build with 2000 peer rows: %s", costs)
		if costs.grewEveryCall(everyCallBytes, everyCallObjects) || costs.grewSomeCall(someCallBytes) {
			t.Fatalf("PBS poll copied a peer's backup inventory: %s", costs)
		}
	})
	small, large := backupPollingReadFixture(0), backupPollingReadFixture(1000)
	costs := backupPollingCallCosts(t, small, large,
		func(m *Monitor) { backupPollingReadResult = m.state.GetSnapshot() })
	t.Logf("full snapshot positive control: %s", costs)
	if costs.large.objects[0] <= costs.small.objects[0]+1000 {
		t.Fatal("fixture does not expose whole-fleet copying")
	}
}

// Wait for the real callback's own in-memory ingest work before ending each
// allocation sample. Returning from pollStorageBackupsWithNodes does not mean
// its worker has finished: without this boundary, a sample can count a different
// number of worker context/timer allocations or queue several calls together.
// The fixture has no recovery manager, so this never opens a database/store.
func backupPollingCompleteRead(t testing.TB, m *Monitor, call func(*Monitor)) {
	t.Helper()
	call(m)
	deadline := time.Now().Add(5 * time.Second)
	for {
		m.recoveryIngestMu.Lock()
		running, pending := m.recoveryIngestRunning, len(m.recoveryIngestPending)
		m.recoveryIngestMu.Unlock()
		if !running && pending == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("synthetic backup callback ingest did not finish")
		}
		runtime.Gosched()
	}
}

// backupPollingCallCost summarises the heap allocations of single complete
// calls on one fixture.
type backupPollingCallCost struct {
	bytes, objects [3]uint64 // min, median, max
}

func (c backupPollingCallCost) String() string {
	return fmt.Sprintf("bytes min=%d median=%d max=%d, objects min=%d median=%d max=%d",
		c.bytes[0], c.bytes[1], c.bytes[2], c.objects[0], c.objects[1], c.objects[2])
}

// backupPollingCostPair compares a fixture with its grown twin.
type backupPollingCostPair struct {
	small, large backupPollingCallCost
}

func (p backupPollingCostPair) String() string {
	return fmt.Sprintf("small %s; large %s", p.small, p.large)
}

// grewEveryCall reports a copy present on every call: it moves the minimum or
// median bytes, or the median object count.
func (p backupPollingCostPair) grewEveryCall(bytes, objects uint64) bool {
	return p.large.bytes[0] > p.small.bytes[0]+bytes || p.large.bytes[1] > p.small.bytes[1]+bytes ||
		p.large.objects[1] > p.small.objects[1]+objects
}

// grewSomeCall reports a copy on any single call.
func (p backupPollingCostPair) grewSomeCall(bytes uint64) bool {
	return p.large.bytes[2] > p.small.bytes[2]+bytes
}

const (
	backupPollingCostWarmup = 2
	backupPollingCostRounds = 31
)

// backupPollingCallCosts measures every single complete call separately,
// interleaving the fixtures, with one P, the collector paused and no memory
// limit. A callback's own allocations are fixed for a fixture; runtime and
// library caches only add to a call. Race builds, which CI runs, add the
// most: sync.Pool drops a quarter of the values it is given, so any fmt call
// can miss its printer cache. Object counts then spread over 60 or more per
// call, and averaging five calls (testing.AllocsPerRun) folded one noisy call
// into its whole sample until a +4 bound failed. The minimum and median of
// single calls, with bytes as well as objects, stay stable; a copy on every
// call moves both, and the maximum catches a copy on any one call.
func backupPollingCallCosts(t *testing.T, small, large *Monitor, call func(*Monitor)) backupPollingCostPair {
	t.Helper()
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))
	runtime.GC()
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	defer debug.SetMemoryLimit(debug.SetMemoryLimit(math.MaxInt64))
	var stats runtime.MemStats
	measure := func(m *Monitor) (bytes, objects uint64) {
		runtime.ReadMemStats(&stats)
		bytes, objects = stats.TotalAlloc, stats.Mallocs
		backupPollingCompleteRead(t, m, call)
		runtime.ReadMemStats(&stats)
		return stats.TotalAlloc - bytes, stats.Mallocs - objects
	}
	for i := 0; i < backupPollingCostWarmup; i++ {
		measure(small)
		measure(large)
	}
	var sb, so, lb, lo []uint64
	for round := 0; round < backupPollingCostRounds; round++ {
		for _, largeFirst := range []bool{round%2 == 1, round%2 == 0} {
			if largeFirst {
				b, o := measure(large)
				lb, lo = append(lb, b), append(lo, o)
			} else {
				b, o := measure(small)
				sb, so = append(sb, b), append(so, o)
			}
		}
	}
	summarise := func(counts []uint64) [3]uint64 {
		slices.Sort(counts)
		return [3]uint64{counts[0], counts[len(counts)/2], counts[len(counts)-1]}
	}
	return backupPollingCostPair{
		small: backupPollingCallCost{bytes: summarise(sb), objects: summarise(so)},
		large: backupPollingCallCost{bytes: summarise(lb), objects: summarise(lo)},
	}
}

// Fixed callback measurements keep provider operations, retained backup data
// and registry inputs the same while growing only unrelated fleet metadata.
// Exclude diagnostic formatting and include completed fixture ingest work,
// matching the allocation guard rather than timing queued callbacks.
func BenchmarkBackupPollingReadCost(b *testing.B) {
	previous := log.Logger
	log.Logger = zerolog.Nop()
	b.Cleanup(func() { log.Logger = previous })
	for _, c := range backupPollingReadCases() {
		for _, size := range []int{0, 1000} {
			b.Run(fmt.Sprintf("%s/unrelated-%d", c.name, size), func(b *testing.B) {
				m := backupPollingReadFixture(size)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					backupPollingCompleteRead(b, m, c.call)
				}
			})
		}
	}
}

func BenchmarkPVEBackupCarryForwardCost(b *testing.B) {
	previous := log.Logger
	log.Logger = zerolog.Nop()
	b.Cleanup(func() { log.Logger = previous })
	for _, c := range backupPollingReadCases()[1:] {
		for _, peerRows := range []int{0, 2000} {
			b.Run(fmt.Sprintf("%s/peer-rows-per-collection-%d", c.name, peerRows), func(b *testing.B) {
				m := pveBackupCarryForwardFixture(peerRows)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					backupPollingCompleteRead(b, m, c.call)
				}
			})
		}
	}
}
