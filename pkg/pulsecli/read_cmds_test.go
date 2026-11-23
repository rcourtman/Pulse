package pulsecli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

func newTestClientRootWithRemote(server *httptest.Server, isTTY bool, now time.Time) (*bytes.Buffer, func(...string) error) {
	var out bytes.Buffer
	store := func() func(string) string {
		return func(key string) string {
			switch key {
			case "PULSE_API_URL":
				return server.URL
			case "PULSE_API_TOKEN":
				return "synthetic-cli-token"
			default:
				return ""
			}
		}
	}()
	remoteDeps := &RemoteDeps{
		HTTPClient: server.Client(),
		Getenv:     store,
		IsTerminal: func(io.Writer) bool { return isTTY },
		Now:        func() time.Time { return now },
	}
	fleetDeps := &FleetDeps{
		HTTPClient: server.Client(),
		Getenv:     store,
		IsTerminal: func(io.Writer) bool { return isTTY },
	}
	actionsDeps := &ActionsDeps{
		HTTPClient: server.Client(),
		Getenv:     store,
	}
	run := func(args ...string) error {
		out.Reset()
		cmd := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{
			Remote:  remoteDeps,
			Fleet:   fleetDeps,
			Actions: actionsDeps,
		})
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(args)
		return cmd.ExecuteContext(context.Background())
	}
	return &out, run
}

func TestReadCommandsResourceChangesMetricsSendCLISurfaceHeader(t *testing.T) {
	fixedNow := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	var surfaces []string
	var lastQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		surfaces = append(surfaces, r.Header.Get(agentcapabilities.AgentSurfaceHeader))
		if r.Header.Get(agentcapabilities.AgentAPITokenHeader) != "synthetic-cli-token" {
			t.Errorf("missing token header on %s", r.URL.Path)
		}
		lastQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/agent/resource-context/vm:101":
			_, _ = io.WriteString(w, `{
				"canonicalId":"vm:101",
				"resourceType":"vm",
				"resourceName":"app-01",
				"technology":"proxmox",
				"activeFindings":[{"id":"f-1","severity":"warning","title":"High memory"}],
				"pendingApprovals":[],
				"recentActions":[],
				"contextSections":[],
				"generatedAt":"2026-11-23T12:00:00Z"
			}`)
		case agentcapabilities.RecentChangesCapabilityPath:
			_, _ = io.WriteString(w, `{
				"changes":[{"timestamp":"2026-11-23T06:00:00Z","kind":"restart","resourceId":"vm:101","resourceName":"app-01","sourceType":"platform_event"}],
				"count":1,
				"truncated":false,
				"since":"2026-11-23T00:00:00Z",
				"limit":50,
				"generatedAt":"2026-11-23T12:00:00Z"
			}`)
		case agentcapabilities.MetricHistoryCapabilityPath:
			_, _ = io.WriteString(w, `{
				"resourceType":"vm",
				"resourceId":"vm:101",
				"metric":"cpu",
				"range":"6h",
				"start":1700000000000,
				"end":1700021600000,
				"source":"store",
				"points":[{"timestamp":1700000000000,"value":25.5,"min":10.0,"max":42.0}]
			}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Non-TTY auto-JSON mode
	out, run := newTestClientRootWithRemote(server, false, fixedNow)
	if err := run("resource", "vm:101"); err != nil {
		t.Fatalf("resource json: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("resource non-TTY did not emit valid JSON: %v\n%s", err, out.String())
	}

	// TTY human table mode for changes with relative --since 12h
	ttyOut, runTTY := newTestClientRootWithRemote(server, true, fixedNow)
	if err := runTTY("changes", "--since", "12h", "--kind", "restart", "--resource", "vm:101"); err != nil {
		t.Fatalf("changes tty: %v", err)
	}
	if !strings.Contains(lastQuery, "since=2026-11-23T00%3A00%3A00Z") || !strings.Contains(lastQuery, "kind=restart") || !strings.Contains(lastQuery, "resourceId=vm%3A101") {
		t.Fatalf("unexpected changes query: %q", lastQuery)
	}
	if !strings.Contains(ttyOut.String(), "TIME") || !strings.Contains(ttyOut.String(), "restart") {
		t.Fatalf("expected human table for changes on TTY, got:\n%s", ttyOut.String())
	}

	// Explicit --json overrides TTY
	if err := runTTY("--json", "metrics", "vm:101", "--range", "6h", "--metric", "cpu"); err != nil {
		t.Fatalf("metrics --json on tty: %v", err)
	}
	if err := json.Unmarshal(ttyOut.Bytes(), &parsed); err != nil {
		t.Fatalf("metrics --json did not emit JSON: %v\n%s", err, ttyOut.String())
	}

	// TTY table for metrics
	if err := runTTY("metrics", "vm:101", "--range", "6h", "--metric", "cpu"); err != nil {
		t.Fatalf("metrics tty: %v", err)
	}
	if !strings.Contains(ttyOut.String(), "METRIC") || !strings.Contains(ttyOut.String(), "cpu") || !strings.Contains(ttyOut.String(), "25.50") {
		t.Fatalf("expected human table for metrics on TTY, got:\n%s", ttyOut.String())
	}

	for i, s := range surfaces {
		if s != agentcapabilities.AgentSurfacePulseCLI {
			t.Fatalf("request %d surface = %q, want %q", i, s, agentcapabilities.AgentSurfacePulseCLI)
		}
	}
}

func TestParseSinceTimestampSupportsRelativeAndRFC3339(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)

	got, err := parseSinceTimestamp("7d", now)
	if err != nil || got != "2026-11-16T12:00:00Z" {
		t.Fatalf("7d = %q, %v; want 2026-11-16T12:00:00Z", got, err)
	}
	got, err = parseSinceTimestamp("2026-11-20T01:02:03Z", now)
	if err != nil || got != "2026-11-20T01:02:03Z" {
		t.Fatalf("rfc3339 = %q, %v", got, err)
	}
	if _, err := parseSinceTimestamp("bogus", now); err == nil {
		t.Fatal("expected error for invalid --since")
	}
}

func TestEventsCommandStreamsActionableSSE(t *testing.T) {
	var seenSurface string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != agentcapabilities.AgentEventsPath {
			http.NotFound(w, r)
			return
		}
		seenSurface = r.Header.Get(agentcapabilities.AgentSurfaceHeader)
		w.Header().Set("Content-Type", agentcapabilities.AgentSSEAccept)
		_, _ = io.WriteString(w, "event: stream.connected\ndata: {\"ok\":true}\n\n")
		_, _ = io.WriteString(w, "event: heartbeat\ndata: {}\n\n")
		_, _ = io.WriteString(w, "event: finding.created\ndata: {\"findingId\":\"f-1\"}\n\n")
		_, _ = io.WriteString(w, "event: alert.fired\ndata: {\"alertId\":\"a-1\"}\n\n")
	}))
	defer server.Close()

	out, run := newTestClientRootWithRemote(server, false, time.Now().UTC())
	if err := run("events", "--limit", "2"); err != nil {
		t.Fatalf("events: %v", err)
	}
	if seenSurface != agentcapabilities.AgentSurfacePulseCLI {
		t.Fatalf("events surface = %q, want %q", seenSurface, agentcapabilities.AgentSurfacePulseCLI)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 actionable NDJSON events, got %d:\n%s", len(lines), out.String())
	}
	if !strings.Contains(lines[0], `"finding.created"`) || !strings.Contains(lines[1], `"alert.fired"`) {
		t.Fatalf("unexpected events output:\n%s", out.String())
	}
}

func TestFindingsAndOperatorStateCommands(t *testing.T) {
	var seen []string
	var lastPutBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/ai/patrol/findings":
			_, _ = io.WriteString(w, `[{"id":"f-1","severity":"warning","status":"active","resourceId":"vm:101","title":"Disk 90%"}]`)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/ai/patrol/"):
			_, _ = io.WriteString(w, `{"success":true,"message":"updated"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/resources/vm:101/operator-state":
			_, _ = io.WriteString(w, `{"canonicalId":"vm:101","monitoringMode":"normal","intentionallyOffline":false,"neverAutoRemediate":false,"setAt":"2026-11-23T10:00:00Z"}`)
		case r.Method == http.MethodPut && r.URL.Path == "/api/resources/vm:101/operator-state":
			_ = json.NewDecoder(r.Body).Decode(&lastPutBody)
			_, _ = io.WriteString(w, `{"canonicalId":"vm:101","monitoringMode":"expected_offline","intentionallyOffline":true,"neverAutoRemediate":true,"setAt":"2026-11-23T10:00:00Z"}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/resources/vm:101/operator-state":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	out, run := newTestClientRootWithRemote(server, false, time.Now().UTC())
	for _, args := range [][]string{
		{"findings"},
		{"findings", "ack", "f-1"},
		{"findings", "snooze", "f-1", "--hours", "12"},
		{"findings", "dismiss", "f-1", "--reason", "not_an_issue", "--note", "expected"},
		{"findings", "resolve", "f-1", "--note", "cleaned"},
		{"operator-state", "get", "vm:101"},
		{"operator-state", "set", "vm:101", "--monitoring-mode", "expected_offline", "--never-auto-remediate", "--note", "night-shutdown"},
		{"operator-state", "clear", "vm:101"},
	} {
		if err := run(args...); err != nil {
			t.Fatalf("command %v failed: %v\noutput=%s", args, err, out.String())
		}
	}

	if lastPutBody["intentionallyOffline"] != true || lastPutBody["neverAutoRemediate"] != true || lastPutBody["monitoringMode"] != "expected_offline" {
		t.Fatalf("unexpected operator-state PUT body: %#v", lastPutBody)
	}
	if len(seen) != 8 {
		t.Fatalf("expected 8 API calls, got %d: %v", len(seen), seen)
	}
}

func TestCallCommandManifestDrivenAndHandles402WithoutRetry(t *testing.T) {
	manifest := agentcapabilities.CanonicalManifest()
	var planCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case agentcapabilities.AgentCapabilitiesPath:
			_ = json.NewEncoder(w).Encode(manifest)
		case "/api/agent/resource-context/vm:101":
			_, _ = io.WriteString(w, `{"canonicalId":"vm:101","resourceType":"vm","resourceName":"app-01","activeFindings":[],"pendingApprovals":[],"recentActions":[],"contextSections":[],"generatedAt":"2026-11-23T12:00:00Z"}`)
		case agentcapabilities.PlanActionCapabilityPath:
			planCalls.Add(1)
			w.WriteHeader(http.StatusPaymentRequired)
			_, _ = io.WriteString(w, `{"error":"license_required","message":"Governed remediation requires Pulse Pro","feature":"ai_autofix","upgrade_url":"https://pulserelay.pro/upgrade"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	out, run := newTestClientRootWithRemote(server, false, time.Now().UTC())
	if err := run("call", "--list"); err != nil {
		t.Fatalf("call --list: %v", err)
	}
	if !strings.Contains(out.String(), "get_fleet_context") {
		t.Fatalf("call --list missing get_fleet_context:\n%s", out.String())
	}

	if err := run("call", "get_resource_context", "--arg", "resourceId=vm:101"); err != nil {
		t.Fatalf("call get_resource_context: %v", err)
	}
	if !strings.Contains(out.String(), `"canonicalId": "vm:101"`) {
		t.Fatalf("call response unexpected:\n%s", out.String())
	}

	err := run("call", "plan_action", "--body-json", `{"requestId":"r1","resourceId":"vm:101","capabilityName":"restart","reason":"test","requestedBy":"cli"}`)
	if err == nil {
		t.Fatal("expected HTTP 402 error")
	}
	if got := planCalls.Load(); got != 1 {
		t.Fatalf("HTTP 402 retried: calls = %d, want 1", got)
	}
	for _, want := range []string{"Pulse Pro license", "ai_autofix", "Governed remediation requires Pulse Pro", "https://pulserelay.pro/upgrade"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("402 error %q missing %q", err.Error(), want)
		}
	}
}
