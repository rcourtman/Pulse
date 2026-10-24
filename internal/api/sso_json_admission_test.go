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
	"github.com/stretchr/testify/require"
)

func TestSSOAdminJSONRejectsAmbiguousControls(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		t.Run(operation, func(t *testing.T) {
			for _, tt := range []struct{ name, fields string }{
				{"identical groups", `"allowedGroups":["ops"],"allowedGroups":["ops"]`},
				{"cleared groups", `"allowedGroups":["ops"],"allowedGroups":[]`},
				{"escaped groups", `"allowedGroups":["ops"],"allowedGroup\u0073":[]`},
				{"folded groups", `"allowedGroups":["ops"],"ALLOWEDGROUPS":[]`},
				{"unicode folded groups", `"allowedGroups":["ops"],"allowedGroupſ":[]`},
				{"cleared domains", `"allowedDomains":["example.test"],"allowedDomains":[]`},
				{"cleared emails", `"allowedEmails":["owner@example.test"],"allowedEmails":[]`},
				{"groups claim", `"groupsClaim":"groups","groupsClaim":"other"`},
				{"whole mappings", `"groupRoleMappings":{"ops":"viewer"},"GROUPROLEMAPPINGS":{}`},
				{"mapping role", `"groupRoleMappings":{"ops":"viewer","op\u0073":"admin"}`},
				{"nested oidc secret", `"oidc":{"clientId":"pulse","clientSecret":"first","CLIENTSECRET":"second"}`},
				{"nested oidc issuer", `"oidc":{"issuerUrl":"https://first.example.test","issuerUrl":"https://second.example.test"}`},
				{"merged oidc", `"oidc":{"clientId":"pulse"},"OIDC":{"clientSecret":"second"}`},
				{"nested saml policy", `"saml":{"wantAssertionsSigned":true,"WANTASSERTIONSSIGNED":false}`},
				{"merged saml", `"saml":{"groupsAttr":"groups"},"SAML":{"groupsAttr":"other"}`},
				{"enabled switch", `"enabled":true,"ENABLED":false`},
			} {
				t.Run(tt.name, func(t *testing.T) {
					router, dir := setupTestRouter(t)
					if operation == "update" {
						seedRestrictedSSOProvider(t, router)
					}
					beforeConfig, err := json.Marshal(router.ensureSSOConfig())
					require.NoError(t, err)
					beforeFiles := ssoAdmissionFiles(t, dir)
					body := `{"id":"guarded","name":"Corporate SSO","type":"oidc",` + tt.fields + `}`
					// Each otherwise incomplete OIDC request gets one baseline object.
					if !strings.Contains(tt.fields, `"oidc"`) {
						body = body[:len(body)-1] + `,"oidc":{"clientId":"pulse"}}`
					}
					rec := serveSSOAdmission(router, operation, body)
					require.Equal(t, http.StatusBadRequest, rec.Code)
					require.Contains(t, rec.Body.String(), `"invalid_json"`)
					afterConfig, err := json.Marshal(router.ensureSSOConfig())
					require.NoError(t, err)
					require.Equal(t, string(beforeConfig), string(afterConfig), "published settings changed")
					require.Equal(t, beforeFiles, ssoAdmissionFiles(t, dir), "persisted settings changed")
				})
			}
		})
	}
}

func TestSSOAdminJSONRequiresCompleteBoundedObject(t *testing.T) {
	valid := `{"id":"guarded","name":"Corporate SSO","type":"oidc","oidc":{"clientId":"pulse"}}`
	for _, operation := range []string{"create", "update"} {
		t.Run(operation, func(t *testing.T) {
			for _, tt := range []struct{ name, body string }{
				{"null", `null`},
				{"array", `[]`},
				{"trailing document", valid + `{}`},
				{"oversized whitespace", valid + strings.Repeat(" ", maxRequestBodySize)},
				{"hidden suffix", valid + strings.Repeat(" ", maxRequestBodySize) + `{}`},
			} {
				t.Run(tt.name, func(t *testing.T) {
					router, dir := setupTestRouter(t)
					if operation == "update" {
						seedRestrictedSSOProvider(t, router)
					}
					before := ssoAdmissionFiles(t, dir)
					require.Equal(t, http.StatusBadRequest, serveSSOAdmission(router, operation, tt.body).Code)
					require.Equal(t, before, ssoAdmissionFiles(t, dir))
				})
			}
		})
	}
}

func TestSSOAdminJSONPreservesUniqueRequestsAndDeliberateClears(t *testing.T) {
	router, _ := setupTestRouter(t)
	body := `{"ID":"guarded","NAME":"Corporate SSO","TYPE":"oidc","OIDC":{"CLIENTID":"pulse","clientSecret":"fixture-secret"},"allowedGroup\u0073":["Ops"],"allowedDomains":["example.test"],"allowedEmails":["owner@example.test"],"groupsClaim":"groups","groupRoleMappings":{"Ops":"admin","ops":"viewer"},"future":{"ALLOWEDGROUPS":[],"allowedGroups":["ignored"]}}`
	require.Equal(t, http.StatusCreated, serveSSOAdmission(router, "create", body).Code)
	stored, err := router.persistence.LoadSSOConfig()
	require.NoError(t, err)
	require.Equal(t, []string{"Ops"}, stored.GetProvider("guarded").AllowedGroups)
	require.Equal(t, map[string]string{"Ops": "admin", "ops": "viewer"}, stored.GetProvider("guarded").GroupRoleMappings)

	// Blank secret saves keep the existing secret, but explicit access clears
	// remain a supported operation; never turn them into an accidental keep.
	clear := `{"name":"Corporate SSO","type":"oidc","oidc":{"clientId":"pulse","clientSecret":""},"allowedGroups":[],"allowedDomains":[],"allowedEmails":[],"groupsClaim":"","groupRoleMappings":{}}`
	rec := serveSSOAdmission(router, "update", clear)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "fixture-secret")
	stored, err = router.persistence.LoadSSOConfig()
	require.NoError(t, err)
	provider := stored.GetProvider("guarded")
	require.Empty(t, provider.AllowedGroups)
	require.Empty(t, provider.AllowedDomains)
	require.Empty(t, provider.AllowedEmails)
	require.Empty(t, provider.GroupRoleMappings)
	require.Equal(t, "fixture-secret", provider.OIDC.ClientSecret)

	require.Equal(t, http.StatusOK, serveSSOAdmission(router, "update", body).Code)
	stored, err = router.persistence.LoadSSOConfig()
	require.NoError(t, err)
	require.Equal(t, []string{"Ops"}, stored.GetProvider("guarded").AllowedGroups)
}

func seedRestrictedSSOProvider(t *testing.T, router *Router) {
	t.Helper()
	provider := config.SSOProvider{
		ID: "guarded", Name: "Corporate SSO", Type: config.SSOProviderTypeOIDC,
		AllowedGroups: []string{"ops"}, AllowedDomains: []string{"example.test"},
		AllowedEmails: []string{"owner@example.test"}, GroupsClaim: "groups",
		GroupRoleMappings: map[string]string{"ops": "viewer"},
		OIDC:              &config.OIDCProviderConfig{ClientID: "pulse", ClientSecret: "fixture-secret", ClientSecretSet: true},
	}
	require.NoError(t, router.ssoConfig.AddProvider(provider))
	require.NoError(t, router.saveSSOConfig())
}

func serveSSOAdmission(router *Router, operation, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	if operation == "create" {
		router.handleCreateSSOProvider(rec, httptest.NewRequest(http.MethodPost, "/api/security/sso/providers", bytes.NewBufferString(body)))
	} else {
		router.handleUpdateSSOProvider(rec, httptest.NewRequest(http.MethodPut, "/api/security/sso/providers/guarded", bytes.NewBufferString(body)), "guarded")
	}
	return rec
}

func ssoAdmissionFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	require.NoError(t, filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[path] = string(data)
		}
		return nil
	}))
	return files
}
