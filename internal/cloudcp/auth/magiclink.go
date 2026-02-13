package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	magicLinkTTL    = 15 * time.Minute
	magicLinkPrefix = "ml1_"
	hmacKeyFile     = ".cp_magic_link_key"
	hmacKeySize     = 32
)

// Token holds the validated data from a consumed magic link token.
type Token struct {
	Email     string
	TenantID  string
	ExpiresAt time.Time
}

// Service manages magic link token generation and validation for the control plane.
// It does NOT import internal/api — it is a standalone reimplementation using its own SQLite store.
type Service struct {
	hmacKey []byte
	store   *Store
	ttl     time.Duration
	now     func() time.Time
}

// NewService creates a Service backed by a SQLite store in cpDataDir.
// It loads (or generates) an HMAC key from {cpDataDir}/.cp_magic_link_key.
func NewService(cpDataDir string) (*Service, error) {
	if err := ensureOwnerOnlyDir(cpDataDir); err != nil {
		return nil, fmt.Errorf("ensure cp data dir: %w", err)
	}

	key, err := loadOrGenerateKey(filepath.Join(cpDataDir, hmacKeyFile))
	if err != nil {
		return nil, fmt.Errorf("magic link hmac key: %w", err)
	}

	store, err := NewStore(cpDataDir)
	if err != nil {
		return nil, fmt.Errorf("magic link store: %w", err)
	}

	return &Service{
		hmacKey: key,
		store:   store,
		ttl:     magicLinkTTL,
		now:     time.Now,
	}, nil
}

// GenerateToken creates a new magic link token for the given email and tenant.
// Returns a string in the format "ml1_<random>" that can be included in a URL.
func (s *Service) GenerateToken(email, tenantID string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("magic link service not configured")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	tenantID = strings.TrimSpace(tenantID)
	if email == "" || tenantID == "" {
		return "", fmt.Errorf("email and tenantID are required")
	}

	expiresAt := s.now().UTC().Add(s.ttl)
	expiresAt = time.Unix(expiresAt.Unix(), 0).UTC()

	token, err := randomToken()
	if err != nil {
		return "", err
	}

	tokenHash := signHMAC(s.hmacKey, token)
	if err := s.store.Put(tokenHash, &TokenRecord{
		Email:     email,
		TenantID:  tenantID,
		ExpiresAt: expiresAt,
	}); err != nil {
		return "", err
	}

	return token, nil
}

// ValidateToken atomically consumes a token and returns the associated data.
// The token can only be used once.
func (s *Service) ValidateToken(token string) (*Token, error) {
	if s == nil {
		return nil, ErrTokenInvalid
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrTokenInvalid
	}

	tokenHash := signHMAC(s.hmacKey, token)
	rec, err := s.store.Consume(tokenHash, s.now().UTC())
	if err != nil {
		return nil, err
	}
	return &Token{
		Email:     rec.Email,
		TenantID:  rec.TenantID,
		ExpiresAt: rec.ExpiresAt,
	}, nil
}

// BuildVerifyURL constructs the full magic link verification URL.
func BuildVerifyURL(baseURL, token string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || token == "" {
		return ""
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/auth/magic-link/verify"
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String()
}

// Close releases resources held by the service.
func (s *Service) Close() {
	if s == nil {
		return
	}
	if s.store != nil {
		s.store.Close()
	}
}

func loadOrGenerateKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil && len(data) >= hmacKeySize {
		return data[:hmacKeySize], nil
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read key file %s: %w", path, err)
	}

	key := make([]byte, hmacKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("write key file %s: %w", path, err)
	}
	log.Info().Str("path", path).Msg("Generated new magic link HMAC key")
	return key, nil
}

func randomToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return magicLinkPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func signHMAC(key []byte, payload string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	return mac.Sum(nil)
}
