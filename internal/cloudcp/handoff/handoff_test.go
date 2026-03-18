package handoff

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func TestMintAndVerify(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	now := time.Now().UTC()

	tokenStr, err := MintHandoffToken(secret, HandoffClaims{
		TenantID:  "t-TESTTENANT",
		UserID:    "u_TESTUSER",
		AccountID: "a_TESTACCOUNT",
		Email:     "tech@example.com",
		Role:      registry.MemberRoleTech,
		IssuedAt:  now,
		ExpiresAt: now.Add(60 * time.Second),
	})
	if err != nil {
		t.Fatalf("MintHandoffToken: %v", err)
	}

	var got jwtHandoffClaims
	parsed, err := jwt.ParseWithClaims(
		tokenStr,
		&got,
		func(t *jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience("t-TESTTENANT"),
	)
	if err != nil {
		t.Fatalf("ParseWithClaims: %v", err)
	}
	if !parsed.Valid {
		t.Fatalf("token valid = false, want true")
	}

	if got.Subject != "u_TESTUSER" {
		t.Fatalf("sub = %q, want %q", got.Subject, "u_TESTUSER")
	}
	if got.AccountID != "a_TESTACCOUNT" {
		t.Fatalf("account_id = %q, want %q", got.AccountID, "a_TESTACCOUNT")
	}
	if got.Email != "tech@example.com" {
		t.Fatalf("email = %q, want %q", got.Email, "tech@example.com")
	}
	if got.Role != registry.MemberRoleTech {
		t.Fatalf("role = %q, want %q", got.Role, registry.MemberRoleTech)
	}
	if got.ID == "" {
		t.Fatalf("jti empty")
	}
	if got.IssuedAt == nil || got.ExpiresAt == nil {
		t.Fatalf("missing iat/exp")
	}
	if got.ExpiresAt.Time.Sub(got.IssuedAt.Time) != 60*time.Second {
		t.Fatalf("exp-iat = %v, want %v", got.ExpiresAt.Time.Sub(got.IssuedAt.Time), 60*time.Second)
	}
}

func TestExpiredToken(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	past := time.Now().UTC().Add(-2 * time.Minute)

	tokenStr, err := MintHandoffToken(secret, HandoffClaims{
		TenantID:  "t-EXPIRED",
		UserID:    "u_USER",
		AccountID: "a_ACCOUNT",
		Email:     "x@example.com",
		Role:      registry.MemberRoleReadOnly,
		IssuedAt:  past,
		ExpiresAt: past.Add(30 * time.Second),
		JTI:       "deadbeefdeadbeefdeadbeefdeadbeef",
	})
	if err != nil {
		t.Fatalf("MintHandoffToken: %v", err)
	}

	var got jwtHandoffClaims
	_, err = jwt.ParseWithClaims(
		tokenStr,
		&got,
		func(t *jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience("t-EXPIRED"),
	)
	if err == nil {
		t.Fatalf("expected expired token error")
	}
	if !errors.Is(err, jwt.ErrTokenExpired) {
		// Leave room for wrapped validation errors.
		t.Fatalf("error = %v, want ErrTokenExpired", err)
	}
}

func TestWrongSecret(t *testing.T) {
	secretA := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	secretB := []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	now := time.Now().UTC()

	tokenStr, err := MintHandoffToken(secretA, HandoffClaims{
		TenantID:  "t-TENANT",
		UserID:    "u_USER",
		AccountID: "a_ACCOUNT",
		Email:     "x@example.com",
		Role:      registry.MemberRoleAdmin,
		IssuedAt:  now,
		ExpiresAt: now.Add(60 * time.Second),
		JTI:       "0123456789abcdef0123456789abcdef",
	})
	if err != nil {
		t.Fatalf("MintHandoffToken: %v", err)
	}

	var got jwtHandoffClaims
	_, err = jwt.ParseWithClaims(
		tokenStr,
		&got,
		func(t *jwt.Token) (any, error) { return secretB, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience("t-TENANT"),
	)
	if err == nil {
		t.Fatalf("expected signature verification failure")
	}
}

func TestWrongAudience(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	now := time.Now().UTC()

	tokenStr, err := MintHandoffToken(secret, HandoffClaims{
		TenantID:  "t-A",
		UserID:    "u_USER",
		AccountID: "a_ACCOUNT",
		Email:     "x@example.com",
		Role:      registry.MemberRoleOwner,
		IssuedAt:  now,
		ExpiresAt: now.Add(60 * time.Second),
		JTI:       "0123456789abcdef0123456789abcdef",
	})
	if err != nil {
		t.Fatalf("MintHandoffToken: %v", err)
	}

	var got jwtHandoffClaims
	_, err = jwt.ParseWithClaims(
		tokenStr,
		&got,
		func(t *jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience("t-B"),
	)
	if err == nil {
		t.Fatalf("expected wrong audience error")
	}
}
