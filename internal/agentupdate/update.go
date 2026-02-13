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
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog"
)

const (
	// maxBinarySize is the maximum allowed size for downloaded binaries (100 MB)
	maxBinarySize = 100 * 1024 * 1024

	// maxVersionResponseSize is the maximum allowed size for version endpoint responses.
	maxVersionResponseSize = 16 * 1024

	// downloadTimeout is the maximum time allowed for downloading a binary
	downloadTimeout = 5 * time.Minute

	developmentVersion = "dev"

	apiTokenHeader       = "X-API-Token"
	authorizationHeader  = "Authorization"
	bearerTokenPrefix    = "Bearer "
	checksumSHA256Header = "X-Checksum-Sha256"

	// updateRequestMaxAttempts is the number of attempts for transient update HTTP failures.
	updateRequestMaxAttempts = 3

	// updateRetryBaseDelay is the initial retry backoff delay for update HTTP failures.
	updateRetryBaseDelay = 25 * time.Millisecond

	// updateRetryMaxDelay caps exponential backoff for update HTTP retries.
	updateRetryMaxDelay = 250 * time.Millisecond
)

type goOS string

const (
	goOSLinux   goOS = "linux"
	goOSDarwin  goOS = "darwin"
	goOSWindows goOS = "windows"
)

type serverVersionResponse struct {
	Version string `json:"version"`
}

var (
	maxBinarySizeBytes     int64 = maxBinarySize
	runtimeGOOS                  = goOS(runtime.GOOS)
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
	retrySleepFn                 = sleepWithContext
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

	checkMu         sync.Mutex
	checkInProgress bool

	performUpdateFn func(context.Context) error
	initialDelay    time.Duration
	newTicker       func(time.Duration) *time.Ticker
}

// New creates a new Updater with the given configuration.
func New(cfg Config) *Updater {
	originalCheckInterval := cfg.CheckInterval
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 1 * time.Hour
	}

	logger := zerolog.Nop()
	if cfg.Logger != nil {
		logger = *cfg.Logger
	}
	if originalCheckInterval < 0 {
		logger.Warn().
			Dur("check_interval", originalCheckInterval).
			Dur("default_check_interval", cfg.CheckInterval).
			Msg("Invalid agent update check interval; using default")
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
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Prevent credential leakage (X-API-Token / Authorization) on redirects.
				return fmt.Errorf("server returned redirect to %s", req.URL.Redacted())
			},
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
		u.logger.Info().Msg("auto-update disabled")
		return
	}

	if u.cfg.CurrentVersion == developmentVersion {
		u.logger.Debug().Msg("Auto-update disabled in development mode")
		return
	}

	// Initial check after a short delay (5s to quickly update outdated agents)
	initialDelayTimer := time.NewTimer(u.initialDelay)
	defer func() {
		if !initialDelayTimer.Stop() {
			select {
			case <-initialDelayTimer.C:
			default:
			}
		}
	}()

	select {
	case <-ctx.Done():
		return
	case <-initialDelayTimer.C:
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
	if !u.startCheck() {
		u.logger.Debug().Msg("Skipping update check - another check is already in progress")
		return
	}
	defer u.finishCheck()

	if u.cfg.Disabled {
		return
	}

	if u.cfg.CurrentVersion == developmentVersion {
		u.logger.Debug().Msg("Skipping update check - running in development mode")
		return
	}

	if u.cfg.PulseURL == "" {
		u.logger.Debug().Msg("skipping update check - no pulse URL configured")
		return
	}
	if err := u.validatePulseURL(); err != nil {
		u.logger.Warn().Err(err).Msg("Skipping update check - insecure or invalid Pulse URL")
		return
	}

	u.logger.Debug().Msg("checking for agent updates")

	serverVersion, err := u.getServerVersion(ctx)
	if err != nil {
		u.logger.Warn().Err(err).Msg("failed to check for updates")
		return
	}

	if serverVersion == developmentVersion {
		u.logger.Debug().Msg("Skipping update - server is in development mode")
		return
	}

	// Compare versions using semver comparison to only update to newer versions.
	// This prevents accidental downgrades if the server temporarily has an older version.
	currentNorm := utils.NormalizeVersion(u.cfg.CurrentVersion)
	serverNorm := utils.NormalizeVersion(serverVersion)

	cmp := utils.CompareVersions(serverNorm, currentNorm)
	if cmp == 0 {
		u.logger.Debug().Str("version", u.cfg.CurrentVersion).Msg("agent is up to date")
		return
	}
	if cmp < 0 {
		u.logger.Debug().
			Str("currentVersion", currentNorm).
			Str("serverVersion", serverNorm).
			Msg("server has older version, skipping downgrade")
		return
	}

	u.logger.Info().
		Str("currentVersion", u.cfg.CurrentVersion).
		Str("availableVersion", serverVersion).
		Msg("new agent version available, performing self-update")

	if err := u.performUpdateFn(ctx); err != nil {
		u.logger.Error().
			Err(err).
			Str("currentVersion", u.cfg.CurrentVersion).
			Str("targetVersion", serverVersion).
			Msg("failed to self-update agent")
		return
	}

	u.logger.Info().Msg("agent updated successfully, restarting")
}

// setAuthHeaders adds authentication headers to the request if an API token is configured.
func (u *Updater) setAuthHeaders(req *http.Request) {
	if u.cfg.APIToken != "" {
		req.Header.Set(apiTokenHeader, u.cfg.APIToken)
		req.Header.Set(authorizationHeader, bearerTokenPrefix+u.cfg.APIToken)
	}
}

func (u *Updater) startCheck() bool {
	u.checkMu.Lock()
	defer u.checkMu.Unlock()

	if u.checkInProgress {
		return false
	}

	u.checkInProgress = true
	return true
}

func (u *Updater) finishCheck() {
	u.checkMu.Lock()
	u.checkInProgress = false
	u.checkMu.Unlock()
}

// getServerVersion fetches the current version from the Pulse server.
func (u *Updater) getServerVersion(ctx context.Context) (string, error) {
	if err := u.validatePulseURL(); err != nil {
		return "", fmt.Errorf("invalid Pulse URL: %w", err)
	}

	url := fmt.Sprintf("%s/api/agent/version", strings.TrimRight(u.cfg.PulseURL, "/"))

	resp, err := u.getWithRetry(ctx, url, "version check")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var versionResp serverVersionResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxVersionResponseSize)).Decode(&versionResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return versionResp.Version, nil
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryBackoffDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return updateRetryBaseDelay
	}

	delay := updateRetryBaseDelay * time.Duration(1<<(attempt-1))
	if delay > updateRetryMaxDelay {
		return updateRetryMaxDelay
	}
	return delay
}

func isRetryableHTTPStatus(code int) bool {
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

func (u *Updater) newAuthedGetRequest(ctx context.Context, requestURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	if u.cfg.APIToken != "" {
		req.Header.Set("X-API-Token", u.cfg.APIToken)
		req.Header.Set("Authorization", "Bearer "+u.cfg.APIToken)
	}

	return req, nil
}

func (u *Updater) getWithRetry(ctx context.Context, requestURL, operation string) (*http.Response, error) {
	var lastErr error

	for attempt := 1; attempt <= updateRequestMaxAttempts; attempt++ {
		req, err := u.newAuthedGetRequest(ctx, requestURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create %s request: %w", operation, err)
		}

		resp, err := u.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("%s request failed: %w", operation, err)
			if attempt == updateRequestMaxAttempts || ctx.Err() != nil {
				break
			}

			delay := retryBackoffDelay(attempt)
			u.logger.Warn().
				Err(err).
				Str("operation", operation).
				Str("url", requestURL).
				Int("attempt", attempt).
				Int("maxAttempts", updateRequestMaxAttempts).
				Dur("retryIn", delay).
				Msg("Transient update request failure, retrying")
			if err := retrySleepFn(ctx, delay); err != nil {
				return nil, fmt.Errorf("%s request canceled while waiting to retry: %w", operation, err)
			}
			continue
		}

		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		lastErr = fmt.Errorf("%s failed with status: %s", operation, resp.Status)
		if !isRetryableHTTPStatus(resp.StatusCode) {
			resp.Body.Close()
			return nil, lastErr
		}

		resp.Body.Close()
		if attempt == updateRequestMaxAttempts || ctx.Err() != nil {
			break
		}

		delay := retryBackoffDelay(attempt)
		u.logger.Warn().
			Str("operation", operation).
			Str("url", requestURL).
			Int("statusCode", resp.StatusCode).
			Int("attempt", attempt).
			Int("maxAttempts", updateRequestMaxAttempts).
			Dur("retryIn", delay).
			Msg("Transient update response status, retrying")
		if err := retrySleepFn(ctx, delay); err != nil {
			return nil, fmt.Errorf("%s request canceled while waiting to retry: %w", operation, err)
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("%s request failed", operation)
	}

	return nil, lastErr
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
		return fmt.Errorf("open %s: %w", path, err)
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
	case goOSLinux:
		// ELF magic: 0x7f 'E' 'L' 'F'
		if magic[0] == 0x7f && magic[1] == 'E' && magic[2] == 'L' && magic[3] == 'F' {
			return nil
		}
		return fmt.Errorf("verifyBinaryMagic: %s is not a valid ELF binary", path)

	case goOSDarwin:
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

	case goOSWindows:
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

func normalizeAgentName(agentName string) (string, error) {
	name := strings.TrimSpace(agentName)
	if name == "" {
		return "", fmt.Errorf("agent name is required")
	}
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("agent name must not contain path traversal")
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '.' {
			continue
		}
		return "", fmt.Errorf("agent name contains invalid character %q", r)
	}
	return name, nil
}

func isLoopbackHost(host string) bool {
	if host == "" {
		return false
	}

	normalized := strings.ToLower(strings.Trim(host, "[]"))
	if normalized == "localhost" || strings.HasSuffix(normalized, ".localhost") {
		return true
	}

	ip := net.ParseIP(normalized)
	return ip != nil && ip.IsLoopback()
}

func (u *Updater) validatePulseURL() error {
	pulseURL := strings.TrimSpace(u.cfg.PulseURL)
	if pulseURL == "" {
		return fmt.Errorf("Pulse URL is empty")
	}

	parsed, err := url.Parse(pulseURL)
	if err != nil {
		return fmt.Errorf("failed to parse Pulse URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("Pulse URL must include scheme and host")
	}
	if parsed.User != nil {
		return fmt.Errorf("Pulse URL must not include user credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("Pulse URL must not include query or fragment")
	}

	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "https":
		return nil
	case "http":
		if u.cfg.InsecureSkipVerify || isLoopbackHost(parsed.Hostname()) {
			return nil
		}
		return fmt.Errorf("HTTP Pulse URL is only allowed for localhost/loopback or when insecure mode is enabled")
	default:
		return fmt.Errorf("unsupported Pulse URL scheme %q", parsed.Scheme)
	}
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
	agentName, err := normalizeAgentName(u.cfg.AgentName)
	if err != nil {
		return fmt.Errorf("invalid agent name: %w", err)
	}
	if err := u.validatePulseURL(); err != nil {
		return fmt.Errorf("invalid Pulse URL: %w", err)
	}

	// Build download URL
	downloadBase := fmt.Sprintf("%s/download/%s", strings.TrimRight(u.cfg.PulseURL, "/"), agentName)
	archParam := determineArch()

	// Try architecture-specific binary first, then fall back to default
	candidates := []string{}
	if archParam != "" {
		candidates = append(candidates, fmt.Sprintf("%s?arch=%s", downloadBase, archParam))
	}
	candidates = append(candidates, downloadBase)

	var resp *http.Response
	lastErr := fmt.Errorf("failed to download binary from all candidate URLs")

	for _, candidateURL := range candidates {
		response, err := u.getWithRetry(ctx, candidateURL, "binary download")
		if err != nil {
			lastErr = err
			continue
		}

		resp = response
		u.logger.Debug().Str("url", candidateURL).Msg("Downloaded agent binary")
		break
	}

	if resp == nil {
		return lastErr
	}
	defer resp.Body.Close()
	if resp.ContentLength > maxBinarySizeBytes {
		return fmt.Errorf("downloaded binary exceeds maximum size (%d bytes)", maxBinarySizeBytes)
	}

	// Verify checksum if provided
	checksumHeader := strings.TrimSpace(resp.Header.Get(checksumSHA256Header))

	// Resolve symlinks to get the real path for atomic rename
	realExecPath, err := evalSymlinksFn(execPath)
	if err != nil {
		// Fall back to original path if symlink resolution fails
		realExecPath = execPath
	}

	// Create temporary file in the same directory as the target binary
	// to ensure atomic rename works (os.Rename fails across filesystems)
	targetDir := filepath.Dir(realExecPath)
	tmpFile, err := createTempFn(targetDir, agentName+"-*.tmp")
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
		return fmt.Errorf("server did not provide checksum header (%s); refusing update for security", checksumSHA256Header)
	}

	expected := strings.ToLower(strings.TrimSpace(checksumHeader))
	actual := strings.ToLower(downloadChecksum)
	if expected != actual {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}
	u.logger.Debug().Str("checksum", downloadChecksum).Msg("checksum verified")

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
		persistPath := unraidPersistentPathFn(agentName)
		if _, err := os.Stat(persistPath); err == nil {
			// Persistent path exists, update it
			u.logger.Debug().Str("path", persistPath).Msg("updating unraid persistent binary")

			// Read the newly installed binary
			newBinary, err := readFileFn(execPath)
			if err != nil {
				u.logger.Warn().Err(err).Str("path", execPath).Msg("failed to read new binary for unraid persistence")
			} else {
				// Write to persistent storage (atomic via temp file)
				tmpPersist := persistPath + ".tmp"
				if err := writeFileFn(tmpPersist, newBinary, 0644); err != nil {
					u.logger.Warn().Err(err).Str("path", tmpPersist).Msg("failed to write unraid persistent binary")
				} else if err := renameFn(tmpPersist, persistPath); err != nil {
					u.logger.Warn().Err(err).Msg("Failed to rename Unraid persistent binary")
					if removeErr := os.Remove(tmpPersist); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
						u.logger.Warn().Err(removeErr).Str("path", tmpPersist).Msg("Failed to remove temporary Unraid persistent binary")
					}
				} else {
					u.logger.Info().Str("path", persistPath).Msg("updated unraid persistent binary")
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
	goos := runtimeGOOS
	arch := runtimeGOARCH

	// Normalize architecture
	switch arch {
	case "arm":
		arch = "armv7"
	case "386":
		arch = "386"
	}

	// For known OS/arch combinations, return directly
	switch goos {
	case goOSLinux, goOSDarwin, goOSWindows, "freebsd":
		return fmt.Sprintf("%s-%s", goos, arch)
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
