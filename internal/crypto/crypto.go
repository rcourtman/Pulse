package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/hkdf"
)

var defaultDataDirFn = utils.GetDataDir

var legacyKeyPath = "/etc/pulse/.encryption.key"

var randReader = rand.Reader

var newCipher = aes.NewCipher

var newGCM = cipher.NewGCM

const (
	encryptionKeyFileName    = ".encryption.key"
	encryptionKeyLength      = 32 // AES-256 key length in bytes
	encryptionKeyFilePerm    = 0o600
	encryptionKeyDirPerm     = 0o700
	maxEncryptionKeyFileSize = 4096
)

var errInvalidKeyMaterial = errors.New("invalid encryption key material")
var errUnsafeKeyPath = errors.New("unsafe encryption key path")

func ensureOwnerOnlyDir(dir string) error {
	if err := os.MkdirAll(dir, encryptionKeyDirPerm); err != nil {
		return err
	}
	return os.Chmod(dir, encryptionKeyDirPerm)
}

func decodeEncryptionKey(data []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(data)
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(trimmed)))
	n, err := base64.StdEncoding.Decode(decoded, trimmed)
	if err != nil {
		return nil, fmt.Errorf("%w: decode base64: %v", errInvalidKeyMaterial, err)
	}
	if n != encryptionKeyLength {
		return nil, fmt.Errorf("%w: decoded %d bytes, expected %d", errInvalidKeyMaterial, n, encryptionKeyLength)
	}
	return decoded[:n], nil
}

func validateEncryptionKeyFile(path string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: refusing symlink key path %q", errUnsafeKeyPath, path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: non-regular key path %q", errInvalidKeyMaterial, path)
	}
	if info.Size() > maxEncryptionKeyFileSize {
		return fmt.Errorf("%w: key file %q is too large (%d bytes)", errUnsafeKeyPath, path, info.Size())
	}
	return nil
}

func loadKeyFromFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if err := validateEncryptionKeyFile(path, info); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) > maxEncryptionKeyFileSize {
		return nil, fmt.Errorf("%w: key file %q exceeded size limit while reading", errUnsafeKeyPath, path)
	}
	return decodeEncryptionKey(data)
}

func isMissingPathError(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

func writeKeyFile(path string, key []byte) error {
	if len(key) != encryptionKeyLength {
		return fmt.Errorf("refusing to write invalid key length %d", len(key))
	}

	dir := filepath.Dir(path)
	if err := ensureOwnerOnlyDir(dir); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(dir, ".encryption.key.*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmpFile.Chmod(encryptionKeyFilePerm); err != nil {
		_ = tmpFile.Close()
		return err
	}

	encoded := base64.StdEncoding.EncodeToString(key)
	if _, err := tmpFile.WriteString(encoded); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	cleanup = false

	return os.Chmod(path, encryptionKeyFilePerm)
}

// CryptoManager handles encryption/decryption of sensitive data
type CryptoManager struct {
	key     []byte
	keyPath string // Path to the encryption key file for runtime validation
}

func resolveDataDir(dataDir string) (string, error) {
	dir := strings.TrimSpace(dataDir)
	if dir == "" {
		dir = strings.TrimSpace(defaultDataDirFn())
	}
	if dir == "" {
		return "", fmt.Errorf("data directory is required")
	}
	return filepath.Clean(dir), nil
}

func resolveLegacyKeyPath() string {
	oldKeyPath := legacyKeyPath
	if v, ok := os.LookupEnv("PULSE_LEGACY_KEY_PATH"); ok {
		trimmed := strings.TrimSpace(v)
		switch {
		case trimmed == "":
			return oldKeyPath
		case !filepath.IsAbs(trimmed):
			log.Warn().
				Str("legacyKeyPath", trimmed).
				Msg("Ignoring non-absolute PULSE_LEGACY_KEY_PATH override")
		default:
			oldKeyPath = filepath.Clean(trimmed)
		}
	}
	return oldKeyPath
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
	resolvedDataDir, err := resolveDataDir(dataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data directory: %w", err)
	}
	keyPath := filepath.Join(resolvedDataDir, ".encryption.key")

	key, err := getOrCreateKeyAt(resolvedDataDir)
	if err != nil {
		return nil, fmt.Errorf("crypto.NewCryptoManagerAt: get or create encryption key at %q: %w", keyPath, err)
	}

	return &CryptoManager{
		key:     key,
		keyPath: keyPath,
	}, nil
}

// getOrCreateKeyAt gets the encryption key or creates one if it doesn't exist
func getOrCreateKeyAt(dataDir string) ([]byte, error) {
	resolvedDataDir, err := resolveDataDir(dataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data directory: %w", err)
	}

	keyPath := filepath.Join(dataDir, encryptionKeyFileName)
	// Test/ops hook: allow overriding the legacy key location to avoid touching /etc/pulse in unit tests.
	// Invalid overrides are ignored to avoid accidentally reading from relative CWD paths.
	oldKeyPath := resolveLegacyKeyPath()
	oldKeyDir := filepath.Dir(oldKeyPath)

	log.Debug().
		Str("dataDir", resolvedDataDir).
		Str("keyPath", keyPath).
		Msg("looking for encryption key")

	var keyReadErr error

	// Try to read existing key from new location.
	if key, err := loadKeyFromFile(keyPath); err == nil {
		if err := ensureOwnerOnlyDir(filepath.Dir(keyPath)); err != nil {
			return nil, fmt.Errorf("failed to harden encryption key directory: %w", err)
		}
		if err := os.Chmod(keyPath, encryptionKeyFilePerm); err != nil {
			return nil, fmt.Errorf("failed to harden encryption key file: %w", err)
		}
		log.Debug().Msg("Found and loaded existing encryption key")
		return key, nil
	} else if isMissingPathError(err) {
		log.Debug().Err(err).Str("path", keyPath).Msg("Could not read encryption key file")
	} else if errors.Is(err, errInvalidKeyMaterial) {
		log.Warn().
			Err(err).
			Str("path", keyPath).
			Msg("Found invalid encryption key file contents, generating a replacement key")
	} else if errors.Is(err, errUnsafeKeyPath) {
		return nil, fmt.Errorf("unsafe encryption key path %q: %w", keyPath, err)
	} else {
		return nil, fmt.Errorf("failed to read encryption key file %q: %w", keyPath, err)
	}

	// Check for key in old location and migrate if found (only if paths differ)
	// CRITICAL: This code deletes the encryption key at oldKeyPath after migrating it.
	// Adding extensive logging to diagnose recurring key deletion bug.
	if resolvedDataDir != oldKeyDir && keyPath != oldKeyPath {
		log.Warn().
			Str("dataDir", resolvedDataDir).
			Str("keyPath", keyPath).
			Str("oldKeyPath", oldKeyPath).
			Msg("checking for legacy encryption key migration")

		if data, err := os.ReadFile(oldKeyPath); err == nil {
			decoded := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
			n, decodeErr := base64.StdEncoding.Decode(decoded, data)
			if decodeErr != nil {
				log.Warn().
					Err(decodeErr).
					Str("path", oldKeyPath).
					Msg("Failed to decode legacy encryption key during migration check")
			} else if n != 32 {
				log.Warn().
					Int("decodedBytes", n).
					Str("path", oldKeyPath).
					Msg("Legacy encryption key has invalid length during migration check")
			} else {
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
					Msg("migrated encryption key to data directory")

				// CRITICAL: This is the ONLY place in the codebase that deletes the encryption key!
				// BUG FIX: Disabling key deletion to prevent key loss.
				// Keeping both copies is safe - the old key at /etc/pulse will just be unused.
				log.Info().
					Str("oldKeyPath", oldKeyPath).
					Str("newKeyPath", keyPath).
					Str("dataDir", resolvedDataDir).
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
		} else if !os.IsNotExist(err) {
			log.Warn().
				Err(err).
				Str("path", oldKeyPath).
				Msg("Failed to read legacy encryption key during migration check")
		}
	} else {
		log.Debug().
			Str("dataDir", resolvedDataDir).
			Str("keyPath", keyPath).
			Bool("sameAsOldPath", dataDir == oldKeyDir).
			Msg("skipping key migration check (legacy and current paths are equivalent)")
	}

	if keyReadErr != nil {
		// Avoid generating a replacement key when an existing key path is unreadable;
		// callers should resolve filesystem issues first to prevent accidental key drift.
		return nil, keyReadErr
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

	var foundFiles []string
	for _, pattern := range checkPatterns {
		globPattern := filepath.Join(dataDir, pattern)
		matches, err := filepath.Glob(globPattern)
		if err != nil {
			return nil, fmt.Errorf("crypto.getOrCreateKeyAt: glob encrypted-data pattern %q: %w", globPattern, err)
		}
		for _, file := range matches {
			info, err := os.Stat(file)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("crypto.getOrCreateKeyAt: stat encrypted-data candidate %q: %w", file, err)
			}
			if info.Size() > 0 {
				foundFiles = append(foundFiles, filepath.Base(file))
			}
		}
	}

	if len(foundFiles) > 0 {
		log.Error().
			Strs("foundFiles", foundFiles).
			Str("dataDir", resolvedDataDir).
			Msg("CRITICAL: Encryption key not found but encrypted/backup/corrupted files exist")
		return nil, fmt.Errorf("encryption key not found but encrypted data exists (%v) - cannot generate new key as it would orphan existing data. Please restore the encryption key from backup or delete ALL .enc* files to start fresh", foundFiles)
	}

	// Generate new key (only if no encrypted data exists)
	key := make([]byte, encryptionKeyLength) // AES-256
	if _, err := io.ReadFull(randReader, key); err != nil {
		return nil, fmt.Errorf("crypto.getOrCreateKeyAt: generate key bytes: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return nil, fmt.Errorf("crypto.getOrCreateKeyAt: create key directory %q: %w", filepath.Dir(keyPath), err)
	}

	// Save key with restricted permissions
	encoded := base64.StdEncoding.EncodeToString(key)
	if err := os.WriteFile(keyPath, []byte(encoded), 0600); err != nil {
		return nil, fmt.Errorf("crypto.getOrCreateKeyAt: save key file %q: %w", keyPath, err)
	}

	log.Info().
		Str("keyPath", keyPath).
		Msg("generated new encryption key")
	return key, nil
}

// newAEAD creates an AES-GCM cipher.AEAD from the manager's key.
func (c *CryptoManager) newAEAD() (cipher.AEAD, error) {
	block, err := newCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := newGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	return gcm, nil
}

// Encrypt encrypts data using AES-GCM
// SAFETY: Verifies the encryption key file still exists on disk before encrypting.
// This prevents orphaned encrypted data if the key was deleted while Pulse was running.
func (c *CryptoManager) Encrypt(plaintext []byte) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("crypto.Encrypt: crypto manager not initialized")
	}

	// CRITICAL: Verify the key file still exists on disk, is not a symlink,
	// and contains the same key material we loaded at startup.
	// This prevents creating orphaned encrypted data that can never be decrypted.
	if c.keyPath != "" {
		diskKey, loadErr := loadKeyFromFile(c.keyPath)
		if loadErr != nil {
			if os.IsNotExist(loadErr) {
				log.Error().
					Str("keyPath", c.keyPath).
					Msg("CRITICAL: Encryption key file has been deleted - refusing to encrypt to prevent orphaned data")
				return nil, fmt.Errorf("encryption key file deleted - cannot encrypt (would create orphaned data)")
			}
			return nil, fmt.Errorf("crypto.Encrypt: verify key file %q: %w", c.keyPath, loadErr)
		}
		if !bytes.Equal(diskKey, c.key) {
			return nil, fmt.Errorf("crypto.Encrypt: key file %q contents changed since load - refusing to encrypt", c.keyPath)
		}
	}

	gcm, err := c.newAEAD()
	if err != nil {
		return nil, fmt.Errorf("crypto.Encrypt: %w", err)
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
	if c == nil {
		return nil, fmt.Errorf("crypto.Decrypt: crypto manager not initialized")
	}

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
		return nil, fmt.Errorf("crypto.Decrypt: ciphertext too short: got %d bytes, need at least %d", len(ciphertext), nonceSize)
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
