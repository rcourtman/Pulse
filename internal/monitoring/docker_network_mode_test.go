package monitoring

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// A container that uses another container's network has no attachments and
// no ports of its own, so the mode is the only thing that says where its
// traffic goes. The report's value has to reach the model unchanged, and a
// report without one (an older agent) must leave the field empty.
func TestApplyDockerReportCarriesContainerNetworkMode(t *testing.T) {
	monitor := newTestMonitor(t)
	vpnID := strings.Repeat("ab", 32)
	report := supersedeTestReport("agent-netmode", "6.9.0", time.Now().UTC())
	report.Containers = []agentsdocker.Container{
		{ID: vpnID, Name: "gluetun", State: "running", Status: "Up 2 minutes", NetworkMode: "bridge"},
		{ID: "c-torrent", Name: "qbittorrent", State: "running", Status: "Up 2 minutes", NetworkMode: " container:" + vpnID + " "},
		{ID: "c-host", Name: "node-exporter", State: "running", Status: "Up 2 minutes", NetworkMode: "host"},
		{ID: "c-named", Name: "app", State: "running", Status: "Up 2 minutes", NetworkMode: "Media_Net"},
		{ID: "c-old", Name: "legacy", State: "running", Status: "Up 2 minutes"},
		{ID: "c-long", Name: "long", State: "running", Status: "Up 2 minutes", NetworkMode: strings.Repeat("n", maxDockerNetworkModeLength+1)},
		{ID: "c-ctl", Name: "control", State: "running", Status: "Up 2 minutes", NetworkMode: "bridge\nhost"},
	}

	host, err := monitor.ApplyDockerReport(report, &config.APITokenRecord{ID: "token-netmode", CreatedAt: time.Now().UTC().Add(-time.Hour)})
	if err != nil {
		t.Fatalf("apply report: %v", err)
	}

	want := map[string]string{
		"gluetun":       "bridge",
		"qbittorrent":   "container:" + vpnID,
		"node-exporter": "host",
		"app":           "Media_Net",
		"legacy":        "",
		"long":          "",
		"control":       "",
	}
	if len(host.Containers) != len(want) {
		t.Fatalf("got %d containers, want %d", len(host.Containers), len(want))
	}
	for _, container := range host.Containers {
		mode, ok := want[container.Name]
		if !ok {
			t.Fatalf("unexpected container %q", container.Name)
		}
		if container.NetworkMode != mode {
			t.Fatalf("container %q network mode = %q, want %q", container.Name, container.NetworkMode, mode)
		}
	}
}

func TestDockerNetworkModeFromReportKeepsTheLongestRealValue(t *testing.T) {
	longest := "container:" + strings.Repeat("f", 64)
	if got := dockerNetworkModeFromReport(longest); got != longest {
		t.Fatalf("a full container reference was changed: %q", got)
	}
	atLimit := strings.Repeat("n", maxDockerNetworkModeLength)
	if got := dockerNetworkModeFromReport(atLimit); got != atLimit {
		t.Fatalf("a value at the limit was dropped")
	}
}
