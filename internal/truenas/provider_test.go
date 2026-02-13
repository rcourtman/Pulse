package truenas

import (
	"context"
	"errors"
	"net/http"
	"testing"
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
