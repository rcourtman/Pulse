package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog/log"
)

const proxmoxGuestDockerInventoryMarker = "PULSE_DOCKER_GUEST_INVENTORY_V1"

// DockerChecker provides the ability to check for Docker inside LXC containers.
// This is typically implemented by wrapping the agentexec.Server.
type DockerChecker interface {
	// CheckDockerInContainer checks if Docker is installed inside an LXC container.
	// Returns true if Docker socket exists, false otherwise.
	// The node parameter is the Proxmox node hostname where the container runs.
	CheckDockerInContainer(ctx context.Context, node string, vmid int) (bool, error)
}

// DockerInventoryCollector provides explicitly opted-in Docker inventory from
// an LXC guest through the Proxmox node that owns it.
type DockerInventoryCollector interface {
	// CollectDockerInventory returns a Docker agent-compatible report for the
	// supplied Proxmox LXC container. The bool is false when collection was
	// intentionally skipped, for example because the container is outside the
	// configured VMID allowlist or no Docker runtime is present.
	CollectDockerInventory(ctx context.Context, container models.Container) (agentsdocker.Report, bool, error)
}

// containerDockerCheck represents a container that needs Docker checking
type containerDockerCheck struct {
	index     int
	container models.Container
	reason    string // "new", "restarted", "first_check"
}

// containerDockerResult holds the result of a Docker check
type containerDockerResult struct {
	index     int
	hasDocker bool
	checked   bool
	err       error
}

// CheckContainersForDocker checks Docker presence for containers that need it.
// This is called during container polling to detect Docker in:
// - New containers that are running (first time seen)
// - Containers that have restarted (uptime reset)
// - Running containers that have never been checked
//
// Checks are performed in parallel for efficiency.
// Returns the containers with updated Docker status.
func (m *Monitor) CheckContainersForDocker(ctx context.Context, containers []models.Container) []models.Container {
	m.mu.RLock()
	checker := m.dockerChecker
	m.mu.RUnlock()

	if checker == nil {
		return containers
	}

	// Get previous container state for comparison
	previousContainers := make(map[string]models.Container)
	for _, ct := range m.state.GetContainers() {
		previousContainers[ct.ID] = ct
	}

	// Identify containers that need Docker checking
	var needsCheck []containerDockerCheck
	for i, ct := range containers {
		if ct.Status != "running" {
			// Not running - preserve previous Docker status if any
			if prev, ok := previousContainers[ct.ID]; ok {
				containers[i].HasDocker = prev.HasDocker
				containers[i].DockerCheckedAt = prev.DockerCheckedAt
			}
			continue
		}

		// Check if this container needs Docker detection
		reason := m.containerNeedsDockerCheck(ct, previousContainers)
		if reason != "" {
			needsCheck = append(needsCheck, containerDockerCheck{
				index:     i,
				container: ct,
				reason:    reason,
			})
		} else {
			// Preserve previous Docker status
			if prev, ok := previousContainers[ct.ID]; ok {
				containers[i].HasDocker = prev.HasDocker
				containers[i].DockerCheckedAt = prev.DockerCheckedAt
			}
		}
	}

	if len(needsCheck) == 0 {
		return containers
	}

	// Check Docker in parallel
	results := m.checkDockerParallel(ctx, checker, needsCheck)

	// Apply results
	checkedCount := 0
	dockerCount := 0
	for _, result := range results {
		if result.checked {
			containers[result.index].HasDocker = result.hasDocker
			containers[result.index].DockerCheckedAt = time.Now()
			checkedCount++
			if result.hasDocker {
				dockerCount++
			}
		} else if result.err != nil {
			// Check failed - preserve previous status if available
			ct := containers[result.index]
			if prev, ok := previousContainers[ct.ID]; ok {
				containers[result.index].HasDocker = prev.HasDocker
				containers[result.index].DockerCheckedAt = prev.DockerCheckedAt
			}
		}
	}

	if checkedCount > 0 {
		log.Info().
			Int("checked", checkedCount).
			Int("with_docker", dockerCount).
			Int("total_candidates", len(needsCheck)).
			Msg("Docker detection completed for containers")
	}

	return containers
}

// containerNeedsDockerCheck determines if a running container needs Docker checking.
// Returns the reason for checking, or empty string if no check is needed.
func (m *Monitor) containerNeedsDockerCheck(ct models.Container, previousContainers map[string]models.Container) string {
	prev, existed := previousContainers[ct.ID]

	// New container - never seen before
	if !existed {
		return "new"
	}

	// Never been checked
	if prev.DockerCheckedAt.IsZero() {
		return "first_check"
	}

	// Container restarted - uptime is less than before
	// (This catches containers that were stopped and started again)
	if ct.Uptime < prev.Uptime && prev.Uptime > 0 {
		return "restarted"
	}

	// Container was previously stopped and is now running
	if prev.Status != "running" {
		return "started"
	}

	// No check needed - use cached value
	return ""
}

// checkDockerParallel checks Docker for multiple containers in parallel
func (m *Monitor) checkDockerParallel(ctx context.Context, checker DockerChecker, checks []containerDockerCheck) []containerDockerResult {
	results := make([]containerDockerResult, len(checks))

	// Use a semaphore to limit concurrent checks (avoid overwhelming the system)
	const maxConcurrent = 5
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for i, check := range checks {
		wg.Add(1)
		go func(idx int, chk containerDockerCheck) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[idx] = containerDockerResult{index: chk.index, err: ctx.Err()}
				return
			}

			// Perform the check
			hasDocker, err := checker.CheckDockerInContainer(ctx, chk.container.Node, chk.container.VMID)
			if err != nil {
				log.Debug().
					Str("container", chk.container.Name).
					Int("vmid", chk.container.VMID).
					Str("reason", chk.reason).
					Err(err).
					Msg("Failed to check Docker in container")
				results[idx] = containerDockerResult{index: chk.index, err: err}
				return
			}

			log.Debug().
				Str("container", chk.container.Name).
				Int("vmid", chk.container.VMID).
				Str("reason", chk.reason).
				Bool("has_docker", hasDocker).
				Msg("Docker check completed")

			results[idx] = containerDockerResult{
				index:     chk.index,
				hasDocker: hasDocker,
				checked:   true,
			}
		}(i, check)
	}

	wg.Wait()
	return results
}

// CollectProxmoxGuestDockerInventory collects Docker container inventory for
// running LXC guests that have already opted into Proxmox-side Docker detection
// and were confirmed to expose a Docker socket.
func (m *Monitor) CollectProxmoxGuestDockerInventory(ctx context.Context, containers []models.Container) {
	m.mu.RLock()
	collector := m.dockerInventoryCollector
	m.mu.RUnlock()

	if collector == nil {
		return
	}

	candidates := make([]models.Container, 0, len(containers))
	for _, ct := range containers {
		if ct.Status != "running" || !ct.HasDocker || ct.VMID <= 0 || strings.TrimSpace(ct.Node) == "" || ct.IsOCI {
			continue
		}
		if m.hasOnlineHostAgentForContainer(ct.ID) {
			log.Debug().
				Str("container", ct.Name).
				Str("containerID", ct.ID).
				Int("vmid", ct.VMID).
				Msg("Skipping Proxmox LXC Docker inventory because a guest-local host agent is linked")
			continue
		}
		candidates = append(candidates, ct)
	}

	if len(candidates) == 0 {
		return
	}

	const maxConcurrent = 3
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	collected := 0
	failed := 0
	skipped := 0

	for _, ct := range candidates {
		wg.Add(1)
		go func(container models.Container) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}

			report, ok, err := collector.CollectDockerInventory(ctx, container)
			if err != nil {
				log.Debug().
					Err(err).
					Str("container", container.Name).
					Int("vmid", container.VMID).
					Msg("Failed to collect Proxmox LXC Docker inventory")
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}
			if !ok {
				mu.Lock()
				skipped++
				mu.Unlock()
				return
			}

			if _, err := m.ApplyDockerReport(report, nil); err != nil {
				log.Warn().
					Err(err).
					Str("container", container.Name).
					Int("vmid", container.VMID).
					Msg("Failed to apply Proxmox LXC Docker inventory report")
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}

			mu.Lock()
			collected++
			mu.Unlock()
		}(ct)
	}

	wg.Wait()

	if collected > 0 || failed > 0 || skipped > 0 {
		log.Info().
			Int("collected", collected).
			Int("failed", failed).
			Int("skipped", skipped).
			Int("candidates", len(candidates)).
			Msg("Proxmox LXC Docker inventory collection completed")
	}
}

func (m *Monitor) hasOnlineHostAgentForContainer(containerID string) bool {
	containerID = strings.TrimSpace(containerID)
	if m == nil || m.state == nil || containerID == "" {
		return false
	}
	for _, host := range m.state.GetHosts() {
		if strings.TrimSpace(host.LinkedContainerID) == containerID && strings.EqualFold(strings.TrimSpace(host.Status), "online") {
			return true
		}
	}
	return false
}

// AgentDockerChecker implements DockerChecker using the agent execution system.
// This wraps command execution to check for Docker inside LXC containers.
type AgentDockerChecker struct {
	executeCommand func(ctx context.Context, hostname string, command string, timeout int) (string, int, error)
}

// NewAgentDockerChecker creates a new checker that uses agent command execution.
// The executeCommand function should execute a command on the given hostname and return
// (stdout, exitCode, error).
func NewAgentDockerChecker(executeCommand func(ctx context.Context, hostname string, command string, timeout int) (string, int, error)) *AgentDockerChecker {
	return &AgentDockerChecker{
		executeCommand: executeCommand,
	}
}

// CheckDockerInContainer checks if Docker is installed inside an LXC container
// by running `pct exec <vmid> -- test -S /var/run/docker.sock` on the Proxmox node.
func (c *AgentDockerChecker) CheckDockerInContainer(ctx context.Context, node string, vmid int) (bool, error) {
	if c.executeCommand == nil {
		return false, fmt.Errorf("no command executor configured")
	}

	// Check for Docker socket - this is the most reliable indicator
	// We use test -S which checks if the file exists and is a socket
	cmd := fmt.Sprintf("pct exec %d -- test -S /var/run/docker.sock && echo yes || echo no", vmid)

	stdout, exitCode, err := c.executeCommand(ctx, node, cmd, 10) // 10 second timeout
	if err != nil {
		return false, fmt.Errorf("command execution failed: %w", err)
	}

	// The test command itself might fail if container is not accessible
	// but our echo fallback should always give us output
	stdout = strings.TrimSpace(stdout)

	// If we got "yes", Docker socket exists
	if strings.Contains(stdout, "yes") {
		return true, nil
	}

	// If exit code is non-zero and we didn't get "no", the container might not be accessible
	if exitCode != 0 && !strings.Contains(stdout, "no") {
		return false, fmt.Errorf("container not accessible (exit code %d): %s", exitCode, stdout)
	}

	return false, nil
}

// AgentDockerInventoryCollector implements DockerInventoryCollector using the
// agent execution system. It intentionally gathers only Docker page inventory
// fields from LXC guests: container ID, name, image, state/status, ports, and
// aggregate docker stats. It does not call docker inspect and does not collect
// labels, environment, mounts, commands, files, or process details.
type AgentDockerInventoryCollector struct {
	executeCommand func(ctx context.Context, hostname string, command string, timeout int) (string, int, error)
	allowedVMIDs   map[int]struct{}
}

// AgentDockerInventoryCollectorOptions configures explicitly opted-in
// Proxmox-side LXC Docker inventory collection.
type AgentDockerInventoryCollectorOptions struct {
	// AllowedVMIDs limits inventory collection to specific Proxmox VMIDs. An
	// empty map means all running Docker-enabled LXC guests are eligible.
	AllowedVMIDs map[int]struct{}
}

// NewAgentDockerInventoryCollector creates a collector that uses agent command
// execution on Proxmox nodes to run a minimal read-only Docker inventory script
// inside LXC guests.
func NewAgentDockerInventoryCollector(
	executeCommand func(ctx context.Context, hostname string, command string, timeout int) (string, int, error),
	options AgentDockerInventoryCollectorOptions,
) *AgentDockerInventoryCollector {
	allowed := make(map[int]struct{}, len(options.AllowedVMIDs))
	for vmid := range options.AllowedVMIDs {
		if vmid > 0 {
			allowed[vmid] = struct{}{}
		}
	}
	return &AgentDockerInventoryCollector{
		executeCommand: executeCommand,
		allowedVMIDs:   allowed,
	}
}

// CollectDockerInventory collects minimal Docker inventory from an LXC guest.
func (c *AgentDockerInventoryCollector) CollectDockerInventory(ctx context.Context, container models.Container) (agentsdocker.Report, bool, error) {
	if c == nil || c.executeCommand == nil {
		return agentsdocker.Report{}, false, fmt.Errorf("no command executor configured")
	}
	if container.VMID <= 0 {
		return agentsdocker.Report{}, false, fmt.Errorf("container VMID is required")
	}
	if len(c.allowedVMIDs) > 0 {
		if _, ok := c.allowedVMIDs[container.VMID]; !ok {
			return agentsdocker.Report{}, false, nil
		}
	}

	command := buildProxmoxGuestDockerInventoryCommand(container.VMID)
	stdout, exitCode, err := c.executeCommand(ctx, container.Node, command, 20)
	if err != nil {
		return agentsdocker.Report{}, false, fmt.Errorf("command execution failed: %w", err)
	}
	if exitCode != 0 && !strings.Contains(stdout, proxmoxGuestDockerInventoryMarker) {
		return agentsdocker.Report{}, false, fmt.Errorf("container docker inventory failed (exit code %d): %s", exitCode, strings.TrimSpace(stdout))
	}

	report, ok, parseErr := parseProxmoxGuestDockerInventory(stdout, container, time.Now().UTC())
	if parseErr != nil {
		return agentsdocker.Report{}, false, parseErr
	}
	return report, ok, nil
}

func buildProxmoxGuestDockerInventoryCommand(vmid int) string {
	script := strings.Join([]string{
		fmt.Sprintf("printf '%s\\n'", proxmoxGuestDockerInventoryMarker),
		"hn=\"$(hostname -f 2>/dev/null || hostname 2>/dev/null || true)\"",
		"printf 'HOSTNAME\\t%s\\n' \"$hn\"",
		"un=\"$(uname -srm 2>/dev/null || true)\"",
		"printf 'UNAME\\t%s\\n' \"$un\"",
		"cpus=\"$(getconf _NPROCESSORS_ONLN 2>/dev/null || true)\"",
		"printf 'CPUS\\t%s\\n' \"$cpus\"",
		"awk '/MemTotal:/ {printf \"MEMTOTAL\\t%.0f\\n\", $2 * 1024}' /proc/meminfo 2>/dev/null || true",
		"if ! command -v docker >/dev/null 2>&1; then printf 'NO_DOCKER\\n'; exit 0; fi",
		"if ! test -S /var/run/docker.sock; then printf 'NO_DOCKER\\n'; exit 0; fi",
		"version=\"$(docker version --format '{{json .Server.Version}}' 2>/dev/null || true)\"",
		"if [ -n \"$version\" ]; then printf 'VERSION\\t%s\\n' \"$version\"; fi",
		"docker ps -a --no-trunc --format 'CONTAINER\\t{{json .ID}}\\t{{json .Names}}\\t{{json .Image}}\\t{{json .State}}\\t{{json .Status}}\\t{{json .Ports}}\\t{{json .RunningFor}}' 2>/dev/null || true",
		"ids=\"$(docker ps -aq --no-trunc 2>/dev/null)\"",
		"if [ -n \"$ids\" ]; then docker stats --no-stream --no-trunc --format 'STAT\\t{{json .ID}}\\t{{json .Name}}\\t{{json .CPUPerc}}\\t{{json .MemUsage}}\\t{{json .MemPerc}}\\t{{json .NetIO}}\\t{{json .BlockIO}}' $ids 2>/dev/null || true; fi",
	}, "\n")

	return fmt.Sprintf("pct exec %d -- sh -c %s", vmid, shellSingleQuote(script))
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func parseProxmoxGuestDockerInventory(output string, container models.Container, timestamp time.Time) (agentsdocker.Report, bool, error) {
	if !strings.Contains(output, proxmoxGuestDockerInventoryMarker) {
		return agentsdocker.Report{}, false, fmt.Errorf("docker inventory output missing marker")
	}

	hostInfo := agentsdocker.HostInfo{
		Hostname: strings.TrimSpace(container.Name),
		Name:     proxmoxGuestDockerDisplayName(container),
		Runtime:  "docker",
	}
	if hostInfo.Hostname == "" {
		hostInfo.Hostname = fmt.Sprintf("lxc-%d", container.VMID)
	}
	hostInfo.Name = proxmoxGuestDockerDisplayName(container)

	containers := make([]agentsdocker.Container, 0)
	containerIndex := make(map[string]int)
	noDocker := false

	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimRight(rawLine, "\r")
		if line == "" || line == proxmoxGuestDockerInventoryMarker {
			continue
		}
		if line == "NO_DOCKER" {
			noDocker = true
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) == 1 && strings.HasPrefix(line, "VERSION\\t") {
			fields = []string{"VERSION", strings.TrimPrefix(line, "VERSION\\t")}
		}
		switch fields[0] {
		case "HOSTNAME":
			if len(fields) >= 2 {
				if hostname := strings.TrimSpace(fields[1]); hostname != "" {
					hostInfo.Hostname = hostname
				}
			}
		case "UNAME":
			if len(fields) >= 2 {
				applyUnameToDockerHostInfo(fields[1], &hostInfo)
			}
		case "CPUS":
			if len(fields) >= 2 {
				if cpus, err := strconv.Atoi(strings.TrimSpace(fields[1])); err == nil && cpus > 0 {
					hostInfo.TotalCPU = cpus
				}
			}
		case "MEMTOTAL":
			if len(fields) >= 2 {
				if total, err := strconv.ParseInt(strings.TrimSpace(fields[1]), 10, 64); err == nil && total > 0 {
					hostInfo.TotalMemoryBytes = total
					hostInfo.Memory.TotalBytes = total
				}
			}
		case "VERSION":
			if len(fields) >= 2 {
				version := decodeInventoryJSONString(fields[1])
				hostInfo.RuntimeVersion = version
				hostInfo.DockerVersion = version
			}
		case "CONTAINER":
			payload, ok := parseDockerInventoryContainerLine(fields)
			if !ok {
				continue
			}
			containerIndex[payload.ID] = len(containers)
			if payload.Name != "" {
				containerIndex[payload.Name] = len(containers)
			}
			containers = append(containers, payload)
		case "STAT":
			stat := parseDockerInventoryStatLine(fields)
			if stat.id == "" && stat.name == "" {
				continue
			}
			idx, ok := findDockerInventoryContainerIndex(containerIndex, containers, stat.id, stat.name)
			if !ok {
				continue
			}
			applyDockerInventoryStat(&containers[idx], stat)
		}
	}

	if noDocker && len(containers) == 0 && hostInfo.DockerVersion == "" {
		return agentsdocker.Report{}, false, nil
	}
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	return agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              proxmoxGuestDockerAgentID(container),
			Type:            "unified",
			IntervalSeconds: 30,
		},
		Host:       hostInfo,
		Containers: containers,
		Timestamp:  timestamp,
	}, true, nil
}

func proxmoxGuestDockerAgentID(container models.Container) string {
	id := strings.TrimSpace(container.ID)
	if id == "" {
		id = fmt.Sprintf("%s:%s:%d", strings.TrimSpace(container.Instance), strings.TrimSpace(container.Node), container.VMID)
	}
	id = strings.Trim(id, ":")
	if id == "" {
		id = fmt.Sprintf("lxc:%d", container.VMID)
	}
	return "proxmox-lxc-docker:" + id
}

func proxmoxGuestDockerDisplayName(container models.Container) string {
	name := strings.TrimSpace(container.Name)
	if name == "" {
		name = fmt.Sprintf("LXC %d", container.VMID)
	}
	if container.VMID > 0 {
		return fmt.Sprintf("%s (LXC %d)", name, container.VMID)
	}
	return name
}

func applyUnameToDockerHostInfo(uname string, host *agentsdocker.HostInfo) {
	if host == nil {
		return
	}
	parts := strings.Fields(uname)
	if len(parts) == 0 {
		return
	}
	host.OS = strings.ToLower(parts[0])
	if len(parts) >= 2 {
		host.KernelVersion = parts[1]
	}
	if len(parts) >= 3 {
		host.Architecture = parts[len(parts)-1]
	}
}

func parseDockerInventoryContainerLine(fields []string) (agentsdocker.Container, bool) {
	if len(fields) < 8 {
		return agentsdocker.Container{}, false
	}
	id := strings.TrimSpace(decodeInventoryJSONString(fields[1]))
	name := strings.TrimSpace(decodeInventoryJSONString(fields[2]))
	if id == "" && name == "" {
		return agentsdocker.Container{}, false
	}
	state := strings.TrimSpace(decodeInventoryJSONString(fields[4]))
	status := strings.TrimSpace(decodeInventoryJSONString(fields[5]))
	if state == "" {
		state = dockerStateFromStatus(status)
	}
	if id == "" {
		id = name
	}

	return agentsdocker.Container{
		ID:            id,
		Name:          strings.TrimPrefix(name, "/"),
		Image:         strings.TrimSpace(decodeInventoryJSONString(fields[3])),
		State:         state,
		Status:        status,
		UptimeSeconds: parseDockerStatusUptime(status),
		Ports:         parseDockerPorts(decodeInventoryJSONString(fields[6])),
	}, true
}

type dockerInventoryStat struct {
	id           string
	name         string
	cpuPercent   float64
	memUsage     int64
	memLimit     int64
	memPercent   float64
	networkRX    uint64
	networkTX    uint64
	blockRead    uint64
	blockWrite   uint64
	hasNetworkIO bool
	hasBlockIO   bool
}

func parseDockerInventoryStatLine(fields []string) dockerInventoryStat {
	if len(fields) < 8 {
		return dockerInventoryStat{}
	}
	stat := dockerInventoryStat{
		id:         strings.TrimSpace(decodeInventoryJSONString(fields[1])),
		name:       strings.TrimSpace(decodeInventoryJSONString(fields[2])),
		cpuPercent: parsePercent(decodeInventoryJSONString(fields[3])),
		memPercent: parsePercent(decodeInventoryJSONString(fields[5])),
	}
	stat.memUsage, stat.memLimit = parseDockerSizePair(decodeInventoryJSONString(fields[4]))
	if rx, tx, ok := parseDockerUintPair(decodeInventoryJSONString(fields[6])); ok {
		stat.networkRX = rx
		stat.networkTX = tx
		stat.hasNetworkIO = true
	}
	if read, write, ok := parseDockerUintPair(decodeInventoryJSONString(fields[7])); ok {
		stat.blockRead = read
		stat.blockWrite = write
		stat.hasBlockIO = true
	}
	return stat
}

func applyDockerInventoryStat(container *agentsdocker.Container, stat dockerInventoryStat) {
	if container == nil {
		return
	}
	container.CPUPercent = stat.cpuPercent
	container.MemoryUsageBytes = stat.memUsage
	container.MemoryLimitBytes = stat.memLimit
	container.MemoryPercent = stat.memPercent
	if stat.hasNetworkIO {
		container.NetworkRXBytes = stat.networkRX
		container.NetworkTXBytes = stat.networkTX
	}
	if stat.hasBlockIO {
		container.BlockIO = &agentsdocker.ContainerBlockIO{
			ReadBytes:  stat.blockRead,
			WriteBytes: stat.blockWrite,
		}
	}
}

func findDockerInventoryContainerIndex(index map[string]int, containers []agentsdocker.Container, id, name string) (int, bool) {
	for _, key := range []string{strings.TrimSpace(id), strings.TrimSpace(name)} {
		if key == "" {
			continue
		}
		if idx, ok := index[key]; ok {
			return idx, true
		}
	}

	if id != "" {
		matched := -1
		for i, container := range containers {
			if strings.HasPrefix(container.ID, id) || strings.HasPrefix(id, container.ID) {
				if matched >= 0 {
					return 0, false
				}
				matched = i
			}
		}
		if matched >= 0 {
			return matched, true
		}
	}

	return 0, false
}

func decodeInventoryJSONString(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var value string
	if err := json.Unmarshal([]byte(raw), &value); err == nil {
		return value
	}
	return strings.Trim(raw, `"`)
}

func dockerStateFromStatus(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch {
	case strings.HasPrefix(normalized, "up "):
		return "running"
	case strings.HasPrefix(normalized, "exited"):
		return "exited"
	case strings.HasPrefix(normalized, "created"):
		return "created"
	default:
		return normalized
	}
}

func parseDockerStatusUptime(status string) int64 {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if !strings.HasPrefix(normalized, "up ") {
		return 0
	}
	normalized = strings.TrimSpace(strings.TrimPrefix(normalized, "up "))
	normalized = strings.TrimPrefix(normalized, "about ")
	if strings.HasPrefix(normalized, "less than a second") {
		return 1
	}
	if strings.HasPrefix(normalized, "an ") {
		normalized = "1 " + strings.TrimPrefix(normalized, "an ")
	}
	fields := strings.Fields(normalized)
	if len(fields) < 2 {
		return 0
	}
	amount, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil || amount < 0 {
		return 0
	}
	unit := strings.TrimSuffix(fields[1], "s")
	switch unit {
	case "second":
		return amount
	case "minute":
		return amount * 60
	case "hour":
		return amount * 60 * 60
	case "day":
		return amount * 24 * 60 * 60
	case "week":
		return amount * 7 * 24 * 60 * 60
	case "month":
		return amount * 30 * 24 * 60 * 60
	case "year":
		return amount * 365 * 24 * 60 * 60
	default:
		return 0
	}
}

func parsePercent(raw string) float64 {
	raw = strings.TrimSpace(strings.TrimSuffix(raw, "%"))
	if raw == "" || raw == "--" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

func parseDockerSizePair(raw string) (int64, int64) {
	left, right, ok := splitSlashPair(raw)
	if !ok {
		return 0, 0
	}
	used, _ := parseDockerByteSize(left)
	limit, _ := parseDockerByteSize(right)
	return used, limit
}

func parseDockerUintPair(raw string) (uint64, uint64, bool) {
	left, right, ok := splitSlashPair(raw)
	if !ok {
		return 0, 0, false
	}
	leftBytes, leftOK := parseDockerByteSize(left)
	rightBytes, rightOK := parseDockerByteSize(right)
	if !leftOK && !rightOK {
		return 0, 0, false
	}
	return uint64(maxInt64(leftBytes, 0)), uint64(maxInt64(rightBytes, 0)), true
}

func splitSlashPair(raw string) (string, string, bool) {
	parts := strings.Split(raw, "/")
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func parseDockerByteSize(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "--" {
		return 0, false
	}
	raw = strings.ReplaceAll(raw, " ", "")
	i := 0
	for i < len(raw) {
		ch := raw[i]
		if (ch >= '0' && ch <= '9') || ch == '.' {
			i++
			continue
		}
		break
	}
	if i == 0 {
		return 0, false
	}
	value, err := strconv.ParseFloat(raw[:i], 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	unit := strings.ToLower(strings.TrimSpace(raw[i:]))
	multiplier := float64(1)
	switch unit {
	case "", "b":
		multiplier = 1
	case "kb":
		multiplier = 1000
	case "kib":
		multiplier = 1024
	case "mb":
		multiplier = 1000 * 1000
	case "mib":
		multiplier = 1024 * 1024
	case "gb":
		multiplier = 1000 * 1000 * 1000
	case "gib":
		multiplier = 1024 * 1024 * 1024
	case "tb":
		multiplier = 1000 * 1000 * 1000 * 1000
	case "tib":
		multiplier = 1024 * 1024 * 1024 * 1024
	case "pb":
		multiplier = 1000 * 1000 * 1000 * 1000 * 1000
	case "pib":
		multiplier = 1024 * 1024 * 1024 * 1024 * 1024
	default:
		return 0, false
	}
	result := value * multiplier
	if result < 0 {
		return 0, false
	}
	if result > float64(math.MaxInt64) {
		return math.MaxInt64, true
	}
	return int64(math.Round(result)), true
}

func parseDockerPorts(raw string) []agentsdocker.ContainerPort {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	segments := strings.Split(raw, ",")
	ports := make([]agentsdocker.ContainerPort, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		if strings.Contains(segment, "->") {
			parts := strings.SplitN(segment, "->", 2)
			private, protocol, ok := parseDockerPortSpec(parts[1])
			if !ok {
				continue
			}
			ip, publicPort := parseDockerPublishedPort(parts[0])
			ports = append(ports, agentsdocker.ContainerPort{
				PrivatePort: private,
				PublicPort:  publicPort,
				Protocol:    protocol,
				IP:          ip,
			})
			continue
		}
		private, protocol, ok := parseDockerPortSpec(segment)
		if !ok {
			continue
		}
		ports = append(ports, agentsdocker.ContainerPort{
			PrivatePort: private,
			Protocol:    protocol,
		})
	}
	if len(ports) == 0 {
		return nil
	}
	return ports
}

func parseDockerPortSpec(raw string) (int, string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, "", false
	}
	protocol := "tcp"
	if before, after, ok := strings.Cut(raw, "/"); ok {
		raw = before
		if strings.TrimSpace(after) != "" {
			protocol = strings.TrimSpace(after)
		}
	}
	if before, _, ok := strings.Cut(raw, "-"); ok {
		raw = before
	}
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || port <= 0 {
		return 0, "", false
	}
	return port, protocol, true
}

func parseDockerPublishedPort(raw string) (string, int) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0
	}
	portPart := raw
	ip := ""
	if idx := strings.LastIndex(raw, ":"); idx >= 0 {
		ip = strings.Trim(raw[:idx], "[]")
		portPart = raw[idx+1:]
	}
	if before, _, ok := strings.Cut(portPart, "-"); ok {
		portPart = before
	}
	port, err := strconv.Atoi(strings.TrimSpace(portPart))
	if err != nil || port <= 0 {
		return ip, 0
	}
	return ip, port
}

// ParseProxmoxGuestDockerInventoryVMIDs parses a comma-separated VMID allowlist.
// Empty input means all running Docker-enabled LXC guests are eligible.
func ParseProxmoxGuestDockerInventoryVMIDs(raw string) (map[int]struct{}, []string) {
	allowed := make(map[int]struct{})
	invalid := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		vmid, err := strconv.Atoi(part)
		if err != nil || vmid <= 0 {
			invalid = append(invalid, part)
			continue
		}
		allowed[vmid] = struct{}{}
	}
	return allowed, invalid
}

// SetDockerChecker configures Docker detection for LXC containers.
// When set, Docker presence will be automatically detected during container polling
// for new containers and containers that have restarted.
func (m *Monitor) SetDockerChecker(checker DockerChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dockerChecker = checker
	if checker != nil {
		log.Info().Msg("Docker detection enabled for LXC containers")
	} else {
		log.Info().Msg("Docker detection disabled for LXC containers")
	}
}

// GetDockerChecker returns the current Docker checker, if configured.
func (m *Monitor) GetDockerChecker() DockerChecker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dockerChecker
}

// SetDockerInventoryCollector configures explicitly opted-in Docker inventory
// collection for Docker-enabled LXC containers.
func (m *Monitor) SetDockerInventoryCollector(collector DockerInventoryCollector) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dockerInventoryCollector = collector
	if collector != nil {
		log.Info().Msg("Docker inventory collection enabled for LXC containers")
	} else {
		log.Info().Msg("Docker inventory collection disabled for LXC containers")
	}
}

// GetDockerInventoryCollector returns the current Docker inventory collector,
// if configured.
func (m *Monitor) GetDockerInventoryCollector() DockerInventoryCollector {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dockerInventoryCollector
}
