package tools

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file raises measured branch coverage for three previously
// partially-covered functions in internal/ai/tools:
//   - summarizeReportTypeForResource (tools_summarize.go) — pure switch over
//     unified ResourceType; every mapped type plus the agent sub-classification
//     and the unknown/default arm.
//   - normalizeDiscoveryResourceRequest (tools_discovery.go) — arg normalisation
//     with many distinct rejection reasons plus the accepting (success) path.
//   - validateRoutingContext (tools_query.go) — routing-mismatch guard with one
//     case per distinct early-return reason plus the blocking (accepting) case.
//
// It reuses the existing in-package fakes `mockResolvedContext` and `mockResource`
// (strict_resolution_test.go) and `stubDiscoveryProvider` (tools_discovery_test.go),
// and adds two small fakes below: a minimal unifiedresources.ReadState and a
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
func (t *branchcov0725amTelemetry) RecordAutoRecoveryAttempt(_, _ string)   {}
func (t *branchcov0725amTelemetry) RecordAutoRecoverySuccess(_, _ string)   {}
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
// normalizeDiscoveryResourceRequest (tools_discovery.go:242)
//
// The function implements a chain of distinct rejection reasons followed by
// the accepting (success) path. The table below drives every distinct error
// message and every success shape, asserting on the concrete error text, the
// ok flag, and the returned request struct (including the computed cliAccess).
// ---------------------------------------------------------------------------

func TestBranchcov0725amNormalizeDiscoveryResourceRequest_Rejections(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{DiscoveryProvider: &stubDiscoveryProvider{}})

	cases := []struct {
		name        string
		args        map[string]interface{}
		wantOK      bool
		wantErrHas  string // substring expected in blocked.Content[0].Text
		wantErrIsEr bool
	}{
		// nil args: every type assertion on a nil map yields "".
		{name: "nil_args_resource_type_required", args: nil, wantOK: false, wantErrHas: "resource_type is required", wantErrIsEr: true},
		// Empty map: no keys at all.
		{name: "empty_map_resource_type_required", args: map[string]interface{}{}, wantOK: false, wantErrHas: "resource_type is required", wantErrIsEr: true},
		// Whitespace-only resource_type: CanonicalDiscoveryResourceType trims to "".
		{name: "whitespace_resource_type_required", args: map[string]interface{}{"resource_type": "   ", "resource_id": "1", "target_id": "n1"}, wantOK: false, wantErrHas: "resource_type is required", wantErrIsEr: true},
		// Legacy "docker" token: duplicate spelling of app-container, explicitly unsupported here.
		{name: "legacy_docker_token_unsupported", args: map[string]interface{}{"resource_type": "docker", "resource_id": "c1", "target_id": "a1"}, wantOK: false, wantErrHas: "unsupported resource_type", wantErrIsEr: true},
		// Non-legacy but unsupported canonical type (passes the legacy check, fails isSupportedDiscoveryResourceType).
		{name: "k8s_cluster_unsupported", args: map[string]interface{}{"resource_type": "k8s-cluster", "resource_id": "1", "target_id": "n1"}, wantOK: false, wantErrHas: "unsupported resource_type", wantErrIsEr: true},
		// Valid type, missing resource_id.
		{name: "empty_resource_id_required", args: map[string]interface{}{"resource_type": "vm", "target_id": "n1"}, wantOK: false, wantErrHas: "resource_id is required", wantErrIsEr: true},
		// NOTE: resource_id is NOT trimmed before the required-check (unlike
		// target_id). A whitespace-only id passes that check, fails strconv.Atoi,
		// and falls through to the name-resolution branch, surfacing the
		// "could not resolve resource name" error instead. This case pins that
		// real behaviour; see the report for the trimming inconsistency.
		{name: "whitespace_resource_id_falls_through_to_name_resolution", args: map[string]interface{}{"resource_type": "vm", "resource_id": "\t ", "target_id": "n1"}, wantOK: false, wantErrHas: "could not resolve resource name", wantErrIsEr: true},
		// Valid type + id, missing target_id.
		{name: "empty_target_id_required", args: map[string]interface{}{"resource_type": "vm", "resource_id": "101"}, wantOK: false, wantErrHas: "target_id is required", wantErrIsEr: true},
		{name: "whitespace_target_id_required", args: map[string]interface{}{"resource_type": "vm", "resource_id": "101", "target_id": "  "}, wantOK: false, wantErrHas: "target_id is required", wantErrIsEr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, blocked, ok := exec.normalizeDiscoveryResourceRequest(tc.args)
			require.Equal(t, tc.wantOK, ok, "ok flag")
			require.True(t, blocked.IsError, "expected an error result for a rejection")
			require.Len(t, blocked.Content, 1, "expected exactly one content block")
			assert.Contains(t, blocked.Content[0].Text, tc.wantErrHas, "error message substring")
		})
	}
}

func TestBranchcov0725amNormalizeDiscoveryResourceRequest_ProviderNil(t *testing.T) {
	// discoveryProvider == nil short-circuits with a friendly text result (not IsError).
	exec := NewPulseToolExecutor(ExecutorConfig{})
	_, blocked, ok := exec.normalizeDiscoveryResourceRequest(map[string]interface{}{
		"resource_type": "vm", "resource_id": "1", "target_id": "n1",
	})
	require.False(t, ok)
	require.False(t, blocked.IsError, "the nil-provider message is advisory text, not an error")
	require.Len(t, blocked.Content, 1)
	assert.Contains(t, blocked.Content[0].Text, "Discovery service not available.")
}

func TestBranchcov0725amNormalizeDiscoveryResourceRequest_CurrentResourceNoContext(t *testing.T) {
	// A current_resource handle in any identity field forces resolveCurrentResource;
	// with no resolved context attached it must surface that specific error.
	exec := NewPulseToolExecutor(ExecutorConfig{DiscoveryProvider: &stubDiscoveryProvider{}})
	_, blocked, ok := exec.normalizeDiscoveryResourceRequest(map[string]interface{}{
		"resource_type": "current_resource",
		"resource_id":   "1",
		"target_id":     "n1",
	})
	require.False(t, ok)
	require.True(t, blocked.IsError)
	require.Len(t, blocked.Content, 1)
	assert.Contains(t, blocked.Content[0].Text, "current_resource")
}

func TestBranchcov0725amNormalizeDiscoveryResourceRequest_CurrentResourceResolved(t *testing.T) {
	// The accepting arm of the current_resource branch: an attached context
	// resolves the handle to canonical vm coordinates, which then flow through
	// the normal success path.
	res := &mockResource{
		resourceID:   "vm:101",
		resourceType: "vm",
		kind:         "vm",
		providerUID:  "101",
		targetHost:   "pve1",
		node:         "pve1",
	}
	ctx := &mockResolvedContext{
		resources: map[string]ResolvedResourceInfo{"vm:101": res},
		aliases:   map[string]ResolvedResourceInfo{"vm:101": res},
	}
	ctx.MarkExplicitAccess("vm:101")

	exec := NewPulseToolExecutor(ExecutorConfig{DiscoveryProvider: &stubDiscoveryProvider{}})
	exec.SetResolvedContext(ctx)

	req, blocked, ok := exec.normalizeDiscoveryResourceRequest(map[string]interface{}{
		"resource_type": "current_resource",
	})
	require.True(t, ok, "expected success after resolving current_resource")
	require.False(t, blocked.IsError)
	assert.Equal(t, "vm", req.resourceType)
	assert.Equal(t, "101", req.resourceID)
	assert.Equal(t, "pve1", req.targetID)
}

func TestBranchcov0725amNormalizeDiscoveryResourceRequest_SuccessCanonical(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{DiscoveryProvider: &stubDiscoveryProvider{}})

	cases := []struct {
		name string
		args map[string]interface{}
		want discoveryResourceRequest
	}{
		// agent: cliAccess uses targetID as the agent id.
		{
			name: "agent_numeric_success",
			args: map[string]interface{}{"resource_type": "agent", "resource_id": "host-1", "target_id": "node-a"},
			want: discoveryResourceRequest{
				resourceType:         "agent",
				providerResourceType: "agent",
				resourceID:           "host-1",
				targetID:             "node-a",
				cliAccess:            "Agent 'node-a'",
			},
		},
		// vm with a numeric VMID: no name resolution needed.
		{
			name: "vm_numeric_vmid_success",
			args: map[string]interface{}{"resource_type": "vm", "resource_id": "101", "target_id": "pve1"},
			want: discoveryResourceRequest{
				resourceType:         "vm",
				providerResourceType: "vm",
				resourceID:           "101",
				targetID:             "pve1",
				cliAccess:            "VM on node 'pve1' (VMID 101)",
			},
		},
		// system-container with a numeric VMID.
		{
			name: "system_container_numeric_vmid_success",
			args: map[string]interface{}{"resource_type": "system-container", "resource_id": "200", "target_id": "pve1"},
			want: discoveryResourceRequest{
				resourceType:         "system-container",
				providerResourceType: "system-container",
				resourceID:           "200",
				targetID:             "pve1",
				cliAccess:            "System container on node 'pve1' (VMID 200)",
			},
		},
		// Canonical app-container (provider type flips to "docker"); nil provider
		// means findCanonicalAppContainerResource is a no-op, so the id passes through.
		{
			name: "app_container_canonical_success",
			args: map[string]interface{}{"resource_type": "app-container", "resource_id": "cid1", "target_id": "agent-1"},
			want: discoveryResourceRequest{
				resourceType:         "app-container",
				providerResourceType: "docker",
				resourceID:           "cid1",
				targetID:             "agent-1",
				cliAccess:            "Docker container 'cid1' on target 'agent-1'",
			},
		},
		// Surrounding whitespace/case is canonicalised on every field; the
		// target_id is trimmed before the required-check.
		{
			name: "whitespace_and_case_canonicalised",
			args: map[string]interface{}{"resource_type": "  VM ", "resource_id": "303", "target_id": "  Pve1  "},
			want: discoveryResourceRequest{
				resourceType:         "vm",
				providerResourceType: "vm",
				resourceID:           "303",
				targetID:             "Pve1",
				cliAccess:            "VM on node 'Pve1' (VMID 303)",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, blocked, ok := exec.normalizeDiscoveryResourceRequest(tc.args)
			require.True(t, ok, "expected success; got error result %q", blockedText(blocked))
			require.False(t, blocked.IsError, "expected a clean (non-error) result on success")
			assert.Equal(t, tc.want, req, "normalised request struct")
		})
	}
}

// TestBranchcov0725amNormalizeDiscoveryResourceRequest_NameToVMIDResolution
// drives the system-container/vm name->VMID resolution branch (the strconv.Atoi
// failure path inside the function). A typed name on the matching node resolves
// to its VMID; the same name on a mismatched node, or an absent name, surfaces
// the specific "could not resolve resource name" error.
func TestBranchcov0725amNormalizeDiscoveryResourceRequest_NameToVMIDResolution(t *testing.T) {
	ct := unifiedresources.NewContainerView(&unifiedresources.Resource{
		Name: "myct", Proxmox: &unifiedresources.ProxmoxData{NodeName: "pve1", VMID: 200},
	})
	vm := unifiedresources.NewVMView(&unifiedresources.Resource{
		Name: "myvm", Proxmox: &unifiedresources.ProxmoxData{NodeName: "pve1", VMID: 101},
	})
	vmOtherNode := unifiedresources.NewVMView(&unifiedresources.Resource{
		Name: "offnode", Proxmox: &unifiedresources.ProxmoxData{NodeName: "pve2", VMID: 999},
	})
	rs := &branchcov0725amReadState{
		vms:        []*unifiedresources.VMView{&vm, &vmOtherNode},
		containers: []*unifiedresources.ContainerView{&ct},
	}

	t.Run("system_container_name_resolves_to_vmid", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{DiscoveryProvider: &stubDiscoveryProvider{}})
		exec.readState = rs
		req, blocked, ok := exec.normalizeDiscoveryResourceRequest(map[string]interface{}{
			"resource_type": "system-container", "resource_id": "myct", "target_id": "pve1",
		})
		require.True(t, ok, blockedText(blocked))
		assert.Equal(t, "system-container", req.resourceType)
		assert.Equal(t, "200", req.resourceID, "name should resolve to VMID")
		assert.Equal(t, "pve1", req.targetID)
	})

	t.Run("vm_name_resolves_to_vmid", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{DiscoveryProvider: &stubDiscoveryProvider{}})
		exec.readState = rs
		req, blocked, ok := exec.normalizeDiscoveryResourceRequest(map[string]interface{}{
			"resource_type": "vm", "resource_id": "myvm", "target_id": "pve1",
		})
		require.True(t, ok, blockedText(blocked))
		assert.Equal(t, "101", req.resourceID, "name should resolve to VMID")
	})

	t.Run("vm_name_on_wrong_node_not_resolved", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{DiscoveryProvider: &stubDiscoveryProvider{}})
		exec.readState = rs
		_, blocked, ok := exec.normalizeDiscoveryResourceRequest(map[string]interface{}{
			"resource_type": "vm", "resource_id": "offnode", "target_id": "pve1",
		})
		require.False(t, ok)
		require.True(t, blocked.IsError)
		assert.Contains(t, blocked.Content[0].Text, "could not resolve resource name 'offnode' to a VMID on target 'pve1'")
	})

	t.Run("vm_name_absent_not_resolved", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{DiscoveryProvider: &stubDiscoveryProvider{}})
		exec.readState = rs
		_, blocked, ok := exec.normalizeDiscoveryResourceRequest(map[string]interface{}{
			"resource_type": "vm", "resource_id": "ghost", "target_id": "pve1",
		})
		require.False(t, ok)
		require.True(t, blocked.IsError)
		assert.Contains(t, blocked.Content[0].Text, "could not resolve resource name 'ghost' to a VMID on target 'pve1'")
	})
}

// blockedText returns the text payload of a CallToolResult for diagnostic
// assertion messages, or "<no content>" when empty.
func blockedText(r CallToolResult) string {
	if len(r.Content) == 0 {
		return "<no content>"
	}
	return r.Content[0].Text
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
