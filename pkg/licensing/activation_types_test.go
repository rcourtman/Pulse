package licensing

import (
	"encoding/json"
	"testing"
	"time"
)

func TestGrantClaimsToClaims(t *testing.T) {
	tests := []struct {
		name          string
		gc            *GrantClaims
		wantTier      Tier
		wantSubState  SubscriptionState
		wantLicenseID string
		wantEmail     string
		wantFeatures  []string
		wantMaxGuests int
		wantMaxUsers  int
	}{
		{
			name: "active state with email",
			gc: &GrantClaims{
				LicenseID:      "lic_123",
				InstallationID: "inst_abc",
				State:          "active",
				Tier:           "pro",
				Email:          "user@example.com",
				Features:       []string{"ai_patrol", "relay"},
				MaxGuests:      5,
				MaxUsers:       3,
				IssuedAt:       time.Now().Unix(),
				ExpiresAt:      time.Now().Add(72 * time.Hour).Unix(),
			},
			wantTier:      TierPro,
			wantSubState:  SubStateActive,
			wantLicenseID: "lic_123",
			wantEmail:     "user@example.com",
			wantFeatures:  []string{"ai_patrol", "relay"},
			wantMaxGuests: 5,
			wantMaxUsers:  3,
		},
		{
			name: "past_due maps to grace",
			gc: &GrantClaims{
				LicenseID: "lic_456",
				State:     "past_due",
				Tier:      "relay",
				IssuedAt:  time.Now().Unix(),
				ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
			},
			wantTier:      TierRelay,
			wantSubState:  SubStateGrace,
			wantLicenseID: "lic_456",
		},
		{
			name: "grace maps to grace",
			gc: &GrantClaims{
				LicenseID: "lic_789",
				State:     "grace",
				Tier:      "pro",
				IssuedAt:  time.Now().Unix(),
				ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
			},
			wantTier:      TierPro,
			wantSubState:  SubStateGrace,
			wantLicenseID: "lic_789",
		},
		{
			name: "unknown state defaults to suspended (fail-closed)",
			gc: &GrantClaims{
				LicenseID: "lic_bad",
				State:     "unknown_state",
				Tier:      "pro",
				IssuedAt:  time.Now().Unix(),
				ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
			},
			wantTier:      TierPro,
			wantSubState:  SubStateSuspended,
			wantLicenseID: "lic_bad",
		},
		{
			name: "empty state defaults to suspended (fail-closed)",
			gc: &GrantClaims{
				LicenseID: "lic_empty",
				State:     "",
				Tier:      "pro",
				IssuedAt:  time.Now().Unix(),
				ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
			},
			wantTier:      TierPro,
			wantSubState:  SubStateSuspended,
			wantLicenseID: "lic_empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := grantClaimsToClaims(tt.gc)
			if c.Tier != tt.wantTier {
				t.Errorf("Tier = %q, want %q", c.Tier, tt.wantTier)
			}
			if c.SubState != tt.wantSubState {
				t.Errorf("SubState = %q, want %q", c.SubState, tt.wantSubState)
			}
			if c.LicenseID != tt.wantLicenseID {
				t.Errorf("LicenseID = %q, want %q", c.LicenseID, tt.wantLicenseID)
			}
			if c.Email != tt.wantEmail {
				t.Errorf("Email = %q, want %q", c.Email, tt.wantEmail)
			}
			if len(c.Features) != len(tt.wantFeatures) {
				t.Errorf("Features length = %d, want %d", len(c.Features), len(tt.wantFeatures))
			} else {
				for i, f := range c.Features {
					if f != tt.wantFeatures[i] {
						t.Errorf("Features[%d] = %q, want %q", i, f, tt.wantFeatures[i])
					}
				}
			}
			if c.MaxGuests != tt.wantMaxGuests {
				t.Errorf("MaxGuests = %d, want %d", c.MaxGuests, tt.wantMaxGuests)
			}
			if c.MaxUsers != tt.wantMaxUsers {
				t.Errorf("MaxUsers = %d, want %d", c.MaxUsers, tt.wantMaxUsers)
			}
		})
	}
}

// TestGrantMaxUsersFlowsToUserLimitEnforcement is the end-to-end instance
// proof for the max_users seat-limit chain: a grant JWT whose payload carries
// max_users=3 (raw server-shaped JSON, not this repo's struct tags) yields
// MaxUsersLimitFromLicense()==3, and a grant without the claim stays 0
// (unlimited), keeping the change inert for every existing license.
func TestGrantMaxUsersFlowsToUserLimitEnforcement(t *testing.T) {
	t.Run("grant with max_users enforces the seat limit", func(t *testing.T) {
		jwt := makeUnsignedTestJWT(t, `{
			"lid": "lic_business_seats",
			"st": "active",
			"tier": "business",
			"plan": "price_business_annual",
			"max_users": 3
		}`)
		gc, err := parseGrantJWTUnsafe(jwt)
		if err != nil {
			t.Fatalf("parseGrantJWTUnsafe: %v", err)
		}
		lic := grantClaimsToLicense(gc, jwt)
		if got := MaxUsersLimitFromLicense(lic); got != 3 {
			t.Fatalf("MaxUsersLimitFromLicense = %d, want 3", got)
		}
	})

	t.Run("grant without max_users stays unlimited", func(t *testing.T) {
		jwt := makeUnsignedTestJWT(t, `{
			"lid": "lic_pro_unlimited",
			"st": "active",
			"tier": "pro",
			"plan": "price_pro_annual"
		}`)
		gc, err := parseGrantJWTUnsafe(jwt)
		if err != nil {
			t.Fatalf("parseGrantJWTUnsafe: %v", err)
		}
		lic := grantClaimsToLicense(gc, jwt)
		if got := MaxUsersLimitFromLicense(lic); got != 0 {
			t.Fatalf("MaxUsersLimitFromLicense = %d, want 0 (unlimited)", got)
		}
	})
}

func TestGrantClaimsToLicense(t *testing.T) {
	t.Run("basic license from grant", func(t *testing.T) {
		gc := &GrantClaims{
			LicenseID: "lic_test",
			State:     "active",
			Tier:      "pro",
			Features:  []string{"relay", "ai_patrol"},
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
		}

		lic := grantClaimsToLicense(gc, "fake.jwt.token")
		if lic == nil {
			t.Fatal("expected non-nil license")
		}
		if lic.Raw != "fake.jwt.token" {
			t.Errorf("Raw = %q, want %q", lic.Raw, "fake.jwt.token")
		}
		if lic.Claims.Tier != TierPro {
			t.Errorf("Tier = %q, want %q", lic.Claims.Tier, TierPro)
		}
		if lic.GracePeriodEnd != nil {
			t.Error("GracePeriodEnd should be nil when no grace_until")
		}
	})

	t.Run("license with grace_until", func(t *testing.T) {
		graceUntil := time.Now().Add(48 * time.Hour).Unix()
		gc := &GrantClaims{
			LicenseID:  "lic_grace",
			State:      "grace",
			Tier:       "relay",
			IssuedAt:   time.Now().Unix(),
			ExpiresAt:  time.Now().Add(72 * time.Hour).Unix(),
			GraceUntil: graceUntil,
		}

		lic := grantClaimsToLicense(gc, "grace.jwt.token")
		if lic.GracePeriodEnd == nil {
			t.Fatal("expected GracePeriodEnd to be set")
		}
		if lic.GracePeriodEnd.Unix() != graceUntil {
			t.Errorf("GracePeriodEnd = %d, want %d", lic.GracePeriodEnd.Unix(), graceUntil)
		}
	})
}

func TestGrantClaimsToClaimsWithContinuityDoesNotSurfaceRetiredMonitoredSystemLimit(t *testing.T) {
	gc := &GrantClaims{
		LicenseID: "lic_floor",
		State:     "active",
		Tier:      "pro",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	}

	claims := grantClaimsToClaimsWithContinuity(gc, ActivationContinuity{
		LegacyMigration: true,
	})

	if !claims.CoreMonitoringUncapped {
		t.Fatal("expected grant-backed self-hosted continuity claims to carry the uncapped core monitoring marker")
	}
	if _, ok := claims.Limits[MaxMonitoredSystemsLicenseGateKey]; ok {
		t.Fatalf("Limits[%q] present, want absent for uncapped self-hosted Pro", MaxMonitoredSystemsLicenseGateKey)
	}
	if got, ok := claims.EffectiveLimits()[MaxMonitoredSystemsLicenseGateKey]; ok {
		t.Fatalf("EffectiveLimits()[max_monitored_systems] = %d present, want absent (uncapped self-hosted)", got)
	}
}

func TestGrantClaimsToClaimsCanonicalizesCloudPlanAtEntitlementBoundary(t *testing.T) {
	gc := &GrantClaims{
		LicenseID:  "lic_cloud",
		State:      "active",
		Tier:       string(TierCloud),
		PlanKey:    "cloud_v1",
		IssuedAt:   time.Now().Unix(),
		ExpiresAt:  time.Now().Add(72 * time.Hour).Unix(),
		GraceUntil: 0,
	}

	claims := grantClaimsToClaims(gc)
	if got := claims.EntitlementPlanVersion(); got != "cloud_starter" {
		t.Fatalf("EntitlementPlanVersion()=%q, want %q", got, "cloud_starter")
	}
	if got := claims.EntitlementSubscriptionState(); got != SubStateActive {
		t.Fatalf("EntitlementSubscriptionState()=%q, want %q", got, SubStateActive)
	}
}

func TestExchangeLegacyLicenseRequestJSONCompatibility(t *testing.T) {
	req := ExchangeLegacyLicenseRequest{
		LegacyLicenseKey:    "header.payload.signature",
		InstanceName:        "pulse-node",
		InstanceFingerprint: "fp-123",
		ClientVersion:       "6.0.0-rc.2",
		Runtime:             CloneRuntimeIdentity(ProRuntimeIdentity()),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal(raw) error = %v", err)
	}
	if got := raw["legacy_license_token"]; got != "header.payload.signature" {
		t.Fatalf("legacy_license_token = %v, want header.payload.signature", got)
	}
	if _, hasLegacyKey := raw["legacy_license_key"]; hasLegacyKey {
		t.Fatal("legacy_license_key should not be emitted by MarshalJSON")
	}
	runtime, ok := raw["runtime"].(map[string]any)
	if !ok {
		t.Fatalf("runtime = %#v, want object", raw["runtime"])
	}
	if got := runtime["build"]; got != RuntimeBuildPro {
		t.Fatalf("runtime.build = %v, want %s", got, RuntimeBuildPro)
	}

	for _, field := range []string{"legacy_license_token", "legacy_license_key"} {
		t.Run("unmarshal "+field, func(t *testing.T) {
			payload := map[string]any{
				field:                  "header.payload.signature",
				"instance_name":        "pulse-node",
				"instance_fingerprint": "fp-123",
				"client_version":       "6.0.0-rc.2",
				"runtime": map[string]any{
					"build": RuntimeBuildCommunity,
				},
			}
			body, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("json.Marshal(payload) error = %v", err)
			}

			var decoded ExchangeLegacyLicenseRequest
			if err := json.Unmarshal(body, &decoded); err != nil {
				t.Fatalf("json.Unmarshal(decoded) error = %v", err)
			}
			if decoded.LegacyLicenseKey != "header.payload.signature" {
				t.Fatalf("LegacyLicenseKey = %q, want header.payload.signature", decoded.LegacyLicenseKey)
			}
			if decoded.InstanceFingerprint != "fp-123" {
				t.Fatalf("InstanceFingerprint = %q, want fp-123", decoded.InstanceFingerprint)
			}
			if decoded.Runtime == nil || decoded.Runtime.Build != RuntimeBuildCommunity {
				t.Fatalf("Runtime build = %#v, want %q", decoded.Runtime, RuntimeBuildCommunity)
			}
		})
	}
}

func TestParseGrantJWTUnsafe(t *testing.T) {
	tests := []struct {
		name    string
		jwt     string
		wantErr bool
		errMsg  string
		check   func(t *testing.T, gc *GrantClaims)
	}{
		{
			name:    "invalid - not a JWT",
			jwt:     "not-a-jwt",
			wantErr: true,
			errMsg:  "expected 3 parts",
		},
		{
			name:    "invalid - too few parts",
			jwt:     "header.payload",
			wantErr: true,
			errMsg:  "expected 3 parts",
		},
		{
			name:    "invalid - bad base64",
			jwt:     "header.!!!invalid!!!.signature",
			wantErr: true,
			errMsg:  "decode grant payload",
		},
		{
			name:    "invalid - bad JSON",
			jwt:     makeUnsignedTestJWT(t, "not json"),
			wantErr: true,
			errMsg:  "unmarshal grant claims",
		},
		{
			name:    "invalid - missing license ID",
			jwt:     makeUnsignedTestGrantJWT(t, &GrantClaims{Tier: "pro"}),
			wantErr: true,
			errMsg:  "grant missing license ID",
		},
		{
			name:    "invalid - missing tier",
			jwt:     makeUnsignedTestGrantJWT(t, &GrantClaims{LicenseID: "lic_123"}),
			wantErr: true,
			errMsg:  "grant missing tier",
		},
		{
			name: "valid grant",
			jwt: makeUnsignedTestGrantJWT(t, &GrantClaims{
				LicenseID:      "lic_test",
				InstallationID: "inst_abc",
				State:          "active",
				Tier:           "pro",
				Features:       []string{"relay"},
				IssuedAt:       1000,
				ExpiresAt:      2000,
			}),
			check: func(t *testing.T, gc *GrantClaims) {
				if gc.LicenseID != "lic_test" {
					t.Errorf("LicenseID = %q, want %q", gc.LicenseID, "lic_test")
				}
				if gc.Tier != "pro" {
					t.Errorf("Tier = %q, want %q", gc.Tier, "pro")
				}
				if gc.State != "active" {
					t.Errorf("State = %q, want %q", gc.State, "active")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gc, err := parseGrantJWTUnsafe(tt.jwt)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, gc)
			}
		})
	}
}

func TestSplitJWT(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  int
	}{
		{"three parts", "a.b.c", 3},
		{"one part", "abc", 1},
		{"two parts", "a.b", 2},
		{"four parts", "a.b.c.d", 4},
		{"empty", "", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := splitJWT(tt.token)
			if len(parts) != tt.want {
				t.Errorf("splitJWT(%q) = %d parts, want %d", tt.token, len(parts), tt.want)
			}
		})
	}
}

func TestLicenseServerError(t *testing.T) {
	e := &LicenseServerError{
		StatusCode: 401,
		Code:       "invalid_token",
		Message:    "Token expired",
	}
	got := e.Error()
	want := "invalid_token: Token expired"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// --- test helpers ---

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
