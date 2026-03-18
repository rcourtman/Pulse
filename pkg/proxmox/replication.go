package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// ReplicationJob represents the parsed status of a Proxmox storage replication job.
type ReplicationJob struct {
	ID                      string     `json:"id"`
	Guest                   string     `json:"guest,omitempty"`
	GuestID                 int        `json:"guestId,omitempty"`
	JobNumber               int        `json:"jobNumber,omitempty"`
	Source                  string     `json:"source,omitempty"`
	SourceStorage           string     `json:"sourceStorage,omitempty"`
	Target                  string     `json:"target,omitempty"`
	TargetStorage           string     `json:"targetStorage,omitempty"`
	Schedule                string     `json:"schedule,omitempty"`
	Type                    string     `json:"type,omitempty"`
	Enabled                 bool       `json:"enabled"`
	State                   string     `json:"state,omitempty"`
	Status                  string     `json:"status,omitempty"`
	LastSyncStatus          string     `json:"lastSyncStatus,omitempty"`
	LastSyncTime            *time.Time `json:"lastSyncTime,omitempty"`
	LastSyncUnix            int64      `json:"lastSyncUnix,omitempty"`
	LastSyncDurationSeconds int        `json:"lastSyncDurationSeconds,omitempty"`
	LastSyncDurationHuman   string     `json:"lastSyncDurationHuman,omitempty"`
	NextSyncTime            *time.Time `json:"nextSyncTime,omitempty"`
	NextSyncUnix            int64      `json:"nextSyncUnix,omitempty"`
	DurationSeconds         int        `json:"durationSeconds,omitempty"`
	DurationHuman           string     `json:"durationHuman,omitempty"`
	FailCount               int        `json:"failCount,omitempty"`
	Error                   string     `json:"error,omitempty"`
	Comment                 string     `json:"comment,omitempty"`
	RemoveJob               string     `json:"removeJob,omitempty"`
	RateLimitMbps           *float64   `json:"rateLimitMbps,omitempty"`
}

// GetReplicationStatus returns the replication jobs configured on a PVE instance.
// It fetches job configuration from /cluster/replication and then enriches each
// job with status data (last_sync, next_sync, duration, fail_count, state) from
// /nodes/{node}/replication/{id}/status.
func (c *Client) GetReplicationStatus(ctx context.Context) ([]ReplicationJob, error) {
	resp, err := c.get(ctx, "/cluster/replication")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw struct {
		Data []map[string]json.RawMessage `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	jobs := make([]ReplicationJob, 0, len(raw.Data))
	for _, entry := range raw.Data {
		jobs = append(jobs, parseReplicationJob(entry))
	}

	// Enrich jobs with status data from the per-node status endpoint
	// The /cluster/replication endpoint only returns config, not status
	for i := range jobs {
		c.enrichReplicationJobStatus(ctx, &jobs[i])
	}

	return jobs, nil
}

// enrichReplicationJobStatus fetches status data for a replication job from
// /nodes/{node}/replication/{id}/status and merges it into the job struct.
func (c *Client) enrichReplicationJobStatus(ctx context.Context, job *ReplicationJob) {
	// Status is stored on the source node
	sourceNode := job.Source
	if sourceNode == "" {
		log.Debug().Str("jobID", job.ID).Msg("Skipping replication status fetch - no source node")
		return
	}

	jobID := job.ID
	if jobID == "" {
		log.Debug().Str("source", sourceNode).Msg("Skipping replication status fetch - no job ID")
		return
	}

	endpoint := fmt.Sprintf("/nodes/%s/replication/%s/status", sourceNode, jobID)
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		log.Debug().
			Str("jobID", jobID).
			Str("source", sourceNode).
			Str("endpoint", endpoint).
			Err(err).
			Msg("Failed to fetch replication job status")
		return
	}
	defer resp.Body.Close()

	// Read body for debugging/parsing
	bodyBytes, err := readResponseBodyLimited(resp.Body)
	if err != nil {
		log.Debug().
			Str("jobID", jobID).
			Str("endpoint", endpoint).
			Err(err).
			Msg("Failed to read replication status response body")
		return
	}

	log.Debug().
		Str("jobID", jobID).
		Str("endpoint", endpoint).
		Str("responseBody", string(bodyBytes)).
		Msg("Received replication status response")

	// Try to parse as array first (common case)
	var statusResp struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	var status map[string]json.RawMessage

	if err := json.Unmarshal(bodyBytes, &statusResp); err != nil {
		log.Debug().
			Str("jobID", jobID).
			Str("endpoint", endpoint).
			Err(err).
			Msg("Failed to decode replication status response as array, trying single object")

		// Try parsing as single object { "data": { ... } }
		var singleResp struct {
			Data map[string]json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(bodyBytes, &singleResp); err != nil {
			log.Debug().
				Str("jobID", jobID).
				Str("endpoint", endpoint).
				Err(err).
				Msg("Failed to decode replication status response as single object")
			return
		}
		if len(singleResp.Data) == 0 {
			log.Debug().
				Str("jobID", jobID).
				Str("endpoint", endpoint).
				Msg("Replication status response has empty data object")
			return
		}
		status = singleResp.Data
	} else {
		// Successfully parsed as array
		if len(statusResp.Data) == 0 {
			log.Debug().
				Str("jobID", jobID).
				Str("endpoint", endpoint).
				Msg("Replication status response has empty data array")
			return
		}
		status = statusResp.Data[0]
	}

	// Parse and merge status fields
	if t, unix := parseReplicationTime(decodeRaw(status["last_sync"])); t != nil {
		job.LastSyncTime = t
		job.LastSyncUnix = unix
	}

	if t, unix := parseReplicationTime(decodeRaw(status["next_sync"])); t != nil {
		job.NextSyncTime = t
		job.NextSyncUnix = unix
	}

	if failCount, ok := intFromAny(decodeRaw(status["fail_count"])); ok {
		job.FailCount = failCount
	}

	if duration, _ := parseDurationSeconds(decodeRaw(status["duration"])); duration > 0 {
		job.DurationSeconds = duration
		job.LastSyncDurationSeconds = duration
	}

	if state := stringFromAny(decodeRaw(status["state"])); state != "" {
		job.State = state
		if job.Status == "" {
			job.Status = state
		}
	}

	if errMsg := stringFromAny(decodeRaw(status["error"])); errMsg != "" {
		job.Error = errMsg
	}

	log.Debug().
		Str("jobID", jobID).
		Str("source", sourceNode).
		Interface("lastSync", job.LastSyncTime).
		Interface("nextSync", job.NextSyncTime).
		Str("state", job.State).
		Msg("Successfully enriched replication job with status")
}

func parseReplicationJob(entry map[string]json.RawMessage) ReplicationJob {
	job := ReplicationJob{
		Enabled: true,
	}

	job.ID = stringFromAny(decodeRaw(entry["id"]))
	if job.ID == "" {
		job.ID = stringFromAny(decodeRaw(entry["jobid"]))
	}

	job.Guest = stringFromAny(decodeRaw(entry["guest"]))
	if guestID, ok := intFromAny(decodeRaw(entry["guest"])); ok {
		job.GuestID = guestID
	}

	if jobNum, ok := intFromAny(decodeRaw(entry["jobnum"])); ok {
		job.JobNumber = jobNum
	} else if parts := strings.Split(job.ID, "-"); len(parts) == 2 {
		if num, err := strconv.Atoi(parts[1]); err == nil {
			job.JobNumber = num
		}
	}

	job.Source = stringFromAny(decodeRaw(entry["source"]))
	job.SourceStorage = stringFromAny(firstNonNilRaw(entry, "source-storage", "source_storage"))
	job.Target = stringFromAny(decodeRaw(entry["target"]))
	job.TargetStorage = stringFromAny(firstNonNilRaw(entry, "target-storage", "target_storage"))
	job.Schedule = stringFromAny(decodeRaw(entry["schedule"]))
	job.Type = stringFromAny(decodeRaw(entry["type"]))
	job.Comment = stringFromAny(decodeRaw(entry["comment"]))
	job.RemoveJob = stringFromAny(firstNonNilRaw(entry, "remove_job", "remove-job"))

	if enabled, ok := boolFromAny(decodeRaw(entry["enabled"])); ok {
		job.Enabled = enabled
	}
	if disabled, ok := boolFromAny(decodeRaw(entry["disable"])); ok && disabled {
		job.Enabled = false
	}
	if active, ok := boolFromAny(decodeRaw(entry["active"])); ok && !active {
		job.Enabled = false
	}

	job.State = stringFromAny(decodeRaw(entry["state"]))
	job.Status = stringFromAny(decodeRaw(entry["status"]))
	if job.Status == "" && job.State != "" {
		job.Status = job.State
	}

	job.LastSyncStatus = stringFromAny(firstNonNilRaw(entry, "last_sync_status", "last-sync-status", "last_sync_state", "last-sync-state"))
	job.Error = stringFromAny(decodeRaw(entry["error"]))
	if job.Error == "" {
		job.Error = stringFromAny(decodeRaw(entry["last_sync_error"]))
	}
	job.FailCount, _ = intFromAny(firstNonNilRaw(entry, "fail_count", "fail-count"))

	if seconds, human := parseDurationSeconds(decodeRaw(entry["last_sync_duration"])); seconds > 0 {
		job.LastSyncDurationSeconds = seconds
		job.LastSyncDurationHuman = human
	} else if seconds, human := parseDurationSeconds(firstNonNilRaw(entry, "last-sync-duration", "last_sync_duration_sec")); seconds > 0 {
		job.LastSyncDurationSeconds = seconds
		job.LastSyncDurationHuman = human
	}

	if seconds, human := parseDurationSeconds(decodeRaw(entry["duration"])); seconds > 0 {
		job.DurationSeconds = seconds
		job.DurationHuman = human
	}

	if t, unix := parseReplicationTime(decodeRaw(entry["last_sync"])); t != nil {
		job.LastSyncTime = t
		job.LastSyncUnix = unix
	} else if t, unix := parseReplicationTime(firstNonNilRaw(entry, "last-sync", "last_sync_time")); t != nil {
		job.LastSyncTime = t
		job.LastSyncUnix = unix
	}

	if t, unix := parseReplicationTime(decodeRaw(entry["next_sync"])); t != nil {
		job.NextSyncTime = t
		job.NextSyncUnix = unix
	} else if t, unix := parseReplicationTime(firstNonNilRaw(entry, "next-sync", "next_sync_time")); t != nil {
		job.NextSyncTime = t
		job.NextSyncUnix = unix
	}

	if rate, ok := floatFromAny(decodeRaw(entry["rate"])); ok {
		job.RateLimitMbps = copyFloat(rate)
	}

	return job
}

func decodeRaw(raw json.RawMessage) interface{} {
	if raw == nil {
		return nil
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return value
}

func firstNonNilRaw(entry map[string]json.RawMessage, keys ...string) interface{} {
	for _, key := range keys {
		if value, ok := entry[key]; ok && value != nil {
			return decodeRaw(value)
		}
	}
	return nil
}

func stringFromAny(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return strings.TrimSpace(v.String())
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return ""
		}
		return strings.TrimSpace(strconv.FormatFloat(v, 'f', -1, 64))
	case float32:
		return strings.TrimSpace(strconv.FormatFloat(float64(v), 'f', -1, 32))
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func intFromAny(value interface{}) (int, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case uint:
		return int(v), true
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		return int(v), true
	case float32:
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return 0, false
		}
		return int(math.Round(float64(v))), true
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return int(math.Round(v)), true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), true
		}
		if f, err := v.Float64(); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
			return int(math.Round(f)), true
		}
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return 0, false
		}
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return int(i), true
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
			return int(math.Round(f)), true
		}
	}
	return 0, false
}

func boolFromAny(value interface{}) (bool, bool) {
	switch v := value.(type) {
	case nil:
		return false, false
	case bool:
		return v, true
	case int, int8, int16, int32, int64:
		i, ok := intFromAny(v)
		return i != 0, ok
	case uint, uint8, uint16, uint32, uint64:
		i, ok := intFromAny(v)
		return i != 0, ok
	case float32, float64:
		i, ok := intFromAny(v)
		return i != 0, ok
	case json.Number:
		i, ok := intFromAny(v)
		return i != 0, ok
	case string:
		s := strings.TrimSpace(strings.ToLower(v))
		switch s {
		case "true", "yes", "1", "on", "enabled":
			return true, true
		case "false", "no", "0", "off", "disabled":
			return false, true
		}
	}
	return false, false
}

func floatFromAny(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return v, true
	case float32:
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return 0, false
		}
		return float64(v), true
	case int, int8, int16, int32, int64:
		i, ok := intFromAny(v)
		return float64(i), ok
	case uint, uint8, uint16, uint32, uint64:
		i, ok := intFromAny(v)
		return float64(i), ok
	case json.Number:
		if f, err := v.Float64(); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
			return f, true
		}
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return 0, false
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
			return f, true
		}
	}
	return 0, false
}

func parseReplicationTime(value interface{}) (*time.Time, int64) {
	switch v := value.(type) {
	case nil:
		return nil, 0
	case time.Time:
		t := v.UTC()
		return &t, t.Unix()
	case *time.Time:
		if v == nil {
			return nil, 0
		}
		t := v.UTC()
		return &t, t.Unix()
	case int, int32, int64, uint, uint32, uint64:
		i, ok := intFromAny(v)
		if !ok || i <= 0 {
			return nil, 0
		}
		t := time.Unix(int64(i), 0).UTC()
		return &t, t.Unix()
	case float32, float64, json.Number:
		f, ok := floatFromAny(v)
		if !ok || f <= 0 {
			return nil, 0
		}
		t := time.Unix(int64(math.Round(f)), 0).UTC()
		return &t, t.Unix()
	case string:
		s := strings.TrimSpace(v)
		if s == "" || strings.EqualFold(s, "n/a") || strings.EqualFold(s, "pending") || strings.EqualFold(s, "-") {
			return nil, 0
		}

		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			if i <= 0 {
				return nil, 0
			}
			t := time.Unix(i, 0).UTC()
			return &t, t.Unix()
		}

		layouts := []string{
			time.RFC3339,
			"2006-01-02 15:04:05",
			"2006-01-02 15:04:05 -0700",
			"2006-01-02 15:04:05 MST",
			"2006-01-02T15:04:05",
		}

		for _, layout := range layouts {
			if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
				t = t.UTC()
				return &t, t.Unix()
			}
		}
	}

	return nil, 0
}

func parseDurationSeconds(value interface{}) (int, string) {
	if value == nil {
		return 0, ""
	}

	switch v := value.(type) {
	case int, int32, int64, uint, uint32, uint64:
		if secs, ok := intFromAny(v); ok && secs >= 0 {
			return secs, strconv.Itoa(secs)
		}
	case float32, float64, json.Number:
		if secs, ok := floatFromAny(v); ok && secs >= 0 {
			return int(math.Round(secs)), strconv.FormatFloat(secs, 'f', -1, 64)
		}
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return 0, ""
		}
		if strings.Contains(s, ":") {
			if secs, ok := parseHHMMSSToSeconds(s); ok {
				return secs, s
			}
		}
		if secs, err := strconv.ParseFloat(s, 64); err == nil && secs >= 0 {
			return int(math.Round(secs)), s
		}
	}

	return 0, stringFromAny(value)
}

func parseHHMMSSToSeconds(value string) (int, bool) {
	parts := strings.Split(value, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}

	total := 0
	multipliers := []int{3600, 60, 1}

	for i := 0; i < len(parts); i++ {
		idx := len(multipliers) - len(parts) + i
		part := strings.TrimSpace(parts[i])
		if part == "" {
			return 0, false
		}
		val, err := strconv.Atoi(part)
		if err != nil {
			return 0, false
		}
		total += val * multipliers[idx]
	}

	return total, true
}

func copyFloat(value float64) *float64 {
	return &value
}
