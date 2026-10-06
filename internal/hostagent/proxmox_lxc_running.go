package hostagent

import (
	"context"
	"errors"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/filesystemprobe"
	"github.com/rcourtman/pulse-go-rewrite/pkg/agents/filesystem"
)

// Socket-free discovery of the node's running containers (#2511).
//
// pct list, pct df and lxc-info reach pmxcfs and the LXC monitor through
// abstract Unix sockets, which belong to a network namespace. The typed helper
// runs with PrivateNetwork=true, so every one of them fails there. Container
// configs are ordinary reads through the pmxcfs FUSE mount, and a running
// container's init process is visible in its cgroup and in /proc, so the helper
// can find what it needs without leaving its sandbox. Usage is then read by
// the shared filesystem prober, which pins the process, re-checks its identity
// and resolves every path inside the container's root.

const (
	// proxmoxLXCCgroupRoot is where pve-container places each container's
	// cgroup (lxc/<vmid>, with the payload under ns/) on a cgroup v2 host.
	proxmoxLXCCgroupRoot          = "/sys/fs/cgroup/lxc"
	proxmoxLXCCgroupV2Marker      = "/sys/fs/cgroup/cgroup.controllers"
	proxmoxLXCMaxCgroupDirs       = 64
	proxmoxLXCMaxCgroupPIDs       = 4096
	proxmoxLXCMaxCgroupProcsBytes = 64 * 1024
	proxmoxLXCMaxProcStatusBytes  = 64 * 1024
	proxmoxLXCMaxProcCgroupBytes  = 64 * 1024
	proxmoxLXCMaxConfigEntries    = 4096
)

// proxmoxLXCDirLister is an optional collector capability, like
// proxmoxLXCObserver: the default collector lists directories and test
// collectors opt in.
type proxmoxLXCDirLister interface {
	ReadDir(name string) ([]os.DirEntry, error)
}

// proxmoxLXCObserver reads a running container's filesystem counters through
// its pinned init process.
type proxmoxLXCObserver interface {
	ObserveProxmoxLXC(ctx context.Context, req filesystemprobe.ContainerRequest) ([]filesystem.Observation, error)
}

// proxmoxLXCFilesystemObserver is shared by every collection in the process.
// A probe stuck on a guest's hung filesystem keeps its slot here, so the
// typed helper never stacks a second probe on that guest.
var proxmoxLXCFilesystemObserver filesystemprobe.Observer

func (c *defaultCollector) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}

func (c *defaultCollector) ObserveProxmoxLXC(ctx context.Context, req filesystemprobe.ContainerRequest) ([]filesystem.Observation, error) {
	return proxmoxLXCFilesystemObserver.Observe(ctx, req)
}

type proxmoxLXCInitState int

const (
	proxmoxLXCStopped proxmoxLXCInitState = iota
	proxmoxLXCRunning
	proxmoxLXCUnknown
)

// discoverRunningProxmoxLXCContainers lists this node's running containers
// with their init PIDs. ok is false when the host cannot be read this way or
// any container's state cannot be established (no directory listing, no
// cgroup v2 hierarchy, an unreadable config or cgroup, a search limit, or
// cancellation), so the caller uses pct instead of reporting an inventory that
// silently misses a running guest. Only a container with no cgroup at all is
// stopped and left out, as pct list leaves it out.
func (a *Agent) discoverRunningProxmoxLXCContainers(ctx context.Context) ([]proxmoxLXCRunningContainer, bool) {
	lister, isLister := a.collector.(proxmoxLXCDirLister)
	if !isLister {
		return nil, false
	}
	if _, err := a.collector.Stat(proxmoxLXCCgroupV2Marker); err != nil {
		return nil, false
	}
	entries, err := lister.ReadDir(proxmoxLXCConfigDir)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Cannot list Proxmox LXC configs; using pct")
		return nil, false
	}
	if len(entries) > proxmoxLXCMaxConfigEntries {
		return nil, false
	}

	vmids := make([]int, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".conf") {
			continue
		}
		vmid, convErr := strconv.Atoi(strings.TrimSuffix(name, ".conf"))
		if convErr != nil || vmid < 100 || vmid > 999999999 {
			continue
		}
		vmids = append(vmids, vmid)
	}
	sort.Ints(vmids)

	running := make([]proxmoxLXCRunningContainer, 0, len(vmids))
	for _, vmid := range vmids {
		if ctx.Err() != nil {
			return nil, false
		}
		pid, state := a.proxmoxLXCInitPID(ctx, lister, vmid)
		switch state {
		case proxmoxLXCStopped:
			continue
		case proxmoxLXCUnknown:
			a.logger.Debug().Int("vmid", vmid).Msg("Cannot establish a Proxmox LXC init process; using pct")
			return nil, false
		}
		raw, readErr := a.collector.ReadFile(proxmoxLXCConfigPath(vmid))
		if readErr != nil || len(raw) > proxmoxLXCMaxConfigBytes {
			// Without its config a running container has no trustworthy name or
			// mounts; leave the whole node to pct rather than guess.
			a.logger.Debug().Err(readErr).Int("vmid", vmid).Msg("Cannot read a running Proxmox LXC config; using pct")
			return nil, false
		}
		running = append(running, proxmoxLXCRunningContainer{VMID: vmid, Name: proxmoxLXCConfigName(string(raw), vmid), PID: pid})
		if len(running) > proxmoxLXCMaxContainers {
			// A truncated list cannot authoritatively declare the node complete.
			// Keep the same bound as pct parsing and let the caller report a
			// degraded fallback, rather than erase uninspected running guests.
			return nil, false
		}
	}
	return running, true
}

// proxmoxLXCInitPID finds the container's init process: the process in the
// container's cgroup tree that is PID 1 of the namespace directly below the
// host's and whose own cgroup is inside lxc/<vmid>. The filesystem prober
// re-checks both on the pinned process before reading anything.
func (a *Agent) proxmoxLXCInitPID(ctx context.Context, lister proxmoxLXCDirLister, vmid int) (int, proxmoxLXCInitState) {
	root := path.Join(proxmoxLXCCgroupRoot, strconv.Itoa(vmid))
	if _, err := lister.ReadDir(root); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, proxmoxLXCStopped
		}
		return 0, proxmoxLXCUnknown
	}

	queue := []string{root}
	queued := 1
	checked := 0
	for len(queue) > 0 {
		if ctx.Err() != nil {
			return 0, proxmoxLXCUnknown
		}
		dir := queue[0]
		queue = queue[1:]

		raw, err := a.collector.ReadFile(path.Join(dir, "cgroup.procs"))
		if err != nil || len(raw) > proxmoxLXCMaxCgroupProcsBytes {
			return 0, proxmoxLXCUnknown
		}
		for _, field := range strings.Fields(string(raw)) {
			if checked == proxmoxLXCMaxCgroupPIDs {
				return 0, proxmoxLXCUnknown
			}
			checked++
			pid, convErr := strconv.Atoi(field)
			if convErr != nil || pid <= 0 {
				continue
			}
			if a.isProxmoxLXCInit(pid, vmid) {
				return pid, proxmoxLXCRunning
			}
		}

		entries, err := lister.ReadDir(dir)
		if err != nil {
			return 0, proxmoxLXCUnknown
		}
		children := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() {
				children = append(children, entry.Name())
			}
		}
		// systemd guests run init in init.scope; look there first.
		sort.SliceStable(children, func(i, j int) bool {
			return children[i] == "init.scope" && children[j] != "init.scope"
		})
		for _, child := range children {
			if queued == proxmoxLXCMaxCgroupDirs {
				// A guest can create cgroups in its delegated subtree; give up
				// rather than walk an unbounded tree.
				return 0, proxmoxLXCUnknown
			}
			queued++
			queue = append(queue, path.Join(dir, child))
		}
	}
	// A cgroup without an init is a guest starting or stopping.
	return 0, proxmoxLXCUnknown
}

func (a *Agent) isProxmoxLXCInit(pid, vmid int) bool {
	status, err := a.collector.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil || len(status) > proxmoxLXCMaxProcStatusBytes {
		return false
	}
	if !filesystemprobe.IsNamespaceInit(string(status), pid) {
		return false
	}
	cgroup, err := a.collector.ReadFile("/proc/" + strconv.Itoa(pid) + "/cgroup")
	if err != nil || len(cgroup) > proxmoxLXCMaxProcCgroupBytes {
		return false
	}
	return filesystemprobe.CgroupMatches(string(cgroup), "lxc", strconv.Itoa(vmid))
}

// proxmoxLXCConfigName is the name pct list and the Proxmox API report: the
// configured hostname, or CT<vmid> when none is set. Pulse matches agent
// readings to API containers by this name.
func proxmoxLXCConfigName(config string, vmid int) string {
	for _, rawLine := range strings.Split(config, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "[") {
			break
		}
		value, ok := strings.CutPrefix(line, "hostname:")
		if !ok {
			continue
		}
		if name := strings.TrimSpace(value); name != "" && safeProxmoxLXCText(name, proxmoxLXCMaxNameBytes) {
			return name
		}
		break
	}
	return "CT" + strconv.Itoa(vmid)
}
