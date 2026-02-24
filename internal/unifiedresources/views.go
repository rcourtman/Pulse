package unifiedresources

import (
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Metric helpers (nil-safe).
//
// Note: This package already has a different `metricPercent(*MetricValue) float64`
// helper used by unified AI adapters. These view helpers intentionally take a
// `*ResourceMetrics` + selector to keep accessors thin and consistent.
func viewMetricPercent(m *ResourceMetrics, pick func(*ResourceMetrics) *MetricValue) float64 {
	if m == nil {
		return 0
	}
	v := pick(m)
	if v == nil {
		return 0
	}
	return v.Percent
}

func viewMetricUsed(m *ResourceMetrics, pick func(*ResourceMetrics) *MetricValue) int64 {
	if m == nil {
		return 0
	}
	v := pick(m)
	if v == nil || v.Used == nil {
		return 0
	}
	return *v.Used
}

func viewMetricTotal(m *ResourceMetrics, pick func(*ResourceMetrics) *MetricValue) int64 {
	if m == nil {
		return 0
	}
	v := pick(m)
	if v == nil || v.Total == nil {
		return 0
	}
	return *v.Total
}

func viewMetricValue(m *ResourceMetrics, pick func(*ResourceMetrics) *MetricValue) float64 {
	if m == nil {
		return 0
	}
	v := pick(m)
	if v == nil {
		return 0
	}
	return v.Value
}

var (
	selectMetricsCPU    = func(m *ResourceMetrics) *MetricValue { return m.CPU }
	selectMetricsMemory = func(m *ResourceMetrics) *MetricValue { return m.Memory }
	selectMetricsDisk   = func(m *ResourceMetrics) *MetricValue { return m.Disk }
	selectMetricsNetIn  = func(m *ResourceMetrics) *MetricValue { return m.NetIn }
	selectMetricsNetOut = func(m *ResourceMetrics) *MetricValue { return m.NetOut }
)

// VMView wraps a VM resource (ResourceTypeVM).
type VMView struct{ r *Resource }

func NewVMView(r *Resource) VMView { return VMView{r: r} }

func (v VMView) String() string { return fmt.Sprintf("VMView(%s, %q)", v.ID(), v.Name()) }

func (v VMView) ID() string {
	if v.r == nil {
		return ""
	}
	return v.r.ID
}

func (v VMView) Name() string {
	if v.r == nil {
		return ""
	}
	return v.r.Name
}

func (v VMView) Status() ResourceStatus {
	if v.r == nil {
		return ""
	}
	return v.r.Status
}

func (v VMView) VMID() int {
	if v.r == nil || v.r.Proxmox == nil {
		return 0
	}
	return v.r.Proxmox.VMID
}

func (v VMView) Node() string {
	if v.r == nil || v.r.Proxmox == nil {
		return ""
	}
	return v.r.Proxmox.NodeName
}

func (v VMView) Instance() string {
	if v.r == nil || v.r.Proxmox == nil {
		return ""
	}
	return v.r.Proxmox.Instance
}

func (v VMView) Template() bool {
	if v.r == nil || v.r.Proxmox == nil {
		return false
	}
	return v.r.Proxmox.Template
}

func (v VMView) CPUs() int {
	if v.r == nil || v.r.Proxmox == nil {
		return 0
	}
	return v.r.Proxmox.CPUs
}

func (v VMView) Uptime() int64 {
	if v.r == nil || v.r.Proxmox == nil {
		return 0
	}
	return v.r.Proxmox.Uptime
}

func (v VMView) LastBackup() time.Time {
	if v.r == nil || v.r.Proxmox == nil {
		return time.Time{}
	}
	return v.r.Proxmox.LastBackup
}

func (v VMView) Disks() []DiskInfo {
	if v.r == nil || v.r.Proxmox == nil {
		return nil
	}
	return cloneDiskInfos(v.r.Proxmox.Disks)
}

func (v VMView) Tags() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Tags)
}

func (v VMView) LastSeen() time.Time {
	if v.r == nil {
		return time.Time{}
	}
	return v.r.LastSeen
}

func (v VMView) ParentID() string {
	if v.r == nil || v.r.ParentID == nil {
		return ""
	}
	return *v.r.ParentID
}

func (v VMView) ParentName() string {
	if v.r == nil {
		return ""
	}
	return v.r.ParentName
}

func (v VMView) CPUPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsCPU)
}

func (v VMView) MemoryUsed() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricUsed(v.r.Metrics, selectMetricsMemory)
}

func (v VMView) MemoryTotal() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricTotal(v.r.Metrics, selectMetricsMemory)
}

func (v VMView) MemoryPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsMemory)
}

func (v VMView) DiskUsed() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricUsed(v.r.Metrics, selectMetricsDisk)
}

func (v VMView) DiskTotal() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricTotal(v.r.Metrics, selectMetricsDisk)
}

func (v VMView) DiskPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsDisk)
}

func (v VMView) NetIn() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricValue(v.r.Metrics, selectMetricsNetIn)
}

func (v VMView) NetOut() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricValue(v.r.Metrics, selectMetricsNetOut)
}

func (v VMView) IPAddresses() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Identity.IPAddresses)
}

// ContainerView wraps a system container resource (ResourceTypeSystemContainer).
// It shares the same Proxmox payload shape and accessor set as VMView.
type ContainerView struct{ r *Resource }

func NewContainerView(r *Resource) ContainerView { return ContainerView{r: r} }

func (v ContainerView) String() string { return fmt.Sprintf("ContainerView(%s, %q)", v.ID(), v.Name()) }

func (v ContainerView) ID() string {
	if v.r == nil {
		return ""
	}
	return v.r.ID
}

func (v ContainerView) Name() string {
	if v.r == nil {
		return ""
	}
	return v.r.Name
}

func (v ContainerView) Status() ResourceStatus {
	if v.r == nil {
		return ""
	}
	return v.r.Status
}

func (v ContainerView) VMID() int {
	if v.r == nil || v.r.Proxmox == nil {
		return 0
	}
	return v.r.Proxmox.VMID
}

func (v ContainerView) Node() string {
	if v.r == nil || v.r.Proxmox == nil {
		return ""
	}
	return v.r.Proxmox.NodeName
}

func (v ContainerView) Instance() string {
	if v.r == nil || v.r.Proxmox == nil {
		return ""
	}
	return v.r.Proxmox.Instance
}

func (v ContainerView) Template() bool {
	if v.r == nil || v.r.Proxmox == nil {
		return false
	}
	return v.r.Proxmox.Template
}

func (v ContainerView) CPUs() int {
	if v.r == nil || v.r.Proxmox == nil {
		return 0
	}
	return v.r.Proxmox.CPUs
}

func (v ContainerView) Uptime() int64 {
	if v.r == nil || v.r.Proxmox == nil {
		return 0
	}
	return v.r.Proxmox.Uptime
}

func (v ContainerView) LastBackup() time.Time {
	if v.r == nil || v.r.Proxmox == nil {
		return time.Time{}
	}
	return v.r.Proxmox.LastBackup
}

func (v ContainerView) Disks() []DiskInfo {
	if v.r == nil || v.r.Proxmox == nil {
		return nil
	}
	return cloneDiskInfos(v.r.Proxmox.Disks)
}

func (v ContainerView) Tags() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Tags)
}

func (v ContainerView) LastSeen() time.Time {
	if v.r == nil {
		return time.Time{}
	}
	return v.r.LastSeen
}

func (v ContainerView) ParentID() string {
	if v.r == nil || v.r.ParentID == nil {
		return ""
	}
	return *v.r.ParentID
}

func (v ContainerView) ParentName() string {
	if v.r == nil {
		return ""
	}
	return v.r.ParentName
}

func (v ContainerView) CPUPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsCPU)
}

func (v ContainerView) MemoryUsed() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricUsed(v.r.Metrics, selectMetricsMemory)
}

func (v ContainerView) MemoryTotal() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricTotal(v.r.Metrics, selectMetricsMemory)
}

func (v ContainerView) MemoryPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsMemory)
}

func (v ContainerView) DiskUsed() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricUsed(v.r.Metrics, selectMetricsDisk)
}

func (v ContainerView) DiskTotal() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricTotal(v.r.Metrics, selectMetricsDisk)
}

func (v ContainerView) DiskPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsDisk)
}

func (v ContainerView) NetIn() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricValue(v.r.Metrics, selectMetricsNetIn)
}

func (v ContainerView) NetOut() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricValue(v.r.Metrics, selectMetricsNetOut)
}

func (v ContainerView) IPAddresses() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Identity.IPAddresses)
}

// NodeView wraps a host-type resource with Proxmox data.
type NodeView struct{ r *Resource }

func NewNodeView(r *Resource) NodeView { return NodeView{r: r} }

func (v NodeView) String() string { return fmt.Sprintf("NodeView(%s, %q)", v.ID(), v.Name()) }

func (v NodeView) ID() string {
	if v.r == nil {
		return ""
	}
	return v.r.ID
}

func (v NodeView) Name() string {
	if v.r == nil {
		return ""
	}
	return v.r.Name
}

func (v NodeView) Status() ResourceStatus {
	if v.r == nil {
		return ""
	}
	return v.r.Status
}

func (v NodeView) NodeName() string {
	if v.r == nil || v.r.Proxmox == nil {
		return ""
	}
	return v.r.Proxmox.NodeName
}

func (v NodeView) ClusterName() string {
	if v.r == nil || v.r.Proxmox == nil {
		return ""
	}
	return v.r.Proxmox.ClusterName
}

func (v NodeView) Instance() string {
	if v.r == nil || v.r.Proxmox == nil {
		return ""
	}
	return v.r.Proxmox.Instance
}

func (v NodeView) PVEVersion() string {
	if v.r == nil || v.r.Proxmox == nil {
		return ""
	}
	return v.r.Proxmox.PVEVersion
}

func (v NodeView) KernelVersion() string {
	if v.r == nil || v.r.Proxmox == nil {
		return ""
	}
	return v.r.Proxmox.KernelVersion
}

func (v NodeView) Uptime() int64 {
	if v.r == nil || v.r.Proxmox == nil {
		return 0
	}
	return v.r.Proxmox.Uptime
}

func (v NodeView) CPUs() int {
	if v.r == nil || v.r.Proxmox == nil {
		return 0
	}
	if v.r.Proxmox.CPUInfo != nil && v.r.Proxmox.CPUInfo.Cores > 0 && v.r.Proxmox.CPUInfo.Sockets > 0 {
		return v.r.Proxmox.CPUInfo.Cores * v.r.Proxmox.CPUInfo.Sockets
	}
	return v.r.Proxmox.CPUs
}

func (v NodeView) Temperature() float64 {
	if v.r == nil || v.r.Proxmox == nil || v.r.Proxmox.Temperature == nil {
		return 0
	}
	return *v.r.Proxmox.Temperature
}

func (v NodeView) HasTemperature() bool {
	return v.r != nil && v.r.Proxmox != nil && v.r.Proxmox.Temperature != nil
}

func (v NodeView) LoadAverage() []float64 {
	if v.r == nil || v.r.Proxmox == nil {
		return nil
	}
	return cloneFloat64Slice(v.r.Proxmox.LoadAverage)
}

func (v NodeView) PendingUpdates() int {
	if v.r == nil || v.r.Proxmox == nil {
		return 0
	}
	return v.r.Proxmox.PendingUpdates
}

func (v NodeView) CPUPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsCPU)
}

func (v NodeView) MemoryUsed() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricUsed(v.r.Metrics, selectMetricsMemory)
}

func (v NodeView) MemoryTotal() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricTotal(v.r.Metrics, selectMetricsMemory)
}

func (v NodeView) MemoryPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsMemory)
}

func (v NodeView) DiskUsed() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricUsed(v.r.Metrics, selectMetricsDisk)
}

func (v NodeView) DiskTotal() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricTotal(v.r.Metrics, selectMetricsDisk)
}

func (v NodeView) DiskPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsDisk)
}

func (v NodeView) Tags() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Tags)
}

func (v NodeView) LastSeen() time.Time {
	if v.r == nil {
		return time.Time{}
	}
	return v.r.LastSeen
}

func (v NodeView) LinkedHostAgentID() string {
	if v.r == nil || v.r.Proxmox == nil {
		return ""
	}
	return v.r.Proxmox.LinkedHostAgentID
}

// HostView wraps a host-type resource with Agent data.
type HostView struct{ r *Resource }

func NewHostView(r *Resource) HostView { return HostView{r: r} }

func (v HostView) String() string { return fmt.Sprintf("HostView(%s, %q)", v.ID(), v.Name()) }

func (v HostView) ID() string {
	if v.r == nil {
		return ""
	}
	return v.r.ID
}

func (v HostView) Name() string {
	if v.r == nil {
		return ""
	}
	return v.r.Name
}

func (v HostView) Hostname() string {
	if v.r == nil || v.r.Agent == nil {
		return ""
	}
	return v.r.Agent.Hostname
}

func (v HostView) Platform() string {
	if v.r == nil || v.r.Agent == nil {
		return ""
	}
	return v.r.Agent.Platform
}

func (v HostView) OSName() string {
	if v.r == nil || v.r.Agent == nil {
		return ""
	}
	return v.r.Agent.OSName
}

func (v HostView) OSVersion() string {
	if v.r == nil || v.r.Agent == nil {
		return ""
	}
	return v.r.Agent.OSVersion
}

func (v HostView) KernelVersion() string {
	if v.r == nil || v.r.Agent == nil {
		return ""
	}
	return v.r.Agent.KernelVersion
}

func (v HostView) Architecture() string {
	if v.r == nil || v.r.Agent == nil {
		return ""
	}
	return v.r.Agent.Architecture
}

func (v HostView) AgentVersion() string {
	if v.r == nil || v.r.Agent == nil {
		return ""
	}
	return v.r.Agent.AgentVersion
}

func (v HostView) AgentID() string {
	if v.r == nil || v.r.Agent == nil {
		return ""
	}
	return v.r.Agent.AgentID
}

func (v HostView) UptimeSeconds() int64 {
	if v.r == nil || v.r.Agent == nil {
		return 0
	}
	return v.r.Agent.UptimeSeconds
}

func (v HostView) Temperature() float64 {
	if v.r == nil || v.r.Agent == nil || v.r.Agent.Temperature == nil {
		return 0
	}
	return *v.r.Agent.Temperature
}

func (v HostView) HasTemperature() bool {
	return v.r != nil && v.r.Agent != nil && v.r.Agent.Temperature != nil
}

func (v HostView) NetworkInterfaces() []NetworkInterface {
	if v.r == nil || v.r.Agent == nil {
		return nil
	}
	return cloneNetworkInterfaces(v.r.Agent.NetworkInterfaces)
}

func (v HostView) Disks() []DiskInfo {
	if v.r == nil || v.r.Agent == nil {
		return nil
	}
	return cloneDiskInfos(v.r.Agent.Disks)
}

func (v HostView) LinkedNodeID() string {
	if v.r == nil || v.r.Agent == nil {
		return ""
	}
	return v.r.Agent.LinkedNodeID
}

func (v HostView) LinkedVMID() string {
	if v.r == nil || v.r.Agent == nil {
		return ""
	}
	return v.r.Agent.LinkedVMID
}

func (v HostView) LinkedContainerID() string {
	if v.r == nil || v.r.Agent == nil {
		return ""
	}
	return v.r.Agent.LinkedContainerID
}

func (v HostView) Status() ResourceStatus {
	if v.r == nil {
		return ""
	}
	return v.r.Status
}

func (v HostView) Tags() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Tags)
}

func (v HostView) LastSeen() time.Time {
	if v.r == nil {
		return time.Time{}
	}
	return v.r.LastSeen
}

func (v HostView) CPUPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsCPU)
}

func (v HostView) MemoryUsed() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricUsed(v.r.Metrics, selectMetricsMemory)
}

func (v HostView) MemoryTotal() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricTotal(v.r.Metrics, selectMetricsMemory)
}

func (v HostView) MemoryPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsMemory)
}

func (v HostView) DiskPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsDisk)
}

func (v HostView) Sensors() *HostSensorMeta {
	if v.r == nil || v.r.Agent == nil {
		return nil
	}
	return cloneHostSensorMeta(v.r.Agent.Sensors)
}

func (v HostView) RAID() []HostRAIDMeta {
	if v.r == nil || v.r.Agent == nil {
		return nil
	}
	return cloneHostRAIDMetaSlice(v.r.Agent.RAID)
}

func (v HostView) DiskIO() []HostDiskIOMeta {
	if v.r == nil || v.r.Agent == nil {
		return nil
	}
	return cloneHostDiskIOMetaSlice(v.r.Agent.DiskIO)
}

func (v HostView) Ceph() *HostCephMeta {
	if v.r == nil || v.r.Agent == nil {
		return nil
	}
	return cloneHostCephMeta(v.r.Agent.Ceph)
}

// DockerHostView wraps a host-type resource with Docker data.
type DockerHostView struct{ r *Resource }

func NewDockerHostView(r *Resource) DockerHostView { return DockerHostView{r: r} }

func (v DockerHostView) String() string {
	return fmt.Sprintf("DockerHostView(%s, %q)", v.ID(), v.Name())
}

func (v DockerHostView) ID() string {
	if v.r == nil {
		return ""
	}
	return v.r.ID
}

func (v DockerHostView) Name() string {
	if v.r == nil {
		return ""
	}
	return v.r.Name
}

func (v DockerHostView) HostSourceID() string {
	if v.r == nil || v.r.Docker == nil {
		return ""
	}
	return v.r.Docker.HostSourceID
}

func (v DockerHostView) Hostname() string {
	if v.r == nil || v.r.Docker == nil {
		return ""
	}
	return v.r.Docker.Hostname
}

func (v DockerHostView) AgentID() string {
	if v.r == nil || v.r.Docker == nil {
		return ""
	}
	return v.r.Docker.AgentID
}

func (v DockerHostView) DockerVersion() string {
	if v.r == nil || v.r.Docker == nil {
		return ""
	}
	return v.r.Docker.DockerVersion
}

func (v DockerHostView) RuntimeVersion() string {
	if v.r == nil || v.r.Docker == nil {
		return ""
	}
	return v.r.Docker.RuntimeVersion
}

func (v DockerHostView) OS() string {
	if v.r == nil || v.r.Docker == nil {
		return ""
	}
	return v.r.Docker.OS
}

func (v DockerHostView) KernelVersion() string {
	if v.r == nil || v.r.Docker == nil {
		return ""
	}
	return v.r.Docker.KernelVersion
}

func (v DockerHostView) Architecture() string {
	if v.r == nil || v.r.Docker == nil {
		return ""
	}
	return v.r.Docker.Architecture
}

func (v DockerHostView) AgentVersion() string {
	if v.r == nil || v.r.Docker == nil {
		return ""
	}
	return v.r.Docker.AgentVersion
}

func (v DockerHostView) UptimeSeconds() int64 {
	if v.r == nil || v.r.Docker == nil {
		return 0
	}
	return v.r.Docker.UptimeSeconds
}

func (v DockerHostView) Temperature() float64 {
	if v.r == nil || v.r.Docker == nil || v.r.Docker.Temperature == nil {
		return 0
	}
	return *v.r.Docker.Temperature
}

func (v DockerHostView) HasTemperature() bool {
	return v.r != nil && v.r.Docker != nil && v.r.Docker.Temperature != nil
}

func (v DockerHostView) Swarm() *DockerSwarmInfo {
	if v.r == nil || v.r.Docker == nil {
		return nil
	}
	return cloneDockerSwarmInfo(v.r.Docker.Swarm)
}

func (v DockerHostView) Services() []models.DockerService {
	if v.r == nil || v.r.Docker == nil {
		return nil
	}
	// Note: these are not deep-cloned since this package doesn't define clones for models.DockerService
	return append([]models.DockerService(nil), v.r.Docker.Services...)
}

func (v DockerHostView) Tasks() []models.DockerTask {
	if v.r == nil || v.r.Docker == nil {
		return nil
	}
	// Note: these are not deep-cloned
	return append([]models.DockerTask(nil), v.r.Docker.Tasks...)
}

func (v DockerHostView) NetworkInterfaces() []NetworkInterface {
	if v.r == nil || v.r.Docker == nil {
		return nil
	}
	return cloneNetworkInterfaces(v.r.Docker.NetworkInterfaces)
}

func (v DockerHostView) Disks() []DiskInfo {
	if v.r == nil || v.r.Docker == nil {
		return nil
	}
	return cloneDiskInfos(v.r.Docker.Disks)
}

func (v DockerHostView) Status() ResourceStatus {
	if v.r == nil {
		return ""
	}
	return v.r.Status
}

func (v DockerHostView) Tags() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Tags)
}

func (v DockerHostView) LastSeen() time.Time {
	if v.r == nil {
		return time.Time{}
	}
	return v.r.LastSeen
}

func (v DockerHostView) CPUPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsCPU)
}

func (v DockerHostView) MemoryPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsMemory)
}

func (v DockerHostView) ChildCount() int {
	if v.r == nil {
		return 0
	}
	return v.r.ChildCount
}

// StoragePoolView wraps a storage resource.
type StoragePoolView struct{ r *Resource }

func NewStoragePoolView(r *Resource) StoragePoolView { return StoragePoolView{r: r} }

func (v StoragePoolView) String() string {
	return fmt.Sprintf("StoragePoolView(%s, %q)", v.ID(), v.Name())
}

func (v StoragePoolView) ID() string {
	if v.r == nil {
		return ""
	}
	return v.r.ID
}

func (v StoragePoolView) Name() string {
	if v.r == nil {
		return ""
	}
	return v.r.Name
}

func (v StoragePoolView) Status() ResourceStatus {
	if v.r == nil {
		return ""
	}
	return v.r.Status
}

func (v StoragePoolView) Node() string {
	if v.r == nil || v.r.Proxmox == nil {
		return ""
	}
	return v.r.Proxmox.NodeName
}

func (v StoragePoolView) Instance() string {
	if v.r == nil || v.r.Proxmox == nil {
		return ""
	}
	return v.r.Proxmox.Instance
}

func (v StoragePoolView) StorageType() string {
	if v.r == nil || v.r.Storage == nil {
		return ""
	}
	return v.r.Storage.Type
}

func (v StoragePoolView) Content() string {
	if v.r == nil || v.r.Storage == nil {
		return ""
	}
	return v.r.Storage.Content
}

func (v StoragePoolView) ContentTypes() []string {
	if v.r == nil || v.r.Storage == nil {
		return nil
	}
	return cloneStringSlice(v.r.Storage.ContentTypes)
}

func (v StoragePoolView) Shared() bool {
	if v.r == nil || v.r.Storage == nil {
		return false
	}
	return v.r.Storage.Shared
}

func (v StoragePoolView) IsCeph() bool {
	if v.r == nil || v.r.Storage == nil {
		return false
	}
	return v.r.Storage.IsCeph
}

func (v StoragePoolView) IsZFS() bool {
	if v.r == nil || v.r.Storage == nil {
		return false
	}
	return v.r.Storage.IsZFS
}

func (v StoragePoolView) ZFSPoolState() string {
	if v.r == nil || v.r.Storage == nil {
		return ""
	}
	return v.r.Storage.ZFSPoolState
}

func (v StoragePoolView) ZFSReadErrors() int64 {
	if v.r == nil || v.r.Storage == nil {
		return 0
	}
	return v.r.Storage.ZFSReadErrors
}

func (v StoragePoolView) ZFSWriteErrors() int64 {
	if v.r == nil || v.r.Storage == nil {
		return 0
	}
	return v.r.Storage.ZFSWriteErrors
}

func (v StoragePoolView) ZFSChecksumErrors() int64 {
	if v.r == nil || v.r.Storage == nil {
		return 0
	}
	return v.r.Storage.ZFSChecksumErrors
}

func (v StoragePoolView) DiskUsed() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricUsed(v.r.Metrics, selectMetricsDisk)
}

func (v StoragePoolView) DiskTotal() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricTotal(v.r.Metrics, selectMetricsDisk)
}

func (v StoragePoolView) DiskPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsDisk)
}

func (v StoragePoolView) Tags() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Tags)
}

func (v StoragePoolView) LastSeen() time.Time {
	if v.r == nil {
		return time.Time{}
	}
	return v.r.LastSeen
}

func (v StoragePoolView) ParentID() string {
	if v.r == nil || v.r.ParentID == nil {
		return ""
	}
	return *v.r.ParentID
}

func (v StoragePoolView) ParentName() string {
	if v.r == nil {
		return ""
	}
	return v.r.ParentName
}

// PBSInstanceView wraps a PBS resource.
type PBSInstanceView struct{ r *Resource }

func NewPBSInstanceView(r *Resource) PBSInstanceView { return PBSInstanceView{r: r} }

func (v PBSInstanceView) String() string {
	return fmt.Sprintf("PBSInstanceView(%s, %q)", v.ID(), v.Name())
}

func (v PBSInstanceView) ID() string {
	if v.r == nil {
		return ""
	}
	return v.r.ID
}

func (v PBSInstanceView) Name() string {
	if v.r == nil {
		return ""
	}
	return v.r.Name
}

func (v PBSInstanceView) Status() ResourceStatus {
	if v.r == nil {
		return ""
	}
	return v.r.Status
}

func (v PBSInstanceView) Hostname() string {
	if v.r == nil || v.r.PBS == nil {
		return ""
	}
	return v.r.PBS.Hostname
}

func (v PBSInstanceView) Version() string {
	if v.r == nil || v.r.PBS == nil {
		return ""
	}
	return v.r.PBS.Version
}

func (v PBSInstanceView) UptimeSeconds() int64 {
	if v.r == nil || v.r.PBS == nil {
		return 0
	}
	return v.r.PBS.UptimeSeconds
}

func (v PBSInstanceView) DatastoreCount() int {
	if v.r == nil || v.r.PBS == nil {
		return 0
	}
	return v.r.PBS.DatastoreCount
}

func (v PBSInstanceView) Datastores() []PBSDatastoreMeta {
	if v.r == nil || v.r.PBS == nil {
		return nil
	}
	return clonePBSDatastoreMetaSlice(v.r.PBS.Datastores)
}

func (v PBSInstanceView) BackupJobCount() int {
	if v.r == nil || v.r.PBS == nil {
		return 0
	}
	return v.r.PBS.BackupJobCount
}

func (v PBSInstanceView) SyncJobCount() int {
	if v.r == nil || v.r.PBS == nil {
		return 0
	}
	return v.r.PBS.SyncJobCount
}

func (v PBSInstanceView) VerifyJobCount() int {
	if v.r == nil || v.r.PBS == nil {
		return 0
	}
	return v.r.PBS.VerifyJobCount
}

func (v PBSInstanceView) PruneJobCount() int {
	if v.r == nil || v.r.PBS == nil {
		return 0
	}
	return v.r.PBS.PruneJobCount
}

func (v PBSInstanceView) GarbageJobCount() int {
	if v.r == nil || v.r.PBS == nil {
		return 0
	}
	return v.r.PBS.GarbageJobCount
}

func (v PBSInstanceView) ConnectionHealth() string {
	if v.r == nil || v.r.PBS == nil {
		return ""
	}
	return v.r.PBS.ConnectionHealth
}

func (v PBSInstanceView) CPUPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsCPU)
}

func (v PBSInstanceView) MemoryPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsMemory)
}

func (v PBSInstanceView) DiskPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsDisk)
}

func (v PBSInstanceView) Tags() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Tags)
}

func (v PBSInstanceView) LastSeen() time.Time {
	if v.r == nil {
		return time.Time{}
	}
	return v.r.LastSeen
}

func (v PBSInstanceView) CustomURL() string {
	if v.r == nil {
		return ""
	}
	return v.r.CustomURL
}

// PMGInstanceView wraps a PMG resource.
type PMGInstanceView struct{ r *Resource }

func NewPMGInstanceView(r *Resource) PMGInstanceView { return PMGInstanceView{r: r} }

func (v PMGInstanceView) String() string {
	return fmt.Sprintf("PMGInstanceView(%s, %q)", v.ID(), v.Name())
}

func (v PMGInstanceView) ID() string {
	if v.r == nil {
		return ""
	}
	return v.r.ID
}

func (v PMGInstanceView) Name() string {
	if v.r == nil {
		return ""
	}
	return v.r.Name
}

func (v PMGInstanceView) InstanceID() string {
	if v.r == nil || v.r.PMG == nil {
		return ""
	}
	return v.r.PMG.InstanceID
}

func (v PMGInstanceView) Status() ResourceStatus {
	if v.r == nil {
		return ""
	}
	return v.r.Status
}

func (v PMGInstanceView) Hostname() string {
	if v.r == nil || v.r.PMG == nil {
		return ""
	}
	return v.r.PMG.Hostname
}

func (v PMGInstanceView) Version() string {
	if v.r == nil || v.r.PMG == nil {
		return ""
	}
	return v.r.PMG.Version
}

func (v PMGInstanceView) NodeCount() int {
	if v.r == nil || v.r.PMG == nil {
		return 0
	}
	return v.r.PMG.NodeCount
}

func (v PMGInstanceView) UptimeSeconds() int64 {
	if v.r == nil || v.r.PMG == nil {
		return 0
	}
	return v.r.PMG.UptimeSeconds
}

func (v PMGInstanceView) QueueActive() int {
	if v.r == nil || v.r.PMG == nil {
		return 0
	}
	return v.r.PMG.QueueActive
}

func (v PMGInstanceView) QueueDeferred() int {
	if v.r == nil || v.r.PMG == nil {
		return 0
	}
	return v.r.PMG.QueueDeferred
}

func (v PMGInstanceView) QueueTotal() int {
	if v.r == nil || v.r.PMG == nil {
		return 0
	}
	return v.r.PMG.QueueTotal
}

func (v PMGInstanceView) MailCountTotal() float64 {
	if v.r == nil || v.r.PMG == nil {
		return 0
	}
	return v.r.PMG.MailCountTotal
}

func (v PMGInstanceView) SpamIn() float64 {
	if v.r == nil || v.r.PMG == nil {
		return 0
	}
	return v.r.PMG.SpamIn
}

func (v PMGInstanceView) VirusIn() float64 {
	if v.r == nil || v.r.PMG == nil {
		return 0
	}
	return v.r.PMG.VirusIn
}

func (v PMGInstanceView) ConnectionHealth() string {
	if v.r == nil || v.r.PMG == nil {
		return ""
	}
	return v.r.PMG.ConnectionHealth
}

func (v PMGInstanceView) CPUPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsCPU)
}

func (v PMGInstanceView) MemoryPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsMemory)
}

func (v PMGInstanceView) DiskPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsDisk)
}

func (v PMGInstanceView) Tags() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Tags)
}

func (v PMGInstanceView) LastSeen() time.Time {
	if v.r == nil {
		return time.Time{}
	}
	return v.r.LastSeen
}

func (v PMGInstanceView) CustomURL() string {
	if v.r == nil {
		return ""
	}
	return v.r.CustomURL
}

func (v PMGInstanceView) Nodes() []PMGNodeMeta {
	if v.r == nil || v.r.PMG == nil {
		return nil
	}
	return clonePMGNodeMetaSlice(v.r.PMG.Nodes)
}

func (v PMGInstanceView) MailStats() *PMGMailStatsMeta {
	if v.r == nil || v.r.PMG == nil {
		return nil
	}
	return clonePMGMailStatsMeta(v.r.PMG.MailStats)
}

func (v PMGInstanceView) Quarantine() *PMGQuarantineMeta {
	if v.r == nil || v.r.PMG == nil {
		return nil
	}
	return clonePMGQuarantineMeta(v.r.PMG.Quarantine)
}

func (v PMGInstanceView) SpamDistribution() []PMGSpamBucketMeta {
	if v.r == nil || v.r.PMG == nil {
		return nil
	}
	return clonePMGSpamBucketMetaSlice(v.r.PMG.SpamDistribution)
}

// K8sClusterView wraps a Kubernetes cluster resource.
type K8sClusterView struct{ r *Resource }

func NewK8sClusterView(r *Resource) K8sClusterView { return K8sClusterView{r: r} }

func (v K8sClusterView) String() string {
	return fmt.Sprintf("K8sClusterView(%s, %q)", v.ID(), v.Name())
}

func (v K8sClusterView) ID() string {
	if v.r == nil {
		return ""
	}
	return v.r.ID
}

func (v K8sClusterView) Name() string {
	if v.r == nil {
		return ""
	}
	return v.r.Name
}

func (v K8sClusterView) Status() ResourceStatus {
	if v.r == nil {
		return ""
	}
	return v.r.Status
}

func (v K8sClusterView) ClusterID() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.ClusterID
}

func (v K8sClusterView) ClusterName() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.ClusterName
}

func (v K8sClusterView) SourceName() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.SourceName
}

func (v K8sClusterView) AgentID() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.AgentID
}

func (v K8sClusterView) Context() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.Context
}

func (v K8sClusterView) Server() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.Server
}

func (v K8sClusterView) Version() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.Version
}

func (v K8sClusterView) PendingUninstall() bool {
	if v.r == nil || v.r.Kubernetes == nil {
		return false
	}
	return v.r.Kubernetes.PendingUninstall
}

func (v K8sClusterView) MetricCapabilities() *K8sMetricCapabilities {
	if v.r == nil || v.r.Kubernetes == nil {
		return nil
	}
	return cloneKubernetesMetricCapabilities(v.r.Kubernetes.MetricCapabilities)
}

func (v K8sClusterView) CPUPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsCPU)
}

func (v K8sClusterView) MemoryPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsMemory)
}

func (v K8sClusterView) Tags() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Tags)
}

func (v K8sClusterView) LastSeen() time.Time {
	if v.r == nil {
		return time.Time{}
	}
	return v.r.LastSeen
}

func (v K8sClusterView) ChildCount() int {
	if v.r == nil {
		return 0
	}
	return v.r.ChildCount
}

// WorkloadView is a polymorphic view over VM + LXC resources.
type WorkloadView struct{ r *Resource }

func NewWorkloadView(r *Resource) WorkloadView { return WorkloadView{r: r} }

func (v WorkloadView) String() string { return fmt.Sprintf("WorkloadView(%s, %q)", v.ID(), v.Name()) }

func (v WorkloadView) ID() string {
	if v.r == nil {
		return ""
	}
	return v.r.ID
}

func (v WorkloadView) Name() string {
	if v.r == nil {
		return ""
	}
	return v.r.Name
}

func (v WorkloadView) Type() ResourceType {
	if v.r == nil {
		return ""
	}
	return v.r.Type
}

func (v WorkloadView) Status() ResourceStatus {
	if v.r == nil {
		return ""
	}
	return v.r.Status
}

func (v WorkloadView) VMID() int {
	if v.r == nil || v.r.Proxmox == nil {
		return 0
	}
	return v.r.Proxmox.VMID
}

func (v WorkloadView) Node() string {
	if v.r == nil || v.r.Proxmox == nil {
		return ""
	}
	return v.r.Proxmox.NodeName
}

func (v WorkloadView) Instance() string {
	if v.r == nil || v.r.Proxmox == nil {
		return ""
	}
	return v.r.Proxmox.Instance
}

func (v WorkloadView) CPUPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsCPU)
}

func (v WorkloadView) MemoryPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsMemory)
}

func (v WorkloadView) DiskPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsDisk)
}

func (v WorkloadView) Tags() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Tags)
}

func (v WorkloadView) LastSeen() time.Time {
	if v.r == nil {
		return time.Time{}
	}
	return v.r.LastSeen
}

func (v WorkloadView) IsVM() bool { return v.r != nil && v.r.Type == ResourceTypeVM }

func (v WorkloadView) IsContainer() bool {
	return v.r != nil && v.r.Type == ResourceTypeSystemContainer
}

// InfrastructureView is a polymorphic view over host resources.
type InfrastructureView struct{ r *Resource }

func NewInfrastructureView(r *Resource) InfrastructureView { return InfrastructureView{r: r} }

func (v InfrastructureView) String() string {
	return fmt.Sprintf("InfrastructureView(%s, %q)", v.ID(), v.Name())
}

func (v InfrastructureView) ID() string {
	if v.r == nil {
		return ""
	}
	return v.r.ID
}

func (v InfrastructureView) Name() string {
	if v.r == nil {
		return ""
	}
	return v.r.Name
}

func (v InfrastructureView) Status() ResourceStatus {
	if v.r == nil {
		return ""
	}
	return v.r.Status
}

func (v InfrastructureView) CPUPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsCPU)
}

func (v InfrastructureView) MemoryPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsMemory)
}

func (v InfrastructureView) DiskPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsDisk)
}

func (v InfrastructureView) Tags() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Tags)
}

func (v InfrastructureView) LastSeen() time.Time {
	if v.r == nil {
		return time.Time{}
	}
	return v.r.LastSeen
}

func (v InfrastructureView) HasProxmox() bool { return v.r != nil && v.r.Proxmox != nil }

func (v InfrastructureView) HasAgent() bool { return v.r != nil && v.r.Agent != nil }

func (v InfrastructureView) HasDocker() bool { return v.r != nil && v.r.Docker != nil }

func (v InfrastructureView) ChildCount() int {
	if v.r == nil {
		return 0
	}
	return v.r.ChildCount
}

// K8sNodeView wraps a Kubernetes node resource (ResourceTypeK8sNode).
type K8sNodeView struct{ r *Resource }

func NewK8sNodeView(r *Resource) K8sNodeView { return K8sNodeView{r: r} }

func (v K8sNodeView) String() string { return fmt.Sprintf("K8sNodeView(%s, %q)", v.ID(), v.Name()) }

func (v K8sNodeView) ID() string {
	if v.r == nil {
		return ""
	}
	return v.r.ID
}

func (v K8sNodeView) Name() string {
	if v.r == nil {
		return ""
	}
	return v.r.Name
}

func (v K8sNodeView) Status() ResourceStatus {
	if v.r == nil {
		return ""
	}
	return v.r.Status
}

func (v K8sNodeView) ClusterName() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.ClusterName
}

func (v K8sNodeView) NodeUID() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.NodeUID
}

func (v K8sNodeView) NodeName() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.NodeName
}

func (v K8sNodeView) Ready() bool {
	if v.r == nil || v.r.Kubernetes == nil {
		return false
	}
	return v.r.Kubernetes.Ready
}

func (v K8sNodeView) Unschedulable() bool {
	if v.r == nil || v.r.Kubernetes == nil {
		return false
	}
	return v.r.Kubernetes.Unschedulable
}

func (v K8sNodeView) Roles() []string {
	if v.r == nil || v.r.Kubernetes == nil {
		return nil
	}
	return cloneStringSlice(v.r.Kubernetes.Roles)
}

func (v K8sNodeView) KubeletVersion() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.KubeletVersion
}

func (v K8sNodeView) ContainerRuntimeVersion() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.ContainerRuntimeVersion
}

func (v K8sNodeView) OSImage() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.OSImage
}

func (v K8sNodeView) KernelVersion() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.KernelVersion
}

func (v K8sNodeView) Architecture() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.Architecture
}

func (v K8sNodeView) CapacityCPU() int64 {
	if v.r == nil || v.r.Kubernetes == nil {
		return 0
	}
	return v.r.Kubernetes.CapacityCPU
}

func (v K8sNodeView) CapacityMemoryBytes() int64 {
	if v.r == nil || v.r.Kubernetes == nil {
		return 0
	}
	return v.r.Kubernetes.CapacityMemoryBytes
}

func (v K8sNodeView) CapacityPods() int64 {
	if v.r == nil || v.r.Kubernetes == nil {
		return 0
	}
	return v.r.Kubernetes.CapacityPods
}

func (v K8sNodeView) AllocCPU() int64 {
	if v.r == nil || v.r.Kubernetes == nil {
		return 0
	}
	return v.r.Kubernetes.AllocCPU
}

func (v K8sNodeView) AllocMemoryBytes() int64 {
	if v.r == nil || v.r.Kubernetes == nil {
		return 0
	}
	return v.r.Kubernetes.AllocMemoryBytes
}

func (v K8sNodeView) AllocPods() int64 {
	if v.r == nil || v.r.Kubernetes == nil {
		return 0
	}
	return v.r.Kubernetes.AllocPods
}

func (v K8sNodeView) CPUPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsCPU)
}

func (v K8sNodeView) MemoryPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsMemory)
}

func (v K8sNodeView) Tags() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Tags)
}

func (v K8sNodeView) LastSeen() time.Time {
	if v.r == nil {
		return time.Time{}
	}
	return v.r.LastSeen
}

func (v K8sNodeView) ParentID() string {
	if v.r == nil || v.r.ParentID == nil {
		return ""
	}
	return *v.r.ParentID
}

// PodView wraps a Pod resource (ResourceTypePod).
type PodView struct{ r *Resource }

func NewPodView(r *Resource) PodView { return PodView{r: r} }

func (v PodView) String() string { return fmt.Sprintf("PodView(%s, %q)", v.ID(), v.Name()) }

func (v PodView) ID() string {
	if v.r == nil {
		return ""
	}
	return v.r.ID
}

func (v PodView) Name() string {
	if v.r == nil {
		return ""
	}
	return v.r.Name
}

func (v PodView) Status() ResourceStatus {
	if v.r == nil {
		return ""
	}
	return v.r.Status
}

func (v PodView) ClusterName() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.ClusterName
}

func (v PodView) Namespace() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.Namespace
}

func (v PodView) PodUID() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.PodUID
}

func (v PodView) PodPhase() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.PodPhase
}

func (v PodView) Restarts() int {
	if v.r == nil || v.r.Kubernetes == nil {
		return 0
	}
	return v.r.Kubernetes.Restarts
}

func (v PodView) OwnerKind() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.OwnerKind
}

func (v PodView) OwnerName() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.OwnerName
}

func (v PodView) Image() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.Image
}

func (v PodView) Labels() map[string]string {
	if v.r == nil || v.r.Kubernetes == nil {
		return nil
	}
	return cloneStringMap(v.r.Kubernetes.Labels)
}

func (v PodView) CPUPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsCPU)
}

func (v PodView) MemoryPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsMemory)
}

func (v PodView) Tags() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Tags)
}

func (v PodView) LastSeen() time.Time {
	if v.r == nil {
		return time.Time{}
	}
	return v.r.LastSeen
}

func (v PodView) ParentID() string {
	if v.r == nil || v.r.ParentID == nil {
		return ""
	}
	return *v.r.ParentID
}

// K8sDeploymentView wraps a Kubernetes deployment resource (ResourceTypeK8sDeployment).
type K8sDeploymentView struct{ r *Resource }

func NewK8sDeploymentView(r *Resource) K8sDeploymentView { return K8sDeploymentView{r: r} }

func (v K8sDeploymentView) String() string {
	return fmt.Sprintf("K8sDeploymentView(%s, %q)", v.ID(), v.Name())
}

func (v K8sDeploymentView) ID() string {
	if v.r == nil {
		return ""
	}
	return v.r.ID
}

func (v K8sDeploymentView) Name() string {
	if v.r == nil {
		return ""
	}
	return v.r.Name
}

func (v K8sDeploymentView) Status() ResourceStatus {
	if v.r == nil {
		return ""
	}
	return v.r.Status
}

func (v K8sDeploymentView) ClusterName() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.ClusterName
}

func (v K8sDeploymentView) Namespace() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.Namespace
}

func (v K8sDeploymentView) DeploymentUID() string {
	if v.r == nil || v.r.Kubernetes == nil {
		return ""
	}
	return v.r.Kubernetes.DeploymentUID
}

func (v K8sDeploymentView) DesiredReplicas() int32 {
	if v.r == nil || v.r.Kubernetes == nil {
		return 0
	}
	return v.r.Kubernetes.DesiredReplicas
}

func (v K8sDeploymentView) UpdatedReplicas() int32 {
	if v.r == nil || v.r.Kubernetes == nil {
		return 0
	}
	return v.r.Kubernetes.UpdatedReplicas
}

func (v K8sDeploymentView) ReadyReplicas() int32 {
	if v.r == nil || v.r.Kubernetes == nil {
		return 0
	}
	return v.r.Kubernetes.ReadyReplicas
}

func (v K8sDeploymentView) AvailableReplicas() int32 {
	if v.r == nil || v.r.Kubernetes == nil {
		return 0
	}
	return v.r.Kubernetes.AvailableReplicas
}

func (v K8sDeploymentView) Labels() map[string]string {
	if v.r == nil || v.r.Kubernetes == nil {
		return nil
	}
	return cloneStringMap(v.r.Kubernetes.Labels)
}

func (v K8sDeploymentView) Tags() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Tags)
}

func (v K8sDeploymentView) LastSeen() time.Time {
	if v.r == nil {
		return time.Time{}
	}
	return v.r.LastSeen
}

func (v K8sDeploymentView) ParentID() string {
	if v.r == nil || v.r.ParentID == nil {
		return ""
	}
	return *v.r.ParentID
}

// DockerContainerView wraps a Docker/OCI app container resource (ResourceTypeAppContainer).
type DockerContainerView struct{ r *Resource }

func NewDockerContainerView(r *Resource) DockerContainerView { return DockerContainerView{r: r} }

func (v DockerContainerView) String() string {
	return fmt.Sprintf("DockerContainerView(%s, %q)", v.ID(), v.Name())
}

func (v DockerContainerView) ID() string {
	if v.r == nil {
		return ""
	}
	return v.r.ID
}

func (v DockerContainerView) Name() string {
	if v.r == nil {
		return ""
	}
	return v.r.Name
}

func (v DockerContainerView) Status() ResourceStatus {
	if v.r == nil {
		return ""
	}
	return v.r.Status
}

func (v DockerContainerView) ContainerID() string {
	if v.r == nil || v.r.Docker == nil {
		return ""
	}
	return v.r.Docker.ContainerID
}

func (v DockerContainerView) Image() string {
	if v.r == nil || v.r.Docker == nil {
		return ""
	}
	return v.r.Docker.Image
}

func (v DockerContainerView) ContainerState() string {
	if v.r == nil || v.r.Docker == nil {
		return ""
	}
	return v.r.Docker.ContainerState
}

func (v DockerContainerView) Health() string {
	if v.r == nil || v.r.Docker == nil {
		return ""
	}
	return v.r.Docker.Health
}

func (v DockerContainerView) RestartCount() int {
	if v.r == nil || v.r.Docker == nil {
		return 0
	}
	return v.r.Docker.RestartCount
}

func (v DockerContainerView) ExitCode() int {
	if v.r == nil || v.r.Docker == nil {
		return 0
	}
	return v.r.Docker.ExitCode
}

func (v DockerContainerView) CPUPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsCPU)
}

func (v DockerContainerView) MemoryUsed() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricUsed(v.r.Metrics, selectMetricsMemory)
}

func (v DockerContainerView) MemoryTotal() int64 {
	if v.r == nil {
		return 0
	}
	return viewMetricTotal(v.r.Metrics, selectMetricsMemory)
}

func (v DockerContainerView) MemoryPercent() float64 {
	if v.r == nil {
		return 0
	}
	return viewMetricPercent(v.r.Metrics, selectMetricsMemory)
}

func (v DockerContainerView) UptimeSeconds() int64 {
	if v.r == nil || v.r.Docker == nil {
		return 0
	}
	return v.r.Docker.UptimeSeconds
}

func (v DockerContainerView) Ports() []DockerPortMeta {
	if v.r == nil || v.r.Docker == nil {
		return nil
	}
	return cloneDockerPortMetaSlice(v.r.Docker.Ports)
}

func (v DockerContainerView) Labels() map[string]string {
	if v.r == nil || v.r.Docker == nil {
		return nil
	}
	return cloneStringMap(v.r.Docker.Labels)
}

func (v DockerContainerView) Networks() []DockerNetworkMeta {
	if v.r == nil || v.r.Docker == nil {
		return nil
	}
	return cloneDockerNetworkMetaSlice(v.r.Docker.Networks)
}

func (v DockerContainerView) Mounts() []DockerMountMeta {
	if v.r == nil || v.r.Docker == nil {
		return nil
	}
	return cloneDockerMountMetaSlice(v.r.Docker.Mounts)
}

func (v DockerContainerView) UpdateStatus() *DockerUpdateStatusMeta {
	if v.r == nil || v.r.Docker == nil {
		return nil
	}
	return cloneDockerUpdateStatusMeta(v.r.Docker.UpdateStatus)
}

func (v DockerContainerView) ParentID() string {
	if v.r == nil || v.r.ParentID == nil {
		return ""
	}
	return *v.r.ParentID
}

func (v DockerContainerView) Tags() []string {
	if v.r == nil {
		return nil
	}
	return cloneStringSlice(v.r.Tags)
}

func (v DockerContainerView) LastSeen() time.Time {
	if v.r == nil {
		return time.Time{}
	}
	return v.r.LastSeen
}
