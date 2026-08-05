package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// normalizeAuditReadActivityRecord drops records that carry no recognized
// activity class. Unknown classes are rejected rather than coerced so a future
// caller cannot silently widen what this history means.
func normalizeAuditReadActivityRecord(record AuditReadActivityRecord) (AuditReadActivityRecord, bool) {
	if record.Timestamp.IsZero() {
		return AuditReadActivityRecord{}, false
	}
	record.Timestamp = record.Timestamp.UTC()
	switch strings.TrimSpace(record.Activity) {
	case AuditReadActivityList, AuditReadActivityExport, AuditReadActivityVerify, AuditReadActivitySummary:
		record.Activity = strings.TrimSpace(record.Activity)
	default:
		return AuditReadActivityRecord{}, false
	}
	return record, true
}

func normalizeAuditReadActivityRecords(records []AuditReadActivityRecord) []AuditReadActivityRecord {
	if records == nil {
		return make([]AuditReadActivityRecord, 0)
	}
	normalized := make([]AuditReadActivityRecord, 0, len(records))
	for _, record := range records {
		record, ok := normalizeAuditReadActivityRecord(record)
		if !ok {
			continue
		}
		normalized = append(normalized, record)
	}
	return normalized
}

func pruneAuditReadActivityRecords(records []AuditReadActivityRecord, cutoff time.Time, maxRecords int) []AuditReadActivityRecord {
	normalized := normalizeAuditReadActivityRecords(records)
	if !cutoff.IsZero() {
		kept := normalized[:0]
		for _, record := range normalized {
			if record.Timestamp.Before(cutoff) {
				continue
			}
			kept = append(kept, record)
		}
		normalized = kept
	}
	if maxRecords > 0 && len(normalized) > maxRecords {
		normalized = normalized[len(normalized)-maxRecords:]
	}
	return normalized
}

func newEmptyAuditReadActivityHistoryData() *AuditReadActivityHistoryData {
	return &AuditReadActivityHistoryData{
		Version: 1,
		Events:  make([]AuditReadActivityRecord, 0),
	}
}

// RecordAuditReadActivity appends a content-free marker that an entitled
// operator reached one of the license-gated audit read or export surfaces.
func (c *ConfigPersistence) RecordAuditReadActivity(record AuditReadActivityRecord) error {
	if c == nil {
		return nil
	}
	now := time.Now().UTC()
	if record.Timestamp.IsZero() {
		record.Timestamp = now
	}
	record, ok := normalizeAuditReadActivityRecord(record)
	if !ok {
		return nil
	}
	return recordActivityHistoryLocked(c, c.auditReadActivityFile,
		"audit read activity history", "Resetting unreadable audit read activity history",
		newEmptyAuditReadActivityHistoryData,
		func(history *AuditReadActivityHistoryData) {
			history.Events = append(history.Events, record)
			history.Events = pruneAuditReadActivityRecords(
				history.Events,
				record.Timestamp.Add(-auditReadActivityHistoryRetention),
				maxAuditReadActivityHistoryRecords,
			)
			history.Version = 1
			history.LastSaved = now
		})
}

// SaveAuditReadActivityHistory replaces the persisted audit-read history.
func (c *ConfigPersistence) SaveAuditReadActivityHistory(events []AuditReadActivityRecord) error {
	normalized := normalizeAuditReadActivityRecords(events)
	data := AuditReadActivityHistoryData{
		Version:   1,
		LastSaved: time.Now().UTC(),
		Events:    normalized,
	}
	return saveHistoryData(c, c.auditReadActivityFile, data, len(normalized),
		"audit read activity history", "Audit read activity history")
}

// LoadAuditReadActivityHistory reads the persisted audit-read history.
func (c *ConfigPersistence) LoadAuditReadActivityHistory() (*AuditReadActivityHistoryData, error) {
	return loadHistoryData(
		c.fs,
		&c.mu,
		c.auditReadActivityFile,
		c.crypto,
		newEmptyAuditReadActivityHistoryData,
		func(data *AuditReadActivityHistoryData) {
			data.Events = normalizeAuditReadActivityRecords(data.Events)
		},
		func(data *AuditReadActivityHistoryData) error {
			jsonData, err := json.Marshal(data)
			if err != nil {
				return fmt.Errorf("marshal audit read activity history migration rewrite: %w", err)
			}
			return rewriteEncryptedJSONLocked(c, c.auditReadActivityFile, jsonData, "audit read activity history migration rewrite")
		},
		func(data *AuditReadActivityHistoryData) int {
			return len(data.Events)
		},
		func(data *AuditReadActivityHistoryData) time.Time {
			return data.LastSaved
		},
		"Audit read activity history",
	)
}

// CountAuditReadActivitySince returns how many audit reads happened at or after
// the cutoff. Count only: the activity classes are not broken out, so the
// telemetry payload cannot describe what an operator looked at.
func (c *ConfigPersistence) CountAuditReadActivitySince(since time.Time) int {
	if c == nil {
		return 0
	}
	history, err := c.LoadAuditReadActivityHistory()
	if err != nil || history == nil {
		return 0
	}
	count := 0
	for _, record := range history.Events {
		if record.Timestamp.Before(since) {
			continue
		}
		count++
	}
	return count
}
