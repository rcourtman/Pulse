package alerts

import (
	"sort"
	"testing"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func unifiedEvalBaseConfig() AlertConfig {
	return AlertConfig{
		Enabled:         true,
		ActivationState: ActivationActive,
		GuestDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:   &HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		NodeDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:   &HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		AgentDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:   &HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		PBSDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
		},
		KubernetesDefaults: ThresholdConfig{
			CPU:        &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory:     &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:       &HysteresisThreshold{Trigger: 90, Clear: 85},
			DiskRead:   &HysteresisThreshold{Trigger: 0, Clear: 0},
			DiskWrite:  &HysteresisThreshold{Trigger: 0, Clear: 0},
			NetworkIn:  &HysteresisThreshold{Trigger: 0, Clear: 0},
			NetworkOut: &HysteresisThreshold{Trigger: 0, Clear: 0},
		},
		TrueNASDefaults: ThresholdConfig{
			CPU:         &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory:      &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:        &HysteresisThreshold{Trigger: 85, Clear: 80},
			Usage:       &HysteresisThreshold{Trigger: 85, Clear: 80},
			Temperature: &HysteresisThreshold{Trigger: 80, Clear: 75},
			DiskRead:    &HysteresisThreshold{Trigger: 0, Clear: 0},
			DiskWrite:   &HysteresisThreshold{Trigger: 0, Clear: 0},
			NetworkIn:   &HysteresisThreshold{Trigger: 0, Clear: 0},
			NetworkOut:  &HysteresisThreshold{Trigger: 0, Clear: 0},
		},
		TrueNASDiskDefaults: ThresholdConfig{
			Temperature: &HysteresisThreshold{Trigger: 55, Clear: 50},
		},
		VMwareDefaults: ThresholdConfig{
			CPU:        &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory:     &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:       &HysteresisThreshold{Trigger: 90, Clear: 85},
			Usage:      &HysteresisThreshold{Trigger: 85, Clear: 80},
			DiskRead:   &HysteresisThreshold{Trigger: 0, Clear: 0},
			DiskWrite:  &HysteresisThreshold{Trigger: 0, Clear: 0},
			NetworkIn:  &HysteresisThreshold{Trigger: 0, Clear: 0},
			NetworkOut: &HysteresisThreshold{Trigger: 0, Clear: 0},
		},
		StorageDefault: HysteresisThreshold{Trigger: 85, Clear: 80},
		Overrides:      map[string]ThresholdConfig{},

		// Keep these explicit to make test intent obvious; final values are forced in configureUnifiedEvalManager.
		TimeThresholds:    map[string]int{},
		SuppressionWindow: 0,
		MinimumDelta:      0,
	}
}

func configureUnifiedEvalManager(t *testing.T, m *Manager, cfg AlertConfig) {
	t.Helper()

	m.UpdateConfig(cfg)

	// UpdateConfig normalizes zero values back to defaults; force immediate alerting in tests.
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.MetricTimeThresholds = nil
	m.config.SuppressionWindow = 0
	m.config.MinimumDelta = 0
	m.mu.Unlock()

	m.ClearActiveAlerts()
}

func alertKeys(m *Manager) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.activeAlerts))
	for storageKey, alert := range m.activeAlerts {
		keys = append(keys, effectiveAlertID(alert, storageKey))
	}
	sort.Strings(keys)
	return keys
}

func assertAlertPresent(t *testing.T, m *Manager, alertID string) {
	t.Helper()

	m.mu.RLock()
	_, exists := testLookupActiveAlert(t, m, alertID)
	m.mu.RUnlock()
	if !exists {
		t.Fatalf("expected alert %q to exist, active alerts: %v", alertID, alertKeys(m))
	}
}

func assertAlertMissing(t *testing.T, m *Manager, alertID string) {
	t.Helper()

	m.mu.RLock()
	_, exists := testLookupActiveAlert(t, m, alertID)
	m.mu.RUnlock()
	if exists {
		t.Fatalf("expected alert %q to be absent, active alerts: %v", alertID, alertKeys(m))
	}
}

func TestUnifiedGuestDiskRenotificationRespectsMaxAlertsHour(t *testing.T) {
	m := newTestManager(t)
	cfg := unifiedEvalBaseConfig()
	cfg.GuestDefaults = ThresholdConfig{
		Disk: &HysteresisThreshold{Trigger: 80, Clear: 70},
	}
	cfg.Schedule.Cooldown = 30
	cfg.Schedule.MaxAlertsHour = 1
	configureUnifiedEvalManager(t, m, cfg)

	dispatched := make(chan string, 4)
	m.SetAlertCallback(func(alert *Alert) {
		dispatched <- alert.ID
	})

	container := models.Container{
		ID:     "ct101",
		Name:   "smr",
		Node:   "ryzen5800x",
		Status: "running",
		Disks: []models.Disk{{
			Mountpoint: "/",
			Usage:      90.5,
			Total:      100,
			Used:       90,
			Free:       10,
		}},
	}

	m.CheckGuest(container, "pve1")

	select {
	case <-dispatched:
	case <-time.After(time.Second):
		t.Fatal("expected initial guest disk alert notification")
	}

	alertID := canonicalMetricStateID("ct101-disk-root", "disk")
	lastNotified := time.Now().Add(-31 * time.Minute)
	m.mu.Lock()
	alert, exists := m.getActiveAlertNoLock(alertID)
	if !exists {
		m.mu.Unlock()
		t.Fatalf("expected active guest disk alert %s", alertID)
	}
	alert.LastNotified = &lastNotified
	m.mu.Unlock()

	m.CheckGuest(container, "pve1")

	select {
	case id := <-dispatched:
		t.Fatalf("expected max-alerts/hour to suppress repeated disk notification, got %s", id)
	case <-time.After(250 * time.Millisecond):
	}
}

func TestCheckUnifiedResourceMajorFamilies(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	tests := []struct {
		name    string
		alertID string
		input   *UnifiedResourceInput
	}{
		{
			name:    "VM CPU above threshold creates alert",
			alertID: canonicalMetricStateID("vm-101", "cpu"),
			input: &UnifiedResourceInput{
				ID:   "vm-101",
				Type: "vm",
				Name: "vm-101",
				CPU:  &UnifiedResourceMetric{Percent: 90},
			},
		},
		{
			name:    "System container CPU above threshold creates alert",
			alertID: canonicalMetricStateID("lxc-200", "cpu"),
			input: &UnifiedResourceInput{
				ID:   "lxc-200",
				Type: "system-container",
				Name: "worker-ct",
				CPU:  &UnifiedResourceMetric{Percent: 91},
			},
		},
		{
			name:    "Node memory above threshold creates alert",
			alertID: canonicalMetricStateID("node-a", "memory"),
			input: &UnifiedResourceInput{
				ID:     "node-a",
				Type:   "node",
				Name:   "node-a",
				Memory: &UnifiedResourceMetric{Percent: 90},
			},
		},
		{
			name:    "Agent disk above threshold creates alert",
			alertID: canonicalMetricStateID("host-1", "disk"),
			input: &UnifiedResourceInput{
				ID:   "host-1",
				Type: "agent",
				Name: "host-1",
				Disk: &UnifiedResourceMetric{Percent: 95},
			},
		},
		{
			name:    "Storage usage above threshold creates alert",
			alertID: canonicalMetricStateID("storage-1", "usage"),
			input: &UnifiedResourceInput{
				ID:   "storage-1",
				Type: "storage",
				Name: "storage-1",
				Disk: &UnifiedResourceMetric{Percent: 92},
			},
		},
		{
			name:    "PBS CPU above threshold creates alert",
			alertID: canonicalMetricStateID("pbs-1", "cpu"),
			input: &UnifiedResourceInput{
				ID:   "pbs-1",
				Type: "pbs",
				Name: "pbs-1",
				CPU:  &UnifiedResourceMetric{Percent: 88},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.ClearActiveAlerts()
			m.CheckUnifiedResource(tt.input)
			assertAlertPresent(t, m, tt.alertID)
		})
	}
}

func TestCheckUnifiedResourceSupportsKubernetesTrueNASAndVMwareMetricTargets(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	tests := []struct {
		name             string
		alertID          string
		input            *UnifiedResourceInput
		wantResourceType string
	}{
		{
			name:    "Kubernetes cluster CPU",
			alertID: canonicalMetricStateID("k8s:prod", "cpu"),
			input: &UnifiedResourceInput{
				ID:   "k8s:prod",
				Type: "k8s-cluster",
				Name: "prod",
				CPU:  &UnifiedResourceMetric{Percent: 88},
			},
			wantResourceType: "Kubernetes Cluster",
		},
		{
			name:    "Kubernetes node memory",
			alertID: canonicalMetricStateID("k8s:prod/node:worker-1", "memory"),
			input: &UnifiedResourceInput{
				ID:     "k8s:prod/node:worker-1",
				Type:   "k8s-node",
				Name:   "worker-1",
				Node:   "worker-1",
				Memory: &UnifiedResourceMetric{Percent: 90},
			},
			wantResourceType: "Kubernetes Node",
		},
		{
			name:    "Kubernetes deployment CPU",
			alertID: canonicalMetricStateID("k8s:prod/ns:default/deployment:api", "cpu"),
			input: &UnifiedResourceInput{
				ID:       "k8s:prod/ns:default/deployment:api",
				Type:     "k8s-deployment",
				Name:     "api",
				Node:     "prod",
				Instance: "prod",
				CPU:      &UnifiedResourceMetric{Percent: 83},
			},
			wantResourceType: "Kubernetes Deployment",
		},
		{
			name:    "Kubernetes pod disk",
			alertID: canonicalMetricStateID("k8s:prod/ns:default/pod:api-7d9f", "disk"),
			input: &UnifiedResourceInput{
				ID:       "k8s:prod/ns:default/pod:api-7d9f",
				Type:     "pod",
				Name:     "api-7d9f",
				Node:     "worker-1",
				Instance: "prod",
				Disk:     &UnifiedResourceMetric{Percent: 93},
			},
			wantResourceType: "Kubernetes Pod",
		},
		{
			name:    "TrueNAS system temperature",
			alertID: canonicalMetricStateID("agent:truenas-main", "temperature"),
			input: &UnifiedResourceInput{
				ID:          "agent:truenas-main",
				Type:        "truenas-system",
				Name:        "truenas-main",
				Node:        "truenas-main",
				Temperature: &UnifiedResourceMetric{Value: 82, Percent: 82},
			},
			wantResourceType: "TrueNAS System",
		},
		{
			name:    "TrueNAS pool usage",
			alertID: canonicalMetricStateID("storage:truenas-main/pool:tank", "usage"),
			input: &UnifiedResourceInput{
				ID:       "storage:truenas-main/pool:tank",
				Type:     "truenas-pool",
				Name:     "tank",
				Node:     "truenas-main",
				Instance: "TrueNAS",
				Disk:     &UnifiedResourceMetric{Percent: 89},
			},
			wantResourceType: "TrueNAS Pool",
		},
		{
			name:    "TrueNAS dataset usage",
			alertID: canonicalMetricStateID("storage:truenas-main/dataset:tank/apps", "usage"),
			input: &UnifiedResourceInput{
				ID:       "storage:truenas-main/dataset:tank/apps",
				Type:     "truenas-dataset",
				Name:     "tank/apps",
				Node:     "truenas-main",
				Instance: "TrueNAS",
				Disk:     &UnifiedResourceMetric{Percent: 91},
			},
			wantResourceType: "TrueNAS Dataset",
		},
		{
			name:    "TrueNAS disk temperature",
			alertID: canonicalMetricStateID("physical-disk:truenas-main/ada0", "temperature"),
			input: &UnifiedResourceInput{
				ID:          "physical-disk:truenas-main/ada0",
				Type:        "truenas-disk",
				Name:        "ada0",
				Node:        "truenas-main",
				Instance:    "TrueNAS",
				Temperature: &UnifiedResourceMetric{Value: 58, Percent: 58},
			},
			wantResourceType: "TrueNAS Disk",
		},
		{
			name:    "vSphere host CPU",
			alertID: canonicalMetricStateID("vmware:vc-1:host:host-101", "cpu"),
			input: &UnifiedResourceInput{
				ID:       "vmware:vc-1:host:host-101",
				Type:     "vmware-host",
				Name:     "esxi-01.lab.local",
				Node:     "Prod Compute",
				Instance: "Lab vCenter",
				CPU:      &UnifiedResourceMetric{Percent: 88},
			},
			wantResourceType: "vSphere Host",
		},
		{
			name:    "vSphere VM disk",
			alertID: canonicalMetricStateID("vmware:vc-1:vm:vm-201", "disk"),
			input: &UnifiedResourceInput{
				ID:       "vmware:vc-1:vm:vm-201",
				Type:     "vmware-vm",
				Name:     "app-01",
				Node:     "esxi-01.lab.local",
				Instance: "Lab vCenter",
				Disk:     &UnifiedResourceMetric{Percent: 94},
			},
			wantResourceType: "vSphere VM",
		},
		{
			name:    "vSphere datastore usage",
			alertID: canonicalMetricStateID("vmware:vc-1:datastore:datastore-301", "usage"),
			input: &UnifiedResourceInput{
				ID:       "vmware:vc-1:datastore:datastore-301",
				Type:     "vmware-datastore",
				Name:     "nvme-primary",
				Node:     "Lab Datacenter",
				Instance: "Lab vCenter",
				Disk:     &UnifiedResourceMetric{Percent: 89},
			},
			wantResourceType: "vSphere Datastore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.ClearActiveAlerts()
			m.CheckUnifiedResource(tt.input)

			alert := activeAlert(t, m, tt.alertID)
			if got := alert.Metadata["resourceType"]; got != tt.wantResourceType {
				t.Fatalf("resourceType metadata = %v, want %s", got, tt.wantResourceType)
			}
		})
	}
}

func TestCheckUnifiedResourceUsesKubernetesTrueNASAndVMwareOverrides(t *testing.T) {
	m := newTestManager(t)
	cfg := unifiedEvalBaseConfig()
	cfg.KubernetesDefaults.CPU = &HysteresisThreshold{Trigger: 95, Clear: 90}
	cfg.TrueNASDefaults.Usage = &HysteresisThreshold{Trigger: 95, Clear: 90}
	cfg.VMwareDefaults.CPU = &HysteresisThreshold{Trigger: 95, Clear: 90}
	cfg.Overrides["k8s:prod/ns:default/pod:api-7d9f"] = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 60, Clear: 55},
	}
	cfg.Overrides["storage:truenas-main/pool:tank"] = ThresholdConfig{
		Usage: &HysteresisThreshold{Trigger: 60, Clear: 55},
	}
	cfg.Overrides["vmware:vc-1:host:host-101"] = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 60, Clear: 55},
	}
	configureUnifiedEvalManager(t, m, cfg)

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "k8s:prod/ns:default/pod:api-7d9f",
		Type: "pod",
		Name: "api-7d9f",
		CPU:  &UnifiedResourceMetric{Percent: 65},
	})
	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "storage:truenas-main/pool:tank",
		Type: "truenas-pool",
		Name: "tank",
		Disk: &UnifiedResourceMetric{Percent: 65},
	})
	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "vmware:vc-1:host:host-101",
		Type: "vmware-host",
		Name: "esxi-01.lab.local",
		CPU:  &UnifiedResourceMetric{Percent: 65},
	})

	k8sAlert := activeAlert(t, m, canonicalMetricStateID("k8s:prod/ns:default/pod:api-7d9f", "cpu"))
	if k8sAlert.Threshold != 60 {
		t.Fatalf("kubernetes override threshold = %v, want 60", k8sAlert.Threshold)
	}
	truenasAlert := activeAlert(t, m, canonicalMetricStateID("storage:truenas-main/pool:tank", "usage"))
	if truenasAlert.Threshold != 60 {
		t.Fatalf("truenas override threshold = %v, want 60", truenasAlert.Threshold)
	}
	vmwareAlert := activeAlert(t, m, canonicalMetricStateID("vmware:vc-1:host:host-101", "cpu"))
	if vmwareAlert.Threshold != 60 {
		t.Fatalf("vmware override threshold = %v, want 60", vmwareAlert.Threshold)
	}
}

func TestUnifiedResourceInputFromKubernetesTrueNASAndVMwareResources(t *testing.T) {
	pod := unifiedresources.Resource{
		ID:         " k8s:prod/ns:default/pod:api-7d9f ",
		Type:       unifiedresources.ResourceTypePod,
		Name:       "api-7d9f",
		ParentName: "worker-1",
		Sources:    []unifiedresources.DataSource{unifiedresources.SourceK8s},
		Kubernetes: &unifiedresources.K8sData{
			ClusterName: "prod",
			NodeName:    "worker-1",
			Namespace:   "default",
		},
		Metrics: &unifiedresources.ResourceMetrics{
			CPU:    &unifiedresources.MetricValue{Percent: 82},
			Memory: &unifiedresources.MetricValue{Percent: 64},
		},
	}
	podInput, ok := UnifiedResourceInputFromResource(pod)
	if !ok {
		t.Fatalf("expected Kubernetes pod resource to become unified alert input")
	}
	if podInput.Type != "pod" || podInput.ID != "k8s:prod/ns:default/pod:api-7d9f" || podInput.Node != "worker-1" || podInput.Instance != "prod" {
		t.Fatalf("unexpected pod input: %+v", podInput)
	}
	if podInput.CPU == nil || podInput.CPU.Percent != 82 {
		t.Fatalf("unexpected pod CPU metric: %+v", podInput.CPU)
	}

	pool := unifiedresources.Resource{
		ID:         "storage:truenas-main/pool:tank",
		Type:       unifiedresources.ResourceTypeStorage,
		Name:       "tank",
		ParentName: "truenas-main",
		Sources:    []unifiedresources.DataSource{unifiedresources.SourceTrueNAS},
		Storage: &unifiedresources.StorageMeta{
			Platform: string(unifiedresources.SourceTrueNAS),
			Topology: "pool",
		},
		Metrics: &unifiedresources.ResourceMetrics{
			Disk: &unifiedresources.MetricValue{Percent: 89},
		},
	}
	poolInput, ok := UnifiedResourceInputFromResource(pool)
	if !ok {
		t.Fatalf("expected TrueNAS pool resource to become unified alert input")
	}
	if poolInput.Type != "truenas-pool" || poolInput.ID != "storage:truenas-main/pool:tank" || poolInput.Node != "truenas-main" || poolInput.Instance != "TrueNAS" {
		t.Fatalf("unexpected pool input: %+v", poolInput)
	}
	if poolInput.Disk == nil || poolInput.Disk.Percent != 89 {
		t.Fatalf("unexpected pool disk metric: %+v", poolInput.Disk)
	}

	temp := 58.0
	disk := unifiedresources.Resource{
		ID:          "physical-disk:truenas-main/ada0",
		Type:        unifiedresources.ResourceTypePhysicalDisk,
		Name:        "ada0",
		ParentName:  "truenas-main",
		Sources:     []unifiedresources.DataSource{unifiedresources.SourceTrueNAS},
		Temperature: &temp,
	}
	diskInput, ok := UnifiedResourceInputFromResource(disk)
	if !ok {
		t.Fatalf("expected TrueNAS disk resource to become unified alert input")
	}
	if diskInput.Type != "truenas-disk" || diskInput.Temperature == nil || diskInput.Temperature.Value != 58 {
		t.Fatalf("unexpected disk input: %+v", diskInput)
	}

	vmwareHost := unifiedresources.Resource{
		ID:      " vmware:vc-1:host:host-101 ",
		Type:    unifiedresources.ResourceTypeAgent,
		Name:    "esxi-01.lab.local",
		Sources: []unifiedresources.DataSource{unifiedresources.SourceVMware},
		VMware: &unifiedresources.VMwareData{
			ConnectionName: "Lab vCenter",
			ClusterName:    "Prod Compute",
			VCenterHost:    "vcenter.lab.local",
		},
		Metrics: &unifiedresources.ResourceMetrics{
			CPU: &unifiedresources.MetricValue{Percent: 82},
		},
	}
	hostInput, ok := UnifiedResourceInputFromResource(vmwareHost)
	if !ok {
		t.Fatalf("expected VMware host resource to become unified alert input")
	}
	if hostInput.Type != "vmware-host" || hostInput.ID != "vmware:vc-1:host:host-101" || hostInput.Node != "Prod Compute" || hostInput.Instance != "Lab vCenter" {
		t.Fatalf("unexpected VMware host input: %+v", hostInput)
	}

	vmwareVM := unifiedresources.Resource{
		ID:         "vmware:vc-1:vm:vm-201",
		Type:       unifiedresources.ResourceTypeVM,
		Name:       "app-01",
		ParentName: "esxi-01.lab.local",
		Sources:    []unifiedresources.DataSource{unifiedresources.SourceVMware},
		VMware: &unifiedresources.VMwareData{
			ConnectionName:  "Lab vCenter",
			RuntimeHostName: "esxi-01.lab.local",
		},
		Metrics: &unifiedresources.ResourceMetrics{
			Disk: &unifiedresources.MetricValue{Percent: 94},
		},
	}
	vmInput, ok := UnifiedResourceInputFromResource(vmwareVM)
	if !ok {
		t.Fatalf("expected VMware VM resource to become unified alert input")
	}
	if vmInput.Type != "vmware-vm" || vmInput.Node != "esxi-01.lab.local" || vmInput.Instance != "Lab vCenter" || vmInput.Disk == nil || vmInput.Disk.Percent != 94 {
		t.Fatalf("unexpected VMware VM input: %+v", vmInput)
	}

	vmwareDatastore := unifiedresources.Resource{
		ID:      "vmware:vc-1:datastore:datastore-301",
		Type:    unifiedresources.ResourceTypeStorage,
		Name:    "nvme-primary",
		Sources: []unifiedresources.DataSource{unifiedresources.SourceVMware},
		Storage: &unifiedresources.StorageMeta{
			Platform: "vmware-vsphere",
			Topology: "datastore",
		},
		VMware: &unifiedresources.VMwareData{
			ConnectionName: "Lab vCenter",
			DatacenterName: "Lab Datacenter",
		},
		Metrics: &unifiedresources.ResourceMetrics{
			Disk: &unifiedresources.MetricValue{Percent: 89},
		},
	}
	datastoreInput, ok := UnifiedResourceInputFromResource(vmwareDatastore)
	if !ok {
		t.Fatalf("expected VMware datastore resource to become unified alert input")
	}
	if datastoreInput.Type != "vmware-datastore" || datastoreInput.Node != "Lab Datacenter" || datastoreInput.Instance != "Lab vCenter" {
		t.Fatalf("unexpected VMware datastore input: %+v", datastoreInput)
	}

	vmwareNetwork := unifiedresources.Resource{
		ID:      "vmware:vc-1:network:network-401",
		Type:    unifiedresources.ResourceTypeNetwork,
		Name:    "VM Network",
		Sources: []unifiedresources.DataSource{unifiedresources.SourceVMware},
		VMware: &unifiedresources.VMwareData{
			ConnectionName: "Lab vCenter",
			DatacenterName: "Lab Datacenter",
			EntityType:     "network",
		},
	}
	networkInput, ok := UnifiedResourceInputFromResource(vmwareNetwork)
	if !ok {
		t.Fatalf("expected VMware network resource to become unified alert input")
	}
	if networkInput.Type != "vmware-network" || networkInput.Node != "Lab Datacenter" || networkInput.Instance != "Lab vCenter" {
		t.Fatalf("unexpected VMware network input: %+v", networkInput)
	}
}

func TestCheckUnifiedResourceHonorsKubernetesTrueNASAndVMwareGlobalDisables(t *testing.T) {
	m := newTestManager(t)
	cfg := unifiedEvalBaseConfig()
	cfg.DisableAllKubernetes = true
	cfg.DisableAllTrueNAS = true
	cfg.DisableAllVMware = true
	configureUnifiedEvalManager(t, m, cfg)

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "k8s:prod",
		Type: "k8s-cluster",
		Name: "prod",
		CPU:  &UnifiedResourceMetric{Percent: 90},
	})
	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "storage:truenas-main/pool:tank",
		Type: "truenas-pool",
		Name: "tank",
		Disk: &UnifiedResourceMetric{Percent: 90},
	})
	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "vmware:vc-1:host:host-101",
		Type: "vmware-host",
		Name: "esxi-01.lab.local",
		CPU:  &UnifiedResourceMetric{Percent: 90},
	})

	assertAlertMissing(t, m, canonicalMetricStateID("k8s:prod", "cpu"))
	assertAlertMissing(t, m, canonicalMetricStateID("storage:truenas-main/pool:tank", "usage"))
	assertAlertMissing(t, m, canonicalMetricStateID("vmware:vc-1:host:host-101", "cpu"))
}

func TestCheckUnifiedResourceRejectsLegacyGuestTypeAlias(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "legacy-ct-200",
		Type: "lxc",
		Name: "legacy-ct",
		CPU:  &UnifiedResourceMetric{Percent: 95},
	})

	assertAlertMissing(t, m, canonicalMetricStateID("legacy-ct-200", "cpu"))
}

func TestCheckUnifiedResourceOverrideLowerThresholdCreatesAlert(t *testing.T) {
	m := newTestManager(t)
	cfg := unifiedEvalBaseConfig()
	cfg.Overrides["vm-override"] = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 60, Clear: 55},
	}
	configureUnifiedEvalManager(t, m, cfg)

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "vm-override",
		Type: "vm",
		Name: "vm-override",
		CPU:  &UnifiedResourceMetric{Percent: 65},
	})

	assertAlertPresent(t, m, canonicalMetricStateID("vm-override", "cpu"))
}

func TestResolveResourceThresholdsUsesCephStorageSourceAlias(t *testing.T) {
	m := newTestManager(t)

	m.mu.Lock()
	m.config.StorageDefault = HysteresisThreshold{Trigger: 95, Clear: 90}
	m.config.Overrides = map[string]ThresholdConfig{
		"agent:pve5-ceph-pool-data_replication": {
			Usage: &HysteresisThreshold{Trigger: 50, Clear: 45},
		},
	}
	thresholds := m.resolveResourceThresholds("storage", "pve5-ceph-pool-data_replication")
	m.mu.Unlock()

	if thresholds.Usage == nil {
		t.Fatalf("expected storage usage threshold")
	}
	if thresholds.Usage.Trigger != 50 || thresholds.Usage.Clear != 45 {
		t.Fatalf("usage threshold = %#v, want legacy agent alias override trigger 50 clear 45", thresholds.Usage)
	}
}

func TestCheckUnifiedResourceNilInputNoPanic(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CheckUnifiedResource(nil) panicked: %v", r)
		}
	}()

	m.CheckUnifiedResource(nil)
}

func TestCheckUnifiedResourceDisabledThresholdsNoAlert(t *testing.T) {
	m := newTestManager(t)
	cfg := unifiedEvalBaseConfig()
	cfg.GuestDefaults.Disabled = true
	configureUnifiedEvalManager(t, m, cfg)

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "vm-disabled",
		Type: "vm",
		Name: "vm-disabled",
		CPU:  &UnifiedResourceMetric{Percent: 95},
	})

	assertAlertMissing(t, m, canonicalMetricStateID("vm-disabled", "cpu"))
}

func TestCheckUnifiedResourceAnnotatesMetricAlertsWithCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "vm-annotated",
		Type: "vm",
		Name: "vm-annotated",
		CPU:  &UnifiedResourceMetric{Percent: 90},
	})

	m.mu.RLock()
	alertID := canonicalMetricStateID("vm-annotated", "cpu")
	alert := testRequireActiveAlert(t, m, alertID)
	m.mu.RUnlock()
	if alert == nil {
		t.Fatalf("expected %s alert", alertID)
	}
	if got := alert.Metadata["canonicalAlertKind"]; got != string(alertspecs.AlertSpecKindMetricThreshold) {
		t.Fatalf("canonicalAlertKind = %v, want %s", got, alertspecs.AlertSpecKindMetricThreshold)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != canonicalMetricSpecID("vm-annotated", "cpu") {
		t.Fatalf("canonicalSpecID = %v, want %s", got, canonicalMetricSpecID("vm-annotated", "cpu"))
	}
	if alert.CanonicalSpecID != canonicalMetricSpecID("vm-annotated", "cpu") {
		t.Fatalf("CanonicalSpecID = %q, want %s", alert.CanonicalSpecID, canonicalMetricSpecID("vm-annotated", "cpu"))
	}
	if alert.CanonicalKind != string(alertspecs.AlertSpecKindMetricThreshold) {
		t.Fatalf("CanonicalKind = %q, want %s", alert.CanonicalKind, alertspecs.AlertSpecKindMetricThreshold)
	}
	if alert.CanonicalState != canonicalMetricStateID("vm-annotated", "cpu") {
		t.Fatalf("CanonicalState = %q, want %s", alert.CanonicalState, canonicalMetricStateID("vm-annotated", "cpu"))
	}
}

func TestCheckUnifiedResourceKeepsInstanceScopedNodeDisplayNames(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.UpdateNodeDisplayName("cluster-a", "pve", "Alpha")
	m.UpdateNodeDisplayName("cluster-b", "pve", "Beta")

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:       "vm-a",
		Type:     "vm",
		Name:     "vm-a",
		Node:     "pve",
		Instance: "cluster-a",
		CPU:      &UnifiedResourceMetric{Percent: 90},
	})
	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:       "vm-b",
		Type:     "vm",
		Name:     "vm-b",
		Node:     "pve",
		Instance: "cluster-b",
		CPU:      &UnifiedResourceMetric{Percent: 91},
	})

	m.UpdateNodeDisplayName("cluster-a", "pve", "Alpha Updated")
	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:       "vm-a",
		Type:     "vm",
		Name:     "vm-a",
		Node:     "pve",
		Instance: "cluster-a",
		CPU:      &UnifiedResourceMetric{Percent: 92},
	})

	m.mu.RLock()
	alertA := testRequireActiveAlert(t, m, canonicalMetricStateID("vm-a", "cpu"))
	alertB := testRequireActiveAlert(t, m, canonicalMetricStateID("vm-b", "cpu"))
	m.mu.RUnlock()

	if alertA.NodeDisplayName != "Alpha Updated" {
		t.Fatalf("vm-a node display name = %q, want %q", alertA.NodeDisplayName, "Alpha Updated")
	}
	if alertB.NodeDisplayName != "Beta" {
		t.Fatalf("vm-b node display name = %q, want %q", alertB.NodeDisplayName, "Beta")
	}

	gotByResourceID := make(map[string]Alert)
	for _, alert := range m.GetActiveAlerts() {
		gotByResourceID[alert.ResourceID] = alert
	}
	if gotByResourceID["vm-a"].NodeDisplayName != "Alpha Updated" {
		t.Fatalf("GetActiveAlerts vm-a node display name = %q, want %q", gotByResourceID["vm-a"].NodeDisplayName, "Alpha Updated")
	}
	if gotByResourceID["vm-b"].NodeDisplayName != "Beta" {
		t.Fatalf("GetActiveAlerts vm-b node display name = %q, want %q", gotByResourceID["vm-b"].NodeDisplayName, "Beta")
	}
}

func TestCheckGuestPerDiskAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	guestID := BuildGuestKey("pve1", "node1", 101)
	m.CheckGuest(models.VM{
		ID:       guestID,
		VMID:     101,
		Name:     "app01",
		Node:     "node1",
		Instance: "pve1",
		Status:   "running",
		CPU:      0.20,
		Memory:   models.Memory{Usage: 40},
		Disk:     models.Disk{Usage: 40},
		Disks: []models.Disk{
			{
				Mountpoint: "/",
				Device:     "scsi0",
				Usage:      95,
				Total:      100,
				Used:       95,
				Free:       5,
			},
		},
	}, "pve1")

	resourceID := guestID + "-disk-scsi0"
	alertID := canonicalMetricStateID(resourceID, "disk")
	m.mu.RLock()
	alert := testRequireActiveAlert(t, m, alertID)
	m.mu.RUnlock()
	if alert == nil {
		t.Fatalf("expected guest disk alert %q", alertID)
	}
	if got := alert.Metadata["canonicalAlertKind"]; got != string(alertspecs.AlertSpecKindMetricThreshold) {
		t.Fatalf("canonicalAlertKind = %v, want %s", got, alertspecs.AlertSpecKindMetricThreshold)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != canonicalMetricSpecID(resourceID, "disk") {
		t.Fatalf("canonicalSpecID = %v, want %s", got, canonicalMetricSpecID(resourceID, "disk"))
	}
}

func TestCheckGuestPerDiskCleansUpRemovedDiskAlerts(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	guestID := BuildGuestKey("pve1", "node1", 101)
	baseGuest := models.VM{
		ID:       guestID,
		VMID:     101,
		Name:     "app01",
		Node:     "node1",
		Instance: "pve1",
		Status:   "running",
		CPU:      0.20,
		Memory:   models.Memory{Usage: 40},
		Disk:     models.Disk{Usage: 40},
	}

	withDisk := baseGuest
	withDisk.Disks = []models.Disk{
		{
			Mountpoint: "/",
			Device:     "scsi0",
			Usage:      95,
			Total:      100,
			Used:       95,
			Free:       5,
		},
	}
	m.CheckGuest(withDisk, "pve1")

	originalAlertID := canonicalMetricStateID(guestID+"-disk-scsi0", "disk")
	if activeAlert(t, m, originalAlertID) == nil {
		t.Fatalf("expected guest disk alert %q", originalAlertID)
	}

	changedDisk := baseGuest
	changedDisk.Disks = []models.Disk{
		{
			Mountpoint: "/data",
			Device:     "scsi1",
			Usage:      96,
			Total:      100,
			Used:       96,
			Free:       4,
		},
	}
	m.CheckGuest(changedDisk, "pve1")

	newAlertID := canonicalMetricStateID(guestID+"-disk-data-scsi1", "disk")
	m.mu.RLock()
	_, oldExists := testLookupActiveAlert(t, m, originalAlertID)
	_, newExists := testLookupActiveAlert(t, m, newAlertID)
	m.mu.RUnlock()

	if oldExists {
		t.Fatalf("expected stale guest disk alert %q to be cleared", originalAlertID)
	}
	if !newExists {
		t.Fatalf("expected replacement guest disk alert %q", newAlertID)
	}
}

func TestCheckGuestDiskAlertMigrationDoesNotCrossGuestIdentity(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	oldResourceID := BuildGuestKey("proxmox2", "proxmox2", 107) + "-disk-local-lvm-vm-107-disk-0"
	newResourceID := BuildGuestKey("proxmox2", "proxmox2", 108) + "-disk-local-lvm-vm-108-disk-0"
	oldAlertID := canonicalMetricStateID(oldResourceID, "disk")
	newAlertID := canonicalMetricStateID(newResourceID, "disk")
	start := time.Now().Add(-10 * time.Minute)

	alert := &Alert{
		ID:              oldAlertID,
		Type:            "disk",
		Level:           AlertLevelWarning,
		ResourceID:      oldResourceID,
		CanonicalSpecID: canonicalMetricSpecID(oldResourceID, "disk"),
		CanonicalKind:   string(alertspecs.AlertSpecKindMetricThreshold),
		CanonicalState:  oldAlertID,
		ResourceName:    "wireguard",
		Node:            "proxmox2",
		Instance:        "proxmox2",
		Message:         "wireguard disk at 92.5%",
		Value:           92.5,
		Threshold:       90,
		StartTime:       start,
		LastSeen:        start.Add(5 * time.Minute),
	}

	m.mu.Lock()
	m.activeAlerts[oldAlertID] = alert
	m.historyManager.AddAlert(*alert)
	m.mu.Unlock()

	m.checkMetric(newResourceID, "pulse", "proxmox2", "proxmox2", "vm", "disk", 63.6, &HysteresisThreshold{Trigger: 90, Clear: 85}, nil)

	m.mu.RLock()
	preserved, oldExists := testLookupActiveAlert(t, m, oldAlertID)
	_, newExists := testLookupActiveAlert(t, m, newAlertID)
	m.mu.RUnlock()

	if !oldExists {
		t.Fatal("expected guest 107 disk alert to stay on guest 107")
	}
	if preserved.ResourceID != oldResourceID || preserved.ResourceName != "wireguard" {
		t.Fatalf("guest 107 alert was mutated: %#v", preserved)
	}
	if newExists {
		t.Fatal("did not expect a guest 108 disk alert to be created")
	}
	if resolved := m.GetResolvedAlert(newAlertID); resolved != nil {
		t.Fatalf("did not expect guest 108 resolved alert after evaluating guest 108 below threshold: %#v", resolved)
	}
}

func TestCheckGuestClearsPerDiskAlertsWhenGuestStops(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	guestID := BuildGuestKey("pve1", "node1", 101)
	guest := models.VM{
		ID:       guestID,
		VMID:     101,
		Name:     "app01",
		Node:     "node1",
		Instance: "pve1",
		Status:   "running",
		CPU:      0.20,
		Memory:   models.Memory{Usage: 40},
		Disk:     models.Disk{Usage: 40},
		Disks: []models.Disk{
			{
				Mountpoint: "/",
				Device:     "scsi0",
				Usage:      95,
				Total:      100,
				Used:       95,
				Free:       5,
			},
		},
	}

	m.CheckGuest(guest, "pve1")

	alertID := canonicalMetricStateID(guestID+"-disk-scsi0", "disk")
	if activeAlert(t, m, alertID) == nil {
		t.Fatalf("expected guest disk alert %q", alertID)
	}

	guest.Status = "stopped"
	guest.Disks = nil
	m.CheckGuest(guest, "pve1")

	m.mu.RLock()
	_, exists := testLookupActiveAlert(t, m, alertID)
	m.mu.RUnlock()
	if exists {
		t.Fatalf("expected guest disk alert %q to clear when guest stops", alertID)
	}
}

func TestCheckNodeTemperatureAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.CheckNode(models.Node{
		ID:       "node/pve-1",
		Name:     "pve-1",
		Instance: "pve-1",
		Status:   "online",
		CPU:      0.20,
		Memory:   models.Memory{Usage: 40},
		Disk:     models.Disk{Usage: 40},
		Temperature: &models.Temperature{
			Available:  true,
			CPUPackage: 90,
		},
	})

	m.mu.RLock()
	alertID := canonicalMetricStateID("node/pve-1", "temperature")
	alert := testRequireActiveAlert(t, m, alertID)
	m.mu.RUnlock()
	if alert == nil {
		t.Fatal("expected node temperature alert")
	}
	if got := alert.Metadata["canonicalAlertKind"]; got != string(alertspecs.AlertSpecKindMetricThreshold) {
		t.Fatalf("canonicalAlertKind = %v, want %s", got, alertspecs.AlertSpecKindMetricThreshold)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != canonicalMetricSpecID("node/pve-1", "temperature") {
		t.Fatalf("canonicalSpecID = %v, want %s", got, canonicalMetricSpecID("node/pve-1", "temperature"))
	}
}

func TestCheckGuestPoweredOffAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	resourceID := BuildGuestKey("pve1", "node1", 101)
	guest := models.VM{
		ID:       resourceID,
		VMID:     101,
		Name:     "app01",
		Node:     "node1",
		Instance: "pve1",
		Status:   "stopped",
	}

	m.CheckGuest(guest, "pve1")
	m.CheckGuest(guest, "pve1")

	alert := activeAlert(t, m, "guest-powered-off-"+resourceID)
	if got := alert.Metadata["canonicalAlertKind"]; got != "powered-state" {
		t.Fatalf("canonicalAlertKind = %v, want powered-state", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != resourceID+"-powered-state" {
		t.Fatalf("canonicalSpecID = %v, want %s", got, resourceID+"-powered-state")
	}
}

func TestCheckNodeOfflineAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.mu.Lock()
	m.nodeOfflineCount["node/pve-1"] = 2
	m.mu.Unlock()

	m.CheckNode(models.Node{
		ID:               "node/pve-1",
		Name:             "pve-1",
		Instance:         "pve1",
		Status:           "offline",
		ConnectionHealth: "failed",
	})

	alert := activeAlert(t, m, buildCanonicalStateID("node/pve-1", "node/pve-1-connectivity"))
	if got := alert.Metadata["canonicalAlertKind"]; got != "connectivity" {
		t.Fatalf("canonicalAlertKind = %v, want connectivity", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != "node/pve-1-connectivity" {
		t.Fatalf("canonicalSpecID = %v, want %s", got, "node/pve-1-connectivity")
	}
}

func TestCheckPBSOfflineAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.mu.Lock()
	m.offlineConfirmations["pbs-1"] = 2
	m.mu.Unlock()

	m.CheckPBS(models.PBSInstance{
		ID:               "pbs-1",
		Name:             "pbs-main",
		Host:             "pbs-host",
		Status:           "online",
		ConnectionHealth: "unhealthy",
	})

	alert := activeAlert(t, m, "pbs-offline-pbs-1")
	if got := alert.Metadata["canonicalAlertKind"]; got != "connectivity" {
		t.Fatalf("canonicalAlertKind = %v, want connectivity", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != "pbs-1-connectivity" {
		t.Fatalf("canonicalSpecID = %v, want %s", got, "pbs-1-connectivity")
	}
}

func TestCheckStorageOfflineAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.mu.Lock()
	m.offlineConfirmations["storage-1"] = 1
	m.mu.Unlock()

	m.CheckStorage(models.Storage{
		ID:       "storage-1",
		Name:     "local-lvm",
		Node:     "pve-1",
		Instance: "pve1",
		Status:   "unavailable",
	})

	alert := activeAlert(t, m, "storage-offline-storage-1")
	if got := alert.Metadata["canonicalAlertKind"]; got != "connectivity" {
		t.Fatalf("canonicalAlertKind = %v, want connectivity", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != "storage-1-connectivity" {
		t.Fatalf("canonicalSpecID = %v, want %s", got, "storage-1-connectivity")
	}
}

func TestCheckPMGOfflineAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.mu.Lock()
	m.offlineConfirmations["pmg-1"] = 2
	m.mu.Unlock()

	m.CheckPMG(models.PMGInstance{
		ID:               "pmg-1",
		Name:             "pmg-main",
		Host:             "pmg-host",
		Status:           "online",
		ConnectionHealth: "unhealthy",
	})

	alert := activeAlert(t, m, "pmg-offline-pmg-1")
	if got := alert.Metadata["canonicalAlertKind"]; got != "connectivity" {
		t.Fatalf("canonicalAlertKind = %v, want connectivity", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != "pmg-1-connectivity" {
		t.Fatalf("canonicalSpecID = %v, want %s", got, "pmg-1-connectivity")
	}
}

func TestHandleDockerHostOfflineAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.mu.Lock()
	m.dockerOfflineCount["docker1"] = 2
	m.mu.Unlock()

	m.HandleDockerHostOffline(models.DockerHost{
		ID:          "docker1",
		DisplayName: "Docker Host 1",
		Hostname:    "docker.local",
		AgentID:     "agent-123",
	})

	alert := activeAlert(t, m, "docker-host-offline-docker1")
	if got := alert.Metadata["canonicalAlertKind"]; got != "connectivity" {
		t.Fatalf("canonicalAlertKind = %v, want connectivity", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != "docker:docker1-connectivity" {
		t.Fatalf("canonicalSpecID = %v, want %s", got, "docker:docker1-connectivity")
	}
}

func TestCheckDockerContainerStateAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	host := models.DockerHost{
		ID:          "host-1",
		DisplayName: "Docker Host",
		Hostname:    "docker.local",
		Containers: []models.DockerContainer{
			{
				ID:     "container-1",
				Name:   "web",
				State:  "exited",
				Status: "Exited (1) seconds ago",
			},
		},
	}

	m.CheckDockerHost(host)
	m.CheckDockerHost(host)

	resourceID := dockerResourceID(host.ID, "container-1")
	alert := activeAlert(t, m, "docker-container-state-"+resourceID)
	if got := alert.Metadata["canonicalAlertKind"]; got != "discrete-state" {
		t.Fatalf("canonicalAlertKind = %v, want discrete-state", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != resourceID+"-runtime-state" {
		t.Fatalf("canonicalSpecID = %v, want %s", got, resourceID+"-runtime-state")
	}
}

func TestCheckDockerServiceAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	host := models.DockerHost{
		ID:          "host-1",
		DisplayName: "Prod Swarm",
		Hostname:    "swarm-prod",
		Services: []models.DockerService{
			{
				ID:           "svc-1",
				Name:         "web",
				DesiredTasks: 4,
				RunningTasks: 2,
				Mode:         "replicated",
			},
		},
	}

	m.CheckDockerHost(host)

	resourceID := dockerServiceResourceID(host.ID, "svc-1", "web")
	alert := activeAlert(t, m, "docker-service-health-"+resourceID)
	if got := alert.Metadata["canonicalAlertKind"]; got != "service-gap" {
		t.Fatalf("canonicalAlertKind = %v, want service-gap", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != resourceID+"-service-gap" {
		t.Fatalf("canonicalSpecID = %v, want %s", got, resourceID+"-service-gap")
	}
}

func TestCheckDockerServiceUpdateStateAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	now := time.Now()
	host := models.DockerHost{
		ID:          "host-update",
		DisplayName: "Swarm",
		Hostname:    "swarm.local",
		Services: []models.DockerService{
			{
				ID:           "svc-update",
				Name:         "api",
				DesiredTasks: 1,
				RunningTasks: 1,
				UpdateStatus: &models.DockerServiceUpdate{
					State:       "rollback_failed",
					Message:     "Rollback failed",
					CompletedAt: &now,
				},
			},
		},
	}

	m.CheckDockerHost(host)

	resourceID := dockerServiceResourceID(host.ID, "svc-update", "api")
	alert := activeAlert(t, m, "docker-service-health-"+resourceID)
	if got := alert.Metadata["canonicalAlertKind"]; got != "discrete-state" {
		t.Fatalf("canonicalAlertKind = %v, want discrete-state", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != resourceID+"-update-state" {
		t.Fatalf("canonicalSpecID = %v, want %s", got, resourceID+"-update-state")
	}
}

func TestCheckPMGQueueDepthAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)

	pmg := models.PMGInstance{
		ID:   "pmg-1",
		Name: "PMG 1",
		Nodes: []models.PMGNodeStatus{
			{Name: "node1", QueueStatus: &models.PMGQueueStatus{Total: 300}},
			{Name: "node2", QueueStatus: &models.PMGQueueStatus{Total: 250}},
		},
	}

	m.checkPMGQueueDepths(pmg, PMGThresholdConfig{
		QueueTotalWarning:  500,
		QueueTotalCritical: 1000,
	})

	alert := activeAlert(t, m, "pmg-1-queue-total")
	if got := alert.Metadata["canonicalAlertKind"]; got != "severity-threshold" {
		t.Fatalf("canonicalAlertKind = %v, want severity-threshold", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != "pmg-1-queue-total" {
		t.Fatalf("canonicalSpecID = %v, want pmg-1-queue-total", got)
	}
}

func TestCheckPMGOldestMessageAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)

	pmg := models.PMGInstance{
		ID:   "pmg-1",
		Name: "PMG 1",
		Nodes: []models.PMGNodeStatus{
			{Name: "node1", QueueStatus: &models.PMGQueueStatus{OldestAge: 2400}},
		},
	}

	m.checkPMGOldestMessage(pmg, PMGThresholdConfig{
		OldestMessageWarnMins: 30,
		OldestMessageCritMins: 60,
	})

	alert := activeAlert(t, m, "pmg-1-oldest-message")
	if got := alert.Metadata["canonicalAlertKind"]; got != "severity-threshold" {
		t.Fatalf("canonicalAlertKind = %v, want severity-threshold", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != "pmg-1-oldest-message" {
		t.Fatalf("canonicalSpecID = %v, want pmg-1-oldest-message", got)
	}
	if got := alert.Metadata["resourceType"]; got != string(unifiedresources.ResourceTypePMG) {
		t.Fatalf("resourceType = %v, want pmg", got)
	}
}

func TestCheckPMGNodeQueueAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)

	pmg := models.PMGInstance{
		ID:   "pmg-1",
		Name: "PMG 1",
		Nodes: []models.PMGNodeStatus{
			{Name: "node-a", QueueStatus: &models.PMGQueueStatus{Total: 80}},
		},
	}

	m.checkPMGNodeQueues(pmg, PMGThresholdConfig{
		QueueTotalWarning:  100,
		QueueTotalCritical: 200,
	})

	alert := activeAlert(t, m, "pmg-1-node-a-queue-total")
	if got := alert.Metadata["canonicalAlertKind"]; got != "severity-threshold" {
		t.Fatalf("canonicalAlertKind = %v, want severity-threshold", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != "pmg-1-node-a-queue-total" {
		t.Fatalf("canonicalSpecID = %v, want pmg-1-node-a-queue-total", got)
	}
}

func TestCheckPMGQuarantineAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)

	pmg := models.PMGInstance{
		ID:   "pmg-1",
		Name: "PMG 1",
		Quarantine: &models.PMGQuarantineTotals{
			Spam: 2500,
		},
	}

	m.checkPMGQuarantineBacklog(pmg, PMGThresholdConfig{
		QuarantineSpamWarn:     2000,
		QuarantineSpamCritical: 5000,
	})

	alert := activeAlert(t, m, "pmg-1-quarantine-spam")
	if got := alert.Metadata["canonicalAlertKind"]; got != "change-threshold" {
		t.Fatalf("canonicalAlertKind = %v, want change-threshold", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != "pmg-1-quarantine-spam" {
		t.Fatalf("canonicalSpecID = %v, want pmg-1-quarantine-spam", got)
	}
}

func TestCheckPMGAnomalyAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)
	base := time.Now().Add(-13 * time.Hour)

	tracker := &pmgAnomalyTracker{
		Samples:        make([]pmgMailMetricSample, 0, 12),
		LastSampleTime: base.Add(11 * time.Hour),
		SampleCount:    12,
	}
	for i := 0; i < 12; i++ {
		tracker.Samples = append(tracker.Samples, pmgMailMetricSample{
			SpamIn:    100,
			SpamOut:   10,
			VirusIn:   1,
			VirusOut:  1,
			Timestamp: base.Add(time.Duration(i) * time.Hour),
		})
	}

	m.mu.Lock()
	m.pmgAnomalyTrackers["pmg-1"] = tracker
	m.mu.Unlock()

	pmg := models.PMGInstance{
		ID:   "pmg-1",
		Name: "PMG 1",
		MailCount: []models.PMGMailCountPoint{
			{Timestamp: base.Add(12 * time.Hour), SpamIn: 420},
		},
	}
	m.checkPMGAnomalies(pmg, PMGThresholdConfig{})

	pmg.MailCount = []models.PMGMailCountPoint{
		{Timestamp: base.Add(13 * time.Hour), SpamIn: 430},
	}
	m.checkPMGAnomalies(pmg, PMGThresholdConfig{})

	alert := activeAlert(t, m, "pmg-1-anomaly-spamIn")
	if got := alert.Metadata["canonicalAlertKind"]; got != "baseline-anomaly" {
		t.Fatalf("canonicalAlertKind = %v, want baseline-anomaly", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != "pmg-1-anomaly-spamIn" {
		t.Fatalf("canonicalSpecID = %v, want pmg-1-anomaly-spamIn", got)
	}
}
