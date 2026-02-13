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
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

const (
	// selfUpdateRequestMaxAttempts bounds retries for transient update HTTP failures.
	selfUpdateRequestMaxAttempts = 3

	// selfUpdateRetryBaseDelay is the initial retry backoff delay.
	selfUpdateRetryBaseDelay = 25 * time.Millisecond

	// selfUpdateRetryMaxDelay caps retry backoff growth.
	selfUpdateRetryMaxDelay = 250 * time.Millisecond
)

var selfUpdateRetrySleepFn = selfUpdateSleepWithContext

// checkForUpdates checks if a newer version is available and performs self-update if needed
func (a *Agent) checkForUpdates(ctx context.Context) {
	if !a.tryStartUpdateCheck() {
		a.logger.Debug().Msg("Skipping update check - previous check still running")
		return
	}
	defer a.finishUpdateCheck()

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

	// Skip updates if running as part of the unified agent
	if a.cfg.AgentType == "unified" {
		a.logger.Debug().Msg("Skipping update check - running in unified agent mode")
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
	resp, err := a.doSelfUpdateGetWithRetry(ctx, target, url, "version check")
	if err != nil {
		a.logger.Warn().
			Err(err).
			Str("target", target.URL).
			Str("url", url).
			Msg("Failed to check for updates")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.logger.Warn().
			Int("status", resp.StatusCode).
			Str("target", target.URL).
			Str("url", url).
			Msg("Version endpoint returned non-200 status")
		return
	}

	var versionResp struct {
		Version string `json:"version"`
	}

	body, err := readBodyWithLimit(resp.Body, maxVersionResponseBodyBytes)
	if err != nil {
		a.logger.Warn().Err(err).Msg("Version response too large")
		return
	}

	if err := json.Unmarshal(body, &versionResp); err != nil {
		a.logger.Warn().Err(err).Msg("Failed to decode version response")
		return
	}

	// Skip updates if server is also in development mode
	if versionResp.Version == "dev" {
		a.logger.Debug().Msg("Skipping update - server is in development mode")
		return
	}

	// Compare versions with semver ordering to prevent accidental downgrades.
	currentNorm := utils.NormalizeVersion(Version)
	serverNorm := utils.NormalizeVersion(versionResp.Version)
	cmp := utils.CompareVersions(serverNorm, currentNorm)
	if cmp == 0 {
		a.logger.Debug().Str("version", Version).Msg("Agent is up to date")
		return
	}
	if cmp < 0 {
		a.logger.Debug().
			Str("currentVersion", currentNorm).
			Str("serverVersion", serverNorm).
			Msg("Server has older version, skipping downgrade")
		return
	}

	a.logger.Info().
		Str("currentVersion", Version).
		Str("availableVersion", versionResp.Version).
		Msg("New agent version available, performing self-update")

	// Perform self-update
	if err := selfUpdateFunc(a, ctx); err != nil {
		a.logger.Error().
			Err(err).
			Str("target", target.URL).
			Str("currentVersion", Version).
			Str("availableVersion", versionResp.Version).
			Msg("Failed to self-update agent")
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
func verifyELFMagic(path string) (retErr error) {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file for ELF verification: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			if retErr != nil {
				retErr = errors.Join(retErr, fmt.Errorf("close file after ELF verification: %w", closeErr))
			} else {
				retErr = fmt.Errorf("close file after ELF verification: %w", closeErr)
			}
		}
	}()

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

func selfUpdateSleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func selfUpdateRetryBackoffDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return selfUpdateRetryBaseDelay
	}

	delay := selfUpdateRetryBaseDelay * time.Duration(1<<(attempt-1))
	if delay > selfUpdateRetryMaxDelay {
		return selfUpdateRetryMaxDelay
	}
	return delay
}

func isRetryableSelfUpdateStatus(code int) bool {
	switch code {
	case http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func newAuthedSelfUpdateGetRequest(ctx context.Context, target TargetConfig, requestURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	if target.Token != "" {
		req.Header.Set("X-API-Token", target.Token)
		req.Header.Set("Authorization", "Bearer "+target.Token)
	}

	return req, nil
}

func (a *Agent) doSelfUpdateGetWithRetry(ctx context.Context, target TargetConfig, requestURL, operation string) (*http.Response, error) {
	client := a.httpClientFor(target)
	var lastErr error

	for attempt := 1; attempt <= selfUpdateRequestMaxAttempts; attempt++ {
		req, err := newAuthedSelfUpdateGetRequest(ctx, target, requestURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create %s request: %w", operation, err)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("%s request failed: %w", operation, err)
			if attempt == selfUpdateRequestMaxAttempts || ctx.Err() != nil {
				break
			}

			delay := selfUpdateRetryBackoffDelay(attempt)
			a.logger.Warn().
				Err(err).
				Str("operation", operation).
				Str("url", requestURL).
				Int("attempt", attempt).
				Int("maxAttempts", selfUpdateRequestMaxAttempts).
				Dur("retryIn", delay).
				Msg("Transient self-update request failure, retrying")
			if err := selfUpdateRetrySleepFn(ctx, delay); err != nil {
				return nil, fmt.Errorf("%s request canceled while waiting to retry: %w", operation, err)
			}
			continue
		}

		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		lastErr = fmt.Errorf("%s failed with status: %s", operation, resp.Status)
		if !isRetryableSelfUpdateStatus(resp.StatusCode) {
			resp.Body.Close()
			return nil, lastErr
		}

		resp.Body.Close()
		if attempt == selfUpdateRequestMaxAttempts || ctx.Err() != nil {
			break
		}

		delay := selfUpdateRetryBackoffDelay(attempt)
		a.logger.Warn().
			Str("operation", operation).
			Str("url", requestURL).
			Int("statusCode", resp.StatusCode).
			Int("attempt", attempt).
			Int("maxAttempts", selfUpdateRequestMaxAttempts).
			Dur("retryIn", delay).
			Msg("Transient self-update response status, retrying")
		if err := selfUpdateRetrySleepFn(ctx, delay); err != nil {
			return nil, fmt.Errorf("%s request canceled while waiting to retry: %w", operation, err)
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("%s request failed", operation)
	}

	return nil, lastErr
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

	var resp *http.Response
	lastErr := errors.New("failed to download new binary")

	for _, candidate := range candidates {
		response, err := a.doSelfUpdateGetWithRetry(ctx, target, candidate.url, "binary download")
		if err != nil {
			lastErr = err
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
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			a.logger.Warn().Err(closeErr).Msg("Self-update: failed to close download response body")
		}
	}()

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
		if closeErr := closeFileFn(tmpFile); closeErr != nil {
			a.logger.Warn().Err(closeErr).Msg("Self-update: failed to close temp file after write failure")
		}
		return fmt.Errorf("failed to write downloaded binary: %w", err)
	}
	if written > maxBinarySize {
		if closeErr := closeFileFn(tmpFile); closeErr != nil {
			a.logger.Warn().Err(closeErr).Msg("Self-update: failed to close oversized temp file")
		}
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
		a.logger.Error().
			Err(err).
			Int("outputBytes", len(output)).
			Msg("Self-update: pre-flight check failed")
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
		restoreErr := osRenameFn(backupPath, realExecPath)
		if restoreErr != nil {
			return errors.Join(
				fmt.Errorf("failed to replace binary: %w", err),
				fmt.Errorf("failed to restore backup binary: %w", restoreErr),
			)
		}
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	// Remove backup on success
	if err := osRemoveFn(backupPath); err != nil {
		a.logger.Warn().Err(err).Str("path", backupPath).Msg("Self-update: failed to remove backup binary after successful replacement")
	}

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
					if removeErr := osRemoveFn(tmpPersist); removeErr != nil {
						a.logger.Warn().Err(removeErr).Str("path", tmpPersist).Msg("Failed to remove temporary Unraid persistent binary")
					}
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
