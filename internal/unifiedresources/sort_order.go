package unifiedresources

import "strings"

func canonicalResourceNameKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func compareResourceNameIdentity(
	nameA string,
	typeA ResourceType,
	idA string,
	nameB string,
	typeB ResourceType,
	idB string,
) int {
	if cmp := strings.Compare(canonicalResourceNameKey(nameA), canonicalResourceNameKey(nameB)); cmp != 0 {
		return cmp
	}
	if cmp := strings.Compare(string(typeA), string(typeB)); cmp != 0 {
		return cmp
	}
	return strings.Compare(idA, idB)
}

// CompareResourcesByCanonicalName provides the canonical deterministic ordering
// for unified resources across REST, websocket, and cached read-state views.
func CompareResourcesByCanonicalName(a, b Resource) int {
	return compareResourceNameIdentity(a.Name, a.Type, a.ID, b.Name, b.Type, b.ID)
}

// Per-sort keys avoid normalising mixed-case names O(n log n) times and
// copying a large Resource into every comparison. Keys move with their rows.
type resourceNameSortKey struct {
	name string
	kind ResourceType
	id   string
}

func (k resourceNameSortKey) less(other resourceNameSortKey) bool {
	if k.name != other.name {
		return k.name < other.name
	}
	if k.kind != other.kind {
		return k.kind < other.kind
	}
	return k.id < other.id
}

type resourceNameSort struct {
	rows []Resource
	keys []resourceNameSortKey
}

func (s resourceNameSort) Len() int           { return len(s.rows) }
func (s resourceNameSort) Less(i, j int) bool { return s.keys[i].less(s.keys[j]) }
func (s resourceNameSort) Swap(i, j int) {
	s.rows[i], s.rows[j] = s.rows[j], s.rows[i]
	s.keys[i], s.keys[j] = s.keys[j], s.keys[i]
}

type namedResourceSort[T namedResourceView] struct {
	rows []T
	keys []resourceNameSortKey
}

func (s namedResourceSort[T]) Len() int           { return len(s.rows) }
func (s namedResourceSort[T]) Less(i, j int) bool { return s.keys[i].less(s.keys[j]) }
func (s namedResourceSort[T]) Swap(i, j int) {
	s.rows[i], s.rows[j] = s.rows[j], s.rows[i]
	s.keys[i], s.keys[j] = s.keys[j], s.keys[i]
}
