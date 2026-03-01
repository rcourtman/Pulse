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
//   - All state.* field access patterns and GetState() calls are
//     enforced via ratchet ceilings below. The ceilings must only
//     decrease over time until all direct state access is eliminated.
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
	{dir: "ai", exemptFiles: nil},
	{dir: "api", exemptFiles: nil},
	{dir: "servicediscovery", exemptFiles: nil},
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

// Ceilings captured after SRC-03 + SRC-04a refresh (Feb 2026).
// These represent legacy nil-fallback paths that are dead code when
// ReadState is wired. Each number must only decrease over time.
//
// Last updated: 2026-03-01 (total state.*: 161, GetState: 30).
// SRC-03f: servicediscovery/service.go migrated to ReadState with legacy fallback.
// SRC-03g: forecast/service.go migrated from StateProvider.GetState to ResourceIterator
// (removed 3 GetState, state.VMs -2, state.Containers -2, state.Nodes -2, state.Storage -1).
// SRC-03h: Removed learnBaselines legacy GetState fallback (router.go) and
// patrol_run.go resource counting legacy fallbacks — ReadState is sole path.
// SRC-03j: Removed legacy state fallbacks in patrol_ai.go seedBackupAnalysis
// and seedHealthAndAlerts (state.VMs -4, state.Containers -4, state.Hosts -2,
// state.KubernetesClusters -2) — ReadState is sole path for these functions.
// SRC-03k: Migrated adapters.go from StateGetter interface to functional closures.
// Removed StateGetter/UpdatesMonitor interfaces from adapters.go. Backup, Replication,
// ConnectionHealth, DiskHealth adapters now use functional getters instead of GetState().
// (GetState -1, state.DockerHosts -1, state.Hosts -1, state.PBSInstances -1,
//
//	plus untracked: state.Backups -1, state.ReplicationJobs -1, state.ConnectionHealth -1).
//
// SRC-03l: Removed shadow StateProvider/StateSnapshot interface from servicediscovery
// and discoveryStateAdapter from ai/discovery_adapter.go. ReadState is now the sole
// state source for servicediscovery. (GetState -2 from servicediscovery, discovery_adapter.go
// exemption removed — no longer needed).
// SRC-03m: Removed legacy StateProvider/GetState fallbacks from ai/adapters/adapters.go
// MetricsAdapter. ReadState is now the sole source for both GetMonitoredResourceIDs and
// GetCurrentMetrics. (GetState -2, state.VMs -2, state.Containers -2, state.Nodes -2,
// state.Storage -1).
// SRC-03n: Removed GetState() from NotificationMonitor interface and NotificationMonitorWrapper.
// TestNotification handler now uses ReadState.Nodes() for node info instead of state.Nodes.
// (GetState -2, state.Nodes -2).
// SRC-03o: Migrated lookupGuestsByVMID and GetDebugContext in ai/service.go from
// stateProvider.GetState() to ReadState accessors. ReadState is now the sole data source
// for guest VMID lookups and debug context resource summaries.
// SRC-03o delta: GetState -2, state.VMs -2, state.Containers -2, state.Nodes -1,
// state.DockerHosts -1, state.Hosts -1, state.PBSInstances -1.
// Ceilings also reflect in-flight SRC-03l (servicediscovery migration) reductions.
// SRC-03p: Removed gatherGuestsFromSnapshot legacy fallback from patrol_intelligence.go.
// ReadState is now the sole path for gatherGuestIntelligence (state.VMs -1, state.Containers -1).
// SRC-03q: Renamed local `state` variable to `snap` in servicediscovery/service.go to eliminate
// 34 false-positive ratchet matches. servicediscovery already used ReadState exclusively (SRC-03l)
// but the local StateSnapshot variable was named `state`, triggering regex matches.
// Delta: state.VMs -7, state.Containers -7, state.Nodes -6, state.DockerHosts -6,
// state.Hosts -5, state.KubernetesClusters -3 (total -34 false positives).
var legacyStateRatchets = []legacyStateRatchet{
	{regexp.MustCompile(`state\.VMs\b`), "state.VMs", 29, "ReadState.VMs()"},
	{regexp.MustCompile(`state\.Containers\b`), "state.Containers", 29, "ReadState.Containers()"},
	{regexp.MustCompile(`state\.Nodes\b`), "state.Nodes", 29, "ReadState.Nodes()"},
	{regexp.MustCompile(`state\.DockerHosts\b`), "state.DockerHosts", 21, "ReadState.DockerHosts()"},
	{regexp.MustCompile(`state\.Hosts\b`), "state.Hosts", 10, "ReadState.Hosts()"},
	{regexp.MustCompile(`state\.Storage\b`), "state.Storage", 16, "ReadState.StoragePools()"},
	{regexp.MustCompile(`state\.KubernetesClusters\b`), "state.KubernetesClusters", 6, "ReadState.K8sClusters()"},
	{regexp.MustCompile(`state\.PBSInstances\b`), "state.PBSInstances", 5, "ReadState.PBSInstances()"},
	{regexp.MustCompile(`state\.PMGInstances\b`), "state.PMGInstances", 2, "ReadState.PMGInstances()"},

	// GetState() calls — consumer packages must use ReadState interface
	{regexp.MustCompile(`\.GetState\(\)`), ".GetState()", 30, "ReadState interface"},
}

// TestLegacyStateAccessRatchet is a monotonic ratchet that prevents new
// state.* direct access and GetState() calls from being added to consumer
// packages.
//
// Existing references are legacy paths being migrated to ReadState. Each
// number must only decrease over time. As migration progresses, lower the
// ceiling numbers above until they reach zero (SRC-04b enforcement).
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
