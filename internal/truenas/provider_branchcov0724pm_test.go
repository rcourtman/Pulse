package truenas

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// branchcov0724pmTransportFetcher is a Fetcher that also satisfies
// transportStatusFetcher, so Provider.TransportStatus can exercise its
// delegation arm without standing up a live Client.
type branchcov0724pmTransportFetcher struct {
	status TransportStatus
}

func (f *branchcov0724pmTransportFetcher) Fetch(context.Context) (*FixtureSnapshot, error) {
	return nil, nil
}

func (f *branchcov0724pmTransportFetcher) TransportStatus() TransportStatus {
	return f.status
}

// TestBranchcov0724pmIncidentFromPoolStatus exercises every pool status string
// that incidentsFromPoolHealth maps onto a "zfs_pool_state" incident plus both
// no-incident arms: an incident-free healthy pool, and a pool whose only
// incidents carry a different code (so the helper must not surface them).
func TestBranchcov0724pmIncidentFromPoolStatus(t *testing.T) {
	observedAt := time.Date(2026, 7, 24, 13, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		pool          Pool
		wantOK        bool
		wantSeverity  storagehealth.RiskLevel
		wantNativeID  string
		wantSource    string
		wantSummary   string
		wantStartedAt time.Time
		wantConfirms  int
		wantRecovers  int
		wantProvider  string
	}{
		{
			name:   "DEGRADED maps to warning state incident",
			pool:   Pool{Name: "tank", Status: "DEGRADED"},
			wantOK: true, wantSeverity: storagehealth.RiskWarning,
			wantNativeID: "pool:tank:state", wantSource: "pool.query",
			wantSummary: "ZFS pool tank is DEGRADED", wantStartedAt: observedAt,
			wantConfirms: 2, wantRecovers: 2, wantProvider: "truenas",
		},
		{
			// Status is trimmed+upper-cased inside incidentsFromPoolHealth.
			name:   "lowercase trimmed degraded still maps to state incident",
			pool:   Pool{Name: "tank", Status: "  degraded  "},
			wantOK: true, wantSeverity: storagehealth.RiskWarning,
			wantNativeID: "pool:tank:state", wantSource: "pool.query",
			wantSummary: "ZFS pool tank is DEGRADED", wantStartedAt: observedAt,
		},
		{
			name:   "FAULTED maps to critical state incident",
			pool:   Pool{Name: "tank", Status: "FAULTED"},
			wantOK: true, wantSeverity: storagehealth.RiskCritical,
			wantNativeID: "pool:tank:state", wantSource: "pool.query",
			wantSummary: "ZFS pool tank is FAULTED", wantStartedAt: observedAt,
		},
		{
			name:   "OFFLINE maps to critical state incident",
			pool:   Pool{Name: "tank", Status: "OFFLINE"},
			wantOK: true, wantSeverity: storagehealth.RiskCritical,
			wantNativeID: "pool:tank:state", wantSource: "pool.query",
			wantSummary: "ZFS pool tank is OFFLINE", wantStartedAt: observedAt,
		},
		{
			name:   "REMOVED maps to critical state incident",
			pool:   Pool{Name: "tank", Status: "REMOVED"},
			wantOK: true, wantSeverity: storagehealth.RiskCritical,
			wantNativeID: "pool:tank:state", wantSource: "pool.query",
			wantSummary: "ZFS pool tank is REMOVED", wantStartedAt: observedAt,
		},
		{
			name:   "UNAVAIL maps to critical state incident",
			pool:   Pool{Name: "tank", Status: "UNAVAIL"},
			wantOK: true, wantSeverity: storagehealth.RiskCritical,
			wantNativeID: "pool:tank:state", wantSource: "pool.query",
			wantSummary: "ZFS pool tank is UNAVAIL", wantStartedAt: observedAt,
		},
		{
			name:   "SUSPENDED maps to critical state incident",
			pool:   Pool{Name: "tank", Status: "SUSPENDED"},
			wantOK: true, wantSeverity: storagehealth.RiskCritical,
			wantNativeID: "pool:tank:state", wantSource: "pool.query",
			wantSummary: "ZFS pool tank is SUSPENDED", wantStartedAt: observedAt,
		},
		{
			// A boot pool sources its state incident from boot.get_state.
			name:   "boot pool DEGRADED uses boot.get_state source",
			pool:   Pool{Name: "boot-pool", Status: "DEGRADED", IsBoot: true},
			wantOK: true, wantSeverity: storagehealth.RiskWarning,
			wantNativeID: "pool:boot-pool:state", wantSource: "boot.get_state",
			wantSummary: "ZFS pool boot-pool is DEGRADED", wantStartedAt: observedAt,
		},
		{
			// incidentsFromPoolHealth returns nil for an empty pool name.
			name:   "empty pool name yields no state incident",
			pool:   Pool{Name: "   ", Status: "DEGRADED"},
			wantOK: false,
		},
		{
			// Healthy ONLINE pool with nothing else going on: no incidents at all.
			name:   "healthy ONLINE pool yields no incident",
			pool:   Pool{Name: "tank", Status: "ONLINE"},
			wantOK: false,
		},
		{
			// ONLINE pool with read errors still produces incidents, but their
			// code is zfs_pool_errors (not zfs_pool_state), so incidentFromPoolStatus
			// must NOT surface them and must report no state incident.
			name:   "pool errors do not surface as a state incident",
			pool:   Pool{Name: "tank", Status: "ONLINE", ReadErrors: 3},
			wantOK: false,
		},
		{
			// An unrecognized status falls through the switch to no state incident.
			name:   "unknown status yields no state incident",
			pool:   Pool{Name: "tank", Status: "SOMEFUTURESTATE"},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incident, ok := incidentFromPoolStatus(tt.pool, observedAt)
			if ok != tt.wantOK {
				t.Fatalf("incidentFromPoolStatus(%+v) ok = %v, want %v", tt.pool, ok, tt.wantOK)
			}
			if !tt.wantOK {
				// The false arm must return the zero-valued incident.
				if incident != (unifiedresources.ResourceIncident{}) {
					t.Fatalf("incidentFromPoolStatus(%+v) incident = %+v, want zero value when ok is false", tt.pool, incident)
				}
				return
			}
			if incident.Code != "zfs_pool_state" {
				t.Fatalf("incidentFromPoolStatus(%+v) Code = %q, want %q", tt.pool, incident.Code, "zfs_pool_state")
			}
			if incident.Severity != tt.wantSeverity {
				t.Fatalf("incidentFromPoolStatus(%+v) Severity = %q, want %q", tt.pool, incident.Severity, tt.wantSeverity)
			}
			if incident.NativeID != tt.wantNativeID {
				t.Fatalf("incidentFromPoolStatus(%+v) NativeID = %q, want %q", tt.pool, incident.NativeID, tt.wantNativeID)
			}
			if incident.Source != tt.wantSource {
				t.Fatalf("incidentFromPoolStatus(%+v) Source = %q, want %q", tt.pool, incident.Source, tt.wantSource)
			}
			if incident.Summary != tt.wantSummary {
				t.Fatalf("incidentFromPoolStatus(%+v) Summary = %q, want %q", tt.pool, incident.Summary, tt.wantSummary)
			}
			if !incident.StartedAt.Equal(tt.wantStartedAt) {
				t.Fatalf("incidentFromPoolStatus(%+v) StartedAt = %v, want %v", tt.pool, incident.StartedAt, tt.wantStartedAt)
			}
			if tt.wantConfirms != 0 && incident.ConfirmationsRequired != tt.wantConfirms {
				t.Fatalf("incidentFromPoolStatus(%+v) ConfirmationsRequired = %d, want %d", tt.pool, incident.ConfirmationsRequired, tt.wantConfirms)
			}
			if tt.wantRecovers != 0 && incident.RecoveryConfirmationsRequired != tt.wantRecovers {
				t.Fatalf("incidentFromPoolStatus(%+v) RecoveryConfirmationsRequired = %d, want %d", tt.pool, incident.RecoveryConfirmationsRequired, tt.wantRecovers)
			}
			if tt.wantProvider != "" && incident.Provider != tt.wantProvider {
				t.Fatalf("incidentFromPoolStatus(%+v) Provider = %q, want %q", tt.pool, incident.Provider, tt.wantProvider)
			}
		})
	}
}

// TestBranchcov0724pmIncidentFromPoolStatusStatePrecedesErrors confirms the
// helper returns the zfs_pool_state incident and not the zfs_pool_errors
// incident when both are present (a DEGRADED pool that also reports errors),
// i.e. it selects on code rather than returning the first projection.
func TestBranchcov0724pmIncidentFromPoolStatusStatePrecedesErrors(t *testing.T) {
	observedAt := time.Date(2026, 7, 24, 13, 0, 0, 0, time.UTC)
	pool := Pool{
		Name:           "tank",
		Status:         "DEGRADED",
		ReadErrors:     5,
		WriteErrors:    2,
		ChecksumErrors: 7,
	}
	incident, ok := incidentFromPoolStatus(pool, observedAt)
	if !ok {
		t.Fatalf("incidentFromPoolStatus(%+v) ok = false, want true", pool)
	}
	if incident.Code != "zfs_pool_state" {
		t.Fatalf("incidentFromPoolStatus(%+v) Code = %q, want zfs_pool_state (not the error incident)", pool, incident.Code)
	}
	if incident.Severity != storagehealth.RiskWarning {
		t.Fatalf("incidentFromPoolStatus(%+v) Severity = %q, want warning (the state incident severity)", pool, incident.Severity)
	}
	if incident.Summary != "ZFS pool tank is DEGRADED" {
		t.Fatalf("incidentFromPoolStatus(%+v) Summary = %q, want the state incident summary", pool, incident.Summary)
	}
}

// TestBranchcov0724pmRecordsFromSnapshot covers every arm of
// Provider.RecordsFromSnapshot: nil receiver, feature-flag disabled, nil
// snapshot, an empty snapshot (only the system record), and a fully populated
// snapshot projected through a connection-scoped identity.
func TestBranchcov0724pmRecordsFromSnapshot(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() { SetFeatureEnabled(previous) })

	t.Run("nil receiver returns nil", func(t *testing.T) {
		var p *Provider
		snapshot := DefaultFixtures()
		if got := p.RecordsFromSnapshot(&snapshot); got != nil {
			t.Fatalf("nil provider RecordsFromSnapshot = %d records, want nil", len(got))
		}
	})

	t.Run("feature disabled returns nil even with a snapshot", func(t *testing.T) {
		saved := IsFeatureEnabled()
		SetFeatureEnabled(false)
		defer SetFeatureEnabled(saved)
		p := NewProvider(DefaultFixtures())
		snapshot := DefaultFixtures()
		if got := p.RecordsFromSnapshot(&snapshot); got != nil {
			t.Fatalf("disabled-feature RecordsFromSnapshot = %d records, want nil", len(got))
		}
	})

	t.Run("nil snapshot returns nil", func(t *testing.T) {
		p := NewProvider(DefaultFixtures())
		if got := p.RecordsFromSnapshot(nil); got != nil {
			t.Fatalf("RecordsFromSnapshot(nil) = %d records, want nil", len(got))
		}
	})

	t.Run("empty snapshot yields exactly the system agent record", func(t *testing.T) {
		p := NewProvider(FixtureSnapshot{})
		empty := FixtureSnapshot{}
		records := p.RecordsFromSnapshot(&empty)
		if len(records) != 1 {
			t.Fatalf("RecordsFromSnapshot(empty) = %d records, want exactly 1 (system)", len(records))
		}
		system := records[0]
		if system.Resource.Type != unifiedresources.ResourceTypeAgent {
			t.Fatalf("empty-snapshot record type = %s, want %s", system.Resource.Type, unifiedresources.ResourceTypeAgent)
		}
		// Fixture provider carries no connection, so the system is hostname-scoped;
		// with an empty hostname that collapses to "system:".
		if system.SourceID != "system:" {
			t.Fatalf("empty-snapshot SourceID = %q, want %q", system.SourceID, "system:")
		}
		// Mutating the returned slice must not corrupt a fresh projection.
		records[0].SourceID = "tampered"
		again := p.RecordsFromSnapshot(&empty)
		if again[0].SourceID != "system:" {
			t.Fatalf("RecordsFromSnapshot returned shared slice state: got %q after tampering", again[0].SourceID)
		}
	})

	t.Run("fully populated snapshot is projected with connection-scoped identity", func(t *testing.T) {
		fixtures := DefaultFixtures()
		p := NewLiveProviderForConnection(&FixtureFetcher{Snapshot: fixtures}, "conn-0724pm")
		snapshot := fixtures
		records := p.RecordsFromSnapshot(&snapshot)
		if len(records) == 0 {
			t.Fatal("RecordsFromSnapshot(fully populated) returned no records")
		}

		byType := make(map[unifiedresources.ResourceType]int)
		var systemRecord *unifiedresources.IngestRecord
		for i := range records {
			byType[records[i].Resource.Type]++
			if records[i].Resource.Type == unifiedresources.ResourceTypeAgent {
				systemRecord = &records[i]
			}
		}
		// Concrete per-type counts derived from DefaultFixtures: 1 system, 4 pools
		// + 9 datasets = 13 storage, 5 app containers, 2 VMs, 3 shares, 8 disks.
		wantByType := map[unifiedresources.ResourceType]int{
			unifiedresources.ResourceTypeAgent:        1,
			unifiedresources.ResourceTypeStorage:      13,
			unifiedresources.ResourceTypeAppContainer: 5,
			unifiedresources.ResourceTypeVM:           2,
			unifiedresources.ResourceTypeNetworkShare: 3,
			unifiedresources.ResourceTypePhysicalDisk: 8,
		}
		for typ, want := range wantByType {
			if got := byType[typ]; got != want {
				t.Errorf("record count for %s = %d, want %d", typ, got, want)
			}
		}

		if systemRecord == nil {
			t.Fatal("expected a system agent record in the projection")
		}
		// Connection-scoped identity must thread through RecordsFromSnapshot.
		if systemRecord.SourceID != "system:conn-0724pm" {
			t.Fatalf("system SourceID = %q, want system:conn-0724pm", systemRecord.SourceID)
		}
		if systemRecord.Resource.Name != "truenas-main" {
			t.Fatalf("system Name = %q, want truenas-main", systemRecord.Resource.Name)
		}
		if !systemRecord.Resource.LastSeen.Equal(fixtures.CollectedAt) {
			t.Fatalf("system LastSeen = %v, want snapshot CollectedAt %v", systemRecord.Resource.LastSeen, fixtures.CollectedAt)
		}
	})
}

// TestBranchcov0724pmResetFeatureEnabledFromEnv drives the full env parsing
// ladder through ResetFeatureEnabledFromEnv using t.Setenv (auto-restored).
func TestBranchcov0724pmResetFeatureEnabledFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{
		{name: "empty string defaults feature on", env: "", want: true},
		{name: "whitespace only defaults feature on", env: "   ", want: true},
		{name: "1 enables", env: "1", want: true},
		{name: "true enables case-insensitively", env: "TRUE", want: true},
		{name: "yes enables", env: "yes", want: true},
		{name: "on enables with surrounding whitespace", env: "  on  ", want: true},
		{name: "0 disables", env: "0", want: false},
		{name: "false disables", env: "false", want: false},
		{name: "no disables", env: "no", want: false},
		{name: "off disables", env: "off", want: false},
		{name: "arbitrary unrecognized value disables", env: "maybe", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(FeatureTrueNAS, tt.env)
			ResetFeatureEnabledFromEnv()
			if got := IsFeatureEnabled(); got != tt.want {
				t.Fatalf("after ResetFeatureEnabledFromEnv with %q, IsFeatureEnabled = %v, want %v", tt.env, got, tt.want)
			}
		})
	}

	// ResetFeatureEnabledFromEnv must override a value previously installed by
	// SetFeatureEnabled, proving it re-reads the live environment rather than
	// being a no-op or relying on cached state.
	t.Run("overrides previously set disabled state", func(t *testing.T) {
		SetFeatureEnabled(false)
		t.Setenv(FeatureTrueNAS, "1")
		ResetFeatureEnabledFromEnv()
		if IsFeatureEnabled() != true {
			t.Fatal("ResetFeatureEnabledFromEnv did not flip disabled -> enabled from env")
		}
	})

	t.Run("overrides previously set enabled state", func(t *testing.T) {
		SetFeatureEnabled(true)
		t.Setenv(FeatureTrueNAS, "0")
		ResetFeatureEnabledFromEnv()
		if IsFeatureEnabled() != false {
			t.Fatal("ResetFeatureEnabledFromEnv did not flip enabled -> disabled from env")
		}
	})

	// Restore a deterministic flag value for the rest of the suite.
	t.Cleanup(func() {
		t.Setenv(FeatureTrueNAS, "")
		ResetFeatureEnabledFromEnv()
	})
}

// TestBranchcov0724pmAPIFetcherTransportStatus covers the nil receiver, the
// nil-client guard, and the delegation path through a live Client.
func TestBranchcov0724pmAPIFetcherTransportStatus(t *testing.T) {
	t.Run("nil receiver returns unknown", func(t *testing.T) {
		var f *APIFetcher
		got := f.TransportStatus()
		if got.Mode != TransportUnknown {
			t.Fatalf("nil APIFetcher TransportStatus Mode = %q, want %q", got.Mode, TransportUnknown)
		}
	})

	t.Run("nil client returns unknown", func(t *testing.T) {
		f := &APIFetcher{}
		got := f.TransportStatus()
		if got.Mode != TransportUnknown {
			t.Fatalf("APIFetcher with nil client Mode = %q, want %q", got.Mode, TransportUnknown)
		}
	})

	t.Run("delegates to client transport status", func(t *testing.T) {
		client, err := NewClient(ClientConfig{Host: "status-host"})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		want := TransportStatus{
			Mode:       TransportJSONRPC,
			Endpoint:   "wss://status-host/api/current",
			TLS:        true,
			Connected:  true,
			Reconnects: 3,
		}
		client.updateTransportStatus(func(status *TransportStatus) {
			*status = want
		})
		f := &APIFetcher{Client: client}
		got := f.TransportStatus()
		if got.Mode != want.Mode || got.Endpoint != want.Endpoint ||
			got.TLS != want.TLS || got.Connected != want.Connected || got.Reconnects != want.Reconnects {
			t.Fatalf("APIFetcher.TransportStatus() = %+v, want %+v (delegation failed)", got, want)
		}
	})
}

// TestBranchcov0724pmProviderTransportStatus covers the nil receiver, the
// fallback to unknown for a fixture-backed fetcher, and the delegation path
// when the fetcher implements transportStatusFetcher.
func TestBranchcov0724pmProviderTransportStatus(t *testing.T) {
	t.Run("nil receiver returns unknown", func(t *testing.T) {
		var p *Provider
		got := p.TransportStatus()
		if got.Mode != TransportUnknown {
			t.Fatalf("nil Provider TransportStatus Mode = %q, want %q", got.Mode, TransportUnknown)
		}
	})

	t.Run("fixture fetcher without transport status falls back to unknown", func(t *testing.T) {
		// FixtureFetcher only implements Fetch, not TransportStatus.
		p := NewProvider(DefaultFixtures())
		got := p.TransportStatus()
		if got.Mode != TransportUnknown {
			t.Fatalf("fixture-backed Provider Mode = %q, want %q", got.Mode, TransportUnknown)
		}
	})

	t.Run("delegates to fetcher transport status", func(t *testing.T) {
		want := TransportStatus{
			Mode:             TransportLegacyREST,
			Endpoint:         "https://legacy-host/api/v2.0",
			Connected:        true,
			ApplianceVersion: "TrueNAS-SCALE-24.10.2",
		}
		p := NewLiveProvider(&branchcov0724pmTransportFetcher{status: want})
		got := p.TransportStatus()
		if got.Mode != want.Mode || got.Endpoint != want.Endpoint ||
			got.Connected != want.Connected || got.ApplianceVersion != want.ApplianceVersion {
			t.Fatalf("Provider.TransportStatus() = %+v, want %+v (delegation failed)", got, want)
		}
	})
}

// Compile-time guard: the test fetcher must satisfy both Fetcher and
// transportStatusFetcher so the Provider.TransportStatus delegation path is
// exercised against the real interface contract.
var (
	_ Fetcher                = (*branchcov0724pmTransportFetcher)(nil)
	_ transportStatusFetcher = (*branchcov0724pmTransportFetcher)(nil)
)
