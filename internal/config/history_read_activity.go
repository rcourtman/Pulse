package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const historyReadActivityFile = "history_read_activity.json"

// HistoryReadActivity retains daily counts only. Requests, ranges, metric
// values, resources, actors and individual read timestamps are never stored.
type HistoryReadActivity struct {
	DailySuccessfulReads map[string]int `json:"daily_successful_reads_over_7d"`
}

func (c *ConfigPersistence) loadHistoryReadActivityLocked() (*HistoryReadActivity, error) {
	activity := &HistoryReadActivity{DailySuccessfulReads: map[string]int{}}
	raw, err := c.fs.ReadFile(filepath.Join(c.configDir, historyReadActivityFile))
	if os.IsNotExist(err) {
		return activity, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, activity); err != nil {
		return nil, err
	}
	if activity.DailySuccessfulReads == nil {
		activity.DailySuccessfulReads = map[string]int{}
	}
	return activity, nil
}

// RecordSuccessfulExtendedHistoryRead folds one successfully written response
// containing a genuine metric older than seven days into its UTC-day count.
func (c *ConfigPersistence) RecordSuccessfulExtendedHistoryRead(now time.Time) error {
	if c == nil || c.configDir == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	activity, err := c.loadHistoryReadActivityLocked()
	if err != nil {
		return fmt.Errorf("load history read activity: %w", err)
	}
	day := now.UTC().Format("2006-01-02")
	cutoff := now.UTC().AddDate(0, 0, -30).Format("2006-01-02")
	for retainedDay := range activity.DailySuccessfulReads {
		if retainedDay < cutoff || retainedDay > day {
			delete(activity.DailySuccessfulReads, retainedDay)
		}
	}
	activity.DailySuccessfulReads[day]++
	raw, err := json.Marshal(activity)
	if err != nil {
		return err
	}
	path := filepath.Join(c.configDir, historyReadActivityFile)
	if err := c.fs.MkdirAll(c.configDir, 0o700); err != nil {
		return err
	}
	if err := c.fs.WriteFile(path+".tmp", raw, 0o600); err != nil {
		return err
	}
	if err := c.fs.Rename(path+".tmp", path); err != nil {
		_ = c.fs.Remove(path + ".tmp")
		return err
	}
	return nil
}

func (c *ConfigPersistence) CountSuccessfulExtendedHistoryReadsSince(since, now time.Time) int {
	if c == nil || c.configDir == "" {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	activity, err := c.loadHistoryReadActivityLocked()
	if err != nil {
		return 0
	}
	first, last := since.UTC().Format("2006-01-02"), now.UTC().Format("2006-01-02")
	count := 0
	for day, reads := range activity.DailySuccessfulReads {
		if day >= first && day <= last && reads > 0 {
			count += reads
		}
	}
	return count
}
