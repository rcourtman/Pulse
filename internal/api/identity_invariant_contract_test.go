package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readIdentityContractFile(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Clean(rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(body)
}

func TestContract_HostedIdentityUsesStablePrincipals(t *testing.T) {
	files := map[string]string{
		"magic_link_handlers.go":                     readIdentityContractFile(t, "magic_link_handlers.go"),
		"cloud_handoff.go":                           readIdentityContractFile(t, "cloud_handoff.go"),
		"cloud_handoff_handlers.go":                  readIdentityContractFile(t, "cloud_handoff_handlers.go"),
		"oidc_handlers.go":                           readIdentityContractFile(t, "oidc_handlers.go"),
		"authorization.go":                           readIdentityContractFile(t, "authorization.go"),
		"cloud_org_admin_auth.go":                    readIdentityContractFile(t, "cloud_org_admin_auth.go"),
		"payments_webhook_handlers.go":               readIdentityContractFile(t, "payments_webhook_handlers.go"),
		"saml_handlers.go":                           readIdentityContractFile(t, "saml_handlers.go"),
		"auth_principal_identity.go":                 readIdentityContractFile(t, "auth_principal_identity.go"),
		"security_tokens.go":                         readIdentityContractFile(t, "security_tokens.go"),
		"org_handlers.go":                            readIdentityContractFile(t, "org_handlers.go"),
		"agent_install_command_shared.go":            readIdentityContractFile(t, "agent_install_command_shared.go"),
		"deploy_handlers.go":                         readIdentityContractFile(t, "deploy_handlers.go"),
		"router.go":                                  readIdentityContractFile(t, "router.go"),
		"security_setup_fix.go":                      readIdentityContractFile(t, "security_setup_fix.go"),
		"public_signup_handlers.go":                  readIdentityContractFile(t, "public_signup_handlers.go"),
		"../cloudcp/account/tenant_handlers_test.go": readIdentityContractFile(t, "../cloudcp/account/tenant_handlers_test.go"),
		"../cloudcp/stripe/provisioner.go":           readIdentityContractFile(t, "../cloudcp/stripe/provisioner.go"),
		"../hosted/provisioner.go":                   readIdentityContractFile(t, "../hosted/provisioner.go"),
		"../models/organization.go":                  readIdentityContractFile(t, "../models/organization.go"),
		"../../docs/release-control/v6/internal/IDENTITY_INVARIANTS.md": readIdentityContractFile(t, "../../docs/release-control/v6/internal/IDENTITY_INVARIANTS.md"),
	}

	required := map[string][]string{
		"magic_link_handlers.go": {
			"resolveMagicLinkPrincipal",
			"CreateSession(sessionToken, sessionDuration, userAgent, clientIP, principal.UserID)",
			"TrackUserSession(principal.UserID, sessionToken)",
		},
		"cloud_handoff.go": {
			"CreateSession(sessionToken, sessionDuration, userAgent, clientIP, authz.UserID)",
			"TrackUserSession(authz.UserID, sessionToken)",
			"isEmailShapedHandoffUserID(userID)",
		},
		"cloud_handoff_handlers.go": {
			"CreateSession(sessionToken, sessionDuration, userAgent, clientIP, authz.UserID)",
			"TrackUserSession(authz.UserID, sessionToken)",
			"isEmailShapedHandoffUserID(userID)",
			"handoff user id must be a stable subject",
		},
		"oidc_handlers.go": {
			"stableSSOPrincipal(config.SSOProviderTypeOIDC, providerID, idToken.Subject)",
			"applySSORoleAssignments(authManager, principal",
			"establishOIDCSession(w, req, principal, oidcTokens)",
		},
		"saml_handlers.go": {
			"stableSSOPrincipal(config.SSOProviderTypeSAML, providerID, result.NameID)",
			"applySSORoleAssignments(authManager, principal",
			"establishSAMLSession(w, req, principal, samlSession)",
		},
		"auth_principal_identity.go": {
			"stableSSOPrincipal",
			"ssoLegacyPrincipalCandidates",
			"applySSORoleAssignments",
		},
		"authorization.go": {
			"org.CanUserIDAccess(userID)",
		},
		"cloud_org_admin_auth.go": {
			"org.IsOwnerUserID(userID)",
		},
		"org_handlers.go": {
			"org.CanUserIDAccess(username)",
			"org.CanUserIDManage(username)",
			"org.IsOwnerUserID(username)",
			"org.GetMemberRoleByUserID(username)",
		},
		"security_tokens.go": {
			"mergeAPITokenMetadata",
			"setAPITokenOwnerUserID",
			"reserved token metadata key",
			"apiTokenOwnerUserIDForRequest",
		},
		"agent_install_command_shared.go": {
			"OwnerUserID string",
			"setAPITokenOwnerUserID(record, opts.OwnerUserID)",
			"mergeAPITokenMetadata(record, opts.Metadata)",
		},
		"deploy_handlers.go": {
			"setAPITokenOwnerUserID(record, ownerUserID)",
			"setAPITokenOwnerUserID(runtimeRecord, apiTokenOwnerUserID(*bootstrapToken))",
			"apiTokenOwnerUserIDForRequest(h.config, r)",
		},
		"router.go": {
			"setAPITokenOwnerUserID(record, apiTokenOwnerUserIDForRequest(r.config, req))",
		},
		"security_setup_fix.go": {
			"org.CanUserIDManage(sessionUser)",
			"setAPITokenOwnerUserID(tokenRecord, setupRequest.Username)",
			"setAPITokenOwnerUserID(tokenRecord, apiTokenOwnerUserIDForRequest(r.config, rq))",
		},
		"public_signup_handlers.go": {
			"ownerEmail := strings.ToLower(strings.TrimSpace(result.OwnerEmail))",
			"GenerateToken(ownerEmail, orgID)",
			"SendMagicLink(ownerEmail, orgID, token, baseURL)",
		},
		"../cloudcp/stripe/provisioner.go": {
			"ownerUserID = strings.TrimSpace(user.ID)",
			"org.OwnerEmail = ownerEmail",
			"UserID:  seed.userID",
			"Email:   seed.email",
		},
		"../cloudcp/account/tenant_handlers_test.go": {
			"TestCreateWorkspace_SeedsStableOwnerFromAuthenticatedUserEmail",
			"reg.GetUserByEmail(\"operator@example.com\")",
			"org.OwnerUserID != operatorUser.ID",
			"org.GetMemberRoleByUserID(operatorUser.ID)",
			"org.GetMemberRoleByUserID(\"operator@example.com\")",
		},
		"../hosted/provisioner.go": {
			"newUserID:    registry.GenerateUserID",
			"OwnerEmail:  ownerEmail",
			"UpdateUserRoles(userID, []string{auth.RoleAdmin})",
			"generated user id must not be an email",
		},
		"../models/organization.go": {
			"OwnerEmail string",
			"ResolvePrincipalByEmail",
			"CanonicalizePrincipalIdentity",
			"return \"\", \"\", false",
		},
		"../../docs/release-control/v6/internal/IDENTITY_INVARIANTS.md": {
			"Email is contact metadata",
			"Pulse control-plane user ID",
			"SSO provider subject",
			"reject caller-supplied metadata",
			"Self-hosted SSO sessions now use provider-scoped",
		},
	}
	for file, needles := range required {
		for _, needle := range needles {
			if !strings.Contains(files[file], needle) {
				t.Fatalf("%s must contain %q", file, needle)
			}
		}
	}

	forbidden := map[string][]string{
		"magic_link_handlers.go": {
			"CreateSession(sessionToken, sessionDuration, userAgent, clientIP, token.Email)",
			"TrackUserSession(token.Email, sessionToken)",
		},
		"cloud_handoff.go": {
			"CreateSession(sessionToken, sessionDuration, userAgent, clientIP, email)",
			"TrackUserSession(email, sessionToken)",
			"userID = email",
		},
		"cloud_handoff_handlers.go": {
			"CreateSession(sessionToken, sessionDuration, userAgent, clientIP, claims.Email)",
			"TrackUserSession(claims.Email, sessionToken)",
			"userID = email",
			"email = normalizeHandoffEmail(userID)",
		},
		"oidc_handlers.go": {
			"establishOIDCSession(w, req, username, oidcTokens)",
			"UpdateUserRoles(username, rolesToAssign)",
		},
		"authorization.go": {
			"org.CanUserAccess(userID)",
		},
		"cloud_org_admin_auth.go": {
			"strings.EqualFold(strings.TrimSpace(org.OwnerUserID), strings.TrimSpace(userID))",
		},
		"org_handlers.go": {
			"org.CanUserAccess(username)",
			"org.CanUserManage(username)",
			"org.IsOwner(username)",
			"org.GetMemberRole(username)",
		},
		"saml_handlers.go": {
			"establishSAMLSession(w, req, username, samlSession)",
			"UpdateUserRoles(result.Username, rolesToAssign)",
		},
		"payments_webhook_handlers.go": {
			"sendTo = org.OwnerUserID",
			"sendTo = m.UserID",
		},
		"../cloudcp/stripe/provisioner.go": {
			"org.OwnerUserID = ownerEmail",
			"UserID:  seed.email",
			"memberSeeds[ownerEmail]",
		},
		"../cloudcp/account/tenant_handlers_test.go": {
			"TestCreateWorkspace_SeedsOwnerFromAuthenticatedUserEmail",
			"org.OwnerUserID != \"operator@example.com\"",
			"org.GetMemberRole(\"operator@example.com\")",
		},
		"public_signup_handlers.go": {
			"GenerateToken(userID, orgID)",
			"SendMagicLink(userID, orgID, token, baseURL)",
		},
		"../hosted/provisioner.go": {
			"OwnerUserID: ownerEmail",
			"UserID:  ownerEmail",
			"contactEmailForLegacyUserID",
		},
		"../models/organization.go": {
			"userID = email",
		},
	}
	for file, needles := range forbidden {
		for _, needle := range needles {
			if strings.Contains(files[file], needle) {
				t.Fatalf("%s must not contain legacy email-principal pattern %q", file, needle)
			}
		}
	}
}
