package api

import (
	"encoding/json"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// preserveUnrepresentedSSOFields keeps only settings absent from the provider
// detail/editor model. Visible fields retain PUT replacement/clear semantics;
// client secrets retain their separate blank-means-keep rule. Never expose
// advanced settings (in particular signing keys) merely to round-trip an edit.
func preserveUnrepresentedSSOFields(data []byte, updated, existing *config.SSOProvider) error {
	if updated.Type != existing.Type {
		return nil
	}
	var fields struct {
		OIDC map[string]json.RawMessage `json:"oidc"`
		SAML map[string]json.RawMessage `json:"saml"`
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	// Match encoding/json aliases, including escaped names and Unicode folds.
	// The admission decoder already rejects competing aliases before this point.
	present := func(fields map[string]json.RawMessage, name string) bool {
		for key := range fields {
			if strings.EqualFold(key, name) {
				return true
			}
		}
		return false
	}
	if updated.Type == config.SSOProviderTypeOIDC && updated.OIDC != nil && existing.OIDC != nil {
		u, e := updated.OIDC, existing.OIDC
		if !present(fields.OIDC, "usernameClaim") {
			u.UsernameClaim = e.UsernameClaim
		}
		if !present(fields.OIDC, "emailClaim") {
			u.EmailClaim = e.EmailClaim
		}
		if !present(fields.OIDC, "caBundle") {
			u.CABundle = e.CABundle
		}
	}
	if updated.Type == config.SSOProviderTypeSAML && updated.SAML != nil && existing.SAML != nil {
		u, e := updated.SAML, existing.SAML
		if !present(fields.SAML, "idpSloUrl") {
			u.IDPSLOURL = e.IDPSLOURL
		}
		if !present(fields.SAML, "idpCertFile") {
			u.IDPCertFile = e.IDPCertFile
		}
		if !present(fields.SAML, "idpIssuer") {
			u.IDPIssuer = e.IDPIssuer
		}
		if !present(fields.SAML, "spAcsPath") {
			u.SPACSPath = e.SPACSPath
		}
		if !present(fields.SAML, "spMetadataPath") {
			u.SPMetadataPath = e.SPMetadataPath
		}
		if !present(fields.SAML, "spCertificate") {
			u.SPCertificate = e.SPCertificate
		}
		if !present(fields.SAML, "spPrivateKey") {
			u.SPPrivateKey = e.SPPrivateKey
		}
		if !present(fields.SAML, "spCertFile") {
			u.SPCertFile = e.SPCertFile
		}
		if !present(fields.SAML, "spKeyFile") {
			u.SPKeyFile = e.SPKeyFile
		}
		if !present(fields.SAML, "wantAssertionsSigned") {
			u.WantAssertionsSigned = e.WantAssertionsSigned
		}
		if !present(fields.SAML, "allowUnencrypted") {
			u.AllowUnencrypted = e.AllowUnencrypted
		}
		if !present(fields.SAML, "firstNameAttr") {
			u.FirstNameAttr = e.FirstNameAttr
		}
		if !present(fields.SAML, "lastNameAttr") {
			u.LastNameAttr = e.LastNameAttr
		}
		if !present(fields.SAML, "nameIdFormat") {
			u.NameIDFormat = e.NameIDFormat
		}
		if !present(fields.SAML, "forceAuthn") {
			u.ForceAuthn = e.ForceAuthn
		}
		if !present(fields.SAML, "relayStateTemplate") {
			u.RelayStateTemplate = e.RelayStateTemplate
		}
	}
	return nil
}
