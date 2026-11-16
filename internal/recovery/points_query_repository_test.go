package recovery

import (
	"reflect"
	"sort"
	"testing"
)

// The in-memory filter (demo and tests) keeps the same repository semantics as
// the store: Type and Name must match, an empty Namespace or Class matches any
// value, and a filter without a type or name matches nothing.
func TestFilterPointsByRepository(t *testing.T) {
	point := func(id, typ, namespace, name, class string) RecoveryPoint {
		return RecoveryPoint{ID: id, Kind: KindBackup, RepositoryRef: &ExternalRef{Type: typ, Namespace: namespace, Name: name, Class: class}}
	}
	all := []RecoveryPoint{
		point("n1", "proxmox-storage", "pve1", "local", "node1"),
		point("n2", "proxmox-storage", "pve1", "local", "node2"),
		point("other", "proxmox-storage", "pve2", "local", "node1"),
		point("pbs", "proxmox-pbs-datastore", "pbs1", "local", ""),
		{ID: "no-repo", Kind: KindBackup},
	}
	ids := func(filters ...RepositoryFilter) []string {
		var out []string
		for _, p := range FilterPoints(all, ListPointsOptions{Repositories: filters}) {
			out = append(out, p.ID)
		}
		sort.Strings(out)
		return out
	}
	for _, tc := range []struct {
		name    string
		filters []RepositoryFilter
		want    []string
	}{
		{"node", []RepositoryFilter{{Type: "proxmox-storage", Namespace: "pve1", Name: "local", Class: "node1"}}, []string{"n1"}},
		{"any node", []RepositoryFilter{{Type: "proxmox-storage", Namespace: "pve1", Name: "local"}}, []string{"n1", "n2"}},
		{"either", []RepositoryFilter{
			{Type: "proxmox-storage", Namespace: "pve2", Name: "local"},
			{Type: "proxmox-pbs-datastore", Namespace: "pbs1", Name: "local"},
		}, []string{"other", "pbs"}},
		{"no type", []RepositoryFilter{{Name: "local"}}, nil},
		{"none", nil, []string{"n1", "n2", "no-repo", "other", "pbs"}},
	} {
		if got := ids(tc.filters...); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
