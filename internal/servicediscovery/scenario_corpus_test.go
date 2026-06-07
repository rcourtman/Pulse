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
	mustContain  []string           // substrings the context pack must include
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
