package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

// These synthetic settings cover every persisted nested field omitted from the
// detail DTO, rather than copying the implementation's preservation allowlist.
func unrepresentedSSOFixture(kind config.SSOProviderType) config.SSOProvider {
	return config.SSOProvider{
		ID: "advanced", Name: "Advanced SSO", Type: kind,
		AllowedGroups: []string{"operators"}, AllowedDomains: []string{"example.test"},
		AllowedEmails: []string{"owner@example.test"}, GroupsClaim: "groups",
		GroupRoleMappings: map[string]string{"operators": "admin"},
		OIDC: &config.OIDCProviderConfig{
			IssuerURL: "https://idp.example.test/oidc", ClientID: "pulse", ClientSecret: "synthetic-oidc-secret", ClientSecretSet: true,
			RedirectURL: "https://pulse.example.test/callback", LogoutURL: "https://idp.example.test/logout", Scopes: []string{"openid", "email"},
			UsernameClaim: "preferred_username", EmailClaim: "mail", CABundle: "/synthetic/trust.pem",
		},
		SAML: &config.SAMLProviderConfig{
			IDPSSOURL: "https://idp.example.test/sso", IDPEntityID: "https://idp.example.test", IDPCertificate: "synthetic-idp-certificate",
			SPEntityID: "https://pulse.example.test", SignRequests: true, UsernameAttr: "username", EmailAttr: "email", GroupsAttr: "groups",
			IDPSLOURL: "https://idp.example.test/slo", IDPCertFile: "/synthetic/idp.pem", IDPIssuer: "synthetic-issuer",
			SPACSPath: "/synthetic/acs", SPMetadataPath: "/synthetic/metadata", SPCertificate: "synthetic-sp-certificate", SPPrivateKey: "synthetic-private-key-canary",
			SPCertFile: "/synthetic/sp.pem", SPKeyFile: "/synthetic/sp.key", WantAssertionsSigned: true, AllowUnencrypted: true,
			FirstNameAttr: "given_name", LastNameAttr: "family_name", NameIDFormat: "synthetic-name-format", ForceAuthn: true, RelayStateTemplate: "synthetic-relay-state",
		},
	}
}

func unrepresentedSSOFields(kind config.SSOProviderType) []reflect.StructField {
	var runtime, response reflect.Type
	if kind == config.SSOProviderTypeOIDC {
		runtime, response = reflect.TypeOf(config.OIDCProviderConfig{}), reflect.TypeOf(SSOProviderOIDCResponse{})
	} else {
		runtime, response = reflect.TypeOf(config.SAMLProviderConfig{}), reflect.TypeOf(SSOProviderSAMLResponse{})
	}
	represented := map[string]bool{"clientSecret": true, "-": true}
	for i := 0; i < response.NumField(); i++ {
		represented[strings.Split(response.Field(i).Tag.Get("json"), ",")[0]] = true
	}
	var hidden []reflect.StructField
	for i := 0; i < runtime.NumField(); i++ {
		field := runtime.Field(i)
		if !represented[strings.Split(field.Tag.Get("json"), ",")[0]] {
			hidden = append(hidden, field)
		}
	}
	return hidden
}

func putUnrepresentedSSO(t *testing.T, router *Router, payload interface{}) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, "/api/security/sso/providers/advanced", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	router.handleSSOProvider(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "synthetic-oidc-secret")
	require.NotContains(t, rec.Body.String(), "synthetic-private-key-canary")
	return rec
}

func seedUnrepresentedSSO(t *testing.T, kind config.SSOProviderType) (*Router, config.SSOProvider) {
	t.Helper()
	router, _ := setupTestRouter(t)
	provider := unrepresentedSSOFixture(kind)
	// Only the selected protocol is persisted in ordinary configurations.
	if kind == config.SSOProviderTypeOIDC {
		provider.SAML = nil
	} else {
		provider.OIDC = nil
	}
	require.NoError(t, router.ssoConfig.AddProvider(provider))
	require.NoError(t, router.saveSSOConfig())
	return router, provider
}

func TestSSOProviderUnrepresentedFields(t *testing.T) {
	for _, kind := range []config.SSOProviderType{config.SSOProviderTypeOIDC, config.SSOProviderTypeSAML} {
		t.Run(string(kind), func(t *testing.T) {
			hidden := unrepresentedSSOFields(kind)
			wantCount := 3
			if kind == config.SSOProviderTypeSAML {
				wantCount = 16
			}
			require.Len(t, hidden, wantCount)
			for _, mode := range []string{"detail round trip", "toggle", "null config", "empty config"} {
				t.Run(mode, func(t *testing.T) {
					router, original := seedUnrepresentedSSO(t, kind)
					snapshot := router.ensureSSOConfig()
					before, err := json.Marshal(snapshot)
					require.NoError(t, err)
					payload := map[string]interface{}{}
					detail := providerToDetailResponse(&original, router.config.PublicURL, httptest.NewRequest("GET", "/", nil))
					raw, err := json.Marshal(detail)
					require.NoError(t, err)
					require.NoError(t, json.Unmarshal(raw, &payload))
					payload["name"] = "Renamed SSO"
					switch mode {
					case "toggle":
						delete(payload, string(kind))
					case "null config":
						payload[string(kind)] = nil
					case "empty config":
						payload[string(kind)] = map[string]interface{}{}
					}
					putUnrepresentedSSO(t, router, payload)
					loaded, err := router.persistence.LoadSSOConfig()
					require.NoError(t, err)
					stored := loaded.GetProvider("advanced")
					require.Equal(t, "Renamed SSO", stored.Name)
					var actual, prior reflect.Value
					if kind == config.SSOProviderTypeOIDC {
						actual, prior = reflect.ValueOf(stored.OIDC).Elem(), reflect.ValueOf(original.OIDC).Elem()
						require.Equal(t, original.OIDC.ClientSecret, stored.OIDC.ClientSecret)
					} else {
						actual, prior = reflect.ValueOf(stored.SAML).Elem(), reflect.ValueOf(original.SAML).Elem()
					}
					for _, field := range hidden {
						require.Equal(t, prior.FieldByIndex(field.Index).Interface(), actual.FieldByIndex(field.Index).Interface(), field.Name)
					}
					after, err := json.Marshal(snapshot)
					require.NoError(t, err)
					require.Equal(t, string(before), string(after), "published prior snapshot mutated")
					// A fresh router reads the same saved advanced settings, without exposing them.
					fresh := &Router{persistence: router.persistence, config: router.config}
					require.Equal(t, stored, fresh.ensureSSOConfig().GetProvider("advanced"))
					rec := httptest.NewRecorder()
					fresh.handleGetSSOProvider(rec, httptest.NewRequest("GET", "/", nil), "advanced")
					require.NotContains(t, rec.Body.String(), "synthetic-private-key-canary")
					require.NotContains(t, rec.Body.String(), "synthetic-oidc-secret")
				})
			}
			for _, field := range hidden {
				key := strings.Split(field.Tag.Get("json"), ",")[0]
				for _, mode := range []string{"clear", "null", "replace folded"} {
					t.Run(key+"/"+mode, func(t *testing.T) {
						router, _ := seedUnrepresentedSSO(t, kind)
						var value interface{} = ""
						if field.Type.Kind() == reflect.Bool {
							value = false
						}
						want := value
						if mode == "null" {
							value = nil
						}
						if mode == "replace folded" {
							key = strings.ToUpper(key)
							if field.Type.Kind() == reflect.String {
								value = "https://replacement.example.test/value"
								want = value
							}
						}
						payload := map[string]interface{}{"name": "Advanced SSO", "type": kind, string(kind): map[string]interface{}{key: value}}
						putUnrepresentedSSO(t, router, payload)
						saved, err := router.persistence.LoadSSOConfig()
						require.NoError(t, err)
						var actual reflect.Value
						if kind == config.SSOProviderTypeOIDC {
							actual = reflect.ValueOf(saved.GetProvider("advanced").OIDC).Elem()
						} else {
							actual = reflect.ValueOf(saved.GetProvider("advanced").SAML).Elem()
						}
						require.Equal(t, want, actual.FieldByIndex(field.Index).Interface())
					})
				}
			}
		})
	}
}

func TestSSOProviderUnrepresentedFieldsRespectProtocolChanges(t *testing.T) {
	for _, kind := range []config.SSOProviderType{config.SSOProviderTypeOIDC, config.SSOProviderTypeSAML} {
		t.Run(string(kind), func(t *testing.T) {
			router, _ := setupTestRouter(t)
			opposite := config.SSOProviderTypeSAML
			if kind == opposite {
				opposite = config.SSOProviderTypeOIDC
			}
			// Even an old/inactive protocol block must not donate settings or secrets
			// when switching types. Only this switch's supplied settings are accepted.
			prior := unrepresentedSSOFixture(opposite)
			require.NoError(t, router.ssoConfig.AddProvider(prior))
			require.NoError(t, router.saveSSOConfig())
			nested := map[string]interface{}{}
			if kind == config.SSOProviderTypeOIDC {
				nested["issuerUrl"] = "https://new.example.test"
				nested["clientId"] = "new-client"
			} else {
				nested["idpSsoUrl"] = "https://new.example.test/sso"
			}
			putUnrepresentedSSO(t, router, map[string]interface{}{"name": "New protocol", "type": kind, string(kind): nested})
			saved, err := router.persistence.LoadSSOConfig()
			require.NoError(t, err)
			p := saved.GetProvider("advanced")
			var value reflect.Value
			if kind == config.SSOProviderTypeOIDC {
				value = reflect.ValueOf(p.OIDC).Elem()
				require.Empty(t, p.OIDC.ClientSecret)
				require.False(t, p.OIDC.ClientSecretSet)
				require.Nil(t, p.SAML)
			} else {
				value = reflect.ValueOf(p.SAML).Elem()
				require.Nil(t, p.OIDC)
			}
			for _, field := range unrepresentedSSOFields(kind) {
				require.True(t, value.FieldByIndex(field.Index).IsZero(), field.Name)
			}
		})
	}
}

func TestSSOProviderUnrepresentedAliasesAndBooleans(t *testing.T) {
	router, _ := seedUnrepresentedSSO(t, config.SSOProviderTypeSAML)
	// JSON aliases retain the admission decoder's Unicode and escape semantics.
	req := httptest.NewRequest("PUT", "/api/security/sso/providers/advanced", strings.NewReader(`{"name":"Advanced SSO","type":"saml","SAML":{"wantAſſertionsSigned":false,"spPrivateKe\u0079":""}}`))
	rec := httptest.NewRecorder()
	router.handleSSOProvider(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, router.ensureSSOConfig().GetProvider("advanced").SAML.WantAssertionsSigned)
	require.Empty(t, router.ensureSSOConfig().GetProvider("advanced").SAML.SPPrivateKey)
	putUnrepresentedSSO(t, router, map[string]interface{}{"name": "Advanced SSO", "type": "saml", "saml": map[string]interface{}{"wantAssertionsSigned": true, "allowUnencrypted": false, "forceAuthn": false}})
	p := router.ensureSSOConfig().GetProvider("advanced")
	require.True(t, p.SAML.WantAssertionsSigned)
	require.False(t, p.SAML.AllowUnencrypted)
	require.False(t, p.SAML.ForceAuthn)
}
