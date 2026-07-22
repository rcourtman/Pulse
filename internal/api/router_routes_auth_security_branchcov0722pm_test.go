package api

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

// These tests target the small pure converter functions in
// router_routes_auth_security.go that translate between the internal/api SSO
// DTOs and the config/extensions types. Each converter has a nil-input arm and
// a fully-populated arm; both are exercised with concrete inputs and asserted
// field-by-field. Slice/map clone independence is asserted where the converter
// performs a copy, and aliasing (where the converter assigns by reference) is
// documented via characterization assertions plus a note in the report.

// --- shared fixture helpers ---

func branchcov0722PMCertInfo(subject string) CertificateInfo {
	return CertificateInfo{
		Subject:   subject,
		Issuer:    "issuer-" + subject,
		NotBefore: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
		IsExpired: subject == "expired",
	}
}

func branchcov0722PMExtCertInfo(subject string) extensions.CertificateInfo {
	return extensions.CertificateInfo{
		Subject:   subject,
		Issuer:    "issuer-" + subject,
		NotBefore: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
		IsExpired: subject == "expired",
	}
}

// branchcov0722PMPopulatedSAML builds a fully-populated config.SAMLProviderConfig
// with a distinct value in every field so that a missing or misplaced copy is
// detectable.
func branchcov0722PMPopulatedCoreSAML() *config.SAMLProviderConfig {
	return &config.SAMLProviderConfig{
		IDPMetadataURL:       "https://idp.example.com/metadata",
		IDPMetadataXML:       "<EntityDescriptor>core</EntityDescriptor>",
		IDPSSOURL:            "https://idp.example.com/sso",
		IDPSLOURL:            "https://idp.example.com/slo",
		IDPCertificate:       "core-idp-cert",
		IDPCertFile:          "/etc/pulse/core-idp-cert.pem",
		IDPEntityID:          "core-entity-id",
		IDPIssuer:            "core-issuer",
		SPEntityID:           "core-sp-entity-id",
		SPACSPath:            "/api/saml/core/acs",
		SPMetadataPath:       "/api/saml/core/metadata",
		SPCertificate:        "core-sp-cert",
		SPPrivateKey:         "core-sp-key",
		SPCertFile:           "/etc/pulse/core-sp-cert.pem",
		SPKeyFile:            "/etc/pulse/core-sp-key.pem",
		SignRequests:         true,
		WantAssertionsSigned: true,
		AllowUnencrypted:     true,
		UsernameAttr:         "core-username",
		EmailAttr:            "core-email",
		GroupsAttr:           "core-groups",
		FirstNameAttr:        "core-first",
		LastNameAttr:         "core-last",
		NameIDFormat:         "core-nameid",
		ForceAuthn:           true,
		AllowIDPInitiated:    true,
		RelayStateTemplate:   "core-relay",
	}
}

func branchcov0722PMPopulatedExtSAML() *extensions.SAMLProviderConfig {
	return &extensions.SAMLProviderConfig{
		IDPMetadataURL:       "https://idp.example.com/metadata",
		IDPMetadataXML:       "<EntityDescriptor>ext</EntityDescriptor>",
		IDPSSOURL:            "https://idp.example.com/sso",
		IDPSLOURL:            "https://idp.example.com/slo",
		IDPCertificate:       "ext-idp-cert",
		IDPCertFile:          "/etc/pulse/ext-idp-cert.pem",
		IDPEntityID:          "ext-entity-id",
		IDPIssuer:            "ext-issuer",
		SPEntityID:           "ext-sp-entity-id",
		SPACSPath:            "/api/saml/ext/acs",
		SPMetadataPath:       "/api/saml/ext/metadata",
		SPCertificate:        "ext-sp-cert",
		SPPrivateKey:         "ext-sp-key",
		SPCertFile:           "/etc/pulse/ext-sp-cert.pem",
		SPKeyFile:            "/etc/pulse/ext-sp-key.pem",
		SignRequests:         true,
		WantAssertionsSigned: true,
		AllowUnencrypted:     true,
		UsernameAttr:         "ext-username",
		EmailAttr:            "ext-email",
		GroupsAttr:           "ext-groups",
		FirstNameAttr:        "ext-first",
		LastNameAttr:         "ext-last",
		NameIDFormat:         "ext-nameid",
		ForceAuthn:           true,
		AllowIDPInitiated:    true,
		RelayStateTemplate:   "ext-relay",
	}
}

func branchcov0722PMPopulatedCoreOIDC() *config.OIDCProviderConfig {
	return &config.OIDCProviderConfig{
		IssuerURL:       "https://issuer.example.com",
		ClientID:        "core-client-id",
		ClientSecret:    "core-secret",
		RedirectURL:     "https://pulse.example.com/api/oidc/callback",
		LogoutURL:       "https://issuer.example.com/logout",
		Scopes:          []string{"openid", "profile", "email"},
		UsernameClaim:   "preferred_username",
		EmailClaim:      "email",
		CABundle:        "/etc/pulse/core-ca.pem",
		ClientSecretSet: true,
	}
}

func branchcov0722PMPopulatedCoreProvider() config.SSOProvider {
	return config.SSOProvider{
		ID:                "provider-1",
		Name:              "Corporate SSO",
		Type:              config.SSOProviderTypeSAML,
		Enabled:           true,
		DisplayName:       "Login with Corp",
		IconURL:           "https://example.com/icon.png",
		Priority:          3,
		AllowedGroups:     []string{"admins", "users"},
		AllowedDomains:    []string{"example.com"},
		AllowedEmails:     []string{"bob@example.com"},
		GroupsClaim:       "groups",
		GroupRoleMappings: map[string]string{"admins": "admin", "users": "viewer"},
		OIDC:              branchcov0722PMPopulatedCoreOIDC(),
		SAML:              branchcov0722PMPopulatedCoreSAML(),
	}
}

// ----------------------------------------------------------------------------
// toAPISAMLTestConfig
// ----------------------------------------------------------------------------

func TestBranchcov0722PM_ToAPISAMLTestConfig(t *testing.T) {
	t.Run("nil_returns_nil", func(t *testing.T) {
		if got := toAPISAMLTestConfig(nil); got != nil {
			t.Fatalf("toAPISAMLTestConfig(nil) = %#v, want nil", got)
		}
	})

	t.Run("populated_copies_fields", func(t *testing.T) {
		src := &extensions.SAMLTestConfig{
			IDPMetadataURL: "https://idp.example.com/metadata",
			IDPMetadataXML: "<EntityDescriptor>",
			IDPSSOURL:      "https://idp.example.com/sso",
			IDPCertificate: "idp-cert-pem",
		}
		got := toAPISAMLTestConfig(src)
		if got == nil {
			t.Fatal("toAPISAMLTestConfig(populated) returned nil, want non-nil")
		}
		if got.IDPMetadataURL != src.IDPMetadataURL {
			t.Errorf("IDPMetadataURL = %q, want %q", got.IDPMetadataURL, src.IDPMetadataURL)
		}
		if got.IDPMetadataXML != src.IDPMetadataXML {
			t.Errorf("IDPMetadataXML = %q, want %q", got.IDPMetadataXML, src.IDPMetadataXML)
		}
		if got.IDPSSOURL != src.IDPSSOURL {
			t.Errorf("IDPSSOURL = %q, want %q", got.IDPSSOURL, src.IDPSSOURL)
		}
		if got.IDPCertificate != src.IDPCertificate {
			t.Errorf("IDPCertificate = %q, want %q", got.IDPCertificate, src.IDPCertificate)
		}
		// The api DTO has IDPSLOURL but the extensions DTO does not; the
		// converter must leave it as the zero value.
		if got.IDPSLOURL != "" {
			t.Errorf("IDPSLOURL = %q, want empty (not present on source type)", got.IDPSLOURL)
		}
		// Strings are copied by value; mutating the source after conversion must
		// not affect the converted result.
		src.IDPMetadataURL = "https://mutated.example.com/metadata"
		src.IDPCertificate = "mutated-cert"
		if got.IDPMetadataURL != "https://idp.example.com/metadata" {
			t.Errorf("IDPMetadataURL mutated after conversion to %q, expected independence", got.IDPMetadataURL)
		}
		if got.IDPCertificate != "idp-cert-pem" {
			t.Errorf("IDPCertificate mutated after conversion to %q, expected independence", got.IDPCertificate)
		}
	})
}

// ----------------------------------------------------------------------------
// toAPIOIDCTestConfig
// ----------------------------------------------------------------------------

func TestBranchcov0722PM_ToAPIOIDCTestConfig(t *testing.T) {
	t.Run("nil_returns_nil", func(t *testing.T) {
		if got := toAPIOIDCTestConfig(nil); got != nil {
			t.Fatalf("toAPIOIDCTestConfig(nil) = %#v, want nil", got)
		}
	})

	t.Run("populated_copies_fields", func(t *testing.T) {
		src := &extensions.OIDCTestConfig{
			IssuerURL: "https://issuer.example.com",
			ClientID:  "client-123",
		}
		got := toAPIOIDCTestConfig(src)
		if got == nil {
			t.Fatal("toAPIOIDCTestConfig(populated) returned nil, want non-nil")
		}
		if got.IssuerURL != src.IssuerURL {
			t.Errorf("IssuerURL = %q, want %q", got.IssuerURL, src.IssuerURL)
		}
		if got.ClientID != src.ClientID {
			t.Errorf("ClientID = %q, want %q", got.ClientID, src.ClientID)
		}
		src.IssuerURL = "https://mutated.example.com"
		if got.IssuerURL != "https://issuer.example.com" {
			t.Errorf("IssuerURL mutated after conversion to %q, expected independence", got.IssuerURL)
		}
	})
}

// ----------------------------------------------------------------------------
// toExtensionSSOTestResponse
// ----------------------------------------------------------------------------

func TestBranchcov0722PM_ToExtensionSSOTestResponse(t *testing.T) {
	t.Run("details_nil_propagates", func(t *testing.T) {
		resp := SSOTestResponse{
			Success: true,
			Message: "connection ok",
			Error:   "",
			Details: nil,
		}
		got := toExtensionSSOTestResponse(resp)
		if !got.Success {
			t.Errorf("Success = false, want true")
		}
		if got.Message != "connection ok" {
			t.Errorf("Message = %q, want %q", got.Message, "connection ok")
		}
		if got.Error != "" {
			t.Errorf("Error = %q, want empty", got.Error)
		}
		if got.Details != nil {
			t.Fatalf("Details = %#v, want nil", got.Details)
		}
	})

	t.Run("with_error_message", func(t *testing.T) {
		resp := SSOTestResponse{
			Success: false,
			Message: "failed",
			Error:   "dial tcp: connection refused",
		}
		got := toExtensionSSOTestResponse(resp)
		if got.Success {
			t.Errorf("Success = true, want false")
		}
		if got.Error != "dial tcp: connection refused" {
			t.Errorf("Error = %q, want the connection-refused error", got.Error)
		}
	})

	t.Run("details_populated_copies_fields", func(t *testing.T) {
		resp := SSOTestResponse{
			Success: true,
			Message: "ok",
			Details: &SSOTestDetails{
				Type:             "oidc",
				EntityID:         "entity-1",
				SSOURL:           "https://idp/sso",
				SLOURL:           "https://idp/slo",
				TokenEndpoint:    "https://idp/token",
				UserinfoEndpoint: "https://idp/userinfo",
				JWKSURI:          "https://idp/jwks",
				SupportedScopes:  []string{"openid", "email"},
				Certificates:     []CertificateInfo{branchcov0722PMCertInfo("subject-a")},
			},
		}
		got := toExtensionSSOTestResponse(resp)
		if got.Details == nil {
			t.Fatal("Details = nil, want non-nil")
		}
		d := got.Details
		if d.Type != "oidc" {
			t.Errorf("Details.Type = %q, want oidc", d.Type)
		}
		if d.EntityID != "entity-1" {
			t.Errorf("Details.EntityID = %q, want entity-1", d.EntityID)
		}
		if d.SSOURL != "https://idp/sso" {
			t.Errorf("Details.SSOURL = %q, want https://idp/sso", d.SSOURL)
		}
		if d.SLOURL != "https://idp/slo" {
			t.Errorf("Details.SLOURL = %q, want https://idp/slo", d.SLOURL)
		}
		if d.TokenEndpoint != "https://idp/token" {
			t.Errorf("Details.TokenEndpoint = %q, want https://idp/token", d.TokenEndpoint)
		}
		if d.UserinfoEndpoint != "https://idp/userinfo" {
			t.Errorf("Details.UserinfoEndpoint = %q, want https://idp/userinfo", d.UserinfoEndpoint)
		}
		if d.JWKSURI != "https://idp/jwks" {
			t.Errorf("Details.JWKSURI = %q, want https://idp/jwks", d.JWKSURI)
		}
		if len(d.SupportedScopes) != 2 || d.SupportedScopes[0] != "openid" || d.SupportedScopes[1] != "email" {
			t.Errorf("Details.SupportedScopes = %#v, want [openid email]", d.SupportedScopes)
		}
		if !reflect.DeepEqual(d.Certificates, []extensions.CertificateInfo{branchcov0722PMExtCertInfo("subject-a")}) {
			t.Errorf("Details.Certificates = %#v, want single subject-a cert", d.Certificates)
		}
	})
}

// ----------------------------------------------------------------------------
// toExtensionSSOTestDetails
// ----------------------------------------------------------------------------

func TestBranchcov0722PM_ToExtensionSSOTestDetails(t *testing.T) {
	t.Run("nil_returns_nil", func(t *testing.T) {
		if got := toExtensionSSOTestDetails(nil); got != nil {
			t.Fatalf("toExtensionSSOTestDetails(nil) = %#v, want nil", got)
		}
	})

	t.Run("populated_without_certificates", func(t *testing.T) {
		src := &SSOTestDetails{
			Type:            "saml",
			EntityID:        "saml-entity",
			SSOURL:          "https://saml/sso",
			SupportedScopes: nil,
		}
		got := toExtensionSSOTestDetails(src)
		if got == nil {
			t.Fatal("returned nil, want non-nil")
		}
		if got.Type != "saml" {
			t.Errorf("Type = %q, want saml", got.Type)
		}
		if got.EntityID != "saml-entity" {
			t.Errorf("EntityID = %q, want saml-entity", got.EntityID)
		}
		if got.SSOURL != "https://saml/sso" {
			t.Errorf("SSOURL = %q, want https://saml/sso", got.SSOURL)
		}
		if got.Certificates != nil {
			t.Errorf("Certificates = %#v, want nil when source empty", got.Certificates)
		}
	})

	t.Run("certificates_are_independently_copied", func(t *testing.T) {
		src := &SSOTestDetails{
			Type:         "saml",
			Certificates: []CertificateInfo{branchcov0722PMCertInfo("subject-a"), branchcov0722PMCertInfo("expired")},
		}
		got := toExtensionSSOTestDetails(src)
		if len(got.Certificates) != 2 {
			t.Fatalf("len(Certificates) = %d, want 2", len(got.Certificates))
		}
		// Field-by-field verification of the first certificate.
		want := branchcov0722PMExtCertInfo("subject-a")
		c0 := got.Certificates[0]
		if c0.Subject != want.Subject || c0.Issuer != want.Issuer || c0.IsExpired != want.IsExpired {
			t.Errorf("Certificates[0] = %+v, want %+v", c0, want)
		}
		if !c0.NotBefore.Equal(want.NotBefore) || !c0.NotAfter.Equal(want.NotAfter) {
			t.Errorf("Certificates[0] timestamps = %v..%v, want %v..%v", c0.NotBefore, c0.NotAfter, want.NotBefore, want.NotAfter)
		}
		if !got.Certificates[1].IsExpired {
			t.Errorf("Certificates[1].IsExpired = false, want true")
		}

		// Clone independence: mutate the source slice and elements after
		// conversion. The converted Certificates must be unaffected (the
		// converter allocates a fresh slice and copies each CertificateInfo by
		// value).
		src.Certificates[0] = branchcov0722PMCertInfo("tampered")
		src.Certificates = append(src.Certificates, branchcov0722PMCertInfo("extra"))
		if got.Certificates[0].Subject != "subject-a" {
			t.Errorf("Certificates[0].Subject mutated to %q after source change; converter must deep-copy", got.Certificates[0].Subject)
		}
		if len(got.Certificates) != 2 {
			t.Errorf("len(Certificates) changed to %d after source append; converter must not alias backing array", len(got.Certificates))
		}
	})

	t.Run("supported_scopes_alias_source_backing_array", func(t *testing.T) {
		// CHARACTERIZATION (not desired behaviour): the converter assigns
		// SupportedScopes by reference, so element mutation of the source slice
		// is visible through the converted value. This locks in the current
		// aliasing and is reported as a suspected source bug (see report).
		src := &SSOTestDetails{
			Type:            "oidc",
			SupportedScopes: []string{"openid", "email"},
		}
		got := toExtensionSSOTestDetails(src)
		src.SupportedScopes[0] = "mutated"
		if got.SupportedScopes[0] != "mutated" {
			t.Errorf("SupportedScopes[0] = %q, want %q (documents aliasing bug)", got.SupportedScopes[0], "mutated")
		}
		// Reassigning the source slice header, however, must NOT affect the
		// converted slice (different headers).
		src.SupportedScopes = []string{"entirely", "new"}
		if len(got.SupportedScopes) != 2 || got.SupportedScopes[0] != "mutated" {
			t.Errorf("SupportedScopes = %#v, want unchanged [mutated email] after header reassign", got.SupportedScopes)
		}
	})
}

// ----------------------------------------------------------------------------
// toExtensionSSOConfigSnapshot
// ----------------------------------------------------------------------------

func TestBranchcov0722PM_ToExtensionSSOConfigSnapshot(t *testing.T) {
	t.Run("nil_returns_defaulted_empty_snapshot", func(t *testing.T) {
		got := toExtensionSSOConfigSnapshot(nil)
		// The nil arm must return a non-nil, empty Providers slice and
		// AllowMultipleProviders=true (a safe default).
		if got.Providers == nil {
			t.Fatal("Providers = nil, want non-nil empty slice")
		}
		if len(got.Providers) != 0 {
			t.Fatalf("len(Providers) = %d, want 0", len(got.Providers))
		}
		if !got.AllowMultipleProviders {
			t.Errorf("AllowMultipleProviders = false, want true (default)")
		}
		if got.DefaultProviderID != "" {
			t.Errorf("DefaultProviderID = %q, want empty", got.DefaultProviderID)
		}
	})

	t.Run("populated_maps_providers_and_metadata", func(t *testing.T) {
		cfg := &config.SSOConfig{
			DefaultProviderID:      "provider-1",
			AllowMultipleProviders: false,
			Providers: []config.SSOProvider{
				branchcov0722PMPopulatedCoreProvider(),
				{
					ID:      "provider-2",
					Name:    "OIDC Only",
					Type:    config.SSOProviderTypeOIDC,
					Enabled: false,
					OIDC:    branchcov0722PMPopulatedCoreOIDC(),
				},
			},
		}
		got := toExtensionSSOConfigSnapshot(cfg)
		if got.DefaultProviderID != "provider-1" {
			t.Errorf("DefaultProviderID = %q, want provider-1", got.DefaultProviderID)
		}
		if got.AllowMultipleProviders {
			t.Errorf("AllowMultipleProviders = true, want false")
		}
		if len(got.Providers) != 2 {
			t.Fatalf("len(Providers) = %d, want 2", len(got.Providers))
		}
		if got.Providers[0].ID != "provider-1" {
			t.Errorf("Providers[0].ID = %q, want provider-1", got.Providers[0].ID)
		}
		if got.Providers[1].ID != "provider-2" {
			t.Errorf("Providers[1].ID = %q, want provider-2", got.Providers[1].ID)
		}
		// Provider detail is verified exhaustively in TestBranchcov0722PM_ToExtensionSSOProvider;
		// here spot-check the type conversion and nested config presence.
		if got.Providers[0].Type != extensions.SSOProviderTypeSAML {
			t.Errorf("Providers[0].Type = %q, want %q", got.Providers[0].Type, extensions.SSOProviderTypeSAML)
		}
		if got.Providers[0].SAML == nil {
			t.Errorf("Providers[0].SAML = nil, want populated")
		}
		if got.Providers[1].OIDC == nil {
			t.Errorf("Providers[1].OIDC = nil, want populated")
		}
		if got.Providers[1].SAML != nil {
			t.Errorf("Providers[1].SAML = %#v, want nil (OIDC-only provider)", got.Providers[1].SAML)
		}
	})

	t.Run("providers_slice_is_independent_of_source", func(t *testing.T) {
		cfg := &config.SSOConfig{
			Providers:              []config.SSOProvider{branchcov0722PMPopulatedCoreProvider()},
			AllowMultipleProviders: true,
		}
		got := toExtensionSSOConfigSnapshot(cfg)
		// Mutate the source slice; the converted snapshot must not change.
		cfg.Providers[0].ID = "mutated"
		cfg.Providers = append(cfg.Providers, config.SSOProvider{ID: "extra"})
		if got.Providers[0].ID != "provider-1" {
			t.Errorf("Providers[0].ID changed to %q after source mutation; converter must copy the slice", got.Providers[0].ID)
		}
		if len(got.Providers) != 1 {
			t.Errorf("len(Providers) = %d, want 1 after source append (slice must be copied)", len(got.Providers))
		}
	})

	t.Run("provider_collection_fields_alias_source", func(t *testing.T) {
		// CHARACTERIZATION: AllowedGroups and GroupRoleMappings on each
		// provider are assigned by reference by toExtensionSSOProvider, so the
		// converted snapshot shares their backing storage with the source. This
		// documents the aliasing (reported as a suspected source bug).
		cfg := &config.SSOConfig{
			Providers:              []config.SSOProvider{branchcov0722PMPopulatedCoreProvider()},
			AllowMultipleProviders: true,
		}
		got := toExtensionSSOConfigSnapshot(cfg)
		cfg.Providers[0].AllowedGroups[0] = "mutated-group"
		cfg.Providers[0].GroupRoleMappings["admins"] = "mutated-role"
		if got.Providers[0].AllowedGroups[0] != "mutated-group" {
			t.Errorf("AllowedGroups[0] = %q, want %q (documents aliasing bug)", got.Providers[0].AllowedGroups[0], "mutated-group")
		}
		if got.Providers[0].GroupRoleMappings["admins"] != "mutated-role" {
			t.Errorf("GroupRoleMappings[admins] = %q, want %q (documents aliasing bug)", got.Providers[0].GroupRoleMappings["admins"], "mutated-role")
		}
	})
}

// ----------------------------------------------------------------------------
// toExtensionSSOProvider
// ----------------------------------------------------------------------------

func TestBranchcov0722PM_ToExtensionSSOProvider(t *testing.T) {
	t.Run("nil_nested_configs_omit_oidc_and_saml", func(t *testing.T) {
		p := config.SSOProvider{
			ID:      "bare",
			Name:    "Bare",
			Type:    config.SSOProviderTypeOIDC,
			Enabled: true,
		}
		got := toExtensionSSOProvider(p)
		if got.ID != "bare" {
			t.Errorf("ID = %q, want bare", got.ID)
		}
		if got.Type != extensions.SSOProviderTypeOIDC {
			t.Errorf("Type = %q, want %q", got.Type, extensions.SSOProviderTypeOIDC)
		}
		if !got.Enabled {
			t.Errorf("Enabled = false, want true")
		}
		if got.OIDC != nil {
			t.Errorf("OIDC = %#v, want nil when source OIDC is nil", got.OIDC)
		}
		if got.SAML != nil {
			t.Errorf("SAML = %#v, want nil when source SAML is nil", got.SAML)
		}
	})

	t.Run("populated_copies_all_fields", func(t *testing.T) {
		p := branchcov0722PMPopulatedCoreProvider()
		got := toExtensionSSOProvider(p)

		// Top-level scalar fields.
		if got.ID != "provider-1" {
			t.Errorf("ID = %q, want provider-1", got.ID)
		}
		if got.Name != "Corporate SSO" {
			t.Errorf("Name = %q, want Corporate SSO", got.Name)
		}
		if got.Type != extensions.SSOProviderTypeSAML {
			t.Errorf("Type = %q, want %q", got.Type, extensions.SSOProviderTypeSAML)
		}
		if !got.Enabled {
			t.Errorf("Enabled = false, want true")
		}
		if got.DisplayName != "Login with Corp" {
			t.Errorf("DisplayName = %q, want Login with Corp", got.DisplayName)
		}
		if got.IconURL != "https://example.com/icon.png" {
			t.Errorf("IconURL = %q, want icon url", got.IconURL)
		}
		if got.Priority != 3 {
			t.Errorf("Priority = %d, want 3", got.Priority)
		}
		if got.GroupsClaim != "groups" {
			t.Errorf("GroupsClaim = %q, want groups", got.GroupsClaim)
		}

		// Top-level collection fields.
		if !reflect.DeepEqual(got.AllowedGroups, []string{"admins", "users"}) {
			t.Errorf("AllowedGroups = %#v, want [admins users]", got.AllowedGroups)
		}
		if !reflect.DeepEqual(got.AllowedDomains, []string{"example.com"}) {
			t.Errorf("AllowedDomains = %#v, want [example.com]", got.AllowedDomains)
		}
		if !reflect.DeepEqual(got.AllowedEmails, []string{"bob@example.com"}) {
			t.Errorf("AllowedEmails = %#v, want [bob@example.com]", got.AllowedEmails)
		}
		wantMappings := map[string]string{"admins": "admin", "users": "viewer"}
		if !reflect.DeepEqual(got.GroupRoleMappings, wantMappings) {
			t.Errorf("GroupRoleMappings = %#v, want %#v", got.GroupRoleMappings, wantMappings)
		}

		// OIDC nested config, field by field.
		if got.OIDC == nil {
			t.Fatal("OIDC = nil, want populated")
		}
		oidc := got.OIDC
		srcOIDC := p.OIDC
		if oidc.IssuerURL != srcOIDC.IssuerURL {
			t.Errorf("OIDC.IssuerURL = %q, want %q", oidc.IssuerURL, srcOIDC.IssuerURL)
		}
		if oidc.ClientID != srcOIDC.ClientID {
			t.Errorf("OIDC.ClientID = %q, want %q", oidc.ClientID, srcOIDC.ClientID)
		}
		if oidc.ClientSecret != srcOIDC.ClientSecret {
			t.Errorf("OIDC.ClientSecret = %q, want %q", oidc.ClientSecret, srcOIDC.ClientSecret)
		}
		if oidc.RedirectURL != srcOIDC.RedirectURL {
			t.Errorf("OIDC.RedirectURL = %q, want %q", oidc.RedirectURL, srcOIDC.RedirectURL)
		}
		if oidc.LogoutURL != srcOIDC.LogoutURL {
			t.Errorf("OIDC.LogoutURL = %q, want %q", oidc.LogoutURL, srcOIDC.LogoutURL)
		}
		if !reflect.DeepEqual(oidc.Scopes, srcOIDC.Scopes) {
			t.Errorf("OIDC.Scopes = %#v, want %#v", oidc.Scopes, srcOIDC.Scopes)
		}
		if oidc.UsernameClaim != srcOIDC.UsernameClaim {
			t.Errorf("OIDC.UsernameClaim = %q, want %q", oidc.UsernameClaim, srcOIDC.UsernameClaim)
		}
		if oidc.EmailClaim != srcOIDC.EmailClaim {
			t.Errorf("OIDC.EmailClaim = %q, want %q", oidc.EmailClaim, srcOIDC.EmailClaim)
		}
		if oidc.CABundle != srcOIDC.CABundle {
			t.Errorf("OIDC.CABundle = %q, want %q", oidc.CABundle, srcOIDC.CABundle)
		}
		if !oidc.ClientSecretSet {
			t.Errorf("OIDC.ClientSecretSet = false, want true")
		}

		// SAML nested config: verify every copied field via DeepEqual against a
		// hand-built expected value (built from the same source fixture).
		if got.SAML == nil {
			t.Fatal("SAML = nil, want populated")
		}
		srcSAML := p.SAML
		wantSAML := &extensions.SAMLProviderConfig{
			IDPMetadataURL:       srcSAML.IDPMetadataURL,
			IDPMetadataXML:       srcSAML.IDPMetadataXML,
			IDPSSOURL:            srcSAML.IDPSSOURL,
			IDPSLOURL:            srcSAML.IDPSLOURL,
			IDPCertificate:       srcSAML.IDPCertificate,
			IDPCertFile:          srcSAML.IDPCertFile,
			IDPEntityID:          srcSAML.IDPEntityID,
			IDPIssuer:            srcSAML.IDPIssuer,
			SPEntityID:           srcSAML.SPEntityID,
			SPACSPath:            srcSAML.SPACSPath,
			SPMetadataPath:       srcSAML.SPMetadataPath,
			SPCertificate:        srcSAML.SPCertificate,
			SPPrivateKey:         srcSAML.SPPrivateKey,
			SPCertFile:           srcSAML.SPCertFile,
			SPKeyFile:            srcSAML.SPKeyFile,
			SignRequests:         srcSAML.SignRequests,
			WantAssertionsSigned: srcSAML.WantAssertionsSigned,
			AllowUnencrypted:     srcSAML.AllowUnencrypted,
			UsernameAttr:         srcSAML.UsernameAttr,
			EmailAttr:            srcSAML.EmailAttr,
			GroupsAttr:           srcSAML.GroupsAttr,
			FirstNameAttr:        srcSAML.FirstNameAttr,
			LastNameAttr:         srcSAML.LastNameAttr,
			NameIDFormat:         srcSAML.NameIDFormat,
			ForceAuthn:           srcSAML.ForceAuthn,
			AllowIDPInitiated:    srcSAML.AllowIDPInitiated,
			RelayStateTemplate:   srcSAML.RelayStateTemplate,
		}
		if !reflect.DeepEqual(got.SAML, wantSAML) {
			t.Errorf("SAML = %+v, want %+v", got.SAML, wantSAML)
		}
	})

	t.Run("collections_alias_source_backing_storage", func(t *testing.T) {
		// CHARACTERIZATION: AllowedGroups, AllowedDomains, AllowedEmails,
		// GroupRoleMappings and OIDC.Scopes are assigned by reference, so the
		// converted provider shares backing storage with the source. Reported
		// as a suspected source bug.
		p := branchcov0722PMPopulatedCoreProvider()
		got := toExtensionSSOProvider(p)

		p.AllowedGroups[0] = "mutated-group"
		if got.AllowedGroups[0] != "mutated-group" {
			t.Errorf("AllowedGroups[0] = %q, want %q (documents aliasing bug)", got.AllowedGroups[0], "mutated-group")
		}

		p.GroupRoleMappings["admins"] = "mutated-role"
		if got.GroupRoleMappings["admins"] != "mutated-role" {
			t.Errorf("GroupRoleMappings[admins] = %q, want %q (documents aliasing bug)", got.GroupRoleMappings["admins"], "mutated-role")
		}

		if got.OIDC != nil {
			p.OIDC.Scopes[0] = "mutated-scope"
			if got.OIDC.Scopes[0] != "mutated-scope" {
				t.Errorf("OIDC.Scopes[0] = %q, want %q (documents aliasing bug)", got.OIDC.Scopes[0], "mutated-scope")
			}
		}
	})
}

// ----------------------------------------------------------------------------
// toCoreSAMLProviderConfig
// ----------------------------------------------------------------------------

func TestBranchcov0722PM_ToCoreSAMLProviderConfig(t *testing.T) {
	t.Run("nil_returns_nil", func(t *testing.T) {
		if got := toCoreSAMLProviderConfig(nil); got != nil {
			t.Fatalf("toCoreSAMLProviderConfig(nil) = %#v, want nil", got)
		}
	})

	t.Run("populated_copies_all_fields", func(t *testing.T) {
		src := branchcov0722PMPopulatedExtSAML()
		got := toCoreSAMLProviderConfig(src)
		if got == nil {
			t.Fatal("returned nil, want non-nil")
		}

		want := &config.SAMLProviderConfig{
			IDPMetadataURL:       src.IDPMetadataURL,
			IDPMetadataXML:       src.IDPMetadataXML,
			IDPSSOURL:            src.IDPSSOURL,
			IDPSLOURL:            src.IDPSLOURL,
			IDPCertificate:       src.IDPCertificate,
			IDPCertFile:          src.IDPCertFile,
			IDPEntityID:          src.IDPEntityID,
			IDPIssuer:            src.IDPIssuer,
			SPEntityID:           src.SPEntityID,
			SPACSPath:            src.SPACSPath,
			SPMetadataPath:       src.SPMetadataPath,
			SPCertificate:        src.SPCertificate,
			SPPrivateKey:         src.SPPrivateKey,
			SPCertFile:           src.SPCertFile,
			SPKeyFile:            src.SPKeyFile,
			SignRequests:         src.SignRequests,
			WantAssertionsSigned: src.WantAssertionsSigned,
			AllowUnencrypted:     src.AllowUnencrypted,
			UsernameAttr:         src.UsernameAttr,
			EmailAttr:            src.EmailAttr,
			GroupsAttr:           src.GroupsAttr,
			FirstNameAttr:        src.FirstNameAttr,
			LastNameAttr:         src.LastNameAttr,
			NameIDFormat:         src.NameIDFormat,
			ForceAuthn:           src.ForceAuthn,
			AllowIDPInitiated:    src.AllowIDPInitiated,
			RelayStateTemplate:   src.RelayStateTemplate,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("converted config = %+v\nwant %+v", got, want)
		}

		// Explicit checks on representative boolean and string fields to keep
		// the assertions readable in failure output.
		if got.IDPMetadataURL != "https://idp.example.com/metadata" {
			t.Errorf("IDPMetadataURL = %q, want metadata url", got.IDPMetadataURL)
		}
		if got.IDPSLOURL != "https://idp.example.com/slo" {
			t.Errorf("IDPSLOURL = %q, want slo url", got.IDPSLOURL)
		}
		if !got.SignRequests {
			t.Errorf("SignRequests = false, want true")
		}
		if !got.AllowIDPInitiated {
			t.Errorf("AllowIDPInitiated = false, want true")
		}
		if got.RelayStateTemplate != "ext-relay" {
			t.Errorf("RelayStateTemplate = %q, want ext-relay", got.RelayStateTemplate)
		}

		// The result is an independent copy: mutating the source must not
		// affect the converted value (all fields are value types here).
		src.IDPMetadataURL = "https://mutated.example.com/metadata"
		src.SignRequests = false
		if got.IDPMetadataURL != "https://idp.example.com/metadata" {
			t.Errorf("IDPMetadataURL mutated to %q after source change; converter must copy", got.IDPMetadataURL)
		}
		if !got.SignRequests {
			t.Errorf("SignRequests mutated to false after source change; converter must copy")
		}
	})
}
