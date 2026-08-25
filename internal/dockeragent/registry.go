package dockeragent

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	// credentials optionally resolves host Docker credentials so private
	// registries can be checked; nil keeps every lookup anonymous.
	credentials registryCredentialSource
	logger      zerolog.Logger
	mu          sync.RWMutex

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
	latestDigest      string
	comparisonDigests string
	expiresAt         time.Time
	err               string // cached error message
}

const (
	// DefaultCacheTTL is the default time-to-live for cached digests.
	defaultCacheTTL = 6 * time.Hour
	// ErrorCacheTTL is the TTL for caching errors (shorter to allow retry).
	errorCacheTTL = 15 * time.Minute
	// RateLimitCacheTTL is the TTL for rate-limited lookups. Retrying these on
	// the short error TTL keeps hammering a registry whose allowance is
	// already exhausted (every refused HEAD still counts against it), which
	// can hold a strict registry's limit tripped indefinitely. Back off for
	// an hour instead so the allowance can actually recover.
	rateLimitCacheTTL = 1 * time.Hour
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

func isPulseManagedImageReference(image string) bool {
	normalized := strings.ToLower(strings.TrimSpace(image))
	for _, repository := range []string{
		"rcourtman/pulse",
		"docker.io/rcourtman/pulse",
		"license.pulserelay.pro/pulse-pro",
		"registry.pulserelay.pro/pulse/pulse-pro",
	} {
		if normalized == repository ||
			strings.HasPrefix(normalized, repository+":") ||
			strings.HasPrefix(normalized, repository+"@") {
			return true
		}
	}
	return false
}

// NewRegistryChecker creates a new registry checker for the Docker / Podman module.
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

// proBrokerRegistry is the entitled Pulse Pro registry. Digest checks against
// it require a license credential the agent does not hold, so an anonymous
// HEAD can never succeed and would pin a permanent "authentication required"
// badge on the Pulse Pro container. That container updates through the
// broker's digest-pinned commands instead, so the generic checker stays out.
const proBrokerRegistry = "license.pulserelay.pro"

// CheckImageUpdate checks if a newer version of the image is available.
func (r *RegistryChecker) CheckImageUpdate(ctx context.Context, image, currentDigest, arch, goos, variant string) *ImageUpdateResult {
	if !r.Enabled() {
		return nil
	}
	if isPulseManagedImageReference(image) {
		return nil
	}

	registry, repository, tag := parseImageReference(image)

	if registry == proBrokerRegistry {
		return nil
	}

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

	// Check cache first
	cacheKey := fmt.Sprintf("%s/%s:%s|%s/%s/%s", registry, repository, tag, arch, goos, variant)
	r.logger.Debug().Str("image", image).Str("cacheKey", cacheKey).Msg("Checking update (internal)")

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
			UpdateAvailable: r.digestsDiffer(currentDigest, cached.comparisonDigests),
			CheckedAt:       time.Now(),
		}
	}

	// Fetch latest digest from registry
	latestDigest, headDigest, err := r.fetchDigest(ctx, registry, repository, tag, arch, goos, variant)
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

	// Compare against both digest layers. Docker's RepoDigest identifies the
	// tag/index while the resolved digest identifies this host's platform
	// manifest; either one proves that the local image is current.
	comparisonDigests := latestDigest
	if headDigest != latestDigest && headDigest != "" {
		comparisonDigests = latestDigest + "," + headDigest
	}
	publicLatestDigest := latestDigest
	if headDigest != "" {
		publicLatestDigest = headDigest
	}

	// Cache the successful result
	r.cacheDigestResult(cacheKey, publicLatestDigest, comparisonDigests)

	updateAvailable := r.digestsDiffer(currentDigest, comparisonDigests)

	r.logger.Debug().
		Str("image", image).
		Str("currentDigest", currentDigest).
		Str("latestDigest", latestDigest).
		Str("headDigest", headDigest).
		Str("arch", arch).
		Str("os", goos).
		Str("variant", variant).
		Bool("updateAvailable", updateAvailable).
		Msg("Checked image update")

	return &ImageUpdateResult{
		Image:           image,
		CurrentDigest:   currentDigest,
		LatestDigest:    publicLatestDigest,
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
func (r *RegistryChecker) fetchDigest(ctx context.Context, registry, repository, tag, arch, goos, variant string) (string, string, error) {
	creds := r.lookupCredentials(ctx, registry)

	// Get auth token if needed
	token, tokenUsedCreds, err := r.getScopedAuthToken(ctx, registry, repository, creds)
	if err != nil {
		return "", "", fmt.Errorf("auth: %w", err)
	}
	authorization := ""
	if token != "" {
		authorization = "Bearer " + token
	}

	// Construct the manifest URL
	manifestURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, repository, tag)

	resp, err := r.headManifest(ctx, manifestURL, authorization)
	if err != nil {
		return "", "", err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		// Registries without a hardcoded token endpoint (lscr.io, quay.io, ...)
		// advertise it in the WWW-Authenticate challenge. Negotiate a pull
		// token (with host credentials when the store has them) or answer a
		// Basic challenge directly, and retry once.
		challenge := resp.Header.Get("Www-Authenticate")
		if retryAuthorization, ok := r.retryAuthorization(ctx, challenge, repository, token, tokenUsedCreds, creds); ok {
			resp.Body.Close()
			authorization = retryAuthorization
			resp, err = r.headManifest(ctx, manifestURL, authorization)
			if err != nil {
				return "", "", err
			}
		}
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

	if digest == "" {
		if closeErr := resp.Body.Close(); closeErr != nil {
			r.logger.Debug().Err(closeErr).Msg("Failed to close registry HEAD response")
		}
		manifestDigest, manifestContentType, manifestBody, err := r.fetchManifest(ctx, manifestURL, authorization)
		if err != nil {
			return "", "", err
		}
		if len(manifestBody) == 0 {
			return "", "", fmt.Errorf("no digest in response")
		}

		digest = manifestDigest
		if digest == "" {
			digest = fmt.Sprintf("sha256:%x", sha256.Sum256(manifestBody))
		}

		isManifestList = isManifestList ||
			strings.Contains(manifestContentType, "manifest.list") ||
			strings.Contains(manifestContentType, "image.index")
		if isManifestList && arch != "" && goos != "" {
			resolved, resolveErr := resolveManifestListBody(manifestBody, arch, goos, variant)
			return resolved, digest, resolveErr
		}
	}

	// If it's a manifest list and we have arch info, resolve the platform
	// manifest only after establishing the parent digest. Registries that
	// identify an index on HEAD but omit its digest need the GET fallback above
	// to preserve both values.
	if isManifestList && arch != "" && goos != "" {
		resolved, err := r.resolveManifestList(ctx, registry, repository, tag, arch, goos, variant, authorization)
		return resolved, digest, err
	}

	return digest, digest, nil
}

// fetchManifest retrieves a manifest body when a registry accepts HEAD but
// omits the digest headers. The digest of the exact response body is a valid
// fallback under the registry content-addressing contract.
func (r *RegistryChecker) fetchManifest(ctx context.Context, manifestURL, authorization string) (string, string, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return "", "", nil, fmt.Errorf("create manifest request: %w", err)
	}
	r.setManifestRequestHeaders(req, authorization)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", "", nil, fmt.Errorf("manifest request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", "", nil, fmt.Errorf("authentication required")
	}
	if resp.StatusCode == http.StatusNotFound {
		return "", "", nil, fmt.Errorf("image not found")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", "", nil, fmt.Errorf("rate limited")
	}
	if resp.StatusCode >= 400 {
		return "", "", nil, fmt.Errorf("registry error: %d", resp.StatusCode)
	}

	body, err := readBodyWithLimit(resp.Body, maxRegistryManifestBodyBytes)
	if err != nil {
		return "", "", nil, fmt.Errorf("read manifest body: %w", err)
	}
	digest := strings.Trim(resp.Header.Get("Docker-Content-Digest"), `"`)
	if digest == "" {
		digest = strings.Trim(resp.Header.Get("Etag"), `"`)
	}
	return digest, resp.Header.Get("Content-Type"), body, nil
}

// headManifest issues a manifest HEAD request with the multi-arch Accept set.
func (r *RegistryChecker) headManifest(ctx context.Context, manifestURL, authorization string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, manifestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	r.setManifestRequestHeaders(req, authorization)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	return resp, nil
}

// setManifestRequestHeaders applies the multi-arch Accept set and an optional
// full Authorization header value ("Bearer ..." or "Basic ...").
func (r *RegistryChecker) setManifestRequestHeaders(req *http.Request, authorization string) {
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.oci.image.index.v1+json",
	}, ", "))

	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
}

// retryAuthorization derives the Authorization value for the single retry
// after an unauthorized manifest HEAD. Bearer challenges are negotiated at
// the advertised token endpoint (anonymously, as before, or with host
// credentials when the store has them); Basic challenges are answered
// directly from host credentials. It returns false when no retry could do
// better than the attempt that already failed.
func (r *RegistryChecker) retryAuthorization(ctx context.Context, challenge, repository, token string, tokenUsedCreds bool, creds *registryCredential) (string, bool) {
	scheme := strings.ToLower(strings.TrimSpace(challenge))
	switch {
	case strings.HasPrefix(scheme, "basic"):
		if creds == nil || creds.IdentityToken {
			return "", false
		}
		return creds.authorizationHeader(), true
	case strings.HasPrefix(scheme, "bearer"):
		if token != "" && (creds == nil || tokenUsedCreds) {
			// The failed attempt already carried our best token; negotiating
			// the same one again cannot succeed.
			return "", false
		}
		negotiated, err := r.tokenFromChallenge(ctx, challenge, repository, creds)
		if err != nil {
			return "", false
		}
		return "Bearer " + negotiated, true
	default:
		return "", false
	}
}

// tokenFromChallenge negotiates a pull token from the token endpoint named in
// a registry's WWW-Authenticate Bearer challenge (Docker registry v2 token
// auth), anonymously or with host credentials.
func (r *RegistryChecker) tokenFromChallenge(ctx context.Context, challenge, repository string, creds *registryCredential) (string, error) {
	params := parseBearerChallenge(challenge)
	realm := params["realm"]
	if realm == "" {
		return "", fmt.Errorf("no bearer challenge")
	}
	scope := params["scope"]
	if scope == "" {
		scope = fmt.Sprintf("repository:%s:pull", repository)
	}
	return r.authorizeToken(ctx, realm, params["service"], scope, creds)
}

// parseBearerChallenge extracts the key="value" parameters from a
// WWW-Authenticate Bearer challenge header.
func parseBearerChallenge(header string) map[string]string {
	params := map[string]string{}
	rest, ok := strings.CutPrefix(strings.TrimSpace(header), "Bearer ")
	if !ok {
		return params
	}
	for _, part := range strings.Split(rest, ",") {
		key, value, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			continue
		}
		params[strings.ToLower(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	return params
}

// resolveManifestList fetches the manifest list and finds the matching digest for the architecture.
func (r *RegistryChecker) resolveManifestList(ctx context.Context, registry, repository, tag, arch, goos, variant, authorization string) (string, error) {
	manifestURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, repository, tag)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return "", fmt.Errorf("create list request: %w", err)
	}

	r.setManifestRequestHeaders(req, authorization)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("list request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch manifest list failed: %d", resp.StatusCode)
	}

	body, err := readBodyWithLimit(resp.Body, maxRegistryManifestBodyBytes)
	if err != nil {
		return "", fmt.Errorf("read list body: %w", err)
	}

	return resolveManifestListBody(body, arch, goos, variant)
}

func resolveManifestListBody(body []byte, arch, goos, variant string) (string, error) {
	var list manifestList
	if err := json.Unmarshal(body, &list); err != nil {
		return "", fmt.Errorf("decode manifest list: %w", err)
	}

	for _, m := range list.Manifests {
		if m.Platform.Architecture == arch && m.Platform.OS == goos {
			if variant != "" && m.Platform.Variant != "" && variant != m.Platform.Variant {
				continue
			}
			return m.Digest, nil
		}
	}

	return "", fmt.Errorf("no matching manifest found for %s/%s in list", goos, arch)
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
	token, _, err := r.getScopedAuthToken(ctx, registry, repository, r.lookupCredentials(ctx, registry))
	return token, err
}

// getScopedAuthToken negotiates a pull token for registries with hardcoded
// token endpoints. It also reports whether the returned token actually
// carried the supplied credentials, so the 401 retry can tell a fresh
// credentialed attempt apart from replaying one that already failed.
func (r *RegistryChecker) getScopedAuthToken(ctx context.Context, registry, repository string, creds *registryCredential) (string, bool, error) {
	var realm, service string
	switch registry {
	// Docker Hub and ghcr.io require a token even for public images.
	case "registry-1.docker.io":
		realm, service = "https://auth.docker.io/token", "registry.docker.io"
	case "ghcr.io":
		realm, service = "https://ghcr.io/token", "ghcr.io"
	default:
		// For other registries, try anonymous access first.
		return "", false, nil
	}

	if creds != nil {
		token, err := r.authorizeToken(ctx, realm, service, fmt.Sprintf("repository:%s:pull", repository), creds)
		if err == nil {
			return token, true, nil
		}
		// A stale login must not break checks that used to work anonymously.
		r.logger.Debug().Str("registry", registry).Err(err).Msg("Credentialed token negotiation failed; retrying anonymously")
	}

	tokenURL := fmt.Sprintf("%s?service=%s&scope=repository:%s:pull", realm, service, repository)
	token, err := r.fetchAuthToken(ctx, tokenURL)
	return token, false, err
}

// authorizeToken requests a pull token from a token endpoint, attaching Basic
// credentials or exchanging an identity token when host credentials exist.
func (r *RegistryChecker) authorizeToken(ctx context.Context, realm, service, scope string, creds *registryCredential) (string, error) {
	realmURL, err := url.Parse(realm)
	if err != nil || realmURL.Scheme != "https" {
		return "", fmt.Errorf("invalid token realm")
	}

	if creds != nil && creds.IdentityToken {
		return r.exchangeIdentityToken(ctx, realmURL.String(), service, scope, creds)
	}

	query := realmURL.Query()
	if service != "" {
		query.Set("service", service)
	}
	query.Set("scope", scope)
	realmURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, realmURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	if creds != nil {
		req.SetBasicAuth(creds.Username, creds.Secret)
	}
	return r.doTokenRequest(req)
}

// exchangeIdentityToken swaps an OAuth identity token (docker-credential
// helper logins that return Username "<token>", for example Azure ACR) for a
// pull token via the refresh-token grant of the registry token endpoint.
func (r *RegistryChecker) exchangeIdentityToken(ctx context.Context, realm, service, scope string, creds *registryCredential) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", creds.Secret)
	form.Set("client_id", "pulse-agent")
	if service != "" {
		form.Set("service", service)
	}
	form.Set("scope", scope)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, realm, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r.doTokenRequest(req)
}

// lookupCredentials resolves host Docker credentials for a registry. It
// returns nil when no source is wired, the registry has no stored login, or
// resolution fails; the check then proceeds anonymously as before.
func (r *RegistryChecker) lookupCredentials(ctx context.Context, registry string) *registryCredential {
	if r.credentials == nil {
		return nil
	}
	return r.credentials.Lookup(ctx, registry)
}

// fetchAuthToken fetches an auth token anonymously from a token endpoint.
func (r *RegistryChecker) fetchAuthToken(ctx context.Context, tokenURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	return r.doTokenRequest(req)
}

// doTokenRequest executes a token-endpoint request and decodes the token.
func (r *RegistryChecker) doTokenRequest(req *http.Request) (string, error) {
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send token request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			r.logger.Warn().Err(closeErr).Msg("Failed to close token response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed: %d", resp.StatusCode)
	}

	body, err := readBodyWithLimit(resp.Body, maxRegistryTokenBodyBytes)
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}

	var tokenResp struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if tokenResp.Token != "" {
		return tokenResp.Token, nil
	}
	return tokenResp.AccessToken, nil
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
	r.cacheDigestResult(key, digest, digest)
}

func (r *RegistryChecker) cacheDigestResult(key, latestDigest, comparisonDigests string) {
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()

	r.cache.entries[key] = cacheEntry{
		latestDigest:      latestDigest,
		comparisonDigests: comparisonDigests,
		expiresAt:         time.Now().Add(defaultCacheTTL),
	}
}

func (r *RegistryChecker) cacheError(key, errMsg string) {
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()

	ttl := errorCacheTTL
	if strings.Contains(strings.ToLower(errMsg), "rate limit") {
		ttl = rateLimitCacheTTL
	}

	r.cache.entries[key] = cacheEntry{
		err:       errMsg,
		expiresAt: time.Now().Add(ttl),
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
