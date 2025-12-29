package updatedetection

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (int, error) {
	return 0, e.err
}

func newResponse(status int, headers http.Header, body io.Reader) *http.Response {
	if headers == nil {
		headers = http.Header{}
	}
	if body == nil {
		body = bytes.NewReader(nil)
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     headers,
		Body:       io.NopCloser(body),
	}
}

func TestRegistryCheckerConfig(t *testing.T) {
	r := NewRegistryChecker(zerolog.Nop())
	if r.httpClient == nil || r.cache == nil || r.configs == nil {
		t.Fatalf("expected registry checker to initialize")
	}

	cfg := RegistryConfig{Host: "example.com", Username: "user", Password: "pass", Insecure: true}
	r.AddRegistryConfig(cfg)

	r.mu.RLock()
	stored := r.configs["example.com"]
	r.mu.RUnlock()
	if stored.Username != "user" || !stored.Insecure {
		t.Fatalf("expected config to be stored")
	}
}

func TestRegistryCache(t *testing.T) {
	r := NewRegistryChecker(zerolog.Nop())
	r.cacheDigest("key-digest", "sha256:abc")
	r.cacheError("key-error", "boom")

	if r.CacheSize() != 2 {
		t.Fatalf("expected cache size 2, got %d", r.CacheSize())
	}
	if entry := r.getCached("key-digest"); entry == nil || entry.digest != "sha256:abc" {
		t.Fatalf("expected cached digest")
	}
	if entry := r.getCached("missing"); entry != nil {
		t.Fatalf("expected missing cache entry to be nil")
	}

	r.cache.entries["expired"] = cacheEntry{
		digest:    "old",
		expiresAt: time.Now().Add(-time.Minute),
	}
	if entry := r.getCached("expired"); entry != nil {
		t.Fatalf("expected expired cache entry to be nil")
	}

	r.CleanupCache()
	if r.CacheSize() != 2 {
		t.Fatalf("expected expired cache entry to be removed")
	}
}

func TestRegistryCheckImageUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("DigestPinned", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		info, err := r.CheckImageUpdate(ctx, "nginx@sha256:abc", "sha256:old")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.Error == "" {
			t.Fatalf("expected digest-pinned error")
		}
	})

	t.Run("CachedError", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		key := "registry-1.docker.io/library/nginx:latest"
		r.cache.entries[key] = cacheEntry{
			err:       "cached error",
			expiresAt: time.Now().Add(time.Hour),
		}

		info, err := r.CheckImageUpdate(ctx, "nginx:latest", "sha256:old")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.Error != "cached error" {
			t.Fatalf("expected cached error")
		}
	})

	t.Run("CachedDigest", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		key := "registry-1.docker.io/library/nginx:latest"
		r.cache.entries[key] = cacheEntry{
			digest:    "sha256:new",
			expiresAt: time.Now().Add(time.Hour),
		}

		info, err := r.CheckImageUpdate(ctx, "nginx:latest", "sha256:old")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !info.UpdateAvailable || info.LatestDigest != "sha256:new" {
			t.Fatalf("expected update available from cache")
		}
	})

	t.Run("FetchErrorCaches", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return newResponse(http.StatusInternalServerError, nil, nil), nil
			}),
		}

		info, err := r.CheckImageUpdate(ctx, "example.com/repo:tag", "sha256:old")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.Error == "" {
			t.Fatalf("expected error from fetch")
		}
		if cached := r.getCached("example.com/repo:tag"); cached == nil || cached.err == "" {
			t.Fatalf("expected error to be cached")
		}
	})

	t.Run("FetchSuccessCaches", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				headers := http.Header{"Docker-Content-Digest": []string{"sha256:new"}}
				return newResponse(http.StatusOK, headers, nil), nil
			}),
		}

		info, err := r.CheckImageUpdate(ctx, "example.com/repo:tag", "sha256:old")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !info.UpdateAvailable {
			t.Fatalf("expected update available")
		}
		if cached := r.getCached("example.com/repo:tag"); cached == nil || cached.digest == "" {
			t.Fatalf("expected digest to be cached")
		}
	})
}

func TestFetchDigest(t *testing.T) {
	ctx := context.Background()

	t.Run("AuthError", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("auth fail")
			}),
		}

		if _, err := r.fetchDigest(ctx, "registry-1.docker.io", "library/nginx", "latest"); err == nil {
			t.Fatalf("expected auth error")
		}
	})

	t.Run("InsecureScheme", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.AddRegistryConfig(RegistryConfig{Host: "insecure.local", Insecure: true})
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Scheme != "http" {
					t.Fatalf("expected http scheme, got %q", req.URL.Scheme)
				}
				headers := http.Header{"Docker-Content-Digest": []string{"sha256:abc"}}
				return newResponse(http.StatusOK, headers, nil), nil
			}),
		}

		if _, err := r.fetchDigest(ctx, "insecure.local", "repo", "latest"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("RequestError", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		if _, err := r.fetchDigest(ctx, "bad host", "repo", "latest"); err == nil {
			t.Fatalf("expected request creation error")
		}
	})

	t.Run("TokenHeader", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.AddRegistryConfig(RegistryConfig{Host: "ghcr.io", Password: "pat"})
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if got := req.Header.Get("Authorization"); got != "Bearer pat" {
					t.Fatalf("expected bearer token, got %q", got)
				}
				headers := http.Header{"Docker-Content-Digest": []string{"sha256:abc"}}
				return newResponse(http.StatusOK, headers, nil), nil
			}),
		}

		if _, err := r.fetchDigest(ctx, "ghcr.io", "owner/repo", "latest"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("DoError", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("network")
			}),
		}

		if _, err := r.fetchDigest(ctx, "example.com", "repo", "latest"); err == nil {
			t.Fatalf("expected request error")
		}
	})

	t.Run("StatusUnauthorized", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return newResponse(http.StatusUnauthorized, nil, nil), nil
			}),
		}

		if _, err := r.fetchDigest(ctx, "example.com", "repo", "latest"); err == nil {
			t.Fatalf("expected unauthorized error")
		}
	})

	t.Run("StatusNotFound", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return newResponse(http.StatusNotFound, nil, nil), nil
			}),
		}

		if _, err := r.fetchDigest(ctx, "example.com", "repo", "latest"); err == nil {
			t.Fatalf("expected not found error")
		}
	})

	t.Run("StatusRateLimited", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return newResponse(http.StatusTooManyRequests, nil, nil), nil
			}),
		}

		if _, err := r.fetchDigest(ctx, "example.com", "repo", "latest"); err == nil {
			t.Fatalf("expected rate limit error")
		}
	})

	t.Run("StatusOtherError", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return newResponse(http.StatusInternalServerError, nil, nil), nil
			}),
		}

		if _, err := r.fetchDigest(ctx, "example.com", "repo", "latest"); err == nil {
			t.Fatalf("expected registry error")
		}
	})

	t.Run("DigestHeader", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				headers := http.Header{"Docker-Content-Digest": []string{"sha256:abc"}}
				return newResponse(http.StatusOK, headers, nil), nil
			}),
		}

		digest, err := r.fetchDigest(ctx, "example.com", "repo", "latest")
		if err != nil || digest != "sha256:abc" {
			t.Fatalf("expected digest, got %q err %v", digest, err)
		}
	})

	t.Run("EtagHeader", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				headers := http.Header{"Etag": []string{`"sha256:etag"`}}
				return newResponse(http.StatusOK, headers, nil), nil
			}),
		}

		digest, err := r.fetchDigest(ctx, "example.com", "repo", "latest")
		if err != nil || digest != "sha256:etag" {
			t.Fatalf("expected etag digest, got %q err %v", digest, err)
		}
	})

	t.Run("NoDigest", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return newResponse(http.StatusOK, nil, nil), nil
			}),
		}

		if _, err := r.fetchDigest(ctx, "example.com", "repo", "latest"); err == nil {
			t.Fatalf("expected missing digest error")
		}
	})
}

func TestGetAuthToken(t *testing.T) {
	ctx := context.Background()

	t.Run("DockerHubSuccess", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.AddRegistryConfig(RegistryConfig{Host: "registry-1.docker.io", Username: "user", Password: "pass"})
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.Header.Get("Authorization"), "Basic ") {
					t.Fatalf("expected basic auth header")
				}
				body := `{"token":"tok"}`
				return newResponse(http.StatusOK, nil, strings.NewReader(body)), nil
			}),
		}

		token, err := r.getAuthToken(ctx, "registry-1.docker.io", "library/nginx")
		if err != nil || token != "tok" {
			t.Fatalf("expected token, got %q err %v", token, err)
		}
	})

	t.Run("DockerHubRequestError", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		if _, err := r.getAuthToken(ctx, "registry-1.docker.io", "bad\nrepo"); err == nil {
			t.Fatalf("expected request error")
		}
	})

	t.Run("DockerHubDoError", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("network")
			}),
		}

		if _, err := r.getAuthToken(ctx, "registry-1.docker.io", "library/nginx"); err == nil {
			t.Fatalf("expected network error")
		}
	})

	t.Run("DockerHubStatusError", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return newResponse(http.StatusInternalServerError, nil, nil), nil
			}),
		}

		if _, err := r.getAuthToken(ctx, "registry-1.docker.io", "library/nginx"); err == nil {
			t.Fatalf("expected status error")
		}
	})

	t.Run("DockerHubReadError", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				body := &errorReader{err: errors.New("read")}
				return newResponse(http.StatusOK, nil, body), nil
			}),
		}

		if _, err := r.getAuthToken(ctx, "registry-1.docker.io", "library/nginx"); err == nil {
			t.Fatalf("expected read error")
		}
	})

	t.Run("DockerHubJSONError", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				body := "{"
				return newResponse(http.StatusOK, nil, strings.NewReader(body)), nil
			}),
		}

		if _, err := r.getAuthToken(ctx, "registry-1.docker.io", "library/nginx"); err == nil {
			t.Fatalf("expected json error")
		}
	})

	t.Run("GHCRToken", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.AddRegistryConfig(RegistryConfig{Host: "ghcr.io", Password: "pat"})
		token, err := r.getAuthToken(ctx, "ghcr.io", "owner/repo")
		if err != nil || token != "pat" {
			t.Fatalf("expected ghcr token")
		}
	})

	t.Run("GHCRAnonymous", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		token, err := r.getAuthToken(ctx, "ghcr.io", "owner/repo")
		if err != nil || token != "" {
			t.Fatalf("expected empty ghcr token")
		}
	})

	t.Run("OtherRegistryBasicAuth", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		r.AddRegistryConfig(RegistryConfig{Host: "example.com", Username: "user", Password: "pass"})
		token, err := r.getAuthToken(ctx, "example.com", "repo")
		if err != nil || token != "" {
			t.Fatalf("expected empty token for basic auth registry")
		}
	})

	t.Run("OtherRegistryNoAuth", func(t *testing.T) {
		r := NewRegistryChecker(zerolog.Nop())
		token, err := r.getAuthToken(ctx, "example.com", "repo")
		if err != nil || token != "" {
			t.Fatalf("expected empty token for registry without auth")
		}
	})
}
