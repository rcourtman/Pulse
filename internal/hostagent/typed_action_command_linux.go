//go:build linux

package hostagent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const (
	typedActionUnitPrefix       = "pulse-agent-action-"
	typedActionUnitSuffix       = ".service"
	typedActionResultVersion    = 1
	typedActionSystemdStopGrace = "10s"
	typedActionRuntimeMaximum   = "20m"
)

var typedActionSystemdGate = make(chan struct{}, 1)

var (
	typedActionResolveSystemdTool = trustedSystemdTool
	typedActionCurrentExecutable  = os.Executable
	typedActionQueryUnitLoadState = typedActionUnitLoadState
	typedActionStopUnit           = requestTypedActionUnitStop
	typedActionResolveTarget      = resolveTypedActionTarget
	typedActionBecomeSubreaper    = becomeTypedActionSubreaper
	typedActionInspectDescendants = typedActionHasLiveDescendants
	typedActionWait4              = syscall.Wait4
)

type typedActionLauncherResult struct {
	Version             int    `json:"version"`
	TargetStarted       bool   `json:"target_started"`
	ExitCode            int    `json:"exit_code"`
	Signal              int    `json:"signal,omitempty"`
	ScanComplete        bool   `json:"scan_complete"`
	DescendantsObserved bool   `json:"descendants_observed"`
	Error               string `json:"error,omitempty"`
}

type typedActionWaitResult struct {
	err error
}

func runTypedActionCommandPlatform(ctx context.Context, env []string, catalog typedActionCatalog, name string, args ...string) typedActionCommandResult {
	select {
	case typedActionSystemdGate <- struct{}{}:
		defer func() { <-typedActionSystemdGate }()
	case <-ctx.Done():
		return typedActionCommandResult{exitCode: -1, err: ctx.Err()}
	}

	systemdRun, err := typedActionResolveSystemdTool("systemd-run")
	if err != nil {
		return typedActionCommandResult{exitCode: -1, err: err}
	}
	systemctl, err := typedActionResolveSystemdTool("systemctl")
	if err != nil {
		return typedActionCommandResult{exitCode: -1, err: err}
	}
	runnerPath, err := typedActionCurrentExecutable()
	if err != nil {
		return typedActionCommandResult{exitCode: -1, err: fmt.Errorf("resolve action-runner executable: %w", err)}
	}
	runnerPath, err = filepath.EvalSymlinks(runnerPath)
	if err != nil || !filepath.IsAbs(runnerPath) {
		return typedActionCommandResult{exitCode: -1, err: fmt.Errorf("resolve absolute action-runner executable: %w", err)}
	}
	if err := validateTrustedExecutable(runnerPath, uint32(os.Geteuid())); err != nil {
		return typedActionCommandResult{exitCode: -1, err: fmt.Errorf("validate action-runner executable: %w", err)}
	}
	unit, err := newTypedActionUnitName()
	if err != nil {
		return typedActionCommandResult{exitCode: -1, err: err}
	}
	resultPath, err := prepareTypedActionResultPath(unit)
	if err != nil {
		return typedActionCommandResult{exitCode: -1, err: err}
	}
	defer os.Remove(resultPath)

	commandArgs, err := typedActionSystemdRunArgs(unit, runnerPath, resultPath, env, catalog, name, args)
	if err != nil {
		return typedActionCommandResult{exitCode: -1, err: err}
	}
	cmd := exec.Command(systemdRun, commandArgs...)
	cmd.WaitDelay = 5 * time.Second
	stdout := newCappedBuffer(maxCommandOutputSize)
	stderr := newCappedBuffer(maxCommandOutputSize)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Start(); err != nil {
		return typedActionCommandResult{exitCode: -1, err: fmt.Errorf("start typed action transient service: %w", err)}
	}
	waited := make(chan typedActionWaitResult, 1)
	go func() { waited <- typedActionWaitResult{err: cmd.Wait()} }()

	var waitResult typedActionWaitResult
	collected := false
	select {
	case waitResult = <-waited:
	case <-ctx.Done():
		waitResult = waitForTypedActionUnitCollection(systemctl, unit, waited, true)
		collected = true
	}
	if !collected {
		waitResult = waitForTypedActionUnitCollection(systemctl, unit, completedTypedActionWait(waitResult), ctx.Err() != nil || waitResult.err != nil)
	}

	launcherResult, resultErr := readTypedActionLauncherResult(resultPath)
	result := typedActionCommandResult{stdout: stdout.String(), stderr: stderr.String(), exitCode: -1}
	if ctxErr := ctx.Err(); ctxErr != nil {
		result.err = errors.Join(errTypedActionContainmentIndeterminate, fmt.Errorf("typed action command interrupted after authoritative unit cleanup: %w", ctxErr))
		return result
	}
	if resultErr != nil {
		result.err = errors.Join(errTypedActionContainmentIndeterminate, waitResult.err, fmt.Errorf("typed action launcher result unavailable: %w", resultErr))
		return result
	}
	result.exitCode = launcherResult.ExitCode
	if !launcherResult.TargetStarted || !launcherResult.ScanComplete || launcherResult.Error != "" {
		result.err = errors.Join(errTypedActionContainmentIndeterminate, waitResult.err, fmt.Errorf("typed action launcher could not establish terminal state: %s", launcherResult.Error))
		result.exitCode = -1
		return result
	}
	if launcherResult.DescendantsObserved {
		result.err = errors.Join(errTypedActionContainmentIndeterminate, waitResult.err, errors.New("typed action direct process did not establish clean descendant completion"))
		result.exitCode = -1
		return result
	}
	if launcherResult.Signal != 0 {
		result.err = errors.Join(waitResult.err, fmt.Errorf("typed action terminated by signal %d", launcherResult.Signal))
		return result
	}
	if launcherResult.ExitCode != 0 {
		result.err = errors.Join(waitResult.err, fmt.Errorf("typed action exited with status %d", launcherResult.ExitCode))
		return result
	}
	if waitResult.err != nil {
		result.err = errors.Join(errTypedActionContainmentIndeterminate, fmt.Errorf("typed action supervisor failed: %w", waitResult.err))
		result.exitCode = -1
	}
	return result
}

func completedTypedActionWait(result typedActionWaitResult) <-chan typedActionWaitResult {
	ch := make(chan typedActionWaitResult, 1)
	ch <- result
	return ch
}

func waitForTypedActionUnitCollection(systemctl, unit string, waited <-chan typedActionWaitResult, requestStop bool) typedActionWaitResult {
	var result typedActionWaitResult
	waitComplete := false
	for {
		select {
		case result = <-waited:
			waitComplete = true
		default:
		}
		loadState, queryErr := typedActionQueryUnitLoadState(systemctl, unit)
		if queryErr == nil && loadState == "not-found" && waitComplete {
			return result
		}
		if requestStop || waitComplete {
			_ = typedActionStopUnit(systemctl, unit)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func typedActionUnitLoadState(systemctl, unit string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, systemctl, "--no-ask-password", "show", "--property=LoadState", "--value", unit).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("query transient unit: %w", err)
	}
	state := strings.TrimSpace(string(output))
	if state == "" {
		return "", errors.New("query transient unit returned no load state")
	}
	return state, nil
}

func requestTypedActionUnitStop(systemctl, unit string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, systemctl, "--no-ask-password", "--no-block", "stop", unit).Run()
}

func typedActionSystemdRunArgs(unit, runnerPath, resultPath string, env []string, catalog typedActionCatalog, name string, args []string) ([]string, error) {
	if err := validateTypedActionEnvironment(catalog, env); err != nil {
		return nil, err
	}
	restrictSUIDSGID := "yes"
	protectKernelModules := "yes"
	if catalog == typedActionCatalogPackage {
		// apt/dpkg must be able to install setuid/setgid files and kernel
		// package payloads. This relaxation remains inside the exact closed
		// package catalog and the systemd-owned transient service.
		restrictSUIDSGID = "no"
		protectKernelModules = "no"
	}
	result := []string{
		"--no-ask-password", "--quiet", "--wait", "--pipe", "--collect", "--service-type=exec", "--unit=" + unit,
		"--property=User=root", "--property=Group=root", "--property=UMask=0077", "--property=Restart=no",
		"--property=KillMode=control-group", "--property=KillSignal=SIGTERM", "--property=SendSIGKILL=yes", "--property=FinalKillSignal=SIGKILL",
		"--property=TimeoutStopSec=" + typedActionSystemdStopGrace, "--property=RuntimeMaxSec=" + typedActionRuntimeMaximum,
		"--property=BindsTo=pulse-agent-runner.service", "--property=After=pulse-agent-runner.service", "--property=PartOf=pulse-agent-runner.service",
		"--property=NoNewPrivileges=yes", "--property=PrivateTmp=yes", "--property=ProtectHome=yes", "--property=ProtectSystem=no",
		"--property=ProtectKernelTunables=yes", "--property=ProtectKernelModules=" + protectKernelModules, "--property=ProtectControlGroups=yes",
		"--property=LockPersonality=yes", "--property=RestrictSUIDSGID=" + restrictSUIDSGID, "--property=RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6",
		"--property=SystemCallArchitectures=native", "--property=WorkingDirectory=/",
		"--setenv=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "--setenv=LC_ALL=C",
	}
	for _, value := range env {
		result = append(result, "--setenv="+value)
	}
	result = append(result, "--", runnerPath, typedActionLauncherCommand, "--catalog", string(catalog), "--result-file", resultPath, "--", name)
	return append(result, args...), nil
}

func validateTypedActionEnvironment(catalog typedActionCatalog, env []string) error {
	if catalog != typedActionCatalogPackage && len(env) != 0 {
		return errors.New("typed action environment is not allowed for this catalog")
	}
	allowed := map[string]bool{"DEBIAN_FRONTEND=noninteractive": true, "APT_LISTCHANGES_FRONTEND=none": true, "NEEDRESTART_MODE=a": true}
	for _, value := range env {
		if !allowed[value] {
			return fmt.Errorf("typed package environment is outside the closed catalog")
		}
	}
	return nil
}

func newTypedActionUnitName() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", fmt.Errorf("generate typed action unit identity: %w", err)
	}
	return typedActionUnitPrefix + hex.EncodeToString(id[:]) + typedActionUnitSuffix, nil
}

func prepareTypedActionResultPath(unit string) (string, error) {
	stateDir := strings.TrimSpace(os.Getenv("PULSE_AGENT_RUNNER_STATE_DIR"))
	if !filepath.IsAbs(stateDir) {
		return "", errors.New("PULSE_AGENT_RUNNER_STATE_DIR must be absolute for typed action containment")
	}
	dir := filepath.Join(stateDir, "typed-actions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create typed action result directory: %w", err)
	}
	if err := validatePrivateDirectory(dir); err != nil {
		return "", err
	}
	resultPath := filepath.Join(dir, strings.TrimSuffix(unit, typedActionUnitSuffix)+".json")
	if _, err := os.Lstat(resultPath); err == nil {
		return "", errors.New("typed action result identity collision")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	return resultPath, nil
}

func validatePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || info.Mode().Perm()&0o077 != 0 || stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("typed action state directory must be a private directory owned by the runner")
	}
	return nil
}

func readTypedActionLauncherResult(path string) (typedActionLauncherResult, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return typedActionLauncherResult{}, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = unix.Close(fd)
		return typedActionLauncherResult{}, errors.New("open typed action result file")
	}
	defer file.Close()
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return typedActionLauncherResult{}, err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG || fs.FileMode(stat.Mode).Perm() != 0o600 || stat.Uid != uint32(os.Geteuid()) || stat.Size <= 0 || stat.Size > 4096 {
		return typedActionLauncherResult{}, errors.New("typed action result file failed ownership, mode, type, or size validation")
	}
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return typedActionLauncherResult{}, err
	}
	if len(data) == 0 || len(data) > 4096 {
		return typedActionLauncherResult{}, errors.New("typed action result file exceeded its bound")
	}
	var result typedActionLauncherResult
	if err := json.Unmarshal(data, &result); err != nil || result.Version != typedActionResultVersion {
		return typedActionLauncherResult{}, errors.New("typed action result file is malformed")
	}
	return result, nil
}

func trustedSystemdTool(name string) (string, error) {
	for _, candidate := range []string{filepath.Join("/usr/bin", name), filepath.Join("/bin", name)} {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err == nil && validateTrustedExecutable(resolved, 0) == nil {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("trusted %s is required for typed action containment", name)
}

func validateTrustedExecutable(path string, owner uint32) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode().Perm()&0o022 != 0 || info.Mode().Perm()&0o111 == 0 || stat.Uid != owner {
		return errors.New("executable must be a regular owner-controlled file")
	}
	return nil
}

func RunTypedActionLauncher(args []string) int {
	catalog, resultPath, name, commandArgs, err := parseTypedActionLauncherArgs(args)
	if err != nil {
		return typedActionLauncherSupervisorFailureExit
	}
	result := typedActionLauncherResult{Version: typedActionResultVersion, ExitCode: -1}
	if err := typedActionBecomeSubreaper(); err != nil {
		result.Error = "establish typed action child subreaper"
		_ = writeTypedActionLauncherResult(resultPath, result)
		return typedActionLauncherSupervisorFailureExit
	}
	target, err := typedActionResolveTarget(catalog, name)
	if err != nil {
		result.Error = err.Error()
		_ = writeTypedActionLauncherResult(resultPath, result)
		return typedActionLauncherStartFailureExit
	}
	cmd := exec.Command(target, commandArgs...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		result.Error = "start fixed-catalog target"
		_ = writeTypedActionLauncherResult(resultPath, result)
		return typedActionLauncherStartFailureExit
	}
	result.TargetStarted = true
	waitErr := cmd.Wait()
	result.ExitCode = 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			status, _ := exitErr.Sys().(syscall.WaitStatus)
			result.ExitCode = status.ExitStatus()
			if status.Signaled() {
				result.ExitCode = -1
				result.Signal = int(status.Signal())
			}
		} else {
			result.ExitCode = -1
			result.Error = "wait for fixed-catalog target"
		}
	}
	descendants, scanErr := typedActionInspectDescendants()
	if scanErr != nil {
		result.Error = "inspect transient service cgroup"
	} else {
		result.ScanComplete = true
		result.DescendantsObserved = descendants
	}
	if err := writeTypedActionLauncherResult(resultPath, result); err != nil {
		return typedActionLauncherSupervisorFailureExit
	}
	if scanErr != nil {
		return typedActionLauncherInspectionFailureExit
	}
	if descendants {
		return typedActionLauncherDescendantExit
	}
	if result.Signal != 0 || result.ExitCode < 0 {
		return typedActionLauncherSupervisorFailureExit
	}
	return result.ExitCode
}

func parseTypedActionLauncherArgs(args []string) (typedActionCatalog, string, string, []string, error) {
	if len(args) < 6 || args[0] != "--catalog" || args[2] != "--result-file" || args[4] != "--" {
		return "", "", "", nil, errors.New("invalid typed action launcher arguments")
	}
	catalog := typedActionCatalog(args[1])
	resultPath, name, commandArgs := args[3], args[5], args[6:]
	if !filepath.IsAbs(resultPath) || filepath.Base(resultPath) == "." {
		return "", "", "", nil, errors.New("invalid typed action result path")
	}
	if err := validateTypedActionInvocation(catalog, name, commandArgs); err != nil {
		return "", "", "", nil, err
	}
	return catalog, resultPath, name, commandArgs, nil
}

func resolveTypedActionTarget(catalog typedActionCatalog, name string) (string, error) {
	allowed := map[typedActionCatalog]map[string][]string{
		typedActionCatalogPackage: {"apt-get": {"/usr/bin/apt-get"}, "dpkg": {"/usr/bin/dpkg"}},
		typedActionCatalogProxmox: {"qm": {"/usr/sbin/qm", "/usr/bin/qm"}, "pct": {"/usr/sbin/pct", "/usr/bin/pct"}},
		typedActionCatalogProbe:   {"true": {"/usr/bin/true", "/bin/true"}},
	}
	for _, candidate := range allowed[catalog][name] {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err == nil && validateTrustedExecutable(resolved, 0) == nil {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("fixed-catalog executable %s is unavailable", name)
}

func becomeTypedActionSubreaper() error {
	return unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0)
}

func typedActionHasLiveDescendants() (bool, error) {
	failedAdoptedChild := false
	for {
		var status syscall.WaitStatus
		pid, err := typedActionWait4(-1, &status, syscall.WNOHANG, nil)
		switch {
		case err == nil && pid > 0:
			// Reap an adopted descendant that already exited, then establish
			// whether any live descendants remain. A failed adopted worker is
			// terminally indeterminate even after it has exited.
			if !status.Exited() || status.ExitStatus() != 0 {
				failedAdoptedChild = true
			}
			continue
		case err == nil && pid == 0:
			return true, nil
		case errors.Is(err, syscall.EINTR):
			continue
		case errors.Is(err, syscall.ECHILD):
			return failedAdoptedChild, nil
		default:
			return false, err
		}
	}
}

func writeTypedActionLauncherResult(path string, result typedActionLauncherResult) error {
	data, err := json.Marshal(result)
	if err != nil || len(data) > 4096 {
		return errors.New("encode typed action result")
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".typed-action-result-")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmpPath, path)
	}
	if err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	err = dir.Sync()
	closeErr = dir.Close()
	if err != nil {
		return err
	}
	return closeErr
}

var typedActionReconcileMu sync.Mutex

func ReconcileTypedActionUnits(ctx context.Context) error {
	typedActionReconcileMu.Lock()
	defer typedActionReconcileMu.Unlock()
	if _, err := typedActionResolveSystemdTool("systemd-run"); err != nil {
		return err
	}
	systemctl, err := typedActionResolveSystemdTool("systemctl")
	if err != nil {
		return err
	}
	output, err := exec.CommandContext(ctx, systemctl, "--no-ask-password", "list-units", "--all", "--full", "--plain", "--no-legend", typedActionUnitPrefix+"*"+typedActionUnitSuffix).CombinedOutput()
	if err != nil {
		return fmt.Errorf("enumerate stale typed action units: %w", err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || !validTypedActionUnitName(fields[0]) {
			continue
		}
		unit := fields[0]
		for {
			state, queryErr := typedActionQueryUnitLoadState(systemctl, unit)
			if queryErr == nil && state == "not-found" {
				break
			}
			_ = typedActionStopUnit(systemctl, unit)
			select {
			case <-ctx.Done():
				return fmt.Errorf("reconcile stale typed action unit %s: %w", unit, ctx.Err())
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	probe := runTypedActionCommand(ctx, nil, typedActionCatalogProbe, "true")
	if probe.err != nil || probe.exitCode != 0 {
		return fmt.Errorf("qualify typed action systemd containment: %w", probe.err)
	}
	return nil
}

func validTypedActionUnitName(unit string) bool {
	value := strings.TrimSuffix(strings.TrimPrefix(unit, typedActionUnitPrefix), typedActionUnitSuffix)
	if len(value) != 32 || typedActionUnitPrefix+value+typedActionUnitSuffix != unit {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
