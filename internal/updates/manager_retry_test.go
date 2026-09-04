package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

func setRetrySettingsForTest(t *testing.T, attempts int, backoff, maxBackoff time.Duration) {
	t.Helper()

	prevAttempts := updateHTTPAttempts
	prevBackoff := updateHTTPBackoff
	prevMaxBackoff := updateHTTPMaxBackoff

	updateHTTPAttempts = attempts
	updateHTTPBackoff = backoff
	updateHTTPMaxBackoff = maxBackoff

	t.Cleanup(func() {
		updateHTTPAttempts = prevAttempts
		updateHTTPBackoff = prevBackoff
		updateHTTPMaxBackoff = prevMaxBackoff
	})
}

func TestManagerDownloadFileRetriesTransientStatus(t *testing.T) {
	setRetrySettingsForTest(t, 3, time.Millisecond, 5*time.Millisecond)

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&hits, 1)
		if attempt == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("payload"))
	}))
	defer server.Close()

	manager := &Manager{}
	dest := filepath.Join(t.TempDir(), "download.tar.gz")
	n, err := manager.downloadFile(context.Background(), server.URL, dest)
	if err != nil {
		t.Fatalf("downloadFile error: %v", err)
	}
	if n != int64(len("payload")) {
		t.Fatalf("downloaded bytes = %d, want %d", n, len("payload"))
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("request count = %d, want 2", got)
	}
}

func TestManagerDownloadFileDoesNotRetryNonRetryableStatus(t *testing.T) {
	setRetrySettingsForTest(t, 3, time.Millisecond, 5*time.Millisecond)

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	manager := &Manager{}
	dest := filepath.Join(t.TempDir(), "download.tar.gz")
	_, err := manager.downloadFile(context.Background(), server.URL, dest)
	if err == nil {
		t.Fatal("expected download error")
	}
	if !strings.Contains(err.Error(), "status 404") {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
}

func TestManagerVerifyChecksumRetriesTransientChecksumDownload(t *testing.T) {
	setRetrySettingsForTest(t, 3, time.Millisecond, 5*time.Millisecond)

	tarballPath := filepath.Join(t.TempDir(), "pulse.tar.gz")
	if err := os.WriteFile(tarballPath, []byte("payload"), 0600); err != nil {
		t.Fatalf("write tarball: %v", err)
	}
	sum := sha256.Sum256([]byte("payload"))
	checksum := hex.EncodeToString(sum[:])

	var checksumHits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "SHA256SUMS") {
			attempt := atomic.AddInt32(&checksumHits, 1)
			if attempt == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(checksum + "  pulse.tar.gz\n"))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	manager := &Manager{}
	if err := manager.verifyChecksum(context.Background(), server.URL+"/pulse.tar.gz", tarballPath); err != nil {
		t.Fatalf("verifyChecksum error: %v", err)
	}
	if got := atomic.LoadInt32(&checksumHits); got != 2 {
		t.Fatalf("checksum request count = %d, want 2", got)
	}
}

func TestGetLatestReleaseForChannelRetriesTransientStatus(t *testing.T) {
	setRetrySettingsForTest(t, 3, time.Millisecond, 5*time.Millisecond)

	releases := []ReleaseInfo{
		{
			TagName:    "v9.9.9",
			Prerelease: false,
		},
	}

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&hits, 1)
		if r.URL.Path != "/repos/rcourtman/Pulse/releases" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if attempt == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases)
	}))
	defer server.Close()

	t.Setenv("PULSE_UPDATE_SERVER", server.URL)

	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	currentVer, err := ParseVersion("1.0.0")
	if err != nil {
		t.Fatalf("ParseVersion: %v", err)
	}

	release, err := manager.getLatestReleaseForChannel(context.Background(), "stable", currentVer)
	if err != nil {
		t.Fatalf("getLatestReleaseForChannel error: %v", err)
	}
	if release.TagName != "v9.9.9" {
		t.Fatalf("release tag = %s, want v9.9.9", release.TagName)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("request count = %d, want 2", got)
	}
}

func TestGetLatestReleaseForChannelFallsBackWhenReleaseMetadataIsOversized(t *testing.T) {
	setRetrySettingsForTest(t, 1, time.Millisecond, time.Millisecond)

	feed := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <title>Pulse v6.4.0-rc.11</title>
    <updated>2026-08-28T12:52:29Z</updated>
  </entry>
  <entry>
    <title>v6.4.2</title>
    <updated>2026-08-31T19:08:45Z</updated>
  </entry>
</feed>`

	origTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		status := http.StatusNotFound
		body := "not found"
		header := http.Header{"Content-Type": []string{"text/plain"}}
		switch req.URL.String() {
		case "https://api.github.com/repos/rcourtman/Pulse/releases":
			status = http.StatusOK
			body = `[{"tag_name":"v9.9.9","body":"` +
				strings.Repeat("x", int(maxReleaseMetadataBytes)) + `"}]`
			header.Set("Content-Type", "application/json")
		case "https://github.com/rcourtman/Pulse/releases.atom":
			status = http.StatusOK
			body = feed
			header.Set("Content-Type", "application/atom+xml")
		}
		return &http.Response{
			StatusCode:    status,
			Status:        http.StatusText(status),
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: -1,
			Header:        header,
			Request:       req,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = origTransport })

	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	currentVer, err := ParseVersion("6.4.1")
	if err != nil {
		t.Fatalf("ParseVersion: %v", err)
	}

	release, err := manager.getLatestReleaseForChannel(context.Background(), "stable", currentVer)
	if err != nil {
		t.Fatalf("getLatestReleaseForChannel error: %v", err)
	}
	if release.TagName != "v6.4.2" {
		t.Fatalf("release tag = %q, want bare-tag feed release v6.4.2", release.TagName)
	}
	expectedAsset, supported := updateReleaseAssetForRuntime(release.TagName)
	if !supported {
		t.Fatalf("test runner architecture %q must map to a release asset", runtime.GOARCH)
	}
	if len(release.Assets) != 1 || release.Assets[0] != expectedAsset {
		t.Fatalf("release assets = %+v, want %+v", release.Assets, expectedAsset)
	}
}

func TestGetLatestReleaseForChannelDoesNotReplaceCustomOversizedMetadata(t *testing.T) {
	setRetrySettingsForTest(t, 1, time.Millisecond, time.Millisecond)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.(http.Flusher).Flush()
		_, _ = io.WriteString(w, `[{"tag_name":"v9.9.9","body":"`)
		_, _ = io.WriteString(w, strings.Repeat("x", int(maxReleaseMetadataBytes)))
		_, _ = io.WriteString(w, `"}]`)
	}))
	defer server.Close()

	t.Setenv("PULSE_UPDATE_SERVER", server.URL)
	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	currentVer, err := ParseVersion("6.4.1")
	if err != nil {
		t.Fatalf("ParseVersion: %v", err)
	}

	_, err = manager.getLatestReleaseForChannel(context.Background(), "stable", currentVer)
	if !securityutil.IsResponseBodyTooLarge(err) {
		t.Fatalf("getLatestReleaseForChannel error = %v, want typed response size rejection", err)
	}
}

func TestGetLatestReleaseForChannelDoesNotMaskMalformedMetadata(t *testing.T) {
	setRetrySettingsForTest(t, 1, time.Millisecond, time.Millisecond)

	var feedHits atomic.Int32
	origTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body := "{"
		status := http.StatusOK
		if req.URL.String() == "https://github.com/rcourtman/Pulse/releases.atom" {
			feedHits.Add(1)
			body = `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><entry><title>Pulse v9.9.9</title></entry></feed>`
		}
		return &http.Response{
			StatusCode:    status,
			Status:        http.StatusText(status),
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
			Header:        make(http.Header),
			Request:       req,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = origTransport })

	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	currentVer, err := ParseVersion("6.4.1")
	if err != nil {
		t.Fatalf("ParseVersion: %v", err)
	}

	_, err = manager.getLatestReleaseForChannel(context.Background(), "stable", currentVer)
	if err == nil || !strings.Contains(err.Error(), "failed to decode releases") {
		t.Fatalf("getLatestReleaseForChannel error = %v, want JSON decode failure", err)
	}
	if got := feedHits.Load(); got != 0 {
		t.Fatalf("Atom feed requests = %d, want 0 for malformed metadata", got)
	}
}

func TestGetLatestReleaseForChannelFallsBackWhenGitHubMetadataExceedsLimit(t *testing.T) {
	setRetrySettingsForTest(t, 1, time.Millisecond, time.Millisecond)

	const feed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <title>Pulse v6.4.3-rc.1</title>
    <updated>2026-09-02T03:05:16Z</updated>
  </entry>
  <entry>
    <title>v6.4.2</title>
    <updated>2026-08-31T19:08:45Z</updated>
  </entry>
</feed>`

	origTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		status := http.StatusNotFound
		body := "not found"
		contentLength := int64(len(body))
		header := http.Header{"Content-Type": []string{"text/plain"}}
		switch req.URL.String() {
		case "https://api.github.com/repos/rcourtman/Pulse/releases":
			status = http.StatusOK
			body = `[{"tag_name":"v6.4.3-rc.1","body":"` + strings.Repeat("x", int(maxReleaseMetadataBytes)) + `"}]`
			// GitHub's live releases response is streamed without a declared
			// Content-Length, so exercise the bounded reader rather than only
			// the early header rejection.
			contentLength = -1
			header.Set("Content-Type", "application/json")
		case "https://github.com/rcourtman/Pulse/releases.atom":
			status = http.StatusOK
			body = feed
			contentLength = int64(len(body))
			header.Set("Content-Type", "application/atom+xml")
		}
		return &http.Response{
			StatusCode:    status,
			Status:        http.StatusText(status),
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: contentLength,
			Header:        header,
			Request:       req,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = origTransport })

	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	currentVer, err := ParseVersion("6.4.1")
	if err != nil {
		t.Fatalf("ParseVersion: %v", err)
	}

	release, err := manager.getLatestReleaseForChannel(context.Background(), "stable", currentVer)
	if err != nil {
		t.Fatalf("getLatestReleaseForChannel error: %v", err)
	}
	if release.TagName != "v6.4.2" {
		t.Fatalf("release tag = %q, want v6.4.2", release.TagName)
	}
	if len(release.Assets) != 1 || release.Assets[0].BrowserDownloadURL == "" {
		t.Fatalf("fallback release assets = %#v, want synthesized runtime asset", release.Assets)
	}
}

func TestGetLatestReleaseForChannelRateLimitFallbackIncludesRuntimeAsset(t *testing.T) {
	setRetrySettingsForTest(t, 1, time.Millisecond, time.Millisecond)
	withBuildVersion(t, "6.4.0-rc.10")

	releaseTime := time.Date(2026, 8, 28, 12, 52, 29, 0, time.UTC)
	feed := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <title>Pulse v6.4.0-rc.11</title>
    <updated>2026-08-28T12:52:29Z</updated>
  </entry>
</feed>`

	origTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		status := http.StatusNotFound
		body := "not found"
		header := http.Header{"Content-Type": []string{"text/plain"}}
		switch req.URL.String() {
		case "https://api.github.com/repos/rcourtman/Pulse/releases":
			status = http.StatusForbidden
			body = `{"message":"API rate limit exceeded"}`
			header.Set("X-RateLimit-Remaining", "0")
		case "https://github.com/rcourtman/Pulse/releases.atom":
			status = http.StatusOK
			body = feed
			header.Set("Content-Type", "application/atom+xml")
		}
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     header,
			Request:    req,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = origTransport })

	manager := NewManager(&config.Config{UpdateChannel: "rc"})
	info, err := manager.CheckForUpdatesWithChannel(context.Background(), "rc")
	if err != nil {
		t.Fatalf("CheckForUpdatesWithChannel error: %v", err)
	}
	if !info.Available || info.LatestVersion != "6.4.0-rc.11" {
		t.Fatalf("fallback update = %+v, want available v6.4.0-rc.11", info)
	}
	expectedAsset, supported := updateReleaseAssetForRuntime("v6.4.0-rc.11")
	if !supported {
		t.Fatalf("test runner architecture %q must map to a release asset", runtime.GOARCH)
	}
	if info.DownloadURL != expectedAsset.BrowserDownloadURL {
		t.Fatalf("fallback DownloadURL = %q, want %q", info.DownloadURL, expectedAsset.BrowserDownloadURL)
	}
	if !info.ReleaseDate.Equal(releaseTime) {
		t.Fatalf("fallback ReleaseDate = %s, want %s", info.ReleaseDate, releaseTime)
	}
	if target, err := ValidateApplyTargetVersion("rc", info.DownloadURL); err != nil || target != "v6.4.0-rc.11" {
		t.Fatalf("fallback apply target = %q, err=%v", target, err)
	}
}

func TestGetLatestReleaseForChannelRejectsUpdateServerUserinfo(t *testing.T) {
	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	currentVer, err := ParseVersion("1.0.0")
	if err != nil {
		t.Fatalf("ParseVersion: %v", err)
	}

	t.Setenv("PULSE_UPDATE_SERVER", "https://user:pass@example.com/proxy")

	_, err = manager.getLatestReleaseForChannel(context.Background(), "stable", currentVer)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "userinfo") {
		t.Fatalf("expected userinfo validation error, got %v", err)
	}
}

func TestGetLatestReleaseForChannelPreservesUpdateServerBasePath(t *testing.T) {
	setRetrySettingsForTest(t, 2, time.Millisecond, 5*time.Millisecond)

	releases := []ReleaseInfo{
		{
			TagName:    "v9.9.9",
			Prerelease: false,
		},
	}

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxy/repos/example/pulse-fork/releases" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases)
	}))
	defer server.Close()

	t.Setenv("PULSE_GITHUB_REPO", "example/pulse-fork")
	t.Setenv("PULSE_UPDATE_SERVER", server.URL+"/proxy")

	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	currentVer, err := ParseVersion("1.0.0")
	if err != nil {
		t.Fatalf("ParseVersion: %v", err)
	}

	release, err := manager.getLatestReleaseForChannel(context.Background(), "stable", currentVer)
	if err != nil {
		t.Fatalf("getLatestReleaseForChannel error: %v", err)
	}
	if release.TagName != "v9.9.9" {
		t.Fatalf("release tag = %s, want v9.9.9", release.TagName)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
}

func TestGetLatestReleaseForChannelDoesNotRetryNonRetryableStatus(t *testing.T) {
	setRetrySettingsForTest(t, 3, time.Millisecond, 5*time.Millisecond)

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer server.Close()

	t.Setenv("PULSE_UPDATE_SERVER", server.URL)

	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	currentVer, err := ParseVersion("1.0.0")
	if err != nil {
		t.Fatalf("ParseVersion: %v", err)
	}

	_, err = manager.getLatestReleaseForChannel(context.Background(), "stable", currentVer)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
}
