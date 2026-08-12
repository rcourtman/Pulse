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
	signatureV3Prefix = "v3:"
	canonicalV2Domain = "pulse.audit.event\x00v2\x00"
	canonicalV3Domain = "pulse.audit.event\x00v3\x00"
	v3KeyDomain       = "pulse.audit.hmac-key\x00v3\x00"
)

// SignatureVersion identifies the representation authenticated by a signature.
// Legacy signatures remain verifiable for historical compatibility, but their
// delimiter-separated representation does not protect field boundaries.
type SignatureVersion string

const (
	SignatureVersionUnsigned SignatureVersion = "unsigned"
	SignatureVersionUnknown  SignatureVersion = "unknown"
	SignatureVersionLegacy   SignatureVersion = "legacy"
	SignatureVersionV2       SignatureVersion = "v2"
	SignatureVersionV3       SignatureVersion = "v3"
)

// SignatureAssurance describes the integrity claim supported by a successful
// verification. Historical schemes remain compatibility-only because their
// signing domain either has ambiguous field boundaries or shares the legacy
// master-key domain.
type SignatureAssurance string

const (
	SignatureAssuranceNone          SignatureAssurance = "none"
	SignatureAssuranceCompatibility SignatureAssurance = "compatibility"
	SignatureAssuranceStrong        SignatureAssurance = "strong"
)

// VerificationStatus is the authoritative signature-verification outcome.
// Verified remains available as a compatibility projection, but callers must
// use Status and Assurance when presenting the strength of the evidence.
type VerificationStatus string

const (
	VerificationStatusStrong        VerificationStatus = "strong"
	VerificationStatusCompatibility VerificationStatus = "compatibility"
	VerificationStatusInvalid       VerificationStatus = "invalid"
	VerificationStatusUnknown       VerificationStatus = "unknown"
	VerificationStatusUnsigned      VerificationStatus = "unsigned"
)

// SignatureVerification carries a verification result without collapsing
// compatibility evidence into the current strong assurance class.
type SignatureVerification struct {
	Status    VerificationStatus `json:"status"`
	Version   SignatureVersion   `json:"version"`
	Assurance SignatureAssurance `json:"assurance"`
	Verified  bool               `json:"verified"`
}

// SignatureVerifier is the compatibility verification surface implemented by
// persistent audit loggers.
type SignatureVerifier interface {
	VerifySignature(Event) bool
}

// ClassifiedSignatureVerifier exposes the authoritative verification result.
type ClassifiedSignatureVerifier interface {
	VerifySignatureResult(Event) SignatureVerification
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

// Sign computes an HMAC-SHA256 signature over the injective v3 representation.
// v3 derives a version-specific HMAC key from the retained master key and
// authenticates its version domain. Removing or substituting the envelope
// therefore cannot move a v3 digest into an accepted historical MAC domain.
func (s *Signer) Sign(event Event) string {
	if s.key == nil {
		return ""
	}

	return signatureV3Prefix + hex.EncodeToString(s.v3MAC(s.canonicalV3Form(event)))
}

// Verify is the compatibility boolean projection of VerifyResult. Both strong
// and compatibility results return true; callers that display or aggregate
// assurance must use VerifyResult instead.
func (s *Signer) Verify(event Event) bool {
	return s.VerifyResult(event).Verified
}

// VerifyResult checks the signature using exactly the representation selected
// by its envelope and returns the evidence strength explicitly.
func (s *Signer) VerifyResult(event Event) SignatureVerification {
	version := DetectSignatureVersion(event.Signature)
	if version == SignatureVersionUnsigned {
		return signatureVerification(VerificationStatusUnsigned, version, SignatureAssuranceNone, false)
	}
	if s.key == nil {
		return signatureVerification(VerificationStatusUnknown, version, SignatureAssuranceNone, false)
	}

	switch version {
	case SignatureVersionV3:
		provided, err := decodeEnvelopeMAC(event.Signature, signatureV3Prefix)
		if err != nil || !hmac.Equal(s.v3MAC(s.canonicalV3Form(event)), provided) {
			return signatureVerification(VerificationStatusInvalid, version, SignatureAssuranceNone, false)
		}
		return signatureVerification(VerificationStatusStrong, version, SignatureAssuranceStrong, true)
	case SignatureVersionV2:
		provided, err := decodeEnvelopeMAC(event.Signature, signatureV2Prefix)
		if err != nil || !hmac.Equal(s.mac(s.canonicalV2Form(event)), provided) {
			return signatureVerification(VerificationStatusInvalid, version, SignatureAssuranceNone, false)
		}
		return signatureVerification(VerificationStatusCompatibility, version, SignatureAssuranceCompatibility, true)
	case SignatureVersionLegacy:
		provided, err := hex.DecodeString(event.Signature)
		if err != nil || len(provided) != sha256.Size {
			return signatureVerification(VerificationStatusInvalid, version, SignatureAssuranceNone, false)
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
				return signatureVerification(VerificationStatusCompatibility, version, SignatureAssuranceCompatibility, true)
			}
		}
		return signatureVerification(VerificationStatusInvalid, version, SignatureAssuranceNone, false)
	default:
		return signatureVerification(VerificationStatusUnknown, version, SignatureAssuranceNone, false)
	}
}

func signatureVerification(status VerificationStatus, version SignatureVersion, assurance SignatureAssurance, verified bool) SignatureVerification {
	return SignatureVerification{Status: status, Version: version, Assurance: assurance, Verified: verified}
}

func decodeEnvelopeMAC(signature, prefix string) ([]byte, error) {
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, prefix))
	if err != nil || len(provided) != sha256.Size {
		return nil, fmt.Errorf("invalid signature MAC")
	}
	return provided, nil
}

// ClassifySignature uses the richer logger contract when available and keeps a
// conservative compatibility path for third-party Logger implementations that
// expose only the historical boolean method.
func ClassifySignature(verifier any, event Event) SignatureVerification {
	if classified, ok := verifier.(ClassifiedSignatureVerifier); ok {
		return classified.VerifySignatureResult(event)
	}

	version := DetectSignatureVersion(event.Signature)
	if version == SignatureVersionUnsigned {
		return signatureVerification(VerificationStatusUnsigned, version, SignatureAssuranceNone, false)
	}
	// A boolean-only verifier cannot establish the semantics of an unknown
	// envelope. Never let its result move malformed or unsupported input into
	// an accepted historical domain.
	if version == SignatureVersionUnknown {
		return signatureVerification(VerificationStatusUnknown, version, SignatureAssuranceNone, false)
	}
	booleanVerifier, ok := verifier.(SignatureVerifier)
	if !ok {
		return signatureVerification(VerificationStatusUnknown, version, SignatureAssuranceNone, false)
	}
	if !booleanVerifier.VerifySignature(event) {
		return signatureVerification(VerificationStatusInvalid, version, SignatureAssuranceNone, false)
	}
	// The old boolean contract can prove only that an implementation accepted
	// the event. It cannot prove that it applied the v3 domain-separated
	// verifier, so even a v3 envelope remains compatibility assurance here.
	return signatureVerification(VerificationStatusCompatibility, version, SignatureAssuranceCompatibility, true)
}

// DetectSignatureVersion classifies only well-formed signature envelopes.
// Unprefixed 64-digit hexadecimal MACs are historical. Any prefix other than
// v2/v3, or any malformed/truncated MAC, is unknown and must fail closed.
func DetectSignatureVersion(signature string) SignatureVersion {
	if signature == "" {
		return SignatureVersionUnsigned
	}
	if strings.HasPrefix(signature, signatureV3Prefix) {
		if isSHA256Hex(signature[len(signatureV3Prefix):]) {
			return SignatureVersionV3
		}
		return SignatureVersionUnknown
	}
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

func (s *Signer) v3MAC(message []byte) []byte {
	mac := hmac.New(sha256.New, s.v3Key())
	_, _ = mac.Write(message)
	return mac.Sum(nil)
}

func (s *Signer) v3Key() []byte {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(v3KeyDomain))
	return mac.Sum(nil)
}

// canonicalV3Form is the current authenticated SQLite tuple. It retains the
// v2 field ordering and adds the canonical legacy timestamp evidence column.
// The version domain is inside the MAC as well as being cryptographically
// separated by v3Key.
func (s *Signer) canonicalV3Form(event Event) []byte {
	var canonical bytes.Buffer
	canonical.WriteString(canonicalV3Domain)
	writeLengthPrefixedString(&canonical, signatureV3Prefix)
	writeLengthPrefixedString(&canonical, event.ID)

	writeInt64BigEndian(&canonical, event.Timestamp.Unix())

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
	writeLengthPrefixedString(&canonical, event.SignatureTimestamp)
	return canonical.Bytes()
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

	writeInt64BigEndian(&canonical, event.Timestamp.Unix())

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

func writeInt64BigEndian(dst *bytes.Buffer, value int64) {
	// bytes.Buffer writes cannot fail. binary.Write preserves the exact two's
	// complement big-endian representation used by the previous uint64 cast,
	// without relying on a signed-to-unsigned conversion.
	if err := binary.Write(dst, binary.BigEndian, value); err != nil {
		panic("audit: encode int64 into bytes.Buffer: " + err.Error())
	}
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
	timestamp := event.SignatureTimestamp
	if timestamp == "" {
		timestamp = event.Timestamp.UTC().Format(time.RFC3339Nano)
	}
	return event.ID + "|" +
		timestamp + "|" +
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
