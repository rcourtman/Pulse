package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

func TestAttentionHandlersListUsesCanonicalCountAndFilters(t *testing.T) {
	now := time.Date(2026, 7, 19, 6, 0, 0, 0, time.UTC)
	handler := &AttentionHandlers{
		readAlerts: func(context.Context) ([]alerts.Alert, []alerts.Alert, error) {
			return []alerts.Alert{
				attentionHandlerAlert("open", operationaltrust.OperationalOpen, now),
				attentionHandlerAlert("ack", operationaltrust.OperationalAcknowledged, now),
				attentionHandlerAlert("stale", operationaltrust.OperationalStale, now),
			}, nil, nil
		},
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/ai/patrol/attention?filter=acknowledged&limit=10",
		nil,
	)
	response := httptest.NewRecorder()
	handler.HandleAttention(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var payload attentionListResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].State != operationaltrust.OperationalAcknowledged {
		t.Fatalf("data = %+v, want acknowledged item", payload.Data)
	}
	if payload.Summary.ActiveCount != 2 ||
		payload.Summary.AcknowledgedCount != 1 ||
		payload.Summary.UncertainCount != 1 {
		t.Fatalf("summary = %+v", payload.Summary)
	}
	if payload.Summary.CoverageState != "partial" {
		t.Fatalf("coverage = %q, want partial without posture source", payload.Summary.CoverageState)
	}
}

func TestAttentionHandlersSummaryAndDetailShareOneProjection(t *testing.T) {
	now := time.Date(2026, 7, 19, 7, 0, 0, 0, time.UTC)
	handler := &AttentionHandlers{
		readAlerts: func(context.Context) ([]alerts.Alert, []alerts.Alert, error) {
			return []alerts.Alert{
				attentionHandlerAlert("record-1", operationaltrust.OperationalOpen, now),
			}, nil, nil
		},
	}

	summaryRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/ai/patrol/attention/summary",
		nil,
	)
	summaryResponse := httptest.NewRecorder()
	handler.HandleAttention(summaryResponse, summaryRequest)
	if summaryResponse.Code != http.StatusOK {
		t.Fatalf("summary status = %d body=%s", summaryResponse.Code, summaryResponse.Body.String())
	}
	var summary ai.AttentionSummary
	if err := json.Unmarshal(summaryResponse.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.ActiveCount != 1 {
		t.Fatalf("ActiveCount = %d, want 1", summary.ActiveCount)
	}

	detailRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/ai/patrol/attention/record-1",
		nil,
	)
	detailResponse := httptest.NewRecorder()
	handler.HandleAttention(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail status = %d body=%s", detailResponse.Code, detailResponse.Body.String())
	}
	var detail ai.AttentionItemDetail
	if err := json.Unmarshal(detailResponse.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.Item.ID != "record-1" ||
		detail.OperationalRecord.ID != detail.Item.OperationalRecordID ||
		len(detail.Evidence) != 1 {
		t.Fatalf("detail = %+v", detail)
	}
}

func TestAttentionHandlersDetailSupportsCanonicalIDsContainingSlashes(t *testing.T) {
	now := time.Date(2026, 7, 19, 7, 30, 0, 0, time.UTC)
	const recordID = "agent:node-1/disk:mnt-disk2::metric-threshold:disk"
	handler := &AttentionHandlers{
		readAlerts: func(context.Context) ([]alerts.Alert, []alerts.Alert, error) {
			return []alerts.Alert{
				attentionHandlerAlert(recordID, operationaltrust.OperationalOpen, now),
			}, nil, nil
		},
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/ai/patrol/attention/agent%3Anode-1%2Fdisk%3Amnt-disk2%3A%3Ametric-threshold%3Adisk",
		nil,
	)
	response := httptest.NewRecorder()
	handler.HandleAttention(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var detail ai.AttentionItemDetail
	if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.Item.ID != recordID {
		t.Fatalf("detail ID = %q, want %q", detail.Item.ID, recordID)
	}
}

func TestAttentionHandlersFailClosedWhenLifecycleUnavailable(t *testing.T) {
	handler := &AttentionHandlers{
		readAlerts: func(context.Context) ([]alerts.Alert, []alerts.Alert, error) {
			return nil, nil, errors.New("collector state unavailable")
		},
	}
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/ai/patrol/attention/summary",
		nil,
	)
	response := httptest.NewRecorder()
	handler.HandleAttention(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if body := response.Body.String(); body == "" ||
		!json.Valid(response.Body.Bytes()) {
		t.Fatalf("expected typed unavailable response, got %q", body)
	}
}

func TestAttentionHandlersRejectInvalidOrUnboundedQueries(t *testing.T) {
	handler := &AttentionHandlers{
		readAlerts: func(context.Context) ([]alerts.Alert, []alerts.Alert, error) {
			return nil, nil, nil
		},
	}
	for _, path := range []string{
		"/api/ai/patrol/attention?filter=healthy",
		"/api/ai/patrol/attention?page=0",
		"/api/ai/patrol/attention?limit=201",
		"/api/ai/patrol/attention/%20",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.HandleAttention(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestParseAttentionActionPlanPathPreservesOperationalIDsContainingSlashes(t *testing.T) {
	itemID, capability, ok := parseAttentionActionPlanPath(
		"/agent:node-1/disk:mnt-disk2::metric-threshold:disk/actions/restart/plan",
	)
	if !ok ||
		itemID != "agent:node-1/disk:mnt-disk2::metric-threshold:disk" ||
		capability != "restart" {
		t.Fatalf("item=%q capability=%q ok=%t", itemID, capability, ok)
	}
}

func TestAttentionEvidenceDetailIsBoundedAndReportsExpiredRetention(t *testing.T) {
	now := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	const recordID = "agent:node-1/disk:mnt-disk2::metric-threshold:disk"
	alert := attentionHandlerAlert(recordID, operationaltrust.OperationalOpen, now)
	handler := &AttentionHandlers{
		readAlerts: func(context.Context) ([]alerts.Alert, []alerts.Alert, error) {
			return []alerts.Alert{alert}, nil, nil
		},
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/ai/patrol/attention/agent:node-1/disk:mnt-disk2::metric-threshold:disk/evidence/"+
			url.PathEscape(alert.Evidence[0].ID),
		nil,
	)
	response := httptest.NewRecorder()
	handler.HandleAttention(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var payload attentionEvidenceResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode evidence response: %v", err)
	}
	if !payload.Retained || payload.Evidence.ID != alert.Evidence[0].ID {
		t.Fatalf("evidence response = %+v", payload)
	}

	alert.OperationalRecord.EvidenceIDs = append(
		alert.OperationalRecord.EvidenceIDs,
		"evidence-expired",
	)
	expiredRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/ai/patrol/attention/agent:node-1/disk:mnt-disk2::metric-threshold:disk/evidence/evidence-expired",
		nil,
	)
	expiredResponse := httptest.NewRecorder()
	handler.HandleAttention(expiredResponse, expiredRequest)
	if expiredResponse.Code != http.StatusGone {
		t.Fatalf(
			"expired status = %d, want %d: %s",
			expiredResponse.Code,
			http.StatusGone,
			expiredResponse.Body.String(),
		)
	}
}

func TestAttentionMutationPathPreservesOperationalIDsContainingSlashes(t *testing.T) {
	for _, kind := range []attentionMutationKind{
		attentionMutationAcknowledge,
		attentionMutationUnacknowledge,
		attentionMutationSuppress,
		attentionMutationUnsuppress,
	} {
		itemID, gotKind, ok := parseAttentionMutationPath(
			"/agent:node-1/disk:mnt-disk2::metric-threshold:disk/" + string(kind),
		)
		if !ok ||
			itemID != "agent:node-1/disk:mnt-disk2::metric-threshold:disk" ||
			gotKind != kind {
			t.Fatalf("item=%q kind=%q ok=%t", itemID, gotKind, ok)
		}
	}
}

func TestAttentionLifecycleMutationsRefreshTheCanonicalProjection(t *testing.T) {
	now := time.Now().UTC()
	alert := attentionHandlerAlert(
		"agent:node-1/disk:mnt-disk2::metric-threshold:disk",
		operationaltrust.OperationalOpen,
		now,
	)
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	setUnexportedField(
		t,
		manager,
		"activeAlerts",
		map[string]*alerts.Alert{alert.ID: alert.Clone()},
	)
	setUnexportedField(
		t,
		manager,
		"activeAlertAlias",
		map[string]string{alert.OperationalRecord.ID: alert.ID},
	)
	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", models.NewState())
	setUnexportedField(t, monitor, "alertManager", manager)
	handler := NewAttentionHandlers(
		func(context.Context) *monitoring.Monitor { return monitor },
		nil,
	)

	acknowledge := httptest.NewRequest(
		http.MethodPost,
		"/api/ai/patrol/attention/agent:node-1/disk:mnt-disk2::metric-threshold:disk/acknowledge",
		bytes.NewBufferString("{}"),
	)
	acknowledgeResponse := httptest.NewRecorder()
	handler.HandleAttention(acknowledgeResponse, acknowledge)
	if acknowledgeResponse.Code != http.StatusOK {
		t.Fatalf(
			"acknowledge status = %d: %s",
			acknowledgeResponse.Code,
			acknowledgeResponse.Body.String(),
		)
	}
	assertAttentionHandlerState(
		t,
		handler,
		alert.ID,
		operationaltrust.OperationalAcknowledged,
	)

	suppress := httptest.NewRequest(
		http.MethodPost,
		"/api/ai/patrol/attention/agent:node-1/disk:mnt-disk2::metric-threshold:disk/suppress",
		bytes.NewBufferString(`{
			"reason":"Planned storage maintenance",
			"expiresAt":"`+now.Add(time.Hour).Format(time.RFC3339Nano)+`"
		}`),
	)
	suppressResponse := httptest.NewRecorder()
	handler.HandleAttention(suppressResponse, suppress)
	if suppressResponse.Code != http.StatusOK {
		t.Fatalf(
			"suppress status = %d: %s",
			suppressResponse.Code,
			suppressResponse.Body.String(),
		)
	}
	assertAttentionHandlerState(
		t,
		handler,
		alert.ID,
		operationaltrust.OperationalSuppressed,
	)

	activeList := httptest.NewRequest(
		http.MethodGet,
		"/api/ai/patrol/attention?filter=active",
		nil,
	)
	activeResponse := httptest.NewRecorder()
	handler.HandleAttention(activeResponse, activeList)
	if activeResponse.Code != http.StatusOK {
		t.Fatalf("active list status = %d: %s", activeResponse.Code, activeResponse.Body.String())
	}
	var active attentionListResponse
	if err := json.Unmarshal(activeResponse.Body.Bytes(), &active); err != nil {
		t.Fatalf("decode active list: %v", err)
	}
	if len(active.Data) != 0 || active.Summary.SuppressedCount != 1 {
		t.Fatalf("suppressed active projection = %+v", active)
	}
}

func assertAttentionHandlerState(
	t *testing.T,
	handler *AttentionHandlers,
	itemID string,
	want operationaltrust.OperationalState,
) {
	t.Helper()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/ai/patrol/attention/"+url.PathEscape(itemID),
		nil,
	)
	response := httptest.NewRecorder()
	handler.HandleAttention(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("detail status = %d: %s", response.Code, response.Body.String())
	}
	var detail ai.AttentionItemDetail
	if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.Item.State != want || detail.OperationalRecord.State != want {
		t.Fatalf(
			"detail state item=%q record=%q, want %q",
			detail.Item.State,
			detail.OperationalRecord.State,
			want,
		)
	}
}

func attentionHandlerAlert(
	id string,
	state operationaltrust.OperationalState,
	now time.Time,
) alerts.Alert {
	resourceID := "resource-" + id
	validUntil := now.Add(time.Hour)
	evidence := operationaltrust.EvidenceEnvelope{
		ID: "evidence-" + id,
		Source: operationaltrust.EvidenceSource{
			Provider:  "test",
			Collector: "test",
		},
		Subject:      operationaltrust.EvidenceSubject{ResourceID: resourceID},
		ObservedAt:   now,
		IngestedAt:   now,
		ValidUntil:   &validUntil,
		Completeness: operationaltrust.EvidenceComplete,
		Confidence:   operationaltrust.EvidenceConfirmed,
		Permissions:  operationaltrust.EvidencePermissionsSufficient,
	}
	record := operationaltrust.OperationalRecord{
		ID:                 id,
		CanonicalSpecID:    "spec-" + id,
		SubjectResourceID:  resourceID,
		State:              state,
		Severity:           operationaltrust.SeverityWarning,
		FirstObservedAt:    now.Add(-time.Hour),
		LastObservedAt:     now,
		StateChangedAt:     now,
		EvidenceIDs:        []string{evidence.ID},
		CauseKey:           "cause-" + id,
		ImpactSummary:      "Service interruption is possible.",
		RelatedResourceIDs: []string{},
	}
	switch state {
	case operationaltrust.OperationalAcknowledged:
		record.Acknowledgement = &operationaltrust.Acknowledgement{At: now, By: "operator"}
	case operationaltrust.OperationalSuppressed:
		record.Suppression = &operationaltrust.Suppression{
			At:     now,
			By:     "operator",
			Reason: "maintenance",
		}
	case operationaltrust.OperationalResolved:
		record.ResolvedAt = &now
	}
	return alerts.Alert{
		ID:                "alert-" + id,
		Type:              "service-health",
		Level:             alerts.AlertLevelWarning,
		ResourceID:        resourceID,
		ResourceName:      "Resource " + id,
		Message:           "Service health needs attention.",
		StartTime:         now.Add(-time.Hour),
		LastSeen:          now,
		OperationalRecord: &record,
		Evidence:          []operationaltrust.EvidenceEnvelope{evidence},
	}
}
