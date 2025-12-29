package dockeragent

import (
	"context"
	"io"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	systemtypes "github.com/docker/docker/api/types/system"
	"github.com/opencontainers/image-spec/specs-go/v1"
)

type dockerClient interface {
	Info(ctx context.Context) (systemtypes.Info, error)
	DaemonHost() string
	ContainerList(ctx context.Context, options containertypes.ListOptions) ([]containertypes.Summary, error)
	ContainerInspectWithRaw(ctx context.Context, containerID string, size bool) (containertypes.InspectResponse, []byte, error)
	ContainerStatsOneShot(ctx context.Context, containerID string) (containertypes.StatsResponseReader, error)
	ContainerInspect(ctx context.Context, containerID string) (containertypes.InspectResponse, error)
	ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	ContainerStop(ctx context.Context, containerID string, options containertypes.StopOptions) error
	ContainerRename(ctx context.Context, containerID, newName string) error
	ContainerCreate(ctx context.Context, config *containertypes.Config, hostConfig *containertypes.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (containertypes.CreateResponse, error)
	NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error
	ContainerStart(ctx context.Context, containerID string, options containertypes.StartOptions) error
	ContainerRemove(ctx context.Context, containerID string, options containertypes.RemoveOptions) error
	ServiceList(ctx context.Context, options swarmtypes.ServiceListOptions) ([]swarmtypes.Service, error)
	TaskList(ctx context.Context, options swarmtypes.TaskListOptions) ([]swarmtypes.Task, error)
	Close() error
}
