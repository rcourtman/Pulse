package alerts

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// Include multiple pending metrics for one resource, overlap between core,
// shadow, intent and active state, and identities seen only by one owner.
// Nothing here needs an event store or a running manager worker.
func pendingCollectionFixture(hosts int, mode string) *Manager {
	m := &Manager{
		core:                        reducer.NewState(),
		shadow:                      &shadowFeed{state: reducer.NewState()},
		activeAlerts:                make(map[string]*Alert),
		intentPending:               make(map[string]IntentPendingState),
		hostDiskTempPendingContexts: make(map[string]hostDiskTempContext),
		hostDiskTempAbsences:        make(map[string]int),
		hostNativeSensorPendingCtx:  make(map[string]hostNativeSensorContext),
		config:                      AlertConfig{Overrides: make(map[string]ThresholdConfig)},
		stopping:                    true, // no persistence worker in this in-memory fixture
	}
	at := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	for h := 0; h < hosts; h++ {
		hostID := fmt.Sprintf("host-%04d", h)
		m.hostDiskTempAbsences[hostDiskTemperatureResourceID(hostID, "/dev/absent")] = 1
		for i, resource := range []struct{ id, metric string }{
			{hostDiskTemperatureResourceID(hostID, "/dev/sda"), "diskTemperature"},
			{HostNativeSensorResourceID(hostID, "chip/temp1"), "temperature"},
			{hostResourceID(hostID) + "/disk:root", "disk"},
			{hostResourceID(hostID), "cpu"},
		} {
			pending := mode == "pending" || mode == "mixed" && (h+i)%2 == 0
			key := canonicalMetricSpecID(resource.id, resource.metric)
			for _, state := range m.mirrorStatesNoLock() {
				if pending {
					state.ApplyMetric(reducer.MetricSignal{ResourceID: resource.id, Key: key, Metric: resource.metric, Value: 85, ObservedAt: at},
						reducer.MetricRule{Trigger: 80, DelaySeconds: 3600})
					// A second pending key on the same resource must not make
					// cleanup run twice or create a duplicate collection entry.
					state.ApplyDiscrete(reducer.DiscreteSignal{ResourceID: resource.id, Key: "another-condition", Matched: true, ObservedAt: at},
						reducer.DiscreteRule{Confirmations: 3})
				} else {
					state.SeedFiringIncident(resource.id, key, reducer.SeverityWarning, at, true, "operator", at)
				}
			}
			if pending {
				m.intentPending[resource.id] = IntentPendingState{ResourceID: resource.id, Signal: MetricAlertIntentSignal(resource.metric)}
			} else {
				m.activeAlerts[resource.id] = &Alert{ID: resource.id, ResourceID: resource.id, Type: resource.metric, StartTime: at}
			}
			if resource.metric == "diskTemperature" && pending {
				m.hostDiskTempPendingContexts[resource.id] = hostDiskTempContext{hostID: hostID}
			}
			if resource.metric == "temperature" {
				m.config.Overrides[resource.id] = ThresholdConfig{Temperature: &HysteresisThreshold{Trigger: 80, Clear: 75}}
			}
		}
	}
	return m
}

func TestHostPendingResourceCollectionIsolation(t *testing.T) {
	for _, mode := range []string{"pending", "firing", "mixed"} {
		t.Run(mode, func(t *testing.T) {
			m := pendingCollectionFixture(12, mode)
			hostID := "host-0000"
			want := map[string]struct{}{HostNativeSensorResourceID(hostID, "chip/temp1"): {}}
			// These have no live configuration or core incident. Cleanup
			// must still reach a shadow-only run and an intent-only run.
			shadowOnly := HostNativeSensorResourceID(hostID, "shadow/temp2")
			m.shadow.state.ApplyDiscrete(reducer.DiscreteSignal{ResourceID: shadowOnly, Key: "health", Matched: true}, reducer.DiscreteRule{Confirmations: 2})
			intentOnly := HostNativeSensorResourceID(hostID, "intent/temp3")
			m.intentPending[intentOnly] = IntentPendingState{ResourceID: intentOnly, Signal: MetricAlertIntentSignal("temperature")}
			want[shadowOnly], want[intentOnly] = struct{}{}, struct{}{}
			if got := m.hostNativeSensorTrackedResourcesNoLock(hostID); !reflect.DeepEqual(got, want) {
				t.Fatalf("native sensor resources = %v, want %v", got, want)
			}
			if got := m.hostNativeSensorTrackedResourcesNoLock("host-000"); len(got) != 0 {
				t.Fatalf("namesake prefix collected another host's sensors: %v", got)
			}

			beforeCore, beforeShadow := m.core.PendingCount(), m.shadow.state.PendingCount()
			seen := map[string]struct{}{hostDiskTemperatureResourceID(hostID, "/dev/sda"): {}}
			m.interruptHostDiskTemperatureRunsExcept(hostID, seen)
			if m.core.PendingCount() != beforeCore || m.shadow.state.PendingCount() != beforeShadow {
				t.Fatal("observed disk or other-host pending state was interrupted")
			}
			if len(m.hostDiskTempAbsences) != 11 {
				t.Fatal("observation gap changed another host's absence tracking")
			}
		})
	}
}

var pendingCollectionResult map[string]struct{}

// Exercise report-time callers, not just the reducer enumeration. A pending
// storm is deliberately included: normal/firing-only inventories do not expose
// the cost of collecting every other host's pending resources on every report.
func BenchmarkHostPendingResourceCollection(b *testing.B) {
	for _, hosts := range []int{1, 100, 1000} {
		for _, mode := range []string{"pending", "firing", "mixed"} {
			b.Run(fmt.Sprintf("hosts=%d/%s", hosts, mode), func(b *testing.B) {
				b.Run("native-sensors", func(b *testing.B) {
					m := pendingCollectionFixture(hosts, mode)
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						pendingCollectionResult = m.hostNativeSensorTrackedResourcesNoLock("host-0000")
					}
				})
				b.Run("disk-temperature-gap", func(b *testing.B) {
					m := pendingCollectionFixture(hosts, mode)
					seen := map[string]struct{}{hostDiskTemperatureResourceID("host-0000", "/dev/sda"): {}}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						m.interruptHostDiskTemperatureRunsExcept("host-0000", seen)
					}
				})
				b.Run("filesystem-gap", func(b *testing.B) {
					m := pendingCollectionFixture(hosts, mode)
					host := models.Host{ID: "host-0000", Disks: []models.Disk{{Mountpoint: "/root"}}}
					seen := map[string]struct{}{hostResourceID(host.ID) + "/disk:root": {}}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						m.reconcileHostFilesystemObservationGaps(host, seen)
					}
				})
				b.Run("host-report", func(b *testing.B) {
					m := NewManagerWithDataDir(b.TempDir())
					b.Cleanup(m.Stop)
					m.stopMu.Lock()
					m.stopping = true // close checkpoint admission before warm-up
					m.stopMu.Unlock()
					fleet := pendingCollectionFixture(hosts, mode)
					m.mu.Lock()
					m.config = AlertConfig{Enabled: true, ActivationState: ActivationActive, Overrides: fleet.config.Overrides,
						AgentDefaults: ThresholdConfig{DiskTemperature: &HysteresisThreshold{Trigger: 80, Clear: 75}}}
					m.core, m.shadow = fleet.core, fleet.shadow
					m.activeAlerts, m.intentPending = fleet.activeAlerts, fleet.intentPending
					m.hostDiskTempPendingContexts = fleet.hostDiskTempPendingContexts
					m.hostDiskTempAbsences = fleet.hostDiskTempAbsences
					m.mu.Unlock()
					cool := 25.0
					host := models.Host{ID: "host-0000", Hostname: "selected", Status: "online",
						Disks: []models.Disk{{Mountpoint: "/root", Usage: 10, Total: 100, Used: 10}},
						Sensors: models.HostSensorSummary{
							SMART: []models.HostDiskSMART{{Device: "/dev/sda", Temperature: 25}},
							Native: []models.HostNativeSensor{{ID: "chip/temp1", Kind: agentshost.NativeSensorKindTemperature,
								Status: agentshost.NativeSensorStatusOK, Value: &cool}},
						}}
					m.CheckHost(host) // steady report, after first-report transitions
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						m.CheckHost(host)
					}
				})
			})
		}
	}
}

// Keep a small executable reference of the old collect-then-filter algorithm.
// This is also a check on early prefix admission: core/shadow, old active
// records and intent-only runs must all still reach the same cleanup set.
func legacyDiskTemperatureResources(m *Manager, prefix string) map[string]struct{} {
	all := make(map[string]struct{})
	for id := range m.hostDiskTempPendingContexts {
		all[id] = struct{}{}
	}
	for _, state := range m.mirrorStatesNoLock() {
		for _, id := range state.PendingResourceIDs() {
			all[id] = struct{}{}
		}
	}
	for _, alert := range m.activeAlerts {
		if alert != nil {
			all[alert.ResourceID] = struct{}{}
		}
	}
	for _, pending := range m.intentPending {
		if pending.Signal == MetricAlertIntentSignal("diskTemperature") {
			all[pending.ResourceID] = struct{}{}
		}
	}
	for id := range all {
		if !strings.HasPrefix(id, prefix) {
			delete(all, id)
		}
	}
	return all
}

func TestHostPendingResourceCollectionMatchesLegacySets(t *testing.T) {
	for _, mode := range []string{"pending", "firing", "mixed"} {
		m := pendingCollectionFixture(12, mode)
		m.activeAlerts["nil-entry"] = nil
		contextOnly := hostDiskTemperatureResourceID("host-0000", "/dev/context-only")
		m.hostDiskTempPendingContexts[contextOnly] = hostDiskTempContext{hostID: "host-0000"}
		shadowOnly := hostDiskTemperatureResourceID("host-0000", "/dev/shadow-only")
		m.shadow.state.ApplyDiscrete(reducer.DiscreteSignal{ResourceID: shadowOnly, Key: "health", Matched: true}, reducer.DiscreteRule{Confirmations: 2})
		intentOnly := hostDiskTemperatureResourceID("host-0000", "/dev/intent-only")
		m.intentPending[intentOnly] = IntentPendingState{ResourceID: intentOnly, Signal: MetricAlertIntentSignal("diskTemperature")}
		for _, prefix := range []string{"", hostDiskTemperatureResourcePrefix("host-0000"), hostDiskTemperatureResourcePrefix("host-000"), "unmatched/"} {
			t.Run(mode+"/"+prefix, func(t *testing.T) {
				if got, want := m.hostDiskTemperatureTrackedResourcesNoLock(prefix), legacyDiskTemperatureResources(m, prefix); !reflect.DeepEqual(got, want) {
					t.Fatalf("disk temperature resources = %v, legacy = %v", got, want)
				}
			})
		}
	}
}

func TestHostPendingResourceCollectionAllocationsStayHostLocal(t *testing.T) {
	for _, mode := range []string{"pending", "firing", "mixed"} {
		for _, path := range []string{"native-sensors", "disk-temperature-gap", "filesystem-gap"} {
			t.Run(mode+"/"+path, func(t *testing.T) {
				measure := func(hosts int) float64 {
					m := pendingCollectionFixture(hosts, mode)
					seen := map[string]struct{}{hostDiskTemperatureResourceID("host-0000", "/dev/sda"): {}}
					host := models.Host{ID: "host-0000", Disks: []models.Disk{{Mountpoint: "/root"}}}
					filesystems := map[string]struct{}{hostResourceID(host.ID) + "/disk:root": {}}
					return testing.AllocsPerRun(10, func() {
						if path == "native-sensors" {
							pendingCollectionResult = m.hostNativeSensorTrackedResourcesNoLock("host-0000")
						} else if path == "filesystem-gap" {
							m.reconcileHostFilesystemObservationGaps(host, filesystems)
						} else {
							m.interruptHostDiskTemperatureRunsExcept("host-0000", seen)
						}
					})
				}
				small, fleet := measure(1), measure(1000)
				t.Logf("one host %.0f allocations, 1000 hosts %.0f", small, fleet)
				if fleet > small+4 {
					t.Fatalf("temporary allocations grew with unrelated fleet: %.0f -> %.0f", small, fleet)
				}
			})
		}
	}
}
