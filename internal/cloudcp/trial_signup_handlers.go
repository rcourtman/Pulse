package cloudcp

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpsec"
	cpemail "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/email"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/entitlements"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rs/zerolog/log"
	stripe "github.com/stripe/stripe-go/v82"
	stripesession "github.com/stripe/stripe-go/v82/checkout/session"
)

const (
	trialSignupDefaultOrgID            = "default"
	trialSignupTrialDays               = 14
	trialSignupVerificationTTL         = 20 * time.Minute
	trialSignupActivationTokenTTL      = 10 * time.Minute
	stripeCheckoutSessionIDPlaceholder = "{CHECKOUT_SESSION_ID}"
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
        <h1>Start your 14-day Pro trial for {{.ReturnTarget}}</h1>
        <p>Pulse uses a secure hosted handoff to create the trial entitlement for this specific instance and send you straight back here. I only keep your work email as a recovery contact if this browser session is lost.</p>
      </div>
      <div class="content">
        <div class="form-col">
          {{if .ErrorMessage}}<div class="error">{{.ErrorMessage}}</div>{{end}}
          {{if .Cancelled}}<div class="note">Secure setup was cancelled. You can continue again below.</div>{{end}}
          {{if .VerificationSent}}<div class="note">A backup link was sent to {{.Email}}. You do not need it unless this browser session is lost.</div>{{end}}

          {{if .Verified}}
          <h2>Backup link confirmed</h2>
          <p>This recovery link is valid. Continue securely to finish starting the Pro trial for this Pulse instance.</p>

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
            <button class="cta" type="submit">Continue To Secure Trial Setup</button>
          </form>
          <p class="fine">This recovery link is optional. The secure trial session remains the authoritative entitlement handoff.</p>
          {{else}}
          <h2>Continue securely</h2>
          <p>Start the secure trial handoff from this Pulse-initiated session. Email is attached as a recovery contact only.</p>

          <form method="POST" action="/api/trial-signup/checkout">
            <input type="hidden" name="org_id" value="{{.OrgID}}">
            <input type="hidden" name="return_url" value="{{.ReturnURL}}">
            <input type="hidden" name="instance_token" value="{{.InstanceToken}}">

            <div class="row">
              <div>
                <label for="name">Name</label>
                <input id="name" name="name" type="text" value="{{.Name}}" autocomplete="name" required>
              </div>
              <div>
                <label for="email">Work Email</label>
                <input id="email" name="email" type="email" value="{{.Email}}" autocomplete="email" required>
              </div>
            </div>

            <label for="company">Company (optional)</label>
            <input id="company" name="company" type="text" value="{{.Company}}" autocomplete="organization">

            <button class="cta" type="submit">Continue To Secure Trial Setup</button>
          </form>
          <p class="fine">Email is used as a recovery contact only. The secure session and signed return to Pulse carry the entitlement.</p>
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
          <p class="mini">After secure setup completes, Pulse sends you back to the exact instance that opened this flow.</p>
          <ol class="steps">
            <li>Open a secure trial session for this Pulse instance.</li>
            <li>Confirm the 14-day Pro trial handoff.</li>
            <li>Return to Pulse and activate the entitlement immediately.</li>
          </ol>
        </div>
      </div>
    </div>
  </div>
</body>
</html>
`))

var trialSignupSuccessTemplate = template.Must(template.New("trial-signup-success").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Pulse Pro Trial Ready</title>
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
      --good-bg: #eefbf4;
      --good-line: #bfe8ce;
      --good-text: #166534;
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
    .wrap { max-width: 760px; margin: 40px auto; padding: 0 18px; }
    .card {
      background: var(--card);
      border-radius: 18px;
      border: 1px solid rgba(20,38,31,.10);
      box-shadow: 0 18px 60px rgba(20,38,31,.10);
      padding: 28px;
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
    h1 { margin: 0 0 10px; font-size: 34px; line-height: 1.05; }
    p { margin: 0 0 16px; line-height: 1.6; color: var(--muted); }
    .status {
      background: var(--good-bg);
      border: 1px solid var(--good-line);
      border-radius: 12px;
      color: var(--good-text);
      padding: 14px 16px;
      margin: 20px 0;
      font-size: 14px;
      line-height: 1.5;
    }
    .meta {
      display: grid;
      gap: 16px;
      margin: 20px 0;
    }
    @media (min-width: 680px) { .meta { grid-template-columns: 1fr 1fr; } }
    .meta-card {
      background: rgba(255,255,255,.78);
      border: 1px solid var(--line);
      border-radius: 12px;
      padding: 14px 16px;
    }
    .meta-card strong {
      display: block;
      margin-bottom: 6px;
      color: var(--text);
      font-size: 12px;
      letter-spacing: .03em;
      text-transform: uppercase;
    }
    .cta {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      border: 0;
      border-radius: 12px;
      background: var(--accent);
      color: #fff;
      font-size: 16px;
      font-weight: 700;
      padding: 13px 18px;
      text-decoration: none;
    }
    .cta:hover { background: var(--accent-deep); }
    .fine { font-size: 12px; color: #64748b; margin-top: 14px; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <div class="eyebrow">Pulse Pro Trial</div>
      <h1>Trial entitlement ready</h1>
      <p>Secure trial setup completed successfully. This session has prepared the signed activation handoff for {{.ReturnTarget}}.</p>

      <div class="status">
        Pulse can activate the Pro trial immediately when you return to the originating instance.
      </div>

      <div class="meta">
        <div class="meta-card">
          <strong>Pulse instance</strong>
          <div>{{.ReturnTarget}}</div>
        </div>
        <div class="meta-card">
          <strong>Backup email</strong>
          <div>{{.Email}}</div>
        </div>
      </div>

      <a class="cta" href="{{.ActivateURL}}">Return To Pulse</a>
      <p class="fine">Email is only a recovery contact. The secure session and this signed return are the authoritative entitlement handoff.</p>
    </div>
  </div>
</body>
</html>
`))

var trialSignupFailureTemplate = template.Must(template.New("trial-signup-failure").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Pulse Pro Trial Setup Issue</title>
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
      --warn-bg: #fff7ed;
      --warn-line: #fdba74;
      --warn-text: #9a3412;
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
    .wrap { max-width: 760px; margin: 40px auto; padding: 0 18px; }
    .card {
      background: var(--card);
      border-radius: 18px;
      border: 1px solid rgba(20,38,31,.10);
      box-shadow: 0 18px 60px rgba(20,38,31,.10);
      padding: 28px;
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
    h1 { margin: 0 0 10px; font-size: 34px; line-height: 1.05; }
    p { margin: 0 0 16px; line-height: 1.6; color: var(--muted); }
    .status {
      background: var(--warn-bg);
      border: 1px solid var(--warn-line);
      border-radius: 12px;
      color: var(--warn-text);
      padding: 14px 16px;
      margin: 20px 0;
      font-size: 14px;
      line-height: 1.5;
    }
    .meta {
      background: rgba(255,255,255,.78);
      border: 1px solid var(--line);
      border-radius: 12px;
      padding: 14px 16px;
      margin: 20px 0;
    }
    .meta strong {
      display: block;
      margin-bottom: 6px;
      color: var(--text);
      font-size: 12px;
      letter-spacing: .03em;
      text-transform: uppercase;
    }
    .cta {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      border: 0;
      border-radius: 12px;
      background: var(--accent);
      color: #fff;
      font-size: 16px;
      font-weight: 700;
      padding: 13px 18px;
      text-decoration: none;
    }
    .cta:hover { background: var(--accent-deep); }
    .fine { font-size: 12px; color: #64748b; margin-top: 14px; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <div class="eyebrow">Pulse Pro Trial</div>
      <h1>{{.Title}}</h1>
      <p>{{.Message}}</p>

      <div class="status">{{.StatusMessage}}</div>

      <div class="meta">
        <strong>Pulse instance</strong>
        <div>{{.ReturnTarget}}</div>
      </div>

      {{if .ActionURL}}<a class="cta" href="{{.ActionURL}}">{{.ActionLabel}}</a>{{end}}
      <p class="fine">{{.FinePrint}}</p>
    </div>
  </div>
</body>
</html>
`))

type TrialSignupHandlers struct {
	cfg                   *CPConfig
	emailSender           cpemail.Sender
	verificationStore     *TrialSignupStore
	entitlements          *entitlements.Service
	createCheckoutSession func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error)
	getCheckoutSession    func(id string, params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error)
	now                   func() time.Time
}

type trialSignupPageData struct {
	OrgID            string
	ReturnURL        string
	InstanceToken    string
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

type trialSignupSuccessData struct {
	ReturnTarget string
	Email        string
	ActivateURL  string
	Nonce        string
}

type trialSignupFailureData struct {
	Title         string
	Message       string
	ReturnTarget  string
	StatusMessage string
	ActionURL     string
	ActionLabel   string
	FinePrint     string
	Nonce         string
}

type trialSignupFailureKind string

const (
	trialSignupFailureRetryable   trialSignupFailureKind = "retryable"
	trialSignupFailureConflict    trialSignupFailureKind = "conflict"
	trialSignupFailureInvalidLink trialSignupFailureKind = "invalid_link"
	trialSignupFailureUnavailable trialSignupFailureKind = "unavailable"
)

type trialSignupRedeemRequest struct {
	Token string `json:"token"`
}

type trialSignupRedeemResponse struct {
	EntitlementJWT          string `json:"entitlement_jwt"`
	EntitlementRefreshToken string `json:"entitlement_refresh_token"`
}

func NewTrialSignupHandlers(cfg *CPConfig, emailSender cpemail.Sender, verificationStore *TrialSignupStore, entitlementService *entitlements.Service) *TrialSignupHandlers {
	return &TrialSignupHandlers{
		cfg:                   cfg,
		emailSender:           emailSender,
		verificationStore:     verificationStore,
		entitlements:          entitlementService,
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
		OrgID:         normalizeTrialOrgID(r.URL.Query().Get("org_id")),
		ReturnURL:     strings.TrimSpace(r.URL.Query().Get("return_url")),
		InstanceToken: strings.TrimSpace(r.URL.Query().Get("instance_token")),
		Name:          strings.TrimSpace(r.URL.Query().Get("name")),
		Email:         strings.TrimSpace(r.URL.Query().Get("email")),
		Company:       strings.TrimSpace(r.URL.Query().Get("company")),
		Cancelled:     strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("cancelled")), "1"),
		ReturnTarget:  summarizeTrialReturnTarget(r.URL.Query().Get("return_url")),
	}
	if data.ReturnURL == "" {
		data.ErrorMessage = "Missing return_url. Please restart from Pulse Settings > Pro License."
	}
	if strings.TrimSpace(data.InstanceToken) == "" {
		data.ErrorMessage = "Missing trial initiation token. Please restart from Pulse Settings > Pro License."
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
		OrgID:         normalizeTrialOrgID(r.FormValue("org_id")),
		ReturnURL:     strings.TrimSpace(r.FormValue("return_url")),
		InstanceToken: strings.TrimSpace(r.FormValue("instance_token")),
		Name:          strings.TrimSpace(r.FormValue("name")),
		Email:         strings.TrimSpace(r.FormValue("email")),
		Company:       strings.TrimSpace(r.FormValue("company")),
		ReturnTarget:  summarizeTrialReturnTarget(r.FormValue("return_url")),
	}
	if strings.TrimSpace(data.InstanceToken) == "" {
		data.ErrorMessage = "This trial request must be started from Pulse. Return to Pulse Settings > Pro License and try again."
		h.renderTrialSignupPage(w, r, http.StatusBadRequest, data)
		return
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
			h.renderTrialSignupFailurePage(w, r, http.StatusConflict, trialSignupFailureDataForPage(
				h.cfg,
				data,
				trialSignupFailureConflict,
				trialSignupIssuanceConflictMessage(conflict),
			))
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
		InstanceToken:         data.InstanceToken,
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
			h.renderTrialSignupFailurePage(w, r, http.StatusBadRequest, trialSignupFailureDataForPage(
				h.cfg,
				trialSignupPageData{ReturnTarget: "your Pulse instance"},
				trialSignupFailureInvalidLink,
				"That verification link is invalid or expired. Return to Pulse to request a fresh backup email.",
			))
			return
		}
		verifiedToken, err := h.verificationStore.IssueCheckoutToken(record.ID, h.now().UTC(), trialSignupVerificationTTL)
		if err != nil {
			log.Error().Err(err).Str("request_id", record.ID).Msg("trial checkout token issuance failed")
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
		h.renderTrialSignupFailurePage(w, r, http.StatusBadRequest, trialSignupFailureDataForPage(
			h.cfg,
			trialSignupPageData{ReturnTarget: "your Pulse instance"},
			trialSignupFailureInvalidLink,
			"That verification link is invalid or expired. Return to Pulse to request a fresh backup email.",
		))
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
	record := &TrialSignupRecord{}
	data := trialSignupPageData{}
	if verifiedToken != "" {
		verifiedRecord, err := h.lookupVerifiedTrialSignupRecord(verifiedToken)
		if err != nil {
			log.Warn().Err(err).Msg("trial signup checkout requested without valid verified token")
			h.renderTrialSignupFailurePage(w, r, http.StatusBadRequest, trialSignupFailureDataForPage(
				h.cfg,
				trialSignupPageData{ReturnTarget: "your Pulse instance"},
				trialSignupFailureInvalidLink,
				"That backup link is invalid or expired. Return to Pulse to create a fresh secure trial session.",
			))
			return
		}
		record = verifiedRecord
		data = trialSignupPageData{
			OrgID:         record.OrgID,
			ReturnURL:     record.ReturnURL,
			InstanceToken: record.InstanceToken,
			ReturnTarget:  summarizeTrialReturnTarget(record.ReturnURL),
			Name:          record.Name,
			Email:         record.Email,
			Company:       record.Company,
			Verified:      true,
			VerifiedToken: verifiedToken,
		}
	} else {
		data = trialSignupPageData{
			OrgID:         normalizeTrialOrgID(r.FormValue("org_id")),
			ReturnURL:     strings.TrimSpace(r.FormValue("return_url")),
			InstanceToken: strings.TrimSpace(r.FormValue("instance_token")),
			Name:          strings.TrimSpace(r.FormValue("name")),
			Email:         strings.TrimSpace(r.FormValue("email")),
			Company:       strings.TrimSpace(r.FormValue("company")),
			ReturnTarget:  summarizeTrialReturnTarget(r.FormValue("return_url")),
		}
		if strings.TrimSpace(data.InstanceToken) == "" {
			data.ErrorMessage = "This checkout must be started from Pulse. Return to Pulse Settings > Pro License and try again."
			h.renderTrialSignupPage(w, r, http.StatusBadRequest, data)
			return
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
		if h.verificationStore == nil {
			data.ErrorMessage = "Trial checkout is unavailable right now. Please try again."
			h.renderTrialSignupPage(w, r, http.StatusServiceUnavailable, data)
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
			h.renderTrialSignupFailurePage(w, r, http.StatusConflict, trialSignupFailureDataForPage(
				h.cfg,
				data,
				trialSignupFailureConflict,
				trialSignupIssuanceConflictMessage(conflict),
			))
			return
		}
		record = &TrialSignupRecord{
			OrgID:         data.OrgID,
			ReturnURL:     data.ReturnURL,
			InstanceToken: data.InstanceToken,
			Name:          data.Name,
			Email:         data.Email,
			Company:       data.Company,
			CreatedAt:     h.now().UTC(),
		}
		if err := h.verificationStore.CreateCheckoutRequest(record); err != nil {
			log.Error().Err(err).Str("email", data.Email).Msg("trial signup checkout request creation failed")
			data.ErrorMessage = "Unable to prepare checkout. Please try again."
			h.renderTrialSignupPage(w, r, http.StatusInternalServerError, data)
			return
		}
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
	successURL := buildTrialSignupSuccessURL(h.cfg.BaseURL)
	cancelURL := buildTrialSignupCancelURL(h.cfg.BaseURL, data, verifiedToken)

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
			"instance_token":   data.InstanceToken,
			"name":             data.Name,
			"email":            data.Email,
			"company":          data.Company,
			"trial_request_id": record.ID,
			"signup_source":    "pulse_pro_trial",
			"email_mode":       "backup",
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

// HandleTrialSignupComplete validates the Stripe checkout session and renders
// the hosted success page with the signed activation handoff back to Pulse.
func (h *TrialSignupHandlers) HandleTrialSignupComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fail := func(status int, kind trialSignupFailureKind, data trialSignupPageData, message string) {
		h.renderTrialSignupFailurePage(w, r, status, trialSignupFailureDataForPage(h.cfg, data, kind, message))
	}

	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID == "" {
		fail(http.StatusBadRequest, trialSignupFailureRetryable, trialSignupPageData{}, "This secure trial session is missing its completion token.")
		return
	}
	if strings.TrimSpace(h.cfg.StripeAPIKey) == "" {
		fail(http.StatusServiceUnavailable, trialSignupFailureUnavailable, trialSignupPageData{}, "Secure trial setup is unavailable right now.")
		return
	}

	stripe.Key = strings.TrimSpace(h.cfg.StripeAPIKey)
	session, err := h.getCheckoutSession(sessionID, nil)
	if err != nil || session == nil {
		log.Warn().Err(err).Str("session_id", sessionID).Msg("trial signup checkout session lookup failed")
		fail(http.StatusBadRequest, trialSignupFailureRetryable, trialSignupPageData{}, "This secure trial session could not be confirmed.")
		return
	}
	data := trialSignupPageData{
		OrgID:        normalizeTrialOrgID(session.Metadata["org_id"]),
		ReturnURL:    strings.TrimSpace(session.Metadata["return_url"]),
		ReturnTarget: summarizeTrialReturnTarget(session.Metadata["return_url"]),
	}
	if session.Status != stripe.CheckoutSessionStatusComplete {
		fail(http.StatusBadRequest, trialSignupFailureRetryable, data, "This secure trial session has not completed yet.")
		return
	}
	if session.Mode != stripe.CheckoutSessionModeSubscription {
		fail(http.StatusBadRequest, trialSignupFailureRetryable, data, "This secure trial session is not valid for a Pulse Pro trial.")
		return
	}
	if strings.TrimSpace(session.Metadata["signup_source"]) != "pulse_pro_trial" {
		fail(http.StatusBadRequest, trialSignupFailureRetryable, data, "This secure trial session is not valid for a Pulse Pro trial.")
		return
	}

	returnURL := strings.TrimSpace(session.Metadata["return_url"])
	if !isValidTrialReturnURL(returnURL) {
		fail(http.StatusBadRequest, trialSignupFailureRetryable, data, "This trial return target is no longer valid.")
		return
	}
	instanceHost, err := trialSignupReturnURLHost(returnURL)
	if err != nil {
		fail(http.StatusBadRequest, trialSignupFailureRetryable, data, "This trial return target is no longer valid.")
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
		fail(http.StatusBadRequest, trialSignupFailureRetryable, data, "This secure trial session is missing its request binding.")
		return
	}
	if h.verificationStore == nil {
		fail(http.StatusServiceUnavailable, trialSignupFailureUnavailable, data, "Secure trial setup is unavailable right now.")
		return
	}
	record, err := h.verificationStore.GetRecord(requestID)
	if err != nil {
		fail(http.StatusBadRequest, trialSignupFailureRetryable, data, "This secure trial request could not be confirmed.")
		return
	}
	data.InstanceToken = record.InstanceToken
	data.Name = record.Name
	data.Email = record.Email
	data.Company = record.Company
	if record.VerifiedAt.IsZero() {
		fail(http.StatusBadRequest, trialSignupFailureRetryable, data, "This secure trial request could not be confirmed.")
		return
	}
	if strings.TrimSpace(record.OrgID) != orgID {
		fail(http.StatusBadRequest, trialSignupFailureRetryable, data, "This secure trial request does not match the originating workspace.")
		return
	}
	if strings.TrimSpace(record.ReturnURL) != returnURL {
		fail(http.StatusBadRequest, trialSignupFailureRetryable, data, "This secure trial request does not match the originating Pulse instance.")
		return
	}
	if strings.TrimSpace(record.InstanceToken) == "" {
		fail(http.StatusBadRequest, trialSignupFailureRetryable, data, "This secure trial request is missing its Pulse initiation binding.")
		return
	}
	if verifiedEmail := normalizeTrialSignupEmail(record.Email); verifiedEmail == "" || normalizeTrialSignupEmail(email) != verifiedEmail {
		fail(http.StatusBadRequest, trialSignupFailureRetryable, data, "This secure trial request does not match the verified recovery contact.")
		return
	}
	if existingSessionID := strings.TrimSpace(record.CheckoutSessionID); existingSessionID != "" && existingSessionID != sessionID {
		fail(http.StatusBadRequest, trialSignupFailureRetryable, data, "This secure trial request does not match the checkout session that was already started.")
		return
	}
	if err := h.verificationStore.MarkCheckoutCompleted(requestID, sessionID, now); err != nil {
		switch {
		case errors.Is(err, ErrTrialSignupRecordNotFound):
			fail(http.StatusBadRequest, trialSignupFailureRetryable, data, "This secure trial request could not be confirmed.")
		default:
			log.Error().Err(err).Str("request_id", requestID).Str("session_id", sessionID).Msg("failed to record checkout completion")
			fail(http.StatusInternalServerError, trialSignupFailureUnavailable, data, "Pulse could not finish recording this secure trial checkout.")
		}
		return
	}
	if err := h.verificationStore.MarkTrialIssued(requestID, now); err != nil {
		switch {
		case errors.Is(err, ErrTrialSignupEmailAlreadyUsed):
			fail(http.StatusConflict, trialSignupFailureConflict, data, "This recovery email has already used a Pulse Pro trial.")
		case errors.Is(err, ErrTrialSignupOrganizationUsed):
			fail(http.StatusConflict, trialSignupFailureConflict, data, "This organization has already used a Pulse Pro trial.")
		case errors.Is(err, ErrTrialSignupRecordNotFound), errors.Is(err, ErrTrialSignupVerificationInvalid):
			fail(http.StatusBadRequest, trialSignupFailureRetryable, data, "This secure trial request could not be confirmed.")
		default:
			log.Error().Err(err).Str("request_id", requestID).Msg("failed to record trial issuance")
			fail(http.StatusInternalServerError, trialSignupFailureUnavailable, data, "Pulse could not finalize this trial issuance.")
		}
		return
	}
	if h.entitlements == nil {
		fail(http.StatusServiceUnavailable, trialSignupFailureUnavailable, data, "Secure trial activation is unavailable right now.")
		return
	}
	token, err := h.entitlements.IssueTrialActivation(entitlements.TrialActivationInput{
		RequestID:         requestID,
		OrgID:             orgID,
		Email:             email,
		ReturnURL:         returnURL,
		InstanceToken:     record.InstanceToken,
		InstanceHost:      instanceHost,
		CheckoutSessionID: sessionID,
		TrialStartedAt:    now,
		IssuedAt:          now,
		TTL:               trialSignupActivationTokenTTL,
	})
	if err != nil {
		log.Error().Err(err).Str("request_id", requestID).Str("session_id", sessionID).Msg("failed to persist trial activation token")
		fail(http.StatusInternalServerError, trialSignupFailureUnavailable, data, "Pulse could not prepare the activation handoff for this trial.")
		return
	}

	finalReturnURL, err := appendQueryParams(returnURL, map[string]string{
		"token": token,
	})
	if err != nil {
		log.Error().Err(err).Str("return_url", returnURL).Msg("failed to build trial activation redirect URL")
		fail(http.StatusInternalServerError, trialSignupFailureUnavailable, data, "Pulse could not prepare the return link back to your instance.")
		return
	}
	h.renderTrialSignupSuccessPage(w, r, http.StatusOK, trialSignupSuccessData{
		ReturnTarget: summarizeTrialReturnTarget(returnURL),
		Email:        email,
		ActivateURL:  finalReturnURL,
	})
}

func (h *TrialSignupHandlers) HandleTrialSignupRedeem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.verificationStore == nil {
		http.Error(w, "trial signup store not configured", http.StatusServiceUnavailable)
		return
	}
	if h.entitlements == nil {
		http.Error(w, "hosted entitlement service unavailable", http.StatusServiceUnavailable)
		return
	}

	var reqBody trialSignupRedeemRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	token := strings.TrimSpace(reqBody.Token)
	if token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	privateKey, err := pkglicensing.DecodeEd25519PrivateKey(strings.TrimSpace(h.cfg.TrialActivationPrivateKey))
	if err != nil {
		log.Error().Err(err).Msg("trial activation private key invalid")
		http.Error(w, "trial activation verifier unavailable", http.StatusServiceUnavailable)
		return
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok || len(publicKey) != ed25519.PublicKeySize {
		http.Error(w, "trial activation verifier unavailable", http.StatusServiceUnavailable)
		return
	}

	now := h.now().UTC()
	claims, err := pkglicensing.VerifyTrialActivationToken(token, publicKey, "", now)
	if err != nil {
		switch {
		case errors.Is(err, pkglicensing.ErrTrialActivationHostMismatch):
			http.Error(w, "activation token host mismatch", http.StatusBadRequest)
		case errors.Is(err, pkglicensing.ErrTrialActivationReturnURLMissing), errors.Is(err, pkglicensing.ErrTrialActivationReturnURLInvalid):
			http.Error(w, "invalid return url", http.StatusBadRequest)
		default:
			http.Error(w, "invalid activation token", http.StatusBadRequest)
		}
		return
	}
	if _, err := trialSignupReturnURLHost(claims.ReturnURL); err != nil {
		http.Error(w, "invalid return url host", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(claims.InstanceToken) == "" {
		http.Error(w, "activation token missing initiation token", http.StatusBadRequest)
		return
	}

	entitlement, err := h.entitlements.ResolveTrialActivation(token, claims.InstanceHost)
	if err != nil {
		switch {
		case errors.Is(err, entitlements.ErrHostedEntitlementNotFound), errors.Is(err, entitlements.ErrHostedEntitlementTargetMismatch), errors.Is(err, entitlements.ErrHostedEntitlementInactive), errors.Is(err, entitlements.ErrHostedEntitlementInvalid):
			http.Error(w, "invalid trial redemption", http.StatusBadRequest)
		default:
			log.Error().Err(err).Str("org_id", claims.OrgID).Msg("failed to resolve hosted trial activation")
			http.Error(w, "failed to load trial redemption", http.StatusInternalServerError)
		}
		return
	}
	if err := h.verificationStore.MarkRedemptionRecorded(entitlement.TrialRequestID, claims.OrgID, claims.ReturnURL, claims.InstanceToken, claims.InstanceHost, now); err != nil {
		switch {
		case errors.Is(err, ErrTrialSignupRecordNotFound):
			http.Error(w, "invalid trial redemption", http.StatusBadRequest)
		default:
			log.Error().Err(err).Str("org_id", claims.OrgID).Msg("failed to record hosted trial redemption")
			http.Error(w, "failed to record trial redemption", http.StatusInternalServerError)
		}
		return
	}
	record, err := h.verificationStore.GetRecord(entitlement.TrialRequestID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTrialSignupRecordNotFound):
			http.Error(w, "invalid trial redemption", http.StatusBadRequest)
		default:
			log.Error().Err(err).Str("org_id", claims.OrgID).Msg("failed to load hosted trial redemption record")
			http.Error(w, "failed to load trial redemption", http.StatusInternalServerError)
		}
		return
	}
	trialStartedAt := now
	if !record.CheckoutCompletedAt.IsZero() {
		trialStartedAt = record.CheckoutCompletedAt.UTC()
	}
	redemption, err := h.entitlements.RedeemTrialEntitlement(entitlements.TrialEntitlementInput{
		RequestID:      entitlement.TrialRequestID,
		OrgID:          claims.OrgID,
		Email:          record.Email,
		ReturnURL:      claims.ReturnURL,
		InstanceToken:  record.InstanceToken,
		InstanceHost:   claims.InstanceHost,
		TrialStartedAt: trialStartedAt,
		IssuedAt:       now,
		RedeemedAt:     now,
	})
	if err != nil {
		log.Error().Err(err).Str("org_id", claims.OrgID).Msg("failed to redeem hosted trial entitlement")
		http.Error(w, "failed to generate entitlement lease", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(trialSignupRedeemResponse{
		EntitlementJWT:          redemption.EntitlementJWT,
		EntitlementRefreshToken: redemption.EntitlementRefreshToken,
	}); err != nil {
		log.Error().Err(err).Str("org_id", claims.OrgID).Msg("failed to encode hosted trial redemption response")
	}
}

func (h *TrialSignupHandlers) renderTrialSignupPage(w http.ResponseWriter, r *http.Request, status int, data trialSignupPageData) {
	data.Nonce = cpsec.NonceFromContext(r.Context())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := trialSignupPageTemplate.Execute(w, data); err != nil {
		log.Error().Err(err).Msg("trial signup page render failed")
	}
}

func (h *TrialSignupHandlers) renderTrialSignupSuccessPage(w http.ResponseWriter, r *http.Request, status int, data trialSignupSuccessData) {
	data.Nonce = cpsec.NonceFromContext(r.Context())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := trialSignupSuccessTemplate.Execute(w, data); err != nil {
		log.Error().Err(err).Msg("trial signup success page render failed")
	}
}

func (h *TrialSignupHandlers) HandleRateLimitedTrialSignup(w http.ResponseWriter, r *http.Request, retryAfter int) {
	data := h.trialSignupPageDataFromRequest(r)
	data.ErrorMessage = trialSignupRateLimitMessage(retryAfter)
	h.renderTrialSignupPage(w, r, http.StatusTooManyRequests, data)
}

func (h *TrialSignupHandlers) trialSignupPageDataFromRequest(r *http.Request) trialSignupPageData {
	data := trialSignupPageData{
		OrgID:         trialSignupDefaultOrgID,
		ReturnTarget:  "your Pulse instance",
		Cancelled:     strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("cancelled")), "1"),
		VerifiedToken: strings.TrimSpace(r.URL.Query().Get("verified")),
	}

	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		data.OrgID = normalizeTrialOrgID(q.Get("org_id"))
		data.ReturnURL = strings.TrimSpace(q.Get("return_url"))
		data.InstanceToken = strings.TrimSpace(q.Get("instance_token"))
		data.Name = strings.TrimSpace(q.Get("name"))
		data.Email = strings.TrimSpace(q.Get("email"))
		data.Company = strings.TrimSpace(q.Get("company"))
	default:
		if err := r.ParseForm(); err == nil {
			data.OrgID = normalizeTrialOrgID(r.FormValue("org_id"))
			data.ReturnURL = strings.TrimSpace(r.FormValue("return_url"))
			data.InstanceToken = strings.TrimSpace(r.FormValue("instance_token"))
			data.Name = strings.TrimSpace(r.FormValue("name"))
			data.Email = strings.TrimSpace(r.FormValue("email"))
			data.Company = strings.TrimSpace(r.FormValue("company"))
			if verifiedToken := strings.TrimSpace(r.FormValue("verified_token")); verifiedToken != "" {
				data.VerifiedToken = verifiedToken
			}
		}
	}

	if data.ReturnURL != "" {
		data.ReturnTarget = summarizeTrialReturnTarget(data.ReturnURL)
	}
	if data.VerifiedToken == "" {
		return data
	}

	record, err := h.lookupVerifiedTrialSignupRecord(data.VerifiedToken)
	if err != nil || record == nil {
		return data
	}

	data.OrgID = record.OrgID
	data.ReturnURL = record.ReturnURL
	data.InstanceToken = record.InstanceToken
	data.ReturnTarget = summarizeTrialReturnTarget(record.ReturnURL)
	data.Name = record.Name
	data.Email = record.Email
	data.Company = record.Company
	data.Verified = true
	return data
}

func (h *TrialSignupHandlers) renderTrialSignupFailurePage(w http.ResponseWriter, r *http.Request, status int, data trialSignupFailureData) {
	data.Nonce = cpsec.NonceFromContext(r.Context())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := trialSignupFailureTemplate.Execute(w, data); err != nil {
		log.Error().Err(err).Msg("trial signup failure page render failed")
	}
}

func trialSignupRateLimitMessage(retryAfter int) string {
	return "Too many trial setup attempts from this browser. " + humanizeTrialSignupRetryAfter(retryAfter) + "."
}

func humanizeTrialSignupRetryAfter(retryAfter int) string {
	if retryAfter <= 0 {
		return "Try again later"
	}
	if retryAfter < 90 {
		return "Try again in about a minute"
	}
	if retryAfter < 3600 {
		minutes := int((time.Duration(retryAfter)*time.Second + time.Minute - 1) / time.Minute)
		return "Try again in about " + strconv.Itoa(minutes) + " minutes"
	}
	hours := int((time.Duration(retryAfter)*time.Second + time.Hour - 1) / time.Hour)
	if hours == 1 {
		return "Try again in about 1 hour"
	}
	return "Try again in about " + strconv.Itoa(hours) + " hours"
}

func (h *TrialSignupHandlers) lookupVerifiedTrialSignupRecord(verifiedToken string) (*TrialSignupRecord, error) {
	if h.verificationStore == nil {
		return nil, ErrTrialSignupRecordNotFound
	}
	record, err := h.verificationStore.GetRecordByCheckoutToken(verifiedToken, h.now().UTC())
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
	_, err := pkglicensing.ValidateTrialActivationReturnURL(raw, "")
	return err == nil
}

func normalizeTrialOrgID(raw string) string {
	orgID := strings.TrimSpace(raw)
	if orgID == "" {
		return trialSignupDefaultOrgID
	}
	return orgID
}

func trialSignupFailureDataForPage(cfg *CPConfig, data trialSignupPageData, kind trialSignupFailureKind, message string) trialSignupFailureData {
	title := "Trial setup could not be completed"
	statusMessage := "Return to Pulse and start the secure trial handoff again if you still want to activate Pro on this instance."
	finePrint := "Pulse could not complete this secure trial handoff."

	switch kind {
	case trialSignupFailureConflict:
		title = "Trial already used"
		statusMessage = "This trial request cannot be restarted for the same recovery contact or organization."
		finePrint = "Upgrade the existing account or contact support if you need help reconciling prior trial usage."
	case trialSignupFailureInvalidLink:
		title = "Backup link expired"
		statusMessage = "This backup link can no longer continue the hosted trial handoff."
		finePrint = "Return to Pulse to request a fresh backup email or restart the secure trial setup."
	case trialSignupFailureUnavailable:
		title = "Trial setup is unavailable"
		statusMessage = "Pulse could not finish the secure trial handoff right now."
		finePrint = "Return to Pulse and try again later."
	}

	page := trialSignupFailureData{
		Title:         title,
		Message:       strings.TrimSpace(message),
		ReturnTarget:  strings.TrimSpace(data.ReturnTarget),
		StatusMessage: statusMessage,
		FinePrint:     finePrint,
	}
	if page.Message == "" {
		page.Message = "Pulse could not complete this secure trial handoff."
	}
	if page.ReturnTarget == "" {
		page.ReturnTarget = "your Pulse instance"
	}
	if kind != trialSignupFailureRetryable {
		return page
	}
	if strings.TrimSpace(data.ReturnURL) == "" || strings.TrimSpace(data.InstanceToken) == "" {
		return page
	}
	baseURL := ""
	if cfg != nil {
		baseURL = cfg.BaseURL
	}
	page.ActionURL = buildTrialSignupStartURL(baseURL, data, false)
	page.ActionLabel = "Start Trial Again"
	return page
}

func trialSignupIssuanceConflictMessage(conflict *TrialSignupIssuanceConflict) string {
	switch {
	case conflict == nil:
		return "This Pulse Pro trial request conflicts with a previous trial issuance. Upgrade the existing account or contact support if you need help."
	case conflict.Kind == trialSignupConflictEmail:
		return "This recovery email has already used a Pulse Pro trial. Upgrade the existing account or contact support if you need help."
	default:
		return "This organization has already used a Pulse Pro trial. Upgrade the existing account or contact support if you need help."
	}
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

func buildTrialSignupVerificationURL(baseURL, token string) string {
	query := url.Values{}
	if strings.TrimSpace(token) != "" {
		query.Set("token", strings.TrimSpace(token))
	}
	return buildCPURL(baseURL, "/trial-signup/verify", query)
}

func buildTrialSignupStartURL(baseURL string, data trialSignupPageData, cancelled bool) string {
	query := url.Values{
		"org_id":         {data.OrgID},
		"return_url":     {data.ReturnURL},
		"instance_token": {data.InstanceToken},
		"name":           {data.Name},
		"email":          {data.Email},
		"company":        {data.Company},
	}
	if cancelled {
		query.Set("cancelled", "1")
	}
	return buildCPURL(baseURL, "/start-pro-trial", query)
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

func buildTrialSignupCancelURL(baseURL string, data trialSignupPageData, verifiedToken string) string {
	if strings.TrimSpace(verifiedToken) != "" {
		return buildTrialSignupVerifiedURL(baseURL, verifiedToken, true)
	}
	return buildTrialSignupStartURL(baseURL, data, true)
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
