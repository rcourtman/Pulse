package audit

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"
)

const exportTestMaxEvents = 10000

type exportBoundsLogger struct {
	events        []Event
	ignoreLimit   bool
	queryErr      error
	lastFilter    QueryFilter
	queries       int
	verifications int
}

func (l *exportBoundsLogger) Query(filter QueryFilter) ([]Event, error) {
	l.lastFilter = filter
	l.queries++
	events := l.events
	if !l.ignoreLimit && filter.Limit > 0 && len(events) > filter.Limit {
		events = events[:filter.Limit]
	}
	return events, l.queryErr
}

func (l *exportBoundsLogger) VerifySignature(event Event) bool {
	l.verifications++
	return event.Signature == "valid-fixture-signature"
}

func exportBoundsEvents(count int) []Event {
	events := make([]Event, count)
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for i := range events {
		events[i] = Event{
			ID: fmt.Sprintf("event-%d", i), Timestamp: at.Add(time.Duration(i) * time.Second),
			EventType: "login", User: "synthetic-operator", IP: "192.0.2.1", Path: "/synthetic",
			Success: i%2 == 0, Details: "synthetic, complete\nquoted \"details\"",
			Signature: "valid-fixture-signature",
		}
	}
	if count > 1 {
		events[count-1].Signature = "invalid-fixture-signature"
	}
	return events
}

func exportBoundsFilter() QueryFilter {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	success := true
	return QueryFilter{ID: "selected-id", StartTime: &start, EndTime: &end, EventType: "login", User: "synthetic-operator", Success: &success, Limit: 1, Offset: 99}
}

func assertExportBoundsQuery(t *testing.T, logger *exportBoundsLogger, filter QueryFilter) {
	t.Helper()
	filter.Limit = exportTestMaxEvents + 1
	filter.Offset = 0
	if logger.queries != 1 || !reflect.DeepEqual(logger.lastFilter, filter) {
		t.Errorf("query count/filter = %d/%+v, want one bounded read with %+v", logger.queries, logger.lastFilter, filter)
	}
}

func TestExporterRejectsOversizedSelections(t *testing.T) {
	for _, mode := range []string{"json", "csv", "summary"} {
		for _, ignoreLimit := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/ignore-limit-%t", mode, ignoreLimit), func(t *testing.T) {
				logger := &exportBoundsLogger{events: exportBoundsEvents(exportTestMaxEvents + 2), ignoreLimit: ignoreLimit}
				exporter := NewExporter(logger)
				filter := exportBoundsFilter()
				if mode == "summary" {
					result, err := exporter.GenerateSummary(filter, true)
					if err == nil || result != nil {
						t.Error("oversized summary must return an error and no partial result")
					}
				} else {
					result, err := exporter.Export(filter, ExportFormat(mode), true)
					if err == nil || result != nil {
						t.Error("oversized export must return an error and no partial result")
					}
				}
				assertExportBoundsQuery(t, logger, filter)
				if logger.verifications != 0 {
					t.Errorf("verified %d signatures before refusing overflow", logger.verifications)
				}
			})
		}
	}
}

func TestExporterBoundedCompleteSelections(t *testing.T) {
	for _, mode := range []string{"json", "csv", "summary"} {
		for _, count := range []int{0, 2, exportTestMaxEvents} {
			for _, verify := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/verify-%t", mode, count, verify), func(t *testing.T) {
					events := exportBoundsEvents(count)
					logger := &exportBoundsLogger{events: events}
					exporter := NewExporter(logger)
					filter := exportBoundsFilter()
					if mode == "summary" {
						result, err := exporter.GenerateSummary(filter, verify)
						if err != nil {
							t.Fatal(err)
						}
						invalid := 0
						if count > 1 && verify {
							invalid = 1
						}
						if result.TotalEvents != count || result.SuccessCount != (count+1)/2 || result.FailureCount != count/2 || result.EventsByType["login"] != count || result.EventsByUser["synthetic-operator"] != count || result.InvalidSigCount != invalid {
							t.Fatalf("incomplete summary: %+v", result)
						}
						if count > 0 && (result.StartTime == nil || result.EndTime == nil || !result.StartTime.Equal(events[0].Timestamp) || !result.EndTime.Equal(events[count-1].Timestamp)) {
							t.Fatal("summary lost its time range")
						}
					} else {
						result, err := exporter.Export(filter, ExportFormat(mode), verify)
						if err != nil {
							t.Fatal(err)
						}
						if result.EventCount != count {
							t.Fatalf("event count = %d, want %d", result.EventCount, count)
						}
						if mode == "json" {
							var decoded struct {
								EventCount int           `json:"event_count"`
								Events     []ExportEvent `json:"events"`
							}
							if err := json.Unmarshal(result.Data, &decoded); err != nil {
								t.Fatal(err)
							}
							if decoded.EventCount != count || len(decoded.Events) != count || decoded.Events == nil {
								t.Fatal("JSON export is not a complete array")
							}
							for i, got := range decoded.Events {
								want := events[i]
								if got.ID != want.ID || !got.Timestamp.Equal(want.Timestamp) || got.EventType != want.EventType || got.User != want.User || got.IP != want.IP || got.Path != want.Path || got.Success != want.Success || got.Details != want.Details || got.Signature != want.Signature {
									t.Fatalf("JSON event %d lost fields", i)
								}
								if verify && (got.SignatureValid == nil || *got.SignatureValid != (want.Signature == "valid-fixture-signature")) || !verify && got.SignatureValid != nil {
									t.Fatalf("JSON event %d has incorrect verification", i)
								}
							}
						} else {
							rows, err := csv.NewReader(bytes.NewReader(result.Data)).ReadAll()
							if err != nil {
								t.Fatal(err)
							}
							columns := 9
							if verify {
								columns++
							}
							if len(rows) != count+1 || len(rows[0]) != columns {
								t.Fatal("CSV export is not complete")
							}
							for i, row := range rows[1:] {
								event := events[i]
								want := []string{event.ID, event.Timestamp.Format(time.RFC3339), event.EventType, event.User, event.IP, event.Path, strconv.FormatBool(event.Success), event.Details, event.Signature}
								if verify {
									want = append(want, strconv.FormatBool(event.Signature == "valid-fixture-signature"))
								}
								if !reflect.DeepEqual(row, want) {
									t.Fatalf("CSV event %d lost fields or verification", i)
								}
							}
						}
					}
					assertExportBoundsQuery(t, logger, filter)
					wantVerifications := 0
					if verify {
						wantVerifications = count
					}
					if logger.verifications != wantVerifications {
						t.Errorf("verifications = %d, want %d", logger.verifications, wantVerifications)
					}
				})
			}
		}
	}
}

func TestExporterBoundedReadFailureReturnsNoResult(t *testing.T) {
	readErr := errors.New("synthetic partial read failure")
	for _, mode := range []string{"json", "csv", "summary"} {
		t.Run(mode, func(t *testing.T) {
			logger := &exportBoundsLogger{events: exportBoundsEvents(2), queryErr: readErr}
			exporter := NewExporter(logger)
			filter := exportBoundsFilter()
			if mode == "summary" {
				result, err := exporter.GenerateSummary(filter, true)
				if result != nil || !errors.Is(err, readErr) {
					t.Fatal("partial summary read must fail without a result and preserve its error")
				}
			} else {
				result, err := exporter.Export(filter, ExportFormat(mode), true)
				if result != nil || !errors.Is(err, readErr) {
					t.Fatal("partial export read must fail without a result and preserve its error")
				}
			}
			assertExportBoundsQuery(t, logger, filter)
			if logger.verifications != 0 {
				t.Fatal("partial read verified signatures")
			}
		})
	}
}
