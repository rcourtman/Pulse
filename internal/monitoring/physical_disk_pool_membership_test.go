package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestDiskPoolAssignmentLookup(t *testing.T) {
	pools := []proxmox.ZFSPoolInfo{
		{
			Name: "rpool",
			Devices: []proxmox.ZFSPoolDevice{
				{
					Name: "mirror-0",
					Children: []proxmox.ZFSPoolDevice{
						{Name: "/dev/disk/by-id/ata-Samsung_SSD_870_EVO_1TB_S5Y2NX0R500001Z-part3", Leaf: 1},
						{Name: "/dev/disk/by-id/ata-Samsung_SSD_870_EVO_1TB_S5Y2NX0R500002Z-part3", Leaf: 1},
					},
				},
			},
		},
		{
			Name: "tank",
			Devices: []proxmox.ZFSPoolDevice{
				{Name: "sdc", Leaf: 1},
				{Name: "/dev/nvme1n1p1", Leaf: 1},
			},
		},
		{
			Name: "scratch",
			Devices: []proxmox.ZFSPoolDevice{
				{Name: "wwn-0x50014ee2123456ab", Leaf: 1},
			},
		},
	}

	assignment := buildDiskPoolAssignment(pools)

	cases := []struct {
		label string
		disk  models.PhysicalDisk
		want  string
	}{
		{
			label: "by-id match via serial token",
			disk:  models.PhysicalDisk{DevPath: "/dev/sda", Serial: "S5Y2NX0R500001Z"},
			want:  "rpool",
		},
		{
			label: "short leaf name matches partition-stripped devpath",
			disk:  models.PhysicalDisk{DevPath: "/dev/sdc"},
			want:  "tank",
		},
		{
			label: "nvme partition leaf matches devpath with partition stripping",
			disk:  models.PhysicalDisk{DevPath: "/dev/nvme1n1"},
			want:  "tank",
		},
		{
			label: "wwn leaf matches via disk WWN",
			disk:  models.PhysicalDisk{DevPath: "/dev/sdd", WWN: "0x50014ee2123456ab"},
			want:  "scratch",
		},
		{
			label: "disk with no pool membership returns empty",
			disk:  models.PhysicalDisk{DevPath: "/dev/sde", Serial: "UNUSED1"},
			want:  "",
		},
		{
			label: "blank devpath returns empty",
			disk:  models.PhysicalDisk{DevPath: ""},
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := assignment.lookup(tc.disk)
			if got != tc.want {
				t.Fatalf("lookup = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNilAssignmentLookupReturnsEmpty(t *testing.T) {
	var a *diskPoolAssignment
	got := a.lookup(models.PhysicalDisk{DevPath: "/dev/sda"})
	if got != "" {
		t.Fatalf("nil lookup = %q, want empty", got)
	}
}

func TestStripPartitionSuffix(t *testing.T) {
	cases := map[string]string{
		"sda":        "sda",
		"sda3":       "sda",
		"nvme0n1":    "nvme0n1",
		"nvme0n1p1":  "nvme0n1",
		"nvme10n1p3": "nvme10n1",
		"":           "",
	}
	for in, want := range cases {
		if got := stripPartitionSuffix(in); got != want {
			t.Fatalf("stripPartitionSuffix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSerialFromByID(t *testing.T) {
	cases := map[string]string{
		"/dev/disk/by-id/ata-Samsung_SSD_870_EVO_1TB_S5Y2NX0R500001Z":       "s5y2nx0r500001z",
		"/dev/disk/by-id/ata-Samsung_SSD_870_EVO_1TB_S5Y2NX0R500001Z-part3": "s5y2nx0r500001z",
		"nvme-INTEL_SSDPEKNW512G8_BTNH123456789":                            "btnh123456789",
		"wwn-0x50014ee2123456ab":                                            "50014ee2123456ab",
		"/dev/sda":                                                          "",
	}
	for in, want := range cases {
		if got := serialFromByID(in); got != want {
			t.Fatalf("serialFromByID(%q) = %q, want %q", in, got, want)
		}
	}
}
