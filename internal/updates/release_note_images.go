package updates

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const maxReleaseNoteImageBytes = 10 * 1024 * 1024

var releaseNoteImageName = regexp.MustCompile(`^release-note-[a-z0-9]+(?:-[a-z0-9]+)*-(?:before|now)\.png$`)
var releaseNoteVersion = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+(?:-(?:alpha|beta|rc)\.[0-9]+)?$`)

func releaseNoteImageClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 || req.URL.Scheme != "https" || req.URL.User != nil || req.URL.Port() != "" {
				return fmt.Errorf("invalid release screenshot redirect")
			}
			switch req.URL.Hostname() {
			case "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
				return nil
			default:
				return fmt.Errorf("release screenshot redirect left GitHub asset hosting")
			}
		},
	}
}

// GetReleaseNoteImage serves only PNG sidecars referenced by the running
// release's canonical notes. It is not a general URL or historical-asset proxy.
func (m *Manager) GetReleaseNoteImage(ctx context.Context, version, name string) ([]byte, error) {
	return m.getReleaseNoteImage(ctx, version, name, releaseNoteImageClient())
}

func (m *Manager) getReleaseNoteImage(ctx context.Context, version, name string, client *http.Client) ([]byte, error) {
	if len(version) > 80 || !releaseNoteVersion.MatchString(version) || len(name) > 128 || !releaseNoteImageName.MatchString(name) {
		return nil, ErrReleaseNotFound
	}
	version = strings.TrimPrefix(version, "v")
	notes, err := m.GetReleaseNotes(ctx, version)
	if err != nil {
		return nil, err
	}
	target := "https://github.com/rcourtman/Pulse/releases/download/v" + version + "/" + name
	if notes.Version != version || !strings.Contains(notes.ReleaseNotes, "]("+target+")") {
		return nil, ErrReleaseNotFound
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/png")
	req.Header.Set("User-Agent", "Pulse-Release-Notes")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release screenshot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrReleaseNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release screenshot returned status %d", resp.StatusCode)
	}
	if resp.ContentLength > maxReleaseNoteImageBytes {
		return nil, fmt.Errorf("release screenshot exceeds size limit")
	}
	image, err := io.ReadAll(io.LimitReader(resp.Body, maxReleaseNoteImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read release screenshot: %w", err)
	}
	if len(image) > maxReleaseNoteImageBytes {
		return nil, fmt.Errorf("release screenshot exceeds size limit")
	}
	dimensions, err := png.DecodeConfig(bytes.NewReader(image))
	if err != nil || dimensions.Width > 8192 || dimensions.Height > 8192 || int64(dimensions.Width)*int64(dimensions.Height) > 20_000_000 {
		return nil, fmt.Errorf("invalid release screenshot PNG")
	}
	return image, nil
}
