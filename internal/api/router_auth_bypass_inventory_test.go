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

func TestRouterAuthBypassInventory(t *testing.T) {
	bypassPaths := parseAuthBypassPaths(t)

	expected := sliceToSet(t, authBypassAllowlist, "auth bypass allowlist")
	actual := sliceToSet(t, bypassPaths, "router auth bypass paths")
	publicRoutes := sliceToSet(t, publicRouteAllowlist, "public route allowlist")

	if missing := setDifference(actual, expected); len(missing) > 0 {
		t.Fatalf("auth bypass paths missing from allowlist: %s", strings.Join(sortedKeys(missing), ", "))
	}
	if stale := setDifference(expected, actual); len(stale) > 0 {
		t.Fatalf("auth bypass allowlist contains paths not in router.go: %s", strings.Join(sortedKeys(stale), ", "))
	}
	if missing := setDifference(expected, publicRoutes); len(missing) > 0 {
		t.Fatalf("auth bypass paths must be listed as public: %s", strings.Join(sortedKeys(missing), ", "))
	}
}

func parseAuthBypassPaths(t *testing.T) []string {
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

	start := strings.Index(src, "normalizedPath := path.Clean")
	if start == -1 {
		t.Fatalf("normalizedPath block not found in router.go")
	}
	end := strings.Index(src[start:], "Dev mode bypass for admin endpoints")
	if end == -1 {
		t.Fatalf("auth bypass block end not found in router.go")
	}
	block := src[start : start+end]

	re := regexp.MustCompile(`normalizedPath == "([^"]+)"`)
	matches := re.FindAllStringSubmatch(block, -1)
	if len(matches) == 0 {
		t.Fatalf("no auth bypass paths found in router.go")
	}

	seen := map[string]struct{}{}
	paths := make([]string, 0, len(matches))
	for _, match := range matches {
		path := match[1]
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

var authBypassAllowlist = []string{
	"/api/auto-register",
	"/api/setup-script",
	"/api/system/ssh-config",
	"/api/system/verify-temperature-ssh",
}
