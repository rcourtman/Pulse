package updates

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// InstallShAdapter wraps the install.sh script for systemd/LXC deployments
type InstallShAdapter struct {
	history          *UpdateHistory
	installScriptURL string
	logDir           string
}

// NewInstallShAdapter creates a new install.sh adapter
func NewInstallShAdapter(history *UpdateHistory) *InstallShAdapter {
	return &InstallShAdapter{
		history:          history,
		installScriptURL: "https://github.com/rcourtman/Pulse/releases/latest/download/install.sh",
		logDir:           "/var/log/pulse",
	}
}

// SupportsApply returns true for systemd and proxmoxve deployments
func (a *InstallShAdapter) SupportsApply() bool {
	return true
}

// GetDeploymentType returns the deployment type
func (a *InstallShAdapter) GetDeploymentType() string {
	return "systemd" // Can be "systemd" or "proxmoxve"
}

// PrepareUpdate returns update plan information
func (a *InstallShAdapter) PrepareUpdate(ctx context.Context, request UpdateRequest) (*UpdatePlan, error) {
	plan := &UpdatePlan{
		CanAutoUpdate:   true,
		RequiresRoot:    true,
		RollbackSupport: true,
		EstimatedTime:   "2-5 minutes",
		Instructions: []string{
			fmt.Sprintf("Download and install Pulse %s", request.Version),
			"Create backup of current installation",
			"Extract and apply update",
			"Restart Pulse service",
		},
		Prerequisites: []string{
			"Root access (sudo)",
			"Internet connection",
			"At least 100MB free disk space",
		},
	}

	return plan, nil
}

// Execute performs the update by calling install.sh
func (a *InstallShAdapter) Execute(ctx context.Context, request UpdateRequest, progressCb ProgressCallback) error {
	// Ensure log directory exists
	if err := os.MkdirAll(a.logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create log file
	logFile := filepath.Join(a.logDir, fmt.Sprintf("update-%s.log", time.Now().Format("20060102-150405")))
	logFd, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer func() {
		if closeErr := logFd.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Str("log_file", logFile).Msg("Failed to close update log file")
		}
	}()

	// Download install script
	progressCb(UpdateProgress{
		Stage:    "downloading",
		Progress: 10,
		Message:  "Downloading installation script...",
	})

	log.Info().
		Str("url", a.installScriptURL).
		Str("version", request.Version).
		Msg("downloading install script")

	installScript, err := a.downloadInstallScript(ctx)
	if err != nil {
		return fmt.Errorf("failed to download install script: %w", err)
	}

	// Prepare command
	progressCb(UpdateProgress{
		Stage:    "preparing",
		Progress: 20,
		Message:  "Preparing update...",
	})

	// Validate version string to prevent command injection
	// Version must match semantic versioning format (with optional 'v' prefix)
	versionPattern := regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?(?:\+[a-zA-Z0-9.-]+)?$`)
	if !versionPattern.MatchString(request.Version) {
		return fmt.Errorf("invalid version format: %s", request.Version)
	}

	// Build command: bash install.sh --version vX.Y.Z
	args := []string{"-s", "--", "--version", request.Version}
	if request.Force {
		args = append(args, "--force")
	}

	cmd := exec.CommandContext(ctx, "bash", args...)
	cmd.Stdin = strings.NewReader(installScript)

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start install script: %w", err)
	}

	// Track backup path from output
	var backupPath string
	var backupPathMu sync.RWMutex
	var progressMu sync.Mutex
	emitProgress := func(progress UpdateProgress) {
		progressMu.Lock()
		defer progressMu.Unlock()
		progressCb(progress)
	}

	backupRe := regexp.MustCompile(`[Bb]ackup.*:\s*(.+)`)
	var streamWG sync.WaitGroup

	// Monitor output
	streamWG.Add(1)
	go func() {
		defer streamWG.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()

			// Write to log file
			if _, writeErr := fmt.Fprintln(logFd, line); writeErr != nil {
				log.Warn().Err(writeErr).Str("log_file", logFile).Msg("Failed to write update stdout line to log")
			}

			// Parse for backup path
			if matches := backupRe.FindStringSubmatch(line); len(matches) > 1 {
				backupPathMu.Lock()
				backupPath = strings.TrimSpace(matches[1])
				backupPathMu.Unlock()
			}

			// Emit progress based on output
			progress := a.parseProgress(line)
			if progress.Message != "" {
				emitProgress(progress)
			}
		}
		if err := scanner.Err(); err != nil {
			log.Debug().Err(err).Msg("Failed scanning install.sh stdout")
		}
	}()

	// Also capture stderr
	streamWG.Add(1)
	go func() {
		defer streamWG.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if _, writeErr := fmt.Fprintln(logFd, "STDERR:", line); writeErr != nil {
				log.Warn().Err(writeErr).Str("log_file", logFile).Msg("Failed to write update stderr line to log")
			}
		}
		if err := scanner.Err(); err != nil {
			log.Debug().Err(err).Msg("Failed scanning install.sh stderr")
		}
	}()

	// Wait for completion
	emitProgress(UpdateProgress{
		Stage:    "installing",
		Progress: 50,
		Message:  "Installing update...",
	})

	err = cmd.Wait()
	streamWG.Wait()

	backupPathMu.RLock()
	finalBackupPath := backupPath
	backupPathMu.RUnlock()

	if err != nil {
		// Read last few lines of log for error context
		errorDetails := a.readLastLines(logFile, 10)

		emitProgress(UpdateProgress{
			Stage:      "failed",
			Progress:   0,
			Message:    "Update failed",
			IsComplete: true,
			Error:      errorDetails,
		})

		return fmt.Errorf("install script failed: %w\n%s", err, errorDetails)
	}

	emitProgress(UpdateProgress{
		Stage:      "completed",
		Progress:   100,
		Message:    "Update completed successfully",
		IsComplete: true,
	})

	log.Info().
		Str("version", request.Version).
		Str("backup", finalBackupPath).
		Str("log", logFile).
		Msg("Update completed successfully")

	return nil
}

// Rollback rolls back to a previous version
func (a *InstallShAdapter) Rollback(ctx context.Context, eventID string) error {
	// Get the event from history
	entry, err := a.history.GetEntry(eventID)
	if err != nil {
		return fmt.Errorf("failed to get history entry: %w", err)
	}

	if entry.BackupPath == "" {
		return fmt.Errorf("no backup path available for event %s", eventID)
	}

	// Check if backup exists
	if _, err := os.Stat(entry.BackupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", entry.BackupPath)
	}

	targetVersion := entry.VersionFrom
	if targetVersion == "" {
		return fmt.Errorf("no target version available in history")
	}

	log.Info().
		Str("event_id", eventID).
		Str("backup", entry.BackupPath).
		Str("current_version", entry.VersionTo).
		Str("target_version", targetVersion).
		Msg("starting rollback")

	// Create rollback history entry
	rollbackEventID, err := a.history.CreateEntry(ctx, UpdateHistoryEntry{
		Action:         "rollback",
		VersionFrom:    entry.VersionTo,
		VersionTo:      targetVersion,
		DeploymentType: a.GetDeploymentType(),
		InitiatedBy:    InitiatedByUser,
		InitiatedVia:   InitiatedViaCLI,
		Status:         StatusInProgress,
		RelatedEventID: eventID,
		Notes:          fmt.Sprintf("Rolling back update %s", eventID),
	})
	if err != nil {
		return fmt.Errorf("failed to create rollback history entry: %w", err)
	}

	rollbackErr := a.executeRollback(ctx, entry, targetVersion)

	// Update rollback history
	finalStatus := StatusSuccess
	var updateError *UpdateError
	if rollbackErr != nil {
		finalStatus = StatusFailed
		updateError = &UpdateError{
			Message: rollbackErr.Error(),
			Code:    "rollback_failed",
		}
	}

	if err := a.history.UpdateEntry(ctx, rollbackEventID, func(e *UpdateHistoryEntry) error {
		e.Status = finalStatus
		e.Error = updateError
		return nil
	}); err != nil {
		log.Warn().
			Err(err).
			Str("event_id", rollbackEventID).
			Msg("Failed to update rollback history entry")
	}

	return rollbackErr
}

// executeRollback performs the actual rollback operation
func (a *InstallShAdapter) executeRollback(ctx context.Context, entry *UpdateHistoryEntry, targetVersion string) error {
	// Step 1: Detect service name
	serviceName, err := a.detectServiceName()
	if err != nil {
		return fmt.Errorf("failed to detect service name: %w", err)
	}

	log.Info().Str("service", serviceName).Msg("detected Pulse service")

	// Step 2: Download old binary
	log.Info().Str("version", targetVersion).Msg("downloading old binary")
	binaryPath, err := a.downloadBinary(ctx, targetVersion)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	defer os.RemoveAll(filepath.Dir(binaryPath))

	// Step 3: Stop service
	log.Info().Msg("stopping Pulse service")
	if err := a.stopService(ctx, serviceName); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	// Step 4: Backup current config (safety)
	configDir := "/etc/pulse"
	safetyBackup := fmt.Sprintf("%s.rollback-safety.%s", configDir, time.Now().Format("20060102-150405"))
	log.Info().Str("backup", safetyBackup).Msg("creating safety backup of current config")
	if err := exec.CommandContext(ctx, "cp", "-a", configDir, safetyBackup).Run(); err != nil {
		log.Warn().Err(err).Msg("failed to create safety backup")
	}

	// Step 5: Restore config from backup
	log.Info().Str("source", entry.BackupPath).Msg("restoring configuration")
	if err := a.restoreConfig(ctx, entry.BackupPath, configDir); err != nil {
		// Try to start service anyway
		if startErr := a.startService(ctx, serviceName); startErr != nil {
			log.Warn().
				Err(startErr).
				Str("service", serviceName).
				Msg("Failed to restart Pulse service after configuration restore failure")
		}
		return fmt.Errorf("failed to restore config: %w", err)
	}

	// Step 6: Install old binary
	log.Info().Str("version", targetVersion).Msg("installing old binary")
	installDir := "/opt/pulse/bin/pulse"
	if err := a.installBinary(ctx, binaryPath, installDir); err != nil {
		// Try to start service anyway
		if startErr := a.startService(ctx, serviceName); startErr != nil {
			log.Warn().
				Err(startErr).
				Str("service", serviceName).
				Msg("Failed to restart Pulse service after binary install failure")
		}
		return fmt.Errorf("failed to install binary: %w", err)
	}

	// Step 7: Start service
	log.Info().Msg("starting Pulse service")
	if err := a.startService(ctx, serviceName); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Step 8: Health check
	log.Info().Msg("verifying service health")
	if err := a.waitForHealth(ctx, 30*time.Second); err != nil {
		return fmt.Errorf("service health check failed: %w", err)
	}

	log.Info().Str("version", targetVersion).Msg("rollback completed successfully")
	return nil
}

// detectServiceName detects the active Pulse service name
func (a *InstallShAdapter) detectServiceName() (string, error) {
	candidates := []string{"pulse", "pulse-backend", "pulse-hot-dev"}

	for _, name := range candidates {
		cmd := exec.Command("systemctl", "is-active", name)
		if output, err := cmd.Output(); err == nil {
			status := strings.TrimSpace(string(output))
			if status == "active" || status == "activating" {
				return name, nil
			}
		}
	}

	// Default to "pulse" if none are active
	return "pulse", nil
}

// downloadBinary downloads a specific version binary from GitHub
func (a *InstallShAdapter) downloadBinary(ctx context.Context, version string) (string, error) {
	// Ensure version has 'v' prefix
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	// Determine architecture
	arch := "amd64"
	if _, err := os.Stat("/proc/cpuinfo"); err == nil {
		output, cmdErr := exec.Command("uname", "-m").Output()
		if cmdErr != nil {
			log.Debug().Err(cmdErr).Msg("Failed to detect CPU architecture with uname -m; using amd64")
		} else {
			machine := strings.TrimSpace(string(output))
			if machine == "aarch64" || machine == "arm64" {
				arch = "arm64"
			}
		}
	}

	// Download URL - tarball with version in filename
	tarballName := fmt.Sprintf("pulse-%s-linux-%s.tar.gz", version, arch)
	url := fmt.Sprintf("https://github.com/rcourtman/Pulse/releases/download/%s/%s", version, tarballName)

	// Create temp file
	tmpDir, err := os.MkdirTemp("", "pulse-rollback-*")
	if err != nil {
		return "", fmt.Errorf("failed to create rollback temp directory: %w", err)
	}

	tarballPath := filepath.Join(tmpDir, tarballName)

	// Download tarball
	cmd := exec.CommandContext(ctx, "curl", "-fsSL", "-o", tarballPath, url)
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("download failed: %w", err)
	}

	// Download checksum
	checksumURL := url + ".sha256"
	checksumPath := tarballPath + ".sha256"
	cmd = exec.CommandContext(ctx, "curl", "-fsSL", "-o", checksumPath, checksumURL)
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("failed to download checksum: %w", err)
	}

	checksumData, err := os.ReadFile(checksumPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("failed to read checksum file: %w", err)
	}

	expectedHash := strings.Fields(strings.TrimSpace(string(checksumData)))
	if len(expectedHash) == 0 {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("checksum file was empty")
	}

	file, err := os.Open(tarballPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("failed to open downloaded tarball: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Str("path", tarballPath).Msg("Failed to close downloaded tarball")
		}
	}()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("failed to hash downloaded binary: %w", err)
	}
	actualHash := hex.EncodeToString(hasher.Sum(nil))

	if !strings.EqualFold(actualHash, expectedHash[0]) {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("checksum verification failed for %s", tarballName)
	}

	if removeErr := os.Remove(checksumPath); removeErr != nil && !os.IsNotExist(removeErr) {
		log.Debug().Err(removeErr).Str("path", checksumPath).Msg("Failed to remove checksum file")
	}

	// Extract tarball to get the binary
	extractDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("failed to create extract directory: %w", err)
	}

	// Extract: tar -xzf tarball -C extractDir
	cmd = exec.CommandContext(ctx, "tar", "-xzf", tarballPath, "-C", extractDir)
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("failed to extract tarball: %w", err)
	}

	// The binary is at extractDir/bin/pulse
	binaryPath := filepath.Join(extractDir, "bin", "pulse")
	if _, err := os.Stat(binaryPath); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("binary not found in tarball at expected path: %w", err)
	}

	// Remove tarball to save space
	if removeErr := os.Remove(tarballPath); removeErr != nil && !os.IsNotExist(removeErr) {
		log.Debug().Err(removeErr).Str("path", tarballPath).Msg("Failed to remove downloaded tarball")
	}

	return binaryPath, nil
}

// stopService stops the Pulse service
func (a *InstallShAdapter) stopService(ctx context.Context, serviceName string) error {
	cmd := exec.CommandContext(ctx, "systemctl", "stop", serviceName)
	return cmd.Run()
}

// startService starts the Pulse service
func (a *InstallShAdapter) startService(ctx context.Context, serviceName string) error {
	cmd := exec.CommandContext(ctx, "systemctl", "start", serviceName)
	return cmd.Run()
}

// restoreConfig restores configuration from backup
func (a *InstallShAdapter) restoreConfig(ctx context.Context, backupPath, targetPath string) error {
	// Remove current config
	if err := os.RemoveAll(targetPath); err != nil {
		return fmt.Errorf("failed to remove current config: %w", err)
	}

	// Copy backup to target
	cmd := exec.CommandContext(ctx, "cp", "-a", backupPath, targetPath)
	return cmd.Run()
}

// installBinary installs a binary to the target location
func (a *InstallShAdapter) installBinary(ctx context.Context, sourcePath, targetPath string) error {
	// Backup current binary
	if _, err := os.Stat(targetPath); err == nil {
		backupPath := targetPath + ".pre-rollback"
		if renameErr := os.Rename(targetPath, backupPath); renameErr != nil {
			log.Warn().
				Err(renameErr).
				Str("target", targetPath).
				Str("backup", backupPath).
				Msg("Failed to back up existing binary before install")
		}
	}

	// Copy new binary
	if err := exec.CommandContext(ctx, "cp", sourcePath, targetPath).Run(); err != nil {
		return fmt.Errorf("copy binary to %s: %w", targetPath, err)
	}

	// Set permissions
	if err := os.Chmod(targetPath, 0755); err != nil {
		return fmt.Errorf("set permissions on %s: %w", targetPath, err)
	}

	// Set ownership
	if err := exec.CommandContext(ctx, "chown", "pulse:pulse", targetPath).Run(); err != nil {
		return fmt.Errorf("set ownership on %s: %w", targetPath, err)
	}
	return nil
}

// waitForHealth waits for the service to become healthy
func (a *InstallShAdapter) waitForHealth(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Try to hit health endpoint
		cmd := exec.CommandContext(ctx, "curl", "-fsS", "http://localhost:7655/api/health")
		if err := cmd.Run(); err == nil {
			return nil
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return fmt.Errorf("service did not become healthy within %v", timeout)
}

// downloadInstallScript downloads the install.sh script
func (a *InstallShAdapter) downloadInstallScript(ctx context.Context) (string, error) {
	tmpDir, err := os.MkdirTemp("", "pulse-installsh-*")
	if err != nil {
		return "", fmt.Errorf("failed to create install.sh temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "install.sh")

	cmd := exec.CommandContext(ctx, "curl", "-fsSL", "-o", scriptPath, a.installScriptURL)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to download install.sh: %w", err)
	}

	checksumURL := a.installScriptURL + ".sha256"
	checksumPath := scriptPath + ".sha256"
	cmd = exec.CommandContext(ctx, "curl", "-fsSL", "-o", checksumPath, checksumURL)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to download install.sh checksum: %w", err)
	}

	checksumData, err := os.ReadFile(checksumPath)
	if err != nil {
		return "", fmt.Errorf("failed to read install.sh checksum: %w", err)
	}

	expectedParts := strings.Fields(strings.TrimSpace(string(checksumData)))
	if len(expectedParts) == 0 {
		return "", fmt.Errorf("install.sh checksum file was empty")
	}

	file, err := os.Open(scriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to open install.sh for hashing: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Str("path", scriptPath).Msg("Failed to close install.sh file")
		}
	}()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to hash install.sh: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actualHash, expectedParts[0]) {
		return "", fmt.Errorf("install.sh checksum verification failed")
	}

	content, err := os.ReadFile(scriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read install.sh: %w", err)
	}

	return string(content), nil
}

// parseProgress attempts to parse progress from install script output
func (a *InstallShAdapter) parseProgress(line string) UpdateProgress {
	line = strings.ToLower(line)

	// Map common install.sh output to progress stages
	patterns := map[string]UpdateProgress{
		"downloading": {Stage: "downloading", Progress: 30, Message: "Downloading update..."},
		"extracting":  {Stage: "extracting", Progress: 40, Message: "Extracting files..."},
		"installing":  {Stage: "installing", Progress: 60, Message: "Installing..."},
		"backup":      {Stage: "backing-up", Progress: 25, Message: "Creating backup..."},
		"configur":    {Stage: "configuring", Progress: 70, Message: "Configuring..."},
		"restart":     {Stage: "restarting", Progress: 90, Message: "Restarting service..."},
		"complet":     {Stage: "completed", Progress: 100, Message: "Update completed", IsComplete: true},
		"success":     {Stage: "completed", Progress: 100, Message: "Update completed", IsComplete: true},
	}

	for pattern, progress := range patterns {
		if strings.Contains(line, pattern) {
			return progress
		}
	}

	return UpdateProgress{}
}

// readLastLines reads the last N lines from a file
func (a *InstallShAdapter) readLastLines(filepath string, n int) string {
	if n <= 0 {
		return ""
	}

	file, err := os.Open(filepath)
	if err != nil {
		return ""
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.Debug().Err(closeErr).Str("path", filepath).Msg("Failed to close log file while reading last lines")
		}
	}()

	// Read file backwards (simplified approach - read all lines and take last N)
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) == 0 {
		return ""
	}

	start := len(lines) - n
	if start < 0 {
		start = 0
	}

	return strings.Join(lines[start:], "\n")
}

// DockerUpdater provides instructions for Docker deployments
type DockerUpdater struct{}

// NewDockerUpdater creates an updater for Docker deployments.
func NewDockerUpdater() *DockerUpdater {
	return &DockerUpdater{}
}

func (u *DockerUpdater) SupportsApply() bool {
	return false
}

func (u *DockerUpdater) GetDeploymentType() string {
	return "docker"
}

func (u *DockerUpdater) PrepareUpdate(ctx context.Context, request UpdateRequest) (*UpdatePlan, error) {
	return &UpdatePlan{
		CanAutoUpdate: false,
		Instructions: []string{
			fmt.Sprintf("docker pull rcourtman/pulse:%s", strings.TrimPrefix(request.Version, "v")),
			"docker stop pulse",
			fmt.Sprintf("docker run -d --name pulse rcourtman/pulse:%s", strings.TrimPrefix(request.Version, "v")),
		},
		RequiresRoot:    false,
		RollbackSupport: true,
		EstimatedTime:   "1-2 minutes",
	}, nil
}

func (u *DockerUpdater) Execute(ctx context.Context, request UpdateRequest, progressCb ProgressCallback) error {
	return fmt.Errorf("docker deployments do not support automated updates")
}

func (u *DockerUpdater) Rollback(ctx context.Context, eventID string) error {
	return fmt.Errorf("docker rollback not supported via API")
}

// AURUpdater provides instructions for Arch Linux AUR deployments
type AURUpdater struct{}

// NewAURUpdater creates an updater for Arch Linux AUR deployments.
func NewAURUpdater() *AURUpdater {
	return &AURUpdater{}
}

func (u *AURUpdater) SupportsApply() bool {
	return false
}

func (u *AURUpdater) GetDeploymentType() string {
	return "aur"
}

func (u *AURUpdater) PrepareUpdate(ctx context.Context, request UpdateRequest) (*UpdatePlan, error) {
	return &UpdatePlan{
		CanAutoUpdate: false,
		Instructions: []string{
			"yay -Syu pulse-monitoring",
			"# or",
			"paru -Syu pulse-monitoring",
		},
		RequiresRoot:    false,
		RollbackSupport: false,
		EstimatedTime:   "1-2 minutes",
	}, nil
}

func (u *AURUpdater) Execute(ctx context.Context, request UpdateRequest, progressCb ProgressCallback) error {
	return fmt.Errorf("aur deployments must be updated via package manager")
}

func (u *AURUpdater) Rollback(ctx context.Context, eventID string) error {
	return fmt.Errorf("aur rollback not supported")
}

// Ensure adapters implement Updater interface
var (
	_ Updater = (*InstallShAdapter)(nil)
	_ Updater = (*DockerUpdater)(nil)
	_ Updater = (*AURUpdater)(nil)
)
