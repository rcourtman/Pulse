package updates

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// UpdateStatus represents the current status of an update
type UpdateStatus struct {
	Status    string  `json:"status"`
	Progress  int     `json:"progress"`
	Message   string  `json:"message"`
	Error     string  `json:"error,omitempty"`
	UpdatedAt string  `json:"updatedAt"`
}

// ReleaseInfo represents a GitHub release
type ReleaseInfo struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// UpdateInfo represents available update information
type UpdateInfo struct {
	Available      bool       `json:"available"`
	CurrentVersion string     `json:"currentVersion"`
	LatestVersion  string     `json:"latestVersion"`
	ReleaseNotes   string     `json:"releaseNotes"`
	ReleaseDate    time.Time  `json:"releaseDate"`
	DownloadURL    string     `json:"downloadUrl"`
	IsPrerelease   bool       `json:"isPrerelease"`
}

// Manager handles update operations
type Manager struct {
	config       *config.Config
	status       UpdateStatus
	statusMu     sync.RWMutex
	checkCache   *UpdateInfo
	cacheTime    time.Time
	cacheDuration time.Duration
	progressChan chan UpdateStatus
}

// NewManager creates a new update manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		config:        cfg,
		cacheDuration: 5 * time.Minute, // Cache update checks for 5 minutes
		progressChan:  make(chan UpdateStatus, 100),
		status: UpdateStatus{
			Status:    "idle",
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
}

// GetProgressChannel returns the channel for update progress
func (m *Manager) GetProgressChannel() <-chan UpdateStatus {
	return m.progressChan
}

// CheckForUpdates checks GitHub for available updates
func (m *Manager) CheckForUpdates(ctx context.Context) (*UpdateInfo, error) {
	// Check cache first
	if m.checkCache != nil && time.Since(m.cacheTime) < m.cacheDuration {
		return m.checkCache, nil
	}

	m.updateStatus("checking", 0, "Checking for updates...")

	// Get current version
	currentInfo, err := GetCurrentVersion()
	if err != nil {
		m.updateStatus("error", 0, "Failed to get current version")
		return nil, fmt.Errorf("failed to get current version: %w", err)
	}

	// Skip update check for Docker
	if currentInfo.IsDocker {
		info := &UpdateInfo{
			Available:      false,
			CurrentVersion: currentInfo.Version,
			LatestVersion:  currentInfo.Version,
		}
		m.checkCache = info
		m.cacheTime = time.Now()
		m.updateStatus("idle", 0, "Updates not available in Docker")
		return info, nil
	}

	// Get latest release from GitHub
	release, err := m.getLatestRelease(ctx)
	if err != nil {
		m.updateStatus("error", 0, "Failed to check for updates")
		return nil, err
	}

	// Parse versions
	currentVer, err := ParseVersion(currentInfo.Version)
	if err != nil {
		m.updateStatus("error", 0, "Invalid current version")
		return nil, fmt.Errorf("failed to parse current version: %w", err)
	}

	latestVer, err := ParseVersion(release.TagName)
	if err != nil {
		m.updateStatus("error", 0, "Invalid latest version")
		return nil, fmt.Errorf("failed to parse latest version: %w", err)
	}

	// Find download URL
	downloadURL := ""
	for _, asset := range release.Assets {
		if strings.HasPrefix(asset.Name, "pulse-") && strings.HasSuffix(asset.Name, ".tar.gz") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	info := &UpdateInfo{
		Available:      latestVer.IsNewerThan(currentVer),
		CurrentVersion: currentInfo.Version,
		LatestVersion:  release.TagName,
		ReleaseNotes:   release.Body,
		ReleaseDate:    release.PublishedAt,
		DownloadURL:    downloadURL,
		IsPrerelease:   release.Prerelease,
	}

	// Cache the result
	m.checkCache = info
	m.cacheTime = time.Now()

	status := "idle"
	message := "No updates available"
	if info.Available {
		status = "available"
		message = fmt.Sprintf("Update available: %s", info.LatestVersion)
	}
	m.updateStatus(status, 100, message)

	return info, nil
}

// ApplyUpdate downloads and applies an update
func (m *Manager) ApplyUpdate(ctx context.Context, downloadURL string) error {
	// Validate download URL
	if !strings.HasPrefix(downloadURL, "https://github.com/rcourtman/Pulse/releases/download/") {
		return fmt.Errorf("invalid download URL")
	}

	// Check if Docker
	currentInfo, _ := GetCurrentVersion()
	if currentInfo.IsDocker {
		return fmt.Errorf("updates cannot be applied in Docker environment")
	}

	// Check for pre-v4 installation
	if isPreV4Installation() {
		return fmt.Errorf("manual migration required: Pulse v4 is a complete rewrite. Please create a fresh installation. See https://github.com/rcourtman/Pulse/releases/v4.0.0")
	}

	m.updateStatus("downloading", 10, "Downloading update...")

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "pulse-update-*")
	if err != nil {
		m.updateStatus("error", 10, "Failed to create temp directory")
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download update
	tarballPath := filepath.Join(tempDir, "update.tar.gz")
	if err := m.downloadFile(ctx, downloadURL, tarballPath); err != nil {
		m.updateStatus("error", 20, "Failed to download update")
		return fmt.Errorf("failed to download update: %w", err)
	}

	m.updateStatus("extracting", 40, "Extracting update...")

	// Extract tarball
	extractDir := filepath.Join(tempDir, "extracted")
	if err := m.extractTarball(tarballPath, extractDir); err != nil {
		m.updateStatus("error", 40, "Failed to extract update")
		return fmt.Errorf("failed to extract update: %w", err)
	}

	m.updateStatus("backing-up", 60, "Creating backup...")

	// Create backup
	backupPath, err := m.createBackup()
	if err != nil {
		m.updateStatus("error", 60, "Failed to create backup")
		return fmt.Errorf("failed to create backup: %w", err)
	}
	log.Info().Str("backup", backupPath).Msg("Created backup")

	m.updateStatus("applying", 80, "Applying update...")

	// Apply update
	if err := m.applyUpdateFiles(extractDir); err != nil {
		m.updateStatus("error", 80, "Failed to apply update")
		// Attempt to restore backup
		if restoreErr := m.restoreBackup(backupPath); restoreErr != nil {
			log.Error().Err(restoreErr).Msg("Failed to restore backup")
		}
		return fmt.Errorf("failed to apply update: %w", err)
	}

	m.updateStatus("restarting", 95, "Restarting service...")

	// Schedule service restart
	go func() {
		time.Sleep(2 * time.Second)
		if err := m.restartService(); err != nil {
			log.Error().Err(err).Msg("Failed to restart service")
		}
	}()

	m.updateStatus("completed", 100, "Update completed, restarting...")
	return nil
}

// GetStatus returns the current update status
func (m *Manager) GetStatus() UpdateStatus {
	m.statusMu.RLock()
	defer m.statusMu.RUnlock()
	return m.status
}

// getLatestRelease fetches the latest release from GitHub
func (m *Manager) getLatestRelease(ctx context.Context) (*ReleaseInfo, error) {
	channel := m.config.UpdateChannel
	if channel == "" {
		channel = "stable"
	}

	// GitHub API URL
	url := "https://api.github.com/repos/rcourtman/Pulse/releases"
	if channel == "stable" {
		url += "/latest"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add headers
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Pulse-Update-Checker")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	if channel == "stable" {
		var release ReleaseInfo
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			return nil, fmt.Errorf("failed to decode release: %w", err)
		}
		return &release, nil
	}

	// For RC channel, get all releases and find latest RC
	var releases []ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode releases: %w", err)
	}

	// Find latest release (including prereleases)
	if len(releases) > 0 {
		return &releases[0], nil
	}

	return nil, fmt.Errorf("no releases found")
}

// downloadFile downloads a file from URL to dest
func (m *Manager) downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	// Copy with progress updates
	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	log.Info().Int64("bytes", written).Str("file", dest).Msg("Downloaded file")
	return nil
}

// extractTarball extracts a gzipped tarball
func (m *Manager) extractTarball(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)
		
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			
			out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}

	return nil
}

// createBackup creates a backup of the current installation
func (m *Manager) createBackup() (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	backupDir := fmt.Sprintf("/tmp/pulse-backup-%s", timestamp)
	
	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", err
	}

	// Backup important directories
	dirsToBackup := []string{"data", "config"}
	pulseDir := "/opt/pulse"

	for _, dir := range dirsToBackup {
		src := filepath.Join(pulseDir, dir)
		dest := filepath.Join(backupDir, dir)
		
		if _, err := os.Stat(src); err == nil {
			cmd := exec.Command("cp", "-r", src, dest)
			if err := cmd.Run(); err != nil {
				log.Warn().Str("dir", dir).Err(err).Msg("Failed to backup directory")
			}
		}
	}

	// Backup .env file
	envSrc := filepath.Join(pulseDir, ".env")
	if _, err := os.Stat(envSrc); err == nil {
		envDest := filepath.Join(backupDir, ".env")
		cmd := exec.Command("cp", envSrc, envDest)
		cmd.Run()
	}

	return backupDir, nil
}

// restoreBackup restores from a backup
func (m *Manager) restoreBackup(backupDir string) error {
	pulseDir := "/opt/pulse"
	
	// Restore directories
	dirsToRestore := []string{"data", "config"}
	for _, dir := range dirsToRestore {
		src := filepath.Join(backupDir, dir)
		dest := filepath.Join(pulseDir, dir)
		
		if _, err := os.Stat(src); err == nil {
			cmd := exec.Command("cp", "-r", src, dest)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to restore %s: %w", dir, err)
			}
		}
	}

	// Restore .env
	envSrc := filepath.Join(backupDir, ".env")
	if _, err := os.Stat(envSrc); err == nil {
		envDest := filepath.Join(pulseDir, ".env")
		cmd := exec.Command("cp", envSrc, envDest)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to restore .env: %w", err)
		}
	}

	return nil
}

// applyUpdateFiles copies update files to the installation directory
func (m *Manager) applyUpdateFiles(extractDir string) error {
	pulseDir := "/opt/pulse"
	
	// Find the extracted pulse directory
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return err
	}

	var sourceDir string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "pulse-") {
			sourceDir = filepath.Join(extractDir, entry.Name())
			break
		}
	}

	if sourceDir == "" {
		return fmt.Errorf("no pulse directory found in extract")
	}

	// Copy files, preserving structure
	cmd := exec.Command("cp", "-r", sourceDir+"/.", pulseDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy update files: %w", err)
	}

	// Set permissions
	cmd = exec.Command("chown", "-R", "pulse:pulse", pulseDir)
	if err := cmd.Run(); err != nil {
		log.Warn().Err(err).Msg("Failed to set ownership")
	}

	return nil
}

// restartService attempts to restart the Pulse service
func (m *Manager) restartService() error {
	// Try systemctl first
	cmd := exec.Command("systemctl", "restart", "pulse-backend")
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Try with sudo
	cmd = exec.Command("sudo", "systemctl", "restart", "pulse-backend")
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Try pkexec (polkit)
	cmd = exec.Command("pkexec", "systemctl", "restart", "pulse-backend")
	if err := cmd.Run(); err == nil {
		return nil
	}

	return fmt.Errorf("failed to restart service")
}

// updateStatus updates the current status
func (m *Manager) updateStatus(status string, progress int, message string) {
	m.statusMu.Lock()
	m.status = UpdateStatus{
		Status:    status,
		Progress:  progress,
		Message:   message,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	statusCopy := m.status
	m.statusMu.Unlock()

	// Send to progress channel (non-blocking)
	select {
	case m.progressChan <- statusCopy:
	default:
	}
}

// isPreV4Installation checks if this is a pre-v4 (Node.js based) installation
func isPreV4Installation() bool {
	// Check for .env file (used by Node.js version)
	if _, err := os.Stat("/opt/pulse/.env"); err == nil {
		return true
	}
	
	// Check for old service names
	cmd := exec.Command("systemctl", "list-unit-files", "--no-legend", "pulse-backend.service")
	if output, err := cmd.Output(); err == nil && len(output) > 0 {
		return true
	}
	
	// Check for Node.js artifacts
	nodeArtifacts := []string{
		"/opt/pulse/package.json",
		"/opt/pulse/node_modules",
		"/opt/pulse/server.js",
		"/opt/pulse/backend",
		"/opt/pulse/frontend",
	}
	
	for _, artifact := range nodeArtifacts {
		if _, err := os.Stat(artifact); err == nil {
			return true
		}
	}
	
	return false
}