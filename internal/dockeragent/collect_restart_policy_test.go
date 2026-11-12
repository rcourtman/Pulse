package dockeragent

import (
	"context"
	"encoding/json"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

// The server tells a finished job from a stopped service by the restart
// policy, so the agent reports it for every container. Docker's empty name
// is the same as "no" and is reported as "no"; only a missing host config
// leaves the field empty.
func TestCollectContainerReportsRestartPolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		host *containertypes.HostConfig
		want string
	}{
		{name: "no policy set", host: &containertypes.HostConfig{}, want: "no"},
		{name: "explicit no", host: &containertypes.HostConfig{RestartPolicy: containertypes.RestartPolicy{Name: containertypes.RestartPolicyDisabled}}, want: "no"},
		{name: "on-failure", host: &containertypes.HostConfig{RestartPolicy: containertypes.RestartPolicy{Name: containertypes.RestartPolicyOnFailure, MaximumRetryCount: 3}}, want: "on-failure"},
		{name: "always", host: &containertypes.HostConfig{RestartPolicy: containertypes.RestartPolicy{Name: containertypes.RestartPolicyAlways}}, want: "always"},
		{name: "unless-stopped with odd casing", host: &containertypes.HostConfig{RestartPolicy: containertypes.RestartPolicy{Name: " Unless-Stopped "}}, want: "unless-stopped"},
		{name: "absent host config", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inspect := baseInspect()
			inspect.State = &containertypes.State{Status: containertypes.StateExited, ExitCode: 0}
			inspect.HostConfig = tc.host
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

			got, err := a.collectContainer(context.Background(), containertypes.Summary{ID: "job-container", Names: []string{"/db-migrate"}, Image: "alpine:3.20", State: "exited", Status: "Exited (0) 2 minutes ago"})
			if err != nil {
				t.Fatal(err)
			}
			if got.RestartPolicy != tc.want {
				t.Fatalf("restart policy = %q, want %q", got.RestartPolicy, tc.want)
			}

			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var wire agentsdocker.Container
			if err := json.Unmarshal(encoded, &wire); err != nil {
				t.Fatal(err)
			}
			if wire.RestartPolicy != tc.want {
				t.Fatalf("report carried restart policy %q, want %q", wire.RestartPolicy, tc.want)
			}
		})
	}
}
