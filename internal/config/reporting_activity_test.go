package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestReportingActivityMonthlyReuseSurvivesRestartAndDeduplicates(t *testing.T) {
	dir := t.TempDir()
	p := config.NewConfigPersistence(dir)
	first := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	key, period := strings.Repeat("a", 64), strings.Repeat("b", 64)
	event := config.ReportingActivityEvent{At: first, Kind: config.ReportScheduleKindBackupHealth, Event: "successful_period", ScheduleKey: key, PeriodKey: period}
	for i := 0; i < 3; i++ {
		if err := p.RecordReportingActivity(event); err != nil {
			t.Fatal(err)
		}
	}
	p = config.NewConfigPersistence(dir)
	event.At = first.AddDate(0, 1, 0)
	event.PeriodKey = strings.Repeat("c", 64)
	for i := 0; i < 3; i++ {
		if err := p.RecordReportingActivity(event); err != nil {
			t.Fatal(err)
		}
	}
	got, err := p.ReportingActivitySnapshot(event.At)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || got.ObservationDays != 30 || got.Counts.Successful[2] != 1 || got.Counts.Repeated[2] != 1 {
		t.Fatalf("snapshot: %+v", got)
	}
	// Beyond marker retention, the old schedule is no longer repeat evidence.
	event.At = event.At.AddDate(0, 0, 91)
	event.PeriodKey = strings.Repeat("d", 64)
	if err := p.RecordReportingActivity(event); err != nil {
		t.Fatal(err)
	}
	got, err = p.ReportingActivitySnapshot(event.At)
	if err != nil || got.Counts.Successful[2] != 1 || got.Counts.Repeated[2] != 0 {
		t.Fatalf("expired repeat: %+v %v", got, err)
	}
	info, err := os.Stat(filepath.Join(dir, "reporting_activity.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissions: %v %v", info, err)
	}
}

func TestReportingActivityCountsOutOfOrderUTCCompletionsAndConcurrentEvents(t *testing.T) {
	p := config.NewConfigPersistence(t.TempDir())
	now := time.Date(2026, 10, 17, 0, 0, 1, 0, time.UTC)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			at := now
			if i%2 == 0 {
				at = now.Add(-2 * time.Second)
			}
			if err := p.RecordReportingActivity(config.ReportingActivityEvent{At: at, Kind: config.ReportScheduleKindResources, Event: "generated"}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	got, err := p.ReportingActivitySnapshot(now)
	if err != nil || got.Counts.Generated[0] != 16 || got.ObservationDays != 2 || got.ActiveDays[0] != 2 {
		t.Fatalf("out-of-order counts: %+v %v", got, err)
	}
	got, err = p.ReportingActivitySnapshot(now.AddDate(0, 0, 29))
	if err != nil || got.Counts.Generated[0] != 8 || got.ActiveDays[0] != 1 {
		t.Fatalf("UTC retention: %+v %v", got, err)
	}
}

func TestReportingActivityAvailabilityClosedFieldsAndOutcomes(t *testing.T) {
	dir := t.TempDir()
	p := config.NewConfigPersistence(dir)
	now := time.Now().UTC()
	empty, err := p.ReportingActivitySnapshot(now)
	if err != nil || !empty.Available || empty.ObservationDays != 1 {
		t.Fatalf("empty observed: %+v %v", empty, err)
	}
	for _, event := range []string{"saved", "email_accepted", "delivery_failed", "delivery_unconfirmed"} {
		if err := p.RecordReportingActivity(config.ReportingActivityEvent{At: now, Kind: config.ReportScheduleKindVMInventory, Event: event}); err != nil {
			t.Fatal(err)
		}
	}
	for _, state := range []string{"complete", "partial", "unavailable", "private-content"} {
		if err := p.RecordReportingActivity(config.ReportingActivityEvent{At: now, Kind: config.ReportScheduleKindBackupHealth, Event: "generated", DataState: state}); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.RecordReportingActivity(config.ReportingActivityEvent{At: now, Kind: "private-customer", Event: "generated"}); err == nil {
		t.Fatal("accepted arbitrary kind")
	}
	if err := p.RecordReportingActivity(config.ReportingActivityEvent{At: now, Kind: config.ReportScheduleKindResources, Event: "private-error"}); err == nil {
		t.Fatal("accepted arbitrary event")
	}
	if err := p.RecordReportingActivity(config.ReportingActivityEvent{At: now, Kind: config.ReportScheduleKindResources, Event: "successful_period", ScheduleKey: "customer", PeriodKey: "date"}); err == nil {
		t.Fatal("accepted raw identifiers")
	}
	got, err := p.ReportingActivitySnapshot(now)
	if err != nil || got.Counts.Generated[2] != 4 || got.Counts.DataComplete != 1 || got.Counts.DataIncomplete != 1 || got.Counts.DataUnavailable != 1 || got.Counts.Saved != 1 || got.Counts.EmailAccepted != 1 || got.Counts.DeliveryFailed != 1 || got.Counts.DeliveryUnconfirmed != 1 {
		t.Fatalf("outcomes: %+v %v", got, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "reporting_activity.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"private-content", "private-customer", "private-error"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("retained %q", forbidden)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "reporting_activity.json"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = p.ReportingActivitySnapshot(now)
	if err == nil || got.Available {
		t.Fatalf("corrupt is unmeasured: %+v %v", got, err)
	}
}

func TestReportingActivityIncompleteStoresRemainUnmeasured(t *testing.T) {
	for _, raw := range []string{"null", "{}", `{"version":1,"observed_since":"2026-10-17"}`, `{"version":1,"observed_since":"2026-10-17","daily":null,"periods":{}}`} {
		p := config.NewConfigPersistence(t.TempDir())
		if err := os.WriteFile(filepath.Join(p.GetConfigDir(), "reporting_activity.json"), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := p.ReportingActivitySnapshot(time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC))
		if err == nil || got.Available {
			t.Fatalf("%s must remain unmeasured: %+v %v", raw, got, err)
		}
	}
}
