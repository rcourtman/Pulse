package truenas

import (
	"context"
	"errors"
	"net/http"
	"testing"

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
	if system.Resource.Type != unifiedresources.ResourceTypeHost {
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
}
