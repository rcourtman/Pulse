package monitoring

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

func TestCleanupTrackingMaps_PreservesActiveTypedCircuitBreakers(t *testing.T) {
	now := time.Now()
	stale := now.Add(-25 * time.Hour)

	activeKey := schedulerKey(InstanceTypePVE, "pve-active")
	inactiveKey := schedulerKey(InstanceTypePVE, "pve-removed")

	activeBreaker := newCircuitBreaker(3, time.Second, time.Minute, 30*time.Second)
	inactiveBreaker := newCircuitBreaker(3, time.Second, time.Minute, 30*time.Second)

	activeBreaker.lastTransition = stale
	inactiveBreaker.lastTransition = stale

	m := &Monitor{
		pveClients: map[string]PVEClientInterface{
			"pve-active": nil,
		},
		pbsClients:      make(map[string]*pbs.Client),
		pmgClients:      make(map[string]*pmg.Client),
		circuitBreakers: map[string]*circuitBreaker{activeKey: activeBreaker, inactiveKey: inactiveBreaker},
		failureCounts:   map[string]int{activeKey: 2, inactiveKey: 3},
		lastOutcome: map[string]taskOutcome{
			activeKey:   {recordedAt: stale},
			inactiveKey: {recordedAt: stale},
		},
	}

	m.cleanupTrackingMaps(now)

	if _, ok := m.circuitBreakers[activeKey]; !ok {
		t.Fatalf("active circuit breaker %q was incorrectly removed", activeKey)
	}
	if _, ok := m.failureCounts[activeKey]; !ok {
		t.Fatalf("active failure count %q was incorrectly removed", activeKey)
	}
	if _, ok := m.lastOutcome[activeKey]; !ok {
		t.Fatalf("active lastOutcome %q was incorrectly removed", activeKey)
	}

	if _, ok := m.circuitBreakers[inactiveKey]; ok {
		t.Fatalf("inactive stale circuit breaker %q was not removed", inactiveKey)
	}
	if _, ok := m.failureCounts[inactiveKey]; ok {
		t.Fatalf("inactive stale failure count %q was not removed", inactiveKey)
	}
	if _, ok := m.lastOutcome[inactiveKey]; ok {
		t.Fatalf("inactive stale lastOutcome %q was not removed", inactiveKey)
	}
}

type trackingCleanupCatalog struct {
	config.FileSystem
	reads      atomic.Int32
	beforeRead func()
	failRead   bool
}

func (fs *trackingCleanupCatalog) ReadFile(name string) ([]byte, error) {
	if filepath.Base(name) == "availability_targets.enc" {
		fs.reads.Add(1)
		if fs.beforeRead != nil {
			fs.beforeRead()
		}
		if fs.failRead {
			return nil, os.ErrPermission
		}
	}
	return os.ReadFile(name)
}

func trackingCleanupPersistence(t testing.TB, beforeRead func(), targets ...config.AvailabilityTarget) (*config.ConfigPersistence, *trackingCleanupCatalog) {
	t.Helper()
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), "no-legacy-key"))
	p := config.NewConfigPersistence(t.TempDir())
	require.NoError(t, p.SaveAvailabilityTargets(targets))
	catalog := &trackingCleanupCatalog{beforeRead: beforeRead}
	p.SetFileSystem(catalog)
	return p, catalog
}

func TestCleanupTrackingMapsOnlyListsProvidersWithExpiredBreakers(t *testing.T) {
	p, catalog := trackingCleanupPersistence(t, nil, config.AvailabilityTarget{
		ID: "retained", Name: "Fixture check", Enabled: true,
		Protocol: config.AvailabilityProbeHTTPS, Address: "https://fixture.invalid",
	})
	now := time.Now()
	retained := schedulerKey(InstanceTypeAvailability, "retained")
	removed := schedulerKey(InstanceTypeAvailability, "removed")
	peer := schedulerKey(InstanceTypePVE, "retained")
	breakerAt := func(age time.Duration) *circuitBreaker {
		b := newCircuitBreaker(3, time.Second, time.Minute, 30*time.Second)
		b.lastTransition = now.Add(-age)
		return b
	}
	m := &Monitor{
		configPersist:   p,
		pveClients:      map[string]PVEClientInterface{"retained": nil},
		circuitBreakers: map[string]*circuitBreaker{retained: breakerAt(24 * time.Hour), peer: breakerAt(25 * time.Hour)},
		lastAuthAttempt: map[string]time.Time{"expired": now.Add(-25 * time.Hour), "current": now},
	}

	// An aged PVE breaker and an availability breaker exactly on the
	// boundary cannot require a saved availability read.
	m.cleanupTrackingMaps(now)
	unneededReads := catalog.reads.Load()
	t.Logf("availability catalog reads without an expired availability breaker: %d", unneededReads)
	if unneededReads != 0 {
		t.Error("cleanup read the catalog without an expired availability breaker")
	}
	require.Contains(t, m.circuitBreakers, peer)
	require.Contains(t, m.circuitBreakers, retained)
	require.Equal(t, map[string]time.Time{"current": now}, m.lastAuthAttempt)

	m.circuitBreakers[retained] = breakerAt(25 * time.Hour)
	m.circuitBreakers[removed] = breakerAt(25 * time.Hour)
	m.failureCounts = map[string]int{retained: 2, removed: 3}
	staleResult := now.Add(-25 * time.Hour)
	m.lastOutcome = map[string]taskOutcome{retained: {recordedAt: staleResult}, removed: {recordedAt: staleResult}}
	m.cleanupTrackingMaps(now)
	neededReads := catalog.reads.Load() - unneededReads
	t.Logf("availability catalog reads for two expired availability breakers: %d", neededReads)
	require.EqualValues(t, 1, neededReads, "read the needed provider just once")
	require.Contains(t, m.circuitBreakers, retained, "an active check keeps its breaker")
	require.NotContains(t, m.circuitBreakers, removed)
	require.Equal(t, map[string]int{retained: 2}, m.failureCounts)
	require.Equal(t, map[string]taskOutcome{retained: {recordedAt: staleResult}}, m.lastOutcome)
}

func TestCleanupTrackingMapsPreservesBreakersWhenAvailabilityRosterUnreadable(t *testing.T) {
	p, catalog := trackingCleanupPersistence(t, nil, config.AvailabilityTarget{
		ID: "saved", Name: "Fixture check", Enabled: true,
		Protocol: config.AvailabilityProbeHTTPS, Address: "https://fixture.invalid",
	})
	now := time.Now()
	old := now.Add(-25 * time.Hour)
	breaker := func() *circuitBreaker {
		b := newCircuitBreaker(3, time.Second, time.Minute, 30*time.Second)
		b.lastTransition = old
		return b
	}
	saved := schedulerKey(InstanceTypeAvailability, "saved")
	removed := schedulerKey(InstanceTypeAvailability, "removed")
	peer := schedulerKey(InstanceTypePBS, "removed")
	savedBreaker, removedBreaker := breaker(), breaker()
	m := &Monitor{
		configPersist: p,
		circuitBreakers: map[string]*circuitBreaker{
			saved: savedBreaker, removed: removedBreaker, peer: breaker(),
		},
		failureCounts: map[string]int{saved: 2, removed: 3, peer: 1},
		lastOutcome:   map[string]taskOutcome{saved: {recordedAt: old}, removed: {recordedAt: old}, peer: {recordedAt: old}},
	}
	catalog.failRead = true
	m.cleanupTrackingMaps(now)
	require.EqualValues(t, 1, catalog.reads.Load())
	require.Equal(t, map[string]*circuitBreaker{saved: savedBreaker, removed: removedBreaker}, m.circuitBreakers,
		"an unavailable availability roster must not be treated as empty or stop other providers' cleanup")
	require.Equal(t, map[string]int{saved: 2, removed: 3}, m.failureCounts)
	require.Equal(t, map[string]taskOutcome{saved: {recordedAt: old}, removed: {recordedAt: old}}, m.lastOutcome)

	catalog.failRead = false
	m.cleanupTrackingMaps(now)
	require.EqualValues(t, 2, catalog.reads.Load())
	require.Equal(t, map[string]*circuitBreaker{saved: savedBreaker}, m.circuitBreakers)
	require.Equal(t, map[string]int{saved: 2}, m.failureCounts)
	require.Equal(t, map[string]taskOutcome{saved: {recordedAt: old}}, m.lastOutcome)
}

func TestCleanupTrackingMapsRechecksBreakerAfterProviderLookup(t *testing.T) {
	var started, release chan struct{}
	p, _ := trackingCleanupPersistence(t, func() {
		close(started)
		<-release
	})
	synctest.Test(t, func(t *testing.T) {
		started, release = make(chan struct{}), make(chan struct{})
		now := time.Now()
		oldBreaker := func() *circuitBreaker {
			b := newCircuitBreaker(3, time.Second, time.Minute, 30*time.Second)
			b.lastTransition = now.Add(-25 * time.Hour)
			return b
		}
		refreshed := schedulerKey(InstanceTypeAvailability, "refreshed")
		replaced := schedulerKey(InstanceTypeAvailability, "replaced")
		removed := schedulerKey(InstanceTypeAvailability, "removed")
		m := &Monitor{
			configPersist:   p,
			circuitBreakers: map[string]*circuitBreaker{refreshed: oldBreaker(), replaced: oldBreaker(), removed: oldBreaker()},
			pollStatusMap:   make(map[string]*pollStatus),
			lastOutcome:     make(map[string]taskOutcome),
		}
		go m.cleanupTrackingMaps(now)
		<-started
		// A closed breaker can receive a success without changing its
		// transition time. Don't discard that result, or a replacement,
		// based on a snapshot taken before the provider read.
		m.recordTaskResult(InstanceTypeAvailability, "refreshed", nil)
		replacement := oldBreaker()
		m.mu.Lock()
		m.circuitBreakers[replaced] = replacement
		m.mu.Unlock()
		close(release)
		synctest.Wait()
		require.Contains(t, m.circuitBreakers, refreshed)
		require.True(t, m.lastOutcome[refreshed].success)
		require.Same(t, replacement, m.circuitBreakers[replaced])
		require.NotContains(t, m.circuitBreakers, removed)
	})
}

const trackingCleanupNewPeer = "fixture-late-pve"

func observeUnneededTrackingCatalogRead(t *testing.T, recentAvailabilityOutcome bool, observe func(*testing.T, *Monitor, int32, time.Time)) {
	t.Helper()
	previousMock := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	defer mustSetMockEnabled(t, previousMock)

	var hold atomic.Bool
	var release chan struct{}
	p, catalog := trackingCleanupPersistence(t, func() {
		if hold.CompareAndSwap(true, false) {
			<-release
		}
	})
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		release = make(chan struct{})
		defer func() {
			cancel()
			close(release)
		}()
		m := newScheduleFixture(ctx, false, func(context.Context) ([]proxmox.Node, error) {
			return []proxmox.Node{{Node: scheduleFixtureNode, Status: "online", CPU: 0.24, Uptime: 36000}}, nil
		})
		m.configPersist = p
		breaker := newCircuitBreaker(3, time.Second, time.Minute, 30*time.Second)
		breaker.lastTransition = time.Now().Add(-25 * time.Hour)
		m.circuitBreakers = map[string]*circuitBreaker{schedulerKey(InstanceTypePVE, scheduleFixtureNode): breaker}
		go m.Start(ctx, nil)
		synctest.Wait()
		require.Len(t, m.state.GetSnapshot().Nodes, 1, "startup must publish the existing PVE peer")
		if recentAvailabilityOutcome {
			const name = "fixture-availability"
			key := schedulerKey(InstanceTypeAvailability, name)
			m.recordTaskResult(InstanceTypeAvailability, name, nil)
			_, _, _, _, oldTransition := breaker.stateDetails()
			availabilityBreaker := m.circuitBreakers[key]
			availabilityBreaker.mu.Lock()
			availabilityBreaker.lastTransition = oldTransition
			availabilityBreaker.mu.Unlock()
		}

		// Occupy just this tick's planner slots so that the only caller
		// which can take the injected hold is tracking cleanup. Later
		// planner reads are admitted normally, with a real saved catalog.
		readsAtStart := catalog.reads.Load()
		atomic.StoreInt32(&m.activePollCount, 2)
		hold.Store(true)
		time.Sleep(scheduleFixtureCadence)
		synctest.Wait()
		readsWhileHeld := catalog.reads.Load() - readsAtStart
		hold.Store(false)
		atomic.StoreInt32(&m.activePollCount, 0)

		physicalDisks := false
		m.mu.Lock()
		m.config.PVEInstances = append(m.config.PVEInstances, config.PVEInstance{
			Name: trackingCleanupNewPeer, Host: "https://fixture-late-pve.invalid:8006", MonitorPhysicalDisks: &physicalDisks,
		})
		m.lastClusterCheck[trackingCleanupNewPeer] = time.Now()
		m.pveClients[trackingCleanupNewPeer] = &pollWorkFixturePVE{getNodes: func(context.Context) ([]proxmox.Node, error) {
			return []proxmox.Node{{Node: trackingCleanupNewPeer, Status: "online", CPU: 0.42, Uptime: 36000}}, nil
		}}
		m.mu.Unlock()
		since := time.Now()
		time.Sleep(scheduleFixtureCadence + time.Second)
		synctest.Wait()
		observe(t, m, readsWhileHeld, since)
	})
}

func TestMonitorTickDoesNotWaitForUnneededCatalogRead(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		recentAvailabilityOutcome bool
	}{
		{"only-another-provider-is-stale", false},
		{"old-availability-breaker-with-fresh-result", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observeUnneededTrackingCatalogRead(t, tc.recentAvailabilityOutcome, assertUnneededTrackingReadDidNotBlock)
		})
	}
}

func assertUnneededTrackingReadDidNotBlock(t *testing.T, m *Monitor, readsWhileHeld int32, since time.Time) {
	nodes := m.state.GetSnapshot().Nodes
	progress := m.deadManMonitoringProgress()
	t.Logf("unneeded reads=%d, published PVE nodes=%d, loop advanced after connection became ready=%t",
		readsWhileHeld, len(nodes), progress.After(since))
	if readsWhileHeld != 0 || !progress.After(since) {
		t.Error("uncalled-for catalog read delayed the monitor ticker")
	}
	require.Len(t, nodes, 2, "ordinary planning must collect the new PVE peer")
	for _, node := range nodes {
		if node.Name == trackingCleanupNewPeer {
			require.Equal(t, 0.42, node.CPU)
			history := m.metricsHistory.GetNodeMetrics(node.ID, "cpu", time.Minute)
			require.Len(t, history, 1)
			require.EqualValues(t, 42, history[0].Value)
			return
		}
	}
	t.Fatal("missing the new PVE peer in published state")
}
