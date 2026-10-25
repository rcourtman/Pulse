package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func pbsContinuityManager(t *testing.T, explicit bool) (*Manager, *atomic.Int64) {
	t.Helper()
	m, clock := continuityManager(t, explicit)
	m.mu.Lock()
	m.config.PBSDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 70}
	m.config.PBSDefaults.Memory = &HysteresisThreshold{Trigger: 80, Clear: 70}
	m.mu.Unlock()
	return m, clock
}

func pbsContinuitySample(id, metric string, value float64, missing bool) models.PBSInstance {
	pbs := models.PBSInstance{ID: id, Name: "backup " + id, Host: id + ".example.invalid",
		Status: "online", ConnectionHealth: "healthy", NodeMetricsUnavailable: missing}
	if metric == "cpu" {
		pbs.CPU = value
	} else {
		pbs.Memory = value
	}
	return pbs
}

func pbsContinuityAlerts(t *testing.T, m *Manager) []byte {
	t.Helper()
	data, err := json.Marshal(m.GetActiveAlerts())
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPBSMetricGapRestartsActivation(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		for _, metric := range []string{"cpu", "memory"} {
			for _, retained := range []float64{0, 85, 99} {
				t.Run(fmt.Sprintf("%s/intent=%v/retained=%.0f", metric, explicit, retained), func(t *testing.T) {
					m, clock := pbsContinuityManager(t, explicit)
					const id, peer = "pbs-primary", "pbs-primary-other"
					m.CheckPBS(pbsContinuitySample(id, metric, 85, false))
					m.CheckPBS(pbsContinuitySample(peer, metric, 85, false))
					if incident, ok := continuityIncident(m, id, metric); !ok || incident.State != reducer.StatePending {
						t.Fatalf("fresh warning did not start a pending run: %+v", incident)
					}
					clock.Store(int64(2 * time.Minute))
					for range 3 {
						m.CheckPBS(pbsContinuitySample(id, metric, retained, true))
					}
					if _, ok := continuityIncident(m, id, metric); ok {
						t.Error("PBS_GAP_ACTIVATION: missing node status retained the pending run")
					}
					if _, ok := continuityIncident(m, peer, metric); !ok {
						t.Error("missing node status interrupted another PBS")
					}
					m.mu.RLock()
					for _, pending := range m.intentPending {
						if pending.ResourceID == id {
							t.Error("PBS_GAP_INTENT: missing node status retained durable intent grace")
						}
					}
					m.mu.RUnlock()
					if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{})) != 0 {
						t.Fatal("missing node status manufactured a lifecycle transition")
					}
					m.CheckPBS(pbsContinuitySample(id, metric, 85, false))
					if len(m.GetActiveAlerts()) != 0 {
						t.Fatal("PBS_GAP_FRESH: first fresh poll completed grace across missing node status")
					}
					clock.Store(int64(2*time.Minute + 59*time.Second))
					m.CheckPBS(pbsContinuitySample(id, metric, 85, false))
					if len(m.GetActiveAlerts()) != 0 {
						t.Fatal("fresh activation completed before the whole configured delay")
					}
					clock.Store(int64(3 * time.Minute))
					m.CheckPBS(pbsContinuitySample(id, metric, 85, false))
					active := m.GetActiveAlerts()
					if len(active) != 1 || active[0].ResourceID != id || active[0].Type != metric ||
						len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeFired}})) != 1 {
						t.Fatalf("fresh continuous evidence did not fire exactly once: %+v", active)
					}
					if m.ShadowDivergences() != 0 {
						t.Fatal("PBS interruption diverged from the shadow reducer")
					}
				})
			}
		}
	}
}

func TestPBSMetricGapRestartsRecovery(t *testing.T) {
	for _, retained := range []float64{0, 85, 99} {
		t.Run(fmt.Sprintf("retained=%.0f", retained), func(t *testing.T) {
			m, clock := pbsContinuityManager(t, false)
			const id = "pbs-primary"
			m.CheckPBS(pbsContinuitySample(id, "memory", 95, false))
			initial := m.GetActiveAlerts()
			if len(initial) != 1 {
				t.Fatal("critical memory control must fire immediately")
			}
			if err := m.AcknowledgeAlert(initial[0].ID, "operator"); err != nil {
				t.Fatal(err)
			}
			clock.Store(int64(10 * time.Second))
			m.CheckPBS(pbsContinuitySample(id, "memory", 0, false))
			if incident, _ := continuityIncident(m, id, "memory"); incident.RecoverySince.IsZero() {
				t.Fatal("fresh zero must begin the existing memory recovery delay")
			}
			before := pbsContinuityAlerts(t, m)
			events := len(queryAlertEvents(t, m, eventlog.Filter{}))
			clock.Store(int64(2 * time.Minute))
			for range 3 {
				m.CheckPBS(pbsContinuitySample(id, "memory", retained, true))
			}
			if incident, _ := continuityIncident(m, id, "memory"); !incident.RecoverySince.IsZero() {
				t.Error("PBS_GAP_RECOVERY: missing node status retained the recovery run")
			}
			if !bytes.Equal(before, pbsContinuityAlerts(t, m)) || len(queryAlertEvents(t, m, eventlog.Filter{})) != events {
				t.Fatal("missing node status changed the occurrence, acknowledgement, trusted evidence or ledger")
			}
			m.CheckPBS(pbsContinuitySample(id, "memory", 0, false))
			if len(m.GetActiveAlerts()) != 1 {
				t.Fatal("PBS_GAP_RESOLVED: first fresh zero falsely recovered across the missing interval")
			}
			clock.Store(int64(2*time.Minute + 59*time.Second))
			m.CheckPBS(pbsContinuitySample(id, "memory", 0, false))
			if len(m.GetActiveAlerts()) != 1 {
				t.Fatal("fresh recovery completed before the whole configured delay")
			}
			clock.Store(int64(3 * time.Minute))
			m.CheckPBS(pbsContinuitySample(id, "memory", 0, false))
			if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeResolved}})) != 1 {
				t.Fatal("fresh continuous evidence must resolve the original exactly once")
			}
			if m.ShadowDivergences() != 0 {
				t.Fatal("PBS recovery interruption diverged from the shadow reducer")
			}
		})
	}
}

func TestPBSMetricGapKeepsImmediateCPURecoveryAndGlobalPause(t *testing.T) {
	m, _ := pbsContinuityManager(t, false)
	m.mu.Lock()
	m.config.MetricTimeThresholds["all"]["cpu"] = 0
	m.mu.Unlock()
	pbs := pbsContinuitySample("pbs-primary", "cpu", 95, false)
	m.CheckPBS(pbs)
	if len(m.GetActiveAlerts()) != 1 {
		t.Fatal("CPU control did not fire")
	}
	pbs.CPU, pbs.NodeMetricsUnavailable = 0, true
	m.CheckPBS(pbs)
	if len(m.GetActiveAlerts()) != 1 {
		t.Fatal("missing CPU observation resolved the alert")
	}
	m.mu.Lock()
	m.config.Enabled = false
	m.mu.Unlock()
	pbs.NodeMetricsUnavailable = false
	m.CheckPBS(pbs)
	if len(m.GetActiveAlerts()) != 1 {
		t.Fatal("global pause must not be treated as metric recovery")
	}
	m.mu.Lock()
	m.config.Enabled = true
	m.mu.Unlock()
	m.CheckPBS(pbs)
	if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeResolved}})) != 1 {
		t.Fatal("measured zero must retain immediate CPU recovery, not gain a new delay")
	}
}
