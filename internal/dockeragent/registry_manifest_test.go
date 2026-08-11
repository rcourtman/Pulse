package dockeragent

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestRegistryChecker_ResolveManifestList(t *testing.T) {
	logger := zerolog.Nop()
	t.Run("resolve manifest list", func(t *testing.T) {
		manifestListBody := `{
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
		wantIndexDigest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(manifestListBody)))
		checker := NewRegistryChecker(logger)
		checker.httpClient = &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method == "HEAD" {
					return newStringResponse(http.StatusOK, map[string]string{
						"Content-Type": "application/vnd.docker.distribution.manifest.list.v2+json",
					}, ""), nil
				}
				// GET request for body
				return newStringResponse(http.StatusOK, nil, manifestListBody), nil
			}),
		}

		// Test matching amd64
		result := checker.CheckImageUpdate(context.Background(), "image:tag", "sha256:amd64", "amd64", "linux", "")
		if result.LatestDigest != wantIndexDigest {
			t.Errorf("Expected index digest %s, got %s", wantIndexDigest, result.LatestDigest)
		}
		if result.UpdateAvailable {
			t.Error("Expected matching amd64 platform digest to suppress the update")
		}

		// Test matching arm/v7
		result = checker.CheckImageUpdate(context.Background(), "image:tag", "sha256:armv7", "arm", "linux", "v7")
		if result.LatestDigest != wantIndexDigest {
			t.Errorf("Expected index digest %s, got %s", wantIndexDigest, result.LatestDigest)
		}
		if result.UpdateAvailable {
			t.Error("Expected matching arm/v7 platform digest to suppress the update")
		}
	})

	t.Run("manifest list body too large", func(t *testing.T) {
		checker := &RegistryChecker{
			httpClient: &http.Client{
				Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
					return newStringResponse(http.StatusOK, nil, strings.Repeat("x", maxRegistryManifestBodyBytes+1)), nil
				}),
			},
		}

		_, err := checker.resolveManifestList(context.Background(), "example.test", "repo", "latest", "amd64", "linux", "", "")
		if err == nil || !strings.Contains(err.Error(), "response body exceeds") {
			t.Fatalf("expected oversized manifest list error, got %v", err)
		}
	})
}

func TestRegistryChecker_MultiArchDigestLayersStayConsistentAcrossCache(t *testing.T) {
	logger := zerolog.Nop()
	requestCount := 0
	checker := NewRegistryChecker(logger)
	checker.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestCount++
			if req.Method == http.MethodHead {
				return newStringResponse(http.StatusOK, map[string]string{
					"Content-Type":          "application/vnd.oci.image.index.v1+json",
					"Docker-Content-Digest": "sha256:index",
				}, ""), nil
			}
			return newStringResponse(http.StatusOK, nil, `{
				"manifests": [{
					"digest": "sha256:platform",
					"platform": {"architecture": "amd64", "os": "linux"}
				}]
			}`), nil
		}),
	}

	currentDigests := []string{"sha256:index", "sha256:platform"}
	for attempt, currentDigest := range currentDigests {
		result := checker.CheckImageUpdate(
			context.Background(),
			"example.test/repo:tag",
			currentDigest,
			"amd64",
			"linux",
			"",
		)
		if result == nil {
			t.Fatal("expected an update result")
		}
		if result.LatestDigest != "sha256:index" {
			t.Fatalf("attempt %d: expected index digest, got %q", attempt+1, result.LatestDigest)
		}
		if result.UpdateAvailable {
			t.Fatalf("attempt %d: expected matching digest %q to suppress the update", attempt+1, currentDigest)
		}
	}

	if requestCount != 2 {
		t.Fatalf("expected one HEAD and one GET before the cache hit, got %d requests", requestCount)
	}
}
