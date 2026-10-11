package monitoring

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func storeWiringProvider(sourceID string) *testMonitorSupplementalProvider {
	return &testMonitorSupplementalProvider{
		recordsByOrg: map[string][]unifiedresources.IngestRecord{
			"default": {{
				SourceID: sourceID,
				Resource: unifiedresources.Resource{
					Type:     unifiedresources.ResourceTypeAgent,
					Name:     sourceID,
					Status:   unifiedresources.StatusOnline,
					LastSeen: time.Now().UTC(),
				},
				Identity: unifiedresources.ResourceIdentity{Hostnames: []string{sourceID}},
			}},
		},
	}
}

// Every wiring call that publishes populates the whole registry and runs the
// alert pass over every resource, so startup, which wires a store and one
// provider per supplemental source, must publish once, not once per source.
func TestSetResourceStoreAndProvidersPublishesOnce(t *testing.T) {
	store := &testSupplementalResourceStore{}
	monitor := &Monitor{state: models.NewState()}

	monitor.SetResourceStoreAndProviders(store, map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{
		unifiedresources.SourceTrueNAS: storeWiringProvider("tn-host-1"),
		unifiedresources.SourceVMware:  storeWiringProvider("esxi-host-1"),
	})

	if store.snapshotCalls != 1 {
		t.Fatalf("wiring a store and two providers populated the registry %d times, want 1", store.snapshotCalls)
	}
	for source, sourceID := range map[unifiedresources.DataSource]string{
		unifiedresources.SourceTrueNAS: "tn-host-1",
		unifiedresources.SourceVMware:  "esxi-host-1",
	} {
		records := store.recordsBySource[source]
		if len(records) != 1 || records[0].SourceID != sourceID {
			t.Fatalf("the one pass ingested %v for %s, want the %s record", records, source, sourceID)
		}
	}
}

// A router re-applies its wiring to a monitor it already wired (a provider
// change re-runs the tenant initializer on every existing monitor, and so do
// SetMonitor and SetMultiTenantMonitor). The same store and providers must not
// repopulate the registry again; a different provider or store must.
func TestSetResourceStoreAndProvidersPublishesOnlyWhenWiringChanged(t *testing.T) {
	store := &testSupplementalResourceStore{}
	monitor := &Monitor{state: models.NewState()}
	trueNAS := storeWiringProvider("tn-host-1")
	vmware := storeWiringProvider("esxi-host-1")
	providers := map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{
		unifiedresources.SourceTrueNAS: trueNAS,
		unifiedresources.SourceVMware:  vmware,
	}

	monitor.SetResourceStoreAndProviders(store, providers)
	monitor.SetResourceStoreAndProviders(store, providers)
	monitor.SetResourceStoreAndProviders(store, map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{unifiedresources.SourceVMware: vmware})
	if store.snapshotCalls != 1 {
		t.Fatalf("re-wiring the same store and providers populated the registry %d times in all, want 1", store.snapshotCalls)
	}

	monitor.SetResourceStoreAndProviders(store, map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{
		unifiedresources.SourceVMware: storeWiringProvider("esxi-host-2"),
	})
	if store.snapshotCalls != 2 {
		t.Fatalf("wiring a different provider populated the registry %d times in all, want 2", store.snapshotCalls)
	}

	monitor.SetResourceStoreAndProviders(&testSupplementalResourceStore{}, nil)
	if store.snapshotCalls != 2 {
		t.Fatalf("the replaced store was populated again: %d calls", store.snapshotCalls)
	}
	replacement, ok := monitor.resourceStore.(*testSupplementalResourceStore)
	if !ok || replacement == store {
		t.Fatalf("the monitor kept the store it was wired with: %#v", monitor.resourceStore)
	}
	if replacement.snapshotCalls != 1 {
		t.Fatalf("wiring a different store populated it %d times, want 1", replacement.snapshotCalls)
	}
}

// A provider type that cannot be compared must read as a change, not panic.
type uncomparableWiringProvider struct {
	records []unifiedresources.IngestRecord
}

func (uncomparableWiringProvider) SupplementalRecords(*Monitor, string) []unifiedresources.IngestRecord {
	return nil
}

func TestSetResourceStoreAndProvidersTreatsUncomparableProvidersAsChanged(t *testing.T) {
	store := &testSupplementalResourceStore{}
	monitor := &Monitor{state: models.NewState()}
	providers := map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{
		unifiedresources.SourceTrueNAS: uncomparableWiringProvider{},
	}

	monitor.SetResourceStoreAndProviders(store, providers)
	monitor.SetResourceStoreAndProviders(store, providers)

	if store.snapshotCalls != 2 {
		t.Fatalf("an uncomparable provider populated the registry %d times for two wirings, want 2", store.snapshotCalls)
	}
}

func TestSetSupplementalRecordsProvidersPublishesOnce(t *testing.T) {
	store := &testSupplementalResourceStore{}
	monitor := &Monitor{state: models.NewState(), resourceStore: store}

	monitor.SetSupplementalRecordsProviders(map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{
		unifiedresources.SourceTrueNAS: storeWiringProvider("tn-host-1"),
		unifiedresources.SourceVMware:  storeWiringProvider("esxi-host-1"),
	})

	if store.snapshotCalls != 1 {
		t.Fatalf("registering two providers populated the registry %d times, want 1", store.snapshotCalls)
	}
	if len(store.recordsBySource[unifiedresources.SourceTrueNAS]) != 1 || len(store.recordsBySource[unifiedresources.SourceVMware]) != 1 {
		t.Fatalf("the pass did not ingest both providers: %v", store.recordsBySource)
	}
}

func TestSetSupplementalRecordsProvidersNormalizesRemovesAndSkipsEmpty(t *testing.T) {
	store := &testSupplementalResourceStore{}
	monitor := &Monitor{state: models.NewState(), resourceStore: store}

	monitor.SetSupplementalRecordsProviders(map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{
		" TrueNAS ": storeWiringProvider("tn-host-1"),
		"   ":       storeWiringProvider("nameless"),
	})
	if store.snapshotCalls != 1 {
		t.Fatalf("a usable source populated the registry %d times, want 1", store.snapshotCalls)
	}
	if len(store.recordsBySource[unifiedresources.SourceTrueNAS]) != 1 {
		t.Fatalf("a provider registered as %q was not found under %q: %v", " TrueNAS ", unifiedresources.SourceTrueNAS, store.recordsBySource)
	}
	if len(store.recordsBySource) != 1 {
		t.Fatalf("a blank source was registered: %v", store.recordsBySource)
	}

	store.snapshotCalls = 0
	monitor.SetSupplementalRecordsProviders(map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{"  ": storeWiringProvider("nameless")})
	monitor.SetSupplementalRecordsProviders(nil)
	if store.snapshotCalls != 0 {
		t.Fatalf("a change naming no usable source populated the registry %d times, want 0", store.snapshotCalls)
	}

	store.recordsBySource = nil
	monitor.SetSupplementalRecordsProviders(map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: nil})
	if store.snapshotCalls != 1 {
		t.Fatalf("removing a provider populated the registry %d times, want 1", store.snapshotCalls)
	}
	if len(store.recordsBySource) != 0 {
		t.Fatalf("a removed provider still supplied records: %v", store.recordsBySource)
	}
}

// The single-source setters keep publishing, so a caller that wires one piece
// at a time still ends with a populated registry.
func TestSingleWiringSettersStillPublish(t *testing.T) {
	store := &testSupplementalResourceStore{}
	monitor := &Monitor{state: models.NewState()}

	monitor.SetResourceStore(store)
	if store.snapshotCalls != 1 {
		t.Fatalf("SetResourceStore populated the registry %d times, want 1", store.snapshotCalls)
	}

	provider := storeWiringProvider("tn-host-1")
	monitor.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, provider)
	if store.snapshotCalls != 2 {
		t.Fatalf("SetSupplementalRecordsProvider populated the registry %d times in all, want 2", store.snapshotCalls)
	}
	if len(store.recordsBySource[unifiedresources.SourceTrueNAS]) != 1 {
		t.Fatalf("the provider's records were not ingested: %v", store.recordsBySource)
	}

	// Unlike the batched wiring step, the single setters are a refresh the
	// caller asked for: the same arguments publish again.
	monitor.SetResourceStore(store)
	monitor.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, provider)
	if store.snapshotCalls != 4 {
		t.Fatalf("repeating the single setters populated the registry %d times in all, want 4", store.snapshotCalls)
	}
}

// This store uses the real in-memory adapter and records every publication,
// rather than only checking the final rows (which could conceal a stale pass).
type wiringPublicationStore struct {
	*unifiedresources.MonitorAdapter
	mu      sync.Mutex
	batches [][]string
	before  func()
}

func (s *wiringPublicationStore) PopulateSnapshotAndSupplemental(snapshot models.StateSnapshot, records map[unifiedresources.DataSource][]unifiedresources.IngestRecord) {
	if s.before != nil {
		s.before()
	}
	names := []string{}
	for _, batch := range records {
		for _, record := range batch {
			names = append(names, record.SourceID)
		}
	}
	s.mu.Lock()
	s.batches = append(s.batches, names)
	s.mu.Unlock()
	s.MonitorAdapter.PopulateSnapshotAndSupplemental(snapshot, records)
}

func newWiringPublicationStore() *wiringPublicationStore {
	return &wiringPublicationStore{MonitorAdapter: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))}
}

type wiringRecordsFunc func(*Monitor, string) []unifiedresources.IngestRecord

func (f wiringRecordsFunc) SupplementalRecords(m *Monitor, org string) []unifiedresources.IngestRecord {
	return f(m, org)
}

func wiringRecords(name string) []unifiedresources.IngestRecord {
	return storeWiringProvider(name).recordsByOrg["default"]
}

// Registration owns a new refresh. A delayed collector is refused before it
// can publish old rows, not repaired by publishing them and repeating a pass.
func TestResourceWiringRefusesCollectedReplacedOrWithdrawnProviders(t *testing.T) {
	for _, withdraw := range []bool{false, true} {
		name := "replace"
		if withdraw {
			name = "withdraw"
		}
		t.Run(name, func(t *testing.T) {
			store := newWiringPublicationStore()
			m := &Monitor{state: models.NewState(), resourceStore: store}
			entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			m.supplementalProviders = map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{
				unifiedresources.SourceTrueNAS: wiringRecordsFunc(func(*Monitor, string) []unifiedresources.IngestRecord {
					close(entered)
					<-release
					return wiringRecords("old-host")
				}),
			}
			go func() { defer close(done); m.updateResourceStore(m.currentStateWithScope()) }()
			waitForSignal(t, entered, 5*time.Second, "old collector did not start")
			var replacement MonitorSupplementalRecordsProvider
			if !withdraw {
				replacement = wiringRecordsFunc(func(*Monitor, string) []unifiedresources.IngestRecord { return wiringRecords("new-host") })
			}
			m.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, replacement)
			unblock()
			waitForSignal(t, done, 5*time.Second, "old collector did not return")
			if len(store.batches) != 1 {
				t.Fatalf("published %v, want only the changed binding", store.batches)
			}
			want := 1
			if withdraw {
				want = 0
			}
			if len(store.batches[0]) != want || (!withdraw && store.batches[0][0] != "new-host") {
				t.Fatalf("publications = %v", store.batches)
			}
			for _, resource := range store.GetAll() {
				if resource.Name == "old-host" {
					t.Fatal("withdrawn/replaced host was republished")
				}
			}
		})
	}
}

// A registration that changes during a store write queues its latest publish
// after that write. It cannot finish and then be overwritten by the old pass.
func TestResourceWiringOrdersChangesDuringPublication(t *testing.T) {
	store := newWiringPublicationStore()
	m := &Monitor{state: models.NewState(), resourceStore: store}
	m.supplementalProviders = map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{
		unifiedresources.SourceTrueNAS: wiringRecordsFunc(func(*Monitor, string) []unifiedresources.IngestRecord { return wiringRecords("old-host") }),
	}
	entered, release, oldDone, newDone := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once, released sync.Once
	unblock := func() { released.Do(func() { close(release) }) }
	defer unblock()
	store.before = func() { once.Do(func() { close(entered); <-release }) }
	go func() { defer close(oldDone); m.updateResourceStore(m.currentStateWithScope()) }()
	waitForSignal(t, entered, 5*time.Second, "old publication did not start")
	go func() {
		defer close(newDone)
		m.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, wiringRecordsFunc(func(*Monitor, string) []unifiedresources.IngestRecord { return wiringRecords("new-host") }))
	}()
	waitForCondition(t, 5*time.Second, func() bool { return m.resourceWiringGeneration.Load() != 0 }, "new binding was not registered")
	select {
	case <-newDone:
		t.Fatal("new binding completed before the old store write finished")
	default:
	}
	unblock()
	waitForSignal(t, oldDone, 5*time.Second, "old publication did not return")
	waitForSignal(t, newDone, 5*time.Second, "new publication did not return")
	if len(store.batches) != 2 || store.batches[1][0] != "new-host" {
		t.Fatalf("publications = %v, want old then new", store.batches)
	}
	if !m.mockModeFence.registryCurrent() {
		t.Fatal("final registry remained marked stale")
	}
	for _, resource := range store.GetAll() {
		if resource.Name == "old-host" {
			t.Fatal("old row survived the changed binding")
		}
	}
}

func TestResourceWiringProviderCanReenterWithoutPublishingItsOldRecords(t *testing.T) {
	store := newWiringPublicationStore()
	m := &Monitor{state: models.NewState(), resourceStore: store}
	m.supplementalProviders = map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{
		unifiedresources.SourceTrueNAS: wiringRecordsFunc(func(m *Monitor, _ string) []unifiedresources.IngestRecord {
			m.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, wiringRecordsFunc(func(*Monitor, string) []unifiedresources.IngestRecord { return wiringRecords("new-host") }))
			return wiringRecords("old-host")
		}),
	}
	done := make(chan struct{})
	go func() { defer close(done); m.updateResourceStore(m.currentStateWithScope()) }()
	waitForSignal(t, done, 5*time.Second, "provider re-entry deadlocked")
	if len(store.batches) != 1 || store.batches[0][0] != "new-host" {
		t.Fatalf("publications = %v", store.batches)
	}
}

func TestResourceWiringJoinsAdmittedChecksAndRefusesOldScopes(t *testing.T) {
	m := &Monitor{state: models.NewState()}
	old := m.mockModeFence.begin()
	entered, release, checkDone, wiringDone := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	go func() { defer close(checkDone); old.run(func() { close(entered); <-release }) }()
	waitForSignal(t, entered, 5*time.Second, "old check did not start")
	go func() {
		defer close(wiringDone)
		m.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, storeWiringProvider("new-host"))
	}()
	waitForCondition(t, 5*time.Second, func() bool { return m.resourceWiringGeneration.Load() != 0 }, "binding did not change")
	select {
	case <-wiringDone:
		t.Fatal("rewiring did not join admitted old checks")
	default:
	}
	unblock()
	waitForSignal(t, checkDone, 5*time.Second, "old check did not finish")
	waitForSignal(t, wiringDone, 5*time.Second, "wiring did not finish")
	if old.run(func() { t.Error("old check was admitted after rewiring") }) {
		t.Fatal("old scope remained usable")
	}
	if !m.mockModeFence.begin().run(func() {}) {
		t.Fatal("new scope refused its check")
	}
}

// Store changes must invalidate a collector too, even with identical providers.
func TestResourceWiringRefusesCollectedRecordsForAReplacedStore(t *testing.T) {
	oldStore, newStore := newWiringPublicationStore(), newWiringPublicationStore()
	m := &Monitor{state: models.NewState(), resourceStore: oldStore}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var held atomic.Bool
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	p := wiringRecordsFunc(func(*Monitor, string) []unifiedresources.IngestRecord {
		if held.CompareAndSwap(false, true) {
			close(entered)
			<-release
		}
		return wiringRecords("host")
	})
	m.supplementalProviders = map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: p}
	go func() { defer close(done); m.updateResourceStore(m.currentStateWithScope()) }()
	waitForSignal(t, entered, 5*time.Second, "collector did not start")
	m.SetResourceStore(newStore)
	unblock()
	waitForSignal(t, done, 5*time.Second, "collector did not return")
	if len(oldStore.batches) != 0 || len(newStore.batches) != 1 {
		t.Fatalf("old store published %v; new store published %v", oldStore.batches, newStore.batches)
	}
}

type modeBoundWiringProvider struct{ mock bool }

func (p *modeBoundWiringProvider) SupplementalRecordsMockMode() bool { return p.mock }
func (p *modeBoundWiringProvider) SupplementalRecords(*Monitor, string) []unifiedresources.IngestRecord {
	return wiringRecords("bound-host")
}
func (p *modeBoundWiringProvider) SnapshotOwnedSources() []unifiedresources.DataSource {
	return []unifiedresources.DataSource{unifiedresources.SourceTrueNAS}
}

// Exercise both intervals before the post-switch router hook has replaced the
// old pair. Its records and ownership must be absent, not a live/demo mixture.
func TestResourceWiringModeBoundProvidersDoNotCrossTheSwitchHookGap(t *testing.T) {
	previous := mock.IsMockEnabled()
	t.Cleanup(func() { mustSetMockEnabled(t, previous) })
	for _, mode := range []bool{false, true} {
		mustSetMockEnabled(t, mode)
		m := &Monitor{supplementalProviders: map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: &modeBoundWiringProvider{mock: !mode}}}
		if got := m.collectSupplementalRecordsBySource(); len(got) != 0 {
			t.Fatalf("mode %v included opposite-mode records: %v", mode, got)
		}
		if got := m.providerOwnedSnapshotSources(); len(got) != 0 {
			t.Fatalf("mode %v kept opposite-mode snapshot ownership: %v", mode, got)
		}
		m.supplementalProviders[unifiedresources.SourceTrueNAS] = &modeBoundWiringProvider{mock: mode}
		if got := m.collectSupplementalRecordsBySource(); len(got[unifiedresources.SourceTrueNAS]) != 1 {
			t.Fatalf("mode %v lost its matching records: %v", mode, got)
		}
	}
}

func TestResourceWiringDoesNotClearExistingSystemAlerts(t *testing.T) {
	manager := newMockHostAlertTestManager(t)
	if !manager.RaiseSystemAlert(alerts.SystemAlertInput{Type: alerts.BackupEvaluationAlertType, Level: alerts.AlertLevelWarning, Message: "existing condition"}) {
		t.Fatal("control did not create system alert")
	}
	m := &Monitor{state: models.NewState(), alertManager: manager}
	m.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, storeWiringProvider("new-host"))
	active := manager.GetActiveAlerts()
	if len(active) != 1 || active[0].Type != alerts.BackupEvaluationAlertType {
		t.Fatalf("rewiring cleared existing alerts: %v", active)
	}
}

func TestResourceWiringSingleProviderPublishesWithoutLegacyState(t *testing.T) {
	store := newWiringPublicationStore()
	m := &Monitor{resourceStore: store}
	m.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, storeWiringProvider("native-host"))
	if len(store.batches) != 1 || len(store.batches[0]) != 1 || store.batches[0][0] != "native-host" {
		t.Fatalf("source-native backfill without state = %v", store.batches)
	}
}
