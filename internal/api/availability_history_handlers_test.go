package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func TestHandleAvailabilityHistoryReturnsBatchResultsAndPerTargetNotFound(t *testing.T) {
	monitor, _, _ := newTestMonitor(t)
	store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	setUnexportedField(t, monitor, "metricsStore", store)

	persistence := config.NewConfigPersistence(t.TempDir())
	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID: "target-one", Name: "Gateway", Address: "gateway.local", TargetKind: config.AvailabilityTargetService,
		Protocol: config.AvailabilityProbeHTTPS, Enabled: true,
	})
	if err := persistence.SaveAvailabilityTargets([]config.AvailabilityTarget{target}); err != nil {
		t.Fatal(err)
	}
	latency := int64(24)
	at := time.Now().UTC().Truncate(time.Minute).Add(-10 * time.Minute)
	if err := store.WriteAvailabilityObservationSync(metrics.AvailabilityObservation{
		ObservationID: "api-observation", TargetID: target.ID, ConfigRevision: target.ConfigRevision,
		Outcome: metrics.AvailabilityReachable, ObservedAt: at, TimelineAt: at, IngestedAt: at,
		ValidFor: 5 * time.Minute, ExecutionSource: metrics.AvailabilitySourceLocal, LatencyMillis: &latency,
	}); err != nil {
		t.Fatal(err)
	}

	router := &Router{monitor: monitor, persistence: persistence}
	body, _ := json.Marshal(availabilityHistoryRequest{TargetIDs: []string{target.ID, "missing"}, Range: "24h"})
	req := httptest.NewRequest(http.MethodPost, "/api/availability-history", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.handleAvailabilityHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var response availabilityHistoryResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Targets) != 2 {
		t.Fatalf("targets = %+v", response.Targets)
	}
	if response.Targets[0].Summary == nil || response.Targets[0].Summary.ReachableSeconds != 300 {
		t.Fatalf("known target = %+v", response.Targets[0])
	}
	if len(response.Targets[0].Buckets) == 0 || len(response.Targets[0].Buckets) > availabilityHistoryMaxBuckets {
		t.Fatalf("bucket count = %d", len(response.Targets[0].Buckets))
	}
	if response.Targets[1].Error == nil || response.Targets[1].Error.Code != "not_found" {
		t.Fatalf("missing target = %+v", response.Targets[1])
	}
}

func TestHandleAvailabilityHistoryEnforcesMethodRangeAndUniqueTargetBound(t *testing.T) {
	router := &Router{}

	rec := httptest.NewRecorder()
	router.handleAvailabilityHistory(rec, httptest.NewRequest(http.MethodGet, "/api/availability-history", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d", rec.Code)
	}

	body, _ := json.Marshal(availabilityHistoryRequest{TargetIDs: []string{"one"}, Range: "14d"})
	rec = httptest.NewRecorder()
	router.handleAvailabilityHistory(rec, httptest.NewRequest(http.MethodPost, "/api/availability-history", bytes.NewReader(body)))
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("14d free status = %d, body=%s", rec.Code, rec.Body.String())
	}

	ids := make([]string, availabilityHistoryMaxTargets+1)
	for index := range ids {
		ids[index] = fmt.Sprintf("target-%d", index)
	}
	body, _ = json.Marshal(availabilityHistoryRequest{TargetIDs: ids, Range: "24h"})
	rec = httptest.NewRecorder()
	router.handleAvailabilityHistory(rec, httptest.NewRequest(http.MethodPost, "/api/availability-history", bytes.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("201 target status = %d, body=%s", rec.Code, rec.Body.String())
	}
}
