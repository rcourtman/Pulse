package unifiedresources

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func ptrInt64(v int64) *int64 { return &v }

func ptrFloat64(v float64) *float64 { return &v }

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d items, got %d: got=%v want=%v", len(want), len(got), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: expected %q, got %q (got=%v want=%v)", i, want[i], got[i], got, want)
		}
	}
}

func assertFloatSlice(t *testing.T, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d items, got %d: got=%v want=%v", len(want), len(got), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: expected %v, got %v (got=%v want=%v)", i, want[i], got[i], got, want)
		}
	}
}

// testResource creates a minimal resource of the provided type for tests.
func testResource(typ ResourceType) *Resource {
	now := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
	return &Resource{
		ID:       string(typ) + "-id",
		Type:     typ,
		Name:     string(typ) + "-name",
		Status:   StatusOnline,
		LastSeen: now,
	}
}

func TestView_VMViewAccessors(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	lastBackup := time.Date(2026, 2, 9, 8, 30, 0, 0, time.UTC)

	parentID := "host-parent-1"
	r := &Resource{
		ID:       "vm-1",
		Type:     ResourceTypeVM,
		Name:     "app-vm",
		Status:   StatusOnline,
		LastSeen: now,
		Tags:     []string{"prod", "tier:web"},
		ParentID: &parentID,
		Identity: ResourceIdentity{IPAddresses: []string{"10.0.0.10", "fd00::10"}},
		Proxmox: &ProxmoxData{
			NodeName:   "pve-a",
			Instance:   "lab",
			VMID:       101,
			CPUs:       4,
			Uptime:     1234,
			Template:   false,
			LastBackup: lastBackup,
		},
		Metrics: &ResourceMetrics{
			CPU:    &MetricValue{Percent: 12.5},
			Memory: &MetricValue{Used: ptrInt64(2 * 1024), Total: ptrInt64(8 * 1024), Percent: 25},
			Disk:   &MetricValue{Used: ptrInt64(40 * 1024), Total: ptrInt64(100 * 1024), Percent: 40},
			NetIn:  &MetricValue{Value: 123.4},
			NetOut: &MetricValue{Value: 567.8},
		},
	}

	v := NewVMView(r)

	if v.ID() != "vm-1" {
		t.Fatalf("expected ID %q, got %q", "vm-1", v.ID())
	}
	if v.Name() != "app-vm" {
		t.Fatalf("expected Name %q, got %q", "app-vm", v.Name())
	}
	if v.Status() != StatusOnline {
		t.Fatalf("expected Status %q, got %q", StatusOnline, v.Status())
	}
	if v.VMID() != 101 {
		t.Fatalf("expected VMID %d, got %d", 101, v.VMID())
	}
	if v.Node() != "pve-a" {
		t.Fatalf("expected Node %q, got %q", "pve-a", v.Node())
	}
	if v.Instance() != "lab" {
		t.Fatalf("expected Instance %q, got %q", "lab", v.Instance())
	}
	if v.Template() != false {
		t.Fatalf("expected Template=false, got %v", v.Template())
	}
	if v.CPUs() != 4 {
		t.Fatalf("expected CPUs %d, got %d", 4, v.CPUs())
	}
	if v.Uptime() != 1234 {
		t.Fatalf("expected Uptime %d, got %d", 1234, v.Uptime())
	}
	if !v.LastBackup().Equal(lastBackup) {
		t.Fatalf("expected LastBackup %v, got %v", lastBackup, v.LastBackup())
	}
	assertStringSlice(t, v.Tags(), []string{"prod", "tier:web"})
	if !v.LastSeen().Equal(now) {
		t.Fatalf("expected LastSeen %v, got %v", now, v.LastSeen())
	}
	if v.ParentID() != parentID {
		t.Fatalf("expected ParentID %q, got %q", parentID, v.ParentID())
	}
	if v.ParentName() != "" {
		t.Fatalf("expected ParentName %q, got %q", "", v.ParentName())
	}
	if v.CPUPercent() != 12.5 {
		t.Fatalf("expected CPUPercent %v, got %v", 12.5, v.CPUPercent())
	}
	if v.MemoryUsed() != 2*1024 {
		t.Fatalf("expected MemoryUsed %d, got %d", 2*1024, v.MemoryUsed())
	}
	if v.MemoryTotal() != 8*1024 {
		t.Fatalf("expected MemoryTotal %d, got %d", 8*1024, v.MemoryTotal())
	}
	if v.MemoryPercent() != 25 {
		t.Fatalf("expected MemoryPercent %v, got %v", 25.0, v.MemoryPercent())
	}
	if v.DiskUsed() != 40*1024 {
		t.Fatalf("expected DiskUsed %d, got %d", 40*1024, v.DiskUsed())
	}
	if v.DiskTotal() != 100*1024 {
		t.Fatalf("expected DiskTotal %d, got %d", 100*1024, v.DiskTotal())
	}
	if v.DiskPercent() != 40 {
		t.Fatalf("expected DiskPercent %v, got %v", 40.0, v.DiskPercent())
	}
	if v.NetIn() != 123.4 {
		t.Fatalf("expected NetIn %v, got %v", 123.4, v.NetIn())
	}
	if v.NetOut() != 567.8 {
		t.Fatalf("expected NetOut %v, got %v", 567.8, v.NetOut())
	}
	assertStringSlice(t, v.IPAddresses(), []string{"10.0.0.10", "fd00::10"})

	t.Run("NilResourceIsSafe", func(t *testing.T) {
		var zero VMView
		if zero.ID() != "" ||
			zero.Name() != "" ||
			zero.Status() != "" ||
			zero.VMID() != 0 ||
			zero.Node() != "" ||
			zero.Instance() != "" ||
			zero.Template() != false ||
			zero.CPUs() != 0 ||
			zero.Uptime() != 0 ||
			!zero.LastBackup().IsZero() ||
			zero.Tags() != nil ||
			!zero.LastSeen().IsZero() ||
			zero.ParentID() != "" ||
			zero.ParentName() != "" ||
			zero.CPUPercent() != 0 ||
			zero.MemoryUsed() != 0 ||
			zero.MemoryTotal() != 0 ||
			zero.MemoryPercent() != 0 ||
			zero.DiskUsed() != 0 ||
			zero.DiskTotal() != 0 ||
			zero.DiskPercent() != 0 ||
			zero.NetIn() != 0 ||
			zero.NetOut() != 0 ||
			zero.IPAddresses() != nil {
			t.Fatalf("expected zero values for nil resource, got %+v", zero)
		}
	})

	t.Run("NilProxmoxAndMetricsAreSafe", func(t *testing.T) {
		r := testResource(ResourceTypeVM)
		r.Proxmox = nil
		r.Metrics = nil
		r.Tags = nil
		r.Identity = ResourceIdentity{}
		v := NewVMView(r)

		if v.VMID() != 0 || v.Node() != "" || v.Instance() != "" || v.Template() != false || v.CPUs() != 0 || v.Uptime() != 0 || !v.LastBackup().IsZero() {
			t.Fatalf("expected proxmox accessors to return zero values when Proxmox is nil")
		}
		if v.CPUPercent() != 0 || v.MemoryUsed() != 0 || v.MemoryTotal() != 0 || v.MemoryPercent() != 0 || v.DiskUsed() != 0 || v.DiskTotal() != 0 || v.DiskPercent() != 0 || v.NetIn() != 0 || v.NetOut() != 0 {
			t.Fatalf("expected metric accessors to return zero values when Metrics is nil")
		}
		if v.Tags() != nil || v.IPAddresses() != nil {
			t.Fatalf("expected nil slices when fields are unset, got tags=%v ips=%v", v.Tags(), v.IPAddresses())
		}
	})
}

func TestView_ContainerViewAccessors(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 1, 0, 0, time.UTC)
	lastBackup := time.Date(2026, 2, 9, 9, 0, 0, 0, time.UTC)
	parentID := "host-parent-2"

	r := &Resource{
		ID:       "ct-1",
		Type:     ResourceTypeSystemContainer,
		Name:     "db-ct",
		Status:   StatusWarning,
		LastSeen: now,
		Tags:     []string{"prod", "tier:db"},
		ParentID: &parentID,
		Identity: ResourceIdentity{IPAddresses: []string{"10.0.0.20"}},
		Proxmox: &ProxmoxData{
			NodeName:   "pve-b",
			Instance:   "lab",
			VMID:       201,
			CPUs:       2,
			Uptime:     888,
			Template:   true,
			LastBackup: lastBackup,
		},
		Metrics: &ResourceMetrics{
			CPU:    &MetricValue{Percent: 3},
			Memory: &MetricValue{Used: ptrInt64(512), Total: ptrInt64(1024), Percent: 50},
			Disk:   &MetricValue{Used: ptrInt64(10), Total: ptrInt64(100), Percent: 10},
			NetIn:  &MetricValue{Value: 1.5},
			NetOut: &MetricValue{Value: 2.5},
		},
	}

	v := NewContainerView(r)

	if v.ID() != "ct-1" || v.Name() != "db-ct" || v.Status() != StatusWarning {
		t.Fatalf("expected basic accessors to match resource; got id=%q name=%q status=%q", v.ID(), v.Name(), v.Status())
	}
	if v.VMID() != 201 || v.Node() != "pve-b" || v.Instance() != "lab" || v.Template() != true || v.CPUs() != 2 || v.Uptime() != 888 {
		t.Fatalf("expected proxmox accessors to match, got vmid=%d node=%q instance=%q template=%v cpus=%d uptime=%d", v.VMID(), v.Node(), v.Instance(), v.Template(), v.CPUs(), v.Uptime())
	}
	if !v.LastBackup().Equal(lastBackup) {
		t.Fatalf("expected LastBackup %v, got %v", lastBackup, v.LastBackup())
	}
	assertStringSlice(t, v.Tags(), []string{"prod", "tier:db"})
	if !v.LastSeen().Equal(now) {
		t.Fatalf("expected LastSeen %v, got %v", now, v.LastSeen())
	}
	if v.ParentID() != parentID || v.ParentName() != "" {
		t.Fatalf("expected parent id=%q name=%q, got id=%q name=%q", parentID, "", v.ParentID(), v.ParentName())
	}
	if v.CPUPercent() != 3 || v.MemoryUsed() != 512 || v.MemoryTotal() != 1024 || v.MemoryPercent() != 50 || v.DiskUsed() != 10 || v.DiskTotal() != 100 || v.DiskPercent() != 10 || v.NetIn() != 1.5 || v.NetOut() != 2.5 {
		t.Fatalf("expected metric accessors to match, got cpu=%v memUsed=%d memTotal=%d memPct=%v diskUsed=%d diskTotal=%d diskPct=%v netIn=%v netOut=%v",
			v.CPUPercent(), v.MemoryUsed(), v.MemoryTotal(), v.MemoryPercent(), v.DiskUsed(), v.DiskTotal(), v.DiskPercent(), v.NetIn(), v.NetOut())
	}
	assertStringSlice(t, v.IPAddresses(), []string{"10.0.0.20"})

	t.Run("NilResourceIsSafe", func(t *testing.T) {
		var zero ContainerView
		if zero.ID() != "" ||
			zero.Name() != "" ||
			zero.Status() != "" ||
			zero.VMID() != 0 ||
			zero.Node() != "" ||
			zero.Instance() != "" ||
			zero.Template() != false ||
			zero.CPUs() != 0 ||
			zero.Uptime() != 0 ||
			!zero.LastBackup().IsZero() ||
			zero.Tags() != nil ||
			!zero.LastSeen().IsZero() ||
			zero.ParentID() != "" ||
			zero.ParentName() != "" ||
			zero.CPUPercent() != 0 ||
			zero.MemoryUsed() != 0 ||
			zero.MemoryTotal() != 0 ||
			zero.MemoryPercent() != 0 ||
			zero.DiskUsed() != 0 ||
			zero.DiskTotal() != 0 ||
			zero.DiskPercent() != 0 ||
			zero.NetIn() != 0 ||
			zero.NetOut() != 0 ||
			zero.IPAddresses() != nil {
			t.Fatalf("expected zero values for nil resource, got %+v", zero)
		}
	})

	t.Run("NilProxmoxAndMetricsAreSafe", func(t *testing.T) {
		r := testResource(ResourceTypeSystemContainer)
		r.Proxmox = nil
		r.Metrics = nil
		r.Tags = nil
		r.Identity = ResourceIdentity{}
		v := NewContainerView(r)

		if v.VMID() != 0 || v.Node() != "" || v.Instance() != "" || v.Template() != false || v.CPUs() != 0 || v.Uptime() != 0 || !v.LastBackup().IsZero() {
			t.Fatalf("expected proxmox accessors to return zero values when Proxmox is nil")
		}
		if v.CPUPercent() != 0 || v.MemoryUsed() != 0 || v.MemoryTotal() != 0 || v.MemoryPercent() != 0 || v.DiskUsed() != 0 || v.DiskTotal() != 0 || v.DiskPercent() != 0 || v.NetIn() != 0 || v.NetOut() != 0 {
			t.Fatalf("expected metric accessors to return zero values when Metrics is nil")
		}
		if v.Tags() != nil || v.IPAddresses() != nil {
			t.Fatalf("expected nil slices when fields are unset, got tags=%v ips=%v", v.Tags(), v.IPAddresses())
		}
	})
}

func TestView_NodeViewAccessors(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 2, 0, 0, time.UTC)
	temp := 66.6

	r := &Resource{
		ID:       "node-1",
		Type:     ResourceTypeHost,
		Name:     "pve-node-1",
		Status:   StatusOnline,
		LastSeen: now,
		Tags:     []string{"pve", "rack:1"},
		Proxmox: &ProxmoxData{
			NodeName:          "pve-1",
			ClusterName:       "cluster-a",
			Instance:          "lab",
			PVEVersion:        "8.2.2",
			KernelVersion:     "6.8.0",
			Uptime:            999,
			Temperature:       &temp,
			CPUInfo:           &CPUInfo{Model: "Xeon", Cores: 8, Sockets: 2},
			LoadAverage:       []float64{0.12, 0.34, 0.56},
			PendingUpdates:    7,
			LinkedHostAgentID: "agent-123",
			CPUs:              4, // should be ignored when CPUInfo is valid
		},
		Metrics: &ResourceMetrics{
			CPU:    &MetricValue{Percent: 91},
			Memory: &MetricValue{Used: ptrInt64(1), Total: ptrInt64(10), Percent: 10},
			Disk:   &MetricValue{Used: ptrInt64(2), Total: ptrInt64(20), Percent: 10},
		},
	}

	v := NewNodeView(r)
	if v.ID() != "node-1" || v.Name() != "pve-node-1" || v.Status() != StatusOnline {
		t.Fatalf("expected basic accessors to match, got id=%q name=%q status=%q", v.ID(), v.Name(), v.Status())
	}
	if v.NodeName() != "pve-1" || v.ClusterName() != "cluster-a" || v.Instance() != "lab" {
		t.Fatalf("expected proxmox name/cluster/instance accessors to match, got nodeName=%q clusterName=%q instance=%q", v.NodeName(), v.ClusterName(), v.Instance())
	}
	if v.PVEVersion() != "8.2.2" || v.KernelVersion() != "6.8.0" || v.Uptime() != 999 {
		t.Fatalf("expected version/uptime accessors to match, got pve=%q kernel=%q uptime=%d", v.PVEVersion(), v.KernelVersion(), v.Uptime())
	}
	if v.CPUs() != 16 {
		t.Fatalf("expected CPUs() to return cores*sockets=%d, got %d", 16, v.CPUs())
	}
	if v.Temperature() != temp || !v.HasTemperature() {
		t.Fatalf("expected temperature=%v and HasTemperature=true, got temperature=%v has=%v", temp, v.Temperature(), v.HasTemperature())
	}
	assertFloatSlice(t, v.LoadAverage(), []float64{0.12, 0.34, 0.56})
	if v.PendingUpdates() != 7 {
		t.Fatalf("expected PendingUpdates %d, got %d", 7, v.PendingUpdates())
	}
	if v.LinkedHostAgentID() != "agent-123" {
		t.Fatalf("expected LinkedHostAgentID %q, got %q", "agent-123", v.LinkedHostAgentID())
	}
	if v.CPUPercent() != 91 || v.MemoryUsed() != 1 || v.MemoryTotal() != 10 || v.MemoryPercent() != 10 || v.DiskUsed() != 2 || v.DiskTotal() != 20 || v.DiskPercent() != 10 {
		t.Fatalf("expected metric accessors to match, got cpu=%v memUsed=%d memTotal=%d memPct=%v diskUsed=%d diskTotal=%d diskPct=%v",
			v.CPUPercent(), v.MemoryUsed(), v.MemoryTotal(), v.MemoryPercent(), v.DiskUsed(), v.DiskTotal(), v.DiskPercent())
	}
	assertStringSlice(t, v.Tags(), []string{"pve", "rack:1"})
	if !v.LastSeen().Equal(now) {
		t.Fatalf("expected LastSeen %v, got %v", now, v.LastSeen())
	}

	t.Run("CPUsFallbackToProxmoxCPUs", func(t *testing.T) {
		r := testResource(ResourceTypeHost)
		r.Name = "fallback-node"
		r.Proxmox = &ProxmoxData{
			CPUs:    12,
			CPUInfo: &CPUInfo{Cores: 0, Sockets: 2},
		}
		v := NewNodeView(r)
		if v.CPUs() != 12 {
			t.Fatalf("expected fallback CPUs=%d, got %d", 12, v.CPUs())
		}
	})
}

func TestView_HostViewAccessors(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 3, 0, 0, time.UTC)
	temp := 55.5
	speed := int64(1000)
	r := &Resource{
		ID:       "host-1",
		Type:     ResourceTypeHost,
		Name:     "agent-host-1",
		Status:   StatusOnline,
		LastSeen: now,
		Tags:     []string{"linux", "site:1"},
		Agent: &AgentData{
			AgentID:       "agent-1",
			AgentVersion:  "1.2.3",
			Hostname:      "agent-host-1",
			Platform:      "linux",
			OSName:        "Ubuntu",
			OSVersion:     "24.04",
			KernelVersion: "6.8.0",
			Architecture:  "amd64",
			UptimeSeconds: 3600,
			Temperature:   &temp,
			NetworkInterfaces: []NetworkInterface{
				{Name: "eth0", MAC: "aa:bb:cc:dd:ee:ff", Addresses: []string{"10.0.0.30/24"}, SpeedMbps: &speed, Status: "up"},
			},
			Disks: []DiskInfo{
				{Device: "/dev/sda1", Mountpoint: "/", Filesystem: "ext4", Total: 100, Used: 60, Free: 40},
			},
			LinkedNodeID:      "node-99",
			LinkedVMID:        "vm-99",
			LinkedContainerID: "ct-99",
		},
		Metrics: &ResourceMetrics{
			CPU:    &MetricValue{Percent: 7.7},
			Memory: &MetricValue{Used: ptrInt64(1), Total: ptrInt64(4), Percent: 25},
			Disk:   &MetricValue{Percent: 66},
		},
	}

	v := NewHostView(r)
	if v.ID() != "host-1" || v.Name() != "agent-host-1" {
		t.Fatalf("expected ID/Name to match, got id=%q name=%q", v.ID(), v.Name())
	}
	if v.Hostname() != "agent-host-1" || v.Platform() != "linux" || v.OSName() != "Ubuntu" || v.OSVersion() != "24.04" || v.KernelVersion() != "6.8.0" || v.Architecture() != "amd64" {
		t.Fatalf("expected agent OS accessors to match, got hostname=%q platform=%q os=%q %q kernel=%q arch=%q",
			v.Hostname(), v.Platform(), v.OSName(), v.OSVersion(), v.KernelVersion(), v.Architecture())
	}
	if v.AgentVersion() != "1.2.3" || v.AgentID() != "agent-1" {
		t.Fatalf("expected AgentVersion/AgentID to match, got version=%q id=%q", v.AgentVersion(), v.AgentID())
	}
	if v.UptimeSeconds() != 3600 {
		t.Fatalf("expected UptimeSeconds %d, got %d", 3600, v.UptimeSeconds())
	}
	if v.Temperature() != temp || !v.HasTemperature() {
		t.Fatalf("expected temperature=%v and HasTemperature=true, got temperature=%v has=%v", temp, v.Temperature(), v.HasTemperature())
	}
	if len(v.NetworkInterfaces()) != 1 || v.NetworkInterfaces()[0].Name != "eth0" || v.NetworkInterfaces()[0].SpeedMbps == nil || *v.NetworkInterfaces()[0].SpeedMbps != speed {
		t.Fatalf("expected network interface data, got %+v", v.NetworkInterfaces())
	}
	if len(v.Disks()) != 1 || v.Disks()[0].Device != "/dev/sda1" {
		t.Fatalf("expected disk data, got %+v", v.Disks())
	}
	if v.LinkedNodeID() != "node-99" || v.LinkedVMID() != "vm-99" || v.LinkedContainerID() != "ct-99" {
		t.Fatalf("expected linked IDs to match, got node=%q vm=%q ct=%q", v.LinkedNodeID(), v.LinkedVMID(), v.LinkedContainerID())
	}
	if v.Status() != StatusOnline {
		t.Fatalf("expected Status %q, got %q", StatusOnline, v.Status())
	}
	assertStringSlice(t, v.Tags(), []string{"linux", "site:1"})
	if !v.LastSeen().Equal(now) {
		t.Fatalf("expected LastSeen %v, got %v", now, v.LastSeen())
	}
	if v.CPUPercent() != 7.7 || v.MemoryUsed() != 1 || v.MemoryTotal() != 4 || v.MemoryPercent() != 25 || v.DiskPercent() != 66 {
		t.Fatalf("expected metric accessors to match, got cpu=%v memUsed=%d memTotal=%d memPct=%v diskPct=%v",
			v.CPUPercent(), v.MemoryUsed(), v.MemoryTotal(), v.MemoryPercent(), v.DiskPercent())
	}
}

func TestView_DockerHostViewAccessors(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 4, 0, 0, time.UTC)
	temp := 44.4
	speed := int64(2500)
	swarm := &DockerSwarmInfo{NodeID: "swarm-node-1", NodeRole: "manager", LocalState: "active", ClusterID: "swarm-1", ClusterName: "prod"}

	r := &Resource{
		ID:         "dockerhost-1",
		Type:       ResourceTypeHost,
		Name:       "docker-host-1",
		Status:     StatusWarning,
		LastSeen:   now,
		Tags:       []string{"docker"},
		ChildCount: 2,
		Docker: &DockerData{
			Hostname:       "docker-host-1",
			DockerVersion:  "25.0.0",
			RuntimeVersion: "1.7.0",
			OS:             "Ubuntu",
			KernelVersion:  "6.8.0",
			Architecture:   "amd64",
			AgentVersion:   "2.0.0",
			UptimeSeconds:  7200,
			Temperature:    &temp,
			Swarm:          swarm,
			NetworkInterfaces: []NetworkInterface{
				{Name: "eno1", Addresses: []string{"10.0.0.40/24"}, SpeedMbps: &speed},
			},
			Disks: []DiskInfo{{Device: "/dev/nvme0n1p1", Total: 1000, Used: 100, Free: 900}},
		},
		Metrics: &ResourceMetrics{
			CPU:    &MetricValue{Percent: 22},
			Memory: &MetricValue{Percent: 33},
		},
	}

	v := NewDockerHostView(r)
	if v.ID() != "dockerhost-1" || v.Name() != "docker-host-1" || v.Status() != StatusWarning {
		t.Fatalf("expected basic accessors to match, got id=%q name=%q status=%q", v.ID(), v.Name(), v.Status())
	}
	if v.Hostname() != "docker-host-1" || v.DockerVersion() != "25.0.0" || v.RuntimeVersion() != "1.7.0" || v.OS() != "Ubuntu" {
		t.Fatalf("expected docker accessors to match, got hostname=%q docker=%q runtime=%q os=%q", v.Hostname(), v.DockerVersion(), v.RuntimeVersion(), v.OS())
	}
	if v.KernelVersion() != "6.8.0" || v.Architecture() != "amd64" || v.AgentVersion() != "2.0.0" {
		t.Fatalf("expected kernel/arch/agentVersion to match, got kernel=%q arch=%q agent=%q", v.KernelVersion(), v.Architecture(), v.AgentVersion())
	}
	if v.UptimeSeconds() != 7200 {
		t.Fatalf("expected uptime %d, got %d", 7200, v.UptimeSeconds())
	}
	if v.Temperature() != temp || !v.HasTemperature() {
		t.Fatalf("expected temperature=%v and HasTemperature=true, got temperature=%v has=%v", temp, v.Temperature(), v.HasTemperature())
	}
	if v.Swarm() == nil || v.Swarm().ClusterID != "swarm-1" || v.Swarm().NodeRole != "manager" {
		t.Fatalf("expected swarm info, got %+v", v.Swarm())
	}
	if len(v.NetworkInterfaces()) != 1 || v.NetworkInterfaces()[0].Name != "eno1" {
		t.Fatalf("expected network interface data, got %+v", v.NetworkInterfaces())
	}
	if len(v.Disks()) != 1 || v.Disks()[0].Device != "/dev/nvme0n1p1" {
		t.Fatalf("expected disk data, got %+v", v.Disks())
	}
	assertStringSlice(t, v.Tags(), []string{"docker"})
	if !v.LastSeen().Equal(now) {
		t.Fatalf("expected LastSeen %v, got %v", now, v.LastSeen())
	}
	if v.CPUPercent() != 22 || v.MemoryPercent() != 33 {
		t.Fatalf("expected cpu/memory percents %v/%v, got %v/%v", 22.0, 33.0, v.CPUPercent(), v.MemoryPercent())
	}
	if v.ChildCount() != 2 {
		t.Fatalf("expected ChildCount %d, got %d", 2, v.ChildCount())
	}
}

func TestView_StoragePoolViewAccessors(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 5, 0, 0, time.UTC)
	parentID := "node-parent-1"
	r := &Resource{
		ID:       "storage-1",
		Type:     ResourceTypeStorage,
		Name:     "local-zfs",
		Status:   StatusOnline,
		LastSeen: now,
		Tags:     []string{"fast"},
		ParentID: &parentID,
		Proxmox:  &ProxmoxData{NodeName: "pve-a", Instance: "lab"},
		Storage: &StorageMeta{
			Type:              "zfspool",
			Content:           "images,iso",
			ContentTypes:      []string{"images", "iso"},
			Shared:            false,
			IsCeph:            false,
			IsZFS:             true,
			ZFSPoolState:      "ONLINE",
			ZFSReadErrors:     1,
			ZFSWriteErrors:    2,
			ZFSChecksumErrors: 3,
		},
		Metrics: &ResourceMetrics{
			Disk: &MetricValue{Used: ptrInt64(10), Total: ptrInt64(100), Percent: 10},
		},
	}

	v := NewStoragePoolView(r)
	if v.ID() != "storage-1" || v.Name() != "local-zfs" || v.Status() != StatusOnline {
		t.Fatalf("expected basic accessors to match, got id=%q name=%q status=%q", v.ID(), v.Name(), v.Status())
	}
	if v.Node() != "pve-a" || v.Instance() != "lab" {
		t.Fatalf("expected node/instance %q/%q, got %q/%q", "pve-a", "lab", v.Node(), v.Instance())
	}
	if v.StorageType() != "zfspool" || v.Content() != "images,iso" || !reflect.DeepEqual(v.ContentTypes(), []string{"images", "iso"}) {
		t.Fatalf("expected storage meta to match, got type=%q content=%q contentTypes=%v", v.StorageType(), v.Content(), v.ContentTypes())
	}
	if v.Shared() != false || v.IsCeph() != false || v.IsZFS() != true {
		t.Fatalf("expected shared/ceph/zfs false/false/true, got %v/%v/%v", v.Shared(), v.IsCeph(), v.IsZFS())
	}
	if v.ZFSPoolState() != "ONLINE" || v.ZFSReadErrors() != 1 || v.ZFSWriteErrors() != 2 || v.ZFSChecksumErrors() != 3 {
		t.Fatalf("expected ZFS fields to match, got state=%q read=%d write=%d cksum=%d", v.ZFSPoolState(), v.ZFSReadErrors(), v.ZFSWriteErrors(), v.ZFSChecksumErrors())
	}
	if v.DiskUsed() != 10 || v.DiskTotal() != 100 || v.DiskPercent() != 10 {
		t.Fatalf("expected disk metrics used/total/percent 10/100/10, got %d/%d/%v", v.DiskUsed(), v.DiskTotal(), v.DiskPercent())
	}
	assertStringSlice(t, v.Tags(), []string{"fast"})
	if !v.LastSeen().Equal(now) {
		t.Fatalf("expected LastSeen %v, got %v", now, v.LastSeen())
	}
	if v.ParentID() != parentID || v.ParentName() != "" {
		t.Fatalf("expected parent id=%q name=%q, got id=%q name=%q", parentID, "", v.ParentID(), v.ParentName())
	}
}

func TestView_PBSAndPMGInstanceViewAccessors(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 6, 0, 0, time.UTC)

	t.Run("PBSInstanceView", func(t *testing.T) {
		r := &Resource{
			ID:        "pbs-1",
			Type:      ResourceTypePBS,
			Name:      "pbs-a",
			Status:    StatusOnline,
			LastSeen:  now,
			Tags:      []string{"backup"},
			CustomURL: "https://pbs.example/ui",
			PBS: &PBSData{
				Hostname:         "pbs.example",
				Version:          "3.2",
				UptimeSeconds:    100,
				DatastoreCount:   2,
				BackupJobCount:   3,
				SyncJobCount:     4,
				VerifyJobCount:   5,
				PruneJobCount:    6,
				GarbageJobCount:  7,
				ConnectionHealth: "online",
			},
			Metrics: &ResourceMetrics{
				CPU:    &MetricValue{Percent: 1},
				Memory: &MetricValue{Percent: 2},
				Disk:   &MetricValue{Percent: 3},
			},
		}
		v := NewPBSInstanceView(r)
		if v.ID() != "pbs-1" || v.Name() != "pbs-a" || v.Status() != StatusOnline {
			t.Fatalf("expected basic accessors to match, got id=%q name=%q status=%q", v.ID(), v.Name(), v.Status())
		}
		if v.Hostname() != "pbs.example" || v.Version() != "3.2" || v.UptimeSeconds() != 100 {
			t.Fatalf("expected hostname/version/uptime to match, got %q/%q/%d", v.Hostname(), v.Version(), v.UptimeSeconds())
		}
		if v.DatastoreCount() != 2 || v.BackupJobCount() != 3 || v.SyncJobCount() != 4 || v.VerifyJobCount() != 5 || v.PruneJobCount() != 6 || v.GarbageJobCount() != 7 {
			t.Fatalf("expected pbs job/datastore counts to match, got ds=%d backup=%d sync=%d verify=%d prune=%d garbage=%d",
				v.DatastoreCount(), v.BackupJobCount(), v.SyncJobCount(), v.VerifyJobCount(), v.PruneJobCount(), v.GarbageJobCount())
		}
		if v.ConnectionHealth() != "online" {
			t.Fatalf("expected connection health %q, got %q", "online", v.ConnectionHealth())
		}
		if v.CPUPercent() != 1 || v.MemoryPercent() != 2 || v.DiskPercent() != 3 {
			t.Fatalf("expected cpu/memory/disk percents 1/2/3, got %v/%v/%v", v.CPUPercent(), v.MemoryPercent(), v.DiskPercent())
		}
		assertStringSlice(t, v.Tags(), []string{"backup"})
		if !v.LastSeen().Equal(now) {
			t.Fatalf("expected LastSeen %v, got %v", now, v.LastSeen())
		}
		if v.CustomURL() != "https://pbs.example/ui" {
			t.Fatalf("expected CustomURL %q, got %q", "https://pbs.example/ui", v.CustomURL())
		}
	})

	t.Run("PMGInstanceView", func(t *testing.T) {
		r := &Resource{
			ID:        "pmg-1",
			Type:      ResourceTypePMG,
			Name:      "pmg-a",
			Status:    StatusWarning,
			LastSeen:  now,
			Tags:      []string{"mail"},
			CustomURL: "https://pmg.example/ui",
			PMG: &PMGData{
				Hostname:         "pmg.example",
				Version:          "8.0",
				NodeCount:        2,
				UptimeSeconds:    200,
				QueueActive:      10,
				QueueDeferred:    20,
				QueueTotal:       30,
				MailCountTotal:   1000,
				SpamIn:           11,
				VirusIn:          22,
				ConnectionHealth: "online",
			},
			Metrics: &ResourceMetrics{
				CPU:    &MetricValue{Percent: 4},
				Memory: &MetricValue{Percent: 5},
				Disk:   &MetricValue{Percent: 6},
			},
		}
		v := NewPMGInstanceView(r)
		if v.ID() != "pmg-1" || v.Name() != "pmg-a" || v.Status() != StatusWarning {
			t.Fatalf("expected basic accessors to match, got id=%q name=%q status=%q", v.ID(), v.Name(), v.Status())
		}
		if v.Hostname() != "pmg.example" || v.Version() != "8.0" || v.NodeCount() != 2 || v.UptimeSeconds() != 200 {
			t.Fatalf("expected pmg fields to match, got host=%q ver=%q nodes=%d uptime=%d", v.Hostname(), v.Version(), v.NodeCount(), v.UptimeSeconds())
		}
		if v.QueueActive() != 10 || v.QueueDeferred() != 20 || v.QueueTotal() != 30 {
			t.Fatalf("expected queue active/deferred/total 10/20/30, got %d/%d/%d", v.QueueActive(), v.QueueDeferred(), v.QueueTotal())
		}
		if v.MailCountTotal() != 1000 || v.SpamIn() != 11 || v.VirusIn() != 22 {
			t.Fatalf("expected mail stats total/spam/virus 1000/11/22, got %v/%v/%v", v.MailCountTotal(), v.SpamIn(), v.VirusIn())
		}
		if v.ConnectionHealth() != "online" {
			t.Fatalf("expected connection health %q, got %q", "online", v.ConnectionHealth())
		}
		if v.CPUPercent() != 4 || v.MemoryPercent() != 5 || v.DiskPercent() != 6 {
			t.Fatalf("expected cpu/memory/disk percents 4/5/6, got %v/%v/%v", v.CPUPercent(), v.MemoryPercent(), v.DiskPercent())
		}
		assertStringSlice(t, v.Tags(), []string{"mail"})
		if !v.LastSeen().Equal(now) {
			t.Fatalf("expected LastSeen %v, got %v", now, v.LastSeen())
		}
		if v.CustomURL() != "https://pmg.example/ui" {
			t.Fatalf("expected CustomURL %q, got %q", "https://pmg.example/ui", v.CustomURL())
		}
	})
}

func TestView_K8sClusterViewAccessors(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 7, 0, 0, time.UTC)
	caps := &K8sMetricCapabilities{NodeCPUMemory: true, PodCPUMemory: true}
	r := &Resource{
		ID:         "k8s-1",
		Type:       ResourceTypeK8sCluster,
		Name:       "prod-k8s",
		Status:     StatusOnline,
		LastSeen:   now,
		Tags:       []string{"k8s"},
		ChildCount: 3,
		Kubernetes: &K8sData{
			ClusterID:          "cluster-1",
			ClusterName:        "prod-k8s",
			AgentID:            "agent-k8s-1",
			Context:            "prod",
			Server:             "https://k8s.example",
			Version:            "1.31.2",
			PendingUninstall:   true,
			MetricCapabilities: caps,
		},
		Metrics: &ResourceMetrics{
			CPU:    &MetricValue{Percent: 10},
			Memory: &MetricValue{Percent: 20},
		},
	}

	v := NewK8sClusterView(r)
	if v.ID() != "k8s-1" || v.Name() != "prod-k8s" || v.Status() != StatusOnline {
		t.Fatalf("expected basic accessors to match, got id=%q name=%q status=%q", v.ID(), v.Name(), v.Status())
	}
	if v.ClusterID() != "cluster-1" || v.ClusterName() != "prod-k8s" || v.AgentID() != "agent-k8s-1" || v.Context() != "prod" || v.Server() != "https://k8s.example" || v.Version() != "1.31.2" {
		t.Fatalf("expected k8s fields to match, got id=%q name=%q agent=%q ctx=%q server=%q ver=%q",
			v.ClusterID(), v.ClusterName(), v.AgentID(), v.Context(), v.Server(), v.Version())
	}
	if v.PendingUninstall() != true {
		t.Fatalf("expected PendingUninstall=true, got %v", v.PendingUninstall())
	}
	if v.MetricCapabilities() == nil || v.MetricCapabilities().NodeCPUMemory != true || v.MetricCapabilities().PodCPUMemory != true {
		t.Fatalf("expected metric capabilities to be present, got %+v", v.MetricCapabilities())
	}
	if v.CPUPercent() != 10 || v.MemoryPercent() != 20 {
		t.Fatalf("expected cpu/memory 10/20, got %v/%v", v.CPUPercent(), v.MemoryPercent())
	}
	assertStringSlice(t, v.Tags(), []string{"k8s"})
	if !v.LastSeen().Equal(now) {
		t.Fatalf("expected LastSeen %v, got %v", now, v.LastSeen())
	}
	if v.ChildCount() != 3 {
		t.Fatalf("expected ChildCount %d, got %d", 3, v.ChildCount())
	}
}

func TestView_WorkloadView(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 8, 0, 0, time.UTC)

	t.Run("VMWorkload", func(t *testing.T) {
		r := &Resource{
			ID:       "vm-w",
			Type:     ResourceTypeVM,
			Name:     "vm-name",
			Status:   StatusOnline,
			LastSeen: now,
			Tags:     []string{"vm"},
			Proxmox:  &ProxmoxData{VMID: 100, NodeName: "pve-a", Instance: "lab"},
			Metrics:  &ResourceMetrics{CPU: &MetricValue{Percent: 1}, Memory: &MetricValue{Percent: 2}, Disk: &MetricValue{Percent: 3}},
		}
		v := NewWorkloadView(r)
		if !v.IsVM() || v.IsContainer() {
			t.Fatalf("expected IsVM=true and IsContainer=false, got vm=%v ct=%v", v.IsVM(), v.IsContainer())
		}
		if v.ID() != "vm-w" || v.Name() != "vm-name" || v.Type() != ResourceTypeVM || v.Status() != StatusOnline {
			t.Fatalf("expected basic fields to match")
		}
		if v.VMID() != 100 || v.Node() != "pve-a" || v.Instance() != "lab" {
			t.Fatalf("expected proxmox fields to match")
		}
		if v.CPUPercent() != 1 || v.MemoryPercent() != 2 || v.DiskPercent() != 3 {
			t.Fatalf("expected metric percents 1/2/3, got %v/%v/%v", v.CPUPercent(), v.MemoryPercent(), v.DiskPercent())
		}
		assertStringSlice(t, v.Tags(), []string{"vm"})
		if !v.LastSeen().Equal(now) {
			t.Fatalf("expected LastSeen %v, got %v", now, v.LastSeen())
		}
	})

	t.Run("ContainerWorkload", func(t *testing.T) {
		r := &Resource{
			ID:       "ct-w",
			Type:     ResourceTypeSystemContainer,
			Name:     "ct-name",
			Status:   StatusWarning,
			LastSeen: now,
			Tags:     []string{"ct"},
			Proxmox:  &ProxmoxData{VMID: 200, NodeName: "pve-b", Instance: "lab"},
			Metrics:  &ResourceMetrics{CPU: &MetricValue{Percent: 4}},
		}
		v := NewWorkloadView(r)
		if v.IsVM() || !v.IsContainer() {
			t.Fatalf("expected IsVM=false and IsContainer=true, got vm=%v ct=%v", v.IsVM(), v.IsContainer())
		}
		if v.Type() != ResourceTypeSystemContainer || v.VMID() != 200 {
			t.Fatalf("expected type/vmid to match")
		}
	})

	t.Run("NilIsSafe", func(t *testing.T) {
		var zero WorkloadView
		if zero.IsVM() || zero.IsContainer() || zero.ID() != "" || zero.Name() != "" || zero.Type() != "" || zero.Status() != "" {
			t.Fatalf("expected nil workload view to return zero values")
		}
	})
}

func TestView_InfrastructureView(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 9, 0, 0, time.UTC)
	r := &Resource{
		ID:         "infra-1",
		Type:       ResourceTypeHost,
		Name:       "infra-host",
		Status:     StatusOnline,
		LastSeen:   now,
		Tags:       []string{"infra"},
		ChildCount: 5,
		Proxmox:    &ProxmoxData{NodeName: "pve-a"},
		Agent:      &AgentData{Hostname: "infra-host"},
		Docker:     &DockerData{Hostname: "infra-host"},
		Metrics:    &ResourceMetrics{CPU: &MetricValue{Percent: 1}, Memory: &MetricValue{Percent: 2}, Disk: &MetricValue{Percent: 3}},
	}

	v := NewInfrastructureView(r)
	if v.ID() != "infra-1" || v.Name() != "infra-host" || v.Status() != StatusOnline {
		t.Fatalf("expected basic fields to match")
	}
	if v.CPUPercent() != 1 || v.MemoryPercent() != 2 || v.DiskPercent() != 3 {
		t.Fatalf("expected metric percents 1/2/3, got %v/%v/%v", v.CPUPercent(), v.MemoryPercent(), v.DiskPercent())
	}
	assertStringSlice(t, v.Tags(), []string{"infra"})
	if !v.LastSeen().Equal(now) {
		t.Fatalf("expected LastSeen %v, got %v", now, v.LastSeen())
	}
	if !v.HasProxmox() || !v.HasAgent() || !v.HasDocker() {
		t.Fatalf("expected HasProxmox/HasAgent/HasDocker all true, got %v/%v/%v", v.HasProxmox(), v.HasAgent(), v.HasDocker())
	}
	if v.ChildCount() != 5 {
		t.Fatalf("expected ChildCount %d, got %d", 5, v.ChildCount())
	}

	t.Run("NilIsSafe", func(t *testing.T) {
		var zero InfrastructureView
		if zero.HasProxmox() || zero.HasAgent() || zero.HasDocker() || zero.ID() != "" || zero.Name() != "" || zero.Status() != "" || zero.ChildCount() != 0 {
			t.Fatalf("expected nil infrastructure view to return zero values")
		}
	})
}

func TestView_RegistryCachedAccessors(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 10, 0, 0, time.UTC)

	snapshot := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:          "node-src-1",
				Name:        "pve-a",
				DisplayName: "b-node",
				Instance:    "lab",
				Host:        "https://pve-a.example:8006",
				Status:      "online",
				CPUInfo:     models.CPUInfo{Model: "Xeon", Cores: 4, Sockets: 1},
				LastSeen:    now,
			},
			{
				ID:          "node-src-2",
				Name:        "pve-b",
				DisplayName: "a-node",
				Instance:    "lab",
				Host:        "https://pve-b.example:8006",
				Status:      "online",
				CPUInfo:     models.CPUInfo{Model: "Xeon", Cores: 4, Sockets: 1},
				LastSeen:    now,
			},
		},
		VMs: []models.VM{
			{ID: "vm-src-2", VMID: 102, Name: "b-vm", Node: "pve-b", Instance: "lab", Status: "running", CPUs: 2, LastSeen: now, IPAddresses: []string{"10.0.0.2"}},
			{ID: "vm-src-1", VMID: 101, Name: "a-vm", Node: "pve-a", Instance: "lab", Status: "running", CPUs: 4, LastSeen: now, IPAddresses: []string{"10.0.0.1"}},
		},
		Containers: []models.Container{
			{ID: "ct-src-2", VMID: 202, Name: "b-ct", Node: "pve-b", Instance: "lab", Status: "running", CPUs: 1, LastSeen: now, IPAddresses: []string{"10.0.1.2"}},
			{ID: "ct-src-1", VMID: 201, Name: "a-ct", Node: "pve-a", Instance: "lab", Status: "running", CPUs: 1, LastSeen: now, IPAddresses: []string{"10.0.1.1"}},
		},
		Hosts: []models.Host{
			{ID: "agent-src-1", Hostname: "host-b", DisplayName: "b-host", MachineID: "machine-agent-b", Status: "online", LastSeen: now, AgentVersion: "1.0"},
			{ID: "agent-src-2", Hostname: "host-a", DisplayName: "a-host", MachineID: "machine-agent-a", Status: "online", LastSeen: now, AgentVersion: "1.0"},
		},
		DockerHosts: []models.DockerHost{
			{
				ID:           "docker-src-1",
				AgentID:      "docker-agent-1",
				Hostname:     "docker-b",
				DisplayName:  "b-docker",
				MachineID:    "machine-docker-b",
				Status:       "online",
				LastSeen:     now,
				CPUs:         2,
				Memory:       models.Memory{Total: 8, Used: 4, Usage: 50},
				Containers:   []models.DockerContainer{{ID: "dockerct-1", Name: "ignored-docker-container", State: "running"}},
				AgentVersion: "2.0",
			},
			{
				ID:           "docker-src-2",
				AgentID:      "docker-agent-2",
				Hostname:     "docker-a",
				DisplayName:  "a-docker",
				MachineID:    "machine-docker-a",
				Status:       "online",
				LastSeen:     now,
				CPUs:         2,
				Memory:       models.Memory{Total: 8, Used: 4, Usage: 50},
				Containers:   nil,
				AgentVersion: "2.0",
			},
		},
		Storage: []models.Storage{
			{ID: "storage-src-2", Name: "b-store", Node: "pve-b", Instance: "lab", Type: "zfspool", Status: "available", Content: "images,iso", Shared: false, Total: 100, Used: 10, Free: 90, Usage: 10},
			{ID: "storage-src-1", Name: "a-store", Node: "pve-a", Instance: "lab", Type: "dir", Status: "available", Content: "images", Shared: true, Total: 100, Used: 10, Free: 90, Usage: 10},
		},
		PBSInstances: []models.PBSInstance{
			{ID: "pbs-src-1", Name: "b-pbs", Host: "https://pbs-b.example:8007", Status: "online", Version: "3.2", LastSeen: now},
			{ID: "pbs-src-2", Name: "a-pbs", Host: "https://pbs-a.example:8007", Status: "online", Version: "3.2", LastSeen: now},
		},
		PMGInstances: []models.PMGInstance{
			{ID: "pmg-src-2", Name: "b-pmg", Host: "https://pmg-b.example:8006", Status: "online", Version: "8.0", LastSeen: now},
			{ID: "pmg-src-1", Name: "a-pmg", Host: "https://pmg-a.example:8006", Status: "online", Version: "8.0", LastSeen: now},
		},
		KubernetesClusters: []models.KubernetesCluster{
			{ID: "k8s-src-1", AgentID: "k8s-agent-1", Name: "b-k8s", Status: "online", LastSeen: now, IntervalSeconds: 30},
			{ID: "k8s-src-2", AgentID: "k8s-agent-2", Name: "a-k8s", Status: "online", LastSeen: now, IntervalSeconds: 30},
		},
	}

	rr := NewRegistry(nil)
	rr.IngestSnapshot(snapshot)

	// Access all typed, cached getters and validate counts and deterministic sort by name.
	vms := rr.VMs()
	if len(vms) != 2 {
		t.Fatalf("expected 2 VMs, got %d", len(vms))
	}
	if vms[0].Name() != "a-vm" || vms[1].Name() != "b-vm" {
		t.Fatalf("expected VMs sorted by name, got %q then %q", vms[0].Name(), vms[1].Name())
	}
	for _, v := range vms {
		if v == nil || v.r == nil || v.r.Type != ResourceTypeVM {
			t.Fatalf("expected VMView to wrap a vm resource, got %+v", v)
		}
	}

	cts := rr.Containers()
	if len(cts) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(cts))
	}
	if cts[0].Name() != "a-ct" || cts[1].Name() != "b-ct" {
		t.Fatalf("expected containers sorted by name, got %q then %q", cts[0].Name(), cts[1].Name())
	}
	for _, v := range cts {
		if v == nil || v.r == nil || v.r.Type != ResourceTypeSystemContainer {
			t.Fatalf("expected ContainerView to wrap an lxc resource, got %+v", v)
		}
	}

	nodes := rr.Nodes()
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	// Name sorting uses the unified resource name (which may use display name).
	if nodes[0].Name() != "a-node" || nodes[1].Name() != "b-node" {
		t.Fatalf("expected nodes sorted by name, got %q then %q", nodes[0].Name(), nodes[1].Name())
	}
	for _, v := range nodes {
		if v == nil || v.r == nil || v.r.Type != ResourceTypeHost || v.r.Proxmox == nil {
			t.Fatalf("expected NodeView to wrap a host resource with proxmox data, got %+v", v)
		}
	}

	hosts := rr.Hosts()
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
	if hosts[0].Name() != "a-host" || hosts[1].Name() != "b-host" {
		t.Fatalf("expected hosts sorted by name, got %q then %q", hosts[0].Name(), hosts[1].Name())
	}
	for _, v := range hosts {
		if v == nil || v.r == nil || v.r.Type != ResourceTypeHost || v.r.Agent == nil {
			t.Fatalf("expected HostView to wrap a host resource with agent data, got %+v", v)
		}
	}

	dockerHosts := rr.DockerHosts()
	if len(dockerHosts) != 2 {
		t.Fatalf("expected 2 docker hosts, got %d", len(dockerHosts))
	}
	if dockerHosts[0].Name() != "a-docker" || dockerHosts[1].Name() != "b-docker" {
		t.Fatalf("expected docker hosts sorted by name, got %q then %q", dockerHosts[0].Name(), dockerHosts[1].Name())
	}
	for _, v := range dockerHosts {
		if v == nil || v.r == nil || v.r.Type != ResourceTypeHost || v.r.Docker == nil {
			t.Fatalf("expected DockerHostView to wrap a host resource with docker data, got %+v", v)
		}
	}

	storage := rr.StoragePools()
	if len(storage) != 2 {
		t.Fatalf("expected 2 storage pools, got %d", len(storage))
	}
	if storage[0].Name() != "a-store" || storage[1].Name() != "b-store" {
		t.Fatalf("expected storage pools sorted by name, got %q then %q", storage[0].Name(), storage[1].Name())
	}
	for _, v := range storage {
		if v == nil || v.r == nil || v.r.Type != ResourceTypeStorage {
			t.Fatalf("expected StoragePoolView to wrap a storage resource, got %+v", v)
		}
	}

	pbs := rr.PBSInstances()
	if len(pbs) != 2 {
		t.Fatalf("expected 2 pbs instances, got %d", len(pbs))
	}
	if pbs[0].Name() != "a-pbs" || pbs[1].Name() != "b-pbs" {
		t.Fatalf("expected pbs instances sorted by name, got %q then %q", pbs[0].Name(), pbs[1].Name())
	}

	pmg := rr.PMGInstances()
	if len(pmg) != 2 {
		t.Fatalf("expected 2 pmg instances, got %d", len(pmg))
	}
	if pmg[0].Name() != "a-pmg" || pmg[1].Name() != "b-pmg" {
		t.Fatalf("expected pmg instances sorted by name, got %q then %q", pmg[0].Name(), pmg[1].Name())
	}

	k8s := rr.K8sClusters()
	if len(k8s) != 2 {
		t.Fatalf("expected 2 k8s clusters, got %d", len(k8s))
	}
	if k8s[0].Name() != "a-k8s" || k8s[1].Name() != "b-k8s" {
		t.Fatalf("expected k8s clusters sorted by name, got %q then %q", k8s[0].Name(), k8s[1].Name())
	}

	workloads := rr.Workloads()
	if len(workloads) != 4 {
		t.Fatalf("expected 4 workloads (VMs + LXCs), got %d", len(workloads))
	}
	if workloads[0].Name() != "a-ct" || workloads[1].Name() != "a-vm" || workloads[2].Name() != "b-ct" || workloads[3].Name() != "b-vm" {
		t.Fatalf("expected workloads sorted by name across VM+LXC, got: [%q %q %q %q]", workloads[0].Name(), workloads[1].Name(), workloads[2].Name(), workloads[3].Name())
	}
	for _, w := range workloads {
		if w == nil || w.r == nil {
			t.Fatalf("expected workload view to be non-nil")
		}
		if w.r.Type != ResourceTypeVM && w.r.Type != ResourceTypeSystemContainer {
			t.Fatalf("expected workload type vm or lxc, got %q", w.r.Type)
		}
	}

	infra := rr.Infrastructure()
	if len(infra) != 6 { // 2 nodes + 2 agent hosts + 2 docker hosts
		t.Fatalf("expected 6 infrastructure resources (all host-type), got %d", len(infra))
	}
	for _, v := range infra {
		if v == nil || v.r == nil || v.r.Type != ResourceTypeHost {
			t.Fatalf("expected infrastructure to include only host-type resources, got %+v", v)
		}
	}

	t.Run("CacheInvalidationOnIngestSnapshot", func(t *testing.T) {
		// Force cache build.
		_ = rr.VMs()

		snapshot2 := snapshot
		snapshot2.VMs = append([]models.VM{}, snapshot.VMs...)
		snapshot2.VMs = append(snapshot2.VMs, models.VM{ID: "vm-src-3", VMID: 103, Name: "c-vm", Node: "pve-a", Instance: "lab", Status: "running", CPUs: 1, LastSeen: now})

		rr.IngestSnapshot(snapshot2)

		vms2 := rr.VMs()
		if len(vms2) != 3 {
			t.Fatalf("expected 3 VMs after second ingest, got %d", len(vms2))
		}
		if vms2[0].Name() != "a-vm" || vms2[1].Name() != "b-vm" || vms2[2].Name() != "c-vm" {
			t.Fatalf("expected VMs sorted by name after second ingest, got %q %q %q", vms2[0].Name(), vms2[1].Name(), vms2[2].Name())
		}
	})
}

func TestView_ReadStateInterfaceUsage(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 11, 0, 0, time.UTC)

	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-iface-1", Name: "pve-i", DisplayName: "iface-node", Instance: "lab", Host: "https://pve-i.example:8006", Status: "online", CPUInfo: models.CPUInfo{Cores: 4, Sockets: 1}, LastSeen: now},
		},
		VMs: []models.VM{
			{ID: "vm-iface-1", VMID: 100, Name: "iface-vm", Node: "pve-i", Instance: "lab", Status: "running", CPUs: 2, LastSeen: now},
		},
		Containers: []models.Container{
			{ID: "ct-iface-1", VMID: 200, Name: "iface-ct", Node: "pve-i", Instance: "lab", Status: "running", CPUs: 1, LastSeen: now},
		},
		Hosts: []models.Host{
			{ID: "host-iface-1", Hostname: "iface-host", DisplayName: "iface-host", MachineID: "machine-iface-host", Status: "online", LastSeen: now, AgentVersion: "1.0"},
		},
		DockerHosts: []models.DockerHost{
			{ID: "docker-iface-1", AgentID: "docker-agent-iface-1", Hostname: "iface-docker", DisplayName: "iface-docker", MachineID: "machine-iface-docker", Status: "online", LastSeen: now, CPUs: 2, Memory: models.Memory{Total: 8, Used: 4, Usage: 50}},
		},
		Storage: []models.Storage{
			{ID: "storage-iface-1", Name: "iface-store", Node: "pve-i", Instance: "lab", Type: "dir", Status: "available", Content: "images", Shared: false, Total: 100, Used: 10, Free: 90, Usage: 10},
		},
		PBSInstances: []models.PBSInstance{
			{ID: "pbs-iface-1", Name: "iface-pbs", Host: "https://pbs-iface.example:8007", Status: "online", Version: "3.2", LastSeen: now},
		},
		PMGInstances: []models.PMGInstance{
			{ID: "pmg-iface-1", Name: "iface-pmg", Host: "https://pmg-iface.example:8006", Status: "online", Version: "8.0", LastSeen: now},
		},
		KubernetesClusters: []models.KubernetesCluster{
			{ID: "k8s-iface-1", AgentID: "k8s-agent-iface-1", Name: "iface-k8s", Status: "online", LastSeen: now, IntervalSeconds: 30},
		},
	})

	var rs ReadState = rr

	if got := rs.VMs(); len(got) != 1 || got[0].Name() != "iface-vm" {
		t.Fatalf("expected 1 VM via ReadState, got %d", len(got))
	}
	if got := rs.Containers(); len(got) != 1 || got[0].Name() != "iface-ct" {
		t.Fatalf("expected 1 container via ReadState, got %d", len(got))
	}
	if got := rs.Nodes(); len(got) != 1 || got[0].Name() != "iface-node" {
		t.Fatalf("expected 1 node via ReadState, got %d", len(got))
	}
	if got := rs.Hosts(); len(got) != 1 || got[0].Name() != "iface-host" {
		t.Fatalf("expected 1 host via ReadState, got %d", len(got))
	}
	if got := rs.DockerHosts(); len(got) != 1 || got[0].Name() != "iface-docker" {
		t.Fatalf("expected 1 docker host via ReadState, got %d", len(got))
	}
	if got := rs.StoragePools(); len(got) != 1 || got[0].Name() != "iface-store" {
		t.Fatalf("expected 1 storage pool via ReadState, got %d", len(got))
	}
	if got := rs.PBSInstances(); len(got) != 1 || got[0].Name() != "iface-pbs" {
		t.Fatalf("expected 1 PBS instance via ReadState, got %d", len(got))
	}
	if got := rs.PMGInstances(); len(got) != 1 || got[0].Name() != "iface-pmg" {
		t.Fatalf("expected 1 PMG instance via ReadState, got %d", len(got))
	}
	if got := rs.K8sClusters(); len(got) != 1 || got[0].Name() != "iface-k8s" {
		t.Fatalf("expected 1 k8s cluster via ReadState, got %d", len(got))
	}
	if got := rs.Workloads(); len(got) != 2 {
		t.Fatalf("expected 2 workloads (vm + lxc) via ReadState, got %d", len(got))
	}
	if got := rs.Infrastructure(); len(got) != 3 {
		t.Fatalf("expected 3 infrastructure resources (node + host + docker) via ReadState, got %d", len(got))
	}
}
