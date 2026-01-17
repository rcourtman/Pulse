package monitoring

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// DockerChecker provides the ability to check for Docker inside LXC containers.
// This is typically implemented by wrapping the agentexec.Server.
type DockerChecker interface {
	// CheckDockerInContainer checks if Docker is installed inside an LXC container.
	// Returns true if Docker socket exists, false otherwise.
	// The node parameter is the Proxmox node hostname where the container runs.
	CheckDockerInContainer(ctx context.Context, node string, vmid int) (bool, error)
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
