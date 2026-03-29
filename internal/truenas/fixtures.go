package truenas

import "time"

// DefaultFixtures returns a realistic fixture snapshot for contract testing.
func DefaultFixtures() FixtureSnapshot {
	return FixtureSnapshot{
		CollectedAt: time.Date(2026, 2, 8, 12, 0, 0, 0, time.UTC),
		System: SystemInfo{
			Hostname:      "truenas-main",
			Version:       "TrueNAS-SCALE-24.10.2",
			Build:         "24.10.2.1",
			UptimeSeconds: 42 * 24 * 60 * 60,
			Healthy:       true,
			MachineID:     "truenas-1-machine-id",
		},
		Pools: []Pool{
			{
				ID:         "pool-tank",
				Name:       "tank",
				Status:     "ONLINE",
				TotalBytes: 30 * 1024 * 1024 * 1024 * 1024,
				UsedBytes:  12 * 1024 * 1024 * 1024 * 1024,
				FreeBytes:  18 * 1024 * 1024 * 1024 * 1024,
			},
			{
				ID:         "pool-fast",
				Name:       "fast",
				Status:     "ONLINE",
				TotalBytes: 4 * 1024 * 1024 * 1024 * 1024,
				UsedBytes:  1400 * 1024 * 1024 * 1024,
				FreeBytes:  2696 * 1024 * 1024 * 1024,
			},
			{
				ID:         "pool-archive",
				Name:       "archive",
				Status:     "DEGRADED",
				TotalBytes: 60 * 1024 * 1024 * 1024 * 1024,
				UsedBytes:  51 * 1024 * 1024 * 1024 * 1024,
				FreeBytes:  9 * 1024 * 1024 * 1024 * 1024,
			},
		},
		Datasets: []Dataset{
			{
				ID:         "dataset-tank-apps",
				Name:       "tank/apps",
				Pool:       "tank",
				UsedBytes:  5 * 1024 * 1024 * 1024 * 1024,
				AvailBytes: 13 * 1024 * 1024 * 1024 * 1024,
				Mounted:    true,
			},
			{
				ID:         "dataset-tank-media",
				Name:       "tank/media",
				Pool:       "tank",
				UsedBytes:  7 * 1024 * 1024 * 1024 * 1024,
				AvailBytes: 11 * 1024 * 1024 * 1024 * 1024,
				Mounted:    true,
			},
			{
				ID:         "dataset-fast-vm",
				Name:       "fast/vm-images",
				Pool:       "fast",
				UsedBytes:  900 * 1024 * 1024 * 1024,
				AvailBytes: 1796 * 1024 * 1024 * 1024,
				Mounted:    true,
			},
			{
				ID:         "dataset-archive-backups",
				Name:       "archive/backups",
				Pool:       "archive",
				UsedBytes:  40 * 1024 * 1024 * 1024 * 1024,
				AvailBytes: 5 * 1024 * 1024 * 1024 * 1024,
				Mounted:    true,
			},
			{
				ID:         "dataset-archive-cold",
				Name:       "archive/cold",
				Pool:       "archive",
				UsedBytes:  11 * 1024 * 1024 * 1024 * 1024,
				AvailBytes: 4 * 1024 * 1024 * 1024 * 1024,
				Mounted:    true,
				ReadOnly:   true,
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
				SizeBytes:   16 * 1024 * 1024 * 1024 * 1024,
				Temperature: 34,
				Transport:   "sata",
				Rotational:  true,
			},
			{
				ID:          "disk-sdb",
				Name:        "sdb",
				Pool:        "tank",
				Status:      "ONLINE",
				Model:       "Seagate Exos X18",
				Serial:      "ZL0A1235",
				SizeBytes:   16 * 1024 * 1024 * 1024 * 1024,
				Temperature: 36,
				Transport:   "sata",
				Rotational:  true,
			},
			{
				ID:          "disk-nvme0n1",
				Name:        "nvme0n1",
				Pool:        "fast",
				Status:      "ONLINE",
				Model:       "Samsung PM9A3",
				Serial:      "S65ANX0R123456",
				SizeBytes:   2 * 1024 * 1024 * 1024 * 1024,
				Temperature: 48,
				Transport:   "nvme",
				Rotational:  false,
			},
			{
				ID:          "disk-sdc",
				Name:        "sdc",
				Pool:        "archive",
				Status:      "DEGRADED",
				Model:       "WDC Ultrastar DC HC550",
				Serial:      "WD-WX12A3456",
				SizeBytes:   20 * 1024 * 1024 * 1024 * 1024,
				Temperature: 63,
				Transport:   "sas",
				Rotational:  true,
			},
		},
		Alerts: []Alert{
			{
				ID:       "alert-degraded-pool",
				Level:    "WARNING",
				Message:  "Pool archive state is DEGRADED: One or more devices has been removed by the administrator.",
				Source:   "VolumeStatus",
				Datetime: time.Date(2026, 2, 6, 8, 15, 0, 0, time.UTC),
			},
			{
				ID:       "alert-smart",
				Level:    "WARNING",
				Message:  "Device /dev/sdc has SMART test failures.",
				Source:   "SMART",
				Datetime: time.Date(2026, 2, 7, 14, 30, 0, 0, time.UTC),
			},
			{
				ID:        "alert-scrub-finished",
				Level:     "INFO",
				Message:   "Scrub of pool tank finished without errors.",
				Source:    "Scrub",
				Dismissed: true,
				Datetime:  time.Date(2026, 2, 5, 3, 0, 0, 0, time.UTC),
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
			},
			{
				ID:           "adguard-home",
				Name:         "AdGuard Home",
				State:        "STOPPED",
				Version:      "0.1.2",
				HumanVersion: "0.107.64",
				CustomApp:    true,
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
	}
}
