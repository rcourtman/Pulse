package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestDetectRepeatedVMMemoryUsage(t *testing.T) {
	tests := []struct {
		name            string
		vms             []models.VM
		wantSuspicious  bool
		wantRepeated    int
		wantRunning     int
		wantRepeatedMem int64
	}{
		{
			name: "detects suspicious repeated memory used values",
			vms: []models.VM{
				{Name: "vm1", Type: "qemu", Status: "running", MemorySource: "status-mem", Memory: models.Memory{Total: 8 << 30, Used: 3 << 30}},
				{Name: "vm2", Type: "qemu", Status: "running", MemorySource: "status-mem", Memory: models.Memory{Total: 8 << 30, Used: 3 << 30}},
				{Name: "vm3", Type: "qemu", Status: "running", MemorySource: "status-mem", Memory: models.Memory{Total: 8 << 30, Used: 3 << 30}},
				{Name: "vm4", Type: "qemu", Status: "running", MemorySource: "status-mem", Memory: models.Memory{Total: 8 << 30, Used: 3 << 30}},
				{Name: "vm5", Type: "qemu", Status: "running", MemorySource: "rrd-memavailable", Memory: models.Memory{Total: 8 << 30, Used: 2 << 30}},
			},
			wantSuspicious:  true,
			wantRepeated:    4,
			wantRunning:     5,
			wantRepeatedMem: 3 << 30,
		},
		{
			name: "ignores patterns under share threshold",
			vms: []models.VM{
				{Name: "vm1", Type: "qemu", Status: "running", MemorySource: "status-mem", Memory: models.Memory{Total: 8 << 30, Used: 3 << 30}},
				{Name: "vm2", Type: "qemu", Status: "running", MemorySource: "status-mem", Memory: models.Memory{Total: 8 << 30, Used: 3 << 30}},
				{Name: "vm3", Type: "qemu", Status: "running", MemorySource: "rrd-memavailable", Memory: models.Memory{Total: 8 << 30, Used: 2 << 30}},
				{Name: "vm4", Type: "qemu", Status: "running", MemorySource: "rrd-memavailable", Memory: models.Memory{Total: 8 << 30, Used: 1 << 30}},
			},
			wantSuspicious: false,
			wantRepeated:   0,
			wantRunning:    4,
		},
		{
			name: "ignores when not enough running qemu guests",
			vms: []models.VM{
				{Name: "vm1", Type: "qemu", Status: "running", MemorySource: "status-mem", Memory: models.Memory{Total: 8 << 30, Used: 3 << 30}},
				{Name: "vm2", Type: "qemu", Status: "running", MemorySource: "status-mem", Memory: models.Memory{Total: 8 << 30, Used: 3 << 30}},
				{Name: "vm3", Type: "qemu", Status: "stopped", MemorySource: "powered-off", Memory: models.Memory{Total: 8 << 30, Used: 0}},
			},
			wantSuspicious: false,
			wantRepeated:   0,
			wantRunning:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectRepeatedVMMemoryUsage(tt.vms)
			if got.suspicious != tt.wantSuspicious {
				t.Fatalf("suspicious = %v, want %v", got.suspicious, tt.wantSuspicious)
			}
			if got.repeatedCount != tt.wantRepeated {
				t.Fatalf("repeatedCount = %d, want %d", got.repeatedCount, tt.wantRepeated)
			}
			if got.runningCount != tt.wantRunning {
				t.Fatalf("runningCount = %d, want %d", got.runningCount, tt.wantRunning)
			}
			if got.repeatedMemUsed != tt.wantRepeatedMem {
				t.Fatalf("repeatedMemUsed = %d, want %d", got.repeatedMemUsed, tt.wantRepeatedMem)
			}
		})
	}
}

func TestFilterVMsByInstance(t *testing.T) {
	vms := []models.VM{
		{ID: "a", Instance: "pve1"},
		{ID: "b", Instance: "pve2"},
		{ID: "c", Instance: "pve1"},
	}

	filtered := filterVMsByInstance(vms, "pve1")
	if len(filtered) != 2 {
		t.Fatalf("len(filtered) = %d, want 2", len(filtered))
	}
	if filtered[0].ID != "a" || filtered[1].ID != "c" {
		t.Fatalf("unexpected filtered result: %#v", filtered)
	}
}
