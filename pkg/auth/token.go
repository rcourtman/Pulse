package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"

	"golang.org/x/crypto/sha3"
)

// randRead is a variable to allow mocking in tests
var randRead = rand.Read

// GenerateAPIToken generates a secure random API token
func GenerateAPIToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := randRead(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// HashAPIToken creates a one-way hash of an API token for storage
// We use SHA3-256 for API tokens since we need to compare exact values
func HashAPIToken(token string) string {
	hash := sha3.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// CompareAPIToken compares a provided token with a stored hash
func CompareAPIToken(token, hash string) bool {
	tokenHash := HashAPIToken(token)
	return subtle.ConstantTimeCompare([]byte(tokenHash), []byte(hash)) == 1
}

// IsAPITokenHashed checks if a string looks like a hashed API token
func IsAPITokenHashed(token string) bool {
	// SHA3-256 produces 64 character hex strings
	if len(token) != 64 {
		return false
	}
	// Check if it's valid hex
	_, err := hex.DecodeString(token)
	return err == nil
}
