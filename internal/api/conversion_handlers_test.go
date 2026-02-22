package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rcourtman/pulse-go-rewrite/pkg/licensing/metering"
)

func TestConversionHandleRecordEventValidPOST(t *testing.T) {
	handlers := NewConversionHandlers(nil, nil, nil, nil, nil)

	body := []byte(fmt.Sprintf(`{
		"type":"paywall_viewed",
		"capability":"long_term_metrics",
		"surface":"history_chart",
		"tenant_mode":"single",
		"timestamp":%d,
		"idempotency_key":"paywall_viewed:history_chart:long_term_metrics:1"
	}`, time.Now().UnixMilli()))

	req := httptest.NewRequest(http.MethodPost, "/api/conversion/events", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handlers.HandleRecordEvent(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	var resp map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	if !resp["accepted"] {
		t.Fatalf("accepted = %v, want true", resp["accepted"])
	}
}

func TestConversionHandleRecordEventInvalidBody(t *testing.T) {
	handlers := NewConversionHandlers(nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/conversion/events", bytes.NewReader([]byte("{")))
	rec := httptest.NewRecorder()

	handlers.HandleRecordEvent(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	if resp["error"] != "validation_error" {
		t.Fatalf("error = %q, want validation_error", resp["error"])
	}
}

func TestConversionHandleRecordEventMissingRequiredFields(t *testing.T) {
	handlers := NewConversionHandlers(nil, nil, nil, nil, nil)

	body := []byte(fmt.Sprintf(`{
		"type":"paywall_viewed",
		"surface":"history_chart",
		"timestamp":%d,
		"idempotency_key":"paywall_viewed:history_chart::1"
	}`, time.Now().UnixMilli()))

	req := httptest.NewRequest(http.MethodPost, "/api/conversion/events", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handlers.HandleRecordEvent(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	if resp["error"] != "validation_error" {
		t.Fatalf("error = %q, want validation_error", resp["error"])
	}
}

func TestConversionHandleRecordEventNonPOST(t *testing.T) {
	handlers := NewConversionHandlers(nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/conversion/events", nil)
	rec := httptest.NewRecorder()

	handlers.HandleRecordEvent(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestConversionHandleGetStats(t *testing.T) {
	agg := metering.NewWindowedAggregator()
	recorder := pkglicensing.NewRecorderFromWindowedAggregator(agg, nil)
	handlers := NewConversionHandlers(recorder, nil, nil, nil, nil)

	events := []pkglicensing.ConversionEvent{
		{
			Type:           pkglicensing.EventPaywallViewed,
			OrgID:          "default",
			Capability:     "long_term_metrics",
			Surface:        "history_chart",
			Timestamp:      time.Now().UnixMilli(),
			IdempotencyKey: "paywall_viewed:history_chart:long_term_metrics:1",
		},
		{
			Type:           pkglicensing.EventPaywallViewed,
			OrgID:          "default",
			Capability:     "long_term_metrics",
			Surface:        "history_chart",
			Timestamp:      time.Now().UnixMilli(),
			IdempotencyKey: "paywall_viewed:history_chart:long_term_metrics:2",
		},
		{
			Type:           pkglicensing.EventTrialStarted,
			OrgID:          "default",
			Surface:        "license_panel",
			Timestamp:      time.Now().UnixMilli(),
			IdempotencyKey: "trial_started:license_panel::1",
		},
	}
	for _, event := range events {
		if err := recorder.Record(event); err != nil {
			t.Fatalf("record failed: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/conversion/stats", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		WindowStart int64 `json:"window_start"`
		WindowEnd   int64 `json:"window_end"`
		Buckets     []struct {
			Type       string `json:"type"`
			Key        string `json:"key"`
			Count      int64  `json:"count"`
			TotalValue int64  `json:"total_value"`
		} `json:"buckets"`
		TotalEvents int64 `json:"total_events"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}

	if resp.WindowStart <= 0 {
		t.Fatalf("window_start = %d, want > 0", resp.WindowStart)
	}
	if resp.WindowEnd < resp.WindowStart {
		t.Fatalf("window_end = %d, want >= window_start %d", resp.WindowEnd, resp.WindowStart)
	}
	if len(resp.Buckets) != 2 {
		t.Fatalf("len(buckets) = %d, want 2", len(resp.Buckets))
	}
	if resp.TotalEvents != 3 {
		t.Fatalf("total_events = %d, want 3", resp.TotalEvents)
	}

	byKey := make(map[string]struct {
		Type       string
		Count      int64
		TotalValue int64
	}, len(resp.Buckets))
	for _, bucket := range resp.Buckets {
		byKey[bucket.Key] = struct {
			Type       string
			Count      int64
			TotalValue int64
		}{
			Type:       bucket.Type,
			Count:      bucket.Count,
			TotalValue: bucket.TotalValue,
		}
	}

	paywallBucket, ok := byKey["history_chart:long_term_metrics"]
	if !ok {
		t.Fatal("missing history_chart:long_term_metrics bucket")
	}
	if paywallBucket.Type != pkglicensing.EventPaywallViewed {
		t.Fatalf("paywall bucket type = %q, want %q", paywallBucket.Type, pkglicensing.EventPaywallViewed)
	}
	if paywallBucket.Count != 2 {
		t.Fatalf("paywall bucket count = %d, want 2", paywallBucket.Count)
	}
	if paywallBucket.TotalValue != 2 {
		t.Fatalf("paywall bucket total_value = %d, want 2", paywallBucket.TotalValue)
	}

	trialBucket, ok := byKey["license_panel:"]
	if !ok {
		t.Fatal("missing license_panel: bucket")
	}
	if trialBucket.Type != pkglicensing.EventTrialStarted {
		t.Fatalf("trial bucket type = %q, want %q", trialBucket.Type, pkglicensing.EventTrialStarted)
	}
	if trialBucket.Count != 1 {
		t.Fatalf("trial bucket count = %d, want 1", trialBucket.Count)
	}
	if trialBucket.TotalValue != 1 {
		t.Fatalf("trial bucket total_value = %d, want 1", trialBucket.TotalValue)
	}
}

func TestConversionHandleGetStatsNonGET(t *testing.T) {
	handlers := NewConversionHandlers(nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/conversion/stats", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetStats(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestConversionHandleGetHealth(t *testing.T) {
	health := pkglicensing.NewPipelineHealth()
	handlers := NewConversionHandlers(nil, health, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/conversion/health", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp pkglicensing.HealthStatus
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}

	if resp.Status == "" {
		t.Fatal("status is empty")
	}
	if resp.StartedAt <= 0 {
		t.Fatalf("started_at = %d, want > 0", resp.StartedAt)
	}
	if resp.EventsByType == nil {
		t.Fatal("events_by_type is nil")
	}
}

func TestConversionHandleGetHealthNonGET(t *testing.T) {
	handlers := NewConversionHandlers(nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/conversion/health", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetHealth(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestConversionHandleRecordEventUpdatesHealth(t *testing.T) {
	agg := metering.NewWindowedAggregator()
	recorder := pkglicensing.NewRecorderFromWindowedAggregator(agg, nil)
	health := pkglicensing.NewPipelineHealth()
	handlers := NewConversionHandlers(recorder, health, nil, nil, nil)

	body := []byte(fmt.Sprintf(`{
		"type":"paywall_viewed",
		"capability":"long_term_metrics",
		"surface":"history_chart",
		"tenant_mode":"single",
		"timestamp":%d,
		"idempotency_key":"paywall_viewed:history_chart:long_term_metrics:health"
	}`, time.Now().UnixMilli()))

	req := httptest.NewRequest(http.MethodPost, "/api/conversion/events", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handlers.HandleRecordEvent(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	healthReq := httptest.NewRequest(http.MethodGet, "/api/conversion/health", nil)
	healthRec := httptest.NewRecorder()
	handlers.HandleGetHealth(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", healthRec.Code, http.StatusOK)
	}

	var healthResp pkglicensing.HealthStatus
	if err := json.NewDecoder(healthRec.Body).Decode(&healthResp); err != nil {
		t.Fatalf("failed decoding health response: %v", err)
	}
	if healthResp.EventsTotal != 1 {
		t.Fatalf("health events_total = %d, want 1", healthResp.EventsTotal)
	}
	if healthResp.EventsByType[pkglicensing.EventPaywallViewed] != 1 {
		t.Fatalf("health events_by_type[%q] = %d, want 1", pkglicensing.EventPaywallViewed, healthResp.EventsByType[pkglicensing.EventPaywallViewed])
	}
}

func TestConversionHandleGetConfigDefaults(t *testing.T) {
	handlers := NewConversionHandlers(nil, nil, pkglicensing.NewCollectionConfig(), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/conversion/config", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var snapshot pkglicensing.CollectionConfigSnapshot
	if err := json.NewDecoder(rec.Body).Decode(&snapshot); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	if !snapshot.Enabled {
		t.Fatal("snapshot.Enabled = false, want true")
	}
	if len(snapshot.DisabledSurfaces) != 0 {
		t.Fatalf("len(snapshot.DisabledSurfaces) = %d, want 0", len(snapshot.DisabledSurfaces))
	}
}

func TestConversionHandleUpdateConfigDisablesCollection(t *testing.T) {
	config := pkglicensing.NewCollectionConfig()
	handlers := NewConversionHandlers(nil, nil, config, nil, nil)

	body := []byte(`{"enabled":false,"disabled_surfaces":["history_chart"]}`)
	req := httptest.NewRequest(http.MethodPut, "/api/conversion/config", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handlers.HandleUpdateConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var snapshot pkglicensing.CollectionConfigSnapshot
	if err := json.NewDecoder(rec.Body).Decode(&snapshot); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	if snapshot.Enabled {
		t.Fatal("snapshot.Enabled = true, want false")
	}
	if config.IsEnabled() {
		t.Fatal("config.IsEnabled() = true, want false")
	}
}

func TestConversionHandleRecordEventReturnsAcceptedWhenCollectionDisabled(t *testing.T) {
	agg := metering.NewWindowedAggregator()
	recorder := pkglicensing.NewRecorderFromWindowedAggregator(agg, nil)
	config := pkglicensing.NewCollectionConfig()
	config.UpdateConfig(pkglicensing.CollectionConfigSnapshot{Enabled: false})
	handlers := NewConversionHandlers(recorder, nil, config, nil, nil)

	body := []byte(fmt.Sprintf(`{
		"type":"paywall_viewed",
		"capability":"long_term_metrics",
		"surface":"history_chart",
		"tenant_mode":"single",
		"timestamp":%d,
		"idempotency_key":"paywall_viewed:history_chart:long_term_metrics:disabled"
	}`, time.Now().UnixMilli()))

	req := httptest.NewRequest(http.MethodPost, "/api/conversion/events", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handlers.HandleRecordEvent(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	buckets := recorder.Snapshot()
	if len(buckets) != 0 {
		t.Fatalf("len(recorder.Snapshot()) = %d, want 0", len(buckets))
	}
}

func TestConversionHandleRecordEventRejectsCrossTenantOrgIDSpoof(t *testing.T) {
	tmp := t.TempDir()
	store, err := pkglicensing.NewConversionStore(filepath.Join(tmp, "conversion.db"))
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	recorder := pkglicensing.NewRecorder(nil, store, nil)
	handlers := NewConversionHandlers(recorder, nil, nil, store, nil)

	body := []byte(fmt.Sprintf(`{
		"type":"paywall_viewed",
		"org_id":"org-b",
		"capability":"long_term_metrics",
		"surface":"history_chart",
		"tenant_mode":"multi",
		"timestamp":%d,
		"idempotency_key":"org-spoof-attempt-1"
	}`, time.Now().UnixMilli()))

	req := httptest.NewRequest(http.MethodPost, "/api/conversion/events", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-a"))
	rec := httptest.NewRecorder()

	handlers.HandleRecordEvent(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	if resp["error"] != "org_mismatch" {
		t.Fatalf("error = %q, want org_mismatch", resp["error"])
	}

	orgAEvents, err := store.Query("org-a", time.Time{}, time.Time{}, "")
	if err != nil {
		t.Fatalf("Query(org-a) error = %v", err)
	}
	if len(orgAEvents) != 0 {
		t.Fatalf("expected no org-a events, got %d", len(orgAEvents))
	}

	orgBEvents, err := store.Query("org-b", time.Time{}, time.Time{}, "")
	if err != nil {
		t.Fatalf("Query(org-b) error = %v", err)
	}
	if len(orgBEvents) != 0 {
		t.Fatalf("expected no org-b events, got %d", len(orgBEvents))
	}
}

func TestConversionHandleRecordEventAllowsMatchingOrgID(t *testing.T) {
	tmp := t.TempDir()
	store, err := pkglicensing.NewConversionStore(filepath.Join(tmp, "conversion.db"))
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	recorder := pkglicensing.NewRecorder(nil, store, nil)
	handlers := NewConversionHandlers(recorder, nil, nil, store, nil)

	body := []byte(fmt.Sprintf(`{
		"type":"paywall_viewed",
		"org_id":"org-a",
		"capability":"long_term_metrics",
		"surface":"history_chart",
		"tenant_mode":"multi",
		"timestamp":%d,
		"idempotency_key":"org-matching-1"
	}`, time.Now().UnixMilli()))

	req := httptest.NewRequest(http.MethodPost, "/api/conversion/events", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-a"))
	rec := httptest.NewRecorder()

	handlers.HandleRecordEvent(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	orgAEvents, err := store.Query("org-a", time.Time{}, time.Time{}, "")
	if err != nil {
		t.Fatalf("Query(org-a) error = %v", err)
	}
	if len(orgAEvents) != 1 {
		t.Fatalf("expected one org-a event, got %d", len(orgAEvents))
	}
}

func TestConversionHandleConfigMethodNotAllowed(t *testing.T) {
	handlers := NewConversionHandlers(nil, nil, pkglicensing.NewCollectionConfig(), nil, nil)

	getReq := httptest.NewRequest(http.MethodPost, "/api/conversion/config", nil)
	getRec := httptest.NewRecorder()
	handlers.HandleGetConfig(getRec, getReq)
	if getRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET handler status = %d, want %d", getRec.Code, http.StatusMethodNotAllowed)
	}

	putReq := httptest.NewRequest(http.MethodGet, "/api/conversion/config", nil)
	putRec := httptest.NewRecorder()
	handlers.HandleUpdateConfig(putRec, putReq)
	if putRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT handler status = %d, want %d", putRec.Code, http.StatusMethodNotAllowed)
	}
}

func TestConversionHandleConversionFunnelAggregatesPerOrg(t *testing.T) {
	tmp := t.TempDir()
	store, err := pkglicensing.NewConversionStore(filepath.Join(tmp, "conversion.db"))
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	recorder := pkglicensing.NewRecorder(nil, store, nil)
	handlers := NewConversionHandlers(recorder, nil, nil, store, nil)

	now := time.Now().UTC()
	post := func(orgID string, payload string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/conversion/events", bytes.NewReader([]byte(payload)))
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, orgID))
		rec := httptest.NewRecorder()
		handlers.HandleRecordEvent(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("record status = %d, want %d (%s)", rec.Code, http.StatusAccepted, rec.Body.String())
		}
	}

	post("org-a", fmt.Sprintf(`{"type":"paywall_viewed","capability":"long_term_metrics","surface":"history_chart","timestamp":%d,"idempotency_key":"a:pv:1"}`, now.UnixMilli()))
	post("org-a", fmt.Sprintf(`{"type":"trial_started","surface":"license_panel","timestamp":%d,"idempotency_key":"a:ts:1"}`, now.UnixMilli()))
	post("org-a", fmt.Sprintf(`{"type":"upgrade_clicked","capability":"relay","surface":"paywall_modal","timestamp":%d,"idempotency_key":"a:uc:1"}`, now.UnixMilli()))
	post("org-a", fmt.Sprintf(`{"type":"checkout_completed","surface":"stripe_return","timestamp":%d,"idempotency_key":"a:cc:1"}`, now.UnixMilli()))
	post("org-b", fmt.Sprintf(`{"type":"paywall_viewed","capability":"relay","surface":"mobile_onboarding","timestamp":%d,"idempotency_key":"b:pv:1"}`, now.UnixMilli()))

	from := now.Add(-1 * time.Hour)
	to := now.Add(1 * time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/conversion-funnel", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-a"))
	q := req.URL.Query()
	q.Set("org_id", "org-a")
	q.Set("from", from.Format(time.RFC3339Nano))
	q.Set("to", to.Format(time.RFC3339Nano))
	req.URL.RawQuery = q.Encode()
	rec := httptest.NewRecorder()
	handlers.HandleConversionFunnel(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var summary pkglicensing.FunnelSummary
	if err := json.NewDecoder(rec.Body).Decode(&summary); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	if summary.PaywallViewed != 1 {
		t.Fatalf("PaywallViewed = %d, want 1", summary.PaywallViewed)
	}
	if summary.TrialStarted != 1 {
		t.Fatalf("TrialStarted = %d, want 1", summary.TrialStarted)
	}
	if summary.UpgradeClicked != 1 {
		t.Fatalf("UpgradeClicked = %d, want 1", summary.UpgradeClicked)
	}
	if summary.CheckoutCompleted != 1 {
		t.Fatalf("CheckoutCompleted = %d, want 1", summary.CheckoutCompleted)
	}

	reqAll := httptest.NewRequest(http.MethodGet, "/api/admin/conversion-funnel", nil)
	reqAll = reqAll.WithContext(context.WithValue(reqAll.Context(), OrgIDContextKey, "org-a"))
	qAll := reqAll.URL.Query()
	qAll.Set("from", from.Format(time.RFC3339Nano))
	qAll.Set("to", to.Format(time.RFC3339Nano))
	reqAll.URL.RawQuery = qAll.Encode()
	recAll := httptest.NewRecorder()
	handlers.HandleConversionFunnel(recAll, reqAll)
	if recAll.Code != http.StatusOK {
		t.Fatalf("status(all) = %d, want %d (%s)", recAll.Code, http.StatusOK, recAll.Body.String())
	}
	var summaryAll pkglicensing.FunnelSummary
	if err := json.NewDecoder(recAll.Body).Decode(&summaryAll); err != nil {
		t.Fatalf("failed decoding all-org response: %v", err)
	}
	if summaryAll.PaywallViewed != 1 {
		t.Fatalf("scoped PaywallViewed = %d, want 1", summaryAll.PaywallViewed)
	}
}

func TestConversionHandleConversionFunnelRejectsCrossTenantOrgOverride(t *testing.T) {
	tmp := t.TempDir()
	store, err := pkglicensing.NewConversionStore(filepath.Join(tmp, "conversion.db"))
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	defer store.Close()

	handlers := NewConversionHandlers(nil, nil, nil, store, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/conversion-funnel?org_id=org-b", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-a"))
	rec := httptest.NewRecorder()

	handlers.HandleConversionFunnel(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestConversionConversionFunnelRouteRequiresAdminProxyRole(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Proxy-User"
	cfg.ProxyAuthRoleHeader = "X-Proxy-Roles"
	cfg.ProxyAuthAdminRole = "admin"

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/conversion-funnel", nil)
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set(cfg.ProxyAuthUserHeader, "alice")
	req.Header.Set(cfg.ProxyAuthRoleHeader, "viewer|user")
	rec := httptest.NewRecorder()

	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}
