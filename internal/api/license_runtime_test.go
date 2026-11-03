package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	licensetestsupport "github.com/rcourtman/pulse-go-rewrite/pkg/licensing/testsupport"
)

// activateAndCaptureRuntime activates a key through the handler against a fake
// license server and returns the runtime identity the server received.
func activateAndCaptureRuntime(t *testing.T, handler *LicenseHandlers, server *captureActivationServer) *pkglicensing.RuntimeIdentity {
	t.Helper()
	t.Cleanup(handler.StopAllBackgroundLoops)

	body, _ := json.Marshal(map[string]string{"license_key": "ppk_live_runtime_identity"})
	req := httptest.NewRequest(http.MethodPost, "/api/license/activate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleActivateLicense(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	return server.capturedRuntime(t)
}

type captureActivationServer struct {
	mu      sync.Mutex
	request *pkglicensing.ActivateInstallationRequest
}

func (s *captureActivationServer) capturedRuntime(t *testing.T) *pkglicensing.RuntimeIdentity {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.request == nil {
		t.Fatal("license server never received an activation request")
	}
	return s.request.Runtime
}

func newCaptureActivationServer(t *testing.T) *captureActivationServer {
	t.Helper()
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	grantJWT, grantPublicKey, err := licensetestsupport.GenerateGrantJWTForTesting(pkglicensing.GrantClaims{
		LicenseID:      "lic_v6_runtime",
		LicenseVersion: 1,
		Tier:           "pro",
		State:          "active",
		Features:       []string{"relay"},
		IssuedAt:       time.Now().Unix(),
		ExpiresAt:      time.Now().Add(72 * time.Hour).Unix(),
		Email:          "runtime@example.com",
	})
	if err != nil {
		t.Fatalf("generate grant jwt: %v", err)
	}
	license.SetPublicKey(grantPublicKey)
	t.Cleanup(func() { license.SetPublicKey(nil) })

	captured := &captureActivationServer{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleTestInstallationStatus(w, r, 1) {
			return
		}
		if r.URL.Path != "/v1/activate" {
			t.Errorf("path = %q, want /v1/activate", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var activation pkglicensing.ActivateInstallationRequest
		if err := json.NewDecoder(r.Body).Decode(&activation); err != nil {
			t.Errorf("decode activation request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		captured.mu.Lock()
		captured.request = &activation
		captured.mu.Unlock()

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"license": map[string]any{
				"license_id":            "lic_v6_runtime",
				"state":                 "active",
				"tier":                  "pro",
				"features":              []string{"relay"},
				"max_monitored_systems": 10,
			},
			"installation": map[string]any{
				"installation_id":    "inst_v6_runtime",
				"installation_token": "pit_live_runtime",
				"status":             "active",
			},
			"grant": map[string]any{
				"jwt":        grantJWT,
				"jti":        "grant_v6_runtime",
				"expires_at": time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	t.Cleanup(server.Close)
	t.Setenv("PULSE_LICENSE_SERVER_URL", server.URL)
	return captured
}

// A Pro build must tell the license server it is a Pro runtime when it
// activates a key, whether the tenant's license service is created by the
// activation itself or was already cached by an earlier request.
func TestHandleActivateLicense_ReportsRuntimeIdentityToLicenseServer(t *testing.T) {
	t.Run("new tenant service", func(t *testing.T) {
		server := newCaptureActivationServer(t)
		handler := createTestHandler(t)
		handler.SetRuntimeIdentity(pkglicensing.ProRuntimeIdentity())

		runtime := activateAndCaptureRuntime(t, handler, server)
		if runtime == nil || runtime.Build != pkglicensing.RuntimeBuildPro {
			t.Fatalf("activation runtime = %+v, want %q build", runtime, pkglicensing.RuntimeBuildPro)
		}
	})

	t.Run("cached tenant service", func(t *testing.T) {
		server := newCaptureActivationServer(t)
		handler := createTestHandler(t)

		// An earlier request caches the tenant service while the handler still
		// reports the community runtime.
		probe := httptest.NewRequest(http.MethodGet, "/api/license/status", nil)
		if _, _, err := handler.getTenantComponents(probe.Context()); err != nil {
			t.Fatalf("getTenantComponents: %v", err)
		}
		handler.SetRuntimeIdentity(pkglicensing.ProRuntimeIdentity())

		runtime := activateAndCaptureRuntime(t, handler, server)
		if runtime == nil || runtime.Build != pkglicensing.RuntimeBuildPro {
			t.Fatalf("activation runtime = %+v, want %q build", runtime, pkglicensing.RuntimeBuildPro)
		}
	})
}
