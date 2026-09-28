package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

// issue2282ReleaseJSON renders a release shaped like GitHub's real payload:
// hundreds of assets, each with a nested uploader object, so a 30-release page
// is several megabytes — the shape that broke the 1 MiB buffered decode.
func issue2282ReleaseJSON(t *testing.T, tag string, prerelease bool, assetCount int) map[string]any {
	t.Helper()
	uploader := map[string]any{
		"login": "rcourtman", "id": 1, "node_id": strings.Repeat("n", 40),
		"avatar_url": "https://avatars.githubusercontent.com/u/1?v=4",
		"url":        "https://api.github.com/users/rcourtman",
		"html_url":   "https://github.com/rcourtman",
		"type":       "User", "site_admin": false,
	}
	assets := make([]map[string]any, 0, assetCount+1)
	for i := 0; i < assetCount; i++ {
		name := fmt.Sprintf("pulse-agent-%s-component-%03d.zip", tag, i)
		assets = append(assets, map[string]any{
			"url":                  "https://api.github.com/repos/rcourtman/Pulse/releases/assets/1",
			"id":                   i,
			"name":                 name,
			"label":                nil,
			"uploader":             uploader,
			"content_type":         "application/zip",
			"state":                "uploaded",
			"size":                 123456,
			"download_count":       42,
			"browser_download_url": "https://github.com/rcourtman/Pulse/releases/download/" + tag + "/" + name,
		})
	}
	if runtimeAsset, ok := updateReleaseAssetForRuntime(tag); ok {
		assets = append(assets, map[string]any{
			"name":                 runtimeAsset.Name,
			"uploader":             uploader,
			"browser_download_url": runtimeAsset.BrowserDownloadURL,
		})
	}
	return map[string]any{
		"url":          "https://api.github.com/repos/rcourtman/Pulse/releases/1",
		"tag_name":     tag,
		"name":         "Pulse " + tag,
		"draft":        false,
		"prerelease":   prerelease,
		"published_at": "2026-09-27T12:00:00Z",
		"author":       uploader,
		"assets":       assets,
		"body":         "notes for " + tag,
		"reactions":    map[string]any{"+1": 3, "total_count": 3},
	}
}

func TestIssue2282StableChannelSelectsStableFromMultiMegabyteReleaseList(t *testing.T) {
	setRetrySettingsForTest(t, 1, time.Millisecond, time.Millisecond)

	// Mirror the live page that broke stable checks: prereleases and
	// helm-chart releases crowd the newest entries and the stable sits deep.
	var releases []map[string]any
	for _, tag := range []string{"v6.4.5-rc.3", "v6.4.5-rc.2", "v6.4.5-rc.1", "v6.4.5-beta.3", "v6.4.5-beta.1", "v6.4.4-beta.2", "v6.4.4-beta.1", "v6.4.3-rc.1"} {
		releases = append(releases, issue2282ReleaseJSON(t, tag, true, 349))
		releases = append(releases, issue2282ReleaseJSON(t, "helm-chart-"+strings.TrimPrefix(tag, "v"), true, 0))
	}
	releases = append(releases,
		issue2282ReleaseJSON(t, "v6.4.1", false, 219),
		issue2282ReleaseJSON(t, "v6.4.0", false, 226),
		issue2282ReleaseJSON(t, "helm-chart-6.4.1", false, 0),
	)
	payload, err := json.Marshal(releases)
	if err != nil {
		t.Fatalf("marshal releases: %v", err)
	}
	if len(payload) <= 1<<20 {
		t.Fatalf("fixture is %d bytes, want it larger than the old 1 MiB bound", len(payload))
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	t.Setenv("PULSE_UPDATE_SERVER", server.URL)

	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	for _, tc := range []struct {
		channel, current, want string
	}{
		{"stable", "6.4.1", "v6.4.1"},
		{"stable", "6.0.0-rc.2", "v6.4.1"},
		{"rc", "6.4.3-rc.1", "v6.4.5-rc.3"},
	} {
		currentVer, err := ParseVersion(tc.current)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", tc.current, err)
		}
		release, err := manager.getLatestReleaseForChannel(context.Background(), tc.channel, currentVer)
		if err != nil {
			t.Fatalf("%s channel from %s: getLatestReleaseForChannel error: %v", tc.channel, tc.current, err)
		}
		if release.TagName != tc.want {
			t.Fatalf("%s channel from %s: release = %q, want %q", tc.channel, tc.current, release.TagName, tc.want)
		}
		if release.Body != "notes for "+tc.want {
			t.Fatalf("%s channel: release notes = %q, want the streamed body", tc.channel, release.Body)
		}
		runtimeAsset, ok := updateReleaseAssetForRuntime(tc.want)
		if !ok {
			continue
		}
		if len(release.Assets) != 1 || release.Assets[0] != runtimeAsset {
			t.Fatalf("%s channel: retained assets = %+v, want only runtime asset %+v", tc.channel, release.Assets, runtimeAsset)
		}
	}
}

func TestIssue2282ReleaseNotesDecodeLargeSingleRelease(t *testing.T) {
	payload, err := json.Marshal(issue2282ReleaseJSON(t, "v6.4.5", false, 2000))
	if err != nil {
		t.Fatalf("marshal release: %v", err)
	}
	if len(payload) <= 1<<20 {
		t.Fatalf("fixture is %d bytes, want it larger than the old 1 MiB bound", len(payload))
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	t.Setenv("PULSE_UPDATE_SERVER", server.URL)

	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	info, err := manager.GetReleaseNotes(context.Background(), "v6.4.5")
	if err != nil {
		t.Fatalf("GetReleaseNotes error: %v", err)
	}
	if info.Version != "6.4.5" || info.ReleaseNotes != "notes for v6.4.5" || info.IsPrerelease {
		t.Fatalf("release notes = %+v, want 6.4.5 stable notes", info)
	}
}

func issue2282Response(body string) *http.Response {
	return &http.Response{
		StatusCode:    http.StatusOK,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: -1,
	}
}

func TestIssue2282DecodeReleaseListShapes(t *testing.T) {
	releases, err := decodeReleaseList(issue2282Response(`[
		{"tag_name":"v6.4.1","name":null,"body":null,"published_at":null,"assets":null,
		 "author":{"nested":[{"deep":[1,2,{"x":"y"}]}]},"draft":false,"prerelease":false},
		{"draft":true,"tag_name":"v6.5.0","published_at":"2026-09-30T08:00:00Z",
		 "assets":[{"name":"pulse-v6.5.0-linux-amd64.tar.gz","browser_download_url":"https://example.invalid/a"},
		           {"name":"pulse-v6.5.0-darwin-arm64.tar.gz","browser_download_url":"https://example.invalid/b"},
		           {"name":"checksums.txt","browser_download_url":"https://example.invalid/c"}]}
	]`))
	if err != nil {
		t.Fatalf("decodeReleaseList error: %v", err)
	}
	if len(releases) != 2 {
		t.Fatalf("decoded %d releases, want 2", len(releases))
	}
	if first := releases[0]; first.TagName != "v6.4.1" || first.Name != "" || first.Body != "" || !first.PublishedAt.IsZero() || first.Assets != nil || first.Draft || first.Prerelease {
		t.Fatalf("null-field release = %+v", first)
	}
	second := releases[1]
	if !second.Draft || second.TagName != "v6.5.0" || !second.PublishedAt.Equal(time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("second release = %+v", second)
	}
	if len(second.Assets) != 1 || second.Assets[0].Name != "pulse-v6.5.0-linux-amd64.tar.gz" {
		t.Fatalf("retained assets = %+v, want only the linux server archive", second.Assets)
	}

	for name, body := range map[string]string{
		"object instead of list": `{"message":"Not Found"}`,
		"trailing data":          `[] []`,
		"truncated":              `[{"tag_name":"v6.4.1"`,
		"wrong field type":       `[{"prerelease":"yes"}]`,
		"assets not a list":      `[{"assets":{"name":"x"}}]`,
	} {
		if _, err := decodeReleaseList(issue2282Response(body)); err == nil {
			t.Fatalf("%s: decodeReleaseList accepted malformed metadata", name)
		}
	}
}

func TestIssue2282StreamedDecodeKeepsTypedByteBound(t *testing.T) {
	body := `[{"tag_name":"v9.9.9","body":"` + strings.Repeat("x", int(maxReleaseMetadataBytes)) + `"}]`
	_, err := decodeReleaseList(issue2282Response(body))
	if !securityutil.IsResponseBodyTooLarge(err) {
		t.Fatalf("decodeReleaseList error = %v, want typed response size rejection", err)
	}
}

func TestIssue2282OnlyServerArchivesAreRetained(t *testing.T) {
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"pulse-v6.4.5-linux-amd64.tar.gz", true},
		{"pulse-v6.4.5-rc.4-linux-arm64.tar.gz", true},
		{"pulse-agent-v6.4.5-linux-amd64.tar.gz", false},
		{"pulse-mcp-v6.4.5-linux-amd64.tar.gz", false},
		{"pulse-agent-helper-v6.4.5-linux-amd64.tar.gz", false},
		{"pulse-v6.4.5-darwin-arm64.tar.gz", false},
		{"pulse-v6.4.5-linux-amd64.tar.gz.sshsig", false},
	} {
		if got := isRuntimeReleaseAssetName(tc.name); got != tc.want {
			t.Errorf("isRuntimeReleaseAssetName(%q) = %t, want %t", tc.name, got, tc.want)
		}
	}
}

func TestIssue2282UpdateCheckRequiresExactServerArchive(t *testing.T) {
	withBuildVersion(t, "6.4.0")
	exact, ok := updateReleaseAssetForRuntime("v99.0.0")
	if !ok {
		t.Skip("no release archive for this architecture")
	}
	otherArch := "amd64"
	if strings.HasSuffix(exact.Name, "-amd64.tar.gz") {
		otherArch = "arm64"
	}
	sameArch := strings.TrimSuffix(strings.TrimPrefix(exact.Name, "pulse-v99.0.0-linux-"), ".tar.gz")
	decoys := []ReleaseAsset{
		{Name: "pulse-agent-v99.0.0-linux-" + sameArch + ".tar.gz", BrowserDownloadURL: "https://example.invalid/agent"},
		{Name: "pulse-mcp-v99.0.0-linux-" + sameArch + ".tar.gz", BrowserDownloadURL: "https://example.invalid/mcp"},
		{Name: "pulse-v99.0.0-linux-" + otherArch + ".tar.gz", BrowserDownloadURL: "https://example.invalid/wrong-arch"},
		{Name: "pulse-v98.0.0-linux-" + sameArch + ".tar.gz", BrowserDownloadURL: "https://example.invalid/wrong-version"},
	}
	for _, tc := range []struct {
		name, wantURL string
		wantAvailable bool
		assets        []ReleaseAsset
	}{
		{"missing exact archive", "", false, decoys},
		{"exact archive after decoys", exact.BrowserDownloadURL, true, append(append([]ReleaseAsset{}, decoys...), exact)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := newReleaseServer(t, []ReleaseInfo{{TagName: "v99.0.0", Assets: tc.assets}}, nil)
			defer server.Close()
			t.Setenv("PULSE_UPDATE_SERVER", server.URL)

			manager := NewManager(&config.Config{UpdateChannel: "stable"})
			info, err := manager.CheckForUpdatesWithChannel(context.Background(), "stable")
			if err != nil {
				t.Fatalf("check update: %v", err)
			}
			if info.Available != tc.wantAvailable || info.DownloadURL != tc.wantURL {
				t.Fatalf("update = %+v, want available=%t with download URL %q", info, tc.wantAvailable, tc.wantURL)
			}
		})
	}
}
