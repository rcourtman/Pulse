package alerts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A recovery-file failure must be reported without discarding a successful
// SQLite checkpoint, including an empty checkpoint after resolution.
func TestSQLiteCheckpointSurvivesRecoveryMirrorRenameFailure(t *testing.T) {
	for _, resolved := range []bool{false, true} {
		name := "fired"
		if resolved {
			name = "resolved"
		}
		t.Run(name, func(t *testing.T) {
			dataDir := t.TempDir()
			alert := durableRestoreAlert(time.Now().Add(-72 * time.Hour).UTC())
			initial := []*Alert{}
			if resolved {
				initial = append(initial, alert)
			}
			writeActiveRecoveryFixture(t, dataDir, initial)
			m := NewManagerWithDataDir(dataDir, WithDurableAlertStore())
			t.Cleanup(m.Stop)
			if !m.activeStateAuthoritative.Load() {
				t.Fatal("durable constructor did not establish SQLite authority")
			}

			// A non-empty directory forces rename failure even when tests run as
			// root; chmod-based fault injection would not reliably do so.
			mirror := filepath.Join(dataDir, "alerts", "active-alerts.json")
			if err := os.Remove(mirror); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(mirror, alertsDirPerm); err != nil {
				t.Fatal(err)
			}
			sentinel := filepath.Join(mirror, "preserve")
			if err := os.WriteFile(sentinel, []byte("unchanged"), alertsFilePerm); err != nil {
				t.Fatal(err)
			}
			// Change only memory: SaveActiveAlerts, not a lifecycle append,
			// must be responsible for the durable state checked after restart.
			m.mu.Lock()
			if resolved {
				m.removeActiveAlertNoLock(alert.ID)
			} else {
				m.setActiveAlertNoLock(alert.ID, alert)
			}
			m.mu.Unlock()
			err := m.SaveActiveAlerts()
			if err == nil || !strings.Contains(err.Error(), "SQLite active alert checkpoint succeeded but recovery persistence failed") || !strings.Contains(err.Error(), "failed to rename") {
				t.Fatalf("checkpoint error = %v; want reported mirror rename failure after SQLite success", err)
			}
			// Close SQLite before shutdown so its final save cannot repair a
			// missing checkpoint and conceal a failure of the explicit save.
			m.SetEventLog(nil)
			m.Stop()
			remaining, err := filepath.Glob(filepath.Join(dataDir, "alerts", "active-alerts-*.json.tmp"))
			if err != nil || len(remaining) != 0 {
				t.Fatalf("temporary mirrors left after failed writes = %v, error = %v", remaining, err)
			}
			if got, err := os.ReadFile(sentinel); err != nil || string(got) != "unchanged" {
				t.Fatalf("rename failure damaged destination: %q, %v", got, err)
			}
			if err := os.Remove(sentinel); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(mirror); err != nil {
				t.Fatal(err)
			}

			restarted := NewManagerWithDataDir(dataDir, WithDurableAlertStore())
			t.Cleanup(restarted.Stop)
			if !restarted.activeStateAuthoritative.Load() {
				t.Fatal("restart did not establish SQLite authority")
			}
			alerts := restarted.snapshotActiveAlerts()
			if resolved {
				if len(alerts) != 0 {
					t.Fatalf("resolved alert resurrected: %+v", alerts)
				}
			} else if len(alerts) != 1 || alerts[0].ID != alert.ID || !alerts[0].StartTime.Equal(alert.StartTime) || !alerts[0].Acknowledged || alerts[0].AckUser != alert.AckUser {
				t.Fatalf("checkpoint lost incident identity or acknowledgement: %+v", alerts)
			}
		})
	}
}
