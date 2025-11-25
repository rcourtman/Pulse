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
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
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
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		},
	}

	return &Updater{
		cfg: cfg,
		client: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		logger: logger,
	}
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

	// Initial check after a short delay
	select {
	case <-ctx.Done():
		return
	case <-time.After(30 * time.Second):
		u.CheckAndUpdate(ctx)
	}

	ticker := time.NewTicker(u.cfg.CheckInterval)
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

	if serverVersion == u.cfg.CurrentVersion {
		u.logger.Debug().Str("version", u.cfg.CurrentVersion).Msg("Agent is up to date")
		return
	}

	u.logger.Info().
		Str("currentVersion", u.cfg.CurrentVersion).
		Str("availableVersion", serverVersion).
		Msg("New agent version available, performing self-update")

	if err := u.performUpdate(ctx); err != nil {
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
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	if u.cfg.APIToken != "" {
		req.Header.Set("X-API-Token", u.cfg.APIToken)
		req.Header.Set("Authorization", "Bearer "+u.cfg.APIToken)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var versionResp struct {
		Version string `json:"version"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&versionResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return versionResp.Version, nil
}

// performUpdate downloads and installs the new agent binary.
func (u *Updater) performUpdate(ctx context.Context) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

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
	var lastErr error

	for _, url := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			lastErr = fmt.Errorf("failed to create download request: %w", err)
			continue
		}

		if u.cfg.APIToken != "" {
			req.Header.Set("X-API-Token", u.cfg.APIToken)
			req.Header.Set("Authorization", "Bearer "+u.cfg.APIToken)
		}

		response, err := u.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to download binary: %w", err)
			continue
		}

		if response.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("download failed with status: %s", response.Status)
			response.Body.Close()
			continue
		}

		resp = response
		u.logger.Debug().Str("url", url).Msg("Downloaded agent binary")
		break
	}

	if resp == nil {
		if lastErr == nil {
			lastErr = errors.New("failed to download binary")
		}
		return lastErr
	}
	defer resp.Body.Close()

	// Verify checksum if provided
	checksumHeader := strings.TrimSpace(resp.Header.Get("X-Checksum-Sha256"))

	// Create temporary file
	tmpFile, err := os.CreateTemp("", u.cfg.AgentName+"-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up on failure

	// Write downloaded binary with checksum calculation
	hasher := sha256.New()
	if _, err := io.Copy(tmpFile, io.TeeReader(resp.Body, hasher)); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write binary: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Verify checksum
	downloadChecksum := hex.EncodeToString(hasher.Sum(nil))
	if checksumHeader != "" {
		expected := strings.ToLower(strings.TrimSpace(checksumHeader))
		actual := strings.ToLower(downloadChecksum)
		if expected != actual {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
		}
		u.logger.Debug().Str("checksum", downloadChecksum).Msg("Checksum verified")
	} else {
		u.logger.Warn().Msg("No checksum header; skipping verification")
	}

	// Make executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to chmod: %w", err)
	}

	// Atomic replacement with backup
	backupPath := execPath + ".backup"
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		// Restore backup on failure
		os.Rename(backupPath, execPath)
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	// Remove backup on success
	os.Remove(backupPath)

	// Restart with same arguments
	args := os.Args
	env := os.Environ()

	if err := syscall.Exec(execPath, args, env); err != nil {
		return fmt.Errorf("failed to restart: %w", err)
	}

	return nil
}

// determineArch returns the architecture string for download URLs (e.g., "linux-amd64", "darwin-arm64").
func determineArch() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

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
	out, err := exec.Command("uname", "-m").Output()
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
