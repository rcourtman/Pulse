package hostagent

import (
	"context"
	"errors"
	"testing"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func withMdadmCommandRunner(t *testing.T, fn func(ctx context.Context, name string, args ...string) ([]byte, error)) {
	t.Helper()
	orig := mdadmCommandRunner
	mdadmCommandRunner = fn
	t.Cleanup(func() { mdadmCommandRunner = orig })
}

func TestParseMdadmDetail(t *testing.T) {
	tests := []struct {
		name    string
		device  string
		output  string
		want    agentshost.RAIDArray
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
			want: agentshost.RAIDArray{
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
				Devices: []agentshost.RAIDDevice{
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
			want: agentshost.RAIDArray{
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
				Devices: []agentshost.RAIDDevice{
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
			want: agentshost.RAIDArray{
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
				Devices: []agentshost.RAIDDevice{
					{Device: "/dev/sda1", State: "active sync", Slot: 0},
					{Device: "/dev/sdb1", State: "active sync", Slot: 1},
					{Device: "/dev/sdc1", State: "active sync", Slot: 2},
					{Device: "/dev/sdd1", State: "active sync", Slot: 3},
					{Device: "/dev/sdf1", State: "spare rebuilding", Slot: 6},
					{Device: "/dev/sde1", State: "active sync", Slot: 5},
				},
			},
		},
		{
			name:   "RAID1 with spare devices",
			device: "/dev/md3",
			output: `/dev/md3:
           Version : 1.2
        Raid Level : raid1
      Raid Devices : 2
     Total Devices : 3

             State : clean
    Active Devices : 2
   Working Devices : 3
    Failed Devices : 0
     Spare Devices : 1

              Name : server:3
              UUID : 11223344:55667788:99aabbcc:ddeeff00

    Number   Major   Minor   RaidDevice State
       0       8        1        0      active sync   /dev/sda1
       1       8       17        1      active sync   /dev/sdb1

       2       8       33        -      spare   /dev/sdc1`,
			want: agentshost.RAIDArray{
				Device:         "/dev/md3",
				Name:           "server:3",
				Level:          "raid1",
				State:          "clean",
				TotalDevices:   3,
				ActiveDevices:  2,
				WorkingDevices: 3,
				FailedDevices:  0,
				SpareDevices:   1,
				UUID:           "11223344:55667788:99aabbcc:ddeeff00",
				Devices: []agentshost.RAIDDevice{
					{Device: "/dev/sda1", State: "active sync", Slot: 0},
					{Device: "/dev/sdb1", State: "active sync", Slot: 1},
					{Device: "/dev/sdc1", State: "spare", Slot: -1},
				},
			},
		},
		{
			name:   "RAID10 array",
			device: "/dev/md4",
			output: `/dev/md4:
           Version : 1.2
        Raid Level : raid10
        Array Size : 209715200 (200.00 GiB 214.75 GB)
      Raid Devices : 4
     Total Devices : 4

             State : clean
    Active Devices : 4
   Working Devices : 4
    Failed Devices : 0
     Spare Devices : 0

              Layout : near=2
          Chunk Size : 512K

              Name : server:4
              UUID : aabbccdd:eeff0011:22334455:66778899

    Number   Major   Minor   RaidDevice State
       0       8        1        0      active sync set-A   /dev/sda1
       1       8       17        1      active sync set-B   /dev/sdb1
       2       8       33        2      active sync set-A   /dev/sdc1
       3       8       49        3      active sync set-B   /dev/sdd1`,
			want: agentshost.RAIDArray{
				Device:         "/dev/md4",
				Name:           "server:4",
				Level:          "raid10",
				State:          "clean",
				TotalDevices:   4,
				ActiveDevices:  4,
				WorkingDevices: 4,
				FailedDevices:  0,
				SpareDevices:   0,
				UUID:           "aabbccdd:eeff0011:22334455:66778899",
				Devices: []agentshost.RAIDDevice{
					{Device: "/dev/sda1", State: "active sync set-A", Slot: 0},
					{Device: "/dev/sdb1", State: "active sync set-B", Slot: 1},
					{Device: "/dev/sdc1", State: "active sync set-A", Slot: 2},
					{Device: "/dev/sdd1", State: "active sync set-B", Slot: 3},
				},
			},
		},
		{
			name:   "RAID0 array (striped)",
			device: "/dev/md5",
			output: `/dev/md5:
           Version : 1.2
        Raid Level : raid0
        Array Size : 209715200 (200.00 GiB 214.75 GB)
      Raid Devices : 2
     Total Devices : 2

             State : clean
    Active Devices : 2
   Working Devices : 2
    Failed Devices : 0
     Spare Devices : 0

          Chunk Size : 512K

              Name : server:5
              UUID : 12ab34cd:56ef78gh:90ij12kl:34mn56op

    Number   Major   Minor   RaidDevice State
       0       8        1        0      active sync   /dev/sda1
       1       8       17        1      active sync   /dev/sdb1`,
			want: agentshost.RAIDArray{
				Device:         "/dev/md5",
				Name:           "server:5",
				Level:          "raid0",
				State:          "clean",
				TotalDevices:   2,
				ActiveDevices:  2,
				WorkingDevices: 2,
				FailedDevices:  0,
				SpareDevices:   0,
				UUID:           "12ab34cd:56ef78gh:90ij12kl:34mn56op",
				Devices: []agentshost.RAIDDevice{
					{Device: "/dev/sda1", State: "active sync", Slot: 0},
					{Device: "/dev/sdb1", State: "active sync", Slot: 1},
				},
			},
		},
		{
			name:   "reshaping array",
			device: "/dev/md6",
			output: `/dev/md6:
           Version : 1.2
        Raid Level : raid5
      Raid Devices : 4
     Total Devices : 4

             State : active, reshaping
    Active Devices : 4
   Working Devices : 4
    Failed Devices : 0
     Spare Devices : 0

    Reshape Status : 23.5% complete

              UUID : 99887766:55443322:11009988:77665544

    Number   Major   Minor   RaidDevice State
       0       8        1        0      active sync   /dev/sda1
       1       8       17        1      active sync   /dev/sdb1
       2       8       33        2      active sync   /dev/sdc1
       3       8       49        3      active sync   /dev/sdd1`,
			want: agentshost.RAIDArray{
				Device:         "/dev/md6",
				Level:          "raid5",
				State:          "active, reshaping",
				TotalDevices:   4,
				ActiveDevices:  4,
				WorkingDevices: 4,
				FailedDevices:  0,
				SpareDevices:   0,
				UUID:           "99887766:55443322:11009988:77665544",
				RebuildPercent: 23.5,
				Devices: []agentshost.RAIDDevice{
					{Device: "/dev/sda1", State: "active sync", Slot: 0},
					{Device: "/dev/sdb1", State: "active sync", Slot: 1},
					{Device: "/dev/sdc1", State: "active sync", Slot: 2},
					{Device: "/dev/sdd1", State: "active sync", Slot: 3},
				},
			},
		},
		{
			name:   "array with multiple faulty and spare devices",
			device: "/dev/md7",
			output: `/dev/md7:
           Version : 1.2
        Raid Level : raid5
      Raid Devices : 3
     Total Devices : 5

             State : clean, degraded
    Active Devices : 2
   Working Devices : 3
    Failed Devices : 2
     Spare Devices : 1

              UUID : ffeeddcc:bbaa9988:77665544:33221100

    Number   Major   Minor   RaidDevice State
       0       8        1        0      active sync   /dev/sda1
       -       0        0        1      removed
       2       8       33        2      active sync   /dev/sdc1

       3       8       49        -      spare   /dev/sdd1
       1       8       17        -      faulty   /dev/sdb1
       4       8       65        -      faulty   /dev/sde1`,
			want: agentshost.RAIDArray{
				Device:         "/dev/md7",
				Level:          "raid5",
				State:          "clean, degraded",
				TotalDevices:   5,
				ActiveDevices:  2,
				WorkingDevices: 3,
				FailedDevices:  2,
				SpareDevices:   1,
				UUID:           "ffeeddcc:bbaa9988:77665544:33221100",
				Devices: []agentshost.RAIDDevice{
					{Device: "/dev/sda1", State: "active sync", Slot: 0},
					{Device: "/dev/sdc1", State: "active sync", Slot: 2},
					{Device: "/dev/sdd1", State: "spare", Slot: -1},
					{Device: "/dev/sdb1", State: "faulty", Slot: -1},
					{Device: "/dev/sde1", State: "faulty", Slot: -1},
				},
			},
		},
		{
			name:   "empty output",
			device: "/dev/md99",
			output: "",
			want: agentshost.RAIDArray{
				Device:  "/dev/md99",
				Devices: []agentshost.RAIDDevice{},
			},
		},
		{
			name:   "minimal output with only version",
			device: "/dev/md10",
			output: `/dev/md10:
           Version : 1.2`,
			want: agentshost.RAIDArray{
				Device:  "/dev/md10",
				Devices: []agentshost.RAIDDevice{},
			},
		},
		{
			name:   "output with extra whitespace",
			device: "/dev/md11",
			output: `

/dev/md11:
           Version : 1.2
        Raid Level : raid1


             State : clean

    Active Devices : 2
   Working Devices : 2
    Failed Devices : 0
     Spare Devices : 0

              UUID : 12341234:56785678:90ab90ab:cdefcdef


    Number   Major   Minor   RaidDevice State
       0       8        1        0      active sync   /dev/sda1

       1       8       17        1      active sync   /dev/sdb1

`,
			want: agentshost.RAIDArray{
				Device:         "/dev/md11",
				Level:          "raid1",
				State:          "clean",
				ActiveDevices:  2,
				WorkingDevices: 2,
				FailedDevices:  0,
				SpareDevices:   0,
				UUID:           "12341234:56785678:90ab90ab:cdefcdef",
				Devices: []agentshost.RAIDDevice{
					{Device: "/dev/sda1", State: "active sync", Slot: 0},
					{Device: "/dev/sdb1", State: "active sync", Slot: 1},
				},
			},
		},
		{
			name:   "rebuild with decimal percentage",
			device: "/dev/md12",
			output: `/dev/md12:
           Version : 1.2
        Raid Level : raid1
      Raid Devices : 2
     Total Devices : 2

             State : active, degraded, recovering
    Active Devices : 1
   Working Devices : 2
    Failed Devices : 0
     Spare Devices : 1

    Rebuild Status : 67.8% complete

              UUID : abcdef12:34567890:abcdef12:34567890

    Number   Major   Minor   RaidDevice State
       0       8        1        0      active sync   /dev/sda1
       1       8       17        1      spare rebuilding   /dev/sdb1`,
			want: agentshost.RAIDArray{
				Device:         "/dev/md12",
				Level:          "raid1",
				State:          "active, degraded, recovering",
				TotalDevices:   2,
				ActiveDevices:  1,
				WorkingDevices: 2,
				FailedDevices:  0,
				SpareDevices:   1,
				UUID:           "abcdef12:34567890:abcdef12:34567890",
				RebuildPercent: 67.8,
				Devices: []agentshost.RAIDDevice{
					{Device: "/dev/sda1", State: "active sync", Slot: 0},
					{Device: "/dev/sdb1", State: "spare rebuilding", Slot: 1},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMdadmDetail(tt.device, tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseMdadmDetail() error = %v, wantErr %v", err, tt.wantErr)
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

func TestIsMdadmAvailable(t *testing.T) {
	t.Run("available", func(t *testing.T) {
		withMdadmCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("mdadm"), nil
		})

		if !isMdadmAvailable(context.Background()) {
			t.Fatal("expected mdadm available")
		}
	})

	t.Run("missing", func(t *testing.T) {
		withMdadmCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, errors.New("missing")
		})

		if isMdadmAvailable(context.Background()) {
			t.Fatal("expected mdadm unavailable")
		}
	})
}

func TestListArrayDevices(t *testing.T) {
	mdstat := `Personalities : [raid1] [raid6]
md0 : active raid1 sdb1[1] sda1[0]
md1 : active raid6 sdc1[2] sdb1[1] sda1[0]
unused devices: <none>`
	withMdadmCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(mdstat), nil
	})

	devices, err := listArrayDevices(context.Background())
	if err != nil {
		t.Fatalf("listArrayDevices error: %v", err)
	}
	if len(devices) != 2 || devices[0] != "/dev/md0" || devices[1] != "/dev/md1" {
		t.Fatalf("unexpected devices: %v", devices)
	}
}

func TestListArrayDevicesError(t *testing.T) {
	withMdadmCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("read failed")
	})

	if _, err := listArrayDevices(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestCollectArrayDetailError(t *testing.T) {
	withMdadmCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("detail failed")
	})

	if _, err := collectArrayDetail(context.Background(), "/dev/md0"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCollectRAIDArraysNotAvailable(t *testing.T) {
	withMdadmCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("missing")
	})

	arrays, err := CollectRAIDArrays(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if arrays != nil {
		t.Fatalf("expected nil arrays, got %v", arrays)
	}
}

func TestCollectRAIDArraysListError(t *testing.T) {
	withMdadmCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "mdadm" {
			return []byte("mdadm"), nil
		}
		return nil, errors.New("read failed")
	})

	if _, err := CollectRAIDArrays(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestCollectRAIDArraysNoDevices(t *testing.T) {
	withMdadmCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "mdadm" {
			return []byte("mdadm"), nil
		}
		return []byte("unused devices: <none>"), nil
	})

	arrays, err := CollectRAIDArrays(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if arrays != nil {
		t.Fatalf("expected nil arrays, got %v", arrays)
	}
}

func TestCollectRAIDArraysSkipsDetailError(t *testing.T) {
	withMdadmCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		switch name {
		case "mdadm":
			if len(args) > 0 && args[0] == "--version" {
				return []byte("mdadm"), nil
			}
			return nil, errors.New("detail failed")
		case "cat":
			return []byte("md0 : active raid1 sda1[0]"), nil
		default:
			return nil, errors.New("unexpected")
		}
	})

	arrays, err := CollectRAIDArrays(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(arrays) != 0 {
		t.Fatalf("expected empty arrays, got %v", arrays)
	}
}

func TestCollectRAIDArraysSuccess(t *testing.T) {
	detail := `/dev/md0:
        Raid Level : raid1
             State : clean
     Total Devices : 2
    Active Devices : 2
   Working Devices : 2
    Failed Devices : 0
     Spare Devices : 0

    Number   Major   Minor   RaidDevice State
       0       8        1        0      active sync   /dev/sda1`

	withMdadmCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		switch name {
		case "mdadm":
			if len(args) > 0 && args[0] == "--version" {
				return []byte("mdadm"), nil
			}
			return []byte(detail), nil
		case "cat":
			return []byte("md0 : active raid1 sda1[0]"), nil
		default:
			return nil, errors.New("unexpected")
		}
	})

	arrays, err := CollectRAIDArrays(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(arrays) != 1 || arrays[0].Device != "/dev/md0" {
		t.Fatalf("unexpected arrays: %v", arrays)
	}
}

func TestGetRebuildSpeed(t *testing.T) {
	mdstat := `md0 : active raid1 sda1[0] sdb1[1]
      [>....................]  recovery = 12.6% (37043392/293039104) finish=127.5min speed=33440K/sec
`
	withMdadmCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(mdstat), nil
	})

	if speed := getRebuildSpeed("/dev/md0"); speed != "33440K/sec" {
		t.Fatalf("unexpected speed: %s", speed)
	}
}

func TestGetRebuildSpeedNoMatch(t *testing.T) {
	withMdadmCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("md0 : active raid1 sda1[0]"), nil
	})

	if speed := getRebuildSpeed("/dev/md0"); speed != "" {
		t.Fatalf("expected empty speed, got %s", speed)
	}
}

func TestGetRebuildSpeedError(t *testing.T) {
	withMdadmCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("read failed")
	})

	if speed := getRebuildSpeed("/dev/md0"); speed != "" {
		t.Fatalf("expected empty speed, got %s", speed)
	}
}

func TestParseMdadmDetailSetsRebuildSpeed(t *testing.T) {
	output := `/dev/md0:
        Raid Level : raid1
             State : clean
    Rebuild Status : 12% complete

    Number   Major   Minor   RaidDevice State
       0       8        1        0      active sync   /dev/sda1`

	mdstat := `md0 : active raid1 sda1[0]
      [>....................]  recovery = 12.6% (37043392/293039104) finish=127.5min speed=1234K/sec
`
	withMdadmCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(mdstat), nil
	})

	array, err := parseMdadmDetail("/dev/md0", output)
	if err != nil {
		t.Fatalf("parseMdadmDetail error: %v", err)
	}
	if array.RebuildSpeed != "1234K/sec" {
		t.Fatalf("expected rebuild speed, got %s", array.RebuildSpeed)
	}
}

func TestGetRebuildSpeedSectionExit(t *testing.T) {
	mdstat := `md0 : active raid1 sda1[0]
      [>....................]  recovery = 12.6% (37043392/293039104) finish=127.5min
md1 : active raid1 sdb1[0]
`
	withMdadmCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(mdstat), nil
	})

	if speed := getRebuildSpeed("/dev/md0"); speed != "" {
		t.Fatalf("expected empty speed, got %s", speed)
	}
}
