package reporting

import (
	"strings"
	"testing"
)

func TestDescribeVMInventoryExport_DefinesCanonicalColumns(t *testing.T) {
	definition := DescribeVMInventoryExport()

	if definition.ID != "vm_inventory" {
		t.Fatalf("definition ID = %q, want vm_inventory", definition.ID)
	}
	if definition.Format != FormatCSV {
		t.Fatalf("definition format = %q, want %q", definition.Format, FormatCSV)
	}
	if got := len(definition.Columns); got != 12 {
		t.Fatalf("definition columns = %d, want 12", got)
	}
	if definition.Columns[3].Key != "pool" || definition.Columns[3].Label != "Pool" {
		t.Fatalf("expected canonical Pool column at index 3, got %+v", definition.Columns[3])
	}
}

func TestGenerateVMInventoryCSV_SortsRowsAndWritesHeader(t *testing.T) {
	data := VMInventoryData{
		Rows: []VMInventoryRow{
			{
				ResourceID:           "vm-20",
				Instance:             "lab-b",
				Node:                 "node-2",
				Pool:                 "pool-b",
				VMID:                 20,
				Name:                 "beta",
				Status:               "online",
				CPUCores:             4,
				MemoryAllocatedBytes: 8192,
				DiskAllocatedBytes:   20000,
				DiskUsedBytes:        15000,
			},
			{
				ResourceID:           "vm-10",
				Instance:             "lab-a",
				Node:                 "node-1",
				Pool:                 "pool-a",
				VMID:                 10,
				Name:                 "alpha",
				Status:               "warning",
				CPUCores:             2,
				MemoryAllocatedBytes: 4096,
				DiskAllocatedBytes:   10000,
				DiskUsedBytes:        5000,
				DiskStatusReason:     "guest agent offline",
			},
		},
	}

	got, err := GenerateVMInventoryCSV(data)
	if err != nil {
		t.Fatalf("GenerateVMInventoryCSV returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected header plus two rows, got %d lines: %q", len(lines), string(got))
	}
	if !strings.Contains(lines[0], "Resource ID,Instance,Node,Pool,VMID,VM Name") {
		t.Fatalf("expected spreadsheet-friendly header row, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "vm-10,lab-a,node-1,pool-a,10,alpha,warning,2,4096,10000,5000,guest agent offline") {
		t.Fatalf("expected rows sorted by instance/node/name, got first data row %q", lines[1])
	}
	if !strings.Contains(lines[2], "vm-20,lab-b,node-2,pool-b,20,beta,online,4,8192,20000,15000,") {
		t.Fatalf("expected second data row for vm-20, got %q", lines[2])
	}
}
