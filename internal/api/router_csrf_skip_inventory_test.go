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

func TestRouterCSRFSkipInventory(t *testing.T) {
	skipPaths := parseCSRFSkipPaths(t)

	expected := sliceToSet(t, csrfSkipAllowlist, "CSRF skip allowlist")
	actual := sliceToSet(t, skipPaths, "router CSRF skip paths")

	if missing := setDifference(actual, expected); len(missing) > 0 {
		t.Fatalf("CSRF skip paths missing from allowlist: %s", strings.Join(sortedKeys(missing), ", "))
	}
	if stale := setDifference(expected, actual); len(stale) > 0 {
		t.Fatalf("CSRF skip allowlist contains paths not skipped in router.go: %s", strings.Join(sortedKeys(stale), ", "))
	}
}

func parseCSRFSkipPaths(t *testing.T) []string {
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

	start := strings.Index(src, "// Check CSRF for state-changing requests")
	if start == -1 {
		t.Fatalf("CSRF check block not found in router.go")
	}
	end := strings.Index(src[start:], "if strings.HasPrefix(req.URL.Path, \"/api/\") && !skipCSRF")
	if end == -1 {
		t.Fatalf("CSRF enforcement block not found in router.go")
	}
	block := src[start : start+end]

	re := regexp.MustCompile(`req\.URL\.Path == "([^"]+)"`)
	matches := re.FindAllStringSubmatch(block, -1)
	if len(matches) == 0 {
		t.Fatalf("no CSRF skip paths found in router.go")
	}

	paths := make([]string, 0, len(matches))
	seen := map[string]struct{}{}
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

var csrfSkipAllowlist = []string{
	"/api/login",
	"/api/public/magic-link/request",
	"/api/public/signup",
	"/api/security/apply-restart",
	"/api/security/quick-setup",
	"/api/security/validate-bootstrap-token",
	"/api/setup-script-url",
}
