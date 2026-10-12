package tools

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file raises measured branch coverage for two previously
// partially-covered functions in internal/ai/tools:
//   - summarizeReportTypeForResource (tools_summarize.go) — pure switch over
//     unified ResourceType; every mapped type plus the agent sub-classification
//     and the unknown/default arm.
//   - validateRoutingContext (tools_query.go) — routing-mismatch guard with one
//     case per distinct early-return reason plus the blocking (accepting) case.
//
// It reuses the existing in-package fakes `mockResolvedContext` and `mockResource`
// (strict_resolution_test.go), and adds two small fakes below: a minimal unifiedresources.ReadState and a
// TelemetryCallback recorder.

// branchcov0725amReadState is a minimal ReadState whose only populated buckets
// are the Proxmox node/vm/container views that resolveResourceLocation and
// findRecentlyReferencedChildrenOnNode walk. Every other accessor returns nil,
// which is the documented empty-state contract.
type branchcov0725amReadState struct {
	nodes      []*unifiedresources.NodeView
	vms        []*unifiedresources.VMView
	containers []*unifiedresources.ContainerView
	hosts      []*unifiedresources.HostView
}

func (f *branchcov0725amReadState) Nodes() []*unifiedresources.NodeView { return f.nodes }
func (f *branchcov0725amReadState) VMs() []*unifiedresources.VMView     { return f.vms }
func (f *branchcov0725amReadState) Containers() []*unifiedresources.ContainerView {
	return f.containers
}
func (f *branchcov0725amReadState) Hosts() []*unifiedresources.HostView { return f.hosts }

func (f *branchcov0725amReadState) DockerHosts() []*unifiedresources.DockerHostView {
	return nil
}
func (f *branchcov0725amReadState) DockerContainers() []*unifiedresources.DockerContainerView {
	return nil
}
func (f *branchcov0725amReadState) StoragePools() []*unifiedresources.StoragePoolView {
	return nil
}
func (f *branchcov0725amReadState) PhysicalDisks() []*unifiedresources.PhysicalDiskView {
	return nil
}
func (f *branchcov0725amReadState) PBSInstances() []*unifiedresources.PBSInstanceView {
	return nil
}
func (f *branchcov0725amReadState) PMGInstances() []*unifiedresources.PMGInstanceView {
	return nil
}
func (f *branchcov0725amReadState) K8sClusters() []*unifiedresources.K8sClusterView { return nil }
func (f *branchcov0725amReadState) K8sNodes() []*unifiedresources.K8sNodeView       { return nil }
func (f *branchcov0725amReadState) Pods() []*unifiedresources.PodView               { return nil }
func (f *branchcov0725amReadState) K8sDeployments() []*unifiedresources.K8sDeploymentView {
	return nil
}
func (f *branchcov0725amReadState) Workloads() []*unifiedresources.WorkloadView {
	return nil
}
func (f *branchcov0725amReadState) Infrastructure() []*unifiedresources.InfrastructureView {
	return nil
}

// branchcov0725amTelemetry records the routing-mismatch telemetry call so the
// test can assert the real function invoked it with the expected labels.
type branchcov0725amTelemetry struct {
	routingMismatchCalls int
	lastTool             string
	lastTargetKind       string
	lastChildKind        string
}

func (t *branchcov0725amTelemetry) RecordStrictResolutionBlock(_, _ string) {}
func (t *branchcov0725amTelemetry) RecordRoutingMismatchBlock(tool, targetKind, childKind string) {
	t.routingMismatchCalls++
	t.lastTool = tool
	t.lastTargetKind = targetKind
	t.lastChildKind = childKind
}

// ---------------------------------------------------------------------------
// summarizeReportTypeForResource (tools_summarize.go:292)
//
// Pure switch over CanonicalResourceType(res.Type). The agent arm further
// classifies by the richest populated host payload (Agent -> "agent",
// Proxmox -> "node", Docker -> "docker-host", default -> "agent"). Every
// other mapped type returns a fixed string; unknown types return "".
// ---------------------------------------------------------------------------

func TestBranchcov0725amSummarizeReportTypeForResource(t *testing.T) {
	cases := []struct {
		name string
		res  unifiedresources.Resource
		want string
	}{
		// --- ResourceTypeAgent: inner precedence switch, each arm. ---
		{
			name: "agent_with_agent_payload",
			res:  unifiedresources.Resource{Type: unifiedresources.ResourceTypeAgent, Agent: &unifiedresources.AgentData{}},
			want: "agent",
		},
		{
			name: "agent_with_proxmox_payload_maps_to_node",
			res:  unifiedresources.Resource{Type: unifiedresources.ResourceTypeAgent, Proxmox: &unifiedresources.ProxmoxData{}},
			want: "node",
		},
		{
			name: "agent_with_docker_payload_maps_to_docker_host",
			res:  unifiedresources.Resource{Type: unifiedresources.ResourceTypeAgent, Docker: &unifiedresources.DockerData{}},
			want: "docker-host",
		},
		{
			name: "agent_with_no_host_payload_defaults_to_agent",
			res:  unifiedresources.Resource{Type: unifiedresources.ResourceTypeAgent},
			want: "agent",
		},
		// Precedence is first-match: Agent beats Proxmox beats Docker.
		{
			name: "agent_with_agent_and_proxmex_picks_agent",
			res: unifiedresources.Resource{
				Type:    unifiedresources.ResourceTypeAgent,
				Agent:   &unifiedresources.AgentData{},
				Proxmox: &unifiedresources.ProxmoxData{},
			},
			want: "agent",
		},
		{
			name: "agent_with_proxmox_and_docker_picks_node",
			res: unifiedresources.Resource{
				Type:    unifiedresources.ResourceTypeAgent,
				Proxmox: &unifiedresources.ProxmoxData{},
				Docker:  &unifiedresources.DockerData{},
			},
			want: "node",
		},

		// --- Direct canonical-type arms (each maps to a fixed report type). ---
		{name: "vm", res: unifiedresources.Resource{Type: unifiedresources.ResourceTypeVM}, want: "vm"},
		{name: "system_container", res: unifiedresources.Resource{Type: unifiedresources.ResourceTypeSystemContainer}, want: "system-container"},
		{name: "app_container", res: unifiedresources.Resource{Type: unifiedresources.ResourceTypeAppContainer}, want: "app-container"},
		{name: "k8s_cluster", res: unifiedresources.Resource{Type: unifiedresources.ResourceTypeK8sCluster}, want: "k8s"},
		{name: "storage", res: unifiedresources.Resource{Type: unifiedresources.ResourceTypeStorage}, want: "storage"},
		{name: "pbs", res: unifiedresources.Resource{Type: unifiedresources.ResourceTypePBS}, want: "pbs"},
		{name: "pmg", res: unifiedresources.Resource{Type: unifiedresources.ResourceTypePMG}, want: "pmg"},
		{name: "physical_disk_maps_to_disk", res: unifiedresources.Resource{Type: unifiedresources.ResourceTypePhysicalDisk}, want: "disk"},
		{name: "pod", res: unifiedresources.Resource{Type: unifiedresources.ResourceTypePod}, want: "pod"},

		// --- default arm: unknown / empty types return "". ---
		{name: "unknown_type_returns_empty", res: unifiedresources.Resource{Type: unifiedresources.ResourceType("widget")}, want: ""},
		{name: "empty_type_returns_empty", res: unifiedresources.Resource{Type: ""}, want: ""},

		// CanonicalResourceType normalises whitespace + case before the switch.
		{name: "padded_upper_vm_normalises", res: unifiedresources.Resource{Type: unifiedresources.ResourceType("  VM ")}, want: "vm"},
		{name: "padded_upper_agent_with_proxmex_normalises", res: unifiedresources.Resource{Type: unifiedresources.ResourceType(" Agent "), Proxmox: &unifiedresources.ProxmoxData{}}, want: "node"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := summarizeReportTypeForResource(tc.res)
			assert.Equal(t, tc.want, got, "summarizeReportTypeForResource mismatch")
		})
	}
}

// ---------------------------------------------------------------------------
// validateRoutingContext (tools_query.go:1985)
//
// One case per distinct early-return (not-blocked) reason, plus the accepting
// (blocking) case that returns an ErrRoutingMismatch. Each not-blocked case
// asserts IsBlocked()==false; the block case asserts the exact mismatch fields
// and that telemetry was recorded.
// ---------------------------------------------------------------------------

func TestBranchcov0725amValidateRoutingContext_NotBlocked(t *testing.T) {
	// Shared read state with one node "pve1", one container on pve1, one vm on pve1.
	pve1 := unifiedresources.NewNodeView(&unifiedresources.Resource{
		Name: "pve1", Type: unifiedresources.ResourceTypeAgent, Proxmox: &unifiedresources.ProxmoxData{NodeName: "pve1"},
	})
	jellyfinCT := unifiedresources.NewContainerView(&unifiedresources.Resource{
		Name: "jellyfin", Proxmox: &unifiedresources.ProxmoxData{NodeName: "pve1", VMID: 100},
	})
	winVM := unifiedresources.NewVMView(&unifiedresources.Resource{
		Name: "win10", Proxmox: &unifiedresources.ProxmoxData{NodeName: "pve1", VMID: 200},
	})
	rs := &branchcov0725amReadState{
		nodes:      []*unifiedresources.NodeView{&pve1},
		containers: []*unifiedresources.ContainerView{&jellyfinCT},
		vms:        []*unifiedresources.VMView{&winVM},
	}

	// A child that exists on pve1 but is NOT recently accessed (drives the
	// WasRecentlyAccessed==false arm inside findRecentlyReferencedChildrenOnNode).
	staleChild := &mockResource{
		resourceID: "system-container:100", resourceType: "system-container",
		kind: "system-container", providerUID: "100", node: "pve1",
	}
	staleCtx := &mockResolvedContext{
		resources: map[string]ResolvedResourceInfo{"system-container:100": staleChild},
		aliases:   map[string]ResolvedResourceInfo{"jellyfin": staleChild, "system-container:100": staleChild},
		// lastAccessed deliberately nil: not recently accessed.
	}

	// A direct-alias target so GetResolvedResourceByAlias hits on the first lookup.
	directHost := &mockResource{resourceID: "agent:delly", kind: "agent"}
	directCtx := &mockResolvedContext{
		aliases: map[string]ResolvedResourceInfo{"delly": directHost},
	}

	cases := []struct {
		name       string
		exec       *PulseToolExecutor
		targetHost string
	}{
		{
			// Arm: !hasReadState() — no read state provider wired at all, so the
			// very first guard returns not-blocked regardless of context.
			name:       "no_read_state_not_blocked",
			exec:       &PulseToolExecutor{resolvedContext: staleCtx},
			targetHost: "pve1",
		},
		{
			// Arm: hasReadState() true but resolvedContext == nil.
			name:       "nil_resolved_context_not_blocked",
			exec:       &PulseToolExecutor{readState: rs},
			targetHost: "pve1",
		},
		{
			// Arm: targetHost resolves directly to a resource alias -> no mismatch.
			name:       "direct_alias_match_not_blocked",
			exec:       &PulseToolExecutor{readState: rs, resolvedContext: directCtx},
			targetHost: "delly",
		},
		{
			// Arm: resolveResourceLocation can't find the target -> !loc.Found.
			name:       "location_not_found_not_blocked",
			exec:       &PulseToolExecutor{readState: rs, resolvedContext: staleCtx},
			targetHost: "does-not-exist",
		},
		{
			// Arm: loc.Found but ResourceType != "node" (here a vm). Note "win10"
			// is intentionally absent from aliases so the direct-lookup is skipped.
			name:       "location_found_but_not_node_not_blocked",
			exec:       &PulseToolExecutor{readState: rs, resolvedContext: staleCtx},
			targetHost: "win10",
		},
		{
			// Arm: target is a node, a child exists on it, but the child was NOT
			// recently accessed -> len(recentChildren)==0.
			name:       "node_no_recent_children_not_blocked",
			exec:       &PulseToolExecutor{readState: rs, resolvedContext: staleCtx},
			targetHost: "pve1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := tc.exec.validateRoutingContext(tc.targetHost)
			assert.False(t, res.IsBlocked(), "this arm must not block")
			assert.Nil(t, res.RoutingError)
		})
	}
}

func TestBranchcov0725amValidateRoutingContext_BlocksSystemContainer(t *testing.T) {
	// Accepting case: target is a Proxmox node and a system-container child on
	// that node was recently referenced -> block with an ErrRoutingMismatch whose
	// fields name the offending child, and fire the telemetry callback.
	pve1 := unifiedresources.NewNodeView(&unifiedresources.Resource{
		Name: "pve1", Type: unifiedresources.ResourceTypeAgent, Proxmox: &unifiedresources.ProxmoxData{NodeName: "pve1"},
	})
	jellyfinCT := unifiedresources.NewContainerView(&unifiedresources.Resource{
		Name: "jellyfin", Proxmox: &unifiedresources.ProxmoxData{NodeName: "pve1", VMID: 100},
	})
	rs := &branchcov0725amReadState{
		nodes:      []*unifiedresources.NodeView{&pve1},
		containers: []*unifiedresources.ContainerView{&jellyfinCT},
	}

	child := &mockResource{
		resourceID: "system-container:100", resourceType: "system-container",
		kind: "system-container", providerUID: "100", node: "pve1",
	}
	ctx := &mockResolvedContext{
		resources: map[string]ResolvedResourceInfo{"system-container:100": child},
		aliases:   map[string]ResolvedResourceInfo{"jellyfin": child, "system-container:100": child},
	}
	ctx.MarkExplicitAccess("system-container:100")

	telemetry := &branchcov0725amTelemetry{}
	exec := &PulseToolExecutor{readState: rs, resolvedContext: ctx, telemetryCallback: telemetry}

	res := exec.validateRoutingContext("pve1")

	require.True(t, res.IsBlocked(), "expected the routing mismatch to block")
	require.NotNil(t, res.RoutingError)
	mm := res.RoutingError
	assert.Equal(t, "pve1", mm.TargetHost)
	assert.Equal(t, []string{"jellyfin"}, mm.MoreSpecificResources)
	assert.Equal(t, []string{"system-container:100"}, mm.MoreSpecificIDs)
	assert.Equal(t, []string{"system-container"}, mm.ChildKinds)
	assert.Contains(t, mm.Message, "target_host 'pve1' is a hypervisor node")
	assert.Contains(t, mm.Message, "jellyfin")
	assert.Equal(t, "routing_validation", telemetry.lastTool)
	assert.Equal(t, "node", telemetry.lastTargetKind)
	assert.Equal(t, "system-container", telemetry.lastChildKind)
	assert.Equal(t, 1, telemetry.routingMismatchCalls)
}

func TestBranchcov0725amValidateRoutingContext_BlocksVMChildWithoutTelemetry(t *testing.T) {
	// Drives the VM iteration arm of findRecentlyReferencedChildrenOnNode and the
	// telemetryCallback==nil arm of the block path: the block still fires, the
	// vm kind propagates into ChildKinds, and no telemetry is recorded.
	pve1 := unifiedresources.NewNodeView(&unifiedresources.Resource{
		Name: "pve1", Type: unifiedresources.ResourceTypeAgent, Proxmox: &unifiedresources.ProxmoxData{NodeName: "pve1"},
	})
	winVM := unifiedresources.NewVMView(&unifiedresources.Resource{
		Name: "win10", Proxmox: &unifiedresources.ProxmoxData{NodeName: "pve1", VMID: 200},
	})
	rs := &branchcov0725amReadState{
		nodes: []*unifiedresources.NodeView{&pve1},
		vms:   []*unifiedresources.VMView{&winVM},
	}

	child := &mockResource{
		resourceID: "vm:200", resourceType: "vm", kind: "vm", providerUID: "200", node: "pve1",
	}
	ctx := &mockResolvedContext{
		resources: map[string]ResolvedResourceInfo{"vm:200": child},
		aliases:   map[string]ResolvedResourceInfo{"win10": child, "vm:200": child},
	}
	ctx.MarkExplicitAccess("vm:200")

	exec := &PulseToolExecutor{readState: rs, resolvedContext: ctx}
	res := exec.validateRoutingContext("pve1")

	require.True(t, res.IsBlocked())
	require.NotNil(t, res.RoutingError)
	assert.Equal(t, []string{"win10"}, res.RoutingError.MoreSpecificResources)
	assert.Equal(t, []string{"vm"}, res.RoutingError.ChildKinds)
	assert.Equal(t, ErrCodeRoutingMismatch, res.RoutingError.Code())
}
