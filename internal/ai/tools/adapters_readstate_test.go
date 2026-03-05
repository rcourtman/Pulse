package tools

import (
	"fmt"
	"testing"
	"time"

	ur "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// fakeReadState is a minimal ReadState implementation for testing the
// ReadState-preferred path in adapters that support SRC migration.
type fakeReadState struct {
	vms              []*ur.VMView
	containers       []*ur.ContainerView
	nodes            []*ur.NodeView
	hosts            []*ur.HostView
	dockerHosts      []*ur.DockerHostView
	dockerContainers []*ur.DockerContainerView
}

func (f *fakeReadState) VMs() []*ur.VMView                           { return f.vms }
func (f *fakeReadState) Containers() []*ur.ContainerView             { return f.containers }
func (f *fakeReadState) Nodes() []*ur.NodeView                       { return f.nodes }
func (f *fakeReadState) Hosts() []*ur.HostView                       { return f.hosts }
func (f *fakeReadState) DockerHosts() []*ur.DockerHostView           { return f.dockerHosts }
func (f *fakeReadState) DockerContainers() []*ur.DockerContainerView { return f.dockerContainers }
func (f *fakeReadState) StoragePools() []*ur.StoragePoolView         { return nil }
func (f *fakeReadState) PBSInstances() []*ur.PBSInstanceView         { return nil }
func (f *fakeReadState) PMGInstances() []*ur.PMGInstanceView         { return nil }
func (f *fakeReadState) K8sClusters() []*ur.K8sClusterView           { return nil }
func (f *fakeReadState) K8sNodes() []*ur.K8sNodeView                 { return nil }
func (f *fakeReadState) Pods() []*ur.PodView                         { return nil }
func (f *fakeReadState) K8sDeployments() []*ur.K8sDeploymentView     { return nil }
func (f *fakeReadState) Workloads() []*ur.WorkloadView               { return nil }
func (f *fakeReadState) Infrastructure() []*ur.InfrastructureView    { return nil }

func newVMView(id string, name string, vmid int) *ur.VMView {
	r := &ur.Resource{
		ID:   id,
		Name: name,
		Type: ur.ResourceTypeVM,
		Proxmox: &ur.ProxmoxData{
			VMID: vmid,
		},
	}
	v := ur.NewVMView(r)
	return &v
}

func newContainerView(id string, name string, vmid int) *ur.ContainerView {
	r := &ur.Resource{
		ID:   id,
		Name: name,
		Type: ur.ResourceTypeSystemContainer,
		Proxmox: &ur.ProxmoxData{
			VMID: vmid,
		},
	}
	v := ur.NewContainerView(r)
	return &v
}

func newNodeView(id string, name string, sourceID string) *ur.NodeView {
	r := &ur.Resource{
		ID:   id,
		Name: name,
		Type: ur.ResourceTypeAgent,
		Proxmox: &ur.ProxmoxData{
			SourceID: sourceID,
		},
	}
	v := ur.NewNodeView(r)
	return &v
}

func newHostView(id string, name string, agentID string, hostname string, sensors *ur.HostSensorMeta, raid []ur.HostRAIDMeta, ceph *ur.HostCephMeta) *ur.HostView {
	r := &ur.Resource{
		ID:   id,
		Name: name,
		Type: ur.ResourceTypeAgent,
		Agent: &ur.AgentData{
			AgentID:  agentID,
			Hostname: hostname,
			Sensors:  sensors,
			RAID:     raid,
			Ceph:     ceph,
		},
	}
	v := ur.NewHostView(r)
	return &v
}

func newDockerHostView(id string, sourceID string, name string, hostname string) *ur.DockerHostView {
	r := &ur.Resource{
		ID:   id,
		Name: name,
		Type: ur.ResourceTypeAgent,
		Docker: &ur.DockerData{
			HostSourceID: sourceID,
			Hostname:     hostname,
		},
	}
	v := ur.NewDockerHostView(r)
	return &v
}

func newDockerContainerView(id string, parentID string, hostSourceID string, name string, containerID string, image string, updateStatus *ur.DockerUpdateStatusMeta) *ur.DockerContainerView {
	r := &ur.Resource{
		ID:       id,
		Name:     name,
		Type:     ur.ResourceTypeAppContainer,
		ParentID: &parentID,
		Docker: &ur.DockerData{
			HostSourceID: hostSourceID,
			ContainerID:  containerID,
			Image:        image,
			UpdateStatus: updateStatus,
		},
	}
	v := ur.NewDockerContainerView(r)
	return &v
}

func TestMetricsSummaryWithReadState(t *testing.T) {
	now := time.Now()
	source := &fakeMetricsSource{
		guest: map[string]map[string][]RawMetricPoint{
			"100": {
				"cpu":    {{Value: 10, Timestamp: now}, {Value: 20, Timestamp: now.Add(time.Minute)}},
				"memory": {{Value: 30, Timestamp: now}},
			},
		},
		node: map[string]map[string][]RawMetricPoint{
			"node1": {
				"cpu":    {{Value: 5, Timestamp: now}},
				"memory": {{Value: 15, Timestamp: now}},
			},
		},
	}

	rs := &fakeReadState{
		vms:        []*ur.VMView{newVMView("vm-100", "vm1", 100)},
		containers: []*ur.ContainerView{newContainerView("ct-200", "ct1", 200)},
		nodes:      []*ur.NodeView{newNodeView("reg-node-hash", "node-1", "node1")},
	}

	adapter := NewMetricsHistoryMCPAdapter(source, rs)
	if adapter == nil {
		t.Fatal("expected non-nil adapter when readState provided")
	}

	summary, err := adapter.GetAllMetricsSummary(time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summary) != 2 {
		t.Fatalf("expected 2 summaries (vm + node), got %d", len(summary))
	}
	if summary["100"].ResourceName != "vm1" {
		t.Fatalf("expected vm name 'vm1', got %q", summary["100"].ResourceName)
	}
	if summary["node1"].ResourceName != "node-1" {
		t.Fatalf("expected node name 'node-1', got %q", summary["node1"].ResourceName)
	}
}

func TestMetricsSummaryReadStateVMsOnly(t *testing.T) {
	// When readState has no nodes, only VMs/Containers from ReadState are returned.
	now := time.Now()
	rs := &fakeReadState{
		vms: []*ur.VMView{newVMView("vm-100", "vm1", 100)},
	}
	source := &fakeMetricsSource{
		guest: map[string]map[string][]RawMetricPoint{
			"100": {"cpu": {{Value: 10, Timestamp: now}}},
		},
	}
	adapter := NewMetricsHistoryMCPAdapter(source, rs)
	if adapter == nil {
		t.Fatal("expected non-nil adapter with readState")
	}
	summary, err := adapter.GetAllMetricsSummary(time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summary) != 1 {
		t.Fatalf("expected 1 summary (vm only, nodes skipped), got %d", len(summary))
	}
	if summary["100"].ResourceName != "vm1" {
		t.Fatalf("expected vm name 'vm1', got %q", summary["100"].ResourceName)
	}
}

func TestMetricsSummaryNilMetricsSource(t *testing.T) {
	// When metricsSource is nil, constructor returns nil
	rs := &fakeReadState{}
	if adapter := NewMetricsHistoryMCPAdapter(nil, rs); adapter != nil {
		t.Fatal("expected nil adapter for nil metricsSource")
	}
}

func TestMetricsSummaryNilReadState(t *testing.T) {
	// When readState is nil, constructor returns nil
	source := &fakeMetricsSource{}
	if adapter := NewMetricsHistoryMCPAdapter(source, nil); adapter != nil {
		t.Fatal("expected nil adapter when readState is nil")
	}
}

func TestPatternAdapterWithReadState(t *testing.T) {
	rs := &fakeReadState{
		vms:        []*ur.VMView{newVMView("vm-100", "vm1", 100)},
		containers: []*ur.ContainerView{newContainerView("ct-200", "ct1", 200)},
		nodes:      []*ur.NodeView{newNodeView("reg-node-hash", "node-1", "node1")},
	}

	source := &fakePatternSource{
		patterns: []PatternData{
			{ResourceID: "100", PatternType: "cpu", Description: "spike"},
			{ResourceID: "node1", PatternType: "disk", Description: "trend"},
		},
		predictions: []PredictionData{
			{ResourceID: "200", IssueType: "memory", Recommendation: "scale"},
		},
	}

	adapter := NewPatternMCPAdapter(source, rs)
	patterns := adapter.GetPatterns()
	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(patterns))
	}
	if patterns[0].ResourceName != "vm1" {
		t.Fatalf("expected pattern[0] name 'vm1', got %q", patterns[0].ResourceName)
	}
	if patterns[1].ResourceName != "node-1" {
		t.Fatalf("expected pattern[1] name 'node-1', got %q", patterns[1].ResourceName)
	}

	predictions := adapter.GetPredictions()
	if len(predictions) != 1 {
		t.Fatalf("expected 1 prediction, got %d", len(predictions))
	}
	if predictions[0].ResourceName != "ct1" {
		t.Fatalf("expected prediction name 'ct1', got %q", predictions[0].ResourceName)
	}
}

func TestPatternAdapterReadStateFallbackForUnknownID(t *testing.T) {
	rs := &fakeReadState{
		vms: []*ur.VMView{newVMView("vm-100", "vm1", 100)},
	}

	source := &fakePatternSource{
		patterns: []PatternData{
			{ResourceID: "999", PatternType: "cpu", Description: "unknown"},
		},
	}

	adapter := NewPatternMCPAdapter(source, rs)
	patterns := adapter.GetPatterns()
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}
	// Unknown resource should return its ID as name
	if patterns[0].ResourceName != "999" {
		t.Fatalf("expected unknown resource name '999', got %q", patterns[0].ResourceName)
	}
}

func TestPatternAdapterNilReadState(t *testing.T) {
	source := &fakePatternSource{
		patterns: []PatternData{
			{ResourceID: "100", PatternType: "cpu"},
		},
	}

	adapter := NewPatternMCPAdapter(source, nil)
	patterns := adapter.GetPatterns()
	// nil readState → resource ID returned as name
	if patterns[0].ResourceName != "100" {
		t.Fatalf("expected resource ID '100' as name, got %q", patterns[0].ResourceName)
	}
}

// Verify that container name resolution also works via ReadState by checking
// the VMID-formatted lookup matches correctly.
func TestPatternAdapterReadStateContainerVMIDLookup(t *testing.T) {
	rs := &fakeReadState{
		containers: []*ur.ContainerView{
			newContainerView("ct-300", "my-container", 300),
		},
	}

	source := &fakePatternSource{
		patterns: []PatternData{
			{ResourceID: fmt.Sprintf("%d", 300), PatternType: "memory"},
		},
	}

	adapter := NewPatternMCPAdapter(source, rs)
	patterns := adapter.GetPatterns()
	if patterns[0].ResourceName != "my-container" {
		t.Fatalf("expected 'my-container', got %q", patterns[0].ResourceName)
	}
}

// TestMetricsSummaryNodeEmptySourceIDSkipped verifies that nodes with empty
// SourceID are gracefully skipped (no panic, no phantom summaries).
func TestMetricsSummaryNodeEmptySourceIDSkipped(t *testing.T) {
	now := time.Now()
	source := &fakeMetricsSource{
		guest: map[string]map[string][]RawMetricPoint{
			"100": {"cpu": {{Value: 10, Timestamp: now}}},
		},
	}

	// Node with empty SourceID (e.g., ingested via IngestRecords without legacy ID).
	emptySourceNode := newNodeView("reg-hash-only", "orphan-node", "")
	rs := &fakeReadState{
		vms:   []*ur.VMView{newVMView("vm-100", "vm1", 100)},
		nodes: []*ur.NodeView{emptySourceNode},
	}

	adapter := NewMetricsHistoryMCPAdapter(source, rs)
	summary, err := adapter.GetAllMetricsSummary(time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only the VM should appear — the empty-SourceID node is skipped.
	if len(summary) != 1 {
		t.Fatalf("expected 1 summary (vm only), got %d", len(summary))
	}
}

// TestPatternAdapterNodeEmptySourceIDFallsBackToID verifies that when a node
// has an empty SourceID, pattern name resolution falls through to returning
// the raw resourceID.
func TestPatternAdapterNodeEmptySourceIDFallsBackToID(t *testing.T) {
	emptySourceNode := newNodeView("reg-hash-only", "orphan-node", "")
	rs := &fakeReadState{
		nodes: []*ur.NodeView{emptySourceNode},
	}

	source := &fakePatternSource{
		patterns: []PatternData{
			{ResourceID: "some-legacy-id", PatternType: "cpu"},
		},
	}

	adapter := NewPatternMCPAdapter(source, rs)
	patterns := adapter.GetPatterns()
	// No node matches "some-legacy-id" (the node has empty SourceID),
	// so the raw resource ID is returned as the name.
	if patterns[0].ResourceName != "some-legacy-id" {
		t.Fatalf("expected fallback to resource ID 'some-legacy-id', got %q", patterns[0].ResourceName)
	}
}
