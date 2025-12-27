package updatedetection

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// RegistryConfig holds authentication for a container registry.
type RegistryConfig struct {
	Host     string `json:"host"`              // e.g., "registry-1.docker.io", "ghcr.io"
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"` // token or password
	Insecure bool   `json:"insecure,omitempty"` // Skip TLS verification
}

// RegistryChecker handles digest lookups against container registries.
type RegistryChecker struct {
	httpClient *http.Client
	configs    map[string]RegistryConfig // keyed by registry host
	cache      *digestCache
	logger     zerolog.Logger
	mu         sync.RWMutex
}

// digestCache provides thread-safe caching of digest lookups.
type digestCache struct {
	entries map[string]cacheEntry
	mu      sync.RWMutex
}

type cacheEntry struct {
	digest    string
	expiresAt time.Time
	err       string // cached error message
}

// DefaultCacheTTL is the default time-to-live for cached digests.
const DefaultCacheTTL = 6 * time.Hour

// ErrorCacheTTL is the TTL for caching errors (shorter to allow retry).
const ErrorCacheTTL = 15 * time.Minute

// NewRegistryChecker creates a new registry checker.
func NewRegistryChecker(logger zerolog.Logger) *RegistryChecker {
	return &RegistryChecker{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
				MaxIdleConns:        10,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
				DisableKeepAlives:   false,
			},
		},
		configs: make(map[string]RegistryConfig),
		cache: &digestCache{
			entries: make(map[string]cacheEntry),
		},
		logger: logger,
	}
}

// AddRegistryConfig adds or updates registry authentication configuration.
func (r *RegistryChecker) AddRegistryConfig(cfg RegistryConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[cfg.Host] = cfg
}

// ParseImageReference parses an image reference into registry, repository, and tag.
// Examples:
//   - "nginx" -> registry-1.docker.io, library/nginx, latest
//   - "nginx:1.25" -> registry-1.docker.io, library/nginx, 1.25
//   - "myrepo/myapp:v1" -> registry-1.docker.io, myrepo/myapp, v1
//   - "ghcr.io/owner/repo:tag" -> ghcr.io, owner/repo, tag
//   - "registry.example.com:5000/app:v2" -> registry.example.com:5000, app, v2
func ParseImageReference(image string) (registry, repository, tag string) {
	// Default values
	registry = "registry-1.docker.io"
	tag = "latest"

	// Check if this is a digest-pinned image (image@sha256:...)
	if strings.Contains(image, "@sha256:") {
		// Digest-pinned images cannot be updated via tag comparison
		return "", "", ""
	}

	// Split off the tag first
	parts := strings.Split(image, ":")
	if len(parts) > 1 {
		// Check if the last part looks like a tag (not a port)
		lastPart := parts[len(parts)-1]
		if !strings.Contains(lastPart, "/") {
			tag = lastPart
			image = strings.Join(parts[:len(parts)-1], ":")
		}
	}

	// Now parse the registry and repository
	parts = strings.Split(image, "/")

	// If first part looks like a registry (contains . or :, or is localhost)
	if len(parts) > 1 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") || parts[0] == "localhost") {
		registry = parts[0]
		repository = strings.Join(parts[1:], "/")
	} else if len(parts) == 1 {
		// Official image (e.g., "nginx")
		repository = "library/" + parts[0]
	} else {
		// Docker Hub with namespace (e.g., "myrepo/myapp")
		repository = image
	}

	return registry, repository, tag
}

// CheckImageUpdate compares current digest with registry's latest.
func (r *RegistryChecker) CheckImageUpdate(ctx context.Context, image, currentDigest string) (*ImageUpdateInfo, error) {
	registry, repository, tag := ParseImageReference(image)
	
	// Skip digest-pinned images
	if registry == "" {
		return &ImageUpdateInfo{
			Image:           image,
			CurrentDigest:   currentDigest,
			UpdateAvailable: false,
			CheckedAt:       time.Now(),
			Error:           "digest-pinned image, cannot check for updates",
		}, nil
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s/%s:%s", registry, repository, tag)
	if cached := r.getCached(cacheKey); cached != nil {
		if cached.err != "" {
			return &ImageUpdateInfo{
				Image:           image,
				CurrentDigest:   currentDigest,
				UpdateAvailable: false,
				CheckedAt:       time.Now(),
				Error:           cached.err,
			}, nil
		}
		return &ImageUpdateInfo{
			Image:           image,
			CurrentDigest:   currentDigest,
			LatestDigest:    cached.digest,
			UpdateAvailable: currentDigest != "" && cached.digest != "" && currentDigest != cached.digest,
			CheckedAt:       time.Now(),
		}, nil
	}

	// Fetch latest digest from registry
	latestDigest, err := r.fetchDigest(ctx, registry, repository, tag)
	if err != nil {
		// Cache the error to avoid hammering the registry
		r.cacheError(cacheKey, err.Error())
		
		r.logger.Debug().
			Str("image", image).
			Str("registry", registry).
			Err(err).
			Msg("Failed to fetch image digest from registry")

		return &ImageUpdateInfo{
			Image:           image,
			CurrentDigest:   currentDigest,
			UpdateAvailable: false,
			CheckedAt:       time.Now(),
			Error:           err.Error(),
		}, nil
	}

	// Cache the successful result
	r.cacheDigest(cacheKey, latestDigest)

	return &ImageUpdateInfo{
		Image:           image,
		CurrentDigest:   currentDigest,
		LatestDigest:    latestDigest,
		UpdateAvailable: currentDigest != "" && latestDigest != "" && currentDigest != latestDigest,
		CheckedAt:       time.Now(),
	}, nil
}

// fetchDigest retrieves the digest for an image from the registry.
func (r *RegistryChecker) fetchDigest(ctx context.Context, registry, repository, tag string) (string, error) {
	// Get auth token if needed
	token, err := r.getAuthToken(ctx, registry, repository)
	if err != nil {
		return "", fmt.Errorf("auth: %w", err)
	}

	// Construct the manifest URL
	scheme := "https"
	r.mu.RLock()
	if cfg, ok := r.configs[registry]; ok && cfg.Insecure {
		scheme = "http"
	}
	r.mu.RUnlock()

	manifestURL := fmt.Sprintf("%s://%s/v2/%s/manifests/%s", scheme, registry, repository, tag)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, manifestURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	// Accept headers for multi-arch manifest support
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.oci.image.index.v1+json",
	}, ", "))

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("authentication required")
	}
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("image not found")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", fmt.Errorf("rate limited")
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("registry error: %d", resp.StatusCode)
	}

	// Get digest from Docker-Content-Digest header
	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		// Some registries don't return digest on HEAD, try etag
		digest = resp.Header.Get("Etag")
		if digest != "" {
			// Clean up etag format
			digest = strings.Trim(digest, "\"")
		}
	}

	if digest == "" {
		return "", fmt.Errorf("no digest in response")
	}

	return digest, nil
}

// getAuthToken retrieves an auth token for the registry.
// For Docker Hub, this implements the v2 token flow.
func (r *RegistryChecker) getAuthToken(ctx context.Context, registry, repository string) (string, error) {
	r.mu.RLock()
	cfg, hasConfig := r.configs[registry]
	r.mu.RUnlock()

	// Docker Hub requires auth token even for public images
	if registry == "registry-1.docker.io" {
		tokenURL := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", repository)
		
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
		if err != nil {
			return "", err
		}

		// Add basic auth if configured
		if hasConfig && cfg.Username != "" && cfg.Password != "" {
			req.SetBasicAuth(cfg.Username, cfg.Password)
		}

		resp, err := r.httpClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("token request failed: %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}

		var tokenResp struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			return "", err
		}

		return tokenResp.Token, nil
	}

	// For GitHub Container Registry
	if registry == "ghcr.io" {
		if hasConfig && cfg.Password != "" {
			// Use the password as a PAT token
			return cfg.Password, nil
		}
		// GHCR allows anonymous access for public images
		return "", nil
	}

	// For other registries with basic auth
	if hasConfig && cfg.Username != "" && cfg.Password != "" {
		// Return empty - we'll use basic auth directly
		return "", nil
	}

	return "", nil
}

func (r *RegistryChecker) getCached(key string) *cacheEntry {
	r.cache.mu.RLock()
	defer r.cache.mu.RUnlock()

	entry, ok := r.cache.entries[key]
	if !ok {
		return nil
	}
	if time.Now().After(entry.expiresAt) {
		return nil
	}
	return &entry
}

func (r *RegistryChecker) cacheDigest(key, digest string) {
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()

	r.cache.entries[key] = cacheEntry{
		digest:    digest,
		expiresAt: time.Now().Add(DefaultCacheTTL),
	}
}

func (r *RegistryChecker) cacheError(key, errMsg string) {
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()

	r.cache.entries[key] = cacheEntry{
		err:       errMsg,
		expiresAt: time.Now().Add(ErrorCacheTTL),
	}
}

// CleanupCache removes expired entries from the cache.
func (r *RegistryChecker) CleanupCache() {
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()

	now := time.Now()
	for key, entry := range r.cache.entries {
		if now.After(entry.expiresAt) {
			delete(r.cache.entries, key)
		}
	}
}

// CacheSize returns the current number of cached entries.
func (r *RegistryChecker) CacheSize() int {
	r.cache.mu.RLock()
	defer r.cache.mu.RUnlock()
	return len(r.cache.entries)
}

// isValidDigest checks if a string looks like a valid digest.
var digestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func isValidDigest(s string) bool {
	return digestPattern.MatchString(s)
}
