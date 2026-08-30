package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	WorkloadHistoryActivityPreview         = "preview"
	WorkloadHistoryActivityScrub           = "scrub"
	WorkloadHistoryActivityRangeChange     = "range_change"
	WorkloadHistoryActivityDetailsSelected = "details_selected"

	workloadHistoryActivityTallyFileName = "workload_history_activity_tally.json"
	workloadHistoryActivityTallyVersion  = 1
	workloadHistoryActivityRetentionDays = 31
)

// WorkloadHistoryActivityTallyData stores only daily counts for four closed,
// session-deduplicated UI milestones. It never receives a user, guest, route,
// browser, coordinate, duration, or other event-level identifier.
type WorkloadHistoryActivityTallyData struct {
	Version       int            `json:"version"`
	LastSaved     time.Time      `json:"last_saved"`
	DailyPreviews map[string]int `json:"daily_previews,omitempty"`
	DailyScrubs   map[string]int `json:"daily_scrubs,omitempty"`
	DailyRanges   map[string]int `json:"daily_range_changes,omitempty"`
	DailyDetails  map[string]int `json:"daily_details_selected,omitempty"`
}

func workloadHistoryActivityDayKey(at time.Time) string {
	return at.UTC().Format("2006-01-02")
}

func workloadHistoryActivityCountSince(daily map[string]int, since time.Time) int {
	sinceDay := workloadHistoryActivityDayKey(since)
	total := 0
	for day, count := range daily {
		if day >= sinceDay {
			total += count
		}
	}
	return total
}

func (data *WorkloadHistoryActivityTallyData) PreviewsSince(since time.Time) int {
	if data == nil {
		return 0
	}
	return workloadHistoryActivityCountSince(data.DailyPreviews, since)
}

func (data *WorkloadHistoryActivityTallyData) ScrubsSince(since time.Time) int {
	if data == nil {
		return 0
	}
	return workloadHistoryActivityCountSince(data.DailyScrubs, since)
}

func (data *WorkloadHistoryActivityTallyData) RangeChangesSince(since time.Time) int {
	if data == nil {
		return 0
	}
	return workloadHistoryActivityCountSince(data.DailyRanges, since)
}

func (data *WorkloadHistoryActivityTallyData) DetailsSelectedSince(since time.Time) int {
	if data == nil {
		return 0
	}
	return workloadHistoryActivityCountSince(data.DailyDetails, since)
}

func pruneWorkloadHistoryActivityTally(data *WorkloadHistoryActivityTallyData, now time.Time) {
	if data == nil {
		return
	}
	cutoff := workloadHistoryActivityDayKey(now.UTC().AddDate(0, 0, -workloadHistoryActivityRetentionDays))
	for _, daily := range []map[string]int{
		data.DailyPreviews,
		data.DailyScrubs,
		data.DailyRanges,
		data.DailyDetails,
	} {
		for day := range daily {
			if day < cutoff {
				delete(daily, day)
			}
		}
	}
}

func (c *ConfigPersistence) workloadHistoryActivityTallyPath() string {
	return filepath.Join(c.configDir, workloadHistoryActivityTallyFileName)
}

func (c *ConfigPersistence) loadWorkloadHistoryActivityTallyLocked() *WorkloadHistoryActivityTallyData {
	data := &WorkloadHistoryActivityTallyData{Version: workloadHistoryActivityTallyVersion}
	if raw, err := c.fs.ReadFile(c.workloadHistoryActivityTallyPath()); err == nil && len(raw) > 0 {
		decoded := &WorkloadHistoryActivityTallyData{}
		if err := json.Unmarshal(raw, decoded); err == nil {
			data = decoded
		} else {
			log.Warn().Err(err).Msg("Discarding unreadable workload history activity tally")
		}
	}
	if data.DailyPreviews == nil {
		data.DailyPreviews = make(map[string]int, workloadHistoryActivityRetentionDays)
	}
	if data.DailyScrubs == nil {
		data.DailyScrubs = make(map[string]int, workloadHistoryActivityRetentionDays)
	}
	if data.DailyRanges == nil {
		data.DailyRanges = make(map[string]int, workloadHistoryActivityRetentionDays)
	}
	if data.DailyDetails == nil {
		data.DailyDetails = make(map[string]int, workloadHistoryActivityRetentionDays)
	}
	return data
}

func (c *ConfigPersistence) LoadWorkloadHistoryActivityTally() (*WorkloadHistoryActivityTallyData, error) {
	if c == nil || c.configDir == "" {
		return &WorkloadHistoryActivityTallyData{}, nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loadWorkloadHistoryActivityTallyLocked(), nil
}

// RecordWorkloadHistoryActivity folds one allowlisted, browser-session-level
// milestone into a bounded daily counter. Unknown values are rejected before
// any write so this file can never become an event or content sink.
func (c *ConfigPersistence) RecordWorkloadHistoryActivity(activity string, now time.Time) error {
	if c == nil || c.configDir == "" {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()

	c.mu.Lock()
	defer c.mu.Unlock()

	data := c.loadWorkloadHistoryActivityTallyLocked()
	day := workloadHistoryActivityDayKey(now)
	switch activity {
	case WorkloadHistoryActivityPreview:
		data.DailyPreviews[day]++
	case WorkloadHistoryActivityScrub:
		data.DailyScrubs[day]++
	case WorkloadHistoryActivityRangeChange:
		data.DailyRanges[day]++
	case WorkloadHistoryActivityDetailsSelected:
		data.DailyDetails[day]++
	default:
		return fmt.Errorf("unknown workload history activity")
	}

	pruneWorkloadHistoryActivityTally(data, now)
	data.Version = workloadHistoryActivityTallyVersion
	data.LastSaved = now
	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode workload history activity tally: %w", err)
	}

	path := c.workloadHistoryActivityTallyPath()
	if err := c.fs.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create workload history activity tally directory: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := c.fs.WriteFile(tmpPath, encoded, 0o600); err != nil {
		return fmt.Errorf("write temp workload history activity tally: %w", err)
	}
	if err := c.fs.Rename(tmpPath, path); err != nil {
		if removeErr := c.fs.Remove(tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			log.Warn().Err(removeErr).Str("tmp_path", tmpPath).Msg("Failed to remove temporary workload history activity tally after failed rename")
		}
		return fmt.Errorf("commit workload history activity tally: %w", err)
	}
	return nil
}
