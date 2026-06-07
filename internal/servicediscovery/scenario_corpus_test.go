package servicediscovery

import (
	"strings"
	"testing"
)

// The Discovery -> Assistant context oracle.
//
// Long-horizon goal: opening the Assistant on any workload should give it, with
// zero user re-explanation, everything it needs to act — how to reach the
// guest, where the app's config/data/logs/automations live, what is listening.
// The verifiable test is per workload: given a realistic discovered resource,
// the AI context pack (FormatForAIContext — what Chat and Patrol actually see)
// must surface the context a real troubleshooting/management question requires.
//
// This corpus grows one service-type cell at a time. Each case pins the
// must-have context for a concrete user question; a missing substring is a
// concrete gap in what the Assistant can do, to be closed in the analyzer
// (what discovery captures) or the formatter (what reaches the Assistant).
type contextScenario struct {
	name         string             // service-type cell
	userQuestion string             // the real question this context must answer
	discovery    *ResourceDiscovery // a realistic discovered workload
	mustContain  []string           // substrings the chat context pack (FormatForAIContext) must include
	// substrings the remediation pack (FormatForRemediation, what Patrol/fix
	// flows consume) must include; nil skips the remediation check for this cell.
	remediationMustContain []string
}

func contextScenarioCorpus() []contextScenario {
	return []contextScenario{
		{
			name:         "home-assistant LXC",
			userQuestion: "my blinds automation didn't fire this morning",
			discovery: &ResourceDiscovery{
				ID:             MakeResourceID(ResourceTypeSystemContainer, "delly", "101"),
				ResourceType:   ResourceTypeSystemContainer,
				ResourceID:     "101",
				TargetID:       "delly",
				Hostname:       "home-assistant",
				ServiceType:    "home-assistant",
				ServiceName:    "Home Assistant",
				ServiceVersion: "2026.5.1",
				Category:       CategoryHomeAuto,
				CLIAccess:      "pct exec 101 -- bash",
				ConfigPaths:    []string{"/config/configuration.yaml", "/config/automations.yaml"},
				DataPaths:      []string{"/config"},
				LogPaths:       []string{"/config/home-assistant.log"},
				Ports:          []PortInfo{{Port: 8123, Protocol: "tcp", Process: "python3"}},
			},
			// To act on "automation didn't fire" the Assistant needs: how to
			// reach the guest, the automations file, and the log to inspect.
			mustContain: []string{
				"pct exec 101",
				"/config/automations.yaml",
				"/config/home-assistant.log",
			},
		},
		{
			name:         "home-assistant Docker (bind mount)",
			userQuestion: "edit my blinds automation on the host",
			discovery: &ResourceDiscovery{
				ID:           MakeResourceID(ResourceTypeDocker, "nuc", "homeassistant"),
				ResourceType: ResourceTypeDocker,
				ResourceID:   "homeassistant",
				TargetID:     "nuc",
				Hostname:     "nuc",
				ServiceType:  "home-assistant",
				ServiceName:  "Home Assistant",
				Category:     CategoryHomeAuto,
				CLIAccess:    "docker exec homeassistant bash",
				ConfigPaths:  []string{"/config/automations.yaml"},
				DockerMounts: []DockerBindMount{
					{
						ContainerName: "homeassistant",
						Source:        "/opt/homeassistant/config",
						Destination:   "/config",
						Type:          "bind",
					},
				},
			},
			// The container path /config is meaningless for editing from the host
			// without its bind-mount source on the host filesystem.
			mustContain: []string{
				"docker exec homeassistant",
				"/opt/homeassistant/config -> /config",
			},
		},
		{
			name:         "postgresql LXC",
			userQuestion: "the database is slow and I may need to restart it",
			discovery: &ResourceDiscovery{
				ID:             MakeResourceID(ResourceTypeSystemContainer, "minipc", "112"),
				ResourceType:   ResourceTypeSystemContainer,
				ResourceID:     "112",
				TargetID:       "minipc",
				Hostname:       "pg-primary",
				ServiceType:    "postgresql",
				ServiceName:    "PostgreSQL",
				ServiceVersion: "16.3",
				Category:       CategoryDatabase,
				CLIAccess:      "pct exec 112 -- bash",
				ConfigPaths:    []string{"/etc/postgresql/16/main/postgresql.conf"},
				DataPaths:      []string{"/var/lib/postgresql/16/main"},
				LogPaths:       []string{"/var/log/postgresql/postgresql-16-main.log"},
				Ports:          []PortInfo{{Port: 5432, Protocol: "tcp", Process: "postgres"}},
				Facts: []DiscoveryFact{
					{Category: FactCategoryService, Key: "systemd_unit", Value: "postgresql@16-main.service", Source: "running_services", Confidence: 0.9},
					{Category: FactCategoryStorage, Key: "data_filesystem", Value: "zfs tank/postgres", Source: "disk_usage", Confidence: 0.85},
				},
			},
			// To triage slowness and restart, the Assistant needs how to reach
			// the guest, the config + log, AND how the service is managed (the
			// systemd unit) and where its data lives (the dataset) — service and
			// storage facts the context pack used to drop.
			mustContain: []string{
				"pct exec 112",
				"postgresql.conf",
				"postgresql@16-main.service",
				"tank/postgres",
			},
			remediationMustContain: []string{
				"postgresql@16-main.service",  // service control
				"postgresql.conf",             // config
				"/var/lib/postgresql/16/main", // data directory
				"tank/postgres",               // storage fact (now surfaced)
			},
		},
		{
			name:         "frigate Docker (fact-heavy)",
			userQuestion: "my driveway camera keeps going offline",
			discovery: &ResourceDiscovery{
				ID:             MakeResourceID(ResourceTypeDocker, "nvr", "frigate"),
				ResourceType:   ResourceTypeDocker,
				ResourceID:     "frigate",
				TargetID:       "nvr",
				Hostname:       "nvr",
				ServiceType:    "frigate",
				ServiceName:    "Frigate NVR",
				ServiceVersion: "0.14.1",
				Category:       CategoryNVR,
				CLIAccess:      "docker exec frigate bash",
				ConfigPaths:    []string{"/config/config.yml"},
				LogPaths:       []string{"/config/frigate.log"},
				Ports:          []PortInfo{{Port: 5000, Protocol: "tcp", Process: "frigate"}},
				// Six priority facts — more than the old cap of 5. The
				// service-control fact (how to restart the camera service) is last
				// in input order, so it must be sorted ahead of the informational
				// facts to survive the cap.
				Facts: []DiscoveryFact{
					{Category: FactCategoryVersion, Key: "frigate_version", Value: "0.14.1", Source: "config_files", Confidence: 0.95},
					{Category: FactCategoryHardware, Key: "coral_tpu", Value: "/dev/apex_0", Source: "gpu_devices", Confidence: 0.9},
					{Category: FactCategoryHardware, Key: "gpu", Value: "intel-vaapi", Source: "gpu_devices", Confidence: 0.85},
					{Category: FactCategoryDependency, Key: "mqtt_broker", Value: "mosquitto:1883", Source: "listening_ports", Confidence: 0.85},
					{Category: FactCategoryStorage, Key: "media_volume", Value: "/mnt/nvr zfs", Source: "disk_usage", Confidence: 0.8},
					{Category: FactCategoryService, Key: "restart", Value: "docker restart frigate", Source: "running_services", Confidence: 0.9},
				},
			},
			// To triage an offline camera the Assistant needs the config + log,
			// the detector (coral) it depends on, and how to restart the service.
			mustContain: []string{
				"docker exec frigate",
				"/config/config.yml",
				"coral_tpu",
				"docker restart frigate",
			},
		},
		{
			name:         "nginx reverse proxy (Docker, read-only config mount)",
			userQuestion: "my site returns 502 bad gateway",
			discovery: &ResourceDiscovery{
				ID:             MakeResourceID(ResourceTypeDocker, "web1", "nginx"),
				ResourceType:   ResourceTypeDocker,
				ResourceID:     "nginx",
				TargetID:       "web1",
				Hostname:       "web1",
				ServiceType:    "nginx",
				ServiceName:    "Nginx",
				ServiceVersion: "1.27",
				Category:       CategoryWebServer,
				CLIAccess:      "docker exec nginx sh",
				ConfigPaths:    []string{"/etc/nginx/nginx.conf", "/etc/nginx/conf.d/default.conf"},
				LogPaths:       []string{"/var/log/nginx/error.log", "/var/log/nginx/access.log"},
				Ports: []PortInfo{
					{Port: 80, Protocol: "tcp", Process: "nginx"},
					{Port: 443, Protocol: "tcp", Process: "nginx"},
				},
				DockerMounts: []DockerBindMount{
					{
						ContainerName: "nginx",
						Source:        "/srv/nginx/conf.d",
						Destination:   "/etc/nginx/conf.d",
						Type:          "bind",
						ReadOnly:      true,
					},
				},
				Facts: []DiscoveryFact{
					{Category: FactCategoryDependency, Key: "upstream", Value: "app:3000", Source: "config_files", Confidence: 0.85},
					{Category: FactCategoryService, Key: "reload", Value: "docker exec nginx nginx -s reload", Source: "running_services", Confidence: 0.9},
				},
			},
			// A 502 means the upstream is unreachable: the Assistant needs the
			// config, the error log, the upstream target, and how to reload after
			// a fix. The config mount is read-only — it must know that before
			// trying to edit (pins the read-only marker).
			mustContain: []string{
				"/etc/nginx/conf.d/default.conf",
				"/var/log/nginx/error.log",
				"app:3000",
				"/srv/nginx/conf.d -> /etc/nginx/conf.d (read-only)",
				"nginx -s reload",
			},
			remediationMustContain: []string{
				"nginx -s reload",   // service control
				"default.conf",      // config
				"/srv/nginx/conf.d", // bind-mount host source
				"app:3000",          // dependency fact (now surfaced) — the 502 upstream
			},
		},
		{
			name:         "mosquitto MQTT broker (LXC, auth)",
			userQuestion: "my smart-home devices can't connect to MQTT",
			discovery: &ResourceDiscovery{
				ID:             MakeResourceID(ResourceTypeSystemContainer, "delly", "120"),
				ResourceType:   ResourceTypeSystemContainer,
				ResourceID:     "120",
				TargetID:       "delly",
				Hostname:       "mqtt",
				ServiceType:    "mosquitto",
				ServiceName:    "Mosquitto MQTT",
				ServiceVersion: "2.0.18",
				Category:       CategoryNetwork,
				CLIAccess:      "pct exec 120 -- bash",
				ConfigPaths:    []string{"/etc/mosquitto/mosquitto.conf", "/etc/mosquitto/conf.d/auth.conf"},
				LogPaths:       []string{"/var/log/mosquitto/mosquitto.log"},
				Ports: []PortInfo{
					{Port: 1883, Protocol: "tcp", Process: "mosquitto"},
					{Port: 8883, Protocol: "tcp", Process: "mosquitto"},
				},
				Facts: []DiscoveryFact{
					{Category: FactCategorySecurity, Key: "auth", Value: "password_file set; allow_anonymous false", Source: "config_files", Confidence: 0.9},
					{Category: FactCategoryService, Key: "restart", Value: "systemctl restart mosquitto", Source: "running_services", Confidence: 0.9},
				},
			},
			// "Can't connect" is usually auth or the listener: the Assistant needs
			// the config, the listening port, the auth setup (a security fact —
			// pins security-category surfacing), and how to restart after a change.
			mustContain: []string{
				"pct exec 120",
				"/etc/mosquitto/conf.d/auth.conf",
				"1883/tcp",
				"allow_anonymous false",
				"systemctl restart mosquitto",
			},
			remediationMustContain: []string{
				"systemctl restart mosquitto", // service control
				"auth.conf",                   // config
				"allow_anonymous false",       // security fact (now surfaced) — the connect failure
			},
		},
		{
			name:         "plex media server (VM, qm guest exec)",
			userQuestion: "playback keeps buffering and transcodes are failing",
			discovery: &ResourceDiscovery{
				ID:             MakeResourceID(ResourceTypeVM, "minipc", "200"),
				ResourceType:   ResourceTypeVM,
				ResourceID:     "200",
				TargetID:       "minipc",
				Hostname:       "plex",
				ServiceType:    "plex",
				ServiceName:    "Plex Media Server",
				ServiceVersion: "1.40.2",
				Category:       CategoryMedia,
				// VMs are reached via the QEMU guest agent, not pct/docker exec —
				// the one resource type the other cells don't cover.
				CLIAccess: "qm guest exec 200 -- bash",
				ConfigPaths: []string{
					"/var/lib/plexmediaserver/Library/Application Support/Plex Media Server/Preferences.xml",
				},
				DataPaths: []string{"/mnt/media"},
				LogPaths: []string{
					"/var/lib/plexmediaserver/Library/Application Support/Plex Media Server/Logs/Plex Media Server.log",
				},
				Ports: []PortInfo{{Port: 32400, Protocol: "tcp", Process: "Plex Media Server"}},
				Facts: []DiscoveryFact{
					{Category: FactCategoryHardware, Key: "gpu", Value: "intel-quicksync (/dev/dri/renderD128)", Source: "gpu_devices", Confidence: 0.9},
					{Category: FactCategoryService, Key: "restart", Value: "systemctl restart plexmediaserver", Source: "running_services", Confidence: 0.9},
				},
			},
			// Transcode failures point at the hardware decoder and the service: the
			// Assistant needs how to reach the VM (qm guest exec), the GPU it should
			// be using, and how to restart Plex.
			mustContain: []string{
				"qm guest exec 200",
				"intel-quicksync",
				"systemctl restart plexmediaserver",
				"32400/tcp",
			},
			remediationMustContain: []string{
				"systemctl restart plexmediaserver", // service control
				"Preferences.xml",                   // config
				"intel-quicksync",                   // hardware fact (transcode decoder)
			},
		},
		{
			name:         "redis cache (Kubernetes pod)",
			userQuestion: "the app keeps losing its cache and redis restarts",
			discovery: &ResourceDiscovery{
				ID:             MakeResourceID(ResourceTypeK8s, "prod-cluster", "cache/redis-0"),
				ResourceType:   ResourceTypeK8s,
				ResourceID:     "cache/redis-0",
				TargetID:       "prod-cluster",
				Hostname:       "redis-0",
				ServiceType:    "redis",
				ServiceName:    "Redis",
				ServiceVersion: "7.2",
				Category:       CategoryCache,
				// k8s pods are reached via kubectl exec, and "restart" is a
				// rollout/delete, not systemctl — completes the resource-type matrix.
				CLIAccess:   "kubectl exec -n cache redis-0 -- sh",
				ConfigPaths: []string{"/usr/local/etc/redis/redis.conf"},
				Ports:       []PortInfo{{Port: 6379, Protocol: "tcp", Process: "redis-server"}},
				Facts: []DiscoveryFact{
					{Category: FactCategoryService, Key: "restart", Value: "kubectl rollout restart statefulset/redis -n cache", Source: "running_services", Confidence: 0.85},
					{Category: FactCategoryStorage, Key: "maxmemory", Value: "256mb (no eviction policy)", Source: "config_files", Confidence: 0.8},
				},
			},
			// Cache loss + restarts on k8s usually means OOM / eviction config: the
			// Assistant needs kubectl access, redis.conf, the rollout-restart, and
			// the memory-limit fact.
			mustContain: []string{
				"kubectl exec -n cache redis-0",
				"redis.conf",
				"kubectl rollout restart statefulset/redis",
				"6379/tcp",
			},
			remediationMustContain: []string{
				"kubectl rollout restart statefulset/redis", // service control
				"redis.conf",                 // config
				"256mb (no eviction policy)", // storage fact (memory limit)
			},
		},
	}
}

func TestContextScenarioCorpus(t *testing.T) {
	for _, sc := range contextScenarioCorpus() {
		t.Run(sc.name, func(t *testing.T) {
			pack := FormatForAIContext([]*ResourceDiscovery{sc.discovery})
			for _, want := range sc.mustContain {
				if !strings.Contains(pack, want) {
					t.Errorf("context pack for %q (question: %q) is missing %q —\nthe Assistant could not answer the question without it.\n--- context pack ---\n%s",
						sc.name, sc.userQuestion, want, pack)
				}
			}
		})
	}
}

// The remediation pack (FormatForRemediation, what Patrol/fix flows consume)
// must carry the same act-on-it context as the chat pack — including the
// dependency, security, and storage facts a fix actually needs (a 502's
// upstream, a broker's auth, a database's backing disk).
func TestRemediationScenarioCorpus(t *testing.T) {
	for _, sc := range contextScenarioCorpus() {
		if len(sc.remediationMustContain) == 0 {
			continue
		}
		t.Run(sc.name, func(t *testing.T) {
			pack := FormatForRemediation(sc.discovery)
			for _, want := range sc.remediationMustContain {
				if !strings.Contains(pack, want) {
					t.Errorf("remediation pack for %q (question: %q) is missing %q —\nthe fix flow could not act without it.\n--- remediation pack ---\n%s",
						sc.name, sc.userQuestion, want, pack)
				}
			}
		})
	}
}
