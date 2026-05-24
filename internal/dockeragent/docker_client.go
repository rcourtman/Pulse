package dockeragent

import (
	"context"
	"io"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	swarmtypes "github.com/moby/moby/api/types/swarm"
	systemtypes "github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type dockerFilters map[string]map[string]bool

func newDockerFilters() dockerFilters {
	return make(dockerFilters)
}

func (f dockerFilters) Add(key, value string) {
	if _, ok := f[key]; ok {
		f[key][value] = true
		return
	}
	f[key] = map[string]bool{value: true}
}

func (f dockerFilters) Get(key string) []string {
	values := f[key]
	if len(values) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	return result
}

func (f dockerFilters) Len() int {
	return len(f)
}

func (f dockerFilters) toClientFilters() client.Filters {
	if len(f) == 0 {
		return nil
	}

	out := make(client.Filters, len(f))
	for key, values := range f {
		copied := make(map[string]bool, len(values))
		for value, enabled := range values {
			copied[value] = enabled
		}
		out[key] = copied
	}
	return out
}

type dockerContainerListOptions struct {
	All     bool
	Filters dockerFilters
}

type dockerStatsResponseReader struct {
	Body io.ReadCloser
}

type dockerImagePullOptions struct{}

type dockerContainerStopOptions struct {
	Signal  string
	Timeout *int
}

type dockerContainerStartOptions struct {
	CheckpointID  string
	CheckpointDir string
}

type dockerContainerRemoveOptions struct {
	RemoveVolumes bool
	RemoveLinks   bool
	Force         bool
}

type dockerServiceListOptions struct {
	Status bool
}

type dockerImageListOptions struct {
	All        bool
	SharedSize bool
	Filters    dockerFilters
}

type dockerVolumeListOptions struct {
	Filters dockerFilters
}

type dockerNetworkListOptions struct {
	Filters dockerFilters
}

type dockerDiskUsageOptions struct {
	Containers bool
	Images     bool
	BuildCache bool
	Volumes    bool
	Verbose    bool
}

type dockerTaskListOptions struct {
	Filters dockerFilters
}

type dockerClient interface {
	Info(ctx context.Context) (systemtypes.Info, error)
	DaemonHost() string
	ContainerList(ctx context.Context, options dockerContainerListOptions) ([]containertypes.Summary, error)
	ContainerInspectWithRaw(ctx context.Context, containerID string, size bool) (containertypes.InspectResponse, []byte, error)
	ContainerStatsOneShot(ctx context.Context, containerID string) (dockerStatsResponseReader, error)
	ContainerInspect(ctx context.Context, containerID string) (containertypes.InspectResponse, error)
	ImagePull(ctx context.Context, ref string, options dockerImagePullOptions) (io.ReadCloser, error)
	ContainerStop(ctx context.Context, containerID string, options dockerContainerStopOptions) error
	ContainerRename(ctx context.Context, containerID, newName string) error
	ContainerCreate(ctx context.Context, config *containertypes.Config, hostConfig *containertypes.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (containertypes.CreateResponse, error)
	NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error
	ContainerStart(ctx context.Context, containerID string, options dockerContainerStartOptions) error
	ContainerRemove(ctx context.Context, containerID string, options dockerContainerRemoveOptions) error
	ImageList(ctx context.Context, options dockerImageListOptions) ([]image.Summary, error)
	VolumeList(ctx context.Context, options dockerVolumeListOptions) ([]volume.Volume, error)
	NetworkList(ctx context.Context, options dockerNetworkListOptions) ([]network.Summary, error)
	DiskUsage(ctx context.Context, options dockerDiskUsageOptions) (client.DiskUsageResult, error)
	ServiceList(ctx context.Context, options dockerServiceListOptions) ([]swarmtypes.Service, error)
	TaskList(ctx context.Context, options dockerTaskListOptions) ([]swarmtypes.Task, error)
	ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error)
	Close() error
}

type mobyDockerClient struct {
	*client.Client
}

func newMobyDockerClient(opts ...client.Opt) (dockerClient, error) {
	cli, err := client.New(opts...)
	if err != nil {
		return nil, err
	}
	return &mobyDockerClient{Client: cli}, nil
}

func (m *mobyDockerClient) Info(ctx context.Context) (systemtypes.Info, error) {
	result, err := m.Client.Info(ctx, client.InfoOptions{})
	if err != nil {
		return systemtypes.Info{}, err
	}
	return result.Info, nil
}

func (m *mobyDockerClient) ContainerList(ctx context.Context, options dockerContainerListOptions) ([]containertypes.Summary, error) {
	result, err := m.Client.ContainerList(ctx, client.ContainerListOptions{
		All:     options.All,
		Filters: options.Filters.toClientFilters(),
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (m *mobyDockerClient) ContainerInspectWithRaw(ctx context.Context, containerID string, size bool) (containertypes.InspectResponse, []byte, error) {
	result, err := m.Client.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{Size: size})
	if err != nil {
		return containertypes.InspectResponse{}, nil, err
	}
	return result.Container, result.Raw, nil
}

func (m *mobyDockerClient) ContainerStatsOneShot(ctx context.Context, containerID string) (dockerStatsResponseReader, error) {
	result, err := m.Client.ContainerStats(ctx, containerID, client.ContainerStatsOptions{
		Stream:                false,
		IncludePreviousSample: false,
	})
	if err != nil {
		return dockerStatsResponseReader{}, err
	}
	return dockerStatsResponseReader{Body: result.Body}, nil
}

func (m *mobyDockerClient) ContainerInspect(ctx context.Context, containerID string) (containertypes.InspectResponse, error) {
	result, err := m.Client.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return containertypes.InspectResponse{}, err
	}
	return result.Container, nil
}

func (m *mobyDockerClient) ImagePull(ctx context.Context, ref string, _ dockerImagePullOptions) (io.ReadCloser, error) {
	return m.Client.ImagePull(ctx, ref, client.ImagePullOptions{})
}

func (m *mobyDockerClient) ContainerStop(ctx context.Context, containerID string, options dockerContainerStopOptions) error {
	_, err := m.Client.ContainerStop(ctx, containerID, client.ContainerStopOptions{
		Signal:  options.Signal,
		Timeout: options.Timeout,
	})
	return err
}

func (m *mobyDockerClient) ContainerRename(ctx context.Context, containerID, newName string) error {
	_, err := m.Client.ContainerRename(ctx, containerID, client.ContainerRenameOptions{NewName: newName})
	return err
}

func (m *mobyDockerClient) ContainerCreate(ctx context.Context, config *containertypes.Config, hostConfig *containertypes.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (containertypes.CreateResponse, error) {
	result, err := m.Client.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
		Platform:         platform,
		Name:             containerName,
	})
	if err != nil {
		return containertypes.CreateResponse{}, err
	}
	return containertypes.CreateResponse{ID: result.ID, Warnings: result.Warnings}, nil
}

func (m *mobyDockerClient) NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error {
	_, err := m.Client.NetworkConnect(ctx, networkID, client.NetworkConnectOptions{
		Container:      containerID,
		EndpointConfig: config,
	})
	return err
}

func (m *mobyDockerClient) ContainerStart(ctx context.Context, containerID string, options dockerContainerStartOptions) error {
	_, err := m.Client.ContainerStart(ctx, containerID, client.ContainerStartOptions{
		CheckpointID:  options.CheckpointID,
		CheckpointDir: options.CheckpointDir,
	})
	return err
}

func (m *mobyDockerClient) ContainerRemove(ctx context.Context, containerID string, options dockerContainerRemoveOptions) error {
	_, err := m.Client.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{
		RemoveVolumes: options.RemoveVolumes,
		RemoveLinks:   options.RemoveLinks,
		Force:         options.Force,
	})
	return err
}

func (m *mobyDockerClient) ImageList(ctx context.Context, options dockerImageListOptions) ([]image.Summary, error) {
	result, err := m.Client.ImageList(ctx, client.ImageListOptions{
		All:        options.All,
		SharedSize: options.SharedSize,
		Filters:    options.Filters.toClientFilters(),
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (m *mobyDockerClient) VolumeList(ctx context.Context, options dockerVolumeListOptions) ([]volume.Volume, error) {
	result, err := m.Client.VolumeList(ctx, client.VolumeListOptions{Filters: options.Filters.toClientFilters()})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (m *mobyDockerClient) NetworkList(ctx context.Context, options dockerNetworkListOptions) ([]network.Summary, error) {
	result, err := m.Client.NetworkList(ctx, client.NetworkListOptions{Filters: options.Filters.toClientFilters()})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (m *mobyDockerClient) DiskUsage(ctx context.Context, options dockerDiskUsageOptions) (client.DiskUsageResult, error) {
	return m.Client.DiskUsage(ctx, client.DiskUsageOptions{
		Containers: options.Containers,
		Images:     options.Images,
		BuildCache: options.BuildCache,
		Volumes:    options.Volumes,
		Verbose:    options.Verbose,
	})
}

func (m *mobyDockerClient) ServiceList(ctx context.Context, options dockerServiceListOptions) ([]swarmtypes.Service, error) {
	result, err := m.Client.ServiceList(ctx, client.ServiceListOptions{Status: options.Status})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (m *mobyDockerClient) TaskList(ctx context.Context, options dockerTaskListOptions) ([]swarmtypes.Task, error) {
	result, err := m.Client.TaskList(ctx, client.TaskListOptions{Filters: options.Filters.toClientFilters()})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (m *mobyDockerClient) ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
	result, err := m.Client.ImageInspect(ctx, imageID)
	if err != nil {
		return image.InspectResponse{}, nil, err
	}
	return result.InspectResponse, nil, nil
}
