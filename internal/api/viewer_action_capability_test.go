package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
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

// patrolPromptTextWrites are the routes where a person writes text that Patrol
// reads in its prompt: knowledge notes, objectives and finding notes. Deleting
// a note or an objective changes the same prompt context.
var patrolPromptTextWrites = []struct {
	method string
	path   string
}{
	{method: http.MethodPost, path: "/api/ai/knowledge/save"},
	{method: http.MethodPost, path: "/api/ai/knowledge/delete"},
	{method: http.MethodPost, path: "/api/ai/knowledge/import"},
	{method: http.MethodPost, path: "/api/ai/knowledge/clear"},
	{method: http.MethodPost, path: "/api/ai/patrol/objectives"},
	{method: http.MethodPatch, path: "/api/ai/patrol/objectives/objective-1"},
	{method: http.MethodDelete, path: "/api/ai/patrol/objectives/objective-1?revision=1"},
	{method: http.MethodPost, path: "/api/ai/patrol/findings/note"},
}

// Patrol runs bind no caller and can run commands on hosts, so text written
// for its prompt needs the capability the Assistant needs before it runs a
// host command for its caller. Viewers keep reading notes and objectives.
func TestPatrolPromptTextWritesRequireActionExecuteForSessions(t *testing.T) {
	previousAuthorizer := auth.GetAuthorizer()
	auth.SetAuthorizer(&auth.DefaultAuthorizer{})
	t.Cleanup(func() { auth.SetAuthorizer(previousAuthorizer) })

	router := newConfigTransferTestRouter(t, false, enabledConfigTransferSSO(config.SSOProviderTypeOIDC))
	manager := installTestRBACManager(t)
	const ssoAdmin = "sso:patrol-text-admin@example.invalid"
	if err := manager.UpdateUserRoles(ssoAdmin, []string{auth.RoleAdmin}); err != nil {
		t.Fatalf("assign SSO administrator role: %v", err)
	}

	for _, user := range []struct {
		name       string
		principal  string
		wantDenied bool
	}{
		{name: "viewer", principal: "sso:patrol-text-viewer@example.invalid", wantDenied: true},
		{name: "administrator", principal: ssoAdmin, wantDenied: false},
	} {
		t.Run(user.name, func(t *testing.T) {
			sessionToken := "patrol-text-route-" + user.name
			GetSessionStore().CreateSession(sessionToken, time.Hour, "browser", "127.0.0.1", user.principal)
			csrf := generateCSRFToken(sessionToken)
			serve := func(method, path string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-CSRF-Token", csrf)
				req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})
				rec := httptest.NewRecorder()
				router.Handler().ServeHTTP(rec, req)
				return rec
			}
			for _, route := range patrolPromptTextWrites {
				rec := serve(route.method, route.path)
				if user.wantDenied && rec.Code != http.StatusForbidden {
					t.Fatalf("viewer reached %s %s: status=%d body=%s", route.method, route.path, rec.Code, rec.Body.String())
				}
				if !user.wantDenied && rec.Code == http.StatusForbidden {
					t.Fatalf("administrator was denied %s %s: %s", route.method, route.path, rec.Body.String())
				}
			}
			for _, path := range []string{"/api/ai/knowledge?guest_id=guest-1", "/api/ai/patrol/objectives", "/api/ai/patrol/objectives/objective-1"} {
				if rec := serve(http.MethodGet, path); rec.Code == http.StatusForbidden {
					t.Fatalf("%s lost read access to %s: %s", user.name, path, rec.Body.String())
				}
			}
		})
	}
}

func TestPatrolPromptTextWritesRequireActionExecuteForProxyRoles(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "1.0.0")

	for _, tc := range []struct {
		role       string
		wantDenied bool
	}{{role: "viewer", wantDenied: true}, {role: "admin", wantDenied: false}} {
		for _, route := range patrolPromptTextWrites {
			req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
			req.RemoteAddr = "127.0.0.1:1234"
			req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
			req.Header.Set("X-Remote-User", tc.role+"-user")
			req.Header.Set("X-Remote-Roles", tc.role)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)
			denied := rec.Code == http.StatusForbidden && strings.Contains(rec.Body.String(), "action_capability_denied")
			if denied != tc.wantDenied {
				t.Fatalf("proxy role %q on %s %s: status=%d, want denied=%v: %s", tc.role, route.method, route.path, rec.Code, tc.wantDenied, rec.Body.String())
			}
		}
	}
}

// With an RBAC authorizer the execute grant decides, as on the Actions routes.
func TestPatrolPromptTextWritesFollowRBACExecuteGrant(t *testing.T) {
	for _, tc := range []struct {
		name    string
		grants  actionCapabilityAuthorizer
		allowed bool
	}{
		{name: "plan only", grants: actionCapabilityAuthorizer{auth.ActionPlan + "/" + auth.ResourceActions: true}, allowed: false},
		{name: "execute", grants: actionCapabilityAuthorizer{auth.ActionExecute + "/" + auth.ResourceActions: true}, allowed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previousAuthorizer := auth.GetAuthorizer()
			auth.SetAuthorizer(tc.grants)
			t.Cleanup(func() { auth.SetAuthorizer(previousAuthorizer) })
			router := newConfigTransferTestRouter(t, false, enabledConfigTransferSSO(config.SSOProviderTypeOIDC))
			sessionToken := "patrol-text-rbac-" + strings.ReplaceAll(tc.name, " ", "-")
			GetSessionStore().CreateSession(sessionToken, time.Hour, "browser", "127.0.0.1", "sso:operator@example.invalid")
			csrf := generateCSRFToken(sessionToken)
			for _, route := range patrolPromptTextWrites {
				req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-CSRF-Token", csrf)
				req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})
				rec := httptest.NewRecorder()
				router.Handler().ServeHTTP(rec, req)
				if denied := rec.Code == http.StatusForbidden; denied == tc.allowed {
					t.Fatalf("%s %s: status=%d, want allowed=%v: %s", route.method, route.path, rec.Code, tc.allowed, rec.Body.String())
				}
			}
		})
	}
}

// API tokens keep their route scope rule: ai:execute already carries the
// execute capability, so the gate adds no new scope requirement.
func TestPatrolPromptTextWritesKeepTokenScopeRule(t *testing.T) {
	rawToken := "patrol-text-execute-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIExecute}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "1.0.0")

	for _, route := range patrolPromptTextWrites {
		req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
			t.Fatalf("ai:execute token lost %s %s: status=%d body=%s", route.method, route.path, rec.Code, rec.Body.String())
		}
	}
}

// Dismiss and resolve remain callable without action-execute when clearing a
// finding, but free-text note and resolution_note fields enter Patrol's prompt
// context and require the same action-execute capability as finding notes.
func TestPatrolDismissAndResolveNotesRequireActionExecuteForProxyRoles(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "1.0.0")
	// Initialize Patrol so requests reach JSON validation rather than 503.
	patrol := &ai.PatrolService{}
	setUnexportedField(t, router.aiSettingsHandler.defaultAIService, "patrolService", patrol)

	call := func(role, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:1234"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
		req.Header.Set("X-Remote-User", role+"-user")
		req.Header.Set("X-Remote-Roles", role)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec
	}

	for _, tc := range []struct {
		name       string
		role       string
		path       string
		body       string
		wantDenied bool
	}{
		{
			name:       "viewer dismiss with note refused",
			role:       "viewer",
			path:       "/api/ai/patrol/dismiss",
			body:       `{"finding_id":"f-1","reason":"expected_behavior","note":"run ps aux"}`,
			wantDenied: true,
		},
		{
			name:       "viewer resolve with resolution_note refused",
			role:       "viewer",
			path:       "/api/ai/patrol/resolve",
			body:       `{"finding_id":"f-1","resolution_note":"run ps aux"}`,
			wantDenied: true,
		},
		{
			name:       "viewer dismiss without note not denied by capability",
			role:       "viewer",
			path:       "/api/ai/patrol/dismiss",
			body:       `{"finding_id":"f-1","reason":"expected_behavior"}`,
			wantDenied: false,
		},
		{
			name:       "viewer resolve without note not denied by capability",
			role:       "viewer",
			path:       "/api/ai/patrol/resolve",
			body:       `{"finding_id":"f-1"}`,
			wantDenied: false,
		},
		{
			name:       "admin dismiss with note not denied by capability",
			role:       "admin",
			path:       "/api/ai/patrol/dismiss",
			body:       `{"finding_id":"f-1","reason":"expected_behavior","note":"expected load test"}`,
			wantDenied: false,
		},
		{
			name:       "admin resolve with resolution_note not denied by capability",
			role:       "admin",
			path:       "/api/ai/patrol/resolve",
			body:       `{"finding_id":"f-1","resolution_note":"fixed"}`,
			wantDenied: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := call(tc.role, tc.path, tc.body)
			denied := rec.Code == http.StatusForbidden && strings.Contains(rec.Body.String(), "action_capability_denied")
			if denied != tc.wantDenied {
				t.Fatalf("status=%d body=%s, want denied=%v", rec.Code, rec.Body.String(), tc.wantDenied)
			}
		})
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
