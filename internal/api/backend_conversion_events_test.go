package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rcourtman/pulse-go-rewrite/pkg/licensing/metering"
)

// newTestRecorderWithStore creates a conversion recorder backed by both an
// in-memory aggregator and a SQLite durable store rooted in dir.
func newTestRecorderWithStore(t *testing.T, dir string) (*pkglicensing.Recorder, *pkglicensing.ConversionStore) {
	t.Helper()
	store, err := pkglicensing.NewConversionStore(filepath.Join(dir, "conversion.db"))
	if err != nil {
		t.Fatalf("NewConversionStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	rec := pkglicensing.NewRecorderFromWindowedAggregator(metering.NewWindowedAggregator(), store)
	return rec, store
}

// queryAllEvents queries all events for an org within a generous time window.
func queryAllEvents(t *testing.T, store *pkglicensing.ConversionStore, orgID string) []pkglicensing.StoredConversionEvent {
	t.Helper()
	from := time.Now().Add(-1 * time.Hour)
	to := time.Now().Add(1 * time.Hour)
	events, err := store.Query(orgID, from, to, "")
	if err != nil {
		t.Fatalf("Query(%q): %v", orgID, err)
	}
	return events
}

// --- LicenseHandlers.emitConversionEvent tests ---

func TestLicenseHandlersEmitConversionEvent_CheckoutStartedViaHandleStartTrial(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false, &config.Config{PublicURL: "https://pulse.example.com"})

	rec, store := newTestRecorderWithStore(t, baseDir)
	health := pkglicensing.NewPipelineHealth()
	h.SetConversionRecorder(rec, health)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	req := httptest.NewRequest(http.MethodPost, "/api/license/trial/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	h.HandleStartTrial(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("HandleStartTrial status=%d, want %d: %s", w.Code, http.StatusConflict, w.Body.String())
	}

	events := queryAllEvents(t, store, "default")
	found := false
	for _, e := range events {
		if e.EventType == pkglicensing.EventCheckoutStarted && e.Surface == "license_api" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected checkout_started event with surface=license_api in store, got %d events: %+v", len(events), events)
	}

	if hs := health.CheckHealth(); hs.EventsTotal < 1 {
		t.Fatalf("pipeline health events_total=%d, want >=1", hs.EventsTotal)
	}
}

func TestLicenseHandlersEmitConversionEvent_ActivationFailedOnBadKey(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)

	rec, store := newTestRecorderWithStore(t, baseDir)
	health := pkglicensing.NewPipelineHealth()
	h.SetConversionRecorder(rec, health)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")

	body := []byte(`{"license_key":"invalid-key-that-will-fail"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/license/activate", nil).WithContext(ctx)
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleActivateLicense(w, req)

	// Activation should fail (bad key).
	if w.Code != http.StatusBadRequest {
		t.Fatalf("HandleActivateLicense status=%d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}

	// We still expect a license_activation_failed event.
	events := queryAllEvents(t, store, "default")
	found := false
	for _, e := range events {
		if e.EventType == pkglicensing.EventLicenseActivationFailed && e.Surface == "license_api" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected license_activation_failed event in store after bad key, got %d events: %+v", len(events), events)
	}
}

func TestLicenseHandlersEmitConversionEvent_RespectsDisableFlag(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	cfg := &config.Config{
		DisableLocalUpgradeMetrics: true,
		PublicURL:                  "https://pulse.example.com",
	}
	h := NewLicenseHandlers(mtp, false, cfg)

	rec, store := newTestRecorderWithStore(t, baseDir)
	h.SetConversionRecorder(rec, pkglicensing.NewPipelineHealth())

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	req := httptest.NewRequest(http.MethodPost, "/api/license/trial/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	h.HandleStartTrial(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("HandleStartTrial status=%d, want %d: %s", w.Code, http.StatusConflict, w.Body.String())
	}

	events := queryAllEvents(t, store, "default")
	for _, e := range events {
		if e.EventType == pkglicensing.EventCheckoutStarted {
			t.Fatalf("expected no checkout_started event when DisableLocalUpgradeMetrics=true, but found one: %+v", e)
		}
	}
}

func TestLicenseHandlersEmitConversionEvent_NilRecorderDoesNotPanic(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false, &config.Config{PublicURL: "https://pulse.example.com"})
	// Deliberately NOT setting a conversion recorder.

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	req := httptest.NewRequest(http.MethodPost, "/api/license/trial/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	h.HandleStartTrial(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("HandleStartTrial status=%d, want %d: %s", w.Code, http.StatusConflict, w.Body.String())
	}
	// No panic — success.
}

func TestLicenseHandlersEmitConversionEvent_LicenseActivatedDirectEmit(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)

	rec, store := newTestRecorderWithStore(t, baseDir)
	h.SetConversionRecorder(rec, pkglicensing.NewPipelineHealth())

	h.emitConversionEvent("default", conversionEvent{
		Type:    conversionEventLicenseActivated,
		Surface: "license_api",
	})

	events := queryAllEvents(t, store, "default")
	found := false
	for _, e := range events {
		if e.EventType == pkglicensing.EventLicenseActivated && e.Surface == "license_api" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected license_activated event with surface=license_api, got %d events: %+v", len(events), events)
	}
}

// --- StripeWebhookHandlers.emitConversionEvent tests ---

func TestStripeWebhookHandlersEmitConversionEvent_DirectEmit(t *testing.T) {
	dir := t.TempDir()
	rec, store := newTestRecorderWithStore(t, dir)
	health := pkglicensing.NewPipelineHealth()

	h := &StripeWebhookHandlers{
		conversionRecorder: rec,
		conversionHealth:   health,
	}

	h.emitConversionEvent("org-stripe-1", conversionEvent{
		Type:    conversionEventCheckoutCompleted,
		Surface: "stripe_webhook",
	})

	events := queryAllEvents(t, store, "org-stripe-1")
	if len(events) == 0 {
		t.Fatal("expected checkout_completed event in store, got none")
	}
	if events[0].EventType != pkglicensing.EventCheckoutCompleted {
		t.Fatalf("event type=%q, want %q", events[0].EventType, pkglicensing.EventCheckoutCompleted)
	}
	if events[0].Surface != "stripe_webhook" {
		t.Fatalf("surface=%q, want stripe_webhook", events[0].Surface)
	}
	if hs := health.CheckHealth(); hs.EventsTotal < 1 {
		t.Fatalf("pipeline health events_total=%d, want >=1", hs.EventsTotal)
	}
}

func TestStripeWebhookHandlersEmitConversionEvent_RespectsDisableMetrics(t *testing.T) {
	dir := t.TempDir()
	rec, store := newTestRecorderWithStore(t, dir)

	h := &StripeWebhookHandlers{
		conversionRecorder: rec,
		disableMetrics:     func() bool { return true },
	}

	h.emitConversionEvent("org-stripe-2", conversionEvent{
		Type:    conversionEventCheckoutCompleted,
		Surface: "stripe_webhook",
	})

	events := queryAllEvents(t, store, "org-stripe-2")
	if len(events) != 0 {
		t.Fatalf("expected no events when disableMetrics=true, got %d", len(events))
	}
}

func TestStripeWebhookHandlersEmitConversionEvent_NilRecorderDoesNotPanic(t *testing.T) {
	h := &StripeWebhookHandlers{} // no recorder
	// Should not panic.
	h.emitConversionEvent("org-stripe-3", conversionEvent{
		Type:    conversionEventCheckoutCompleted,
		Surface: "stripe_webhook",
	})
}

func TestStripeWebhookHandlersEmitConversionEvent_DefaultsOrgIDToDefault(t *testing.T) {
	dir := t.TempDir()
	rec, store := newTestRecorderWithStore(t, dir)

	h := &StripeWebhookHandlers{
		conversionRecorder: rec,
	}

	h.emitConversionEvent("", conversionEvent{
		Type:    conversionEventCheckoutCompleted,
		Surface: "stripe_webhook",
	})

	events := queryAllEvents(t, store, "default")
	if len(events) == 0 {
		t.Fatal("expected event with org_id=default when empty string passed, got none")
	}
}

// --- emitLimitBlockedEvent tests (package-level function) ---

func TestEmitLimitBlockedEvent_RecordsEvent(t *testing.T) {
	dir := t.TempDir()
	rec, store := newTestRecorderWithStore(t, dir)
	health := pkglicensing.NewPipelineHealth()

	// Wire the package-level enforcement recorder.
	SetEnforcementConversionRecorder(rec, health)
	t.Cleanup(func() { SetEnforcementConversionRecorder(nil, nil, nil) })

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "test-org")
	emitLimitBlockedEvent(ctx, 5, 5)

	events := queryAllEvents(t, store, "test-org")
	found := false
	for _, e := range events {
		if e.EventType == pkglicensing.EventLimitBlocked && e.Surface == "agent_enforcement" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected limit_blocked event with surface=agent_enforcement, got %d events: %+v", len(events), events)
	}
	if hs := health.CheckHealth(); hs.EventsTotal < 1 {
		t.Fatalf("pipeline health events_total=%d, want >=1", hs.EventsTotal)
	}
}

func TestEmitLimitBlockedEvent_DefaultOrgWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	rec, store := newTestRecorderWithStore(t, dir)

	SetEnforcementConversionRecorder(rec, nil)
	t.Cleanup(func() { SetEnforcementConversionRecorder(nil, nil, nil) })

	ctx := context.Background() // No org ID in context.
	emitLimitBlockedEvent(ctx, 5, 5)

	events := queryAllEvents(t, store, "default")
	if len(events) == 0 {
		t.Fatal("expected limit_blocked event with org_id=default, got none")
	}
}

func TestEmitLimitBlockedEvent_RespectsDisableAll(t *testing.T) {
	dir := t.TempDir()
	rec, store := newTestRecorderWithStore(t, dir)

	SetEnforcementConversionRecorder(rec, nil, func() bool { return true })
	t.Cleanup(func() { SetEnforcementConversionRecorder(nil, nil, nil) })

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "test-org")
	emitLimitBlockedEvent(ctx, 5, 5)

	events := queryAllEvents(t, store, "test-org")
	if len(events) != 0 {
		t.Fatalf("expected no events when disableAll=true, got %d", len(events))
	}
}

func TestEmitLimitBlockedEvent_NilRecorderDoesNotPanic(t *testing.T) {
	SetEnforcementConversionRecorder(nil, nil)
	t.Cleanup(func() { SetEnforcementConversionRecorder(nil, nil, nil) })

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "test-org")
	emitLimitBlockedEvent(ctx, 5, 5) // Should not panic.
}
