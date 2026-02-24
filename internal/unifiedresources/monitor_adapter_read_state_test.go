package unifiedresources

import "testing"

var _ ReadState = (*MonitorAdapter)(nil)

func TestMonitorAdapterReadStateForwardsToRegistry(t *testing.T) {
	adapter := NewMonitorAdapter(NewRegistry(nil))
	nodeID := "node-1"

	adapter.PopulateSupplementalRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: "node-1",
			Resource: Resource{
				ID:     nodeID,
				Type:   ResourceTypeHost,
				Name:   "node-1",
				Status: StatusOnline,
				Proxmox: &ProxmoxData{
					NodeName: "node-1",
				},
			},
		},
		{
			SourceID: "vm-101",
			Resource: Resource{
				ID:       "vm-101",
				Type:     ResourceTypeVM,
				Name:     "vm-101",
				Status:   StatusOnline,
				ParentID: &nodeID,
				Proxmox: &ProxmoxData{
					VMID:     101,
					NodeName: "node-1",
				},
			},
		},
		{
			SourceID: "storage-local",
			Resource: Resource{
				ID:       "storage-local",
				Type:     ResourceTypeStorage,
				Name:     "local",
				Status:   StatusOnline,
				ParentID: &nodeID,
				Storage: &StorageMeta{
					Type: "dir",
				},
			},
		},
	})

	if got := len(adapter.Nodes()); got != 1 {
		t.Fatalf("expected 1 node view, got %d", got)
	}
	if got := len(adapter.VMs()); got != 1 {
		t.Fatalf("expected 1 VM view, got %d", got)
	}
	if got := len(adapter.StoragePools()); got != 1 {
		t.Fatalf("expected 1 storage pool view, got %d", got)
	}
	if got := len(adapter.Workloads()); got == 0 {
		t.Fatal("expected workload views from registry-backed adapter")
	}
	if got := len(adapter.Infrastructure()); got == 0 {
		t.Fatal("expected infrastructure views from registry-backed adapter")
	}
}
