package cloudcp

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"unicode"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpsec"
	cpemail "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/email"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
	stripe "github.com/stripe/stripe-go/v82"
	stripesession "github.com/stripe/stripe-go/v82/checkout/session"
)

const publicCloudSignupRequestBodyLimit = 64 * 1024

var publicCloudSignupPageTemplate = template.Must(template.New("public-cloud-signup-page").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Start Pulse Cloud</title>
  <style nonce="{{.Nonce}}">
    :root { color-scheme: light; }
    body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: linear-gradient(140deg, #f8fafc, #e2e8f0); color: #0f172a; }
    .wrap { max-width: 760px; margin: 36px auto; padding: 0 16px; }
    .card { background: #fff; border-radius: 12px; border: 1px solid #e2e8f0; box-shadow: 0 8px 30px rgba(15,23,42,.08); padding: 24px; }
    h1 { margin: 0 0 8px; font-size: 30px; }
    p { margin: 0 0 16px; line-height: 1.5; color: #334155; }
    .error { background: #fef2f2; color: #991b1b; border: 1px solid #fecaca; border-radius: 8px; padding: 10px 12px; margin-bottom: 12px; font-size: 14px; }
    .note { background: #eff6ff; color: #1e3a8a; border: 1px solid #bfdbfe; border-radius: 8px; padding: 10px 12px; margin-bottom: 12px; font-size: 14px; }
    label { display: block; margin: 12px 0 6px; font-size: 14px; font-weight: 600; color: #0f172a; }
    input { width: 100%; box-sizing: border-box; border: 1px solid #cbd5e1; border-radius: 8px; padding: 10px 12px; font-size: 15px; }
    .cta { margin-top: 16px; border: 0; border-radius: 10px; background: #1d4ed8; color: #fff; font-size: 16px; font-weight: 600; padding: 12px 16px; width: 100%; cursor: pointer; }
    .cta:hover { background: #1e40af; }
    .fine { font-size: 12px; color: #64748b; margin-top: 12px; }
    ol { margin: 0; padding-left: 20px; color: #334155; }
    li { margin-bottom: 8px; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <h1>Start Pulse Cloud</h1>
      <p>Create your hosted Pulse workspace. Checkout is secure and provisioning starts automatically after payment confirmation.</p>
      {{if .ErrorMessage}}<div class="error">{{.ErrorMessage}}</div>{{end}}
      {{if .Cancelled}}<div class="note">Checkout was cancelled. You can start again below.</div>{{end}}

      <form method="POST" action="/signup">
        <label for="email">Work Email</label>
        <input id="email" name="email" type="email" value="{{.Email}}" autocomplete="email" required>

        <label for="org_name">Organization Name</label>
        <input id="org_name" name="org_name" type="text" value="{{.OrgName}}" autocomplete="organization" required>

        <button class="cta" type="submit">Continue To Secure Checkout</button>
      </form>

      <p class="fine">After checkout, you will receive a magic-link email to access your dedicated tenant.</p>
      <ol>
        <li>Your Stripe checkout completes securely.</li>
        <li>Pulse Cloud provisions your dedicated tenant container.</li>
        <li>You receive a sign-in magic link by email.</li>
      </ol>
    </div>
  </div>
</body>
</html>
`))

var publicCloudSignupCompleteTemplate = template.Must(template.New("public-cloud-signup-complete").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Pulse Cloud Signup Complete</title>
  <style nonce="{{.Nonce}}">
    :root { color-scheme: light; }
    body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #f8fafc; color: #0f172a; }
    .wrap { max-width: 680px; margin: 48px auto; padding: 0 16px; }
    .card { background: #fff; border-radius: 12px; border: 1px solid #e2e8f0; box-shadow: 0 8px 30px rgba(15,23,42,.08); padding: 24px; }
    h1 { margin: 0 0 8px; font-size: 28px; }
    p { margin: 0 0 14px; line-height: 1.5; color: #334155; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <h1>Signup Received</h1>
      <p>Your checkout completed. Pulse Cloud is provisioning your workspace.</p>
      <p>Watch your inbox for a magic-link sign-in email. If it does not arrive shortly, return to signup and request a new link.</p>
    </div>
  </div>
</body>
</html>
`))

type PublicCloudSignupHandlers struct {
	cfg        *CPConfig
	registry   *registry.TenantRegistry
	magicLinks interface {
		GenerateToken(email, tenantID string) (string, error)
	}
	emailSender           cpemail.Sender
	createCheckoutSession func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error)
}

type publicCloudSignupPageData struct {
	Email        string
	OrgName      string
	ErrorMessage string
	Cancelled    bool
	Nonce        string
}

// publicCloudSignupCompleteData carries nonce for the signup-complete page.
type publicCloudSignupCompleteData struct {
	Nonce string
}

type publicCloudSignupRequest struct {
	Email   string `json:"email"`
	OrgName string `json:"org_name"`
}

type publicMagicLinkRequest struct {
	Email string `json:"email"`
}

func NewPublicCloudSignupHandlers(cfg *CPConfig, reg *registry.TenantRegistry, magicLinks interface {
	GenerateToken(email, tenantID string) (string, error)
}, emailSender cpemail.Sender) *PublicCloudSignupHandlers {
	return &PublicCloudSignupHandlers{
		cfg:                   cfg,
		registry:              reg,
		magicLinks:            magicLinks,
		emailSender:           emailSender,
		createCheckoutSession: stripesession.New,
	}
}

func (h *PublicCloudSignupHandlers) HandleSignupPage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		data := publicCloudSignupPageData{
			Email:     strings.TrimSpace(r.URL.Query().Get("email")),
			OrgName:   strings.TrimSpace(r.URL.Query().Get("org_name")),
			Cancelled: strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("cancelled")), "1"),
		}
		h.renderSignupPage(w, r, http.StatusOK, data)
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form body", http.StatusBadRequest)
			return
		}
		email := strings.TrimSpace(r.FormValue("email"))
		orgName := strings.TrimSpace(r.FormValue("org_name"))
		if !isValidCloudSignupEmail(email) {
			h.renderSignupPage(w, r, http.StatusBadRequest, publicCloudSignupPageData{
				Email:        email,
				OrgName:      orgName,
				ErrorMessage: "A valid email address is required.",
			})
			return
		}
		if !isValidCloudSignupOrgName(orgName) {
			h.renderSignupPage(w, r, http.StatusBadRequest, publicCloudSignupPageData{
				Email:        email,
				OrgName:      orgName,
				ErrorMessage: "Organization name must be 3-64 characters and cannot contain slashes.",
			})
			return
		}

		checkoutURL, err := h.createCheckout(email, orgName)
		if err != nil {
			log.Warn().Err(err).Str("email", email).Msg("public cloud signup checkout creation failed")
			h.renderSignupPage(w, r, http.StatusBadGateway, publicCloudSignupPageData{
				Email:        email,
				OrgName:      orgName,
				ErrorMessage: "Unable to create checkout session. Please try again.",
			})
			return
		}
		http.Redirect(w, r, checkoutURL, http.StatusSeeOther)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *PublicCloudSignupHandlers) HandleSignupComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := publicCloudSignupCompleteTemplate.Execute(w, publicCloudSignupCompleteData{
		Nonce: cpsec.NonceFromContext(r.Context()),
	}); err != nil {
		log.Error().Err(err).Msg("public cloud signup complete page render failed")
	}
}

func (h *PublicCloudSignupHandlers) HandlePublicSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writePublicSignupError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, publicCloudSignupRequestBodyLimit)

	var req publicCloudSignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writePublicSignupError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.OrgName = strings.TrimSpace(req.OrgName)
	if !isValidCloudSignupEmail(req.Email) {
		writePublicSignupError(w, http.StatusBadRequest, "invalid_email", "Invalid email format")
		return
	}
	if !isValidCloudSignupOrgName(req.OrgName) {
		writePublicSignupError(w, http.StatusBadRequest, "invalid_org_name", "Invalid organization name")
		return
	}

	checkoutURL, err := h.createCheckout(req.Email, req.OrgName)
	if err != nil {
		log.Warn().Err(err).Str("email", req.Email).Msg("public cloud signup API checkout creation failed")
		writePublicSignupError(w, http.StatusBadGateway, "checkout_failed", "Unable to create checkout session")
		return
	}

	writePublicSignupJSON(w, http.StatusCreated, map[string]any{
		"checkout_url": checkoutURL,
		"message":      "Checkout session created. Continue in Stripe to provision your Pulse Cloud tenant.",
	})
}

func (h *PublicCloudSignupHandlers) HandlePublicMagicLinkRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writePublicSignupError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var req publicMagicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writePublicSignupError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	email := strings.TrimSpace(req.Email)
	if !isValidCloudSignupEmail(email) {
		writePublicSignupError(w, http.StatusBadRequest, "invalid_email", "Invalid email format")
		return
	}

	const msg = "If that email is registered, you'll receive a magic link shortly."
	if h.registry == nil || h.magicLinks == nil {
		writePublicSignupJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": msg,
		})
		return
	}

	tenantID, ok, err := h.findTenantForEmail(email)
	if err != nil {
		log.Warn().Err(err).Str("email", email).Msg("public magic link request: tenant lookup failed")
		writePublicSignupJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": msg,
		})
		return
	}
	if !ok {
		writePublicSignupJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": msg,
		})
		return
	}

	token, err := h.magicLinks.GenerateToken(email, tenantID)
	if err != nil {
		log.Warn().Err(err).Str("email", email).Str("tenant_id", tenantID).Msg("public magic link request: token generation failed")
		writePublicSignupJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": msg,
		})
		return
	}

	verifyURL := ""
	if h.cfg != nil {
		verifyURL = buildVerifyURLForEmail(h.cfg.BaseURL, token)
	}
	if verifyURL != "" {
		if err := h.sendMagicLinkEmail(email, verifyURL); err != nil {
			log.Warn().Err(err).Str("email", email).Str("tenant_id", tenantID).Msg("public magic link request: send failed")
		}
	}

	writePublicSignupJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": msg,
	})
}

func (h *PublicCloudSignupHandlers) createCheckout(email, orgName string) (string, error) {
	if h.cfg == nil {
		return "", fmt.Errorf("control plane config is missing")
	}
	if strings.TrimSpace(h.cfg.StripeAPIKey) == "" {
		return "", fmt.Errorf("stripe api key not configured")
	}
	priceID := strings.TrimSpace(h.cfg.TrialSignupPriceID)
	if priceID == "" {
		return "", fmt.Errorf("trial signup price id not configured")
	}

	stripe.Key = strings.TrimSpace(h.cfg.StripeAPIKey)
	successURL := buildCPURL(h.cfg.BaseURL, "/signup/complete", nil)
	cancelURL := buildCPURL(h.cfg.BaseURL, "/signup", url.Values{
		"cancelled": {"1"},
		"email":     {email},
		"org_name":  {orgName},
	})
	params := &stripe.CheckoutSessionParams{
		Mode:                    stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:              stripe.String(successURL),
		CancelURL:               stripe.String(cancelURL),
		CustomerEmail:           stripe.String(email),
		PaymentMethodCollection: stripe.String(string(stripe.CheckoutSessionPaymentMethodCollectionAlways)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			TrialPeriodDays: stripe.Int64(trialSignupTrialDays),
		},
		Metadata: map[string]string{
			"account_kind":         string(registry.AccountKindIndividual),
			"account_display_name": orgName,
			"display_name":         orgName,
			"signup_source":        "public_cloud_signup",
		},
	}
	session, err := h.createCheckoutSession(params)
	if err != nil {
		return "", err
	}
	if session == nil || strings.TrimSpace(session.URL) == "" {
		return "", fmt.Errorf("stripe returned empty checkout URL")
	}
	return strings.TrimSpace(session.URL), nil
}

func (h *PublicCloudSignupHandlers) renderSignupPage(w http.ResponseWriter, r *http.Request, status int, data publicCloudSignupPageData) {
	data.Nonce = cpsec.NonceFromContext(r.Context())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := publicCloudSignupPageTemplate.Execute(w, data); err != nil {
		log.Error().Err(err).Msg("public cloud signup page render failed")
	}
}

func (h *PublicCloudSignupHandlers) findTenantForEmail(email string) (string, bool, error) {
	tenants, err := h.registry.List()
	if err != nil {
		return "", false, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", false, nil
	}
	for _, tenant := range tenants {
		if tenant == nil {
			continue
		}
		if !strings.EqualFold(tenant.Email, email) {
			continue
		}
		if tenant.State != registry.TenantStateActive && tenant.State != registry.TenantStateProvisioning {
			continue
		}
		return strings.TrimSpace(tenant.ID), true, nil
	}
	return "", false, nil
}

func (h *PublicCloudSignupHandlers) sendMagicLinkEmail(email, verifyURL string) error {
	if h.emailSender == nil || h.cfg == nil || strings.TrimSpace(h.cfg.EmailFrom) == "" {
		log.Info().
			Str("email", email).
			Str("magic_link_url_redacted", redactCloudMagicLinkURL(verifyURL)).
			Msg("Magic link generated (email sender unavailable)")
		return nil
	}

	htmlBody, textBody, err := cpemail.RenderMagicLinkEmail(cpemail.MagicLinkData{
		MagicLinkURL: verifyURL,
	})
	if err != nil {
		return fmt.Errorf("render magic link email: %w", err)
	}
	if err := h.emailSender.Send(context.Background(), cpemail.Message{
		From:    strings.TrimSpace(h.cfg.EmailFrom),
		To:      email,
		Subject: "Sign in to Pulse",
		HTML:    htmlBody,
		Text:    textBody,
	}); err != nil {
		return fmt.Errorf("send magic link email: %w", err)
	}
	return nil
}

func buildVerifyURLForEmail(baseURL, token string) string {
	baseURL = strings.TrimSpace(baseURL)
	token = strings.TrimSpace(token)
	if baseURL == "" || token == "" {
		return ""
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/auth/magic-link/verify"
	q := parsed.Query()
	q.Set("token", token)
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

func redactCloudMagicLinkURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return ""
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func isValidCloudSignupEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	return strings.TrimSpace(parsed.Address) != ""
}

func isValidCloudSignupOrgName(orgName string) bool {
	orgName = strings.TrimSpace(orgName)
	if len(orgName) < 3 || len(orgName) > 64 {
		return false
	}
	for _, r := range orgName {
		if unicode.IsControl(r) {
			return false
		}
		if r == '/' || r == '\\' {
			return false
		}
	}
	return true
}

func writePublicSignupError(w http.ResponseWriter, status int, code, message string) {
	writePublicSignupJSON(w, status, map[string]any{
		"code":    code,
		"message": message,
	})
}

func writePublicSignupJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Error().Err(err).Msg("public cloud signup: encode response failed")
	}
}
