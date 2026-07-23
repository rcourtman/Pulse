package dockeragent

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

func TestLogDockerReportSizeUsesSharedContract(t *testing.T) {
	tests := []struct {
		name                string
		encodedBytes        int64
		decodedBytes        int64
		wantLog             bool
		wantLevel           string
		wantEncodedExceeded bool
		wantDecodedExceeded bool
	}{
		{
			name:         "ordinary fleet",
			encodedBytes: 64 * 1024,
			decodedBytes: 512 * 1024,
		},
		{
			name:         "reported 890 KiB fleet no longer warns",
			encodedBytes: 100 * 1024,
			decodedBytes: 890 * 1024,
		},
		{
			name:         "encoded warning boundary",
			encodedBytes: agentsdocker.ReportEncodedBodyWarningBytes,
			decodedBytes: 890 * 1024,
			wantLog:      true,
			wantLevel:    "warn",
		},
		{
			name:         "decoded warning boundary",
			encodedBytes: 100 * 1024,
			decodedBytes: agentsdocker.ReportDecodedBodyWarningBytes,
			wantLog:      true,
			wantLevel:    "warn",
		},
		{
			name:                "encoded over limit",
			encodedBytes:        agentsdocker.ReportEncodedBodyLimitBytes + 1,
			decodedBytes:        agentsdocker.ReportDecodedBodyWarningBytes,
			wantLog:             true,
			wantLevel:           "error",
			wantEncodedExceeded: true,
		},
		{
			name:                "decoded over limit",
			encodedBytes:        agentsdocker.ReportEncodedBodyWarningBytes,
			decodedBytes:        agentsdocker.ReportDecodedBodyLimitBytes + 1,
			wantLog:             true,
			wantLevel:           "error",
			wantDecodedExceeded: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			agent := &Agent{logger: zerolog.New(&output)}

			agent.logReportSize(163, test.encodedBytes, test.decodedBytes)

			if !test.wantLog {
				if output.Len() != 0 {
					t.Fatalf("unexpected size diagnostic: %s", output.String())
				}
				return
			}

			var event map[string]any
			if err := json.Unmarshal(output.Bytes(), &event); err != nil {
				t.Fatalf("decode log event: %v: %s", err, output.String())
			}
			if event["level"] != test.wantLevel {
				t.Fatalf("level = %v, want %q", event["level"], test.wantLevel)
			}
			if event["containers"] != float64(163) {
				t.Fatalf("containers = %v, want 163", event["containers"])
			}
			if event["reportEncodedBytes"] != float64(test.encodedBytes) {
				t.Fatalf("reportEncodedBytes = %v, want %d", event["reportEncodedBytes"], test.encodedBytes)
			}
			if event["reportDecodedBytes"] != float64(test.decodedBytes) {
				t.Fatalf("reportDecodedBytes = %v, want %d", event["reportDecodedBytes"], test.decodedBytes)
			}
			if event["reportEncodedLimitBytes"] != float64(agentsdocker.ReportEncodedBodyLimitBytes) {
				t.Fatalf("encoded limit = %v", event["reportEncodedLimitBytes"])
			}
			if event["reportDecodedLimitBytes"] != float64(agentsdocker.ReportDecodedBodyLimitBytes) {
				t.Fatalf("decoded limit = %v", event["reportDecodedLimitBytes"])
			}
			if event["reportEncodedWarningBytes"] != float64(agentsdocker.ReportEncodedBodyWarningBytes) {
				t.Fatalf("encoded warning = %v", event["reportEncodedWarningBytes"])
			}
			if event["reportDecodedWarningBytes"] != float64(agentsdocker.ReportDecodedBodyWarningBytes) {
				t.Fatalf("decoded warning = %v", event["reportDecodedWarningBytes"])
			}
			if event["reportEncodedLimitExceeded"] != test.wantEncodedExceeded {
				t.Fatalf("encoded exceeded = %v, want %v", event["reportEncodedLimitExceeded"], test.wantEncodedExceeded)
			}
			if event["reportDecodedLimitExceeded"] != test.wantDecodedExceeded {
				t.Fatalf("decoded exceeded = %v, want %v", event["reportDecodedLimitExceeded"], test.wantDecodedExceeded)
			}

			message, _ := event["message"].(string)
			if !strings.Contains(message, agentsdocker.ReportSizeLimitDescription()) {
				t.Fatalf("message does not use shared limit description: %q", message)
			}
			if strings.Contains(message, "512") || strings.Contains(message, "KB") {
				t.Fatalf("message contains obsolete limit wording: %q", message)
			}
		})
	}
}
