package monitoring

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type guestConfigCoverageClient struct {
	stubPVEClient

	containerConfig map[string]interface{}
	containerErr    error
	vmConfig        map[string]interface{}
	vmErr           error

	containerCalls int
	vmCalls        int
	lastNode       string
	lastVMID       int
}

func (c *guestConfigCoverageClient) GetContainerConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	c.containerCalls++
	c.lastNode = node
	c.lastVMID = vmid
	return c.containerConfig, c.containerErr
}

func (c *guestConfigCoverageClient) GetVMConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	c.vmCalls++
	c.lastNode = node
	c.lastVMID = vmid
	return c.vmConfig, c.vmErr
}

func TestGetGuestConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("nil monitor", func(t *testing.T) {
		var m *Monitor
		_, err := m.GetGuestConfig(ctx, "vm", "pve1", "node1", 101)
		if err == nil || !strings.Contains(err.Error(), "monitor not available") {
			t.Fatalf("expected monitor not available error, got %v", err)
		}
	})

	t.Run("invalid vmid", func(t *testing.T) {
		m := &Monitor{}
		_, err := m.GetGuestConfig(ctx, "vm", "pve1", "node1", 0)
		if err == nil || !strings.Contains(err.Error(), "invalid vmid") {
			t.Fatalf("expected invalid vmid error, got %v", err)
		}
	})

	t.Run("guest type required", func(t *testing.T) {
		m := &Monitor{}
		_, err := m.GetGuestConfig(ctx, "   ", "pve1", "node1", 100)
		if err == nil || !strings.Contains(err.Error(), "guest type is required") {
			t.Fatalf("expected guest type required error, got %v", err)
		}
	})

	t.Run("state required for resolution", func(t *testing.T) {
		m := &Monitor{}
		_, err := m.GetGuestConfig(ctx, "vm", "", "", 100)
		if err == nil || !strings.Contains(err.Error(), "state not available") {
			t.Fatalf("expected state not available error, got %v", err)
		}
	})

	t.Run("unsupported guest type", func(t *testing.T) {
		m := &Monitor{state: models.NewState()}
		_, err := m.GetGuestConfig(ctx, "unsupported", "", "", 100)
		if err == nil || !strings.Contains(err.Error(), "unsupported guest type") {
			t.Fatalf("expected unsupported guest type error, got %v", err)
		}
	})

	t.Run("resolution miss", func(t *testing.T) {
		m := &Monitor{state: models.NewState()}
		_, err := m.GetGuestConfig(ctx, "vm", "", "", 100)
		if err == nil || !strings.Contains(err.Error(), "unable to resolve instance or node") {
			t.Fatalf("expected unresolved instance/node error, got %v", err)
		}
	})

	t.Run("resolved client missing", func(t *testing.T) {
		state := models.NewState()
		state.VMs = []models.VM{{VMID: 100, Instance: "pve1", Node: "node1"}}
		m := &Monitor{
			state:      state,
			pveClients: map[string]PVEClientInterface{},
		}

		_, err := m.GetGuestConfig(ctx, "vm", "", "", 100)
		if err == nil || !strings.Contains(err.Error(), "no PVE client for instance pve1") {
			t.Fatalf("expected missing client error, got %v", err)
		}
	})

	t.Run("container direct lookup", func(t *testing.T) {
		client := &guestConfigCoverageClient{
			containerConfig: map[string]interface{}{"hostname": "ct100"},
		}
		m := &Monitor{
			pveClients: map[string]PVEClientInterface{"pve1": client},
		}

		cfg, err := m.GetGuestConfig(ctx, "  LXC  ", "pve1", "node1", 100)
		if err != nil {
			t.Fatalf("GetGuestConfig returned error: %v", err)
		}
		if cfg["hostname"] != "ct100" {
			t.Fatalf("unexpected config payload: %#v", cfg)
		}
		if client.containerCalls != 1 || client.lastNode != "node1" || client.lastVMID != 100 {
			t.Fatalf("unexpected container call state: calls=%d node=%q vmid=%d", client.containerCalls, client.lastNode, client.lastVMID)
		}
	})

	t.Run("container resolution from state", func(t *testing.T) {
		state := models.NewState()
		state.Containers = []models.Container{{VMID: 101, Instance: "pve2", Node: "node2"}}
		client := &guestConfigCoverageClient{
			containerConfig: map[string]interface{}{"hostname": "ct101"},
		}
		m := &Monitor{
			state:      state,
			pveClients: map[string]PVEClientInterface{"pve2": client},
		}

		cfg, err := m.GetGuestConfig(ctx, "container", "", "", 101)
		if err != nil {
			t.Fatalf("GetGuestConfig returned error: %v", err)
		}
		if cfg["hostname"] != "ct101" {
			t.Fatalf("unexpected config payload: %#v", cfg)
		}
		if client.containerCalls != 1 || client.lastNode != "node2" || client.lastVMID != 101 {
			t.Fatalf("unexpected container call state: calls=%d node=%q vmid=%d", client.containerCalls, client.lastNode, client.lastVMID)
		}
	})

	t.Run("vm client missing method", func(t *testing.T) {
		m := &Monitor{
			pveClients: map[string]PVEClientInterface{"pve1": &stubPVEClient{}},
		}

		_, err := m.GetGuestConfig(ctx, "vm", "pve1", "node1", 200)
		if err == nil || !strings.Contains(err.Error(), "VM config not supported by client") {
			t.Fatalf("expected VM config unsupported error, got %v", err)
		}
	})

	t.Run("vm success with resolution", func(t *testing.T) {
		state := models.NewState()
		state.VMs = []models.VM{{VMID: 200, Instance: "pve-vm", Node: "node-vm"}}
		client := &guestConfigCoverageClient{
			vmConfig: map[string]interface{}{"name": "vm200"},
		}
		m := &Monitor{
			state:      state,
			pveClients: map[string]PVEClientInterface{"pve-vm": client},
		}

		cfg, err := m.GetGuestConfig(ctx, "vm", "", "", 200)
		if err != nil {
			t.Fatalf("GetGuestConfig returned error: %v", err)
		}
		if cfg["name"] != "vm200" {
			t.Fatalf("unexpected config payload: %#v", cfg)
		}
		if client.vmCalls != 1 || client.lastNode != "node-vm" || client.lastVMID != 200 {
			t.Fatalf("unexpected VM call state: calls=%d node=%q vmid=%d", client.vmCalls, client.lastNode, client.lastVMID)
		}
	})
}

func TestMonitorFrontendAndMetricHelpers(t *testing.T) {
	t.Run("resource type mapping", func(t *testing.T) {
		tests := []struct {
			name     string
			resource unifiedresources.Resource
			want     string
		}{
			{name: "vm", resource: unifiedresources.Resource{Type: unifiedresources.ResourceTypeVM}, want: "vm"},
			{name: "lxc", resource: unifiedresources.Resource{Type: unifiedresources.ResourceTypeSystemContainer}, want: "system-container"},
			{name: "docker container", resource: unifiedresources.Resource{Type: unifiedresources.ResourceTypeAppContainer}, want: "app-container"},
			{name: "k8s cluster", resource: unifiedresources.Resource{Type: unifiedresources.ResourceTypeK8sCluster}, want: "k8s-cluster"},
			{name: "k8s node", resource: unifiedresources.Resource{Type: unifiedresources.ResourceTypeK8sNode}, want: "k8s-node"},
			{name: "pod", resource: unifiedresources.Resource{Type: unifiedresources.ResourceTypePod}, want: "pod"},
			{name: "k8s deployment", resource: unifiedresources.Resource{Type: unifiedresources.ResourceTypeK8sDeployment}, want: "k8s-deployment"},
			{name: "pbs", resource: unifiedresources.Resource{Type: unifiedresources.ResourceTypePBS}, want: "pbs"},
			{name: "pmg", resource: unifiedresources.Resource{Type: unifiedresources.ResourceTypePMG}, want: "pmg"},
			{name: "storage", resource: unifiedresources.Resource{Type: unifiedresources.ResourceTypeStorage}, want: "storage"},
			{name: "ceph", resource: unifiedresources.Resource{Type: unifiedresources.ResourceTypeCeph}, want: "pool"},
			{name: "host proxmox", resource: unifiedresources.Resource{Type: unifiedresources.ResourceTypeAgent, Proxmox: &unifiedresources.ProxmoxData{}}, want: "node"},
			{name: "host docker", resource: unifiedresources.Resource{Type: unifiedresources.ResourceTypeAgent, Docker: &unifiedresources.DockerData{}}, want: "docker-host"},
			{name: "host agent", resource: unifiedresources.Resource{Type: unifiedresources.ResourceTypeAgent}, want: "agent"},
			{name: "unknown passthrough", resource: unifiedresources.Resource{Type: unifiedresources.ResourceType("custom")}, want: "custom"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := monitorFrontendResourceType(tt.resource); got != tt.want {
					t.Fatalf("monitorFrontendResourceType() = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("platform type mapping and source fallbacks", func(t *testing.T) {
		tests := []struct {
			name         string
			resourceType string
			resource     unifiedresources.Resource
			want         string
		}{
			{name: "node explicit", resourceType: "node", want: "proxmox-pve"},
			{name: "docker explicit", resourceType: "app-container", want: "docker"},
			{name: "k8s explicit", resourceType: "k8s-node", want: "kubernetes"},
			{name: "pbs explicit", resourceType: "pbs", want: "proxmox-pbs"},
			{name: "pmg explicit", resourceType: "pmg", want: "proxmox-pmg"},
			{name: "agent explicit", resourceType: "agent", want: "agent"},
			{
				name:         "fallback k8s precedence",
				resourceType: "custom",
				resource:     unifiedresources.Resource{Sources: []unifiedresources.DataSource{unifiedresources.SourceDocker, unifiedresources.SourceK8s}},
				want:         "kubernetes",
			},
			{
				name:         "fallback docker",
				resourceType: "custom",
				resource:     unifiedresources.Resource{Sources: []unifiedresources.DataSource{unifiedresources.SourceDocker}},
				want:         "docker",
			},
			{
				name:         "fallback pbs",
				resourceType: "custom",
				resource:     unifiedresources.Resource{Sources: []unifiedresources.DataSource{unifiedresources.SourcePBS}},
				want:         "proxmox-pbs",
			},
			{
				name:         "fallback pmg",
				resourceType: "custom",
				resource:     unifiedresources.Resource{Sources: []unifiedresources.DataSource{unifiedresources.SourcePMG}},
				want:         "proxmox-pmg",
			},
			{
				name:         "fallback agent",
				resourceType: "custom",
				resource:     unifiedresources.Resource{Sources: []unifiedresources.DataSource{unifiedresources.SourceAgent}},
				want:         "agent",
			},
			{
				name:         "fallback proxmox source",
				resourceType: "custom",
				resource:     unifiedresources.Resource{Sources: []unifiedresources.DataSource{unifiedresources.SourceProxmox}},
				want:         "proxmox-pve",
			},
			{
				name:         "fallback custom source",
				resourceType: "custom",
				resource:     unifiedresources.Resource{Sources: []unifiedresources.DataSource{"xcp"}},
				want:         "xcp",
			},
			{
				name:         "fallback unknown",
				resourceType: "custom",
				resource:     unifiedresources.Resource{},
				want:         "unknown",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := monitorPlatformType(tt.resource, tt.resourceType); got != tt.want {
					t.Fatalf("monitorPlatformType() = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("platform id mapping", func(t *testing.T) {
		parent := " parent-1 "
		tests := []struct {
			name         string
			resourceType string
			resource     unifiedresources.Resource
			want         string
		}{
			{
				name:         "node uses proxmox instance",
				resourceType: "node",
				resource:     unifiedresources.Resource{ID: "fallback", Proxmox: &unifiedresources.ProxmoxData{Instance: "  pve-a "}},
				want:         "pve-a",
			},
			{
				name:         "agent uses agent id",
				resourceType: "agent",
				resource:     unifiedresources.Resource{ID: "fallback", Agent: &unifiedresources.AgentData{AgentID: " agent-1 "}},
				want:         "agent-1",
			},
			{
				name:         "docker host uses hostname",
				resourceType: "docker-host",
				resource:     unifiedresources.Resource{ID: "fallback", Docker: &unifiedresources.DockerData{Hostname: " docker-a "}},
				want:         "docker-a",
			},
			{
				name:         "app container uses parent fallback",
				resourceType: "app-container",
				resource:     unifiedresources.Resource{ID: "fallback", ParentID: &parent, Docker: &unifiedresources.DockerData{}},
				want:         "parent-1",
			},
			{
				name:         "k8s uses agent id",
				resourceType: "k8s-node",
				resource:     unifiedresources.Resource{ID: "fallback", Kubernetes: &unifiedresources.K8sData{AgentID: " k8s-agent "}},
				want:         "k8s-agent",
			},
			{
				name:         "pbs uses hostname",
				resourceType: "pbs",
				resource:     unifiedresources.Resource{ID: "fallback", PBS: &unifiedresources.PBSData{Hostname: " pbs-a "}},
				want:         "pbs-a",
			},
			{
				name:         "pmg uses hostname",
				resourceType: "pmg",
				resource:     unifiedresources.Resource{ID: "fallback", PMG: &unifiedresources.PMGData{Hostname: " pmg-a "}},
				want:         "pmg-a",
			},
			{
				name:         "fallback id",
				resourceType: "custom",
				resource:     unifiedresources.Resource{ID: "fallback"},
				want:         "fallback",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := monitorPlatformID(tt.resource, tt.resourceType); got != tt.want {
					t.Fatalf("monitorPlatformID() = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("legacy status mapping", func(t *testing.T) {
		tests := []struct {
			name         string
			resourceType string
			resource     unifiedresources.Resource
			want         string
		}{
			{
				name:         "docker online running",
				resourceType: "app-container",
				resource:     unifiedresources.Resource{Status: unifiedresources.StatusOnline},
				want:         "running",
			},
			{
				name:         "docker offline stopped",
				resourceType: "app-container",
				resource:     unifiedresources.Resource{Status: unifiedresources.StatusOffline},
				want:         "stopped",
			},
			{
				name:         "docker warning degraded",
				resourceType: "app-container",
				resource:     unifiedresources.Resource{Status: unifiedresources.StatusWarning},
				want:         "degraded",
			},
			{
				name:         "pod pending degraded",
				resourceType: "pod",
				resource: unifiedresources.Resource{
					Status:     unifiedresources.StatusOnline,
					Kubernetes: &unifiedresources.K8sData{PodPhase: "pending"},
				},
				want: "degraded",
			},
			{
				name:         "pod succeeded stopped",
				resourceType: "pod",
				resource: unifiedresources.Resource{
					Status:     unifiedresources.StatusOnline,
					Kubernetes: &unifiedresources.K8sData{PodPhase: "succeeded"},
				},
				want: "stopped",
			},
			{
				name:         "workload online running fallback",
				resourceType: "system-container",
				resource:     unifiedresources.Resource{Status: unifiedresources.StatusOnline},
				want:         "running",
			},
			{
				name:         "node offline",
				resourceType: "node",
				resource:     unifiedresources.Resource{Status: unifiedresources.StatusOffline},
				want:         "offline",
			},
			{
				name:         "unknown status",
				resourceType: "node",
				resource:     unifiedresources.Resource{Status: unifiedresources.StatusUnknown},
				want:         "unknown",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := monitorFrontendStatus(tt.resource, tt.resourceType); got != tt.want {
					t.Fatalf("monitorFrontendStatus() = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("metric and source helpers", func(t *testing.T) {
		if got := monitorMetricInt64(nil, func(metrics *unifiedresources.ResourceMetrics) *unifiedresources.MetricValue {
			return metrics.CPU
		}); got != 0 {
			t.Fatalf("monitorMetricInt64(nil) = %d, want 0", got)
		}

		gotRounded := monitorMetricInt64(&unifiedresources.ResourceMetrics{
			CPU: &unifiedresources.MetricValue{Value: 12.6},
		}, func(metrics *unifiedresources.ResourceMetrics) *unifiedresources.MetricValue {
			return metrics.CPU
		})
		if gotRounded != 13 {
			t.Fatalf("monitorMetricInt64 rounding = %d, want 13", gotRounded)
		}

		if got := monitorMetricUsed(nil); got != 0 {
			t.Fatalf("monitorMetricUsed(nil) = %d, want 0", got)
		}
		if got := monitorMetricTotal(nil); got != 0 {
			t.Fatalf("monitorMetricTotal(nil) = %d, want 0", got)
		}

		used := int64(9)
		total := int64(20)
		if got := monitorMetricUsed(&unifiedresources.MetricValue{Used: &used}); got != 9 {
			t.Fatalf("monitorMetricUsed() = %d, want 9", got)
		}
		if got := monitorMetricTotal(&unifiedresources.MetricValue{Total: &total}); got != 20 {
			t.Fatalf("monitorMetricTotal() = %d, want 20", got)
		}

		if monitorHasSource(nil, unifiedresources.SourceK8s) {
			t.Fatalf("monitorHasSource(nil) should be false")
		}
		if !monitorHasSource([]unifiedresources.DataSource{unifiedresources.SourceDocker, unifiedresources.SourceK8s}, unifiedresources.SourceK8s) {
			t.Fatalf("monitorHasSource should detect present source")
		}

		if got := monitorSourceStatus(nil, unifiedresources.SourceDocker); got != "" {
			t.Fatalf("monitorSourceStatus(nil) = %q, want empty", got)
		}
		if got := monitorSourceStatus(map[unifiedresources.DataSource]unifiedresources.SourceStatus{
			unifiedresources.SourceDocker: {Status: "stale"},
		}, unifiedresources.SourceDocker); got != "stale" {
			t.Fatalf("monitorSourceStatus hit = %q, want stale", got)
		}
	})
}

func TestMonitorPlatformData(t *testing.T) {
	t.Run("node payload", func(t *testing.T) {
		resource := unifiedresources.Resource{
			Proxmox: &unifiedresources.ProxmoxData{
				Instance:      "pve-a",
				ClusterName:   "cluster-a",
				PVEVersion:    "8.2",
				KernelVersion: "6.8",
			},
			SourceStatus: map[unifiedresources.DataSource]unifiedresources.SourceStatus{
				unifiedresources.SourceProxmox: {Status: "online"},
			},
		}

		payload := decodePlatformDataPayload(t, monitorPlatformData(resource, "node", "ignored"))
		if payload["instance"] != "pve-a" {
			t.Fatalf("instance = %#v, want pve-a", payload["instance"])
		}
		if payload["connectionHealth"] != "online" {
			t.Fatalf("connectionHealth = %#v, want online", payload["connectionHealth"])
		}
		if payload["isClusterMember"] != true {
			t.Fatalf("isClusterMember = %#v, want true", payload["isClusterMember"])
		}
	})

	t.Run("vm payload with metrics", func(t *testing.T) {
		resource := unifiedresources.Resource{
			Proxmox: &unifiedresources.ProxmoxData{
				VMID:     101,
				NodeName: "node-1",
				Instance: "pve-a",
				CPUs:     4,
			},
			Identity: unifiedresources.ResourceIdentity{
				IPAddresses: []string{"10.0.0.10"},
			},
			Metrics: &unifiedresources.ResourceMetrics{
				NetIn:     &unifiedresources.MetricValue{Value: 10.6},
				NetOut:    &unifiedresources.MetricValue{Value: 8.2},
				DiskRead:  &unifiedresources.MetricValue{Value: 7.5},
				DiskWrite: &unifiedresources.MetricValue{Value: 6.5},
			},
		}

		payload := decodePlatformDataPayload(t, monitorPlatformData(resource, "vm", "ignored"))
		if payload["networkIn"] != float64(11) {
			t.Fatalf("networkIn = %#v, want 11", payload["networkIn"])
		}
		if payload["networkOut"] != float64(8) {
			t.Fatalf("networkOut = %#v, want 8", payload["networkOut"])
		}
		if payload["diskRead"] != float64(8) {
			t.Fatalf("diskRead = %#v, want 8", payload["diskRead"])
		}
		if payload["diskWrite"] != float64(7) {
			t.Fatalf("diskWrite = %#v, want 7", payload["diskWrite"])
		}
	})

	t.Run("pbs payload memory helpers", func(t *testing.T) {
		used := int64(20)
		total := int64(100)
		resource := unifiedresources.Resource{
			PBS: &unifiedresources.PBSData{
				Hostname:         "pbs-1",
				Version:          "3.2",
				ConnectionHealth: "healthy",
				DatastoreCount:   2,
			},
			Metrics: &unifiedresources.ResourceMetrics{
				Memory: &unifiedresources.MetricValue{Used: &used, Total: &total},
			},
		}

		payload := decodePlatformDataPayload(t, monitorPlatformData(resource, "pbs", "ignored"))
		if payload["memoryUsed"] != float64(20) {
			t.Fatalf("memoryUsed = %#v, want 20", payload["memoryUsed"])
		}
		if payload["memoryTotal"] != float64(100) {
			t.Fatalf("memoryTotal = %#v, want 100", payload["memoryTotal"])
		}
	})

	t.Run("storage and pool payload", func(t *testing.T) {
		parent := "node-a"
		resource := unifiedresources.Resource{
			Status:   unifiedresources.StatusOnline,
			ParentID: &parent,
		}

		storagePayload := decodePlatformDataPayload(t, monitorPlatformData(resource, "storage", "pve-a"))
		if storagePayload["instance"] != "pve-a" {
			t.Fatalf("instance = %#v, want pve-a", storagePayload["instance"])
		}
		if storagePayload["node"] != "node-a" {
			t.Fatalf("node = %#v, want node-a", storagePayload["node"])
		}
		if storagePayload["active"] != true {
			t.Fatalf("active = %#v, want true", storagePayload["active"])
		}

		poolPayload := decodePlatformDataPayload(t, monitorPlatformData(resource, "pool", "pve-a"))
		if poolPayload["active"] != true {
			t.Fatalf("pool active = %#v, want true", poolPayload["active"])
		}
	})

	t.Run("nil payload branches", func(t *testing.T) {
		if got := monitorPlatformData(unifiedresources.Resource{}, "agent", "id"); got != nil {
			t.Fatalf("expected nil payload for agent without resource agent data, got %s", string(got))
		}
		if got := monitorPlatformData(unifiedresources.Resource{}, "unknown", "id"); got != nil {
			t.Fatalf("expected nil payload for unknown type, got %s", string(got))
		}
	})
}

func TestMonitorGetLiveStateSnapshot(t *testing.T) {
	t.Run("nil monitor", func(t *testing.T) {
		var m *Monitor
		snapshot := m.GetLiveStateSnapshot()
		if len(snapshot.Nodes) != 0 || len(snapshot.VMs) != 0 || len(snapshot.Containers) != 0 {
			t.Fatalf("expected empty snapshot for nil monitor, got %#v", snapshot)
		}
	})

	t.Run("returns underlying state snapshot", func(t *testing.T) {
		state := models.NewState()
		state.Nodes = []models.Node{{ID: "node-1", Name: "node-1", LastSeen: time.Now()}}
		m := &Monitor{state: state}

		snapshot := m.GetLiveStateSnapshot()
		if len(snapshot.Nodes) != 1 {
			t.Fatalf("expected one node in snapshot, got %d", len(snapshot.Nodes))
		}
		if snapshot.Nodes[0].ID != "node-1" {
			t.Fatalf("unexpected node id: %q", snapshot.Nodes[0].ID)
		}
	})
}

func TestMonitorGetLiveHostsSnapshot(t *testing.T) {
	t.Run("nil monitor", func(t *testing.T) {
		var m *Monitor
		hosts := m.GetLiveHostsSnapshot()
		if len(hosts) != 0 {
			t.Fatalf("expected empty hosts for nil monitor, got %#v", hosts)
		}
	})

	t.Run("returns underlying host registrations", func(t *testing.T) {
		state := models.NewState()
		state.UpsertHost(models.Host{ID: "host-1", Hostname: "host-1.local"})
		m := &Monitor{state: state}

		hosts := m.GetLiveHostsSnapshot()
		if len(hosts) != 1 {
			t.Fatalf("expected one host in snapshot, got %d", len(hosts))
		}
		if hosts[0].ID != "host-1" {
			t.Fatalf("unexpected host id: %q", hosts[0].ID)
		}
	})
}

func TestMonitorHostsSnapshot(t *testing.T) {
	t.Run("nil monitor", func(t *testing.T) {
		var m *Monitor
		hosts := m.HostsSnapshot()
		if len(hosts) != 0 {
			t.Fatalf("expected empty hosts for nil monitor, got %#v", hosts)
		}
	})

	t.Run("prefers canonical read state hosts over legacy snapshot", func(t *testing.T) {
		now := time.Date(2026, 3, 11, 12, 30, 0, 0, time.UTC)
		tokenLastUsed := now.Add(-5 * time.Minute)
		canonicalState := models.StateSnapshot{
			Hosts: []models.Host{{
				ID:            "host-1",
				Hostname:      "host-1.local",
				DisplayName:   "Host One",
				Platform:      "linux",
				OSName:        "Ubuntu",
				OSVersion:     "24.04",
				KernelVersion: "6.8.0",
				Architecture:  "amd64",
				CPUCount:      16,
				CPUUsage:      15.5,
				Memory:        models.Memory{Total: 64, Used: 16, Free: 48, Usage: 25, SwapUsed: 2, SwapTotal: 8},
				LoadAverage:   []float64{0.2, 0.3, 0.4},
				Disks:         []models.Disk{{Device: "/dev/sda1", Mountpoint: "/", Type: "ext4", Total: 100, Used: 60, Free: 40, Usage: 60}},
				DiskIO:        []models.DiskIO{{Device: "/dev/sda", ReadBytes: 1, WriteBytes: 2, ReadOps: 3, WriteOps: 4, IOTime: 5}},
				NetworkInterfaces: []models.HostNetworkInterface{{
					Name:      "eth0",
					MAC:       "aa:bb:cc:dd:ee:ff",
					Addresses: []string{"10.0.0.5/24"},
				}},
				Sensors: models.HostSensorSummary{
					TemperatureCelsius: map[string]float64{"cpu_package": 55.5},
					SMART: []models.HostDiskSMART{{
						Device:      "sda",
						Model:       "Samsung",
						Serial:      "serial-1",
						Temperature: 39,
						Health:      "PASSED",
					}},
				},
				RAID: []models.HostRAIDArray{{
					Device: "/dev/md0",
					Level:  "raid1",
					State:  "clean",
				}},
				Unraid: &models.HostUnraidStorage{
					ArrayStarted: true,
					ArrayState:   "STARTED",
				},
				Ceph: &models.HostCephCluster{
					FSID: "fsid-1",
					Health: models.HostCephHealth{
						Status: "HEALTH_OK",
					},
				},
				Status:            "online",
				UptimeSeconds:     7200,
				IntervalSeconds:   15,
				LastSeen:          now,
				AgentVersion:      "1.2.3",
				MachineID:         "machine-1",
				CommandsEnabled:   true,
				ReportIP:          "10.0.0.99",
				TokenID:           "token-1",
				TokenName:         "Agent Token",
				TokenHint:         "agt_1234",
				TokenLastUsedAt:   &tokenLastUsed,
				Tags:              []string{"linux", "site:1"},
				DiskExclude:       []string{"/dev/loop*"},
				IsLegacy:          true,
				NetInRate:         10.5,
				NetOutRate:        11.5,
				DiskReadRate:      12.5,
				DiskWriteRate:     13.5,
				LinkedNodeID:      "node-1",
				LinkedVMID:        "vm-1",
				LinkedContainerID: "ct-1",
			}},
		}
		registry := unifiedresources.NewRegistry(nil)
		registry.IngestSnapshot(canonicalState)

		legacyState := models.NewState()
		legacyState.Hosts = []models.Host{{ID: "legacy-host", Hostname: "legacy"}}

		m := &Monitor{
			state:         legacyState,
			resourceStore: unifiedresources.NewMonitorAdapter(registry),
		}

		hosts := m.HostsSnapshot()
		if len(hosts) != 1 {
			t.Fatalf("expected one host entry from read-state, got %d", len(hosts))
		}
		host := hosts[0]
		if host.ID != "host-1" || host.Hostname != "host-1.local" || host.DisplayName != "Host One" {
			t.Fatalf("expected canonical host identity, got %#v", host)
		}
		if host.CPUCount != 16 || host.CPUUsage != 15.5 || host.IntervalSeconds != 15 {
			t.Fatalf("expected canonical cpu/interval fields, got cpuCount=%d cpuUsage=%v interval=%d", host.CPUCount, host.CPUUsage, host.IntervalSeconds)
		}
		if host.MachineID != "machine-1" || !host.CommandsEnabled || host.ReportIP != "10.0.0.99" {
			t.Fatalf("expected canonical machine/command/report fields, got machine=%q commands=%v reportIP=%q", host.MachineID, host.CommandsEnabled, host.ReportIP)
		}
		if len(host.LoadAverage) != 3 || host.LoadAverage[0] != 0.2 {
			t.Fatalf("expected canonical load average, got %v", host.LoadAverage)
		}
		if len(host.Disks) != 1 || host.Disks[0].Type != "ext4" {
			t.Fatalf("expected canonical disks, got %+v", host.Disks)
		}
		if len(host.DiskIO) != 1 || host.DiskIO[0].IOTime != 5 {
			t.Fatalf("expected canonical disk io, got %+v", host.DiskIO)
		}
		if len(host.NetworkInterfaces) != 1 || host.NetworkInterfaces[0].Name != "eth0" {
			t.Fatalf("expected canonical network interfaces, got %+v", host.NetworkInterfaces)
		}
		if len(host.Sensors.SMART) != 1 || host.Sensors.SMART[0].Device != "sda" {
			t.Fatalf("expected canonical sensor smart data, got %+v", host.Sensors)
		}
		if host.Unraid == nil || host.Ceph == nil {
			t.Fatalf("expected canonical unraid and ceph data, got unraid=%+v ceph=%+v", host.Unraid, host.Ceph)
		}
		if host.TokenLastUsedAt == nil || !host.TokenLastUsedAt.Equal(tokenLastUsed) {
			t.Fatalf("expected token last used at %v, got %+v", tokenLastUsed, host.TokenLastUsedAt)
		}
		if len(host.DiskExclude) != 1 || host.DiskExclude[0] != "/dev/loop*" {
			t.Fatalf("expected canonical disk exclude patterns, got %v", host.DiskExclude)
		}
		if host.NetInRate != 10.5 || host.NetOutRate != 11.5 || host.DiskReadRate != 12.5 || host.DiskWriteRate != 13.5 {
			t.Fatalf("expected canonical host rates, got netIn=%v netOut=%v diskRead=%v diskWrite=%v", host.NetInRate, host.NetOutRate, host.DiskReadRate, host.DiskWriteRate)
		}
		if host.LinkedNodeID != "node-1" || host.LinkedVMID != "vm-1" || host.LinkedContainerID != "ct-1" {
			t.Fatalf("expected canonical linked IDs, got node=%q vm=%q ct=%q", host.LinkedNodeID, host.LinkedVMID, host.LinkedContainerID)
		}
	})

	t.Run("does not fall back to stale snapshot when live read state is empty", func(t *testing.T) {
		state := models.NewState()
		state.Hosts = []models.Host{{ID: "legacy-host", Hostname: "legacy"}}

		m := &Monitor{
			state:         state,
			resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)),
		}

		hosts := m.HostsSnapshot()
		if len(hosts) != 0 {
			t.Fatalf("expected empty hosts from live canonical read-state, got %#v", hosts)
		}
	})
}

func TestMonitorDockerHostsSnapshot(t *testing.T) {
	t.Run("nil monitor", func(t *testing.T) {
		var m *Monitor
		hosts := m.DockerHostsSnapshot()
		if len(hosts) != 0 {
			t.Fatalf("expected empty docker hosts for nil monitor, got %#v", hosts)
		}
	})

	t.Run("prefers canonical read state docker hosts over legacy snapshot", func(t *testing.T) {
		now := time.Date(2026, 3, 11, 13, 0, 0, 0, time.UTC)
		tokenLastUsed := now.Add(-10 * time.Minute)
		canonicalState := models.StateSnapshot{
			DockerHosts: []models.DockerHost{{
				ID:                "docker-host-1",
				AgentID:           "agent-docker-1",
				Hostname:          "docker-1.local",
				DisplayName:       "Docker One",
				CustomDisplayName: "Custom Docker One",
				MachineID:         "machine-docker-1",
				OS:                "Ubuntu",
				KernelVersion:     "6.8.0",
				Architecture:      "amd64",
				Runtime:           "docker",
				RuntimeVersion:    "1.7.0",
				DockerVersion:     "25.0.0",
				CPUs:              12,
				TotalMemoryBytes:  32768,
				UptimeSeconds:     7200,
				CPUUsage:          22,
				LoadAverage:       []float64{0.4, 0.5, 0.6},
				Memory:            models.Memory{Total: 32768, Used: 8192, Free: 24576, Usage: 25},
				Disks:             []models.Disk{{Device: "/dev/nvme0n1p1", Total: 1000, Used: 100, Free: 900, Usage: 10}},
				NetworkInterfaces: []models.HostNetworkInterface{{Name: "eno1", Addresses: []string{"10.0.0.40/24"}}},
				Status:            "warning",
				LastSeen:          now,
				IntervalSeconds:   20,
				AgentVersion:      "2.0.0",
				Containers:        []models.DockerContainer{{ID: "ctr-1", Name: "app"}},
				Services:          []models.DockerService{{ID: "svc-1", Name: "svc"}},
				Tasks:             []models.DockerTask{{ID: "task-1", DesiredState: "running"}},
				Swarm:             &models.DockerSwarmInfo{ClusterID: "swarm-1", ClusterName: "prod", NodeRole: "manager"},
				TokenID:           "token-1",
				TokenName:         "docker-token",
				TokenHint:         "docke...123",
				TokenLastUsedAt:   &tokenLastUsed,
				Hidden:            true,
				PendingUninstall:  true,
				Command:           &models.DockerHostCommandStatus{ID: "cmd-1", Status: "queued"},
				IsLegacy:          true,
				NetInRate:         101.5,
				NetOutRate:        102.5,
				DiskReadRate:      103.5,
				DiskWriteRate:     104.5,
			}},
		}
		registry := unifiedresources.NewRegistry(nil)
		registry.IngestSnapshot(canonicalState)

		legacyState := models.NewState()
		legacyState.DockerHosts = []models.DockerHost{{ID: "legacy-docker", Hostname: "legacy"}}

		m := &Monitor{
			state:         legacyState,
			resourceStore: unifiedresources.NewMonitorAdapter(registry),
		}

		hosts := m.DockerHostsSnapshot()
		if len(hosts) != 1 {
			t.Fatalf("expected one docker host entry from read-state, got %d", len(hosts))
		}
		host := hosts[0]
		if host.ID != "docker-host-1" || host.Hostname != "docker-1.local" || host.CustomDisplayName != "Custom Docker One" {
			t.Fatalf("expected canonical docker host identity, got %#v", host)
		}
		if host.Runtime != "docker" || host.CPUs != 12 || host.TotalMemoryBytes != 32768 || host.IntervalSeconds != 20 {
			t.Fatalf("expected canonical runtime/cpu/memory/interval fields, got runtime=%q cpus=%d memory=%d interval=%d", host.Runtime, host.CPUs, host.TotalMemoryBytes, host.IntervalSeconds)
		}
		if len(host.Containers) != 1 || host.Containers[0].ID != "ctr-1" || len(host.Services) != 1 || len(host.Tasks) != 1 {
			t.Fatalf("expected canonical containers/services/tasks, got containers=%+v services=%+v tasks=%+v", host.Containers, host.Services, host.Tasks)
		}
		if host.Swarm == nil || host.Swarm.ClusterID != "swarm-1" {
			t.Fatalf("expected canonical swarm info, got %+v", host.Swarm)
		}
		if host.TokenLastUsedAt == nil || !host.TokenLastUsedAt.Equal(tokenLastUsed) {
			t.Fatalf("expected token last used at %v, got %+v", tokenLastUsed, host.TokenLastUsedAt)
		}
		if !host.Hidden || !host.PendingUninstall || host.Command == nil || host.Command.ID != "cmd-1" || !host.IsLegacy {
			t.Fatalf("expected canonical host flags/command, got hidden=%v pending=%v command=%+v legacy=%v", host.Hidden, host.PendingUninstall, host.Command, host.IsLegacy)
		}
		if host.NetInRate != 101.5 || host.NetOutRate != 102.5 || host.DiskReadRate != 103.5 || host.DiskWriteRate != 104.5 {
			t.Fatalf("expected canonical host rates, got netIn=%v netOut=%v diskRead=%v diskWrite=%v", host.NetInRate, host.NetOutRate, host.DiskReadRate, host.DiskWriteRate)
		}
	})

	t.Run("does not fall back to stale snapshot when live read state is empty", func(t *testing.T) {
		state := models.NewState()
		state.DockerHosts = []models.DockerHost{{ID: "legacy-docker", Hostname: "legacy"}}

		m := &Monitor{
			state:         state,
			resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)),
		}

		hosts := m.DockerHostsSnapshot()
		if len(hosts) != 0 {
			t.Fatalf("expected empty docker hosts from live canonical read-state, got %#v", hosts)
		}
	})
}

func TestMonitorWorkloadSnapshots(t *testing.T) {
	t.Run("nil monitor", func(t *testing.T) {
		var m *Monitor
		if got := m.VMsSnapshot(); len(got) != 0 {
			t.Fatalf("expected empty VMs for nil monitor, got %#v", got)
		}
		if got := m.ContainersSnapshot(); len(got) != 0 {
			t.Fatalf("expected empty containers for nil monitor, got %#v", got)
		}
	})

	t.Run("returns current vm and container snapshots", func(t *testing.T) {
		state := models.NewState()
		state.UpdateVMsForInstance("pve", []models.VM{{ID: "vm-1", Name: "vm-1"}})
		state.UpdateContainersForInstance("pve", []models.Container{{ID: "ct-1", Name: "ct-1"}})
		m := &Monitor{state: state}

		vms := m.VMsSnapshot()
		if len(vms) != 1 {
			t.Fatalf("expected one VM in snapshot, got %d", len(vms))
		}
		if vms[0].ID != "vm-1" {
			t.Fatalf("unexpected vm id: %q", vms[0].ID)
		}

		containers := m.ContainersSnapshot()
		if len(containers) != 1 {
			t.Fatalf("expected one container in snapshot, got %d", len(containers))
		}
		if containers[0].ID != "ct-1" {
			t.Fatalf("unexpected container id: %q", containers[0].ID)
		}
	})
}

func TestMonitorNodesSnapshot(t *testing.T) {
	t.Run("nil monitor", func(t *testing.T) {
		var m *Monitor
		nodes := m.NodesSnapshot()
		if len(nodes) != 0 {
			t.Fatalf("expected empty nodes for nil monitor, got %#v", nodes)
		}
	})

	t.Run("returns current proxmox nodes", func(t *testing.T) {
		state := models.NewState()
		state.Nodes = []models.Node{{ID: "node-1", Name: "pve-1"}}
		m := &Monitor{state: state}

		nodes := m.NodesSnapshot()
		if len(nodes) != 1 {
			t.Fatalf("expected one node in snapshot, got %d", len(nodes))
		}
		if nodes[0].ID != "node-1" {
			t.Fatalf("unexpected node id: %q", nodes[0].ID)
		}
	})

	t.Run("prefers canonical read state nodes over legacy snapshot", func(t *testing.T) {
		now := time.Date(2026, 3, 11, 12, 0, 0, 0, time.UTC)
		tempEnabled := true
		checkedAt := now.Add(-15 * time.Minute)
		canonicalState := models.StateSnapshot{
			Nodes: []models.Node{{
				ID:                           "node-source-1",
				Name:                         "pve-1",
				DisplayName:                  "PVE One",
				Instance:                     "lab",
				Host:                         "https://pve-1.example:8006",
				GuestURL:                     "https://pve-1-guest.example:8006",
				Status:                       "online",
				Type:                         "node",
				CPU:                          12,
				Memory:                       models.Memory{Used: 32, Total: 64, Free: 32, Usage: 50},
				Disk:                         models.Disk{Used: 100, Total: 200, Free: 100, Usage: 50},
				Uptime:                       3600,
				LoadAverage:                  []float64{0.1, 0.2, 0.3},
				KernelVersion:                "6.8.0",
				PVEVersion:                   "8.2.2",
				CPUInfo:                      models.CPUInfo{Model: "Xeon", Cores: 8, Sockets: 2},
				Temperature:                  &models.Temperature{CPUPackage: 64.5, CPUMax: 66.6, Available: true, HasCPU: true, LastUpdate: now},
				TemperatureMonitoringEnabled: &tempEnabled,
				LastSeen:                     now,
				ConnectionHealth:             "healthy",
				IsClusterMember:              true,
				ClusterName:                  "cluster-a",
				PendingUpdates:               4,
				PendingUpdatesCheckedAt:      checkedAt,
				LinkedAgentID:                "agent-1",
			}},
		}
		registry := unifiedresources.NewRegistry(nil)
		registry.IngestSnapshot(canonicalState)

		legacyState := models.NewState()
		legacyState.Nodes = []models.Node{{ID: "legacy-node", Name: "legacy"}}

		m := &Monitor{
			state:         legacyState,
			resourceStore: unifiedresources.NewMonitorAdapter(registry),
		}

		nodes := m.NodesSnapshot()
		if len(nodes) != 1 {
			t.Fatalf("expected one node entry from read-state, got %d", len(nodes))
		}
		if nodes[0].ID != "node-source-1" || nodes[0].Name != "pve-1" || nodes[0].DisplayName != "PVE One" {
			t.Fatalf("expected canonical node identity, got %#v", nodes[0])
		}
		if nodes[0].GuestURL != "https://pve-1-guest.example:8006" || nodes[0].ConnectionHealth != "healthy" {
			t.Fatalf("expected guest URL and connection health from canonical state, got guest=%q health=%q", nodes[0].GuestURL, nodes[0].ConnectionHealth)
		}
		if nodes[0].Temperature == nil || !nodes[0].Temperature.Available || nodes[0].Temperature.CPUMax != 66.6 {
			t.Fatalf("expected temperature details from canonical state, got %+v", nodes[0].Temperature)
		}
		if nodes[0].TemperatureMonitoringEnabled == nil || !*nodes[0].TemperatureMonitoringEnabled {
			t.Fatalf("expected temperature monitoring flag from canonical state, got %+v", nodes[0].TemperatureMonitoringEnabled)
		}
		if !nodes[0].PendingUpdatesCheckedAt.Equal(checkedAt) {
			t.Fatalf("expected pending updates checked at %v, got %v", checkedAt, nodes[0].PendingUpdatesCheckedAt)
		}
		if nodes[0].LinkedAgentID != "agent-1" {
			t.Fatalf("expected linked agent id from canonical state, got %q", nodes[0].LinkedAgentID)
		}
	})

	t.Run("does not fall back to stale snapshot when live read state is empty", func(t *testing.T) {
		state := models.NewState()
		state.Nodes = []models.Node{{ID: "legacy-node", Name: "legacy"}}

		m := &Monitor{
			state:         state,
			resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)),
		}

		nodes := m.NodesSnapshot()
		if len(nodes) != 0 {
			t.Fatalf("expected empty nodes from live canonical read-state, got %#v", nodes)
		}
	})
}

func TestMonitorStorageSnapshot(t *testing.T) {
	t.Run("nil monitor", func(t *testing.T) {
		var m *Monitor
		storage := m.StorageSnapshot()
		if len(storage) != 0 {
			t.Fatalf("expected empty storage for nil monitor, got %#v", storage)
		}
	})

	t.Run("returns current storage snapshot", func(t *testing.T) {
		state := models.NewState()
		state.Storage = []models.Storage{{ID: "store-1", Name: "Store One"}}
		m := &Monitor{state: state}

		storage := m.StorageSnapshot()
		if len(storage) != 1 {
			t.Fatalf("expected one storage entry, got %d", len(storage))
		}
		if storage[0].ID != "store-1" {
			t.Fatalf("unexpected storage id: %q", storage[0].ID)
		}
	})

	t.Run("prefers canonical read state storage over legacy snapshot", func(t *testing.T) {
		canonicalState := models.StateSnapshot{
			Storage: []models.Storage{{
				ID:       "store-canonical",
				Name:     "Canonical Store",
				Node:     "cluster",
				Instance: "lab",
				Nodes:    []string{"pve-a", "pve-b"},
				Type:     "zfspool",
				Status:   "warning",
				Path:     "/mnt/pve/store-canonical",
				Total:    100,
				Used:     70,
				Free:     30,
				Usage:    70,
				Content:  "images,iso",
				Shared:   true,
				Enabled:  true,
				Active:   true,
				ZFSPool: &models.ZFSPool{
					Name:           "Canonical Store",
					State:          "DEGRADED",
					ReadErrors:     1,
					WriteErrors:    2,
					ChecksumErrors: 3,
				},
			}},
		}
		registry := unifiedresources.NewRegistry(nil)
		registry.IngestSnapshot(canonicalState)

		legacyState := models.NewState()
		legacyState.Storage = []models.Storage{{
			ID:   "store-legacy",
			Name: "Legacy Store",
		}}

		m := &Monitor{
			state:         legacyState,
			resourceStore: unifiedresources.NewMonitorAdapter(registry),
		}

		storage := m.StorageSnapshot()
		if len(storage) != 1 {
			t.Fatalf("expected one storage entry from read-state, got %d", len(storage))
		}
		if storage[0].ID != "store-canonical" || storage[0].Name != "Canonical Store" {
			t.Fatalf("expected canonical storage entry, got %#v", storage[0])
		}
		if got := storage[0].NodeIDs; len(got) != 2 || got[0] != "lab-pve-a" || got[1] != "lab-pve-b" {
			t.Fatalf("expected derived node IDs [lab-pve-a lab-pve-b], got %v", got)
		}
		if storage[0].ZFSPool == nil || storage[0].ZFSPool.State != "DEGRADED" || storage[0].ZFSPool.ReadErrors != 1 {
			t.Fatalf("expected canonical ZFS pool details, got %#v", storage[0].ZFSPool)
		}
		if !storage[0].Enabled || !storage[0].Active {
			t.Fatalf("expected enabled and active flags from canonical read-state, got enabled=%v active=%v", storage[0].Enabled, storage[0].Active)
		}
	})

	t.Run("does not fall back to stale snapshot when live read state is empty", func(t *testing.T) {
		state := models.NewState()
		state.Storage = []models.Storage{{ID: "store-legacy", Name: "Legacy Store"}}

		m := &Monitor{
			state:         state,
			resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)),
		}

		storage := m.StorageSnapshot()
		if len(storage) != 0 {
			t.Fatalf("expected empty storage from live canonical read-state, got %#v", storage)
		}
	})
}

func decodePlatformDataPayload(t *testing.T, raw json.RawMessage) map[string]interface{} {
	t.Helper()
	if len(raw) == 0 {
		t.Fatal("expected non-empty json payload")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	return payload
}
