package updates

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func newReleaseNotesServer(t *testing.T, tag string, release *ReleaseInfo, hitCount *int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != updateReleaseAPIPath()+"/tags/"+tag {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if hitCount != nil {
			atomic.AddInt32(hitCount, 1)
		}
		if release == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release)
	}))
}

func TestGetReleaseNotes_FetchesByTagAndCaches(t *testing.T) {
	var hits int32
	release := &ReleaseInfo{
		TagName:     "v4.13.0",
		Name:        "v4.13.0",
		Body:        "## Highlights\n- Something shiny",
		Prerelease:  false,
		PublishedAt: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
	}

	server := newReleaseNotesServer(t, "v4.13.0", release, &hits)
	defer server.Close()
	t.Setenv("PULSE_UPDATE_SERVER", server.URL)

	manager := NewManager(&config.Config{UpdateChannel: "stable"})

	// Version without the "v" prefix must resolve to the v-prefixed tag.
	info, err := manager.GetReleaseNotes(context.Background(), "4.13.0")
	if err != nil {
		t.Fatalf("GetReleaseNotes returned error: %v", err)
	}
	if info.Version != "4.13.0" {
		t.Fatalf("expected version 4.13.0, got %q", info.Version)
	}
	if info.ReleaseNotes != release.Body {
		t.Fatalf("expected release notes %q, got %q", release.Body, info.ReleaseNotes)
	}
	if !info.ReleaseDate.Equal(release.PublishedAt) {
		t.Fatalf("expected release date %v, got %v", release.PublishedAt, info.ReleaseDate)
	}

	if _, err := manager.GetReleaseNotes(context.Background(), "v4.13.0"); err != nil {
		t.Fatalf("cached GetReleaseNotes returned error: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected 1 GitHub request after caching, got %d", got)
	}
}

func TestGetReleaseNotes_NotFoundIsCached(t *testing.T) {
	var hits int32
	server := newReleaseNotesServer(t, "v9.9.9", nil, &hits)
	defer server.Close()
	t.Setenv("PULSE_UPDATE_SERVER", server.URL)

	manager := NewManager(&config.Config{UpdateChannel: "stable"})

	for i := 0; i < 2; i++ {
		_, err := manager.GetReleaseNotes(context.Background(), "9.9.9")
		if !errors.Is(err, ErrReleaseNotFound) {
			t.Fatalf("attempt %d: expected ErrReleaseNotFound, got %v", i+1, err)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected 1 GitHub request after negative caching, got %d", got)
	}
}

func TestGetReleaseNotes_RequiresVersion(t *testing.T) {
	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	if _, err := manager.GetReleaseNotes(context.Background(), "  "); err == nil {
		t.Fatal("expected error for empty version")
	}
}
