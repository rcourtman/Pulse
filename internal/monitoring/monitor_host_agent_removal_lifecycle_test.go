package monitoring

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func newHostRemovalLifecycleMonitor(t *testing.T, dataPath string) *Monitor {
	t.Helper()
	events, err := eventlog.OpenInMemory()
	if err != nil {
		t.Fatalf("open alert event log: %v", err)
	}
	monitor := &Monitor{
		state:               models.NewState(),
		alertManager:        alerts.NewManagerWithDataDir(dataPath),
		hostTokenBindings:   make(map[string]string),
		removedHostAgents:   make(map[string]time.Time),
		rateTracker:         NewRateTracker(),
		config:              &config.Config{DataPath: dataPath},
		hostContinuityStore: config.NewHostContinuityStore(dataPath, nil),
	}
	// Production monitor construction enables the alerts-owned event log.
	// Exercise agent removal and re-enrollment with the same adjacent runtime
	// enabled so it cannot accidentally become lifecycle authority.
	monitor.alertManager.SetEventLog(events)
	t.Cleanup(func() { monitor.alertManager.Stop() })
	return monitor
}

func hostRemovalLifecycleReport(hostID, machineID, agentID, hostname, platform string, at time.Time) agentshost.Report {
	return agentshost.Report{
		Host: agentshost.HostInfo{
			ID:        hostID,
			MachineID: machineID,
			Hostname:  hostname,
			Platform:  platform,
		},
		Agent: agentshost.AgentInfo{
			ID:      agentID,
			Version: "6.1.1",
			Type:    "unified",
		},
		Timestamp: at.UTC(),
	}
}

func TestHostAgentRemovalLifecycleSameProcessAndPlatformAliases(t *testing.T) {
	identities := []struct {
		name      string
		hostID    string
		machineID string
		agentID   string
		hostname  string
		platform  string
	}{
		{
			name:      "linux systemd machine id",
			hostID:    "01234567-89ab-cdef-0123-456789abcdef",
			machineID: "01234567-89ab-cdef-0123-456789abcdef",
			agentID:   "linux-agent-state-id",
			hostname:  "pve-node.local",
			platform:  "linux",
		},
		{
			name:      "docker unified identity",
			hostID:    "docker-machine-7",
			machineID: "docker-machine-7",
			agentID:   "docker-agent-state-id",
			hostname:  "docker-node.local",
			platform:  "linux",
		},
		{
			name:      "windows machine guid",
			hostID:    "a12b34c5-d678-49ef-a012-3456789abcde",
			machineID: "A12B34C5-D678-49EF-A012-3456789ABCDE",
			agentID:   "windows-agent-state-id",
			hostname:  "win-node.corp.local",
			platform:  "windows",
		},
	}

	for _, identity := range identities {
		t.Run(identity.name, func(t *testing.T) {
			monitor := newHostRemovalLifecycleMonitor(t, t.TempDir())
			now := time.Now().UTC()
			oldToken := &config.APITokenRecord{
				ID:        "old-" + identity.agentID,
				CreatedAt: now.Add(-time.Hour),
			}
			report := hostRemovalLifecycleReport(
				identity.hostID,
				identity.machineID,
				identity.agentID,
				identity.hostname,
				identity.platform,
				now,
			)
			host, err := monitor.ApplyHostReport(report, oldToken)
			if err != nil {
				t.Fatalf("initial ApplyHostReport: %v", err)
			}
			if _, err := monitor.RemoveHostAgent(host.ID); err != nil {
				t.Fatalf("RemoveHostAgent: %v", err)
			}
			tombstones := monitor.hostContinuityStore.RemovedEntries()
			if len(tombstones) != 1 ||
				len(tombstones[0].DeniedTokenIDs) != 1 ||
				tombstones[0].DeniedTokenIDs[0] != oldToken.ID {
				t.Fatalf("removal tombstone token lineage = %+v", tombstones)
			}
			if _, err := monitor.ApplyHostReport(report, oldToken); err == nil {
				t.Fatal("pre-removal token re-enrolled a deliberately removed host")
			}
			if _, ok := monitor.MatchHostConfigContinuity(host.ID, oldToken.ID); ok {
				t.Fatal("removed host remained available through remote-config continuity")
			}

			aliasReport := report
			aliasReport.Host.ID = identity.agentID
			aliasReport.Timestamp = now.Add(2 * time.Minute)
			freshToken := &config.APITokenRecord{
				ID:        "fresh-" + identity.agentID,
				CreatedAt: now.Add(time.Minute),
			}
			reEnrolled, err := monitor.ApplyHostReport(aliasReport, freshToken)
			if err != nil {
				t.Fatalf("fresh-token alias re-enrollment: %v", err)
			}
			if reEnrolled.ID != host.ID {
				t.Fatalf("re-enrolled host ID = %q, want canonical %q", reEnrolled.ID, host.ID)
			}
			if got := monitor.hostContinuityStore.RemovedEntries(); len(got) != 0 {
				t.Fatalf("durable tombstone survived re-enrollment: %+v", got)
			}
			if continuity, ok := monitor.MatchHostConfigContinuity(host.ID, freshToken.ID); !ok || continuity.ID != host.ID {
				t.Fatalf("fresh-token remote-config continuity = (%+v, %v)", continuity, ok)
			}
		})
	}
}

func TestHostAgentRemovalLifecycleRevokesDedicatedCredentialAndRetainsDenial(t *testing.T) {
	dataPath := t.TempDir()
	monitor := newHostRemovalLifecycleMonitor(t, dataPath)
	monitor.persistence = config.NewConfigPersistence(dataPath)

	now := time.Now().UTC()
	oldToken := config.APITokenRecord{
		ID:        "dedicated-old-token",
		Name:      "Dedicated host token",
		CreatedAt: now.Add(-time.Hour),
	}
	monitor.config.APITokens = []config.APITokenRecord{oldToken}
	if err := monitor.persistence.SaveAPITokens(monitor.config.APITokens); err != nil {
		t.Fatalf("SaveAPITokens: %v", err)
	}

	report := hostRemovalLifecycleReport(
		"dedicated-machine-id",
		"dedicated-machine-id",
		"dedicated-agent-id",
		"dedicated.local",
		"linux",
		now,
	)
	host, err := monitor.ApplyHostReport(report, &oldToken)
	if err != nil {
		t.Fatalf("initial ApplyHostReport: %v", err)
	}
	if _, err := monitor.RemoveHostAgent(host.ID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}

	if len(monitor.config.APITokens) != 0 {
		t.Fatalf("dedicated credential remained in memory: %+v", monitor.config.APITokens)
	}
	reloadedTokens, err := monitor.persistence.LoadAPITokens()
	if err != nil {
		t.Fatalf("LoadAPITokens: %v", err)
	}
	if len(reloadedTokens) != 0 {
		t.Fatalf("dedicated credential remained on disk: %+v", reloadedTokens)
	}
	if _, err := monitor.ApplyHostReport(report, &oldToken); err == nil {
		t.Fatal("explicitly revoked credential bypassed the durable identity denial")
	}
}

func TestRevokeAPITokenRollsBackCompleteInventoryWhenPersistenceFails(t *testing.T) {
	now := time.Now().UTC()
	tokens := []config.APITokenRecord{
		{ID: "newest", Name: "newest", Hash: "hash-newest", CreatedAt: now, Scopes: []string{config.ScopeWildcard}},
		{ID: "target", Name: "target", Hash: "hash-target", CreatedAt: now.Add(-time.Minute), Scopes: []string{config.ScopeAgentReport}},
		{ID: "oldest", Name: "oldest", Hash: "hash-oldest", CreatedAt: now.Add(-2 * time.Minute), Scopes: []string{config.ScopeMonitoringRead}},
	}
	stateDir := filepath.Join(t.TempDir(), "state")
	persistence := config.NewConfigPersistence(stateDir)
	if err := persistence.SaveAPITokens(tokens); err != nil {
		t.Fatalf("save initial tokens: %v", err)
	}
	if err := os.RemoveAll(stateDir); err != nil {
		t.Fatalf("remove persistence directory: %v", err)
	}
	if err := os.WriteFile(stateDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create persistence blocker: %v", err)
	}

	monitor := &Monitor{
		config:      &config.Config{APITokens: append([]config.APITokenRecord(nil), tokens...)},
		persistence: persistence,
	}
	monitor.config.SortAPITokens()

	removed, err := monitor.revokeAPIToken("target")
	if err == nil {
		t.Fatal("revokeAPIToken succeeded despite persistence failure")
	}
	if removed != nil {
		t.Fatalf("failed revocation returned removed token: %#v", removed)
	}
	if len(monitor.config.APITokens) != len(tokens) {
		t.Fatalf("token count = %d, want %d: %#v", len(monitor.config.APITokens), len(tokens), monitor.config.APITokens)
	}
	for index, want := range tokens {
		if monitor.config.APITokens[index].ID != want.ID {
			t.Fatalf("token[%d].ID = %q, want %q", index, monitor.config.APITokens[index].ID, want.ID)
		}
	}
	if monitor.config.APIToken != "hash-newest" {
		t.Fatalf("legacy primary token = %q, want %q", monitor.config.APIToken, "hash-newest")
	}
}

func TestRevokeAPITokenCommitsExactReducedInventory(t *testing.T) {
	stateDir := t.TempDir()
	persistence := config.NewConfigPersistence(stateDir)
	monitor := &Monitor{
		config: &config.Config{APITokens: []config.APITokenRecord{
			{ID: "keep", Name: "keep", Hash: "hash-keep", CreatedAt: time.Now().UTC(), Scopes: []string{config.ScopeWildcard}},
			{ID: "remove", Name: "remove", Hash: "hash-remove", CreatedAt: time.Now().UTC().Add(-time.Minute), Scopes: []string{config.ScopeAgentReport}},
		}},
		persistence: persistence,
	}
	monitor.config.SortAPITokens()

	removed, err := monitor.revokeAPIToken("remove")
	if err != nil {
		t.Fatalf("revokeAPIToken: %v", err)
	}
	if removed == nil || removed.ID != "remove" {
		t.Fatalf("removed token = %#v", removed)
	}
	if len(monitor.config.APITokens) != 1 || monitor.config.APITokens[0].ID != "keep" {
		t.Fatalf("live token inventory = %#v", monitor.config.APITokens)
	}
	persisted, err := persistence.LoadAPITokens()
	if err != nil {
		t.Fatalf("LoadAPITokens: %v", err)
	}
	if len(persisted) != 1 || persisted[0].ID != "keep" {
		t.Fatalf("persisted token inventory = %#v", persisted)
	}
}

func TestHostAgentRemovalLifecycleRepublishesUnifiedReadState(t *testing.T) {
	monitor := newHostRemovalLifecycleMonitor(t, t.TempDir())
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	monitor.SetResourceStore(adapter)

	now := time.Now().UTC()
	report := hostRemovalLifecycleReport(
		"state-convergence-machine",
		"state-convergence-machine",
		"state-convergence-agent",
		"state-convergence.local",
		"linux",
		now,
	)
	host, err := monitor.ApplyHostReport(report, &config.APITokenRecord{
		ID:        "state-convergence-token",
		CreatedAt: now.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	if hosts := adapter.Hosts(); len(hosts) != 1 {
		t.Fatalf("unified read state before removal = %+v, want one host", hosts)
	}

	if _, err := monitor.RemoveHostAgent(host.ID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}
	if hosts := adapter.Hosts(); len(hosts) != 0 {
		t.Fatalf("unified read state retained removed host: %+v", hosts)
	}
}

func TestHostAgentRemovalLifecycleRemovesAssociatedDockerSurfacesAndAlerts(t *testing.T) {
	monitor := newHostRemovalLifecycleMonitor(t, t.TempDir())
	now := time.Now().UTC()
	report := hostRemovalLifecycleReport(
		"host-machine-id",
		"host-machine-id",
		"host-agent-id",
		"shared-name.local",
		"linux",
		now,
	)
	host, err := monitor.ApplyHostReport(report, &config.APITokenRecord{
		ID:        "host-token",
		CreatedAt: now.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}

	dockerHosts := []models.DockerHost{
		{
			ID:          "docker-agent-alias",
			AgentID:     report.Agent.ID,
			Hostname:    report.Host.Hostname,
			DisplayName: report.Host.Hostname,
			Status:      "offline",
			TokenID:     "docker-token-one",
		},
		{
			ID:          "docker-machine-alias",
			MachineID:   report.Host.MachineID,
			Hostname:    report.Host.Hostname,
			DisplayName: report.Host.Hostname,
			Status:      "offline",
			TokenID:     "docker-token-two",
		},
		{
			ID:          "unrelated-docker-host",
			AgentID:     "unrelated-agent",
			MachineID:   "unrelated-machine",
			Hostname:    report.Host.Hostname,
			DisplayName: report.Host.Hostname,
			Status:      "offline",
			TokenID:     "unrelated-token",
		},
	}
	for _, dockerHost := range dockerHosts {
		monitor.state.UpsertDockerHost(dockerHost)
		for range 3 {
			monitor.alertManager.HandleDockerHostOffline(dockerHost)
		}
	}
	activeResources := make(map[string]bool)
	for _, alert := range monitor.alertManager.GetActiveAlerts() {
		activeResources[alert.ResourceID] = true
	}
	for _, dockerHost := range dockerHosts {
		resourceID := "docker:" + dockerHost.ID
		if !activeResources[resourceID] {
			t.Fatalf("active alerts before host removal did not include %q", resourceID)
		}
	}

	if _, err := monitor.RemoveHostAgent(host.ID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}
	remaining := monitor.state.GetDockerHosts()
	if len(remaining) != 1 || remaining[0].ID != "unrelated-docker-host" {
		t.Fatalf("remaining Docker hosts = %+v, want only unrelated host", remaining)
	}
	activeResources = make(map[string]bool)
	for _, alert := range monitor.alertManager.GetActiveAlerts() {
		activeResources[alert.ResourceID] = true
	}
	if activeResources["docker:docker-agent-alias"] || activeResources["docker:docker-machine-alias"] {
		t.Fatalf("associated Docker alerts remained active after host removal: %+v", activeResources)
	}
	if !activeResources["docker:unrelated-docker-host"] {
		t.Fatalf("unrelated Docker alert was cleared after host removal: %+v", activeResources)
	}

	removedIDs := make(map[string]bool)
	for _, removed := range monitor.state.GetRemovedDockerHosts() {
		removedIDs[removed.ID] = true
	}
	if !removedIDs["docker-agent-alias"] || !removedIDs["docker-machine-alias"] {
		t.Fatalf("removed Docker hosts = %+v, want both associated surfaces", removedIDs)
	}
	if removedIDs["unrelated-docker-host"] {
		t.Fatal("same-hostname unrelated Docker host was removed")
	}
}

func TestHostAgentRemovalLifecycleSurvivesMonitorReconstruction(t *testing.T) {
	dataPath := t.TempDir()
	now := time.Now().UTC()
	oldToken := &config.APITokenRecord{
		ID:        "shared-old-token",
		CreatedAt: now.Add(-time.Hour),
	}
	report := hostRemovalLifecycleReport(
		"systemd-machine-id",
		"systemd-machine-id",
		"persisted-agent-id",
		"restart-node.local",
		"linux",
		now,
	)

	first := newHostRemovalLifecycleMonitor(t, dataPath)
	host, err := first.ApplyHostReport(report, oldToken)
	if err != nil {
		t.Fatalf("initial ApplyHostReport: %v", err)
	}
	if _, err := first.RemoveHostAgent(host.ID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}

	restarted, err := New(&config.Config{
		DataPath:   dataPath,
		ConfigPath: dataPath,
		MetricsDBPath: filepath.Join(
			dataPath,
			"restarted-metrics.db",
		),
	})
	if err != nil {
		t.Fatalf("New restarted monitor: %v", err)
	}
	t.Cleanup(restarted.Stop)

	aliasReport := report
	aliasReport.Host.ID = report.Agent.ID
	aliasReport.Timestamp = now.Add(2 * time.Minute)
	if _, err := restarted.ApplyHostReport(aliasReport, oldToken); err == nil {
		t.Fatal("restart lost the durable removal deny boundary")
	}
	if _, ok := restarted.MatchHostConfigContinuity(host.ID, oldToken.ID); ok {
		t.Fatal("restart exposed removed host through remote-config continuity")
	}

	wrongIdentity := aliasReport
	wrongIdentity.Host.MachineID = "cloned-machine-id"
	wrongIdentity.Host.Hostname = "different-node.local"
	wrongIdentity.Timestamp = now.Add(3 * time.Minute)
	freshToken := &config.APITokenRecord{
		ID:        "fresh-after-restart",
		CreatedAt: now.Add(time.Minute),
	}
	if _, err := restarted.ApplyHostReport(wrongIdentity, freshToken); err == nil {
		t.Fatal("fresh token cleared a tombstone for a different machine identity")
	}

	reEnrolled, err := restarted.ApplyHostReport(aliasReport, freshToken)
	if err != nil {
		t.Fatalf("fresh-token re-enrollment after restart: %v", err)
	}
	if reEnrolled.ID != host.ID {
		t.Fatalf("re-enrolled host ID = %q, want canonical %q", reEnrolled.ID, host.ID)
	}
	aliasReport.Timestamp = now.Add(4 * time.Minute)
	if _, err := restarted.ApplyHostReport(aliasReport, oldToken); err == nil {
		t.Fatal("detached old token created a duplicate after fresh-token re-enrollment")
	}
	if got := restarted.GetLiveHostsSnapshot(); len(got) != 1 || got[0].ID != host.ID {
		t.Fatalf("detached old token changed active host inventory: %+v", got)
	}
}

func TestHostAgentRemovalLifecycleKeepsOfflineRowRemovableAfterRestart(t *testing.T) {
	dataPath := t.TempDir()
	now := time.Now().UTC()
	token := &config.APITokenRecord{
		ID:        "offline-restart-token",
		CreatedAt: now.Add(-time.Hour),
	}
	report := hostRemovalLifecycleReport(
		"offline-restart-machine",
		"offline-restart-machine",
		"offline-restart-agent",
		"offline-restart.local",
		"linux",
		now,
	)

	first := newHostRemovalLifecycleMonitor(t, dataPath)
	host, err := first.ApplyHostReport(report, token)
	if err != nil {
		t.Fatalf("initial ApplyHostReport: %v", err)
	}

	// Reconstruct the monitor without another agent report. The durable row
	// must remain visible as offline so its removal action stays reachable.
	restarted := newHostRemovalLifecycleMonitor(t, dataPath)
	hosts := restarted.HostsSnapshot()
	if len(hosts) != 1 || hosts[0].ID != host.ID {
		t.Fatalf("restart host inventory = %+v, want continuity row %q", hosts, host.ID)
	}
	if hosts[0].Status != "offline" {
		t.Fatalf("restart continuity status = %q, want offline", hosts[0].Status)
	}

	for i := 0; i < 3; i++ {
		restarted.evaluateHostAgents(now.Add(10*time.Minute + time.Duration(i)*time.Second))
	}
	if alerts := restarted.alertManager.GetActiveAlerts(); len(alerts) != 1 || alerts[0].Type != "host-offline" {
		t.Fatalf("restart offline alerts = %+v, want one host-offline alert", alerts)
	}

	removed, err := restarted.RemoveHostAgent(host.ID)
	if err != nil {
		t.Fatalf("RemoveHostAgent after restart: %v", err)
	}
	if removed.ID != host.ID || removed.Hostname != host.Hostname {
		t.Fatalf("removed continuity host = %+v, want %+v", removed, host)
	}
	if hosts := restarted.HostsSnapshot(); len(hosts) != 0 {
		t.Fatalf("removed host remained visible after restart: %+v", hosts)
	}
	if alerts := restarted.alertManager.GetActiveAlerts(); len(alerts) != 0 {
		t.Fatalf("removed host alerts remained active: %+v", alerts)
	}
	if tombstones := restarted.hostContinuityStore.RemovedEntries(); len(tombstones) != 1 || tombstones[0].HostID != host.ID {
		t.Fatalf("removal tombstones = %+v, want host %q", tombstones, host.ID)
	}
}

func TestHostAgentRemovalLifecycleDoesNotPoisonDuplicateActiveIdentity(t *testing.T) {
	monitor := newHostRemovalLifecycleMonitor(t, t.TempDir())
	now := time.Now().UTC()
	firstReport := hostRemovalLifecycleReport("shared-machine", "shared-machine", "agent-one", "node-one.local", "linux", now)
	secondReport := hostRemovalLifecycleReport("shared-machine", "shared-machine", "agent-two", "node-two.local", "linux", now)
	firstToken := &config.APITokenRecord{ID: "token-one", CreatedAt: now.Add(-time.Hour)}
	secondToken := &config.APITokenRecord{ID: "token-two", CreatedAt: now.Add(-time.Hour)}

	first, err := monitor.ApplyHostReport(firstReport, firstToken)
	if err != nil {
		t.Fatalf("first ApplyHostReport: %v", err)
	}
	second, err := monitor.ApplyHostReport(secondReport, secondToken)
	if err != nil {
		t.Fatalf("second ApplyHostReport: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("duplicate active identities collapsed to %q", first.ID)
	}

	if _, err := monitor.RemoveHostAgent(first.ID); err != nil {
		t.Fatalf("RemoveHostAgent(first): %v", err)
	}
	secondReport.Timestamp = now.Add(time.Minute)
	updated, err := monitor.ApplyHostReport(secondReport, secondToken)
	if err != nil {
		t.Fatalf("unrelated duplicate host was poisoned by removal: %v", err)
	}
	if updated.ID != second.ID {
		t.Fatalf("unrelated duplicate host changed ID from %q to %q", second.ID, updated.ID)
	}
}

func TestHostAgentRemovalLifecycleDoesNotUseProxmoxDisplayNameAsIdentity(t *testing.T) {
	monitor := newHostRemovalLifecycleMonitor(t, t.TempDir())
	monitor.state.UpdateNodesForInstance("production-api", []models.Node{{
		ID:              "production-pve1",
		NodeIdentity:    "production-pve1",
		Name:            "pve1",
		DisplayName:     "Render East",
		Instance:        "production-api",
		IsClusterMember: true,
		LinkedAgentID:   "agent-machine",
	}})

	now := time.Now().UTC()
	report := hostRemovalLifecycleReport(
		"agent-machine",
		"agent-machine",
		"agent-state",
		"pve1",
		"linux",
		now,
	)
	token := &config.APITokenRecord{ID: "agent-token", CreatedAt: now.Add(-time.Hour)}
	host, err := monitor.ApplyHostReport(report, token)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	if _, err := monitor.RemoveHostAgent(host.ID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}

	nodes := monitor.state.GetSnapshot().Nodes
	if len(nodes) != 1 ||
		nodes[0].NodeIdentity != "production-pve1" ||
		nodes[0].Name != "pve1" ||
		nodes[0].DisplayName != "Render East" {
		t.Fatalf("agent removal changed provider-owned node presentation identity: %+v", nodes)
	}
}

func TestHostAgentRemovalLifecycleFailsClosedWhenTombstoneCannotPersist(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(dataPath, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write non-directory fixture: %v", err)
	}
	monitor := newHostRemovalLifecycleMonitor(t, dataPath)
	now := time.Now().UTC()
	report := hostRemovalLifecycleReport("persist-failure-host", "persist-failure-host", "agent-id", "persist.local", "linux", now)
	host, err := monitor.ApplyHostReport(report, &config.APITokenRecord{ID: "persist-token", CreatedAt: now.Add(-time.Hour)})
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}

	if _, err := monitor.RemoveHostAgent(host.ID); err == nil {
		t.Fatal("RemoveHostAgent succeeded without a durable tombstone")
	}
	if got := monitor.state.GetHosts(); len(got) != 1 || got[0].ID != host.ID {
		t.Fatalf("failed removal did not restore live host: %+v", got)
	}
}

func TestHostAgentRemovalLifecycleFailsClosedWhenJournalCannotLoad(t *testing.T) {
	dataPath := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dataPath, "host_continuity.json"),
		[]byte(`{"broken":`),
		0o600,
	); err != nil {
		t.Fatalf("write corrupt journal: %v", err)
	}
	if monitor, err := New(&config.Config{DataPath: dataPath, ConfigPath: dataPath}); err == nil {
		monitor.Stop()
		t.Fatal("monitor started without a readable host lifecycle journal")
	}
}

func TestHostAgentRemovalLifecycleOrdersConcurrentReportsBeforeDeletion(t *testing.T) {
	monitor := newHostRemovalLifecycleMonitor(t, t.TempDir())
	now := time.Now().UTC()
	report := hostRemovalLifecycleReport("race-machine", "race-machine", "race-agent", "race.local", "linux", now)
	token := &config.APITokenRecord{ID: "race-token", CreatedAt: now.Add(-time.Hour)}
	host, err := monitor.ApplyHostReport(report, token)
	if err != nil {
		t.Fatalf("initial ApplyHostReport: %v", err)
	}

	const reporters = 32
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(reporters)
	for i := 0; i < reporters; i++ {
		go func(offset int) {
			defer wg.Done()
			<-start
			concurrent := report
			concurrent.Timestamp = now.Add(time.Duration(offset+1) * time.Millisecond)
			_, _ = monitor.ApplyHostReport(concurrent, token)
		}(i)
	}
	close(start)
	if _, err := monitor.RemoveHostAgent(host.ID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}
	wg.Wait()

	if got := monitor.state.GetHosts(); len(got) != 0 {
		t.Fatalf("concurrent report resurrected removed host: %+v", got)
	}
	report.Timestamp = now.Add(time.Minute)
	if _, err := monitor.ApplyHostReport(report, token); err == nil {
		t.Fatal("old token reported after concurrent deletion completed")
	}
}
