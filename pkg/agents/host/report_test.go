package host

import (
	"encoding/json"
	"testing"
	"time"
)

func TestReport_JSONMarshal(t *testing.T) {
	report := Report{
		Agent: AgentInfo{
			ID:              "agent-1",
			Version:         "1.0.0",
			Type:            "unified",
			IntervalSeconds: 30,
			Hostname:        "myhost",
		},
		Host: HostInfo{
			ID:            "host-1",
			Hostname:      "myhost",
			DisplayName:   "My Host",
			Platform:      "linux",
			CPUCount:      4,
			UptimeSeconds: 86400,
			LoadAverage:   []float64{0.5, 0.7, 0.9},
		},
		Metrics: Metrics{
			CPUUsagePercent: 25.5,
			Memory: MemoryMetric{
				TotalBytes: 16000000000,
				UsedBytes:  8000000000,
				FreeBytes:  8000000000,
				Usage:      50.0,
			},
		},
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Failed to marshal Report: %v", err)
	}

	var decoded Report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal Report: %v", err)
	}

	if decoded.Agent.ID != report.Agent.ID {
		t.Errorf("Agent.ID = %q, want %q", decoded.Agent.ID, report.Agent.ID)
	}
	if decoded.Host.Hostname != report.Host.Hostname {
		t.Errorf("Host.Hostname = %q, want %q", decoded.Host.Hostname, report.Host.Hostname)
	}
	if decoded.Metrics.CPUUsagePercent != report.Metrics.CPUUsagePercent {
		t.Errorf("Metrics.CPUUsagePercent = %f, want %f", decoded.Metrics.CPUUsagePercent, report.Metrics.CPUUsagePercent)
	}
}

func TestAgentInfo_Fields(t *testing.T) {
	agent := AgentInfo{
		ID:              "agent-123",
		Version:         "2.0.0",
		Type:            "host",
		IntervalSeconds: 60,
		Hostname:        "server1",
	}

	if agent.ID != "agent-123" {
		t.Errorf("ID = %q, want agent-123", agent.ID)
	}
	if agent.Version != "2.0.0" {
		t.Errorf("Version = %q, want 2.0.0", agent.Version)
	}
	if agent.Type != "host" {
		t.Errorf("Type = %q, want host", agent.Type)
	}
	if agent.IntervalSeconds != 60 {
		t.Errorf("IntervalSeconds = %d, want 60", agent.IntervalSeconds)
	}
	if agent.Hostname != "server1" {
		t.Errorf("Hostname = %q, want server1", agent.Hostname)
	}
}

func TestHostInfo_Fields(t *testing.T) {
	host := HostInfo{
		ID:            "host-456",
		Hostname:      "myserver",
		DisplayName:   "My Server",
		MachineID:     "abc123",
		Platform:      "linux",
		OSName:        "Ubuntu",
		OSVersion:     "22.04",
		KernelVersion: "5.15.0",
		Architecture:  "x86_64",
		CPUModel:      "Intel Core i7",
		CPUCount:      8,
		UptimeSeconds: 172800,
		LoadAverage:   []float64{1.0, 0.8, 0.6},
	}

	if host.ID != "host-456" {
		t.Errorf("ID = %q, want host-456", host.ID)
	}
	if host.Platform != "linux" {
		t.Errorf("Platform = %q, want linux", host.Platform)
	}
	if host.CPUCount != 8 {
		t.Errorf("CPUCount = %d, want 8", host.CPUCount)
	}
	if len(host.LoadAverage) != 3 {
		t.Errorf("LoadAverage length = %d, want 3", len(host.LoadAverage))
	}
}

func TestMemoryMetric_Fields(t *testing.T) {
	mem := MemoryMetric{
		TotalBytes: 32000000000,
		UsedBytes:  16000000000,
		FreeBytes:  16000000000,
		Usage:      50.0,
		SwapTotal:  8000000000,
		SwapUsed:   2000000000,
	}

	if mem.TotalBytes != 32000000000 {
		t.Errorf("TotalBytes = %d, want 32000000000", mem.TotalBytes)
	}
	if mem.Usage != 50.0 {
		t.Errorf("Usage = %f, want 50.0", mem.Usage)
	}
	if mem.SwapTotal != 8000000000 {
		t.Errorf("SwapTotal = %d, want 8000000000", mem.SwapTotal)
	}
}

func TestDisk_Fields(t *testing.T) {
	disk := Disk{
		Device:     "/dev/sda1",
		Mountpoint: "/",
		Filesystem: "ext4",
		Type:       "ext4",
		TotalBytes: 500000000000,
		UsedBytes:  250000000000,
		FreeBytes:  250000000000,
		Usage:      50.0,
	}

	if disk.Device != "/dev/sda1" {
		t.Errorf("Device = %q, want /dev/sda1", disk.Device)
	}
	if disk.Mountpoint != "/" {
		t.Errorf("Mountpoint = %q, want /", disk.Mountpoint)
	}
	if disk.Usage != 50.0 {
		t.Errorf("Usage = %f, want 50.0", disk.Usage)
	}
}

func TestNetworkInterface_Fields(t *testing.T) {
	speed := int64(1000)
	net := NetworkInterface{
		Name:      "eth0",
		MAC:       "00:11:22:33:44:55",
		Addresses: []string{"192.168.1.100", "fe80::1"},
		RXBytes:   1000000,
		TXBytes:   500000,
		SpeedMbps: &speed,
	}

	if net.Name != "eth0" {
		t.Errorf("Name = %q, want eth0", net.Name)
	}
	if net.MAC != "00:11:22:33:44:55" {
		t.Errorf("MAC = %q, want 00:11:22:33:44:55", net.MAC)
	}
	if len(net.Addresses) != 2 {
		t.Errorf("Addresses length = %d, want 2", len(net.Addresses))
	}
	if *net.SpeedMbps != 1000 {
		t.Errorf("SpeedMbps = %d, want 1000", *net.SpeedMbps)
	}
}

func TestSensors_Fields(t *testing.T) {
	sensors := Sensors{
		TemperatureCelsius: map[string]float64{
			"cpu_package": 45.0,
			"cpu_core_0":  43.0,
			"cpu_core_1":  44.0,
		},
		FanRPM: map[string]float64{
			"cpu_fan": 1200.0,
		},
		Additional: map[string]float64{
			"voltage": 12.0,
		},
	}

	if len(sensors.TemperatureCelsius) != 3 {
		t.Errorf("TemperatureCelsius count = %d, want 3", len(sensors.TemperatureCelsius))
	}
	if sensors.TemperatureCelsius["cpu_package"] != 45.0 {
		t.Errorf("cpu_package temp = %f, want 45.0", sensors.TemperatureCelsius["cpu_package"])
	}
	if sensors.FanRPM["cpu_fan"] != 1200.0 {
		t.Errorf("cpu_fan RPM = %f, want 1200.0", sensors.FanRPM["cpu_fan"])
	}
}

func TestRAIDArray_Fields(t *testing.T) {
	array := RAIDArray{
		Device:         "/dev/md0",
		Name:           "myarray",
		Level:          "raid1",
		State:          "clean",
		TotalDevices:   2,
		ActiveDevices:  2,
		WorkingDevices: 2,
		FailedDevices:  0,
		SpareDevices:   0,
		UUID:           "12345678:90abcdef:12345678:90abcdef",
		RebuildPercent: 0,
		RebuildSpeed:   "",
		Devices: []RAIDDevice{
			{Device: "/dev/sda1", State: "active sync", Slot: 0},
			{Device: "/dev/sdb1", State: "active sync", Slot: 1},
		},
	}

	if array.Device != "/dev/md0" {
		t.Errorf("Device = %q, want /dev/md0", array.Device)
	}
	if array.Level != "raid1" {
		t.Errorf("Level = %q, want raid1", array.Level)
	}
	if array.State != "clean" {
		t.Errorf("State = %q, want clean", array.State)
	}
	if len(array.Devices) != 2 {
		t.Errorf("Devices count = %d, want 2", len(array.Devices))
	}
}

func TestRAIDDevice_Fields(t *testing.T) {
	device := RAIDDevice{
		Device: "/dev/sda1",
		State:  "active sync",
		Slot:   0,
	}

	if device.Device != "/dev/sda1" {
		t.Errorf("Device = %q, want /dev/sda1", device.Device)
	}
	if device.State != "active sync" {
		t.Errorf("State = %q, want active sync", device.State)
	}
	if device.Slot != 0 {
		t.Errorf("Slot = %d, want 0", device.Slot)
	}
}

func TestRAIDArray_Degraded(t *testing.T) {
	array := RAIDArray{
		Device:         "/dev/md1",
		Level:          "raid5",
		State:          "clean, degraded",
		TotalDevices:   3,
		ActiveDevices:  2,
		WorkingDevices: 2,
		FailedDevices:  1,
		SpareDevices:   0,
		Devices: []RAIDDevice{
			{Device: "/dev/sda1", State: "active sync", Slot: 0},
			{Device: "/dev/sdb1", State: "faulty", Slot: -1},
			{Device: "/dev/sdc1", State: "active sync", Slot: 2},
		},
	}

	if array.FailedDevices != 1 {
		t.Errorf("FailedDevices = %d, want 1", array.FailedDevices)
	}
	if array.State != "clean, degraded" {
		t.Errorf("State = %q, want 'clean, degraded'", array.State)
	}
}

func TestRAIDArray_Rebuilding(t *testing.T) {
	array := RAIDArray{
		Device:         "/dev/md2",
		Level:          "raid6",
		State:          "active, recovering",
		RebuildPercent: 42.5,
		RebuildSpeed:   "50000K/sec",
	}

	if array.RebuildPercent != 42.5 {
		t.Errorf("RebuildPercent = %f, want 42.5", array.RebuildPercent)
	}
	if array.RebuildSpeed != "50000K/sec" {
		t.Errorf("RebuildSpeed = %q, want 50000K/sec", array.RebuildSpeed)
	}
}

func TestReport_WithDisksAndNetwork(t *testing.T) {
	report := Report{
		Disks: []Disk{
			{Device: "/dev/sda1", Mountpoint: "/", TotalBytes: 500000000000},
			{Device: "/dev/sdb1", Mountpoint: "/data", TotalBytes: 1000000000000},
		},
		Network: []NetworkInterface{
			{Name: "eth0", RXBytes: 1000000, TXBytes: 500000},
			{Name: "eth1", RXBytes: 2000000, TXBytes: 1000000},
		},
	}

	if len(report.Disks) != 2 {
		t.Errorf("Disks count = %d, want 2", len(report.Disks))
	}
	if len(report.Network) != 2 {
		t.Errorf("Network count = %d, want 2", len(report.Network))
	}
}

func TestReport_WithSensorsAndRAID(t *testing.T) {
	report := Report{
		Sensors: Sensors{
			TemperatureCelsius: map[string]float64{"cpu": 45.0},
		},
		RAID: []RAIDArray{
			{Device: "/dev/md0", Level: "raid1", State: "clean"},
		},
	}

	if len(report.Sensors.TemperatureCelsius) != 1 {
		t.Errorf("TemperatureCelsius count = %d, want 1", len(report.Sensors.TemperatureCelsius))
	}
	if len(report.RAID) != 1 {
		t.Errorf("RAID count = %d, want 1", len(report.RAID))
	}
}

func TestReport_Tags(t *testing.T) {
	report := Report{
		Tags: []string{"production", "web", "us-east"},
	}

	if len(report.Tags) != 3 {
		t.Errorf("Tags count = %d, want 3", len(report.Tags))
	}
	if report.Tags[0] != "production" {
		t.Errorf("Tags[0] = %q, want production", report.Tags[0])
	}
}

func TestReport_JSONOmitEmpty(t *testing.T) {
	// Test that empty fields are omitted in JSON
	report := Report{
		Agent: AgentInfo{ID: "agent-1"},
		Host:  HostInfo{Hostname: "myhost"},
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	jsonStr := string(data)

	// These should not appear in JSON when empty
	if contains(jsonStr, `"disks":[]`) {
		t.Error("Empty disks should be omitted")
	}
	if contains(jsonStr, `"network":[]`) {
		t.Error("Empty network should be omitted")
	}
	if contains(jsonStr, `"tags":[]`) {
		t.Error("Empty tags should be omitted")
	}
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
