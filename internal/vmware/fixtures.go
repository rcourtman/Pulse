package vmware

import (
	"fmt"
	"time"
)

func DefaultFixtures() InventorySnapshot {
	collectedAt := time.Date(2026, time.March, 31, 9, 45, 0, 0, time.UTC)

	accessible := true
	multipleHostAccess := true

	snapshot := defaultFixturesPrimaryCluster(collectedAt, accessible, multipleHostAccess)
	appendEdgeClusterFixtures(&snapshot, collectedAt, accessible, multipleHostAccess)
	return snapshot
}

func defaultFixturesPrimaryCluster(
	collectedAt time.Time,
	accessible bool,
	multipleHostAccess bool,
) InventorySnapshot {
	return InventorySnapshot{
		ConnectionID:   "vc-mock-1",
		ConnectionName: "Lab vCenter",
		VCenterHost:    "vcsa.lab.local",
		VIRelease:      "8.0.3",
		CollectedAt:    collectedAt,
		Hosts: []InventoryHost{
			{
				Host:                "host-101",
				Name:                "esxi-01.lab.local",
				ConnectionState:     "CONNECTED",
				PowerState:          "POWERED_ON",
				HostUUID:            "uuid-host-101",
				DatacenterID:        "datacenter-1",
				DatacenterName:      "Primary DC",
				ComputeResourceID:   "domain-c101",
				ComputeResourceName: "Production Cluster",
				ClusterID:           "domain-c101",
				ClusterName:         "Production Cluster",
				FolderID:            "group-h4",
				FolderName:          "Cluster Hosts",
				DatastoreIDs:        []string{"datastore-201", "datastore-202", "datastore-203"},
				DatastoreNames:      []string{"nvme-primary", "archive-tier", "analytics-vsan"},
				OverallStatus:       "green",
				Metrics: &InventoryMetrics{
					CPUPercent:              float64Ptr(24.6),
					MemoryPercent:           float64Ptr(56.3),
					MemoryUsedBytes:         int64Ptr(48_370_278_400),
					MemoryTotalBytes:        int64Ptr(85_899_345_920),
					NetInBytesPerSecond:     float64Ptr(1_340_000),
					NetOutBytesPerSecond:    float64Ptr(1_040_000),
					DiskReadBytesPerSecond:  float64Ptr(2_480_000),
					DiskWriteBytesPerSecond: float64Ptr(2_040_000),
				},
				RecentTasks: []InventoryTask{{
					Task:      "task-301",
					Name:      "Apply host profile remediation",
					State:     "success",
					StartedAt: collectedAt.Add(-28 * time.Minute),
				}},
				RecentEvents: []InventoryEvent{{
					Event:     "event-601",
					Type:      "HostConnectedEvent",
					Message:   "Host reconnected to vCenter and is reporting healthy telemetry",
					User:      "svc-pulse",
					CreatedAt: collectedAt.Add(-52 * time.Minute),
				}},
			},
			{
				Host:                "host-102",
				Name:                "esxi-02.lab.local",
				ConnectionState:     "CONNECTED",
				PowerState:          "POWERED_ON",
				HostUUID:            "uuid-host-102",
				DatacenterID:        "datacenter-1",
				DatacenterName:      "Primary DC",
				ComputeResourceID:   "domain-c101",
				ComputeResourceName: "Production Cluster",
				ClusterID:           "domain-c101",
				ClusterName:         "Production Cluster",
				FolderID:            "group-h4",
				FolderName:          "Cluster Hosts",
				DatastoreIDs:        []string{"datastore-201", "datastore-204"},
				DatastoreNames:      []string{"nvme-primary", "backup-nfs"},
				OverallStatus:       "yellow",
				Metrics: &InventoryMetrics{
					CPUPercent:              float64Ptr(43.6),
					MemoryPercent:           float64Ptr(71.5),
					MemoryUsedBytes:         int64Ptr(61_419_929_600),
					MemoryTotalBytes:        int64Ptr(85_899_345_920),
					NetInBytesPerSecond:     float64Ptr(1_960_000),
					NetOutBytesPerSecond:    float64Ptr(1_420_000),
					DiskReadBytesPerSecond:  float64Ptr(3_180_000),
					DiskWriteBytesPerSecond: float64Ptr(2_540_000),
				},
				TriggeredAlarms: []InventoryAlarm{{
					Alarm:         "alarm-401",
					Name:          "Host memory contention",
					OverallStatus: "yellow",
					TriggeredAt:   collectedAt.Add(-11 * time.Minute),
				}},
				RecentTasks: []InventoryTask{{
					Task:      "task-302",
					Name:      "vMotion virtual machine",
					State:     "running",
					StartedAt: collectedAt.Add(-3 * time.Minute),
				}},
			},
			{
				Host:                "host-103",
				Name:                "esxi-03.lab.local",
				ConnectionState:     "CONNECTED",
				PowerState:          "POWERED_ON",
				HostUUID:            "uuid-host-103",
				DatacenterID:        "datacenter-1",
				DatacenterName:      "Primary DC",
				ComputeResourceID:   "domain-c101",
				ComputeResourceName: "Production Cluster",
				ClusterID:           "domain-c101",
				ClusterName:         "Production Cluster",
				FolderID:            "group-h4",
				FolderName:          "Cluster Hosts",
				DatastoreIDs:        []string{"datastore-201", "datastore-203"},
				DatastoreNames:      []string{"nvme-primary", "analytics-vsan"},
				OverallStatus:       "green",
				Metrics: &InventoryMetrics{
					CPUPercent:              float64Ptr(18.9),
					MemoryPercent:           float64Ptr(49.2),
					MemoryUsedBytes:         int64Ptr(42_271_203_328),
					MemoryTotalBytes:        int64Ptr(85_899_345_920),
					NetInBytesPerSecond:     float64Ptr(980_000),
					NetOutBytesPerSecond:    float64Ptr(910_000),
					DiskReadBytesPerSecond:  float64Ptr(1_860_000),
					DiskWriteBytesPerSecond: float64Ptr(1_220_000),
				},
				RecentTasks: []InventoryTask{{
					Task:      "task-303",
					Name:      "Enter maintenance mode check",
					State:     "success",
					StartedAt: collectedAt.Add(-43 * time.Minute),
				}},
			},
			{
				Host:                "host-104",
				Name:                "esxi-04.lab.local",
				ConnectionState:     "CONNECTED",
				PowerState:          "POWERED_ON",
				HostUUID:            "uuid-host-104",
				DatacenterID:        "datacenter-1",
				DatacenterName:      "Primary DC",
				ComputeResourceID:   "domain-c101",
				ComputeResourceName: "Production Cluster",
				ClusterID:           "domain-c101",
				ClusterName:         "Production Cluster",
				FolderID:            "group-h4",
				FolderName:          "Cluster Hosts",
				DatastoreIDs:        []string{"datastore-202", "datastore-203", "datastore-204"},
				DatastoreNames:      []string{"archive-tier", "analytics-vsan", "backup-nfs"},
				OverallStatus:       "green",
				Metrics: &InventoryMetrics{
					CPUPercent:              float64Ptr(31.4),
					MemoryPercent:           float64Ptr(63.7),
					MemoryUsedBytes:         int64Ptr(54_722_805_760),
					MemoryTotalBytes:        int64Ptr(85_899_345_920),
					NetInBytesPerSecond:     float64Ptr(1_420_000),
					NetOutBytesPerSecond:    float64Ptr(1_260_000),
					DiskReadBytesPerSecond:  float64Ptr(2_660_000),
					DiskWriteBytesPerSecond: float64Ptr(2_180_000),
				},
				RecentEvents: []InventoryEvent{{
					Event:     "event-602",
					Type:      "HostProfileAppliedEvent",
					Message:   "Host profile compliance restored after storage heartbeat remediation",
					User:      "svc-pulse",
					CreatedAt: collectedAt.Add(-67 * time.Minute),
				}},
			},
		},
		VMs: []InventoryVM{
			{
				VM:                  "vm-201",
				Name:                "orders-api-01",
				PowerState:          "POWERED_ON",
				CPUCount:            4,
				MemorySizeMiB:       16 * 1024,
				DatacenterID:        "datacenter-1",
				DatacenterName:      "Primary DC",
				ComputeResourceID:   "domain-c101",
				ComputeResourceName: "Production Cluster",
				ClusterID:           "domain-c101",
				ClusterName:         "Production Cluster",
				FolderID:            "group-v7",
				FolderName:          "Production VMs",
				ResourcePoolID:      "resgroup-22",
				ResourcePoolName:    "Tier 1",
				RuntimeHostID:       "host-101",
				RuntimeHostName:     "esxi-01.lab.local",
				DatastoreIDs:        []string{"datastore-201"},
				DatastoreNames:      []string{"nvme-primary"},
				InstanceUUID:        "vm-instance-201",
				BIOSUUID:            "vm-bios-201",
				GuestOSFamily:       "LINUX",
				GuestHostname:       "orders-api-01.internal",
				GuestIPAddresses:    []string{"10.42.10.21"},
				OverallStatus:       "green",
				SnapshotCount:       1,
				CurrentSnapshotID:   "snapshot-201",
				NetworkAdapters:     fixtureVMNetworkAdapters("vm-201", "VM Network", true),
				VirtualDisks:        fixtureVMVirtualDisks("vm-201", "nvme-primary", 128_000_000_000),
				Tools:               fixtureVMTools("POWERED_ON", "green"),
				Hardware:            fixtureVMHardware("UBUNTU_64", "VMX_20", true),
				SnapshotTree: []InventoryVMSnapshot{{
					Snapshot:    "snapshot-201",
					Name:        "pre-deploy-checkpoint",
					Description: "Before service deployment",
					ID:          201,
					CreatedAt:   inventorySnapshotTime(collectedAt.Add(-46 * time.Minute)),
					State:       "poweredOn",
					Quiesced:    true,
					Current:     true,
				}},
				Metrics: &InventoryMetrics{
					CPUPercent:              float64Ptr(36.2),
					MemoryPercent:           float64Ptr(64.4),
					MemoryUsedBytes:         int64Ptr(11_058_069_504),
					MemoryTotalBytes:        int64Ptr(17_179_869_184),
					NetInBytesPerSecond:     float64Ptr(620_000),
					NetOutBytesPerSecond:    float64Ptr(710_000),
					DiskReadBytesPerSecond:  float64Ptr(1_180_000),
					DiskWriteBytesPerSecond: float64Ptr(940_000),
				},
				RecentEvents: []InventoryEvent{{
					Event:     "event-501",
					Type:      "VmMessageEvent",
					Message:   "Snapshot completed successfully",
					User:      "svc-pulse",
					CreatedAt: collectedAt.Add(-46 * time.Minute),
				}},
			},
			{
				VM:                  "vm-202",
				Name:                "postgres-ha-01",
				PowerState:          "POWERED_ON",
				CPUCount:            8,
				MemorySizeMiB:       32 * 1024,
				DatacenterID:        "datacenter-1",
				DatacenterName:      "Primary DC",
				ComputeResourceID:   "domain-c101",
				ComputeResourceName: "Production Cluster",
				ClusterID:           "domain-c101",
				ClusterName:         "Production Cluster",
				FolderID:            "group-v7",
				FolderName:          "Production VMs",
				ResourcePoolID:      "resgroup-23",
				ResourcePoolName:    "Stateful",
				RuntimeHostID:       "host-102",
				RuntimeHostName:     "esxi-02.lab.local",
				DatastoreIDs:        []string{"datastore-201", "datastore-202"},
				DatastoreNames:      []string{"nvme-primary", "archive-tier"},
				InstanceUUID:        "vm-instance-202",
				BIOSUUID:            "vm-bios-202",
				GuestOSFamily:       "LINUX",
				GuestHostname:       "postgres-ha-01.internal",
				GuestIPAddresses:    []string{"10.42.10.32"},
				OverallStatus:       "yellow",
				SnapshotCount:       3,
				CurrentSnapshotID:   "snapshot-213",
				NetworkAdapters:     fixtureVMNetworkAdapters("vm-202", "Database Network", true),
				VirtualDisks:        fixtureVMVirtualDisks("vm-202", "nvme-primary", 256_000_000_000),
				Tools:               fixtureVMTools("POWERED_ON", "yellow"),
				Hardware:            fixtureVMHardware("RHEL_8_64", "VMX_19", true),
				SnapshotTree: []InventoryVMSnapshot{{
					Snapshot:  "snapshot-211",
					Name:      "baseline",
					ID:        211,
					CreatedAt: inventorySnapshotTime(collectedAt.Add(-72 * time.Hour)),
					State:     "poweredOn",
					Quiesced:  true,
					Children: []InventoryVMSnapshot{{
						Snapshot:  "snapshot-212",
						Name:      "schema-migration",
						ID:        212,
						CreatedAt: inventorySnapshotTime(collectedAt.Add(-36 * time.Hour)),
						State:     "poweredOn",
						Quiesced:  true,
						Children: []InventoryVMSnapshot{{
							Snapshot:  "snapshot-213",
							Name:      "post-migration",
							ID:        213,
							CreatedAt: inventorySnapshotTime(collectedAt.Add(-18 * time.Hour)),
							State:     "poweredOn",
							Current:   true,
						}},
					}},
				}},
				Metrics: &InventoryMetrics{
					CPUPercent:              float64Ptr(58.8),
					MemoryPercent:           float64Ptr(82.1),
					MemoryUsedBytes:         int64Ptr(28_215_830_528),
					MemoryTotalBytes:        int64Ptr(34_359_738_368),
					NetInBytesPerSecond:     float64Ptr(880_000),
					NetOutBytesPerSecond:    float64Ptr(930_000),
					DiskReadBytesPerSecond:  float64Ptr(2_940_000),
					DiskWriteBytesPerSecond: float64Ptr(3_220_000),
				},
				TriggeredAlarms: []InventoryAlarm{{
					Alarm:         "alarm-402",
					Name:          "Virtual machine memory usage",
					OverallStatus: "yellow",
					TriggeredAt:   collectedAt.Add(-17 * time.Minute),
				}},
				RecentTasks: []InventoryTask{{
					Task:      "task-304",
					Name:      "Consolidate virtual machine disks",
					State:     "queued",
					StartedAt: collectedAt.Add(-2 * time.Minute),
				}},
			},
			{
				VM:                  "vm-203",
				Name:                "web-frontend-01",
				PowerState:          "POWERED_ON",
				CPUCount:            4,
				MemorySizeMiB:       12 * 1024,
				DatacenterID:        "datacenter-1",
				DatacenterName:      "Primary DC",
				ComputeResourceID:   "domain-c101",
				ComputeResourceName: "Production Cluster",
				ClusterID:           "domain-c101",
				ClusterName:         "Production Cluster",
				FolderID:            "group-v7",
				FolderName:          "Production VMs",
				ResourcePoolID:      "resgroup-22",
				ResourcePoolName:    "Tier 1",
				RuntimeHostID:       "host-103",
				RuntimeHostName:     "esxi-03.lab.local",
				DatastoreIDs:        []string{"datastore-203"},
				DatastoreNames:      []string{"analytics-vsan"},
				InstanceUUID:        "vm-instance-203",
				BIOSUUID:            "vm-bios-203",
				GuestOSFamily:       "LINUX",
				GuestHostname:       "web-frontend-01.internal",
				GuestIPAddresses:    []string{"10.42.10.44"},
				OverallStatus:       "green",
				SnapshotCount:       1,
				NetworkAdapters:     fixtureVMNetworkAdapters("vm-203", "Web Network", true),
				VirtualDisks:        fixtureVMVirtualDisks("vm-203", "analytics-vsan", 96_000_000_000),
				Tools:               fixtureVMTools("POWERED_ON", "green"),
				Hardware:            fixtureVMHardware("UBUNTU_64", "VMX_20", true),
				Metrics: &InventoryMetrics{
					CPUPercent:              float64Ptr(22.1),
					MemoryPercent:           float64Ptr(51.2),
					MemoryUsedBytes:         int64Ptr(6_589_939_712),
					MemoryTotalBytes:        int64Ptr(12_884_901_888),
					NetInBytesPerSecond:     float64Ptr(510_000),
					NetOutBytesPerSecond:    float64Ptr(690_000),
					DiskReadBytesPerSecond:  float64Ptr(880_000),
					DiskWriteBytesPerSecond: float64Ptr(620_000),
				},
			},
			{
				VM:                  "vm-204",
				Name:                "observability-01",
				PowerState:          "POWERED_ON",
				CPUCount:            8,
				MemorySizeMiB:       24 * 1024,
				DatacenterID:        "datacenter-1",
				DatacenterName:      "Primary DC",
				ComputeResourceID:   "domain-c101",
				ComputeResourceName: "Production Cluster",
				ClusterID:           "domain-c101",
				ClusterName:         "Production Cluster",
				FolderID:            "group-v9",
				FolderName:          "Platform VMs",
				ResourcePoolID:      "resgroup-25",
				ResourcePoolName:    "Platform",
				RuntimeHostID:       "host-104",
				RuntimeHostName:     "esxi-04.lab.local",
				DatastoreIDs:        []string{"datastore-203", "datastore-204"},
				DatastoreNames:      []string{"analytics-vsan", "backup-nfs"},
				InstanceUUID:        "vm-instance-204",
				BIOSUUID:            "vm-bios-204",
				GuestOSFamily:       "LINUX",
				GuestHostname:       "observability-01.internal",
				GuestIPAddresses:    []string{"10.42.20.15"},
				OverallStatus:       "green",
				SnapshotCount:       2,
				NetworkAdapters:     fixtureVMNetworkAdapters("vm-204", "Platform Network", true),
				VirtualDisks:        fixtureVMVirtualDisks("vm-204", "analytics-vsan", 192_000_000_000),
				Tools:               fixtureVMTools("POWERED_ON", "green"),
				Hardware:            fixtureVMHardware("UBUNTU_64", "VMX_20", true),
				Metrics: &InventoryMetrics{
					CPUPercent:              float64Ptr(27.8),
					MemoryPercent:           float64Ptr(59.6),
					MemoryUsedBytes:         int64Ptr(15_367_036_928),
					MemoryTotalBytes:        int64Ptr(25_769_803_776),
					NetInBytesPerSecond:     float64Ptr(740_000),
					NetOutBytesPerSecond:    float64Ptr(680_000),
					DiskReadBytesPerSecond:  float64Ptr(1_160_000),
					DiskWriteBytesPerSecond: float64Ptr(1_020_000),
				},
			},
			{
				VM:                  "vm-205",
				Name:                "windows-jump-01",
				PowerState:          "POWERED_ON",
				CPUCount:            4,
				MemorySizeMiB:       8 * 1024,
				DatacenterID:        "datacenter-1",
				DatacenterName:      "Primary DC",
				ComputeResourceID:   "domain-c101",
				ComputeResourceName: "Production Cluster",
				ClusterID:           "domain-c101",
				ClusterName:         "Production Cluster",
				FolderID:            "group-v10",
				FolderName:          "Utility VMs",
				ResourcePoolID:      "resgroup-26",
				ResourcePoolName:    "Utility",
				RuntimeHostID:       "host-103",
				RuntimeHostName:     "esxi-03.lab.local",
				DatastoreIDs:        []string{"datastore-203"},
				DatastoreNames:      []string{"analytics-vsan"},
				InstanceUUID:        "vm-instance-205",
				BIOSUUID:            "vm-bios-205",
				GuestOSFamily:       "WINDOWS",
				GuestHostname:       "windows-jump-01.internal",
				GuestIPAddresses:    []string{"10.42.30.18"},
				OverallStatus:       "green",
				SnapshotCount:       0,
				NetworkAdapters:     fixtureVMNetworkAdapters("vm-205", "Utility Network", true),
				VirtualDisks:        fixtureVMVirtualDisks("vm-205", "analytics-vsan", 80_000_000_000),
				Tools:               fixtureVMTools("POWERED_ON", "green"),
				Hardware:            fixtureVMHardware("WINDOWS_2019_64", "VMX_19", true),
				Metrics: &InventoryMetrics{
					CPUPercent:              float64Ptr(16.8),
					MemoryPercent:           float64Ptr(46.1),
					MemoryUsedBytes:         int64Ptr(3_959_726_080),
					MemoryTotalBytes:        int64Ptr(8_589_934_592),
					NetInBytesPerSecond:     float64Ptr(180_000),
					NetOutBytesPerSecond:    float64Ptr(220_000),
					DiskReadBytesPerSecond:  float64Ptr(320_000),
					DiskWriteBytesPerSecond: float64Ptr(280_000),
				},
			},
			{
				VM:                  "vm-206",
				Name:                "batch-worker-01",
				PowerState:          "POWERED_OFF",
				CPUCount:            4,
				MemorySizeMiB:       8 * 1024,
				DatacenterID:        "datacenter-1",
				DatacenterName:      "Primary DC",
				ComputeResourceID:   "domain-c101",
				ComputeResourceName: "Production Cluster",
				ClusterID:           "domain-c101",
				ClusterName:         "Production Cluster",
				FolderID:            "group-v8",
				FolderName:          "Batch VMs",
				ResourcePoolID:      "resgroup-24",
				ResourcePoolName:    "Workers",
				RuntimeHostID:       "host-101",
				RuntimeHostName:     "esxi-01.lab.local",
				DatastoreIDs:        []string{"datastore-202"},
				DatastoreNames:      []string{"archive-tier"},
				InstanceUUID:        "vm-instance-206",
				BIOSUUID:            "vm-bios-206",
				GuestOSFamily:       "LINUX",
				GuestHostname:       "batch-worker-01.internal",
				GuestIPAddresses:    []string{},
				OverallStatus:       "gray",
				VirtualDisks:        fixtureVMVirtualDisks("vm-206", "archive-tier", 64_000_000_000),
				Tools:               fixtureVMTools("POWERED_OFF", "gray"),
				Hardware:            fixtureVMHardware("OTHER_64", "VMX_17", false),
			},
		},
		Datastores: []InventoryDatastore{
			{
				Datastore:          "datastore-201",
				Name:               "nvme-primary",
				Type:               "VMFS",
				FreeSpace:          4_600_000_000_000,
				Capacity:           8_000_000_000_000,
				DatacenterID:       "datacenter-1",
				DatacenterName:     "Primary DC",
				FolderID:           "group-s4",
				FolderName:         "Datastores",
				HostIDs:            []string{"host-101", "host-102", "host-103"},
				HostNames:          []string{"esxi-01.lab.local", "esxi-02.lab.local", "esxi-03.lab.local"},
				VMIDs:              []string{"vm-201", "vm-202"},
				VMNames:            []string{"orders-api-01", "postgres-ha-01"},
				Accessible:         &accessible,
				MultipleHostAccess: &multipleHostAccess,
				MaintenanceMode:    "normal",
				URL:                "ds:///vmfs/volumes/datastore-201/",
				OverallStatus:      "green",
			},
			{
				Datastore:          "datastore-202",
				Name:               "archive-tier",
				Type:               "NFS41",
				FreeSpace:          12_400_000_000_000,
				Capacity:           16_000_000_000_000,
				DatacenterID:       "datacenter-1",
				DatacenterName:     "Primary DC",
				FolderID:           "group-s4",
				FolderName:         "Datastores",
				HostIDs:            []string{"host-101", "host-104"},
				HostNames:          []string{"esxi-01.lab.local", "esxi-04.lab.local"},
				VMIDs:              []string{"vm-202", "vm-206"},
				VMNames:            []string{"postgres-ha-01", "batch-worker-01"},
				Accessible:         &accessible,
				MultipleHostAccess: &multipleHostAccess,
				MaintenanceMode:    "normal",
				URL:                "ds:///nfs/archive-tier/",
				OverallStatus:      "yellow",
				TriggeredAlarms: []InventoryAlarm{{
					Alarm:         "alarm-403",
					Name:          "Datastore latency above threshold",
					OverallStatus: "yellow",
					TriggeredAt:   collectedAt.Add(-9 * time.Minute),
				}},
				RecentTasks: []InventoryTask{{
					Task:      "task-305",
					Name:      "Rescan datastore",
					State:     "success",
					StartedAt: collectedAt.Add(-14 * time.Minute),
				}},
			},
			{
				Datastore:          "datastore-203",
				Name:               "analytics-vsan",
				Type:               "vSAN",
				FreeSpace:          6_900_000_000_000,
				Capacity:           12_000_000_000_000,
				DatacenterID:       "datacenter-1",
				DatacenterName:     "Primary DC",
				FolderID:           "group-s4",
				FolderName:         "Datastores",
				HostIDs:            []string{"host-101", "host-103", "host-104"},
				HostNames:          []string{"esxi-01.lab.local", "esxi-03.lab.local", "esxi-04.lab.local"},
				VMIDs:              []string{"vm-203", "vm-204", "vm-205"},
				VMNames:            []string{"web-frontend-01", "observability-01", "windows-jump-01"},
				Accessible:         &accessible,
				MultipleHostAccess: &multipleHostAccess,
				MaintenanceMode:    "normal",
				URL:                "ds:///vsan/analytics-vsan/",
				OverallStatus:      "green",
			},
			{
				Datastore:          "datastore-204",
				Name:               "backup-nfs",
				Type:               "NFS41",
				FreeSpace:          20_800_000_000_000,
				Capacity:           24_000_000_000_000,
				DatacenterID:       "datacenter-1",
				DatacenterName:     "Primary DC",
				FolderID:           "group-s4",
				FolderName:         "Datastores",
				HostIDs:            []string{"host-102", "host-104"},
				HostNames:          []string{"esxi-02.lab.local", "esxi-04.lab.local"},
				VMIDs:              []string{"vm-204"},
				VMNames:            []string{"observability-01"},
				Accessible:         &accessible,
				MultipleHostAccess: &multipleHostAccess,
				MaintenanceMode:    "normal",
				URL:                "ds:///nfs/backup-nfs/",
				OverallStatus:      "green",
				RecentTasks: []InventoryTask{{
					Task:      "task-306",
					Name:      "Backup consistency check",
					State:     "success",
					StartedAt: collectedAt.Add(-74 * time.Minute),
				}},
			},
		},
	}
}

// appendEdgeClusterFixtures grows the canonical VMware mock inventory toward
// a mature SMB lab. The edge cluster lives in a second datacenter and adds
// 3 more ESXi hosts, 12 more VMs across a tier-1 application set + a
// stateful tier + a windows fleet, and 4 more datastores (one of each
// canonical storage technology) so the platform-page tables exercise
// grouping, sorting, and responsive density beyond a single small cluster.
func appendEdgeClusterFixtures(
	snapshot *InventorySnapshot,
	collectedAt time.Time,
	accessible bool,
	multipleHostAccess bool,
) {
	const (
		edgeDatacenterID   = "datacenter-2"
		edgeDatacenterName = "Edge DC"
		edgeClusterID      = "domain-c201"
		edgeClusterName    = "Edge Services"
		edgeFolderHosts    = "group-h6"
		edgeFolderHostsLbl = "Edge Cluster Hosts"
		edgeFolderVMs      = "group-v9"
		edgeFolderVMsLbl   = "Edge Production VMs"
		edgeFolderDS       = "group-s6"
		edgeFolderDSLbl    = "Edge Datastores"
	)

	edgeDatastoreIDs := []string{"datastore-301", "datastore-302", "datastore-303", "datastore-304"}
	edgeDatastoreNames := []string{"edge-nvme-tier", "edge-warm-nfs", "edge-vsan", "edge-cold-iscsi"}

	hostSpecs := []struct {
		ID       string
		Name     string
		UUID     string
		CPUPct   float64
		MemPct   float64
		MemUsed  int64
		MemTotal int64
		Status   string
	}{
		{"host-201", "esxi-05.lab.local", "uuid-host-201", 27.4, 51.6, 44_277_059_584, 85_899_345_920, "green"},
		{"host-202", "esxi-06.lab.local", "uuid-host-202", 41.2, 67.9, 58_271_233_120, 85_899_345_920, "green"},
		{"host-203", "esxi-07.lab.local", "uuid-host-203", 19.8, 38.4, 32_968_499_584, 85_899_345_920, "yellow"},
	}

	hostNames := make([]string, 0, len(hostSpecs))
	for _, h := range hostSpecs {
		hostNames = append(hostNames, h.Name)
	}

	for _, h := range hostSpecs {
		snapshot.Hosts = append(snapshot.Hosts, InventoryHost{
			Host:                h.ID,
			Name:                h.Name,
			ConnectionState:     "CONNECTED",
			PowerState:          "POWERED_ON",
			HostUUID:            h.UUID,
			DatacenterID:        edgeDatacenterID,
			DatacenterName:      edgeDatacenterName,
			ComputeResourceID:   edgeClusterID,
			ComputeResourceName: edgeClusterName,
			ClusterID:           edgeClusterID,
			ClusterName:         edgeClusterName,
			FolderID:            edgeFolderHosts,
			FolderName:          edgeFolderHostsLbl,
			DatastoreIDs:        edgeDatastoreIDs,
			DatastoreNames:      edgeDatastoreNames,
			OverallStatus:       h.Status,
			Metrics: &InventoryMetrics{
				CPUPercent:              float64Ptr(h.CPUPct),
				MemoryPercent:           float64Ptr(h.MemPct),
				MemoryUsedBytes:         int64Ptr(h.MemUsed),
				MemoryTotalBytes:        int64Ptr(h.MemTotal),
				NetInBytesPerSecond:     float64Ptr(960_000),
				NetOutBytesPerSecond:    float64Ptr(820_000),
				DiskReadBytesPerSecond:  float64Ptr(1_820_000),
				DiskWriteBytesPerSecond: float64Ptr(1_540_000),
			},
		})
	}

	vmSpecs := []struct {
		ID         string
		Name       string
		Host       string
		CPUCount   int
		MemMiB     int64
		Tier       string
		Datastores []string
		PowerState string
		Status     string
		Guest      string
	}{
		{"vm-301", "edge-api-01", "host-201", 4, 16 * 1024, "Tier 1", []string{"datastore-301"}, "POWERED_ON", "green", "edge-api-01.internal"},
		{"vm-302", "edge-api-02", "host-202", 4, 16 * 1024, "Tier 1", []string{"datastore-301"}, "POWERED_ON", "green", "edge-api-02.internal"},
		{"vm-303", "mariadb-replica-01", "host-203", 8, 32 * 1024, "Stateful", []string{"datastore-302"}, "POWERED_ON", "yellow", "mariadb-replica-01.internal"},
		{"vm-304", "redis-cache-01", "host-201", 2, 8 * 1024, "Stateful", []string{"datastore-303"}, "POWERED_ON", "green", "redis-cache-01.internal"},
		{"vm-305", "redis-cache-02", "host-202", 2, 8 * 1024, "Stateful", []string{"datastore-303"}, "POWERED_ON", "green", "redis-cache-02.internal"},
		{"vm-306", "ingress-proxy-01", "host-201", 4, 12 * 1024, "Tier 1", []string{"datastore-301"}, "POWERED_ON", "green", "ingress-proxy-01.internal"},
		{"vm-307", "ingress-proxy-02", "host-203", 4, 12 * 1024, "Tier 1", []string{"datastore-301"}, "POWERED_ON", "green", "ingress-proxy-02.internal"},
		{"vm-308", "win-fleet-rdp-01", "host-202", 4, 16 * 1024, "Workstations", []string{"datastore-303"}, "POWERED_ON", "green", "win-fleet-rdp-01.lab.local"},
		{"vm-309", "win-fleet-rdp-02", "host-203", 4, 16 * 1024, "Workstations", []string{"datastore-303"}, "POWERED_ON", "green", "win-fleet-rdp-02.lab.local"},
		{"vm-310", "logging-collector-01", "host-201", 4, 24 * 1024, "Observability", []string{"datastore-302"}, "POWERED_ON", "green", "logging-collector-01.internal"},
		{"vm-311", "logging-collector-02", "host-202", 4, 24 * 1024, "Observability", []string{"datastore-302"}, "POWERED_ON", "yellow", "logging-collector-02.internal"},
		{"vm-312", "cold-archive-01", "host-203", 2, 8 * 1024, "Archive", []string{"datastore-304"}, "POWERED_OFF", "gray", ""},
	}

	for _, v := range vmSpecs {
		hostName := ""
		for _, hs := range hostSpecs {
			if hs.ID == v.Host {
				hostName = hs.Name
				break
			}
		}
		dsNames := make([]string, 0, len(v.Datastores))
		for _, dsID := range v.Datastores {
			for i, id := range edgeDatastoreIDs {
				if id == dsID {
					dsNames = append(dsNames, edgeDatastoreNames[i])
				}
			}
		}
		cpuPct := 28.0
		memPct := 60.0
		if v.Status == "yellow" {
			cpuPct = 64.0
			memPct = 81.0
		} else if v.Status == "gray" {
			cpuPct = 0
			memPct = 0
		}
		guestIPs := []string{}
		if v.PowerState == "POWERED_ON" {
			guestIPs = []string{fmt.Sprintf("10.50.20.%d", 10+len(snapshot.VMs))}
		}
		vm := InventoryVM{
			VM:                  v.ID,
			Name:                v.Name,
			PowerState:          v.PowerState,
			CPUCount:            v.CPUCount,
			MemorySizeMiB:       v.MemMiB,
			DatacenterID:        edgeDatacenterID,
			DatacenterName:      edgeDatacenterName,
			ComputeResourceID:   edgeClusterID,
			ComputeResourceName: edgeClusterName,
			ClusterID:           edgeClusterID,
			ClusterName:         edgeClusterName,
			FolderID:            edgeFolderVMs,
			FolderName:          edgeFolderVMsLbl,
			ResourcePoolID:      fmt.Sprintf("resgroup-edge-%s", v.Tier),
			ResourcePoolName:    v.Tier,
			RuntimeHostID:       v.Host,
			RuntimeHostName:     hostName,
			DatastoreIDs:        v.Datastores,
			DatastoreNames:      dsNames,
			InstanceUUID:        fmt.Sprintf("vm-instance-%s", v.ID),
			BIOSUUID:            fmt.Sprintf("vm-bios-%s", v.ID),
			GuestOSFamily:       guestOSFamily(v.Name),
			GuestHostname:       v.Guest,
			GuestIPAddresses:    guestIPs,
			OverallStatus:       v.Status,
			NetworkAdapters:     fixtureVMNetworkAdapters(v.ID, edgeVMNetworkName(v.Tier), v.PowerState == "POWERED_ON"),
			VirtualDisks:        fixtureVMVirtualDisks(v.ID, firstNonEmptyTrimmed(dsNames...), int64(v.MemMiB)*1024*1024*4),
			Tools:               fixtureVMTools(v.PowerState, v.Status),
			Hardware:            fixtureVMHardware("UBUNTU_64", "VMX_20", v.PowerState == "POWERED_ON"),
		}
		if v.PowerState == "POWERED_ON" {
			vm.Metrics = &InventoryMetrics{
				CPUPercent:              float64Ptr(cpuPct),
				MemoryPercent:           float64Ptr(memPct),
				MemoryUsedBytes:         int64Ptr(int64(float64(v.MemMiB) * float64(memPct) / 100.0 * 1024 * 1024)),
				MemoryTotalBytes:        int64Ptr(int64(v.MemMiB) * 1024 * 1024),
				NetInBytesPerSecond:     float64Ptr(360_000),
				NetOutBytesPerSecond:    float64Ptr(420_000),
				DiskReadBytesPerSecond:  float64Ptr(720_000),
				DiskWriteBytesPerSecond: float64Ptr(610_000),
			}
		}
		snapshot.VMs = append(snapshot.VMs, vm)
	}

	type datastoreSpec struct {
		ID       string
		Name     string
		Type     string
		Free     int64
		Cap      int64
		HostIDs  []string
		HostName []string
		VMIDs    []string
		VMNames  []string
		URL      string
		Status   string
	}
	dsList := []datastoreSpec{
		{"datastore-301", "edge-nvme-tier", "VMFS", 3_200_000_000_000, 6_000_000_000_000, []string{"host-201", "host-202", "host-203"}, hostNames, []string{"vm-301", "vm-302", "vm-306", "vm-307"}, []string{"edge-api-01", "edge-api-02", "ingress-proxy-01", "ingress-proxy-02"}, "ds:///vmfs/volumes/datastore-301/", "green"},
		{"datastore-302", "edge-warm-nfs", "NFS41", 9_100_000_000_000, 12_000_000_000_000, []string{"host-201", "host-203"}, []string{"esxi-05.lab.local", "esxi-07.lab.local"}, []string{"vm-303", "vm-310", "vm-311"}, []string{"mariadb-replica-01", "logging-collector-01", "logging-collector-02"}, "ds:///nfs/edge-warm-nfs/", "green"},
		{"datastore-303", "edge-vsan", "vSAN", 4_900_000_000_000, 9_000_000_000_000, []string{"host-201", "host-202", "host-203"}, hostNames, []string{"vm-304", "vm-305", "vm-308", "vm-309"}, []string{"redis-cache-01", "redis-cache-02", "win-fleet-rdp-01", "win-fleet-rdp-02"}, "ds:///vsan/edge-vsan/", "green"},
		{"datastore-304", "edge-cold-iscsi", "VMFS", 18_500_000_000_000, 24_000_000_000_000, []string{"host-202", "host-203"}, []string{"esxi-06.lab.local", "esxi-07.lab.local"}, []string{"vm-312"}, []string{"cold-archive-01"}, "ds:///vmfs/volumes/datastore-304/", "yellow"},
	}
	for _, ds := range dsList {
		snapshot.Datastores = append(snapshot.Datastores, InventoryDatastore{
			Datastore:          ds.ID,
			Name:               ds.Name,
			Type:               ds.Type,
			FreeSpace:          ds.Free,
			Capacity:           ds.Cap,
			DatacenterID:       edgeDatacenterID,
			DatacenterName:     edgeDatacenterName,
			FolderID:           edgeFolderDS,
			FolderName:         edgeFolderDSLbl,
			HostIDs:            ds.HostIDs,
			HostNames:          ds.HostName,
			VMIDs:              ds.VMIDs,
			VMNames:            ds.VMNames,
			Accessible:         &accessible,
			MultipleHostAccess: &multipleHostAccess,
			MaintenanceMode:    "normal",
			URL:                ds.URL,
			OverallStatus:      ds.Status,
		})
	}
}

func guestOSFamily(name string) string {
	if name == "" {
		return ""
	}
	switch name[:3] {
	case "win":
		return "WINDOWS"
	default:
		return "LINUX"
	}
}

func edgeVMNetworkName(tier string) string {
	switch tier {
	case "Stateful":
		return "Edge Stateful"
	case "Archive":
		return "Edge Archive"
	case "Workstations":
		return "Edge Workstations"
	case "Observability":
		return "Edge Observability"
	default:
		return "Edge App"
	}
}

func fixtureVMNetworkAdapters(vmID, networkName string, connected bool) []InventoryVMNetworkAdapter {
	state := "NOT_CONNECTED"
	if connected {
		state = "CONNECTED"
	}
	return []InventoryVMNetworkAdapter{{
		NIC:               "4000",
		Label:             "Network adapter 1",
		Type:              "VMXNET3",
		MACType:           "GENERATED",
		MACAddress:        fixtureVMMACAddress(vmID),
		PCISlotNumber:     int64Ptr(160),
		BackingType:       "STANDARD_PORTGROUP",
		NetworkID:         "network-" + vmID,
		NetworkName:       networkName,
		State:             state,
		StartConnected:    true,
		AllowGuestControl: true,
		WakeOnLANEnabled:  true,
	}}
}

func fixtureVMVirtualDisks(vmID, datastoreName string, capacityBytes int64) []InventoryVMVirtualDisk {
	if capacityBytes <= 0 {
		capacityBytes = 64_000_000_000
	}
	datastoreName = firstNonEmptyTrimmed(datastoreName, "vmfs-primary")
	return []InventoryVMVirtualDisk{{
		Disk:          "2000",
		Label:         "Hard disk 1",
		Type:          "SCSI",
		SCSIBus:       int64Ptr(0),
		SCSIUnit:      int64Ptr(0),
		BackingType:   "VMDK_FILE",
		VMDKFile:      fmt.Sprintf("[%s] %s/%s.vmdk", datastoreName, vmID, vmID),
		DatastoreName: datastoreName,
		CapacityBytes: int64Ptr(capacityBytes),
	}}
}

func fixtureVMTools(powerState, status string) *InventoryVMTools {
	autoUpdateSupported := true
	versionNumber := int64(12352)
	runState := "RUNNING"
	versionStatus := "CURRENT"
	if powerState != "POWERED_ON" {
		runState = "NOT_RUNNING"
	}
	if status == "yellow" {
		versionStatus = "SUPPORTED_OLD"
	}
	return &InventoryVMTools{
		AutoUpdateSupported: boolPointer(autoUpdateSupported),
		VersionNumber:       &versionNumber,
		Version:             "12.4.0",
		UpgradePolicy:       "MANUAL",
		VersionStatus:       versionStatus,
		InstallType:         "OPEN_VM_TOOLS",
		RunState:            runState,
	}
}

func fixtureVMHardware(guestOS, version string, hotAddEnabled bool) *InventoryVMHardware {
	instantCloneFrozen := false
	efiLegacyBoot := false
	bootDelayMilliseconds := int64(0)
	bootRetry := false
	bootRetryDelayMilliseconds := int64(10000)
	enterSetupMode := false
	coresPerSocket := int64(2)
	memoryHotAddIncrementMiB := int64(256)
	memoryHotAddLimitMiB := int64(16 * 1024)
	return &InventoryVMHardware{
		GuestOS:                    guestOS,
		InstantCloneFrozen:         boolPointer(instantCloneFrozen),
		Version:                    version,
		UpgradePolicy:              "NEVER",
		UpgradeStatus:              "NONE",
		BootType:                   "EFI",
		EFILegacyBoot:              &efiLegacyBoot,
		BootNetworkProtocol:        "IPV4",
		BootDelayMilliseconds:      &bootDelayMilliseconds,
		BootRetry:                  &bootRetry,
		BootRetryDelayMilliseconds: &bootRetryDelayMilliseconds,
		EnterSetupMode:             &enterSetupMode,
		BootDevices: []InventoryVMBootDevice{{
			Type:  "DISK",
			Disks: []string{"2000"},
		}},
		CPUCoresPerSocket:        &coresPerSocket,
		CPUHotAddEnabled:         boolPointer(hotAddEnabled),
		CPUHotRemoveEnabled:      boolPointer(false),
		MemoryHotAddEnabled:      boolPointer(hotAddEnabled),
		MemoryHotAddIncrementMiB: &memoryHotAddIncrementMiB,
		MemoryHotAddLimitMiB:     &memoryHotAddLimitMiB,
	}
}

func boolPointer(value bool) *bool {
	return &value
}

func fixtureVMMACAddress(vmID string) string {
	var value int
	for _, r := range vmID {
		value += int(r)
	}
	return fmt.Sprintf("00:50:56:%02x:%02x:%02x", (value>>8)&0xff, value&0xff, (value+37)&0xff)
}

func inventorySnapshotTime(value time.Time) *time.Time {
	timestamp := value.UTC()
	return &timestamp
}
