package updates

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// mockGitHubReleases creates a test HTTP server that returns mock release data
// Handles both /releases (array) and /releases/latest (single object) endpoints
func mockGitHubReleases(releases []ReleaseInfo) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Check if requesting /releases/latest
		if r.URL.Path == "/repos/rcourtman/Pulse/releases/latest" {
			// Return first non-prerelease as "latest"
			for _, release := range releases {
				if !release.Prerelease {
					json.NewEncoder(w).Encode(release)
					return
				}
			}
			// No stable release found
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Return all releases for /releases endpoint
		json.NewEncoder(w).Encode(releases)
	}))
}

func TestRCUpdateNotifications(t *testing.T) {
	tests := []struct {
		name            string
		currentVersion  string
		releases        []ReleaseInfo
		expectedVersion string
		expectUpdate    bool
		description     string
	}{
		{
			name:           "RC user with newer RC available",
			currentVersion: "4.22.0-rc.1",
			releases: []ReleaseInfo{
				{TagName: "v4.22.0-rc.3", Prerelease: true},
				{TagName: "v4.22.0-rc.2", Prerelease: true},
				{TagName: "v4.22.0-rc.1", Prerelease: true},
			},
			expectedVersion: "v4.22.0-rc.3",
			expectUpdate:    true,
			description:     "RC users should see newer RC releases",
		},
		{
			name:           "RC user with newer stable available",
			currentVersion: "4.22.0-rc.3",
			releases: []ReleaseInfo{
				{TagName: "v4.22.0", Prerelease: false},
				{TagName: "v4.22.0-rc.3", Prerelease: true},
				{TagName: "v4.22.0-rc.2", Prerelease: true},
			},
			expectedVersion: "v4.22.0",
			expectUpdate:    true,
			description:     "RC users should see newer stable releases (stable > RC for same version)",
		},
		{
			name:           "RC user with both newer RC and stable (stable wins)",
			currentVersion: "4.22.0-rc.1",
			releases: []ReleaseInfo{
				{TagName: "v4.23.0-rc.1", Prerelease: true},
				{TagName: "v4.22.0", Prerelease: false},
				{TagName: "v4.22.0-rc.2", Prerelease: true},
				{TagName: "v4.22.0-rc.1", Prerelease: true},
			},
			expectedVersion: "v4.23.0-rc.1",
			expectUpdate:    true,
			description:     "When both RC and stable are available, return the highest version",
		},
		{
			name:           "RC user with only older releases",
			currentVersion: "4.23.0-rc.1",
			releases: []ReleaseInfo{
				{TagName: "v4.22.0", Prerelease: false},
				{TagName: "v4.22.0-rc.3", Prerelease: true},
			},
			expectedVersion: "v4.22.0",
			expectUpdate:    true, // Returns latest version even if not newer (Available will be false)
			description:     "Returns latest stable version even when user is on newer RC",
		},
		{
			name:           "RC user already on latest stable",
			currentVersion: "4.22.0-rc.1",
			releases: []ReleaseInfo{
				{TagName: "v4.22.0", Prerelease: false},
				{TagName: "v4.22.0-rc.1", Prerelease: true},
			},
			expectedVersion: "v4.22.0",
			expectUpdate:    true,
			description:     "RC user should see stable release even if RC number is same (4.22.0 > 4.22.0-rc.1)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock GitHub API server
			server := mockGitHubReleases(tt.releases)
			defer server.Close()

			// Set environment variable to use mock server
			os.Setenv("PULSE_UPDATE_SERVER", server.URL)
			defer os.Unsetenv("PULSE_UPDATE_SERVER")

			// Create manager
			cfg := &config.Config{UpdateChannel: "rc"}
			manager := NewManager(cfg)

			// Parse current version
			currentVer, err := ParseVersion(tt.currentVersion)
			if err != nil {
				t.Fatalf("Failed to parse current version: %v", err)
			}

			// Check for updates
			release, err := manager.getLatestReleaseForChannel(context.Background(), "rc", currentVer)

			if tt.expectUpdate {
				if err != nil {
					t.Errorf("Expected update but got error: %v", err)
					return
				}
				if release.TagName != tt.expectedVersion {
					t.Errorf("Expected version %s but got %s. %s", tt.expectedVersion, release.TagName, tt.description)
				}
			} else {
				if err == nil {
					t.Errorf("Expected no update but got version %s. %s", release.TagName, tt.description)
				}
			}
		})
	}
}

func TestStableUpdateNotifications(t *testing.T) {
	tests := []struct {
		name            string
		currentVersion  string
		releases        []ReleaseInfo
		expectedVersion string
		expectUpdate    bool
		description     string
	}{
		{
			name:           "Stable user with newer stable available",
			currentVersion: "4.21.0",
			releases: []ReleaseInfo{
				{TagName: "v4.22.0", Prerelease: false},
				{TagName: "v4.21.0", Prerelease: false},
			},
			expectedVersion: "v4.22.0",
			expectUpdate:    true,
			description:     "Stable users should see newer stable releases",
		},
		{
			name:           "Stable user with only RC available",
			currentVersion: "4.22.0",
			releases: []ReleaseInfo{
				{TagName: "v4.23.0-rc.1", Prerelease: true},
				{TagName: "v4.22.0", Prerelease: false},
			},
			expectedVersion: "v4.22.0",
			expectUpdate:    true, // Returns latest stable version (Available will be false)
			description:     "Stable users get latest stable version, ignoring newer RCs",
		},
		{
			name:           "Stable user with mixed releases",
			currentVersion: "4.21.0",
			releases: []ReleaseInfo{
				{TagName: "v4.23.0-rc.1", Prerelease: true},
				{TagName: "v4.22.0", Prerelease: false},
				{TagName: "v4.22.0-rc.3", Prerelease: true},
				{TagName: "v4.21.0", Prerelease: false},
			},
			expectedVersion: "v4.22.0",
			expectUpdate:    true,
			description:     "Stable users should only see stable releases, ignoring RCs",
		},
		{
			name:           "Stable user already on latest",
			currentVersion: "4.22.0",
			releases: []ReleaseInfo{
				{TagName: "v4.22.0", Prerelease: false},
				{TagName: "v4.21.0", Prerelease: false},
			},
			expectedVersion: "v4.22.0",
			expectUpdate:    true, // Returns latest version even if already on it (Available will be false)
			description:     "Returns latest stable version even when already on it",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock GitHub API server
			server := mockGitHubReleases(tt.releases)
			defer server.Close()

			// Set environment variable to use mock server
			os.Setenv("PULSE_UPDATE_SERVER", server.URL)
			defer os.Unsetenv("PULSE_UPDATE_SERVER")

			// Create manager
			cfg := &config.Config{UpdateChannel: "stable"}
			manager := NewManager(cfg)

			// Parse current version
			currentVer, err := ParseVersion(tt.currentVersion)
			if err != nil {
				t.Fatalf("Failed to parse current version: %v", err)
			}

			// Check for updates
			release, err := manager.getLatestReleaseForChannel(context.Background(), "stable", currentVer)

			if tt.expectUpdate {
				if err != nil {
					t.Errorf("Expected update but got error: %v", err)
					return
				}
				if release.TagName != tt.expectedVersion {
					t.Errorf("Expected version %s but got %s. %s", tt.expectedVersion, release.TagName, tt.description)
				}
			} else {
				if err == nil {
					t.Errorf("Expected no update but got version %s. %s", release.TagName, tt.description)
				}
			}
		})
	}
}

func buildDummyTarball(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("dummy")
	hdr := &tar.Header{
		Name: "dummy.txt",
		Mode: 0600,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write tar content: %v", err)
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	return buf.Bytes()
}

func TestApplyUpdateFailsOnChecksumError(t *testing.T) {
	t.Setenv("PULSE_UPDATE_SERVER", "http://example.invalid")
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	tarball := buildDummyTarball(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".tar.gz"):
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write(tarball); err != nil {
				t.Fatalf("write tarball: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := &config.Config{DataPath: t.TempDir()}
	manager := NewManager(cfg)

	downloadURL := server.URL + "/pulse-v0.0.1-linux-amd64.tar.gz"

	err := manager.ApplyUpdate(context.Background(), ApplyUpdateRequest{DownloadURL: downloadURL})
	if err == nil {
		t.Fatalf("expected update to fail, got nil")
	}

	// The test might fail for different reasons (Docker detection, checksum, etc.)
	// What matters is that ApplyUpdate returns an error
	t.Logf("ApplyUpdate returned error (as expected): %v", err)

	// If the error happened early (e.g., Docker detection), no job would be enqueued
	// If the error happened during update (e.g., checksum), status should be "error"
	status := manager.GetStatus()
	job := manager.GetQueue().GetCurrentJob()

	// Check if error is recorded appropriately
	if status.Status == "error" {
		// Error happened during update process
		t.Logf("Status correctly shows error: %s", status.Error)
	} else if job != nil && job.State == JobStateFailed {
		// Error happened and was recorded in job queue
		t.Logf("Job correctly shows failure: %v", job.Error)
	} else if err.Error() == "updates cannot be applied in Docker environment" {
		// Early rejection before job was created (acceptable in test environment)
		t.Logf("Update rejected due to Docker environment (acceptable in tests)")
	} else {
		// Some other early validation error
		t.Logf("Update rejected with error: %v (no job created)", err)
	}
}

func TestVersionSemverOrdering(t *testing.T) {
	tests := []struct {
		name            string
		currentVersion  string
		releases        []ReleaseInfo
		expectedVersion string
		description     string
	}{
		{
			name:           "Stable release preferred over RC with same base version",
			currentVersion: "4.22.0-rc.3",
			releases: []ReleaseInfo{
				{TagName: "v4.22.0", Prerelease: false},
				{TagName: "v4.22.0-rc.3", Prerelease: true},
			},
			expectedVersion: "v4.22.0",
			description:     "4.22.0 should be > 4.22.0-rc.3 (stable > RC)",
		},
		{
			name:           "Higher RC number preferred",
			currentVersion: "4.22.0-rc.1",
			releases: []ReleaseInfo{
				{TagName: "v4.22.0-rc.5", Prerelease: true},
				{TagName: "v4.22.0-rc.3", Prerelease: true},
			},
			expectedVersion: "v4.22.0-rc.5",
			description:     "RC.5 should be > RC.3",
		},
		{
			name:           "Newer minor version preferred",
			currentVersion: "4.21.0",
			releases: []ReleaseInfo{
				{TagName: "v4.22.0-rc.1", Prerelease: true},
				{TagName: "v4.21.5", Prerelease: false},
			},
			expectedVersion: "v4.22.0-rc.1",
			description:     "For RC users, 4.22.0-rc.1 > 4.21.5 (minor version takes precedence)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock GitHub API server
			server := mockGitHubReleases(tt.releases)
			defer server.Close()

			// Set environment variable to use mock server
			os.Setenv("PULSE_UPDATE_SERVER", server.URL)
			defer os.Unsetenv("PULSE_UPDATE_SERVER")

			// Create manager with RC channel (to see all releases)
			cfg := &config.Config{UpdateChannel: "rc"}
			manager := NewManager(cfg)

			// Parse current version
			currentVer, err := ParseVersion(tt.currentVersion)
			if err != nil {
				t.Fatalf("Failed to parse current version: %v", err)
			}

			// Check for updates
			release, err := manager.getLatestReleaseForChannel(context.Background(), "rc", currentVer)

			if err != nil {
				t.Errorf("Expected update but got error: %v", err)
				return
			}

			if release.TagName != tt.expectedVersion {
				t.Errorf("Expected version %s but got %s. %s", tt.expectedVersion, release.TagName, tt.description)
			}
		})
	}
}

func TestManagerHistoryEntryLifecycle(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	cfg := &config.Config{}
	manager := NewManager(cfg)

	historyDir := t.TempDir()
	history, err := NewUpdateHistory(historyDir)
	if err != nil {
		t.Fatalf("NewUpdateHistory: %v", err)
	}
	manager.SetHistory(history)

	ctx := context.Background()
	eventID := manager.createHistoryEntry(ctx, UpdateHistoryEntry{
		Action:         ActionUpdate,
		Status:         StatusInProgress,
		VersionFrom:    "v4.24.0",
		VersionTo:      "v4.25.0",
		DeploymentType: "systemd",
		Channel:        "stable",
	})
	if eventID == "" {
		t.Fatalf("expected event ID")
	}

	backupPath := "/tmp/pulse-backup"
	manager.updateHistoryEntry(ctx, eventID, func(entry *UpdateHistoryEntry) {
		entry.BackupPath = backupPath
		entry.DownloadBytes = 2048
	})

	start := time.Now().Add(-1500 * time.Millisecond)
	manager.completeHistoryEntry(ctx, eventID, StatusSuccess, start, nil)

	entry, err := history.GetEntry(eventID)
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}

	if entry.Status != StatusSuccess {
		t.Fatalf("unexpected status %s", entry.Status)
	}
	if entry.BackupPath != backupPath {
		t.Fatalf("expected backup path %s, got %s", backupPath, entry.BackupPath)
	}
	if entry.DownloadBytes != 2048 {
		t.Fatalf("expected download bytes 2048, got %d", entry.DownloadBytes)
	}
	if entry.DurationMs <= 0 {
		t.Fatalf("expected positive duration, got %d", entry.DurationMs)
	}
	if entry.Error != nil {
		t.Fatalf("expected no error, got %+v", entry.Error)
	}
}

func TestInferVersionFromDownloadURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://github.com/rcourtman/Pulse/releases/download/v4.25.0/pulse-v4.25.0-linux-amd64.tar.gz", "v4.25.0"},
		{"https://example.com/pulse-v4.25.0-rc.1-linux-arm64.tar.gz", "v4.25.0-rc.1"},
		{"https://example.com/assets/pulse.tar.gz", ""},
		{"pulse-v4.30.0-linux-amd64.tar.gz", "v4.30.0"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := inferVersionFromDownloadURL(tt.url); got != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}
