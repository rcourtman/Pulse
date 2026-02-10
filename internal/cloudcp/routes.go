package cloudcp

import (
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/account"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/admin"
	cpauth "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	cpstripe "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/stripe"
)

// Deps holds shared dependencies injected into HTTP handlers.
type Deps struct {
	Config     *CPConfig
	Registry   *registry.TenantRegistry
	Docker     *docker.Manager // nil if Docker is unavailable
	MagicLinks *cpauth.Service // control plane magic link service
	Version    string
}

// RegisterRoutes wires all HTTP handlers onto the given ServeMux.
func RegisterRoutes(mux *http.ServeMux, deps *Deps) {
	// Health / readiness / status (unauthenticated)
	mux.HandleFunc("/healthz", admin.HandleHealthz)
	mux.HandleFunc("/readyz", admin.HandleReadyz(deps.Registry))
	mux.HandleFunc("/status", admin.HandleStatus(deps.Registry, deps.Version))

	// Stripe webhook (signature-authenticated)
	provisioner := cpstripe.NewProvisioner(deps.Registry, deps.Config.TenantsDir(), deps.Docker, deps.MagicLinks, deps.Config.BaseURL)
	webhookHandler := cpstripe.NewWebhookHandler(deps.Config.StripeWebhookSecret, provisioner)
	mux.Handle("/api/stripe/webhook", webhookHandler)

	// Magic link verification (public, token-authenticated)
	baseDomain := baseDomainFromURL(deps.Config.BaseURL)
	mux.HandleFunc("/auth/magic-link/verify", cpauth.HandleMagicLinkVerify(deps.MagicLinks, deps.Registry, deps.Config.TenantsDir(), baseDomain))

	// Admin API (key-authenticated)
	tenantsHandler := admin.HandleListTenants(deps.Registry)
	mux.Handle("/admin/tenants", admin.AdminKeyMiddleware(deps.Config.AdminKey, tenantsHandler))

	// Account membership (admin-key authenticated for now; session auth in M-4)
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

	mux.Handle("/api/accounts/{account_id}/members", admin.AdminKeyMiddleware(deps.Config.AdminKey, membersCollection))
	mux.Handle("/api/accounts/{account_id}/members/{user_id}", admin.AdminKeyMiddleware(deps.Config.AdminKey, member))

	// Workspace management (admin-key authenticated for now; session auth in M-4)
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

	mux.Handle("/api/accounts/{account_id}/tenants", admin.AdminKeyMiddleware(deps.Config.AdminKey, tenantsCollection))
	mux.Handle("/api/accounts/{account_id}/tenants/{tenant_id}", admin.AdminKeyMiddleware(deps.Config.AdminKey, tenant))
}
