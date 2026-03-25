package monitoring

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

var bannedSnapshotResourceAccessPatterns = []struct {
	re      *regexp.Regexp
	message string
}{
	{
		re:      regexp.MustCompile(`GetState\(\)\.(VMs|Containers|Nodes|Hosts|DockerHosts|PBSInstances|Storage|PhysicalDisks)\b`),
		message: "derive canonical monitoring resource views through ReadState-backed helpers instead of GetState() resource arrays",
	},
}

func readMonitoringRuntimeFiles(t *testing.T) map[string]string {
	t.Helper()

	files := make(map[string]string)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read monitoring directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		files[name] = string(data)
	}

	return files
}

func TestNoGetStateResourceArrayRegression(t *testing.T) {
	for name, content := range readMonitoringRuntimeFiles(t) {
		for _, pattern := range bannedSnapshotResourceAccessPatterns {
			if matches := pattern.re.FindAllStringIndex(content, -1); len(matches) > 0 {
				for _, match := range matches {
					line := 1 + strings.Count(content[:match[0]], "\n")
					t.Errorf("%s:%d: %s", name, line, pattern.message)
				}
			}
		}
	}
}

func TestConnectedInfrastructureUsesSharedTopLevelSystemResolver(t *testing.T) {
	data, err := os.ReadFile("connected_infrastructure.go")
	if err != nil {
		t.Fatalf("failed to read connected_infrastructure.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"resolver := unifiedresources.ResolveTopLevelSystems(resources)",
		"key := resolver.GroupIDForResource(resource)",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("connected_infrastructure.go must contain %q", snippet)
		}
	}
}

func TestLegacyMemorySourceAliasesRemainCanonicalized(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source    string
		canonical string
	}{
		{source: "avail-field", canonical: "available-field"},
		{source: "meminfo-available", canonical: "available-field"},
		{source: "node-status-available", canonical: "available-field"},
		{source: "meminfo-derived", canonical: "derived-free-buffers-cached"},
		{source: "calculated", canonical: "derived-free-buffers-cached"},
		{source: "meminfo-total-minus-used", canonical: "derived-total-minus-used"},
		{source: "rrd-available", canonical: "rrd-memavailable"},
		{source: "rrd-data", canonical: "rrd-memused"},
		{source: "listing-mem", canonical: "cluster-resources"},
		{source: "listing", canonical: "cluster-resources"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.source, func(t *testing.T) {
			t.Parallel()
			if got := CanonicalMemorySource(tt.source); got != tt.canonical {
				t.Fatalf("CanonicalMemorySource(%q) = %q, want %q", tt.source, got, tt.canonical)
			}
		})
	}
}

func TestProxmoxNodeDiskUsesCanonicalResolver(t *testing.T) {
	data, err := os.ReadFile("monitor_polling_node.go")
	if err != nil {
		t.Fatalf("failed to read monitor_polling_node.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"modelNode.Disk, _ = m.resolveNodeDisk(",
		"if resolvedDisk, diskSource := m.resolveNodeDisk(",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor_polling_node.go must contain %q", snippet)
		}
	}
}

func TestProxmoxGuestPollersCarryPoolIntoCanonicalModels(t *testing.T) {
	requiredSnippets := map[string][]string{
		"monitor_pve_guest_builders.go": {"Pool:     strings.TrimSpace(res.Pool)"},
		"monitor_pve_guest_lxc.go":      {"Pool:     strings.TrimSpace(res.Pool)"},
		"monitor_polling_vm.go":         {"Pool:     strings.TrimSpace(vm.Pool)"},
		"monitor_polling_containers.go": {"Pool:     strings.TrimSpace(container.Pool)"},
	}

	for file, snippets := range requiredSnippets {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}
		source := string(data)
		for _, snippet := range snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must contain %q", file, snippet)
			}
		}
	}
}

func TestAlertLifecycleCanonicalChangesRemainWritable(t *testing.T) {
	store := unifiedresources.NewMemoryStore()
	incidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	monitor := &Monitor{
		incidentStore: incidentStore,
	}
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store)))

	startedAt := time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)
	alert := &alerts.Alert{
		ID:         "alert-guardrail-1",
		Type:       "cpu",
		Level:      alerts.AlertLevelCritical,
		ResourceID: "vm-guardrail",
		Message:    "CPU threshold exceeded",
		StartTime:  startedAt,
	}

	monitor.handleAlertFired(alert)

	changes, err := store.GetRecentChanges("vm-guardrail", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 canonical alert change, got %d", len(changes))
	}
	if changes[0].Kind != unifiedresources.ChangeAlertFired {
		t.Fatalf("Kind = %q, want %q", changes[0].Kind, unifiedresources.ChangeAlertFired)
	}

	timeline := incidentStore.GetTimelineByAlertIdentifier(alert.ID)
	if timeline == nil {
		t.Fatal("expected incident timeline")
	}
	if len(timeline.Events) != 1 || timeline.Events[0].Type != memory.IncidentEventAlertFired {
		t.Fatalf("expected projected alert-fired event, got %#v", timeline.Events)
	}
}
