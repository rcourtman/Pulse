//go:build linux

package hostagent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const localGuestExecQM = "/usr/sbin/qm"

func readLocalGuestExecConfig(ctx context.Context, vmid string) ([]byte, error) {
	// qm can read a remote node's cluster config too. Require the VM to be
	// owned by this node before AND after the non-QGA config query.
	configPath := filepath.Join("/etc/pve/local/qemu-server", vmid+".conf")
	isLocal := func() bool {
		info, err := os.Lstat(configPath)
		return err == nil && info.Mode().IsRegular()
	}
	if !isLocal() {
		return nil, errors.New("local guest config unavailable")
	}
	resolved, err := filepath.EvalSymlinks(localGuestExecQM)
	if err != nil || validateTrustedExecutable(resolved, 0) != nil {
		return nil, errors.New("trusted qm unavailable")
	}
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(readCtx, resolved, "config", vmid, "--current")
	configureCommandProcessGroup(cmd)
	cmd.WaitDelay = time.Second
	stdout, stderr := newCappedBuffer(guestExecConfigLimit), newCappedBuffer(1024)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil || stdout.truncated || readCtx.Err() != nil || !isLocal() {
		return nil, errors.New("local guest config unverified")
	}
	return []byte(stdout.String()), nil
}
