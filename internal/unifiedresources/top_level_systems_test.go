package unifiedresources

import "testing"

func TestResolveTopLevelSystemsTopLevelSourceMatrix(t *testing.T) {
	testCases := []struct {
		name      string
		resources []Resource
		wantCount int
		same      [][2]string
		different [][2]string
	}{
		{
			name: "agent and docker host share one top-level system",
			resources: []Resource{
				topLevelTestAgent("agent-host", "tower.local", "machine-1", "agent-1"),
				topLevelTestDockerHost("docker-host", "tower.local", "docker-runtime-1", "agent-1"),
			},
			wantCount: 1,
			same:      [][2]string{{"agent-host", "docker-host"}},
		},
		{
			name: "agent and proxmox node share one top-level system",
			resources: []Resource{
				topLevelTestAgent("agent-host", "tower.local", "machine-1", "agent-1"),
				topLevelTestProxmoxNode("proxmox-node", "tower", "proxmox-1", "https://tower.local:8006"),
			},
			wantCount: 1,
			same:      [][2]string{{"agent-host", "proxmox-node"}},
		},
		{
			name: "agent and truenas share one top-level system",
			resources: []Resource{
				topLevelTestAgent("agent-host", "archive.local", "machine-1", "agent-1"),
				topLevelTestTrueNAS("truenas-system", "archive.local"),
			},
			wantCount: 1,
			same:      [][2]string{{"agent-host", "truenas-system"}},
		},
		{
			name: "agent and pbs share one top-level system",
			resources: []Resource{
				topLevelTestAgent("agent-host", "backup.local", "machine-1", "agent-1"),
				topLevelTestPBS("pbs-system", "backup.local", "pbs-1"),
			},
			wantCount: 1,
			same:      [][2]string{{"agent-host", "pbs-system"}},
		},
		{
			name: "agent and pmg share one top-level system",
			resources: []Resource{
				topLevelTestAgent("agent-host", "mail.local", "machine-1", "agent-1"),
				topLevelTestPMG("pmg-system", "mail.local", "pmg-1"),
			},
			wantCount: 1,
			same:      [][2]string{{"agent-host", "pmg-system"}},
		},
		{
			name: "kubernetes cluster remains separate even when the managing agent id is shared",
			resources: []Resource{
				topLevelTestAgent("agent-host", "cluster-host.local", "machine-1", "agent-1"),
				topLevelTestK8sCluster("cluster-1", "cluster-1", "agent-1", "https://cluster-host.local:6443"),
			},
			wantCount: 2,
			different: [][2]string{{"agent-host", "cluster-1"}},
		},
		{
			name: "equal-priority hostname-only hosts stay separate",
			resources: []Resource{
				topLevelTestAgentWithoutMachineID("host-a", "shared.local", "agent-1"),
				topLevelTestAgentWithoutMachineID("host-b", "shared.local", "agent-2"),
			},
			wantCount: 2,
			different: [][2]string{{"host-a", "host-b"}},
		},
		{
			name: "lower-priority fallback stays separate when host ownership is ambiguous",
			resources: []Resource{
				topLevelTestAgentWithoutMachineID("host-a", "shared.local", "agent-1"),
				topLevelTestAgentWithoutMachineID("host-b", "shared.local", "agent-2"),
				topLevelTestPBS("pbs-a", "shared.local", "pbs-1"),
			},
			wantCount: 3,
			different: [][2]string{
				{"host-a", "host-b"},
				{"host-a", "pbs-a"},
				{"host-b", "pbs-a"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolver := ResolveTopLevelSystems(tc.resources)
			if resolver.Count() != tc.wantCount {
				t.Fatalf("ResolveTopLevelSystems() count = %d, want %d", resolver.Count(), tc.wantCount)
			}
			assertTopLevelSystemGroupPairs(t, resolver, tc.same, tc.different)
		})
	}
}

func TestResolveTopLevelSystemsMixedEnvironmentCharacterization(t *testing.T) {
	resolver := ResolveTopLevelSystems([]Resource{
		topLevelTestAgent("agent-host", "tower.local", "machine-1", "agent-1"),
		topLevelTestDockerHost("docker-host", "tower.local", "docker-runtime-1", "agent-1"),
		topLevelTestProxmoxNode("proxmox-node", "tower", "proxmox-1", "https://tower.local:8006"),
		topLevelTestPBS("pbs-system", "tower.local", "pbs-1"),
		topLevelTestPMG("pmg-system", "mail.local", "pmg-1"),
		topLevelTestK8sCluster("cluster-1", "cluster-1", "agent-1", "https://tower.local:6443"),
	})

	if resolver.Count() != 3 {
		t.Fatalf("ResolveTopLevelSystems() count = %d, want 3", resolver.Count())
	}

	assertTopLevelSystemGroupPairs(t, resolver,
		[][2]string{
			{"agent-host", "docker-host"},
			{"agent-host", "proxmox-node"},
			{"agent-host", "pbs-system"},
		},
		[][2]string{
			{"agent-host", "pmg-system"},
			{"agent-host", "cluster-1"},
			{"pbs-system", "cluster-1"},
			{"pmg-system", "cluster-1"},
		},
	)
}

func TestResolveTopLevelSystemsExplainsStandaloneSystem(t *testing.T) {
	resolver := ResolveTopLevelSystems([]Resource{
		topLevelTestAgent("agent-host", "tower.local", "machine-1", "agent-1"),
	})

	records := resolver.records()
	if len(records) != 1 {
		t.Fatalf("expected one monitored-system record, got %d", len(records))
	}

	explanation := records[0].Explanation
	if explanation.Summary == "" {
		t.Fatal("expected standalone monitored system to carry an explanation summary")
	}
	if len(explanation.Reasons) != 1 || explanation.Reasons[0].Kind != "standalone" {
		t.Fatalf("expected standalone explanation reason, got %+v", explanation.Reasons)
	}
	if len(explanation.Surfaces) != 1 || explanation.Surfaces[0].Source != "agent" {
		t.Fatalf("expected one agent surface, got %+v", explanation.Surfaces)
	}
}

func TestResolveTopLevelSystemsExplainsStrongIdentityAndHostnameAttachment(t *testing.T) {
	resolver := ResolveTopLevelSystems([]Resource{
		topLevelTestAgent("agent-host", "tower.local", "machine-1", "agent-1"),
		topLevelTestDockerHost("docker-host", "tower.local", "docker-runtime-1", "agent-1"),
		topLevelTestPBS("pbs-system", "tower.local", "pbs-1"),
	})

	records := resolver.records()
	if len(records) != 1 {
		t.Fatalf("expected one monitored-system record, got %d", len(records))
	}

	reasons := records[0].Explanation.Reasons
	if len(reasons) < 2 {
		t.Fatalf("expected multiple grouping reasons, got %+v", reasons)
	}

	if !hasGroupingReason(reasons, "shared-identity", "agent-id") {
		t.Fatalf("expected shared agent identity reason, got %+v", reasons)
	}
	if !hasGroupingReason(reasons, "exact-host-attachment", "exact-host") {
		t.Fatalf("expected exact hostname attachment reason, got %+v", reasons)
	}
	if len(records[0].Explanation.Surfaces) != 3 {
		t.Fatalf("expected three grouped surfaces, got %+v", records[0].Explanation.Surfaces)
	}
}

func TestHasMatchingMonitoredSystemDoesNotMergeKubernetesCandidateBySharedAgentID(t *testing.T) {
	registry := NewRegistry(nil)
	registry.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: topLevelTestAgent("host-1", "tower.local", "machine-1", "agent-1"),
		},
	})

	if HasMatchingMonitoredSystem(registry, MonitoredSystemCandidate{
		Type:     ResourceTypeK8sCluster,
		AgentID:  "agent-1",
		Hostname: "tower.local",
		HostURL:  "https://tower.local:6443",
	}) {
		t.Fatal("expected kubernetes candidate to remain separate from the underlying host")
	}
}

func assertTopLevelSystemGroupPairs(t *testing.T, resolver TopLevelSystemResolver, same, different [][2]string) {
	t.Helper()

	for _, pair := range same {
		left, right := resolver.resourceToGroup[pair[0]], resolver.resourceToGroup[pair[1]]
		if left == "" || right == "" {
			t.Fatalf("expected resolver group ids for %q and %q, got %q and %q", pair[0], pair[1], left, right)
		}
		if left != right {
			t.Fatalf("expected %q and %q to share a group, got %q and %q", pair[0], pair[1], left, right)
		}
	}

	for _, pair := range different {
		left, right := resolver.resourceToGroup[pair[0]], resolver.resourceToGroup[pair[1]]
		if left == "" || right == "" {
			t.Fatalf("expected resolver group ids for %q and %q, got %q and %q", pair[0], pair[1], left, right)
		}
		if left == right {
			t.Fatalf("expected %q and %q to remain separate, both resolved to %q", pair[0], pair[1], left)
		}
	}
}

func hasGroupingReason(
	reasons []MonitoredSystemGroupingReason,
	kind string,
	signal string,
) bool {
	for _, reason := range reasons {
		if reason.Kind == kind && reason.Signal == signal {
			return true
		}
	}
	return false
}

func topLevelTestAgent(id, hostname, machineID, agentID string) Resource {
	resource := topLevelTestAgentWithoutMachineID(id, hostname, agentID)
	resource.Identity.MachineID = machineID
	resource.Agent.MachineID = machineID
	return resource
}

func topLevelTestAgentWithoutMachineID(id, hostname, agentID string) Resource {
	return Resource{
		ID:     id,
		Type:   ResourceTypeAgent,
		Name:   hostname,
		Status: StatusOnline,
		Agent: &AgentData{
			AgentID:  agentID,
			Hostname: hostname,
		},
		Identity: ResourceIdentity{
			Hostnames: []string{hostname},
		},
	}
}

func topLevelTestDockerHost(id, hostname, runtimeID, agentID string) Resource {
	return Resource{
		ID:     id,
		Type:   ResourceTypeAgent,
		Name:   hostname,
		Status: StatusOnline,
		Docker: &DockerData{
			HostSourceID: runtimeID,
			AgentID:      agentID,
			Hostname:     hostname,
		},
		Identity: ResourceIdentity{
			Hostnames: []string{hostname},
		},
	}
}

func topLevelTestProxmoxNode(id, nodeName, sourceID, hostURL string) Resource {
	return Resource{
		ID:     id,
		Type:   ResourceTypeAgent,
		Name:   nodeName,
		Status: StatusOnline,
		Proxmox: &ProxmoxData{
			SourceID: sourceID,
			NodeName: nodeName,
			HostURL:  hostURL,
		},
	}
}

func topLevelTestTrueNAS(id, hostname string) Resource {
	return Resource{
		ID:     id,
		Type:   ResourceTypeAgent,
		Name:   hostname,
		Status: StatusOnline,
		TrueNAS: &TrueNASData{
			Hostname: hostname,
		},
		Identity: ResourceIdentity{
			Hostnames: []string{hostname},
		},
	}
}

func topLevelTestPBS(id, hostname, instanceID string) Resource {
	return Resource{
		ID:     id,
		Type:   ResourceTypePBS,
		Name:   hostname,
		Status: StatusOnline,
		PBS: &PBSData{
			InstanceID: instanceID,
			Hostname:   hostname,
			HostURL:    "https://" + hostname + ":8007",
		},
	}
}

func topLevelTestPMG(id, hostname, instanceID string) Resource {
	return Resource{
		ID:     id,
		Type:   ResourceTypePMG,
		Name:   hostname,
		Status: StatusOnline,
		PMG: &PMGData{
			InstanceID: instanceID,
			Hostname:   hostname,
		},
	}
}

func topLevelTestK8sCluster(id, clusterID, agentID, server string) Resource {
	return Resource{
		ID:     id,
		Type:   ResourceTypeK8sCluster,
		Name:   clusterID,
		Status: StatusOnline,
		Kubernetes: &K8sData{
			ClusterID:   clusterID,
			ClusterName: clusterID,
			AgentID:     agentID,
			Server:      server,
		},
		Identity: ResourceIdentity{
			Hostnames: []string{clusterID, extractHostname(server)},
		},
	}
}
