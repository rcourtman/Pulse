package stripe

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"errors"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/entitlements"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	stripewebhook "github.com/stripe/stripe-go/v82/webhook"
)

// checkoutRaceFixture exercises the real signed handler. Its Docker API is a
// loopback fixture, not a daemon: start is the deterministic post-lease boundary.
type checkoutRaceFixture struct {
	reg           *registry.TenantRegistry
	p             *Provisioner
	handler       *WebhookHandler
	session       CheckoutSession
	dir           string
	beforeRuntime func()
	runtimeCalls  []string
	runtimeMu     sync.Mutex
}

func newCheckoutRaceFixture(t *testing.T, runtime bool) *checkoutRaceFixture {
	t.Helper()
	reg := newStripeTestRegistry(t)
	f := &checkoutRaceFixture{reg: reg, dir: t.TempDir(), session: CheckoutSession{ID: "cs_checkout", Customer: "cus_checkout", Subscription: "sub_checkout", CustomerEmail: "owner@example.invalid", Metadata: map[string]string{"plan_version": "cloud_starter"}}}
	if err := reg.CreateAccount(&registry.Account{ID: "a-checkout", Kind: registry.AccountKindIndividual, DisplayName: "Checkout fixture"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateStripeAccount(&registry.StripeAccount{AccountID: "a-checkout", StripeCustomerID: f.session.Customer, StripeSubscriptionID: f.session.Subscription, SubscriptionState: "trial", PlanVersion: "cloud_starter"}); err != nil {
		t.Fatal(err)
	}
	var mgr *docker.Manager
	if runtime {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f.runtimeMu.Lock()
			f.runtimeCalls = append(f.runtimeCalls, r.Method+" "+r.URL.Path)
			f.runtimeMu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			path := r.URL.Path
			switch {
			case strings.HasSuffix(path, "/_ping"):
				w.Header().Set("API-Version", "1.52")
				fmt.Fprint(w, "OK")
			case strings.Contains(path, "/images/") && strings.HasSuffix(path, "/json"):
				fmt.Fprint(w, `{"Id":"sha256:fixture","Config":{"Labels":{}}}`)
			case strings.Contains(path, "/networks/"):
				fmt.Fprint(w, `{"Id":"fixture-network","IPAM":{"Config":[{"Subnet":"172.30.0.0/24"}]}}`)
			case strings.HasSuffix(path, "/containers/create"):
				w.WriteHeader(http.StatusCreated)
				fmt.Fprint(w, `{"Id":"fixture-container-000001"}`)
			case strings.HasSuffix(path, "/start"):
				if f.beforeRuntime != nil {
					f.beforeRuntime()
				}
				w.WriteHeader(http.StatusNoContent)
			case strings.HasSuffix(path, "/stop") || r.Method == http.MethodDelete:
				w.WriteHeader(http.StatusNoContent)
			case strings.Contains(path, "/containers/") && strings.HasSuffix(path, "/json"):
				fmt.Fprint(w, `{"Id":"fixture-container-000001","Config":{"Labels":{}},"State":{"Running":true,"Health":{"Status":"healthy"}}}`)
			default:
				t.Errorf("unexpected fixture Docker request: %s %s", r.Method, path)
				http.Error(w, "unmapped fixture request", 500)
			}
		}))
		t.Cleanup(server.Close)
		t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(server.URL, "http://"))
		t.Setenv("DOCKER_TLS_VERIFY", "")
		t.Setenv("DOCKER_CERT_PATH", "")
		var err error
		mgr, err = docker.NewManager(docker.ManagerConfig{Image: "pulse:fixture", Network: "bridge", BaseDomain: "cloud.example.com", TenantRuntimeUID: os.Getuid(), TenantRuntimeGID: os.Getgid()})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = mgr.Close() })
	}
	f.p = newTestProvisioner(t, reg, f.dir, mgr, !runtime)
	f.handler = NewWebhookHandler("whsec_checkout_fixture", f.p)
	return f
}

func (f *checkoutRaceFixture) event(t *testing.T, id, kind string, object any) int {
	t.Helper()
	body, err := json.Marshal(map[string]any{"id": id, "object": "event", "type": kind, "data": map[string]any{"object": object}})
	if err != nil {
		t.Fatal(err)
	}
	signed := stripewebhook.GenerateTestSignedPayload(&stripewebhook.UnsignedPayload{Payload: body, Secret: "whsec_checkout_fixture"})
	r := httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", bytes.NewReader(body))
	r.Header.Set("Stripe-Signature", signed.Header)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w.Code
}
func (f *checkoutRaceFixture) withdraw(t *testing.T, status string) int {
	kind := "customer.subscription.updated"
	if status == "canceled" {
		kind = "customer.subscription.deleted"
	}
	return f.event(t, "evt_withdraw", kind, Subscription{ID: f.session.Subscription, Customer: f.session.Customer, Status: status, Metadata: f.session.Metadata})
}
func (f *checkoutRaceFixture) schedule(t *testing.T, window, status string, onWithdrawal func(int)) {
	t.Helper()
	action := func() { onWithdrawal(f.withdraw(t, status)) }
	if window == "runtime_start" {
		f.beforeRuntime = action
		return
	}
	fired := false
	f.p.chownFile = func(path string, _, _ int) error {
		if !fired && filepath.Base(path) == "handoff.key" {
			fired = true
			action()
		}
		return nil
	}
}

func TestCheckoutWithdrawalCannotBeForgotten(t *testing.T) {
	for _, window := range []string{"before_tenant_insert", "runtime_start"} {
		for _, status := range []string{"canceled", "paused"} {
			t.Run(window+"/"+status, func(t *testing.T) {
				f := newCheckoutRaceFixture(t, window == "runtime_start")
				seen := 0
				var issued *registry.HostedEntitlement
				if window == "runtime_start" {
					f.beforeRuntime = func() {
						rows, err := f.reg.ListHostedEntitlements()
						if err != nil || len(rows) != 1 {
							t.Errorf("expected initial lease before runtime start: %v", err)
							return
						}
						issued = rows[0]
						seen = f.withdraw(t, status)
					}
				} else {
					f.schedule(t, window, status, func(code int) { seen = code })
				}
				if code := f.event(t, "evt_checkout", "checkout.session.completed", f.session); code != 500 {
					t.Fatalf("withdrawn checkout status%d, want500", code)
				}
				want := 200
				if window == "runtime_start" {
					want = 500
				}
				if seen != want {
					t.Fatalf("withdrawal status%d, want%d (authority must not span runtime start)", seen, want)
				}
				sa, err := f.reg.GetStripeAccount("a-checkout")
				if err != nil {
					t.Fatal(err)
				}
				state := "canceled"
				if status == "paused" {
					state = "suspended"
				}
				if sa.SubscriptionState != state {
					t.Fatalf("withdrawal was lost: %+v", sa)
				}
				// This checkout only created new artifacts. Owned successful rollback may
				// remove them, but never the saved account or its withdrawal.
				tenant, err := f.reg.GetByStripeCustomerID(f.session.Customer)
				if err != nil || tenant != nil {
					t.Fatalf("new partial tenant was not rolled back: %+v/%v", tenant, err)
				}
				entries, err := os.ReadDir(f.dir)
				if err != nil || len(entries) != 0 {
					t.Fatalf("new partial files not rolled back: %v/%v", entries, err)
				}
				rows, err := f.reg.ListHostedEntitlements()
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) != 0 {
					t.Fatal("new partial checkout retained an entitlement after owned rollback")
				}
				if window == "runtime_start" {
					if issued == nil {
						t.Fatal("missing initial lease witness")
					}
					result, err := f.p.hostedEntitlements.RefreshEntitlement(issued.RefreshToken, issued.TenantID+".cloud.example.com")
					if result != nil || !errors.Is(err, entitlements.ErrHostedEntitlementNotFound) {
						t.Fatalf("rolled-back checkout token was not refused: %v", err)
					}
				}
				if window == "runtime_start" {
					f.runtimeMu.Lock()
					calls := strings.Join(f.runtimeCalls, "\n")
					f.runtimeMu.Unlock()
					if !strings.Contains(calls, "/stop") || !strings.Contains(calls, "DELETE ") {
						t.Fatalf("new runtime rollback not exercised: %s", calls)
					}
				} else if len(rows) != 0 {
					t.Fatal("pre-insert withdrawal issued an entitlement")
				}
				// The exact same provider events cannot reconstruct authority on retry.
				if code := f.withdraw(t, status); code != 200 {
					t.Fatalf("withdrawal retry status%d", code)
				}
				if code := f.event(t, "evt_checkout", "checkout.session.completed", f.session); code != 500 {
					t.Fatalf("checkout redelivery recreated authority: status%d", code)
				}
				if sa, err = f.reg.GetStripeAccount("a-checkout"); err != nil || sa.SubscriptionState != state {
					t.Fatalf("retry lost withdrawal: %+v/%v", sa, err)
				}
				// Both exclusion domains are released, not left held by a failed checkout.
				_, lease, err := f.reg.AcquireBillingAuthority(context.Background(), f.session.Customer)
				if err != nil {
					t.Fatal(err)
				}
				_ = lease.Close()
			})
		}
	}
}

func TestCheckoutCompletionPreservesCurrentTenantFields(t *testing.T) {
	f := newCheckoutRaceFixture(t, true)
	f.beforeRuntime = func() {
		current, err := f.reg.GetByStripeCustomerID(f.session.Customer)
		if err != nil || current == nil {
			t.Errorf("runtime fixture tenant: %v", err)
			return
		}
		current.DisplayName = "Saved client label"
		// Account-owned metadata changes while the provider is starting a runtime.
		if err := f.reg.Update(current); err != nil {
			t.Error(err)
		}
	}
	if code := f.event(t, "evt_checkout", "checkout.session.completed", f.session); code != 200 {
		t.Fatalf("active checkout status%d", code)
	}
	tenant, err := f.reg.GetByStripeCustomerID(f.session.Customer)
	if err != nil || tenant == nil {
		t.Fatal(err)
	}
	if tenant.DisplayName != "Saved client label" || tenant.State != registry.TenantStateActive || tenant.ContainerID != "fixture-container-000001" {
		t.Fatalf("stale checkout completion replaced current fields: %+v", tenant)
	}
	rows, err := f.reg.ListHostedEntitlements()
	if err != nil || len(rows) != 1 {
		t.Fatalf("initial entitlement: %v", err)
	}
	result, err := f.p.hostedEntitlements.RefreshEntitlement(rows[0].RefreshToken, tenant.ID+".cloud.example.com")
	if err != nil || result == nil {
		t.Fatalf("active checkout cannot renew: %v", err)
	}
	pub, err := base64.StdEncoding.DecodeString(os.Getenv("PULSE_TRIAL_ACTIVATION_PUBLIC_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pkglicensing.VerifyEntitlementLeaseToken(result.EntitlementJWT, pub, tenant.ID+".cloud.example.com", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if code := f.event(t, "evt_checkout", "checkout.session.completed", f.session); code != 200 {
		t.Fatalf("active duplicate status%d", code)
	}
}

func TestCheckoutRefusesWithdrawnBillingBeforeMetadataBackfill(t *testing.T) {
	f := newCheckoutRaceFixture(t, false)
	sa, err := f.reg.GetStripeAccount("a-checkout")
	if err != nil {
		t.Fatal(err)
	}
	sa.StripeSubscriptionID = ""
	sa.PlanVersion = ""
	if err := f.reg.UpdateStripeAccount(sa); err != nil {
		t.Fatal(err)
	}
	f.p.admissionCheck = func(context.Context) error {
		return f.p.HandleSubscriptionDeleted(context.Background(), Subscription{ID: "sub_checkout", Customer: f.session.Customer})
	}
	if code := f.event(t, "evt_checkout", "checkout.session.completed", f.session); code != 500 {
		t.Fatalf("stale mapping restored authority: status%d", code)
	}
	sa, err = f.reg.GetStripeAccount("a-checkout")
	if err != nil || sa.SubscriptionState != "canceled" {
		t.Fatalf("stale backfill replaced withdrawal: %+v/%v", sa, err)
	}
	entries, err := os.ReadDir(f.dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("denied checkout wrote files: %v/%v", entries, err)
	}
}

func TestCheckoutCanRecoverOnlyAfterMatchingBillingGrant(t *testing.T) {
	for _, status := range []string{"canceled", "paused"} {
		t.Run(status, func(t *testing.T) {
			f := newCheckoutRaceFixture(t, false)
			f.schedule(t, "before_tenant_insert", status, func(code int) {
				if code != 200 {
					t.Fatalf("withdrawal status%d", code)
				}
			})
			if code := f.event(t, "evt_checkout", "checkout.session.completed", f.session); code != 500 {
				t.Fatalf("withdrawn checkout status%d", code)
			}
			original := f.session
			sub := Subscription{ID: "sub_recovered", Customer: f.session.Customer, Status: "active", Metadata: f.session.Metadata}
			if code := f.event(t, "evt_recover_billing", "customer.subscription.updated", sub); code != 200 {
				t.Fatalf("grant status%d", code)
			}
			sa, err := f.reg.GetStripeAccount("a-checkout")
			if err != nil || sa.SubscriptionState != "active" || sa.StripeSubscriptionID != sub.ID {
				t.Fatalf("grant lost before tenant: %+v/%v", sa, err)
			}
			if code := f.event(t, "evt_stale_checkout", "checkout.session.completed", original); code != 500 {
				t.Fatalf("old subscription checkout overrode new billing: status%d", code)
			}
			f.session.Subscription = sub.ID
			if code := f.event(t, "evt_recovered_checkout", "checkout.session.completed", f.session); code != 200 {
				t.Fatalf("matching checkout could not recover: status%d", code)
			}
			tenant, err := f.reg.GetByStripeCustomerID(f.session.Customer)
			if err != nil || tenant == nil || tenant.State != registry.TenantStateActive {
				t.Fatalf("recovered tenant: %+v/%v", tenant, err)
			}
			rows, err := f.reg.ListHostedEntitlements()
			if err != nil || len(rows) != 1 {
				t.Fatalf("recovered entitlement: %v", err)
			}
			result, err := f.p.hostedEntitlements.RefreshEntitlement(rows[0].RefreshToken, tenant.ID+".cloud.example.com")
			if err != nil || result == nil {
				t.Fatalf("matching grant cannot renew: %v", err)
			}
		})
	}
	t.Run("missing_billing_is_not_recreated", func(t *testing.T) {
		reg := newStripeTestRegistry(t)
		p := newTestProvisioner(t, reg, t.TempDir(), nil, true)
		if err := p.HandleSubscriptionUpdated(context.Background(), Subscription{ID: "sub_unknown", Customer: "cus_unknown", Status: "active", Metadata: map[string]string{"plan_version": "cloud_starter"}}); err != nil {
			t.Fatal(err)
		}
		rows, err := reg.ListStripeAccounts()
		if err != nil || len(rows) != 0 {
			t.Fatalf("grant manufactured missing billing: %+v/%v", rows, err)
		}
	})
}

func TestUnprovisionedCheckoutGrantCannotBypassRegisteredWorkspace(t *testing.T) {
	f := newCheckoutRaceFixture(t, false)
	tenant := &registry.Tenant{ID: "t-registered", AccountID: "a-checkout", State: registry.TenantStateActive, PlanVersion: "cloud_starter"}
	if err := f.reg.Create(tenant); err != nil {
		t.Fatal(err)
	}
	billing, err := f.p.hostedEntitlements.IssueTenantBillingState(tenant, "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	host := tenant.ID + ".cloud.example.com"
	if result, err := f.p.hostedEntitlements.RefreshEntitlement(billing.EntitlementRefreshToken, host); err != nil || result == nil {
		t.Fatalf("registered workspace control: %v", err)
	}
	_, lease, err := f.reg.AcquireRuntimeLifecycle(context.Background(), tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if code := f.withdraw(t, "canceled"); code != 200 {
		t.Fatalf("withdrawal status%d", code)
	}
	if result, err := f.p.hostedEntitlements.RefreshEntitlement(billing.EntitlementRefreshToken, host); result != nil || !errors.Is(err, entitlements.ErrHostedEntitlementInactive) {
		t.Fatalf("withdrawal did not deny registered refresh: %v", err)
	}
	if code := f.event(t, "evt_existing_grant", "customer.subscription.updated", Subscription{ID: f.session.Subscription, Customer: f.session.Customer, Status: "active", Metadata: f.session.Metadata}); code != 200 {
		t.Fatalf("grant status%d", code)
	}
	sa, err := f.reg.GetStripeAccount("a-checkout")
	if err != nil || sa.SubscriptionState != "canceled" {
		t.Fatalf("new-checkout fallback granted a registered workspace: %+v/%v", sa, err)
	}
	if result, err := f.p.hostedEntitlements.RefreshEntitlement(billing.EntitlementRefreshToken, host); result != nil || !errors.Is(err, entitlements.ErrHostedEntitlementInactive) {
		t.Fatalf("registered workspace bypassed its billing/runtime path: %v", err)
	}
}
