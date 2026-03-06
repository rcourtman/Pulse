package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	ur "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// mockReadState is a minimal ReadState implementation for tests that need
// ReadState-backed seed context functions.
type mockReadState struct {
	nodes       []*ur.NodeView
	vms         []*ur.VMView
	containers  []*ur.ContainerView
	hosts       []*ur.HostView
	dockerHosts []*ur.DockerHostView
	dockerCtrs  []*ur.DockerContainerView
	storage     []*ur.StoragePoolView
	pbs         []*ur.PBSInstanceView
	pmg         []*ur.PMGInstanceView
	k8sClusters []*ur.K8sClusterView
}

func (m *mockReadState) Nodes() []*ur.NodeView             { return m.nodes }
func (m *mockReadState) VMs() []*ur.VMView                 { return m.vms }
func (m *mockReadState) Containers() []*ur.ContainerView   { return m.containers }
func (m *mockReadState) Hosts() []*ur.HostView             { return m.hosts }
func (m *mockReadState) DockerHosts() []*ur.DockerHostView { return m.dockerHosts }
func (m *mockReadState) DockerContainers() []*ur.DockerContainerView {
	return m.dockerCtrs
}
func (m *mockReadState) StoragePools() []*ur.StoragePoolView     { return m.storage }
func (m *mockReadState) PBSInstances() []*ur.PBSInstanceView     { return m.pbs }
func (m *mockReadState) PMGInstances() []*ur.PMGInstanceView     { return m.pmg }
func (m *mockReadState) K8sClusters() []*ur.K8sClusterView       { return m.k8sClusters }
func (m *mockReadState) K8sNodes() []*ur.K8sNodeView             { return nil }
func (m *mockReadState) Pods() []*ur.PodView                     { return nil }
func (m *mockReadState) K8sDeployments() []*ur.K8sDeploymentView { return nil }
func (m *mockReadState) Workloads() []*ur.WorkloadView           { return nil }
func (m *mockReadState) Infrastructure() []*ur.InfrastructureView {
	return nil
}

type precomputeMetricsHistoryProvider struct {
	metrics map[string][]models.MetricPoint
	storage map[string][]models.MetricPoint
}

func (p *precomputeMetricsHistoryProvider) GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []models.MetricPoint {
	return p.metrics[nodeID+":"+metricType]
}

func (p *precomputeMetricsHistoryProvider) GetGuestMetrics(guestID string, metricType string, duration time.Duration) []models.MetricPoint {
	return p.metrics[guestID+":"+metricType]
}

func (p *precomputeMetricsHistoryProvider) GetAllGuestMetrics(guestID string, duration time.Duration) map[string][]models.MetricPoint {
	return nil
}

func (p *precomputeMetricsHistoryProvider) GetAllStorageMetrics(storageID string, duration time.Duration) map[string][]models.MetricPoint {
	if p.storage == nil {
		return nil
	}
	points := p.storage[storageID+":usage"]
	if len(points) == 0 {
		return nil
	}
	return map[string][]models.MetricPoint{"usage": points}
}

func TestSeedPrecomputeIntelligence_PopulatesSignals(t *testing.T) {
	now := time.Now()
	ps := NewPatrolService(nil, nil)

	bs := baseline.NewStore(baseline.StoreConfig{MinSamples: 1})
	baselinePoints := []baseline.MetricPoint{
		{Value: 10, Timestamp: now.Add(-5 * time.Hour)},
		{Value: 10, Timestamp: now.Add(-4 * time.Hour)},
		{Value: 10, Timestamp: now.Add(-3 * time.Hour)},
		{Value: 10, Timestamp: now.Add(-2 * time.Hour)},
		{Value: 10, Timestamp: now.Add(-1 * time.Hour)},
	}
	_ = bs.Learn("node-1", "node", "cpu", baselinePoints)
	_ = bs.Learn("node-1", "node", "memory", baselinePoints)
	_ = bs.Learn("vm-1", "vm", "memory", baselinePoints)
	_ = bs.Learn("vm-1", "vm", "disk", baselinePoints)
	_ = bs.Learn("ct-1", "container", "memory", baselinePoints)
	_ = bs.Learn("ct-1", "container", "disk", baselinePoints)
	_ = bs.Learn("storage-1", "storage", "usage", baselinePoints)
	ps.SetBaselineStore(bs)

	series := func(start float64) []models.MetricPoint {
		return []models.MetricPoint{
			{Value: start, Timestamp: now.Add(-5 * time.Hour)},
			{Value: start + 2, Timestamp: now.Add(-4 * time.Hour)},
			{Value: start + 4, Timestamp: now.Add(-3 * time.Hour)},
			{Value: start + 6, Timestamp: now.Add(-2 * time.Hour)},
			{Value: start + 8, Timestamp: now.Add(-1 * time.Hour)},
		}
	}
	mh := &precomputeMetricsHistoryProvider{
		metrics: map[string][]models.MetricPoint{
			"node-1:memory": series(70),
			"vm-1:memory":   series(60),
			"vm-1:disk":     series(50),
			"ct-1:memory":   series(55),
			"ct-1:disk":     series(45),
		},
		storage: map[string][]models.MetricPoint{
			"storage-1:usage": series(65),
		},
	}
	ps.SetMetricsHistoryProvider(mh)

	patternCfg := DefaultPatternConfig()
	patternCfg.MinOccurrences = 2
	patternCfg.PredictionLimit = 72 * time.Hour
	pd := NewPatternDetector(patternCfg)
	pd.RecordEvent(HistoricalEvent{ResourceID: "vm-1", EventType: EventHighCPU, Timestamp: now.Add(-36 * time.Hour)})
	pd.RecordEvent(HistoricalEvent{ResourceID: "vm-1", EventType: EventHighCPU, Timestamp: now.Add(-12 * time.Hour)})
	ps.SetPatternDetector(pd)

	cd := NewChangeDetector(ChangeDetectorConfig{MaxChanges: 10})
	cd.DetectChanges([]ResourceSnapshot{{ID: "vm-1", Name: "vm-1", Type: "vm", Status: "running", SnapshotTime: now.Add(-2 * time.Hour)}})
	cd.DetectChanges([]ResourceSnapshot{{ID: "vm-1", Name: "vm-1", Type: "vm", Status: "stopped", SnapshotTime: now.Add(-1 * time.Hour)}})
	ps.SetChangeDetector(cd)

	corrCfg := DefaultCorrelationConfig()
	corrCfg.MinOccurrences = 1
	corrCfg.CorrelationWindow = time.Hour
	corr := NewCorrelationDetector(corrCfg)
	for i := 0; i < 3; i++ {
		base := now.Add(time.Duration(-30+i*5) * time.Minute)
		corr.RecordEvent(CorrelationEvent{ResourceID: "node-1", ResourceName: "node-1", ResourceType: "node", EventType: CorrelationEventHighCPU, Timestamp: base})
		corr.RecordEvent(CorrelationEvent{ResourceID: "vm-1", ResourceName: "vm-1", ResourceType: "vm", EventType: CorrelationEventRestart, Timestamp: base.Add(2 * time.Minute)})
	}
	ps.SetCorrelationDetector(corr)

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "node-1", CPU: 80, Memory: models.Memory{Usage: 80}},
		},
		VMs: []models.VM{
			{ID: "vm-1", Name: "vm-1", Status: "running", CPU: 20, Memory: models.Memory{Usage: 70}, Disk: models.Disk{Usage: 60}},
		},
		Containers: []models.Container{
			{ID: "ct-1", Name: "ct-1", Status: "running", CPU: 10, Memory: models.Memory{Usage: 65}, Disk: models.Disk{Usage: 55}},
		},
		Storage: []models.Storage{
			{ID: "storage-1", Name: "local", Usage: 85},
		},
		ActiveAlerts: []models.Alert{{ID: "alert-1"}},
	}

	scoped := map[string]bool{"node-1": true, "vm-1": true, "ct-1": true, "storage-1": true}
	intel := ps.seedPrecomputeIntelligence(state, scoped, now)

	if !intel.hasBaselineStore {
		t.Fatalf("expected baseline store flag to be true")
	}
	if len(intel.anomalies) == 0 {
		t.Fatalf("expected anomalies to be populated")
	}
	nameSet := false
	for _, a := range intel.anomalies {
		if a.ResourceName != "" {
			nameSet = true
			break
		}
	}
	if !nameSet {
		t.Fatalf("expected anomaly resource names to be set")
	}
	if len(intel.forecasts) == 0 {
		t.Fatalf("expected capacity forecasts")
	}
	if len(intel.predictions) == 0 {
		t.Fatalf("expected failure predictions")
	}
	if len(intel.recentChanges) == 0 {
		t.Fatalf("expected recent changes")
	}
	if len(intel.correlations) == 0 {
		t.Fatalf("expected correlations")
	}
	if intel.isQuiet {
		t.Fatalf("expected infrastructure to be non-quiet")
	}
}

func TestSeedBackupAnalysis_StaleAndRecent(t *testing.T) {
	now := time.Now()
	ps := NewPatrolService(nil, nil)

	// Wire ReadState — seedBackupAnalysis uses ReadState as sole path.
	vm1 := newTestVMView("qemu/101", "vm-1", 101, "pve1", "", ur.StatusOnline, false, nil)
	// Set LastBackup on vm-1's underlying resource via view construction.
	vm1Res := &ur.Resource{
		ID: "vm-1", Name: "vm-1", Type: ur.ResourceTypeVM, Status: ur.StatusOnline,
		Proxmox: &ur.ProxmoxData{VMID: 101, LastBackup: now.Add(-24 * time.Hour)},
	}
	vm1v := ur.NewVMView(vm1Res)
	vm1 = &vm1v

	ps.SetReadState(&mockReadState{
		vms: []*ur.VMView{
			vm1,
			newTestVMView("qemu/102", "vm-2", 102, "pve1", "", ur.StatusOnline, false, nil),
		},
		containers: []*ur.ContainerView{
			newTestContainerView("lxc/201", "ct-1", 201, "pve1", "", ur.StatusOnline, false, nil),
		},
	})

	state := models.StateSnapshot{
		PVEBackups: models.PVEBackups{
			BackupTasks:    []models.BackupTask{{VMID: 102, Status: "OK", EndTime: now.Add(-72 * time.Hour)}},
			StorageBackups: []models.StorageBackup{{VMID: 101, Time: now.Add(-36 * time.Hour)}},
		},
		PBSBackups: []models.PBSBackup{{VMID: "102", BackupTime: now.Add(-72 * time.Hour)}},
	}

	output := ps.seedBackupAnalysis(state, now)
	if output == "" {
		t.Fatalf("expected backup analysis output")
	}
	if !strings.Contains(output, "Guests with no backup in >48h") {
		t.Fatalf("expected stale backup section, got: %s", output)
	}
	if !strings.Contains(output, "ct-1 (never)") {
		t.Fatalf("expected never-backed guest, got: %s", output)
	}
	if !strings.Contains(output, "vm-2 (last:") {
		t.Fatalf("expected stale vm entry, got: %s", output)
	}
	if !strings.Contains(output, "Guests with recent backups: 1/3") {
		t.Fatalf("expected recent backup count, got: %s", output)
	}
}

func TestSeedPrecomputeIntelligenceState_UsesRuntimeReadState(t *testing.T) {
	now := time.Now()
	ps := NewPatrolService(nil, nil)

	bs := baseline.NewStore(baseline.StoreConfig{MinSamples: 1})
	baselinePoints := []baseline.MetricPoint{
		{Value: 10, Timestamp: now.Add(-5 * time.Hour)},
		{Value: 10, Timestamp: now.Add(-4 * time.Hour)},
		{Value: 10, Timestamp: now.Add(-3 * time.Hour)},
		{Value: 10, Timestamp: now.Add(-2 * time.Hour)},
		{Value: 10, Timestamp: now.Add(-1 * time.Hour)},
	}
	_ = bs.Learn("node-1", "node", "cpu", baselinePoints)
	_ = bs.Learn("node-1", "node", "memory", baselinePoints)
	_ = bs.Learn("vm-1", "vm", "memory", baselinePoints)
	_ = bs.Learn("vm-1", "vm", "disk", baselinePoints)
	_ = bs.Learn("ct-1", "container", "memory", baselinePoints)
	_ = bs.Learn("ct-1", "container", "disk", baselinePoints)
	_ = bs.Learn("storage-1", "storage", "usage", baselinePoints)
	ps.SetBaselineStore(bs)

	series := func(start float64) []models.MetricPoint {
		return []models.MetricPoint{
			{Value: start, Timestamp: now.Add(-5 * time.Hour)},
			{Value: start + 2, Timestamp: now.Add(-4 * time.Hour)},
			{Value: start + 4, Timestamp: now.Add(-3 * time.Hour)},
			{Value: start + 6, Timestamp: now.Add(-2 * time.Hour)},
			{Value: start + 8, Timestamp: now.Add(-1 * time.Hour)},
		}
	}
	ps.SetMetricsHistoryProvider(&precomputeMetricsHistoryProvider{
		metrics: map[string][]models.MetricPoint{
			"node-1:memory": series(70),
			"vm-1:memory":   series(60),
			"vm-1:disk":     series(50),
			"ct-1:memory":   series(55),
			"ct-1:disk":     series(45),
		},
		storage: map[string][]models.MetricPoint{
			"storage-1:usage": series(65),
		},
	})

	runtimeState := newPatrolRuntimeState(models.StateSnapshot{ActiveAlerts: []models.Alert{{ID: "alert-1"}}})
	nodeView := ur.NewNodeView(&ur.Resource{
		ID:     "node-1",
		Name:   "node-1",
		Type:   ur.ResourceTypeAgent,
		Status: ur.StatusOnline,
		Metrics: &ur.ResourceMetrics{
			CPU:    &ur.MetricValue{Percent: 80},
			Memory: &ur.MetricValue{Percent: 80},
		},
	})
	vmView := ur.NewVMView(&ur.Resource{
		ID:     "vm-1",
		Name:   "vm-1",
		Type:   ur.ResourceTypeVM,
		Status: "running",
		Metrics: &ur.ResourceMetrics{
			CPU:    &ur.MetricValue{Percent: 20},
			Memory: &ur.MetricValue{Percent: 70},
			Disk:   &ur.MetricValue{Percent: 60},
		},
	})
	ctView := ur.NewContainerView(&ur.Resource{
		ID:     "ct-1",
		Name:   "ct-1",
		Type:   ur.ResourceTypeSystemContainer,
		Status: "running",
		Metrics: &ur.ResourceMetrics{
			CPU:    &ur.MetricValue{Percent: 10},
			Memory: &ur.MetricValue{Percent: 65},
			Disk:   &ur.MetricValue{Percent: 55},
		},
	})
	storageView := ur.NewStoragePoolView(&ur.Resource{
		ID:     "storage-1",
		Name:   "local",
		Type:   ur.ResourceTypeStorage,
		Status: ur.StatusOnline,
		Metrics: &ur.ResourceMetrics{
			Disk: &ur.MetricValue{Percent: 85},
		},
	})
	runtimeState.readState = &mockReadState{
		nodes:      []*ur.NodeView{&nodeView},
		vms:        []*ur.VMView{&vmView},
		containers: []*ur.ContainerView{&ctView},
		storage:    []*ur.StoragePoolView{&storageView},
	}

	ps.SetReadState(nil)

	scoped := map[string]bool{"node-1": true, "vm-1": true, "ct-1": true, "storage-1": true}
	intel := ps.seedPrecomputeIntelligenceState(runtimeState, scoped, now)
	if !intel.hasBaselineStore {
		t.Fatalf("expected baseline store flag to be true")
	}
	if len(intel.anomalies) == 0 {
		t.Fatalf("expected anomalies from runtime readState")
	}
	if len(intel.forecasts) == 0 {
		t.Fatalf("expected forecasts from runtime readState")
	}
}

func TestSeedFindingsAndContext_ResolvesMissingAndAddsNotes(t *testing.T) {
	now := time.Now()
	ps := NewPatrolService(nil, nil)

	missing := &Finding{
		ID:           "find-missing",
		Severity:     FindingSeverityInfo,
		Category:     FindingCategoryPerformance,
		ResourceID:   "vm-missing",
		ResourceName: "vm-missing",
		Title:        "Missing VM",
		Description:  "no longer exists",
		DetectedAt:   now.Add(-2 * time.Hour),
		LastSeenAt:   now.Add(-2 * time.Hour),
	}
	active := &Finding{
		ID:           "find-active",
		Severity:     FindingSeverityInfo,
		Category:     FindingCategoryPerformance,
		ResourceID:   "node-1",
		ResourceName: "node-1",
		Title:        "High CPU",
		Description:  "cpu high",
		UserNote:     "keep an eye",
		DetectedAt:   now.Add(-1 * time.Hour),
		LastSeenAt:   now.Add(-1 * time.Hour),
	}
	dismissed := &Finding{
		ID:           "find-dismissed",
		Severity:     FindingSeverityInfo,
		Category:     FindingCategoryPerformance,
		ResourceID:   "node-1",
		ResourceName: "node-1",
		Title:        "Noisy alerts",
		Description:  "expected",
		DetectedAt:   now.Add(-30 * time.Minute),
		LastSeenAt:   now.Add(-30 * time.Minute),
	}
	dismissedOutOfScope := &Finding{
		ID:           "find-dismissed-out-of-scope",
		Severity:     FindingSeverityInfo,
		Category:     FindingCategoryPerformance,
		ResourceID:   "node-2",
		ResourceName: "node-2",
		Title:        "Ignore node-2",
		Description:  "out of scope",
		DetectedAt:   now.Add(-20 * time.Minute),
		LastSeenAt:   now.Add(-20 * time.Minute),
	}

	ps.findings.Add(missing)
	ps.findings.Add(active)
	ps.findings.Add(dismissed)
	ps.findings.Add(dismissedOutOfScope)
	ps.findings.Dismiss(dismissed.ID, "expected_behavior", "known workload")
	ps.findings.Dismiss(dismissedOutOfScope.ID, "expected_behavior", "different resource")

	resolvedID := ""
	ps.unifiedFindingResolver = func(findingID string) {
		resolvedID = findingID
	}

	knowledgeStore, err := knowledge.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create knowledge store: %v", err)
	}
	if err := knowledgeStore.SaveNote("node-1", "node-1", "node", "config", "Pinned", "keep settings"); err != nil {
		t.Fatalf("failed to save knowledge note: %v", err)
	}
	if err := knowledgeStore.SaveNote("node-2", "node-2", "node", "config", "Ignore", "out of scope"); err != nil {
		t.Fatalf("failed to save out-of-scope knowledge note: %v", err)
	}
	ps.knowledgeStore = knowledgeStore

	// Set up readState so seedFindingsAndContext can build knownResources
	// and auto-resolve findings for resources that no longer exist.
	nodeView := ur.NewNodeView(&ur.Resource{ID: "node-1", Name: "node-1"})
	ps.readState = &mockReadState{nodes: []*ur.NodeView{&nodeView}}

	state := models.StateSnapshot{Nodes: []models.Node{{ID: "node-1", Name: "node-1"}}}
	output, seeded := ps.seedFindingsAndContext(&PatrolScope{ResourceIDs: []string{"node-1"}}, state)

	if resolvedID != missing.ID {
		t.Fatalf("expected unified resolver to be called for missing finding")
	}
	if missing.ResolvedAt == nil {
		t.Fatalf("expected missing finding to be resolved")
	}
	if len(seeded) != 1 || seeded[0] != active.ID {
		t.Fatalf("expected active finding to be seeded")
	}
	if !strings.Contains(output, "Active Findings to Re-check") {
		t.Fatalf("expected active findings section, got: %s", output)
	}
	if !strings.Contains(output, "User note: \"keep an eye\"") {
		t.Fatalf("expected user note in output, got: %s", output)
	}
	if !strings.Contains(output, "User Feedback on Previous Findings") {
		t.Fatalf("expected dismissed findings context, got: %s", output)
	}
	if strings.Contains(output, dismissedOutOfScope.Title) {
		t.Fatalf("expected out-of-scope dismissed feedback to be omitted, got: %s", output)
	}
	if strings.Contains(output, "out of scope") {
		t.Fatalf("expected out-of-scope knowledge note to be omitted, got: %s", output)
	}
	if !strings.Contains(output, "# User Notes") || !strings.Contains(output, "Saved Knowledge") {
		t.Fatalf("expected knowledge context, got: %s", output)
	}
}

func TestSeedFindingsAndContext_ScopedPatrolSkipsOutOfScopeFindingsWithoutResolving(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	inScope := &Finding{
		ID:           "finding-in-scope",
		ResourceID:   "node-1",
		ResourceName: "node-1",
		Title:        "Scoped finding",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryGeneral,
		DetectedAt:   time.Now().Add(-time.Hour),
		LastSeenAt:   time.Now().Add(-time.Hour),
	}
	outOfScope := &Finding{
		ID:           "finding-out-of-scope",
		ResourceID:   "node-2",
		ResourceName: "node-2",
		Title:        "Out of scope finding",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryGeneral,
		DetectedAt:   time.Now().Add(-time.Hour),
		LastSeenAt:   time.Now().Add(-time.Hour),
	}
	ps.findings.Add(inScope)
	ps.findings.Add(outOfScope)

	node1 := ur.NewNodeView(&ur.Resource{ID: "node-1", Name: "node-1"})
	node2 := ur.NewNodeView(&ur.Resource{ID: "node-2", Name: "node-2"})
	ps.SetReadState(&mockReadState{nodes: []*ur.NodeView{&node1, &node2}})

	scopedRuntime := newPatrolRuntimeState(models.StateSnapshot{
		Nodes: []models.Node{{ID: "node-1", Name: "node-1"}},
	})
	scopedRuntime.readState = &mockReadState{nodes: []*ur.NodeView{&node1}}

	output, seeded := ps.seedFindingsAndContextState(&PatrolScope{ResourceIDs: []string{"node-1"}}, scopedRuntime)

	if outOfScope.ResolvedAt != nil {
		t.Fatalf("expected out-of-scope finding to remain active, got resolved at %v", outOfScope.ResolvedAt)
	}
	if len(seeded) != 1 || seeded[0] != inScope.ID {
		t.Fatalf("expected only in-scope finding to be seeded, got %v", seeded)
	}
	if !strings.Contains(output, inScope.Title) {
		t.Fatalf("expected in-scope finding in output, got: %s", output)
	}
	if strings.Contains(output, outOfScope.Title) {
		t.Fatalf("expected out-of-scope finding to be omitted, got: %s", output)
	}
}

func TestSeedFindingsAndContext_ScopedPatrolWithoutRuntimeResourcesOmitsGlobalKnowledge(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	knowledgeStore, err := knowledge.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create knowledge store: %v", err)
	}
	if err := knowledgeStore.SaveNote("node-1", "node-1", "node", "config", "Pinned", "keep settings"); err != nil {
		t.Fatalf("failed to save knowledge note: %v", err)
	}
	ps.knowledgeStore = knowledgeStore

	output, _ := ps.seedFindingsAndContextState(&PatrolScope{ResourceTypes: []string{"node"}}, newPatrolRuntimeState(models.StateSnapshot{}))
	if strings.Contains(output, "# User Notes") || strings.Contains(output, "keep settings") {
		t.Fatalf("expected scoped patrol without runtime resources to omit global knowledge, got: %s", output)
	}
}

func TestSeedFindingsAndContext_ScopedPatrolExplicitIDsStillUseKnowledgeFallback(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	knowledgeStore, err := knowledge.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create knowledge store: %v", err)
	}
	if err := knowledgeStore.SaveNote("node-1", "node-1", "node", "config", "Pinned", "keep settings"); err != nil {
		t.Fatalf("failed to save knowledge note: %v", err)
	}
	ps.knowledgeStore = knowledgeStore

	output, _ := ps.seedFindingsAndContextState(&PatrolScope{ResourceIDs: []string{"node-1"}}, newPatrolRuntimeState(models.StateSnapshot{}))
	if !strings.Contains(output, "# User Notes") || !strings.Contains(output, "keep settings") {
		t.Fatalf("expected explicit scoped patrol to retain knowledge fallback, got: %s", output)
	}
}

// newTestVMView creates a VMView with Proxmox fields needed for gatherGuestIntelligence tests.
// sourceID is the legacy guest ID (e.g. "qemu/100") stored in Proxmox.SourceID.
// status should be a normalized ResourceStatus (e.g. ur.StatusOnline, ur.StatusOffline).
func newTestVMView(sourceID, name string, vmid int, node, instance string, status ur.ResourceStatus, template bool, ips []string) *ur.VMView {
	r := &ur.Resource{
		ID:     "reg-" + sourceID, // unified registry ID (not used by gatherGuestIntelligence)
		Name:   name,
		Type:   ur.ResourceTypeVM,
		Status: status,
		Proxmox: &ur.ProxmoxData{
			SourceID: sourceID,
			VMID:     vmid,
			NodeName: node,
			Instance: instance,
			Template: template,
		},
		Identity: ur.ResourceIdentity{
			IPAddresses: ips,
		},
	}
	v := ur.NewVMView(r)
	return &v
}

// newTestContainerView creates a ContainerView with Proxmox fields needed for gatherGuestIntelligence tests.
// sourceID is the legacy guest ID (e.g. "lxc/101") stored in Proxmox.SourceID.
// status should be a normalized ResourceStatus (e.g. ur.StatusOnline, ur.StatusOffline).
func newTestContainerView(sourceID, name string, vmid int, node, instance string, status ur.ResourceStatus, template bool, ips []string) *ur.ContainerView {
	r := &ur.Resource{
		ID:     "reg-" + sourceID,
		Name:   name,
		Type:   ur.ResourceTypeSystemContainer,
		Status: status,
		Proxmox: &ur.ProxmoxData{
			SourceID: sourceID,
			VMID:     vmid,
			NodeName: node,
			Instance: instance,
			Template: template,
		},
		Identity: ur.ResourceIdentity{
			IPAddresses: ips,
		},
	}
	v := ur.NewContainerView(r)
	return &v
}

func TestGatherGuestIntelligence_ReadStatePath(t *testing.T) {
	// Set up discovery store with service info for one VM
	store := setupTestDiscoveryStore(t, []*servicediscovery.ResourceDiscovery{
		{
			ID:           "vm:pve1:100",
			ResourceType: servicediscovery.ResourceTypeVM,
			TargetID:     "pve1",
			ResourceID:   "100",
			ServiceName:  "PostgreSQL 15",
			ServiceType:  "postgres",
		},
	})

	ps := NewPatrolService(nil, nil)
	ps.SetDiscoveryStore(store)

	// Wire ReadState instead of using state snapshot.
	// Status uses normalized StatusOnline/StatusOffline (registry normalizes "running" → "online").
	rs := &mockReadState{
		vms: []*ur.VMView{
			newTestVMView("qemu/100", "db-server", 100, "pve1", "", ur.StatusOnline, false, nil),
			newTestVMView("qemu/200", "unknown-vm", 200, "pve1", "", ur.StatusOnline, false, nil),
			newTestVMView("qemu/9000", "template-vm", 9000, "pve1", "", ur.StatusOffline, true, nil),
		},
		containers: []*ur.ContainerView{
			newTestContainerView("lxc/101", "web-proxy", 101, "pve1", "", ur.StatusOnline, false, nil),
		},
	}
	ps.SetReadState(rs)

	intel := ps.gatherGuestIntelligence(context.Background())

	// Expect 3 entries: 2 VMs (template skipped) + 1 container
	if len(intel) != 3 {
		t.Fatalf("expected 3 entries from ReadState path, got %d", len(intel))
	}
	if gi := intel["qemu/100"]; gi == nil || gi.ServiceName != "PostgreSQL 15" {
		t.Fatalf("expected db-server with discovery, got: %+v", gi)
	}
	if gi := intel["qemu/200"]; gi == nil || gi.Name != "unknown-vm" {
		t.Fatalf("expected unknown-vm entry, got: %+v", gi)
	}
	if gi := intel["lxc/101"]; gi == nil || gi.GuestType != "system-container" {
		t.Fatalf("expected web-proxy container entry, got: %+v", gi)
	}
	// Template should be skipped
	if _, ok := intel["qemu/9000"]; ok {
		t.Fatal("template VM should be skipped")
	}
}

func TestGatherGuestIntelligence_ReadStateReachability(t *testing.T) {
	prober := &mockGuestProber{
		agents: map[string]string{"pve1": "agent-1"},
		results: map[string]map[string]PingResult{
			"agent-1": {
				"10.0.0.1": {Reachable: true},
				"10.0.0.2": {Reachable: false},
			},
		},
	}

	ps := NewPatrolService(nil, nil)
	ps.SetGuestProber(prober)

	rs := &mockReadState{
		vms: []*ur.VMView{
			newTestVMView("qemu/100", "vm-up", 100, "pve1", "", ur.StatusOnline, false, []string{"10.0.0.1"}),
			newTestVMView("qemu/101", "vm-down", 101, "pve1", "", ur.StatusOnline, false, []string{"10.0.0.2"}),
		},
	}
	ps.SetReadState(rs)

	intel := ps.gatherGuestIntelligence(context.Background())

	if len(intel) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(intel))
	}
	if gi := intel["qemu/100"]; gi == nil || gi.Reachable == nil || !*gi.Reachable {
		t.Fatal("expected vm-up to be reachable")
	}
	if gi := intel["qemu/101"]; gi == nil || gi.Reachable == nil || *gi.Reachable {
		t.Fatal("expected vm-down to be unreachable")
	}
}

func TestGatherGuestIntelligence_ReadStateInstanceFallback(t *testing.T) {
	// Test that Instance-based discovery lookup works via ReadState
	store := setupTestDiscoveryStore(t, []*servicediscovery.ResourceDiscovery{
		{
			ID:           "vm:my-instance:100",
			ResourceType: servicediscovery.ResourceTypeVM,
			TargetID:     "my-instance",
			ResourceID:   "100",
			ServiceName:  "Redis",
			ServiceType:  "redis",
		},
	})

	ps := NewPatrolService(nil, nil)
	ps.SetDiscoveryStore(store)

	rs := &mockReadState{
		vms: []*ur.VMView{
			newTestVMView("qemu/100", "cache", 100, "pve1", "my-instance", ur.StatusOnline, false, nil),
		},
	}
	ps.SetReadState(rs)

	intel := ps.gatherGuestIntelligence(context.Background())

	if gi := intel["qemu/100"]; gi == nil || gi.ServiceName != "Redis" {
		t.Fatalf("expected Redis service from instance lookup, got: %+v", gi)
	}
}
