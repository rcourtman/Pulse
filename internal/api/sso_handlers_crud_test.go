package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ... (skipping unchanged parts until test)

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
		{"ftp://example.com", []string{"http", "https"}, false},
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
	json.Unmarshal(w.Body.Bytes(), &updated)
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
	router.ssoConfig.AddProvider(p)

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
