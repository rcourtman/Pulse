package updates

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestGetReleaseNoteImage(t *testing.T) {
	const name = "release-note-backups-now.png"
	const target = "https://github.com/rcourtman/Pulse/releases/download/v6.6.0/" + name
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name         string
		version      string
		asset        string
		body         string
		payload      []byte
		size         int64
		status       int
		wantErr      bool
		wantRequests int
	}{
		{name: "published referenced PNG", version: "6.6.0", asset: name, body: "![Now](" + target + ")", payload: picture.Bytes(), status: 200, wantRequests: 1},
		{name: "unreferenced asset", version: "6.6.0", asset: name, body: "## Highlights\n- Text only", wantErr: true},
		{name: "different version", version: "6.5.0", asset: name, body: "![Now](" + target + ")", wantErr: true},
		{name: "path traversal", version: "6.6.0", asset: "../" + name, wantErr: true},
		{name: "unrelated asset", version: "6.6.0", asset: "pulse-linux-amd64.tar.gz", wantErr: true},
		{name: "invalid version", version: "../../private", asset: name, wantErr: true},
		{name: "SVG disguised as PNG", version: "6.6.0", asset: name, body: "![Now](" + target + ")", payload: []byte("<svg onload='alert(1)'/>"), status: 200, wantErr: true, wantRequests: 1},
		{name: "missing sidecar", version: "6.6.0", asset: name, body: "![Now](" + target + ")", status: 404, wantErr: true, wantRequests: 1},
		{name: "declared oversized PNG", version: "6.6.0", asset: name, body: "![Now](" + target + ")", payload: picture.Bytes(), size: maxReleaseNoteImageBytes + 1, status: 200, wantErr: true, wantRequests: 1},
		{name: "streamed oversized PNG", version: "6.6.0", asset: name, body: "![Now](" + target + ")", payload: make([]byte, maxReleaseNoteImageBytes+1), status: 200, wantErr: true, wantRequests: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager := NewManager(&config.Config{})
			manager.cacheReleaseNotes("v"+tc.version, &ReleaseNotesInfo{Version: tc.version, ReleaseNotes: tc.body})
			requests := 0
			client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.URL.String() != target || req.Header.Get("Authorization") != "" {
					t.Fatalf("unexpected image request %s", req.URL)
				}
				return &http.Response{StatusCode: tc.status, ContentLength: tc.size, Body: io.NopCloser(bytes.NewReader(tc.payload))}, nil
			})}
			got, err := manager.getReleaseNoteImage(context.Background(), tc.version, tc.asset, client)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, want error %v", err, tc.wantErr)
			}
			if !tc.wantErr && !bytes.Equal(got, picture.Bytes()) {
				t.Fatal("PNG changed in proxy")
			}
			if requests != tc.wantRequests {
				t.Fatalf("requests = %d, want %d", requests, tc.wantRequests)
			}
		})
	}
}

func TestReleaseNoteImageRedirects(t *testing.T) {
	client := releaseNoteImageClient()
	for _, target := range []string{"http://github.com/image", "https://127.0.0.1/private", "https://example.com/image", "https://user:pass@github.com/image"} {
		parsed, _ := url.Parse(target)
		if err := client.CheckRedirect(&http.Request{URL: parsed}, nil); err == nil {
			t.Fatalf("allowed unsafe redirect %s", target)
		}
	}
	parsed, _ := url.Parse("https://release-assets.githubusercontent.com/image?signature=example")
	if err := client.CheckRedirect(&http.Request{URL: parsed}, nil); err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(&http.Request{URL: parsed}, make([]*http.Request, 5)); err == nil {
		t.Fatal("allowed a redirect loop")
	}
}

func TestGetReleaseNoteImageMissingRelease(t *testing.T) {
	manager := NewManager(&config.Config{})
	manager.cacheReleaseNotes("v6.6.0", nil)
	_, err := manager.GetReleaseNoteImage(context.Background(), "6.6.0", "release-note-backups-now.png")
	if !errors.Is(err, ErrReleaseNotFound) {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "https://") {
		t.Fatal("missing release unexpectedly fetched an image")
	}
}
