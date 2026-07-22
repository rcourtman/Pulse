package api

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// This file exercises the previously-uncovered branch arms of three helpers
// that sit on the boundary between the connections ledger and the alerts
// package:
//   - connectionTypeForAlerts        (connections_alerts.go)
//   - snapshotConnectionsForAlerts   (connections_alerts.go)
//   - connectionHostIndex.uniqueMatch (connections_grouping.go)
//
// Every top-level function is prefixed with TestBranchcov0722 so the run can
// be scoped with -run "^TestBranchcov0722".

// TestBranchcov0722ConnectionTypeForAlerts covers every case arm of the
// platform-type switch plus the default fallthrough that signals "drop this
// row before invoking CheckConnection".
func TestBranchcov0722ConnectionTypeForAlerts(t *testing.T) {
	cases := []struct {
		name   string
		input  ConnectionType
		want   alerts.ConnectionType
		wantOK bool
	}{
		{name: "pve", input: ConnectionTypePVE, want: alerts.ConnectionTypePVE, wantOK: true},
		{name: "pbs", input: ConnectionTypePBS, want: alerts.ConnectionTypePBS, wantOK: true},
		{name: "pmg", input: ConnectionTypePMG, want: alerts.ConnectionTypePMG, wantOK: true},
		{name: "vmware", input: ConnectionTypeVMware, want: alerts.ConnectionTypeVMware, wantOK: true},
		{name: "truenas", input: ConnectionTypeTrueNAS, want: alerts.ConnectionTypeTrueNAS, wantOK: true},
		// Non-platform types share the default arm with truly unknown values;
		// both must report ok==false so the caller skips them.
		{name: "agent", input: ConnectionTypeAgent, want: "", wantOK: false},
		{name: "docker", input: ConnectionTypeDocker, want: "", wantOK: false},
		{name: "kubernetes", input: ConnectionTypeKubernetes, want: "", wantOK: false},
		{name: "availability", input: ConnectionTypeAvailability, want: "", wantOK: false},
		{name: "unknown", input: ConnectionType("nope"), want: "", wantOK: false},
		{name: "zero", input: ConnectionType(""), want: "", wantOK: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := connectionTypeForAlerts(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("connectionTypeForAlerts(%q) ok = %v, want %v", tc.input, ok, tc.wantOK)
			}
			if got != tc.want {
				t.Fatalf("connectionTypeForAlerts(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestBranchcov0722SnapshotConnectionsForAlerts asserts the real behaviour
// of the snapshot translator: empty input returns a non-nil empty slice,
// every platform connection maps its full field set, and non-platform
// connections are dropped while the platform siblings survive.
func TestBranchcov0722SnapshotConnectionsForAlerts(t *testing.T) {
	t.Run("empty_input_returns_non_nil_empty_slice", func(t *testing.T) {
		got := snapshotConnectionsForAlerts(nil)
		if got == nil {
			t.Fatalf("expected non-nil slice for nil input, got nil")
		}
		if len(got) != 0 {
			t.Fatalf("expected empty slice, got length %d (%+v)", len(got), got)
		}

		gotEmpty := snapshotConnectionsForAlerts([]Connection{})
		if gotEmpty == nil {
			t.Fatalf("expected non-nil slice for empty input, got nil")
		}
		if len(gotEmpty) != 0 {
			t.Fatalf("expected empty slice, got length %d (%+v)", len(gotEmpty), gotEmpty)
		}
	})

	t.Run("all_platform_connections_map_with_full_field_set", func(t *testing.T) {
		lastSeen := time.Date(2026, 7, 22, 9, 30, 0, 0, time.UTC)
		errAt := time.Date(2026, 7, 22, 9, 25, 0, 0, time.UTC)
		conns := []Connection{
			{
				ID:          "pve:node-1",
				Type:        ConnectionTypePVE,
				Name:        "node-1",
				State:       ConnectionStateUnauthorized,
				StateReason: "token rejected",
				Enabled:     true,
				LastSeen:    &lastSeen,
				LastError: &ConnectionError{
					At:       errAt,
					Message:  "401 Unauthorized",
					Category: "auth",
				},
			},
			{
				ID:    "pbs:store-1",
				Type:  ConnectionTypePBS,
				Name:  "store-1",
				State: ConnectionStateActive,
			},
		}

		got := snapshotConnectionsForAlerts(conns)
		if len(got) != 2 {
			t.Fatalf("expected 2 snapshots, got %d (%+v)", len(got), got)
		}

		// Full field mapping for the rich row (with LastError + LastSeen).
		wantPVE := alerts.ConnectionSnapshot{
			ID:          "pve:node-1",
			Name:        "node-1",
			Type:        alerts.ConnectionTypePVE,
			State:       alerts.ConnectionStateUnauthorized,
			StateReason: "token rejected",
			Enabled:     true,
			LastSeen:    &lastSeen,
			LastError: &alerts.ConnectionErrorSnapshot{
				At:       errAt,
				Message:  "401 Unauthorized",
				Category: "auth",
			},
		}
		if !reflect.DeepEqual(got[0], wantPVE) {
			t.Fatalf("pve snapshot mismatch:\n got  %+v\n want %+v", got[0], wantPVE)
		}
		// The LastSeen pointer is copied by reference, so the snapshot must
		// point at the same *time.Time as the source connection.
		if got[0].LastSeen != &lastSeen {
			t.Fatalf("LastSeen pointer was not copied by reference: got %p, want %p", got[0].LastSeen, &lastSeen)
		}

		// Sparse row with no LastSeen / no LastError: those fields stay nil.
		wantPBS := alerts.ConnectionSnapshot{
			ID:    "pbs:store-1",
			Name:  "store-1",
			Type:  alerts.ConnectionTypePBS,
			State: alerts.ConnectionStateActive,
		}
		if !reflect.DeepEqual(got[1], wantPBS) {
			t.Fatalf("pbs snapshot mismatch:\n got  %+v\n want %+v", got[1], wantPBS)
		}
		if got[1].LastSeen != nil {
			t.Fatalf("pbs snapshot LastSeen must be nil, got %+v", got[1].LastSeen)
		}
		if got[1].LastError != nil {
			t.Fatalf("pbs snapshot LastError must be nil, got %+v", got[1].LastError)
		}
	})

	t.Run("non_platform_types_are_skipped_but_platform_siblings_survive", func(t *testing.T) {
		conns := []Connection{
			{ID: "agent:a", Type: ConnectionTypeAgent, Name: "a", State: ConnectionStateActive},
			{ID: "pve:keep", Type: ConnectionTypePVE, Name: "keep", State: ConnectionStateStale},
			{ID: "docker:d", Type: ConnectionTypeDocker, Name: "d", State: ConnectionStateActive},
			{ID: "avail:x", Type: ConnectionTypeAvailability, Name: "x", State: ConnectionStateActive},
			{ID: "k8s:k", Type: ConnectionTypeKubernetes, Name: "k", State: ConnectionStateActive},
			{ID: "vmware:vc", Type: ConnectionTypeVMware, Name: "vc", State: ConnectionStateUnreachable},
			{ID: "unknown:u", Type: ConnectionType("nope"), Name: "u", State: ConnectionStateActive},
		}

		got := snapshotConnectionsForAlerts(conns)
		if len(got) != 2 {
			t.Fatalf("expected 2 platform snapshots, got %d (%+v)", len(got), got)
		}

		ids := make(map[string]alerts.ConnectionSnapshot, len(got))
		for _, snap := range got {
			ids[snap.ID] = snap
		}

		// The platform survivors must be present with their type mapped.
		pve, ok := ids["pve:keep"]
		if !ok {
			t.Fatalf("pve:keep must survive the skip filter; got IDs %+v", ids)
		}
		if pve.Type != alerts.ConnectionTypePVE {
			t.Fatalf("pve:keep type = %q, want %q", pve.Type, alerts.ConnectionTypePVE)
		}
		if pve.State != alerts.ConnectionStateStale {
			t.Fatalf("pve:keep state = %q, want %q", pve.State, alerts.ConnectionStateStale)
		}

		vmw, ok := ids["vmware:vc"]
		if !ok {
			t.Fatalf("vmware:vc must survive the skip filter; got IDs %+v", ids)
		}
		if vmw.Type != alerts.ConnectionTypeVMware {
			t.Fatalf("vmware:vc type = %q, want %q", vmw.Type, alerts.ConnectionTypeVMware)
		}

		// Every non-platform row must be absent.
		for _, dropped := range []string{"agent:a", "docker:d", "avail:x", "k8s:k", "unknown:u"} {
			if _, present := ids[dropped]; present {
				t.Fatalf("non-platform connection %q must be skipped, but appeared in snapshots %+v", dropped, got)
			}
		}
	})
}

// TestBranchcov0722UniqueMatch exercises connectionHostIndex.uniqueMatch across
// every branch: empty/whitespace raw, missing type bucket, single hit, and
// ambiguous (more than one) hit which must collapse to "" so the caller does
// not silently pick a random primary.
func TestBranchcov0722UniqueMatch(t *testing.T) {
	t.Run("empty_raw_returns_empty", func(t *testing.T) {
		idx := buildConnectionHostIndex([]Connection{
			{ID: "truenas:tn1", Type: ConnectionTypeTrueNAS, Address: "nas.lab.local"},
		})
		if got := idx.uniqueMatch(ConnectionTypeTrueNAS, ""); got != "" {
			t.Fatalf("uniqueMatch(empty) = %q, want %q", got, "")
		}
	})

	t.Run("whitespace_only_raw_returns_empty", func(t *testing.T) {
		idx := buildConnectionHostIndex([]Connection{
			{ID: "truenas:tn1", Type: ConnectionTypeTrueNAS, Address: "nas.lab.local"},
		})
		if got := idx.uniqueMatch(ConnectionTypeTrueNAS, "   \t  "); got != "" {
			t.Fatalf("uniqueMatch(whitespace) = %q, want %q", got, "")
		}
	})

	t.Run("type_with_no_bucket_returns_empty", func(t *testing.T) {
		// Index only knows about TrueNAS; asking for PVE must miss the bucket.
		idx := buildConnectionHostIndex([]Connection{
			{ID: "truenas:tn1", Type: ConnectionTypeTrueNAS, Address: "nas.lab.local"},
		})
		if got := idx.uniqueMatch(ConnectionTypePVE, "nas.lab.local"); got != "" {
			t.Fatalf("uniqueMatch(missing-bucket) = %q, want %q", got, "")
		}
	})

	t.Run("host_with_no_entries_returns_empty", func(t *testing.T) {
		idx := buildConnectionHostIndex([]Connection{
			{ID: "truenas:tn1", Type: ConnectionTypeTrueNAS, Address: "nas.lab.local"},
		})
		if got := idx.uniqueMatch(ConnectionTypeTrueNAS, "does-not-exist.lab.local"); got != "" {
			t.Fatalf("uniqueMatch(no-entries) = %q, want %q", got, "")
		}
	})

	t.Run("single_match_returns_connection_id", func(t *testing.T) {
		idx := buildConnectionHostIndex([]Connection{
			{ID: "truenas:tn1", Type: ConnectionTypeTrueNAS, Address: "nas.lab.local"},
			// A second TrueNAS with a different host must not pollute the match.
			{ID: "truenas:tn2", Type: ConnectionTypeTrueNAS, Address: "other.lab.local"},
		})
		got := idx.uniqueMatch(ConnectionTypeTrueNAS, "nas.lab.local")
		if got != "truenas:tn1" {
			t.Fatalf("uniqueMatch(single) = %q, want %q", got, "truenas:tn1")
		}
	})

	// When two connections normalise to the same host string the match is
	// ambiguous; uniqueMatch returns "" so primaryConnectionIDForResource
	// does not silently pick whichever ID happened to be appended last.
	// This mirrors directProxmoxHostAttachment's "more than one → no match"
	// contract, and is the reason the helper is named unique*Match.
	t.Run("ambiguous_match_returns_empty", func(t *testing.T) {
		idx := buildConnectionHostIndex([]Connection{
			{ID: "truenas:tn1", Type: ConnectionTypeTrueNAS, Address: "nas.lab.local"},
			{ID: "truenas:tn2", Type: ConnectionTypeTrueNAS, Name: "nas.lab.local"},
		})
		got := idx.uniqueMatch(ConnectionTypeTrueNAS, "nas.lab.local")
		if got != "" {
			t.Fatalf("uniqueMatch(ambiguous) = %q, want %q (must not silently pick one of multiple matches)", got, "")
		}
	})
}
