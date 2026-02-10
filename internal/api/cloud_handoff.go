package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/cloudauth"
	"github.com/rs/zerolog/log"
)

// HandleCloudHandoff returns an HTTP handler that completes the control-plane → tenant
// auth handoff. It reads a per-tenant HMAC key, verifies the handoff token, creates a
// session, and redirects to the dashboard.
//
// Self-guards: returns 404 if the handoff key file does not exist in dataPath,
// meaning this is not a cloud-managed tenant.
func HandleCloudHandoff(dataPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Self-guard: only respond if a handoff key exists.
		keyPath := filepath.Join(dataPath, cloudauth.HandoffKeyFile)
		handoffKey, err := os.ReadFile(keyPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		tokenStr := strings.TrimSpace(r.URL.Query().Get("token"))
		if tokenStr == "" {
			http.Redirect(w, r, "/login?error=handoff_invalid", http.StatusTemporaryRedirect)
			return
		}

		email, _, err := cloudauth.Verify(handoffKey, tokenStr)
		if err != nil {
			log.Warn().Err(err).Msg("Cloud handoff token verification failed")
			http.Redirect(w, r, "/login?error=handoff_invalid", http.StatusTemporaryRedirect)
			return
		}

		// Create session using existing machinery (same pattern as HandlePublicMagicLinkVerify).
		sessionToken := generateSessionToken()
		if sessionToken == "" {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		userAgent := r.Header.Get("User-Agent")
		clientIP := GetClientIP(r)
		sessionDuration := 24 * time.Hour
		GetSessionStore().CreateSession(sessionToken, sessionDuration, userAgent, clientIP, email)
		TrackUserSession(email, sessionToken)

		csrfToken := generateCSRFToken(sessionToken)
		isSecure, sameSitePolicy := getCookieSettings(r)
		cookieMaxAge := int(sessionDuration.Seconds())

		http.SetCookie(w, &http.Cookie{
			Name:     "pulse_session",
			Value:    sessionToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   isSecure,
			SameSite: sameSitePolicy,
			MaxAge:   cookieMaxAge,
		})
		http.SetCookie(w, &http.Cookie{
			Name:     "pulse_csrf",
			Value:    csrfToken,
			Path:     "/",
			Secure:   isSecure,
			SameSite: sameSitePolicy,
			MaxAge:   cookieMaxAge,
		})
		http.SetCookie(w, &http.Cookie{
			Name:     "pulse_org_id",
			Value:    "default",
			Path:     "/",
			Secure:   isSecure,
			SameSite: sameSitePolicy,
			MaxAge:   cookieMaxAge,
		})

		log.Info().
			Str("email", email).
			Msg("Cloud handoff completed, session created")

		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	}
}
