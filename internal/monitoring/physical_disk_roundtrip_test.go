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
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
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

// An agent disk may inherit its linked PVE node's instance for presentation,
// even when the PVE disks/list endpoint did not report that disk. The skipped
// poll must not turn that presentation scope into a PVE inventory observation:
// an empty PVE inventory on the next full poll would then remove the invented
// observation and record spurious configuration changes on every cycle (#2319).
func TestPhysicalDiskSkippedPollDoesNotPromoteAgentOnlySMARTToPVEInventory(t *testing.T) {
	state := models.NewState()
	now := time.Now().UTC()
	state.UpdateNodesForInstance("pve", []models.Node{{
		ID: "pve-node", Name: "node", Instance: "pve", LinkedAgentID: "agent",
		Status: "online", LastSeen: now,
	}})
	state.UpsertHost(models.Host{
		ID: "agent", Hostname: "node", LinkedNodeID: "pve-node",
		Status: "online", LastSeen: now,
		Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{
			// A stable serial keeps the canonical identity unchanged; without
			// the source guard, the false PVE observation produces exactly the
			// tags-only change reported in #2319 rather than a tags+identity row.
			Device: "sda", Serial: "disk-serial", Type: "sata", Health: "PASSED",
		}}},
	})
	store := unifiedresources.NewMemoryStore()
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store))
	monitor := &Monitor{
		state: state, resourceStore: adapter,
		lastPhysicalDiskPoll: map[string]time.Time{"pve": now},
	}
	for cycle := 0; cycle < 3; cycle++ {
		// A successful full PVE inventory read found no disks on this node.
		state.UpdatePhysicalDisks("pve", nil)
		adapter.PopulateFromSnapshot(state.GetSnapshot())
		disks := adapter.PhysicalDisks()
		if len(disks) != 1 || disks[0].Instance() != "pve" {
			t.Fatalf("cycle %d: linked Agent SMART disk missing from presentation: %+v", cycle, disks)
		}
		if _, hasPVE := disks[0].SourceStatus(unifiedresources.SourceProxmox); hasPVE {
			t.Fatalf("cycle %d: Agent-only disk unexpectedly has PVE source", cycle)
		}
		before, err := store.GetRecentChanges(disks[0].ID(), time.Time{}, 100)
		if err != nil {
			t.Fatal(err)
		}
		monitor.maybePollPhysicalDisksAsync(context.Background(), "pve", &config.PVEInstance{}, nil, nil, nil, nil)
		adapter.PopulateFromSnapshot(state.GetSnapshot())
		after, err := store.GetRecentChanges(disks[0].ID(), time.Time{}, 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(after) != len(before) {
			t.Fatalf("cycle %d: unchanged SMART disk emitted %d new history rows: %+v", cycle, len(after)-len(before), after)
		}
		if got := state.GetSnapshot().PhysicalDisks; len(got) != 0 {
			t.Fatalf("cycle %d: Agent-only disk was written into PVE inventory: %+v", cycle, got)
		}
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

type silentAgentDiskPVEClient struct {
	fakeStorageClient
}

// The Proxmox inventory carries no temperature or I/O counters; for this disk
// both come only from the node's linked host agent.
func (*silentAgentDiskPVEClient) GetDisks(ctx context.Context, node string) ([]proxmox.Disk, error) {
	return []proxmox.Disk{{
		DevPath: "/dev/sdb",
		Model:   "WDC WD40EFRX",
		Serial:  "WD-SILENT1",
		Type:    "hdd",
		Health:  "PASSED",
		Wearout: 100,
		Size:    4000787030016,
	}}, nil
}

// A host agent that stops reporting keeps its last SMART rows in state. Once
// its reporting lease expires, the linked Proxmox disk may retain the agent's
// last temperature and I/O counters, but the disk state, the canonical
// resource and temperature history must stop treating them as collected.
func TestSilentLinkedAgentSMARTIsRetainedButNotPresentedAsCollected(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	now := time.Now()
	state := models.NewState()
	state.UpdateNodesForInstance("pve1", []models.Node{{
		ID: "pve1-node1", Name: "node1", Instance: "pve1", Status: "online",
		LastSeen: now, LinkedAgentID: "agent-1",
	}})
	report := func(seen time.Time, temperature int, readBytes uint64) {
		state.UpsertHost(models.Host{
			ID: "agent-1", Hostname: "node1", LinkedNodeID: "pve1-node1",
			Status: "online", IntervalSeconds: 30, LastSeen: seen,
			Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{
				Device: "/dev/sdb", Serial: "WD-SILENT1", Type: "sata", Health: "PASSED",
				Temperature: temperature,
				IO:          &models.DiskIO{Device: "sdb", ReadBytes: readBytes},
				Collection: &diskinventory.CollectionStatus{
					Serial:      diskinventory.Available("smartctl"),
					Temperature: diskinventory.Available("smartctl"),
					IO:          diskinventory.Available("kernel_diskstats"),
				},
			}}},
		})
	}

	alertManager := alerts.NewManager()
	t.Cleanup(alertManager.Stop)
	storeConfig := metrics.DefaultConfig(t.TempDir())
	storeConfig.FlushInterval = time.Hour
	store, err := metrics.NewStore(storeConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	history := NewMetricsHistory(100, time.Hour)
	m := &Monitor{
		state: state, resourceStore: adapter, metricsHistory: history, metricsStore: store,
		alertManager: alertManager, startTime: now.Add(-time.Hour),
		lastPhysicalDiskPoll: make(map[string]time.Time),
	}
	pveNodes := []proxmox.Node{{Node: "node1", Status: "online"}}
	nodeStatus := map[string]string{"node1": "online"}

	disk := func() models.PhysicalDisk {
		t.Helper()
		disks := state.GetSnapshot().PhysicalDisks
		if len(disks) != 1 {
			t.Fatalf("physical disks = %+v, want one", disks)
		}
		return disks[0]
	}
	fullPoll := func() models.PhysicalDisk {
		t.Helper()
		adapter.PopulateFromSnapshot(state.GetSnapshot())
		started := time.Now()
		delete(m.lastPhysicalDiskPoll, "pve1")
		m.maybePollPhysicalDisksAsync(context.Background(), "pve1", &config.PVEInstance{}, &silentAgentDiskPVEClient{}, pveNodes, nodeStatus, nil)
		deadline := time.Now().Add(3 * time.Second)
		for {
			if disks := state.GetSnapshot().PhysicalDisks; len(disks) == 1 && !disks[0].LastChecked.Before(started) {
				return disks[0]
			}
			if time.Now().After(deadline) {
				t.Fatal("full physical disk poll did not land in state")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	skippedPoll := func() models.PhysicalDisk {
		t.Helper()
		adapter.PopulateFromSnapshot(state.GetSnapshot())
		m.lastPhysicalDiskPoll["pve1"] = time.Now()
		m.maybePollPhysicalDisksAsync(context.Background(), "pve1", &config.PVEInstance{}, nil, nil, nil, nil)
		return disk()
	}
	// Chart samples, and the persisted values. The store keys samples by
	// second, so polls run back to back in this test can share one row.
	temperatureSamples := func(d models.PhysicalDisk) (int, []float64) {
		t.Helper()
		id := unifiedresources.PhysicalDiskMetricID(d)
		chart := len(history.GetDiskMetrics(id, "smart_temp", time.Hour))
		store.Flush()
		points, err := store.Query("disk", id, "smart_temp", now.Add(-time.Hour), time.Now().Add(time.Hour), 0)
		if err != nil {
			t.Fatal(err)
		}
		stored := make([]float64, 0, len(points))
		for _, point := range points {
			stored = append(stored, point.Value)
		}
		return chart, stored
	}
	canonicalCollection := func() *diskinventory.CollectionStatus {
		t.Helper()
		adapter.PopulateFromSnapshot(state.GetSnapshot())
		views := adapter.PhysicalDisks()
		if len(views) != 1 {
			t.Fatalf("canonical physical disks = %d, want one", len(views))
		}
		if views[0].Temperature() != 41 {
			t.Fatalf("canonical disk temperature = %d, want the retained 41", views[0].Temperature())
		}
		return views[0].Collection()
	}
	assertRetainedNotCollected := func(step string, got models.PhysicalDisk) {
		t.Helper()
		if got.Temperature != 41 || got.IO == nil || got.IO.ReadBytes != 1000 {
			t.Fatalf("%s: last-known temperature and I/O were not retained: temp=%d io=%+v", step, got.Temperature, got.IO)
		}
		if got.Collection == nil ||
			got.Collection.Temperature.State != diskinventory.FieldUnavailable ||
			got.Collection.IO.State != diskinventory.FieldUnavailable {
			t.Fatalf("%s: silent agent's readings presented as collected: %+v", step, got.Collection)
		}
	}

	report(now, 41, 1000)
	got := fullPoll()
	if got.Temperature != 41 || got.Collection == nil || got.Collection.Temperature.State != diskinventory.FieldAvailable {
		t.Fatalf("reporting agent temperature not merged: temp=%d collection=%+v", got.Temperature, got.Collection)
	}
	if chart, stored := temperatureSamples(got); chart != 1 || len(stored) != 1 || stored[0] != 41 {
		t.Fatalf("reporting agent temperature history: chart=%d stored=%v, want one 41 sample", chart, stored)
	}

	// The agent stops reporting; its lease expires on the next evaluation.
	state.TouchHost("agent-1", now.Add(-hostAgentHealthWindow(30)-time.Minute))
	m.evaluateHostAgents(now)

	// No disk poll runs when the whole host is down (the PVE poll fails before
	// reaching it), so the canonical disk must reflect the expiry on its own
	// even though the Proxmox disk row still carries the earlier merge.
	if collection := canonicalCollection(); collection == nil ||
		collection.Temperature.State != diskinventory.FieldUnavailable ||
		collection.IO.State != diskinventory.FieldUnavailable {
		t.Fatalf("canonical disk presents the silent agent's readings as collected before any disk poll: %+v", collection)
	}
	assertRetainedNotCollected("skipped poll", skippedPoll())
	got = fullPoll()
	assertRetainedNotCollected("full poll", got)
	if chart, stored := temperatureSamples(got); chart != 1 || len(stored) != 1 {
		t.Fatalf("silent agent's retained temperature was recorded as new history: chart=%d stored=%v", chart, stored)
	}
	if collection := canonicalCollection(); collection == nil ||
		collection.Temperature.State != diskinventory.FieldUnavailable ||
		collection.IO.State != diskinventory.FieldUnavailable {
		t.Fatalf("canonical disk presents the silent agent's readings as collected: %+v", collection)
	}

	// A resumed agent is collected again.
	report(time.Now(), 39, 2000)
	m.evaluateHostAgents(time.Now())
	got = fullPoll()
	if got.Temperature != 39 || got.Collection.Temperature.State != diskinventory.FieldAvailable ||
		got.IO == nil || got.IO.ReadBytes != 2000 || got.Collection.IO.State != diskinventory.FieldAvailable {
		t.Fatalf("resumed agent readings not collected: temp=%d io=%+v collection=%+v", got.Temperature, got.IO, got.Collection)
	}
	if chart, stored := temperatureSamples(got); chart != 2 || len(stored) == 0 || stored[len(stored)-1] != 39 {
		t.Fatalf("resumed agent temperature history: chart=%d stored=%v, want a new 39 sample", chart, stored)
	}
}

// Agents before 6.2 send SMART rows without collection provenance. Their
// temperature is still a collected reading (history records it), and once the
// lease expires the disk must stop presenting it as collected even when a
// skipped disk poll carries the previous merge forward.
func TestMergeHostAgentSMARTIntoDisks_LegacyAgentTemperatureFollowsLease(t *testing.T) {
	proxmoxDisk := models.PhysicalDisk{
		ID: "pve1-node1-sdb", Node: "node1", Instance: "pve1", DevPath: "/dev/sdb", Serial: "LEGACY1",
		Collection: &diskinventory.CollectionStatus{
			Temperature: diskinventory.Unsupported("proxmox_disks", "Proxmox disk inventory does not expose temperature"),
		},
	}
	nodes := []models.Node{{Name: "node1", LinkedAgentID: "agent-1"}}
	state := models.NewState()
	state.UpsertHost(models.Host{ID: "agent-1", Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{
		Device: "/dev/sdb", Serial: "LEGACY1", Temperature: 38,
	}}}})

	reporting := mergeHostAgentSMARTIntoDisks([]models.PhysicalDisk{proxmoxDisk}, nodes, state.GetHosts())[0]
	if reporting.Temperature != 38 || reporting.Collection.Temperature != diskinventory.Available(hostAgentLegacySource) {
		t.Fatalf("legacy agent temperature not recorded as collected: temp=%d collection=%+v", reporting.Temperature, reporting.Collection)
	}
	if !diskTemperatureCollected(reporting.Temperature, reporting.Collection) {
		t.Fatal("a reporting legacy agent's temperature must reach history")
	}

	state.ExpireHostTelemetry("agent-1", time.Time{})
	for name, disk := range map[string]models.PhysicalDisk{
		"full poll":    proxmoxDisk,
		"skipped poll": reporting,
	} {
		got := mergeHostAgentSMARTIntoDisks([]models.PhysicalDisk{disk}, nodes, state.GetHosts())[0]
		if got.Temperature != 38 || got.Collection.Temperature.State != diskinventory.FieldUnavailable {
			t.Fatalf("%s: silent legacy agent temperature presented as collected: temp=%d collection=%+v", name, got.Temperature, got.Collection)
		}
		if diskTemperatureCollected(got.Temperature, got.Collection) {
			t.Fatalf("%s: a silent legacy agent's retained temperature must not reach history", name)
		}
	}
}
