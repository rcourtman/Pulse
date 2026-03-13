package licensing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewLicenseServerClient(t *testing.T) {
	t.Run("explicit URL", func(t *testing.T) {
		c := NewLicenseServerClient("https://custom.example.com/")
		if c.BaseURL() != "https://custom.example.com" {
			t.Errorf("BaseURL = %q, want trailing slash stripped", c.BaseURL())
		}
	})

	t.Run("default URL when empty", func(t *testing.T) {
		// Clear env to test default.
		t.Setenv("PULSE_LICENSE_SERVER_URL", "")
		c := NewLicenseServerClient("")
		if c.BaseURL() != DefaultLicenseServerURL {
			t.Errorf("BaseURL = %q, want %q", c.BaseURL(), DefaultLicenseServerURL)
		}
	})

	t.Run("env override", func(t *testing.T) {
		t.Setenv("PULSE_LICENSE_SERVER_URL", "https://env.example.com")
		c := NewLicenseServerClient("")
		if c.BaseURL() != "https://env.example.com" {
			t.Errorf("BaseURL = %q, want env override", c.BaseURL())
		}
	})
}

func TestClientActivate(t *testing.T) {
	t.Run("successful activation", func(t *testing.T) {
		grantJWT := makeTestGrantJWT(t, &GrantClaims{
			LicenseID: "lic_test",
			Tier:      "pro",
			State:     "active",
			IssuedAt:  1000,
			ExpiresAt: 2000,
		})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("Method = %q, want POST", r.Method)
			}
			if r.URL.Path != "/v1/activate" {
				t.Errorf("Path = %q, want /v1/activate", r.URL.Path)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
			}
			if r.Header.Get("Idempotency-Key") == "" {
				t.Error("missing Idempotency-Key header")
			}

			var req ActivateInstallationRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode request: %v", err)
			}
			if req.ActivationKey != "ppk_live_test123" {
				t.Errorf("ActivationKey = %q, want ppk_live_test123", req.ActivationKey)
			}

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(ActivateInstallationResponse{
				License: ActivateResponseLicense{
					LicenseID: "lic_test",
					State:     "active",
					Tier:      "pro",
				},
				Installation: ActivateResponseInstallation{
					InstallationID:    "inst_abc",
					InstallationToken: "pit_live_token",
					Status:            "active",
				},
				Grant: GrantEnvelope{
					JWT:       grantJWT,
					JTI:       "grant_123",
					ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
				},
				RefreshPolicy: RefreshHints{
					IntervalSeconds: 21600,
					JitterPercent:   0.2,
				},
			})
		}))
		defer server.Close()

		client := NewLicenseServerClient(server.URL)
		resp, err := client.Activate(context.Background(), ActivateInstallationRequest{
			ActivationKey:       "ppk_live_test123",
			InstanceFingerprint: "fp-123",
		})
		if err != nil {
			t.Fatalf("Activate failed: %v", err)
		}
		if resp.Installation.InstallationID != "inst_abc" {
			t.Errorf("InstallationID = %q, want inst_abc", resp.Installation.InstallationID)
		}
		if resp.Installation.InstallationToken != "pit_live_token" {
			t.Errorf("InstallationToken = %q, want pit_live_token", resp.Installation.InstallationToken)
		}
		if resp.Grant.JTI != "grant_123" {
			t.Errorf("Grant.JTI = %q, want grant_123", resp.Grant.JTI)
		}
	})

	t.Run("server returns structured error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{
				"code":      "invalid_key",
				"message":   "Activation key not found",
				"retryable": false,
			})
		}))
		defer server.Close()

		client := NewLicenseServerClient(server.URL)
		_, err := client.Activate(context.Background(), ActivateInstallationRequest{
			ActivationKey: "ppk_live_bad",
		})
		if err == nil {
			t.Fatal("expected error")
		}

		apiErr, ok := err.(*LicenseServerError)
		if !ok {
			t.Fatalf("expected *LicenseServerError, got %T", err)
		}
		if apiErr.StatusCode != 400 {
			t.Errorf("StatusCode = %d, want 400", apiErr.StatusCode)
		}
		if apiErr.Code != "invalid_key" {
			t.Errorf("Code = %q, want invalid_key", apiErr.Code)
		}
		if apiErr.Retryable {
			t.Error("expected Retryable=false for 400")
		}
	})

	t.Run("server error is retryable", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewLicenseServerClient(server.URL)
		_, err := client.Activate(context.Background(), ActivateInstallationRequest{
			ActivationKey: "ppk_live_test",
		})
		if err == nil {
			t.Fatal("expected error")
		}

		apiErr, ok := err.(*LicenseServerError)
		if !ok {
			t.Fatalf("expected *LicenseServerError, got %T", err)
		}
		if !apiErr.Retryable {
			t.Error("expected 500 errors to be retryable")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Slow response — never completes
			<-r.Context().Done()
		}))
		defer server.Close()

		client := NewLicenseServerClient(server.URL)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := client.Activate(ctx, ActivateInstallationRequest{
			ActivationKey: "ppk_live_test",
		})
		if err == nil {
			t.Fatal("expected error from canceled context")
		}
	})
}

func TestClientExchangeLegacyLicense(t *testing.T) {
	t.Run("successful exchange", func(t *testing.T) {
		grantJWT := makeTestGrantJWT(t, &GrantClaims{
			LicenseID: "lic_legacy",
			Tier:      "lifetime",
			State:     "active",
			IssuedAt:  1000,
			ExpiresAt: 2000,
		})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("Method = %q, want POST", r.Method)
			}
			if r.URL.Path != "/v1/licenses/exchange" {
				t.Errorf("Path = %q, want /v1/licenses/exchange", r.URL.Path)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
			}
			if r.Header.Get("Idempotency-Key") == "" {
				t.Error("missing Idempotency-Key header")
			}

			var raw map[string]any
			if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
				t.Errorf("decode request: %v", err)
			}
			body, err := json.Marshal(raw)
			if err != nil {
				t.Fatalf("re-marshal request: %v", err)
			}
			var req ExchangeLegacyLicenseRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("decode compatibility request: %v", err)
			}
			if req.LegacyLicenseKey != "header.payload.signature" {
				t.Errorf("LegacyLicenseKey = %q, want header.payload.signature", req.LegacyLicenseKey)
			}
			if got := raw["legacy_license_token"]; got != "header.payload.signature" {
				t.Errorf("legacy_license_token = %v, want header.payload.signature", got)
			}
			if _, hasLegacyKey := raw["legacy_license_key"]; hasLegacyKey {
				t.Error("legacy_license_key should not be sent by client")
			}

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(ActivateInstallationResponse{
				License: ActivateResponseLicense{
					LicenseID: "lic_legacy",
					State:     "active",
					Tier:      "lifetime",
				},
				Installation: ActivateResponseInstallation{
					InstallationID:    "inst_legacy",
					InstallationToken: "pit_live_legacy",
					Status:            "active",
				},
				Grant: GrantEnvelope{
					JWT:       grantJWT,
					JTI:       "grant_legacy",
					ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
				},
			})
		}))
		defer server.Close()

		client := NewLicenseServerClient(server.URL)
		resp, err := client.ExchangeLegacyLicense(context.Background(), ExchangeLegacyLicenseRequest{
			LegacyLicenseKey:    "header.payload.signature",
			InstanceFingerprint: "fp-legacy",
		})
		if err != nil {
			t.Fatalf("ExchangeLegacyLicense failed: %v", err)
		}
		if resp.Installation.InstallationID != "inst_legacy" {
			t.Errorf("InstallationID = %q, want inst_legacy", resp.Installation.InstallationID)
		}
		if resp.Grant.JTI != "grant_legacy" {
			t.Errorf("Grant.JTI = %q, want grant_legacy", resp.Grant.JTI)
		}
	})
}

func TestClientRefreshGrant(t *testing.T) {
	t.Run("successful refresh", func(t *testing.T) {
		newGrantJWT := makeTestGrantJWT(t, &GrantClaims{
			LicenseID: "lic_test",
			Tier:      "pro",
			State:     "active",
			IssuedAt:  3000,
			ExpiresAt: 4000,
		})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("Method = %q, want POST", r.Method)
			}
			if r.URL.Path != "/v1/grants/refresh" {
				t.Errorf("Path = %q, want /v1/grants/refresh", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer pit_live_token" {
				t.Errorf("Authorization = %q, want Bearer pit_live_token", got)
			}

			json.NewEncoder(w).Encode(RefreshGrantResponse{
				Grant: GrantEnvelope{
					JWT:       newGrantJWT,
					JTI:       "grant_456",
					ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
				},
			})
		}))
		defer server.Close()

		client := NewLicenseServerClient(server.URL)
		resp, err := client.RefreshGrant(context.Background(), "inst_abc", "pit_live_token", RefreshGrantRequest{
			InstallationID:      "inst_abc",
			InstanceFingerprint: "fp-123",
			CurrentGrantJTI:     "grant_123",
		})
		if err != nil {
			t.Fatalf("RefreshGrant failed: %v", err)
		}
		if resp.Grant.JTI != "grant_456" {
			t.Errorf("Grant.JTI = %q, want grant_456", resp.Grant.JTI)
		}
	})

	t.Run("401 returns structured error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"code":    "token_revoked",
				"message": "Installation token has been revoked",
			})
		}))
		defer server.Close()

		client := NewLicenseServerClient(server.URL)
		_, err := client.RefreshGrant(context.Background(), "inst_abc", "bad_token", RefreshGrantRequest{})
		if err == nil {
			t.Fatal("expected error")
		}

		apiErr, ok := err.(*LicenseServerError)
		if !ok {
			t.Fatalf("expected *LicenseServerError, got %T", err)
		}
		if apiErr.StatusCode != 401 {
			t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
		}
		if apiErr.Code != "token_revoked" {
			t.Errorf("Code = %q, want token_revoked", apiErr.Code)
		}
	})
}
