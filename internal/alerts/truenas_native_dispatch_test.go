package alerts

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Cover every documented TrueNAS severity through native projection and the
// manager's callback boundary. This does not claim external transport receipt.
func TestTrueNASNativeSeverityDispatch(t *testing.T) {
	for _, tc := range []struct {
		native string
		level  AlertLevel
	}{
		{"INFO", ""},
		{"NOTICE", AlertLevelInfo},
		{"WARNING", AlertLevelWarning},
		{"ERROR", AlertLevelCritical},
		{"CRITICAL", AlertLevelCritical},
		{"ALERT", AlertLevelCritical},
		{"EMERGENCY", AlertLevelCritical},
	} {
		t.Run(tc.native, func(t *testing.T) {
			m := newTestManager(t)
			configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())
			var dispatched []Alert
			// Unified incident dispatch is synchronous; retain values, not live pointers.
			m.SetAlertCallback(func(alert *Alert) { dispatched = append(dispatched, *alert) })
			resolved := make(chan string, 4)
			m.SetResolvedCallback(func(id string) { resolved <- id })
			project := func(level string) []unifiedresources.Resource {
				records := truenas.FixtureRecords(truenas.FixtureSnapshot{
					CollectedAt: time.Now(),
					System:      truenas.SystemInfo{Hostname: "native-dispatch", Healthy: true},
					Alerts:      []truenas.Alert{{ID: "native-1", Level: level, Message: "Provider condition"}},
				})
				var resources []unifiedresources.Resource
				for _, record := range records {
					record.Resource.ID = "host:" + record.SourceID
					resources = append(resources, record.Resource)
				}
				return resources
			}
			resources := project(" " + strings.ToLower(tc.native) + " ")
			m.SyncUnifiedResourceIncidents(resources)
			m.SyncUnifiedResourceIncidents(resources)
			want := 1
			if tc.level == "" {
				want = 0
			}
			if len(dispatched) != want || len(m.GetActiveAlerts()) != want {
				t.Fatalf("dispatches/active = %d/%d, want %d/%d", len(dispatched), len(m.GetActiveAlerts()), want, want)
			}
			if want == 0 {
				return
			}
			if dispatched[0].Level != tc.level {
				t.Fatalf("dispatched severity = %s, want %s", dispatched[0].Level, tc.level)
			}
			// Missing telemetry cannot resolve a native incident.
			m.SyncUnifiedResourceIncidents(nil)
			m.SyncUnifiedResourceIncidents(nil)
			if len(m.GetActiveAlerts()) != 1 {
				t.Fatal("missing telemetry resolved incident")
			}
			information := project("INFO")
			m.SyncUnifiedResourceIncidents(information)
			if len(m.GetActiveAlerts()) != 1 {
				t.Fatal("single recovery observation resolved incident")
			}
			m.SyncUnifiedResourceIncidents(information)
			if len(m.GetActiveAlerts()) != 0 {
				t.Fatal("confirmed recovery retained incident")
			}
			select {
			case id := <-resolved:
				if id != dispatched[0].ID {
					t.Fatalf("resolved ID = %s, want %s", id, dispatched[0].ID)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("missing resolution callback")
			}
			if len(dispatched) != 1 {
				t.Fatal("INFO recovery dispatched an alert")
			}
		})
	}
}
