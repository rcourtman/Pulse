package mock

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
)

// branchcov0723pmVMwareFixtureGraph builds a FixtureGraph carrying a single
// controlled VMware host with one recent task and one recent event. It is the
// sole input to (FixtureGraph).SupplementalChanges in these tests, so every
// returned ResourceChange is attributable to known, inspectable fixture data
// rather than whatever the default mock graph happens to contain.
func branchcov0723pmVMwareFixtureGraph() FixtureGraph {
	taskStarted := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	taskCompleted := taskStarted.Add(90 * time.Second)
	eventAt := time.Date(2026, 7, 23, 11, 30, 0, 0, time.UTC)

	host := vmware.InventoryHost{
		Host: "host-1",
		Name: "host-one",
		RecentTasks: []vmware.InventoryTask{{
			Task:          "task-branchcov-1",
			Name:          "Relocate VM",
			State:         "success",
			DescriptionID: "VirtualMachine.migrate",
			StartedAt:     taskStarted,
			CompletedAt:   taskCompleted,
		}},
		RecentEvents: []vmware.InventoryEvent{{
			Event:     "event-branchcov-1",
			Type:      "UserLoginSessionEvent",
			Message:   "User administrator@vsphere.local logged in",
			User:      "administrator@vsphere.local",
			CreatedAt: eventAt,
		}},
	}

	return FixtureGraph{
		PlatformFixtures: PlatformFixtures{
			VMware: vmware.InventorySnapshot{
				ConnectionID: "vc-branchcov",
				Hosts:        []vmware.InventoryHost{host},
			},
		},
	}
}

// TestBranchcov0723pm_AvailabilityFixtures covers both branches of
// AvailabilityFixtures (the mock-disabled nil return and the mock-enabled
// catalog return) and asserts catalog invariants on the enabled result that
// would catch a malformed fixture added later: a non-empty catalog, unique
// identifiers, every required field populated, and the documented
// relationships between fields (http probes carry a path, tcp probes carry a
// port, service targets carry a linked resource, offline targets carry a
// failure reason, online targets report a latency).
func TestBranchcov0723pm_AvailabilityFixtures(t *testing.T) {
	t.Run("disabled returns nil", func(t *testing.T) {
		setMockEnabledForTest(t, false)
		if got := AvailabilityFixtures(); got != nil {
			t.Fatalf("AvailabilityFixtures() = %v, want nil when mock disabled", got)
		}
	})

	t.Run("enabled catalog satisfies invariants", func(t *testing.T) {
		setMockEnabledForTest(t, true)
		fixtures := AvailabilityFixtures()

		if len(fixtures) == 0 {
			t.Fatal("expected a non-empty availability fixture catalog when mock enabled")
		}

		seenIDs := make(map[string]int, len(fixtures))
		sawAvailable, sawUnavailable := false, false
		validKinds := map[string]bool{"machine": true, "service": true, "device": true}
		validProtocols := map[string]bool{"icmp": true, "tcp": true, "http": true}

		for i, f := range fixtures {
			target := f.Target

			// Identity: non-empty, whitespace-normalized, globally unique.
			if target.ID == "" {
				t.Fatalf("fixture[%d]: Target.ID is empty", i)
			}
			if target.ID != strings.TrimSpace(target.ID) {
				t.Fatalf("fixture[%d]: Target.ID %q is not trimmed", i, target.ID)
			}
			if prev := seenIDs[target.ID]; prev != 0 {
				t.Fatalf("fixture[%d]: duplicate Target.ID %q (also at index %d)", i, target.ID, prev-1)
			}
			seenIDs[target.ID] = i + 1

			// Required descriptive fields.
			if target.Name == "" {
				t.Fatalf("fixture[%d] %q: Target.Name is empty", i, target.ID)
			}
			if target.Address == "" {
				t.Fatalf("fixture[%d] %q: Target.Address is empty", i, target.ID)
			}

			// Constrained enumerations.
			if !validKinds[target.TargetKind] {
				t.Fatalf("fixture[%d] %q: TargetKind %q is not a documented kind", i, target.ID, target.TargetKind)
			}
			if !validProtocols[target.Protocol] {
				t.Fatalf("fixture[%d] %q: Protocol %q is not a documented probe", i, target.ID, target.Protocol)
			}

			// The default catalog ships every target enabled with sane
			// polling cadence; a zero here would surface as a broken probe.
			if !target.Enabled {
				t.Fatalf("fixture[%d] %q: expected Enabled=true in the default catalog", i, target.ID)
			}
			if target.PollIntervalSecs <= 0 {
				t.Fatalf("fixture[%d] %q: PollIntervalSecs = %d, want > 0", i, target.ID, target.PollIntervalSecs)
			}
			if target.TimeoutMillis <= 0 {
				t.Fatalf("fixture[%d] %q: TimeoutMillis = %d, want > 0", i, target.ID, target.TimeoutMillis)
			}
			if target.FailureThreshold <= 0 {
				t.Fatalf("fixture[%d] %q: FailureThreshold = %d, want > 0", i, target.ID, target.FailureThreshold)
			}

			// Documented protocol/field relationships.
			if target.Protocol == "http" && strings.TrimSpace(target.Path) == "" {
				t.Fatalf("fixture[%d] %q: http probe must carry a Path", i, target.ID)
			}
			if target.Protocol == "tcp" && target.Port <= 0 {
				t.Fatalf("fixture[%d] %q: tcp probe must carry a Port > 0", i, target.ID)
			}
			if target.TargetKind == "service" && strings.TrimSpace(target.LinkedResourceID) == "" {
				t.Fatalf("fixture[%d] %q: service target must reference a LinkedResourceID", i, target.ID)
			}

			// A probe must have run at least once.
			if f.LastChecked.IsZero() {
				t.Fatalf("fixture[%d] %q: LastChecked is zero", i, target.ID)
			}

			// Documented availability/state relationships.
			if f.Available {
				sawAvailable = true
				if f.LatencyMillis <= 0 {
					t.Fatalf("fixture[%d] %q: available target reports LatencyMillis = %d, want > 0", i, target.ID, f.LatencyMillis)
				}
			} else {
				sawUnavailable = true
				if f.ConsecutiveFailures < 1 {
					t.Fatalf("fixture[%d] %q: unavailable target has ConsecutiveFailures = %d, want >= 1", i, target.ID, f.ConsecutiveFailures)
				}
				if strings.TrimSpace(f.LastError) == "" {
					t.Fatalf("fixture[%d] %q: unavailable target must carry a LastError reason", i, target.ID)
				}
			}
		}

		// The catalog intentionally demonstrates both healthy and failing
		// endpoints so the UI has something to render in each state.
		if !sawAvailable {
			t.Fatal("catalog has no available fixture; expected at least one online target")
		}
		if !sawUnavailable {
			t.Fatal("catalog has no unavailable fixture; expected at least one offline target")
		}
	})
}

// TestBranchcov0723pm_FixtureGraphSupplementalChanges exercises every
// DataSource arm of (FixtureGraph).SupplementalChanges, whose switch only
// handles SourceVMware. It covers the handled VMware arm (including the
// "vmware-vsphere" alias routed through normalizeSupplementalSource) and the
// default nil-return for sources the switch does NOT handle (TrueNAS,
// Availability), plus an unknown and an empty source. For the VMware arm it
// asserts the concrete ResourceChange contents projected from the controlled
// fixture task and event.
func TestBranchcov0723pm_FixtureGraphSupplementalChanges(t *testing.T) {
	graph := branchcov0723pmVMwareFixtureGraph()
	const wantResourceID = "vc-branchcov:host:host-1"

	t.Run("VMware arm projects task and event changes", func(t *testing.T) {
		changes := graph.SupplementalChanges(unifiedresources.SourceVMware)
		if len(changes) != 2 {
			t.Fatalf("SupplementalChanges(SourceVMware) returned %d changes, want 2 (1 task + 1 event)", len(changes))
		}

		sawTask, sawEvent := false, false
		for _, c := range changes {
			// Canonical timeline contract for provider activity.
			if c.Kind != unifiedresources.ChangeActivity {
				t.Fatalf("change %q: Kind = %q, want %q", c.ID, c.Kind, unifiedresources.ChangeActivity)
			}
			if c.SourceType != unifiedresources.SourcePlatformEvent {
				t.Fatalf("change %q: SourceType = %q, want %q", c.ID, c.SourceType, unifiedresources.SourcePlatformEvent)
			}
			if c.SourceAdapter != unifiedresources.AdapterVMware {
				t.Fatalf("change %q: SourceAdapter = %q, want %q", c.ID, c.SourceAdapter, unifiedresources.AdapterVMware)
			}
			if c.Confidence != unifiedresources.ConfidenceHigh {
				t.Fatalf("change %q: Confidence = %q, want %q", c.ID, c.Confidence, unifiedresources.ConfidenceHigh)
			}

			// Resource identity projected from the controlled host fixture.
			if c.ResourceID != wantResourceID {
				t.Fatalf("change %q: ResourceID = %q, want %q", c.ID, c.ResourceID, wantResourceID)
			}

			// Stable activity identifier and observed timestamp.
			if !strings.HasPrefix(c.ID, "activity-") {
				t.Fatalf("change ID %q must carry the activity- prefix", c.ID)
			}
			if c.ObservedAt.IsZero() {
				t.Fatalf("change %q: ObservedAt is zero", c.ID)
			}
			if !c.ObservedAt.Equal(c.ObservedAt.UTC()) {
				t.Fatalf("change %q: ObservedAt %s is not UTC-normalized", c.ID, c.ObservedAt)
			}
			if strings.TrimSpace(c.Reason) == "" {
				t.Fatalf("change %q: Reason is empty", c.ID)
			}

			// Provider context preserved verbatim from the fixture.
			if c.Metadata["vmwareConnectionId"] != "vc-branchcov" {
				t.Fatalf("change %q: vmwareConnectionId = %v, want vc-branchcov", c.ID, c.Metadata["vmwareConnectionId"])
			}
			if c.Metadata["vmwareEntityType"] != "host" {
				t.Fatalf("change %q: vmwareEntityType = %v, want host", c.ID, c.Metadata["vmwareEntityType"])
			}
			if c.Metadata["vmwareManagedObjectId"] != "host-1" {
				t.Fatalf("change %q: vmwareManagedObjectId = %v, want host-1", c.ID, c.Metadata["vmwareManagedObjectId"])
			}

			switch c.Metadata["activity_type"] {
			case "vmware_task":
				sawTask = true
				if c.Metadata["vmwareTask"] != "task-branchcov-1" {
					t.Fatalf("task change %q: vmwareTask = %v, want task-branchcov-1", c.ID, c.Metadata["vmwareTask"])
				}
			case "vmware_event":
				sawEvent = true
				if c.Metadata["vmwareEvent"] != "event-branchcov-1" {
					t.Fatalf("event change %q: vmwareEvent = %v, want event-branchcov-1", c.ID, c.Metadata["vmwareEvent"])
				}
			default:
				t.Fatalf("change %q: unexpected activity_type %v", c.ID, c.Metadata["activity_type"])
			}
		}

		if !sawTask {
			t.Fatal("expected one vmware_task change projected from the host RecentTask")
		}
		if !sawEvent {
			t.Fatal("expected one vmware_event change projected from the host RecentEvent")
		}
	})

	t.Run("vmware-vsphere alias routes to the VMware arm", func(t *testing.T) {
		alias := graph.SupplementalChanges(unifiedresources.DataSource("vmware-vsphere"))
		if len(alias) != 2 {
			t.Fatalf("vmware-vsphere alias returned %d changes, want 2", len(alias))
		}
	})

	t.Run("switch does not handle TrueNAS", func(t *testing.T) {
		if got := graph.SupplementalChanges(unifiedresources.SourceTrueNAS); got != nil {
			t.Fatalf("SupplementalChanges(SourceTrueNAS) = %v, want nil (no switch arm)", got)
		}
	})

	t.Run("switch does not handle Availability", func(t *testing.T) {
		if got := graph.SupplementalChanges(unifiedresources.SourceAvailability); got != nil {
			t.Fatalf("SupplementalChanges(SourceAvailability) = %v, want nil (no switch arm)", got)
		}
	})

	t.Run("unknown source hits default arm", func(t *testing.T) {
		if got := graph.SupplementalChanges(unifiedresources.DataSource("kubernetes")); got != nil {
			t.Fatalf("SupplementalChanges(unknown) = %v, want nil via default arm", got)
		}
	})

	t.Run("empty and whitespace-only source normalize away and hit default arm", func(t *testing.T) {
		if got := graph.SupplementalChanges(unifiedresources.DataSource("")); got != nil {
			t.Fatalf("SupplementalChanges(\"\") = %v, want nil via default arm", got)
		}
		if got := graph.SupplementalChanges(unifiedresources.DataSource("   ")); got != nil {
			t.Fatalf("SupplementalChanges(whitespace) = %v, want nil via default arm", got)
		}
	})

	t.Run("empty graph yields no VMware changes", func(t *testing.T) {
		empty := FixtureGraph{}
		if got := empty.SupplementalChanges(unifiedresources.SourceVMware); len(got) != 0 {
			t.Fatalf("empty graph SupplementalChanges(SourceVMware) = %d changes, want 0", len(got))
		}
	})
}

// TestBranchcov0723pm_GenerateMockHostRate covers every arm of the
// generateMockHostRate switch. generateMockHostRate first delegates to
// generateRealisticIO and only falls through to its own switch when that
// helper reports an idle (zero) rate, and the package-global math/rand source
// cannot be made deterministic without re-implementing the helper, so these
// subtests assert the documented OUTPUT BANDS rather than exact values:
//
//   - For an unknown ioType, generateRealisticIO has no case and returns 0,
//     so ONLY the default switch arm can run. Its band is tight and exclusive:
//     (32 + Intn(512)) * 1024 -> [32768, 556032].
//
//   - For each known ioType the observable output is the union of a non-idle
//     generateRealisticIO value (which the function returns directly) and the
//     switch fallback (used when generateRealisticIO is idle). The asserted
//     band is therefore [switch-fallback min, generateRealisticIO active max].
//
// Every code path produces a whole number of bytes that is an exact multiple
// of 1024 (switch arms multiply by 1024; generateRealisticIO emits multiples
// of 1024*1024 or 1024*1024/8), so that structural invariant is checked too.
func TestBranchcov0723pm_GenerateMockHostRate(t *testing.T) {
	const samples = 4000

	multipleOf1024 := func(rate float64) bool {
		return math.Mod(rate, 1024) == 0
	}

	t.Run("unknown ioType lands exclusively in the default arm band", func(t *testing.T) {
		const (
			minBand = float64(32 * 1024)         // (32 + 0) * 1024
			maxBand = float64((32 + 511) * 1024) // (32 + 511) * 1024
			midBand = float64((32 + 256) * 1024) // midpoint of the arm's range
		)
		belowMid, aboveMid := false, false
		for i := 0; i < samples; i++ {
			rate := generateMockHostRate("unknown-io-type")
			if rate < minBand || rate > maxBand {
				t.Fatalf("sample %d: rate %g outside default band [%g, %g]", i, rate, minBand, maxBand)
			}
			if !multipleOf1024(rate) {
				t.Fatalf("sample %d: rate %g is not a multiple of 1024", i, rate)
			}
			if rate < midBand {
				belowMid = true
			} else {
				aboveMid = true
			}
		}
		// The default arm's Intn(512) should span its full range; seeing
		// values on both sides of the midpoint proves the arm is live and
		// not clamped to a single bucket.
		if !belowMid || !aboveMid {
			t.Fatalf("default arm did not span its range: belowMid=%v aboveMid=%v", belowMid, aboveMid)
		}
	})

	// Each known ioType: [switch-fallback min, generateRealisticIO active max].
	knownBands := []struct {
		name    string
		minBand float64
		maxBand float64
	}{
		// network-in: switch (128 + Intn(4096))*1024 floor; realisticIO high
		// ceiling (100 + Intn(400)) * 1024*1024/8 = 499 * 131072.
		{"network-in", float64(128 * 1024), float64(499 * 1024 * 1024 / 8)},
		// network-out: switch (96 + Intn(3072))*1024 floor; realisticIO high
		// ceiling (50 + Intn(200)) * 1024*1024/8 = 249 * 131072.
		{"network-out", float64(96 * 1024), float64(249 * 1024 * 1024 / 8)},
		// disk-read: switch (64 + Intn(2048))*1024 floor; realisticIO high
		// ceiling (25 + Intn(75)) * 1024*1024 = 99 * 1048576.
		{"disk-read", float64(64 * 1024), float64(99 * 1024 * 1024)},
		// disk-write: switch (32 + Intn(1536))*1024 floor; realisticIO high
		// ceiling (18 + Intn(32)) * 1024*1024 = 49 * 1048576.
		{"disk-write", float64(32 * 1024), float64(49 * 1024 * 1024)},
	}

	for _, kb := range knownBands {
		kb := kb
		t.Run(kb.name+" stays within its output band", func(t *testing.T) {
			for i := 0; i < samples; i++ {
				rate := generateMockHostRate(kb.name)
				if rate < kb.minBand || rate > kb.maxBand {
					t.Fatalf("%s sample %d: rate %g outside band [%g, %g]", kb.name, i, rate, kb.minBand, kb.maxBand)
				}
				if !multipleOf1024(rate) {
					t.Fatalf("%s sample %d: rate %g is not a multiple of 1024", kb.name, i, rate)
				}
			}
		})
	}
}
