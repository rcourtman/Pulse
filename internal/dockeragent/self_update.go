package dockeragent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

// checkForUpdates checks if a newer version is available and performs self-update if needed
func (a *Agent) checkForUpdates(ctx context.Context) {
	// Skip updates if disabled via config
	if a.cfg.DisableAutoUpdate {
		a.logger.Info().Msg("Skipping update check - auto-update disabled")
		return
	}

	// Skip updates in development mode to prevent update loops
	if Version == "dev" {
		a.logger.Debug().Msg("Skipping update check - running in development mode")
		return
	}

	a.logger.Debug().Msg("Checking for agent updates")

	target := a.primaryTarget()
	if target.URL == "" {
		a.logger.Debug().Msg("Skipping update check - no Pulse target configured")
		return
	}

	// Get current version from server
	url := fmt.Sprintf("%s/api/agent/version", target.URL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		a.logger.Warn().Err(err).Msg("Failed to create version check request")
		return
	}

	if target.Token != "" {
		req.Header.Set("X-API-Token", target.Token)
		req.Header.Set("Authorization", "Bearer "+target.Token)
	}

	client := a.httpClientFor(target)
	resp, err := client.Do(req)
	if err != nil {
		a.logger.Warn().Err(err).Msg("Failed to check for updates")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.logger.Warn().Int("status", resp.StatusCode).Msg("Version endpoint returned non-200 status")
		return
	}

	var versionResp struct {
		Version string `json:"version"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&versionResp); err != nil {
		a.logger.Warn().Err(err).Msg("Failed to decode version response")
		return
	}

	// Skip updates if server is also in development mode
	if versionResp.Version == "dev" {
		a.logger.Debug().Msg("Skipping update - server is in development mode")
		return
	}

	// Compare versions - normalize by stripping "v" prefix for comparison.
	// Server returns version without prefix (e.g., "4.33.1"), but agent's
	// Version may include it (e.g., "v4.33.1") depending on build.
	if utils.NormalizeVersion(versionResp.Version) == utils.NormalizeVersion(Version) {
		a.logger.Debug().Str("version", Version).Msg("Agent is up to date")
		return
	}

	a.logger.Info().
		Str("currentVersion", Version).
		Str("availableVersion", versionResp.Version).
		Msg("New agent version available, performing self-update")

	// Perform self-update
	if err := selfUpdateFunc(a, ctx); err != nil {
		a.logger.Error().Err(err).Msg("Failed to self-update agent")
		return
	}

	a.logger.Info().Msg("Agent updated successfully, restarting...")
}

// isUnraid checks if we're running on Unraid by looking for /etc/unraid-version
func isUnraid() bool {
	_, err := osStatFn(unraidVersionPath)
	return err == nil
}

// resolveSymlink resolves symlinks to get the real path of a file.
// This is needed for self-update because os.Rename() fails across filesystems.
func resolveSymlink(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

// verifyELFMagic checks that the file is a valid ELF binary
func verifyELFMagic(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return fmt.Errorf("failed to read magic bytes: %w", err)
	}

	// ELF magic: 0x7f 'E' 'L' 'F'
	if magic[0] == 0x7f && magic[1] == 'E' && magic[2] == 'L' && magic[3] == 'F' {
		return nil
	}
	return errors.New("not a valid ELF binary")
}

func determineSelfUpdateArch() string {
	switch goArch {
	case "amd64":
		return "linux-amd64"
	case "arm64":
		return "linux-arm64"
	case "arm":
		return "linux-armv7"
	}

	out, err := unameMachine()
	if err != nil {
		return ""
	}

	normalized := strings.ToLower(strings.TrimSpace(out))
	switch normalized {
	case "x86_64", "amd64":
		return "linux-amd64"
	case "aarch64", "arm64":
		return "linux-arm64"
	case "armv7l", "armhf", "armv7":
		return "linux-armv7"
	default:
		return ""
	}
}

// selfUpdate downloads the new agent binary and replaces the current one
func (a *Agent) selfUpdate(ctx context.Context) error {
	target := a.primaryTarget()
	if target.URL == "" {
		return errors.New("no Pulse target configured for self-update")
	}

	// Get path to current executable
	execPath, err := osExecutableFn()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks to get the real path for atomic rename
	// os.Rename() fails across filesystems, so we need the actual target path
	realExecPath, err := resolveSymlink(execPath)
	if err != nil {
		a.logger.Debug().Err(err).Str("path", execPath).Msg("Failed to resolve symlinks, using original path")
		realExecPath = execPath
	} else if realExecPath != execPath {
		a.logger.Debug().
			Str("symlink", execPath).
			Str("target", realExecPath).
			Msg("Resolved symlink for self-update")
	}

	downloadBase := strings.TrimRight(target.URL, "/") + "/download/pulse-docker-agent"
	archParam := determineSelfUpdateArch()

	type downloadCandidate struct {
		url  string
		arch string
	}

	candidates := make([]downloadCandidate, 0, 2)
	if archParam != "" {
		candidates = append(candidates, downloadCandidate{
			url:  fmt.Sprintf("%s?arch=%s", downloadBase, archParam),
			arch: archParam,
		})
	}
	candidates = append(candidates, downloadCandidate{url: downloadBase})

	client := a.httpClientFor(target)
	var resp *http.Response
	lastErr := errors.New("failed to download new binary")

	for _, candidate := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate.url, nil)
		if err != nil {
			lastErr = fmt.Errorf("failed to create download request: %w", err)
			continue
		}

		if target.Token != "" {
			req.Header.Set("X-API-Token", target.Token)
			req.Header.Set("Authorization", "Bearer "+target.Token)
		}

		response, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to download new binary: %w", err)
			continue
		}

		if response.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("download failed with status: %s", response.Status)
			response.Body.Close()
			continue
		}

		resp = response
		if candidate.arch != "" {
			a.logger.Debug().
				Str("arch", candidate.arch).
				Msg("Self-update: downloaded architecture-specific agent binary")
		} else if archParam != "" {
			a.logger.Debug().Msg("Self-update: falling back to server default agent binary")
		}
		break
	}

	if resp == nil {
		return lastErr
	}
	defer resp.Body.Close()

	checksumHeader := strings.TrimSpace(resp.Header.Get("X-Checksum-Sha256"))

	// Create temporary file in the same directory as the target binary
	// to ensure atomic rename works (os.Rename fails across filesystems)
	targetDir := filepath.Dir(realExecPath)
	tmpFile, err := osCreateTempFn(targetDir, "pulse-docker-agent-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file in %s: %w", targetDir, err)
	}
	tmpPath := tmpFile.Name()
	defer osRemoveFn(tmpPath) // Clean up if something goes wrong

	// Write downloaded binary to temp file with size limit (100 MB max)
	const maxBinarySize = 100 * 1024 * 1024
	hasher := sha256.New()
	limitedReader := io.LimitReader(resp.Body, maxBinarySize+1)
	written, err := io.Copy(tmpFile, io.TeeReader(limitedReader, hasher))
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write downloaded binary: %w", err)
	}
	if written > maxBinarySize {
		tmpFile.Close()
		return fmt.Errorf("downloaded binary exceeds maximum size (%d bytes)", maxBinarySize)
	}
	if err := closeFileFn(tmpFile); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Verify it's a valid ELF binary (basic sanity check for Linux)
	if err := verifyELFMagic(tmpPath); err != nil {
		return fmt.Errorf("downloaded file is not a valid executable: %w", err)
	}

	// Verify checksum (mandatory for security)
	downloadChecksum := hex.EncodeToString(hasher.Sum(nil))
	if checksumHeader == "" {
		return fmt.Errorf("server did not provide checksum header (X-Checksum-Sha256); refusing update for security")
	}

	expected := strings.ToLower(strings.TrimSpace(checksumHeader))
	actual := strings.ToLower(downloadChecksum)
	if expected != actual {
		return fmt.Errorf("checksum verification failed: expected %s, got %s", expected, actual)
	}
	a.logger.Debug().Str("checksum", downloadChecksum).Msg("Self-update: checksum verified")

	// Verify cryptographic signature (X-Signature-Ed25519 header)
	signatureHeader := strings.TrimSpace(resp.Header.Get("X-Signature-Ed25519"))
	if signatureHeader != "" {
		if err := verifyFileSignature(tmpPath, signatureHeader); err != nil {
			return fmt.Errorf("signature verification failed: %w", err)
		}
		a.logger.Info().Msg("Self-update: cryptographic signature verified")
	} else {
		// For now, only warn if missing. In strict mode, we would error here.
		a.logger.Warn().Msg("Self-update: server did not provide cryptographic signature (X-Signature-Ed25519)")
	}

	// Make temp file executable
	if err := osChmodFn(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to make temp file executable: %w", err)
	}

	// Pre-flight check: Run the new binary with --self-test
	// We use the same environment as the current process
	// We also need to supply minimal required flags if they are mandatory (like token)
	// However, we just grab current args and replace the executable path,
	// and force --self-test
	a.logger.Debug().Msg("Self-update: running pre-flight check on new binary...")

	// Construct args for self-test. We need a valid token source for config load to pass.
	// Since we are running on the same host, we can pass the token file or env.
	// Simplest approach: pass --self-test and --token=dummy (if validation requires it)
	// But our config loader checks token presence.
	// Let's use the actual token configured to be safe.
	checkCmd := execCommandContextFn(ctx, tmpPath, "--self-test", "--token", a.cfg.APIToken)
	if output, err := checkCmd.CombinedOutput(); err != nil {
		a.logger.Error().Str("output", string(output)).Err(err).Msg("Self-update: pre-flight check failed")
		return fmt.Errorf("new binary failed self-test: %w", err)
	}
	a.logger.Debug().Msg("Self-update: pre-flight check passed")

	// Create backup of current binary (use realExecPath for atomic operations)
	backupPath := realExecPath + ".backup"
	if err := osRenameFn(realExecPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	// Move new binary to current location
	if err := osRenameFn(tmpPath, realExecPath); err != nil {
		// Restore backup on failure
		_ = osRenameFn(backupPath, realExecPath)
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	// Remove backup on success
	_ = osRemoveFn(backupPath)

	// On Unraid, also update the persistent copy on the flash drive
	if isUnraid() {
		persistPath := unraidPersistPath
		if _, err := osStatFn(persistPath); err == nil {
			a.logger.Debug().Str("path", persistPath).Msg("Updating Unraid persistent binary")
			if newBinary, err := osReadFileFn(execPath); err == nil {
				tmpPersist := persistPath + ".tmp"
				if err := osWriteFileFn(tmpPersist, newBinary, 0644); err != nil {
					a.logger.Warn().Err(err).Msg("Failed to write Unraid persistent binary")
				} else if err := osRenameFn(tmpPersist, persistPath); err != nil {
					a.logger.Warn().Err(err).Msg("Failed to rename Unraid persistent binary")
					_ = osRemoveFn(tmpPersist)
				} else {
					a.logger.Info().Str("path", persistPath).Msg("Updated Unraid persistent binary")
				}
			}
		}
	}

	// Restart agent with same arguments
	args := os.Args
	env := os.Environ()

	if err := syscallExecFn(execPath, args, env); err != nil {
		return fmt.Errorf("failed to restart agent: %w", err)
	}

	return nil
}
