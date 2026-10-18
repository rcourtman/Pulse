package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// globalDefaultThresholds lists every pointer threshold the thresholds page
// edits in a Global Defaults row.
func globalDefaultThresholds(config *AlertConfig) map[string]**HysteresisThreshold {
	return map[string]**HysteresisThreshold{
		"guest.cpu":                &config.GuestDefaults.CPU,
		"guest.memory":             &config.GuestDefaults.Memory,
		"guest.disk":               &config.GuestDefaults.Disk,
		"guest.diskRead":           &config.GuestDefaults.DiskRead,
		"guest.diskWrite":          &config.GuestDefaults.DiskWrite,
		"guest.networkIn":          &config.GuestDefaults.NetworkIn,
		"guest.networkOut":         &config.GuestDefaults.NetworkOut,
		"node.cpu":                 &config.NodeDefaults.CPU,
		"node.memory":              &config.NodeDefaults.Memory,
		"node.disk":                &config.NodeDefaults.Disk,
		"node.temperature":         &config.NodeDefaults.Temperature,
		"agent.cpu":                &config.AgentDefaults.CPU,
		"agent.memory":             &config.AgentDefaults.Memory,
		"agent.disk":               &config.AgentDefaults.Disk,
		"agent.diskTemperature":    &config.AgentDefaults.DiskTemperature,
		"pbs.cpu":                  &config.PBSDefaults.CPU,
		"pbs.memory":               &config.PBSDefaults.Memory,
		"kubernetes.cpu":           &config.KubernetesDefaults.CPU,
		"kubernetes.memory":        &config.KubernetesDefaults.Memory,
		"kubernetes.disk":          &config.KubernetesDefaults.Disk,
		"kubernetes.diskRead":      &config.KubernetesDefaults.DiskRead,
		"kubernetes.diskWrite":     &config.KubernetesDefaults.DiskWrite,
		"kubernetes.networkIn":     &config.KubernetesDefaults.NetworkIn,
		"kubernetes.networkOut":    &config.KubernetesDefaults.NetworkOut,
		"truenas.cpu":              &config.TrueNASDefaults.CPU,
		"truenas.memory":           &config.TrueNASDefaults.Memory,
		"truenas.disk":             &config.TrueNASDefaults.Disk,
		"truenas.usage":            &config.TrueNASDefaults.Usage,
		"truenas.temperature":      &config.TrueNASDefaults.Temperature,
		"truenas.diskRead":         &config.TrueNASDefaults.DiskRead,
		"truenas.diskWrite":        &config.TrueNASDefaults.DiskWrite,
		"truenas.networkIn":        &config.TrueNASDefaults.NetworkIn,
		"truenas.networkOut":       &config.TrueNASDefaults.NetworkOut,
		"truenas.disk.temperature": &config.TrueNASDiskDefaults.Temperature,
		"vmware.cpu":               &config.VMwareDefaults.CPU,
		"vmware.memory":            &config.VMwareDefaults.Memory,
		"vmware.disk":              &config.VMwareDefaults.Disk,
		"vmware.usage":             &config.VMwareDefaults.Usage,
		"vmware.diskRead":          &config.VMwareDefaults.DiskRead,
		"vmware.diskWrite":         &config.VMwareDefaults.DiskWrite,
		"vmware.networkIn":         &config.VMwareDefaults.NetworkIn,
		"vmware.networkOut":        &config.VMwareDefaults.NetworkOut,
	}
}

// The thresholds page saves a Global Defaults Off as trigger 0. Normalization
// reads a negative trigger as unset and restores the factory threshold, so 0
// is the value that has to survive it in every section.
func TestUpdateConfigKeepsZeroTriggerGlobalDefaultsOff(t *testing.T) {
	m := newTestManager(t)
	config := m.GetConfig()
	for _, field := range globalDefaultThresholds(&config) {
		*field = &HysteresisThreshold{Trigger: 0, Clear: 0}
	}
	config.DockerDefaults.CPU = HysteresisThreshold{Trigger: 0, Clear: 0}
	config.DockerDefaults.Memory = HysteresisThreshold{Trigger: 0, Clear: 0}
	config.DockerDefaults.Disk = HysteresisThreshold{Trigger: 0, Clear: 0}
	config.StorageDefault = HysteresisThreshold{Trigger: 0, Clear: 0}

	m.UpdateConfig(config)

	got := m.GetConfig()
	for name, field := range globalDefaultThresholds(&got) {
		if *field == nil || **field != (HysteresisThreshold{Trigger: 0, Clear: 0}) {
			t.Errorf("%s = %+v after UpdateConfig, want trigger 0 (off)", name, *field)
		}
	}
	for name, threshold := range map[string]HysteresisThreshold{
		"docker.cpu":    got.DockerDefaults.CPU,
		"docker.memory": got.DockerDefaults.Memory,
		"docker.disk":   got.DockerDefaults.Disk,
		"storage":       got.StorageDefault,
	} {
		if threshold != (HysteresisThreshold{Trigger: 0, Clear: 0}) {
			t.Errorf("%s = %+v after UpdateConfig, want trigger 0 (off)", name, threshold)
		}
	}
}

func TestCheckHostRaisesNoMetricAlertForAgentDefaultsSavedOff(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()
	config := m.GetConfig()
	config.AgentDefaults.CPU = &HysteresisThreshold{Trigger: 0, Clear: 0}
	config.AgentDefaults.Memory = &HysteresisThreshold{Trigger: 0, Clear: 0}
	config.AgentDefaults.Disk = &HysteresisThreshold{Trigger: 0, Clear: 0}
	m.UpdateConfig(config)
	// UpdateConfig restores the default delay; without one a breach fires on
	// the first check, so a missing alert means the metric is off.
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	host := models.Host{
		ID:              "host-off",
		DisplayName:     "Off Host",
		Hostname:        "host-off.example",
		Platform:        "linux",
		CPUUsage:        99,
		CPUCount:        8,
		Memory:          models.Memory{Usage: 99, Total: 16384, Used: 16220, Free: 164},
		Disks:           []models.Disk{{Mountpoint: "/", Usage: 99, Total: 100, Used: 99, Free: 1}},
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Now(),
	}
	m.CheckHost(host)

	resourceID := hostResourceID(host.ID)
	diskResourceID, _ := hostDiskResourceID(host, host.Disks[0])
	for _, alertID := range []string{
		canonicalMetricStateID(resourceID, "cpu"),
		canonicalMetricStateID(resourceID, "memory"),
		canonicalMetricStateID(diskResourceID, "disk"),
	} {
		if _, exists := testLookupActiveAlert(t, m, alertID); exists {
			t.Errorf("alert %q fired although its agent default was saved off", alertID)
		}
	}
}
