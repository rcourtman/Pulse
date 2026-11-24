package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

type agentReportErrorResponse struct {
	Code    string            `json:"code"`
	Details map[string]string `json:"details"`
}

func TestAgentReportHandlersClassifyBodyFailures(t *testing.T) {
	unified, _ := newUnifiedAgentHandlers(t, nil)
	kubernetes, _ := newKubernetesAgentHandlers(t, nil)

	tests := []struct {
		name         string
		handle       http.HandlerFunc
		path         string
		encodedLimit int64
		decodedLimit int64
	}{
		{
			name: "unified agent", handle: unified.HandleReport,
			path: "/api/agents/agent/report", encodedLimit: 256 * 1024, decodedLimit: 1536 * 1024,
		},
		{
			name: "kubernetes agent", handle: kubernetes.HandleReport,
			path: "/api/agents/kubernetes/report", encodedLimit: 2 * 1024 * 1024, decodedLimit: 10 * 1024 * 1024,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name+" encoded limit", func(t *testing.T) {
			body := oversizedJSON(tc.encodedLimit)
			rec := invokeAgentReportHandler(tc.handle, tc.path, body, "")
			assertAgentReportError(t, rec, http.StatusRequestEntityTooLarge, "report_too_large", agentReportSizeEncodedBody, tc.encodedLimit)
		})

		t.Run(tc.name+" decoded limit", func(t *testing.T) {
			body, err := utils.CompressJSON(oversizedJSON(tc.decodedLimit))
			if err != nil {
				t.Fatalf("compress oversized report: %v", err)
			}
			if int64(len(body)) >= tc.encodedLimit {
				t.Fatalf("fixture compressed to %d bytes, must stay below encoded limit %d", len(body), tc.encodedLimit)
			}
			rec := invokeAgentReportHandler(tc.handle, tc.path, body, "gzip")
			assertAgentReportError(t, rec, http.StatusRequestEntityTooLarge, "report_too_large", agentReportSizeDecodedJSON, tc.decodedLimit)
		})

		t.Run(tc.name+" malformed gzip", func(t *testing.T) {
			rec := invokeAgentReportHandler(tc.handle, tc.path, []byte("not a gzip stream"), "gzip")
			assertAgentReportError(t, rec, http.StatusBadRequest, "invalid_compression", "", 0)
		})

		t.Run(tc.name+" unsupported encoding", func(t *testing.T) {
			rec := invokeAgentReportHandler(tc.handle, tc.path, []byte(`{}`), "br")
			assertAgentReportError(t, rec, http.StatusUnsupportedMediaType, "unsupported_encoding", "", 0)
		})
	}
}

func TestAgentReportItemCounterCountsItemsAndRecords(t *testing.T) {
	tests := []struct {
		document string
		items    int64
		records  int64
	}{
		{`{}`, 1, 0},
		{`{"pods":[]}`, 1, 0},
		{`{"pods":[{},{},{}]}`, 4, 3},
		{`{"pods":[null, null ,0,"",[]]}`, 6, 5},
		{`{"pods":[{"labels":{"a":"b"},"containers":[{"name":"c"},{}]}],"cluster":{"id":"x"},"nodes":[{"roles":["a"]}]}`, 8, 2},
		{`{"tags":["a,b","[{]}","\\","\"{},{}\"",1.5e3,true]}`, 7, 6},
		{`{"host":{"packages":[{},{}]}}`, 4, 0},
		{`[[[]],[{}]]`, 4, 0},
	}

	for _, test := range tests {
		whole := newAgentReportItemCounter(1 << 20)
		if err := whole.add([]byte(test.document)); err != nil || whole.items != test.items || whole.records != test.records {
			t.Errorf("%s: %d items, %d records, err %v; want %d items, %d records",
				test.document, whole.items, whole.records, err, test.items, test.records)
		}
		chunked := newAgentReportItemCounter(1 << 20)
		for i := range len(test.document) {
			if err := chunked.add([]byte{test.document[i]}); err != nil {
				t.Errorf("%s fed a byte at a time: %v", test.document, err)
			}
		}
		if chunked.items != test.items || chunked.records != test.records {
			t.Errorf("%s fed a byte at a time: %d items, %d records; want %d items, %d records",
				test.document, chunked.items, chunked.records, test.items, test.records)
		}
	}
}

func TestAgentReportHandlersRejectReportsOverItemLimits(t *testing.T) {
	unified, _ := newUnifiedAgentHandlers(t, nil)
	kubernetes, _ := newKubernetesAgentHandlers(t, nil)
	docker := &DockerAgentHandlers{}

	tests := []struct {
		name         string
		handle       http.HandlerFunc
		path         string
		list         string
		nested       string
		decodedLimit int64
	}{
		{"unified agent", unified.HandleReport, "/api/agents/agent/report", "network", "addresses", 1536 * 1024},
		{"kubernetes agent", kubernetes.HandleReport, "/api/agents/kubernetes/report", "pods", "containers", 10 * 1024 * 1024},
		{"docker agent", docker.HandleReport, "/api/agents/docker/report", "containers", "ports", agentsdocker.ReportDecodedBodyLimitBytes},
	}

	for _, tc := range tests {
		maxRecords := tc.decodedLimit / agentReportDecodedBytesPerRecord
		maxItems := tc.decodedLimit / agentReportDecodedBytesPerItem
		documents := []struct {
			name      string
			document  []byte
			dimension string
			limit     int64
		}{
			{"empty records", itemListJSON(tc.list, `{}`, maxRecords+1), agentReportSizeRecords, maxRecords},
			{"null records", itemListJSON(tc.list, `null`, maxRecords+1), agentReportSizeRecords, maxRecords},
			{"named records", itemListJSON(tc.list, `{"id":"a","uid":"a","name":"a"}`, maxRecords+1), agentReportSizeRecords, maxRecords},
			// One record whose nested list carries the items: the report object
			// and the record are two, so maxItems-1 more are one over.
			{
				"nested items",
				[]byte(`{"` + tc.list + `":[` + string(itemListJSON(tc.nested, `{}`, maxItems-1)) + `]}`),
				agentReportSizeItems, maxItems,
			},
		}
		for _, doc := range documents {
			t.Run(tc.name+" "+doc.name, func(t *testing.T) {
				if int64(len(doc.document)) >= tc.decodedLimit {
					t.Fatalf("fixture is %d bytes, must stay below decoded limit %d", len(doc.document), tc.decodedLimit)
				}
				body, err := utils.CompressJSON(doc.document)
				if err != nil {
					t.Fatalf("compress report: %v", err)
				}

				rec := invokeAgentReportHandler(tc.handle, tc.path, body, "gzip")
				if rec.Code != http.StatusRequestEntityTooLarge {
					t.Fatalf("status = %d, want 413: %s", rec.Code, rec.Body.String())
				}
				var response agentReportErrorResponse
				if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
					t.Fatalf("decode error response: %v", err)
				}
				if response.Code != "report_too_large" || response.Details["dimension"] != doc.dimension ||
					response.Details["limitCount"] != strconv.FormatInt(doc.limit, 10) {
					t.Fatalf("response = %+v, want report_too_large for %s with limit %d", response, doc.dimension, doc.limit)
				}
			})
		}
	}
}

func TestDecodeCompressedAgentReportItemLimitsAreInclusive(t *testing.T) {
	const decodedLimit = 64 * 1024
	const maxRecords = decodedLimit / agentReportDecodedBytesPerRecord
	const maxItems = decodedLimit / agentReportDecodedBytesPerItem

	type record struct {
		Nested []struct{} `json:"nested"`
	}
	decode := func(document []byte) (*httptest.ResponseRecorder, []record, bool) {
		req := httptest.NewRequest(http.MethodPost, "/report", bytes.NewReader(document))
		rec := httptest.NewRecorder()
		var report struct {
			Records []record `json:"records"`
		}
		ok := decodeCompressedAgentReport(rec, req, decodedLimit, decodedLimit, &report)
		return rec, report.Records, ok
	}
	// A report of one record holding count nested items: count+2 items in all.
	nested := func(count int64) []byte {
		return []byte(`{"records":[` + string(itemListJSON("nested", `{}`, count)) + `]}`)
	}

	if rec, records, ok := decode(itemListJSON("records", `{}`, maxRecords)); !ok || len(records) != maxRecords {
		t.Fatalf("report with exactly %d records: ok = %v, decoded %d: %s", maxRecords, ok, len(records), rec.Body.String())
	}
	if rec, records, ok := decode(nested(maxItems - 2)); !ok || len(records) != 1 || len(records[0].Nested) != maxItems-2 {
		t.Fatalf("report with exactly %d items: ok = %v: %s", maxItems, ok, rec.Body.String())
	}

	for name, document := range map[string][]byte{
		"one record over": itemListJSON("records", `{}`, maxRecords+1),
		"one item over":   nested(maxItems - 1),
	} {
		rec, records, ok := decode(document)
		if ok || len(records) != 0 {
			t.Fatalf("%s: ok = %v, decoded %d records; want it refused undecoded", name, ok, len(records))
		}
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("%s: status = %d, want 413: %s", name, rec.Code, rec.Body.String())
		}
	}

	// A complete report followed by more records must not pass on its first value.
	trailing := append(itemListJSON("records", `{}`, 1), itemListJSON("records", `{}`, maxRecords)...)
	if rec, _, ok := decode(trailing); ok || rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("report with trailing records: ok = %v, status = %d, want 413", ok, rec.Code)
	}
}

// itemListJSON returns a report object whose one list holds count elements.
func itemListJSON(list string, element string, count int64) []byte {
	document := []byte(`{"` + list + `":[`)
	document = append(document, bytes.Repeat([]byte(element+","), int(count-1))...)
	return append(document, element+"]}"...)
}

func oversizedJSON(limit int64) []byte {
	prefix := []byte(`{"padding":"`)
	suffix := []byte(`"}`)
	padding := limit + 1 - int64(len(prefix)) - int64(len(suffix))
	if padding < 1 {
		padding = 1
	}
	body := make([]byte, 0, int64(len(prefix))+padding+int64(len(suffix)))
	body = append(body, prefix...)
	body = append(body, bytes.Repeat([]byte("x"), int(padding))...)
	body = append(body, suffix...)
	return body
}

func invokeAgentReportHandler(handle http.HandlerFunc, path string, body []byte, encoding string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	if encoding != "" {
		req.Header.Set("Content-Encoding", encoding)
	}
	rec := httptest.NewRecorder()
	handle(rec, req)
	return rec
}

func assertAgentReportError(
	t *testing.T,
	rec *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
	wantDimension string,
	wantLimit int64,
) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d: %s", rec.Code, wantStatus, rec.Body.String())
	}
	var response agentReportErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Code != wantCode {
		t.Fatalf("code = %q, want %q", response.Code, wantCode)
	}
	if wantDimension == "" {
		return
	}
	if response.Details["dimension"] != wantDimension {
		t.Fatalf("dimension = %q, want %q", response.Details["dimension"], wantDimension)
	}
	if response.Details["limitBytes"] != strconv.FormatInt(wantLimit, 10) {
		t.Fatalf("limitBytes = %q, want %d", response.Details["limitBytes"], wantLimit)
	}
}
