package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

// CryptoManager handles encryption/decryption of sensitive data
type CryptoManager struct {
	key []byte
}

// NewCryptoManager creates a new crypto manager
func NewCryptoManager() (*CryptoManager, error) {
	key, err := getOrCreateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
	}
	
	return &CryptoManager{
		key: key,
	}, nil
}

// getOrCreateKey gets the encryption key or creates one if it doesn't exist
func getOrCreateKey() ([]byte, error) {
	// Use data directory for key storage (for Docker persistence)
	dataDir := os.Getenv("PULSE_DATA_DIR")
	if dataDir == "" {
		dataDir = "/etc/pulse"
	}
	keyPath := filepath.Join(dataDir, ".encryption.key")
	oldKeyPath := "/etc/pulse/.encryption.key"
	
	// Try to read existing key from new location
	if data, err := os.ReadFile(keyPath); err == nil {
		key := make([]byte, 32)
		n, err := base64.StdEncoding.Decode(key, data)
		if err == nil && n == 32 {
			return key, nil
		}
	}
	
	// Check for key in old location and migrate if found
	if dataDir != "/etc/pulse" {
		if data, err := os.ReadFile(oldKeyPath); err == nil {
			key := make([]byte, 32)
			n, err := base64.StdEncoding.Decode(key, data)
			if err == nil && n == 32 {
				// Migrate key to new location
				if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err == nil {
					if err := os.WriteFile(keyPath, data, 0600); err == nil {
						log.Info().Msg("Migrated encryption key to data directory")
						// Try to remove old key (ignore errors)
						os.Remove(oldKeyPath)
					}
				}
				return key, nil
			}
		}
	}
	
	// Generate new key
	key := make([]byte, 32) // AES-256
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}
	
	// Save key with restricted permissions
	encoded := base64.StdEncoding.EncodeToString(key)
	if err := os.WriteFile(keyPath, []byte(encoded), 0600); err != nil {
		return nil, fmt.Errorf("failed to save key: %w", err)
	}
	
	log.Info().Msg("Generated new encryption key")
	return key, nil
}

// Encrypt encrypts data using AES-GCM
func (c *CryptoManager) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts data using AES-GCM
func (c *CryptoManager) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	
	return plaintext, nil
}

// EncryptString encrypts a string and returns base64
func (c *CryptoManager) EncryptString(plaintext string) (string, error) {
	encrypted, err := c.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// DecryptString decrypts a base64 string
func (c *CryptoManager) DecryptString(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	
	decrypted, err := c.Decrypt(data)
	if err != nil {
		return "", err
	}
	
	return string(decrypted), nil
}

// HashPassword creates a SHA256 hash of a password for comparison
func HashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return base64.StdEncoding.EncodeToString(hash[:])
}