package ai

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestAlertThresholdAdapter_Defaults(t *testing.T) {
	a := NewAlertThresholdAdapter(nil)
	if a.GetNodeCPUThreshold() != 80 {
		t.Fatalf("GetNodeCPUThreshold default = %v", a.GetNodeCPUThreshold())
	}
	if a.GetNodeMemoryThreshold() != 85 {
		t.Fatalf("GetNodeMemoryThreshold default = %v", a.GetNodeMemoryThreshold())
	}
	if a.GetGuestMemoryThreshold() != 85 {
		t.Fatalf("GetGuestMemoryThreshold default = %v", a.GetGuestMemoryThreshold())
	}
	if a.GetGuestDiskThreshold() != 90 {
		t.Fatalf("GetGuestDiskThreshold default = %v", a.GetGuestDiskThreshold())
	}
	if a.GetStorageThreshold() != 85 {
		t.Fatalf("GetStorageThreshold default = %v", a.GetStorageThreshold())
	}
}

func TestAlertThresholdAdapter_UsesAlertManagerConfig(t *testing.T) {
	mgr := alerts.NewManager()
	cfg := mgr.GetConfig()
	cfg.NodeDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 70, Clear: 65}
	cfg.NodeDefaults.Memory = &alerts.HysteresisThreshold{Trigger: 75, Clear: 70}
	cfg.GuestDefaults.Memory = &alerts.HysteresisThreshold{Trigger: 72, Clear: 70}
	cfg.GuestDefaults.Disk = &alerts.HysteresisThreshold{Trigger: 91, Clear: 90}
	cfg.StorageDefault.Trigger = 77
	mgr.UpdateConfig(cfg)

	a := NewAlertThresholdAdapter(mgr)
	if a.GetNodeCPUThreshold() != 70 {
		t.Fatalf("GetNodeCPUThreshold = %v", a.GetNodeCPUThreshold())
	}
	if a.GetNodeMemoryThreshold() != 75 {
		t.Fatalf("GetNodeMemoryThreshold = %v", a.GetNodeMemoryThreshold())
	}
	if a.GetGuestMemoryThreshold() != 72 {
		t.Fatalf("GetGuestMemoryThreshold = %v", a.GetGuestMemoryThreshold())
	}
	if a.GetGuestDiskThreshold() != 91 {
		t.Fatalf("GetGuestDiskThreshold = %v", a.GetGuestDiskThreshold())
	}
	if a.GetStorageThreshold() != 77 {
		t.Fatalf("GetStorageThreshold = %v", a.GetStorageThreshold())
	}
}

func TestAlertThresholdAdapter_Fallbacks(t *testing.T) {
	mgr := alerts.NewManager()
	cfg := mgr.GetConfig()

	cfg.NodeDefaults.CPU = nil
	cfg.NodeDefaults.Memory = &alerts.HysteresisThreshold{Trigger: 0, Clear: 0}
	cfg.GuestDefaults.Memory = nil
	cfg.GuestDefaults.Disk = &alerts.HysteresisThreshold{Trigger: 0, Clear: 0}
	cfg.StorageDefault.Trigger = 0

	mgr.UpdateConfig(cfg)

	a := NewAlertThresholdAdapter(mgr)
	if a.GetNodeCPUThreshold() != 80 {
		t.Fatalf("GetNodeCPUThreshold default = %v", a.GetNodeCPUThreshold())
	}
	if a.GetNodeMemoryThreshold() != 85 {
		t.Fatalf("GetNodeMemoryThreshold default = %v", a.GetNodeMemoryThreshold())
	}
	if a.GetGuestMemoryThreshold() != 85 {
		t.Fatalf("GetGuestMemoryThreshold default = %v", a.GetGuestMemoryThreshold())
	}
	if a.GetGuestDiskThreshold() != 90 {
		t.Fatalf("GetGuestDiskThreshold default = %v", a.GetGuestDiskThreshold())
	}
	if a.GetStorageThreshold() != 85 {
		t.Fatalf("GetStorageThreshold default = %v", a.GetStorageThreshold())
	}
}

func TestAlertThresholdAdapter_DiskTemperatureFollowsAlertPolicy(t *testing.T) {
	if trigger, clear := NewAlertThresholdAdapter(nil).GetDiskTemperatureThreshold(alerts.DiskTemperatureHost{}, "nvme"); trigger != 70 || clear != 65 {
		t.Fatalf("no manager: nvme = %v/%v, want the factory 70/65", trigger, clear)
	}

	mgr := alerts.NewManager()
	cfg := mgr.GetConfig()
	cfg.DiskTempByType["nvme"] = alerts.HysteresisThreshold{Trigger: 75, Clear: 70}
	mgr.UpdateConfig(cfg)
	a := NewAlertThresholdAdapter(mgr)
	if trigger, clear := a.GetDiskTemperatureThreshold(alerts.DiskTemperatureHost{}, "nvme"); trigger != 75 || clear != 70 {
		t.Fatalf("raised nvme = %v/%v, want 75/70", trigger, clear)
	}
	if trigger, clear := a.GetDiskTemperatureThreshold(alerts.DiskTemperatureHost{}, "sata"); trigger != 55 || clear != 50 {
		t.Fatalf("sata = %v/%v, want 55/50", trigger, clear)
	}

	cfg = mgr.GetConfig()
	cfg.AgentDefaults.DiskTemperature = &alerts.HysteresisThreshold{Trigger: 0, Clear: 0}
	mgr.UpdateConfig(cfg)
	if trigger, clear := a.GetDiskTemperatureThreshold(alerts.DiskTemperatureHost{}, "nvme"); trigger != 0 || clear != 0 {
		t.Fatalf("disk temperature alerting off: nvme = %v/%v, want 0/0", trigger, clear)
	}
}

// A TrueNAS disk reaches Patrol through its own alert tiers: its override,
// then the TrueNAS-wide value, then the per-type policy.
func TestAlertThresholdAdapter_TrueNASDiskTemperatureFollowsItsAlertTiers(t *testing.T) {
	if trigger, clear := NewAlertThresholdAdapter(nil).GetTrueNASDiskTemperatureThreshold("disk-any", "nvme"); trigger != 70 || clear != 65 {
		t.Fatalf("no manager: nvme = %v/%v, want the factory 70/65", trigger, clear)
	}

	mgr := alerts.NewManager()
	a := NewAlertThresholdAdapter(mgr)
	if trigger, clear := a.GetTrueNASDiskTemperatureThreshold("disk-plain", "sata"); trigger != 55 || clear != 50 {
		t.Fatalf("no TrueNAS-wide value: sata = %v/%v, want the per-type 55/50", trigger, clear)
	}

	cfg := mgr.GetConfig()
	cfg.TrueNASDiskDefaults.Temperature = &alerts.HysteresisThreshold{Trigger: 62, Clear: 57}
	cfg.Overrides = map[string]alerts.ThresholdConfig{
		"disk-raised": {Temperature: &alerts.HysteresisThreshold{Trigger: 75, Clear: 70}},
		"disk-off":    {Disabled: true},
	}
	mgr.UpdateConfig(cfg)
	for id, want := range map[string][2]float64{
		"disk-raised": {75, 70},
		"disk-plain":  {62, 57},
		"disk-off":    {0, 0},
	} {
		trigger, clear := a.GetTrueNASDiskTemperatureThreshold(id, "nvme")
		if trigger != want[0] || clear != want[1] {
			t.Errorf("%s nvme = %v/%v, want %v/%v", id, trigger, clear, want[0], want[1])
		}
	}
}

// A host's Disk Temp override reaches Patrol for that host's disks only, and a
// host whose alerts are switched off has no disk heat policy at all.
func TestAlertThresholdAdapter_DiskTemperatureHonoursHostOverrides(t *testing.T) {
	mgr := alerts.NewManager()
	cfg := mgr.GetConfig()
	cfg.Overrides = map[string]alerts.ThresholdConfig{
		"host-raised": {DiskTemperature: &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}},
		"host-off":    {Disabled: true},
	}
	mgr.UpdateConfig(cfg)
	a := NewAlertThresholdAdapter(mgr)

	for host, want := range map[string][2]float64{
		"host-raised": {80, 75},
		"host-plain":  {70, 65},
		"host-off":    {0, 0},
		"":            {70, 65},
	} {
		trigger, clear := a.GetDiskTemperatureThreshold(alerts.DiskTemperatureHost{ID: host}, "nvme")
		if trigger != want[0] || clear != want[1] {
			t.Errorf("host %q nvme = %v/%v, want %v/%v", host, trigger, clear, want[0], want[1])
		}
	}
}
