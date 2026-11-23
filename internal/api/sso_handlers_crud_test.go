package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeProviderName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Normal Name", "Normal Name"},
		{" Trimmed Space ", "Trimmed Space"},
		{"Control\x00Char", "ControlChar"},
		{strings.Repeat("a", 200), strings.Repeat("a", 128)}, // Truncation
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizeProviderName(tt.input))
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		url      string
		schemes  []string
		expected bool
	}{
		{"https://example.com", []string{"https"}, true},
		{"http://example.com", []string{"https"}, false},
		{"https://user:pass@example.com", []string{"https"}, false},
		{"ftp://example.com", []string{"http", "https"}, false},
		{"data:image/png;base64,abc", []string{"data"}, true},
		{"not-a-url", []string{"https"}, false},
		{"", []string{"https"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			assert.Equal(t, tt.expected, validateURL(tt.url, tt.schemes))
		})
	}
}

// Test CRUD Handlers

func setupTestRouter(t *testing.T) (*Router, string) {
	tempDir := t.TempDir()
	persistence := config.NewConfigPersistence(tempDir)

	// Create a dummy config
	cfg := &config.Config{
		DataPath:  tempDir,
		PublicURL: "http://localhost:8080",
	}

	// Manual Router initialization for testing
	router := &Router{
		persistence: persistence,
		ssoConfig:   config.NewSSOConfig(),
		config:      cfg,
		// samlManager is needed if we enable saml provider, initialized here if strict dependencies allow
		samlManager: NewSAMLServiceManager("http://localhost:8080"),
		oidcManager: NewOIDCServiceManager(),
	}

	// Save initial empty config to disk so persistence works
	err := persistence.SaveSSOConfig(router.ssoConfig)
	require.NoError(t, err)

	return router, tempDir
}

func TestSSOProviderCRUD(t *testing.T) {
	router, _ := setupTestRouter(t)

	// 1. Create Provider
	newProvider := config.SSOProvider{
		ID:   "test-oidc",
		Name: "Test OIDC",
		Type: config.SSOProviderTypeOIDC,
		OIDC: &config.OIDCProviderConfig{
			IssuerURL: "https://accounts.google.com",
			ClientID:  "client-id",
		},
	}

	body, _ := json.Marshal(newProvider)
	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers", bytes.NewReader(body))
	// Add context with org ID if needed by audit logging (LogAuditEventForTenant)
	// Mocking GetOrgID might be needed closer to middleware,
	// but let's see if it executes without auth middleware first.
	// LogAuditEventForTenant usually fails gracefully or just logs.

	w := httptest.NewRecorder()
	router.handleCreateSSOProvider(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var created SSOProviderResponse
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)
	assert.Equal(t, "test-oidc", created.ID)
	assert.Equal(t, "Test OIDC", created.Name)

	// 2. Get Provider
	req = httptest.NewRequest(http.MethodGet, "/api/security/sso/providers/test-oidc", nil)
	w = httptest.NewRecorder()
	router.handleSSOProvider(w, req) // This routes to handleGet/Update/Delete

	require.Equal(t, http.StatusOK, w.Code)
	var fetched SSOProviderResponse
	err = json.Unmarshal(w.Body.Bytes(), &fetched)
	require.NoError(t, err)
	assert.Equal(t, created.ID, fetched.ID)

	// 3. Update Provider
	updatePayload := config.SSOProvider{
		ID:   "test-oidc", // ID must match
		Name: "Updated Name",
		Type: config.SSOProviderTypeOIDC,
		OIDC: &config.OIDCProviderConfig{
			IssuerURL: "https://accounts.google.com",
			ClientID:  "client-id",
		},
	}
	body, _ = json.Marshal(updatePayload)
	req = httptest.NewRequest(http.MethodPut, "/api/security/sso/providers/test-oidc", bytes.NewReader(body))
	w = httptest.NewRecorder()
	router.handleSSOProvider(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var updated SSOProviderResponse
	_ = json.Unmarshal(w.Body.Bytes(), &updated)
	assert.Equal(t, "Updated Name", updated.Name)

	// Verify persistence
	loadedConfig, err := router.persistence.LoadSSOConfig()
	require.NoError(t, err)
	stored := loadedConfig.GetProvider("test-oidc")
	require.NotNil(t, stored)
	assert.Equal(t, "Updated Name", stored.Name)

	// 4. Delete Provider
	req = httptest.NewRequest(http.MethodDelete, "/api/security/sso/providers/test-oidc", nil)
	w = httptest.NewRecorder()
	router.handleSSOProvider(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)

	// Verify deletion
	loadedConfig, err = router.persistence.LoadSSOConfig()
	require.NoError(t, err)
	assert.Nil(t, loadedConfig.GetProvider("test-oidc"))
}

func TestSSOProviderDetailRoundTripPreservesOIDCEditableSettings(t *testing.T) {
	router, _ := setupTestRouter(t)

	provider := config.SSOProvider{
		ID:          "corp-oidc",
		Name:        "Corporate OIDC",
		Type:        config.SSOProviderTypeOIDC,
		Enabled:     true,
		DisplayName: "Sign in with Corporate SSO",
		Priority:    2,
		AllowedGroups: []string{
			"admins",
			"operators",
		},
		AllowedDomains: []string{"example.com"},
		AllowedEmails:  []string{"owner@example.com"},
		GroupsClaim:    "groups",
		GroupRoleMappings: map[string]string{
			"admins": "admin",
		},
		OIDC: &config.OIDCProviderConfig{
			IssuerURL:    "https://idp.example.com/realms/pulse",
			ClientID:     "pulse-client",
			ClientSecret: "super-secret",
			RedirectURL:  "https://pulse.example.com/api/oidc/corp-oidc/callback",
			LogoutURL:    "https://idp.example.com/logout",
			Scopes:       []string{"openid", "profile", "email", "groups"},
		},
	}
	require.NoError(t, router.ssoConfig.AddProvider(provider))
	require.NoError(t, router.saveSSOConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/security/sso/providers/corp-oidc", nil)
	w := httptest.NewRecorder()
	router.handleSSOProvider(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "super-secret")

	var detail SSOProviderResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &detail))
	require.NotNil(t, detail.OIDC)
	assert.Equal(t, "https://idp.example.com/realms/pulse", detail.OIDC.IssuerURL)
	assert.Equal(t, "pulse-client", detail.OIDC.ClientID)
	assert.Equal(t, "https://pulse.example.com/api/oidc/corp-oidc/callback", detail.OIDC.RedirectURL)
	assert.Equal(t, "https://idp.example.com/logout", detail.OIDC.LogoutURL)
	assert.Equal(t, []string{"openid", "profile", "email", "groups"}, detail.OIDC.Scopes)
	assert.True(t, detail.OIDC.ClientSecretSet)
	assert.Equal(t, "groups", detail.GroupsClaim)
	assert.Equal(t, map[string]string{"admins": "admin"}, detail.GroupRoleMappings)

	body, err := json.Marshal(detail)
	require.NoError(t, err)
	req = httptest.NewRequest(http.MethodPut, "/api/security/sso/providers/corp-oidc", bytes.NewReader(body))
	w = httptest.NewRecorder()
	router.handleSSOProvider(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	loadedConfig, err := router.persistence.LoadSSOConfig()
	require.NoError(t, err)
	stored := loadedConfig.GetProvider("corp-oidc")
	require.NotNil(t, stored)
	require.NotNil(t, stored.OIDC)
	assert.Equal(t, "https://idp.example.com/realms/pulse", stored.OIDC.IssuerURL)
	assert.Equal(t, "pulse-client", stored.OIDC.ClientID)
	assert.Equal(t, "super-secret", stored.OIDC.ClientSecret)
	assert.True(t, stored.OIDC.ClientSecretSet)
	assert.Equal(t, "https://pulse.example.com/api/oidc/corp-oidc/callback", stored.OIDC.RedirectURL)
	assert.Equal(t, "https://idp.example.com/logout", stored.OIDC.LogoutURL)
	assert.Equal(t, []string{"openid", "profile", "email", "groups"}, stored.OIDC.Scopes)
	assert.Equal(t, []string{"admins", "operators"}, stored.AllowedGroups)
	assert.Equal(t, []string{"example.com"}, stored.AllowedDomains)
	assert.Equal(t, []string{"owner@example.com"}, stored.AllowedEmails)
	assert.Equal(t, "groups", stored.GroupsClaim)
	assert.Equal(t, map[string]string{"admins": "admin"}, stored.GroupRoleMappings)
}

func TestSSOOIDCRejectsUnusableEnabledScopeWrites(t *testing.T) {
	f := newPresentationFixture(t, config.SSOProviderTypeOIDC)
	r, original := f.router, f.provider
	draft := original
	draft.ID, draft.Enabled = "legacy-draft", false
	draftOIDC := *original.OIDC
	draftOIDC.Scopes = []string{"groups"}
	draft.OIDC = &draftOIDC
	require.NoError(t, r.ssoConfig.AddProvider(draft))
	require.NoError(t, r.saveSSOConfig())
	t.Cleanup(func() {
		r.oidcManager.RemoveService("new-oidc")
		r.oidcManager.RemoveService(draft.ID)
	})

	file := filepath.Join(r.config.DataPath, "sso.enc")
	before, err := os.ReadFile(file)
	require.NoError(t, err)
	snapshot := r.ensureSSOConfig()
	live := r.oidcManager.GetService(original.ID)
	requests := f.requests.Load()
	newProvider := draft
	newProvider.ID, newProvider.Enabled = "new-oidc", true
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	edit := providerToDetailResponse(&original, r.config.PublicURL, req)
	edit.OIDC.Scopes = []string{"OpenID", "openid-extra", "groups\topenid"}
	enable := providerToResponse(&draft, r.config.PublicURL, req)
	enable.Enabled = true

	for _, tc := range []struct {
		action, method, suffix string
		body                   any
	}{
		{"add", http.MethodPost, "", newProvider},
		{"edit", http.MethodPut, "/" + original.ID, edit},
		{"enable-with-omitted-oidc", http.MethodPut, "/" + draft.ID, enable},
	} {
		t.Run(tc.action, func(t *testing.T) {
			rec := ssoScopeWrite(t, r, tc.method, tc.suffix, tc.body)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var reply APIError
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &reply))
			require.Contains(t, reply.ErrorMessage, "Include openid in Scopes")
			require.NotContains(t, rec.Body.String(), "openid-extra")
			require.NotContains(t, rec.Body.String(), original.OIDC.ClientSecret)
			require.Same(t, snapshot, r.currentSSOConfig())
			require.Same(t, live, r.oidcManager.GetService(original.ID))
			require.Nil(t, r.oidcManager.GetService(draft.ID))
			require.Equal(t, requests, f.requests.Load(), "refused write must not discover the issuer")
			after, err := os.ReadFile(file)
			require.NoError(t, err)
			require.Equal(t, before, after, "refused write must not persist")
		})
	}
}

func TestSSOOIDCScopeWritesKeepDefaultsAndLegacyRecovery(t *testing.T) {
	f := newPresentationFixture(t, config.SSOProviderTypeOIDC)
	r, original := f.router, f.provider
	legacy := original
	legacy.ID = "legacy-enabled"
	legacyOIDC := *original.OIDC
	legacyOIDC.Scopes = []string{"groups"}
	legacy.OIDC = &legacyOIDC
	require.NoError(t, r.ssoConfig.AddProvider(legacy))
	require.NoError(t, r.saveSSOConfig())
	t.Cleanup(func() { r.oidcManager.RemoveService(legacy.ID) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	readBack := func(id string, wantScopes []string, enabled bool) *config.SSOProvider {
		t.Helper()
		cfg, err := r.persistence.LoadSSOConfig()
		require.NoError(t, err)
		p := cfg.GetProvider(id)
		require.NotNil(t, p)
		require.Equal(t, wantScopes, p.OIDC.Scopes)
		require.Equal(t, enabled, p.Enabled)
		require.Equal(t, original.OIDC.ClientSecret, p.OIDC.ClientSecret)
		require.Equal(t, original.AllowedGroups, p.AllowedGroups)
		require.Equal(t, original.GroupRoleMappings, p.GroupRoleMappings)
		return p
	}
	save := func(p *config.SSOProvider, scopes []string, enabled bool) {
		t.Helper()
		edit := providerToDetailResponse(p, r.config.PublicURL, req)
		edit.Enabled, edit.OIDC.Scopes = enabled, scopes
		rec := ssoScopeWrite(t, r, http.MethodPut, "/"+p.ID, edit)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}

	// A legacy entry elsewhere cannot prevent a valid provider from saving.
	// A compound ASCII-space entry has the same wire scope as separate entries.
	custom := []string{"profile openid", "groups"}
	save(&original, custom, true)
	saved := readBack(original.ID, custom, true)
	readBack(legacy.ID, legacyOIDC.Scopes, true)
	require.Equal(t, custom, r.oidcManager.GetService(original.ID).oauth2Cfg.Scopes)

	// Explicit empty selection still uses the runtime defaults, without
	// manufacturing a custom array in persistence.
	save(saved, []string{}, true)
	readBack(original.ID, nil, true)
	require.Equal(t, []string{"openid", "profile", "email"}, r.oidcManager.GetService(original.ID).oauth2Cfg.Scopes)

	// Round-tripping a legacy selection doesn't rewrite it. Disabling it is
	// still possible, as is correcting and enabling it with exact new scopes.
	save(&legacy, legacyOIDC.Scopes, true)
	save(&legacy, legacyOIDC.Scopes, false)
	disabled := readBack(legacy.ID, legacyOIDC.Scopes, false)
	require.Nil(t, r.oidcManager.GetService(legacy.ID))
	corrected := []string{"openid", "groups"}
	save(disabled, corrected, true)
	readBack(legacy.ID, corrected, true)
	require.Equal(t, corrected, r.oidcManager.GetService(legacy.ID).oauth2Cfg.Scopes)
}

func ssoScopeWrite(t *testing.T, r *Router, method, suffix string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	req := httptest.NewRequest(method, "/api/security/sso/providers"+suffix, bytes.NewReader(data))
	rec := httptest.NewRecorder()
	if suffix == "" {
		r.handleSSOProviders(rec, req)
	} else {
		r.handleSSOProvider(rec, req)
	}
	return rec
}

func TestUpdateSSOProvider_EmptyValuesClearRestrictionFields(t *testing.T) {
	router, _ := setupTestRouter(t)

	provider := config.SSOProvider{
		ID:             "corp-oidc",
		Name:           "Corporate OIDC",
		Type:           config.SSOProviderTypeOIDC,
		Enabled:        true,
		AllowedGroups:  []string{"admins"},
		AllowedDomains: []string{"example.com"},
		AllowedEmails:  []string{"owner@example.com"},
		GroupsClaim:    "groups",
		GroupRoleMappings: map[string]string{
			"admins": "admin",
		},
		OIDC: &config.OIDCProviderConfig{
			IssuerURL:    "https://idp.example.com/realms/pulse",
			ClientID:     "pulse-client",
			ClientSecret: "super-secret",
		},
	}
	require.NoError(t, router.ssoConfig.AddProvider(provider))
	require.NoError(t, router.saveSSOConfig())

	// An intentionally emptied field in a PUT clears it; only the nested
	// config (absent from toggle payloads) and the client secret (never
	// echoed in reads) are preserved.
	body := `{"name":"Corporate OIDC","type":"oidc","enabled":true,"allowedGroups":[],"allowedDomains":[],"allowedEmails":[],"groupsClaim":"","groupRoleMappings":{}}`
	req := httptest.NewRequest(http.MethodPut, "/api/security/sso/providers/corp-oidc", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.handleSSOProvider(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	loadedConfig, err := router.persistence.LoadSSOConfig()
	require.NoError(t, err)
	stored := loadedConfig.GetProvider("corp-oidc")
	require.NotNil(t, stored)
	assert.Empty(t, stored.AllowedGroups)
	assert.Empty(t, stored.AllowedDomains)
	assert.Empty(t, stored.AllowedEmails)
	assert.Empty(t, stored.GroupsClaim)
	assert.Empty(t, stored.GroupRoleMappings)
	require.NotNil(t, stored.OIDC)
	assert.Equal(t, "https://idp.example.com/realms/pulse", stored.OIDC.IssuerURL)
	assert.Equal(t, "super-secret", stored.OIDC.ClientSecret)
}

func TestCreateSSOProvider_Validation(t *testing.T) {
	router, _ := setupTestRouter(t)

	tests := []struct {
		name       string
		provider   config.SSOProvider
		statusCode int
		errMsg     string
	}{
		{
			name: "invalid type",
			provider: config.SSOProvider{
				Name: "Bad Type",
				Type: "invalid",
			},
			statusCode: http.StatusBadRequest,
			errMsg:     "must be 'oidc' or 'saml'",
		},
		{
			name: "missing name",
			provider: config.SSOProvider{
				Type: config.SSOProviderTypeOIDC,
			},
			statusCode: http.StatusBadRequest,
			errMsg:     "Provider name is required",
		},
		{
			name: "oidc missing config",
			provider: config.SSOProvider{
				Name: "No Config",
				Type: config.SSOProviderTypeOIDC,
			},
			statusCode: http.StatusBadRequest, // Config validation inside config package, might return error
			// The handler checks validation manually too
		},
		{
			name: "invalid issuer url",
			provider: config.SSOProvider{
				Name: "Bad URL",
				Type: config.SSOProviderTypeOIDC,
				OIDC: &config.OIDCProviderConfig{
					IssuerURL: "not-a-url",
					ClientID:  "id",
				},
			},
			statusCode: http.StatusBadRequest,
			errMsg:     "Invalid OIDC issuer URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.provider)
			req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers", bytes.NewReader(body))
			w := httptest.NewRecorder()
			router.handleCreateSSOProvider(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
			if tt.errMsg != "" {
				assert.Contains(t, w.Body.String(), tt.errMsg)
			}
		})
	}
}

func TestHandleListSSOProviders(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Add a provider directly
	p := config.SSOProvider{
		ID: "p1", Name: "P1", Type: config.SSOProviderTypeOIDC,
		OIDC: &config.OIDCProviderConfig{IssuerURL: "https://a.com", ClientID: "c"},
	}
	_ = router.ssoConfig.AddProvider(p)

	req := httptest.NewRequest(http.MethodGet, "/api/security/sso/providers", nil)
	w := httptest.NewRecorder()
	router.handleSSOProviders(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var list SSOProvidersListResponse
	err := json.Unmarshal(w.Body.Bytes(), &list)
	require.NoError(t, err)
	assert.Len(t, list.Providers, 1)
	assert.Equal(t, "P1", list.Providers[0].Name)
}

func TestSSOProviderResponsesUseCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptySSOProvidersListResponse())
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))

	providers, ok := decoded["providers"].([]any)
	if !ok || len(providers) != 0 {
		t.Fatalf("expected providers to be an empty array, got %T (%v)", decoded["providers"], decoded["providers"])
	}

	payload, err = json.Marshal(EmptySSOProviderResponse())
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(payload, &decoded))

	for _, key := range []string{"allowedGroups", "allowedDomains", "allowedEmails"} {
		values, ok := decoded[key].([]any)
		if !ok || len(values) != 0 {
			t.Fatalf("expected %s to be an empty array, got %T (%v)", key, decoded[key], decoded[key])
		}
	}
}
