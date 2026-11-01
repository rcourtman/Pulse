package api

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// preIOStampHostReport is how a receiver built before ioCountersCollectedAt
// existed sees a host report: the identity, ordering and counter fields it
// already relies on, and no counter read stamp.
type preIOStampHostReport struct {
	Agent struct {
		ID              string `json:"id"`
		IntervalSeconds int    `json:"intervalSeconds"`
	} `json:"agent"`
	Host struct {
		ID       string `json:"id"`
		Hostname string `json:"hostname"`
	} `json:"host"`
	Network []struct {
		Name    string `json:"name"`
		RXBytes uint64 `json:"rxBytes"`
	} `json:"network"`
	DiskIO []struct {
		Device    string `json:"device"`
		ReadBytes uint64 `json:"readBytes"`
	} `json:"diskIO"`
	Timestamp  time.Time `json:"timestamp"`
	SequenceID string    `json:"sequenceId"`
}

const ioStampedHostReportJSON = `{"agent":{"id":"agent-a","intervalSeconds":30},` +
	`"host":{"id":"host-a","hostname":"host-a"},` +
	`"network":[{"name":"eth0","rxBytes":1000}],"diskIO":[{"device":"sda","readBytes":2000}],` +
	`"timestamp":"2026-10-30T12:00:50Z","sequenceId":"v1:stream-a:2",` +
	`"ioCountersCollectedAt":"2026-10-30T12:00:30.123456789Z"}`

// Preservation check: the report decoder accepts unknown fields, so a
// receiver without the optional counter read stamp still decodes a stamped
// report, plain or gzip, keeps every field it knows and ignores the stamp.
// It passes before and after the stamp is added.
func TestDecodeCompressedAgentReportIgnoresIOCounterStampOnPreChangeReceiver(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(ioStampedHostReportJSON)); err != nil {
		t.Fatalf("gzip report: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	for _, tc := range []struct {
		name     string
		body     []byte
		encoding string
	}{
		{"uncompressed", []byte(ioStampedHostReportJSON), ""},
		{"gzip", compressed.Bytes(), "gzip"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/report", bytes.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			if tc.encoding != "" {
				request.Header.Set("Content-Encoding", tc.encoding)
			}
			recorder := httptest.NewRecorder()

			var report preIOStampHostReport
			if !decodeCompressedAgentReport(recorder, request, 256*1024, 1536*1024, &report) {
				t.Fatalf("decoder refused a report carrying the optional stamp: %d %s", recorder.Code, recorder.Body.String())
			}
			if recorder.Body.Len() != 0 {
				t.Fatalf("decoder wrote a response on success: %s", recorder.Body.String())
			}
			wantTimestamp := time.Date(2026, 10, 30, 12, 0, 50, 0, time.UTC)
			if report.Agent.ID != "agent-a" || report.Agent.IntervalSeconds != 30 ||
				report.Host.ID != "host-a" || report.Host.Hostname != "host-a" ||
				!report.Timestamp.Equal(wantTimestamp) || report.SequenceID != "v1:stream-a:2" {
				t.Fatalf("known fields lost: %+v", report)
			}
			if len(report.Network) != 1 || report.Network[0].Name != "eth0" || report.Network[0].RXBytes != 1000 ||
				len(report.DiskIO) != 1 || report.DiskIO[0].Device != "sda" || report.DiskIO[0].ReadBytes != 2000 {
				t.Fatalf("counters lost: network %+v disk %+v", report.Network, report.DiskIO)
			}
		})
	}
}
