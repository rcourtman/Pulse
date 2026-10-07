package monitoring

import (
	"context"
	"encoding/json"
	"sync"
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
	if reporting.Temperature != 38 || reporting.Collection.Temperature != diskinventory.Available(diskinventory.LegacyHostAgentSource) {
		t.Fatalf("legacy agent temperature not recorded as collected: temp=%d collection=%+v", reporting.Temperature, reporting.Collection)
	}
	if !diskinventory.TemperatureCollected(reporting.Temperature, reporting.Collection) {
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
		if diskinventory.TemperatureCollected(got.Temperature, got.Collection) {
			t.Fatalf("%s: a silent legacy agent's retained temperature must not reach history", name)
		}
	}
}

// A legacy agent (before collection provenance) withdraws its readings without
// a source when its lease expires, while monitoring stamps its reading copied
// onto the Proxmox disk with the legacy agent source. When no disk poll
// refreshes that copy (the host is down), the canonical disk merged from the
// agent's row and the Proxmox row must still follow the agent's withdrawal,
// and collect the reading again once the agent reports.
func TestSilentLegacyAgentCanonicalDiskFollowsItsWithdrawal(t *testing.T) {
	state := models.NewState()
	state.UpdateNodesForInstance("pve1", []models.Node{{
		ID: "pve1-node1", Name: "node1", Instance: "pve1", Status: "online",
		LastSeen: time.Now(), LinkedAgentID: "agent-1",
	}})
	report := func(seen time.Time) {
		state.UpsertHost(models.Host{
			ID: "agent-1", Hostname: "node1", LinkedNodeID: "pve1-node1", Status: "online",
			IntervalSeconds: 30, LastSeen: seen,
			Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{
				Device: "/dev/sdb", Serial: "LEGACY2", Type: "sata", Health: "PASSED", Temperature: 38,
			}}},
		})
	}
	lastReport := time.Now()
	report(lastReport)
	inventory := models.PhysicalDisk{
		ID: "pve1-node1-sdb", Node: "node1", Instance: "pve1", DevPath: "/dev/sdb", Serial: "LEGACY2",
		Type: "sata", Health: "PASSED", Wearout: -1, LastChecked: time.Now(),
		Collection: &diskinventory.CollectionStatus{
			Temperature: diskinventory.Unsupported("proxmox_disks", "Proxmox disk inventory does not expose temperature"),
		},
	}
	// The disk poll while the agent reported is the copy the Proxmox row
	// keeps once its host stops being polled.
	state.UpdatePhysicalDisks("pve1", mergeHostAgentSMARTIntoDisks(
		[]models.PhysicalDisk{inventory}, state.GetSnapshot().Nodes, state.GetHosts()))
	canonical := func() (int, *diskinventory.CollectionStatus) {
		t.Helper()
		adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
		adapter.PopulateFromSnapshot(state.GetSnapshot())
		disks := adapter.PhysicalDisks()
		if len(disks) != 1 || disks[0].Serial() != "LEGACY2" {
			t.Fatalf("canonical disks = %d, want the agent and Proxmox rows merged into one", len(disks))
		}
		return disks[0].Temperature(), disks[0].Collection()
	}

	if temperature, collection := canonical(); temperature != 38 || !diskinventory.TemperatureCollected(temperature, collection) {
		t.Fatalf("reporting legacy agent: temperature=%d collection=%+v, want 38 collected", temperature, collection)
	}

	if _, expired := state.ExpireHostTelemetry("agent-1", lastReport); !expired {
		t.Fatal("the agent's lease did not expire")
	}
	temperature, collection := canonical()
	if temperature != 38 || diskinventory.TemperatureCollected(temperature, collection) {
		t.Fatalf("silent legacy agent: temperature=%d collection=%+v, want the retained 38 not collected", temperature, collection)
	}
	if collection.Temperature.Reason != models.HostAgentStoppedReportingReason {
		t.Fatalf("silent legacy agent: temperature state %+v, want the agent's own withdrawal", collection.Temperature)
	}

	report(time.Now())
	if temperature, collection := canonical(); temperature != 38 || !diskinventory.TemperatureCollected(temperature, collection) {
		t.Fatalf("resumed legacy agent: temperature=%d collection=%+v, want 38 collected again", temperature, collection)
	}
}

// A disk that goes into standby keeps its pre-sleep temperature as retained
// evidence (preserveUnavailablePhysicalDiskEvidence), with the agent's standby
// state. Full disk polls during standby must not record that value again.
func TestStandbyDiskRetainedTemperatureIsNotRecordedAsHistory(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	state := models.NewState()
	state.UpdateNodesForInstance("pve1", []models.Node{{
		ID: "pve1-node1", Name: "node1", Instance: "pve1", Status: "online",
		LastSeen: time.Now(), LinkedAgentID: "agent-1",
	}})
	report := func(row models.HostDiskSMART) {
		row.Device, row.Serial, row.Type, row.Health = "/dev/sdb", "WD-SILENT1", "sata", "PASSED"
		state.UpsertHost(models.Host{
			ID: "agent-1", Hostname: "node1", LinkedNodeID: "pve1-node1", Status: "online",
			IntervalSeconds: 30, LastSeen: time.Now(),
			Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{row}},
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
		alertManager: alertManager, startTime: time.Now().Add(-time.Hour),
		lastPhysicalDiskPoll: make(map[string]time.Time),
	}
	fullPoll := func() models.PhysicalDisk {
		t.Helper()
		adapter.PopulateFromSnapshot(state.GetSnapshot())
		started := time.Now()
		delete(m.lastPhysicalDiskPoll, "pve1")
		m.maybePollPhysicalDisksAsync(context.Background(), "pve1", &config.PVEInstance{}, &silentAgentDiskPVEClient{},
			[]proxmox.Node{{Node: "node1", Status: "online"}}, map[string]string{"node1": "online"}, nil)
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
	samples := func(d models.PhysicalDisk) int {
		return len(history.GetDiskMetrics(unifiedresources.PhysicalDiskMetricID(d), "smart_temp", time.Hour))
	}

	report(models.HostDiskSMART{Temperature: 38, Collection: &diskinventory.CollectionStatus{
		Temperature: diskinventory.Available("smartctl"),
	}})
	if got := fullPoll(); got.Temperature != 38 || samples(got) != 1 {
		t.Fatalf("awake disk temperature not recorded: temp=%d samples=%d", got.Temperature, samples(got))
	}

	// The agent keeps reporting, but the disk is now asleep.
	report(models.HostDiskSMART{Standby: true, Collection: &diskinventory.CollectionStatus{
		Temperature: diskinventory.Unavailable("smartctl", "disk is in standby"),
	}})
	for poll := 0; poll < 2; poll++ {
		got := fullPoll()
		if got.Temperature != 38 || got.Collection == nil || got.Collection.Temperature.State != diskinventory.FieldUnavailable {
			t.Fatalf("poll %d: standby disk lost its retained temperature or claims it was collected: temp=%d collection=%+v",
				poll, got.Temperature, got.Collection)
		}
		if n := samples(got); n != 1 {
			t.Fatalf("poll %d: standby disk's pre-sleep temperature recorded as new history: samples=%d", poll, n)
		}
	}
}

// An Unraid agent reports array disk temperatures in its Unraid inventory,
// which carries no per-field provenance, and a SMART row without a temperature
// falls back to that inventory reading. Once the agent's reporting lease
// expires both rows stay as last-known context, so the canonical disks must say
// the retained temperature is no longer collected. The next report collects
// it again.
func TestSilentUnraidAgentDiskTemperatureIsRetainedButNotCollected(t *testing.T) {
	now := time.Now()
	state := models.NewState()
	report := func(seen time.Time, temperature int) {
		state.UpsertHost(models.Host{
			ID: "agent-tower", Hostname: "tower", Status: "online", IntervalSeconds: 30, LastSeen: seen,
			Unraid: &models.HostUnraidStorage{ArrayStarted: true, Disks: []models.HostUnraidDisk{
				{Name: "disk1", Device: "sdc", Role: "data", Status: "online", Serial: "UNRAID-ONLY1", Temperature: temperature},
				{Name: "disk2", Device: "sdd", Role: "data", Status: "online", Serial: "UNRAID-SMART2", Temperature: temperature + 2},
				{Name: "disk3", Device: "sde", Role: "data", Status: "online", Serial: "UNRAID-SAS3", Temperature: temperature + 4},
			}},
			// The SMART rows for disk2 and disk3 carry no temperature of their
			// own; disk3's smartctl could not read one at all.
			Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{
				{Device: "/dev/sdd", Serial: "UNRAID-SMART2", Type: "sata", Health: "PASSED"},
				{Device: "/dev/sde", Serial: "UNRAID-SAS3", Type: "sas", Health: "PASSED", Collection: &diskinventory.CollectionStatus{
					Temperature: diskinventory.Unsupported("smartctl", "no temperature attribute"),
				}},
			}},
		})
	}
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	m := &Monitor{state: state, resourceStore: adapter, startTime: now.Add(-time.Hour)}
	temperatures := func() map[string]diskinventory.FieldStatus {
		t.Helper()
		adapter.PopulateFromSnapshot(state.GetSnapshot())
		got := make(map[string]diskinventory.FieldStatus)
		for _, view := range adapter.PhysicalDisks() {
			status := diskinventory.FieldStatus{}
			if collection := view.Collection(); collection != nil {
				status = collection.Temperature
			}
			if _, seen := got[view.Serial()]; seen {
				t.Fatalf("disk %s surfaced twice", view.Serial())
			}
			got[view.Serial()] = status
			wantTemperature := map[string]int{"UNRAID-ONLY1": 37, "UNRAID-SMART2": 39, "UNRAID-SAS3": 41}[view.Serial()]
			if view.Temperature() != wantTemperature {
				t.Fatalf("disk %s temperature = %d, want %d", view.Serial(), view.Temperature(), wantTemperature)
			}
		}
		if len(got) != 3 {
			t.Fatalf("canonical disks = %v, want the three Unraid disks", got)
		}
		return got
	}

	// A reporting agent's inventory readings are collected, including where
	// a SMART row borrows them.
	collected := map[string]diskinventory.FieldStatus{
		"UNRAID-ONLY1":  diskinventory.Available("unraid"),
		"UNRAID-SMART2": diskinventory.Available("unraid"),
		"UNRAID-SAS3":   diskinventory.Available("unraid"),
	}
	report(now, 37)
	for serial, status := range temperatures() {
		if status != collected[serial] {
			t.Fatalf("reporting agent: disk %s temperature status = %+v, want %+v", serial, status, collected[serial])
		}
	}

	state.TouchHost("agent-tower", now.Add(-hostAgentHealthWindow(30)-time.Minute))
	m.evaluateHostAgents(now)
	for serial, status := range temperatures() {
		if status != diskinventory.Unavailable("unraid", "host agent stopped reporting") {
			t.Fatalf("silent agent: disk %s retained temperature presented as collected: %+v", serial, status)
		}
	}

	report(time.Now(), 37)
	m.evaluateHostAgents(time.Now())
	for serial, status := range temperatures() {
		if status != collected[serial] {
			t.Fatalf("resumed agent: disk %s temperature status = %+v, want collected again (%+v)", serial, status, collected[serial])
		}
	}
}

type slotDiskPVEClient struct {
	fakeStorageClient
	mu   sync.Mutex
	disk proxmox.Disk
}

func (client *slotDiskPVEClient) setDisk(disk proxmox.Disk) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.disk = disk
}

func (client *slotDiskPVEClient) GetDisks(context.Context, string) ([]proxmox.Disk, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	return []proxmox.Disk{client.disk}, nil
}

// A disk swapped into the same slot keeps the Proxmox source ID, which is
// path-shaped. When a disk record arrives with the replacement's WWN but an
// empty serial, the poller must not copy the previous occupant's serial onto
// it: the registry keys the disk on that serial, so the replacement would take
// over the old disk's canonical resource, and every later poll would copy the
// borrowed serial forward again. Current Proxmox spells a missing serial as
// "unknown", which the poller records as empty, as it does for producers that
// omit the field, such as the host-agent fallback rows. The fake client below
// stands in for any of them.
func TestPhysicalDiskReplacementInSameSlotDoesNotInheritPreviousSerial(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	const (
		oldSerial = "ZR5OLD0001"
		oldWWN    = "0x5000c500aaaa0001"
		newWWN    = "0x5000c500bbbb0002"
	)
	type pollResult struct {
		disk models.PhysicalDisk
		view *unifiedresources.PhysicalDiskView
	}
	newHarness := func(t *testing.T) (*slotDiskPVEClient, func() pollResult) {
		t.Helper()
		state := models.NewState()
		state.UpdateNodesForInstance("pve1", []models.Node{{
			ID: "pve1-node1", Name: "node1", Instance: "pve1", Status: "online", LastSeen: time.Now(),
		}})
		alertManager := alerts.NewManager()
		t.Cleanup(alertManager.Stop)
		adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
		m := &Monitor{
			state: state, resourceStore: adapter, alertManager: alertManager,
			startTime: time.Now().Add(-time.Hour), lastPhysicalDiskPoll: make(map[string]time.Time),
		}
		client := &slotDiskPVEClient{}
		poll := func() pollResult {
			t.Helper()
			adapter.PopulateFromSnapshot(state.GetSnapshot())
			started := time.Now()
			delete(m.lastPhysicalDiskPoll, "pve1")
			m.maybePollPhysicalDisksAsync(context.Background(), "pve1", &config.PVEInstance{}, client,
				[]proxmox.Node{{Node: "node1", Status: "online"}}, map[string]string{"node1": "online"}, nil)
			deadline := time.Now().Add(3 * time.Second)
			for {
				if disks := state.GetSnapshot().PhysicalDisks; len(disks) == 1 && !disks[0].LastChecked.Before(started) {
					adapter.PopulateFromSnapshot(state.GetSnapshot())
					views := adapter.PhysicalDisks()
					if len(views) != 1 {
						t.Fatalf("canonical physical disks = %d, want one", len(views))
					}
					return pollResult{disk: disks[0], view: views[0]}
				}
				if time.Now().After(deadline) {
					t.Fatal("physical disk poll did not land in state")
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
		return client, poll
	}
	pveDisk := func(serial, wwn string) proxmox.Disk {
		return proxmox.Disk{
			DevPath: "/dev/sdb", Model: "ST4000NM000A", Serial: serial, WWN: wwn,
			Type: "hdd", Health: "PASSED", Wearout: 100, Size: 4000787030016,
		}
	}

	t.Run("replacement reporting a different WWN", func(t *testing.T) {
		client, poll := newHarness(t)
		client.setDisk(pveDisk(oldSerial, oldWWN))
		first := poll()
		if first.disk.Serial != oldSerial || first.view.Serial() != oldSerial {
			t.Fatalf("original disk serial: state %q, canonical %q", first.disk.Serial, first.view.Serial())
		}

		client.setDisk(pveDisk("", newWWN))
		for round := 1; round <= 2; round++ {
			got := poll()
			if got.disk.Serial != "" || got.view.Serial() != "" {
				t.Fatalf("poll %d: replacement inherited serial: state %q, canonical %q", round, got.disk.Serial, got.view.Serial())
			}
			if got.disk.WWN != newWWN {
				t.Fatalf("poll %d: replacement WWN = %q, want %q", round, got.disk.WWN, newWWN)
			}
			if got.view.ID() == first.view.ID() {
				t.Fatalf("poll %d: replacement took over the old disk's canonical resource %q", round, got.view.ID())
			}
			if got.disk.Collection == nil || got.disk.Collection.Serial.State != diskinventory.FieldMissing {
				t.Fatalf("poll %d: replacement serial collection = %+v, want missing", round, got.disk.Collection)
			}
		}
	})

	t.Run("same disk keeps its serial while Proxmox omits it", func(t *testing.T) {
		for name, missing := range map[string]proxmox.Disk{
			"WWN still reported": pveDisk("", oldWWN),
			"no identity at all": pveDisk("", ""),
		} {
			t.Run(name, func(t *testing.T) {
				client, poll := newHarness(t)
				client.setDisk(pveDisk(oldSerial, oldWWN))
				first := poll()

				client.setDisk(missing)
				for round := 1; round <= 2; round++ {
					got := poll()
					if got.disk.Serial != oldSerial || got.view.ID() != first.view.ID() {
						t.Fatalf("poll %d: same disk lost its identity: serial %q, resource %q (was %q)",
							round, got.disk.Serial, got.view.ID(), first.view.ID())
					}
					if got.disk.Collection == nil || got.disk.Collection.Serial.State != diskinventory.FieldMissing {
						t.Fatalf("poll %d: retained serial presented as collected: %+v", round, got.disk.Collection)
					}
				}
			})
		}
	})
}

// Previous evidence matches a disk by stable hardware identity first and by
// slot (source ID, then device token) only when no reported serial or WWN
// says the slot now holds a different disk. The earlier record is the
// registry's merged view, which keeps a linked agent's WWN spelling; Proxmox
// reports a missing serial or WWN as the literal "unknown".
func TestPreviousPhysicalDiskEvidenceRejectsSlotMatchAcrossConflictingIdentity(t *testing.T) {
	const slotID = "pve1-node1--dev-sdb"
	previous := models.PhysicalDisk{
		ID: slotID, Instance: "pve1", Node: "node1", DevPath: "/dev/sdb",
		Serial: "ZR5OLD0001", WWN: "5-c50-aaaa0001", Temperature: 38, StorageGroup: "tank",
	}
	// Identity cases sit on another device path, so only identity can match.
	moved := func(serial, wwn string) models.PhysicalDisk {
		return models.PhysicalDisk{ID: "pve1-node1--dev-sdc", DevPath: "/dev/sdc", Serial: serial, WWN: wwn}
	}
	inSlot := func(serial, wwn string) models.PhysicalDisk {
		return models.PhysicalDisk{ID: slotID, DevPath: "/dev/sdb", Serial: serial, WWN: wwn}
	}
	for _, tc := range []struct {
		name       string
		current    models.PhysicalDisk
		want       bool
		wantSerial string
	}{
		{"renamed device, same serial", moved("ZR5OLD0001", "unknown"), true, "ZR5OLD0001"},
		{"renamed device, PVE spelling of the agent WWN", moved("unknown", "0x5000c500aaaa0001"), true, "unknown"},
		{"renamed device, old serial reported as WWN", moved("", "ZR5OLD0001"), true, "ZR5OLD0001"},
		{"renamed device, different disk", moved("ZR5NEW0002", "0x5000c500bbbb0002"), false, ""},
		{"same slot, PVE spelling of the agent WWN", inSlot("unknown", "0x5000c500aaaa0001"), true, "unknown"},
		{"same slot, SAS address serial with matching WWN", inSlot("5000c500aaaa0003", "0x5000c500aaaa0001"), true, "5000c500aaaa0003"},
		{"same slot, identity unreported", inSlot("unknown", "unknown"), true, "unknown"},
		{"same slot, serial omitted by producer", inSlot("", ""), true, "ZR5OLD0001"},
		{"same slot, placeholder WWN", inSlot("", "0x0000000000000000"), true, "ZR5OLD0001"},
		{"same slot, different WWN", inSlot("", "0x5000c500bbbb0002"), false, ""},
		{"same slot, different serial", inSlot("ZR5NEW0002", "unknown"), false, ""},
		{"device token only, different WWN", models.PhysicalDisk{ID: "agent-fallback-id", DevPath: "/dev/sdb", WWN: "0x5000c500bbbb0002"}, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current := tc.current
			current.Instance, current.Node = "pve1", "node1"
			current.Collection = &diskinventory.CollectionStatus{
				Serial:      diskinventory.Missing("proxmox_disks", "disk serial was not reported"),
				Temperature: diskinventory.Unsupported("proxmox_disks", "no temperature"),
				Pool:        diskinventory.Unavailable("proxmox_zfs", "query failed"),
			}
			matched, ok := previousPhysicalDiskEvidence(current, []models.PhysicalDisk{previous})
			if ok != tc.want {
				t.Fatalf("previous evidence match = %v, want %v", ok, tc.want)
			}
			if !ok {
				return
			}
			got := preserveUnavailablePhysicalDiskEvidence(current, matched)
			if got.Serial != tc.wantSerial || got.Temperature != previous.Temperature || got.StorageGroup != previous.StorageGroup {
				t.Fatalf("same disk evidence: serial %q (want %q), temperature %d, pool %q", got.Serial, tc.wantSerial, got.Temperature, got.StorageGroup)
			}
		})
	}
}
