package auth

import (
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// SessionCookieName is the cookie used for control-plane authenticated sessions.
	SessionCookieName = "pulse_cp_session"
	// SessionTTL is the default session token lifetime.
	SessionTTL    = 12 * time.Hour
	sessionPrefix = "cps1_"
)

var (
	ErrSessionInvalid = errors.New("session token invalid")
	ErrSessionExpired = errors.New("session token expired")
)

// SessionClaims are the authenticated claims for a control-plane session.
type SessionClaims struct {
	UserID         string
	Email          string
	SessionVersion int64
	IssuedAt       time.Time
	ExpiresAt      time.Time
}

type sessionPayload struct {
	UserID   string `json:"u"`
	Email    string `json:"e"`
	Version  int64  `json:"v"`
	IssuedAt int64  `json:"i"`
	Expiry   int64  `json:"x"`
}

// GenerateSessionToken creates an HMAC-signed control-plane session token.
func (s *Service) GenerateSessionToken(userID, email string, ttl time.Duration) (string, error) {
	return s.GenerateSessionTokenWithVersion(userID, email, 1, ttl)
}

// GenerateSessionTokenWithVersion creates an HMAC-signed control-plane session token
// bound to a user session-version counter.
func (s *Service) GenerateSessionTokenWithVersion(userID, email string, sessionVersion int64, ttl time.Duration) (string, error) {
	if s == nil || len(s.hmacKey) == 0 {
		return "", ErrSessionInvalid
	}

	userID = strings.TrimSpace(userID)
	email = strings.ToLower(strings.TrimSpace(email))
	if userID == "" || email == "" {
		return "", fmt.Errorf("userID and email are required")
	}
	if sessionVersion < 1 {
		sessionVersion = 1
	}
	if ttl <= 0 {
		ttl = SessionTTL
	}

	now := s.now().UTC()
	payload := sessionPayload{
		UserID:   userID,
		Email:    email,
		Version:  sessionVersion,
		IssuedAt: now.Unix(),
		Expiry:   now.Add(ttl).Unix(),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal session payload: %w", err)
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadBytes)
	sig := signHMAC(s.hmacKey, string(payloadBytes))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return sessionPrefix + payloadB64 + "." + sigB64, nil
}

// ValidateSessionToken validates a session token and returns claims on success.
func (s *Service) ValidateSessionToken(token string) (*SessionClaims, error) {
	if s == nil || len(s.hmacKey) == 0 {
		return nil, ErrSessionInvalid
	}

	token = strings.TrimSpace(token)
	if token == "" || !strings.HasPrefix(token, sessionPrefix) {
		return nil, ErrSessionInvalid
	}
	raw := strings.TrimPrefix(token, sessionPrefix)

	dot := strings.IndexByte(raw, '.')
	if dot <= 0 || dot >= len(raw)-1 {
		return nil, ErrSessionInvalid
	}
	payloadB64 := raw[:dot]
	sigB64 := raw[dot+1:]

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, ErrSessionInvalid
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, ErrSessionInvalid
	}

	expectedSig := signHMAC(s.hmacKey, string(payloadBytes))
	if !hmac.Equal(sigBytes, expectedSig) {
		return nil, ErrSessionInvalid
	}

	var payload sessionPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, ErrSessionInvalid
	}
	if strings.TrimSpace(payload.UserID) == "" || strings.TrimSpace(payload.Email) == "" {
		return nil, ErrSessionInvalid
	}
	if payload.Version < 1 {
		payload.Version = 1
	}

	now := s.now().UTC().Unix()
	if now > payload.Expiry {
		return nil, ErrSessionExpired
	}

	return &SessionClaims{
		UserID:         strings.TrimSpace(payload.UserID),
		Email:          strings.ToLower(strings.TrimSpace(payload.Email)),
		SessionVersion: payload.Version,
		IssuedAt:       time.Unix(payload.IssuedAt, 0).UTC(),
		ExpiresAt:      time.Unix(payload.Expiry, 0).UTC(),
	}, nil
}
