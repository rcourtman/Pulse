package dockeragent

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestDockerHubReferenceRouting(t *testing.T) {
	for _, host := range []string{"docker.io", "index.docker.io", "registry-1.docker.io", "registry.docker.io"} {
		for _, repo := range []string{"nginx", "library/nginx", "team/app"} {
			ref := host + "/" + repo + ":stable"
			t.Run(ref, func(t *testing.T) {
				wantRepo := repo
				if !strings.Contains(repo, "/") {
					wantRepo = "library/" + repo
				}
				registry, repository, tag := parseImageReference(ref)
				if registry != "registry-1.docker.io" || repository != wantRepo || tag != "stable" {
					t.Fatalf("parseImageReference(%q) = %q, %q, %q", ref, registry, repository, tag)
				}
			})
		}
	}
	for _, tt := range []struct{ ref, registry, repository, tag string }{
		{"nginx", "registry-1.docker.io", "library/nginx", "latest"},
		{"team/app:v2", "registry-1.docker.io", "team/app", "v2"},
		{"docker.io:5000/nginx:stable", "docker.io:5000", "nginx", "stable"},
		{"index.docker.io:5000/app", "index.docker.io:5000", "app", "latest"},
		{"docker.io.example/app:v2", "docker.io.example", "app", "v2"},
		{"registry.example:5000/team/app:v2", "registry.example:5000", "team/app", "v2"},
		{"localhost:5000/app:v2", "localhost:5000", "app", "v2"},
		{"[::1]:5000/app:v2", "[::1]:5000", "app", "v2"},
		{"ghcr.io/team/app:v2", "ghcr.io", "team/app", "v2"},
		{"docker.io/nginx@sha256:" + strings.Repeat("a", 64), "", "", ""},
	} {
		t.Run(tt.ref, func(t *testing.T) {
			registry, repository, tag := parseImageReference(tt.ref)
			if registry != tt.registry || repository != tt.repository || tag != tt.tag {
				t.Fatalf("parseImageReference(%q) = %q, %q, %q", tt.ref, registry, repository, tag)
			}
		})
	}
}

// Drive the real token, manifest fallback, platform comparison and cache paths.
// Only the HTTP transport and host credential files are synthetic; no Docker
// daemon, live registry, credential helper or container mutation is involved.
func TestDockerHubAliasLookupContract(t *testing.T) {
	index := `{"schemaVersion":2,"manifests":[{"digest":"sha256:amd64","platform":{"architecture":"amd64","os":"linux"}},{"digest":"sha256:armv7","platform":{"architecture":"arm","os":"linux","variant":"v7"}}]}`
	indexDigest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(index)))
	for _, host := range []string{"", "docker.io/", "index.docker.io/", "registry-1.docker.io/", "registry.docker.io/"} {
		for _, repo := range []string{"nginx", "team/app"} {
			for _, credentialed := range []bool{false, true} {
				ref := host + repo + ":stable"
				t.Run(fmt.Sprintf("%s/credentialed=%t", ref, credentialed), func(t *testing.T) {
					checker := NewRegistryChecker(zerolog.Nop())
					if credentialed {
						auth := base64.StdEncoding.EncodeToString([]byte("synthetic-user:synthetic-secret"))
						checker.credentials = newTestCredentialStore(map[string]string{
							"/synthetic/docker/config.json": fmt.Sprintf(`{"auths":{"https://index.docker.io/v1/":{"auth":%q}}}`, auth),
						}, map[string]string{"DOCKER_CONFIG": "/synthetic/docker"})
					}
					wantRepo := repo
					if repo == "nginx" {
						wantRepo = "library/nginx"
					}
					var requests []string
					checker.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						requests = append(requests, req.Method+" "+req.URL.Host+req.URL.Path)
						if req.URL.Scheme != "https" {
							t.Error("registry lookup lost HTTPS")
						}
						if req.URL.Host == "auth.docker.io" {
							if req.Method != http.MethodGet || req.URL.Path != "/token" || req.URL.Query().Get("service") != "registry.docker.io" || req.URL.Query().Get("scope") != "repository:"+wantRepo+":pull" {
								t.Error("token request lost the canonical Hub pull scope")
							}
							user, secret, ok := req.BasicAuth()
							if credentialed && (!ok || user != "synthetic-user" || secret != "synthetic-secret") {
								t.Error("legacy Hub credential key was not resolved")
							} else if !credentialed && req.Header.Get("Authorization") != "" {
								t.Error("anonymous lookup attached credentials")
							}
							return newStringResponse(200, nil, `{"token":"synthetic-hub-token"}`), nil
						}
						if req.URL.Host != "registry-1.docker.io" || req.URL.Path != "/v2/"+wantRepo+"/manifests/stable" {
							t.Error("manifest lookup left the canonical Hub route")
							return newStringResponse(200, nil, `<html>not a registry</html>`), nil
						}
						if req.Header.Get("Authorization") != "Bearer synthetic-hub-token" || !strings.Contains(req.Header.Get("Accept"), "application/vnd.oci.image.index.v1+json") {
							t.Error("manifest request lost its token or multi-platform Accept header")
						}
						headers := map[string]string{"Content-Type": "application/vnd.oci.image.index.v1+json"}
						if req.Method == http.MethodHead {
							return newStringResponse(200, headers, ""), nil
						}
						return newStringResponse(200, headers, index), nil
					})}
					local := "sha256:other,sha256:amd64"
					got := checker.CheckImageUpdate(context.Background(), ref, local, "amd64", "linux", "")
					if got == nil || got.Error != "" || got.Image != ref || got.CurrentDigest != "sha256:other" || got.LatestDigest != indexDigest || got.UpdateAvailable {
						t.Fatalf("same-platform lookup = %+v", got)
					}
					if len(requests) != 3 {
						t.Fatalf("requests = %v, want token, HEAD and GET", requests)
					}
					for _, cachedRef := range []string{wantRepo + ":stable", "docker.io/" + wantRepo + ":stable", "index.docker.io/" + wantRepo + ":stable"} {
						got = checker.CheckImageUpdate(context.Background(), cachedRef, "sha256:old", "amd64", "linux", "")
						if got == nil || got.Error != "" || !got.UpdateAvailable || got.LatestDigest != indexDigest {
							t.Fatalf("changed-image cached lookup = %+v", got)
						}
					}
					if len(requests) != 3 {
						t.Error("equivalent Hub aliases repeated a cached registry lookup")
					}
					got = checker.CheckImageUpdate(context.Background(), ref, "sha256:armv7", "arm", "linux", "v7")
					if got == nil || got.Error != "" || got.UpdateAvailable || got.LatestDigest != indexDigest || len(requests) != 6 {
						t.Fatalf("ARMv7 lookup did not keep its separate platform cache: %+v", got)
					}
				})
			}
		}
	}
}

func TestDockerHubAliasLookupFailures(t *testing.T) {
	for _, tt := range []struct {
		name        string
		tokenStatus int
		headStatus  int
		body        string
		wantError   string
	}{
		{"HTML", 200, 200, `<html>login</html>`, "non-manifest response"},
		{"JSON error", 200, 200, `{"errors":[{"code":"UNAVAILABLE"}]}`, "non-manifest response"},
		{"token refused", 401, 200, "", "token request failed: 401"},
		{"unauthorized", 200, 401, "", "authentication required"},
		{"not found", 200, 404, "", "image not found"},
		{"rate limited", 200, 429, "", "rate limited"},
		{"server error", 200, 500, "", "registry error: 500"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewRegistryChecker(zerolog.Nop())
			calls := 0
			checker.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.URL.Host == "auth.docker.io" {
					return newStringResponse(tt.tokenStatus, nil, `{"token":"synthetic"}`), nil
				}
				if req.URL.Host != "registry-1.docker.io" {
					t.Error("failure lookup left the canonical Hub route")
				}
				if req.Method == http.MethodHead {
					return newStringResponse(tt.headStatus, nil, ""), nil
				}
				return newStringResponse(200, nil, tt.body), nil
			})}
			for _, ref := range []string{"docker.io/nginx:stable", "index.docker.io/library/nginx:stable"} {
				got := checker.CheckImageUpdate(context.Background(), ref, "sha256:old", "amd64", "linux", "")
				if got == nil || !strings.Contains(got.Error, tt.wantError) || got.LatestDigest != "" || got.UpdateAvailable {
					t.Fatalf("failure invented an update: %+v", got)
				}
				if ref == "docker.io/nginx:stable" {
					continue
				}
				wantCalls := 2
				if tt.tokenStatus != 200 {
					wantCalls = 1
				} else if tt.headStatus == 200 {
					wantCalls = 3
				}
				if calls != wantCalls {
					t.Fatalf("failure cache issued %d calls, want %d", calls, wantCalls)
				}
			}
		})
	}
}

func TestDockerHubAliasesPreserveManagedAndPinnedExclusions(t *testing.T) {
	checker := NewRegistryChecker(zerolog.Nop())
	checker.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("excluded image attempted a registry request")
		return nil, nil
	})}
	for _, host := range []string{"", "docker.io/", "index.docker.io/", "registry-1.docker.io/", "registry.docker.io/"} {
		for _, suffix := range []string{":latest", "@sha256:" + strings.Repeat("a", 64)} {
			ref := host + "rcourtman/pulse" + suffix
			if got := checker.CheckImageUpdate(context.Background(), ref, "sha256:old", "", "", ""); got != nil {
				t.Errorf("managed alias %q returned %+v", ref, got)
			}
		}
		ref := host + "nginx@sha256:" + strings.Repeat("a", 64)
		got := checker.CheckImageUpdate(context.Background(), ref, "sha256:old", "", "", "")
		if got == nil || got.Error != "digest-pinned image" || got.UpdateAvailable || got.LatestDigest != "" {
			t.Errorf("pinned alias returned %+v", got)
		}
	}
	for _, ref := range []string{"docker.io/rcourtman/pulse-extra:latest", "docker.io/another/pulse:latest", "docker.io:5000/rcourtman/pulse:latest", "docker.io.example/rcourtman/pulse:latest"} {
		if isPulseManagedImageReference(ref) {
			t.Errorf("non-managed reference %q was excluded", ref)
		}
	}
}
