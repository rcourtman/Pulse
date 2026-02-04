package api

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestRouterFrontendRouteInventory(t *testing.T) {
	block := extractRouterBlock(t, "isFrontendRoute :=", "isStaticAsset :=")

	prefixes := matchAll(block, `!strings\.HasPrefix\(req\.URL\.Path, "([^"]+)"\)`)
	exacts := matchAll(block, `req\.URL\.Path != "([^"]+)"`)

	expectedPrefixes := sliceToSet(t, frontendRoutePrefixExclusionAllowlist, "frontend route prefix allowlist")
	expectedExacts := sliceToSet(t, frontendRouteExactExclusionAllowlist, "frontend route exact allowlist")

	actualPrefixes := sliceToSet(t, prefixes, "frontend route prefixes")
	actualExacts := sliceToSet(t, exacts, "frontend route exacts")

	if missing := setDifference(actualPrefixes, expectedPrefixes); len(missing) > 0 {
		t.Fatalf("frontend route prefixes missing from allowlist: %s", strings.Join(sortedKeys(missing), ", "))
	}
	if stale := setDifference(expectedPrefixes, actualPrefixes); len(stale) > 0 {
		t.Fatalf("frontend prefix allowlist contains entries not in router.go: %s", strings.Join(sortedKeys(stale), ", "))
	}
	if missing := setDifference(actualExacts, expectedExacts); len(missing) > 0 {
		t.Fatalf("frontend exact exclusions missing from allowlist: %s", strings.Join(sortedKeys(missing), ", "))
	}
	if stale := setDifference(expectedExacts, actualExacts); len(stale) > 0 {
		t.Fatalf("frontend exact allowlist contains entries not in router.go: %s", strings.Join(sortedKeys(stale), ", "))
	}
}

func TestRouterStaticAssetInventory(t *testing.T) {
	block := extractRouterBlock(t, "isStaticAsset :=", "isPublic :=")

	prefixes := matchAll(block, `strings\.HasPrefix\(req\.URL\.Path, "([^"]+)"\)`)
	suffixes := matchAll(block, `strings\.HasSuffix\(req\.URL\.Path, "([^"]+)"\)`)
	exacts := matchAll(block, `req\.URL\.Path == "([^"]+)"`)

	expectedPrefixes := sliceToSet(t, staticAssetPrefixAllowlist, "static asset prefix allowlist")
	expectedSuffixes := sliceToSet(t, staticAssetSuffixAllowlist, "static asset suffix allowlist")
	expectedExacts := sliceToSet(t, staticAssetExactAllowlist, "static asset exact allowlist")

	actualPrefixes := sliceToSet(t, prefixes, "static asset prefixes")
	actualSuffixes := sliceToSet(t, suffixes, "static asset suffixes")
	actualExacts := sliceToSet(t, exacts, "static asset exacts")

	if missing := setDifference(actualPrefixes, expectedPrefixes); len(missing) > 0 {
		t.Fatalf("static asset prefixes missing from allowlist: %s", strings.Join(sortedKeys(missing), ", "))
	}
	if stale := setDifference(expectedPrefixes, actualPrefixes); len(stale) > 0 {
		t.Fatalf("static asset prefix allowlist contains entries not in router.go: %s", strings.Join(sortedKeys(stale), ", "))
	}
	if missing := setDifference(actualSuffixes, expectedSuffixes); len(missing) > 0 {
		t.Fatalf("static asset suffixes missing from allowlist: %s", strings.Join(sortedKeys(missing), ", "))
	}
	if stale := setDifference(expectedSuffixes, actualSuffixes); len(stale) > 0 {
		t.Fatalf("static asset suffix allowlist contains entries not in router.go: %s", strings.Join(sortedKeys(stale), ", "))
	}
	if missing := setDifference(actualExacts, expectedExacts); len(missing) > 0 {
		t.Fatalf("static asset exacts missing from allowlist: %s", strings.Join(sortedKeys(missing), ", "))
	}
	if stale := setDifference(expectedExacts, actualExacts); len(stale) > 0 {
		t.Fatalf("static asset exact allowlist contains entries not in router.go: %s", strings.Join(sortedKeys(stale), ", "))
	}
}

func extractRouterBlock(t *testing.T, startMarker, endMarker string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to locate test file path")
	}
	routerPath := filepath.Join(filepath.Dir(file), "router.go")
	data, err := os.ReadFile(routerPath)
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}

	src := string(data)
	start := strings.Index(src, startMarker)
	if start == -1 {
		t.Fatalf("marker %q not found in router.go", startMarker)
	}
	end := strings.Index(src[start:], endMarker)
	if end == -1 {
		t.Fatalf("marker %q not found after %q in router.go", endMarker, startMarker)
	}
	return src[start : start+end]
}

func matchAll(block, pattern string) []string {
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(block, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		value := match[1]
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

var frontendRoutePrefixExclusionAllowlist = []string{
	"/api/",
	"/ws",
	"/socket.io/",
	"/download/",
}

var frontendRouteExactExclusionAllowlist = []string{
	"/simple-stats",
	"/install-docker-agent.sh",
	"/install-container-agent.sh",
	"/install-host-agent.sh",
	"/install-host-agent.ps1",
	"/uninstall-host-agent.sh",
	"/uninstall-host-agent.ps1",
	"/install.sh",
	"/install.ps1",
}

var staticAssetPrefixAllowlist = []string{
	"/assets/",
	"/@vite/",
	"/@solid-refresh",
	"/src/",
	"/node_modules/",
}

var staticAssetSuffixAllowlist = []string{
	".js",
	".css",
	".map",
	".ts",
	".tsx",
	".mjs",
	".jsx",
}

var staticAssetExactAllowlist = []string{
	"/",
	"/index.html",
	"/favicon.ico",
	"/logo.svg",
}
