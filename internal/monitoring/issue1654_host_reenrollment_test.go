package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func issue1654Monitor() *Monitor {
	return &Monitor{
		state:             models.NewState(),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
		hostTokenBindings: make(map[string]string),
		hostReportOrders:  make(map[string]hostReportOrder),
	}
}

func issue1654Report(timestamp time.Time) agentshost.Report {
	return agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "new-agent-install",
			Version:         "6.2.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "new-install-id",
			MachineID: "machine-stable",
			Hostname:  "disk-host.local",
			Platform:  "linux",
		},
		Timestamp: timestamp,
	}
}

func TestIssue1654FreshInstallReusesStalePhysicalHostIdentity(t *testing.T) {
	monitor := issue1654Monitor()
	createdAt := time.Now().UTC()
	monitor.state.UpsertHost(models.Host{
		ID:        "original-host-id",
		Hostname:  "disk-host.local",
		MachineID: "machine-stable",
		TokenID:   "old-token",
		LastSeen:  createdAt.Add(-time.Minute),
		Status:    "offline",
	})
	monitor.hostTokenBindings["old-token:disk-host.local"] = "original-host-id"

	host, err := monitor.ApplyHostReport(
		issue1654Report(createdAt.Add(time.Minute)),
		&config.APITokenRecord{ID: "fresh-token", CreatedAt: createdAt},
	)
	if err != nil {
		t.Fatalf("ApplyHostReport() error = %v", err)
	}
	if host.ID != "original-host-id" {
		t.Fatalf("host ID = %q, want stable original-host-id", host.ID)
	}
	if got := len(monitor.state.GetHosts()); got != 1 {
		t.Fatalf("host count = %d, want 1", got)
	}
	if got := monitor.hostTokenBindings["fresh-token:disk-host.local"]; got != "original-host-id" {
		t.Fatalf("fresh token binding = %q, want original-host-id", got)
	}
	if _, ok := monitor.hostTokenBindings["old-token:disk-host.local"]; ok {
		t.Fatal("pre-install token binding survived stable-ID re-enrollment")
	}
}

func TestIssue1654ExistingDuplicateGenerationIsSuperseded(t *testing.T) {
	monitor := issue1654Monitor()
	createdAt := time.Now().UTC()
	monitor.state.UpsertHost(models.Host{
		ID:        "stale-host-id",
		Hostname:  "disk-host.local",
		MachineID: "machine-stable",
		TokenID:   "old-token",
		LastSeen:  createdAt.Add(-time.Minute),
		Status:    "offline",
	})
	monitor.state.UpsertHost(models.Host{
		ID:        "current-host-id",
		Hostname:  "disk-host.local",
		MachineID: "machine-stable",
		TokenID:   "fresh-token",
		LastSeen:  createdAt.Add(time.Minute),
		Status:    "online",
	})
	monitor.hostTokenBindings["old-token:disk-host.local"] = "stale-host-id"
	monitor.hostTokenBindings["fresh-token:disk-host.local"] = "current-host-id"

	host, err := monitor.ApplyHostReport(
		issue1654Report(createdAt.Add(2*time.Minute)),
		&config.APITokenRecord{ID: "fresh-token", CreatedAt: createdAt},
	)
	if err != nil {
		t.Fatalf("ApplyHostReport() error = %v", err)
	}
	if host.ID != "current-host-id" {
		t.Fatalf("host ID = %q, want current-host-id", host.ID)
	}
	hosts := monitor.state.GetHosts()
	if len(hosts) != 1 || hosts[0].ID != "current-host-id" {
		t.Fatalf("hosts after supersession = %+v", hosts)
	}
	if _, ok := monitor.hostTokenBindings["old-token:disk-host.local"]; ok {
		t.Fatal("stale token binding survived supersession")
	}
}

func TestIssue1654LivePreexistingAgentIsNotSuperseded(t *testing.T) {
	monitor := issue1654Monitor()
	createdAt := time.Now().UTC()
	monitor.state.UpsertHost(models.Host{
		ID:        "still-live-host",
		Hostname:  "disk-host.local",
		MachineID: "machine-stable",
		TokenID:   "old-token",
		LastSeen:  createdAt.Add(time.Minute),
		Status:    "online",
	})

	_, err := monitor.ApplyHostReport(
		issue1654Report(createdAt.Add(2*time.Minute)),
		&config.APITokenRecord{ID: "fresh-token", CreatedAt: createdAt},
	)
	if err != nil {
		t.Fatalf("ApplyHostReport() error = %v", err)
	}
	if got := len(monitor.state.GetHosts()); got != 2 {
		t.Fatalf("host count = %d, want both live identities preserved", got)
	}
}
