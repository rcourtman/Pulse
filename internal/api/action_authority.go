package api

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
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
	if a.orgChecker == nil || !a.orgChecker.CheckAccessContext(ctx, token, actor.SubjectID, orgID).Allowed {
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

// AuthorizeLiveCheck admits the agent's check of an action before dispatch
// (an apt-get -s upgrade for OS updates, a container inspect) only for callers
// who could execute it. Planning, refreshing, reviewing and approving need
// their own capabilities; asking the agent to do work needs execute, as every
// other route that reaches an agent does. Service and policy actors are
// server-stamped broker identities and cannot come from a request; a broker
// that plans on a person's behalf binds that person's authority with
// actionlifecycle.WithLiveCheckAuthority.
func (a actionAuthority) AuthorizeLiveCheck(ctx context.Context, orgID string, actor unified.ActionActor) error {
	switch unified.NormalizeActionActor(actor).Kind {
	case unified.ActionActorService, unified.ActionActorPolicy:
		return nil
	}
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

// requireActionCapability keeps the OSS default authorizer from turning an
// authenticated organization membership into infrastructure-control
// authority. A real RBAC authorizer remains authoritative for explicit action
// grants, while the default authorizer requires the canonical administrator
// boundary for browser and proxy sessions. Explicitly scoped API tokens keep
// using the scope and owner checks enforced by the surrounding route and the
// action lifecycle.
func requireActionCapability(cfg *config.Config, authorizer auth.Authorizer, capability string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, isDefaultAuthorizer := authorizer.(*auth.DefaultAuthorizer); isDefaultAuthorizer && getAPITokenRecordFromRequest(r) == nil {
			if cfg != nil && strings.TrimSpace(cfg.ProxyAuthSecret) != "" {
				if valid, _, isAdmin := CheckProxyAuth(cfg, r); valid && !isAdmin {
					writeJSONError(w, http.StatusForbidden, "action_capability_denied", "Administrator privileges or an explicit action grant are required")
					return
				}
			}
			if !ensureAdminSession(cfg, w, r) {
				return
			}
		}
		if err := authorizeActionCapability(r.Context(), authorizer, capability); err != nil {
			writeJSONError(w, http.StatusForbidden, "action_capability_denied", "You do not have permission to perform this action")
			return
		}
		handler(w, r)
	}
}

// requestHasActionCapability answers requireActionCapability's question without
// refusing the request. It is for routes that stay open to every signed-in user
// but hand one operation only to callers with the capability, such as Assistant
// chat, where only callers allowed to execute actions may have the model run
// commands on hosts. API tokens also need a scope that grants the capability
// (tokenHasActionCapabilityScope), because the chat route admits tokens that
// hold only ai:chat.
func requestHasActionCapability(cfg *config.Config, authorizer auth.Authorizer, r *http.Request, capability string) bool {
	if record := getAPITokenRecordFromRequest(r); record != nil {
		if !tokenHasActionCapabilityScope(record, capability) {
			return false
		}
	} else if _, isDefaultAuthorizer := authorizer.(*auth.DefaultAuthorizer); isDefaultAuthorizer {
		if cfg != nil && strings.TrimSpace(cfg.ProxyAuthSecret) != "" {
			if valid, _, isAdmin := CheckProxyAuth(cfg, r); valid && !isAdmin {
				return false
			}
		}
		if !requestCarriesInstanceAuthority(cfg, r) {
			return false
		}
	}
	return authorizeActionCapability(r.Context(), authorizer, capability) == nil
}

type assistantActionPlanAuthorityContextKey struct{}

// withAssistantActionAuthority binds what the Assistant may do on this
// caller's behalf beyond reading Pulse's own data, using the same capabilities
// as the Actions routes:
//   - host commands (tools that run a command on a host through an agent)
//     need action-execute;
//   - planning an action (pulse_control) needs action-plan, the capability
//     POST /api/actions/plan requires.
//
// Everyone who can chat keeps the Assistant for Pulse's own data.
func withAssistantActionAuthority(cfg *config.Config, authorizer auth.Authorizer, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hostCommands := requestHasActionCapability(cfg, authorizer, r, auth.ActionExecute)
		plan := assistantRequestHasActionPlanCapability(cfg, authorizer, r)
		ctx := chat.WithHostCommandAuthority(r.Context(), hostCommands)
		ctx = context.WithValue(ctx, assistantActionPlanAuthorityContextKey{}, plan)
		handler(w, r.WithContext(ctx))
	}
}

// assistantRequestHasActionPlanCapability is the role half of the Assistant's
// planning authority. Sessions and proxy users need what POST
// /api/actions/plan needs. API tokens keep the Assistant's own token scope
// rule (assistantExecuteAuthority: ai:execute, or relay-mobile pinned to
// approval) and must still pass the authorizer, so an RBAC role without the
// plan grant cannot plan through a token either.
func assistantRequestHasActionPlanCapability(cfg *config.Config, authorizer auth.Authorizer, r *http.Request) bool {
	if getAPITokenRecordFromRequest(r) != nil {
		return authorizeActionCapability(r.Context(), authorizer, auth.ActionPlan) == nil
	}
	return requestHasActionCapability(cfg, authorizer, r, auth.ActionPlan)
}

// assistantActionPlanAuthorityFromContext reports the plan capability bound
// by withAssistantActionAuthority. Absent means false.
func assistantActionPlanAuthorityFromContext(ctx context.Context) bool {
	allowed, _ := ctx.Value(assistantActionPlanAuthorityContextKey{}).(bool)
	return allowed
}

// requireAIApprovalReview reuses canonical action review authority for browser
// and proxy sessions. API tokens retain the existing per-route AI/mobile scopes:
// older paired clients are not bound to the newer action-lifecycle actor model.
// The registered routes must keep RequireAuth outside this admission.
func requireAIApprovalReview(cfg *config.Config, authorizer auth.Authorizer, handler http.HandlerFunc) http.HandlerFunc {
	sessionReview := requireActionCapability(cfg, authorizer, auth.ActionApprove, handler)
	return func(w http.ResponseWriter, r *http.Request) {
		if getAPITokenRecordFromRequest(r) != nil {
			handler(w, r)
			return
		}
		sessionReview(w, r)
	}
}
