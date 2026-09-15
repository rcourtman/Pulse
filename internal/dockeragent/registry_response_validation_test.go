package dockeragent

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// A successful HTTP status is not proof that a fallback body is a manifest:
// proxies and registry frontends can return the same HTML/JSON error for unrelated tags.
func TestRegistryCheckerRejectsNonManifestFallback(t *testing.T) {
	for _, body := range []string{`<html>registry unavailable</html>`, `{"errors":[{"code":"UNAVAILABLE"}]}`, `{}`, `null`, `{"schemaVersion":2}`} {
		t.Run(body, func(t *testing.T) {
			checker := NewRegistryChecker(zerolog.Nop())
			checker.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host == "auth.docker.io" || strings.HasSuffix(req.URL.Path, "/token") {
					return newStringResponse(200, nil, `{"token":"synthetic"}`), nil
				}
				if req.Method == http.MethodHead {
					return newStringResponse(200, nil, ""), nil
				}
				return newStringResponse(200, nil, body), nil
			})}
			for _, ref := range []string{"redis:8-alpine", "postgres:16-alpine", "ghcr.io/searxng/searxng:latest"} {
				for attempt := 0; attempt < 2; attempt++ {
					got := checker.CheckImageUpdate(context.Background(), ref, "sha256:current", "amd64", "linux", "")
					if got.Error == "" || got.LatestDigest != "" || got.UpdateAvailable {
						t.Errorf("%s attempt %d accepted non-manifest response: %+v", ref, attempt, got)
					}
				}
			}
		})
	}
}
