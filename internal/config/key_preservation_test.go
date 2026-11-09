package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConfigInitializationCorruptKeyPreservesSavedAccess(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), "missing-key"))
	cp, err := newConfigPersistence(dir)
	if err != nil {
		t.Fatal(err)
	}
	token, err := NewAPITokenRecord("synthetic-saved-reader-token", "Saved reader", []string{ScopeMonitoringRead})
	if err != nil {
		t.Fatal(err)
	}
	token.OrgID = "saved-workspace"
	if err := cp.SaveAPITokens([]APITokenRecord{*token}); err != nil {
		t.Fatal(err)
	}
	wantTokens, err := cp.LoadAPITokens()
	if err != nil {
		t.Fatal(err)
	}
	wantedSchedules := NormalizeReportScheduleStore(ReportScheduleStore{Schedules: []ReportSchedule{{
		ID: "saved-schedule", Name: "Saved monitoring report", Enabled: true,
		Cadence:  ReportScheduleCadence{Type: ReportScheduleCadenceWeekly, Weekday: "monday", Time: "08:00", Timezone: "UTC"},
		Scope:    ReportScheduleScope{Resources: []ReportScheduleResource{{ResourceType: "host", ResourceID: "saved-host"}}},
		Delivery: ReportScheduleDelivery{Method: ReportScheduleDeliveryDisk, SaveToDisk: true}, Format: ReportScheduleFormatCSV,
	}}})
	if err := cp.SaveReportScheduleStore(wantedSchedules); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, ".encryption.key")
	originalKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := make(map[string][]byte)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == ".encryption.key" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		snapshot[entry.Name()] = raw
	}
	invalidKey := []byte("truncated-synthetic-key")
	if err := os.WriteFile(keyPath, invalidKey, 0600); err != nil {
		t.Fatal(err)
	}
	failed, err := newConfigPersistence(dir)
	if failed != nil || err == nil || !strings.Contains(err.Error(), "refusing to replace") {
		t.Errorf("corrupt key must fail before a writable config store is returned: store=%t error=%v", failed != nil, err)
	}
	if got, err := os.ReadFile(keyPath); err != nil || !bytes.Equal(got, invalidKey) {
		t.Errorf("config initialization replaced invalid key: %v", err)
	}
	for name, original := range snapshot {
		if got, err := os.ReadFile(filepath.Join(dir, name)); err != nil || !bytes.Equal(got, original) {
			t.Errorf("config initialization changed saved %s: %v", name, err)
		}
	}
	after, err := os.ReadDir(dir)
	if err != nil || len(after) != len(entries) {
		t.Errorf("config initialization added or removed saved files: %v", err)
	}

	if err := os.WriteFile(keyPath, originalKey, 0600); err != nil {
		t.Fatal(err)
	}
	recovered, err := newConfigPersistence(dir)
	if err != nil {
		t.Fatalf("original-key restoration did not recover config initialization: %v", err)
	}
	gotTokens, err := recovered.LoadAPITokens()
	if err != nil || !reflect.DeepEqual(gotTokens, wantTokens) {
		t.Errorf("saved least-privilege token identity did not recover unchanged: %v", err)
	}
	gotSchedules, err := recovered.LoadReportScheduleStore()
	if err != nil || gotSchedules == nil || !reflect.DeepEqual(*gotSchedules, wantedSchedules) {
		t.Errorf("saved schedule did not recover unchanged: %v", err)
	}
	for name, original := range snapshot {
		if got, err := os.ReadFile(filepath.Join(dir, name)); err != nil || !bytes.Equal(got, original) {
			t.Errorf("recovered read changed saved %s: %v", name, err)
		}
	}
}
