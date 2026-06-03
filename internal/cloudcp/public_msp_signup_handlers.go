package cloudcp

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpsec"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rs/zerolog/log"
)

const canonicalPublicMSPSignupPath = "/cloud/msp/signup"

// mspTier represents an MSP plan tier for public signup. The three tiers
// mirror the canonical MSP plan-version ladder in pkg/licensing (msp_starter /
// msp_growth / msp_scale), each capped at a different number of client
// workspaces.
type mspTier string

const (
	mspTierStarter mspTier = "starter"
	mspTierGrowth  mspTier = "growth"
	mspTierScale   mspTier = "scale"
)

var validMSPTiers = map[mspTier]bool{
	mspTierStarter: true,
	mspTierGrowth:  true,
	mspTierScale:   true,
}

// parseMSPTier normalizes a tier string from user input. Returns mspTierStarter
// if the input is empty (default). Returns ("", false) if the input is a
// non-empty but unrecognized tier.
func parseMSPTier(raw string) (mspTier, bool) {
	t := mspTier(strings.ToLower(strings.TrimSpace(raw)))
	if t == "" {
		return mspTierStarter, true
	}
	if validMSPTiers[t] {
		return t, true
	}
	return "", false
}

func expectedPlanVersionForMSPTier(tier mspTier) string {
	switch tier {
	case mspTierStarter:
		return "msp_starter"
	case mspTierGrowth:
		return "msp_growth"
	case mspTierScale:
		return "msp_scale"
	default:
		return ""
	}
}

// priceIDForMSPTier returns the configured Stripe price ID for the given MSP
// tier. Returns ("", false) if the tier's price ID is not configured.
func (h *PublicCloudSignupHandlers) priceIDForMSPTier(tier mspTier) (string, bool) {
	if h.cfg == nil {
		return "", false
	}
	switch tier {
	case mspTierStarter:
		id := strings.TrimSpace(h.cfg.CloudMSPStarterPriceID)
		return id, id != ""
	case mspTierGrowth:
		id := strings.TrimSpace(h.cfg.CloudMSPGrowthPriceID)
		return id, id != ""
	case mspTierScale:
		id := strings.TrimSpace(h.cfg.CloudMSPScalePriceID)
		return id, id != ""
	default:
		return "", false
	}
}

func isSelfServeMSPTier(tier mspTier) bool {
	return tier == mspTierStarter
}

func (h *PublicCloudSignupHandlers) selfServeMSPPriceIDForTier(tier mspTier) (string, bool) {
	if !isSelfServeMSPTier(tier) {
		return "", false
	}
	return h.priceIDForMSPTier(tier)
}

func validatePublicMSPSignupPriceID(tier mspTier, priceID string) error {
	wantPlanVersion := expectedPlanVersionForMSPTier(tier)
	if wantPlanVersion == "" {
		return fmt.Errorf("unsupported msp tier %q", tier)
	}
	if err := validateCloudStripePriceID("production", "", "public msp signup price", priceID, wantPlanVersion); err != nil {
		return err
	}
	return nil
}

var publicMSPSignupPageTemplate = template.Must(template.New("public-msp-signup-page").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Request Pulse MSP Access</title>
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
    .tier-group { display: flex; flex-direction: column; gap: 6px; margin-bottom: 4px; }
    .tier-option { display: flex; align-items: center; gap: 8px; font-size: 14px; font-weight: 400; cursor: pointer; padding: 8px 10px; border: 1px solid #e2e8f0; border-radius: 8px; }
    .tier-option:has(input:checked) { border-color: #1d4ed8; background: #eff6ff; }
    ol { margin: 0; padding-left: 20px; color: #334155; }
    li { margin-bottom: 8px; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <h1>Request Pulse MSP Access</h1>
      <p>Run Pulse for multiple clients from one provider account. The provider-hosted model runs a Stripe-free control plane and one isolated Pulse runtime per client workspace. MSP is an explicit provider path, so ordinary self-hosted Pulse stays out of it unless you choose this model. Starter, Growth, and Scale have published prices, but access is request-assisted while rollout, reporting, hosting, and support expectations are sized with you.</p>
      {{if .ErrorMessage}}<div class="error">{{.ErrorMessage}}</div>{{end}}
      {{if .Cancelled}}<div class="note">The previous request was cancelled. You can start again below.</div>{{end}}

      {{if .Available}}
      <form method="POST" action="{{.FormAction}}">
        <input type="hidden" name="tier" value="{{.Tier}}">
        <div class="tier-group">
          <div class="tier-option"><strong>Starter</strong>, up to 5 client workspaces, $149/mo, request-assisted access</div>
          <div class="tier-option"><strong>Growth</strong>, up to 15 client workspaces, $249/mo, request-assisted access</div>
          <div class="tier-option"><strong>Scale</strong>, up to 40 client workspaces, $399/mo, request-assisted access</div>
        </div>

        <label for="email">Work Email</label>
        <input id="email" name="email" type="email" value="{{.Email}}" autocomplete="email" required>

        <label for="org_name">Company Name</label>
        <input id="org_name" name="org_name" type="text" value="{{.OrgName}}" autocomplete="organization" required>

        <button class="cta" type="submit">Request Assisted Access</button>
      </form>

      <p class="fine">MSP is not open for public self-serve purchase yet. I will confirm the right tier, license-backed provider activation, and whether provider-hosted or Pulse-hosted operation fits before client data starts flowing. For Growth or Scale, email support@pulserelay.pro and include the tier you want and your expected client workspace count.</p>
      <ol>
        <li>Confirm the expected client workspace count and hosting model.</li>
        <li>Issue or verify the signed MSP license that sets the provider plan and client cap.</li>
        <li>Set up the provider account so you can add client workspaces and hand off into each isolated Pulse runtime.</li>
      </ol>
      {{else}}
      <div class="note">Pulse MSP is not open for public self-serve purchase yet. Email support@pulserelay.pro and I will get you set up.</div>
      {{end}}
    </div>
  </div>
</body>
</html>
`))

var publicMSPSignupCompleteTemplate = template.Must(template.New("public-msp-signup-complete").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Pulse MSP Access Request Complete</title>
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
      <h1>Request received</h1>
      <p>Your Pulse MSP access request has been received.</p>
      <p>Watch your inbox for the next setup step. MSP access is request-assisted, and setup confirms the signed provider license, client workspace cap, and hosting boundary before rollout.</p>
    </div>
  </div>
</body>
</html>
`))

type publicMSPSignupPageData struct {
	Email        string
	OrgName      string
	Tier         string // selected tier slug ("starter", "growth", "scale")
	FormAction   string
	ErrorMessage string
	Cancelled    bool
	Nonce        string
	Available    bool // true if the gated MSP request path is configured
}

// newMSPSignupPageData seeds page data from the current public MSP buying
// motion: all MSP plans are request-assisted while public launch is gated.
func (h *PublicCloudSignupHandlers) newMSPSignupPageData() publicMSPSignupPageData {
	_, starterConfigured := h.selfServeMSPPriceIDForTier(mspTierStarter)
	data := publicMSPSignupPageData{
		FormAction: canonicalPublicMSPSignupPath,
		Available:  starterConfigured,
		Tier:       string(mspTierStarter),
	}
	return data
}

func (h *PublicCloudSignupHandlers) renderMSPSignupPage(w http.ResponseWriter, r *http.Request, status int, data publicMSPSignupPageData) {
	data.Nonce = cpsec.NonceFromContext(r.Context())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := publicMSPSignupPageTemplate.Execute(w, data); err != nil {
		log.Error().Err(err).Msg("public msp signup page render failed")
	}
}

func (h *PublicCloudSignupHandlers) HandleMSPSignupPage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		data := h.newMSPSignupPageData()
		data.Email = strings.TrimSpace(r.URL.Query().Get("email"))
		data.OrgName = strings.TrimSpace(r.URL.Query().Get("org_name"))
		data.Cancelled = strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("cancelled")), "1")
		h.renderMSPSignupPage(w, r, http.StatusOK, data)
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form body", http.StatusBadRequest)
			return
		}
		email := strings.TrimSpace(r.FormValue("email"))
		orgName := strings.TrimSpace(r.FormValue("org_name"))
		tierStr := strings.TrimSpace(r.FormValue("tier"))

		renderErr := func(status int, msg string) {
			data := h.newMSPSignupPageData()
			data.Email = email
			data.OrgName = orgName
			data.ErrorMessage = msg
			h.renderMSPSignupPage(w, r, status, data)
		}

		tier, tierOK := parseMSPTier(tierStr)
		if !tierOK {
			renderErr(http.StatusBadRequest, "Invalid plan tier selected.")
			return
		}
		if !isValidCloudSignupEmail(email) {
			renderErr(http.StatusBadRequest, "A valid email address is required.")
			return
		}
		if !isValidCloudSignupOrgName(orgName) {
			renderErr(http.StatusBadRequest, "Company name must be 3-64 characters and cannot contain slashes.")
			return
		}
		if !isSelfServeMSPTier(tier) {
			renderErr(http.StatusBadRequest, "MSP Growth and Scale are request-assisted. Email support@pulserelay.pro with your client workspace count.")
			return
		}
		if _, avail := h.selfServeMSPPriceIDForTier(tier); !avail {
			renderErr(http.StatusBadRequest, "MSP Starter access is not currently available.")
			return
		}

		checkoutURL, err := h.createMSPCheckout(email, orgName, tier)
		if err != nil {
			log.Warn().Err(err).Str("email", email).Str("tier", string(tier)).Msg("public msp signup checkout creation failed")
			renderErr(http.StatusBadGateway, "Unable to start MSP access request. Please try again.")
			return
		}
		http.Redirect(w, r, checkoutURL, http.StatusSeeOther)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *PublicCloudSignupHandlers) HandleMSPSignupComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := publicMSPSignupCompleteTemplate.Execute(w, publicCloudSignupCompleteData{
		Nonce: cpsec.NonceFromContext(r.Context()),
	}); err != nil {
		log.Error().Err(err).Msg("public msp signup complete page render failed")
	}
}

func (h *PublicCloudSignupHandlers) HandleMSPPublicSignup(w http.ResponseWriter, r *http.Request) {
	h.servePublicSignupCheckout(w, r,
		"Invalid plan tier. Must be one of: starter, growth, scale",
		"public msp signup API checkout creation failed",
		"MSP access request created. Continue with the assisted setup instructions.",
		func(tierRaw string) (bool, bool, func(email, orgName string) (string, error)) {
			tier, ok := parseMSPTier(tierRaw)
			if !ok {
				return false, false, nil
			}
			_, available := h.selfServeMSPPriceIDForTier(tier)
			return true, available, func(email, orgName string) (string, error) {
				return h.createMSPCheckout(email, orgName, tier)
			}
		},
	)
}

func (h *PublicCloudSignupHandlers) createMSPCheckout(email, orgName string, tier mspTier) (string, error) {
	if h.cfg == nil {
		return "", fmt.Errorf("control plane config is missing")
	}
	priceID, ok := h.selfServeMSPPriceIDForTier(tier)
	if !ok || priceID == "" {
		return "", fmt.Errorf("checkout is not configured for msp tier %q", tier)
	}
	if err := validatePublicMSPSignupPriceID(tier, priceID); err != nil {
		return "", err
	}

	successURL := buildCPURL(h.cfg.BaseURL, canonicalPublicMSPSignupPath+"/complete", nil)
	cancelURL := buildCPURL(h.cfg.BaseURL, canonicalPublicMSPSignupPath, url.Values{
		"cancelled": {"1"},
		"email":     {email},
		"org_name":  {orgName},
		"tier":      {string(tier)},
	})
	return h.createImmediateCheckoutSession(email, priceID, successURL, cancelURL, h.buildMSPCheckoutMetadata(priceID, orgName))
}

func (h *PublicCloudSignupHandlers) buildMSPCheckoutMetadata(priceID, orgName string) map[string]string {
	meta := map[string]string{
		"account_kind":                 string(registry.AccountKindMSP),
		"account_display_name":         orgName,
		"display_name":                 orgName,
		"signup_source":                "public_msp_signup",
		checkoutBillingModeMetadataKey: checkoutBillingModeImmediate,
	}
	// Only accept msp_* plan versions on the MSP signup path. This is the
	// mirror of the individual path's cloud_* guard: it prevents granting
	// MSP-level workspace limits from a misconfigured non-MSP price.
	if plan, ok := pkglicensing.PlanVersionForPriceID(priceID); ok && strings.HasPrefix(plan, "msp_") {
		meta["plan_version"] = plan
	}
	return meta
}
