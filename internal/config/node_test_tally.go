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
	nodeTestTallyFileName = "node_test_tally.json"
	nodeTestTallyVersion  = 1
	// nodeTestTallyRetentionDays bounds the tally. It exceeds the thirty-day
	// telemetry window by a day so the day the window opens on is still
	// present when the tally is read.
	nodeTestTallyRetentionDays = 31
)

// NodeTestTallyData counts node connection tests per UTC day. Telemetry can
// otherwise see only the saved-connection count, so an install that tried to
// reach a node and failed is indistinguishable from one that never opened the
// add-node dialog. Both report zero connections and stall at the "secured"
// activation stage. Two day-bucketed integers separate them and cost one small
// map entry per retained day.
//
// The tally holds counts only. Hosts, credentials, and error text never enter
// it, which is why it is stored as plain JSON alongside the encrypted
// configuration rather than through the encrypted history helpers.
type NodeTestTallyData struct {
	Version   int       `json:"version"`
	LastSaved time.Time `json:"last_saved"`
	// DailyAttempts counts tests that reached the connection stage, meaning
	// the request carried a well-formed target and credentials.
	DailyAttempts map[string]int `json:"daily_attempts,omitempty"`
	// DailyFailures counts the subset of those attempts that could not reach
	// or authenticate against the target. Successes are the remainder.
	DailyFailures map[string]int `json:"daily_failures,omitempty"`
}

// NodeTestTallyDayKey renders the UTC day key used by NodeTestTallyData.
func NodeTestTallyDayKey(at time.Time) string {
	return at.UTC().Format("2006-01-02")
}

func nodeTestTallyCountSince(daily map[string]int, since time.Time) int {
	if len(daily) == 0 {
		return 0
	}
	sinceDay := NodeTestTallyDayKey(since)
	total := 0
	for day, count := range daily {
		if day >= sinceDay {
			total += count
		}
	}
	return total
}

// AttemptsSince counts node connection tests observed at or after since. The
// tally is day-granular, so the day containing since is counted whole.
func (data *NodeTestTallyData) AttemptsSince(since time.Time) int {
	if data == nil {
		return 0
	}
	return nodeTestTallyCountSince(data.DailyAttempts, since)
}

// FailuresSince counts failed node connection tests observed at or after since.
func (data *NodeTestTallyData) FailuresSince(since time.Time) int {
	if data == nil {
		return 0
	}
	return nodeTestTallyCountSince(data.DailyFailures, since)
}

// pruneNodeTestTally drops days outside the retention window so the tally stays
// bounded no matter how long an install runs.
func pruneNodeTestTally(data *NodeTestTallyData, now time.Time) {
	if data == nil {
		return
	}
	cutoff := NodeTestTallyDayKey(now.UTC().AddDate(0, 0, -nodeTestTallyRetentionDays))
	for day := range data.DailyAttempts {
		if day < cutoff {
			delete(data.DailyAttempts, day)
		}
	}
	for day := range data.DailyFailures {
		if day < cutoff {
			delete(data.DailyFailures, day)
		}
	}
}

func (c *ConfigPersistence) nodeTestTallyPath() string {
	return filepath.Join(c.configDir, nodeTestTallyFileName)
}

// loadNodeTestTallyLocked reads the tally with c.mu already held. A missing,
// empty, or corrupt file yields an empty tally rather than an error: the
// counters are advisory telemetry and must never block a connection test.
func (c *ConfigPersistence) loadNodeTestTallyLocked() *NodeTestTallyData {
	data := &NodeTestTallyData{Version: nodeTestTallyVersion}
	if raw, err := c.fs.ReadFile(c.nodeTestTallyPath()); err == nil && len(raw) > 0 {
		decoded := &NodeTestTallyData{}
		if err := json.Unmarshal(raw, decoded); err == nil {
			data = decoded
		} else {
			log.Warn().Err(err).Msg("Discarding unreadable node test tally")
		}
	}
	if data.DailyAttempts == nil {
		data.DailyAttempts = make(map[string]int, nodeTestTallyRetentionDays)
	}
	if data.DailyFailures == nil {
		data.DailyFailures = make(map[string]int, nodeTestTallyRetentionDays)
	}
	return data
}

// LoadNodeTestTally returns the persisted node connection test tally.
func (c *ConfigPersistence) LoadNodeTestTally() (*NodeTestTallyData, error) {
	if c == nil || c.configDir == "" {
		return &NodeTestTallyData{}, nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loadNodeTestTallyLocked(), nil
}

// RecordNodeTestOutcome folds one node connection test into the daily tally.
// Callers report only tests that reached the connection stage, so a failure
// always means the target could not be reached or authenticated rather than
// that the form was incomplete.
func (c *ConfigPersistence) RecordNodeTestOutcome(failed bool, now time.Time) error {
	if c == nil || c.configDir == "" {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()

	c.mu.Lock()
	defer c.mu.Unlock()

	data := c.loadNodeTestTallyLocked()
	day := NodeTestTallyDayKey(now)
	data.DailyAttempts[day]++
	if failed {
		data.DailyFailures[day]++
	}
	pruneNodeTestTally(data, now)
	data.Version = nodeTestTallyVersion
	data.LastSaved = now

	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode node test tally: %w", err)
	}

	path := c.nodeTestTallyPath()
	if err := c.fs.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create node test tally directory: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := c.fs.WriteFile(tmpPath, encoded, 0o600); err != nil {
		return fmt.Errorf("write temp node test tally: %w", err)
	}
	if err := c.fs.Rename(tmpPath, path); err != nil {
		if removeErr := c.fs.Remove(tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			log.Warn().Err(removeErr).Str("tmp_path", tmpPath).Msg("Failed to remove temporary node test tally after failed rename")
		}
		return fmt.Errorf("commit node test tally: %w", err)
	}
	return nil
}
