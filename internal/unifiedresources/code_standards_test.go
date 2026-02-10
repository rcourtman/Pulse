package unifiedresources

// Unified Resources Architecture — Code Standards Enforcement
//
// END STATE (State Read Consolidation — SRC):
//
// The Unified Resources Registry is the PRIMARY read surface for all
// internal business logic. StateSnapshot is the write/ingest buffer and
// frontend wire DTO only.
//
// Architecture:
//
//   1. StateSnapshot (models.StateSnapshot)
//      WRITE-ONLY ingest buffer. Monitoring/polling populates typed arrays.
//      The registry reads from it via IngestSnapshot(). Frontend reads via
//      ToFrontend()/WebSocket. Internal business logic MUST NOT read from
//      StateSnapshot directly — use the ReadState interface instead.
//
//   2. Unified Resources Registry (unifiedresources.ResourceRegistry)
//      The canonical read model. Provides cross-source identity resolution,
//      deduplication, typed views, and a normalized resource model.
//      Implements the ReadState interface with typed accessor methods
//      (VMs(), Containers(), Nodes(), etc.) backed by cached per-type
//      indexes that are O(1) to read and invalidated per ingest cycle.
//
// Consumer package policy:
//
//   - internal/ai/*, internal/api/*, internal/infradiscovery/,
//     internal/servicediscovery/: MUST use ReadState. Direct state reads
//     are banned for all migrated resource types.
//
//   - internal/monitoring/, internal/mock/, internal/models/,
//     internal/websocket/: Exempt (producer/wire-format packages).
//
//   - Resource types with completed migrations are enforced below.
//     As SRC progresses, more patterns will be banned until all
//     state.* access is removed from consumer packages.
//
// See: docs/architecture/state-read-consolidation-plan-2026-02.md
// Progress: docs/architecture/state-read-consolidation-progress-2026-02.md
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

// legacyStateRatchet defines a state.* access pattern and its maximum allowed
// count across all consumer packages. The ceiling is the count at the time
// the ratchet was added. Each cleanup PR that removes fallback paths should
// lower these numbers. Adding new state.* usage will cause the test to fail.
type legacyStateRatchet struct {
	re       *regexp.Regexp
	field    string
	ceiling  int
	readView string // the ReadState accessor to use instead
}

// Ceilings captured after SRC-03 + SRC-04a (Feb 2026).
// These represent legacy nil-fallback paths that are dead code when
// ReadState is wired. Each number must only decrease over time.
var legacyStateRatchets = []legacyStateRatchet{
	{regexp.MustCompile(`state\.VMs\b`), "state.VMs", 49, "ReadState.VMs()"},
	{regexp.MustCompile(`state\.Containers\b`), "state.Containers", 49, "ReadState.Containers()"},
	{regexp.MustCompile(`state\.Nodes\b`), "state.Nodes", 43, "ReadState.Nodes()"},
	{regexp.MustCompile(`state\.DockerHosts\b`), "state.DockerHosts", 45, "ReadState.DockerHosts()"},
	{regexp.MustCompile(`state\.Hosts\b`), "state.Hosts", 26, "ReadState.Hosts()"},
	{regexp.MustCompile(`state\.Storage\b`), "state.Storage", 21, "ReadState.StoragePools()"},
	{regexp.MustCompile(`state\.KubernetesClusters\b`), "state.KubernetesClusters", 19, "ReadState.K8sClusters()"},
	{regexp.MustCompile(`state\.PBSInstances\b`), "state.PBSInstances", 9, "ReadState.PBSInstances()"},
	{regexp.MustCompile(`state\.PMGInstances\b`), "state.PMGInstances", 16, "ReadState.PMGInstances()"},
}

// TestLegacyStateAccessRatchet is a monotonic ratchet that prevents new
// state.* direct access from being added to consumer packages.
//
// Existing references are in nil-fallback branches from the SRC-03 migration.
// They are dead code when ReadState is wired (SRC-04a) but remain as a safety
// net. As fallback branches are removed, lower the ceiling numbers above.
//
// If this test fails with "count N exceeds ceiling M":
//
//	You added new state.* access — use ReadState views instead.
//
// If the count drops below the ceiling:
//
//	Great! Lower the ceiling constant to lock in the improvement.
func TestLegacyStateAccessRatchet(t *testing.T) {
	// Collect all consumer file contents, deduplicating across overlapping
	// package entries (e.g., "ai" walks into "ai/tools" and "ai/chat").
	allFiles := make(map[string]string)
	for _, pkg := range consumerPackages {
		for path, content := range readConsumerGoFiles(t, pkg.dir) {
			allFiles[path] = content
		}
	}
	allContent := make([]string, 0, len(allFiles))
	for _, content := range allFiles {
		allContent = append(allContent, content)
	}

	for _, r := range legacyStateRatchets {
		count := 0
		for _, content := range allContent {
			count += len(r.re.FindAllStringIndex(content, -1))
		}

		if count > r.ceiling {
			t.Errorf("%s: count %d exceeds ceiling %d — use %s instead of adding new %s access",
				r.field, count, r.ceiling, r.readView, r.field)
		}
		if count < r.ceiling {
			t.Logf("%s: count %d is below ceiling %d — lower the ceiling in legacyStateRatchets to lock in this improvement",
				r.field, count, r.ceiling)
		}
	}
}
