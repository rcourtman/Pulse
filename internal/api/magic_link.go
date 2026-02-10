package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
	ErrMagicLinkRateLimited  = errors.New("magic link requests rate limited")
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
	log.Info().Str("to", to).Str("magic_link_url", magicLinkURL).Msg("Magic link email (log-only)")
	return nil
}

type MagicLinkToken struct {
	Email     string
	OrgID     string
	ExpiresAt time.Time
	Used      bool
	Token     string // HMAC-signed opaque token (base64url(payload).base64url(sig))
}

type MagicLinkStore interface {
	Put(token *MagicLinkToken) error
	Get(token string) (*MagicLinkToken, bool)
	MarkUsed(token string) (*MagicLinkToken, error)
	DeleteExpired(now time.Time)
	Stop()
}

type InMemoryMagicLinkStore struct {
	mu     sync.RWMutex
	tokens map[string]*MagicLinkToken

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

func (s *InMemoryMagicLinkStore) Put(token *MagicLinkToken) error {
	if token == nil || token.Token == "" {
		return fmt.Errorf("token is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token.Token] = token
	return nil
}

func (s *InMemoryMagicLinkStore) Get(token string) (*MagicLinkToken, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tokens[token]
	if !ok || t == nil {
		return nil, false
	}
	clone := *t
	return &clone, true
}

func (s *InMemoryMagicLinkStore) MarkUsed(token string) (*MagicLinkToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tokens[token]
	if !ok || t == nil {
		return nil, ErrMagicLinkInvalidToken
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
	hmacKey []byte
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
	return NewMagicLinkServiceWithKey(key, nil, emailer, nil), nil
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
	// The signed payload stores seconds; truncate to seconds so validation can compare exactly.
	expiresAt = time.Unix(expiresAt.Unix(), 0).UTC()
	nonce, err := randomNonce(18)
	if err != nil {
		return "", err
	}

	payload := fmt.Sprintf("%s|%s|%d|%s", email, orgID, expiresAt.Unix(), nonce)
	sig := signHMACSHA256(s.hmacKey, payload)
	token := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(sig)

	if err := s.store.Put(&MagicLinkToken{
		Email:     email,
		OrgID:     orgID,
		ExpiresAt: expiresAt,
		Used:      false,
		Token:     token,
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

	email, orgID, expiresAt, err := validateAndParseSignedToken(s.hmacKey, token)
	if err != nil {
		return nil, err
	}

	now := s.now().UTC()
	if now.After(expiresAt) {
		return nil, ErrMagicLinkExpired
	}

	// Single-use: require presence in store and atomically mark used.
	stored, ok := s.store.Get(token)
	if !ok || stored == nil {
		return nil, ErrMagicLinkInvalidToken
	}
	if stored.Used {
		return nil, ErrMagicLinkUsed
	}
	if stored.Email != email || stored.OrgID != orgID || !stored.ExpiresAt.Equal(expiresAt) {
		return nil, ErrMagicLinkInvalidToken
	}
	used, err := s.store.MarkUsed(token)
	if err != nil {
		return nil, err
	}
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
		return err
	}
	return s.emailer.SendMagicLink(email, magicURL)
}

func randomNonce(n int) (string, error) {
	if n <= 0 {
		n = 18
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func signHMACSHA256(key []byte, payload string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}

func validateAndParseSignedToken(key []byte, token string) (email string, orgID string, expiresAt time.Time, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", "", time.Time{}, ErrMagicLinkInvalidToken
	}

	payloadBytes, decErr := base64.RawURLEncoding.DecodeString(parts[0])
	if decErr != nil {
		return "", "", time.Time{}, ErrMagicLinkInvalidToken
	}
	sigBytes, decErr := base64.RawURLEncoding.DecodeString(parts[1])
	if decErr != nil || len(sigBytes) != sha256.Size {
		return "", "", time.Time{}, ErrMagicLinkInvalidToken
	}

	payload := string(payloadBytes)
	expected := signHMACSHA256(key, payload)
	if !hmac.Equal(sigBytes, expected) {
		return "", "", time.Time{}, ErrMagicLinkInvalidToken
	}

	fields := strings.Split(payload, "|")
	if len(fields) != 4 {
		return "", "", time.Time{}, ErrMagicLinkInvalidToken
	}

	email = strings.ToLower(strings.TrimSpace(fields[0]))
	orgID = strings.TrimSpace(fields[1])
	expUnixStr := strings.TrimSpace(fields[2])
	nonce := strings.TrimSpace(fields[3])
	if email == "" || orgID == "" || expUnixStr == "" || nonce == "" {
		return "", "", time.Time{}, ErrMagicLinkInvalidToken
	}

	expUnix, parseErr := parseInt64(expUnixStr)
	if parseErr != nil {
		return "", "", time.Time{}, ErrMagicLinkInvalidToken
	}
	expiresAt = time.Unix(expUnix, 0).UTC()
	return email, orgID, expiresAt, nil
}

func parseInt64(s string) (int64, error) {
	var n int64
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int64(s[i]-'0')
	}
	return n, nil
}
