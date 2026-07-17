package updates

// Pins the stranded-upgrade cases from the 2026-07 production telemetry tail:
// installs on 6.0.0-rc.x and 6.0.x must be offered the newest stable (6.0.5)
// even though the same GitHub repo interleaves v5-line maintenance releases
// (v5.1.36 shipped between v6.0.5-rc.4 and v6.0.5) and helm-chart-* tags.
// Release selection must go by version, never by GitHub's created_at order.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func withBuildVersion(t *testing.T, version string) {
	t.Helper()
	orig := BuildVersion
	BuildVersion = version
	t.Cleanup(func() { BuildVersion = orig })
}

func makeRelease(tag string, prerelease bool) ReleaseInfo {
	release := ReleaseInfo{
		TagName:     tag,
		Name:        "Pulse " + tag,
		Prerelease:  prerelease,
		PublishedAt: time.Date(2026, 7, 9, 14, 36, 38, 0, time.UTC),
	}
	for _, arch := range []string{"amd64", "arm64", "armv7", "386"} {
		release.Assets = append(release.Assets, struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		}{
			Name:               fmt.Sprintf("pulse-%s-linux-%s.tar.gz", tag, arch),
			BrowserDownloadURL: fmt.Sprintf("https://example.com/%s/pulse-%s-linux-%s.tar.gz", tag, tag, arch),
		})
	}
	return release
}

// The production release list around the 6.0.5 launch, in GitHub's
// created_at ordering. helm-chart-* releases are flagged prerelease and have
// unparseable tags; v5.1.36 is a stable release from the v5 maintenance line.
func julyReleaseList() []ReleaseInfo {
	return []ReleaseInfo{
		makeRelease("v6.0.5", false),
		makeRelease("helm-chart-6.0.5", true),
		makeRelease("v6.0.5-rc.4", true),
		makeRelease("v5.1.36", false),
		makeRelease("v6.0.4", false),
		makeRelease("v6.0.3", false),
		makeRelease("v6.0.0-rc.7", true),
		makeRelease("v6.0.0-rc.6", true),
	}
}

// A 6.0.0-rc.6 install (auto-detected rc channel, no persisted setting) must
// be offered 6.0.5 once the rc line lands on stable.
func TestStrandedUpgrade_RC6IsOfferedStable605(t *testing.T) {
	withBuildVersion(t, "6.0.0-rc.6")

	server := newReleaseServer(t, julyReleaseList(), nil)
	defer server.Close()
	t.Setenv("PULSE_UPDATE_SERVER", server.URL)

	manager := NewManager(&config.Config{})
	info, err := manager.CheckForUpdatesWithChannel(context.Background(), "")
	if err != nil {
		t.Fatalf("CheckForUpdatesWithChannel returned error: %v", err)
	}
	if !info.Available {
		t.Fatalf("expected update available for 6.0.0-rc.6, got none (latest=%q)", info.LatestVersion)
	}
	if info.LatestVersion != "6.0.5" {
		t.Fatalf("LatestVersion = %q, want 6.0.5", info.LatestVersion)
	}
	if info.IsPrerelease {
		t.Fatalf("6.0.5 must not be flagged prerelease")
	}
	if info.DownloadURL == "" || !strings.Contains(info.DownloadURL, "v6.0.5") {
		t.Fatalf("DownloadURL = %q, want a v6.0.5 asset", info.DownloadURL)
	}
}

// A 6.0.4 install on the stable channel must be offered 6.0.5 even when a
// v5-line maintenance release was created more recently and therefore sits
// ahead of it in GitHub's list ordering.
func TestStrandedUpgrade_604IsOffered605DespiteNewerV5Release(t *testing.T) {
	withBuildVersion(t, "6.0.4")

	releases := append([]ReleaseInfo{
		makeRelease("v5.1.37", false),
		makeRelease("helm-chart-5.1.37", true),
	}, julyReleaseList()...)

	server := newReleaseServer(t, releases, nil)
	defer server.Close()
	t.Setenv("PULSE_UPDATE_SERVER", server.URL)

	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	info, err := manager.CheckForUpdatesWithChannel(context.Background(), "")
	if err != nil {
		t.Fatalf("CheckForUpdatesWithChannel returned error: %v", err)
	}
	if !info.Available {
		t.Fatalf("expected update available for 6.0.4, got none (latest=%q)", info.LatestVersion)
	}
	if info.LatestVersion != "6.0.5" {
		t.Fatalf("LatestVersion = %q, want 6.0.5 (v5-line release must not mask it)", info.LatestVersion)
	}
	if info.DownloadURL == "" || !strings.Contains(info.DownloadURL, "v6.0.5") {
		t.Fatalf("DownloadURL = %q, want a v6.0.5 asset", info.DownloadURL)
	}
}

// Once the next rc line opens (6.1.0-rc.x), rc installs move onto it instead
// of the older stable.
func TestStrandedUpgrade_RCChannelPrefersNewerRCLine(t *testing.T) {
	withBuildVersion(t, "6.0.0-rc.6")

	releases := append([]ReleaseInfo{
		makeRelease("v6.1.0-rc.2", true),
		makeRelease("v6.1.0-rc.1", true),
	}, julyReleaseList()...)

	server := newReleaseServer(t, releases, nil)
	defer server.Close()
	t.Setenv("PULSE_UPDATE_SERVER", server.URL)

	manager := NewManager(&config.Config{UpdateChannel: "rc"})
	info, err := manager.CheckForUpdatesWithChannel(context.Background(), "")
	if err != nil {
		t.Fatalf("CheckForUpdatesWithChannel returned error: %v", err)
	}
	if !info.Available {
		t.Fatalf("expected update available for 6.0.0-rc.6, got none (latest=%q)", info.LatestVersion)
	}
	if info.LatestVersion != "6.1.0-rc.2" {
		t.Fatalf("LatestVersion = %q, want 6.1.0-rc.2", info.LatestVersion)
	}
	if !info.IsPrerelease {
		t.Fatalf("6.1.0-rc.2 must be flagged prerelease")
	}
}

// The exact version-string comparisons behind the stranded-upgrade cases.
func TestVersionCompare_StrandedUpgradeStrings(t *testing.T) {
	cases := []struct {
		newer, older string
	}{
		{"6.0.5", "6.0.0-rc.6"},
		{"6.0.5", "6.0.0-rc.1"},
		{"6.0.5", "6.0.0-rc.7"},
		{"6.0.5", "6.0.3"},
		{"6.0.5", "6.0.4"},
		{"6.0.5", "6.0.5-rc.4"},
		{"6.0.0-rc.7", "6.0.0-rc.6"},
		{"6.1.0-rc.2", "6.0.5"},
	}
	for _, tc := range cases {
		newer, err := ParseVersion(tc.newer)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", tc.newer, err)
		}
		older, err := ParseVersion(tc.older)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", tc.older, err)
		}
		if !newer.IsNewerThan(older) {
			t.Errorf("%s must be newer than %s", tc.newer, tc.older)
		}
		if older.IsNewerThan(newer) {
			t.Errorf("%s must not be newer than %s", tc.older, tc.newer)
		}
	}
}

// The RSS fallback must also select by version, not feed order: a v5-line
// entry published after 6.0.5 sits first in the feed.
func TestFeedFallback_SelectsHighestVersionNotFirstEntry(t *testing.T) {
	feed := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry><title>Pulse v5.1.37</title></entry>
  <entry><title>Pulse v6.0.5</title></entry>
  <entry><title>Pulse v6.0.5-rc.4</title></entry>
  <entry><title>Pulse v6.0.0-rc.7</title></entry>
</feed>`

	origTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(feed)),
			Header:     http.Header{"Content-Type": []string{"application/atom+xml"}},
			Request:    req,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = origTransport })

	manager := NewManager(&config.Config{})

	release, err := manager.getLatestReleaseFromFeed(context.Background(), "stable")
	if err != nil {
		t.Fatalf("stable feed error: %v", err)
	}
	if release.TagName != "v6.0.5" {
		t.Fatalf("stable feed tag = %q, want v6.0.5", release.TagName)
	}

	release, err = manager.getLatestReleaseFromFeed(context.Background(), "rc")
	if err != nil {
		t.Fatalf("rc feed error: %v", err)
	}
	if release.TagName != "v6.0.5" {
		t.Fatalf("rc feed tag = %q, want v6.0.5 (stable outranks its own rc)", release.TagName)
	}
}
