package relay

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"crypto/sha256"
	"golang.org/x/crypto/hkdf"
	"io"
)

const (
	// nonceSize is the AES-256-GCM nonce size (12 bytes).
	nonceSize = 12

	// hkdfInfoAppToInstance is the HKDF info string for the app→instance direction.
	hkdfInfoAppToInstance = "relay-e2e-app-to-instance"

	// hkdfInfoInstanceToApp is the HKDF info string for the instance→app direction.
	hkdfInfoInstanceToApp = "relay-e2e-instance-to-app"

	// aesKeySize is the AES-256 key size in bytes.
	aesKeySize = 32
)

var (
	ErrNonceOverflow       = errors.New("nonce counter overflow")
	ErrCiphertextTooShort  = errors.New("ciphertext too short: need at least nonce + tag")
	ErrKeyExchangeTooShort = errors.New("key exchange payload too short")
	ErrNonceReplay         = errors.New("nonce replay or out-of-order: expected higher nonce")
	ErrIdentityKeyRequired = errors.New("identity private key required for key exchange signing")
	ErrKeyExchangeSigCheck = errors.New("key exchange signature verification failed")
)

// channelCipher holds the encryption state for one channel direction.
type channelCipher struct {
	aead      cipher.AEAD
	nonce     uint64 // send-side: next nonce to use
	recvNonce uint64 // recv-side: next expected nonce (must be strictly monotonic)
	mu        sync.Mutex
}

// ChannelEncryption holds the full encryption state for a channel.
type ChannelEncryption struct {
	sendCipher *channelCipher // outbound direction
	recvCipher *channelCipher // inbound direction
}

// GenerateEphemeralKeyPair creates an X25519 keypair for key exchange.
func GenerateEphemeralKeyPair() (*ecdh.PrivateKey, error) {
	return ecdh.X25519().GenerateKey(rand.Reader)
}

// DeriveChannelKeys performs ECDH + HKDF to produce directional AES-256-GCM ciphers.
// iAmInstance determines which direction maps to send vs recv:
//   - instance: send = instance→app, recv = app→instance
//   - app:      send = app→instance, recv = instance→app
func DeriveChannelKeys(myPrivate *ecdh.PrivateKey, theirPublic *ecdh.PublicKey, iAmInstance bool) (*ChannelEncryption, error) {
	sharedSecret, err := myPrivate.ECDH(theirPublic)
	if err != nil {
		return nil, fmt.Errorf("ecdh: %w", err)
	}

	// Derive app→instance key
	a2iKey, err := deriveKey(sharedSecret, hkdfInfoAppToInstance)
	if err != nil {
		return nil, fmt.Errorf("derive app→instance key: %w", err)
	}

	// Derive instance→app key
	i2aKey, err := deriveKey(sharedSecret, hkdfInfoInstanceToApp)
	if err != nil {
		return nil, fmt.Errorf("derive instance→app key: %w", err)
	}

	a2iCipher, err := newChannelCipher(a2iKey)
	if err != nil {
		return nil, fmt.Errorf("create app→instance cipher: %w", err)
	}

	i2aCipher, err := newChannelCipher(i2aKey)
	if err != nil {
		return nil, fmt.Errorf("create instance→app cipher: %w", err)
	}

	if iAmInstance {
		return &ChannelEncryption{
			sendCipher: i2aCipher, // instance sends on instance→app
			recvCipher: a2iCipher, // instance receives on app→instance
		}, nil
	}
	return &ChannelEncryption{
		sendCipher: a2iCipher, // app sends on app→instance
		recvCipher: i2aCipher, // app receives on instance→app
	}, nil
}

func deriveKey(secret []byte, info string) ([]byte, error) {
	hkdfReader := hkdf.New(sha256.New, secret, nil, []byte(info))
	key := make([]byte, aesKeySize)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, fmt.Errorf("hkdf read: %w", err)
	}
	return key, nil
}

func newChannelCipher(key []byte) (*channelCipher, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM cipher: %w", err)
	}
	return &channelCipher{aead: aead}, nil
}

// Encrypt seals plaintext with an incrementing nonce.
// Output format: [12-byte nonce][ciphertext + 16-byte GCM tag]
func (ce *ChannelEncryption) Encrypt(plaintext []byte) ([]byte, error) {
	c := ce.sendCipher
	c.mu.Lock()
	defer c.mu.Unlock()

	nonce, err := c.nextNonce()
	if err != nil {
		return nil, fmt.Errorf("generate next nonce: %w", err)
	}

	ciphertext := c.aead.Seal(nil, nonce, plaintext, nil)

	// Prepend nonce
	out := make([]byte, nonceSize+len(ciphertext))
	copy(out[:nonceSize], nonce)
	copy(out[nonceSize:], ciphertext)
	return out, nil
}

// Decrypt opens ciphertext in the format [12-byte nonce][ciphertext + tag].
// It enforces strict nonce monotonicity to prevent replay attacks: each
// received nonce must be strictly greater than or equal to the expected
// next nonce. The expected nonce advances after each successful decryption.
func (ce *ChannelEncryption) Decrypt(data []byte) ([]byte, error) {
	c := ce.recvCipher
	c.mu.Lock()
	defer c.mu.Unlock()

	minSize := nonceSize + c.aead.Overhead()
	if len(data) < minSize {
		return nil, ErrCiphertextTooShort
	}

	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	// Extract nonce counter (little-endian uint64 in first 8 bytes)
	receivedNonce := binary.LittleEndian.Uint64(nonce[:8])
	if receivedNonce < c.recvNonce {
		return nil, ErrNonceReplay
	}

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}

	// Only advance after successful decryption
	c.recvNonce = receivedNonce + 1

	return plaintext, nil
}

// nextNonce returns the next incrementing nonce and advances the counter.
func (c *channelCipher) nextNonce() ([]byte, error) {
	n := c.nonce
	if n == ^uint64(0) {
		return nil, ErrNonceOverflow
	}
	c.nonce++

	nonce := make([]byte, nonceSize)
	binary.LittleEndian.PutUint64(nonce[:8], n)
	// bytes 8-11 remain zero (upper 32 bits of uint96)
	return nonce, nil
}

// SignKeyExchange signs an ephemeral public key with the Ed25519 identity key.
func SignKeyExchange(ephemeralPub []byte, identityPrivateKeyB64 string) ([]byte, error) {
	privBytes, err := base64.StdEncoding.DecodeString(identityPrivateKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}
	if len(privBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key length: got %d, want %d", len(privBytes), ed25519.PrivateKeySize)
	}

	privKey := ed25519.PrivateKey(privBytes)
	sig := ed25519.Sign(privKey, ephemeralPub)
	return sig, nil
}

// VerifyKeyExchangeSignature verifies the Ed25519 signature on a KEY_EXCHANGE.
func VerifyKeyExchangeSignature(ephemeralPub, signature []byte, identityPublicKeyB64 string) error {
	pubBytes, err := base64.StdEncoding.DecodeString(identityPublicKeyB64)
	if err != nil {
		return fmt.Errorf("decode public key: %w", err)
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key length: got %d, want %d", len(pubBytes), ed25519.PublicKeySize)
	}

	pubKey := ed25519.PublicKey(pubBytes)
	if !ed25519.Verify(pubKey, ephemeralPub, signature) {
		return ErrKeyExchangeSigCheck
	}
	return nil
}

// MarshalKeyExchangePayload encodes a KEY_EXCHANGE payload as binary.
// Wire format: [1 byte pubkey len][pubkey][signature or nothing]
func MarshalKeyExchangePayload(pub []byte, sig []byte) []byte {
	out := make([]byte, 1+len(pub)+len(sig))
	out[0] = byte(len(pub))
	copy(out[1:1+len(pub)], pub)
	if len(sig) > 0 {
		copy(out[1+len(pub):], sig)
	}
	return out
}

// UnmarshalKeyExchangePayload decodes a KEY_EXCHANGE payload.
func UnmarshalKeyExchangePayload(data []byte) (pub []byte, sig []byte, err error) {
	if len(data) < 2 {
		return nil, nil, ErrKeyExchangeTooShort
	}

	pubLen := int(data[0])
	if len(data) < 1+pubLen {
		return nil, nil, ErrKeyExchangeTooShort
	}

	pub = data[1 : 1+pubLen]
	if len(data) > 1+pubLen {
		sig = data[1+pubLen:]
	}
	return pub, sig, nil
}
