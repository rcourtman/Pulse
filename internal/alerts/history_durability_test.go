package alerts

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
)

func durabilityManager(t *testing.T, dir string, calls *atomic.Int64) *Manager {
	t.Helper()
	m := NewManagerWithDataDir(dir, WithDurableAlertStore())
	cfg := m.GetConfig()
	cfg.Enabled, cfg.ActivationState = true, ActivationActive
	m.UpdateConfig(cfg)
	if calls != nil {
		m.SetAlertCallback(func(*Alert) { calls.Add(1) })
	}
	return m
}

// This uses a real SQLite trigger fault through the production producer. The
// trigger rejects the event writer regardless of its pooled connection. It
// makes no claim about actual notification-provider delivery.
func TestHistoryJournalRecoversRetiredProducerOccurrenceAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	var calls atomic.Int64
	seed := durabilityManager(t, dir, &calls)
	seed.RaiseSystemAlert(SystemAlertInput{Type: "journal-seed", Level: AlertLevelWarning, Message: "seed"})
	seed.ClearSystemAlert("journal-seed")
	seed.Stop()
	m := durabilityManager(t, dir, &calls)
	if !m.historyManager.storageRetired {
		m.Stop()
		t.Fatal("legacy history was not retired")
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "alerts", "events.db"))
	if err != nil {
		m.Stop()
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TRIGGER history_test_write_fault BEFORE INSERT ON alert_events BEGIN SELECT RAISE(ABORT,'history driver fault'); END`); err != nil {
		m.Stop()
		t.Fatal(err)
	}
	beforeCalls := calls.Load()
	var transitions []LifecycleEvent
	unsubscribe := m.SubscribeLifecycleCallback(func(event LifecycleEvent) {
		if event.Alert.ID == SystemAlertID("journal-occurrence") {
			transitions = append(transitions, event)
		}
	})
	if !m.RaiseSystemAlert(SystemAlertInput{Type: "journal-occurrence", Level: AlertLevelCritical, Message: "retained through fault"}) {
		m.Stop()
		t.Fatal("evaluation failed closed")
	}
	if !m.ClearSystemAlert("journal-occurrence") {
		m.Stop()
		t.Fatal("resolution failed closed")
	}
	unsubscribe()
	if len(transitions) != 2 || transitions[0].Persisted || transitions[1].Persisted {
		m.Stop()
		t.Fatalf("faulted lifecycle callbacks: %+v", transitions)
	}
	at := transitions[0].OccurredAt
	if m.EventHistoryDurability().PendingEvents < 2 || m.AlertQualityTelemetrySnapshot(time.Now()).EventHistoryHealthyTenants != 0 {
		m.Stop()
		t.Fatal("unsettled history reported healthy")
	}
	m.Stop() // keep trigger installed through final drain
	// Raising the alert admits an asynchronous delivery callback. Count it
	// only after Stop has drained that owner, not after the producer returns.
	if got := calls.Load(); got != beforeCalls+1 {
		t.Fatalf("delivery callback invocation count=%d, want %d", got, beforeCalls+1)
	}
	if _, err := db.Exec(`DROP TRIGGER history_test_write_fault`); err != nil {
		t.Fatal(err)
	}
	for boot := 0; boot < 2; boot++ {
		m = durabilityManager(t, dir, &calls)
		rows, err := m.AlertEvents(eventlog.Filter{AlertID: SystemAlertID("journal-occurrence")})
		if err != nil {
			m.Stop()
			t.Fatal(err)
		}
		counts := map[string]int{}
		for _, event := range rows {
			counts[event.Type]++
			if event.Type == eventlog.TypeFired && !event.OccurredAt.Equal(at) {
				m.Stop()
				t.Fatal("replay changed original occurrence time")
			}
		}
		if counts[eventlog.TypeFired] != 1 || counts[eventlog.TypeResolved] != 1 || counts[eventlog.TypeNotificationDispatched] != 1 {
			m.Stop()
			t.Fatalf("reopen lost or duplicated lifecycle/decision: %v", counts)
		}
		found := 0
		for _, alert := range m.GetAlertHistory(0) {
			if alert.ID == SystemAlertID("journal-occurrence") {
				found++
			}
		}
		if found != 1 || m.AlertQualityTelemetrySnapshot(time.Now()).EventHistoryHealthyTenants != 1 {
			m.Stop()
			t.Fatalf("history/health after recovery: %d %+v", found, m.EventHistoryDurability())
		}
		m.Stop() // include any asynchronous startup callback in the replay check
		if got := calls.Load(); got != beforeCalls+1 {
			t.Fatalf("history recovery invoked delivery callback: count=%d, want %d", got, beforeCalls+1)
		}
	}
}

func TestHistoryJournalResolutionCannotBeUndoneByOlderRecoveryMarker(t *testing.T) {
	dir := t.TempDir()
	m := durabilityManager(t, dir, nil)
	if !m.RaiseSystemAlert(SystemAlertInput{Type: "journal-stale-marker", Level: AlertLevelWarning, Message: "active before crash"}) {
		m.Stop()
		t.Fatal("seed not raised")
	}
	active := m.GetActiveAlerts()
	if len(active) != 1 {
		m.Stop()
		t.Fatalf("seed active set: %+v", active)
	}
	// Preserve an older marker, then model an admitted resolution in the next
	// owner before its producer can checkpoint that marker. The separate store
	// provides the same immutable journal/SQLite boundary without a runtime hook.
	if err := m.writeActiveStateDegradedRecovery([]*Alert{active[0].Clone()}, fmt.Errorf("previous write fault"), m.eventLogStore().JournalBoundary()); err != nil {
		m.Stop()
		t.Fatal(err)
	}
	m.Stop()
	db, err := sql.Open("sqlite", filepath.Join(dir, "alerts", "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Install the fixture's write fault before starting another store owner.
	// Its startup maintenance may write too; a no-wait auxiliary connection
	// must not race that owner while creating the test trigger.
	if _, err := db.Exec(`CREATE TRIGGER history_test_write_fault BEFORE INSERT ON alert_events BEGIN SELECT RAISE(ABORT,'history driver fault'); END`); err != nil {
		t.Fatal(err)
	}
	store, err := eventlog.Open(filepath.Join(dir, "alerts"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := json.Marshal(active[0])
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.AppendDurable(eventlog.Event{Type: eventlog.TypeResolved, AlertID: active[0].ID, OccurredAt: time.Now().UTC(), Snapshot: snapshot}); err == nil {
		store.Close()
		t.Fatal("write fault did not fail")
	}
	store.Close()
	if _, err := db.Exec(`DROP TRIGGER history_test_write_fault`); err != nil {
		t.Fatal(err)
	}
	restarted := durabilityManager(t, dir, nil)
	defer restarted.Stop()
	if active := restarted.GetActiveAlerts(); len(active) != 0 {
		t.Fatalf("older marker resurrected journal-resolved occurrence: %+v", active)
	}
	rows, err := restarted.AlertEvents(eventlog.Filter{AlertID: active[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	resolved := 0
	for _, event := range rows {
		if event.Type == eventlog.TypeResolved {
			resolved++
		}
	}
	if resolved != 1 {
		t.Fatalf("resolution missing or duplicated: %+v", rows)
	}
}

func TestHistoryJournalPendingResolutionSurvivesContinuingFaultAtRestart(t *testing.T) {
	dir := t.TempDir()
	m := durabilityManager(t, dir, nil)
	if !m.RaiseSystemAlert(SystemAlertInput{Type: "journal-continuing-fault", Level: AlertLevelWarning, Message: "before fault"}) {
		m.Stop()
		t.Fatal("seed not raised")
	}
	active := m.GetActiveAlerts()
	if len(active) != 1 {
		m.Stop()
		t.Fatal("seed active set missing")
	}
	if err := m.writeActiveStateDegradedRecovery([]*Alert{active[0].Clone()}, fmt.Errorf("old fault"), m.eventLogStore().JournalBoundary()); err != nil {
		m.Stop()
		t.Fatal(err)
	}
	m.Stop()
	db, err := sql.Open("sqlite", filepath.Join(dir, "alerts", "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TRIGGER history_test_write_fault BEFORE INSERT ON alert_events BEGIN SELECT RAISE(ABORT,'continuing history fault'); END`); err != nil {
		t.Fatal(err)
	}
	store, err := eventlog.Open(filepath.Join(dir, "alerts"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := json.Marshal(active[0])
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.AppendDurable(eventlog.Event{Type: eventlog.TypeResolved, AlertID: active[0].ID, OccurredAt: time.Now().UTC(), Snapshot: snapshot}); err == nil {
		store.Close()
		t.Fatal("driver fault disappeared")
	}
	store.Close()
	for boot := 0; boot < 2; boot++ {
		m = durabilityManager(t, dir, nil)
		if got := m.GetActiveAlerts(); len(got) != 0 {
			m.Stop()
			t.Fatalf("pending resolution resurrected under continuing fault: %+v", got)
		}
		status := m.EventHistoryDurability()
		if status.PendingEvents < 1 || status.Healthy() {
			m.Stop()
			t.Fatalf("pending history hidden: %+v", status)
		}
		m.Stop()
	}
	if _, err := db.Exec(`DROP TRIGGER history_test_write_fault`); err != nil {
		t.Fatal(err)
	}
	m = durabilityManager(t, dir, nil)
	defer m.Stop()
	events, err := m.AlertEvents(eventlog.Filter{AlertID: active[0].ID, Types: []string{eventlog.TypeResolved}})
	if err != nil || len(events) != 1 || len(m.GetActiveAlerts()) != 0 {
		t.Fatalf("settled resolution lost: %+v %v", events, err)
	}
}

func TestHistoryJournalUnreadableSQLiteStillFoldsPendingResolution(t *testing.T) {
	dir := t.TempDir()
	m := durabilityManager(t, dir, nil)
	if !m.RaiseSystemAlert(SystemAlertInput{Type: "unreadable-target", Level: AlertLevelWarning, Message: "active"}) {
		t.Fatal("seed not raised")
	}
	active := m.GetActiveAlerts()[0].Clone()
	boundary := m.eventLogStore().JournalBoundary()
	m.Stop()
	// A stale self-contained marker is independent of the compatibility mirror.
	if err := m.writeActiveStateDegradedRecovery([]*Alert{active}, fmt.Errorf("crash before checkpoint"), boundary); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "alerts", "events.db")
	db, err := sql.Open("sqlite", target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER unreadable_test_fault BEFORE INSERT ON alert_events BEGIN SELECT RAISE(ABORT,'continuing history fault'); END`); err != nil {
		t.Fatal(err)
	}
	store, err := eventlog.Open(filepath.Join(dir, "alerts"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := json.Marshal(active)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendDurable(eventlog.Event{Type: eventlog.TypeResolved, AlertID: active.ID, OccurredAt: time.Now().UTC(), Snapshot: snapshot}); err == nil {
		t.Fatal("trigger fault hidden")
	}
	store.Close()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(target, target+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int64
	for boot := 0; boot < 2; boot++ {
		m = durabilityManager(t, dir, &calls)
		status := m.EventHistoryDurability()
		if got := m.GetActiveAlerts(); len(got) != 0 {
			m.Stop()
			t.Fatalf("unreadable target resurrected resolved alert: %+v", got)
		}
		if status.PendingEvents < 1 || status.Healthy() {
			m.Stop()
			t.Fatalf("unavailable target hidden: %+v", status)
		}
		m.Stop()
	}
	if calls.Load() != 0 {
		t.Fatal("recovery invoked notification callback")
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(target+".saved", target); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite", target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TRIGGER unreadable_test_fault`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	m = durabilityManager(t, dir, &calls)
	defer m.Stop()
	rows, err := m.AlertEvents(eventlog.Filter{AlertID: active.ID, Types: []string{eventlog.TypeResolved}})
	if err != nil || len(rows) != 1 || len(m.GetActiveAlerts()) != 0 {
		t.Fatalf("restored target lost resolution: %+v %v", rows, err)
	}
}

func TestHistoryJournalReplayPreservesNewerObservationAndAppliesControlTransition(t *testing.T) {
	store, err := eventlog.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	at := time.Now().Add(-time.Hour).UTC()
	original := &Alert{ID: SystemAlertID("newer-observation"), Type: "newer-observation", Level: AlertLevelWarning, Message: "old", StartTime: at, LastSeen: at, Metadata: map[string]interface{}{systemAlertFingerprintKey: "old"}}
	snapshot, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	boundary := store.JournalBoundary()
	if err := store.AppendDurable(eventlog.Event{Type: eventlog.TypeFired, AlertID: original.ID, OccurredAt: at, Snapshot: snapshot}); err != nil {
		t.Fatal(err)
	}
	observed := original.Clone()
	observed.Level, observed.Message, observed.LastSeen = AlertLevelCritical, "new", at.Add(time.Minute)
	observed.Metadata[systemAlertFingerprintKey] = "new"
	original.Acknowledged, original.AckUser = true, "operator"
	ack := at.Add(2 * time.Minute)
	original.AckTime = &ack
	snapshot, err = json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendDurable(eventlog.Event{Type: eventlog.TypeAcknowledged, AlertID: original.ID, OccurredAt: ack, Snapshot: snapshot}); err != nil {
		t.Fatal(err)
	}
	recovered, err := reconcileActiveRecoveryHistory([]*Alert{observed}, store, boundary)
	if err != nil || len(recovered) != 1 {
		t.Fatalf("reconcile: %+v %v", recovered, err)
	}
	got := recovered[0]
	if got.Level != AlertLevelCritical || got.Metadata[systemAlertFingerprintKey] != "new" || got.Message != "new" || !got.Acknowledged || got.AckUser != "operator" {
		t.Fatalf("replay rolled back observation or lost explicit acknowledgement: %+v", got)
	}
}

func TestHistoryJournalReplaceAndClearResetFailureCheckpointMirror(t *testing.T) {
	m := durabilityManager(t, t.TempDir(), nil)
	defer m.Stop()
	m.RaiseSystemAlert(SystemAlertInput{Type: "stale-recovery-map", Level: AlertLevelWarning, Message: "active"})
	if len(m.activeRecoverySnapshot()) != 1 {
		t.Fatal("seed absent from recovery map")
	}
	if err := m.restoreActiveAlertSnapshots(nil, "canonical empty restore", true); err != nil {
		t.Fatal(err)
	}
	if err := m.checkpointActiveRecoveryAfterDurableFailure(fmt.Errorf("subsequent lifecycle fault")); err != nil {
		t.Fatal(err)
	}
	recovered, selfContained, err := m.loadActiveStateDegradedRecovery()
	if err != nil || !selfContained || len(recovered) != 0 {
		t.Fatalf("replacement left stale checkpoint state: %+v %v", recovered, err)
	}
	m.RaiseSystemAlert(SystemAlertInput{Type: "clear-recovery-map", Level: AlertLevelWarning, Message: "active"})
	m.ClearActiveAlerts()
	if err := m.checkpointActiveRecoveryAfterDurableFailure(fmt.Errorf("fault after clear")); err != nil {
		t.Fatal(err)
	}
	recovered, _, err = m.loadActiveStateDegradedRecovery()
	if err != nil || len(recovered) != 0 {
		t.Fatalf("clear left stale checkpoint state: %+v %v", recovered, err)
	}
}

func TestHistoryJournalCaptureBoundaryKeepsCleanupAbsenceAcrossUnreadableRestart(t *testing.T) {
	dir := t.TempDir()
	m := durabilityManager(t, dir, nil)
	m.RaiseSystemAlert(SystemAlertInput{Type: "retired-without-resolution", Level: AlertLevelWarning, Message: "active before clear"})
	m.ClearActiveAlerts() // This is removal, not detector recovery or a resolution event.
	m.Stop()
	if boundary := m.activeRecoveryCaptureBoundary(); boundary.ID == "" || boundary.Sequence == 0 {
		t.Fatalf("mirror capture prefix absent: %+v", boundary)
	}
	target := filepath.Join(dir, "alerts", "events.db")
	if err := os.Rename(target, target+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	for boot := 0; boot < 2; boot++ {
		m = durabilityManager(t, dir, nil)
		if got := m.GetActiveAlerts(); len(got) != 0 {
			m.Stop()
			t.Fatalf("floor replay resurrected retired alert: %+v", got)
		}
		m.Stop()
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(target+".saved", target); err != nil {
		t.Fatal(err)
	}
	m = durabilityManager(t, dir, nil)
	defer m.Stop()
	if got := m.GetActiveAlerts(); len(got) != 0 {
		t.Fatalf("healthy replay resurrected retired alert: %+v", got)
	}
}

func TestHistoryJournalCaptureNeverBindsDifferentMirrorBytes(t *testing.T) {
	m := durabilityManager(t, t.TempDir(), nil)
	defer m.Stop()
	m.RaiseSystemAlert(SystemAlertInput{Type: "capture-hash", Level: AlertLevelWarning, Message: "active"})
	if err := m.SaveActiveAlerts(); err != nil {
		t.Fatal(err)
	}
	// A synchronous save does not drain the producer's queued checkpoint.
	// Stop owns that drain and the final save, so no background writer can
	// repair our deliberately changed mirror before the binding assertion.
	m.Stop()
	boundary := m.activeRecoveryCaptureBoundary()
	if boundary.ID == "" || boundary.Sequence == 0 {
		t.Fatalf("capture absent: %+v", boundary)
	}
	path := filepath.Join(m.getAlertsDir(), "active-alerts.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var seeded []*Alert
	if err := json.Unmarshal(original, &seeded); err != nil || len(seeded) != 1 || seeded[0] == nil || seeded[0].Message != "active" {
		t.Fatalf("nonempty captured seed missing: alerts=%+v error=%v", seeded, err)
	}
	edited := bytes.Replace(original, []byte(`"message":"active"`), []byte(`"message":"edited"`), 1)
	if bytes.Equal(edited, original) || len(edited) != len(original) {
		t.Fatal("same-length mirror edit did not change the seed")
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{name: "empty-set", data: []byte("[]")},
		{name: "same-length-record", data: edited},
		{name: "whitespace-only", data: append(append([]byte(nil), original...), '\n')},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, tc.data, alertsFilePerm); err != nil {
				t.Fatal(err)
			}
			if got := m.activeRecoveryCaptureBoundary(); got.ID != "" {
				t.Fatalf("capture accepted different mirror bytes: %+v", got)
			}
			if err := os.WriteFile(path, original, alertsFilePerm); err != nil {
				t.Fatal(err)
			}
			if got := m.activeRecoveryCaptureBoundary(); got != boundary {
				t.Fatalf("exact restored bytes lost their matching capture: got %+v, want %+v", got, boundary)
			}
		})
	}
}

func TestHistoryJournalPartialSnapshotCannotConcealCompactedActiveFacts(t *testing.T) {
	for _, pruneHistory := range []bool{false, true} {
		name := "retained-history"
		if pruneHistory {
			name = "pruned-history"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			m := durabilityManager(t, dir, nil)
			t.Cleanup(func() { m.Stop() })
			m.RaiseSystemAlert(SystemAlertInput{Type: "compacted-active", Level: AlertLevelWarning, Message: "active authority"})
			active := m.GetActiveAlerts()[0].Clone()
			if err := m.AcknowledgeAlert(active.ID, "operator"); err != nil {
				t.Fatal(err)
			}
			journalID := m.eventLogStore().JournalBoundary().ID
			m.Stop()
			store, err := eventlog.Open(filepath.Join(dir, "alerts"))
			if err != nil {
				t.Fatal(err)
			}
			store.Close() // Compact the committed frames; only SQLite retains the facts.
			if pruneHistory {
				db, err := sql.Open("sqlite", filepath.Join(dir, "alerts", "events.db"))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`DELETE FROM alert_events WHERE alert_id = ?`, active.ID); err != nil {
					db.Close()
					t.Fatal(err)
				}
				var activeRows int
				if err := db.QueryRow(`SELECT count(*) FROM alert_active_state WHERE alert_id = ?`, active.ID).Scan(&activeRows); err != nil || activeRows != 1 {
					db.Close()
					t.Fatalf("retention fixture lost active authority: count=%d error=%v", activeRows, err)
				}
				db.Close()
			}

			if err := os.WriteFile(filepath.Join(dir, "alerts", "active-alerts.json"), []byte("[]"), alertsFilePerm); err != nil {
				t.Fatal(err)
			}
			if err := m.writeActiveRecoveryCapture([]byte("[]"), eventlog.JournalBoundary{ID: journalID}); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(dir, "alerts", "events.db")
			if err := os.Rename(target, target+".saved"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(target, 0700); err != nil {
				t.Fatal(err)
			}
			m = durabilityManager(t, dir, nil)
			// New live removal during a partial restore must be explicit, because its
			// checkpoint cannot claim absence for the unknown committed prefix.
			m.RaiseSystemAlert(SystemAlertInput{Type: "partial-run-retirement", Level: AlertLevelWarning, Message: "retire during outage"})
			m.ClearActiveAlerts()
			m.Stop()
			if got := m.activeRecoverySnapshotBoundary(); got.Sequence != 0 {
				t.Fatalf("partial checkpoint overclaimed prefix: %+v", got)
			}
			if err := os.Remove(target); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(target+".saved", target); err != nil {
				t.Fatal(err)
			}
			m = durabilityManager(t, dir, nil)
			defer m.Stop()
			restored := m.GetActiveAlerts()
			if len(restored) != 1 || restored[0].ID != active.ID || !restored[0].StartTime.Equal(active.StartTime) || !restored[0].Acknowledged || restored[0].AckUser != "operator" {
				t.Fatalf("partial checkpoint erased canonical active facts or revived retired alert: %+v", restored)
			}
			retired, err := m.AlertEvents(eventlog.Filter{AlertID: SystemAlertID("partial-run-retirement"), Types: []string{eventlog.TypeActiveRetired}})
			if err != nil || len(retired) != 1 {
				t.Fatalf("retirement was not explicit: %+v %v", retired, err)
			}
		})
	}
}

func TestHistoryJournalDelayedControlCannotRepopulateFailureMirror(t *testing.T) {
	dir := t.TempDir()
	m := durabilityManager(t, dir, nil)
	m.RaiseSystemAlert(SystemAlertInput{Type: "delayed-control", Level: AlertLevelWarning, Message: "active"})
	stale := m.GetActiveAlerts()[0].Clone()
	m.ClearSystemAlert("delayed-control")
	stale.Acknowledged, stale.AckUser = true, "operator"
	now := time.Now()
	stale.AckTime = &now
	// Reproduce the user-control emission window after its primary mutation
	// released m.mu and a concurrent resolution admitted its terminal fact.
	m.recordAlertEvent(eventlog.TypeAcknowledged, stale, stale.ID, "", "delayed acknowledgement", nil)
	if got := m.activeRecoverySnapshot(); len(got) != 0 {
		t.Fatalf("stale control repopulated failure mirror: %+v", got)
	}
	if err := m.checkpointActiveRecoveryAfterDurableFailure(fmt.Errorf("later write fault")); err != nil {
		t.Fatal(err)
	}
	m.Stop()
	m = durabilityManager(t, dir, nil)
	defer m.Stop()
	if got := m.GetActiveAlerts(); len(got) != 0 {
		t.Fatalf("stale control reactivated resolved alert: %+v", got)
	}
}

func TestHistoryJournalGuestIdentityMoveSurvivesPartialRecovery(t *testing.T) {
	dir := t.TempDir()
	m := durabilityManager(t, dir, nil)
	t.Cleanup(func() { m.Stop() })
	m.Stop()
	target := filepath.Join(dir, "alerts", "events.db")
	if err := os.Rename(target, target+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	m = durabilityManager(t, dir, nil)
	m.UpdateConfig(characterizationBaseConfig())
	// Match the migration characterization fixture's immediate evaluation;
	// UpdateConfig otherwise supplies the normal observation delay.
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.MetricTimeThresholds = nil
	m.mu.Unlock()
	oldResourceID := BuildGuestKey("pve1", "node1", 101)
	newResourceID := BuildGuestKey("pve1", "node2", 101)
	oldID := canonicalMetricStateID(oldResourceID, "cpu")
	newID := canonicalMetricStateID(newResourceID, "cpu")
	m.CheckGuest(testVM(oldResourceID, 101, "app01", "node1", "pve1", "running", 0.95), "pve1")
	initial := m.GetActiveAlerts()
	if len(initial) != 1 || initial[0].ID != oldID {
		t.Fatalf("initial migration occurrence not active: %+v", initial)
	}
	original := initial[0].Clone()
	if err := m.AcknowledgeAlert(oldID, "operator"); err != nil {
		t.Fatal(err)
	}
	m.CheckGuest(testVM(newResourceID, 101, "app01", "node2", "pve1", "running", 0.92), "pve1")
	if got := m.GetActiveAlerts(); len(got) != 1 || got[0].ID != newID {
		t.Fatalf("live migration duplicated occurrence: %+v", got)
	}
	m.Stop()
	m = durabilityManager(t, dir, nil)
	if got := m.GetActiveAlerts(); len(got) != 1 || got[0].ID != newID || !got[0].StartTime.Equal(original.StartTime) || !got[0].Acknowledged {
		t.Fatalf("partial restart duplicated or reset migrated occurrence: %+v", got)
	}
	m.Stop()
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(target+".saved", target); err != nil {
		t.Fatal(err)
	}
	m = durabilityManager(t, dir, nil)
	defer m.Stop()
	got := m.GetActiveAlerts()
	if len(got) != 1 || got[0].ID != newID || !got[0].StartTime.Equal(original.StartTime) || !got[0].Acknowledged {
		t.Fatalf("healthy replay duplicated or reset migrated occurrence: %+v", got)
	}
	moved, err := m.AlertEvents(eventlog.Filter{Types: []string{eventlog.TypeActiveMoved}})
	if err != nil || len(moved) != 1 || moved[0].Details["previousAlertId"] != oldID {
		t.Fatalf("identity move not retained exactly once: %+v %v", moved, err)
	}
	activations, err := m.AlertEvents(eventlog.Filter{AlertID: newID, Types: []string{eventlog.TypeFired, eventlog.TypeRefired, eventlog.TypeResolved}})
	if err != nil || len(activations) != 0 {
		t.Fatalf("identity move invented lifecycle transition: %+v %v", activations, err)
	}
}

func TestHistoryJournalPartialRepairPreservesCanonicalControlsAndNewObservations(t *testing.T) {
	dir := t.TempDir()
	m := durabilityManager(t, dir, nil)
	t.Cleanup(func() { m.Stop() })
	m.RaiseSystemAlert(SystemAlertInput{Type: "partial-observation", Level: AlertLevelWarning, Message: "old"})
	id := SystemAlertID("partial-observation")
	if err := m.AcknowledgeAlert(id, "operator"); err != nil {
		t.Fatal(err)
	}
	canonical := m.GetActiveAlerts()[0].Clone()
	m.RaiseSystemAlert(SystemAlertInput{Type: "stale-partial-entry", Level: AlertLevelWarning, Message: "resolved later"})
	var stale *Alert
	for _, alert := range m.GetActiveAlerts() {
		if alert.ID == SystemAlertID("stale-partial-entry") {
			stale = alert.Clone()
		}
	}
	m.ClearSystemAlert("stale-partial-entry")
	boundary := m.eventLogStore().JournalBoundary()
	m.Stop()
	db, err := sql.Open("sqlite", filepath.Join(dir, "alerts", "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM alert_events`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	partial := canonical.Clone()
	partial.Level, partial.Message, partial.LastSeen = AlertLevelCritical, "new observation", time.Now().UTC()
	partial.Metadata[systemAlertFingerprintKey] = "new"
	partial.Acknowledged, partial.AckTime, partial.AckUser = false, nil, ""
	notified := time.Now().UTC()
	partial.LastNotified = &notified
	m.recoveryMirrorMu.Lock()
	floor := eventlog.JournalBoundary{ID: boundary.ID}
	m.activeRecoveryJournalFloor = &floor
	err = m.writeActiveStateDegradedRecovery([]*Alert{partial, stale}, fmt.Errorf("partial prior boot"), boundary)
	m.recoveryMirrorMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	for boot := 0; boot < 2; boot++ {
		m = durabilityManager(t, dir, nil)
		active := m.GetActiveAlerts()
		if len(active) != 1 || active[0].ID != id || !active[0].Acknowledged || active[0].AckUser != "operator" || active[0].Level != AlertLevelCritical || active[0].Metadata[systemAlertFingerprintKey] != "new" || active[0].LastNotified == nil || !active[0].LastNotified.Equal(notified) {
			t.Fatalf("partial recovery replaced authority or lost observations: %+v", active)
		}
		if !m.activeStateAuthoritative.Load() || m.activeStateDegraded() {
			t.Fatal("complete repair remained unavailable")
		}
		m.Stop()
	}
}

func TestHistoryJournalPartialFirstBaselineRecoversAllReadableFacts(t *testing.T) {
	for _, scenario := range []struct {
		name                     string
		canonical, newOccurrence bool
		controlHistory           bool
	}{
		{name: "empty"},
		{name: "new-occurrence", newOccurrence: true},
		{name: "pruned-canonical-and-new", canonical: true, newOccurrence: true},
		{name: "retained-old-control", canonical: true, controlHistory: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			dir := t.TempDir()
			alertsDir := filepath.Join(dir, "alerts")
			if err := os.MkdirAll(alertsDir, 0700); err != nil {
				t.Fatal(err)
			}
			store, err := eventlog.Open(alertsDir)
			if err != nil {
				t.Fatal(err)
			}
			canonicalID := "pruned-first-baseline"
			if scenario.canonical {
				now := time.Now().UTC()
				canonical := &Alert{ID: canonicalID, Type: "system", Level: AlertLevelWarning, StartTime: now, LastSeen: now, Acknowledged: true, AckUser: "canonical-operator", AckTime: &now}
				data, err := json.Marshal(canonical)
				if err != nil {
					t.Fatal(err)
				}
				if err := store.AppendDurable(eventlog.Event{Type: eventlog.TypeFired, AlertID: canonicalID, Snapshot: data}); err != nil {
					t.Fatal(err)
				}
				if scenario.controlHistory {
					if err := store.AppendDurable(eventlog.Event{Type: eventlog.TypeAcknowledged, AlertID: canonicalID, Snapshot: data}); err != nil {
						t.Fatal(err)
					}
					// The fallback has the old acknowledgement. SQLite's current
					// unacknowledgement outlives its own history row.
					if err := os.WriteFile(filepath.Join(alertsDir, "active-alerts.json"), append(append([]byte{'['}, data...), ']'), 0600); err != nil {
						t.Fatal(err)
					}
					canonical.Acknowledged, canonical.AckUser, canonical.AckTime = false, "", nil
					data, err = json.Marshal(canonical)
					if err != nil {
						t.Fatal(err)
					}
					if err := store.AppendDurable(eventlog.Event{Type: eventlog.TypeUnacknowledged, AlertID: canonicalID, Snapshot: data}); err != nil {
						t.Fatal(err)
					}
				}
				db, err := sql.Open("sqlite", filepath.Join(alertsDir, "events.db"))
				if err != nil {
					t.Fatal(err)
				}
				statement := `DELETE FROM alert_events`
				if scenario.controlHistory {
					statement += ` WHERE event_type = 'unacknowledged'`
				}
				if _, err := db.Exec(statement); err != nil {
					t.Fatal(err)
				}
				db.Close()
			}
			store.Close() // No complete active checkpoint has ever been established.
			target := filepath.Join(alertsDir, "events.db")
			if err := os.Rename(target, target+".saved"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(target, 0700); err != nil {
				t.Fatal(err)
			}
			m := durabilityManager(t, dir, nil)
			t.Cleanup(func() { m.Stop() })
			if scenario.newOccurrence {
				m.RaiseSystemAlert(SystemAlertInput{Type: "known-partial-occurrence", Level: AlertLevelWarning, Message: "known"})
			}
			m.Stop()
			if !m.activeRecoveryMarkerIncomplete() {
				t.Fatal("unreadable first baseline was not labelled partial")
			}
			if err := os.Remove(target); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(target+".saved", target); err != nil {
				t.Fatal(err)
			}
			want := 0
			if scenario.canonical {
				want++
			}
			if scenario.newOccurrence {
				want++
			}
			for boot := 0; boot < 2; boot++ {
				m = durabilityManager(t, dir, nil)
				if !m.activeStateAuthoritative.Load() || len(m.GetActiveAlerts()) != want {
					t.Fatalf("readable positive facts failed to recover on boot %d: authority=%t active=%d want=%d", boot, m.activeStateAuthoritative.Load(), len(m.GetActiveAlerts()), want)
				}
				if complete, err := m.eventLogStore().ActiveCheckpointComplete(); err != nil || !complete {
					t.Fatalf("full union checkpoint not established: %t %v", complete, err)
				}
				if scenario.canonical {
					found := false
					for _, alert := range m.GetActiveAlerts() {
						if alert.ID == canonicalID {
							found = alert.Acknowledged == !scenario.controlHistory
							if !scenario.controlHistory {
								found = found && alert.AckUser == "canonical-operator"
							}
						}
					}
					if !found {
						t.Fatal("pruned canonical occurrence or its controls were erased")
					}
				}
				m.Stop()
			}
		})
	}
}

func TestHistoryJournalFirstCheckpointFailureRecoversCompleteFallback(t *testing.T) {
	dir := t.TempDir()
	alertsDir := filepath.Join(dir, "alerts")
	if err := os.MkdirAll(alertsDir, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := eventlog.Open(alertsDir)
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	db, err := sql.Open("sqlite", filepath.Join(alertsDir, "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TRIGGER first_checkpoint_fault BEFORE INSERT ON alert_store_meta WHEN NEW.key = 'active_state_checkpoint_complete' BEGIN SELECT RAISE(ABORT,'first full checkpoint fault'); END`); err != nil {
		t.Fatal(err)
	}
	m := durabilityManager(t, dir, nil)
	t.Cleanup(func() { m.Stop() })
	if m.activeStateAuthoritative.Load() {
		t.Fatal("faulted first checkpoint claimed authority")
	}
	m.RaiseSystemAlert(SystemAlertInput{Type: "first-checkpoint-recovery", Level: AlertLevelWarning, Message: "retained"})
	if err := m.eventLogStore().Settle(); err != nil {
		t.Fatal(err)
	}
	if initialized, err := m.eventLogStore().ActiveStateInitialized(); err != nil || !initialized {
		t.Fatalf("fault did not reach initialized-only state: %t %v", initialized, err)
	}
	if complete, err := m.eventLogStore().ActiveCheckpointComplete(); err != nil || complete {
		t.Fatalf("fault unexpectedly established a full checkpoint: %t %v", complete, err)
	}
	m.Stop()
	if m.activeRecoveryMarkerIncomplete() {
		t.Fatal("failed storage checkpoint turned a known complete baseline into a partial one")
	}
	if _, err := db.Exec(`DROP TRIGGER first_checkpoint_fault`); err != nil {
		t.Fatal(err)
	}
	for boot := 0; boot < 2; boot++ {
		m = durabilityManager(t, dir, nil)
		if !m.activeStateAuthoritative.Load() || len(m.GetActiveAlerts()) != 1 {
			t.Fatalf("complete fallback remained stuck on boot %d: authority=%t active=%d", boot, m.activeStateAuthoritative.Load(), len(m.GetActiveAlerts()))
		}
		if complete, err := m.eventLogStore().ActiveCheckpointComplete(); err != nil || !complete {
			t.Fatalf("repaired full checkpoint not established: %t %v", complete, err)
		}
		m.Stop()
	}
}

func TestHistoryJournalInitializedOnlyImportsTrustedMirrorAndRetainedFacts(t *testing.T) {
	dir := t.TempDir()
	alertsDir := filepath.Join(dir, "alerts")
	if err := os.MkdirAll(alertsDir, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := eventlog.Open(alertsDir)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	mirror := &Alert{ID: "full-mirror-occurrence", Type: "system", Level: AlertLevelWarning, StartTime: start, LastSeen: start}
	data, err := json.Marshal([]*Alert{mirror})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(alertsDir, "active-alerts.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	fact := &Alert{ID: "retained-only-occurrence", Type: "system", Level: AlertLevelWarning, StartTime: start, LastSeen: start}
	data, err = json.Marshal(fact)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendDurable(eventlog.Event{Type: eventlog.TypeFired, AlertID: fact.ID, Snapshot: data}); err != nil {
		t.Fatal(err)
	}
	store.Close()
	m := durabilityManager(t, dir, nil)
	defer m.Stop()
	if !m.activeStateAuthoritative.Load() || len(m.GetActiveAlerts()) != 2 {
		t.Fatalf("initialized-only projection displaced trusted baseline or retained fact: authority=%t active=%d", m.activeStateAuthoritative.Load(), len(m.GetActiveAlerts()))
	}
}
