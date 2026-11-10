package unifiedresources

import (
	"fmt"
	"reflect"
	"testing"
)

// Original application-order algorithm: a small independent reference for the
// batch optimization, including repeated predecessors and cycles.
func referenceSettledGroup(group savedGroup, rekeys []rekeyedID) []string {
	members := append([]string(nil), group.members...)
	for _, rekey := range rekeys[group.index+1:] {
		for i, id := range members {
			if id == rekey.oldID {
				members[i] = rekey.newID
			}
		}
	}
	return flattenComponents([][]string{members})
}

func TestSettledGroupsMatchesApplicationOrderExhaustively(t *testing.T) {
	ids := []string{"a", "b", "c"}
	cases := 0
	for length := 1; length <= 4; length++ {
		combinations := 1
		for i := 0; i < length; i++ {
			combinations *= 9
		}
		for code := 0; code < combinations; code++ {
			value := code
			rekeys := make([]rekeyedID, length)
			for i := range rekeys {
				pair := value % 9
				value /= 9
				rekeys[i] = rekeyedID{oldID: ids[pair/3], newID: ids[pair%3]}
			}
			var groups []savedGroup
			for index := range rekeys {
				for mask := 0; mask < 8; mask++ {
					var members []string
					for i, id := range ids {
						if mask&(1<<i) != 0 {
							members = append(members, id)
						}
					}
					groups = append(groups, savedGroup{index: index, members: members})
				}
			}
			got := settledGroups(groups, rekeys)
			for i, group := range groups {
				want := referenceSettledGroup(group, rekeys)
				if !reflect.DeepEqual(got[i], want) {
					t.Fatalf("batch=%v group=%+v: got%v want%v", rekeys, group, got[i], want)
				}
				cases++
			}
		}
	}
	t.Logf("exact old/new group comparisons=%d", cases)
}

func TestSettledGroupsOwnsMembersAndKeepsSparseRepeatedIndices(t *testing.T) {
	rekeys := []rekeyedID{{"a", "b"}, {"b", "c"}, {"c", "a"}, {"b", "d"}}
	groups := []savedGroup{{0, []string{"a", "b", "b", "unknown", ""}}, {0, []string{"c"}}, {2, []string{"a", "b"}}}
	originals := make([][]string, len(groups))
	for i := range groups {
		originals[i] = append([]string(nil), groups[i].members...)
	}
	got := settledGroups(groups, rekeys)
	for i, group := range groups {
		if !reflect.DeepEqual(got[i], referenceSettledGroup(group, rekeys)) {
			t.Fatalf("group%v settled differently", group)
		}
		if !reflect.DeepEqual(group.members, originals[i]) {
			t.Fatal("settlement mutated saved group")
		}
		if len(got[i]) > 0 {
			got[i][0] = "edited-result"
		}
		if !reflect.DeepEqual(group.members, originals[i]) {
			t.Fatal("result aliases saved group")
		}
	}
	if settledGroups(nil, rekeys) != nil {
		t.Fatal("empty group list must stay nil")
	}
}

func settlementBenchmarkBatch(n int) ([]savedGroup, []rekeyedID) {
	groups := make([]savedGroup, n)
	rekeys := make([]rekeyedID, n)
	for i := range rekeys {
		rekeys[i] = rekeyedID{oldID: fmt.Sprintf("old-%06d", i), newID: fmt.Sprintf("new-%06d", i)}
		groups[i] = savedGroup{index: i, members: []string{rekeys[i].newID}}
	}
	return groups, rekeys
}

func BenchmarkSuccessionGroupSettlement(b *testing.B) {
	groups, rekeys := settlementBenchmarkBatch(5000)
	b.Run("before", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			for _, group := range groups {
				referenceSettledGroup(group, rekeys)
			}
		}
	})
	b.Run("batch", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			settledGroups(groups, rekeys)
		}
	})
}
