package vmware

import (
	"crypto/x509"
	"errors"
	"strings"
	"testing"
	"time"
)

// silentX509Wrap wraps an *x509.UnknownAuthorityError but reports a
// keyword-free Error() string so the substring check in
// classifyTransportError is bypassed and the errors.As branch is the
// only path that can classify the wrapped error as TLS.
type silentX509Wrap struct {
	wrapped error
}

func (w *silentX509Wrap) Error() string { return "innocuous transport failure" }
func (w *silentX509Wrap) Unwrap() error { return w.wrapped }

// fakeNetError implements the net.Error interface (Timeout/Temporary) without
// embedding x509/tls/certificate keywords in its message.
type fakeNetError struct{ msg string }

func (e *fakeNetError) Error() string   { return e.msg }
func (e *fakeNetError) Timeout() bool   { return false }
func (e *fakeNetError) Temporary() bool { return false }

func TestInventoryAlarmSortTime(t *testing.T) {
	pst := time.FixedZone("PST", -8*3600)
	// 10:30 PST == 18:30 UTC. Using a non-UTC input proves the function
	// normalises to UTC rather than returning the raw stored value.
	nonUTC := time.Date(2026, time.January, 15, 10, 30, 0, 0, pst)
	wantUTC := time.Date(2026, time.January, 15, 18, 30, 0, 0, time.UTC)

	cases := []struct {
		name  string
		alarm InventoryAlarm
		want  time.Time
	}{
		{
			name:  "zero triggered_at returns zero time",
			alarm: InventoryAlarm{},
			want:  time.Time{},
		},
		{
			name:  "non-zero triggered_at returns same instant in UTC",
			alarm: InventoryAlarm{TriggeredAt: nonUTC},
			want:  wantUTC,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := inventoryAlarmSortTime(tc.alarm)
			if tc.want.IsZero() {
				if !got.IsZero() {
					t.Fatalf("inventoryAlarmSortTime = %v, want zero time", got)
				}
				return
			}
			if !got.Equal(tc.want) {
				t.Fatalf("inventoryAlarmSortTime = %v, want instant equal to %v", got, tc.want)
			}
			if got.Location() != time.UTC {
				t.Fatalf("inventoryAlarmSortTime location = %v, want UTC", got.Location())
			}
		})
	}
}

func TestInventoryTaskSortTime(t *testing.T) {
	pst := time.FixedZone("PST", -8*3600)
	started := time.Date(2026, time.February, 1, 9, 0, 0, 0, pst)    // 17:00 UTC
	completed := time.Date(2026, time.February, 1, 11, 0, 0, 0, pst) // 19:00 UTC
	wantStartedUTC := time.Date(2026, time.February, 1, 17, 0, 0, 0, time.UTC)
	wantCompletedUTC := time.Date(2026, time.February, 1, 19, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		task InventoryTask
		want time.Time
	}{
		{
			name: "no times returns zero",
			task: InventoryTask{},
			want: time.Time{},
		},
		{
			name: "only completed returns completed utc",
			task: InventoryTask{CompletedAt: completed},
			want: wantCompletedUTC,
		},
		{
			name: "only started returns started utc",
			task: InventoryTask{StartedAt: started},
			want: wantStartedUTC,
		},
		{
			name: "started takes precedence over completed",
			task: InventoryTask{StartedAt: started, CompletedAt: completed},
			want: wantStartedUTC,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := inventoryTaskSortTime(tc.task)
			if tc.want.IsZero() {
				if !got.IsZero() {
					t.Fatalf("inventoryTaskSortTime = %v, want zero time", got)
				}
				return
			}
			if !got.Equal(tc.want) {
				t.Fatalf("inventoryTaskSortTime = %v, want instant equal to %v", got, tc.want)
			}
			if got.Location() != time.UTC {
				t.Fatalf("inventoryTaskSortTime location = %v, want UTC", got.Location())
			}
		})
	}
}

func TestInventoryEventSortTime(t *testing.T) {
	pst := time.FixedZone("PST", -8*3600)
	nonUTC := time.Date(2026, time.March, 12, 5, 15, 0, 0, pst) // 13:15 UTC
	wantUTC := time.Date(2026, time.March, 12, 13, 15, 0, 0, time.UTC)

	cases := []struct {
		name  string
		event InventoryEvent
		want  time.Time
	}{
		{
			name:  "zero created_at returns zero time",
			event: InventoryEvent{},
			want:  time.Time{},
		},
		{
			name:  "non-zero created_at returns same instant in UTC",
			event: InventoryEvent{CreatedAt: nonUTC},
			want:  wantUTC,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := inventoryEventSortTime(tc.event)
			if tc.want.IsZero() {
				if !got.IsZero() {
					t.Fatalf("inventoryEventSortTime = %v, want zero time", got)
				}
				return
			}
			if !got.Equal(tc.want) {
				t.Fatalf("inventoryEventSortTime = %v, want instant equal to %v", got, tc.want)
			}
			if got.Location() != time.UTC {
				t.Fatalf("inventoryEventSortTime location = %v, want UTC", got.Location())
			}
		})
	}
}

func TestIsAutomationNotFound(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error returns false", err: nil, want: false},
		{name: "non-connection error returns false", err: errors.New("boom"), want: false},
		{name: "not_found category returns true", err: &ConnectionError{Category: "not_found", Message: "missing"}, want: true},
		{name: "unavailable category returns false", err: &ConnectionError{Category: "unavailable", Message: "busy"}, want: false},
		{name: "endpoint category returns false", err: &ConnectionError{Category: "endpoint", Message: "boom"}, want: false},
		{name: "empty category returns false", err: &ConnectionError{}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAutomationNotFound(tc.err); got != tc.want {
				t.Fatalf("isAutomationNotFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsAutomationUnavailable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error returns false", err: nil, want: false},
		{name: "non-connection error returns false", err: errors.New("boom"), want: false},
		{name: "unavailable category returns true", err: &ConnectionError{Category: "unavailable", Message: "busy"}, want: true},
		{name: "not_found category returns false", err: &ConnectionError{Category: "not_found", Message: "missing"}, want: false},
		{name: "endpoint category returns false", err: &ConnectionError{Category: "endpoint", Message: "boom"}, want: false},
		{name: "empty category returns false", err: &ConnectionError{}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAutomationUnavailable(tc.err); got != tc.want {
				t.Fatalf("isAutomationUnavailable(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestClassifyTransportError(t *testing.T) {
	cases := []struct {
		name            string
		stage           string
		err             error
		wantCategory    string
		wantMsgContains string
	}{
		{
			name:            "nil error returns nil",
			stage:           "automation session",
			err:             nil,
			wantCategory:    "",
			wantMsgContains: "",
		},
		{
			name:            "x509 substring classifies as tls",
			stage:           "automation session",
			err:             errors.New("Get https://vc/sdk: x509: certificate signed by unknown authority"),
			wantCategory:    "tls",
			wantMsgContains: "VMware TLS validation failed during automation session",
		},
		{
			name:            "certificate substring classifies as tls",
			stage:           "vi-json login",
			err:             errors.New("certificate verify failed"),
			wantCategory:    "tls",
			wantMsgContains: "VMware TLS validation failed during vi-json login",
		},
		{
			name:            "tls substring classifies as tls",
			stage:           "automation session",
			err:             errors.New("tls: handshake failure"),
			wantCategory:    "tls",
			wantMsgContains: "VMware TLS validation failed during automation session",
		},
		{
			name:            "wrapped unknown authority without keyword hits errors.As tls branch",
			stage:           "vi-json service content",
			err:             &silentX509Wrap{wrapped: &x509.UnknownAuthorityError{}},
			wantCategory:    "tls",
			wantMsgContains: "VMware TLS validation failed during vi-json service content",
		},
		{
			name:            "net.Error classifies as network error",
			stage:           "automation session",
			err:             &fakeNetError{msg: "dial tcp: connection refused"},
			wantCategory:    "network",
			wantMsgContains: "VMware network error during automation session",
		},
		{
			name:            "unclassified error falls through to connection failed",
			stage:           "label",
			err:             errors.New("unexpected EOF"),
			wantCategory:    "network",
			wantMsgContains: "VMware connection failed during label",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyTransportError(tc.stage, tc.err)
			if tc.err == nil {
				if got != nil {
					t.Fatalf("classifyTransportError(%q, nil) = %v, want nil", tc.stage, got)
				}
				return
			}
			connErr, ok := got.(*ConnectionError)
			if !ok {
				t.Fatalf("classifyTransportError(%q, %v) = %T, want *ConnectionError", tc.stage, tc.err, got)
			}
			if connErr.Category != tc.wantCategory {
				t.Fatalf("classifyTransportError category = %q, want %q (msg=%q)", connErr.Category, tc.wantCategory, connErr.Message)
			}
			if !strings.Contains(connErr.Message, tc.wantMsgContains) {
				t.Fatalf("classifyTransportError message = %q, want substring %q", connErr.Message, tc.wantMsgContains)
			}
		})
	}
}

func TestVMwareSortKey(t *testing.T) {
	cases := []struct {
		name       string
		id         string
		entityName string
		want       string
	}{
		{name: "both populated returns lowercased trimmed id", id: "VM-1", entityName: "vm one", want: "vm-1"},
		{name: "id wins over name regardless of name case", id: "Host-X", entityName: "lower-priority", want: "host-x"},
		{name: "empty id falls back to lowercased trimmed name", id: "", entityName: "VM Two", want: "vm two"},
		{name: "whitespace-only id falls back to name", id: "   ", entityName: "Spaces", want: "spaces"},
		{name: "empty name returns lowercased trimmed id", id: "vm-1", entityName: "", want: "vm-1"},
		{name: "both empty returns empty", id: "", entityName: "", want: ""},
		{name: "id is trimmed and lowercased", id: "  Host-X  ", entityName: "ignored", want: "host-x"},
		{name: "name is trimmed and lowercased when id empty", id: "", entityName: "  Host-Y  ", want: "host-y"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := vmwareSortKey(tc.id, tc.entityName); got != tc.want {
				t.Fatalf("vmwareSortKey(%q, %q) = %q, want %q", tc.id, tc.entityName, got, tc.want)
			}
		})
	}
}

func TestSortInventorySnapshot(t *testing.T) {
	t.Run("nil snapshot is a no-op", func(t *testing.T) {
		// Should not panic on a nil receiver-by-argument.
		sortInventorySnapshot(nil)
	})

	t.Run("empty snapshot is a no-op", func(t *testing.T) {
		sortInventorySnapshot(&InventorySnapshot{})
	})

	t.Run("each slice sorted ascending by vmwareSortKey", func(t *testing.T) {
		// Each slice is presented in reverse-sorted (z-first, a-second)
		// order so that an in-place sort produces a distinct, checkable
		// permutation. IDs are populated so the sort key is lowercased id.
		snapshot := &InventorySnapshot{
			Hosts: []InventoryHost{
				{Host: "host-z", Name: "Z Host"},
				{Host: "host-a", Name: "A Host"},
			},
			VMs: []InventoryVM{
				{VM: "vm-z", Name: "Z VM"},
				{VM: "vm-a", Name: "A VM"},
			},
			Datastores: []InventoryDatastore{
				{Datastore: "ds-z", Name: "Z DS"},
				{Datastore: "ds-a", Name: "A DS"},
			},
			Clusters: []InventoryCluster{
				{Cluster: "domain-z", Name: "Z Cluster"},
				{Cluster: "domain-a", Name: "A Cluster"},
			},
			Networks: []InventoryNetwork{
				{Network: "net-z", Name: "Z Net"},
				{Network: "net-a", Name: "A Net"},
			},
			EnrichmentIssues: []InventoryEnrichmentIssue{
				{Stage: "topology", EntityType: "cluster", EntityID: "domain-z", Category: "unavailable", Message: "z msg"},
				{Stage: "signals", EntityType: "vm", EntityID: "vm-a", Category: "not_found", Message: "a msg"},
			},
		}

		sortInventorySnapshot(snapshot)

		if got := snapshot.Hosts[0].Host; got != "host-a" {
			t.Fatalf("hosts not sorted ascending; first Host = %q, want %q", got, "host-a")
		}
		if got := snapshot.Hosts[1].Host; got != "host-z" {
			t.Fatalf("hosts not sorted ascending; second Host = %q, want %q", got, "host-z")
		}
		if got := snapshot.VMs[0].VM; got != "vm-a" {
			t.Fatalf("vms not sorted ascending; first VM = %q, want %q", got, "vm-a")
		}
		if got := snapshot.VMs[1].VM; got != "vm-z" {
			t.Fatalf("vms not sorted ascending; second VM = %q, want %q", got, "vm-z")
		}
		if got := snapshot.Datastores[0].Datastore; got != "ds-a" {
			t.Fatalf("datastores not sorted ascending; first Datastore = %q, want %q", got, "ds-a")
		}
		if got := snapshot.Datastores[1].Datastore; got != "ds-z" {
			t.Fatalf("datastores not sorted ascending; second Datastore = %q, want %q", got, "ds-z")
		}
		if got := snapshot.Clusters[0].Cluster; got != "domain-a" {
			t.Fatalf("clusters not sorted ascending; first Cluster = %q, want %q", got, "domain-a")
		}
		if got := snapshot.Clusters[1].Cluster; got != "domain-z" {
			t.Fatalf("clusters not sorted ascending; second Cluster = %q, want %q", got, "domain-z")
		}
		if got := snapshot.Networks[0].Network; got != "net-a" {
			t.Fatalf("networks not sorted ascending; first Network = %q, want %q", got, "net-a")
		}
		if got := snapshot.Networks[1].Network; got != "net-z" {
			t.Fatalf("networks not sorted ascending; second Network = %q, want %q", got, "net-z")
		}
		// Enrichment issue key is "stage\x00entityType\x00..."; "signals"
		// sorts before "topology".
		if got := snapshot.EnrichmentIssues[0].Stage; got != "signals" {
			t.Fatalf("enrichment issues not sorted ascending; first Stage = %q, want %q", got, "signals")
		}
		if got := snapshot.EnrichmentIssues[1].Stage; got != "topology" {
			t.Fatalf("enrichment issues not sorted ascending; second Stage = %q, want %q", got, "topology")
		}

		// Re-sorting an already-sorted snapshot must leave it stable.
		before := snapshot.Hosts[0].Host
		sortInventorySnapshot(snapshot)
		if snapshot.Hosts[0].Host != before {
			t.Fatalf("re-sort changed hosts order; first Host = %q, want %q", snapshot.Hosts[0].Host, before)
		}
	})
}
