package monitoring

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
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

func TestCollectorUninstallHostAgentTransactionRollsBackAndRetries(t *testing.T) {
	dataPath := t.TempDir()
	monitor := newHostRemovalLifecycleMonitor(t, dataPath)
	monitor.persistence = config.NewConfigPersistence(dataPath)

	now := time.Now().UTC()
	token := config.APITokenRecord{
		ID:        "collector-uninstall-token",
		Name:      "Collector uninstall token",
		Hash:      "collector-uninstall-hash",
		CreatedAt: now.Add(-time.Hour),
		Scopes:    []string{config.ScopeAgentReport, config.ScopeAgentConfigRead},
	}
	monitor.config.APITokens = []config.APITokenRecord{token}
	if err := monitor.persistence.SaveAPITokens(monitor.config.APITokens); err != nil {
		t.Fatalf("SaveAPITokens: %v", err)
	}

	report := hostRemovalLifecycleReport(
		"collector-uninstall-machine",
		"collector-uninstall-machine",
		"collector-uninstall-agent",
		"collector-uninstall.local",
		"linux",
		now,
	)
	host, err := monitor.ApplyHostReport(report, &token)
	if err != nil {
		t.Fatalf("initial ApplyHostReport: %v", err)
	}

	// Block only the credential inventory's atomic replacement. The removal
	// tombstone can still be written first, so this exercises the rollback
	// half of the durable uninstall transaction rather than a preflight error.
	credentialBlocker := filepath.Join(dataPath, "api_tokens.json.tmp")
	if err := os.Mkdir(credentialBlocker, 0o700); err != nil {
		t.Fatalf("create credential persistence blocker: %v", err)
	}
	if _, err := monitor.UninstallHostAgent(host.ID, token.ID); err == nil {
		t.Fatal("UninstallHostAgent authorized teardown without durable credential revocation")
	}
	if hosts := monitor.GetLiveHostsSnapshot(); len(hosts) != 1 || hosts[0].ID != host.ID {
		t.Fatalf("failed uninstall changed live host state: %+v", hosts)
	}
	if len(monitor.config.APITokens) != 1 || monitor.config.APITokens[0].ID != token.ID {
		t.Fatalf("failed uninstall changed live credential inventory: %+v", monitor.config.APITokens)
	}
	continuity, ok := monitor.hostContinuityStore.Get(host.ID)
	if !ok || !continuity.RemovedAt.IsZero() || continuity.TokenID != token.ID {
		t.Fatalf("failed uninstall did not restore active continuity: (%+v, %v)", continuity, ok)
	}

	if err := os.Remove(credentialBlocker); err != nil {
		t.Fatalf("remove credential persistence blocker: %v", err)
	}
	removed, err := monitor.UninstallHostAgent(host.ID, token.ID)
	if err != nil {
		t.Fatalf("retry UninstallHostAgent: %v", err)
	}
	if removed.ID != host.ID {
		t.Fatalf("removed host = %+v, want %q", removed, host.ID)
	}
	if hosts := monitor.GetLiveHostsSnapshot(); len(hosts) != 0 {
		t.Fatalf("successful uninstall retained live host: %+v", hosts)
	}
	if len(monitor.config.APITokens) != 0 {
		t.Fatalf("successful uninstall retained live credential: %+v", monitor.config.APITokens)
	}
	persistedTokens, err := monitor.persistence.LoadAPITokens()
	if err != nil {
		t.Fatalf("LoadAPITokens after retry: %v", err)
	}
	if len(persistedTokens) != 0 {
		t.Fatalf("successful uninstall retained durable credential: %+v", persistedTokens)
	}
	tombstone, ok := monitor.hostContinuityStore.Get(host.ID)
	if !ok || tombstone.RemovedAt.IsZero() || !slices.Contains(tombstone.DeniedTokenIDs, token.ID) {
		t.Fatalf("successful uninstall tombstone = (%+v, %v)", tombstone, ok)
	}
	if _, err := monitor.UninstallHostAgent(host.ID, token.ID); err != nil {
		t.Fatalf("idempotent uninstall retry: %v", err)
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

// After a restart a host that has not reported again exists only as its saved
// continuity row, and its offline alert names it agent:<host ID>. The intent
// the operator set on that row must still hold the alert back: alert intent
// resolves through the read state that overlays saved hosts, not through the
// published registry alone, which no longer lists the host.
func TestHostAgentRemovalLifecycleHonorsSavedHostIntentAfterRestart(t *testing.T) {
	dataPath := t.TempDir()
	now := time.Now().UTC()
	token := &config.APITokenRecord{ID: "intent-restart-token", CreatedAt: now.Add(-time.Hour)}
	report := hostRemovalLifecycleReport("intent-restart-host", "intent-restart-machine", "intent-restart-agent", "intent-restart.local", "linux", now)
	if _, err := newHostRemovalLifecycleMonitor(t, dataPath).ApplyHostReport(report, token); err != nil {
		t.Fatalf("initial ApplyHostReport: %v", err)
	}

	restarted := newHostRemovalLifecycleMonitor(t, dataPath)
	store := unifiedresources.NewMemoryStore()
	savedID := unifiedresources.MachineIdentityCanonicalID(unifiedresources.ResourceTypeAgent, "intent-restart-machine")
	if err := store.SetResourceOperatorState(unifiedresources.ResourceOperatorState{CanonicalID: savedID, IntentionallyOffline: true}); err != nil {
		t.Fatalf("set saved host intent: %v", err)
	}
	restarted.SetResourceStore(unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store)))
	for i := 0; i < 3; i++ {
		restarted.evaluateHostAgents(now.Add(10*time.Minute + time.Duration(i)*time.Second))
	}
	if alerts := restarted.alertManager.GetActiveAlerts(); len(alerts) != 0 {
		t.Fatalf("intentionally offline saved host raised %+v after restart, want no alert", alerts)
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

func TestUnifiedStorageMetricSyncPreservesHostRemovalBlock(t *testing.T) {
	monitor := newHostRemovalLifecycleMonitor(t, t.TempDir())
	now := time.Now().UTC().Truncate(time.Second)
	report := hostRemovalLifecycleReport(
		"storage-sync-machine",
		"storage-sync-machine",
		"storage-sync-agent",
		"storage-sync.local",
		"linux",
		now,
	)
	token := &config.APITokenRecord{ID: "storage-sync-token", CreatedAt: now.Add(-time.Hour)}
	host, err := monitor.ApplyHostReport(report, token)
	if err != nil {
		t.Fatalf("initial ApplyHostReport: %v", err)
	}
	if _, err := monitor.RemoveHostAgent(host.ID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}

	resourceStore := unifiedresources.NewMonitorAdapter(nil)
	resourceStore.PopulateFromSnapshot(models.StateSnapshot{
		PBSInstances: []models.PBSInstance{{
			ID:       "pbs-lifecycle-boundary",
			Name:     "pbs-lifecycle-boundary",
			Status:   "online",
			LastSeen: now,
			Datastores: []models.PBSDatastore{{
				Name:   "backups",
				Status: "available",
				Total:  1000,
				Used:   400,
				Free:   600,
				Usage:  40,
			}},
		}},
	})
	monitor.metricsHistory = NewMetricsHistory(16, time.Hour)
	monitor.syncUnifiedStorageMetrics(resourceStore)

	if got := monitor.hostContinuityStore.RemovedEntries(); len(got) != 1 {
		t.Fatalf("storage metric sync changed durable removal entries: %+v", got)
	}
	report.Timestamp = now.Add(time.Minute)
	if _, err := monitor.ApplyHostReport(report, token); err == nil {
		t.Fatal("storage metric sync allowed a removed host to report with its denied token")
	}
}

func TestMockHostAgentLeavingFixtureUsesRemovalLifecycle(t *testing.T) {
	mustSetMockEnabled(t, true)
	t.Cleanup(func() { mustSetMockEnabled(t, false) })
	manager := newMockHostAlertTestManager(t)
	monitor := &Monitor{alertManager: manager}

	memory := models.Memory{Total: 32 << 30, Used: 30 << 30, Free: 2 << 30, Usage: 93.75}
	kept := models.Host{ID: "host-linux-1", Hostname: "apollo-114", Status: "online", Memory: memory}
	departed := models.Host{ID: "host-node-pve9", Hostname: "pve9", LinkedNodeID: "mock-cluster-1-pve9", Status: "online", Memory: memory}

	monitor.evaluateMockHostAgents(monitor.mockModeFence.begin(), []models.Host{kept, departed}, nil, 1)
	// A runtime mock config change rebuilt the estate without pve9.
	monitor.evaluateMockHostAgents(monitor.mockModeFence.begin(), []models.Host{kept}, nil, 2)

	keptAlert := false
	for _, alert := range manager.GetActiveAlerts() {
		switch alert.ResourceID {
		case "agent:" + departed.ID:
			t.Fatalf("agent that left the mock estate kept alert %s", alert.ID)
		case "agent:" + kept.ID:
			keptAlert = keptAlert || alert.Type == "memory"
		}
	}
	if !keptAlert {
		t.Fatal("agent still in the mock estate lost its memory alert")
	}

	// The departed agent's node link is dropped too, so the node it was
	// linked to owns its metric alerts again.
	node := models.Node{
		ID:       "mock-cluster-1-pve9",
		Name:     "pve9",
		Instance: "mock-cluster-1",
		Status:   "online",
		Memory:   memory,
	}
	manager.CheckNode(node)
	for _, alert := range manager.GetActiveAlerts() {
		if alert.ResourceID == node.ID && alert.Type == "memory" {
			return
		}
	}
	t.Fatal("node pve9 raised no memory alert after its agent left the mock estate")
}

func TestMockDockerHostLeavingFixtureUsesRemovalLifecycle(t *testing.T) {
	mustSetMockEnabled(t, true)
	t.Cleanup(func() { mustSetMockEnabled(t, false) })
	manager := newMockHostAlertTestManager(t)
	monitor := &Monitor{alertManager: manager}

	kept := newMockDockerHostWithExitedContainers("nebula-1-mock", "nebula-1")
	// Nested Docker-in-LXC hosts are named after their guest's VMID, which a
	// runtime mock config change can shift.
	departed := newMockDockerHostWithExitedContainers("proxmox-lxc-docker:Production West:pve1:108", "pve1-ct108")

	monitor.evaluateMockDockerHosts(monitor.mockModeFence.begin(), []models.DockerHost{kept, departed}, 1)
	monitor.evaluateMockDockerHosts(monitor.mockModeFence.begin(), []models.DockerHost{kept, departed}, 1)
	if ids := dockerAlertIDsForHost(manager, departed.ID); len(ids) != len(departed.Containers) {
		t.Fatalf("departing host opened %v, want one alert per exited container", ids)
	}

	monitor.evaluateMockDockerHosts(monitor.mockModeFence.begin(), []models.DockerHost{kept}, 2)

	if ids := dockerAlertIDsForHost(manager, departed.ID); len(ids) != 0 {
		t.Fatalf("Docker host that left the mock estate kept alerts %v", ids)
	}
	if ids := dockerAlertIDsForHost(manager, kept.ID); len(ids) != len(kept.Containers) {
		t.Fatalf("Docker host still in the mock estate has alerts %v, want one per exited container", ids)
	}
}

func TestLeavingMockModeReleasesFixtureAgentNodeLinksOnEveryRunningMonitor(t *testing.T) {
	setMockSamplerTestEnv(t, time.Hour, 5*time.Minute)
	pinDefaultMockEstate(t)
	mustSetMockEnabled(t, true)
	defaultMonitor, tenantMonitor := startTenantMonitors(t)

	// org-b's start pass registers the fixture agents in org-b's own alert
	// manager, each owning the usage alerts of the fixture node it runs on.
	var linked models.Node
	waitForCondition(t, 30*time.Second, func() bool {
		tenantMonitor.mockHostAgentsMu.Lock()
		defer tenantMonitor.mockHostAgentsMu.Unlock()
		for _, host := range tenantMonitor.mockHostAgents {
			if host.Status != "online" || host.LinkedNodeID == "" {
				continue
			}
			for _, node := range mock.CurrentFixtureGraph().State.Nodes {
				if node.ID == host.LinkedNodeID {
					linked = node
					return true
				}
			}
		}
		return false
	}, "org-b registered no online fixture agent linked to a fixture node")

	// The demo-fixture licence sync switches through the default monitor.
	mustSetMonitorMockMode(t, defaultMonitor, false)

	// org-b's fixture agents go through the removal lifecycle too, so the
	// node an agent covered owns its metric alerts again.
	linked.Status = "online"
	linked.Memory = models.Memory{Total: 64 << 30, Used: 60 << 30, Free: 4 << 30, Usage: 93.75}
	tenantMonitor.alertManager.CheckNode(linked)
	for _, alert := range tenantMonitor.alertManager.GetActiveAlerts() {
		if alert.ResourceID == linked.ID && alert.Type == "memory" {
			return
		}
	}
	t.Fatalf("org-b's node %s raised no memory alert: its fixture agent's node link outlived a switch made through the default monitor", linked.ID)
}

// nodeAgentSplitMonitor is a Proxmox node with a guest and the pulse-agent
// on the same machine, monitored through a store-backed resource adapter.
type nodeAgentSplitMonitor struct {
	t       *testing.T
	monitor *Monitor
	store   unifiedresources.ResourceStore
	adapter *unifiedresources.MonitorAdapter
	nodes   []models.Node
	vm      models.VM
	report  agentshost.Report
	host    models.Host
}

func newNodeAgentSplitMonitor(t *testing.T) *nodeAgentSplitMonitor {
	return newNodeAgentSplitMonitorWithStore(t, nil)
}

// flakyDecisionStore fails reads of the operator's manual decisions while
// fail is set, as a store that cannot be read for a moment does.
type flakyDecisionStore struct {
	unifiedresources.ResourceStore
	fail atomic.Bool
}

func (s *flakyDecisionStore) GetLinks() ([]unifiedresources.ResourceLink, error) {
	if s.fail.Load() {
		return nil, errors.New("decisions unreadable")
	}
	return s.ResourceStore.GetLinks()
}

func (s *flakyDecisionStore) GetExclusions() ([]unifiedresources.ResourceExclusion, error) {
	if s.fail.Load() {
		return nil, errors.New("decisions unreadable")
	}
	return s.ResourceStore.GetExclusions()
}

func newNodeAgentSplitMonitorWithStore(t *testing.T, wrap func(unifiedresources.ResourceStore) unifiedresources.ResourceStore) *nodeAgentSplitMonitor {
	t.Helper()
	sqliteStore, err := unifiedresources.NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	var store unifiedresources.ResourceStore = sqliteStore
	if wrap != nil {
		store = wrap(store)
	}
	now := time.Now().UTC()
	f := &nodeAgentSplitMonitor{
		t:       t,
		monitor: issue1654Monitor(),
		store:   store,
		adapter: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store)),
		nodes: []models.Node{
			{ID: "lab-pve1", Name: "pve1", Instance: "lab", Host: "https://10.0.0.5:8006", Status: "online", LastSeen: now},
			{ID: "lab-pve2", Name: "pve2", Instance: "lab", Host: "https://10.0.0.6:8006", Status: "online", LastSeen: now},
		},
		vm: models.VM{ID: "lab-pve1-100", VMID: 100, Name: "web", Node: "pve1", Instance: "lab", Type: "qemu", Status: "running", LastSeen: now},
		report: agentshost.Report{
			Agent: agentshost.AgentInfo{ID: "agent-pve1", Version: "6.2.0", IntervalSeconds: 30, DiskExclude: []string{"/dev/sdz"}},
			Host: agentshost.HostInfo{
				ID: "pve1-host", MachineID: "0123456789abcdef", Hostname: "pve1", Platform: "linux", ReportIP: "10.0.0.5",
			},
			Timestamp: now,
		},
	}
	f.monitor.hostContinuityStore = config.NewHostContinuityStore(t.TempDir(), nil)
	f.monitor.SetResourceStore(f.adapter)
	return f
}

// cycle polls the nodes, rebuilds the adapter, takes one agent report and
// rebuilds again, as the poll loop and report ingest interleave.
func (f *nodeAgentSplitMonitor) cycle() {
	f.t.Helper()
	f.monitor.state.UpdateNodesForInstance("lab", f.nodes)
	f.monitor.state.UpdateVMsForInstance("lab", []models.VM{f.vm})
	f.adapter.PopulateFromSnapshot(f.monitor.state.GetSnapshot())
	f.report.Timestamp = f.report.Timestamp.Add(time.Second)
	host, err := f.monitor.ApplyHostReport(f.report, nil)
	if err != nil {
		f.t.Fatal(err)
	}
	f.host = host
	f.adapter.PopulateFromSnapshot(f.monitor.state.GetSnapshot())
}

// recordReportMerge records what POST /api/resources/{id}/report-merge
// records for the joined node and agent row, read from a registry rebuilt
// from the monitor's snapshot as the resources API builds one.
func (f *nodeAgentSplitMonitor) recordReportMerge() {
	f.t.Helper()
	registry := unifiedresources.NewRegistry(f.store)
	registry.IngestSnapshot(f.monitor.state.GetSnapshot())
	merged := ""
	for _, resource := range registry.List() {
		if resource.Proxmox != nil && resource.Agent != nil {
			merged = resource.ID
		}
	}
	if merged == "" {
		f.t.Fatal("no joined node and agent row to report")
	}
	for _, target := range registry.SourceTargets(merged) {
		if target.CandidateID == "" || target.CandidateID == merged {
			continue
		}
		if err := f.store.AddExclusion(unifiedresources.ResourceExclusion{ResourceA: merged, ResourceB: target.CandidateID, CreatedAt: time.Now().UTC()}); err != nil {
			f.t.Fatal(err)
		}
	}
}

// seedLXCFilesystems caches one container filesystem reading from the agent,
// as an earlier report with a Proxmox LXC inventory would have.
func (f *nodeAgentSplitMonitor) seedLXCFilesystems() {
	f.t.Helper()
	f.monitor.proxmoxLXCFilesystemsMu.Lock()
	defer f.monitor.proxmoxLXCFilesystemsMu.Unlock()
	f.monitor.proxmoxLXCFilesystemsCache = map[string]agentLXCFilesystemCacheEntry{
		agentLXCFilesystemCacheKey("lab", "pve1", 200): {
			agentID:   f.host.ID,
			name:      "ct200",
			disks:     []models.Disk{{Mountpoint: "/", Total: 10 << 30, Used: 1 << 30}},
			expiresAt: time.Now().Add(10 * time.Minute),
		},
	}
}

func (f *nodeAgentSplitMonitor) lxcFilesystemEntries() int {
	f.monitor.proxmoxLXCFilesystemsMu.RLock()
	defer f.monitor.proxmoxLXCFilesystemsMu.RUnlock()
	return len(f.monitor.proxmoxLXCFilesystemsCache)
}

// reportOnly takes one agent report with no poll or rebuild before it.
func (f *nodeAgentSplitMonitor) reportOnly() models.Host {
	f.t.Helper()
	f.report.Timestamp = f.report.Timestamp.Add(time.Second)
	host, err := f.monitor.ApplyHostReport(f.report, nil)
	if err != nil {
		f.t.Fatal(err)
	}
	f.host = host
	return host
}

// exclusions lists the store's exclusions as "a|b", and how many name the
// node's own source-specific ID.
func (f *nodeAgentSplitMonitor) exclusions() (all []string, namingNode int) {
	f.t.Helper()
	exclusions, err := f.store.GetExclusions()
	if err != nil {
		f.t.Fatal(err)
	}
	nodeCandidate := unifiedresources.SourceSpecificID(unifiedresources.ResourceTypeAgent, unifiedresources.SourceProxmox, f.nodes[0].ID)
	for _, exclusion := range exclusions {
		all = append(all, exclusion.ResourceA+"|"+exclusion.ResourceB)
		if exclusion.ResourceA == nodeCandidate || exclusion.ResourceB == nodeCandidate {
			namingNode++
		}
	}
	slices.Sort(all)
	return all, namingNode
}

// assertLinked checks every reader of the node<->agent link: the state's
// two sides, report ingest's result, the node's linked agent in the read
// state (agent deployment, service discovery and the physical disk poll's
// SMART fallback and --disk-exclude patterns read it), the node's disk
// source host, the guest's inherited agent (guest discovery and commands)
// and the shared-system alert correlation, and whether the registry lists
// the pair as one row.
func (f *nodeAgentSplitMonitor) assertLinked(step string, want bool) {
	f.t.Helper()
	node, agent := f.nodes[0].ID, f.host.ID
	wantAgent, wantNode := "", ""
	if want {
		wantAgent, wantNode = agent, node
	}
	snapshot := f.monitor.state.GetSnapshot()
	for _, n := range snapshot.Nodes {
		if n.ID == node && n.LinkedAgentID != wantAgent {
			f.t.Fatalf("%s: state node links agent %q, want %q", step, n.LinkedAgentID, wantAgent)
		}
	}
	for _, h := range snapshot.Hosts {
		if h.ID == agent && h.LinkedNodeID != wantNode {
			f.t.Fatalf("%s: state agent links node %q, want %q", step, h.LinkedNodeID, wantNode)
		}
	}
	if f.host.LinkedNodeID != wantNode {
		f.t.Fatalf("%s: report ingest linked node %q, want %q", step, f.host.LinkedNodeID, wantNode)
	}
	readState := f.monitor.GetUnifiedReadStateOrSnapshot()
	for _, view := range readState.Nodes() {
		if view.SourceID() == node && view.LinkedAgentID() != wantAgent {
			f.t.Fatalf("%s: read-state node links agent %q, want %q", step, view.LinkedAgentID(), wantAgent)
		}
	}
	diskHost := f.monitor.linkedHostForNode("lab", node, f.nodes[0].Name)
	if (diskHost != nil) != want || (diskHost != nil && diskHost.ID != agent) {
		f.t.Fatalf("%s: node disk source host %+v, want linked=%v", step, diskHost, want)
	}
	joined := false
	for _, resource := range f.adapter.GetAll() {
		if resource.Type == unifiedresources.ResourceTypeVM && resource.Proxmox != nil && resource.Proxmox.LinkedAgentID != wantAgent {
			f.t.Fatalf("%s: guest inherits agent %q, want %q", step, resource.Proxmox.LinkedAgentID, wantAgent)
		}
		if resource.Proxmox != nil && resource.Agent != nil {
			joined = true
		}
	}
	if joined != want {
		f.t.Fatalf("%s: registry lists the pair joined=%v, want %v", step, joined, want)
	}
	if correlated := sharedSystemAlertCorrelationForHost(f.host, snapshot.Nodes) != nil; correlated != want {
		f.t.Fatalf("%s: shared-system alert correlation=%v, want %v", step, correlated, want)
	}
}

// An operator's split of a Proxmox node and its pulse-agent (report-merge
// or unlink in the resources API) stops monitoring treating them as one
// machine: the agent's SMART inventory, --disk-exclude patterns, guest
// discovery target, deployment status and alert correlation no longer
// reach the node. An exclusion that splits neither leaves all of it alone.
func TestOperatorSplitStopsMonitoringTreatingNodeAndAgentAsLinked(t *testing.T) {
	t.Run("unrelated exclusion keeps the link", func(t *testing.T) {
		f := newNodeAgentSplitMonitor(t)
		f.cycle()
		f.cycle()
		f.assertLinked("before", true)
		joined := ""
		for _, resource := range f.adapter.GetAll() {
			if resource.Proxmox != nil && resource.Agent != nil {
				joined = resource.ID
			}
		}
		f.seedLXCFilesystems()
		if err := f.store.AddExclusion(unifiedresources.ResourceExclusion{ResourceA: joined, ResourceB: "agent-00000000deadbeef", CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
		f.cycle()
		f.cycle()
		f.assertLinked("after an unrelated exclusion", true)
		if got := f.lxcFilesystemEntries(); got != 1 {
			t.Fatalf("an unrelated exclusion changed the agent's container filesystem readings to %d", got)
		}
	})

	t.Run("report-merge splits the link", func(t *testing.T) {
		f := newNodeAgentSplitMonitor(t)
		f.cycle()
		f.cycle()
		f.assertLinked("before", true)
		f.seedLXCFilesystems()
		f.recordReportMerge()
		f.cycle()
		f.cycle()
		f.assertLinked("after report-merge", false)
		if got := f.lxcFilesystemEntries(); got != 0 {
			t.Fatalf("the split left %d of the agent's container filesystem readings on the node's containers", got)
		}
	})
}

// The agents API's node link is the operator's newer decision about the
// pair, so it replaces an earlier split in the resource store, which both
// the registry and monitoring read, instead of leaving monitoring linked
// and the registry split. A link to another node lifts nothing. A split
// recorded after a manual link wins in turn and ends the manual intent, so
// the node is no longer reserved for the agent, and a later relink of the
// rows in the resources API joins them again through the link monitoring
// infers.
func TestManualNodeLinkReplacesAnOperatorSplit(t *testing.T) {
	f := newNodeAgentSplitMonitor(t)
	f.cycle()
	f.cycle()
	f.recordReportMerge()
	f.cycle()
	f.cycle()
	f.assertLinked("after report-merge", false)
	split, namingNode := f.exclusions()
	if namingNode != 1 {
		t.Fatalf("report-merge recorded %v, want one exclusion naming the node's candidate", split)
	}

	if err := f.monitor.LinkHostAgent(f.host.ID, f.nodes[1].ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.exclusions(); !slices.Equal(got, split) {
		t.Fatalf("linking the agent to another node changed the exclusions from %v to %v", split, got)
	}
	// The node's split goes. Report-merge's exclusion of the agent's own
	// candidate from the machine-derived ID it merged under stays: it never
	// split the node (nodeAgentSplit), and it is also what splitting another
	// source off the agent records.
	if err := f.monitor.LinkHostAgent(f.host.ID, f.nodes[0].ID); err != nil {
		t.Fatal(err)
	}
	if got, namingNode := f.exclusions(); namingNode != 0 || len(got) != len(split)-1 {
		t.Fatalf("manual link left exclusions %v of %v", got, split)
	}
	f.cycle()
	f.cycle()
	f.assertLinked("after the manual link", true)

	f.recordReportMerge()
	f.cycle()
	f.cycle()
	f.assertLinked("after a split of the manual link", false)
	entry, ok := f.monitor.hostContinuityStore.Get(f.host.ID)
	if !ok || entry.NodeLinkSource != "automatic" || entry.LinkedNodeID != "" {
		t.Fatalf("split left the manual intent persisted: %+v", entry)
	}
	if reserved := f.monitor.hostContinuityStore.NodeLinkReservedByOther("another-agent", f.nodes[0].ID); reserved {
		t.Fatal("the ended manual intent still reserves the node against other agents")
	}

	var nodeRow, agentRow string
	for _, resource := range f.adapter.GetAll() {
		switch {
		case resource.Proxmox != nil && resource.Agent == nil && resource.Proxmox.SourceID == f.nodes[0].ID:
			nodeRow = resource.ID
		case resource.Agent != nil && resource.Proxmox == nil:
			agentRow = resource.ID
		}
	}
	if err := f.store.AddLink(unifiedresources.ResourceLink{ResourceA: nodeRow, ResourceB: agentRow, PrimaryID: nodeRow, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	f.cycle()
	f.cycle()
	f.assertLinked("after a resources API relink", true)
}

// A manual node link that cannot persist its intent leaves the operator's
// split as it was: the exclusions it removed are recorded again, with their
// original times, and the pair stays apart.
func TestManualNodeLinkKeepsTheSplitWhenItsIntentCannotPersist(t *testing.T) {
	f := newNodeAgentSplitMonitor(t)
	f.cycle()
	f.cycle()
	f.recordReportMerge()
	f.cycle()
	f.cycle()
	before, err := f.store.GetExclusions()
	if err != nil {
		t.Fatal(err)
	}

	continuity := f.monitor.hostContinuityStore
	f.monitor.hostContinuityStore = nil
	if err := f.monitor.LinkHostAgent(f.host.ID, f.nodes[0].ID); err == nil {
		t.Fatal("manual link without intent storage succeeded")
	}
	f.monitor.hostContinuityStore = continuity
	after, err := f.store.GetExclusions()
	if err != nil {
		t.Fatal(err)
	}
	key := func(exclusions []unifiedresources.ResourceExclusion) []string {
		out := make([]string, 0, len(exclusions))
		for _, exclusion := range exclusions {
			out = append(out, exclusion.ResourceA+"|"+exclusion.ResourceB+"|"+exclusion.CreatedAt.UTC().Format(time.RFC3339Nano))
		}
		slices.Sort(out)
		return out
	}
	if !slices.Equal(key(before), key(after)) {
		t.Fatalf("failed manual link changed the split from %v to %v", key(before), key(after))
	}
	f.cycle()
	f.cycle()
	f.assertLinked("after the failed manual link", false)
}

// A manual node link the state holds back only because the split cannot be
// confirmed (the store is unreadable just after the link removed it) keeps
// its persisted intent, and links once the store reads again. Only a split
// the store confirms ends a manual intent.
func TestManualNodeLinkSurvivesAnUnreadableSplitStore(t *testing.T) {
	var flaky *flakyDecisionStore
	f := newNodeAgentSplitMonitorWithStore(t, func(store unifiedresources.ResourceStore) unifiedresources.ResourceStore {
		flaky = &flakyDecisionStore{ResourceStore: store}
		return flaky
	})
	f.cycle()
	f.cycle()
	f.recordReportMerge()
	f.cycle()
	f.cycle()
	f.assertLinked("after report-merge", false)
	if err := f.monitor.LinkHostAgent(f.host.ID, f.nodes[0].ID); err != nil {
		t.Fatal(err)
	}

	// A report before the adapter rebuilds: its generation still holds the
	// split the link removed, which the unreadable store cannot confirm gone,
	// so the state holds the link back while the persisted intent keeps its
	// node. (Without the persisted fallback the entry would lose the node.)
	flaky.fail.Store(true)
	held := f.reportOnly()
	if held.LinkedNodeID != "" {
		t.Fatalf("the report linked node %q although the split could not be confirmed gone", held.LinkedNodeID)
	}
	snapshot := f.monitor.state.GetSnapshot()
	for _, n := range snapshot.Nodes {
		if n.LinkedAgentID != "" {
			t.Fatalf("state links node %s to agent %q", n.ID, n.LinkedAgentID)
		}
	}
	entry, ok := f.monitor.hostContinuityStore.Get(f.host.ID)
	if !ok || entry.NodeLinkSource != "manual" || entry.LinkedNodeID != f.nodes[0].ID {
		t.Fatalf("a held-back manual link lost its persisted intent: %+v", entry)
	}

	// A rebuild that cannot read the decisions loads none, so the link
	// returns for now and the intent is unchanged.
	f.cycle()
	entry, ok = f.monitor.hostContinuityStore.Get(f.host.ID)
	if !ok || entry.NodeLinkSource != "manual" || entry.LinkedNodeID != f.nodes[0].ID {
		t.Fatalf("an unreadable split store ended the manual intent: %+v", entry)
	}
	flaky.fail.Store(false)
	f.cycle()
	f.cycle()
	f.assertLinked("after the store reads again", true)
	if entry, _ := f.monitor.hostContinuityStore.Get(f.host.ID); entry.NodeLinkSource != "manual" {
		t.Fatalf("manual intent became %q", entry.NodeLinkSource)
	}
}

// sequencedSplitDecider answers "not split" for its first calls and "split"
// from then on, as an operator's split recorded while a report is in flight
// does.
type sequencedSplitDecider struct {
	calls     atomic.Int32
	splitFrom atomic.Int32
}

func (d *sequencedSplitDecider) ProxmoxNodeAgentSplit(models.Node, models.Host) bool {
	return d.calls.Add(1) > d.splitFrom.Load()
}

// A split recorded after a report's own check but before the state stores
// the agent is applied by the state; the report then neither returns, links
// nor persists the node the state refused.
func TestReportUsesTheLinkTheStateKeptWhenASplitLandsMidReport(t *testing.T) {
	// The split is first seen by the state's own check of the agent's link
	// (call 2), or only by its check of each node linked to the agent (call 3).
	for _, seenAtCall := range []int32{1, 2} {
		t.Run(fmt.Sprintf("seen after %d checks", seenAtCall), func(t *testing.T) {
			testReportUsesTheLinkTheStateKept(t, seenAtCall)
		})
	}
}

func testReportUsesTheLinkTheStateKept(t *testing.T, splitFrom int32) {
	f := newNodeAgentSplitMonitor(t)
	f.cycle()
	f.cycle()
	f.assertLinked("before", true)
	f.seedLXCFilesystems()

	decider := &sequencedSplitDecider{}
	decider.splitFrom.Store(1 << 30)
	f.monitor.state.SetNodeAgentSplitDecider(decider)
	f.monitor.state.UpdateNodesForInstance("lab", f.nodes)
	f.monitor.state.UpsertHost(f.host)
	// The report's own check (its first call) finds no split; the state's
	// store of the agent (the next) does.
	decider.calls.Store(0)
	decider.splitFrom.Store(splitFrom)

	held := f.reportOnly()
	if held.LinkedNodeID != "" {
		t.Fatalf("the report returned node %q that the state refused", held.LinkedNodeID)
	}
	snapshot := f.monitor.state.GetSnapshot()
	for _, n := range snapshot.Nodes {
		if n.LinkedAgentID != "" {
			t.Fatalf("state links node %s to agent %q", n.ID, n.LinkedAgentID)
		}
	}
	for _, h := range snapshot.Hosts {
		if h.LinkedNodeID != "" {
			t.Fatalf("state links agent %s to node %q", h.ID, h.LinkedNodeID)
		}
	}
	entry, ok := f.monitor.hostContinuityStore.Get(f.host.ID)
	if !ok || entry.LinkedNodeID != "" {
		t.Fatalf("the report persisted node %q that the state refused: %+v", entry.LinkedNodeID, entry)
	}
	if got := f.lxcFilesystemEntries(); got != 0 {
		t.Fatalf("%d container filesystem readings from the agent outlived the split", got)
	}
}

// A confirmed split of a manual link ends the intent and clears what the
// agent cached for the node's containers even where nothing infers another
// node for the agent (inference has no node to substitute, so no later branch
// would clear it).
func TestConfirmedSplitOfAManualLinkClearsTheAgentsContainerReadings(t *testing.T) {
	f := newNodeAgentSplitMonitor(t)
	f.cycle()
	f.cycle()
	if err := f.monitor.LinkHostAgent(f.host.ID, f.nodes[0].ID); err != nil {
		t.Fatal(err)
	}
	f.cycle()
	f.cycle()
	f.assertLinked("after the manual link", true)

	// The agent no longer looks like the node to inference, and nothing but
	// the manual intent ties it to the node.
	f.report.Host.Hostname = "elsewhere"
	f.report.Host.ReportIP = ""
	f.seedLXCFilesystems()
	f.recordReportMerge()
	f.cycle()
	f.cycle()

	entry, ok := f.monitor.hostContinuityStore.Get(f.host.ID)
	if !ok || entry.NodeLinkSource != "automatic" || entry.LinkedNodeID != "" {
		t.Fatalf("a confirmed split left the manual intent: %+v", entry)
	}
	if got := f.lxcFilesystemEntries(); got != 0 {
		t.Fatalf("%d container filesystem readings from the split agent outlived its manual link", got)
	}
}

// A split agent whose name is its node's name must not hand the node its
// sensors through the unlinked-agent hostname fallback.
func TestSplitAgentSensorsDoNotReachTheNodeThroughItsName(t *testing.T) {
	f := newNodeAgentSplitMonitor(t)
	f.report.Sensors.TemperatureCelsius = map[string]float64{"cpu_package": 61}
	f.cycle()
	f.cycle()
	node := f.nodes[0]
	if temp := f.monitor.getHostAgentTemperatureForNode(node); temp == nil || temp.CPUPackage != 61 {
		t.Fatalf("a linked agent's temperature did not reach its node: %+v", temp)
	}

	f.recordReportMerge()
	f.cycle()
	f.cycle()
	f.assertLinked("after report-merge", false)
	if temp := f.monitor.getHostAgentTemperatureForNode(node); temp != nil {
		t.Fatalf("the split agent's temperature reached the node through its name: %+v", temp)
	}

	// A reading the agent supplied before the split must not outlive the
	// agent's lease because the split took it out of the node's slot.
	carried := &models.Temperature{Available: true, CPUPackage: 61, LastUpdate: f.host.LastSeen}
	if !f.monitor.carriedTemperatureOutlivesAgentLease(node, carried, time.Now().Add(time.Hour)) {
		t.Fatal("a split agent's carried reading would outlive the agent's lapsed lease")
	}
}

// Container filesystem readings an agent cached are not shown for a node the
// operator split from that agent. Seeded after the split, so no cleanup has
// run, they are ignored; a linked pair's reading is shown.
func TestAgentContainerReadingsAreNotShownForANodeSplitFromTheAgent(t *testing.T) {
	f := newNodeAgentSplitMonitor(t)
	f.cycle()
	f.cycle()
	f.recordReportMerge()
	f.cycle()
	f.cycle()
	f.assertLinked("after report-merge", false)

	// The agent's reports name no node (as when its name matches none), so
	// nothing on the report path clears what is cached.
	f.seedLXCFilesystems()
	container := models.Container{VMID: 200, Name: "ct200", Status: "running"}
	f.monitor.enrichContainerWithAgentLXCFilesystems("lab", "pve1", &container, time.Now())
	if len(container.Disks) != 0 {
		t.Fatalf("the split agent's readings were shown for the node's container: %+v", container.Disks)
	}

	// A node that is not in state under this connection (a proven duplicate
	// connection folded it into another's) is not split from the agent.
	f.monitor.proxmoxLXCFilesystemsMu.Lock()
	f.monitor.proxmoxLXCFilesystemsCache[agentLXCFilesystemCacheKey("ghost", "pve9", 300)] = agentLXCFilesystemCacheEntry{
		agentID:   f.host.ID,
		name:      "ct300",
		disks:     []models.Disk{{Mountpoint: "/", Total: 10 << 30, Used: 1 << 30}},
		expiresAt: time.Now().Add(10 * time.Minute),
	}
	f.monitor.proxmoxLXCFilesystemsMu.Unlock()
	ghost := models.Container{VMID: 300, Name: "ct300", Status: "running"}
	f.monitor.enrichContainerWithAgentLXCFilesystems("ghost", "pve9", &ghost, time.Now())
	if len(ghost.Disks) != 1 {
		t.Fatalf("a reading for a node that left this connection's slot was hidden: %+v", ghost.Disks)
	}

	// The control: with the agent linked the same reading is shown.
	linked := newNodeAgentSplitMonitor(t)
	linked.cycle()
	linked.cycle()
	linked.assertLinked("control", true)
	linked.seedLXCFilesystems()
	control := models.Container{VMID: 200, Name: "ct200", Status: "running"}
	linked.monitor.enrichContainerWithAgentLXCFilesystems("lab", "pve1", &control, time.Now())
	if len(control.Disks) != 1 {
		t.Fatalf("a linked agent's reading was not shown: %+v", control.Disks)
	}
}
