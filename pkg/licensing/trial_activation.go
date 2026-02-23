package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// TrialActivationPublicKeyEnvVar overrides the public key used to validate
	// hosted trial activation tokens.
	TrialActivationPublicKeyEnvVar = "PULSE_TRIAL_ACTIVATION_PUBLIC_KEY"

	// TrialActivationIssuer is the JWT issuer for hosted trial activation tokens.
	TrialActivationIssuer = "pulse-pro-trial-signup"

	// TrialActivationAudience is the JWT audience for hosted trial activation tokens.
	TrialActivationAudience = "pulse-pro-trial-activation"
)

var (
	ErrTrialActivationPrivateKeyMissing = errors.New("trial activation private key is required")
	ErrTrialActivationPrivateKeyInvalid = errors.New("invalid trial activation private key")
	ErrTrialActivationPublicKeyMissing  = errors.New("trial activation public key is not configured")
	ErrTrialActivationPublicKeyInvalid  = errors.New("invalid trial activation public key")
	ErrTrialActivationOrgIDMissing      = errors.New("trial activation org_id is required")
	ErrTrialActivationInstanceMissing   = errors.New("trial activation instance host is required")
	ErrTrialActivationHostMismatch      = errors.New("trial activation token host mismatch")
)

// TrialActivationClaims are signed by the hosted signup service and consumed by
// self-hosted Pulse instances to start a Pro trial after registration/checkout.
type TrialActivationClaims struct {
	OrgID        string `json:"org_id"`
	Email        string `json:"email,omitempty"`
	InstanceHost string `json:"instance_host"`
	jwt.RegisteredClaims
}

// DecodeEd25519PrivateKey decodes a base64-encoded Ed25519 private key.
// Supports 64-byte private keys and 32-byte seeds.
func DecodeEd25519PrivateKey(encoded string) (ed25519.PrivateKey, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, ErrTrialActivationPrivateKeyMissing
	}

	decoded, err := decodeBase64Flexible(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTrialActivationPrivateKeyInvalid, err)
	}

	switch len(decoded) {
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(decoded), nil
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(decoded), nil
	default:
		return nil, fmt.Errorf("%w: expected %d or %d bytes, got %d",
			ErrTrialActivationPrivateKeyInvalid, ed25519.PrivateKeySize, ed25519.SeedSize, len(decoded))
	}
}

// TrialActivationPublicKey resolves the verification key for hosted trial
// activation tokens. Environment override takes precedence over embedded key.
func TrialActivationPublicKey() (ed25519.PublicKey, error) {
	if env := strings.TrimSpace(os.Getenv(TrialActivationPublicKeyEnvVar)); env != "" {
		key, err := DecodePublicKey(env)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrTrialActivationPublicKeyInvalid, err)
		}
		return key, nil
	}

	embedded := strings.TrimSpace(EmbeddedPublicKey)
	if embedded == "" {
		return nil, ErrTrialActivationPublicKeyMissing
	}
	key, err := DecodePublicKey(embedded)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTrialActivationPublicKeyInvalid, err)
	}
	return key, nil
}

// SignTrialActivationToken signs a hosted trial activation JWT.
func SignTrialActivationToken(privateKey ed25519.PrivateKey, claims TrialActivationClaims) (string, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return "", ErrTrialActivationPrivateKeyInvalid
	}
	claims.OrgID = strings.TrimSpace(claims.OrgID)
	if claims.OrgID == "" {
		return "", ErrTrialActivationOrgIDMissing
	}
	claims.InstanceHost = normalizeHost(claims.InstanceHost)
	if claims.InstanceHost == "" {
		return "", ErrTrialActivationInstanceMissing
	}

	now := time.Now().UTC()
	if claims.IssuedAt == nil {
		claims.IssuedAt = jwt.NewNumericDate(now)
	}
	if claims.ExpiresAt == nil {
		claims.ExpiresAt = jwt.NewNumericDate(now.Add(10 * time.Minute))
	}
	if strings.TrimSpace(claims.ID) == "" {
		jti, err := randomJTI()
		if err != nil {
			return "", fmt.Errorf("generate trial activation jti: %w", err)
		}
		claims.ID = jti
	}
	if strings.TrimSpace(claims.Issuer) == "" {
		claims.Issuer = TrialActivationIssuer
	}
	if len(claims.Audience) == 0 {
		claims.Audience = jwt.ClaimStrings{TrialActivationAudience}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signed, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("sign trial activation token: %w", err)
	}
	return signed, nil
}

// VerifyTrialActivationToken verifies a hosted trial activation JWT.
func VerifyTrialActivationToken(token string, publicKey ed25519.PublicKey, expectedInstanceHost string, now time.Time) (*TrialActivationClaims, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return nil, ErrTrialActivationPublicKeyInvalid
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("trial activation token is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	claims := &TrialActivationClaims{}
	parsed, err := jwt.ParseWithClaims(
		token,
		claims,
		func(t *jwt.Token) (any, error) {
			if t.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
				return nil, fmt.Errorf("unexpected signing method: %s", t.Method.Alg())
			}
			return publicKey, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(TrialActivationIssuer),
		jwt.WithAudience(TrialActivationAudience),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("trial activation token is invalid")
	}

	claims.OrgID = strings.TrimSpace(claims.OrgID)
	if claims.OrgID == "" {
		return nil, ErrTrialActivationOrgIDMissing
	}
	claims.InstanceHost = normalizeHost(claims.InstanceHost)
	if claims.InstanceHost == "" {
		return nil, ErrTrialActivationInstanceMissing
	}

	expected := normalizeHost(expectedInstanceHost)
	if expected != "" && !strings.EqualFold(claims.InstanceHost, expected) {
		return nil, ErrTrialActivationHostMismatch
	}
	return claims, nil
}

func randomJTI() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func decodeBase64Flexible(encoded string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err == nil {
		return decoded, nil
	}
	decoded, err = base64.RawStdEncoding.DecodeString(encoded)
	if err == nil {
		return decoded, nil
	}
	decoded, err = base64.URLEncoding.DecodeString(encoded)
	if err == nil {
		return decoded, nil
	}
	return base64.RawURLEncoding.DecodeString(encoded)
}

func normalizeHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if strings.Contains(raw, "://") {
		if parsed, err := url.Parse(raw); err == nil {
			raw = parsed.Host
		}
	}

	host := raw
	if h, p, err := net.SplitHostPort(raw); err == nil {
		if p != "" {
			host = h
		}
	}
	host = strings.Trim(host, "[]")
	return strings.ToLower(strings.TrimSpace(host))
}
