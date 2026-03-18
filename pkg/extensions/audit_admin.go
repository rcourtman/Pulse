package extensions

import (
	"context"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
)

// AuditAdminEndpoints defines the enterprise audit admin endpoint surface.
type AuditAdminEndpoints interface {
	HandleListEvents(http.ResponseWriter, *http.Request)
	HandleVerifyEvent(http.ResponseWriter, *http.Request)
	HandleExportEvents(http.ResponseWriter, *http.Request)
	HandleSummary(http.ResponseWriter, *http.Request)
	HandleGetWebhooks(http.ResponseWriter, *http.Request)
	HandleUpdateWebhooks(http.ResponseWriter, *http.Request)
}

// WriteAuditErrorFunc writes a structured audit error response.
type WriteAuditErrorFunc func(http.ResponseWriter, int, string, string, map[string]string)

// AuditAdminRuntime exposes API/runtime capabilities needed by audit admin endpoints.
type AuditAdminRuntime struct {
	GetRequestOrgID    func(context.Context) string
	ResolveLogger      func(orgID string) audit.Logger
	IsPersistentLogger func(logger audit.Logger) bool
	ValidateWebhookURL func(rawURL string) error
	WriteError         WriteAuditErrorFunc
}

// BindAuditAdminEndpointsFunc allows enterprise modules to bind replacement
// audit admin endpoints while retaining access to default handlers.
type BindAuditAdminEndpointsFunc func(defaults AuditAdminEndpoints, runtime AuditAdminRuntime) AuditAdminEndpoints
