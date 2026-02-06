package unifiedresources

import (
	"net/url"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func resourceFromProxmoxNode(node models.Node) (Resource, ResourceIdentity) {
	name := node.Name
	if node.DisplayName != "" {
		name = node.DisplayName
	}

	identity := ResourceIdentity{
		Hostnames:   uniqueStrings([]string{node.Name, extractHostname(node.Host)}),
		ClusterName: node.ClusterName,
	}

	if node.ClusterName != "" {
		identity.Hostnames = uniqueStrings(append(identity.Hostnames, node.ClusterName+":"+node.Name))
	}

	proxmox := &ProxmoxData{
		NodeName:          node.Name,
		ClusterName:       node.ClusterName,
		PVEVersion:        node.PVEVersion,
		KernelVersion:     node.KernelVersion,
		Uptime:            node.Uptime,
		CPUInfo:           &CPUInfo{Model: node.CPUInfo.Model, Cores: node.CPUInfo.Cores, Sockets: node.CPUInfo.Sockets},
		LinkedHostAgentID: node.LinkedHostAgentID,
	}

	metrics := metricsFromProxmoxNode(node)

	resource := Resource{
		Type:      ResourceTypeHost,
		Name:      name,
		Status:    statusFromNode(node.Status),
		LastSeen:  node.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metrics,
		Proxmox:   proxmox,
		Tags:      nil,
	}

	return resource, identity
}

func resourceFromHost(host models.Host) (Resource, ResourceIdentity) {
	name := host.Hostname
	if host.DisplayName != "" {
		name = host.DisplayName
	}

	ips, macs := collectInterfaceIDs(host.NetworkInterfaces)
	if host.ReportIP != "" {
		ips = append(ips, host.ReportIP)
	}

	identity := ResourceIdentity{
		MachineID:    strings.TrimSpace(host.MachineID),
		Hostnames:    uniqueStrings([]string{host.Hostname}),
		IPAddresses:  uniqueStrings(ips),
		MACAddresses: uniqueStrings(macs),
	}

	agent := &AgentData{
		AgentID:           host.ID,
		AgentVersion:      host.AgentVersion,
		Hostname:          host.Hostname,
		Platform:          host.Platform,
		OSName:            host.OSName,
		OSVersion:         host.OSVersion,
		KernelVersion:     host.KernelVersion,
		Architecture:      host.Architecture,
		UptimeSeconds:     host.UptimeSeconds,
		Temperature:       maxCPUTemp(host.Sensors),
		NetworkInterfaces: convertInterfaces(host.NetworkInterfaces),
		Disks:             convertDisks(host.Disks),
		LinkedNodeID:      host.LinkedNodeID,
		LinkedVMID:        host.LinkedVMID,
		LinkedContainerID: host.LinkedContainerID,
	}

	metrics := metricsFromHost(host)

	resource := Resource{
		Type:      ResourceTypeHost,
		Name:      name,
		Status:    statusFromHost(host.Status),
		LastSeen:  host.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metrics,
		Agent:     agent,
		Tags:      host.Tags,
	}

	return resource, identity
}

func resourceFromDockerHost(host models.DockerHost) (Resource, ResourceIdentity) {
	name := host.Hostname
	if host.CustomDisplayName != "" {
		name = host.CustomDisplayName
	} else if host.DisplayName != "" {
		name = host.DisplayName
	}

	ips, macs := collectInterfaceIDs(host.NetworkInterfaces)

	identity := ResourceIdentity{
		MachineID:    host.MachineID,
		Hostnames:    uniqueStrings([]string{host.Hostname}),
		IPAddresses:  uniqueStrings(ips),
		MACAddresses: uniqueStrings(macs),
	}

	docker := &DockerData{
		Hostname:          host.Hostname,
		Runtime:           host.Runtime,
		RuntimeVersion:    host.RuntimeVersion,
		DockerVersion:     host.DockerVersion,
		OS:                host.OS,
		KernelVersion:     host.KernelVersion,
		Architecture:      host.Architecture,
		AgentVersion:      host.AgentVersion,
		Swarm:             convertSwarm(host.Swarm),
		NetworkInterfaces: convertInterfaces(host.NetworkInterfaces),
		Disks:             convertDisks(host.Disks),
	}

	metrics := metricsFromDockerHost(host)

	resource := Resource{
		Type:      ResourceTypeHost,
		Name:      name,
		Status:    statusFromHost(host.Status),
		LastSeen:  host.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metrics,
		Docker:    docker,
		Tags:      nil,
	}

	return resource, identity
}

func resourceFromVM(vm models.VM) (Resource, ResourceIdentity) {
	metrics := metricsFromVM(vm)
	proxmox := &ProxmoxData{
		NodeName:   vm.Node,
		Instance:   vm.Instance,
		VMID:       vm.VMID,
		CPUs:       vm.CPUs,
		Uptime:     vm.Uptime,
		Template:   vm.Template,
		LastBackup: vm.LastBackup,
	}
	resource := Resource{
		Type:      ResourceTypeVM,
		Name:      vm.Name,
		Status:    statusFromGuest(vm.Status),
		LastSeen:  vm.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metrics,
		Proxmox:   proxmox,
		Tags:      vm.Tags,
	}
	identity := ResourceIdentity{
		Hostnames:   uniqueStrings([]string{vm.Name}),
		IPAddresses: uniqueStrings(vm.IPAddresses),
	}
	return resource, identity
}

func resourceFromContainer(ct models.Container) (Resource, ResourceIdentity) {
	metrics := metricsFromContainer(ct)
	proxmox := &ProxmoxData{
		NodeName:   ct.Node,
		Instance:   ct.Instance,
		VMID:       ct.VMID,
		CPUs:       ct.CPUs,
		Uptime:     ct.Uptime,
		Template:   ct.Template,
		LastBackup: ct.LastBackup,
	}
	resource := Resource{
		Type:      ResourceTypeLXC,
		Name:      ct.Name,
		Status:    statusFromGuest(ct.Status),
		LastSeen:  ct.LastSeen,
		UpdatedAt: time.Now().UTC(),
		Metrics:   metrics,
		Proxmox:   proxmox,
		Tags:      ct.Tags,
	}
	identity := ResourceIdentity{
		Hostnames:   uniqueStrings([]string{ct.Name}),
		IPAddresses: uniqueStrings(ct.IPAddresses),
	}
	return resource, identity
}

func resourceFromDockerContainer(ct models.DockerContainer) (Resource, ResourceIdentity) {
	metrics := metricsFromDockerContainer(ct)
	resource := Resource{
		Type:      ResourceTypeContainer,
		Name:      ct.Name,
		Status:    statusFromDockerState(ct.State),
		LastSeen:  time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Metrics:   metrics,
		Docker:    &DockerData{Image: ct.Image, UptimeSeconds: ct.UptimeSeconds},
	}
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{ct.Name}),
	}
	return resource, identity
}

func convertInterfaces(interfaces []models.HostNetworkInterface) []NetworkInterface {
	out := make([]NetworkInterface, 0, len(interfaces))
	for _, iface := range interfaces {
		out = append(out, NetworkInterface{
			Name:      iface.Name,
			MAC:       iface.MAC,
			Addresses: iface.Addresses,
			SpeedMbps: iface.SpeedMbps,
		})
	}
	return out
}

func convertDisks(disks []models.Disk) []DiskInfo {
	out := make([]DiskInfo, 0, len(disks))
	for _, disk := range disks {
		out = append(out, DiskInfo{
			Device:     disk.Device,
			Mountpoint: disk.Mountpoint,
			Filesystem: disk.Type,
			Total:      disk.Total,
			Used:       disk.Used,
			Free:       disk.Free,
		})
	}
	return out
}

func convertSwarm(info *models.DockerSwarmInfo) *DockerSwarmInfo {
	if info == nil {
		return nil
	}
	return &DockerSwarmInfo{
		NodeID:           info.NodeID,
		NodeRole:         info.NodeRole,
		LocalState:       info.LocalState,
		ControlAvailable: info.ControlAvailable,
		ClusterID:        info.ClusterID,
		ClusterName:      info.ClusterName,
		Scope:            info.Scope,
		Error:            info.Error,
	}
}

func collectInterfaceIDs(interfaces []models.HostNetworkInterface) ([]string, []string) {
	var ips []string
	var macs []string
	for _, iface := range interfaces {
		if iface.MAC != "" {
			macs = append(macs, iface.MAC)
		}
		for _, addr := range iface.Addresses {
			ip := addr
			if strings.Contains(ip, "/") {
				ip = strings.Split(ip, "/")[0]
			}
			ips = append(ips, ip)
		}
	}
	return ips, macs
}

func extractHostname(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err == nil && parsed.Host != "" {
		host := parsed.Host
		if strings.Contains(host, ":") {
			host = strings.Split(host, ":")[0]
		}
		return host
	}

	if strings.Contains(raw, "/") {
		raw = strings.Split(raw, "/")[0]
	}
	if strings.Contains(raw, ":") {
		raw = strings.Split(raw, ":")[0]
	}
	return raw
}

// maxCPUTemp returns the highest CPU temperature from host sensor readings.
// It looks for cpu_package first, then falls back to max of any cpu_core_* key.
func maxCPUTemp(sensors models.HostSensorSummary) *float64 {
	temps := sensors.TemperatureCelsius
	if len(temps) == 0 {
		return nil
	}
	// Prefer cpu_package if available.
	if v, ok := temps["cpu_package"]; ok {
		return &v
	}
	// Fall back to max of any cpu-related key.
	var best float64
	found := false
	for k, v := range temps {
		if strings.HasPrefix(k, "cpu") {
			if !found || v > best {
				best = v
				found = true
			}
		}
	}
	if found {
		return &best
	}
	return nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
