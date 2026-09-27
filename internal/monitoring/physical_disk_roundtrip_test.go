package monitoring

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Monitor construction restores notification destinations before admitting
// agent observations. A previously queued disk alert must not be cancelled as
// "delivery disabled" just because its saved webhook has not loaded yet.
func TestMonitorStartupRetainsQueuedAgentDiskAlertUntilDestinationRestored(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	persistence := config.NewConfigPersistence(dir)
	if err := persistence.SaveAlertConfig(alerts.AlertConfig{
		Enabled: true, ActivationState: alerts.ActivationActive,
	}); err != nil {
		t.Fatal(err)
	}
	hook := notifications.WebhookConfig{
		ID: "disk-ops", Name: "disk-ops", URL: "http://127.0.0.1:1/alert",
		Enabled: true, Service: "generic",
	}
	if err := persistence.SaveWebhooks([]notifications.WebhookConfig{hook}); err != nil {
		t.Fatal(err)
	}
	configJSON, err := json.Marshal(hook)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := notifications.NewNotificationQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := seed.Enqueue(&notifications.QueuedNotification{
		ID: "startup-agent-disk-wearout", Type: "webhook", Status: notifications.QueueStatusPending,
		DestinationID: hook.ID, Config: configJSON, MaxAttempts: 3,
		Alerts: []*alerts.Alert{{
			ID: "disk-wearout", Type: "disk-wearout", ResourceName: "agent-disk",
			Level: alerts.AlertLevelWarning, StartTime: time.Now().Add(-time.Minute),
		}},
	}); err != nil {
		_ = seed.Stop()
		t.Fatal(err)
	}
	if err := seed.Stop(); err != nil {
		t.Fatal(err)
	}

	monitor, err := New(&config.Config{DataPath: dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(monitor.Stop)
	queue := monitor.GetNotificationManager().GetQueue()
	if queue == nil {
		t.Fatal("notification queue unavailable")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		stats, err := queue.GetQueueStats()
		if err != nil {
			t.Fatal(err)
		}
		if stats[string(notifications.QueueStatusCancelled)] != 0 {
			t.Fatalf("queued disk alert cancelled before destination restore: %v", stats)
		}
		telemetry, err := queue.GetTelemetryStats(time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		if telemetry.Attempts != 0 {
			if telemetry.Deliveries != 0 {
				t.Fatalf("synthetic blocked endpoint unexpectedly delivered: %+v", telemetry)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("restored destination never reached queue processor: %v", stats)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Exercise the real skipped-poll path, not just the ID helper: canonical views
// are converted back into source state and then re-ingested by the adapter.
func TestPhysicalDiskSkippedPollPreservesSourceIdentity(t *testing.T) {
	for _, device := range []string{"/dev/sdx", "sdx"} {
		t.Run(device, func(t *testing.T) {
			state := models.NewState()
			nodes := []models.Node{{ID: "pve-node1", Name: "node1", Instance: "pve"}, {ID: "pve-node2", Name: "node2", Instance: "pve"}}
			state.UpdateNodesForInstance("pve", nodes)
			var disks []models.PhysicalDisk
			for _, fixture := range []struct{ node, target string }{{"node1", ""}, {"node2", ""}, {"node1", "megaraid,0"}, {"node1", "megaraid,1"}} {
				disks = append(disks, models.PhysicalDisk{
					ID:       unifiedresources.ProxmoxPhysicalDiskSourceID("pve", fixture.node, device, "", fixture.target),
					Instance: "pve", Node: fixture.node, DevPath: device, Target: fixture.target,
					Model: "USB fixture", Size: 1024, Temperature: 37, Wearout: -1, LastChecked: time.Now(),
					ExpectedUpdateInterval: 5 * time.Minute,
				})
			}
			state.UpdatePhysicalDisks("pve", disks)
			adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
			adapter.PopulateFromSnapshot(state.GetSnapshot())
			m := &Monitor{state: state, resourceStore: adapter, lastPhysicalDiskPoll: map[string]time.Time{"pve": time.Now()}}
			canonical := map[string]bool{}
			for _, view := range adapter.PhysicalDisks() {
				canonical[view.ID()] = true
			}
			if len(canonical) != len(disks) {
				t.Fatalf("initial disks = %d, want %d", len(canonical), len(disks))
			}
			for cycle := 0; cycle < 8; cycle++ {
				m.maybePollPhysicalDisksAsync(context.Background(), "pve", &config.PVEInstance{}, nil, nil, nil, nil)
				got := state.GetSnapshot().PhysicalDisks
				if len(got) != len(disks) {
					t.Fatalf("cycle %d: source count %d", cycle, len(got))
				}
				wantIDs := map[string]bool{}
				for _, disk := range disks {
					wantIDs[disk.ID] = true
				}
				for _, disk := range got {
					if !wantIDs[disk.ID] {
						t.Fatalf("cycle %d: canonical ID leaked into provider state: %q", cycle, disk.ID)
					}
					if disk.DevPath != device || disk.Temperature != 37 || disk.Size != 1024 || disk.ExpectedUpdateInterval != 5*time.Minute {
						t.Fatalf("cycle %d: metadata lost: %+v", cycle, disk)
					}
				}
				adapter.PopulateFromSnapshot(state.GetSnapshot())
				views := adapter.PhysicalDisks()
				if len(views) != len(disks) {
					t.Fatalf("cycle %d: view count = %d", cycle, len(views))
				}
				for _, view := range views {
					if !canonical[view.ID()] {
						t.Fatalf("cycle %d: canonical identity churn: %s", cycle, view.ID())
					}
				}
				// Retain the JSON-visible projection as well as the typed read-state checks.
				payload, err := json.Marshal(adapter.GetAll())
				if err != nil {
					t.Fatal(err)
				}
				var resources []unifiedresources.Resource
				if err := json.Unmarshal(payload, &resources); err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, resource := range resources {
					if resource.Type != unifiedresources.ResourceTypePhysicalDisk {
						continue
					}
					count++
					if !canonical[resource.ID] || resource.Proxmox == nil || !wantIDs[resource.Proxmox.SourceID] || resource.PhysicalDisk == nil || resource.PhysicalDisk.Temperature != 37 {
						t.Fatalf("cycle %d: JSON disk identity/metadata lost: %+v", cycle, resource)
					}
				}
				if count != len(disks) {
					t.Fatalf("cycle %d: JSON disk count %d", cycle, count)
				}
			}
			// A confirmed full inventory removal must still remove source-owned disks.
			state.UpdatePhysicalDisks("pve", nil)
			adapter.PopulateFromSnapshot(state.GetSnapshot())
			if got := len(adapter.PhysicalDisks()); got != 0 {
				t.Fatalf("removed inventory retained %d disks", got)
			}
		})
	}
}

func TestPhysicalDiskReadbackSourceIDFallback(t *testing.T) {
	for _, resource := range []unifiedresources.Resource{
		{ID: "canonical"},
		{ID: "canonical", Proxmox: &unifiedresources.ProxmoxData{}},
		{ID: "canonical", Proxmox: &unifiedresources.ProxmoxData{SourceID: " native "}},
	} {
		view := unifiedresources.NewPhysicalDiskView(&resource)
		want := "canonical"
		if resource.Proxmox != nil && resource.Proxmox.SourceID != "" {
			want = "native"
		}
		if got := physicalDiskFromReadStateView(&view).ID; got != want {
			t.Fatalf("ID = %q, want %q", got, want)
		}
	}
	var view unifiedresources.PhysicalDiskView
	if view.SourceID() != "" {
		t.Fatal("nil resource has a source ID")
	}
}

// TestMergeHostAgentSMARTIntoDisks_AgentWearoutDoesNotHideLowPVELife pins the
// #2112 regression: a worn drive whose NVMe endurance log intermittently reads
// PercentageUsed 0 must not have its Proxmox-reported remaining life raised to
// 100, which resolved the low-life alert and let it re-fire on the next poll.
func TestMergeHostAgentSMARTIntoDisks_AgentWearoutDoesNotHideLowPVELife(t *testing.T) {
	disks := []models.PhysicalDisk{{
		ID:      "d1",
		Node:    "pve1",
		Serial:  "SER1",
		Type:    "nvme",
		Health:  "PASSED",
		Wearout: 0,
	}}
	nodes := []models.Node{{Name: "pve1", LinkedAgentID: "host-1"}}
	used := 0
	hosts := []models.Host{{
		ID: "host-1",
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{{
				Device: "/dev/nvme0n1",
				Serial: "SER1",
				Health: "PASSED",
				Attributes: &models.SMARTAttributes{
					PercentageUsed: &used,
				},
			}},
		},
	}}

	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if result[0].Wearout != 0 {
		t.Fatalf("agent endurance reading raised the remaining life of a worn disk: wearout=%d", result[0].Wearout)
	}
	if result[0].SmartAttributes == nil || result[0].SmartAttributes.PercentageUsed == nil || *result[0].SmartAttributes.PercentageUsed != 0 {
		t.Fatalf("agent SMART attributes were not merged: %+v", result[0].SmartAttributes)
	}
}

// TestMergeHostAgentSMARTIntoDisks_AgentWearoutFillsUnreportedPVELife confirms
// the agent still supplies endurance when the Proxmox inventory reports none.
func TestMergeHostAgentSMARTIntoDisks_AgentWearoutFillsUnreportedPVELife(t *testing.T) {
	disks := []models.PhysicalDisk{{
		ID:      "d1",
		Node:    "pve1",
		Serial:  "SER1",
		Type:    "nvme",
		Wearout: -1,
	}}
	nodes := []models.Node{{Name: "pve1", LinkedAgentID: "host-1"}}
	used := 40
	hosts := []models.Host{{
		ID: "host-1",
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{{
				Device: "/dev/nvme0n1",
				Serial: "SER1",
				Attributes: &models.SMARTAttributes{
					PercentageUsed: &used,
				},
			}},
		},
	}}

	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if result[0].Wearout != 60 {
		t.Fatalf("agent endurance did not fill unreported PVE life: wearout=%d", result[0].Wearout)
	}
}
