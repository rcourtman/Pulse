package hostagent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	gohost "github.com/shirou/gopsutil/v4/host"
)

func TestAgentMetricsSMARTProbeAttemptsRetryExplicitSAT(t *testing.T) {
	stubLinuxSysfs(t, []string{"sda"}, nil)

	origRun := smartRunCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
	})
	execLookPath = func(string) (string, error) { return "smartctl", nil }

	var attempts [][]string
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		attempts = append(attempts, append([]string(nil), args...))
		if len(args) >= 2 && args[0] == "-d" && args[1] == "sat" {
			return []byte(smartctlSATTemperatureAttributeJSON), nil
		}
		if len(args) > 0 && args[0] == "-d" {
			return []byte(smartctlNoDataJSON), nil
		}
		return []byte(smartctlUntypedHealthOnlyJSON), nil
	}

	result, err := collectSMARTTarget(context.Background(), smartctlTarget{Path: "/dev/sda"})
	if err != nil {
		t.Fatalf("collectSMARTTarget error: %v", err)
	}
	if len(attempts) != 2 {
		t.Fatalf("expected untyped attempt then explicit SAT retry, got %v", attempts)
	}
	if attempts[0][0] == "-d" {
		t.Fatalf("first attempt should use smartctl auto-detection, got %v", attempts[0])
	}
	if len(attempts[1]) < 2 || attempts[1][0] != "-d" || attempts[1][1] != "sat" {
		t.Fatalf("second attempt should force -d sat, got %v", attempts[1])
	}
	if result == nil || result.Temperature != 32 || result.Type != "sata" {
		t.Fatalf("expected SAT temperature result, got %#v", result)
	}
}

func TestProxmoxLXCConfigPathRemainsLinuxNative(t *testing.T) {
	if got, want := proxmoxLXCConfigPath(100), "/etc/pve/lxc/100.conf"; got != want {
		t.Fatalf("proxmoxLXCConfigPath(100) = %q, want %q", got, want)
	}
}

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
		AgentID:       "agent-123",
		APIToken:      "test-token",
		LogLevel:      -1,
		Collector:     mc,
		AppliedConfig: &agentshost.ConfigFingerprint{Version: "v1", Hash: "sha256:test"},
		UpdateStatus: func() agentupdate.Status {
			return agentupdate.Status{State: agentupdate.UpdateStateIdle, AutoUpdate: true}
		},
		ModuleStatus: func() []agentshost.ModuleStatus {
			return []agentshost.ModuleStatus{{Name: "host", Enabled: true, State: "running", UpdatedAt: fixedTime}}
		},
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
		if report.Agent.AppliedConfig == nil || report.Agent.AppliedConfig.Hash != "sha256:test" {
			t.Fatalf("Agent.AppliedConfig = %+v", report.Agent.AppliedConfig)
		}
		if report.Agent.Update == nil || report.Agent.Update.State != agentupdate.UpdateStateIdle || !report.Agent.Update.AutoUpdate {
			t.Fatalf("Agent.Update = %+v", report.Agent.Update)
		}
		if len(report.Agent.Modules) != 1 || report.Agent.Modules[0].Name != "host" || report.Agent.Modules[0].State != "running" {
			t.Fatalf("Agent.Modules = %+v", report.Agent.Modules)
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

	t.Run("RAID collection preserves QNAP sparse role map health", func(t *testing.T) {
		mc.raidArraysFn = func(ctx context.Context) ([]agentshost.RAIDArray, error) {
			return parseMDStatArrays(`md13 : active raid1 sdb4[25] sda4[24]
      458880 blocks super 1.0 [24/2] [UU______________________]
      bitmap: 1/1 pages [4KB], 65536KB chunk`), nil
		}

		report, err := agent.buildReport(context.Background())
		if err != nil {
			t.Fatalf("buildReport failed: %v", err)
		}
		if len(report.RAID) != 1 {
			t.Fatalf("expected 1 RAID array, got %d", len(report.RAID))
		}
		array := report.RAID[0]
		if array.Device != "/dev/md13" || array.State != "active" {
			t.Fatalf("unexpected QNAP RAID array summary: %+v", array)
		}
		if array.TotalDevices != 2 || array.ActiveDevices != 2 || array.WorkingDevices != 2 || array.FailedDevices != 0 {
			t.Fatalf("unexpected QNAP RAID array counts: %+v", array)
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
		mc.smartLocalFn = func(_ context.Context, _ []string, _ *agentshost.UnraidStorage) ([]DiskSMART, error) {
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
		mc.smartLocalFn = func(_ context.Context, _ []string, _ *agentshost.UnraidStorage) ([]DiskSMART, error) {
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

func TestBuildReportForwardsExplicitDiskIncludesAndExcludes(t *testing.T) {
	var gotExclude, gotInclude []string
	mc := &mockCollector{
		metricsWithDiskFiltersFn: func(_ context.Context, exclude, include []string) (hostmetrics.Snapshot, error) {
			gotExclude = append([]string(nil), exclude...)
			gotInclude = append([]string(nil), include...)
			return hostmetrics.Snapshot{}, nil
		},
	}
	agent, err := New(Config{
		APIToken:    "token",
		AgentID:     "agent-1",
		LogLevel:    -1,
		Collector:   mc,
		DiskExclude: []string{"/mnt/private"},
		DiskInclude: []string{"/mnt/containers"},
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if _, err := agent.buildReport(context.Background()); err != nil {
		t.Fatalf("buildReport() failed: %v", err)
	}
	if got := strings.Join(gotExclude, ","); got != "/mnt/private" {
		t.Fatalf("disk excludes = %q, want /mnt/private", got)
	}
	if got := strings.Join(gotInclude, ","); got != "/mnt/containers" {
		t.Fatalf("disk includes = %q, want /mnt/containers", got)
	}
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

func TestBuildReportIncludesNVIDIASMITemperaturesWhenLMSensorsUnavailable(t *testing.T) {
	mc := &mockCollector{
		goos:  "linux",
		nowFn: func() time.Time { return time.Date(2026, 6, 30, 8, 0, 0, 0, time.UTC) },
		hostInfoFn: func(ctx context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{
				Hostname: "gpu-node",
				OS:       "linux",
				Platform: "ubuntu",
				HostID:   "gpu-node-id",
			}, nil
		},
		hostUptimeFn: func(context.Context) (uint64, error) {
			return 7200, nil
		},
		metricsFn: func(context.Context, []string) (hostmetrics.Snapshot, error) {
			return hostmetrics.Snapshot{}, nil
		},
		sensorsLocalFn: func(context.Context) (string, error) {
			return "", errors.New("lm-sensors unavailable")
		},
		lookPathFn: func(file string) (string, error) {
			if file == "nvidia-smi" {
				return "/usr/bin/nvidia-smi", nil
			}
			return "", os.ErrNotExist
		},
		commandCombinedOutputFn: func(_ context.Context, name string, arg ...string) (string, error) {
			if name != "/usr/bin/nvidia-smi" {
				t.Fatalf("command name = %q, want /usr/bin/nvidia-smi", name)
			}
			if len(arg) != 2 || arg[0] != "--query-gpu=index,name,temperature.gpu,utilization.gpu,memory.used,memory.total" || arg[1] != "--format=csv,noheader,nounits" {
				t.Fatalf("command args = %#v, want NVIDIA stats query", arg)
			}
			return "0, NVIDIA GeForce RTX 4090, 63, 42, 8192, 24576\n", nil
		},
	}

	agent, err := New(Config{
		AgentID:   "gpu-agent",
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

	if report.Sensors.TemperatureCelsius["gpu_nvidia_0"] != 63 {
		t.Fatalf("NVIDIA GPU temp = %v, want 63", report.Sensors.TemperatureCelsius["gpu_nvidia_0"])
	}
	if len(report.Sensors.GPU) != 1 {
		t.Fatalf("GPU stats = %d, want 1: %+v", len(report.Sensors.GPU), report.Sensors.GPU)
	}
	if report.Sensors.GPU[0].UtilizationPercent == nil || *report.Sensors.GPU[0].UtilizationPercent != 42 {
		t.Fatalf("GPU utilization = %#v, want 42", report.Sensors.GPU[0].UtilizationPercent)
	}
	if report.Sensors.GPU[0].MemoryTotalBytes == nil || *report.Sensors.GPU[0].MemoryTotalBytes != 24576*1024*1024 {
		t.Fatalf("GPU memory total = %#v, want 24576 MiB", report.Sensors.GPU[0].MemoryTotalBytes)
	}
}

func TestBuildReportIncludesNVIDIASMITelemetryOnWindows(t *testing.T) {
	libreHardwareMonitor := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/data.json" {
			t.Fatalf("LibreHardwareMonitor path = %q, want /data.json", request.URL.Path)
		}
		_, _ = response.Write([]byte(`{
			"Children": [{
				"HardwareId": "/intelcpu/0",
				"Children": [{
					"SensorId": "/intelcpu/0/temperature/0",
					"Type": "Temperature",
					"RawValue": 51
				}]
			}, {
				"HardwareId": "/lpc/nct6798d/0",
				"Children": [{
					"SensorId": "/lpc/nct6798d/0/temperature/1",
					"Type": "Temperature",
					"RawValue": 36
				}]
			}]
		}`))
	}))
	t.Cleanup(libreHardwareMonitor.Close)

	mc := &mockCollector{
		goos:  "windows",
		nowFn: func() time.Time { return time.Date(2026, 7, 30, 18, 0, 0, 0, time.UTC) },
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{
				Hostname: "windows-gpu-node",
				OS:       "windows",
				Platform: "Microsoft Windows 11 Pro",
				HostID:   "windows-gpu-node-id",
			}, nil
		},
		hostUptimeFn: func(context.Context) (uint64, error) {
			return 3600, nil
		},
		metricsFn: func(context.Context, []string) (hostmetrics.Snapshot, error) {
			return hostmetrics.Snapshot{}, nil
		},
		lookPathFn: func(file string) (string, error) {
			switch file {
			case "powershell.exe":
				return `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`, nil
			case "nvidia-smi":
				return `C:\Windows\System32\nvidia-smi.exe`, nil
			default:
				return "", os.ErrNotExist
			}
		},
		commandCombinedOutputFn: func(_ context.Context, name string, arg ...string) (string, error) {
			switch name {
			case `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`:
				return `[{"deviceId":"0","friendlyName":"Windows NVMe","busType":"NVMe","mediaType":"SSD","sizeBytes":1000000000000,"temperature":39}]`, nil
			case `C:\Windows\System32\nvidia-smi.exe`:
				if len(arg) != 2 || arg[0] != "--query-gpu=index,name,temperature.gpu,utilization.gpu,memory.used,memory.total" || arg[1] != "--format=csv,noheader,nounits" {
					t.Fatalf("command args = %#v, want NVIDIA stats query", arg)
				}
				return "0, NVIDIA GeForce RTX 3070, 57, 31, 2048, 8192\r\n", nil
			default:
				t.Fatalf("unexpected Windows telemetry command %q", name)
				return "", nil
			}
		},
	}

	agent, err := New(Config{
		AgentID:   "windows-gpu-agent",
		APIToken:  "token",
		LogLevel:  -1,
		Collector: mc,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	agent.libreHardwareMonitorEndpoint = libreHardwareMonitor.URL + "/data.json"

	report, err := agent.buildReport(context.Background())
	if err != nil {
		t.Fatalf("buildReport failed: %v", err)
	}

	if report.Host.Platform != "windows" {
		t.Fatalf("report host platform = %q, want windows", report.Host.Platform)
	}
	if report.Sensors.TemperatureCelsius["gpu_nvidia_0"] != 57 {
		t.Fatalf("Windows NVIDIA GPU temp = %v, want 57", report.Sensors.TemperatureCelsius["gpu_nvidia_0"])
	}
	if report.Sensors.TemperatureCelsius["cpu_lhm_intelcpu_0_temperature_0"] != 51 {
		t.Fatalf("Windows CPU temperature = %+v, want LibreHardwareMonitor reading", report.Sensors.TemperatureCelsius)
	}
	if report.Sensors.TemperatureCelsius["motherboard_lhm_lpc_nct6798d_0_temperature_1"] != 36 {
		t.Fatalf("Windows motherboard temperature = %+v, want LibreHardwareMonitor reading", report.Sensors.TemperatureCelsius)
	}
	if len(report.Sensors.GPU) != 1 {
		t.Fatalf("Windows GPU stats = %d, want 1: %+v", len(report.Sensors.GPU), report.Sensors.GPU)
	}
	if report.Sensors.GPU[0].UtilizationPercent == nil || *report.Sensors.GPU[0].UtilizationPercent != 31 {
		t.Fatalf("Windows GPU utilization = %#v, want 31", report.Sensors.GPU[0].UtilizationPercent)
	}
	if report.Sensors.GPU[0].MemoryTotalBytes == nil || *report.Sensors.GPU[0].MemoryTotalBytes != 8192*1024*1024 {
		t.Fatalf("Windows GPU memory total = %#v, want 8192 MiB", report.Sensors.GPU[0].MemoryTotalBytes)
	}
	if len(report.Sensors.SMART) != 1 ||
		report.Sensors.SMART[0].Device != "PhysicalDisk0" ||
		report.Sensors.SMART[0].Type != "nvme" ||
		report.Sensors.SMART[0].Temperature != 39 {
		t.Fatalf("Windows storage temperatures = %+v, want native PhysicalDisk0 reading", report.Sensors.SMART)
	}
}

func TestBuildReportIncludesProxmoxLXCFilesystems(t *testing.T) {
	const pctPath = "/usr/sbin/pct"
	now := time.Date(2026, 7, 30, 20, 30, 0, 0, time.UTC)
	mc := &mockCollector{
		goos:  "linux",
		nowFn: func() time.Time { return now },
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{
				Hostname: "pve-a",
				HostID:   "pve-a-id",
				Platform: "debian",
			}, nil
		},
		lookPathFn: func(file string) (string, error) {
			if file == "pct" {
				return pctPath, nil
			}
			return "", os.ErrNotExist
		},
		commandCombinedOutputLimitedFn: func(
			_ context.Context,
			_ int,
			name string,
			args ...string,
		) (string, error) {
			if name != pctPath {
				return "", os.ErrNotExist
			}
			switch strings.Join(args, " ") {
			case "list":
				return fmt.Sprintf(
					"%-10s %-10s %-12s %-20s\n%-10d %-10s %-12s %-20s\n",
					"VMID", "Status", "Lock", "Name",
					200, "running", "", "app",
				), nil
			case "df 200":
				return "MP Volume Size Used Avail Use% Path\nrootfs local:subvol-200-disk-0 20.0G 5.0G 15.0G 25.0 /\n", nil
			default:
				return "", fmt.Errorf("unexpected pct args: %v", args)
			}
		},
	}
	agent, err := New(Config{
		AgentID:   "pve-a-agent",
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
	if report.ProxmoxLXC == nil || len(report.ProxmoxLXC.Containers) != 1 {
		t.Fatalf("Proxmox LXC inventory = %+v", report.ProxmoxLXC)
	}
	if report.ProxmoxLXC.Containers[0].VMID != 200 ||
		len(report.ProxmoxLXC.Containers[0].Disks) != 1 ||
		report.ProxmoxLXC.Containers[0].Disks[0].Mountpoint != "/" {
		t.Fatalf("Proxmox LXC container = %+v", report.ProxmoxLXC.Containers[0])
	}
	foundModule := false
	for _, module := range report.Agent.Modules {
		if module.Name == "proxmox-lxc-filesystems" && module.State == "running" {
			foundModule = true
		}
	}
	if !foundModule {
		t.Fatalf("agent modules = %+v, want Proxmox LXC filesystem module", report.Agent.Modules)
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

type legacyCommandOutputCollector struct {
	SystemCollector
	output string
	err    error
}

func (c *legacyCommandOutputCollector) CommandCombinedOutput(
	context.Context,
	string,
	...string,
) (string, error) {
	return c.output, c.err
}

func TestCollectCommandOutputLimitedKeepsLegacyCollectorsCompatible(t *testing.T) {
	collector := &legacyCommandOutputCollector{output: "123456"}
	output, err := collectCommandOutputLimited(
		context.Background(),
		collector,
		4,
		"example-command",
	)
	if err == nil {
		t.Fatal("expected oversized legacy collector output to fail")
	}
	if output != "1234" {
		t.Fatalf("bounded output = %q, want %q", output, "1234")
	}
}
