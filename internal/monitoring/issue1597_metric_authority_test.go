package monitoring

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestIssue1597UnifiedGuestCPUFeedsDashboardDetailsAndAlerts(t *testing.T) {
	now := time.Now().UTC()
	store := unifiedresources.NewMemoryStore()
	if err := store.AddLink(unifiedresources.ResourceLink{
		ResourceA: "guest",
		ResourceB: "agent",
		PrimaryID: "agent",
	}); err != nil {
		t.Fatalf("AddLink() error = %v", err)
	}

	registry := unifiedresources.NewRegistry(store)
	registry.IngestResources([]unifiedresources.Resource{
		{
			ID:       "guest",
			Type:     unifiedresources.ResourceTypeSystemContainer,
			Name:     "database",
			Status:   unifiedresources.StatusOnline,
			LastSeen: now.Add(-2 * time.Hour),
			Sources:  []unifiedresources.DataSource{unifiedresources.SourceProxmox},
			SourceStatus: map[unifiedresources.DataSource]unifiedresources.SourceStatus{
				unifiedresources.SourceProxmox: {
					Status:   "stale",
					LastSeen: now.Add(-2 * time.Hour),
				},
			},
			Metrics: &unifiedresources.ResourceMetrics{
				CPU: &unifiedresources.MetricValue{
					Value:   0.58,
					Percent: 0.58,
					Unit:    "percent",
					Source:  unifiedresources.SourceProxmox,
				},
			},
			Proxmox: &unifiedresources.ProxmoxData{
				SourceID:      "cluster-a:pve-1:301",
				NodeName:      "pve-1",
				ClusterName:   "cluster-a",
				Instance:      "connection-a",
				VMID:          301,
				ContainerType: "lxc",
				CPUs:          4,
			},
		},
		{
			ID:       "agent",
			Type:     unifiedresources.ResourceTypeAgent,
			Name:     "database-agent",
			Status:   unifiedresources.StatusOnline,
			LastSeen: now,
			Sources:  []unifiedresources.DataSource{unifiedresources.SourceAgent},
			SourceStatus: map[unifiedresources.DataSource]unifiedresources.SourceStatus{
				unifiedresources.SourceAgent: {Status: "online", LastSeen: now},
			},
			Metrics: &unifiedresources.ResourceMetrics{
				CPU: &unifiedresources.MetricValue{
					Value:   58,
					Percent: 58,
					Unit:    "percent",
					Source:  unifiedresources.SourceAgent,
				},
				NetIn: &unifiedresources.MetricValue{
					Value:  4096,
					Unit:   "bytes_per_second",
					Source: unifiedresources.SourceAgent,
				},
			},
			Agent: &unifiedresources.AgentData{
				AgentID:   "agent",
				Hostname:  "database",
				OSName:    "Linux",
				OSVersion: "6.8",
			},
		},
	})

	adapter := unifiedresources.NewMonitorAdapter(registry)
	frontend := convertResourcesForBroadcast(adapter.GetAll(), adapter)
	if len(frontend) != 1 {
		t.Fatalf("dashboard resource count = %d, want one unified guest", len(frontend))
	}
	if frontend[0].Type != "system-container" || frontend[0].CPU == nil {
		t.Fatalf("dashboard guest shape = %+v, want system-container with CPU", frontend[0])
	}
	if math.Abs(frontend[0].CPU.Current-0.58) > 0.000001 {
		t.Fatalf("dashboard CPU = %v, want authoritative Proxmox 0.58", frontend[0].CPU.Current)
	}
	if len(frontend[0].Proxmox) == 0 || len(frontend[0].Agent) == 0 {
		t.Fatalf("details payload lost source facets: proxmox=%s agent=%s", frontend[0].Proxmox, frontend[0].Agent)
	}

	var metricsTarget unifiedresources.MetricsTarget
	if err := json.Unmarshal(frontend[0].MetricsTarget, &metricsTarget); err != nil {
		t.Fatalf("decode metrics target: %v", err)
	}
	if metricsTarget.ResourceType != "system-container" || metricsTarget.ResourceID != "cluster-a:pve-1:301" {
		t.Fatalf("dashboard metrics target = %+v, want platform guest history", metricsTarget)
	}

	containers := adapter.Containers()
	if len(containers) != 1 {
		t.Fatalf("canonical container view count = %d, want one", len(containers))
	}
	alertGuest := containerFromReadStateView(containers[0])
	alertGuest.Status = "running"
	if math.Abs(alertGuest.CPU-0.0058) > 0.000001 || alertGuest.CPUs != 4 {
		t.Fatalf("alert guest CPU/core shape = cpu:%v cpus:%d, want ratio 0.0058 and metadata-only four cores", alertGuest.CPU, alertGuest.CPUs)
	}

	alertManager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(alertManager.Stop)
	alertManager.UpdateConfig(alerts.AlertConfig{
		Enabled:         true,
		ActivationState: alerts.ActivationActive,
		GuestDefaults: alerts.ThresholdConfig{
			CPU: &alerts.HysteresisThreshold{Trigger: 10, Clear: 5},
		},
		TimeThresholds:       map[string]int{},
		MetricTimeThresholds: map[string]map[string]int{},
	})
	alertManager.CheckGuest(alertGuest, "connection-a")
	if cpuAlert := issue1597CPUAlert(alertManager.GetActiveAlerts()); cpuAlert != nil {
		t.Fatalf("authoritative 0.58%% platform CPU incorrectly fired alert: %+v", cpuAlert)
	}

	// Prove the threshold would have fired if the 58% in-guest agent value had
	// leaked into the canonical guest alert path.
	alertGuest.CPU = 0.58
	alertManager.CheckGuest(alertGuest, "connection-a")
	if cpuAlert := issue1597CPUAlert(alertManager.GetActiveAlerts()); cpuAlert == nil {
		t.Fatal("alert control did not fire for the rejected 58% agent CPU value")
	}
}

func issue1597CPUAlert(active []alerts.Alert) *alerts.Alert {
	for index := range active {
		if active[index].Type == "cpu" {
			return &active[index]
		}
	}
	return nil
}
