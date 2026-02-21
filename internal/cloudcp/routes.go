package cloudcp

import (
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/account"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/admin"
	cpauth "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/email"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/handoff"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/portal"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	cpstripe "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/stripe"
)

// Deps holds shared dependencies injected into HTTP handlers.
type Deps struct {
	Config      *CPConfig
	Registry    *registry.TenantRegistry
	Docker      *docker.Manager // nil if Docker is unavailable
	MagicLinks  *cpauth.Service // control plane magic link service
	Provisioner *cpstripe.Provisioner
	Version     string
	EmailSender email.Sender
}

// RegisterRoutes wires all HTTP handlers onto the given ServeMux.
func RegisterRoutes(mux *http.ServeMux, deps *Deps) {
	adminAuth := func(next http.Handler) http.Handler {
		return admin.AdminKeyMiddleware(deps.Config.AdminKey, next)
	}
	sessionAuth := func(next http.Handler) http.Handler {
		if deps.MagicLinks == nil {
			return adminAuth(next)
		}
		return requireSessionAuth(deps.MagicLinks, next)
	}
	accountSessionAuth := func(extract accountIDExtractor, next http.Handler) http.Handler {
		if deps.MagicLinks == nil {
			return adminAuth(next)
		}
		return sessionAuth(requireAccountMembership(deps.Registry, extract, next))
	}

	// Health / readiness are unauthenticated liveness/readiness probes.
	mux.HandleFunc("/healthz", admin.HandleHealthz)
	mux.HandleFunc("/readyz", admin.HandleReadyz(deps.Registry))

	// Status and metrics are private by default.
	statusHandler := http.HandlerFunc(admin.HandleStatus(deps.Registry, deps.Version))
	if deps.Config.PublicStatus {
		mux.Handle("/status", statusHandler)
	} else {
		mux.Handle("/status", adminAuth(statusHandler))
	}

	metricsHandler := promhttp.Handler()
	if deps.Config.PublicMetrics {
		mux.Handle("/metrics", metricsHandler)
	} else {
		mux.Handle("/metrics", adminAuth(metricsHandler))
	}

	// Stripe webhook (signature-authenticated)
	provisioner := deps.Provisioner
	if provisioner == nil {
		provisioner = cpstripe.NewProvisioner(deps.Registry, deps.Config.TenantsDir(), deps.Docker, deps.MagicLinks, deps.Config.BaseURL, deps.EmailSender, deps.Config.EmailFrom)
	}
	webhookHandler := cpstripe.NewWebhookHandler(deps.Config.StripeWebhookSecret, provisioner)
	webhookLimiter := NewCPRateLimiter(120, time.Minute)
	mux.Handle("/api/stripe/webhook", webhookLimiter.Middleware(webhookHandler))

	// Magic link verification (public, token-authenticated)
	baseDomain := baseDomainFromURL(deps.Config.BaseURL)
	mux.HandleFunc("/auth/magic-link/verify", cpauth.HandleMagicLinkVerify(deps.MagicLinks, deps.Registry, deps.Config.TenantsDir(), baseDomain))

	// Admin API (key-authenticated)
	tenantsHandler := admin.HandleListTenants(deps.Registry)
	mux.Handle("/admin/tenants", adminAuth(tenantsHandler))
	mux.Handle("/admin/magic-link", adminAuth(cpauth.HandleAdminGenerateMagicLink(deps.MagicLinks, deps.Config.BaseURL, deps.EmailSender, deps.Config.EmailFrom)))

	// Account membership (session + account-membership authenticated)
	listMembers := account.HandleListMembers(deps.Registry)
	inviteMember := account.HandleInviteMember(deps.Registry)
	updateRole := account.HandleUpdateMemberRole(deps.Registry)
	removeMember := account.HandleRemoveMember(deps.Registry)

	membersCollection := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listMembers(w, r)
		case http.MethodPost:
			inviteMember(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	member := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			updateRole(w, r)
		case http.MethodDelete:
			removeMember(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	accountIDFromPath := func(r *http.Request) string {
		return r.PathValue("account_id")
	}
	accountIDFromPortalRequest := func(r *http.Request) string {
		if v := strings.TrimSpace(r.URL.Query().Get("account_id")); v != "" {
			return v
		}
		if v := strings.TrimSpace(r.Header.Get("X-Account-ID")); v != "" {
			return v
		}
		if v := strings.TrimSpace(r.Header.Get("X-Account-Id")); v != "" {
			return v
		}
		return ""
	}

	mux.Handle("/api/accounts/{account_id}/members", accountSessionAuth(accountIDFromPath, membersCollection))
	mux.Handle("/api/accounts/{account_id}/members/{user_id}", accountSessionAuth(accountIDFromPath, member))

	// Workspace management (session + account-membership authenticated)
	listTenants := account.HandleListTenants(deps.Registry)
	createTenant := account.HandleCreateTenant(deps.Registry, provisioner)
	updateTenant := account.HandleUpdateTenant(deps.Registry)
	deleteTenant := account.HandleDeleteTenant(deps.Registry, provisioner)

	tenantsCollection := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listTenants(w, r)
		case http.MethodPost:
			createTenant(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	tenant := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			updateTenant(w, r)
		case http.MethodDelete:
			deleteTenant(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.Handle("/api/accounts/{account_id}/tenants", accountSessionAuth(accountIDFromPath, tenantsCollection))
	mux.Handle("/api/accounts/{account_id}/tenants/{tenant_id}", accountSessionAuth(accountIDFromPath, tenant))

	// Tenant switching handoff (session + account-membership authenticated)
	handoffHandler := handoff.HandleHandoff(deps.Registry, deps.Config.TenantsDir())
	mux.Handle("/api/accounts/{account_id}/tenants/{tenant_id}/handoff", accountSessionAuth(accountIDFromPath, handoffHandler))

	// MSP portal API (session + account-membership authenticated)
	mux.Handle("/api/portal/dashboard", accountSessionAuth(accountIDFromPortalRequest, portal.HandlePortalDashboard(deps.Registry)))
	mux.Handle("/api/portal/workspaces/{tenant_id}", accountSessionAuth(accountIDFromPortalRequest, portal.HandlePortalWorkspaceDetail(deps.Registry)))
}
