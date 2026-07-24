package audit

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

// Signer handles HMAC-SHA256 signing and verification for audit events.
// The signing key is stored encrypted at rest using the provided crypto manager.
type Signer struct {
	key []byte // 32-byte HMAC signing key
}

// CryptoEncryptor interface for encrypting/decrypting the signing key.
// This matches the methods from internal/crypto.CryptoManager.
type CryptoEncryptor interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

// NewSigner creates a new signer, loading or generating the HMAC key.
// The key is stored encrypted in the data directory.
// If cryptoMgr is nil, signing will be disabled (returns empty signatures).
func NewSigner(dataDir string, cryptoMgr CryptoEncryptor) (*Signer, error) {
	if cryptoMgr == nil {
		log.Warn().Msg("Crypto manager not provided, audit signing disabled")
		return &Signer{key: nil}, nil
	}

	keyPath := filepath.Join(dataDir, ".audit-signing.key")

	// Try to load existing key
	if encryptedKey, err := os.ReadFile(keyPath); err == nil {
		key, migratedPlaintext, err := loadAuditSigningKey(cryptoMgr, encryptedKey)
		if err != nil {
			return nil, err
		}
		if len(key) < 32 {
			return nil, fmt.Errorf("invalid audit signing key length: got %d, want at least 32", len(key))
		}
		if migratedPlaintext {
			rewritten, err := cryptoMgr.Encrypt(key)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt migrated audit signing key: %w", err)
			}
			if err := os.WriteFile(keyPath, rewritten, 0600); err != nil {
				return nil, fmt.Errorf("failed to rewrite audit signing key: %w", err)
			}
		}
		log.Debug().Msg("Loaded existing audit signing key")
		return &Signer{key: key}, nil
	}

	// Generate new key
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate audit signing key: %w", err)
	}

	// Encrypt and save
	encryptedKey, err := cryptoMgr.Encrypt(key)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt audit signing key: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return nil, fmt.Errorf("failed to create directory for audit signing key: %w", err)
	}

	if err := os.WriteFile(keyPath, encryptedKey, 0600); err != nil {
		return nil, fmt.Errorf("failed to save audit signing key: %w", err)
	}

	log.Info().Msg("Generated new audit signing key")
	return &Signer{key: key}, nil
}

// NewSignerWithKey creates a signer backed by externally managed key material.
func NewSignerWithKey(key []byte) (*Signer, error) {
	if len(key) < 32 {
		return nil, fmt.Errorf("invalid audit signing key length: got %d, want at least 32", len(key))
	}
	return &Signer{key: append([]byte(nil), key...)}, nil
}

func loadAuditSigningKey(cryptoMgr CryptoEncryptor, data []byte) ([]byte, bool, error) {
	key, err := cryptoMgr.Decrypt(data)
	if err == nil {
		return key, false, nil
	}
	plaintext := bytes.TrimSpace(data)
	if len(plaintext) == 32 {
		return append([]byte(nil), plaintext...), true, nil
	}
	if len(plaintext) == 64 {
		if _, decodeErr := hex.DecodeString(string(plaintext)); decodeErr == nil {
			// The former Pro store used the printable hex value itself as the
			// HMAC key. Preserve those bytes while encrypting the file in place.
			return append([]byte(nil), plaintext...), true, nil
		}
	}
	return nil, false, fmt.Errorf("failed to decrypt audit signing key: %w", err)
}

// Sign computes an HMAC-SHA256 signature over the event's canonical form.
// Returns hex-encoded signature, or empty string if signing is disabled.
func (s *Signer) Sign(event Event) string {
	if s.key == nil {
		return ""
	}

	canonical := s.canonicalForm(event)
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks if the event's signature matches its content.
// Returns true if the signature is valid, false if invalid or signing is disabled.
func (s *Signer) Verify(event Event) bool {
	if s.key == nil || event.Signature == "" {
		return false
	}

	for _, canonical := range []string{
		s.canonicalForm(event),
		s.legacyUnixCanonicalForm(event),
		s.legacyTimeCanonicalForm(event),
	} {
		expected := s.signCanonical(canonical)
		if hmac.Equal([]byte(expected), []byte(event.Signature)) {
			return true
		}
	}
	return false
}

func (s *Signer) signCanonical(canonical string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

// canonicalForm creates a deterministic string representation of an event for signing.
// Format: ID|Timestamp(Unix)|EventType|User|IP|Path|Success(0/1)|Details
func (s *Signer) canonicalForm(event Event) string {
	success := "0"
	if event.Success {
		success = "1"
	}

	return event.ID + "|" +
		strconv.FormatInt(event.Timestamp.Unix(), 10) + "|" +
		event.EventType + "|" +
		event.User + "|" +
		event.IP + "|" +
		event.Path + "|" +
		success + "|" +
		event.Details
}

func (s *Signer) legacyUnixCanonicalForm(event Event) string {
	return event.ID + "|" +
		strconv.FormatInt(event.Timestamp.Unix(), 10) + "|" +
		event.EventType + "|" +
		event.User + "|" +
		event.IP + "|" +
		event.Path + "|" +
		strconv.FormatBool(event.Success) + "|" +
		event.Details
}

func (s *Signer) legacyTimeCanonicalForm(event Event) string {
	return event.ID + "|" +
		event.Timestamp.UTC().Format(time.RFC3339Nano) + "|" +
		event.EventType + "|" +
		event.User + "|" +
		event.IP + "|" +
		event.Path + "|" +
		strconv.FormatBool(event.Success) + "|" +
		event.Details
}

// SigningEnabled returns true if the signer has a valid key.
func (s *Signer) SigningEnabled() bool {
	return s.key != nil
}

// ExportKey exports the signing key as base64 for backup purposes.
// Returns empty string if signing is disabled.
func (s *Signer) ExportKey() string {
	if s.key == nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(s.key)
}
