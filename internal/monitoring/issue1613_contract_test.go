package monitoring

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestIssue1613GuestRateIdentitySurvivesMigrationAndIsolatesDuplicates(t *testing.T) {
	beforeMigration := makeGuestRateKey("site-a", "qemu", 100)
	afterMigration := makeGuestRateKey("site-a", "qemu", 100)
	if beforeMigration != afterMigration {
		t.Fatalf("migration changed rate key: %q != %q", beforeMigration, afterMigration)
	}
	if beforeMigration == makeGuestRateKey("site-b", "qemu", 100) {
		t.Fatal("duplicate cluster registrations shared a rate key")
	}
	if beforeMigration == makeGuestRateKey("site-a", "lxc", 100) {
		t.Fatal("QEMU and LXC with the same VMID shared a rate key")
	}
}

func TestIssue1613QEMUStatusUsesPresentLowerCounterAndKeepsMissingListingCounter(t *testing.T) {
	listingAt := time.Unix(1_700_000_000, 0)
	statusAt := listingAt.Add(3 * time.Second)
	state := vmBuildState{
		diskReadBytes:     9_000,
		diskWriteBytes:    8_000,
		networkInBytes:    7_000,
		networkOutBytes:   6_000,
		counterPresence:   models.IOCounterPresence{Explicit: true, DiskRead: true, DiskWrite: true, NetworkIn: true, NetworkOut: true},
		counterObservedAt: listingAt,
		counterObservationTimes: models.IOCounterObservationTimes{
			DiskRead:   listingAt,
			DiskWrite:  listingAt,
			NetworkIn:  listingAt,
			NetworkOut: listingAt,
		},
		counterUptime: 10_000,
	}
	mergeVMRuntimeCounters(&state, &proxmox.VMStatus{
		DiskRead: 0,
		NetOut:   500,
		Uptime:   20,
		IOCounters: proxmox.IOCounterPresence{
			Explicit:   true,
			DiskRead:   true,
			NetworkOut: true,
		},
		ObservedAt: statusAt,
	})

	if state.diskReadBytes != 0 || state.networkOutBytes != 500 {
		t.Fatalf("present status counters did not replace listing values: %+v", state)
	}
	if state.diskWriteBytes != 8_000 || state.networkInBytes != 7_000 {
		t.Fatalf("missing status counters replaced listing values: %+v", state)
	}
	if !state.counterObservationTimes.DiskRead.Equal(statusAt) ||
		!state.counterObservationTimes.NetworkOut.Equal(statusAt) ||
		!state.counterObservationTimes.DiskWrite.Equal(listingAt) ||
		!state.counterObservationTimes.NetworkIn.Equal(listingAt) {
		t.Fatalf("per-counter receipt authority was not preserved: %+v", state.counterObservationTimes)
	}
	if state.counterUptime != 20 {
		t.Fatalf("status uptime = %d, want 20", state.counterUptime)
	}
}

func TestIssue1613PVEGraceTracksConfiguredPollingCadence(t *testing.T) {
	for _, test := range []struct {
		interval time.Duration
		want     time.Duration
	}{
		{interval: 30 * time.Second, want: 60 * time.Second},
		{interval: 60 * time.Second, want: 120 * time.Second},
		{interval: 90 * time.Second, want: 180 * time.Second},
	} {
		monitor := &Monitor{config: &config.Config{PVEPollingInterval: test.interval}}
		if got := monitor.pveNodeOfflineGracePeriod(); got != test.want {
			t.Fatalf("interval %s grace = %s, want %s", test.interval, got, test.want)
		}
	}
}

func TestIssue1613HistoryWritesValidZeroAndSkipsUnknownRates(t *testing.T) {
	monitor := &Monitor{metricsHistory: NewMetricsHistory(32, time.Hour)}
	now := time.Now()

	diskRead, diskWrite, networkIn, networkOut := guestHistoryRates(0, 0, 0, 0, models.IORateValidity{Explicit: true})
	monitor.recordGuestMetric("vm", "unknown", 0, 0, 0, diskRead, diskWrite, networkIn, networkOut, now)
	if points := monitor.metricsHistory.GetGuestMetrics("unknown", "diskread", time.Hour); len(points) != 0 {
		t.Fatalf("unknown disk read rate was written to history: %+v", points)
	}

	diskRead, diskWrite, networkIn, networkOut = guestHistoryRates(0, 0, 0, 0, models.IORateValidity{
		Explicit:   true,
		DiskRead:   true,
		DiskWrite:  true,
		NetworkIn:  true,
		NetworkOut: true,
	})
	monitor.recordGuestMetric("vm", "idle", 0, 0, 0, diskRead, diskWrite, networkIn, networkOut, now)
	points := monitor.metricsHistory.GetGuestMetrics("idle", "diskread", time.Hour)
	if len(points) != 1 || points[0].Value != 0 {
		t.Fatalf("valid idle disk read rate was not written as zero: %+v", points)
	}
}

func TestIssue1613NodeDoesNotGreyBetweenNinetySecondPolls(t *testing.T) {
	instance := &config.PVEInstance{Name: "site-a"}
	monitor := &Monitor{
		config:         &config.Config{PVEPollingInterval: 90 * time.Second},
		nodeLastOnline: map[string]time.Time{"site-a-pve-a": time.Now().Add(-100 * time.Second)},
	}

	_, status := monitor.determineNodeIDAndStatus("site-a", instance, proxmox.Node{
		Node:   "pve-a",
		Status: "offline",
	})
	if status != "online" {
		t.Fatalf("node status = %q inside cadence grace, want online", status)
	}

	monitor.nodeLastOnline["site-a-pve-a"] = time.Now().Add(-181 * time.Second)
	_, status = monitor.determineNodeIDAndStatus("site-a", instance, proxmox.Node{
		Node:   "pve-a",
		Status: "offline",
	})
	if status != "offline" {
		t.Fatalf("node status = %q after cadence grace, want offline", status)
	}
}

func TestIssue1613WebsocketStateKeepsUnknownRatesNumeric(t *testing.T) {
	monitor := &Monitor{
		state: models.NewState(),
		resourceStore: &resourceOnlyStore{resources: []unifiedresources.Resource{
			{
				ID:      "site-a:pve-a:100",
				Type:    unifiedresources.ResourceTypeVM,
				Name:    "idle-vm",
				Status:  unifiedresources.StatusOnline,
				Sources: []unifiedresources.DataSource{unifiedresources.SourceProxmox},
				Metrics: &unifiedresources.ResourceMetrics{
					DiskRead: &unifiedresources.MetricValue{Value: 0, Unit: "bytes/s"},
				},
			},
			{
				ID:      "site-a:pve-a:101",
				Type:    unifiedresources.ResourceTypeVM,
				Name:    "unknown-vm",
				Status:  unifiedresources.StatusOnline,
				Sources: []unifiedresources.DataSource{unifiedresources.SourceProxmox},
				Metrics: &unifiedresources.ResourceMetrics{},
			},
		}},
	}
	frontend := monitor.BuildBroadcastFrontendState()
	payload, err := json.Marshal(frontend)
	if err != nil {
		t.Fatal(err)
	}
	wire := string(payload)
	if !strings.Contains(wire, `"diskIO":{"readRate":0,"writeRate":0}`) {
		t.Fatalf("websocket payload does not contain numeric valid zero disk rate: %s", wire)
	}
	if strings.Count(wire, `"diskIO"`) != 1 {
		t.Fatalf("unknown rate projected a disk I/O object: %s", wire)
	}
	if strings.Contains(wire, `"diskWrite":null`) || strings.Contains(wire, `"netIn":null`) {
		t.Fatalf("websocket payload contains unstable null I/O values: %s", wire)
	}
}
