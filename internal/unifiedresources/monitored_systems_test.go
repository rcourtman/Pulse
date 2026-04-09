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
		Selector: MonitoredSystemReplacementSelector{
			Hostname: "archive.local",
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
		Selector: MonitoredSystemReplacementSelector{
			ResourceID: "pbs-a",
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

func TestPreviewMonitoredSystemCandidateReturnsCurrentAndProjectedSystems(t *testing.T) {
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

	preview := PreviewMonitoredSystemCandidate(registry, MonitoredSystemCandidate{
		Source:   SourceProxmox,
		Type:     ResourceTypeAgent,
		Name:     "tower",
		Hostname: "tower.local",
		HostURL:  "https://tower.local:8006",
	})

	if preview.CurrentCount != 1 {
		t.Fatalf("CurrentCount = %d, want 1", preview.CurrentCount)
	}
	if preview.ProjectedCount != 1 {
		t.Fatalf("ProjectedCount = %d, want 1", preview.ProjectedCount)
	}
	if preview.AdditionalCount != 0 {
		t.Fatalf("AdditionalCount = %d, want 0", preview.AdditionalCount)
	}
	if preview.CurrentSystem == nil {
		t.Fatal("expected current system preview")
	}
	if preview.ProjectedSystem == nil {
		t.Fatal("expected projected system preview")
	}
	if len(preview.CurrentSystems) != 1 {
		t.Fatalf("len(CurrentSystems) = %d, want 1", len(preview.CurrentSystems))
	}
	if len(preview.ProjectedSystems) != 1 {
		t.Fatalf("len(ProjectedSystems) = %d, want 1", len(preview.ProjectedSystems))
	}
	if preview.CurrentSystem.Source != "agent" {
		t.Fatalf("CurrentSystem.Source = %q, want agent", preview.CurrentSystem.Source)
	}
	if preview.ProjectedSystem.Source != "multiple" {
		t.Fatalf("ProjectedSystem.Source = %q, want multiple", preview.ProjectedSystem.Source)
	}
	if len(preview.ProjectedSystem.Explanation.Surfaces) != 2 {
		t.Fatalf("expected projected system to include 2 grouped surfaces, got %+v", preview.ProjectedSystem.Explanation.Surfaces)
	}
}

func TestPreviewMonitoredSystemCandidateInactiveKeepsCountUnchanged(t *testing.T) {
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

	preview := PreviewMonitoredSystemCandidate(registry, MonitoredSystemCandidate{
		Source:   SourceTrueNAS,
		Type:     ResourceTypeAgent,
		Name:     "tower storage",
		Hostname: "tower.local",
		HostURL:  "https://tower.local",
		State:    MonitoredSystemCandidateStateInactive,
	})

	if preview.CurrentCount != 1 {
		t.Fatalf("CurrentCount = %d, want 1", preview.CurrentCount)
	}
	if preview.ProjectedCount != 1 {
		t.Fatalf("ProjectedCount = %d, want 1", preview.ProjectedCount)
	}
	if preview.AdditionalCount != 0 {
		t.Fatalf("AdditionalCount = %d, want 0", preview.AdditionalCount)
	}
	if preview.CurrentSystem != nil {
		t.Fatalf("CurrentSystem = %+v, want nil", preview.CurrentSystem)
	}
	if preview.ProjectedSystem != nil {
		t.Fatalf("ProjectedSystem = %+v, want nil", preview.ProjectedSystem)
	}
	if len(preview.CurrentSystems) != 0 {
		t.Fatalf("len(CurrentSystems) = %d, want 0", len(preview.CurrentSystems))
	}
	if len(preview.ProjectedSystems) != 0 {
		t.Fatalf("len(ProjectedSystems) = %d, want 0", len(preview.ProjectedSystems))
	}
}

func TestPreviewMonitoredSystemCandidateReplacementReturnsAffectedSystems(t *testing.T) {
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

	preview := PreviewMonitoredSystemCandidateReplacement(registry, MonitoredSystemReplacement{
		Source: SourceTrueNAS,
		Selector: MonitoredSystemReplacementSelector{
			Hostname: "archive.local",
		},
	}, MonitoredSystemCandidate{
		Source:   SourceTrueNAS,
		Type:     ResourceTypeAgent,
		Name:     "backup",
		Hostname: "backup.local",
		HostURL:  "https://backup.local",
	})

	if preview.CurrentCount != 1 {
		t.Fatalf("CurrentCount = %d, want 1", preview.CurrentCount)
	}
	if preview.ProjectedCount != 2 {
		t.Fatalf("ProjectedCount = %d, want 2", preview.ProjectedCount)
	}
	if preview.AdditionalCount != 1 {
		t.Fatalf("AdditionalCount = %d, want 1", preview.AdditionalCount)
	}
	if preview.CurrentSystem == nil {
		t.Fatal("expected current system preview")
	}
	if preview.ProjectedSystem == nil {
		t.Fatal("expected projected system preview")
	}
	if len(preview.CurrentSystems) != 1 {
		t.Fatalf("len(CurrentSystems) = %d, want 1", len(preview.CurrentSystems))
	}
	if len(preview.ProjectedSystems) != 1 {
		t.Fatalf("len(ProjectedSystems) = %d, want 1", len(preview.ProjectedSystems))
	}
	if preview.CurrentSystem.Source != "multiple" {
		t.Fatalf("CurrentSystem.Source = %q, want multiple", preview.CurrentSystem.Source)
	}
	if preview.ProjectedSystem.Source != "truenas" {
		t.Fatalf("ProjectedSystem.Source = %q, want truenas", preview.ProjectedSystem.Source)
	}
	if preview.ProjectedSystem.Name != "backup" {
		t.Fatalf("ProjectedSystem.Name = %q, want backup", preview.ProjectedSystem.Name)
	}
}

func TestPreviewMonitoredSystemCandidateReplacementInactiveRemovesVMwareConnection(t *testing.T) {
	registry := NewRegistry(nil)
	registry.IngestRecords(SourceVMware, []IngestRecord{
		{
			SourceID: "vc-1:host:host-101",
			Resource: Resource{
				ID:     "vmware-host-101",
				Type:   ResourceTypeAgent,
				Name:   "esxi-01.lab.local",
				Status: StatusOnline,
				VMware: &VMwareData{
					ConnectionID:    "vc-1",
					ConnectionName:  "Lab VC",
					VCenterHost:     "vcsa.lab.local",
					ManagedObjectID: "host-101",
					EntityType:      "host",
					HostUUID:        "host-uuid-101",
				},
			},
			Identity: ResourceIdentity{
				DMIUUID:   "host-uuid-101",
				Hostnames: []string{"esxi-01.lab.local"},
			},
		},
	})

	preview := PreviewMonitoredSystemCandidateReplacement(registry, MonitoredSystemReplacement{
		Source: SourceVMware,
		Selector: MonitoredSystemReplacementSelector{
			ResourceID: "vc-1",
		},
	}, MonitoredSystemCandidate{
		Source:   SourceVMware,
		Type:     ResourceTypeAgent,
		Name:     "Lab VC",
		Hostname: "vcsa.lab.local",
		HostURL:  "vcsa.lab.local",
		State:    MonitoredSystemCandidateStateInactive,
	})

	if preview.CurrentCount != 1 {
		t.Fatalf("CurrentCount = %d, want 1", preview.CurrentCount)
	}
	if preview.ProjectedCount != 0 {
		t.Fatalf("ProjectedCount = %d, want 0", preview.ProjectedCount)
	}
	if preview.AdditionalCount != 0 {
		t.Fatalf("AdditionalCount = %d, want 0", preview.AdditionalCount)
	}
	if preview.CurrentSystem == nil {
		t.Fatal("expected current system preview")
	}
	if preview.ProjectedSystem != nil {
		t.Fatalf("ProjectedSystem = %+v, want nil", preview.ProjectedSystem)
	}
	if len(preview.CurrentSystems) != 1 {
		t.Fatalf("len(CurrentSystems) = %d, want 1", len(preview.CurrentSystems))
	}
	if len(preview.ProjectedSystems) != 0 {
		t.Fatalf("len(ProjectedSystems) = %d, want 0", len(preview.ProjectedSystems))
	}
	if preview.CurrentSystem.Source != "vmware" {
		t.Fatalf("CurrentSystem.Source = %q, want vmware", preview.CurrentSystem.Source)
	}
}

func TestPreviewMonitoredSystemRecordsReturnsAffectedSystems(t *testing.T) {
	registry := NewRegistry(nil)
	registry.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "agent-host-1",
			Resource: Resource{
				Type:   ResourceTypeAgent,
				Name:   "esxi-01.lab.local",
				Status: StatusOnline,
			},
			Identity: ResourceIdentity{
				DMIUUID:   "uuid-host-1",
				Hostnames: []string{"esxi-01.lab.local"},
			},
		},
	})

	preview := PreviewMonitoredSystemRecords(registry, map[DataSource][]IngestRecord{
		SourceVMware: {
			{
				SourceID: "vc-1:host:host-101",
				Resource: Resource{
					Type:   ResourceTypeAgent,
					Name:   "esxi-01.lab.local",
					Status: StatusOnline,
					VMware: &VMwareData{
						ConnectionID:    "vc-1",
						ConnectionName:  "Lab VC",
						ManagedObjectID: "host-101",
						EntityType:      "host",
						HostUUID:        "uuid-host-1",
					},
				},
				Identity: ResourceIdentity{
					DMIUUID:   "uuid-host-1",
					Hostnames: []string{"esxi-01.lab.local"},
				},
			},
			{
				SourceID: "vc-1:host:host-102",
				Resource: Resource{
					Type:   ResourceTypeAgent,
					Name:   "esxi-02.lab.local",
					Status: StatusOnline,
					VMware: &VMwareData{
						ConnectionID:    "vc-1",
						ConnectionName:  "Lab VC",
						ManagedObjectID: "host-102",
						EntityType:      "host",
						HostUUID:        "uuid-host-2",
					},
				},
				Identity: ResourceIdentity{
					DMIUUID:   "uuid-host-2",
					Hostnames: []string{"esxi-02.lab.local"},
				},
			},
		},
	})

	if preview.CurrentCount != 1 {
		t.Fatalf("CurrentCount = %d, want 1", preview.CurrentCount)
	}
	if preview.ProjectedCount != 2 {
		t.Fatalf("ProjectedCount = %d, want 2", preview.ProjectedCount)
	}
	if preview.AdditionalCount != 1 {
		t.Fatalf("AdditionalCount = %d, want 1", preview.AdditionalCount)
	}
	if len(preview.CurrentSystems) != 1 {
		t.Fatalf("len(CurrentSystems) = %d, want 1", len(preview.CurrentSystems))
	}
	if len(preview.ProjectedSystems) != 2 {
		t.Fatalf("len(ProjectedSystems) = %d, want 2", len(preview.ProjectedSystems))
	}
	if preview.CurrentSystems[0].Source != "agent" {
		t.Fatalf("CurrentSystems[0].Source = %q, want agent", preview.CurrentSystems[0].Source)
	}
}
