package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// CryptoManager handles encryption/decryption of sensitive data
type CryptoManager struct {
	key []byte
}

// NewCryptoManagerAt creates a new crypto manager with an explicit data directory override.
func NewCryptoManagerAt(dataDir string) (*CryptoManager, error) {
	key, err := getOrCreateKeyAt(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
	}

	return &CryptoManager{
		key: key,
	}, nil
}

// getOrCreateKeyAt gets the encryption key or creates one if it doesn't exist
func getOrCreateKeyAt(dataDir string) ([]byte, error) {
	if dataDir == "" {
		dataDir = utils.GetDataDir()
	}

	keyPath := filepath.Join(dataDir, ".encryption.key")
	oldKeyPath := "/etc/pulse/.encryption.key"

	log.Debug().
		Str("dataDir", dataDir).
		Str("keyPath", keyPath).
		Msg("Looking for encryption key")

	// Try to read existing key from new location
	if data, err := os.ReadFile(keyPath); err == nil {
		// Use DecodedLen to allocate sufficient space, then slice to actual length
		// This prevents panics if the file contains more data than expected
		decoded := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
		n, err := base64.StdEncoding.Decode(decoded, data)
		if err == nil {
			if n == 32 {
				log.Debug().Msg("Found and loaded existing encryption key")
				return decoded[:n], nil
			}
			log.Warn().
				Int("decodedBytes", n).
				Msg("Encryption key has invalid length (expected 32 bytes)")
		} else {
			log.Warn().
				Err(err).
				Msg("Failed to decode encryption key")
		}
	} else {
		log.Debug().Err(err).Str("path", keyPath).Msg("Could not read encryption key file")
	}

	// Check for key in old location and migrate if found (only if paths differ)
	if dataDir != "/etc/pulse" && keyPath != oldKeyPath {
		if data, err := os.ReadFile(oldKeyPath); err == nil {
			decoded := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
			n, err := base64.StdEncoding.Decode(decoded, data)
			if err == nil && n == 32 {
				key := decoded[:n]
				// Migrate key to new location
				if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
					// Migration failed, but we can still use the old key
					log.Warn().Err(err).Msg("Failed to create directory for key migration, using old location")
					return key, nil
				}
				if err := os.WriteFile(keyPath, data, 0600); err != nil {
					// Migration failed, but we can still use the old key
					log.Warn().Err(err).Msg("Failed to migrate encryption key, using old location")
					return key, nil
				}
				log.Info().
					Str("from", oldKeyPath).
					Str("to", keyPath).
					Msg("Successfully migrated encryption key to data directory")
				// Try to remove old key (ignore errors as this is cleanup)
				if err := os.Remove(oldKeyPath); err != nil {
					log.Debug().Err(err).Msg("Could not remove old encryption key (may lack permissions)")
				}
				return key, nil
			}
		}
	}

	// Before generating a new key, check if encrypted data exists OR if there are any backup/corrupted files
	// This prevents silently orphaning existing encrypted configurations
	// CRITICAL: Also check for .backup and .corrupted files to prevent data loss
	checkPatterns := []string{
		"nodes.enc*",
		"email.enc*",
		"webhooks.enc*",
		"oidc.enc*",
	}

	hasEncryptedData := false
	var foundFiles []string
	for _, pattern := range checkPatterns {
		matches, _ := filepath.Glob(filepath.Join(dataDir, pattern))
		for _, file := range matches {
			if info, err := os.Stat(file); err == nil && info.Size() > 0 {
				hasEncryptedData = true
				foundFiles = append(foundFiles, filepath.Base(file))
			}
		}
	}

	if hasEncryptedData {
		log.Error().
			Strs("foundFiles", foundFiles).
			Str("dataDir", dataDir).
			Msg("CRITICAL: Encryption key not found but encrypted/backup/corrupted files exist")
		return nil, fmt.Errorf("encryption key not found but encrypted data exists (%v) - cannot generate new key as it would orphan existing data. Please restore the encryption key from backup or delete ALL .enc* files to start fresh", foundFiles)
	}

	// Generate new key (only if no encrypted data exists)
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

// Note: Password hashing has been moved to the auth package
// which uses bcrypt for secure password hashing.
// Never use SHA256 or other fast hashes for passwords!
