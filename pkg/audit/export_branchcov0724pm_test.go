package audit

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

// boolPtr returns the address of b so we can populate ExportEvent.SignatureValid.
func boolPtr(b bool) *bool { return &b }

// TestBranchcov0724pmExportCSV_EmptyEventsAndVerificationArms exercises the arms
// the existing Export test never reaches: an empty event slice, the
// includeVerification=false header branch, and every state of
// ExportEvent.SignatureValid (nil, true, false) inside the per-row block.
func TestBranchcov0724pmExportCSV_EmptyEventsAndVerificationArms(t *testing.T) {
	exporter := &Exporter{}

	// Arm 1: empty event list with includeVerification=false. The for-loop body
	// is skipped entirely; output must be a header-only CSV with NO
	// "Signature Valid" column.
	res, err := exporter.exportCSV(nil, "20240101-120000", false)
	if err != nil {
		t.Fatalf("exportCSV(nil,false): %v", err)
	}
	if res.EventCount != 0 {
		t.Fatalf("EventCount = %d, want 0", res.EventCount)
	}
	if res.ContentType != "text/csv; charset=utf-8" {
		t.Fatalf("ContentType = %q", res.ContentType)
	}
	if res.Filename != "audit-log-20240101-120000.csv" {
		t.Fatalf("Filename = %q", res.Filename)
	}
	// Header only, and it must NOT carry the verification column.
	reader := csv.NewReader(bytes.NewReader(res.Data))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse empty csv: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("want exactly 1 header record, got %d", len(records))
	}
	wantHeader := []string{"ID", "Timestamp", "Event Type", "User", "IP", "Path", "Success", "Details", "Signature", "Signature Version"}
	if len(records[0]) != len(wantHeader) {
		t.Fatalf("header has %d cols, want %d (%v)", len(records[0]), len(wantHeader), records[0])
	}
	for i, h := range wantHeader {
		if records[0][i] != h {
			t.Fatalf("header[%d] = %q, want %q", i, records[0][i], h)
		}
	}

	// Arm 2: includeVerification=true with all three SignatureValid states on a
	// single batch. This is the only way to reach the inner
	// `if event.SignatureValid != nil` block and both of its sub-arms.
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	events := []ExportEvent{
		{ID: "nil-verdict", Timestamp: ts, EventType: "login", SignatureValid: nil},
		{ID: "true-verdict", Timestamp: ts, EventType: "login", SignatureValid: boolPtr(true)},
		{ID: "false-verdict", Timestamp: ts, EventType: "login", SignatureValid: boolPtr(false)},
	}
	res, err = exporter.exportCSV(events, "20240101-120000", true)
	if err != nil {
		t.Fatalf("exportCSV(verification,true): %v", err)
	}
	if res.EventCount != 3 {
		t.Fatalf("EventCount = %d, want 3", res.EventCount)
	}
	reader = csv.NewReader(bytes.NewReader(res.Data))
	records, err = reader.ReadAll()
	if err != nil {
		t.Fatalf("parse verification csv: %v", err)
	}
	if len(records) != 4 { // header + 3 rows
		t.Fatalf("want 4 records, got %d", len(records))
	}
	// Header carries validity plus authoritative status and assurance columns.
	if records[0][len(records[0])-3] != "Signature Valid" || records[0][len(records[0])-2] != "Signature Status" || records[0][len(records[0])-1] != "Signature Assurance" {
		t.Fatalf("verification headers = %v", records[0])
	}
	// Per-row validity column must reflect each sub-arm exactly.
	wantVerdicts := map[string]string{
		"nil-verdict":   "",
		"true-verdict":  "true",
		"false-verdict": "false",
	}
	for _, row := range records[1:] {
		id := row[0]
		verdict := row[len(row)-3]
		if wantVerdicts[id] != verdict {
			t.Fatalf("row %q verdict col = %q, want %q", id, verdict, wantVerdicts[id])
		}
	}
}

// TestBranchcov0724pmExportCSV_SuccessFlagAndFieldRoundTrip pins the two Success
// arms (true/false) and confirms optional fields round-trip through a real CSV
// parser rather than merely that no error was returned.
func TestBranchcov0724pmExportCSV_SuccessFlagAndFieldRoundTrip(t *testing.T) {
	exporter := &Exporter{}
	ts := time.Date(2024, 6, 15, 9, 30, 0, 0, time.UTC)
	events := []ExportEvent{
		{
			ID: "ok", Timestamp: ts, EventType: "login", User: "alice", IP: "10.0.0.1",
			Path: "/login", Success: true, Details: "ok", Signature: "sig-ok",
		},
		{
			ID: "fail", Timestamp: ts, EventType: "login", User: "bob", IP: "10.0.0.2",
			Path: "/login", Success: false, Details: "bad", Signature: "sig-bad",
		},
	}

	res, err := exporter.exportCSV(events, "20240101-120000", false)
	if err != nil {
		t.Fatalf("exportCSV: %v", err)
	}
	reader := csv.NewReader(bytes.NewReader(res.Data))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("want header+2 rows, got %d", len(records))
	}
	// Success column index 6 must be "true"/"false" (the only two arms).
	if records[1][6] != "true" {
		t.Fatalf("success col for ok row = %q, want \"true\"", records[1][6])
	}
	if records[2][6] != "false" {
		t.Fatalf("success col for fail row = %q, want \"false\"", records[2][6])
	}
	// Round-trip every populated field of the first row.
	if records[1][0] != "ok" || records[1][3] != "alice" || records[1][4] != "10.0.0.1" ||
		records[1][5] != "/login" || records[1][7] != "ok" || records[1][8] != "sig-ok" {
		t.Fatalf("row fields did not round-trip: %v", records[1])
	}
	// Timestamp column must be RFC3339 formatted.
	if _, perr := time.Parse(time.RFC3339, records[1][1]); perr != nil {
		t.Fatalf("timestamp %q not RFC3339: %v", records[1][1], perr)
	}
}

// TestBranchcov0724pmExportCSV_CSVEscaping verifies that detail fields
// containing commas, double quotes and embedded newlines survive a CSV
// write/parse round-trip byte-for-byte (RFC 4180 quoting).
func TestBranchcov0724pmExportCSV_CSVEscaping(t *testing.T) {
	exporter := &Exporter{}
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	specials := []string{
		"plain",
		"has,comma",
		`has"quote`,
		"has\nnewline",
		`mix"d, and\nall`,
		"", // empty detail must also survive
	}
	events := make([]ExportEvent, 0, len(specials))
	for i, d := range specials {
		events = append(events, ExportEvent{
			ID: "e" + strconv.Itoa(i), Timestamp: ts, EventType: "t", Details: d,
		})
	}

	res, err := exporter.exportCSV(events, "ts", false)
	if err != nil {
		t.Fatalf("exportCSV: %v", err)
	}
	reader := csv.NewReader(bytes.NewReader(res.Data))
	// Allow variable number of fields per record (relaxed) just in case, but we
	// expect consistent columns here.
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(records) != len(specials)+1 {
		t.Fatalf("want %d records, got %d", len(specials)+1, len(records))
	}
	for i, want := range specials {
		got := records[i+1][7] // Details column
		if got != want {
			t.Fatalf("detail[%d] round-trip = %q, want %q", i, got, want)
		}
	}
	// Prove the raw bytes actually contain quoting for the comma case, so a
	// naive split on "," would have broken (defends against a future regression
	// to unescaped output).
	if !bytes.Contains(res.Data, []byte(`"has,comma"`)) {
		t.Fatalf("expected quoted comma field in output: %q", res.Data)
	}
	if !bytes.Contains(res.Data, []byte(`"has""quote"`)) {
		t.Fatalf("expected doubled-quote escaping in output: %q", res.Data)
	}
}

// TestBranchcov0724pmExportJSON_RoundTripAndOmitempty covers the empty slice,
// single event, special-character detail, and the omitempty behaviour of
// ExportEvent. It parses the produced JSON back rather than asserting on a
// brittle timestamped blob.
func TestBranchcov0724pmExportJSON_RoundTripAndOmitempty(t *testing.T) {
	exporter := &Exporter{}

	// Empty (nil) slice: EventCount must be 0 and the wrapper still well-formed.
	res, err := exporter.exportJSON(nil, "20240101-120000")
	if err != nil {
		t.Fatalf("exportJSON(nil): %v", err)
	}
	if res.EventCount != 0 {
		t.Fatalf("EventCount = %d, want 0", res.EventCount)
	}
	if res.ContentType != "application/json; charset=utf-8" {
		t.Fatalf("ContentType = %q", res.ContentType)
	}
	if res.Filename != "audit-log-20240101-120000.json" {
		t.Fatalf("Filename = %q", res.Filename)
	}
	var empty struct {
		EventCount int           `json:"event_count"`
		Events     []ExportEvent `json:"events"`
		ExportedAt time.Time     `json:"exported_at"`
	}
	if err := json.Unmarshal(res.Data, &empty); err != nil {
		t.Fatalf("unmarshal empty json: %v", err)
	}
	if empty.EventCount != 0 || len(empty.Events) != 0 {
		t.Fatalf("empty json = %+v", empty)
	}
	if empty.ExportedAt.IsZero() {
		t.Fatal("ExportedAt should be populated")
	}

	// omitempty: an event with only required fields must omit every optional key.
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	bareEvents := []ExportEvent{{ID: "bare", Timestamp: ts, EventType: "login", Success: false}}
	res, err = exporter.exportJSON(bareEvents, "ts")
	if err != nil {
		t.Fatalf("exportJSON(bare): %v", err)
	}
	if !bytes.Contains(res.Data, []byte(`"event_count": 1`)) {
		t.Fatalf("expected event_count=1 in %s", res.Data)
	}
	// Parse the first event into a generic map so we can reason about the exact
	// key set rather than matching against the indented (space-padded) bytes.
	var wrapper struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.Unmarshal(res.Data, &wrapper); err != nil {
		t.Fatalf("unmarshal bare json: %v", err)
	}
	if len(wrapper.Events) != 1 {
		t.Fatalf("want 1 bare event, got %d", len(wrapper.Events))
	}
	ev := wrapper.Events[0]
	// The optional string fields must NOT appear as keys.
	for _, key := range []string{"user", "ip", "path", "details", "signature", "signature_valid"} {
		if _, ok := ev[key]; ok {
			t.Fatalf("omitempty leaked key %q into output: %s", key, res.Data)
		}
	}
	// Required keys must appear with their concrete values.
	for k, want := range map[string]any{"id": "bare", "event_type": "login", "success": false} {
		got, ok := ev[k]
		if !ok {
			t.Fatalf("missing required key %q in %s", k, res.Data)
		}
		if got != want {
			t.Fatalf("key %q = %v, want %v", k, got, want)
		}
	}

	// Full event with special chars in Details round-trips byte-for-byte.
	fullEvents := []ExportEvent{{
		ID: "full", Timestamp: ts, EventType: "config", User: "a,b", IP: "10.0.0.9",
		Path: "/p", Success: true, Details: "line1\nline2\ttab", Signature: "0dead",
		SignatureValid: boolPtr(true),
	}}
	res, err = exporter.exportJSON(fullEvents, "ts")
	if err != nil {
		t.Fatalf("exportJSON(full): %v", err)
	}
	var parsed struct {
		Events []ExportEvent `json:"events"`
	}
	if err := json.Unmarshal(res.Data, &parsed); err != nil {
		t.Fatalf("unmarshal full json: %v", err)
	}
	if len(parsed.Events) != 1 {
		t.Fatalf("want 1 event, got %d", len(parsed.Events))
	}
	got := parsed.Events[0]
	if got.Details != "line1\nline2\ttab" || got.User != "a,b" || got.Signature != "0dead" {
		t.Fatalf("full event did not round-trip: %+v", got)
	}
	if got.SignatureValid == nil || *got.SignatureValid != true {
		t.Fatalf("SignatureValid did not round-trip: %v", got.SignatureValid)
	}
}

// TestBranchcov0724pmExportJSON_NilVsEmptySlice distinguishes the two zero-event
// shapes a caller can pass: a nil slice encodes as JSON null, a non-nil empty
// slice encodes as []. Both must marshal without error (the error arm of
// exportJSON is only reachable with an unmarshalable type, which ExportEvent
// cannot hold - see report).
func TestBranchcov0724pmExportJSON_NilVsEmptySlice(t *testing.T) {
	exporter := &Exporter{}

	nilRes, err := exporter.exportJSON(nil, "ts")
	if err != nil {
		t.Fatalf("exportJSON(nil): %v", err)
	}
	// Decode into a RawMessage map so we can distinguish JSON null from [];
	// both decode to a nil []ExportEvent, so the typed round-trip cannot tell
	// them apart.
	var nilRaw map[string]json.RawMessage
	if err := json.Unmarshal(nilRes.Data, &nilRaw); err != nil {
		t.Fatalf("unmarshal nil json: %v", err)
	}
	// A nil []ExportEvent serialises to the JSON literal `null`.
	if string(nilRaw["events"]) != "null" {
		t.Fatalf("nil slice should encode events as null, got %q: %s", nilRaw["events"], nilRes.Data)
	}

	emptyRes, err := exporter.exportJSON([]ExportEvent{}, "ts")
	if err != nil {
		t.Fatalf("exportJSON(empty): %v", err)
	}
	var emptyRaw map[string]json.RawMessage
	if err := json.Unmarshal(emptyRes.Data, &emptyRaw); err != nil {
		t.Fatalf("unmarshal empty json: %v", err)
	}
	// A non-nil empty slice serialises to `[]`.
	if string(emptyRaw["events"]) != "[]" {
		t.Fatalf("empty slice should encode events as [], got %q: %s", emptyRaw["events"], emptyRes.Data)
	}
}
