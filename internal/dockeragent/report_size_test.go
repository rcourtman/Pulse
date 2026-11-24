package dockeragent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
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

// reportSizeContractServer answers like the Pulse report endpoint: 413 for a
// body over the shared size contract, 200 otherwise. It records every report
// it accepts.
type reportSizeContractServer struct {
	*httptest.Server
	mu       sync.Mutex
	accepted []agentsdocker.Report
}

func newReportSizeContractServer(t *testing.T) *reportSizeContractServer {
	t.Helper()
	server := &reportSizeContractServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encoded, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		decoded, err := agentsdocker.DecodeReportBody(encoded, r.Header.Get("Content-Encoding"))
		var sizeErr *agentsdocker.ReportSizeError
		if errors.As(err, &sizeErr) {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_, _ = w.Write([]byte(`{"error":"report exceeds the server size limit","code":"report_too_large"}`))
			return
		}
		var report agentsdocker.Report
		if err != nil || json.Unmarshal(decoded, &report) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		server.mu.Lock()
		server.accepted = append(server.accepted, report)
		server.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return server
}

func (s *reportSizeContractServer) acceptedReports() []agentsdocker.Report {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]agentsdocker.Report(nil), s.accepted...)
}

// incompressibleText returns text that gzip cannot shrink by much.
func incompressibleText(size int) string {
	noise := make([]byte, size)
	rand.New(rand.NewSource(1)).Read(noise)
	return base64.StdEncoding.EncodeToString(noise)[:size]
}

func TestDeliverReportWithholdsMetadataOfOversizedObjects(t *testing.T) {
	server := newReportSizeContractServer(t)
	var output bytes.Buffer
	agent := &Agent{
		logger:       zerolog.New(&output),
		targets:      []TargetConfig{{URL: server.URL, Token: "token"}},
		httpClients:  map[bool]*http.Client{false: server.Client()},
		reportBuffer: utils.New[agentsdocker.Report](5),
	}
	noise := incompressibleText(4 * 1024 * 1024)
	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{ID: "host"},
		Containers: []agentsdocker.Container{
			{ID: "web", State: "exited", Labels: map[string]string{"com.docker.compose.project": "shop"}, Env: []string{"TZ=UTC"}},
			{ID: "batch-job", State: "running", Labels: map[string]string{"notes": noise}, Env: []string{"TZ=UTC"}},
		},
	}

	if err := agent.deliverReport(context.Background(), report); err != nil {
		t.Fatalf("deliver report: %v", err)
	}
	if buffered := agent.bufferedReports(); buffered != 0 {
		t.Fatalf("buffered %d reports, want none", buffered)
	}
	accepted := server.acceptedReports()
	if len(accepted) != 1 || len(accepted[0].Containers) != 2 {
		t.Fatalf("expected the server to accept one report with both containers, got %d reports", len(accepted))
	}
	web, batchJob := accepted[0].Containers[0], accepted[0].Containers[1]
	if web.State != "exited" || web.Labels["com.docker.compose.project"] != "shop" || len(web.Env) != 1 {
		t.Fatalf("ordinary container changed: state=%q labels=%d env=%d", web.State, len(web.Labels), len(web.Env))
	}
	if batchJob.State != "running" || len(batchJob.Labels) != 0 || len(batchJob.Env) != 0 {
		t.Fatalf("oversized container: state=%q labels=%d env=%d, want state without metadata", batchJob.State, len(batchJob.Labels), len(batchJob.Env))
	}
	if len(report.Containers[1].Labels) != 1 {
		t.Fatal("the caller's report must not be modified")
	}
	if !strings.Contains(output.String(), `"objectMetadataLimitBytes":32768`) {
		t.Fatalf("expected a diagnostic naming the metadata limit, got %s", output.String())
	}
}

func TestWithholdReportMetadata(t *testing.T) {
	small := map[string]string{"tier": "web"}
	large := map[string]string{"notes": strings.Repeat("x", 40*1024)}
	report := agentsdocker.Report{
		Containers: []agentsdocker.Container{
			{ID: "small", Labels: small, Env: []string{"TZ=UTC"}},
			{ID: "large-labels", Labels: large, Podman: &agentsdocker.PodmanContainer{PodName: "pod"}},
			{ID: "large-env", Env: []string{"BLOB=" + large["notes"]}},
		},
		Images:   []agentsdocker.Image{{ID: "small", Labels: small}, {ID: "large", Labels: large}},
		Volumes:  []agentsdocker.Volume{{Name: "small", Labels: small}, {Name: "large", Options: large}},
		Networks: []agentsdocker.Network{{ID: "small", Options: small}, {ID: "large", Labels: large}},
		Services: []agentsdocker.Service{{ID: "small", Labels: small, Stack: "shop"}, {ID: "large", Labels: large, Stack: "shop"}},
		Nodes:    []agentsdocker.Node{{ID: "small", Labels: small}, {ID: "large", EngineLabels: large}},
		Secrets:  []agentsdocker.Secret{{ID: "small", Labels: small}, {ID: "large", Labels: large}},
		Configs:  []agentsdocker.Config{{ID: "small", Labels: small}, {ID: "large", Labels: large}},
	}

	trimmed, withheld := withholdReportMetadata(report, 32*1024)
	if withheld != 9 {
		t.Fatalf("withheld metadata of %d objects, want 9", withheld)
	}
	if c := trimmed.Containers; c[0].Labels["tier"] != "web" || len(c[0].Env) != 1 ||
		c[1].Labels != nil || c[1].Podman != nil || c[2].Env != nil {
		t.Fatalf("unexpected containers: %d/%d labels, %d env", len(c[0].Labels), len(c[1].Labels), len(c[2].Env))
	}
	if trimmed.Images[0].Labels == nil || trimmed.Images[1].Labels != nil ||
		trimmed.Volumes[0].Labels == nil || trimmed.Volumes[1].Options != nil ||
		trimmed.Networks[0].Options == nil || trimmed.Networks[1].Labels != nil ||
		trimmed.Nodes[0].Labels == nil || trimmed.Nodes[1].EngineLabels != nil ||
		trimmed.Secrets[0].Labels == nil || trimmed.Secrets[1].Labels != nil ||
		trimmed.Configs[0].Labels == nil || trimmed.Configs[1].Labels != nil {
		t.Fatal("expected only the objects over the limit to lose their metadata")
	}
	if s := trimmed.Services; s[0].Stack != "shop" || s[0].Labels == nil || s[1].Stack != "" || s[1].Labels != nil {
		t.Fatalf("unexpected services: %+v", s)
	}
	if report.Containers[1].Labels == nil || report.Services[1].Stack != "shop" {
		t.Fatal("the caller's report must not be modified")
	}

	if _, withheld := withholdReportMetadata(trimmed, 0); withheld != 8 {
		t.Fatalf("last step withheld metadata of %d objects, want the 8 that still carry some", withheld)
	}
}

func TestDeliverReportDoesNotBufferReportRefusedAsTooLarge(t *testing.T) {
	server := newReportSizeContractServer(t)
	var output bytes.Buffer
	agent := &Agent{
		logger:       zerolog.New(&output),
		targets:      []TargetConfig{{URL: server.URL, Token: "token"}},
		httpClients:  map[bool]*http.Client{false: server.Client()},
		reportBuffer: utils.New[agentsdocker.Report](5),
	}
	// No metadata to withhold: the report stays over the limit.
	oversized := agentsdocker.Report{
		Agent:      agentsdocker.AgentInfo{ID: "oversized"},
		Containers: []agentsdocker.Container{{ID: "container-1", Status: incompressibleText(4 * 1024 * 1024)}},
	}

	if err := agent.deliverReport(context.Background(), oversized); err != nil {
		t.Fatalf("deliver oversized report: %v", err)
	}
	if buffered := agent.bufferedReports(); buffered != 0 {
		t.Fatalf("buffered %d reports the server can never accept", buffered)
	}
	if !strings.Contains(output.String(), `"reportEncodedLimitExceeded":true`) {
		t.Fatal("expected the size diagnostic before the report was sent")
	}

	if err := agent.deliverReport(context.Background(), agentsdocker.Report{Agent: agentsdocker.AgentInfo{ID: "ordinary"}}); err != nil {
		t.Fatalf("deliver ordinary report: %v", err)
	}
	if accepted := server.acceptedReports(); len(accepted) != 1 || accepted[0].Agent.ID != "ordinary" {
		t.Fatalf("accepted %d reports, want only the ordinary one", len(accepted))
	}
}

func TestFlushBufferDropsReportRefusedAsTooLarge(t *testing.T) {
	server := newReportSizeContractServer(t)
	agent := &Agent{
		logger:       zerolog.Nop(),
		targets:      []TargetConfig{{URL: server.URL, Token: "token"}},
		httpClients:  map[bool]*http.Client{false: server.Client()},
		reportBuffer: utils.New[agentsdocker.Report](5),
	}
	agent.reportBuffer.Push(agentsdocker.Report{
		Agent:      agentsdocker.AgentInfo{ID: "oversized"},
		Containers: []agentsdocker.Container{{ID: "container-1", Status: incompressibleText(4 * 1024 * 1024)}},
	})
	agent.reportBuffer.Push(agentsdocker.Report{Agent: agentsdocker.AgentInfo{ID: "queued"}})

	agent.flushBuffer(context.Background())

	if agent.reportBuffer.Len() != 0 {
		t.Fatalf("expected the buffer to drain, %d reports remain", agent.reportBuffer.Len())
	}
	if accepted := server.acceptedReports(); len(accepted) != 1 || accepted[0].Agent.ID != "queued" {
		t.Fatalf("accepted %d reports, want the one queued behind the oversized report", len(accepted))
	}
}

func TestDeliverReportWithholdsOversizedPortsWhenOverLimit(t *testing.T) {
	server := newReportSizeContractServer(t)
	agent := &Agent{
		logger:       zerolog.Nop(),
		targets:      []TargetConfig{{URL: server.URL, Token: "token"}},
		httpClients:  map[bool]*http.Client{false: server.Client()},
		reportBuffer: utils.New[agentsdocker.Report](5),
	}
	ports := make([]agentsdocker.ContainerPort, 0, 200000)
	for i := 0; i < 200000; i++ {
		ports = append(ports, agentsdocker.ContainerPort{
			PrivatePort: i%65535 + 1,
			PublicPort:  i%65535 + 1,
			Protocol:    "tcp",
			IP:          "0.0.0.0",
		})
	}
	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{ID: "many-ports"},
		Containers: []agentsdocker.Container{
			{ID: "heavy", Name: "heavy", State: "running", Ports: ports},
			{ID: "normal", Name: "normal", State: "running", Ports: []agentsdocker.ContainerPort{{PrivatePort: 80, PublicPort: 8080, Protocol: "tcp"}}},
		},
	}

	if err := agent.deliverReport(context.Background(), report); err != nil {
		t.Fatalf("deliver report with oversized ports: %v", err)
	}
	accepted := server.acceptedReports()
	if len(accepted) != 1 {
		t.Fatalf("accepted %d reports, want 1", len(accepted))
	}
	if len(accepted[0].Containers) != 2 {
		t.Fatalf("accepted %d containers, want 2", len(accepted[0].Containers))
	}
	if len(accepted[0].Containers[0].Ports) != 0 {
		t.Fatalf("expected oversized container ports to be withheld, got %d", len(accepted[0].Containers[0].Ports))
	}
	if len(accepted[0].Containers[1].Ports) != 1 {
		t.Fatalf("expected normal container ports to remain intact, got %d", len(accepted[0].Containers[1].Ports))
	}
}
