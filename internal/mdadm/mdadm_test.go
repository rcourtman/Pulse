package mdadm

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func TestParseDetail(t *testing.T) {
	tests := []struct {
		name    string
		device  string
		output  string
		want    host.RAIDArray
		wantErr bool
	}{
		{
			name:   "RAID1 healthy array",
			device: "/dev/md0",
			output: `/dev/md0:
           Version : 1.2
     Creation Time : Thu Jan 15 10:00:00 2025
        Raid Level : raid1
        Array Size : 102400000 (97.66 GiB 104.86 GB)
     Used Dev Size : 102400000 (97.66 GiB 104.86 GB)
      Raid Devices : 2
     Total Devices : 2
       Persistence : Superblock is persistent

       Update Time : Thu Jan 16 12:00:00 2025
             State : clean
    Active Devices : 2
   Working Devices : 2
    Failed Devices : 0
     Spare Devices : 0

Consistency Policy : resync

              Name : server:0
              UUID : 12345678:90abcdef:12345678:90abcdef

    Number   Major   Minor   RaidDevice State
       0       8        1        0      active sync   /dev/sda1
       1       8       17        1      active sync   /dev/sdb1`,
			want: host.RAIDArray{
				Device:         "/dev/md0",
				Name:           "server:0",
				Level:          "raid1",
				State:          "clean",
				TotalDevices:   2,
				ActiveDevices:  2,
				WorkingDevices: 2,
				FailedDevices:  0,
				SpareDevices:   0,
				UUID:           "12345678:90abcdef:12345678:90abcdef",
				Devices: []host.RAIDDevice{
					{Device: "/dev/sda1", State: "active sync", Slot: 0},
					{Device: "/dev/sdb1", State: "active sync", Slot: 1},
				},
			},
		},
		{
			name:   "RAID5 degraded array",
			device: "/dev/md1",
			output: `/dev/md1:
           Version : 1.2
     Creation Time : Wed Jan 14 08:00:00 2025
        Raid Level : raid5
        Array Size : 204800000 (195.31 GiB 209.72 GB)
     Used Dev Size : 102400000 (97.66 GiB 104.86 GB)
      Raid Devices : 3
     Total Devices : 2
       Persistence : Superblock is persistent

       Update Time : Thu Jan 16 12:30:00 2025
             State : clean, degraded
    Active Devices : 2
   Working Devices : 2
    Failed Devices : 1
     Spare Devices : 0

              Name : server:1
              UUID : abcdef12:34567890:abcdef12:34567890

    Number   Major   Minor   RaidDevice State
       0       8        1        0      active sync   /dev/sda1
       -       0        0        1      removed
       2       8       33        2      active sync   /dev/sdc1

       1       8       17        -      faulty   /dev/sdb1`,
			want: host.RAIDArray{
				Device:         "/dev/md1",
				Name:           "server:1",
				Level:          "raid5",
				State:          "clean, degraded",
				TotalDevices:   2,
				ActiveDevices:  2,
				WorkingDevices: 2,
				FailedDevices:  1,
				SpareDevices:   0,
				UUID:           "abcdef12:34567890:abcdef12:34567890",
				Devices: []host.RAIDDevice{
					{Device: "/dev/sda1", State: "active sync", Slot: 0},
					{Device: "/dev/sdc1", State: "active sync", Slot: 2},
					{Device: "/dev/sdb1", State: "faulty", Slot: -1},
				},
			},
		},
		{
			name:   "RAID6 rebuilding",
			device: "/dev/md2",
			output: `/dev/md2:
           Version : 1.2
     Creation Time : Wed Jan 14 08:00:00 2025
        Raid Level : raid6
        Array Size : 409600000 (390.62 GiB 419.43 GB)
     Used Dev Size : 102400000 (97.66 GiB 104.86 GB)
      Raid Devices : 6
     Total Devices : 6
       Persistence : Superblock is persistent

       Update Time : Thu Jan 16 13:00:00 2025
             State : active, recovering
    Active Devices : 5
   Working Devices : 6
    Failed Devices : 0
     Spare Devices : 1

    Rebuild Status : 42% complete

              Name : server:2
              UUID : fedcba09:87654321:fedcba09:87654321

    Number   Major   Minor   RaidDevice State
       0       8        1        0      active sync   /dev/sda1
       1       8       17        1      active sync   /dev/sdb1
       2       8       33        2      active sync   /dev/sdc1
       3       8       49        3      active sync   /dev/sdd1
       6       8       81        4      spare rebuilding   /dev/sdf1
       5       8       65        5      active sync   /dev/sde1`,
			want: host.RAIDArray{
				Device:         "/dev/md2",
				Name:           "server:2",
				Level:          "raid6",
				State:          "active, recovering",
				TotalDevices:   6,
				ActiveDevices:  5,
				WorkingDevices: 6,
				FailedDevices:  0,
				SpareDevices:   1,
				UUID:           "fedcba09:87654321:fedcba09:87654321",
				RebuildPercent: 42.0,
				Devices: []host.RAIDDevice{
					{Device: "/dev/sda1", State: "active sync", Slot: 0},
					{Device: "/dev/sdb1", State: "active sync", Slot: 1},
					{Device: "/dev/sdc1", State: "active sync", Slot: 2},
					{Device: "/dev/sdd1", State: "active sync", Slot: 3},
					{Device: "/dev/sdf1", State: "spare rebuilding", Slot: 6},
					{Device: "/dev/sde1", State: "active sync", Slot: 5},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDetail(tt.device, tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDetail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Compare fields
			if got.Device != tt.want.Device {
				t.Errorf("Device = %v, want %v", got.Device, tt.want.Device)
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name = %v, want %v", got.Name, tt.want.Name)
			}
			if got.Level != tt.want.Level {
				t.Errorf("Level = %v, want %v", got.Level, tt.want.Level)
			}
			if got.State != tt.want.State {
				t.Errorf("State = %v, want %v", got.State, tt.want.State)
			}
			if got.TotalDevices != tt.want.TotalDevices {
				t.Errorf("TotalDevices = %v, want %v", got.TotalDevices, tt.want.TotalDevices)
			}
			if got.ActiveDevices != tt.want.ActiveDevices {
				t.Errorf("ActiveDevices = %v, want %v", got.ActiveDevices, tt.want.ActiveDevices)
			}
			if got.WorkingDevices != tt.want.WorkingDevices {
				t.Errorf("WorkingDevices = %v, want %v", got.WorkingDevices, tt.want.WorkingDevices)
			}
			if got.FailedDevices != tt.want.FailedDevices {
				t.Errorf("FailedDevices = %v, want %v", got.FailedDevices, tt.want.FailedDevices)
			}
			if got.SpareDevices != tt.want.SpareDevices {
				t.Errorf("SpareDevices = %v, want %v", got.SpareDevices, tt.want.SpareDevices)
			}
			if got.UUID != tt.want.UUID {
				t.Errorf("UUID = %v, want %v", got.UUID, tt.want.UUID)
			}
			if got.RebuildPercent != tt.want.RebuildPercent {
				t.Errorf("RebuildPercent = %v, want %v", got.RebuildPercent, tt.want.RebuildPercent)
			}

			// Compare devices
			if len(got.Devices) != len(tt.want.Devices) {
				t.Errorf("Devices count = %v, want %v", len(got.Devices), len(tt.want.Devices))
			}
			for i := range got.Devices {
				if i >= len(tt.want.Devices) {
					break
				}
				if got.Devices[i].Device != tt.want.Devices[i].Device {
					t.Errorf("Device[%d].Device = %v, want %v", i, got.Devices[i].Device, tt.want.Devices[i].Device)
				}
				if got.Devices[i].State != tt.want.Devices[i].State {
					t.Errorf("Device[%d].State = %v, want %v", i, got.Devices[i].State, tt.want.Devices[i].State)
				}
				if got.Devices[i].Slot != tt.want.Devices[i].Slot {
					t.Errorf("Device[%d].Slot = %v, want %v", i, got.Devices[i].Slot, tt.want.Devices[i].Slot)
				}
			}
		})
	}
}
