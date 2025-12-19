package license

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	// LicenseFileName is the name of the encrypted license file.
	LicenseFileName = "license.enc"
)

// Persistence handles encrypted storage of license keys.
type Persistence struct {
	configDir string
	machineID string // Used as encryption key material
}

// NewPersistence creates a new license persistence handler.
func NewPersistence(configDir string) (*Persistence, error) {
	machineID, err := getMachineID()
	if err != nil {
		// Fall back to a fixed key for development
		machineID = "pulse-dev-fallback-machine-id"
	}

	return &Persistence{
		configDir: configDir,
		machineID: machineID,
	}, nil
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
		return "", err
	}
	return persisted.LicenseKey, nil
}

// LoadWithMetadata reads and decrypts a license with metadata from disk.
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

	decrypted, err := p.decrypt(encrypted)
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

// decrypt uses AES-GCM to decrypt data.
func (p *Persistence) decrypt(ciphertext []byte) ([]byte, error) {
	key := p.deriveKey()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// deriveKey derives a 32-byte key from the machine ID.
func (p *Persistence) deriveKey() []byte {
	hash := sha256.Sum256([]byte("pulse-license-" + p.machineID))
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
