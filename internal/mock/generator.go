package mock

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// titleCase capitalizes the first letter of each word (simple ASCII-safe version)
func titleCase(s string) string {
	var result strings.Builder
	capitalizeNext := true
	for _, r := range s {
		if unicode.IsSpace(r) || r == '-' {
			capitalizeNext = true
			if r == '-' {
				result.WriteRune(' ')
			} else {
				result.WriteRune(r)
			}
		} else if capitalizeNext {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			result.WriteRune(unicode.ToLower(r))
		}
	}
	return result.String()
}

type MockConfig struct {
	NodeCount                int
	VMsPerNode               int
	LXCsPerNode              int
	DockerHostCount          int
	DockerContainersPerHost  int
	GenericHostCount         int
	K8sClusterCount          int
	K8sNodesPerCluster       int
	K8sPodsPerCluster        int
	K8sDeploymentsPerCluster int
	RandomMetrics            bool
	HighLoadNodes            []string // Specific nodes to simulate high load
	StoppedPercent           float64  // Percentage of guests that should be stopped
}

const (
	dockerConnectionPrefix     = "docker-"
	kubernetesConnectionPrefix = "kubernetes-"
	hostConnectionPrefix       = "host-"
)

var DefaultConfig = MockConfig{
	NodeCount:                7, // Test the 5-9 node range by default
	VMsPerNode:               5,
	LXCsPerNode:              8,
	DockerHostCount:          3,
	DockerContainersPerHost:  12,
	GenericHostCount:         4,
	K8sClusterCount:          2,
	K8sNodesPerCluster:       4,
	K8sPodsPerCluster:        30,
	K8sDeploymentsPerCluster: 12,
	RandomMetrics:            true,
	StoppedPercent:           0.2,
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

var dockerHostPrefixes = []string{
	"nebula", "orion", "aurora", "atlas", "zephyr",
	"draco", "phoenix", "hydra", "pegasus", "lyra",
}

type podDefinition struct {
	Name              string
	ID                string
	ComposeProject    string
	ComposeWorkdir    string
	ComposeConfigHash string
	AutoUpdatePolicy  string
	AutoUpdateRestart string
	UserNamespace     string
}

var dockerOperatingSystems = []string{
	"Debian GNU/Linux 12 (bookworm)",
	"Ubuntu 24.04.1 LTS",
	"Ubuntu 22.04.4 LTS",
	"Fedora Linux 40",
	"Alpine Linux 3.19",
}

var dockerKernelVersions = []string{
	"6.8.12-1-amd64",
	"6.6.32-1-lts",
	"6.5.0-27-generic",
	"6.1.55-1-arm64",
}

var dockerArchitectures = []string{"x86_64", "aarch64"}

var dockerVersions = []string{
	"27.3.1",
	"26.1.3",
	"25.0.6",
}

var podmanVersions = []string{
	"5.0.2",
	"4.9.3",
	"4.8.1",
	"4.7.2",
}

var dockerAgentVersions = []string{
	"0.1.0",
	"0.1.0-dev",
}

var dockerImageTags = []string{
	"1.0.0",
	"1.1.2",
	"2025.09.1",
	"2025.10.4",
	"latest",
	"stable",
}

var genericHostProfiles = []struct {
	Platform     string
	OSName       string
	OSVersion    string
	Kernel       string
	Architecture string
}{
	{"linux", "Debian GNU/Linux", "12 (bookworm)", "6.8.12-1-amd64", "x86_64"},
	{"linux", "Ubuntu Server", "24.04 LTS", "6.8.0-31-generic", "x86_64"},
	{"linux", "Rocky Linux", "9.3", "5.14.0-427.22.1.el9_4.x86_64", "x86_64"},
	{"linux", "Alpine Linux", "3.20.1", "6.6.32-0-lts", "x86_64"},
	{"windows", "Windows Server", "2022 Datacenter", "10.0.20348.2244", "x86_64"},
	{"windows", "Windows 11 Pro", "23H2", "10.0.22631.3737", "x86_64"},
	{"macos", "macOS Ventura", "13.6.8", "22.6.0", "arm64"},
	{"macos", "macOS Sonoma", "14.6.1", "23G93", "arm64"},
}

var genericHostPrefixes = []string{
	"apollo", "centauri", "ceres", "europa", "hyperion",
	"kepler", "meridian", "orion", "polaris", "spectrum",
	"vega", "zenith", "halcyon", "icarus", "rigel",
}

var hostAgentVersions = []string{
	"0.1.0",
	"0.1.1",
	"0.2.0-alpha",
}

var k8sClusterNames = []string{
	"production",
	"staging",
	"development",
	"edge",
	"internal",
	"platform",
}

var k8sNamespaces = []string{
	"default",
	"kube-system",
	"monitoring",
	"logging",
	"ingress-nginx",
	"cert-manager",
	"argocd",
	"apps",
	"services",
	"databases",
	"cache",
}

var k8sPodPrefixes = []string{
	"nginx",
	"redis",
	"postgres",
	"mysql",
	"mongodb",
	"prometheus",
	"grafana",
	"loki",
	"jaeger",
	"api",
	"auth",
	"worker",
	"cron",
	"coredns",
	"metrics-server",
	"cert-manager",
	"ingress-controller",
	"fluentd",
}

var k8sVersions = []string{
	"v1.31.2",
	"v1.30.4",
	"v1.29.8",
}

var k8sImages = []string{
	"nginx:1.27",
	"redis:7.4",
	"postgres:16",
	"mysql:8.4",
	"mongo:7.0",
	"prom/prometheus:v2.54",
	"grafana/grafana:11.3",
	"grafana/loki:3.0",
	"busybox:1.36",
	"alpine:3.20",
}

var k8sNodeOS = []string{
	"Ubuntu 24.04.1 LTS",
	"Ubuntu 22.04.5 LTS",
	"Debian GNU/Linux 12 (bookworm)",
	"Fedora CoreOS 40",
	"Talos Linux v1.8",
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

// GenerateMockData creates a simulated state snapshot for demo/test environments.
func GenerateMockData(config MockConfig) models.StateSnapshot {
	// rand is automatically seeded in Go 1.20+

	data := models.StateSnapshot{
		Nodes:                     generateNodes(config),
		DockerHosts:               generateDockerHosts(config),
		KubernetesClusters:        generateKubernetesClusters(config),
		RemovedKubernetesClusters: []models.RemovedKubernetesCluster{},
		Hosts:                     generateHosts(config),
		VMs:                       []models.VM{},
		Containers:                []models.Container{},
		PhysicalDisks:             []models.PhysicalDisk{},
		ReplicationJobs:           []models.ReplicationJob{},
		LastUpdate:                time.Now(),
		ConnectionHealth:          make(map[string]bool),
		Stats:                     models.Stats{},
		ActiveAlerts:              []models.Alert{},
	}

	ensureMockNodeHostLinks(&data)
	ensureMockKubernetesNodeHostLinks(&data)

	// Generate physical disks for each node
	for _, node := range data.Nodes {
		data.PhysicalDisks = append(data.PhysicalDisks, generateDisksForNode(node)...)
	}

	for _, host := range data.DockerHosts {
		data.ConnectionHealth[dockerConnectionPrefix+host.ID] = host.Status != "offline"
	}

	for _, cluster := range data.KubernetesClusters {
		data.ConnectionHealth[kubernetesConnectionPrefix+cluster.ID] = cluster.Status != "offline"
	}

	for _, host := range data.Hosts {
		data.ConnectionHealth[hostConnectionPrefix+host.ID] = host.Status != "offline"
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
		var vmCount, lxcCount int

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

		if node.Status == "offline" {
			// Create placeholder offline guests with zeroed metrics
			if vmCount == 0 {
				vmCount = 1
			}
			for i := 0; i < vmCount; i++ {
				vm := generateVM(node.Name, node.Instance, vmidCounter, config)
				vm.Status = "stopped"
				vm.CPU = 0
				vm.Memory.Used = 0
				vm.Memory.Usage = 0
				vm.Memory.Free = vm.Memory.Total
				vm.Memory.SwapUsed = 0
				vm.Disk.Used = 0
				vm.Disk.Free = vm.Disk.Total
				vm.Disk.Usage = -1
				vm.NetworkIn = 0
				vm.NetworkOut = 0
				vm.DiskRead = 0
				vm.DiskWrite = 0
				vm.Uptime = 0
				data.VMs = append(data.VMs, vm)
				vmidCounter++
			}

			if lxcCount == 0 {
				lxcCount = 1
			}
			for i := 0; i < lxcCount; i++ {
				lxc := generateContainer(node.Name, node.Instance, vmidCounter, config)
				lxc.Status = "stopped"
				lxc.CPU = 0
				lxc.Memory.Used = 0
				lxc.Memory.Usage = 0
				lxc.Memory.Free = lxc.Memory.Total
				lxc.Memory.SwapUsed = 0
				lxc.Disk.Used = 0
				lxc.Disk.Free = lxc.Disk.Total
				lxc.Disk.Usage = -1
				lxc.NetworkIn = 0
				lxc.NetworkOut = 0
				lxc.DiskRead = 0
				lxc.DiskWrite = 0
				lxc.Uptime = 0
				data.Containers = append(data.Containers, lxc)
				vmidCounter++
			}
			continue
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

	// Generate Ceph cluster data if Ceph-backed storage is present
	data.CephClusters = generateCephClusters(data.Nodes, data.Storage)

	// Generate PBS instances and backups
	data.PBSInstances = generatePBSInstances()
	data.PBSBackups = generatePBSBackups(data.VMs, data.Containers)

	// Set PBS connection health
	for _, pbs := range data.PBSInstances {
		data.ConnectionHealth[fmt.Sprintf("pbs-%s", pbs.Name)] = true
	}

	// Generate PMG instances and mail data
	data.PMGInstances = generatePMGInstances()
	for _, pmg := range data.PMGInstances {
		data.ConnectionHealth[fmt.Sprintf("pmg-%s", pmg.Name)] = true
	}

	// Generate backups for VMs and containers
	data.PVEBackups = models.PVEBackups{
		BackupTasks:    []models.BackupTask{},
		StorageBackups: generateBackups(data.VMs, data.Containers),
		GuestSnapshots: generateSnapshots(data.VMs, data.Containers),
	}
	data.PMGBackups = extractPMGBackups(data.PVEBackups.StorageBackups)
	data.Backups = models.Backups{
		PVE: data.PVEBackups,
		PBS: append([]models.PBSBackup(nil), data.PBSBackups...),
		PMG: append([]models.PMGBackup(nil), data.PMGBackups...),
	}

	data.ReplicationJobs = generateReplicationJobs(data.Nodes, data.VMs)

	// Calculate stats
	data.Stats.StartTime = time.Now()
	data.Stats.Uptime = 0
	data.Stats.Version = "v4.9.0-mock"

	return data
}

func generateReplicationJobs(nodes []models.Node, vms []models.VM) []models.ReplicationJob {
	if len(nodes) == 0 || len(vms) == 0 {
		return []models.ReplicationJob{}
	}

	maxJobs := len(vms)
	if maxJobs > 8 {
		maxJobs = 8
	}

	jobs := make([]models.ReplicationJob, 0, maxJobs)
	now := time.Now()
	nodeCount := len(nodes)

	for i := 0; i < maxJobs; i++ {
		vm := vms[i%len(vms)]
		instance := vm.Instance
		if instance == "" {
			instance = vm.Node
		}

		jobNumber := i % 3
		jobID := fmt.Sprintf("%d-%d", vm.VMID, jobNumber)
		lastSync := now.Add(-time.Duration(300+rand.Intn(3600)) * time.Second)
		nextSync := lastSync.Add(15 * time.Minute)
		durationSeconds := 90 + rand.Intn(240)
		durationHuman := formatSecondsAsClock(durationSeconds)
		status := "idle"
		lastStatus := "ok"
		errorMessage := ""
		failCount := 0

		roll := rand.Float64()
		if roll < 0.1 {
			status = "error"
			lastStatus = "error"
			errorMessage = "last sync timed out"
			failCount = 1 + rand.Intn(2)
		} else if roll < 0.35 {
			status = "syncing"
		}

		targetNode := nodes[(i+1)%nodeCount].Name
		rate := 80.0 + rand.Float64()*140.0

		job := models.ReplicationJob{
			ID:                      fmt.Sprintf("%s-%s", instance, jobID),
			Instance:                instance,
			JobID:                   jobID,
			JobNumber:               jobNumber,
			Guest:                   fmt.Sprintf("%d", vm.VMID),
			GuestID:                 vm.VMID,
			GuestName:               vm.Name,
			GuestType:               vm.Type,
			GuestNode:               vm.Node,
			SourceNode:              vm.Node,
			SourceStorage:           "local-zfs",
			TargetNode:              targetNode,
			TargetStorage:           "replica-zfs",
			Schedule:                "*/15",
			Type:                    "local",
			Enabled:                 true,
			State:                   status,
			Status:                  status,
			LastSyncStatus:          lastStatus,
			LastSyncTime:            ptrTime(lastSync),
			LastSyncUnix:            lastSync.Unix(),
			LastSyncDurationSeconds: durationSeconds,
			LastSyncDurationHuman:   durationHuman,
			NextSyncTime:            ptrTime(nextSync),
			NextSyncUnix:            nextSync.Unix(),
			DurationSeconds:         durationSeconds,
			DurationHuman:           durationHuman,
			FailCount:               failCount,
			Error:                   errorMessage,
			RateLimitMbps:           ptrFloat64(rate),
			LastPolled:              now,
		}

		jobs = append(jobs, job)
	}

	return jobs
}

func formatSecondsAsClock(totalSeconds int) string {
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func ptrFloat64(v float64) *float64 {
	return &v
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
		node.DisplayName = fmt.Sprintf("%s (%s)", node.Instance, nodeName)
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
			node.Memory.Free = node.Memory.Total
			node.Uptime = 0
			node.LoadAverage = []float64{0, 0, 0}
			node.Disk.Used = 0
			node.Disk.Usage = 0
			node.Disk.Free = node.Disk.Total
			if node.Temperature != nil {
				node.Temperature = &models.Temperature{Available: false}
			}
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
		node.DisplayName = node.Instance
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
		DisplayName:   name,
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
	// Keep missing temperatures uncommon in mock mode so Infrastructure has broad coverage.
	if rand.Float64() < 0.1 {
		return &models.Temperature{
			Available: false,
			HasCPU:    false,
			HasNVMe:   false,
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
		HasCPU:     true,
		HasNVMe:    len(nvmeTemps) > 0,
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
		balloon := int64(0)
		if rand.Float64() < 0.2 {
			// Simulate ballooning active (10-30% of total memory)
			balloon = int64(float64(totalMem) * (0.1 + rand.Float64()*0.2))
			// Ensure balloon doesn't exceed used memory (which would result in 0 active)
			if balloon > usedMem {
				balloon = int64(float64(usedMem) * 0.8)
			}
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

	vm := models.VM{
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
		LastBackup:        generateLastBackupTime(),
	}

	if status != "running" {
		vm.CPU = 0
		vm.Memory.Usage = 0
		vm.Memory.SwapUsed = 0
		vm.Memory.Used = 0
		vm.Memory.Free = vm.Memory.Total
		vm.Disk.Used = 0
		vm.Disk.Free = vm.Disk.Total
		vm.Disk.Usage = -1
		vm.NetworkIn = 0
		vm.NetworkOut = 0
		vm.DiskRead = 0
		vm.DiskWrite = 0
		vm.Uptime = 0
	}

	return vm
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

		rxBytes := rand.Int63n(8*1024*1024*1024) + rand.Int63n(256*1024*1024)
		txBytes := rand.Int63n(6*1024*1024*1024) + rand.Int63n(256*1024*1024)

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

func generateKubernetesClusters(config MockConfig) []models.KubernetesCluster {
	clusterCount := config.K8sClusterCount
	if clusterCount <= 0 {
		return []models.KubernetesCluster{}
	}

	nodeCount := config.K8sNodesPerCluster
	if nodeCount <= 0 {
		nodeCount = 4
	}

	podCount := config.K8sPodsPerCluster
	if podCount < 0 {
		podCount = 0
	}

	deploymentCount := config.K8sDeploymentsPerCluster
	if deploymentCount < 0 {
		deploymentCount = 0
	}

	now := time.Now()
	clusters := make([]models.KubernetesCluster, 0, clusterCount)

	for i := 0; i < clusterCount; i++ {
		name := k8sClusterNames[i%len(k8sClusterNames)]
		clusterID := fmt.Sprintf("k8s-%s-%d", strings.ToLower(name), i+1)
		server := fmt.Sprintf("https://%s.k8s.local:6443", strings.ToLower(name))
		context := fmt.Sprintf("%s-context", strings.ToLower(name))

		nodes := generateKubernetesNodes(clusterID, nodeCount)
		pods := generateKubernetesPods(clusterID, nodes, podCount)
		deployments := generateKubernetesDeployments(clusterID, deploymentCount)

		lastSeen := now.Add(-time.Duration(rand.Intn(20)) * time.Second)
		status := "online"

		// Make the last cluster offline occasionally for UI coverage.
		if clusterCount > 1 && i == clusterCount-1 && rand.Float64() < 0.55 {
			status = "offline"
			lastSeen = now.Add(-time.Duration(5+rand.Intn(20)) * time.Minute)
			for nodeIdx := range nodes {
				nodes[nodeIdx].Ready = false
			}
		} else if clusterHasIssues(nodes, pods, deployments) {
			status = "degraded"
		}

		cluster := models.KubernetesCluster{
			ID:               clusterID,
			AgentID:          fmt.Sprintf("%s-agent", clusterID),
			Name:             name,
			DisplayName:      titleCase(name),
			Server:           server,
			Context:          context,
			Version:          k8sVersions[rand.Intn(len(k8sVersions))],
			Status:           status,
			LastSeen:         lastSeen,
			IntervalSeconds:  30,
			AgentVersion:     "0.1.0-mock",
			Nodes:            nodes,
			Pods:             pods,
			Deployments:      deployments,
			Hidden:           false,
			PendingUninstall: false,
		}
		initializeMockKubernetesClusterUsage(&cluster, now, true)
		clusters = append(clusters, cluster)
	}

	return clusters
}

func initializeMockKubernetesClusterUsage(cluster *models.KubernetesCluster, now time.Time, randomize bool) {
	if cluster == nil {
		return
	}

	reconcileMockKubernetesPodScheduling(cluster, randomize)

	nodeByName := make(map[string]*models.KubernetesNode, len(cluster.Nodes))
	for i := range cluster.Nodes {
		node := &cluster.Nodes[i]
		normalizeMockKubernetesNodeCapacity(node)
		name := strings.TrimSpace(node.Name)
		if name != "" {
			nodeByName[name] = node
		}
	}

	clusterOffline := strings.EqualFold(strings.TrimSpace(cluster.Status), "offline")
	for i := range cluster.Pods {
		pod := &cluster.Pods[i]
		node := nodeByName[strings.TrimSpace(pod.NodeName)]
		updateMockKubernetesPodUsage(pod, node, now, randomize, clusterOffline)
	}

	recomputeMockKubernetesNodeUsage(cluster, now)
}

func reconcileMockKubernetesPodScheduling(cluster *models.KubernetesCluster, randomize bool) {
	if cluster == nil {
		return
	}
	clusterOffline := strings.EqualFold(strings.TrimSpace(cluster.Status), "offline")

	if clusterOffline {
		for i := range cluster.Nodes {
			cluster.Nodes[i].Ready = false
		}
		for i := range cluster.Pods {
			pod := &cluster.Pods[i]
			if strings.EqualFold(strings.TrimSpace(pod.Phase), "running") {
				pod.Phase = "Unknown"
				pod.Reason = "ClusterOffline"
				pod.Message = "Cluster telemetry temporarily unavailable"
				for j := range pod.Containers {
					if strings.EqualFold(strings.TrimSpace(pod.Containers[j].State), "terminated") {
						continue
					}
					pod.Containers[j].Ready = false
					pod.Containers[j].State = "unknown"
					pod.Containers[j].Reason = "ClusterOffline"
					pod.Containers[j].Message = "Cluster telemetry temporarily unavailable"
				}
			}
		}
		return
	}

	readyNodes := make([]string, 0, len(cluster.Nodes))
	readyLookup := make(map[string]struct{}, len(cluster.Nodes))
	for _, node := range cluster.Nodes {
		name := strings.TrimSpace(node.Name)
		if name == "" {
			continue
		}
		if node.Ready && !node.Unschedulable {
			readyNodes = append(readyNodes, name)
			readyLookup[name] = struct{}{}
		}
	}

	for i := range cluster.Pods {
		pod := &cluster.Pods[i]
		phase := strings.ToLower(strings.TrimSpace(pod.Phase))
		nodeName := strings.TrimSpace(pod.NodeName)
		_, nodeReady := readyLookup[nodeName]

		switch phase {
		case "running":
			if nodeReady {
				continue
			}
			if len(readyNodes) == 0 {
				pod.Phase = "Pending"
				pod.Reason = "Unschedulable"
				pod.Message = "No ready nodes available"
				pod.StartTime = nil
				continue
			}
			if randomize && rand.Float64() < 0.25 {
				pod.Phase = "Pending"
				pod.Reason = "NodeNotReady"
				pod.Message = "Waiting for node recovery"
				pod.StartTime = nil
				continue
			}
			assignIndex := int(mockStableHash64(strings.TrimSpace(pod.UID), pod.Name, "ready-node") % uint64(len(readyNodes)))
			pod.NodeName = readyNodes[assignIndex]
			pod.Reason = ""
			pod.Message = ""
			if pod.StartTime == nil {
				start := time.Now().Add(-time.Duration(30+rand.Intn(240)) * time.Second)
				pod.StartTime = &start
			}
		case "pending", "unknown":
			if len(readyNodes) == 0 {
				continue
			}
			if randomize && rand.Float64() >= 0.22 {
				continue
			}
			assignIndex := int(mockStableHash64(strings.TrimSpace(pod.UID), pod.Name, "recover-node") % uint64(len(readyNodes)))
			pod.NodeName = readyNodes[assignIndex]
			pod.Phase = "Running"
			pod.Reason = ""
			pod.Message = ""
			start := time.Now().Add(-time.Duration(20+rand.Intn(180)) * time.Second)
			pod.StartTime = &start
			for j := range pod.Containers {
				if strings.EqualFold(pod.Containers[j].State, "terminated") {
					continue
				}
				pod.Containers[j].State = "running"
				pod.Containers[j].Reason = ""
				pod.Containers[j].Message = ""
				pod.Containers[j].Ready = true
			}
		}
	}
}

func normalizeMockKubernetesNodeCapacity(node *models.KubernetesNode) {
	if node == nil {
		return
	}
	if node.CapacityCPU <= 0 {
		node.CapacityCPU = 4
	}
	if node.AllocCPU <= 0 || node.AllocCPU > node.CapacityCPU {
		node.AllocCPU = node.CapacityCPU
	}
	if node.CapacityMemoryBytes <= 0 {
		node.CapacityMemoryBytes = int64(16) * 1024 * 1024 * 1024
	}
	if node.AllocMemoryBytes <= 0 || node.AllocMemoryBytes > node.CapacityMemoryBytes {
		node.AllocMemoryBytes = node.CapacityMemoryBytes
	}
	if node.CapacityPods <= 0 {
		node.CapacityPods = 110
	}
	if node.AllocPods <= 0 || node.AllocPods > node.CapacityPods {
		node.AllocPods = node.CapacityPods
	}
}

func updateMockKubernetesPodUsage(
	pod *models.KubernetesPod,
	node *models.KubernetesNode,
	now time.Time,
	randomize bool,
	clusterOffline bool,
) {
	if pod == nil {
		return
	}
	if clusterOffline || !mockKubernetesPodIsActive(pod, node) {
		applyMockKubernetesPodZeroUsage(pod)
		return
	}

	allocCPU := int64(2)
	allocMemory := int64(8) * 1024 * 1024 * 1024
	if node != nil {
		if node.AllocCPU > 0 {
			allocCPU = node.AllocCPU
		} else if node.CapacityCPU > 0 {
			allocCPU = node.CapacityCPU
		}
		if node.AllocMemoryBytes > 0 {
			allocMemory = node.AllocMemoryBytes
		} else if node.CapacityMemoryBytes > 0 {
			allocMemory = node.CapacityMemoryBytes
		}
	}
	if allocCPU <= 0 {
		allocCPU = 2
	}
	if allocMemory <= 0 {
		allocMemory = int64(8) * 1024 * 1024 * 1024
	}

	seedID := strings.TrimSpace(pod.UID)
	if seedID == "" {
		seedID = strings.TrimSpace(pod.Namespace) + "/" + strings.TrimSpace(pod.Name)
	}

	ownerKind := strings.ToLower(strings.TrimSpace(pod.OwnerKind))
	ownerScale := 1.0
	switch ownerKind {
	case "statefulset":
		ownerScale = 1.28
	case "daemonset":
		ownerScale = 0.82
	case "job":
		ownerScale = 0.74
	}
	switch strings.ToLower(strings.TrimSpace(pod.QoSClass)) {
	case "guaranteed":
		ownerScale *= 1.12
	case "besteffort":
		ownerScale *= 0.78
	}

	readyContainers := 0
	for _, container := range pod.Containers {
		if container.Ready {
			readyContainers++
		}
	}
	readyRatio := 1.0
	if len(pod.Containers) > 0 {
		readyRatio = float64(readyContainers) / float64(len(pod.Containers))
		if readyRatio < 0.3 {
			readyRatio = 0.3
		}
	}

	activity := mockKubernetesActivity(seedID, now, 5*60, randomize)
	activity *= 0.65 + (readyRatio * 0.45)
	activity = clampFloat(activity, 0.05, 0.98)

	baseCPU := 1.5 + float64(mockStableHash64(seedID, "cpu-base")%10)
	burstCPU := 18.0 + float64(mockStableHash64(seedID, "cpu-burst")%52)
	targetCPU := (baseCPU + activity*burstCPU) * ownerScale
	pod.UsageCPUPercent = clampFloat(smoothMetricToward(pod.UsageCPUPercent, targetCPU, 0.34), 0.1, 96)

	cpuMilli := int((pod.UsageCPUPercent / 100.0) * float64(allocCPU*1000))
	if cpuMilli < 1 {
		cpuMilli = 1
	}
	pod.UsageCPUMilliCores = cpuMilli

	baseMemory := 9.0 + float64(mockStableHash64(seedID, "mem-base")%20)
	burstMemory := 14.0 + float64(mockStableHash64(seedID, "mem-burst")%36)
	targetMemory := (baseMemory + activity*burstMemory) * (0.92 + ownerScale*0.12)
	pod.UsageMemoryPercent = clampFloat(smoothMetricToward(pod.UsageMemoryPercent, targetMemory, 0.22), 3, 97)
	pod.UsageMemoryBytes = int64(float64(allocMemory) * (pod.UsageMemoryPercent / 100.0))

	baseNetIn := float64((24 + int(mockStableHash64(seedID, "netin-base")%180)) * 1024)
	burstNetIn := float64((128 + int(mockStableHash64(seedID, "netin-burst")%4096)) * 1024)
	baseNetOut := float64((20 + int(mockStableHash64(seedID, "netout-base")%160)) * 1024)
	burstNetOut := float64((96 + int(mockStableHash64(seedID, "netout-burst")%3072)) * 1024)
	if ownerKind == "statefulset" {
		baseNetOut *= 0.75
		burstNetOut *= 0.75
	}

	targetNetIn := (baseNetIn + activity*burstNetIn) * (0.85 + ownerScale*0.25)
	targetNetOut := (baseNetOut + activity*burstNetOut) * (0.8 + ownerScale*0.2)
	pod.NetInRate = clampFloat(smoothMetricToward(pod.NetInRate, targetNetIn, 0.36), 4*1024, 180*1024*1024)
	pod.NetOutRate = clampFloat(smoothMetricToward(pod.NetOutRate, targetNetOut, 0.36), 4*1024, 150*1024*1024)

	seconds := updateInterval.Seconds()
	if seconds <= 0 {
		seconds = 2
	}
	if pod.NetworkRxBytes <= 0 {
		seedSeconds := int64(180 + (mockStableHash64(seedID, "rx-seed") % 1800))
		pod.NetworkRxBytes = int64(pod.NetInRate * float64(seedSeconds))
	} else {
		pod.NetworkRxBytes += int64(pod.NetInRate * seconds)
	}
	if pod.NetworkTxBytes <= 0 {
		seedSeconds := int64(180 + (mockStableHash64(seedID, "tx-seed") % 1800))
		pod.NetworkTxBytes = int64(pod.NetOutRate * float64(seedSeconds))
	} else {
		pod.NetworkTxBytes += int64(pod.NetOutRate * seconds)
	}

	capacityGiB := int64(8 + (mockStableHash64(seedID, "ephemeral-cap") % 180))
	if ownerKind == "statefulset" {
		capacityGiB += 64
	}
	if ownerKind == "daemonset" {
		capacityGiB += 16
	}
	if capacityGiB < 4 {
		capacityGiB = 4
	}
	capacityBytes := capacityGiB * 1024 * 1024 * 1024
	if pod.EphemeralStorageCapacityBytes > 0 && pod.EphemeralStorageCapacityBytes > capacityBytes/2 {
		capacityBytes = pod.EphemeralStorageCapacityBytes
	}
	pod.EphemeralStorageCapacityBytes = capacityBytes

	baseDisk := 8.0 + float64(mockStableHash64(seedID, "disk-base")%18)
	burstDisk := 12.0 + float64(mockStableHash64(seedID, "disk-burst")%40)
	targetDisk := (baseDisk + activity*burstDisk) * (0.95 + ownerScale*0.08)
	if ownerKind == "statefulset" {
		targetDisk += 8
	}
	pod.DiskUsagePercent = clampFloat(smoothMetricToward(pod.DiskUsagePercent, targetDisk, 0.18), 1, 95)
	pod.EphemeralStorageUsedBytes = int64(float64(capacityBytes) * (pod.DiskUsagePercent / 100.0))
}

func mockKubernetesPodIsActive(pod *models.KubernetesPod, node *models.KubernetesNode) bool {
	if pod == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(pod.Phase), "running") {
		return false
	}
	if strings.TrimSpace(pod.NodeName) == "" {
		return false
	}
	if node != nil && !node.Ready {
		return false
	}
	return true
}

func applyMockKubernetesPodZeroUsage(pod *models.KubernetesPod) {
	if pod == nil {
		return
	}
	pod.UsageCPUMilliCores = 0
	pod.UsageMemoryBytes = 0
	pod.UsageCPUPercent = 0
	pod.UsageMemoryPercent = 0
	pod.NetInRate = 0
	pod.NetOutRate = 0
	pod.DiskUsagePercent = 0
	pod.EphemeralStorageUsedBytes = 0
}

func recomputeMockKubernetesNodeUsage(cluster *models.KubernetesCluster, now time.Time) {
	if cluster == nil {
		return
	}

	type aggregate struct {
		cpuMilli int64
		memBytes int64
	}

	podTotals := make(map[string]aggregate, len(cluster.Nodes))
	for _, pod := range cluster.Pods {
		if !strings.EqualFold(strings.TrimSpace(pod.Phase), "running") {
			continue
		}
		nodeName := strings.TrimSpace(pod.NodeName)
		if nodeName == "" {
			continue
		}
		sum := podTotals[nodeName]
		if pod.UsageCPUMilliCores > 0 {
			sum.cpuMilli += int64(pod.UsageCPUMilliCores)
		}
		if pod.UsageMemoryBytes > 0 {
			sum.memBytes += pod.UsageMemoryBytes
		}
		podTotals[nodeName] = sum
	}

	clusterOffline := strings.EqualFold(strings.TrimSpace(cluster.Status), "offline")
	for i := range cluster.Nodes {
		node := &cluster.Nodes[i]
		normalizeMockKubernetesNodeCapacity(node)
		if clusterOffline || !node.Ready {
			node.UsageCPUMilliCores = 0
			node.UsageMemoryBytes = 0
			node.UsageCPUPercent = 0
			node.UsageMemoryPercent = 0
			continue
		}

		allocCPU := node.AllocCPU
		if allocCPU <= 0 {
			allocCPU = node.CapacityCPU
		}
		allocMemory := node.AllocMemoryBytes
		if allocMemory <= 0 {
			allocMemory = node.CapacityMemoryBytes
		}
		if allocCPU <= 0 || allocMemory <= 0 {
			continue
		}

		name := strings.TrimSpace(node.Name)
		sum := podTotals[name]
		activity := mockKubernetesActivity(name+"-node", now, 7*60, true)
		overheadCPU := int64(float64(allocCPU*1000) * clampFloat(0.04+activity*0.08, 0.03, 0.18))
		overheadMemory := int64(float64(allocMemory) * clampFloat(0.1+activity*0.12, 0.08, 0.26))

		usedCPU := sum.cpuMilli + overheadCPU
		maxCPU := allocCPU * 1000
		if usedCPU < 0 {
			usedCPU = 0
		}
		if usedCPU > maxCPU {
			usedCPU = maxCPU
		}

		usedMemory := sum.memBytes + overheadMemory
		if usedMemory < 0 {
			usedMemory = 0
		}
		if usedMemory > allocMemory {
			usedMemory = allocMemory
		}

		node.UsageCPUMilliCores = usedCPU
		node.UsageMemoryBytes = usedMemory
		node.UsageCPUPercent = clampFloat((float64(usedCPU)/float64(maxCPU))*100, 0, 100)
		node.UsageMemoryPercent = clampFloat((float64(usedMemory)/float64(allocMemory))*100, 0, 100)
	}
}

func mockStableHash64(parts ...string) uint64 {
	h := fnv.New64a()
	for _, part := range parts {
		if part == "" {
			continue
		}
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}

func mockKubernetesActivity(seed string, now time.Time, windowSeconds int64, randomize bool) float64 {
	if windowSeconds <= 0 {
		windowSeconds = 5 * 60
	}
	hash := mockStableHash64(seed)
	bucket := (now.Unix() / windowSeconds) + int64(hash%19)
	phase := bucket % 9

	base := 0.2
	switch phase {
	case 0, 1, 2, 3:
		base = 0.16
	case 4, 5:
		base = 0.42
	case 6:
		base = 0.74
	case 7:
		base = 0.55
	default:
		base = 0.3
	}

	progress := float64(now.Unix()%windowSeconds) / float64(windowSeconds)
	phaseOffset := float64(hash%628) / 100.0
	wave := 0.1 * math.Sin((progress*2*math.Pi)+phaseOffset)
	if randomize {
		wave += (rand.Float64() - 0.5) * 0.08
	}
	return clampFloat(base+wave, 0.05, 0.98)
}

func smoothMetricToward(current, target, weight float64) float64 {
	if weight <= 0 || weight > 1 {
		weight = 0.25
	}
	if current <= 0 {
		current = target * (0.68 + rand.Float64()*0.22)
	}
	return current + ((target - current) * weight)
}

func generateKubernetesNodes(clusterID string, count int) []models.KubernetesNode {
	if count <= 0 {
		return []models.KubernetesNode{}
	}

	nodes := make([]models.KubernetesNode, 0, count)
	architectures := []string{"amd64", "arm64"}
	runtimes := []string{"containerd://1.7.21", "containerd://1.7.20", "cri-o://1.30.4"}

	for i := 0; i < count; i++ {
		name := fmt.Sprintf("%s-node-%d", clusterID, i+1)
		ready := rand.Float64() > 0.08
		unschedulable := rand.Float64() < 0.08

		roles := []string{"worker"}
		if i == 0 {
			roles = []string{"control-plane"}
		} else if i == 1 && count > 3 {
			roles = []string{"worker", "gpu"}
		}

		cpu := int64(2 + rand.Intn(30))
		memGiB := int64(8 + rand.Intn(248))
		pods := int64(110 + rand.Intn(50))
		capacityMemory := memGiB * 1024 * 1024 * 1024
		allocCPU := cpu - int64(rand.Intn(2))
		if allocCPU < 1 {
			allocCPU = 1
		}
		allocMem := capacityMemory - int64(rand.Intn(3))*1024*1024*1024
		if allocMem < 1 {
			allocMem = capacityMemory
		}

		nodes = append(nodes, models.KubernetesNode{
			UID:                     fmt.Sprintf("%s-%s", name, randomHexString(8)),
			Name:                    name,
			Ready:                   ready,
			Unschedulable:           unschedulable,
			KubeletVersion:          k8sVersions[rand.Intn(len(k8sVersions))],
			ContainerRuntimeVersion: runtimes[rand.Intn(len(runtimes))],
			OSImage:                 k8sNodeOS[rand.Intn(len(k8sNodeOS))],
			KernelVersion:           dockerKernelVersions[rand.Intn(len(dockerKernelVersions))],
			Architecture:            architectures[rand.Intn(len(architectures))],
			CapacityCPU:             cpu,
			CapacityMemoryBytes:     capacityMemory,
			CapacityPods:            pods,
			AllocCPU:                allocCPU,
			AllocMemoryBytes:        allocMem,
			AllocPods:               pods - int64(rand.Intn(10)),
			Roles:                   roles,
		})
	}

	// Ensure at least one node issue sometimes.
	if count > 2 && rand.Float64() < 0.35 {
		idx := 1 + rand.Intn(count-1)
		nodes[idx].Ready = false
	}

	return nodes
}

func generateKubernetesPods(clusterID string, nodes []models.KubernetesNode, count int) []models.KubernetesPod {
	if count <= 0 {
		return []models.KubernetesPod{}
	}

	now := time.Now()
	pods := make([]models.KubernetesPod, 0, count)

	for i := 0; i < count; i++ {
		namespace := k8sNamespaces[rand.Intn(len(k8sNamespaces))]
		prefix := k8sPodPrefixes[rand.Intn(len(k8sPodPrefixes))]
		name := fmt.Sprintf("%s-%s-%d", prefix, randomHexString(5), i+1)
		nodeName := ""
		if len(nodes) > 0 && rand.Float64() > 0.08 {
			nodeName = nodes[rand.Intn(len(nodes))].Name
		}

		createdAt := now.Add(-time.Duration(30+rand.Intn(7200)) * time.Second)
		startTime := createdAt.Add(time.Duration(10+rand.Intn(120)) * time.Second)
		qos := []string{"Guaranteed", "Burstable", "BestEffort"}[rand.Intn(3)]

		phase := "Running"
		reason := ""
		message := ""
		containerState := "running"
		containerReason := ""
		containerMessage := ""
		containerReady := true
		restarts := 0

		roll := rand.Float64()
		switch {
		case roll < 0.08:
			phase = "Pending"
			reason = "Unschedulable"
			message = "0/3 nodes available"
			containerState = "waiting"
			containerReason = "PodInitializing"
			containerReady = false
		case roll < 0.14:
			phase = "Running"
			containerState = "waiting"
			containerReason = "CrashLoopBackOff"
			containerMessage = "Back-off restarting failed container"
			containerReady = false
			restarts = 3 + rand.Intn(20)
		case roll < 0.18:
			phase = "Failed"
			reason = "Error"
			message = "Pod terminated with non-zero exit code"
			containerState = "terminated"
			containerReason = "Error"
			containerReady = false
			restarts = 1 + rand.Intn(6)
		case roll < 0.22:
			phase = "Unknown"
			reason = "NodeLost"
			message = "Node status is unknown"
			containerState = "unknown"
			containerReady = false
		}

		containerCount := 1
		if rand.Float64() < 0.12 {
			containerCount = 2
		}

		containers := make([]models.KubernetesPodContainer, 0, containerCount)
		for c := 0; c < containerCount; c++ {
			image := k8sImages[rand.Intn(len(k8sImages))]
			containers = append(containers, models.KubernetesPodContainer{
				Name:         fmt.Sprintf("%s-%d", prefix, c+1),
				Image:        image,
				Ready:        containerReady,
				RestartCount: int32(restarts),
				State:        containerState,
				Reason:       containerReason,
				Message:      containerMessage,
			})
		}

		ownerKind := []string{"Deployment", "StatefulSet", "DaemonSet", "Job"}[rand.Intn(4)]
		ownerName := fmt.Sprintf("%s-%s", strings.ToLower(prefix), randomHexString(4))

		labels := map[string]string{
			"app.kubernetes.io/name":       prefix,
			"app.kubernetes.io/instance":   ownerName,
			"app.kubernetes.io/managed-by": "mock",
		}

		pod := models.KubernetesPod{
			UID:        fmt.Sprintf("%s-%s", name, randomHexString(10)),
			Name:       name,
			Namespace:  namespace,
			NodeName:   nodeName,
			Phase:      phase,
			Reason:     reason,
			Message:    message,
			QoSClass:   qos,
			CreatedAt:  createdAt,
			StartTime:  &startTime,
			Restarts:   restarts,
			Labels:     labels,
			OwnerKind:  ownerKind,
			OwnerName:  ownerName,
			Containers: containers,
		}

		if phase == "Pending" {
			pod.StartTime = nil
		}

		pods = append(pods, pod)
	}

	return pods
}

func generateKubernetesDeployments(clusterID string, count int) []models.KubernetesDeployment {
	if count <= 0 {
		return []models.KubernetesDeployment{}
	}

	deployments := make([]models.KubernetesDeployment, 0, count)
	for i := 0; i < count; i++ {
		namespace := k8sNamespaces[rand.Intn(len(k8sNamespaces))]
		prefix := k8sPodPrefixes[rand.Intn(len(k8sPodPrefixes))]
		name := fmt.Sprintf("%s-%s", prefix, randomHexString(4))

		desired := int32(1 + rand.Intn(6))
		updated := desired
		ready := desired
		available := desired

		// Degrade some deployments for UI coverage.
		if rand.Float64() < 0.20 && desired > 0 {
			ready = int32(rand.Intn(int(desired)))
			available = ready
			updated = int32(rand.Intn(int(desired) + 1))
		}

		deployments = append(deployments, models.KubernetesDeployment{
			UID:               fmt.Sprintf("%s-%s", name, randomHexString(10)),
			Name:              name,
			Namespace:         namespace,
			DesiredReplicas:   desired,
			UpdatedReplicas:   updated,
			ReadyReplicas:     ready,
			AvailableReplicas: available,
			Labels: map[string]string{
				"app.kubernetes.io/name": prefix,
				"cluster":                clusterID,
			},
		})
	}

	return deployments
}

func clusterHasIssues(nodes []models.KubernetesNode, pods []models.KubernetesPod, deployments []models.KubernetesDeployment) bool {
	for _, node := range nodes {
		if !node.Ready || node.Unschedulable {
			return true
		}
	}

	for _, pod := range pods {
		if !kubernetesPodHealthy(pod) {
			return true
		}
	}

	for _, d := range deployments {
		if !kubernetesDeploymentHealthy(d) {
			return true
		}
	}

	return false
}

func kubernetesPodHealthy(pod models.KubernetesPod) bool {
	if strings.ToLower(strings.TrimSpace(pod.Phase)) != "running" {
		return false
	}
	for _, c := range pod.Containers {
		if !c.Ready {
			return false
		}
		state := strings.ToLower(strings.TrimSpace(c.State))
		if state != "" && state != "running" {
			return false
		}
	}
	return true
}

func kubernetesDeploymentHealthy(d models.KubernetesDeployment) bool {
	desired := d.DesiredReplicas
	if desired <= 0 {
		return true
	}
	return d.ReadyReplicas >= desired && d.AvailableReplicas >= desired && d.UpdatedReplicas >= desired
}

func generateDockerHosts(config MockConfig) []models.DockerHost {
	hostCount := config.DockerHostCount
	if hostCount <= 0 {
		return []models.DockerHost{}
	}

	now := time.Now()
	hosts := make([]models.DockerHost, 0, hostCount)

	for i := 0; i < hostCount; i++ {
		agentVersion := dockerAgentVersions[rand.Intn(len(dockerAgentVersions))]
		prefix := dockerHostPrefixes[i%len(dockerHostPrefixes)]
		hostname := fmt.Sprintf("%s-%d", prefix, i+1)
		hostID := fmt.Sprintf("%s-mock", hostname)

		cpus := []int{4, 6, 8, 12, 16}[rand.Intn(5)]
		totalMemoryBytes := int64((16 + rand.Intn(64)) * 1024 * 1024 * 1024) // 16-79 GB
		interval := 30
		uptime := int64(86400*(3+rand.Intn(25))) + int64(rand.Intn(3600))

		isPodman := hostCount > 1 && i%3 == 0
		runtime := "docker"
		runtimeVersion := dockerVersions[rand.Intn(len(dockerVersions))]
		dockerVersion := runtimeVersion
		if isPodman {
			runtime = "podman"
			runtimeVersion = podmanVersions[rand.Intn(len(podmanVersions))]
			dockerVersion = ""
		}

		containers := generateDockerContainers(hostname, i, config, isPodman)

		// Optionally ensure the second host looks degraded for UI coverage
		if i == 1 && len(containers) > 0 && hostCount > 1 {
			idx := rand.Intn(len(containers))
			containers[idx].Health = "unhealthy"
			containers[idx].CPUPercent = clampFloat(containers[idx].CPUPercent+35, 5, 190)
		}

		// Determine initial status - only mark last host as offline for testing
		status := "online"
		explicitlyOffline := false
		if hostCount > 2 && i == hostCount-1 {
			status = "offline"
			explicitlyOffline = true
		}

		lastSeen := now.Add(-time.Duration(rand.Intn(20)) * time.Second)
		if explicitlyOffline {
			// If host is explicitly offline, stop all containers and update metrics
			lastSeen = now.Add(-time.Duration(5+rand.Intn(20)) * time.Minute)
			for idx := range containers {
				exitCode := containers[idx].ExitCode
				if exitCode == 0 {
					exitCode = []int{0, 1, 137}[rand.Intn(3)]
				}
				containers[idx].State = "exited"
				containers[idx].Health = ""
				containers[idx].CPUPercent = 0
				containers[idx].MemoryUsage = 0
				containers[idx].MemoryPercent = 0
				containers[idx].UptimeSeconds = 0
				finished := now.Add(-time.Duration(rand.Intn(72)+1) * time.Hour)
				containers[idx].StartedAt = nil
				containers[idx].FinishedAt = &finished
				containers[idx].Status = fmt.Sprintf("Exited (%d) %s ago", exitCode, formatDurationForStatus(now.Sub(finished)))
			}
		} else {
			// For hosts not explicitly offline, calculate status based on container health
			running := 0
			unhealthy := 0
			for _, ct := range containers {
				if strings.ToLower(ct.State) == "running" {
					running++
					healthState := strings.ToLower(ct.Health)
					if healthState == "unhealthy" || healthState == "starting" {
						unhealthy++
					}
				}
			}

			// Only mark as offline if there are containers but none are running
			// (Swarm-only hosts with no standalone containers should stay online)
			if len(containers) > 0 && running == 0 {
				status = "offline"
				lastSeen = now.Add(-time.Duration(3+rand.Intn(5)) * time.Minute)
			} else if unhealthy > 0 || (len(containers) > 0 && float64(len(containers)-running)/float64(len(containers)) > 0.35) {
				status = "degraded"
				if lastSeen.After(now.Add(-30 * time.Second)) {
					lastSeen = now.Add(-35 * time.Second)
				}
			} else {
				status = "online"
			}
		}

		var swarmInfo *models.DockerSwarmInfo
		var services []models.DockerService
		var tasks []models.DockerTask
		if !isPodman {
			swarmInfo = &models.DockerSwarmInfo{
				NodeID:           fmt.Sprintf("%s-node", hostID),
				NodeRole:         "worker",
				LocalState:       "active",
				ControlAvailable: false,
				Scope:            "node",
			}

			if i%2 == 0 {
				swarmInfo.NodeRole = "manager"
				swarmInfo.ControlAvailable = true
				swarmInfo.Scope = "cluster"
				services, tasks = generateDockerServicesAndTasks(hostname, containers, now)
				if len(services) == 0 {
					swarmInfo.Scope = "node"
				}
			} else if i%3 == 0 {
				services, tasks = generateDockerServicesAndTasks(hostname, containers, now)
			}
		}

		cpuUsage := clampFloat(10+rand.Float64()*70, 4, 98)
		loadAverage := []float64{
			clampFloat(rand.Float64()*float64(cpus), 0, float64(cpus)+1),
			clampFloat(rand.Float64()*float64(cpus), 0, float64(cpus)+1),
			clampFloat(rand.Float64()*float64(cpus), 0, float64(cpus)+1),
		}

		memUsageRatio := clampFloat(0.3+rand.Float64()*0.55, 0.05, 0.98)
		usedMemoryBytes := int64(float64(totalMemoryBytes) * memUsageRatio)
		if usedMemoryBytes > totalMemoryBytes {
			usedMemoryBytes = totalMemoryBytes
		}
		freeMemoryBytes := totalMemoryBytes - usedMemoryBytes
		memory := models.Memory{
			Total: totalMemoryBytes,
			Used:  usedMemoryBytes,
			Free:  freeMemoryBytes,
		}
		if totalMemoryBytes > 0 {
			memory.Usage = clampFloat((float64(usedMemoryBytes)/float64(totalMemoryBytes))*100, 0, 100)
		}

		diskTotal := int64((250 + rand.Intn(750)) * 1024 * 1024 * 1024) // 250-999 GB
		diskUsed := int64(float64(diskTotal) * clampFloat(0.35+rand.Float64()*0.5, 0.1, 0.97))
		if diskUsed > diskTotal {
			diskUsed = diskTotal
		}
		diskFree := diskTotal - diskUsed
		diskUsage := 0.0
		if diskTotal > 0 {
			diskUsage = clampFloat((float64(diskUsed)/float64(diskTotal))*100, 0, 100)
		}

		disks := []models.Disk{
			{
				Total:      diskTotal,
				Used:       diskUsed,
				Free:       diskFree,
				Usage:      diskUsage,
				Mountpoint: "/",
				Type:       "ext4",
				Device:     "/dev/sda1",
			},
		}

		networkInterfaces := []models.HostNetworkInterface{
			{
				Name:      "eth0",
				Addresses: []string{fmt.Sprintf("10.10.%d.%d/24", i%20, rand.Intn(200)+10)},
				RXBytes:   uint64(rand.Int63n(5_000_000_000) + 500_000_000),
				TXBytes:   uint64(rand.Int63n(4_000_000_000) + 400_000_000),
			},
		}
		if rand.Intn(4) == 0 {
			networkInterfaces = append(networkInterfaces, models.HostNetworkInterface{
				Name:      "eth1",
				Addresses: []string{fmt.Sprintf("172.16.%d.%d/24", i%16, rand.Intn(200)+20)},
				RXBytes:   uint64(rand.Int63n(2_000_000_000) + 200_000_000),
				TXBytes:   uint64(rand.Int63n(1_500_000_000) + 150_000_000),
			})
		}
		temperature := generateMockTemperature(cpuUsage)
		if status == "offline" {
			temperature = nil
		}

		host := models.DockerHost{
			ID:                hostID,
			AgentID:           fmt.Sprintf("agent-%s", randomHexString(6)),
			Hostname:          hostname,
			DisplayName:       humanizeHostDisplayName(hostname),
			MachineID:         randomHexString(32),
			OS:                dockerOperatingSystems[rand.Intn(len(dockerOperatingSystems))],
			KernelVersion:     dockerKernelVersions[rand.Intn(len(dockerKernelVersions))],
			Architecture:      dockerArchitectures[rand.Intn(len(dockerArchitectures))],
			Runtime:           runtime,
			RuntimeVersion:    runtimeVersion,
			DockerVersion:     dockerVersion,
			CPUs:              cpus,
			TotalMemoryBytes:  totalMemoryBytes,
			UptimeSeconds:     uptime,
			CPUUsage:          cpuUsage,
			LoadAverage:       loadAverage,
			Memory:            memory,
			Disks:             disks,
			NetworkInterfaces: networkInterfaces,
			Status:            status,
			LastSeen:          lastSeen,
			IntervalSeconds:   interval,
			AgentVersion:      agentVersion,
			Containers:        containers,
			Services:          services,
			Tasks:             tasks,
			Swarm:             swarmInfo,
			Temperature:       temperature,
			NetInRate:         generateMockHostRate("network-in"),
			NetOutRate:        generateMockHostRate("network-out"),
			DiskReadRate:      generateMockHostRate("disk-read"),
			DiskWriteRate:     generateMockHostRate("disk-write"),
		}
		if status == "offline" {
			host.NetInRate = 0
			host.NetOutRate = 0
			host.DiskReadRate = 0
			host.DiskWriteRate = 0
		}

		hosts = append(hosts, host)
	}

	return hosts
}

func generateHosts(config MockConfig) []models.Host {
	count := config.GenericHostCount
	if count <= 0 {
		return nil
	}

	now := time.Now()
	hosts := make([]models.Host, 0, count)
	usedNames := make(map[string]struct{}, count)

	for i := 0; i < count; i++ {
		profile := genericHostProfiles[rand.Intn(len(genericHostProfiles))]

		baseName := genericHostPrefixes[rand.Intn(len(genericHostPrefixes))]
		suffix := 1 + rand.Intn(900)
		hostname := fmt.Sprintf("%s-%d", baseName, suffix)
		for {
			if _, exists := usedNames[hostname]; !exists {
				usedNames[hostname] = struct{}{}
				break
			}
			suffix++
			hostname = fmt.Sprintf("%s-%d", baseName, suffix)
		}

		displayName := strings.ToUpper(hostname[:1]) + hostname[1:]

		cpuCount := 4 + rand.Intn(28) // 4-32 cores
		if profile.Platform == "macos" {
			cpuCount = 8 + rand.Intn(10)
		}
		cpuUsage := clampFloat(10+rand.Float64()*55, 4, 94)

		memTotalGiB := 16 + rand.Intn(192)
		if profile.Platform == "macos" {
			memTotalGiB = 16 + rand.Intn(64)
		}
		memTotal := int64(memTotalGiB) << 30
		memUsage := clampFloat(30+rand.Float64()*50, 12, 96)
		memUsed := int64(float64(memTotal) * (memUsage / 100.0))
		memFree := memTotal - memUsed

		swapTotal := int64(rand.Intn(32)) << 30
		swapUsed := int64(float64(swapTotal) * rand.Float64())

		rootDiskTotal := int64(120+rand.Intn(680)) << 30
		rootDiskUsage := clampFloat(25+rand.Float64()*55, 8, 95)
		rootDiskUsed := int64(float64(rootDiskTotal) * (rootDiskUsage / 100.0))
		rootDisk := models.Disk{
			Total:      rootDiskTotal,
			Used:       rootDiskUsed,
			Free:       rootDiskTotal - rootDiskUsed,
			Usage:      rootDiskUsage,
			Mountpoint: "/",
			Type:       "ext4",
			Device:     "/dev/sda1",
		}
		if profile.Platform == "windows" {
			rootDisk.Mountpoint = "C:"
			rootDisk.Type = "ntfs"
			rootDisk.Device = `\\.\PHYSICALDRIVE0`
		}
		if profile.Platform == "macos" {
			rootDisk.Type = "apfs"
			rootDisk.Device = "/dev/disk1s1"
		}

		disks := []models.Disk{rootDisk}
		if rand.Float64() < 0.45 {
			dataDiskTotal := int64(200+rand.Intn(1400)) << 30
			dataDiskUsage := clampFloat(35+rand.Float64()*45, 6, 97)
			dataDiskUsed := int64(float64(dataDiskTotal) * (dataDiskUsage / 100.0))
			mount := "/data"
			device := "/dev/sdb1"
			fsType := "xfs"
			if profile.Platform == "windows" {
				mount = "D:"
				device = `\\.\PHYSICALDRIVE1`
				fsType = "ntfs"
			}
			if profile.Platform == "macos" {
				mount = "/Volumes/Data"
				device = "/dev/disk3s1"
				fsType = "apfs"
			}
			disks = append(disks, models.Disk{
				Total:      dataDiskTotal,
				Used:       dataDiskUsed,
				Free:       dataDiskTotal - dataDiskUsed,
				Usage:      dataDiskUsage,
				Mountpoint: mount,
				Type:       fsType,
				Device:     device,
			})
		}

		primaryIP := fmt.Sprintf("192.168.%d.%d", 10+rand.Intn(60), 10+rand.Intn(200))
		network := []models.HostNetworkInterface{
			{
				Name:      "eth0",
				MAC:       fmt.Sprintf("02:42:%02x:%02x:%02x:%02x", rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256)),
				Addresses: []string{primaryIP},
				RXBytes:   uint64(256+rand.Intn(4096)) * 1024 * 1024,
				TXBytes:   uint64(256+rand.Intn(4096)) * 1024 * 1024,
			},
		}
		if rand.Float64() < 0.32 {
			network[0].Addresses = append(network[0].Addresses, fmt.Sprintf("10.%d.%d.%d", 10+rand.Intn(90), rand.Intn(200), rand.Intn(200)))
		}

		var loadAverage []float64
		if profile.Platform == "linux" {
			loadAverage = []float64{
				clampFloat(rand.Float64()*float64(cpuCount)/4, 0.05, float64(cpuCount)*0.8),
				clampFloat(rand.Float64()*float64(cpuCount)/4, 0.05, float64(cpuCount)*0.8),
				clampFloat(rand.Float64()*float64(cpuCount)/4, 0.05, float64(cpuCount)*0.8),
			}
		}

		sensors := models.HostSensorSummary{}
		if profile.Platform == "linux" || profile.Platform == "macos" {
			sensors.TemperatureCelsius = map[string]float64{
				"cpu.package": clampFloat(38+rand.Float64()*22, 30, 85),
			}
			if rand.Float64() < 0.4 {
				sensors.Additional = map[string]float64{
					"nvme0": clampFloat(40+rand.Float64()*20, 30, 90),
				}
			}
		}

		status := "online"
		if rand.Float64() < 0.1 {
			status = "offline"
		} else if rand.Float64() < 0.12 {
			status = "degraded"
		}

		lastSeen := now.Add(-time.Duration(rand.Intn(60)) * time.Second)
		if status == "offline" {
			lastSeen = now.Add(-time.Duration(300+rand.Intn(2400)) * time.Second)
		}

		uptimeSeconds := int64(3600*(12+rand.Intn(720))) + int64(rand.Intn(3600))
		intervalSeconds := 30 + rand.Intn(45)

		tags := make([]string, 0, 2)
		for _, candidate := range []string{"production", "lab", "edge", "backup", "database", "web"} {
			if rand.Float64() < 0.18 {
				tags = append(tags, candidate)
			}
		}

		var tokenID, tokenName, tokenHint string
		var tokenLastUsed *time.Time
		if rand.Float64() < 0.8 {
			tokenID = fmt.Sprintf("hst_%s", randomHexString(8))
			tokenName = fmt.Sprintf("%s agent", displayName)
			tokenHint = fmt.Sprintf("%s…%s", tokenID[:3], tokenID[len(tokenID)-2:])
			lastUsed := now.Add(-time.Duration(rand.Intn(720)) * time.Minute)
			tokenLastUsed = &lastUsed
		}

		if rand.Float64() < 0.1 {
			// Simulate a revoked token that hasn't been rotated yet
			tokenID = ""
			tokenHint = fmt.Sprintf("%s…%s", randomHexString(3), randomHexString(2))
			tokenLastUsed = nil
		}

		host := models.Host{
			ID:                fmt.Sprintf("host-%s-%d", profile.Platform, i+1),
			Hostname:          hostname,
			DisplayName:       displayName,
			Platform:          profile.Platform,
			OSName:            profile.OSName,
			OSVersion:         profile.OSVersion,
			KernelVersion:     profile.Kernel,
			Architecture:      profile.Architecture,
			CPUCount:          cpuCount,
			CPUUsage:          cpuUsage,
			LoadAverage:       loadAverage,
			Memory:            models.Memory{Total: memTotal, Used: memUsed, Free: memFree, Usage: memUsage, SwapTotal: swapTotal, SwapUsed: swapUsed},
			Disks:             disks,
			NetworkInterfaces: network,
			Sensors:           sensors,
			Status:            status,
			UptimeSeconds:     uptimeSeconds,
			IntervalSeconds:   intervalSeconds,
			LastSeen:          lastSeen,
			AgentVersion:      hostAgentVersions[rand.Intn(len(hostAgentVersions))],
			Tags:              tags,
			TokenID:           tokenID,
			TokenName:         tokenName,
			TokenHint:         tokenHint,
			TokenLastUsedAt:   tokenLastUsed,
			NetInRate:         generateMockHostRate("network-in"),
			NetOutRate:        generateMockHostRate("network-out"),
			DiskReadRate:      generateMockHostRate("disk-read"),
			DiskWriteRate:     generateMockHostRate("disk-write"),
		}
		if status == "offline" {
			host.NetInRate = 0
			host.NetOutRate = 0
			host.DiskReadRate = 0
			host.DiskWriteRate = 0
		}

		hosts = append(hosts, host)
	}

	sort.Slice(hosts, func(i, j int) bool {
		return hosts[i].Hostname < hosts[j].Hostname
	})

	return hosts
}

func ensureMockNodeHostLinks(data *models.StateSnapshot) {
	if data == nil || len(data.Nodes) == 0 {
		return
	}

	now := time.Now()

	for i := range data.Hosts {
		host := &data.Hosts[i]
		host.LinkedNodeID = ""
		host.LinkedVMID = ""
		host.LinkedContainerID = ""
		if host.Status == "offline" {
			host.NetInRate = 0
			host.NetOutRate = 0
			host.DiskReadRate = 0
			host.DiskWriteRate = 0
			continue
		}
		if host.NetInRate <= 0 {
			host.NetInRate = generateMockHostRate("network-in")
		}
		if host.NetOutRate <= 0 {
			host.NetOutRate = generateMockHostRate("network-out")
		}
		if host.DiskReadRate <= 0 {
			host.DiskReadRate = generateMockHostRate("disk-read")
		}
		if host.DiskWriteRate <= 0 {
			host.DiskWriteRate = generateMockHostRate("disk-write")
		}
	}

	for i := range data.Nodes {
		node := &data.Nodes[i]
		hostID := ""
		if i < len(data.Hosts) {
			hostID = data.Hosts[i].ID
		}
		linkedHost := buildMockLinkedHostFromNode(*node, hostID, i, now)
		if i < len(data.Hosts) {
			data.Hosts[i] = linkedHost
		} else {
			data.Hosts = append(data.Hosts, linkedHost)
		}
		node.LinkedHostAgentID = linkedHost.ID
	}
}

func ensureMockKubernetesNodeHostLinks(data *models.StateSnapshot) {
	if data == nil || len(data.KubernetesClusters) == 0 {
		return
	}

	now := time.Now()
	hostsByName := make(map[string]int, len(data.Hosts))
	for i := range data.Hosts {
		name := strings.ToLower(strings.TrimSpace(data.Hosts[i].Hostname))
		if name != "" {
			hostsByName[name] = i
		}
	}

	nextHostIndex := len(data.Hosts)
	for clusterIndex := range data.KubernetesClusters {
		cluster := &data.KubernetesClusters[clusterIndex]
		for nodeIndex := range cluster.Nodes {
			node := &cluster.Nodes[nodeIndex]
			hostname := strings.ToLower(strings.TrimSpace(node.Name))
			if hostname == "" {
				continue
			}

			hostIndex, exists := hostsByName[hostname]
			hostID := ""
			if exists && hostIndex >= 0 && hostIndex < len(data.Hosts) {
				hostID = data.Hosts[hostIndex].ID
			}

			host := buildMockLinkedHostFromKubernetesNode(*cluster, *node, hostID, nextHostIndex, now)
			if exists && hostIndex >= 0 && hostIndex < len(data.Hosts) {
				data.Hosts[hostIndex] = host
			} else {
				data.Hosts = append(data.Hosts, host)
				hostsByName[hostname] = len(data.Hosts) - 1
				nextHostIndex++
			}
		}
	}
}

func buildMockLinkedHostFromKubernetesNode(
	cluster models.KubernetesCluster,
	node models.KubernetesNode,
	hostID string,
	hostIndex int,
	now time.Time,
) models.Host {
	if hostID == "" {
		hostID = fmt.Sprintf("host-k8s-%s-%s", strings.TrimSpace(cluster.ID), strings.TrimSpace(node.UID))
		if strings.TrimSpace(node.UID) == "" {
			hostID = fmt.Sprintf("host-k8s-%s-%d", strings.TrimSpace(cluster.ID), hostIndex+1)
		}
	}

	allocCPU := node.AllocCPU
	if allocCPU <= 0 {
		allocCPU = node.CapacityCPU
	}
	if allocCPU <= 0 {
		allocCPU = 4
	}
	allocMemory := node.AllocMemoryBytes
	if allocMemory <= 0 {
		allocMemory = node.CapacityMemoryBytes
	}
	if allocMemory <= 0 {
		allocMemory = int64(16) * 1024 * 1024 * 1024
	}

	cpuUsage := node.UsageCPUPercent
	if cpuUsage <= 0 && node.UsageCPUMilliCores > 0 {
		cpuUsage = (float64(node.UsageCPUMilliCores) / float64(allocCPU*1000)) * 100
	}
	cpuUsage = clampFloat(cpuUsage, 4, 96)

	memUsage := node.UsageMemoryPercent
	if memUsage <= 0 && node.UsageMemoryBytes > 0 {
		memUsage = (float64(node.UsageMemoryBytes) / float64(allocMemory)) * 100
	}
	memUsage = clampFloat(memUsage, 12, 94)
	memUsed := int64(float64(allocMemory) * (memUsage / 100.0))
	memFree := allocMemory - memUsed
	if memFree < 0 {
		memFree = 0
	}

	status := "online"
	if !node.Ready {
		status = "degraded"
	}
	if strings.EqualFold(strings.TrimSpace(cluster.Status), "offline") {
		status = "offline"
	}
	lastSeen := now.Add(-time.Duration(rand.Intn(20)) * time.Second)
	if status == "offline" {
		lastSeen = now.Add(-time.Duration(180+rand.Intn(1200)) * time.Second)
	}

	diskTotal := int64(128+(mockStableHash64(cluster.ID, node.Name, "disk-total")%1024)) * 1024 * 1024 * 1024
	diskUsage := clampFloat(24+float64(mockStableHash64(cluster.ID, node.Name, "disk-usage")%44), 8, 92)
	diskUsed := int64(float64(diskTotal) * (diskUsage / 100.0))
	diskFree := diskTotal - diskUsed
	if diskFree < 0 {
		diskFree = 0
	}

	sensors := models.HostSensorSummary{}
	if temp := generateMockTemperature(cpuUsage); temp != nil {
		sensors.TemperatureCelsius = map[string]float64{
			"cpu_package": *temp,
		}
	}

	tags := []string{"mock", "kubernetes-node"}
	for _, role := range node.Roles {
		role = strings.TrimSpace(role)
		if role == "" {
			continue
		}
		already := false
		for _, existing := range tags {
			if existing == role {
				already = true
				break
			}
		}
		if !already {
			tags = append(tags, role)
		}
	}

	host := models.Host{
		ID:            hostID,
		Hostname:      node.Name,
		DisplayName:   humanizeHostDisplayName(node.Name),
		Platform:      "linux",
		OSName:        node.OSImage,
		KernelVersion: node.KernelVersion,
		Architecture:  node.Architecture,
		CPUCount:      int(allocCPU),
		CPUUsage:      cpuUsage,
		Memory: models.Memory{
			Total: allocMemory,
			Used:  memUsed,
			Free:  memFree,
			Usage: memUsage,
		},
		Disks: []models.Disk{
			{
				Total:      diskTotal,
				Used:       diskUsed,
				Free:       diskFree,
				Usage:      diskUsage,
				Mountpoint: "/",
				Type:       "ext4",
				Device:     "/dev/nvme0n1p1",
			},
		},
		NetworkInterfaces: []models.HostNetworkInterface{
			{
				Name: "eth0",
				MAC:  fmt.Sprintf("02:6f:%02x:%02x:%02x:%02x", hostIndex%255, (hostIndex*17)%255, (hostIndex*31)%255, (hostIndex*41)%255),
				Addresses: []string{
					fmt.Sprintf("10.%d.%d.%d", 20+(hostIndex%90), 10+rand.Intn(120), 20+rand.Intn(200)),
				},
			},
		},
		Sensors:         sensors,
		Status:          status,
		UptimeSeconds:   int64(3600 * (24 + int(mockStableHash64(cluster.ID, node.Name, "uptime")%720))),
		IntervalSeconds: 30,
		LastSeen:        lastSeen,
		AgentVersion:    hostAgentVersions[rand.Intn(len(hostAgentVersions))],
		MachineID:       randomHexString(32),
		Tags:            tags,
	}
	updateMockHostRates(&host)
	if status == "offline" {
		host.NetInRate = 0
		host.NetOutRate = 0
		host.DiskReadRate = 0
		host.DiskWriteRate = 0
	}
	return host
}

func syncMockKubernetesNodeHosts(data *models.StateSnapshot) {
	if data == nil || len(data.Hosts) == 0 || len(data.KubernetesClusters) == 0 {
		return
	}

	hostsByName := make(map[string]*models.Host, len(data.Hosts))
	for i := range data.Hosts {
		host := &data.Hosts[i]
		hostname := strings.ToLower(strings.TrimSpace(host.Hostname))
		if hostname != "" {
			hostsByName[hostname] = host
		}
	}

	for _, cluster := range data.KubernetesClusters {
		for _, node := range cluster.Nodes {
			host := hostsByName[strings.ToLower(strings.TrimSpace(node.Name))]
			if host == nil {
				continue
			}

			if strings.EqualFold(strings.TrimSpace(cluster.Status), "offline") {
				host.Status = "offline"
				host.NetInRate = 0
				host.NetOutRate = 0
				host.DiskReadRate = 0
				host.DiskWriteRate = 0
				continue
			}

			if !node.Ready {
				host.Status = "degraded"
			} else if strings.EqualFold(host.Status, "offline") {
				host.Status = "online"
			}

			if node.UsageCPUPercent > 0 {
				host.CPUUsage = clampFloat(smoothMetricToward(host.CPUUsage, node.UsageCPUPercent, 0.42), 2, 99)
			}

			totalMemory := node.AllocMemoryBytes
			if totalMemory <= 0 {
				totalMemory = node.CapacityMemoryBytes
			}
			if totalMemory > 0 {
				usedMemory := node.UsageMemoryBytes
				if usedMemory <= 0 && node.UsageMemoryPercent > 0 {
					usedMemory = int64(float64(totalMemory) * (node.UsageMemoryPercent / 100.0))
				}
				if usedMemory < 0 {
					usedMemory = 0
				}
				if usedMemory > totalMemory {
					usedMemory = totalMemory
				}
				host.Memory.Total = totalMemory
				host.Memory.Used = usedMemory
				host.Memory.Free = totalMemory - usedMemory
				if totalMemory > 0 {
					host.Memory.Usage = clampFloat((float64(usedMemory)/float64(totalMemory))*100, 0, 100)
				}
			}

			if temp := generateMockTemperature(host.CPUUsage); temp != nil {
				if host.Sensors.TemperatureCelsius == nil {
					host.Sensors.TemperatureCelsius = make(map[string]float64)
				}
				host.Sensors.TemperatureCelsius["cpu_package"] = *temp
			}
		}
	}
}

func buildMockLinkedHostFromNode(node models.Node, hostID string, hostIndex int, now time.Time) models.Host {
	if hostID == "" {
		hostID = fmt.Sprintf("host-node-%s", node.ID)
	}

	cpuCount := node.CPUInfo.Cores
	if cpuCount <= 0 {
		cpuCount = 8
	}

	cpuUsage := node.CPU
	if cpuUsage <= 1.0 {
		cpuUsage = cpuUsage * 100
	}
	cpuUsage = clampFloat(cpuUsage, 0, 100)

	memory := node.Memory
	if memory.Total <= 0 {
		memory.Total = int64(32) * 1024 * 1024 * 1024
		memory.Used = int64(float64(memory.Total) * 0.5)
	}
	if memory.Used <= 0 && memory.Usage > 0 {
		memory.Used = int64(float64(memory.Total) * (memory.Usage / 100.0))
	}
	if memory.Usage <= 0 && memory.Total > 0 {
		memory.Usage = float64(memory.Used) / float64(memory.Total) * 100.0
	}
	memory.Free = memory.Total - memory.Used
	if memory.Free < 0 {
		memory.Free = 0
	}

	disk := node.Disk
	if disk.Total <= 0 {
		disk.Total = int64(512) * 1024 * 1024 * 1024
		disk.Used = int64(float64(disk.Total) * 0.45)
	}
	if disk.Used <= 0 && disk.Usage > 0 {
		disk.Used = int64(float64(disk.Total) * (disk.Usage / 100.0))
	}
	if disk.Usage <= 0 && disk.Total > 0 {
		disk.Usage = float64(disk.Used) / float64(disk.Total) * 100.0
	}
	disk.Free = disk.Total - disk.Used
	if disk.Free < 0 {
		disk.Free = 0
	}

	disk.Mountpoint = "/"
	disk.Type = "zfs"
	disk.Device = "rpool"

	status := nodeStatusToHostStatus(node.Status)
	lastSeen := now.Add(-time.Duration(rand.Intn(12)) * time.Second)
	if status == "offline" {
		lastSeen = now.Add(-time.Duration(240+rand.Intn(1200)) * time.Second)
	}

	displayName := node.DisplayName
	if strings.TrimSpace(displayName) == "" {
		displayName = humanizeHostDisplayName(node.Name)
	}

	host := models.Host{
		ID:            hostID,
		Hostname:      node.Name,
		DisplayName:   displayName,
		Platform:      "linux",
		OSName:        "Proxmox VE",
		OSVersion:     node.PVEVersion,
		KernelVersion: node.KernelVersion,
		Architecture:  "x86_64",
		CPUCount:      cpuCount,
		CPUUsage:      cpuUsage,
		LoadAverage: []float64{
			clampFloat((cpuUsage/100.0)*float64(cpuCount)*0.8, 0.05, float64(cpuCount)*1.2),
			clampFloat((cpuUsage/100.0)*float64(cpuCount)*0.7, 0.05, float64(cpuCount)*1.2),
			clampFloat((cpuUsage/100.0)*float64(cpuCount)*0.6, 0.05, float64(cpuCount)*1.2),
		},
		Memory: memory,
		Disks:  []models.Disk{disk},
		NetworkInterfaces: []models.HostNetworkInterface{
			{
				Name: "eth0",
				MAC:  fmt.Sprintf("02:50:%02x:%02x:%02x:%02x", hostIndex%255, (hostIndex*13)%255, (hostIndex*29)%255, (hostIndex*37)%255),
				Addresses: []string{
					fmt.Sprintf("192.168.%d.%d", 80+(hostIndex%10), 20+(hostIndex%200)),
				},
				RXBytes: uint64(1_000_000_000 + rand.Int63n(5_000_000_000)),
				TXBytes: uint64(750_000_000 + rand.Int63n(4_000_000_000)),
			},
		},
		Sensors:         nodeTemperatureToHostSensors(node.Temperature),
		Status:          status,
		UptimeSeconds:   node.Uptime,
		IntervalSeconds: 30,
		LastSeen:        lastSeen,
		AgentVersion:    hostAgentVersions[rand.Intn(len(hostAgentVersions))],
		MachineID:       randomHexString(32),
		Tags:            []string{"mock", "proxmox-node"},
		LinkedNodeID:    node.ID,
	}

	updateMockHostRates(&host)
	if status == "offline" {
		host.NetInRate = 0
		host.NetOutRate = 0
		host.DiskReadRate = 0
		host.DiskWriteRate = 0
	}

	return host
}

func nodeStatusToHostStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "offline":
		return "offline"
	case "degraded":
		return "degraded"
	default:
		return "online"
	}
}

func nodeTemperatureToHostSensors(temperature *models.Temperature) models.HostSensorSummary {
	if temperature == nil || !temperature.Available {
		return models.HostSensorSummary{}
	}

	sensors := models.HostSensorSummary{
		TemperatureCelsius: map[string]float64{},
	}

	if temperature.CPUPackage > 0 {
		sensors.TemperatureCelsius["cpu_package"] = temperature.CPUPackage
	}
	for _, core := range temperature.Cores {
		sensors.TemperatureCelsius[fmt.Sprintf("cpu_core_%d", core.Core)] = core.Temp
	}

	if len(temperature.NVMe) > 0 {
		sensors.Additional = make(map[string]float64, len(temperature.NVMe))
		for _, nvme := range temperature.NVMe {
			if nvme.Device == "" {
				continue
			}
			sensors.Additional[nvme.Device] = nvme.Temp
		}
	}

	if len(sensors.TemperatureCelsius) == 0 {
		sensors.TemperatureCelsius = nil
	}
	if len(sensors.Additional) == 0 {
		sensors.Additional = nil
	}

	return sensors
}

func generateMockHostRate(ioType string) float64 {
	rate := float64(generateRealisticIO(ioType))
	if rate > 0 {
		return rate
	}

	switch ioType {
	case "network-in":
		return float64((128 + rand.Intn(4096)) * 1024) // 128 KiB/s - 4 MiB/s
	case "network-out":
		return float64((96 + rand.Intn(3072)) * 1024) // 96 KiB/s - 3 MiB/s
	case "disk-read":
		return float64((64 + rand.Intn(2048)) * 1024) // 64 KiB/s - 2 MiB/s
	case "disk-write":
		return float64((32 + rand.Intn(1536)) * 1024) // 32 KiB/s - 1.5 MiB/s
	default:
		return float64((32 + rand.Intn(512)) * 1024)
	}
}

func generateMockTemperature(cpuUsagePercent float64) *float64 {
	if cpuUsagePercent <= 0 {
		return nil
	}
	temp := clampFloat(34+(cpuUsagePercent*0.45)+rand.Float64()*4, 32, 88)
	return &temp
}

func fluctuateMockHostRate(current float64, ioType string, min, max float64) float64 {
	if current <= 0 {
		current = generateMockHostRate(ioType)
	}
	changeFactor := 1 + ((rand.Float64()*2)-1)*0.25
	next := current * changeFactor
	if rand.Float64() < 0.04 {
		next *= 1.4 + rand.Float64()*0.8
	}
	return clampFloat(next, min, max)
}

func updateMockHostRates(host *models.Host) {
	if host == nil {
		return
	}
	if strings.EqualFold(host.Status, "offline") {
		host.NetInRate = 0
		host.NetOutRate = 0
		host.DiskReadRate = 0
		host.DiskWriteRate = 0
		return
	}

	host.NetInRate = fluctuateMockHostRate(host.NetInRate, "network-in", 32*1024, 250*1024*1024)
	host.NetOutRate = fluctuateMockHostRate(host.NetOutRate, "network-out", 24*1024, 200*1024*1024)
	host.DiskReadRate = fluctuateMockHostRate(host.DiskReadRate, "disk-read", 16*1024, 120*1024*1024)
	host.DiskWriteRate = fluctuateMockHostRate(host.DiskWriteRate, "disk-write", 8*1024, 90*1024*1024)
}

func updateMockDockerHostRates(host *models.DockerHost) {
	if host == nil {
		return
	}
	if strings.EqualFold(host.Status, "offline") {
		host.NetInRate = 0
		host.NetOutRate = 0
		host.DiskReadRate = 0
		host.DiskWriteRate = 0
		return
	}

	host.NetInRate = fluctuateMockHostRate(host.NetInRate, "network-in", 32*1024, 250*1024*1024)
	host.NetOutRate = fluctuateMockHostRate(host.NetOutRate, "network-out", 24*1024, 200*1024*1024)
	host.DiskReadRate = fluctuateMockHostRate(host.DiskReadRate, "disk-read", 16*1024, 120*1024*1024)
	host.DiskWriteRate = fluctuateMockHostRate(host.DiskWriteRate, "disk-write", 8*1024, 90*1024*1024)
}

func generateDockerContainers(hostName string, hostIdx int, config MockConfig, podman bool) []models.DockerContainer {
	base := config.DockerContainersPerHost
	if base < 1 {
		base = 6
	}
	variation := rand.Intn(5) - 2
	count := base + variation
	if count < 3 {
		count = 3
	}

	now := time.Now()
	containers := make([]models.DockerContainer, 0, count)
	nameUsage := make(map[string]int)

	var podDefs []podDefinition
	infraAssigned := make(map[string]bool)
	if podman {
		maxPods := 1
		if count > 1 {
			if count < 3 {
				maxPods = count
			} else {
				maxPods = 3
			}
		}

		podCount := maxPods
		if maxPods > 1 {
			podCount = 1 + rand.Intn(maxPods)
		}

		podDefs = make([]podDefinition, podCount)
		baseProject := strings.ReplaceAll(hostName, "-", "")
		for idx := 0; idx < podCount; idx++ {
			composeProject := ""
			composeWorkdir := ""
			if rand.Float64() < 0.7 {
				composeProject = fmt.Sprintf("%s-stack", baseProject)
				composeWorkdir = fmt.Sprintf("/srv/%s", baseProject)
			}

			autoUpdatePolicy := ""
			autoUpdateRestart := ""
			if rand.Float64() < 0.35 {
				autoUpdatePolicy = []string{"image", "registry"}[rand.Intn(2)]
				if rand.Float64() < 0.5 {
					autoUpdateRestart = []string{"rolling", "daily"}[rand.Intn(2)]
				}
			}

			userNS := []string{"keep-id", "host", "private"}[rand.Intn(3)]

			podDefs[idx] = podDefinition{
				Name:              fmt.Sprintf("%s-pod-%d", baseProject, idx+1),
				ID:                randomHexString(24),
				ComposeProject:    composeProject,
				ComposeWorkdir:    composeWorkdir,
				ComposeConfigHash: randomHexString(16),
				AutoUpdatePolicy:  autoUpdatePolicy,
				AutoUpdateRestart: autoUpdateRestart,
				UserNamespace:     userNS,
			}
		}
	}

	for i := 0; i < count; i++ {
		baseName := appNames[rand.Intn(len(appNames))]
		nameUsage[baseName]++
		containerName := baseName
		if nameUsage[baseName] > 1 {
			containerName = fmt.Sprintf("%s-%d", baseName, nameUsage[baseName])
		}

		var pod *podDefinition
		isInfra := false
		if podman && len(podDefs) > 0 {
			pod = &podDefs[rand.Intn(len(podDefs))]
			if !infraAssigned[pod.ID] {
				isInfra = true
				infraAssigned[pod.ID] = true
			}
		}

		id := fmt.Sprintf("%s-%s", hostName, randomHexString(12))
		running := rand.Float64() >= config.StoppedPercent
		if running && rand.Float64() < 0.05 {
			running = false // small chance of paused/exited container regardless of stopped percent
		}

		memLimit := int64((256 + rand.Intn(4096)) * 1024 * 1024) // 256MB - ~4.25GB
		if memLimit < 256*1024*1024 {
			memLimit = 256 * 1024 * 1024
		}
		memPercent := clampFloat(20+rand.Float64()*65, 2, 96)
		if !running {
			memPercent = 0
		}
		memUsage := int64(float64(memLimit) * (memPercent / 100.0))

		cpuPercent := clampFloat(rand.Float64()*70, 0.2, 180)
		if !running {
			cpuPercent = 0
		}

		restartCount := rand.Intn(4)
		exitCode := 0
		health := "healthy"

		if rand.Float64() < 0.08 {
			health = "starting"
		}
		if rand.Float64() < 0.08 {
			health = "unhealthy"
		}

		var startedAt *time.Time
		var finishedAt *time.Time
		uptime := int64(0)
		state := "running"
		statusText := ""
		createdAt := now.Add(-time.Duration(rand.Intn(60*24)) * time.Hour)

		if running {
			uptime = int64(3600 + rand.Intn(86400*14)) // 1 hour to ~14 days
			start := now.Add(-time.Duration(uptime) * time.Second)
			startedAt = &start
			statusText = fmt.Sprintf("Up %s", formatDurationForStatus(time.Duration(uptime)*time.Second))
		} else {
			stateOptions := []string{"exited", "paused"}
			state = stateOptions[rand.Intn(len(stateOptions))]
			if state == "exited" {
				exitCode = []int{0, 1, 137, 139}[rand.Intn(4)]
				restartCount = 1 + rand.Intn(4)
				finished := now.Add(-time.Duration(rand.Intn(72)+1) * time.Hour)
				finishedAt = &finished
				statusText = fmt.Sprintf("Exited (%d) %s ago", exitCode, formatDurationForStatus(now.Sub(finished)))
				health = ""
			} else {
				statusText = "Paused"
				if rand.Float64() < 0.3 {
					health = ""
				}
			}
		}

		labels := generateDockerLabels(containerName, hostName, podman && pod != nil, pod, isInfra)

		container := models.DockerContainer{
			ID:            id,
			Name:          containerName,
			Image:         fmt.Sprintf("ghcr.io/mock/%s:%d.%d.%d", containerName, 1+rand.Intn(2), rand.Intn(10), rand.Intn(10)),
			State:         state,
			Status:        statusText,
			Health:        health,
			CPUPercent:    cpuPercent,
			MemoryUsage:   memUsage,
			MemoryLimit:   memLimit,
			MemoryPercent: memPercent,
			UptimeSeconds: uptime,
			RestartCount:  restartCount,
			ExitCode:      exitCode,
			CreatedAt:     createdAt,
			StartedAt:     startedAt,
			FinishedAt:    finishedAt,
			Ports:         generateDockerPorts(),
			Labels:        labels,
			Networks:      generateDockerNetworks(hostIdx, i),
		}

		if pod != nil {
			container.Podman = &models.DockerPodmanContainer{
				PodName:           pod.Name,
				PodID:             pod.ID,
				Infra:             isInfra,
				ComposeProject:    pod.ComposeProject,
				ComposeService:    containerName,
				ComposeWorkdir:    pod.ComposeWorkdir,
				ComposeConfigHash: pod.ComposeConfigHash,
				AutoUpdatePolicy:  pod.AutoUpdatePolicy,
				AutoUpdateRestart: pod.AutoUpdateRestart,
				UserNamespace:     pod.UserNamespace,
			}
		}

		containers = append(containers, container)
	}

	return containers
}

func generateDockerPorts() []models.DockerContainerPort {
	if rand.Float64() < 0.45 {
		return nil
	}

	portChoices := []int{80, 443, 3000, 3306, 5432, 6379, 8080, 9000}
	count := 1 + rand.Intn(2)
	ports := make([]models.DockerContainerPort, 0, count)
	used := make(map[int]bool)

	for len(ports) < count && len(used) < len(portChoices) {
		private := portChoices[rand.Intn(len(portChoices))]
		if used[private] {
			continue
		}
		used[private] = true

		port := models.DockerContainerPort{
			PrivatePort: private,
			Protocol:    []string{"tcp", "udp"}[rand.Intn(2)],
		}

		if rand.Float64() < 0.75 {
			port.PublicPort = 20000 + rand.Intn(20000)
			port.IP = "0.0.0.0"
		}

		ports = append(ports, port)
	}

	return ports
}

func generateDockerNetworks(hostIdx, containerIdx int) []models.DockerContainerNetworkLink {
	networks := []models.DockerContainerNetworkLink{{
		Name: "bridge",
		IPv4: fmt.Sprintf("172.18.%d.%d", hostIdx+10, containerIdx+20),
	}}

	if rand.Float64() < 0.3 {
		networks = append(networks, models.DockerContainerNetworkLink{
			Name: "frontend",
			IPv4: fmt.Sprintf("10.%d.%d.%d", hostIdx+20, containerIdx+10, rand.Intn(200)+20),
		})
	}

	if rand.Float64() < 0.2 {
		networks[len(networks)-1].IPv6 = fmt.Sprintf("fd00:%x:%x::%x", hostIdx+1, containerIdx+1, rand.Intn(4000))
	}

	return networks
}

func generateDockerLabels(serviceName, hostName string, podman bool, pod *podDefinition, infra bool) map[string]string {
	if podman && pod != nil {
		labels := map[string]string{
			"io.podman.annotations.pod.name": pod.Name,
			"io.podman.annotations.pod.id":   pod.ID,
		}

		if infra {
			labels["io.podman.annotations.pod.infra"] = "true"
		} else {
			labels["io.podman.annotations.pod.infra"] = "false"
		}

		if pod.ComposeProject != "" {
			labels["io.podman.compose.project"] = pod.ComposeProject
			labels["io.podman.compose.service"] = serviceName
			if pod.ComposeWorkdir != "" {
				labels["io.podman.compose.working_dir"] = pod.ComposeWorkdir
			}
			if pod.ComposeConfigHash != "" {
				labels["io.podman.compose.config-hash"] = pod.ComposeConfigHash
			}
		}

		if pod.AutoUpdatePolicy != "" {
			labels["io.containers.autoupdate"] = pod.AutoUpdatePolicy
		}
		if pod.AutoUpdateRestart != "" {
			labels["io.containers.autoupdate.restart"] = pod.AutoUpdateRestart
		}
		if pod.UserNamespace != "" {
			labels["io.podman.annotations.userns"] = pod.UserNamespace
		}

		if rand.Float64() < 0.25 {
			labels["io.containers.capabilities"] = "CHOWN,DAC_OVERRIDE,SETUID,SETGID"
		}
		if rand.Float64() < 0.2 {
			labels["environment"] = []string{"production", "staging", "development"}[rand.Intn(3)]
		}

		return labels
	}

	labels := map[string]string{
		"com.docker.compose.project": hostName,
		"com.docker.compose.service": serviceName,
	}

	if rand.Float64() < 0.25 {
		labels["environment"] = []string{"production", "staging", "development"}[rand.Intn(3)]
	}

	if rand.Float64() < 0.2 {
		labels["traefik.enable"] = "true"
	}

	return labels
}

func formatDurationForStatus(d time.Duration) string {
	if d < 0 {
		d = -d
	}

	if d < time.Minute {
		return "less than a minute"
	}

	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}

	if d < 24*time.Hour {
		hours := int(d / time.Hour)
		minutes := int((d % time.Hour) / time.Minute)
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}

	days := int(d / (24 * time.Hour))
	hours := int((d % (24 * time.Hour)) / time.Hour)
	if hours == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd%dh", days, hours)
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// naturalMetricUpdate produces realistic time-driven metric values that mimic
// real infrastructure behavior. CPU-like metrics (speed >= 0.8) sit at a low
// baseline with occasional sharp spikes — matching how real servers behave
// (mostly idle, with bursts of activity). Memory uses stable plateaus with
// slow drift. Disk changes very gradually.
//
// speed controls metric character: 1.0 = CPU (low baseline + spikes),
// ~0.5 = memory (stable plateau), ~0.12 = disk (very gradual drift).
func naturalMetricUpdate(current, min, max float64, resourceID, metric string, speed float64) float64 {
	seed := mockStableHash64(resourceID, metric)
	now := time.Now()
	span := math.Max(1, max-min)
	if speed <= 0 {
		speed = 1.0
	}

	// Time-varying noise seed (unique per tick).
	noiseSeed := seed ^ uint64(now.UnixNano()/int64(time.Millisecond))
	rng := rand.New(rand.NewSource(int64(noiseSeed)))

	// --- Baseline target ---
	// Shifts periodically, but stays in a characteristic range for each
	// metric type. The personality (seed) determines where in the range
	// each resource sits, so different resources have different baselines.
	targetWindow := int64(float64(300+seed%420) / math.Max(speed, 0.1))
	if targetWindow < 60 {
		targetWindow = 60
	}
	targetSeed := seed ^ uint64(now.Unix()/targetWindow)
	targetRng := rand.New(rand.NewSource(int64(targetSeed)))

	isCPULike := speed >= 0.8
	var baselineFraction float64
	if isCPULike {
		// CPU: most resources idle low. Personality determines the tier.
		// ~75% low (5-18%), ~20% moderate (18-32%), ~5% busy (32-48%).
		personality := seed % 20
		r := targetRng.Float64()
		switch {
		case personality == 0: // 5%: busy resource
			baselineFraction = 0.32 + r*0.16
		case personality <= 4: // 20%: moderate load
			baselineFraction = 0.18 + r*0.14
		default: // 75%: low/idle
			baselineFraction = 0.05 + r*0.13
		}
	} else if speed >= 0.3 {
		// Memory: moderate stable level.
		baselineFraction = 0.28 + targetRng.Float64()*0.37
	} else {
		// Disk: moderate, very stable.
		baselineFraction = 0.22 + targetRng.Float64()*0.42
	}
	baseTarget := min + span*baselineFraction

	// --- Spike events (CPU-like metrics only) ---
	// Each time bucket is deterministically either a spike or not.
	// Spikes ramp up fast and decay gradually, like real CPU bursts.
	var spikeValue float64
	if isCPULike {
		spikeBucket := int64(25 + seed%75) // 25-100 second spike windows
		bucketSeed := seed ^ uint64(now.Unix()/spikeBucket)
		bucketRng := rand.New(rand.NewSource(int64(bucketSeed)))

		// ~12-22% of buckets are spike periods (varies per resource).
		spikeFreq := 0.10 + float64(seed%12)*0.01
		if bucketRng.Float64() < spikeFreq {
			// Spike height: 15-50% of range above baseline.
			spikeHeight := span * (0.12 + bucketRng.Float64()*0.38)
			// Position within bucket: fast rise (~12%), then power-law decay.
			progress := float64(now.Unix()%spikeBucket) / float64(spikeBucket)
			if progress < 0.12 {
				spikeValue = spikeHeight * (progress / 0.12)
			} else {
				decay := (progress - 0.12) / 0.88
				spikeValue = spikeHeight * math.Pow(1-decay, 1.5)
			}
		}
	}

	// --- Smoothing toward ideal ---
	// Exponential smoothing: fast tracking during spikes so the value
	// actually reaches the peak, slower drift otherwise.
	ideal := baseTarget + spikeValue
	alpha := 0.06 * speed
	if spikeValue > span*0.03 {
		alpha = 0.30 // snap toward spike quickly
	}
	if alpha < 0.005 {
		alpha = 0.005
	}
	if alpha > 0.5 {
		alpha = 0.5
	}
	newValue := current + alpha*(ideal-current)

	// --- Per-tick noise ---
	noiseScale := span * 0.003 * math.Min(speed, 1.0)
	newValue += rng.NormFloat64() * noiseScale

	// --- Step quantization for fast metrics ---
	if speed >= 0.25 {
		step := span * (0.004 + float64(seed%3)*0.001)
		if step > 0.05 {
			newValue = math.Round(newValue/step) * step
		}
	}

	return clampFloat(newValue, min, max)
}

func randomHexString(n int) string {
	const hexChars = "0123456789abcdef"
	if n <= 0 {
		return ""
	}

	b := make([]byte, n)
	for i := range b {
		b[i] = hexChars[rand.Intn(len(hexChars))]
	}
	return string(b)
}

func humanizeHostDisplayName(hostname string) string {
	parts := strings.Split(hostname, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
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
		balloon := int64(0)
		// LXC containers typically don't use ballooning in the same way, keep it 0 for clear "Active" usage

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

	ct := models.Container{
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
		LastBackup: generateLastBackupTime(),
	}

	if status != "running" {
		ct.CPU = 0
		ct.Memory.Usage = 0
		ct.Memory.SwapUsed = 0
		ct.Memory.Used = 0
		ct.Memory.Free = ct.Memory.Total
		ct.Disk.Used = 0
		ct.Disk.Free = ct.Disk.Total
		ct.Disk.Usage = -1
		ct.NetworkIn = 0
		ct.NetworkOut = 0
		ct.DiskRead = 0
		ct.DiskWrite = 0
		ct.Uptime = 0
	}

	return ct
}

func generateGuestName(prefix string) string {
	return fmt.Sprintf("%s-%s-%d", prefix, appNames[rand.Intn(len(appNames))], rand.Intn(100))
}

// generateLastBackupTime generates a realistic last backup timestamp.
// Distribution: 70% within 24h (fresh), 15% within 72h (stale), 10% older (critical), 5% never backed up
func generateLastBackupTime() time.Time {
	r := rand.Float64()
	now := time.Now()

	if r < 0.05 {
		// 5% never backed up - return zero time
		return time.Time{}
	} else if r < 0.15 {
		// 10% critical - backup 4-30 days ago
		hoursAgo := 96 + rand.Intn(624) // 4-30 days in hours
		return now.Add(-time.Duration(hoursAgo) * time.Hour)
	} else if r < 0.30 {
		// 15% stale - backup 24-72 hours ago
		hoursAgo := 24 + rand.Intn(48)
		return now.Add(-time.Duration(hoursAgo) * time.Hour)
	} else {
		// 70% fresh - backup within last 24 hours
		hoursAgo := rand.Intn(24)
		return now.Add(-time.Duration(hoursAgo) * time.Hour)
	}
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
		isOffline := node.Status != "online" || node.ConnectionHealth == "offline" || node.Uptime <= 0
		// Local storage (always present)
		localTotal := int64(500 * 1024 * 1024 * 1024) // 500GB
		localUsed := int64(float64(localTotal) * (0.3 + rand.Float64()*0.5))
		if isOffline {
			localUsed = 0
		}
		localFree := localTotal - localUsed
		localUsage := 0.0
		if localTotal > 0 {
			localUsage = float64(localUsed) / float64(localTotal) * 100
		}
		localStatus := "available"
		localEnabled := true
		localActive := true
		if isOffline {
			localStatus = "offline"
			localEnabled = false
			localActive = false
		}

		storage = append(storage, models.Storage{
			ID:       fmt.Sprintf("%s-%s-local", node.Instance, node.Name),
			Name:     "local",
			Node:     node.Name,
			Instance: node.Instance,
			Type:     "dir",
			Status:   localStatus,
			Total:    localTotal,
			Used:     localUsed,
			Free:     localFree,
			Usage:    localUsage,
			Content:  "vztmpl,iso",
			Shared:   false,
			Enabled:  localEnabled,
			Active:   localActive,
		})

		// Local-zfs (common)
		zfsTotal := int64(2 * 1024 * 1024 * 1024 * 1024) // 2TB
		zfsUsed := int64(float64(zfsTotal) * (0.2 + rand.Float64()*0.6))
		if isOffline {
			zfsUsed = 0
		}
		zfsFree := zfsTotal - zfsUsed
		zfsUsage := 0.0
		if zfsTotal > 0 {
			zfsUsage = float64(zfsUsed) / float64(zfsTotal) * 100
		}

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

		zfsStatus := "available"
		zfsEnabled := true
		zfsActive := true
		if isOffline {
			zfsStatus = "offline"
			zfsEnabled = false
			zfsActive = false
		}

		storage = append(storage, models.Storage{
			ID:       fmt.Sprintf("%s-%s-local-zfs", node.Instance, node.Name),
			Name:     "local-zfs",
			Node:     node.Name,
			Instance: node.Instance,
			Type:     "zfspool",
			Status:   zfsStatus,
			Total:    zfsTotal,
			Used:     zfsUsed,
			Free:     zfsFree,
			Usage:    zfsUsage,
			Content:  "images,rootdir",
			Shared:   false,
			Enabled:  zfsEnabled,
			Active:   zfsActive,
			ZFSPool:  zfsPool,
		})

		// Add one more random storage per node
		if rand.Float64() > 0.3 {
			storageType := storageTypes[rand.Intn(len(storageTypes))]
			storageName := fmt.Sprintf("storage-%s-%d", node.Name, rand.Intn(100))
			total := int64((1 + rand.Intn(10)) * 1024 * 1024 * 1024 * 1024) // 1-10TB
			used := int64(float64(total) * rand.Float64())
			if isOffline {
				used = 0
			}
			free := total - used
			usage := 0.0
			if total > 0 {
				usage = float64(used) / float64(total) * 100
			}

			status := "available"
			enabled := true
			active := true
			if isOffline {
				status = "offline"
				enabled = false
				active = false
			}

			storage = append(storage, models.Storage{
				ID:       fmt.Sprintf("%s-%s-%s", node.Instance, node.Name, storageName),
				Name:     storageName,
				Node:     node.Name,
				Instance: node.Instance,
				Type:     storageType,
				Status:   status,
				Total:    total,
				Used:     used,
				Free:     free,
				Usage:    usage,
				Content:  contentTypes[rand.Intn(len(contentTypes))],
				Shared:   storageType == "nfs" || storageType == "cephfs",
				Enabled:  enabled,
				Active:   active,
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

	if len(clusterNodes) > 0 {
		nodeNames := make([]string, 0, len(clusterNodes))
		nodeIDs := make([]string, 0, len(clusterNodes))
		for _, clusterNode := range clusterNodes {
			nodeNames = append(nodeNames, clusterNode.Name)
			nodeIDs = append(nodeIDs, fmt.Sprintf("%s-%s", clusterNode.Instance, clusterNode.Name))
		}

		cephTotal := int64(120 * 1024 * 1024 * 1024 * 1024) // 120 TiB shared CephFS
		cephUsed := int64(float64(cephTotal) * (0.52 + rand.Float64()*0.18))
		storage = append(storage, models.Storage{
			ID:        fmt.Sprintf("%s-shared-cephfs", clusterNodes[0].Instance),
			Name:      "cephfs-shared",
			Node:      "shared",
			Instance:  clusterNodes[0].Instance,
			Type:      "cephfs",
			Status:    "available",
			Total:     cephTotal,
			Used:      cephUsed,
			Free:      cephTotal - cephUsed,
			Usage:     float64(cephUsed) / float64(cephTotal) * 100,
			Content:   "images,rootdir,backup",
			Shared:    true,
			Enabled:   true,
			Active:    true,
			Nodes:     nodeNames,
			NodeIDs:   nodeIDs,
			NodeCount: len(nodeNames),
		})

		rbdTotal := int64(80 * 1024 * 1024 * 1024 * 1024) // 80 TiB shared RBD pool
		rbdUsed := int64(float64(rbdTotal) * (0.48 + rand.Float64()*0.22))
		storage = append(storage, models.Storage{
			ID:        fmt.Sprintf("%s-shared-rbd", clusterNodes[0].Instance),
			Name:      "ceph-rbd-pool",
			Node:      "shared",
			Instance:  clusterNodes[0].Instance,
			Type:      "rbd",
			Status:    "available",
			Total:     rbdTotal,
			Used:      rbdUsed,
			Free:      rbdTotal - rbdUsed,
			Usage:     float64(rbdUsed) / float64(rbdTotal) * 100,
			Content:   "images,rootdir",
			Shared:    true,
			Enabled:   true,
			Active:    true,
			Nodes:     nodeNames,
			NodeIDs:   nodeIDs,
			NodeCount: len(nodeNames),
		})
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

func generateCephClusters(nodes []models.Node, storage []models.Storage) []models.CephCluster {
	cephStorageByInstance := make(map[string][]models.Storage)
	for _, st := range storage {
		typeLower := strings.ToLower(strings.TrimSpace(st.Type))
		if typeLower == "cephfs" || typeLower == "rbd" || typeLower == "ceph" {
			cephStorageByInstance[st.Instance] = append(cephStorageByInstance[st.Instance], st)
		}
	}

	if len(cephStorageByInstance) == 0 {
		return nil
	}

	nodesByInstance := make(map[string][]models.Node)
	for _, node := range nodes {
		nodesByInstance[node.Instance] = append(nodesByInstance[node.Instance], node)
	}

	var clusters []models.CephCluster
	for instanceName, cephStorages := range cephStorageByInstance {
		instanceNodes := nodesByInstance[instanceName]
		uniqueNodeNames := make(map[string]struct{})
		for _, node := range instanceNodes {
			if node.Name != "" {
				uniqueNodeNames[node.Name] = struct{}{}
			}
		}

		var totalBytes int64
		var usedBytes int64
		for _, st := range cephStorages {
			totalBytes += st.Total
			usedBytes += st.Used
		}
		if totalBytes == 0 {
			totalBytes = int64(120 * 1024 * 1024 * 1024 * 1024)
		}
		if usedBytes == 0 {
			usedBytes = int64(float64(totalBytes) * 0.52)
		}
		availableBytes := totalBytes - usedBytes
		usagePercent := float64(usedBytes) / float64(totalBytes) * 100

		health := "HEALTH_OK"
		healthMessage := ""
		if rand.Float64() < 0.2 {
			health = "HEALTH_WARN"
			healthMessage = "1 PG degraded"
		}

		pools := make([]models.CephPool, 0, len(cephStorages))
		for idx, st := range cephStorages {
			percent := 0.0
			if st.Total > 0 {
				percent = float64(st.Used) / float64(st.Total) * 100
			}
			poolName := st.Name
			if poolName == "" {
				if idx == 0 {
					poolName = "rbd"
				} else if idx == 1 {
					poolName = "cephfs-data"
				} else {
					poolName = fmt.Sprintf("pool-%d", idx+1)
				}
			}
			pools = append(pools, models.CephPool{
				ID:             idx + 1,
				Name:           poolName,
				StoredBytes:    st.Used,
				AvailableBytes: st.Free,
				Objects:        int64(1_200_000 + rand.Intn(800_000)),
				PercentUsed:    percent,
			})
		}

		numMons := 1
		if len(uniqueNodeNames) >= 3 {
			numMons = 3
		} else if len(uniqueNodeNames) == 2 {
			numMons = 2
		}

		numMgrs := 1
		if len(uniqueNodeNames) > 1 {
			numMgrs = 2
		}

		numOSDs := maxInt(len(uniqueNodeNames)*3, 6)
		numOSDsUp := numOSDs
		if health != "HEALTH_OK" && numOSDsUp > 0 {
			numOSDsUp--
		}

		services := []models.CephServiceStatus{
			{Type: "mon", Running: numMons, Total: numMons},
			{Type: "mgr", Running: numMgrs, Total: numMgrs},
		}

		if len(uniqueNodeNames) > 1 {
			mdsRunning := 2
			mdsTotal := 2
			if rand.Float64() < 0.25 {
				mdsRunning = 1
			}
			mds := models.CephServiceStatus{Type: "mds", Running: mdsRunning, Total: mdsTotal}
			if mdsRunning < mdsTotal {
				mds.Message = "1 standby"
			}
			services = append(services, mds)
		}

		cluster := models.CephCluster{
			ID:             fmt.Sprintf("%s-ceph", instanceName),
			Instance:       instanceName,
			Name:           fmt.Sprintf("%s Ceph", titleCase(instanceName)),
			FSID:           fmt.Sprintf("00000000-0000-4000-8000-%012d", rand.Int63n(1_000_000_000_000)),
			Health:         health,
			HealthMessage:  healthMessage,
			TotalBytes:     totalBytes,
			UsedBytes:      usedBytes,
			AvailableBytes: availableBytes,
			UsagePercent:   usagePercent,
			NumMons:        numMons,
			NumMgrs:        numMgrs,
			NumOSDs:        numOSDs,
			NumOSDsUp:      numOSDsUp,
			NumOSDsIn:      numOSDs,
			NumPGs:         512 + rand.Intn(256),
			Pools:          pools,
			Services:       services,
			LastUpdated:    time.Now().Add(-time.Duration(rand.Intn(180)) * time.Second),
		}

		clusters = append(clusters, cluster)
	}

	return clusters
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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

		// Generate 4-10 backups per VM spread across the past year
		numBackups := 4 + rand.Intn(7)
		for i := 0; i < numBackups; i++ {
			backupTime := time.Now().Add(-time.Duration(rand.Intn(365*24)) * time.Hour)
			backupSize := vm.Disk.Total/10 + rand.Int63n(vm.Disk.Total/5) // 10-30% of disk size

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

		// Generate 3-8 backups per container spread across the past year
		numBackups := 3 + rand.Intn(6)
		for i := 0; i < numBackups; i++ {
			backupTime := time.Now().Add(-time.Duration(rand.Intn(365*24)) * time.Hour)
			backupSize := ct.Disk.Total/20 + rand.Int63n(ct.Disk.Total/10) // 5-15% of disk size

			backup := models.StorageBackup{
				ID:        fmt.Sprintf("backup-%s-ct-%d-%d", ct.Node, ct.VMID, i),
				Storage:   "local",
				Node:      ct.Node,
				Instance:  ct.Instance,
				Type:      "lxc",
				VMID:      ct.VMID,
				Time:      backupTime,
				CTime:     backupTime.Unix(),
				Size:      backupSize,
				Format:    "tar.zst",
				Notes:     fmt.Sprintf("Backup of %s", ct.Name),
				Protected: rand.Float64() > 0.9, // 10% protected
				Volid:     fmt.Sprintf("local:backup/vzdump-lxc-%d-%s.tar.zst", ct.VMID, backupTime.Format("2006_01_02-15_04_05")),
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
	// Add 5-12 PMG host backups spread across the past year
	numPMGBackups := 5 + rand.Intn(8)
	pmgNodes := []string{"pmg-01", "pmg-02", "mail-gateway"}
	for i := 0; i < numPMGBackups; i++ {
		backupTime := time.Now().Add(-time.Duration(rand.Intn(365*24)) * time.Hour)
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

func extractPMGBackups(storageBackups []models.StorageBackup) []models.PMGBackup {
	pmgBackups := make([]models.PMGBackup, 0)
	for _, backup := range storageBackups {
		if !strings.EqualFold(backup.Type, "host") {
			continue
		}
		format := strings.ToLower(backup.Format)
		notes := strings.ToLower(backup.Notes)
		if !strings.Contains(format, "pmg") && !strings.Contains(notes, "pmg") {
			continue
		}

		filename := backup.Volid
		if filename == "" {
			filename = backup.Notes
		}
		instance := backup.Instance
		if instance == "" && backup.Node != "" {
			instance = fmt.Sprintf("PMG:%s", backup.Node)
		}
		if instance == "" {
			instance = "PMG:mock"
		}

		pmgBackups = append(pmgBackups, models.PMGBackup{
			ID:         backup.ID,
			Instance:   instance,
			Node:       backup.Node,
			Filename:   filename,
			BackupTime: backup.Time,
			Size:       backup.Size,
		})
	}

	sort.Slice(pmgBackups, func(i, j int) bool {
		return pmgBackups[i].BackupTime.After(pmgBackups[j].BackupTime)
	})

	return pmgBackups
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

		// Generate 5-12 PBS backups per VM spread across the past year
		numBackups := 5 + rand.Intn(8)
		for i := 0; i < numBackups; i++ {
			backupTime := time.Now().Add(-time.Duration(rand.Intn(365*24)) * time.Hour)

			backup := models.PBSBackup{
				ID:         fmt.Sprintf("pbs-backup-vm-%d-%d", vm.VMID, i),
				Instance:   pbsInstances[rand.Intn(len(pbsInstances))],
				Datastore:  datastores[rand.Intn(len(datastores))],
				Namespace:  "root",
				BackupType: "vm",
				VMID:       fmt.Sprintf("%d", vm.VMID),
				BackupTime: backupTime,
				Size:       vm.Disk.Total/8 + rand.Int63n(vm.Disk.Total/4),
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

		// Generate 4-10 PBS backups per container spread across the past year
		numBackups := 4 + rand.Intn(7)
		for i := 0; i < numBackups; i++ {
			backupTime := time.Now().Add(-time.Duration(rand.Intn(365*24)) * time.Hour)

			backup := models.PBSBackup{
				ID:         fmt.Sprintf("pbs-backup-ct-%d-%d", ct.VMID, i),
				Instance:   pbsInstances[rand.Intn(len(pbsInstances))],
				Datastore:  datastores[rand.Intn(len(datastores))],
				Namespace:  "root",
				BackupType: "ct",
				VMID:       fmt.Sprintf("%d", ct.VMID),
				BackupTime: backupTime,
				Size:       ct.Disk.Total/15 + rand.Int63n(ct.Disk.Total/8),
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
	hostBackupCount := 5 + rand.Intn(6) // 5-10 host backups
	for i := 0; i < hostBackupCount; i++ {
		backupTime := time.Now().Add(-time.Duration(rand.Intn(365*24)) * time.Hour)

		backup := models.PBSBackup{
			ID:         fmt.Sprintf("pbs-backup-host-0-%d", i),
			Instance:   pbsInstances[rand.Intn(len(pbsInstances))],
			Datastore:  datastores[rand.Intn(len(datastores))],
			Namespace:  "root",
			BackupType: "ct", // Host configs are stored as 'ct' type in PBS
			VMID:       "0",  // VMID 0 indicates host config
			BackupTime: backupTime,
			Size:       50*1024*1024 + rand.Int63n(100*1024*1024), // 50-150MB for host configs
			Protected:  rand.Float64() > 0.7,                      // 30% protected
			Verified:   rand.Float64() > 0.1,                      // 90% verified
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

func generatePMGInstances() []models.PMGInstance {
	now := time.Now()
	mailStats := models.PMGMailStats{
		Timeframe:            "day",
		CountTotal:           2800 + rand.Float64()*400,
		CountIn:              1800 + rand.Float64()*200,
		CountOut:             900 + rand.Float64()*100,
		SpamIn:               320 + rand.Float64()*40,
		SpamOut:              45 + rand.Float64()*10,
		VirusIn:              12 + rand.Float64()*3,
		VirusOut:             3 + rand.Float64()*2,
		BouncesIn:            18 + rand.Float64()*5,
		BouncesOut:           6 + rand.Float64()*3,
		BytesIn:              9.6e9 + rand.Float64()*1.5e9,
		BytesOut:             3.2e9 + rand.Float64()*0.8e9,
		GreylistCount:        210 + rand.Float64()*40,
		JunkIn:               120 + rand.Float64()*20,
		AverageProcessTimeMs: 480 + rand.Float64()*120,
		RBLRejects:           140 + rand.Float64()*30,
		PregreetRejects:      60 + rand.Float64()*15,
		UpdatedAt:            now,
	}

	mailPoints := make([]models.PMGMailCountPoint, 0, 24)
	for i := 0; i < 24; i++ {
		pointTime := now.Add(-time.Duration(23-i) * time.Hour)
		baseCount := 80 + rand.Float64()*25
		mailPoints = append(mailPoints, models.PMGMailCountPoint{
			Timestamp:   pointTime,
			Count:       baseCount + rand.Float64()*20,
			CountIn:     baseCount*0.65 + rand.Float64()*10,
			CountOut:    baseCount*0.35 + rand.Float64()*5,
			SpamIn:      baseCount*0.12 + rand.Float64()*4,
			SpamOut:     baseCount*0.02 + rand.Float64()*1,
			VirusIn:     rand.Float64() * 2,
			VirusOut:    rand.Float64(),
			RBLRejects:  rand.Float64() * 5,
			Pregreet:    rand.Float64() * 3,
			BouncesIn:   rand.Float64() * 4,
			BouncesOut:  rand.Float64() * 2,
			Greylist:    rand.Float64() * 6,
			Index:       i,
			Timeframe:   "hour",
			WindowStart: pointTime,
			WindowEnd:   pointTime.Add(time.Hour),
		})
	}

	spamBuckets := []models.PMGSpamBucket{
		{Score: "0-2", Count: 950 + rand.Float64()*40},
		{Score: "3-5", Count: 420 + rand.Float64()*25},
		{Score: "6-8", Count: 180 + rand.Float64()*15},
		{Score: "9-10", Count: 70 + rand.Float64()*10},
	}

	quarantine := models.PMGQuarantineTotals{
		Spam:        25 + rand.Intn(30),
		Virus:       2 + rand.Intn(6),
		Attachment:  4 + rand.Intn(6),
		Blacklisted: rand.Intn(8),
	}

	nodes := []models.PMGNodeStatus{
		{
			Name:    "pmg-primary",
			Status:  "online",
			Role:    "master",
			Uptime:  int64(86400 * 18),
			LoadAvg: fmt.Sprintf("%.2f", 0.75+rand.Float64()*0.25),
			QueueStatus: &models.PMGQueueStatus{
				Active:    rand.Intn(5),
				Deferred:  rand.Intn(15),
				Hold:      rand.Intn(3),
				Incoming:  rand.Intn(8),
				Total:     0, // Will be calculated below
				OldestAge: int64(rand.Intn(3600)),
				UpdatedAt: now,
			},
		},
	}
	// Calculate total for primary node
	nodes[0].QueueStatus.Total = nodes[0].QueueStatus.Active + nodes[0].QueueStatus.Deferred +
		nodes[0].QueueStatus.Hold + nodes[0].QueueStatus.Incoming

	if rand.Float64() > 0.5 {
		queueStatus := &models.PMGQueueStatus{
			Active:    rand.Intn(3),
			Deferred:  rand.Intn(10),
			Hold:      rand.Intn(2),
			Incoming:  rand.Intn(5),
			Total:     0,
			OldestAge: int64(rand.Intn(1800)),
			UpdatedAt: now,
		}
		queueStatus.Total = queueStatus.Active + queueStatus.Deferred + queueStatus.Hold + queueStatus.Incoming

		nodes = append(nodes, models.PMGNodeStatus{
			Name:        "pmg-secondary",
			Status:      "online",
			Role:        "node",
			Uptime:      int64(86400 * 9),
			LoadAvg:     fmt.Sprintf("%.2f", 0.55+rand.Float64()*0.2),
			QueueStatus: queueStatus,
		})
	}

	primary := models.PMGInstance{
		ID:               "pmg-main",
		Name:             "pmg-main",
		Host:             "https://pmg.mock.lan:8006",
		Status:           "online",
		Version:          "8.1-2",
		Nodes:            nodes,
		MailStats:        &mailStats,
		MailCount:        mailPoints,
		SpamDistribution: spamBuckets,
		Quarantine:       &quarantine,
		ConnectionHealth: "healthy",
		LastSeen:         now,
		LastUpdated:      now,
	}

	instances := []models.PMGInstance{primary}

	if rand.Float64() > 0.65 {
		backupNow := now.Add(-6 * time.Hour)
		secondaryStats := mailStats
		secondaryStats.CountTotal *= 0.6
		secondaryStats.CountIn *= 0.6
		secondaryStats.CountOut *= 0.6
		secondaryStats.UpdatedAt = backupNow

		edgeQueue := &models.PMGQueueStatus{
			Active:    rand.Intn(2),
			Deferred:  rand.Intn(5),
			Hold:      rand.Intn(2), // rand.Intn(1) always returns 0
			Incoming:  rand.Intn(3),
			Total:     0,
			OldestAge: int64(rand.Intn(600)),
			UpdatedAt: backupNow,
		}
		edgeQueue.Total = edgeQueue.Active + edgeQueue.Deferred + edgeQueue.Hold + edgeQueue.Incoming

		instances = append(instances, models.PMGInstance{
			ID:      "pmg-edge",
			Name:    "pmg-edge",
			Host:    "https://pmg-edge.mock.lan:8006",
			Status:  "online",
			Version: "8.0-7",
			Nodes: []models.PMGNodeStatus{{
				Name:        "pmg-edge",
				Status:      "online",
				Role:        "standalone",
				Uptime:      int64(86400 * 4),
				QueueStatus: edgeQueue,
			}},
			MailStats:        &secondaryStats,
			MailCount:        mailPoints[:12],
			SpamDistribution: spamBuckets,
			Quarantine:       &models.PMGQuarantineTotals{Spam: 8 + rand.Intn(5), Virus: rand.Intn(3)},
			ConnectionHealth: "healthy",
			LastSeen:         backupNow,
			LastUpdated:      backupNow,
		})
	}

	return instances
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

		// Generate 3-8 snapshots per VM spread across the past year
		numSnapshots := 3 + rand.Intn(6)
		for i := 0; i < numSnapshots; i++ {
			snapshotTime := time.Now().Add(-time.Duration(rand.Intn(365*24)) * time.Hour)

			snapshot := models.GuestSnapshot{
				ID:          fmt.Sprintf("snapshot-%s-vm-%d-%d", vm.Node, vm.VMID, i),
				Name:        snapshotNames[rand.Intn(len(snapshotNames))],
				Node:        vm.Node,
				Instance:    vm.Instance,
				Type:        "qemu",
				VMID:        vm.VMID,
				Time:        snapshotTime,
				Description: fmt.Sprintf("Snapshot of %s taken on %s", vm.Name, snapshotTime.Format("2006-01-02")),
				VMState:     rand.Float64() > 0.5,          // 50% include VM state
				SizeBytes:   int64(10+rand.Intn(90)) << 30, // 10-99 GiB
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

		// Generate 2-6 snapshots per container spread across the past year
		numSnapshots := 2 + rand.Intn(5)
		for i := 0; i < numSnapshots; i++ {
			snapshotTime := time.Now().Add(-time.Duration(rand.Intn(365*24)) * time.Hour)

			snapshot := models.GuestSnapshot{
				ID:          fmt.Sprintf("snapshot-%s-ct-%d-%d", ct.Node, ct.VMID, i),
				Name:        snapshotNames[rand.Intn(len(snapshotNames))],
				Node:        ct.Node,
				Instance:    ct.Instance,
				Type:        "lxc",
				VMID:        ct.VMID,
				Time:        snapshotTime,
				Description: fmt.Sprintf("Container snapshot for %s", ct.Name),
				VMState:     false,                        // Containers don't have VM state
				SizeBytes:   int64(5+rand.Intn(40)) << 30, // 5-44 GiB
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
	updateDockerHosts(data, config)
	updateKubernetesClusters(data, config)
	updateHosts(data, config)
	syncMockKubernetesNodeHosts(data)

	if !config.RandomMetrics {
		return
	}

	// Update node metrics
	for i := range data.Nodes {
		node := &data.Nodes[i]
		node.CPU = naturalMetricUpdate(node.CPU, 0.05, 0.85, node.ID, "cpu", 1.0)

		node.Memory.Usage = naturalMetricUpdate(node.Memory.Usage, 10, 85, node.ID, "memory", 0.5)
		node.Memory.Used = int64(float64(node.Memory.Total) * (node.Memory.Usage / 100))
		node.Memory.Free = node.Memory.Total - node.Memory.Used
	}

	// Update VM metrics
	for i := range data.VMs {
		vm := &data.VMs[i]
		if vm.Status != "running" {
			continue
		}

		vm.CPU = naturalMetricUpdate(vm.CPU, 0.01, 0.85, vm.ID, "cpu", 1.0)

		vm.Memory.Usage = naturalMetricUpdate(vm.Memory.Usage, 10, 85, vm.ID, "memory", 0.5)
		vm.Memory.Used = int64(float64(vm.Memory.Total) * (vm.Memory.Usage / 100))
		vm.Memory.Free = vm.Memory.Total - vm.Memory.Used

		vm.Disk.Usage = naturalMetricUpdate(vm.Disk.Usage, 10, 90, vm.ID, "disk", 0.12)
		vm.Disk.Used = int64(float64(vm.Disk.Total) * (vm.Disk.Usage / 100))
		vm.Disk.Free = vm.Disk.Total - vm.Disk.Used

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

		ct.CPU = naturalMetricUpdate(ct.CPU, 0.01, 0.75, ct.ID, "cpu", 1.0)

		ct.Memory.Usage = naturalMetricUpdate(ct.Memory.Usage, 5, 85, ct.ID, "memory", 0.5)
		ct.Memory.Used = int64(float64(ct.Memory.Total) * (ct.Memory.Usage / 100))
		ct.Memory.Free = ct.Memory.Total - ct.Memory.Used

		ct.Disk.Usage = naturalMetricUpdate(ct.Disk.Usage, 5, 85, ct.ID, "disk", 0.10)
		ct.Disk.Used = int64(float64(ct.Disk.Total) * (ct.Disk.Usage / 100))
		ct.Disk.Free = ct.Disk.Total - ct.Disk.Used

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

	// Update PMG metrics with gentle fluctuations
	for i := range data.PMGInstances {
		inst := &data.PMGInstances[i]
		now := time.Now()
		inst.LastSeen = now
		inst.LastUpdated = now

		if inst.MailStats != nil {
			inst.MailStats.CountTotal = fluctuateFloat(inst.MailStats.CountTotal, 0.05, 0, math.MaxFloat64)
			inst.MailStats.CountIn = fluctuateFloat(inst.MailStats.CountIn, 0.05, 0, math.MaxFloat64)
			inst.MailStats.CountOut = fluctuateFloat(inst.MailStats.CountOut, 0.05, 0, math.MaxFloat64)
			inst.MailStats.SpamIn = fluctuateFloat(inst.MailStats.SpamIn, 0.08, 0, math.MaxFloat64)
			inst.MailStats.SpamOut = fluctuateFloat(inst.MailStats.SpamOut, 0.08, 0, math.MaxFloat64)
			inst.MailStats.VirusIn = fluctuateFloat(inst.MailStats.VirusIn, 0.1, 0, math.MaxFloat64)
			inst.MailStats.VirusOut = fluctuateFloat(inst.MailStats.VirusOut, 0.1, 0, math.MaxFloat64)
			inst.MailStats.RBLRejects = fluctuateFloat(inst.MailStats.RBLRejects, 0.07, 0, math.MaxFloat64)
			inst.MailStats.PregreetRejects = fluctuateFloat(inst.MailStats.PregreetRejects, 0.07, 0, math.MaxFloat64)
			inst.MailStats.GreylistCount = fluctuateFloat(inst.MailStats.GreylistCount, 0.05, 0, math.MaxFloat64)
			inst.MailStats.AverageProcessTimeMs = fluctuateFloat(inst.MailStats.AverageProcessTimeMs, 0.05, 200, 2000)
			inst.MailStats.UpdatedAt = now
		}

		if len(inst.MailCount) > 0 {
			// Drop oldest point if we already have 24
			if len(inst.MailCount) >= 24 {
				inst.MailCount = inst.MailCount[1:]
			}

			base := 60 + rand.Float64()*30
			newPoint := models.PMGMailCountPoint{
				Timestamp:  now,
				Count:      base + rand.Float64()*15,
				CountIn:    base*0.6 + rand.Float64()*10,
				CountOut:   base*0.4 + rand.Float64()*8,
				SpamIn:     base*0.1 + rand.Float64()*5,
				SpamOut:    base*0.02 + rand.Float64()*1,
				VirusIn:    rand.Float64() * 2,
				VirusOut:   rand.Float64(),
				RBLRejects: rand.Float64() * 4,
				Pregreet:   rand.Float64() * 3,
				BouncesIn:  rand.Float64() * 3,
				BouncesOut: rand.Float64() * 2,
				Greylist:   rand.Float64() * 5,
				Index:      len(inst.MailCount),
				Timeframe:  "hour",
			}
			inst.MailCount = append(inst.MailCount, newPoint)
		}

		if len(inst.SpamDistribution) > 0 {
			for j := range inst.SpamDistribution {
				inst.SpamDistribution[j].Count = fluctuateFloat(inst.SpamDistribution[j].Count, 0.05, 0, math.MaxFloat64)
			}
		}

		if inst.Quarantine != nil {
			inst.Quarantine.Spam = fluctuateInt(inst.Quarantine.Spam, 5, 0, 500)
			inst.Quarantine.Virus = fluctuateInt(inst.Quarantine.Virus, 2, 0, 200)
			inst.Quarantine.Attachment = fluctuateInt(inst.Quarantine.Attachment, 2, 0, 200)
			inst.Quarantine.Blacklisted = fluctuateInt(inst.Quarantine.Blacklisted, 1, 0, 100)
		}

		if len(inst.Nodes) > 0 {
			for j := range inst.Nodes {
				if inst.Nodes[j].Status == "online" {
					inst.Nodes[j].Uptime += int64(updateInterval.Seconds())
				}
			}
		}
	}

	data.LastUpdate = time.Now()
}

func updateKubernetesClusters(data *models.StateSnapshot, config MockConfig) {
	if len(data.KubernetesClusters) == 0 {
		return
	}

	now := time.Now()

	for i := range data.KubernetesClusters {
		cluster := &data.KubernetesClusters[i]

		if cluster.Status != "offline" {
			cluster.LastSeen = now.Add(-time.Duration(rand.Intn(12)) * time.Second)
		} else if config.RandomMetrics && rand.Float64() < 0.01 {
			cluster.Status = "online"
			cluster.LastSeen = now
			for nodeIdx := range cluster.Nodes {
				cluster.Nodes[nodeIdx].Ready = rand.Float64() > 0.12
			}
		}

		if config.RandomMetrics {
			// Small chance to flip a node Ready state.
			if len(cluster.Nodes) > 0 && rand.Float64() < 0.05 {
				idx := rand.Intn(len(cluster.Nodes))
				cluster.Nodes[idx].Ready = !cluster.Nodes[idx].Ready
			}

			// Small chance to flip a pod into/out of CrashLoopBackOff.
			if len(cluster.Pods) > 0 && rand.Float64() < 0.07 {
				idx := rand.Intn(len(cluster.Pods))
				pod := &cluster.Pods[idx]
				if kubernetesPodHealthy(*pod) {
					pod.Phase = "Running"
					pod.Reason = ""
					pod.Message = ""
					pod.Restarts += 1 + rand.Intn(3)
					for j := range pod.Containers {
						pod.Containers[j].Ready = false
						pod.Containers[j].State = "waiting"
						pod.Containers[j].Reason = "CrashLoopBackOff"
						pod.Containers[j].Message = "Back-off restarting failed container"
						pod.Containers[j].RestartCount += int32(1 + rand.Intn(3))
					}
				} else {
					pod.Phase = "Running"
					pod.Reason = ""
					pod.Message = ""
					for j := range pod.Containers {
						pod.Containers[j].Ready = true
						pod.Containers[j].State = "running"
						pod.Containers[j].Reason = ""
						pod.Containers[j].Message = ""
					}
				}
			}
		}

		if cluster.Status != "offline" {
			if clusterHasIssues(cluster.Nodes, cluster.Pods, cluster.Deployments) {
				cluster.Status = "degraded"
			} else {
				cluster.Status = "online"
			}
		}

		initializeMockKubernetesClusterUsage(cluster, now, config.RandomMetrics)

		if data.ConnectionHealth != nil {
			data.ConnectionHealth[kubernetesConnectionPrefix+cluster.ID] = cluster.Status != "offline"
		}
	}
}

func updateDockerHosts(data *models.StateSnapshot, config MockConfig) {
	if len(data.DockerHosts) == 0 {
		return
	}

	now := time.Now()
	step := int64(updateInterval.Seconds())
	if step <= 0 {
		step = 2
	}

	for i := range data.DockerHosts {
		host := &data.DockerHosts[i]

		if host.Status != "offline" {
			host.LastSeen = now.Add(-time.Duration(rand.Intn(6)) * time.Second)
			host.UptimeSeconds += step
			if config.RandomMetrics {
				updateMockDockerHostRates(host)
				host.CPUUsage = naturalMetricUpdate(host.CPUUsage, 3, 99, host.ID, "cpu", 1.0)
			}
			host.Temperature = generateMockTemperature(host.CPUUsage)
		} else if config.RandomMetrics && rand.Float64() < 0.01 {
			// Occasionally bring an offline host back online
			host.Status = "online"
			host.LastSeen = now
			updateMockDockerHostRates(host)
			host.CPUUsage = clampFloat(8+rand.Float64()*35, 3, 99)
			host.Temperature = generateMockTemperature(host.CPUUsage)
			if len(host.Containers) == 0 {
				isPodman := strings.EqualFold(host.Runtime, "podman")
				host.Containers = generateDockerContainers(host.Hostname, i, config, isPodman)
			}
		} else {
			host.NetInRate = 0
			host.NetOutRate = 0
			host.DiskReadRate = 0
			host.DiskWriteRate = 0
			host.Temperature = nil
		}

		running := 0
		flagged := 0

		for j := range host.Containers {
			container := &host.Containers[j]
			state := strings.ToLower(container.State)
			health := strings.ToLower(container.Health)

			if state != "running" {
				if health == "unhealthy" || health == "starting" {
					flagged++
				}

				if config.RandomMetrics && (state == "exited" || state == "paused") && rand.Float64() < 0.02 {
					container.State = "running"
					container.Status = "Up a few seconds"
					container.Health = "starting"
					container.CPUPercent = clampFloat(rand.Float64()*35, 2, 90)
					container.MemoryPercent = clampFloat(25+rand.Float64()*40, 5, 85)
					if container.MemoryLimit > 0 {
						container.MemoryUsage = int64(float64(container.MemoryLimit) * (container.MemoryPercent / 100.0))
					}
					container.UptimeSeconds = step
					start := now.Add(-time.Duration(container.UptimeSeconds) * time.Second)
					container.StartedAt = &start
					container.FinishedAt = nil
					state = "running"
					health = "starting"
				} else {
					continue
				}
			}

			if state == "running" {
				running++

				if config.RandomMetrics {
					container.CPUPercent = naturalMetricUpdate(container.CPUPercent, 0, 190, container.ID, "cpu", 1.0)
					container.MemoryPercent = naturalMetricUpdate(container.MemoryPercent, 1, 97, container.ID, "memory", 0.6)
					if container.MemoryLimit > 0 {
						container.MemoryUsage = int64(float64(container.MemoryLimit) * (container.MemoryPercent / 100.0))
					}

					container.UptimeSeconds += step

					switch health {
					case "unhealthy":
						if rand.Float64() < 0.3 {
							container.Health = "healthy"
							health = "healthy"
						}
					case "starting":
						if rand.Float64() < 0.5 {
							container.Health = "healthy"
							health = "healthy"
						}
					default:
						if rand.Float64() < 0.03 {
							container.Health = "unhealthy"
							health = "unhealthy"
						} else if rand.Float64() < 0.04 {
							container.Health = "starting"
							health = "starting"
						}
					}

					if rand.Float64() < 0.01 {
						container.RestartCount++
					}
				}

				if health == "unhealthy" || health == "starting" {
					flagged++
				}
			}
		}

		if data.ConnectionHealth != nil {
			data.ConnectionHealth[dockerConnectionPrefix+host.ID] = host.Status != "offline"
		}

		if len(host.Services) > 0 || len(host.Tasks) > 0 {
			recalculateDockerServiceHealth(host, now)
		}
		ensureDockerSwarmInfo(host)

		if host.Status == "offline" {
			host.NetInRate = 0
			host.NetOutRate = 0
			host.DiskReadRate = 0
			host.DiskWriteRate = 0
			host.Temperature = nil
			continue
		}

		total := len(host.Containers)
		if total == 0 {
			host.Status = "offline"
			host.Temperature = nil
			if data.ConnectionHealth != nil {
				data.ConnectionHealth[dockerConnectionPrefix+host.ID] = false
			}
			continue
		}

		if running == 0 {
			host.Status = "offline"
			host.LastSeen = now.Add(-90 * time.Second)
			host.Temperature = nil
			if data.ConnectionHealth != nil {
				data.ConnectionHealth[dockerConnectionPrefix+host.ID] = false
			}
			continue
		}

		stopped := total - running
		if flagged > 0 || float64(stopped)/float64(total) > 0.35 {
			host.Status = "degraded"
		} else {
			host.Status = "online"
		}
		host.Temperature = generateMockTemperature(host.CPUUsage)
	}
}

func serviceKey(id, name string) string {
	if id != "" {
		return id
	}
	return name
}

func recalculateDockerServiceHealth(host *models.DockerHost, now time.Time) {
	if host == nil {
		return
	}

	containerByID := make(map[string]models.DockerContainer, len(host.Containers))
	for _, container := range host.Containers {
		containerByID[container.ID] = container
		if len(container.ID) >= 12 {
			containerByID[container.ID[:12]] = container
		}
	}

	tasksByService := make(map[string][]int, len(host.Services))
	for idx := range host.Tasks {
		task := &host.Tasks[idx]
		key := serviceKey(task.ServiceID, task.ServiceName)
		tasksByService[key] = append(tasksByService[key], idx)

		container, ok := containerByID[task.ContainerID]
		if !ok {
			if host.Status == "offline" {
				task.CurrentState = "shutdown"
				if task.CompletedAt == nil {
					completed := now.Add(-time.Minute)
					task.CompletedAt = &completed
				}
			}
			continue
		}

		state := strings.ToLower(container.State)
		switch state {
		case "running":
			task.CurrentState = "running"
			if container.StartedAt != nil {
				started := *container.StartedAt
				task.StartedAt = &started
			}
			task.CompletedAt = nil
		case "paused":
			task.CurrentState = "paused"
			if container.StartedAt != nil {
				started := *container.StartedAt
				task.StartedAt = &started
			}
		default:
			task.CurrentState = state
			if container.FinishedAt != nil {
				finished := *container.FinishedAt
				task.CompletedAt = &finished
			} else if host.Status == "offline" {
				if task.CompletedAt == nil {
					finished := now.Add(-2 * time.Minute)
					task.CompletedAt = &finished
				}
			}
		}
	}

	for idx := range host.Services {
		service := &host.Services[idx]
		key := serviceKey(service.ID, service.Name)
		taskIdxs := tasksByService[key]

		if service.DesiredTasks <= 0 {
			service.DesiredTasks = len(taskIdxs)
		}

		running := 0
		completed := 0
		for _, taskIndex := range taskIdxs {
			task := host.Tasks[taskIndex]
			state := strings.ToLower(task.CurrentState)
			if state == "running" {
				running++
			}
			if task.CompletedAt != nil || state == "shutdown" || state == "failed" {
				completed++
			}
		}

		service.RunningTasks = running
		service.CompletedTasks = completed

		if service.DesiredTasks > 0 && running < service.DesiredTasks {
			if service.UpdateStatus == nil {
				service.UpdateStatus = &models.DockerServiceUpdate{}
			}
			service.UpdateStatus.State = "rollback_started"
			service.UpdateStatus.Message = "Service replicas below desired threshold"
			service.UpdateStatus.CompletedAt = nil
		} else if service.UpdateStatus != nil && running >= service.DesiredTasks {
			service.UpdateStatus = nil
		}
	}
}

func ensureDockerSwarmInfo(host *models.DockerHost) {
	if host == nil {
		return
	}

	if host.Swarm == nil {
		host.Swarm = &models.DockerSwarmInfo{
			NodeID:           fmt.Sprintf("%s-node", host.ID),
			NodeRole:         "worker",
			LocalState:       "active",
			ControlAvailable: false,
			Scope:            "node",
		}
	}

	if host.Swarm.NodeID == "" {
		host.Swarm.NodeID = fmt.Sprintf("%s-node", host.ID)
	}
	if host.Swarm.NodeRole == "" {
		host.Swarm.NodeRole = "worker"
	}

	if host.Status == "offline" {
		host.Swarm.LocalState = "inactive"
	} else {
		host.Swarm.LocalState = "active"
	}

	if host.Swarm.NodeRole == "manager" {
		if host.Swarm.ControlAvailable && len(host.Services) > 0 {
			host.Swarm.Scope = "cluster"
		} else {
			host.Swarm.Scope = "node"
		}
	} else {
		host.Swarm.Scope = "node"
		host.Swarm.ControlAvailable = false
	}
}

func updateHosts(data *models.StateSnapshot, config MockConfig) {
	if len(data.Hosts) == 0 {
		return
	}

	now := time.Now()
	step := int64(updateInterval.Seconds())
	if step <= 0 {
		step = 2
	}

	for i := range data.Hosts {
		host := &data.Hosts[i]

		if data.ConnectionHealth != nil {
			data.ConnectionHealth[hostConnectionPrefix+host.ID] = host.Status != "offline"
		}

		if host.Status == "offline" {
			host.NetInRate = 0
			host.NetOutRate = 0
			host.DiskReadRate = 0
			host.DiskWriteRate = 0
			if config.RandomMetrics && rand.Float64() < 0.02 {
				host.Status = "online"
				host.LastSeen = now
				host.UptimeSeconds = int64(120 + rand.Intn(3600))
				updateMockHostRates(host)
			}
			continue
		}

		host.LastSeen = now.Add(-time.Duration(rand.Intn(25)) * time.Second)
		host.UptimeSeconds += step

		if !config.RandomMetrics {
			continue
		}

		updateMockHostRates(host)

		host.CPUUsage = naturalMetricUpdate(host.CPUUsage, 4, 97, host.ID, "cpu", 1.0)

		host.Memory.Usage = naturalMetricUpdate(host.Memory.Usage, 12, 96, host.ID, "memory", 0.5)
		host.Memory.Used = int64(float64(host.Memory.Total) * (host.Memory.Usage / 100.0))
		host.Memory.Free = host.Memory.Total - host.Memory.Used

		for j := range host.Disks {
			diskID := fmt.Sprintf("%s-disk-%d", host.ID, j)
			host.Disks[j].Usage = naturalMetricUpdate(host.Disks[j].Usage, 5, 98, diskID, "disk", 0.12)
			host.Disks[j].Used = int64(float64(host.Disks[j].Total) * (host.Disks[j].Usage / 100.0))
			host.Disks[j].Free = host.Disks[j].Total - host.Disks[j].Used
		}

		if len(host.LoadAverage) == 3 {
			for j := range host.LoadAverage {
				host.LoadAverage[j] = clampFloat(host.LoadAverage[j]+(rand.Float64()-0.5)*0.4, 0.05, float64(host.CPUCount))
			}
		}

		if host.Status == "degraded" {
			if rand.Float64() < 0.25 {
				host.Status = "online"
			}
		} else if rand.Float64() < 0.05 {
			host.Status = "degraded"
		}
	}
}

func fluctuateFloat(value, variance, min, max float64) float64 {
	change := (rand.Float64()*2 - 1) * variance
	newValue := value * (1 + change)
	if newValue < min {
		newValue = min
	}
	if newValue > max {
		newValue = max
	}
	return newValue
}

func fluctuateInt(value, delta, min, max int) int {
	if delta <= 0 {
		return value
	}
	change := rand.Intn(delta*2+1) - delta
	newValue := value + change
	if newValue < min {
		newValue = min
	}
	if newValue > max {
		newValue = max
	}
	return newValue
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

func generateDockerServicesAndTasks(hostname string, containers []models.DockerContainer, now time.Time) ([]models.DockerService, []models.DockerTask) {
	if len(containers) == 0 {
		return nil, nil
	}

	type svcAgg struct {
		service models.DockerService
		tasks   []models.DockerTask
	}

	aggregates := make(map[string]*svcAgg)
	stackNames := []string{"frontend", "backend", "ops", "infra"}

	for idx, container := range containers {
		baseName := strings.Split(container.Name, "-")[0]
		stack := stackNames[idx%len(stackNames)]
		serviceName := fmt.Sprintf("%s-%s", stack, baseName)
		serviceID := fmt.Sprintf("svc-%s-%d", stack, idx)

		agg, exists := aggregates[serviceID]
		if !exists {
			agg = &svcAgg{
				service: models.DockerService{
					ID:   serviceID,
					Name: serviceName,
					Mode: []string{"replicated", "global"}[rand.Intn(2)],
					Labels: map[string]string{
						"com.docker.stack.namespace": stack,
					},
					EndpointPorts: []models.DockerServicePort{
						{
							Protocol:      "tcp",
							TargetPort:    uint32(8000 + idx%10),
							PublishedPort: uint32(18000 + idx%10),
							PublishMode:   "ingress",
						},
					},
				},
			}
			if rand.Float64() < 0.5 {
				agg.service.Image = fmt.Sprintf("registry.example.com/%s:%s", serviceName, dockerImageTags[rand.Intn(len(dockerImageTags))])
			}
			aggregates[serviceID] = agg
		}

		desired := 1 + rand.Intn(4)
		agg.service.DesiredTasks += desired

		slots := desired
		if agg.service.Mode == "global" {
			slots = 1
		}

		for slot := 0; slot < slots; slot++ {
			currentState := "running"
			if rand.Float64() < 0.15 {
				currentState = []string{"failed", "shutdown", "pending", "starting"}[rand.Intn(4)]
			}

			// Create a unique container name for each task
			taskContainerName := container.Name
			if slots > 1 {
				taskContainerName = fmt.Sprintf("%s.%d", container.Name, slot+1)
			}

			taskID := fmt.Sprintf("%s-task-%d", serviceID, slot)
			task := models.DockerTask{
				ID:            taskID,
				ServiceID:     serviceID,
				ServiceName:   serviceName,
				Slot:          slot + 1,
				NodeID:        fmt.Sprintf("node-%s", hostname),
				NodeName:      hostname,
				DesiredState:  "running",
				CurrentState:  currentState,
				ContainerID:   fmt.Sprintf("%s-%d", container.ID, slot),
				ContainerName: taskContainerName,
				CreatedAt:     now.Add(-time.Duration(rand.Intn(48)) * time.Hour),
			}

			// Set varied start times for each task (not all identical)
			if currentState == "running" {
				startTime := now.Add(-time.Duration(30+rand.Intn(3600*24)) * time.Second)
				task.StartedAt = &startTime
			} else if container.StartedAt != nil && (currentState == "failed" || currentState == "shutdown") {
				started := *container.StartedAt
				task.StartedAt = &started
			}

			if currentState == "failed" || currentState == "shutdown" {
				task.Error = "container exit"
				task.Message = "Replica exited unexpectedly"
				if container.FinishedAt != nil {
					finished := *container.FinishedAt
					task.CompletedAt = &finished
				} else {
					completedTime := now.Add(-time.Duration(rand.Intn(3600)) * time.Second)
					task.CompletedAt = &completedTime
				}
			}

			agg.tasks = append(agg.tasks, task)
		}
	}

	services := make([]models.DockerService, 0, len(aggregates))
	tasks := make([]models.DockerTask, 0, len(containers))

	for _, agg := range aggregates {
		running := 0
		completed := 0
		for _, task := range agg.tasks {
			if strings.EqualFold(task.CurrentState, "running") {
				running++
			}
			if task.CompletedAt != nil && strings.EqualFold(task.CurrentState, "shutdown") {
				completed++
			}
		}
		agg.service.RunningTasks = running
		agg.service.CompletedTasks = completed

		if running < agg.service.DesiredTasks {
			agg.service.UpdateStatus = &models.DockerServiceUpdate{
				State:       "rollback_started",
				Message:     "Service replicas below desired",
				CompletedAt: nil,
			}
		}

		services = append(services, agg.service)
		tasks = append(tasks, agg.tasks...)
	}

	sort.Slice(services, func(i, j int) bool {
		if services[i].Name == services[j].Name {
			return services[i].ID < services[j].ID
		}
		return services[i].Name < services[j].Name
	})

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].ServiceName == tasks[j].ServiceName {
			if tasks[i].Slot == tasks[j].Slot {
				return tasks[i].ID < tasks[j].ID
			}
			return tasks[i].Slot < tasks[j].Slot
		}
		return tasks[i].ServiceName < tasks[j].ServiceName
	})

	return services, tasks
}
