package cloudcp

import (
	"html/template"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpsec"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rs/zerolog/log"
	stripe "github.com/stripe/stripe-go/v82"
	stripesession "github.com/stripe/stripe-go/v82/checkout/session"
)

const (
	trialSignupDefaultOrgID            = "default"
	trialSignupTrialDays               = 14
	stripeCheckoutSessionIDPlaceholder = "{CHECKOUT_SESSION_ID}"
)

var trialSignupPageTemplate = template.Must(template.New("trial-signup-page").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Start Pulse Pro Trial</title>
  <style nonce="{{.Nonce}}">
    :root { color-scheme: light; }
    body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: linear-gradient(140deg, #f7fafc, #edf2f7); color: #1a202c; }
    .wrap { max-width: 720px; margin: 36px auto; padding: 0 16px; }
    .card { background: #fff; border-radius: 12px; border: 1px solid #e2e8f0; box-shadow: 0 8px 30px rgba(15,23,42,.08); padding: 24px; }
    h1 { margin: 0 0 8px; font-size: 28px; }
    p { margin: 0 0 16px; line-height: 1.5; color: #334155; }
    .meta { background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 8px; padding: 12px; margin-bottom: 16px; font-size: 14px; color: #475569; }
    label { display: block; margin: 12px 0 6px; font-size: 14px; font-weight: 600; color: #0f172a; }
    input { width: 100%; box-sizing: border-box; border: 1px solid #cbd5e1; border-radius: 8px; padding: 10px 12px; font-size: 15px; }
    .row { display: grid; gap: 12px; grid-template-columns: 1fr; }
    @media (min-width: 680px) { .row { grid-template-columns: 1fr 1fr; } }
    .error { background: #fef2f2; color: #991b1b; border: 1px solid #fecaca; border-radius: 8px; padding: 10px 12px; margin-bottom: 12px; font-size: 14px; }
    .note { background: #f0fdf4; color: #166534; border: 1px solid #bbf7d0; border-radius: 8px; padding: 10px 12px; margin-bottom: 12px; font-size: 14px; }
    .cta { margin-top: 16px; border: 0; border-radius: 10px; background: #0f766e; color: #fff; font-size: 16px; font-weight: 600; padding: 12px 16px; width: 100%; cursor: pointer; }
    .cta:hover { background: #0d6b63; }
    .fine { font-size: 12px; color: #64748b; margin-top: 12px; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <h1>Start Your 14-Day Pulse Pro Trial</h1>
      <p>Complete registration to start your trial. Your card is collected now and billing begins only after the trial period unless you cancel first.</p>

      {{if .ErrorMessage}}<div class="error">{{.ErrorMessage}}</div>{{end}}
      {{if .Cancelled}}<div class="note">Checkout was cancelled. You can retry at any time.</div>{{end}}

      <div class="meta">
        <div><strong>Organization:</strong> {{.OrgID}}</div>
        <div><strong>Activation Return URL:</strong> {{.ReturnURL}}</div>
      </div>

      <form method="POST" action="/api/trial-signup/checkout">
        <input type="hidden" name="org_id" value="{{.OrgID}}">
        <input type="hidden" name="return_url" value="{{.ReturnURL}}">

        <div class="row">
          <div>
            <label for="name">Full Name</label>
            <input id="name" name="name" type="text" value="{{.Name}}" autocomplete="name" required>
          </div>
          <div>
            <label for="email">Work Email</label>
            <input id="email" name="email" type="email" value="{{.Email}}" autocomplete="email" required>
          </div>
        </div>

        <label for="company">Company</label>
        <input id="company" name="company" type="text" value="{{.Company}}" autocomplete="organization">

        <button class="cta" type="submit">Continue To Secure Checkout</button>
      </form>

      <p class="fine">By continuing, you agree to the Pulse terms and billing policy.</p>
    </div>
  </div>
</body>
</html>
`))

type TrialSignupHandlers struct {
	cfg                   *CPConfig
	createCheckoutSession func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error)
	getCheckoutSession    func(id string, params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error)
	now                   func() time.Time
}

type trialSignupPageData struct {
	OrgID        string
	ReturnURL    string
	Name         string
	Email        string
	Company      string
	ErrorMessage string
	Cancelled    bool
	Nonce        string
}

func NewTrialSignupHandlers(cfg *CPConfig) *TrialSignupHandlers {
	return &TrialSignupHandlers{
		cfg:                   cfg,
		createCheckoutSession: stripesession.New,
		getCheckoutSession:    stripesession.Get,
		now:                   func() time.Time { return time.Now().UTC() },
	}
}

// HandleStartProTrial renders the hosted trial registration form.
func (h *TrialSignupHandlers) HandleStartProTrial(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data := trialSignupPageData{
		OrgID:     normalizeTrialOrgID(r.URL.Query().Get("org_id")),
		ReturnURL: strings.TrimSpace(r.URL.Query().Get("return_url")),
		Email:     strings.TrimSpace(r.URL.Query().Get("email")),
		Name:      strings.TrimSpace(r.URL.Query().Get("name")),
		Company:   strings.TrimSpace(r.URL.Query().Get("company")),
		Cancelled: strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("cancelled")), "1"),
	}
	if data.ReturnURL == "" {
		data.ErrorMessage = "Missing return_url. Please restart from Pulse Settings > Pro License."
	}
	h.renderTrialSignupPage(w, r, http.StatusOK, data)
}

// HandleCheckout creates a Stripe Checkout Session for trial signup.
func (h *TrialSignupHandlers) HandleCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form body", http.StatusBadRequest)
		return
	}

	data := trialSignupPageData{
		OrgID:     normalizeTrialOrgID(r.FormValue("org_id")),
		ReturnURL: strings.TrimSpace(r.FormValue("return_url")),
		Name:      strings.TrimSpace(r.FormValue("name")),
		Email:     strings.TrimSpace(r.FormValue("email")),
		Company:   strings.TrimSpace(r.FormValue("company")),
	}

	if !isValidTrialReturnURL(data.ReturnURL) {
		data.ErrorMessage = "A valid return URL is required. Please restart from Pulse Settings > Pro License."
		h.renderTrialSignupPage(w, r, http.StatusBadRequest, data)
		return
	}
	if !isValidTrialEmail(data.Email) {
		data.ErrorMessage = "A valid email address is required."
		h.renderTrialSignupPage(w, r, http.StatusBadRequest, data)
		return
	}
	if strings.TrimSpace(h.cfg.StripeAPIKey) == "" || strings.TrimSpace(h.cfg.TrialSignupPriceID) == "" {
		data.ErrorMessage = "Checkout is not configured yet. Please contact support."
		h.renderTrialSignupPage(w, r, http.StatusServiceUnavailable, data)
		return
	}

	stripe.Key = strings.TrimSpace(h.cfg.StripeAPIKey)
	successURL := buildTrialSignupSuccessURL(h.cfg.BaseURL)
	cancelURL := buildCPURL(h.cfg.BaseURL, "/start-pro-trial", url.Values{
		"cancelled":  {"1"},
		"org_id":     {data.OrgID},
		"return_url": {data.ReturnURL},
		"name":       {data.Name},
		"email":      {data.Email},
		"company":    {data.Company},
	})

	params := &stripe.CheckoutSessionParams{
		Mode:                    stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:              stripe.String(successURL),
		CancelURL:               stripe.String(cancelURL),
		CustomerEmail:           stripe.String(data.Email),
		PaymentMethodCollection: stripe.String(string(stripe.CheckoutSessionPaymentMethodCollectionAlways)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(strings.TrimSpace(h.cfg.TrialSignupPriceID)),
				Quantity: stripe.Int64(1),
			},
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			TrialPeriodDays: stripe.Int64(trialSignupTrialDays),
		},
		Metadata: map[string]string{
			"org_id":     data.OrgID,
			"return_url": data.ReturnURL,
			"name":       data.Name,
			"email":      data.Email,
			"company":    data.Company,
		},
	}

	session, err := h.createCheckoutSession(params)
	if err != nil || session == nil || strings.TrimSpace(session.URL) == "" {
		log.Error().Err(err).
			Str("org_id", data.OrgID).
			Str("email", data.Email).
			Msg("trial signup checkout session creation failed")
		data.ErrorMessage = "Unable to create checkout session. Please try again."
		h.renderTrialSignupPage(w, r, http.StatusBadGateway, data)
		return
	}

	http.Redirect(w, r, session.URL, http.StatusSeeOther)
}

// HandleTrialSignupComplete validates the Stripe checkout session and redirects
// back to Pulse with a signed one-time activation token.
func (h *TrialSignupHandlers) HandleTrialSignupComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(h.cfg.StripeAPIKey) == "" {
		http.Error(w, "stripe api key not configured", http.StatusServiceUnavailable)
		return
	}

	privateKey, err := pkglicensing.DecodeEd25519PrivateKey(strings.TrimSpace(h.cfg.TrialActivationPrivateKey))
	if err != nil {
		log.Error().Err(err).Msg("trial activation private key invalid")
		http.Error(w, "trial activation signer unavailable", http.StatusServiceUnavailable)
		return
	}

	stripe.Key = strings.TrimSpace(h.cfg.StripeAPIKey)
	session, err := h.getCheckoutSession(sessionID, nil)
	if err != nil || session == nil {
		log.Warn().Err(err).Str("session_id", sessionID).Msg("trial signup checkout session lookup failed")
		http.Error(w, "invalid checkout session", http.StatusBadRequest)
		return
	}
	if session.Status != stripe.CheckoutSessionStatusComplete {
		http.Error(w, "checkout session not complete", http.StatusBadRequest)
		return
	}

	returnURL := strings.TrimSpace(session.Metadata["return_url"])
	if !isValidTrialReturnURL(returnURL) {
		http.Error(w, "invalid return url", http.StatusBadRequest)
		return
	}
	parsedReturnURL, _ := url.Parse(returnURL)
	instanceHost := strings.TrimSpace(parsedReturnURL.Hostname())
	if instanceHost == "" {
		http.Error(w, "invalid return url host", http.StatusBadRequest)
		return
	}

	orgID := normalizeTrialOrgID(session.Metadata["org_id"])
	email := strings.TrimSpace(session.Metadata["email"])
	if strings.TrimSpace(email) == "" {
		email = strings.TrimSpace(session.CustomerEmail)
	}
	if session.CustomerDetails != nil && strings.TrimSpace(session.CustomerDetails.Email) != "" {
		email = strings.TrimSpace(session.CustomerDetails.Email)
	}

	now := h.now().UTC()
	token, err := pkglicensing.SignTrialActivationToken(privateKey, pkglicensing.TrialActivationClaims{
		OrgID:        orgID,
		Email:        email,
		InstanceHost: instanceHost,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
			Subject:   sessionID,
		},
	})
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionID).Msg("trial activation token signing failed")
		http.Error(w, "failed to generate activation token", http.StatusInternalServerError)
		return
	}

	finalReturnURL, err := appendQueryParams(returnURL, map[string]string{
		"token": token,
	})
	if err != nil {
		log.Error().Err(err).Str("return_url", returnURL).Msg("failed to build trial activation redirect URL")
		http.Error(w, "failed to build redirect URL", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, finalReturnURL, http.StatusSeeOther)
}

func (h *TrialSignupHandlers) renderTrialSignupPage(w http.ResponseWriter, r *http.Request, status int, data trialSignupPageData) {
	data.Nonce = cpsec.NonceFromContext(r.Context())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := trialSignupPageTemplate.Execute(w, data); err != nil {
		log.Error().Err(err).Msg("trial signup page render failed")
	}
}

func isValidTrialEmail(email string) bool {
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

func isValidTrialReturnURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return false
	}
	if !parsed.IsAbs() || strings.TrimSpace(parsed.Host) == "" {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return true
	default:
		return false
	}
}

func normalizeTrialOrgID(raw string) string {
	orgID := strings.TrimSpace(raw)
	if orgID == "" {
		return trialSignupDefaultOrgID
	}
	return orgID
}

func buildCPURL(baseURL, path string, query url.Values) string {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		return path
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return path
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func buildTrialSignupSuccessURL(baseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed == nil {
		return "/trial-signup/complete?session_id=" + stripeCheckoutSessionIDPlaceholder
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/trial-signup/complete"
	encoded := url.Values{
		"session_id": {stripeCheckoutSessionIDPlaceholder},
	}.Encode()
	parsed.RawQuery = strings.ReplaceAll(
		encoded,
		url.QueryEscape(stripeCheckoutSessionIDPlaceholder),
		stripeCheckoutSessionIDPlaceholder,
	)
	return parsed.String()
}

func appendQueryParams(base string, params map[string]string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	for key, value := range params {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
