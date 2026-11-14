package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/stretchr/testify/mock"
)

// Routes that run commands on hosts through an agent, or start work that can
// plan an action, need the same action capabilities as the Actions routes.
// Signed-in viewers keep their read surfaces but cannot reach these.
func TestHostWorkRoutesRequireActionCapabilityForSessions(t *testing.T) {
	previousAuthorizer := auth.GetAuthorizer()
	auth.SetAuthorizer(&auth.DefaultAuthorizer{})
	t.Cleanup(func() { auth.SetAuthorizer(previousAuthorizer) })

	router := newConfigTransferTestRouter(t, false, enabledConfigTransferSSO(config.SSOProviderTypeOIDC))
	manager := installTestRBACManager(t)
	const ssoAdmin = "sso:host-work-admin@example.invalid"
	if err := manager.UpdateUserRoles(ssoAdmin, []string{auth.RoleAdmin}); err != nil {
		t.Fatalf("assign SSO administrator role: %v", err)
	}

	routes := []struct {
		name string
		path string
		body string
	}{
		{name: "docker update scan", path: "/api/infra-updates/check", body: `{"agentId":"missing-agent"}`},
		{name: "finding reinvestigation", path: "/api/ai/findings/missing-finding/reinvestigate", body: `{}`},
	}

	for _, user := range []struct {
		name       string
		principal  string
		wantDenied bool
	}{
		{name: "viewer", principal: "sso:host-work-viewer@example.invalid", wantDenied: true},
		{name: "administrator", principal: ssoAdmin, wantDenied: false},
	} {
		t.Run(user.name, func(t *testing.T) {
			sessionToken := "host-work-route-" + user.name
			GetSessionStore().CreateSession(sessionToken, time.Hour, "browser", "127.0.0.1", user.principal)
			csrf := generateCSRFToken(sessionToken)
			for _, route := range routes {
				t.Run(route.name, func(t *testing.T) {
					req := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(route.body))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("X-CSRF-Token", csrf)
					req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})
					rec := httptest.NewRecorder()
					router.Handler().ServeHTTP(rec, req)
					if user.wantDenied && rec.Code != http.StatusForbidden {
						t.Fatalf("viewer reached %s: status=%d body=%s", route.path, rec.Code, rec.Body.String())
					}
					if !user.wantDenied && rec.Code == http.StatusForbidden {
						t.Fatalf("administrator was denied %s: %s", route.path, rec.Body.String())
					}
				})
			}
		})
	}
}

// With an RBAC authorizer each route needs its own grants: the update scan
// needs execute, and reinvestigation needs both plan and execute.
func TestHostWorkRoutesFollowRBACActionGrants(t *testing.T) {
	plan := auth.ActionPlan + "/" + auth.ResourceActions
	execute := auth.ActionExecute + "/" + auth.ResourceActions
	for _, tc := range []struct {
		name              string
		grants            actionCapabilityAuthorizer
		wantCheck         bool
		wantReinvestigate bool
	}{
		{name: "plan only", grants: actionCapabilityAuthorizer{plan: true}, wantCheck: false, wantReinvestigate: false},
		{name: "execute only", grants: actionCapabilityAuthorizer{execute: true}, wantCheck: true, wantReinvestigate: false},
		{name: "plan and execute", grants: actionCapabilityAuthorizer{plan: true, execute: true}, wantCheck: true, wantReinvestigate: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previousAuthorizer := auth.GetAuthorizer()
			auth.SetAuthorizer(tc.grants)
			t.Cleanup(func() { auth.SetAuthorizer(previousAuthorizer) })
			router := newConfigTransferTestRouter(t, false, enabledConfigTransferSSO(config.SSOProviderTypeOIDC))
			sessionToken := "host-work-rbac-" + strings.ReplaceAll(tc.name, " ", "-")
			GetSessionStore().CreateSession(sessionToken, time.Hour, "browser", "127.0.0.1", "sso:operator@example.invalid")
			csrf := generateCSRFToken(sessionToken)
			for _, route := range []struct {
				path    string
				body    string
				allowed bool
			}{
				{path: "/api/infra-updates/check", body: `{"agentId":"missing-agent"}`, allowed: tc.wantCheck},
				{path: "/api/ai/findings/missing-finding/reinvestigate", body: `{}`, allowed: tc.wantReinvestigate},
			} {
				req := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(route.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-CSRF-Token", csrf)
				req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})
				rec := httptest.NewRecorder()
				router.Handler().ServeHTTP(rec, req)
				if denied := rec.Code == http.StatusForbidden; denied == route.allowed {
					t.Fatalf("%s: status=%d, want allowed=%v: %s", route.path, rec.Code, route.allowed, rec.Body.String())
				}
			}
		})
	}
}

// API tokens keep their route scope rules: the capability gate adds no new
// scope requirement for a token that already holds the route's scope.
func TestInfraUpdateCheckKeepsTokenScopeRule(t *testing.T) {
	rawToken := "infra-check-write-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/infra-updates/check", strings.NewReader(`{"agentId":"missing-agent"}`))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
		t.Fatalf("monitoring:write token lost the infra update check: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

// actionCapabilityAuthorizer grants exactly the listed (action, resource)
// pairs, like an RBAC role with some action grants and not others.
type actionCapabilityAuthorizer map[string]bool

func (a actionCapabilityAuthorizer) Authorize(_ context.Context, action, resource string) (bool, error) {
	return a[action+"/"+resource], nil
}

// Planning through the Assistant (pulse_control) needs the capability POST
// /api/actions/plan needs. These cases pin who gets it at the chat boundary.
func TestAssistantChatBindsActionPlanAuthorityFromCallerRole(t *testing.T) {
	InitSessionStore(t.TempDir())
	cfg := &config.Config{AuthUser: "admin"}
	executeOnly := actionCapabilityAuthorizer{auth.ActionExecute + "/" + auth.ResourceActions: true}
	callers := []assistantHostCommandCaller{
		{name: "viewer session", authorizer: &auth.DefaultAuthorizer{}, prepare: assistantSessionCaller("viewer"), want: false},
		{name: "administrator session", authorizer: &auth.DefaultAuthorizer{}, prepare: assistantSessionCaller("admin"), want: true},
		{name: "explicit action grant", authorizer: allowActionAuthorityAuthorizer{}, prepare: assistantSessionCaller("operator"), want: true},
		{name: "RBAC grants execute but not plan", authorizer: executeOnly, prepare: assistantSessionCaller("operator"), want: false},
		{name: "RBAC denies everything", authorizer: fixedActionAuthorityAuthorizer(false), prepare: assistantSessionCaller("admin"), want: false},
		{name: "ai:chat token", authorizer: &auth.DefaultAuthorizer{}, prepare: assistantTokenCaller(config.ScopeAIChat), want: false},
		{name: "ai:chat and ai:execute token", authorizer: &auth.DefaultAuthorizer{}, prepare: assistantTokenCaller(config.ScopeAIChat, config.ScopeAIExecute), want: true},
		{name: "relay-mobile token", authorizer: &auth.DefaultAuthorizer{}, prepare: assistantTokenCaller(config.ScopeRelayMobileAccess), want: true},
		{name: "ai:execute token denied by RBAC", authorizer: fixedActionAuthorityAuthorizer(false), prepare: assistantTokenCaller(config.ScopeAIChat, config.ScopeAIExecute), want: false},
	}
	for _, caller := range callers {
		t.Run(caller.name, func(t *testing.T) {
			h := newTestAIHandler(cfg, nil, nil)
			svc := new(MockAIService)
			h.defaultService = svc
			svc.On("IsRunning").Return(true)
			var got *chat.ExecuteRequest
			svc.On("ExecuteStream", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
				req := args.Get(1).(chat.ExecuteRequest)
				got = &req
			})

			req := caller.prepare(t, httptest.NewRequest(http.MethodPost, "/api/ai/chat", strings.NewReader(`{"prompt":"restart vm 101"}`)))
			rec := httptest.NewRecorder()
			withAssistantActionAuthority(cfg, caller.authorizer, h.HandleChat)(rec, req)

			if got == nil {
				t.Fatalf("chat did not reach the service: status=%d body=%q", rec.Code, rec.Body.String())
			}
			if got.HasExecuteAuthority != caller.want {
				t.Fatalf("HasExecuteAuthority = %v, want %v", got.HasExecuteAuthority, caller.want)
			}
		})
	}
}

func TestAssistantChatBindsActionPlanAuthorityForProxyRoles(t *testing.T) {
	cfg := &config.Config{
		AuthUser:            "admin",
		ProxyAuthSecret:     "proxy-secret",
		ProxyAuthUserHeader: "X-Remote-User",
		ProxyAuthRoleHeader: "X-Remote-Roles",
		ProxyAuthAdminRole:  "admin",
	}
	for _, tc := range []struct {
		role string
		want bool
	}{{role: "viewer", want: false}, {role: "admin", want: true}} {
		req := httptest.NewRequest(http.MethodPost, "/api/ai/chat", nil)
		req.Header.Set("X-Proxy-Secret", "proxy-secret")
		req.Header.Set("X-Remote-User", "someone")
		req.Header.Set("X-Remote-Roles", tc.role)
		req = req.WithContext(auth.WithUser(req.Context(), "someone"))
		if got := assistantRequestHasActionPlanCapability(cfg, &auth.DefaultAuthorizer{}, req); got != tc.want {
			t.Fatalf("proxy role %q: plan capability = %v, want %v", tc.role, got, tc.want)
		}
	}
}

func TestAssistantChatWithoutBindingHasNoPlanAuthority(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	svc := new(MockAIService)
	h.defaultService = svc
	svc.On("IsRunning").Return(true)
	var got *chat.ExecuteRequest
	svc.On("ExecuteStream", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		req := args.Get(1).(chat.ExecuteRequest)
		got = &req
	})
	h.HandleChat(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/ai/chat", strings.NewReader(`{"prompt":"hi"}`)))
	if got == nil || got.HasExecuteAuthority {
		t.Fatalf("unbound chat request must fail closed, got %+v", got)
	}
}
