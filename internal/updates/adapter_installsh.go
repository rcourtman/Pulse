package updates

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// InstallShAdapter wraps the install.sh script for systemd/LXC deployments
type InstallShAdapter struct {
	history        *UpdateHistory
	installScriptURL string
	logDir         string
}

// NewInstallShAdapter creates a new install.sh adapter
func NewInstallShAdapter(history *UpdateHistory) *InstallShAdapter {
	return &InstallShAdapter{
		history:        history,
		installScriptURL: "https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh",
		logDir:         "/var/log/pulse",
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
	defer logFd.Close()

	// Download install script
	progressCb(UpdateProgress{
		Stage:    "downloading",
		Progress: 10,
		Message:  "Downloading installation script...",
	})

	log.Info().
		Str("url", a.installScriptURL).
		Str("version", request.Version).
		Msg("Downloading install script")

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
	backupRe := regexp.MustCompile(`[Bb]ackup.*:\s*(.+)`)

	// Monitor output
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()

			// Write to log file
			fmt.Fprintln(logFd, line)

			// Parse for backup path
			if matches := backupRe.FindStringSubmatch(line); len(matches) > 1 {
				backupPath = strings.TrimSpace(matches[1])
			}

			// Emit progress based on output
			progress := a.parseProgress(line)
			if progress.Message != "" {
				progressCb(progress)
			}
		}
	}()

	// Also capture stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(logFd, "STDERR:", line)
		}
	}()

	// Wait for completion
	progressCb(UpdateProgress{
		Stage:    "installing",
		Progress: 50,
		Message:  "Installing update...",
	})

	err = cmd.Wait()

	if err != nil {
		// Read last few lines of log for error context
		errorDetails := a.readLastLines(logFile, 10)

		progressCb(UpdateProgress{
			Stage:      "failed",
			Progress:   0,
			Message:    "Update failed",
			IsComplete: true,
			Error:      errorDetails,
		})

		return fmt.Errorf("install script failed: %w\n%s", err, errorDetails)
	}

	progressCb(UpdateProgress{
		Stage:      "completed",
		Progress:   100,
		Message:    "Update completed successfully",
		IsComplete: true,
	})

	log.Info().
		Str("version", request.Version).
		Str("backup", backupPath).
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

	log.Info().
		Str("event_id", eventID).
		Str("backup", entry.BackupPath).
		Str("version_from", entry.VersionTo).
		Str("version_to", entry.VersionFrom).
		Msg("Starting rollback")

	// TODO: Implement actual rollback logic
	// This would involve:
	// 1. Stop service
	// 2. Restore files from backup
	// 3. Restart service
	// 4. Create rollback history entry

	return fmt.Errorf("rollback not yet implemented")
}

// downloadInstallScript downloads the install.sh script
func (a *InstallShAdapter) downloadInstallScript(ctx context.Context) (string, error) {
	// Use curl to download (simpler than http.Get for script execution)
	cmd := exec.CommandContext(ctx, "curl", "-fsSL", a.installScriptURL)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
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
	file, err := os.Open(filepath)
	if err != nil {
		return ""
	}
	defer file.Close()

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
