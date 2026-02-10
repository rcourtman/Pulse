package cloudauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// HandoffKeyFile is the filename for the per-tenant handoff key written into the tenant data dir.
const HandoffKeyFile = ".cloud_handoff_key"

var (
	ErrHandoffInvalid = errors.New("handoff token invalid")
	ErrHandoffExpired = errors.New("handoff token expired")
)

// handoffPayload is the JSON structure inside a handoff token.
type handoffPayload struct {
	Email    string `json:"e"`
	TenantID string `json:"t"`
	Expiry   int64  `json:"x"`
	Nonce    string `json:"n"`
}

// GenerateHandoffKey returns 32 cryptographically random bytes suitable for HMAC-SHA256 signing.
func GenerateHandoffKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate handoff key: %w", err)
	}
	return key, nil
}

// Sign creates an HMAC-SHA256-signed handoff token encoding the given email, tenant ID, and TTL.
// The returned string is base64url-encoded: payload + "." + signature.
func Sign(key []byte, email, tenantID string, ttl time.Duration) (string, error) {
	if len(key) == 0 {
		return "", fmt.Errorf("handoff key is empty")
	}
	if email == "" || tenantID == "" {
		return "", fmt.Errorf("email and tenantID are required")
	}

	nonce := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	payload := handoffPayload{
		Email:    email,
		TenantID: tenantID,
		Expiry:   time.Now().UTC().Add(ttl).Unix(),
		Nonce:    base64.RawURLEncoding.EncodeToString(nonce),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal handoff payload: %w", err)
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadBytes)
	sig := computeHMAC(key, payloadBytes)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return payloadB64 + "." + sigB64, nil
}

// Verify decodes and validates a handoff token. Returns the email and tenant ID on success.
func Verify(key []byte, tokenStr string) (email, tenantID string, err error) {
	if len(key) == 0 || tokenStr == "" {
		return "", "", ErrHandoffInvalid
	}

	// Split into payload.signature
	dotIdx := -1
	for i := 0; i < len(tokenStr); i++ {
		if tokenStr[i] == '.' {
			dotIdx = i
			break
		}
	}
	if dotIdx < 1 || dotIdx >= len(tokenStr)-1 {
		return "", "", ErrHandoffInvalid
	}

	payloadB64 := tokenStr[:dotIdx]
	sigB64 := tokenStr[dotIdx+1:]

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return "", "", ErrHandoffInvalid
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return "", "", ErrHandoffInvalid
	}

	// Verify HMAC
	expected := computeHMAC(key, payloadBytes)
	if !hmac.Equal(sigBytes, expected) {
		return "", "", ErrHandoffInvalid
	}

	// Decode payload
	var p handoffPayload
	if err := json.Unmarshal(payloadBytes, &p); err != nil {
		return "", "", ErrHandoffInvalid
	}

	// Check expiry
	if time.Now().UTC().Unix() > p.Expiry {
		return "", "", ErrHandoffExpired
	}

	if p.Email == "" || p.TenantID == "" {
		return "", "", ErrHandoffInvalid
	}

	return p.Email, p.TenantID, nil
}

func computeHMAC(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
