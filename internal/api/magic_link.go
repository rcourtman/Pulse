package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/rs/zerolog/log"
)

var (
	ErrMagicLinkInvalidToken = errors.New("magic link token is invalid")
	ErrMagicLinkExpired      = errors.New("magic link token is expired")
	ErrMagicLinkUsed         = errors.New("magic link token already used")
)

const (
	magicLinkDefaultTTL           = 15 * time.Minute
	magicLinkStoreCleanupInterval = 5 * time.Minute
	magicLinkRateLimitCount       = 3
	magicLinkRateLimitWindow      = 1 * time.Hour
)

type MagicLinkEmailer interface {
	SendMagicLink(to, magicLinkURL string) error
}

// LogEmailer logs the magic link URL instead of sending email.
// This is intentionally used for dev/staging until a real email subsystem lands.
type LogEmailer struct{}

func (e LogEmailer) SendMagicLink(to, magicLinkURL string) error {
	// SECURITY: Do not log full magic link URLs (token in query string). Even opaque tokens are auth
	// credentials and can be replayed if leaked via logs.
	log.Info().
		Str("to", to).
		Str("magic_link_url_redacted", redactMagicLinkURL(magicLinkURL)).
		Msg("Magic link email (log-only)")
	return nil
}

type MagicLinkToken struct {
	Email     string
	OrgID     string
	ExpiresAt time.Time
	Used      bool
	Token     string // Opaque random token ID (not self-describing)
}

type MagicLinkStore interface {
	Put(tokenHash []byte, token *MagicLinkToken) error
	Consume(tokenHash []byte, now time.Time) (*MagicLinkToken, error)
	DeleteExpired(now time.Time)
	Stop()
}

type InMemoryMagicLinkStore struct {
	mu     sync.RWMutex
	tokens map[string]*MagicLinkToken // keyed by hex(tokenHash)

	stopCleanup chan struct{}
}

func NewInMemoryMagicLinkStore() *InMemoryMagicLinkStore {
	s := &InMemoryMagicLinkStore{
		tokens:      make(map[string]*MagicLinkToken),
		stopCleanup: make(chan struct{}),
	}

	// Tokens are short-lived; periodic cleanup keeps memory bounded.
	go func() {
		ticker := time.NewTicker(magicLinkStoreCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.DeleteExpired(time.Now().UTC())
			case <-s.stopCleanup:
				return
			}
		}
	}()

	return s
}

func (s *InMemoryMagicLinkStore) Stop() {
	select {
	case <-s.stopCleanup:
		// already stopped
	default:
		close(s.stopCleanup)
	}
}

func (s *InMemoryMagicLinkStore) Put(tokenHash []byte, token *MagicLinkToken) error {
	if len(tokenHash) == 0 {
		return fmt.Errorf("tokenHash is required")
	}
	if token == nil || token.Email == "" || token.OrgID == "" || token.ExpiresAt.IsZero() {
		return fmt.Errorf("token record is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := hex.EncodeToString(tokenHash)
	clone := *token
	clone.Used = false
	clone.Token = ""
	s.tokens[key] = &clone
	return nil
}

func (s *InMemoryMagicLinkStore) Consume(tokenHash []byte, now time.Time) (*MagicLinkToken, error) {
	if len(tokenHash) == 0 {
		return nil, ErrMagicLinkInvalidToken
	}
	key := hex.EncodeToString(tokenHash)
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tokens[key]
	if !ok || t == nil {
		return nil, ErrMagicLinkInvalidToken
	}
	if now.After(t.ExpiresAt) {
		return nil, ErrMagicLinkExpired
	}
	if t.Used {
		return nil, ErrMagicLinkUsed
	}
	t.Used = true
	clone := *t
	return &clone, nil
}

func (s *InMemoryMagicLinkStore) DeleteExpired(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k, tok := range s.tokens {
		if tok == nil {
			delete(s.tokens, k)
			continue
		}
		if now.After(tok.ExpiresAt) {
			delete(s.tokens, k)
		}
	}
}

type MagicLinkService struct {
	hmacKey []byte // used to derive a stable lookup key for opaque tokens (HMAC-SHA256(token))
	store   MagicLinkStore
	emailer MagicLinkEmailer

	ttl time.Duration
	now func() time.Time

	// Per-email request limiting: max 3 magic link sends per email per hour.
	requestLimiter *RateLimiter
}

func NewMagicLinkServiceForDataPath(dataPath string, emailer MagicLinkEmailer) (*MagicLinkService, error) {
	cm, err := crypto.NewCryptoManagerAt(dataPath)
	if err != nil {
		return nil, fmt.Errorf("init crypto manager: %w", err)
	}
	key, err := cm.DeriveKey("magic-link-hmac-v1", 32)
	if err != nil {
		return nil, fmt.Errorf("derive hmac key: %w", err)
	}
	store, err := NewSQLiteMagicLinkStore(dataPath)
	if err != nil {
		return nil, fmt.Errorf("init magic link store: %w", err)
	}
	return NewMagicLinkServiceWithKey(key, store, emailer, nil), nil
}

func NewMagicLinkServiceWithKey(hmacKey []byte, store MagicLinkStore, emailer MagicLinkEmailer, limiter *RateLimiter) *MagicLinkService {
	if store == nil {
		store = NewInMemoryMagicLinkStore()
	}
	if emailer == nil {
		emailer = LogEmailer{}
	}
	if limiter == nil {
		limiter = NewRateLimiter(magicLinkRateLimitCount, magicLinkRateLimitWindow)
	}
	return &MagicLinkService{
		hmacKey:        append([]byte(nil), hmacKey...),
		store:          store,
		emailer:        emailer,
		ttl:            magicLinkDefaultTTL,
		now:            time.Now,
		requestLimiter: limiter,
	}
}

func (s *MagicLinkService) Stop() {
	if s == nil {
		return
	}
	if s.store != nil {
		s.store.Stop()
	}
	if s.requestLimiter != nil {
		s.requestLimiter.Stop()
	}
}

func (s *MagicLinkService) AllowRequest(email string) bool {
	if s == nil || s.requestLimiter == nil {
		return true
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return false
	}
	return s.requestLimiter.Allow(email)
}

func (s *MagicLinkService) GenerateToken(email, orgID string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("magic link service not configured")
	}
	if len(s.hmacKey) == 0 {
		return "", fmt.Errorf("magic link hmac key not configured")
	}

	email = strings.ToLower(strings.TrimSpace(email))
	orgID = strings.TrimSpace(orgID)
	if email == "" || orgID == "" {
		return "", fmt.Errorf("email and orgID are required")
	}

	expiresAt := s.now().UTC().Add(s.ttl)
	// Store expiry at second precision for stable comparisons across stores.
	expiresAt = time.Unix(expiresAt.Unix(), 0).UTC()

	token, err := randomOpaqueTokenID()
	if err != nil {
		return "", err
	}
	tokenHash := signHMACSHA256(s.hmacKey, token)

	if err := s.store.Put(tokenHash, &MagicLinkToken{
		Email:     email,
		OrgID:     orgID,
		ExpiresAt: expiresAt,
		Used:      false,
		Token:     "",
	}); err != nil {
		return "", err
	}

	return token, nil
}

func (s *MagicLinkService) ValidateToken(token string) (*MagicLinkToken, error) {
	if s == nil {
		return nil, ErrMagicLinkInvalidToken
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrMagicLinkInvalidToken
	}

	now := s.now().UTC()
	tokenHash := signHMACSHA256(s.hmacKey, token)

	used, err := s.store.Consume(tokenHash, now)
	if err != nil {
		return nil, err
	}
	used.Token = token
	return used, nil
}

func (s *MagicLinkService) BuildMagicLinkURL(baseURL, token string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("baseURL is required")
	}
	if token == "" {
		return "", fmt.Errorf("token is required")
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("baseURL must include scheme and host")
	}

	u.Path = strings.TrimRight(u.Path, "/") + "/api/public/magic-link/verify"
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// SendMagicLink is a convenience wrapper when a full base URL is available.
func (s *MagicLinkService) SendMagicLink(email, orgID, token, baseURL string) error {
	if s == nil || s.emailer == nil {
		return fmt.Errorf("magic link emailer not configured")
	}
	magicURL, err := s.BuildMagicLinkURL(baseURL, token)
	if err != nil {
		return fmt.Errorf("build magic link URL: %w", err)
	}
	if err := s.emailer.SendMagicLink(email, magicURL); err != nil {
		return fmt.Errorf("send magic link email: %w", err)
	}
	return nil
}

func signHMACSHA256(key []byte, payload string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}

func randomOpaqueTokenID() (string, error) {
	// 32 bytes => 43 chars base64url (no padding). Prefix enables future migrations/format checks.
	raw := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", fmt.Errorf("token id: %w", err)
	}
	return "ml1_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func redactMagicLinkURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return ""
	}
	// Keep scheme/host/path. Drop query/fragment entirely to avoid logging credentials.
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
