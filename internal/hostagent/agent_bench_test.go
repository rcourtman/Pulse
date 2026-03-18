package hostagent

import (
	"encoding/json"
	"testing"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// BenchmarkReportMarshal measures JSON marshaling of a realistic host report.
func BenchmarkReportMarshal(b *testing.B) {
	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-abc123",
			Version:         "v5.4.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			Hostname:      "production-server-01",
			Platform:      "linux",
			OSName:        "Ubuntu",
			OSVersion:     "22.04",
			KernelVersion: "5.15.0-91-generic",
			Architecture:  "amd64",
			MachineID:     "abc123def456",
			CPUCount:      16,
			UptimeSeconds: 86400 * 30,
			LoadAverage:   []float64{2.5, 1.8, 1.2},
		},
		Metrics: agentshost.Metrics{
			CPUUsagePercent: 42.5,
			Memory: agentshost.MemoryMetric{
				TotalBytes: 68719476736,
				UsedBytes:  34359738368,
				FreeBytes:  34359738368,
				Usage:      50.0,
				SwapTotal:  8589934592,
				SwapUsed:   1073741824,
			},
		},
		Disks: buildSampleDisks(12),
		DiskIO: []agentshost.DiskIO{
			{Device: "sda", ReadBytes: 1e12, WriteBytes: 5e11, ReadOps: 1e6, WriteOps: 5e5},
			{Device: "sdb", ReadBytes: 2e12, WriteBytes: 1e12, ReadOps: 2e6, WriteOps: 1e6},
		},
		Network: []agentshost.NetworkInterface{
			{Name: "eth0", TXBytes: 1e12, RXBytes: 5e12},
			{Name: "eth1", TXBytes: 5e11, RXBytes: 2e12},
		},
		Tags:      []string{"production", "web", "us-east-1"},
		Timestamp: time.Now().UTC(),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(report)
	}
}

// BenchmarkReportMarshal_Parallel measures concurrent report marshaling.
func BenchmarkReportMarshal_Parallel(b *testing.B) {
	report := agentshost.Report{
		Agent: agentshost.AgentInfo{ID: "agent-abc123", Version: "v5.4.0"},
		Host:  agentshost.HostInfo{Hostname: "server-01", Platform: "linux"},
		Disks: buildSampleDisks(8),
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = json.Marshal(report)
		}
	})
}

func buildSampleDisks(n int) []agentshost.Disk {
	disks := make([]agentshost.Disk, n)
	for i := range disks {
		disks[i] = agentshost.Disk{
			Device:     "sda",
			Mountpoint: "/data",
			Filesystem: "ext4",
			TotalBytes: 1e12,
			UsedBytes:  5e11,
			FreeBytes:  5e11,
			Usage:      50.0,
		}
	}
	return disks
}
