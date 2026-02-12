package updates

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestResolveChannel(t *testing.T) {
	manager := NewManager(&config.Config{UpdateChannel: "stable"})

	if got := manager.resolveChannel("rc", nil); got != "rc" {
		t.Fatalf("expected requested channel, got %s", got)
	}
	if got := manager.resolveChannel("", nil); got != "stable" {
		t.Fatalf("expected config channel, got %s", got)
	}
	if got := manager.resolveChannel("", &VersionInfo{Channel: "rc"}); got != "stable" {
		t.Fatalf("expected config to win, got %s", got)
	}

	manager.config.UpdateChannel = ""
	if got := manager.resolveChannel("", &VersionInfo{Channel: "rc"}); got != "rc" {
		t.Fatalf("expected version channel, got %s", got)
	}
	if got := manager.resolveChannel("", nil); got != "stable" {
		t.Fatalf("expected default channel, got %s", got)
	}
}

func TestGetCachedUpdateInfo(t *testing.T) {
	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	expected := &UpdateInfo{Available: true, LatestVersion: "v1.2.3"}
	manager.statusMu.Lock()
	manager.checkCache["stable"] = expected
	manager.cacheTime["stable"] = time.Now()
	manager.statusMu.Unlock()

	if got := manager.GetCachedUpdateInfo(); got != expected {
		t.Fatalf("expected cached info, got %+v", got)
	}
}

func TestManagerUpdateStatus(t *testing.T) {
	manager := NewManager(&config.Config{})

	manager.updateStatus("checking", 12, "progress", errors.New("boom"))
	status := manager.GetStatus()
	if status.Status != "checking" || status.Progress != 12 || status.Message != "progress" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.Error == "" || !strings.Contains(status.Error, "boom") {
		t.Fatalf("unexpected status error: %s", status.Error)
	}

	select {
	case got := <-manager.GetProgressChannel():
		if got.Status != "checking" || got.Progress != 12 {
			t.Fatalf("unexpected progress: %+v", got)
		}
	default:
		t.Fatal("expected progress update on channel")
	}
}

func TestConfiguredStageDelay(t *testing.T) {
	stageDelayOnce = sync.Once{}
	stageDelayValue = 0
	t.Setenv("PULSE_UPDATE_STAGE_DELAY_MS", "15")
	if got := configuredStageDelay(); got != 15*time.Millisecond {
		t.Fatalf("expected 15ms, got %v", got)
	}
	if got := statusDelayForStage("downloading"); got != 15*time.Millisecond {
		t.Fatalf("expected 15ms delay for downloading, got %v", got)
	}
	if got := statusDelayForStage("idle"); got != 0 {
		t.Fatalf("expected 0 delay for idle, got %v", got)
	}

	stageDelayOnce = sync.Once{}
	stageDelayValue = 0
	t.Setenv("PULSE_UPDATE_STAGE_DELAY_MS", "bad")
	if got := configuredStageDelay(); got != 0 {
		t.Fatalf("expected 0 delay for invalid value, got %v", got)
	}
}

func TestManagerCloseIsIdempotentAndUpdateStatusAfterCloseIsSafe(t *testing.T) {
	manager := NewManager(&config.Config{})

	manager.Close()
	manager.Close()

	manager.updateStatus("idle", 0, "after close")

	select {
	case _, ok := <-manager.GetProgressChannel():
		if ok {
			t.Fatal("expected progress channel to remain closed")
		}
	default:
		t.Fatal("expected progress channel to be closed")
	}
}

func TestGetLatestReleaseFromFeedMocked(t *testing.T) {
	feed := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry><title>Pulse v5.0.0-rc.1</title></entry>
  <entry><title>Pulse v4.36.2</title></entry>
</feed>`

	origTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body := io.NopCloser(strings.NewReader(feed))
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       body,
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
	if release.TagName != "v4.36.2" {
		t.Fatalf("unexpected stable tag: %s", release.TagName)
	}

	release, err = manager.getLatestReleaseFromFeed(context.Background(), "rc")
	if err != nil {
		t.Fatalf("rc feed error: %v", err)
	}
	if release.TagName != "v5.0.0-rc.1" {
		t.Fatalf("unexpected rc tag: %s", release.TagName)
	}

	feed = `<?xml version="1.0" encoding="UTF-8"?><feed></feed>`
	if _, err := manager.getLatestReleaseFromFeed(context.Background(), "stable"); err == nil {
		t.Fatal("expected error for empty feed")
	}
}

func TestManagerDownloadFile(t *testing.T) {
	content := "payload"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer server.Close()

	manager := NewManager(&config.Config{})
	dest := filepath.Join(t.TempDir(), "file.bin")

	n, err := manager.downloadFile(context.Background(), server.URL, dest)
	if err != nil {
		t.Fatalf("downloadFile error: %v", err)
	}
	if n != int64(len(content)) {
		t.Fatalf("expected %d bytes, got %d", len(content), n)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read file error: %v", err)
	}
	if string(data) != content {
		t.Fatalf("unexpected file content: %s", string(data))
	}
}
