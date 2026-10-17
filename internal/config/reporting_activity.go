package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const reportingActivityFile = "reporting_activity.json"
const reportingActivityLimit = 100000

// The array order is fixed by ReportingActivityKindIndex, never by user input.
// Daily buckets retain counts only. Opaque period markers stay local and use
// the same encryption boundary as report schedule configuration.
type ReportingActivityCounts struct {
	Generated           [4]int `json:"generated"`
	Successful          [4]int `json:"successful_periods"`
	Repeated            [4]int `json:"repeat_periods"`
	Saved               int    `json:"saved"`
	EmailAccepted       int    `json:"email_accepted"`
	DeliveryFailed      int    `json:"delivery_failed"`
	DeliveryUnconfirmed int    `json:"delivery_unconfirmed"`
	DataComplete        int    `json:"data_complete"`
	DataIncomplete      int    `json:"data_incomplete"`
	DataUnavailable     int    `json:"data_unavailable"`
}

type ReportingActivitySummary struct {
	ActiveDays      [4]int
	Available       bool
	ObservationDays int
	Counts          ReportingActivityCounts
}

type ReportingActivityEvent struct {
	At        time.Time
	Kind      string
	Event     string
	DataState string
	// Only SHA-256 tokens are accepted here. No raw schedule/tenant identifier,
	// report period, recipient, scope or contents are retained in this tally.
	ScheduleKey string
	PeriodKey   string
}

type reportingActivityStore struct {
	Version       int                                `json:"version"`
	ObservedSince string                             `json:"observed_since"`
	Daily         map[string]ReportingActivityCounts `json:"daily"`
	Periods       map[string]map[string]string       `json:"periods"`
}

var reportingActivityToken = regexp.MustCompile(`^[a-f0-9]{64}$`)

func ReportingActivityKindIndex(kind string) (int, bool) {
	switch kind {
	case ReportScheduleKindResources:
		return 0, true
	case ReportScheduleKindVMInventory:
		return 1, true
	case ReportScheduleKindBackupHealth:
		return 2, true
	case ReportScheduleKindPatrolDigest:
		return 3, true
	default:
		return 0, false
	}
}

func (c *ConfigPersistence) loadReportingActivityLocked(now time.Time) (*reportingActivityStore, error) {
	store := &reportingActivityStore{Version: 1, ObservedSince: now.UTC().Format(time.DateOnly), Daily: map[string]ReportingActivityCounts{}, Periods: map[string]map[string]string{}}
	raw, err := c.fs.ReadFile(filepath.Join(c.configDir, reportingActivityFile))
	if os.IsNotExist(err) {
		return store, nil
	}
	if err != nil {
		return nil, err
	}
	if c.crypto != nil {
		raw, err = c.crypto.Decrypt(raw)
		if err != nil {
			return nil, fmt.Errorf("decrypt reporting activity: %w", err)
		}
	}
	// Defaults apply only to a genuinely absent file. An incomplete existing
	// file must not become apparently measured zero usage.
	store = &reportingActivityStore{}
	if err := json.Unmarshal(raw, store); err != nil {
		return nil, err
	}
	if store.Version != 1 {
		return nil, fmt.Errorf("unsupported reporting activity version")
	}
	if _, err := time.Parse(time.DateOnly, store.ObservedSince); err != nil {
		return nil, fmt.Errorf("invalid reporting observation day")
	}
	if store.Daily == nil || store.Periods == nil {
		return nil, fmt.Errorf("incomplete reporting activity store")
	}
	return store, nil
}

func pruneReportingActivity(store *reportingActivityStore, now time.Time) {
	day := now.UTC().Format(time.DateOnly)
	first := now.UTC().AddDate(0, 0, -29).Format(time.DateOnly)
	previous := now.UTC().AddDate(0, 0, -90).Format(time.DateOnly)
	if store.ObservedSince > day {
		store.ObservedSince = day
	}
	for date := range store.Daily {
		if _, err := time.Parse(time.DateOnly, date); err != nil || date < first || date > day {
			delete(store.Daily, date)
		}
	}
	for key, periods := range store.Periods {
		if !reportingActivityToken.MatchString(key) {
			delete(store.Periods, key)
			continue
		}
		for period, date := range periods {
			if _, err := time.Parse(time.DateOnly, date); err != nil || date < previous || date > day || !reportingActivityToken.MatchString(period) {
				delete(periods, period)
			}
		}
		if len(periods) == 0 {
			delete(store.Periods, key)
		}
	}
}

func (c *ConfigPersistence) saveReportingActivityLocked(store *reportingActivityStore) error {
	raw, err := json.Marshal(store)
	if err != nil {
		return err
	}
	if c.crypto != nil {
		raw, err = c.crypto.Encrypt(raw)
		if err != nil {
			return err
		}
	}
	if err := c.fs.MkdirAll(c.configDir, 0700); err != nil {
		return err
	}
	return c.writeConfigFileLocked(filepath.Join(c.configDir, reportingActivityFile), raw, 0600)
}

func reportingCountIncrement(value *int) {
	if *value < 0 {
		*value = 0
	}
	if *value < reportingActivityLimit {
		*value++
	}
}

// RecordReportingActivity stores only closed kinds/outcomes and local opaque
// deduplication tokens. Repeated successes require another distinct reporting
// period on the same saved schedule, observed in the preceding 90 days. This
// allows monthly schedules to qualify without mistaking retries for reuse.
func (c *ConfigPersistence) RecordReportingActivity(event ReportingActivityEvent) error {
	if c == nil || c.configDir == "" || event.At.IsZero() {
		return nil
	}
	index, ok := ReportingActivityKindIndex(event.Kind)
	if !ok {
		return fmt.Errorf("unrecognized reporting activity kind")
	}
	switch event.Event {
	case "generated", "saved", "email_accepted", "delivery_failed", "delivery_unconfirmed", "successful_period":
	default:
		return fmt.Errorf("unrecognized reporting activity outcome")
	}
	if event.Event == "successful_period" && (!reportingActivityToken.MatchString(event.ScheduleKey) || !reportingActivityToken.MatchString(event.PeriodKey)) {
		return fmt.Errorf("invalid reporting period marker")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	store, err := c.loadReportingActivityLocked(event.At)
	if err != nil {
		return err
	}
	// Concurrent completions can arrive out of order across midnight. Prune
	// against the newest retained day rather than deleting a later completion.
	anchor := event.At.UTC()
	for day := range store.Daily {
		if retained, err := time.Parse(time.DateOnly, day); err == nil && retained.After(anchor) {
			anchor = retained
		}
	}
	pruneReportingActivity(store, anchor)
	day := event.At.UTC().Format(time.DateOnly)
	if day < anchor.AddDate(0, 0, -29).Format(time.DateOnly) {
		return nil
	}
	if day < store.ObservedSince {
		store.ObservedSince = day
	}
	counts := store.Daily[day]
	switch event.Event {
	case "generated":
		reportingCountIncrement(&counts.Generated[index])
		switch event.DataState {
		case "complete":
			reportingCountIncrement(&counts.DataComplete)
		case "partial", "incomplete":
			reportingCountIncrement(&counts.DataIncomplete)
		case "unavailable":
			reportingCountIncrement(&counts.DataUnavailable)
		}
	case "saved":
		reportingCountIncrement(&counts.Saved)
	case "email_accepted":
		reportingCountIncrement(&counts.EmailAccepted)
	case "delivery_failed":
		reportingCountIncrement(&counts.DeliveryFailed)
	case "delivery_unconfirmed":
		reportingCountIncrement(&counts.DeliveryUnconfirmed)
	case "successful_period":
		periods := store.Periods[event.ScheduleKey]
		if periods == nil {
			periods = map[string]string{}
			store.Periods[event.ScheduleKey] = periods
		}
		if _, seen := periods[event.PeriodKey]; !seen {
			if len(periods) > 0 {
				reportingCountIncrement(&counts.Repeated[index])
			}
			reportingCountIncrement(&counts.Successful[index])
			periods[event.PeriodKey] = day
		}
	}
	store.Daily[day] = counts
	return c.saveReportingActivityLocked(store)
}

func reportingCountAdd(dst *int, value int) {
	if value <= 0 {
		return
	}
	if value > reportingActivityLimit {
		value = reportingActivityLimit
	}
	if *dst > reportingActivityLimit-value {
		*dst = reportingActivityLimit
	} else {
		*dst += value
	}
}

// ReportingActivitySnapshot returns aggregate counts for the current UTC day
// and previous 29 UTC days. An unreadable tally is unavailable, never evidence
// of zero usage. The first observation day is persisted even before first use.
func (c *ConfigPersistence) ReportingActivitySnapshot(now time.Time) (ReportingActivitySummary, error) {
	var result ReportingActivitySummary
	if c == nil || c.configDir == "" {
		return result, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	store, err := c.loadReportingActivityLocked(now)
	if err != nil {
		return result, err
	}
	pruneReportingActivity(store, now)
	if err := c.saveReportingActivityLocked(store); err != nil {
		return result, err
	}
	start, _ := time.Parse(time.DateOnly, store.ObservedSince)
	end, _ := time.Parse(time.DateOnly, now.UTC().Format(time.DateOnly))
	result.Available = true
	result.ObservationDays = int(end.Sub(start)/(24*time.Hour)) + 1
	if result.ObservationDays > 30 {
		result.ObservationDays = 30
	}
	for _, daily := range store.Daily {
		for i := range daily.Generated {
			if daily.Generated[i] > 0 {
				result.ActiveDays[i]++
			}
			reportingCountAdd(&result.Counts.Generated[i], daily.Generated[i])
			reportingCountAdd(&result.Counts.Successful[i], daily.Successful[i])
			reportingCountAdd(&result.Counts.Repeated[i], daily.Repeated[i])
		}
		reportingCountAdd(&result.Counts.Saved, daily.Saved)
		reportingCountAdd(&result.Counts.EmailAccepted, daily.EmailAccepted)
		reportingCountAdd(&result.Counts.DeliveryFailed, daily.DeliveryFailed)
		reportingCountAdd(&result.Counts.DeliveryUnconfirmed, daily.DeliveryUnconfirmed)
		reportingCountAdd(&result.Counts.DataComplete, daily.DataComplete)
		reportingCountAdd(&result.Counts.DataIncomplete, daily.DataIncomplete)
		reportingCountAdd(&result.Counts.DataUnavailable, daily.DataUnavailable)
	}
	return result, nil
}
