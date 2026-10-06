// Package filesystemprobe reads container filesystem counters without executing
// container-controlled programs or exposing file contents.
package filesystemprobe

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/agents/filesystem"
)

const (
	Source         = "linux-proc-root-statfs"
	maxMountpoints = 128
	maxInFlight    = 32
	probeTimeout   = 3 * time.Second
)

type ContainerRequest struct {
	PID         int
	ContainerID string
	// Runtime is docker or podman (ContainerID is the 64-hex ID) or lxc, a
	// Proxmox container (ContainerID is the decimal VMID and PID must be the
	// container's init process).
	Runtime     string
	Mountpoints []string
	// MountsOnly reports a mountpoint that shares its parent's device as
	// ErrNotMounted instead of returning the parent filesystem's counters. A
	// Proxmox config can declare a mount point the running guest never mounted.
	MountsOnly bool
}

// ErrNotMounted marks an observation whose path is not a distinct mounted
// filesystem inside the container. Callers test it with IsNotMounted.
var ErrNotMounted = errors.New("path is not a mounted filesystem in the container")

// IsNotMounted reports whether an observation was rejected as ErrNotMounted.
func IsNotMounted(observation filesystem.Observation) bool {
	return observation.Error == ErrNotMounted.Error()
}

// Observer bounds potentially uninterruptible kernel reads. A timed-out probe
// keeps its slot until the syscall returns. Later collections never share its
// stale result or start another probe for the same container in the meantime.
// The zero value is ready to use and must not be copied after first use.
type Observer struct {
	mu       sync.Mutex
	inFlight map[string]struct{}
	probe    func(context.Context, ContainerRequest) ([]filesystem.Observation, error)
	timeout  time.Duration
}

func (o *Observer) Observe(ctx context.Context, req ContainerRequest) ([]filesystem.Observation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateRequest(req); err != nil {
		return nil, err
	}
	// The asynchronous read must own its coordinates after its caller returns.
	req.Mountpoints = append([]string(nil), req.Mountpoints...)
	key := req.Runtime + ":" + req.ContainerID
	o.mu.Lock()
	if _, exists := o.inFlight[key]; exists {
		o.mu.Unlock()
		return nil, errors.New("previous filesystem observation is still running")
	}
	if len(o.inFlight) >= maxInFlight {
		o.mu.Unlock()
		return nil, errors.New("filesystem observation concurrency limit reached")
	}
	if o.inFlight == nil {
		o.inFlight = make(map[string]struct{})
	}
	o.inFlight[key] = struct{}{}
	o.mu.Unlock()

	timeout := o.timeout
	if timeout <= 0 {
		timeout = probeTimeout
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	type result struct {
		observations []filesystem.Observation
		err          error
	}
	done := make(chan result, 1)
	probe := o.probe
	if probe == nil {
		probe = observeContainer
	}
	go func() {
		observations, err := probe(probeCtx, req)
		o.mu.Lock()
		delete(o.inFlight, key)
		o.mu.Unlock()
		done <- result{observations, err}
	}()
	select {
	case r := <-done:
		if err := probeCtx.Err(); err != nil {
			return nil, err
		}
		return r.observations, r.err
	case <-probeCtx.Done():
		return nil, probeCtx.Err()
	}
}

func validateRequest(req ContainerRequest) error {
	if req.PID <= 0 {
		return errors.New("container process is unavailable")
	}
	switch req.Runtime {
	case "docker", "podman":
		if len(req.ContainerID) != 64 || strings.IndexFunc(req.ContainerID, func(r rune) bool {
			return !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f')
		}) >= 0 {
			return errors.New("filesystem observation requires an exact container ID")
		}
	case "lxc":
		if !validLXCVMID(req.ContainerID) {
			return errors.New("filesystem observation requires an exact Proxmox VMID")
		}
	default:
		return errors.New("unsupported container runtime")
	}
	if len(req.Mountpoints) == 0 || len(req.Mountpoints) > maxMountpoints {
		return errors.New("invalid filesystem mountpoint count")
	}
	seen := make(map[string]bool, len(req.Mountpoints))
	for _, mount := range req.Mountpoints {
		if !path.IsAbs(mount) || path.Clean(mount) != mount || strings.ContainsRune(mount, 0) || len(mount) > 4096 || seen[mount] {
			return fmt.Errorf("invalid or duplicate filesystem mountpoint %q", mount)
		}
		seen[mount] = true
	}
	return nil
}

func validLXCVMID(id string) bool {
	vmid, err := strconv.Atoi(id)
	return err == nil && vmid >= 100 && vmid <= 999999999 && strconv.Itoa(vmid) == id
}

// CgroupMatches reports whether a /proc/<pid>/cgroup body places the process
// inside the given container, with the same exact-identity rules the observer
// applies to its pinned process.
func CgroupMatches(raw, runtime, containerID string) bool {
	return cgroupMatches(raw, ContainerRequest{Runtime: runtime, ContainerID: containerID})
}

// IsNamespaceInit reports whether a /proc/<pid>/status body describes pid as
// PID 1 of a PID namespace one level below the reader's: NSpid is exactly
// "<pid> 1". A process that is PID 1 of a namespace nested inside the
// container (Docker inside an LXC guest) lists three or more PIDs and is not
// the container's init.
func IsNamespaceInit(status string, pid int) bool {
	for _, line := range strings.Split(status, "\n") {
		rest, ok := strings.CutPrefix(line, "NSpid:")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		return len(fields) == 2 && fields[0] == strconv.Itoa(pid) && fields[1] == "1"
	}
	return false
}

// cgroupMatches accepts an exact runtime-owned cgroup component, never a
// substring, container name, short ID, or unrelated process with the same PID.
func cgroupMatches(raw string, req ContainerRequest) bool {
	if req.Runtime == "lxc" {
		// pve-container places a guest under lxc/<vmid> (payload in ns/).
		// lxc.monitor/<vmid> and other VMIDs sharing a prefix never match.
		for _, line := range strings.Split(raw, "\n") {
			parts := strings.SplitN(line, ":", 3)
			if len(parts) != 3 {
				continue
			}
			components := strings.Split(parts[2], "/")
			if len(components) >= 3 && components[0] == "" && components[1] == "lxc" && components[2] == req.ContainerID {
				return true
			}
		}
		return false
	}
	scope := "docker-" + req.ContainerID + ".scope"
	if req.Runtime == "podman" {
		scope = "libpod-" + req.ContainerID + ".scope"
	}
	for _, line := range strings.Split(raw, "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		for _, component := range strings.Split(parts[2], "/") {
			if component == scope || req.Runtime == "docker" && component == req.ContainerID {
				return true
			}
		}
	}
	return false
}
