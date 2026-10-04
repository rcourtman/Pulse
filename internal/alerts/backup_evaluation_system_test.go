package alerts

import "testing"

func TestBackupEvaluationSystemAlertUsesExistingIdentityAndDeduplication(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	defer m.Stop()
	input := SystemAlertInput{Type: BackupEvaluationAlertType, Level: AlertLevelWarning,
		Fingerprint: "recovery-rollups-unavailable", Message: "Backup-age alerts were not evaluated."}
	if !m.RaiseSystemAlert(input) {
		t.Fatal("first failure not raised")
	}
	if m.RaiseSystemAlert(input) {
		t.Fatal("repeated failure re-notified")
	}
	active := m.GetActiveAlerts()
	if len(active) != 1 || active[0].ID != SystemAlertID(BackupEvaluationAlertType) ||
		active[0].ResourceID != "" || active[0].Metadata["systemAlert"] != true {
		t.Fatalf("system projection=%+v", active)
	}
	m.CleanupAlertsForNodes(map[string]bool{})
	if len(m.GetActiveAlerts()) != 1 {
		t.Fatal("node cleanup erased the evaluator outage")
	}
	if !m.ClearSystemAlert(BackupEvaluationAlertType) || len(m.GetActiveAlerts()) != 0 {
		t.Fatal("recovery did not clear the warning")
	}
}
