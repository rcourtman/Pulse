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
