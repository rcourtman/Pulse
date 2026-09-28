package updates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func newReleaseServer(t *testing.T, releases []ReleaseInfo, hitCount *int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != updateReleaseAPIPath() {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if hitCount != nil {
			atomic.AddInt32(hitCount, 1)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases)
	}))
}

func TestCheckForUpdatesWithChannel_UsesConfiguredRepoPath(t *testing.T) {
	var hits int32
	releases := []ReleaseInfo{
		{
			TagName:     "v99.0.0",
			Name:        "v99.0.0",
			Prerelease:  false,
			PublishedAt: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		},
	}

	t.Setenv("PULSE_GITHUB_REPO", "example/pulse-fork")
	server := newReleaseServer(t, releases, &hits)
	defer server.Close()

	t.Setenv("PULSE_UPDATE_SERVER", server.URL)

	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	if _, err := manager.CheckForUpdatesWithChannel(context.Background(), ""); err != nil {
		t.Fatalf("CheckForUpdatesWithChannel returned error: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected 1 request, got %d", got)
	}
}

func TestCheckForUpdatesWithChannel_SourceBuild(t *testing.T) {
	markerPath := "BUILD_FROM_SOURCE"
	if err := os.WriteFile(markerPath, []byte("1"), 0644); err != nil {
		t.Fatalf("write %s: %v", markerPath, err)
	}
	t.Cleanup(func() {
		_ = os.Remove(markerPath)
	})

	manager := NewManager(&config.Config{UpdateChannel: "stable"})

	info, err := manager.CheckForUpdatesWithChannel(context.Background(), "")
	if err != nil {
		t.Fatalf("CheckForUpdatesWithChannel returned error: %v", err)
	}
	if info.Available {
		t.Fatalf("expected no updates for source build, got available")
	}
	if info.LatestVersion != info.CurrentVersion {
		t.Fatalf("LatestVersion = %q, want %q", info.LatestVersion, info.CurrentVersion)
	}
}

func TestCheckForUpdatesWithChannel_AvailableUsesCache(t *testing.T) {
	var hits int32
	releaseTime := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	releases := []ReleaseInfo{
		{
			TagName:     "v99.0.0",
			Name:        "v99.0.0",
			Body:        "Release notes",
			Prerelease:  false,
			PublishedAt: releaseTime,
			Assets:      []ReleaseAsset{runtimeArchiveFixture(t, "v99.0.0")},
		},
	}

	server := newReleaseServer(t, releases, &hits)
	defer server.Close()

	t.Setenv("PULSE_UPDATE_SERVER", server.URL)

	manager := NewManager(&config.Config{UpdateChannel: "stable"})

	info, err := manager.CheckForUpdatesWithChannel(context.Background(), "")
	if err != nil {
		t.Fatalf("CheckForUpdatesWithChannel returned error: %v", err)
	}
	if !info.Available {
		t.Fatalf("expected update to be available")
	}
	if info.LatestVersion != "99.0.0" {
		t.Fatalf("LatestVersion = %q, want 99.0.0", info.LatestVersion)
	}
	if info.DownloadURL == "" {
		t.Fatalf("DownloadURL not set")
	}

	info2, err := manager.CheckForUpdatesWithChannel(context.Background(), "")
	if err != nil {
		t.Fatalf("CheckForUpdatesWithChannel second call error: %v", err)
	}
	if info2.LatestVersion != info.LatestVersion {
		t.Fatalf("cached LatestVersion = %q, want %q", info2.LatestVersion, info.LatestVersion)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected 1 request, got %d", got)
	}
}

func TestCheckForUpdatesWithChannel_NoReleases(t *testing.T) {
	var hits int32
	releases := []ReleaseInfo{
		{
			TagName:     "v99.0.0-rc.1",
			Name:        "v99.0.0-rc.1",
			Prerelease:  true,
			PublishedAt: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		},
	}

	server := newReleaseServer(t, releases, &hits)
	defer server.Close()

	t.Setenv("PULSE_UPDATE_SERVER", server.URL)

	manager := NewManager(&config.Config{UpdateChannel: "stable"})

	info, err := manager.CheckForUpdatesWithChannel(context.Background(), "")
	if err != nil {
		t.Fatalf("CheckForUpdatesWithChannel returned error: %v", err)
	}
	if info.Available {
		t.Fatalf("expected no updates for stable channel with only prereleases")
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected 1 request, got %d", got)
	}
}

func TestCheckForUpdates_Wrapper(t *testing.T) {
	var hits int32
	releases := []ReleaseInfo{
		{
			TagName:     "v99.1.0",
			Name:        "v99.1.0",
			Body:        "Release notes",
			Prerelease:  false,
			PublishedAt: time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC),
			Assets:      []ReleaseAsset{runtimeArchiveFixture(t, "v99.1.0")},
		},
	}

	server := newReleaseServer(t, releases, &hits)
	defer server.Close()

	t.Setenv("PULSE_UPDATE_SERVER", server.URL)

	manager := NewManager(&config.Config{UpdateChannel: "stable"})

	info, err := manager.CheckForUpdates(context.Background())
	if err != nil {
		t.Fatalf("CheckForUpdates returned error: %v", err)
	}
	if !info.Available {
		t.Fatalf("expected update to be available")
	}
	if info.LatestVersion != "99.1.0" {
		t.Fatalf("LatestVersion = %q, want 99.1.0", info.LatestVersion)
	}
}

// runtimeArchiveFixture returns the server archive the update check requires
// for the architecture running the test. The check offers an update only when
// that exact archive exists, so a hard-coded linux-amd64 fixture made these
// tests fail on arm64 hosts.
func runtimeArchiveFixture(t *testing.T, tag string) ReleaseAsset {
	t.Helper()
	asset, ok := updateReleaseAssetForRuntime(tag)
	if !ok {
		t.Skipf("no Pulse server archive is published for %s", runtime.GOARCH)
	}
	return ReleaseAsset{Name: asset.Name, BrowserDownloadURL: "https://example.com/" + asset.Name}
}
