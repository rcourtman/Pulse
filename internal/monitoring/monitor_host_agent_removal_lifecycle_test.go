package monitoring

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func newHostRemovalLifecycleMonitor(t *testing.T, dataPath string) *Monitor {
	t.Helper()
	monitor := &Monitor{
		state:               models.NewState(),
		alertManager:        alerts.NewManager(),
		hostTokenBindings:   make(map[string]string),
		removedHostAgents:   make(map[string]time.Time),
		rateTracker:         NewRateTracker(),
		config:              &config.Config{DataPath: dataPath},
		hostContinuityStore: config.NewHostContinuityStore(dataPath, nil),
	}
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
