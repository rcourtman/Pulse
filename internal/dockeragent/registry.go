package dockeragent

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

// RegistryChecker handles container image digest lookups against registries.
type RegistryChecker struct {
	httpClient *http.Client
	cache      *digestCache
	logger     zerolog.Logger
	mu         sync.RWMutex

	// Configuration
	enabled       bool
	checkInterval time.Duration
	lastFullCheck time.Time
}

// digestCache provides thread-safe caching of digest lookups.
type digestCache struct {
	entries map[string]cacheEntry
	mu      sync.RWMutex
}

type cacheEntry struct {
	latestDigest string
	expiresAt    time.Time
	err          string // cached error message
}

const (
	// DefaultCacheTTL is the default time-to-live for cached digests.
	defaultCacheTTL = 6 * time.Hour
	// ErrorCacheTTL is the TTL for caching errors (shorter to allow retry).
	errorCacheTTL = 15 * time.Minute
	// DefaultCheckInterval is how often to check for updates.
	defaultCheckInterval = 6 * time.Hour
)

// ImageUpdateResult contains the result of an image update check.
type ImageUpdateResult struct {
	Image           string    `json:"image"`
	CurrentDigest   string    `json:"currentDigest"`
	LatestDigest    string    `json:"latestDigest"`
	UpdateAvailable bool      `json:"updateAvailable"`
	CheckedAt       time.Time `json:"checkedAt"`
	Error           string    `json:"error,omitempty"`
}

// NewRegistryChecker creates a new registry checker for the Docker agent.
func NewRegistryChecker(logger zerolog.Logger) *RegistryChecker {
	return newRegistryCheckerWithConfig(logger, true)
}

// newRegistryCheckerWithConfig creates a registry checker with the enabled state set.
func newRegistryCheckerWithConfig(logger zerolog.Logger, enabled bool) *RegistryChecker {
	return &RegistryChecker{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
				MaxIdleConns:       10,
				IdleConnTimeout:    90 * time.Second,
				DisableCompression: false,
				DisableKeepAlives:  false,
			},
		},
		cache: &digestCache{
			entries: make(map[string]cacheEntry),
		},
		logger:        logger,
		enabled:       enabled,
		checkInterval: defaultCheckInterval,
	}
}

// SetEnabled enables or disables update checking.
func (r *RegistryChecker) SetEnabled(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled = enabled
}

// Enabled returns whether update checking is enabled.
func (r *RegistryChecker) Enabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.enabled
}

// ShouldCheck returns true if enough time has passed since the last full check.
func (r *RegistryChecker) ShouldCheck() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.enabled {
		return false
	}

	return time.Since(r.lastFullCheck) >= r.checkInterval
}

// MarkChecked updates the last check timestamp.
func (r *RegistryChecker) MarkChecked() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastFullCheck = time.Now()
}

// ForceCheck clears the cache and resets the last check timestamp.
func (r *RegistryChecker) ForceCheck() {
	r.mu.Lock()
	r.lastFullCheck = time.Time{}
	r.mu.Unlock()

	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()
	r.cache.entries = make(map[string]cacheEntry)
}

// CheckImageUpdate checks if a newer version of the image is available.
func (r *RegistryChecker) CheckImageUpdate(ctx context.Context, image, currentDigest, arch, os, variant string) *ImageUpdateResult {
	if !r.Enabled() {
		return nil
	}

	registry, repository, tag := parseImageReference(image)

	// Skip digest-pinned images (image@sha256:...)
	if registry == "" {
		return &ImageUpdateResult{
			Image:           image,
			CurrentDigest:   currentDigest,
			UpdateAvailable: false,
			CheckedAt:       time.Now(),
			Error:           "digest-pinned image",
		}
	}

	// Check cache first.
	cacheKey := fmt.Sprintf("%s/%s:%s|%s/%s/%s", registry, repository, tag, arch, os, variant)
	r.logger.Debug().Str("image", image).Str("cacheKey", cacheKey).Msg("checking image update cache")

	if cached := r.getCached(cacheKey); cached != nil {
		r.logger.Debug().Str("image", image).Msg("cache hit for update check")
		if cached.err != "" {
			return &ImageUpdateResult{
				Image:           image,
				CurrentDigest:   currentDigest,
				UpdateAvailable: false,
				CheckedAt:       time.Now(),
				Error:           cached.err,
			}
		}
		return &ImageUpdateResult{
			Image:           image,
			CurrentDigest:   currentDigest,
			LatestDigest:    cached.latestDigest,
			UpdateAvailable: r.digestsDiffer(currentDigest, cached.latestDigest),
			CheckedAt:       time.Now(),
		}
	}

	// Fetch latest digest from registry
	latestDigest, headDigest, err := r.fetchDigest(ctx, registry, repository, tag, arch, os, variant)
	if err != nil {
		// Cache the error to avoid hammering the registry
		r.cacheError(cacheKey, err.Error())

		r.logger.Debug().
			Str("image", image).
			Str("registry", registry).
			Err(err).
			Msg("Failed to fetch image digest from registry")

		return &ImageUpdateResult{
			Image:           image,
			CurrentDigest:   currentDigest,
			UpdateAvailable: false,
			CheckedAt:       time.Now(),
			Error:           err.Error(),
		}
	}

	// Store both digests in cache (comma separated) to allow matching against either
	cacheValue := latestDigest
	if headDigest != latestDigest && headDigest != "" {
		cacheValue = latestDigest + "," + headDigest
	}

	// Cache the successful result
	r.cacheDigest(cacheKey, cacheValue)

	updateAvailable := r.digestsDiffer(currentDigest, cacheValue)

	r.logger.Debug().
		Str("image", image).
		Str("currentDigest", currentDigest).
		Str("latestDigest", latestDigest).
		Str("headDigest", headDigest).
		Str("arch", arch).
		Str("os", os).
		Str("variant", variant).
		Bool("updateAvailable", updateAvailable).
		Msg("Checked image update")

	return &ImageUpdateResult{
		Image:           image,
		CurrentDigest:   currentDigest,
		LatestDigest:    latestDigest,
		UpdateAvailable: updateAvailable,
		CheckedAt:       time.Now(),
	}
}

// digestsDiffer compares two digests, handling format differences.
func (r *RegistryChecker) digestsDiffer(current, latest string) bool {
	if current == "" || latest == "" {
		return false
	}

	// Normalize digests - lowercase and remove "sha256:" prefix
	normCurrent := strings.ToLower(strings.TrimPrefix(current, "sha256:"))

	// latest may contain multiple comma-separated digests (resolved + head)
	for _, l := range strings.Split(latest, ",") {
		normLatest := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(l), "sha256:"))
		if normCurrent == normLatest {
			return false // Match found
		}
	}

	return true // No match found
}

// fetchDigest retrieves the digest for an image from the registry.
// Returns the resolved platform-specific digest AND the raw HEAD digest (which might be a manifest list).
func (r *RegistryChecker) fetchDigest(ctx context.Context, registry, repository, tag, arch, os, variant string) (string, string, error) {
	// Get auth token if needed
	token, err := r.getAuthToken(ctx, registry, repository)
	if err != nil {
		return "", "", fmt.Errorf("auth: %w", err)
	}

	// Construct the manifest URL
	manifestURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, repository, tag)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, manifestURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
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
		return "", "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", "", fmt.Errorf("authentication required")
	}
	if resp.StatusCode == http.StatusNotFound {
		return "", "", fmt.Errorf("image not found")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", "", fmt.Errorf("rate limited")
	}
	if resp.StatusCode >= 400 {
		return "", "", fmt.Errorf("registry error: %d", resp.StatusCode)
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

	contentType := resp.Header.Get("Content-Type")
	isManifestList := strings.Contains(contentType, "manifest.list") || strings.Contains(contentType, "image.index")

	// If it's a manifest list and we have arch info, we need to resolve it
	if isManifestList && arch != "" && os != "" {
		resolved, err := r.resolveManifestList(ctx, registry, repository, tag, arch, os, variant, token)
		return resolved, digest, err
	}

	if digest == "" {
		return "", "", fmt.Errorf("no digest in response")
	}

	return digest, digest, nil
}

// resolveManifestList fetches the manifest list and finds the matching digest for the architecture.
func (r *RegistryChecker) resolveManifestList(ctx context.Context, registry, repository, tag, arch, os, variant, token string) (string, error) {
	manifestURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, repository, tag)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return "", fmt.Errorf("create list request: %w", err)
	}

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
		return "", fmt.Errorf("list request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch manifest list failed: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read list body: %w", err)
	}

	var list manifestList
	if err := json.Unmarshal(body, &list); err != nil {
		return "", fmt.Errorf("decode manifest list: %w", err)
	}

	// Find the matching manifest
	// We matched arch and os. Variant is tricky as it's not always passed or available clearly.
	// We'll prioritize exact match including variant (if we had it), but for now standard match.
	// Since we strictly want to match what the local image is, and we'll get that from ImageInspect.

	// Simple matching logic for now: exact match on Arch and OS
	for _, m := range list.Manifests {
		if m.Platform.Architecture == arch && m.Platform.OS == os {
			if variant != "" && m.Platform.Variant != "" && variant != m.Platform.Variant {
				continue
			}
			r.logger.Debug().
				Str("image", repository+":"+tag).
				Str("arch", arch).
				Str("variant", variant).
				Str("foundDigest", m.Digest).
				Str("foundVariant", m.Platform.Variant).
				Msg("Resolved manifest list digest")
			return m.Digest, nil
		}
	}

	return "", fmt.Errorf("no matching manifest found for %s/%s in list", os, arch)
}

type manifestList struct {
	Manifests []manifestDescriptor `json:"manifests"`
}

type manifestDescriptor struct {
	Digest   string           `json:"digest"`
	Platform manifestPlatform `json:"platform"`
}

type manifestPlatform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
}

// getAuthToken retrieves an auth token for the registry.
func (r *RegistryChecker) getAuthToken(ctx context.Context, registry, repository string) (string, error) {
	// Docker Hub requires auth token even for public images
	if registry == "registry-1.docker.io" {
		tokenURL := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", repository)
		return r.fetchAuthToken(ctx, tokenURL)
	}

	// GitHub Container Registry (ghcr.io) requires auth token for public images
	if registry == "ghcr.io" {
		tokenURL := fmt.Sprintf("https://ghcr.io/token?service=ghcr.io&scope=repository:%s:pull", repository)
		return r.fetchAuthToken(ctx, tokenURL)
	}

	// For other registries, try anonymous access first
	return "", nil
}

// fetchAuthToken fetches an auth token from a token endpoint.
func (r *RegistryChecker) fetchAuthToken(ctx context.Context, tokenURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return "", err
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
		latestDigest: digest,
		expiresAt:    time.Now().Add(defaultCacheTTL),
	}
}

func (r *RegistryChecker) cacheError(key, errMsg string) {
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()

	r.cache.entries[key] = cacheEntry{
		err:       errMsg,
		expiresAt: time.Now().Add(errorCacheTTL),
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

// parseImageReference parses an image reference into registry, repository, and tag.
func parseImageReference(image string) (registry, repository, tag string) {
	// Default values
	registry = "registry-1.docker.io"
	tag = "latest"

	// Check if this is a digest or digest-pinned image (image@sha256: or sha256:...)
	if strings.Contains(image, "@sha256:") || strings.HasPrefix(image, "sha256:") || isValidDigest(image) {
		return "", "", ""
	}

	// Also check for 64-character hex strings (often used as image IDs)
	if len(image) == 64 {
		isHex := true
		for _, c := range image {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				isHex = false
				break
			}
		}
		if isHex {
			return "", "", ""
		}
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

// isValidDigest checks if a string looks like a valid digest.
var digestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func isValidDigest(s string) bool {
	return digestPattern.MatchString(s)
}
