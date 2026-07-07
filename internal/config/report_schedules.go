package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	ReportScheduleCadenceMonthly = "monthly"
	ReportScheduleCadenceWeekly  = "weekly"

	ReportScheduleFormatPDF = "pdf"
	ReportScheduleFormatCSV = "csv"

	ReportScheduleDeliveryEmail = "email"
	ReportScheduleDeliveryDisk  = "disk"

	ReportScheduleLastRunOK     = "ok"
	ReportScheduleLastRunFailed = "failed"

	DefaultReportScheduleRetentionCount = 12
)

type ReportScheduleStore struct {
	Schedules []ReportSchedule `json:"schedules"`
}

type ReportSchedule struct {
	ID                string                 `json:"id"`
	Name              string                 `json:"name"`
	Enabled           bool                   `json:"enabled"`
	Cadence           ReportScheduleCadence  `json:"cadence"`
	Scope             ReportScheduleScope    `json:"scope"`
	Window            string                 `json:"window,omitempty"`
	Format            string                 `json:"format"`
	Delivery          ReportScheduleDelivery `json:"delivery"`
	RetentionCount    int                    `json:"retention_count,omitempty"`
	LastRunAt         *time.Time             `json:"last_run_at,omitempty"`
	LastRunStatus     string                 `json:"last_run_status,omitempty"`
	LastError         string                 `json:"last_error,omitempty"`
	NextRunAt         *time.Time             `json:"next_run_at,omitempty"`
	LastOccurrenceKey string                 `json:"last_occurrence_key,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

type ReportScheduleCadence struct {
	Type       string `json:"type"`
	DayOfMonth int    `json:"day_of_month,omitempty"`
	Weekday    string `json:"weekday,omitempty"`
	Time       string `json:"time"`
	Timezone   string `json:"timezone"`
}

type ReportScheduleScope struct {
	Resources []ReportScheduleResource `json:"resources,omitempty"`
	Tags      []string                 `json:"tags,omitempty"`
}

type ReportScheduleResource struct {
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	Name         string `json:"name,omitempty"`
}

type ReportScheduleDelivery struct {
	Method     string   `json:"method"`
	To         []string `json:"to,omitempty"`
	Attach     bool     `json:"attach"`
	SaveToDisk bool     `json:"save_to_disk"`
}

func EmptyReportScheduleStore() ReportScheduleStore {
	return ReportScheduleStore{Schedules: []ReportSchedule{}}
}

func NormalizeReportScheduleStore(store ReportScheduleStore) ReportScheduleStore {
	if store.Schedules == nil {
		store.Schedules = []ReportSchedule{}
	}
	for i := range store.Schedules {
		store.Schedules[i] = NormalizeReportSchedule(store.Schedules[i])
	}
	return store
}

func NormalizeReportSchedule(schedule ReportSchedule) ReportSchedule {
	schedule.ID = strings.TrimSpace(schedule.ID)
	schedule.Name = strings.TrimSpace(schedule.Name)
	schedule.Cadence.Type = strings.ToLower(strings.TrimSpace(schedule.Cadence.Type))
	schedule.Cadence.Weekday = strings.ToLower(strings.TrimSpace(schedule.Cadence.Weekday))
	schedule.Cadence.Time = strings.TrimSpace(schedule.Cadence.Time)
	schedule.Cadence.Timezone = strings.TrimSpace(schedule.Cadence.Timezone)
	schedule.Window = strings.TrimSpace(schedule.Window)
	schedule.Format = strings.ToLower(strings.TrimSpace(schedule.Format))
	schedule.Delivery.Method = strings.ToLower(strings.TrimSpace(schedule.Delivery.Method))
	schedule.LastRunStatus = strings.ToLower(strings.TrimSpace(schedule.LastRunStatus))
	schedule.LastError = strings.TrimSpace(schedule.LastError)
	schedule.LastOccurrenceKey = strings.TrimSpace(schedule.LastOccurrenceKey)
	if schedule.RetentionCount <= 0 {
		schedule.RetentionCount = DefaultReportScheduleRetentionCount
	}
	if schedule.Scope.Resources == nil {
		schedule.Scope.Resources = []ReportScheduleResource{}
	}
	for i := range schedule.Scope.Resources {
		schedule.Scope.Resources[i].ResourceType = strings.ToLower(strings.TrimSpace(schedule.Scope.Resources[i].ResourceType))
		schedule.Scope.Resources[i].ResourceID = strings.TrimSpace(schedule.Scope.Resources[i].ResourceID)
		schedule.Scope.Resources[i].Name = strings.TrimSpace(schedule.Scope.Resources[i].Name)
	}
	schedule.Scope.Tags = normalizeReportScheduleStringSlice(schedule.Scope.Tags)
	schedule.Delivery.To = normalizeReportScheduleStringSlice(schedule.Delivery.To)
	return schedule
}

func normalizeReportScheduleStringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func parseReportScheduleStoreJSON(data []byte) (ReportScheduleStore, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return EmptyReportScheduleStore(), nil
	}
	var store ReportScheduleStore
	if err := json.Unmarshal(data, &store); err == nil && store.Schedules != nil {
		return NormalizeReportScheduleStore(store), nil
	}

	var schedules []ReportSchedule
	if err := json.Unmarshal(data, &schedules); err != nil {
		return ReportScheduleStore{}, err
	}
	return NormalizeReportScheduleStore(ReportScheduleStore{Schedules: schedules}), nil
}

func (c *ConfigPersistence) ReportSchedulesPath() string {
	if c == nil {
		return ""
	}
	return c.reportSchedulesFile
}

func (c *ConfigPersistence) LoadReportScheduleStore() (*ReportScheduleStore, error) {
	if c == nil {
		store := EmptyReportScheduleStore()
		return &store, nil
	}

	c.mu.RLock()
	data, err := c.fs.ReadFile(c.reportSchedulesFile)
	cryptoMgr := c.crypto
	c.mu.RUnlock()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			store := EmptyReportScheduleStore()
			return &store, nil
		}
		return nil, fmt.Errorf("read report schedules: %w", err)
	}

	store, parseErr := parseReportScheduleStoreJSON(data)
	if parseErr == nil {
		return &store, nil
	}
	if cryptoMgr == nil {
		return nil, fmt.Errorf("parse report schedules: %w", parseErr)
	}

	decrypted, decryptErr := cryptoMgr.Decrypt(data)
	if decryptErr != nil {
		return nil, fmt.Errorf("decrypt report schedules: %w", decryptErr)
	}
	store, parseErr = parseReportScheduleStoreJSON(decrypted)
	if parseErr != nil {
		return nil, fmt.Errorf("parse decrypted report schedules: %w", parseErr)
	}
	return &store, nil
}

func (c *ConfigPersistence) SaveReportScheduleStore(store ReportScheduleStore) error {
	if c == nil {
		return fmt.Errorf("config persistence is not configured")
	}

	store = NormalizeReportScheduleStore(store)
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize report schedules: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.crypto != nil {
		encrypted, err := c.crypto.Encrypt(data)
		if err != nil {
			return fmt.Errorf("encrypt report schedules: %w", err)
		}
		data = encrypted
	}
	if err := c.writeConfigFileLocked(c.reportSchedulesFile, data, 0600); err != nil {
		return fmt.Errorf("write report schedules: %w", err)
	}
	return nil
}
