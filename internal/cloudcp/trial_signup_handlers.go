package cloudcp

import (
	"context"
	"crypto/ed25519"
	"errors"
	"html/template"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpsec"
	cpemail "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/email"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rs/zerolog/log"
	stripe "github.com/stripe/stripe-go/v82"
	stripesession "github.com/stripe/stripe-go/v82/checkout/session"
)

const (
	trialSignupDefaultOrgID            = "default"
	trialSignupTrialDays               = 14
	trialSignupVerificationTTL         = 20 * time.Minute
	stripeCheckoutSessionIDPlaceholder = "{CHECKOUT_SESSION_ID}"
	trialSignupCheckoutIssuer          = "pulse-pro-trial-checkout"
	trialSignupCheckoutAudience        = "pulse-pro-trial-checkout"
)

var trialSignupPageTemplate = template.Must(template.New("trial-signup-page").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Start Pulse Pro Trial</title>
  <style nonce="{{.Nonce}}">
    :root {
      color-scheme: light;
      --bg-top: #f7f3ea;
      --bg-bottom: #e9efe8;
      --card: #fffdf8;
      --line: #d8dccf;
      --text: #14261f;
      --muted: #4d5f57;
      --accent: #0f766e;
      --accent-deep: #115e59;
      --soft-accent: #def7ec;
      --error-bg: #fef2f2;
      --error-line: #fecaca;
      --error-text: #991b1b;
      --note-bg: #eefbf4;
      --note-line: #bfe8ce;
      --note-text: #166534;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background:
        radial-gradient(circle at top left, rgba(15,118,110,.10), transparent 34%),
        linear-gradient(155deg, var(--bg-top), var(--bg-bottom));
      color: var(--text);
    }
    .wrap { max-width: 840px; margin: 40px auto; padding: 0 18px; }
    .card {
      background: var(--card);
      border-radius: 18px;
      border: 1px solid rgba(20,38,31,.10);
      box-shadow: 0 18px 60px rgba(20,38,31,.10);
      overflow: hidden;
    }
    .hero {
      padding: 28px 28px 18px;
      background:
        linear-gradient(130deg, rgba(15,118,110,.10), rgba(15,118,110,0)),
        linear-gradient(180deg, rgba(255,255,255,.92), rgba(255,255,255,.75));
      border-bottom: 1px solid rgba(20,38,31,.08);
    }
    .eyebrow {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      padding: 6px 10px;
      border-radius: 999px;
      background: rgba(15,118,110,.10);
      color: var(--accent-deep);
      font-size: 12px;
      font-weight: 700;
      letter-spacing: .04em;
      text-transform: uppercase;
      margin-bottom: 14px;
    }
    .hero h1 { margin: 0 0 10px; font-size: 34px; line-height: 1.05; }
    .hero p { margin: 0; max-width: 620px; font-size: 16px; line-height: 1.6; color: var(--muted); }
    .content { display: grid; gap: 0; }
    @media (min-width: 760px) { .content { grid-template-columns: 1.2fr .8fr; } }
    .form-col { padding: 24px 28px 28px; }
    .aside {
      padding: 24px 28px 28px;
      background: rgba(20,38,31,.03);
      border-top: 1px solid rgba(20,38,31,.08);
    }
    @media (min-width: 760px) { .aside { border-top: 0; border-left: 1px solid rgba(20,38,31,.08); } }
    h2 { margin: 0 0 10px; font-size: 20px; }
    p { margin: 0 0 16px; line-height: 1.6; color: var(--muted); }
    .meta {
      background: rgba(255,255,255,.75);
      border: 1px solid var(--line);
      border-radius: 12px;
      padding: 14px 16px;
      margin-bottom: 16px;
      font-size: 14px;
      color: var(--muted);
    }
    .meta strong, .summary strong { display: block; margin-bottom: 3px; color: var(--text); font-size: 12px; letter-spacing: .03em; text-transform: uppercase; }
    .summary {
      background: rgba(255,255,255,.78);
      border: 1px solid var(--line);
      border-radius: 12px;
      padding: 14px 16px;
      margin: 16px 0;
    }
    .summary + .summary { margin-top: 12px; }
    label { display: block; margin: 12px 0 6px; font-size: 14px; font-weight: 600; color: #0f172a; }
    input {
      width: 100%;
      border: 1px solid #c8d2ca;
      border-radius: 10px;
      padding: 11px 13px;
      font-size: 15px;
      background: #fff;
      color: var(--text);
    }
    .row { display: grid; gap: 12px; grid-template-columns: 1fr; }
    @media (min-width: 680px) { .row { grid-template-columns: 1fr 1fr; } }
    .error, .note {
      border-radius: 10px;
      padding: 11px 13px;
      margin-bottom: 14px;
      font-size: 14px;
      line-height: 1.5;
    }
    .error { background: var(--error-bg); color: var(--error-text); border: 1px solid var(--error-line); }
    .note { background: var(--note-bg); color: var(--note-text); border: 1px solid var(--note-line); }
    .cta {
      margin-top: 18px;
      border: 0;
      border-radius: 12px;
      background: var(--accent);
      color: #fff;
      font-size: 16px;
      font-weight: 700;
      padding: 13px 18px;
      width: 100%;
      cursor: pointer;
    }
    .cta:hover { background: var(--accent-deep); }
    .fine { font-size: 12px; color: #64748b; margin-top: 12px; }
    .steps { margin: 0; padding-left: 18px; color: var(--muted); }
    .steps li { margin-bottom: 10px; line-height: 1.5; }
    .mini { font-size: 13px; color: #5f7168; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <div class="hero">
        <div class="eyebrow">Pulse Pro Trial</div>
        <h1>Start a 14-day Pro trial for {{.ReturnTarget}}</h1>
        <p>Use your work email to confirm ownership, then continue to secure setup. When registration completes, Pulse returns you to this instance automatically.</p>
      </div>
      <div class="content">
        <div class="form-col">
          {{if .ErrorMessage}}<div class="error">{{.ErrorMessage}}</div>{{end}}
          {{if .Cancelled}}<div class="note">Secure setup was cancelled. You can continue again below.</div>{{end}}
          {{if .VerificationSent}}<div class="note">Check {{.Email}} for a verification link. It expires in 20 minutes.</div>{{end}}

          {{if .Verified}}
          <h2>Email verified</h2>
          <p>Your work email has been confirmed. Continue to secure setup to register the Pro trial for this Pulse instance.</p>

          <div class="summary">
            <strong>Work email</strong>
            <div>{{.Email}}</div>
          </div>
          <div class="summary">
            <strong>Name</strong>
            <div>{{.Name}}</div>
          </div>
          {{if .Company}}<div class="summary">
            <strong>Company</strong>
            <div>{{.Company}}</div>
          </div>{{end}}

          <form method="POST" action="/api/trial-signup/checkout">
            <input type="hidden" name="verified_token" value="{{.VerifiedToken}}">
            <button class="cta" type="submit">Continue To Secure Setup</button>
          </form>
          <p class="fine">No credit card is required to start the trial.</p>
          {{else}}
          <h2>Verify your work email</h2>
          <p>I’ll send a one-time verification link before trial setup continues.</p>

          <form method="POST" action="/api/trial-signup/request-verification">
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

            <button class="cta" type="submit">Email Me a Verification Link</button>
          </form>
          <p class="fine">I only use this email to verify the trial request and attach the resulting entitlement.</p>
          {{end}}
        </div>

        <div class="aside">
          <div class="meta">
            <strong>Pulse instance</strong>
            <div>{{.ReturnTarget}}</div>
          </div>
          <div class="meta">
            <strong>Workspace</strong>
            <div>{{.OrgID}}</div>
          </div>
          <p class="mini">After trial setup completes, Pulse sends you straight back to the instance that started this flow.</p>
          <ol class="steps">
            <li>Confirm the request from your work inbox.</li>
            <li>Complete secure setup for the 14-day Pro trial.</li>
            <li>Return to Pulse with a signed activation token.</li>
          </ol>
        </div>
      </div>
    </div>
  </div>
</body>
</html>
`))

type TrialSignupHandlers struct {
	cfg                   *CPConfig
	emailSender           cpemail.Sender
	verificationStore     *TrialSignupStore
	createCheckoutSession func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error)
	getCheckoutSession    func(id string, params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error)
	now                   func() time.Time
}

type trialSignupPageData struct {
	OrgID            string
	ReturnURL        string
	ReturnTarget     string
	Name             string
	Email            string
	Company          string
	ErrorMessage     string
	Cancelled        bool
	VerificationSent bool
	Verified         bool
	VerifiedToken    string
	Nonce            string
}

type trialSignupCheckoutClaims struct {
	RequestID string `json:"request_id"`
	jwt.RegisteredClaims
}

func NewTrialSignupHandlers(cfg *CPConfig, emailSender cpemail.Sender, verificationStore *TrialSignupStore) *TrialSignupHandlers {
	return &TrialSignupHandlers{
		cfg:                   cfg,
		emailSender:           emailSender,
		verificationStore:     verificationStore,
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
		OrgID:        normalizeTrialOrgID(r.URL.Query().Get("org_id")),
		ReturnURL:    strings.TrimSpace(r.URL.Query().Get("return_url")),
		Name:         strings.TrimSpace(r.URL.Query().Get("name")),
		Email:        strings.TrimSpace(r.URL.Query().Get("email")),
		Company:      strings.TrimSpace(r.URL.Query().Get("company")),
		Cancelled:    strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("cancelled")), "1"),
		ReturnTarget: summarizeTrialReturnTarget(r.URL.Query().Get("return_url")),
	}
	if data.ReturnURL == "" {
		data.ErrorMessage = "Missing return_url. Please restart from Pulse Settings > Pro License."
	}
	h.renderTrialSignupPage(w, r, http.StatusOK, data)
}

// HandleRequestVerification emails a short-lived verification link before checkout.
func (h *TrialSignupHandlers) HandleRequestVerification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form body", http.StatusBadRequest)
		return
	}

	data := trialSignupPageData{
		OrgID:        normalizeTrialOrgID(r.FormValue("org_id")),
		ReturnURL:    strings.TrimSpace(r.FormValue("return_url")),
		Name:         strings.TrimSpace(r.FormValue("name")),
		Email:        strings.TrimSpace(r.FormValue("email")),
		Company:      strings.TrimSpace(r.FormValue("company")),
		ReturnTarget: summarizeTrialReturnTarget(r.FormValue("return_url")),
	}
	if !isValidTrialReturnURL(data.ReturnURL) {
		data.ErrorMessage = "A valid Pulse return URL is required. Please restart from Pulse Settings > Pro License."
		h.renderTrialSignupPage(w, r, http.StatusBadRequest, data)
		return
	}
	if strings.TrimSpace(data.Name) == "" {
		data.ErrorMessage = "Your name is required."
		h.renderTrialSignupPage(w, r, http.StatusBadRequest, data)
		return
	}
	if !isValidTrialEmail(data.Email) {
		data.ErrorMessage = "A valid work email address is required."
		h.renderTrialSignupPage(w, r, http.StatusBadRequest, data)
		return
	}
	if isPublicTrialSignupEmailDomain(normalizeTrialSignupEmailDomain(data.Email)) {
		data.ErrorMessage = "Use your work email to start a Pulse Pro trial. Consumer email addresses are not eligible."
		h.renderTrialSignupPage(w, r, http.StatusBadRequest, data)
		return
	}
	if h.verificationStore != nil {
		pendingRecord, err := h.verificationStore.FindPendingVerificationByEmail(data.Email, h.now().UTC())
		if err != nil {
			log.Error().Err(err).Str("email", data.Email).Msg("trial signup pending verification lookup failed")
			data.ErrorMessage = "Unable to validate trial eligibility right now. Please try again."
			h.renderTrialSignupPage(w, r, http.StatusInternalServerError, data)
			return
		}
		if pendingRecord != nil {
			data.ErrorMessage = "A verification email was already sent recently. Check your inbox or wait for the current link to expire before requesting another one."
			h.renderTrialSignupPage(w, r, http.StatusTooManyRequests, data)
			return
		}
		conflict, err := h.verificationStore.FindIssuedTrialConflict(data.Email, data.Company)
		if err != nil {
			log.Error().Err(err).Str("email", data.Email).Msg("trial signup issuance lookup failed")
			data.ErrorMessage = "Unable to validate trial eligibility right now. Please try again."
			h.renderTrialSignupPage(w, r, http.StatusInternalServerError, data)
			return
		}
		if conflict != nil {
			data.ErrorMessage = "This organization has already used a Pulse Pro trial. Upgrade the existing account or contact support if you need help."
			h.renderTrialSignupPage(w, r, http.StatusConflict, data)
			return
		}
	}
	if h.emailSender == nil || h.cfg == nil || strings.TrimSpace(h.cfg.EmailFrom) == "" || h.verificationStore == nil {
		data.ErrorMessage = "Email verification is not configured yet. Please contact support."
		h.renderTrialSignupPage(w, r, http.StatusServiceUnavailable, data)
		return
	}

	token, err := h.verificationStore.CreateVerification(&TrialSignupRecord{
		OrgID:                 data.OrgID,
		ReturnURL:             data.ReturnURL,
		Name:                  data.Name,
		Email:                 data.Email,
		Company:               data.Company,
		CreatedAt:             h.now().UTC(),
		VerificationExpiresAt: h.now().UTC().Add(trialSignupVerificationTTL),
	})
	if err != nil {
		log.Error().Err(err).Str("email", data.Email).Msg("trial signup verification record creation failed")
		data.ErrorMessage = "Unable to prepare the verification link. Please try again."
		h.renderTrialSignupPage(w, r, http.StatusInternalServerError, data)
		return
	}

	verifyURL := buildTrialSignupVerificationURL(h.cfg.BaseURL, token)
	if err := h.sendTrialVerificationEmail(data.Email, verifyURL); err != nil {
		log.Error().Err(err).Str("email", data.Email).Msg("trial signup verification email send failed")
		data.ErrorMessage = "Unable to send the verification email. Please try again."
		h.renderTrialSignupPage(w, r, http.StatusBadGateway, data)
		return
	}

	data.VerificationSent = true
	h.renderTrialSignupPage(w, r, http.StatusOK, data)
}

// HandleVerifyEmail validates the verification token and shows the final checkout step.
func (h *TrialSignupHandlers) HandleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
		record, err := h.verificationStore.ConsumeVerification(token, h.now().UTC())
		if err != nil {
			log.Warn().Err(err).Msg("trial signup verification token invalid")
			h.renderTrialSignupPage(w, r, http.StatusBadRequest, trialSignupPageData{
				ErrorMessage: "That verification link is invalid or expired. Request a fresh email from Pulse and try again.",
				ReturnTarget: "your Pulse instance",
			})
			return
		}
		checkoutPrivateKey, err := h.trialCheckoutPrivateKey()
		if err != nil {
			log.Error().Err(err).Msg("trial checkout private key unavailable")
			h.renderTrialSignupPage(w, r, http.StatusServiceUnavailable, trialSignupPageData{
				ErrorMessage: "Trial checkout is unavailable right now. Please try again shortly.",
				ReturnTarget: summarizeTrialReturnTarget(record.ReturnURL),
			})
			return
		}
		verifiedToken, err := h.signTrialSignupCheckoutToken(checkoutPrivateKey, trialSignupCheckoutClaims{
			RequestID: record.ID,
			RegisteredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(h.now().UTC()),
				ExpiresAt: jwt.NewNumericDate(h.now().UTC().Add(trialSignupVerificationTTL)),
				Subject:   record.ID,
			},
		})
		if err != nil {
			log.Error().Err(err).Str("request_id", record.ID).Msg("trial checkout token signing failed")
			h.renderTrialSignupPage(w, r, http.StatusInternalServerError, trialSignupPageData{
				ErrorMessage: "Unable to continue to trial checkout. Please try again.",
				ReturnTarget: summarizeTrialReturnTarget(record.ReturnURL),
			})
			return
		}
		http.Redirect(w, r, buildTrialSignupVerifiedURL(h.cfg.BaseURL, verifiedToken, false), http.StatusSeeOther)
		return
	}

	verifiedToken := strings.TrimSpace(r.URL.Query().Get("verified"))
	record, err := h.lookupVerifiedTrialSignupRecord(verifiedToken)
	if err != nil {
		log.Warn().Err(err).Msg("trial signup verified state invalid")
		h.renderTrialSignupPage(w, r, http.StatusBadRequest, trialSignupPageData{
			ErrorMessage: "That verification link is invalid or expired. Request a fresh email from Pulse and try again.",
			ReturnTarget: "your Pulse instance",
		})
		return
	}

	h.renderTrialSignupPage(w, r, http.StatusOK, trialSignupPageData{
		OrgID:         record.OrgID,
		ReturnURL:     record.ReturnURL,
		ReturnTarget:  summarizeTrialReturnTarget(record.ReturnURL),
		Name:          record.Name,
		Email:         record.Email,
		Company:       record.Company,
		Cancelled:     strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("cancelled")), "1"),
		Verified:      true,
		VerifiedToken: verifiedToken,
	})
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

	verifiedToken := strings.TrimSpace(r.FormValue("verified_token"))
	record, err := h.lookupVerifiedTrialSignupRecord(verifiedToken)
	if err != nil {
		log.Warn().Err(err).Msg("trial signup checkout requested without valid verified token")
		h.renderTrialSignupPage(w, r, http.StatusBadRequest, trialSignupPageData{
			ErrorMessage: "Please verify your work email before continuing to secure setup.",
			ReturnTarget: "your Pulse instance",
		})
		return
	}

	data := trialSignupPageData{
		OrgID:         record.OrgID,
		ReturnURL:     record.ReturnURL,
		ReturnTarget:  summarizeTrialReturnTarget(record.ReturnURL),
		Name:          record.Name,
		Email:         record.Email,
		Company:       record.Company,
		Verified:      true,
		VerifiedToken: verifiedToken,
	}

	if !isValidTrialReturnURL(data.ReturnURL) {
		data.ErrorMessage = "A valid Pulse return URL is required. Please restart from Pulse Settings > Pro License."
		h.renderTrialSignupPage(w, r, http.StatusBadRequest, data)
		return
	}
	if strings.TrimSpace(h.cfg.StripeAPIKey) == "" || strings.TrimSpace(h.cfg.TrialSignupPriceID) == "" {
		data.ErrorMessage = "Checkout is not configured yet. Please contact support."
		h.renderTrialSignupPage(w, r, http.StatusServiceUnavailable, data)
		return
	}

	stripe.Key = strings.TrimSpace(h.cfg.StripeAPIKey)
	if existingSessionID := strings.TrimSpace(record.CheckoutSessionID); existingSessionID != "" {
		existingSession, err := h.getCheckoutSession(existingSessionID, nil)
		if err == nil && existingSession != nil {
			switch existingSession.Status {
			case stripe.CheckoutSessionStatusComplete:
				http.Redirect(w, r, buildTrialSignupSuccessURLWithSession(h.cfg.BaseURL, existingSessionID), http.StatusSeeOther)
				return
			case stripe.CheckoutSessionStatusOpen:
				if existingURL := strings.TrimSpace(existingSession.URL); existingURL != "" {
					http.Redirect(w, r, existingURL, http.StatusSeeOther)
					return
				}
			}
		}
	}
	successURL := buildTrialSignupSuccessURL(h.cfg.BaseURL)
	cancelURL := buildTrialSignupVerifiedURL(h.cfg.BaseURL, verifiedToken, true)

	params := &stripe.CheckoutSessionParams{
		Mode:                    stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:              stripe.String(successURL),
		CancelURL:               stripe.String(cancelURL),
		CustomerEmail:           stripe.String(data.Email),
		PaymentMethodCollection: stripe.String(string(stripe.CheckoutSessionPaymentMethodCollectionIfRequired)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(strings.TrimSpace(h.cfg.TrialSignupPriceID)),
				Quantity: stripe.Int64(1),
			},
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			TrialPeriodDays: stripe.Int64(trialSignupTrialDays),
			TrialSettings: &stripe.CheckoutSessionSubscriptionDataTrialSettingsParams{
				EndBehavior: &stripe.CheckoutSessionSubscriptionDataTrialSettingsEndBehaviorParams{
					MissingPaymentMethod: stripe.String("cancel"),
				},
			},
		},
		Metadata: map[string]string{
			"org_id":           data.OrgID,
			"return_url":       data.ReturnURL,
			"name":             data.Name,
			"email":            data.Email,
			"company":          data.Company,
			"trial_request_id": record.ID,
			"signup_source":    "pulse_pro_trial",
			"email_verified":   "true",
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
	if err := h.verificationStore.MarkCheckoutStarted(record.ID, session.ID, h.now().UTC()); err != nil {
		log.Warn().Err(err).Str("request_id", record.ID).Msg("trial signup checkout start could not be recorded")
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
	if session.Mode != stripe.CheckoutSessionModeSubscription {
		http.Error(w, "invalid checkout session mode", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(session.Metadata["signup_source"]) != "pulse_pro_trial" {
		http.Error(w, "invalid trial signup source", http.StatusBadRequest)
		return
	}
	if !strings.EqualFold(strings.TrimSpace(session.Metadata["email_verified"]), "true") {
		http.Error(w, "trial email not verified", http.StatusBadRequest)
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
	requestID := strings.TrimSpace(session.Metadata["trial_request_id"])
	if requestID == "" {
		http.Error(w, "missing trial request id", http.StatusBadRequest)
		return
	}
	if h.verificationStore == nil {
		http.Error(w, "trial signup store not configured", http.StatusServiceUnavailable)
		return
	}
	record, err := h.verificationStore.GetRecord(requestID)
	if err != nil {
		http.Error(w, "invalid trial request", http.StatusBadRequest)
		return
	}
	if record.VerifiedAt.IsZero() {
		http.Error(w, "invalid trial request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(record.OrgID) != orgID {
		http.Error(w, "trial request org mismatch", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(record.ReturnURL) != returnURL {
		http.Error(w, "trial request return url mismatch", http.StatusBadRequest)
		return
	}
	if verifiedEmail := normalizeTrialSignupEmail(record.Email); verifiedEmail == "" || normalizeTrialSignupEmail(email) != verifiedEmail {
		http.Error(w, "trial request email mismatch", http.StatusBadRequest)
		return
	}
	if existingSessionID := strings.TrimSpace(record.CheckoutSessionID); existingSessionID != "" && existingSessionID != sessionID {
		http.Error(w, "trial request checkout session mismatch", http.StatusBadRequest)
		return
	}
	if err := h.verificationStore.MarkCheckoutCompleted(requestID, sessionID, now); err != nil {
		switch {
		case errors.Is(err, ErrTrialSignupRecordNotFound):
			http.Error(w, "invalid trial request", http.StatusBadRequest)
		default:
			log.Error().Err(err).Str("request_id", requestID).Str("session_id", sessionID).Msg("failed to record checkout completion")
			http.Error(w, "failed to record checkout completion", http.StatusInternalServerError)
		}
		return
	}
	if err := h.verificationStore.MarkTrialIssued(requestID, now); err != nil {
		switch {
		case errors.Is(err, ErrTrialSignupEmailAlreadyUsed):
			http.Error(w, "trial already used for this email", http.StatusConflict)
		case errors.Is(err, ErrTrialSignupOrganizationUsed):
			http.Error(w, "trial already used for this organization", http.StatusConflict)
		case errors.Is(err, ErrTrialSignupRecordNotFound), errors.Is(err, ErrTrialSignupVerificationInvalid):
			http.Error(w, "invalid trial request", http.StatusBadRequest)
		default:
			log.Error().Err(err).Str("request_id", requestID).Msg("failed to record trial issuance")
			http.Error(w, "failed to record trial issuance", http.StatusInternalServerError)
		}
		return
	}
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
	token, _, err = h.verificationStore.StoreOrLoadActivationToken(requestID, token, now)
	if err != nil {
		log.Error().Err(err).Str("request_id", requestID).Msg("failed to persist trial activation token")
		http.Error(w, "failed to persist activation token", http.StatusInternalServerError)
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

func (h *TrialSignupHandlers) trialActivationPrivateKey() (ed25519.PrivateKey, error) {
	if h == nil || h.cfg == nil {
		return nil, pkglicensing.ErrTrialActivationPrivateKeyMissing
	}
	return pkglicensing.DecodeEd25519PrivateKey(strings.TrimSpace(h.cfg.TrialActivationPrivateKey))
}

func (h *TrialSignupHandlers) trialCheckoutPrivateKey() (ed25519.PrivateKey, error) {
	if h == nil || h.cfg == nil {
		return nil, pkglicensing.ErrTrialActivationPrivateKeyMissing
	}
	return pkglicensing.DecodeEd25519PrivateKey(strings.TrimSpace(h.cfg.TrialCheckoutPrivateKey))
}

func (h *TrialSignupHandlers) signTrialSignupCheckoutToken(privateKey ed25519.PrivateKey, claims trialSignupCheckoutClaims) (string, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return "", pkglicensing.ErrTrialActivationPrivateKeyInvalid
	}
	claims.RequestID = strings.TrimSpace(claims.RequestID)
	if claims.RequestID == "" {
		return "", jwt.ErrTokenMalformed
	}
	if claims.IssuedAt == nil {
		now := h.now().UTC()
		claims.IssuedAt = jwt.NewNumericDate(now)
	}
	if claims.ExpiresAt == nil {
		claims.ExpiresAt = jwt.NewNumericDate(h.now().UTC().Add(trialSignupVerificationTTL))
	}
	if strings.TrimSpace(claims.Issuer) == "" {
		claims.Issuer = trialSignupCheckoutIssuer
	}
	if len(claims.Audience) == 0 {
		claims.Audience = jwt.ClaimStrings{trialSignupCheckoutAudience}
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	return token.SignedString(privateKey)
}

func (h *TrialSignupHandlers) verifyTrialSignupCheckoutToken(token string) (*trialSignupCheckoutClaims, error) {
	privateKey, err := h.trialCheckoutPrivateKey()
	if err != nil {
		return nil, err
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok || len(publicKey) != ed25519.PublicKeySize {
		return nil, pkglicensing.ErrTrialActivationPublicKeyInvalid
	}

	claims := &trialSignupCheckoutClaims{}
	parsed, err := jwt.ParseWithClaims(
		strings.TrimSpace(token),
		claims,
		func(t *jwt.Token) (any, error) {
			if t.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
				return nil, jwt.ErrTokenSignatureInvalid
			}
			return publicKey, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(trialSignupCheckoutIssuer),
		jwt.WithAudience(trialSignupCheckoutAudience),
		jwt.WithTimeFunc(func() time.Time { return h.now().UTC() }),
	)
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	claims.RequestID = strings.TrimSpace(claims.RequestID)
	if claims.RequestID == "" {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

func (h *TrialSignupHandlers) lookupVerifiedTrialSignupRecord(verifiedToken string) (*TrialSignupRecord, error) {
	claims, err := h.verifyTrialSignupCheckoutToken(verifiedToken)
	if err != nil {
		return nil, err
	}
	if h.verificationStore == nil {
		return nil, ErrTrialSignupRecordNotFound
	}
	record, err := h.verificationStore.GetRecord(claims.RequestID)
	if err != nil {
		return nil, err
	}
	if record.VerifiedAt.IsZero() {
		return nil, ErrTrialSignupVerificationInvalid
	}
	return record, nil
}

func (h *TrialSignupHandlers) sendTrialVerificationEmail(email, verifyURL string) error {
	htmlBody, textBody, err := cpemail.RenderTrialVerificationEmail(cpemail.TrialVerificationData{
		VerifyURL: verifyURL,
	})
	if err != nil {
		return err
	}
	return h.emailSender.Send(context.Background(), cpemail.Message{
		From:    strings.TrimSpace(h.cfg.EmailFrom),
		To:      email,
		Subject: "Verify your Pulse Pro trial request",
		HTML:    htmlBody,
		Text:    textBody,
	})
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

func buildTrialSignupSuccessURLWithSession(baseURL, sessionID string) string {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed == nil {
		return "/trial-signup/complete?session_id=" + url.QueryEscape(strings.TrimSpace(sessionID))
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/trial-signup/complete"
	parsed.RawQuery = url.Values{
		"session_id": {strings.TrimSpace(sessionID)},
	}.Encode()
	return parsed.String()
}

func buildTrialSignupVerificationURL(baseURL, token string) string {
	query := url.Values{}
	if strings.TrimSpace(token) != "" {
		query.Set("token", strings.TrimSpace(token))
	}
	return buildCPURL(baseURL, "/trial-signup/verify", query)
}

func buildTrialSignupVerifiedURL(baseURL, verifiedToken string, cancelled bool) string {
	query := url.Values{}
	if strings.TrimSpace(verifiedToken) != "" {
		query.Set("verified", strings.TrimSpace(verifiedToken))
	}
	if cancelled {
		query.Set("cancelled", "1")
	}
	return buildCPURL(baseURL, "/trial-signup/verify", query)
}

func summarizeTrialReturnTarget(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return "your Pulse instance"
	}
	if host := strings.TrimSpace(parsed.Host); host != "" {
		return host
	}
	return "your Pulse instance"
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
