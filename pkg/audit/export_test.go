package audit

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"reflect"
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

// All seven text columns are untrusted, including malformed signature text.
// The timestamp and generated booleans are not spreadsheet text projections.
func TestExporterCSVLiteralCells(t *testing.T) {
	cases := []struct {
		name, text string
		literal    bool
	}{
		{"equals", "=1+1", true}, {"plus", "+1+1", true},
		{"minus", "-12.5", true}, {"at", "@SUM(1,1)", true},
		{"tab", "\toperator", true}, {"cr", "\roperator", true}, {"lf", "\noperator", true},
		{"spaces", " \t=1+1", true}, {"nbsp", "\u00a0+1+1", true},
		{"unicode_space", "\u2003@SUM(1,1)", true}, {"bom", "\uFEFF-1", true},
		{"mixed_bom_space", " \uFEFF\t＝1+1", true},
		{"wide_equals", "＝1+1", true}, {"wide_plus", "＋1", true},
		{"wide_minus", "－1", true}, {"wide_at", "＠SUM(1,1)", true},
		{"quoted_multiline", "=\"comma,quote\"\r\nand newline", true},
		{"empty", "", false}, {"ordinary", "node-1", false},
		{"space_text", " node-1", false}, {"unicode", "東京", false},
		{"apostrophe", "O'Brien", false}, {"already_literal", "'=1+1", false},
		{"already_tab_literal", "'\toperator", false}, {"numeric_text", "12.50", false},
		{"whitespace_only", " \u00a0", false}, {"embedded", "name=1+1", false},
		{"ordinary_multiline", "comma,quote\"and\r\nnewline", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.text
			if tc.literal {
				want = "'" + want
			}
			for _, verify := range []bool{false, true} {
				event := Event{ID: tc.text, Timestamp: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC),
					EventType: tc.text, User: tc.text, IP: tc.text, Path: tc.text, Details: tc.text, Signature: tc.text, Success: true}
				logger := &exportBoundsLogger{events: []Event{event}}
				result, err := NewExporter(logger).Export(QueryFilter{}, ExportFormatCSV, verify)
				if err != nil {
					t.Fatal(err)
				}
				header := []string{"ID", "Timestamp", "Event Type", "User", "IP", "Path", "Success", "Details", "Signature"}
				row := []string{want, "2026-10-27T12:00:00Z", want, want, want, want, "true", want, want}
				count := 0
				if verify {
					header = append(header, "Signature Valid")
					verdict := ""
					if tc.text != "" {
						verdict = "false"
						count = 1
					}
					row = append(row, verdict)
				}
				// Compare encoded bytes too: csv.Reader normalises embedded CRLF to LF.
				var expected bytes.Buffer
				writer := csv.NewWriter(&expected)
				if err := writer.WriteAll([][]string{header, row}); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(result.Data, expected.Bytes()) {
					t.Fatalf("verify=%t: CSV bytes = %q, want %q", verify, result.Data, expected.Bytes())
				}
				rows, err := csv.NewReader(bytes.NewReader(result.Data)).ReadAll()
				if err != nil || len(rows) != 2 || len(rows[1]) != len(row) {
					t.Fatalf("invalid CSV shape: %q / %v", rows, err)
				}
				if result.EventCount != 1 || result.ContentType != "text/csv; charset=utf-8" || logger.verifications != count || logger.events[0] != event {
					t.Fatal("projection changed metadata, verification count or source event")
				}
			}
		})
	}
}

type csvSignatureLogger struct {
	events   []Event
	signer   *Signer
	verified []Event
}

func (l *csvSignatureLogger) Query(QueryFilter) ([]Event, error) { return l.events, nil }
func (l *csvSignatureLogger) VerifySignature(event Event) bool {
	l.verified = append(l.verified, event)
	return l.signer.Verify(event)
}

func TestExporterCSVVerifiesOriginalSignedEvents(t *testing.T) {
	signer, err := NewSignerWithKey(bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		t.Fatal(err)
	}
	base := Event{ID: "=event", Timestamp: time.Date(2026, 10, 27, 12, 0, 0, 123, time.UTC), EventType: "+login",
		User: "\uFEFF@operator", IP: "-address", Path: "＝path", Success: true, Details: "\t=1+1,\"detail\"\nline two"}
	valid := base
	valid.Signature = signer.Sign(valid)
	legacy := base
	legacy.ID = "legacy"
	legacy.Signature = signer.signCanonical(signer.legacyTimeCanonicalForm(legacy))
	tampered := valid
	tampered.Details += " tampered"
	malformed := base
	malformed.Signature = "=invalid-signature"
	unsigned := base
	unsigned.Signature = ""
	unsigned.Success = false
	original := []Event{valid, legacy, tampered, malformed, unsigned}
	logger := &csvSignatureLogger{events: append([]Event(nil), original...), signer: signer}
	exporter := NewExporter(logger)
	for _, verify := range []bool{false, true} {
		for repeat := 0; repeat < 2; repeat++ {
			logger.verified = nil
			csvResult, err := exporter.Export(QueryFilter{}, ExportFormatCSV, verify)
			if err != nil {
				t.Fatal(err)
			}
			rows, err := csv.NewReader(bytes.NewReader(csvResult.Data)).ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 6 {
				t.Fatal("missing rows")
			}
			for i, row := range rows[1:] {
				signature := original[i].Signature
				if i == 3 {
					signature = "'" + signature
				}
				if row[3] != "'"+original[i].User || row[7] != "'"+original[i].Details || row[8] != signature {
					t.Fatalf("row %d lost literal text/signature", i)
				}
				if verify && row[9] != []string{"true", "true", "false", "false", ""}[i] {
					t.Fatalf("row %d changed original signature verdict: %q", i, row[9])
				}
			}
			if verify && !reflect.DeepEqual(logger.verified, original[:4]) || !verify && len(logger.verified) != 0 {
				t.Fatal("verification did not use exactly the original signed rows")
			}
			jsonResult, err := exporter.Export(QueryFilter{}, ExportFormatJSON, verify)
			if err != nil {
				t.Fatal(err)
			}
			var body struct {
				Events []ExportEvent `json:"events"`
			}
			if err := json.Unmarshal(jsonResult.Data, &body); err != nil {
				t.Fatal(err)
			}
			if len(body.Events) != len(original) {
				t.Fatal("JSON lost rows")
			}
			for i, got := range body.Events {
				want := original[i]
				if got.ID != want.ID || got.Timestamp != want.Timestamp || got.EventType != want.EventType || got.User != want.User || got.IP != want.IP || got.Path != want.Path || got.Success != want.Success || got.Details != want.Details || got.Signature != want.Signature {
					t.Fatal("JSON rewrote evidence")
				}
				if !verify || i == 4 {
					if got.SignatureValid != nil {
						t.Fatal("invented signature verdict")
					}
				} else if got.SignatureValid == nil || *got.SignatureValid != (i < 2) {
					t.Fatal("JSON signature verdict changed")
				}
			}
			if !reflect.DeepEqual(logger.events, original) {
				t.Fatal("export mutated signed source")
			}
		}
	}
}
