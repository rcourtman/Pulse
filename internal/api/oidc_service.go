package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
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
}

type oidcSnapshot struct {
	issuer       string
	clientID     string
	clientSecret string
	redirectURL  string
	scopes       []string
}

// NewOIDCService fetches provider metadata and prepares helper structures.
func NewOIDCService(ctx context.Context, cfg *config.OIDCConfig) (*OIDCService, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, errors.New("oidc is not enabled")
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to discover OIDC provider: %w", err)
	}

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
	}

	service := &OIDCService{
		snapshot:   snapshot,
		provider:   provider,
		oauth2Cfg:  oauth2Cfg,
		verifier:   verifier,
		stateStore: newOIDCStateStore(),
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

func (s *OIDCService) newStateEntry(returnTo string) (string, *oidcStateEntry, error) {
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
	opts := []oauth2.AuthCodeOption{}
	if entry.CodeVerifier != "" {
		opts = append(opts, oauth2.SetAuthURLParam("code_verifier", entry.CodeVerifier))
	}
	return s.oauth2Cfg.Exchange(ctx, code, opts...)
}

// oidcStateStore keeps short-lived authorization state tokens.
type oidcStateStore struct {
	mu      sync.RWMutex
	entries map[string]*oidcStateEntry
}

type oidcStateEntry struct {
	Nonce         string
	CodeVerifier  string
	CodeChallenge string
	ReturnTo      string
	ExpiresAt     time.Time
}

func newOIDCStateStore() *oidcStateStore {
	return &oidcStateStore{entries: make(map[string]*oidcStateEntry)}
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
