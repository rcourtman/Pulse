package hostagent

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

const (
	packageUpdateCacheTTL    = 30 * time.Minute
	packageUpdateMaxPackages = 200
)

var (
	packageUpdateCommandContext = exec.CommandContext
	packageUpdateLookPath       = exec.LookPath
	packageUpdateStat           = os.Stat
)

// packageUpdateManager owns both host package telemetry and the only command
// catalog allowed to mutate OS packages. Callers select a typed operation;
// package-manager commands and flags never cross the network boundary.
type packageUpdateManager struct {
	platform string
	now      func() time.Time
	cacheTTL time.Duration
	run      func(context.Context, []string, string, ...string) packageUpdateCommandResult
	lookPath func(string) (string, error)
	stat     func(string) (os.FileInfo, error)
	lease    *packageManagerLease

	mu     sync.Mutex
	cached *agentexec.HostPackageUpdateSnapshot
}

func newPackageUpdateManager(platform string, lease *packageManagerLease) *packageUpdateManager {
	return &packageUpdateManager{
		platform: strings.ToLower(strings.TrimSpace(platform)),
		now:      time.Now,
		cacheTTL: packageUpdateCacheTTL,
		run:      runPackageUpdateCommand,
		lookPath: packageUpdateLookPath,
		stat:     packageUpdateStat,
		lease:    lease,
	}
}

func (m *packageUpdateManager) Snapshot(ctx context.Context, force bool) agentexec.HostPackageUpdateSnapshot {
	if m == nil {
		return agentexec.HostPackageUpdateSnapshot{}
	}
	release, err := m.lease.acquire(ctx)
	if err != nil {
		return agentexec.HostPackageUpdateSnapshot{CheckedAt: m.currentTime(), Error: "package update inspection canceled"}
	}
	defer release()
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshotLocked(ctx, force)
}

func (m *packageUpdateManager) snapshotLocked(ctx context.Context, force bool) agentexec.HostPackageUpdateSnapshot {
	now := m.currentTime()
	if !force && m.cached != nil && now.Sub(m.cached.CheckedAt) < m.cacheTTL {
		return cloneHostPackageUpdateSnapshot(*m.cached)
	}

	snapshot := agentexec.HostPackageUpdateSnapshot{CheckedAt: now}
	if !supportsAPTPlatform(m.platform) {
		m.storeSnapshot(snapshot)
		return snapshot
	}
	if _, err := m.lookPath("apt-get"); err != nil {
		m.storeSnapshot(snapshot)
		return snapshot
	}

	snapshot.Supported = true
	snapshot.Manager = "apt"
	result := m.run(ctx, nil, "apt-get", "-s", "-o", "Debug::NoLocking=1", "upgrade")
	if result.err != nil || result.exitCode != 0 {
		snapshot.Error = "package update inspection failed"
		snapshot.RebootRequired = m.hostRebootRequired()
		m.storeSnapshot(snapshot)
		return snapshot
	}

	snapshot.Packages = parseAPTSimulatedUpgrades(result.stdout)
	snapshot.PendingCount = countAPTSimulatedUpgrades(result.stdout)
	snapshot.InventoryHash = aptUpgradeInventoryHash(result.stdout)
	snapshot.RebootRequired = m.hostRebootRequired()
	m.storeSnapshot(snapshot)
	return cloneHostPackageUpdateSnapshot(snapshot)
}

func (m *packageUpdateManager) Apply(ctx context.Context, req agentexec.HostUpdatePayload) (result agentexec.HostUpdateResultPayload) {
	startedAt := time.Now()
	result = agentexec.HostUpdateResultPayload{
		RequestID: strings.TrimSpace(req.RequestID), ActionID: strings.TrimSpace(req.ActionID),
		ExecutionPhase: agentexec.HostUpdatePhasePreflight,
	}
	defer func() { result.Duration = time.Since(startedAt).Milliseconds() }()

	if strings.TrimSpace(req.Operation) != agentexec.HostUpdateOperationInstall {
		result.Verification = agentexec.HostUpdateVerificationInconclusive
		result.Error = "unsupported typed host update operation"
		return result
	}
	release, err := m.lease.acquire(ctx)
	if err != nil {
		result.Verification = agentexec.HostUpdateVerificationInconclusive
		result.Error = "package manager lease unavailable"
		return result
	}
	defer release()

	m.mu.Lock()
	defer m.mu.Unlock()

	probe := m.snapshotLocked(ctx, true)
	if !probe.Supported {
		result.Before = probe
		result.After = probe
		result.Verification = agentexec.HostUpdateVerificationInconclusive
		result.Error = "host package updates are not supported by this agent"
		return result
	}

	result.ExecutionPhase = agentexec.HostUpdatePhaseRefresh
	refresh := m.run(ctx, nil, "apt-get", "update")
	if refresh.err != nil || refresh.exitCode != 0 {
		result.Before = probe
		result.After = probe
		result.HealthChecked, result.PackageManagerHealthy = m.checkPackageManagerHealth(ctx)
		result.Verification = agentexec.HostUpdateVerificationInconclusive
		result.Error = "package index refresh failed"
		return result
	}

	before := m.snapshotLocked(ctx, true)
	result.Before = before
	if before.Error != "" {
		result.After = before
		result.HealthChecked, result.PackageManagerHealthy = m.checkPackageManagerHealth(ctx)
		result.Verification = agentexec.HostUpdateVerificationInconclusive
		result.Error = "package update preflight failed"
		return result
	}
	if before.InventoryHash != strings.TrimSpace(req.ExpectedInventoryHash) {
		result.After = before
		result.HealthChecked, result.PackageManagerHealthy = m.checkPackageManagerHealth(ctx)
		result.Verification = agentexec.HostUpdateVerificationInconclusive
		result.Error = "package update inventory changed; replan required"
		return result
	}
	if before.PendingCount == 0 {
		result.After = before
		result.HealthChecked, result.PackageManagerHealthy = m.checkPackageManagerHealth(ctx)
		if !result.HealthChecked || !result.PackageManagerHealthy {
			result.Verification = agentexec.HostUpdateVerificationInconclusive
			result.Error = "package manager health could not be established"
			return result
		}
		result.Success = true
		result.ExecutionPhase = agentexec.HostUpdatePhaseComplete
		result.Verification = agentexec.HostUpdateVerificationVerified
		return result
	}
	result.HealthChecked, result.PackageManagerHealthy = m.checkPackageManagerHealth(ctx)
	if !result.HealthChecked || !result.PackageManagerHealthy {
		result.After = before
		result.ExecutionPhase = agentexec.HostUpdatePhasePreflight
		result.Verification = agentexec.HostUpdateVerificationInconclusive
		result.Error = "package manager health check refused installation"
		return result
	}

	env := []string{
		"DEBIAN_FRONTEND=noninteractive",
		"APT_LISTCHANGES_FRONTEND=none",
		"NEEDRESTART_MODE=a",
	}
	result.ExecutionPhase = agentexec.HostUpdatePhaseInstall
	result.MutationStarted = true
	install := m.run(ctx, env, "apt-get", "-y", "--no-remove", "-o", "Dpkg::Options::=--force-confold", "upgrade")
	if install.err != nil || install.exitCode != 0 {
		after := m.snapshotLocked(ctx, true)
		result.After = after
		result.HealthChecked, result.PackageManagerHealthy = m.checkPackageManagerHealth(ctx)
		result.RecoveryRequired = true
		result.Verification = agentexec.HostUpdateVerificationFailed
		result.Error = "package installation failed"
		return result
	}

	result.Success = true
	result.ExecutionPhase = agentexec.HostUpdatePhaseVerify
	after := m.snapshotLocked(ctx, true)
	result.After = after
	if after.Error != "" {
		result.HealthChecked, result.PackageManagerHealthy = m.checkPackageManagerHealth(ctx)
		result.RecoveryRequired = true
		result.Verification = agentexec.HostUpdateVerificationInconclusive
		result.Error = "package installation completed but verification was inconclusive"
		return result
	}
	if after.PendingCount != 0 {
		result.HealthChecked, result.PackageManagerHealthy = m.checkPackageManagerHealth(ctx)
		result.RecoveryRequired = true
		result.Verification = agentexec.HostUpdateVerificationFailed
		result.Error = "package installation completed but updates remain pending"
		return result
	}
	result.HealthChecked, result.PackageManagerHealthy = m.checkPackageManagerHealth(ctx)
	if !result.HealthChecked || !result.PackageManagerHealthy {
		result.Verification = agentexec.HostUpdateVerificationInconclusive
		result.RecoveryRequired = true
		result.Error = "package installation completed but package manager health was not established"
		return result
	}
	result.Verification = agentexec.HostUpdateVerificationVerified
	result.ExecutionPhase = agentexec.HostUpdatePhaseComplete
	return result
}

func (m *packageUpdateManager) checkPackageManagerHealth(ctx context.Context) (checked, healthy bool) {
	if _, err := m.lookPath("dpkg"); err != nil {
		return false, false
	}
	result := m.run(ctx, nil, "dpkg", "--audit")
	if result.err != nil && result.exitCode < 0 {
		return false, false
	}
	return true, result.err == nil && result.exitCode == 0 && strings.TrimSpace(result.stdout) == ""
}

type packageUpdateCommandResult struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

func runPackageUpdateCommand(ctx context.Context, env []string, name string, args ...string) packageUpdateCommandResult {
	cmd := packageUpdateCommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	stdout := newCappedBuffer(maxCommandOutputSize)
	stderr := newCappedBuffer(maxCommandOutputSize)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	result := packageUpdateCommandResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
	if err == nil {
		return result
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.exitCode = exitErr.ExitCode()
	} else {
		result.exitCode = -1
	}
	return result
}

func parseAPTSimulatedUpgrades(output string) []agentexec.HostPackageUpdate {
	updates := make([]agentexec.HostPackageUpdate, 0)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() && len(updates) < packageUpdateMaxPackages {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "Inst ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimSpace(fields[1])
		if name == "" {
			continue
		}
		updates = append(updates, agentexec.HostPackageUpdate{
			Name:             name,
			InstalledVersion: delimitedValue(line, "[", "]"),
			AvailableVersion: firstParenthesizedField(line),
		})
	}
	return updates
}

func countAPTSimulatedUpgrades(output string) int {
	count := 0
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Inst ") && len(strings.Fields(line)) >= 2 {
			count++
		}
	}
	return count
}

func aptUpgradeInventoryHash(output string) string {
	lines := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Inst ") && len(strings.Fields(line)) >= 2 {
			lines = append(lines, line)
		}
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func delimitedValue(value, start, end string) string {
	startIndex := strings.Index(value, start)
	if startIndex < 0 {
		return ""
	}
	remainder := value[startIndex+len(start):]
	endIndex := strings.Index(remainder, end)
	if endIndex < 0 {
		return ""
	}
	return strings.TrimSpace(remainder[:endIndex])
}

func firstParenthesizedField(value string) string {
	start := strings.Index(value, "(")
	if start < 0 {
		return ""
	}
	remainder := strings.TrimSpace(value[start+1:])
	if end := strings.IndexAny(remainder, " )"); end >= 0 {
		return strings.TrimSpace(remainder[:end])
	}
	return strings.TrimSuffix(remainder, ")")
}

func (m *packageUpdateManager) hostRebootRequired() bool {
	if m == nil || m.stat == nil {
		return false
	}
	_, err := m.stat("/var/run/reboot-required")
	return err == nil
}

func (m *packageUpdateManager) currentTime() time.Time {
	if m != nil && m.now != nil {
		return m.now().UTC()
	}
	return time.Now().UTC()
}

func (m *packageUpdateManager) storeSnapshot(snapshot agentexec.HostPackageUpdateSnapshot) {
	copy := cloneHostPackageUpdateSnapshot(snapshot)
	m.cached = &copy
}

func cloneHostPackageUpdateSnapshot(snapshot agentexec.HostPackageUpdateSnapshot) agentexec.HostPackageUpdateSnapshot {
	snapshot.Packages = append([]agentexec.HostPackageUpdate(nil), snapshot.Packages...)
	return snapshot
}
