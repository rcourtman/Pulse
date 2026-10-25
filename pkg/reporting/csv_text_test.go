package reporting

import (
	"encoding/csv"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSpreadsheetText(t *testing.T) {
	for _, value := range []string{
		"=1+1", "+1+1", "-1+1", "@SUM(1,1)",
		"\tplain text", "\rplain text", "\nplain text", " =1+1",
		"\u00a0+1+1", "\uFEFF-1+1", " \t@SUM(1,1)",
		"＝1+1", "＋1+1", "－1+1", "＠SUM(1,1)",
	} {
		t.Run(value, func(t *testing.T) {
			if got := spreadsheetText(value); got != "'"+value {
				t.Errorf("spreadsheetText(%q) = %q, want text marker before original bytes", value, got)
			}
			if got := spreadsheetText(spreadsheetText(value)); got != "'"+value {
				t.Errorf("already escaped text was changed again: %q", got)
			}
		})
	}
	for _, value := range []string{
		"", "plain text", " node-1", "node-1", "name=1+1", "O'Brien", "'=-literal",
		"comma,quote\"and\nnewline", "東京", "0", "12.50", "  ",
	} {
		t.Run(value, func(t *testing.T) {
			if got := spreadsheetText(value); got != value {
				t.Errorf("ordinary text changed: %q -> %q", value, got)
			}
		})
	}
}

func readSpreadsheetCSV(t *testing.T, data []byte) [][]string {
	t.Helper()
	r := csv.NewReader(strings.NewReader(string(data)))
	r.FieldsPerRecord = -1 // Reports contain header, summary and data sections.
	records, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return records
}

func requireCSVRecord(t *testing.T, records [][]string, want []string) {
	t.Helper()
	for _, record := range records {
		if reflect.DeepEqual(record, want) {
			return
		}
	}
	t.Errorf("missing exact CSV record %q in %q", want, records)
}

func TestCSVReportsTreatExternalTextAsLiteral(t *testing.T) {
	now := time.Date(2026, 10, 25, 1, 0, 0, 0, time.UTC)
	for _, text := range []string{
		"=1+1", "+1+1", "-1+1", "@SUM(1,1)", " \t=1+1",
		"\tname", "\rname", "\nname", "\uFEFF=1+1", "＝1+1",
		"=\"quoted,comma\nand newline\"",
	} {
		t.Run(text, func(t *testing.T) {
			escaped := "'" + text
			data := &ReportData{
				Title: text, ResourceType: text, ResourceID: text,
				Resource: &ResourceInfo{Name: text},
				Start:    now, End: now.Add(time.Hour), GeneratedAt: now,
				Metrics: map[string][]MetricDataPoint{text: {{Timestamp: now, Value: -3.5}}},
				Summary: MetricSummary{ByMetric: map[string]MetricStats{
					text: {Count: 1, Min: -3.5, Max: -3.5, Avg: -3.5, Current: -3.5},
				}},
			}
			t.Run("single", func(t *testing.T) {
				out, err := NewCSVGenerator().Generate(data)
				if err != nil {
					t.Fatal(err)
				}
				records := readSpreadsheetCSV(t, out)
				requireCSVRecord(t, records, []string{"# Title:", escaped})
				requireCSVRecord(t, records, []string{"# Resource Type:", escaped})
				requireCSVRecord(t, records, []string{"# Resource ID:", escaped})
				requireCSVRecord(t, records, []string{escaped, "1", "-3.50", "-3.50", "-3.50", "-3.50", ""})
				requireCSVRecord(t, records, []string{"Timestamp", escaped})
				requireCSVRecord(t, records, []string{now.Format(time.RFC3339), "-3.50"})
			})
			t.Run("multi", func(t *testing.T) {
				out, err := NewCSVGenerator().GenerateMulti(&MultiReportData{
					Title: text, Start: now, End: now.Add(time.Hour), GeneratedAt: now,
					Resources: []*ReportData{data},
				})
				if err != nil {
					t.Fatal(err)
				}
				records := readSpreadsheetCSV(t, out)
				requireCSVRecord(t, records, []string{"# Title:", escaped})
				requireCSVRecord(t, records, []string{escaped, escaped, escaped, escaped, "1", "-3.50", "-3.50", "-3.50", "-3.50", ""})
				requireCSVRecord(t, records, []string{"Timestamp", "Resource Name", "Resource Type", "Resource ID", escaped})
				requireCSVRecord(t, records, []string{now.Format(time.RFC3339), escaped, escaped, escaped, "-3.50"})
			})
			t.Run("inventory", func(t *testing.T) {
				rows := []VMInventoryRow{{
					ResourceID: text, Instance: text, Node: text, Pool: text, Name: text,
					Status: text, DiskStatusReason: text,
					VMID: 101, CPUCores: 2, MemoryAllocatedBytes: 0, DiskAllocatedBytes: 512, DiskUsedBytes: -1,
				}}
				original := append([]VMInventoryRow(nil), rows...)
				out, err := GenerateVMInventoryCSV(VMInventoryData{Rows: rows})
				if err != nil {
					t.Fatal(err)
				}
				requireCSVRecord(t, readSpreadsheetCSV(t, out), []string{
					escaped, escaped, escaped, escaped, "101", escaped, escaped, "2", "0", "512", "-1", escaped,
				})
				if !reflect.DeepEqual(rows, original) {
					t.Fatal("CSV escaping changed the runtime inventory")
				}
			})
		})
	}
}
