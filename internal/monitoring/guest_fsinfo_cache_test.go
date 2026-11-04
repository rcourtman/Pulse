package monitoring

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

const fsInfoFixtureInstance = "fsinfo"

var fsInfoFixtureResource = proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "guest", VMID: 105, Status: "running", MaxMem: 8000, Mem: 2000, MaxDisk: 2000}

type fsInfoPollClient struct {
	stubPVEClient
	fsCalls   int
	used      uint64
	empty     bool
	fsErr     error
	lock      string
	agentOff  bool
	statusErr error
}

func (c *fsInfoPollClient) GetVMStatus(context.Context, string, int) (*proxmox.VMStatus, error) {
	if c.statusErr != nil {
		return nil, c.statusErr
	}
	status := &proxmox.VMStatus{Status: "running", MaxMem: 8000, Mem: 2000, Lock: c.lock, Agent: proxmox.VMAgentField{Value: 1}}
	if c.agentOff {
		status.Agent = proxmox.VMAgentField{Enabled: 1, AvailabilityKnown: true}
	}
	return status, nil
}

func (c *fsInfoPollClient) GetVMFSInfo(context.Context, string, int) ([]proxmox.VMFileSystem, error) {
	c.fsCalls++
	if c.fsErr != nil {
		return nil, c.fsErr
	}
	if c.empty {
		return nil, nil
	}
	return []proxmox.VMFileSystem{{Mountpoint: "/", Type: "ext4", TotalBytes: 1000, UsedBytes: c.used, Disk: "sda1"}}, nil
}

func newFSInfoPollMonitor() *Monitor {
	return &Monitor{rateTracker: NewRateTracker(), guestAgentFSInfoCacheTTL: vmFSInfoCacheTTL, guestAgentFSInfoTimeout: time.Second}
}

func fsInfoFixtureKey() string {
	return guestMemoryCacheKey(fsInfoFixtureInstance, fsInfoFixtureResource.Node, fsInfoFixtureResource.VMID)
}

// Moves the read into the past. Only the reply's age changes; the previous VM
// passed to the next poll keeps whatever the poll recorded.
func ageVMFSInfoRead(t *testing.T, m *Monitor, by time.Duration) {
	t.Helper()
	entry, ok := m.vmFSInfoCache[fsInfoFixtureKey()]
	if !ok {
		t.Fatal("no cached filesystem reply to age")
	}
	entry.fetchedAt = entry.fetchedAt.Add(-by)
	m.vmFSInfoCache[fsInfoFixtureKey()] = entry
}

// replicationFixtureHost is for a real client that reads replication jobs. The
// client records them for the life of the process under its address, and a later
// test's server can get the same port. No other test's server.URL matches this name.
func replicationFixtureHost(server *httptest.Server) string {
	return strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
}

func pollFSInfoGuest(t *testing.T, m *Monitor, client PVEClientInterface, prev *models.VM) models.VM {
	t.Helper()
	vm, _, _, _, _, ok := m.buildVMFromClusterResource(context.Background(), fsInfoFixtureInstance, fsInfoFixtureResource, client, makeGuestID(fsInfoFixtureInstance, "node", 105), nil, prev)
	if !ok {
		t.Fatal("guest disappeared")
	}
	return vm
}

func TestGuestFSInfoReuseStaysDistinctFromAFreshRead(t *testing.T) {
	client := &fsInfoPollClient{used: 400}
	m := newFSInfoPollMonitor()

	before := time.Now()
	fresh := pollFSInfoGuest(t, m, client, nil)
	if client.fsCalls != 1 || fresh.DiskStatusReason != "" || fresh.Disk.Used != 400 {
		t.Fatalf("first poll: calls=%d %+v", client.fsCalls, fresh)
	}
	readAt := fresh.DiskObservation.ObservedAt
	if readAt.Before(before) || fresh.DiskObservation.Source != "guest-agent" || fresh.GuestAgentEvidence.ObservedAt != readAt {
		t.Fatalf("a fresh read must date its own observation: %+v", fresh)
	}

	// The guest changed, but no command goes out while the reply is young.
	client.used = 700
	reused := pollFSInfoGuest(t, m, client, &fresh)
	if client.fsCalls != 1 {
		t.Fatalf("reuse sent a guest command: %d", client.fsCalls)
	}
	if reused.DiskStatusReason != "" || reused.Disk.Used != 400 || len(reused.Disks) != 1 || reused.Disks[0].Used != 400 {
		t.Fatalf("reused reading is still current: %+v", reused)
	}
	if !reused.DiskObservation.ObservedAt.Equal(readAt) || !reused.GuestAgentEvidence.ObservedAt.Equal(readAt) {
		t.Fatalf("reuse renewed the read time: %+v / %+v, read at %v", reused.DiskObservation, reused.GuestAgentEvidence, readAt)
	}
	reused.Disks[0].Used = 1
	again := pollFSInfoGuest(t, m, client, &reused)
	if again.Disks[0].Used != 400 {
		t.Fatal("a published row aliased the cached reply")
	}

	ageVMFSInfoRead(t, m, vmFSInfoCacheTTL)
	refreshed := pollFSInfoGuest(t, m, client, &again)
	if client.fsCalls != 2 || refreshed.Disk.Used != 700 || !refreshed.DiskObservation.ObservedAt.After(readAt) {
		t.Fatalf("expired reply was not read again: calls=%d %+v", client.fsCalls, refreshed)
	}
}

// The default poll is 10 seconds. Timers wander, and the poll interval is
// configurable, so the read must not fall to every other poll at a minute.
func TestVMFSInfoCacheTTLGivesAMinuteCadence(t *testing.T) {
	for _, tc := range []struct {
		name      string
		poll      time.Duration
		jitter    time.Duration
		wantEvery int
	}{
		{"10s poll", 10 * time.Second, 0, 6},
		{"10s poll running early", 10 * time.Second, -500 * time.Millisecond, 6},
		{"10s poll running late", 10 * time.Second, 500 * time.Millisecond, 6},
		{"15s poll", 15 * time.Second, 0, 4},
		{"60s poll running early", 60 * time.Second, -2 * time.Second, 1},
		{"60s poll", 60 * time.Second, 0, 1},
		{"5m poll", 5 * time.Minute, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newFSInfoPollMonitor()
			now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
			var lastRead int
			for poll := 0; poll < 100; poll++ {
				now = now.Add(tc.poll + tc.jitter)
				if _, hit := m.cachedVMFSInfo("i", "n", 1, 0, now); hit {
					continue
				}
				if poll > 0 && poll-lastRead != tc.wantEvery {
					t.Fatalf("read at poll %d, %d after the last, want every %d", poll, poll-lastRead, tc.wantEvery)
				}
				m.storeVMFSInfo("i", "n", 1, 0, nil, now)
				lastRead = poll
			}
		})
	}
}

func TestGuestFSInfoDisabledWithoutATTL(t *testing.T) {
	client := &fsInfoPollClient{used: 400}
	m := &Monitor{rateTracker: NewRateTracker(), guestAgentFSInfoTimeout: time.Second}
	first := pollFSInfoGuest(t, m, client, nil)
	second := pollFSInfoGuest(t, m, client, &first)
	if client.fsCalls != 2 || len(m.vmFSInfoCache) != 0 || !second.DiskObservation.ObservedAt.After(first.DiskObservation.ObservedAt) {
		t.Fatalf("calls=%d cache=%d", client.fsCalls, len(m.vmFSInfoCache))
	}
}

func TestNewMonitorReusesGuestFilesystemReplies(t *testing.T) {
	configureAlertShutdownFixture(t)
	monitor, err := New(&config.Config{DataPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	client := &fsInfoPollClient{used: 400}
	first := pollFSInfoGuest(t, monitor, client, nil)
	pollFSInfoGuest(t, monitor, client, &first)
	if client.fsCalls != 1 {
		t.Fatalf("a production monitor sent %d commands in two polls", client.fsCalls)
	}
}

// Asking again on every poll for a reply Pulse can't use would only add commands.
func TestGuestFSInfoCompletedUnusableReplyIsReused(t *testing.T) {
	client := &unusableFSInfoClient{}
	m := newFSInfoPollMonitor()
	first := pollFSInfoGuest(t, m, client, nil)
	second := pollFSInfoGuest(t, m, client, &first)
	if client.calls != 1 || second.DiskStatusReason != "agent-error" || !second.DiskObservation.ObservedAt.IsZero() {
		t.Fatalf("calls=%d %+v", client.calls, second)
	}
}

type unusableFSInfoClient struct {
	fsInfoPollClient
	calls int
}

func (c *unusableFSInfoClient) GetVMFSInfo(context.Context, string, int) ([]proxmox.VMFileSystem, error) {
	c.calls++
	return []proxmox.VMFileSystem{{Mountpoint: "/", Type: "ext4", TotalBytes: 10, UsedBytes: 20}}, nil
}

func TestGuestFSInfoCancelledReadIsNotStored(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &cancellingFSInfoClient{cancel: cancel}
	m := newFSInfoPollMonitor()
	_, _, _, _, disks, fromAgent, _, read := m.updateVMDisksFromGuestAgentFSInfo(ctx, fsInfoFixtureInstance, fsInfoFixtureResource, client, 2000, 0, 0)
	if len(disks) != 1 || !fromAgent || read.reused {
		t.Fatalf("the completed read itself still counts: disks=%d fromAgent=%v read=%+v", len(disks), fromAgent, read)
	}
	if _, stored := m.vmFSInfoCache[fsInfoFixtureKey()]; stored {
		t.Fatal("an obsolete collection repopulated the cache")
	}
}

type cancellingFSInfoClient struct {
	fsInfoPollClient
	cancel context.CancelFunc
}

func (c *cancellingFSInfoClient) GetVMFSInfo(context.Context, string, int) ([]proxmox.VMFileSystem, error) {
	c.cancel()
	return []proxmox.VMFileSystem{{Mountpoint: "/", Type: "ext4", TotalBytes: 1000, UsedBytes: 400, Disk: "sda1"}}, nil
}

func TestGuestFSInfoCompletedEmptyReplyIsReused(t *testing.T) {
	client := &fsInfoPollClient{empty: true}
	m := newFSInfoPollMonitor()
	first := pollFSInfoGuest(t, m, client, nil)
	second := pollFSInfoGuest(t, m, client, &first)
	if client.fsCalls != 1 || second.DiskStatusReason != "no-filesystems" || second.Disk.Usage != -1 || !second.DiskObservation.ObservedAt.IsZero() {
		t.Fatalf("calls=%d %+v", client.fsCalls, second)
	}

	ageVMFSInfoRead(t, m, vmFSInfoCacheTTL)
	client.empty, client.used = false, 400
	recovered := pollFSInfoGuest(t, m, client, &second)
	if client.fsCalls != 2 || recovered.DiskStatusReason != "" || recovered.Disk.Used != 400 {
		t.Fatalf("calls=%d %+v", client.fsCalls, recovered)
	}
}

func TestGuestFSInfoFailedReadIsRetriedAndKeepsTheLastReadTime(t *testing.T) {
	client := &fsInfoPollClient{used: 400}
	m := newFSInfoPollMonitor()
	fresh := pollFSInfoGuest(t, m, client, nil)
	reused := pollFSInfoGuest(t, m, client, &fresh)
	readAt := fresh.DiskObservation.ObservedAt

	ageVMFSInfoRead(t, m, vmFSInfoCacheTTL)
	client.fsErr = errors.New("agent exploded")
	failed := pollFSInfoGuest(t, m, client, &reused)
	if client.fsCalls != 2 || failed.DiskStatusReason != "prev-agent-error" || failed.Disk.Used != 400 || !failed.DiskObservation.ObservedAt.Equal(readAt) {
		t.Fatalf("retention must date from the real read: calls=%d %+v, read at %v", client.fsCalls, failed, readAt)
	}
	again := pollFSInfoGuest(t, m, client, &failed)
	if client.fsCalls != 3 || again.DiskStatusReason != "prev-agent-error" {
		t.Fatalf("a failure was cached: calls=%d %+v", client.fsCalls, again)
	}

	// Retention still ends ten minutes after the read, whoever reused it since.
	expired := reused
	expired.DiskObservation.ObservedAt = time.Now().Add(-recentGuestAgentEvidenceMaxAge - time.Second)
	gone := pollFSInfoGuest(t, m, client, &expired)
	if gone.DiskStatusReason != "agent-error" || gone.Disk.Usage != -1 {
		t.Fatalf("reuse stretched retention: %+v", gone)
	}
}

func TestGuestFSInfoAroundABackupLock(t *testing.T) {
	t.Run("short lock", func(t *testing.T) {
		client := &fsInfoPollClient{used: 400}
		m := newFSInfoPollMonitor()
		fresh := pollFSInfoGuest(t, m, client, nil)
		readAt := fresh.DiskObservation.ObservedAt

		client.lock = "backup"
		locked := pollFSInfoGuest(t, m, client, &fresh)
		if client.fsCalls != 1 || locked.DiskStatusReason != "prev-vm-locked" || locked.GuestAgentStatus != "deferred" || !locked.DiskObservation.ObservedAt.Equal(readAt) {
			t.Fatalf("a lock must outrank a young reply: calls=%d %+v", client.fsCalls, locked)
		}

		client.lock = ""
		resumed := pollFSInfoGuest(t, m, client, &locked)
		if client.fsCalls != 1 || resumed.DiskStatusReason != "" || resumed.Disk.Used != 400 || !resumed.DiskObservation.ObservedAt.Equal(readAt) {
			t.Fatalf("resumption inside the window: calls=%d %+v", client.fsCalls, resumed)
		}
	})

	t.Run("lock outlasting the reply", func(t *testing.T) {
		client := &fsInfoPollClient{used: 400}
		m := newFSInfoPollMonitor()
		fresh := pollFSInfoGuest(t, m, client, nil)
		readAt := fresh.DiskObservation.ObservedAt

		client.lock, client.used = "backup", 700
		ageVMFSInfoRead(t, m, vmFSInfoCacheTTL)
		locked := pollFSInfoGuest(t, m, client, &fresh)
		if client.fsCalls != 1 || locked.DiskStatusReason != "prev-vm-locked" || locked.Disk.Used != 400 || !locked.DiskObservation.ObservedAt.Equal(readAt) {
			t.Fatalf("expiry queued a command during the lock: calls=%d %+v", client.fsCalls, locked)
		}

		client.lock = ""
		resumed := pollFSInfoGuest(t, m, client, &locked)
		if client.fsCalls != 2 || resumed.DiskStatusReason != "" || resumed.Disk.Used != 700 || !resumed.DiskObservation.ObservedAt.After(readAt) {
			t.Fatalf("first poll after the lock must read: calls=%d %+v", client.fsCalls, resumed)
		}
	})
}

// A real client, so the replication check itself is in the path: a reuse costs
// neither the guest command nor the protected status read in front of it.
func TestGuestFSInfoAroundReplication(t *testing.T) {
	const statusNone = `{"data":[]}`
	var status atomic.Value
	status.Store(statusNone)
	var agentCalls, statusCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, `{"data":{}}`)
		case strings.HasSuffix(r.URL.Path, "/cluster/replication"):
			fmt.Fprint(w, `{"data":[{"id":"105-0","guest":105}]}`)
		case strings.HasSuffix(r.URL.Path, "/nodes/node/replication"):
			statusCalls.Add(1)
			fmt.Fprint(w, status.Load().(string))
		case strings.HasSuffix(r.URL.Path, "/agent/get-fsinfo"):
			agentCalls.Add(1)
			fmt.Fprint(w, `{"data":{"result":[{"mountpoint":"/","type":"ext4","total-bytes":1000,"used-bytes":400,"name":"sda1"}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: replicationFixtureHost(server), TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: guestPollFixtureHangGuard})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetReplicationStatus(context.Background()); err != nil {
		t.Fatal(err)
	}
	m := newFSInfoPollMonitor()
	read := func() string {
		t.Helper()
		_, _, _, _, _, _, reason, _ := m.updateVMDisksFromGuestAgentFSInfo(context.Background(), fsInfoFixtureInstance, fsInfoFixtureResource, client, 2000, 0, 0)
		return reason
	}

	if got := read(); got != "" || agentCalls.Load() != 1 || statusCalls.Load() != 1 {
		t.Fatalf("first read: reason=%q agent=%d status=%d", got, agentCalls.Load(), statusCalls.Load())
	}

	status.Store(fmt.Sprintf(`{"data":[{"id":"105-0","guest":105,"pid":4242,"last_try":%d}]}`, time.Now().Unix()))
	if got := read(); got != "" || agentCalls.Load() != 1 || statusCalls.Load() != 1 {
		t.Fatalf("reuse during a run: reason=%q agent=%d status=%d", got, agentCalls.Load(), statusCalls.Load())
	}

	ageVMFSInfoRead(t, m, vmFSInfoCacheTTL)
	if got := read(); got != "replication-active" || agentCalls.Load() != 1 {
		t.Fatalf("expired reply during a run: reason=%q agent=%d", got, agentCalls.Load())
	}

	status.Store(statusNone)
	if got := read(); got != "" || agentCalls.Load() != 2 {
		t.Fatalf("first read after the run: reason=%q agent=%d", got, agentCalls.Load())
	}
}

func TestGuestFSInfoEndsWithTheGuestAgentLifecycle(t *testing.T) {
	t.Run("stopped guest", func(t *testing.T) {
		client := &fsInfoPollClient{used: 400}
		m := newFSInfoPollMonitor()
		fresh := pollFSInfoGuest(t, m, client, nil)
		stopped := fsInfoFixtureResource
		stopped.Status = "stopped"
		if _, _, _, _, _, ok := m.buildVMFromClusterResource(context.Background(), fsInfoFixtureInstance, stopped, client, makeGuestID(fsInfoFixtureInstance, "node", 105), nil, &fresh); !ok {
			t.Fatal("guest disappeared")
		}
		if _, cached := m.vmFSInfoCache[fsInfoFixtureKey()]; cached {
			t.Fatal("a stopped guest kept its filesystem reply")
		}
		client.used = 700
		if restarted := pollFSInfoGuest(t, m, client, &fresh); client.fsCalls != 2 || restarted.Disk.Used != 700 {
			t.Fatalf("restart reused the old boot's reply: calls=%d %+v", client.fsCalls, restarted)
		}
	})

	t.Run("agent stopped", func(t *testing.T) {
		client := &fsInfoPollClient{used: 400}
		m := newFSInfoPollMonitor()
		fresh := pollFSInfoGuest(t, m, client, nil)
		client.agentOff = true
		down := pollFSInfoGuest(t, m, client, &fresh)
		if client.fsCalls != 1 || down.DiskStatusReason != "prev-agent-not-running" {
			t.Fatalf("calls=%d %+v", client.fsCalls, down)
		}
		client.agentOff, client.used = false, 700
		if back := pollFSInfoGuest(t, m, client, &down); client.fsCalls != 2 || back.Disk.Used != 700 || back.DiskStatusReason != "" {
			t.Fatalf("a returning agent reused its last reply: calls=%d %+v", client.fsCalls, back)
		}
	})

	t.Run("restart between polls", func(t *testing.T) {
		client := &fsInfoPollClient{used: 400}
		m := newFSInfoPollMonitor()
		res := fsInfoFixtureResource
		id := makeGuestID(fsInfoFixtureInstance, "node", 105)
		poll := func(res proxmox.ClusterResource, prev *models.VM) models.VM {
			t.Helper()
			vm, _, _, _, _, ok := m.buildVMFromClusterResource(context.Background(), fsInfoFixtureInstance, res, client, id, nil, prev)
			if !ok {
				t.Fatal("guest disappeared")
			}
			return vm
		}
		res.Uptime = 5
		fresh := poll(res, nil)
		client.used = 700
		res.Uptime = 15
		reused := poll(res, &fresh)
		if client.fsCalls != 1 || reused.Disk.Used != 400 {
			t.Fatalf("rising uptime must not read: calls=%d %+v", client.fsCalls, reused)
		}

		// 8 is above the uptime at the read, but below the 15 last seen.
		res.Uptime = 8
		client.fsErr = errors.New("agent still starting")
		failed := poll(res, &reused)
		if client.fsCalls != 2 || failed.DiskStatusReason != "prev-agent-error" {
			t.Fatalf("restart reused the old boot's reply: calls=%d %+v", client.fsCalls, failed)
		}
		client.fsErr = nil
		res.Uptime = 18
		if back := poll(res, &failed); client.fsCalls != 3 || back.Disk.Used != 700 {
			t.Fatalf("the old boot's reply came back: calls=%d %+v", client.fsCalls, back)
		}
	})

	t.Run("listing without uptime", func(t *testing.T) {
		client := &fsInfoPollClient{used: 400}
		m := newFSInfoPollMonitor()
		fresh := pollFSInfoGuest(t, m, client, nil)
		if reused := pollFSInfoGuest(t, m, client, &fresh); client.fsCalls != 1 || reused.Disk.Used != 400 {
			t.Fatalf("zero uptime must not force a read every poll: calls=%d", client.fsCalls)
		}
	})

	t.Run("agent stopped behind a lock", func(t *testing.T) {
		client := &fsInfoPollClient{used: 400}
		m := newFSInfoPollMonitor()
		fresh := pollFSInfoGuest(t, m, client, nil)
		client.lock, client.agentOff = "backup", true
		locked := pollFSInfoGuest(t, m, client, &fresh)
		client.lock, client.agentOff, client.used = "", false, 700
		if back := pollFSInfoGuest(t, m, client, &locked); client.fsCalls != 2 || back.Disk.Used != 700 {
			t.Fatalf("calls=%d %+v", client.fsCalls, back)
		}
	})

	t.Run("migrated guest", func(t *testing.T) {
		client := &fsInfoPollClient{used: 400}
		m := newFSInfoPollMonitor()
		fresh := pollFSInfoGuest(t, m, client, nil)
		moved := fsInfoFixtureResource
		moved.Node = "other"
		client.used = 700
		vm, _, _, _, _, ok := m.buildVMFromClusterResource(context.Background(), fsInfoFixtureInstance, moved, client, makeGuestID(fsInfoFixtureInstance, "other", 105), nil, &fresh)
		if !ok || client.fsCalls != 2 || vm.Disk.Used != 700 {
			t.Fatalf("calls=%d %+v", client.fsCalls, vm)
		}
	})

	t.Run("retired instance and idle cleanup", func(t *testing.T) {
		m := newFSInfoPollMonitor()
		now := time.Now()
		m.storeVMFSInfo("retired", "node", 1, 0, nil, now)
		m.storeVMFSInfo("retired/kept", "node", 2, 0, nil, now)
		m.storeVMFSInfo("idle", "node", 3, 0, nil, now.Add(-vmAgentMemCleanupMaxAge-time.Second))
		m.retireGuestMemoryCache("retired")
		if _, ok := m.cachedVMFSInfo("retired", "node", 1, 0, now); ok {
			t.Fatal("retired connection kept a reply")
		}
		if _, ok := m.cachedVMFSInfo("retired/kept", "node", 2, 0, now); !ok {
			t.Fatal("a similarly named connection lost its reply")
		}
		m.cleanupRRDCache(now)
		if _, ok := m.vmFSInfoCache[guestMemoryCacheKey("idle", "node", 3)]; ok {
			t.Fatal("cleanup kept a deleted guest's reply")
		}
		if _, ok := m.vmFSInfoCache[guestMemoryCacheKey("retired/kept", "node", 2)]; !ok {
			t.Fatal("cleanup removed a young reply")
		}
	})
}

// Without a status reply, only a real answer may report the agent as available.
// Reused strings already follow this rule through the metadata cache.
func TestGuestFSInfoReuseDoesNotProveTheAgentIsAvailable(t *testing.T) {
	client := &fsInfoPollClient{used: 400, statusErr: errors.New("status unavailable")}
	m := newFSInfoPollMonitor()
	prev := models.VM{Type: "qemu", GuestAgentEvidence: models.GuestAgentEvidence{Explicit: true, ObservedAt: time.Now().Add(-time.Minute)}}

	fresh := pollFSInfoGuest(t, m, client, &prev)
	if client.fsCalls != 1 || fresh.GuestAgentStatus != "available" {
		t.Fatalf("fresh read: calls=%d status=%q", client.fsCalls, fresh.GuestAgentStatus)
	}
	reused := pollFSInfoGuest(t, m, client, &fresh)
	if client.fsCalls != 1 || reused.GuestAgentStatus == "available" || reused.DiskStatusReason != "" {
		t.Fatalf("reuse: calls=%d status=%q reason=%q", client.fsCalls, reused.GuestAgentStatus, reused.DiskStatusReason)
	}
	if !reused.GuestAgentEvidence.ObservedAt.Equal(fresh.DiskObservation.ObservedAt) {
		t.Fatalf("reuse renewed query eligibility: %v vs %v", reused.GuestAgentEvidence.ObservedAt, fresh.DiskObservation.ObservedAt)
	}
}

// The drawer breaks a series wherever another series in its panel has a reading
// and it does not, so a current reading must keep a point per poll or the disk
// line falls apart into dots. A retained one stays a gap.
func TestGuestFSInfoHistoryKeepsAPointPerPollAndDeferralsAsGaps(t *testing.T) {
	client := &fsInfoPollClient{used: 400}
	m := newFSInfoPollMonitor()
	m.metricsHistory = NewMetricsHistory(100, time.Hour)
	id := makeGuestID(fsInfoFixtureInstance, "node", 105)
	poll := func(prev *models.VM) models.VM {
		t.Helper()
		cycleStart := time.Now()
		vm := pollFSInfoGuest(t, m, client, prev)
		m.recordGuestMetrics([]models.VM{vm}, nil, cycleStart)
		return vm
	}
	points := func() int { return len(m.metricsHistory.GetGuestMetrics(id, "disk", time.Hour)) }

	fresh := poll(nil)
	reused := poll(&fresh)
	if client.fsCalls != 1 || points() != 2 {
		t.Fatalf("calls=%d points=%d", client.fsCalls, points())
	}
	client.lock = "backup"
	locked := poll(&reused)
	if points() != 2 {
		t.Fatalf("a retained reading was written as new telemetry: %d points", points())
	}
	client.lock = ""
	poll(&locked)
	if client.fsCalls != 1 || points() != 3 {
		t.Fatalf("resumed reuse: calls=%d points=%d", client.fsCalls, points())
	}
}

func TestGuestFSInfoReuseKeepsDiskAlertsEvaluated(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)
	alertConfig := manager.GetConfig()
	alertConfig.Enabled = true
	alertConfig.GuestDefaults.Disk = &alerts.HysteresisThreshold{Trigger: 90, Clear: 85}
	alertConfig.MetricTimeThresholds = map[string]map[string]int{"guest": {"disk": 0}}
	manager.UpdateConfig(alertConfig)
	diskAlerts := func() int {
		count := 0
		for _, alert := range manager.GetActiveAlerts() {
			if alert.Type == "disk" {
				count++
			}
		}
		return count
	}

	client := &fsInfoPollClient{used: 950}
	m := newFSInfoPollMonitor()
	fresh := pollFSInfoGuest(t, m, client, nil)
	manager.CheckGuest(fresh, fresh.Instance)
	if diskAlerts() == 0 {
		t.Fatal("fresh reading did not alert")
	}
	raised := diskAlerts()

	client.used = 100
	reused := pollFSInfoGuest(t, m, client, &fresh)
	manager.CheckGuest(reused, reused.Instance)
	if client.fsCalls != 1 || diskAlerts() != raised {
		t.Fatalf("reuse changed the alert: calls=%d alerts=%d, want %d", client.fsCalls, diskAlerts(), raised)
	}

	ageVMFSInfoRead(t, m, vmFSInfoCacheTTL)
	refreshed := pollFSInfoGuest(t, m, client, &reused)
	manager.CheckGuest(refreshed, refreshed.Instance)
	if client.fsCalls != 2 || diskAlerts() != 0 {
		t.Fatalf("the next read did not clear the alert: calls=%d alerts=%d", client.fsCalls, diskAlerts())
	}
}
