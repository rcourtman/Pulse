package handoff

import (
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auditlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpsec"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
)

const (
	handoffKeyRelPath = "secrets/handoff.key"
)

var handoffHTMLTemplate = template.Must(template.New("handoff").Parse(`<!DOCTYPE html>
<html><body>
<form method="POST" action="https://{{.TenantID}}.{{.BaseDomain}}/api/cloud/handoff/exchange">
  <input type="hidden" name="token" value="{{.Token}}" />
</form>
<script nonce="{{.Nonce}}">document.forms[0].submit()</script>
</body></html>
`))

type handoffHTMLData struct {
	TenantID    string
	BaseDomain  string
	Token       string
	Nonce       string
	GeneratedAt time.Time
}

// HandleHandoff mints a tenant handoff token and returns an auto-submit HTML page.
// Route (wiring happens elsewhere): POST /api/accounts/{account_id}/tenants/{tenant_id}/handoff
//
// Auth: control-plane session + account membership middleware. User identity is
// propagated from middleware via X-User-ID.
func HandleHandoff(reg *registry.TenantRegistry, tenantsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if reg == nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		accountID := strings.TrimSpace(r.PathValue("account_id"))
		tenantID := strings.TrimSpace(r.PathValue("tenant_id"))
		if accountID == "" || tenantID == "" {
			log.Warn().
				Str("audit_event", "cp_handoff").
				Str("outcome", "failure").
				Str("reason", "missing_account_id_or_tenant_id").
				Str("client_ip", auditlog.ClientIP(r)).
				Str("method", r.Method).
				Str("path", auditlog.RequestPath(r)).
				Msg("Tenant handoff denied")
			http.Error(w, "missing account_id or tenant_id", http.StatusBadRequest)
			return
		}

		a, err := reg.GetAccount(accountID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if a == nil {
			http.Error(w, "account not found", http.StatusNotFound)
			return
		}

		t, err := reg.GetTenantForAccount(accountID, tenantID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if t == nil {
			log.Info().
				Str("audit_event", "cp_handoff").
				Str("outcome", "failure").
				Str("reason", "tenant_not_found").
				Str("account_id", accountID).
				Str("tenant_id", tenantID).
				Str("client_ip", auditlog.ClientIP(r)).
				Str("method", r.Method).
				Str("path", auditlog.RequestPath(r)).
				Msg("Tenant handoff denied")
			http.Error(w, "tenant not found", http.StatusNotFound)
			return
		}

		// Temporary request-scoped identity until control plane session auth exists.
		userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
		if userID == "" {
			userID = strings.TrimSpace(r.Header.Get("X-User-Id"))
		}
		if userID == "" {
			log.Warn().
				Str("audit_event", "cp_handoff").
				Str("outcome", "failure").
				Str("reason", "missing_user_identity").
				Str("account_id", accountID).
				Str("tenant_id", tenantID).
				Str("client_ip", auditlog.ClientIP(r)).
				Str("method", r.Method).
				Str("path", auditlog.RequestPath(r)).
				Msg("Tenant handoff denied")
			http.Error(w, "missing user identity", http.StatusBadRequest)
			return
		}

		m, err := reg.GetMembership(accountID, userID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if m == nil {
			log.Info().
				Str("audit_event", "cp_handoff").
				Str("outcome", "failure").
				Str("reason", "forbidden").
				Str("account_id", accountID).
				Str("tenant_id", tenantID).
				Str("user_id", userID).
				Str("client_ip", auditlog.ClientIP(r)).
				Str("method", r.Method).
				Str("path", auditlog.RequestPath(r)).
				Msg("Tenant handoff denied")
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		u, err := reg.GetUser(userID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if u == nil || strings.TrimSpace(u.Email) == "" {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		keyPath := filepath.Join(filepath.Clean(tenantsDir), tenantID, filepath.FromSlash(handoffKeyRelPath))
		secret, err := os.ReadFile(keyPath)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		now := time.Now().UTC()
		token, err := MintHandoffToken(secret, HandoffClaims{
			TenantID:  tenantID,
			UserID:    userID,
			AccountID: accountID,
			Email:     u.Email,
			Role:      m.Role,
			IssuedAt:  now,
			ExpiresAt: now.Add(defaultTTL),
		})
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		log.Info().
			Str("audit_event", "cp_handoff").
			Str("outcome", "success").
			Str("account_id", accountID).
			Str("tenant_id", tenantID).
			Str("user_id", userID).
			Str("email", u.Email).
			Str("role", string(m.Role)).
			Str("client_ip", auditlog.ClientIP(r)).
			Str("method", r.Method).
			Str("path", auditlog.RequestPath(r)).
			Msg("Tenant handoff token minted")

		baseDomain := deriveBaseDomain(r)
		if baseDomain == "" {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Read the per-request nonce set by CPSecurityHeaders middleware.
		nonce := cpsec.NonceFromContext(r.Context())

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if err := handoffHTMLTemplate.Execute(w, handoffHTMLData{
			TenantID:    tenantID,
			BaseDomain:  baseDomain,
			Token:       token,
			Nonce:       nonce,
			GeneratedAt: now,
		}); err != nil {
			log.Error().Err(err).
				Str("tenant_id", tenantID).
				Str("account_id", accountID).
				Str("user_id", userID).
				Msg("cloudcp.handoff: render response")
		}
	}
}

func deriveBaseDomain(r *http.Request) string {
	if v := strings.TrimSpace(os.Getenv("CP_BASE_DOMAIN")); v != "" {
		return v
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return ""
	}
	// Strip port if present.
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	} else {
		// net.SplitHostPort errors if there's no port; that's fine.
		if strings.Count(host, ":") > 1 {
			// IPv6 without port isn't valid for our use case.
			return ""
		}
	}
	return host
}
