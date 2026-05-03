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
		"magic_link_handlers.go":                                        readIdentityContractFile(t, "magic_link_handlers.go"),
		"cloud_handoff.go":                                              readIdentityContractFile(t, "cloud_handoff.go"),
		"cloud_handoff_handlers.go":                                     readIdentityContractFile(t, "cloud_handoff_handlers.go"),
		"payments_webhook_handlers.go":                                  readIdentityContractFile(t, "payments_webhook_handlers.go"),
		"../cloudcp/stripe/provisioner.go":                              readIdentityContractFile(t, "../cloudcp/stripe/provisioner.go"),
		"../models/organization.go":                                     readIdentityContractFile(t, "../models/organization.go"),
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
		},
		"cloud_handoff_handlers.go": {
			"CreateSession(sessionToken, sessionDuration, userAgent, clientIP, authz.UserID)",
			"TrackUserSession(authz.UserID, sessionToken)",
		},
		"../cloudcp/stripe/provisioner.go": {
			"ownerUserID = strings.TrimSpace(user.ID)",
			"org.OwnerEmail = ownerEmail",
			"UserID:  seed.userID",
			"Email:   seed.email",
		},
		"../models/organization.go": {
			"OwnerEmail string",
			"ResolvePrincipalByEmail",
			"CanonicalizePrincipalIdentity",
		},
		"../../docs/release-control/v6/internal/IDENTITY_INVARIANTS.md": {
			"Email is contact metadata",
			"Pulse control-plane user ID",
			"SSO provider subject",
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
		},
		"cloud_handoff_handlers.go": {
			"CreateSession(sessionToken, sessionDuration, userAgent, clientIP, claims.Email)",
			"TrackUserSession(claims.Email, sessionToken)",
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
	}
	for file, needles := range forbidden {
		for _, needle := range needles {
			if strings.Contains(files[file], needle) {
				t.Fatalf("%s must not contain legacy email-principal pattern %q", file, needle)
			}
		}
	}
}
