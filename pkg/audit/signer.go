package audit

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Signer handles HMAC-SHA256 signing and verification for audit events.
// The signing key is stored encrypted at rest using the provided crypto manager.
type Signer struct {
	key []byte // 32-byte HMAC signing key
}

const (
	signatureV2Prefix = "v2:"
	canonicalV2Domain = "pulse.audit.event\x00v2\x00"
)

// SignatureVersion identifies the representation authenticated by a signature.
// Legacy signatures remain verifiable for historical compatibility, but their
// delimiter-separated representation does not protect field boundaries.
type SignatureVersion string

const (
	SignatureVersionUnknown SignatureVersion = "unknown"
	SignatureVersionLegacy  SignatureVersion = "legacy"
	SignatureVersionV2      SignatureVersion = "v2"
)

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

// Sign computes an HMAC-SHA256 signature over the injective v2 representation.
// The version prefix is persisted with the hex-encoded MAC so verification can
// select exactly one representation without downgrade fallback.
func (s *Signer) Sign(event Event) string {
	if s.key == nil {
		return ""
	}

	return signatureV2Prefix + hex.EncodeToString(s.mac(s.canonicalV2Form(event)))
}

// Verify checks if the event's signature matches its content. A true result for
// SignatureVersionLegacy confirms only a historical delimiter-based MAC; call
// DetectSignatureVersion when the stronger v2 boundary guarantee matters.
// Returns false for invalid, unknown, malformed, or disabled signatures.
func (s *Signer) Verify(event Event) bool {
	if s.key == nil || event.Signature == "" {
		return false
	}

	switch DetectSignatureVersion(event.Signature) {
	case SignatureVersionV2:
		provided, err := hex.DecodeString(strings.TrimPrefix(event.Signature, signatureV2Prefix))
		if err != nil || len(provided) != sha256.Size {
			return false
		}
		return hmac.Equal(s.mac(s.canonicalV2Form(event)), provided)
	case SignatureVersionLegacy:
		provided, err := hex.DecodeString(event.Signature)
		if err != nil || len(provided) != sha256.Size {
			return false
		}
		// These three unversioned encodings were emitted by historical Pulse
		// releases. They are intentionally confined to the legacy dispatch arm:
		// a v2-prefixed signature is never retried here.
		for _, canonical := range []string{
			s.legacyZeroOneCanonicalForm(event),
			s.legacyUnixCanonicalForm(event),
			s.legacyTimeCanonicalForm(event),
		} {
			if hmac.Equal(s.mac([]byte(canonical)), provided) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// DetectSignatureVersion classifies only well-formed signature envelopes.
// Unprefixed 64-digit hexadecimal MACs are historical. Any prefix other than
// v2, or any malformed/truncated MAC, is unknown and must fail closed.
func DetectSignatureVersion(signature string) SignatureVersion {
	if strings.HasPrefix(signature, signatureV2Prefix) {
		if isSHA256Hex(signature[len(signatureV2Prefix):]) {
			return SignatureVersionV2
		}
		return SignatureVersionUnknown
	}
	if strings.Contains(signature, ":") {
		return SignatureVersionUnknown
	}
	if isSHA256Hex(signature) {
		return SignatureVersionLegacy
	}
	return SignatureVersionUnknown
}

func isSHA256Hex(value string) bool {
	if len(value) != hex.EncodedLen(sha256.Size) {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func (s *Signer) mac(message []byte) []byte {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(message)
	return mac.Sum(nil)
}

// canonicalV2Form is an injective representation of the exact SQLite tuple:
// domain || len(ID) || ID || int64 Unix seconds || len(EventType) || EventType
// || len(User) || User || len(IP) || IP || len(Path) || Path || Success byte
// || len(Details) || Details. Integers and uint64 byte lengths are big-endian;
// strings are their unmodified UTF-8 bytes. SQLite persists timestamps as Unix
// seconds, so sub-second time data is deliberately outside the signed tuple.
func (s *Signer) canonicalV2Form(event Event) []byte {
	var canonical bytes.Buffer
	canonical.WriteString(canonicalV2Domain)
	writeLengthPrefixedString(&canonical, event.ID)

	var timestamp [8]byte
	binary.BigEndian.PutUint64(timestamp[:], uint64(event.Timestamp.Unix()))
	canonical.Write(timestamp[:])

	writeLengthPrefixedString(&canonical, event.EventType)
	writeLengthPrefixedString(&canonical, event.User)
	writeLengthPrefixedString(&canonical, event.IP)
	writeLengthPrefixedString(&canonical, event.Path)
	if event.Success {
		canonical.WriteByte(1)
	} else {
		canonical.WriteByte(0)
	}
	writeLengthPrefixedString(&canonical, event.Details)
	return canonical.Bytes()
}

func writeLengthPrefixedString(dst *bytes.Buffer, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	dst.Write(length[:])
	dst.WriteString(value)
}

func (s *Signer) signCanonical(canonical string) string {
	return hex.EncodeToString(s.mac([]byte(canonical)))
}

// legacyZeroOneCanonicalForm is the last unversioned representation emitted
// before v2. It is ambiguous when string values contain pipe characters.
func (s *Signer) legacyZeroOneCanonicalForm(event Event) string {
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
