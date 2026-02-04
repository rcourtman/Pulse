package api

import (
	"strings"
	"testing"
)

func TestRouterDownloadInventory(t *testing.T) {
	literalRoutes, _, _ := parseRouterRoutes(t)

	var downloadRoutes []string
	for _, route := range literalRoutes {
		if strings.HasPrefix(route, "/download/") {
			downloadRoutes = append(downloadRoutes, route)
		}
	}

	expectedAll := sliceToSet(t, downloadRouteAllowlist, "download route allowlist")
	expectedPublic := sliceToSet(t, publicDownloadAllowlist, "public download allowlist")
	expectedProtected := sliceToSet(t, protectedDownloadAllowlist, "protected download allowlist")
	actualAll := sliceToSet(t, downloadRoutes, "router download routes")

	if missing := setDifference(actualAll, expectedAll); len(missing) > 0 {
		t.Fatalf("download routes missing from allowlist: %s", strings.Join(sortedKeys(missing), ", "))
	}
	if stale := setDifference(expectedAll, actualAll); len(stale) > 0 {
		t.Fatalf("download allowlist contains paths not in router.go: %s", strings.Join(sortedKeys(stale), ", "))
	}
	if missing := setDifference(expectedPublic, expectedAll); len(missing) > 0 {
		t.Fatalf("public download allowlist contains unknown routes: %s", strings.Join(sortedKeys(missing), ", "))
	}
	if missing := setDifference(expectedProtected, expectedAll); len(missing) > 0 {
		t.Fatalf("protected download allowlist contains unknown routes: %s", strings.Join(sortedKeys(missing), ", "))
	}

	union := mergeSets(expectedPublic, expectedProtected)
	if missing := setDifference(expectedAll, union); len(missing) > 0 {
		t.Fatalf("download allowlist missing classification: %s", strings.Join(sortedKeys(missing), ", "))
	}

	publicPaths := sliceToSet(t, publicPathsAllowlist, "public paths allowlist")
	if missing := setDifference(expectedPublic, publicPaths); len(missing) > 0 {
		t.Fatalf("public downloads missing from public paths allowlist: %s", strings.Join(sortedKeys(missing), ", "))
	}
	if overlap := setIntersection(expectedProtected, publicPaths); len(overlap) > 0 {
		t.Fatalf("protected downloads should not be public: %s", strings.Join(sortedKeys(overlap), ", "))
	}
}

func mergeSets(a, b map[string]struct{}) map[string]struct{} {
	merged := make(map[string]struct{}, len(a)+len(b))
	for key := range a {
		merged[key] = struct{}{}
	}
	for key := range b {
		merged[key] = struct{}{}
	}
	return merged
}

func setIntersection(a, b map[string]struct{}) map[string]struct{} {
	intersection := make(map[string]struct{})
	for key := range a {
		if _, ok := b[key]; ok {
			intersection[key] = struct{}{}
		}
	}
	return intersection
}

var downloadRouteAllowlist = []string{
	"/download/pulse-docker-agent",
	"/download/pulse-host-agent",
	"/download/pulse-host-agent.sha256",
	"/download/pulse-agent",
}

var publicDownloadAllowlist = []string{
	"/download/pulse-docker-agent",
	"/download/pulse-host-agent",
	"/download/pulse-agent",
}

var protectedDownloadAllowlist = []string{
	"/download/pulse-host-agent.sha256",
}
