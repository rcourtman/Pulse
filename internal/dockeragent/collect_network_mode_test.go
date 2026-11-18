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

// A container that uses another container's network (a download client behind
// a VPN container) has no network attachments and no published ports of its
// own. The runtime's network mode is the one place that names the container
// it borrows from, so the agent reports the mode as the runtime wrote it.
// Only a missing host config leaves the field empty.
func TestCollectContainerReportsNetworkMode(t *testing.T) {
	vpnID := strings.Repeat("ab", 32)
	for _, tc := range []struct {
		name string
		host *containertypes.HostConfig
		want string
	}{
		{name: "another container's network", host: &containertypes.HostConfig{NetworkMode: containertypes.NetworkMode("container:" + vpnID)}, want: "container:" + vpnID},
		{name: "host network", host: &containertypes.HostConfig{NetworkMode: "host"}, want: "host"},
		{name: "no network", host: &containertypes.HostConfig{NetworkMode: "none"}, want: "none"},
		{name: "default bridge", host: &containertypes.HostConfig{NetworkMode: "bridge"}, want: "bridge"},
		{name: "a named network keeps its case", host: &containertypes.HostConfig{NetworkMode: " Media_Net "}, want: "Media_Net"},
		{name: "runtime wrote no mode", host: &containertypes.HostConfig{}, want: ""},
		{name: "absent host config", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inspect := baseInspect()
			// The mode is configuration, so a stopped container states it too,
			// and a stopped one needs no stats read.
			inspect.State = &containertypes.State{Status: containertypes.StateExited, ExitCode: 0}
			inspect.HostConfig = tc.host
			// What the runtime returns for a container in another's namespace.
			inspect.NetworkSettings = nil
			a := &Agent{
				logger:           zerolog.Nop(),
				runtime:          RuntimeDocker,
				prevContainerCPU: make(map[string]cpuSample),
				docker: &fakeDockerClient{
					containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
						return inspect, nil, nil
					},
				},
			}

			got, err := a.collectContainer(context.Background(), containertypes.Summary{ID: "torrent-container", Names: []string{"/qbittorrent"}, Image: "lscr.io/linuxserver/qbittorrent:latest", State: "exited", Status: "Exited (0) 2 minutes ago"})
			if err != nil {
				t.Fatal(err)
			}
			if got.NetworkMode != tc.want {
				t.Fatalf("network mode = %q, want %q", got.NetworkMode, tc.want)
			}

			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var wire agentsdocker.Container
			if err := json.Unmarshal(encoded, &wire); err != nil {
				t.Fatal(err)
			}
			if wire.NetworkMode != tc.want {
				t.Fatalf("report carried network mode %q, want %q", wire.NetworkMode, tc.want)
			}
			if tc.want == "" && strings.Contains(string(encoded), `"networkMode"`) {
				t.Fatalf("an unreported mode must be omitted from the report: %s", encoded)
			}
		})
	}
}
