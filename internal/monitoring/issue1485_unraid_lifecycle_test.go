package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func newIssue1485Monitor(t *testing.T, dataDir string) *Monitor {
	t.Helper()
	manager := alerts.NewManagerWithDataDir(dataDir)
	monitor := &Monitor{
		config:              &config.Config{DataPath: dataDir},
		state:               models.NewState(),
		alertManager:        manager,
		hostContinuityStore: config.NewHostContinuityStore(dataDir, nil),
		rateTracker:         NewRateTracker(),
		metricsHistory:      NewMetricsHistory(100, time.Hour),
		hostTokenBindings:   make(map[string]string),
		hostReportOrders:    make(map[string]hostReportOrder),
		removedHostAgents:   make(map[string]time.Time),
		clusterSensorsCache: make(map[string]clusterSensorsCacheEntry),
		resourceStore:       unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)),
		guestMetadataStore:  config.NewGuestMetadataStore(dataDir, nil),
		dockerMetadataStore: config.NewDockerMetadataStore(dataDir, nil),
		hostMetadataStore:   config.NewHostMetadataStore(dataDir, nil),
		removedDockerHosts:  make(map[string]time.Time),
		dockerTokenBindings: make(map[string]string),
	}
	t.Cleanup(manager.Stop)
	return monitor
}

func issue1485Report(observedAt time.Time, streamID string, sequence uint64, action string, progress float64) agentshost.Report {
	return agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "unraid-agent",
			Version:         "6.1.1",
			Type:            "unified",
			IntervalSeconds: 10,
		},
		Host: agentshost.HostInfo{
			ID:            "unraid-machine",
			Hostname:      "tower",
			DisplayName:   "Tower",
			MachineID:     "unraid-machine",
			Platform:      "unraid",
			OSName:        "Unraid",
			UptimeSeconds: 3600,
		},
		Metrics: agentshost.Metrics{
			CPUUsagePercent: float64(sequence),
			Memory: agentshost.MemoryMetric{
				TotalBytes: 16 << 30,
				UsedBytes:  4 << 30,
				Usage:      25,
			},
		},
		Unraid: &agentshost.UnraidStorage{
			ArrayStarted: true,
			ArrayState:   "STARTED",
			SyncAction:   action,
			SyncProgress: progress,
			NumProtected: 1,
			Disks: []agentshost.UnraidDisk{
				{Name: "parity", Role: "parity", Status: "online", Device: "/dev/sda"},
				{Name: "disk1", Role: "data", Status: "online", Device: "/dev/sdb"},
			},
		},
		Timestamp:  observedAt.UTC(),
		SequenceID: agentshost.FormatReportSequenceID(streamID, sequence),
	}
}

func issue1485HasAlertType(manager *alerts.Manager, alertType string) bool {
	for _, alert := range manager.GetActiveAlerts() {
		if alert.Type == alertType {
			return true
		}
	}
	return false
}

func issue1485UnraidResource(t *testing.T, monitor *Monitor) unifiedresources.Resource {
	t.Helper()
	for _, resource := range monitor.resourceStore.GetAll() {
		if resource.Type == unifiedresources.ResourceTypeStorage &&
			resource.Technology == "unraid" &&
			resource.Storage != nil &&
			resource.Storage.Type == "unraid-array" {
			return resource
		}
	}
	t.Fatal("Unraid array resource not found")
	return unifiedresources.Resource{}
}

func assertIssue1485Idle(t *testing.T, monitor *Monitor, host models.Host) {
	t.Helper()
	if host.Unraid == nil {
		t.Fatal("Unraid state missing")
	}
	if host.Unraid.SyncAction != "" || host.Unraid.SyncProgress != 0 {
		t.Fatalf("transient sync state = (%q, %.1f), want idle", host.Unraid.SyncAction, host.Unraid.SyncProgress)
	}
	if issue1485HasAlertType(monitor.alertManager, "storage-topology") {
		t.Fatal("Unraid operation alert remained active")
	}
	resource := issue1485UnraidResource(t, monitor)
	if resource.Storage.SyncAction != "" || resource.Storage.SyncProgress != 0 {
		t.Fatalf("resource sync state = (%q, %.1f), want idle", resource.Storage.SyncAction, resource.Storage.SyncProgress)
	}
	if resource.Storage.Risk != nil {
		for _, reason := range resource.Storage.Risk.Reasons {
			if reason.Code == "unraid_sync_active" {
				t.Fatalf("resource retained active-sync risk: %+v", reason)
			}
		}
	}
}

func TestIssue1485UnraidParityLifecycle(t *testing.T) {
	base := time.Now().UTC().Add(-time.Minute)

	t.Run("cancelled and normally completed checks resolve every projection", func(t *testing.T) {
		for _, terminal := range []struct {
			name     string
			progress float64
		}{
			{name: "cancelled", progress: 0},
			{name: "completed", progress: 100},
		} {
			t.Run(terminal.name, func(t *testing.T) {
				monitor := newIssue1485Monitor(t, t.TempDir())
				running, err := monitor.ApplyHostReport(issue1485Report(base, "stream-a", 1, "check", 40), nil)
				if err != nil {
					t.Fatalf("apply running report: %v", err)
				}
				if running.Unraid == nil || running.Unraid.SyncAction != "check" {
					t.Fatalf("running state = %+v", running.Unraid)
				}
				if !issue1485HasAlertType(monitor.alertManager, "storage-topology") {
					t.Fatal("running parity check did not create an alert")
				}

				terminalReport := issue1485Report(base.Add(time.Second), "stream-a", 2, "", terminal.progress)
				idle, err := monitor.ApplyHostReport(terminalReport, nil)
				if err != nil {
					t.Fatalf("apply terminal report: %v", err)
				}
				assertIssue1485Idle(t, monitor, idle)
			})
		}
	})

	t.Run("stale and retired reports cannot resurrect a cancelled check or append metrics", func(t *testing.T) {
		monitor := newIssue1485Monitor(t, t.TempDir())
		if _, err := monitor.ApplyHostReport(issue1485Report(base, "stream-a", 1, "check", 20), nil); err != nil {
			t.Fatal(err)
		}
		idle, err := monitor.ApplyHostReport(issue1485Report(base.Add(time.Second), "stream-a", 2, "", 0), nil)
		if err != nil {
			t.Fatal(err)
		}
		assertIssue1485Idle(t, monitor, idle)
		metricKey := "agent:" + idle.ID
		before := len(monitor.metricsHistory.GetGuestMetrics(metricKey, "cpu", time.Hour))

		stale, err := monitor.ApplyHostReport(issue1485Report(base, "stream-a", 1, "check", 20), nil)
		if err != nil {
			t.Fatal(err)
		}
		assertIssue1485Idle(t, monitor, stale)
		if !stale.LastSeen.Equal(idle.LastSeen) {
			t.Fatalf("stale report extended accepted telemetry lease: got %v want %v", stale.LastSeen, idle.LastSeen)
		}
		if after := len(monitor.metricsHistory.GetGuestMetrics(metricKey, "cpu", time.Hour)); after != before {
			t.Fatalf("stale report appended metrics: before=%d after=%d", before, after)
		}

		reconnected, err := monitor.ApplyHostReport(issue1485Report(base.Add(-time.Hour), "stream-b", 1, "", 0), nil)
		if err != nil {
			t.Fatal(err)
		}
		assertIssue1485Idle(t, monitor, reconnected)

		retired, err := monitor.ApplyHostReport(issue1485Report(base.Add(2*time.Second), "stream-a", 3, "check", 60), nil)
		if err != nil {
			t.Fatal(err)
		}
		assertIssue1485Idle(t, monitor, retired)
	})

	t.Run("polling gap expires transient operations without hiding telemetry loss", func(t *testing.T) {
		monitor := newIssue1485Monitor(t, t.TempDir())
		running, err := monitor.ApplyHostReport(issue1485Report(base, "stream-a", 1, "check", 30), nil)
		if err != nil {
			t.Fatal(err)
		}
		monitor.evaluateHostAgents(running.LastSeen.Add(hostAgentHealthWindow(running.IntervalSeconds) - time.Second))
		stillRunning, ok := monitor.hostByID(running.ID)
		if !ok {
			t.Fatal("host missing during ordinary polling gap")
		}
		if stillRunning.Unraid == nil || stillRunning.Unraid.SyncAction != "check" {
			t.Fatalf("active check cleared before reporting lease expired: %+v", stillRunning.Unraid)
		}
		if !issue1485HasAlertType(monitor.alertManager, "storage-topology") {
			t.Fatal("active check alert cleared before reporting lease expired")
		}

		expiredAt := running.LastSeen.Add(hostAgentHealthWindow(running.IntervalSeconds) + time.Second)
		monitor.evaluateHostAgents(expiredAt)

		expired, ok := monitor.hostByID(running.ID)
		if !ok {
			t.Fatal("expired host missing")
		}
		if expired.Status != "offline" {
			t.Fatalf("status = %q, want offline", expired.Status)
		}
		assertIssue1485Idle(t, monitor, expired)
		if issue1485HasAlertType(monitor.alertManager, "host-offline") {
			t.Fatal("connectivity alert activated before confirmation window")
		}
		monitor.evaluateHostAgents(expiredAt.Add(time.Second))
		monitor.evaluateHostAgents(expiredAt.Add(2 * time.Second))
		if !issue1485HasAlertType(monitor.alertManager, "host-offline") {
			t.Fatal("telemetry loss was hidden instead of becoming a connectivity alert")
		}
	})

	t.Run("server restart preserves order and expires persisted state", func(t *testing.T) {
		dataDir := t.TempDir()
		first := newIssue1485Monitor(t, dataDir)
		runningBeforeRestart, err := first.ApplyHostReport(issue1485Report(base, "stream-a", 1, "check", 25), nil)
		if err != nil {
			t.Fatal(err)
		}
		if !issue1485HasAlertType(first.alertManager, "storage-topology") {
			t.Fatal("running alert missing before restart")
		}
		first.alertManager.Stop()

		restarted := newIssue1485Monitor(t, dataDir)
		if !issue1485HasAlertType(restarted.alertManager, "storage-topology") {
			t.Fatal("active alert was not restored for restart proof")
		}
		staleBeforeFresh, err := restarted.ApplyHostReport(issue1485Report(base.Add(-time.Second), "stream-a", 1, "check", 10), nil)
		if err != nil {
			t.Fatal(err)
		}
		if !staleBeforeFresh.LastSeen.Equal(runningBeforeRestart.LastSeen) {
			t.Fatalf("stale restart report extended accepted telemetry lease: got %v want %v", staleBeforeFresh.LastSeen, runningBeforeRestart.LastSeen)
		}
		if !issue1485HasAlertType(restarted.alertManager, "storage-topology") {
			t.Fatal("stale restart report prematurely cleared a genuinely active check")
		}
		idle, err := restarted.ApplyHostReport(issue1485Report(base.Add(time.Second), "stream-a", 2, "", 0), nil)
		if err != nil {
			t.Fatal(err)
		}
		assertIssue1485Idle(t, restarted, idle)
		stale, err := restarted.ApplyHostReport(issue1485Report(base, "stream-a", 1, "check", 25), nil)
		if err != nil {
			t.Fatal(err)
		}
		assertIssue1485Idle(t, restarted, stale)

		// A separate restart with no reconnect must wait for the reporting
		// lease, then retire persisted task evidence and raise connectivity.
		gapDir := t.TempDir()
		beforeGap := newIssue1485Monitor(t, gapDir)
		gapRunning, err := beforeGap.ApplyHostReport(issue1485Report(base, "stream-gap", 1, "check", 25), nil)
		if err != nil {
			t.Fatal(err)
		}
		beforeGap.alertManager.Stop()
		afterGap := newIssue1485Monitor(t, gapDir)
		expiredAt := gapRunning.LastSeen.Add(hostAgentHealthWindow(gapRunning.IntervalSeconds) + time.Second)
		afterGap.evaluateHostAgents(expiredAt)
		if issue1485HasAlertType(afterGap.alertManager, "storage-topology") {
			t.Fatal("persisted Unraid alert survived the reporting lease")
		}
		afterGap.evaluateHostAgents(expiredAt.Add(time.Second))
		afterGap.evaluateHostAgents(expiredAt.Add(2 * time.Second))
		if !issue1485HasAlertType(afterGap.alertManager, "host-offline") {
			t.Fatal("restart continuity did not surface telemetry loss")
		}
	})
}

func TestIssue1485LegacyReportOrderingAllowsClockEpochReset(t *testing.T) {
	monitor := newIssue1485Monitor(t, t.TempDir())
	base := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	report := issue1485Report(base, "", 0, "check", 10)
	report.SequenceID = ""
	report.Agent.IntervalSeconds = 30

	order, accepted := monitor.reserveHostReportOrder("legacy-host", report, base.Add(time.Second))
	if !accepted {
		t.Fatal("first legacy report rejected")
	}

	report.Timestamp = base.Add(time.Second)
	order, accepted = monitor.reserveHostReportOrder("legacy-host", report, base.Add(30*time.Second))
	if !accepted {
		t.Fatal("newer legacy report rejected")
	}

	report.Timestamp = base
	if _, accepted = monitor.reserveHostReportOrder("legacy-host", report, base.Add(31*time.Second)); accepted {
		t.Fatal("reconnect-burst legacy report was accepted out of order")
	}

	report.Timestamp = base.Add(-time.Hour)
	order, accepted = monitor.reserveHostReportOrder("legacy-host", report, base.Add(time.Minute))
	if !accepted {
		t.Fatal("legacy clock epoch reset was rejected after a normal report interval")
	}
	if !order.ObservedAt.Equal(report.Timestamp) {
		t.Fatalf("clock-reset watermark = %v, want %v", order.ObservedAt, report.Timestamp)
	}
}

func TestIssue1485RejectedReportCannotRegressReceiptLiveness(t *testing.T) {
	monitor := newIssue1485Monitor(t, t.TempDir())
	base := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)

	newer := issue1485Report(base.Add(2*time.Second), "stream-a", 2, "", 0)
	order, accepted := monitor.reserveHostReportOrder("unraid-host", newer, base.Add(20*time.Second))
	if !accepted {
		t.Fatal("newer report rejected")
	}

	older := issue1485Report(base.Add(time.Second), "stream-a", 1, "check", 20)
	order, accepted = monitor.reserveHostReportOrder("unraid-host", older, base.Add(10*time.Second))
	if accepted {
		t.Fatal("older report accepted")
	}
	if want := base.Add(20 * time.Second); !order.LastReceivedAt.Equal(want) {
		t.Fatalf("receipt liveness regressed to %v, want %v", order.LastReceivedAt, want)
	}
}

func TestIssue1485HostReportApplicationIsSerializedPerHost(t *testing.T) {
	monitor := newIssue1485Monitor(t, t.TempDir())
	unlockFirst := monitor.lockHostReportApplication("unraid-host")

	attempting := make(chan struct{})
	acquired := make(chan struct{})
	released := make(chan struct{})
	go func() {
		close(attempting)
		unlockSecond := monitor.lockHostReportApplication("unraid-host")
		close(acquired)
		unlockSecond()
		close(released)
	}()
	<-attempting

	select {
	case <-acquired:
		t.Fatal("second report application acquired the same-host lock early")
	case <-time.After(50 * time.Millisecond):
	}

	unlockFirst()
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("second report application did not proceed after the first completed")
	}

	monitor.hostReportApplyLocksMu.Lock()
	remaining := len(monitor.hostReportApplyLocks)
	monitor.hostReportApplyLocksMu.Unlock()
	if remaining != 0 {
		t.Fatalf("per-host report locks retained after release: %d", remaining)
	}
}

func TestIssue1485NormalizesTerminalMDRAIDProgress(t *testing.T) {
	monitor := newIssue1485Monitor(t, t.TempDir())
	report := issue1485Report(time.Now().UTC(), "raid-stream", 1, "", 0)
	report.Host.Platform = "linux"
	report.Host.OSName = "Debian"
	report.Unraid = nil
	report.RAID = []agentshost.RAIDArray{{
		Device:         "/dev/md127",
		Level:          "raid1",
		State:          "active",
		TotalDevices:   2,
		ActiveDevices:  2,
		WorkingDevices: 2,
		Operation:      "",
		RebuildPercent: 100,
		RebuildSpeed:   "100M/sec",
	}}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(host.RAID) != 1 {
		t.Fatalf("RAID arrays = %d, want 1", len(host.RAID))
	}
	array := host.RAID[0]
	if array.Operation != "" || array.RebuildPercent != 0 || array.RebuildSpeed != "" {
		t.Fatalf("terminal RAID operation retained transient state: %+v", array)
	}
}
