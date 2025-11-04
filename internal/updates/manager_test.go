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

	err := manager.ApplyUpdate(context.Background(), downloadURL)
	if err == nil {
		t.Fatalf("expected checksum failure error, got nil")
	}

	status := manager.GetStatus()
	if status.Status != "error" {
		t.Fatalf("expected status error, got %q", status.Status)
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
