package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Exercise real storage evaluation across manager restarts, rather than seeding
// a generic active-alert fixture. This is not installed receiver qualification.
func TestStorageRecoveryAcrossRestart(t *testing.T) {
	for _, sqlite := range []bool{false, true} {
		name := "recovery mirror"
		if sqlite {
			name = "SQLite authority"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			start := func() *Manager {
				m := NewManagerWithDataDir(dir)
				t.Cleanup(m.Stop)
				if sqlite {
					m.EnableEventLog()
					if !m.activeStateAuthoritative.Load() {
						t.Fatal("SQLite authority not enabled")
					}
				}
				m.UpdateConfig(AlertConfig{Enabled: true, ActivationState: ActivationActive,
					TimeThresholds: map[string]int{"storage": 0}, StorageDefault: HysteresisThreshold{Trigger: 80, Clear: 70}})
				return m
			}
			s := models.Storage{ID: "pbs-restart-backups", Name: "backups", Type: "pbs", Status: "online", Total: 1000, Used: 850, Free: 150, Usage: 85}
			id := canonicalMetricStateID(s.ID, "usage")
			m := start()
			for range 5 {
				m.CheckStorage(s)
			}
			original := *testRequireActiveAlert(t, m, id)
			if err := m.SaveActiveAlerts(); err != nil {
				t.Fatal(err)
			}
			m.Stop()

			m = start()
			restored := testRequireActiveAlert(t, m, id)
			if !restored.StartTime.Equal(original.StartTime) {
				t.Fatal("restart replaced the incident")
			}
			s.Usage, s.Total, s.Used, s.Free = 0, 0, 0, 0
			for range 5 {
				m.CheckStorage(s)
			}
			if !testHasActiveAlert(t, m, id) || m.GetResolvedAlert(id) != nil {
				t.Fatal("missing capacity after restart fabricated recovery")
			}
			s.Total, s.Free = 1000, 1000
			for range 5 {
				m.CheckStorage(s)
			}
			if testHasActiveAlert(t, m, id) {
				t.Fatal("confirmed empty capacity after restart did not recover")
			}
			resolved := m.GetResolvedAlert(id)
			if resolved == nil || resolved.Value != 0 || !resolved.StartTime.Equal(original.StartTime) {
				t.Fatalf("wrong recovered incident: %+v", resolved)
			}
			if err := m.SaveActiveAlerts(); err != nil {
				t.Fatal(err)
			}
			m.Stop()

			m = start()
			if testHasActiveAlert(t, m, id) {
				t.Fatal("second restart resurrected resolved incident")
			}
			for range 5 {
				m.CheckStorage(s)
			}
			if testHasActiveAlert(t, m, id) {
				t.Fatal("empty observation recreated resolved incident")
			}

			// Metric recovery followed by renewed pressure starts a fresh
			// occurrence; discrete-alert refire retention does not apply.
			s.Usage, s.Used, s.Free = 90, 900, 100
			for range 5 {
				m.CheckStorage(s)
			}
			recurrent := testRequireActiveAlert(t, m, id)
			if !recurrent.StartTime.After(original.StartTime) || recurrent.Value != 90 {
				t.Fatalf("recurrence reused the resolved start or lost fresh measurement: %+v", recurrent)
			}
			if got := len(m.GetActiveAlerts()); got != 1 {
				t.Fatalf("recurrence created %d active incidents, want 1", got)
			}
			recurrenceStart := recurrent.StartTime
			if err := m.SaveActiveAlerts(); err != nil {
				t.Fatal(err)
			}
			m.Stop()

			m = start()
			recurrent = testRequireActiveAlert(t, m, id)
			if !recurrent.StartTime.Equal(recurrenceStart) || recurrent.Value != 90 {
				t.Fatalf("recurrent incident did not survive another restart: %+v", recurrent)
			}
			s.Usage, s.Total, s.Used, s.Free = 0, 0, 0, 0
			for range 5 {
				m.CheckStorage(s)
			}
			if retained := testRequireActiveAlert(t, m, id); !retained.StartTime.Equal(recurrenceStart) || retained.Value != 90 {
				t.Fatalf("missing capacity changed recurrent incident: %+v", retained)
			}
			s.Total, s.Free = 1000, 1000
			for range 5 {
				m.CheckStorage(s)
			}
			if testHasActiveAlert(t, m, id) {
				t.Fatal("confirmed empty capacity did not resolve recurrence")
			}
			if resolved := m.GetResolvedAlert(id); resolved == nil || !resolved.StartTime.Equal(recurrenceStart) || resolved.Value != 0 {
				t.Fatalf("recovery resolved the wrong occurrence: %+v", resolved)
			}
		})
	}
}
