package audit

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"time"
)

// ExportFormat defines the export file format.
type ExportFormat string

const (
	ExportFormatCSV  ExportFormat = "csv"
	ExportFormatJSON ExportFormat = "json"
)

// ExportResult contains export data and metadata.
type ExportResult struct {
	Data        []byte
	ContentType string
	Filename    string
	EventCount  int
}

// ExportEvent extends Event with verification status for exports.
type ExportEvent struct {
	ID             string    `json:"id"`
	Timestamp      time.Time `json:"timestamp"`
	EventType      string    `json:"event_type"`
	User           string    `json:"user,omitempty"`
	IP             string    `json:"ip,omitempty"`
	Path           string    `json:"path,omitempty"`
	Success        bool      `json:"success"`
	Details        string    `json:"details,omitempty"`
	Signature      string    `json:"signature,omitempty"`
	SignatureValid *bool     `json:"signature_valid,omitempty"`
}

// PersistentLogger defines the interface for loggers that support querying and verification.
type PersistentLogger interface {
	Query(filter QueryFilter) ([]Event, error)
	VerifySignature(event Event) bool
}

// Exporter provides export functionality for audit logs.
type Exporter struct {
	logger PersistentLogger
}

// NewExporter creates a new exporter for the given logger.
func NewExporter(logger PersistentLogger) *Exporter {
	return &Exporter{logger: logger}
}

// Export generates an export in the specified format.
func (e *Exporter) Export(filter QueryFilter, format ExportFormat, includeVerification bool) (*ExportResult, error) {
	// Remove limit for export (get all matching events)
	filter.Limit = 0
	filter.Offset = 0

	events, err := e.logger.Query(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query events for export: %w", err)
	}

	// Convert to export events
	exportEvents := make([]ExportEvent, len(events))
	for i, event := range events {
		exportEvents[i] = ExportEvent{
			ID:        event.ID,
			Timestamp: event.Timestamp,
			EventType: event.EventType,
			User:      event.User,
			IP:        event.IP,
			Path:      event.Path,
			Success:   event.Success,
			Details:   event.Details,
			Signature: event.Signature,
		}

		if includeVerification && event.Signature != "" {
			valid := e.logger.VerifySignature(event)
			exportEvents[i].SignatureValid = &valid
		}
	}

	// Generate timestamp for filename
	timestamp := time.Now().Format("20060102-150405")

	switch format {
	case ExportFormatCSV:
		return e.exportCSV(exportEvents, timestamp, includeVerification)
	case ExportFormatJSON:
		return e.exportJSON(exportEvents, timestamp)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportCSV generates a CSV export.
func (e *Exporter) exportCSV(events []ExportEvent, timestamp string, includeVerification bool) (*ExportResult, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"ID", "Timestamp", "Event Type", "User", "IP", "Path", "Success", "Details", "Signature"}
	if includeVerification {
		header = append(header, "Signature Valid")
	}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write rows
	for _, event := range events {
		success := "false"
		if event.Success {
			success = "true"
		}

		row := []string{
			event.ID,
			event.Timestamp.Format(time.RFC3339),
			event.EventType,
			event.User,
			event.IP,
			event.Path,
			success,
			event.Details,
			event.Signature,
		}

		if includeVerification {
			sigValid := ""
			if event.SignatureValid != nil {
				if *event.SignatureValid {
					sigValid = "true"
				} else {
					sigValid = "false"
				}
			}
			row = append(row, sigValid)
		}

		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("CSV writer error: %w", err)
	}

	return &ExportResult{
		Data:        buf.Bytes(),
		ContentType: "text/csv; charset=utf-8",
		Filename:    fmt.Sprintf("audit-log-%s.csv", timestamp),
		EventCount:  len(events),
	}, nil
}

// exportJSON generates a JSON export.
func (e *Exporter) exportJSON(events []ExportEvent, timestamp string) (*ExportResult, error) {
	// Wrap in an object for better structure
	export := struct {
		ExportedAt time.Time     `json:"exported_at"`
		EventCount int           `json:"event_count"`
		Events     []ExportEvent `json:"events"`
	}{
		ExportedAt: time.Now(),
		EventCount: len(events),
		Events:     events,
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON export: %w", err)
	}

	return &ExportResult{
		Data:        data,
		ContentType: "application/json; charset=utf-8",
		Filename:    fmt.Sprintf("audit-log-%s.json", timestamp),
		EventCount:  len(events),
	}, nil
}

// ExportSummary generates a summary of audit activity.
type ExportSummary struct {
	TotalEvents     int            `json:"total_events"`
	SuccessCount    int            `json:"success_count"`
	FailureCount    int            `json:"failure_count"`
	EventsByType    map[string]int `json:"events_by_type"`
	EventsByUser    map[string]int `json:"events_by_user"`
	StartTime       *time.Time     `json:"start_time,omitempty"`
	EndTime         *time.Time     `json:"end_time,omitempty"`
	InvalidSigCount int            `json:"invalid_signature_count,omitempty"`
}

// GenerateSummary creates a summary of audit events matching the filter.
func (e *Exporter) GenerateSummary(filter QueryFilter, verifySignatures bool) (*ExportSummary, error) {
	// Remove limit for summary
	filter.Limit = 0
	filter.Offset = 0

	events, err := e.logger.Query(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query events for summary: %w", err)
	}

	summary := &ExportSummary{
		TotalEvents:  len(events),
		EventsByType: make(map[string]int),
		EventsByUser: make(map[string]int),
	}

	var minTime, maxTime *time.Time

	for _, event := range events {
		if event.Success {
			summary.SuccessCount++
		} else {
			summary.FailureCount++
		}

		summary.EventsByType[event.EventType]++

		if event.User != "" {
			summary.EventsByUser[event.User]++
		}

		// Track time range
		if minTime == nil || event.Timestamp.Before(*minTime) {
			t := event.Timestamp
			minTime = &t
		}
		if maxTime == nil || event.Timestamp.After(*maxTime) {
			t := event.Timestamp
			maxTime = &t
		}

		// Verify signatures if requested
		if verifySignatures && event.Signature != "" {
			if !e.logger.VerifySignature(event) {
				summary.InvalidSigCount++
			}
		}
	}

	summary.StartTime = minTime
	summary.EndTime = maxTime

	return summary, nil
}
