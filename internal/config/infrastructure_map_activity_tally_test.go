package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

var infrastructureMapActivitiesUnderTest = []string{
	config.InfrastructureMapActivityOpen,
	config.InfrastructureMapActivityExport,
	config.InfrastructureMapActivityExportNamesHidden,
}

func TestInfrastructureMapActivityTallyCountsEachClosedMilestone(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()
	for i, activity := range infrastructureMapActivitiesUnderTest {
		for n := 0; n <= i; n++ {
			if err := persistence.RecordInfrastructureMapActivity(activity, now); err != nil {
				t.Fatalf("RecordInfrastructureMapActivity(%q) = %v", activity, err)
			}
		}
	}

	tally, err := persistence.LoadInfrastructureMapActivityTally()
	if err != nil {
		t.Fatalf("LoadInfrastructureMapActivityTally = %v", err)
	}
	since := now.AddDate(0, 0, -30)
	for i, activity := range infrastructureMapActivitiesUnderTest {
		if got := tally.CountSince(activity, since); got != i+1 {
			t.Fatalf("CountSince(%q) = %d, want %d", activity, got, i+1)
		}
	}
}

func TestInfrastructureMapActivityTallyRejectsUnknownActivity(t *testing.T) {
	dir := t.TempDir()
	persistence := config.NewConfigPersistence(dir)
	for _, activity := range []string{"", "pve-node-1", "192.168.1.10", "export_png", "Open"} {
		if err := persistence.RecordInfrastructureMapActivity(activity, time.Now().UTC()); err == nil {
			t.Fatalf("unknown activity %q was accepted", activity)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "infrastructure_map_activity_tally.json")); !os.IsNotExist(err) {
		t.Fatalf("a refused activity wrote the tally file: %v", err)
	}
}

func TestInfrastructureMapActivityTallyPrunesOutsideRetention(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()
	if err := persistence.RecordInfrastructureMapActivity(config.InfrastructureMapActivityOpen, now.AddDate(0, 0, -40)); err != nil {
		t.Fatalf("record old activity = %v", err)
	}
	if err := persistence.RecordInfrastructureMapActivity(config.InfrastructureMapActivityOpen, now); err != nil {
		t.Fatalf("record current activity = %v", err)
	}
	tally, err := persistence.LoadInfrastructureMapActivityTally()
	if err != nil {
		t.Fatalf("LoadInfrastructureMapActivityTally = %v", err)
	}
	if got := tally.CountSince(config.InfrastructureMapActivityOpen, time.Time{}); got != 1 {
		t.Fatalf("CountSince over all retained days = %d, want 1", got)
	}
}

// A tally file edited by hand, or written by a build that knew another
// milestone, must not carry an unknown key into a count or back to disk.
func TestInfrastructureMapActivityTallyDropsUnknownKeysFromDisk(t *testing.T) {
	dir := t.TempDir()
	today := time.Now().UTC().Format("2006-01-02")
	seeded := `{"version":1,"daily":{"open":{"` + today + `":2},"host-names":{"` + today + `":9}}}`
	path := filepath.Join(dir, "infrastructure_map_activity_tally.json")
	if err := os.WriteFile(path, []byte(seeded), 0o600); err != nil {
		t.Fatal(err)
	}
	persistence := config.NewConfigPersistence(dir)

	tally, err := persistence.LoadInfrastructureMapActivityTally()
	if err != nil {
		t.Fatalf("LoadInfrastructureMapActivityTally = %v", err)
	}
	if got := tally.CountSince(config.InfrastructureMapActivityOpen, time.Time{}); got != 2 {
		t.Fatalf("open count = %d, want 2", got)
	}
	if got := tally.CountSince("host-names", time.Time{}); got != 0 {
		t.Fatalf("unknown key counted %d", got)
	}

	if err := persistence.RecordInfrastructureMapActivity(config.InfrastructureMapActivityOpen, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "host-names") {
		t.Fatalf("unknown key survived a write: %s", raw)
	}
}
