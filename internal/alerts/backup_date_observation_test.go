package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func backupDateManager(t *testing.T) *Manager {
	t.Helper()
	m := newEventLogManager(t)
	cfg := m.GetConfig()
	cfg.BackupDefaults = BackupAlertConfig{Enabled: true, WarningDays: 7, CriticalDays: 14}
	cfg.TimeThresholds = map[string]int{}
	m.UpdateConfig(cfg)
	return m
}

func backupDateFixture(kind string) (recovery.ProtectionRollup, map[string]GuestLookup, map[string][]GuestLookup) {
	r := recovery.ProtectionRollup{RollupID: "res:date-" + kind,
		LastSuccessAt: ptrTime(time.Now().Add(-16 * 24 * time.Hour)), LastOutcome: recovery.OutcomeSuccess,
		SubjectRef: &recovery.ExternalRef{Type: "proxmox-vm", ID: "east:pve:101", Namespace: "east", Name: "database", Class: "pve"},
		Providers:  []recovery.Provider{recovery.ProviderProxmoxPVE}}
	guest := GuestLookup{ResourceID: "east:pve:101", Name: "database", Instance: "east", Node: "pve", VMID: 101, Type: "qemu"}
	guests := map[string]GuestLookup{guest.ResourceID: guest}
	byVMID := map[string][]GuestLookup{"101": {guest}}
	switch kind {
	case "pbs":
		r.Providers = []recovery.Provider{recovery.ProviderProxmoxPBS}
	case "orphan":
		r.Providers = []recovery.Provider{recovery.ProviderProxmoxPBS}
		r.SubjectRef.ID, r.SubjectRef.Namespace, r.SubjectRef.Name = "404", "pbs", "orphan-404"
		guests, byVMID = nil, nil
	case "pmg":
		r.Providers = []recovery.Provider{recovery.ProviderProxmoxPMG}
		r.SubjectRef = &recovery.ExternalRef{Type: "pmg", ID: "mail", Name: "mail-gateway"}
		guests, byVMID = nil, nil
	}
	return r, guests, byVMID
}

func backupDateJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func testBackupDateObservations(t *testing.T) {
	invalid := map[string]*time.Time{
		"missing": nil, "zero": ptrTime(time.Time{}), "future": ptrTime(time.Now().Add(24 * time.Hour)),
	}
	for _, kind := range []string{"pve", "pbs", "orphan", "pmg"} {
		for label, date := range invalid {
			t.Run(kind+"/"+label, func(t *testing.T) {
				m := backupDateManager(t)
				r, guests, byVMID := backupDateFixture(kind)
				m.CheckBackups([]recovery.ProtectionRollup{r}, guests, byVMID)
				active := m.GetActiveAlerts()
				if len(active) != 1 || active[0].Level != AlertLevelCritical {
					t.Fatalf("stale observation must establish one critical incident: %+v", active)
				}
				id := active[0].ID
				if err := m.AcknowledgeAlert(id, "operator"); err != nil {
					t.Fatal(err)
				}
				before := backupDateJSON(t, m.GetActiveAlerts())
				r.LastSuccessAt = date
				for range 4 {
					m.CheckBackups([]recovery.ProtectionRollup{r}, guests, byVMID)
				}
				if after := backupDateJSON(t, m.GetActiveAlerts()); !bytes.Equal(before, after) {
					t.Fatalf("unknown date changed occurrence/evidence/acknowledgement: before=%s after=%s", before, after)
				}
				if got := len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeFired, eventlog.TypeRefired, eventlog.TypeResolved}})); got != 1 {
					t.Fatalf("unknown date invented a lifecycle transition: events=%d", got)
				}
				r.LastSuccessAt = ptrTime(time.Now().Add(-time.Hour))
				m.CheckBackups([]recovery.ProtectionRollup{r}, guests, byVMID)
				m.CheckBackups([]recovery.ProtectionRollup{r}, guests, byVMID)
				if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeResolved}})) != 1 {
					t.Fatal("valid recent success must still resolve exactly once")
				}
			})
		}
	}

	t.Run("never-observed-does-not-fire", func(t *testing.T) {
		m := backupDateManager(t)
		r, guests, byVMID := backupDateFixture("pve")
		for _, date := range invalid {
			r.LastSuccessAt = date
			m.CheckBackups([]recovery.ProtectionRollup{r}, guests, byVMID)
		}
		if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeFired, eventlog.TypeResolved}})) != 0 {
			t.Fatal("an unobserved date cannot establish age or recovery")
		}
	})

	for _, recent := range []bool{false, true} {
		for label, date := range invalid {
			for _, reverse := range []bool{false, true} {
				t.Run(fmt.Sprintf("multiple-providers/recent=%t/%s/reverse=%t", recent, label, reverse), func(t *testing.T) {
					m := backupDateManager(t)
					r, guests, byVMID := backupDateFixture("pve")
					m.CheckBackups([]recovery.ProtectionRollup{r}, guests, byVMID)
					other := r
					other.RollupID, other.LastSuccessAt, other.Providers = "res:other-provider", date, []recovery.Provider{recovery.ProviderProxmoxPBS}
					if recent {
						r.LastSuccessAt = ptrTime(time.Now().Add(-time.Hour))
					}
					rollups := []recovery.ProtectionRollup{r, other}
					if reverse {
						rollups[0], rollups[1] = rollups[1], rollups[0]
					}
					m.CheckBackups(rollups, guests, byVMID)
					want := 1
					if recent {
						want = 0
					}
					if got := len(m.GetActiveAlerts()); got != want {
						t.Fatalf("invalid provider displaced the valid observation: active=%d want=%d", got, want)
					}
				})
			}
		}
	}

	for _, policy := range []string{"global-disabled", "guest-disabled", "backup-disabled", "ignored-vmid", "no-alerts-tag", "removed"} {
		t.Run("explicit-policy/"+policy, func(t *testing.T) {
			m := backupDateManager(t)
			r, guests, byVMID := backupDateFixture("pve")
			rollups := []recovery.ProtectionRollup{r}
			m.CheckBackups(rollups, guests, byVMID)
			if len(m.GetActiveAlerts()) != 1 {
				t.Fatal("control must start with a firing incident")
			}
			rollups[0].LastSuccessAt = nil
			cfg := m.GetConfig()
			switch policy {
			case "global-disabled":
				cfg.Enabled = false
			case "guest-disabled":
				cfg.Overrides = map[string]ThresholdConfig{"east:pve:101": {Disabled: true}}
			case "backup-disabled":
				cfg.BackupDefaults.Enabled = false
			case "ignored-vmid":
				cfg.BackupDefaults.IgnoreVMIDs = []string{"101"}
			case "no-alerts-tag":
				g := guests["east:pve:101"]
				g.Tags = []string{"pulse-no-alerts"}
				guests[g.ResourceID], byVMID["101"] = g, []GuestLookup{g}
			case "removed":
				rollups = nil
			}
			m.UpdateConfig(cfg)
			m.CheckBackups(rollups, guests, byVMID)
			if len(m.GetActiveAlerts()) != 0 {
				t.Fatal("missing date overrode explicit disablement, exclusion or authoritative removal")
			}
		})
	}

	t.Run("same-vmid-other-instance-recovers", func(t *testing.T) {
		m := backupDateManager(t)
		r, guests, byVMID := backupDateFixture("pve")
		other := r
		other.RollupID, other.SubjectRef = "res:west", &recovery.ExternalRef{Type: "proxmox-vm", ID: "west:pve:101", Name: "west-database", Namespace: "west", Class: "pve"}
		g := guests["east:pve:101"]
		g.ResourceID, g.Name, g.Instance = "west:pve:101", "west-database", "west"
		guests[g.ResourceID] = g
		byVMID["101"] = append(byVMID["101"], g)
		m.CheckBackups([]recovery.ProtectionRollup{r, other}, guests, byVMID)
		if len(m.GetActiveAlerts()) != 2 {
			t.Fatal("both instances must first establish independent incidents")
		}
		r.LastSuccessAt, other.LastSuccessAt = nil, ptrTime(time.Now().Add(-time.Hour))
		m.CheckBackups([]recovery.ProtectionRollup{r, other}, guests, byVMID)
		active := m.GetActiveAlerts()
		if len(active) != 1 || active[0].ResourceID != "east:pve:101" {
			t.Fatalf("unknown date prevented an independent recovery or leaked across instances: %+v", active)
		}
	})

	t.Run("browser-producer", testBackupDateBrowserProducer)
}

// The browser consumes these actual CheckBackups/GetActiveAlerts results, not
// independently invented alert props. Opt-in emission keeps normal CI concise.
func testBackupDateBrowserProducer(t *testing.T) {
	m := backupDateManager(t)
	r, guests, byVMID := backupDateFixture("pve")
	frames := map[string]json.RawMessage{}
	for _, phase := range []string{"stale", "missing", "future", "recovered"} {
		switch phase {
		case "missing":
			r.LastSuccessAt = nil
		case "future":
			r.LastSuccessAt = ptrTime(time.Now().Add(24 * time.Hour))
		case "recovered":
			r.LastSuccessAt = ptrTime(time.Now().Add(-time.Hour))
		}
		m.CheckBackups([]recovery.ProtectionRollup{r}, guests, byVMID)
		frames[phase] = backupDateJSON(t, m.GetActiveAlerts())
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
	if os.Getenv("PULSE_BACKUP_DATE_FRAMES") == "1" {
		fmt.Printf("BACKUP_DATE_FRAMES=%s\n", backupDateJSON(t, frames))
	}
}
