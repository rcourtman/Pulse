package monitoring

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// This file is a purpose-built branch-coverage test set (selected via
// `-run "^TestBranchcov0724pm"`) for two pure helpers in package monitoring
// that previously had 0.0% coverage:
//
//   - trueNASAppRunning(app *truenas.App) bool — truenas_poller.go:1449
//   - supplementalProviderOwnedSourcesForOrg(provider MonitorSupplementalRecordsProvider, orgID string) []unifiedresources.DataSource — monitored_system_usage.go:162
//
// Every arm of each function is exercised directly here.
//
// For trueNASAppRunning: nil app, the app.State=="RUNNING" arm (exact,
// lower-case, whitespace-padded, mixed-case), the container.State=="running"
// arm (single container, container found after earlier non-matching
// containers, whitespace-padded container state), and the fall-through false
// arm (non-matching state with no containers, empty state, and all-non-running
// containers).
//
// For supplementalProviderOwnedSourcesForOrg: nil provider, a provider that
// implements neither owned-source method, a provider with the org-scoped
// method returning nil / empty / populated sources, a legacy provider
// (org-agnostic method) returning nil / populated sources, a dual provider
// where the org-scoped method must take precedence over the legacy one,
// all-whitespace sources that normalise to empty, and clone independence of
// the returned slice.
//
// Conventions match sibling in-package tests in this directory (see
// purehelpers_branchcov0722am_test.go, truenas_poller_test.go): stdlib
// `testing` only, table-driven subtests, reflect.DeepEqual / t.Fatalf
// assertions, no testify.

// ---------------------------------------------------------------------------
// Fake supplemental providers. All use a distinctive prefix to avoid colliding
// with existing test helpers in this large package.
// ---------------------------------------------------------------------------

// bareProvider implements only the base MonitorSupplementalRecordsProvider
// interface — neither owned-source method is present, so both type assertions
// in the function under test fail and the result is nil.
type branchcov0724pmBareProvider struct{}

func (branchcov0724pmBareProvider) SupplementalRecords(_ *Monitor, _ string) []unifiedresources.IngestRecord {
	return nil
}

// forOrgProvider implements the org-scoped SnapshotOwnedSourcesForOrg method.
type branchcov0724pmForOrgProvider struct {
	mapping map[string][]unifiedresources.DataSource
	seenOrg string
}

func (p *branchcov0724pmForOrgProvider) SupplementalRecords(_ *Monitor, _ string) []unifiedresources.IngestRecord {
	return nil
}

func (p *branchcov0724pmForOrgProvider) SnapshotOwnedSourcesForOrg(orgID string) []unifiedresources.DataSource {
	p.seenOrg = orgID
	return p.mapping[orgID]
}

// legacyProvider implements only the org-agnostic SnapshotOwnedSources method.
type branchcov0724pmLegacyProvider struct {
	sources []unifiedresources.DataSource
	called  bool
}

func (p *branchcov0724pmLegacyProvider) SupplementalRecords(_ *Monitor, _ string) []unifiedresources.IngestRecord {
	return nil
}

func (p *branchcov0724pmLegacyProvider) SnapshotOwnedSources() []unifiedresources.DataSource {
	p.called = true
	return p.sources
}

// dualProvider implements both methods; the org-scoped variant must win.
type branchcov0724pmDualProvider struct {
	forOrgSources []unifiedresources.DataSource
	legacySources []unifiedresources.DataSource
	forOrgCalled  bool
	legacyCalled  bool
}

func (p *branchcov0724pmDualProvider) SupplementalRecords(_ *Monitor, _ string) []unifiedresources.IngestRecord {
	return nil
}

func (p *branchcov0724pmDualProvider) SnapshotOwnedSourcesForOrg(_ string) []unifiedresources.DataSource {
	p.forOrgCalled = true
	return p.forOrgSources
}

func (p *branchcov0724pmDualProvider) SnapshotOwnedSources() []unifiedresources.DataSource {
	p.legacyCalled = true
	return p.legacySources
}

// ---------------------------------------------------------------------------
// trueNASAppRunning
// ---------------------------------------------------------------------------

func TestBranchcov0724pmTrueNASAppRunning(t *testing.T) {
	t.Run("nil app returns false", func(t *testing.T) {
		if got := trueNASAppRunning(nil); got {
			t.Fatal("trueNASAppRunning(nil) = true, want false")
		}
	})

	tests := []struct {
		name string
		app  *truenas.App
		want bool
	}{
		// Arm: app.State matches "RUNNING" (case-insensitive, trimmed).
		{"state RUNNING exact matches", &truenas.App{State: "RUNNING"}, true},
		{"state running lower-case matches via EqualFold", &truenas.App{State: "running"}, true},
		{"state RUNNING with surrounding whitespace trimmed", &truenas.App{State: "  RUNNING  "}, true},
		{"state RuNnInG mixed case matches", &truenas.App{State: "RuNnInG"}, true},

		// Arm: app.State does not match but a container.State matches
		// "running" (case-insensitive, trimmed).
		{"single container running when state stopped", &truenas.App{
			State:      "STOPPED",
			Containers: []truenas.AppContainer{{State: "running"}},
		}, true},
		{"container found after earlier non-matching containers", &truenas.App{
			State: "STOPPED",
			Containers: []truenas.AppContainer{
				{State: "exited"},
				{State: "paused"},
				{State: "RUNNING"},
			},
		}, true},
		{"container state with surrounding whitespace trimmed", &truenas.App{
			State: "STOPPED",
			Containers: []truenas.AppContainer{
				{State: "  running  "},
			},
		}, true},

		// Arm: nothing matches — fall-through false.
		{"state STOPPED with no containers is false", &truenas.App{State: "STOPPED"}, false},
		{"empty state with no containers is false", &truenas.App{State: ""}, false},
		{"state STOPPED with all non-running containers is false", &truenas.App{
			State: "STOPPED",
			Containers: []truenas.AppContainer{
				{State: "exited"},
				{State: "paused"},
			},
		}, false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := trueNASAppRunning(tc.app); got != tc.want {
				state := ""
				if tc.app != nil {
					state = tc.app.State
				}
				t.Fatalf("trueNASAppRunning(state=%q) = %v, want %v", state, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// supplementalProviderOwnedSourcesForOrg
// ---------------------------------------------------------------------------

func TestBranchcov0724pmSupplementalProviderOwnedSourcesForOrg(t *testing.T) {
	t.Run("nil provider returns nil", func(t *testing.T) {
		if got := supplementalProviderOwnedSourcesForOrg(nil, "org-1"); got != nil {
			t.Fatalf("nil provider = %v, want nil", got)
		}
	})

	t.Run("provider implementing neither owned-source method returns nil", func(t *testing.T) {
		prov := branchcov0724pmBareProvider{}
		if got := supplementalProviderOwnedSourcesForOrg(prov, "org-1"); got != nil {
			t.Fatalf("bare provider = %v, want nil", got)
		}
	})

	t.Run("forOrg provider with no entry for org returns nil", func(t *testing.T) {
		prov := &branchcov0724pmForOrgProvider{
			mapping: map[string][]unifiedresources.DataSource{},
		}
		if got := supplementalProviderOwnedSourcesForOrg(prov, "org-missing"); got != nil {
			t.Fatalf("missing-org = %v, want nil", got)
		}
	})

	t.Run("forOrg provider with empty slice returns nil", func(t *testing.T) {
		prov := &branchcov0724pmForOrgProvider{
			mapping: map[string][]unifiedresources.DataSource{"org-1": {}},
		}
		if got := supplementalProviderOwnedSourcesForOrg(prov, "org-1"); got != nil {
			t.Fatalf("empty-slice = %v, want nil", got)
		}
	})

	t.Run("forOrg provider passes orgID through and returns normalized sources", func(t *testing.T) {
		prov := &branchcov0724pmForOrgProvider{
			mapping: map[string][]unifiedresources.DataSource{
				"org-1": {"Proxmox", " Docker ", ""},
			},
		}
		got := supplementalProviderOwnedSourcesForOrg(prov, "org-1")
		want := []unifiedresources.DataSource{"proxmox", "docker"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got = %v, want %v", got, want)
		}
		if prov.seenOrg != "org-1" {
			t.Fatalf("SnapshotOwnedSourcesForOrg received orgID = %q, want %q", prov.seenOrg, "org-1")
		}
	})

	t.Run("legacy provider returns normalized sources", func(t *testing.T) {
		prov := &branchcov0724pmLegacyProvider{
			sources: []unifiedresources.DataSource{"PBS", "  agent "},
		}
		got := supplementalProviderOwnedSourcesForOrg(prov, "org-1")
		want := []unifiedresources.DataSource{"pbs", "agent"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got = %v, want %v", got, want)
		}
		if !prov.called {
			t.Fatal("SnapshotOwnedSources was not called on legacy provider")
		}
	})

	t.Run("legacy provider with nil sources returns nil", func(t *testing.T) {
		prov := &branchcov0724pmLegacyProvider{}
		if got := supplementalProviderOwnedSourcesForOrg(prov, "org-1"); got != nil {
			t.Fatalf("nil-sources legacy = %v, want nil", got)
		}
	})

	t.Run("dual provider prefers forOrg over legacy", func(t *testing.T) {
		prov := &branchcov0724pmDualProvider{
			forOrgSources: []unifiedresources.DataSource{"truenas"},
			legacySources: []unifiedresources.DataSource{"vmware"},
		}
		got := supplementalProviderOwnedSourcesForOrg(prov, "org-1")
		want := []unifiedresources.DataSource{"truenas"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got = %v, want %v (forOrg should win)", got, want)
		}
		if !prov.forOrgCalled {
			t.Fatal("expected SnapshotOwnedSourcesForOrg to be called")
		}
		if prov.legacyCalled {
			t.Fatal("SnapshotOwnedSources should NOT be called when forOrg is available")
		}
	})

	t.Run("all-whitespace sources are dropped leaving empty result", func(t *testing.T) {
		// len(sources) is 3 here so the len==0 early-nil arm is NOT taken;
		// instead the function builds make([]DataSource,0,3) and every
		// element normalizes to "" and is skipped via continue. The result
		// is therefore a non-nil empty slice (distinct from the nil returned
		// when len(sources)==0).
		prov := &branchcov0724pmForOrgProvider{
			mapping: map[string][]unifiedresources.DataSource{
				"org-1": {"  ", "", "\t"},
			},
		}
		got := supplementalProviderOwnedSourcesForOrg(prov, "org-1")
		if len(got) != 0 {
			t.Fatalf("all-empty-normalized = %v, want length 0", got)
		}
	})

	t.Run("returned slice is independent of input slice", func(t *testing.T) {
		input := []unifiedresources.DataSource{"Proxmox", "Docker"}
		prov := &branchcov0724pmLegacyProvider{sources: input}
		got := supplementalProviderOwnedSourcesForOrg(prov, "org-1")

		if len(got) != 2 {
			t.Fatalf("got = %v (len %d), want 2 normalized entries", got, len(got))
		}
		// Mutate the returned slice; the provider's backing slice must be
		// unaffected because the function builds a fresh slice.
		got[0] = "tampered"
		if input[0] != "Proxmox" {
			t.Fatalf("mutating returned slice corrupted input: input[0] = %q", input[0])
		}
	})
}
