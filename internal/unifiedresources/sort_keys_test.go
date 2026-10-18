package unifiedresources

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"testing"
)

func referenceSortResources(rows []Resource) {
	sort.SliceStable(rows, func(i, j int) bool { return CompareResourcesByCanonicalName(rows[i], rows[j]) < 0 })
}
func sortKeyFixture(count int) []Resource {
	r := rand.New(rand.NewSource(2199))
	rows := make([]Resource, count)
	names := []string{"  Mixed-Case  ", "mixed-case", "Σίσυφος", " İSTANBUL ", "", "ZZZ", "aaa"}
	for i := range rows {
		rows[i] = Resource{ID: fmt.Sprintf("%d", r.Intn(count/2+1)), Name: names[r.Intn(len(names))], Type: ResourceType(fmt.Sprintf("kind-%d", r.Intn(4))), Uptime: int64(i)}
	}
	return rows
}
func TestPrecomputedResourceSortMatchesCanonicalStableOrder(t *testing.T) {
	for _, count := range []int{0, 1, 2, 17, 1236} {
		rows := sortKeyFixture(count)
		want := append([]Resource(nil), rows...)
		referenceSortResources(want)
		sortResourcesByName(rows)
		if !reflect.DeepEqual(rows, want) && len(rows) > 0 {
			t.Fatalf("canonical/stable order changed at size %d", count)
		}
	}
}

type sortTestView struct {
	name, id string
	sequence int
}

func (v sortTestView) Name() string { return v.name }
func (v sortTestView) ID() string   { return v.id }
func TestPrecomputedViewSortMatchesCanonicalStableOrder(t *testing.T) {
	rows := []sortTestView{{" B ", "z", 0}, {"b", "a", 1}, {"b", "a", 2}, {"A", "c", 3}, {"", "d", 4}, {"İ", "e", 5}}
	want := append([]sortTestView(nil), rows...)
	sort.SliceStable(want, func(i, j int) bool {
		return compareResourceNameIdentity(want[i].Name(), "", want[i].ID(), want[j].Name(), "", want[j].ID()) < 0
	})
	sortNamedResourceViewsByName(rows)
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("view order changed: %v", rows)
	}
}
func BenchmarkCanonicalResourceSort(b *testing.B) {
	seed := sortKeyFixture(1236)
	for _, impl := range []struct {
		name string
		sort func([]Resource)
	}{{"reference", referenceSortResources}, {"precomputed", sortResourcesByName}} {
		b.Run(impl.name, func(b *testing.B) {
			rows := make([]Resource, len(seed))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy(rows, seed)
				impl.sort(rows)
			}
		})
	}
}
