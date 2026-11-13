package monitoring

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// The server does not trust an agent to have shaped a failure log. Whatever
// arrives is masked and bounded again, an unknown reason is refused, and a
// capture time from the future is brought back to the server's clock.
func TestApplyDockerReportShapesTheFailureLogItself(t *testing.T) {
	monitor := newTestMonitor(t)
	now := time.Now().UTC()
	report := supersedeTestReport("agent-faillog", "6.9.0", now)
	var flood []string
	for i := 0; i < agentsdocker.FailureLogMaxLines*3; i++ {
		flood = append(flood, "line of noise")
	}
	report.Containers = []agentsdocker.Container{
		{ID: "c-honest", Name: "vaultwarden", State: "exited", ExitCode: 1, FailureLog: &agentsdocker.ContainerFailureLog{
			Reason: agentsdocker.FailureLogReasonExited, CapturedAt: now.Add(-time.Minute),
			Lines: []string{"[vaultwarden::db][ERROR] invalid DATABASE_URL", "exiting"}}},
		{ID: "c-unmasked", Name: "leaky", State: "exited", ExitCode: 1, FailureLog: &agentsdocker.ContainerFailureLog{
			Reason: agentsdocker.FailureLogReasonExited, CapturedAt: now.Add(48 * time.Hour),
			Lines: []string{"connect postgres://vault:s3cr3t@db:5432/vault failed", "API_TO\x00KEN=hunter2\x07", strings.Repeat("y", agentsdocker.FailureLogMaxLineBytes*4)}}},
		{ID: "c-flood", Name: "noisy", State: "restarting", ExitCode: 1, FailureLog: &agentsdocker.ContainerFailureLog{
			Reason: agentsdocker.FailureLogReasonRestarting, CapturedAt: now, Lines: flood}},
		{ID: "c-reason", Name: "odd", State: "exited", ExitCode: 1, FailureLog: &agentsdocker.ContainerFailureLog{
			Reason: "anything else", CapturedAt: now, Lines: []string{"text"}}},
		{ID: "c-empty", Name: "blank", State: "exited", ExitCode: 1, FailureLog: &agentsdocker.ContainerFailureLog{
			Reason: agentsdocker.FailureLogReasonExited, CapturedAt: now, Lines: []string{"  ", ""}}},
		{ID: "c-none", Name: "quiet", State: "exited", ExitCode: 1},
	}

	host, err := monitor.ApplyDockerReport(report, &config.APITokenRecord{ID: "token-faillog", CreatedAt: now.Add(-time.Hour)})
	if err != nil {
		t.Fatalf("apply report: %v", err)
	}
	got := map[string]int{}
	for _, container := range host.Containers {
		log := container.FailureLog
		if log == nil {
			got[container.Name] = -1
			continue
		}
		got[container.Name] = len(log.Lines)
		text := strings.Join(log.Lines, "\n")
		switch container.Name {
		case "vaultwarden":
			if text != "[vaultwarden::db][ERROR] invalid DATABASE_URL\nexiting" || log.Truncated || log.Redactions != 0 {
				t.Fatalf("an honest log was changed: %+v", log)
			}
		case "leaky":
			for _, secret := range []string{"s3cr3t", "hunter2", "\x07"} {
				if strings.Contains(text, secret) {
					t.Fatalf("%q was stored: %q", secret, text)
				}
			}
			if log.Redactions < 2 || !log.Truncated || len(log.Lines[2]) > agentsdocker.FailureLogMaxLineBytes {
				t.Fatalf("a log the agent did not shape was stored as sent: %+v", log)
			}
			if log.CapturedAt.After(time.Now().Add(time.Minute)) {
				t.Fatalf("capture time %s is in the future", log.CapturedAt)
			}
		case "noisy":
			if len(log.Lines) != agentsdocker.FailureLogMaxLines || !log.Truncated {
				t.Fatalf("a flood kept %d lines, truncated=%t", len(log.Lines), log.Truncated)
			}
		}
	}
	want := map[string]int{"vaultwarden": 2, "leaky": 3, "noisy": agentsdocker.FailureLogMaxLines, "odd": -1, "blank": -1, "quiet": -1}
	for name, lines := range want {
		if got[name] != lines {
			t.Errorf("%s: %d lines stored, want %d (-1 is none)", name, got[name], lines)
		}
	}
}
