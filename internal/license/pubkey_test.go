package license

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

func TestInitPublicKey(t *testing.T) {
	// Generate a temporary key pair for testing
	pub, _, _ := ed25519.GenerateKey(nil)
	base64Pub := base64.StdEncoding.EncodeToString(pub)

	tests := []struct {
		name           string
		envKey         string
		embeddedKey    string
		devMode        string
		expectedLoaded bool
	}{
		{
			name:           "load from environment",
			envKey:         base64Pub,
			expectedLoaded: true,
		},
		{
			name:           "load from embedded",
			embeddedKey:    base64Pub,
			expectedLoaded: true,
		},
		{
			name:           "dev mode",
			devMode:        "true",
			expectedLoaded: false, // In dev mode it doesn't set the key
		},
		{
			name:           "no key available",
			expectedLoaded: false,
		},
		{
			name:           "malformed env key falls back",
			envKey:         "not-base64",
			embeddedKey:    base64Pub,
			expectedLoaded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear state
			SetPublicKey(nil)
			os.Unsetenv("PULSE_LICENSE_PUBLIC_KEY")
			os.Unsetenv("PULSE_LICENSE_DEV_MODE")
			EmbeddedPublicKey = tt.embeddedKey

			if tt.envKey != "" {
				os.Setenv("PULSE_LICENSE_PUBLIC_KEY", tt.envKey)
			}
			if tt.devMode != "" {
				os.Setenv("PULSE_LICENSE_DEV_MODE", tt.devMode)
			}

			InitPublicKey()

			loaded := publicKey != nil
			if loaded != tt.expectedLoaded {
				t.Errorf("expectedLoaded = %v, got %v", tt.expectedLoaded, loaded)
			}
		})
	}

	// Clean up for other tests
	os.Unsetenv("PULSE_LICENSE_PUBLIC_KEY")
	os.Unsetenv("PULSE_LICENSE_DEV_MODE")
	EmbeddedPublicKey = ""
}

func TestDecodePublicKey(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid standard base64",
			input:   base64.StdEncoding.EncodeToString(pub),
			wantErr: false,
		},
		{
			name:    "valid URL-safe base64",
			input:   base64.RawURLEncoding.EncodeToString(pub),
			wantErr: false,
		},
		{
			name:    "invalid base64",
			input:   "!!!",
			wantErr: true,
		},
		{
			name:    "wrong size",
			input:   base64.StdEncoding.EncodeToString([]byte("too-short")),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodePublicKey(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodePublicKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPublicKeyFingerprint(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)

	got := publicKeyFingerprint(pub)
	if !strings.HasPrefix(got, "SHA256:") {
		t.Fatalf("expected SHA256 prefix, got %q", got)
	}

	sum := sha256.Sum256(pub)
	want := "SHA256:" + base64.StdEncoding.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("fingerprint mismatch: got %q want %q", got, want)
	}

	if empty := publicKeyFingerprint(nil); empty != "" {
		t.Fatalf("expected empty fingerprint for nil key, got %q", empty)
	}
}
