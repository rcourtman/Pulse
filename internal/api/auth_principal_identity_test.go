package api

import (
	"errors"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type ssoIdentityRBACManager struct {
	auth.Manager
	assignments  map[string]auth.UserRoleAssignment
	updatedUser  string
	updatedRoles []string
	identityUser string
	identity     auth.UserIdentityMetadata
}

func (m *ssoIdentityRBACManager) UpsertUserIdentity(username string, metadata auth.UserIdentityMetadata) error {
	m.identityUser = username
	m.identity = metadata
	return nil
}

func (m *ssoIdentityRBACManager) GetUserAssignment(username string) (auth.UserRoleAssignment, bool) {
	if m.assignments == nil {
		return auth.UserRoleAssignment{}, false
	}
	assignment, ok := m.assignments[username]
	return assignment, ok
}

func (m *ssoIdentityRBACManager) UpdateUserRoles(username string, roleIDs []string) error {
	m.updatedUser = username
	m.updatedRoles = append([]string{}, roleIDs...)
	if m.assignments == nil {
		m.assignments = make(map[string]auth.UserRoleAssignment)
	}
	m.assignments[username] = auth.UserRoleAssignment{Username: username, RoleIDs: append([]string{}, roleIDs...)}
	return nil
}

func TestStableSSOPrincipalUsesProviderScopedOpaqueSubject(t *testing.T) {
	principal, err := stableSSOPrincipal(config.SSOProviderTypeOIDC, "okta", "alice@example.com")
	if err != nil {
		t.Fatalf("stableSSOPrincipal error: %v", err)
	}
	again, err := stableSSOPrincipal(config.SSOProviderTypeOIDC, "okta", "alice@example.com")
	if err != nil {
		t.Fatalf("stableSSOPrincipal repeat error: %v", err)
	}
	otherProvider, err := stableSSOPrincipal(config.SSOProviderTypeOIDC, "entra", "alice@example.com")
	if err != nil {
		t.Fatalf("stableSSOPrincipal other provider error: %v", err)
	}

	if principal != again {
		t.Fatalf("stable principal is not deterministic: %q != %q", principal, again)
	}
	if principal == otherProvider {
		t.Fatal("stable principal must be scoped by provider")
	}
	if !strings.HasPrefix(principal, "sso:oidc:okta:") {
		t.Fatalf("principal %q missing expected provider scope", principal)
	}
	if strings.Contains(principal, "alice") || strings.Contains(principal, "example.com") {
		t.Fatalf("principal %q must not expose the raw email subject", principal)
	}
}

func TestStableSSOPrincipalRequiresSubject(t *testing.T) {
	if _, err := stableSSOPrincipal(config.SSOProviderTypeSAML, "okta", ""); err == nil {
		t.Fatal("expected missing subject to fail")
	}
}

func TestRecordSSOIdentityKeepsDisplayClaimsSeparate(t *testing.T) {
	manager := &ssoIdentityRBACManager{}
	principal := "sso:oidc:okta:stable"
	metadata := auth.UserIdentityMetadata{DisplayName: "Alice Example", Email: "alice@example.com", ProviderType: "oidc", ProviderID: "okta"}
	if err := recordSSOIdentity(manager, principal, metadata); err != nil {
		t.Fatalf("recordSSOIdentity: %v", err)
	}
	if manager.identityUser != principal || manager.identity.DisplayName != "Alice Example" || manager.identity.Email != "alice@example.com" {
		t.Fatalf("identity write = user %q metadata %#v", manager.identityUser, manager.identity)
	}
}

func TestApplySSORoleAssignmentsMigratesLegacyRoles(t *testing.T) {
	manager := &ssoIdentityRBACManager{
		assignments: map[string]auth.UserRoleAssignment{
			"alice@example.com": {Username: "alice@example.com", RoleIDs: []string{"admin", "viewer"}},
		},
	}
	principal := "sso:oidc:okta:stable"

	err := applySSORoleAssignments(manager, principal, ssoLegacyPrincipalCandidates("Alice@Example.com"), nil, false, true)
	if err != nil {
		t.Fatalf("applySSORoleAssignments error: %v", err)
	}

	if manager.updatedUser != principal {
		t.Fatalf("updated user = %q, want %q", manager.updatedUser, principal)
	}
	if got := strings.Join(manager.updatedRoles, ","); got != "admin,viewer" {
		t.Fatalf("updated roles = %q, want admin,viewer", got)
	}
}

func TestApplySSORoleAssignmentsUsesAuthoritativeMapping(t *testing.T) {
	manager := &ssoIdentityRBACManager{
		assignments: map[string]auth.UserRoleAssignment{
			"alice@example.com": {Username: "alice@example.com", RoleIDs: []string{"admin"}},
		},
	}
	principal := "sso:saml:okta:stable"

	err := applySSORoleAssignments(manager, principal, []string{"alice@example.com"}, []string{"viewer"}, true, true)
	if err != nil {
		t.Fatalf("applySSORoleAssignments error: %v", err)
	}

	if manager.updatedUser != principal {
		t.Fatalf("updated user = %q, want %q", manager.updatedUser, principal)
	}
	if got := strings.Join(manager.updatedRoles, ","); got != "viewer" {
		t.Fatalf("updated roles = %q, want viewer", got)
	}
}

func TestApplySSORoleAssignmentsMovesLegacySQLiteAlias(t *testing.T) {
	manager, err := auth.NewSQLiteManager(auth.SQLiteManagerConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewSQLiteManager: %v", err)
	}
	defer manager.Close()
	if err := manager.UpdateUserRoles("alice@example.com", []string{auth.RoleViewer}); err != nil {
		t.Fatalf("seed legacy assignment: %v", err)
	}
	principal := "sso:oidc:okta:stable"

	if err := applySSORoleAssignments(
		manager,
		principal,
		[]string{"alice@example.com"},
		nil,
		false,
		true,
	); err != nil {
		t.Fatalf("applySSORoleAssignments: %v", err)
	}

	assignment, ok := manager.GetUserAssignment(principal)
	if !ok || len(assignment.RoleIDs) != 1 || assignment.RoleIDs[0] != auth.RoleViewer {
		t.Fatalf("canonical assignment = %#v, exists=%v", assignment, ok)
	}
	if stale, ok := manager.GetUserAssignment("alice@example.com"); ok {
		t.Fatalf("legacy alias retained a reusable grant: %#v", stale)
	}
}

func TestApplySSORoleAssignmentsRejectsConflictingLegacySQLiteAlias(t *testing.T) {
	manager, err := auth.NewSQLiteManager(auth.SQLiteManagerConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewSQLiteManager: %v", err)
	}
	defer manager.Close()
	if err := manager.UpdateUserRoles("alice@example.com", []string{auth.RoleAdmin}); err != nil {
		t.Fatalf("seed legacy assignment: %v", err)
	}
	principal := "sso:oidc:okta:stable"
	if err := manager.UpdateUserRoles(principal, []string{auth.RoleViewer}); err != nil {
		t.Fatalf("seed canonical assignment: %v", err)
	}

	err = applySSORoleAssignments(
		manager,
		principal,
		[]string{"alice@example.com"},
		nil,
		false,
		true,
	)
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("error = %v, want a conflicting-grant failure", err)
	}
	canonical, ok := manager.GetUserAssignment(principal)
	if !ok || len(canonical.RoleIDs) != 1 || canonical.RoleIDs[0] != auth.RoleViewer {
		t.Fatalf("canonical assignment changed: %#v, exists=%v", canonical, ok)
	}
	legacy, ok := manager.GetUserAssignment("alice@example.com")
	if !ok || len(legacy.RoleIDs) != 1 || legacy.RoleIDs[0] != auth.RoleAdmin {
		t.Fatalf("legacy assignment changed before conflict resolution: %#v, exists=%v", legacy, ok)
	}
}

func TestApplySSORoleAssignmentsWithdrawsMappedRolesWhenMappingIsRemoved(t *testing.T) {
	manager, err := auth.NewSQLiteManager(auth.SQLiteManagerConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewSQLiteManager: %v", err)
	}
	defer manager.Close()
	mapped := "sso:oidc:okta:mapped"
	manual := "sso:oidc:okta:manual"

	// While the provider maps groups, sign-in saves the mapped role.
	if err := applySSORoleAssignments(manager, mapped, nil, []string{auth.RoleAdmin}, true, true); err != nil {
		t.Fatalf("mapped sign-in: %v", err)
	}
	if err := manager.UpdateUserRoles(manual, []string{auth.RoleOperator}); err != nil {
		t.Fatalf("seed manual assignment: %v", err)
	}

	// The last mapping is removed: the next sign-in is no longer authoritative.
	for _, principal := range []string{mapped, manual} {
		if err := applySSORoleAssignments(manager, principal, nil, nil, false, true); err != nil {
			t.Fatalf("sign-in without mapping for %s: %v", principal, err)
		}
	}

	assignment, ok := manager.GetUserAssignment(mapped)
	if !ok || len(assignment.RoleIDs) != 0 || assignment.RolesFromMapping {
		t.Fatalf("roles saved by a removed mapping survived: %#v, exists=%v", assignment, ok)
	}
	assignment, ok = manager.GetUserAssignment(manual)
	if !ok || len(assignment.RoleIDs) != 1 || assignment.RoleIDs[0] != auth.RoleOperator {
		t.Fatalf("manual assignment was not preserved: %#v, exists=%v", assignment, ok)
	}

	// Roles an administrator saves for a mapped user are theirs to keep.
	if err := applySSORoleAssignments(manager, mapped, nil, []string{auth.RoleAdmin}, true, true); err != nil {
		t.Fatalf("mapped sign-in: %v", err)
	}
	if err := manager.UpdateUserRoles(mapped, []string{auth.RoleViewer}); err != nil {
		t.Fatalf("manual reassignment: %v", err)
	}
	if err := applySSORoleAssignments(manager, mapped, nil, nil, false, true); err != nil {
		t.Fatalf("sign-in without mapping: %v", err)
	}
	assignment, ok = manager.GetUserAssignment(mapped)
	if !ok || len(assignment.RoleIDs) != 1 || assignment.RoleIDs[0] != auth.RoleViewer {
		t.Fatalf("manual reassignment was not preserved: %#v, exists=%v", assignment, ok)
	}
}

func TestStableSSOPrincipalRefusesSubjectPaddedOntoAnotherAccount(t *testing.T) {
	for _, providerType := range []config.SSOProviderType{config.SSOProviderTypeOIDC, config.SSOProviderTypeSAML} {
		victim, err := stableSSOPrincipal(providerType, "company", "alice")
		if err != nil {
			t.Fatalf("stableSSOPrincipal victim: %v", err)
		}
		for _, subject := range []string{"alice ", " alice", "alice\t", "alice\n", "alice\u00a0", "\u2003alice", "alice\u3000", "\u00a0"} {
			if principal, err := stableSSOPrincipal(providerType, "company", subject); err == nil {
				t.Fatalf("%s subject %q must be refused, got principal %q (subject %q has %q)", providerType, subject, principal, "alice", victim)
			}
		}
		spaced, err := stableSSOPrincipal(providerType, "company", "alice smith")
		if err != nil {
			t.Fatalf("stableSSOPrincipal interior whitespace: %v", err)
		}
		if spaced == victim {
			t.Fatalf("%s subjects %q and %q share a principal", providerType, "alice smith", "alice")
		}
	}
}

func TestStableSSOPrincipalForIssuerIsolatesRepointedEntriesWithoutRenamingExisting(t *testing.T) {
	manager, err := auth.NewSQLiteManager(auth.SQLiteManagerConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewSQLiteManager: %v", err)
	}
	defer manager.Close()

	base, err := stableSSOPrincipal(config.SSOProviderTypeOIDC, "company", "2")
	if err != nil {
		t.Fatalf("stableSSOPrincipal: %v", err)
	}
	// Original issuer keeps compatibility principal format (no existing users renamed).
	original, err := stableSSOPrincipalForIssuer(manager, config.SSOProviderTypeOIDC, "company", "https://old-idp.example.com", "2")
	if err != nil {
		t.Fatalf("stableSSOPrincipalForIssuer original: %v", err)
	}
	if original != base {
		t.Fatalf("original issuer principal = %q, want %q", original, base)
	}
	// Repointing or recreating the same entry ID for another issuer yields a distinct principal.
	repointed, err := stableSSOPrincipalForIssuer(manager, config.SSOProviderTypeOIDC, "company", "https://new-idp.example.com", "2")
	if err != nil {
		t.Fatalf("stableSSOPrincipalForIssuer repointed: %v", err)
	}
	if repointed == original {
		t.Fatalf("repointed issuer collided with original principal %q", original)
	}
	// Returning to the original issuer restores the original principal.
	restored, err := stableSSOPrincipalForIssuer(manager, config.SSOProviderTypeOIDC, "company", "https://old-idp.example.com", "2")
	if err != nil {
		t.Fatalf("stableSSOPrincipalForIssuer restored: %v", err)
	}
	if restored != original {
		t.Fatalf("restored principal = %q, want %q", restored, original)
	}
	// Unknown original issuer sentinel fails closed for any real issuer.
	bindSSOProviderOriginalIssuer(manager, config.SSOProviderTypeSAML, "lazy-saml", "", true)
	samlBase, err := stableSSOPrincipal(config.SSOProviderTypeSAML, "lazy-saml", "2")
	if err != nil {
		t.Fatalf("stableSSOPrincipal saml: %v", err)
	}
	samlAfterEdit, err := stableSSOPrincipalForIssuer(manager, config.SSOProviderTypeSAML, "lazy-saml", "https://idp.example.com/saml", "2")
	if err != nil {
		t.Fatalf("stableSSOPrincipalForIssuer lazy-saml: %v", err)
	}
	if samlAfterEdit == samlBase {
		t.Fatalf("lazy SAML entry edited with unknown issuer must fail closed, got %q", samlAfterEdit)
	}
}

func TestApplySSORoleAssignmentsIgnoresStablePrincipalCandidates(t *testing.T) {
	victim, err := stableSSOPrincipal(config.SSOProviderTypeOIDC, "corp", "1042")
	if err != nil {
		t.Fatalf("stableSSOPrincipal victim: %v", err)
	}
	principal, err := stableSSOPrincipal(config.SSOProviderTypeOIDC, "guests", "7")
	if err != nil {
		t.Fatalf("stableSSOPrincipal caller: %v", err)
	}
	candidates := ssoLegacyPrincipalCandidates("mallory", "mallory@guests.example", " "+victim)

	t.Run("sqlite migrator", func(t *testing.T) {
		manager, err := auth.NewSQLiteManager(auth.SQLiteManagerConfig{DataDir: t.TempDir()})
		if err != nil {
			t.Fatalf("NewSQLiteManager: %v", err)
		}
		defer manager.Close()
		if err := manager.UpdateUserRoles(victim, []string{auth.RoleAdmin}); err != nil {
			t.Fatalf("seed canonical assignment: %v", err)
		}
		if err := applySSORoleAssignments(manager, principal, candidates, nil, false, true); err != nil {
			t.Fatalf("applySSORoleAssignments: %v", err)
		}
		kept, ok := manager.GetUserAssignment(victim)
		if !ok || len(kept.RoleIDs) != 1 || kept.RoleIDs[0] != auth.RoleAdmin {
			t.Fatalf("another principal's assignment changed: %#v, exists=%v", kept, ok)
		}
		if got, ok := manager.GetUserAssignment(principal); !ok || len(got.RoleIDs) != 0 {
			t.Fatalf("caller assignment = %#v, exists=%v, want listed without roles", got, ok)
		}
		// Defense-in-depth: direct MigrateUserAssignment also refuses sso: source.
		if err := manager.MigrateUserAssignment(victim, principal); err == nil {
			t.Fatal("MigrateUserAssignment must refuse canonical sso: principal as source")
		}
	})

	t.Run("copy fallback", func(t *testing.T) {
		manager := &ssoIdentityRBACManager{
			assignments: map[string]auth.UserRoleAssignment{
				victim: {Username: victim, RoleIDs: []string{auth.RoleAdmin}},
			},
		}
		if err := applySSORoleAssignments(manager, principal, candidates, nil, false, true); err != nil {
			t.Fatalf("applySSORoleAssignments: %v", err)
		}
		if manager.updatedUser != principal || len(manager.updatedRoles) != 0 {
			t.Fatalf("caller roles = %q %v, want listed without roles", manager.updatedUser, manager.updatedRoles)
		}
	})
}

func TestApplySSORoleAssignmentsReplacesStaleRolesWhenMappingNamesMissingRole(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mapped []string
		want   string
	}{
		{name: "only a missing role", mapped: []string{"deleted-role"}, want: ""},
		{name: "missing role beside an existing one", mapped: []string{"deleted-role", auth.RoleViewer}, want: auth.RoleViewer},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager, err := auth.NewSQLiteManager(auth.SQLiteManagerConfig{DataDir: t.TempDir()})
			if err != nil {
				t.Fatalf("NewSQLiteManager: %v", err)
			}
			defer manager.Close()
			principal := "sso:oidc:okta:stable"
			if err := manager.UpdateUserRoles(principal, []string{auth.RoleAdmin}); err != nil {
				t.Fatalf("seed earlier sign-in: %v", err)
			}

			err = applySSORoleAssignments(manager, principal, nil, tc.mapped, true, true)
			if err == nil || errors.Is(err, errSSORolesNotReplaced) {
				t.Fatalf("applySSORoleAssignments error = %v, want the recoverable store error", err)
			}
			assignment, ok := manager.GetUserAssignment(principal)
			if got := strings.Join(assignment.RoleIDs, ","); !ok || got != tc.want {
				t.Fatalf("roles after a mapping with a missing role = %q, exists=%v, want %q", got, ok, tc.want)
			}
		})
	}
}

type ssoFailingRBACManager struct {
	auth.Manager
}

func (ssoFailingRBACManager) UpdateUserRoles(string, []string) error {
	return errors.New("store unavailable")
}

func TestApplySSORoleAssignmentsReportsUnreplacedRoles(t *testing.T) {
	err := applySSORoleAssignments(ssoFailingRBACManager{}, "sso:saml:okta:stable", nil, nil, true, false)
	if !errors.Is(err, errSSORolesNotReplaced) {
		t.Fatalf("applySSORoleAssignments error = %v, want errSSORolesNotReplaced", err)
	}
}
