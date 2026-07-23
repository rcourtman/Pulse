package vmware

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestBranchcov0723Am_FixtureFetcherFetch covers (f *FixtureFetcher) Fetch in
// provider.go.
//
// NOTE: the spec described FixtureFetcher.Fetch as reading a fixture file
// (missing path / invalid JSON / empty file). The actual implementation does
// NO file I/O — it returns a defensive deep copy of the in-memory Snapshot
// field, with a single nil-receiver guard. These subtests drive that real
// behaviour: both arms of the nil-receiver guard, and real clone semantics
// (the returned snapshot equals the source by value; mutating a returned
// slice does not bleed back into the fetcher; two Fetch calls return
// independent slices). The spec's "missing path / invalid JSON / empty file"
// arms do not exist in the code and cannot be covered.
func TestBranchcov0723Am_FixtureFetcherFetch(t *testing.T) {
	t.Run("nil_receiver_returns_nil_nil", func(t *testing.T) {
		var f *FixtureFetcher
		got, err := f.Fetch(context.Background())
		if err != nil {
			t.Fatalf("nil-receiver Fetch err = %v, want nil", err)
		}
		if got != nil {
			t.Fatalf("nil-receiver Fetch = %v, want nil", got)
		}
	})

	t.Run("empty_snapshot_returns_non_nil_clone", func(t *testing.T) {
		f := &FixtureFetcher{}
		got, err := f.Fetch(context.Background())
		if err != nil {
			t.Fatalf("Fetch err = %v, want nil", err)
		}
		if got == nil {
			t.Fatal("Fetch returned nil snapshot for non-nil receiver; want non-nil empty clone")
		}
		if len(got.Hosts) != 0 || len(got.VMs) != 0 || len(got.Datastores) != 0 || len(got.Networks) != 0 {
			t.Fatalf("empty fixture produced non-empty snapshot: %+v", got)
		}
	})

	populated := InventorySnapshot{
		ConnectionID:   "vc-1",
		ConnectionName: "Lab VC",
		VCenterHost:    "vc.lab.local",
		VIRelease:      "8.0.3",
		CollectedAt:    time.Date(2026, time.July, 23, 9, 0, 0, 0, time.UTC),
		Hosts: []InventoryHost{{
			Host:            "host-1",
			Name:            "esxi-1.lab.local",
			ConnectionState: "CONNECTED",
			PowerState:      "POWERED_ON",
			DatastoreIDs:    []string{"ds-1"},
		}},
		VMs: []InventoryVM{{
			VM:         "vm-1",
			Name:       "app-1",
			PowerState: "POWERED_ON",
		}},
		Datastores: []InventoryDatastore{{
			Datastore: "ds-1",
			Name:      "nvme-primary",
			Type:      "VMFS",
		}},
	}

	t.Run("populated_snapshot_round_trips_fields", func(t *testing.T) {
		f := &FixtureFetcher{Snapshot: populated}
		got, err := f.Fetch(context.Background())
		if err != nil {
			t.Fatalf("Fetch err = %v, want nil", err)
		}
		if got == nil {
			t.Fatal("Fetch returned nil snapshot; want clone")
		}
		if got.ConnectionID != populated.ConnectionID {
			t.Errorf("ConnectionID = %q, want %q", got.ConnectionID, populated.ConnectionID)
		}
		if got.ConnectionName != populated.ConnectionName {
			t.Errorf("ConnectionName = %q, want %q", got.ConnectionName, populated.ConnectionName)
		}
		if got.VCenterHost != populated.VCenterHost {
			t.Errorf("VCenterHost = %q, want %q", got.VCenterHost, populated.VCenterHost)
		}
		if got.VIRelease != populated.VIRelease {
			t.Errorf("VIRelease = %q, want %q", got.VIRelease, populated.VIRelease)
		}
		if !got.CollectedAt.Equal(populated.CollectedAt) {
			t.Errorf("CollectedAt = %v, want %v", got.CollectedAt, populated.CollectedAt)
		}
		if len(got.Hosts) != 1 || got.Hosts[0].Host != "host-1" {
			t.Errorf("Hosts = %+v, want one entry with Host=host-1", got.Hosts)
		}
		if len(got.VMs) != 1 || got.VMs[0].VM != "vm-1" {
			t.Errorf("VMs = %+v, want one entry with VM=vm-1", got.VMs)
		}
		if len(got.Datastores) != 1 || got.Datastores[0].Datastore != "ds-1" {
			t.Errorf("Datastores = %+v, want one entry with Datastore=ds-1", got.Datastores)
		}
	})

	t.Run("returned_snapshot_is_a_defensive_copy", func(t *testing.T) {
		f := &FixtureFetcher{Snapshot: populated}
		got, err := f.Fetch(context.Background())
		if err != nil {
			t.Fatalf("Fetch err = %v, want nil", err)
		}
		// Mutate the returned snapshot's slice and scalar fields. Because
		// Fetch returns a deep clone, the fetcher's underlying snapshot must
		// remain unchanged for subsequent calls.
		got.Hosts[0].Host = "MUTATED"
		got.Hosts = append(got.Hosts, InventoryHost{Host: "host-2"})
		got.VMs = nil
		got.CollectedAt = time.Time{}

		again, err := f.Fetch(context.Background())
		if err != nil {
			t.Fatalf("second Fetch err = %v, want nil", err)
		}
		if len(again.Hosts) != 1 || again.Hosts[0].Host != "host-1" {
			t.Fatalf("mutation bled into fetcher: again.Hosts = %+v, want one entry with Host=host-1", again.Hosts)
		}
		if len(again.VMs) != 1 {
			t.Fatalf("mutation bled into fetcher: again.VMs = %+v, want one entry", again.VMs)
		}
		if !again.CollectedAt.Equal(populated.CollectedAt) {
			t.Fatalf("mutation bled into fetcher: again.CollectedAt = %v, want %v", again.CollectedAt, populated.CollectedAt)
		}
	})

	t.Run("two_consecutive_calls_return_independent_slices", func(t *testing.T) {
		f := &FixtureFetcher{Snapshot: populated}
		first, err := f.Fetch(context.Background())
		if err != nil {
			t.Fatalf("first Fetch err = %v, want nil", err)
		}
		second, err := f.Fetch(context.Background())
		if err != nil {
			t.Fatalf("second Fetch err = %v, want nil", err)
		}
		first.Hosts[0].Host = "FIRST-MUT"
		if second.Hosts[0].Host == "FIRST-MUT" {
			t.Fatal("second Fetch result shares storage with first; expected independent clones")
		}
		if first.Hosts[0].Host != "FIRST-MUT" {
			t.Fatalf("mutating first result did not stick: got %q", first.Hosts[0].Host)
		}
	})
}

// TestBranchcov0723Am_ResetFeatureEnabledFromEnv covers
// ResetFeatureEnabledFromEnv in client.go. The function re-reads the
// PULSE_ENABLE_VMWARE env var, re-parses it through parseFeatureEnabled, and
// stores the result in package-level state (featureVMwareEnabled). The four
// subtests below are the spec-required inputs (enabled, disabled, unset,
// garbage) and collectively cover every case body of parseFeatureEnabled
// (true-arm, false-arm, default-arm) plus the env-unset path through
// os.Getenv. Each subtest restores both the previous flag value and the
// previous environment via t.Cleanup so no other test observes the mutation.
func TestBranchcov0723Am_ResetFeatureEnabledFromEnv(t *testing.T) {
	cases := []struct {
		name  string
		value string
		unset bool
		want  bool
	}{
		// parseFeatureEnabled true-arm.
		{name: "enabled_token_enables", value: "1", want: true},
		// parseFeatureEnabled false-arm.
		{name: "disabled_token_disables", value: "false", want: false},
		// os.Getenv returns "" when unset; parseFeatureEnabled("") hits the
		// true-arm. Semantically distinct from value="1" because the env var
		// is genuinely absent.
		{name: "unset_env_defaults_to_enabled", unset: true, want: true},
		// parseFeatureEnabled default-arm.
		{name: "garbage_value_defaults_to_enabled", value: "maybe", want: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			previous := IsFeatureEnabled()
			t.Cleanup(func() { SetFeatureEnabled(previous) })

			if tc.unset {
				prevValue, hadPrev := os.LookupEnv(FeatureVMware)
				if err := os.Unsetenv(FeatureVMware); err != nil {
					t.Fatalf("os.Unsetenv: %v", err)
				}
				t.Cleanup(func() {
					if !hadPrev {
						return
					}
					if err := os.Setenv(FeatureVMware, prevValue); err != nil {
						t.Logf("restore os.Setenv: %v", err)
					}
				})
			} else {
				t.Setenv(FeatureVMware, tc.value)
			}

			ResetFeatureEnabledFromEnv()
			if got := IsFeatureEnabled(); got != tc.want {
				t.Fatalf("IsFeatureEnabled() after ResetFeatureEnabledFromEnv(value=%q, unset=%v) = %v, want %v",
					tc.value, tc.unset, got, tc.want)
			}
		})
	}
}

// stubFetchSnapshot is a deterministic Fetcher used to drive the error and
// nil-snapshot arms of (*Provider).Refresh without touching a real client.
type stubFetchSnapshot struct {
	snapshot *InventorySnapshot
	err      error
}

func (s *stubFetchSnapshot) Fetch(context.Context) (*InventorySnapshot, error) {
	return s.snapshot, s.err
}

// stubCloseFetch is a Fetcher that also implements fetcherCloser (unexported,
// same package) so (*Provider).Close's type-assertion arm is reachable and
// observable through the closes counter.
type stubCloseFetch struct {
	closes atomic.Int32
}

func (s *stubCloseFetch) Fetch(context.Context) (*InventorySnapshot, error) {
	return &InventorySnapshot{}, nil
}

func (s *stubCloseFetch) Close() { s.closes.Add(1) }

// stubPlainFetch is a Fetcher without a Close method, used to drive the
// type-assertion-fail (non-closer) arm of (*Provider).Close.
type stubPlainFetch struct{}

func (stubPlainFetch) Fetch(context.Context) (*InventorySnapshot, error) {
	return &InventorySnapshot{}, nil
}

// TestBranchcov0723Am_ProviderRefresh covers (*Provider).Refresh in
// provider.go. It drives all five return paths through deterministic stub
// fetchers (no vCenter dial): nil receiver, nil fetcher, wrapped fetch error,
// nil-snapshot error, and the success path which caches and sorts the
// snapshot.
func TestBranchcov0723Am_ProviderRefresh(t *testing.T) {
	ctx := context.Background()

	t.Run("nil_receiver_returns_provider_nil_error", func(t *testing.T) {
		var p *Provider
		err := p.Refresh(ctx)
		if err == nil {
			t.Fatal("nil-receiver Refresh err = nil, want error")
		}
		if !strings.Contains(err.Error(), "provider is nil") {
			t.Errorf("nil-receiver Refresh err = %q, want it to mention 'provider is nil'", err.Error())
		}
	})

	t.Run("nil_fetcher_returns_fetcher_nil_error", func(t *testing.T) {
		p := &Provider{}
		err := p.Refresh(ctx)
		if err == nil {
			t.Fatal("nil-fetcher Refresh err = nil, want error")
		}
		if !strings.Contains(err.Error(), "fetcher is nil") {
			t.Errorf("nil-fetcher Refresh err = %q, want it to mention 'fetcher is nil'", err.Error())
		}
	})

	sentinel := errors.New("stub fetch failure")
	t.Run("fetcher_error_is_wrapped_with_refresh_prefix", func(t *testing.T) {
		p := NewLiveProvider(&stubFetchSnapshot{err: sentinel})
		err := p.Refresh(ctx)
		if err == nil {
			t.Fatal("Refresh err = nil, want wrapped error")
		}
		if !errors.Is(err, sentinel) {
			t.Errorf("Refresh err = %q, want it to wrap %v via %%w", err.Error(), sentinel)
		}
		if !strings.Contains(err.Error(), "refresh vmware inventory") {
			t.Errorf("Refresh err = %q, want it to mention 'refresh vmware inventory'", err.Error())
		}
	})

	t.Run("fetcher_returns_nil_snapshot_returns_nil_inventory_error", func(t *testing.T) {
		p := NewLiveProvider(&stubFetchSnapshot{snapshot: nil})
		err := p.Refresh(ctx)
		if err == nil {
			t.Fatal("Refresh err = nil, want nil-inventory error")
		}
		if !strings.Contains(err.Error(), "nil inventory") {
			t.Errorf("Refresh err = %q, want it to mention 'nil inventory'", err.Error())
		}
	})

	t.Run("valid_snapshot_is_cached_and_sorted", func(t *testing.T) {
		// Provide hosts in reverse sort order; Refresh calls
		// sortInventorySnapshot, so the cached snapshot must come back sorted
		// by vmwareSortKey (firstNonEmptyTrimmed of lower-cased id then name).
		snapshot := &InventorySnapshot{
			ConnectionID: "vc-1",
			Hosts: []InventoryHost{
				{Host: "host-9", Name: "esxi-9"},
				{Host: "host-1", Name: "esxi-1"},
			},
		}
		p := NewLiveProvider(&stubFetchSnapshot{snapshot: snapshot})
		if err := p.Refresh(ctx); err != nil {
			t.Fatalf("Refresh err = %v, want nil", err)
		}
		cached := p.Snapshot()
		if cached == nil {
			t.Fatal("Snapshot() = nil after successful Refresh; want cached snapshot")
		}
		if len(cached.Hosts) != 2 {
			t.Fatalf("cached Hosts len = %d, want 2", len(cached.Hosts))
		}
		if got := cached.Hosts[0].Host; got != "host-1" {
			t.Errorf("Refresh did not sort cached hosts: cached.Hosts[0].Host = %q, want host-1", got)
		}
	})
}

// TestBranchcov0723Am_ProviderClose covers (*Provider).Close in provider.go.
// It drives the nil-receiver guard, the nil-fetcher guard, the
// type-assertion-fail arm (a fetcher without a Close method), and the
// type-assertion-success arm observed through the stub's closes counter.
func TestBranchcov0723Am_ProviderClose(t *testing.T) {
	t.Run("nil_receiver_is_a_documented_noop", func(t *testing.T) {
		var p *Provider
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("(*Provider).Close on nil receiver panicked: %v", r)
			}
		}()
		p.Close()
	})

	t.Run("nil_fetcher_is_a_documented_noop", func(t *testing.T) {
		p := &Provider{}
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("(*Provider).Close with nil fetcher panicked: %v", r)
			}
		}()
		p.Close()
	})

	t.Run("non_closer_fetcher_skips_close", func(t *testing.T) {
		// stubPlainFetch has no Close method, so the fetcherCloser type
		// assertion fails and Close must be a no-op rather than panicking.
		p := NewLiveProvider(stubPlainFetch{})
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("(*Provider).Close on non-closer fetcher panicked: %v", r)
			}
		}()
		p.Close()
	})

	t.Run("closer_fetcher_close_invoked_exactly_once", func(t *testing.T) {
		closer := &stubCloseFetch{}
		p := NewLiveProvider(closer)
		p.Close()
		if got := closer.closes.Load(); got != 1 {
			t.Fatalf("Provider.Close proxied %d calls to fetcher.Close, want exactly 1", got)
		}
	})
}

// TestBranchcov0723Am_NewAPIProvider covers NewAPIProvider in provider.go.
// It asserts the returned Provider wraps the supplied client and metadata in
// an *APIFetcher (field plumbing is the documented behaviour of the
// constructor), and that the resulting fetcher also satisfies fetcherCloser
// so Provider.Close proxies through to Client.Close.
func TestBranchcov0723Am_NewAPIProvider(t *testing.T) {
	client, err := NewClient(ClientConfig{Host: "vc.example.com", Port: 443})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	meta := ProviderMetadata{
		ConnectionID:   "vc-1",
		ConnectionName: "Lab VC",
		VCenterHost:    "vc.lab.local",
	}
	p := NewAPIProvider(meta, client)

	apiFetch, ok := p.fetcher.(*APIFetcher)
	if !ok {
		t.Fatalf("p.fetcher type = %T, want *APIFetcher", p.fetcher)
	}
	if apiFetch.Client != client {
		t.Errorf("APIFetcher.Client = %p, want the exact client passed to NewAPIProvider (%p)", apiFetch.Client, client)
	}
	if apiFetch.Metadata != meta {
		t.Errorf("APIFetcher.Metadata = %+v, want %+v", apiFetch.Metadata, meta)
	}

	// The wrapped *APIFetcher has a Close method, so (*Provider).Close must
	// proxy through to Client.Close without panicking. (Client.Close on a
	// never-used client is a benign drain of an empty idle pool.)
	p.Close()
}

// TestBranchcov0723Am_APIFetcherFetchErrorArm covers (*APIFetcher).Fetch in
// provider.go. Only the deterministic error arm (f == nil || f.Client == nil)
// is reachable offline; the success arm calls Client.CollectInventory which
// dials a live vCenter and is skipped per the spec's purity gate.
func TestBranchcov0723Am_APIFetcherFetchErrorArm(t *testing.T) {
	t.Run("nil_client_returns_client_nil_error", func(t *testing.T) {
		f := &APIFetcher{}
		got, err := f.Fetch(context.Background())
		if got != nil {
			t.Errorf("nil-client Fetch = %v, want nil", got)
		}
		if err == nil {
			t.Fatal("nil-client Fetch err = nil, want error")
		}
		if !strings.Contains(err.Error(), "client is nil") {
			t.Errorf("nil-client Fetch err = %q, want it to mention 'client is nil'", err.Error())
		}
	})

	t.Run("nil_receiver_returns_client_nil_error", func(t *testing.T) {
		// Distinct from nil-Client: this exercises Go's method-on-nil-pointer
		// resolution plus the f == nil short-circuit in the guard.
		var f *APIFetcher
		got, err := f.Fetch(context.Background())
		if got != nil {
			t.Errorf("nil-receiver Fetch = %v, want nil", got)
		}
		if err == nil {
			t.Fatal("nil-receiver Fetch err = nil, want error")
		}
		if !strings.Contains(err.Error(), "client is nil") {
			t.Errorf("nil-receiver Fetch err = %q, want it to mention 'client is nil'", err.Error())
		}
	})
}

// newCloseCountingClient constructs a *Client pointed at a loopback TLS test
// server that counts accepted TCP connections (via http.Server.ConnState on
// StateNew). The returned doReq closure issues a GET against the server
// through the client's own http client, so callers can observe whether the
// client reused an idle connection (accept count unchanged) or dialled a new
// one (accept count incremented). This is offline (127.0.0.1 only) and does
// not dial any vCenter. The server is cleaned up via t.Cleanup.
func newCloseCountingClient(t *testing.T) (client *Client, doReq func(), accepted func() int32) {
	t.Helper()
	var count atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewUnstartedServer(handler)
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			count.Add(1)
		}
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	host := strings.TrimPrefix(srv.URL, "https://")
	c, err := NewClient(ClientConfig{
		Host:               host,
		InsecureSkipVerify: true,
	})
	if err != nil {
		t.Fatalf("NewClient(%q): %v", host, err)
	}

	doReq = func() {
		t.Helper()
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://"+c.baseURL.Host+"/", nil)
		if err != nil {
			t.Fatalf("http.NewRequestWithContext: %v", err)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			t.Fatalf("httpClient.Do: %v", err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	return c, doReq, func() int32 { return count.Load() }
}

// waitForAcceptCount polls the accepted-connection count for up to 2 seconds
// so the test does not race the server-side ConnState callback (which fires
// from the server's goroutine on each new connection).
func waitForAcceptCount(t *testing.T, accepted func() int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if accepted() >= want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("accepted connection count = %d, want >= %d", accepted(), want)
}

// TestBranchcov0723Am_ClientClose covers (*Client).Close in client.go. The
// meaningful, observable arm — the (*http.Transport) type assertion
// succeeding and calling transport.CloseIdleConnections — is asserted
// behaviourally via a loopback TLS server whose ConnState callback counts
// accepted TCP connections: after Close, a follow-up request must open a
// fresh connection rather than reuse a pooled idle one. The nil-receiver and
// nil-httpClient guard arms are covered as documented nil-safety contracts
// (explicit recover-based assertions).
func TestBranchcov0723Am_ClientClose(t *testing.T) {
	t.Run("nil_receiver_is_a_documented_noop", func(t *testing.T) {
		var c *Client
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("(*Client).Close on nil receiver panicked: %v", r)
			}
		}()
		c.Close()
	})

	t.Run("nil_http_client_short_circuits", func(t *testing.T) {
		c := &Client{}
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("(*Client).Close with nil httpClient panicked: %v", r)
			}
		}()
		c.Close()
	})

	t.Run("transport_arm_drains_idle_connections", func(t *testing.T) {
		client, doReq, accepted := newCloseCountingClient(t)

		// First request opens connection #1.
		doReq()
		waitForAcceptCount(t, accepted, 1)

		// Second request must reuse the pooled idle connection rather than
		// opening a new one: the accept count must stay at 1.
		doReq()
		waitForAcceptCount(t, accepted, 1)
		if got := accepted(); got != 1 {
			t.Fatalf("idle conn was not reused: server accepted %d conns after two requests, want 1", got)
		}

		client.Close() // drain the idle pool

		// Third request must dial a fresh connection because Close reaped the
		// idle one: the accept count must climb to 2.
		doReq()
		waitForAcceptCount(t, accepted, 2)
		if got := accepted(); got != 2 {
			t.Fatalf("Close did not drain idle pool: third request reused a pooled conn (accepted=%d), want a fresh conn (2)", got)
		}
	})
}

// TestBranchcov0723Am_APIFetcherClose covers (*APIFetcher).Close in
// provider.go. The proxies-to-Client.Close arm is asserted behaviourally with
// the same loopback TLS server machinery used for (*Client).Close, proving
// the call chains down to transport.CloseIdleConnections. The nil-receiver
// and nil-Client guard arms are covered as documented nil-safety contracts.
func TestBranchcov0723Am_APIFetcherClose(t *testing.T) {
	t.Run("nil_receiver_is_a_documented_noop", func(t *testing.T) {
		var f *APIFetcher
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("(*APIFetcher).Close on nil receiver panicked: %v", r)
			}
		}()
		f.Close()
	})

	t.Run("nil_client_short_circuits", func(t *testing.T) {
		f := &APIFetcher{}
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("(*APIFetcher).Close with nil Client panicked: %v", r)
			}
		}()
		f.Close()
	})

	t.Run("proxies_through_to_client_close_draining_idle_pool", func(t *testing.T) {
		client, doReq, accepted := newCloseCountingClient(t)
		f := &APIFetcher{Client: client}

		doReq()
		waitForAcceptCount(t, accepted, 1)
		doReq()
		if got := accepted(); got != 1 {
			t.Fatalf("idle conn was not reused: server accepted %d conns after two requests, want 1", got)
		}

		f.Close() // must chain through to Client.Close -> transport.CloseIdleConnections

		doReq()
		waitForAcceptCount(t, accepted, 2)
		if got := accepted(); got != 2 {
			t.Fatalf("APIFetcher.Close did not drain idle pool through Client.Close: third request reused a pooled conn (accepted=%d), want a fresh conn (2)", got)
		}
	})
}
