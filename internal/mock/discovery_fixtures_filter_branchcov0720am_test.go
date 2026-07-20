package mock

import (
	"strings"
	"testing"
)

// branchcovDiscoveryFixtures builds a deterministic, controlled set of
// discovery fixtures (including a defensive nil entry) used to drive every
// branch of the CurrentDiscoveryFixturesByType and CurrentDiscoveryFixturesByTarget
// filters without depending on whatever the default mock graph happens to
// contain. Identifiers are intentionally chosen so that each candidate-matching
// arm of discoveryFixtureMatchesTarget can be exercised in isolation.
func branchcovDiscoveryFixtures() []*DiscoveryFixture {
	return []*DiscoveryFixture{
		{
			ID:           "vm:host-a:101",
			ResourceType: "vm",
			ResourceID:   "101",
			TargetID:     "host-a",
			AgentID:      "host-a",
			Hostname:     "vm-host-a",
			ServiceName:  "VM 101",
			ConfigPaths:  []string{"/etc/vm/config"},
		},
		{
			ID:           "vm:host-b:102",
			ResourceType: "vm",
			ResourceID:   "102",
			TargetID:     "host-b",
			AgentID:      "host-b",
			Hostname:     "vm-host-b",
			ServiceName:  "VM 102",
		},
		{
			ID:           "docker:host-a:redis-cache",
			ResourceType: "docker",
			ResourceID:   "redis-cache",
			TargetID:     "host-a",
			AgentID:      "host-a",
			Hostname:     "docker-host-a",
			ServiceName:  "Redis",
		},
		{
			// An agent-type fixture whose ResourceID differs from every other
			// candidate identifier, so it can only match a target lookup via
			// the agent-only ResourceID candidate arm.
			ID:           "agent:node-x:unique-agent-id",
			ResourceType: "agent",
			ResourceID:   "unique-agent-id",
			TargetID:     "node-x",
			AgentID:      "node-x",
			Hostname:     "pulse-node-x",
			ServiceName:  "Pulse Agent",
		},
		nil, // exercises the defensive nil-skip branch in both filters
	}
}

// setMockEnabledForTest toggles mock mode for the duration of the test and
// restores the prior state on cleanup.
func setMockEnabledForTest(t testing.TB, enabled bool) {
	t.Helper()
	previous := IsMockEnabled()
	if err := SetEnabled(enabled); err != nil {
		t.Fatalf("SetEnabled(%v): %v", enabled, err)
	}
	t.Cleanup(func() {
		_ = SetEnabled(previous)
	})
}

// swapDiscoveryFixturesForTest replaces mockGraph.DiscoveryFixtures under the
// data lock for the duration of the test and restores the prior value on
// cleanup, so the controlled fixture list is the sole input to the filters.
func swapDiscoveryFixturesForTest(t testing.TB, next []*DiscoveryFixture) {
	t.Helper()
	dataMu.Lock()
	previous := mockGraph.DiscoveryFixtures
	mockGraph.DiscoveryFixtures = next
	dataMu.Unlock()
	t.Cleanup(func() {
		dataMu.Lock()
		mockGraph.DiscoveryFixtures = previous
		dataMu.Unlock()
	})
}

// TestBranchcov0720am_CurrentDiscoveryFixturesByType exercises every branch of
// CurrentDiscoveryFixturesByType: the matching path, the non-matching path,
// whitespace normalization of the input, the empty-input path, and the
// defensive nil-skip (the injected nil entry must never appear in the result
// nor cause a panic).
func TestBranchcov0720am_CurrentDiscoveryFixturesByType(t *testing.T) {
	setMockEnabledForTest(t, true)
	swapDiscoveryFixturesForTest(t, branchcovDiscoveryFixtures())

	testCases := []struct {
		name         string
		resourceType string
		wantCount    int
		wantService  string
	}{
		{
			name:         "matching type returns matching fixtures and skips nil entry",
			resourceType: "vm",
			wantCount:    2,
		},
		{
			name:         "single match for a distinct type",
			resourceType: "docker",
			wantCount:    1,
			wantService:  "Redis",
		},
		{
			name:         "agent type matched",
			resourceType: "agent",
			wantCount:    1,
			wantService:  "Pulse Agent",
		},
		{
			name:         "whitespace-padded input is trimmed and still matches",
			resourceType: "  vm  ",
			wantCount:    2,
		},
		{
			name:         "non-matching type yields empty result",
			resourceType: "k8s",
			wantCount:    0,
		},
		{
			name:         "empty resourceType yields empty result",
			resourceType: "",
			wantCount:    0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := CurrentDiscoveryFixturesByType(tc.resourceType)
			if len(got) != tc.wantCount {
				t.Fatalf("CurrentDiscoveryFixturesByType(%q) returned %d fixtures, want %d", tc.resourceType, len(got), tc.wantCount)
			}
			if tc.wantCount == 0 {
				return
			}
			// Every returned fixture must be non-nil and carry the requested
			// type after normalization, proving the filter neither leaked other
			// types nor the injected nil entry.
			wantType := strings.TrimSpace(tc.resourceType)
			for _, f := range got {
				if f == nil {
					t.Fatalf("CurrentDiscoveryFixturesByType(%q) returned a nil fixture", tc.resourceType)
				}
				if strings.TrimSpace(f.ResourceType) != wantType {
					t.Fatalf("CurrentDiscoveryFixturesByType(%q) returned fixture of type %q", tc.resourceType, f.ResourceType)
				}
			}
			if tc.wantService != "" {
				ok := false
				for _, f := range got {
					if f.ServiceName == tc.wantService {
						ok = true
						break
					}
				}
				if !ok {
					t.Fatalf("CurrentDiscoveryFixturesByType(%q) returned no fixture with ServiceName %q", tc.resourceType, tc.wantService)
				}
			}
		})
	}

	t.Run("returned fixtures are defensive copies", func(t *testing.T) {
		got := CurrentDiscoveryFixturesByType("vm")
		if len(got) != 2 {
			t.Fatalf("expected 2 vm fixtures, got %d", len(got))
		}
		// Mutate the returned clone and confirm a fresh call is unaffected: the
		// store must not share backing storage with the caller.
		got[0].ServiceName = "mutated-by-test"
		if len(got[0].ConfigPaths) > 0 {
			got[0].ConfigPaths[0] = "/mutated"
		}
		again := CurrentDiscoveryFixturesByType("vm")
		if len(again) != 2 {
			t.Fatalf("expected 2 vm fixtures on re-query, got %d", len(again))
		}
		for _, f := range again {
			if f.ServiceName == "mutated-by-test" {
				t.Fatal("returned fixture shared its ServiceName backing value with the underlying store")
			}
			for _, p := range f.ConfigPaths {
				if p == "/mutated" {
					t.Fatal("returned fixture shared its ConfigPaths backing slice with the underlying store")
				}
			}
		}
	})
}

// TestBranchcov0720am_CurrentDiscoveryFixturesByTarget exercises every candidate
// arm of discoveryFixtureMatchesTarget through the public filter: TargetID,
// AgentID, Hostname, and the agent-only ResourceID arm (plus the negative case
// proving a non-agent ResourceID is not a candidate). It also covers whitespace
// trimming, case-insensitive matching, the empty/whitespace-only early return,
// the non-matching path, and the defensive nil-skip.
func TestBranchcov0720am_CurrentDiscoveryFixturesByTarget(t *testing.T) {
	setMockEnabledForTest(t, true)
	swapDiscoveryFixturesForTest(t, branchcovDiscoveryFixtures())

	testCases := []struct {
		name      string
		targetID  string
		wantCount int
		wantIDs   []string
	}{
		{
			name:      "match by TargetID returns every fixture for that target",
			targetID:  "host-a",
			wantCount: 2,
			wantIDs:   []string{"vm:host-a:101", "docker:host-a:redis-cache"},
		},
		{
			name:      "match by TargetID for a single-fixture target",
			targetID:  "host-b",
			wantCount: 1,
			wantIDs:   []string{"vm:host-b:102"},
		},
		{
			name:      "match via Hostname candidate only",
			targetID:  "vm-host-a",
			wantCount: 1,
			wantIDs:   []string{"vm:host-a:101"},
		},
		{
			name:      "match via AgentID candidate only",
			targetID:  "node-x",
			wantCount: 1,
			wantIDs:   []string{"agent:node-x:unique-agent-id"},
		},
		{
			name:      "agent ResourceID candidate arm matches when no other identifier does",
			targetID:  "unique-agent-id",
			wantCount: 1,
			wantIDs:   []string{"agent:node-x:unique-agent-id"},
		},
		{
			name:      "non-agent ResourceID is not a candidate so no match",
			targetID:  "redis-cache",
			wantCount: 0,
		},
		{
			name:      "whitespace-padded input is trimmed and still matches",
			targetID:  "  host-a  ",
			wantCount: 2,
			wantIDs:   []string{"vm:host-a:101", "docker:host-a:redis-cache"},
		},
		{
			name:      "case-insensitive match via EqualFold",
			targetID:  "HOST-A",
			wantCount: 2,
			wantIDs:   []string{"vm:host-a:101", "docker:host-a:redis-cache"},
		},
		{
			name:      "non-matching target yields empty result",
			targetID:  "does-not-exist",
			wantCount: 0,
		},
		{
			name:      "empty targetID yields empty result",
			targetID:  "",
			wantCount: 0,
		},
		{
			name:      "whitespace-only targetID trims to empty and yields empty result",
			targetID:  "   ",
			wantCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := CurrentDiscoveryFixturesByTarget(tc.targetID)
			if len(got) != tc.wantCount {
				t.Fatalf("CurrentDiscoveryFixturesByTarget(%q) returned %d fixtures, want %d", tc.targetID, len(got), tc.wantCount)
			}
			if len(tc.wantIDs) == 0 {
				return
			}
			gotIDs := make(map[string]struct{}, len(got))
			for _, f := range got {
				if f == nil {
					t.Fatalf("CurrentDiscoveryFixturesByTarget(%q) returned a nil fixture", tc.targetID)
				}
				if _, dup := gotIDs[f.ID]; dup {
					t.Fatalf("CurrentDiscoveryFixturesByTarget(%q) returned duplicate ID %q", tc.targetID, f.ID)
				}
				gotIDs[f.ID] = struct{}{}
			}
			for _, want := range tc.wantIDs {
				if _, ok := gotIDs[want]; !ok {
					t.Fatalf("CurrentDiscoveryFixturesByTarget(%q): expected ID %q in result, got %v", tc.targetID, want, gotIDs)
				}
			}
		})
	}

	t.Run("returned fixtures are defensive copies", func(t *testing.T) {
		got := CurrentDiscoveryFixturesByTarget("host-a")
		if len(got) != 2 {
			t.Fatalf("expected 2 fixtures for host-a, got %d", len(got))
		}
		got[0].ServiceName = "mutated-by-test"
		if len(got[0].ConfigPaths) > 0 {
			got[0].ConfigPaths[0] = "/mutated"
		}
		again := CurrentDiscoveryFixturesByTarget("host-a")
		if len(again) != 2 {
			t.Fatalf("expected 2 fixtures on re-query, got %d", len(again))
		}
		for _, f := range again {
			if f.ServiceName == "mutated-by-test" {
				t.Fatal("returned fixture shared its ServiceName backing value with the underlying store")
			}
			for _, p := range f.ConfigPaths {
				if p == "/mutated" {
					t.Fatal("returned fixture shared its ConfigPaths backing slice with the underlying store")
				}
			}
		}
	})
}

// TestBranchcov0720am_DiscoveryFilters_DisabledMock covers the empty-result
// branch reached when mock mode is disabled: CurrentDiscoveryFixtures
// short-circuits before reading the fixture store, so both filters must return
// an empty result without panicking.
func TestBranchcov0720am_DiscoveryFilters_DisabledMock(t *testing.T) {
	setMockEnabledForTest(t, false)

	t.Run("ByType returns empty when mock disabled", func(t *testing.T) {
		got := CurrentDiscoveryFixturesByType("vm")
		if len(got) != 0 {
			t.Fatalf("expected empty result when mock disabled, got %d", len(got))
		}
	})
	t.Run("ByTarget returns empty when mock disabled", func(t *testing.T) {
		got := CurrentDiscoveryFixturesByTarget("host-a")
		if len(got) != 0 {
			t.Fatalf("expected empty result when mock disabled, got %d", len(got))
		}
	})
}
