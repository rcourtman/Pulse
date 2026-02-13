package hostagent

import (
	"context"
	"errors"
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

	// Test case 5: SMART collection
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
}
