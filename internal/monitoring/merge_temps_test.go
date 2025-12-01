package monitoring

import (
	"math"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestMergeNVMeTempsIntoDisks(t *testing.T) {
	tests := []struct {
		name     string
		disks    []models.PhysicalDisk
		nodes    []models.Node
		expected []models.PhysicalDisk
	}{
		{
			name:     "empty disks returns empty",
			disks:    []models.PhysicalDisk{},
			nodes:    []models.Node{{Name: "node1", Temperature: &models.Temperature{Available: true}}},
			expected: []models.PhysicalDisk{},
		},
		{
			name:     "empty nodes returns disks unchanged",
			disks:    []models.PhysicalDisk{{Node: "node1", DevPath: "/dev/sda", Temperature: 0}},
			nodes:    []models.Node{},
			expected: []models.PhysicalDisk{{Node: "node1", DevPath: "/dev/sda", Temperature: 0}},
		},
		{
			name:  "nil temperature returns disks unchanged",
			disks: []models.PhysicalDisk{{Node: "node1", DevPath: "/dev/sda", Temperature: 0}},
			nodes: []models.Node{{Name: "node1", Temperature: nil}},
			expected: []models.PhysicalDisk{{Node: "node1", DevPath: "/dev/sda", Temperature: 0}},
		},
		{
			name:  "temperature available false returns disks unchanged",
			disks: []models.PhysicalDisk{{Node: "node1", DevPath: "/dev/sda", Temperature: 0}},
			nodes: []models.Node{{Name: "node1", Temperature: &models.Temperature{Available: false}}},
			expected: []models.PhysicalDisk{{Node: "node1", DevPath: "/dev/sda", Temperature: 0}},
		},
		{
			name:  "no SMART or NVMe temps returns disks unchanged",
			disks: []models.PhysicalDisk{{Node: "node1", DevPath: "/dev/sda", Temperature: 0}},
			nodes: []models.Node{{Name: "node1", Temperature: &models.Temperature{Available: true, SMART: nil, NVMe: nil}}},
			expected: []models.PhysicalDisk{{Node: "node1", DevPath: "/dev/sda", Temperature: 0}},
		},
		{
			name: "SMART temperature matched by WWN",
			disks: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/sda", WWN: "5000c5001234abcd", Temperature: 0},
			},
			nodes: []models.Node{
				{
					Name: "node1",
					Temperature: &models.Temperature{
						Available: true,
						SMART: []models.DiskTemp{
							{Device: "/dev/sda", WWN: "5000C5001234ABCD", Temperature: 42},
						},
					},
				},
			},
			expected: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/sda", WWN: "5000c5001234abcd", Temperature: 42},
			},
		},
		{
			name: "SMART temperature matched by serial case insensitive",
			disks: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/sdb", Serial: "WD-ABC123", Temperature: 0},
			},
			nodes: []models.Node{
				{
					Name: "node1",
					Temperature: &models.Temperature{
						Available: true,
						SMART: []models.DiskTemp{
							{Device: "/dev/sdb", Serial: "wd-abc123", Temperature: 38},
						},
					},
				},
			},
			expected: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/sdb", Serial: "WD-ABC123", Temperature: 38},
			},
		},
		{
			name: "SMART temperature matched by device path",
			disks: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/sdc", Temperature: 0},
			},
			nodes: []models.Node{
				{
					Name: "node1",
					Temperature: &models.Temperature{
						Available: true,
						SMART: []models.DiskTemp{
							{Device: "/dev/sdc", Temperature: 35},
						},
					},
				},
			},
			expected: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/sdc", Temperature: 35},
			},
		},
		{
			name: "NVMe legacy fallback when no SMART match",
			disks: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/nvme0n1", Type: "nvme", Temperature: 0},
			},
			nodes: []models.Node{
				{
					Name: "node1",
					Temperature: &models.Temperature{
						Available: true,
						NVMe: []models.NVMeTemp{
							{Device: "nvme0", Temp: 45.5},
						},
					},
				},
			},
			expected: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/nvme0n1", Type: "nvme", Temperature: 46},
			},
		},
		{
			name: "temperature zero is not applied",
			disks: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/sda", WWN: "5000c500", Temperature: 0},
			},
			nodes: []models.Node{
				{
					Name: "node1",
					Temperature: &models.Temperature{
						Available: true,
						SMART: []models.DiskTemp{
							{Device: "/dev/sda", WWN: "5000c500", Temperature: 0},
						},
					},
				},
			},
			expected: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/sda", WWN: "5000c500", Temperature: 0},
			},
		},
		{
			name: "standby skipped is not applied",
			disks: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/sda", WWN: "5000c500", Temperature: 0},
			},
			nodes: []models.Node{
				{
					Name: "node1",
					Temperature: &models.Temperature{
						Available: true,
						SMART: []models.DiskTemp{
							{Device: "/dev/sda", WWN: "5000c500", Temperature: 40, StandbySkipped: true},
						},
					},
				},
			},
			expected: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/sda", WWN: "5000c500", Temperature: 0},
			},
		},
		{
			name: "multiple nodes multiple disks with various matches",
			disks: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/sda", WWN: "wwn1", Temperature: 0},
				{Node: "node1", DevPath: "/dev/sdb", Serial: "SERIAL2", Temperature: 0},
				{Node: "node1", DevPath: "/dev/nvme0n1", Type: "nvme", Temperature: 0},
				{Node: "node2", DevPath: "/dev/sda", Temperature: 0},
				{Node: "node2", DevPath: "/dev/nvme0n1", Type: "nvme", Temperature: 0},
				{Node: "node2", DevPath: "/dev/nvme1n1", Type: "nvme", Temperature: 0},
			},
			nodes: []models.Node{
				{
					Name: "node1",
					Temperature: &models.Temperature{
						Available: true,
						SMART: []models.DiskTemp{
							{Device: "/dev/sda", WWN: "WWN1", Temperature: 41},
							{Device: "/dev/sdb", Serial: "serial2", Temperature: 42},
						},
						NVMe: []models.NVMeTemp{
							{Device: "nvme0", Temp: 50.2},
						},
					},
				},
				{
					Name: "node2",
					Temperature: &models.Temperature{
						Available: true,
						SMART: []models.DiskTemp{
							{Device: "/dev/sda", Temperature: 43},
						},
						NVMe: []models.NVMeTemp{
							{Device: "nvme0", Temp: 51.8},
							{Device: "nvme1", Temp: 52.3},
						},
					},
				},
			},
			expected: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/sda", WWN: "wwn1", Temperature: 41},
				{Node: "node1", DevPath: "/dev/sdb", Serial: "SERIAL2", Temperature: 42},
				{Node: "node1", DevPath: "/dev/nvme0n1", Type: "nvme", Temperature: 50},
				{Node: "node2", DevPath: "/dev/sda", Temperature: 43},
				{Node: "node2", DevPath: "/dev/nvme0n1", Type: "nvme", Temperature: 52},
				{Node: "node2", DevPath: "/dev/nvme1n1", Type: "nvme", Temperature: 52},
			},
		},
		{
			name: "NaN temperature is not applied",
			disks: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/nvme0n1", Type: "nvme", Temperature: 0},
			},
			nodes: []models.Node{
				{
					Name: "node1",
					Temperature: &models.Temperature{
						Available: true,
						NVMe: []models.NVMeTemp{
							{Device: "nvme0", Temp: math.NaN()},
						},
					},
				},
			},
			expected: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/nvme0n1", Type: "nvme", Temperature: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeNVMeTempsIntoDisks(tt.disks, tt.nodes)

			if len(result) != len(tt.expected) {
				t.Fatalf("got %d disks, want %d", len(result), len(tt.expected))
			}

			for i := range result {
				if result[i].Temperature != tt.expected[i].Temperature {
					t.Errorf("disk[%d] %s: got temperature %d, want %d",
						i, result[i].DevPath, result[i].Temperature, tt.expected[i].Temperature)
				}
			}
		})
	}
}

func TestMergeNVMeTempsIntoDisks_OriginalSliceUnchanged(t *testing.T) {
	original := []models.PhysicalDisk{
		{Node: "node1", DevPath: "/dev/nvme0n1", Type: "nvme", Temperature: 0},
	}

	nodes := []models.Node{
		{
			Name: "node1",
			Temperature: &models.Temperature{
				Available: true,
				NVMe: []models.NVMeTemp{
					{Device: "nvme0", Temp: 45.0},
				},
			},
		},
	}

	result := mergeNVMeTempsIntoDisks(original, nodes)

	if result[0].Temperature != 45 {
		t.Errorf("merged disk temperature = %d, want 45", result[0].Temperature)
	}

	if original[0].Temperature != 0 {
		t.Errorf("original disk temperature was modified: got %d, want 0", original[0].Temperature)
	}
}
