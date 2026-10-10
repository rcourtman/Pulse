package monitoring

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Changes are genuinely consume-once, not the repeatable slice used by the
// generic fixtures. A misplaced drain cannot be hidden by another refresh.
type consumeOnceWiringProvider struct {
	name          string
	beforeRecords func()
	beforeChanges func(*Monitor)
	mu            sync.Mutex
	pending       bool
	drains        int
}

func (p *consumeOnceWiringProvider) SupplementalRecords(*Monitor, string) []unifiedresources.IngestRecord {
	if p.beforeRecords != nil {
		p.beforeRecords()
	}
	return wiringRecords(p.name)
}

func (p *consumeOnceWiringProvider) SupplementalChanges(m *Monitor, org string) []unifiedresources.ResourceChange {
	if p.beforeChanges != nil {
		p.beforeChanges(m)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.pending {
		return nil
	}
	p.pending = false
	p.drains++
	return []unifiedresources.ResourceChange{{ResourceID: p.name, ResourceName: p.name, OrgID: org}}
}

func (p *consumeOnceWiringProvider) drainCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.drains
}

type wiringChangeStore struct {
	*wiringPublicationStore
	changeMu sync.Mutex
	changes  []string
}

func newWiringChangeStore() *wiringChangeStore {
	return &wiringChangeStore{wiringPublicationStore: newWiringPublicationStore()}
}

func (s *wiringChangeStore) RecordChange(change unifiedresources.ResourceChange) error {
	s.changeMu.Lock()
	defer s.changeMu.Unlock()
	s.changes = append(s.changes, change.ResourceID)
	return nil
}

func assertWiringChanges(t *testing.T, s *wiringChangeStore, want ...string) {
	t.Helper()
	s.changeMu.Lock()
	defer s.changeMu.Unlock()
	if len(s.changes) != len(want) {
		t.Fatalf("changes = %v, want %v", s.changes, want)
	}
	for i := range want {
		if s.changes[i] != want[i] {
			t.Fatalf("changes = %v, want %v", s.changes, want)
		}
	}
}

// Hold the first provider callback after the old publication. The returned
// implementation called Type while taking a new provider snapshot; the fixed
// implementation uses the captured set and goes straight to Changes. Both
// reach this same channel barrier before the manual provider is drained.
type changeSelectionBarrier struct {
	testPollProvider
	published *atomic.Bool
	entered   chan struct{}
	release   chan struct{}
	once      sync.Once
}

func (p *changeSelectionBarrier) hold() {
	if p.published.Load() {
		p.once.Do(func() { close(p.entered); <-p.release })
	}
}

func (p *changeSelectionBarrier) Type() InstanceType {
	p.hold()
	return "barrier"
}

func (*changeSelectionBarrier) SupplementalSource() unifiedresources.DataSource {
	return "barrier"
}

func (p *changeSelectionBarrier) SupplementalChanges(*Monitor, string) []unifiedresources.ResourceChange {
	p.hold()
	return nil
}

func TestResourcePublicationChangesStayWithTheirCapturedStoreAndProvider(t *testing.T) {
	for _, read := range []bool{false, true} {
		name := "ingest"
		if read {
			name = "read"
		}
		t.Run(name, func(t *testing.T) {
			oldStore, newStore := newWiringChangeStore(), newWiringChangeStore()
			oldProvider := &consumeOnceWiringProvider{name: "old-host", pending: true}
			newProvider := &consumeOnceWiringProvider{name: "new-host", pending: true}
			var published atomic.Bool
			barrier := &changeSelectionBarrier{published: &published, entered: make(chan struct{}), release: make(chan struct{})}
			// Read refresh uses the native MonitorAdapter; for it the first
			// post-publication callback itself is the unambiguous boundary.
			if read {
				oldProvider.beforeChanges = func(*Monitor) { published.Store(true); barrier.hold() }
			} else {
				oldStore.before = func() { published.Store(true) }
			}
			m := &Monitor{state: models.NewState(), resourceStore: oldStore,
				pollProviders:         map[InstanceType]PollProvider{"barrier": barrier},
				supplementalProviders: map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: oldProvider}}
			newCollecting, releaseNew := make(chan struct{}), make(chan struct{})
			var newOnce, releaseOldOnce, releaseNewOnce sync.Once
			newProvider.beforeRecords = func() { newOnce.Do(func() { close(newCollecting); <-releaseNew }) }
			unblockOld := func() { releaseOldOnce.Do(func() { close(barrier.release) }) }
			unblockNew := func() { releaseNewOnce.Do(func() { close(releaseNew) }) }
			defer unblockOld()
			defer unblockNew()
			oldDone, newDone := make(chan struct{}), make(chan struct{})
			go func() {
				defer close(oldDone)
				state, scope := m.currentStateWithScope()
				if read {
					m.updateResourceStoreForRead(state, scope)
				} else {
					m.updateResourceStore(state, scope)
				}
			}()
			waitForSignal(t, barrier.entered, 5*time.Second, "old publication did not reach change selection")
			go func() {
				defer close(newDone)
				m.SetResourceStoreAndProviders(newStore, map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: newProvider})
			}()
			waitForSignal(t, newCollecting, 5*time.Second, "replacement did not attach and collect")
			unblockOld()
			waitForSignal(t, oldDone, 5*time.Second, "old pass did not finish")
			assertWiringChanges(t, oldStore, "old-host")
			assertWiringChanges(t, newStore)
			if newProvider.drainCount() != 0 {
				t.Fatal("replacement's unpublished change was consumed")
			}
			unblockNew()
			waitForSignal(t, newDone, 5*time.Second, "replacement did not finish")
			m.updateResourceStore(m.currentStateWithScope())
			assertWiringChanges(t, oldStore, "old-host")
			assertWiringChanges(t, newStore, "new-host")
			if oldProvider.drainCount() != 1 || newProvider.drainCount() != 1 {
				t.Fatal("an admitted change was lost or drained twice")
			}
		})
	}
}

func TestResourcePublicationRefusedCollectionsDoNotDrainChanges(t *testing.T) {
	for _, read := range []bool{false, true} {
		for _, endEpoch := range []bool{false, true} {
			name := "ingest-rewire"
			if read {
				name = "read-rewire"
			}
			if endEpoch {
				name += "-ended-epoch"
			}
			t.Run(name, func(t *testing.T) {
				oldStore, newStore := newWiringChangeStore(), newWiringChangeStore()
				entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
				var held, released sync.Once
				unblock := func() { released.Do(func() { close(release) }) }
				defer unblock()
				p := &consumeOnceWiringProvider{name: "delayed-host", pending: true}
				p.beforeRecords = func() { held.Do(func() { close(entered); <-release }) }
				m := &Monitor{state: models.NewState(), resourceStore: oldStore,
					supplementalProviders: map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: p}}
				go func() {
					defer close(done)
					state, scope := m.currentStateWithScope()
					if read {
						m.updateResourceStoreForRead(state, scope)
					} else {
						m.updateResourceStore(state, scope)
					}
				}()
				waitForSignal(t, entered, 5*time.Second, "records did not enter")
				if endEpoch {
					m.mockModeFence.advance()
				} else {
					m.SetResourceStoreAndProviders(newStore, map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: nil})
				}
				unblock()
				waitForSignal(t, done, 5*time.Second, "refused collection did not return")
				assertWiringChanges(t, oldStore)
				if p.drainCount() != 0 || !oldStore.LastRebuiltAt().IsZero() {
					t.Fatal("refused pass published or consumed a change")
				}
				// Reattach/retry in the current epoch: exactly one event survives.
				m.SetResourceStoreAndProviders(newStore, map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: p})
				assertWiringChanges(t, newStore, "delayed-host")
			})
		}
	}
}

func TestResourcePublicationReadSkipsDoNotDrainOrQueue(t *testing.T) {
	store := newWiringChangeStore()
	store.MonitorAdapter.PopulateFromSnapshot(models.StateSnapshot{})
	p := &consumeOnceWiringProvider{name: "pending-host", pending: true}
	m := &Monitor{state: models.NewState(), resourceStore: store,
		supplementalProviders: map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: p}}
	m.updateResourceStoreForRead(m.currentStateWithScope())
	assertWiringChanges(t, store)
	if p.drainCount() != 0 {
		t.Fatal("fresh read drained changes")
	}
	// An adapter without a LastRebuiltAt extension also must not queue behind
	// the monitor publication lock. Its decline is not a successful publish.
	declining := &decliningWiringReadStore{testSupplementalResourceStore: &testSupplementalResourceStore{}}
	m.resourceStore = declining
	m.resourcePublishMu.Lock()
	done := make(chan struct{})
	go func() { defer close(done); m.updateResourceStoreForRead(m.currentStateWithScope()) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		m.resourcePublishMu.Unlock()
		t.Fatal("warm read queued behind an ingest publication")
	}
	m.resourcePublishMu.Unlock()
	m.updateResourceStoreForRead(m.currentStateWithScope())
	if p.drainCount() != 0 || len(declining.recordedChanges) != 0 {
		t.Fatal("busy or declined read drained changes")
	}
	m.updateResourceStore(m.currentStateWithScope())
	if p.drainCount() != 1 || len(declining.recordedChanges) != 1 {
		t.Fatal("skipped reads lost the pending change")
	}
}

type decliningWiringReadStore struct{ *testSupplementalResourceStore }

func (*decliningWiringReadStore) TryReplaceRegistryForRead(models.StateSnapshot, time.Duration, func() map[unifiedresources.DataSource][]unifiedresources.IngestRecord) bool {
	return false
}

func TestResourcePublicationReadProviderCanReenterWiring(t *testing.T) {
	store := newWiringChangeStore()
	newProvider := &consumeOnceWiringProvider{name: "new-host", pending: true}
	oldProvider := &consumeOnceWiringProvider{name: "old-host", pending: true}
	m := &Monitor{state: models.NewState(), resourceStore: store,
		supplementalProviders: map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: oldProvider}}
	oldProvider.beforeRecords = func() {
		m.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, newProvider)
	}
	done := make(chan struct{})
	go func() { defer close(done); m.updateResourceStoreForRead(m.currentStateWithScope()) }()
	waitForSignal(t, done, 5*time.Second, "read provider re-entry deadlocked")
	assertWiringChanges(t, store, "new-host")
	if oldProvider.drainCount() != 0 {
		t.Fatal("refused old read drained changes")
	}
}
