package truenas

import "time"

// DefaultFixtures returns a curated TrueNAS demo snapshot for contract tests
// and frontend mock mode.
func DefaultFixtures() FixtureSnapshot {
	collectedAt := time.Date(2026, 3, 31, 11, 20, 0, 0, time.UTC)

	return FixtureSnapshot{
		CollectedAt: collectedAt,
		System: SystemInfo{
			Hostname:             "truenas-main",
			Version:              "TrueNAS-SCALE-24.10.2",
			Build:                "24.10.2.1",
			UptimeSeconds:        42 * 24 * 60 * 60,
			Healthy:              true,
			MachineID:            "truenas-main-machine-id",
			CPUCount:             16,
			MemoryTotalBytes:     truenasGiB(64),
			MemoryAvailableBytes: truenasGiB(22),
			CPUPercent:           38,
			NetInRate:            48_000_000,
			NetOutRate:           19_500_000,
			DiskReadRate:         7_200_000,
			DiskWriteRate:        3_400_000,
			TemperatureCelsius: map[string]float64{
				"cpu_package": 61.5,
				"cpu_core_0":  58.0,
				"cpu_core_1":  59.0,
			},
			IntervalSeconds: 2,
			CollectedAt:     collectedAt,
		},
		Pools: []Pool{
			{
				ID:         "pool-tank",
				Name:       "tank",
				Status:     "ONLINE",
				TotalBytes: truenasTiB(30),
				UsedBytes:  truenasTiB(12),
				FreeBytes:  truenasTiB(18),
			},
			{
				ID:         "pool-fast",
				Name:       "fast",
				Status:     "ONLINE",
				TotalBytes: truenasTiB(8),
				UsedBytes:  truenasGiB(2200),
				FreeBytes:  truenasGiB(5992),
			},
			{
				ID:         "pool-archive",
				Name:       "archive",
				Status:     "DEGRADED",
				TotalBytes: truenasTiB(72),
				UsedBytes:  truenasTiB(49),
				FreeBytes:  truenasTiB(23),
			},
			{
				ID:         "pool-vault",
				Name:       "vault",
				Status:     "ONLINE",
				TotalBytes: truenasTiB(12),
				UsedBytes:  truenasGiB(3200),
				FreeBytes:  truenasGiB(9088),
			},
		},
		Datasets: []Dataset{
			{
				ID:         "dataset-tank-apps",
				Name:       "tank/apps",
				Pool:       "tank",
				UsedBytes:  truenasTiB(5),
				AvailBytes: truenasTiB(13),
				Mounted:    true,
			},
			{
				ID:         "dataset-tank-media",
				Name:       "tank/media",
				Pool:       "tank",
				UsedBytes:  truenasTiB(6),
				AvailBytes: truenasTiB(20),
				Mounted:    true,
			},
			{
				ID:         "dataset-tank-projects",
				Name:       "tank/projects",
				Pool:       "tank",
				UsedBytes:  truenasTiB(2),
				AvailBytes: truenasTiB(24),
				Mounted:    true,
			},
			{
				ID:         "dataset-tank-photos",
				Name:       "tank/photos",
				Pool:       "tank",
				UsedBytes:  truenasGiB(1800),
				AvailBytes: truenasGiB(24_824),
				Mounted:    true,
			},
			{
				ID:         "dataset-fast-vm-images",
				Name:       "fast/vm-images",
				Pool:       "fast",
				UsedBytes:  truenasGiB(1200),
				AvailBytes: truenasGiB(5992),
				Mounted:    true,
			},
			{
				ID:         "dataset-fast-analytics",
				Name:       "fast/analytics",
				Pool:       "fast",
				UsedBytes:  truenasGiB(900),
				AvailBytes: truenasGiB(6292),
				Mounted:    true,
			},
			{
				ID:         "dataset-archive-backups",
				Name:       "archive/backups",
				Pool:       "archive",
				UsedBytes:  truenasTiB(24),
				AvailBytes: truenasTiB(18),
				Mounted:    true,
			},
			{
				ID:         "dataset-archive-cold",
				Name:       "archive/cold",
				Pool:       "archive",
				UsedBytes:  truenasTiB(19),
				AvailBytes: truenasTiB(5),
				Mounted:    true,
				ReadOnly:   true,
			},
			{
				ID:         "dataset-vault-compliance",
				Name:       "vault/compliance",
				Pool:       "vault",
				UsedBytes:  truenasGiB(2500),
				AvailBytes: truenasGiB(9788),
				Mounted:    true,
			},
		},
		Disks: []Disk{
			{
				ID:          "disk-sda",
				Name:        "sda",
				Pool:        "tank",
				Status:      "ONLINE",
				Model:       "Seagate Exos X18",
				Serial:      "ZL0A1234",
				SizeBytes:   truenasTiB(16),
				Temperature: 34,
				TemperatureAggregate: DiskTemperatureAggregate{
					WindowDays: 7,
					MinCelsius: 29.0,
					AvgCelsius: 32.7,
					MaxCelsius: 38.0,
				},
				Transport:  "sata",
				Rotational: true,
			},
			{
				ID:          "disk-sdb",
				Name:        "sdb",
				Pool:        "tank",
				Status:      "ONLINE",
				Model:       "Seagate Exos X18",
				Serial:      "ZL0A1235",
				SizeBytes:   truenasTiB(16),
				Temperature: 34,
				TemperatureAggregate: DiskTemperatureAggregate{
					WindowDays: 7,
					MinCelsius: 30.0,
					AvgCelsius: 33.3,
					MaxCelsius: 38.0,
				},
				Transport:  "sata",
				Rotational: true,
			},
			{
				ID:          "disk-nvme0n1",
				Name:        "nvme0n1",
				Pool:        "fast",
				Status:      "ONLINE",
				Model:       "Samsung PM9A3",
				Serial:      "S65ANX0R123456",
				SizeBytes:   truenasTiB(4),
				Temperature: 46,
				TemperatureAggregate: DiskTemperatureAggregate{
					WindowDays: 7,
					MinCelsius: 40.0,
					AvgCelsius: 44.7,
					MaxCelsius: 50.0,
				},
				Transport:  "nvme",
				Rotational: false,
			},
			{
				ID:          "disk-nvme1n1",
				Name:        "nvme1n1",
				Pool:        "fast",
				Status:      "ONLINE",
				Model:       "Samsung PM9A3",
				Serial:      "S65ANX0R123457",
				SizeBytes:   truenasTiB(4),
				Temperature: 45,
				TemperatureAggregate: DiskTemperatureAggregate{
					WindowDays: 7,
					MinCelsius: 39.0,
					AvgCelsius: 43.8,
					MaxCelsius: 49.0,
				},
				Transport:  "nvme",
				Rotational: false,
			},
			{
				ID:          "disk-sdc",
				Name:        "sdc",
				Pool:        "archive",
				Status:      "DEGRADED",
				Model:       "WDC Ultrastar DC HC550",
				Serial:      "WD-WX12A3456",
				SizeBytes:   truenasTiB(24),
				Temperature: 63,
				TemperatureAggregate: DiskTemperatureAggregate{
					WindowDays: 7,
					MinCelsius: 52.0,
					AvgCelsius: 58.9,
					MaxCelsius: 66.0,
				},
				Transport:  "sas",
				Rotational: true,
			},
			{
				ID:          "disk-sdd",
				Name:        "sdd",
				Pool:        "archive",
				Status:      "ONLINE",
				Model:       "WDC Ultrastar DC HC560",
				Serial:      "WD-WX12A3457",
				SizeBytes:   truenasTiB(24),
				Temperature: 41,
				TemperatureAggregate: DiskTemperatureAggregate{
					WindowDays: 7,
					MinCelsius: 35.0,
					AvgCelsius: 39.4,
					MaxCelsius: 44.0,
				},
				Transport:  "sas",
				Rotational: true,
			},
			{
				ID:          "disk-sde",
				Name:        "sde",
				Pool:        "vault",
				Status:      "ONLINE",
				Model:       "Micron 7450 Pro",
				Serial:      "MTFDKBA3T8QFM-1",
				SizeBytes:   truenasTiB(6),
				Temperature: 38,
				TemperatureAggregate: DiskTemperatureAggregate{
					WindowDays: 7,
					MinCelsius: 33.0,
					AvgCelsius: 36.5,
					MaxCelsius: 41.0,
				},
				Transport:  "sas",
				Rotational: false,
			},
			{
				ID:          "disk-sdf",
				Name:        "sdf",
				Pool:        "vault",
				Status:      "ONLINE",
				Model:       "Micron 7450 Pro",
				Serial:      "MTFDKBA3T8QFM-2",
				SizeBytes:   truenasTiB(6),
				Temperature: 37,
				TemperatureAggregate: DiskTemperatureAggregate{
					WindowDays: 7,
					MinCelsius: 32.0,
					AvgCelsius: 35.9,
					MaxCelsius: 40.0,
				},
				Transport:  "sas",
				Rotational: false,
			},
		},
		Alerts: []Alert{
			{
				ID:       "alert-archive-degraded",
				Level:    "WARNING",
				Message:  "Pool archive is DEGRADED: one member of archive mirror-0 is reporting checksum and SMART faults.",
				Source:   "VolumeStatus",
				Datetime: time.Date(2026, 3, 31, 8, 12, 0, 0, time.UTC),
			},
			{
				ID:       "alert-smart-sdc",
				Level:    "WARNING",
				Message:  "Device /dev/sdc has SMART test failures.",
				Source:   "SMART",
				Datetime: time.Date(2026, 3, 30, 21, 45, 0, 0, time.UTC),
			},
			{
				ID:        "alert-replication-success",
				Level:     "INFO",
				Message:   "Replication task replicate-tank-apps completed successfully to vault/compliance.",
				Source:    "Replication",
				Dismissed: true,
				Datetime:  time.Date(2026, 3, 31, 8, 25, 0, 0, time.UTC),
			},
		},
		Apps: []App{
			{
				ID:                    "nextcloud",
				Name:                  "Nextcloud",
				State:                 "RUNNING",
				Version:               "1.0.3",
				HumanVersion:          "29.0.7",
				UpgradeAvailable:      true,
				ImageUpdatesAvailable: true,
				Notes:                 "Team cloud and file sync",
				ContainerCount:        2,
				UsedHostIPs:           []string{"0.0.0.0"},
				UsedPorts: []AppPort{
					{
						ContainerPort: 443,
						Protocol:      "tcp",
						HostPorts: []AppHostPort{
							{HostPort: 30443, HostIP: "0.0.0.0"},
						},
					},
				},
				Containers: []AppContainer{
					{
						ID:          "nextcloud-web-1",
						ServiceName: "nextcloud",
						Image:       "docker.io/library/nextcloud:29.0.7",
						State:       "running",
						PortConfig: []AppPort{
							{
								ContainerPort: 443,
								Protocol:      "tcp",
								HostPorts: []AppHostPort{
									{HostPort: 30443, HostIP: "0.0.0.0"},
								},
							},
						},
						VolumeMounts: []AppVolume{
							{
								Source:      "/mnt/tank/apps/nextcloud",
								Destination: "/var/www/html",
								Mode:        "rw",
								Type:        "bind",
							},
						},
					},
					{
						ID:          "nextcloud-redis-1",
						ServiceName: "redis",
						Image:       "docker.io/library/redis:7.2",
						State:       "running",
						VolumeMounts: []AppVolume{
							{
								Source:      "ix-nextcloud-redis",
								Destination: "/data",
								Mode:        "rw",
								Type:        "volume",
							},
						},
					},
				},
				Volumes: []AppVolume{
					{
						Source:      "/mnt/tank/apps/nextcloud",
						Destination: "/var/www/html",
						Mode:        "rw",
						Type:        "bind",
					},
					{
						Source:      "ix-nextcloud-redis",
						Destination: "/data",
						Mode:        "rw",
						Type:        "volume",
					},
				},
				Images: []string{
					"docker.io/library/nextcloud:29.0.7",
					"docker.io/library/redis:7.2",
				},
				Networks: []AppNetwork{
					{
						ID:   "ix-nextcloud-default",
						Name: "ix-nextcloud_default",
						Labels: map[string]string{
							"com.docker.compose.project": "nextcloud",
						},
					},
				},
				Stats: &AppStats{
					CPUPercent:      18,
					MemoryBytes:     truenasMiB(768),
					NetInRate:       2_100_000,
					NetOutRate:      1_250_000,
					BlockReadBytes:  15_000_000,
					BlockWriteBytes: 9_000_000,
					DiskReadRate:    320_000,
					DiskWriteRate:   180_000,
					IntervalSeconds: 2,
					CollectedAt:     collectedAt,
					Interfaces: []AppInterfaceStats{
						{Name: "eth0", RxBytesPS: 2_100_000, TxBytesPS: 1_250_000},
					},
				},
			},
			{
				ID:                    "immich",
				Name:                  "Immich",
				State:                 "RUNNING",
				Version:               "1.126.1",
				HumanVersion:          "1.126.1",
				UpgradeAvailable:      false,
				ImageUpdatesAvailable: false,
				Notes:                 "Shared photo archive and mobile uploads",
				ContainerCount:        3,
				UsedHostIPs:           []string{"0.0.0.0"},
				UsedPorts: []AppPort{
					{
						ContainerPort: 2283,
						Protocol:      "tcp",
						HostPorts: []AppHostPort{
							{HostPort: 32283, HostIP: "0.0.0.0"},
						},
					},
				},
				Containers: []AppContainer{
					{
						ID:          "immich-server-1",
						ServiceName: "immich-server",
						Image:       "ghcr.io/immich-app/immich-server:v1.126.1",
						State:       "running",
						PortConfig: []AppPort{
							{
								ContainerPort: 2283,
								Protocol:      "tcp",
								HostPorts: []AppHostPort{
									{HostPort: 32283, HostIP: "0.0.0.0"},
								},
							},
						},
						VolumeMounts: []AppVolume{
							{
								Source:      "/mnt/tank/photos",
								Destination: "/usr/src/app/upload",
								Mode:        "rw",
								Type:        "bind",
							},
						},
					},
					{
						ID:          "immich-postgres-1",
						ServiceName: "database",
						Image:       "docker.io/tensorchord/pgvecto-rs:pg16-v0.3.0",
						State:       "running",
					},
					{
						ID:          "immich-redis-1",
						ServiceName: "redis",
						Image:       "docker.io/library/redis:7.2",
						State:       "running",
					},
				},
				Volumes: []AppVolume{
					{
						Source:      "/mnt/tank/photos",
						Destination: "/usr/src/app/upload",
						Mode:        "rw",
						Type:        "bind",
					},
					{
						Source:      "/mnt/fast/analytics/immich-cache",
						Destination: "/cache",
						Mode:        "rw",
						Type:        "bind",
					},
				},
				Images: []string{
					"ghcr.io/immich-app/immich-server:v1.126.1",
					"docker.io/tensorchord/pgvecto-rs:pg16-v0.3.0",
					"docker.io/library/redis:7.2",
				},
				Networks: []AppNetwork{
					{
						ID:   "ix-immich-default",
						Name: "ix-immich_default",
						Labels: map[string]string{
							"com.docker.compose.project": "immich",
						},
					},
				},
				Stats: &AppStats{
					CPUPercent:      11.3,
					MemoryBytes:     truenasMiB(1792),
					NetInRate:       4_400_000,
					NetOutRate:      3_100_000,
					BlockReadBytes:  24_000_000,
					BlockWriteBytes: 18_000_000,
					DiskReadRate:    620_000,
					DiskWriteRate:   430_000,
					IntervalSeconds: 2,
					CollectedAt:     collectedAt,
					Interfaces: []AppInterfaceStats{
						{Name: "eth0", RxBytesPS: 4_400_000, TxBytesPS: 3_100_000},
					},
				},
			},
			{
				ID:                    "paperless-ngx",
				Name:                  "Paperless-ngx",
				State:                 "RUNNING",
				Version:               "2.14.7",
				HumanVersion:          "2.14.7",
				CustomApp:             true,
				UpgradeAvailable:      false,
				ImageUpdatesAvailable: false,
				Notes:                 "Document OCR and workflow inbox",
				ContainerCount:        2,
				UsedHostIPs:           []string{"0.0.0.0"},
				UsedPorts: []AppPort{
					{
						ContainerPort: 8000,
						Protocol:      "tcp",
						HostPorts: []AppHostPort{
							{HostPort: 30080, HostIP: "0.0.0.0"},
						},
					},
				},
				Containers: []AppContainer{
					{
						ID:          "paperless-web-1",
						ServiceName: "webserver",
						Image:       "ghcr.io/paperless-ngx/paperless-ngx:2.14.7",
						State:       "running",
						PortConfig: []AppPort{
							{
								ContainerPort: 8000,
								Protocol:      "tcp",
								HostPorts: []AppHostPort{
									{HostPort: 30080, HostIP: "0.0.0.0"},
								},
							},
						},
						VolumeMounts: []AppVolume{
							{
								Source:      "/mnt/tank/projects/paperless",
								Destination: "/usr/src/paperless/data",
								Mode:        "rw",
								Type:        "bind",
							},
						},
					},
					{
						ID:          "paperless-redis-1",
						ServiceName: "broker",
						Image:       "docker.io/library/redis:7.2",
						State:       "running",
					},
				},
				Volumes: []AppVolume{
					{
						Source:      "/mnt/tank/projects/paperless",
						Destination: "/usr/src/paperless/data",
						Mode:        "rw",
						Type:        "bind",
					},
				},
				Images: []string{
					"ghcr.io/paperless-ngx/paperless-ngx:2.14.7",
					"docker.io/library/redis:7.2",
				},
				Networks: []AppNetwork{
					{
						ID:   "ix-paperless-default",
						Name: "ix-paperless_default",
						Labels: map[string]string{
							"com.docker.compose.project": "paperless",
						},
					},
				},
				Stats: &AppStats{
					CPUPercent:      7.6,
					MemoryBytes:     truenasMiB(768),
					NetInRate:       1_100_000,
					NetOutRate:      820_000,
					BlockReadBytes:  8_000_000,
					BlockWriteBytes: 6_000_000,
					DiskReadRate:    210_000,
					DiskWriteRate:   120_000,
					IntervalSeconds: 2,
					CollectedAt:     collectedAt,
					Interfaces: []AppInterfaceStats{
						{Name: "eth0", RxBytesPS: 1_100_000, TxBytesPS: 820_000},
					},
				},
			},
			{
				ID:                    "grafana",
				Name:                  "Grafana",
				State:                 "RUNNING",
				Version:               "11.4.0",
				HumanVersion:          "11.4.0",
				CustomApp:             true,
				UpgradeAvailable:      false,
				ImageUpdatesAvailable: false,
				Notes:                 "Ops dashboards and service overviews",
				ContainerCount:        1,
				UsedHostIPs:           []string{"0.0.0.0"},
				UsedPorts: []AppPort{
					{
						ContainerPort: 3000,
						Protocol:      "tcp",
						HostPorts: []AppHostPort{
							{HostPort: 30300, HostIP: "0.0.0.0"},
						},
					},
				},
				Containers: []AppContainer{
					{
						ID:          "grafana-1",
						ServiceName: "grafana",
						Image:       "docker.io/grafana/grafana:11.4.0",
						State:       "running",
						PortConfig: []AppPort{
							{
								ContainerPort: 3000,
								Protocol:      "tcp",
								HostPorts: []AppHostPort{
									{HostPort: 30300, HostIP: "0.0.0.0"},
								},
							},
						},
					},
				},
				Volumes: []AppVolume{
					{
						Source:      "/mnt/fast/analytics/grafana",
						Destination: "/var/lib/grafana",
						Mode:        "rw",
						Type:        "bind",
					},
				},
				Images: []string{"docker.io/grafana/grafana:11.4.0"},
				Networks: []AppNetwork{
					{
						ID:   "ix-grafana-default",
						Name: "ix-grafana_default",
						Labels: map[string]string{
							"com.docker.compose.project": "grafana",
						},
					},
				},
				Stats: &AppStats{
					CPUPercent:      4.1,
					MemoryBytes:     truenasMiB(320),
					NetInRate:       620_000,
					NetOutRate:      410_000,
					BlockReadBytes:  5_000_000,
					BlockWriteBytes: 2_400_000,
					DiskReadRate:    120_000,
					DiskWriteRate:   65_000,
					IntervalSeconds: 2,
					CollectedAt:     collectedAt,
					Interfaces: []AppInterfaceStats{
						{Name: "eth0", RxBytesPS: 620_000, TxBytesPS: 410_000},
					},
				},
			},
			{
				ID:           "adguard-home",
				Name:         "AdGuard Home",
				State:        "STOPPED",
				Version:      "0.1.2",
				HumanVersion: "0.107.64",
				CustomApp:    true,
				Notes:        "Edge DNS and network filtering",
				Stats: &AppStats{
					CPUPercent:      0,
					MemoryBytes:     truenasMiB(96),
					NetInRate:       0,
					NetOutRate:      0,
					BlockReadBytes:  1_000_000,
					BlockWriteBytes: 500_000,
					DiskReadRate:    0,
					DiskWriteRate:   0,
					IntervalSeconds: 2,
					CollectedAt:     collectedAt,
				},
				Containers: []AppContainer{
					{
						ID:          "adguard-home-1",
						ServiceName: "adguard-home",
						Image:       "docker.io/adguard/adguardhome:v0.107.64",
						State:       "exited",
					},
				},
				Volumes: []AppVolume{
					{
						Source:      "/mnt/tank/apps/adguard-home",
						Destination: "/opt/adguardhome/work",
						Mode:        "rw",
						Type:        "bind",
					},
				},
				Images: []string{"docker.io/adguard/adguardhome:v0.107.64"},
			},
		},
		ZFSSnapshots: []ZFSSnapshot{
			{
				ID:         "zfs-snapshot-tank-apps-20260331-0600",
				Dataset:    "tank/apps",
				Name:       "auto-20260331-0600",
				FullName:   "tank/apps@auto-20260331-0600",
				CreatedAt:  truenasTimePtr(time.Date(2026, 3, 31, 6, 0, 0, 0, time.UTC)),
				UsedBytes:  truenasInt64Ptr(truenasGiB(18)),
				Referenced: truenasInt64Ptr(truenasGiB(4020)),
			},
			{
				ID:         "zfs-snapshot-tank-media-20260330-2200",
				Dataset:    "tank/media",
				Name:       "auto-20260330-2200",
				FullName:   "tank/media@auto-20260330-2200",
				CreatedAt:  truenasTimePtr(time.Date(2026, 3, 30, 22, 0, 0, 0, time.UTC)),
				UsedBytes:  truenasInt64Ptr(truenasGiB(42)),
				Referenced: truenasInt64Ptr(truenasTiB(6)),
			},
			{
				ID:         "zfs-snapshot-tank-photos-20260331-0145",
				Dataset:    "tank/photos",
				Name:       "auto-20260331-0145",
				FullName:   "tank/photos@auto-20260331-0145",
				CreatedAt:  truenasTimePtr(time.Date(2026, 3, 31, 1, 45, 0, 0, time.UTC)),
				UsedBytes:  truenasInt64Ptr(truenasGiB(26)),
				Referenced: truenasInt64Ptr(truenasGiB(1800)),
			},
			{
				ID:         "zfs-snapshot-fast-vm-images-20260330-0315",
				Dataset:    "fast/vm-images",
				Name:       "nightly-20260330",
				FullName:   "fast/vm-images@nightly-20260330",
				CreatedAt:  truenasTimePtr(time.Date(2026, 3, 30, 3, 15, 0, 0, time.UTC)),
				UsedBytes:  truenasInt64Ptr(truenasGiB(64)),
				Referenced: truenasInt64Ptr(truenasGiB(1200)),
			},
			{
				ID:         "zfs-snapshot-archive-backups-20260329-2300",
				Dataset:    "archive/backups",
				Name:       "daily-20260329",
				FullName:   "archive/backups@daily-20260329",
				CreatedAt:  truenasTimePtr(time.Date(2026, 3, 29, 23, 0, 0, 0, time.UTC)),
				UsedBytes:  truenasInt64Ptr(truenasGiB(96)),
				Referenced: truenasInt64Ptr(truenasTiB(24)),
			},
			{
				ID:         "zfs-snapshot-vault-compliance-20260331-0415",
				Dataset:    "vault/compliance",
				Name:       "hourly-20260331-0415",
				FullName:   "vault/compliance@hourly-20260331-0415",
				CreatedAt:  truenasTimePtr(time.Date(2026, 3, 31, 4, 15, 0, 0, time.UTC)),
				UsedBytes:  truenasInt64Ptr(truenasGiB(8)),
				Referenced: truenasInt64Ptr(truenasGiB(2500)),
			},
		},
		ReplicationTasks: []ReplicationTask{
			{
				ID:             "rep-task-tank-apps",
				Name:           "replicate-tank-apps",
				SourceDatasets: []string{"tank/apps"},
				TargetDataset:  "vault/compliance/tank_apps",
				Direction:      "PUSH",
				LastRun:        truenasTimePtr(time.Date(2026, 3, 31, 8, 20, 0, 0, time.UTC)),
				LastState:      "SUCCESS",
				LastSnapshot:   "tank/apps@auto-20260331-0600",
			},
			{
				ID:             "rep-task-tank-media",
				Name:           "replicate-tank-media",
				SourceDatasets: []string{"tank/media"},
				TargetDataset:  "archive/backups/tank_media",
				Direction:      "PUSH",
				LastRun:        truenasTimePtr(time.Date(2026, 3, 30, 23, 10, 0, 0, time.UTC)),
				LastState:      "SUCCESS",
				LastSnapshot:   "tank/media@auto-20260330-2200",
			},
			{
				ID:             "rep-task-archive-backups",
				Name:           "replicate-archive-backups",
				SourceDatasets: []string{"archive/backups"},
				TargetDataset:  "offsite/archive_backups",
				Direction:      "PUSH",
				LastRun:        truenasTimePtr(time.Date(2026, 3, 30, 4, 45, 0, 0, time.UTC)),
				LastState:      "WARNING",
				LastError:      "destination latency exceeded target window",
				LastSnapshot:   "archive/backups@daily-20260329",
			},
			{
				ID:             "rep-task-vault-compliance",
				Name:           "replicate-vault-compliance",
				SourceDatasets: []string{"vault/compliance"},
				TargetDataset:  "offsite/vault_compliance",
				Direction:      "PUSH",
				LastRun:        truenasTimePtr(time.Date(2026, 3, 31, 5, 40, 0, 0, time.UTC)),
				LastState:      "RUNNING",
				LastSnapshot:   "vault/compliance@hourly-20260331-0415",
			},
		},
	}
}

func truenasInt64Ptr(value int64) *int64 {
	return &value
}

func truenasTimePtr(value time.Time) *time.Time {
	return &value
}

func truenasMiB(value int64) int64 {
	return value * 1024 * 1024
}

func truenasGiB(value int64) int64 {
	return value * 1024 * 1024 * 1024
}

func truenasTiB(value int64) int64 {
	return value * 1024 * 1024 * 1024 * 1024
}
