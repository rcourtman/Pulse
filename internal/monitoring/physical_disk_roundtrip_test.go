package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
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

	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts, nil)
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

	result := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts, nil)
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

	reporting := mergeHostAgentSMARTIntoDisks([]models.PhysicalDisk{proxmoxDisk}, nodes, state.GetHosts(), nil)[0]
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
		got := mergeHostAgentSMARTIntoDisks([]models.PhysicalDisk{disk}, nodes, state.GetHosts(), nil)[0]
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
		[]models.PhysicalDisk{inventory}, state.GetSnapshot().Nodes, state.GetHosts(), nil))
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
	err  error
}

func (client *slotDiskPVEClient) setDisk(disk proxmox.Disk) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.disk = disk
}

// setError makes the disk query fail, as it does on a wide node that exceeds
// the API window (#1516), until it is cleared with nil.
func (client *slotDiskPVEClient) setError(err error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.err = err
}

func (client *slotDiskPVEClient) GetDisks(context.Context, string) ([]proxmox.Disk, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.err != nil {
		return nil, client.err
	}
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

// QEMU gives a virtual disk without a configured serial a default one built
// from its drive ID or a per-VM counter, so every VM built the same way
// reports the same value. Proxmox's disks/list serial is udev's
// ID_SERIAL_SHORT, which for a nested Proxmox node's first SCSI disk is
// "drive-scsi0" on every node. Treated as hardware identity, the registry
// minted one canonical disk for the whole cluster and every node but one lost
// its disk from inventory.
func TestNestedProxmoxDefaultQEMUSerialsStayPerNode(t *testing.T) {
	now := time.Now()
	snapshot := models.StateSnapshot{}
	for _, node := range []string{"pve1", "pve2", "pve3"} {
		snapshot.Nodes = append(snapshot.Nodes, models.Node{
			ID: "lab-" + node, Name: node, Instance: "lab", Status: "online", LastSeen: now,
		})
		for _, disk := range []struct{ devPath, serial string }{
			{"/dev/sda", "drive-scsi0"},
			{"/dev/sdb", "QM00005"},
			{"/dev/sdc", "drive-scsi0-0-0-1"},
		} {
			snapshot.PhysicalDisks = append(snapshot.PhysicalDisks, models.PhysicalDisk{
				ID:       unifiedresources.ProxmoxPhysicalDiskSourceID("lab", node, disk.devPath, "", ""),
				Instance: "lab", Node: node, DevPath: disk.devPath, Model: "QEMU HARDDISK",
				Serial: disk.serial, Type: "hdd", Health: "PASSED", Size: 34359738368, LastChecked: now,
			})
		}
	}

	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	adapter.PopulateFromSnapshot(snapshot)
	views := adapter.PhysicalDisks()
	if len(views) != len(snapshot.PhysicalDisks) {
		t.Fatalf("canonical physical disks = %d, want %d (one per node and device)", len(views), len(snapshot.PhysicalDisks))
	}
	seenSlots := make(map[string]bool, len(views))
	seenMetricIDs := make(map[string]bool, len(views))
	for _, view := range views {
		seenSlots[view.Node()+":"+view.DevPath()] = true
		seenMetricIDs[view.MetricResourceID()] = true
	}
	if len(seenSlots) != len(snapshot.PhysicalDisks) {
		t.Fatalf("canonical disks cover %d node/device slots, want %d: %v", len(seenSlots), len(snapshot.PhysicalDisks), seenSlots)
	}
	if len(seenMetricIDs) != len(snapshot.PhysicalDisks) {
		t.Fatalf("canonical disks resolve %d metric keys, want one per disk: %v", len(seenMetricIDs), seenMetricIDs)
	}
	writerIDs := make(map[string]bool, len(snapshot.PhysicalDisks))
	for _, disk := range snapshot.PhysicalDisks {
		writerIDs[unifiedresources.PhysicalDiskMetricID(disk)] = true
	}
	if len(writerIDs) != len(snapshot.PhysicalDisks) {
		t.Fatalf("SMART metric series = %d, want one per disk: %v", len(writerIDs), writerIDs)
	}
	for writerID := range writerIDs {
		if !seenMetricIDs[writerID] {
			t.Fatalf("SMART writer key %q is not a key readers resolve: %v", writerID, seenMetricIDs)
		}
	}

	// The host agent inside each nested node reads the same ATA default
	// through smartctl.
	agentIDs := make(map[string]bool, 3)
	for _, host := range []string{"agent-pve1", "agent-pve2", "agent-pve3"} {
		agentIDs[unifiedresources.HostSMARTDiskSourceID(models.Host{ID: host}, models.HostDiskSMART{Device: "sdb", Serial: "QM00005"})] = true
	}
	if len(agentIDs) != 3 {
		t.Fatalf("host agent disk source IDs = %v, want one per host", agentIDs)
	}
}

// A TrueNAS VM's virtual disks report QEMU's default serial, the same on
// every appliance built the same way, and other appliances report "UNKNOWN".
// Neither may merge disks across appliances, and the SMART writer must file
// history under the key the chart reader resolves, the disk's source ID,
// rather than under the canonical resource ID.
func TestTrueNASPlaceholderDiskSerialsStayPerApplianceAndShareOneHistoryKey(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("metrics.NewStore() error = %v", err)
	}
	defer func() { _ = store.Close() }()

	collectedAt := time.Now().UTC().Truncate(time.Second)
	var records []unifiedresources.IngestRecord
	diskCount := 0
	for _, hostname := range []string{"truenas-a", "truenas-b"} {
		fixtures := truenas.DefaultFixtures()
		fixtures.CollectedAt = collectedAt
		fixtures.System.CollectedAt = collectedAt
		fixtures.System.Hostname = hostname
		fixtures.System.MachineID = hostname + "-machine-id"
		for i := range fixtures.Disks {
			fixtures.Disks[i].Serial = "drive-scsi" + strconv.Itoa(i)
		}
		fixtures.Disks[0].Serial = "UNKNOWN"
		diskCount += len(fixtures.Disks)
		records = append(records, truenas.NewProvider(fixtures).Records()...)
	}
	resourceStore := unifiedresources.NewMonitorAdapter(nil)
	resourceStore.PopulateSnapshotAndSupplemental(models.StateSnapshot{}, map[unifiedresources.DataSource][]unifiedresources.IngestRecord{
		unifiedresources.SourceTrueNAS: records,
	})
	monitor := &Monitor{resourceStore: resourceStore, metricsStore: store}
	monitor.syncUnifiedPhysicalDiskMetrics(resourceStore)

	disks := 0
	keys := make(map[string]string, diskCount)
	for _, resource := range resourceStore.GetAll() {
		if resource.Type != unifiedresources.ResourceTypePhysicalDisk || resource.PhysicalDisk == nil {
			continue
		}
		disks++
		target := resourceStore.MetricsTargetForResource(resource.ID)
		if target == nil || target.ResourceType != "disk" || target.ResourceID == "" {
			t.Fatalf("disk %s metrics target = %+v", resource.ID, target)
		}
		if other, shared := keys[target.ResourceID]; shared {
			t.Fatalf("disks %s and %s share metric key %q", other, resource.ID, target.ResourceID)
		}
		keys[target.ResourceID] = resource.ID
		if resource.PhysicalDisk.Temperature <= 0 {
			continue
		}
		points, err := store.Query("disk", target.ResourceID, "smart_temp", collectedAt.Add(-time.Minute), time.Now().Add(time.Minute), 0)
		if err != nil {
			t.Fatalf("store.Query(%q) error = %v", target.ResourceID, err)
		}
		if len(points) == 0 {
			t.Fatalf("disk %s (serial %q): no SMART history under the reader's key %q", resource.ID, resource.PhysicalDisk.Serial, target.ResourceID)
		}
	}
	if disks != diskCount {
		t.Fatalf("canonical TrueNAS disks = %d, want %d (one per appliance and device)", disks, diskCount)
	}
}

// The agent SMART merge marks each Proxmox disk a reporting linked agent
// lists, so the disk's temperature alert is left to the agent's CheckHost.
// Disks the agent does not list, disks on nodes without an agent, and disks
// of an agent whose lease lapsed stay unmarked. Disks the poller builds from
// the agent's SMART report when the Proxmox query fails go through the same
// merge.
func TestMergeHostAgentSMARTIntoDisksMarksDisksTheAgentReports(t *testing.T) {
	disks := []models.PhysicalDisk{
		{Instance: "pve1", Node: "node1", DevPath: "/dev/sda"},
		{Instance: "pve1", Node: "node1", DevPath: "/dev/sdb"},
		{Instance: "pve1", Node: "node2", DevPath: "/dev/sda"},
		{Instance: "pve1", Node: "node3", DevPath: "/dev/sda"},
	}
	nodes := []models.Node{
		{Name: "node1", LinkedAgentID: "host-node1"},
		{Name: "node2"},
		{Name: "node3", LinkedAgentID: "host-node3"},
	}
	smart := []models.HostDiskSMART{{Device: "sda", Serial: "SER-SDA", Type: "sata", Temperature: 40}}
	hosts := []models.Host{
		{ID: "host-node1", Status: "online", Sensors: models.HostSensorSummary{SMART: smart}},
		{ID: "host-node3", Status: "offline", Sensors: models.HostSensorSummary{SMART: smart}},
	}

	merged := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts, nil)
	for i, want := range []bool{true, false, false, false} {
		if merged[i].AgentSMARTReported != want {
			t.Fatalf("%s/%s AgentSMARTReported = %v, want %v", merged[i].Node, merged[i].DevPath, merged[i].AgentSMARTReported, want)
		}
	}
	if disks[0].AgentSMARTReported {
		t.Fatalf("merge modified the input slice")
	}
	if merged[3].Serial != "SER-SDA" {
		t.Fatalf("a silent agent's retained row no longer enriches the disk: %+v", merged[3])
	}

	fallback := mergeHostAgentSMARTIntoDisks(physicalDisksFromHostAgentSMART("pve1", "node1", smart), nodes, hosts, nil)
	if len(fallback) != 1 || !fallback[0].AgentSMARTReported {
		t.Fatalf("a disk built from a reporting agent's SMART report is not the agent's: %+v", fallback)
	}
}

// checkPhysicalDiskAlerts raises a Proxmox disk's temperature alert from a
// reading collected this poll, under the disk temperature policy, and leaves
// alone a retained reading, a disk the linked agent reports and an excluded
// device.
func TestCheckPhysicalDiskAlertsRaisesProxmoxDiskTemperatureAlerts(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	m := &Monitor{alertManager: manager}
	const instance, node = "pve1", "node1"
	alertID := func(devPath string) string {
		return unifiedresources.ProxmoxPhysicalDiskAlertResourceID(instance, node, devPath) + "::metric-threshold:diskTemperature"
	}
	active := func(devPath string) bool {
		for _, alert := range manager.GetActiveAlerts() {
			if alert.ID == alertID(devPath) {
				return true
			}
		}
		return false
	}
	// 70C is critical for SATA (trigger 55, critical 65), which fires without
	// the stability delay a warning waits out.
	sata := func(devPath string) models.PhysicalDisk {
		return models.PhysicalDisk{Instance: instance, Node: node, DevPath: devPath, Model: "Test SATA", Type: "sata", Health: "PASSED", Wearout: -1, Temperature: 70}
	}

	m.checkPhysicalDiskAlerts(instance, sata("/dev/sda"), nil)
	if !active("/dev/sda") {
		t.Fatalf("a SATA disk at 70C raised no temperature alert: %+v", manager.GetActiveAlerts())
	}

	retained := sata("/dev/sdb")
	retained.Collection = &diskinventory.CollectionStatus{Temperature: diskinventory.Unavailable("proxmox_node_smart", "disk is in standby")}
	m.checkPhysicalDiskAlerts(instance, retained, nil)
	if active("/dev/sdb") {
		t.Fatalf("a retained 70C reading raised a temperature alert")
	}

	agentOwned := sata("/dev/sdc")
	agentOwned.AgentSMARTReported = true
	m.checkPhysicalDiskAlerts(instance, agentOwned, nil)
	if active("/dev/sdc") {
		t.Fatalf("a disk the linked agent reports raised a second temperature alert")
	}

	m.checkPhysicalDiskAlerts(instance, sata("/dev/sdd"), []string{"sdd"})
	if active("/dev/sdd") {
		t.Fatalf("an excluded device raised a temperature alert")
	}
}

// An operator split of a Proxmox disk from the disk its linked agent reports
// (report-merge on the merged disk, the drawer's Split merged resource) holds
// for the PVE disk poller too. The poller paired the agent's SMART row with the
// Proxmox disk by WWN, serial or path before the registry saw either, and read
// no exclusions, so the registry kept two rows while the Proxmox record still
// carried the agent's attributes, I/O, health, serial, type and temperature,
// and the agent kept the disk's temperature alert (AgentSMARTReported). A split
// row carries what Proxmox reported, whatever an earlier poll merged into it,
// on a full poll and on the skipped polls between. That includes the
// temperature: a node with a linked agent takes its sensor lists from the
// agent, so the reading the node poll matches to the disk is the agent's row
// of the disk it was split from, and the disk neither shows it nor alerts on it
// a second time beside the agent's own alert.
func TestOperatorSplitKeepsAgentSMARTOffTheProxmoxDisk(t *testing.T) {
	percentUsed, hours := 40, int64(1200)
	agentSMART := func(device, model, serial, diskType string, temperature int) models.HostDiskSMART {
		return models.HostDiskSMART{
			Device: device, Model: model, Serial: serial, Type: diskType, Health: "FAILED", Temperature: temperature,
			IO:         &models.DiskIO{Device: device, ReadBytes: 4096, WriteBytes: 8192},
			Attributes: &models.SMARTAttributes{PowerOnHours: &hours, PercentageUsed: &percentUsed},
			Collection: &diskinventory.CollectionStatus{
				Serial:      diskinventory.Available("smartctl"),
				Temperature: diskinventory.Available("smartctl"),
				IO:          diskinventory.Available("kernel_diskstats"),
			},
		}
	}
	for _, shape := range []struct {
		name  string
		agent models.HostDiskSMART
		pve   proxmox.Disk
		// nodeTemp is the node sensor list's row for the disk. The node poll
		// takes that list from the linked agent, so it is the agent's reading.
		nodeTemp *models.DiskTemp
		// What a paired record carries: the agent's serial and type, and the
		// temperature it reports unless the node's sensors read the disk.
		pairedSerial, pairedType string
		pairedTemperature        int
		// The temperature a skipped poll reads back from the joined canonical
		// disk, and what the split disk shows once the agent's reading is gone.
		skippedTemperature, splitTemperature int
		// Whether the PVE disk check alerts on the split disk's reading.
		splitAlert bool
	}{
		// Proxmox reports the SAS transport address as the serial (#1595), so
		// only the device path pairs the rows, and the pairing promotes the
		// agent's serial and transport over Proxmox's.
		{
			name:  "sas-path",
			agent: agentSMART("/dev/sda", "ST4000NM0023", "Z1Z0ABCD", "sas", 70),
			pve: proxmox.Disk{DevPath: "/dev/sda", Model: "ST4000NM0023", Serial: "5000c500a1b2c3d4", Type: "hdd",
				Health: "PASSED", Wearout: 100, Size: 4000787030016},
			pairedSerial: "Z1Z0ABCD", pairedType: "sas", pairedTemperature: 70, skippedTemperature: 70,
		},
		// One serial on both sides, and the node's sensor list reads the disk,
		// so it carries a reading and can raise a temperature alert.
		{
			name:  "matching-serial",
			agent: agentSMART("/dev/nvme0n1", "WD Black SN850X", "S6B0NL0W123456", "nvme", 40),
			pve: proxmox.Disk{DevPath: "/dev/nvme0n1", Model: "WD Black SN850X", Serial: "S6B0NL0W123456", Type: "nvme",
				Health: "PASSED", Wearout: 100, Size: 1000204886016},
			nodeTemp:     &models.DiskTemp{Device: "/dev/nvme0n1", Serial: "S6B0NL0W123456", Type: "nvme", Temperature: 85},
			pairedSerial: "S6B0NL0W123456", pairedType: "nvme", pairedTemperature: 85, skippedTemperature: 40,
		},
		// The agent's report carries no usable temperature, so the node's SMART
		// list is the SSH collector's: the reading is the node's own, survives the
		// split, and the PVE disk check, no longer deferring to the agent, alerts.
		{
			name:  "ssh-sensors",
			agent: agentSMART("/dev/nvme1n1", "WD Black SN770", "S7SSH0000001", "nvme", 0),
			pve: proxmox.Disk{DevPath: "/dev/nvme1n1", Model: "WD Black SN770", Serial: "S7SSH0000001", Type: "nvme",
				Health: "PASSED", Wearout: 100, Size: 1000204886016},
			nodeTemp:     &models.DiskTemp{Device: "/dev/nvme1n1", Serial: "S7SSH0000001", Type: "nvme", Temperature: 85},
			pairedSerial: "S7SSH0000001", pairedType: "nvme", pairedTemperature: 85, skippedTemperature: 85,
			splitTemperature: 85, splitAlert: true,
		},
	} {
		for _, request := range []struct {
			name string
			// exclusions are the pairs report-merge records for the merged disk.
			exclusions func(merged, agentCandidate, proxmoxCandidate string) [][2]string
			split      bool
		}{
			{"report-merge", func(merged, agent, proxmox string) [][2]string {
				return [][2]string{{merged, agent}, {merged, proxmox}}
			}, true},
			{"proxmox-candidate-only", func(merged, _, proxmox string) [][2]string {
				return [][2]string{{merged, proxmox}}
			}, true},
			// The agent's row holds the merged ID, so a pair naming only its
			// candidate separates nothing, here as in the registry.
			{"agent-candidate-only", func(merged, agent, _ string) [][2]string {
				return [][2]string{{merged, agent}}
			}, false},
			{"unrelated-exclusion", func(merged, _, _ string) [][2]string {
				return [][2]string{{merged, "physical_disk-00000000deadbeef"}}
			}, false},
		} {
			t.Run(shape.name+"/"+request.name, func(t *testing.T) {
				t.Setenv("PULSE_DATA_DIR", t.TempDir())
				now := time.Now()
				const instance, nodeName = "pve1", "node1"
				node := models.Node{
					ID: "pve1-node1", Name: nodeName, Instance: instance, Host: "https://10.0.0.5:8006",
					Status: "online", LastSeen: now, LinkedAgentID: "agent-1",
				}
				if shape.nodeTemp != nil {
					reading := *shape.nodeTemp
					reading.LastUpdated = now
					node.Temperature = &models.Temperature{Available: true, HasSMART: true, SMART: []models.DiskTemp{reading}, LastUpdate: now}
				}
				state := models.NewState()
				state.UpdateNodesForInstance(instance, []models.Node{node})
				state.UpsertHost(models.Host{
					ID: "agent-1", Hostname: nodeName, MachineID: "0123456789abcdef", LinkedNodeID: node.ID,
					Status: "online", IntervalSeconds: 30, LastSeen: now,
					Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{shape.agent}},
				})
				store, err := unifiedresources.NewSQLiteResourceStore(t.TempDir(), "default")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = store.Close() })
				alertManager := alerts.NewManager()
				t.Cleanup(alertManager.Stop)
				adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store))
				m := &Monitor{
					state: state, resourceStore: adapter, alertManager: alertManager,
					metricsHistory: NewMetricsHistory(100, time.Hour),
					startTime:      now.Add(-time.Hour), lastPhysicalDiskPoll: make(map[string]time.Time),
				}
				client := &slotDiskPVEClient{}
				client.setDisk(shape.pve)

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
					delete(m.lastPhysicalDiskPoll, instance)
					m.maybePollPhysicalDisksAsync(context.Background(), instance, &config.PVEInstance{}, client,
						[]proxmox.Node{{Node: nodeName, Status: "online"}}, map[string]string{nodeName: "online"}, nil)
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
					m.lastPhysicalDiskPoll[instance] = time.Now()
					m.maybePollPhysicalDisksAsync(context.Background(), instance, &config.PVEInstance{}, nil, nil, nil, nil)
					return disk()
				}
				temperatureAlert := func() bool {
					id := unifiedresources.ProxmoxPhysicalDiskAlertResourceID(instance, nodeName, shape.pve.DevPath) + "::metric-threshold:diskTemperature"
					for _, alert := range alertManager.GetActiveAlerts() {
						if alert.ID == id {
							return true
						}
					}
					return false
				}
				healthAlert := func() bool {
					for _, alert := range alertManager.GetActiveAlerts() {
						if alert.Type == "disk-health" {
							return true
						}
					}
					return false
				}
				// A poll whose Proxmox disk query fails carries the stored record
				// (or the agent's rows) instead of a fresh one, so it is done when
				// the state is written rather than when a disk is newly checked.
				failedQueryPoll := func() models.PhysicalDisk {
					t.Helper()
					adapter.PopulateFromSnapshot(state.GetSnapshot())
					client.setError(errors.New("disks/list timed out"))
					defer client.setError(nil)
					started := time.Now()
					delete(m.lastPhysicalDiskPoll, instance)
					m.maybePollPhysicalDisksAsync(context.Background(), instance, &config.PVEInstance{}, client,
						[]proxmox.Node{{Node: nodeName, Status: "online"}}, map[string]string{nodeName: "online"}, nil)
					deadline := time.Now().Add(3 * time.Second)
					for {
						if snapshot := state.GetSnapshot(); snapshot.LastUpdate.After(started) && len(snapshot.PhysicalDisks) == 1 {
							return snapshot.PhysicalDisks[0]
						}
						if time.Now().After(deadline) {
							t.Fatal("physical disk poll with a failing disk query did not land in state")
						}
						time.Sleep(10 * time.Millisecond)
					}
				}
				// Whether the canonical disks are one joined row or two.
				canonicalRows := func() (joined, agentOnly, proxmoxOnly []*unifiedresources.PhysicalDiskView) {
					t.Helper()
					adapter.PopulateFromSnapshot(state.GetSnapshot())
					for _, view := range adapter.PhysicalDisks() {
						_, fromAgent := view.SourceStatus(unifiedresources.SourceAgent)
						_, fromProxmox := view.SourceStatus(unifiedresources.SourceProxmox)
						switch {
						case fromAgent && fromProxmox:
							joined = append(joined, view)
						case fromAgent:
							agentOnly = append(agentOnly, view)
						case fromProxmox:
							proxmoxOnly = append(proxmoxOnly, view)
						}
					}
					return joined, agentOnly, proxmoxOnly
				}

				assertPaired := func(step string, got models.PhysicalDisk, wantTemperature int) {
					t.Helper()
					if got.Serial != shape.pairedSerial || got.Type != shape.pairedType || got.Health != "FAILED" ||
						got.Temperature != wantTemperature || got.Wearout != 60 ||
						got.SmartAttributes == nil || got.IO == nil || !got.AgentSMARTReported || got.AgentSMARTSplit {
						t.Fatalf("%s: the agent's row was not paired with the Proxmox disk: %+v", step, got)
					}
				}
				assertProxmoxOnly := func(step string, got models.PhysicalDisk) {
					t.Helper()
					if got.Serial != shape.pve.Serial || got.Type != shape.pve.Type || got.Health != "PASSED" ||
						got.Temperature != shape.splitTemperature || got.Wearout != shape.pve.Wearout ||
						got.SmartAttributes != nil || got.IO != nil || got.AgentSMARTReported || !got.AgentSMARTSplit {
						t.Fatalf("%s: the split disk carries the agent's data: %+v", step, got)
					}
				}

				// The disk's temperature alert is open before the agent's row
				// is paired with it, as when the node's sensors read it alone.
				// A shape whose alert should open after the split starts without
				// one: the alert manager suppresses an alert it just resolved.
				if shape.nodeTemp != nil && !shape.splitAlert {
					m.checkPhysicalDiskAlerts(instance, models.PhysicalDisk{
						Instance: instance, Node: nodeName, DevPath: shape.pve.DevPath, Model: shape.pve.Model,
						Type: shape.pve.Type, Health: "PASSED", Wearout: -1, Temperature: shape.nodeTemp.Temperature,
					}, nil)
					if !temperatureAlert() {
						t.Fatal("fixture did not open the Proxmox disk's temperature alert")
					}
				}
				assertPaired("before the request", fullPoll(), shape.pairedTemperature)
				if !healthAlert() {
					t.Fatal("the agent's failed health did not raise the disk health alert")
				}
				if joined, agentOnly, proxmoxOnly := canonicalRows(); len(joined) != 1 || len(agentOnly) != 0 || len(proxmoxOnly) != 0 {
					t.Fatalf("fixture did not join the disks: joined %d, agent only %d, proxmox only %d", len(joined), len(agentOnly), len(proxmoxOnly))
				}
				if temperatureAlert() {
					t.Fatal("the PVE disk's temperature alert stayed open for a disk the agent reports")
				}

				// Report-merge names the merged disk and each source's candidate.
				ids := unifiedresources.NewRegistry(nil)
				ids.IngestSnapshot(state.GetSnapshot())
				var merged, agentCandidate, proxmoxCandidate string
				for _, resource := range ids.List() {
					if resource.Type == unifiedresources.ResourceTypePhysicalDisk {
						merged = resource.ID
					}
				}
				for _, target := range ids.SourceTargets(merged) {
					switch target.Source {
					case unifiedresources.SourceAgent:
						agentCandidate = target.CandidateID
					case unifiedresources.SourceProxmox:
						proxmoxCandidate = target.CandidateID
					}
				}
				if merged == "" || agentCandidate == "" || proxmoxCandidate == "" || proxmoxCandidate == merged {
					t.Fatalf("merged disk %q lists candidates agent=%q proxmox=%q", merged, agentCandidate, proxmoxCandidate)
				}
				for _, pair := range request.exclusions(merged, agentCandidate, proxmoxCandidate) {
					if err := store.AddExclusion(unifiedresources.ResourceExclusion{ResourceA: pair[0], ResourceB: pair[1], CreatedAt: time.Now().UTC()}); err != nil {
						t.Fatal(err)
					}
				}

				if !request.split {
					assertPaired("after the request", fullPoll(), shape.pairedTemperature)
					// A skipped poll merges the stored record, which the joined
					// canonical disk reports at the agent's reading.
					assertPaired("on a skipped poll", skippedPoll(), shape.skippedTemperature)
					return
				}
				// A full poll builds the record from what Proxmox reported,
				// and the poll after it must not retain what the one before
				// held from the agent. The skipped polls between, which merge
				// the stored record again, keep the agent's data off too.
				assertProxmoxOnly("after the request", fullPoll())
				// The split disk is still evaluated, and Proxmox's own verdict
				// of it closes the health alert the agent's failure raised.
				if healthAlert() {
					t.Fatal("the disk health alert raised by the agent's row stayed open on the split disk")
				}
				assertProxmoxOnly("on a skipped poll", skippedPoll())
				assertProxmoxOnly("on the next full poll", fullPoll())
				assertProxmoxOnly("on the next skipped poll", skippedPoll())
				// When Proxmox cannot list the node's disks, the agent's rows
				// stand in for them (#1516), except the row split from the
				// disk in its slot.
				assertProxmoxOnly("when the disk query fails", failedQueryPoll())

				joined, agentOnly, proxmoxOnly := canonicalRows()
				if len(joined) != 0 || len(agentOnly) != 1 || len(proxmoxOnly) != 1 {
					t.Fatalf("canonical disks: joined %d, agent only %d, proxmox only %d, want the two apart", len(joined), len(agentOnly), len(proxmoxOnly))
				}
				if proxmoxOnly[0].Health() != "PASSED" || proxmoxOnly[0].Serial() != shape.pve.Serial {
					t.Fatalf("canonical Proxmox disk took the agent's data: health %q, serial %q", proxmoxOnly[0].Health(), proxmoxOnly[0].Serial())
				}
				if agentOnly[0].Health() != "FAILED" || agentOnly[0].Serial() != shape.agent.Serial {
					t.Fatalf("canonical agent disk lost its own data: health %q, serial %q", agentOnly[0].Health(), agentOnly[0].Serial())
				}
				// A reading the agent supplied is gone, so the PVE disk check has
				// none to judge and the alert the agent took over does not come
				// back; one the SSH collector supplied is judged by the PVE check.
				if temperatureAlert() != shape.splitAlert {
					t.Fatalf("PVE temperature alert active = %v, want %v once the disks were split", temperatureAlert(), shape.splitAlert)
				}
			})
		}
	}
}

// fakeDiskAgentSplits answers the PVE disk poller's split question from a
// set of Proxmox device paths and records what it was asked.
type fakeDiskAgentSplits struct {
	splitDevPaths map[string]bool
	splitIDs      map[string]bool
	asked         []string
	askedIDs      []string
}

func (f *fakeDiskAgentSplits) ProxmoxDiskAgentSMARTSplit(disk models.PhysicalDisk, host models.Host, smart models.HostDiskSMART) bool {
	f.asked = append(f.asked, host.ID+":"+disk.DevPath+":"+smart.Serial)
	f.askedIDs = append(f.askedIDs, disk.ID)
	return f.splitDevPaths[disk.DevPath] || f.splitIDs[disk.ID]
}

// The agent's SMART merge asks the split decider once per disk it matches a
// row to. A split disk takes nothing from the row, not even the alert
// ownership, and is marked so the poll does not retain the agent's data for it
// either; a disk no row matches is not asked about; the rows of the disks that
// are not split still merge, as they do without a decider.
func TestMergeHostAgentSMARTIntoDisksLeavesDisksTheOperatorSplitAlone(t *testing.T) {
	percentUsed := 40
	smart := func(device, serial string) models.HostDiskSMART {
		return models.HostDiskSMART{
			Device: device, Serial: serial, Type: "sata", Health: "FAILED", Temperature: 61,
			WWN: "5-c50-" + serial, Controller: "ctrl", Target: "0:1", Pool: "tank", Model: "AgentModel", SizeBytes: 1000,
			IO:         &models.DiskIO{Device: device, ReadBytes: 1},
			Attributes: &models.SMARTAttributes{PercentageUsed: &percentUsed},
		}
	}
	disk := func(devPath, serial string) models.PhysicalDisk {
		return models.PhysicalDisk{ID: unifiedresources.ProxmoxPhysicalDiskSourceID("pve1", "node1", devPath, "", ""), Node: "node1", Instance: "pve1", DevPath: devPath, Serial: serial, Type: "sata", Health: "PASSED", Wearout: 100}
	}
	nodes := []models.Node{{Name: "node1", LinkedAgentID: "agent-1"}}
	hosts := []models.Host{{ID: "agent-1", Status: "online", Sensors: models.HostSensorSummary{
		SMART: []models.HostDiskSMART{smart("sda", "SERIAL-A"), smart("sdb", "SERIAL-B")},
	}}}
	disks := []models.PhysicalDisk{disk("/dev/sda", "SERIAL-A"), disk("/dev/sdb", "SERIAL-B"), disk("/dev/sdc", "SERIAL-C")}
	// The node's sensor list, which a linked agent supplies, was matched to the
	// split disk before the agent merge ran.
	disks[0].Temperature = 58
	disks[0].Collection = &diskinventory.CollectionStatus{Temperature: diskinventory.Available(proxmoxNodeSMARTTemperatureSource)}
	// Proxmox's own verdict of the split disk, which the agent's PASSED/FAILED
	// would otherwise replace or correct.
	disks[0].Health, disks[0].Wearout = "FAILED", 30

	splits := &fakeDiskAgentSplits{splitDevPaths: map[string]bool{"/dev/sda": true, "/dev/sdc": true}}
	merged := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts, splits)

	wantSplit := disk("/dev/sda", "SERIAL-A")
	wantSplit.AgentSMARTSplit = true
	if got := merged[0]; got.Temperature != 0 || got.Health != "FAILED" || got.Wearout != 30 || got.SmartAttributes != nil || got.IO != nil ||
		got.AgentSMARTReported || !got.AgentSMARTSplit || got.ID != wantSplit.ID || got.Serial != wantSplit.Serial ||
		got.WWN != "" || got.Controller != "" || got.Target != "" || got.StorageGroup != "" || got.Model != "" || got.Size != 0 {
		t.Fatalf("a split disk took the agent's row: %+v", got)
	}
	if got := merged[1]; got.Temperature != 61 || got.Health != "FAILED" || got.Wearout != 60 || got.SmartAttributes == nil || got.IO == nil ||
		!got.AgentSMARTReported || got.AgentSMARTSplit {
		t.Fatalf("a disk that is not split lost the agent's row: %+v", got)
	}
	if got := merged[2]; got.AgentSMARTSplit || got.AgentSMARTReported {
		t.Fatalf("a disk no row matches was marked: %+v", got)
	}
	if want := []string{"agent-1:/dev/sda:SERIAL-A", "agent-1:/dev/sdb:SERIAL-B"}; !slices.Equal(splits.asked, want) {
		t.Fatalf("decider asked %v, want one question per matched disk %v", splits.asked, want)
	}
	if disks[0].AgentSMARTSplit || disks[0].Temperature != 58 || disks[0].Collection.Temperature.State != diskinventory.FieldAvailable {
		t.Fatalf("the merge modified the caller's slice: %+v / %+v", disks[0], disks[0].Collection)
	}
	if merged[0].Collection == nil || merged[0].Collection.Temperature.State == diskinventory.FieldAvailable {
		t.Fatalf("the split disk still counts the node sensors' reading as collected: %+v", merged[0].Collection)
	}

	// The decision is made under the observation's own ID, the key the registry
	// holds it by: a controller member recorded by device alone is a different
	// observation from the stand-in keyed by its target, so a split recorded
	// under the other key is ignored, as it is for the registry.
	memberRow := smart("sdd", "SERIAL-D")
	memberRow.Controller, memberRow.Target = "megaraid", "megaraid,7"
	memberHosts := []models.Host{{ID: "agent-1", Status: "online", Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{memberRow}}}}
	slotID := unifiedresources.ProxmoxPhysicalDiskSourceID("pve1", "node1", "/dev/sdd", "", "")
	memberID := unifiedresources.ProxmoxPhysicalDiskSourceID("pve1", "node1", "/dev/sdd", "megaraid", "megaraid,7")
	if slotID == memberID {
		t.Fatal("fixture: a controller member's key should differ from the slot's")
	}
	bySlot := disk("/dev/sdd", "SERIAL-D")
	byMember := bySlot
	byMember.ID = memberID
	for name, tc := range map[string]struct {
		disk      models.PhysicalDisk
		splitID   string
		wantSplit bool
		wantAsked string
	}{
		"slot-keyed record, split under its key":    {bySlot, slotID, true, slotID},
		"slot-keyed record, split under member key": {bySlot, memberID, false, slotID},
		"member-keyed record, split under its key":  {byMember, memberID, true, memberID},
		"member-keyed record, split under slot key": {byMember, slotID, false, memberID},
	} {
		decider := &fakeDiskAgentSplits{splitIDs: map[string]bool{tc.splitID: true}}
		got := mergeHostAgentSMARTIntoDisks([]models.PhysicalDisk{tc.disk}, nodes, memberHosts, decider)[0]
		if got.AgentSMARTSplit != tc.wantSplit || got.AgentSMARTReported == tc.wantSplit {
			t.Fatalf("%s: split = %v, reported = %v, want split %v", name, got.AgentSMARTSplit, got.AgentSMARTReported, tc.wantSplit)
		}
		if !slices.Equal(decider.askedIDs, []string{tc.wantAsked}) {
			t.Fatalf("%s: decider asked about %v, want exactly the record's own ID %q", name, decider.askedIDs, tc.wantAsked)
		}
	}

	// The node-sensor reading already on the disk stays; the agent's fills only an empty one.
	if unchanged := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts, &fakeDiskAgentSplits{}); !unchanged[0].AgentSMARTReported || unchanged[0].Temperature != 58 || unchanged[0].Health != "FAILED" {
		t.Fatalf("a decider that splits nothing changed the merge: %+v", unchanged[0])
	}
}

// When the Proxmox disk query fails, the linked agent's SMART rows stand in
// for the node's disks (#1516). A row the operator split from the Proxmox disk
// in its slot is not that disk's stand-in: it would put the agent's disk, with
// its readings and temperature alert, in a second row beside the agent's own.
// The stand-in is judged under its own ID, the key the registry holds it by
// once recorded: a controller member's is qualified by its target.
func TestHostAgentSMARTFallbackSkipsRowsTheOperatorSplit(t *testing.T) {
	rows := []models.HostDiskSMART{
		{Device: "sda", Serial: "SERIAL-A", Type: "sata", Temperature: 61},
		{Device: "sdb", Serial: "SERIAL-B", Type: "sata", Temperature: 62},
	}
	host := models.Host{ID: "agent-1"}

	if got := unsplitPhysicalDisksFromHostAgentSMART("pve1", "node1", host, rows, nil); len(got) != 2 {
		t.Fatalf("without split decisions the fallback listed %d disks, want 2", len(got))
	}
	if got := unsplitPhysicalDisksFromHostAgentSMART("pve1", "node1", host, rows, &fakeDiskAgentSplits{}); len(got) != 2 {
		t.Fatalf("a decider that splits nothing left %d stand-ins, want 2", len(got))
	}
	splits := &fakeDiskAgentSplits{splitDevPaths: map[string]bool{"/dev/sda": true}}
	got := unsplitPhysicalDisksFromHostAgentSMART("pve1", "node1", host, rows, splits)
	if len(got) != 1 || got[0].DevPath != "/dev/sdb" || got[0].Temperature != 62 {
		t.Fatalf("fallback disks = %+v, want only /dev/sdb", got)
	}
	if want := []string{"agent-1:/dev/sda:SERIAL-A", "agent-1:/dev/sdb:SERIAL-B"}; !slices.Equal(splits.asked, want) {
		t.Fatalf("decider asked %v, want one question per row, with the row %v", splits.asked, want)
	}

	member := []models.HostDiskSMART{{Device: "sdc", Serial: "SERIAL-C", Type: "sat", Controller: "megaraid", Target: "megaraid,7"}}
	stand := physicalDisksFromHostAgentSMART("pve1", "node1", member)
	slot := unifiedresources.ProxmoxPhysicalDiskSourceID("pve1", "node1", "/dev/sdc", "", "")
	if len(stand) != 1 || stand[0].ID == slot {
		t.Fatalf("a controller member's stand-in should be keyed by its target: %+v", stand)
	}
	byOwnID := &fakeDiskAgentSplits{splitIDs: map[string]bool{stand[0].ID: true}}
	if got := unsplitPhysicalDisksFromHostAgentSMART("pve1", "node1", host, member, byOwnID); len(got) != 0 {
		t.Fatalf("a split recorded under the stand-in's key did not hold: %+v", got)
	}
	if want := []string{stand[0].ID}; !slices.Equal(byOwnID.askedIDs, want) {
		t.Fatalf("decider asked about %v, want the stand-in's own ID %v", byOwnID.askedIDs, want)
	}
	// A split recorded under the slot's device-only key names a different
	// observation, as for the registry: the stand-in is not that disk.
	bySlot := &fakeDiskAgentSplits{splitIDs: map[string]bool{slot: true}}
	if got := unsplitPhysicalDisksFromHostAgentSMART("pve1", "node1", host, member, bySlot); len(got) != 1 {
		t.Fatalf("a split recorded under the slot's key applied to the member's stand-in: %+v", got)
	}
	if want := []string{stand[0].ID}; !slices.Equal(bySlot.askedIDs, want) {
		t.Fatalf("decider asked about %v, want only the stand-in's own ID %v", bySlot.askedIDs, want)
	}
}

// When the Proxmox disk query fails and the operator split one of the node's
// disks from its agent row, only the rows that were not split stand in for the
// node's disks, as before; the split disk gets no stand-in, which would be the
// agent's disk again, so its Proxmox record is not listed while the query fails.
func TestFailedDiskQueryListsOnlyTheRowsTheOperatorDidNotSplit(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	now := time.Now()
	state := models.NewState()
	state.UpdateNodesForInstance("pve1", []models.Node{{
		ID: "pve1-node1", Name: "node1", Instance: "pve1", Status: "online", LastSeen: now, LinkedAgentID: "agent-1",
	}})
	state.UpsertHost(models.Host{
		ID: "agent-1", Hostname: "node1", LinkedNodeID: "pve1-node1", Status: "online", IntervalSeconds: 30, LastSeen: now,
		Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{
			{Device: "/dev/sda", Serial: "AGENT-A", Type: "sata", Health: "FAILED", Temperature: 61},
			{Device: "/dev/sdb", Serial: "SERIAL-B", Type: "sata", Health: "PASSED", Temperature: 62},
		}},
	})
	record := func(device, serial string) models.PhysicalDisk {
		return models.PhysicalDisk{
			ID: unifiedresources.ProxmoxPhysicalDiskSourceID("pve1", "node1", device, "", ""), Node: "node1", Instance: "pve1",
			DevPath: device, Serial: serial, Type: "sata", Health: "PASSED", Wearout: 100, LastChecked: now,
			Collection: &diskinventory.CollectionStatus{Temperature: diskinventory.Unsupported("proxmox_disks", "no temperature")},
		}
	}
	state.UpdatePhysicalDisks("pve1", []models.PhysicalDisk{record("/dev/sda", "PVE-A"), record("/dev/sdb", "SERIAL-B")})

	alertManager := alerts.NewManager()
	t.Cleanup(alertManager.Stop)
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	m := &Monitor{
		state:         state,
		resourceStore: splitDecidingStore{adapter, &fakeDiskAgentSplits{splitDevPaths: map[string]bool{"/dev/sda": true}}},
		alertManager:  alertManager, metricsHistory: NewMetricsHistory(100, time.Hour),
		startTime: now.Add(-time.Hour), lastPhysicalDiskPoll: make(map[string]time.Time),
	}
	adapter.PopulateFromSnapshot(state.GetSnapshot())
	client := &slotDiskPVEClient{}
	client.setError(errors.New("disks/list timed out"))
	started := time.Now()
	m.maybePollPhysicalDisksAsync(context.Background(), "pve1", &config.PVEInstance{}, client,
		[]proxmox.Node{{Node: "node1", Status: "online"}}, map[string]string{"node1": "online"}, nil)
	var disks []models.PhysicalDisk
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if snapshot := state.GetSnapshot(); snapshot.LastUpdate.After(started) {
			disks = snapshot.PhysicalDisks
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("physical disk poll with a failing disk query did not land in state")
		}
	}
	byDevice := map[string]models.PhysicalDisk{}
	for _, disk := range disks {
		byDevice[disk.DevPath] = disk
	}
	if len(disks) != 1 {
		t.Fatalf("disks = %+v, want only the row that was not split", disks)
	}
	if split, listed := byDevice["/dev/sda"]; listed {
		t.Fatalf("the agent's row stood in for the disk the operator split from it: %+v", split)
	}
	if other := byDevice["/dev/sdb"]; other.Serial != "SERIAL-B" || other.Temperature != 62 || !other.AgentSMARTReported ||
		other.ID != unifiedresources.ProxmoxPhysicalDiskSourceID("pve1", "node1", "/dev/sdb", "", "") {
		t.Fatalf("the other row did not stand in for its disk: %+v", other)
	}
}

// splitDecidingStore is the resource store a monitor test hands out when it
// wants to decide the operator's splits itself.
type splitDecidingStore struct {
	*unifiedresources.MonitorAdapter
	splits *fakeDiskAgentSplits
}

func (s splitDecidingStore) ProxmoxDiskAgentSMARTSplit(disk models.PhysicalDisk, host models.Host, smart models.HostDiskSMART) bool {
	return s.splits.ProxmoxDiskAgentSMARTSplit(disk, host, smart)
}

// The node poll takes the node's SMART sensor list from a linked agent when
// the agent's report carries a usable temperature, and from the SSH collector
// otherwise, so only the first kind of reading is the agent's row of the disk
// a split Proxmox disk was split from. The legacy NVMe list names no disk, so
// a split disk drops its guess either way. A disk whose temperature has
// another source keeps it, and the caller's collection is never modified.
func TestDropNodeSensorTemperatureRemovesOnlyAgentSuppliedReadings(t *testing.T) {
	for _, tc := range []struct {
		source         string
		smartFromAgent bool
		dropped        bool
	}{
		{proxmoxNodeSMARTTemperatureSource, true, true},
		{proxmoxNodeSMARTTemperatureSource, false, false},
		{proxmoxNodeNVMeTemperatureSource, true, true},
		{proxmoxNodeNVMeTemperatureSource, false, true},
	} {
		shared := &diskinventory.CollectionStatus{
			Temperature: diskinventory.Available(tc.source), Serial: diskinventory.Available("proxmox_disks"),
			IO: diskinventory.Unsupported("proxmox_disks", "no counters"), Controller: diskinventory.Missing("proxmox_disks", "none"),
			Pool: diskinventory.Available("proxmox_zfs"),
		}
		disk := models.PhysicalDisk{Temperature: 60, Collection: shared}
		dropNodeSensorTemperature(&disk, tc.smartFromAgent)
		if tc.dropped {
			other := *disk.Collection
			other.Temperature = shared.Temperature
			if disk.Temperature != 0 || disk.Collection.Temperature.State != diskinventory.FieldUnsupported ||
				disk.Collection.Temperature.Source != "proxmox_disks" || other != *shared {
				t.Fatalf("%+v: reading not dropped cleanly: %+v / %+v", tc, disk, disk.Collection)
			}
		} else if disk.Temperature != 60 || disk.Collection != shared {
			t.Fatalf("%+v: an SSH-supplied reading was dropped: %+v", tc, disk)
		}
		if shared.Temperature.State != diskinventory.FieldAvailable || shared.Temperature.Source != tc.source {
			t.Fatalf("%+v: the caller's collection was modified: %+v", tc, shared)
		}
	}
	for _, collection := range []*diskinventory.CollectionStatus{
		nil,
		{Temperature: diskinventory.Available("smartctl")},
		{Temperature: diskinventory.Unsupported("proxmox_disks", "no temperature")},
	} {
		disk := models.PhysicalDisk{Temperature: 60, Collection: collection}
		dropNodeSensorTemperature(&disk, true)
		if disk.Temperature != 60 || disk.Collection != collection {
			t.Fatalf("a reading from another source was dropped: %+v", disk)
		}
	}
}

// mergeNVMeTempsIntoDisks labels what it takes from the node's sensor lists
// with the sources dropNodeSensorTemperature recognises: the SMART list
// matched by identity or path, and the legacy NVMe list matched by order.
func TestMergeNVMeTempsLabelsReadingsWithTheNodeSensorSources(t *testing.T) {
	node := func(temperature *models.Temperature) []models.Node {
		return []models.Node{{Name: "node1", Temperature: temperature}}
	}
	disk := models.PhysicalDisk{Node: "node1", DevPath: "/dev/nvme0n1", Serial: "S6B0NL0W123456", Type: "nvme"}

	bySMART := mergeNVMeTempsIntoDisks([]models.PhysicalDisk{disk}, node(&models.Temperature{
		Available: true, HasSMART: true,
		SMART: []models.DiskTemp{{Device: "/dev/nvme0n1", Serial: "S6B0NL0W123456", Temperature: 55}},
	}))[0]
	if bySMART.Temperature != 55 || bySMART.Collection == nil || bySMART.Collection.Temperature.Source != proxmoxNodeSMARTTemperatureSource {
		t.Fatalf("SMART list reading not labelled %q: %+v / %+v", proxmoxNodeSMARTTemperatureSource, bySMART, bySMART.Collection)
	}

	for name, row := range map[string]models.DiskTemp{
		"WWN":         {Device: "/dev/other", WWN: "0x5000c500a1b2c3d4", Temperature: 52},
		"device path": {Device: "/dev/nvme0n1", Temperature: 53},
	} {
		wwnDisk := models.PhysicalDisk{Node: "node1", DevPath: "/dev/nvme0n1", WWN: "0x5000c500a1b2c3d4", Type: "nvme"}
		if name == "device path" {
			wwnDisk = models.PhysicalDisk{Node: "node1", DevPath: "/dev/nvme0n1", Type: "nvme"}
		}
		got := mergeNVMeTempsIntoDisks([]models.PhysicalDisk{wwnDisk}, node(&models.Temperature{
			Available: true, HasSMART: true, SMART: []models.DiskTemp{row},
		}))[0]
		if got.Temperature != row.Temperature || got.Collection == nil || got.Collection.Temperature.Source != proxmoxNodeSMARTTemperatureSource {
			t.Fatalf("%s match not labelled %q: %+v / %+v", name, proxmoxNodeSMARTTemperatureSource, got, got.Collection)
		}
	}

	byOrder := mergeNVMeTempsIntoDisks([]models.PhysicalDisk{disk}, node(&models.Temperature{
		Available: true, HasNVMe: true, NVMe: []models.NVMeTemp{{Device: "nvme0", Temp: 47}},
	}))[0]
	if byOrder.Temperature != 47 || byOrder.Collection == nil || byOrder.Collection.Temperature.Source != proxmoxNodeNVMeTemperatureSource {
		t.Fatalf("legacy NVMe reading not labelled %q: %+v / %+v", proxmoxNodeNVMeTemperatureSource, byOrder, byOrder.Collection)
	}
}

// Only a SMART report that carries a usable temperature makes the node's SMART
// list the agent's (mergeTemperatureData).
func TestHostSuppliesNodeSMARTTemperaturesFollowsUsableReadings(t *testing.T) {
	host := func(rows ...models.HostDiskSMART) models.Host {
		return models.Host{Sensors: models.HostSensorSummary{SMART: rows}}
	}
	for name, tc := range map[string]struct {
		host models.Host
		want bool
	}{
		"a reading":              {host(models.HostDiskSMART{Device: "sda", Temperature: 40}), true},
		"one reading of several": {host(models.HostDiskSMART{Device: "sda"}, models.HostDiskSMART{Device: "sdb", Temperature: 40}), true},
		"no temperature":         {host(models.HostDiskSMART{Device: "sda"}), false},
		"standby row":            {host(models.HostDiskSMART{Device: "sda", Temperature: 40, Standby: true}), false},
		"negative reading":       {host(models.HostDiskSMART{Device: "sda", Temperature: -1}), false},
		"no rows":                {host(), false},
	} {
		if got := hostSuppliesNodeSMARTTemperatures(tc.host); got != tc.want {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)
		}
	}
}

// When the linked agent's SMART report carries no usable temperature, the
// node's SMART sensor list comes from the SSH collector, so a reading of a
// split Proxmox disk taken from it is the node's own and stays, while the
// agent's row still takes nothing else from the disk.
func TestMergeHostAgentSMARTIntoDisksKeepsSSHNodeSensorReadingOfSplitDisk(t *testing.T) {
	nodes := []models.Node{{Name: "node1", LinkedAgentID: "agent-1"}}
	hosts := []models.Host{{ID: "agent-1", Status: "online", Sensors: models.HostSensorSummary{
		SMART: []models.HostDiskSMART{{Device: "sda", Serial: "SERIAL-A", Type: "sata", Health: "FAILED"}},
	}}}
	disk := models.PhysicalDisk{
		ID: "pve1-node1-/dev/sda", Node: "node1", Instance: "pve1", DevPath: "/dev/sda", Serial: "SERIAL-A", Type: "sata",
		Health: "PASSED", Wearout: 100, Temperature: 52,
		Collection: &diskinventory.CollectionStatus{Temperature: diskinventory.Available(proxmoxNodeSMARTTemperatureSource)},
	}
	splits := &fakeDiskAgentSplits{splitDevPaths: map[string]bool{"/dev/sda": true}}
	got := mergeHostAgentSMARTIntoDisks([]models.PhysicalDisk{disk}, nodes, hosts, splits)[0]
	if got.Temperature != 52 || got.Collection.Temperature.Source != proxmoxNodeSMARTTemperatureSource ||
		got.Health != "PASSED" || got.AgentSMARTReported || !got.AgentSMARTSplit {
		t.Fatalf("split disk with an SSH-supplied reading: %+v / %+v", got, got.Collection)
	}

	// An agent that stopped reporting keeps its rows, temperatures included,
	// but the node poll ignores them once its lease lapsed, so the reading is
	// the SSH collector's again.
	hosts[0].Status = "offline"
	hosts[0].Sensors.SMART[0].Temperature = 61
	lapsed := mergeHostAgentSMARTIntoDisks([]models.PhysicalDisk{disk}, nodes, hosts, splits)[0]
	if lapsed.Temperature != 52 || lapsed.Collection.Temperature.Source != proxmoxNodeSMARTTemperatureSource || !lapsed.AgentSMARTSplit {
		t.Fatalf("split disk with a lapsed agent dropped the node's own reading: %+v / %+v", lapsed, lapsed.Collection)
	}
	hosts[0].Status = "online"
	live := mergeHostAgentSMARTIntoDisks([]models.PhysicalDisk{disk}, nodes, hosts, splits)[0]
	if live.Temperature != 0 || live.Collection.Temperature.State != diskinventory.FieldUnsupported {
		t.Fatalf("split disk with a reporting agent kept the agent-supplied reading: %+v / %+v", live, live.Collection)
	}
}
