package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestGetTenantComponents_IgnoresPersistedLegacyJWT(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init default persistence: %v", err)
	}

	// Create and persist a legacy test JWT.
	legacyJWT, err := pkglicensing.GenerateLicenseForTesting("user@example.com", pkglicensing.TierPro, 365*24*time.Hour)
	if err != nil {
		t.Fatalf("generate test license: %v", err)
	}
	cp, _ := mtp.GetPersistence("default")
	persistence, err := pkglicensing.NewPersistence(cp.GetConfigDir())
	if err != nil {
		t.Fatalf("new persistence: %v", err)
	}
	if err := persistence.Save(legacyJWT); err != nil {
		t.Fatalf("save legacy JWT: %v", err)
	}

	// Create handlers.
	handlers := NewLicenseHandlers(mtp, false)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	svc := handlers.Service(ctx)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}

	// Strict v6: persisted legacy JWTs are not auto-loaded on startup.
	if svc.Current() != nil {
		t.Fatal("expected no active license from persisted legacy JWT on startup")
	}
	if svc.IsActivated() {
		t.Error("expected IsActivated=false when no activation state is present")
	}

	// Persisted legacy JWT is left untouched for explicit/manual migration handling.
	loaded, _ := persistence.Load()
	if loaded == "" {
		t.Error("expected persisted legacy JWT to remain on disk")
	}

	handlers.StopAllBackgroundLoops()
}

func TestGetTenantComponents_SkipsExchange_WhenActivationStateExists(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	// Mock server that should NOT be called if activation state exists.
	var exchangeCalled atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/licenses/exchange" {
			exchangeCalled.Add(1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("PULSE_LICENSE_SERVER_URL", server.URL)

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init default persistence: %v", err)
	}

	// Create both a legacy JWT and activation state on disk.
	// The activation state should take priority and no exchange should happen.
	cp, _ := mtp.GetPersistence("default")
	persistence, err := pkglicensing.NewPersistence(cp.GetConfigDir())
	if err != nil {
		t.Fatalf("new persistence: %v", err)
	}

	// Save a legacy JWT (shouldn't be used since activation state exists).
	legacyJWT, err := pkglicensing.GenerateLicenseForTesting("user@example.com", pkglicensing.TierPro, 365*24*time.Hour)
	if err != nil {
		t.Fatalf("generate test license: %v", err)
	}
	if err := persistence.Save(legacyJWT); err != nil {
		t.Fatalf("save legacy JWT: %v", err)
	}

	// Create a grant JWT for the activation state.
	grantClaims := map[string]any{
		"lid":        "lic_existing",
		"tier":       "pro",
		"st":         "active",
		"feat":       []string{"relay"},
		"max_agents": 10,
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(72 * time.Hour).Unix(),
	}
	grantPayload, _ := json.Marshal(grantClaims)
	grantJWT := "eyJhbGciOiJFZERTQSJ9." + base64RawURLEncode(grantPayload) + "." + base64RawURLEncode([]byte("test-sig"))

	// Save activation state.
	activationState := &pkglicensing.ActivationState{
		InstallationID:      "inst_existing",
		InstallationToken:   "pit_live_existing",
		LicenseID:           "lic_existing",
		GrantJWT:            grantJWT,
		GrantJTI:            "grant_existing",
		GrantExpiresAt:      time.Now().Add(72 * time.Hour).Unix(),
		InstanceFingerprint: "fp-existing",
		LicenseServerURL:    server.URL,
		ActivatedAt:         time.Now().Unix(),
		LastRefreshedAt:     time.Now().Unix(),
	}
	if err := persistence.SaveActivationState(activationState); err != nil {
		t.Fatalf("save activation state: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, false)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	svc := handlers.Service(ctx)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}

	// Should have restored from activation state, NOT called exchange.
	// Give a brief moment for any potential background goroutine (shouldn't exist).
	time.Sleep(100 * time.Millisecond)
	if exchangeCalled.Load() != 0 {
		t.Error("exchange should NOT be called when activation state exists")
	}
	if !svc.IsActivated() {
		t.Error("expected IsActivated=true from restored activation state")
	}

	handlers.StopAllBackgroundLoops()
}

// base64RawURLEncode is a helper for tests.
func base64RawURLEncode(data []byte) string {
	// Use the same encoding as JWT: base64 URL-safe without padding.
	const encodeURL = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	result := make([]byte, 0, (len(data)*4+2)/3)
	for i := 0; i < len(data); i += 3 {
		var b0, b1, b2 byte
		b0 = data[i]
		if i+1 < len(data) {
			b1 = data[i+1]
		}
		if i+2 < len(data) {
			b2 = data[i+2]
		}
		result = append(result, encodeURL[b0>>2])
		result = append(result, encodeURL[((b0&0x03)<<4)|(b1>>4)])
		if i+1 < len(data) {
			result = append(result, encodeURL[((b1&0x0f)<<2)|(b2>>6)])
		}
		if i+2 < len(data) {
			result = append(result, encodeURL[b2&0x3f])
		}
	}
	return string(result)
}
