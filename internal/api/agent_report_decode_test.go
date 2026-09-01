package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
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
