package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type wornDiskPVEClient struct {
	fakeStorageClient
}

func (*wornDiskPVEClient) GetDisks(ctx context.Context, node string) ([]proxmox.Disk, error) {
	return []proxmox.Disk{{
		DevPath: "/dev/sdb",
		Model:   "WDC WDS500G1R0A",
		Serial:  "1234",
		Type:    "sata",
		Health:  "PASSED",
		Wearout: 1,
		Size:    500107862016,
	}}, nil
}

// node1DiskAlerts filters to alerts raised for the test disk so persisted
// active alerts loaded from a previous test's manager cannot leak in.
func node1DiskAlerts(m *Monitor) []alerts.Alert {
	var matched []alerts.Alert
	for _, alert := range m.alertManager.GetActiveAlerts() {
		if (alert.Type == "disk-wearout" || alert.Type == "disk-health") && alert.Node == "node1" {
			matched = append(matched, alert)
		}
	}
	return matched
}

func waitForPhysicalDisks(t *testing.T, m *Monitor) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if len(m.state.GetSnapshot().PhysicalDisks) > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("expected physical disks to land in state")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A monitor reload rebuilds host state from live agent reports, so a disk
// poll that runs before a node's agent has re-reported must not evaluate
// disk alerts for that node: it would treat the agent's --disk-exclude
// patterns as absent and fire alerts the next cycle resolves (#1674).
func TestMaybePollPhysicalDisksAsync_DefersAlertsUntilAgentLinkSettles(t *testing.T) {
	m := &Monitor{
		state:                models.NewState(),
		lastPhysicalDiskPoll: make(map[string]time.Time),
		alertManager:         alerts.NewManager(),
		startTime:            time.Now(),
	}

	m.state.UpdateNodesForInstance("pve1", []models.Node{{
		ID:       "pve1-node1",
		Name:     "node1",
		Instance: "pve1",
	}})

	m.maybePollPhysicalDisksAsync(
		context.Background(),
		"pve1",
		&config.PVEInstance{},
		&wornDiskPVEClient{},
		[]proxmox.Node{{Node: "node1", Status: "online"}},
		map[string]string{"node1": "online"},
		[]models.Node{{Name: "node1"}},
	)

	waitForPhysicalDisks(t, m)
	if alertsNow := node1DiskAlerts(m); len(alertsNow) != 0 {
		t.Fatalf("expected no disk alerts during the agent-link settle window, got %+v", alertsNow)
	}
}

func TestMaybePollPhysicalDisksAsync_EvaluatesAlertsOnceSettled(t *testing.T) {
	m := &Monitor{
		state:                models.NewState(),
		lastPhysicalDiskPoll: make(map[string]time.Time),
		alertManager:         alerts.NewManager(),
		startTime:            time.Now().Add(-2 * hostAgentLinkSettleWindow),
	}

	m.state.UpdateNodesForInstance("pve1", []models.Node{{
		ID:       "pve1-node1",
		Name:     "node1",
		Instance: "pve1",
	}})

	m.maybePollPhysicalDisksAsync(
		context.Background(),
		"pve1",
		&config.PVEInstance{},
		&wornDiskPVEClient{},
		[]proxmox.Node{{Node: "node1", Status: "online"}},
		map[string]string{"node1": "online"},
		[]models.Node{{Name: "node1"}},
	)

	waitForPhysicalDisks(t, m)
	found := false
	for _, alert := range node1DiskAlerts(m) {
		if alert.Type == "disk-wearout" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a disk-wearout alert once the settle window has passed, got %+v", m.alertManager.GetActiveAlerts())
	}
}

// A node whose agent is already linked evaluates immediately even inside the
// settle window, and the agent's --disk-exclude patterns suppress the alert.
func TestMaybePollPhysicalDisksAsync_LinkedNodeSuppressesExcludedDisk(t *testing.T) {
	m := &Monitor{
		state:                models.NewState(),
		lastPhysicalDiskPoll: make(map[string]time.Time),
		alertManager:         alerts.NewManager(),
		startTime:            time.Now(),
	}

	m.state.UpsertHost(models.Host{
		ID:          "host-1",
		Hostname:    "node1",
		DiskExclude: []string{"sdb"},
	})
	m.state.UpdateNodesForInstance("pve1", []models.Node{{
		ID:            "pve1-node1",
		Name:          "node1",
		Instance:      "pve1",
		LinkedAgentID: "host-1",
	}})

	m.maybePollPhysicalDisksAsync(
		context.Background(),
		"pve1",
		&config.PVEInstance{},
		&wornDiskPVEClient{},
		[]proxmox.Node{{Node: "node1", Status: "online"}},
		map[string]string{"node1": "online"},
		[]models.Node{{Name: "node1"}},
	)

	waitForPhysicalDisks(t, m)
	if alertsNow := node1DiskAlerts(m); len(alertsNow) != 0 {
		t.Fatalf("expected agent disk exclusions to suppress the wearout alert, got %+v", alertsNow)
	}
}
