package api

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
)

type countingSupplementalRecords struct {
	reads atomic.Int32
}

func (c *countingSupplementalRecords) SupplementalRecords(*monitoring.Monitor, string) []unifiedresources.IngestRecord {
	c.reads.Add(1)
	return nil
}

func newWiringTestMonitor(t *testing.T) *monitoring.Monitor {
	t.Helper()
	monitor, err := monitoring.New(&config.Config{DataPath: t.TempDir()})
	if err != nil {
		t.Fatalf("new monitor: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })
	return monitor
}

// Wiring a monitor attaches its store and every supplemental provider. Each
// separate wiring step published the whole estate, so a router with two
// platform providers ran three full store passes (and the first two read an
// incomplete provider set) before the listener opened.
func TestConfigureMonitorDependenciesPublishesTheEstateOnce(t *testing.T) {
	trueNAS := &countingSupplementalRecords{}
	vmware := &countingSupplementalRecords{}
	router := &Router{
		monitorResourceAdapter:  unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)),
		monitorResourceAdapters: make(map[string]*unifiedresources.MonitorAdapter),
		monitorSupplementalRecords: map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider{
			unifiedresources.SourceTrueNAS: trueNAS,
			unifiedresources.SourceVMware:  vmware,
		},
	}

	monitor := newWiringTestMonitor(t)
	router.configureMonitorDependencies(monitor)

	if got := trueNAS.reads.Load(); got != 1 {
		t.Fatalf("TrueNAS provider read %d times while wiring one monitor, want 1 store pass", got)
	}
	if got := vmware.reads.Load(); got != 1 {
		t.Fatalf("VMware provider read %d times while wiring one monitor, want 1 store pass", got)
	}

	// NewRouter, SetMonitor, SetMultiTenantMonitor and every provider change
	// wire the same monitor again; none of it has anything new to publish.
	router.configureMonitorDependencies(monitor)
	router.configureMonitorDependencies(monitor)
	if trueNAS.reads.Load() != 1 || vmware.reads.Load() != 1 {
		t.Fatalf("re-wiring the same monitor read the providers %d and %d times in all, want one store pass", trueNAS.reads.Load(), vmware.reads.Load())
	}
}

// passCountingStore counts the registry rebuilds a monitor publishes into it.
type passCountingStore struct {
	*unifiedresources.MonitorAdapter
	passes atomic.Int32
}

func (s *passCountingStore) PopulateSnapshotAndSupplemental(snapshot models.StateSnapshot, recordsBySource map[unifiedresources.DataSource][]unifiedresources.IngestRecord) {
	s.passes.Add(1)
	s.MonitorAdapter.PopulateSnapshotAndSupplemental(snapshot, recordsBySource)
}

// A mock-mode switch re-registers both platform providers; the running monitor
// must publish once for the pair, not once per source.
func TestSyncPlatformSupplementalProvidersPublishesOnceForBothSources(t *testing.T) {
	previousTrueNAS := truenas.IsFeatureEnabled()
	previousVMware := vmware.IsFeatureEnabled()
	t.Cleanup(func() {
		truenas.SetFeatureEnabled(previousTrueNAS)
		vmware.SetFeatureEnabled(previousVMware)
	})

	monitor := newWiringTestMonitor(t)
	store := &passCountingStore{MonitorAdapter: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))}
	monitor.SetResourceStore(store)
	router := &Router{monitor: monitor}

	before := store.passes.Load()
	router.syncPlatformSupplementalProviders(true)
	if got := store.passes.Load() - before; got != 1 {
		t.Fatalf("switching mock mode on published %d times, want 1 for the TrueNAS and VMware pair", got)
	}

	before = store.passes.Load()
	router.syncPlatformSupplementalProviders(false)
	if got := store.passes.Load() - before; got != 1 {
		t.Fatalf("switching mock mode off published %d times, want 1 for the TrueNAS and VMware pair", got)
	}
}

// This uses a real alert on the first publication, not a later provider read:
// saved policy must be installed before any alert reaches the lifecycle hook.
func TestConfigureMonitorDependenciesAppliesPolicyBeforeFirstAlert(t *testing.T) {
	monitor := newWiringTestMonitor(t)
	manager := monitor.GetAlertManager()
	cfg := manager.GetConfig()
	cfg.Enabled = false
	manager.UpdateConfig(cfg)
	// Install a cached source-native provider before a store is attached. Its
	// first publication evaluates a real TrueNAS memory threshold; a normal
	// host-agent row is checked on its separate legacy alert path instead.
	provider := &alertWiringProvider{}
	monitor.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, provider)
	cfg = manager.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = alerts.ActivationActive
	cfg.TimeThresholds = map[string]int{}
	cfg.MetricTimeThresholds = nil
	cfg.TrueNASDefaults.Memory = &alerts.HysteresisThreshold{Trigger: 1, Clear: 0.5}
	manager.UpdateConfig(cfg)
	persistence := config.NewConfigPersistence(t.TempDir())
	settings := config.DefaultSystemSettings()
	settings.PublicURL = "https://pulse.example.test"
	settings.WebhookAllowedPrivateCIDRs = "192.168.50.0/24"
	if err := persistence.SaveSystemSettings(*settings); err != nil {
		t.Fatal(err)
	}
	notifications := monitor.GetNotificationManager()
	if notifications.ValidateWebhookURL("http://192.168.50.2/hook") == nil {
		t.Fatal("fresh manager already allows the saved private network")
	}
	observations := 0
	unsubscribe := manager.SubscribeLifecycleCallback(func(alerts.LifecycleEvent) {
		observations++
		if got := notifications.GetPublicURL(); got != settings.PublicURL {
			t.Errorf("first-publication alert used PublicURL %q, want %q", got, settings.PublicURL)
		}
		if err := notifications.ValidateWebhookURL("http://192.168.50.2/hook"); err != nil {
			t.Errorf("saved allowlist absent at first-publication alert: %v", err)
		}
		if notifications.ValidateWebhookURL("http://192.168.51.2/hook") == nil {
			t.Error("unlisted private network was admitted")
		}
	})
	defer unsubscribe()
	router := &Router{persistence: persistence, monitorResourceAdapter: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))}
	router.configureMonitorDependencies(monitor)
	if observations == 0 {
		resources := wiringResources(monitor)
		if len(resources) != 1 {
			t.Fatalf("no alert; resources = %v", resources)
		}
		input, _ := alerts.UnifiedResourceInputFromResource(resources[0])
		t.Fatalf("first publication emitted no lifecycle event; input=%+v policy=%+v", input, manager.GetConfig().AgentDefaults)
	}
}

type tenantWiringProvider struct{ prefix string }

func (p *tenantWiringProvider) SupplementalRecords(_ *monitoring.Monitor, org string) []unifiedresources.IngestRecord {
	name := p.prefix + "-" + org
	return []unifiedresources.IngestRecord{{SourceID: name, Resource: unifiedresources.Resource{Type: unifiedresources.ResourceTypeAgent, Name: name, Status: unifiedresources.StatusOnline, LastSeen: time.Now().UTC()}, Identity: unifiedresources.ResourceIdentity{Hostnames: []string{name}}}}
}

func TestRouterResourceWiringWithdrawsProvidersFromEveryExistingAndFutureTenant(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	for _, org := range []string{"tenant-a", "tenant-b", "tenant-c"} {
		if err := persistence.SaveOrganization(&models.Organization{ID: org}); err != nil {
			t.Fatal(err)
		}
	}
	mt := monitoring.NewMultiTenantMonitor(&config.Config{DataPath: t.TempDir()}, persistence, nil)
	defer mt.Stop()
	router := &Router{mtMonitor: mt}
	provider := &tenantWiringProvider{prefix: "source"}
	router.setMonitorSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, provider)
	for _, org := range []string{"tenant-a", "tenant-b"} {
		m, err := mt.GetMonitor(org)
		if err != nil {
			t.Fatal(err)
		}
		resources := wiringResources(m)
		if len(resources) != 1 || resources[0].Name != "source-"+org {
			t.Fatalf("%s registry = %v", org, resources)
		}
	}
	router.setMonitorSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, nil)
	for _, org := range []string{"tenant-a", "tenant-b", "tenant-c"} {
		m, err := mt.GetMonitor(org)
		if err != nil {
			t.Fatal(err)
		}
		if resources := wiringResources(m); len(resources) != 0 {
			t.Fatalf("%s retained withdrawn provider: %v", org, resources)
		}
	}
}

func TestRouterResourceWiringSerializesInitializationAndWithdrawal(t *testing.T) {
	monitor := newWiringTestMonitor(t)
	entered, release, initialized, withdrawn := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	provider := &blockingRouterWiringProvider{entered: entered, release: release}
	router := &Router{monitor: monitor, monitorResourceAdapter: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)), monitorSupplementalRecords: map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: provider}}
	go func() { defer close(initialized); router.configureMonitorDependencies(monitor) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("initializer did not collect")
	}
	go func() {
		defer close(withdrawn)
		router.setMonitorSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, nil)
	}()
	// Check the exact exclusion boundary while the initializer is blocked.
	if router.monitorSupplementalMu.TryLock() {
		router.monitorSupplementalMu.Unlock()
		t.Fatal("initializer released its provider registry before applying it")
	}
	unblock()
	select {
	case <-initialized:
	case <-time.After(5 * time.Second):
		t.Fatal("initializer did not finish")
	}
	select {
	case <-withdrawn:
	case <-time.After(5 * time.Second):
		t.Fatal("withdrawal did not finish")
	}
	if resources := wiringResources(monitor); len(resources) != 0 {
		t.Fatalf("withdrawal left rows: %v", resources)
	}
}

type blockingRouterWiringProvider struct{ entered, release chan struct{} }

func (p *blockingRouterWiringProvider) SupplementalRecords(*monitoring.Monitor, string) []unifiedresources.IngestRecord {
	close(p.entered)
	<-p.release
	return (&tenantWiringProvider{prefix: "old"}).SupplementalRecords(nil, "default")
}

func wiringResources(m *monitoring.Monitor) []unifiedresources.Resource {
	return m.GetUnifiedReadState().(interface {
		GetAll() []unifiedresources.Resource
	}).GetAll()
}

func TestConfigureMonitorDependenciesRevokesEmptySavedAllowlist(t *testing.T) {
	monitor := newWiringTestMonitor(t)
	persistence := config.NewConfigPersistence(t.TempDir())
	settings := config.DefaultSystemSettings()
	settings.PublicURL = "https://pulse.example.test"
	settings.WebhookAllowedPrivateCIDRs = "192.168.50.0/24"
	if err := persistence.SaveSystemSettings(*settings); err != nil {
		t.Fatal(err)
	}
	router := &Router{persistence: persistence, monitorResourceAdapter: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))}
	router.configureMonitorDependencies(monitor)
	nm := monitor.GetNotificationManager()
	if nm.GetPublicURL() != settings.PublicURL || nm.ValidateWebhookURL("http://192.168.50.2/hook") != nil {
		t.Fatal("saved policy did not apply")
	}
	settings.WebhookAllowedPrivateCIDRs = ""
	if err := persistence.SaveSystemSettings(*settings); err != nil {
		t.Fatal(err)
	}
	router.configureMonitorDependencies(monitor)
	if nm.GetPublicURL() != settings.PublicURL || nm.ValidateWebhookURL("http://192.168.50.2/hook") == nil {
		t.Fatal("reinitialization retained revoked notification policy")
	}
}

// A cached, synthetic source-native observation. No provider request is made.
type alertWiringProvider struct{}

func (*alertWiringProvider) SupplementalRecords(*monitoring.Monitor, string) []unifiedresources.IngestRecord {
	return []unifiedresources.IngestRecord{{SourceID: "policy-proof-host", Resource: unifiedresources.Resource{
		Type: unifiedresources.ResourceTypeAgent, Name: "policy-proof-host", Status: unifiedresources.StatusOnline, LastSeen: time.Now().UTC(),
		Sources: []unifiedresources.DataSource{unifiedresources.SourceTrueNAS},
		Metrics: &unifiedresources.ResourceMetrics{Memory: &unifiedresources.MetricValue{Value: 99, Percent: 99, Unit: "percent"}},
	}, Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"policy-proof-host"}}}}
}
