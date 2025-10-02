package mock

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type MockConfig struct {
	NodeCount      int
	VMsPerNode     int
	LXCsPerNode    int
	RandomMetrics  bool
	HighLoadNodes  []string // Specific nodes to simulate high load
	StoppedPercent float64  // Percentage of guests that should be stopped
}

var DefaultConfig = MockConfig{
	NodeCount:      7, // Test the 5-9 node range by default
	VMsPerNode:     5,
	LXCsPerNode:    8,
	RandomMetrics:  true,
	StoppedPercent: 0.2,
}

var appNames = []string{
	"jellyfin", "plex", "nextcloud", "pihole", "homeassistant",
	"gitlab", "postgres", "mysql", "redis", "nginx",
	"traefik", "portainer", "grafana", "prometheus", "influxdb",
	"sonarr", "radarr", "transmission", "deluge", "sabnzbd",
	"bitwarden", "vaultwarden", "wireguard", "openvpn", "cloudflare",
	"minecraft", "valheim", "terraria", "factorio", "csgo",
	"webserver", "database", "cache", "loadbalancer", "firewall",
	"docker", "kubernetes", "rancher", "jenkins", "gitea",
	"syncthing", "seafile", "owncloud", "minio", "sftp",
}

// Common tags used for VMs and containers
var commonTags = []string{
	"production", "staging", "development", "testing",
	"web", "database", "cache", "queue", "storage",
	"frontend", "backend", "api", "microservice",
	"docker", "kubernetes", "monitoring", "logging",
	"backup", "critical", "important", "experimental",
	"media", "gaming", "home", "automation",
	"public", "private", "dmz", "internal",
	"linux", "windows", "debian", "ubuntu", "alpine",
	"managed", "unmanaged", "legacy", "deprecated",
	"team-a", "team-b", "customer-1", "project-x",
}

var vmMountpoints = []string{
	"/",
	"/var",
	"/home",
	"/srv",
	"/opt",
	"/data",
	"/backup",
	"/logs",
	"/mnt/data",
	"/mnt/backup",
}

var vmFilesystemTypes = []string{"ext4", "xfs", "btrfs", "zfs"}
var vmDevices = []string{"vda", "vdb", "vdc", "sda", "sdb", "sdc", "nvme0n1", "nvme1n1"}

func generateVirtualDisks() ([]models.Disk, models.Disk) {
	diskCount := 1 + rand.Intn(3)
	disks := make([]models.Disk, 0, diskCount)
	usedMounts := make(map[string]struct{})
	var total int64
	var used int64

	for i := 0; i < diskCount; i++ {
		mount := "/"
		if i == 0 {
			usedMounts[mount] = struct{}{}
		} else {
			candidate := vmMountpoints[rand.Intn(len(vmMountpoints))]
			for _, taken := usedMounts[candidate]; taken && len(usedMounts) < len(vmMountpoints); _, taken = usedMounts[candidate] {
				candidate = vmMountpoints[rand.Intn(len(vmMountpoints))]
			}
			mount = candidate
			usedMounts[mount] = struct{}{}
		}

		sizeGiB := 40 + rand.Intn(360) // 40 - 400 GiB
		totalBytes := int64(sizeGiB) * 1024 * 1024 * 1024
		usage := 0.25 + rand.Float64()*0.6
		usedBytes := int64(float64(totalBytes) * usage)

		device := vmDevices[i%len(vmDevices)]
		fsType := vmFilesystemTypes[rand.Intn(len(vmFilesystemTypes))]

		disks = append(disks, models.Disk{
			Total:      totalBytes,
			Used:       usedBytes,
			Free:       totalBytes - usedBytes,
			Usage:      usage * 100,
			Mountpoint: mount,
			Type:       fsType,
			Device:     fmt.Sprintf("/dev/%s", device),
		})

		total += totalBytes
		used += usedBytes
	}

	aggregated := models.Disk{
		Total: total,
		Used:  used,
		Free:  total - used,
	}
	if total > 0 {
		aggregated.Usage = float64(used) / float64(total) * 100
	}

	return disks, aggregated
}

func GenerateMockData(config MockConfig) models.StateSnapshot {
	rand.Seed(time.Now().UnixNano())

	data := models.StateSnapshot{
		Nodes:            generateNodes(config),
		VMs:              []models.VM{},
		Containers:       []models.Container{},
		PhysicalDisks:    []models.PhysicalDisk{},
		LastUpdate:       time.Now(),
		ConnectionHealth: make(map[string]bool),
		Stats:            models.Stats{},
		ActiveAlerts:     []models.Alert{},
	}

	// Generate physical disks for each node
	for _, node := range data.Nodes {
		data.PhysicalDisks = append(data.PhysicalDisks, generateDisksForNode(node)...)
	}

	// Generate VMs and containers for each node
	vmidCounter := 100
	for nodeIdx, node := range data.Nodes {
		// Determine node specialty for more realistic distribution
		nodeRole := "mixed"
		if nodeIdx > 0 {
			roleRand := rand.Float64()
			if roleRand < 0.3 {
				nodeRole = "vm-heavy" // 30% chance of being VM-focused
			} else if roleRand < 0.5 {
				nodeRole = "container-heavy" // 20% chance of being container-focused
			} else if roleRand < 0.6 {
				nodeRole = "light" // 10% chance of having few guests
			}
			// 40% remain mixed
		}

		// Calculate VM count based on node role
		vmCount := config.VMsPerNode
		lxcCount := config.LXCsPerNode

		switch nodeRole {
		case "vm-heavy":
			vmCount = config.VMsPerNode + rand.Intn(config.VMsPerNode)        // 100-200% of base
			lxcCount = config.LXCsPerNode/2 + rand.Intn(config.LXCsPerNode/2) // 50-75% of base
		case "container-heavy":
			vmCount = rand.Intn(config.VMsPerNode/2 + 1)                    // 0-50% of base
			lxcCount = config.LXCsPerNode*2 + rand.Intn(config.LXCsPerNode) // 200-300% of base
		case "light":
			vmCount = rand.Intn(config.VMsPerNode/2 + 1)   // 0-50% of base
			lxcCount = rand.Intn(config.LXCsPerNode/2 + 1) // 0-50% of base
		default: // mixed
			// Add some variation
			vmCount = config.VMsPerNode + rand.Intn(5) - 2   // +/- 2
			lxcCount = config.LXCsPerNode + rand.Intn(7) - 3 // +/- 3
		}

		// Ensure at least some activity on most nodes
		if nodeIdx < 3 && vmCount == 0 && lxcCount == 0 {
			if rand.Float64() < 0.5 {
				vmCount = 1 + rand.Intn(2)
			} else {
				lxcCount = 2 + rand.Intn(3)
			}
		}

		// Generate VMs
		for i := 0; i < vmCount; i++ {
			vm := generateVM(node.Name, node.Instance, vmidCounter, config)
			data.VMs = append(data.VMs, vm)
			vmidCounter++
		}

		// Generate containers
		for i := 0; i < lxcCount; i++ {
			lxc := generateContainer(node.Name, node.Instance, vmidCounter, config)
			data.Containers = append(data.Containers, lxc)
			vmidCounter++
		}

		// Set connection health
		data.ConnectionHealth[fmt.Sprintf("pve-%s", node.Name)] = true
	}

	// Generate storage for each node
	data.Storage = generateStorage(data.Nodes)

	// Generate PBS instances and backups
	data.PBSInstances = generatePBSInstances()
	data.PBSBackups = generatePBSBackups(data.VMs, data.Containers)

	// Set PBS connection health
	for _, pbs := range data.PBSInstances {
		data.ConnectionHealth[fmt.Sprintf("pbs-%s", pbs.Name)] = true
	}

	// Generate backups for VMs and containers
	data.PVEBackups = models.PVEBackups{
		BackupTasks:    []models.BackupTask{},
		StorageBackups: generateBackups(data.VMs, data.Containers),
		GuestSnapshots: generateSnapshots(data.VMs, data.Containers),
	}

	// Calculate stats
	data.Stats.StartTime = time.Now()
	data.Stats.Uptime = 0
	data.Stats.Version = "v4.9.0-mock"

	return data
}

func generateNodes(config MockConfig) []models.Node {
	nodes := make([]models.Node, 0, config.NodeCount)

	// First 5 nodes are part of the cluster
	clusterNodeCount := 5
	if config.NodeCount < 5 {
		clusterNodeCount = config.NodeCount
	}

	// Generate clustered nodes
	for i := 0; i < clusterNodeCount; i++ {
		nodeName := fmt.Sprintf("pve%d", i+1)
		isHighLoad := false
		for _, n := range config.HighLoadNodes {
			if n == nodeName {
				isHighLoad = true
				break
			}
		}

		node := generateNode(nodeName, isHighLoad, config)
		node.Instance = "mock-cluster" // Part of cluster
		node.IsClusterMember = true
		node.ClusterName = "mock-cluster"
		// ID format matches real system: instance-nodename
		node.ID = fmt.Sprintf("%s-%s", node.Instance, nodeName)

		// Make pve3 offline to test offline node handling
		if nodeName == "pve3" {
			node.Status = "offline"
			node.CPU = 0
			node.Memory.Used = 0
			node.Memory.Usage = 0
			node.ConnectionHealth = "offline"
		} else {
			// For cluster nodes, since one is offline, the cluster is degraded
			node.ConnectionHealth = "degraded"
		}

		nodes = append(nodes, node)
	}

	// Generate standalone nodes (if we have more than 5 nodes)
	for i := clusterNodeCount; i < config.NodeCount; i++ {
		nodeName := fmt.Sprintf("standalone%d", i-clusterNodeCount+1)
		isHighLoad := false
		for _, n := range config.HighLoadNodes {
			if n == nodeName {
				isHighLoad = true
				break
			}
		}

		node := generateNode(nodeName, isHighLoad, config)
		node.Instance = nodeName // Standalone - instance matches name
		node.IsClusterMember = false
		node.ClusterName = ""             // Empty for standalone
		node.ConnectionHealth = "healthy" // Standalone nodes are healthy if online
		// ID format matches real system: instance-nodename (same when standalone)
		node.ID = fmt.Sprintf("%s-%s", node.Instance, nodeName)
		nodes = append(nodes, node)
	}

	return nodes
}

func generateNode(name string, highLoad bool, config MockConfig) models.Node {
	baseLoad := 0.15
	if highLoad {
		baseLoad = 0.75
	}

	cpu := baseLoad + rand.Float64()*0.2
	if !config.RandomMetrics {
		cpu = baseLoad
	}

	// Memory in GB
	totalMem := int64(32 + rand.Intn(96)) // 32-128 GB
	usedMem := int64(float64(totalMem) * (baseLoad + rand.Float64()*0.3))

	// Disk in GB
	totalDisk := int64(500 + rand.Intn(2000)) // 500-2500 GB
	usedDisk := int64(float64(totalDisk) * (0.3 + rand.Float64()*0.4))

	// Generate CPU info
	coreCounts := []int{4, 8, 12, 16, 24, 32, 48, 64}
	cores := coreCounts[rand.Intn(len(coreCounts))]

	// Generate realistic version information
	pveVersions := []string{"8.2.4", "8.2.2", "8.1.10", "8.0.12", "7.4-18"}
	kernelVersions := []string{
		"6.8.12-1-pve",
		"6.8.8-2-pve",
		"6.5.13-5-pve",
		"6.2.16-20-pve",
		"5.15.143-1-pve",
	}

	// Generate temperature data
	temp := generateNodeTemperature(cores)

	return models.Node{
		Name:          name,
		Instance:      "", // Set by generateNodes based on cluster/standalone
		Type:          "pve",
		Status:        "online",
		Uptime:        int64(86400 * (1 + rand.Intn(30))), // 1-30 days
		CPU:           cpu,
		PVEVersion:    pveVersions[rand.Intn(len(pveVersions))],
		KernelVersion: kernelVersions[rand.Intn(len(kernelVersions))],
		Memory: models.Memory{
			Total: totalMem * 1024 * 1024 * 1024, // Convert to bytes
			Used:  usedMem * 1024 * 1024 * 1024,
			Free:  (totalMem - usedMem) * 1024 * 1024 * 1024,
			Usage: float64(usedMem) / float64(totalMem) * 100,
		},
		Disk: models.Disk{
			Total: totalDisk * 1024 * 1024 * 1024, // Convert to bytes
			Used:  usedDisk * 1024 * 1024 * 1024,
			Free:  (totalDisk - usedDisk) * 1024 * 1024 * 1024,
			Usage: float64(usedDisk) / float64(totalDisk) * 100,
		},
		CPUInfo: models.CPUInfo{
			Model:   "Intel(R) Xeon(R) CPU E5-2680 v4 @ 2.40GHz",
			Cores:   cores,
			Sockets: cores / 4, // Assume 4 cores per socket
			MHz:     "2400",
		},
		Temperature: temp,
		Host:        fmt.Sprintf("https://%s.local:8006", name),
		ID:          "", // Will be set by generateNodes to match real format: instance-nodename
	}
}

// generateNodeTemperature generates realistic temperature data for a node
func generateNodeTemperature(cores int) *models.Temperature {
	// 30% chance of no temperature data (sensors not available)
	if rand.Float64() < 0.3 {
		return &models.Temperature{
			Available: false,
		}
	}

	// Generate CPU package temperature (40-75°C normal range)
	cpuPackage := 40.0 + rand.Float64()*35.0
	if rand.Float64() < 0.1 { // 10% chance of high temp
		cpuPackage = 75.0 + rand.Float64()*15.0 // 75-90°C
	}

	// Generate core temperatures (similar to package, with variation)
	coreTemps := make([]models.CoreTemp, cores)
	maxTemp := cpuPackage
	for i := 0; i < cores; i++ {
		coreTemp := cpuPackage + (rand.Float64()-0.5)*10.0 // ±5°C from package
		if coreTemp < 30.0 {
			coreTemp = 30.0
		}
		if coreTemp > maxTemp {
			maxTemp = coreTemp
		}
		coreTemps[i] = models.CoreTemp{
			Core: i,
			Temp: coreTemp,
		}
	}

	// Generate NVMe temperatures (0-2 NVMe drives)
	numNVMe := rand.Intn(3)
	nvmeTemps := make([]models.NVMeTemp, numNVMe)
	for i := 0; i < numNVMe; i++ {
		nvmeTemp := 35.0 + rand.Float64()*40.0 // 35-75°C normal range
		if rand.Float64() < 0.05 {             // 5% chance of high temp
			nvmeTemp = 75.0 + rand.Float64()*10.0 // 75-85°C
		}
		nvmeTemps[i] = models.NVMeTemp{
			Device: fmt.Sprintf("nvme%d", i),
			Temp:   nvmeTemp,
		}
	}

	return &models.Temperature{
		CPUPackage: cpuPackage,
		CPUMax:     maxTemp,
		Cores:      coreTemps,
		NVMe:       nvmeTemps,
		Available:  true,
		LastUpdate: time.Now(),
	}
}

// generateRealisticIO generates more realistic I/O values
// Most systems are idle or have very low I/O
func generateRealisticIO(ioType string) int64 {
	chance := rand.Float64()

	switch ioType {
	case "disk-read":
		if chance < 0.60 { // 60% are idle
			return 0
		} else if chance < 0.85 { // 25% have low activity
			return int64(rand.Intn(5)) * 1024 * 1024 // 0-5 MB/s
		} else if chance < 0.95 { // 10% moderate
			return int64(5+rand.Intn(20)) * 1024 * 1024 // 5-25 MB/s
		} else { // 5% high activity
			return int64(25+rand.Intn(75)) * 1024 * 1024 // 25-100 MB/s
		}

	case "disk-write":
		if chance < 0.70 { // 70% are idle (writes are less common)
			return 0
		} else if chance < 0.90 { // 20% have low activity
			return int64(rand.Intn(3)) * 1024 * 1024 // 0-3 MB/s
		} else if chance < 0.97 { // 7% moderate
			return int64(3+rand.Intn(15)) * 1024 * 1024 // 3-18 MB/s
		} else { // 3% high activity
			return int64(18+rand.Intn(32)) * 1024 * 1024 // 18-50 MB/s
		}

	case "network-in":
		if chance < 0.50 { // 50% are idle
			return 0
		} else if chance < 0.80 { // 30% have low activity
			return int64(rand.Intn(10)) * 1024 * 1024 / 8 // 0-10 Mbps
		} else if chance < 0.93 { // 13% moderate
			return int64(10+rand.Intn(90)) * 1024 * 1024 / 8 // 10-100 Mbps
		} else { // 7% high activity
			return int64(100+rand.Intn(400)) * 1024 * 1024 / 8 // 100-500 Mbps
		}

	case "network-out":
		if chance < 0.55 { // 55% are idle
			return 0
		} else if chance < 0.82 { // 27% have low activity
			return int64(rand.Intn(5)) * 1024 * 1024 / 8 // 0-5 Mbps
		} else if chance < 0.94 { // 12% moderate
			return int64(5+rand.Intn(45)) * 1024 * 1024 / 8 // 5-50 Mbps
		} else { // 6% high activity
			return int64(50+rand.Intn(200)) * 1024 * 1024 / 8 // 50-250 Mbps
		}

	// Container I/O (generally lower than VMs)
	case "disk-read-ct":
		if chance < 0.65 { // 65% are idle
			return 0
		} else if chance < 0.90 { // 25% have low activity
			return int64(rand.Intn(3)) * 1024 * 1024 // 0-3 MB/s
		} else if chance < 0.97 { // 7% moderate
			return int64(3+rand.Intn(12)) * 1024 * 1024 // 3-15 MB/s
		} else { // 3% high activity
			return int64(15+rand.Intn(35)) * 1024 * 1024 // 15-50 MB/s
		}

	case "disk-write-ct":
		if chance < 0.75 { // 75% are idle
			return 0
		} else if chance < 0.92 { // 17% have low activity
			return int64(rand.Intn(2)) * 1024 * 1024 // 0-2 MB/s
		} else if chance < 0.98 { // 6% moderate
			return int64(2+rand.Intn(8)) * 1024 * 1024 // 2-10 MB/s
		} else { // 2% high activity
			return int64(10+rand.Intn(20)) * 1024 * 1024 // 10-30 MB/s
		}

	case "network-in-ct":
		if chance < 0.55 { // 55% are idle
			return 0
		} else if chance < 0.85 { // 30% have low activity
			return int64(rand.Intn(5)) * 1024 * 1024 / 8 // 0-5 Mbps
		} else if chance < 0.96 { // 11% moderate
			return int64(5+rand.Intn(25)) * 1024 * 1024 / 8 // 5-30 Mbps
		} else { // 4% high activity
			return int64(30+rand.Intn(70)) * 1024 * 1024 / 8 // 30-100 Mbps
		}

	case "network-out-ct":
		if chance < 0.60 { // 60% are idle
			return 0
		} else if chance < 0.87 { // 27% have low activity
			return int64(rand.Intn(3)) * 1024 * 1024 / 8 // 0-3 Mbps
		} else if chance < 0.96 { // 9% moderate
			return int64(3+rand.Intn(17)) * 1024 * 1024 / 8 // 3-20 Mbps
		} else { // 4% high activity
			return int64(20+rand.Intn(80)) * 1024 * 1024 / 8 // 20-100 Mbps
		}
	}

	return 0
}

func generateVM(nodeName string, instance string, vmid int, config MockConfig) models.VM {
	name := generateGuestName("vm")
	status := "running"
	if rand.Float64() < config.StoppedPercent {
		status = "stopped"
	}

	cpu := float64(0)
	mem := models.Memory{}
	uptime := int64(0)

	if status == "running" {
		// More realistic CPU usage: mostly low with occasional spikes
		cpuRand := rand.Float64()
		if cpuRand < 0.85 { // 85% of VMs have low CPU
			cpu = rand.Float64() * 0.25 // 0-25%
		} else if cpuRand < 0.97 { // 12% moderate CPU
			cpu = 0.25 + rand.Float64()*0.35 // 25-60%
		} else { // 3% high CPU (can trigger alerts at 80%)
			cpu = 0.60 + rand.Float64()*0.25 // 60-85%
		}

		totalMem := int64((4 + rand.Intn(28)) * 1024 * 1024 * 1024) // 4-32 GB
		// More realistic memory usage: most VMs use 30-65% memory
		var memUsage float64
		memRand := rand.Float64()
		if memRand < 0.85 { // 85% typical usage
			memUsage = 0.3 + rand.Float64()*0.35 // 30-65%
		} else if memRand < 0.97 { // 12% moderate usage
			memUsage = 0.65 + rand.Float64()*0.15 // 65-80%
		} else { // 3% high memory (can trigger alerts at 85%)
			memUsage = 0.80 + rand.Float64()*0.1 // 80-90%
		}
		usedMem := int64(float64(totalMem) * memUsage)
		balloon := totalMem
		if rand.Float64() < 0.7 {
			balloon = int64(float64(totalMem) * (0.65 + rand.Float64()*0.25))
		}

		swapTotal := int64(0)
		swapUsed := int64(0)
		if rand.Float64() < 0.6 {
			swapTotal = int64((1 + rand.Intn(5)) * 1024 * 1024 * 1024) // 1-5 GB
			swapUsed = int64(float64(swapTotal) * (0.1 + rand.Float64()*0.4))
		}

		mem = models.Memory{
			Total:     totalMem,
			Used:      usedMem,
			Free:      totalMem - usedMem,
			Usage:     memUsage * 100,
			Balloon:   balloon,
			SwapTotal: swapTotal,
			SwapUsed:  swapUsed,
		}
		uptime = int64(3600 * (1 + rand.Intn(720))) // 1-720 hours
	}

	// Disk stats
	virtualDisks, aggregatedDisk := generateVirtualDisks()
	diskStatusReason := ""

	if status != "running" {
		diskStatusReason = "vm-stopped"
		aggregatedDisk.Usage = -1
		aggregatedDisk.Used = 0
		aggregatedDisk.Free = aggregatedDisk.Total
		virtualDisks = nil
	} else if rand.Float64() < 0.1 {
		// Simulate agent issues where detailed disks are unavailable
		diskStatusReason = "agent-not-running"
		aggregatedDisk.Usage = -1
		aggregatedDisk.Used = 0
		aggregatedDisk.Free = aggregatedDisk.Total
		virtualDisks = nil
	}

	// Generate ID matching production logic: standalone uses "node-vmid", cluster uses "instance-node-vmid"
	var vmID string
	if instance == nodeName {
		vmID = fmt.Sprintf("%s-%d", nodeName, vmid)
	} else {
		vmID = fmt.Sprintf("%s-%s-%d", instance, nodeName, vmid)
	}

	osName, osVersion := generateGuestOSMetadata()
	ipAddresses, networkIfaces := generateGuestNetworkInfo()

	return models.VM{
		Name:              name,
		VMID:              vmid,
		Node:              nodeName,
		Instance:          instance,
		Type:              "qemu",
		Status:            status,
		CPU:               cpu,
		CPUs:              2 + rand.Intn(6), // 2-8 cores
		Memory:            mem,
		Disk:              aggregatedDisk,
		Disks:             virtualDisks,
		DiskStatusReason:  diskStatusReason,
		DiskRead:          generateRealisticIO("disk-read"),
		DiskWrite:         generateRealisticIO("disk-write"),
		NetworkIn:         generateRealisticIO("network-in"),
		NetworkOut:        generateRealisticIO("network-out"),
		Uptime:            uptime,
		ID:                vmID,
		Tags:              generateTags(),
		IPAddresses:       ipAddresses,
		OSName:            osName,
		OSVersion:         osVersion,
		NetworkInterfaces: networkIfaces,
	}
}

func generateGuestNetworkInfo() ([]string, []models.GuestNetworkInterface) {
	ifaceCount := 1 + rand.Intn(2)
	ipSet := make(map[string]struct{})
	ipAddresses := make([]string, 0, ifaceCount*2)
	interfaces := make([]models.GuestNetworkInterface, 0, ifaceCount)

	for i := 0; i < ifaceCount; i++ {
		name := fmt.Sprintf("eth%d", i)
		if rand.Float64() < 0.3 {
			name = fmt.Sprintf("ens%d", i+3)
		}

		mac := fmt.Sprintf("52:54:%02x:%02x:%02x:%02x",
			rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))

		addrCount := 1 + rand.Intn(2)
		addresses := make([]string, 0, addrCount)

		for len(addresses) < addrCount {
			var ip string
			if rand.Float64() < 0.2 {
				ip = fmt.Sprintf("fd00:%x:%x::%x", rand.Intn(1<<16), rand.Intn(1<<16), rand.Intn(1<<16))
			} else {
				ip = fmt.Sprintf("10.%d.%d.%d", rand.Intn(200)+1, rand.Intn(254), rand.Intn(254))
			}

			if _, exists := ipSet[ip]; exists {
				continue
			}
			ipSet[ip] = struct{}{}
			addresses = append(addresses, ip)
			ipAddresses = append(ipAddresses, ip)
		}

		rxBytes := int64(rand.Int63n(8*1024*1024*1024) + rand.Int63n(256*1024*1024))
		txBytes := int64(rand.Int63n(6*1024*1024*1024) + rand.Int63n(256*1024*1024))

		interfaces = append(interfaces, models.GuestNetworkInterface{
			Name:      name,
			MAC:       mac,
			Addresses: addresses,
			RXBytes:   rxBytes,
			TXBytes:   txBytes,
		})
	}

	sort.Strings(ipAddresses)

	return ipAddresses, interfaces
}

func generateGuestOSMetadata() (string, string) {
	variants := []struct {
		Name    string
		Version string
	}{
		{"Ubuntu", "22.04 LTS"},
		{"Ubuntu", "24.04 LTS"},
		{"Debian GNU/Linux", "12 (Bookworm)"},
		{"Debian GNU/Linux", "11 (Bullseye)"},
		{"CentOS Stream", "9"},
		{"Rocky Linux", "9.3"},
		{"AlmaLinux", "8.10"},
		{"Fedora", fmt.Sprintf("%d", 38+rand.Intn(3))},
		{"Windows Server", "2019"},
		{"Windows Server", "2022"},
		{"Arch Linux", "rolling"},
	}

	choice := variants[rand.Intn(len(variants))]
	return choice.Name, choice.Version
}

func generateContainer(nodeName string, instance string, vmid int, config MockConfig) models.Container {
	name := generateGuestName("lxc")
	status := "running"
	if rand.Float64() < config.StoppedPercent {
		status = "stopped"
	}

	cpu := float64(0)
	mem := models.Memory{}
	uptime := int64(0)

	if status == "running" {
		// More realistic CPU for containers: mostly very low
		cpuRand := rand.Float64()
		if cpuRand < 0.90 { // 90% of containers have minimal CPU
			cpu = rand.Float64() * 0.12 // 0-12%
		} else if cpuRand < 0.98 { // 8% moderate CPU
			cpu = 0.12 + rand.Float64()*0.28 // 12-40%
		} else { // 2% higher CPU (can trigger alerts at 80%)
			cpu = 0.40 + rand.Float64()*0.35 // 40-75%
		}

		totalMem := int64((512 + rand.Intn(7680)) * 1024 * 1024) // 512 MB - 8 GB
		// More realistic memory for containers
		var memUsage float64
		memRand := rand.Float64()
		if memRand < 0.90 { // 90% typical usage
			memUsage = 0.35 + rand.Float64()*0.35 // 35-70%
		} else if memRand < 0.98 { // 8% moderate usage
			memUsage = 0.70 + rand.Float64()*0.12 // 70-82%
		} else { // 2% high memory (can trigger alerts at 85%)
			memUsage = 0.82 + rand.Float64()*0.08 // 82-90%
		}
		usedMem := int64(float64(totalMem) * memUsage)
		balloon := totalMem
		if rand.Float64() < 0.5 {
			balloon = int64(float64(totalMem) * (0.7 + rand.Float64()*0.2))
		}

		swapTotal := int64(0)
		swapUsed := int64(0)
		if rand.Float64() < 0.4 {
			swapTotal = int64((256 + rand.Intn(1024)) * 1024 * 1024) // 256MB - 1.25GB
			swapUsed = int64(float64(swapTotal) * (0.1 + rand.Float64()*0.4))
		}

		mem = models.Memory{
			Total:     totalMem,
			Used:      usedMem,
			Free:      totalMem - usedMem,
			Usage:     memUsage * 100,
			Balloon:   balloon,
			SwapTotal: swapTotal,
			SwapUsed:  swapUsed,
		}
		uptime = int64(3600 * (1 + rand.Intn(1440))) // 1-1440 hours (up to 60 days)
	}

	// Disk stats - containers typically smaller
	totalDisk := int64((8 + rand.Intn(120)) * 1024 * 1024 * 1024) // 8-128 GB
	usedDisk := int64(float64(totalDisk) * (0.1 + rand.Float64()*0.6))

	// Generate ID matching production logic: standalone uses "node-vmid", cluster uses "instance-node-vmid"
	var ctID string
	if instance == nodeName {
		ctID = fmt.Sprintf("%s-%d", nodeName, vmid)
	} else {
		ctID = fmt.Sprintf("%s-%s-%d", instance, nodeName, vmid)
	}

	return models.Container{
		Name:     name,
		VMID:     vmid,
		Node:     nodeName,
		Instance: instance,
		Type:     "lxc",
		Status:   status,
		CPU:      cpu,
		CPUs:     1 + rand.Intn(4), // 1-4 cores
		Memory:   mem,
		Disk: models.Disk{
			Total: totalDisk,
			Used:  usedDisk,
			Free:  totalDisk - usedDisk,
			Usage: float64(usedDisk) / float64(totalDisk) * 100,
		},
		DiskRead:   generateRealisticIO("disk-read-ct"),
		DiskWrite:  generateRealisticIO("disk-write-ct"),
		NetworkIn:  generateRealisticIO("network-in-ct"),
		NetworkOut: generateRealisticIO("network-out-ct"),
		Uptime:     uptime,
		ID:         ctID,
		Tags:       generateTags(),
	}
}

func generateGuestName(prefix string) string {
	return fmt.Sprintf("%s-%s-%d", prefix, appNames[rand.Intn(len(appNames))], rand.Intn(100))
}

// generateTags generates random tags for a guest
func generateTags() []string {
	// 30% chance of no tags
	if rand.Float64() < 0.3 {
		return []string{}
	}

	// Generate 1-4 tags
	numTags := 1 + rand.Intn(4)
	tags := make([]string, 0, numTags)
	usedTags := make(map[string]bool)

	for len(tags) < numTags {
		tag := commonTags[rand.Intn(len(commonTags))]
		// Avoid duplicate tags
		if !usedTags[tag] {
			tags = append(tags, tag)
			usedTags[tag] = true
		}
	}

	return tags
}

// GenerateAlerts generates random alerts for testing
func GenerateAlerts(nodes []models.Node, vms []models.VM, containers []models.Container) []models.Alert {
	alerts := []models.Alert{}

	// Generate some node alerts
	for _, node := range nodes {
		// Add offline alert for offline nodes
		if node.Status == "offline" {
			alerts = append(alerts, models.Alert{
				ID:           fmt.Sprintf("node-offline-%s", node.ID),
				Type:         "connectivity",
				Level:        "critical",
				ResourceID:   node.ID,
				ResourceName: node.Name,
				Node:         node.Name,
				Message:      fmt.Sprintf("Node '%s' is offline", node.Name),
				Value:        0,
				Threshold:    0,
				StartTime:    time.Now().Add(-time.Minute * 5), // Offline for 5 minutes
			})
			continue // Skip other checks for offline nodes
		}

		if node.CPU > 0.8 {
			alerts = append(alerts, models.Alert{
				ID:           fmt.Sprintf("alert-%s-cpu", node.Name),
				Type:         "threshold",
				Level:        "warning",
				ResourceID:   node.ID,
				ResourceName: node.Name,
				Node:         node.Name,
				Message:      fmt.Sprintf("Node %s CPU usage is %.0f%%", node.Name, node.CPU*100),
				Value:        node.CPU * 100,
				Threshold:    80,
				StartTime:    time.Now(),
			})
		}
	}

	// Generate some VM/container alerts
	allGuests := make([]interface{}, 0, len(vms)+len(containers))
	for _, vm := range vms {
		allGuests = append(allGuests, vm)
	}
	for _, ct := range containers {
		allGuests = append(allGuests, ct)
	}

	// Pick random guests to have alerts
	numAlerts := rand.Intn(5) + 1
	for i := 0; i < numAlerts && i < len(allGuests); i++ {
		guestIdx := rand.Intn(len(allGuests))

		var name, id string
		var cpu float64
		var memUsage float64

		switch g := allGuests[guestIdx].(type) {
		case models.VM:
			name = g.Name
			id = g.ID
			cpu = g.CPU
			memUsage = g.Memory.Usage
		case models.Container:
			name = g.Name
			id = g.ID
			cpu = g.CPU
			memUsage = g.Memory.Usage
		}

		// Randomly choose alert type
		switch rand.Intn(3) {
		case 0: // CPU alert
			if cpu > 0.7 {
				alerts = append(alerts, models.Alert{
					ID:           fmt.Sprintf("alert-%s-cpu", id),
					Type:         "threshold",
					Level:        "warning",
					ResourceID:   id,
					ResourceName: name,
					Message:      fmt.Sprintf("%s CPU usage is %.0f%%", name, cpu*100),
					Value:        cpu * 100,
					Threshold:    70,
					StartTime:    time.Now(),
				})
			}
		case 1: // Memory alert
			if memUsage > 80 {
				alerts = append(alerts, models.Alert{
					ID:           fmt.Sprintf("alert-%s-mem", id),
					Type:         "threshold",
					Level:        "warning",
					ResourceID:   id,
					ResourceName: name,
					Message:      fmt.Sprintf("%s memory usage is %.0f%%", name, memUsage),
					Value:        memUsage,
					Threshold:    80,
					StartTime:    time.Now(),
				})
			}
		case 2: // Disk alert
			diskUsage := 70 + rand.Float64()*25
			alerts = append(alerts, models.Alert{
				ID:           fmt.Sprintf("alert-%s-disk", id),
				Type:         "threshold",
				Level:        "critical",
				ResourceID:   id,
				ResourceName: name,
				Message:      fmt.Sprintf("%s disk usage is %.0f%%", name, diskUsage),
				Value:        diskUsage,
				Threshold:    90,
				StartTime:    time.Now(),
			})
		}
	}

	return alerts
}

// generateZFSPoolWithIssues creates a ZFS pool with various issues for testing
func generateZFSPoolWithIssues(poolName string) *models.ZFSPool {
	scenarios := []func() *models.ZFSPool{
		// Degraded pool with device errors
		func() *models.ZFSPool {
			return &models.ZFSPool{
				Name:           poolName,
				State:          "DEGRADED",
				Status:         "Degraded",
				Scan:           "resilver in progress",
				ReadErrors:     12,
				WriteErrors:    0,
				ChecksumErrors: 3,
				Devices: []models.ZFSDevice{
					{
						Name:           "sda2",
						Type:           "disk",
						State:          "ONLINE",
						ReadErrors:     0,
						WriteErrors:    0,
						ChecksumErrors: 0,
					},
					{
						Name:           "sdb2",
						Type:           "disk",
						State:          "DEGRADED",
						ReadErrors:     12,
						WriteErrors:    0,
						ChecksumErrors: 3,
					},
				},
			}
		},
		// Pool with checksum errors
		func() *models.ZFSPool {
			return &models.ZFSPool{
				Name:           poolName,
				State:          "ONLINE",
				Status:         "Healthy",
				Scan:           "scrub in progress",
				ReadErrors:     0,
				WriteErrors:    0,
				ChecksumErrors: 7,
				Devices: []models.ZFSDevice{
					{
						Name:           "sda2",
						Type:           "disk",
						State:          "ONLINE",
						ReadErrors:     0,
						WriteErrors:    0,
						ChecksumErrors: 7,
					},
				},
			}
		},
		// Faulted device
		func() *models.ZFSPool {
			return &models.ZFSPool{
				Name:           poolName,
				State:          "DEGRADED",
				Status:         "Degraded",
				Scan:           "none",
				ReadErrors:     0,
				WriteErrors:    0,
				ChecksumErrors: 0,
				Devices: []models.ZFSDevice{
					{
						Name:           "mirror-0",
						Type:           "mirror",
						State:          "DEGRADED",
						ReadErrors:     0,
						WriteErrors:    0,
						ChecksumErrors: 0,
					},
					{
						Name:           "sda2",
						Type:           "disk",
						State:          "ONLINE",
						ReadErrors:     0,
						WriteErrors:    0,
						ChecksumErrors: 0,
					},
					{
						Name:           "sdb2",
						Type:           "disk",
						State:          "FAULTED",
						ReadErrors:     0,
						WriteErrors:    0,
						ChecksumErrors: 0,
					},
				},
			}
		},
	}

	// Pick a random scenario
	return scenarios[rand.Intn(len(scenarios))]()
}

// generateStorage generates mock storage data for nodes
func generateStorage(nodes []models.Node) []models.Storage {
	var storage []models.Storage
	storageTypes := []string{"dir", "zfspool", "lvm", "nfs", "cephfs"}
	contentTypes := []string{"images", "vztmpl,iso", "rootdir", "backup", "snippets"}

	for _, node := range nodes {
		// Local storage (always present)
		localTotal := int64(500 * 1024 * 1024 * 1024) // 500GB
		localUsed := int64(float64(localTotal) * (0.3 + rand.Float64()*0.5))
		storage = append(storage, models.Storage{
			ID:       fmt.Sprintf("%s-%s-local", node.Instance, node.Name),
			Name:     "local",
			Node:     node.Name,
			Instance: node.Instance,
			Type:     "dir",
			Status:   "available",
			Total:    localTotal,
			Used:     localUsed,
			Free:     localTotal - localUsed,
			Usage:    float64(localUsed) / float64(localTotal) * 100,
			Content:  "vztmpl,iso",
			Shared:   false,
			Enabled:  true,
			Active:   true,
		})

		// Local-zfs (common)
		zfsTotal := int64(2 * 1024 * 1024 * 1024 * 1024) // 2TB
		zfsUsed := int64(float64(zfsTotal) * (0.2 + rand.Float64()*0.6))

		// Generate ZFS pool status
		var zfsPool *models.ZFSPool
		if rand.Float64() < 0.15 { // 15% chance of ZFS pool with issues
			zfsPool = generateZFSPoolWithIssues("local-zfs")
		} else if rand.Float64() < 0.95 { // Most ZFS pools are healthy
			zfsPool = &models.ZFSPool{
				Name:           "rpool/data",
				State:          "ONLINE",
				Status:         "Healthy",
				Scan:           "none",
				ReadErrors:     0,
				WriteErrors:    0,
				ChecksumErrors: 0,
				Devices: []models.ZFSDevice{
					{
						Name:           "sda2",
						Type:           "disk",
						State:          "ONLINE",
						ReadErrors:     0,
						WriteErrors:    0,
						ChecksumErrors: 0,
					},
				},
			}
		}

		storage = append(storage, models.Storage{
			ID:       fmt.Sprintf("%s-%s-local-zfs", node.Instance, node.Name),
			Name:     "local-zfs",
			Node:     node.Name,
			Instance: node.Instance,
			Type:     "zfspool",
			Status:   "available",
			Total:    zfsTotal,
			Used:     zfsUsed,
			Free:     zfsTotal - zfsUsed,
			Usage:    float64(zfsUsed) / float64(zfsTotal) * 100,
			Content:  "images,rootdir",
			Shared:   false,
			Enabled:  true,
			Active:   true,
			ZFSPool:  zfsPool,
		})

		// Add one more random storage per node
		if rand.Float64() > 0.3 {
			storageType := storageTypes[rand.Intn(len(storageTypes))]
			storageName := fmt.Sprintf("storage-%s-%d", node.Name, rand.Intn(100))
			total := int64((1 + rand.Intn(10)) * 1024 * 1024 * 1024 * 1024) // 1-10TB
			used := int64(float64(total) * rand.Float64())

			storage = append(storage, models.Storage{
				ID:       fmt.Sprintf("%s-%s-%s", node.Instance, node.Name, storageName),
				Name:     storageName,
				Node:     node.Name,
				Instance: node.Instance,
				Type:     storageType,
				Status:   "available",
				Total:    total,
				Used:     used,
				Free:     total - used,
				Usage:    float64(used) / float64(total) * 100,
				Content:  contentTypes[rand.Intn(len(contentTypes))],
				Shared:   storageType == "nfs" || storageType == "cephfs",
				Enabled:  true,
				Active:   true,
			})
		}
	}

	// Add PBS storage for each node (simulating node-specific PBS namespaces)
	// In real clusters, each node reports ALL PBS storage entries but with node-specific namespaces
	// This matches real behavior where each node sees all PBS configurations
	clusterNodes := []models.Node{}
	for _, n := range nodes {
		if n.Instance == "mock-cluster" {
			clusterNodes = append(clusterNodes, n)
		}
	}

	// Each cluster node reports ALL PBS storage entries (one for each node)
	for _, node := range clusterNodes {
		// Each node sees ALL PBS storage configurations
		for _, pbsTargetNode := range clusterNodes {
			pbsTotal := int64(950 * 1024 * 1024 * 1024) // ~950GB matching real PBS
			pbsUsed := int64(float64(pbsTotal) * 0.14)  // ~14% usage matching real data
			storage = append(storage, models.Storage{
				ID:       fmt.Sprintf("%s-%s-pbs-%s", node.Instance, node.Name, pbsTargetNode.Name),
				Name:     fmt.Sprintf("pbs-%s", pbsTargetNode.Name),
				Node:     node.Name, // The node that reports this storage
				Instance: node.Instance,
				Type:     "pbs",
				Status:   "available",
				Total:    pbsTotal,
				Used:     pbsUsed,
				Free:     pbsTotal - pbsUsed,
				Usage:    float64(pbsUsed) / float64(pbsTotal) * 100,
				Content:  "backup",
				Shared:   true, // PBS storage is shared cluster-wide (all nodes can access it)
				Enabled:  true,
				Active:   true,
			})
		}
	}

	// Add a shared storage (NFS or CephFS)
	if len(nodes) > 1 {
		sharedTotal := int64(10 * 1024 * 1024 * 1024 * 1024) // 10TB
		sharedUsed := int64(float64(sharedTotal) * (0.4 + rand.Float64()*0.3))
		storage = append(storage, models.Storage{
			ID:       "shared-storage",
			Name:     "shared-storage",
			Node:     "shared", // Shared storage uses "shared" as node per production code
			Instance: nodes[0].Instance,
			Type:     "nfs",
			Status:   "available",
			Total:    sharedTotal,
			Used:     sharedUsed,
			Free:     sharedTotal - sharedUsed,
			Usage:    float64(sharedUsed) / float64(sharedTotal) * 100,
			Content:  "images,rootdir,backup",
			Shared:   true,
			Enabled:  true,
			Active:   true,
		})
	}

	return storage
}

// generateBackups generates mock backup data for VMs and containers
func generateBackups(vms []models.VM, containers []models.Container) []models.StorageBackup {
	var backups []models.StorageBackup
	backupFormats := []string{"vma.zst", "vma.lzo", "tar.zst", "tar.gz"}

	// Generate backups for ~60% of VMs
	for _, vm := range vms {
		if rand.Float64() > 0.4 {
			continue
		}

		// Generate 1-3 backups per VM
		numBackups := 1 + rand.Intn(3)
		for i := 0; i < numBackups; i++ {
			backupTime := time.Now().Add(-time.Duration(rand.Intn(30*24)) * time.Hour)
			backupSize := int64(vm.Disk.Total/10 + rand.Int63n(vm.Disk.Total/5)) // 10-30% of disk size

			backup := models.StorageBackup{
				ID:        fmt.Sprintf("backup-%s-vm-%d-%d", vm.Node, vm.VMID, i),
				Storage:   "local",
				Node:      vm.Node,
				Instance:  vm.Instance,
				Type:      "qemu",
				VMID:      vm.VMID,
				Time:      backupTime,
				CTime:     backupTime.Unix(),
				Size:      backupSize,
				Format:    backupFormats[rand.Intn(len(backupFormats))],
				Notes:     fmt.Sprintf("Backup of %s", vm.Name),
				Protected: rand.Float64() > 0.8, // 20% protected
				Volid:     fmt.Sprintf("local:backup/vzdump-qemu-%d-%s.%s", vm.VMID, backupTime.Format("2006_01_02-15_04_05"), backupFormats[0]),
				IsPBS:     false,
				Verified:  rand.Float64() > 0.3, // 70% verified
			}

			if backup.Verified {
				backup.Verification = "OK"
			}

			backups = append(backups, backup)
		}
	}

	// Generate backups for ~70% of containers
	for _, ct := range containers {
		if rand.Float64() > 0.3 {
			continue
		}

		// Generate 1-2 backups per container
		numBackups := 1 + rand.Intn(2)
		for i := 0; i < numBackups; i++ {
			backupTime := time.Now().Add(-time.Duration(rand.Intn(30*24)) * time.Hour)
			backupSize := int64(ct.Disk.Total/20 + rand.Int63n(ct.Disk.Total/10)) // 5-15% of disk size

			backup := models.StorageBackup{
				ID:        fmt.Sprintf("backup-%s-ct-%d-%d", ct.Node, int(ct.VMID), i),
				Storage:   "local",
				Node:      ct.Node,
				Instance:  ct.Instance,
				Type:      "lxc",
				VMID:      int(ct.VMID),
				Time:      backupTime,
				CTime:     backupTime.Unix(),
				Size:      backupSize,
				Format:    "tar.zst",
				Notes:     fmt.Sprintf("Backup of %s", ct.Name),
				Protected: rand.Float64() > 0.9, // 10% protected
				Volid:     fmt.Sprintf("local:backup/vzdump-lxc-%d-%s.tar.zst", int(ct.VMID), backupTime.Format("2006_01_02-15_04_05")),
				IsPBS:     false,
				Verified:  rand.Float64() > 0.2, // 80% verified
			}

			if backup.Verified {
				backup.Verification = "OK"
			}

			backups = append(backups, backup)
		}
	}

	// Generate PMG host config backups (VMID=0)
	// Add 2-4 PMG host backups
	numPMGBackups := 2 + rand.Intn(3)
	pmgNodes := []string{"pmg-01", "pmg-02", "mail-gateway"}
	for i := 0; i < numPMGBackups; i++ {
		backupTime := time.Now().Add(-time.Duration(rand.Intn(60*24)) * time.Hour)
		nodeIdx := rand.Intn(len(pmgNodes))

		backup := models.StorageBackup{
			ID:        fmt.Sprintf("backup-pmg-host-%d", i),
			Storage:   "local",
			Node:      pmgNodes[nodeIdx],
			Type:      "host", // This will now display as "Host" in the UI
			VMID:      0,      // Host backups have VMID=0
			Time:      backupTime,
			CTime:     backupTime.Unix(),
			Size:      int64(50*1024*1024 + rand.Intn(200*1024*1024)), // 50-250 MB
			Format:    "pmgbackup.tar.zst",
			Notes:     fmt.Sprintf("PMG host config backup - %s", pmgNodes[nodeIdx]),
			Protected: rand.Float64() > 0.7, // 30% protected
			Volid:     fmt.Sprintf("local:backup/pmgbackup-%s-%s.tar.zst", pmgNodes[nodeIdx], backupTime.Format("2006_01_02-15_04_05")),
			IsPBS:     false,
			Verified:  rand.Float64() > 0.2, // 80% verified
		}

		if backup.Verified {
			backup.Verification = "OK"
		}

		backups = append(backups, backup)
	}

	// Sort backups by time (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Time.After(backups[j].Time)
	})

	return backups
}

// generatePBSInstances generates mock PBS instances
func generatePBSInstances() []models.PBSInstance {
	pbsInstances := []models.PBSInstance{
		{
			ID:          "pbs-main",
			Name:        "pbs-main",
			Host:        "192.168.0.10:8007",
			Status:      "online",
			Version:     "3.2.1",
			CPU:         15.5 + rand.Float64()*10,
			Memory:      45.2 + rand.Float64()*20,
			MemoryUsed:  int64(8 * 1024 * 1024 * 1024),  // 8GB
			MemoryTotal: int64(16 * 1024 * 1024 * 1024), // 16GB
			Uptime:      int64(86400 * 30),              // 30 days
			Datastores: []models.PBSDatastore{
				{
					Name:   "backup-store",
					Total:  int64(10 * 1024 * 1024 * 1024 * 1024), // 10TB
					Used:   int64(4 * 1024 * 1024 * 1024 * 1024),  // 4TB
					Free:   int64(6 * 1024 * 1024 * 1024 * 1024),  // 6TB
					Usage:  40.0,
					Status: "available",
				},
				{
					Name:   "offsite-backup",
					Total:  int64(20 * 1024 * 1024 * 1024 * 1024), // 20TB
					Used:   int64(12 * 1024 * 1024 * 1024 * 1024), // 12TB
					Free:   int64(8 * 1024 * 1024 * 1024 * 1024),  // 8TB
					Usage:  60.0,
					Status: "available",
				},
			},
			ConnectionHealth: "healthy",
			LastSeen:         time.Now(),
		},
	}

	// Add a secondary PBS if we have enough nodes
	if rand.Float64() > 0.4 {
		pbsInstances = append(pbsInstances, models.PBSInstance{
			ID:          "pbs-secondary",
			Name:        "pbs-secondary",
			Host:        "192.168.0.11:8007",
			Status:      "online",
			Version:     "3.2.0",
			CPU:         10.2 + rand.Float64()*8,
			Memory:      35.5 + rand.Float64()*15,
			MemoryUsed:  int64(4 * 1024 * 1024 * 1024), // 4GB
			MemoryTotal: int64(8 * 1024 * 1024 * 1024), // 8GB
			Uptime:      int64(86400 * 15),             // 15 days
			Datastores: []models.PBSDatastore{
				{
					Name:   "replica-store",
					Total:  int64(5 * 1024 * 1024 * 1024 * 1024), // 5TB
					Used:   int64(2 * 1024 * 1024 * 1024 * 1024), // 2TB
					Free:   int64(3 * 1024 * 1024 * 1024 * 1024), // 3TB
					Usage:  40.0,
					Status: "available",
				},
			},
			ConnectionHealth: "healthy",
			LastSeen:         time.Now(),
		})
	}

	return pbsInstances
}

// generatePBSBackups generates mock PBS backup data
func generatePBSBackups(vms []models.VM, containers []models.Container) []models.PBSBackup {
	var backups []models.PBSBackup
	pbsInstances := []string{"pbs-main"}
	datastores := []string{"backup-store", "offsite-backup"}
	owners := []string{"admin@pbs", "backup@pbs", "root@pam", "automation@pbs", "user1@pve", "service@pbs"}

	// Add secondary PBS to list if it might exist
	if rand.Float64() > 0.4 {
		pbsInstances = append(pbsInstances, "pbs-secondary")
		datastores = append(datastores, "replica-store")
	}

	// Generate PBS backups for ~50% of VMs
	for _, vm := range vms {
		if rand.Float64() > 0.5 {
			continue
		}

		// Generate 2-4 PBS backups per VM
		numBackups := 2 + rand.Intn(3)
		for i := 0; i < numBackups; i++ {
			backupTime := time.Now().Add(-time.Duration(rand.Intn(60*24)) * time.Hour)

			backup := models.PBSBackup{
				ID:         fmt.Sprintf("pbs-backup-vm-%d-%d", vm.VMID, i),
				Instance:   pbsInstances[rand.Intn(len(pbsInstances))],
				Datastore:  datastores[rand.Intn(len(datastores))],
				Namespace:  "root",
				BackupType: "vm",
				VMID:       fmt.Sprintf("%d", vm.VMID),
				BackupTime: backupTime,
				Size:       int64(vm.Disk.Total/8 + rand.Int63n(vm.Disk.Total/4)),
				Protected:  rand.Float64() > 0.85, // 15% protected
				Verified:   rand.Float64() > 0.2,  // 80% verified
				Comment:    fmt.Sprintf("Automated backup of %s", vm.Name),
				Owner:      owners[rand.Intn(len(owners))],
			}

			backups = append(backups, backup)
		}
	}

	// Generate PBS backups for ~60% of containers
	for _, ct := range containers {
		if rand.Float64() > 0.4 {
			continue
		}

		// Generate 1-3 PBS backups per container
		numBackups := 1 + rand.Intn(3)
		for i := 0; i < numBackups; i++ {
			backupTime := time.Now().Add(-time.Duration(rand.Intn(45*24)) * time.Hour)

			backup := models.PBSBackup{
				ID:         fmt.Sprintf("pbs-backup-ct-%d-%d", int(ct.VMID), i),
				Instance:   pbsInstances[rand.Intn(len(pbsInstances))],
				Datastore:  datastores[rand.Intn(len(datastores))],
				Namespace:  "root",
				BackupType: "ct",
				VMID:       fmt.Sprintf("%d", int(ct.VMID)),
				BackupTime: backupTime,
				Size:       int64(ct.Disk.Total/15 + rand.Int63n(ct.Disk.Total/8)),
				Protected:  rand.Float64() > 0.9,  // 10% protected
				Verified:   rand.Float64() > 0.15, // 85% verified
				Comment:    fmt.Sprintf("Daily backup of %s", ct.Name),
				Owner:      owners[rand.Intn(len(owners))],
			}

			backups = append(backups, backup)
		}
	}

	// Generate host config backups (VMID 0) - PMG/PVE host configs
	// These are common when backing up Proxmox Mail Gateway hosts
	hostBackupCount := 2 + rand.Intn(3) // 2-4 host backups
	for i := 0; i < hostBackupCount; i++ {
		backupTime := time.Now().Add(-time.Duration(rand.Intn(30*24)) * time.Hour)

		backup := models.PBSBackup{
			ID:         fmt.Sprintf("pbs-backup-host-0-%d", i),
			Instance:   pbsInstances[rand.Intn(len(pbsInstances))],
			Datastore:  datastores[rand.Intn(len(datastores))],
			Namespace:  "root",
			BackupType: "ct", // Host configs are stored as 'ct' type in PBS
			VMID:       "0",  // VMID 0 indicates host config
			BackupTime: backupTime,
			Size:       int64(50*1024*1024 + rand.Int63n(100*1024*1024)), // 50-150MB for host configs
			Protected:  rand.Float64() > 0.7,                             // 30% protected
			Verified:   rand.Float64() > 0.1,                             // 90% verified
			Comment:    "PMG host configuration backup",
			Owner:      "root@pam",
		}

		backups = append(backups, backup)
	}

	// Sort by time (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].BackupTime.After(backups[j].BackupTime)
	})

	return backups
}

// generateSnapshots generates mock snapshot data for VMs and containers
func generateSnapshots(vms []models.VM, containers []models.Container) []models.GuestSnapshot {
	var snapshots []models.GuestSnapshot
	snapshotNames := []string{"before-upgrade", "production", "testing", "stable", "rollback-point", "pre-maintenance", "weekly-snapshot"}

	// Generate snapshots for ~40% of VMs
	for _, vm := range vms {
		if rand.Float64() > 0.6 {
			continue
		}

		// Generate 1-3 snapshots per VM
		numSnapshots := 1 + rand.Intn(3)
		for i := 0; i < numSnapshots; i++ {
			snapshotTime := time.Now().Add(-time.Duration(rand.Intn(90*24)) * time.Hour)

			snapshot := models.GuestSnapshot{
				ID:          fmt.Sprintf("snapshot-%s-vm-%d-%d", vm.Node, vm.VMID, i),
				Name:        snapshotNames[rand.Intn(len(snapshotNames))],
				Node:        vm.Node,
				Instance:    vm.Instance,
				Type:        "qemu",
				VMID:        vm.VMID,
				Time:        snapshotTime,
				Description: fmt.Sprintf("Snapshot of %s taken on %s", vm.Name, snapshotTime.Format("2006-01-02")),
				VMState:     rand.Float64() > 0.5, // 50% include VM state
			}

			// Add parent relationship for some snapshots
			if i > 0 && rand.Float64() > 0.5 {
				snapshot.Parent = fmt.Sprintf("snapshot-%s-vm-%d-%d", vm.Node, vm.VMID, i-1)
			}

			snapshots = append(snapshots, snapshot)
		}
	}

	// Generate snapshots for ~30% of containers
	for _, ct := range containers {
		if rand.Float64() > 0.7 {
			continue
		}

		// Generate 1-2 snapshots per container
		numSnapshots := 1 + rand.Intn(2)
		for i := 0; i < numSnapshots; i++ {
			snapshotTime := time.Now().Add(-time.Duration(rand.Intn(60*24)) * time.Hour)

			snapshot := models.GuestSnapshot{
				ID:          fmt.Sprintf("snapshot-%s-ct-%d-%d", ct.Node, int(ct.VMID), i),
				Name:        snapshotNames[rand.Intn(len(snapshotNames))],
				Node:        ct.Node,
				Instance:    ct.Instance,
				Type:        "lxc",
				VMID:        int(ct.VMID),
				Time:        snapshotTime,
				Description: fmt.Sprintf("Container snapshot for %s", ct.Name),
				VMState:     false, // Containers don't have VM state
			}

			snapshots = append(snapshots, snapshot)
		}
	}

	// Sort by time (newest first)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Time.After(snapshots[j].Time)
	})

	return snapshots
}

// UpdateMetrics simulates changing metrics over time
func UpdateMetrics(data *models.StateSnapshot, config MockConfig) {
	if !config.RandomMetrics {
		return
	}

	// Update node metrics
	for i := range data.Nodes {
		node := &data.Nodes[i]
		// Small random walk for CPU with mean reversion
		change := (rand.Float64() - 0.5) * 0.03
		// Mean reversion: pull toward 20% CPU
		meanReversion := (0.20 - node.CPU) * 0.05
		node.CPU += change + meanReversion
		node.CPU = math.Max(0.05, math.Min(0.85, node.CPU))

		// Update memory with very small changes and mean reversion toward 50%
		memChange := (rand.Float64() - 0.5) * 0.02
		memMeanReversion := (50.0 - node.Memory.Usage) * 0.03
		node.Memory.Usage += (memChange * 100) + memMeanReversion
		node.Memory.Usage = math.Max(10, math.Min(85, node.Memory.Usage))
		node.Memory.Used = int64(float64(node.Memory.Total) * (node.Memory.Usage / 100))
		node.Memory.Free = node.Memory.Total - node.Memory.Used
	}

	// Update VM metrics
	for i := range data.VMs {
		vm := &data.VMs[i]
		if vm.Status != "running" {
			continue
		}

		// Random walk for CPU with mean reversion toward 15%
		cpuChange := (rand.Float64() - 0.5) * 0.04
		cpuMeanReversion := (0.15 - vm.CPU) * 0.08
		vm.CPU += cpuChange + cpuMeanReversion
		vm.CPU = math.Max(0.01, math.Min(0.85, vm.CPU))

		// Update memory with smaller fluctuations and mean reversion toward 50%
		memChange := (rand.Float64() - 0.5) * 0.03 // 3% swing
		memMeanReversion := (50.0 - vm.Memory.Usage) * 0.04
		vm.Memory.Usage += (memChange * 100) + memMeanReversion
		vm.Memory.Usage = math.Max(10, math.Min(85, vm.Memory.Usage))
		vm.Memory.Used = int64(float64(vm.Memory.Total) * (vm.Memory.Usage / 100))
		vm.Memory.Free = vm.Memory.Total - vm.Memory.Used

		// Update disk usage very slowly (disks fill up gradually)
		if rand.Float64() < 0.05 { // 5% chance to change disk usage
			diskChange := (rand.Float64() - 0.48) * 0.3 // Very slight bias toward filling
			vm.Disk.Usage += diskChange
			vm.Disk.Usage = math.Max(10, math.Min(90, vm.Disk.Usage))
			vm.Disk.Used = int64(float64(vm.Disk.Total) * (vm.Disk.Usage / 100))
			vm.Disk.Free = vm.Disk.Total - vm.Disk.Used
		}

		// Update network/disk I/O with small chance of changing
		if rand.Float64() < 0.15 { // 15% chance of I/O change
			vm.NetworkIn = generateRealisticIO("network-in")
			vm.NetworkOut = generateRealisticIO("network-out")
			vm.DiskRead = generateRealisticIO("disk-read")
			vm.DiskWrite = generateRealisticIO("disk-write")
		}

		// Update uptime
		vm.Uptime += 2 // Add 2 seconds per update
	}

	// Update container metrics
	for i := range data.Containers {
		ct := &data.Containers[i]
		if ct.Status != "running" {
			continue
		}

		// Random walk for CPU with mean reversion toward 8% (containers typically lower)
		cpuChange := (rand.Float64() - 0.5) * 0.02
		cpuMeanReversion := (0.08 - ct.CPU) * 0.10
		ct.CPU += cpuChange + cpuMeanReversion
		ct.CPU = math.Max(0.01, math.Min(0.75, ct.CPU))

		// Update memory with smaller fluctuations and mean reversion toward 55%
		memChange := (rand.Float64() - 0.5) * 0.02 // 2% swing
		memMeanReversion := (55.0 - ct.Memory.Usage) * 0.04
		ct.Memory.Usage += (memChange * 100) + memMeanReversion
		ct.Memory.Usage = math.Max(5, math.Min(85, ct.Memory.Usage))
		ct.Memory.Used = int64(float64(ct.Memory.Total) * (ct.Memory.Usage / 100))
		ct.Memory.Free = ct.Memory.Total - ct.Memory.Used

		// Update disk usage very slowly
		if rand.Float64() < 0.03 { // 3% chance (containers change disk less)
			diskChange := (rand.Float64() - 0.48) * 0.2 // Very slight bias toward filling
			ct.Disk.Usage += diskChange
			ct.Disk.Usage = math.Max(5, math.Min(85, ct.Disk.Usage))
			ct.Disk.Used = int64(float64(ct.Disk.Total) * (ct.Disk.Usage / 100))
			ct.Disk.Free = ct.Disk.Total - ct.Disk.Used
		}

		// Update network/disk I/O with small chance of changing
		if rand.Float64() < 0.10 { // 10% chance of I/O change (containers change less often)
			ct.NetworkIn = generateRealisticIO("network-in-ct")
			ct.NetworkOut = generateRealisticIO("network-out-ct")
			ct.DiskRead = generateRealisticIO("disk-read-ct")
			ct.DiskWrite = generateRealisticIO("disk-write-ct")
		}

		// Update uptime
		ct.Uptime += 2
	}

	// Update disk metrics occasionally
	for i := range data.PhysicalDisks {
		disk := &data.PhysicalDisks[i]

		// Occasionally change temperature
		if rand.Float64() < 0.1 {
			disk.Temperature += rand.Intn(5) - 2
			if disk.Temperature < 30 {
				disk.Temperature = 30
			}
			if disk.Temperature > 85 {
				disk.Temperature = 85
			}
		}

		// Occasionally degrade SSD life
		if disk.Wearout > 0 && rand.Float64() < 0.01 {
			disk.Wearout = disk.Wearout - 1
			if disk.Wearout < 0 {
				disk.Wearout = 0
			}
		}

		disk.LastChecked = time.Now()
	}

	data.LastUpdate = time.Now()
}

func generateDisksForNode(node models.Node) []models.PhysicalDisk {
	disks := []models.PhysicalDisk{}

	// Generate 1-3 disks per node
	diskCount := rand.Intn(3) + 1

	diskModels := []struct {
		model    string
		diskType string
		size     int64
	}{
		{"Samsung SSD 970 EVO Plus 1TB", "nvme", 1000204886016},
		{"WD Blue SN570 500GB", "nvme", 500107862016},
		{"Crucial MX500 2TB", "sata", 2000398934016},
		{"Seagate BarraCuda 4TB", "sata", 4000787030016},
		{"Kingston NV2 250GB", "nvme", 250059350016},
		{"WD Red Pro 8TB", "sata", 8001563222016},
		{"Samsung 980 PRO 2TB", "nvme", 2000398934016},
		{"Intel SSD 660p 1TB", "nvme", 1000204886016},
		{"Toshiba X300 6TB", "sata", 6001175126016},
	}

	for i := 0; i < diskCount; i++ {
		diskModel := diskModels[rand.Intn(len(diskModels))]

		// Generate health status - most are healthy
		health := "PASSED"
		if rand.Float64() < 0.05 { // 5% chance of failure
			health = "FAILED"
		} else if rand.Float64() < 0.1 { // 10% chance of unknown
			health = "UNKNOWN"
		}

		// Generate wearout for SSDs (percentage life remaining; lower numbers mean heavy wear)
		wearout := 0
		if diskModel.diskType == "nvme" || diskModel.diskType == "sata" {
			if rand.Float64() < 0.7 { // 70% chance it's an SSD with wearout data
				wearout = rand.Intn(50) + 50 // 50-100% life remaining
				if rand.Float64() < 0.1 {    // 10% chance of low life
					wearout = rand.Intn(15) + 5 // 5-20% life remaining
				}
			}
		}
		if wearout < 0 {
			wearout = 0
		}
		if wearout > 100 {
			wearout = 100
		}

		// Generate temperature
		temp := rand.Intn(20) + 35 // 35-55°C normal range
		if rand.Float64() < 0.1 {  // 10% chance of high temp
			temp = rand.Intn(15) + 65 // 65-80°C hot
		}

		disk := models.PhysicalDisk{
			ID:          fmt.Sprintf("%s-%s-/dev/%s%d", node.Instance, node.Name, []string{"nvme", "sd"}[i%2], i),
			Node:        node.Name,
			Instance:    node.Instance,
			DevPath:     fmt.Sprintf("/dev/%s%d", []string{"nvme", "sd"}[i%2], i),
			Model:       diskModel.model,
			Serial:      fmt.Sprintf("SERIAL%d%d%d", rand.Intn(9999), rand.Intn(9999), rand.Intn(9999)),
			Type:        diskModel.diskType,
			Size:        diskModel.size,
			Health:      health,
			Wearout:     wearout,
			Temperature: temp,
			Used:        []string{"ext4", "zfs", "btrfs", "xfs"}[rand.Intn(4)],
			LastChecked: time.Now(),
		}

		disks = append(disks, disk)
	}

	return disks
}
