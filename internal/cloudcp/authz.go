package cloudcp

import (
	"encoding/json"
	"net/http"
	"strings"

	cpauth "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

type accountIDExtractor func(*http.Request) string

func requireSessionAuth(sessionSvc *cpauth.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sessionSvc == nil {
			writeAuthzError(w, http.StatusServiceUnavailable, "session_service_unavailable")
			return
		}

		token := sessionTokenFromRequest(r)
		if token == "" {
			writeAuthzError(w, http.StatusUnauthorized, "missing_session")
			return
		}

		claims, err := sessionSvc.ValidateSessionToken(token)
		if err != nil {
			writeAuthzError(w, http.StatusUnauthorized, "invalid_session")
			return
		}

		req := r.Clone(r.Context())
		req.Header.Set("X-User-ID", claims.UserID)
		req.Header.Set("X-User-Email", claims.Email)
		next.ServeHTTP(w, req)
	})
}

func requireAccountMembership(reg *registry.TenantRegistry, extract accountIDExtractor, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reg == nil {
			writeAuthzError(w, http.StatusInternalServerError, "registry_unavailable")
			return
		}

		accountID := ""
		if extract != nil {
			accountID = strings.TrimSpace(extract(r))
		}
		if accountID == "" {
			writeAuthzError(w, http.StatusBadRequest, "missing_account_id")
			return
		}

		userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
		if userID == "" {
			writeAuthzError(w, http.StatusUnauthorized, "missing_user_identity")
			return
		}

		m, err := reg.GetMembership(accountID, userID)
		if err != nil {
			writeAuthzError(w, http.StatusInternalServerError, "membership_lookup_failed")
			return
		}
		if m == nil {
			writeAuthzError(w, http.StatusForbidden, "forbidden")
			return
		}

		req := r.Clone(r.Context())
		req.Header.Set("X-Account-ID", accountID)
		req.Header.Set("X-User-Role", string(m.Role))
		next.ServeHTTP(w, req)
	})
}

func sessionTokenFromRequest(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	cookie, err := r.Cookie(cpauth.SessionCookieName)
	if err == nil {
		return strings.TrimSpace(cookie.Value)
	}
	return ""
}

func writeAuthzError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": code,
	})
}
