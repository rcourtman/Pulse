package relay

import (
	"bytes"
	"crypto/ecdh"
	"testing"
)

func TestGenerateEphemeralKeyPair(t *testing.T) {
	key1, err := GenerateEphemeralKeyPair()
	if err != nil {
		t.Fatalf("GenerateEphemeralKeyPair: %v", err)
	}
	if len(key1.PublicKey().Bytes()) != 32 {
		t.Errorf("public key length: got %d, want 32", len(key1.PublicKey().Bytes()))
	}

	key2, err := GenerateEphemeralKeyPair()
	if err != nil {
		t.Fatalf("GenerateEphemeralKeyPair (second): %v", err)
	}
	if bytes.Equal(key1.PublicKey().Bytes(), key2.PublicKey().Bytes()) {
		t.Error("two generated keys should be different")
	}
}

func TestDeriveChannelKeys_Roundtrip(t *testing.T) {
	instancePriv, err := GenerateEphemeralKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	appPriv, err := GenerateEphemeralKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	instanceEnc, err := DeriveChannelKeys(instancePriv, appPriv.PublicKey(), true)
	if err != nil {
		t.Fatalf("DeriveChannelKeys (instance): %v", err)
	}
	appEnc, err := DeriveChannelKeys(appPriv, instancePriv.PublicKey(), false)
	if err != nil {
		t.Fatalf("DeriveChannelKeys (app): %v", err)
	}

	// App encrypts, instance decrypts
	plaintext := []byte("hello from the app")
	ciphertext, err := appEnc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("app Encrypt: %v", err)
	}
	decrypted, err := instanceEnc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("instance Decrypt: %v", err)
	}
	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("roundtrip app→instance: got %q, want %q", decrypted, plaintext)
	}

	// Instance encrypts, app decrypts
	plaintext2 := []byte("hello from the instance")
	ciphertext2, err := instanceEnc.Encrypt(plaintext2)
	if err != nil {
		t.Fatalf("instance Encrypt: %v", err)
	}
	decrypted2, err := appEnc.Decrypt(ciphertext2)
	if err != nil {
		t.Fatalf("app Decrypt: %v", err)
	}
	if !bytes.Equal(plaintext2, decrypted2) {
		t.Errorf("roundtrip instance→app: got %q, want %q", decrypted2, plaintext2)
	}
}

func TestChannelEncryption_IncrementingNonce(t *testing.T) {
	instancePriv, _ := GenerateEphemeralKeyPair()
	appPriv, _ := GenerateEphemeralKeyPair()

	enc, err := DeriveChannelKeys(instancePriv, appPriv.PublicKey(), true)
	if err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("same plaintext")
	ct1, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	ct2, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(ct1, ct2) {
		t.Error("same plaintext should produce different ciphertext with incrementing nonces")
	}

	// Verify nonces are different (first 12 bytes)
	if bytes.Equal(ct1[:nonceSize], ct2[:nonceSize]) {
		t.Error("nonces should differ between calls")
	}
}

func TestChannelEncryption_ReplayRejected(t *testing.T) {
	instancePriv, _ := GenerateEphemeralKeyPair()
	appPriv, _ := GenerateEphemeralKeyPair()

	instanceEnc, _ := DeriveChannelKeys(instancePriv, appPriv.PublicKey(), true)
	appEnc, _ := DeriveChannelKeys(appPriv, instancePriv.PublicKey(), false)

	// App encrypts a message
	ciphertext, err := appEnc.Encrypt([]byte("pay $100"))
	if err != nil {
		t.Fatal(err)
	}

	// Instance decrypts it successfully the first time
	plaintext, err := instanceEnc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("first decrypt: %v", err)
	}
	if string(plaintext) != "pay $100" {
		t.Fatalf("first decrypt: got %q", plaintext)
	}

	// Replaying the same frame must fail
	_, err = instanceEnc.Decrypt(ciphertext)
	if err == nil {
		t.Fatal("expected error on replay, got nil")
	}
	if err != ErrNonceReplay {
		t.Errorf("expected ErrNonceReplay, got: %v", err)
	}
}

func TestChannelEncryption_OutOfOrderNonceRejected(t *testing.T) {
	instancePriv, _ := GenerateEphemeralKeyPair()
	appPriv, _ := GenerateEphemeralKeyPair()

	instanceEnc, _ := DeriveChannelKeys(instancePriv, appPriv.PublicKey(), true)
	appEnc, _ := DeriveChannelKeys(appPriv, instancePriv.PublicKey(), false)

	// App sends three messages
	ct0, _ := appEnc.Encrypt([]byte("msg 0"))
	ct1, _ := appEnc.Encrypt([]byte("msg 1"))
	ct2, _ := appEnc.Encrypt([]byte("msg 2"))

	// Instance receives them in order: 0, then 2 (skipping 1)
	_, err := instanceEnc.Decrypt(ct0)
	if err != nil {
		t.Fatalf("decrypt ct0: %v", err)
	}

	// Skip ct1, decrypt ct2 — nonce 2 > expected 1, should succeed
	_, err = instanceEnc.Decrypt(ct2)
	if err != nil {
		t.Fatalf("decrypt ct2 (skip): %v", err)
	}

	// Now try ct1 — nonce 1 < expected 3, must fail
	_, err = instanceEnc.Decrypt(ct1)
	if err == nil {
		t.Fatal("expected error on out-of-order nonce, got nil")
	}
	if err != ErrNonceReplay {
		t.Errorf("expected ErrNonceReplay, got: %v", err)
	}
}

func TestChannelEncryption_TamperedCiphertext(t *testing.T) {
	instancePriv, _ := GenerateEphemeralKeyPair()
	appPriv, _ := GenerateEphemeralKeyPair()

	instanceEnc, _ := DeriveChannelKeys(instancePriv, appPriv.PublicKey(), true)
	appEnc, _ := DeriveChannelKeys(appPriv, instancePriv.PublicKey(), false)

	ciphertext, err := instanceEnc.Encrypt([]byte("secret data"))
	if err != nil {
		t.Fatal(err)
	}

	// Tamper with the ciphertext body (after nonce)
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[nonceSize+5] ^= 0xFF

	_, err = appEnc.Decrypt(tampered)
	if err == nil {
		t.Error("expected error decrypting tampered ciphertext")
	}
}

func TestSignAndVerifyKeyExchange(t *testing.T) {
	// Generate identity keypair
	privB64, pubB64, _, err := GenerateIdentityKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	ephemeralPub := []byte("32-byte-ephemeral-public-key!!!!") // 32 bytes

	sig, err := SignKeyExchange(ephemeralPub, privB64)
	if err != nil {
		t.Fatalf("SignKeyExchange: %v", err)
	}
	if len(sig) != 64 {
		t.Errorf("signature length: got %d, want 64", len(sig))
	}

	// Verify succeeds
	if err := VerifyKeyExchangeSignature(ephemeralPub, sig, pubB64); err != nil {
		t.Errorf("VerifyKeyExchangeSignature: %v", err)
	}

	// Tampered data fails
	tampered := make([]byte, len(ephemeralPub))
	copy(tampered, ephemeralPub)
	tampered[0] ^= 0xFF
	if err := VerifyKeyExchangeSignature(tampered, sig, pubB64); err == nil {
		t.Error("expected verification to fail on tampered data")
	}

	// Tampered signature fails
	badSig := make([]byte, len(sig))
	copy(badSig, sig)
	badSig[0] ^= 0xFF
	if err := VerifyKeyExchangeSignature(ephemeralPub, badSig, pubB64); err == nil {
		t.Error("expected verification to fail on tampered signature")
	}
}

func TestMarshalUnmarshalKeyExchangePayload(t *testing.T) {
	// With signature
	pub := bytes.Repeat([]byte{0xAA}, 32)
	sig := bytes.Repeat([]byte{0xBB}, 64)

	data := MarshalKeyExchangePayload(pub, sig)
	gotPub, gotSig, err := UnmarshalKeyExchangePayload(data)
	if err != nil {
		t.Fatalf("UnmarshalKeyExchangePayload (with sig): %v", err)
	}
	if !bytes.Equal(pub, gotPub) {
		t.Error("public key mismatch")
	}
	if !bytes.Equal(sig, gotSig) {
		t.Error("signature mismatch")
	}

	// Without signature
	data2 := MarshalKeyExchangePayload(pub, nil)
	gotPub2, gotSig2, err := UnmarshalKeyExchangePayload(data2)
	if err != nil {
		t.Fatalf("UnmarshalKeyExchangePayload (no sig): %v", err)
	}
	if !bytes.Equal(pub, gotPub2) {
		t.Error("public key mismatch (no sig)")
	}
	if gotSig2 != nil {
		t.Error("expected nil signature")
	}

	// Too short
	_, _, err = UnmarshalKeyExchangePayload([]byte{0x01})
	if err != ErrKeyExchangeTooShort {
		t.Errorf("expected ErrKeyExchangeTooShort, got %v", err)
	}
}

func TestDeriveChannelKeys_DirectionalKeys(t *testing.T) {
	instancePriv, _ := GenerateEphemeralKeyPair()
	appPriv, _ := GenerateEphemeralKeyPair()

	instanceEnc, err := DeriveChannelKeys(instancePriv, appPriv.PublicKey(), true)
	if err != nil {
		t.Fatal(err)
	}

	// Encrypt in both directions with the same plaintext
	plaintext := []byte("test message")

	sendCt, err := instanceEnc.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	// The send cipher (instance→app) and recv cipher (app→instance) use different keys.
	// Verify by checking that the recv cipher cannot decrypt what the send cipher produced.
	_, err = instanceEnc.Decrypt(sendCt)
	if err == nil {
		t.Error("expected error: recv cipher should not decrypt send cipher output (different keys)")
	}
}

// TestDeriveChannelKeys_CrossDirection verifies that two sides with swapped roles
// cannot cross-decrypt (i.e., the app's send direction matches the instance's recv).
func TestDeriveChannelKeys_CrossDirection(t *testing.T) {
	priv1, _ := ecdh.X25519().GenerateKey(fakeRandReader(1))
	priv2, _ := ecdh.X25519().GenerateKey(fakeRandReader(2))

	enc1, err := DeriveChannelKeys(priv1, priv2.PublicKey(), true)
	if err != nil {
		t.Fatal(err)
	}
	enc2, err := DeriveChannelKeys(priv2, priv1.PublicKey(), false)
	if err != nil {
		t.Fatal(err)
	}

	// enc1.send (instance→app) should be decryptable by enc2.recv (instance→app)
	ct, err := enc1.Encrypt([]byte("from instance"))
	if err != nil {
		t.Fatal(err)
	}
	pt, err := enc2.Decrypt(ct)
	if err != nil {
		t.Fatalf("cross-direction decrypt failed: %v", err)
	}
	if !bytes.Equal(pt, []byte("from instance")) {
		t.Errorf("cross-direction: got %q", pt)
	}
}

// fakeRandReader returns a deterministic reader for testing. NOT cryptographically secure.
func fakeRandReader(seed byte) *deterministicReader {
	return &deterministicReader{val: seed}
}

type deterministicReader struct {
	val byte
}

func (r *deterministicReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.val
		r.val = r.val*7 + 13 // simple LCG-style mutation
	}
	return len(p), nil
}
