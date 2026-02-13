package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

// oidcStateTTL defines how long we accept OIDC login attempts before expiring the state entry.
const oidcStateTTL = 10 * time.Minute

// OIDCService caches provider metadata and manages transient state for authorization flows.
type OIDCService struct {
	snapshot   oidcSnapshot
	provider   *oidc.Provider
	oauth2Cfg  *oauth2.Config
	verifier   *oidc.IDTokenVerifier
	stateStore *oidcStateStore
	httpClient *http.Client
}

// OIDCServiceManager manages multiple OIDC services for different SSO providers.
type OIDCServiceManager struct {
	mu       sync.RWMutex
	services map[string]*OIDCService
}

// NewOIDCServiceManager creates a new OIDC service manager.
func NewOIDCServiceManager() *OIDCServiceManager {
	return &OIDCServiceManager{
		services: make(map[string]*OIDCService),
	}
}

// GetService returns the cached OIDC service for a provider, or nil.
func (m *OIDCServiceManager) GetService(providerID string) *OIDCService {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.services[providerID]
}

// InitializeProvider creates or replaces the OIDC service for a provider.
func (m *OIDCServiceManager) InitializeProvider(ctx context.Context, providerID string, provider *config.SSOProvider, redirectURL string) error {
	if m == nil {
		return errors.New("oidc service manager not initialized")
	}
	cfg := ssoProviderToOIDCConfig(provider, redirectURL)
	service, err := NewOIDCService(ctx, cfg)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// Stop old state store cleanup goroutine if replacing
	if old, ok := m.services[providerID]; ok {
		old.stateStore.Stop()
	}
	m.services[providerID] = service

	log.Info().
		Str("provider_id", providerID).
		Str("issuer", cfg.IssuerURL).
		Msg("Initialized SSO OIDC provider")

	return nil
}

// RemoveService removes and cleans up the OIDC service for a provider.
func (m *OIDCServiceManager) RemoveService(providerID string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if svc, ok := m.services[providerID]; ok {
		svc.stateStore.Stop()
		delete(m.services, providerID)
	}
}

type oidcSnapshot struct {
	issuer       string
	clientID     string
	clientSecret string
	redirectURL  string
	scopes       []string
	caBundle     string
	caBundleHash string
}

// NewOIDCService fetches provider metadata and prepares helper structures.
func NewOIDCService(ctx context.Context, cfg *config.OIDCConfig) (*OIDCService, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, errors.New("oidc is not enabled")
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	log.Debug().
		Str("issuer", cfg.IssuerURL).
		Str("redirect_url", cfg.RedirectURL).
		Strs("scopes", cfg.Scopes).
		Str("ca_bundle", cfg.CABundle).
		Msg("Initializing OIDC provider")

	httpClient, caHash, err := newOIDCHTTPClient(cfg.CABundle)
	if err != nil {
		return nil, fmt.Errorf("failed to build OIDC HTTP client: %w", err)
	}

	ctx = oidc.ClientContext(ctx, httpClient)

	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to discover OIDC provider: %w", err)
	}

	log.Debug().
		Str("issuer", cfg.IssuerURL).
		Str("auth_endpoint", provider.Endpoint().AuthURL).
		Str("token_endpoint", provider.Endpoint().TokenURL).
		Msg("OIDC provider discovery successful")

	oauth2Cfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       append([]string{}, cfg.Scopes...),
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})

	snapshot := oidcSnapshot{
		issuer:       cfg.IssuerURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		redirectURL:  cfg.RedirectURL,
		scopes:       append([]string{}, cfg.Scopes...),
		caBundle:     cfg.CABundle,
		caBundleHash: caHash,
	}

	service := &OIDCService{
		snapshot:   snapshot,
		provider:   provider,
		oauth2Cfg:  oauth2Cfg,
		verifier:   verifier,
		stateStore: newOIDCStateStore(),
		httpClient: httpClient,
	}

	return service, nil
}

// Matches checks whether the cached configuration matches the provided settings.
func (s *OIDCService) Matches(cfg *config.OIDCConfig) bool {
	if s == nil || cfg == nil {
		return false
	}

	if s.snapshot.issuer != cfg.IssuerURL {
		return false
	}
	if s.snapshot.clientID != cfg.ClientID {
		return false
	}
	if s.snapshot.clientSecret != cfg.ClientSecret {
		return false
	}
	if s.snapshot.redirectURL != cfg.RedirectURL {
		return false
	}
	if s.snapshot.caBundle != cfg.CABundle {
		return false
	}
	if cfg.CABundle != "" {
		currentHash, err := hashCABundle(cfg.CABundle)
		if err != nil {
			return false
		}
		if s.snapshot.caBundleHash != currentHash {
			return false
		}
	}
	if len(s.snapshot.scopes) != len(cfg.Scopes) {
		return false
	}
	for i, scope := range s.snapshot.scopes {
		if scope != cfg.Scopes[i] {
			return false
		}
	}

	return true
}

func (s *OIDCService) newStateEntry(providerID, returnTo string) (string, *oidcStateEntry, error) {
	state, err := generateRandomURLString(32)
	if err != nil {
		return "", nil, err
	}
	nonce, err := generateRandomURLString(32)
	if err != nil {
		return "", nil, err
	}

	codeVerifier, codeChallenge, err := generatePKCEPair()
	if err != nil {
		return "", nil, err
	}

	entry := &oidcStateEntry{
		ProviderID:    providerID,
		Nonce:         nonce,
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
		ReturnTo:      returnTo,
		ExpiresAt:     time.Now().Add(oidcStateTTL),
	}

	s.stateStore.Put(state, entry)
	return state, entry, nil
}

func (s *OIDCService) consumeState(state string) (*oidcStateEntry, bool) {
	return s.stateStore.Consume(state)
}

func (s *OIDCService) authCodeURL(state string, entry *oidcStateEntry) string {
	opts := []oauth2.AuthCodeOption{oidc.Nonce(entry.Nonce)}
	if entry.CodeChallenge != "" {
		opts = append(opts,
			oauth2.SetAuthURLParam("code_challenge_method", "S256"),
			oauth2.SetAuthURLParam("code_challenge", entry.CodeChallenge),
		)
	}
	return s.oauth2Cfg.AuthCodeURL(state, opts...)
}

func (s *OIDCService) exchangeCode(ctx context.Context, code string, entry *oidcStateEntry) (*oauth2.Token, error) {
	ctx = s.contextWithHTTPClient(ctx)
	opts := []oauth2.AuthCodeOption{}
	if entry.CodeVerifier != "" {
		opts = append(opts, oauth2.SetAuthURLParam("code_verifier", entry.CodeVerifier))
	}
	return s.oauth2Cfg.Exchange(ctx, code, opts...)
}

func (s *OIDCService) contextWithHTTPClient(ctx context.Context) context.Context {
	if s.httpClient == nil {
		return ctx
	}
	return oidc.ClientContext(ctx, s.httpClient)
}

// Stop releases background resources owned by the service.
func (s *OIDCService) Stop() {
	if s == nil || s.stateStore == nil {
		return
	}
	s.stateStore.Stop()
}

// OIDCRefreshResult contains the result of a token refresh operation
type OIDCRefreshResult struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
}

// RefreshToken uses the refresh token to obtain new access and refresh tokens from the IdP
func (s *OIDCService) RefreshToken(ctx context.Context, refreshToken string) (*OIDCRefreshResult, error) {
	if refreshToken == "" {
		return nil, errors.New("no refresh token provided")
	}

	ctx = s.contextWithHTTPClient(ctx)

	// Create a token source from the refresh token
	token := &oauth2.Token{
		RefreshToken: refreshToken,
		// Set expiry in the past to force refresh
		Expiry: time.Now().Add(-time.Hour),
	}

	tokenSource := s.oauth2Cfg.TokenSource(ctx, token)

	// This will trigger a refresh since the token is expired
	newToken, err := tokenSource.Token()
	if err != nil {
		log.Warn().Err(err).Msg("OIDC token refresh failed")
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	result := &OIDCRefreshResult{
		AccessToken: newToken.AccessToken,
		Expiry:      newToken.Expiry,
	}

	// The new refresh token might be the same or different depending on the IdP
	if newToken.RefreshToken != "" {
		result.RefreshToken = newToken.RefreshToken
	} else {
		// Keep the old refresh token if a new one wasn't issued
		result.RefreshToken = refreshToken
	}

	log.Debug().
		Time("new_expiry", result.Expiry).
		Bool("new_refresh_token", newToken.RefreshToken != "").
		Msg("OIDC token refresh successful")

	return result, nil
}

func newOIDCHTTPClient(caBundle string) (*http.Client, string, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	var clone *http.Transport
	if ok && transport != nil {
		clone = transport.Clone()
	} else {
		clone = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		}
	}
	if strings.TrimSpace(caBundle) == "" {
		return &http.Client{Transport: clone}, "", nil
	}

	caData, err := os.ReadFile(caBundle)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read OIDC CA bundle: %w", err)
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}

	if ok := pool.AppendCertsFromPEM(caData); !ok {
		return nil, "", fmt.Errorf("OIDC CA bundle does not contain any certificates")
	}

	if clone.TLSClientConfig == nil {
		clone.TLSClientConfig = &tls.Config{}
	}
	clone.TLSClientConfig.MinVersion = tls.VersionTLS12
	clone.TLSClientConfig.RootCAs = pool

	sum := sha256.Sum256(caData)
	caHash := fmt.Sprintf("%x", sum[:])

	return &http.Client{Transport: clone}, caHash, nil
}

func hashCABundle(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:]), nil
}

// oidcStateStore keeps short-lived authorization state tokens.
type oidcStateStore struct {
	mu          sync.RWMutex
	entries     map[string]*oidcStateEntry
	stopCleanup chan struct{}
	stopOnce    sync.Once
}

type oidcStateEntry struct {
	ProviderID    string // SSO provider ID (empty for legacy flow)
	Nonce         string
	CodeVerifier  string
	CodeChallenge string
	ReturnTo      string
	ExpiresAt     time.Time
}

func newOIDCStateStore() *oidcStateStore {
	s := &oidcStateStore{
		entries:     make(map[string]*oidcStateEntry),
		stopCleanup: make(chan struct{}),
	}

	// Start cleanup routine to prevent memory leak from abandoned OIDC flows
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.cleanup()
			case <-s.stopCleanup:
				return
			}
		}
	}()

	return s
}

// cleanup removes expired state entries
func (s *oidcStateStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for state, entry := range s.entries {
		if now.After(entry.ExpiresAt) {
			delete(s.entries, state)
		}
	}
}

// Stop stops the cleanup routine
func (s *oidcStateStore) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCleanup)
	})
}

func (s *oidcStateStore) Put(state string, entry *oidcStateEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[state] = entry
}

func (s *oidcStateStore) Consume(state string) (*oidcStateEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.entries[state]
	if !exists {
		return nil, false
	}
	delete(s.entries, state)

	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	return entry, true
}

func generateRandomURLString(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func generatePKCEPair() (verifier string, challenge string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}

	verifier = base64.RawURLEncoding.EncodeToString(buf)
	hash := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(hash[:])
	return verifier, challenge, nil
}
