package monitoring

import (
	"math"
	"testing"
	"time"

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
			name:     "nil temperature returns disks unchanged",
			disks:    []models.PhysicalDisk{{Node: "node1", DevPath: "/dev/sda", Temperature: 0}},
			nodes:    []models.Node{{Name: "node1", Temperature: nil}},
			expected: []models.PhysicalDisk{{Node: "node1", DevPath: "/dev/sda", Temperature: 0}},
		},
		{
			name:     "temperature available false returns disks unchanged",
			disks:    []models.PhysicalDisk{{Node: "node1", DevPath: "/dev/sda", Temperature: 0}},
			nodes:    []models.Node{{Name: "node1", Temperature: &models.Temperature{Available: false}}},
			expected: []models.PhysicalDisk{{Node: "node1", DevPath: "/dev/sda", Temperature: 0}},
		},
		{
			name:     "no SMART or NVMe temps returns disks unchanged",
			disks:    []models.PhysicalDisk{{Node: "node1", DevPath: "/dev/sda", Temperature: 0}},
			nodes:    []models.Node{{Name: "node1", Temperature: &models.Temperature{Available: true, SMART: nil, NVMe: nil}}},
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
		{
			name: "NVMe disk with no legacy temps for node (continue branch)",
			disks: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/nvme0n1", Type: "nvme", Temperature: 0},
				{Node: "node2", DevPath: "/dev/nvme0n1", Type: "nvme", Temperature: 0},
			},
			nodes: []models.Node{
				{
					Name: "node1",
					Temperature: &models.Temperature{
						Available: true,
						NVMe: []models.NVMeTemp{
							{Device: "nvme0", Temp: 45.0},
						},
					},
				},
				{
					Name: "node2",
					Temperature: &models.Temperature{
						Available: true,
						// No NVMe temps for node2, but SMART is empty too
					},
				},
			},
			expected: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/nvme0n1", Type: "nvme", Temperature: 45},
				{Node: "node2", DevPath: "/dev/nvme0n1", Type: "nvme", Temperature: 0},
			},
		},
		{
			name: "more NVMe disks than temps (break branch)",
			disks: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/nvme0n1", Type: "nvme", Temperature: 0},
				{Node: "node1", DevPath: "/dev/nvme1n1", Type: "nvme", Temperature: 0},
				{Node: "node1", DevPath: "/dev/nvme2n1", Type: "nvme", Temperature: 0},
			},
			nodes: []models.Node{
				{
					Name: "node1",
					Temperature: &models.Temperature{
						Available: true,
						NVMe: []models.NVMeTemp{
							{Device: "nvme0", Temp: 40.0},
						},
					},
				},
			},
			expected: []models.PhysicalDisk{
				{Node: "node1", DevPath: "/dev/nvme0n1", Type: "nvme", Temperature: 40},
				{Node: "node1", DevPath: "/dev/nvme1n1", Type: "nvme", Temperature: 0},
				{Node: "node1", DevPath: "/dev/nvme2n1", Type: "nvme", Temperature: 0},
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

func TestNormalizeSMARTDeviceIdentifier(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"sdd [scsi]", "sdd"},
		{"/dev/sdd [scsi]", "sdd"},
		{"/dev/nvme0n1", "nvme0n1"},
		{"sda", "sda"},
		{"  sdb [ata]  ", "sdb"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeSMARTDeviceIdentifier(tc.in); got != tc.want {
			t.Errorf("normalizeSMARTDeviceIdentifier(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMergeHostAgentSMARTIntoDisks_EnrichesMissingIdentityAndZFSPool(t *testing.T) {
	powerOnHours := int64(12345)
	percentageUsed := 12
	disks := []models.PhysicalDisk{
		{
			ID:          "pve1-node1--dev-sda",
			Instance:    "pve1",
			Node:        "node1",
			DevPath:     "/dev/sda",
			Health:      "UNKNOWN",
			Wearout:     -1,
			LastChecked: mustParseTime(t, "2026-06-01T12:00:00Z"),
		},
	}
	nodes := []models.Node{
		{Name: "node1", LinkedAgentID: "host-node1"},
	}
	hosts := []models.Host{
		{
			ID: "host-node1",
			Sensors: models.HostSensorSummary{
				SMART: []models.HostDiskSMART{
					{
						Device:      "sda [sat]",
						Model:       "WDC Red",
						Serial:      "WD-SATA-1",
						WWN:         "5000c5001234abcd",
						Type:        "sata",
						SizeBytes:   4_000_000_000_000,
						Temperature: 38,
						Health:      "PASSED",
						Pool:        "tank",
						Attributes: &models.SMARTAttributes{
							PowerOnHours:   &powerOnHours,
							PercentageUsed: &percentageUsed,
						},
					},
				},
			},
		},
	}

	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if len(result) != 1 {
		t.Fatalf("got %d disks, want 1", len(result))
	}
	got := result[0]
	if got.Temperature != 38 {
		t.Fatalf("temperature = %d, want 38", got.Temperature)
	}
	if got.Model != "WDC Red" || got.Serial != "WD-SATA-1" || got.WWN != "5000c5001234abcd" || got.Type != "sata" {
		t.Fatalf("identity not enriched from SMART row: %+v", got)
	}
	if got.Size != 4_000_000_000_000 {
		t.Fatalf("size = %d, want host-agent SMART size", got.Size)
	}
	if got.StorageGroup != "tank" {
		t.Fatalf("storageGroup = %q, want tank", got.StorageGroup)
	}
	if got.Health != "PASSED" {
		t.Fatalf("health = %q, want PASSED", got.Health)
	}
	if got.Wearout != 88 {
		t.Fatalf("wearout = %d, want 88 derived from SMART percentage-used", got.Wearout)
	}
	if got.SmartAttributes == nil || got.SmartAttributes.PowerOnHours == nil || *got.SmartAttributes.PowerOnHours != powerOnHours {
		t.Fatalf("SMART attributes were not copied: %+v", got.SmartAttributes)
	}

	if disks[0].Serial != "" || disks[0].Temperature != 0 || disks[0].StorageGroup != "" {
		t.Fatalf("original disk slice was modified: %+v", disks[0])
	}
}

func TestMergeHostAgentSMARTIntoDisks_PreservesExistingProxmoxIdentity(t *testing.T) {
	disks := []models.PhysicalDisk{
		{
			ID:           "pve1-node1--dev-sdb",
			Instance:     "pve1",
			Node:         "node1",
			DevPath:      "/dev/sdb",
			Model:        "Proxmox Model",
			Serial:       "PVE-SERIAL",
			WWN:          "pve-wwn",
			Type:         "sas",
			Size:         100,
			Health:       "PASSED",
			Temperature:  41,
			StorageGroup: "api-pool",
		},
	}
	nodes := []models.Node{{Name: "node1", LinkedAgentID: "host-node1"}}
	hosts := []models.Host{
		{
			ID: "host-node1",
			Sensors: models.HostSensorSummary{
				SMART: []models.HostDiskSMART{
					{
						Device:      "/dev/sdb",
						Model:       "Agent Model",
						Serial:      "AGENT-SERIAL",
						WWN:         "agent-wwn",
						Type:        "sata",
						SizeBytes:   200,
						Temperature: 35,
						Health:      "FAILED",
						Pool:        "agent-pool",
					},
				},
			},
		},
	}

	got := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)[0]
	if got.Model != "Proxmox Model" || got.Serial != "PVE-SERIAL" || got.WWN != "pve-wwn" || got.Type != "sas" {
		t.Fatalf("existing Proxmox identity was overwritten: %+v", got)
	}
	if got.Size != 100 || got.Temperature != 41 || got.Health != "PASSED" || got.StorageGroup != "api-pool" {
		t.Fatalf("existing Proxmox disk fields were overwritten: %+v", got)
	}
}

func mustParseTime(t *testing.T, raw string) time.Time {
	t.Helper()
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse time %q: %v", raw, err)
	}
	return value
}
