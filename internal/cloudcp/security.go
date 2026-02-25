package cloudcp

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpsec"
	"github.com/rs/zerolog/log"
)

// generateCPCSPNonce returns a 16-byte (128-bit) base64-encoded cryptographic nonce.
func generateCPCSPNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		log.Error().Err(err).Msg("CP CSP nonce generation failed — falling back to unsafe-inline")
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

// CPSecurityHeaders wraps an http.Handler to set security headers including a
// nonce-based Content Security Policy on all responses. The nonce is stored in
// the request context for use by downstream template renderers via
// cpsec.NonceFromContext.
func CPSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := generateCPCSPNonce()
		if nonce != "" {
			r = r.WithContext(cpsec.WithNonce(r.Context(), nonce))
		}

		// Deny all framing — CP pages should never be embedded.
		w.Header().Set("X-Frame-Options", "DENY")

		// Prevent MIME type sniffing.
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Disable legacy XSS auditor.
		w.Header().Set("X-XSS-Protection", "0")

		// Referrer policy — avoid leaking full URL to third parties (Stripe redirect).
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions policy — CP doesn't use any device APIs.
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=(), usb=()")

		// Build CSP.
		var styleSrc, scriptSrc string
		if nonce != "" {
			scriptSrc = "script-src 'self' 'nonce-" + nonce + "'"
			styleSrc = "style-src 'self' 'nonce-" + nonce + "'"
		} else {
			// Fallback if nonce generation failed.
			scriptSrc = "script-src 'self' 'unsafe-inline'"
			styleSrc = "style-src 'self' 'unsafe-inline'"
		}

		// form-action allows 'self' (signup forms) plus https: because the
		// handoff template's auto-submit form targets dynamic tenant subdomains
		// (https://<tid>.basedomain/...) that can't be enumerated at startup.
		csp := "default-src 'self'; " +
			scriptSrc + "; " +
			styleSrc + "; " +
			"img-src 'self' data:; " +
			"connect-src 'self'; " +
			"font-src 'self'; " +
			"form-action 'self' https:; " +
			"frame-ancestors 'none'"

		w.Header().Set("Content-Security-Policy", csp)

		next.ServeHTTP(w, r)
	})
}
