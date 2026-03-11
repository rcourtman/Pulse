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
