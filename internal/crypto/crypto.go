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

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/hkdf"
)

var defaultDataDirFn = utils.GetDataDir

var legacyKeyPath = "/etc/pulse/.encryption.key"

var randReader = rand.Reader

var newCipher = aes.NewCipher

var newGCM = cipher.NewGCM

// CryptoManager handles encryption/decryption of sensitive data
type CryptoManager struct {
	key     []byte
	keyPath string // Path to the encryption key file for runtime validation
}

// DeriveKey derives a purpose-specific key from the master encryption key using HKDF-SHA256.
// This avoids reusing the raw encryption key across unrelated cryptographic contexts.
func (c *CryptoManager) DeriveKey(purpose string, length int) ([]byte, error) {
	if c == nil || len(c.key) == 0 {
		return nil, fmt.Errorf("crypto manager not initialized")
	}
	if length <= 0 {
		return nil, fmt.Errorf("invalid derived key length: %d", length)
	}
	if purpose == "" {
		return nil, fmt.Errorf("purpose is required")
	}

	out := make([]byte, length)
	hkdfReader := hkdf.New(sha256.New, c.key, nil, []byte(purpose))
	if _, err := io.ReadFull(hkdfReader, out); err != nil {
		return nil, fmt.Errorf("hkdf read: %w", err)
	}
	return out, nil
}

// NewCryptoManagerAt creates a new crypto manager with an explicit data directory override.
func NewCryptoManagerAt(dataDir string) (*CryptoManager, error) {
	if dataDir == "" {
		dataDir = defaultDataDirFn()
	}
	keyPath := filepath.Join(dataDir, ".encryption.key")

	key, err := getOrCreateKeyAt(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
	}

	return &CryptoManager{
		key:     key,
		keyPath: keyPath,
	}, nil
}

// getOrCreateKeyAt gets the encryption key or creates one if it doesn't exist
func getOrCreateKeyAt(dataDir string) ([]byte, error) {
	if dataDir == "" {
		dataDir = defaultDataDirFn()
	}

	keyPath := filepath.Join(dataDir, ".encryption.key")
	oldKeyPath := legacyKeyPath
	// Test/ops hook: allow overriding the legacy key location to avoid touching /etc/pulse in unit tests.
	// This is only used during migration checks and has no effect unless explicitly set.
	if v := os.Getenv("PULSE_LEGACY_KEY_PATH"); v != "" {
		oldKeyPath = v
	}
	oldKeyDir := filepath.Dir(oldKeyPath)

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
				Str("keyPath", keyPath).
				Msg("Encryption key has invalid length (expected 32 bytes)")
		} else {
			log.Warn().
				Err(err).
				Str("keyPath", keyPath).
				Msg("Failed to decode encryption key")
		}
	} else {
		log.Debug().Err(err).Str("path", keyPath).Msg("Could not read encryption key file")
	}

	// Check for key in old location and migrate if found (only if paths differ)
	// CRITICAL: This code deletes the encryption key at oldKeyPath after migrating it.
	// Adding extensive logging to diagnose recurring key deletion bug.
	if dataDir != oldKeyDir && keyPath != oldKeyPath {
		log.Warn().
			Str("dataDir", dataDir).
			Str("keyPath", keyPath).
			Str("oldKeyPath", oldKeyPath).
			Msg("ENCRYPTION KEY MIGRATION: Checking if old key exists for migration (this code path CAN delete the key!)")

		if data, err := os.ReadFile(oldKeyPath); err == nil {
			decoded := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
			n, err := base64.StdEncoding.Decode(decoded, data)
			if err == nil && n == 32 {
				key := decoded[:n]
				// Migrate key to new location
				if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
					// Migration failed, but we can still use the old key
					log.Warn().
						Err(err).
						Str("from", oldKeyPath).
						Str("to", keyPath).
						Msg("Failed to create directory for key migration, using old location")
					return key, nil
				}
				if err := os.WriteFile(keyPath, data, 0600); err != nil {
					// Migration failed, but we can still use the old key
					log.Warn().
						Err(err).
						Str("from", oldKeyPath).
						Str("to", keyPath).
						Msg("Failed to migrate encryption key, using old location")
					return key, nil
				}
				log.Info().
					Str("from", oldKeyPath).
					Str("to", keyPath).
					Msg("Successfully migrated encryption key to data directory")

				// CRITICAL: This is the ONLY place in the codebase that deletes the encryption key!
				// BUG FIX: Disabling key deletion to prevent key loss.
				// Keeping both copies is safe - the old key at /etc/pulse will just be unused.
				log.Info().
					Str("oldKeyPath", oldKeyPath).
					Str("newKeyPath", keyPath).
					Str("dataDir", dataDir).
					Msg("Key migration complete - PRESERVING old key at original location for safety")

				// DISABLED: Key deletion was causing mysterious key loss bugs.
				// The old key is now preserved. This is safe because:
				// 1. We just successfully wrote the key to the new location
				// 2. Future reads will use the new location (checked first)
				// 3. Keeping the backup prevents data loss if something goes wrong
				//
				// if err := os.Remove(oldKeyPath); err != nil {
				// 	log.Debug().Err(err).Msg("Could not remove old encryption key (may lack permissions)")
				// } else {
				// 	log.Error().
				// 		Str("deletedPath", oldKeyPath).
				// 		Msg("CRITICAL: ENCRYPTION KEY HAS BEEN DELETED")
				// }
				return key, nil
			}
		}
	} else {
		log.Debug().
			Str("dataDir", dataDir).
			Str("keyPath", keyPath).
			Bool("sameAsOldPath", dataDir == oldKeyDir).
			Msg("Skipping key migration check (dataDir is /etc/pulse or paths match)")
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
	if _, err := io.ReadFull(randReader, key); err != nil {
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

	log.Info().Str("keyPath", keyPath).Msg("Generated new encryption key")
	return key, nil
}

// Encrypt encrypts data using AES-GCM
// SAFETY: Verifies the encryption key file still exists on disk before encrypting.
// This prevents orphaned encrypted data if the key was deleted while Pulse was running.
func (c *CryptoManager) Encrypt(plaintext []byte) ([]byte, error) {
	// CRITICAL: Verify the key file still exists on disk before encrypting
	// This prevents creating orphaned encrypted data that can never be decrypted
	if c.keyPath != "" {
		if _, err := os.Stat(c.keyPath); os.IsNotExist(err) {
			log.Error().
				Str("keyPath", c.keyPath).
				Msg("CRITICAL: Encryption key file has been deleted - refusing to encrypt to prevent orphaned data")
			return nil, fmt.Errorf("encryption key file deleted - cannot encrypt (would create orphaned data)")
		}
	}

	block, err := newCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("crypto.Encrypt: create AES cipher: %w", err)
	}

	gcm, err := newGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto.Encrypt: create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(randReader, nonce); err != nil {
		return nil, fmt.Errorf("crypto.Encrypt: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts data using AES-GCM
func (c *CryptoManager) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := newCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("crypto.Decrypt: create AES cipher: %w", err)
	}

	gcm, err := newGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto.Decrypt: create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto.Decrypt: open ciphertext: %w", err)
	}

	return plaintext, nil
}

// EncryptString encrypts a string and returns base64
func (c *CryptoManager) EncryptString(plaintext string) (string, error) {
	encrypted, err := c.Encrypt([]byte(plaintext))
	if err != nil {
		return "", fmt.Errorf("crypto.EncryptString: encrypt bytes: %w", err)
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// DecryptString decrypts a base64 string
func (c *CryptoManager) DecryptString(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("crypto.DecryptString: decode base64: %w", err)
	}

	decrypted, err := c.Decrypt(data)
	if err != nil {
		return "", fmt.Errorf("crypto.DecryptString: decrypt bytes: %w", err)
	}

	return string(decrypted), nil
}

// Note: Password hashing has been moved to the auth package
// which uses bcrypt for secure password hashing.
// Never use SHA256 or other fast hashes for passwords!
