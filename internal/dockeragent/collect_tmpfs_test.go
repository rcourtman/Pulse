package dockeragent

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

func TestCollectContainerPreservesTmpfsMounts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		host   *containertypes.HostConfig
		mounts []containertypes.MountPoint
		want   []agentsdocker.ContainerMount
	}{
		{
			name: "native tmpfs only",
			host: &containertypes.HostConfig{Tmpfs: map[string]string{
				"/var/lib/service-cache": "rw,noexec,nosuid,nodev,size=8388608",
			}},
			want: []agentsdocker.ContainerMount{{Type: "tmpfs", Destination: "/var/lib/service-cache", Mode: "rw,noexec,nosuid,nodev,size=8388608", RW: true}},
		},
		{
			name:   "mixed mounts and stable tmpfs order",
			host:   &containertypes.HostConfig{Tmpfs: map[string]string{"/z-cache": "", "/a-cache": "ro,noexec"}},
			mounts: []containertypes.MountPoint{{Type: "bind", Source: "/host-data", Destination: "/data", RW: true}},
			want: []agentsdocker.ContainerMount{
				{Type: "bind", Source: "/host-data", Destination: "/data", RW: true},
				{Type: "tmpfs", Destination: "/a-cache", Mode: "ro,noexec", RW: false},
				{Type: "tmpfs", Destination: "/z-cache", RW: true},
			},
		},
		{
			name:   "reported mount is authoritative",
			host:   &containertypes.HostConfig{Tmpfs: map[string]string{"/cache": "rw,size=8388608"}},
			mounts: []containertypes.MountPoint{{Type: "tmpfs", Destination: "/cache", Mode: "ro", RW: false}},
			want:   []agentsdocker.ContainerMount{{Type: "tmpfs", Destination: "/cache", Mode: "ro", RW: false}},
		},
		{name: "absent host config"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inspect := baseInspect()
			inspect.HostConfig = tc.host
			inspect.Mounts = tc.mounts
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
			got, err := a.collectContainer(context.Background(), containertypes.Summary{ID: "owned-storage-container", Names: []string{"/worker"}, Image: "alpine:3.20", State: "running"})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Mounts, tc.want) {
				t.Fatalf("mount inventory = %#v, want %#v", got.Mounts, tc.want)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var wire agentsdocker.Container
			if err := json.Unmarshal(encoded, &wire); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(wire.Mounts, tc.want) {
				t.Fatalf("report lost mounts: %#v", wire.Mounts)
			}
			t.Logf("TMPFS_COLLECTOR_REPORT %s", encoded)
		})
	}
}
