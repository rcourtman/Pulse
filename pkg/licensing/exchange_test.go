package licensing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientExchangeLegacy(t *testing.T) {
	t.Run("successful exchange", func(t *testing.T) {
		grantJWT := makeTestGrantJWT(t, &GrantClaims{
			LicenseID: "lic_exchanged",
			Tier:      "pro",
			State:     "active",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
		})

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("Method = %q, want POST", r.Method)
			}
			if r.URL.Path != "/v1/licenses/exchange" {
				t.Errorf("Path = %q, want /v1/licenses/exchange", r.URL.Path)
			}

			var req ExchangeLegacyRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode request: %v", err)
			}
			if req.LegacyLicenseToken == "" {
				t.Error("expected non-empty legacy_license_token")
			}

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(ExchangeLegacyResponse{
				Migration: ExchangeMigrationInfo{
					Source:              "legacy_jwt",
					LegacyLID:           "old_lid",
					ResolvedV6LicenseID: "lic_exchanged",
				},
				Installation: ExchangeInstallation{
					InstallationID:    "inst_exch",
					InstallationToken: "pit_live_exch",
					Status:            "active",
				},
				Grant: GrantEnvelope{
					JWT:       grantJWT,
					JTI:       "grant_exch",
					ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
					Refresh: RefreshHints{
						IntervalSeconds: 21600,
						JitterPercent:   0.2,
					},
				},
			})
		}))
		defer server.Close()

		client := NewLicenseServerClient(server.URL)
		resp, err := client.ExchangeLegacy(t.Context(), ExchangeLegacyRequest{
			LegacyLicenseToken:  "legacy.jwt.token",
			InstanceFingerprint: "fp-123",
		})
		if err != nil {
			t.Fatalf("ExchangeLegacy failed: %v", err)
		}
		if resp.Installation.InstallationID != "inst_exch" {
			t.Errorf("InstallationID = %q, want inst_exch", resp.Installation.InstallationID)
		}
		if resp.Migration.ResolvedV6LicenseID != "lic_exchanged" {
			t.Errorf("ResolvedV6LicenseID = %q, want lic_exchanged", resp.Migration.ResolvedV6LicenseID)
		}
	})

	t.Run("exchange error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{
				"code":    "invalid_legacy_token",
				"message": "Legacy token signature invalid",
			})
		}))
		defer server.Close()

		client := NewLicenseServerClient(server.URL)
		_, err := client.ExchangeLegacy(t.Context(), ExchangeLegacyRequest{
			LegacyLicenseToken: "bad.jwt.token",
		})
		if err == nil {
			t.Fatal("expected error")
		}
		apiErr, ok := err.(*LicenseServerError)
		if !ok {
			t.Fatalf("expected *LicenseServerError, got %T", err)
		}
		if apiErr.Code != "invalid_legacy_token" {
			t.Errorf("Code = %q, want invalid_legacy_token", apiErr.Code)
		}
	})
}

func TestServiceExchangeLegacyLicense(t *testing.T) {
	grantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_v6",
		Tier:      "pro",
		State:     "active",
		Features:  []string{"relay", "ai_patrol"},
		MaxAgents: 15,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ExchangeLegacyResponse{
			Migration: ExchangeMigrationInfo{
				Source:              "legacy_jwt",
				LegacyLID:           "old_lid",
				ResolvedV6LicenseID: "lic_v6",
			},
			Installation: ExchangeInstallation{
				InstallationID:    "inst_v6",
				InstallationToken: "pit_live_v6",
				Status:            "active",
			},
			Grant: GrantEnvelope{
				JWT:       grantJWT,
				JTI:       "grant_v6",
				ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
				Refresh: RefreshHints{
					IntervalSeconds: 21600,
					JitterPercent:   0.2,
				},
			},
		})
	}))
	defer server.Close()

	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))

	lic, err := svc.ExchangeLegacyLicense("legacy.jwt.here")
	if err != nil {
		t.Fatalf("ExchangeLegacyLicense failed: %v", err)
	}

	// Verify license was set.
	if lic == nil {
		t.Fatal("expected non-nil license")
	}
	if lic.Claims.LicenseID != "lic_v6" {
		t.Errorf("LicenseID = %q, want lic_v6", lic.Claims.LicenseID)
	}
	if lic.Claims.Tier != TierPro {
		t.Errorf("Tier = %q, want pro", lic.Claims.Tier)
	}
	if lic.Claims.MaxAgents != 15 {
		t.Errorf("MaxAgents = %d, want 15", lic.Claims.MaxAgents)
	}

	// Verify activation state was set.
	if !svc.IsActivated() {
		t.Error("expected IsActivated=true after exchange")
	}
	state := svc.GetActivationState()
	if state.InstallationID != "inst_v6" {
		t.Errorf("InstallationID = %q, want inst_v6", state.InstallationID)
	}
	// LicenseID should come from JWT claim, not migration response.
	if state.LicenseID != "lic_v6" {
		t.Errorf("LicenseID = %q, want lic_v6", state.LicenseID)
	}
}

func TestServiceExchangeNoClient(t *testing.T) {
	svc := NewService()
	_, err := svc.ExchangeLegacyLicense("legacy.jwt.here")
	if err == nil {
		t.Fatal("expected error when no client configured")
	}
}
