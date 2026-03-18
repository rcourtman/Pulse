package auth

import (
	"testing"
	"time"
)

func TestSessionTokenIncludesVersion(t *testing.T) {
	svc, err := NewService(t.TempDir())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(svc.Close)

	token, err := svc.GenerateSessionTokenWithVersion("u_test", "owner@example.com", 7, time.Hour)
	if err != nil {
		t.Fatalf("GenerateSessionTokenWithVersion: %v", err)
	}

	claims, err := svc.ValidateSessionToken(token)
	if err != nil {
		t.Fatalf("ValidateSessionToken: %v", err)
	}
	if claims.SessionVersion != 7 {
		t.Fatalf("SessionVersion = %d, want 7", claims.SessionVersion)
	}
}

func TestSessionTokenDefaultVersion(t *testing.T) {
	svc, err := NewService(t.TempDir())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(svc.Close)

	token, err := svc.GenerateSessionToken("u_test", "owner@example.com", time.Hour)
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}

	claims, err := svc.ValidateSessionToken(token)
	if err != nil {
		t.Fatalf("ValidateSessionToken: %v", err)
	}
	if claims.SessionVersion != 1 {
		t.Fatalf("SessionVersion = %d, want 1", claims.SessionVersion)
	}
}
