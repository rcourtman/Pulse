package cloudcp

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// providerMSPTestIssuer signs provider MSP licences with one licence-server
// key, so a test can hold an evaluation and a renewed licence at once.
type providerMSPTestIssuer struct {
	private ed25519.PrivateKey
}

func newProviderMSPTestIssuer(t *testing.T) *providerMSPTestIssuer {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv("PULSE_LICENSE_PUBLIC_KEY", base64.StdEncoding.EncodeToString(public))
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")
	t.Cleanup(func() { pkglicensing.SetPublicKey(nil) })
	return &providerMSPTestIssuer{private: private}
}

func (i *providerMSPTestIssuer) sign(t *testing.T, licenseID, planVersion string, expiresAt time.Time, binding ed25519.PublicKey) string {
	t.Helper()
	claims := pkglicensing.Claims{
		LicenseID:                   licenseID,
		Email:                       "provider@example.com",
		Tier:                        pkglicensing.TierMSP,
		IssuedAt:                    time.Now().Add(-time.Minute).Unix(),
		ExpiresAt:                   expiresAt.Unix(),
		PlanVersion:                 planVersion,
		EntitlementSigningPublicKey: base64.StdEncoding.EncodeToString(binding),
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("Marshal claims: %v", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signature := base64.RawURLEncoding.EncodeToString(ed25519.Sign(i.private, []byte(header+"."+payload)))
	return header + "." + payload + "." + signature
}

func writeProviderMSPTestFile(t *testing.T, path, contents string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

type fakeProviderMSPLicenseServer struct {
	t *testing.T

	mu             sync.Mutex
	currentStatus  int
	currentLicense string
	checkoutStatus int
	portalStatus   int
	plans          string
	lastCheckout   map[string]string
	signedActions  []string
}

func (f *fakeProviderMSPLicenseServer) verify(r *http.Request, action string) map[string]any {
	f.t.Helper()
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		f.t.Fatalf("decode signed body: %v", err)
	}
	key, _ := base64.StdEncoding.DecodeString(body["entitlement_signing_public_key"].(string))
	sig, _ := base64.StdEncoding.DecodeString(body["signature"].(string))
	ts := int64(body["timestamp"].(float64))
	message := "pulse-provider-msp-v1\n" + action + "\n" + body["entitlement_signing_public_key"].(string) + "\n" + strconv.FormatInt(ts, 10)
	if !ed25519.Verify(ed25519.PublicKey(key), []byte(message), sig) {
		f.t.Fatalf("%s request is not signed by the lease signing key", action)
	}
	f.signedActions = append(f.signedActions, action)
	return body
}

func (f *fakeProviderMSPLicenseServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/v1/provider-msp/license/current":
		f.verify(r, "license-current")
		w.WriteHeader(f.currentStatus)
		if f.currentStatus == http.StatusOK {
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "active", "license": f.currentLicense, "paid_through": "2026-10-23T00:00:00Z"})
		} else {
			_, _ = w.Write([]byte(`{"status":"no_paid_subscription"}`))
		}
	case "/v1/provider-msp/billing-portal":
		body := f.verify(r, "billing-portal")
		if body["return_url"] != "https://msp.example.com/portal?provider_msp_checkout=billing" {
			f.t.Fatalf("billing portal return_url = %v, want the portal flagged to apply a plan change", body["return_url"])
		}
		w.WriteHeader(f.portalStatus)
		_, _ = w.Write([]byte(`{"url":"https://billing.stripe.com/p/session/test"}`))
	case "/v1/provider-msp/checkout":
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &f.lastCheckout)
		w.WriteHeader(f.checkoutStatus)
		_, _ = w.Write([]byte(`{"url":"https://checkout.stripe.com/c/pay/cs_test"}`))
	case "/v1/provider-msp/plans":
		_, _ = w.Write([]byte(f.plans))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newProviderMSPRefreshTestConfig(t *testing.T, serverURL string) *CPConfig {
	t.Helper()
	setTrialSigningEnv(t)
	publicKey := base64.StdEncoding.EncodeToString(trialSigningEnvPublicKey(t))
	return &CPConfig{
		DataDir:                          t.TempDir(),
		ControlPlaneMode:                 ControlPlaneModeProviderHostedMSP,
		BaseURL:                          "https://msp.example.com",
		LicenseServerURL:                 serverURL,
		TrialActivationPrivateKey:        "A8medgdNdm12GXfTXWo6+TMZ2BeHPCLg2kd0znn6ZUk=",
		TrialActivationPublicKey:         publicKey,
		ProviderMSPPlanVersion:           pkglicensing.PlanVersionMSPEval,
		ProviderMSPPlanSource:            ProviderMSPPlanSourceLicenseFile,
		ProviderMSPLicenseID:             "lic_msp_eval",
		ProviderMSPLeaseSigningPublicKey: publicKey,
		ProviderMSPLicenseExpiresAt:      time.Now().Add(30 * 24 * time.Hour).Truncate(time.Second),
	}
}

func TestProviderMSPLicenseRefreshInstallsThePaidLicenceAndRestarts(t *testing.T) {
	issuer := newProviderMSPTestIssuer(t)
	fake := &fakeProviderMSPLicenseServer{t: t, currentStatus: http.StatusNotFound}
	server := httptest.NewServer(fake)
	defer server.Close()
	cfg := newProviderMSPRefreshTestConfig(t, server.URL)
	refresher := NewProviderMSPLicenseRefresher(cfg)
	restarted := make(chan struct{}, 1)
	refresher.SetRestart(func() { restarted <- struct{}{} })

	// No paid subscription: nothing installed, nothing restarted.
	result, err := refresher.Refresh(context.Background())
	if err != nil || result.Status != "no_paid_subscription" || result.Changed {
		t.Fatalf("unpaid refresh = %+v, %v", result, err)
	}
	if _, err := os.Stat(ProviderMSPRenewedLicensePath(cfg.DataDir)); !os.IsNotExist(err) {
		t.Fatalf("unpaid refresh wrote a licence: %v", err)
	}

	// A licence bound to another platform's key is refused and not installed.
	otherKey, _, _ := ed25519.GenerateKey(rand.Reader)
	fake.currentStatus = http.StatusOK
	fake.currentLicense = issuer.sign(t, "lic_msp_paid", "msp_solo", time.Now().Add(45*24*time.Hour), otherKey)
	if _, err := refresher.Refresh(context.Background()); err == nil || !strings.Contains(err.Error(), "another platform") {
		t.Fatalf("foreign-key licence accepted: %v", err)
	}
	if _, err := os.Stat(ProviderMSPRenewedLicensePath(cfg.DataDir)); !os.IsNotExist(err) {
		t.Fatalf("foreign-key licence was installed: %v", err)
	}

	// The paid licence for this key is installed and applied by a restart.
	fake.currentLicense = issuer.sign(t, "lic_msp_paid", "msp_solo", time.Now().Add(45*24*time.Hour), trialSigningEnvPublicKey(t))
	result, err = refresher.Refresh(context.Background())
	if err != nil || result.Status != "active" || !result.Changed || !result.RestartScheduled || result.PlanVersion != "msp_solo" {
		t.Fatalf("paid refresh = %+v, %v", result, err)
	}
	installed, err := os.ReadFile(ProviderMSPRenewedLicensePath(cfg.DataDir))
	if err != nil || strings.TrimSpace(string(installed)) != fake.currentLicense {
		t.Fatalf("installed licence mismatch: %v", err)
	}
	if info, _ := os.Stat(ProviderMSPRenewedLicensePath(cfg.DataDir)); info.Mode().Perm() != 0o600 {
		t.Fatalf("installed licence mode = %v", info.Mode().Perm())
	}
	select {
	case <-restarted:
	case <-time.After(5 * time.Second):
		t.Fatal("a changed licence did not restart the control plane")
	}

	// Running on that licence, the same answer is not a change.
	cfg.ProviderMSPLicenseID = "lic_msp_paid"
	cfg.ProviderMSPPlanVersion = "msp_solo"
	resolved, _ := resolveProviderMSPPlanFromLicenseFile(ProviderMSPRenewedLicensePath(cfg.DataDir))
	cfg.ProviderMSPLicenseExpiresAt = resolved.ExpiresAt
	if result, err := refresher.Refresh(context.Background()); err != nil || result.Changed || result.RestartScheduled {
		t.Fatalf("unchanged refresh = %+v, %v", result, err)
	}
}

func TestProviderMSPLicenseRefreshAppliesShorterPaidPeriod(t *testing.T) {
	issuer := newProviderMSPTestIssuer(t)
	fake := &fakeProviderMSPLicenseServer{t: t, currentStatus: http.StatusOK}
	server := httptest.NewServer(fake)
	defer server.Close()
	cfg := newProviderMSPRefreshTestConfig(t, server.URL)
	cfg.ProviderMSPLicenseID = "lic_msp_paid"
	cfg.ProviderMSPPlanVersion = "msp_solo"
	cfg.ProviderMSPLicenseExpiresAt = time.Now().Add(90 * 24 * time.Hour).Truncate(time.Second)

	// A billing-cycle change can shorten the paid period without changing the
	// licence ID or plan. Retaining the old expiry would over-entitle it.
	shorterExpiry := time.Now().Add(30 * 24 * time.Hour).Truncate(time.Second)
	fake.currentLicense = issuer.sign(t, cfg.ProviderMSPLicenseID, cfg.ProviderMSPPlanVersion, shorterExpiry, trialSigningEnvPublicKey(t))
	refresher := NewProviderMSPLicenseRefresher(cfg)
	restarted := make(chan struct{}, 1)
	refresher.SetRestart(func() { restarted <- struct{}{} })

	result, err := refresher.Refresh(context.Background())
	if err != nil || !result.Changed || !result.RestartScheduled || result.ExpiresAt != shorterExpiry.UTC().Format(time.RFC3339) {
		t.Fatalf("shorter paid-period refresh = %+v, %v", result, err)
	}
	installed, err := os.ReadFile(ProviderMSPRenewedLicensePath(cfg.DataDir))
	if err != nil || strings.TrimSpace(string(installed)) != fake.currentLicense {
		t.Fatalf("shorter paid-period licence not installed: %v", err)
	}
	select {
	case <-restarted:
	case <-time.After(5 * time.Second):
		t.Fatal("shorter paid-period licence did not restart the control plane")
	}
}

func TestProviderMSPPortalRoutesRelayToTheLicenceServer(t *testing.T) {
	fake := &fakeProviderMSPLicenseServer{
		t:              t,
		currentStatus:  http.StatusNotFound,
		checkoutStatus: http.StatusOK,
		portalStatus:   http.StatusOK,
		plans:          `{"plans":[{"plan_version":"msp_solo","billing_cycle":"monthly","unit_amount":6900,"currency":"usd"},{"plan_version":"msp_mystery","billing_cycle":"monthly","unit_amount":1,"currency":"usd"},{"plan_version":"msp_scale","billing_cycle":"annual","unit_amount":399000,"currency":"usd"}]}`,
	}
	server := httptest.NewServer(fake)
	defer server.Close()
	cfg := newProviderMSPRefreshTestConfig(t, server.URL)
	refresher := NewProviderMSPLicenseRefresher(cfg)

	rec := httptest.NewRecorder()
	HandleProviderMSPPlan(cfg, refresher)(rec, httptest.NewRequest(http.MethodGet, "/api/accounts/a_1/provider-msp/plan", nil))
	var state ProviderMSPPlanState
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatalf("decode plan state: %v", err)
	}
	if !state.Evaluation || state.WorkspaceLimit != 2 || !state.PurchaseAvailable || state.LicenseID != "lic_msp_eval" {
		t.Fatalf("plan state = %+v", state)
	}
	if len(state.Plans) != 2 || state.Plans[0].PlanVersion != "msp_solo" || state.Plans[0].WorkspaceLimit != 3 || state.Plans[1].WorkspaceLimit != 40 {
		t.Fatalf("offered plans = %+v (an unenforceable plan must be dropped)", state.Plans)
	}

	rec = httptest.NewRecorder()
	HandleProviderMSPCheckout(refresher)(rec, httptest.NewRequest(http.MethodPost, "/api/accounts/a_1/provider-msp/checkout", strings.NewReader(`{"plan_version":"msp_solo","billing_cycle":"monthly"}`)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "checkout.stripe.com") {
		t.Fatalf("checkout status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.lastCheckout["license_id"] != "lic_msp_eval" || fake.lastCheckout["return_url"] != "https://msp.example.com/portal" {
		t.Fatalf("checkout request = %v", fake.lastCheckout)
	}

	fake.checkoutStatus = http.StatusConflict
	rec = httptest.NewRecorder()
	HandleProviderMSPCheckout(refresher)(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"plan_version":"msp_solo","billing_cycle":"monthly"}`)))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "already_subscribed") {
		t.Fatalf("second checkout status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	HandleProviderMSPBillingPortal(refresher)(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "billing.stripe.com") {
		t.Fatalf("billing portal status=%d body=%s", rec.Code, rec.Body.String())
	}
	fake.portalStatus = http.StatusNotFound
	rec = httptest.NewRecorder()
	HandleProviderMSPBillingPortal(refresher)(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "no_paid_subscription") {
		t.Fatalf("billing portal without a subscription status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	HandleProviderMSPLicenseRefresh(refresher)(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "no_paid_subscription") {
		t.Fatalf("refresh status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Join(fake.signedActions, ",") != "billing-portal,billing-portal,license-current" {
		t.Fatalf("signed actions = %v", fake.signedActions)
	}

	// Without a licence server the portal still reports the plan but offers
	// no purchase, and the actions answer unavailable instead of failing open.
	rec = httptest.NewRecorder()
	HandleProviderMSPPlan(cfg, nil)(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	_ = json.Unmarshal(rec.Body.Bytes(), &state)
	if state.PurchaseAvailable || len(state.Plans) != 0 {
		t.Fatalf("plan state without refresher = %+v", state)
	}
	rec = httptest.NewRecorder()
	HandleProviderMSPCheckout(nil)(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("checkout without refresher status=%d", rec.Code)
	}
}
