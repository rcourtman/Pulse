package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func hostFilesystemManager(t *testing.T, grace bool) (*Manager, *atomic.Int64) {
	t.Helper()
	m, elapsed := continuityManager(t, grace)
	m.mu.Lock()
	m.config.AgentDefaults = ThresholdConfig{Disk: &HysteresisThreshold{Trigger: 80, Clear: 70}}
	m.config.NodeDefaults = ThresholdConfig{Disk: &HysteresisThreshold{Trigger: 80, Clear: 70}}
	m.config.DiskFillByType = nil
	if !grace {
		m.config.MetricTimeThresholds = map[string]map[string]int{"all": {"disk": 0}}
	}
	m.mu.Unlock()
	return m, elapsed
}

func hostFilesystemSample(id string, usage float64) models.Host {
	return models.Host{ID: id, Hostname: "storage-host", DisplayName: "Storage host", Status: "online",
		Disks: []models.Disk{{Mountpoint: "/srv", Device: "/dev/sda1", Total: 1000, Used: int64(usage * 10), Usage: usage}}}
}

func hostFilesystemGap(host models.Host, gap string) models.Host {
	disk := host.Disks[0]
	switch gap {
	case "empty":
		host.Disks = nil
		return host
	case "unknown-zero":
		disk.Total, disk.Used, disk.Usage = 0, 0, 0
	case "unknown-positive":
		disk.Total = 0
	case "negative-capacity":
		disk.Total = -1
	case "negative-used":
		disk.Used = -1
	case "over-capacity":
		disk.Used = disk.Total + 1
	case "negative":
		disk.Usage = -1
	case "over-100":
		disk.Usage = 101
	case "NaN":
		disk.Usage = math.NaN()
	case "+Inf":
		disk.Usage = math.Inf(1)
	}
	host.Disks = []models.Disk{disk}
	return host
}

func hostFilesystemID(host models.Host) string {
	id, _ := hostDiskResourceID(host, host.Disks[0])
	return id
}

func hostFilesystemJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestHostFilesystemUnknownPreservesOccurrence(t *testing.T) {
	for _, gap := range []string{"empty", "unknown-zero", "unknown-positive", "negative-capacity", "negative-used", "over-capacity", "negative", "over-100", "NaN", "+Inf"} {
		t.Run(gap, func(t *testing.T) {
			m, elapsed := hostFilesystemManager(t, false)
			host := hostFilesystemSample("host-1", 95)
			m.CheckHost(host)
			if len(m.GetActiveAlerts()) != 1 {
				t.Fatal("trusted full disk must open an alert")
			}
			id := m.GetActiveAlerts()[0].ID
			if err := m.AcknowledgeAlert(id, "operator"); err != nil {
				t.Fatal(err)
			}
			before := hostFilesystemJSON(t, m.GetActiveAlerts())
			events := len(queryAlertEvents(t, m, eventlog.Filter{}))
			elapsed.Store(int64(2 * time.Minute))
			for range 3 {
				m.CheckHost(hostFilesystemGap(host, gap))
			}
			if !bytes.Equal(before, hostFilesystemJSON(t, m.GetActiveAlerts())) || len(queryAlertEvents(t, m, eventlog.Filter{})) != events {
				t.Fatal("unavailable filesystem changed the occurrence, trusted evidence, acknowledgement or lifecycle ledger")
			}
			// Observed zero, not a missing reading, must still recover the original.
			host.Disks[0].Used, host.Disks[0].Usage = 0, 0
			m.CheckHost(host)
			if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{AlertID: id, Types: []string{eventlog.TypeResolved}})) != 1 {
				t.Fatal("an observed empty filesystem must resolve the original exactly once")
			}
		})
	}
}

func TestHostFilesystemGapRestartsTiming(t *testing.T) {
	for _, grace := range []bool{false, true} {
		for _, recovery := range []bool{false, true} {
			for _, gap := range []string{"empty", "unknown-zero", "unknown-positive", "negative-used", "over-capacity"} {
				t.Run(fmt.Sprintf("intent=%v/recovery=%v/%s", grace, recovery, gap), func(t *testing.T) {
					m, elapsed := hostFilesystemManager(t, grace)
					m.mu.Lock()
					m.config.MetricTimeThresholds = map[string]map[string]int{"all": {"disk": 60}}
					m.mu.Unlock()
					host := hostFilesystemSample("host-1", 85)
					id := hostFilesystemID(host)
					if recovery {
						host.Disks[0].Used, host.Disks[0].Usage = 950, 95
						m.CheckHost(host)
						elapsed.Store(int64(time.Minute))
						m.CheckHost(host)
						if len(m.GetActiveAlerts()) != 1 {
							t.Fatal("critical control must fire")
						}
						host.Disks[0].Used, host.Disks[0].Usage = 100, 10
						elapsed.Store(int64(70 * time.Second))
					}
					m.CheckHost(host)
					incident, exists := continuityIncident(m, id, "disk")
					if !exists || (!recovery && incident.State != reducer.StatePending) || (recovery && incident.RecoverySince.IsZero()) {
						t.Fatal("control must start the real timed run")
					}
					// A similarly prefixed host must not be interrupted with this host.
					other := hostFilesystemSample("host-10", 85)
					m.CheckHost(other)
					elapsed.Store(int64(3 * time.Minute))
					m.CheckHost(hostFilesystemGap(host, gap))
					incident, exists = continuityIncident(m, id, "disk")
					if (!recovery && exists) || (recovery && (!exists || !incident.RecoverySince.IsZero())) {
						t.Fatal("observation gap retained timed evidence")
					}
					if _, exists := continuityIncident(m, hostFilesystemID(other), "disk"); !exists {
						t.Fatal("gap crossed the exact host boundary")
					}
					m.CheckHost(host)
					want := 0
					if recovery {
						want = 1
					}
					if len(m.GetActiveAlerts()) != want {
						t.Fatal("first fresh sample completed the pre-gap run")
					}
					elapsed.Store(int64(3*time.Minute + 59*time.Second))
					m.CheckHost(host)
					if len(m.GetActiveAlerts()) != want {
						t.Fatal("fresh run completed early")
					}
					elapsed.Store(int64(4 * time.Minute))
					m.CheckHost(host)
					if recovery {
						want = 0
					} else {
						want = 1
					}
					if len(m.GetActiveAlerts()) != want {
						t.Fatal("full continuous fresh run did not complete")
					}
					if m.ShadowDivergences() != 0 {
						t.Fatal("interruption diverged from the shadow reducer")
					}
				})
			}
		}
	}
}

func TestHostFilesystemGapHonoursPolicyAndRemoval(t *testing.T) {
	for _, gap := range []string{"empty", "unknown-zero"} {
		for _, policy := range []string{"host-off", "disk-off", "disk-disabled", "inherited-off", "agent-removed", "filesystem-removed"} {
			t.Run(gap+"/"+policy, func(t *testing.T) {
				m, _ := hostFilesystemManager(t, false)
				host := hostFilesystemSample("host-1", 95)
				host.LinkedNodeID = "node-1"
				id := hostFilesystemID(host)
				m.CheckHost(host)
				if len(m.GetActiveAlerts()) != 1 {
					t.Fatal("control must fire")
				}
				host = hostFilesystemGap(host, gap)
				// Change config directly: this proves CheckHost, not config-save cleanup.
				m.mu.Lock()
				switch policy {
				case "host-off":
					m.config.AgentDefaults.Disk = nil
				case "disk-off":
					m.config.Overrides[id] = ThresholdConfig{Disk: &HysteresisThreshold{}}
				case "disk-disabled":
					m.config.Overrides[id] = ThresholdConfig{Disabled: true}
				case "inherited-off":
					m.config.Overrides[host.LinkedNodeID] = ThresholdConfig{Disk: &HysteresisThreshold{}}
				}
				m.mu.Unlock()
				switch policy {
				case "agent-removed":
					m.HandleHostRemoved(host)
				case "filesystem-removed":
					host.Disks = hostFilesystemSample(host.ID, 0).Disks
					host.Disks[0].Mountpoint = "/other"
					m.CheckHost(host)
				default:
					m.CheckHost(host)
				}
				if len(m.GetActiveAlerts()) != 0 {
					t.Fatal("unknown telemetry overrode explicit policy or authoritative removal")
				}
			})
		}
	}
}

func TestHostFilesystemGapKeepsNodeOwnershipOnlyForHeldAlert(t *testing.T) {
	for _, gap := range []string{"empty", "unknown-zero"} {
		for _, firing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/firing=%v", gap, firing), func(t *testing.T) {
				m, elapsed := hostFilesystemManager(t, !firing)
				host := hostFilesystemSample("host-1", 85)
				host.LinkedNodeID = "node-1"
				if firing {
					host.Disks[0].Used, host.Disks[0].Usage = 950, 95
				}
				m.CheckHost(host)
				m.CheckHost(hostFilesystemGap(host, gap))
				m.mu.RLock()
				owner := m.hostAgentNodeLinks[host.ID]
				m.mu.RUnlock()
				if owner.disk != firing {
					t.Fatalf("gap disk ownership=%v, want %v", owner.disk, firing)
				}
				node := models.Node{ID: "node-1", Name: "storage-host", Status: "online", Disk: models.Disk{Total: 1000, Used: 950, Usage: 95}}
				m.CheckNode(node)
				elapsed.Store(int64(time.Minute))
				m.CheckNode(node)
				if len(m.GetActiveAlerts()) != 1 {
					t.Fatal("node handover left duplicate alerts or no alert")
				}
			})
		}
	}
}

// Real manager payloads are optionally exported for the production UI consumer.
func TestHostFilesystemBrowserProducer(t *testing.T) {
	m, elapsed := hostFilesystemManager(t, false)
	host := hostFilesystemSample("host-browser", 95)
	frames := map[string]json.RawMessage{}
	for _, phase := range []string{"breach", "unknown-zero", "empty", "recovered"} {
		sample := host
		if phase == "recovered" {
			sample.Disks = hostFilesystemSample(host.ID, 0).Disks
		} else if phase != "breach" {
			sample = hostFilesystemGap(host, phase)
		}
		elapsed.Add(int64(time.Second))
		m.CheckHost(sample)
		active := m.GetActiveAlerts()
		want := 1
		if phase == "recovered" {
			want = 0
		}
		if len(active) != want {
			t.Fatalf("phase %s: %d active, want %d", phase, len(active), want)
		}
		frames[phase] = hostFilesystemJSON(t, active)
	}
	if !bytes.Equal(frames["breach"], frames["unknown-zero"]) || !bytes.Equal(frames["breach"], frames["empty"]) {
		t.Fatal("unknown telemetry changed the exported occurrence")
	}
	if os.Getenv("PULSE_HOST_FILESYSTEM_FRAMES") == "1" {
		fmt.Printf("HOST_FILESYSTEM_FRAMES=%s\n", hostFilesystemJSON(t, frames))
	}
}

func TestHostFilesystemUnobservedNeverFires(t *testing.T) {
	for _, gap := range []string{"empty", "unknown-zero", "unknown-positive", "negative-capacity", "negative-used", "over-capacity", "negative", "over-100", "NaN", "+Inf"} {
		t.Run(gap, func(t *testing.T) {
			m, elapsed := hostFilesystemManager(t, false)
			host := hostFilesystemGap(hostFilesystemSample("host-1", 95), gap)
			m.CheckHost(host)
			elapsed.Store(int64(time.Hour))
			m.CheckHost(host)
			if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{})) != 0 {
				t.Fatal("an unavailable filesystem invented an alert or transition")
			}
		})
	}
}

func TestHostFilesystemReplacementRetiresPendingRun(t *testing.T) {
	m, elapsed := hostFilesystemManager(t, true)
	host := hostFilesystemSample("host-1", 85)
	id := hostFilesystemID(host)
	m.CheckHost(host)
	if _, exists := continuityIncident(m, id, "disk"); !exists {
		t.Fatal("control must start a pending run")
	}
	replacement := hostFilesystemSample(host.ID, 0)
	replacement.Disks[0].Mountpoint = "/other"
	elapsed.Store(int64(2 * time.Minute))
	m.CheckHost(replacement)
	if _, exists := continuityIncident(m, id, "disk"); exists {
		t.Fatal("removed filesystem retained timed evidence")
	}
	m.CheckHost(host)
	if len(m.GetActiveAlerts()) != 0 {
		t.Fatal("reappearing filesystem fired using retired evidence")
	}
	elapsed.Store(int64(3 * time.Minute))
	m.CheckHost(host)
	if len(m.GetActiveAlerts()) != 1 {
		t.Fatal("fresh full run did not fire")
	}
}
