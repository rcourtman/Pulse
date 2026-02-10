package unifiedresources

// Unified Resources Architecture — Code Standards Enforcement
//
// This codebase has two data access layers for infrastructure resources:
//
//   1. StateSnapshot (models.StateSnapshot)
//      The canonical typed observation from monitoring. Contains rich typed
//      arrays (state.VMs, state.Containers, state.Nodes, etc.) populated by
//      the monitoring/polling layer. Passed as a function parameter to internal
//      code that needs current metric values or resource iteration.
//
//   2. Unified Resources Registry (unifiedresources.ResourceRegistry)
//      A derived projection built from StateSnapshot via IngestSnapshot().
//      Provides cross-source identity resolution, deduplication, and a
//      normalized resource model. Adds real value for host/agent merging,
//      physical disk dedup (by serial), and Ceph cluster dedup (by FSID).
//
// Policy — which layer to use:
//
//   - API handlers, AI tools, reporting, and consumer-facing surfaces that
//     benefit from cross-source identity: USE THE REGISTRY.
//
//   - Monitoring/polling, mock data generation, ingestion pipelines, and
//     internal computation that naturally iterates typed slices: USE STATE.
//
//   - Resource types with completed registry migrations (PhysicalDisks, Ceph,
//     Storage pools) MUST use the registry in consumer code. Direct state
//     reads for these types are banned in consumer packages.
//
// The tests below enforce these rules by scanning consumer packages for
// banned direct-state access patterns.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// readConsumerGoFiles returns the contents of all non-test .go files in the
// specified directory (relative to the repo internal/ root).
func readConsumerGoFiles(t *testing.T, relDir string) map[string]string {
	t.Helper()

	// Walk up from unifiedresources/ to internal/
	internalDir := filepath.Join("..", relDir)

	files := make(map[string]string)
	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("failed to read %s: %v", path, readErr)
		}
		files[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk %s: %v", relDir, err)
	}
	return files
}

// bannedPattern defines a state access pattern that should not appear in
// consumer code because the resource type has been migrated to the registry.
type bannedPattern struct {
	re      *regexp.Regexp
	message string
}

var migratedResourcePatterns = []bannedPattern{
	{
		re:      regexp.MustCompile(`state\.PhysicalDisks\b`),
		message: "use unified resources registry (GetByType/ListByType with ResourceTypePhysicalDisk) instead of state.PhysicalDisks",
	},
	{
		re:      regexp.MustCompile(`state\.CephClusters\b`),
		message: "use unified resources registry (GetByType/ListByType with ResourceTypeCeph) instead of state.CephClusters",
	},
	{
		re:      regexp.MustCompile(`GetCephClusters\(\)`),
		message: "GetCephClusters() was removed — use unified resources registry instead",
	},
	{
		re:      regexp.MustCompile(`StorageProvider\b`),
		message: "StorageProvider was removed — storage pools are accessed via unified resources registry",
	},
}

// consumerPackage defines a package directory to scan and any files that are
// exempt from the banned patterns (e.g., adapters that bridge between layers).
type consumerPackage struct {
	dir         string
	exemptFiles map[string]bool
}

var consumerPackages = []consumerPackage{
	{dir: "ai/tools", exemptFiles: nil},
	{dir: "ai/chat", exemptFiles: nil},
	{dir: "ai", exemptFiles: map[string]bool{
		// discovery_adapter.go bridges state → service discovery (producer-side)
		"discovery_adapter.go": true,
	}},
	{dir: "api", exemptFiles: map[string]bool{
		// reporting_handlers.go is fully migrated; router.go still has
		// state reads for non-migrated resource types (VMs, Nodes, etc.)
		// which is correct per the architecture policy.
	}},
}

// TestNoDirectStateAccessForMigratedResources ensures that consumer packages
// do not directly access state.PhysicalDisks, state.CephClusters, or use
// removed provider interfaces. These resource types have been migrated to
// the unified resources registry.
func TestNoDirectStateAccessForMigratedResources(t *testing.T) {
	for _, pkg := range consumerPackages {
		files := readConsumerGoFiles(t, pkg.dir)
		for path, content := range files {
			base := filepath.Base(path)
			if pkg.exemptFiles[base] {
				continue
			}
			for _, bp := range migratedResourcePatterns {
				if matches := bp.re.FindAllStringIndex(content, -1); len(matches) > 0 {
					for _, m := range matches {
						line := 1 + strings.Count(content[:m[0]], "\n")
						t.Errorf("%s:%d: %s", path, line, bp.message)
					}
				}
			}
		}
	}
}
