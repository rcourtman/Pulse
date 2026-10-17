package alerts

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Exercise callers that bypass buildUnifiedMetricCandidates, including the
// actual agent filesystem entry point. Missing percentages are not recovery.
func directPercentageObserver(t *testing.T, m *Manager, route, metric string) (string, func(float64)) {
	t.Helper()
	threshold := &HysteresisThreshold{Trigger: 80, Clear: 70}
	resourceID := "site:node:105"
	if route == "host-disk" {
		host := models.Host{ID: "filesystem-proof", Hostname: "fixture-host", DisplayName: "Fixture host", Status: "online", Disks: []models.Disk{{Mountpoint: "/archive", Device: "/dev/fixture", Total: 1000, Used: 950, Free: 50, Usage: 95}}}
		m.mu.Lock()
		m.config.AgentDefaults.Disk = threshold
		m.mu.Unlock()
		resourceID, _ = hostDiskResourceID(host, host.Disks[0])
		return resourceID, func(value float64) {
			host.Disks[0].Usage = value
			if value >= 0 && value <= 100 {
				host.Disks[0].Used = int64(value * 10)
				host.Disks[0].Free = host.Disks[0].Total - host.Disks[0].Used
			}
			m.CheckHost(host)
		}
	}
	kind := unifiedresources.ResourceTypeVM
	if metric == "usage" {
		kind = unifiedresources.ResourceTypeStorage
	}
	spec, err := buildCanonicalMetricSpec(resourceID, "fixture", kind, metric, threshold)
	if err != nil {
		t.Fatal(err)
	}
	return resourceID, func(value float64) {
		if route == "legacy" {
			m.checkMetric(resourceID, "fixture", "node", "site", string(kind), metric, value, threshold, nil)
		} else {
			m.checkMetricWithCanonicalSpec(spec, "fixture", "node", "site", string(kind), value, threshold, nil)
		}
	}
}

func TestDirectPercentageObservationContinuity(t *testing.T) {
	for _, route := range []string{"legacy", "canonical", "host-disk"} {
		metrics := []string{"cpu", "memory", "disk", "usage"}
		if route == "host-disk" {
			metrics = []string{"disk"}
		}
		for _, metric := range metrics {
			for _, gap := range []struct {
				name  string
				value float64
			}{{"negative", -1}, {"over-100", 101}} {
				t.Run(route+"/"+metric+"/"+gap.name, func(t *testing.T) {
					t.Run("activation", func(t *testing.T) {
						m, elapsed := continuityManager(t, false)
						_, observe := directPercentageObserver(t, m, route, metric)
						observe(gap.value)
						if len(m.GetActiveAlerts()) != 0 {
							t.Fatal("invalid percentage raised an incident")
						}
						observe(85)
						elapsed.Store(int64(2 * time.Minute))
						observe(gap.value)
						if len(m.GetActiveAlerts()) != 0 {
							t.Fatal("invalid percentage completed an activation run")
						}
						observe(85)
						if len(m.GetActiveAlerts()) != 0 {
							t.Fatal("activation continued across an invalid percentage")
						}
						elapsed.Store(int64(3 * time.Minute))
						observe(85)
						if len(m.GetActiveAlerts()) != 1 {
							t.Fatal("fresh continuous warning did not activate")
						}
					})
					t.Run("recovery", func(t *testing.T) {
						m, elapsed := continuityManager(t, false)
						id, observe := directPercentageObserver(t, m, route, metric)
						observe(100)
						// Complete ordinary activation even for non-noisy metrics.
						elapsed.Store(int64(time.Minute))
						observe(100)
						active := m.GetActiveAlerts()
						if len(active) != 1 {
							t.Fatal("100% control did not fire")
						}
						if err := m.AcknowledgeAlert(active[0].ID, "operator"); err != nil {
							t.Fatal(err)
						}
						if metric == "memory" {
							elapsed.Store(int64(time.Minute + 10*time.Second))
							observe(0)
							incident, _ := continuityIncident(m, id, metric)
							if incident.RecoverySince.IsZero() {
								t.Fatal("0% control did not start recovery")
							}
						}
						before, _ := json.Marshal(m.GetActiveAlerts())
						events := len(queryAlertEvents(t, m, eventlog.Filter{}))
						elapsed.Store(int64(3 * time.Minute))
						observe(gap.value)
						incident, _ := continuityIncident(m, id, metric)
						after, _ := json.Marshal(m.GetActiveAlerts())
						if !incident.RecoverySince.IsZero() || !bytes.Equal(before, after) || len(queryAlertEvents(t, m, eventlog.Filter{})) != events {
							t.Fatal("invalid percentage changed the incident, acknowledgement, timing or ledger")
						}
						observe(0)
						if metric != "memory" {
							if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeResolved}})) != 1 {
								t.Fatal("valid zero did not preserve immediate recovery")
							}
							return
						}
						if len(m.GetActiveAlerts()) != 1 {
							t.Fatal("fresh recovery crossed the invalid observation")
						}
						elapsed.Store(int64(3*time.Minute + 59*time.Second))
						observe(0)
						if len(m.GetActiveAlerts()) != 1 {
							t.Fatal("recovery completed early")
						}
						elapsed.Store(int64(4 * time.Minute))
						observe(0)
						if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeResolved}})) != 1 {
							t.Fatal("fresh continuous recovery did not resolve exactly once")
						}
						if m.ShadowDivergences() != 0 {
							t.Fatal("observation handling diverged from the shadow")
						}
					})
				})
			}
		}
	}
}

func TestHostDiskPercentageConsumerProjection(t *testing.T) {
	m, elapsed := continuityManager(t, false)
	id, observe := directPercentageObserver(t, m, "host-disk", "disk")
	emit := func(phase string) {
		t.Helper()
		b, err := json.Marshal(struct {
			Phase   string  `json:"phase"`
			Active  []Alert `json:"active"`
			History []Alert `json:"history"`
		}{phase, m.GetActiveAlerts(), m.GetAlertHistory(20)})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("ALERT_PERCENT_PROJECTION %s", b)
	}
	observe(95)
	elapsed.Store(int64(time.Minute))
	observe(95)
	active := m.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatal("fixture did not fire")
	}
	if err := m.AcknowledgeAlert(active[0].ID, "fixture operator"); err != nil {
		t.Fatal(err)
	}
	emit("firing")
	elapsed.Store(int64(time.Minute + 10*time.Second))
	observe(75) // Valid hysteresis-band evidence holds the same incident.
	emit("held-in-band")
	before, _ := json.Marshal(m.GetActiveAlerts())
	for i, gap := range []struct {
		phase string
		value float64
	}{{"gap-negative", -1}, {"gap-over-range", 101}} {
		elapsed.Store(int64(time.Duration(i+3) * time.Minute))
		observe(gap.value)
		after, _ := json.Marshal(m.GetActiveAlerts())
		incident, _ := continuityIncident(m, id, "disk")
		if !bytes.Equal(before, after) || !incident.RecoverySince.IsZero() {
			t.Fatal("invalid sample changed the held incident")
		}
		emit(gap.phase)
	}
	observe(0)
	if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeResolved}})) != 1 {
		t.Fatal("valid zero did not recover immediately and exactly once")
	}
	emit("recovered")
}

func TestDirectInvalidPercentageHonoursDisablementAndScalarMetrics(t *testing.T) {
	for _, route := range []string{"legacy", "canonical"} {
		t.Run(route, func(t *testing.T) {
			m, elapsed := continuityManager(t, false)
			id, observe := directPercentageObserver(t, m, route, "disk")
			observe(100)
			elapsed.Store(int64(time.Minute))
			observe(100)
			if len(m.GetActiveAlerts()) != 1 {
				t.Fatal("disablement control did not first activate")
			}
			disabled := &HysteresisThreshold{Trigger: -1}
			spec, err := buildCanonicalMetricSpec(id, "fixture", unifiedresources.ResourceTypeVM, "disk", disabled)
			if err != nil {
				t.Fatal(err)
			}
			if route == "legacy" {
				m.checkMetric(id, "fixture", "node", "site", "vm", "disk", -1, disabled, nil)
			} else {
				m.checkMetricWithCanonicalSpec(spec, "fixture", "node", "site", "vm", -1, disabled, nil)
			}
			if len(m.GetActiveAlerts()) != 0 {
				t.Fatal("explicit disablement was blocked by missing telemetry")
			}
			threshold := &HysteresisThreshold{Trigger: 80, Clear: 70}
			spec, err = buildCanonicalMetricSpec(id, "fixture", unifiedresources.ResourceTypeVM, "temperature", threshold)
			if err != nil {
				t.Fatal(err)
			}
			scalar := func(v float64) {
				if route == "legacy" {
					m.checkMetric(id, "fixture", "node", "site", "vm", "temperature", v, threshold, nil)
				} else {
					m.checkMetricWithCanonicalSpec(spec, "fixture", "node", "site", "vm", v, threshold, nil)
				}
			}
			scalar(150)
			elapsed.Store(int64(2 * time.Minute))
			scalar(150)
			if len(m.GetActiveAlerts()) != 1 {
				t.Fatal("scalar above 100 was rejected")
			}
			scalar(-10)
			incident, _ := continuityIncident(m, id, "temperature")
			if incident.RecoverySince.IsZero() {
				t.Fatal("negative temperature was rejected")
			}
		})
	}
}
