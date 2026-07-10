package updates

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SSHSIG verification constants for release artifacts.
//
// The release-build pipeline (scripts/build-release.sh, scripts/release_asset_common.sh)
// signs every published asset with ssh-keygen -Y sign using these identity
// and namespace values. The unattended timer (scripts/pulse-auto-update.sh)
// and the public bootstrap (scripts/install.sh, /install.sh) verify against
// the same pinned key. Anything that opens up a release artifact for
// installation must verify against the same trust root, otherwise the in-app
// path runs at a lower trust bar than the shell paths.
//
// If the signing key is rotated, all four locations need to move together:
//   - scripts/pulse-auto-update.sh PINNED_RELEASE_SSH_PUBLIC_KEY
//   - scripts/install.sh PINNED_INSTALLER_SSH_PUBLIC_KEY (template-filled)
//   - /install.sh PINNED_RELEASE_SSH_PUBLIC_KEY (public bootstrap)
//   - this file
const (
	releaseSignatureIdentity  = "pulse-installer"
	releaseSignatureNamespace = "pulse-install"
	pinnedReleaseSSHPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer"

	maxReleaseSignatureBytes int64 = 16 * 1024
	signatureFetchTimeout          = 30 * time.Second
)

// verifyReleaseSignatureFunc is the package-level signature verifier. Tests
// override it to avoid requiring ssh-keygen on the build host.
var verifyReleaseSignatureFunc = verifyReleaseSignatureExec

// verifyReleaseSignatureExec verifies that signaturePath is a valid SSHSIG
// over the contents of targetPath, signed by the pinned pulse-installer key.
// Shells out to ssh-keygen -Y verify so the in-app updater shares the exact
// trust path used by scripts/pulse-auto-update.sh and scripts/install.sh.
func verifyReleaseSignatureExec(ctx context.Context, targetPath, signaturePath string) error {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		return fmt.Errorf("ssh-keygen is required to verify release signatures: %w", err)
	}

	allowed, err := os.CreateTemp("", "pulse-allowed-signers-*")
	if err != nil {
		return fmt.Errorf("create allowed_signers file: %w", err)
	}
	allowedPath := allowed.Name()
	defer os.Remove(allowedPath)

	if _, err := fmt.Fprintf(allowed, "%s %s\n", releaseSignatureIdentity, pinnedReleaseSSHPublicKey); err != nil {
		allowed.Close()
		return fmt.Errorf("write allowed_signers file: %w", err)
	}
	if err := allowed.Close(); err != nil {
		return fmt.Errorf("close allowed_signers file: %w", err)
	}

	target, err := os.Open(targetPath)
	if err != nil {
		return fmt.Errorf("open signed payload %q: %w", targetPath, err)
	}
	defer target.Close()

	cmd := exec.CommandContext(ctx, "ssh-keygen",
		"-Y", "verify",
		"-f", allowedPath,
		"-I", releaseSignatureIdentity,
		"-n", releaseSignatureNamespace,
		"-s", signaturePath,
	)
	cmd.Stdin = target
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh-keygen verify: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// signatureURLFor returns the .sshsig sidecar URL for a given release-asset URL.
func signatureURLFor(assetURL *url.URL) *url.URL {
	sigURL := *assetURL
	sigURL.Path = assetURL.Path + ".sshsig"
	sigURL.RawPath = ""
	return &sigURL
}

// downloadAndVerifyReleaseSignature fetches the .sshsig sidecar for assetURL
// and verifies it against assetPath, fail-closed. The signature is downloaded
// to a temp file in the same directory as assetPath and removed before
// returning. If ssh-keygen is missing, the signature endpoint is unreachable,
// or verification fails, the caller must abort the update.
func (m *Manager) downloadAndVerifyReleaseSignature(ctx context.Context, assetURL *url.URL, assetPath string) error {
	sigURL := signatureURLFor(assetURL)

	client := &http.Client{Timeout: signatureFetchTimeout}
	resp, err := m.getWithRetry(ctx, client, sigURL, nil, "download release signature")
	if err != nil {
		return fmt.Errorf("fetch %s: %w", sigURL.String(), err)
	}
	defer resp.Body.Close()

	return readSignatureAndVerify(ctx, resp, sigURL.String(), assetPath)
}

func readSignatureAndVerify(ctx context.Context, resp *http.Response, sigURL string, assetPath string) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("signature URL %s returned status %d", sigURL, resp.StatusCode)
	}

	sigPath := assetPath + ".sshsig"
	sigFile, err := os.OpenFile(sigPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create signature file: %w", err)
	}
	defer os.Remove(sigPath)

	written, err := io.Copy(sigFile, io.LimitReader(resp.Body, maxReleaseSignatureBytes+1))
	closeErr := sigFile.Close()
	if err != nil {
		return fmt.Errorf("read signature body: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close signature file: %w", closeErr)
	}
	if written > maxReleaseSignatureBytes {
		return fmt.Errorf("signature %s exceeds %d bytes", filepath.Base(sigPath), maxReleaseSignatureBytes)
	}
	if written == 0 {
		return fmt.Errorf("signature %s was empty", filepath.Base(sigPath))
	}

	if err := verifyReleaseSignatureFunc(ctx, assetPath, sigPath); err != nil {
		return fmt.Errorf("verify signature for %s: %w", filepath.Base(assetPath), err)
	}
	return nil
}
