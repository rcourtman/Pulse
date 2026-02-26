package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestBillingStateGetReturnsDefaultWhenMissing(t *testing.T) {
	router, _ := newBillingStateTestRouter(t, true)

	rec := doBillingStateRequest(router, http.MethodGet, "/api/admin/orgs/acme/billing-state", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload entitlements.BillingState
	if err := decodeResponse(rec, &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.SubscriptionState != entitlements.SubStateTrial {
		t.Fatalf("expected subscription_state %q, got %q", entitlements.SubStateTrial, payload.SubscriptionState)
	}
	if payload.PlanVersion != string(entitlements.SubStateTrial) {
		t.Fatalf("expected plan_version %q, got %q", entitlements.SubStateTrial, payload.PlanVersion)
	}
	if len(payload.Capabilities) != 0 {
		t.Fatalf("expected empty capabilities, got %v", payload.Capabilities)
	}
	if len(payload.Limits) != 0 {
		t.Fatalf("expected empty limits, got %v", payload.Limits)
	}
	if len(payload.MetersEnabled) != 0 {
		t.Fatalf("expected empty meters_enabled, got %v", payload.MetersEnabled)
	}
}

func TestBillingStatePutGetRoundTrip(t *testing.T) {
	router, baseDir := newBillingStateTestRouter(t, true)

	putBody := `{
		"capabilities":["feature_x","feature_y"],
		"limits":{"max_agents":25,"max_guests":100},
		"meters_enabled":["api_requests"],
		"plan_version":"pro-v2",
		"subscription_state":"active"
	}`

	putRec := doBillingStateRequest(router, http.MethodPut, "/api/admin/orgs/acme/billing-state", putBody)
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", putRec.Code, putRec.Body.String())
	}

	var putPayload entitlements.BillingState
	if err := decodeResponse(putRec, &putPayload); err != nil {
		t.Fatalf("decode put response: %v", err)
	}
	if putPayload.SubscriptionState != entitlements.SubStateActive {
		t.Fatalf("expected subscription_state %q, got %q", entitlements.SubStateActive, putPayload.SubscriptionState)
	}

	billingFile := filepath.Join(baseDir, "orgs", "acme", "billing.json")
	if _, err := os.Stat(billingFile); err != nil {
		t.Fatalf("expected billing file to exist at %s: %v", billingFile, err)
	}

	getRec := doBillingStateRequest(router, http.MethodGet, "/api/admin/orgs/acme/billing-state", "")
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var getPayload entitlements.BillingState
	if err := decodeResponse(getRec, &getPayload); err != nil {
		t.Fatalf("decode get response: %v", err)
	}

	if !reflect.DeepEqual(getPayload, putPayload) {
		t.Fatalf("expected persisted payload %+v, got %+v", putPayload, getPayload)
	}
}

func TestBillingStatePutAuditLogEmitted(t *testing.T) {
	baseDir := t.TempDir()
	store := config.NewFileBillingStore(baseDir)
	handlers := NewBillingStateHandlers(store, true)

	if err := store.SaveBillingState("acme", &entitlements.BillingState{
		Capabilities:      []string{},
		Limits:            map[string]int64{},
		MetersEnabled:     []string{},
		PlanVersion:       string(entitlements.SubStateTrial),
		SubscriptionState: entitlements.SubStateTrial,
	}); err != nil {
		t.Fatalf("seed billing state: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/admin/orgs/acme/billing-state", strings.NewReader(`{
		"capabilities":[],
		"limits":{},
		"meters_enabled":[],
		"plan_version":"active",
		"subscription_state":"active"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "acme")

	rec := httptest.NewRecorder()
	handlers.HandlePutBillingState(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	var payload entitlements.BillingState
	if err := decodeResponse(rec, &payload); err != nil {
		t.Fatalf("decode put response: %v", err)
	}
	if payload.SubscriptionState != entitlements.SubStateActive {
		t.Fatalf("expected subscription_state %q, got %q", entitlements.SubStateActive, payload.SubscriptionState)
	}
	if !strings.Contains(body, `"subscription_state":"active"`) {
		t.Fatalf("expected response body to include active subscription_state, got %s", body)
	}
}

func TestBillingStatePutRejectsInvalidSubscriptionState(t *testing.T) {
	router, _ := newBillingStateTestRouter(t, true)

	rec := doBillingStateRequest(router, http.MethodPut, "/api/admin/orgs/acme/billing-state", `{
		"capabilities":["feature_x"],
		"limits":{"max_agents":10},
		"meters_enabled":["api_requests"],
		"plan_version":"pro-v1",
		"subscription_state":"not-a-real-state"
	}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var apiErr APIError
	if err := decodeResponse(rec, &apiErr); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if apiErr.Code != "invalid_subscription_state" {
		t.Fatalf("expected error code invalid_subscription_state, got %q", apiErr.Code)
	}
}

func TestBillingStateHostedModeGate(t *testing.T) {
	router, _ := newBillingStateTestRouter(t, false)

	testCases := []struct {
		method string
		body   string
	}{
		{
			method: http.MethodGet,
			body:   "",
		},
		{
			method: http.MethodPut,
			body: `{
				"capabilities":["feature_x"],
				"limits":{"max_agents":10},
				"meters_enabled":["api_requests"],
				"plan_version":"pro-v1",
				"subscription_state":"active"
			}`,
		},
	}

	for _, tc := range testCases {
		rec := doBillingStateRequest(router, tc.method, "/api/admin/orgs/acme/billing-state", tc.body)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 when hosted mode is disabled for %s, got %d: %s", tc.method, rec.Code, rec.Body.String())
		}
	}
}

func newBillingStateTestRouter(t *testing.T, hostedMode bool) (*Router, string) {
	t.Helper()

	baseDir := t.TempDir()
	hashed, err := internalauth.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	router := &Router{
		mux:         http.NewServeMux(),
		config:      &config.Config{DataPath: baseDir, AuthUser: "admin", AuthPass: hashed},
		multiTenant: config.NewMultiTenantPersistence(baseDir),
		hostedMode:  hostedMode,
	}
	router.registerHostedRoutes(nil, nil, nil)
	t.Cleanup(func() {
		if router.signupRateLimiter != nil {
			router.signupRateLimiter.Stop()
		}
	})

	return router, baseDir
}

func doBillingStateRequest(router *Router, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if method == http.MethodPut {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:Password!1")))
	rec := httptest.NewRecorder()
	router.mux.ServeHTTP(rec, req)
	return rec
}

func decodeResponse[T any](rec *httptest.ResponseRecorder, out *T) error {
	return json.NewDecoder(rec.Body).Decode(out)
}
