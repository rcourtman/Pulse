package cloudcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const catalogueKnownOffer = `[{"plan_version":"msp_solo","billing_cycle":"monthly","unit_amount":6900,"currency":"usd"},{"plan_version":"future_plan","billing_cycle":"annual","unit_amount":1,"currency":"usd"}]`

func TestProviderMSPPlanCatalogueCompletenessAndRetryWindows(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		status     int
		complete   string
		offers     int
		failure    bool
		ttl        time.Duration
	}{
		{"complete", `{"plans":` + catalogueKnownOffer + `,"complete":true}`, 200, "true", 1, false, 5 * time.Minute},
		{"closed", `{"plans":[],"complete":true}`, 200, "true", 0, false, 5 * time.Minute},
		{"partial", `{"plans":` + catalogueKnownOffer + `,"complete":false}`, 200, "false", 1, false, time.Minute},
		{"unknown", `{"plans":` + catalogueKnownOffer + `}`, 200, "unknown", 1, false, time.Minute},
		{"unknown_empty", `{"plans":[]}`, 200, "unknown", 0, false, time.Minute},
		{"null_completeness", `{"plans":[],"complete":null}`, 200, "unknown", 0, false, time.Minute},
		{"unavailable", `temporarily unavailable`, 503, "unknown", 0, true, time.Minute},
		{"quota", `too many requests`, 429, "unknown", 0, true, time.Minute},
		{"bad_json", `{`, 200, "unknown", 0, true, time.Minute},
		{"bad_flag", `{"plans":[],"complete":"true"}`, 200, "unknown", 0, true, time.Minute},
		{"missing_list", `{"complete":true}`, 200, "unknown", 0, true, time.Minute},
		{"null_list", `{"plans":null,"complete":true}`, 200, "unknown", 0, true, time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var reads atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reads.Add(1)
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			cfg := newProviderMSPRefreshTestConfig(t, server.URL)
			refresher := NewProviderMSPLicenseRefresher(cfg)
			now := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
			refresher.now = func() time.Time { return now }
			read := func() ProviderMSPPlanState {
				rec := httptest.NewRecorder()
				HandleProviderMSPPlan(cfg, refresher)(rec, httptest.NewRequest(http.MethodGet, "/", nil))
				if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" {
					t.Fatalf("public response %d %v", rec.Code, rec.Header())
				}
				var state ProviderMSPPlanState
				if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
					t.Fatal(err)
				}
				if len(state.Plans) != tt.offers || state.PurchaseAvailable != (tt.offers > 0) || (state.PlansError != "") != tt.failure {
					t.Fatalf("state=%+v", state)
				}
				actual := "unknown"
				if state.PlansComplete != nil {
					if *state.PlansComplete {
						actual = "true"
					} else {
						actual = "false"
					}
				}
				if actual != tt.complete {
					t.Fatalf("completeness=%s, want %s", actual, tt.complete)
				}
				if len(state.Plans) > 0 && (state.Plans[0].WorkspaceLimit != 3 || state.Plans[0].UnitAmount != 6900 || state.Plans[0].Currency != "usd" || state.Plans[0].BillingCycle != "monthly") {
					t.Fatalf("offer changed=%+v", state.Plans)
				}
				if state.LicenseID != cfg.ProviderMSPLicenseID || state.WorkspaceLimit != 2 || !state.Evaluation {
					t.Fatalf("saved licence changed=%+v", state)
				}
				return state
			}
			state := read()
			if tt.ttl == time.Minute && state.PlansRetryAfterSeconds != 60 {
				t.Fatalf("initial retry=%d", state.PlansRetryAfterSeconds)
			}
			if tt.ttl > time.Minute && state.PlansRetryAfterSeconds != 0 {
				t.Fatal("complete list advertises a failure cooldown")
			}
			for i := 0; i < 12; i++ {
				read()
			}
			if reads.Load() != 1 {
				t.Fatalf("repeated reads spent %d upstream requests", reads.Load())
			}
			now = now.Add(tt.ttl - time.Millisecond)
			state = read()
			if reads.Load() != 1 {
				t.Fatal("retry window ended early")
			}
			if tt.ttl == time.Minute && state.PlansRetryAfterSeconds != 1 {
				t.Fatalf("fractional second rounded down=%d", state.PlansRetryAfterSeconds)
			}
			now = now.Add(time.Millisecond)
			read()
			if reads.Load() != 2 {
				t.Fatalf("deliberate boundary read count=%d", reads.Load())
			}
		})
	}
}

func TestProviderMSPPlanCatalogueRecoversAndRetiresOldOffers(t *testing.T) {
	var body atomic.Value
	body.Store(`{"plans":` + catalogueKnownOffer + `,"complete":false}`)
	var status atomic.Int64
	status.Store(200)
	var reads atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		w.WriteHeader(int(status.Load()))
		_, _ = w.Write([]byte(body.Load().(string)))
	}))
	defer server.Close()
	refresher := NewProviderMSPLicenseRefresher(newProviderMSPRefreshTestConfig(t, server.URL))
	now := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
	refresher.now = func() time.Time { return now }
	partial, err := refresher.Plans(context.Background())
	if err != nil || partial.Complete == nil || *partial.Complete || len(partial.Plans) != 1 {
		t.Fatalf("partial=%+v %v", partial, err)
	}
	// Mutating a caller's list cannot change the shared verified prices.
	partial.Plans[0].UnitAmount = 1
	same, _ := refresher.Plans(context.Background())
	if same.Plans[0].UnitAmount != 6900 {
		t.Fatal("caller changed cached price")
	}
	body.Store(`{"plans":` + catalogueKnownOffer + `,"complete":true}`)
	now = now.Add(time.Minute)
	complete, err := refresher.Plans(context.Background())
	if err != nil || complete.Complete == nil || !*complete.Complete || complete.RetryAfterSeconds != 0 {
		t.Fatalf("recovery=%+v %v", complete, err)
	}
	status.Store(503)
	now = now.Add(5 * time.Minute)
	failed, err := refresher.Plans(context.Background())
	if err == nil || len(failed.Plans) != 0 || failed.Complete != nil || failed.RetryAfterSeconds != 60 {
		t.Fatalf("stale offers retained=%+v %v", failed, err)
	}
	status.Store(200)
	now = now.Add(time.Minute)
	recovered, err := refresher.Plans(context.Background())
	if err != nil || len(recovered.Plans) != 1 || recovered.RetryAfterSeconds != 0 || reads.Load() != 4 {
		t.Fatalf("final=%+v err=%v reads=%d", recovered, err, reads.Load())
	}
}

func TestProviderMSPPlanCatalogueConcurrentFailuresShareOneRead(t *testing.T) {
	var reads atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reads.Add(1); w.WriteHeader(503) }))
	defer server.Close()
	refresher := NewProviderMSPLicenseRefresher(newProviderMSPRefreshTestConfig(t, server.URL))
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := refresher.Plans(context.Background())
			if err == nil || len(result.Plans) != 0 || result.RetryAfterSeconds < 59 {
				t.Errorf("result=%+v err=%v", result, err)
			}
		}()
	}
	wg.Wait()
	if reads.Load() != 1 {
		t.Fatalf("concurrent failure requests=%d", reads.Load())
	}
}

func TestProviderMSPPlanCatalogueCancellationDoesNotPoisonSharedWindow(t *testing.T) {
	entered := make(chan struct{})
	var reads atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reads.Add(1) == 1 {
			close(entered)
			<-r.Context().Done()
			return
		}
		_, _ = w.Write([]byte(`{"plans":[],"complete":true}`))
	}))
	defer server.Close()
	refresher := NewProviderMSPLicenseRefresher(newProviderMSPRefreshTestConfig(t, server.URL))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := refresher.Plans(ctx); done <- err }()
	<-entered
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error=%v", err)
	}
	result, err := refresher.Plans(context.Background())
	if err != nil || result.Complete == nil || !*result.Complete || reads.Load() != 2 {
		t.Fatalf("cancel poisoned next read: %+v %v count=%d", result, err, reads.Load())
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	_, err = refresher.Plans(ctx)
	if !errors.Is(err, context.Canceled) || reads.Load() != 2 {
		t.Fatalf("cancelled cached reader=%v count=%d", err, reads.Load())
	}
}

func TestProviderMSPPlanCatalogueTransportFailureIsBounded(t *testing.T) {
	var reads atomic.Int64
	cfg := newProviderMSPRefreshTestConfig(t, "https://license.example.com")
	refresher := NewProviderMSPLicenseRefresher(cfg)
	refresher.client = &http.Client{Transport: catalogueFailureTransport{reads: &reads}}
	for i := 0; i < 3; i++ {
		result, err := refresher.Plans(context.Background())
		if err == nil || !strings.Contains(err.Error(), "unreachable") || len(result.Plans) > 0 {
			t.Fatalf("transport=%+v %v", result, err)
		}
	}
	if reads.Load() != 1 {
		t.Fatalf("transport failures spent %d reads", reads.Load())
	}
}

type catalogueFailureTransport struct{ reads *atomic.Int64 }

func (c catalogueFailureTransport) RoundTrip(*http.Request) (*http.Response, error) {
	c.reads.Add(1)
	return nil, errors.New("fixture transport unavailable")
}
