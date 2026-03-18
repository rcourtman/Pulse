package cpsec

import (
	"context"
	"testing"
)

func TestWithNonce_AndNonceFromContext(t *testing.T) {
	nonce := "abc123def456"
	ctx := WithNonce(context.Background(), nonce)

	got := NonceFromContext(ctx)
	if got != nonce {
		t.Errorf("expected nonce %q, got %q", nonce, got)
	}
}

func TestNonceFromContext_EmptyContext(t *testing.T) {
	got := NonceFromContext(context.Background())
	if got != "" {
		t.Errorf("expected empty nonce from bare context, got %q", got)
	}
}

func TestNonceFromContext_EmptyNonce(t *testing.T) {
	ctx := WithNonce(context.Background(), "")
	got := NonceFromContext(ctx)
	if got != "" {
		t.Errorf("expected empty nonce when set to empty, got %q", got)
	}
}

func TestWithNonce_OverwritesPrevious(t *testing.T) {
	ctx := WithNonce(context.Background(), "first")
	ctx = WithNonce(ctx, "second")
	got := NonceFromContext(ctx)
	if got != "second" {
		t.Errorf("expected second nonce to win, got %q", got)
	}
}

func TestNonceFromContext_DoesNotLeakToParent(t *testing.T) {
	parent := context.Background()
	child := WithNonce(parent, "child-nonce")

	if NonceFromContext(parent) != "" {
		t.Error("parent context should not have nonce")
	}
	if NonceFromContext(child) != "child-nonce" {
		t.Error("child context should have nonce")
	}
}

func TestWithNonce_SpecialCharacters(t *testing.T) {
	// CSP nonces are typically base64-encoded; verify special characters work.
	nonce := "R2FyYmFnZQ==+/test"
	ctx := WithNonce(context.Background(), nonce)
	got := NonceFromContext(ctx)
	if got != nonce {
		t.Errorf("expected %q, got %q", nonce, got)
	}
}
