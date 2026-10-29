package api

import (
	"reflect"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// ssoPresentationOnlyUpdate compares the entire effective provider except its
// four presentation fields. New protocol or policy fields therefore require
// initialization by default. Lifecycle changes never reuse an enabled service.
func ssoPresentationOnlyUpdate(existing, updated config.SSOProvider) bool {
	if existing.Type != updated.Type || !existing.Enabled || !updated.Enabled {
		return false
	}
	switch existing.Type {
	case config.SSOProviderTypeOIDC:
		if existing.OIDC == nil || updated.OIDC == nil {
			return false
		}
	case config.SSOProviderTypeSAML:
		if existing.SAML == nil || updated.SAML == nil {
			return false
		}
	default:
		return false
	}
	normalize := func(p config.SSOProvider) config.SSOProvider {
		p.Name, p.DisplayName, p.IconURL, p.Priority = "", "", "", 0
		// Empty API collections and omitted persisted collections are the same
		// policy. Work on copies so comparison cannot mutate either snapshot.
		if len(p.AllowedGroups) == 0 {
			p.AllowedGroups = nil
		}
		if len(p.AllowedDomains) == 0 {
			p.AllowedDomains = nil
		}
		if len(p.AllowedEmails) == 0 {
			p.AllowedEmails = nil
		}
		if len(p.GroupRoleMappings) == 0 {
			p.GroupRoleMappings = nil
		}
		if p.OIDC != nil {
			oidc := *p.OIDC
			// This response marker is not a credential. The actual secret,
			// merged before comparison, remains significant.
			oidc.ClientSecretSet = oidc.ClientSecret != ""
			if len(oidc.Scopes) == 0 {
				oidc.Scopes = nil
			}
			p.OIDC = &oidc
		}
		return p
	}
	return reflect.DeepEqual(normalize(existing), normalize(updated))
}
