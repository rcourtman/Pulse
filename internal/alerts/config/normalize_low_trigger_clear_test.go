package config_test

import (
	"testing"

	alertconfig "github.com/rcourtman/pulse-go-rewrite/internal/alerts/config"
)

// normalizeLikeUpdateConfig runs the threshold normalizers in the order
// alerts.Manager.UpdateConfig runs them.
func normalizeLikeUpdateConfig(cfg *alertconfig.AlertConfig) {
	alertconfig.NormalizeStorageDefaults(cfg)
	alertconfig.NormalizeDockerDefaults(cfg)
	alertconfig.NormalizePBSDefaults(cfg)
	alertconfig.NormalizeNodeDefaults(cfg)
	alertconfig.NormalizeAgentDefaults(cfg)
	alertconfig.NormalizeKubernetesDefaults(cfg)
	alertconfig.NormalizeTrueNASDefaults(cfg)
	alertconfig.NormalizeVMwareDefaults(cfg)
	alertconfig.ValidateHysteresisThresholds(cfg)
}

type defaultThresholdSlot struct {
	name string
	set  func(*alertconfig.AlertConfig, alertconfig.HysteresisThreshold)
	get  func(*alertconfig.AlertConfig) alertconfig.HysteresisThreshold
}

func pointerSlot(name string, field func(*alertconfig.AlertConfig) **alertconfig.HysteresisThreshold) defaultThresholdSlot {
	return defaultThresholdSlot{
		name: name,
		set: func(cfg *alertconfig.AlertConfig, threshold alertconfig.HysteresisThreshold) {
			*field(cfg) = &threshold
		},
		get: func(cfg *alertconfig.AlertConfig) alertconfig.HysteresisThreshold {
			if got := *field(cfg); got != nil {
				return *got
			}
			return alertconfig.HysteresisThreshold{Trigger: -1, Clear: -1}
		},
	}
}

func valueSlot(name string, field func(*alertconfig.AlertConfig) *alertconfig.HysteresisThreshold) defaultThresholdSlot {
	return defaultThresholdSlot{
		name: name,
		set: func(cfg *alertconfig.AlertConfig, threshold alertconfig.HysteresisThreshold) {
			*field(cfg) = threshold
		},
		get: func(cfg *alertconfig.AlertConfig) alertconfig.HysteresisThreshold {
			return *field(cfg)
		},
	}
}

// percentDefaultThresholdSlots lists every global default the thresholds page
// edits as a hysteresis pair that a user can set to a single-digit value.
func percentDefaultThresholdSlots() []defaultThresholdSlot {
	type cfg = alertconfig.AlertConfig
	type th = alertconfig.HysteresisThreshold
	return []defaultThresholdSlot{
		pointerSlot("guest.cpu", func(c *cfg) **th { return &c.GuestDefaults.CPU }),
		pointerSlot("guest.memory", func(c *cfg) **th { return &c.GuestDefaults.Memory }),
		pointerSlot("guest.disk", func(c *cfg) **th { return &c.GuestDefaults.Disk }),
		pointerSlot("node.cpu", func(c *cfg) **th { return &c.NodeDefaults.CPU }),
		pointerSlot("node.memory", func(c *cfg) **th { return &c.NodeDefaults.Memory }),
		pointerSlot("node.temperature", func(c *cfg) **th { return &c.NodeDefaults.Temperature }),
		pointerSlot("pbs.cpu", func(c *cfg) **th { return &c.PBSDefaults.CPU }),
		pointerSlot("pbs.memory", func(c *cfg) **th { return &c.PBSDefaults.Memory }),
		pointerSlot("agent.cpu", func(c *cfg) **th { return &c.AgentDefaults.CPU }),
		pointerSlot("agent.memory", func(c *cfg) **th { return &c.AgentDefaults.Memory }),
		pointerSlot("agent.disk", func(c *cfg) **th { return &c.AgentDefaults.Disk }),
		pointerSlot("agent.diskTemperature", func(c *cfg) **th { return &c.AgentDefaults.DiskTemperature }),
		pointerSlot("kubernetes.cpu", func(c *cfg) **th { return &c.KubernetesDefaults.CPU }),
		pointerSlot("kubernetes.memory", func(c *cfg) **th { return &c.KubernetesDefaults.Memory }),
		pointerSlot("kubernetes.disk", func(c *cfg) **th { return &c.KubernetesDefaults.Disk }),
		pointerSlot("truenas.cpu", func(c *cfg) **th { return &c.TrueNASDefaults.CPU }),
		pointerSlot("truenas.memory", func(c *cfg) **th { return &c.TrueNASDefaults.Memory }),
		pointerSlot("truenas.usage", func(c *cfg) **th { return &c.TrueNASDefaults.Usage }),
		pointerSlot("truenas.temperature", func(c *cfg) **th { return &c.TrueNASDefaults.Temperature }),
		pointerSlot("vmware.cpu", func(c *cfg) **th { return &c.VMwareDefaults.CPU }),
		pointerSlot("vmware.memory", func(c *cfg) **th { return &c.VMwareDefaults.Memory }),
		pointerSlot("vmware.usage", func(c *cfg) **th { return &c.VMwareDefaults.Usage }),
		valueSlot("docker.cpu", func(c *cfg) *th { return &c.DockerDefaults.CPU }),
		valueSlot("docker.memory", func(c *cfg) *th { return &c.DockerDefaults.Memory }),
		valueSlot("docker.disk", func(c *cfg) *th { return &c.DockerDefaults.Disk }),
		valueSlot("storage", func(c *cfg) *th { return &c.StorageDefault }),
	}
}

// The thresholds page sends a positive trigger with clear max(0, trigger-5),
// so a trigger at or below the 5-point margin arrives with clear 0. Every
// default family must keep that clear below the trigger. PBS, node
// temperature and agent defaults used to fall back to the factory clear,
// storing {trigger: 1, clear: 75} for a 1% Machines CPU default.
func TestLowTriggerDefaultsKeepClearBelowTrigger(t *testing.T) {
	cases := []struct {
		in   alertconfig.HysteresisThreshold
		want alertconfig.HysteresisThreshold
	}{
		{alertconfig.HysteresisThreshold{Trigger: 1, Clear: 0}, alertconfig.HysteresisThreshold{Trigger: 1, Clear: 0}},
		{alertconfig.HysteresisThreshold{Trigger: 2.5, Clear: 0}, alertconfig.HysteresisThreshold{Trigger: 2.5, Clear: 0}},
		{alertconfig.HysteresisThreshold{Trigger: 5, Clear: 0}, alertconfig.HysteresisThreshold{Trigger: 5, Clear: 0}},
		// A clear at or above the trigger, such as one stored before this
		// fix, is repaired the same way.
		{alertconfig.HysteresisThreshold{Trigger: 1, Clear: 75}, alertconfig.HysteresisThreshold{Trigger: 1, Clear: 0}},
		{alertconfig.HysteresisThreshold{Trigger: 50, Clear: 60}, alertconfig.HysteresisThreshold{Trigger: 50, Clear: 45}},
		// Off stays off.
		{alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}, alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}},
	}
	for _, slot := range percentDefaultThresholdSlots() {
		for _, tc := range cases {
			cfg := &alertconfig.AlertConfig{}
			slot.set(cfg, tc.in)
			normalizeLikeUpdateConfig(cfg)
			if got := slot.get(cfg); got != tc.want {
				t.Errorf("%s %+v normalized to %+v, want %+v", slot.name, tc.in, got, tc.want)
			}
		}
	}
}

// A clear at or above the trigger that reaches ValidateHysteresisThresholds
// directly is repaired for agent defaults too, as for every other family.
func TestValidateHysteresisThresholdsRepairsAgentDefaults(t *testing.T) {
	cfg := &alertconfig.AlertConfig{
		AgentDefaults: alertconfig.ThresholdConfig{
			CPU:             &alertconfig.HysteresisThreshold{Trigger: 1, Clear: 75},
			Memory:          &alertconfig.HysteresisThreshold{Trigger: 50, Clear: 50},
			Disk:            &alertconfig.HysteresisThreshold{Trigger: 90, Clear: 95},
			DiskTemperature: &alertconfig.HysteresisThreshold{Trigger: 55, Clear: 60},
		},
	}
	alertconfig.ValidateHysteresisThresholds(cfg)
	for name, tc := range map[string]struct {
		got  *alertconfig.HysteresisThreshold
		want alertconfig.HysteresisThreshold
	}{
		"cpu":             {cfg.AgentDefaults.CPU, alertconfig.HysteresisThreshold{Trigger: 1, Clear: 0}},
		"memory":          {cfg.AgentDefaults.Memory, alertconfig.HysteresisThreshold{Trigger: 50, Clear: 45}},
		"disk":            {cfg.AgentDefaults.Disk, alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85}},
		"diskTemperature": {cfg.AgentDefaults.DiskTemperature, alertconfig.HysteresisThreshold{Trigger: 55, Clear: 50}},
	} {
		if !ptrHtEq(tc.got, tc.want) {
			t.Errorf("agent %s = %+v, want %+v", name, tc.got, tc.want)
		}
	}
}

// Per-type disk fill and disk temperature entries keep a positive trigger and
// follow the same clear rule. They used to reset to the factory pair whenever
// the clear was not positive, losing a trigger of 1-5 (the thresholds page
// sends those with clear 0) and an API entry such as {88, 0}. A non-positive
// trigger still resets to the default, because a per-type entry cannot be
// switched off on its own.
func TestPerTypeDiskEntriesKeepTriggerAndClearBelowIt(t *testing.T) {
	maps := map[string]struct {
		set      func(*alertconfig.AlertConfig, string, alertconfig.HysteresisThreshold)
		get      func(*alertconfig.AlertConfig, string) alertconfig.HysteresisThreshold
		defaults map[string]alertconfig.HysteresisThreshold
	}{
		"diskFillByType": {
			set: func(cfg *alertconfig.AlertConfig, key string, th alertconfig.HysteresisThreshold) {
				cfg.DiskFillByType = map[string]alertconfig.HysteresisThreshold{key: th}
			},
			get: func(cfg *alertconfig.AlertConfig, key string) alertconfig.HysteresisThreshold {
				return cfg.DiskFillByType[key]
			},
			defaults: map[string]alertconfig.HysteresisThreshold{
				"nvme": {Trigger: 92, Clear: 87},
				"sata": {Trigger: 90, Clear: 85},
				"hdd":  {Trigger: 85, Clear: 80},
			},
		},
		"diskTempByType": {
			set: func(cfg *alertconfig.AlertConfig, key string, th alertconfig.HysteresisThreshold) {
				cfg.DiskTempByType = map[string]alertconfig.HysteresisThreshold{key: th}
			},
			get: func(cfg *alertconfig.AlertConfig, key string) alertconfig.HysteresisThreshold {
				return cfg.DiskTempByType[key]
			},
			defaults: map[string]alertconfig.HysteresisThreshold{
				"nvme": {Trigger: 70, Clear: 65},
				"sas":  {Trigger: 65, Clear: 60},
				"sata": {Trigger: 55, Clear: 50},
			},
		},
	}
	for mapName, m := range maps {
		for key, factory := range m.defaults {
			for _, tc := range []struct {
				in   alertconfig.HysteresisThreshold
				want alertconfig.HysteresisThreshold
			}{
				{alertconfig.HysteresisThreshold{Trigger: 1, Clear: 0}, alertconfig.HysteresisThreshold{Trigger: 1, Clear: 0}},
				{alertconfig.HysteresisThreshold{Trigger: 5, Clear: 0}, alertconfig.HysteresisThreshold{Trigger: 5, Clear: 0}},
				{alertconfig.HysteresisThreshold{Trigger: 88, Clear: 0}, alertconfig.HysteresisThreshold{Trigger: 88, Clear: 83}},
				{alertconfig.HysteresisThreshold{Trigger: 1, Clear: 75}, alertconfig.HysteresisThreshold{Trigger: 1, Clear: 0}},
				{alertconfig.HysteresisThreshold{Trigger: 60, Clear: 70}, alertconfig.HysteresisThreshold{Trigger: 60, Clear: 55}},
				{alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}, factory},
				{alertconfig.HysteresisThreshold{Trigger: -1, Clear: 0}, factory},
			} {
				cfg := &alertconfig.AlertConfig{}
				m.set(cfg, key, tc.in)
				normalizeLikeUpdateConfig(cfg)
				if got := m.get(cfg, key); got != tc.want {
					t.Errorf("%s[%s] %+v normalized to %+v, want %+v", mapName, key, tc.in, got, tc.want)
				}
			}
		}
	}
}
