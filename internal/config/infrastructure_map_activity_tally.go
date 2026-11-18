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
	InfrastructureMapActivityOpen              = "open"
	InfrastructureMapActivityExport            = "export"
	InfrastructureMapActivityExportNamesHidden = "export_names_hidden"

	infrastructureMapActivityTallyFileName = "infrastructure_map_activity_tally.json"
	infrastructureMapActivityTallyVersion  = 1
	infrastructureMapActivityRetentionDays = 31
)

// infrastructureMapActivities is the closed set this tally will hold. Anything
// else is refused on write and dropped on read.
var infrastructureMapActivities = map[string]struct{}{
	InfrastructureMapActivityOpen:              {},
	InfrastructureMapActivityExport:            {},
	InfrastructureMapActivityExportNamesHidden: {},
}

// IsInfrastructureMapActivity reports whether activity is one of the closed
// map milestones.
func IsInfrastructureMapActivity(activity string) bool {
	_, ok := infrastructureMapActivities[activity]
	return ok
}

// InfrastructureMapActivityTallyData stores only daily counts for the closed,
// session-deduplicated map milestones. It never receives a resource, name,
// address, network, user, browser or other event-level identifier.
type InfrastructureMapActivityTallyData struct {
	Version   int       `json:"version"`
	LastSaved time.Time `json:"last_saved"`
	// Daily maps a milestone to its count per UTC day (YYYY-MM-DD).
	Daily map[string]map[string]int `json:"daily,omitempty"`
}

func infrastructureMapActivityDayKey(at time.Time) string {
	return at.UTC().Format("2006-01-02")
}

// CountSince totals one milestone from the UTC day of since onwards.
func (data *InfrastructureMapActivityTallyData) CountSince(activity string, since time.Time) int {
	if data == nil {
		return 0
	}
	sinceDay := infrastructureMapActivityDayKey(since)
	total := 0
	for day, count := range data.Daily[activity] {
		if day >= sinceDay && count > 0 {
			total += count
		}
	}
	return total
}

func pruneInfrastructureMapActivityTally(data *InfrastructureMapActivityTallyData, now time.Time) {
	cutoff := infrastructureMapActivityDayKey(now.UTC().AddDate(0, 0, -infrastructureMapActivityRetentionDays))
	for activity, daily := range data.Daily {
		if !IsInfrastructureMapActivity(activity) {
			delete(data.Daily, activity)
			continue
		}
		for day := range daily {
			if day < cutoff {
				delete(daily, day)
			}
		}
	}
}

func (c *ConfigPersistence) infrastructureMapActivityTallyPath() string {
	return filepath.Join(c.configDir, infrastructureMapActivityTallyFileName)
}

func (c *ConfigPersistence) loadInfrastructureMapActivityTallyLocked() *InfrastructureMapActivityTallyData {
	data := &InfrastructureMapActivityTallyData{Version: infrastructureMapActivityTallyVersion}
	if raw, err := c.fs.ReadFile(c.infrastructureMapActivityTallyPath()); err == nil && len(raw) > 0 {
		decoded := &InfrastructureMapActivityTallyData{}
		if err := json.Unmarshal(raw, decoded); err == nil {
			data = decoded
		} else {
			log.Warn().Err(err).Msg("Discarding unreadable infrastructure map activity tally")
		}
	}
	if data.Daily == nil {
		data.Daily = make(map[string]map[string]int, len(infrastructureMapActivities))
	}
	for activity := range data.Daily {
		if !IsInfrastructureMapActivity(activity) {
			delete(data.Daily, activity)
		}
	}
	return data
}

func (c *ConfigPersistence) LoadInfrastructureMapActivityTally() (*InfrastructureMapActivityTallyData, error) {
	if c == nil || c.configDir == "" {
		return &InfrastructureMapActivityTallyData{}, nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loadInfrastructureMapActivityTallyLocked(), nil
}

// RecordInfrastructureMapActivity folds one allowlisted, browser-session-level
// milestone into a bounded daily counter. Unknown values are rejected before
// any write so this file can never become an event or content sink.
func (c *ConfigPersistence) RecordInfrastructureMapActivity(activity string, now time.Time) error {
	if !IsInfrastructureMapActivity(activity) {
		return fmt.Errorf("unknown infrastructure map activity")
	}
	if c == nil || c.configDir == "" {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()

	c.mu.Lock()
	defer c.mu.Unlock()

	data := c.loadInfrastructureMapActivityTallyLocked()
	if data.Daily[activity] == nil {
		data.Daily[activity] = make(map[string]int, infrastructureMapActivityRetentionDays)
	}
	data.Daily[activity][infrastructureMapActivityDayKey(now)]++

	pruneInfrastructureMapActivityTally(data, now)
	data.Version = infrastructureMapActivityTallyVersion
	data.LastSaved = now
	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode infrastructure map activity tally: %w", err)
	}

	path := c.infrastructureMapActivityTallyPath()
	if err := c.fs.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create infrastructure map activity tally directory: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := c.fs.WriteFile(tmpPath, encoded, 0o600); err != nil {
		return fmt.Errorf("write temp infrastructure map activity tally: %w", err)
	}
	if err := c.fs.Rename(tmpPath, path); err != nil {
		if removeErr := c.fs.Remove(tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			log.Warn().Err(removeErr).Str("tmp_path", tmpPath).Msg("Failed to remove temporary infrastructure map activity tally after failed rename")
		}
		return fmt.Errorf("commit infrastructure map activity tally: %w", err)
	}
	return nil
}
