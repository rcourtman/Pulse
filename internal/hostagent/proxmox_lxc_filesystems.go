package hostagent

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

// resolvePctPath mirrors resolveSmartctlPath: PULSE_PCT_PATH lets a
// least-privilege install point the read-only pct list/df queries at a scoped
// privilege helper instead of requiring the whole agent to run as root. The
// override must be absolute so a PATH-relative name cannot be hijacked.
func resolvePctPath(lookPath func(string) (string, error)) (string, error) {
	if configured := strings.TrimSpace(os.Getenv("PULSE_PCT_PATH")); configured != "" {
		if !filepath.IsAbs(configured) {
			return "", fmt.Errorf("PULSE_PCT_PATH must be an absolute path")
		}
		return configured, nil
	}
	return lookPath("pct")
}

const (
	proxmoxLXCQueryTimeout          = 10 * time.Second
	proxmoxLXCContainerQueryTimeout = 5 * time.Second
	proxmoxLXCCollectionTimeout     = 20 * time.Second
	proxmoxLXCMaxContainers         = 128
	proxmoxLXCMaxDisks              = 257
	proxmoxLXCMaxListOutputBytes    = 64 * 1024
	proxmoxLXCMaxDFOutputBytes      = 64 * 1024
	proxmoxLXCMaxConfigBytes        = 64 * 1024
	proxmoxLXCMaxPIDOutputBytes     = 4 * 1024
	proxmoxLXCMaxNameBytes          = 256
	proxmoxLXCMaxLabelBytes         = 512
)

// proxmoxLXCConfigDir is a pmxcfs symlink to the local node's container
// configs, so reading through it never picks up guests owned by cluster peers.
const proxmoxLXCConfigDir = "/etc/pve/lxc"

type proxmoxLXCRunningContainer struct {
	VMID int
	Name string
}

// ProxmoxLXCFilesystemCollectionResult describes applicability separately from
// collection health without changing the helper protocol-v1 response shape.
// A degraded result may retain a partial inventory for local diagnostics, but
// the privileged helper fails the typed operation so the collector cannot
// mistake a partial snapshot for a complete one.
type ProxmoxLXCFilesystemCollectionResult struct {
	Applicable       bool
	Inventory        *agentshost.ProxmoxLXCInventory
	Degraded         bool
	FailedContainers int
}

// hostFilesystemUsage carries statfs-derived usage for one path plus the
// st_dev identity needed to tell a real mount from its parent filesystem.
type hostFilesystemUsage struct {
	TotalBytes int64
	UsedBytes  int64
	AvailBytes int64
	Device     uint64
}

// filesystemUsageProber is an optional collector capability. It exists on
// Linux builds of the default collector; mocks opt in for tests.
type filesystemUsageProber interface {
	FilesystemUsage(path string) (hostFilesystemUsage, error)
}

type proxmoxLXCConfigMount struct {
	Key    string
	Volume string
	Path   string
}

// CollectProxmoxLXCFilesystemsLocal runs the bounded, node-local Proxmox LXC
// filesystem collector with the production system collector. It exists for
// the root-only typed helper, whose caller cannot supply VMIDs, paths, command
// names, or arguments.
func CollectProxmoxLXCFilesystemsLocal(ctx context.Context) *agentshost.ProxmoxLXCInventory {
	return CollectProxmoxLXCFilesystemsLocalResult(ctx).Inventory
}

// CollectProxmoxLXCFilesystemsLocalResult exposes applicability and completeness
// to the fixed local helper provider. It never accepts caller-selected VMIDs,
// paths, executables, or command arguments.
func CollectProxmoxLXCFilesystemsLocalResult(ctx context.Context) ProxmoxLXCFilesystemCollectionResult {
	logger := zerolog.Nop()
	agent := &Agent{
		collector: NewDefaultCollector(),
		logger:    logger,
	}
	return agent.collectProxmoxLXCFilesystemsResult(ctx)
}

// collectProxmoxLXCFilesystems auto-detects the local Proxmox Container
// Toolkit. It lists node-local guests, then resolves per-mount usage only for
// containers that the same list reported as running. Collection is read-only
// and best-effort; a failed per-container query does not suppress other
// results.
//
// Usage comes from statfs against /proc/<pid>/root when the agent can resolve
// the container's init PID, because pct df takes the container config lock and
// costs over a second per guest; that expense starved later containers when
// every query shared one budget. pct df remains as a per-container fallback
// for least-privilege installs that cannot traverse /proc/<pid>/root, and each
// container gets its own deadline so one slow guest cannot consume the whole
// collection window.
func (a *Agent) collectProxmoxLXCFilesystems(ctx context.Context) *agentshost.ProxmoxLXCInventory {
	return a.collectProxmoxLXCFilesystemsResult(ctx).Inventory
}

func (a *Agent) collectProxmoxLXCFilesystemsResult(ctx context.Context) ProxmoxLXCFilesystemCollectionResult {
	result := ProxmoxLXCFilesystemCollectionResult{}
	if a.collector.GOOS() != "linux" {
		return result
	}
	pctPath, err := resolvePctPath(a.collector.LookPath)
	if err != nil {
		if !errors.Is(err, exec.ErrNotFound) && !os.IsNotExist(err) {
			a.logger.Debug().Err(err).Msg("Failed to locate pct for Proxmox LXC filesystems")
			result.Applicable = true
			result.Degraded = true
		}
		return result
	}
	result.Applicable = true

	listCtx, cancelList := context.WithTimeout(ctx, proxmoxLXCQueryTimeout)
	listOutput, err := collectCommandOutputLimited(
		listCtx,
		a.collector,
		proxmoxLXCMaxListOutputBytes,
		pctPath,
		"list",
	)
	cancelList()
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to list local Proxmox LXC containers")
		result.Degraded = true
		return result
	}
	containers, err := parseProxmoxLXCRunningContainers(listOutput)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to parse local Proxmox LXC container list")
		result.Degraded = true
		return result
	}

	prober, _ := a.collector.(filesystemUsageProber)
	lxcInfoPath := ""
	if prober != nil {
		if resolved, lookErr := a.collector.LookPath("lxc-info"); lookErr == nil {
			lxcInfoPath = resolved
		}
	}

	collectionCtx, cancelCollection := context.WithTimeout(ctx, proxmoxLXCCollectionTimeout)
	defer cancelCollection()

	result.Inventory = &agentshost.ProxmoxLXCInventory{
		Containers:  []agentshost.ProxmoxLXCContainer{},
		CollectedAt: a.collector.Now().UTC(),
	}
	for _, container := range containers {
		if collectionCtx.Err() != nil {
			result.FailedContainers++
			continue
		}
		containerCtx, cancelContainer := context.WithTimeout(collectionCtx, proxmoxLXCContainerQueryTimeout)
		disks, collectionErr := a.collectProxmoxLXCContainerDisks(containerCtx, prober, lxcInfoPath, pctPath, container.VMID)
		cancelContainer()
		if collectionErr != nil {
			result.FailedContainers++
			continue
		}
		result.Inventory.Containers = append(result.Inventory.Containers, agentshost.ProxmoxLXCContainer{
			VMID:  container.VMID,
			Name:  container.Name,
			Disks: disks,
		})
	}
	if result.FailedContainers > 0 {
		result.Degraded = true
		a.logger.Warn().
			Int("collected", len(result.Inventory.Containers)).
			Int("failed", result.FailedContainers).
			Msg("Proxmox LXC filesystem collection omitted one or more running containers")
	}

	return result
}

func (a *Agent) collectProxmoxLXCContainerDisks(
	ctx context.Context,
	prober filesystemUsageProber,
	lxcInfoPath string,
	pctPath string,
	vmid int,
) ([]agentshost.Disk, error) {
	if prober != nil && lxcInfoPath != "" {
		disks, fastErr := a.collectProxmoxLXCContainerDisksFast(ctx, prober, lxcInfoPath, vmid)
		if fastErr == nil {
			return disks, nil
		}
		a.logger.Debug().
			Err(fastErr).
			Int("vmid", vmid).
			Msg("Proxmox LXC statfs probe unavailable; falling back to pct df")
	}

	output, outputErr := collectCommandOutputLimited(
		ctx,
		a.collector,
		proxmoxLXCMaxDFOutputBytes,
		pctPath,
		"df",
		strconv.Itoa(vmid),
	)
	if outputErr != nil {
		a.logger.Debug().
			Err(outputErr).
			Int("vmid", vmid).
			Msg("Failed to collect Proxmox LXC filesystem usage")
		return nil, errors.New("pct df collection failed")
	}
	disks, parseErr := parseProxmoxLXCDF(output)
	if parseErr != nil || len(disks) == 0 {
		a.logger.Debug().
			Err(parseErr).
			Int("vmid", vmid).
			Msg("Failed to parse Proxmox LXC filesystem usage")
		return nil, errors.New("pct df result contained no usable filesystems")
	}
	return disks, nil
}

// collectProxmoxLXCContainerDisksFast reads the container's config-declared
// mounts and resolves live usage with statfs through /proc/<pid>/root, which
// follows into the container's mount namespace without taking the pct config
// lock. A mount whose device identity matches its parent directory is not a
// distinct mounted filesystem there — statfs would report the parent's
// numbers — so it is dropped rather than invented.
func (a *Agent) collectProxmoxLXCContainerDisksFast(
	ctx context.Context,
	prober filesystemUsageProber,
	lxcInfoPath string,
	vmid int,
) ([]agentshost.Disk, error) {
	raw, err := a.collector.ReadFile(proxmoxLXCConfigPath(vmid))
	if err != nil {
		return nil, fmt.Errorf("read container config: %w", err)
	}
	if len(raw) > proxmoxLXCMaxConfigBytes {
		return nil, fmt.Errorf("container config exceeds %d bytes", proxmoxLXCMaxConfigBytes)
	}
	mounts := parseProxmoxLXCConfigMounts(string(raw))
	if len(mounts) == 0 {
		return nil, errors.New("no filesystem mounts in container config")
	}

	pidOutput, err := collectCommandOutputLimited(
		ctx,
		a.collector,
		proxmoxLXCMaxPIDOutputBytes,
		lxcInfoPath,
		"-n",
		strconv.Itoa(vmid),
		"-p",
	)
	if err != nil {
		return nil, fmt.Errorf("resolve container pid: %w", err)
	}
	pid, err := parseLXCInfoPID(pidOutput)
	if err != nil {
		return nil, err
	}

	procRoot := fmt.Sprintf("/proc/%d/root", pid)
	result := make([]agentshost.Disk, 0, len(mounts))
	for _, mount := range mounts {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		target := path.Join(procRoot, mount.Path)
		usage, usageErr := prober.FilesystemUsage(target)
		if usageErr != nil {
			return nil, fmt.Errorf("probe %s filesystem usage: %w", mount.Key, usageErr)
		}
		if usage.TotalBytes <= 0 {
			return nil, fmt.Errorf("probe %s filesystem usage: invalid total bytes", mount.Key)
		}
		if mount.Path != "/" {
			parent, parentErr := prober.FilesystemUsage(path.Dir(target))
			if parentErr != nil {
				return nil, fmt.Errorf("probe %s parent filesystem usage: %w", mount.Key, parentErr)
			}
			if parent.Device == usage.Device {
				continue
			}
		}
		used := usage.UsedBytes
		if used < 0 {
			used = 0
		}
		if used > usage.TotalBytes {
			used = usage.TotalBytes
		}
		avail := usage.AvailBytes
		if avail < 0 {
			avail = 0
		}
		if avail > usage.TotalBytes {
			avail = usage.TotalBytes
		}
		usagePct := 0.0
		if denominator := used + avail; denominator > 0 {
			usagePct = float64(used) / float64(denominator) * 100
		}
		if usagePct > 100 {
			usagePct = 100
		}
		result = append(result, agentshost.Disk{
			Device:     mount.Volume,
			Mountpoint: mount.Path,
			Type:       mount.Key,
			TotalBytes: usage.TotalBytes,
			UsedBytes:  used,
			FreeBytes:  avail,
			Usage:      usagePct,
		})
		if len(result) == proxmoxLXCMaxDisks {
			break
		}
	}
	if len(result) == 0 {
		return nil, errors.New("no mounted container filesystems resolved")
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Mountpoint == "/" {
			return true
		}
		if result[j].Mountpoint == "/" {
			return false
		}
		return result[i].Mountpoint < result[j].Mountpoint
	})
	return result, nil
}

func proxmoxLXCConfigPath(vmid int) string {
	// The collector reads the Linux Proxmox namespace even when native tests
	// exercise this path from another GOOS. filepath.Join would turn this into
	// a Windows path and silently force the slower pct fallback.
	return path.Join(proxmoxLXCConfigDir, strconv.Itoa(vmid)+".conf")
}

// parseProxmoxLXCConfigMounts extracts rootfs and mpN entries from the main
// section of a container config. Parsing stops at the first section header so
// snapshot and pending sections can never contribute mounts.
func parseProxmoxLXCConfigMounts(content string) []proxmoxLXCConfigMount {
	result := make([]proxmoxLXCConfigMount, 0, 4)
	seen := make(map[string]struct{})
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if strings.HasPrefix(line, "[") {
			break
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key != "rootfs" && !validProxmoxLXCMountKey(key) {
			continue
		}
		parts := strings.Split(strings.TrimSpace(value), ",")
		volume := strings.TrimSpace(parts[0])
		if !safeProxmoxLXCText(volume, proxmoxLXCMaxLabelBytes) {
			continue
		}
		mountPath := ""
		if key == "rootfs" {
			mountPath = "/"
		}
		for _, part := range parts[1:] {
			optionKey, optionValue, optionFound := strings.Cut(part, "=")
			if optionFound && strings.TrimSpace(optionKey) == "mp" {
				mountPath = strings.TrimSpace(optionValue)
			}
		}
		if !validProxmoxLXCMountpoint(mountPath) {
			continue
		}
		dedupeKey := strings.ToLower(mountPath)
		if _, exists := seen[dedupeKey]; exists {
			continue
		}
		seen[dedupeKey] = struct{}{}
		result = append(result, proxmoxLXCConfigMount{Key: key, Volume: volume, Path: mountPath})
		if len(result) == proxmoxLXCMaxDisks {
			break
		}
	}
	return result
}

func parseLXCInfoPID(output string) (int, error) {
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		value, found := strings.CutPrefix(line, "PID:")
		if !found {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || pid < 2 {
			return 0, fmt.Errorf("invalid lxc-info pid %q", strings.TrimSpace(value))
		}
		return pid, nil
	}
	return 0, errors.New("lxc-info output contains no pid")
}

func parseProxmoxLXCRunningContainers(output string) ([]proxmoxLXCRunningContainer, error) {
	if len(output) > proxmoxLXCMaxListOutputBytes {
		return nil, fmt.Errorf("pct list output exceeds %d bytes", proxmoxLXCMaxListOutputBytes)
	}

	result := make([]proxmoxLXCRunningContainer, 0)
	seen := make(map[int]struct{})
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSuffix(rawLine, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "VMID") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.EqualFold(fields[1], "running") {
			continue
		}
		vmid, err := strconv.Atoi(fields[0])
		if err != nil || vmid < 100 || vmid > 999999999 {
			continue
		}
		if _, exists := seen[vmid]; exists {
			continue
		}

		name := ""
		if len(line) > 35 {
			name = strings.TrimSpace(line[35:])
		} else if len(fields) >= 3 {
			name = fields[len(fields)-1]
		}
		if !safeProxmoxLXCText(name, proxmoxLXCMaxNameBytes) {
			continue
		}

		seen[vmid] = struct{}{}
		result = append(result, proxmoxLXCRunningContainer{VMID: vmid, Name: name})
		if len(result) == proxmoxLXCMaxContainers {
			break
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].VMID < result[j].VMID })
	return result, nil
}

func parseProxmoxLXCDF(output string) ([]agentshost.Disk, error) {
	if len(output) > proxmoxLXCMaxDFOutputBytes {
		return nil, fmt.Errorf("pct df output exceeds %d bytes", proxmoxLXCMaxDFOutputBytes)
	}

	result := make([]agentshost.Disk, 0)
	seen := make(map[string]struct{})
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if line == "" || strings.HasPrefix(line, "MP ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}

		diskType := strings.ToLower(strings.TrimSpace(fields[0]))
		if diskType != "rootfs" && !validProxmoxLXCMountKey(diskType) {
			continue
		}
		device := strings.TrimSpace(fields[1])
		if !safeProxmoxLXCText(device, proxmoxLXCMaxLabelBytes) {
			continue
		}
		total, totalOK := parseProxmoxLXCSize(fields[2])
		used, usedOK := parseProxmoxLXCSize(fields[3])
		free, freeOK := parseProxmoxLXCSize(fields[4])
		usage, usageOK := parseProxmoxLXCUsage(fields[5])
		if !totalOK || !usedOK || !freeOK || !usageOK || total <= 0 {
			continue
		}
		if used > total {
			used = total
		}
		if free > total {
			free = total
		}

		mountpoint := strings.TrimSpace(strings.Join(fields[6:], " "))
		if !validProxmoxLXCMountpoint(mountpoint) {
			continue
		}
		key := strings.ToLower(mountpoint)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		result = append(result, agentshost.Disk{
			Device:     device,
			Mountpoint: mountpoint,
			Type:       diskType,
			TotalBytes: total,
			UsedBytes:  used,
			FreeBytes:  free,
			Usage:      usage,
		})
		if len(result) == proxmoxLXCMaxDisks {
			break
		}
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Mountpoint == "/" {
			return true
		}
		if result[j].Mountpoint == "/" {
			return false
		}
		return result[i].Mountpoint < result[j].Mountpoint
	})
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func parseProxmoxLXCSize(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	multiplier := float64(1)
	last := value[len(value)-1]
	switch last {
	case 'K', 'k':
		multiplier = 1 << 10
	case 'M', 'm':
		multiplier = 1 << 20
	case 'G', 'g':
		multiplier = 1 << 30
	case 'T', 't':
		multiplier = 1 << 40
	default:
		last = 0
	}
	if last != 0 {
		value = strings.TrimSpace(value[:len(value)-1])
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 {
		return 0, false
	}
	bytes := parsed * multiplier
	if bytes > math.MaxInt64 {
		return 0, false
	}
	return int64(math.Round(bytes)), true
}

func parseProxmoxLXCUsage(value string) (float64, bool) {
	value = strings.TrimSuffix(strings.TrimSpace(value), "%")
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 || parsed > 100 {
		return 0, false
	}
	return parsed, true
}

func validProxmoxLXCMountKey(value string) bool {
	if !strings.HasPrefix(value, "mp") || len(value) < 3 || len(value) > 5 {
		return false
	}
	index, err := strconv.Atoi(value[2:])
	return err == nil && index >= 0 && index <= 255
}

func validProxmoxLXCMountpoint(value string) bool {
	if !safeProxmoxLXCText(value, proxmoxLXCMaxLabelBytes) || !strings.HasPrefix(value, "/") {
		return false
	}
	cleaned := path.Clean(value)
	return cleaned == value && !strings.Contains(value, "\x00")
}

func safeProxmoxLXCText(value string, maxBytes int) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxBytes || !utf8.ValidString(value) {
		return false
	}
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return false
		}
	}
	return true
}
