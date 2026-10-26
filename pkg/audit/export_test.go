package audit

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestExporterOverflowErrorIdentity(t *testing.T) {
	if MaxExportEvents != exportTestMaxEvents {
		t.Fatalf("event ceiling = %d, want %d", MaxExportEvents, exportTestMaxEvents)
	}
	for _, mode := range []string{"json", "csv", "summary"} {
		t.Run(mode, func(t *testing.T) {
			exporter := NewExporter(&exportBoundsLogger{events: exportBoundsEvents(MaxExportEvents + 1)})
			var err error
			if mode == "summary" {
				_, err = exporter.GenerateSummary(QueryFilter{}, true)
			} else {
				_, err = exporter.Export(QueryFilter{}, ExportFormat(mode), true)
			}
			if !errors.Is(err, ErrExportTooLarge) {
				t.Fatalf("overflow error = %v, want wrapped ErrExportTooLarge", err)
			}
		})
	}
}

func TestExporterExportAndSummary(t *testing.T) {
	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	events := []Event{
		{
			ID:        "e1",
			Timestamp: time.Now().Add(-time.Minute),
			EventType: "login",
			User:      "alice",
			IP:        "127.0.0.1",
			Success:   true,
			Details:   "ok",
		},
		{
			ID:        "e2",
			Timestamp: time.Now(),
			EventType: "config_change",
			User:      "",
			IP:        "127.0.0.2",
			Success:   false,
			Details:   "failed",
		},
	}

	for _, event := range events {
		if err := logger.Record(event); err != nil {
			t.Fatalf("log event: %v", err)
		}
	}

	exporter := NewExporter(logger)
	result, err := exporter.Export(QueryFilter{}, ExportFormatCSV, true)
	if err != nil {
		t.Fatalf("export csv: %v", err)
	}
	if !strings.HasPrefix(result.Filename, "audit-log-") || !strings.HasSuffix(result.Filename, ".csv") {
		t.Fatalf("unexpected filename: %s", result.Filename)
	}

	reader := csv.NewReader(strings.NewReader(string(result.Data)))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(records) < 3 || records[0][0] != "ID" {
		t.Fatalf("unexpected csv records: %+v", records)
	}

	jsonResult, err := exporter.Export(QueryFilter{}, ExportFormatJSON, false)
	if err != nil {
		t.Fatalf("export json: %v", err)
	}
	var parsed struct {
		EventCount int           `json:"event_count"`
		Events     []ExportEvent `json:"events"`
	}
	if err := json.Unmarshal(jsonResult.Data, &parsed); err != nil {
		t.Fatalf("decode json export: %v", err)
	}
	if parsed.EventCount != 2 || len(parsed.Events) != 2 {
		t.Fatalf("unexpected json export: %+v", parsed)
	}

	if _, err := exporter.Export(QueryFilter{}, "xml", false); err == nil {
		t.Fatal("expected error for unsupported format")
	}

	// Tamper with signature to test verification in summary/export
	if _, err := logger.db.Exec(`UPDATE audit_events SET signature = 'bad' WHERE id = ?`, "e2"); err != nil {
		t.Fatalf("tamper signature: %v", err)
	}

	summary, err := exporter.GenerateSummary(QueryFilter{}, true)
	if err != nil {
		t.Fatalf("summary error: %v", err)
	}
	if summary.TotalEvents != 2 || summary.InvalidSigCount == 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}
