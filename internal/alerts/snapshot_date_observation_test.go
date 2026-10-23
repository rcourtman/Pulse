package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func snapshotDateManager(t *testing.T) *Manager {
	t.Helper()
	m := newEventLogManager(t)
	cfg := m.GetConfig()
	cfg.SnapshotDefaults = SnapshotAlertConfig{Enabled: true, WarningDays: 7, CriticalDays: 14, WarningSizeGiB: 50, CriticalSizeGiB: 100}
	cfg.TimeThresholds = map[string]int{}
	m.UpdateConfig(cfg)
	return m
}

func snapshotDateFixture(kind string) (models.GuestSnapshot, map[string]GuestLookup) {
	s := models.GuestSnapshot{ID: "east-pve-101-weekly", Name: "weekly", Instance: "east", Node: "pve", VMID: 101, Type: kind, Time: time.Now().Add(-16 * 24 * time.Hour), SizeBytes: 120 << 30}
	g := GuestLookup{ResourceID: "east:pve:101", Name: "database", Instance: "east", Node: "pve", VMID: 101, Type: kind}
	return s, map[string]GuestLookup{g.ResourceID: g}
}

func snapshotDateJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSnapshotDateObservations(t *testing.T) {
	invalid := map[string]time.Time{
		"zero": {}, "epoch": time.Unix(0, 0), "negative": time.Unix(-1, 0), "future": time.Now().Add(24 * time.Hour),
	}
	for _, kind := range []string{"qemu", "lxc"} {
		for _, metric := range []string{"age", "size", "both"} {
			for label, date := range invalid {
				t.Run(kind+"/"+metric+"/"+label, func(t *testing.T) {
					m := snapshotDateManager(t)
					s, guests := snapshotDateFixture(kind)
					if metric == "size" {
						s.Time = time.Now().Add(-time.Hour)
					}
					if metric == "age" {
						s.SizeBytes = 1 << 30
					}
					m.CheckSnapshotsForInstance("east", []models.GuestSnapshot{s}, guests)
					active := m.GetActiveAlerts()
					if len(active) != 1 || active[0].Level != AlertLevelCritical {
						t.Fatalf("control must establish one critical incident: %+v", active)
					}
					if err := m.AcknowledgeAlert(active[0].ID, "operator"); err != nil {
						t.Fatal(err)
					}
					before := snapshotDateJSON(t, m.GetActiveAlerts())
					s.Time, s.SizeBytes = date, 0
					for range 4 {
						m.CheckSnapshotsForInstance("east", []models.GuestSnapshot{s}, guests)
					}
					if after := snapshotDateJSON(t, m.GetActiveAlerts()); !bytes.Equal(before, after) {
						t.Fatalf("unknown date changed occurrence/evidence/acknowledgement: before=%s after=%s", before, after)
					}
					if got := len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeFired, eventlog.TypeRefired, eventlog.TypeResolved}})); got != 1 {
						t.Fatalf("unknown date invented a lifecycle transition: events=%d", got)
					}
					s.Time, s.SizeBytes = time.Now().Add(-time.Hour), 1<<30
					m.CheckSnapshotsForInstance("east", []models.GuestSnapshot{s}, guests)
					m.CheckSnapshotsForInstance("east", []models.GuestSnapshot{s}, guests)
					if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeResolved}})) != 1 {
						t.Fatal("trustworthy normal observation must resolve exactly once")
					}
				})
			}
		}
	}

	t.Run("never-observed", func(t *testing.T) {
		m := snapshotDateManager(t)
		s, guests := snapshotDateFixture("qemu")
		for _, date := range invalid {
			s.Time = date
			m.CheckSnapshotsForInstance("east", []models.GuestSnapshot{s}, guests)
		}
		if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeFired, eventlog.TypeResolved}})) != 0 {
			t.Fatal("an unknown date cannot invent snapshot age or a lifecycle event")
		}
	})

	for _, policy := range []string{"global-disabled", "guest-disabled", "snapshot-disabled", "override-disabled", "threshold-raised", "no-alerts-tag", "removed"} {
		t.Run("policy/"+policy, func(t *testing.T) {
			m := snapshotDateManager(t)
			s, guests := snapshotDateFixture("qemu")
			m.CheckSnapshotsForInstance("east", []models.GuestSnapshot{s}, guests)
			if len(m.GetActiveAlerts()) != 1 {
				t.Fatal("control must establish an incident")
			}
			s.Time = time.Time{}
			snapshots := []models.GuestSnapshot{s}
			cfg := m.GetConfig()
			switch policy {
			case "global-disabled":
				cfg.Enabled = false
			case "guest-disabled":
				cfg.Overrides = map[string]ThresholdConfig{"east:pve:101": {Disabled: true}}
			case "snapshot-disabled":
				cfg.SnapshotDefaults.Enabled = false
			case "override-disabled":
				cfg.Overrides = map[string]ThresholdConfig{"east:pve:101": {Snapshot: &SnapshotAlertConfig{Enabled: false}}}
			case "threshold-raised":
				cfg.SnapshotDefaults = SnapshotAlertConfig{Enabled: true, WarningDays: 30, CriticalDays: 60, WarningSizeGiB: 200, CriticalSizeGiB: 400}
			case "no-alerts-tag":
				g := guests["east:pve:101"]
				g.Tags = []string{"pulse-no-alerts"}
				guests[g.ResourceID] = g
			case "removed":
				snapshots = nil
			}
			m.UpdateConfig(cfg)
			m.CheckSnapshotsForInstance("east", snapshots, guests)
			if len(m.GetActiveAlerts()) != 0 {
				t.Fatal("unknown date overrode explicit policy or authoritative removal")
			}
		})
	}

	t.Run("independent-instances-and-snapshots", func(t *testing.T) {
		m := snapshotDateManager(t)
		s, guests := snapshotDateFixture("qemu")
		other := s
		other.ID, other.Name = "east-pve-101-daily", "daily"
		west := s
		west.ID, west.Instance = "west-pve-101-weekly", "west"
		g := guests["east:pve:101"]
		g.ResourceID, g.Instance, g.Name = "west:pve:101", "west", "west-database"
		guests[g.ResourceID] = g
		m.CheckSnapshotsForInstance("", []models.GuestSnapshot{s, other, west}, guests)
		if len(m.GetActiveAlerts()) != 3 {
			t.Fatal("control must establish three independent incidents")
		}
		s.Time = time.Time{}
		// One same-guest snapshot is removed and the other instance recovers.
		m.CheckSnapshotsForInstance("east", []models.GuestSnapshot{s}, guests)
		if len(m.GetActiveAlerts()) != 2 {
			t.Fatal("unknown date protected a removed sibling or cleared another instance")
		}
		west.Time, west.SizeBytes = time.Now().Add(-time.Hour), 1<<30
		m.CheckSnapshotsForInstance("west", []models.GuestSnapshot{west}, guests)
		active := m.GetActiveAlerts()
		if len(active) != 1 || active[0].Instance != "east" || active[0].Metadata["snapshotName"] != "weekly" {
			t.Fatalf("unknown date leaked across independent snapshot identity: %+v", active)
		}
	})
}

// Actual manager output is the only alert payload used by the browser fixture.
func TestSnapshotDateBrowserProducer(t *testing.T) {
	m := snapshotDateManager(t)
	s, guests := snapshotDateFixture("qemu")
	frames := map[string]json.RawMessage{}
	for _, phase := range []string{"stale", "missing", "future", "recovered"} {
		switch phase {
		case "missing":
			s.Time = time.Unix(0, 0) // real absent-snaptime conversion
		case "future":
			s.Time = time.Now().Add(24 * time.Hour)
		case "recovered":
			s.Time, s.SizeBytes = time.Now().Add(-time.Hour), 1<<30
		}
		m.CheckSnapshotsForInstance("east", []models.GuestSnapshot{s}, guests)
		frames[phase] = snapshotDateJSON(t, m.GetActiveAlerts())
		want := 1
		if phase == "recovered" {
			want = 0
		}
		if len(m.GetActiveAlerts()) != want {
			t.Fatalf("phase %s: want %d active alerts", phase, want)
		}
	}
	if !bytes.Equal(frames["stale"], frames["missing"]) || !bytes.Equal(frames["stale"], frames["future"]) {
		t.Fatal("browser frames changed the retained occurrence across a date gap")
	}
	if os.Getenv("PULSE_SNAPSHOT_DATE_FRAMES") == "1" {
		fmt.Printf("SNAPSHOT_DATE_FRAMES=%s\n", snapshotDateJSON(t, frames))
	}
}
