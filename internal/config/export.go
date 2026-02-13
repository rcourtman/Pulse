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
	"github.com/rs/zerolog/log"
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
	APITokens     []APITokenRecord              `json:"apiTokens,omitempty"`
}

// ExportConfig exports all configuration with passphrase-based encryption
func (c *ConfigPersistence) ExportConfig(passphrase string) (string, error) {
	if passphrase == "" {
		return "", fmt.Errorf("passphrase is required for export")
	}

	// Each Load function handles its own locking. Do NOT hold an outer lock
	// here: LoadNodesConfig may need to acquire a write lock to persist
	// migrations, which would deadlock against an outer read lock.
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

	apiTokens, err := c.LoadAPITokens()
	if err != nil {
		return "", fmt.Errorf("failed to load api tokens: %w", err)
	}
	if apiTokens == nil {
		apiTokens = []APITokenRecord{}
	}

	// Load guest metadata from the active persistence scope.
	guestMetadata := c.GetGuestMetadataStore().GetAll()

	// Create export data
	exportData := ExportData{
		Version:       "4.1",
		ExportedAt:    time.Now(),
		Nodes:         *nodes,
		Alerts:        *alertConfig,
		Email:         *emailConfig,
		Webhooks:      webhooks,
		Apprise:       *appriseConfig,
		System:        *systemSettings,
		GuestMetadata: guestMetadata,
		OIDC:          oidcConfig,
		APITokens:     apiTokens,
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
	switch exportData.Version {
	case "4.1", "":
		// current version, nothing to do
	case "4.0":
		log.Info().Msg("Config was exported from version 4.0. API tokens were not included in that format.")
	default:
		log.Warn().Str("version", exportData.Version).Msg("Config was exported from unsupported version. Proceeding with best effort.")
	}

	tx, err := newImportTransaction(c.configDir)
	if err != nil {
		return fmt.Errorf("failed to start import transaction: %w", err)
	}
	defer tx.Cleanup()

	c.beginTransaction(tx)
	defer c.endTransaction(tx)

	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

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

	// Import API tokens for newer export formats
	if exportData.Version == "4.1" {
		if exportData.APITokens == nil {
			exportData.APITokens = []APITokenRecord{}
		}
		if err := c.SaveAPITokens(exportData.APITokens); err != nil {
			return fmt.Errorf("failed to import api tokens: %w", err)
		}
	}

	// Import OIDC configuration
	if exportData.OIDC != nil {
		if err := c.SaveOIDCConfig(*exportData.OIDC); err != nil {
			return fmt.Errorf("failed to import oidc configuration: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit import transaction: %w", err)
	}
	committed = true

	if exportData.OIDC == nil {
		// Remove existing OIDC config if backup did not include one
		if err := c.fs.Remove(c.oidcFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove existing oidc configuration: %w", err)
		}
	}

	// Import guest metadata into the active persistence scope.
	if err := c.GetGuestMetadataStore().ReplaceAll(exportData.GuestMetadata); err != nil {
		log.Warn().Err(err).Msg("Failed to import guest metadata")
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
