package api

import (
	"errors"
	"testing"
	"time"
)

func TestMagicLinkTokenRoundTrip(t *testing.T) {
	svc := NewMagicLinkServiceWithKey([]byte("0123456789abcdef0123456789abcdef"), nil, nil, nil)
	t.Cleanup(func() { svc.Stop() })

	token, err := svc.GenerateToken("User@Example.com", "org-123")
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}

	got, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken error: %v", err)
	}
	if got.Email != "user@example.com" {
		t.Fatalf("Email = %q, want user@example.com", got.Email)
	}
	if got.OrgID != "org-123" {
		t.Fatalf("OrgID = %q, want org-123", got.OrgID)
	}
	if !got.Used {
		t.Fatalf("Used = false, want true")
	}
	if got.Token != token {
		t.Fatalf("Token mismatch")
	}
}

func TestMagicLinkExpiredTokenRejected(t *testing.T) {
	svc := NewMagicLinkServiceWithKey([]byte("0123456789abcdef0123456789abcdef"), nil, nil, nil)
	t.Cleanup(func() { svc.Stop() })

	base := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return base }

	token, err := svc.GenerateToken("user@example.com", "org-123")
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}

	svc.now = func() time.Time { return base.Add(16 * time.Minute) }
	_, err = svc.ValidateToken(token)
	if !errors.Is(err, ErrMagicLinkExpired) {
		t.Fatalf("ValidateToken err = %v, want %v", err, ErrMagicLinkExpired)
	}
}

func TestMagicLinkUsedTokenRejected(t *testing.T) {
	svc := NewMagicLinkServiceWithKey([]byte("0123456789abcdef0123456789abcdef"), nil, nil, nil)
	t.Cleanup(func() { svc.Stop() })

	token, err := svc.GenerateToken("user@example.com", "org-123")
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}

	if _, err := svc.ValidateToken(token); err != nil {
		t.Fatalf("first ValidateToken error: %v", err)
	}
	_, err = svc.ValidateToken(token)
	if !errors.Is(err, ErrMagicLinkUsed) {
		t.Fatalf("second ValidateToken err = %v, want %v", err, ErrMagicLinkUsed)
	}
}

func TestMagicLinkInvalidTokenRejected(t *testing.T) {
	svc := NewMagicLinkServiceWithKey([]byte("0123456789abcdef0123456789abcdef"), nil, nil, nil)
	t.Cleanup(func() { svc.Stop() })

	token, err := svc.GenerateToken("user@example.com", "org-123")
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}

	bad := token + "x"

	_, err = svc.ValidateToken(bad)
	if !errors.Is(err, ErrMagicLinkInvalidToken) {
		t.Fatalf("ValidateToken err = %v, want %v", err, ErrMagicLinkInvalidToken)
	}
}

func TestMagicLinkRateLimiting(t *testing.T) {
	limiter := NewRateLimiter(3, 1*time.Hour)
	svc := NewMagicLinkServiceWithKey([]byte("0123456789abcdef0123456789abcdef"), nil, nil, limiter)
	t.Cleanup(func() { svc.Stop() })

	email := "user@example.com"
	if !svc.AllowRequest(email) {
		t.Fatalf("1st request unexpectedly blocked")
	}
	if !svc.AllowRequest(email) {
		t.Fatalf("2nd request unexpectedly blocked")
	}
	if !svc.AllowRequest(email) {
		t.Fatalf("3rd request unexpectedly blocked")
	}
	if svc.AllowRequest(email) {
		t.Fatalf("4th request allowed, expected rate limited")
	}
}
