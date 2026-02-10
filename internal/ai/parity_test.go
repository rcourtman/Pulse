package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

func TestParityVMFields(t *testing.T) {
	now := time.Now().UTC()

	vm := models.VM{
		ID:           "vm-001",
		VMID:         101,
		Name:         "app-vm-01",
		Node:         "pve1",
		Instance:     "pve",
		Status:       "running",
		Type:         "qemu",
		CPU:          0.42, // fraction
		CPUs:         4,
		Memory:       testMemory(),
		Disk:         testDisk(),
		Disks:        []models.Disk{testDisk(), testDisk()},
		IPAddresses:  []string{"10.0.0.10", "fd00::10"},
		OSName:       "Ubuntu",
		OSVersion:    "24.04",
		AgentVersion: "qemu-guest-agent 1.0.0",
		NetworkInterfaces: []models.GuestNetworkInterface{
			{Name: "eth0", MAC: "aa:bb:cc:dd:ee:ff", Addresses: []string{"10.0.0.10/24"}},
		},
		NetworkIn:        12345,
		NetworkOut:       54321,
		DiskRead:         999,
		DiskWrite:        111,
		Uptime:           86400,
		Template:         false,
		LastBackup:       now.Add(-12 * time.Hour),
		Tags:             []string{"env:prod", "team:payments"},
		Lock:             "none",
		LastSeen:         now,
		DiskStatusReason: "",
	}

	parentNode := models.Node{
		ID:            proxmoxNodeID(vm.Instance, vm.Node),
		Name:          vm.Node,
		DisplayName:   "pve1 (rack-a)",
		Instance:      vm.Instance,
		Host:          "https://pve1.example.test:8006",
		Status:        "online",
		Type:          "node",
		CPU:           0.11,
		Memory:        testMemory(),
		Disk:          testDisk(),
		Uptime:        123456,
		LoadAverage:   []float64{0.11, 0.22, 0.33},
		KernelVersion: "6.8.0-rc",
		PVEVersion:    "8.2.2",
		CPUInfo: models.CPUInfo{
			Model:   "AMD EPYC",
			Cores:   16,
			Sockets: 1,
			MHz:     "3200",
		},
		LastSeen:                now,
		ConnectionHealth:        "ok",
		IsClusterMember:         true,
		ClusterName:             "lab",
		PendingUpdates:          3,
		LinkedHostAgentID:       "",
		GuestURL:                "",
		Temperature:             nil,
		PendingUpdatesCheckedAt: now.Add(-30 * time.Minute),
	}

	snapshot := models.StateSnapshot{
		Nodes: []models.Node{parentNode},
		VMs:   []models.VM{vm},
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	vms := registry.VMs()
	require.Len(t, vms, 1)
	v := vms[0]

	require.Equal(t, vm.Name, v.Name())
	require.Equal(t, unifiedresources.StatusOnline, v.Status())
	require.Equal(t, vm.VMID, v.VMID())
	require.Equal(t, vm.Node, v.Node())
	require.Equal(t, vm.Instance, v.Instance())
	require.Equal(t, vm.Template, v.Template())
	require.Equal(t, vm.CPUs, v.CPUs())
	require.Equal(t, vm.Uptime, v.Uptime())

	require.InEpsilon(t, percentFromUsage(vm.CPU), v.CPUPercent(), 1e-9)
	require.Equal(t, vm.Memory.Used, v.MemoryUsed())
	require.Equal(t, vm.Memory.Total, v.MemoryTotal())
	require.InEpsilon(t, percentFromUsage(vm.Memory.Usage), v.MemoryPercent(), 1e-9)
	require.Equal(t, vm.Disk.Used, v.DiskUsed())
	require.Equal(t, vm.Disk.Total, v.DiskTotal())
	require.InEpsilon(t, percentFromUsage(vm.Disk.Usage), v.DiskPercent(), 1e-9)

	require.Equal(t, vm.Tags, v.Tags())
	require.True(t, v.LastSeen().Equal(vm.LastSeen))
}

func TestParityContainerFields(t *testing.T) {
	now := time.Now().UTC()

	ct := models.Container{
		ID:         "ct-001",
		VMID:       201,
		Name:       "app-lxc-01",
		Node:       "pve1",
		Instance:   "pve",
		Status:     "running",
		Type:       "lxc",
		CPU:        0.31,
		CPUs:       2,
		Memory:     testMemory(),
		Disk:       testDisk(),
		Disks:      []models.Disk{testDisk()},
		NetworkIn:  222,
		NetworkOut: 333,
		DiskRead:   444,
		DiskWrite:  555,
		Uptime:     43210,
		Template:   false,
		LastBackup: now.Add(-24 * time.Hour),
		Tags:       []string{"env:prod", "role:web"},
		Lock:       "none",
		LastSeen:   now,
		IPAddresses: []string{
			"10.0.0.20",
		},
		NetworkInterfaces: []models.GuestNetworkInterface{
			{Name: "eth0", MAC: "11:22:33:44:55:66", Addresses: []string{"10.0.0.20/24"}},
		},
		OSName:          "Debian",
		IsOCI:           true,
		OSTemplate:      "docker:alpine:latest",
		HasDocker:       true,
		DockerCheckedAt: now.Add(-10 * time.Minute),
	}

	parentNode := models.Node{
		ID:          proxmoxNodeID(ct.Instance, ct.Node),
		Name:        ct.Node,
		DisplayName: "pve1 (rack-a)",
		Instance:    ct.Instance,
		Host:        "https://pve1.example.test:8006",
		Status:      "online",
		Type:        "node",
		CPU:         0.11,
		Memory:      testMemory(),
		Disk:        testDisk(),
		Uptime:      123456,
		LastSeen:    now,
	}

	snapshot := models.StateSnapshot{
		Nodes:      []models.Node{parentNode},
		Containers: []models.Container{ct},
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	cts := registry.Containers()
	require.Len(t, cts, 1)
	v := cts[0]

	require.Equal(t, ct.Name, v.Name())
	require.Equal(t, unifiedresources.StatusOnline, v.Status())
	require.Equal(t, ct.VMID, v.VMID())
	require.Equal(t, ct.Node, v.Node())
	require.Equal(t, ct.Instance, v.Instance())
	require.Equal(t, ct.Template, v.Template())
	require.Equal(t, ct.CPUs, v.CPUs())
	require.Equal(t, ct.Uptime, v.Uptime())

	require.InEpsilon(t, percentFromUsage(ct.CPU), v.CPUPercent(), 1e-9)
	require.Equal(t, ct.Memory.Used, v.MemoryUsed())
	require.Equal(t, ct.Memory.Total, v.MemoryTotal())
	require.InEpsilon(t, percentFromUsage(ct.Memory.Usage), v.MemoryPercent(), 1e-9)
	require.Equal(t, ct.Disk.Used, v.DiskUsed())
	require.Equal(t, ct.Disk.Total, v.DiskTotal())
	require.InEpsilon(t, percentFromUsage(ct.Disk.Usage), v.DiskPercent(), 1e-9)

	require.Equal(t, ct.Tags, v.Tags())
	require.True(t, v.LastSeen().Equal(ct.LastSeen))
}

func TestParityNodeFields(t *testing.T) {
	now := time.Now().UTC()

	temp := &models.Temperature{
		CPUMax:     80.5,
		Available:  true,
		HasCPU:     true,
		HasGPU:     false,
		HasNVMe:    false,
		HasSMART:   false,
		CPUPackage: 79.1,
	}

	node := models.Node{
		ID:            proxmoxNodeID("pve", "pve1"),
		Name:          "pve1",
		DisplayName:   "pve1 (rack-a)",
		Instance:      "pve",
		Host:          "https://pve1.example.test:8006",
		GuestURL:      "https://pve1-guest.example.test",
		Status:        "online",
		Type:          "node",
		CPU:           0.33,
		Memory:        testMemory(),
		Disk:          testDisk(),
		Uptime:        54321,
		LoadAverage:   []float64{0.12, 0.34, 0.56},
		KernelVersion: "6.5.0",
		PVEVersion:    "8.2.2",
		CPUInfo: models.CPUInfo{
			Model:   "Intel Xeon",
			Cores:   8,
			Sockets: 2,
			MHz:     "2800",
		},
		Temperature:             temp,
		LastSeen:                now,
		ConnectionHealth:        "ok",
		IsClusterMember:         true,
		ClusterName:             "lab",
		PendingUpdates:          12,
		PendingUpdatesCheckedAt: now.Add(-10 * time.Minute),
		LinkedHostAgentID:       "agent1",
	}

	snapshot := models.StateSnapshot{
		Nodes: []models.Node{node},
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	nodes := registry.Nodes()
	require.Len(t, nodes, 1)
	v := nodes[0]

	require.Equal(t, node.DisplayName, v.Name())
	require.Equal(t, unifiedresources.StatusOnline, v.Status())
	require.Equal(t, node.Name, v.NodeName())
	require.Equal(t, node.ClusterName, v.ClusterName())
	require.Equal(t, node.PVEVersion, v.PVEVersion())
	require.Equal(t, node.KernelVersion, v.KernelVersion())
	require.Equal(t, node.Uptime, v.Uptime())

	require.True(t, v.HasTemperature())
	require.InEpsilon(t, node.Temperature.CPUMax, v.Temperature(), 1e-9)
	require.Equal(t, node.LoadAverage, v.LoadAverage())
	require.Equal(t, node.PendingUpdates, v.PendingUpdates())

	require.Equal(t, node.CPUInfo.Cores*node.CPUInfo.Sockets, v.CPUs())
	require.InEpsilon(t, percentFromUsage(node.CPU), v.CPUPercent(), 1e-9)
	require.InEpsilon(t, percentFromUsage(node.Memory.Usage), v.MemoryPercent(), 1e-9)
	require.InEpsilon(t, percentFromUsage(node.Disk.Usage), v.DiskPercent(), 1e-9)

	require.Equal(t, node.LinkedHostAgentID, v.LinkedHostAgentID())
}

func TestParityHostFields(t *testing.T) {
	now := time.Now().UTC()

	hostTemp := 62.25
	host := models.Host{
		ID:            "agent1",
		Hostname:      "host-agent-01",
		DisplayName:   "host-agent-01 (prod)",
		Platform:      "linux",
		OSName:        "Ubuntu",
		OSVersion:     "24.04",
		KernelVersion: "6.8.0",
		Architecture:  "amd64",
		CPUCount:      16,
		CPUUsage:      0.25,
		Memory:        testMemory(),
		LoadAverage:   []float64{0.21, 0.18, 0.22},
		Disks: []models.Disk{
			{
				Total:      107374182400,
				Used:       10737418240,
				Free:       96636764160,
				Usage:      float64(10737418240) / float64(107374182400),
				Mountpoint: "/",
				Type:       "ext4",
				Device:     "/dev/sda1",
			},
		},
		NetworkInterfaces: []models.HostNetworkInterface{
			{Name: "eth0", MAC: "aa:bb:cc:00:11:22", Addresses: []string{"192.168.1.10/24"}},
			{Name: "eth1", MAC: "aa:bb:cc:33:44:55", Addresses: []string{"10.0.0.5/24"}},
		},
		Sensors: models.HostSensorSummary{
			TemperatureCelsius: map[string]float64{
				"cpu_package": hostTemp,
			},
		},
		Status:          "online",
		UptimeSeconds:   98765,
		IntervalSeconds: 10,
		LastSeen:        now,
		AgentVersion:    "pulse-agent 0.9.0",
		MachineID:       "machine-host-01",
		CommandsEnabled: true,
		ReportIP:        "10.0.0.5",
		TokenID:         "tok1",
		TokenName:       "token",
		TokenHint:       "hint",
		Tags:            []string{"env:prod", "role:host"},
		LinkedNodeID:    proxmoxNodeID("pve", "pve1"),
		NetInRate:       1000,
		NetOutRate:      2000,
		DiskReadRate:    3000,
		DiskWriteRate:   4000,
	}

	snapshot := models.StateSnapshot{
		Hosts: []models.Host{host},
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	hosts := registry.Hosts()
	require.Len(t, hosts, 1)
	v := hosts[0]

	require.Equal(t, host.Hostname, v.Hostname())
	require.Equal(t, host.Platform, v.Platform())
	require.Equal(t, host.OSName, v.OSName())
	require.Equal(t, host.Architecture, v.Architecture())
	require.Equal(t, host.AgentVersion, v.AgentVersion())

	require.Equal(t, host.UptimeSeconds, v.UptimeSeconds())
	require.True(t, v.HasTemperature())
	require.InEpsilon(t, hostTemp, v.Temperature(), 1e-9)
	require.Len(t, v.NetworkInterfaces(), len(host.NetworkInterfaces))
	require.Len(t, v.Disks(), len(host.Disks))
	require.Equal(t, host.LinkedNodeID, v.LinkedNodeID())
}

func TestParityDockerHostFields(t *testing.T) {
	now := time.Now().UTC()

	dockerTemp := 55.0
	dh := models.DockerHost{
		ID:                "dockerhost-1",
		AgentID:           "agent-docker-1",
		Hostname:          "docker-01",
		DisplayName:       "docker-01",
		CustomDisplayName: "docker-01 (prod)",
		MachineID:         "machine-docker-01",
		OS:                "Ubuntu 24.04",
		KernelVersion:     "6.8.0",
		Architecture:      "amd64",
		Runtime:           "runc",
		RuntimeVersion:    "1.2.3",
		DockerVersion:     "26.0.0",
		CPUs:              8,
		TotalMemoryBytes:  17179869184,
		UptimeSeconds:     123456,
		CPUUsage:          0.15,
		LoadAverage:       []float64{0.12, 0.10, 0.09},
		Memory:            testMemory(),
		Disks: []models.Disk{
			{
				Total:      107374182400,
				Used:       10737418240,
				Free:       96636764160,
				Usage:      float64(10737418240) / float64(107374182400),
				Mountpoint: "/",
				Type:       "ext4",
				Device:     "/dev/sda1",
			},
		},
		NetworkInterfaces: []models.HostNetworkInterface{
			{Name: "eth0", MAC: "de:ad:be:ef:00:01", Addresses: []string{"10.10.0.10/24"}},
		},
		Status:          "online",
		LastSeen:        now,
		IntervalSeconds: 15,
		AgentVersion:    "pulse-agent 0.9.0",
		Containers:      nil,
		Services:        nil,
		Tasks:           nil,
		Swarm: &models.DockerSwarmInfo{
			NodeID:           "node-1",
			NodeRole:         "manager",
			LocalState:       "active",
			ControlAvailable: true,
			ClusterID:        "cluster-1",
			ClusterName:      "swarm-1",
			Scope:            "swarm",
		},
		Temperature:      &dockerTemp,
		Hidden:           false,
		PendingUninstall: false,
		NetInRate:        111,
		NetOutRate:       222,
		DiskReadRate:     333,
		DiskWriteRate:    444,
	}

	snapshot := models.StateSnapshot{
		DockerHosts: []models.DockerHost{dh},
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	hosts := registry.DockerHosts()
	require.Len(t, hosts, 1)
	v := hosts[0]

	require.Equal(t, dh.Hostname, v.Hostname())
	require.Equal(t, dh.DockerVersion, v.DockerVersion())
	require.Equal(t, dh.RuntimeVersion, v.RuntimeVersion())
	require.Equal(t, dh.OS, v.OS())
	require.Equal(t, dh.KernelVersion, v.KernelVersion())
	require.Equal(t, dh.Architecture, v.Architecture())
	require.Equal(t, dh.AgentVersion, v.AgentVersion())
	require.Equal(t, dh.UptimeSeconds, v.UptimeSeconds())
	require.True(t, v.HasTemperature())
	require.InEpsilon(t, dockerTemp, v.Temperature(), 1e-9)
	require.Len(t, v.NetworkInterfaces(), len(dh.NetworkInterfaces))
	require.Len(t, v.Disks(), len(dh.Disks))
	require.InEpsilon(t, percentFromUsage(dh.CPUUsage), v.CPUPercent(), 1e-9)
	require.InEpsilon(t, percentFromUsage(dh.Memory.Usage), v.MemoryPercent(), 1e-9)
	require.True(t, v.LastSeen().Equal(dh.LastSeen))
}

func TestParityStorageFields(t *testing.T) {
	now := time.Now().UTC()

	node := models.Node{
		ID:       proxmoxNodeID("pve", "pve1"),
		Name:     "pve1",
		Instance: "pve",
		Host:     "https://pve1.example.test:8006",
		Status:   "online",
		CPU:      0.1,
		Memory:   testMemory(),
		Disk:     testDisk(),
		Uptime:   1000,
		LastSeen: now,
	}

	storage := models.Storage{
		ID:       "storage-1",
		Name:     "tank",
		Node:     "pve1",
		Instance: "pve",
		Type:     "zfspool",
		Status:   "online",
		Path:     "/tank",
		Total:    107374182400,
		Used:     10737418240,
		Free:     96636764160,
		Usage:    float64(10737418240) / float64(107374182400),
		Content:  "images, iso, backup",
		Shared:   false,
		Enabled:  true,
		Active:   true,
		ZFSPool: &models.ZFSPool{
			Name:           "tank",
			State:          "ONLINE",
			Status:         "Healthy",
			Scan:           "none",
			ReadErrors:     1,
			WriteErrors:    2,
			ChecksumErrors: 3,
			Devices: []models.ZFSDevice{
				{Name: "mirror-0", Type: "mirror", State: "ONLINE"},
			},
		},
	}

	snapshot := models.StateSnapshot{
		Nodes:   []models.Node{node},
		Storage: []models.Storage{storage},
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	pools := registry.StoragePools()
	require.Len(t, pools, 1)
	v := pools[0]

	require.Equal(t, storage.Name, v.Name())
	require.Equal(t, unifiedresources.StatusOnline, v.Status())
	require.Equal(t, storage.Node, v.Node())
	require.Equal(t, storage.Instance, v.Instance())
	require.Equal(t, "zfspool", v.StorageType())
	require.Equal(t, storage.Content, v.Content())
	require.Equal(t, []string{"images", "iso", "backup"}, v.ContentTypes())
	require.Equal(t, storage.Shared, v.Shared())
	require.False(t, v.IsCeph())
	require.True(t, v.IsZFS())
	require.Equal(t, storage.ZFSPool.State, v.ZFSPoolState())
	require.Equal(t, storage.ZFSPool.ReadErrors, v.ZFSReadErrors())
	require.Equal(t, storage.ZFSPool.WriteErrors, v.ZFSWriteErrors())
	require.Equal(t, storage.ZFSPool.ChecksumErrors, v.ZFSChecksumErrors())

	require.Equal(t, storage.Used, v.DiskUsed())
	require.Equal(t, storage.Total, v.DiskTotal())
	require.InEpsilon(t, percentFromUsage(storage.Usage), v.DiskPercent(), 1e-9)
	require.NotZero(t, v.LastSeen())
}

func TestParityPBSInstanceFields(t *testing.T) {
	now := time.Now().UTC()

	pbs := models.PBSInstance{
		ID:               "pbs-1",
		Name:             "pbs-main",
		Host:             "https://pbs.example.test:8007",
		GuestURL:         "https://pbs-guest.example.test",
		Status:           "online",
		Version:          "3.2-1",
		CPU:              12.3,
		Memory:           73.0,
		MemoryUsed:       1073741824,
		MemoryTotal:      4294967296,
		Uptime:           222222,
		Datastores:       []models.PBSDatastore{{Name: "ds1"}, {Name: "ds2"}},
		BackupJobs:       []models.PBSBackupJob{{ID: "job1"}},
		SyncJobs:         []models.PBSSyncJob{{ID: "sync1"}, {ID: "sync2"}},
		VerifyJobs:       []models.PBSVerifyJob{{ID: "verify1"}},
		PruneJobs:        []models.PBSPruneJob{{ID: "prune1"}},
		GarbageJobs:      []models.PBSGarbageJob{{ID: "gc1"}},
		ConnectionHealth: "connected",
		LastSeen:         now,
	}

	snapshot := models.StateSnapshot{
		PBSInstances: []models.PBSInstance{pbs},
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	instances := registry.PBSInstances()
	require.Len(t, instances, 1)
	v := instances[0]

	require.Equal(t, pbs.Name, v.Name())
	require.Equal(t, unifiedresources.StatusOnline, v.Status())
	require.Equal(t, "pbs.example.test", v.Hostname())
	require.Equal(t, pbs.Version, v.Version())
	require.Equal(t, pbs.Uptime, v.UptimeSeconds())
	require.Equal(t, len(pbs.Datastores), v.DatastoreCount())
	require.Equal(t, len(pbs.BackupJobs), v.BackupJobCount())
	require.Equal(t, len(pbs.SyncJobs), v.SyncJobCount())
	require.Equal(t, len(pbs.VerifyJobs), v.VerifyJobCount())
	require.Equal(t, len(pbs.PruneJobs), v.PruneJobCount())
	require.Equal(t, len(pbs.GarbageJobs), v.GarbageJobCount())
	require.Equal(t, pbs.ConnectionHealth, v.ConnectionHealth())
	require.InEpsilon(t, percentFromUsage(pbs.CPU), v.CPUPercent(), 1e-9)
	require.InEpsilon(t, percentFromUsage(pbs.Memory), v.MemoryPercent(), 1e-9)
	require.Equal(t, 0.0, v.DiskPercent())
	require.True(t, v.LastSeen().Equal(pbs.LastSeen))
	require.Equal(t, pbs.GuestURL, v.CustomURL())
}

func TestParityPMGInstanceFields(t *testing.T) {
	now := time.Now().UTC()
	queueUpdated := now.Add(-2 * time.Minute)

	pmg := models.PMGInstance{
		ID:       "pmg-1",
		Name:     "pmg-main",
		Host:     "https://pmg.example.test:8006",
		GuestURL: "https://pmg-guest.example.test",
		Status:   "online",
		Version:  "8.2-1",
		Nodes: []models.PMGNodeStatus{
			{
				Name:   "pmg1",
				Status: "online",
				Role:   "master",
				Uptime: 1000,
				QueueStatus: &models.PMGQueueStatus{
					Active:    2,
					Deferred:  3,
					Hold:      1,
					Incoming:  0,
					Total:     6,
					OldestAge: 120,
					UpdatedAt: queueUpdated,
				},
			},
			{
				Name:   "pmg2",
				Status: "online",
				Role:   "node",
				Uptime: 2000, // max uptime
				QueueStatus: &models.PMGQueueStatus{
					Active:    1,
					Deferred:  0,
					Hold:      0,
					Incoming:  4,
					Total:     5,
					OldestAge: 300,
					UpdatedAt: queueUpdated.Add(1 * time.Minute),
				},
			},
		},
		MailStats: &models.PMGMailStats{
			Timeframe:  "hour",
			CountTotal: 123.0,
			SpamIn:     7.0,
			VirusIn:    2.0,
			UpdatedAt:  now.Add(-5 * time.Minute),
		},
		ConnectionHealth: "connected",
		LastSeen:         now,
		LastUpdated:      now.Add(-1 * time.Minute),
	}

	snapshot := models.StateSnapshot{
		PMGInstances: []models.PMGInstance{pmg},
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	instances := registry.PMGInstances()
	require.Len(t, instances, 1)
	v := instances[0]

	require.Equal(t, pmg.Name, v.Name())
	require.Equal(t, unifiedresources.StatusOnline, v.Status())
	require.Equal(t, "pmg.example.test", v.Hostname())
	require.Equal(t, pmg.Version, v.Version())
	require.Equal(t, len(pmg.Nodes), v.NodeCount())
	require.Equal(t, int64(2000), v.UptimeSeconds())

	// Queue totals are aggregated across nodes.
	require.Equal(t, 3, v.QueueActive())
	require.Equal(t, 3, v.QueueDeferred())
	require.Equal(t, 11, v.QueueTotal())

	require.Equal(t, pmg.MailStats.CountTotal, v.MailCountTotal())
	require.Equal(t, pmg.MailStats.SpamIn, v.SpamIn())
	require.Equal(t, pmg.MailStats.VirusIn, v.VirusIn())

	require.Equal(t, pmg.ConnectionHealth, v.ConnectionHealth())
	require.Equal(t, 0.0, v.CPUPercent())
	require.Equal(t, 0.0, v.MemoryPercent())
	require.Equal(t, 0.0, v.DiskPercent())
	require.True(t, v.LastSeen().Equal(pmg.LastSeen))
	require.Equal(t, pmg.GuestURL, v.CustomURL())
}

func TestParityWorkloadCounts(t *testing.T) {
	snapshot := testStateSnapshot()
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	require.Len(t, registry.VMs(), len(snapshot.VMs))
	require.Len(t, registry.Containers(), len(snapshot.Containers))
	require.Len(t, registry.Workloads(), len(snapshot.VMs)+len(snapshot.Containers))
}

func TestParityResourceCounts(t *testing.T) {
	snapshot := testStateSnapshot()

	// Add host agent records, including one mutually-linked node+agent pair
	// that should merge into a single unified host resource.
	nodeIDToMerge := snapshot.Nodes[0].ID
	snapshot.Nodes[0].LinkedHostAgentID = "agent-merge-1"
	snapshot.Hosts = append(snapshot.Hosts,
		models.Host{
			ID:            "agent-merge-1",
			Hostname:      "pve1-host-agent",
			Platform:      "linux",
			OSName:        "Debian",
			Architecture:  "amd64",
			Status:        "online",
			UptimeSeconds: 111,
			LastSeen:      time.Now().UTC(),
			AgentVersion:  "pulse-agent 0.9.0",
			LinkedNodeID:  nodeIDToMerge,
			MachineID:     "machine-merge-1",
			NetworkInterfaces: []models.HostNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.50.10/24"}},
			},
			Sensors:  models.HostSensorSummary{TemperatureCelsius: map[string]float64{"cpu_package": 50.0}},
			Disks:    []models.Disk{testDisk()},
			Memory:   testMemory(),
			CPUUsage: 0.05,
		},
		// One standalone host agent that should not merge with anything.
		models.Host{
			ID:            "agent-standalone-1",
			Hostname:      "standalone-host-01",
			Platform:      "linux",
			OSName:        "Ubuntu",
			Architecture:  "amd64",
			Status:        "online",
			UptimeSeconds: 222,
			LastSeen:      time.Now().UTC(),
			AgentVersion:  "pulse-agent 0.9.0",
			MachineID:     "machine-standalone-1",
			NetworkInterfaces: []models.HostNetworkInterface{
				{Name: "eth0", MAC: "00:aa:bb:cc:dd:ee", Addresses: []string{"10.250.0.10/24"}},
			},
			Sensors:  models.HostSensorSummary{TemperatureCelsius: map[string]float64{"cpu_package": 45.0}},
			Disks:    []models.Disk{testDisk()},
			Memory:   testMemory(),
			CPUUsage: 0.07,
		},
	)

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	require.Len(t, registry.VMs(), len(snapshot.VMs))
	require.Len(t, registry.Containers(), len(snapshot.Containers))
	require.Len(t, registry.StoragePools(), len(snapshot.Storage))
	require.Len(t, registry.DockerHosts(), len(snapshot.DockerHosts))
	require.Len(t, registry.PBSInstances(), len(snapshot.PBSInstances))
	require.Len(t, registry.PMGInstances(), len(snapshot.PMGInstances))

	// View-level counts should match snapshot counts, even when host identities merge.
	require.Len(t, registry.Nodes(), len(snapshot.Nodes))
	require.Len(t, registry.Hosts(), len(snapshot.Hosts))

	// Expected delta: host identity resolution merges Proxmox nodes with their linked
	// host agents into a single unified resource. This means consumers see fewer
	// resources via ReadState than via raw StateSnapshot, but each resource has
	// richer data from multiple sources.
	rawInfra := len(snapshot.Nodes) + len(snapshot.Hosts) + len(snapshot.DockerHosts)
	require.Equal(t, rawInfra-1, len(registry.Infrastructure()))
}

func TestExpectedDeltaHostMerge(t *testing.T) {
	// Expected behavior change: Host identity resolution merges Proxmox nodes with their
	// linked host agents into a single unified resource. This means consumers see fewer
	// resources via ReadState than via raw StateSnapshot, but each resource has richer
	// data from multiple sources.
	now := time.Now().UTC()

	node := models.Node{
		ID:                proxmoxNodeID("pve", "pve1"),
		Name:              "pve1",
		Instance:          "pve",
		Host:              "https://pve1.example.test:8006",
		Status:            "online",
		CPU:               0.1,
		Memory:            testMemory(),
		Disk:              testDisk(),
		Uptime:            1000,
		LastSeen:          now,
		LinkedHostAgentID: "agent1",
	}

	host := models.Host{
		ID:            "agent1",
		Hostname:      "pve1",
		Platform:      "linux",
		OSName:        "Debian",
		Architecture:  "amd64",
		Status:        "online",
		UptimeSeconds: 2000,
		LastSeen:      now.Add(1 * time.Second),
		AgentVersion:  "pulse-agent 0.9.0",
		LinkedNodeID:  node.ID,
		MachineID:     "machine-merge-1",
		NetworkInterfaces: []models.HostNetworkInterface{
			{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.50.10/24"}},
		},
		Sensors:  models.HostSensorSummary{TemperatureCelsius: map[string]float64{"cpu_package": 51.0}},
		Disks:    []models.Disk{testDisk()},
		Memory:   testMemory(),
		CPUUsage: 0.05,
	}

	snapshot := models.StateSnapshot{
		Nodes: []models.Node{node},
		Hosts: []models.Host{host},
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	hostsByType := registry.ListByType(unifiedresources.ResourceTypeHost)
	require.Len(t, hostsByType, 1)
	merged := hostsByType[0]

	require.NotNil(t, merged.Proxmox)
	require.NotNil(t, merged.Agent)
	require.Contains(t, merged.Sources, unifiedresources.SourceProxmox)
	require.Contains(t, merged.Sources, unifiedresources.SourceAgent)
}

func TestExpectedDeltaNodesThroughTwoViews(t *testing.T) {
	now := time.Now().UTC()

	node := models.Node{
		ID:                proxmoxNodeID("pve", "pve1"),
		Name:              "pve1",
		Instance:          "pve",
		Host:              "https://pve1.example.test:8006",
		Status:            "online",
		CPU:               0.1,
		Memory:            testMemory(),
		Disk:              testDisk(),
		Uptime:            1000,
		LastSeen:          now,
		LinkedHostAgentID: "agent1",
	}

	host := models.Host{
		ID:            "agent1",
		Hostname:      "pve1",
		Platform:      "linux",
		OSName:        "Debian",
		Architecture:  "amd64",
		Status:        "online",
		UptimeSeconds: 2000,
		LastSeen:      now.Add(1 * time.Second),
		AgentVersion:  "pulse-agent 0.9.0",
		LinkedNodeID:  node.ID,
		MachineID:     "machine-merge-1",
		NetworkInterfaces: []models.HostNetworkInterface{
			{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.50.10/24"}},
		},
		Sensors:  models.HostSensorSummary{TemperatureCelsius: map[string]float64{"cpu_package": 51.0}},
		Disks:    []models.Disk{testDisk()},
		Memory:   testMemory(),
		CPUUsage: 0.05,
	}

	snapshot := models.StateSnapshot{
		Nodes: []models.Node{node},
		Hosts: []models.Host{host},
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	require.Len(t, registry.Infrastructure(), 1)
	require.Len(t, registry.Nodes(), 1)
	require.Len(t, registry.Hosts(), 1)

	// Same underlying unified resource projected through two different view types.
	require.Equal(t, registry.Nodes()[0].ID(), registry.Hosts()[0].ID())
}

func testStateSnapshot() models.StateSnapshot {
	now := time.Now().UTC()

	nodes := []models.Node{
		{
			ID:            proxmoxNodeID("pve", "pve1"),
			Name:          "pve1",
			DisplayName:   "pve1 (rack-a)",
			Instance:      "pve",
			Host:          "https://pve1.example.test:8006",
			Status:        "online",
			Type:          "node",
			CPU:           0.10,
			Memory:        testMemory(),
			Disk:          testDisk(),
			Uptime:        111111,
			LoadAverage:   []float64{0.10, 0.12, 0.15},
			KernelVersion: "6.5.0",
			PVEVersion:    "8.2.2",
			CPUInfo: models.CPUInfo{
				Model:   "Intel Xeon",
				Cores:   8,
				Sockets: 2,
				MHz:     "2800",
			},
			LastSeen:         now,
			ConnectionHealth: "ok",
			IsClusterMember:  true,
			ClusterName:      "lab",
			PendingUpdates:   5,
		},
		{
			ID:            proxmoxNodeID("pve", "pve2"),
			Name:          "pve2",
			DisplayName:   "pve2 (rack-b)",
			Instance:      "pve",
			Host:          "https://pve2.example.test:8006",
			Status:        "online",
			Type:          "node",
			CPU:           0.12,
			Memory:        testMemory(),
			Disk:          testDisk(),
			Uptime:        222222,
			LoadAverage:   []float64{0.08, 0.09, 0.10},
			KernelVersion: "6.5.0",
			PVEVersion:    "8.2.2",
			CPUInfo: models.CPUInfo{
				Model:   "Intel Xeon",
				Cores:   16,
				Sockets: 1,
				MHz:     "3200",
			},
			LastSeen:         now,
			ConnectionHealth: "ok",
			IsClusterMember:  true,
			ClusterName:      "lab",
			PendingUpdates:   0,
		},
	}

	vms := []models.VM{
		{
			ID:           "vm-001",
			VMID:         101,
			Name:         "app-vm-01",
			Node:         "pve1",
			Instance:     "pve",
			Status:       "running",
			Type:         "qemu",
			CPU:          0.30,
			CPUs:         4,
			Memory:       testMemory(),
			Disk:         testDisk(),
			NetworkIn:    1000,
			NetworkOut:   2000,
			DiskRead:     300,
			DiskWrite:    400,
			Uptime:       10000,
			Template:     false,
			LastBackup:   now.Add(-1 * time.Hour),
			Tags:         []string{"env:prod"},
			LastSeen:     now,
			IPAddresses:  []string{"10.0.0.10"},
			OSName:       "Ubuntu",
			OSVersion:    "24.04",
			AgentVersion: "qemu-guest-agent 1.0.0",
		},
		{
			ID:           "vm-002",
			VMID:         102,
			Name:         "app-vm-02",
			Node:         "pve2",
			Instance:     "pve",
			Status:       "stopped",
			Type:         "qemu",
			CPU:          0.01,
			CPUs:         2,
			Memory:       testMemory(),
			Disk:         testDisk(),
			Uptime:       0,
			Template:     false,
			LastBackup:   now.Add(-2 * time.Hour),
			Tags:         []string{"env:prod"},
			LastSeen:     now,
			IPAddresses:  []string{"10.0.0.11"},
			OSName:       "Ubuntu",
			OSVersion:    "24.04",
			AgentVersion: "qemu-guest-agent 1.0.0",
		},
		{
			ID:           "vm-003",
			VMID:         103,
			Name:         "db-vm-01",
			Node:         "pve1",
			Instance:     "pve",
			Status:       "running",
			Type:         "qemu",
			CPU:          0.55,
			CPUs:         8,
			Memory:       testMemory(),
			Disk:         testDisk(),
			NetworkIn:    500,
			NetworkOut:   800,
			DiskRead:     120,
			DiskWrite:    240,
			Uptime:       20000,
			Template:     false,
			LastBackup:   now.Add(-30 * time.Minute),
			Tags:         []string{"env:prod", "role:db"},
			LastSeen:     now,
			IPAddresses:  []string{"10.0.0.12"},
			OSName:       "Debian",
			OSVersion:    "12",
			AgentVersion: "qemu-guest-agent 1.0.0",
		},
	}

	containers := []models.Container{
		{
			ID:              "ct-001",
			VMID:            201,
			Name:            "web-lxc-01",
			Node:            "pve1",
			Instance:        "pve",
			Status:          "running",
			Type:            "lxc",
			CPU:             0.20,
			CPUs:            2,
			Memory:          testMemory(),
			Disk:            testDisk(),
			NetworkIn:       111,
			NetworkOut:      222,
			DiskRead:        10,
			DiskWrite:       20,
			Uptime:          3000,
			Template:        false,
			LastBackup:      now.Add(-3 * time.Hour),
			Tags:            []string{"env:prod", "role:web"},
			LastSeen:        now,
			IPAddresses:     []string{"10.0.0.20"},
			OSName:          "Debian",
			IsOCI:           true,
			OSTemplate:      "docker:alpine:latest",
			HasDocker:       true,
			DockerCheckedAt: now.Add(-1 * time.Hour),
		},
		{
			ID:          "ct-002",
			VMID:        202,
			Name:        "worker-lxc-01",
			Node:        "pve2",
			Instance:    "pve",
			Status:      "stopped",
			Type:        "lxc",
			CPU:         0.05,
			CPUs:        1,
			Memory:      testMemory(),
			Disk:        testDisk(),
			Uptime:      0,
			Template:    false,
			LastBackup:  now.Add(-4 * time.Hour),
			Tags:        []string{"env:prod", "role:worker"},
			LastSeen:    now,
			IPAddresses: []string{"10.0.0.21"},
			OSName:      "Debian",
		},
	}

	dockerTemp := 55.0
	dockerHosts := []models.DockerHost{
		{
			ID:                "dockerhost-1",
			AgentID:           "agent-docker-1",
			Hostname:          "docker-01",
			DisplayName:       "docker-01",
			CustomDisplayName: "docker-01 (prod)",
			MachineID:         "machine-docker-01",
			OS:                "Ubuntu 24.04",
			KernelVersion:     "6.8.0",
			Architecture:      "amd64",
			Runtime:           "runc",
			RuntimeVersion:    "1.2.3",
			DockerVersion:     "26.0.0",
			CPUs:              8,
			TotalMemoryBytes:  17179869184,
			UptimeSeconds:     123456,
			CPUUsage:          0.15,
			LoadAverage:       []float64{0.12, 0.10, 0.09},
			Memory:            testMemory(),
			Disks:             []models.Disk{testDisk()},
			NetworkInterfaces: []models.HostNetworkInterface{{Name: "eth0", MAC: "de:ad:be:ef:00:01", Addresses: []string{"10.10.0.10/24"}}},
			Status:            "online",
			LastSeen:          now,
			IntervalSeconds:   15,
			AgentVersion:      "pulse-agent 0.9.0",
			Containers:        nil,
			Temperature:       &dockerTemp,
		},
	}

	storage := []models.Storage{
		{
			ID:       "storage-1",
			Name:     "tank",
			Node:     "pve1",
			Instance: "pve",
			Type:     "zfspool",
			Status:   "online",
			Path:     "/tank",
			Total:    107374182400,
			Used:     10737418240,
			Free:     96636764160,
			Usage:    float64(10737418240) / float64(107374182400),
			Content:  "images, iso, backup",
			Shared:   false,
			Enabled:  true,
			Active:   true,
			ZFSPool: &models.ZFSPool{
				Name:           "tank",
				State:          "ONLINE",
				Status:         "Healthy",
				Scan:           "none",
				ReadErrors:     0,
				WriteErrors:    0,
				ChecksumErrors: 0,
			},
		},
	}

	pbs := []models.PBSInstance{
		{
			ID:               "pbs-1",
			Name:             "pbs-main",
			Host:             "https://pbs.example.test:8007",
			GuestURL:         "https://pbs-guest.example.test",
			Status:           "online",
			Version:          "3.2-1",
			CPU:              12.3,
			Memory:           73.0,
			MemoryUsed:       1073741824,
			MemoryTotal:      4294967296,
			Uptime:           222222,
			Datastores:       []models.PBSDatastore{{Name: "ds1"}, {Name: "ds2"}},
			BackupJobs:       []models.PBSBackupJob{{ID: "job1"}},
			SyncJobs:         []models.PBSSyncJob{{ID: "sync1"}},
			VerifyJobs:       []models.PBSVerifyJob{{ID: "verify1"}},
			PruneJobs:        []models.PBSPruneJob{{ID: "prune1"}},
			GarbageJobs:      []models.PBSGarbageJob{{ID: "gc1"}},
			ConnectionHealth: "connected",
			LastSeen:         now,
		},
	}

	queueUpdated := now.Add(-2 * time.Minute)
	pmg := []models.PMGInstance{
		{
			ID:       "pmg-1",
			Name:     "pmg-main",
			Host:     "https://pmg.example.test:8006",
			GuestURL: "https://pmg-guest.example.test",
			Status:   "online",
			Version:  "8.2-1",
			Nodes: []models.PMGNodeStatus{
				{
					Name:   "pmg1",
					Status: "online",
					Role:   "master",
					Uptime: 1000,
					QueueStatus: &models.PMGQueueStatus{
						Active:    1,
						Deferred:  1,
						Hold:      0,
						Incoming:  0,
						Total:     2,
						OldestAge: 120,
						UpdatedAt: queueUpdated,
					},
				},
				{
					Name:   "pmg2",
					Status: "online",
					Role:   "node",
					Uptime: 2000,
					QueueStatus: &models.PMGQueueStatus{
						Active:    0,
						Deferred:  1,
						Hold:      0,
						Incoming:  1,
						Total:     2,
						OldestAge: 300,
						UpdatedAt: queueUpdated.Add(1 * time.Minute),
					},
				},
			},
			MailStats: &models.PMGMailStats{
				Timeframe:  "hour",
				CountTotal: 123.0,
				SpamIn:     7.0,
				VirusIn:    2.0,
				UpdatedAt:  now.Add(-5 * time.Minute),
			},
			ConnectionHealth: "connected",
			LastSeen:         now,
			LastUpdated:      now.Add(-1 * time.Minute),
		},
	}

	return models.StateSnapshot{
		Nodes:        nodes,
		VMs:          vms,
		Containers:   containers,
		DockerHosts:  dockerHosts,
		Storage:      storage,
		PBSInstances: pbs,
		PMGInstances: pmg,
		LastUpdate:   now,
	}
}

func testMemory() models.Memory {
	used := int64(1073741824)  // 1GB
	total := int64(4294967296) // 4GB
	return models.Memory{
		Used:  used,
		Total: total,
		Free:  total - used,
		Usage: float64(used) / float64(total),
	}
}

func testDisk() models.Disk {
	used := int64(10737418240)   // 10GB
	total := int64(107374182400) // 100GB
	return models.Disk{
		Used:  used,
		Total: total,
		Free:  total - used,
		Usage: float64(used) / float64(total),
	}
}

func percentFromUsage(value float64) float64 {
	if value <= 1.0 {
		return value * 100
	}
	return value
}

func proxmoxNodeID(instance, nodeName string) string {
	if instance == "" {
		return nodeName
	}
	return instance + "-" + nodeName
}
