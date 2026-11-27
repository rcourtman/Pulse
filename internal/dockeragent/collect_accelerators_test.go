package dockeragent

import (
	"reflect"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func TestContainerAccelerators(t *testing.T) {
	tests := []struct {
		name       string
		hostConfig *containertypes.HostConfig
		mounts     []agentsdocker.ContainerMount
		want       []string
	}{
		{
			name: "nil hostConfig and no mounts",
			want: nil,
		},
		{
			name: "nvidia device request",
			hostConfig: &containertypes.HostConfig{
				Resources: containertypes.Resources{
					DeviceRequests: []containertypes.DeviceRequest{
						{Driver: "nvidia", Capabilities: [][]string{{"gpu"}}},
					},
				},
			},
			want: []string{"GPU"},
		},
		{
			name: "nvidia runtime fallback",
			hostConfig: &containertypes.HostConfig{
				Runtime: "nvidia",
			},
			want: []string{"GPU"},
		},
		{
			name: "dri igpu and coral tpu devices",
			hostConfig: &containertypes.HostConfig{
				Resources: containertypes.Resources{
					Devices: []containertypes.DeviceMapping{
						{PathOnHost: "/dev/dri/renderD128", PathInContainer: "/dev/dri/renderD128"},
						{PathOnHost: "/dev/apex_0", PathInContainer: "/dev/apex_0"},
					},
				},
			},
			want: []string{"TPU", "iGPU"},
		},
		{
			name: "amd rocm kfd and hailo npu",
			hostConfig: &containertypes.HostConfig{
				Resources: containertypes.Resources{
					Devices: []containertypes.DeviceMapping{
						{PathOnHost: "/dev/kfd", PathInContainer: "/dev/kfd"},
						{PathOnHost: "/dev/hailo0", PathInContainer: "/dev/hailo0"},
					},
				},
			},
			want: []string{"AMD GPU", "NPU"},
		},
		{
			name: "bind-mounted dri device",
			mounts: []agentsdocker.ContainerMount{
				{Type: "bind", Source: "/dev/dri", Destination: "/dev/dri"},
				{Type: "bind", Source: "/mnt/user/media", Destination: "/media"},
			},
			want: []string{"iGPU"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := containerAccelerators(tc.hostConfig, tc.mounts)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("containerAccelerators() = %v, want %v", got, tc.want)
			}
		})
	}
}
