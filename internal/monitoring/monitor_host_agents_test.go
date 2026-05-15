package monitoring

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func TestFindLinkedProxmoxEntity_MatchesCanonicalReadStateViews(t *testing.T) {
	monitor := &Monitor{
		state: models.NewState(),
	}

	monitor.state.UpdateNodes([]models.Node{
		{ID: "node-1", Name: "pve-a", Instance: "pve1"},
	})
	monitor.state.UpdateVMs([]models.VM{
		{ID: "vm-100", Name: "vm-a", Instance: "pve1", VMID: 100},
	})
	monitor.state.UpdateContainers([]models.Container{
		{ID: "ct-200", Name: "ct-a", Instance: "pve1", VMID: 200},
	})

	nodeID, vmID, ctID := monitor.findLinkedProxmoxEntity("pve-a")
	if nodeID != "node-1" || vmID != "" || ctID != "" {
		t.Fatalf("expected node match only, got node=%q vm=%q ct=%q", nodeID, vmID, ctID)
	}

	nodeID, vmID, ctID = monitor.findLinkedProxmoxEntity("vm-a")
	if nodeID != "" || vmID != "vm-100" || ctID != "" {
		t.Fatalf("expected vm match only, got node=%q vm=%q ct=%q", nodeID, vmID, ctID)
	}

	nodeID, vmID, ctID = monitor.findLinkedProxmoxEntity("ct-a")
	if nodeID != "" || vmID != "" || ctID != "ct-200" {
		t.Fatalf("expected container match only, got node=%q vm=%q ct=%q", nodeID, vmID, ctID)
	}
}

func TestFindLinkedProxmoxEntity_AmbiguousNodeNameReturnsNoLink(t *testing.T) {
	monitor := &Monitor{
		state: models.NewState(),
	}

	monitor.state.UpdateNodes([]models.Node{
		{ID: "node-1", Name: "pve", Instance: "pve-a"},
		{ID: "node-2", Name: "pve", Instance: "pve-b"},
	})

	nodeID, vmID, ctID := monitor.findLinkedProxmoxEntity("pve")
	if nodeID != "" || vmID != "" || ctID != "" {
		t.Fatalf("expected ambiguous node name to produce no link, got node=%q vm=%q ct=%q", nodeID, vmID, ctID)
	}
}

func TestFindLinkedProxmoxEntityWithHints_UsesEndpointIPToDisambiguateNodes(t *testing.T) {
	monitor := &Monitor{
		state: models.NewState(),
	}

	monitor.state.UpdateNodes([]models.Node{
		{ID: "node-1", Name: "pve", Instance: "pve-a", Host: "https://10.0.0.1:8006"},
		{ID: "node-2", Name: "pve", Instance: "pve-b", Host: "https://10.0.0.2:8006"},
	})

	nodeID, vmID, ctID := monitor.findLinkedProxmoxEntityWithHints("pve", "10.0.0.2", nil)
	if nodeID != "node-2" || vmID != "" || ctID != "" {
		t.Fatalf("expected endpoint IP to disambiguate node-2, got node=%q vm=%q ct=%q", nodeID, vmID, ctID)
	}
}

func TestFindLinkedProxmoxEntityWithHints_UsesExactEndpointHostnameBeforeNameFallback(t *testing.T) {
	monitor := &Monitor{
		state: models.NewState(),
	}

	monitor.state.UpdateNodes([]models.Node{
		{ID: "node-1", Name: "pve", Instance: "pve-a", Host: "https://pve-a.lab:8006"},
		{ID: "node-2", Name: "pve", Instance: "pve-b", Host: "https://pve-b.lab:8006"},
	})

	nodeID, vmID, ctID := monitor.findLinkedProxmoxEntityWithHints("pve-b.lab", "", nil)
	if nodeID != "node-2" || vmID != "" || ctID != "" {
		t.Fatalf("expected endpoint hostname to disambiguate node-2, got node=%q vm=%q ct=%q", nodeID, vmID, ctID)
	}
}

func TestMonitor_HostAgentConfigUpdatePreservesReportedCommandStateInHostState(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		hostMetadataStore: config.NewHostMetadataStore(t.TempDir(), nil),
		config:            &config.Config{},
	}
	monitor.state.UpsertHost(models.Host{
		ID:              "host-command-policy",
		Hostname:        "host-command-policy",
		CommandsEnabled: true,
	})

	desired := false
	if err := monitor.UpdateHostAgentConfig("host-command-policy", &desired); err != nil {
		t.Fatalf("UpdateHostAgentConfig: %v", err)
	}

	hosts := monitor.state.GetHosts()
	if len(hosts) != 1 {
		t.Fatalf("expected one host, got %d", len(hosts))
	}
	if !hosts[0].CommandsEnabled {
		t.Fatalf("reported CommandsEnabled should remain true until the agent applies and reports the desired policy")
	}

	cfg := monitor.GetHostAgentConfig("host-command-policy")
	if cfg.CommandsEnabled == nil || *cfg.CommandsEnabled {
		t.Fatalf("desired CommandsEnabled = %#v, want false", cfg.CommandsEnabled)
	}
	if cfg.DesiredConfig == nil {
		t.Fatal("expected desired config metadata for command-policy update")
	}
}

func TestApplyDockerReport_RecreatedContainerAgentIDKeepsTokenBinding(t *testing.T) {
	monitor := newTestMonitor(t)
	token := &config.APITokenRecord{ID: "token-recreated-container", Name: "Docker Token"}

	firstReport := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              "container-machine-id-a",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:  "docker-lxc",
			MachineID: "container-machine-id-a",
		},
		Timestamp: time.Now().UTC(),
	}

	host1, err := monitor.ApplyDockerReport(firstReport, token)
	if err != nil {
		t.Fatalf("first ApplyDockerReport failed: %v", err)
	}

	recreatedReport := firstReport
	recreatedReport.Agent.ID = "container-machine-id-b"
	recreatedReport.Host.MachineID = "container-machine-id-b"
	recreatedReport.Timestamp = firstReport.Timestamp.Add(time.Minute)

	host2, err := monitor.ApplyDockerReport(recreatedReport, token)
	if err != nil {
		t.Fatalf("recreated container report should keep the existing token binding: %v", err)
	}

	if host1.ID != host2.ID {
		t.Fatalf("expected recreated container to retain host ID %q, got %q", host1.ID, host2.ID)
	}
	if got := monitor.dockerTokenBindings[token.ID]; got != host1.ID {
		t.Fatalf("token binding = %q, want stable host ID %q", got, host1.ID)
	}
}

func TestEvaluateHostAgentsTriggersOfflineAlert(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-offline"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "offline.local",
		DisplayName:     "Offline Host",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Now().Add(-10 * time.Minute),
	})

	now := time.Now()
	for i := 0; i < 3; i++ {
		monitor.evaluateHostAgents(now.Add(time.Duration(i) * time.Second))
	}

	snapshot := monitor.state.GetSnapshot()
	statusUpdated := false
	for _, host := range snapshot.Hosts {
		if host.ID == hostID {
			statusUpdated = true
			if got := host.Status; got != "offline" {
				t.Fatalf("expected host status offline, got %q", got)
			}
		}
	}
	if !statusUpdated {
		t.Fatalf("host %q not found in state snapshot", hostID)
	}

	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || healthy {
		t.Fatalf("expected connection health false, got %v (exists=%v)", healthy, ok)
	}

	alerts := monitor.alertManager.GetActiveAlerts()
	found := false
	for _, alert := range alerts {
		if alert.Type == "host-offline" && alert.ResourceID == "agent:"+hostID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected host offline alert to remain active")
	}
}

func TestEvaluateHostAgentsClearsAlertWhenHostReturns(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-recover"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "recover.local",
		DisplayName:     "Recover Host",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Now().Add(-10 * time.Minute),
	})

	for i := 0; i < 3; i++ {
		monitor.evaluateHostAgents(time.Now().Add(time.Duration(i) * time.Second))
	}

	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "recover.local",
		DisplayName:     "Recover Host",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Now(),
	})

	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true after recovery, got %v (exists=%v)", healthy, ok)
	}

	for _, alert := range monitor.alertManager.GetActiveAlerts() {
		if alert.ID == "host-offline-"+hostID {
			t.Fatalf("offline alert still active after recovery")
		}
	}
}

func TestCleanupTrackingMapsClearsStalePVEBackupInventoryScope(t *testing.T) {
	now := time.Now()
	stale := now.Add(-25 * time.Hour)
	fresh := now.Add(-time.Hour)
	staleSubject := pveBackupTemplateSubjectKey("pve-stale", "qemu", "node1", 900)
	freshSubject := pveBackupTemplateSubjectKey("pve-fresh", "qemu", "node1", 901)

	monitor := &Monitor{
		lastPVEBackupPoll: map[string]time.Time{
			"pve-stale": stale,
			"pve-fresh": fresh,
		},
		pveBackupInventoryReady: map[string]map[string]bool{
			"pve-stale": {"qemu": true},
			"pve-fresh": {"qemu": true},
		},
		pveBackupTemplateSubjects: map[string]map[string]struct{}{
			"pve-stale": {staleSubject: {}},
			"pve-fresh": {freshSubject: {}},
		},
	}

	monitor.cleanupTrackingMaps(now)

	if _, ok := monitor.lastPVEBackupPoll["pve-stale"]; ok {
		t.Fatalf("expected stale PVE backup poll marker to be removed")
	}
	if _, ok := monitor.pveBackupInventoryReady["pve-stale"]; ok {
		t.Fatalf("expected stale PVE backup inventory readiness to be removed")
	}
	if _, ok := monitor.pveBackupTemplateSubjects["pve-stale"]; ok {
		t.Fatalf("expected stale PVE backup template subjects to be removed")
	}
	if _, ok := monitor.lastPVEBackupPoll["pve-fresh"]; !ok {
		t.Fatalf("expected fresh PVE backup poll marker to remain")
	}
	if !monitor.pveBackupInventoryReady["pve-fresh"]["qemu"] {
		t.Fatalf("expected fresh PVE backup inventory readiness to remain")
	}
	if _, ok := monitor.pveBackupTemplateSubjects["pve-fresh"][freshSubject]; !ok {
		t.Fatalf("expected fresh PVE backup template subject to remain")
	}
}

func TestApplyHostReportAllowsTokenReuseAcrossHosts(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	now := time.Now().UTC()
	baseReport := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-one",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-one",
			Hostname:  "host-one",
			Platform:  "linux",
			OSName:    "debian",
			OSVersion: "12",
		},
		Timestamp: now,
		Metrics: agentshost.Metrics{
			CPUUsagePercent: 1.0,
		},
	}

	token := &config.APITokenRecord{ID: "token-one", Name: "Token One"}

	hostOne, err := monitor.ApplyHostReport(baseReport, token)
	if err != nil {
		t.Fatalf("ApplyHostReport hostOne: %v", err)
	}
	if hostOne.ID == "" {
		t.Fatalf("expected hostOne to have an identifier")
	}

	secondReport := baseReport
	secondReport.Agent.ID = "agent-two"
	secondReport.Host.ID = "machine-two"
	secondReport.Host.Hostname = "host-two"
	secondReport.Timestamp = now.Add(30 * time.Second)

	hostTwo, err := monitor.ApplyHostReport(secondReport, token)
	if err != nil {
		t.Fatalf("ApplyHostReport hostTwo: %v", err)
	}
	if hostTwo.ID == "" {
		t.Fatalf("expected hostTwo to have an identifier")
	}
	if hostTwo.ID == hostOne.ID {
		t.Fatalf("expected different host IDs for different machines, got %q", hostTwo.ID)
	}

	snapshot := monitor.state.GetSnapshot()
	if got := len(snapshot.Hosts); got != 2 {
		t.Fatalf("expected 2 hosts in state, got %d", got)
	}
}

func TestApplyDockerReportDerivesCanonicalDockerSecurityPosture(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:               models.NewState(),
		alertManager:        alerts.NewManager(),
		config:              &config.Config{},
		rateTracker:         NewRateTracker(),
		dockerMetadataStore: config.NewDockerMetadataStore(t.TempDir(), nil),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	host, err := monitor.ApplyDockerReport(agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              "agent-secure",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname: "docker-secure-host",
			Runtime:  "docker",
			Security: &agentsdocker.HostSecurityInfo{
				AuthorizationPlugins: []string{"opa", " audit "},
			},
		},
		Timestamp: time.Now().UTC(),
	}, nil)
	if err != nil {
		t.Fatalf("ApplyDockerReport returned error: %v", err)
	}

	if host.Security == nil {
		t.Fatalf("expected security posture on docker host")
	}
	if !host.Security.MutatingCommandsBlocked {
		t.Fatalf("expected mutating commands to be blocked")
	}
	if got := host.Security.AuthorizationPlugins; len(got) != 2 || got[0] != "opa" || got[1] != "audit" {
		t.Fatalf("expected normalized authorization plugins, got %#v", got)
	}
	if !strings.Contains(host.Security.MutatingCommandsBlockedReason, "GO-2026-4887") {
		t.Fatalf("expected advisory reason, got %q", host.Security.MutatingCommandsBlockedReason)
	}
}

func TestHostReportMatchesKnownIdentityUsesPersistedContinuity(t *testing.T) {
	now := time.Now().UTC()
	store := config.NewHostContinuityStore(t.TempDir(), nil)
	if err := store.Upsert(config.HostContinuityEntry{
		HostID:          "host-1",
		ReportHostID:    "machine-1",
		AgentReportedID: "agent-1",
		Hostname:        "host-1.local",
		MachineID:       "machine-1",
		TokenID:         "token-1",
		LastSeen:        now,
	}); err != nil {
		t.Fatalf("Upsert continuity: %v", err)
	}

	monitor := &Monitor{
		state:               models.NewState(),
		hostContinuityStore: store,
	}

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:      "agent-1",
			Version: "6.0.0-rc.1",
		},
		Host: agentshost.HostInfo{
			ID:        "machine-1",
			MachineID: "machine-1",
			Hostname:  "host-1.local",
			Platform:  "linux",
		},
		Timestamp: now.Add(time.Minute),
	}

	if !monitor.HostReportMatchesKnownIdentity(report, &config.APITokenRecord{ID: "token-1"}) {
		t.Fatal("expected persisted continuity to count as a known host identity")
	}
}

func TestApplyHostReportReusesPersistedContinuityAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()

	newMonitor := func() *Monitor {
		monitor := &Monitor{
			state:               models.NewState(),
			alertManager:        alerts.NewManager(),
			hostTokenBindings:   make(map[string]string),
			config:              &config.Config{DataPath: dir},
			rateTracker:         NewRateTracker(),
			hostContinuityStore: config.NewHostContinuityStore(dir, nil),
		}
		t.Cleanup(func() { monitor.alertManager.Stop() })
		return monitor
	}

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-1",
			Version:         "6.0.0-rc.1",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-1",
			MachineID: "machine-1",
			Hostname:  "host-1.local",
			Platform:  "linux",
		},
		Timestamp: now,
	}
	token := &config.APITokenRecord{ID: "token-1", Name: "Token One"}

	firstMonitor := newMonitor()
	firstHost, err := firstMonitor.ApplyHostReport(report, token)
	if err != nil {
		t.Fatalf("first ApplyHostReport: %v", err)
	}

	restartedMonitor := newMonitor()
	restartReport := report
	restartReport.Timestamp = now.Add(30 * time.Second)
	restartedHost, err := restartedMonitor.ApplyHostReport(restartReport, token)
	if err != nil {
		t.Fatalf("restarted ApplyHostReport: %v", err)
	}

	if restartedHost.ID != firstHost.ID {
		t.Fatalf("expected persisted continuity to preserve host ID %q, got %q", firstHost.ID, restartedHost.ID)
	}
}

func TestApplyHostReportDisambiguatesCollidingIdentifiersAcrossTokens(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	now := time.Now().UTC()
	baseReport := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-one",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "colliding-machine-id",
			Hostname:  "nas-one",
			Platform:  "linux",
			OSName:    "synology",
			OSVersion: "7.0",
		},
		Timestamp: now,
		Metrics: agentshost.Metrics{
			CPUUsagePercent: 1.0,
		},
	}

	hostOne, err := monitor.ApplyHostReport(baseReport, &config.APITokenRecord{ID: "token-one"})
	if err != nil {
		t.Fatalf("ApplyHostReport hostOne: %v", err)
	}
	if hostOne.ID == "" {
		t.Fatalf("expected hostOne to have an identifier")
	}

	secondReport := baseReport
	secondReport.Agent.ID = "agent-two"
	secondReport.Host.Hostname = "nas-two"
	secondReport.Timestamp = now.Add(30 * time.Second)

	hostTwo, err := monitor.ApplyHostReport(secondReport, &config.APITokenRecord{ID: "token-two"})
	if err != nil {
		t.Fatalf("ApplyHostReport hostTwo: %v", err)
	}
	if hostTwo.ID == "" {
		t.Fatalf("expected hostTwo to have an identifier")
	}
	if hostTwo.ID == hostOne.ID {
		t.Fatalf("expected disambiguated host IDs, got %q", hostTwo.ID)
	}

	hostTwoRepeat, err := monitor.ApplyHostReport(secondReport, &config.APITokenRecord{ID: "token-two"})
	if err != nil {
		t.Fatalf("ApplyHostReport hostTwo repeat: %v", err)
	}
	if hostTwoRepeat.ID != hostTwo.ID {
		t.Fatalf("expected stable host ID for repeated reports, got %q want %q", hostTwoRepeat.ID, hostTwo.ID)
	}

	// Removing the first host should not cause the second host to change identity.
	if _, err := monitor.RemoveHostAgent(hostOne.ID); err != nil {
		t.Fatalf("RemoveHostAgent hostOne: %v", err)
	}

	hostTwoAfterRemoval, err := monitor.ApplyHostReport(secondReport, &config.APITokenRecord{ID: "token-two"})
	if err != nil {
		t.Fatalf("ApplyHostReport hostTwo after removal: %v", err)
	}
	if hostTwoAfterRemoval.ID != hostTwo.ID {
		t.Fatalf("expected stable host ID after removal, got %q want %q", hostTwoAfterRemoval.ID, hostTwo.ID)
	}

	snapshot := monitor.state.GetSnapshot()
	if got := len(snapshot.Hosts); got != 1 {
		t.Fatalf("expected 1 host in state after removal, got %d", got)
	}
}

func TestRemoveHostAgentUnbindsToken(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-to-remove"
	tokenID := "token-remove"
	monitor.state.UpsertHost(models.Host{
		ID:       hostID,
		Hostname: "remove.me",
		TokenID:  tokenID,
	})
	monitor.hostTokenBindings[tokenID+":remove.me"] = hostID
	monitor.hostTokenBindings[tokenID] = hostID

	if _, err := monitor.RemoveHostAgent(hostID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}

	if _, exists := monitor.hostTokenBindings[tokenID+":remove.me"]; exists {
		t.Fatalf("expected token binding to be cleared after host removal")
	}
	if _, exists := monitor.hostTokenBindings[tokenID]; exists {
		t.Fatalf("expected legacy token binding to be cleared after host removal")
	}
}

func TestRemoveHostAgent_PreservesLinkedGuestIdentityInRemovedState(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-linked-guest"
	monitor.state.UpsertHost(models.Host{
		ID:                hostID,
		Hostname:          "guest-host.local",
		DisplayName:       "guest-host",
		LinkedVMID:        "101",
		LinkedContainerID: "102",
	})

	if _, err := monitor.RemoveHostAgent(hostID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}

	removedHosts := monitor.state.GetRemovedHostAgents()
	if len(removedHosts) != 1 {
		t.Fatalf("expected one removed host entry, got %d", len(removedHosts))
	}
	if removedHosts[0].LinkedVMID != "101" {
		t.Fatalf("expected linked VM id to persist, got %q", removedHosts[0].LinkedVMID)
	}
	if removedHosts[0].LinkedContainerID != "102" {
		t.Fatalf(
			"expected linked container id to persist, got %q",
			removedHosts[0].LinkedContainerID,
		)
	}
}

func TestRemoveHostAgent_KeepsSharedTokenUsedByDockerRuntime(t *testing.T) {
	t.Helper()

	tokenID := "shared-token"
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config: &config.Config{
			APITokens: []config.APITokenRecord{
				{ID: tokenID, Name: "Shared Token"},
			},
		},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-shared"
	monitor.state.UpsertHost(models.Host{
		ID:       hostID,
		Hostname: "shared-host.local",
		TokenID:  tokenID,
	})
	monitor.state.UpsertDockerHost(models.DockerHost{
		ID:       "docker-shared",
		Hostname: "docker-shared.local",
		TokenID:  tokenID,
		Status:   "online",
	})
	monitor.hostTokenBindings[tokenID+":shared-host.local"] = hostID

	if _, err := monitor.RemoveHostAgent(hostID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}

	if got := len(monitor.config.APITokens); got != 1 {
		t.Fatalf("expected shared API token to remain, got %d tokens", got)
	}
	if monitor.config.APITokens[0].ID != tokenID {
		t.Fatalf("expected shared token %q to remain, got %q", tokenID, monitor.config.APITokens[0].ID)
	}
}

func TestApplyHostReport_PreservesPreviousTokenMetadata(t *testing.T) {
	t.Helper()

	lastUsed := time.Now().UTC().Add(-5 * time.Minute)
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	monitor.state.UpsertHost(models.Host{
		ID:              "host-prev",
		Hostname:        "preserve.local",
		TokenID:         "token-prev",
		TokenName:       "Previous Token",
		TokenHint:       "prev_1234",
		TokenLastUsedAt: &lastUsed,
	})

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-prev",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:       "host-prev",
			Hostname: "preserve.local",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}

	if host.TokenID != "token-prev" || host.TokenName != "Previous Token" || host.TokenHint != "prev_1234" {
		t.Fatalf("expected previous token metadata to be preserved, got id=%q name=%q hint=%q", host.TokenID, host.TokenName, host.TokenHint)
	}
	if host.TokenLastUsedAt == nil || !host.TokenLastUsedAt.Equal(lastUsed) {
		t.Fatalf("expected TokenLastUsedAt %v, got %v", lastUsed, host.TokenLastUsedAt)
	}
}

func TestApplyHostReportStoresUnraidTopology(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-tower",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-tower",
			Hostname:  "tower",
			MachineID: "machine-tower",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Unraid: &agentshost.UnraidStorage{
			ArrayStarted: true,
			ArrayState:   "STARTED",
			SyncAction:   "check",
			SyncProgress: 55,
			Disks: []agentshost.UnraidDisk{
				{Name: "parity", Device: "/dev/sdb", Role: "parity", Status: "online", RawStatus: "DISK_OK", Serial: "SERIAL-PARITY"},
				{
					Name:        "disk1",
					Device:      "/dev/sdc",
					Role:        "data",
					Status:      "online",
					RawStatus:   "DISK_OK",
					Model:       "WDC WD60EFRX",
					Serial:      "SERIAL-DATA",
					Filesystem:  "xfs",
					Transport:   "sata",
					SizeBytes:   6_000_000_000_000,
					UsedBytes:   4_000,
					FreeBytes:   2_000,
					Temperature: 31,
					SpunDown:    true,
					ReadCount:   11,
					WriteCount:  12,
					ErrorCount:  16,
					Slot:        1,
				},
			},
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}

	if host.Unraid == nil {
		t.Fatal("expected unraid topology on host")
	}
	if !host.Unraid.ArrayStarted || host.Unraid.SyncAction != "check" {
		t.Fatalf("unexpected unraid summary %+v", host.Unraid)
	}
	if len(host.Unraid.Disks) != 2 || host.Unraid.Disks[0].Role != "parity" {
		t.Fatalf("unexpected unraid disks %+v", host.Unraid.Disks)
	}
	disk := host.Unraid.Disks[1]
	if disk.Model != "WDC WD60EFRX" || disk.Transport != "sata" || disk.SizeBytes != 6_000_000_000_000 {
		t.Fatalf("expected enriched unraid disk metadata, got %+v", disk)
	}
	if disk.UsedBytes != 4_000 || disk.FreeBytes != 2_000 || disk.Temperature != 31 || !disk.SpunDown {
		t.Fatalf("expected native unraid capacity and state fields, got %+v", disk)
	}
	if disk.ReadCount != 11 || disk.WriteCount != 12 || disk.ErrorCount != 16 {
		t.Fatalf("expected native unraid counters, got %+v", disk)
	}
}

func TestApplyHostReportNormalizesLegacyUnraidRawStatuses(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-tower-legacy",
			Version:         "5.1.27",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-tower-legacy",
			Hostname:  "tower",
			MachineID: "machine-tower-legacy",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Unraid: &agentshost.UnraidStorage{
			ArrayStarted: true,
			ArrayState:   "STARTED",
			SyncAction:   "check",
			SyncProgress: 55,
			NumDisabled:  1,
			NumInvalid:   1,
			Disks: []agentshost.UnraidDisk{
				{Name: "parity", Device: "/dev/sdb", Role: "parity", RawStatus: "DISK_OK", Serial: "SERIAL-PARITY"},
				{Name: "disk1", Device: "/dev/sdc", Role: "data", RawStatus: "DISK_OK", Serial: "SERIAL-DATA"},
			},
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}

	if host.Unraid == nil {
		t.Fatal("expected unraid topology on host")
	}
	for _, disk := range host.Unraid.Disks {
		if disk.Status != "online" {
			t.Fatalf("expected normalized online status from rawStatus, got %+v", host.Unraid.Disks)
		}
	}

	assessment := storagehealth.AssessUnraidStorage(*host.Unraid)
	if assessment.Level != storagehealth.RiskWarning {
		t.Fatalf("assessment level = %q, want %q", assessment.Level, storagehealth.RiskWarning)
	}
	for _, reason := range assessment.Reasons {
		if reason.Code == "unraid_disabled_disks" || reason.Code == "unraid_invalid_disks" {
			t.Fatalf("unexpected aggregate-count reason after raw-status normalization: %+v", assessment.Reasons)
		}
	}
}

func TestApplyHostReportFiltersLegacyUnraidEmptySlots(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-tower-empty-slots",
			Version:         "5.1.27",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-tower-empty-slots",
			Hostname:  "tower",
			MachineID: "machine-tower-empty-slots",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Unraid: &agentshost.UnraidStorage{
			ArrayStarted: true,
			ArrayState:   "STARTED",
			NumDisabled:  2,
			NumInvalid:   2,
			Disks: []agentshost.UnraidDisk{
				{Name: "parity", Role: "parity", RawStatus: "DISK_NP_DSBL"},
				{Name: "md1p1", Device: "/dev/sde", RawStatus: "DISK_OK", SizeBytes: 5860522532},
				{RawStatus: "DISK_NP", Slot: 5},
				{RawStatus: "DISK_NP_DSBL", Slot: 29},
			},
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	if host.Unraid == nil {
		t.Fatal("expected unraid topology on host")
	}
	if len(host.Unraid.Disks) != 1 {
		t.Fatalf("unraid disk count = %d, want only assigned disks: %+v", len(host.Unraid.Disks), host.Unraid.Disks)
	}
	if got := host.Unraid.Disks[0]; got.Device != "/dev/sde" || got.Status != "online" {
		t.Fatalf("unexpected assigned disk: %+v", got)
	}
}

func TestApplyHostReportFiltersVendorManagedSystemRAIDArrays(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	now := time.Now().UTC()
	testCases := []struct {
		name        string
		hostInfo    agentshost.HostInfo
		reportRAID  []agentshost.RAIDArray
		wantDevices []string
	}{
		{
			name: "synology suppresses md0 and md1",
			hostInfo: agentshost.HostInfo{
				ID:       "machine-synology",
				Hostname: "synology.local",
				Platform: "linux",
				OSName:   "Synology DSM",
			},
			reportRAID: []agentshost.RAIDArray{
				{Device: "/dev/md0", Level: "raid1", State: "degraded", FailedDevices: 1},
				{Device: "/dev/md1", Level: "raid1", State: "resyncing", RebuildPercent: 42},
				{Device: "/dev/md2", Level: "raid5", State: "clean"},
			},
			wantDevices: []string{"/dev/md2"},
		},
		{
			name: "qnap suppresses md9 and md13",
			hostInfo: agentshost.HostInfo{
				ID:       "machine-qnap",
				Hostname: "qnap.local",
				Platform: "linux",
				OSName:   "QNAP QTS",
			},
			reportRAID: []agentshost.RAIDArray{
				{Device: "/dev/md9", Level: "raid1", State: "degraded", FailedDevices: 1},
				{Device: "/dev/md13", Level: "raid1", State: "clean"},
				{Device: "/dev/md2", Level: "raid5", State: "clean"},
			},
			wantDevices: []string{"/dev/md2"},
		},
		{
			name: "generic hosts keep md0",
			hostInfo: agentshost.HostInfo{
				ID:       "machine-generic",
				Hostname: "generic.local",
				Platform: "linux",
				OSName:   "Ubuntu",
			},
			reportRAID: []agentshost.RAIDArray{
				{Device: "/dev/md0", Level: "raid1", State: "clean"},
				{Device: "/dev/md2", Level: "raid5", State: "clean"},
			},
			wantDevices: []string{"/dev/md0", "/dev/md2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			report := agentshost.Report{
				Agent: agentshost.AgentInfo{
					ID:              "agent-" + tc.hostInfo.ID,
					Version:         "1.0.0",
					IntervalSeconds: 30,
				},
				Host:      tc.hostInfo,
				Timestamp: now,
				Metrics: agentshost.Metrics{
					CPUUsagePercent: 5.5,
				},
				RAID: tc.reportRAID,
			}

			host, err := monitor.ApplyHostReport(report, nil)
			if err != nil {
				t.Fatalf("ApplyHostReport: %v", err)
			}

			if len(host.RAID) != len(tc.wantDevices) {
				t.Fatalf("host RAID count = %d, want %d", len(host.RAID), len(tc.wantDevices))
			}
			for i, device := range tc.wantDevices {
				if host.RAID[i].Device != device {
					t.Fatalf("host RAID[%d] = %q, want %q", i, host.RAID[i].Device, device)
				}
			}

			snapshot := monitor.state.GetSnapshot()
			var stored *models.Host
			for i := range snapshot.Hosts {
				if snapshot.Hosts[i].ID == host.ID {
					stored = &snapshot.Hosts[i]
					break
				}
			}
			if stored == nil {
				t.Fatalf("stored host %q not found", host.ID)
			}
			if len(stored.RAID) != len(tc.wantDevices) {
				t.Fatalf("stored RAID count = %d, want %d", len(stored.RAID), len(tc.wantDevices))
			}
			for i, device := range tc.wantDevices {
				if stored.RAID[i].Device != device {
					t.Fatalf("stored RAID[%d] = %q, want %q", i, stored.RAID[i].Device, device)
				}
			}
		})
	}
}

func TestApplyHostReportKeepsLocalMergerFSMounts(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-mergerfs-host",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-mergerfs-host",
			Hostname:  "mergerfs-host",
			MachineID: "machine-mergerfs-host",
		},
		Disks: []agentshost.Disk{
			{
				Device:     "mergerfs",
				Mountpoint: "/mnt/storage",
				Type:       "fuse.mergerfs",
				TotalBytes: 10_000,
				UsedBytes:  4_000,
				FreeBytes:  6_000,
				Usage:      40,
			},
			{
				Device:     "sshfs",
				Mountpoint: "/mnt/remote",
				Type:       "fuse.sshfs",
				TotalBytes: 10_000,
				UsedBytes:  4_000,
				FreeBytes:  6_000,
				Usage:      40,
			},
		},
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}

	if len(host.Disks) != 1 {
		t.Fatalf("host disk count = %d, want 1 (%+v)", len(host.Disks), host.Disks)
	}
	if host.Disks[0].Type != "fuse.mergerfs" || host.Disks[0].Mountpoint != "/mnt/storage" {
		t.Fatalf("unexpected retained disk %+v", host.Disks[0])
	}

	snapshot := monitor.state.GetSnapshot()
	if len(snapshot.Hosts) != 1 {
		t.Fatalf("snapshot host count = %d, want 1", len(snapshot.Hosts))
	}
	if len(snapshot.Hosts[0].Disks) != 1 {
		t.Fatalf("stored host disk count = %d, want 1 (%+v)", len(snapshot.Hosts[0].Disks), snapshot.Hosts[0].Disks)
	}
	if snapshot.Hosts[0].Disks[0].Type != "fuse.mergerfs" || snapshot.Hosts[0].Disks[0].Mountpoint != "/mnt/storage" {
		t.Fatalf("unexpected stored retained disk %+v", snapshot.Hosts[0].Disks[0])
	}
}

func TestApplyHostReportPersistsSMARTMetricsForAgentDisks(t *testing.T) {
	t.Helper()

	storeCfg := metrics.DefaultConfig(t.TempDir())
	storeCfg.WriteBufferSize = 1
	store, err := metrics.NewStore(storeCfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
		metricsStore:      store,
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	powerOnHours := int64(1234)
	reallocated := int64(2)
	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-tower",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-tower",
			Hostname:  "tower",
			MachineID: "machine-tower",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Sensors: agentshost.Sensors{
			SMART: []agentshost.DiskSMART{
				{
					Device:      "/dev/sda",
					Model:       "IronWolf",
					Serial:      "SERIAL-TOWER-1",
					Temperature: 41,
					Attributes: &agentshost.SMARTAttributes{
						PowerOnHours:       &powerOnHours,
						ReallocatedSectors: &reallocated,
					},
				},
			},
		},
		Timestamp: time.Now().UTC(),
	}

	if _, err := monitor.ApplyHostReport(report, nil); err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	store.Flush()

	points := waitForStoredDiskMetric(t, store, "SERIAL-TOWER-1", "smart_temp")
	if len(points) == 0 {
		t.Fatal("expected SMART temperature metric for agent disk")
	}

	points = waitForStoredDiskMetric(t, store, "SERIAL-TOWER-1", "smart_power_on_hours")
	if len(points) == 0 || points[len(points)-1].Value != float64(powerOnHours) {
		t.Fatalf("expected power-on-hours metric %.0f, got %+v", float64(powerOnHours), points)
	}

	points = waitForStoredDiskMetric(t, store, "SERIAL-TOWER-1", "smart_reallocated_sectors")
	if len(points) == 0 || points[len(points)-1].Value != float64(reallocated) {
		t.Fatalf("expected reallocated-sectors metric %.0f, got %+v", float64(reallocated), points)
	}
}

func TestApplyHostReportPersistsAgentTemperatureMetric(t *testing.T) {
	t.Helper()

	storeCfg := metrics.DefaultConfig(t.TempDir())
	storeCfg.WriteBufferSize = 1
	store, err := metrics.NewStore(storeCfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
		metricsHistory:    NewMetricsHistory(10, time.Hour),
		metricsStore:      store,
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-thermal",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-thermal",
			Hostname:  "thermal-node",
			MachineID: "machine-thermal",
		},
		Metrics: agentshost.Metrics{
			CPUUsagePercent: 12.5,
			Memory:          agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Sensors: agentshost.Sensors{
			TemperatureCelsius: map[string]float64{
				"cpu_core_0":  59,
				"cpu_package": 62.5,
			},
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	store.Flush()

	storePoints := waitForStoredMetric(t, store, "agent", host.ID, "temperature")
	if got := storePoints[len(storePoints)-1].Value; got != 62.5 {
		t.Fatalf("expected persisted temperature 62.5, got %f", got)
	}

	historyPoints := monitor.GetGuestMetrics("agent:"+host.ID, time.Hour)["temperature"]
	if len(historyPoints) == 0 || historyPoints[len(historyPoints)-1].Value != 62.5 {
		t.Fatalf("expected in-memory temperature 62.5, got %+v", historyPoints)
	}
}

func TestApplyHostReportPersistsSMARTMetricsForAgentDisksWithFallbackID(t *testing.T) {
	t.Helper()

	storeCfg := metrics.DefaultConfig(t.TempDir())
	storeCfg.WriteBufferSize = 1
	store, err := metrics.NewStore(storeCfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
		metricsStore:      store,
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	mediaErrors := int64(7)
	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-tower-fallback",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-tower",
			Hostname:  "tower",
			MachineID: "machine-tower",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Sensors: agentshost.Sensors{
			SMART: []agentshost.DiskSMART{
				{
					Device:      "/dev/nvme0n1",
					Model:       "CacheDisk",
					Temperature: 39,
					Attributes: &agentshost.SMARTAttributes{
						MediaErrors: &mediaErrors,
					},
				},
			},
		},
		Timestamp: time.Now().UTC(),
	}

	if _, err := monitor.ApplyHostReport(report, nil); err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	store.Flush()

	resourceID := "machine-tower:nvme0n1"
	points := waitForStoredDiskMetric(t, store, resourceID, "smart_temp")
	if len(points) == 0 {
		t.Fatal("expected SMART temperature metric for fallback-id agent disk")
	}

	points = waitForStoredDiskMetric(t, store, resourceID, "smart_media_errors")
	if len(points) == 0 || points[len(points)-1].Value != float64(mediaErrors) {
		t.Fatalf("expected media-errors metric %.0f, got %+v", float64(mediaErrors), points)
	}
}

func TestApplyHostReportPersistsPhysicalDiskIOMetricsForAgentDisks(t *testing.T) {
	t.Helper()

	storeCfg := metrics.DefaultConfig(t.TempDir())
	storeCfg.WriteBufferSize = 1
	store, err := metrics.NewStore(storeCfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
		metricsHistory:    NewMetricsHistory(1000, 24*time.Hour),
		metricsStore:      store,
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	baseReport := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-pve2",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "host-pve2",
			Hostname:  "pve2",
			MachineID: "machine-pve2",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Sensors: agentshost.Sensors{
			SMART: []agentshost.DiskSMART{
				{
					Device:      "/dev/nvme2",
					Model:       "Samsung 980 PRO 2TB",
					Serial:      "SERIAL884006359727",
					Temperature: 46,
				},
			},
		},
		DiskIO: []agentshost.DiskIO{
			{
				Device:     "nvme2",
				ReadBytes:  1_000_000,
				WriteBytes: 2_000_000,
				ReadOps:    100,
				WriteOps:   200,
				IOTime:     10_000,
			},
		},
		Timestamp: time.Now().UTC(),
	}

	if _, err := monitor.ApplyHostReport(baseReport, nil); err != nil {
		t.Fatalf("ApplyHostReport initial: %v", err)
	}

	nextReport := baseReport
	nextReport.Timestamp = baseReport.Timestamp.Add(30 * time.Second)
	nextReport.DiskIO = []agentshost.DiskIO{
		{
			Device:     "nvme2",
			ReadBytes:  4_000_000,
			WriteBytes: 5_000_000,
			ReadOps:    250,
			WriteOps:   350,
			IOTime:     22_000,
		},
	}

	if _, err := monitor.ApplyHostReport(nextReport, nil); err != nil {
		t.Fatalf("ApplyHostReport second: %v", err)
	}
	store.Flush()

	readPoints := waitForStoredDiskMetric(t, store, "SERIAL884006359727", "diskread")
	writePoints := waitForStoredDiskMetric(t, store, "SERIAL884006359727", "diskwrite")
	busyPoints := waitForStoredDiskMetric(t, store, "SERIAL884006359727", "disk")

	if got := readPoints[len(readPoints)-1].Value; got <= 0 {
		t.Fatalf("expected persisted diskread rate > 0, got %+v", readPoints)
	}
	if got := writePoints[len(writePoints)-1].Value; got <= 0 {
		t.Fatalf("expected persisted diskwrite rate > 0, got %+v", writePoints)
	}
	if got := busyPoints[len(busyPoints)-1].Value; got <= 0 || got > 100 {
		t.Fatalf("expected persisted disk busy percent within (0,100], got %+v", busyPoints)
	}

	if got := monitor.metricsHistory.GetDiskMetrics("SERIAL884006359727", "diskread", time.Hour); len(got) == 0 {
		t.Fatal("expected in-memory diskread history for physical disk")
	}
	if got := monitor.metricsHistory.GetDiskMetrics("SERIAL884006359727", "disk", time.Hour); len(got) == 0 {
		t.Fatal("expected in-memory busy history for physical disk")
	}
}

func TestApplyHostReportSkipsMetricsAndSMARTWritesInMockMode(t *testing.T) {
	previous := mock.IsMockEnabled()
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(previous) })

	storeCfg := metrics.DefaultConfig(t.TempDir())
	storeCfg.WriteBufferSize = 1
	store, err := metrics.NewStore(storeCfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
		metricsHistory:    NewMetricsHistory(1000, 24*time.Hour),
		metricsStore:      store,
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-demo",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-demo",
			Hostname:  "demo-host",
			MachineID: "machine-demo",
		},
		Metrics: agentshost.Metrics{
			CPUUsagePercent: 44,
			Memory:          agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Sensors: agentshost.Sensors{
			SMART: []agentshost.DiskSMART{
				{
					Device:      "/dev/sda",
					Serial:      "SERIAL-DEMO-1",
					Temperature: 39,
				},
			},
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	store.Flush()

	if got := monitor.metricsHistory.GetGuestMetrics("agent:"+host.ID, "cpu", time.Hour); len(got) != 0 {
		t.Fatalf("expected mock mode to skip host-agent metrics history, got %+v", got)
	}

	now := time.Now().UTC()
	points, err := store.Query("disk", "SERIAL-DEMO-1", "smart_temp", now.Add(-time.Hour), now.Add(time.Hour), 60)
	if err != nil {
		t.Fatalf("Query smart_temp: %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("expected mock mode to skip SMART metric persistence, got %+v", points)
	}
}

func waitForStoredDiskMetric(t *testing.T, store *metrics.Store, resourceID, metric string) []metrics.MetricPoint {
	t.Helper()

	return waitForStoredMetric(t, store, "disk", resourceID, metric)
}

func waitForStoredMetric(t *testing.T, store *metrics.Store, resourceType, resourceID, metric string) []metrics.MetricPoint {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for {
		now := time.Now().UTC()
		points, err := store.Query(resourceType, resourceID, metric, now.Add(-time.Hour), now.Add(time.Hour), 60)
		if err != nil {
			t.Fatalf("Query %s: %v", metric, err)
		}
		if len(points) > 0 {
			return points
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for disk metric %s for %s", metric, resourceID)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestEvaluateHostAgentsEmptyHostsList(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	// No hosts in state - should complete without error or state changes
	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	if len(snapshot.Hosts) != 0 {
		t.Errorf("expected 0 hosts, got %d", len(snapshot.Hosts))
	}
	if len(snapshot.ConnectionHealth) != 0 {
		t.Errorf("expected 0 connection health entries, got %d", len(snapshot.ConnectionHealth))
	}
}

func TestEvaluateHostAgentsZeroIntervalUsesDefault(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-zero-interval"
	// IntervalSeconds = 0, LastSeen = now, should use default interval (60s)
	// Default window = 60s * 6 = 360s, but minimum is 60s, so window = 60s
	// With LastSeen = now, the host should be healthy
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "zero-interval.local",
		Status:          "unknown",
		IntervalSeconds: 0, // Zero interval - should use default
		LastSeen:        time.Now(),
	})

	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true for zero-interval host with recent LastSeen, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "online" {
			t.Errorf("expected host status online, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsNegativeIntervalUsesDefault(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-negative-interval"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "negative-interval.local",
		Status:          "unknown",
		IntervalSeconds: -10, // Negative interval - should use default
		LastSeen:        time.Now(),
	})

	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true for negative-interval host with recent LastSeen, got %v (exists=%v)", healthy, ok)
	}
}

func TestEvaluateHostAgentsWindowClampedToMinimum(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-min-window"
	// IntervalSeconds = 1, so window = 1s * 6 = 6s, but minimum is 60s
	// Host last seen 55s ago should still be healthy (within 60s minimum window)
	now := time.Now()
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "min-window.local",
		Status:          "unknown",
		IntervalSeconds: 1, // Very small interval
		LastSeen:        now.Add(-55 * time.Second),
	})

	monitor.evaluateHostAgents(now)

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true (window clamped to minimum 60s), got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "online" {
			t.Errorf("expected host status online, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsWindowClampedToMaximum(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-max-window"
	// IntervalSeconds = 300 (5 min), so window = 300s * 6 = 1800s (30 min)
	// But maximum is 10 min = 600s
	// Host last seen 11 minutes ago should be unhealthy (outside 10 min max window)
	now := time.Now()
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "max-window.local",
		Status:          "online",
		IntervalSeconds: 300, // 5 minute interval
		LastSeen:        now.Add(-11 * time.Minute),
	})

	monitor.evaluateHostAgents(now)

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || healthy {
		t.Fatalf("expected connection health false (window clamped to maximum 10m), got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "offline" {
			t.Errorf("expected host status offline, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsRecentLastSeenIsHealthy(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-recent"
	now := time.Now()
	// IntervalSeconds = 30, window = 30s * 6 = 180s (clamped to min 60s is not needed)
	// LastSeen = 10s ago, should be healthy
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "recent.local",
		Status:          "unknown",
		IntervalSeconds: 30,
		LastSeen:        now.Add(-10 * time.Second),
	})

	monitor.evaluateHostAgents(now)

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true for recent LastSeen, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "online" {
			t.Errorf("expected host status online, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsZeroLastSeenIsUnhealthy(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-zero-lastseen"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "zero-lastseen.local",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Time{}, // Zero time
	})

	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || healthy {
		t.Fatalf("expected connection health false for zero LastSeen, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "offline" {
			t.Errorf("expected host status offline for zero LastSeen, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsOldLastSeenIsUnhealthy(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-old-lastseen"
	now := time.Now()
	// IntervalSeconds = 30, window = 30s * 6 = 180s
	// LastSeen = 5 minutes ago, should be unhealthy
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "old-lastseen.local",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        now.Add(-5 * time.Minute),
	})

	monitor.evaluateHostAgents(now)

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || healthy {
		t.Fatalf("expected connection health false for old LastSeen, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "offline" {
			t.Errorf("expected host status offline for old LastSeen, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsNilAlertManagerOnline(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: nil, // No alert manager
		config:       &config.Config{},
	}

	hostID := "host-nil-am-online"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "nil-am-online.local",
		Status:          "unknown",
		IntervalSeconds: 30,
		LastSeen:        time.Now(),
	})

	// Should not panic with nil alertManager
	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "online" {
			t.Errorf("expected host status online, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsNilAlertManagerOffline(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: nil, // No alert manager
		config:       &config.Config{},
	}

	hostID := "host-nil-am-offline"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "nil-am-offline.local",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Time{}, // Zero time - unhealthy
	})

	// Should not panic with nil alertManager
	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || healthy {
		t.Fatalf("expected connection health false, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "offline" {
			t.Errorf("expected host status offline, got %q", host.Status)
		}
	}
}

func TestRemoveHostAgent_ClearsConnectionHealth(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-connhealth"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "connhealth.local",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Now(),
	})

	// Seed connection health for this host (as evaluateHostAgents would)
	monitor.state.SetConnectionHealth(hostConnectionPrefix+hostID, true)

	// Verify it's present before removal
	snapshot := monitor.state.GetSnapshot()
	if _, ok := snapshot.ConnectionHealth[hostConnectionPrefix+hostID]; !ok {
		t.Fatalf("expected connection health entry to exist before removal")
	}

	// Remove the host
	if _, err := monitor.RemoveHostAgent(hostID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}

	// Verify connection health entry is gone
	snapshot = monitor.state.GetSnapshot()
	if _, ok := snapshot.ConnectionHealth[hostConnectionPrefix+hostID]; ok {
		t.Fatalf("expected connection health entry to be removed after RemoveHostAgent")
	}
}

func TestRemoveHostAgent_EmptyHostID(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	// Empty hostID should return an error
	_, err := monitor.RemoveHostAgent("")
	if err == nil {
		t.Error("expected error for empty hostID")
	}
	if err != nil && err.Error() != "host id is required" {
		t.Errorf("expected 'host id is required' error, got: %v", err)
	}

	// Whitespace-only hostID should also return an error
	_, err = monitor.RemoveHostAgent("   ")
	if err == nil {
		t.Error("expected error for whitespace-only hostID")
	}
}

func TestRemoveHostAgent_NotFound(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	// Host does not exist in state - should return synthetic host without error
	host, err := monitor.RemoveHostAgent("nonexistent-host")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return a synthetic host with ID/Hostname matching the requested ID
	if host.ID != "nonexistent-host" {
		t.Errorf("expected host.ID = 'nonexistent-host', got %q", host.ID)
	}
	if host.Hostname != "nonexistent-host" {
		t.Errorf("expected host.Hostname = 'nonexistent-host', got %q", host.Hostname)
	}
}

func TestRemoveHostAgent_NoTokenBinding(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-no-binding"
	tokenID := "token-no-binding"
	monitor.state.UpsertHost(models.Host{
		ID:       hostID,
		Hostname: "no-binding.local",
		TokenID:  tokenID,
	})
	// Intentionally NOT adding to hostTokenBindings

	host, err := monitor.RemoveHostAgent(hostID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if host.ID != hostID {
		t.Errorf("expected host.ID = %q, got %q", hostID, host.ID)
	}
}

func TestRemoveHostAgent_NilAlertManager(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      nil, // No alert manager
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}

	hostID := "host-nil-am-remove"
	monitor.state.UpsertHost(models.Host{
		ID:       hostID,
		Hostname: "nil-am-remove.local",
	})

	// Should not panic with nil alertManager
	host, err := monitor.RemoveHostAgent(hostID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if host.ID != hostID {
		t.Errorf("expected host.ID = %q, got %q", hostID, host.ID)
	}
}

func TestRemoveHostAgent_BlocksFutureReportsUntilAllowed(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		removedHostAgents: make(map[string]time.Time),
		rateTracker:       NewRateTracker(),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-blocked"
	monitor.state.UpsertHost(models.Host{
		ID:       hostID,
		Hostname: "blocked.local",
	})

	if _, err := monitor.RemoveHostAgent(hostID); err != nil {
		t.Fatalf("remove host agent: %v", err)
	}

	report := agentshost.Report{
		Host: agentshost.HostInfo{
			ID:       hostID,
			Hostname: "blocked.local",
		},
		Agent:     agentshost.AgentInfo{ID: hostID},
		Timestamp: time.Now(),
	}

	if _, err := monitor.ApplyHostReport(report, nil); err == nil {
		t.Fatal("expected removed host agent report to be rejected")
	}

	if err := monitor.AllowHostAgentReenroll(hostID); err != nil {
		t.Fatalf("allow host reenroll: %v", err)
	}

	if _, err := monitor.ApplyHostReport(report, nil); err != nil {
		t.Fatalf("expected host report after allow reenroll, got %v", err)
	}
}

func TestApplyHostReport_MissingHostname(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	// Report with empty hostname should fail
	report := agentshost.Report{
		Host: agentshost.HostInfo{
			Hostname: "", // Missing hostname
			ID:       "machine-id",
		},
		Agent: agentshost.AgentInfo{
			ID:      "agent-id",
			Version: "1.0.0",
		},
		Timestamp: time.Now(),
	}

	_, err := monitor.ApplyHostReport(report, nil)
	if err == nil {
		t.Error("expected error for missing hostname")
	}
	if err != nil && err.Error() != "host report missing hostname" {
		t.Errorf("expected 'host report missing hostname' error, got: %v", err)
	}
}

func TestApplyHostReport_WhitespaceHostname(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	// Report with whitespace-only hostname should fail
	report := agentshost.Report{
		Host: agentshost.HostInfo{
			Hostname: "   ", // Whitespace only
			ID:       "machine-id",
		},
		Agent: agentshost.AgentInfo{
			ID:      "agent-id",
			Version: "1.0.0",
		},
		Timestamp: time.Now(),
	}

	_, err := monitor.ApplyHostReport(report, nil)
	if err == nil {
		t.Error("expected error for whitespace-only hostname")
	}
}

func TestApplyHostReport_NilTokenBindingsMap(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: nil, // Nil map
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	report := agentshost.Report{
		Host: agentshost.HostInfo{
			Hostname: "test-host",
			ID:       "machine-id",
		},
		Agent: agentshost.AgentInfo{
			ID:      "agent-id",
			Version: "1.0.0",
		},
		Timestamp: time.Now(),
	}

	token := &config.APITokenRecord{ID: "token-id", Name: "Test Token"}

	// Should not panic with nil hostTokenBindings - map should be initialized
	host, err := monitor.ApplyHostReport(report, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host.Hostname != "test-host" {
		t.Errorf("expected hostname 'test-host', got %q", host.Hostname)
	}
}

func TestApplyHostReport_FallbackIdentifier(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	// Report with no ID fields - should generate fallback identifier
	report := agentshost.Report{
		Host: agentshost.HostInfo{
			Hostname: "fallback-host",
			// No ID, MachineID
		},
		Agent: agentshost.AgentInfo{
			// No ID
			Version: "1.0.0",
		},
		Timestamp: time.Now(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use hostname as fallback identifier
	if host.ID == "" {
		t.Error("expected host to have an identifier")
	}
	if host.Hostname != "fallback-host" {
		t.Errorf("expected hostname 'fallback-host', got %q", host.Hostname)
	}
}
