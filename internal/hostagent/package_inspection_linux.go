//go:build linux

package hostagent

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"
)

const packageInspectionTimeout = 20 * time.Second

var packageInspectionResolveAPT = func() (string, error) {
	return resolveTypedActionTarget(typedActionCatalogPackage, "apt-get")
}

// runPackageInspectionCommand is a local monitoring read, not a command
// executor. It accepts no executable, arguments or environment from callers,
// and runs at the collector's existing uid without a runner or privilege gain.
// All action preflight, mutation and verification keep typed containment.
func runPackageInspectionCommand(ctx context.Context) packageUpdateCommandResult {
	ctx, cancel := context.WithTimeout(ctx, packageInspectionTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return packageUpdateCommandResult{exitCode: -1, err: err}
	}
	path, err := packageInspectionResolveAPT()
	if err != nil {
		return packageUpdateCommandResult{exitCode: -1, err: err}
	}
	cmd := exec.CommandContext(ctx, path, "-s", "-o", "Debug::NoLocking=1", "upgrade")
	// In particular, do not inherit APT_CONFIG, loader variables, locale or
	// the agent's credentials. Resolve apt-get from the trusted fixed catalog,
	// not this PATH; the latter is only for APT's own fixed helper programs.
	cmd.Env = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "LC_ALL=C"}
	cmd.Dir = "/"
	configureCommandProcessGroup(cmd)
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
	cmd.WaitDelay = time.Second
	stdout, stderr := newCappedBuffer(maxCommandOutputSize), newCappedBuffer(maxCommandOutputSize)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err = cmd.Run()
	result := packageUpdateCommandResult{stdout: stdout.String(), stderr: stderr.String(), exitCode: -1, err: err}
	if cmd.ProcessState != nil {
		result.exitCode = cmd.ProcessState.ExitCode()
	}
	// Partial output is not an inventory. Never hash/count a truncated or
	// canceled simulation as a successful zero or a smaller pending count.
	if stdout.truncated || stderr.truncated {
		result.err = errors.Join(result.err, errors.New("package inspection output exceeded its limit"))
	}
	if ctx.Err() != nil {
		result.err = errors.Join(result.err, ctx.Err())
	}
	return result
}
