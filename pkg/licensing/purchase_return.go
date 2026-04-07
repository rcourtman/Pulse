package licensing

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// PurchaseReturnTokenIssuer is the JWT issuer for self-hosted commercial return tokens.
	PurchaseReturnTokenIssuer = "pulse-self-hosted-purchase"

	// PurchaseReturnTokenAudience is the JWT audience for self-hosted commercial return tokens.
	PurchaseReturnTokenAudience = "pulse-self-hosted-purchase-return"

	// PurchaseReturnPath is the canonical Pulse callback path for completing a self-hosted upgrade.
	PurchaseReturnPath = "/auth/license-purchase-activate"
)

var (
	ErrPurchaseReturnSigningKeyInvalid = errors.New("purchase return signing key is invalid")
	ErrPurchaseReturnOrgIDMissing      = errors.New("purchase return org_id is required")
	ErrPurchaseReturnInstanceMissing   = errors.New("purchase return instance host is required")
	ErrPurchaseReturnReturnURLMissing  = errors.New("purchase return return_url is required")
	ErrPurchaseReturnReturnURLInvalid  = errors.New("purchase return return_url is invalid")
	ErrPurchaseReturnHostMismatch      = errors.New("purchase return token host mismatch")
)

// PurchaseReturnClaims are minted by the local Pulse instance before handing an
// operator into Pulse Account checkout. Pulse verifies them when the completed
// checkout returns to the local instance for activation.
type PurchaseReturnClaims struct {
	OrgID        string `json:"org_id"`
	Feature      string `json:"feature,omitempty"`
	InstanceHost string `json:"instance_host"`
	ReturnURL    string `json:"return_url"`
	jwt.RegisteredClaims
}

// SignPurchaseReturnToken signs a self-hosted Pulse Account return JWT using a
// local instance-owned HMAC key.
func SignPurchaseReturnToken(signingKey []byte, claims PurchaseReturnClaims) (string, error) {
	if len(signingKey) < 32 {
		return "", ErrPurchaseReturnSigningKeyInvalid
	}
	claims.OrgID = strings.TrimSpace(claims.OrgID)
	if claims.OrgID == "" {
		return "", ErrPurchaseReturnOrgIDMissing
	}
	claims.Feature = strings.TrimSpace(claims.Feature)
	claims.InstanceHost = normalizeHost(claims.InstanceHost)
	if claims.InstanceHost == "" {
		return "", ErrPurchaseReturnInstanceMissing
	}
	claims.ReturnURL = strings.TrimSpace(claims.ReturnURL)
	returnHost, err := ValidatePurchaseReturnURL(claims.ReturnURL, claims.InstanceHost)
	if err != nil {
		return "", err
	}
	claims.InstanceHost = returnHost

	now := time.Now().UTC()
	if claims.IssuedAt == nil {
		claims.IssuedAt = jwt.NewNumericDate(now)
	}
	if claims.ExpiresAt == nil {
		claims.ExpiresAt = jwt.NewNumericDate(now.Add(2 * time.Hour))
	}
	if strings.TrimSpace(claims.ID) == "" {
		jti, err := randomJTI()
		if err != nil {
			return "", fmt.Errorf("generate purchase return jti: %w", err)
		}
		claims.ID = jti
	}
	if strings.TrimSpace(claims.Subject) == "" {
		claims.Subject = claims.OrgID
	}
	if strings.TrimSpace(claims.Issuer) == "" {
		claims.Issuer = PurchaseReturnTokenIssuer
	}
	if len(claims.Audience) == 0 {
		claims.Audience = jwt.ClaimStrings{PurchaseReturnTokenAudience}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(signingKey)
	if err != nil {
		return "", fmt.Errorf("sign purchase return token: %w", err)
	}
	return signed, nil
}

// VerifyPurchaseReturnToken verifies a self-hosted purchase-return JWT.
func VerifyPurchaseReturnToken(token string, signingKey []byte, expectedInstanceHost string, now time.Time) (*PurchaseReturnClaims, error) {
	if len(signingKey) < 32 {
		return nil, ErrPurchaseReturnSigningKeyInvalid
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("purchase return token is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	claims := &PurchaseReturnClaims{}
	parsed, err := jwt.ParseWithClaims(
		token,
		claims,
		func(t *jwt.Token) (any, error) {
			if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, fmt.Errorf("unexpected signing method: %s", t.Method.Alg())
			}
			return signingKey, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(PurchaseReturnTokenIssuer),
		jwt.WithAudience(PurchaseReturnTokenAudience),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("purchase return token is invalid")
	}

	claims.OrgID = strings.TrimSpace(claims.OrgID)
	if claims.OrgID == "" {
		return nil, ErrPurchaseReturnOrgIDMissing
	}
	claims.Feature = strings.TrimSpace(claims.Feature)
	claims.InstanceHost = normalizeHost(claims.InstanceHost)
	claims.ReturnURL = strings.TrimSpace(claims.ReturnURL)
	if claims.InstanceHost == "" {
		return nil, ErrPurchaseReturnInstanceMissing
	}
	returnHost, err := ValidatePurchaseReturnURL(claims.ReturnURL, claims.InstanceHost)
	if err != nil {
		return nil, err
	}
	claims.InstanceHost = returnHost

	expected := normalizeHost(expectedInstanceHost)
	if expected != "" && !strings.EqualFold(claims.InstanceHost, expected) {
		return nil, ErrPurchaseReturnHostMismatch
	}
	return claims, nil
}

// ValidatePurchaseReturnURL validates the Pulse Account checkout callback target
// and returns its normalized hostname.
func ValidatePurchaseReturnURL(raw, expectedInstanceHost string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrPurchaseReturnReturnURLMissing
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return "", ErrPurchaseReturnReturnURLInvalid
	}
	if !parsed.IsAbs() || strings.TrimSpace(parsed.Host) == "" {
		return "", ErrPurchaseReturnReturnURLInvalid
	}
	if parsed.EscapedPath() != PurchaseReturnPath {
		return "", ErrPurchaseReturnReturnURLInvalid
	}
	if strings.TrimSpace(parsed.RawQuery) != "" || strings.TrimSpace(parsed.Fragment) != "" {
		return "", ErrPurchaseReturnReturnURLInvalid
	}

	host := normalizeHost(parsed.Hostname())
	if host == "" {
		return "", ErrPurchaseReturnReturnURLInvalid
	}
	switch strings.ToLower(strings.TrimSpace(parsed.Scheme)) {
	case "https":
	case "http":
		if !isAllowedInsecureTrialActivationReturnHost(host) {
			return "", ErrPurchaseReturnReturnURLInvalid
		}
	default:
		return "", ErrPurchaseReturnReturnURLInvalid
	}

	expected := normalizeHost(expectedInstanceHost)
	if expected != "" && !strings.EqualFold(host, expected) {
		return "", ErrPurchaseReturnHostMismatch
	}
	return host, nil
}
