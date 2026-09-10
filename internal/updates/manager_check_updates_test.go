package updates

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
	// Exercise published-release behaviour independently of the checkout identity.
	withBuildVersion(t, "6.4.0")
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
	withBuildVersion(t, "6.4.0")
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

func TestCheckForUpdatesWithChannel_DiagnosticBuild(t *testing.T) {
	oldBuildVersion := BuildVersion
	BuildVersion = "6.4.1+test.1913.bd37ae18"
	t.Cleanup(func() { BuildVersion = oldBuildVersion })
	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	info, err := manager.CheckForUpdatesWithChannel(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if info.Available || info.CurrentVersion != BuildVersion || info.LatestVersion != BuildVersion {
		t.Fatalf("diagnostic build must not enter release update flow: %+v", info)
	}
}

func TestCheckForUpdatesWithChannel_AvailableUsesCache(t *testing.T) {
	// Exercise published-release behaviour independently of the checkout identity.
	withBuildVersion(t, "6.4.0")
	var hits int32
	releaseTime := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	releases := []ReleaseInfo{
		{
			TagName:     "v99.0.0",
			Name:        "v99.0.0",
			Body:        "Release notes",
			Prerelease:  false,
			PublishedAt: releaseTime,
			Assets: []ReleaseAsset{
				{
					Name:               "pulse-v99.0.0-linux-amd64.tar.gz",
					BrowserDownloadURL: "https://example.com/pulse-v99.0.0-linux-amd64.tar.gz",
				},
			},
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
	// Exercise published-release behaviour independently of the checkout identity.
	withBuildVersion(t, "6.4.0")
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
	// Exercise published-release behaviour independently of the checkout identity.
	withBuildVersion(t, "6.4.0")
	var hits int32
	releases := []ReleaseInfo{
		{
			TagName:     "v99.1.0",
			Name:        "v99.1.0",
			Body:        "Release notes",
			Prerelease:  false,
			PublishedAt: time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC),
			Assets: []ReleaseAsset{
				{
					Name:               "pulse-v99.1.0-linux-amd64.tar.gz",
					BrowserDownloadURL: "https://example.com/pulse-v99.1.0-linux-amd64.tar.gz",
				},
			},
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

func TestForcedUpdateCheckRefreshesSavedChannelCache(t *testing.T) {
	// Exercise published-release behaviour independently of the checkout identity.
	withBuildVersion(t, "6.4.0")
	var hits int32
	server := newReleaseServer(t, []ReleaseInfo{{TagName: "v99.0.0", Assets: []ReleaseAsset{{Name: "pulse-v99.0.0-linux-amd64.tar.gz", BrowserDownloadURL: "https://example.com/new.tar.gz"}}}}, &hits)
	defer server.Close()
	t.Setenv("PULSE_UPDATE_SERVER", server.URL)
	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	manager.checkCache["stable"] = &UpdateInfo{LatestVersion: "98.0.0"}
	manager.cacheTime["stable"] = time.Now()
	cached, err := manager.CheckForUpdates(context.Background())
	if err != nil || cached.LatestVersion != "98.0.0" || hits != 0 {
		t.Fatalf("cached check: %+v %v hits=%d", cached, err, hits)
	}
	fresh, err := manager.CheckForUpdatesWithOptions(context.Background(), UpdateCheckOptions{Force: true})
	if err != nil || fresh.LatestVersion != "99.0.0" {
		t.Fatalf("fresh check: %+v %v", fresh, err)
	}
	again, err := manager.CheckForUpdates(context.Background())
	if err != nil || again.LatestVersion != "99.0.0" || atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("refreshed cache: %+v %v hits=%d", again, err, hits)
	}
}

func TestUpdateInfoReleaseDateOmitsUnknownTime(t *testing.T) {
	for _, date := range []time.Time{{}, time.Date(2026, 9, 10, 14, 6, 1, 0, time.UTC)} {
		raw, err := json.Marshal(UpdateInfo{ReleaseDate: date})
		if err != nil {
			t.Fatal(err)
		}
		var wire map[string]any
		if err := json.Unmarshal(raw, &wire); err != nil {
			t.Fatal(err)
		}
		value, present := wire["releaseDate"]
		if present == date.IsZero() {
			t.Fatalf("unexpected date presence: %s", raw)
		}
		if present && value != date.Format(time.RFC3339) {
			t.Fatalf("date changed: %v", value)
		}
	}
}

func TestForcedUpdateCheckDoesNotReturnCachedResultOnRateLimit(t *testing.T) {
	setRetrySettingsForTest(t, 1, time.Millisecond, time.Millisecond)
	withBuildVersion(t, "6.4.0")
	t.Setenv("PULSE_UPDATE_SERVER", "")
	original := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		status := http.StatusNotFound
		body := "not found"
		header := http.Header{}
		if req.URL.Host == "api.github.com" {
			status = http.StatusForbidden
			body = `{"message":"API rate limit exceeded"}`
			header.Set("X-RateLimit-Remaining", "0")
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: header, Request: req}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = original })
	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	old := &UpdateInfo{LatestVersion: "6.4.1"}
	at := time.Now()
	manager.checkCache["stable"] = old
	manager.cacheTime["stable"] = at
	info, err := manager.CheckForUpdatesWithOptions(context.Background(), UpdateCheckOptions{Force: true})
	if err == nil || info != nil {
		t.Fatalf("forced failure returned %+v, %v", info, err)
	}
	if manager.checkCache["stable"] != old || !manager.cacheTime["stable"].Equal(at) {
		t.Fatal("failure replaced cache or freshness")
	}
	if manager.GetStatus().Status != "error" {
		t.Fatalf("status=%+v", manager.GetStatus())
	}
}
