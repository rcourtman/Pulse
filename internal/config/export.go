package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"golang.org/x/crypto/pbkdf2"
)

// ExportData contains all configuration data for export
type ExportData struct {
	Version       string                        `json:"version"`
	ExportedAt    time.Time                     `json:"exportedAt"`
	Nodes         NodesConfig                   `json:"nodes"`
	Alerts        alerts.AlertConfig            `json:"alerts"`
	Email         notifications.EmailConfig     `json:"email"`
	Webhooks      []notifications.WebhookConfig `json:"webhooks"`
	Apprise       notifications.AppriseConfig   `json:"apprise"`
	System        SystemSettings                `json:"system"`
	GuestMetadata map[string]*GuestMetadata     `json:"guestMetadata,omitempty"`
	OIDC          *OIDCConfig                   `json:"oidc,omitempty"`
}

// ExportConfig exports all configuration with passphrase-based encryption
func (c *ConfigPersistence) ExportConfig(passphrase string) (string, error) {
	if passphrase == "" {
		return "", fmt.Errorf("passphrase is required for export")
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Load all configurations
	nodes, err := c.LoadNodesConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load nodes config: %w", err)
	}

	alertConfig, err := c.LoadAlertConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load alert config: %w", err)
	}

	emailConfig, err := c.LoadEmailConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load email config: %w", err)
	}

	appriseConfig, err := c.LoadAppriseConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load Apprise config: %w", err)
	}

	webhooks, err := c.LoadWebhooks()
	if err != nil {
		return "", fmt.Errorf("failed to load webhooks: %w", err)
	}

	systemSettings, err := c.LoadSystemSettings()
	if err != nil {
		return "", fmt.Errorf("failed to load system settings: %w", err)
	}
	if systemSettings == nil {
		systemSettings = DefaultSystemSettings()
	}

	oidcConfig, err := c.LoadOIDCConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load oidc configuration: %w", err)
	}

	// Load guest metadata (stored in data directory)
	// Use PULSE_DATA_DIR if set, otherwise use /etc/pulse for backwards compatibility
	dataPath := os.Getenv("PULSE_DATA_DIR")
	if dataPath == "" {
		dataPath = "/etc/pulse"
	}
	guestMetadataStore := NewGuestMetadataStore(dataPath)
	guestMetadata := guestMetadataStore.GetAll()

	// Create export data
	exportData := ExportData{
		Version:       "4.0",
		ExportedAt:    time.Now(),
		Nodes:         *nodes,
		Alerts:        *alertConfig,
		Email:         *emailConfig,
		Webhooks:      webhooks,
		Apprise:       *appriseConfig,
		System:        *systemSettings,
		GuestMetadata: guestMetadata,
		OIDC:          oidcConfig,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(exportData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal export data: %w", err)
	}

	// Encrypt with passphrase
	encrypted, err := encryptWithPassphrase(jsonData, passphrase)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt export data: %w", err)
	}

	// Return base64 encoded
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// ImportConfig imports configuration from encrypted export
func (c *ConfigPersistence) ImportConfig(encryptedData string, passphrase string) error {
	if passphrase == "" {
		return fmt.Errorf("passphrase is required for import")
	}

	// Decode from base64
	encrypted, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return fmt.Errorf("failed to decode import data: %w", err)
	}

	// Decrypt with passphrase
	decrypted, err := decryptWithPassphrase(encrypted, passphrase)
	if err != nil {
		return fmt.Errorf("failed to decrypt import data: %w", err)
	}

	// Unmarshal JSON
	var exportData ExportData
	if err := json.Unmarshal(decrypted, &exportData); err != nil {
		return fmt.Errorf("failed to unmarshal import data: %w", err)
	}

	// Check version compatibility (warn but don't fail)
	if exportData.Version != "4.0" {
		// Log warning but continue - future versions might be compatible
		fmt.Printf("Warning: Config was exported from version %s, current version is 4.0\n", exportData.Version)
	}
	// Import all configurations
	if err := c.SaveNodesConfig(exportData.Nodes.PVEInstances, exportData.Nodes.PBSInstances, exportData.Nodes.PMGInstances); err != nil {
		return fmt.Errorf("failed to import nodes config: %w", err)
	}

	if err := c.SaveAlertConfig(exportData.Alerts); err != nil {
		return fmt.Errorf("failed to import alert config: %w", err)
	}

	if err := c.SaveEmailConfig(exportData.Email); err != nil {
		return fmt.Errorf("failed to import email config: %w", err)
	}

	if err := c.SaveAppriseConfig(exportData.Apprise); err != nil {
		return fmt.Errorf("failed to import Apprise config: %w", err)
	}

	if err := c.SaveWebhooks(exportData.Webhooks); err != nil {
		return fmt.Errorf("failed to import webhooks: %w", err)
	}

	if err := c.SaveSystemSettings(exportData.System); err != nil {
		return fmt.Errorf("failed to import system settings: %w", err)
	}

	// Import OIDC configuration
	if exportData.OIDC != nil {
		if err := c.SaveOIDCConfig(*exportData.OIDC); err != nil {
			return fmt.Errorf("failed to import oidc configuration: %w", err)
		}
	} else {
		// Remove existing OIDC config if backup did not include one
		if err := os.Remove(c.oidcFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove existing oidc configuration: %w", err)
		}
	}

	// Import guest metadata if present
	// Use PULSE_DATA_DIR if set, otherwise use /etc/pulse for backwards compatibility
	dataPath := os.Getenv("PULSE_DATA_DIR")
	if dataPath == "" {
		dataPath = "/etc/pulse"
	}
	guestMetadataStore := NewGuestMetadataStore(dataPath)
	if err := guestMetadataStore.ReplaceAll(exportData.GuestMetadata); err != nil {
		fmt.Printf("Warning: Failed to import guest metadata: %v\n", err)
	}

	return nil
}

// encryptWithPassphrase encrypts data using a passphrase-derived key
func encryptWithPassphrase(plaintext []byte, passphrase string) ([]byte, error) {
	// Generate salt
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	// Derive key from passphrase using PBKDF2
	key := pbkdf2.Key([]byte(passphrase), salt, 100000, 32, sha256.New)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Use GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Prepend salt to ciphertext
	result := make([]byte, len(salt)+len(ciphertext))
	copy(result, salt)
	copy(result[len(salt):], ciphertext)

	return result, nil
}

// decryptWithPassphrase decrypts data using a passphrase-derived key
func decryptWithPassphrase(ciphertext []byte, passphrase string) ([]byte, error) {
	if len(ciphertext) < 32 {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Extract salt
	salt := ciphertext[:32]
	ciphertext = ciphertext[32:]

	// Derive key from passphrase
	key := pbkdf2.Key([]byte(passphrase), salt, 100000, 32, sha256.New)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Use GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
