package monitoring

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func TestApplyHostReportRejectsOversizedIdentity(t *testing.T) {
	oversized := strings.Repeat("x", maxReportedIdentityBytes+1)

	cases := []struct {
		name   string
		mutate func(*agentshost.Report)
		want   string
	}{
		{"display name", func(r *agentshost.Report) { r.Host.DisplayName = oversized }, "host report host.displayName exceeds 256 bytes"},
		{"hostname", func(r *agentshost.Report) { r.Host.Hostname = oversized }, "host report host.hostname exceeds 256 bytes"},
		{"architecture", func(r *agentshost.Report) { r.Host.Architecture = oversized }, "host report host.architecture exceeds 256 bytes"},
		{"os name", func(r *agentshost.Report) { r.Host.OSName = oversized }, "host report host.osName exceeds 256 bytes"},
		{"agent version", func(r *agentshost.Report) { r.Agent.Version = oversized }, "host report agent.version exceeds 256 bytes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			monitor := &Monitor{
				state:             models.NewState(),
				alertManager:      alerts.NewManager(),
				hostTokenBindings: make(map[string]string),
				config:            &config.Config{},
			}
			t.Cleanup(func() { monitor.alertManager.Stop() })

			report := agentshost.Report{
				Host:      agentshost.HostInfo{Hostname: "lab", ID: "machine-id"},
				Agent:     agentshost.AgentInfo{ID: "agent-id", Version: "1.0.0"},
				Timestamp: time.Now(),
			}
			tc.mutate(&report)

			_, err := monitor.ApplyHostReport(report, nil)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("ApplyHostReport error = %v, want %q", err, tc.want)
			}
			if hosts := monitor.state.GetHosts(); len(hosts) != 0 {
				t.Fatalf("rejected report stored %d host(s)", len(hosts))
			}
		})
	}
}

func TestApplyHostReportTruncatesTagsCustomSensorsAndDisks(t *testing.T) {
	monitor := newTestMonitor(t)
	tooManyTags := make([]string, maxReportedHostTags+5)
	for i := range tooManyTags {
		tooManyTags[i] = strings.Repeat("t", maxReportedHostTagBytes+10)
	}
	tooManySensors := make([]agentshost.CustomSensorMetric, maxHostCustomSensors+10)
	for i := range tooManySensors {
		tooManySensors[i] = agentshost.CustomSensorMetric{
			ID:    strings.Repeat("s", maxReportedIdentityBytes+20),
			Name:  strings.Repeat("n", maxReportedIdentityBytes+20),
			Error: strings.Repeat("e", maxReportedIdentityBytes+20),
		}
	}
	tooManySMART := make([]agentshost.DiskSMART, maxReportedPhysicalDisks+5)
	for i := range tooManySMART {
		tooManySMART[i] = agentshost.DiskSMART{Device: "/dev/sd"}
	}

	host, err := monitor.ApplyHostReport(agentshost.Report{
		Host:      agentshost.HostInfo{Hostname: "lab", ID: "machine-trunc"},
		Agent:     agentshost.AgentInfo{ID: "agent-trunc", Version: "1.0.0"},
		Tags:      tooManyTags,
		Sensors:   agentshost.Sensors{Custom: tooManySensors, SMART: tooManySMART},
		Timestamp: time.Now(),
	}, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport unexpectedly rejected oversized tags/sensors: %v", err)
	}
	if len(host.Tags) != maxReportedHostTags {
		t.Fatalf("host tags = %d, want %d", len(host.Tags), maxReportedHostTags)
	}
	if len(host.Tags[0]) != maxReportedHostTagBytes {
		t.Fatalf("tag length = %d, want %d", len(host.Tags[0]), maxReportedHostTagBytes)
	}
	if len(host.Sensors.Custom) != maxHostCustomSensors {
		t.Fatalf("custom sensors = %d, want %d", len(host.Sensors.Custom), maxHostCustomSensors)
	}
	if len(host.Sensors.Custom[0].Name) != maxReportedIdentityBytes {
		t.Fatalf("custom sensor name length = %d, want %d", len(host.Sensors.Custom[0].Name), maxReportedIdentityBytes)
	}
	if len(host.Sensors.SMART) != maxReportedPhysicalDisks {
		t.Fatalf("SMART disks = %d, want %d", len(host.Sensors.SMART), maxReportedPhysicalDisks)
	}
}

func TestValidateHostReportIdentityAcceptsBoundaryValues(t *testing.T) {
	name := strings.Repeat("x", maxReportedIdentityBytes)
	tags := make([]string, maxReportedHostTags)
	for i := range tags {
		tags[i] = strings.Repeat("t", maxReportedHostTagBytes)
	}
	report := agentshost.Report{
		Host:  agentshost.HostInfo{Hostname: name, DisplayName: name, OSName: name},
		Agent: agentshost.AgentInfo{ID: name, Version: name},
		Tags:  tags,
	}
	if err := validateHostReportIdentity(report); err != nil {
		t.Fatalf("validateHostReportIdentity rejected boundary values: %v", err)
	}
}

func TestApplyDockerReportRejectsOversizedIdentity(t *testing.T) {
	oversized := strings.Repeat("x", maxReportedIdentityBytes+1)

	cases := []struct {
		name   string
		mutate func(*agentsdocker.Report)
		want   string
	}{
		{"name", func(r *agentsdocker.Report) { r.Host.Name = oversized }, "docker report host.name exceeds 256 bytes"},
		{"hostname", func(r *agentsdocker.Report) { r.Host.Hostname = oversized }, "docker report host.hostname exceeds 256 bytes"},
		{"agent id", func(r *agentsdocker.Report) { r.Agent.ID = oversized }, "docker report agent.id exceeds 256 bytes"},
		{"machine id", func(r *agentsdocker.Report) { r.Host.MachineID = oversized }, "docker report host.machineId exceeds 256 bytes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			monitor := newTestMonitor(t)

			report := agentsdocker.Report{
				Agent:     agentsdocker.AgentInfo{ID: "agent-id", Version: "1.0.0"},
				Host:      agentsdocker.HostInfo{Hostname: "lab"},
				Timestamp: time.Now(),
			}
			tc.mutate(&report)

			_, err := monitor.ApplyDockerReport(report, nil)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("ApplyDockerReport error = %v, want %q", err, tc.want)
			}
			if hosts := monitor.state.GetDockerHosts(); len(hosts) != 0 {
				t.Fatalf("rejected report stored %d docker host(s)", len(hosts))
			}
		})
	}
}
