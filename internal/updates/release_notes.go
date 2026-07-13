package updates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

// ErrReleaseNotFound indicates no published GitHub release exists for the
// requested version tag.
var ErrReleaseNotFound = errors.New("release not found")

const (
	releaseNotesCacheDuration     = 24 * time.Hour
	releaseNotesMissCacheDuration = time.Hour
)

// ReleaseNotesInfo carries the release notes for a specific published version.
type ReleaseNotesInfo struct {
	Version      string    `json:"version"`
	ReleaseNotes string    `json:"releaseNotes"`
	ReleaseDate  time.Time `json:"releaseDate"`
	IsPrerelease bool      `json:"isPrerelease"`
}

func resolveReleaseByTagURL(tag string) (*url.URL, error) {
	baseURL, err := securityutil.NormalizeHTTPBaseURL(updateReleaseAPIBaseURL(), "https")
	if err != nil {
		return nil, fmt.Errorf("invalid update server base URL: %w", err)
	}

	target, err := securityutil.ResolveRelativeURL(baseURL, updateReleaseAPIPath()+"/tags/"+url.PathEscape(tag))
	if err != nil {
		return nil, fmt.Errorf("build release notes URL: %w", err)
	}

	return target, nil
}

// GetReleaseNotes fetches the GitHub release notes for a specific version tag.
// Results (including "no such release") are cached so repeated UI requests
// don't burn GitHub API quota.
func (m *Manager) GetReleaseNotes(ctx context.Context, version string) (*ReleaseNotesInfo, error) {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return nil, fmt.Errorf("version is required")
	}
	tag := "v" + strings.TrimPrefix(trimmed, "v")

	m.statusMu.RLock()
	if m.notesCacheTag == tag {
		if m.notesCacheMiss && time.Since(m.notesCacheTime) < releaseNotesMissCacheDuration {
			m.statusMu.RUnlock()
			return nil, fmt.Errorf("%w: %s", ErrReleaseNotFound, tag)
		}
		if m.notesCache != nil && time.Since(m.notesCacheTime) < releaseNotesCacheDuration {
			cached := m.notesCache
			m.statusMu.RUnlock()
			return cached, nil
		}
	}
	m.statusMu.RUnlock()

	target, err := resolveReleaseByTagURL(tag)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := m.getWithRetry(ctx, client, target, map[string]string{
		"Accept":     "application/vnd.github.v3+json",
		"User-Agent": "Pulse-Update-Checker",
	}, "fetch release notes")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release notes: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		m.cacheReleaseNotes(tag, nil)
		return nil, fmt.Errorf("%w: %s", ErrReleaseNotFound, tag)
	case resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("%w: fetching release notes for %s", errGitHubRateLimited, tag)
	case resp.StatusCode != http.StatusOK:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = resp.Status
		}
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, detail)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxReleaseFeedBytes)).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release notes: %w", err)
	}

	info := &ReleaseNotesInfo{
		Version:      strings.TrimPrefix(release.TagName, "v"),
		ReleaseNotes: release.Body,
		ReleaseDate:  release.PublishedAt,
		IsPrerelease: release.Prerelease,
	}
	m.cacheReleaseNotes(tag, info)

	return info, nil
}

func (m *Manager) cacheReleaseNotes(tag string, info *ReleaseNotesInfo) {
	m.statusMu.Lock()
	m.notesCacheTag = tag
	m.notesCache = info
	m.notesCacheMiss = info == nil
	m.notesCacheTime = time.Now()
	m.statusMu.Unlock()
}
