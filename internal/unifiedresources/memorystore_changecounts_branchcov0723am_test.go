package unifiedresources

import (
	"reflect"
	"testing"
	"time"
)

// Branch-coverage tests for currently-0.0%-covered MemoryStore methods in
// internal/unifiedresources/store.go:
//   - MemoryStore.CountRecentChangesByKind               (store.go:3356)
//   - MemoryStore.CountRecentChangesByKindFiltered        (store.go:3360)
//   - MemoryStore.CountRecentChangesBySourceType          (store.go:3384)
//   - MemoryStore.CountRecentChangesBySourceTypeFiltered  (store.go:3388)
//   - MemoryStore.CountRecentChangesBySourceAdapter       (store.go:3412)
//   - MemoryStore.CountRecentChangesBySourceAdapterFiltered (store.go:3416)
//   - MemoryStore.RecordExportAudit                       (store.go:3964)
//   - MemoryStore.GetExportAudits                         (store.go:3971)
//
// Every subtest constructs its OWN MemoryStore so it passes when run alone via
// -run. The shared "now" (branchcov0723amNow) comes from the sibling
// memorystore_actions_branchcov0723am_test.go in this same package.

// branchcov0723amCountChange builds a minimal but valid ResourceChange for the
// count-family subtests, letting each subtest vary only the fields it reasons
// about. IDs are unique so RecordChange does not dedupe them (recordChangeLocked
// drops an incoming change only when its non-empty ID already exists).
func branchcov0723amCountChange(id string, at time.Time, kind ChangeKind, sourceType ChangeSourceType, adapter ChangeSourceAdapter, resourceID string) ResourceChange {
	return ResourceChange{
		ID:            id,
		ResourceID:    resourceID,
		ObservedAt:    at,
		Kind:          kind,
		SourceType:    sourceType,
		SourceAdapter: adapter,
		Confidence:    ConfidenceHigh,
	}
}

// branchcov0723amSeed appends the supplied changes to store in order, failing
// the subtest if any RecordChange errors (e.g. an accidental duplicate id).
func branchcov0723amSeed(t *testing.T, store *MemoryStore, changes ...ResourceChange) {
	t.Helper()
	for _, c := range changes {
		if err := store.RecordChange(c); err != nil {
			t.Fatalf("RecordChange(%s): %v", c.ID, err)
		}
	}
}

// ---------------------------------------------------------------------------
// MemoryStore.CountRecentChangesByKind / CountRecentChangesByKindFiltered
// ---------------------------------------------------------------------------

func TestBranchcov0723Am_CountRecentChangesByKind(t *testing.T) {
	now := branchcov0723amNow

	// emptyStore: source returns a nil map (not a non-nil empty map) when
	// nothing is counted, because of the explicit `if len(counts) == 0`
	// return. The non-Filtered entrypoint must inherit this via delegation.
	t.Run("empty_store_returns_nil_map", func(t *testing.T) {
		store := NewMemoryStore()
		got, err := store.CountRecentChangesByKind("vm:1", now.Add(-time.Hour))
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if got != nil {
			t.Fatalf("got=%#v want nil (empty result must be a nil map)", got)
		}
		// The filtered entrypoint must agree.
		gotF, err := store.CountRecentChangesByKindFiltered("vm:1", now.Add(-time.Hour), ResourceChangeFilters{})
		if err != nil {
			t.Fatalf("filtered err=%v want nil", err)
		}
		if gotF != nil {
			t.Fatalf("filtered got=%#v want nil", gotF)
		}
	})

	// differentCanonicalIDExcluded: changes recorded for vm:1 must not be
	// counted when querying vm:2 (no identity pins relate the two), so the
	// result is nil.
	t.Run("different_canonical_id_excluded", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("ck-diff-1", now, ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
		)
		got, err := store.CountRecentChangesByKind("vm:2", now.Add(-time.Hour))
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if got != nil {
			t.Fatalf("got=%#v want nil (vm:2 must not count vm:1 changes)", got)
		}
	})

	// sinceBoundary: ObservedAt.Before(since) is the gate, so a change AT
	// exactly `since` and one just AFTER are counted, while one just BEFORE
	// is excluded. Asserts the real comparison side (>= on since).
	t.Run("since_boundary_excludes_before_includes_at_and_after", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("ck-before", now.Add(-2*time.Minute), ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("ck-at", now, ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("ck-after", now.Add(time.Minute), ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
		)
		got, err := store.CountRecentChangesByKind("vm:1", now)
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if !reflect.DeepEqual(got, map[ChangeKind]int{ChangeAnomaly: 2}) {
			t.Fatalf("got=%#v want {Anomaly:2} (only at+after must count)", got)
		}
	})

	// zeroSinceAndEmptyCanonicalID: a zero since disables the time gate and
	// an empty canonicalID disables the resource gate, so every change
	// across distinct resources is counted.
	t.Run("zero_since_and_empty_canonical_id_count_all", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("ck-all-1", now.Add(-10*time.Hour), ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("ck-all-2", now.Add(-9*time.Hour), ChangeStateTransition, SourcePlatformEvent, AdapterDocker, "vm:2"),
			branchcov0723amCountChange("ck-all-3", now.Add(-8*time.Hour), ChangeAnomaly, SourceHeuristic, AdapterProxmox, "vm:3"),
		)
		got, err := store.CountRecentChangesByKind("", time.Time{})
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if !reflect.DeepEqual(got, map[ChangeKind]int{ChangeAnomaly: 2, ChangeStateTransition: 1}) {
			t.Fatalf("got=%#v want {Anomaly:2, StateTransition:1} across all resources", got)
		}
	})

	// severalSameKindAndMultipleKinds: assert the concrete count value (3)
	// for one kind and the presence of several kinds at once.
	t.Run("several_same_kind_and_multiple_kinds", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("ck-multi-1", now, ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("ck-multi-2", now, ChangeAnomaly, SourcePulseDiff, AdapterDocker, "vm:1"),
			branchcov0723amCountChange("ck-multi-3", now, ChangeAnomaly, SourcePlatformEvent, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("ck-multi-4", now, ChangeStateTransition, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("ck-multi-5", now, ChangeCapability, SourceHeuristic, AdapterVMware, "vm:1"),
		)
		got, err := store.CountRecentChangesByKind("vm:1", time.Time{})
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		want := map[ChangeKind]int{ChangeAnomaly: 3, ChangeStateTransition: 1, ChangeCapability: 1}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got=%#v want %#v", got, want)
		}
	})

	// filteredExcludesEverything: a filter whose kind is not present must
	// match nothing, yielding the nil-map empty result.
	t.Run("filtered_excludes_everything_returns_nil", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("ck-none-1", now, ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
		)
		got, err := store.CountRecentChangesByKindFiltered("vm:1", time.Time{}, ResourceChangeFilters{
			Kinds: []ChangeKind{ChangeRestart},
		})
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if got != nil {
			t.Fatalf("got=%#v want nil (filter excludes everything)", got)
		}
	})

	// filteredSubsetDiffersFromUnfiltered: a SourceTypes filter keeps only
	// the PulseDiff Anomalies, dropping the PlatformEvent StateTransition,
	// so the filtered result must differ from the unfiltered one.
	t.Run("filtered_subset_differs_from_unfiltered", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("ck-sub-1", now, ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("ck-sub-2", now, ChangeAnomaly, SourcePulseDiff, AdapterDocker, "vm:1"),
			branchcov0723amCountChange("ck-sub-3", now, ChangeStateTransition, SourcePlatformEvent, AdapterProxmox, "vm:1"),
		)
		unfiltered, err := store.CountRecentChangesByKind("vm:1", time.Time{})
		if err != nil {
			t.Fatalf("unfiltered err=%v", err)
		}
		filtered, err := store.CountRecentChangesByKindFiltered("vm:1", time.Time{}, ResourceChangeFilters{
			SourceTypes: []ChangeSourceType{SourcePulseDiff},
		})
		if err != nil {
			t.Fatalf("filtered err=%v", err)
		}
		if reflect.DeepEqual(unfiltered, filtered) {
			t.Fatalf("unfiltered==filtered=%#v (filter had no effect)", filtered)
		}
		if !reflect.DeepEqual(filtered, map[ChangeKind]int{ChangeAnomaly: 2}) {
			t.Fatalf("filtered=%#v want {Anomaly:2}", filtered)
		}
	})

	// filteredIncludeRelated: a change whose ResourceID is NOT the queried
	// canonical id, but which lists it in RelatedResources, is counted only
	// when IncludeRelated is true — covering the includeRelated branch of
	// changeMatchesResource reached through the count path.
	t.Run("filtered_include_related_matches_via_related_resources", func(t *testing.T) {
		store := NewMemoryStore()
		related := branchcov0723amCountChange("ck-rel", now, ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:other")
		related.RelatedResources = []string{"vm:1"}
		branchcov0723amSeed(t, store, related)

		excluded, err := store.CountRecentChangesByKindFiltered("vm:1", time.Time{}, ResourceChangeFilters{IncludeRelated: false})
		if err != nil {
			t.Fatalf("excluded err=%v", err)
		}
		if excluded != nil {
			t.Fatalf("excluded=%#v want nil (related must NOT match when IncludeRelated=false)", excluded)
		}
		included, err := store.CountRecentChangesByKindFiltered("vm:1", time.Time{}, ResourceChangeFilters{IncludeRelated: true})
		if err != nil {
			t.Fatalf("included err=%v", err)
		}
		if !reflect.DeepEqual(included, map[ChangeKind]int{ChangeAnomaly: 1}) {
			t.Fatalf("included=%#v want {Anomaly:1} (related must match when IncludeRelated=true)", included)
		}
	})
}

// ---------------------------------------------------------------------------
// MemoryStore.CountRecentChangesBySourceType / CountRecentChangesBySourceTypeFiltered
// ---------------------------------------------------------------------------

func TestBranchcov0723Am_CountRecentChangesBySourceType(t *testing.T) {
	now := branchcov0723amNow

	t.Run("empty_store_returns_nil_map", func(t *testing.T) {
		store := NewMemoryStore()
		got, err := store.CountRecentChangesBySourceType("vm:1", now.Add(-time.Hour))
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if got != nil {
			t.Fatalf("got=%#v want nil", got)
		}
		gotF, err := store.CountRecentChangesBySourceTypeFiltered("vm:1", now.Add(-time.Hour), ResourceChangeFilters{})
		if err != nil {
			t.Fatalf("filtered err=%v want nil", err)
		}
		if gotF != nil {
			t.Fatalf("filtered got=%#v want nil", gotF)
		}
	})

	t.Run("different_canonical_id_excluded", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("cs-diff-1", now, ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
		)
		got, err := store.CountRecentChangesBySourceType("vm:2", now.Add(-time.Hour))
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if got != nil {
			t.Fatalf("got=%#v want nil", got)
		}
	})

	t.Run("since_boundary_excludes_before_includes_at_and_after", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("cs-before", now.Add(-2*time.Minute), ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("cs-at", now, ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("cs-after", now.Add(time.Minute), ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
		)
		got, err := store.CountRecentChangesBySourceType("vm:1", now)
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if !reflect.DeepEqual(got, map[ChangeSourceType]int{SourcePulseDiff: 2}) {
			t.Fatalf("got=%#v want {PulseDiff:2}", got)
		}
	})

	t.Run("zero_since_and_empty_canonical_id_count_all", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("cs-all-1", now.Add(-10*time.Hour), ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("cs-all-2", now.Add(-9*time.Hour), ChangeStateTransition, SourcePlatformEvent, AdapterDocker, "vm:2"),
			branchcov0723amCountChange("cs-all-3", now.Add(-8*time.Hour), ChangeAnomaly, SourceHeuristic, AdapterProxmox, "vm:3"),
		)
		got, err := store.CountRecentChangesBySourceType("", time.Time{})
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if !reflect.DeepEqual(got, map[ChangeSourceType]int{SourcePulseDiff: 1, SourcePlatformEvent: 1, SourceHeuristic: 1}) {
			t.Fatalf("got=%#v want one of each source type across resources", got)
		}
	})

	t.Run("several_same_source_and_multiple_sources", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("cs-multi-1", now, ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("cs-multi-2", now, ChangeStateTransition, SourcePulseDiff, AdapterDocker, "vm:1"),
			branchcov0723amCountChange("cs-multi-3", now, ChangeCapability, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("cs-multi-4", now, ChangeAnomaly, SourcePlatformEvent, AdapterProxmox, "vm:1"),
		)
		got, err := store.CountRecentChangesBySourceType("vm:1", time.Time{})
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		want := map[ChangeSourceType]int{SourcePulseDiff: 3, SourcePlatformEvent: 1}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got=%#v want %#v", got, want)
		}
	})

	t.Run("filtered_excludes_everything_returns_nil", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("cs-none-1", now, ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
		)
		got, err := store.CountRecentChangesBySourceTypeFiltered("vm:1", time.Time{}, ResourceChangeFilters{
			Kinds: []ChangeKind{ChangeRestart},
		})
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if got != nil {
			t.Fatalf("got=%#v want nil", got)
		}
	})

	// filteredSubset: a Kinds filter keeps only the Anomaly changes, so the
	// PlatformEvent count (driven by a StateTransition) is dropped.
	t.Run("filtered_subset_differs_from_unfiltered", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("cs-sub-1", now, ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("cs-sub-2", now, ChangeAnomaly, SourceHeuristic, AdapterDocker, "vm:1"),
			branchcov0723amCountChange("cs-sub-3", now, ChangeStateTransition, SourcePlatformEvent, AdapterProxmox, "vm:1"),
		)
		unfiltered, err := store.CountRecentChangesBySourceType("vm:1", time.Time{})
		if err != nil {
			t.Fatalf("unfiltered err=%v", err)
		}
		filtered, err := store.CountRecentChangesBySourceTypeFiltered("vm:1", time.Time{}, ResourceChangeFilters{
			Kinds: []ChangeKind{ChangeAnomaly},
		})
		if err != nil {
			t.Fatalf("filtered err=%v", err)
		}
		if reflect.DeepEqual(unfiltered, filtered) {
			t.Fatalf("unfiltered==filtered=%#v (filter had no effect)", filtered)
		}
		if !reflect.DeepEqual(filtered, map[ChangeSourceType]int{SourcePulseDiff: 1, SourceHeuristic: 1}) {
			t.Fatalf("filtered=%#v want {PulseDiff:1, Heuristic:1}", filtered)
		}
	})
}

// ---------------------------------------------------------------------------
// MemoryStore.CountRecentChangesBySourceAdapter / CountRecentChangesBySourceAdapterFiltered
// ---------------------------------------------------------------------------

func TestBranchcov0723Am_CountRecentChangesBySourceAdapter(t *testing.T) {
	now := branchcov0723amNow

	t.Run("empty_store_returns_nil_map", func(t *testing.T) {
		store := NewMemoryStore()
		got, err := store.CountRecentChangesBySourceAdapter("vm:1", now.Add(-time.Hour))
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if got != nil {
			t.Fatalf("got=%#v want nil", got)
		}
		gotF, err := store.CountRecentChangesBySourceAdapterFiltered("vm:1", now.Add(-time.Hour), ResourceChangeFilters{})
		if err != nil {
			t.Fatalf("filtered err=%v want nil", err)
		}
		if gotF != nil {
			t.Fatalf("filtered got=%#v want nil", gotF)
		}
	})

	t.Run("different_canonical_id_excluded", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("ca-diff-1", now, ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
		)
		got, err := store.CountRecentChangesBySourceAdapter("vm:2", now.Add(-time.Hour))
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if got != nil {
			t.Fatalf("got=%#v want nil", got)
		}
	})

	t.Run("since_boundary_excludes_before_includes_at_and_after", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("ca-before", now.Add(-2*time.Minute), ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("ca-at", now, ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("ca-after", now.Add(time.Minute), ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
		)
		got, err := store.CountRecentChangesBySourceAdapter("vm:1", now)
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if !reflect.DeepEqual(got, map[ChangeSourceAdapter]int{AdapterProxmox: 2}) {
			t.Fatalf("got=%#v want {Proxmox:2}", got)
		}
	})

	t.Run("zero_since_and_empty_canonical_id_count_all", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("ca-all-1", now.Add(-10*time.Hour), ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("ca-all-2", now.Add(-9*time.Hour), ChangeStateTransition, SourcePlatformEvent, AdapterDocker, "vm:2"),
			branchcov0723amCountChange("ca-all-3", now.Add(-8*time.Hour), ChangeAnomaly, SourceHeuristic, AdapterProxmox, "vm:3"),
		)
		got, err := store.CountRecentChangesBySourceAdapter("", time.Time{})
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if !reflect.DeepEqual(got, map[ChangeSourceAdapter]int{AdapterProxmox: 2, AdapterDocker: 1}) {
			t.Fatalf("got=%#v want {Proxmox:2, Docker:1} across resources", got)
		}
	})

	t.Run("several_same_adapter_and_multiple_adapters", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("ca-multi-1", now, ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("ca-multi-2", now, ChangeStateTransition, SourcePlatformEvent, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("ca-multi-3", now, ChangeCapability, SourceHeuristic, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("ca-multi-4", now, ChangeAnomaly, SourcePulseDiff, AdapterDocker, "vm:1"),
		)
		got, err := store.CountRecentChangesBySourceAdapter("vm:1", time.Time{})
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		want := map[ChangeSourceAdapter]int{AdapterProxmox: 3, AdapterDocker: 1}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got=%#v want %#v", got, want)
		}
	})

	t.Run("filtered_excludes_everything_returns_nil", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("ca-none-1", now, ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
		)
		got, err := store.CountRecentChangesBySourceAdapterFiltered("vm:1", time.Time{}, ResourceChangeFilters{
			SourceAdapters: []ChangeSourceAdapter{AdapterVMware},
		})
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if got != nil {
			t.Fatalf("got=%#v want nil", got)
		}
	})

	// filteredSubset: a SourceTypes filter keeps only PulseDiff changes, so
	// the PlatformEvent/Proxmox contribution is dropped and the Proxmox
	// adapter count shrinks from 2 to 1.
	t.Run("filtered_subset_differs_from_unfiltered", func(t *testing.T) {
		store := NewMemoryStore()
		branchcov0723amSeed(t, store,
			branchcov0723amCountChange("ca-sub-1", now, ChangeAnomaly, SourcePulseDiff, AdapterProxmox, "vm:1"),
			branchcov0723amCountChange("ca-sub-2", now, ChangeAnomaly, SourcePulseDiff, AdapterDocker, "vm:1"),
			branchcov0723amCountChange("ca-sub-3", now, ChangeStateTransition, SourcePlatformEvent, AdapterProxmox, "vm:1"),
		)
		unfiltered, err := store.CountRecentChangesBySourceAdapter("vm:1", time.Time{})
		if err != nil {
			t.Fatalf("unfiltered err=%v", err)
		}
		filtered, err := store.CountRecentChangesBySourceAdapterFiltered("vm:1", time.Time{}, ResourceChangeFilters{
			SourceTypes: []ChangeSourceType{SourcePulseDiff},
		})
		if err != nil {
			t.Fatalf("filtered err=%v", err)
		}
		if reflect.DeepEqual(unfiltered, filtered) {
			t.Fatalf("unfiltered==filtered=%#v (filter had no effect)", filtered)
		}
		if !reflect.DeepEqual(filtered, map[ChangeSourceAdapter]int{AdapterProxmox: 1, AdapterDocker: 1}) {
			t.Fatalf("filtered=%#v want {Proxmox:1, Docker:1}", filtered)
		}
	})
}

// ---------------------------------------------------------------------------
// MemoryStore.RecordExportAudit / GetExportAudits
// ---------------------------------------------------------------------------

func TestBranchcov0723Am_ExportAudits(t *testing.T) {
	mkRecord := func(id string, at time.Time) ExportAuditRecord {
		return ExportAuditRecord{
			ID:           id,
			Timestamp:    at,
			Actor:        "agent:test",
			EnvelopeHash: "sha256:" + id,
			Decision:     ExportRedacted,
			Destination:  "local-llama",
			Redactions:   []string{"metadata.hostname"},
		}
	}

	// emptyStore: GetExportAudits returns a nil slice (var out is never
	// appended) and no error.
	t.Run("empty_store_returns_nil_slice", func(t *testing.T) {
		store := NewMemoryStore()
		got, err := store.GetExportAudits(time.Time{}, 10)
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if got != nil {
			t.Fatalf("got=%#v want nil", got)
		}
	})

	// recordRoundTripsFields: RecordExportAudit appends; GetExportAudits
	// returns the same concrete field values.
	t.Run("record_then_get_round_trips_fields", func(t *testing.T) {
		store := NewMemoryStore()
		now := branchcov0723amNow
		rec := mkRecord("exp-rt", now)
		if err := store.RecordExportAudit(rec); err != nil {
			t.Fatalf("RecordExportAudit: %v", err)
		}
		got, err := store.GetExportAudits(time.Time{}, 10)
		if err != nil {
			t.Fatalf("GetExportAudits: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("len=%d want 1", len(got))
		}
		if got[0].ID != rec.ID || got[0].Decision != rec.Decision || got[0].Destination != rec.Destination {
			t.Fatalf("round-trip mismatch: %+v", got[0])
		}
		if !reflect.DeepEqual(got[0].Redactions, rec.Redactions) {
			t.Fatalf("redactions=%#v want %#v", got[0].Redactions, rec.Redactions)
		}
		if !got[0].Timestamp.Equal(rec.Timestamp) {
			t.Fatalf("Timestamp=%v want %v", got[0].Timestamp, rec.Timestamp)
		}
	})

	// sinceCutoffExcludesOlder: records older than `since` (strictly Before)
	// are skipped while records at-or-after `since` are returned.
	t.Run("since_cutoff_excludes_older", func(t *testing.T) {
		store := NewMemoryStore()
		base := branchcov0723amNow
		branch := []ExportAuditRecord{
			mkRecord("exp-old", base.Add(-2*time.Hour)),
			mkRecord("exp-mid", base.Add(-1*time.Hour)),
			mkRecord("exp-new", base),
		}
		for _, r := range branch {
			if err := store.RecordExportAudit(r); err != nil {
				t.Fatalf("RecordExportAudit(%s): %v", r.ID, err)
			}
		}
		// since between mid and new: mid (-1h) is before base-30m -> excluded;
		// new (base) is at-or-after -> included; old excluded.
		got, err := store.GetExportAudits(base.Add(-30*time.Minute), 10)
		if err != nil {
			t.Fatalf("GetExportAudits: %v", err)
		}
		if len(got) != 1 || got[0].ID != "exp-new" {
			t.Fatalf("got=%#v want only exp-new", got)
		}
	})

	// sinceInclusiveAtExact: a record whose Timestamp equals `since` must
	// be returned (Before(since) is false), covering the equality side.
	t.Run("since_inclusive_at_exact_timestamp", func(t *testing.T) {
		store := NewMemoryStore()
		at := branchcov0723amNow
		if err := store.RecordExportAudit(mkRecord("exp-at", at)); err != nil {
			t.Fatalf("RecordExportAudit: %v", err)
		}
		got, err := store.GetExportAudits(at, 10)
		if err != nil {
			t.Fatalf("GetExportAudits: %v", err)
		}
		if len(got) != 1 || got[0].ID != "exp-at" {
			t.Fatalf("got=%#v want exp-at (record AT since must be included)", got)
		}
	})

	// orderingMostRecentFirst: GetExportAudits iterates from the last
	// inserted record backwards, so the result is newest-first.
	t.Run("ordering_most_recent_insertion_first", func(t *testing.T) {
		store := NewMemoryStore()
		base := branchcov0723amNow
		ids := []string{"exp-order-1", "exp-order-2", "exp-order-3"}
		for i, id := range ids {
			if err := store.RecordExportAudit(mkRecord(id, base.Add(time.Duration(i)*time.Minute))); err != nil {
				t.Fatalf("RecordExportAudit(%s): %v", id, err)
			}
		}
		got, err := store.GetExportAudits(time.Time{}, 10)
		if err != nil {
			t.Fatalf("GetExportAudits: %v", err)
		}
		gotIDs := make([]string, len(got))
		for i, r := range got {
			gotIDs[i] = r.ID
		}
		wantIDs := []string{"exp-order-3", "exp-order-2", "exp-order-1"}
		if !reflect.DeepEqual(gotIDs, wantIDs) {
			t.Fatalf("order=%#v want %#v (newest-inserted first)", gotIDs, wantIDs)
		}
	})

	// limitSmallerThanMatches: with limit < match count, exactly `limit`
	// records are returned and they are the most-recently-inserted ones.
	t.Run("limit_smaller_than_matches_returns_most_recent", func(t *testing.T) {
		store := NewMemoryStore()
		base := branchcov0723amNow
		ids := []string{"exp-lim-1", "exp-lim-2", "exp-lim-3"}
		for i, id := range ids {
			if err := store.RecordExportAudit(mkRecord(id, base.Add(time.Duration(i)*time.Minute))); err != nil {
				t.Fatalf("RecordExportAudit(%s): %v", id, err)
			}
		}
		got, err := store.GetExportAudits(time.Time{}, 2)
		if err != nil {
			t.Fatalf("GetExportAudits: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len=%d want 2 (limit honoured)", len(got))
		}
		if got[0].ID != "exp-lim-3" || got[1].ID != "exp-lim-2" {
			t.Fatalf("got=%#v want the two most-recent [exp-lim-3, exp-lim-2]", got)
		}
	})

	// limitZeroAndNegativeReturnAll: limit <= 0 disables truncation.
	t.Run("limit_zero_and_negative_return_all", func(t *testing.T) {
		store := NewMemoryStore()
		base := branchcov0723amNow
		for i := 0; i < 3; i++ {
			if err := store.RecordExportAudit(mkRecord("exp-lim0-"+string(rune('a'+i)), base.Add(time.Duration(i)*time.Minute))); err != nil {
				t.Fatalf("RecordExportAudit: %v", err)
			}
		}
		gotZero, err := store.GetExportAudits(time.Time{}, 0)
		if err != nil {
			t.Fatalf("limit=0 err=%v", err)
		}
		if len(gotZero) != 3 {
			t.Fatalf("limit=0 len=%d want 3", len(gotZero))
		}
		gotNeg, err := store.GetExportAudits(time.Time{}, -5)
		if err != nil {
			t.Fatalf("limit=-5 err=%v", err)
		}
		if len(gotNeg) != 3 {
			t.Fatalf("limit=-5 len=%d want 3", len(gotNeg))
		}
	})
}
