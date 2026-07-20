package vmware

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// TestBranchcov0720am_FixtureActivityChanges drives every conditional arm of
// FixtureActivityChanges -> activityChangesFromSnapshot -> entityActivityChanges:
//
//   - the empty-snapshot early-return path (no hosts/vms/datastores/networks to
//     iterate, leaving the result slice at length zero),
//   - each of the four per-entity iteration arms (hosts, vms, datastores,
//     networks) projecting tasks and events onto the timeline,
//   - the `if change != nil` filter arm in entityActivityChanges, reached when a
//     task/event with no native id, title, or message causes
//     BuildPlatformActivityChange to return nil,
//   - both arms of the sort comparator (ObservedAt differs vs. ObservedAt equal
//     so the ID tie-breaker fires),
//   - and independence of the returned slice from a fresh call (mutating one
//     result must not bleed into the next).
//
// The projection is behavioural: we assert classification (Kind/SourceType/
// SourceAdapter), key metadata plumbing (connectionId/entityType/
// managedObjectId), ordering outcomes, and filter outcomes rather than pinning
// exact literal change IDs.
func TestBranchcov0720am_FixtureActivityChanges(t *testing.T) {
	// Use fixed, non-zero instants so ObservedAt is deterministic across the
	// sort-driven subtests. Use a non-UTC zone to also exercise the UTC
	// normalisation performed by BuildPlatformActivityChange.
	pst := time.FixedZone("PST", -8*3600)
	earlier := time.Date(2026, time.July, 20, 9, 0, 0, 0, pst) // 17:00 UTC
	later := time.Date(2026, time.July, 20, 11, 0, 0, 0, pst)  // 19:00 UTC
	shared := time.Date(2026, time.July, 20, 12, 0, 0, 0, pst) // 20:00 UTC

	t.Run("empty_snapshot_returns_empty_non_nil_slice", func(t *testing.T) {
		got := FixtureActivityChanges(InventorySnapshot{ConnectionID: "vc-empty"})
		if got == nil {
			t.Fatalf("FixtureActivityChanges(empty) returned nil; want a non-nil empty slice so callers can range safely")
		}
		if len(got) != 0 {
			t.Fatalf("FixtureActivityChanges(empty) returned %d changes; want 0", len(got))
		}
	})

	t.Run("zero_valued_snapshot_returns_empty_non_nil_slice", func(t *testing.T) {
		// A brand-new InventorySnapshot has no ConnectionID and no entities; the
		// four iteration loops should all be no-ops and the sorter should leave
		// the empty slice alone.
		got := FixtureActivityChanges(InventorySnapshot{})
		if got == nil {
			t.Fatalf("FixtureActivityChanges(zero) returned nil; want non-nil empty slice")
		}
		if len(got) != 0 {
			t.Fatalf("FixtureActivityChanges(zero) returned %d changes; want 0", len(got))
		}
	})

	t.Run("host_with_populated_task_emits_activity_change", func(t *testing.T) {
		snapshot := InventorySnapshot{
			ConnectionID: "vc-1",
			Hosts: []InventoryHost{{
				Host:        "host-1",
				RecentTasks: []InventoryTask{{Task: "task-1", Name: "PowerOn", State: "success", StartedAt: earlier}},
			}},
		}
		got := FixtureActivityChanges(snapshot)
		if len(got) != 1 {
			t.Fatalf("expected 1 change for a host with one populated task, got %d", len(got))
		}
		c := got[0]
		if c.Kind != unifiedresources.ChangeActivity {
			t.Errorf("change.Kind = %q, want %q", c.Kind, unifiedresources.ChangeActivity)
		}
		if c.SourceType != unifiedresources.SourcePlatformEvent {
			t.Errorf("change.SourceType = %q, want %q", c.SourceType, unifiedresources.SourcePlatformEvent)
		}
		if c.SourceAdapter != unifiedresources.AdapterVMware {
			t.Errorf("change.SourceAdapter = %q, want %q", c.SourceAdapter, unifiedresources.AdapterVMware)
		}
		// The resource id must thread snapshot.ConnectionID + "host" + host.Host
		// through; assert the components appear rather than pinning the exact
		// canonical string so this remains a behavioural assertion.
		if !strings.Contains(c.ResourceID, "vc-1") || !strings.Contains(c.ResourceID, "host-1") || !strings.Contains(c.ResourceID, "host") {
			t.Errorf("change.ResourceID = %q; want it to carry connection/entity/managed-object components", c.ResourceID)
		}
		// Metadata must carry the provenance keys produced by entityActivityChanges.
		if got := metadataString(c.Metadata, "vmwareConnectionId"); got != "vc-1" {
			t.Errorf("metadata vmwareConnectionId = %q, want %q", got, "vc-1")
		}
		if got := metadataString(c.Metadata, "vmwareEntityType"); got != "host" {
			t.Errorf("metadata vmwareEntityType = %q, want %q", got, "host")
		}
		if got := metadataString(c.Metadata, "vmwareManagedObjectId"); got != "host-1" {
			t.Errorf("metadata vmwareManagedObjectId = %q, want %q", got, "host-1")
		}
		if got := metadataString(c.Metadata, "vmwareTask"); got != "task-1" {
			t.Errorf("metadata vmwareTask = %q, want %q", got, "task-1")
		}
		// CompletedAt falls back to StartedAt via firstNonZeroTime, and then is
		// normalised to UTC by BuildPlatformActivityChange.
		wantOccurredUTC := earlier.UTC()
		if c.OccurredAt == nil || !c.OccurredAt.Equal(wantOccurredUTC) {
			t.Errorf("change.OccurredAt = %v, want %v (CompletedAt->StartedAt fallback in UTC)", c.OccurredAt, wantOccurredUTC)
		}
	})

	t.Run("host_with_populated_event_emits_activity_change", func(t *testing.T) {
		snapshot := InventorySnapshot{
			ConnectionID: "vc-1",
			Hosts: []InventoryHost{{
				Host: "host-1",
				RecentEvents: []InventoryEvent{{
					Event:     "event-1",
					Type:      "HostConnectedEvent",
					Message:   "host reconnected",
					User:      "svc-pulse",
					CreatedAt: earlier,
				}},
			}},
		}
		got := FixtureActivityChanges(snapshot)
		if len(got) != 1 {
			t.Fatalf("expected 1 change for a host with one populated event, got %d", len(got))
		}
		c := got[0]
		if c.Kind != unifiedresources.ChangeActivity {
			t.Errorf("change.Kind = %q, want %q", c.Kind, unifiedresources.ChangeActivity)
		}
		if got := metadataString(c.Metadata, "vmwareEvent"); got != "event-1" {
			t.Errorf("metadata vmwareEvent = %q, want %q", got, "event-1")
		}
		if got := metadataString(c.Metadata, "vmwareEventType"); got != "HostConnectedEvent" {
			t.Errorf("metadata vmwareEventType = %q, want %q", got, "HostConnectedEvent")
		}
		if got := metadataString(c.Metadata, "vmwareEventUser"); got != "svc-pulse" {
			t.Errorf("metadata vmwareEventUser = %q, want %q", got, "svc-pulse")
		}
		if c.Actor != "svc-pulse" {
			t.Errorf("change.Actor = %q, want %q", c.Actor, "svc-pulse")
		}
	})

	t.Run("host_with_whitespace_only_task_is_filtered_out", func(t *testing.T) {
		// A task whose Task/Name/ErrorMessage all trim to empty causes
		// BuildPlatformActivityChange to return nil, so entityActivityChanges
		// drops it via `if change != nil`. This is the only way to reach that
		// filter arm for tasks.
		snapshot := InventorySnapshot{
			ConnectionID: "vc-1",
			Hosts: []InventoryHost{{
				Host: "host-1",
				RecentTasks: []InventoryTask{
					{Task: "   ", Name: "\t", ErrorMessage: " "},
				},
			}},
		}
		got := FixtureActivityChanges(snapshot)
		if len(got) != 0 {
			t.Fatalf("expected whitespace-only task to be filtered out, got %d changes: %+v", len(got), got)
		}
	})

	t.Run("host_with_whitespace_only_event_is_filtered_out", func(t *testing.T) {
		// An event whose Event/Type/Message all trim to empty causes
		// BuildPlatformActivityChange to return nil, exercising the filter arm
		// for events.
		snapshot := InventorySnapshot{
			ConnectionID: "vc-1",
			Hosts: []InventoryHost{{
				Host: "host-1",
				RecentEvents: []InventoryEvent{
					{Event: " ", Type: "\t", Message: ""},
				},
			}},
		}
		got := FixtureActivityChanges(snapshot)
		if len(got) != 0 {
			t.Fatalf("expected whitespace-only event to be filtered out, got %d changes: %+v", len(got), got)
		}
	})

	t.Run("vm_datastore_network_each_project_one_change_per_task", func(t *testing.T) {
		// Exercises all four per-entity iteration arms of
		// activityChangesFromSnapshot in one snapshot. Each entity carries a
		// single populated task; the managed object id differs per entity so we
		// can prove the right entity type threaded its id into ResourceID.
		snapshot := InventorySnapshot{
			ConnectionID: "vc-1",
			VMs: []InventoryVM{{
				VM:          "vm-1",
				RecentTasks: []InventoryTask{{Task: "vm-task", Name: "snapshot", StartedAt: earlier}},
			}},
			Datastores: []InventoryDatastore{{
				Datastore:   "ds-1",
				RecentTasks: []InventoryTask{{Task: "ds-task", Name: "rescan", StartedAt: earlier}},
			}},
			Networks: []InventoryNetwork{{
				Network:     "net-1",
				RecentTasks: []InventoryTask{{Task: "net-task", Name: "refresh", StartedAt: earlier}},
			}},
		}
		got := FixtureActivityChanges(snapshot)
		if len(got) != 3 {
			t.Fatalf("expected 3 changes (one per entity type), got %d", len(got))
		}
		seen := map[string]bool{}
		for _, c := range got {
			seen[c.ResourceID] = true
		}
		for _, want := range []string{"vm-1", "ds-1", "net-1"} {
			found := false
			for rid := range seen {
				if strings.Contains(rid, want) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected a change carrying managed object id %q; ResourceIDs seen = %v", want, seen)
			}
		}
	})

	t.Run("changes_sorted_by_observed_at_descending", func(t *testing.T) {
		// Two tasks at distinct UTC instants; the comparator must take the
		// `!Equal` arm and order the later-OccurredAt change first.
		snapshot := InventorySnapshot{
			ConnectionID: "vc-1",
			Hosts: []InventoryHost{{
				Host: "host-1",
				RecentTasks: []InventoryTask{
					{Task: "earlier-task", Name: "earlier", StartedAt: earlier},
					{Task: "later-task", Name: "later", StartedAt: later},
				},
			}},
		}
		got := FixtureActivityChanges(snapshot)
		if len(got) != 2 {
			t.Fatalf("expected 2 changes, got %d", len(got))
		}
		if !got[0].ObservedAt.After(got[1].ObservedAt) {
			t.Errorf("expected changes sorted by ObservedAt descending; got[0]=%v got[1]=%v", got[0].ObservedAt, got[1].ObservedAt)
		}
		if got := metadataString(got[0].Metadata, "vmwareTask"); got != "later-task" {
			t.Errorf("expected first change to be the later task; got vmwareTask=%q", got)
		}
	})

	t.Run("ties_broken_by_id_descending", func(t *testing.T) {
		// Two tasks at the same instant force the comparator's `Equal` arm,
		// which falls through to `changes[i].ID > changes[j].ID`. We craft two
		// tasks with distinct native ids so they produce distinct change IDs;
		// whichever ID sorts lexicographically higher must come first.
		snapshot := InventorySnapshot{
			ConnectionID: "vc-1",
			Hosts: []InventoryHost{{
				Host: "host-1",
				RecentTasks: []InventoryTask{
					{Task: "task-zzz", Name: "same-time", StartedAt: shared},
					{Task: "task-aaa", Name: "same-time", StartedAt: shared},
				},
			}},
		}
		got := FixtureActivityChanges(snapshot)
		if len(got) != 2 {
			t.Fatalf("expected 2 changes, got %d", len(got))
		}
		if !got[0].ObservedAt.Equal(got[1].ObservedAt) {
			t.Fatalf("precondition failed: expected equal ObservedAt to force the ID tie-breaker; got %v vs %v", got[0].ObservedAt, got[1].ObservedAt)
		}
		// The ID that is lexicographically greater must sort first when the
		// ObservedAt values tie.
		if got[0].ID < got[1].ID {
			t.Errorf("expected ID tie-breaker in descending order; got[0].ID=%q < got[1].ID=%q", got[0].ID, got[1].ID)
		}
		// Cross-check by re-sorting the same changes by ID desc and comparing;
		// this keeps the assertion robust to comparator quirks while still
		// pinning behaviour.
		byID := append([]unifiedresources.ResourceChange(nil), got...)
		sort.SliceStable(byID, func(i, j int) bool { return byID[i].ID > byID[j].ID })
		if byID[0].ID != got[0].ID || byID[1].ID != got[1].ID {
			t.Errorf("tie-break order does not match ID desc; got=%v,%v want=%v,%v", got[0].ID, got[1].ID, byID[0].ID, byID[1].ID)
		}
	})

	t.Run("returned_slice_is_independent_of_subsequent_calls", func(t *testing.T) {
		// Mutating the slice returned by one call (including a metadata map
		// value) must not leak into a second call on the same snapshot. This
		// proves FixtureActivityChanges builds fresh state per invocation
		// rather than aliasing a cached slice or the input snapshot's maps.
		snapshot := InventorySnapshot{
			ConnectionID: "vc-1",
			Hosts: []InventoryHost{{
				Host:        "host-1",
				RecentTasks: []InventoryTask{{Task: "task-1", Name: "PowerOn", StartedAt: earlier}},
			}},
		}
		first := FixtureActivityChanges(snapshot)
		if len(first) != 1 {
			t.Fatalf("precondition: expected 1 change, got %d", len(first))
		}
		originalID := first[0].ID
		originalTask := metadataString(first[0].Metadata, "vmwareTask")

		// Mutate the first result in-place.
		first[0].ID = "mutated-id"
		if m, ok := first[0].Metadata["vmwareTask"].(string); ok {
			first[0].Metadata["vmwareTask"] = m + "-mutated"
		}
		first[0].Metadata["new-key"] = "injected"

		second := FixtureActivityChanges(snapshot)
		if len(second) != 1 {
			t.Fatalf("expected second call to also return 1 change, got %d", len(second))
		}
		if second[0].ID != originalID {
			t.Errorf("second call ID = %q, want %q (first call mutation leaked)", second[0].ID, originalID)
		}
		if got := metadataString(second[0].Metadata, "vmwareTask"); got != originalTask {
			t.Errorf("second call vmwareTask metadata = %q, want %q (metadata mutation leaked)", got, originalTask)
		}
		if _, leaked := second[0].Metadata["new-key"]; leaked {
			t.Errorf("second call metadata contains injected key 'new-key' (metadata map aliased across calls)")
		}
	})

	t.Run("whitespace_in_task_fields_is_trimmed_in_emitted_change", func(t *testing.T) {
		// entityActivityChanges applies strings.TrimSpace to every field before
		// building the change. Driving padded inputs proves the trimming arm is
		// actually exercised end-to-end (rather than asserting on a TrimSpace
		// constant).
		snapshot := InventorySnapshot{
			ConnectionID: "  vc-1  ",
			Hosts: []InventoryHost{{
				Host: "\thost-1\n",
				RecentTasks: []InventoryTask{{
					Task:         "  task-1  ",
					Name:         "  PowerOn  ",
					State:        "  success  ",
					ErrorMessage: "  no-error  ",
					StartedAt:    earlier,
				}},
			}},
		}
		got := FixtureActivityChanges(snapshot)
		if len(got) != 1 {
			t.Fatalf("expected 1 change, got %d", len(got))
		}
		c := got[0]
		if got := metadataString(c.Metadata, "vmwareConnectionId"); got != "vc-1" {
			t.Errorf("vmwareConnectionId not trimmed: %q", got)
		}
		if got := metadataString(c.Metadata, "vmwareManagedObjectId"); got != "host-1" {
			t.Errorf("vmwareManagedObjectId not trimmed: %q", got)
		}
		if got := metadataString(c.Metadata, "vmwareTask"); got != "task-1" {
			t.Errorf("vmwareTask not trimmed: %q", got)
		}
		if got := metadataString(c.Metadata, "vmwareTaskName"); got != "PowerOn" {
			t.Errorf("vmwareTaskName not trimmed: %q", got)
		}
		if got := metadataString(c.Metadata, "vmwareTaskState"); got != "success" {
			t.Errorf("vmwareTaskState not trimmed: %q", got)
		}
		if got := metadataString(c.Metadata, "vmwareTaskError"); got != "no-error" {
			t.Errorf("vmwareTaskError not trimmed: %q", got)
		}
	})
}

// TestBranchcov0720am_ConnectionError_Error exercises both arms of
// (*ConnectionError).Error(): the nil-receiver guard (`if e == nil`) and the
// straight-line `return e.Message` path. The behavioural contract is "Error()
// returns e.Message verbatim when non-nil, and the empty string when the
// receiver is nil" -- so we assert against that contract rather than against a
// formatted string that might smuggle in Category.
func TestBranchcov0720am_ConnectionError_Error(t *testing.T) {
	t.Run("nil_receiver_returns_empty_string", func(t *testing.T) {
		// Calling Error() on a nil *ConnectionError exercises the `if e == nil`
		// arm. This is a real call site: package-internal code stores
		// *ConnectionError in `err` variables that can be nil-typed at runtime.
		var e *ConnectionError
		got := e.Error()
		if got != "" {
			t.Errorf("nil (*ConnectionError).Error() = %q, want %q", got, "")
		}
	})

	t.Run("populated_receiver_returns_message_ignoring_category", func(t *testing.T) {
		// A ConnectionError carries both Category and Message; Error() must
		// surface Message only. Asserting that Category never appears in the
		// result distinguishes the real implementation from a hypothetical
		// "Category: Message" formatter.
		const category = "endpoint"
		const message = "VMware VI JSON API service-instance response was not valid JSON"
		e := &ConnectionError{Category: category, Message: message}
		got := e.Error()
		if got != message {
			t.Errorf("Error() = %q, want exactly Message %q", got, message)
		}
		if strings.Contains(got, category) {
			t.Errorf("Error() = %q unexpectedly embeds Category %q; want Message verbatim", got, category)
		}
	})

	t.Run("empty_message_returns_empty_string", func(t *testing.T) {
		// A non-nil ConnectionError with an empty Message still takes the
		// `return e.Message` arm; the result must be the empty string rather
		// than a fallback like Category or a static placeholder.
		e := &ConnectionError{Category: "auth"}
		if got := e.Error(); got != "" {
			t.Errorf("Error() = %q, want empty string when Message is empty", got)
		}
	})

	t.Run("typed_nil_in_error_interface_still_returns_empty", func(t *testing.T) {
		// A typed-nil pointer stored in an `error` interface is a common shape
		// for `var err error = (*ConnectionError)(nil)`; calling Error() via
		// the interface must still hit the nil-receiver arm rather than panic.
		var ifaceErr error = (*ConnectionError)(nil)
		got := ifaceErr.Error()
		if got != "" {
			t.Errorf("typed-nil error.Error() = %q, want empty string", got)
		}
	})
}

// metadataString reads a string-valued key from a change metadata map without
// panicking on missing keys or non-string values; it returns "" if the key is
// absent so callers can compare against an expected value.
func metadataString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	v, ok := meta[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
