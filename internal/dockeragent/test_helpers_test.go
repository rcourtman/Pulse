package dockeragent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	systemtypes "github.com/docker/docker/api/types/system"
	"github.com/opencontainers/image-spec/specs-go/v1"
)

type fakeDockerClient struct {
	daemonHost                string
	infoFunc                  func(ctx context.Context) (systemtypes.Info, error)
	containerListFunc         func(ctx context.Context, opts containertypes.ListOptions) ([]containertypes.Summary, error)
	containerInspectWithRawFn func(ctx context.Context, id string, size bool) (containertypes.InspectResponse, []byte, error)
	containerStatsOneShotFn   func(ctx context.Context, id string) (containertypes.StatsResponseReader, error)
	containerInspectFn        func(ctx context.Context, id string) (containertypes.InspectResponse, error)
	imagePullFn               func(ctx context.Context, ref string, opts image.PullOptions) (io.ReadCloser, error)
	containerStopFn           func(ctx context.Context, id string, opts containertypes.StopOptions) error
	containerRenameFn         func(ctx context.Context, id, newName string) error
	containerCreateFn         func(ctx context.Context, config *containertypes.Config, hostConfig *containertypes.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (containertypes.CreateResponse, error)
	networkConnectFn          func(ctx context.Context, netName, containerID string, endpoint *network.EndpointSettings) error
	containerStartFn          func(ctx context.Context, id string, opts containertypes.StartOptions) error
	containerRemoveFn         func(ctx context.Context, id string, opts containertypes.RemoveOptions) error
	serviceListFn             func(ctx context.Context, opts swarmtypes.ServiceListOptions) ([]swarmtypes.Service, error)
	taskListFn                func(ctx context.Context, opts swarmtypes.TaskListOptions) ([]swarmtypes.Task, error)
	closeFn                   func() error
}

func (f *fakeDockerClient) Info(ctx context.Context) (systemtypes.Info, error) {
	if f.infoFunc == nil {
		return systemtypes.Info{}, errors.New("unexpected Info call")
	}
	return f.infoFunc(ctx)
}

func (f *fakeDockerClient) DaemonHost() string {
	return f.daemonHost
}

func (f *fakeDockerClient) ContainerList(ctx context.Context, opts containertypes.ListOptions) ([]containertypes.Summary, error) {
	if f.containerListFunc == nil {
		return nil, errors.New("unexpected ContainerList call")
	}
	return f.containerListFunc(ctx, opts)
}

func (f *fakeDockerClient) ContainerInspectWithRaw(ctx context.Context, id string, size bool) (containertypes.InspectResponse, []byte, error) {
	if f.containerInspectWithRawFn == nil {
		return containertypes.InspectResponse{}, nil, errors.New("unexpected ContainerInspectWithRaw call")
	}
	return f.containerInspectWithRawFn(ctx, id, size)
}

func (f *fakeDockerClient) ContainerStatsOneShot(ctx context.Context, id string) (containertypes.StatsResponseReader, error) {
	if f.containerStatsOneShotFn == nil {
		return containertypes.StatsResponseReader{}, errors.New("unexpected ContainerStatsOneShot call")
	}
	return f.containerStatsOneShotFn(ctx, id)
}

func (f *fakeDockerClient) ContainerInspect(ctx context.Context, id string) (containertypes.InspectResponse, error) {
	if f.containerInspectFn == nil {
		return containertypes.InspectResponse{}, errors.New("unexpected ContainerInspect call")
	}
	return f.containerInspectFn(ctx, id)
}

func (f *fakeDockerClient) ImagePull(ctx context.Context, ref string, opts image.PullOptions) (io.ReadCloser, error) {
	if f.imagePullFn == nil {
		return nil, errors.New("unexpected ImagePull call")
	}
	return f.imagePullFn(ctx, ref, opts)
}

func (f *fakeDockerClient) ContainerStop(ctx context.Context, id string, opts containertypes.StopOptions) error {
	if f.containerStopFn == nil {
		return errors.New("unexpected ContainerStop call")
	}
	return f.containerStopFn(ctx, id, opts)
}

func (f *fakeDockerClient) ContainerRename(ctx context.Context, id, newName string) error {
	if f.containerRenameFn == nil {
		return errors.New("unexpected ContainerRename call")
	}
	return f.containerRenameFn(ctx, id, newName)
}

func (f *fakeDockerClient) ContainerCreate(ctx context.Context, config *containertypes.Config, hostConfig *containertypes.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (containertypes.CreateResponse, error) {
	if f.containerCreateFn == nil {
		return containertypes.CreateResponse{}, errors.New("unexpected ContainerCreate call")
	}
	return f.containerCreateFn(ctx, config, hostConfig, networkingConfig, platform, containerName)
}

func (f *fakeDockerClient) NetworkConnect(ctx context.Context, netName, containerID string, endpoint *network.EndpointSettings) error {
	if f.networkConnectFn == nil {
		return errors.New("unexpected NetworkConnect call")
	}
	return f.networkConnectFn(ctx, netName, containerID, endpoint)
}

func (f *fakeDockerClient) ContainerStart(ctx context.Context, id string, opts containertypes.StartOptions) error {
	if f.containerStartFn == nil {
		return errors.New("unexpected ContainerStart call")
	}
	return f.containerStartFn(ctx, id, opts)
}

func (f *fakeDockerClient) ContainerRemove(ctx context.Context, id string, opts containertypes.RemoveOptions) error {
	if f.containerRemoveFn == nil {
		return errors.New("unexpected ContainerRemove call")
	}
	return f.containerRemoveFn(ctx, id, opts)
}

func (f *fakeDockerClient) ServiceList(ctx context.Context, opts swarmtypes.ServiceListOptions) ([]swarmtypes.Service, error) {
	if f.serviceListFn == nil {
		return nil, errors.New("unexpected ServiceList call")
	}
	return f.serviceListFn(ctx, opts)
}

func (f *fakeDockerClient) TaskList(ctx context.Context, opts swarmtypes.TaskListOptions) ([]swarmtypes.Task, error) {
	if f.taskListFn == nil {
		return nil, errors.New("unexpected TaskList call")
	}
	return f.taskListFn(ctx, opts)
}

func (f *fakeDockerClient) Close() error {
	if f.closeFn == nil {
		return nil
	}
	return f.closeFn()
}

func statsReader(t *testing.T, stats containertypes.StatsResponse) containertypes.StatsResponseReader {
	t.Helper()

	payload, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("marshal stats: %v", err)
	}

	return containertypes.StatsResponseReader{
		Body: io.NopCloser(bytes.NewReader(payload)),
	}
}

func swap[T any](t *testing.T, target *T, value T) {
	t.Helper()
	prev := *target
	*target = value
	t.Cleanup(func() {
		*target = prev
	})
}
