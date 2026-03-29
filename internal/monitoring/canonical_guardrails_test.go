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
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

func TestUnifiedAppContainerMetricsUseCanonicalGuestHistoryPath(t *testing.T) {
	data, err := os.ReadFile("monitor.go")
	if err != nil {
		t.Fatalf("failed to read monitor.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"m.syncUnifiedAppContainerMetrics(store)",
		`if target == nil || target.ResourceType != "app-container" || strings.TrimSpace(target.ResourceID) == "" {`,
		`metricKey := fmt.Sprintf("docker:%s", targetID)`,
		`m.metricsStore.Write("dockerContainer", targetID, "cpu", value, now)`,
		`m.metricsStore.Write("dockerContainer", targetID, "diskwrite", metric.Value, now)`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor.go must contain %q", snippet)
		}
	}
}

func TestUnifiedAgentMetricsUseCanonicalHostHistoryPath(t *testing.T) {
	data, err := os.ReadFile("monitor.go")
	if err != nil {
		t.Fatalf("failed to read monitor.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"m.syncUnifiedAgentMetrics(store)",
		`if target == nil || target.ResourceType != "agent" || strings.TrimSpace(target.ResourceID) == "" {`,
		`metricKey := fmt.Sprintf("agent:%s", targetID)`,
		`m.metricsStore.Write("agent", targetID, "cpu", value, now)`,
		`m.metricsStore.Write("agent", targetID, "diskwrite", metric.Value, now)`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor.go must contain %q", snippet)
		}
	}
}

func TestUnifiedPhysicalDiskMetricsUseCanonicalDiskHistoryPath(t *testing.T) {
	data, err := os.ReadFile("monitor.go")
	if err != nil {
		t.Fatalf("failed to read monitor.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"m.syncUnifiedPhysicalDiskMetrics(store)",
		`if target == nil || target.ResourceType != "disk" || strings.TrimSpace(target.ResourceID) == "" {`,
		`if source == unifiedresources.SourceProxmox || source == unifiedresources.SourceAgent {`,
		`m.writeSMARTMetrics(disk, now)`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor.go must contain %q", snippet)
		}
	}
}

func TestTrueNASSystemTelemetryUsesCanonicalHostTemperatureModel(t *testing.T) {
	clientData, err := os.ReadFile(filepath.Join("..", "truenas", "client.go"))
	if err != nil {
		t.Fatalf("failed to read ../truenas/client.go: %v", err)
	}
	clientSource := string(clientData)
	clientSnippets := []string{
		`"reporting.get_data"`,
		`telemetry.TemperatureCelsius = cloneTemperatureMap(temperatures)`,
		`return parseSystemTemperatures(response), nil`,
	}
	for _, snippet := range clientSnippets {
		if !strings.Contains(clientSource, snippet) {
			t.Fatalf("../truenas/client.go must contain %q", snippet)
		}
	}

	providerData, err := os.ReadFile(filepath.Join("..", "truenas", "provider.go"))
	if err != nil {
		t.Fatalf("failed to read ../truenas/provider.go: %v", err)
	}
	providerSource := string(providerData)
	providerSnippets := []string{
		`if temperature := maxTrueNASSystemTemperature(system); temperature != nil {`,
		`agent.Temperature = temperature`,
		`if sensors := sensorMetaFromTrueNASSystem(system); sensors != nil {`,
		`agent.Sensors = sensors`,
	}
	for _, snippet := range providerSnippets {
		if !strings.Contains(providerSource, snippet) {
			t.Fatalf("../truenas/provider.go must contain %q", snippet)
		}
	}
}

func TestHostAgentRemovalGuardUsesResolvedIdentifier(t *testing.T) {
	data, err := os.ReadFile("monitor_agents.go")
	if err != nil {
		t.Fatalf("failed to read monitor_agents.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"removedAt, wasRemoved := m.lookupRemovedHostAgent(identifier, hostname)",
		`Str("hostID", identifier)`,
		`fmt.Errorf("host agent %q had monitoring stopped at %v and cannot report again.`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor_agents.go must contain %q", snippet)
		}
	}
	if strings.Contains(source, "m.lookupRemovedHostAgent(baseIdentifier, hostname)") {
		t.Fatal("monitor_agents.go must not check removed host-agent state against pre-resolution baseIdentifier")
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

func TestTelemetrySnapshotAggregationUsesProvisionedTenantSet(t *testing.T) {
	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	for _, orgID := range []string{"default", "org-a"} {
		if _, err := persistence.GetPersistence(orgID); err != nil {
			t.Fatalf("GetPersistence(%s): %v", orgID, err)
		}
	}

	rm, err := NewReloadableMonitor(&config.Config{DataPath: baseDir}, persistence, nil)
	if err != nil {
		t.Fatalf("NewReloadableMonitor: %v", err)
	}

	mtm := rm.GetMultiTenantMonitor()
	if mtm == nil {
		t.Fatal("expected multi-tenant monitor")
	}
	mtm.monitors["default"] = testTelemetryMonitor(
		nil,
		[]models.VM{{ID: "vm-default", VMID: 101, Name: "vm-default", Instance: "pve-default"}},
		nil,
		nil,
		nil,
		nil,
		nil,
		1,
	)
	mtm.monitors["org-a"] = testTelemetryMonitor(
		nil,
		[]models.VM{{ID: "vm-a", VMID: 201, Name: "vm-a", Instance: "pve-a"}},
		nil,
		nil,
		nil,
		nil,
		nil,
		2,
	)

	counts := rm.AggregateInstallSnapshotCounts()
	if counts.VMs != 2 {
		t.Fatalf("VMs = %d, want 2 across provisioned tenants", counts.VMs)
	}
	if counts.ActiveAlerts != 3 {
		t.Fatalf("ActiveAlerts = %d, want 3 across provisioned tenants", counts.ActiveAlerts)
	}
}
