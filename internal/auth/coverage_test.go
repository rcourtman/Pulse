package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashPassword_Error(t *testing.T) {
	// bcrypt has a max length limit (usually 72 bytes).
	// Passing a very long password should trigger an error.
	longPassword := strings.Repeat("A", 80)
	_, err := HashPassword(longPassword)
	if err == nil {
		t.Error("HashPassword() expected error for long password, got nil")
	}
}

func TestGenerateAPIToken_Error(t *testing.T) {
	originalRandRead := randRead
	defer func() { randRead = originalRandRead }()

	randRead = func(b []byte) (n int, err error) {
		return 0, errors.New("forced error")
	}

	_, err := GenerateAPIToken()
	if err == nil {
		t.Error("GenerateAPIToken() expected error when rand.Read fails, got nil")
	}
}
