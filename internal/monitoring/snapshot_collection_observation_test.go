package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type snapshotCollectionReply struct {
	snapshots []proxmox.Snapshot
	err       error
}

type snapshotCollectionClient struct {
	mockPVEClientExtra
	replies map[string]snapshotCollectionReply
}

func (c *snapshotCollectionClient) GetVMSnapshots(_ context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	r := c.replies[fmt.Sprintf("qemu/%s/%d", node, vmid)]
	return r.snapshots, r.err
}

func (c *snapshotCollectionClient) GetContainerSnapshots(_ context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	r := c.replies[fmt.Sprintf("lxc/%s/%d", node, vmid)]
	return r.snapshots, r.err
}

func (c *snapshotCollectionClient) GetStorage(context.Context, string) ([]proxmox.Storage, error) {
	return nil, nil // These observations do not require any storage or guest-agent operation.
}

func snapshotCollectionMonitor(t *testing.T, kind string, withPeers bool) *Monitor {
	t.Helper()
	am := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(am.Stop)
	cfg := am.GetConfig()
	cfg.Enabled, cfg.ActivationState = true, alerts.ActivationActive
	cfg.SnapshotDefaults = alerts.SnapshotAlertConfig{Enabled: true, WarningDays: 7, CriticalDays: 14}
	cfg.TimeThresholds = map[string]int{}
	am.UpdateConfig(cfg)
	m := &Monitor{state: models.NewState(), alertManager: am}
	for _, site := range []string{"site", "other-site"} {
		if site == "other-site" && !withPeers {
			continue
		}
		var vms []models.VM
		var containers []models.Container
		for _, node := range []string{"node-a", "node-b"} {
			if node == "node-b" && (!withPeers || site == "other-site") {
				continue
			}
			id := alerts.BuildGuestKey(site, node, 101)
			if kind == "qemu" {
				vms = append(vms, models.VM{ID: id, VMID: 101, Name: "database", Node: node, Instance: site, Type: kind, Status: "running"})
			} else {
				containers = append(containers, models.Container{ID: id, VMID: 101, Name: "database", Node: node, Instance: site, Type: kind, Status: "running"})
			}
		}
		m.state.UpdateGuestsForInstance(site, vms, containers)
	}
	return m
}

func snapshotCollectionByID(m *Monitor) map[string]models.GuestSnapshot {
	result := map[string]models.GuestSnapshot{}
	for _, snapshot := range m.state.GetSnapshot().PVEBackups.GuestSnapshots {
		result[snapshot.ID] = snapshot
	}
	return result
}

func snapshotCollectionAlert(t *testing.T, m *Monitor, resourceID string) models.Alert {
	t.Helper()
	for _, alert := range m.GetState().ActiveAlerts {
		if alert.Type == "snapshot-age" && alert.ResourceID == resourceID {
			return alert
		}
	}
	t.Fatalf("snapshot warning missing for %s: %+v", resourceID, m.GetState().ActiveAlerts)
	return models.Alert{}
}

func TestSnapshotCollectionFailureIsNotRemoval(t *testing.T) {
	for _, kind := range []string{"qemu", "lxc"} {
		for _, message := range []string{"API error 596: node offline", "snapshot service not available", "permission denied", "request timed out", "unexpected response"} {
			t.Run(kind+"/"+message, func(t *testing.T) {
				m := snapshotCollectionMonitor(t, kind, true)
				old := proxmox.Snapshot{Name: "weekly", SnapTime: time.Now().Add(-16 * 24 * time.Hour).Unix(), Description: "last accepted listing", Parent: "base"}
				key := kind + "/node-a/101"
				peerKey := kind + "/node-b/101"
				client := &snapshotCollectionClient{replies: map[string]snapshotCollectionReply{key: {snapshots: []proxmox.Snapshot{old}}, peerKey: {snapshots: []proxmox.Snapshot{old}}}}
				m.pollGuestSnapshots(context.Background(), "site", client)
				m.pollGuestSnapshots(context.Background(), "other-site", client)
				resourceID := "site:node-a:101"
				initial := snapshotCollectionAlert(t, m, resourceID)
				if err := m.alertManager.AcknowledgeAlert(initial.ID, "operator"); err != nil {
					t.Fatal(err)
				}
				initial = snapshotCollectionAlert(t, m, resourceID)
				before := snapshotCollectionByID(m)
				targetID := "site-node-a-101-weekly"
				otherID := "other-site-node-a-101-weekly"
				// Discard even a partial result returned alongside an error.
				client.replies[key] = snapshotCollectionReply{snapshots: []proxmox.Snapshot{{Name: "unaccepted", SnapTime: time.Now().Unix()}}, err: errors.New(message)}
				client.replies[peerKey] = snapshotCollectionReply{snapshots: []proxmox.Snapshot{{Name: "fresh-peer", SnapTime: time.Now().Add(-time.Hour).Unix()}}}
				for range 3 {
					m.pollGuestSnapshots(context.Background(), "site", client)
				}
				after := snapshotCollectionByID(m)
				if !reflect.DeepEqual(after[targetID], before[targetID]) {
					t.Fatalf("failed %s listing removed or changed accepted snapshot: before=%+v after=%+v", kind, before[targetID], after[targetID])
				}
				if len(after) != 3 || !reflect.DeepEqual(after[otherID], before[otherID]) || after["site-node-b-101-fresh-peer"].Name != "fresh-peer" {
					t.Fatalf("failed listing leaked across node/instance or blocked accepted peer: %+v", after)
				}
				current := snapshotCollectionAlert(t, m, resourceID)
				if current.ID != initial.ID || !current.StartTime.Equal(initial.StartTime) || !current.Acknowledged || !reflect.DeepEqual(current.Metadata["snapshotCreatedAt"], initial.Metadata["snapshotCreatedAt"]) {
					t.Fatalf("failed listing replaced occurrence, acknowledgement or creation evidence: before=%+v after=%+v", initial, current)
				}
				if len(m.GetState().ActiveAlerts) != 2 {
					t.Fatalf("only failed guest and untouched other instance should retain warnings: %+v", m.GetState().ActiveAlerts)
				}
				// A later accepted listing can still confirm the old snapshot.
				client.replies[key] = snapshotCollectionReply{snapshots: []proxmox.Snapshot{old}}
				m.pollGuestSnapshots(context.Background(), "site", client)
				if current = snapshotCollectionAlert(t, m, resourceID); current.ID != initial.ID || !current.StartTime.Equal(initial.StartTime) || !current.Acknowledged {
					t.Fatal("read recovery replaced an existing warning")
				}
				// Only a successful empty listing establishes removal for this guest.
				client.replies[key] = snapshotCollectionReply{}
				m.pollGuestSnapshots(context.Background(), "site", client)
				m.pollGuestSnapshots(context.Background(), "site", client)
				if _, exists := snapshotCollectionByID(m)[targetID]; exists || len(m.GetState().ActiveAlerts) != 1 {
					t.Fatalf("accepted removal failed or cleared unrelated warning: %+v", m.GetState())
				}
				snapshotCollectionAlert(t, m, "other-site:node-a:101")
			})
		}
	}
}

func TestSnapshotCollectionFailureWithoutPreviousEvidenceDoesNotInventSnapshot(t *testing.T) {
	m := snapshotCollectionMonitor(t, "lxc", false)
	client := &snapshotCollectionClient{replies: map[string]snapshotCollectionReply{"lxc/node-a/101": {err: errors.New("596: snapshot API not available")}}}
	m.pollGuestSnapshots(context.Background(), "site", client)
	if len(snapshotCollectionByID(m)) != 0 || len(m.GetState().ActiveAlerts) != 0 {
		t.Fatal("a failed first read invented a snapshot or warning")
	}
}

func TestSnapshotCollectionFailureStillHonoursDisabledAlerts(t *testing.T) {
	m := snapshotCollectionMonitor(t, "lxc", false)
	client := &snapshotCollectionClient{replies: map[string]snapshotCollectionReply{"lxc/node-a/101": {snapshots: []proxmox.Snapshot{{Name: "weekly", SnapTime: time.Now().Add(-16 * 24 * time.Hour).Unix()}}}}}
	m.pollGuestSnapshots(context.Background(), "site", client)
	snapshotCollectionAlert(t, m, "site:node-a:101")
	cfg := m.alertManager.GetConfig()
	cfg.SnapshotDefaults.Enabled = false
	m.alertManager.UpdateConfig(cfg)
	client.replies["lxc/node-a/101"] = snapshotCollectionReply{err: errors.New("596: snapshot API not available")}
	m.pollGuestSnapshots(context.Background(), "site", client)
	if len(snapshotCollectionByID(m)) != 1 || len(m.GetState().ActiveAlerts) != 0 {
		t.Fatal("retained inventory overrode explicit alert disablement")
	}
}

// Opt-in frames connect the actual poller, live manager snapshot and browser.
// No guest-agent, backup, freeze/thaw or external service is invoked.
func TestSnapshotCollectionBrowserProducer(t *testing.T) {
	m := snapshotCollectionMonitor(t, "lxc", false)
	old := proxmox.Snapshot{Name: "weekly", SnapTime: time.Now().Add(-16 * 24 * time.Hour).Unix(), Description: "last accepted listing"}
	client := &snapshotCollectionClient{replies: map[string]snapshotCollectionReply{}}
	type frame struct {
		Snapshots []models.GuestSnapshot `json:"snapshots"`
		Alerts    []models.Alert         `json:"alerts"`
	}
	frames := map[string]frame{}
	for _, phase := range []string{"observed", "failed-596", "failed-unavailable", "confirmed-empty"} {
		reply := snapshotCollectionReply{snapshots: []proxmox.Snapshot{old}}
		switch phase {
		case "failed-596":
			reply = snapshotCollectionReply{err: errors.New("API error 596: node offline")}
		case "failed-unavailable":
			reply = snapshotCollectionReply{err: errors.New("snapshot service not available")}
		case "confirmed-empty":
			reply = snapshotCollectionReply{}
		}
		client.replies["lxc/node-a/101"] = reply
		m.pollGuestSnapshots(context.Background(), "site", client)
		state := m.GetState()
		frames[phase] = frame{Snapshots: state.PVEBackups.GuestSnapshots, Alerts: state.ActiveAlerts}
		want := 1
		if phase == "confirmed-empty" {
			want = 0
		}
		if len(state.PVEBackups.GuestSnapshots) != want || len(state.ActiveAlerts) != want {
			t.Fatalf("phase %s: want %d snapshots and warnings, got %d/%d", phase, want, len(state.PVEBackups.GuestSnapshots), len(state.ActiveAlerts))
		}
	}
	if os.Getenv("PULSE_SNAPSHOT_COLLECTION_FRAMES") == "1" {
		data, err := json.Marshal(frames)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("SNAPSHOT_COLLECTION_FRAMES=%s\n", data)
	}
}
