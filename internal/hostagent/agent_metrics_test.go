package hostagent

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	gohost "github.com/shirou/gopsutil/v4/host"
)

func TestBuildReport(t *testing.T) {
	// Setup mocks
	fixedTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	mc := &mockCollector{
		nowFn: func() time.Time { return fixedTime },
		hostInfoFn: func(ctx context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{
				Hostname:             "test-host",
				Uptime:               1000,
				BootTime:             1000000,
				Procs:                100,
				OS:                   "linux",
				Platform:             "debian",
				PlatformFamily:       "debian",
				PlatformVersion:      "11",
				KernelVersion:        "5.10.0",
				VirtualizationSystem: "kvm",
				VirtualizationRole:   "guest",
				HostID:               "host-id-123",
				KernelArch:           "x86_64",
			}, nil
		},
		hostUptimeFn: func(ctx context.Context) (uint64, error) {
			return 3600, nil
		},
		metricsFn: func(ctx context.Context, diskExclude []string) (hostmetrics.Snapshot, error) {
			return hostmetrics.Snapshot{
				CPUUsagePercent: 50.0,
				Memory: agentshost.MemoryMetric{
					TotalBytes: 1000,
					UsedBytes:  500,
					Usage:      50.0,
				},
				Disks: []agentshost.Disk{
					{
						Device:     "/dev/sda1",
						Mountpoint: "/",
						UsedBytes:  200,
						TotalBytes: 1000,
						Usage:      20.0,
					},
				},
				Network: []agentshost.NetworkInterface{
					{
						Name: "eth0",
					},
				},
			}, nil
		},
	}

	// Create Agent with mock
	cfg := Config{
		AgentID:   "agent-123",
		APIToken:  "test-token",
		LogLevel:  -1,
		Collector: mc,
	}
	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Test case 1: Successful collection
	t.Run("Successful collection", func(t *testing.T) {
		report, err := agent.buildReport(context.Background())
		if err != nil {
			t.Fatalf("buildReport failed: %v", err)
		}

		// Verify Agent Info
		if report.Agent.ID != "agent-123" {
			t.Errorf("Agent.ID = %q, want %q", report.Agent.ID, "agent-123")
		}

		// Verify Host Info
		if report.Host.Hostname != "test-host" {
			t.Errorf("Host.Hostname = %q, want %q", report.Host.Hostname, "test-host")
		}
		if report.Host.UptimeSeconds != 3600 {
			t.Errorf("Host.UptimeSeconds = %d, want %d", report.Host.UptimeSeconds, 3600)
		}
		if report.Host.OSName != "debian" {
			t.Errorf("Host.OSName = %q, want %q", report.Host.OSName, "debian")
		}

		// Verify Metrics
		if report.Metrics.CPUUsagePercent != 50.0 {
			t.Errorf("CPU Usage = %f, want 50.0", report.Metrics.CPUUsagePercent)
		}

		// Verify Timestamp
		if !report.Timestamp.Equal(fixedTime) {
			t.Errorf("Timestamp = %v, want %v", report.Timestamp, fixedTime)
		}
	})

	// Test case 2: Uptime failure
	t.Run("Uptime failure", func(t *testing.T) {
		mc.hostUptimeFn = func(ctx context.Context) (uint64, error) {
			return 0, errors.New("uptime failed")
		}

		report, err := agent.buildReport(context.Background())
		if err != nil {
			t.Logf("buildReport returned error on uptime fail: %v", err)
		} else {
			if report.Host.UptimeSeconds != 0 {
				t.Errorf("Host Uptime reported as %d, want 0 on failure", report.Host.UptimeSeconds)
			}
		}
		// Reset mock
		mc.hostUptimeFn = func(ctx context.Context) (uint64, error) { return 3600, nil }
	})

	// Test case 3: RAID Array collection
	t.Run("RAID collection", func(t *testing.T) {
		mc.raidArraysFn = func(ctx context.Context) ([]agentshost.RAIDArray, error) {
			return []agentshost.RAIDArray{
				{Name: "md0", State: "clean"},
			}, nil
		}
		// Ensure OS check is passed
		mc.goos = "linux"

		report, err := agent.buildReport(context.Background())
		if err != nil {
			t.Fatalf("buildReport failed: %v", err)
		}

		if len(report.RAID) != 1 {
			t.Errorf("Expected 1 RAID array, got %d", len(report.RAID))
		} else if report.RAID[0].Name != "md0" {
			t.Errorf("Expected RAID name md0, got %s", report.RAID[0].Name)
		}
		mc.raidArraysFn = nil
	})

	t.Run("RAID collection preserves mdstat fallback topology", func(t *testing.T) {
		mc.raidArraysFn = func(ctx context.Context) ([]agentshost.RAIDArray, error) {
			return []agentshost.RAIDArray{
				{
					Device:         "/dev/md0",
					Level:          "raid10",
					State:          "active",
					TotalDevices:   4,
					ActiveDevices:  4,
					WorkingDevices: 4,
					Devices: []agentshost.RAIDDevice{
						{Device: "/dev/sda1", State: "active sync", Slot: 0},
						{Device: "/dev/sdb1", State: "active sync", Slot: 1},
						{Device: "/dev/sdc1", State: "active sync", Slot: 2},
						{Device: "/dev/sdd1", State: "active sync", Slot: 3},
					},
				},
			}, nil
		}

		report, err := agent.buildReport(context.Background())
		if err != nil {
			t.Fatalf("buildReport failed: %v", err)
		}

		if len(report.RAID) != 1 {
			t.Fatalf("expected 1 RAID array, got %d", len(report.RAID))
		}
		array := report.RAID[0]
		if array.Device != "/dev/md0" || array.Level != "raid10" || array.State != "active" {
			t.Fatalf("unexpected RAID array summary: %+v", array)
		}
		if array.TotalDevices != 4 || array.ActiveDevices != 4 || len(array.Devices) != 4 {
			t.Fatalf("unexpected RAID array topology: %+v", array)
		}
		mc.raidArraysFn = nil
	})

	// Test case 4: Ceph collection
	t.Run("Ceph collection", func(t *testing.T) {
		mc.cephStatusFn = func(ctx context.Context) (*CephClusterStatus, error) {
			return &CephClusterStatus{
				FSID: "ceph-fsid-123",
				Health: CephHealthStatus{
					Status: "HEALTH_OK",
				},
				MonMap: CephMonitorMap{
					NumMons: 1,
					Monitors: []CephMonitor{
						{Name: "a"},
					},
				},
			}, nil
		}
		mc.goos = "linux"

		report, err := agent.buildReport(context.Background())
		if err != nil {
			t.Fatalf("buildReport failed: %v", err)
		}

		if report.Ceph == nil {
			t.Errorf("Ceph report is nil")
		} else {
			if report.Ceph.FSID != "ceph-fsid-123" {
				t.Errorf("Expected Ceph FSID ceph-fsid-123, got %s", report.Ceph.FSID)
			}
		}
		mc.cephStatusFn = nil
	})

	// Test case 5: Unraid collection
	t.Run("Unraid collection", func(t *testing.T) {
		mc.unraidStorageFn = func(ctx context.Context) (*agentshost.UnraidStorage, error) {
			return &agentshost.UnraidStorage{
				ArrayStarted: true,
				ArrayState:   "STARTED",
				SyncAction:   "check",
				Disks: []agentshost.UnraidDisk{
					{Name: "parity", Role: "parity", Status: "online"},
					{
						Name:        "disk1",
						Device:      "/dev/sdc",
						Role:        "data",
						Status:      "online",
						RawStatus:   "DISK_OK",
						Model:       "WDC WD60EFRX",
						Serial:      "DATA-1",
						Filesystem:  "xfs",
						Transport:   "sata",
						SizeBytes:   6_000_000_000_000,
						UsedBytes:   4_000,
						FreeBytes:   2_000,
						Temperature: 31,
						SpunDown:    true,
						ReadCount:   11,
						WriteCount:  12,
						ErrorCount:  1,
						Slot:        1,
					},
				},
			}, nil
		}
		mc.goos = "linux"

		report, err := agent.buildReport(context.Background())
		if err != nil {
			t.Fatalf("buildReport failed: %v", err)
		}

		if report.Unraid == nil {
			t.Fatal("Unraid report is nil")
		}
		if !report.Unraid.ArrayStarted {
			t.Fatal("expected Unraid array to be started")
		}
		if len(report.Unraid.Disks) != 2 {
			t.Fatalf("expected 2 Unraid disks, got %d", len(report.Unraid.Disks))
		}
		disk := report.Unraid.Disks[1]
		if disk.Model != "WDC WD60EFRX" || disk.Transport != "sata" || disk.SizeBytes != 6_000_000_000_000 {
			t.Fatalf("expected enriched Unraid disk metadata, got %+v", disk)
		}
		if disk.UsedBytes != 4_000 || disk.FreeBytes != 2_000 || disk.Temperature != 31 || !disk.SpunDown {
			t.Fatalf("expected native Unraid capacity and state fields, got %+v", disk)
		}
		if disk.ReadCount != 11 || disk.WriteCount != 12 || disk.ErrorCount != 1 {
			t.Fatalf("expected native Unraid counters, got %+v", disk)
		}
		mc.unraidStorageFn = nil
	})

	t.Run("Unraid parser skips empty no-present slots", func(t *testing.T) {
		storage, err := parseUnraidStatusOutput(`
mdState=STARTED
diskNumber.0=0
diskName.0=
diskSize.0=0
rdevStatus.0=DISK_NP_DSBL
rdevName.0=
diskId.0=
rdevId.0=
diskNumber.1=1
diskName.1=md1p1
diskSize.1=5860522532
rdevStatus.1=DISK_OK
rdevName.1=sde
diskId.1=WDC_DATA
rdevId.1=WDC_DATA
diskNumber.29=29
diskName.29=
diskSize.29=0
rdevStatus.29=DISK_NP
rdevName.29=
diskId.29=
rdevId.29=
`)
		if err != nil {
			t.Fatalf("parseUnraidStatusOutput() error = %v", err)
		}
		if len(storage.Disks) != 1 {
			t.Fatalf("disk count = %d, want assigned slots only: %+v", len(storage.Disks), storage.Disks)
		}
		if got := storage.Disks[0]; got.Device != "/dev/sde" || got.Serial != "WDC_DATA" {
			t.Fatalf("assigned disk = %+v, want device /dev/sde with serial fallback", got)
		}
	})

	// Test case 6: SMART collection
	t.Run("SMART collection", func(t *testing.T) {
		mc.smartLocalFn = func(_ context.Context, _ []string) ([]DiskSMART, error) {
			return []DiskSMART{
				{
					Device:      "/dev/sda",
					Model:       "TestDisk",
					Health:      "PASSED",
					Temperature: 35,
				},
			}, nil
		}
		mc.goos = "linux"

		report, err := agent.buildReport(context.Background())
		if err != nil {
			t.Fatalf("buildReport failed: %v", err)
		}

		if len(report.Sensors.SMART) != 1 {
			t.Errorf("Expected 1 SMART disk, got %d", len(report.Sensors.SMART))
		} else if report.Sensors.SMART[0].Device != "/dev/sda" {
			t.Errorf("Expected device /dev/sda, got %s", report.Sensors.SMART[0].Device)
		}
		mc.smartLocalFn = nil
	})

	t.Run("SMART collection preserves typed controller-backed attributes", func(t *testing.T) {
		used := 6
		spare := 94
		mc.smartLocalFn = func(_ context.Context, _ []string) ([]DiskSMART, error) {
			return []DiskSMART{
				{
					Device: "/dev/sda [megaraid,7]",
					Model:  "RAID SSD",
					Type:   "nvme",
					Health: "PASSED",
					Attributes: &SMARTAttributes{
						PercentageUsed: &used,
						AvailableSpare: &spare,
					},
				},
			}, nil
		}
		mc.goos = "linux"

		report, err := agent.buildReport(context.Background())
		if err != nil {
			t.Fatalf("buildReport failed: %v", err)
		}
		if len(report.Sensors.SMART) != 1 {
			t.Fatalf("Expected 1 SMART disk, got %d", len(report.Sensors.SMART))
		}

		disk := report.Sensors.SMART[0]
		if disk.Device != "/dev/sda [megaraid,7]" {
			t.Fatalf("Expected typed controller-backed device label, got %s", disk.Device)
		}
		if disk.Attributes == nil {
			t.Fatal("expected SMART attributes to be preserved")
		}
		if disk.Attributes.PercentageUsed == nil || *disk.Attributes.PercentageUsed != used {
			t.Fatalf("expected PercentageUsed=%d, got %#v", used, disk.Attributes.PercentageUsed)
		}
		if disk.Attributes.AvailableSpare == nil || *disk.Attributes.AvailableSpare != spare {
			t.Fatalf("expected AvailableSpare=%d, got %#v", spare, disk.Attributes.AvailableSpare)
		}
		mc.smartLocalFn = nil
	})
}

func TestBuildReportIncludesDarwinThermalState(t *testing.T) {
	mc := &mockCollector{
		goos:  "darwin",
		nowFn: func() time.Time { return time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC) },
		hostInfoFn: func(ctx context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{
				Hostname: "mac-mini",
				OS:       "darwin",
				Platform: "darwin",
				HostID:   "mac-machine-id",
			}, nil
		},
		hostUptimeFn: func(ctx context.Context) (uint64, error) {
			return 3600, nil
		},
		metricsFn: func(ctx context.Context, diskExclude []string) (hostmetrics.Snapshot, error) {
			return hostmetrics.Snapshot{}, nil
		},
		commandCombinedOutputFn: func(ctx context.Context, name string, arg ...string) (string, error) {
			if name != "pmset" || len(arg) != 2 || arg[0] != "-g" || arg[1] != "therm" {
				t.Fatalf("unexpected command %s %v", name, arg)
			}
			return "Thermal Warning Level: 1\nCPU_Speed_Limit = 72\n", nil
		},
	}

	agent, err := New(Config{
		AgentID:   "mac-agent",
		APIToken:  "token",
		LogLevel:  -1,
		Collector: mc,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	report, err := agent.buildReport(context.Background())
	if err != nil {
		t.Fatalf("buildReport failed: %v", err)
	}

	if report.Sensors.ThermalState == nil {
		t.Fatalf("expected Darwin thermal state in report, got %+v", report.Sensors)
	}
	state := report.Sensors.ThermalState
	if state.Source != "pmset" || state.Pressure != agentshost.ThermalPressureConstrained {
		t.Fatalf("unexpected thermal state: %+v", state)
	}
	if state.ThermalWarningLevel == nil || *state.ThermalWarningLevel != 1 {
		t.Fatalf("thermal warning level = %+v, want 1", state.ThermalWarningLevel)
	}
	if got := state.LimitsPercent["cpu_speed_limit"]; got != 72 {
		t.Fatalf("cpu_speed_limit = %d, want 72", got)
	}
	if len(report.Sensors.TemperatureCelsius) != 0 {
		t.Fatalf("Darwin pressure-only report must not invent Celsius readings: %+v", report.Sensors.TemperatureCelsius)
	}
}

func TestBuildReportUsesResolvedNASOSIdentity(t *testing.T) {
	fixedTime := time.Date(2026, time.April, 15, 12, 0, 0, 0, time.UTC)

	mc := &mockCollector{
		nowFn: func() time.Time { return fixedTime },
		goos:  "linux",
		hostInfoFn: func(ctx context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{
				Hostname:        "nas",
				HostID:          "machine-id-1",
				Platform:        "linux",
				PlatformFamily:  "linux",
				PlatformVersion: "",
				KernelVersion:   "4.4.302+",
				KernelArch:      "x86_64",
			}, nil
		},
		hostUptimeFn: func(context.Context) (uint64, error) {
			return 3600, nil
		},
		readFileFn: func(name string) ([]byte, error) {
			switch name {
			case "/etc.defaults/VERSION":
				return []byte(`majorversion="7"
minorversion="2"
productversion="7.2.2"
buildnumber="72806"
smallfixnumber="3"
`), nil
			case "/etc/machine-id":
				return []byte("0123456789abcdef0123456789abcdef\n"), nil
			default:
				return nil, os.ErrNotExist
			}
		},
		metricsFn: func(ctx context.Context, diskExclude []string) (hostmetrics.Snapshot, error) {
			return hostmetrics.Snapshot{
				Memory: agentshost.MemoryMetric{
					TotalBytes: 1024,
					UsedBytes:  512,
					FreeBytes:  512,
					Usage:      50,
				},
			}, nil
		},
	}

	agent, err := New(Config{
		APIToken:  "token",
		AgentID:   "agent-1",
		LogLevel:  -1,
		Collector: mc,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	report, err := agent.buildReport(context.Background())
	if err != nil {
		t.Fatalf("buildReport() failed: %v", err)
	}

	if report.Host.OSName != "Synology DSM" {
		t.Fatalf("Host.OSName = %q, want %q", report.Host.OSName, "Synology DSM")
	}
	if report.Host.OSVersion != "7.2.2-72806 Update 3" {
		t.Fatalf("Host.OSVersion = %q, want %q", report.Host.OSVersion, "7.2.2-72806 Update 3")
	}
}
