package dockeragent

import (
	"context"
	"net/http"
	"testing"
    "github.com/rs/zerolog"
)

func TestRegistryChecker_ResolveManifestList(t *testing.T) {
    logger := zerolog.Nop()
	t.Run("resolve manifest list", func(t *testing.T) {
		checker := NewRegistryChecker(logger)
		checker.httpClient = &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
                if req.Method == "HEAD" {
				    return newStringResponse(http.StatusOK, map[string]string{
                        "Content-Type": "application/vnd.docker.distribution.manifest.list.v2+json",
                    }, ""), nil
                }
                // GET request for body
                body := `{
                    "manifests": [
                        {
                            "digest": "sha256:armv7",
                            "platform": { "architecture": "arm", "os": "linux", "variant": "v7" }
                        },
                        {
                            "digest": "sha256:amd64",
                            "platform": { "architecture": "amd64", "os": "linux" }
                        }
                    ]
                }`
                return newStringResponse(http.StatusOK, nil, body), nil
			}),
		}

		// Test matching amd64
		result := checker.CheckImageUpdate(context.Background(), "image:tag", "sha256:current", "amd64", "linux", "")
        if result.LatestDigest != "sha256:amd64" {
            t.Errorf("Expected sha256:amd64, got %s", result.LatestDigest)
        }

        // Test matching arm/v7
        result = checker.CheckImageUpdate(context.Background(), "image:tag", "sha256:current", "arm", "linux", "v7")
        if result.LatestDigest != "sha256:armv7" {
            t.Errorf("Expected sha256:armv7, got %s", result.LatestDigest)
        }
	})
}
