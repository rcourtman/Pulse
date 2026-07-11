package api

import (
	"context"
	"errors"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type allowActionAuthorityAuthorizer struct{}

func (allowActionAuthorityAuthorizer) Authorize(context.Context, string, string) (bool, error) {
	return true, nil
}

type fixedActionAuthorityAuthorizer bool

func (a fixedActionAuthorityAuthorizer) Authorize(context.Context, string, string) (bool, error) {
	return bool(a), nil
}

func ownerBoundActionToken(id, owner string, scopes ...string) *config.APITokenRecord {
	record := &config.APITokenRecord{ID: id, OrgID: "default", Scopes: scopes}
	setAPITokenOwnerUserID(record, owner)
	return record
}

func tokenActionActor(record *config.APITokenRecord) unified.ActionActor {
	return unified.ActionActor{
		SubjectID: apiTokenAuthenticatedUser(record), Kind: unified.ActionActorAPIToken,
		CredentialID: "api-token:" + record.ID, OrgID: "default",
	}
}

func actionAuthorityContext(record *config.APITokenRecord) context.Context {
	ctx := auth.WithUser(context.Background(), apiTokenAuthenticatedUser(record))
	return auth.WithAPIToken(ctx, record)
}

func testActionAuthority() actionAuthority {
	return actionAuthority{authorizer: allowActionAuthorityAuthorizer{}, orgChecker: NewAuthorizationChecker(nil)}
}

func TestActionAuthorityAllowsOwnerBoundTokenWithCanonicalApproveAndExecuteScopes(t *testing.T) {
	authority := testActionAuthority()
	record := ownerBoundActionToken("owned", "alice", config.ScopeActionsApprove, config.ScopeActionsExecute)
	ctx := actionAuthorityContext(record)
	actor := tokenActionActor(record)
	if err := authority.authorizeActor(ctx, "default", actor, auth.ActionApprove); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := authority.authorizeActor(ctx, "default", actor, auth.ActionExecute); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func TestActionAuthorityRejectsOwnerBoundTokenWithoutApplicableScope(t *testing.T) {
	record := ownerBoundActionToken("wrong-scope", "alice", config.ScopeMonitoringRead)
	err := testActionAuthority().authorizeActor(actionAuthorityContext(record), "default", tokenActionActor(record), auth.ActionApprove)
	if !errors.Is(err, actionlifecycle.ErrActionAuthorizationDenied) {
		t.Fatalf("error=%v", err)
	}
}

func TestActionAuthorityRejectsDetachedTokenEvenWithApprovalScope(t *testing.T) {
	record := &config.APITokenRecord{ID: "detached", OrgID: "default", Scopes: []string{config.ScopeActionsApprove}}
	err := testActionAuthority().authorizeActor(actionAuthorityContext(record), "default", tokenActionActor(record), auth.ActionApprove)
	if !errors.Is(err, actionlifecycle.ErrApprovalActorNotHuman) {
		t.Fatalf("error=%v", err)
	}
}

func TestActionAuthorityLegacyCompatibilityScopesAreExactlyEnumerated(t *testing.T) {
	for _, tc := range []struct {
		name       string
		capability string
		scope      string
		allowed    bool
	}{
		{name: "plan ai execute", capability: auth.ActionPlan, scope: config.ScopeAIExecute, allowed: true},
		{name: "approve ai execute", capability: auth.ActionApprove, scope: config.ScopeAIExecute, allowed: true},
		{name: "approve relay mobile", capability: auth.ActionApprove, scope: config.ScopeRelayMobileAccess, allowed: true},
		{name: "execute ai execute", capability: auth.ActionExecute, scope: config.ScopeAIExecute, allowed: true},
		{name: "execute relay mobile", capability: auth.ActionExecute, scope: config.ScopeRelayMobileAccess, allowed: true},
		{name: "plan relay mobile denied", capability: auth.ActionPlan, scope: config.ScopeRelayMobileAccess, allowed: false},
		{name: "unrelated denied", capability: auth.ActionApprove, scope: config.ScopeMonitoringRead, allowed: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := ownerBoundActionToken("compat", "alice", tc.scope)
			err := testActionAuthority().authorizeActor(actionAuthorityContext(record), "default", tokenActionActor(record), tc.capability)
			if tc.allowed && err != nil {
				t.Fatalf("error=%v", err)
			}
			if !tc.allowed && !errors.Is(err, actionlifecycle.ErrActionAuthorizationDenied) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestActionAuthorityScopeRevocationDeniesBeforeDecisionOrExecution(t *testing.T) {
	authority := testActionAuthority()
	record := ownerBoundActionToken("revoked", "alice", config.ScopeActionsApprove, config.ScopeActionsExecute)
	ctx := actionAuthorityContext(record)
	actor := tokenActionActor(record)
	if err := authority.authorizeActor(ctx, "default", actor, auth.ActionApprove); err != nil {
		t.Fatal(err)
	}
	record.Scopes = []string{config.ScopeMonitoringRead}
	if err := authority.authorizeActor(ctx, "default", actor, auth.ActionApprove); !errors.Is(err, actionlifecycle.ErrActionAuthorizationDenied) {
		t.Fatalf("approve after revocation=%v", err)
	}
	if err := authority.authorizeActor(ctx, "default", actor, auth.ActionExecute); !errors.Is(err, actionlifecycle.ErrActionAuthorizationDenied) {
		t.Fatalf("execute after revocation=%v", err)
	}
}

func TestHandleDecideActionRejectsViewerSessionDespiteSessionScopeBypass(t *testing.T) {
	authority := actionAuthority{authorizer: fixedActionAuthorityAuthorizer(false), orgChecker: NewAuthorizationChecker(nil)}
	actor := unified.ActionActor{SubjectID: "viewer", Kind: unified.ActionActorUser, CredentialID: "session:viewer", OrgID: "default"}
	ctx := auth.WithUser(context.Background(), "viewer")
	if err := authority.authorizeActor(ctx, "default", actor, auth.ActionApprove); !errors.Is(err, actionlifecycle.ErrActionAuthorizationDenied) {
		t.Fatalf("error=%v", err)
	}
}

func TestHandleDecideActionRejectsDetachedTokenAsHumanApprover(t *testing.T) {
	record := &config.APITokenRecord{ID: "detached-handler", OrgID: "default", Scopes: []string{config.ScopeActionsApprove}}
	err := testActionAuthority().authorizeActor(actionAuthorityContext(record), "default", tokenActionActor(record), auth.ActionApprove)
	if !errors.Is(err, actionlifecycle.ErrApprovalActorNotHuman) {
		t.Fatalf("error=%v", err)
	}
}

func TestHandleDecideActionRejectsOwnerBoundTokenAfterRoleRevocation(t *testing.T) {
	record := ownerBoundActionToken("role-revoked", "alice", config.ScopeActionsApprove)
	authority := actionAuthority{authorizer: fixedActionAuthorityAuthorizer(false), orgChecker: NewAuthorizationChecker(nil)}
	err := authority.authorizeActor(actionAuthorityContext(record), "default", tokenActionActor(record), auth.ActionApprove)
	if !errors.Is(err, actionlifecycle.ErrActionAuthorizationDenied) {
		t.Fatalf("error=%v", err)
	}
}

func TestHandleExecuteActionRejectsCrossOrgAuditLookup(t *testing.T) {
	record := ownerBoundActionToken("cross-org", "alice", config.ScopeActionsExecute)
	actor := tokenActionActor(record)
	if err := testActionAuthority().authorizeActor(actionAuthorityContext(record), "other-org", actor, auth.ActionExecute); !errors.Is(err, actionlifecycle.ErrActionAuthorizationDenied) {
		t.Fatalf("error=%v", err)
	}
}

func TestHandleExecuteActionRejectsExecutorWithoutCurrentCapability(t *testing.T) {
	record := ownerBoundActionToken("execute-role-revoked", "alice", config.ScopeActionsExecute)
	authority := actionAuthority{authorizer: fixedActionAuthorityAuthorizer(false), orgChecker: NewAuthorizationChecker(nil)}
	err := authority.authorizeActor(actionAuthorityContext(record), "default", tokenActionActor(record), auth.ActionExecute)
	if !errors.Is(err, actionlifecycle.ErrActionAuthorizationDenied) {
		t.Fatalf("error=%v", err)
	}
}
