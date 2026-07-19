package unifiedresources

import (
	"reflect"
	"testing"
)

// These tests augment views_test.go with branch-coverage assertions for the
// misc-view accessors and String() formatters requested for the 0719late
// coverage pass. They reuse the package's existing testResource helper and
// table-driven style, and they intentionally never mutate source under test.

func TestContainerViewPool_MiscViewsBranchcov0719late(t *testing.T) {
	// Arm: nil receiver must return "" (v.r == nil branch).
	var zero ContainerView
	if got := zero.Pool(); got != "" {
		t.Fatalf("nil receiver Pool: expected %q, got %q", "", got)
	}

	// Arm: resource present but Proxmox payload nil must return "" (v.r.Proxmox == nil branch).
	rNoProx := testResource(ResourceTypeSystemContainer)
	rNoProx.Proxmox = nil
	if got := NewContainerView(rNoProx).Pool(); got != "" {
		t.Fatalf("nil Proxmox Pool: expected %q, got %q", "", got)
	}

	// Arm: populated exercises strings.TrimSpace(" production ").
	rPop := testResource(ResourceTypeSystemContainer)
	rPop.Proxmox = &ProxmoxData{Pool: "  production  "}
	if got, want := NewContainerView(rPop).Pool(), "production"; got != want {
		t.Fatalf("populated Pool: expected %q, got %q", want, got)
	}
}

func TestNodeViewIsClusterMember_MiscViewsBranchcov0719late(t *testing.T) {
	// Arm: nil receiver must return false (v.r == nil branch).
	var zero NodeView
	if zero.IsClusterMember() {
		t.Fatalf("nil receiver IsClusterMember: expected %v, got %v", false, true)
	}

	// Arm: resource present but Proxmox nil must return false (v.r.Proxmox == nil branch).
	rNoProx := testResource(ResourceTypeAgent)
	rNoProx.Proxmox = nil
	if NewNodeView(rNoProx).IsClusterMember() {
		t.Fatalf("nil Proxmox IsClusterMember: expected %v, got %v", false, true)
	}

	// Arm: populated true projects the underlying flag.
	rTrue := testResource(ResourceTypeAgent)
	rTrue.Proxmox = &ProxmoxData{IsClusterMember: true}
	if !NewNodeView(rTrue).IsClusterMember() {
		t.Fatalf("populated IsClusterMember=true: expected %v, got %v", true, false)
	}

	// Arm: populated false projects the underlying flag (explicit boundary).
	rFalse := testResource(ResourceTypeAgent)
	rFalse.Proxmox = &ProxmoxData{IsClusterMember: false}
	if NewNodeView(rFalse).IsClusterMember() {
		t.Fatalf("populated IsClusterMember=false: expected %v, got %v", false, true)
	}
}

func TestPhysicalDiskViewMetricResourceID_MiscViewsBranchcov0719late(t *testing.T) {
	// Arm: nil receiver must return "" (v.r == nil branch).
	var zero PhysicalDiskView
	if got := zero.MetricResourceID(); got != "" {
		t.Fatalf("nil receiver MetricResourceID: expected %q, got %q", "", got)
	}

	// Arm: MetricsTarget non-nil wins, even when PhysicalDisk would otherwise resolve.
	rTarget := testResource(ResourceTypePhysicalDisk)
	rTarget.MetricsTarget = &MetricsTarget{ResourceID: "vc-1:disk:serial-abc"}
	rTarget.PhysicalDisk = &PhysicalDiskMeta{Serial: "should-not-win"}
	if got, want := NewPhysicalDiskView(rTarget).MetricResourceID(), "vc-1:disk:serial-abc"; got != want {
		t.Fatalf("MetricsTarget arm: expected %q, got %q", want, got)
	}

	// Arm: MetricsTarget nil and PhysicalDisk has Serial → PhysicalDiskMetaMetricID returns trimmed serial.
	rSerial := testResource(ResourceTypePhysicalDisk)
	rSerial.PhysicalDisk = &PhysicalDiskMeta{Serial: "  SER123  "}
	if got, want := NewPhysicalDiskView(rSerial).MetricResourceID(), "SER123"; got != want {
		t.Fatalf("PhysicalDiskMetaMetricID serial fallback arm: expected %q, got %q", want, got)
	}

	// Arm: MetricsTarget nil and PhysicalDisk nil → fallback to trimmed resource ID.
	rFallback := testResource(ResourceTypePhysicalDisk)
	rFallback.ID = "  disk-id-1  "
	rFallback.PhysicalDisk = nil
	if got, want := NewPhysicalDiskView(rFallback).MetricResourceID(), "disk-id-1"; got != want {
		t.Fatalf("PhysicalDiskMetaMetricID nil-disk fallback arm: expected %q, got %q", want, got)
	}
}

func TestPBSInstanceViewDatastores_MiscViewsBranchcov0719late(t *testing.T) {
	// Arm: nil receiver must return nil (v.r == nil branch).
	var zero PBSInstanceView
	if got := zero.Datastores(); got != nil {
		t.Fatalf("nil receiver Datastores: expected nil, got %+v", got)
	}

	// Arm: resource present but PBS nil must return nil (v.r.PBS == nil branch).
	rNoPBS := testResource(ResourceTypePBS)
	rNoPBS.PBS = nil
	if got := NewPBSInstanceView(rNoPBS).Datastores(); got != nil {
		t.Fatalf("nil PBS Datastores: expected nil, got %+v", got)
	}

	// Arm: PBS present but Datastores slice itself nil must still return nil.
	rNilSlice := testResource(ResourceTypePBS)
	rNilSlice.PBS = &PBSData{}
	if got := NewPBSInstanceView(rNilSlice).Datastores(); got != nil {
		t.Fatalf("nil Datastores slice: expected nil, got %+v", got)
	}

	// Arm: populated multiple datastores project correctly AND the clone is independent.
	want := []PBSDatastoreMeta{
		{Name: "fast", Total: 100, Used: 96, Available: 4, UsagePercent: 96, Status: "online", DeduplicationFactor: 1.6},
		{Name: "archive", Total: 1000, Used: 500, Available: 500, UsagePercent: 50, Status: "online"},
	}
	rPop := testResource(ResourceTypePBS)
	rPop.PBS = &PBSData{Datastores: want}

	got := NewPBSInstanceView(rPop).Datastores()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("populated Datastores: expected %+v, got %+v", want, got)
	}

	// Mutating the returned slice (and its elements) must not affect the backing resource.
	got[0].Name = "mutated"
	got[0].Total = 9999
	again := NewPBSInstanceView(rPop).Datastores()
	if !reflect.DeepEqual(again, want) {
		t.Fatalf("Datastores independence: expected clone to remain %+v, got %+v", want, again)
	}
}

func TestPMGInstanceViewInstanceID_MiscViewsBranchcov0719late(t *testing.T) {
	// Arm: nil receiver must return "" (v.r == nil branch).
	var zero PMGInstanceView
	if got := zero.InstanceID(); got != "" {
		t.Fatalf("nil receiver InstanceID: expected %q, got %q", "", got)
	}

	// Arm: resource present but PMG nil must return "" (v.r.PMG == nil branch).
	rNoPMG := testResource(ResourceTypePMG)
	rNoPMG.PMG = nil
	if got := NewPMGInstanceView(rNoPMG).InstanceID(); got != "" {
		t.Fatalf("nil PMG InstanceID: expected %q, got %q", "", got)
	}

	// Arm: populated projects the underlying InstanceID verbatim.
	rPop := testResource(ResourceTypePMG)
	rPop.PMG = &PMGData{InstanceID: "pmg-instance-1"}
	if got, want := NewPMGInstanceView(rPop).InstanceID(), "pmg-instance-1"; got != want {
		t.Fatalf("populated InstanceID: expected %q, got %q", want, got)
	}
}

func TestViewsStringMethods_MiscViewsBranchcov0719late(t *testing.T) {
	// Populated arm: each formatter must compose ID()/Name() as `<Type>(<id>, "<name>")`.
	populatedCases := []struct {
		label string
		got   string
		want  string
	}{
		{"VMView", NewVMView(&Resource{ID: "vm-1", Type: ResourceTypeVM, Name: "app-vm"}).String(), `VMView(vm-1, "app-vm")`},
		{"ContainerView", NewContainerView(&Resource{ID: "ct-1", Type: ResourceTypeSystemContainer, Name: "db-ct"}).String(), `ContainerView(ct-1, "db-ct")`},
		{"NodeView", NewNodeView(&Resource{ID: "node-1", Type: ResourceTypeAgent, Name: "pve-node-1"}).String(), `NodeView(node-1, "pve-node-1")`},
		{"StoragePoolView", NewStoragePoolView(&Resource{ID: "storage-1", Type: ResourceTypeStorage, Name: "local-zfs"}).String(), `StoragePoolView(storage-1, "local-zfs")`},
		{"PhysicalDiskView", NewPhysicalDiskView(&Resource{ID: "disk-1", Type: ResourceTypePhysicalDisk, Name: "Samsung 990 PRO"}).String(), `PhysicalDiskView(disk-1, "Samsung 990 PRO")`},
		{"PBSInstanceView", NewPBSInstanceView(&Resource{ID: "pbs-1", Type: ResourceTypePBS, Name: "pbs-a"}).String(), `PBSInstanceView(pbs-1, "pbs-a")`},
		{"PMGInstanceView", NewPMGInstanceView(&Resource{ID: "pmg-1", Type: ResourceTypePMG, Name: "pmg-a"}).String(), `PMGInstanceView(pmg-1, "pmg-a")`},
		{"WorkloadView", NewWorkloadView(&Resource{ID: "vm-2", Type: ResourceTypeVM, Name: "web-vm"}).String(), `WorkloadView(vm-2, "web-vm")`},
		{"InfrastructureView", NewInfrastructureView(&Resource{ID: "host-1", Type: ResourceTypeAgent, Name: "agent-host-1"}).String(), `InfrastructureView(host-1, "agent-host-1")`},
	}
	for _, c := range populatedCases {
		if c.got != c.want {
			t.Errorf("%s populated String(): expected %q, got %q", c.label, c.want, c.got)
		}
	}

	// Nil-receiver arm: every String() must not panic and must render the empty-id/empty-name form.
	nilCases := []struct {
		label string
		got   string
		want  string
	}{
		{"VMView nil", VMView{}.String(), `VMView(, "")`},
		{"ContainerView nil", ContainerView{}.String(), `ContainerView(, "")`},
		{"NodeView nil", NodeView{}.String(), `NodeView(, "")`},
		{"StoragePoolView nil", StoragePoolView{}.String(), `StoragePoolView(, "")`},
		{"PhysicalDiskView nil", PhysicalDiskView{}.String(), `PhysicalDiskView(, "")`},
		{"PBSInstanceView nil", PBSInstanceView{}.String(), `PBSInstanceView(, "")`},
		{"PMGInstanceView nil", PMGInstanceView{}.String(), `PMGInstanceView(, "")`},
		{"WorkloadView nil", WorkloadView{}.String(), `WorkloadView(, "")`},
		{"InfrastructureView nil", InfrastructureView{}.String(), `InfrastructureView(, "")`},
	}
	for _, c := range nilCases {
		if c.got != c.want {
			t.Errorf("%s String(): expected %q, got %q", c.label, c.want, c.got)
		}
	}
}
