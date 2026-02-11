// Package agentupdate provides self-update functionality for Pulse agents.
// It handles checking for new versions, downloading binaries, and performing
// atomic binary replacement with rollback support.
package agentupdate

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog"
)

const (
	// maxBinarySize is the maximum allowed size for downloaded binaries (100 MB)
	maxBinarySize = 100 * 1024 * 1024

	// downloadTimeout is the maximum time allowed for downloading a binary
	downloadTimeout = 5 * time.Minute
)

var (
	maxBinarySizeBytes     int64 = maxBinarySize
	runtimeGOOS                  = runtime.GOOS
	runtimeGOARCH                = runtime.GOARCH
	unameCommand                 = func() ([]byte, error) { return exec.Command("uname", "-m").Output() }
	unraidVersionPath            = "/etc/unraid-version"
	unraidPersistentPathFn       = unraidPersistentPath
	restartProcessFn             = restartProcess
	osExecutableFn               = os.Executable
	evalSymlinksFn               = filepath.EvalSymlinks
	createTempFn                 = os.CreateTemp
	chmodFn                      = os.Chmod
	renameFn                     = os.Rename
	closeFileFn                  = func(f *os.File) error { return f.Close() }
	readFileFn                   = os.ReadFile
	writeFileFn                  = os.WriteFile
)

// Config holds the configuration for the updater.
type Config struct {
	// PulseURL is the base URL of the Pulse server (e.g., "https://pulse.example.com:7655")
	PulseURL string

	// APIToken is the authentication token for the Pulse server
	APIToken string

	// AgentName is the name of the agent binary to download (e.g., "pulse-agent", "pulse-docker-agent")
	AgentName string

	// CurrentVersion is the version currently running
	CurrentVersion string

	// CheckInterval is how often to check for updates (default: 1 hour)
	CheckInterval time.Duration

	// InsecureSkipVerify skips TLS certificate verification
	InsecureSkipVerify bool

	// Logger is the zerolog logger instance
	Logger *zerolog.Logger

	// Disabled skips all update checks when true
	Disabled bool
}

// Updater handles automatic updates for Pulse agents.
type Updater struct {
	cfg    Config
	client *http.Client
	logger zerolog.Logger

	performUpdateFn func(context.Context) error
	initialDelay    time.Duration
	newTicker       func(time.Duration) *time.Ticker
}

// New creates a new Updater with the given configuration.
func New(cfg Config) *Updater {
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 1 * time.Hour
	}

	logger := zerolog.Nop()
	if cfg.Logger != nil {
		logger = *cfg.Logger
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec
		},
	}

	u := &Updater{
		cfg: cfg,
		client: &http.Client{
			Transport: transport,
			Timeout:   downloadTimeout,
		},
		logger: logger,
	}
	u.performUpdateFn = u.performUpdate
	u.initialDelay = 5 * time.Second
	u.newTicker = time.NewTicker
	return u
}

// RunLoop starts the update check loop. It blocks until the context is cancelled.
func (u *Updater) RunLoop(ctx context.Context) {
	if u.cfg.Disabled {
		u.logger.Info().Msg("Auto-update disabled")
		return
	}

	if u.cfg.CurrentVersion == "dev" {
		u.logger.Debug().Msg("Auto-update disabled in development mode")
		return
	}

	// Initial check after a short delay (5s to quickly update outdated agents)
	select {
	case <-ctx.Done():
		return
	case <-time.After(u.initialDelay):
		u.CheckAndUpdate(ctx)
	}

	ticker := u.newTicker(u.cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			u.CheckAndUpdate(ctx)
		}
	}
}

// CheckAndUpdate checks for a new version and performs the update if available.
func (u *Updater) CheckAndUpdate(ctx context.Context) {
	if u.cfg.Disabled {
		return
	}

	if u.cfg.CurrentVersion == "dev" {
		u.logger.Debug().Msg("Skipping update check - running in development mode")
		return
	}

	if u.cfg.PulseURL == "" {
		u.logger.Debug().Msg("Skipping update check - no Pulse URL configured")
		return
	}

	u.logger.Debug().Msg("Checking for agent updates")

	serverVersion, err := u.getServerVersion(ctx)
	if err != nil {
		u.logger.Warn().Err(err).Msg("Failed to check for updates")
		return
	}

	if serverVersion == "dev" {
		u.logger.Debug().Msg("Skipping update - server is in development mode")
		return
	}

	// Compare versions using semver comparison to only update to newer versions.
	// This prevents accidental downgrades if the server temporarily has an older version.
	currentNorm := utils.NormalizeVersion(u.cfg.CurrentVersion)
	serverNorm := utils.NormalizeVersion(serverVersion)

	cmp := utils.CompareVersions(serverNorm, currentNorm)
	if cmp == 0 {
		u.logger.Debug().Str("version", u.cfg.CurrentVersion).Msg("Agent is up to date")
		return
	}
	if cmp < 0 {
		u.logger.Debug().
			Str("currentVersion", currentNorm).
			Str("serverVersion", serverNorm).
			Msg("Server has older version, skipping downgrade")
		return
	}

	u.logger.Info().
		Str("currentVersion", u.cfg.CurrentVersion).
		Str("availableVersion", serverVersion).
		Msg("New agent version available, performing self-update")

	if err := u.performUpdateFn(ctx); err != nil {
		u.logger.Error().Err(err).Msg("Failed to self-update agent")
		return
	}

	u.logger.Info().Msg("Agent updated successfully, restarting...")
}

// getServerVersion fetches the current version from the Pulse server.
func (u *Updater) getServerVersion(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/api/agent/version", strings.TrimRight(u.cfg.PulseURL, "/"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("agentupdate.getServerVersion: create request: %w", err)
	}

	if u.cfg.APIToken != "" {
		req.Header.Set("X-API-Token", u.cfg.APIToken)
		req.Header.Set("Authorization", "Bearer "+u.cfg.APIToken)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("agentupdate.getServerVersion: execute request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			u.logger.Warn().Err(closeErr).Msg("agentupdate.getServerVersion: failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("agentupdate.getServerVersion: server returned status %d", resp.StatusCode)
	}

	var versionResp struct {
		Version string `json:"version"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&versionResp); err != nil {
		return "", fmt.Errorf("agentupdate.getServerVersion: decode response: %w", err)
	}

	return versionResp.Version, nil
}

// isUnraid checks if we're running on Unraid by looking for /etc/unraid-version
func isUnraid() bool {
	_, err := os.Stat(unraidVersionPath)
	return err == nil
}

// verifyBinaryMagic checks that the file is a valid executable for the current platform
func verifyBinaryMagic(path string) (retErr error) {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("verifyBinaryMagic: open %s: %w", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			closeWrapped := fmt.Errorf("verifyBinaryMagic: close %s: %w", path, closeErr)
			if retErr != nil {
				retErr = errors.Join(retErr, closeWrapped)
				return
			}
			retErr = closeWrapped
		}
	}()

	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return fmt.Errorf("verifyBinaryMagic: read magic bytes from %s: %w", path, err)
	}

	switch runtimeGOOS {
	case "linux":
		// ELF magic: 0x7f 'E' 'L' 'F'
		if magic[0] == 0x7f && magic[1] == 'E' && magic[2] == 'L' && magic[3] == 'F' {
			return nil
		}
		return fmt.Errorf("verifyBinaryMagic: %s is not a valid ELF binary", path)

	case "darwin":
		// Mach-O magic bytes (little-endian):
		// - 0xfeedface (32-bit)
		// - 0xfeedfacf (64-bit)
		// - 0xcafebabe (universal/fat binary)
		// Note: bytes are reversed due to little-endian
		if (magic[0] == 0xcf && magic[1] == 0xfa && magic[2] == 0xed && magic[3] == 0xfe) || // 64-bit
			(magic[0] == 0xce && magic[1] == 0xfa && magic[2] == 0xed && magic[3] == 0xfe) || // 32-bit
			(magic[0] == 0xca && magic[1] == 0xfe && magic[2] == 0xba && magic[3] == 0xbe) { // universal
			return nil
		}
		return fmt.Errorf("verifyBinaryMagic: %s is not a valid Mach-O binary", path)

	case "windows":
		// PE magic: 'M' 'Z'
		if magic[0] == 'M' && magic[1] == 'Z' {
			return nil
		}
		return fmt.Errorf("verifyBinaryMagic: %s is not a valid PE binary", path)

	default:
		// Unknown platform, skip verification
		return nil
	}
}

// unraidPersistentPath returns the path where the binary should be persisted on Unraid
func unraidPersistentPath(agentName string) string {
	return fmt.Sprintf("/boot/config/plugins/%s/%s", agentName, agentName)
}

// performUpdate downloads and installs the new agent binary.
func (u *Updater) performUpdate(ctx context.Context) error {
	execPath, err := osExecutableFn()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	return u.performUpdateWithExecPath(ctx, execPath)
}

func (u *Updater) performUpdateWithExecPath(ctx context.Context, execPath string) error {

	// Build download URL
	downloadBase := fmt.Sprintf("%s/download/%s", strings.TrimRight(u.cfg.PulseURL, "/"), u.cfg.AgentName)
	archParam := determineArch()

	// Try architecture-specific binary first, then fall back to default
	candidates := []string{}
	if archParam != "" {
		candidates = append(candidates, fmt.Sprintf("%s?arch=%s", downloadBase, archParam))
	}
	candidates = append(candidates, downloadBase)

	var resp *http.Response
	lastErr := fmt.Errorf("failed to download binary from all candidate URLs")

	for _, url := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			lastErr = fmt.Errorf("failed to create download request for %s: %w", url, err)
			continue
		}

		if u.cfg.APIToken != "" {
			req.Header.Set("X-API-Token", u.cfg.APIToken)
			req.Header.Set("Authorization", "Bearer "+u.cfg.APIToken)
		}

		response, err := u.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to download binary from %s: %w", url, err)
			continue
		}

		if response.StatusCode != http.StatusOK {
			statusErr := fmt.Errorf("download from %s failed with status: %s", url, response.Status)
			if closeErr := response.Body.Close(); closeErr != nil {
				lastErr = errors.Join(statusErr, fmt.Errorf("failed to close failed download response body: %w", closeErr))
			} else {
				lastErr = statusErr
			}
			continue
		}

		resp = response
		u.logger.Debug().Str("url", url).Msg("Downloaded agent binary")
		break
	}

	if resp == nil {
		return lastErr
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			u.logger.Warn().Err(closeErr).Msg("agentupdate.performUpdateWithExecPath: failed to close download response body")
		}
	}()

	// Verify checksum if provided
	checksumHeader := strings.TrimSpace(resp.Header.Get("X-Checksum-Sha256"))

	// Resolve symlinks to get the real path for atomic rename
	realExecPath, err := evalSymlinksFn(execPath)
	if err != nil {
		// Fall back to original path if symlink resolution fails
		realExecPath = execPath
	}

	// Create temporary file in the same directory as the target binary
	// to ensure atomic rename works (os.Rename fails across filesystems)
	targetDir := filepath.Dir(realExecPath)
	tmpFile, err := createTempFn(targetDir, u.cfg.AgentName+"-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if removeErr := os.Remove(tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			u.logger.Warn().Err(removeErr).Str("path", tmpPath).Msg("agentupdate.performUpdateWithExecPath: failed to remove temp file")
		}
	}() // Clean up on failure

	// Write downloaded binary with checksum calculation and size limit
	hasher := sha256.New()
	limitedReader := io.LimitReader(resp.Body, maxBinarySizeBytes+1) // +1 to detect overflow
	written, err := io.Copy(tmpFile, io.TeeReader(limitedReader, hasher))
	if err != nil {
		writeErr := fmt.Errorf("failed to write binary: %w", err)
		if closeErr := closeFileFn(tmpFile); closeErr != nil {
			return errors.Join(writeErr, fmt.Errorf("failed to close temp file after write failure: %w", closeErr))
		}
		return writeErr
	}
	if written > maxBinarySizeBytes {
		sizeErr := fmt.Errorf("downloaded binary exceeds maximum size (%d bytes)", maxBinarySizeBytes)
		if closeErr := closeFileFn(tmpFile); closeErr != nil {
			return errors.Join(sizeErr, fmt.Errorf("failed to close temp file after size validation failure: %w", closeErr))
		}
		return sizeErr
	}
	if err := closeFileFn(tmpFile); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Verify it's a valid executable (basic sanity check)
	if err := verifyBinaryMagic(tmpPath); err != nil {
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
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}
	u.logger.Debug().Str("checksum", downloadChecksum).Msg("Checksum verified")

	// Make executable
	if err := chmodFn(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to chmod: %w", err)
	}

	// Atomic replacement with backup (use realExecPath for rename operations)
	backupPath := realExecPath + ".backup"
	if err := renameFn(realExecPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	if err := renameFn(tmpPath, realExecPath); err != nil {
		// Restore backup on failure
		if restoreErr := renameFn(backupPath, realExecPath); restoreErr != nil {
			return errors.Join(
				fmt.Errorf("failed to replace binary: %w", err),
				fmt.Errorf("failed to restore backup binary: %w", restoreErr),
			)
		}
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	// Remove backup on success
	if err := os.Remove(backupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		u.logger.Warn().Err(err).Str("path", backupPath).Msg("agentupdate.performUpdateWithExecPath: failed to remove backup binary")
	}

	// Write previous version to a file so the agent can report "updated from X" on next start
	updateInfoPath := filepath.Join(targetDir, ".pulse-update-info")
	if err := writeFileFn(updateInfoPath, []byte(u.cfg.CurrentVersion), 0644); err != nil {
		u.logger.Warn().Err(err).Str("path", updateInfoPath).Msg("agentupdate.performUpdateWithExecPath: failed to write update info")
	}

	// On Unraid, also update the persistent copy on the flash drive
	// This ensures the update survives reboots
	if isUnraid() {
		persistPath := unraidPersistentPathFn(u.cfg.AgentName)
		if _, err := os.Stat(persistPath); err == nil {
			// Persistent path exists, update it
			u.logger.Debug().Str("path", persistPath).Msg("Updating Unraid persistent binary")

			// Read the newly installed binary
			newBinary, err := readFileFn(execPath)
			if err != nil {
				u.logger.Warn().Err(err).Msg("Failed to read new binary for Unraid persistence")
			} else {
				// Write to persistent storage (atomic via temp file)
				tmpPersist := persistPath + ".tmp"
				if err := writeFileFn(tmpPersist, newBinary, 0644); err != nil {
					u.logger.Warn().Err(err).Msg("Failed to write Unraid persistent binary")
				} else if err := renameFn(tmpPersist, persistPath); err != nil {
					u.logger.Warn().Err(err).Msg("Failed to rename Unraid persistent binary")
					if removeErr := os.Remove(tmpPersist); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
						u.logger.Warn().Err(removeErr).Str("path", tmpPersist).Msg("Failed to remove temporary Unraid persistent binary")
					}
				} else {
					u.logger.Info().Str("path", persistPath).Msg("Updated Unraid persistent binary")
				}
			}
		}
	}

	// Restart the process using platform-specific implementation
	return restartProcessFn(execPath)
}

// GetUpdatedFromVersion checks if the agent was recently updated and returns the previous version.
// Returns empty string if no update info exists. Clears the info file after reading.
func GetUpdatedFromVersion() string {
	execPath, err := osExecutableFn()
	if err != nil {
		return ""
	}

	// Resolve symlinks to get the real path
	realExecPath, err := evalSymlinksFn(execPath)
	if err != nil {
		realExecPath = execPath
	}

	updateInfoPath := filepath.Join(filepath.Dir(realExecPath), ".pulse-update-info")
	data, err := os.ReadFile(updateInfoPath)
	if err != nil {
		return ""
	}

	// Clear the file after reading (only report once)
	os.Remove(updateInfoPath)

	return strings.TrimSpace(string(data))
}

// determineArch returns the architecture string for download URLs (e.g., "linux-amd64", "darwin-arm64").
func determineArch() string {
	os := runtimeGOOS
	arch := runtimeGOARCH

	// Normalize architecture
	switch arch {
	case "arm":
		arch = "armv7"
	case "386":
		arch = "386"
	}

	// For known OS/arch combinations, return directly
	switch os {
	case "linux", "darwin", "windows":
		return fmt.Sprintf("%s-%s", os, arch)
	}

	// Fall back to uname for edge cases on unknown OS
	out, err := unameCommand()
	if err != nil {
		return ""
	}

	normalized := strings.ToLower(strings.TrimSpace(string(out)))
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
