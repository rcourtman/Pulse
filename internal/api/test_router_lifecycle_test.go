package api

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
	"weak"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

// newTestRouter builds a Router and stops what it owns when the test ends.
// NewRouter starts rate limiters, pollers and store workers whose goroutines
// keep the whole Router reachable until they are stopped, so a test that drops
// its Router still holds it for the rest of the package run.
//
// Register t.TempDir and t.Setenv before calling this: cleanups run last-in
// first-out, and the workers may still write to the data directory as they stop.
func newTestRouter(t testing.TB, cfg *config.Config, monitor *monitoring.Monitor, mtMonitor *monitoring.MultiTenantMonitor, wsHub *websocket.Hub, reloadFunc func() error, serverVersion string) *Router {
	t.Helper()
	router := NewRouter(cfg, monitor, mtMonitor, wsHub, reloadFunc, serverVersion)
	t.Cleanup(func() {
		router.ShutdownBackgroundWorkers()
		router.ShutdownResourceStores()
		router.ShutdownRBAC()
	})
	return router
}

// A worker that outlives its Router pins everything the Router built. This is
// the production shutdown path too, so a new worker has to join it.
func TestStoppedRouterBecomesUnreachable(t *testing.T) {
	var built weak.Pointer[Router]
	t.Run("build", func(t *testing.T) {
		built = weak.Make(newTestRouter(t, &config.Config{DataPath: t.TempDir()}, nil, nil, nil, nil, "test"))
	})

	deadline := time.Now().Add(10 * time.Second)
	for built.Value() != nil {
		if time.Now().After(deadline) {
			t.Fatal("Router stayed reachable after its workers stopped")
		}
		runtime.GC()
		time.Sleep(20 * time.Millisecond)
	}
}

// A direct NewRouter call in this package's tests leaks its Router until the
// test binary exits, which is what grew the package's peak memory past the
// release runner's limit.
func TestPackageTestsBuildRoutersThroughLifecycleHelper(t *testing.T) {
	const helperFile = "test_router_lifecycle_test.go"
	direct := regexp.MustCompile(`(^|[^.\w])NewRouter\(`)

	paths, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatalf("list test files: %v", err)
	}
	for _, path := range paths {
		if path == helperFile {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if packageClause(string(src)) == "package api" && direct.Match(src) {
			t.Errorf("%s calls NewRouter directly; use newTestRouter so the Router stops with the test", path)
		}
	}
}

func packageClause(src string) string {
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "package ") {
			return line
		}
	}
	return ""
}
