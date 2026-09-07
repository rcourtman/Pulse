package dockeragent

import (
	"context"
	"errors"
	"net/url"
	"path"
	"sort"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/rcourtman/pulse-go-rewrite/internal/filesystemprobe"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rcourtman/pulse-go-rewrite/pkg/agents/filesystem"
)

func (a *Agent) collectContainerFilesystems(ctx context.Context, id string, inspect containertypes.InspectResponse, mounts []agentsdocker.ContainerMount) []filesystem.Observation {
	// Native filesystem counters are ordinary resource evidence. The separate
	// CollectDiskMetrics switch controls Docker's expensive image-layer sizing,
	// which the unified agent deliberately disables.
	paths := []string{"/"}
	seen := map[string]bool{"/": true}
	for _, mount := range mounts {
		if !seen[mount.Destination] {
			paths = append(paths, mount.Destination)
			seen[mount.Destination] = true
		}
	}
	sort.Strings(paths)
	failed := func(err error) []filesystem.Observation {
		out := make([]filesystem.Observation, len(paths))
		for i, mount := range paths {
			out[i] = filesystem.Observation{Mountpoint: mount, Source: filesystemprobe.Source, ObservedAt: time.Now().UTC(), Error: err.Error()}
		}
		return out
	}
	if inspect.State == nil || (!inspect.State.Running && !inspect.State.Paused) || inspect.State.Pid <= 0 {
		return failed(errors.New("container filesystem namespace is not running"))
	}
	endpoint, err := url.Parse(a.docker.DaemonHost())
	if err != nil || endpoint.Scheme != "unix" || endpoint.Host != "" || !path.IsAbs(endpoint.Path) {
		return failed(errors.New("filesystem observations require a local runtime socket and an attested local container process"))
	}
	if inspect.ID != id {
		return failed(errors.New("container inspect identity does not match the collected resource"))
	}
	observe := a.observeFilesystems
	if observe == nil {
		observe = a.filesystemObserver.Observe
	}
	observations, err := observe(ctx, filesystemprobe.ContainerRequest{
		PID: inspect.State.Pid, ContainerID: id, Runtime: string(a.runtime), Mountpoints: paths,
	})
	if err != nil {
		return failed(err)
	}
	// Reinspect after the native read. Never attach the old namespace's counters
	// to a container that restarted during collection, even if its ID is stable.
	current, err := a.docker.ContainerInspect(ctx, id)
	if err != nil {
		return failed(errors.New("container identity could not be revalidated after filesystem observation"))
	}
	if current.ID != id || current.State == nil || current.State.Pid != inspect.State.Pid || current.State.StartedAt != inspect.State.StartedAt || (!current.State.Running && !current.State.Paused) {
		return failed(errors.New("container process changed during filesystem observation"))
	}
	return observations
}
