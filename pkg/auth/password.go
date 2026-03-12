package auth

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost is the cost factor for bcrypt hashing
	// Higher values are more secure but slower
	BcryptCost = 12

	// MinPasswordLength is the minimum required password length
	MinPasswordLength = 12
)

// HashPassword generates a bcrypt hash from a plain text password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), hashPasswordCost())
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

// ValidatePasswordComplexity checks if a password meets complexity requirements
func ValidatePasswordComplexity(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters long", MinPasswordLength)
	}

	// Let users choose their own passwords beyond length.
	// No character type requirements.
	return nil
}

func hashPasswordCost() int {
	if !runningUnderGoTest() {
		return BcryptCost
	}

	raw := strings.TrimSpace(os.Getenv("PULSE_TEST_BCRYPT_COST"))
	if raw == "" {
		return BcryptCost
	}

	cost, err := strconv.Atoi(raw)
	if err != nil || cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return BcryptCost
	}

	return cost
}

func runningUnderGoTest() bool {
	return strings.HasSuffix(os.Args[0], ".test")
}
