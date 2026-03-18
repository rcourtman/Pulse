package unifiedresources

import "testing"

func TestMonitoredSystemCountDedupesAcrossAgentAndAPIViews(t *testing.T) {
	registry := NewRegistry(nil)
	registry.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				ID:     "host-1",
				Type:   ResourceTypeAgent,
				Name:   "lab-a",
				Status: StatusOnline,
				Agent: &AgentData{
					AgentID:   "agent-1",
					Hostname:  "lab-a",
					MachineID: "machine-1",
				},
				Identity: ResourceIdentity{
					MachineID: "machine-1",
					Hostnames: []string{"lab-a"},
				},
			},
		},
	})
	registry.IngestRecords(SourcePBS, []IngestRecord{
		{
			SourceID: "pbs-1",
			Resource: Resource{
				ID:     "pbs-1",
				Type:   ResourceTypePBS,
				Name:   "pbs-a",
				Status: StatusOnline,
				PBS: &PBSData{
					InstanceID: "pbs-1",
					Hostname:   "lab-a",
					HostURL:    "https://lab-a:8007",
				},
			},
		},
	})
	registry.IngestRecords(SourcePMG, []IngestRecord{
		{
			SourceID: "pmg-1",
			Resource: Resource{
				ID:     "pmg-1",
				Type:   ResourceTypePMG,
				Name:   "pmg-b",
				Status: StatusOnline,
				PMG: &PMGData{
					InstanceID: "pmg-1",
					Hostname:   "mail-b",
				},
			},
		},
	})
	registry.IngestRecords(SourceK8s, []IngestRecord{
		{
			SourceID: "k8s-1",
			Resource: Resource{
				ID:     "k8s-1",
				Type:   ResourceTypeK8sCluster,
				Name:   "cluster-a",
				Status: StatusOnline,
				Kubernetes: &K8sData{
					ClusterID: "cluster-a",
					AgentID:   "k8s-agent-1",
					Server:    "https://cluster-a.example:6443",
				},
			},
		},
	})

	if got := MonitoredSystemCount(registry); got != 3 {
		t.Fatalf("MonitoredSystemCount() = %d, want 3", got)
	}
}

func TestHasMatchingMonitoredSystemMatchesByCanonicalHostIdentity(t *testing.T) {
	registry := NewRegistry(nil)
	registry.IngestRecords(SourceTrueNAS, []IngestRecord{
		{
			SourceID: "truenas-1",
			Resource: Resource{
				ID:     "truenas-1",
				Type:   ResourceTypeAgent,
				Name:   "archive",
				Status: StatusOnline,
				TrueNAS: &TrueNASData{
					Hostname: "archive.local",
				},
				Identity: ResourceIdentity{
					Hostnames: []string{"archive.local"},
				},
			},
		},
	})

	if !HasMatchingMonitoredSystem(registry, MonitoredSystemCandidate{
		Type:     ResourceTypeAgent,
		Hostname: "archive.local",
		HostURL:  "https://archive.local",
	}) {
		t.Fatal("expected candidate to match existing counted system")
	}

	if HasMatchingMonitoredSystem(registry, MonitoredSystemCandidate{
		Type:     ResourceTypePBS,
		Hostname: "other.local",
		HostURL:  "https://other.local:8007",
	}) {
		t.Fatal("expected unrelated candidate not to match existing counted system")
	}
}
