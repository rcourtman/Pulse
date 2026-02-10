package cloudcp

import (
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/admin"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	cpstripe "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/stripe"
)

// Deps holds shared dependencies injected into HTTP handlers.
type Deps struct {
	Config   *CPConfig
	Registry *registry.TenantRegistry
	Docker   *docker.Manager // nil if Docker is unavailable
	Version  string
}

// RegisterRoutes wires all HTTP handlers onto the given ServeMux.
func RegisterRoutes(mux *http.ServeMux, deps *Deps) {
	// Health / readiness / status (unauthenticated)
	mux.HandleFunc("/healthz", admin.HandleHealthz)
	mux.HandleFunc("/readyz", admin.HandleReadyz(deps.Registry))
	mux.HandleFunc("/status", admin.HandleStatus(deps.Registry, deps.Version))

	// Stripe webhook (signature-authenticated)
	provisioner := cpstripe.NewProvisioner(deps.Registry, deps.Config.TenantsDir())
	webhookHandler := cpstripe.NewWebhookHandler(deps.Config.StripeWebhookSecret, provisioner)
	mux.Handle("/api/stripe/webhook", webhookHandler)

	// Admin API (key-authenticated)
	tenantsHandler := admin.HandleListTenants(deps.Registry)
	mux.Handle("/admin/tenants", admin.AdminKeyMiddleware(deps.Config.AdminKey, tenantsHandler))
}
