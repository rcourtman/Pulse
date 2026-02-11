package dockeragent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errReadCloser struct {
	err error
}

func (e errReadCloser) Read(_ []byte) (int, error) {
	return 0, e.err
}

func (e errReadCloser) Close() error {
	return nil
}

func newStringResponse(status int, headers map[string]string, body string) *http.Response {
	resp := &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	for key, value := range headers {
		resp.Header.Set(key, value)
	}
	return resp
}

func TestRegistryChecker_CheckImageUpdate_CacheHits(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("cached error", func(t *testing.T) {
		checker := NewRegistryChecker(logger)
		cacheKey := "example.test/repo:tag|//"
		checker.cacheError(cacheKey, "cached error")

		result := checker.CheckImageUpdate(context.Background(), "example.test/repo:tag", "sha256:current", "", "", "")
		if result == nil {
			t.Fatal("Expected result for cached error")
		}
		if result.Error != "cached error" {
			t.Errorf("Expected cached error, got %q", result.Error)
		}
		if result.UpdateAvailable {
			t.Error("Expected no update when cached error is present")
		}
	})

	t.Run("cached digest", func(t *testing.T) {
		checker := NewRegistryChecker(logger)
		cacheKey := "example.test/repo:tag|//"
		checker.cacheDigest(cacheKey, "sha256:latest")

		result := checker.CheckImageUpdate(context.Background(), "example.test/repo:tag", "sha256:current", "", "", "")
		if result == nil {
			t.Fatal("Expected result for cached digest")
		}
		if result.LatestDigest != "sha256:latest" {
			t.Errorf("Expected latest digest sha256:latest, got %q", result.LatestDigest)
		}
		if !result.UpdateAvailable {
			t.Error("Expected update available for cached digest")
		}
	})
}

func TestRegistryChecker_CheckImageUpdate_FetchPaths(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("fetch error caches error", func(t *testing.T) {
		checker := NewRegistryChecker(logger)
		checker.httpClient = &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return newStringResponse(http.StatusInternalServerError, nil, ""), nil
			}),
		}

		result := checker.CheckImageUpdate(context.Background(), "example.test/repo:tag", "sha256:current", "", "", "")
		if result == nil {
			t.Fatal("Expected result on fetch error")
		}
		if result.Error != "registry error: 500" {
			t.Fatalf("Expected registry error, got %q", result.Error)
		}

		cacheKey := "example.test/repo:tag|//"
		cached := checker.getCached(cacheKey)
		if cached == nil || cached.err != "registry error: 500" {
			t.Fatalf("Expected cached error to be stored, got %+v", cached)
		}
	})

	t.Run("fetch success caches digest", func(t *testing.T) {
		checker := NewRegistryChecker(logger)
		checker.httpClient = &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				headers := map[string]string{
					"Docker-Content-Digest": "sha256:latest",
				}
				return newStringResponse(http.StatusOK, headers, ""), nil
			}),
		}

		result := checker.CheckImageUpdate(context.Background(), "example.test/repo:tag", "sha256:current", "", "", "")
		if result == nil {
			t.Fatal("Expected result on fetch success")
		}
		if result.LatestDigest != "sha256:latest" {
			t.Fatalf("Expected latest digest sha256:latest, got %q", result.LatestDigest)
		}
		if !result.UpdateAvailable {
			t.Fatal("Expected update available for new digest")
		}

		cacheKey := "example.test/repo:tag|//"
		cached := checker.getCached(cacheKey)
		if cached == nil || cached.latestDigest != "sha256:latest" {
			t.Fatalf("Expected cached digest to be stored, got %+v", cached)
		}
	})
}

func TestRegistryChecker_GetCached_ExpiredEntry(t *testing.T) {
	checker := NewRegistryChecker(zerolog.Nop())
	checker.cache.entries["expired"] = cacheEntry{
		latestDigest: "sha256:old",
		expiresAt:    time.Now().Add(-time.Minute),
	}

	if got := checker.getCached("expired"); got != nil {
		t.Fatalf("Expected expired cache entry to return nil, got %+v", got)
	}
}

func TestRegistryChecker_ForceCheck(t *testing.T) {
	checker := NewRegistryChecker(zerolog.Nop())
	checker.MarkChecked()
	checker.cacheDigest("test-key", "sha256:test")

	checker.ForceCheck()

	checker.mu.RLock()
	lastFullCheck := checker.lastFullCheck
	checker.mu.RUnlock()
	if !lastFullCheck.IsZero() {
		t.Fatalf("expected ForceCheck to reset lastFullCheck, got %s", lastFullCheck)
	}

	checker.cache.mu.RLock()
	cacheLen := len(checker.cache.entries)
	checker.cache.mu.RUnlock()
	if cacheLen != 0 {
		t.Fatalf("expected ForceCheck to clear cache, found %d entries", cacheLen)
	}
}

func TestRegistryChecker_FetchDigest_StatusErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr string
	}{
		{
			name:    "unauthorized",
			status:  http.StatusUnauthorized,
			wantErr: "authentication required",
		},
		{
			name:    "not found",
			status:  http.StatusNotFound,
			wantErr: "image not found",
		},
		{
			name:    "rate limited",
			status:  http.StatusTooManyRequests,
			wantErr: "rate limited",
		},
		{
			name:    "registry error",
			status:  http.StatusInternalServerError,
			wantErr: "registry error: 500",
		},
		{
			name:    "missing digest",
			status:  http.StatusOK,
			wantErr: "no digest in response",
		},
	}

	expectedAccept := strings.Join([]string{
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.oci.image.index.v1+json",
	}, ", ")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotAccept string
			checker := &RegistryChecker{
				httpClient: &http.Client{
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						gotAccept = req.Header.Get("Accept")
						return newStringResponse(tt.status, nil, ""), nil
					}),
				},
			}

			_, _, err := checker.fetchDigest(context.Background(), "example.test", "repo", "tag", "", "", "")
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("Expected error %q, got %v", tt.wantErr, err)
			}
			if gotAccept != expectedAccept {
				t.Fatalf("Expected Accept header %q, got %q", expectedAccept, gotAccept)
			}
		})
	}
}

func TestRegistryChecker_FetchDigest_RequestError(t *testing.T) {
	checker := &RegistryChecker{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("boom")
			}),
		},
	}

	_, _, err := checker.fetchDigest(context.Background(), "example.test", "repo", "tag", "", "", "")
	if err == nil || !strings.Contains(err.Error(), "request:") {
		t.Fatalf("Expected request error, got %v", err)
	}
}

func TestRegistryChecker_FetchDigest_RequestCreationError(t *testing.T) {
	checker := &RegistryChecker{httpClient: &http.Client{}}

	_, _, err := checker.fetchDigest(context.Background(), "bad host", "repo", "tag", "", "", "")
	if err == nil || !strings.Contains(err.Error(), "create request:") {
		t.Fatalf("Expected create request error, got %v", err)
	}
}

func TestRegistryChecker_FetchDigest_DigestHeaders(t *testing.T) {
	expectedAccept := strings.Join([]string{
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.oci.image.index.v1+json",
	}, ", ")

	t.Run("docker content digest header", func(t *testing.T) {
		var gotAccept string
		checker := &RegistryChecker{
			httpClient: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotAccept = req.Header.Get("Accept")
					headers := map[string]string{
						"Docker-Content-Digest": "sha256:abc123",
					}
					return newStringResponse(http.StatusOK, headers, ""), nil
				}),
			},
		}

		digest, _, err := checker.fetchDigest(context.Background(), "example.test", "repo", "tag", "", "", "")
		if err != nil {
			t.Fatalf("Expected digest, got error %v", err)
		}
		if digest != "sha256:abc123" {
			t.Fatalf("Expected digest sha256:abc123, got %q", digest)
		}
		if gotAccept != expectedAccept {
			t.Fatalf("Expected Accept header %q, got %q", expectedAccept, gotAccept)
		}
	})

	t.Run("etag digest header", func(t *testing.T) {
		checker := &RegistryChecker{
			httpClient: &http.Client{
				Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
					headers := map[string]string{
						"Etag": "\"sha256:etag\"",
					}
					return newStringResponse(http.StatusOK, headers, ""), nil
				}),
			},
		}

		digest, _, err := checker.fetchDigest(context.Background(), "example.test", "repo", "tag", "", "", "")
		if err != nil {
			t.Fatalf("Expected digest, got error %v", err)
		}
		if digest != "sha256:etag" {
			t.Fatalf("Expected digest sha256:etag, got %q", digest)
		}
	})
}

func TestRegistryChecker_FetchDigest_AuthPaths(t *testing.T) {
	t.Run("auth token error", func(t *testing.T) {
		checker := &RegistryChecker{
			httpClient: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.URL.Host == "auth.docker.io" {
						return newStringResponse(http.StatusInternalServerError, nil, ""), nil
					}
					return nil, errors.New("unexpected manifest request")
				}),
			},
		}

		_, _, err := checker.fetchDigest(context.Background(), "registry-1.docker.io", "library/nginx", "latest", "", "", "")
		if err == nil || err.Error() != "auth: token request failed: 500" {
			t.Fatalf("Expected auth error, got %v", err)
		}
	})

	t.Run("auth token header set", func(t *testing.T) {
		var gotAuth string
		checker := &RegistryChecker{
			httpClient: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.URL.Host {
					case "auth.docker.io":
						return newStringResponse(http.StatusOK, nil, `{"token":"token123"}`), nil
					case "registry-1.docker.io":
						gotAuth = req.Header.Get("Authorization")
						headers := map[string]string{
							"Docker-Content-Digest": "sha256:latest",
						}
						return newStringResponse(http.StatusOK, headers, ""), nil
					default:
						return nil, errors.New("unexpected host")
					}
				}),
			},
		}

		digest, _, err := checker.fetchDigest(context.Background(), "registry-1.docker.io", "library/nginx", "latest", "", "", "")
		if err != nil {
			t.Fatalf("Expected digest, got error %v", err)
		}
		if digest != "sha256:latest" {
			t.Fatalf("Expected digest sha256:latest, got %q", digest)
		}
		if gotAuth != "Bearer token123" {
			t.Fatalf("Expected Authorization header, got %q", gotAuth)
		}
	})
}

func TestRegistryChecker_GetAuthToken(t *testing.T) {
	t.Run("docker hub", func(t *testing.T) {
		var gotURL string
		checker := &RegistryChecker{
			httpClient: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotURL = req.URL.String()
					return newStringResponse(http.StatusOK, nil, `{"token":"dockertoken"}`), nil
				}),
			},
		}

		token, err := checker.getAuthToken(context.Background(), "registry-1.docker.io", "library/nginx")
		if err != nil {
			t.Fatalf("Expected token, got error %v", err)
		}
		if token != "dockertoken" {
			t.Fatalf("Expected dockertoken, got %q", token)
		}
		if !strings.Contains(gotURL, "service=registry.docker.io") || !strings.Contains(gotURL, "scope=repository:library/nginx:pull") {
			t.Fatalf("Unexpected token URL %q", gotURL)
		}
	})

	t.Run("ghcr", func(t *testing.T) {
		var gotURL string
		checker := &RegistryChecker{
			httpClient: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotURL = req.URL.String()
					return newStringResponse(http.StatusOK, nil, `{"token":"ghcrtoken"}`), nil
				}),
			},
		}

		token, err := checker.getAuthToken(context.Background(), "ghcr.io", "owner/repo")
		if err != nil {
			t.Fatalf("Expected token, got error %v", err)
		}
		if token != "ghcrtoken" {
			t.Fatalf("Expected ghcrtoken, got %q", token)
		}
		if !strings.Contains(gotURL, "service=ghcr.io") || !strings.Contains(gotURL, "scope=repository:owner/repo:pull") {
			t.Fatalf("Unexpected token URL %q", gotURL)
		}
	})

	t.Run("other registry", func(t *testing.T) {
		checker := &RegistryChecker{}
		token, err := checker.getAuthToken(context.Background(), "example.test", "repo")
		if err != nil {
			t.Fatalf("Expected nil error, got %v", err)
		}
		if token != "" {
			t.Fatalf("Expected empty token, got %q", token)
		}
	})
}

func TestRegistryChecker_FetchAuthToken(t *testing.T) {
	t.Run("bad url", func(t *testing.T) {
		checker := &RegistryChecker{httpClient: &http.Client{}}
		_, err := checker.fetchAuthToken(context.Background(), "http://bad host")
		if err == nil {
			t.Fatal("Expected error for bad URL")
		}
	})

	t.Run("request error", func(t *testing.T) {
		checker := &RegistryChecker{
			httpClient: &http.Client{
				Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
					return nil, errors.New("transport failure")
				}),
			},
		}

		_, err := checker.fetchAuthToken(context.Background(), "https://auth.example.test/token")
		if err == nil {
			t.Fatal("Expected request error")
		}
	})

	t.Run("status error", func(t *testing.T) {
		checker := &RegistryChecker{
			httpClient: &http.Client{
				Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
					return newStringResponse(http.StatusInternalServerError, nil, ""), nil
				}),
			},
		}

		_, err := checker.fetchAuthToken(context.Background(), "https://auth.example.test/token")
		if err == nil || err.Error() != "token request failed: 500" {
			t.Fatalf("Expected status error, got %v", err)
		}
	})

	t.Run("read error", func(t *testing.T) {
		checker := &RegistryChecker{
			httpClient: &http.Client{
				Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
					resp := &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       errReadCloser{err: errors.New("read failure")},
					}
					return resp, nil
				}),
			},
		}

		_, err := checker.fetchAuthToken(context.Background(), "https://auth.example.test/token")
		if err == nil {
			t.Fatal("Expected read error")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		checker := &RegistryChecker{
			httpClient: &http.Client{
				Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
					return newStringResponse(http.StatusOK, nil, "{"), nil
				}),
			},
		}

		_, err := checker.fetchAuthToken(context.Background(), "https://auth.example.test/token")
		if err == nil {
			t.Fatal("Expected JSON error")
		}
	})

	t.Run("success", func(t *testing.T) {
		checker := &RegistryChecker{
			httpClient: &http.Client{
				Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
					return newStringResponse(http.StatusOK, nil, `{"token":"ok"}`), nil
				}),
			},
		}

		token, err := checker.fetchAuthToken(context.Background(), "https://auth.example.test/token")
		if err != nil {
			t.Fatalf("Expected token, got error %v", err)
		}
		if token != "ok" {
			t.Fatalf("Expected token ok, got %q", token)
		}
	})
}
