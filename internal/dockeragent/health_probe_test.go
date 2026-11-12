package dockeragent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

// The reason is named only when the probe output states it, in the words the
// common probe tools and the engine print. Output that states nothing known
// is "other": the exit code is still reported and nobody is sent the wrong way.
func TestClassifyHealthProbeNamesOnlyWhatTheOutputStates(t *testing.T) {
	for _, tc := range []struct {
		name       string
		exitCode   int
		output     string
		wantReason string
		wantStatus int
	}{
		{"busybox wget, nothing listening", 1, "wget: can't connect to remote host (127.0.0.1): Connection refused\n", agentsdocker.HealthProbeReasonConnectionRefused, 0},
		{"curl, nothing listening", 7, "curl: (7) Failed to connect to localhost port 8080 after 0 ms: Connection refused", agentsdocker.HealthProbeReasonConnectionRefused, 0},
		{"redis-cli, nothing listening", 1, "Could not connect to Redis at 127.0.0.1:6379: Connection refused", agentsdocker.HealthProbeReasonConnectionRefused, 0},
		{"curl, no route is not a refusal", 7, "curl: (7) Failed to connect to 10.0.0.9 port 80 after 3069 ms: No route to host", agentsdocker.HealthProbeReasonOther, 0},
		{"curl -f, server error", 22, "curl: (22) The requested URL returned error: 503", agentsdocker.HealthProbeReasonHTTPStatus, 503},
		{"busybox wget, server error", 1, "wget: server returned error: HTTP/1.1 500 Internal Server Error", agentsdocker.HealthProbeReasonHTTPStatus, 500},
		{"GNU wget, not found", 8, "2026-11-12 10:00:00 ERROR 404: Not Found.", agentsdocker.HealthProbeReasonHTTPStatus, 404},
		{"engine timeout", -1, "Health check exceeded timeout (5s)", agentsdocker.HealthProbeReasonTimeout, 0},
		{"curl timeout", 28, "curl: (28) Operation timed out after 5001 milliseconds with 0 bytes received", agentsdocker.HealthProbeReasonTimeout, 0},
		{"curl, name", 6, "curl: (6) Could not resolve host: db", agentsdocker.HealthProbeReasonNameNotResolved, 0},
		{"busybox wget, name", 1, "wget: bad address 'db:5432'", agentsdocker.HealthProbeReasonNameNotResolved, 0},
		{"curl, certificate", 60, "curl: (60) SSL certificate problem: self-signed certificate", agentsdocker.HealthProbeReasonTLS, 0},
		{"shell cannot find the probe", 127, "/bin/sh: curl: not found", agentsdocker.HealthProbeReasonCommandMissing, 0},
		{"runtime cannot find the probe", -1, `OCI runtime exec failed: exec failed: unable to start container process: exec: "curl": executable file not found in $PATH: unknown`, agentsdocker.HealthProbeReasonCommandMissing, 0},
		{"pg_isready does not say why", 2, "localhost:5432 - no response", agentsdocker.HealthProbeReasonOther, 0},
		{"custom script", 1, "queue depth 4012 over limit", agentsdocker.HealthProbeReasonOther, 0},
		{"no output", 1, "", agentsdocker.HealthProbeReasonOther, 0},
		{"a redirect is not an error status", 22, "HTTP/1.1 301 Moved Permanently", agentsdocker.HealthProbeReasonOther, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reason, status := classifyHealthProbe(tc.exitCode, tc.output)
			if reason != tc.wantReason || status != tc.wantStatus {
				t.Fatalf("classifyHealthProbe(%d, %q) = %q, %d, want %q, %d", tc.exitCode, tc.output, reason, status, tc.wantReason, tc.wantStatus)
			}
			if agentsdocker.NormalizeHealthProbeReason(reason) != reason {
				t.Fatalf("reason %q is outside the shared vocabulary", reason)
			}
		})
	}
}

func TestContainerHealthProbeReportsOnlyAFailingUnhealthyContainer(t *testing.T) {
	refused := &containertypes.HealthcheckResult{ExitCode: 1, Output: "wget: can't connect to remote host (127.0.0.1): Connection refused"}
	passed := &containertypes.HealthcheckResult{ExitCode: 0, Output: "ok"}
	serverError := &containertypes.HealthcheckResult{ExitCode: 22, Output: "curl: (22) The requested URL returned error: 502"}

	for name, health := range map[string]*containertypes.Health{
		"no health check":             nil,
		"healthy":                     {Status: containertypes.Healthy, Log: []*containertypes.HealthcheckResult{passed}},
		"starting, first probes fail": {Status: containertypes.Starting, FailingStreak: 1, Log: []*containertypes.HealthcheckResult{refused}},
		"unhealthy, no result kept":   {Status: containertypes.Unhealthy, FailingStreak: 3},
		"unhealthy, last result nil":  {Status: containertypes.Unhealthy, FailingStreak: 3, Log: []*containertypes.HealthcheckResult{nil}},
		"unhealthy, last one passed":  {Status: containertypes.Unhealthy, Log: []*containertypes.HealthcheckResult{refused, passed}},
	} {
		if probe := containerHealthProbe(health); probe != nil {
			t.Errorf("%s: reported %+v, want nothing", name, probe)
		}
	}

	probe := containerHealthProbe(&containertypes.Health{Status: containertypes.Unhealthy, FailingStreak: 14,
		Log: []*containertypes.HealthcheckResult{refused, serverError}})
	want := agentsdocker.ContainerHealthProbe{Reason: agentsdocker.HealthProbeReasonHTTPStatus, ExitCode: 22, FailingStreak: 14, HTTPStatus: 502}
	if probe == nil || *probe != want {
		t.Fatalf("probe = %+v, want the newest result %+v", probe, want)
	}
}

// Probe output can carry whatever the probe command touched. The report holds
// a reason code and numbers, and none of the output.
func TestCollectContainerSendsTheProbeReasonAndNoneOfItsOutput(t *testing.T) {
	inspect := baseInspect()
	inspect.State = &containertypes.State{Status: containertypes.StateRunning, Running: true, Health: &containertypes.Health{
		Status: containertypes.Unhealthy, FailingStreak: 6,
		Log: []*containertypes.HealthcheckResult{{ExitCode: 22,
			Output: "curl: (22) The requested URL returned error: 401\nhttps://probe:hunter2@internal.example/health?token=tok-abc123 Authorization: Bearer eyJhbGciOi"}},
	}}
	a := &Agent{
		logger:           zerolog.Nop(),
		runtime:          RuntimeDocker,
		prevContainerCPU: make(map[string]cpuSample),
		docker: &fakeDockerClient{
			containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
				return inspect, nil, nil
			},
			containerStatsOneShotFn: func(context.Context, string) (dockerStatsResponseReader, error) {
				return statsReader(t, containertypes.StatsResponse{}), nil
			},
		},
	}

	got, err := a.collectContainer(context.Background(), containertypes.Summary{ID: "web-container", Names: []string{"/paperless"}, Image: "paperless:2", State: "running", Status: "Up 2 hours (unhealthy)"})
	if err != nil {
		t.Fatal(err)
	}
	want := agentsdocker.ContainerHealthProbe{Reason: agentsdocker.HealthProbeReasonHTTPStatus, ExitCode: 22, FailingStreak: 6, HTTPStatus: 401}
	if got.HealthProbe == nil || *got.HealthProbe != want {
		t.Fatalf("health probe = %+v, want %+v", got.HealthProbe, want)
	}

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"hunter2", "tok-abc123", "eyJhbGciOi", "internal.example", "requested URL"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("the report carries probe output %q: %s", secret, encoded)
		}
	}
	var wire agentsdocker.Container
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.HealthProbe == nil || *wire.HealthProbe != want {
		t.Fatalf("report carried %+v, want %+v", wire.HealthProbe, want)
	}
}
