package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestAlertsEndpoints(t *testing.T) {
	srv := newIntegrationServer(t)

	// 1. Get initial alert config
	t.Run("GetAlertConfig", func(t *testing.T) {
		res, err := http.Get(srv.server.URL + "/api/alerts/config")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}

		var config alerts.AlertConfig
		if err := json.NewDecoder(res.Body).Decode(&config); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
	})

	// 2. Update alert config
	t.Run("UpdateAlertConfig", func(t *testing.T) {
		newConfig := alerts.AlertConfig{
			Schedule: alerts.ScheduleConfig{
				Cooldown: 300,
			},
		}
		body, _ := json.Marshal(newConfig)
		// HandleAlerts expects PUT for config updates
		req, err := http.NewRequest(http.MethodPut, srv.server.URL+"/api/alerts/config", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("create request failed: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}

		// Verify update persistence
		resVerify, err := http.Get(srv.server.URL + "/api/alerts/config")
		if err != nil {
			t.Fatalf("verify request failed: %v", err)
		}
		defer resVerify.Body.Close()

		var updatedConfig alerts.AlertConfig
		if err := json.NewDecoder(resVerify.Body).Decode(&updatedConfig); err != nil {
			t.Fatalf("decode failed: %v", err)
		}

		if updatedConfig.Schedule.Cooldown != 300 {
			t.Errorf("expected cooldown 300, got %d", updatedConfig.Schedule.Cooldown)
		}
	})

	t.Run("AlertIntentPolicies", func(t *testing.T) {
		res, err := http.Get(srv.server.URL + "/api/alerts/intent-policies")
		if err != nil {
			t.Fatalf("get policies: %v", err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("get policies status = %d, want %d", res.StatusCode, http.StatusOK)
		}
		var document alerts.AlertIntentPolicyDocument
		if err := json.NewDecoder(res.Body).Decode(&document); err != nil {
			t.Fatalf("decode policies: %v", err)
		}

		grace := 45
		if document.Defaults == nil {
			document.Defaults = make(map[string]alerts.AlertIntentRule)
		}
		document.Defaults[string(alerts.AlertIntentSignalOffline)] = alerts.AlertIntentRule{GraceSeconds: &grace}
		body, err := json.Marshal(document)
		if err != nil {
			t.Fatalf("marshal policies: %v", err)
		}
		req, err := http.NewRequest(http.MethodPut, srv.server.URL+"/api/alerts/intent-policies", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("create policy update: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		updatedResponse, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("update policies: %v", err)
		}
		defer updatedResponse.Body.Close()
		if updatedResponse.StatusCode != http.StatusOK {
			t.Fatalf("update policies status = %d, want %d", updatedResponse.StatusCode, http.StatusOK)
		}
		var updated alerts.AlertIntentPolicyDocument
		if err := json.NewDecoder(updatedResponse.Body).Decode(&updated); err != nil {
			t.Fatalf("decode updated policies: %v", err)
		}
		if updated.Revision != document.Revision+1 {
			t.Fatalf("updated revision = %d, want %d", updated.Revision, document.Revision+1)
		}

		staleReq, err := http.NewRequest(http.MethodPut, srv.server.URL+"/api/alerts/intent-policies", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("create stale update: %v", err)
		}
		staleReq.Header.Set("Content-Type", "application/json")
		staleResponse, err := http.DefaultClient.Do(staleReq)
		if err != nil {
			t.Fatalf("stale update: %v", err)
		}
		defer staleResponse.Body.Close()
		if staleResponse.StatusCode != http.StatusConflict {
			t.Fatalf("stale update status = %d, want %d", staleResponse.StatusCode, http.StatusConflict)
		}

		previewBody := []byte(`{"resourceId":"vm:101","resourceType":"vm","signal":"state.offline","conditionActive":true}`)
		previewResponse, err := http.Post(srv.server.URL+"/api/alerts/intent-policies/preview", "application/json", bytes.NewReader(previewBody))
		if err != nil {
			t.Fatalf("preview policy: %v", err)
		}
		defer previewResponse.Body.Close()
		if previewResponse.StatusCode != http.StatusOK {
			t.Fatalf("preview status = %d, want %d", previewResponse.StatusCode, http.StatusOK)
		}
		var preview alerts.AlertIntentPolicyPreview
		if err := json.NewDecoder(previewResponse.Body).Decode(&preview); err != nil {
			t.Fatalf("decode preview: %v", err)
		}
		if preview.Status != "pending_grace" || preview.Effective.GraceSeconds != grace {
			t.Fatalf("preview = %+v", preview)
		}

		for name, payload := range map[string]string{
			"unsupported signal": `{"resourceId":"vm:101","resourceType":"vm","signal":"state.unknown","conditionActive":true}`,
			"unknown field":      `{"resourceId":"vm:101","resourceType":"vm","signal":"state.offline","conditionActive":true,"pollCount":2}`,
			"trailing document":  `{"resourceId":"vm:101","resourceType":"vm","signal":"state.offline","conditionActive":true}{}`,
		} {
			t.Run(name, func(t *testing.T) {
				response, err := http.Post(
					srv.server.URL+"/api/alerts/intent-policies/preview",
					"application/json",
					bytes.NewBufferString(payload),
				)
				if err != nil {
					t.Fatalf("invalid preview request: %v", err)
				}
				defer response.Body.Close()
				if response.StatusCode != http.StatusBadRequest {
					t.Fatalf("invalid preview status = %d, want %d", response.StatusCode, http.StatusBadRequest)
				}
			})
		}

		unknownPolicy := []byte(`{"schemaVersion":1,"revision":1,"defaults":{},"resourceTypes":{},"resources":{},"pollTolerance":2}`)
		unknownRequest, err := http.NewRequest(http.MethodPut, srv.server.URL+"/api/alerts/intent-policies", bytes.NewReader(unknownPolicy))
		if err != nil {
			t.Fatalf("create unknown-field policy update: %v", err)
		}
		unknownRequest.Header.Set("Content-Type", "application/json")
		unknownResponse, err := http.DefaultClient.Do(unknownRequest)
		if err != nil {
			t.Fatalf("unknown-field policy update: %v", err)
		}
		defer unknownResponse.Body.Close()
		if unknownResponse.StatusCode != http.StatusBadRequest {
			t.Fatalf("unknown-field policy status = %d, want %d", unknownResponse.StatusCode, http.StatusBadRequest)
		}
	})

	// 3. Activate alerts
	t.Run("ActivateAlerts", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/alerts/activate", nil)
		if err != nil {
			t.Fatalf("create request failed: %v", err)
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}

		// Activate again (should be idempotent)
		res2, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("retry request failed: %v", err)
		}
		defer res2.Body.Close()
		if res2.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res2.StatusCode, http.StatusOK)
		}
	})

	// 4. Get active alerts
	t.Run("GetActiveAlerts", func(t *testing.T) {
		res, err := http.Get(srv.server.URL + "/api/alerts/active")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}
	})

	// 5. Get alert history
	t.Run("GetAlertHistory", func(t *testing.T) {
		res, err := http.Get(srv.server.URL + "/api/alerts/history")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}

		// Test filters
		resFilter, err := http.Get(srv.server.URL + "/api/alerts/history?limit=10&severity=critical")
		if err != nil {
			t.Fatalf("filter request failed: %v", err)
		}
		defer resFilter.Body.Close()
		if resFilter.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", resFilter.StatusCode, http.StatusOK)
		}
	})

	// 6. Clear alert history
	t.Run("ClearAlertHistory", func(t *testing.T) {
		// HandleAlerts expects DELETE on /history
		req, err := http.NewRequest(http.MethodDelete, srv.server.URL+"/api/alerts/history", nil)
		if err != nil {
			t.Fatalf("create request failed: %v", err)
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}
	})

	// 7. Acknowledge Alert (Single)
	t.Run("AcknowledgeAlert", func(t *testing.T) {
		body := map[string]string{"alertIdentifier": "test-alert-id"}
		jsonBody, _ := json.Marshal(body)

		req, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/alerts/acknowledge", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("create request failed: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		// Should be 404 because alert doesn't exist, but that proves the handler code ran
		if res.StatusCode != http.StatusNotFound && res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want 404 or 200", res.StatusCode)
		}
	})

	// 8. Bulk Acknowledge
	t.Run("BulkAcknowledge", func(t *testing.T) {
		body := map[string]interface{}{
			"alertIdentifiers": []string{"alert-1", "alert-2"},
		}
		jsonBody, _ := json.Marshal(body)

		req, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/alerts/bulk/acknowledge", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("create request failed: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}
	})

	// 9. Bulk Clear
	t.Run("BulkClear", func(t *testing.T) {
		body := map[string]interface{}{
			"alertIdentifiers": []string{"alert-1", "alert-2"},
		}
		jsonBody, _ := json.Marshal(body)

		req, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/alerts/bulk/clear", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("create request failed: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}
	})

	// 10. Incident Timeline
	t.Run("GetIncidentTimeline", func(t *testing.T) {
		// Test timeline list by resource
		res, err := http.Get(srv.server.URL + "/api/alerts/incidents?resource_id=test-node")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
		}

		// Test specific alert timeline
		res2, err := http.Get(srv.server.URL + "/api/alerts/incidents?alertIdentifier=test-alert")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res2.Body.Close()

		// 200 OK (empty/null) or 404 depending on impl. Implementation returns null/empty usually if not found but status 200.
		if res2.StatusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", res2.StatusCode, http.StatusOK)
		}
	})
}
