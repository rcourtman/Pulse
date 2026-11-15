package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// hostDiskSeedAndDecision builds the Docker fault set's D6 shape: an agent
// host that also reports Docker, with a full mount, and the sustained check
// the alert bridge asks for. It returns that check's seed and the
// finding-decision question it would end with.
func hostDiskSeedAndDecision(t *testing.T, catalog func(hostID string) aicontracts.ActionCapabilityCatalog) (string, string) {
	t.Helper()
	now := time.Now().UTC()
	host := models.Host{
		ID: "66cd990e-host", Hostname: "docker01", MachineID: "machine-docker01", Status: "online", LastSeen: now,
		CommandsEnabled: true,
		Disks:           []models.Disk{{Mountpoint: "/", Usage: 47}, {Mountpoint: "/mnt/nvr", Device: "/dev/loop2", Usage: 92.3}},
	}
	docker := models.DockerHost{
		ID: "66cd990e-host", AgentID: "66cd990e-host", Hostname: "docker01", MachineID: "machine-docker01", Status: "online", LastSeen: now,
		Containers: []models.DockerContainer{{ID: "f41c70a12873", Name: "nextcloud", State: "running", Status: "Up 17 minutes"}},
	}
	alert := models.Alert{
		ID: "agent:66cd990e-host/disk:mnt-nvr::metric-threshold:disk", Type: "disk", Level: "warning",
		ResourceID: "agent:66cd990e-host/disk:mnt-nvr", ResourceName: "docker01 (/mnt/nvr)",
		Message: "Disk at 92.3%", Value: 92.3, Threshold: 90, StartTime: now.Add(-11 * time.Minute),
		Metadata: map[string]interface{}{"mountpoint": "/mnt/nvr", "device": "/dev/loop2"},
	}
	snapshot := models.StateSnapshot{Hosts: []models.Host{host}, DockerHosts: []models.DockerHost{docker}, ActiveAlerts: []models.Alert{alert}}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	_, hostID, found := registry.GetByReference(host.ID)
	if !found {
		t.Fatal("fixture host missing from inventory")
	}

	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "mock:model", PatrolAutonomyLevel: config.PatrolAutonomyMonitor}
	svc.provider = &mockProvider{}
	svc.SetChatService(&patrolMockChatService{executor: tools.NewPulseToolExecutor(tools.ExecutorConfig{})})
	ps := NewPatrolService(svc, &patrolTestStateProvider{state: snapshot})
	ps.SetConfig(PatrolConfig{Enabled: true, Interval: time.Hour, AnalyzeNodes: true, AnalyzeGuests: true})
	ps.SetReadState(registry)
	ps.SetUnifiedResourceProvider(unifiedresources.NewUnifiedAIAdapter(registry))
	ps.SetActionCapabilityLookup(func(_ context.Context, resourceID string) (aicontracts.ActionCapabilityCatalog, error) {
		if resourceID != hostID {
			return aicontracts.ActionCapabilityCatalog{}, nil
		}
		return catalog(hostID), nil
	})

	state := ps.currentPatrolRuntimeState()
	resolved, resolution := resolvePatrolScopeState(state, SustainedAlertPatrolScope(alert.ID, alert.ResourceID, "agent", alert.Type))
	if len(resolution.UnmatchedResourceIDs)+len(resolution.AmbiguousResourceIDs) != 0 || len(resolution.EffectiveResourceIDs) == 0 {
		t.Fatalf("scope did not resolve: %+v", resolution)
	}
	seed, _ := ps.seedFindingsAndContextState(&resolved, filterPatrolStateByScopeState(state, resolved))
	return seed, ps.patrolFindingDecisionPrompt(&resolved, state)
}

// On Docker fault set D6 the alert bridge woke Patrol about a full mount on
// docker01, whose agent has commands on. The check's seed said nothing about
// the host, because the alert names the mount rather than an inventory
// resource, and the closing question's "a reading with nothing known beyond
// the alert itself" exit closed it with no finding. The finding came only when
// the harness asked. The seed and the question now say Pulse can read on the
// host, as they do for a Proxmox guest.
func TestSustainedCheckOnAReadableHostMountSaysTheInvestigationCanRead(t *testing.T) {
	readable := func(hostID string) aicontracts.ActionCapabilityCatalog {
		return aicontracts.ActionCapabilityCatalog{Reads: &aicontracts.CommandReadHost{HostResourceID: hostID, HostName: "docker01"}}
	}
	seed, decision := hostDiskSeedAndDecision(t, readable)
	for _, want := range []string{
		`Pulse can read disk usage, files and logs on "docker01" through its Pulse agent. This check cannot read them; an investigation of a finding can.`,
		"So when you cannot tell from here what fills the disk, report a finding",
	} {
		if !strings.Contains(seed, want) {
			t.Errorf("seed lacks %q:\n%s", want, seed)
		}
	}
	if !strings.Contains(decision, `Pulse can read on "docker01" and this check cannot, so a disk reading whose cause you could not see is one to record`) {
		t.Errorf("decision question lacks the read rule:\n%s", decision)
	}
	if strings.Contains(decision, "a reading with nothing known beyond the alert itself") {
		t.Errorf("decision question still offers the alert-only exit:\n%s", decision)
	}

	// With commands off the seed says why Pulse cannot read, and the question
	// keeps its exit: there is nothing an investigation could read either.
	off := func(string) aicontracts.ActionCapabilityCatalog {
		return aicontracts.ActionCapabilityCatalog{CommandsDisabled: &aicontracts.CommandsDisabledHost{HostName: "docker01"}}
	}
	seed, decision = hostDiskSeedAndDecision(t, off)
	if !strings.Contains(seed, "Commands are not enabled on the Pulse agent on docker01.") || strings.Contains(seed, "Pulse can read") {
		t.Errorf("commands-off seed:\n%s", seed)
	}
	if !strings.Contains(decision, "a reading with nothing known beyond the alert itself") || strings.Contains(decision, "Pulse can read") {
		t.Errorf("commands-off decision question:\n%s", decision)
	}

	// A read fact that names another host (a container's host, say) is not a
	// read on this resource.
	elsewhere := func(string) aicontracts.ActionCapabilityCatalog {
		return aicontracts.ActionCapabilityCatalog{Reads: &aicontracts.CommandReadHost{HostResourceID: "agent-other", HostName: "other"}}
	}
	seed, decision = hostDiskSeedAndDecision(t, elsewhere)
	if strings.Contains(seed, "Pulse can read") || strings.Contains(decision, "Pulse can read") {
		t.Errorf("a read fact for another host was taken as this one:\nseed: %s\ndecision: %s", seed, decision)
	}
}
