package agentupdate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentidentity"
)

// versionOutput bounds both output streams and terminates an overproducing
// candidate immediately. The deadline also covers candidates that never exit.
type versionOutput struct {
	buffer   bytes.Buffer
	cancel   context.CancelFunc
	overflow bool
}

func (b *versionOutput) Len() int       { return b.buffer.Len() }
func (b *versionOutput) String() string { return b.buffer.String() }

func (b *versionOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 256 {
		b.overflow = true
		b.cancel()
		return 0, errors.New("executable version output exceeds 256 bytes")
	}
	return b.buffer.Write(p)
}

func (u *Updater) runIdentityProbe(ctx context.Context, path, flag string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	stdout, stderr := &versionOutput{cancel: cancel}, &versionOutput{cancel: cancel}
	cmd := execCommandContextFn(ctx, path, flag)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.WaitDelay = time.Second
	probeErr := cmd.Run()
	if stdout.overflow || stderr.overflow {
		return "", errors.New("invalid downloaded executable identity output")
	}
	if probeErr != nil {
		return "", fmt.Errorf("downloaded executable %s failed: %w (stderr %q)", flag, probeErr, stderr.String())
	}
	if stderr.Len() != 0 {
		return "", fmt.Errorf("invalid downloaded executable %s identity output (stderr %q)", flag, stderr.String())
	}
	output := strings.TrimSuffix(strings.TrimSuffix(stdout.String(), "\n"), "\r")
	if output != strings.TrimSpace(output) {
		return "", errors.New("unexpected whitespace in executable identity output")
	}
	return output, nil
}

func (u *Updater) verifyDownloadedIdentity(ctx context.Context, path, version, target string) error {
	if target == "" {
		return errors.New("missing expected executable release target")
	}
	expected, err := agentidentity.Version(version)
	if err != nil {
		return fmt.Errorf("invalid advertised executable version: %w", err)
	}
	if err := agentidentity.Validate(path, version, target); err != nil {
		return fmt.Errorf("downloaded executable identity: %w", err)
	}
	output, err := u.runIdentityProbe(ctx, path, "--version")
	if err != nil {
		return err
	}
	actual, err := agentidentity.Version(output)
	if err != nil || actual != expected {
		return fmt.Errorf("downloaded executable reports %q, expected %q", output, expected)
	}
	stamp, err := u.runIdentityProbe(ctx, path, "--release-identity")
	if err != nil {
		return err
	}
	stamped, err := agentidentity.ParseReleaseIdentity(stamp)
	if err != nil || stamped != expected {
		return fmt.Errorf("downloaded executable release identity does not match %s", expected)
	}
	return nil
}
