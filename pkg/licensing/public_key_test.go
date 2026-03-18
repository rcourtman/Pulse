package licensing

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

func TestInitPublicKey(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	base64Pub := base64.StdEncoding.EncodeToString(pub)

	tests := []struct {
		name           string
		envKey         string
		embeddedKey    string
		devMode        bool
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
			devMode:        true,
			expectedLoaded: false,
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
			t.Setenv("PULSE_LICENSE_PUBLIC_KEY", "")
			if tt.envKey != "" {
				t.Setenv("PULSE_LICENSE_PUBLIC_KEY", tt.envKey)
			}

			var loadedKey ed25519.PublicKey
			InitPublicKey(tt.embeddedKey, tt.devMode, func(key ed25519.PublicKey) {
				loadedKey = key
			})

			loaded := loadedKey != nil
			if loaded != tt.expectedLoaded {
				t.Errorf("expectedLoaded = %v, got %v", tt.expectedLoaded, loaded)
			}
		})
	}
}

func TestInitPublicKeyNilCallback(t *testing.T) {
	t.Setenv("PULSE_LICENSE_PUBLIC_KEY", "")
	InitPublicKey("", true, nil)
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
		{
			name:    "whitespace trimmed",
			input:   "  " + base64.StdEncoding.EncodeToString(pub) + " \n",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodePublicKey(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodePublicKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPublicKeyFingerprint(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)

	got := PublicKeyFingerprint(pub)
	if !strings.HasPrefix(got, "SHA256:") {
		t.Fatalf("expected SHA256 prefix, got %q", got)
	}

	sum := sha256.Sum256(pub)
	want := "SHA256:" + base64.StdEncoding.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("fingerprint mismatch: got %q want %q", got, want)
	}

	if empty := PublicKeyFingerprint(nil); empty != "" {
		t.Fatalf("expected empty fingerprint for nil key, got %q", empty)
	}
}

func TestInitPublicKeyEnvironmentPrecedence(t *testing.T) {
	envPub, _, _ := ed25519.GenerateKey(nil)
	embeddedPub, _, _ := ed25519.GenerateKey(nil)

	t.Setenv("PULSE_LICENSE_PUBLIC_KEY", base64.StdEncoding.EncodeToString(envPub))

	var loadedKey ed25519.PublicKey
	InitPublicKey(base64.StdEncoding.EncodeToString(embeddedPub), false, func(key ed25519.PublicKey) {
		loadedKey = key
	})

	if loadedKey == nil {
		t.Fatal("expected key to be loaded from env")
	}
	if string(loadedKey) != string(envPub) {
		t.Fatal("expected environment key to take precedence over embedded key")
	}
}

func TestInitPublicKeyWithUnsetEnv(t *testing.T) {
	_ = os.Unsetenv("PULSE_LICENSE_PUBLIC_KEY")
	InitPublicKey("", false, func(ed25519.PublicKey) {})
}

func TestInitEmbeddedPublicKey(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	encoded := base64.StdEncoding.EncodeToString(pub)

	originalEmbedded := EmbeddedPublicKey
	EmbeddedPublicKey = encoded
	t.Cleanup(func() { EmbeddedPublicKey = originalEmbedded })

	t.Setenv("PULSE_LICENSE_PUBLIC_KEY", "")
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")
	SetPublicKey(nil)
	t.Cleanup(func() { SetPublicKey(nil) })

	InitEmbeddedPublicKey()

	got := currentPublicKey()
	if got == nil {
		t.Fatal("expected embedded key to be loaded")
	}
	if string(got) != string(pub) {
		t.Fatal("loaded key does not match embedded key")
	}
}
