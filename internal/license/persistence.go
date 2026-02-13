package license

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// LicenseFileName is the name of the encrypted license file.
	LicenseFileName = "license.enc"
	// PersistentKeyFileName is the name of the persistent encryption key file.
	// This file is stored in the config directory (e.g., /data/) and survives
	// Docker container recreation, unlike /etc/machine-id.
	PersistentKeyFileName = ".license-key"
)

// Persistence handles encrypted storage of license keys.
type Persistence struct {
	configDir     string
	encryptionKey string // Primary key for encryption (persistent or machine-id)
	machineID     string // Fallback for backwards compatibility
}

// NewPersistence creates a new license persistence handler.
// It tries to use a persistent key stored in configDir first, then falls back
// to machine-id for backwards compatibility with existing installations.
func NewPersistence(configDir string) (*Persistence, error) {
	// Try to load persistent key from config directory first
	persistentKey, err := loadPersistentKey(configDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load persistent license key: %w", err)
	}

	// Get machine-id as fallback for backwards compatibility
	machineID, err := getMachineID()
	if err != nil {
		machineID = "pulse-dev-fallback-machine-id"
	}

	// Use persistent key if available, otherwise machine-id
	encryptionKey := persistentKey
	if encryptionKey == "" {
		encryptionKey = machineID
	}

	return &Persistence{
		configDir:     configDir,
		encryptionKey: encryptionKey,
		machineID:     machineID,
	}, nil
}

// loadPersistentKey attempts to load the persistent encryption key from disk.
func loadPersistentKey(configDir string) (string, error) {
	keyPath := filepath.Join(configDir, PersistentKeyFileName)
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// ensurePersistentKey creates a persistent encryption key if one doesn't exist.
// Returns the key (existing or newly created).
func (p *Persistence) ensurePersistentKey() (string, error) {
	keyPath := filepath.Join(p.configDir, PersistentKeyFileName)

	// Check if key already exists
	if data, err := os.ReadFile(keyPath); err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	// Generate a new random key (32 bytes = 64 hex chars)
	keyBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, keyBytes); err != nil {
		return "", fmt.Errorf("failed to generate encryption key: %w", err)
	}
	key := hex.EncodeToString(keyBytes)

	// Ensure config directory exists
	if err := os.MkdirAll(p.configDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	// Save the key
	if err := os.WriteFile(keyPath, []byte(key), 0600); err != nil {
		return "", fmt.Errorf("failed to write encryption key: %w", err)
	}

	return key, nil
}

// PersistedLicense contains the license key and metadata for storage.
type PersistedLicense struct {
	LicenseKey     string `json:"license_key"`
	GracePeriodEnd *int64 `json:"grace_period_end,omitempty"` // Unix timestamp
}

// Save encrypts and saves a license key to disk.
func (p *Persistence) Save(licenseKey string) error {
	return p.SaveWithGracePeriod(licenseKey, nil)
}

// SaveWithGracePeriod encrypts and saves a license with optional grace period.
func (p *Persistence) SaveWithGracePeriod(licenseKey string, gracePeriodEnd *int64) error {
	if licenseKey == "" {
		return errors.New("license key cannot be empty")
	}

	// Ensure we have a persistent encryption key for future-proofing.
	// This creates .license-key in the config directory if it doesn't exist,
	// ensuring Docker users don't lose their license on container recreation.
	newKey, err := p.ensurePersistentKey()
	if err == nil && newKey != "" {
		p.encryptionKey = newKey
	}

	// Store as JSON with metadata
	persisted := PersistedLicense{
		LicenseKey:     licenseKey,
		GracePeriodEnd: gracePeriodEnd,
	}
	jsonData, err := json.Marshal(persisted)
	if err != nil {
		return fmt.Errorf("failed to marshal license: %w", err)
	}

	encrypted, err := p.encrypt(jsonData)
	if err != nil {
		return fmt.Errorf("failed to encrypt license: %w", err)
	}

	// Ensure config directory exists
	if err := os.MkdirAll(p.configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	licensePath := filepath.Join(p.configDir, LicenseFileName)
	encoded := base64.StdEncoding.EncodeToString(encrypted)

	if err := os.WriteFile(licensePath, []byte(encoded), 0600); err != nil {
		return fmt.Errorf("failed to write license file: %w", err)
	}

	return nil
}

// Load reads and decrypts a license key from disk.
func (p *Persistence) Load() (string, error) {
	persisted, err := p.LoadWithMetadata()
	if err != nil {
		return "", fmt.Errorf("load license metadata: %w", err)
	}
	return persisted.LicenseKey, nil
}

// LoadWithMetadata reads and decrypts a license with metadata from disk.
// It tries to decrypt with the current encryption key first, then falls back
// to machine-id for backwards compatibility with existing installations.
func (p *Persistence) LoadWithMetadata() (PersistedLicense, error) {
	licensePath := filepath.Join(p.configDir, LicenseFileName)

	encoded, err := os.ReadFile(licensePath)
	if err != nil {
		if os.IsNotExist(err) {
			return PersistedLicense{}, nil // No license saved
		}
		return PersistedLicense{}, fmt.Errorf("failed to read license file: %w", err)
	}

	encrypted, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return PersistedLicense{}, fmt.Errorf("failed to decode license file: %w", err)
	}

	// Try to decrypt with current encryption key
	decrypted, err := p.decrypt(encrypted)

	// If decryption failed and we have a different machine-id, try that as fallback
	// This handles the case where an existing license was encrypted with machine-id
	// before the persistent key feature was added
	if err != nil && p.machineID != p.encryptionKey {
		decrypted, err = p.decryptWithKey(encrypted, p.deriveKeyFrom(p.machineID))
	}

	if err != nil {
		return PersistedLicense{}, fmt.Errorf("failed to decrypt license: %w", err)
	}

	// Try to parse as new JSON format
	var persisted PersistedLicense
	if err := json.Unmarshal(decrypted, &persisted); err != nil {
		// Fall back to old format (just raw key)
		persisted.LicenseKey = string(decrypted)
	}

	return persisted, nil
}

// Delete removes the saved license file.
func (p *Persistence) Delete() error {
	licensePath := filepath.Join(p.configDir, LicenseFileName)
	err := os.Remove(licensePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete license file: %w", err)
	}
	return nil
}

// Exists checks if a saved license exists.
func (p *Persistence) Exists() bool {
	licensePath := filepath.Join(p.configDir, LicenseFileName)
	_, err := os.Stat(licensePath)
	return err == nil
}

// encrypt uses AES-GCM to encrypt data.
func (p *Persistence) encrypt(plaintext []byte) ([]byte, error) {
	key := p.deriveKey()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decrypt uses AES-GCM to decrypt data with the current encryption key.
func (p *Persistence) decrypt(ciphertext []byte) ([]byte, error) {
	return p.decryptWithKey(ciphertext, p.deriveKey())
}

// decryptWithKey uses AES-GCM to decrypt data with a specific key.
func (p *Persistence) decryptWithKey(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short: got %d bytes, need at least %d", len(ciphertext), gcm.NonceSize())
	}

	nonce := ciphertext[:gcm.NonceSize()]
	data := ciphertext[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt ciphertext: %w", err)
	}

	return plaintext, nil
}

// deriveKey derives a 32-byte key from the current encryption key.
func (p *Persistence) deriveKey() []byte {
	return p.deriveKeyFrom(p.encryptionKey)
}

// deriveKeyFrom derives a 32-byte key from a given key material.
func (p *Persistence) deriveKeyFrom(keyMaterial string) []byte {
	hash := sha256.Sum256([]byte("pulse-license-" + keyMaterial))
	return hash[:]
}

// getMachineID attempts to get a stable machine identifier.
func getMachineID() (string, error) {
	// Try Linux machine-id
	paths := []string{
		"/etc/machine-id",
		"/var/lib/dbus/machine-id",
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			return string(data), nil
		}
	}

	// Try hostname as fallback
	hostname, err := os.Hostname()
	if err == nil {
		return hostname, nil
	}

	return "", errors.New("could not determine machine ID")
}
