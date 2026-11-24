package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func orgBoundOwnedToken(owner, orgID string, scopes ...string) *config.APITokenRecord {
	record := &config.APITokenRecord{ID: "token", OrgID: orgID, Scopes: scopes}
	setAPITokenOwnerUserID(record, owner)
	return record
}

// A token that acts for its owner (Pulse Mobile, Assistant, actions, full
// access) must not outlive the owner's membership of the organization it is
// bound to. Machine and plain monitoring credentials follow the binding only.
func TestCheckAccessRequiresMembershipOfTokenOwner(t *testing.T) {
	checker := NewAuthorizationChecker(staticOrgLoader{org: &models.Organization{
		ID:          "acme",
		OwnerUserID: "alice",
		Members: []models.OrganizationMember{
			{UserID: "alice", Role: models.OrgRoleOwner},
			{UserID: "carol", Role: models.OrgRoleViewer},
		},
	}})
	bindingOnly := NewAuthorizationChecker(nil)

	for _, tc := range []struct {
		name    string
		checker *DefaultAuthorizationChecker
		token   *config.APITokenRecord
		orgID   string
		allowed bool
	}{
		{"mobile token of the owner", checker, orgBoundOwnedToken("alice", "acme", config.ScopeRelayMobileAccess), "acme", true},
		{"mobile token of a member", checker, orgBoundOwnedToken("carol", "acme", config.ScopeRelayMobileAccess), "acme", true},
		{"mobile token of a removed member", checker, orgBoundOwnedToken("bob", "acme", config.ScopeRelayMobileAccess), "acme", false},
		{"assistant token of a removed member", checker, orgBoundOwnedToken("bob", "acme", config.ScopeAIChat), "acme", false},
		{"action token of a removed member", checker, orgBoundOwnedToken("bob", "acme", config.ScopeMonitoringRead, config.ScopeActionsApprove), "acme", false},
		{"full-access token of a removed member", checker, orgBoundOwnedToken("bob", "acme", config.ScopeWildcard), "acme", false},
		{"agent token issued by a removed member", checker, orgBoundOwnedToken("bob", "acme", config.ScopeAgentReport, config.ScopeAgentConfigRead, config.ScopeAgentExec), "acme", true},
		{"monitoring token issued by a removed member", checker, orgBoundOwnedToken("bob", "acme", config.ScopeMonitoringRead), "acme", true},
		{"ownerless token", checker, orgBoundOwnedToken("", "acme", config.ScopeRelayMobileAccess), "acme", true},
		{"binding still decides first", checker, orgBoundOwnedToken("alice", "other", config.ScopeRelayMobileAccess), "acme", false},
		{"default organization keeps the binding rule", checker, orgBoundOwnedToken("bob", "default", config.ScopeRelayMobileAccess), "default", true},
		{"checker without membership data rechecks the binding", bindingOnly, orgBoundOwnedToken("bob", "acme", config.ScopeRelayMobileAccess), "acme", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.checker.CheckAccess(tc.token, apiTokenAuthenticatedUser(tc.token), tc.orgID)
			if result.Allowed != tc.allowed {
				t.Fatalf("allowed = %v (%s), want %v", result.Allowed, result.Reason, tc.allowed)
			}
		})
	}

	result := checker.CheckAccess(orgBoundOwnedToken("bob", "acme", config.ScopeRelayMobileAccess), "bob", "acme")
	if result.Reason != "Token owner is not a member of the organization" {
		t.Fatalf("reason = %q", result.Reason)
	}
}

// Approving and executing a governed action recheck the same policy, so the
// owner of a relay-mobile token must still belong to the organization.
func TestActionAuthorityRequiresMembershipOfTokenOwner(t *testing.T) {
	authority := actionAuthority{
		authorizer: allowActionAuthorityAuthorizer{},
		orgChecker: NewAuthorizationChecker(staticOrgLoader{org: &models.Organization{
			ID:      "acme",
			Members: []models.OrganizationMember{{UserID: "alice", Role: models.OrgRoleAdmin}},
		}}),
	}

	for owner, want := range map[string]error{"alice": nil, "bob": actionlifecycle.ErrActionAuthorizationDenied} {
		record := orgBoundOwnedToken(owner, "acme", config.ScopeRelayMobileAccess)
		actor := unified.ActionActor{SubjectID: owner, Kind: unified.ActionActorAPIToken, CredentialID: "api-token:" + record.ID, OrgID: "acme"}
		for _, capability := range []string{auth.ActionApprove, auth.ActionExecute} {
			if err := authority.authorizeActor(actionAuthorityContext(record), "acme", actor, capability); !errors.Is(err, want) {
				t.Fatalf("%s %s = %v, want %v", owner, capability, err, want)
			}
		}
	}
}

// A stream admitted for such a token ends when its owner is removed, as a
// signed-in user's stream does.
func TestTokenStreamRevalidationFollowsOwnerMembership(t *testing.T) {
	const raw = "synthetic-mobile-token-123.12345678"
	record := newTokenRecord(t, raw, []string{config.ScopeRelayMobileAccess}, nil)
	record.OrgID = "acme"
	setAPITokenOwnerUserID(&record, "bob")
	cfg := newTestConfigWithTokens(t, record)

	loader := &staticOrgLoader{org: &models.Organization{
		ID:      "acme",
		Members: []models.OrganizationMember{{UserID: "bob", Role: models.OrgRoleAdmin}},
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/chat", nil)
	req.Header.Set("X-API-Token", raw)
	attachAPITokenRecord(req, &record)
	ctx := context.WithValue(req.Context(), OrgIDContextKey, "acme")
	req = req.WithContext(withOrgAccessChecker(ctx, NewAuthorizationChecker(loader)))

	revalidate := tokenStreamSnapshotRevalidator(cfg, req, config.ScopeRelayMobileAccess)
	if revalidate == nil {
		t.Fatal("token request has no revalidator")
	}
	if _, ok := revalidate(); !ok {
		t.Fatal("member's token stream was not admitted")
	}
	loader.org = &models.Organization{ID: "acme"}
	if _, ok := revalidate(); ok {
		t.Fatal("token stream outlived its owner's membership")
	}
}
