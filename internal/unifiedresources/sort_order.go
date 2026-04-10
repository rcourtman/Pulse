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
