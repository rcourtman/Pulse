package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

// ssoPrincipalNamespace prefixes every principal minted by stableSSOPrincipal.
const ssoPrincipalNamespace = "sso:"

// ssoUnknownOriginalIssuerSentinel is recorded when a provider entry is edited
// or deleted while its original issuer cannot be determined, forcing subsequent
// logins on that entry ID to fail closed into issuer-scoped principals.
const ssoUnknownOriginalIssuerSentinel = "__unknown__"

func stableSSOPrincipal(providerType config.SSOProviderType, providerID string, subject string) (string, error) {
	return ssoPrincipalWithOptionalIssuer(providerType, providerID, "", subject)
}

func stableSSOPrincipalWithIssuer(providerType config.SSOProviderType, providerID string, issuer string, subject string) (string, error) {
	iss := strings.TrimSpace(issuer)
	if iss == "" {
		return "", fmt.Errorf("sso issuer is required")
	}
	return ssoPrincipalWithOptionalIssuer(providerType, providerID, iss, subject)
}

func ssoPrincipalWithOptionalIssuer(providerType config.SSOProviderType, providerID string, issuer string, subject string) (string, error) {
	typ := strings.TrimSpace(strings.ToLower(string(providerType)))
	id := strings.TrimSpace(providerID)
	if typ == "" {
		return "", fmt.Errorf("sso provider type is required")
	}
	if id == "" {
		return "", fmt.Errorf("sso provider id is required")
	}
	trimmed := strings.TrimSpace(subject)
	if trimmed == "" {
		return "", fmt.Errorf("sso subject is required")
	}
	// The provider identifies an account by the exact subject, so "alice" and
	// "alice\u00a0" are two accounts. Refuse a padded subject instead of
	// normalizing it onto the principal of the account it resembles.
	if trimmed != subject {
		return "", fmt.Errorf("sso subject must not have leading or trailing whitespace")
	}

	scoped := typ + "\x00" + id + "\x00"
	if iss := strings.TrimSpace(issuer); iss != "" {
		scoped += iss + "\x00"
	}
	sum := sha256.Sum256([]byte(scoped + subject))
	return ssoPrincipalNamespace + typ + ":" + id + ":" + base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// stableSSOPrincipalForIssuer derives the stable principal for an SSO login
// while preventing issuer changes on an existing provider entry ID from
// handing saved roles to another identity provider's subjects. The first
// issuer bound to (providerType, providerID) keeps the compatibility principal
// format so upgrades do not rename existing users; any other issuer hashes the
// issuer into the principal.
func stableSSOPrincipalForIssuer(manager internalauth.Manager, providerType config.SSOProviderType, providerID string, issuer string, subject string) (string, error) {
	iss := strings.TrimSpace(issuer)
	if iss == "" {
		return "", fmt.Errorf("sso issuer is required")
	}
	if registry, ok := manager.(internalauth.ProviderIssuerRegistry); ok {
		originalIssuer, err := registry.GetOrBindProviderOriginalIssuer(string(providerType), providerID, iss)
		if err != nil {
			return "", fmt.Errorf("resolve sso provider issuer binding: %w", err)
		}
		if originalIssuer != iss {
			return stableSSOPrincipalWithIssuer(providerType, providerID, iss, subject)
		}
	}
	return stableSSOPrincipal(providerType, providerID, subject)
}

// isStableSSOPrincipal reports whether value lies in the namespace reserved
// for stableSSOPrincipal. Such a value names a current SSO identity, possibly
// another provider's, and is never a pre-principal username or email.
func isStableSSOPrincipal(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), ssoPrincipalNamespace)
}

func extractSAMLEntityIDFromXML(rawXML string) string {
	trimmed := strings.TrimSpace(rawXML)
	if trimmed == "" {
		return ""
	}
	var descriptor struct {
		EntityID string `xml:"entityID,attr"`
	}
	if err := xml.Unmarshal([]byte(trimmed), &descriptor); err != nil {
		return ""
	}
	return strings.TrimSpace(descriptor.EntityID)
}

func bindSSOProviderOriginalIssuer(manager internalauth.Manager, providerType config.SSOProviderType, providerID string, issuer string, failClosedIfUnknown bool) {
	registry, ok := manager.(internalauth.ProviderIssuerRegistry)
	if !ok {
		return
	}
	iss := strings.TrimSpace(issuer)
	if iss == "" {
		if !failClosedIfUnknown {
			return
		}
		iss = ssoUnknownOriginalIssuerSentinel
	}
	if err := registry.BindProviderOriginalIssuer(string(providerType), providerID, iss); err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("Failed to bind original issuer for SSO provider")
	}
}

func ssoLegacyPrincipalCandidates(values ...string) []string {
	candidates := make([]string, 0, len(values)+2)
	seen := make(map[string]struct{}, len(values)+2)
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; !ok {
			candidates = append(candidates, trimmed)
			seen[trimmed] = struct{}{}
		}

		if strings.Contains(trimmed, "@") {
			lower := strings.ToLower(trimmed)
			if _, ok := seen[lower]; !ok {
				candidates = append(candidates, lower)
				seen[lower] = struct{}{}
			}
		}
	}
	return candidates
}

// errSSORolesNotReplaced reports that an authoritative group mapping could not
// replace the principal's saved roles. Callers must refuse the sign-in rather
// than continue with grants from an earlier one.
var errSSORolesNotReplaced = errors.New("sso role assignments could not be replaced")

func applySSORoleAssignments(manager internalauth.Manager, principal string, legacyCandidates []string, mappedRoles []string, mappingAuthoritative bool, ensureListed bool) error {
	principal = strings.TrimSpace(principal)
	if manager == nil || principal == "" {
		return nil
	}

	if mappingAuthoritative || len(mappedRoles) > 0 {
		err := updateMappedSSORoles(manager, principal, mappedRoles)
		if err == nil || !mappingAuthoritative {
			return err
		}
		// The store replaces a role list completely or not at all, so a mapping
		// that names a missing role leaves the previous sign-in's roles saved.
		// Such a group resolves to no role: replace the saved roles with the
		// mapped roles that exist instead of letting stale grants outlive the
		// group membership that justified them.
		existing := make([]string, 0, len(mappedRoles))
		errorAwareReader, hasErrorAwareReader := manager.(internalauth.ErrorAwareRoleReader)
		for _, roleID := range mappedRoles {
			if hasErrorAwareReader {
				_, ok, readErr := errorAwareReader.GetRoleWithError(roleID)
				if readErr != nil {
					return errors.Join(errSSORolesNotReplaced, err, readErr)
				}
				if ok {
					existing = append(existing, roleID)
				}
				continue
			}
			if _, ok := manager.GetRole(roleID); ok {
				existing = append(existing, roleID)
			}
		}
		if len(existing) == len(mappedRoles) {
			return errors.Join(errSSORolesNotReplaced, err)
		}
		if replaceErr := updateMappedSSORoles(manager, principal, existing); replaceErr != nil {
			return errors.Join(errSSORolesNotReplaced, err, replaceErr)
		}
		return fmt.Errorf("assigned only the mapped roles that exist %q: %w", existing, err)
	}

	principalAssignment, principalExists := manager.GetUserAssignment(principal)
	if principalExists && principalAssignment.RolesFromMapping {
		// The provider no longer maps groups to roles. Roles an earlier mapping
		// saved are not an administrator's assignment and must not outlive it,
		// so only roles assigned by hand are kept below.
		if err := manager.UpdateUserRoles(principal, nil); err != nil {
			return err
		}
		principalAssignment.RoleIDs = nil
	}
	principalHasRoles := principalExists && len(principalAssignment.RoleIDs) > 0

	for _, candidate := range legacyCandidates {
		candidate = strings.TrimSpace(candidate)
		// Candidates are claim text the signing-in user may control. Only a
		// pre-principal alias may migrate: legacy and canonical rows share one
		// key space, so a candidate in the stable principal namespace would
		// select another SSO user's live assignment.
		if candidate == "" || candidate == principal || isStableSSOPrincipal(candidate) {
			continue
		}
		assignment, ok := manager.GetUserAssignment(candidate)
		if !ok || len(assignment.RoleIDs) == 0 {
			continue
		}
		if migrator, ok := manager.(internalauth.AssignmentMigrator); ok {
			return migrator.MigrateUserAssignment(candidate, principal)
		}
		if principalHasRoles {
			return nil
		}
		return manager.UpdateUserRoles(principal, assignment.RoleIDs)
	}

	if principalHasRoles {
		return nil
	}
	if ensureListed {
		return manager.UpdateUserRoles(principal, nil)
	}
	return nil
}

// updateMappedSSORoles replaces the principal's roles with the result of the
// provider's group role mapping. Durable managers record that origin so the
// roles can be withdrawn when the mapping is removed; managers predating it
// remain compatible.
func updateMappedSSORoles(manager internalauth.Manager, principal string, mappedRoles []string) error {
	if mapped, ok := manager.(internalauth.MappedRoleManager); ok {
		return mapped.UpdateUserRolesFromMapping(principal, mappedRoles)
	}
	return manager.UpdateUserRoles(principal, mappedRoles)
}

// recordSSOIdentity keeps mutable presentation claims separate from the
// provider-scoped authorization principal. Managers predating identity
// metadata remain compatible; durable managers implement UserIdentityManager.
func recordSSOIdentity(manager internalauth.Manager, principal string, metadata internalauth.UserIdentityMetadata) error {
	if manager == nil || strings.TrimSpace(principal) == "" {
		return nil
	}
	identityManager, ok := manager.(internalauth.UserIdentityManager)
	if !ok {
		return nil
	}
	return identityManager.UpsertUserIdentity(principal, metadata)
}
