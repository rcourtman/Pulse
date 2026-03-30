package unifiedresources

import "testing"

func TestBuildMetricsTarget_UsesCanonicalAgentTypeForInfrastructureFamilies(t *testing.T) {
	tests := []struct {
		name          string
		resource      Resource
		sourceTargets []SourceTarget
		wantID        string
	}{
		{
			name: "proxmox infrastructure node",
			resource: Resource{
				Type: ResourceTypeAgent,
			},
			sourceTargets: []SourceTarget{{
				Source:   SourceProxmox,
				SourceID: "pve-node-1",
			}},
			wantID: "pve-node-1",
		},
		{
			name: "vmware host",
			resource: Resource{
				Type: ResourceTypeAgent,
			},
			sourceTargets: []SourceTarget{{
				Source:   SourceVMware,
				SourceID: "vc-1:host:host-101",
			}},
			wantID: "vc-1:host:host-101",
		},
		{
			name: "pbs instance",
			resource: Resource{
				Type: ResourceTypePBS,
			},
			sourceTargets: []SourceTarget{{
				Source:   SourcePBS,
				SourceID: "pbs-1",
			}},
			wantID: "pbs-1",
		},
		{
			name: "pmg instance",
			resource: Resource{
				Type: ResourceTypePMG,
			},
			sourceTargets: []SourceTarget{{
				Source:   SourcePMG,
				SourceID: "pmg-1",
			}},
			wantID: "pmg-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := BuildMetricsTarget(tt.resource, tt.sourceTargets)
			if target == nil {
				t.Fatal("BuildMetricsTarget() returned nil")
			}
			if target.ResourceType != "agent" {
				t.Fatalf("ResourceType = %q, want agent", target.ResourceType)
			}
			if target.ResourceID != tt.wantID {
				t.Fatalf("ResourceID = %q, want %q", target.ResourceID, tt.wantID)
			}
		})
	}
}

func TestBuildMetricsTarget_CanonicalizesSourceIDWhitespace(t *testing.T) {
	tests := []struct {
		name          string
		resource      Resource
		sourceTargets []SourceTarget
		wantType      string
		wantID        string
	}{
		{
			name: "agent target",
			resource: Resource{
				Type: ResourceTypeAgent,
			},
			sourceTargets: []SourceTarget{{
				Source:   SourceAgent,
				SourceID: " host-1 ",
			}},
			wantType: "agent",
			wantID:   "host-1",
		},
		{
			name: "docker host target",
			resource: Resource{
				Type: ResourceTypeAgent,
			},
			sourceTargets: []SourceTarget{{
				Source:   SourceDocker,
				SourceID: " docker-runtime-1 ",
			}},
			wantType: "docker-host",
			wantID:   "docker-runtime-1",
		},
		{
			name: "kubernetes pod target",
			resource: Resource{
				Type: ResourceTypePod,
			},
			sourceTargets: []SourceTarget{{
				Source:   SourceK8s,
				SourceID: " ns/pod-1 ",
			}},
			wantType: string(ResourceTypePod),
			wantID:   "ns/pod-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := BuildMetricsTarget(tt.resource, tt.sourceTargets)
			if target == nil {
				t.Fatal("BuildMetricsTarget() returned nil")
			}
			if target.ResourceType != tt.wantType {
				t.Fatalf("ResourceType = %q, want %q", target.ResourceType, tt.wantType)
			}
			if target.ResourceID != tt.wantID {
				t.Fatalf("ResourceID = %q, want %q", target.ResourceID, tt.wantID)
			}
		})
	}
}

func TestBuildMetricsTarget_UsesCanonicalTargetsForVMwareWorkloadAndStorage(t *testing.T) {
	tests := []struct {
		name          string
		resource      Resource
		sourceTargets []SourceTarget
		wantType      string
		wantID        string
	}{
		{
			name: "vmware vm",
			resource: Resource{
				Type: ResourceTypeVM,
			},
			sourceTargets: []SourceTarget{{
				Source:   SourceVMware,
				SourceID: "vc-1:vm:vm-201",
			}},
			wantType: "vm",
			wantID:   "vc-1:vm:vm-201",
		},
		{
			name: "vmware datastore",
			resource: Resource{
				Type: ResourceTypeStorage,
			},
			sourceTargets: []SourceTarget{{
				Source:   SourceVMware,
				SourceID: "vc-1:datastore:datastore-11",
			}},
			wantType: "storage",
			wantID:   "vc-1:datastore:datastore-11",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := BuildMetricsTarget(tt.resource, tt.sourceTargets)
			if target == nil {
				t.Fatal("BuildMetricsTarget() returned nil")
			}
			if target.ResourceType != tt.wantType {
				t.Fatalf("ResourceType = %q, want %q", target.ResourceType, tt.wantType)
			}
			if target.ResourceID != tt.wantID {
				t.Fatalf("ResourceID = %q, want %q", target.ResourceID, tt.wantID)
			}
		})
	}
}

func TestBuildMetricsTarget_RejectsEmptyCanonicalSourceID(t *testing.T) {
	tests := []struct {
		name          string
		resource      Resource
		sourceTargets []SourceTarget
	}{
		{
			name: "agent target with whitespace-only source id",
			resource: Resource{
				Type: ResourceTypeAgent,
			},
			sourceTargets: []SourceTarget{{
				Source:   SourceAgent,
				SourceID: "   ",
			}},
		},
		{
			name: "kubernetes pod target with empty source id",
			resource: Resource{
				Type: ResourceTypePod,
			},
			sourceTargets: []SourceTarget{{
				Source:   SourceK8s,
				SourceID: "",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := BuildMetricsTarget(tt.resource, tt.sourceTargets)
			if target != nil {
				t.Fatalf("expected nil metrics target for empty canonical source id, got %+v", target)
			}
		})
	}
}

func TestBuildMetricsTarget_UsesCanonicalPhysicalDiskMetricIDForTrueNAS(t *testing.T) {
	target := BuildMetricsTarget(
		Resource{
			Type: ResourceTypePhysicalDisk,
			PhysicalDisk: &PhysicalDiskMeta{
				Serial: "SER-TRUE-1",
			},
		},
		[]SourceTarget{{
			Source:   SourceTrueNAS,
			SourceID: "disk:sda",
		}},
	)
	if target == nil {
		t.Fatal("BuildMetricsTarget() returned nil")
	}
	if target.ResourceType != "disk" {
		t.Fatalf("ResourceType = %q, want disk", target.ResourceType)
	}
	if target.ResourceID != "SER-TRUE-1" {
		t.Fatalf("ResourceID = %q, want SER-TRUE-1", target.ResourceID)
	}
}

func TestBuildMetricsTarget_UsesCanonicalAppMetricIDForTrueNAS(t *testing.T) {
	target := BuildMetricsTarget(
		Resource{
			Type: ResourceTypeAppContainer,
		},
		[]SourceTarget{{
			Source:   SourceTrueNAS,
			SourceID: " app:nextcloud ",
		}},
	)
	if target == nil {
		t.Fatal("BuildMetricsTarget() returned nil")
	}
	if target.ResourceType != "app-container" {
		t.Fatalf("ResourceType = %q, want app-container", target.ResourceType)
	}
	if target.ResourceID != "nextcloud" {
		t.Fatalf("ResourceID = %q, want nextcloud", target.ResourceID)
	}
}

func TestBuildMetricsTarget_UsesCanonicalAgentMetricIDForTrueNAS(t *testing.T) {
	target := BuildMetricsTarget(
		Resource{
			Type: ResourceTypeAgent,
		},
		[]SourceTarget{{
			Source:   SourceTrueNAS,
			SourceID: " system:truenas-main ",
		}},
	)
	if target == nil {
		t.Fatal("BuildMetricsTarget() returned nil")
	}
	if target.ResourceType != "agent" {
		t.Fatalf("ResourceType = %q, want agent", target.ResourceType)
	}
	if target.ResourceID != "truenas-main" {
		t.Fatalf("ResourceID = %q, want truenas-main", target.ResourceID)
	}
}
