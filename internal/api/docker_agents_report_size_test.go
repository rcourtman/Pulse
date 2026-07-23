package api

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func TestDockerAgentReportSizeContractAcceptsOrdinaryAndLargeFleets(t *testing.T) {
	tests := []struct {
		name       string
		runtime    string
		containers int
		compressed bool
	}{
		{name: "ordinary Docker uncompressed", runtime: "docker", containers: 1},
		{name: "large Docker compressed", runtime: "docker", containers: 163, compressed: true},
		{name: "ordinary Podman compressed", runtime: "podman", containers: 1, compressed: true},
		{name: "large Podman uncompressed", runtime: "podman", containers: 163},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, _ := newDockerAgentHandlers(t, nil)
			body := dockerFleetReportBody(t, test.runtime, test.containers)
			if test.compressed {
				body = gzipAPIReportBody(t, body)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/agents/docker/report", bytes.NewReader(body))
			if test.compressed {
				req.Header.Set("Content-Encoding", "gzip")
			}
			rec := httptest.NewRecorder()
			handler.HandleReport(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestDockerAgentReportSizeContractBoundaries(t *testing.T) {
	t.Run("uncompressed exact encoded limit accepted", func(t *testing.T) {
		handler, _ := newDockerAgentHandlers(t, nil)
		body := padDockerReportBody(t, dockerFleetReportBody(t, "docker", 1), agentsdocker.ReportEncodedBodyLimitBytes)
		rec := postDockerReport(handler, body, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("uncompressed byte over encoded limit rejected", func(t *testing.T) {
		handler := &DockerAgentHandlers{}
		body := padDockerReportBody(t, dockerFleetReportBody(t, "docker", 1), agentsdocker.ReportEncodedBodyLimitBytes+1)
		rec := postDockerReport(handler, body, "")
		assertDockerReportTooLarge(t, rec, agentsdocker.ReportSizeEncodedBody, agentsdocker.ReportEncodedBodyLimitBytes, 0)
	})

	t.Run("gzip exact encoded limit accepted", func(t *testing.T) {
		handler, _ := newDockerAgentHandlers(t, nil)
		body := gzipAPIReportBodyWithEncodedSize(
			t,
			dockerFleetReportBody(t, "docker", 1),
			agentsdocker.ReportEncodedBodyLimitBytes,
		)
		rec := postDockerReport(handler, body, "gzip")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("gzip byte over encoded limit rejected before decompression", func(t *testing.T) {
		handler := &DockerAgentHandlers{}
		body := gzipAPIReportBodyWithEncodedSize(
			t,
			dockerFleetReportBody(t, "docker", 1),
			agentsdocker.ReportEncodedBodyLimitBytes+1,
		)
		rec := postDockerReport(handler, body, "gzip")
		assertDockerReportTooLarge(t, rec, agentsdocker.ReportSizeEncodedBody, agentsdocker.ReportEncodedBodyLimitBytes, 0)
	})

	t.Run("gzip exact decoded limit accepted", func(t *testing.T) {
		handler, _ := newDockerAgentHandlers(t, nil)
		decoded := padDockerReportBody(t, dockerFleetReportBody(t, "podman", 1), agentsdocker.ReportDecodedBodyLimitBytes)
		rec := postDockerReport(handler, gzipAPIReportBody(t, decoded), "gzip")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("gzip byte over decoded limit rejected", func(t *testing.T) {
		handler := &DockerAgentHandlers{}
		decoded := padDockerReportBody(t, dockerFleetReportBody(t, "podman", 1), agentsdocker.ReportDecodedBodyLimitBytes+1)
		rec := postDockerReport(handler, gzipAPIReportBody(t, decoded), "gzip")
		assertDockerReportTooLarge(
			t,
			rec,
			agentsdocker.ReportSizeDecodedBody,
			agentsdocker.ReportDecodedBodyLimitBytes,
			agentsdocker.ReportDecodedBodyLimitBytes+1,
		)
	})
}

func TestDockerAgentReportCompressionDiagnostics(t *testing.T) {
	t.Run("unsupported encoding", func(t *testing.T) {
		handler := &DockerAgentHandlers{}
		rec := postDockerReport(handler, []byte("{}"), "zstd")
		if rec.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("status = %d, want 415: %s", rec.Code, rec.Body.String())
		}
		assertDockerReportErrorCode(t, rec, "unsupported_encoding")
	})

	t.Run("malformed gzip", func(t *testing.T) {
		handler := &DockerAgentHandlers{}
		rec := postDockerReport(handler, []byte("not-gzip"), "gzip")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
		}
		assertDockerReportErrorCode(t, rec, "invalid_compression")
	})
}

func dockerFleetReportBody(t *testing.T, runtime string, containerCount int) []byte {
	t.Helper()

	containers := make([]agentsdocker.Container, containerCount)
	for index := range containers {
		containers[index] = agentsdocker.Container{
			ID:     "container-" + strconv.Itoa(index),
			Name:   runtime + "-container-" + strconv.Itoa(index),
			Image:  "example/image:latest",
			State:  "running",
			Status: "Up",
		}
	}

	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              runtime + "-agent-" + strconv.Itoa(containerCount),
			Version:         "6.1.1",
			Type:            "unified",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:  runtime + "-host-" + strconv.Itoa(containerCount),
			MachineID: runtime + "-machine-" + strconv.Itoa(containerCount),
			Runtime:   runtime,
		},
		Containers: containers,
		Timestamp:  time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
	}

	body, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	return body
}

func padDockerReportBody(t *testing.T, body []byte, size int64) []byte {
	t.Helper()

	if int64(len(body)) > size {
		t.Fatalf("base report is %d bytes, exceeds requested size %d", len(body), size)
	}
	return append(body, bytes.Repeat([]byte(" "), int(size)-len(body))...)
}

func gzipAPIReportBody(t *testing.T, body []byte) []byte {
	t.Helper()

	var encoded bytes.Buffer
	writer := gzip.NewWriter(&encoded)
	if _, err := writer.Write(body); err != nil {
		t.Fatalf("write gzip report: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip report: %v", err)
	}
	return encoded.Bytes()
}

func gzipAPIReportBodyWithEncodedSize(t *testing.T, body []byte, size int64) []byte {
	t.Helper()

	encoded := gzipAPIReportBody(t, body)
	remaining := size - int64(len(encoded))
	if remaining <= 0 {
		t.Fatalf("base gzip body is %d bytes, cannot pad to %d", len(encoded), size)
	}

	// Concatenated empty gzip members keep the decoded JSON unchanged. Their
	// bounded FEXTRA fields deterministically fill the remaining encoded bytes
	// while preserving a format-valid gzip entity.
	baseMemberSize := int64(len(gzipAPIReportBody(t, nil)))
	const maxExtraSize = int64(0xffff)
	maxMemberSize := baseMemberSize + 2 + maxExtraSize
	memberCount := (remaining + maxMemberSize - 1) / maxMemberSize
	if remaining < memberCount*baseMemberSize {
		t.Fatalf("remaining gzip padding = %d, cannot fit %d members", remaining, memberCount)
	}

	additionalBytes := remaining - memberCount*baseMemberSize
	for memberIndex := int64(0); memberIndex < memberCount; memberIndex++ {
		memberAdditionalBytes := additionalBytes / (memberCount - memberIndex)
		additionalBytes -= memberAdditionalBytes
		if memberAdditionalBytes == 1 || memberAdditionalBytes > maxExtraSize+2 {
			t.Fatalf("gzip member additional bytes = %d, want 0 or [2,%d]", memberAdditionalBytes, maxExtraSize+2)
		}

		var member bytes.Buffer
		writer := gzip.NewWriter(&member)
		if memberAdditionalBytes >= 2 {
			writer.Extra = bytes.Repeat([]byte("x"), int(memberAdditionalBytes-2))
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("close padding gzip member: %v", err)
		}
		encoded = append(encoded, member.Bytes()...)
	}
	if int64(len(encoded)) != size {
		t.Fatalf("encoded gzip size = %d, want %d", len(encoded), size)
	}
	return encoded
}

func postDockerReport(handler *DockerAgentHandlers, body []byte, contentEncoding string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/agents/docker/report", bytes.NewReader(body))
	if contentEncoding != "" {
		req.Header.Set("Content-Encoding", contentEncoding)
	}
	rec := httptest.NewRecorder()
	handler.HandleReport(rec, req)
	return rec
}

func assertDockerReportTooLarge(
	t *testing.T,
	rec *httptest.ResponseRecorder,
	dimension agentsdocker.ReportSizeDimension,
	limit int64,
	actual int64,
) {
	t.Helper()

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Code    string            `json:"code"`
		Error   string            `json:"error"`
		Details map[string]string `json:"details"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != "report_too_large" {
		t.Fatalf("code = %q, want report_too_large", response.Code)
	}
	if response.Details["dimension"] != string(dimension) {
		t.Fatalf("dimension = %q, want %q", response.Details["dimension"], dimension)
	}
	if response.Details["limitBytes"] != strconv.FormatInt(limit, 10) {
		t.Fatalf("limitBytes = %q, want %d", response.Details["limitBytes"], limit)
	}
	if !bytes.Contains([]byte(response.Error), []byte(agentsdocker.ReportSizeLimitDescription())) {
		t.Fatalf("error does not use shared limit description: %q", response.Error)
	}
	if actual == 0 {
		if _, present := response.Details["actualBytes"]; present {
			t.Fatalf("actualBytes = %q, want omitted when MaxBytesReader only proves an overage", response.Details["actualBytes"])
		}
	} else if response.Details["actualBytes"] != strconv.FormatInt(actual, 10) {
		t.Fatalf("actualBytes = %q, want %d", response.Details["actualBytes"], actual)
	}
}

func assertDockerReportErrorCode(t *testing.T, rec *httptest.ResponseRecorder, code string) {
	t.Helper()

	var response struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != code {
		t.Fatalf("code = %q, want %q", response.Code, code)
	}
}
