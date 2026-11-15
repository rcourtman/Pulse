package config

import (
	"reflect"
	"slices"
	"sync"
	"testing"
)

func TestAPITokenBoundOrgsReadOwnsBindings(t *testing.T) {
	for _, tc := range []struct {
		name   string
		orgID  string
		orgIDs []string
		want   []string
	}{
		{name: "unbound"},
		{name: "empty", orgIDs: []string{}},
		{name: "empty-with-capacity", orgIDs: make([]string, 0, 4)},
		{name: "single", orgID: "tenant-a", want: []string{"tenant-a"}},
		{name: "single-with-empty-list", orgID: "tenant-a", orgIDs: make([]string, 0, 4), want: []string{"tenant-a"}},
		{name: "list-single", orgID: "fallback", orgIDs: []string{"tenant-a"}, want: []string{"tenant-a"}},
		{name: "list-multiple", orgID: "fallback", orgIDs: []string{"tenant-b", "tenant-a"}, want: []string{"tenant-b", "tenant-a"}},
		{name: "literal-values", orgID: "fallback", orgIDs: []string{"", " tenant-a ", "tenant-a", "tenant-a"}, want: []string{"", " tenant-a ", "tenant-a", "tenant-a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Keep spare capacity so appending to a read must not leave aliases
			// into the inventory either. Preserve nil versus empty in the record.
			record := APITokenRecord{OrgID: tc.orgID}
			if tc.orgIDs != nil {
				record.OrgIDs = make([]string, len(tc.orgIDs), len(tc.orgIDs)+4)
				copy(record.OrgIDs, tc.orgIDs)
			}
			before := record
			before.OrgIDs = slices.Clone(record.OrgIDs)
			first, second := record.GetBoundOrgs(), record.GetBoundOrgs()
			if !reflect.DeepEqual(first, tc.want) || !reflect.DeepEqual(second, tc.want) {
				t.Fatal("read changed unbound shape, list precedence, ordering or literal binding values")
			}
			for _, org := range []string{"", "tenant-a", "tenant-b", "fallback", " tenant-a ", "snapshot-only"} {
				wantOnly := org != "" && len(tc.want) == 1 && tc.want[0] == org
				if record.BoundOnlyToOrg(org) != wantOnly {
					t.Fatalf("single-organization ownership changed for %q", org)
				}
			}
			if len(first) > 0 {
				first[0] = "snapshot-only"
			}
			first = append(first, "appended-snapshot-only")
			if !reflect.DeepEqual(record, before) || !reflect.DeepEqual(second, tc.want) {
				t.Fatal("editing a binding read rewrote the token or a peer read")
			}
			if record.CanAccessOrg("snapshot-only") || record.CanAccessOrg("appended-snapshot-only") {
				t.Fatal("editing a binding read granted access to another organization")
			}
			ownedRead := slices.Clone(first)
			if len(record.OrgIDs) > 0 {
				record.OrgIDs[0] = "inventory-only"
			}
			record.OrgIDs = append(record.OrgIDs, "appended-inventory-only")
			record.OrgID = "changed-fallback"
			if !reflect.DeepEqual(first, ownedRead) || !reflect.DeepEqual(second, tc.want) {
				t.Fatal("editing the token rewrote an already returned binding read")
			}
		})
	}
}

func TestAPITokenBoundOrgsConcurrentReadOwnership(t *testing.T) {
	cfg := &Config{}
	cfg.UpsertAPIToken(APITokenRecord{ID: "fixture-binding-read", OrgID: "fallback", OrgIDs: []string{"tenant-a", "tenant-b"}, Scopes: []string{ScopeMonitoringRead}})
	record := &cfg.APITokens[0]
	before := record.Clone()
	start := make(chan struct{})
	var workers sync.WaitGroup
	for worker := 0; worker < 6; worker++ {
		workers.Go(func() {
			<-start
			for i := 0; i < 200; i++ {
				// Callers may edit their own snapshot while other request readers
				// continue using the same live, otherwise immutable token record.
				bound := record.GetBoundOrgs()
				bound[0] = "snapshot-only"
				bound = append(bound, "appended-snapshot-only")
				if !record.CanAccessOrg("tenant-a") || !record.CanAccessOrg("tenant-b") ||
					record.CanAccessOrg("snapshot-only") || record.CanAccessOrg("fallback") ||
					record.BoundOnlyToOrg("tenant-a") || !record.HasScope(ScopeMonitoringRead) {
					t.Error("caller-owned binding edit changed live credential authority")
					return
				}
			}
		})
	}
	close(start)
	workers.Wait()
	if !reflect.DeepEqual(*record, before) {
		t.Fatal("concurrent binding reads changed saved token metadata or authority")
	}
}

func BenchmarkAPITokenBoundOnlyToOrg(b *testing.B) {
	for _, tc := range []struct {
		name   string
		record APITokenRecord
		want   bool
	}{
		{"unbound", APITokenRecord{}, false},
		{"single", APITokenRecord{OrgID: "tenant-a"}, true},
		{"list-single", APITokenRecord{OrgIDs: []string{"tenant-a"}}, true},
		{"list-multiple", APITokenRecord{OrgIDs: []string{"tenant-a", "tenant-b"}}, false},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if tc.record.BoundOnlyToOrg("tenant-a") != tc.want {
					b.Fatal("single-organization ownership decision changed")
				}
			}
		})
	}
}

func TestAPITokenBoundOrgsReadOwnsSpareCapacity(t *testing.T) {
	// Loaded JSON arrays may have spare capacity. An append through a read
	// must not write even the currently hidden slots of the live inventory.
	record := APITokenRecord{OrgIDs: make([]string, 5, 8)}
	copy(record.OrgIDs, []string{"tenant-a", "tenant-b", "tenant-c", "tenant-d", "tenant-e"})
	backing := record.OrgIDs[:cap(record.OrgIDs)]
	before := slices.Clone(backing)
	bound := record.GetBoundOrgs()
	bound = append(bound, "snapshot-only")
	if bound[len(bound)-1] != "snapshot-only" || !reflect.DeepEqual(backing, before) {
		t.Fatal("appending to a binding read changed the inventory's backing storage")
	}
}

func TestAPITokenBoundOnlyToOrgKeepsExactOwnership(t *testing.T) {
	for _, tc := range []struct {
		name   string
		record APITokenRecord
		orgID  string
		want   bool
	}{
		{"unbound", APITokenRecord{}, "tenant-a", false},
		{"empty-request", APITokenRecord{OrgID: "tenant-a"}, "", false},
		{"single", APITokenRecord{OrgID: "tenant-a"}, "tenant-a", true},
		{"foreign-single", APITokenRecord{OrgID: "tenant-b"}, "tenant-a", false},
		{"empty-list-fallback", APITokenRecord{OrgID: "tenant-a", OrgIDs: []string{}}, "tenant-a", true},
		{"list-single", APITokenRecord{OrgID: "fallback", OrgIDs: []string{"tenant-a"}}, "tenant-a", true},
		{"list-ignores-fallback", APITokenRecord{OrgID: "tenant-a", OrgIDs: []string{"tenant-b"}}, "tenant-a", false},
		{"multiple", APITokenRecord{OrgIDs: []string{"tenant-a", "tenant-b"}}, "tenant-a", false},
		{"duplicates-are-not-single", APITokenRecord{OrgIDs: []string{"tenant-a", "tenant-a"}}, "tenant-a", false},
		{"blank-list-ignores-fallback", APITokenRecord{OrgID: "tenant-a", OrgIDs: []string{""}}, "tenant-a", false},
		{"blank-list-empty-request", APITokenRecord{OrgIDs: []string{""}}, "", false},
		{"literal-list-space", APITokenRecord{OrgIDs: []string{" tenant-a "}}, "tenant-a", false},
		{"matching-list-space", APITokenRecord{OrgIDs: []string{" tenant-a "}}, " tenant-a ", true},
		{"literal-single-space", APITokenRecord{OrgID: " tenant-a "}, "tenant-a", false},
		{"matching-single-space", APITokenRecord{OrgID: " tenant-a "}, " tenant-a ", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.record.BoundOnlyToOrg(tc.orgID); got != tc.want {
				t.Fatalf("single-organization ownership = %t, want %t", got, tc.want)
			}
		})
	}
}
