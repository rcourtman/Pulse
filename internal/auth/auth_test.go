package auth

import (
	"strings"
	"testing"
)

func TestGenerateAPIToken(t *testing.T) {
	token, err := GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken() error: %v", err)
	}

	// Should be 64 hex characters (32 bytes)
	if len(token) != 64 {
		t.Errorf("GenerateAPIToken() length = %d, want 64", len(token))
	}

	// Should be valid hex
	for _, c := range token {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("GenerateAPIToken() contains invalid hex character: %c", c)
		}
	}

	// Should generate unique tokens
	token2, err := GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken() second call error: %v", err)
	}
	if token == token2 {
		t.Error("GenerateAPIToken() generated duplicate tokens")
	}
}

func TestHashAPIToken(t *testing.T) {
	token := "test-token-12345"
	hash := HashAPIToken(token)

	// Should be 64 hex characters (SHA3-256)
	if len(hash) != 64 {
		t.Errorf("HashAPIToken() length = %d, want 64", len(hash))
	}

	// Should be deterministic
	hash2 := HashAPIToken(token)
	if hash != hash2 {
		t.Error("HashAPIToken() is not deterministic")
	}

	// Different tokens should produce different hashes
	hash3 := HashAPIToken("different-token")
	if hash == hash3 {
		t.Error("HashAPIToken() produced same hash for different tokens")
	}
}

func TestCompareAPIToken(t *testing.T) {
	token := "my-secret-token-abc123"
	hash := HashAPIToken(token)

	// Correct token should match
	if !CompareAPIToken(token, hash) {
		t.Error("CompareAPIToken() returned false for correct token")
	}

	// Wrong token should not match
	if CompareAPIToken("wrong-token", hash) {
		t.Error("CompareAPIToken() returned true for wrong token")
	}

	// Empty token should not match
	if CompareAPIToken("", hash) {
		t.Error("CompareAPIToken() returned true for empty token")
	}

	// Token against wrong hash should not match
	if CompareAPIToken(token, "0000000000000000000000000000000000000000000000000000000000000000") {
		t.Error("CompareAPIToken() returned true for wrong hash")
	}
}

func TestCompareAPIToken_TimingSafe(t *testing.T) {
	// This test verifies the comparison uses constant-time comparison
	// by checking that it uses subtle.ConstantTimeCompare internally
	// (verified by code inspection - this test just ensures the function works)
	token := "timing-test-token"
	hash := HashAPIToken(token)

	// Multiple comparisons should all succeed
	for i := 0; i < 100; i++ {
		if !CompareAPIToken(token, hash) {
			t.Errorf("CompareAPIToken() failed on iteration %d", i)
		}
	}
}

func TestIsAPITokenHashed(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid 64-char hex",
			input:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			expected: true,
		},
		{
			name:     "actual hash",
			input:    HashAPIToken("test"),
			expected: true,
		},
		{
			name:     "too short",
			input:    "0123456789abcdef",
			expected: false,
		},
		{
			name:     "too long",
			input:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef00",
			expected: false,
		},
		{
			name:     "invalid hex characters",
			input:    "ghijklmnopqrstuv0123456789abcdef0123456789abcdef0123456789abcdef",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "plain token (not hashed)",
			input:    "my-api-token",
			expected: false,
		},
		{
			name:     "uppercase hex",
			input:    "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF",
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsAPITokenHashed(tc.input)
			if result != tc.expected {
				t.Errorf("IsAPITokenHashed(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestHashPassword(t *testing.T) {
	password := "mysecretpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	// Should be a bcrypt hash (starts with $2)
	if !strings.HasPrefix(hash, "$2") {
		t.Errorf("HashPassword() did not produce bcrypt hash, got: %s", hash[:10])
	}

	// Should be 60 characters
	if len(hash) != 60 {
		t.Errorf("HashPassword() length = %d, want 60", len(hash))
	}

	// Same password should produce different hashes (due to salt)
	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() second call error: %v", err)
	}
	if hash == hash2 {
		t.Error("HashPassword() produced same hash twice (missing salt?)")
	}
}

func TestCheckPasswordHash(t *testing.T) {
	password := "testpassword456"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	// Correct password should verify
	if !CheckPasswordHash(password, hash) {
		t.Error("CheckPasswordHash() returned false for correct password")
	}

	// Wrong password should not verify
	if CheckPasswordHash("wrongpassword", hash) {
		t.Error("CheckPasswordHash() returned true for wrong password")
	}

	// Empty password should not verify
	if CheckPasswordHash("", hash) {
		t.Error("CheckPasswordHash() returned true for empty password")
	}
}

func TestValidatePasswordComplexity(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		wantError bool
	}{
		{
			name:      "valid long password",
			password:  "thisisaverylongpassword",
			wantError: false,
		},
		{
			name:      "exactly minimum length",
			password:  "123456789012", // 12 chars
			wantError: false,
		},
		{
			name:      "too short",
			password:  "12345678901", // 11 chars
			wantError: true,
		},
		{
			name:      "empty password",
			password:  "",
			wantError: true,
		},
		{
			name:      "single character",
			password:  "a",
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePasswordComplexity(tc.password)
			if tc.wantError && err == nil {
				t.Errorf("ValidatePasswordComplexity(%q) expected error, got nil", tc.password)
			}
			if !tc.wantError && err != nil {
				t.Errorf("ValidatePasswordComplexity(%q) unexpected error: %v", tc.password, err)
			}
		})
	}
}

func TestBcryptCost(t *testing.T) {
	// Verify bcrypt cost is reasonable (10-14 is typical)
	if BcryptCost < 10 || BcryptCost > 14 {
		t.Errorf("BcryptCost = %d, should be between 10 and 14 for reasonable security/performance", BcryptCost)
	}
}

func TestMinPasswordLength(t *testing.T) {
	// Verify minimum password length is reasonable (at least 8, ideally 12+)
	if MinPasswordLength < 8 {
		t.Errorf("MinPasswordLength = %d, should be at least 8", MinPasswordLength)
	}
}
