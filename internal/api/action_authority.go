package api

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type trustedActionActorContextKey struct{}

func withTrustedActionActor(ctx context.Context, actor unified.ActionActor) context.Context {
	return context.WithValue(ctx, trustedActionActorContextKey{}, unified.NormalizeActionActor(actor))
}

func trustedActionActorFromContext(ctx context.Context) (unified.ActionActor, bool) {
	actor, ok := ctx.Value(trustedActionActorContextKey{}).(unified.ActionActor)
	if !ok {
		return unified.ActionActor{}, false
	}
	actor = unified.NormalizeActionActor(actor)
	return actor, unified.ValidateActionActor(actor) == nil
}

func actionCredentialID(prefix, value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%s:%x", prefix, sum[:16])
}

func actionActorForRequest(cfg *config.Config, r *http.Request, orgID string) (unified.ActionActor, error) {
	orgID = strings.TrimSpace(orgID)
	if actor, ok := trustedActionActorFromContext(r.Context()); ok {
		if actor.OrgID != orgID {
			return unified.ActionActor{}, actionlifecycle.ErrActionAuthorizationDenied
		}
		return actor, nil
	}
	if record := getAPITokenRecordFromRequest(r); record != nil {
		subject := apiTokenAuthenticatedUser(record)
		actor := unified.ActionActor{
			SubjectID:    subject,
			Kind:         unified.ActionActorAPIToken,
			CredentialID: "api-token:" + strings.TrimSpace(record.ID),
			OrgID:        orgID,
		}
		if err := unified.ValidateActionActor(actor); err != nil {
			return unified.ActionActor{}, err
		}
		return actor, nil
	}
	user := strings.TrimSpace(auth.GetUser(r.Context()))
	if user == "" && cfg != nil {
		user = strings.TrimSpace(getAuthUsername(cfg, r))
	}
	if user == "" {
		return unified.ActionActor{}, actionlifecycle.ErrActionAuthorizationDenied
	}
	credentialID := ""
	if cookie, err := readSessionCookie(r); err == nil && cookie.Value != "" && ValidateSession(cookie.Value) {
		credentialID = actionCredentialID("session", cookie.Value)
	} else if cfg != nil && cfg.ProxyAuthSecret != "" {
		if valid, proxyUser, _ := CheckProxyAuth(cfg, r); valid && strings.EqualFold(strings.TrimSpace(proxyUser), user) {
			credentialID = actionCredentialID("proxy", user)
		}
	} else if authorization := strings.TrimSpace(r.Header.Get("Authorization")); authorization != "" {
		credentialID = actionCredentialID("http-auth", authorization)
	} else if adminBypassEnabled() {
		credentialID = "development-bypass"
	} else if strings.TrimSpace(auth.GetUser(r.Context())) != "" {
		// Auth context is populated only by trusted server middleware. This
		// covers integrations whose credential was verified upstream without
		// persisting or re-reading that credential here.
		credentialID = actionCredentialID("server-context", user)
	}
	if credentialID == "" {
		return unified.ActionActor{}, actionlifecycle.ErrActionAuthorizationDenied
	}
	return unified.ActionActor{SubjectID: user, Kind: unified.ActionActorUser, CredentialID: credentialID, OrgID: orgID}, nil
}

func approvalEvidenceForRequest(actor unified.ActionActor, record unified.ActionAuditRecord, outcome unified.ApprovalOutcome, now time.Time) unified.ApprovalEvidence {
	method := unified.MethodSession
	if actor.Kind == unified.ActionActorAPIToken {
		method = unified.MethodAPIToken
	}
	return unified.ApprovalEvidence{
		Version:  1,
		Method:   method,
		Actor:    actor,
		OrgID:    actor.OrgID,
		ActionID: record.ID,
		PlanHash: record.Plan.PlanHash,
		Outcome:  outcome,
		IssuedAt: now.UTC(),
	}
}

type actionAuthority struct {
	authorizer auth.Authorizer
	orgChecker *DefaultAuthorizationChecker
}

func (a actionAuthority) authorizeActor(ctx context.Context, orgID string, actor unified.ActionActor, capability string) error {
	actor = unified.NormalizeActionActor(actor)
	var token *config.APITokenRecord
	if actor.Kind == unified.ActionActorAPIToken {
		current, ok := auth.GetAPIToken(ctx).(*config.APITokenRecord)
		if !ok || current == nil || actor.CredentialID != "api-token:"+strings.TrimSpace(current.ID) {
			return actionlifecycle.ErrActionAuthorizationDenied
		}
		owner := apiTokenBoundUser(current)
		if owner == "" || !strings.EqualFold(owner, actor.SubjectID) || strings.HasPrefix(actor.SubjectID, "token:") {
			return actionlifecycle.ErrApprovalActorNotHuman
		}
		if !tokenHasActionCapabilityScope(current, capability) {
			return actionlifecycle.ErrActionAuthorizationDenied
		}
		token = current
	} else if actor.Kind != unified.ActionActorUser || !strings.EqualFold(strings.TrimSpace(auth.GetUser(ctx)), actor.SubjectID) {
		return actionlifecycle.ErrApprovalActorNotHuman
	}
	if a.orgChecker == nil || !a.orgChecker.CheckAccess(token, actor.SubjectID, orgID).Allowed {
		return actionlifecycle.ErrActionAuthorizationDenied
	}
	return authorizeActionCapability(ctx, a.authorizer, capability)
}

func tokenHasActionCapabilityScope(token *config.APITokenRecord, capability string) bool {
	if token == nil {
		return false
	}
	var scopes []string
	switch capability {
	case auth.ActionPlan:
		scopes = []string{config.ScopeActionsPlan, config.ScopeAIExecute}
	case auth.ActionApprove:
		scopes = []string{config.ScopeActionsApprove, config.ScopeAIExecute, config.ScopeRelayMobileAccess}
	case auth.ActionExecute:
		scopes = []string{config.ScopeActionsExecute, config.ScopeAIExecute, config.ScopeRelayMobileAccess}
	default:
		return false
	}
	for _, scope := range scopes {
		if token.HasScope(scope) {
			return true
		}
	}
	return false
}

func (a actionAuthority) AuthorizeDecision(ctx context.Context, orgID string, _ unified.ActionAuditRecord, decision unified.ActionDecision) error {
	return a.authorizeActor(ctx, orgID, decision.Actor, auth.ActionApprove)
}

func (a actionAuthority) AuthorizeExecution(ctx context.Context, orgID string, _ unified.ActionAuditRecord, actor unified.ActionActor) error {
	return a.authorizeActor(ctx, orgID, actor, auth.ActionExecute)
}

func authorizeActionCapability(ctx context.Context, authorizer auth.Authorizer, capability string) error {
	if authorizer == nil {
		return actionlifecycle.ErrActionAuthorizationDenied
	}
	allowed, err := authorizer.Authorize(ctx, capability, auth.ResourceActions)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	legacyAction := auth.ActionAdmin
	if capability == auth.ActionPlan {
		legacyAction = auth.ActionWrite
	}
	allowed, err = authorizer.Authorize(ctx, legacyAction, auth.ResourceAI)
	if err != nil {
		return err
	}
	if !allowed {
		return actionlifecycle.ErrActionAuthorizationDenied
	}
	return nil
}

func requireActionCapability(authorizer auth.Authorizer, capability string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := authorizeActionCapability(r.Context(), authorizer, capability); err != nil {
			writeJSONError(w, http.StatusForbidden, "action_capability_denied", "You do not have permission to perform this action")
			return
		}
		handler(w, r)
	}
}
