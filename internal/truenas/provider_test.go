package truenas

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type stubFetcher struct {
	snapshot *FixtureSnapshot
	err      error
	calls    int
}

func (s *stubFetcher) Fetch(context.Context) (*FixtureSnapshot, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return copyFixtureSnapshot(s.snapshot), nil
}

type closableStubFetcher struct {
	closeCalls int
}

func (s *closableStubFetcher) Fetch(context.Context) (*FixtureSnapshot, error) {
	return nil, nil
}

func (s *closableStubFetcher) Close() {
	s.closeCalls++
}

type controllableStubFetcher struct {
	snapshot    *FixtureSnapshot
	startCalls  []string
	stopCalls   []string
	logReads    []appLogReadCall
	diskHistory map[string][]TimeSeriesPoint
}

type appLogReadCall struct {
	appName     string
	containerID string
	tailLines   int
}

func (s *controllableStubFetcher) Fetch(context.Context) (*FixtureSnapshot, error) {
	return copyFixtureSnapshot(s.snapshot), nil
}

func (s *controllableStubFetcher) StartApp(_ context.Context, appID string) error {
	s.startCalls = append(s.startCalls, appID)
	for i := range s.snapshot.Apps {
		if s.snapshot.Apps[i].ID == appID {
			s.snapshot.Apps[i].State = "RUNNING"
			if len(s.snapshot.Apps[i].Containers) > 0 {
				s.snapshot.Apps[i].Containers[0].State = "running"
			}
		}
	}
	return nil
}

func (s *controllableStubFetcher) StopApp(_ context.Context, appID string) error {
	s.stopCalls = append(s.stopCalls, appID)
	for i := range s.snapshot.Apps {
		if s.snapshot.Apps[i].ID == appID {
			s.snapshot.Apps[i].State = "STOPPED"
			if len(s.snapshot.Apps[i].Containers) > 0 {
				s.snapshot.Apps[i].Containers[0].State = "stopped"
			}
		}
	}
	return nil
}

func (s *controllableStubFetcher) ReadAppLogs(_ context.Context, appName, containerID string, tailLines int) ([]AppLogLine, error) {
	s.logReads = append(s.logReads, appLogReadCall{
		appName:     appName,
		containerID: containerID,
		tailLines:   tailLines,
	})
	return []AppLogLine{
		{Timestamp: "2026-03-29T18:00:00Z", Data: "ready"},
		{Timestamp: "2026-03-29T18:01:00Z", Data: "serving"},
	}, nil
}

func (s *controllableStubFetcher) DiskTemperatureHistory(_ context.Context, identifiers []string, _ time.Duration) (map[string][]TimeSeriesPoint, error) {
	if len(s.diskHistory) == 0 {
		return nil, nil
	}
	result := make(map[string][]TimeSeriesPoint)
	for _, identifier := range identifiers {
		points, ok := s.diskHistory[identifier]
		if !ok || len(points) == 0 {
			continue
		}
		copied := make([]TimeSeriesPoint, len(points))
		copy(copied, points)
		result[identifier] = copied
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func TestFixtureFetcherReturnsSnapshotCopy(t *testing.T) {
	fixtures := DefaultFixtures()
	fetcher := &FixtureFetcher{Snapshot: fixtures}

	first, err := fetcher.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if first == nil {
		t.Fatal("expected snapshot")
	}

	first.Pools[0].Name = "mutated"
	first.Datasets = append(first.Datasets, Dataset{Name: "extra/dataset"})
	first.System.TemperatureCelsius["cpu_package"] = 99.9

	second, err := fetcher.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() second error = %v", err)
	}
	if second == nil {
		t.Fatal("expected second snapshot")
	}
	if second.Pools[0].Name != fixtures.Pools[0].Name {
		t.Fatalf("expected fixture pool name %q, got %q", fixtures.Pools[0].Name, second.Pools[0].Name)
	}
	if len(second.Datasets) != len(fixtures.Datasets) {
		t.Fatalf("expected dataset count %d, got %d", len(fixtures.Datasets), len(second.Datasets))
	}
	if second.System.TemperatureCelsius["cpu_package"] != fixtures.System.TemperatureCelsius["cpu_package"] {
		t.Fatalf("expected fixture cpu_package temperature %v, got %v", fixtures.System.TemperatureCelsius["cpu_package"], second.System.TemperatureCelsius["cpu_package"])
	}
}

func TestAPIFetcherDelegatesToClientFetchSnapshot(t *testing.T) {
	server := newMockServer(t, defaultAPIResponses(), nil)
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	fetcher := &APIFetcher{Client: client}

	snapshot, err := fetcher.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected snapshot")
	}
	if snapshot.System.Hostname != "truenas-main" {
		t.Fatalf("unexpected hostname: %q", snapshot.System.Hostname)
	}
}

func TestProviderRefreshUpdatesLastSnapshot(t *testing.T) {
	server := newMockServer(t, defaultAPIResponses(), nil)
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	provider := NewLiveProvider(&APIFetcher{Client: client})

	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	provider.mu.Lock()
	snapshot := copyFixtureSnapshot(provider.lastSnapshot)
	provider.mu.Unlock()

	if snapshot == nil {
		t.Fatal("expected cached snapshot")
	}
	if snapshot.System.Hostname != "truenas-main" {
		t.Fatalf("unexpected cached hostname: %q", snapshot.System.Hostname)
	}
	if len(snapshot.Pools) != 1 || len(snapshot.Datasets) != 1 {
		t.Fatalf("unexpected cached counts: pools=%d datasets=%d", len(snapshot.Pools), len(snapshot.Datasets))
	}
}

func TestProviderControlAppUsesFetcherAndRefreshesSnapshot(t *testing.T) {
	fixtures := DefaultFixtures()
	for i := range fixtures.Apps {
		if fixtures.Apps[i].ID == "nextcloud" {
			fixtures.Apps[i].State = "STOPPED"
			if len(fixtures.Apps[i].Containers) > 0 {
				fixtures.Apps[i].Containers[0].State = "stopped"
			}
		}
	}

	fetcher := &controllableStubFetcher{snapshot: &fixtures}
	provider := NewLiveProvider(fetcher)

	snapshot, err := provider.ControlApp(context.Background(), "nextcloud", "start")
	if err != nil {
		t.Fatalf("ControlApp(start) error = %v", err)
	}
	if len(fetcher.startCalls) != 1 || fetcher.startCalls[0] != "nextcloud" {
		t.Fatalf("expected start call for nextcloud, got %+v", fetcher.startCalls)
	}
	if snapshot == nil {
		t.Fatal("expected refreshed snapshot after control")
	}
	foundRunning := false
	for _, app := range snapshot.Apps {
		if app.ID == "nextcloud" && app.State == "RUNNING" {
			foundRunning = true
		}
	}
	if !foundRunning {
		t.Fatalf("expected nextcloud to be RUNNING after control, got %+v", snapshot.Apps)
	}
}

func TestProviderReadAppLogsUsesCanonicalPrimaryContainer(t *testing.T) {
	fixtures := DefaultFixtures()
	fetcher := &controllableStubFetcher{snapshot: &fixtures}
	provider := NewLiveProvider(fetcher)
	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	result, err := provider.ReadAppLogs(context.Background(), "nextcloud", "", 25)
	if err != nil {
		t.Fatalf("ReadAppLogs() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected app log result")
	}
	if result.Host != "truenas-main" || result.App.Name != "Nextcloud" {
		t.Fatalf("unexpected app log result identity: %+v", result)
	}
	if result.Container.ServiceName != "nextcloud" || result.Container.ID != "nextcloud-web-1" {
		t.Fatalf("expected canonical primary container, got %+v", result.Container)
	}
	if len(fetcher.logReads) != 1 {
		t.Fatalf("expected one log read, got %+v", fetcher.logReads)
	}
	if call := fetcher.logReads[0]; call.appName != "nextcloud" || call.containerID != "nextcloud-web-1" || call.tailLines != 25 {
		t.Fatalf("unexpected log read call: %+v", call)
	}
}

func TestProviderReadAppLogsSupportsExplicitServiceSelection(t *testing.T) {
	fixtures := DefaultFixtures()
	fetcher := &controllableStubFetcher{snapshot: &fixtures}
	provider := NewLiveProvider(fetcher)
	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	result, err := provider.ReadAppLogs(context.Background(), "nextcloud", "redis", 10)
	if err != nil {
		t.Fatalf("ReadAppLogs() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected app log result")
	}
	if result.Container.ServiceName != "redis" || result.Container.ID != "nextcloud-redis-1" {
		t.Fatalf("expected explicit redis container selection, got %+v", result.Container)
	}
	if len(fetcher.logReads) != 1 || fetcher.logReads[0].containerID != "nextcloud-redis-1" {
		t.Fatalf("unexpected log read calls: %+v", fetcher.logReads)
	}
}

func TestProviderGetAppConfigReturnsCanonicalApp(t *testing.T) {
	fixtures := DefaultFixtures()
	fetcher := &controllableStubFetcher{snapshot: &fixtures}
	provider := NewLiveProvider(fetcher)
	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	result, err := provider.GetAppConfig(context.Background(), "nextcloud")
	if err != nil {
		t.Fatalf("GetAppConfig() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected app config result")
	}
	if result.Host != "truenas-main" {
		t.Fatalf("expected host truenas-main, got %+v", result)
	}
	if result.App.ID != "nextcloud" || result.App.Name != "Nextcloud" {
		t.Fatalf("unexpected app config payload: %+v", result.App)
	}
	if len(result.App.Containers) != 2 {
		t.Fatalf("expected full app container runtime shape, got %+v", result.App.Containers)
	}
}

func TestProviderRefreshPreservesLastSnapshotOnError(t *testing.T) {
	initial := DefaultFixtures()
	provider := NewProvider(initial)

	expectedErr := errors.New("fetch failed")
	provider.fetcher = &stubFetcher{err: expectedErr}

	err := provider.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected Refresh() error")
	}
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	provider.mu.Lock()
	snapshot := copyFixtureSnapshot(provider.lastSnapshot)
	provider.mu.Unlock()

	if snapshot == nil {
		t.Fatal("expected cached snapshot")
	}
	if snapshot.System.Hostname != initial.System.Hostname {
		t.Fatalf("expected hostname %q, got %q", initial.System.Hostname, snapshot.System.Hostname)
	}
	if len(snapshot.Pools) != len(initial.Pools) {
		t.Fatalf("expected pool count %d, got %d", len(initial.Pools), len(snapshot.Pools))
	}
}

func TestProviderRefreshPreservesLastSnapshotOnNilSnapshot(t *testing.T) {
	initial := DefaultFixtures()
	provider := NewProvider(initial)
	provider.fetcher = &stubFetcher{snapshot: nil}

	err := provider.Refresh(context.Background())
	if !errors.Is(err, errNilSnapshot) {
		t.Fatalf("expected errNilSnapshot, got %v", err)
	}

	provider.mu.Lock()
	snapshot := copyFixtureSnapshot(provider.lastSnapshot)
	provider.mu.Unlock()

	if snapshot == nil {
		t.Fatal("expected cached snapshot")
	}
	if snapshot.System.Hostname != initial.System.Hostname {
		t.Fatalf("expected hostname %q, got %q", initial.System.Hostname, snapshot.System.Hostname)
	}
	if len(snapshot.Pools) != len(initial.Pools) {
		t.Fatalf("expected pool count %d, got %d", len(initial.Pools), len(snapshot.Pools))
	}
}

func TestRecordsDoesNotCallRefreshWhenSnapshotMissing(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	stub := &stubFetcher{}
	provider := NewLiveProvider(stub)

	records := provider.Records()
	if records != nil {
		t.Fatalf("expected nil records when no snapshot is cached, got %d records", len(records))
	}
	if stub.calls != 0 {
		t.Fatalf("expected Records() to avoid fetch calls, got %d", stub.calls)
	}
}

func TestSystemRecordPopulatesTrueNASMetadata(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	provider := NewProvider(DefaultFixtures())
	records := provider.Records()
	if len(records) == 0 {
		t.Fatal("expected fixture records from provider")
	}

	system := records[0]
	if system.Resource.Type != unifiedresources.ResourceTypeAgent {
		t.Fatalf("expected first record type host, got %s", system.Resource.Type)
	}
	if system.Resource.TrueNAS == nil {
		t.Fatal("expected TrueNAS metadata on system record")
	}
	if system.Resource.TrueNAS.UptimeSeconds != int64(42*24*60*60) {
		t.Fatalf("expected uptime %d, got %d", int64(42*24*60*60), system.Resource.TrueNAS.UptimeSeconds)
	}
	if system.Resource.TrueNAS.Version != "TrueNAS-SCALE-24.10.2" {
		t.Fatalf("expected version %q, got %q", "TrueNAS-SCALE-24.10.2", system.Resource.TrueNAS.Version)
	}
	if system.Resource.TrueNAS.Hostname != "truenas-main" {
		t.Fatalf("expected hostname %q, got %q", "truenas-main", system.Resource.TrueNAS.Hostname)
	}
	if system.Resource.Agent == nil {
		t.Fatal("expected canonical agent payload on TrueNAS system record")
	}
	if system.Resource.Agent.Platform != "truenas" || system.Resource.Agent.OSName != "TrueNAS" {
		t.Fatalf("expected canonical TrueNAS agent metadata, got %+v", system.Resource.Agent)
	}
	if system.Resource.Agent.CPUCount != 16 || system.Resource.Agent.UptimeSeconds != int64(42*24*60*60) {
		t.Fatalf("expected canonical host telemetry metadata, got %+v", system.Resource.Agent)
	}
	if system.Resource.Agent.Temperature == nil || *system.Resource.Agent.Temperature != 61.5 {
		t.Fatalf("expected canonical host temperature 61.5, got %+v", system.Resource.Agent)
	}
	if system.Resource.Agent.Sensors == nil || system.Resource.Agent.Sensors.TemperatureCelsius["cpu_package"] != 61.5 {
		t.Fatalf("expected canonical host sensors on TrueNAS system record, got %+v", system.Resource.Agent.Sensors)
	}
	if system.Resource.Metrics == nil || system.Resource.Metrics.CPU == nil || system.Resource.Metrics.Memory == nil {
		t.Fatalf("expected canonical system metrics on TrueNAS host record, got %+v", system.Resource.Metrics)
	}
	if system.Resource.Metrics.CPU.Percent != 38 {
		t.Fatalf("expected cpu percent 38, got %+v", system.Resource.Metrics.CPU)
	}
	if system.Resource.Metrics.NetIn == nil || system.Resource.Metrics.NetIn.Value != 48_000_000 {
		t.Fatalf("expected network telemetry metrics, got %+v", system.Resource.Metrics)
	}
}

func TestRecordsProjectTrueNASAppStatsIntoCanonicalMetrics(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	provider := NewProvider(DefaultFixtures())
	records := provider.Records()
	if len(records) == 0 {
		t.Fatal("expected fixture records from provider")
	}

	for _, record := range records {
		if record.Resource.Type != unifiedresources.ResourceTypeAppContainer || record.Resource.Name != "Nextcloud" {
			continue
		}
		if record.Resource.Metrics == nil || record.Resource.Metrics.CPU == nil || record.Resource.Metrics.CPU.Percent != 18 {
			t.Fatalf("expected canonical CPU metrics on Nextcloud, got %+v", record.Resource.Metrics)
		}
		if record.Resource.Docker == nil || record.Resource.Docker.NetInRate != 2_100_000 || record.Resource.Docker.DiskReadRate != 320_000 {
			t.Fatalf("expected projected TrueNAS app rates, got %+v", record.Resource.Docker)
		}
		return
	}

	t.Fatal("expected Nextcloud app record")
}

func TestProviderRefreshDerivesTrueNASAppDiskRatesFromPreviousSnapshot(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	initial := DefaultFixtures()
	initial.CollectedAt = time.Date(2026, 2, 8, 12, 0, 0, 0, time.UTC)
	initial.Apps[0].Stats.BlockReadBytes = 100
	initial.Apps[0].Stats.BlockWriteBytes = 50
	initial.Apps[0].Stats.CollectedAt = initial.CollectedAt

	updated := DefaultFixtures()
	updated.CollectedAt = initial.CollectedAt.Add(4 * time.Second)
	updated.Apps[0].Stats.BlockReadBytes = 500
	updated.Apps[0].Stats.BlockWriteBytes = 250
	updated.Apps[0].Stats.CollectedAt = updated.CollectedAt

	provider := NewProvider(initial)
	provider.fetcher = &stubFetcher{snapshot: &updated}

	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	snapshot := provider.Snapshot()
	if snapshot == nil || snapshot.Apps[0].Stats == nil {
		t.Fatal("expected refreshed app stats")
	}
	if snapshot.Apps[0].Stats.DiskReadRate != 100 {
		t.Fatalf("expected disk read rate 100, got %v", snapshot.Apps[0].Stats.DiskReadRate)
	}
	if snapshot.Apps[0].Stats.DiskWriteRate != 50 {
		t.Fatalf("expected disk write rate 50, got %v", snapshot.Apps[0].Stats.DiskWriteRate)
	}
}

func TestAPIFetcherCloseDelegatesToClient(t *testing.T) {
	transport := &closeTrackingTransport{}
	client := &Client{
		httpClient: &http.Client{
			Transport: transport,
		},
	}
	fetcher := &APIFetcher{Client: client}

	fetcher.Close()
	if transport.closeCalls != 1 {
		t.Fatalf("expected CloseIdleConnections to be called once, got %d", transport.closeCalls)
	}
}

func TestProviderCloseDelegatesToFetcher(t *testing.T) {
	fetcher := &closableStubFetcher{}
	provider := NewLiveProvider(fetcher)

	provider.Close()
	if fetcher.closeCalls != 1 {
		t.Fatalf("expected fetcher Close() to be called once, got %d", fetcher.closeCalls)
	}
}

func TestRecordsIncludeDiskResourcesWithCorrectParentChain(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	provider := NewProvider(DefaultFixtures())
	records := provider.Records()
	if len(records) == 0 {
		t.Fatal("expected fixture records from provider")
	}

	var diskRecords []unifiedresources.IngestRecord
	for _, record := range records {
		if record.Resource.Type == unifiedresources.ResourceTypePhysicalDisk {
			diskRecords = append(diskRecords, record)
		}
	}
	if len(diskRecords) != 4 {
		t.Fatalf("expected 4 disk records, got %d", len(diskRecords))
	}

	var sda, nvme0n1, sdc *unifiedresources.IngestRecord
	for i := range diskRecords {
		record := &diskRecords[i]
		switch record.Resource.Name {
		case "sda":
			sda = record
		case "nvme0n1":
			nvme0n1 = record
		case "sdc":
			sdc = record
		}
	}
	if sda == nil || nvme0n1 == nil || sdc == nil {
		t.Fatalf("expected to find disk records for sda, nvme0n1, and sdc")
	}

	if sda.ParentSourceID != "pool:tank" {
		t.Fatalf("expected sda parent pool:tank, got %q", sda.ParentSourceID)
	}
	if sda.Resource.PhysicalDisk == nil {
		t.Fatal("expected sda PhysicalDiskMeta")
	}
	if sda.Resource.PhysicalDisk.Model != "Seagate Exos X18" {
		t.Fatalf("expected sda model %q, got %q", "Seagate Exos X18", sda.Resource.PhysicalDisk.Model)
	}
	if sda.Resource.PhysicalDisk.Serial != "ZL0A1234" {
		t.Fatalf("expected sda serial %q, got %q", "ZL0A1234", sda.Resource.PhysicalDisk.Serial)
	}
	if sda.Resource.PhysicalDisk.DiskType != "sata" {
		t.Fatalf("expected sda disk type %q, got %q", "sata", sda.Resource.PhysicalDisk.DiskType)
	}
	if sda.Resource.PhysicalDisk.Temperature != 34 {
		t.Fatalf("expected sda temperature 34, got %d", sda.Resource.PhysicalDisk.Temperature)
	}
	if sda.Resource.PhysicalDisk.RPM != 7200 {
		t.Fatalf("expected sda rpm 7200, got %d", sda.Resource.PhysicalDisk.RPM)
	}
	if sda.Identity.MachineID != "ZL0A1234" {
		t.Fatalf("expected sda identity machine ID %q, got %q", "ZL0A1234", sda.Identity.MachineID)
	}
	if len(sda.Identity.Hostnames) != 1 || sda.Identity.Hostnames[0] != "truenas-main" {
		t.Fatalf("expected sda identity hostname [truenas-main], got %v", sda.Identity.Hostnames)
	}

	if nvme0n1.ParentSourceID != "pool:fast" {
		t.Fatalf("expected nvme0n1 parent pool:fast, got %q", nvme0n1.ParentSourceID)
	}
	if nvme0n1.Resource.PhysicalDisk == nil {
		t.Fatal("expected nvme0n1 PhysicalDiskMeta")
	}
	if nvme0n1.Resource.PhysicalDisk.DiskType != "nvme" {
		t.Fatalf("expected nvme0n1 disk type %q, got %q", "nvme", nvme0n1.Resource.PhysicalDisk.DiskType)
	}
	if nvme0n1.Resource.PhysicalDisk.RPM != 0 {
		t.Fatalf("expected nvme0n1 rpm 0, got %d", nvme0n1.Resource.PhysicalDisk.RPM)
	}

	if sdc.Resource.Status != unifiedresources.StatusWarning {
		t.Fatalf("expected sdc status warning, got %s", sdc.Resource.Status)
	}
	if sdc.Resource.PhysicalDisk == nil {
		t.Fatal("expected sdc PhysicalDiskMeta")
	}
	if sdc.Resource.PhysicalDisk.Health != "UNKNOWN" {
		t.Fatalf("expected sdc health UNKNOWN, got %q", sdc.Resource.PhysicalDisk.Health)
	}
	if sdc.Resource.PhysicalDisk.Temperature != 63 {
		t.Fatalf("expected sdc temperature 63, got %d", sdc.Resource.PhysicalDisk.Temperature)
	}
	if sdc.Resource.PhysicalDisk.Risk == nil {
		t.Fatal("expected sdc physical-disk risk")
	}
	foundTemperatureReason := false
	for _, reason := range sdc.Resource.PhysicalDisk.Risk.Reasons {
		if reason.Code == "temperature_high" {
			foundTemperatureReason = true
			break
		}
	}
	if !foundTemperatureReason {
		t.Fatalf("expected sdc physical-disk risk to include temperature_high, got %+v", sdc.Resource.PhysicalDisk.Risk.Reasons)
	}
}

func TestRecordsIncludeTrueNASAppsAsCanonicalWorkloads(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	provider := NewProvider(DefaultFixtures())
	records := provider.Records()
	if len(records) == 0 {
		t.Fatal("expected fixture records from provider")
	}

	var nextcloud *unifiedresources.IngestRecord
	var adguard *unifiedresources.IngestRecord
	for i := range records {
		record := &records[i]
		if record.Resource.Type != unifiedresources.ResourceTypeAppContainer {
			continue
		}
		switch record.Resource.Name {
		case "Nextcloud":
			nextcloud = record
		case "AdGuard Home":
			adguard = record
		}
	}

	if nextcloud == nil || adguard == nil {
		t.Fatalf("expected TrueNAS app-container records for Nextcloud and AdGuard Home")
	}
	if nextcloud.ParentSourceID != "system:truenas-main" {
		t.Fatalf("expected Nextcloud parent system:truenas-main, got %q", nextcloud.ParentSourceID)
	}
	if nextcloud.Resource.Technology != "docker" {
		t.Fatalf("expected Nextcloud technology docker, got %q", nextcloud.Resource.Technology)
	}
	if nextcloud.Resource.Docker == nil {
		t.Fatal("expected Nextcloud docker metadata")
	}
	if nextcloud.Resource.Docker.ContainerID != "nextcloud" {
		t.Fatalf("expected Nextcloud canonical container ID %q, got %q", "nextcloud", nextcloud.Resource.Docker.ContainerID)
	}
	if nextcloud.Resource.Docker.Image != "docker.io/library/nextcloud:29.0.7" {
		t.Fatalf("expected Nextcloud image %q, got %q", "docker.io/library/nextcloud:29.0.7", nextcloud.Resource.Docker.Image)
	}
	if nextcloud.Resource.Docker.Runtime != "docker" {
		t.Fatalf("expected Nextcloud runtime docker, got %q", nextcloud.Resource.Docker.Runtime)
	}
	if len(nextcloud.Resource.Docker.Ports) != 1 || nextcloud.Resource.Docker.Ports[0].PublicPort != 30443 {
		t.Fatalf("unexpected Nextcloud published ports: %+v", nextcloud.Resource.Docker.Ports)
	}
	if len(nextcloud.Resource.Docker.Mounts) != 2 {
		t.Fatalf("expected Nextcloud mounts, got %+v", nextcloud.Resource.Docker.Mounts)
	}
	if nextcloud.Resource.Status != unifiedresources.StatusOnline {
		t.Fatalf("expected Nextcloud status online, got %q", nextcloud.Resource.Status)
	}

	if adguard.Resource.Status != unifiedresources.StatusOffline {
		t.Fatalf("expected AdGuard Home status offline, got %q", adguard.Resource.Status)
	}
	if adguard.Resource.Docker == nil || adguard.Resource.Docker.ContainerState != "exited" {
		t.Fatalf("expected AdGuard Home container state exited, got %+v", adguard.Resource.Docker)
	}
}

func TestRecordsElevateOnlineDiskWhenTemperatureCritical(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	fixtures := DefaultFixtures()
	fixtures.Disks = []Disk{{
		ID:          "disk-hot",
		Name:        "sda",
		Pool:        "tank",
		Status:      "ONLINE",
		Model:       "Seagate Exos X18",
		Serial:      "SER-HOT",
		SizeBytes:   16 * 1024 * 1024 * 1024 * 1024,
		Temperature: 72,
		Transport:   "sata",
		Rotational:  true,
	}}

	records := NewProvider(fixtures).Records()
	if len(records) == 0 {
		t.Fatal("expected fixture records from provider")
	}

	var diskRecord *unifiedresources.IngestRecord
	for i := range records {
		if records[i].Resource.Type == unifiedresources.ResourceTypePhysicalDisk {
			diskRecord = &records[i]
			break
		}
	}
	if diskRecord == nil {
		t.Fatal("expected physical disk record")
	}
	if diskRecord.Resource.Status != unifiedresources.StatusWarning {
		t.Fatalf("expected hot disk status warning, got %s", diskRecord.Resource.Status)
	}
	if diskRecord.Resource.PhysicalDisk == nil || diskRecord.Resource.PhysicalDisk.Risk == nil {
		t.Fatalf("expected hot disk physical risk, got %+v", diskRecord.Resource.PhysicalDisk)
	}
	if diskRecord.Resource.PhysicalDisk.Risk.Level != storagehealth.RiskCritical {
		t.Fatalf("expected hot disk critical risk, got %+v", diskRecord.Resource.PhysicalDisk.Risk)
	}
}

func TestRecordsProjectDiskTemperatureAggregatesIntoCanonicalMetadata(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	provider := NewProvider(DefaultFixtures())
	records := provider.Records()
	if len(records) == 0 {
		t.Fatal("expected fixture records from provider")
	}

	for _, record := range records {
		if record.Resource.Type != unifiedresources.ResourceTypePhysicalDisk || record.Resource.Name != "sda" {
			continue
		}
		if record.Resource.PhysicalDisk == nil {
			t.Fatal("expected canonical physical-disk metadata")
		}
		aggregate := record.Resource.PhysicalDisk.TemperatureAggregate
		if aggregate == nil {
			t.Fatalf("expected temperature aggregate on canonical physical disk: %+v", record.Resource.PhysicalDisk)
		}
		if aggregate.WindowDays != 7 || aggregate.MinCelsius != 29.0 || aggregate.AvgCelsius != 32.7 || aggregate.MaxCelsius != 38.0 {
			t.Fatalf("unexpected canonical disk temperature aggregate: %+v", aggregate)
		}
		return
	}

	t.Fatal("expected sda physical disk record")
}

func TestProviderPhysicalDiskTemperatureHistoryUsesCanonicalMetricIDs(t *testing.T) {
	fixtures := DefaultFixtures()
	now := time.Date(2026, 3, 29, 20, 0, 0, 0, time.UTC)
	fetcher := &controllableStubFetcher{
		snapshot: &fixtures,
		diskHistory: map[string][]TimeSeriesPoint{
			"sda": {
				{Timestamp: now.Add(-2 * time.Hour), Value: 30},
				{Timestamp: now.Add(-1 * time.Hour), Value: 32},
				{Timestamp: now, Value: 34},
			},
		},
	}
	provider := NewLiveProvider(fetcher)
	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	history, err := provider.PhysicalDiskTemperatureHistory(context.Background(), 4*time.Hour)
	if err != nil {
		t.Fatalf("PhysicalDiskTemperatureHistory() error = %v", err)
	}
	points, ok := history["ZL0A1234"]
	if !ok {
		t.Fatalf("expected canonical disk metric id ZL0A1234, got keys %#v", history)
	}
	if len(points) != 3 {
		t.Fatalf("expected 3 temperature history points, got %+v", points)
	}
	if points[len(points)-1].Value != 34 {
		t.Fatalf("expected latest point value 34, got %+v", points)
	}
}
