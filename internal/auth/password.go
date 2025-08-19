package auth

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost is the cost factor for bcrypt hashing
	// Higher values are more secure but slower
	BcryptCost = 12

	// MinPasswordLength is the minimum required password length
	MinPasswordLength = 8
)

// HashPassword generates a bcrypt hash from a plain text password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPasswordHash compares a plain text password with a hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// IsPasswordHashed checks if a string looks like a bcrypt hash
func IsPasswordHashed(password string) bool {
	// Bcrypt hashes start with $2a$, $2b$, or $2y$ and are 60 characters long
	return strings.HasPrefix(password, "$2") && len(password) == 60
}

// MigratePassword takes a password that might be plain text or hashed
// and returns a properly hashed version
func MigratePassword(password string) (string, error) {
	if IsPasswordHashed(password) {
		// Already hashed, return as-is
		return password, nil
	}
	// Plain text password, hash it
	return HashPassword(password)
}

// ValidatePasswordComplexity checks if a password meets complexity requirements
func ValidatePasswordComplexity(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters long", MinPasswordLength)
	}

	// That's it - let users choose their own passwords
	// No annoying character type requirements
	return nil
}
