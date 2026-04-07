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

func TestMonitoredSystemCountDoesNotMergeFriendlyNameCollisions(t *testing.T) {
	registry := NewRegistry(nil)
	registry.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				ID:     "host-1",
				Type:   ResourceTypeAgent,
				Name:   "Production",
				Status: StatusOnline,
				Agent: &AgentData{
					AgentID:   "agent-1",
					Hostname:  "alpha.local",
					MachineID: "machine-1",
				},
				Identity: ResourceIdentity{
					MachineID: "machine-1",
					Hostnames: []string{"alpha.local"},
				},
			},
		},
		{
			SourceID: "host-2",
			Resource: Resource{
				ID:     "host-2",
				Type:   ResourceTypeAgent,
				Name:   "Production",
				Status: StatusOnline,
				Agent: &AgentData{
					AgentID:   "agent-2",
					Hostname:  "beta.local",
					MachineID: "machine-2",
				},
				Identity: ResourceIdentity{
					MachineID: "machine-2",
					Hostnames: []string{"beta.local"},
				},
			},
		},
	})

	if got := MonitoredSystemCount(registry); got != 2 {
		t.Fatalf("MonitoredSystemCount() = %d, want 2", got)
	}

	if HasMatchingMonitoredSystem(registry, MonitoredSystemCandidate{
		Type: ResourceTypeAgent,
		Name: "Production",
	}) {
		t.Fatal("expected friendly name alone not to match an existing counted system")
	}
}

func TestMonitoredSystemCountExactHostnameFallbackDoesNotMergeEqualPriorityHosts(t *testing.T) {
	registry := NewRegistry(nil)
	registry.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				ID:     "host-1",
				Type:   ResourceTypeAgent,
				Name:   "one",
				Status: StatusOnline,
				Agent: &AgentData{
					AgentID:   "agent-1",
					Hostname:  "shared.local",
					MachineID: "machine-1",
				},
				Identity: ResourceIdentity{
					MachineID: "machine-1",
					Hostnames: []string{"shared.local"},
				},
			},
		},
		{
			SourceID: "host-2",
			Resource: Resource{
				ID:     "host-2",
				Type:   ResourceTypeAgent,
				Name:   "two",
				Status: StatusOnline,
				Agent: &AgentData{
					AgentID:   "agent-2",
					Hostname:  "shared.local",
					MachineID: "machine-2",
				},
				Identity: ResourceIdentity{
					MachineID: "machine-2",
					Hostnames: []string{"shared.local"},
				},
			},
		},
	})

	if got := MonitoredSystemCount(registry); got != 2 {
		t.Fatalf("MonitoredSystemCount() = %d, want 2", got)
	}
}

func TestProjectMonitoredSystemCandidateUsesCanonicalPlatformProjection(t *testing.T) {
	registry := NewRegistry(nil)
	registry.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				ID:     "host-1",
				Type:   ResourceTypeAgent,
				Name:   "tower.local",
				Status: StatusOnline,
				Agent: &AgentData{
					AgentID:   "agent-1",
					Hostname:  "tower.local",
					MachineID: "machine-1",
				},
				Identity: ResourceIdentity{
					MachineID: "machine-1",
					Hostnames: []string{"tower.local"},
				},
			},
		},
	})

	testCases := []struct {
		name      string
		candidate MonitoredSystemCandidate
	}{
		{
			name: "proxmox node overlapping agent host",
			candidate: MonitoredSystemCandidate{
				Source:   SourceProxmox,
				Type:     ResourceTypeAgent,
				Name:     "tower",
				Hostname: "tower.local",
				HostURL:  "https://tower.local:8006",
			},
		},
		{
			name: "truenas appliance overlapping agent host",
			candidate: MonitoredSystemCandidate{
				Source:   SourceTrueNAS,
				Type:     ResourceTypeAgent,
				Name:     "tower storage",
				Hostname: "tower.local",
				HostURL:  "https://tower.local",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			projection := ProjectMonitoredSystemCandidate(registry, tc.candidate)
			if projection.CurrentCount != 1 {
				t.Fatalf("CurrentCount = %d, want 1", projection.CurrentCount)
			}
			if projection.ProjectedCount != 1 {
				t.Fatalf("ProjectedCount = %d, want 1", projection.ProjectedCount)
			}
			if projection.AdditionalCount != 0 {
				t.Fatalf("AdditionalCount = %d, want 0", projection.AdditionalCount)
			}
		})
	}
}

func TestProjectMonitoredSystemCandidateReplacementPreservesOverlappingSources(t *testing.T) {
	registry := NewRegistry(nil)
	registry.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				ID:     "host-1",
				Type:   ResourceTypeAgent,
				Name:   "archive.local",
				Status: StatusOnline,
				Agent: &AgentData{
					AgentID:   "agent-1",
					Hostname:  "archive.local",
					MachineID: "machine-1",
				},
				Identity: ResourceIdentity{
					MachineID: "machine-1",
					Hostnames: []string{"archive.local"},
				},
			},
		},
	})
	registry.IngestRecords(SourceTrueNAS, []IngestRecord{
		{
			SourceID: "system:archive.local",
			Resource: Resource{
				ID:     "truenas-1",
				Type:   ResourceTypeAgent,
				Name:   "archive",
				Status: StatusOnline,
				TrueNAS: &TrueNASData{
					Hostname: "archive.local",
				},
			},
			Identity: ResourceIdentity{
				MachineID: "machine-1",
				Hostnames: []string{"archive.local"},
			},
		},
	})

	projection := ProjectMonitoredSystemCandidateReplacement(registry, MonitoredSystemReplacement{
		Source: SourceTrueNAS,
		Matches: func(resource Resource) bool {
			return resource.TrueNAS != nil && resource.TrueNAS.Hostname == "archive.local"
		},
	}, MonitoredSystemCandidate{
		Source:   SourceTrueNAS,
		Type:     ResourceTypeAgent,
		Name:     "backup",
		Hostname: "backup.local",
		HostURL:  "https://backup.local",
	})

	if projection.CurrentCount != 1 {
		t.Fatalf("CurrentCount = %d, want 1", projection.CurrentCount)
	}
	if projection.ProjectedCount != 2 {
		t.Fatalf("ProjectedCount = %d, want 2", projection.ProjectedCount)
	}
	if projection.AdditionalCount != 1 {
		t.Fatalf("AdditionalCount = %d, want 1", projection.AdditionalCount)
	}
}

func TestProjectMonitoredSystemCandidateReplacementRemovesStandaloneSource(t *testing.T) {
	registry := NewRegistry(nil)
	registry.IngestRecords(SourcePBS, []IngestRecord{
		{
			SourceID: "pbs-1",
			Resource: Resource{
				ID:     "pbs-1",
				Type:   ResourceTypePBS,
				Name:   "pbs-a",
				Status: StatusOnline,
				PBS: &PBSData{
					InstanceID: "pbs-a",
					Hostname:   "pbs-a.local",
					HostURL:    "https://pbs-a.local:8007",
				},
			},
		},
	})

	projection := ProjectMonitoredSystemCandidateReplacement(registry, MonitoredSystemReplacement{
		Source: SourcePBS,
		Matches: func(resource Resource) bool {
			return resource.PBS != nil && resource.PBS.InstanceID == "pbs-a"
		},
	}, MonitoredSystemCandidate{
		Source:   SourcePBS,
		Type:     ResourceTypePBS,
		Name:     "pbs-b",
		Hostname: "pbs-b.local",
		HostURL:  "https://pbs-b.local:8007",
	})

	if projection.CurrentCount != 1 {
		t.Fatalf("CurrentCount = %d, want 1", projection.CurrentCount)
	}
	if projection.ProjectedCount != 1 {
		t.Fatalf("ProjectedCount = %d, want 1", projection.ProjectedCount)
	}
	if projection.AdditionalCount != 0 {
		t.Fatalf("AdditionalCount = %d, want 0", projection.AdditionalCount)
	}
}
