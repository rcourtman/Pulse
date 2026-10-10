package ai

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestPatrolObserverGuestMemoryOrigin(t *testing.T) {
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	for _, kind := range []unifiedresources.ResourceType{unifiedresources.ResourceTypeVM, unifiedresources.ResourceTypeSystemContainer} {
		for _, tc := range []struct {
			name, state, source, detail string
			percent                     float64
			at                          time.Time
			missing, noProxmox          bool
		}{
			{"healthy", "current", "guest-agent-meminfo", "", 24, now, false, false},
			{"zero", "current", "agent", "", 0, now, false, false},
			{"real-pressure", "current", "available-field", "guest=96.00", 96, now, false, false},
			{"inclusive-high", "current", "status-mem", "guest_memory_pressure_unknown", 96, now, false, false},
			{"inclusive-low", "current", "status-freemem", "guest_memory_pressure_unknown", 24, now, false, false},
			{"retained-low", "last-known", "agent", "guest_memory_pressure_unknown", 24, now.Add(-time.Hour), false, false},
			{"stale-origin", "current", "agent", "metric_stale", 24, now.Add(-61 * time.Second), false, false},
			{"age-boundary", "current", "agent", "", 24, now.Add(-59 * time.Second), false, false},
			{"missing-time", "current", "agent", "guest_memory_pressure_unknown", 24, time.Time{}, false, false},
			{"future-time", "current", "agent", "guest_memory_pressure_unknown", 24, now.Add(time.Hour), false, false},
			{"unknown-source", "current", "future-source", "guest_memory_pressure_unknown", 24, now, false, false},
			{"missing", "", "", "guest_memory_pressure_unknown", 0, time.Time{}, true, false},
			{"invalid", "current", "agent", "guest_memory_pressure_unknown", math.NaN(), now, false, false},
			{"other-platform", "", "", "", 24, time.Time{}, false, true},
		} {
			t.Run(string(kind)+"/"+tc.name, func(t *testing.T) {
				store := NewInMemoryPatrolObjectiveStore()
				objective, err := store.Create(CreatePatrolObjectiveInput{Brief: "Keep guest memory below 85 percent", ResourceIDs: []string{"guest"}}, now)
				if err != nil {
					t.Fatal(err)
				}
				_, err = store.ProposeObserver(objective.ID, ProposePatrolObserverInput{
					ExpectedRevision: objective.Revision, EvidenceFit: PatrolObserverEvidenceFitDirect,
					Interpretation: "Guest pressure stays below 85 percent.", TriggerKinds: []PatrolObserverTriggerKind{PatrolObserverTriggerInterval},
					ProbeJSON:    `{"runtime":"pulse-resource-metric/v1","metric":"memory_percent","operator":"less_than","threshold":85,"sample_interval_seconds":10,"wake_after_consecutive_failures":1,"max_evidence_age_seconds":60}`,
					WakeEvidence: "Memory objective breached or its pressure evidence is unavailable.", RequirementsJSON: `{}`,
				}, now)
				if err != nil {
					t.Fatal(err)
				}
				r := unifiedresources.Resource{ID: "guest", Type: kind, LastSeen: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
					Proxmox: &unifiedresources.ProxmoxData{Memory: &models.Memory{Usage: 24, Observation: models.MemoryObservation{State: "current", Source: "available-field", ObservedAt: now}}},
					Metrics: &unifiedresources.ResourceMetrics{Memory: &unifiedresources.MetricValue{Percent: tc.percent, Observation: models.MemoryObservation{State: tc.state, Source: tc.source, ObservedAt: tc.at}}},
				}
				if tc.missing {
					r.Metrics.Memory = nil
				}
				if tc.noProxmox {
					r.Proxmox = nil
				}
				patrol := NewPatrolService(nil, nil)
				patrol.SetObjectiveStore(store)
				patrol.SetUnifiedResourceProvider(&mockUnifiedResourceProvider{getAllFunc: func() []unifiedresources.Resource { return []unifiedresources.Resource{r} }})
				tm := NewTriggerManager(TriggerManagerConfig{MaxPendingTriggers: 10})
				patrol.SetTriggerManager(tm)
				patrol.processObjectiveObservers(now.Add(time.Second))
				got, _ := store.Get(objective.ID, now.Add(time.Second))
				if got.Observer == nil || got.Observer.State != PatrolObserverInstalled {
					t.Fatalf("existing observer failed to install: %+v", got.Observer)
				}
				if tc.detail == "" {
					if tm.GetPendingCount() != 0 || got.Coverage.State != PatrolObjectiveCovered {
						t.Fatalf("valid memory control lost coverage: %+v pending=%d", got.Coverage, tm.GetPendingCount())
					}
					return
				}
				if tm.GetPendingCount() != 1 || tm.pendingTriggers[0].ObjectiveContext == nil {
					t.Fatalf("unqualified/breached evidence accepted as healthy: %+v pending=%d", got.Coverage, tm.GetPendingCount())
				}
				context := tm.pendingTriggers[0].ObjectiveContext
				if !strings.Contains(context.Evidence, tc.detail) || fmt.Sprint(context.ObservedResourceIDs) != "[guest]" {
					t.Fatalf("wake lost unknown/breach/identity distinction: %+v", context)
				}
			})
		}
	}
}
