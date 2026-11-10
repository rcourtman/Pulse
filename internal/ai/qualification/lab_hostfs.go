package qualification

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path"
	"strconv"
	"strings"
	"time"
)

// A host_filesystem resource is a small filesystem mounted for the lab alone,
// such as a loop image. The operator creates and mounts it, names its path on
// the command line and the manifest names the same path. The driver then only
// ever writes below that mount, only after measuring that it really is a
// separate filesystem of a bounded size, and removes exactly the files it
// wrote. It needs no root: the mount point is owned by the lab user.
const (
	hostFilesystemMinBytes = int64(64 << 20)
	hostFilesystemMaxBytes = int64(1 << 30)
	// The fill fault writes fixed-size files until the filesystem reports this
	// much use. On the 400 MiB image the runbook describes that is 12 files
	// and 92.3%, the state fault set v1 was scored on.
	hostFillFileMiB       = 20
	hostFillTargetPercent = 92.0
	hostFillMaxFiles      = 64
)

type HostFilesystemState struct {
	MountPath  string   `json:"mount_path"`
	FaultDir   string   `json:"fault_dir,omitempty"`
	FaultFiles []string `json:"fault_files,omitempty"`
}

type hostFilesystemUsage struct {
	BlockSize int64
	Blocks    int64
	Free      int64
	Available int64
}

func (u hostFilesystemUsage) TotalBytes() int64 { return u.BlockSize * u.Blocks }

// UsedPercent follows df: used over used plus what an unprivileged writer may
// still take, rounded to one decimal.
func (u hostFilesystemUsage) UsedPercent() float64 {
	used := u.Blocks - u.Free
	if used+u.Available <= 0 {
		return 0
	}
	return math.Round(float64(used)/float64(used+u.Available)*1000) / 10
}

// host runs one fixed command on the machine that holds the lab. Over SSH that
// is the SSH host. Through a Docker context it is this machine, and only when
// the context talks to a local socket, because a remote daemon's filesystems
// are not reachable from here.
func (l *DockerLab) host(ctx context.Context, name string, args ...string) (CommandResult, error) {
	if l.target.SSHHost != "" {
		quoted := make([]string, 0, len(args)+1)
		quoted = append(quoted, shellQuote(name))
		for _, arg := range args {
			quoted = append(quoted, shellQuote(arg))
		}
		return l.runner.Run(ctx, "ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=15", l.target.SSHHost, strings.Join(quoted, " "))
	}
	if !l.localHostConfirmed {
		result, err := l.runner.Run(ctx, "docker", "context", "inspect", l.target.Context, "--format", "{{ .Endpoints.docker.Host }}")
		if err != nil {
			return CommandResult{}, fmt.Errorf("resolve Docker context %q: %w", l.target.Context, err)
		}
		if endpoint := strings.TrimSpace(result.Stdout); !strings.HasPrefix(endpoint, "unix://") {
			return CommandResult{}, fmt.Errorf("Docker context %q is not a local socket: run the harness on the lab host to use a host filesystem fixture", l.target.Context)
		}
		l.localHostConfirmed = true
	}
	return l.runner.Run(ctx, name, args...)
}

func (l *DockerLab) hostFilesystemUsage(ctx context.Context, mountPath string) (hostFilesystemUsage, error) {
	result, err := l.host(ctx, "stat", "-f", "-c", "%S %b %f %a", mountPath)
	if err != nil {
		return hostFilesystemUsage{}, fmt.Errorf("measure %s: %w", mountPath, err)
	}
	return parseHostFilesystemUsage(result.Stdout)
}

func parseHostFilesystemUsage(output string) (hostFilesystemUsage, error) {
	fields := strings.Fields(output)
	if len(fields) != 4 {
		return hostFilesystemUsage{}, errors.New("unexpected filesystem measurement")
	}
	values := [4]int64{}
	for i, field := range fields {
		value, err := strconv.ParseInt(field, 10, 64)
		if err != nil || value < 0 {
			return hostFilesystemUsage{}, errors.New("unexpected filesystem measurement")
		}
		values[i] = value
	}
	usage := hostFilesystemUsage{BlockSize: values[0], Blocks: values[1], Free: values[2], Available: values[3]}
	if usage.BlockSize == 0 || usage.Blocks == 0 || usage.Free > usage.Blocks || usage.Available > usage.Free {
		return hostFilesystemUsage{}, errors.New("unexpected filesystem measurement")
	}
	return usage, nil
}

// confirmDedicatedHostFilesystem is the guard every write depends on. The path
// has to be one the operator named, a mount of its own, and small. An
// unmounted directory fails twice: it shares its parent's device and reports
// the size of the disk it actually sits on.
func (l *DockerLab) confirmDedicatedHostFilesystem(ctx context.Context, mountPath string) (hostFilesystemUsage, error) {
	if !hostFilesystemPath.MatchString(mountPath) {
		return hostFilesystemUsage{}, fmt.Errorf("%s is not an accepted lab mount path", mountPath)
	}
	if !contains(l.target.HostFilesystems, mountPath) {
		return hostFilesystemUsage{}, fmt.Errorf("host filesystem %s was not handed to the lab: name it with --host-filesystem", mountPath)
	}
	devices, err := l.host(ctx, "stat", "-c", "%d", mountPath, path.Dir(mountPath))
	if err != nil {
		return hostFilesystemUsage{}, fmt.Errorf("inspect %s: %w", mountPath, err)
	}
	ids := strings.Fields(devices.Stdout)
	if len(ids) != 2 || ids[0] == ids[1] {
		return hostFilesystemUsage{}, fmt.Errorf("%s is not a separate mounted filesystem", mountPath)
	}
	usage, err := l.hostFilesystemUsage(ctx, mountPath)
	if err != nil {
		return hostFilesystemUsage{}, err
	}
	if total := usage.TotalBytes(); total < hostFilesystemMinBytes || total > hostFilesystemMaxBytes {
		return hostFilesystemUsage{}, fmt.Errorf("%s holds %d bytes: a lab filesystem is between %d and %d bytes", mountPath, total, hostFilesystemMinBytes, hostFilesystemMaxBytes)
	}
	return usage, nil
}

// hostSeedFile is one file of the standing content of a lab filesystem. The
// content is what makes the fault a question worth asking: several folders
// already hold data, and only one of them grows.
type hostSeedFile struct {
	Path string
	MiB  int
}

func hostFilesystemSeed(now time.Time) []hostSeedFile {
	yesterday := now.UTC().AddDate(0, 0, -1).Format("2006-01-02")
	return []hostSeedFile{
		{Path: "music/library.tar", MiB: 20},
		{Path: "photos/2025-archive.tar", MiB: 30},
		{Path: path.Join("frigate/recordings", yesterday, "14", "front_door", "00.00.mp4"), MiB: 15},
	}
}

// hostSeedMarkers are the seed files with a fixed path. Their presence is how
// a run tells a seeded lab filesystem from an empty one.
var hostSeedMarkers = []string{"music/library.tar", "photos/2025-archive.tar"}

// SeedHostFilesystem writes the standing content once. Like building fixture
// images it is a separate, explicit step, and it refuses to overwrite.
func (l *DockerLab) SeedHostFilesystem(ctx context.Context, mountPath string, now time.Time) (bool, error) {
	if _, err := l.confirmDedicatedHostFilesystem(ctx, mountPath); err != nil {
		return false, err
	}
	if l.hostSeedPresent(ctx, mountPath) {
		return false, nil
	}
	if _, err := l.host(ctx, "mkdir", "-p", path.Join(mountPath, "frigate", "clips")); err != nil {
		return false, err
	}
	for _, file := range hostFilesystemSeed(now) {
		target := path.Join(mountPath, file.Path)
		if _, err := l.host(ctx, "mkdir", "-p", path.Dir(target)); err != nil {
			return false, err
		}
		if err := l.writeHostFile(ctx, target, file.MiB); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (l *DockerLab) hostSeedPresent(ctx context.Context, mountPath string) bool {
	for _, marker := range hostSeedMarkers {
		if _, err := l.host(ctx, "test", "-f", path.Join(mountPath, marker)); err != nil {
			return false
		}
	}
	return true
}

// writeHostFile creates one file of incompressible data. conv=excl refuses an
// existing file, so the driver can never truncate something it did not write.
func (l *DockerLab) writeHostFile(ctx context.Context, target string, mib int) error {
	if _, err := l.host(ctx, "dd", "if=/dev/urandom", "of="+target, "bs=1M", "count="+strconv.Itoa(mib), "conv=excl", "status=none"); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}
	return nil
}

func (l *DockerLab) prepareHostFilesystem(ctx context.Context, lab *PreparedLab, resource ResourceSpec) error {
	if _, err := l.confirmDedicatedHostFilesystem(ctx, resource.MountPath); err != nil {
		return err
	}
	if !l.hostSeedPresent(ctx, resource.MountPath) {
		return fmt.Errorf("host filesystem %s has no standing content: run patrol-qualify -mode build-fixtures with --host-filesystem first", resource.MountPath)
	}
	if lab.HostFilesystems == nil {
		lab.HostFilesystems = make(map[string]*HostFilesystemState)
	}
	lab.HostFilesystems[resource.Alias] = &HostFilesystemState{MountPath: resource.MountPath}
	return nil
}

// fillHostFilesystem makes one folder grow until the filesystem is nearly
// full: one camera's recordings for the current hour. The layout is fixed
// here, so a manifest cannot point the writes anywhere else.
func (l *DockerLab) fillHostFilesystem(ctx context.Context, lab *PreparedLab, alias string) error {
	state := lab.HostFilesystems[alias]
	if state == nil {
		return fmt.Errorf("resource %q is not a prepared host filesystem", alias)
	}
	if len(state.FaultFiles) > 0 {
		return fmt.Errorf("host filesystem %q already holds a fill fault", alias)
	}
	usage, err := l.confirmDedicatedHostFilesystem(ctx, state.MountPath)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	state.FaultDir = path.Join(state.MountPath, "frigate", "recordings", now.Format("2006-01-02"), now.Format("15"), "driveway")
	if _, err := l.host(ctx, "mkdir", "-p", state.FaultDir); err != nil {
		return err
	}
	for index := 0; index < hostFillMaxFiles && usage.UsedPercent() < hostFillTargetPercent; index++ {
		target := path.Join(state.FaultDir, fmt.Sprintf("%02d.00.mp4", index))
		// Record the file before writing it: a write that fails half way
		// still leaves bytes behind for revert to remove.
		state.FaultFiles = append(state.FaultFiles, target)
		if err := l.writeHostFile(ctx, target, hostFillFileMiB); err != nil {
			return err
		}
		if usage, err = l.hostFilesystemUsage(ctx, state.MountPath); err != nil {
			return err
		}
	}
	if usage.UsedPercent() < hostFillTargetPercent {
		return fmt.Errorf("host filesystem %s reached only %.1f%% after %d files", state.MountPath, usage.UsedPercent(), len(state.FaultFiles))
	}
	return nil
}

func (l *DockerLab) clearHostFilesystemFault(ctx context.Context, lab *PreparedLab, alias string) error {
	state := lab.HostFilesystems[alias]
	if state == nil {
		return fmt.Errorf("resource %q is not a prepared host filesystem", alias)
	}
	var errs []error
	for _, file := range state.FaultFiles {
		if !strings.HasPrefix(file, state.MountPath+"/") {
			errs = append(errs, fmt.Errorf("refusing to remove %s outside %s", file, state.MountPath))
			continue
		}
		if _, err := l.host(ctx, "rm", "-f", file); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		state.FaultFiles = nil
	}
	// Remove the folders the fault created, innermost first. rmdir leaves a
	// folder that still holds something, so the standing content is safe.
	for dir, depth := state.FaultDir, 0; dir != "" && depth < 3 && strings.HasPrefix(dir, state.MountPath+"/"); dir, depth = path.Dir(dir), depth+1 {
		if _, err := l.host(ctx, "rmdir", dir); err != nil {
			break
		}
	}
	return errors.Join(errs...)
}
